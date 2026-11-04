# Instructions for coding agents

Read README.md, docs/PLAN.md and docs/kyc-findings.md first. The phase you are working on is described in prompts/ (shared preamble in prompts/00-preamble.md).

- Stack is fixed: Go 1.22 (one module in /go), Kafka, Postgres 16 (schema per service), LangChain.js + TypeScript (advisor), Next.js + Tailwind (web), Expo React Native (mobile), pnpm workspaces in /ts.
- Contracts first: edit /contracts, run `make contracts`, then write code. Never hand-edit *_gen.go or generated TS files.
- Eligibility, pricing and KYC verdicts are deterministic Go code. The LLM never decides them.
- Never log raw GSTIN, Udyam or ID numbers: use logx.MaskID.
- Keep `make up` and `make smoke` green. Add tests with every change. Do not claim something works unless you ran it.
- Finish with `make package PHASE=<n>` and report what was built, what was cut and open risks.
