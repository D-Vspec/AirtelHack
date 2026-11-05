import Link from "next/link";
import { Suspense } from "react";
import { AdminGate } from "@/components/admin-gate";
import { AdminNav } from "@/components/admin-nav";

export default function AdminLayout({ children }: LayoutProps<"/admin">) {
  return (
    <div className="min-h-screen bg-[#f4f4f7]">
      <header className="bg-ink-900 text-white">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-2 px-4 py-3">
          <Link href="/admin" className="flex items-center gap-2 font-bold">
            <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-brand-600 text-sm">UD</span>
            Udyam Digital <span className="font-normal text-white/60">· Back office</span>
          </Link>
          <Link href="/" className="rounded-xl px-3 py-1.5 text-sm text-white/80 ring-1 ring-white/20 hover:bg-white/10">Owner journey →</Link>
        </div>
        <Suspense><AdminNav variant="top" /></Suspense>
      </header>
      <div className="mx-auto grid max-w-7xl gap-6 px-4 py-6 lg:grid-cols-[230px_1fr]">
        <Suspense><AdminNav variant="side" /></Suspense>
        <main className="min-w-0"><AdminGate>{children}</AdminGate></main>
      </div>
    </div>
  );
}
