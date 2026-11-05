// Builds docs/video/Udyam_Digital_Video_Script.docx from content.json.
//   NODE_PATH=<dir with docx installed> node docs/video/build_docx.js
const fs = require("fs");
const path = require("path");
const {
  Document, Packer, Paragraph, TextRun, Table, TableRow, TableCell, WidthType, ShadingType, AlignmentType,
  HeadingLevel, LevelFormat, PageOrientation, BorderStyle, Footer, PageNumber,
} = require("docx");

const C = JSON.parse(fs.readFileSync(path.join(__dirname, "content.json"), "utf8"));
const RED = "E40000", INK = "16161A", GREY = "6B6B76", HEAD = "16161A", BAND = "F6F6F8", MINT = "ECFDF3", AMBER = "FFFBEB", PINK = "FFF3F3";
const FONT = "Arial";
// A4 landscape: 16838 x 11906 twips; 1 cm margins -> content width 16838 - 2*720
const W = 16838 - 2 * 720;

const run = (text, o = {}) => new TextRun({ text, font: FONT, size: o.size ?? 19, bold: o.bold, color: o.color ?? INK, italics: o.italics });
const para = (children, o = {}) => new Paragraph({ children: Array.isArray(children) ? children : [run(children, o)], spacing: { before: 0, after: o.after ?? 60 }, alignment: o.align, ...(o.numbering ? { numbering: o.numbering } : {}), keepNext: o.keepNext });
const h1 = (t) => new Paragraph({ heading: HeadingLevel.HEADING_1, children: [new TextRun({ text: t, font: FONT })] });
const h2 = (t) => new Paragraph({ heading: HeadingLevel.HEADING_2, children: [new TextRun({ text: t, font: FONT })] });
const bullet = (t) => para([run(t)], { numbering: { reference: "bullets", level: 0 } });
const numbered = (t) => para([run(t)], { numbering: { reference: "steps", level: 0 } });

const border = { style: BorderStyle.SINGLE, size: 4, color: "DCDCE3" };
const borders = { top: border, bottom: border, left: border, right: border };
function table(widthsPct, header, rows, shade = () => null) {
  const widths = widthsPct.map((p) => Math.floor((W * p) / 100));
  widths[widths.length - 1] += W - widths.reduce((a, b) => a + b, 0);
  const cell = (content, i, o = {}) => new TableCell({
    width: { size: widths[i], type: WidthType.DXA }, borders,
    shading: o.fill ? { type: ShadingType.CLEAR, color: "auto", fill: o.fill } : undefined,
    margins: { top: 70, bottom: 70, left: 110, right: 110 },
    children: (Array.isArray(content) ? content : [content]).map((c) => (c instanceof Paragraph ? c : para([run(String(c), { bold: o.bold, color: o.color, size: o.size })], { after: 20 }))),
  });
  return new Table({
    width: { size: W, type: WidthType.DXA }, columnWidths: widths,
    rows: [
      new TableRow({ tableHeader: true, children: header.map((h, i) => cell(h, i, { fill: HEAD, bold: true, color: "FFFFFF" })) }),
      ...rows.map((r, ri) => new TableRow({ cantSplit: true, children: r.map((c, i) => cell(c, i, { fill: shade(r, i, ri) ?? (ri % 2 ? BAND : undefined) })) })),
    ],
  });
}
const routeFill = (route) => ({ Pass: MINT, "Re-upload": AMBER, "Manual review": AMBER, Reject: PINK })[route];
const gap = () => para("", { after: 80 });

const body = [];
body.push(new Paragraph({ heading: HeadingLevel.TITLE, children: [new TextRun({ text: C.title, font: FONT })] }));
body.push(para([run(C.subtitle, { color: GREY })], { after: 120 }));
body.push(para([run("How to use this: ", { bold: true }), run("check the test cases in section 3 with the live AI advisor first. When they match, record the video using the script in section 2. Every price in this document came from the running system: the quote service is deterministic, so the same facts always give the same price.")], { after: 160 }));

body.push(h1("1. Before you record"));
C.setup.forEach((s) => body.push(numbered(s)));

body.push(h1("2. Video script (3 minutes)"));
body.push(para([run("About 430 words of voice-over at a calm pace. Record at 1080p, with the phone (web or Expo app) next to the desktop back office.", { color: GREY })]));
body.push(table([9, 33, 58], ["Time", "On screen", "Voice-over"], C.script.map(([t, s, v]) => [t, s, v])));

body.push(h1("3. AI advisor test cases"));
body.push(para([run("Start each case from the home screen (demo owner button or Start). Monthly prices are for a 12-month contract unless stated. \"Check\" is what to look for in the advisor's reply.", { color: GREY })]));
body.push(table([4, 17, 25, 24, 9, 21],
  ["#", "Owner (how to start)", "What to type", "Expected products (monthly)", "Monthly / one-time", "Check"],
  C.advisor.map((a) => [
    a.id,
    [para([run(a.name, { bold: true })], { after: 20 }), para([run(a.start, { color: GREY, size: 17 })], { after: 0 })],
    a.say,
    a.lines.map((l) => para([run(l, { size: 18 })], { after: 10 })),
    [para([run(a.monthly, { bold: true })], { after: 20 }), para([run(a.oneTime + " one-time", { size: 17 })], { after: 20 }), para([run(a.contract, { size: 16, color: GREY })], { after: 0 })],
    a.check,
  ]),
  (r, i) => (i === 5 && String(r[5]).startsWith("KNOWN ISSUE") ? PINK : null)));

