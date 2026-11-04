import type { Bundle, BusinessProfile, Eligibility, Product, Quote, QuoteItem, ServiceError } from "@udyam/shared";

export class ServiceCallError extends Error {
  constructor(public status: number, public body: ServiceError) {
    super(`${body.code}: ${body.message}`);
  }
}

export interface GstinLookup {
  matchType: "exact" | "corrected" | "none";
  id?: string;
  record?: { status: string; legalName: string; tradeName: string };
  suggestions?: string[];
  checksumValid: boolean;
}

/** The deterministic tools the advisor may use. Everything it recommends comes from here. */
export interface Services {
  products(): Promise<Product[]>;
  eligibility(profile: BusinessProfile): Promise<Eligibility>;
  bundles(profile: BusinessProfile): Promise<Bundle[]>;
  createQuote(req: { leadId?: string; profile: BusinessProfile; items: QuoteItem[]; contractMonths?: number }): Promise<Quote>;
  getQuote(id: string): Promise<Quote>;
  acceptQuote(id: string): Promise<Quote>;
  gstin(id: string): Promise<GstinLookup>;
}

export function httpServices(env: Record<string, string | undefined> = process.env): Services {
  const rules = env.RULES_URL ?? "http://localhost:8080/api/rules";
  const quote = env.QUOTE_URL ?? "http://localhost:8080/api/quote";
  const registry = env.REGISTRY_URL ?? "http://localhost:8080/api/registry";
  async function call<T>(url: string, init?: RequestInit, okStatuses = [200, 201]): Promise<T> {
    const res = await fetch(url, { ...init, headers: { "Content-Type": "application/json" }, signal: AbortSignal.timeout(8000) });
    const body = await res.json().catch(() => ({ code: "BAD_RESPONSE", message: res.statusText }));
    if (!okStatuses.includes(res.status)) throw new ServiceCallError(res.status, body as ServiceError);
    return body as T;
  }
  const post = (body: unknown): RequestInit => ({ method: "POST", body: JSON.stringify(body) });
  return {
    products: () => call(`${rules}/products`),
    eligibility: (profile) => call(`${rules}/eligibility`, post({ profile })),
    bundles: async (profile) => (await call<{ bundles: Bundle[] }>(`${rules}/bundles`, post({ profile }))).bundles,
    createQuote: (req) => call(`${quote}/quotes`, post(req)),
    getQuote: (id) => call(`${quote}/quotes/${encodeURIComponent(id)}`),
    acceptQuote: (id) => call(`${quote}/quotes/${encodeURIComponent(id)}/accept`, { method: "POST" }),
    // 404 is a normal "not in registry" answer.
    gstin: (id) => call(`${registry}/gstin/${encodeURIComponent(id)}`, undefined, [200, 404]),
  };
}
