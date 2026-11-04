# Privacy note (draft for compliance / DPO review)

Status: draft written by the build team for the hackathon prototype. It describes what the code in
go/cmd/kyc actually does. It is not legal advice and has not been reviewed against the Digital
Personal Data Protection Act, 2023 by a lawyer; that review is a pre-launch step.

## What we collect, and why

| Data | Where it comes from | Why we need it | Kept as |
|---|---|---|---|
| Consent record: lead ID, purposes, consent-text version, language, time granted / withdrawn | The owner ticks consent before uploading | Proof of what the owner agreed to, and when | `kyc.consents`, kept after withdrawal as the record of consent and withdrawal |
| GST certificate image | Upload | Verify GSTIN, legal and trade name, constitution, address | Image in `kyc.documents` until the retention limit; parsed fields with the check |
| Udyam certificate image | Upload | Verify Udyam number, enterprise name, city and state | Same |
| Address proof (utility bill) image | Upload | Verify the business address and that the bill is recent (90 days) | Same; consumer number is dropped after OCR |
| Authorised signatory ID image | Upload | Verify the signatory is the proprietor and the ID is valid | Same; the ID number is dropped after OCR |
| Check results: route, finding codes, reviewer-facing detail | The KYC pipeline | Decide pass / re-upload / manual review / reject, and audit that decision | `kyc.checks`, `kyc.reviews` |
| Reviewer decision, reviewer ID, note | Operations staff | Close manual reviews | `kyc.reviews` |

Purposes the owner can consent to: `kyc_verification` (required for any upload) and
`service_activation`. Documents are not used for marketing, scoring or model training. The LLM
advisor never sees documents or their contents; KYC verdicts are deterministic code.

## What we do not keep

- The signatory ID number and the electricity consumer number are removed from the parsed fields
  as soon as OCR finishes; the checks do not need them after reading.
- Logs never contain raw GSTIN, Udyam or ID numbers: identifiers are masked to their last four
  characters (`logx.MaskID`); reviewer-facing details are masked the same way.
- Registry lookups send only the GSTIN or Udyam number to the registry service.

## Retention

- Document images are deleted automatically `KYC_RETENTION_DAYS` days after upload (default 30,
  set in `.env`); an hourly sweep in the KYC service does this.
- Parsed fields, findings and review decisions stay with the check as the audit record of the KYC
  decision. Their retention period is a decision for compliance (open question below).

## Withdrawal and deletion

- `DELETE /consents/{leadId}` (withdraw) immediately deletes the lead's document images, checks
  (with their parsed personal fields) and reviews, in one transaction, and emits
  `consent.withdrawn` so other services can purge too. The consent row stays, marked withdrawn.
- After withdrawal, uploads and checks are refused (403 CONSENT_REQUIRED) until consent is given
  again.

## Security

- Documents travel over the internal network between the gateway and the KYC service; the
  prototype runs without TLS inside docker compose. Production needs TLS, encryption at rest for
  the documents table (or object storage with server-side encryption), and access control on the
  review queue.

## Open questions for compliance

1. Retention period for check results and review decisions (audit record) after the customer is
   activated, and after a rejection.
2. Whether `consent.withdrawn` must also trigger deletion in the tracker and notify services
   (Phase 5).
3. Wording and languages of the consent text, and how the version is managed.
4. Whether manual reviewers may see the full document images or only the parsed fields.
