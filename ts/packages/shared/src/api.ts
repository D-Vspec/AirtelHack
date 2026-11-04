export class ApiError extends Error {
  constructor(public status: number, public body: unknown) {
    super(`API error ${status}`);
  }
}

export interface HealthResponse {
  status: "ok" | "degraded";
  services: Record<string, "ok" | "down">;
}

/** Minimal typed client for the gateway. Phase 6 extends it per service from /contracts. */
export function createApiClient(baseUrl: string) {
  const root = baseUrl.replace(/\/$/, "");
  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(`${root}${path}`, {
      ...init,
      headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
    });
    const text = await res.text();
    const body = text ? JSON.parse(text) : null;
    if (!res.ok) throw new ApiError(res.status, body);
    return body as T;
  }
  return {
    get: <T>(path: string) => request<T>(path),
    post: <T>(path: string, data?: unknown) => request<T>(path, { method: "POST", body: JSON.stringify(data ?? {}) }),
    health: () => request<HealthResponse>("/health"),
    ping: (message: string) => request<{ id: string }>("/system/ping", { method: "POST", body: JSON.stringify({ message }) }),
    pings: () => request<{ eventId: string; message: string; receivedAt: string }[]>("/notify/pings"),
  };
}
export type ApiClient = ReturnType<typeof createApiClient>;
