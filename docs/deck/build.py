"""Builds the Udyam Digital submission deck on the Airtel SYNC template (10 slides).

Every content slide reuses the template's content background (logos, red bar); everything on top
is native PowerPoint: text, shapes, charts, pictures. Run: python docs/deck/build.py (python-pptx, Pillow). Numbers come from docs/analytics.md, the case
PDF and measured results in the repo.
"""
import copy
from pathlib import Path

from pptx import Presentation
from pptx.chart.data import CategoryChartData
from pptx.dml.color import RGBColor
from pptx.enum.chart import XL_CHART_TYPE, XL_LABEL_POSITION
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import MSO_ANCHOR, PP_ALIGN
from pptx.util import Emu, Inches, Pt

HERE = Path(__file__).parent
DOCS = HERE.parent
OUT = DOCS / "Udyam_Digital_SYNC_Submission.pptx"
BUILD = HERE / "build"  # extracted background + cropped screenshots (gitignored)


def prepare():
    """Extract the template's content background and crop phone screenshots from docs/screens."""
    import zipfile

    from PIL import Image

    BUILD.mkdir(exist_ok=True)
    with zipfile.ZipFile(HERE / "template.pptx") as z:
        (BUILD / "image1.jpg").write_bytes(z.read("ppt/media/image1.jpg"))
    for k, f in {"chat": "phone-02-chat.png", "quote": "phone-03-quote.png", "kyc": "phone-05-kyc.png", "track": "phone-07-activated.png"}.items():
        im = Image.open(DOCS / "screens" / f).convert("RGB")
        w, h = im.size
        im.crop((0, 0, w, min(h, int(w * 2.05)))).save(BUILD / f"shot-{k}.png")


prepare()

RED = RGBColor(0xE4, 0x00, 0x00)
DARK_RED = RGBColor(0xA8, 0x00, 0x00)
INK = RGBColor(0x1A, 0x1A, 0x1A)
GREY = RGBColor(0x55, 0x55, 0x5E)
MUTED = RGBColor(0x8A, 0x8A, 0x93)
TINT = RGBColor(0xFD, 0xEE, 0xEE)      # light red tint for cards
PANEL = RGBColor(0xF4, 0xF4, 0xF6)     # neutral card
WHITE = RGBColor(0xFF, 0xFF, 0xFF)
GREEN = RGBColor(0x15, 0x80, 0x3D)
FONT = "Arial"

prs = Presentation(HERE / "template.pptx")
title_slide, content_proto = prs.slides[0], prs.slides[1]
content_layout = content_proto.slide_layout
bg_image = BUILD / "image1.jpg"


def new_content_slide():
    s = prs.slides.add_slide(content_layout)
    for ph in list(s.placeholders):  # the layout's empty title/body placeholders
        ph._element.getparent().remove(ph._element)
    pic = s.shapes.add_picture(str(bg_image), 0, 0, prs.slide_width, prs.slide_height)
    pic.name = "Background"
    s.shapes._spTree.remove(pic._element)
    s.shapes._spTree.insert(2, pic._element)  # behind everything
    return s


def text(slide, x, y, w, h, runs, size=13, color=INK, bold=False, align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP, name=None, spacing=None):
    """runs: str, or list of paragraphs; a paragraph is a str or a list of (text, {opts})."""
    tb = slide.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    if name:
        tb.name = name
    tf = tb.text_frame
    tf.word_wrap = True
    tf.margin_left = tf.margin_right = Inches(0.02)
    tf.margin_top = tf.margin_bottom = Inches(0.02)
    tf.vertical_anchor = anchor
    paras = runs if isinstance(runs, list) else [runs]
    for i, para in enumerate(paras):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = align
        if spacing:
            p.space_after = Pt(spacing)
        parts = para if isinstance(para, list) else [(para, {})]
        for t, o in parts:
            r = p.add_run()
            r.text = t
            f = r.font
            f.name = FONT
            f.size = Pt(o.get("size", size))
            f.bold = o.get("bold", bold)
            f.italic = o.get("italic", False)
            f.color.rgb = o.get("color", color)
    return tb


