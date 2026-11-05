"use client";

import Link from "next/link";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { LANGUAGES, type LangCode, type StringKey } from "@udyam/shared";
import { useJourney } from "@/lib/journey";
import { IconAlert, IconCamera, IconChat, IconCheck, IconGlobe, IconReceipt, IconShield, IconSparkle, IconTruck } from "@/components/icons";

export function Button({ variant = "primary", className = "", ...p }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" | "ghost" | "danger" | "dark" }) {
  const styles = {
    primary: "bg-brand-600 text-white shadow-lift hover:bg-brand-700 active:translate-y-px disabled:bg-ink-200 disabled:text-ink-500 disabled:shadow-none",
    secondary: "bg-white text-ink-900 ring-1 ring-ink-200 hover:ring-ink-500 disabled:text-ink-500",
    ghost: "text-brand-700 hover:bg-brand-50",
    danger: "bg-white text-brand-700 ring-1 ring-brand-200 hover:bg-brand-50",
    dark: "bg-ink-900 text-white hover:bg-ink-700",
  }[variant];
  // min-h-12: thumb-sized tap targets on phones
  return (
    <button
      {...p}
      className={`inline-flex min-h-12 items-center justify-center gap-2 rounded-2xl px-5 text-[15px] font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600 disabled:cursor-not-allowed ${styles} ${className}`}
    />
  );
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  // a className with its own p-0 means a flush card (header bands, threads): no default padding
  const pad = /(^|\s)p-0(\s|$)/.test(className) ? "" : "p-4 sm:p-6";
  return <section className={`min-w-0 rounded-[var(--radius-card)] bg-white shadow-card ring-1 ring-ink-100 ${pad} ${className}`}>{children}</section>;
}

export function Pill({ children, tone = "ink" }: { children: ReactNode; tone?: "ink" | "brand" | "mint" | "amber" }) {
  const styles = { ink: "bg-ink-100 text-ink-700", brand: "bg-brand-50 text-brand-700", mint: "bg-mint-50 text-mint-600", amber: "bg-amber-50 text-amber-800" }[tone];
  return <span className={`inline-flex max-w-full items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold break-words ${styles}`}>{children}</span>;
}

export function Spinner({ label }: { label?: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-ink-500" role="status">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-brand-600 border-t-transparent" />
      {label}
    </span>
  );
}

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`skeleton ${className}`} aria-hidden="true" />;
}

