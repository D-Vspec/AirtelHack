import { SEGMENT_EXAMPLES, type BusinessProfile } from "@udyam/shared";
import type { Brief, Turn } from "./advisor.ts";

// Identical prompts for every provider (LLM_PROVIDER only changes the model).

const recent = (h: Turn[], n = 6) => h.slice(-n).map((t) => `${t.role === "owner" ? "Owner" : "Advisor"}: ${t.text}`).join("\n") || "(start of conversation)";

export function extractionPrompt(input: { history: Turn[]; message: string; profile: BusinessProfile; catalogue: { id: string; name: string }[] }) {
  const system = [
    "You extract facts from a small-business owner's chat message for a telecom advisor in India.",
    "Return only facts the owner stated or clearly implied in the latest message (use the conversation for context, e.g. a bare number answering a question). Leave a field out when unknown. Never guess numbers.",
    "Segments (pick one when the business type makes it clear):",
    ...Object.entries(SEGMENT_EXAMPLES).map(([s, ex]) => `- ${s}: ${ex}`),
    "A business with several outlets or branches is 'Small multi-site businesses'.",
    "numSites: a single shop, clinic, office or unit ('ek dukaan', 'my shop', 'oru kadai') means numSites 1.",
    "productsOfInterest: map what the owner asks for to these catalogue ids only:",
    input.catalogue.map((p) => `${p.id} ${p.name}`).join("; "),
    "users: when the owner says only some people need cloud email/office tools ('only 5 people on cloud'), the number of such people. devices/cameras/lines/counters: the counts they ask for, including changes to an earlier quote.",
    "hasGstin: true if they say they have GST/GSTIN registration, false if they say they do not.",
    "language: the language of the latest message (Hinglish -> Hindi, Tanglish -> Tamil, with romanised=true). English -> English, romanised=false.",
    "intent: mobile_only only if they want just a mobile/SIM plan and nothing else; too_expensive for price objections; accept when they agree to proceed (yes, go ahead, theek hai, sari); out_of_scope for questions unrelated to business telecom/IT (tax filing, loans, weather, cricket); otherwise info.",
  ].join("\n");
  const human = `Known profile so far: ${JSON.stringify(input.profile)}\n\nConversation:\n${recent(input.history)}\n\nLatest owner message:\n${input.message}`;
  return { system, human };
}

export function writerPrompt(input: { brief: Brief; history: Turn[]; language: string; romanised: boolean; feedback?: string[] }) {
  const lang = input.romanised
    ? `${input.language} written in Latin letters, code-mixed the way the owner writes (e.g. Hinglish/Tanglish). Also offer once, in one short line, to switch to ${input.language} script or English.`
    : input.language === "English" ? "simple English" : `${input.language}, in its native script`;
  const system = [
    "You are the Accel Business advisor, chatting with a small-business owner on their phone.",
    `Write your next message in ${lang}. Keep product names in English as given.`,
    "Use ONLY the facts in BRIEF. Never mention a product, price, discount, offer or number that is not in BRIEF. Do not invent installation dates; use expectedInstallDays as a number of days.",
    "Write every amount with the ₹ sign and ASCII digits exactly as in BRIEF (for example ₹2177). Short sentences, no markdown tables, at most 120 words.",
  ].join("\n");
  const human = [
    `TASK: ${input.brief.instructions}`,
    `BRIEF: ${JSON.stringify(input.brief.facts)}`,
    `Conversation so far:\n${recent(input.history)}`,
    input.feedback?.length ? `Your previous draft broke these rules, fix them: ${input.feedback.join("; ")}` : "",
  ].filter(Boolean).join("\n\n");
  return { system, human };
}
