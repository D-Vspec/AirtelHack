# Changelog

## Phase 0 - Foundation
Added: docker compose (Postgres 16, Kafka KRaft, topic init, Kafka UI, 8 Go services, advisor), Go shared packages (config, logx, events, kafkax, pg, svc), gateway with reverse proxy, CORS, aggregated health and ping publisher, notify consumer, contracts + generator, TS workspace (shared, advisor, Next.js web, Expo mobile), data and KYC analysis, phase prompts, packaging scripts.

Verified in the authoring sandbox: `go vet`, `go build` and `go test` of pkg/events and pkg/logx against stub Kafka/pgx modules; pnpm install, typecheck of shared, advisor, web, mobile; advisor unit tests; `next build`; contract generator output formatting.

NOT verified there (no Docker, and Go module downloads were blocked): `docker compose up`, real kafka-go and pgx compile (run `make deps` to create go.sum), `make smoke`, the Expo app on a device. First run on your machine is the real test; fix anything that breaks and note it here.

## Phase 0 - verified on a real machine (2026-11-04)
`make setup`, `make up` and `make smoke` run green with Docker 29 / Compose 5. go.sum committed.

## Phase 1 - Data and registry
Added: `make seed` (go/cmd/seed) loads the four case CSVs into leads.smb_leads, catalogue.product_catalogue, registry.gstin and registry.udyam in one transaction (drop + recreate + COPY, so it is idempotent), then creates views leads.v_funnel, v_segment_perf and v_drop_reasons; go/cmd/analytics renders them into docs/analytics.md. Registry service: GET /gstin/{id} and GET /udyam/{id} return the record plus matchType (exact | corrected | none); /gstin also returns checksumValid (mod-36). Correction collapses OCR confusions (O/0, I/1/J/T, S/5, B/8) and only accepts a unique match; misses return near-matches that differ by one character (K06 -> 08YKZXS5189L1ZO). Smoke test now also checks a registry lookup (skipped if not seeded).

Verified: `make seed` twice and from dropped tables; registry unit tests (all 3,534 registry GSTINs pass the checksum; K01-K12 certificate GSTINs as tesseract reads them resolve as documented); live lookups through the gateway; `make smoke`.

Notes: drop reasons use four buckets (fixable_by_digital 1,485, before_first_contact 1,594, pricing 621, out_of_scope 932). Registry data is held in memory, loaded on first request, so the service starts before seeding; restart registry after a re-seed with changed data.
