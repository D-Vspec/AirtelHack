"use client";

import { useEffect, useState } from "react";
import { createApiClient, STRINGS_EN, type HealthResponse } from "@udyam/shared";

const api = createApiClient(process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api");

export default function Home() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pingResult, setPingResult] = useState<string | null>(null);

  useEffect(() => {
    api.health().then(setHealth).catch((e: unknown) => setError(String(e)));
  }, []);

  async function sendPing() {
    try {
      const { id } = await api.ping("hello from web");
      setPingResult(`event ${id} published`);
    } catch (e) {
      setPingResult(`ping failed: ${String(e)}`);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col gap-6 p-6">
      <header>
        <h1 className="text-2xl font-semibold">{STRINGS_EN.appName}</h1>
        <p className="text-zinc-600">{STRINGS_EN.tagline}</p>
      </header>

      <section className="rounded-lg border border-zinc-200 p-4">
        <h2 className="mb-2 font-medium">Phase 0 status</h2>
        {error && <p className="text-red-600">Gateway unreachable: {error}</p>}
        {!health && !error && <p>{STRINGS_EN.checking}</p>}
        {health && (
          <ul className="grid grid-cols-2 gap-1 text-sm">
            {Object.entries(health.services).map(([name, state]) => (
              <li key={name} className="flex justify-between">
                <span>{name}</span>
                <span className={state === "ok" ? "text-green-700" : "text-red-700"}>{state}</span>
              </li>
            ))}
          </ul>
        )}
        <button onClick={sendPing} className="mt-3 rounded bg-black px-3 py-1.5 text-sm text-white">
          Send Kafka ping
        </button>
        {pingResult && <p className="mt-2 text-sm">{pingResult}</p>}
      </section>
    </main>
  );
}
