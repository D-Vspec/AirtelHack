import {
  AdvisorExtraction, BusinessProfile, CITY_TIERS, TOPICS,
  type Bundle, type BundleRecommendedPayload, type Exclusion, type LanguageName, type Product,
  type ProfileUpdatedPayload, type Quote, type QuoteItem,
} from "@udyam/shared";
import { checkReply, dominantScript, numbersInValue } from "./guard.ts";
import { ServiceCallError, type Services } from "./services.ts";

/**
 * The conversation graph, as plain code: extract -> merge -> decide -> (ask | recommend |
 * cheaper | mobile | accept | out_of_scope | follow-up) -> write -> guard. The LLM only extracts
 * facts and phrases the reply; every product, quantity and price comes from the rules and quote
 * services, and the guard rejects replies that mention anything else.
 */

export interface Turn { role: "owner" | "advisor"; text: string }

export interface SessionState {
  sessionId: string;
  leadId: string;
  turn: number;
  history: Turn[];
  profile: BusinessProfile;
  counts: Partial<Record<"lines" | "cameras" | "devices" | "counters" | "vehicles", number>>;
  language: LanguageName;
  romanised: boolean;
  quote?: Quote;
  bundleId?: string;
  quotedProfileKey?: string;
  firstQuoteTurn?: number;
  gstinNote?: string;
}

/** What the writer must say. Only facts in here may appear in the reply. */
export interface Brief {
  kind: "ask" | "recommend" | "cheaper" | "mobile_only" | "accepted" | "out_of_scope" | "follow_up" | "error";
  instructions: string;
  facts: Record<string, unknown>;
  allowedProducts: string[];
}

export interface Llm {
  extract(input: { history: Turn[]; message: string; profile: BusinessProfile; catalogue: { id: string; name: string }[] }): Promise<AdvisorExtraction>;
  write(input: { brief: Brief; history: Turn[]; language: LanguageName; romanised: boolean; feedback?: string[] }): Promise<string>;
}

export type Emit = (topic: string, leadId: string, payload: unknown) => Promise<void>;

export interface Deps { llm: Llm; services: Services; emit: Emit }

export const REQUIRED: (keyof BusinessProfile)[] = ["segment", "cityTier", "employeeCount", "numSites", "hasGstin"];
export const MAX_TURNS_BEFORE_RECOMMEND = 4; // recommend by turn 4 at the latest, well inside 6

export function newSession(sessionId: string, leadId = `S-${sessionId.slice(0, 8)}`): SessionState {
  return { sessionId, leadId, turn: 0, history: [], profile: {}, counts: {}, language: "English", romanised: false };
}

/** Merge extracted facts into the profile; code (not the LLM) decides city tier for known cities. */
export function merge(state: SessionState, ext: AdvisorExtraction, catalogueIds: Set<string>): string[] {
  const p: Record<string, unknown> = { ...state.profile };
  const before = JSON.stringify(state.profile);
  const keys = ["businessName", "businessType", "segment", "city", "employeeCount", "numSites", "annualTurnoverBand",
    "hasGstin", "hasUdyam", "currentInternet", "currentProvider", "monthlyTelecomSpendInr"] as const;
  for (const k of keys) if (ext[k] !== undefined && ext[k] !== null) p[k] = ext[k];
  if (ext.gstin) p.gstin = ext.gstin.replace(/\s/g, "").toUpperCase();
  const city = Object.keys(CITY_TIERS).find((c) => c.toLowerCase() === String(p.city ?? "").trim().toLowerCase());
  if (city) { p.city = city; p.cityTier = CITY_TIERS[city]; }
  else if (ext.cityTier === 1 || ext.cityTier === 2) p.cityTier = ext.cityTier;
  if (typeof p.employeeCount === "number" && p.employeeCount < 1) delete p.employeeCount;
  if (typeof p.numSites === "number" && p.numSites < 1) delete p.numSites;
  const interests = new Set([...(state.profile.productsOfInterest ?? []), ...(ext.productsOfInterest ?? [])].filter((id) => catalogueIds.has(id)));
  if (interests.size) p.productsOfInterest = [...interests].sort();
  for (const k of ["lines", "cameras", "devices", "counters", "vehicles"] as const) {
    if (ext[k] && ext[k]! > 0) state.counts[k] = ext[k];
  }
  const parsed = BusinessProfile.safeParse(p);
  state.profile = parsed.success ? parsed.data : state.profile;
  const after = state.profile as Record<string, unknown>;
  const was = JSON.parse(before) as Record<string, unknown>;
  return Object.keys(after).filter((k) => JSON.stringify(after[k]) !== JSON.stringify(was[k]));
}

