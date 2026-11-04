"use client";

import { KYC_MESSAGES, type KycFinding } from "@udyam/shared";
import { Alert } from "@/components/ui";

/** One KYC finding in the owner's language: what is wrong and the next step (contracts/kyc-messages.json). */
export function FindingItem({ f, lang }: { f: KycFinding; lang: string }) {
  const m = KYC_MESSAGES[f.code]?.[lang as "en"] ?? KYC_MESSAGES[f.code]?.en;
  const tone = f.route === "pass" ? "info" : f.route === "reject" ? "error" : f.route === "manual_review" ? "info" : "warning";
  return (
    <Alert tone={tone}>
      <p className="font-medium">{m?.message ?? f.code}</p>
      {m?.action && <p className="text-sm">→ {m.action}</p>}
    </Alert>
  );
}

