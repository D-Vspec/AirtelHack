// Renders the Mermaid diagrams in docs/architecture.md to PNG with a headless Edge/Chrome
// (playwright-core). Mermaid itself is not a dependency of the app; point MERMAID_JS at
// mermaid.min.js, e.g.:   npm pack mermaid && tar -xzf mermaid-*.tgz
//                          MERMAID_JS=package/dist/mermaid.min.js node scripts/render-diagrams.mjs
import { chromium } from "playwright-core";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const docs = resolve(import.meta.dirname, "../../../../docs");
const md = readFileSync(`${docs}/architecture.md`, "utf8");
const blocks = [...md.matchAll(/```mermaid\n([\s\S]*?)```/g)].map((m) => m[1]);
const names = ["architecture-demo", "architecture-national"];
if (!process.env.MERMAID_JS) throw new Error("set MERMAID_JS to mermaid.min.js (see header)");
const mermaidJs = readFileSync(process.env.MERMAID_JS, "utf8");

const browser = await chromium.launch({ channel: process.env.E2E_BROWSER ?? "msedge" });
try {
  const page = await browser.newPage({ viewport: { width: 1800, height: 1200 }, deviceScaleFactor: 2 });
  for (const [i, src] of blocks.entries()) {
    await page.setContent(`<!doctype html><html><body style="margin:0;background:#fff;font-family:Segoe UI,Arial">
      <div id="d" style="display:inline-block;padding:24px"></div></body></html>`);
    await page.addScriptTag({ content: mermaidJs });
    await page.evaluate(async (code) => {
      // eslint-disable-next-line no-undef
      mermaid.initialize({ startOnLoad: false, theme: "default", flowchart: { htmlLabels: true, curve: "basis", useMaxWidth: false }, themeVariables: { fontSize: "18px" }, securityLevel: "strict" });
      // eslint-disable-next-line no-undef
      const { svg } = await mermaid.render("g", code);
      document.getElementById("d").innerHTML = svg;
    }, src);
    await page.locator("#d").screenshot({ path: `${docs}/${names[i]}.png` });
    console.log("wrote", `docs/${names[i]}.png`);
  }
} finally {
  await browser.close();
}
