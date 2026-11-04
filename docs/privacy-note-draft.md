# Privacy note (draft): KYC documents

Draft for review by compliance or the data protection officer. It describes what the KYC service (go/cmd/kyc) does today, written to match the Digital Personal Data Protection Act, 2023: purpose limitation, data minimisation, withdrawal of consent, and erasure.

## What we collect and why

| Data | Why | Where it is kept |
|---|---|---|
| Consent record: purposes, consent text version, language, time given, time withdrawn | Proves what the owner agreed to | kyc.consents |
| Document images: GST certificate, Udyam certificate, address proof (utility bill), signatory ID | Verify the business and the person who signs | kyc.documents, only until a decision (see retention) |
| Fields read from the documents: GSTIN, legal and trade name, constitution, address, Udyam number, bill name, address and date, signatory name, designation, ID number and expiry | Compare documents with each other and with the GST and Udyam registries | kyc.applications.result |
| Findings, final route, reviewer decision and note | Explain the outcome and keep an audit trail | kyc.applications, kyc.reviews |

We do not collect: Aadhaar, PAN images, photos of the person, location, contacts or device data. The raw OCR text is used during the check and never stored.

## Purposes

Consent is recorded per purpose before any upload. Uploads and checks answer 403 CONSENT_REQUIRED without `kyc_verification`.

- `kyc_verification` (required for KYC): checking the documents above.
- `service_provisioning`: using the verified business details to set up the ordered services.
- `communication`: messages about this application (status, re-upload requests).

Document data is not used for marketing, lead scoring or model training. The scoring model (Phase 7) uses lead-creation fields only.

## Retention

- Document images are deleted when a final decision is made: an automatic pass or reject, or a reviewer's approve or reject. Images are kept while the owner still has to re-upload something, so they only send the missing or failed document.
- Images that never reach a decision are deleted after `KYC_DOC_RETENTION_DAYS` (default 30). An hourly job runs the deletion.
- Extracted fields and findings stay while the consent is active, because provisioning and audit need them. Before launch, a retention period for these must be set with compliance (open item).

## Withdrawal and deletion

`POST /consents/{leadId}/withdraw` marks the consent withdrawn. In the same transaction it deletes every document image, the extracted fields and findings, and any open review for that lead, then publishes `consent.withdrawn` with the number of documents purged. A later upload needs fresh consent.

## Security and minimisation in the code

- Identifiers are masked in logs, events and API responses (`logx.MaskID`, for example `***********P1ZT`). A test checks that finding details do not contain a raw GSTIN.
- Uploads must be PNG or JPEG (checked by magic bytes, not the client's header) and at most 8 MB.
- The KYC service does not read other services' data. Registry lookups go over HTTP to the registry service.
- Kafka events carry codes, routes and masked identifiers only, never images or full ID numbers.

## Open items

- Retention period for extracted fields after the service is live.
- Encryption at rest for kyc.documents (today: Postgres volume encryption only, in a production deployment).
- Reviewer access control: the review queue API has no authentication in the demo. The Phase 8 gateway hardening must add it.
- A native-speaker review of the translated consent and finding messages (contracts/kyc-messages.json).
- A DPDP-compliant notice shown before consent, in the owner's language (Phase 6 UI).
