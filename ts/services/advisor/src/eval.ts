/**
 * Advisor eval harness: `pnpm eval` (needs `make up seed` and an LLM key in ../../../.env or the env).
 * Runs every scripted and multilingual scenario through the real advisor flow against the live
 * rules/quote/registry services, scores each conversation and writes docs/advisor-eval.md.
 *   LLM_PROVIDER=anthropic pnpm eval        EVAL_ONLY=S03,M02 pnpm eval
 */
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { Product, Quote } from "@udyam/shared";
import { newSession, runTurn, type TurnResult } from "./advisor.ts";
import { MULTILINGUAL, SCRIPTED, type EvalScenario } from "./eval-scenarios.ts";
import { dominantScript, expectedScript, numbersIn } from "./guard.ts";
import { advisorLlm, createChatModel } from "./llm.ts";
import { httpServices } from "./services.ts";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "../../../..");
// Load ../.env without overriding the real environment (no dotenv dependency).
if (existsSync(`${root}/.env`)) {
  for (const line of readFileSync(`${root}/.env`, "utf8").split("\n")) {
    const m = line.match(/^\s*([A-Z_]+)\s*=\s*([^#]*?)\s*(#.*)?$/);
    if (m && process.env[m[1]!] === undefined && m[2]) process.env[m[1]!] = m[2];
  }
}

const services = httpServices({
  RULES_URL: process.env.EVAL_RULES_URL ?? "http://localhost:8080/api/rules",
  QUOTE_URL: process.env.EVAL_QUOTE_URL ?? "http://localhost:8080/api/quote",
  REGISTRY_URL: process.env.EVAL_REGISTRY_URL ?? "http://localhost:8080/api/registry",
});

const CHECKS = ["noIneligible", "dependencies", "priceMatches", "language", "within6", "expectations"] as const;
type Check = (typeof CHECKS)[number];
interface Result { s: EvalScenario; checks: Record<Check, boolean>; notes: string[]; firstQuoteTurn?: number; transcript: { owner: string; advisor: string; kind: string }[] }

async function runScenario(s: EvalScenario, llm: ReturnType<typeof advisorLlm>, catalogue: Map<string, Product>): Promise<Result> {
  const state = newSession(`eval-${s.id}-${Date.now()}`);
  const turns: TurnResult[] = [];
  const quotes: Quote[] = [];
  const notes: string[] = [];
  const emit = async () => {};
  for (const msg of s.turns) {
    const r = await runTurn(state, msg, { llm, services, emit });
    turns.push(r);
    if (state.quote && quotes.at(-1)?.id !== state.quote.id) quotes.push(state.quote);
    if (r.brief.kind === "error") notes.push(`turn ${state.turn}: service error (${String(r.brief.facts.error)})`);
    if (r.guardProblems.length) notes.push(`turn ${state.turn}: guard fallback (${r.guardProblems.join("; ")})`);
  }
  const checks = {} as Record<Check, boolean>;
  const truthEl = await services.eligibility(s.truth);
  const eligible = new Set(truthEl.eligible.map((p) => p.id));

  const ineligible = quotes.flatMap((q) => q.lines.filter((l) => !eligible.has(l.productId)).map((l) => l.productId));
  checks.noIneligible = ineligible.length === 0;
  if (ineligible.length) notes.push(`ineligible for the true profile: ${[...new Set(ineligible)].join(", ")}`);

  checks.dependencies = quotes.every((q) => q.lines.every((l) => {
    const deps = catalogue.get(l.productId)?.requiresOneOf ?? [];
    return !deps.length || q.lines.some((x) => deps.includes(x.productId));
  }));

  // Every priced reply states the quote-service monthly total, and the stored quote matches.
  let priceOk = true;
  for (const r of turns) {
    const total = (r.brief.facts as { monthlyTotalInr?: number }).monthlyTotalInr;
    if (total === undefined || !["recommend", "cheaper", "mobile_only", "accepted"].includes(r.brief.kind)) continue;
    const stored = await services.getQuote(String((r.brief.facts as { quoteId: string }).quoteId));
    if (stored.monthlyTotalInr !== total) { priceOk = false; notes.push(`quote ${stored.id} total ${stored.monthlyTotalInr} != brief ${total}`); }
    if (!numbersIn(r.reply).includes(total)) { priceOk = false; notes.push(`reply on turn ${turns.indexOf(r) + 1} does not state ₹${total}`); }
  }
  checks.priceMatches = priceOk;

  const want = expectedScript(s.language, !!s.romanised);
  const names = [...catalogue.values()].map((p) => p.name);
  const wrong = turns.map((r, i) => [i + 1, dominantScript(r.reply, names)] as const).filter(([, got]) => got !== want);
  checks.language = wrong.length === 0;
  if (wrong.length) notes.push(`script ${wrong.map(([i, g]) => `t${i}=${g}`).join(",")} (want ${want})`);

  const e = s.expect ?? {};
  checks.within6 = e.noQuote ? true : state.firstQuoteTurn !== undefined && state.firstQuoteTurn <= 6;

  const exp: string[] = [];
  const final = state.quote;
  const finalIds = new Set(final?.lines.map((l) => l.productId) ?? []);
  if (e.noQuote && quotes.length) exp.push("quoted although out of scope");
  if (e.noQuoteBeforeTurn && state.firstQuoteTurn !== undefined && state.firstQuoteTurn < e.noQuoteBeforeTurn) exp.push(`quoted on turn ${state.firstQuoteTurn}`);
  for (const id of e.exclude ?? []) if (quotes.some((q) => q.lines.some((l) => l.productId === id))) exp.push(`${id} was quoted`);
  for (const id of e.include ?? []) if (!finalIds.has(id)) exp.push(`${id} missing`);
  if (e.onlyProducts && [...finalIds].some((id) => !e.onlyProducts!.includes(id))) exp.push(`extra products ${[...finalIds].join(",")}`);
  if (e.cheaper && !(quotes.length >= 2 && quotes.at(-1)!.monthlyTotalInr < quotes[0]!.monthlyTotalInr)) exp.push("no cheaper quote");
  if (e.accepted && (!final || (await services.getQuote(final.id)).status !== "accepted")) exp.push("not accepted");
  if (e.minInstallDays && (final?.expectedInstallDays ?? 0) < e.minInstallDays) exp.push(`install days ${final?.expectedInstallDays}`);
  checks.expectations = exp.length === 0;
  notes.push(...exp);

  return { s, checks, notes, firstQuoteTurn: state.firstQuoteTurn, transcript: turns.map((r, i) => ({ owner: s.turns[i]!, advisor: r.reply, kind: r.brief.kind })) };
}

async function pool<T, R>(items: T[], n: number, fn: (t: T) => Promise<R>): Promise<R[]> {
  const out: R[] = new Array(items.length);
  let next = 0;
  await Promise.all(Array.from({ length: n }, async () => {
    while (next < items.length) { const i = next++; out[i] = await fn(items[i]!); }
  }));
  return out;
}

async function main() {
  const provider = process.env.LLM_PROVIDER ?? "gemini";
  const model = await createChatModel();
  const modelName = process.env.LLM_MODEL || ({ anthropic: "claude-sonnet-5-5", deepseek: "deepseek-flash" }[provider] ?? "?");
  const llm = advisorLlm(model);
  const catalogue = new Map((await services.products()).map((p) => [p.id, p]));
  const only = process.env.EVAL_ONLY?.split(",");
  const scenarios = [...SCRIPTED, ...MULTILINGUAL].filter((s) => !only || only.includes(s.id));
  console.log(`eval: ${scenarios.length} conversations, provider=${provider} model=${modelName}`);

  const results = await pool(scenarios, Number(process.env.EVAL_CONCURRENCY ?? 3), async (s) => {
    try {
      const r = await runScenario(s, llm, catalogue);
      console.log(`${Object.values(r.checks).every(Boolean) ? "PASS" : "FAIL"} ${s.id} ${s.title}${r.notes.length ? "  - " + r.notes.join(" | ") : ""}`);
      return r;
    } catch (err) {
      console.log(`ERROR ${s.id}: ${(err as Error).message}`);
      return { s, checks: Object.fromEntries(CHECKS.map((c) => [c, false])) as Record<Check, boolean>, notes: [`error: ${(err as Error).message}`], transcript: [] };
    }
  });

  const pass = results.filter((r) => Object.values(r.checks).every(Boolean)).length;
  const rate = (100 * pass) / results.length;
  const yes = (b: boolean) => (b ? "✓" : "✗");
  const table = [
    `| id | scenario | language | first quote turn | ${CHECKS.join(" | ")} | pass | notes |`,
    `|${"---|".repeat(CHECKS.length + 6)}`,
    ...results.map((r) => `| ${r.s.id} | ${r.s.title} | ${r.s.language}${r.s.romanised ? " (romanised)" : ""} | ${r.firstQuoteTurn ?? "-"} | ${CHECKS.map((c) => yes(r.checks[c])).join(" | ")} | ${yes(Object.values(r.checks).every(Boolean))} | ${r.notes.join("; ").replace(/\|/g, "/")} |`),
  ].join("\n");
  const perCheck = CHECKS.map((c) => `${c} ${results.filter((r) => r.checks[c]).length}/${results.length}`).join(", ");
  const section = `## ${provider} / ${modelName}\n\nRun ${new Date().toISOString()}: **${pass}/${results.length} conversations pass (${rate.toFixed(0)}%)**. Per check: ${perCheck}.\n\n${table}\n`;

  const doc = `${root}/docs/advisor-eval.md`;
  const header = "# Advisor eval\n\nGenerated by `cd ts && pnpm --filter @udyam/advisor eval` (ts/services/advisor/src/eval.ts). 16 scripted scenarios and 11 multilingual conversations run against the live rules, quote and registry services. A conversation passes when every check holds:\n\n- **noIneligible**: every quoted product is eligible for the *true* profile (not the extracted one)\n- **dependencies**: every product's requires_one_of is satisfied in each quote\n- **priceMatches**: every priced reply states the quote-service monthly total, and the stored quote matches\n- **language**: every reply is in the owner's script (romanised input gets Latin script)\n- **within6**: the first quote arrives by turn 6\n- **expectations**: scenario-specific checks (exclusions, mobile-only, cheaper path, accept, install days)\n\nTarget: at least 90% on each provider. Transcripts: dist/advisor-eval-<provider>.json.\n";
  let body = existsSync(doc) ? readFileSync(doc, "utf8") : header;
  const re = new RegExp(`## ${provider} / [\\s\\S]*?(?=\\n## |$)`);
  body = re.test(body) ? body.replace(re, section.trimEnd() + "\n") : body.trimEnd() + "\n\n" + section;
  writeFileSync(doc, body);
  mkdirSync(`${root}/dist`, { recursive: true });
  writeFileSync(`${root}/dist/advisor-eval-${provider}.json`, JSON.stringify(results.map((r) => ({ id: r.s.id, checks: r.checks, notes: r.notes, transcript: r.transcript })), null, 2));
  console.log(`\n${pass}/${results.length} pass (${rate.toFixed(0)}%). Wrote docs/advisor-eval.md`);
  process.exit(rate >= 90 ? 0 : 1);
}

main().catch((e) => { console.error(e); process.exit(2); });
