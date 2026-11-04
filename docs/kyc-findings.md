# KYC specimen findings (Phase 0 analysis)

Source: 47 images for 12 applicants (K01-K12) in data/kyc_documents, OCR'd with tesseract and cross-checked against data/mock_gstin_registry.csv and data/mock_udyam_registry.csv. Machine-readable version: data/kyc_expected.json. This is OUR analysis, not the organizers' answer key.

| ID | Business | Planted problem | Check that catches it | Verified how |
|---|---|---|---|---|
| K01 | Vindhya Tours & Travels | GSTIN 37IHKXV5167L1ZS is Cancelled in the registry; Udyam number field is blank | Registry status; required-field check | Registry lookup; image viewed |
| K02 | Harmony Dental Care | GST certificate skewed about 6 degrees | Deskew before OCR | Image viewed |
| K04 | Saffron Dental Care LLP | GST certificate heavily blurred | Blur gate, ask for re-upload | Image viewed |
| K05 | Balaji Realty LLP | No Udyam certificate in the set | Completeness check | File listing |
| K06 | Saffron Unisex Salon | Certificate GSTIN 08YKZXS8189L1ZO differs by one digit from registry 08YKZXS5189L1ZO; checksum fails | Checksum, then registry | Checksum computed; crop viewed |
| K07 | Neelgiri Physio Clinic | Certificate legal name says "Prop. Shri Physio Clinic"; registry, trade name and bill say Neelgiri | Cross-document name match | Crop viewed |
| K11 | Trinity Bake House | Electricity bill dated 2025-10-31, older than 90 days from 2026-07-15 | Document age | OCR text |

K03, K08, K09, K10 and K12 look like clean controls.

## Known gaps (Phase 4 must close them)
- Not every field was compared across all 12 applicants (addresses, Udyam enterprise names and cities, signatory names against proprietors, ID expiry dates).
- OCR cannot separate the letter O from the digit 0 in the certificate font (K02, K04, K08, K10). Resolution must use checksum plus registry fuzzy match.
- Udyam registration dates and activity types on the certificates differ from the registry for almost every applicant. We treat this as generator noise; confirm with the organizers.
- K06, K07 routes (manual_review) are our design choice.
