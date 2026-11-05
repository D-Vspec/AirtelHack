import type { BusinessProfile } from "./profile";
import type { Eligibility, Product, Quote } from "./advisor";
import type { AccountView, Consent, ConsentRequest, KycApplication, KycReview, OtpSent, Prefill, ReuseStatus, ReviewDecision, SignIn, UploadResult, Withdrawal } from "./kyc";
import type { KycDocumentType } from "./kyc-findings";
import type { Order, ResumeState, SentMessage, Timeline, Transition } from "./orders";
import type { HandoffReason, LeadScore, PartnerTask, ScoredLead } from "./scoring";

/** A non-2xx answer from the gateway. `body` is the service's error JSON ({code, message}) when there is one. */
export class ApiError extends Error {
  constructor(public status: number, public body: unknown) {
    super(`API error ${status}`);
  }
  /** The service's error code, e.g. CONSENT_REQUIRED, NOT_ELIGIBLE, DEPENDENCY. */
  get code(): string | undefined {
    return (this.body as { code?: string } | null)?.code;
  }
  /** The service's human-readable message, if any. */
  get detail(): string | undefined {
    return (this.body as { message?: string } | null)?.message;
  }
}

export interface HealthResponse {
  status: "ok" | "degraded";
  services: Record<string, "ok" | "down">;
}

/** POST /advisor/chat answer (ts/services/advisor/src/index.ts). */
export interface ChatResponse {
  sessionId: string;
  leadId: string;
  turn: number;
  reply: string;
  /** what the advisor did this turn: ask, recommend, mobile_only, cheaper, accept, out_of_scope, ... */
  kind: string;
  language: string;
  romanised: boolean;
  profile: BusinessProfile;
  quote: Quote | null;
}

export interface QuoteToggle {
  productId: string;
  enabled: boolean;
  quantity?: number;
}

