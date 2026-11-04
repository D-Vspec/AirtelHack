# Context for Phase 5
- Events involved: quote.accepted, kyc.passed, order.created, order.status.changed, notification.requested, dlq.events (all in contracts/topics.json).
- Phase 0 consumer behaviour (pkg/kafkax.Consume) logs and commits on handler errors; this phase replaces it with retry plus dead-lettering and an idempotency table.
- Abandoned-KYC drop reasons in the data (missing document, paperwork too long, signatory not available) motivate the resume-link flow.
- 319 leads were lost to wrong numbers: consider an OTP check when the lead is created.
- Expected activation date uses the slowest product's typical_install_days (table below).
