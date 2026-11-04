# Context for Phase 2
- Segment codes in the catalogue map to lead segments: micro = Micro retail & services, prof = Professional offices, multi = Small multi-site businesses, mfg = Small manufacturers.
- Eligibility columns: available_city_tiers, min_employees, min_sites, requires_gstin, requires_one_of (any-of dependency on a connectivity product). Contract and price columns drive the quote.
- Case ask: one transparent monthly price; add-ons toggle; dependencies auto-handled and explained. Quote output must be reproducible so the advisor can quote exactly what the quote service returns.
- Unit semantics differ per product (per site, per line, per user, per camera, per device, per counter, per IP, per pack, per account, per channel, per number, per vehicle). Quantity inputs must come from the profile or an explicit user choice; never guess.
- Test fixtures: docs/scenarios.md items 1-10.
