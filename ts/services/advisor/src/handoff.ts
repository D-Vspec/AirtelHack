import type { SessionState } from "./advisor.ts";

/** Why the advisor hands a lead to a person (contracts/scoring.schema.json, lead.unclosed). */
export const HANDOFF_REASONS = ["owner_asked_for_person", "custom_request", "not_ready"] as const;
export type HandoffReason = (typeof HANDOFF_REASONS)[number];

/**
 * A short, deterministic summary for the partner: facts the conversation established plus the
 * last quote. Built from the structured profile, not written by the LLM, so it cannot invent anything.
 */
export function handoffSummary(s: SessionState, reason: HandoffReason): string {
  const p = s.profile;
  const facts = [
    p.businessType ?? p.segment,
    p.city && `in ${p.city}${p.cityTier ? ` (Tier ${p.cityTier})` : ""}`,
    p.employeeCount !== undefined && `${p.employeeCount} staff`,
    p.numSites !== undefined && `${p.numSites} site${p.numSites === 1 ? "" : "s"}`,
    p.hasGstin !== undefined && `GSTIN: ${p.hasGstin ? "yes" : "no"}`,
    p.currentInternet && `now on ${p.currentInternet}${p.currentProvider ? ` (${p.currentProvider})` : ""}`,
    p.productsOfInterest?.length && `asked about ${p.productsOfInterest.join(", ")}`,
  ].filter(Boolean);
  const quote = s.quote
    ? `Last quote ₹${s.quote.monthlyTotalInr}/month, ₹${s.quote.oneTimeTotalInr} one-time: ${s.quote.lines.map((l) => l.productName).join(", ")}.`
    : "No quote yet.";
  const why = { owner_asked_for_person: "Owner asked to talk to a person.", custom_request: "Custom request the catalogue does not cover.", not_ready: "Owner not ready to decide." }[reason];
  return `${facts.join(", ") || "Profile not captured yet"}. Language: ${s.language}. ${quote} ${why} ${s.turn} turn(s).`;
}
