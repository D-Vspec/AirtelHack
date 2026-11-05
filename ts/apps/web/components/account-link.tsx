"use client";

import Link from "next/link";
import { IconUser } from "./icons";
import { at, useAccount } from "@/lib/account";
import { useJourney } from "@/lib/journey";

/** Header link to the customer's account: a green dot once signed in. */
export function AccountLink() {
  const { j } = useJourney();
  const { account } = useAccount();
  return (
    <Link href="/account" data-testid="account-link"
      className="relative flex min-h-10 items-center gap-1.5 rounded-full bg-white/10 px-3 text-sm font-medium text-white ring-1 ring-white/15 hover:bg-white/20">
      <IconUser size={16} /> <span className="hidden sm:inline">{at(j.lang, "account")}</span>
      {account && <span className="absolute -top-0.5 -right-0.5 h-2.5 w-2.5 rounded-full bg-mint-600 ring-2 ring-ink-900" aria-label="signed in" />}
    </Link>
  );
}
