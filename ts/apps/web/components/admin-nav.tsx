"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { IconChart, IconHeart, IconInbox, IconMessage, IconShield, IconTruck, IconUsers } from "@/components/icons";

const TABS = [
  { href: "/admin/leads", label: "Leads by score", icon: IconChart },
  { href: "/admin/handoff", label: "Partner handoff", icon: IconUsers },
  { href: "/admin", label: "Orders", icon: IconTruck },
  { href: "/admin/reviews", label: "KYC review queue", icon: IconShield },
  { href: "/admin/messages", label: "Messages", icon: IconMessage },
  { href: "/admin/status", label: "Status", icon: IconHeart },
];

/** Sidebar on desktop, scrollable pill tabs on phones. */
export function AdminNav({ variant }: { variant: "side" | "top" }) {
  const path = usePathname();
  const active = (href: string) => (href === "/admin" ? path === "/admin" : path.startsWith(href));
  if (variant === "top") {
    return (
      <nav className="flex gap-1.5 overflow-x-auto px-4 pb-3 lg:hidden" aria-label="back office">
        {TABS.map((t) => (
          <Link key={t.href} href={t.href}
            className={`flex shrink-0 items-center gap-1.5 rounded-full px-3 py-1.5 text-sm ${active(t.href) ? "bg-white text-ink-900" : "bg-white/10 text-white/80"}`}>
            <t.icon size={15} /> {t.label}
          </Link>
        ))}
      </nav>
    );
  }
  return (
    <nav className="hidden lg:block" aria-label="back office">
      <ul className="sticky top-6 space-y-1">
        {TABS.map((t) => (
          <li key={t.href}>
            <Link href={t.href}
              className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-[15px] transition ${active(t.href) ? "bg-white font-semibold text-ink-900 shadow-card" : "text-ink-700 hover:bg-white/70"}`}>
              <span className={`flex h-8 w-8 items-center justify-center rounded-lg ${active(t.href) ? "bg-brand-600 text-white" : "bg-ink-100 text-ink-700"}`}><t.icon size={16} /></span>
              {t.label}
            </Link>
          </li>
        ))}
        <li className="pt-4"><span className="flex items-center gap-2 px-3 text-xs text-ink-500"><IconInbox size={14} /> Accel Business ops</span></li>
      </ul>
    </nav>
  );
}
