"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { DEMO_OWNERS, inr, type LangCode, type Quote } from "@udyam/shared";
import { ActionBar, Alert, Button, Card, Pill, Shell } from "@/components/ui";
import { IconArrow, IconReceipt, IconSparkle, IconUser } from "@/components/icons";
import { api, errorText, useJourney } from "@/lib/journey";

// Quick replies in the owner's language (others use English; the advisor understands all ten).
const CHIPS: Partial<Record<LangCode, { ask: string[]; offer: string[] }>> = {
  en: { ask: ["We have GSTIN", "1 branch", "5 staff", "Only a mobile plan"], offer: ["Go ahead", "Too expensive", "Only a mobile plan"] },
  hi: { ask: ["GSTIN है", "एक ही दुकान", "5 लोग काम करते हैं", "सिर्फ़ मोबाइल प्लान"], offer: ["ठीक है, आगे बढ़ें", "बहुत महँगा है", "सिर्फ़ मोबाइल प्लान"] },
  ta: { ask: ["GSTIN உள்ளது", "ஒரே கிளை", "5 பேர் வேலை செய்கிறார்கள்", "மொபைல் பிளான் மட்டும்"], offer: ["சரி, தொடரலாம்", "விலை அதிகம்", "மொபைல் பிளான் மட்டும்"] },
};

function AdvisorAvatar() {
  return (
    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-ink-900 text-white" aria-hidden="true">
      <IconSparkle size={16} />
    </span>
  );
}

