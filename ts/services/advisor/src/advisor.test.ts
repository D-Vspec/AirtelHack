import { test } from "node:test";
import assert from "node:assert/strict";
import type { AdvisorExtraction } from "@udyam/shared";
import { newSession, runTurn, type Brief, type Llm } from "./advisor.ts";
import { checkReply, dominantScript, numbersIn } from "./guard.ts";
import { httpServices } from "./services.ts";

test("numbersIn reads Indian grouping and native digits", () => {
  assert.deepEqual(numbersIn("₹1,00,000 and ₹2,177"), [100000, 2177]);
  assert.deepEqual(numbersIn("₹२१७७ / ₹௨௧௭௭"), [2177, 2177]);
});

test("dominantScript ignores Latin product names", () => {
  assert.equal(dominantScript("आपके लिए Biz Fibre 100 और Cloud POS & Billing सही है", ["Biz Fibre 100", "Cloud POS & Billing"]), "Devanagari");
  assert.equal(dominantScript("Ungalukku Biz Fibre 100 nalla irukkum", ["Biz Fibre 100"]), "Latin");
});

test("guard flags invented prices, unlisted products and wrong script", () => {
  const base = { allowedNumbers: new Set([2177, 7200]), allowedProducts: ["Biz Fibre 100"], catalogueNames: ["Biz Fibre 100", "Managed Firewall"], language: "English" as const, romanised: false };
  assert.deepEqual(checkReply({ ...base, reply: "Biz Fibre 100 at ₹2,177 a month, ₹7200 once." }), []);
  assert.match(checkReply({ ...base, reply: "Only ₹1999 a month!" })[0]!, /1999/);
  assert.match(checkReply({ ...base, reply: "Add a Managed Firewall." })[0]!, /Managed Firewall/);
  assert.match(checkReply({ ...base, language: "Hindi", reply: "Biz Fibre 100 is good." })[0]!, /Devanagari/);
});

// ---- Flow tests against the live rules/quote/registry services (skipped when the stack is down).

const services = httpServices({ RULES_URL: "http://localhost:8080/api/rules", QUOTE_URL: "http://localhost:8080/api/quote", REGISTRY_URL: "http://localhost:8080/api/registry" });
const live = await services.products().then(() => true, () => false);

/** Scripted extractor + a writer that states the brief's facts honestly (or lies, to test the guard). */
function fakeLlm(script: Record<string, Partial<AdvisorExtraction>>, lie = false): Llm & { briefs: Brief[] } {
  const briefs: Brief[] = [];
  return {
    briefs,
    async extract({ message }) {
      return { language: "English", romanised: false, intent: "info", ...script[message] } as AdvisorExtraction;
    },
    async write({ brief }) {
      briefs.push(brief);
      const f = brief.facts as { monthlyTotalInr?: number; lines?: { product: string }[] };
      if (lie) return "Special offer: everything for ₹499 a month!";
      if (f.monthlyTotalInr === undefined) return "Tell me about your business.";
      return `${f.lines!.map((l) => l.product).join(", ")} for ₹${f.monthlyTotalInr} a month.`;
    },
  };
}
const events: string[] = [];
const emit = async (topic: string) => { events.push(topic); };

test("clinic: asks, then recommends an eligible priced bundle within the turn budget", { skip: !live }, async () => {
  const llm = fakeLlm({
    "hi": {},
    "dental clinic in Chennai, 6 staff, one branch, we have GST": { segment: "Professional offices", businessType: "Dental clinic", city: "Chennai", employeeCount: 6, numSites: 1, hasGstin: true },
  });
  const s = newSession("t-clinic");
  const r1 = await runTurn(s, "hi", { llm, services, emit });
  assert.equal(r1.brief.kind, "ask");
  const r2 = await runTurn(s, "dental clinic in Chennai, 6 staff, one branch, we have GST", { llm, services, emit });
  assert.equal(r2.brief.kind, "recommend");
  assert.equal(s.profile.cityTier, 1, "city tier comes from the data, not the LLM");
  const q = await services.getQuote(s.quote!.id);
  assert.equal(q.monthlyTotalInr, s.quote!.monthlyTotalInr);
  assert.ok(r2.reply.includes(String(q.monthlyTotalInr)));
  const el = await services.eligibility(s.profile);
  for (const l of q.lines) assert.ok(el.eligible.some((p) => p.id === l.productId), `${l.productId} eligible`);
  assert.ok(q.lines.some((l) => l.productId === "P16" && l.quantity === 6), "email for every staff member");
  assert.ok(events.includes("advisor.profile.updated") && events.includes("bundle.recommended"));
});

