# Context for Phase 7
- 5,000 leads, 368 positives (7.4%), created 2026-04-01 to 2026-06-30. Use a time-based split (earliest months train, last month test).
- Leakage columns to exclude: stage_reached, outcome, drop_reason, every stage date, days_lead_to_activation, bundle fields (bundle_products, monthly_bundle_value_inr), follow-up call counts, churned_within_6_months, nps_score, acquisition_cost_inr. Confirm each name in docs/data-dictionary.md.
- Observed patterns to sanity check against model importance: referrals convert ~14.8%, WhatsApp campaigns ~2.8%, website forms ~4.3%; micro retail 4.1% vs 11-13% for other segments; GSTIN holders 9.6% vs 2.0%.
- XGBoost serving options: Go library `leaves` (verify model format support) or a minimal Python FastAPI sidecar. Train in ml/ with a pinned requirements.txt.
- Be honest in docs/model-card.md: synthetic data, few positives, no real-world validation.
