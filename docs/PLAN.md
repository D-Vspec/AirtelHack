# Plan (short form)

The full plan, architecture and every phase prompt live in the Claude Doc "Project Udyam Digital - Build Plan, Architecture and Phase Prompts". This file is the short form kept in the repo. The prompts in /prompts are canonical for execution.

## Decisions
- Eligibility, pricing and KYC verdicts are deterministic Go code. The LLM converses, extracts a profile and explains; it never decides what is sellable.
- XGBoost is for lead propensity only (Phase 7). Training uses only features known at lead creation.
- Microservices at real seams only; one docker compose for the demo; the architecture diagram shows the national-scale topology.
- Contracts first (/contracts), generated constants (make contracts).

## Services
| Service | Language | Port | Schema |
|---|---|---|---|
| gateway | Go | 8080 | none |
| registry | Go | 8081 | registry |
| rules | Go | 8082 | catalogue |
| quote | Go | 8083 | quote |
| kyc | Go | 8084 | kyc |
| tracker | Go | 8085 | tracker |
| notify | Go | 8086 | notify |
| scoring | Go | 8087 | scoring |
| advisor | TypeScript + LangChain.js | 8090 | advisor |
| web | Next.js + Tailwind | 3000 | - |
| mobile | Expo React Native | - | - |

## Phases
0 Foundation (this commit) - 1 Data and registry - 2 Rules, catalogue, quote - 3 Advisor - 4 Smart KYC - 5 Orders, tracker, notifications - 6 Frontends - 7 Scoring and handoff - 8 Hardening and submission.

Cut order if short on time: scoring and handoff, then mobile (keep web), then channels beyond WhatsApp, then languages beyond Hindi, Tamil and English. Never cut: rules engine, KYC checks, advisor evals, business case, privacy note.
