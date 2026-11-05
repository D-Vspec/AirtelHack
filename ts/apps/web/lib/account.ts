"use client";

import { useCallback, useEffect, useState } from "react";
import { accountText } from "@udyam/shared";
import { ACCOUNT_KEY, api } from "./journey";

export interface StoredAccount { token: string; mobileMasked: string }

function read(): StoredAccount | null {
  try { return JSON.parse(localStorage.getItem(ACCOUNT_KEY) ?? "null"); } catch { return null; }
}

/** The signed-in customer, kept in localStorage (only the session token and a masked number). */
export function useAccount() {
  const [account, setAccount] = useState<StoredAccount | null>(null);
  const [ready, setReady] = useState(false);
  useEffect(() => { setAccount(read()); setReady(true); }, []);
  const signIn = useCallback((a: StoredAccount) => { localStorage.setItem(ACCOUNT_KEY, JSON.stringify(a)); setAccount(a); }, []);
  const signOut = useCallback(async () => {
    try { await api.accounts.logout(); } catch { /* already expired */ }
    localStorage.removeItem(ACCOUNT_KEY);
    setAccount(null);
  }, []);
  return { account, ready, signIn, signOut };
}

// Text lives in @udyam/shared (account-text.ts) so the Expo app uses the same words.
export const at = accountText;