def box(slide, x, y, w, h, fill=PANEL, radius=True, name=None, line=None):
    shp = slide.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE if radius else MSO_SHAPE.RECTANGLE, Inches(x), Inches(y), Inches(w), Inches(h))
    if radius:
        shp.adjustments[0] = 0.12
    shp.fill.solid()
    shp.fill.fore_color.rgb = fill
    if line:
        shp.line.color.rgb = line
        shp.line.width = Pt(1)
    else:
        shp.line.fill.background()
    shp.shadow.inherit = False
    if name:
        shp.name = name
    return shp


def badge(slide, x, y, label, d=0.42, fill=RED, size=13):
    """The deck's motif: a red circle with a number or short label."""
    c = slide.shapes.add_shape(MSO_SHAPE.OVAL, Inches(x), Inches(y), Inches(d), Inches(d))
    c.fill.solid()
    c.fill.fore_color.rgb = fill
    c.line.fill.background()
    c.shadow.inherit = False
    tf = c.text_frame
    tf.margin_left = tf.margin_right = tf.margin_top = tf.margin_bottom = 0
    tf.vertical_anchor = MSO_ANCHOR.MIDDLE
    p = tf.paragraphs[0]
    p.alignment = PP_ALIGN.CENTER
    r = p.add_run()
    r.text = label
    r.font.name, r.font.size, r.font.bold, r.font.color.rgb = FONT, Pt(size), True, WHITE
    return c


def title(slide, t, sub=None):
    text(slide, 0.55, 0.74, 8.9, 0.5, t, size=22, bold=True, color=INK, name="Title")
    if sub:
        text(slide, 0.55, 1.22, 8.9, 0.3, sub, size=12, color=GREY, name="Subtitle")


def stat(slide, x, y, w, big, label, color=RED, big_size=26):
    text(slide, x, y, w, 0.5, big, size=big_size, bold=True, color=color)
    text(slide, x, y + 0.5, w, 0.5, label, size=11, color=GREY)


def picture(slide, path, x, y, w=None, h=None, border=True):
    pic = slide.shapes.add_picture(str(path), Inches(x), Inches(y), Inches(w) if w else None, Inches(h) if h else None)
    if border:
        pic.line.color.rgb = RGBColor(0xDD, 0xDD, 0xE2)
        pic.line.width = Pt(0.75)
    return pic


def style_chart(chart, colors):
    chart.has_legend = False
    chart.font.name = FONT
    chart.font.size = Pt(10)
    chart.font.color.rgb = GREY
    chart.has_title = False
    plot = chart.plots[0]
    plot.gap_width = 45
    plot.has_data_labels = True
    dl = plot.data_labels
    dl.font.size = Pt(10)
    dl.font.bold = True
    dl.font.color.rgb = INK
    dl.position = XL_LABEL_POSITION.OUTSIDE_END
    ser = plot.series[0]
    for i, c in enumerate(colors):
        pt = ser.points[i]
        pt.format.fill.solid()
        pt.format.fill.fore_color.rgb = c
    va = chart.value_axis
    va.visible = False
    va.has_major_gridlines = False
    ca = chart.category_axis
    ca.format.line.fill.background()
    ca.tick_labels.font.size = Pt(10)
    ca.tick_labels.font.color.rgb = INK
    ca.has_major_gridlines = False


