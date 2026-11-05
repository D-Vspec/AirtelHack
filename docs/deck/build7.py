"""7-slide submission deck (SYNC form asks for 7 slides, PDF + PPTX).

Reuses the helpers and slides of build.py: problem, target customer, bundles and architecture are
kept; journey + KYC are merged, and rollout + risks + asks are merged. Field/partner facts move into
the merged slides. Run: python docs/deck/build7.py  ->  docs/Udyam_Digital_SYNC_Submission_7slides.pptx
"""
from pathlib import Path

SRC = (Path(__file__).parent / "build.py").read_text(encoding="utf-8")
head, rest = SRC.split("# ---- 1. Title", 1)
exec(compile(head, "build.py", "exec"))


def section(n, upto):
    """Source of build.py's slide n up to (not including) slide `upto`."""
    body = "# ---- " + rest if n == 1 else "# ---- " + str(n) + ". " + rest.split("# ---- " + str(n) + ". ", 1)[1]
    return body.split("# ---- " + str(upto) + ". ", 1)[0] if upto else body.split("prs.save(OUT)")[0]


for n, upto in [(1, 5)]:          # 1 title, 2 problem, 3 target customer, 4 bundles
    exec(compile(section(n, upto), "build.py", "exec"))

# ---- 5. Journey + KYC ----------------------------------------------------------------------------------
s = new_content_slide()
title(s, "One phone session replaces the sales visit, KYC included", "Language → AI advisor → quote → consent → smart KYC → order tracker")
shots = [("shot-chat.png", "Advisor in Tamil"), ("shot-quote.png", "Clear quote"), ("shot-kyc.png", "Smart KYC"), ("shot-track.png", "Live tracker")]
for i, (f, cap) in enumerate(shots):
    x = 0.55 + i * 1.42
    picture(s, BUILD / f, x, 1.58, h=2.45)
    badge(s, x, 4.1, str(i + 1), d=0.26, size=9)
    text(s, x + 0.3, 4.1, 1.1, 0.26, cap, size=9.5, bold=True)
text(s, 0.55, 4.45, 5.6, 0.6, [
    [("KYC under the DPDP Act 2023: ", {"bold": True}),
     ("consent per purpose before upload · blur/skew caught instantly · GSTIN and Udyam registry checks · names, address, "
      "bill age cross-checked · unclear cases go to a human · images deleted at decision or 30 days", {})]],
    size=9, color=GREY)
proof = [("27/27", "advisor eval conversations pass, 11 of them multilingual"),
         ("10", "languages, plus Hinglish and Tanglish"),
         ("12/12", "KYC specimen applicants give the expected verdict (pass, re-upload, review, reject)"),
         ("0", "invented products or prices: a guard blocks any number the services didn't return")]
for i, (big, d) in enumerate(proof):
    y = 1.58 + i * 0.85
    box(s, 6.4, y, 3.05, 0.76, fill=TINT if i == 2 else PANEL)
    text(s, 6.5, y + 0.08, 1.0, 0.6, big, size=19, bold=True, color=RED, anchor=MSO_ANCHOR.MIDDLE)
    text(s, 7.45, y + 0.06, 1.95, 0.66, d, size=9, color=INK, anchor=MSO_ANCHOR.MIDDLE)
s.notes_slide.notes_text_frame.text = (
    "This is the whole journey on a phone. The owner picks a language and chats with the advisor, which recommends within six turns. "
    "The quote shows one monthly price. Consent comes before any upload; blur and skew are caught on the spot; GSTIN and Udyam are "
    "verified against the registries and the documents are cross-checked. Unclear cases go to a human reviewer, never a silent reject, "
    "and images are deleted once a decision is made. Then a tracker messages the owner at every milestone. All 27 evaluation "
    "conversations and all 12 KYC test applicants come out as expected.")

# ---- 6. Architecture (from build.py slide 8) --------------------------------------------------------------
exec(compile(section(8, 9).replace("owner OTP + partner ReBAC next", "owner OTP sign-in live; partner ReBAC next").replace("owner OTP login with partner access rules is next", "owners sign in with an OTP; partner access rules are next"), "build.py", "exec"))