export default function ChatPage() {
  const { j, ready, update, t } = useJourney();
  const router = useRouter();
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const [handedOff, setHandedOff] = useState(false);
  const listRef = useRef<HTMLOListElement>(null);

  // demo owners start with their scenario message ready to send
  useEffect(() => {
    if (!ready || j.messages.length) return;
    const owner = DEMO_OWNERS.find((o) => o.id === j.demoOwnerId);
    if (owner) setInput(owner.firstMessage);
  }, [ready, j.demoOwnerId, j.messages.length]);

  useEffect(() => {
    if (ready && j.quoteId && !quote) api.quotes.get(j.quoteId).then(setQuote).catch(() => {});
  }, [ready, j.quoteId, quote]);

  // Scroll the message list itself, never the page: the chat window moves, the screen stays put.
  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: j.messages.length > 1 ? "smooth" : "auto" });
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

  async function handoff() {
    if (!j.sessionId) return;
    setBusy(true);
    try {
      await api.advisor.handoff(j.sessionId, "owner_asked_for_person");
      setHandedOff(true);
    } catch (e) {
      setError(errorText(e, t("error")));
    } finally {
      setBusy(false);
    }
  }

  const chips = (CHIPS[j.lang] ?? CHIPS.en)!;
  const chipSet = quote ? chips.offer : chips.ask;

  return (
    <Shell step={0} title={t("stepChat")}>
      <div className="grid gap-4 xl:grid-cols-[1fr_330px]">
        <Card className={`flex min-h-[420px] flex-col p-3 sm:p-5 ${quote ? "h-[calc(100dvh-19rem)]" : "h-[calc(100dvh-14rem)]"} md:h-[calc(100dvh-13rem)] lg:h-[calc(100dvh-11rem)]`}>
          <ol ref={listRef} className="-mx-1 flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto overscroll-contain px-1 pb-2" aria-live="polite">
            {j.messages.length === 0 && (
              <li className="flex items-start gap-2">
                <AdvisorAvatar />
                <span className="rounded-2xl rounded-tl-md bg-ink-50 px-4 py-2.5 text-ink-700">{t("tagline")}</span>
              </li>
            )}
            {j.messages.map((m, i) =>
              m.role === "owner" ? (
                <li key={i} className="animate-rise ml-auto max-w-[85%] whitespace-pre-line rounded-2xl rounded-tr-md bg-brand-600 px-4 py-2.5 text-white shadow-lift sm:max-w-[72%]">
                  {m.text}
                </li>
              ) : (
                <li key={i} className="animate-rise flex max-w-[92%] items-start gap-2 sm:max-w-[80%]">
                  <AdvisorAvatar />
                  <span className="whitespace-pre-line rounded-2xl rounded-tl-md bg-ink-50 px-4 py-2.5 text-ink-900 ring-1 ring-ink-100">{m.text}</span>
                </li>
              ),
            )}
            {busy && (
              <li className="flex items-center gap-2" role="status" aria-label={t("thinking")}>
                <AdvisorAvatar />
                <span className="flex gap-1 rounded-2xl bg-ink-50 px-4 py-3">
                  <span className="typing-dot h-2 w-2 rounded-full bg-ink-500" />
                  <span className="typing-dot h-2 w-2 rounded-full bg-ink-500" />
                  <span className="typing-dot h-2 w-2 rounded-full bg-ink-500" />
                </span>
              </li>
            )}
          </ol>
          {error && <div className="mt-3"><Alert tone="error">{error}</Alert></div>}
          {handedOff && <div className="mt-3" data-testid="handoff-done"><Alert tone="success">{t("handoffDone")}</Alert></div>}

          <div className="mt-4 flex flex-wrap gap-2" aria-label="quick replies">
            {chipSet.map((c) => (
              <button key={c} disabled={busy} onClick={() => send(c)}
                className="min-h-10 rounded-full bg-white px-3.5 text-sm font-medium text-ink-900 ring-1 ring-ink-200 transition hover:bg-brand-50 hover:ring-brand-200 disabled:opacity-50">
                {c}
              </button>
            ))}
          </div>
          <form className="mt-3 flex items-end gap-2 rounded-2xl bg-ink-50 p-2 ring-1 ring-ink-100 focus-within:ring-brand-500" onSubmit={(e) => { e.preventDefault(); void send(input); }}>
            <label className="sr-only" htmlFor="msg">{t("chatPlaceholder")}</label>
            <textarea
              id="msg"
              rows={2}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); void send(input); } }}
              placeholder={t("chatPlaceholder")}
              className="min-h-12 flex-1 resize-none bg-transparent px-2 py-2 text-base outline-none"
            />
            <Button data-testid="send" type="submit" disabled={busy || !input.trim()} className="px-4" aria-label={t("send")}>
              <span className="hidden sm:inline">{t("send")}</span><IconArrow size={18} />
            </Button>
          </form>
          {j.sessionId && !handedOff && (
            <button data-testid="talk-to-person" onClick={handoff} disabled={busy} className="mt-3 inline-flex items-center gap-1.5 self-start text-sm font-medium text-ink-500 hover:text-brand-700 disabled:opacity-50">
              <IconUser size={16} /> {t("talkToPerson")}
            </button>
          )}
        </Card>

        {quote && (
          <aside className="animate-rise">
            <Card className="overflow-hidden p-0 xl:sticky xl:top-24">
              <div className="bg-ink-900 px-5 py-4 text-white">
                <p className="flex items-center gap-2 text-sm text-white/70"><IconReceipt size={16} /> {t("stepQuote")}</p>
                <p className="mt-1 text-3xl font-bold tracking-tight">{inr(quote.monthlyTotalInr)}<span className="text-base font-normal text-white/70"> {t("perMonth")}</span></p>
                <p className="mt-1 flex flex-wrap gap-1.5 text-xs">
                  <Pill tone="brand">{inr(quote.oneTimeTotalInr)} {t("oneTime")}</Pill>
                </p>
              </div>
              <ul className="divide-y divide-ink-100 px-5 py-2 text-sm">
                {quote.lines.map((l) => (
                  <li key={l.productId} className="flex justify-between gap-2 py-2">
                    <span>{l.productName}{l.quantity > 1 ? ` × ${l.quantity}` : ""}</span>
                    <span className="tabular-nums font-medium">{inr(l.monthlyInr)}</span>
                  </li>
                ))}
              </ul>
              <p className="border-t border-ink-100 px-5 py-3 text-xs text-ink-500">{t("installIn", { days: quote.expectedInstallDays })}</p>
            </Card>
          </aside>
        )}
      </div>
      {quote && (
        <ActionBar>
          <Button data-testid="see-quote" onClick={() => router.push("/quote")}>{t("seeQuote")} <IconArrow size={18} /></Button>
        </ActionBar>
      )}
    </Shell>
  );
}