# ---- 1. Title ------------------------------------------------------------------------------------
s = title_slide
text(s, 0.62, 2.62, 4.6, 0.6, "Udyam Digital", size=34, bold=True, color=WHITE, name="Title")
text(s, 0.62, 3.2, 4.4, 0.75, "Making small businesses go digital in one sitting: on a phone, in their own language", size=14, color=WHITE)
text(s, 0.62, 4.1, 4.4, 0.5, [[("YTL Hackathon 2026 · Enterprise Track A", {"bold": True})], "AI advisor · transparent quote · smart KYC · order tracker"], size=11, color=RGBColor(0xF0, 0xC8, 0xC8))
s.notes_slide.notes_text_frame.text = (
    "Udyam Digital is a phone-first journey that takes a small-business owner from 'what do I need?' to a live connection: "
    "an AI advisor in their language recommends a bundle, a transparent quote follows, KYC is checked in minutes against "
    "the GSTIN and Udyam registries, and a tracker replaces the status-chasing calls. Everything you'll see is a working prototype.")

# ---- 2. Problem -----------------------------------------------------------------------------------
s = content_proto
for ph in list(s.placeholders):
    ph._element.getparent().remove(ph._element)
title(s, "93 of 100 leads are lost, a third to fixable friction", "5,000 SMB leads, Apr–Jun 2026 (case data, smb_leads.csv)")
cd = CategoryChartData()
cd.categories = ["Leads", "Contacted", "Needs assessed", "Quote accepted", "KYC complete", "Activated"]
cd.add_series("Leads", (5000, 3406, 1974, 861, 588, 368))
gf = s.shapes.add_chart(XL_CHART_TYPE.BAR_CLUSTERED, Inches(0.5), Inches(1.6), Inches(4.9), Inches(3.4), cd)
ch = gf.chart
style_chart(ch, [GREY, GREY, GREY, RED, GREY, DARK_RED])
ch.category_axis.reverse_order = True
ch.plots[0].data_labels.number_format = '#,##0'
ch.plots[0].data_labels.number_format_is_linked = False
text(s, 0.6, 4.95, 4.8, 0.25, "Steepest drop: needs assessed → quote accepted (44%)", size=10, color=MUTED)
# right: where the losses are
box(s, 5.65, 1.6, 3.8, 1.55, fill=TINT)
text(s, 5.85, 1.7, 3.5, 0.5, "1,485 drops (32%)", size=24, bold=True, color=RED)
text(s, 5.85, 2.2, 3.5, 0.95, "are fixable by a digital journey: no time for a visit 531, visit never scheduled 296, quote too complex 206 or too slow 179, paperwork and documents 273", size=11, color=INK)
stat(s, 5.75, 3.3, 1.2, "17.6", "days lead to activation", color=INK, big_size=22)
stat(s, 7.0, 3.3, 1.2, "3–5", "days for a quote today", color=INK, big_size=22)
stat(s, 8.25, 3.3, 1.25, "3×", "status chases per customer", color=INK, big_size=22)
stat(s, 5.75, 4.25, 1.9, "₹15,000", "blended cost to acquire one SMB", color=INK, big_size=22)
stat(s, 7.75, 4.25, 1.75, "1,594", "lost before first contact (34%)", color=INK, big_size=22)
s.notes_slide.notes_text_frame.text = (
    "Out of 5,000 leads only 368 activated: 7.4 percent. The biggest leak is between needs assessment and an accepted quote. "
    "When we grouped the 4,632 drop reasons, 1,485, about a third, are pure friction: owners with no time for a visit, visits "
    "never scheduled, quotes that were confusing or took days, and paperwork. Today it takes 17.6 days to activate, and every "
    "customer chases status three times. Those are exactly the steps a phone journey removes.")

# ---- 3. Target customer ----------------------------------------------------------------------------
s = new_content_slide()
title(s, "Start with professional offices: high conversion, low CAC", "Conversion and economics by segment (case data)")
rows = [
    ("Segment", "Leads", "Conv.", "CAC", "Bundle/mo", "Churn"),
    ("Professional offices", "1,247", "11.1%", "₹10,695", "₹6,796", "13.8%"),
    ("Small multi-site", "755", "11.0%", "₹15,614", "₹31,233", "10.8%"),
    ("Small manufacturers", "289", "12.8%", "₹15,324", "₹19,980", "8.1%"),
    ("Micro retail & services", "2,709", "4.1%", "₹20,007", "₹2,505", "22.7%"),
]
tbl = s.shapes.add_table(len(rows), len(rows[0]), Inches(0.55), Inches(1.62), Inches(5.6), Inches(2.3)).table
widths = [1.75, 0.65, 0.85, 0.8, 0.95, 0.6]
for i, w in enumerate(widths):
    tbl.columns[i].width = Inches(w)
