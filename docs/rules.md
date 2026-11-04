# Rules, bundles and pricing

All rules are deterministic Go in `go/pkg/catalogue`, read from `data/product_catalogue.csv` (seeded into `catalogue.product_catalogue`). The LLM never decides eligibility or price. Tests: `go test ./pkg/catalogue` (39 table-driven scenarios from docs/scenarios.md, plus price and template invariants).

## Eligibility (`POST /eligibility`, rules service)

A product is eligible only when every constraint holds for a **known** profile value. An unknown value gives `MISSING_INFO` (ask the owner), never a guess.

| Rule | Source column | Check | Code when it fails |
|---|---|---|---|
| City tier | `available_city_tiers` (`1` or `1;2`) | `profile.cityTier` is in the list. Products sold in every tier skip the check. | `CITY_TIER_UNAVAILABLE` / `MISSING_INFO` |
| Minimum employees | `min_employees` | `profile.employeeCount >= min_employees` (checked when min > 1) | `BELOW_MIN_EMPLOYEES` / `MISSING_INFO` |
| Minimum sites | `min_sites` | `profile.numSites >= min_sites` (checked when min > 1) | `BELOW_MIN_SITES` / `MISSING_INFO` |
| GSTIN required | `requires_gstin = Y` | `profile.hasGstin = true`. The reason text tells the owner that GST registration unlocks the product. | `GSTIN_REQUIRED` / `MISSING_INFO` |
| Dependency | `requires_one_of` (`P01\|P02\|...`) | At least one listed product is itself eligible | `DEPENDENCY_UNAVAILABLE` |

Every excluded product gets all failing reasons, each with a plain-language message, plus up to 3 `alternatives`: eligible products in the same `category`, closest mid-price first (for example P03 in Tier 2 → P02).

`target_segments` is not an eligibility rule. It is targeting, and the bundle templates use it.

GSTIN validity: rules trust `hasGstin` as declared. The registry service (Phase 1) and KYC (Phase 4) check status and checksum. The advisor sets `hasGstin=false` when the registry says a GSTIN is unknown or cancelled.

## Bundles (`POST /bundles`, rules service)

These templates are sales design. They live in `bundle.go` and every product ID in them is checked against the catalogue by a test. For each slot, the first eligible option is used. If no option is eligible, the slot goes into `omitted` with the reasons.

| Template | Segment | Slots (first eligible option) |
|---|---|---|
| micro-essentials | Micro retail & services | P01/P04 internet, P19 POS, P07 mobile line, optional P15 WhatsApp API, optional P22 guest Wi-Fi |
| prof-office | Professional offices | P02/P01/P04 internet, P16 email, P17 backup, P20 endpoint, optional P09 failover, optional P12 telephony |
| multi-sites | Small multi-site businesses | P02/P01/P04 per site, P09 failover, P23 CCTV, optional P10 SD-WAN (only when eligible), optional P19 POS |
| mfg-factory | Small manufacturers | P05/P03/P02/P01 internet, P24 machine monitoring, P20 endpoint, optional P16, optional P21 firewall |
| mobile-only | any | P07/P08: just a mobile plan, no bundle |

The profile's segment gets its own template plus mobile-only. An unknown segment gets every template.

## Dependencies (`requires_one_of`)

In a bundle or quote, a product whose `requires_one_of` is not met gets the **first eligible** option in catalogue order added automatically (for example CCTV → Biz Fibre 100). The new line is marked `addedFor` and carries a note. In a quote, removing the last product that satisfies a dependency is refused with `DEPENDENCY_REQUIRED` (409), and the message names what to remove first.

## Quantities (`unit`)

| Unit | Quantity |
|---|---|
| per site | `profile.numSites` |
| per user | `profile.employeeCount` |
| per account, per pack, per number | 1 (single instance) |
| per line, camera, device, counter, channel, IP, vehicle | must come from the owner; otherwise `QUANTITY_REQUIRED` |

## Pricing (`POST /quotes`, quote service)

| Rule | Source column |
|---|---|
| Unit price = `price_max − (price_max − price_min) × (0.5 × min(months, 24)/24 + 0.5 × min(qty − 1, 9)/9)`, rounded to the rupee. A 1-month single unit pays close to max. 24 months and 10+ units pay exactly min. Always inside the range. | `price_min_inr_per_month`, `price_max_inr_per_month` |
| Line contract = max(chosen contract, product minimum). A raised contract gets a note. Allowed choices: 1, 3, 6, 12, 24 months. | `min_contract_months` |
| One-time = setup fee × quantity, shown separately from the monthly total | `one_time_setup_inr` |
| Expected install days = slowest product in the quote | `typical_install_days` |
| Monthly total = Σ unit price × quantity. Total contract value = Σ monthly × line contract months + one-time | derived |

The same request always gives the same price, so the advisor quotes exactly what the quote service returns. Products must be eligible for the stored profile. Otherwise the response is `NOT_ELIGIBLE` (422) with the reasons. `PATCH /quotes/{id}` toggles add-ons (`{toggles:[{productId, enabled, quantity}], contractMonths}`) and re-prices. `POST /quotes/{id}/accept` is idempotent. Events: `quote.created` and `quote.accepted`, whose payload is the Quote.