test("CA firm asking for a firewall gets it explained, not quoted", { skip: !live }, async () => {
  const llm = fakeLlm({ "CA firm Mumbai 8 people 1 office GST yes, need a firewall": { segment: "Professional offices", city: "Mumbai", employeeCount: 8, numSites: 1, hasGstin: true, productsOfInterest: ["P21"] } });
  const s = newSession("t-ca");
  const r = await runTurn(s, "CA firm Mumbai 8 people 1 office GST yes, need a firewall", { llm, services, emit });
  assert.ok(!s.quote!.lines.some((l) => l.productId === "P21"));
  const na = (r.brief.facts.notAvailable as { product: string; why: string[] }[])[0]!;
  assert.equal(na.product, "Managed Firewall");
  assert.match(na.why[0]!, /10 employees/);
});

test("mobile only, then too expensive, then accept", { skip: !live }, async () => {
  const llm = fakeLlm({
    "just a mobile plan for 2 lines": { intent: "mobile_only", lines: 2 },
    "kirana in Patna, 1 shop, 2 people, no GST": { segment: "Micro retail & services", city: "Patna", numSites: 1, employeeCount: 2, hasGstin: false },
    "too expensive": { intent: "too_expensive" },
    "ok go ahead": { intent: "accept" },
  });
  const s = newSession("t-mobile");
  await runTurn(s, "just a mobile plan for 2 lines", { llm, services, emit });
  assert.deepEqual(s.quote!.lines.map((l) => [l.productId, l.quantity, l.contractMonths]), [["P07", 2, 1]]);
  await runTurn(s, "kirana in Patna, 1 shop, 2 people, no GST", { llm, services, emit });
  const full = s.quote!;
  assert.ok(!full.lines.some((l) => ["P15", "P14"].includes(l.productId)), "no GSTIN products");
  const r = await runTurn(s, "too expensive", { llm, services, emit });
  assert.equal(r.brief.kind, "cheaper");
  assert.ok(s.quote!.monthlyTotalInr < full.monthlyTotalInr);
  assert.ok(typeof r.brief.facts.totalContractValueInr === "number");
  await runTurn(s, "ok go ahead", { llm, services, emit });
  assert.equal((await services.getQuote(s.quote!.id)).status, "accepted");
});

test("cancelled GSTIN (K01) means no GSTIN products", { skip: !live }, async () => {
  const llm = fakeLlm({ "travel agency Visakhapatnam 12 staff 1 office GSTIN 37IHKXV5167L1ZS": { segment: "Professional offices", city: "Visakhapatnam", employeeCount: 12, numSites: 1, hasGstin: true, gstin: "37IHKXV5167L1ZS" } });
  const s = newSession("t-k01");
  const r = await runTurn(s, "travel agency Visakhapatnam 12 staff 1 office GSTIN 37IHKXV5167L1ZS", { llm, services, emit });
  assert.equal(s.profile.hasGstin, false);
  assert.match(String(r.brief.facts.gstinNote), /Cancelled/);
});

test("a writer that invents a price is replaced by the deterministic reply", { skip: !live }, async () => {
  const llm = fakeLlm({ "salon Pune 3 staff 1 shop no GST": { segment: "Micro retail & services", city: "Pune", employeeCount: 3, numSites: 1, hasGstin: false } }, true);
  const s = newSession("t-lie");
  const r = await runTurn(s, "salon Pune 3 staff 1 shop no GST", { llm, services, emit });
  assert.ok(r.guardProblems.length > 0);
  assert.ok(!r.reply.includes("499"));
  assert.ok(r.reply.includes(String(s.quote!.monthlyTotalInr)));
});

test("out-of-scope question gets no products", { skip: !live }, async () => {
  const llm = fakeLlm({ "can you file my income tax?": { intent: "out_of_scope" } });
  const s = newSession("t-oos");
  const r = await runTurn(s, "can you file my income tax?", { llm, services, emit });
  assert.equal(r.brief.kind, "out_of_scope");
  assert.equal(s.quote, undefined);
});

test("romanised comes from the message script, not the extractor", { skip: !live }, async () => {
  const llm = fakeLlm({ "வணக்கம், சென்னையில் பல் மருத்துவமனை": { language: "Tamil", romanised: true }, "vanakkam, chennai-la clinic": { language: "Tamil", romanised: false } });
  const s = newSession("t-script");
  await runTurn(s, "வணக்கம், சென்னையில் பல் மருத்துவமனை", { llm, services, emit });
  assert.equal(s.romanised, false);
  await runTurn(s, "vanakkam, chennai-la clinic", { llm, services, emit });
  assert.equal(s.romanised, true);
});
