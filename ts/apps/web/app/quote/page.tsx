"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { inr, type Product, type Quote } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Pill, Shell, Skeleton, Spinner, Toggle } from "@/components/ui";
import { IconArrow, IconBack, IconCamera, IconChart, IconCheck, IconLock, IconMessage, IconTruck, IconWifi } from "@/components/icons";
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
      {!quote && !error && (
        <div className="grid gap-4 lg:grid-cols-[1fr_320px]">
          <Card><div className="space-y-3"><Skeleton className="h-6 w-1/2" /><Skeleton className="h-14" /><Skeleton className="h-14" /><Skeleton className="h-14" /></div></Card>
          <Card><Skeleton className="h-24" /></Card>
        </div>
      )}
      {error && <div className="mb-4"><Alert tone="error">{error}</Alert></div>}
      {quote && (
        <div className="grid gap-4 lg:grid-cols-[1fr_320px]">
          <Card className="p-3 sm:p-5">
            <ul className="divide-y divide-ink-100">
              {quote.lines.map((l) => (
                <li key={l.productId} className="flex items-start gap-3 py-3">
                  <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-brand-50 text-brand-700">{lineIcon(l.productId)}</span>
                  <div className="min-w-0 flex-1">
                    <p className="font-semibold">{l.productName}{l.quantity > 1 ? <span className="text-ink-500"> × {l.quantity}</span> : null}</p>
                    <p className="text-sm text-ink-500">
                      {inr(l.unitPriceInr)} / {l.unit} · {l.contractMonths} mo{l.oneTimeInr ? ` · ${inr(l.oneTimeInr)} ${t("oneTime")}` : ""}
                    </p>
                    {l.addedFor && <p className="mt-1"><Pill tone="amber">{l.note ?? `Needed for ${l.addedFor}`}</Pill></p>}
                  </div>
                  <div className="flex items-center gap-1">
                    <span className="tabular-nums font-semibold">{inr(l.monthlyInr)}</span>
                    {!accepted && (
                      <button aria-label={`remove ${l.productName}`} disabled={!!busy} onClick={() => toggle(l.productId, false)}
                        className="ml-1 flex h-9 w-9 items-center justify-center rounded-xl text-ink-500 hover:bg-brand-50 hover:text-brand-700">✕</button>
                    )}
                  </div>
                </li>
              ))}
            </ul>
            {quote.notes.length > 0 && (
              <ul className="mt-2 space-y-1 rounded-2xl bg-ink-50 p-3 text-sm text-ink-700">{quote.notes.map((n) => <li key={n}>• {n}</li>)}</ul>
            )}

            {!accepted && addOns.some((p) => !inQuote.has(p.id)) && (
              <>
                <h2 className="mt-6 mb-2 font-bold">{t("addOns")}</h2>
                <ul className="grid gap-2 md:grid-cols-2 lg:grid-cols-1 2xl:grid-cols-2">
                  {addOns.filter((p) => !inQuote.has(p.id)).map((p) => (
                    <li key={p.id} className="flex min-h-16 items-center gap-3 rounded-2xl bg-ink-50 p-3 ring-1 ring-ink-100">
                      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-ink-700 ring-1 ring-ink-200">{lineIcon(p.id)}</span>
                      <span className="min-w-0 flex-1">
                        <span className="block text-[15px] font-semibold leading-tight">{p.name}</span>
                        <span className="block text-xs text-ink-500">{inr(p.priceMinInr)}–{inr(p.priceMaxInr)} / {p.unit}</span>
                      </span>
                      {busy === p.id ? <Spinner /> : <Toggle checked={false} disabled={!!busy} onChange={() => toggle(p.id, true)} label={p.name} />}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </Card>

          <aside>
            <Card className="overflow-hidden p-0 lg:sticky lg:top-24">
              <div className="bg-ink-900 px-5 py-5 text-white">
                <p className="text-sm text-white/70">{t("perMonth")}</p>
                <p key={quote.monthlyTotalInr} className="animate-rise text-4xl font-bold tracking-tight">{inr(quote.monthlyTotalInr)}</p>
              </div>
              <dl className="space-y-2 px-5 py-4 text-sm">
                <div className="flex justify-between"><dt className="text-ink-500">{t("oneTime")}</dt><dd className="tabular-nums font-semibold">{inr(quote.oneTimeTotalInr)}</dd></div>
                <div className="flex justify-between"><dt className="text-ink-500">{t("totalContract", { months: quote.contractMonths })}</dt><dd className="tabular-nums font-semibold">{inr(quote.totalContractValueInr)}</dd></div>
              </dl>
              <p className="flex items-center gap-2 border-t border-ink-100 px-5 py-3 text-sm text-ink-700"><IconTruck size={16} /> {t("installIn", { days: quote.expectedInstallDays })}</p>
            </Card>
          </aside>
        </div>
      )}
      {quote && (
        <ActionBar>
          <Button variant="secondary" onClick={() => router.push("/chat")}><IconBack size={18} /> {t("stepChat")}</Button>
          {accepted ? (
            <Button data-testid="accept" onClick={() => router.push("/consent")}>{t("stepConsent")} <IconArrow size={18} /></Button>
          ) : (
            <Button data-testid="accept" disabled={!!busy} onClick={accept}>{busy === "accept" ? <Spinner /> : <IconCheck size={18} />}{t("accept")}</Button>
          )}
        </ActionBar>
      )}
    </Shell>
  );
}

/** Category icon by catalogue product (P01-P11 connectivity, P12-P15 communication, P16-P19 cloud, P20-P22 security, P23-P25 IoT). */
function lineIcon(id: string) {
  const n = Number(id.slice(1));
  if (n <= 11) return <IconWifi size={18} />;
  if (n <= 15) return <IconMessage size={18} />;
  if (n <= 19) return <IconChart size={18} />;
  if (n <= 22) return <IconLock size={18} />;
  return <IconCamera size={18} />;
}
