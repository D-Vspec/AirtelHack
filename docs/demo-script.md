# 3-minute demo script and shot list

**Setup before recording:**
- Start the stack: `make up seed`.
- Put the LLM key in `.env`.
- Start the web app: `make web`. Open http://localhost:3000 in a phone-sized window (390×844, or a real phone on the LAN), and http://localhost:3000/admin in a second window (token `demo-admin-token`).
- Run `make scenarios` once beforehand so the back office has data.
- Keep the simulator at its default (`SIM_STEP_SECONDS=8`), so activation takes about 30 s on camera.

| Time | Shot | On screen | Voice-over (about 25 words per 10 s) |
|---|---|---|---|
| 0:00–0:15 | Title, then the funnel from docs/business-case.md slide 1 | 7.4% activation; 1,485 drops fixable by digital | "5,000 small-business leads, 368 activations. A third of the losses are friction: no time for a visit, slow or confusing quotes, paperwork. We replace all of that with one phone session." |
| 0:15–0:25 | Phone: home screen, tap **தமிழ்**, then demo owner "Dental clinic, Chennai (Tamil)" | Language grid, Noto Tamil font | "An owner picks their language. Ten Indian languages, fonts self-hosted for low-end phones." |
| 0:25–0:55 | Phone: chat. Send the prefilled Tamil message; the advisor answers in Tamil with the bundle and price | Tamil reply, ₹3,933 a month, quote card | "The AI advisor asks only what it needs and recommends a bundle. But it never invents a product or a price: eligibility and pricing are deterministic code, and a guard rejects any number the quote service didn't return." |
| 0:55–1:10 | Phone: **See my quote**, toggle "Backup 4G/5G Failover" on, then **Accept** | Line items, total updates, accept | "Transparent quote: monthly, one-time and contract total, add-ons with dependencies enforced. One tap to accept." |
| 1:10–1:20 | Phone: consent screen, **I agree** | Three purposes | "Consent first, per purpose, under the DPDP Act. No upload without it, and the owner can withdraw and delete at any time." |
| 1:20–1:45 | Phone: KYC. Take a photo of a blurred certificate (K04 image): instant "too blurry". Then **Use sample documents (K03)**, **Check my documents**, "All documents verified" | Instant finding, then the pass banner | "Smart KYC: blur and skew are caught on upload. OCR reads the certificates, GSTIN and Udyam numbers are checked against the registries, and names, address, bill age and ID validity are cross-checked. All twelve specimen applicants come out as expected." |
| 1:45–2:10 | Phone: **Track order**. Steps tick to Live; WhatsApp messages in Tamil appear | Timeline, ETA, messages | "The tracker replaces status-chasing calls: every milestone goes out on WhatsApp in the owner's language. Events flow over Kafka with an outbox, so even if Kafka or a service goes down, nothing is lost." |
| 2:10–2:30 | Desktop admin: **KYC review queue**, open the K07 case (name mismatch), **Approve** | Findings with reviewer detail | "Ambiguous cases, like a different name on one certificate, go to a human. Never a silent rejection." |
| 2:30–2:45 | Desktop admin: **Leads by score**, then **Partner handoff** | Ranked list with reasons; handoff summary | "Field sales and partners get warm leads: ranked by a propensity model, 2.4 times better than random at finding buyers, with the advisor's summary when it can't close." |
| 2:45–3:00 | Terminal: `make scenarios` ending "33 passed"; then the architecture diagram | Scenario output, docs/architecture-demo.png | "33 user-story scenarios pass against the live stack. Professional offices first, then multi-site and manufacturers with field sales. Thank you." |

**Fallback shots if the LLM is slow on the day:**
- Use the desktop demo owner "Physio clinic where fibre is not feasible". It shows the delay, the 5G offer and the activation.
- Or replay `docs/screens/phone-*.png`.