for r, row in enumerate(rows):
    for c, val in enumerate(row):
        cell = tbl.cell(r, c)
        cell.text = ""
        p = cell.text_frame.paragraphs[0]
        run = p.add_run()
        run.text = val
        run.font.name, run.font.size = FONT, Pt(10.5 if r else 10)
        run.font.bold = r == 0 or (r == 1)
        run.font.color.rgb = WHITE if r == 0 else (RED if r == 1 and c in (2, 3) else INK)
        p.alignment = PP_ALIGN.LEFT if c == 0 else PP_ALIGN.RIGHT
        cell.fill.solid()
        cell.fill.fore_color.rgb = INK if r == 0 else (TINT if r == 1 else (WHITE if r % 2 else PANEL))
        cell.margin_left = cell.margin_right = Inches(0.06)
        cell.vertical_anchor = MSO_ANCHOR.MIDDLE
text(s, 0.55, 4.05, 5.6, 0.9, [
    [("Why not micro retail first? ", {"bold": True}), ("It is 54% of leads, but converts at 4.1%, costs the most to acquire and churns most. It stays self-serve only, with a light mobile + POS bundle.", {})]],
    size=11, color=GREY)
waves = [("1", "Wave 1 · Professional offices", "Clinics, CA firms, law offices, coaching centres: standard bundle, fully self-serve"),
         ("2", "Wave 2 · Multi-site & factories", "Highest bundle value (₹31,233 and ₹19,980/month): digital quote, field sales close"),
         ("3", "Wave 3 · Micro retail", "Self-serve, GSTIN-free options; GST registration unlocks more (9.6% vs 2.0% activation)")]
for i, (n, h, d) in enumerate(waves):
    y = 1.62 + i * 1.12
    box(s, 6.45, y, 3.0, 1.0, fill=TINT if i == 0 else PANEL)
    badge(s, 6.58, y + 0.13, n, d=0.38)
    text(s, 7.05, y + 0.08, 2.35, 0.32, h, size=11, bold=True)
    text(s, 7.05, y + 0.4, 2.35, 0.6, d, size=9.5, color=GREY)
s.notes_slide.notes_text_frame.text = (
    "We start where the economics already work. Professional offices convert at 11 percent with the lowest acquisition cost, and they "
    "buy a standard bundle that can be sold end to end on a phone. Multi-site businesses and manufacturers bring the biggest bundles, "
    "so the digital journey qualifies and quotes them and field sales closes. Micro retail is more than half the leads but converts at 4 "
    "percent and churns most, so it stays self-serve and light.")

# ---- 4. Value, bundles, pricing -----------------------------------------------------------------------
s = new_content_slide()
title(s, "Clear bundles, honest prices, set by rules, never by the AI", "Priced live by our quote engine inside each product's catalogue range")
cards = [
    ("Office ready", "6-person clinic · Tier 1", "₹3,933", "/month · ₹0 one-time · 12 months",
     ["Biz Fibre 300", "Cloud email × 6", "Cloud backup 1 TB", "Endpoint security × 6", "Optional 4G/5G failover"]),
    ("Multi-site", "5-outlet restaurant chain", "₹19,595", "/month · ₹43,000 one-time",
     ["Fibre + failover per outlet", "Cloud CCTV × 10", "Cloud POS per outlet", "Guest Wi-Fi", "One bill for all sites"]),
    ("Mobile quick path", "Owner who wants only SIMs", "1-month", "contract · no forced bundle",
     ["Business postpaid lines", "Upgrade later in one tap", "Soft upsell, never pushed", "", ""]),
]
for i, (name, who, price, sub, items) in enumerate(cards):
    x = 0.55 + i * 3.0
    box(s, x, 1.6, 2.8, 2.6, fill=TINT if i == 0 else PANEL)
    text(s, x + 0.18, 1.7, 2.5, 0.3, name, size=13, bold=True)
    text(s, x + 0.18, 1.98, 2.5, 0.25, who, size=10, color=GREY)
    text(s, x + 0.18, 2.25, 2.5, 0.45, price, size=24, bold=True, color=RED)
    text(s, x + 0.18, 2.7, 2.5, 0.25, sub, size=9.5, color=GREY)
    text(s, x + 0.18, 3.0, 2.5, 1.2, [it for it in items if it], size=10, color=INK, spacing=1)
