// Small inline stroke icons (24x24, currentColor). Inline SVG instead of an icon library keeps the
// bundle tiny for low-end phones and needs no network.
import type { SVGProps } from "react";

type P = SVGProps<SVGSVGElement> & { size?: number };

function Icon({ size = 20, children, ...p }: P & { children: React.ReactNode }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.9} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" {...p}>
      {children}
    </svg>
  );
}

export const IconChat = (p: P) => <Icon {...p}><path d="M21 12a8 8 0 0 1-11.6 7.1L4 20l1-4.4A8 8 0 1 1 21 12Z" /><path d="M8.5 11h.01M12 11h.01M15.5 11h.01" /></Icon>;
export const IconReceipt = (p: P) => <Icon {...p}><path d="M6 3h12v18l-3-2-3 2-3-2-3 2Z" /><path d="M9 8h6M9 12h6M9 16h3" /></Icon>;
export const IconShield = (p: P) => <Icon {...p}><path d="M12 3 4.5 6v5.5c0 4.6 3.2 8.4 7.5 9.5 4.3-1.1 7.5-4.9 7.5-9.5V6Z" /><path d="m9 12 2 2 4-4" /></Icon>;
export const IconCamera = (p: P) => <Icon {...p}><path d="M4 8h3l2-3h6l2 3h3v11H4Z" /><circle cx="12" cy="13" r="3.5" /></Icon>;
export const IconTruck = (p: P) => <Icon {...p}><path d="M3 6h11v10H3zM14 10h4l3 3v3h-7" /><circle cx="7" cy="17.5" r="1.8" /><circle cx="17" cy="17.5" r="1.8" /></Icon>;
export const IconCheck = (p: P) => <Icon {...p}><path d="m5 12.5 4.5 4.5L19 7.5" /></Icon>;
export const IconGlobe = (p: P) => <Icon {...p}><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3c2.8 3 2.8 15 0 18M12 3c-2.8 3-2.8 15 0 18" /></Icon>;
export const IconSparkle = (p: P) => <Icon {...p}><path d="M12 3v4M12 17v4M3 12h4M17 12h4M6 6l2.5 2.5M15.5 15.5 18 18M18 6l-2.5 2.5M8.5 15.5 6 18" /></Icon>;
export const IconBolt = (p: P) => <Icon {...p}><path d="M13 3 5 14h6l-1 7 8-11h-6Z" /></Icon>;
export const IconUser = (p: P) => <Icon {...p}><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></Icon>;
export const IconArrow = (p: P) => <Icon {...p}><path d="M5 12h14M13 6l6 6-6 6" /></Icon>;
export const IconBack = (p: P) => <Icon {...p}><path d="M19 12H5M11 6l-6 6 6 6" /></Icon>;
export const IconAlert = (p: P) => <Icon {...p}><path d="M12 4 2.5 20h19Z" /><path d="M12 10v4M12 17h.01" /></Icon>;
export const IconWifi = (p: P) => <Icon {...p}><path d="M2 9a15 15 0 0 1 20 0M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0" /><path d="M12 19.5h.01" /></Icon>;
export const IconLock = (p: P) => <Icon {...p}><rect x="5" y="11" width="14" height="10" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></Icon>;
export const IconChart = (p: P) => <Icon {...p}><path d="M4 20V10M10 20V4M16 20v-7M22 20H2" /></Icon>;
export const IconInbox = (p: P) => <Icon {...p}><path d="M3 13h5l1.5 3h5L16 13h5" /><path d="M5 5h14l2 8v6H3v-6Z" /></Icon>;
export const IconUsers = (p: P) => <Icon {...p}><circle cx="9" cy="8" r="3.5" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0M16 4.5a3.5 3.5 0 0 1 0 7M18 14a6 6 0 0 1 3.5 6" /></Icon>;
export const IconHeart = (p: P) => <Icon {...p}><path d="M4 13h3l2-5 3 10 2-5h6" /></Icon>;
export const IconMessage = (p: P) => <Icon {...p}><path d="M4 5h16v11H8l-4 4Z" /></Icon>;
