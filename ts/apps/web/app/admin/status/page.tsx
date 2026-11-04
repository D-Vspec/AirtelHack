"use client";

import { useEffect, useState } from "react";
import type { HealthResponse } from "@udyam/shared";
import { Alert, Card, Spinner } from "@/components/ui";
import { api } from "@/lib/journey";

const SERVICES = ["registry", "rules", "quote", "kyc", "tracker", "notify", "scoring", "advisor"];

interface Row { service: string; up: string; requests: number; errors5xx: number; avgMs: number | null; events: number; deadLetters: number; outbox: number | null; extra: string[] }

/** Sum a Prometheus metric's samples whose labels contain every given label="value". */
function sum(text: string, metric: string, labels: string[] = []): number {
  let total = 0;
  for (const line of text.split("\n")) {
    if (!line.startsWith(metric + "{") && !line.startsWith(metric + " ")) continue;
    if (labels.every((l) => line.includes(l))) total += Number(line.slice(line.lastIndexOf(" ") + 1)) || 0;
  }
  return total;
}

/** Service status: gateway health plus each service's /metrics, refreshed every 5 s. */
export default function StatusPage() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [rows, setRows] = useState<Row[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let stop = false;
    async function load() {
      try {
        const h = await api.health();
        const r = await Promise.all(SERVICES.map(async (service): Promise<Row> => {
          const text = await api.metrics(service).catch(() => "");
          const count = sum(text, "udyam_http_request_seconds_count");
          const secs = sum(text, "udyam_http_request_seconds_sum");
          const outbox = text.includes("udyam_outbox_pending") ? sum(text, "udyam_outbox_pending") : null;
          const extra = text.includes("udyam_kyc_reviews_open") ? [`${sum(text, "udyam_kyc_reviews_open")} open reviews`] : [];
          return {
            service, up: h.services[service] ?? "down", requests: sum(text, "udyam_http_requests_total"),
            errors5xx: sum(text, "udyam_http_requests_total", ['status="5xx"']), avgMs: count ? (secs / count) * 1000 : null,
            events: sum(text, "udyam_kafka_events_total", ['result="processed"']), deadLetters: sum(text, "udyam_kafka_events_total", ['result="dead_lettered"']),
            outbox, extra,
          };
        }));
        if (!stop) { setHealth(h); setRows(r); setError(null); }
      } catch {
        if (!stop) setError("Gateway unreachable");
      }
      if (!stop) setTimeout(load, 5000);
    }
    void load();
    return () => { stop = true; };
  }, []);

  return (
    <>
      <h1 className="mb-3 text-xl font-bold">Service status</h1>
      {error && <Alert tone="error">{error}</Alert>}
      {!health && !error && <Spinner />}
      {health && (
        <div className="mb-4">
          <Alert tone={health.status === "ok" ? "success" : "warning"}>
            Overall: <b>{health.status}</b> · Postgres, Kafka and every service behind the gateway
          </Alert>
        </div>
      )}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {rows.map((r) => (
          <Card key={r.service}>
            <div className="flex items-center justify-between">
              <h2 className="font-semibold">{r.service}</h2>
              <span className={`rounded px-2 text-xs ${r.up === "ok" ? "bg-green-100 text-green-800" : "bg-red-100 text-red-800"}`}>{r.up}</span>
            </div>
            <dl className="mt-2 grid grid-cols-2 gap-x-2 text-sm">
              <dt className="text-gray-600">requests</dt><dd className="text-right tabular-nums">{r.requests}</dd>
              <dt className="text-gray-600">5xx</dt><dd className={`text-right tabular-nums ${r.errors5xx ? "text-red-700" : ""}`}>{r.errors5xx}</dd>
              <dt className="text-gray-600">avg latency</dt><dd className="text-right tabular-nums">{r.avgMs === null ? "–" : `${r.avgMs.toFixed(0)} ms`}</dd>
              <dt className="text-gray-600">events in</dt><dd className="text-right tabular-nums">{r.events}</dd>
              <dt className="text-gray-600">dead letters</dt><dd className={`text-right tabular-nums ${r.deadLetters ? "text-amber-700" : ""}`}>{r.deadLetters}</dd>
              {r.outbox !== null && <><dt className="text-gray-600">outbox backlog</dt><dd className={`text-right tabular-nums ${r.outbox > 0 ? "text-amber-700" : ""}`}>{r.outbox}</dd></>}
            </dl>
            {r.extra.map((x) => <p key={x} className="mt-1 text-xs text-gray-600">{x}</p>)}
          </Card>
        ))}
      </div>
      <p className="mt-4 text-xs text-gray-500">Counters reset when a service restarts. Raw Prometheus text: GET /api/&lt;service&gt;/metrics with X-Admin-Token.</p>
    </>
  );
}
