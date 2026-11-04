"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Button, Card } from "@/components/ui";
import { ADMIN_TOKEN_KEY } from "@/lib/journey";

/** Asks for the back-office token (gateway ADMIN_TOKEN) once and keeps it in this browser. */
export function AdminGate({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null | undefined>(undefined);
  const [input, setInput] = useState("");
  useEffect(() => setToken(localStorage.getItem(ADMIN_TOKEN_KEY)), []);

  if (token === undefined) return null;
  if (!token) {
    return (
      <Card className="mx-auto mt-6 max-w-md">
        <form onSubmit={(e) => { e.preventDefault(); localStorage.setItem(ADMIN_TOKEN_KEY, input.trim()); setToken(input.trim()); }}>
          <label className="block">
            <span className="mb-1 block font-semibold">Back-office token</span>
            <input data-testid="admin-token" type="password" value={input} onChange={(e) => setInput(e.target.value)} autoComplete="off"
              className="min-h-12 w-full rounded-xl border border-gray-300 px-3" />
          </label>
          <p className="mt-2 text-xs text-gray-600">The gateway&apos;s ADMIN_TOKEN (.env). Local demo default: demo-admin-token.</p>
          <Button className="mt-3 w-full" disabled={!input.trim()}>Continue</Button>
        </form>
      </Card>
    );
  }
  return (
    <>
      {children}
      <button onClick={() => { localStorage.removeItem(ADMIN_TOKEN_KEY); setToken(null); }} className="mt-8 text-xs text-gray-500 underline">
        Sign out of back office
      </button>
    </>
  );
}
