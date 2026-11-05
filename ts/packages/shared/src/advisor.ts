import { z } from "zod";
import { CURRENT_INTERNET, CURRENT_PROVIDER, LANGUAGE_NAMES, SEGMENTS, TURNOVER_BANDS } from "./profile";

/** City -> tier, from data/smb_leads.csv (every city in the case data has exactly one tier). */
export const CITY_TIERS: Record<string, 1 | 2> = {
  Ahmedabad: 1, Bengaluru: 1, Chennai: 1, Delhi: 1, Hyderabad: 1, Kolkata: 1, Mumbai: 1, Pune: 1,
  Bhopal: 2, Coimbatore: 2, Indore: 2, Jaipur: 2, Kochi: 2, Lucknow: 2, Ludhiana: 2, Mysuru: 2,
  Nagpur: 2, Nashik: 2, Patna: 2, Surat: 2, Vadodara: 2, Visakhapatnam: 2,
};

/** Business types per segment in the case data; shown to the extractor so it can pick a segment. */
export const SEGMENT_EXAMPLES: Record<(typeof SEGMENTS)[number], string> = {
  "Micro retail & services": "kirana, stationery, salon, boutique, mobile repair, bakery, sweet shop, hardware, pharmacy, optician (single small shop)",
  "Professional offices": "law office, coaching centre, travel agency, diagnostic lab, architecture studio, dental or physio clinic, real estate, CA firm, insurance agency",
  "Small multi-site businesses": "chains with several outlets: hotels, electronics, pharmacy, apparel, bakery, restaurant, pre-school, gym",
  "Small manufacturers": "plastic moulding, packaging, textile, auto parts, printing press, furniture workshop, food processing",
};

export const INTENTS = ["info", "mobile_only", "too_expensive", "accept", "out_of_scope"] as const;
export type Intent = (typeof INTENTS)[number];

/**
 * What the LLM extracts from one owner message. Flat, optional fields so every provider's
 * structured-output mode accepts it. Code validates and merges it into a BusinessProfile.
 */
export const AdvisorExtraction = z.object({
  businessName: z.string().optional(),
  businessType: z.string().optional().describe("Free text, e.g. Kirana store, Dental clinic"),
  segment: z.enum(SEGMENTS).optional(),
  city: z.string().optional().describe("City name in English, e.g. Chennai"),
  cityTier: z.number().int().optional().describe("1 or 2; only if the owner says it or the city is well known"),
  employeeCount: z.number().int().optional(),
  numSites: z.number().int().optional().describe("Number of shops/branches/sites"),
  annualTurnoverBand: z.enum(TURNOVER_BANDS).optional(),
  hasGstin: z.boolean().optional(),
  gstin: z.string().optional().describe("The 15-character GSTIN if the owner typed one"),
  hasUdyam: z.boolean().optional(),
  currentInternet: z.enum(CURRENT_INTERNET).optional(),
  currentProvider: z.enum(CURRENT_PROVIDER).optional(),
  monthlyTelecomSpendInr: z.number().optional(),
  productsOfInterest: z.array(z.string()).optional().describe("Catalogue product ids (P01-P25) the owner asked for"),
  lines: z.number().int().optional().describe("Mobile lines wanted"),
  cameras: z.number().int().optional(),
  devices: z.number().int().optional().describe("PCs, laptops and phones to protect with Endpoint Security (never factory machines)"),
  machines: z.number().int().optional().describe("Factory machines/equipment to fit with Machine Monitoring IoT sensors ('10 machines')"),
  users: z.number().int().optional().describe("People who need per-user cloud tools (email/office seats) when the owner gives a number different from total staff"),
  counters: z.number().int().optional().describe("Billing counters"),
  vehicles: z.number().int().optional(),
  language: z.enum(LANGUAGE_NAMES).describe("Language of the owner's latest message"),
  romanised: z.boolean().describe("True if a non-English language is written in Latin letters (Hinglish, Tanglish)"),
  intent: z.enum(INTENTS).describe(
    "mobile_only: only wants a mobile plan; too_expensive: price objection; accept: agrees to go ahead with the quote; out_of_scope: asks something unrelated to telecom/IT for the business; info: anything else",
  ),
});
export type AdvisorExtraction = z.infer<typeof AdvisorExtraction>;

// Shapes returned by the rules and quote services (contracts/rules-quote.schema.json).
export interface Product {
  id: string; category: string; name: string; description: string; unit: string;
  priceMinInr: number; priceMaxInr: number; setupFeeInr: number; minContractMonths: number;
  cityTiers: number[]; minEmployees: number; minSites: number; requiresGstin: boolean;
  requiresOneOf?: string[]; installDays: number; segments: string[];
}
export interface Reason { code: string; message: string }
export interface Exclusion { productId: string; productName: string; reasons: Reason[]; alternatives?: string[] }
export interface Eligibility { eligible: Product[]; excluded: Exclusion[] }
export interface BundleItem { productId: string; productName: string; unit: string; quantity: number | null; optional: boolean; why: string; addedFor?: string }
export interface Bundle { id: string; name: string; segment: string; rationale: string; items: BundleItem[]; omitted?: Exclusion[] }
export interface QuoteItem { productId: string; quantity?: number }
export interface QuoteLine {
  productId: string; productName: string; unit: string; quantity: number; unitPriceInr: number;
  monthlyInr: number; oneTimeInr: number; contractMonths: number; installDays: number; addedFor?: string; note?: string;
}
export interface Quote {
  id: string; leadId?: string; status: "draft" | "accepted"; contractMonths: number; lines: QuoteLine[];
  monthlyTotalInr: number; oneTimeTotalInr: number; totalContractValueInr: number; expectedInstallDays: number;
  notes: string[]; createdAt: string; acceptedAt?: string;
}
export interface ServiceError { code: string; message: string; excluded?: Exclusion[] }

/** advisor.profile.updated payload */
export interface ProfileUpdatedPayload { sessionId: string; profile: Record<string, unknown>; changed: string[] }
/** bundle.recommended payload */
export interface BundleRecommendedPayload {
  sessionId: string; bundleId: string; quoteId: string; productIds: string[]; monthlyTotalInr: number; kind: "bundle" | "mobile_only" | "cheaper";
}
