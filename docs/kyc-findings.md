# KYC specimen findings

Source: 47 images for 12 applicants (K01-K12) in data/kyc_documents, cross-checked against data/mock_gstin_registry.csv and data/mock_udyam_registry.csv. Machine-readable version: data/kyc_expected.json. This is OUR analysis, not the organizers' answer key.

**Status (Phase 4):** the KYC pipeline (go/pkg/kyc) reproduces this table exactly. `go test ./pkg/kyc -run Specimen` checks the final route and the finding codes for all 12 applicants. With `make kyc-test` it runs against real tesseract in a container, and by default against recorded tesseract output.

| ID | Business | Planted problem | Check that catches it | Final route |
|---|---|---|---|---|
| K01 | Vindhya Tours & Travels | GSTIN 37IHKXV5167L1ZS is Cancelled in the registry; Udyam number field is blank | Registry status; required-field check | reject (GST_STATUS_CANCELLED, FIELD_MISSING) |
| K02 | Harmony Dental Care | GST certificate skewed about 7 degrees | Skew detection, deskew before OCR | pass (IMAGE_SKEWED, info only) |
| K04 | Saffron Dental Care LLP | GST certificate heavily blurred | Blur gate, ask for re-upload | reupload (IMAGE_BLURRY) |
| K05 | Balaji Realty LLP | No Udyam certificate in the set | Completeness check | reupload (DOC_MISSING) |
| K06 | Saffron Unisex Salon | Certificate GSTIN 08YKZXS8189L1ZO differs by one digit from registry 08YKZXS5189L1ZO; checksum fails | Checksum, then registry near-match | manual_review (GSTIN_CHECKSUM_INVALID) |
| K07 | Neelgiri Physio Clinic | Certificate legal name says "Prop. Shri Physio Clinic"; registry, trade name and bill say Neelgiri | Cross-document name match | manual_review (NAME_MISMATCH) |
| K11 | Trinity Bake House | Electricity bill dated 2025-10-31, 257 days before 2026-07-15 | Document age (90 days) | reupload (DOC_STALE) |
| K03, K08, K09, K10, K12 | | Clean controls | | pass |

K06 and K07 go to manual_review by our choice. A one-digit GSTIN slip and a different name on one document look like clerical errors, not fraud, so a reviewer decides rather than an automatic reject.

## How the pipeline works

Stages, in order: completeness, image quality (blur, then skew), OCR, identifiers, registry status, name match, address match, document age, signatory ID. Each finding carries a code, a severity, a route, a message key and parameters (contracts/kyc-findings.json, contracts/kyc-messages.json). The final route is the strongest one: reject beats reupload, reupload beats manual_review, and manual_review beats pass. A reviewer should not spend time on a file the owner has to re-upload anyway.

- **Blur:** the variance of the Laplacian over non-blank pixels, read from the red channel. K04 scores 5. Every other page scores above 11,000, and the gate is set at 500.
- **Skew:** the angle (searched over ±15°) that maximises the variance of the horizontal ink profile. K02 reads 7.0°. Every other page reads 0. The page is rotated back before OCR. IMAGE_SKEWED is reported at 1.5° or more.
- **Watermark:** the pink SYNTHETIC SPECIMEN watermark has a high red value, so OCR reads the red channel thresholded at 150. This removes the watermark and keeps the text.
- **OCR:** tesseract (psm 6, TSV with word boxes). Labels are matched with fuzzy matching, because OCR garbles them ("Legaj hame"). Values wrapped onto a second line are joined. The signatory ID's label-above-value layout is parsed separately.
- **Identifiers:** full-page OCR misreads the monospace certificate font, for example "1Z" as "17Z". Each GSTIN and Udyam number is re-read from its own crop: upscaled 2×, strokes thickened by 1 px, padded, then OCR'd as one line (psm 7) restricted to A-Z, 0-9 and '-'. The result is resolved against the registry: exact, then OCR-confusion corrected (O/0, I/1/J/T, S/5, B/8), then one format repair. That repair takes positions 1-13 as read, then the fixed 'Z', then the check digit, and is used only if the registry knows the result. A GSTIN that is still unknown is GSTIN_CHECKSUM_INVALID when the checksum fails (with the closest registry entry for the reviewer) and GSTIN_NOT_FOUND when it passes.
- **Names:** the registry trade name is the reference. It is compared with the business part of the certificate legal name, the certificate trade name, the Udyam certificate and registry names, the bill's billed-to name and the signatory ID's business. Names match at 0.85 similarity or more (lowercase, & read as "and"), which tolerates OCR slips but not a different word. For proprietorships, the proprietor on the certificate must match the registry, and the signatory must be the proprietor.
- **Address:** the certificate and bill addresses are compared with the registry address using letters and digits only, because wrapped lines split words ("Maharashtr a"). The Udyam city and state must appear in the registered address.
- **Document age:** the bill must be at most 90 days old at KYC_REFERENCE_DATE (2026-07-15). Exactly 90 days passes and 91 fails.
- **Signatory ID:** the ID number must be present, and "valid till" must not be before the reference date.

## Field-by-field comparison of all 12 applicants (Phase 4 gap closed)

All fields on every document were compared with the registry. The full table is docs/kyc-field-comparison.md. Apart from the planted problems above, everything matches: names, constitutions, addresses (including PIN codes), GST date of liability against registration date, billed-to names and ID validity. The differences that remain:

| Applicants | Field | What differs | Treatment |
|---|---|---|---|
| K02, K03, K04, K06, K07, K08, K09, K10, K11, K12 | Udyam date of registration | Certificate date differs from registry for every applicant where both exist | Not checked: generator noise. Ask the organizers |
| K06, K10, K11, K12 | Udyam major activity | Certificate says Services, registry says Trading | Not checked: generator noise. Ask the organizers |
| K01, K08, K09 / K02, K07 | Signatory designation | Proprietorship signatory designated "Office admin" / "Manager", but the name is the proprietor's | Pass: the person is the registered proprietor |
| K05, K10, K11 | Signatory designation | LLP or company signatory designated "Owner". The registry has no partner or director list | Not flagged. Production needs an authorisation letter or an MCA partner/director lookup |
| K05 | Udyam | No certificate uploaded, but the registry has UDYAM-KL-00-5521886 for this business | DOC_MISSING stands. The owner could type the number instead (prefill) |
| K02, K03, K07, K08, K10 | GSTIN | OCR cannot tell O from 0 in this font | Resolved silently by registry correction (gstinMatchType=corrected) |

## Open questions for the organizers

- Are the Udyam date and activity differences intended (checks we should add) or generator noise (our assumption)?
- Should LLP and company signatories be verified against a partner or director list we do not have?
- Is manual_review the expected route for K06 and K07, or do they expect reject or reupload?
- The answer key may use different codes for the same problem, for example a skew finding at all for K02, which we report as info and pass.
