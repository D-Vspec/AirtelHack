# Privacy and security note (1 page)

For: compliance / DPO, KYC and risk, and the hackathon judges. It describes what the prototype does today, mapped to the Digital Personal Data Protection Act 2023. Longer KYC detail: `docs/privacy-note-draft.md`.

**What we collect and why (purpose limitation, minimisation)**
- **Advisor:** business profile (type, city, staff, sites, GSTIN yes/no, current provider and spend, products of interest, language) and the chat. Purpose: recommend and price a bundle. No ID numbers are needed.
- **KYC:** four documents (GST certificate, Udyam certificate, utility bill, signatory ID) and the fields read from them. Purpose: verify the business for this order. We do not collect Aadhaar, PAN images, photos of people, location, contacts or device data.
- **Orders and messages:** order status, and message text sent to the owner. The demo has no phone numbers or e-mail addresses; mock adapters address the lead ID.
- **Lead score:** computed from the profile above only. It orders call lists and never affects eligibility, price, KYC or service.

**Consent and rights**
- Consent is recorded per purpose (kyc_verification, service_provisioning, communication) with the text version and language. KYC uploads and checks are refused (403) without it.
- **Withdrawal** (one tap in the KYC screen) deletes the images, extracted fields, findings and open reviews in one transaction and emits `consent.withdrawn`.

**Retention**
- Images: deleted at the final decision (pass, reject or reviewer decision), or after 30 days (`KYC_DOC_RETENTION_DAYS`).
- Customer accounts: mobile number + one-time code (codes and session tokens stored only as SHA-256 hashes). The verified facts from a passed check (not the images) let a later order skip KYC for up to 365 days, only if the owner confirms nothing changed and a live registry re-check matches; withdrawing consent on that order deletes them.
- Extracted fields and findings: kept while consent is active. A fixed period must be set with compliance before launch.
- Resume links expire after 7 days and carry only a random token.

**Security controls in the code**
- **Isolation:** each service has its own Postgres role (`infra/postgres/roles.sql`) and can read only its own schema. Verified: `svc_kyc` reading `quote.quotes` gets "permission denied for schema quote".
- **Gateway:**
  - Per-IP rate limits: 30 req/s by default, 0.5 req/s with a burst of 6 for advisor chat (LLM cost), 1 req/s for uploads (OCR cost).
  - A JSON 503 when a service is down; CORS.
  - Back-office routes (review queue, partner tasks, scores, order transitions, pipeline lists, `/metrics`) need `X-Admin-Token`, compared in constant time. With no token configured, they are closed.
- **Input validation:** strict JSON decoding (unknown fields rejected, 1 MB cap); lead IDs restricted to `[A-Za-z0-9_.:-]{1,64}`; uploads accepted only as PNG or JPEG by magic bytes, at most 8 MB.
- **PII masking:** GSTIN, Udyam and ID numbers are masked in logs, events and API responses (`logx.MaskID`); a test checks findings never carry a raw GSTIN. Kafka events carry codes and masked IDs, never images.
- **Deterministic decisions:** the LLM never decides eligibility, price or KYC. A guard rejects replies containing any number or product the services did not return.
- **Dependencies:**
  - `govulncheck` reports **no vulnerabilities** for the Go modules and toolchain the images are built with. This needed pgx 5.12, x/crypto 0.58, x/text 0.43 and Go 1.26.
  - `pnpm audit --prod`: 2 moderate findings (glob, minimatch), both inside Expo's build tooling and not shipped in any service. Fix with the next Expo SDK upgrade.
- **Secrets:** API keys and passwords live in `.env` (gitignored). Production would use one secret per database role from a vault.

**Known gaps before production**
- No owner login or OTP: owner routes are scoped to unguessable lead IDs.
- The admin token is a shared secret, not per-user SSO with an audit trail.
- No TLS inside the compose network.
- Encryption at rest relies on the disk volume.
- The review queue does not log who viewed which document.
- Fairness review of the lead score (language, city tier) is pending.
- UI and finding translations are machine-drafted.