/** Registry check for a typed GSTIN: unknown, cancelled or suspended means "no usable GSTIN". */
async function verifyGstin(state: SessionState, services: Services): Promise<void> {
  const g = state.profile.gstin;
  if (!g) return;
  try {
    const r = await services.gstin(g);
    if (r.matchType !== "none" && r.record) {
      if (r.record.status === "Active") {
        state.profile = { ...state.profile, gstin: r.id, hasGstin: true };
        state.gstinNote = r.matchType === "corrected" ? `GSTIN read as ${g}, matched to registry entry ${r.id} (${r.record.tradeName}).` : undefined;
      } else {
        state.profile = { ...state.profile, hasGstin: false };
        state.gstinNote = `The GST registry shows this GSTIN as ${r.record.status}, so products that need an active GSTIN are not offered yet.`;
      }
    } else {
      state.profile = { ...state.profile, hasGstin: false };
      state.gstinNote = `This GSTIN was not found in the GST registry${r.suggestions?.length ? `; did you mean ${r.suggestions[0]}?` : ""}. Until it is confirmed, products that need a GSTIN are not offered.`;
    }
  } catch {
    // Registry down: keep the owner's declaration; KYC re-checks it later.
  }
}

const COUNT_FOR_UNIT: Record<string, keyof SessionState["counts"]> = {
  "per line": "lines", "per camera": "cameras", "per device": "devices", "per counter": "counters", "per vehicle": "vehicles",
};

/**
 * Quantity for a bundle item the profile cannot determine. Uses what the owner said, else a stated
 * starting point the reply must call out (devices = staff, counters = sites, cameras = 2 per site, else 1).
 */
export function proposeQuantity(unit: string, state: SessionState): { quantity: number; assumed: boolean } {
  const key = COUNT_FOR_UNIT[unit];
  if (key && state.counts[key]) return { quantity: state.counts[key]!, assumed: false };
  const sites = state.profile.numSites ?? 1;
  const n = key === "devices" ? state.profile.employeeCount ?? 1 : key === "counters" ? sites : key === "cameras" ? 2 * sites : 1;
  return { quantity: n, assumed: true };
}

function quoteFacts(q: Quote) {
  return {
    quoteId: q.id,
    monthlyTotalInr: q.monthlyTotalInr,
    oneTimeTotalInr: q.oneTimeTotalInr,
    totalContractValueInr: q.totalContractValueInr,
    contractMonths: q.contractMonths,
    expectedInstallDays: q.expectedInstallDays,
    lines: q.lines.map((l) => ({ product: l.productName, quantity: l.quantity, unit: l.unit, monthlyInr: l.monthlyInr, oneTimeInr: l.oneTimeInr, contractMonths: l.contractMonths, note: l.note })),
  };
}

function exclusionFacts(xs: Exclusion[], byId: Map<string, Product>) {
  return xs.map((x) => ({ product: x.productName, why: x.reasons.map((r) => r.message), alternatives: (x.alternatives ?? []).map((a) => byId.get(a)?.name).filter(Boolean) }));
}

function names(q: Quote | undefined, xs: Exclusion[], byId: Map<string, Product>, extra: string[] = []): string[] {
  const out = new Set(extra);
  q?.lines.forEach((l) => out.add(l.productName));
  xs.forEach((x) => { out.add(x.productName); (x.alternatives ?? []).forEach((a) => byId.get(a) && out.add(byId.get(a)!.name)); });
  return [...out];
}

const profileKey = (s: SessionState) => JSON.stringify({ p: s.profile, c: s.counts });

export interface TurnResult { reply: string; brief: Brief; state: SessionState; guardProblems: string[] }