value = [("Minutes, not 3–5 days", "for a quote: the advisor prices live, in the owner's language"),
         ("No sales visit needed", "removes the top drop reason (531 leads had no time for a visit)"),
         ("“Too expensive” handled", "cheaper eligible tier shown with the total contract cost")]
for i, (h, d) in enumerate(value):
    x = 0.55 + i * 3.0
    badge(s, x, 4.37, "✓", d=0.34, size=12)
    text(s, x + 0.42, 4.33, 2.4, 0.25, h, size=11, bold=True)
    text(s, x + 0.42, 4.58, 2.4, 0.45, d, size=9.5, color=GREY)
s.notes_slide.notes_text_frame.text = (
    "Here are three real quotes from the prototype. A six-person clinic gets fibre, email for everyone, backup and device security for "
    "3,933 rupees a month with no setup fee. A five-outlet restaurant chain gets per-site connectivity with failover, CCTV and POS on one "
    "bill. An owner who only wants SIMs gets a one-month mobile plan, no forced bundle. Prices are calculated by deterministic rules from "
    "the catalogue, so the AI can explain them but never make them up.")

# ---- 5. Journey ---------------------------------------------------------------------------------------
s = new_content_slide()
title(s, "One phone session replaces the sales visit", "Language → AI advisor → quote → consent → smart KYC → order tracker")
shots = [("shot-chat.png", "Advisor in Tamil"), ("shot-quote.png", "Transparent quote"), ("shot-kyc.png", "Smart KYC"), ("shot-track.png", "Live tracker")]
for i, (f, cap) in enumerate(shots):
    x = 0.55 + i * 1.42
    picture(s, BUILD / f, x, 1.6, h=2.75)
    badge(s, x, 4.42, str(i + 1), d=0.28, size=10)
    text(s, x + 0.32, 4.43, 1.05, 0.28, cap, size=10, bold=True)
proof = [("27/27", "advisor eval conversations pass (DeepSeek), incl. 11 multilingual"),
         ("10", "languages + Hinglish and Tanglish; Noto fonts, low-end phones"),
         ("0", "invented products or prices: a guard blocks any number the services didn't return"),
         ("33/33", "user-story scenarios pass on the live system")]
for i, (big, d) in enumerate(proof):
    y = 1.6 + i * 0.86
    text(s, 6.35, y, 1.0, 0.45, big, size=22, bold=True, color=RED)
    text(s, 7.4, y + 0.04, 2.05, 0.8, d, size=10, color=INK)
s.notes_slide.notes_text_frame.text = (
    "This is the whole journey on a phone. The owner picks a language and chats with the advisor, which asks only what it needs and "
    "recommends within six turns. The quote shows one monthly price with add-ons to toggle. Consent comes before any upload, then KYC, "
    "then a tracker that messages the owner at every milestone. On our evaluation set every conversation passed, across ten languages "
    "including code-mixed Hinglish and Tanglish, and all 33 user-story scenarios pass against the running system.")

