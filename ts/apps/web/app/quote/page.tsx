"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { inr, type Product, type Quote } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Shell, Spinner } from "@/components/ui";
import { api, errorText, useJourney } from "@/lib/journey";

// Add-ons worth offering on top of a bundle; only those the rules service says are eligible are shown.
const ADD_ONS = ["P09", "P11", "P12", "P17", "P20", "P22", "P23"];

export default function QuotePage() {
  const { j, ready, t } = useJourney();
  const router = useRouter();
  const [quote, setQuote] = useState<Quote | null>(null);
  const [addOns, setAddOns] = useState<Product[]>([]);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!ready || !j.quoteId) return;
    api.quotes.get(j.quoteId).then(setQuote).catch((e) => setError(errorText(e, t("error"))));
    api.rules.eligibility(j.profile).then((el) => setAddOns(el.eligible.filter((p) => ADD_ONS.includes(p.id)))).catch(() => setAddOns([]));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, j.quoteId]);

  async function toggle(productId: string, enabled: boolean) {
    if (!quote) return;
    setBusy(productId);
    setError(null);
    try {
      setQuote(await api.quotes.toggle(quote.id, [{ productId, enabled }]));
    } catch (e) {
      setError(errorText(e, t("error"))); // e.g. "Biz Fibre cannot be removed: Cloud CCTV needs one of ..."
    } finally {
      setBusy(null);
    }
  }

  async function accept() {
    if (!quote) return;
    setBusy("accept");
    try {
      await api.quotes.accept(quote.id);
      router.push("/consent");
    } catch (e) {
      setError(errorText(e, t("error")));
      setBusy(null);
    }
  }

  if (ready && !j.quoteId) {
    return (
      <Shell step={1} title={t("stepQuote")}>
        <Alert>{t("noOrder")} <Link className="underline" href="/chat">{t("stepChat")}</Link></Alert>
      </Shell>
    );
  }

  const inQuote = new Set(quote?.lines.map((l) => l.productId));
  const accepted = quote?.status === "accepted";

  return (
    <Shell step={1} title={t("stepQuote")}>
      {!quote && !error && <Spinner />}
      {error && <div className="mb-4"><Alert tone="error">{error}</Alert></div>}
      {quote && (
        <div className="grid gap-4 lg:grid-cols-[1fr_300px]">
          <Card>
            <ul className="divide-y divide-gray-100">
              {quote.lines.map((l) => (
                <li key={l.productId} className="flex flex-wrap items-start justify-between gap-2 py-3">
                  <div className="min-w-0">
                    <p className="font-medium">{l.productName}{l.quantity > 1 ? ` × ${l.quantity}` : ""}</p>
                    <p className="text-sm text-gray-600">
                      {inr(l.unitPriceInr)} / {l.unit} · {l.contractMonths} mo{l.oneTimeInr ? ` · ${inr(l.oneTimeInr)} ${t("oneTime")}` : ""}
                    </p>
                    {l.addedFor && <p className="text-xs text-amber-800">{l.note ?? `Needed for ${l.addedFor}`}</p>}
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="tabular-nums font-semibold">{inr(l.monthlyInr)}</span>
                    {!accepted && (
                      <button aria-label={`remove ${l.productName}`} disabled={!!busy} onClick={() => toggle(l.productId, false)} className="min-h-10 rounded-lg px-2 text-sm text-gray-500 hover:bg-red-50 hover:text-red-700">✕</button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
            {quote.notes.length > 0 && (
              <ul className="mt-3 list-disc space-y-1 pl-5 text-sm text-gray-700">{quote.notes.map((n) => <li key={n}>{n}</li>)}</ul>
            )}

            {!accepted && addOns.some((p) => !inQuote.has(p.id)) && (
              <>
                <h2 className="mt-6 mb-2 font-semibold">{t("addOns")}</h2>
                <ul className="grid gap-2 sm:grid-cols-2">
                  {addOns.filter((p) => !inQuote.has(p.id)).map((p) => (
                    <li key={p.id}>
                      <label className="flex min-h-14 cursor-pointer items-center gap-3 rounded-xl p-3 ring-1 ring-gray-200 hover:ring-brand-600">
                        <input type="checkbox" className="h-5 w-5 accent-brand-600" disabled={!!busy} checked={false} onChange={() => toggle(p.id, true)} />
                        <span className="min-w-0">
                          <span className="block font-medium">{p.name}</span>
                          <span className="block text-xs text-gray-600">{inr(p.priceMinInr)}–{inr(p.priceMaxInr)} / {p.unit}</span>
                        </span>
                        {busy === p.id && <Spinner />}
                      </label>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </Card>

          <aside>
            <Card className="lg:sticky lg:top-24">
              <p className="text-3xl font-bold text-brand-900">{inr(quote.monthlyTotalInr)}</p>
              <p className="text-gray-600">{t("perMonth")}</p>
              <dl className="mt-3 space-y-1 text-sm">
                <div className="flex justify-between"><dt>{t("oneTime")}</dt><dd className="tabular-nums">{inr(quote.oneTimeTotalInr)}</dd></div>
                <div className="flex justify-between"><dt>{t("totalContract", { months: quote.contractMonths })}</dt><dd className="tabular-nums">{inr(quote.totalContractValueInr)}</dd></div>
              </dl>
              <p className="mt-3 text-sm text-gray-700">{t("installIn", { days: quote.expectedInstallDays })}</p>
            </Card>
          </aside>
        </div>
      )}
      {quote && (
        <ActionBar>
          <Button variant="secondary" onClick={() => router.push("/chat")}>← {t("stepChat")}</Button>
          {accepted ? (
            <Button data-testid="accept" onClick={() => router.push("/consent")}>{t("stepConsent")} →</Button>
          ) : (
            <Button data-testid="accept" disabled={!!busy} onClick={accept}>{busy === "accept" ? <Spinner /> : null}{t("accept")}</Button>
          )}
        </ActionBar>
      )}
    </Shell>
  );
}
