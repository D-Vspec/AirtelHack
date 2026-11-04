"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { DEMO_OWNERS, inr, type LangCode, type Quote } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Shell, Spinner } from "@/components/ui";
import { api, errorText, useJourney } from "@/lib/journey";

// Quick replies in the owner's language (others use English; the advisor understands all ten).
const CHIPS: Partial<Record<LangCode, { ask: string[]; offer: string[] }>> = {
  en: { ask: ["We have GSTIN", "1 branch", "5 staff", "Only a mobile plan"], offer: ["Go ahead", "Too expensive", "Only a mobile plan"] },
  hi: { ask: ["GSTIN है", "एक ही दुकान", "5 लोग काम करते हैं", "सिर्फ़ मोबाइल प्लान"], offer: ["ठीक है, आगे बढ़ें", "बहुत महँगा है", "सिर्फ़ मोबाइल प्लान"] },
  ta: { ask: ["GSTIN உள்ளது", "ஒரே கிளை", "5 பேர் வேலை செய்கிறார்கள்", "மொபைல் பிளான் மட்டும்"], offer: ["சரி, தொடரலாம்", "விலை அதிகம்", "மொபைல் பிளான் மட்டும்"] },
};

export default function ChatPage() {
  const { j, ready, update, t } = useJourney();
  const router = useRouter();
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const endRef = useRef<HTMLDivElement>(null);

  // demo owners start with their scenario message ready to send
  useEffect(() => {
    if (!ready || j.messages.length) return;
    const owner = DEMO_OWNERS.find((o) => o.id === j.demoOwnerId);
    if (owner) setInput(owner.firstMessage);
  }, [ready, j.demoOwnerId, j.messages.length]);

  useEffect(() => {
    if (ready && j.quoteId && !quote) api.quotes.get(j.quoteId).then(setQuote).catch(() => {});
  }, [ready, j.quoteId, quote]);

  // braces matter: scrollIntoView returns a Promise in current browsers, and React treats a returned value as cleanup
  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "end" });
  }, [j.messages.length, busy]);

  async function send(text: string) {
    const message = text.trim();
    if (!message || busy) return;
    setError(null);
    setInput("");
    setBusy(true);
    update((cur) => ({ messages: [...cur.messages, { role: "owner", text: message }] }));
    try {
      const r = await api.advisor.chat({ message, sessionId: j.sessionId, leadId: j.leadId });
      update((cur) => ({
        sessionId: r.sessionId,
        profile: r.profile,
        quoteId: r.quote?.id ?? cur.quoteId,
        messages: [...cur.messages, { role: "advisor", text: r.reply, kind: r.kind }],
      }));
      if (r.quote) setQuote(r.quote);
    } catch (e) {
      setError(errorText(e, t("error")));
      setInput(message);
    } finally {
      setBusy(false);
    }
  }

  const chips = (CHIPS[j.lang] ?? CHIPS.en)!;
  const chipSet = quote ? chips.offer : chips.ask;

  return (
    <Shell step={0} title={t("stepChat")}>
      <div className="grid gap-4 xl:grid-cols-[1fr_320px]">
        <Card className="flex min-h-[50vh] flex-col">
          <ol className="flex flex-1 flex-col gap-3" aria-live="polite">
            {j.messages.length === 0 && <li className="text-gray-500">{t("tagline")}</li>}
            {j.messages.map((m, i) => (
              <li key={i} className={`max-w-[88%] whitespace-pre-line rounded-2xl px-4 py-2 sm:max-w-[75%] ${m.role === "owner" ? "self-end bg-brand-600 text-white" : "self-start bg-gray-100 text-gray-900"}`}>
                {m.text}
              </li>
            ))}
            {busy && <li className="self-start"><Spinner label={t("thinking")} /></li>}
          </ol>
          <div ref={endRef} />
          {error && <div className="mt-3"><Alert tone="error">{error}</Alert></div>}

          <div className="mt-4 flex flex-wrap gap-2" aria-label="quick replies">
            {chipSet.map((c) => (
              <button key={c} disabled={busy} onClick={() => send(c)} className="min-h-10 rounded-full bg-brand-50 px-3 text-sm text-brand-900 ring-1 ring-brand-100 hover:ring-brand-600 disabled:opacity-50">
                {c}
              </button>
            ))}
          </div>
          <form className="mt-3 flex gap-2" onSubmit={(e) => { e.preventDefault(); void send(input); }}>
            <label className="sr-only" htmlFor="msg">{t("chatPlaceholder")}</label>
            <textarea
              id="msg"
              rows={2}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void send(input); } }}
              placeholder={t("chatPlaceholder")}
              className="min-h-12 flex-1 resize-none rounded-xl border border-gray-300 px-3 py-2 text-base focus:border-brand-600 focus:outline-none"
            />
            <Button data-testid="send" type="submit" disabled={busy || !input.trim()}>{t("send")}</Button>
          </form>
        </Card>

        {quote && (
          <aside>
            <Card className="xl:sticky xl:top-24">
              <p className="text-sm text-gray-600">{t("stepQuote")}</p>
              <p className="text-3xl font-bold text-brand-900">{inr(quote.monthlyTotalInr)}<span className="text-base font-normal text-gray-600"> {t("perMonth")}</span></p>
              <p className="text-sm text-gray-600">{inr(quote.oneTimeTotalInr)} {t("oneTime")} · {t("installIn", { days: quote.expectedInstallDays })}</p>
              <ul className="mt-3 space-y-1 text-sm">
                {quote.lines.map((l) => (
                  <li key={l.productId} className="flex justify-between gap-2">
                    <span>{l.productName}{l.quantity > 1 ? ` × ${l.quantity}` : ""}</span>
                    <span className="tabular-nums text-gray-700">{inr(l.monthlyInr)}</span>
                  </li>
                ))}
              </ul>
            </Card>
          </aside>
        )}
      </div>
      {quote && (
        <ActionBar>
          <Button data-testid="see-quote" onClick={() => router.push("/quote")}>{t("seeQuote")} →</Button>
        </ActionBar>
      )}
    </Shell>
  );
}