export async function runTurn(state: SessionState, message: string, deps: Deps): Promise<TurnResult> {
  const { llm, services, emit } = deps;
  state.turn += 1;
  const catalogue = await services.products();
  const byId = new Map(catalogue.map((p) => [p.id, p]));
  const ext = await llm.extract({ history: state.history, message, profile: state.profile, catalogue: catalogue.map((p) => ({ id: p.id, name: p.name })) });
  state.history.push({ role: "owner", text: message });
  state.language = ext.language;
  // Romanised is read from the message's script, not the LLM (DeepSeek flagged Tamil script as romanised).
  // A message without letters ("6") keeps the previous setting.
  const latin = /\p{L}/u.test(message) ? dominantScript(message) === "Latin" : state.romanised;
  state.romanised = ext.language !== "English" && latin;
  if (!state.profile.preferredLanguage) state.profile = { ...state.profile, preferredLanguage: ext.language };

  const prevGstin = state.profile.gstin;
  const changed = merge(state, ext, new Set(byId.keys()));
  if (state.profile.gstin && state.profile.gstin !== prevGstin) await verifyGstin(state, services);
  if (changed.length) {
    const payload: ProfileUpdatedPayload = { sessionId: state.sessionId, profile: state.profile as Record<string, unknown>, changed };
    await emit(TOPICS.ADVISOR_PROFILE_UPDATED, state.leadId, payload);
  }

  let brief: Brief;
  try {
    brief = await decide(state, ext, byId, deps);
  } catch (e) {
    const msg = e instanceof ServiceCallError ? e.body.message : String(e);
    brief = { kind: "error", instructions: "Apologise briefly: pricing is temporarily unavailable; say you will retry and ask them to try again in a minute. Do not mention any product or price.", facts: { error: msg }, allowedProducts: [] };
  }

  const catalogueNames = catalogue.map((p) => p.name);
  const allowedNumbers = numbersInValue(brief.facts);
  let reply = "";
  let problems: string[] = [];
  for (let attempt = 0; attempt < 2; attempt++) {
    reply = await llm.write({ brief, history: state.history, language: state.language, romanised: state.romanised, feedback: attempt ? problems : undefined });
    problems = checkReply({ reply, allowedNumbers, allowedProducts: brief.allowedProducts, catalogueNames, language: state.language, romanised: state.romanised });
    if (!problems.length) break;
  }
  if (problems.length) reply = fallbackReply(brief);
  state.history.push({ role: "advisor", text: reply });
  return { reply, brief, state, guardProblems: problems };
}

async function decide(state: SessionState, ext: AdvisorExtraction, byId: Map<string, Product>, deps: Deps): Promise<Brief> {
  const { services, emit } = deps;
  const common = state.gstinNote ? { gstinNote: state.gstinNote } : {};

  if (ext.intent === "out_of_scope") {
    return { kind: "out_of_scope", allowedProducts: [], facts: { ...common, scope: "business internet, mobile lines, cloud tools, security and IoT from Accel Business" },
      instructions: "The owner asked something outside what we sell. Say politely you can only help with business connectivity and digital tools, then steer back with one short question about their business. Do not name any product or price." };
  }
  if (ext.intent === "accept" && state.quote) {
    const q = await services.acceptQuote(state.quote.id);
    state.quote = q;
    return { kind: "accepted", allowedProducts: names(q, [], byId), facts: { ...common, ...quoteFacts(q) },
      instructions: "Confirm the quote is accepted. Restate the monthly total and one-time charge, say the next step is a quick document check (KYC) on the phone, and the expected installation time in days." };
  }
  if (ext.intent === "mobile_only") {
    const lines = state.counts.lines ?? 1;
    const q = await services.createQuote({ leadId: state.leadId, profile: state.profile, items: [{ productId: "P07", quantity: lines }], contractMonths: 1 });
    await recommended(state, q, "mobile-only", "mobile_only", emit);
    return { kind: "mobile_only", allowedProducts: names(q, [], byId), facts: { ...common, ...quoteFacts(q), linesAssumed: !state.counts.lines },
      instructions: "The owner only wants a mobile plan. Give the plan and its monthly price on a 1-month contract, no bundle. If linesAssumed, say it is priced for that many lines and they can tell you more. Offer, in one short sentence, that you can also look at internet later." };
  }
  if (ext.intent === "too_expensive" && state.quote) return cheaper(state, byId, deps, common);

  const missing = REQUIRED.filter((k) => state.profile[k] === undefined);
  if (missing.length && state.turn < MAX_TURNS_BEFORE_RECOMMEND) {
    return { kind: "ask", allowedProducts: [], facts: { ...common, missing, known: state.profile },
      instructions: "Ask for ALL the missing facts in one short, friendly message (a single question or a compact list). Map fields to plain words: segment = what kind of business, cityTier = which city, employeeCount = how many staff, numSites = how many shops/branches, hasGstin = do they have a GST registration. Do not recommend anything yet." };
  }
  if (state.quote && state.quotedProfileKey === profileKey(state)) {
    return { kind: "follow_up", allowedProducts: names(state.quote, [], byId), facts: { ...common, ...quoteFacts(state.quote), ownerSaid: ext },
      instructions: "Answer the owner's message using only the quote facts. If they ask for something not in the quote, say you will check and ask them to name it. Offer to go ahead with the quote." };
  }
  return recommend(state, byId, deps, common, missing);
}

