"use client";

import { useEffect, useState } from "react";
import type { KycReview, ReviewDecisionKind } from "@udyam/shared";
import { Alert, Button, Card, Spinner } from "@/components/ui";
import { api, errorText } from "@/lib/journey";

/** Manual review queue for KYC cases the pipeline routed to manual_review (K06, K07 patterns). */
export default function ReviewQueue() {
  const [reviews, setReviews] = useState<KycReview[] | null>(null);
  const [open, setOpen] = useState<KycReview | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = () => api.kyc.reviews("open").then((r) => setReviews(r.reviews)).catch((e) => setError(errorText(e, "Could not load the queue")));
  useEffect(() => { void load(); }, []);

  async function decide(decision: ReviewDecisionKind) {
    if (!open) return;
    setBusy(true);
    try {
      await api.kyc.decide(open.id, { decision, reviewer: "ops-demo", note: note || undefined });
      setOpen(null);
      setNote("");
      await load();
    } catch (e) {
      setError(errorText(e, "Decision failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-4 lg:grid-cols-[360px_1fr]">
      <div>
        <h1 className="mb-3 text-xl font-bold">KYC review queue</h1>
        {!reviews && <Spinner />}
        {reviews?.length === 0 && <Card>Queue is empty.</Card>}
        <ul className="space-y-2">
          {reviews?.map((r) => (
            <li key={r.id}>
              <button
                onClick={() => api.kyc.review(r.id).then(setOpen)}
                className={`w-full rounded-xl bg-white p-3 text-left ring-1 ${open?.id === r.id ? "ring-brand-600" : "ring-gray-200"}`}
              >
                <span className="block truncate font-mono text-sm">{r.leadId}</span>
                <span className="block text-xs text-gray-600">{r.codes.join(", ")} · {new Date(r.createdAt).toLocaleString()}</span>
              </button>
            </li>
          ))}
        </ul>
      </div>
      <div>
        {error && <div className="mb-3"><Alert tone="error">{error}</Alert></div>}
        {open && (
          <Card>
            <h2 className="font-semibold">Lead {open.leadId}</h2>
            <ul className="mt-3 space-y-2">
              {open.findings?.map((f, i) => (
                <li key={i} className="rounded-lg bg-gray-50 p-3 text-sm">
                  <span className="font-mono font-semibold">{f.code}</span> <span className="text-gray-500">[{f.route}] {f.document}</span>
                  <p>{f.detail}</p>
                </li>
              ))}
            </ul>
            <textarea value={note} onChange={(e) => setNote(e.target.value)} placeholder="Note for the audit trail" rows={2}
              className="mt-3 w-full rounded-xl border border-gray-300 p-2" />
            <div className="mt-3 flex flex-col gap-2 sm:flex-row">
              <Button disabled={busy} onClick={() => decide("approve")}>Approve</Button>
              <Button disabled={busy} variant="secondary" onClick={() => decide("request_reupload")}>Ask for re-upload</Button>
              <Button disabled={busy} variant="danger" onClick={() => decide("reject")}>Reject</Button>
            </div>
          </Card>
        )}
      </div>
    </div>
  );
}
