import localFont from "next/font/local";

// Self-hosted Noto Sans (SIL OFL, app/fonts/OFL-LICENSE.txt): one variable-weight file per script.
// Each face carries its unicode-range, so a phone downloads only the scripts it renders (Latin plus,
// say, Tamil) instead of all nine. Only Latin is preloaded. next/font needs literal options, hence
// the repetition.

export const latin = localFont({
  src: "./fonts/noto-sans-latin-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-latin",
  declarations: [{ prop: "unicode-range", value: "U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+0304,U+0308,U+0329,U+2000-206F,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD" }],
});
export const devanagari = localFont({
  src: "./fonts/noto-sans-devanagari-devanagari-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-deva",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0900-097F,U+1CD0-1CF9,U+200C-200D,U+20A8,U+20B9,U+20F0,U+25CC,U+A830-A839,U+A8E0-A8FF" }],
});
export const bengali = localFont({
  src: "./fonts/noto-sans-bengali-bengali-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-beng",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0951-0952,U+0964-0965,U+0980-09FE,U+1CD0,U+1CD2,U+1CD5-1CD6,U+1CD8,U+1CE1,U+1CEA,U+1CED,U+1CF2,U+1CF5-1CF7,U+200C-200D,U+20B9,U+25CC,U+A8F1" }],
});
export const tamil = localFont({
  src: "./fonts/noto-sans-tamil-tamil-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-taml",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0964-0965,U+0B82-0BFA,U+200C-200D,U+20B9,U+25CC" }],
});
export const telugu = localFont({
  src: "./fonts/noto-sans-telugu-telugu-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-telu",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0951-0952,U+0964-0965,U+0C00-0C7F,U+1CDA,U+1CF2,U+200C-200D,U+25CC" }],
});
export const gujarati = localFont({
  src: "./fonts/noto-sans-gujarati-gujarati-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-gujr",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0951-0952,U+0964-0965,U+0A80-0AFF,U+200C-200D,U+20B9,U+25CC,U+A830-A839" }],
});
export const kannada = localFont({
  src: "./fonts/noto-sans-kannada-kannada-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-knda",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0951-0952,U+0964-0965,U+0C80-0CF3,U+1CD0,U+1CD2-1CD3,U+1CDA,U+1CF2,U+1CF4,U+200C-200D,U+20B9,U+25CC,U+A830-A835" }],
});
export const malayalam = localFont({
  src: "./fonts/noto-sans-malayalam-malayalam-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-mlym",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0307,U+0323,U+0951-0952,U+0964-0965,U+0D00-0D7F,U+1CDA,U+1CF2,U+200C-200D,U+20B9,U+25CC,U+A830-A832" }],
});
export const gurmukhi = localFont({
  src: "./fonts/noto-sans-gurmukhi-gurmukhi-wght-normal.woff2",
  weight: "100 900",
  display: "swap",
  variable: "--font-guru",
  preload: false,
  adjustFontFallback: false,
  declarations: [{ prop: "unicode-range", value: "U+0951-0952,U+0964-0965,U+0A01-0A76,U+200C-200D,U+20B9,U+25CC,U+262C,U+A830-A839" }],
});

export const fontVariables = [latin, devanagari, bengali, tamil, telugu, gujarati, kannada, malayalam, gurmukhi].map((f) => f.variable).join(" ");