async function recommended(state: SessionState, q: Quote, bundleId: string, kind: BundleRecommendedPayload["kind"], emit: Emit) {
  state.quote = q;
  state.bundleId = bundleId;
  state.quotedProfileKey = profileKey(state);
  state.firstQuoteTurn ??= state.turn;
  const payload: BundleRecommendedPayload = { sessionId: state.sessionId, bundleId, quoteId: q.id, productIds: q.lines.map((l) => l.productId), monthlyTotalInr: q.monthlyTotalInr, kind };
  await emit(TOPICS.BUNDLE_RECOMMENDED, state.leadId, payload);
}

async function recommend(state: SessionState, byId: Map<string, Product>, deps: Deps, common: Record<string, unknown>, missing: string[]): Promise<Brief> {
  const { services, emit } = deps;
  const [bundles, el] = await Promise.all([services.bundles(state.profile), services.eligibility(state.profile)]);
  const eligible = new Set(el.eligible.map((p) => p.id));
  const bundle: Bundle | undefined = bundles.find((b) => b.id !== "mobile-only") ?? bundles[0];
  if (!bundle) throw new Error("rules returned no bundle");

  const items: QuoteItem[] = [];
  const assumptions: string[] = [];
  const add = (productId: string, unit: string, quantity: number | null) => {
    if (items.some((i) => i.productId === productId)) return;
    if (quantity === null) {
      const p = proposeQuantity(unit, state);
      if (p.assumed) assumptions.push(`${byId.get(productId)?.name}: priced for ${p.quantity} (${unit}); owner can change it`);
      items.push({ productId, quantity: p.quantity });
    } else items.push({ productId, quantity });
  };
  for (const it of bundle.items.filter((i) => !i.optional || state.profile.productsOfInterest?.includes(i.productId))) add(it.productId, it.unit, it.quantity);
  const interests = state.profile.productsOfInterest ?? [];
  for (const id of interests.filter((id) => eligible.has(id))) {
    const p = byId.get(id)!;
    const fromProfile = p.unit === "per site" ? state.profile.numSites ?? null : p.unit === "per user" ? state.profile.employeeCount ?? null
      : ["per account", "per pack", "per number"].includes(p.unit) ? 1 : null;
    add(id, p.unit, fromProfile);
  }
  if (missing.includes("numSites")) assumptions.push("number of sites not given: priced for 1 site");
  const quoteProfile = { ...state.profile, numSites: state.profile.numSites ?? 1, employeeCount: state.profile.employeeCount ?? 1 };
  const q = await services.createQuote({ leadId: state.leadId, profile: quoteProfile, items, contractMonths: 12 });
  await recommended(state, q, bundle.id, "bundle", emit);

  const excludedInterests = el.excluded.filter((x) => interests.includes(x.productId));
  const optional = bundle.items.filter((i) => i.optional && !q.lines.some((l) => l.productId === i.productId));
  return {
    kind: "recommend",
    allowedProducts: names(q, excludedInterests, byId, optional.map((o) => o.productName)),
    facts: { ...common, bundle: bundle.name, rationale: bundle.rationale, ...quoteFacts(q), optionalAddOns: optional.map((o) => ({ product: o.productName, why: o.why })),
      notAvailable: exclusionFacts(excludedInterests, byId), assumptions, quoteNotes: q.notes },
    instructions: "Recommend this bundle. Lead with ONE monthly price (monthlyTotalInr) and the one-time charge separately, then list each product in a few words with why it helps. For every item in notAvailable say plainly why it is not available and offer its alternatives. Mention assumptions and quoteNotes (automatic additions) in one line each. Mention optional add-ons in one line. Say the expected installation time in days. End by asking if they want to go ahead.",
  };
}