export function Alert({ tone = "info", children }: { tone?: "info" | "success" | "warning" | "error"; children: ReactNode }) {
  const styles = {
    info: "bg-ink-50 text-ink-900 ring-ink-200",
    success: "bg-mint-50 text-green-900 ring-green-200",
    warning: "bg-amber-50 text-amber-900 ring-amber-200",
    error: "bg-brand-50 text-brand-900 ring-brand-200",
  }[tone];
  const icon = tone === "success" ? <IconCheck size={18} /> : tone === "info" ? <IconSparkle size={18} /> : <IconAlert size={18} />;
  return (
    <div className={`flex gap-3 rounded-2xl p-3 ring-1 ${styles}`} role={tone === "error" ? "alert" : undefined}>
      <span className="mt-0.5 shrink-0">{icon}</span>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

/** An accessible on/off switch (role=switch). */
export function Toggle({ checked, onChange, disabled, label }: { checked: boolean; onChange: (v: boolean) => void; disabled?: boolean; label: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-7 w-12 shrink-0 items-center rounded-full transition ${checked ? "bg-brand-600" : "bg-ink-200"} disabled:opacity-50`}
    >
      <span className={`inline-block h-5 w-5 rounded-full bg-white shadow transition ${checked ? "translate-x-6" : "translate-x-1"}`} />
    </button>
  );
}

/** The main action of a step: pinned to the bottom of the screen on phones, inline on larger screens. */
export function ActionBar({ children }: { children: ReactNode }) {
  return (
    <div className="fixed inset-x-0 bottom-0 z-20 border-t border-ink-100 bg-white/95 px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] shadow-[0_-8px_24px_-16px_rgb(22_22_26/0.25)] backdrop-blur md:static md:mt-6 md:border-0 md:bg-transparent md:p-0 md:shadow-none">
      <div className="mx-auto flex max-w-3xl flex-col gap-2 sm:flex-row sm:justify-end">{children}</div>
    </div>
  );
}

export function Logo({ dark = false }: { dark?: boolean }) {
  const { t } = useJourney();
  return (
    <Link href="/" className="flex items-center gap-2">
      <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-brand-600 text-white shadow-lift">
        <IconWifiMark />
      </span>
      <span className={`text-[17px] font-bold tracking-tight ${dark ? "text-white" : "text-ink-900"}`}>{t("appName")}</span>
    </Link>
  );
}

function IconWifiMark() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" aria-hidden="true">
      <path d="M4.5 10a11 11 0 0 1 15 0M7.5 13.5a6.5 6.5 0 0 1 9 0" />
      <circle cx="12" cy="17.5" r="1.4" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function LanguagePicker({ compact = false, dark = false }: { compact?: boolean; dark?: boolean }) {
  const { j, update, t } = useJourney();
  return (
    <label className="flex items-center gap-2 text-sm">
      {!compact && <span className="text-ink-500">{t("chooseLanguage")}</span>}
      <span className={`flex items-center gap-1.5 rounded-xl px-2 ${dark ? "bg-white/10 text-white ring-1 ring-white/20" : "bg-white ring-1 ring-ink-200"}`}>
        <IconGlobe size={16} />
        <select
          aria-label={t("chooseLanguage")}
          value={j.lang}
          onChange={(e) => update({ lang: e.target.value as LangCode })}
          className={`min-h-10 bg-transparent pr-1 text-[15px] outline-none ${dark ? "text-white [&>option]:text-ink-900" : ""}`}
        >
          {LANGUAGES.map((l) => (
            <option key={l.code} value={l.code}>{l.native}</option>
          ))}
        </select>
      </span>
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
const STEP_ICONS = [IconChat, IconReceipt, IconShield, IconCamera, IconTruck];

/** Page frame for the owner journey. `step` is the index into STEPS. */
export function Shell({ step, title, children }: { step: number; title: string; children: ReactNode }) {
  const { t } = useJourney();
  return (
    <div className="bg-journey min-h-screen">
      <header className="sticky top-0 z-30 bg-ink-900 text-white">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-2.5">
          <Logo dark />
          <LanguagePicker compact dark />
        </div>
        {/* phones and tablets: compact progress with the current step named */}
        <ol className="mx-auto flex max-w-6xl gap-1.5 px-4 pb-3 lg:hidden" aria-label="progress">
          {STEPS.map((s, i) => (
            <li key={s.key} className="flex-1">
              <div className={`h-1.5 rounded-full transition ${i < step ? "bg-brand-500" : i === step ? "bg-white" : "bg-white/15"}`} />
              <span className={`mt-1.5 truncate text-[11px] ${i === step ? "block font-semibold text-white" : "hidden text-white/50 sm:block"}`}>
                {t(s.key)}
              </span>
            </li>
          ))}
        </ol>
      </header>
      <div className="mx-auto grid max-w-6xl gap-6 px-4 py-5 lg:grid-cols-[230px_1fr] lg:py-8">
        {/* desktop: vertical stepper with icons */}
        <nav className="hidden lg:block" aria-label="progress">
          <ol className="sticky top-24 space-y-1 rounded-[var(--radius-card)] bg-white p-2 shadow-card ring-1 ring-ink-100">
            {STEPS.map((s, i) => {
              const Ico = STEP_ICONS[i];
              const state = i < step ? "done" : i === step ? "current" : "todo";
              return (
                <li key={s.key} className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-[15px] ${state === "current" ? "bg-brand-50 font-semibold text-ink-900" : state === "done" ? "text-ink-700" : "text-ink-500"}`}>
                  <span className={`flex h-8 w-8 items-center justify-center rounded-full ${state === "done" ? "bg-ink-900 text-white" : state === "current" ? "bg-brand-600 text-white" : "bg-ink-100 text-ink-500"}`}>
                    {state === "done" ? <IconCheck size={16} /> : <Ico size={16} />}
                  </span>
                  {t(s.key)}
                </li>
              );
            })}
          </ol>
        </nav>
        <main className="pb-action min-w-0">
          <h1 className="mb-4 text-2xl font-bold tracking-tight text-ink-900 sm:text-[28px]">{title}</h1>
          {children}
        </main>
      </div>
    </div>
  );
}

export function useLanguageLabel(code: LangCode) {
  return LANGUAGES.find((l) => l.code === code)?.native ?? code;
}