# ---- 6. KYC + DPDP ------------------------------------------------------------------------------------
s = new_content_slide()
title(s, "KYC in minutes that respects the DPDP Act 2023", "Consent first · purpose limitation · minimisation · deletion on withdrawal")
picture(s, BUILD / "shot-kyc.png", 0.55, 1.6, h=3.4)
checks = [("Consent per purpose", "verification, provisioning, messages; no upload without it"),
          ("Instant image checks", "blur and skew caught on upload; pages straightened before OCR"),
          ("Registry-verified", "GSTIN checksum + registry status, Udyam lookup, O/0 OCR fixes"),
          ("Cross-document checks", "names, address, bill age (90 days), signatory ID validity"),
          ("Minimal & deletable", "4 documents; images deleted at decision or 30 days; one-tap withdrawal")]
for i, (h, d) in enumerate(checks):
    y = 1.6 + i * 0.68
    badge(s, 2.45, y + 0.04, str(i + 1), d=0.34, size=11)
    text(s, 2.9, y, 3.3, 0.28, h, size=11.5, bold=True)
    text(s, 2.9, y + 0.27, 3.3, 0.4, d, size=9.5, color=GREY)
box(s, 6.45, 1.6, 3.0, 3.4, fill=TINT)
text(s, 6.65, 1.72, 2.7, 0.5, "12 / 12", size=30, bold=True, color=RED)
text(s, 6.65, 2.25, 2.65, 0.5, "specimen applicants come out exactly as expected", size=11, bold=True)
text(s, 6.65, 2.8, 2.65, 2.1, [
    "Cancelled GSTIN → reject", "Blurred / missing → re-upload", "Stale bill (> 90 days) → fresh one", "Name or checksum mismatch → human review, never a silent reject", "Skewed page → auto-corrected, passes", "5 clean applicants → pass"],
    size=9.5, color=INK, spacing=2)
s.notes_slide.notes_text_frame.text = (
    "KYC is where onboarding usually stalls. Consent is recorded per purpose before anything is uploaded. Blur and skew are caught the "
    "moment a photo is taken. OCR reads the certificates and every identifier is verified against the GSTIN and Udyam registries, then "
    "names, address, bill age and ID validity are cross-checked. Ambiguous cases go to a human reviewer. All twelve test applicants, "
    "including the planted problems, come out as expected, and images are deleted as soon as a decision is made.")

# ---- 7. Field & partners ----------------------------------------------------------------------------
s = new_content_slide()
title(s, "Field sales and partners get warm leads, not cold lists", "Conversion by lead source today (case data)")
cd = CategoryChartData()
cd.categories = ["Referral", "Inbound call", "Channel partner", "Field sales", "Website form", "WhatsApp"]
cd.add_series("Conversion %", (14.8, 9.5, 8.0, 7.3, 4.3, 2.8))
gf = s.shapes.add_chart(XL_CHART_TYPE.COLUMN_CLUSTERED, Inches(0.5), Inches(1.6), Inches(4.7), Inches(2.9), cd)
style_chart(gf.chart, [RED, GREY, GREY, GREY, DARK_RED, DARK_RED])
gf.chart.plots[0].data_labels.number_format = '0.0"%"'
gf.chart.plots[0].data_labels.number_format_is_linked = False
text(s, 0.6, 4.5, 4.6, 0.5, "Digital channels convert worst today because there is no journey behind them. Udyam Digital is that journey.", size=10, color=GREY)
roles = [("Digital does", "discovery, quote and KYC for every lead, 24×7, in 10 languages"),
         ("Field sales does", "multi-site and manufacturers, where a ₹24,335 CAC pays back"),
         ("Partners get", "leads the advisor can't close, with a summary, ranked by score"),
         ("2.4×", "more activations in the top 10% of scored leads than random")]
for i, (h, d) in enumerate(roles):
    y = 1.6 + i * 0.84
    box(s, 5.5, y, 3.95, 0.74, fill=TINT if i == 3 else PANEL)
    text(s, 5.65, y + 0.06, 1.35, 0.62, h, size=12 if i < 3 else 20, bold=True, color=INK if i < 3 else RED, anchor=MSO_ANCHOR.MIDDLE)
    text(s, 7.0, y + 0.06, 2.35, 0.62, d, size=10, color=INK, anchor=MSO_ANCHOR.MIDDLE)
