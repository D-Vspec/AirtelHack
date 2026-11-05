# 3-minute demo video: script and shot list

> Updated version with the account / KYC-reuse step and a full test matrix: `docs/video/Udyam_Digital_Video_Script.docx` (and `.pptx`), built from `docs/video/content.json`.

About 420 words of voice-over: 2:50 at a calm pace, leaving 10 s of slack. Record the screen at 1080p. Show the phone journey in a 390×844 browser window (or a real phone) next to the desktop back office.

**Before recording**
- Start everything: `docker compose up -d`, then `cd ts/apps/web && npx next start -p 3000`.
- Check the LLM key works: `make smoke`, then send one chat message.
- Run `make scenarios` once. The back office then has leads, a review and partner tasks.
- Phone window: http://localhost:3000. Desktop window: http://localhost:3000/admin (token `demo-admin-token`).
- Have `docs/Udyam_Digital_SYNC_Submission.pptx` open on slide 2, and a terminal open in the repo.

| Time | Shot (what is on screen) | Voice-over |
|---|---|---|
| 0:00–0:15 | Deck slide 2: the funnel chart and "1,485 drops (32%)" | "Five thousand small-business leads, three hundred and sixty-eight activations. A third of the losses are pure friction: no time for a sales visit, quotes that take days, paperwork. Udyam Digital replaces all of it with one phone session." |
| 0:15–0:25 | Phone: home screen. Tap **தமிழ்**, then the demo owner **Dental clinic, Chennai (Tamil)** | "An owner picks their language. Ten Indian languages, with fonts built in so it works on low-end phones." |
| 0:25–0:55 | Phone: chat. Press send on the prefilled Tamil message. The advisor answers in Tamil and the quote card appears with ₹3,933 | "Our AI advisor asks only what it needs and recommends a bundle in the owner's own language. The important part: the AI never invents a product or a price. Eligibility and pricing are deterministic rules, and a guard rejects any number the quote service didn't produce." |
| 0:55–1:10 | Phone: **See my quote**. Switch on **Backup 4G/5G Failover**; the total updates. Tap **Accept** | "One monthly price, the one-time charge separately, and add-ons with a single switch. Dependencies are enforced, so you can't buy CCTV without connectivity." |
| 1:10–1:20 | Phone: consent screen, then **I agree** | "Consent comes first, per purpose, under the DPDP Act. Nothing can be uploaded without it, and the owner can withdraw and delete everything in one tap." |
| 1:20–1:50 | Phone: KYC. Upload `data/kyc_documents/K04_gst_certificate.png`: "too blurry" appears instantly. Then **Use sample documents (K03)** → **Check my documents** → the progress ring fills and "All documents verified" shows | "Smart KYC catches a blurred or tilted photo the moment it is taken. OCR reads the certificates. GSTIN and Udyam numbers are checked against the registries, and then names, address, bill age and ID validity are cross-checked. All twelve test applicants, including the planted problems, come out as expected." |
| 1:50–2:15 | Phone: **Track order**. The stepper ticks through to **Live**; WhatsApp messages in Tamil appear on the right | "Instead of calling support three times, the owner gets a WhatsApp update at every milestone, in their language. If fibre isn't feasible at the address, the system offers 5G AirFiber instead of losing the order." |
| 2:15–2:30 | Desktop back office: **KYC review queue**. Open the K07 case (name mismatch) and click **Approve** | "Ambiguous cases, like a different name on one certificate, go to a human reviewer. We never silently reject a customer." |
| 2:30–2:45 | Desktop: **Leads by score**, then **Partner handoff** | "Field sales and partners get warm leads, ranked by a propensity model that finds two point four times more buyers than random calling, with the advisor's summary when it couldn't close." |
| 2:45–3:00 | Terminal: `make scenarios` ending in "33 passed, 0 failed". Then deck slide 8, the architecture | "Thirty-three user-story scenarios pass against the live system, and in our failure drills no event was lost. We start with professional offices, then multi-site businesses and manufacturers. Thank you." |

**If something goes wrong on the day**
- **The LLM is slow:** cut from the sent message straight to the quote card (keep the recording running and trim it later). Or use the desktop demo owner "Physio clinic where fibre is not feasible", which shows the delay, the 5G offer and the activation.
- **No live system:** narrate over `docs/screens/phone-01` … `phone-07` and `desktop-06-fallback-offer.png`.

**Upload checklist** (from the case):
- Demo video, 3 minutes maximum.
- Code repository.
- Architecture diagram: `docs/architecture-demo.png` plus `docs/architecture.md`.
- Business case, 10 slides or fewer: `docs/Udyam_Digital_SYNC_Submission.pptx`.
- 1-page privacy and security note: `docs/privacy-security-note.md`.