/** "Too expensive": cheapest eligible internet link plus the bundle's first non-connectivity item, with total cost. */
async function cheaper(state: SessionState, byId: Map<string, Product>, deps: Deps, common: Record<string, unknown>): Promise<Brief> {
  const { services, emit } = deps;
  const prev = state.quote!;
  const el = await services.eligibility(state.profile);
  const links = el.eligible.filter((p) => p.category === "Connectivity" && p.unit === "per site" && !p.requiresOneOf?.length)
    .sort((a, b) => a.priceMaxInr - b.priceMaxInr);
  const items: QuoteItem[] = links[0] ? [{ productId: links[0].id }] : [];
  const extra = prev.lines.find((l) => byId.get(l.productId)?.category !== "Connectivity" && !byId.get(l.productId)?.requiresOneOf?.length);
  if (extra) items.push({ productId: extra.productId, quantity: extra.quantity });
  const profile = { ...state.profile, numSites: state.profile.numSites ?? 1 };
  let q = await services.createQuote({ leadId: state.leadId, profile, items, contractMonths: 12 });
  if (q.monthlyTotalInr >= prev.monthlyTotalInr && items.length > 1) {
    q = await services.createQuote({ leadId: state.leadId, profile, items: items.slice(0, 1), contractMonths: 12 });
  }
  const isCheaper = q.monthlyTotalInr < prev.monthlyTotalInr;
  if (isCheaper) await recommended(state, q, "cheaper-path", "cheaper", emit);
  return {
    kind: "cheaper",
    allowedProducts: names(isCheaper ? q : prev, [], byId, prev.lines.map((l) => l.productName)),
    facts: { ...common, previousMonthlyTotalInr: prev.monthlyTotalInr, previousTotalContractValueInr: prev.totalContractValueInr, cheaperAvailable: isCheaper, ...(isCheaper ? quoteFacts(q) : {}) },
    instructions: isCheaper
      ? "The owner finds it too expensive. Offer this cheaper, smaller option: give its monthly total, one-time charge and the total cost over the contract (totalContractValueInr), and compare with the previous monthly total. Say they can add things back later. Ask if this works."
      : "The owner finds it too expensive, but this is already the most basic option. Say so kindly, restate the monthly total and the total cost over the contract, and ask what budget they had in mind.",
  };
}

/** Deterministic English reply when the writer keeps failing the guard. */
export function fallbackReply(b: Brief): string {
  const f = b.facts as Record<string, unknown> & { monthlyTotalInr?: number; oneTimeTotalInr?: number; lines?: { product: string; quantity: number }[] };
  switch (b.kind) {
    case "ask":
      return `To suggest the right plan, please tell me: ${(f.missing as string[]).join(", ")}.`;
    case "recommend": case "cheaper": case "mobile_only": case "accepted": case "follow_up":
      if (f.monthlyTotalInr === undefined) return "This is already our most basic option. What monthly budget did you have in mind?";
      return `${b.kind === "accepted" ? "Accepted. " : ""}Your plan: ${f.lines!.map((l) => `${l.product} x${l.quantity}`).join(", ")}. Monthly total ₹${f.monthlyTotalInr}, one-time ₹${f.oneTimeTotalInr}. ${b.kind === "accepted" ? "Next: a quick document check." : "Shall we go ahead?"}`;
    case "out_of_scope":
      return "I can help with business internet, mobile lines and digital tools. What kind of business do you run?";
    default:
      return "Sorry, pricing is briefly unavailable. Please try again in a minute.";
  }
}
