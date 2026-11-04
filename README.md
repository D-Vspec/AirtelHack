# Udyam Digital (YTL Hackathon 2026, Track A)

A phone-first onboarding journey for small-business owners:

- An AI advisor recommends a bundle in the owner's language.
- A transparent quote follows; eligibility and prices are deterministic code, never the AI.
- Smart KYC verifies documents against the GSTIN and Udyam registries, with consent first under the DPDP Act.
- An order tracker sends WhatsApp/SMS updates instead of making owners chase status.
- Leads the advisor cannot close go to partners, ranked by a propensity score.

Stack: Go, Kafka, Postgres, LangChain.js, Next.js, Expo React Native, TypeScript, Tailwind.

| Deliverable | File |
|---|---|
| Architecture (demo + national scale) | [docs/architecture.md](docs/architecture.md), [demo PNG](docs/architecture-demo.png), [national PNG](docs/architecture-national.png) |
| Business case (10 slides) | [docs/business-case.md](docs/business-case.md) |
| Privacy and security note (1 page) | [docs/privacy-security-note.md](docs/privacy-security-note.md) |
| 3-minute demo script and shot list | [docs/demo-script.md](docs/demo-script.md) |
| KYC findings for the 12 specimens | [docs/kyc-findings.md](docs/kyc-findings.md), [field comparison](docs/kyc-field-comparison.md) |
| Advisor eval | [docs/advisor-eval.md](docs/advisor-eval.md) |
| Lead score model card | [docs/model-card.md](docs/model-card.md) |
| Failure drills | [docs/failure-drills.md](docs/failure-drills.md) |
| Screenshots | [docs/screens](docs/screens) |
| What was built per phase, verified and cut | [docs/CHANGELOG.md](docs/CHANGELOG.md) |

## Prerequisites

- Docker Desktop (Compose v2) with about 15 GB free disk.
- Go 1.26+, Node 20+, pnpm (`npm i -g pnpm`, or `npx pnpm@12.16.1` in its place).
- GNU make and bash. On Windows, use Git Bash with `mingw32-make`, or run the commands each target prints.
- Python 3.10+ only to retrain the lead score.

## Quickstart

```bash
cp .env.example .env     # then set one LLM key, e.g. LLM_PROVIDER=deepseek and DEEPSEEK_API_KEY=sk-...
make up                  # builds and starts Postgres, Kafka, 8 Go services, simulator, advisor (first build ~5-10 min)
make seed                # loads data/*.csv into Postgres, writes docs/analytics.md
make smoke               # health of every service + one event through Kafka
make scenarios           # 33 user-story scenarios against the live stack (advisor ones are SKIPPED without an LLM key)
cd ts && pnpm install && cd apps/web && npx next build && npx next start -p 3000   # or: make web
```

Then open:
- **http://localhost:3000**: pick a language, or one of the demo owners.
- **http://localhost:3000/admin**: back office, token `demo-admin-token` (change `ADMIN_TOKEN` in `.env`).

Mobile: `cd ts && pnpm dev:mobile`, and on a real phone set `EXPO_PUBLIC_API_URL=http://<your-LAN-IP>:8080/api`.

### Other targets

| Target | What it does |
|---|---|
| `make test` | Go and TypeScript unit tests (the KYC 12-applicant table replays recorded OCR) |
| `make kyc-test` | The 12-applicant KYC table against real tesseract, in Docker |
| `make integration` | Lead → quote → KYC → activation with services and Kafka stopped mid-flow; scoring and handoff |
| `make drills` | Failure drills: Kafka down, kyc down, Postgres restart |
| `make web-e2e`, `make mobile-e2e` | The demo path in headless Edge at phone and desktop sizes; screenshots to docs/screens |
| `make model` | Retrain the lead score and check Go/Python parity |
| `make contracts` | Regenerate Go/TS constants from /contracts |

URLs: gateway http://localhost:8080/api/health, Kafka UI http://localhost:8089. `/metrics` on every service (via the gateway with `X-Admin-Token`); the admin Status page summarises them.

## Layout

| Path | Contents |
|---|---|
| `go/` | One module. `cmd/<service>` (gateway, registry, rules, quote, kyc, tracker, notify, scoring, simulator, seed, analytics, scenarios); `pkg/` (catalogue rules, kyc pipeline, orders state machine, outbox, scoring, registry index, svc runtime with metrics) |
| `ts/` | pnpm workspace: `packages/shared` (API client, types, 10-language strings), `services/advisor` (LangChain.js), `apps/web` (Next.js), `apps/mobile` (Expo) |
| `contracts/` | Event envelope, topics, KYC findings and messages, and HTTP shapes per service. Edit, then `make contracts` |
| `infra/` | Postgres schemas and per-service roles, Kafka topics |
| `ml/` | Lead score training (`train.py`, pinned requirements) |
| `data/` | Case CSVs, 47 KYC images, `kyc_expected.json` (our expected-results table) |

## Ports

gateway 8080, registry 8081, rules 8082, quote 8083, kyc 8084, tracker 8085, notify 8086, scoring 8087, advisor 8090, Kafka UI 8089, Postgres 5432, Kafka 9092 (host) / kafka:19092 (containers), web 3000. Only the gateway, Postgres, Kafka, Kafka UI and advisor are published on the host.

## What is verified, and what is not

**Verified** on Docker Desktop for Windows, with commands run, not assumed. Details per phase are in the changelog.
- `make up seed smoke scenarios` (33/33) from a fresh clone.
- `go test ./...`; TS typecheck and tests.
- KYC 12/12 with real tesseract.
- Advisor eval 27/27 on DeepSeek V4.1 Flash.
- Integration tests and failure drills.
- The web demo path in Edge at phone and desktop sizes; the Expo app's web build.
- `govulncheck` clean.

**Not verified:**
- The Expo app on a physical device (camera capture is typechecked only).
- Gemini and Anthropic advisor evals (no keys).
- The organisers' hidden KYC answer key.

**Not built** (listed by `make scenarios`):
- e-sign for an absent signatory
- partner attribution
- competitor contract-end reminders
- OTP at lead entry

## Troubleshooting

- `kafka-init` fails with `set: pipefail: invalid option name`: the scripts were checked out with Windows line endings. `.gitattributes` prevents this in new clones; in an old one run `git rm --cached -r . && git reset --hard` (this discards uncommitted changes).
- The advisor answers 503 `LLM_NOT_CONFIGURED`: no LLM key in `.env`. Run `docker compose up -d advisor` after adding it.
- Never put an inline comment after an empty value in `.env` (`KEY=   # comment`): Compose reads the comment as the value.
- 401 `ADMIN_TOKEN_REQUIRED`: back-office routes need `X-Admin-Token`; the web admin asks for it once.
- 429 `RATE_LIMITED`: per-IP limits at the gateway (`RATE_LIMIT_RPS`); advisor chat is limited to 0.5/s with a burst of 6.
- pnpm refuses a very new package: the install policy rejects releases younger than about a day; pin an older version.
- Expo cannot reach the API from a phone: use the computer's LAN IP, not localhost.
