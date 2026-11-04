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

## Phase 2 - Rules, catalogue and quote
Added: contracts/rules-quote.schema.json (HTTP shapes, reason and error codes). go/pkg/catalogue: typed catalogue parser, eligibility (city tier, min employees, min sites, GSTIN, requires_one_of; unknown values give MISSING_INFO), same-category alternatives, segment bundle templates plus a mobile-only quick path, automatic dependency completion, deterministic pricing and add-on toggles. Rules service: GET /products, POST /eligibility, POST /bundles (reads catalogue.product_catalogue). Quote service: POST/GET/PATCH /quotes, POST /quotes/{id}/accept; it calls rules over HTTP, stores quotes in quote.quotes, and emits quote.created and quote.accepted. svc.DecodeJSON gives strict request decoding. docs/rules.md lists every rule with its source column.

Verified: `go test ./...` (39 scenario cases, price-range and monotonicity invariants for all 25 products); live run through the gateway covering eligibility, bundles, a CCTV quote with fibre auto-added, a blocked dependency removal (409), an add-on toggle, accept, the NOT_ELIGIBLE path, and both events read back from Kafka.

Cut / risks: no event on PATCH (only create and accept); events are published after the DB write with no outbox (Phase 5). Pricing by size uses item quantity, not employee count; it is our formula, not given by the case.

## Phase 3 - Advisor
Added: ts/services/advisor conversation flow (src/advisor.ts) written as plain code: extract → merge → decide → write → guard. The LLM does structured extraction with zod (AdvisorExtraction in @udyam/shared) and phrases the reply. Code decides everything else: city tier (from the case data), required profile fields, the template, quantities, the cheaper path, and when to quote. Every product and price comes from the rules, quote and registry services. A guard (src/guard.ts) rejects drafts that contain a number or product name absent from the brief, or that are in the wrong script; it retries once, then falls back to a deterministic reply. Handles mobile-only, too expensive (cheapest eligible link plus one core item, with total contract value), unknown, cancelled or OCR-corrected GSTINs via the registry, out-of-scope questions, and accept. Emits lead.created, advisor.profile.updated and bundle.recommended over Kafka (kafkajs). POST /chat, GET /sessions/{id}. Identical prompts across LLM_PROVIDER=gemini|anthropic|openai (src/prompts.ts). Eval harness `pnpm eval` (src/eval.ts, src/eval-scenarios.ts): 16 scripted scenarios and 11 multilingual conversations (all 10 case languages plus Hinglish and Tanglish), scored on eligibility against the true profile, dependencies, price equal to the quote service, language and script, quote by turn 6, and scenario expectations.

Verified: typecheck; 11 advisor tests, including live flows against the running Go services with a scripted LLM (clinic ask → recommend with email x6; firewall explained, not quoted; mobile-only → cheaper → accept; K01 cancelled GSTIN; a lying writer replaced by the fallback; out of scope); the advisor container starts, connects to Kafka and answers 503 LLM_NOT_CONFIGURED without a key; `make smoke`.

NOT verified: the eval against a real LLM. No API key was available, so docs/advisor-eval.md has no results and the Phase 3 "done when" (at least 90% per provider) is still open. Sessions are in memory. Events are fire-and-forget (outbox comes in Phase 5). Quantities for cameras, devices and counters are stated starting points (2 cameras per site, 1 device per staff member, 1 counter per site) that the reply must call out.

## Phase 3 - Advisor eval run (2026-11-04)
Added: LLM_PROVIDER=deepseek (OpenAI-compatible API via ChatOpenAI, DEEPSEEK_API_KEY, default model deepseek-flash = DeepSeek-V4.1-Flash, thinking disabled because thinking mode rejects forced tool_choice). Extraction uses tool calling for openai and deepseek: ChatOpenAI's default strict json_schema was rejected by OpenAI (it requires every key to be required) and is unsupported by DeepSeek. Romanised/native script is now read from the message's script, not the LLM (DeepSeek marked Tamil script as romanised). The eval notes service errors.
Verified: `pnpm eval` on deepseek/deepseek-flash 27/27 (100%), so the Phase 3 target is met on DeepSeek. openai/gpt-4o 23/27 (85%), below target: 2 conversations failed to parse the tool-call JSON, plus 2 multilingual failures (M01, M02); not tuned yet. Gemini and Anthropic not run (no keys).

