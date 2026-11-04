# User scenarios

Used as test fixtures for rules (Phase 2), advisor evals (Phase 3), KYC (Phase 4), the demo (Phase 6) and `make scenarios` (Phase 8). Catalogue rules live in data/product_catalogue.csv; do not trust any product detail here over the file.

## Advisor and eligibility
1. Kirana owner, Hindi, no GSTIN, wants UPI/WhatsApp: products that need a GSTIN (for example WhatsApp API, SMS packs) must be excluded with a reason; offer GSTIN-free connectivity plus cloud POS and explain how GST registration unlocks more.
2. Clinic, Tier 1, 6 staff, Tamil: fibre, cloud email for 6 users, backup, endpoint security, optional failover; one monthly price.
3. CA firm with 8 employees asks for a managed firewall: excluded (minimum employees); offer endpoint security instead and say why.
4. Owner in a Tier 2 city asks for a Tier 1 only product: excluded with reason; offer the closest eligible tier.
5. Restaurant chain, 5 sites, Tier 1: per-site connectivity, failover, CCTV, one bill; SD-WAN only if the size rules are met.
6. Small manufacturer, 40 staff: leased line or fibre as eligible, machine monitoring, honest 21-30 day install expectation.
7. Owner adds CCTV without internet: auto-add a connectivity product or block with explanation (dependency).
8. Owner only wants a mobile plan: quick path, 1-month contract, no forced bundle; soft upsell later.
9. Price-sensitive micro owner says "too expensive": cheaper path with shorter contracts and the total cost shown.
10. Owner types Tanglish or romanised Hindi: advisor handles code-mixing and offers a language switch.

## KYC
11. GSTIN Cancelled or Suspended (K01 pattern): block with a clear message and alternative path.
12. Names differ between certificate and registry or ID (K07 pattern): manual review, not silent rejection.
13. Address proof older than 90 days (K11 pattern): ask for a fresh one.
14. Signatory not available: send an e-sign link to the signatory's phone and save progress.
15. Owner abandons halfway: WhatsApp resume link restores the session with nothing re-entered.
16. Owner withdraws consent: documents purged and confirmation sent.
17. Blurred or skewed upload (K04, K02 patterns): ask for re-upload, or auto-correct skew.
18. Mistyped GSTIN (K06 pattern): checksum fails; suggest the closest registry match and route to review.

## Fulfilment and back-office
19. Site not feasible for fibre: auto-offer the 5G alternative instead of dropping the lead.
20. Installation runs late: tracker updates proactively so the owner does not call.
21. Advisor cannot close a custom request: handoff to a partner with the conversation summary.
22. Partner onboards a customer on a tablet: same flow, partner credited.
23. Sales rep opens the morning list: leads ranked by propensity score with reasons.
24. Customer is locked in with another provider: capture contract end date, schedule a reminder.
25. Wrong-number leads: OTP check at entry.
