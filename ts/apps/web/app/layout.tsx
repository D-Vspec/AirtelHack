import type { Metadata } from "next";
import "./globals.css";

// System font stack on purpose: no network fetch at build time (demo machines may be offline).
// Phase 6 adds self-hosted Noto Sans for Indic scripts via next/font/local.
export const metadata: Metadata = {
  title: "Udyam Digital",
  description: "The right connectivity bundle for your business, in minutes.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
