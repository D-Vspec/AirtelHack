# Udyam Digital: business case (10 slides)

Audience: Accel Business sales leadership and the YTL Hackathon judges. Every figure comes from `docs/analytics.md` (5,000 case leads, April–June 2026) or from measurements of this prototype, and the source is named beside it. Targets are proposals to agree with sales leadership, not results.

---

## 1. The problem: 93 of every 100 leads are lost, a third of them to friction we can remove

- 5,000 leads produced 368 activations: **7.4%** (funnel table).
- The funnel leaks at every step: 68.1% are contacted, 39.5% are needs-assessed, 17.2% accept a quote, 11.8% complete KYC, 7.4% activate.
- Of the 4,632 drops:
  - **1,485 (32.0%) are fixable by a digital journey.** No time for a visit (531), visit never scheduled (296), quote too complex (206), quote too slow (179), missing or mismatched documents (141), paperwork too long (79), signatory unavailable (53).
  - **1,594 (34.4%) are lost before first contact.** Unreachable (805), not interested on first call (470), wrong number (319).
- **Today's digital channels have no journey behind them:** website forms convert at 4.3% and WhatsApp campaigns at 2.8%, against 14.8% for referrals.

## 2. Where to start: professional offices first

| Segment | Leads | Conversion | CAC | Avg monthly bundle | 6-month churn |
|---|---|---|---|---|---|
| Professional offices | 1,247 | 11.1% | ₹10,695 | ₹6,796 | 13.8% |
| Small multi-site | 755 | 11.0% | ₹15,614 | ₹31,233 | 10.8% |
| Small manufacturers | 289 | 12.8% | ₹15,324 | ₹19,980 | 8.1% |
| Micro retail & services | 2,709 | 4.1% | ₹20,007 | ₹2,505 | 22.7% |

- **Wave 1, professional offices** (clinics, CA firms, law offices, coaching centres): they convert well, have the lowest CAC, and buy a standard bundle that self-serve can sell end to end.
- **Wave 2, multi-site businesses and manufacturers:** the highest bundle values (₹31,233 and ₹19,980 a month). The digital journey qualifies and quotes them, and field sales close them, because installs are longer (leased lines take 21–30 days).
- **Micro retail stays self-serve only and light.** It is 54% of leads but has 4.1% conversion, the highest CAC (₹20,007) and the highest churn (22.7%). Field visits do not pay back here.

## 3. Bundles and pricing (from our quote engine)

Prices are set deterministically inside each product's catalogue price range, never by the AI.

| Bundle | Who | Monthly | One-time | Contract |
|---|---|---|---|---|
| Office ready: Biz Fibre 300, email ×6, backup 1 TB, endpoint security ×6 (+ optional 4G/5G failover) | 6-person clinic, Tier 1 | **₹3,933** | ₹0 | 12 months, ₹47,196 total |
| Multi-site: fibre + failover per outlet, cloud CCTV, POS, guest Wi-Fi | 5-outlet restaurant chain | **₹19,595** | ₹43,000 | 12 months, ₹2,78,140 total |
| Mobile-only quick path | Owner who only wants mobile lines | P07 per line | – | **1 month** |

- **Price objection:** "too expensive" gets a cheaper eligible tier with the total contract cost shown. 621 drops (13.4%) were price-related.
- **No GSTIN:** an owner without one gets GSTIN-free connectivity and POS, plus an explanation of what GST registration unlocks. GSTIN holders activate at 9.6% vs 2.0% without.

## 4. The owner journey: one phone session instead of a visit

**Language → AI advisor → transparent quote → consent → smart KYC → order tracker.** Measured on this prototype:

- **Advisor:**
  - 10 languages plus Hinglish and Tanglish.
  - It never invents a product or price: every product comes from the rules service and every price from the quote service.
  - Eval with DeepSeek V4.1 Flash: **27/27 conversations pass** (16 scripted, 11 multilingual); gpt-4o 23/27 (`docs/advisor-eval.md`).
  - Recommends within 6 turns.
- **KYC:** the planted problems in all **12 specimen applicants are caught as expected**: cancelled GSTIN, skew, blur, missing Udyam, checksum, name mismatch, stale bill, with 5 clean passes (`docs/kyc-findings.md`). The owner gets a message and a next step in their language for every failure.
- **Tracker:** proactive WhatsApp/SMS at each milestone. When fibre is not feasible, it offers 5G AirFiber automatically instead of dropping the lead.
- **End to end:** **33/33 user-story scenarios pass** against the live stack (`make scenarios`).

