import type { LANGUAGE_NAMES } from "./profile";

export type LanguageName = (typeof LANGUAGE_NAMES)[number];

/** Languages present in smb_leads.preferred_language. Phase 6 adds UI strings per language. */
export const LANGUAGES: { name: LanguageName; code: string; native: string }[] = [
  { name: "English", code: "en", native: "English" },
  { name: "Hindi", code: "hi", native: "हिन्दी" },
  { name: "Bengali", code: "bn", native: "বাংলা" },
  { name: "Telugu", code: "te", native: "తెలుగు" },
  { name: "Marathi", code: "mr", native: "मराठी" },
  { name: "Tamil", code: "ta", native: "தமிழ்" },
  { name: "Gujarati", code: "gu", native: "ગુજરાતી" },
  { name: "Kannada", code: "kn", native: "ಕನ್ನಡ" },
  { name: "Malayalam", code: "ml", native: "മലയാളം" },
  { name: "Punjabi", code: "pa", native: "ਪੰਜਾਬੀ" },
];

/** English source strings; other languages are filled in Phase 6 (keep keys stable). */
export const STRINGS_EN = {
  appName: "Udyam Digital",
  tagline: "The right connectivity bundle for your business, in minutes.",
  checking: "Checking services...",
} as const;