# ---- 7. Rollout, KPIs, risks and asks ------------------------------------------------------------------------
s = new_content_slide()
title(s, "Rollout, weekly KPIs and what we need to launch", "Targets are proposals to agree with Accel Business sales leadership")
phases = [("Pilot · wk 0–8", "Professional offices · 8 Tier 1 cities · English, Hindi, Tamil"),
          ("Wave 2 · wk 9–20", "Multi-site + factories: field sales close · partners get scored warm leads (2.4× in top decile)"),
          ("Scale · wk 21+", "Micro retail self-serve · Tier 2 cities · WhatsApp entry point")]
for i, (h, d) in enumerate(phases):
    x = 0.55 + i * 1.97
    box(s, x, 1.58, 1.87, 1.38, fill=TINT if i == 0 else PANEL)
    badge(s, x + 0.12, 1.68, str(i + 1), d=0.3, size=10)
    text(s, x + 0.5, 1.66, 1.35, 0.4, h, size=9.5, bold=True)
    text(s, x + 0.12, 2.1, 1.65, 0.85, d, size=9, color=INK)
kpis = [("Lead → activation", "11.1%", "15%"), ("Fixable drops", "32%", "< 15%"), ("Quote → KYC done", "68.3%", "85%"),
        ("Days to activate", "17.6", "< 10"), ("CAC, prof. offices", "₹10,695", "< ₹7,500")]
for i, (k, base, target) in enumerate(kpis):
    x = 0.55 + i * 1.18
    box(s, x, 3.08, 1.1, 1.0, fill=PANEL)
    text(s, x + 0.08, 3.12, 1.0, 0.3, k, size=8, bold=True, color=GREY)
    text(s, x + 0.08, 3.38, 1.0, 0.25, base + " now", size=9, color=MUTED)
    text(s, x + 0.08, 3.6, 1.0, 0.4, target, size=15, bold=True, color=RED)
text(s, 0.55, 4.18, 5.8, 0.85, [
    [("Risks designed for: ", {"bold": True}),
     ("prices are code and a reply guard blocked 6/6 prompt-injection attacks · per-language evals · DPDP consent, deletion, masked IDs · "
      "score only orders call lists · partners credited for handoffs · no fibre → automatic 5G AirFiber offer", {})]],
    size=9, color=GREY)
box(s, 6.55, 1.58, 2.9, 3.42, fill=INK)
text(s, 6.72, 1.68, 2.6, 0.3, "Asks", size=14, bold=True, color=WHITE)
text(s, 6.72, 2.03, 2.6, 2.9, [
    [("Sales leadership: ", {"bold": True}), ("approve the professional-offices pilot and targets", {})],
    [("Field & partners: ", {"bold": True}), ("handoff workflow and partner credit", {})],
    [("Compliance / DPO: ", {"bold": True}), ("consent text and retention periods", {})],
    [("KYC & risk: ", {"bold": True}), ("review-queue SLAs and routing policy", {})],
    [("Network ops: ", {"bold": True}), ("fibre feasibility data, install dates", {})],
    [("Billing & support: ", {"bold": True}), ("one bill, tracker for 'where is my order'", {})]],
    size=9.5, color=WHITE, spacing=4)
s.notes_slide.notes_text_frame.text = (
    "We roll out in three waves: an eight-week pilot with professional offices in Tier 1 cities, then multi-site businesses and "
    "manufacturers with field sales closing and partners receiving scored warm leads, then micro retail self-serve. We'd track five "
    "KPIs weekly, from 11 to 15 percent lead-to-activation and from 17.6 to under 10 days to activate. The main risks are designed "
    "for: the AI can't invent prices, every language is evaluated, privacy follows the DPDP Act. To launch we need the pilot approved, "
    "compliance sign-off on consent and retention, and network and billing to plug in feasibility and one-bill invoicing. Thank you.")

OUT7 = DOCS / "Udyam_Digital_SYNC_Submission_7slides.pptx"
prs.save(OUT7)
print("saved", OUT7, len(prs.slides), "slides")
