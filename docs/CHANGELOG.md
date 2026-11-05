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


## Phase 4 - Smart KYC
Added: go/pkg/kyc, a deterministic pipeline with these stages: completeness; image quality (blur by Laplacian variance, gate 500 against 11,000+ for sharp pages; skew by projection profile with deskew); OCR via the tesseract CLI on the red channel, which removes the watermark; GSTIN and Udyam numbers re-read from their own crops with a character whitelist; identifiers resolved by registry exact match, then O/0-style correction, then a GSTIN format repair the registry must confirm; GST status; name match (registry trade name against every document, proprietor against registry, signatory against proprietor); address match; bill age (90 days from KYC_REFERENCE_DATE); ID expiry. Each finding has a code, severity, route, stage, message key and masked params. The final route is reject > reupload > manual_review > pass. go/pkg/registry now holds the checksum, the fuzzy index and a CSV-backed registry shared by the registry service and the tests. KYC service (go/cmd/kyc): consent per purpose (uploads and checks answer 403 without kyc_verification); withdrawal purges images, results and reviews in one transaction; GSTIN prefill; PUT upload (PNG/JPEG by magic bytes, 8 MB) with instant blur and skew feedback; specimen loader for the demo; check; review queue with approve, reject and request_reupload. Events: consent.recorded, consent.withdrawn, kyc.submitted, kyc.checked (one per stage with findings), kyc.passed, kyc.failed, kyc.review.requested. Images are deleted at a final decision, at withdrawal, or after KYC_DOC_RETENTION_DAYS (default 30). Contracts: contracts/kyc.schema.json, contracts/kyc-messages.json (message and next step for all 15 codes in 10 languages; the generator fails if any is missing), and DefaultSeverity generated into Go. TS types in @udyam/shared (kyc.ts, kyc-messages.ts). Docs: docs/kyc-findings.md, docs/kyc-field-comparison.md (every field, all 12 applicants), docs/privacy-note-draft.md. data/kyc_expected.json now records the fields compared and the further discrepancies. Tesseract is installed in the kyc image only. `.gitattributes` keeps shell scripts and SQL LF, because Windows checkouts with CRLF broke kafka-init.

Verified: the 12-applicant expectation table passes with real tesseract (`make kyc-test`) and through recorded OCR in plain `go test ./...`. 19 check cases cover suspended, not-found and checksum-invalid GSTINs, Udyam not found, the 90/91-day bill boundary, expired ID, wrong signatory, name and address mismatch, missing field or document, and route priority. Service tests cover sniffing, masking and lead IDs. Live through the gateway: 403 without consent; K07 to manual_review with a review row; K03 pass and K01 reject with images purged; review approve, then 409 on a second decision; blurry and skewed uploads give instant findings; a non-image gets 415; withdrawal purged 4 documents. All seven events were read back from Kafka, and no raw GSTIN appears in the kyc logs. `go test ./...`, TS typecheck and `make smoke` are green.

Cut / risks: the KYC answer key is our own. K06 and K07 manual_review are our design, and Udyam date and activity differences are treated as noise; the organizers should confirm both. LLP and company signatories cannot be verified (no partner data). OCR thresholds are tuned on 47 rendered specimens, not phone photos. The review API has no authentication yet (Phase 8). Event publishing has no outbox yet (Phase 5). Extracted fields have no retention limit until compliance sets one. Translations are machine-drafted.

## Phase 5 - Orders, tracker, notifications
Added: go/pkg/outbox. Each service writes its events into its own outbox table in the same transaction as the change (Enqueue), and a relay publishes them to Kafka (at least once; SKIP LOCKED; a partial batch keeps the rows it already sent). Once() records (consumer, event ID) in processed_events in the handler's own transaction, so a redelivered event has no second effect. The quote, kyc and tracker services now publish only through the outbox. pkg/kafkax.Consume retries a failing handler (200 ms, 1 s, 3 s), then writes a DeadLetter (consumer, original topic, partition, offset, error, original message) to dlq.events and commits. Messages that are not valid envelopes go straight to the DLQ. If the DLQ write fails, the offset is not committed. kafkax.Run restarts a consumer that stops. go/pkg/orders holds the state machine (quote_accepted, kyc_pending, kyc_passed, install_scheduled, installing, activated, plus delayed and failed), timeline steps, and fibre detection. Tracker: it opens an order on quote.accepted, moves it on kyc.passed and on kyc.failed with route reject, and remembers a KYC outcome that arrives before the quote. The expected activation date is the KYC pass date plus the slowest product's install days. A delay with fibre_not_feasible emits order.fallback.offered (5G AirFiber P04) and a notification. API: GET /orders, GET /orders/{id}, GET /orders/{id}/timeline, POST /orders/{id}/transitions (409 for an illegal move), POST /orders/{id}/fallback/accept. Notify: consumes order.created, order.status.changed and notification.requested, and learns each lead's language from advisor.profile.updated. Mock WhatsApp, SMS and email adapters write to sent_messages (one row per event and channel), shown at GET /messages. Templates are in en, hi and ta; other languages fall back to English. cmd/simulator moves orders along the happy path every SIM_STEP_SECONDS, and leads whose ID contains "nofibre" take the fallback branch. KYC resume links: POST /applications/{leadId}/resume-link and GET /resume/{token} show missing documents and findings that need a re-upload. A sweeper sends one link to each application left unfinished longer than KYC_ABANDON_AFTER_MINUTES. Contracts: contracts/orders.schema.json and the topic order.fallback.offered. TS types in @udyam/shared/orders.ts. The Go Dockerfile now downloads modules once in a shared layer instead of running `go mod tidy` per service; that step had failed on a network blip.

