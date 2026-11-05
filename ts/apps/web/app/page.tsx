"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { DEMO_OWNERS, LANGUAGES } from "@udyam/shared";
import { Button, Card, LanguagePicker, Logo, Pill } from "@/components/ui";
import { IconArrow, IconBolt, IconCamera, IconChat, IconReceipt, IconShield, IconTruck } from "@/components/icons";
import { useJourney } from "@/lib/journey";
import { AccountLink } from "@/components/account-link";

/** A phone sketch in plain SVG shapes: no images to download on a slow connection. */
function PhoneArt() {
  return (
    <svg viewBox="0 0 260 400" className="h-auto w-full max-w-[250px] drop-shadow-2xl" aria-hidden="true">
      <rect x="20" y="10" width="220" height="380" rx="34" fill="#16161a" stroke="#3a3a42" strokeWidth="3" />
      <rect x="34" y="40" width="192" height="330" rx="18" fill="#f7f7f9" />
      <rect x="100" y="22" width="60" height="8" rx="4" fill="#3a3a42" />
      <rect x="48" y="58" width="120" height="34" rx="14" fill="#ececf0" />
      <rect x="92" y="104" width="120" height="44" rx="14" fill="#e40000" />
      <rect x="48" y="160" width="140" height="28" rx="14" fill="#ececf0" />
      <rect x="48" y="204" width="164" height="96" rx="16" fill="#fff" stroke="#ffc2c2" />
      <rect x="62" y="218" width="70" height="10" rx="5" fill="#b80000" />
      <rect x="62" y="238" width="110" height="18" rx="6" fill="#16161a" />
      <rect x="62" y="266" width="90" height="8" rx="4" fill="#dcdce3" />
      <rect x="62" y="280" width="120" height="8" rx="4" fill="#dcdce3" />
      <rect x="48" y="316" width="164" height="38" rx="14" fill="#e40000" />
      <circle cx="210" cy="72" r="26" fill="#e40000" opacity="0.18" />
      <circle cx="210" cy="72" r="14" fill="#e40000" />
    </svg>
  );
}

const STEP_ICONS = [IconChat, IconReceipt, IconShield, IconCamera, IconTruck];
const STEP_KEYS = ["stepChat", "stepQuote", "stepConsent", "stepKyc", "stepTrack"] as const;
const LANG_FLAG: Record<string, string> = { en: "EN", hi: "हि", bn: "বা", te: "తె", mr: "म", ta: "த", gu: "ગુ", kn: "ಕ", ml: "മ", pa: "ਪੰ" };

export default function Home() {
  const { j, update, reset, t } = useJourney();
  const router = useRouter();

  return (
    <div className="min-h-screen bg-[#f7f7f9]">
      <section className="bg-hero text-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-4">
          <Logo dark />
          <div className="flex items-center gap-2"><AccountLink /><LanguagePicker compact dark /></div>
        </div>
        <div className="mx-auto grid max-w-6xl items-center gap-8 px-4 pt-6 pb-24 md:grid-cols-[1.2fr_1fr] md:pt-10 md:pb-32">
          <div>
            <Pill tone="brand"><IconBolt size={14} /> Accel Business · for small businesses</Pill>
            <h1 className="mt-4 text-4xl leading-[1.1] font-bold tracking-tight sm:text-5xl">{t("tagline")}</h1>
            <ol className="mt-6 flex flex-wrap gap-2">
              {STEP_KEYS.map((k, i) => {
                const Ico = STEP_ICONS[i];
                return (
                  <li key={k} className="flex items-center gap-1.5 rounded-full bg-white/10 px-3 py-1.5 text-sm ring-1 ring-white/15">
                    <Ico size={16} /> {t(k)}
                  </li>
                );
              })}
            </ol>
          </div>
          <div className="hidden justify-center md:flex"><PhoneArt /></div>
        </div>
      </section>

      <main className="mx-auto -mt-16 max-w-6xl px-4 pb-12 md:-mt-24">
        <Card className="shadow-lift">
          <h2 className="mb-1 text-lg font-bold">{t("chooseLanguage")}</h2>
          <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5">
            {LANGUAGES.map((l) => {
              const on = j.lang === l.code;
              return (
                <button
                  key={l.code}
                  onClick={() => update({ lang: l.code })}
                  aria-pressed={on}
                  className={`flex min-h-14 items-center gap-3 rounded-2xl px-3 text-left transition ${on ? "bg-ink-900 text-white shadow-lift" : "bg-ink-50 text-ink-900 hover:bg-brand-50"}`}
                >
                  <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-sm font-bold ${on ? "bg-brand-600" : "bg-white text-brand-700 ring-1 ring-ink-200"}`}>{LANG_FLAG[l.code]}</span>
                  <span className="text-[15px] font-semibold">{l.native}</span>
                </button>
              );
            })}
          </div>
          <div className="mt-5 flex flex-col gap-3 sm:flex-row sm:items-center">
            <Button className="w-full sm:w-auto" onClick={() => { reset({ lang: j.lang }); router.push("/chat"); }}>
              {t("start")} <IconArrow size={18} />
            </Button>
            {j.messages.length > 0 && (
              <Link href="/chat" className="text-center text-sm font-semibold text-brand-700 hover:underline">{t("resumeWelcome")}</Link>
            )}
          </div>
        </Card>

        <section className="mt-8">
          <div className="mb-3 flex items-end justify-between gap-2">
            <div>
              <h2 className="text-lg font-bold">{t("demoOwners")}</h2>
              <p className="text-sm text-ink-500">Scenarios from docs/scenarios.md, each with a KYC specimen (K01–K12)</p>
            </div>
          </div>
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {DEMO_OWNERS.map((o) => (
              <li key={o.id}>
                <button
                  data-testid={`demo-${o.id}`}
                  onClick={() => {
                    reset({ lang: o.language, leadPrefix: o.leadPrefix, demoOwnerId: o.id, specimen: o.specimen });
                    router.push("/chat");
                  }}
                  className="group flex h-full w-full gap-3 rounded-[var(--radius-card)] bg-white p-4 text-left shadow-card ring-1 ring-ink-100 transition hover:-translate-y-0.5 hover:shadow-lift hover:ring-brand-200"
                >
                  <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-brand-50 text-lg font-bold text-brand-700">{o.title.slice(0, 1)}</span>
                  <span className="min-w-0">
                    <span className="block font-semibold text-ink-900">{o.title}</span>
                    <span className="mt-1 flex flex-wrap gap-1.5">
                      <Pill>{LANG_FLAG[o.language]} · #{o.scenario}</Pill>
                      <Pill tone="brand">KYC {o.specimen}</Pill>
                    </span>
                    <span className="mt-1.5 block text-xs text-ink-500">{o.expect}</span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </section>

        <p className="mt-10 text-center text-sm text-ink-500">
          <Link href="/admin" className="font-semibold hover:text-brand-700">Back office →</Link>
        </p>
      </main>
    </div>
  );
}
