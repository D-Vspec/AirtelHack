"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { createApiClient, newLeadId, t as translate, type BusinessProfile, type LangCode, type StringKey } from "@udyam/shared";

export const api = createApiClient(process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api");

export interface ChatMessage {
  role: "owner" | "advisor";
  text: string;
  kind?: string;
}

/** Everything the owner journey needs to survive a reload. Kept in localStorage: no personal documents, only IDs and the chat. */
export interface Journey {
  lang: LangCode;
  leadId: string;
  sessionId?: string;
  messages: ChatMessage[];
  profile: BusinessProfile;
  quoteId?: string;
  demoOwnerId?: string;
  specimen?: string;
  consented: boolean;
}

const KEY = "udyam.journey.v1";

const fresh = (lang: LangCode = "en", leadPrefix = "web"): Journey => ({
  lang, leadId: newLeadId(leadPrefix), messages: [], profile: {}, consented: false,
});

interface Ctx {
  j: Journey;
  ready: boolean;
  update: (patch: Partial<Journey> | ((j: Journey) => Partial<Journey>)) => void;
  reset: (opts?: { lang?: LangCode; leadPrefix?: string; demoOwnerId?: string; specimen?: string }) => void;
  t: (key: StringKey, params?: Record<string, string | number>) => string;
}

const JourneyContext = createContext<Ctx | null>(null);

export function JourneyProvider({ children }: { children: ReactNode }) {
  // No random lead ID during render (prerendering must be deterministic): it is assigned once
  // the app is running in the browser.
  const [j, setJ] = useState<Journey>({ lang: "en", leadId: "", messages: [], profile: {}, consented: false });
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let restored: Journey | null = null;
    try {
      const raw = localStorage.getItem(KEY);
      if (raw) restored = { ...fresh(), ...JSON.parse(raw) };
    } catch {
      /* corrupted storage: start fresh */
    }
    setJ(restored?.leadId ? restored : fresh(restored?.lang));
    setReady(true);
  }, []);

  useEffect(() => {
    if (ready) localStorage.setItem(KEY, JSON.stringify(j));
    document.documentElement.lang = j.lang;
  }, [j, ready]);

  const update = useCallback<Ctx["update"]>((patch) => {
    setJ((cur) => ({ ...cur, ...(typeof patch === "function" ? patch(cur) : patch) }));
  }, []);

  const reset = useCallback<Ctx["reset"]>((opts = {}) => {
    setJ((cur) => ({ ...fresh(opts.lang ?? cur.lang, opts.leadPrefix), demoOwnerId: opts.demoOwnerId, specimen: opts.specimen }));
  }, []);

  const value = useMemo<Ctx>(() => ({ j, ready, update, reset, t: (k, p) => translate(j.lang, k, p) }), [j, ready, update, reset]);
  return <JourneyContext.Provider value={value}>{children}</JourneyContext.Provider>;
}

export function useJourney(): Ctx {
  const c = useContext(JourneyContext);
  if (!c) throw new Error("useJourney outside JourneyProvider");
  return c;
}

/** Human message for any thrown error (ApiError carries the service's message). */
export function errorText(e: unknown, fallback: string): string {
  const detail = (e as { detail?: string })?.detail;
  return detail || fallback;
}
