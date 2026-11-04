# Context for Phase 8
- Judging (from the case): a working prototype with a 3-minute demo video, an architecture diagram, a business case of 10 slides or fewer, and a 1-page privacy note. Judges run 15 hidden advisor scenarios live and check KYC against an answer key.
- Business case questions the case asks: which segments first and with what bundle, price and contract; AI discovery in Hindi and regional languages; fast KYC under the DPDP Act 2023 (consent, purpose limitation, minimisation); how field staff and partners fit; 3-5 success metrics with targets.
- Numbers for the business case come from docs/analytics.md (Phase 1). Do not invent figures.
- The architecture diagram shows two topologies: the demo (docker compose, single Postgres) and national scale (multi-region Postgres or CockroachDB, Kafka partitioned by lead_id, regional gateways).
- Stakeholders to name: Accel Business sales leadership, field sales and channel partners, compliance/DPO, KYC and risk, network and installation ops, billing and support.
- Unanswered questions for the organizers: allowed external LLM APIs; mocked DigiLocker/e-sign; deadline; video format; Udyam date/activity differences.
