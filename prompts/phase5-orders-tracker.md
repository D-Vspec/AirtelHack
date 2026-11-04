Goal: an event-driven order lifecycle.
1. Tracker consumes quote.accepted and kyc.passed, creates the order (order.created), and runs a state machine: quote_accepted, kyc_pending, kyc_passed, install_scheduled, installing, activated, with failed and delayed branches (for example fibre not feasible, which triggers a 5G fallback offer event). Expected activation date comes from the slowest product's typical install days.
2. Reliability: idempotent consumers (event-id dedupe table), outbox pattern for publishing, dead-letter topic dlq.events. Replace the Phase 0 behaviour in pkg/kafkax.Consume (log and commit on handler error) with retries then dead-lettering.
3. Notify service consumes notification.requested and order.status.changed. WhatsApp, SMS and email adapters are mocks that write to a visible sent-messages table, with template keys per language.
4. cmd/simulator advances an order through stages on a timer for the demo.
5. Resume-link flow for abandoned KYC.
6. GET /orders/{id}/timeline for the UI.
Done when: an integration test runs lead to quote to KYC to activation across real Kafka and Postgres, and killing a service mid-flow loses no events.
