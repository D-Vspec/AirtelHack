import { z } from "zod";

/** Enums come from the case data (docs/data-dictionary.md); mirrors contracts/profile.schema.json */
export const SEGMENTS = [
  "Micro retail & services",
  "Professional offices",
  "Small manufacturers",
  "Small multi-site businesses",
] as const;
export const LANGUAGE_NAMES = [
  "Bengali", "English", "Gujarati", "Hindi", "Kannada",
  "Malayalam", "Marathi", "Punjabi", "Tamil", "Telugu",
] as const;
export const TURNOVER_BANDS = ["< 40 lakh", "40 lakh - 1.5 cr", "1.5 - 5 cr", "5 - 25 cr", "25 - 100 cr"] as const;
export const CURRENT_INTERNET = ["Business broadband", "Home broadband", "Leased line", "Mixed per site", "Mobile hotspot"] as const;
export const CURRENT_PROVIDER = ["Accel", "Accel (mobile only)", "Competitor A", "Competitor B", "Local ISP"] as const;

export const BusinessProfile = z.object({
  businessName: z.string().optional(),
  businessType: z.string().optional(),
  segment: z.enum(SEGMENTS).optional(),
  city: z.string().optional(),
  cityTier: z.union([z.literal(1), z.literal(2)]).optional(),
  preferredLanguage: z.enum(LANGUAGE_NAMES).optional(),
  employeeCount: z.number().int().min(1).optional(),
  numSites: z.number().int().min(1).optional(),
  annualTurnoverBand: z.enum(TURNOVER_BANDS).optional(),
  hasGstin: z.boolean().optional(),
  gstin: z.string().optional(),
  hasUdyam: z.boolean().optional(),
  udyamNumber: z.string().optional(),
  currentInternet: z.enum(CURRENT_INTERNET).optional(),
  currentProvider: z.enum(CURRENT_PROVIDER).optional(),
  monthlyTelecomSpendInr: z.number().min(0).optional(),
  productsOfInterest: z.array(z.string().regex(/^P[0-9]{2}$/)).optional(),
}).strict();
export type BusinessProfile = z.infer<typeof BusinessProfile>;
