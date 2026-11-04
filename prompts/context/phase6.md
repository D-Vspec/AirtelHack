# Context for Phase 6
- The web app is ts/apps/web (Next.js 16, App Router, Tailwind 4, system font stack, no network fonts). The mobile app is ts/apps/mobile (Expo, same React version as web). Both import @udyam/shared.
- Gateway base URL: NEXT_PUBLIC_API_URL / EXPO_PUBLIC_API_URL (http://localhost:8080/api). On a physical phone use the computer's LAN IP.
- pnpm uses node-linker=hoisted (see ts/.npmrc) so Metro works in the monorepo; keep React versions identical across web and mobile.
- Dependency note: a new Next.js release can be rejected by pnpm's release-age policy; the repo pins Next 16.4.2. Check release dates before upgrading.
- Journey: language -> advisor chat -> bundle and explanation -> quote -> consent -> KYC -> tracker. Admin: leads, unclosed leads, KYC review queue, partner handoff.
- Every KYC failure code in contracts/kyc-findings.json needs a human message and a next step in each language.
- Demo owners should match docs/scenarios.md.
