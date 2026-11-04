import type { LangCode } from "./i18n";

/**
 * Demo mode: sample owners from docs/scenarios.md. Each opens the advisor with a first message
 * and, at the KYC step, can load one of the case's specimen applicants (K01-K12) so the demo shows
 * a pass, a manual review, a re-upload or a rejection without real documents.
 */
export interface DemoOwner {
  id: string;
  scenario: number;
  title: string;
  language: LangCode;
  /** lead ID prefix; "nofibre" makes the demo simulator take the fibre-not-feasible branch */
  leadPrefix: string;
  firstMessage: string;
  specimen: string;
  expect: string;
}

export const DEMO_OWNERS: DemoOwner[] = [
  {
    id: "clinic-tamil", scenario: 2, title: "Dental clinic, Chennai (Tamil)", language: "ta", leadPrefix: "demo-clinic",
    firstMessage: "வணக்கம், நான் சென்னையில் ஒரு பல் மருத்துவமனை நடத்துகிறேன். 6 பேர் வேலை செய்கிறார்கள், ஒரே கிளை, GSTIN உள்ளது.",
    specimen: "K03", expect: "Office bundle; KYC passes; order activates",
  },
  {
    id: "kirana-hindi", scenario: 1, title: "Kirana store, Lucknow (Hindi, no GSTIN)", language: "hi", leadPrefix: "demo-kirana",
    firstMessage: "नमस्ते, मेरी लखनऊ में किराना दुकान है, 2 लोग काम करते हैं, एक ही दुकान है, GSTIN नहीं है। UPI बिलिंग और इंटरनेट चाहिए।",
    specimen: "K11", expect: "GSTIN-free bundle; KYC asks for a fresh bill (stale address proof)",
  },
  {
    id: "ca-firewall", scenario: 3, title: "CA firm asks for a firewall (English)", language: "en", leadPrefix: "demo-ca",
    firstMessage: "Hi, we are a CA firm in Pune with 8 employees, one office, we have GSTIN. We want a managed firewall and fast internet.",
    specimen: "K07", expect: "Firewall excluded with a reason; KYC goes to manual review (name mismatch)",
  },
  {
    id: "restaurant-chain", scenario: 5, title: "Restaurant chain, 5 sites, Bengaluru", language: "en", leadPrefix: "demo-chain",
    firstMessage: "We run 5 restaurants in Bengaluru, 40 staff in total, GSTIN registered. Need internet at every outlet, CCTV and billing.",
    specimen: "K12", expect: "Per-site bundle with one bill; KYC passes",
  },
  {
    id: "mobile-only", scenario: 8, title: "Just a mobile plan (Hinglish)", language: "hi", leadPrefix: "demo-mobile",
    firstMessage: "Bhai mujhe sirf 2 mobile connection chahiye business ke liye, Indore mein shop hai, GSTIN hai.",
    specimen: "K01", expect: "Mobile-only quick path; KYC rejected (GSTIN cancelled)",
  },
  {
    id: "no-fibre", scenario: 19, title: "Physio clinic where fibre is not feasible", language: "en", leadPrefix: "demo-nofibre",
    firstMessage: "I run a physiotherapy clinic in Hyderabad, 5 staff, one branch, we have GSTIN. Need reliable internet and email.",
    specimen: "K09", expect: "Installation delayed; 5G AirFiber offered and accepted",
  },
];

/** A fresh lead ID for the web or mobile journey. */
export function newLeadId(prefix = "web"): string {
  const rand = Math.random().toString(36).slice(2, 10);
  return `${prefix}-${rand}`;
}
