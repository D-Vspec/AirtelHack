"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { type KycApplication, type KycDocumentType, type KycFinding, type Prefill, type StringKey } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Pill, Shell, Skeleton, Spinner } from "@/components/ui";
import { IconArrow, IconCamera, IconCheck, IconShield } from "@/components/icons";
import { api, errorText, useJourney } from "@/lib/journey";
import { FindingItem } from "@/components/finding";

const DOCS: { type: KycDocumentType; label: StringKey }[] = [
  { type: "gst_certificate", label: "docGst" },
  { type: "udyam_certificate", label: "docUdyam" },
  { type: "address_proof", label: "docAddress" },
  { type: "signatory_id", label: "docSignatory" },
];

/** Phone photos can be 5-12 MB. Shrink to at most 2000 px and re-encode as JPEG on the device:
 *  faster on slow networks, still sharp enough for OCR. PNGs under 2 MB are sent as they are. */
async function shrink(file: File): Promise<Blob> {
  if (file.type === "image/png" && file.size < 2_000_000) return file;
  const bitmap = await createImageBitmap(file);
  const scale = Math.min(1, 2000 / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement("canvas");
  canvas.width = Math.round(bitmap.width * scale);
  canvas.height = Math.round(bitmap.height * scale);
  canvas.getContext("2d")!.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  return new Promise((resolve) => canvas.toBlob((b) => resolve(b ?? file), "image/jpeg", 0.9));
}

export default function KycPage() {
  const { j, ready, update, t } = useJourney();
  const router = useRouter();
  const [uploaded, setUploaded] = useState<Partial<Record<KycDocumentType, { preview?: string; findings: KycFinding[] }>>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<KycApplication | null>(null);
  const [gstin, setGstin] = useState("");
  const [prefill, setPrefill] = useState<Prefill | null | "none">(null);

  // resume: show what is already on file
  useEffect(() => {
    if (!ready) return;
    api.kyc.application(j.leadId).then((a) => {
      setUploaded(Object.fromEntries(a.documents.map((d) => [d, { findings: [] }])));
      if (a.status !== "collecting") setResult(a);
    }).catch(() => {});
  }, [ready, j.leadId]);

  async function lookUp() {
    setBusy("prefill");
    try {
      setPrefill(await api.kyc.prefill(gstin));
    } catch {
      setPrefill("none");
    } finally {
      setBusy(null);
    }
  }

  async function upload(type: KycDocumentType, file: File) {
    setBusy(type);
    setError(null);
    try {
      const blob = await shrink(file);
      const r = await api.kyc.upload(j.leadId, type, blob);
      setUploaded((u) => ({ ...u, [type]: { preview: URL.createObjectURL(blob), findings: r.findings } }));
    } catch (e) {
      setError(errorText(e, t("error")));
    } finally {
      setBusy(null);
    }
  }

  async function useSample() {
    setBusy("sample");
    setError(null);
    try {
      const r = await api.kyc.loadSpecimen(j.leadId, j.specimen ?? "K03");
      setUploaded(Object.fromEntries(r.documents.map((d) => [d, { findings: [] }])));
      setResult(null);
    } catch (e) {
      setError(errorText(e, t("error")));
    } finally {
      setBusy(null);
    }
  }

  async function check() {
    setBusy("check");
    setError(null);
    try {
      const r = await api.kyc.check(j.leadId);
      setResult(r);
      setUploaded(Object.fromEntries(r.documents.map((d) => [d, uploaded[d] ?? { findings: [] }])));
    } catch (e) {
      setError(errorText(e, t("error")));
    } finally {
      setBusy(null);
    }
  }

  async function withdraw() {
    setBusy("withdraw");
    try {
      await api.kyc.withdraw(j.leadId);
      update({ consented: false });
      setUploaded({});
      setResult(null);
      router.push("/consent");
    } catch (e) {
      setError(errorText(e, t("error")));
      setBusy(null);
    }
  }

  const statusText = result && ({ pass: t("kycPass"), manual_review: t("kycReview"), reupload: t("kycReupload"), reject: t("kycReject") } as Record<string, string>)[result.status];
  const statusTone = result?.status === "pass" ? "success" : result?.status === "reject" ? "error" : result?.status === "reupload" ? "warning" : "info";
  const canTrack = result && (result.status === "pass" || result.status === "manual_review");
  const findingsFor = (d: KycDocumentType) => (result?.findings ?? []).filter((f) => f.document === d);
  const doneCount = DOCS.filter(({ type }) => uploaded[type] || result?.status === "pass").length;

  return (
    <Shell step={3} title={t("stepKyc")}>
      {!j.consented && (
        <div className="mb-4"><Alert tone="warning">{t("consentTitle")}: <Link className="font-semibold underline" href="/consent">{t("agree")}</Link></Alert></div>
      )}

      <div className="mb-4 grid gap-4 md:grid-cols-[auto_1fr]">
        <Card className="flex items-center gap-4">
          <ProgressRing done={doneCount} total={DOCS.length} />
          <div>
            <p className="text-sm text-ink-500">{t("stepKyc")}</p>
            <p className="text-lg font-bold">{doneCount} / {DOCS.length}</p>
          </div>
        </Card>
        <Card>
          <form className="flex flex-col gap-2 sm:flex-row sm:items-end" onSubmit={(e) => { e.preventDefault(); void lookUp(); }}>
            <label className="flex-1">
              <span className="mb-1 block text-sm font-semibold">{t("gstin")}</span>
              <input value={gstin} onChange={(e) => setGstin(e.target.value.toUpperCase())} maxLength={15} autoCapitalize="characters" inputMode="text"
                className="min-h-12 w-full rounded-2xl border-0 bg-ink-50 px-4 font-mono text-base tracking-[0.15em] ring-1 ring-ink-200 outline-none focus:ring-brand-500" placeholder="27XUIXG0767D1ZP" />
            </label>
            <Button variant="dark" disabled={gstin.length < 15 || busy === "prefill"}>{busy === "prefill" && <Spinner />}{t("lookUp")}</Button>
          </form>
          {prefill === "none" && <p className="mt-2 text-sm text-amber-800">{t("prefillNotFound")}</p>}
          {prefill && prefill !== "none" && (
            <div className="mt-3 animate-rise"><Alert tone={prefill.status === "Active" ? "success" : "error"}>
              <p className="font-semibold">{t("prefillFound", { name: prefill.tradeName ?? "", status: prefill.status ?? "" })}</p>
              <p className="text-sm">{prefill.legalName} · {prefill.constitution}</p>
              <p className="text-sm">{prefill.address}</p>
            </Alert></div>
          )}
        </Card>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {DOCS.map(({ type, label }) => {
          // after a pass the images are deleted (privacy), so the card shows the verified state instead
          const u = uploaded[type] ?? (result?.status === "pass" ? { findings: [] } : undefined);
          const fs = [...(u?.findings ?? []), ...findingsFor(type)];
          const problem = fs.some((f) => f.route !== "pass");
          return (
            <Card key={type} className="flex flex-col gap-3">
              <div className="flex items-center gap-3">
                <span className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${u && !problem ? "bg-mint-50 text-mint-600" : "bg-brand-50 text-brand-700"}`}>
                  {u && !problem ? <IconCheck size={20} /> : <IconCamera size={20} />}
                </span>
                <h2 className="min-w-0 flex-1 font-semibold leading-tight">{t(label)}</h2>
                {u && <Pill tone={problem ? "amber" : "mint"}>{problem ? "!" : "✓"} {t("uploaded")}</Pill>}
              </div>
              {u?.preview && (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={u.preview} alt={t(label)} className="h-32 w-full rounded-xl object-cover object-top ring-1 ring-ink-200" />
              )}
              <label className={`flex min-h-12 cursor-pointer items-center justify-center gap-2 rounded-2xl px-3 text-center text-[15px] font-semibold transition ${u ? "bg-ink-50 text-ink-700 hover:bg-ink-100" : "border-2 border-dashed border-brand-200 bg-brand-50/50 text-brand-700 hover:border-brand-500"}`}>
                {busy === type ? <Spinner /> : <IconCamera size={18} />} {t("takePhoto")}
                {/* capture=environment opens the rear camera on phones; desktop gets a file picker */}
                <input type="file" accept="image/png,image/jpeg" capture="environment" className="sr-only" disabled={!!busy}
                  onChange={(e) => { const f = e.target.files?.[0]; if (f) void upload(type, f); e.target.value = ""; }} />
              </label>
              {fs.map((f, i) => <FindingItem key={`${f.code}-${i}`} f={f} lang={j.lang} />)}
            </Card>
          );
        })}
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <Button data-testid="use-sample" variant="ghost" disabled={!!busy} onClick={useSample}>{busy === "sample" && <Spinner />}{t("useSample")} ({j.specimen ?? "K03"})</Button>
        <button onClick={withdraw} disabled={!!busy} className="text-sm text-ink-500 underline hover:text-brand-700">{t("withdraw")}</button>
      </div>

      {error && <div className="mt-4"><Alert tone="error">{error}</Alert></div>}
      {busy === "check" && (
        <Card className="mt-4"><div className="flex items-center gap-3"><Spinner /> <span className="font-semibold">{t("checkingDocs")}</span></div><Skeleton className="mt-3 h-3 w-2/3" /></Card>
      )}
      {result && statusText && (
        <div className="mt-4 animate-rise space-y-2" data-testid="kyc-result" data-status={result.status}>
          <Alert tone={statusTone}><p className="text-[16px] font-bold">{statusText}</p></Alert>
          {result.findings?.filter((f) => !f.document).map((f, i) => <FindingItem key={i} f={f} lang={j.lang} />)}
        </div>
      )}

      <ActionBar>
        <Button variant={canTrack ? "secondary" : "primary"} disabled={!!busy || Object.keys(uploaded).length === 0} data-testid="check-docs" onClick={check}><IconShield size={18} />{t("checkDocs")}</Button>
        {canTrack && <Button data-testid="go-track" onClick={() => router.push("/track")}>{t("stepTrack")} <IconArrow size={18} /></Button>}
      </ActionBar>
    </Shell>
  );
}

function ProgressRing({ done, total }: { done: number; total: number }) {
  const r = 22;
  const c = 2 * Math.PI * r;
  const frac = total ? done / total : 0;
  return (
    <svg width="60" height="60" viewBox="0 0 60 60" aria-hidden="true">
      <circle cx="30" cy="30" r={r} fill="none" stroke="#ececf0" strokeWidth="7" />
      <circle cx="30" cy="30" r={r} fill="none" stroke={frac === 1 ? "#16a34a" : "#e40000"} strokeWidth="7" strokeLinecap="round"
        strokeDasharray={`${c * frac} ${c}`} transform="rotate(-90 30 30)" style={{ transition: "stroke-dasharray .4s ease" }} />
    </svg>
  );
}
