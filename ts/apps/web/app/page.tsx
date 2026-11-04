"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { DEMO_OWNERS, LANGUAGES } from "@udyam/shared";
import { Button, Card } from "@/components/ui";
import { useJourney } from "@/lib/journey";

export default function Home() {
  const { j, update, reset, t } = useJourney();
  const router = useRouter();

  return (
    <main className="mx-auto max-w-3xl px-4 py-6 sm:py-10">
      <header className="mb-6">
        <p className="text-sm font-semibold uppercase tracking-wide text-brand-600">Accel Business</p>
        <h1 className="text-3xl font-bold text-brand-900 sm:text-4xl">{t("appName")}</h1>
        <p className="mt-1 text-lg text-gray-700">{t("tagline")}</p>
      </header>

      <Card>
        <h2 className="mb-3 font-semibold">{t("chooseLanguage")}</h2>
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
          {LANGUAGES.map((l) => (
            <button
              key={l.code}
              onClick={() => update({ lang: l.code })}
              aria-pressed={j.lang === l.code}
              className={`min-h-12 rounded-xl px-3 text-base ring-1 ${j.lang === l.code ? "bg-brand-600 font-semibold text-white ring-brand-600" : "bg-white ring-gray-300 hover:bg-brand-50"}`}
            >
              {l.native}
            </button>
          ))}
        </div>
        <Button
          className="mt-4 w-full sm:w-auto"
          onClick={() => {
            reset({ lang: j.lang });
            router.push("/chat");
          }}
        >
          {t("start")} →
        </Button>
        {j.messages.length > 0 && (
          <Link href="/chat" className="mt-3 block text-sm text-brand-700 underline sm:ml-4 sm:inline">
            {t("resumeWelcome")}
          </Link>
        )}
      </Card>

      <Card className="mt-6">
        <h2 className="mb-1 font-semibold">{t("demoOwners")}</h2>
        <p className="mb-3 text-sm text-gray-600">Scenarios from docs/scenarios.md, each with a KYC specimen (K01-K12).</p>
        <ul className="grid gap-2 sm:grid-cols-2">
          {DEMO_OWNERS.map((o) => (
            <li key={o.id}>
              <button
                data-testid={`demo-${o.id}`}
                onClick={() => {
                  reset({ lang: o.language, leadPrefix: o.leadPrefix, demoOwnerId: o.id, specimen: o.specimen });
                  router.push("/chat");
                }}
                className="h-full w-full rounded-xl p-3 text-left ring-1 ring-gray-200 hover:bg-brand-50 hover:ring-brand-600"
              >
                <span className="block font-medium">{o.title}</span>
                <span className="block text-xs text-gray-600">#{o.scenario} · KYC {o.specimen} · {o.expect}</span>
              </button>
            </li>
          ))}
        </ul>
      </Card>

      <p className="mt-6 text-center text-sm text-gray-500">
        <Link href="/admin" className="underline">Back office</Link>
      </p>
    </main>
  );
}
