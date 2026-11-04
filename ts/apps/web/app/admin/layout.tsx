import Link from "next/link";

const TABS = [
  { href: "/admin/leads", label: "Leads by score" },
  { href: "/admin/handoff", label: "Partner handoff" },
  { href: "/admin", label: "Orders" },
  { href: "/admin/reviews", label: "KYC review queue" },
  { href: "/admin/messages", label: "Messages" },
];

export default function AdminLayout({ children }: LayoutProps<"/admin">) {
  return (
    <div className="min-h-screen">
      <header className="border-b border-gray-200 bg-brand-900 text-white">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-2 px-4 py-3">
          <Link href="/admin" className="font-bold">Udyam Digital · Back office</Link>
          <Link href="/" className="text-sm underline">Owner journey</Link>
        </div>
        <nav className="mx-auto flex max-w-6xl gap-1 overflow-x-auto px-4 pb-2">
          {TABS.map((t) => (
            <Link key={t.href} href={t.href} className="whitespace-nowrap rounded-lg px-3 py-2 text-sm hover:bg-white/10">{t.label}</Link>
          ))}
        </nav>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">{children}</main>
    </div>
  );
}
