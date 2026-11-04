# Model card: lead propensity score

**What it does:** estimates the probability that a new small-business lead activates, using only what is known when the lead is created. Sales and partners use it to decide whom to call first (admin "Leads by score", the partner handoff queue). It never decides eligibility, price or KYC; those are deterministic rules.

**Status:** prototype on synthetic data. The XGBoost model **beats the segment-average baseline** on the time-split test. It does **not reliably beat a plain logistic regression**: XGBoost's PR-AUC is 0.008 higher, but the 95% bootstrap interval for that difference is [−0.020, +0.037], which includes zero. A logistic regression would serve equally well. We kept XGBoost because the plan asked for it, and it is evaluated in plain Go with the same calibration.

## Data and split

- 5,000 leads from data/smb_leads.csv, created 2026-04-01 to 2026-06-30. 368 activated (7.4%).
- Time-based split by created_date: **train** 2026-04-01..05-15 (2,412 leads, 181 activated), **validation** 05-16..05-31 (884 leads, 60 activated; used for early stopping and calibration), **test** June (1,704 leads, 127 activated = 7.5%; never seen in training or tuning).
- Label: outcome = Activated.

## Features (known at lead creation)

lead source, segment, city tier, preferred language, employee count, number of sites, turnover band (ordinal), has GSTIN, has Udyam, current internet, current provider, monthly telecom spend, and products of interest (one flag per product plus the count).

**Excluded as leakage** (they are only known after the lead moves through the funnel, or are outcomes themselves): stage_reached, outcome, drop_reason, contacted/needs_assessed/quote_accepted/kyc_complete/activated dates, days_lead_to_activation, customer_follow_up_calls, bundle_products, monthly_bundle_value_inr, one_time_setup_inr, contract_months, churned_within_6_months, nps_score, acquisition_cost_inr. Identity fields (names, GSTIN and Udyam numbers, contact details) are not features.

## Model

XGBoost (`ml/train.py`, pinned `ml/requirements.txt`): depth-3 trees, learning rate 0.03, early stopping on validation PR-AUC (92 trees). The 7.4% positive rate is handled with `scale_pos_weight` (12.3), followed by Platt scaling on the validation weeks, so probabilities match observed rates again (test Brier 0.0666 calibrated vs 0.2004 uncalibrated).

## Results on the test month (full tables: docs/model-eval.md)

| model | PR-AUC | ROC-AUC | top-decile lift | Brier |
|---|---|---|---|---|
| XGBoost (served) | 0.151 | 0.701 | 2.37 | 0.0666 |
| Logistic regression | 0.143 | 0.710 | 2.13 | 0.0668 |
| Segment average | 0.106 | 0.632 | 1.82 | 0.0677 |
| Random | ≈0.075 | 0.5 | 1.0 | |

- vs segment average: PR-AUC +0.045, 95% bootstrap interval [+0.014, +0.086]. A real improvement.
- vs logistic regression: interval [−0.020, +0.037]. Not distinguishable.
- **What it means for sales:** calling the top 10% of June leads by score finds activations 2.4× more often than calling at random (about 18% vs 7.5%).
- **Calibration:** predicted and observed rates agree by decile, from 0.9% vs 0.6% in the lowest decile to 15.2% vs 17.6% in the highest.

The importance ranking (GSTIN, telecom spend, lead source, products of interest, segment) matches what the raw data shows: referrals 14.8% vs WhatsApp campaigns 2.8%, GSTIN holders 9.6% vs 2.0%, micro retail 4.1% vs 11–13% for other segments.

## Serving

- `go/pkg/scoring` evaluates the exported trees directly (model.json, embedded in the binary). The Go library `leaves` reads only the old XGBoost binary format, which XGBoost 3 no longer writes. Walking 92 depth-3 trees is a few microseconds, so we did not need a Python sidecar.
- A parity test compares Go's probability with Python's on 51 test-month leads (tolerance 1e-5).
- **Reasons:** each split on a lead's path moves the prediction from the parent's cover-weighted value to the child's, and that change is credited to the split's input field (Saabas attribution). The three largest pushes are shown, for example "Lead source: Referral (raises the score)". This is an approximation of SHAP, good for ranking reasons, not exact.
- **Bands:** high is the top ~20% of training scores, medium the next ~40%, low the rest.
- **Live leads:** the scoring service scores them on lead.created and re-scores them on every advisor.profile.updated, emitting lead.scored. Leads from the new digital journey are scored as "Website form" leads, the closest historical channel. We have no outcomes for the new channel yet.

## Limits (read before using)

- **Synthetic data.** The case data is generated; patterns in it may not hold for real leads. There is no real-world validation.
- **Few positives.** 368 activations in total, 127 in the test month. Every metric above has wide uncertainty; see the bootstrap intervals.
- **One test month.** Seasonality, campaign mix and sales-team changes are not represented.
- **New channel.** The advisor/web journey did not exist in the training data. Mapping it to "Website form" (4.3% historical conversion) is an assumption. Retrain once real outcomes arrive.
- **Not for decisions about the customer.** The score orders call lists only. It must not affect eligibility, price, KYC or service quality.
- **Fairness.** Language and city tier are inputs. They carry real signal in this data, but they could also deprioritise owners in some regions or language groups. Before production, check call-back rates and outcomes by language and tier, and consider removing these features if the effect is unacceptable.

## Retraining

`ml/.venv/Scripts/python ml/train.py` (or `make model`), then `go test ./pkg/scoring` (parity) and rebuild the scoring service. The script rewrites model.json, the parity fixtures and docs/model-eval.md. This card's numbers must be updated by hand.
