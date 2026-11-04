Goal: lead propensity scoring and partner handoff.
1. Offline training in ml/ (Python, xgboost, scikit-learn). Use only features known at lead creation: segment, source, city tier, language, employees, sites, turnover band, has_gstin, has_udyam, current provider and spend, products of interest. Exclude leakage: stage_reached, outcome, drop_reason, every stage date, days_lead_to_activation, bundle fields, follow-up calls, churn, NPS, acquisition cost. Use a time-based split, handle the 7.4% positive rate, and report PR-AUC, top-decile lift, calibration and feature importance against a logistic regression and a segment-average baseline.
2. Serve in Go: try the leaves library for the XGBoost model; if the model format is unsupported, use a minimal Python sidecar.
3. Scoring service: POST /score returns probability and top reasons, consumes lead.created, emits lead.scored.
4. Handoff: on lead.unclosed create a partner task with the conversation summary and score; the admin view ranks leads by score.
5. Write docs/model-card.md including limits (368 positives, synthetic data).
Done when: the model beats the baselines on the time-split test, or the write-up says plainly that it does not.
