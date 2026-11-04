Goal: deterministic eligibility, bundle assembly and pricing, with zero LLM involvement.
Read data/product_catalogue.csv carefully. Every eligibility column becomes a typed rule (city tier, minimum employees, minimum sites, GSTIN required, dependency on a connectivity product, contract options, price range, install days). Do not hard-code rules you have not read from the file.
1. Rules service: POST /eligibility takes a BusinessProfile (contracts/profile.schema.json) and returns eligible products plus, for every excluded product, a machine-readable reason code and a plain-language reason.
2. Bundle templates per segment, each with a rationale string, built only from eligible products. Dependencies are added automatically with an explanation.
3. Quote service: POST /quotes prices a bundle (per unit, site or user as the catalogue defines, price within min-max by size and contract length, one monthly total with one-time setup shown separately). PATCH toggles add-ons with dependency enforcement. Emit quote.created and quote.accepted.
4. Table-driven tests for at least 30 scenarios from docs/scenarios.md, including: micro shop without GSTIN; Tier 2 customer asking for a Tier 1 product; 8-employee firm asking for an enterprise firewall; CCTV without internet; 5-site chain; manufacturer.
Done when: all tests pass and docs/rules.md lists every rule with its source column.
