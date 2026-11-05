"""Builds docs/video/Udyam_Digital_Video_Script.pptx (script + test guide) on the submission deck's template.

Reuses the helpers from docs/deck/build.py (everything above its first slide) and the same content.json
as build_docx.js. The submission deck itself stays at 10 slides. Run: python docs/video/build_pptx.py
"""
import csv
import json
from pathlib import Path

from pptx.util import Inches, Pt

HERE = Path(__file__).parent
DECK = HERE.parent / "deck"
src = (DECK / "build.py").read_text(encoding="utf-8")
helpers = src[: src.index("# ---- 1. Title")]
g = {"__file__": str(DECK / "build.py"), "__name__": "deck_helpers"}
exec(compile(helpers, str(DECK / "build.py"), "exec"), g)
prs, text, box, badge, title, new_content_slide = g["prs"], g["text"], g["box"], g["badge"], g["title"], g["new_content_slide"]
RED, INK, GREY, WHITE, PANEL, TINT, GREEN, FONT = (g[k] for k in ("RED", "INK", "GREY", "WHITE", "PANEL", "TINT", "GREEN", "FONT"))
RGB = type(RED)
MINT, AMBER = RGB(0xEC, 0xFD, 0xF3), RGB(0xFF, 0xF4, 0xD6)

C = json.loads((HERE / "content.json").read_text(encoding="utf-8"))
OUT = HERE / "Udyam_Digital_Video_Script.pptx"


def table(slide, x, y, w, widths, header, rows, size=8.5, fills=None, row_h=0.3):
    """widths: fractions of w. rows: lists of str or list-of-lines. fills(r, ci, ri) -> RGB or None."""
    shp = slide.shapes.add_table(len(rows) + 1, len(header), Inches(x), Inches(y), Inches(w), Inches(row_h * (len(rows) + 1)))
    t = shp.table
    t.first_row = True
    t.horz_banding = False
    for i, f in enumerate(widths):
        t.columns[i].width = Inches(w * f)

    def put(cell, value, bold=False, color=INK, fill=None, sz=size):
        cell.margin_left = cell.margin_right = Inches(0.05)
        cell.margin_top = cell.margin_bottom = Inches(0.025)
        cell.fill.solid()
        cell.fill.fore_color.rgb = fill or WHITE
        tf = cell.text_frame
        tf.word_wrap = True
        lines = value if isinstance(value, list) else [value]
        for li, line in enumerate(lines):
            p = tf.paragraphs[0] if li == 0 else tf.add_paragraph()
            r = p.add_run()
            r.text = str(line)
            r.font.name, r.font.size, r.font.bold, r.font.color.rgb = FONT, Pt(sz), bold, color

    for ci, h in enumerate(header):
        put(t.cell(0, ci), h, bold=True, color=WHITE, fill=INK)
    for ri, r in enumerate(rows):
        for ci, v in enumerate(r):
            fill = fills(r, ci, ri) if fills else None
            put(t.cell(ri + 1, ci), v, bold=(ci == 0), fill=fill or (PANEL if ri % 2 else WHITE))
    return t


ROUTE_FILL = {"Pass": MINT, "Re-upload": AMBER, "Manual review": AMBER, "Reject": TINT}

# ---- 1. Title -----------------------------------------------------------------------------------------
s = prs.slides[0]
text(s, 0.62, 2.55, 4.8, 0.6, "Demo video script", size=32, bold=True, color=WHITE, name="Title")
text(s, 0.62, 3.15, 4.4, 0.75, "and test guide: what to show, what to say, and what the AI must answer", size=14, color=WHITE)
text(s, 0.62, 4.1, 4.4, 0.5, [[("Udyam Digital · Airtel SYNC", {"bold": True})], "Checked against the live system, 6 Nov 2026"], size=11, color=RGB(0xF0, 0xC8, 0xC8))

# ---- 2. Before you record ----------------------------------------------------------------------------------
s = prs.slides[1]
for ph in list(s.placeholders):
    ph._element.getparent().remove(ph._element)
title(s, "Before you record", "Everything running, warmed up and seeded")
for i, step in enumerate(C["setup"]):
    y = 1.62 + i * 0.42
    badge(s, 0.6, y, str(i + 1), d=0.3, size=10)
    text(s, 1.02, y + 0.02, 8.4, 0.4, step, size=10.5)

# ---- 3-4. Script ----------------------------------------------------------------------------------------
for part, rows in enumerate((C["script"][:6], C["script"][6:])):
    s = new_content_slide()
    title(s, f"Video script ({part + 1}/2)", "3 minutes, about 430 words of voice-over")
    table(s, 0.55, 1.55, 8.9, [0.1, 0.33, 0.57], ["Time", "On screen", "Voice-over"], [list(r) for r in rows], size=8.5)
    s.notes_slide.notes_text_frame.text = "\n\n".join(f"{t}  {v}" for t, _, v in rows)

