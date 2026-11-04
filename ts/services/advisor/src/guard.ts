import type { LanguageName } from "@udyam/shared";

// Unicode digit blocks for the scripts we reply in; mapped to ASCII before checking numbers.
const DIGIT_ZEROS = [0x0966, 0x09e6, 0x0a66, 0x0ae6, 0x0be6, 0x0c66, 0x0ce6, 0x0d66];

export function asciiDigits(s: string): string {
  return s.replace(/[०-९০-৯੦-੯૦-૯௦-௯౦-౯೦-೯൦-൯]/g, (ch) => {
    const c = ch.codePointAt(0)!;
    const zero = DIGIT_ZEROS.find((z) => c >= z && c <= z + 9)!;
    return String(c - zero);
  });
}

/** Integers in a text, reading Indian/Western grouping (2,177 / 1,00,000) and native digits. */
export function numbersIn(text: string): number[] {
  const t = asciiDigits(text);
  return [...t.matchAll(/\d[\d,]*(?:\.\d+)?/g)].map((m) => Number(m[0].replace(/,/g, ""))).filter((n) => Number.isFinite(n));
}

const SCRIPTS: Record<string, RegExp> = {
  Devanagari: /[ऀ-ॿ]/g, Bengali: /[ঀ-৿]/g, Gurmukhi: /[਀-੿]/g, Gujarati: /[઀-૿]/g,
  Tamil: /[஀-௿]/g, Telugu: /[ఀ-౿]/g, Kannada: /[ಀ-೿]/g, Malayalam: /[ഀ-ൿ]/g, Latin: /[A-Za-z]/g,
};

export const LANGUAGE_SCRIPT: Record<LanguageName, string> = {
  English: "Latin", Hindi: "Devanagari", Marathi: "Devanagari", Bengali: "Bengali", Punjabi: "Gurmukhi", Gujarati: "Gujarati",
  Tamil: "Tamil", Telugu: "Telugu", Kannada: "Kannada", Malayalam: "Malayalam",
};

/** Dominant script by letter count, ignoring product names and digits (Latin brand names are expected). */
export function dominantScript(text: string, ignore: string[] = []): string {
  let t = text;
  for (const s of ignore) t = t.split(s).join(" ");
  t = t.replace(/₹|Rs\.?|INR|GST(IN)?|UPI|Wi-?Fi|SMS|CCTV|IP|SD-WAN|5G|4G|1G|IoT|POS|IVR/gi, " ");
  let best = "Latin";
  let max = 0;
  for (const [name, re] of Object.entries(SCRIPTS)) {
    const n = (t.match(re) ?? []).length;
    if (n > max) [best, max] = [name, n];
  }
  return best;
}

export function expectedScript(language: LanguageName, romanised: boolean): string {
  return romanised ? "Latin" : LANGUAGE_SCRIPT[language];
}

export interface GuardInput {
  reply: string;
  allowedNumbers: Set<number>;
  allowedProducts: string[];
  catalogueNames: string[];
  language: LanguageName;
  romanised: boolean;
}

/**
 * Checks a drafted reply against the brief it was written from. Returns problems; empty = ok.
 * Small numbers (<= 10) are free: counts, list numbering. Every other number must come from the brief.
 */
export function checkReply(g: GuardInput): string[] {
  const problems: string[] = [];
  for (const n of numbersIn(g.reply)) {
    if (n > 10 && !g.allowedNumbers.has(n)) problems.push(`number ${n} is not in the brief`);
  }
  const lower = g.reply.toLowerCase();
  for (const name of g.catalogueNames) {
    if (lower.includes(name.toLowerCase()) && !g.allowedProducts.includes(name)) problems.push(`product "${name}" is not in the brief`);
  }
  const want = expectedScript(g.language, g.romanised);
  const got = dominantScript(g.reply, g.catalogueNames);
  if (got !== want) problems.push(`reply is in ${got} script, expected ${want} (${g.language}${g.romanised ? ", romanised" : ""})`);
  return problems;
}

/** Every number that appears anywhere in a JSON-able value (prices, counts, days). */
export function numbersInValue(v: unknown, out = new Set<number>()): Set<number> {
  if (typeof v === "number") out.add(v);
  else if (typeof v === "string") numbersIn(v).forEach((n) => out.add(n));
  else if (Array.isArray(v)) v.forEach((x) => numbersInValue(x, out));
  else if (v && typeof v === "object") Object.values(v).forEach((x) => numbersInValue(x, out));
  return out;
}
