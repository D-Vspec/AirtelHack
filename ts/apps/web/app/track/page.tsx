"use client";

import { useEffect, useState } from "react";
import { ORDER_STATUS_LABELS, type SentMessage, type Timeline } from "@udyam/shared";
import { Alert, Button, Card, Shell, Spinner } from "@/components/ui";
import { api, errorText, useJourney } from "@/lib/journey";


export default function TrackPage() {
  const { j, ready, t } = useJourney();
  const [tl, setTl] = useState<Timeline | null>(null);
  const [msgs, setMsgs] = useState<SentMessage[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!ready) return;
    let stop = false;
    async function poll() {
      try {
        const { orders } = await api.tracker.orders({ leadId: j.leadId });
        if (orders[0]) setTl(await api.tracker.timeline(orders[0].id));
        setMsgs((await api.notify.messages(j.leadId)).messages);
      } catch {
        /* keep the last good state; polling retries */
      }
      setLoaded(true);
      if (!stop) setTimeout(poll, 3000);
    }
    void poll();
    return () => { stop = true; };
  }, [ready, j.leadId]);

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
  return (
    <Shell step={4} title={t("orderStatus")}>
      {!loaded && <Spinner />}
      {loaded && !o && <Alert>{t("noOrder")}</Alert>}
      {o && (
        <div className="grid gap-4 lg:grid-cols-[1fr_340px]">
          <div className="space-y-4">
            <Card>
              <p className="text-sm text-gray-600">#{o.id.replaceAll("-", "").slice(0, 8).toUpperCase()}</p>
              <p data-testid="order-status" data-status={o.status} className="text-xl font-bold text-brand-900">{STATUS_LABEL[o.status] ?? o.status}</p>
              {o.expectedActivationDate && o.status !== "activated" && <p className="text-gray-700">{t("expectedBy", { date: o.expectedActivationDate })}</p>}

              {/* phones: vertical list; md+: horizontal steps */}
              <ol className="mt-4 flex flex-col gap-3 md:flex-row md:gap-0">
                {tl!.steps.map((s, i) => (
                  <li key={s.status} className="flex items-center gap-3 md:flex-1 md:flex-col md:text-center">
                    <span className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-semibold ${
                      s.state === "done" ? "bg-green-600 text-white" : s.state === "current" ? "bg-brand-600 text-white" : s.state === "blocked" ? "bg-amber-500 text-white" : "bg-gray-200 text-gray-500"}`}>
                      {s.state === "done" ? "✓" : s.state === "blocked" ? "!" : i + 1}
                    </span>
                    <span className="text-sm md:mt-1">
                      <span className={s.state === "upcoming" ? "text-gray-500" : "font-medium"}>{STATUS_LABEL[s.status]}</span>
                      {s.at && <span className="block text-xs text-gray-500">{new Date(s.at).toLocaleString()}</span>}
                    </span>
                  </li>
                ))}
              </ol>
            </Card>

            {o.status === "delayed" && o.fallbackOffered && (
              <Card className="ring-2 ring-amber-400">
                <h2 className="font-semibold">{t("fallbackTitle")}</h2>
                <p className="text-gray-700">{t("fallbackBody")}</p>
                <Button data-testid="fallback-accept" className="mt-3 w-full sm:w-auto" disabled={busy} onClick={acceptFallback}>{busy && <Spinner />}{t("fallbackAccept")}</Button>
              </Card>
            )}
            {error && <Alert tone="error">{error}</Alert>}

            <Card>
              <ul className="space-y-1 text-sm">
                {o.lines.map((l) => (
                  <li key={l.productId} className="flex justify-between gap-2">
                    <span>{l.productName}{l.quantity > 1 ? ` × ${l.quantity}` : ""}</span>
                    <span className="text-gray-500">{l.installDays}d</span>
                  </li>
                ))}
              </ul>
            </Card>
          </div>

          <aside>
            <Card>
              <h2 className="mb-2 font-semibold">{t("messagesSent")}</h2>
              <ul className="space-y-2">
                {msgs.map((m) => (
                  <li key={m.id} className="rounded-xl bg-[#e7f8e9] p-3 text-sm">
                    <span className="mb-1 block text-xs font-semibold uppercase text-green-800">{m.channel}</span>
                    {m.body}
                    <span className="mt-1 block text-right text-[11px] text-gray-500">{new Date(m.createdAt).toLocaleTimeString()}</span>
                  </li>
                ))}
              </ul>
            </Card>
          </aside>
        </div>
      )}
    </Shell>
  );
}
