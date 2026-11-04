"use client";

import { useEffect, useState } from "react";
import type { PartnerTask } from "@udyam/shared";
import { Alert, Button, Card, Spinner } from "@/components/ui";
import { api, errorText } from "@/lib/journey";

/** Leads the advisor could not close (lead.unclosed), highest score first, with the conversation summary. */
export default function HandoffPage() {
  const [status, setStatus] = useState<"open" | "claimed" | "closed">("open");
  const [tasks, setTasks] = useState<PartnerTask[] | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = () => api.scoring.tasks(status).then((r) => setTasks(r.tasks)).catch((e) => setError(errorText(e, "Could not load tasks")));
  useEffect(() => {
    setTasks(null);
    void load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status]);

  async function act(fn: () => Promise<unknown>) {
    try {
      await fn();
      await load();
    } catch (e) {
      setError(errorText(e, "Update failed"));
    }
  }

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-bold">Partner handoff</h1>
        <select value={status} onChange={(e) => setStatus(e.target.value as typeof status)} className="min-h-10 rounded-lg border border-gray-300 bg-white px-2">
          <option value="open">Open</option>
          <option value="claimed">Claimed</option>
          <option value="closed">Closed</option>
        </select>
      </div>
      {error && <div className="mb-3"><Alert tone="error">{error}</Alert></div>}
      {!tasks && <Spinner />}
      {tasks?.length === 0 && <Card>No {status} tasks.</Card>}
      <ul className="space-y-2">
        {tasks?.map((tk) => (
          <li key={tk.id}>
            <Card>
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="font-mono text-sm">{tk.leadId}</p>
                  <p className="text-xs text-gray-600">{tk.reason.replaceAll("_", " ")} · {new Date(tk.createdAt).toLocaleString()}{tk.partner ? ` · ${tk.partner}` : ""}</p>
                </div>
                {tk.probability !== undefined && <p className="text-lg font-bold text-brand-900">{(tk.probability * 100).toFixed(1)}% <span className="text-xs font-normal text-gray-600">{tk.band}</span></p>}
              </div>
              <p className="mt-2">{tk.summary}</p>
              {tk.reasons && tk.reasons.length > 0 && <p className="mt-1 text-xs text-gray-600">{tk.reasons.map((r) => r.text).join(" · ")}</p>}
              <button onClick={() => setOpen(open === tk.id ? null : tk.id)} className="mt-2 text-sm text-brand-700 underline">
                {open === tk.id ? "Hide" : "Show"} conversation ({tk.transcript.length})
              </button>
              {open === tk.id && (
                <ol className="mt-2 space-y-1 text-sm">
                  {tk.transcript.map((m, i) => <li key={i}><span className="font-semibold">{m.role}:</span> {m.text}</li>)}
                </ol>
              )}
              <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                {tk.status === "open" && <Button onClick={() => act(() => api.scoring.claim(tk.id, "partner-demo"))}>Claim</Button>}
                {tk.status !== "closed" && <Button variant="secondary" onClick={() => act(() => api.scoring.close(tk.id))}>Close</Button>}
              </div>
            </Card>
          </li>
        ))}
      </ul>
    </>
  );
}
