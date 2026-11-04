# Changelog

## Phase 0 - Foundation
Added: docker compose (Postgres 16, Kafka KRaft, topic init, Kafka UI, 8 Go services, advisor), Go shared packages (config, logx, events, kafkax, pg, svc), gateway with reverse proxy, CORS, aggregated health and ping publisher, notify consumer, contracts + generator, TS workspace (shared, advisor, Next.js web, Expo mobile), data and KYC analysis, phase prompts, packaging scripts.

Verified in the authoring sandbox: `go vet`, `go build` and `go test` of pkg/events and pkg/logx against stub Kafka/pgx modules; pnpm install, typecheck of shared, advisor, web, mobile; advisor unit tests; `next build`; contract generator output formatting.

NOT verified there (no Docker, and Go module downloads were blocked): `docker compose up`, real kafka-go and pgx compile (run `make deps` to create go.sum), `make smoke`, the Expo app on a device. First run on your machine is the real test; fix anything that breaks and note it here.
