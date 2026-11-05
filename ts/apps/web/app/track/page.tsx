"use client";

import { useEffect, useState } from "react";
import { ORDER_STATUS_LABELS, type SentMessage, type Timeline } from "@udyam/shared";
import { Alert, Button, Card, Pill, Shell, Skeleton, Spinner } from "@/components/ui";
import { IconAlert, IconBolt, IconCamera, IconCheck, IconMessage, IconReceipt, IconShield, IconTruck, IconWifi } from "@/components/icons";
import { api, errorText, useJourney } from "@/lib/journey";

// one icon per happy-path stage (go/pkg/orders.HappyPath)
const STAGE_ICONS = [IconReceipt, IconCamera, IconShield, IconTruck, IconBolt, IconWifi];

export default function TrackPage() {
  const { j, ready, t } = useJourney();
  const [tl, setTl] = useState<Timeline | null>(null);
  const [msgs, setMsgs] = useState<SentMessage[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // /track?lead=<id> opens a past order from the account page; otherwise the current journey's
  const [lead, setLead] = useState<string | null>(null);
  useEffect(() => {
    if (ready) setLead(new URLSearchParams(window.location.search).get("lead") || j.leadId);
  }, [ready, j.leadId]);

  useEffect(() => {
    if (!lead) return;
    let stop = false;
    async function poll() {
      try {
        const { orders } = await api.tracker.orders({ leadId: lead! });
        if (orders[0]) setTl(await api.tracker.timeline(orders[0].id));
        setMsgs((await api.notify.messages(lead!)).messages);
      } catch {
        /* keep the last good state; polling retries */
      }
      setLoaded(true);
      if (!stop) setTimeout(poll, 3000);
    }
    void poll();
    return () => { stop = true; };
  }, [lead]);

  async function acceptFallback() {
    if (!tl) return;
    setBusy(true);
    try {
      await api.tracker.acceptFallback(tl.order.id);
    } catch (e) {
      setError(errorText(e, t("error")));
    } finally {
      setBusy(false);
    }
  }

  const o = tl?.order;
  const STATUS_LABEL = ORDER_STATUS_LABELS[j.lang];
  const live = o?.status === "activated";
  return (
    <Shell step={4} title={t("orderStatus")}>
      {!loaded && (
        <div className="grid gap-4 lg:grid-cols-[1fr_340px]">
          <Card><Skeleton className="h-8 w-1/3" /><Skeleton className="mt-4 h-40" /></Card>
          <Card><Skeleton className="h-48" /></Card>
        </div>
      )}
      {loaded && !o && <Alert>{t("noOrder")}</Alert>}
      {o && (
        <div className="grid gap-4 lg:grid-cols-[1fr_340px]">
          <div className="space-y-4">
            <Card className="overflow-hidden p-0">
              <div className={`px-5 py-5 ${live ? "bg-mint-600" : "bg-ink-900"} text-white`}>
                <p className="text-xs tracking-widest text-white/70">#{o.id.replaceAll("-", "").slice(0, 8).toUpperCase()}</p>
                <p data-testid="order-status" data-status={o.status} className="mt-1 flex items-center gap-2 text-2xl font-bold tracking-tight">
                  {live ? <IconCheck size={26} /> : o.status === "delayed" ? <IconAlert size={24} /> : <IconTruck size={24} />}
                  {STATUS_LABEL[o.status] ?? o.status}
                </p>
                {o.expectedActivationDate && !live && <p className="mt-1 text-white/80">{t("expectedBy", { date: o.expectedActivationDate })}</p>}
              </div>

              {/* phones: vertical list; md+: horizontal steps */}
              <ol className="flex flex-col gap-0 px-5 py-5 md:flex-row">
                {tl!.steps.map((s, i) => {
                  const Ico = STAGE_ICONS[i] ?? IconCheck;
                  const last = i === tl!.steps.length - 1;
                  return (
                    <li key={s.status} className="relative flex gap-3 pb-5 md:flex-1 md:flex-col md:items-center md:pb-0 md:text-center">
                      {!last && <span className={`absolute top-10 left-[19px] h-[calc(100%-2.5rem)] w-0.5 md:top-[19px] md:left-[calc(50%+22px)] md:h-0.5 md:w-[calc(100%-44px)] ${s.state === "done" ? "bg-ink-900" : "bg-ink-200"}`} />}
                      <span className={`relative z-10 flex h-10 w-10 shrink-0 items-center justify-center rounded-full ${
                        s.state === "done" ? "bg-ink-900 text-white" : s.state === "current" ? "pulse-ring bg-brand-600 text-white" : s.state === "blocked" ? "bg-amber-500 text-white" : "bg-ink-100 text-ink-500"}`}>
                        {s.state === "done" ? <IconCheck size={18} /> : s.state === "blocked" ? <IconAlert size={18} /> : <Ico size={18} />}
                      </span>
                      <span className="text-sm md:mt-2">
                        <span className={s.state === "upcoming" ? "text-ink-500" : "font-semibold"}>{STATUS_LABEL[s.status]}</span>
                        {s.at && <span className="block text-xs text-ink-500">{new Date(s.at).toLocaleString([], { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" })}</span>}
                      </span>
                    </li>
                  );
                })}
              </ol>
            </Card>

            {o.status === "delayed" && o.fallbackOffered && (
              <Card className="animate-rise bg-amber-50 ring-2 ring-amber-300">
                <div className="flex items-start gap-3">
                  <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-ink-900 text-white"><IconBolt size={22} /></span>
                  <div>
                    <h2 className="font-bold">{t("fallbackTitle")}</h2>
                    <p className="text-ink-700">{t("fallbackBody")}</p>
                  </div>
                </div>
                <Button data-testid="fallback-accept" className="mt-4 w-full sm:w-auto" disabled={busy} onClick={acceptFallback}>{busy ? <Spinner /> : <IconWifi size={18} />}{t("fallbackAccept")}</Button>
              </Card>
            )}
            {error && <Alert tone="error">{error}</Alert>}

            <Card>
              <ul className="divide-y divide-ink-100 text-sm">
                {o.lines.map((l) => (
                  <li key={l.productId} className="flex items-center justify-between gap-2 py-2">
                    <span className="font-medium">{l.productName}{l.quantity > 1 ? ` × ${l.quantity}` : ""}</span>
                    <Pill>{l.installDays}d</Pill>
                  </li>
                ))}
              </ul>
            </Card>
          </div>

          <aside>
            <Card className="overflow-hidden p-0">
              <div className="flex items-center gap-2 bg-[#075e54] px-4 py-3 text-white">
                <IconMessage size={18} /> <h2 className="font-semibold">{t("messagesSent")}</h2>
              </div>
              <ul className="max-h-[560px] space-y-2 overflow-y-auto bg-[#efeae2] p-3">
                {msgs.map((m) => (
                  <li key={m.id} className="animate-rise ml-4 rounded-2xl rounded-tr-md bg-[#d9fdd3] p-3 text-sm text-ink-900 shadow-sm">
                    <span className="mb-1 block text-[10px] font-bold tracking-wider text-[#075e54] uppercase">{m.channel}</span>
                    {m.body}
                    <span className="mt-1 block text-right text-[11px] text-ink-500">{new Date(m.createdAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} ✓✓</span>
                  </li>
                ))}
                {msgs.length === 0 && <li className="p-6 text-center text-sm text-ink-500">…</li>}
              </ul>
            </Card>
          </aside>
        </div>
      )}
    </Shell>
  );
}