# ---- 5-6. Advisor test cases --------------------------------------------------------------------------------
for part, cases in enumerate((C["advisor"][:6], C["advisor"][6:])):
    s = new_content_slide()
    title(s, f"AI advisor test cases ({part + 1}/2)", "Expected products and prices from the live quote service (12-month contract unless stated)")
    rows = [[a["id"], [a["name"], a["start"]], a["lines"], [a["monthly"], a["oneTime"] + " one-time"], a["check"]] for a in cases]
    table(s, 0.55, 1.5, 8.9, [0.05, 0.2, 0.27, 0.15, 0.33], ["#", "Owner", "Expected products", "Monthly", "Check"], rows, size=7.5,
          fills=lambda r, ci, ri: TINT if ci == 4 and str(r[4]).startswith("KNOWN ISSUE") else None)
    s.notes_slide.notes_text_frame.text = "What to type:\n\n" + "\n\n".join(f"{a['id']} {a['name']}: {a['say']}" for a in cases)

# ---- 7. KYC ----------------------------------------------------------------------------------------------
s = new_content_slide()
title(s, "KYC: 12 test applicants and their routes", "Demo owners load their own set; for the others upload data/kyc_documents/Kxx_*.png")
table(s, 0.55, 1.5, 5.6, [0.1, 0.2, 0.7], ["Set", "Route", "Why"], [[k, r, w] for k, r, w, _ in C["kyc"]], size=7.5,
      fills=lambda r, ci, ri: ROUTE_FILL.get(r[1]) if ci == 1 else None, row_h=0.27)
y = 1.5
for route, nxt in C["kycRoutes"]:
    box(s, 6.35, y, 3.1, 0.82, fill=ROUTE_FILL[route])
    text(s, 6.47, y + 0.06, 2.9, 0.25, route, size=10, bold=True)
    text(s, 6.47, y + 0.3, 2.9, 0.5, nxt, size=7.5, color=GREY)
    y += 0.9
s.notes_slide.notes_text_frame.text = "\n".join(f"{k} ({r}): {how}" for k, r, _, how in C["kyc"])

# ---- 8. Reuse + order routes -------------------------------------------------------------------------------
s = new_content_slide()
title(s, "Returning customers and order routes", "KYC reuse rules are deterministic Go code (go/pkg/kyc/reuse.go)")
table(s, 0.55, 1.5, 5.3, [0.36, 0.44, 0.2], ["Situation", "Expected", "In the app?"], C["reuse"], size=7.5)
table(s, 6.05, 1.5, 3.4, [0.3, 0.7], ["Route", "What you see"], C["fulfilment"], size=7.5)

# ---- 9. Catalogue ------------------------------------------------------------------------------------------
s = new_content_slide()
title(s, "Product catalogue (25 products)", "Offered only when every rule holds; otherwise the reason and the closest alternative")
cat = []
with open(HERE.parent.parent / "data" / "product_catalogue.csv", encoding="utf-8") as f:
    for p in csv.DictReader(f):
        rules = [x for x in (
            "Tier 1" if p["available_city_tiers"] == "1" else None,
            f"{p['min_employees']}+ staff" if int(p["min_employees"]) > 1 else None,
            f"{p['min_sites']}+ sites" if int(p["min_sites"]) > 1 else None,
            "GSTIN" if p["requires_gstin"] == "Y" else None,
            "internet" if p["requires_one_of"] else None) if x]
        cat.append([p["product_id"], p["product_name"], f"₹{int(p['price_min_inr_per_month']):,}–{int(p['price_max_inr_per_month']):,}", ", ".join(rules) or "–"])
for col, rows in enumerate((cat[:13], cat[13:])):
    table(s, 0.55 + col * 4.5, 1.5, 4.4, [0.12, 0.42, 0.22, 0.24], ["ID", "Product", "Monthly", "Needs"], rows, size=7, row_h=0.24)

# ---- 10. Known issues -------------------------------------------------------------------------------------------
s = new_content_slide()
title(s, "Keep off camera", "Known issues found while checking the AI")
for i, issue in enumerate(C["issues"]):
    y = 1.65 + i * 0.75
    box(s, 0.55, y, 8.9, 0.62, fill=TINT if i == 0 else PANEL)
    badge(s, 0.7, y + 0.13, "!" if i == 0 else str(i + 1), d=0.36, size=11)
    text(s, 1.2, y + 0.1, 8.1, 0.5, issue, size=10.5)

prs.save(OUT)
print("saved", OUT, len(prs.slides), "slides")
