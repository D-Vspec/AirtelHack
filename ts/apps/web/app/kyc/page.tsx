"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { type KycApplication, type KycDocumentType, type KycFinding, type Prefill, type StringKey } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Shell, Spinner } from "@/components/ui";
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

  return (
    <Shell step={3} title={t("stepKyc")}>
      {!j.consented && (
        <div className="mb-4"><Alert tone="warning">{t("consentTitle")}: <Link className="underline" href="/consent">{t("agree")}</Link></Alert></div>
      )}

      <Card className="mb-4">
        <form className="flex flex-col gap-2 sm:flex-row sm:items-end" onSubmit={(e) => { e.preventDefault(); void lookUp(); }}>
          <label className="flex-1">
            <span className="mb-1 block text-sm font-medium">{t("gstin")}</span>
            <input value={gstin} onChange={(e) => setGstin(e.target.value.toUpperCase())} maxLength={15} autoCapitalize="characters" inputMode="text"
              className="min-h-12 w-full rounded-xl border border-gray-300 px-3 font-mono text-base tracking-wider" placeholder="27XUIXG0767D1ZP" />
          </label>
          <Button variant="secondary" disabled={gstin.length < 15 || busy === "prefill"}>{busy === "prefill" && <Spinner />}{t("lookUp")}</Button>
        </form>
        {prefill === "none" && <p className="mt-2 text-sm text-amber-800">{t("prefillNotFound")}</p>}
        {prefill && prefill !== "none" && (
          <div className="mt-3"><Alert tone={prefill.status === "Active" ? "success" : "error"}>
            <p>{t("prefillFound", { name: prefill.tradeName ?? "", status: prefill.status ?? "" })}</p>
            <p className="text-sm">{prefill.legalName} · {prefill.constitution}</p>
            <p className="text-sm">{prefill.address}</p>
          </Alert></div>
        )}
      </Card>

      <div className="grid gap-3 sm:grid-cols-2">
        {DOCS.map(({ type, label }) => {
          // after a pass the images are deleted (privacy), so the card shows the verified state instead
          const u = uploaded[type] ?? (result?.status === "pass" ? { findings: [] } : undefined);
          const fs = [...(u?.findings ?? []), ...findingsFor(type)];
          return (
            <Card key={type} className="flex flex-col gap-3">
              <div className="flex items-center justify-between gap-2">
                <h2 className="font-semibold">{t(label)}</h2>
                {u && <span className="text-sm text-green-700">✓ {t("uploaded")}</span>}
              </div>
              {u?.preview && (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={u.preview} alt={t(label)} className="h-32 w-full rounded-lg object-cover object-top ring-1 ring-gray-200" />
              )}
              <label className={`flex min-h-12 cursor-pointer items-center justify-center gap-2 rounded-xl border-2 border-dashed px-3 text-center ${u ? "border-gray-200 text-gray-600" : "border-brand-600 text-brand-700"}`}>
                {busy === type ? <Spinner /> : "📷"} {t("takePhoto")}
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
        <button onClick={withdraw} disabled={!!busy} className="text-sm text-red-700 underline">{t("withdraw")}</button>
      </div>

      {error && <div className="mt-4"><Alert tone="error">{error}</Alert></div>}
      {busy === "check" && <div className="mt-4"><Spinner label={t("checkingDocs")} /></div>}
      {result && statusText && (
        <div className="mt-4 space-y-2" data-testid="kyc-result" data-status={result.status}>
          <Alert tone={statusTone}><p className="font-semibold">{statusText}</p></Alert>
          {result.findings?.filter((f) => !f.document).map((f, i) => <FindingItem key={i} f={f} lang={j.lang} />)}
        </div>
      )}

      <ActionBar>
        <Button variant={canTrack ? "secondary" : "primary"} disabled={!!busy || Object.keys(uploaded).length === 0} data-testid="check-docs" onClick={check}>{t("checkDocs")}</Button>
        {canTrack && <Button data-testid="go-track" onClick={() => router.push("/track")}>{t("stepTrack")} →</Button>}
      </ActionBar>
    </Shell>
  );
}
