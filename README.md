# Udyam Digital (YTL Hackathon 2026, Track A)

Phone-first onboarding for small-business owners: AI advisor, transparent quote, smart KYC, order tracker. Stack: Go, Kafka, Postgres, LangChain.js, Next.js, Expo React Native, TypeScript, Tailwind.

## Prerequisites
Docker with Compose v2, Go 1.22+, Node 22+, pnpm (`npm i -g pnpm`), Python 3.10+ (scripts only).

## Quickstart (Phase 0)
```bash
make setup      # creates .env, runs go mod tidy (creates go.sum) and pnpm install
make up         # builds and starts Postgres, Kafka, topic init, Kafka UI, 8 Go services, advisor
make smoke      # health of every service + one event gateway -> Kafka -> notify -> Postgres -> HTTP
cd ts && pnpm dev:web      # http://localhost:3000 (shows service health, Kafka ping button)
cd ts && pnpm dev:mobile   # Expo; set EXPO_PUBLIC_API_URL to http://<your-LAN-IP>:8080/api on a real phone
```
Useful: `make logs SVC=notify`, `make ps`, Kafka UI at http://localhost:8089, gateway at http://localhost:8080/api/health.

## Layout
- `go/` one Go module: `cmd/<service>` entry points, `pkg/` shared (config, logx, events, kafkax, pg, svc)
- `ts/` pnpm workspace: `packages/shared`, `services/advisor`, `apps/web` (Next.js), `apps/mobile` (Expo)
- `contracts/` event envelope, topics, KYC finding codes, profile schema. Edit, then `make contracts`
- `infra/` Postgres init (schema per service), Kafka topic creation
- `data/` case CSVs and 47 KYC images; `data/kyc_expected.json` is our own expected-results table
- `docs/` plan, KYC findings, scenarios, data dictionary, changelog
- `prompts/` the shared preamble and one prompt per phase; `scripts/` smoke test, packaging, generators

## Ports
gateway 8080, registry 8081, rules 8082, quote 8083, kyc 8084, tracker 8085, notify 8086, scoring 8087, advisor 8090, Kafka UI 8089, Postgres 5432, Kafka 9092 (host) / kafka:19092 (containers), web 3000.

## What was verified when this was built
Go code vetted, built and unit-tested against stub Kafka/pgx modules; TypeScript typechecked for all four packages; advisor tests pass; Next.js production build succeeds. NOT verified in the authoring sandbox: `docker compose up`, the real kafka-go and pgx builds, `make smoke`, Expo on a device. See docs/CHANGELOG.md. If `make up` fails, send the first error from `docker compose logs`.

## Troubleshooting
- Go build fails on missing go.sum: run `make deps`.
- Kafka clients on the host cannot connect: the host listener is localhost:9092; containers use kafka:19092.
- pnpm refuses a very new package: the install policy rejects releases younger than about a day; pin an older version (Next is pinned to 16.4.2 for this reason).
- Expo cannot reach the API from a phone: use the computer's LAN IP, not localhost.

## Phases
0 Foundation (this repo) - 1 Data and registry - 2 Rules and quote - 3 Advisor - 4 KYC - 5 Orders and tracker - 6 Frontends - 7 Scoring and handoff - 8 Submission. Each phase: paste prompts/00-preamble.md plus the phase prompt into your coding agent, then `make package PHASE=<n>`.
