# Context for Phase 1
- Data lives in data/*.csv (see docs/data-dictionary.md for every column and enum). 5,000 leads created 2026-04-01 to 2026-06-30; 368 activated (7.4%); 25 products.
- Funnel stages (stage_reached): Lead, Contacted, Needs assessed, Quote accepted, KYC complete, Activated. Outcome is Activated or Dropped.
- Drop-reason buckets to build in v_drop_reasons. Fixable by digital: owner has no time for a visit, visit not scheduled, quote too complex, quote too slow, missing or mismatched documents, long paperwork, signatory not available. Before first contact (not fixable by the journey itself): unreachable, wrong number, not interested. Pricing: price too high, competitor cheaper. Check the exact reason strings in the data before writing SQL.
- Segment codes in the catalogue map to names in the leads: micro = Micro retail & services, prof = Professional offices, multi = Small multi-site businesses, mfg = Small manufacturers.
- GSTIN checksum is the standard mod-36 algorithm (position weights 1,2 alternating, 15th character is the check character). All 3,534 registry GSTINs pass it.
- KYC GSTINs worth testing: K01 37IHKXV5167L1ZS (registry status Cancelled); K06 08YKZXS8189L1ZO (fails checksum; near-match 08YKZXS5189L1ZO).
- Go module cannot download dependencies in some sandboxes: run `make deps` on a machine with internet to create go.sum.