Verified: unit tests for the state machine, timeline steps, templates (en, hi and ta have the same placeholders in every template) and the simulator's moves. `make integration` (go/integration, build tag integration) runs against the live stack, and all four tests pass:
1. Tracker and notify stopped, quote accepted, KYC passed, both started: the order reaches activated, and every milestone message is sent exactly once.
2. Kafka stopped and quote accepted (the API still answers 200); after Kafka restarts, the order appears.
3. A nofibre lead is delayed, 5G is offered, the owner accepts, the product becomes P04 and the order activates; an illegal transition gets 409.
4. A malformed message on quote.accepted lands in dlq.events, and the tracker keeps processing.

Bug found by the integration test and fixed: GET /orders with no status filter returned nothing (NULL array). `go test ./...`, `make smoke` and TS typecheck are green.

Cut / risks: accepting the fallback swaps the product but does not re-price it (the note says the price must be re-confirmed through the quote service). Notify has no real phone or email (the recipient is the masked lead ID). The advisor still publishes directly to Kafka, not through an outbox. Order across topics is not guaranteed, so the tracker handles kyc.passed arriving before quote.accepted. OTP at lead creation (wrong numbers) is not built.

## Phase 6 - Frontends
Added:
- **@udyam/shared:** a typed API client for every gateway route used (advisor, rules, quotes, KYC, tracker, notify; ApiError exposes the service's code and message). UI strings: 64 keys in all 10 languages, with t() and an Indian-grouping inr(). Order status labels in 10 languages. DEMO_OWNERS: six scenarios from docs/scenarios.md (Tamil clinic, Hindi kirana without GSTIN, CA firm asking for a firewall, 5-site restaurant chain, Hinglish mobile-only, fibre not feasible), each tied to a KYC specimen. Tests check that every language has every string with the same placeholders, and that every KYC code has a message and next step in every language.
- **Web (Next.js 16, Tailwind 4), mobile-first and responsive.** On phones: a compact progress bar, a primary action pinned to the bottom (safe-area aware), 48 px tap targets, and cards instead of tables. On desktop: a vertical stepper, a two-column layout and a sticky price summary.
  - Journey: language picker, advisor chat with quick-reply chips in the owner's language, then quote. The quote has line items, add-on toggles (only products the rules service says are eligible; a dependency refusal is shown as the service explains it) and accept.
  - Then purpose-level consent, then KYC. KYC has GSTIN prefill, four camera/upload slots (photos shrunk on the device to 2000 px before upload), instant blur and skew feedback, sample documents, and the full check with a message and next step for every finding in the owner's language. Consent can be withdrawn here.
  - Then the tracker, polling every 3 s: timeline, ETA, the 5G fallback offer with accept, and the WhatsApp and SMS messages sent. A resume-link landing page (/kyc/resume/[token]) restores the lead.
  - Back office (/admin): orders (filterable), the KYC review queue with approve, re-upload or reject, and sent messages.
  - Fonts: self-hosted Noto Sans (Latin plus 8 Indic scripts, variable weight) through next/font/local, each face with its own unicode-range, so a phone downloads only the scripts on screen. There are no network fonts.
- **Mobile (Expo SDK 58, React 19.3 like web):** the same journey on the shared package. KYC slots open the rear camera (expo-image-picker) and fall back to the photo library. react-native-web is added so the app can be exercised in a browser.
- **Scripts:** e2e.mjs drives the demo path in headless Edge through playwright-core (no browser download), at phone and desktop sizes, and saves docs/screens. e2e-mobile.mjs does the same for the Expo app's web build. Make targets: web, web-e2e, mobile-e2e.
- **Fixes found by the E2E runs:**
  - The advisor added a second fibre line when the owner said "need reliable internet"; there is now one primary access link per order (ACCESS_LINKS), with a live test.
  - A useEffect returned scrollIntoView's Promise (current browsers return one), which crashed the chat after the first reply.
  - The lead ID was generated with Math.random during render, which Cache Components refuses.
  - next/font needs literal options.
  - The test scripts used shell globs that cmd on Windows does not expand.

Verified with the live stack and DeepSeek:
- **Phone web (390x844):** Tamil clinic, chat in Tamil, then quote, consent, KYC K03 pass, and activation in about 50 s, with notify messages in Tamil.
- **Desktop web:** no-fibre clinic, KYC K09 pass, delay with 5G offer, accept, activation on P04. The quote has exactly one access link (P02).
- **Admin:** orders, review queue and messages pages render.
- **Expo app (web build, 390x844):** restaurant chain, per-site quote, KYC K12 pass, activation.
- `next build` passes; typecheck of all packages; shared tests 5/5; advisor tests 13/13 (live); Go tests unchanged.

Cut / risks:
- The native Expo app was not run on a device or emulator. The camera path is typechecked but untested; on a phone, set EXPO_PUBLIC_API_URL to the computer's LAN IP.
- The journey is kept in localStorage on web and in memory on mobile.
- The admin views have no login (Phase 8).
- Lead list, unclosed leads and the partner handoff view need the scoring service (Phase 7).
- UI translations are machine-drafted; a native-speaker review is needed.
- Advisor replies print rupees as the LLM writes them (₹3933), not in Indian grouping.

## Phase 7 - Scoring and handoff
Added:
- **ml/train.py** (pinned ml/requirements.txt, venv in ml/.venv) trains XGBoost on features known at lead creation; all leakage columns are excluded and listed in the card.
  - Time split: train 04-01..05-15, validation 05-16..05-31 for early stopping and Platt calibration, test June.
  - The 7.4% positive rate is handled with scale_pos_weight followed by calibration.
  - Compared with logistic regression and a segment-average baseline: PR-AUC, ROC-AUC, top-decile lift, Brier, calibration by decile, gain importance per input field, and bootstrap intervals for the PR-AUC differences.
  - Writes go/pkg/scoring/model.json, parity fixtures and docs/model-eval.md.
- **go/pkg/scoring** evaluates the exported trees in plain Go. `leaves` cannot read XGBoost 3 models, and the trees are small enough to need no sidecar. Reasons are Saabas path attributions summed per input field (top 3, raises/lowers). Bands come from the training-score deciles.
- **Scoring service:**
  - Endpoints: POST /score, GET /leads (ranked, live or case), GET /tasks (partner tasks ranked by score), POST /tasks/{id}/claim, POST /tasks/{id}/close.
  - Consumes lead.created and advisor.profile.updated, re-scoring live leads and emitting lead.scored through the outbox. Leads from the new digital channel are scored as "Website form".
  - Consumes lead.unclosed, opening (or updating) one partner task per lead.
  - Imports the case data's June leads (the test month) at startup as the demo pipeline.
- **Advisor:** POST /handoff emits lead.unclosed with a deterministic summary (profile facts, last quote, reason) and the last 12 turns.
- **Web:** a "Talk to a person" link in chat; admin pages "Leads by score" (live or case, with reasons) and "Partner handoff" (summary, transcript, score, claim and close).
- **Shared:** scoring types and API methods, and two new strings in all 10 languages.
- **Contracts:** contracts/scoring.schema.json; payloads documented for lead.unclosed and lead.scored.
- **Docs:** docs/model-card.md and docs/model-eval.md.
- **Make:** `make model`.

Results (June test month, 1,704 leads, 127 activated):

| model | PR-AUC | top-decile lift |
|---|---|---|
| XGBoost | 0.151 | 2.37 |
| Logistic regression | 0.143 | 2.13 |
| Segment average | 0.106 | 1.82 |

- XGBoost beats the segment average (95% interval for the PR-AUC gain [+0.014, +0.086]).
- XGBoost does **not** reliably beat logistic regression ([−0.020, +0.037]). The card says so.
- Calibrated Brier 0.0666.

Verified:
- Go/Python parity on 51 test leads (1e-5), plus behaviour tests (GSTIN and referral raise the score; no GSTIN is a top reason).
- Integration tests: case leads served ranked with reasons; a lead.unclosed event opens a partner task, claim works, and a second claim gets 409.
- Live: an advisor conversation about a custom request (private 5G) handed off; the task shows its score and factual summary.
- Desktop E2E including the new admin pages; advisor tests 14/14; `go test ./...` green.

Cut / risks:
- Synthetic data, 368 positives, a single test month.
- The new channel is mapped to "Website form" by assumption.
- Language and city tier are features, and the leads list shows e.g. "Language: Tamil (lowers the score)". This needs a fairness review before production use (model card, Limits).
- Reasons approximate SHAP. Partner tasks have no assignment rules beyond claim.

## Phase 8 - Hardening and submission
Added:
- **Per-service Postgres roles** (infra/postgres/roles.sql, run by the idempotent `db-roles` compose job on every `up`, so it also upgrades an existing database). Each Go service connects as svc_<name>, owns its schema, and cannot read another: `svc_kyc` reading `quote.quotes` gets "permission denied for schema quote". registry and rules get read-only access to the reference data that `make seed` loads.
- **Gateway:**
  - Per-IP token-bucket rate limits: general RATE_LIMIT_RPS=30; advisor chat 0.5/s with a burst of 6; uploads and checks 1/s; 429 with Retry-After.
  - A JSON 503 UPSTREAM_UNAVAILABLE with Retry-After when a service is down.
  - Back-office routes need X-Admin-Token (constant-time compare; closed if unset; compose default `demo-admin-token`).
  - CORS exposes request and correlation IDs.
- **Observability:**
  - Prometheus-format `/metrics` on every Go service (requests by route and status class, latency, Kafka processed/retried/dead-lettered, DB pool, outbox backlog, open KYC reviews) and on the advisor.
  - The access log carries requestId, correlationId and leadId; correlation IDs pass through the gateway.
  - An admin Status page summarises health and metrics.
- **Web:** the admin asks for the token once (AdminGate); the shared client sends it; integration tests and E2E send it and back off on 429.
- **make scenarios** (go/cmd/scenarios): 33 checks mapped to docs/scenarios.md. 10 rules/quote, 10 KYC, 5 fulfilment and ops, 8 live advisor (skipped, not failed, without an LLM key). The 4 unbuilt stories are listed as NOT BUILT.
- **make drills** (go/integration/drills_test.go): Kafka down, kyc down, Postgres restart, with timings in docs/failure-drills.md.
- **Dependency audit:** govulncheck found vulnerable pgx 5.7.1, x/crypto 0.27 and x/text 0.18. Upgraded to pgx 5.12, x/crypto 0.58, x/text 0.43. These need Go 1.26 (go.mod `go 1.26.0`, up from the plan's 1.22), and the build images move to golang:1.26-alpine (1.26.10). govulncheck in that image: no vulnerabilities. `pnpm audit --prod`: 2 moderate, in Expo build tooling only.
- **Deliverables:** docs/architecture.md (Mermaid, demo and national-scale topologies) with PNG exports (ts/apps/web/scripts/render-diagrams.mjs); docs/business-case.md (10 slides, figures from docs/analytics.md and measured results, targets labelled as proposals); docs/privacy-security-note.md (1 page); docs/demo-script.md (3 minutes, shot list); docs/failure-drills.md; README rewritten (quickstart, deliverables, what is and is not verified).

Verified:
- `make scenarios` 33/33 on the main stack, before and after the Go 1.26 rebuild.
- `make drills` all pass. Kafka down: quote and KYC still succeed, the backlog is visible, the order catches up 36 s after Kafka starts. kyc down: 503 JSON, the rest unaffected, back in 3 s. Postgres stop: DB requests fail in about 7 s; reconnects 1 s after start with no service restart.
- Role isolation checked with psql.
- Gateway guard unit tests (admin routes, token, rate limit).
- `go test ./...`; TS tests and typecheck.
- **Fresh clone** (git clone onto D:, the user's .env for the LLM key, isolated compose project `udyamclean` with new volumes): `make up seed smoke scenarios` → SMOKE OK, **33 passed, 0 failed, 0 skipped**, exit 0. The project was removed afterwards (`down -v`, its own volumes only).

Cut / risks:
- Not built: e-sign (#14), partner attribution (#22), contract-end reminders (#24), OTP at lead entry (#25).
- Owner routes have no login (scoped by unguessable lead ID). The admin token is a shared secret, not SSO.
- A Postgres outage costs about 7 s per request before failing (no circuit breaker).
- The advisor publishes without an outbox.
- The Expo app was not run on a device.
- The Go version moved from 1.22 to 1.26 for security fixes.
- The hidden KYC answer key and the 15 hidden advisor scenarios are unknown; our expectations are documented.

## Returning customers (2026-11-05)
Added: customer accounts in the kyc service (mobile + one-time code; demo shows the code on screen, `KYC_DEMO_OTP`), `/account` page with every past order and tracking (`/track?lead=`), "Start a new order", and KYC reuse: `go/pkg/kyc` ReusePolicy re-checks the GST and Udyam registries, signatory ID validity and age (365 days) and needs the owner's "nothing changed" confirmation; anything else runs the full pipeline. `kyc.passed` gains `decidedBy: reuse`. Advisor: owner-stated user counts ("only 5 people on the cloud") re-quote at once.
Verified: go test ./..., advisor tests 15/15 and eval 27/27 (DeepSeek), smoke, web E2E (phone), mobile E2E, new `make account-e2e` (sign in, track a past order, new order verified without uploads).
Not done: account screens in the Expo app; real SMS for the code; account deletion endpoint.
