"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { inr, ORDER_STATUS_LABELS, type AccountView, type Order } from "@udyam/shared";
import { Alert, Button, Card, LanguagePicker, Logo, Pill, Skeleton, Spinner } from "@/components/ui";
import { IconArrow, IconBack, IconCheck, IconShield, IconTruck, IconUser } from "@/components/icons";
import { at, useAccount } from "@/lib/account";
import { api, errorText, useJourney } from "@/lib/journey";

interface Row { leadId: string; reusedKyc: boolean; order: Order | null }

/** Customer account: sign in with mobile + one-time code, every past order with tracking, new order. */
export default function AccountPage() {
  const { j, ready, reset, t } = useJourney();
  const { account, ready: accountReady, signIn, signOut } = useAccount();
  const router = useRouter();
  const L = (k: Parameters<typeof at>[1]) => at(j.lang, k);

  const [mobile, setMobile] = useState("");
  const [code, setCode] = useState("");
  const [sent, setSent] = useState<{ mobileMasked: string; demoCode?: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [view, setView] = useState<AccountView | null>(null);
  const [rows, setRows] = useState<Row[] | null>(null);

  const load = useCallback(async () => {
    try {
      const v = await api.accounts.me();
      setView(v);
      setRows(await Promise.all(v.leads.map(async (l) => {
        const { orders } = await api.tracker.orders({ leadId: l.leadId }).catch(() => ({ orders: [] as Order[] }));
        return { leadId: l.leadId, reusedKyc: l.reusedKyc, order: orders[0] ?? null };
      })));
    } catch (e) {
      if ((e as { status?: number }).status === 401) await signOut();
      else setError(errorText(e, t("error")));
    }
  }, [signOut, t]);

  useEffect(() => {
    if (accountReady && account) void load();
  }, [accountReady, account, load]);

  async function sendCode() {
    setBusy(true);
    setError(null);
    try {
      setSent(await api.accounts.requestCode(mobile));
    } catch (e) {
      setError(errorText(e, t("error")));
    } finally {
      setBusy(false);
    }
  }

  async function verify() {
    setBusy(true);
    setError(null);
    try {
      // the order in progress (if any) joins the account
      const r = await api.accounts.verify({ mobile, code, leadId: j.leadId || undefined });
      signIn({ token: r.token, mobileMasked: r.account.mobileMasked });
      setSent(null);
      setCode("");
    } catch (e) {
      setError(errorText(e, t("error")));
    } finally {
      setBusy(false);
    }
  }

  function newOrder() {
    reset(); // fresh lead ID; JourneyProvider links it to the account
    router.push("/chat");
  }

  const STATUS = ORDER_STATUS_LABELS[j.lang];
  const orders = (rows ?? []).filter((r) => r.order);

  return (
    <div className="bg-journey min-h-screen">
      <header className="sticky top-0 z-30 bg-ink-900 text-white">
        <div className="mx-auto flex max-w-3xl items-center justify-between gap-3 px-4 py-2.5">
          <Logo dark />
          <LanguagePicker compact dark />
        </div>
      </header>
      <main className="mx-auto max-w-3xl space-y-4 px-4 py-6">
        <Link href={ready && j.messages.length ? "/chat" : "/"} className="inline-flex items-center gap-1.5 text-sm text-ink-500 hover:text-brand-700">
          <IconBack size={16} /> {L("back")}
        </Link>
        <h1 className="text-2xl font-bold tracking-tight sm:text-[28px]">{L("account")}</h1>

        {!accountReady && <Card><Skeleton className="h-24" /></Card>}

        {accountReady && !account && (
          <Card className="space-y-4">
            <div className="flex items-start gap-3">
              <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-brand-50 text-brand-700"><IconUser size={22} /></span>
              <div>
                <h2 className="font-bold">{L("signInTitle")}</h2>
                <p className="text-ink-700">{L("signInBody")}</p>
              </div>
            </div>
            {!sent ? (
              <form className="flex flex-col gap-2 sm:flex-row sm:items-end" onSubmit={(e) => { e.preventDefault(); void sendCode(); }}>
                <label className="flex-1">
                  <span className="mb-1 block text-sm font-semibold">{L("mobile")}</span>
                  <span className="flex min-h-12 items-center rounded-2xl bg-ink-50 ring-1 ring-ink-200 focus-within:ring-brand-500">
                    <span className="pl-4 text-ink-500">+91</span>
                    <input data-testid="mobile" value={mobile} onChange={(e) => setMobile(e.target.value.replace(/[^0-9 ]/g, ""))} inputMode="tel"
                      autoComplete="tel-national" maxLength={12} className="min-h-12 w-full bg-transparent px-2 text-base tracking-wider outline-none" placeholder="98765 43210" />
                  </span>
                </label>
                <Button data-testid="send-code" disabled={busy || mobile.replace(/\D/g, "").length !== 10}>{busy && <Spinner />}{L("sendCode")}</Button>
              </form>
            ) : (
              <form className="space-y-3" onSubmit={(e) => { e.preventDefault(); void verify(); }}>
                {sent.demoCode && (
                  <Alert tone="info">{L("demoCode")} <b data-testid="demo-code" className="font-mono tracking-widest">{sent.demoCode}</b></Alert>
                )}
                <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
                  <label className="flex-1">
                    <span className="mb-1 block text-sm font-semibold">{L("code")} · +91 {sent.mobileMasked}</span>
                    <input data-testid="otp" value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))} inputMode="numeric" autoComplete="one-time-code"
                      maxLength={6} className="min-h-12 w-full rounded-2xl bg-ink-50 px-4 font-mono text-lg tracking-[0.4em] ring-1 ring-ink-200 outline-none focus:ring-brand-500" />
                  </label>
                  <Button data-testid="sign-in" disabled={busy || code.length !== 6}>{busy && <Spinner />}{L("signIn")}</Button>
                </div>
                <button type="button" onClick={() => { setSent(null); setCode(""); }} className="text-sm text-ink-500 underline">{L("changeNumber")}</button>
              </form>
            )}
          </Card>
        )}

        {error && <Alert tone="error">{error}</Alert>}

        {account && (
          <>
            <Card className="flex flex-wrap items-center justify-between gap-3">
              <p className="flex items-center gap-2"><IconUser size={18} /> {L("signedInAs")} <b data-testid="signed-in">+91 {account.mobileMasked}</b></p>
              <button onClick={() => { void signOut(); setView(null); setRows(null); }} className="text-sm text-ink-500 underline hover:text-brand-700">{L("signOut")}</button>
            </Card>

            <Card>
              <h2 className="mb-2 flex items-center gap-2 font-bold"><IconShield size={18} /> {L("verified")}</h2>
              {!view && <Skeleton className="h-12" />}
              {view && !view.verified && <p className="text-ink-700">{L("notVerified")}</p>}
              {view?.verified && (
                <div className="text-sm" data-testid="verified-business">
                  <p className="text-base font-semibold">{view.verified.tradeName || view.verified.legalName}</p>
                  {view.verified.legalName && <p className="text-ink-700">{view.verified.legalName}</p>}
                  {view.verified.address && <p className="text-ink-700">{view.verified.address}</p>}
                  <p className="mt-2 flex flex-wrap gap-1.5">
                    <Pill tone="mint"><IconCheck size={12} /> GSTIN {view.verified.gstinMasked}</Pill>
                    <Pill>{L("verifiedOn")} {new Date(view.verified.verifiedAt).toLocaleDateString()}</Pill>
                  </p>
                </div>
              )}
            </Card>

            <Card className="p-0">
              <h2 className="flex items-center gap-2 px-5 pt-4 pb-2 font-bold"><IconTruck size={18} /> {L("orders")}</h2>
              {!rows && <div className="px-5 pb-4"><Skeleton className="h-16" /></div>}
              {rows && orders.length === 0 && <p className="px-5 pb-4 text-ink-700">{L("noOrders")}</p>}
              <ul className="divide-y divide-ink-100" data-testid="orders">
                {orders.map(({ leadId, order, reusedKyc }) => (
                  <li key={leadId} className="flex flex-wrap items-center gap-3 px-5 py-3" data-testid="order-row">
                    <div className="min-w-0 flex-1">
                      <p className="flex flex-wrap items-center gap-2 font-semibold">
                        #{order!.id.replaceAll("-", "").slice(0, 8).toUpperCase()}
                        <Pill tone={order!.status === "activated" ? "mint" : order!.status === "delayed" || order!.status === "failed" ? "amber" : "ink"}>
                          {STATUS[order!.status] ?? order!.status}
                        </Pill>
                        {leadId === j.leadId && <Pill tone="brand">{L("current")}</Pill>}
                        {reusedKyc && <Pill>{L("reusedKyc")}</Pill>}
                      </p>
                      <p className="truncate text-sm text-ink-500">{order!.lines.map((l) => l.productName).join(" · ")}</p>
                      <p className="text-sm text-ink-700">{inr(order!.monthlyTotalInr)} {t("perMonth")} · {new Date(order!.createdAt).toLocaleDateString()}</p>
                    </div>
                    <Button variant="secondary" data-testid="track-order" onClick={() => router.push(`/track?lead=${encodeURIComponent(leadId)}`)}>
                      {L("track")} <IconArrow size={16} />
                    </Button>
                  </li>
                ))}
              </ul>
            </Card>

            <Button data-testid="new-order" className="w-full sm:w-auto" onClick={newOrder}>{L("newOrder")} <IconArrow size={18} /></Button>
          </>
        )}
      </main>
    </div>
  );
}
