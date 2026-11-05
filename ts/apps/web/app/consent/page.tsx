"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { KycPurpose } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Pill, Shell, Spinner, Toggle } from "@/components/ui";
import { IconBack, IconCamera, IconLock, IconMessage, IconShield, IconTruck } from "@/components/icons";
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

  const items: { key: KycPurpose; label: string; icon: React.ReactNode; required?: boolean }[] = [
    { key: "kyc_verification", label: t("consentKyc"), icon: <IconCamera size={18} />, required: true },
    { key: "service_provisioning", label: t("consentProvision"), icon: <IconTruck size={18} /> },
    { key: "communication", label: t("consentComms"), icon: <IconMessage size={18} /> },
  ];

  return (
    <Shell step={2} title={t("consentTitle")}>
      <div className="grid max-w-4xl gap-4 lg:grid-cols-[1fr_280px]">
        <Card>
          <div className="flex items-start gap-3">
            <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-ink-900 text-white"><IconShield size={24} /></span>
            <p className="text-[15.5px] text-ink-700">{t("consentBody")}</p>
          </div>
          <ul className="mt-5 space-y-2">
            {items.map((i) => (
              <li key={i.key} className="flex min-h-14 items-center gap-3 rounded-2xl bg-ink-50 p-3 ring-1 ring-ink-100">
                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-brand-700 ring-1 ring-ink-200">{i.icon}</span>
                <span className="min-w-0 flex-1 text-[15px]">{i.label}</span>
                {i.required ? (
                  <Pill tone="ink"><IconLock size={12} /> on</Pill>
                ) : (
                  <Toggle checked={purposes[i.key]} onChange={(v) => setPurposes({ ...purposes, [i.key]: v })} label={i.label} />
                )}
              </li>
            ))}
          </ul>
          {error && <div className="mt-3"><Alert tone="error">{error}</Alert></div>}
        </Card>
        <aside>
          <Card className="bg-ink-900 text-white ring-0">
            <p className="flex items-center gap-2 font-semibold"><IconLock size={18} /> DPDP Act 2023</p>
            <p className="mt-3 text-sm text-white/80">{t("withdraw")}</p>
            <p className="mt-4 text-xs text-white/50">v{CONSENT_VERSION}</p>
          </Card>
        </aside>
      </div>
      <ActionBar>
        <Button variant="secondary" onClick={() => router.push("/quote")}><IconBack size={18} /> {t("stepQuote")}</Button>
        <Button data-testid="agree" disabled={busy} onClick={agree}>{busy ? <Spinner /> : <IconShield size={18} />}{t("agree")}</Button>
      </ActionBar>
    </Shell>
  );
}
