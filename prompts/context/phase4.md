# Context for Phase 4
- Read docs/kyc-findings.md and data/kyc_expected.json first. They list the planted problems found so far and the known gaps.
- Finding codes, severities and default routes: contracts/kyc-findings.json (generated into Go and TS by `make contracts`).
- Pipeline order: consent, completeness, image quality (blur then skew), OCR, identifier correction (checksum then registry fuzzy), registry status, name match, address match, document age, ID validity.
- Reference date for the 90-day rule: KYC_REFERENCE_DATE=2026-07-15 (from the case).
- OCR tooling: tesseract CLI. Add `apk add tesseract-ocr` to go/Dockerfile for the kyc image. The certificates carry a diagonal SYNTHETIC SPECIMEN watermark across the text, so preprocess (threshold, remove red channel) before OCR. Tesseract cannot distinguish O from 0 in this font.
- Specimen images are in data/kyc_documents (mounted read-only into the kyc container at /data/kyc_documents).
- Privacy: log only masked identifiers (logx.MaskID), store only needed fields, retention configurable, deletion on consent withdrawal.
- Unknown: the organizers' hidden answer key may differ from kyc_expected.json. Ask them before the demo if possible.
