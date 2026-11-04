"use client";

import Link from "next/link";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { LANGUAGES, type LangCode, type StringKey } from "@udyam/shared";
import { useJourney } from "@/lib/journey";

export function Button({ variant = "primary", className = "", ...p }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "ghost" | "danger" }) {
  const styles = {
    primary: "bg-brand-600 text-white hover:bg-brand-700 disabled:bg-gray-300",
    secondary: "bg-white text-brand-700 ring-1 ring-brand-600 hover:bg-brand-50 disabled:text-gray-400 disabled:ring-gray-300",
    ghost: "text-brand-700 hover:bg-brand-50",
    danger: "bg-white text-red-700 ring-1 ring-red-300 hover:bg-red-50",
  }[variant];
  // min-h-12: thumb-sized tap targets on phones
  return <button {...p} className={`inline-flex min-h-12 items-center justify-center gap-2 rounded-xl px-5 text-base font-semibold disabled:cursor-not-allowed ${styles} ${className}`} />;
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <section className={`rounded-2xl bg-white p-4 shadow-sm ring-1 ring-gray-200 sm:p-6 ${className}`}>{children}</section>;
}

export function Spinner({ label }: { label?: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-gray-600" role="status">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-brand-600 border-t-transparent" />
      {label}
    </span>
  );
}

export function Alert({ tone = "info", children }: { tone?: "info" | "success" | "warning" | "error"; children: ReactNode }) {
  const styles = {
    info: "bg-brand-50 text-brand-900 ring-brand-100",
    success: "bg-green-50 text-green-900 ring-green-200",
    warning: "bg-amber-50 text-amber-900 ring-amber-200",
    error: "bg-red-50 text-red-900 ring-red-200",
  }[tone];
  return <div className={`rounded-xl p-3 ring-1 ${styles}`} role={tone === "error" ? "alert" : undefined}>{children}</div>;
}

/** The main action of a step: pinned to the bottom of the screen on phones, inline on larger screens. */
export function ActionBar({ children }: { children: ReactNode }) {
  return (
    <div className="fixed inset-x-0 bottom-0 z-20 border-t border-gray-200 bg-white/95 px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] backdrop-blur md:static md:mt-6 md:border-0 md:bg-transparent md:p-0">
      <div className="mx-auto flex max-w-3xl flex-col gap-2 sm:flex-row sm:justify-end">{children}</div>
    </div>
  );
}

export function LanguagePicker({ compact = false }: { compact?: boolean }) {
  const { j, update, t } = useJourney();
  return (
    <label className="flex items-center gap-2 text-sm">
      {!compact && <span className="text-gray-600">{t("chooseLanguage")}</span>}
      <select
        aria-label={t("chooseLanguage")}
        value={j.lang}
        onChange={(e) => update({ lang: e.target.value as LangCode })}
        className="min-h-10 rounded-lg border border-gray-300 bg-white px-2 text-base"
      >
        {LANGUAGES.map((l) => (
          <option key={l.code} value={l.code}>{l.native}</option>
        ))}
      </select>
    </label>
  );
}

export const STEPS: { key: StringKey; href: string }[] = [
  { key: "stepChat", href: "/chat" },
  { key: "stepQuote", href: "/quote" },
  { key: "stepConsent", href: "/consent" },
  { key: "stepKyc", href: "/kyc" },
  { key: "stepTrack", href: "/track" },
];

/** Page frame for the owner journey. `step` is the index into STEPS. */
export function Shell({ step, title, children }: { step: number; title: string; children: ReactNode }) {
  const { t } = useJourney();
  return (
    <div className="min-h-screen">
      <header className="sticky top-0 z-30 border-b border-gray-200 bg-white/95 backdrop-blur">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-2">
          <Link href="/" className="font-bold text-brand-900">{t("appName")}</Link>
          <LanguagePicker compact />
        </div>
        {/* phones and tablets: compact horizontal progress */}
        <ol className="mx-auto flex max-w-6xl gap-1 px-4 pb-2 lg:hidden" aria-label="progress">
          {STEPS.map((s, i) => (
            <li key={s.key} className="flex-1">
              <div className={`h-1.5 rounded-full ${i <= step ? "bg-brand-600" : "bg-gray-200"}`} />
              {/* on narrow phones only the current step is labelled */}
              <span className={`mt-1 truncate text-[11px] ${i === step ? "block font-semibold text-brand-900" : "hidden text-gray-500 sm:block"}`}>
                {t(s.key)}
              </span>
            </li>
          ))}
        </ol>
      </header>
      <div className="mx-auto grid max-w-6xl gap-6 px-4 py-4 lg:grid-cols-[220px_1fr] lg:py-8">
        {/* desktop: vertical stepper */}
        <nav className="hidden lg:block" aria-label="progress">
          <ol className="sticky top-20 space-y-1">
            {STEPS.map((s, i) => (
              <li key={s.key} className={`flex items-center gap-3 rounded-lg px-3 py-2 ${i === step ? "bg-brand-50 font-semibold text-brand-900" : i < step ? "text-gray-700" : "text-gray-400"}`}>
                <span className={`flex h-7 w-7 items-center justify-center rounded-full text-sm ${i < step ? "bg-brand-600 text-white" : i === step ? "ring-2 ring-brand-600" : "ring-1 ring-gray-300"}`}>
                  {i < step ? "✓" : i + 1}
                </span>
                {t(s.key)}
              </li>
            ))}
          </ol>
        </nav>
        <main className="pb-action min-w-0">
          <h1 className="mb-4 text-xl font-bold text-gray-900 sm:text-2xl">{title}</h1>
          {children}
        </main>
      </div>
    </div>
  );
}

export function useLanguageLabel(code: LangCode) {
  return LANGUAGES.find((l) => l.code === code)?.native ?? code;
}