/** Typed client for the gateway (contracts/*.schema.json). Every path is relative to NEXT_PUBLIC_API_URL / EXPO_PUBLIC_API_URL. */
/** adminToken: back-office X-Admin-Token. accountToken: a signed-in customer (Authorization: Bearer). */
export function createApiClient(baseUrl: string, opts: { adminToken?: () => string | null | undefined; accountToken?: () => string | null | undefined } = {}) {
  const root = baseUrl.replace(/\/$/, "");
  const adminHeader = (): Record<string, string> => {
    const token = opts.adminToken?.();
    const account = opts.accountToken?.();
    return { ...(token ? { "X-Admin-Token": token } : {}), ...(account ? { Authorization: `Bearer ${account}` } : {}) };
  };
  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const isJson = !(init?.body instanceof Blob) && !(init?.body instanceof ArrayBuffer);
    const res = await fetch(`${root}${path}`, {
      ...init,
      headers: { ...(isJson ? { "Content-Type": "application/json" } : {}), ...adminHeader(), ...(init?.headers ?? {}) },
    });
    const text = await res.text();
    let body: unknown = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch {
      body = { message: text };
    }
    if (!res.ok) throw new ApiError(res.status, body);
    return body as T;
  }
  const json = (method: string, data?: unknown): RequestInit => ({ method, body: JSON.stringify(data ?? {}) });
  const q = (params: Record<string, string | undefined>) => {
    const s = new URLSearchParams(Object.entries(params).filter(([, v]) => v) as [string, string][]).toString();
    return s ? `?${s}` : "";
  };

  return {
    get: <T>(path: string) => request<T>(path),
    post: <T>(path: string, data?: unknown) => request<T>(path, json("POST", data)),
    health: () => request<HealthResponse>("/health"),
    ping: (message: string) => request<{ id: string }>("/system/ping", json("POST", { message })),
    pings: () => request<{ eventId: string; message: string; receivedAt: string }[]>("/notify/pings"),

    advisor: {
      chat: (body: { message: string; sessionId?: string; leadId?: string }) => request<ChatResponse>("/advisor/chat", json("POST", body)),
      handoff: (sessionId: string, reason?: HandoffReason) =>
        request<{ leadId: string; reason: HandoffReason; summary: string }>("/advisor/handoff", json("POST", { sessionId, ...(reason ? { reason } : {}) })),
    },
    scoring: {
      score: (lead: Record<string, unknown>) => request<LeadScore>("/scoring/score", json("POST", lead)),
      leads: (params: { source?: "live" | "case"; limit?: number } = {}) =>
        request<{ leads: ScoredLead[] }>(`/scoring/leads${q({ source: params.source, limit: params.limit?.toString() })}`),
      tasks: (status?: "open" | "claimed" | "closed") => request<{ tasks: PartnerTask[] }>(`/scoring/tasks${q({ status })}`),
      claim: (id: string, partner: string) => request<PartnerTask>(`/scoring/tasks/${encodeURIComponent(id)}/claim`, json("POST", { partner })),
      close: (id: string) => request<PartnerTask>(`/scoring/tasks/${encodeURIComponent(id)}/close`, json("POST")),
    },
    rules: {
      products: () => request<Product[]>("/rules/products"),
      eligibility: (profile: BusinessProfile) => request<Eligibility>("/rules/eligibility", json("POST", { profile })),
    },
    quotes: {
      get: (id: string) => request<Quote>(`/quote/quotes/${encodeURIComponent(id)}`),
      toggle: (id: string, toggles: QuoteToggle[], contractMonths?: number) =>
        request<Quote>(`/quote/quotes/${encodeURIComponent(id)}`, json("PATCH", { toggles, ...(contractMonths ? { contractMonths } : {}) })),
      accept: (id: string) => request<Quote>(`/quote/quotes/${encodeURIComponent(id)}/accept`, json("POST")),
    },
    kyc: {
      consent: (body: ConsentRequest) => request<Consent>("/kyc/consents", json("POST", body)),
      getConsent: (leadId: string) => request<Consent>(`/kyc/consents/${encodeURIComponent(leadId)}`),
      withdraw: (leadId: string) => request<Withdrawal>(`/kyc/consents/${encodeURIComponent(leadId)}/withdraw`, json("POST")),
      prefill: (gstin: string) => request<Prefill>(`/kyc/prefill/${encodeURIComponent(gstin.trim())}`),
      upload: (leadId: string, type: KycDocumentType, image: Blob) =>
        request<UploadResult>(`/kyc/applications/${encodeURIComponent(leadId)}/documents/${type}`, {
          method: "PUT", body: image, headers: { "Content-Type": image.type || "application/octet-stream" },
        }),
      loadSpecimen: (leadId: string, applicant: string) =>
        request<{ leadId: string; applicant: string; documents: KycDocumentType[] }>(
          `/kyc/applications/${encodeURIComponent(leadId)}/specimens/${applicant}`, json("POST")),
      check: (leadId: string) => request<KycApplication>(`/kyc/applications/${encodeURIComponent(leadId)}/check`, json("POST")),
      application: (leadId: string) => request<KycApplication>(`/kyc/applications/${encodeURIComponent(leadId)}`),
      resumeLink: (leadId: string) => request<{ token: string; url: string; expiresAt: string }>(`/kyc/applications/${encodeURIComponent(leadId)}/resume-link`, json("POST")),
      resume: (token: string) => request<ResumeState>(`/kyc/resume/${encodeURIComponent(token)}`),
      reviews: (status?: "open" | "decided") => request<{ reviews: KycReview[] }>(`/kyc/reviews${q({ status })}`),
      review: (id: string) => request<KycReview>(`/kyc/reviews/${encodeURIComponent(id)}`),
      decide: (id: string, body: ReviewDecision) => request<KycReview>(`/kyc/reviews/${encodeURIComponent(id)}/decision`, json("POST", body)),
    },
    accounts: {
      requestCode: (mobile: string) => request<OtpSent>("/kyc/accounts/otp", json("POST", { mobile })),
      verify: (body: { mobile: string; code: string; leadId?: string }) => request<SignIn>("/kyc/accounts/verify", json("POST", body)),
      me: () => request<AccountView>("/kyc/accounts/me"),
      linkLead: (leadId: string) => request<{ leadId: string; accountId: string }>("/kyc/accounts/me/leads", json("POST", { leadId })),
      logout: () => request<{ signedOut: boolean }>("/kyc/accounts/me/logout", json("POST")),
      reuseStatus: (leadId?: string) => request<ReuseStatus>(`/kyc/accounts/me/kyc-reuse${q({ leadId })}`),
      reuse: (leadId: string, unchanged: boolean) =>
        request<KycApplication & { reusedFrom: string }>(`/kyc/applications/${encodeURIComponent(leadId)}/reuse`, json("POST", { unchanged })),
    },
    tracker: {
      orders: (params: { leadId?: string; status?: string } = {}) => request<{ orders: Order[] }>(`/tracker/orders${q(params)}`),
      timeline: (orderId: string) => request<Timeline>(`/tracker/orders/${encodeURIComponent(orderId)}/timeline`),
      transition: (orderId: string, body: Transition) => request<Order>(`/tracker/orders/${encodeURIComponent(orderId)}/transitions`, json("POST", body)),
      acceptFallback: (orderId: string) => request<Order>(`/tracker/orders/${encodeURIComponent(orderId)}/fallback/accept`, json("POST")),
    },
    /** Prometheus text from a service's /metrics (back office only). */
    metrics: async (service: string) => {
      const r = await fetch(`${root}/${service}/metrics`, { headers: adminHeader() });
      if (!r.ok) throw new ApiError(r.status, null);
      return r.text();
    },
    notify: {
      messages: (leadId?: string) => request<{ messages: SentMessage[] }>(`/notify/messages${q({ leadId })}`),
    },
  };
}
export type ApiClient = ReturnType<typeof createApiClient>;