s.notes_slide.notes_text_frame.text = (
    "The digital journey adds to the field force and partners instead of competing with them. Referrals convert best; website and "
    "WhatsApp leads convert worst because nothing happens after the click. Now the journey handles discovery, quote and KYC for every lead. "
    "Field sales focuses on high-value multi-site and manufacturing accounts. When the advisor can't close, the lead goes to a partner "
    "with a summary, ranked by a propensity score that finds 2.4 times more buyers in its top decile than random calling.")

# ---- 8. Architecture ------------------------------------------------------------------------------------
s = new_content_slide()
title(s, "Built like a production system, sized for a national rollout", "Demo topology (docker compose) · full diagram: docs/architecture.md")
picture(s, DOCS / "architecture-demo.png", 0.55, 1.58, w=5.9)
arch = [("Decisions are code", "eligibility, pricing and KYC verdicts are deterministic Go; the LLM only talks and explains"),
        ("Nothing is lost", "transactional outbox + idempotent consumers + dead-letter topic; Kafka or a service down: no event lost (drills pass)"),
        ("Private by design", "one schema + DB role per service; masked IDs; chat transcripts back-office only; owner OTP + partner ReBAC next"),
        ("National scale", "regional gateways, Kafka partitioned by lead, multi-region DB with data residency")]
for i, (h, d) in enumerate(arch):
    y = 1.6 + i * 0.86
    badge(s, 6.7, y + 0.02, str(i + 1), d=0.32, size=10)
    text(s, 7.12, y, 2.35, 0.28, h, size=11, bold=True)
    text(s, 7.12, y + 0.27, 2.35, 0.6, d, size=9, color=GREY)
s.notes_slide.notes_text_frame.text = (
    "Under the hood it's built like a production system. Web and mobile talk to one gateway; eight services own their own data; events "
    "flow over Kafka with a transactional outbox, so in our failure drills, stopping Kafka, a service or the database lost nothing. "
    "Eligibility, pricing and KYC are deterministic code: the language model never decides what is sold. Each service has its own database role, "
    "chat transcripts are back-office only, and owner OTP login with partner access rules is next. The same design scales "
    "nationally with regional gateways, partitioned Kafka and a multi-region database.")

# ---- 9. Rollout + KPIs ----------------------------------------------------------------------------------
s = new_content_slide()
title(s, "Three-wave rollout with targets we can measure every week", "Targets are proposals to agree with Accel Business sales leadership")
phases = [("Weeks 0–8", "Pilot", "Professional offices · 8 Tier 1 cities · English, Hindi, Tamil"),
          ("Weeks 9–20", "Wave 2", "Multi-site + manufacturers with field close · all 10 languages · partner handoff"),
          ("Week 21+", "Scale", "Micro retail self-serve · Tier 2 cities · WhatsApp entry point")]
for i, (when, name, d) in enumerate(phases):
    x = 0.55 + i * 3.0
    box(s, x, 1.6, 2.8, 1.25, fill=TINT if i == 0 else PANEL)
    badge(s, x + 0.15, 1.72, str(i + 1), d=0.36, size=11)
    text(s, x + 0.6, 1.7, 2.1, 0.25, when, size=10, color=GREY)
    text(s, x + 0.6, 1.93, 2.1, 0.3, name, size=13, bold=True)
    text(s, x + 0.15, 2.28, 2.55, 0.55, d, size=9.5, color=INK)
kpis = [("Lead → activation", "11.1%", "15%"), ("Fixable drops (share)", "32%", "< 15%"), ("Quote → KYC complete", "68.3%", "85%"),
        ("Lead to activation time", "17.6 days", "< 10 days"), ("CAC, prof. offices", "₹10,695", "< ₹7,500")]
