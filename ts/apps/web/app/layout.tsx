import type { Metadata, Viewport } from "next";
import "./globals.css";
import { fontVariables } from "./fonts";
import { JourneyProvider } from "@/lib/journey";

export const metadata: Metadata = {
  title: "Udyam Digital",
  description: "The right connectivity bundle for your business, in minutes.",
};

// Phones first: no zoom lock (accessibility), theme colour for the browser bar.
export const viewport: Viewport = { width: "device-width", initialScale: 1, themeColor: "#0f3d91" };

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${fontVariables} h-full antialiased`}>
      <body className="min-h-full">
        <JourneyProvider>{children}</JourneyProvider>
      </body>
    </html>
  );
}
