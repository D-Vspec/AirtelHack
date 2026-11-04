"use client";

import { useEffect, useState } from "react";
import type { ScoredLead } from "@udyam/shared";
import { Card, Spinner } from "@/components/ui";
import { api } from "@/lib/journey";

const BAND: Record<string, string> = { high: "bg-green-100 text-green-800", medium: "bg-amber-100 text-amber-900", low: "bg-gray-100 text-gray-700" };

/** The sales "morning list" (scenario 23): leads ranked by propensity, each with its top reasons. */
export default function LeadsPage() {
  const [source, setSource] = useState<"live" | "case">("live");
  const [leads, setLeads] = useState<ScoredLead[] | null>(null);

  useEffect(() => {
    setLeads(null);
    let stop = false;
    const load = () => api.scoring.leads({ source, limit: 100 }).then((r) => { if (!stop) setLeads(r.leads); }).catch(() => {});
    void load();
    const id = setInterval(load, 5000);
    return () => { stop = true; clearInterval(id); };
  }, [source]);

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-bold">Leads by score</h1>
        <div className="flex gap-1 rounded-xl bg-white p-1 ring-1 ring-gray-200">
          {(["live", "case"] as const).map((s) => (
            <button key={s} onClick={() => setSource(s)} className={`min-h-10 rounded-lg px-3 text-sm ${source === s ? "bg-brand-600 text-white" : "text-gray-700"}`}>
              {s === "live" ? "Live (advisor)" : "Case data, June"}
            </button>
          ))}
        </div>
      </div>
      <p className="mb-4 text-sm text-gray-600">
        Probability of activation from features known at lead creation (docs/model-card.md). Call the top of the list first; the reasons say why.
      </p>
      {!leads && <Spinner />}
      {leads?.length === 0 && <Card>No scored leads yet.</Card>}
      <ul className="grid gap-2 lg:grid-cols-2">
        {leads?.map((l, i) => (
          <li key={l.leadId}>
            <Card>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate font-semibold">{i + 1}. {l.businessName || l.leadId}</p>
                  <p className="truncate text-xs text-gray-600">{[l.segment, l.city, l.leadSource].filter(Boolean).join(" · ")}</p>
                </div>
                <div className="shrink-0 text-right">
                  <p className="text-lg font-bold tabular-nums text-brand-900">{(l.probability * 100).toFixed(1)}%</p>
                  <span className={`rounded px-2 text-xs ${BAND[l.band]}`}>{l.band}</span>
                </div>
              </div>
              <ul className="mt-2 space-y-0.5 text-sm">
                {l.reasons.map((r) => (
                  <li key={r.field} className={r.effect === "raises" ? "text-green-800" : "text-red-800"}>{r.effect === "raises" ? "▲" : "▼"} {r.text}</li>
                ))}
              </ul>
            </Card>
          </li>
        ))}
      </ul>
    </>
  );
}
