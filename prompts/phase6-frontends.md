Goal: web and mobile on the same API through the gateway.
1. ts/packages/shared: typed API client generated from contracts (extend api.ts), zod schemas, i18n strings for every language in LANGUAGES.
2. Web (ts/apps/web, Next.js App Router, TypeScript, Tailwind): a mobile-first owner journey with language picker, advisor chat with quick-reply chips, recommendation and explanation, quote with add-on toggles, consent screen, KYC capture with live per-finding feedback, and order tracker timeline. Add /admin: lead list, unclosed leads, KYC review queue, partner handoff view.
3. Mobile (ts/apps/mobile, Expo React Native): the same journey using the shared package, with camera capture for KYC.
4. Low-end phone performance, readable Indic fonts (self-host Noto Sans via next/font/local; no network fonts), and a clear message plus next step for every KYC failure code.
5. Demo mode with seeded sample owners matching docs/scenarios.md.
Done when: the 3-minute demo path works end to end on web, mobile completes the same path, and screenshots are saved in docs/screens.