## 5. Fast KYC that respects the DPDP Act 2023

- **Consent per purpose** (verification, provisioning, communication), recorded with version and language. No upload without it.
- **Minimisation:** four documents. Images are deleted at the final decision or after 30 days. Identifiers are masked in logs and events.
- **Withdrawal** purges images, results and reviews in one step.
- **Human in the loop:** ambiguous cases (one-digit GSTIN typo, a different name on one document) go to a review queue, never to a silent rejection.
- Details: `docs/privacy-security-note.md`.

## 6. Field sales and partners: from cold calls to warm closes

| Source | Conversion | CAC |
|---|---|---|
| Referral | 14.8% | ₹4,099 |
| Inbound call | 9.5% | ₹5,113 |
| Channel partner | 8.0% | ₹20,851 |
| Field sales | 7.3% | ₹24,335 |
| Website form | 4.3% | ₹6,352 |
| WhatsApp campaign | 2.8% | ₹4,026 |

- **Digital handles discovery, quote and KYC.** Field sales spend visits on wave-2 accounts (multi-site, manufacturers), where bundle value justifies a ₹24,335 CAC.
- **Partners get warm handoffs, not raw lists.** When the advisor cannot close (custom request, owner asks for a person), the lead goes to a partner queue with a conversation summary, ranked by propensity score.
- **The score:** calling the top 10% of leads finds activations **2.4×** as often as random (`docs/model-card.md`). It orders call lists only and never touches eligibility or price.
- **Referrals** are the best channel (14.8%, ₹4,099): add a refer-a-business link to the activation message.

## 7. Rollout

| Phase | Weeks | Scope | Gate to next phase |
|---|---|---|---|
| Pilot | 0–8 | Professional offices, Tier 1 cities (Ahmedabad, Bengaluru, Chennai, Delhi, Hyderabad, Kolkata, Mumbai, Pune), English, Hindi, Tamil | KPI targets below met for 4 consecutive weeks |
| Wave 2 | 9–20 | Multi-site and manufacturers with field-sales close; all 10 languages; partner handoff | Partner NPS and field-time saved agreed with sales |
| Scale | 21+ | Micro retail self-serve, Tier 2 cities, WhatsApp entry point | Unit economics positive for micro retail |

## 8. KPIs and targets (proposed)

| KPI | Baseline (case data) | Pilot target |
|---|---|---|
| Lead → activation, professional offices | 11.1% | **15%** |
| Drops fixable by digital, share of all drops | 32.0% | **< 15%** |
| Quote accepted → KYC complete | 68.3% | **85%** |
| Leads contacted / engaged | 68.1% | **85%** (owner starts the journey, plus a WhatsApp resume link) |
| CAC, professional offices | ₹10,695 | **< ₹7,500** |

## 9. Risks and mitigations

| Risk | Mitigation |
|---|---|
| AI recommends something wrong | Rules, prices and KYC verdicts are deterministic code. A guard rejects any reply with a number or product the services did not return. Eval gate ≥ 90% per provider and language. |
| Our KYC expectations differ from the organisers' answer key | Our expected table is documented. K06/K07 routes are design choices to confirm. Thresholds are configurable. |
| Regional-language quality (Tamil, Punjabi) | Per-language eval; machine-drafted UI strings reviewed by native speakers before launch. |
| Score bias by language or region | The score orders call lists only. Fairness review of language and city-tier effects before production (model card). |
| DPDP non-compliance | DPO sign-off on the privacy note; retention set with compliance; reviewer access control. |
| Channel conflict with partners and field | Partners get ranked warm leads and credit for handoffs. Field focuses on high-value accounts. |
| Fibre not feasible | Automatic 5G AirFiber offer instead of a dropped order. |

## 10. Stakeholders and asks

- **Accel Business sales leadership:** approve the pilot segment, targets and partner credit model.
- **Field sales and channel partners:** handoff queue workflow, call-list ranking.
- **Compliance / DPO:** privacy note, retention periods, consent text in 10 languages.
- **KYC and risk:** review-queue SLAs, the K06/K07 routing policy, LLP/company signatory checks.
- **Network and installation ops:** feasibility data for the fibre-or-5G decision, install-date commitments.
- **Billing and support:** one-bill invoicing, tracker data for "where is my order" calls.

**Open questions for the organisers:**
- Are external LLM APIs allowed in production?
- Can DigiLocker and e-sign be mocked (scenario 14)?
- Are the Udyam date and activity differences between certificates and the registry intended?
- What is the answer key's routing for name and checksum mismatches?