text(s, 0.55, 3.0, 4, 0.3, "KPIs: baseline → pilot target", size=12, bold=True)
for i, (k, base, target) in enumerate(kpis):
    x = 0.55 + i * 1.8
    box(s, x, 3.35, 1.68, 1.6, fill=PANEL)
    text(s, x + 0.12, 3.43, 1.48, 0.42, k, size=9.5, bold=True, color=GREY)
    text(s, x + 0.12, 3.9, 1.48, 0.3, base, size=12, color=MUTED)
    text(s, x + 0.12, 4.22, 1.48, 0.5, target, size=20, bold=True, color=RED)
    text(s, x + 0.12, 4.65, 1.48, 0.25, "target", size=9, color=MUTED)
s.notes_slide.notes_text_frame.text = (
    "We roll out in three waves. An eight-week pilot with professional offices in the eight Tier 1 cities and three languages, then multi-site "
    "and manufacturers with field sales closing and all ten languages, then micro retail self-serve and Tier 2 cities. We'd measure five "
    "things weekly: lead-to-activation from 11 to 15 percent, the share of drops a digital journey can fix from 32 to under 15 percent, "
    "quote-to-KYC completion to 85 percent, activation time under ten days, and acquisition cost under 7,500 rupees.")

# ---- 10. Risks & asks -----------------------------------------------------------------------------------
s = new_content_slide()
title(s, "Risks we've designed for, and what we need to launch", "Udyam Digital: from 7.4% to a journey owners finish on their phone")
risks = [("AI wrong or tricked", "prices are code; reply guard blocked 6/6 prompt-injection attacks; eval ≥ 90%"),
         ("Language quality", "per-language evals; native-speaker review of UI and messages"),
         ("DPDP compliance", "consent per purpose, deletion, masked IDs; DPO sign-off on retention"),
         ("Score bias", "score only orders call lists; fairness review by language and city tier"),
         ("Channel conflict", "partners get ranked warm leads and credit for handoffs"),
         ("Fibre not feasible", "automatic 5G AirFiber offer instead of a dropped order")]
for i, (h, d) in enumerate(risks):
    col, row = i % 2, i // 2
    x, y = 0.55 + col * 2.95, 1.6 + row * 1.12
    box(s, x, y, 2.8, 1.0, fill=PANEL)
    text(s, x + 0.15, y + 0.08, 2.5, 0.28, h, size=11, bold=True, color=RED)
    text(s, x + 0.15, y + 0.38, 2.5, 0.6, d, size=9.5, color=INK)
box(s, 6.55, 1.6, 2.9, 3.36, fill=INK)
text(s, 6.72, 1.7, 2.6, 0.3, "Asks", size=14, bold=True, color=WHITE)
text(s, 6.72, 2.05, 2.6, 2.85, [
    [("Sales leadership: ", {"bold": True}), ("approve the professional-offices pilot and targets", {})],
    [("Field & partners: ", {"bold": True}), ("handoff workflow and partner credit", {})],
    [("Compliance / DPO: ", {"bold": True}), ("consent text and retention periods", {})],
    [("KYC & risk: ", {"bold": True}), ("review-queue SLAs and routing policy", {})],
    [("Network ops: ", {"bold": True}), ("fibre feasibility data, install dates", {})],
    [("Billing & support: ", {"bold": True}), ("one bill, tracker for 'where is my order'", {})]],
    size=9.5, color=WHITE, spacing=4)
s.notes_slide.notes_text_frame.text = (
    "We've designed for the main risks: the AI can't make up products or prices because those are code, and it can't be talked into one either: "
    "a reply guard blocked all six prompt-injection attacks we tried. Every language is evaluated, "
    "privacy follows the DPDP Act, the score only orders call lists, partners are credited for handoffs, and an infeasible fibre site gets "
    "a 5G offer instead of a lost order. To launch, we need sales leadership to approve the pilot, compliance to sign off consent and "
    "retention, and network and billing to plug in feasibility and one-bill invoicing. Thank you.")

prs.save(OUT)
print("saved", OUT, len(prs.slides), "slides")
