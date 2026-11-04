// The Expo app's journey, run on its web build (react-native-web) in a phone-sized browser: the same
// screens and shared API client as the native app; only the camera is replaced by "use sample".
//
//   make mobile-e2e   (exports the Expo web build to ts/apps/mobile/dist-web; this script serves it)
//   or MOBILE_URL=http://... node scripts/e2e-mobile.mjs   against an already running build
import { chromium } from "playwright-core";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, resolve } from "node:path";

// With MOBILE_DIR, serve the exported build on a free port (single-page app: unknown paths get index.html).
let server;
let URL = process.env.MOBILE_URL ?? "http://localhost:19006";
if (process.env.MOBILE_DIR) {
  const root = resolve(process.env.MOBILE_DIR);
  const types = { ".html": "text/html", ".js": "text/javascript", ".json": "application/json", ".ico": "image/x-icon", ".png": "image/png" };
  server = createServer(async (req, res) => {
    const path = decodeURIComponent(new globalThis.URL(req.url, "http://x").pathname);
    const file = join(root, path === "/" ? "index.html" : path);
    const body = await readFile(file.startsWith(root) ? file : join(root, "index.html")).catch(() => readFile(join(root, "index.html")));
    res.writeHead(200, { "Content-Type": types[extname(file)] ?? "text/html" }).end(body);
  });
  await new Promise((ok) => server.listen(0, "127.0.0.1", ok));
  URL = `http://127.0.0.1:${server.address().port}`;
}
const OUT = resolve(import.meta.dirname, "../../../../docs/screens");
const log = (...a) => console.log(new Date().toISOString().slice(11, 19), ...a);
const id = (x) => `[data-testid="${x}"]`;

const browser = await chromium.launch({ channel: process.env.E2E_BROWSER ?? "msedge" });
try {
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => log("PAGEERROR", e.message));
  const shot = async (name) => { await page.waitForTimeout(400); await page.screenshot({ path: `${OUT}/mobile-${name}.png` }); log("screenshot", name); };

  await page.goto(URL);
  await page.locator(id("demo-restaurant-chain")).waitFor();
  await shot("01-home");
  await page.locator(id("demo-restaurant-chain")).click();
  await page.locator(id("send")).click();
  const followUps = ["5 outlets, 40 staff, GSTIN yes", "Go ahead"];
  for (let i = 0; ; i++) {
    if (await page.locator(id("see-quote")).waitFor({ timeout: 120_000 }).then(() => true, () => false)) break;
    if (i >= followUps.length) throw new Error("advisor did not quote");
    await page.locator(id("msg")).fill(followUps[i]);
    await page.locator(id("send")).click();
  }
  await shot("02-chat");
  await page.locator(id("see-quote")).click();
  await page.locator(id("accept")).waitFor();
  await shot("03-quote");
  await page.locator(id("accept")).click();
  await page.locator(id("agree")).click();
  await page.locator(id("use-sample")).click();
  await page.waitForFunction(() => document.querySelector('[data-testid="check-docs"]')?.getAttribute("aria-disabled") !== "true");
  await page.locator(id("check-docs")).click();
  await page.locator(id("kyc-result")).waitFor({ timeout: 120_000 });
  const kyc = await page.locator(id("kyc-result")).innerText();
  log("KYC", kyc.replace(/\s+/g, " "));
  await shot("04-kyc");
  await page.locator(id("go-track")).click();
  await page.locator(id("order-activated")).waitFor({ state: "attached", timeout: 150_000 });
  log("order activated");
  await page.waitForTimeout(3500);
  await shot("05-track");
  log("MOBILE E2E OK");
} finally {
  await browser.close();
  server?.close();
}