body.push(h1("4. KYC: the 12 test applicants and their routes"));
body.push(para([run("Demo owners load their own sample set with \"Use sample documents\". For the others, upload the four files from data/kyc_documents/Kxx_*.png (GST certificate, Udyam certificate, address proof, signatory ID).", { color: GREY })]));
body.push(table([7, 13, 50, 30], ["Set", "Route", "Why", "How to get it"], C.kyc, (r, i) => (i === 1 ? routeFill(r[1]) : null)));
body.push(gap());
body.push(h2("What happens after each route"));
body.push(table([16, 84], ["Route", "Next"], C.kycRoutes, (r, i) => (i === 0 ? routeFill(r[0]) : null)));

body.push(h1("5. Returning customer: KYC reuse"));
body.push(para([run("Sign in with 👤 Account (any 10-digit mobile; the code is shown on screen). The rules are deterministic Go code (go/pkg/kyc/reuse.go); the AI is not involved.", { color: GREY })]));
body.push(table([36, 44, 20], ["Situation", "Expected", "Testable in the app?"], C.reuse));

body.push(h1("6. Order routes after KYC"));
body.push(table([20, 80], ["Route", "What you see"], C.fulfilment));

body.push(h1("7. Product catalogue (25 products)"));
body.push(para([run("Source: data/product_catalogue.csv. A product is offered only when every rule holds; otherwise the advisor gives the reason and the closest alternative.", { color: GREY })]));
const cat = fs.readFileSync(path.join(__dirname, "../../data/product_catalogue.csv"), "utf8").trim().split(/\r?\n/).slice(1).map((line) => {
  const f = []; let cur = "", q = false;
  for (const ch of line) { if (ch === '"') q = !q; else if (ch === "," && !q) { f.push(cur); cur = ""; } else cur += ch; }
  f.push(cur);
  const [id, , name, , unit, min, max, setup, months, tiers, minEmp, minSites, gst, needs, days] = f;
  const rules = [tiers === "1" ? "Tier 1 only" : null, +minEmp > 1 ? `${minEmp}+ staff` : null, +minSites > 1 ? `${minSites}+ sites` : null, gst === "Y" ? "GSTIN" : null, needs ? "needs internet" : null].filter(Boolean).join(", ") || "none";
  return [id, name, unit, `₹${(+min).toLocaleString("en-IN")}–${(+max).toLocaleString("en-IN")}`, +setup ? `₹${(+setup).toLocaleString("en-IN")}` : "–", `${months} mo`, rules, `${days} d`];
});
body.push(table([5, 24, 11, 12, 9, 8, 22, 9], ["ID", "Product", "Unit", "Monthly", "One-time", "Min term", "Needs", "Install"], cat));

body.push(h1("8. Screens and routes"));
body.push(table([16, 84], ["Route", "What it is"], C.screens));

body.push(h1("9. Known issues: keep off camera"));
C.issues.forEach((s) => body.push(bullet(s)));

const doc = new Document({
  creator: "Udyam Digital team", title: C.title,
  styles: {
    default: { document: { run: { font: FONT, size: 19 } } },
    paragraphStyles: [
      { id: "Title", name: "Title", basedOn: "Normal", run: { font: FONT, size: 40, bold: true, color: INK }, paragraph: { spacing: { before: 0, after: 60 }, keepNext: true } },
      { id: "Heading1", name: "Heading 1", basedOn: "Normal", next: "Normal", quickFormat: true, run: { font: FONT, size: 28, bold: true, color: RED }, paragraph: { spacing: { before: 280, after: 100 }, keepNext: true, outlineLevel: 0 } },
      { id: "Heading2", name: "Heading 2", basedOn: "Normal", next: "Normal", quickFormat: true, run: { font: FONT, size: 23, bold: true, color: INK }, paragraph: { spacing: { before: 160, after: 80 }, keepNext: true, outlineLevel: 1 } },
    ],
  },
  numbering: { config: [
    { reference: "bullets", levels: [{ level: 0, format: LevelFormat.BULLET, text: "•", alignment: AlignmentType.LEFT, style: { paragraph: { indent: { left: 400, hanging: 260 } } } }] },
    { reference: "steps", levels: [{ level: 0, format: LevelFormat.DECIMAL, text: "%1.", alignment: AlignmentType.LEFT, style: { paragraph: { indent: { left: 400, hanging: 300 } } } }] },
  ] },
  sections: [{
    properties: { page: { size: { width: 11906, height: 16838, orientation: PageOrientation.LANDSCAPE }, margin: { top: 720, bottom: 720, left: 720, right: 720 } } },
    footers: { default: new Footer({ children: [new Paragraph({ alignment: AlignmentType.RIGHT, children: [run("Udyam Digital · video script and test guide · page ", { size: 16, color: GREY }), new TextRun({ children: [PageNumber.CURRENT], font: FONT, size: 16, color: GREY })] })] }) },
    children: body,
  }],
});

const out = process.env.OUT || path.join(__dirname, "Udyam_Digital_Video_Script.docx");
Packer.toBuffer(doc).then((b) => { fs.writeFileSync(out, b); console.log("wrote", out); });
