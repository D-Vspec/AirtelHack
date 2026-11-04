"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { KycPurpose } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Shell, Spinner } from "@/components/ui";
import { api, errorText, useJourney } from "@/lib/journey";

// Version of the consent text shown; stored with the consent record (contracts/kyc.schema.json).
const CONSENT_VERSION = "2026-07-v1";

export default function ConsentPage() {
  const { j, update, t } = useJourney();
  const router = useRouter();
  const [purposes, setPurposes] = useState<Record<KycPurpose, boolean>>({ kyc_verification: true, service_provisioning: true, communication: true });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function agree() {
    setBusy(true);
    setError(null);
    try {
      await api.kyc.consent({
        leadId: j.leadId, version: CONSENT_VERSION, language: j.lang,
        purposes: (Object.keys(purposes) as KycPurpose[]).filter((p) => purposes[p]),
      });
      update({ consented: true });
      router.push("/kyc");
    } catch (e) {
      setError(errorText(e, t("error")));
      setBusy(false);
    }
  }

  const items: { key: KycPurpose; label: string; required?: boolean }[] = [
    { key: "kyc_verification", label: t("consentKyc"), required: true },
    { key: "service_provisioning", label: t("consentProvision") },
    { key: "communication", label: t("consentComms") },
  ];

  return (
    <Shell step={2} title={t("consentTitle")}>
      <Card className="max-w-2xl">
        <p className="text-gray-800">{t("consentBody")}</p>
        <ul className="mt-4 space-y-2">
          {items.map((i) => (
            <li key={i.key}>
              <label className="flex min-h-12 items-start gap-3 rounded-xl p-3 ring-1 ring-gray-200">
                <input
                  type="checkbox"
                  className="mt-1 h-5 w-5 accent-brand-600"
                  checked={purposes[i.key]}
                  disabled={i.required}
                  onChange={(e) => setPurposes({ ...purposes, [i.key]: e.target.checked })}
                />
                <span>{i.label}</span>
              </label>
            </li>
          ))}
        </ul>
        <p className="mt-3 text-xs text-gray-500">v{CONSENT_VERSION}</p>
        {error && <div className="mt-3"><Alert tone="error">{error}</Alert></div>}
      </Card>
      <ActionBar>
        <Button variant="secondary" onClick={() => router.push("/quote")}>← {t("stepQuote")}</Button>
        <Button data-testid="agree" disabled={busy} onClick={agree}>{busy && <Spinner />}{t("agree")}</Button>
      </ActionBar>
    </Shell>
  );
}
