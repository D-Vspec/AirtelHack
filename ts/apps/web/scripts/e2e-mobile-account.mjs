// Returning customer in the Expo app, run on its web build (react-native-web) at phone size. No LLM needed:
// orders are made through the quote API, the account and KYC screens are driven in the browser.
//   1. first order: accepted quote + consent + full KYC (specimen K03), joined to an account
//   2. sign in from the header (mobile + demo code) -> order and verified business listed -> Track opens it
//   3. "Start a new order" -> its quote is accepted -> "Finish verification" -> the KYC screen offers the
//      verified documents -> "Nothing has changed" -> verified without uploading
//
//   make mobile-account-e2e   (exports the Expo web build; needs make up seed)
import { chromium } from "playwright-core";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, resolve } from "node:path";

const API = process.env.API_URL ?? "http://localhost:8080/api";
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
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function call(method, path, body, token) {
  const headers = { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) };
  const r = await fetch(API + path, { method, headers, body: body ? JSON.stringify(body) : undefined });
  const j = await r.json().catch(() => null);
  if (!r.ok) throw new Error(`${method} ${path}: ${r.status} ${JSON.stringify(j)}`);
  return j;
}

const profile = { segment: "Professional offices", cityTier: 1, employeeCount: 6, numSites: 1, hasGstin: true };
/** An accepted quote (the tracker opens an order for it) and consent, as the chat + consent screens would. */
async function order(leadId) {
  const q = await call("POST", "/quote/quotes", { leadId, profile, items: [{ productId: "P02" }, { productId: "P16" }], contractMonths: 12 });
  await call("POST", `/quote/quotes/${q.id}/accept`);
  await call("POST", "/kyc/consents", { leadId, purposes: ["kyc_verification", "service_provisioning", "communication"], version: "2026-07-v1", language: "en" });
  for (let i = 0; i < 40; i++) {
    const { orders } = await call("GET", `/tracker/orders?leadId=${encodeURIComponent(leadId)}`);
    if (orders.length) return orders[0];
    await sleep(500);
  }
  throw new Error("no order for " + leadId);
}

const run = Date.now().toString(36);
const lead1 = `e2e-macct-${run}-1`;
const mobile = "8" + String(Date.now()).slice(-9);

const browser = await chromium.launch({ channel: process.env.E2E_BROWSER ?? "msedge" });
try {
  // 1. first order with full KYC, joined to the account (an earlier sign-in on another phone)
  await order(lead1);
  await call("POST", `/kyc/applications/${lead1}/specimens/K03`);
  const first = await call("POST", `/kyc/applications/${lead1}/check`);
  if (first.status !== "pass") throw new Error("K03 should pass, got " + first.status);
  const otp = await call("POST", "/kyc/accounts/otp", { mobile });
  const { token } = await call("POST", "/kyc/accounts/verify", { mobile, code: otp.demoCode, leadId: lead1 });
  log("first order verified and on the account", lead1);

  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => log("PAGEERROR", e.message));
  const shot = async (name) => { await page.waitForTimeout(400); await page.screenshot({ path: `${OUT}/mobile-${name}.png` }); log("screenshot", name); };

  // 2. sign in from the header
  await page.goto(URL);
  await page.locator(id("account-link")).click();
  await page.locator(id("mobile")).fill(mobile);
  await page.locator(id("send-code")).click();
  const code = (await page.locator(id("demo-code")).textContent()).trim();
  await page.locator(id("otp")).fill(code);
  await page.locator(id("sign-in")).click();
  await page.locator(id("signed-in")).waitFor();
  await page.locator(id("order-row")).first().waitFor({ timeout: 15_000 });
  await page.locator(id("verified-business")).waitFor();
  await shot("06-account");
  log("signed in; order and verified business listed");

  await page.locator(id("track-order")).first().click();
  await page.locator(id("order-status")).waitFor({ timeout: 15_000 });
  log("track opens the past order:", await page.locator(id("order-status")).innerText());

  // 3. new order: fresh lead, joined to the account by the app
  await page.locator(id("account-link")).click();
  await page.locator(id("new-order")).click();
  await page.locator(id("msg")).waitFor();
  let lead2;
  for (let i = 0; i < 20 && !lead2; i++) {
    lead2 = (await call("GET", "/kyc/accounts/me", null, token)).leads.map((l) => l.leadId).find((l) => l !== lead1);
    if (!lead2) await sleep(500);
  }
  if (!lead2) throw new Error("the new order did not join the account");
  await order(lead2); // stands in for chat + quote accept + consent
  log("second order", lead2);

  await page.locator(id("account-link")).click();
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="order-row"]').length >= 2, null, { timeout: 15_000 });
  await page.locator(id("finish-kyc")).click();
  await page.locator(id("reuse-card")).waitFor({ timeout: 15_000 });
  await shot("07-kyc-reuse");
  if ((await page.locator(id("reuse-kyc")).getAttribute("aria-disabled")) !== "true") throw new Error("reuse must wait for the 'nothing changed' tick");
  await page.locator(id("unchanged")).click();
  await page.locator(id("reuse-kyc")).click();
  await page.locator(id("kyc-result")).waitFor({ timeout: 15_000 });
  const status = await page.locator(id("kyc-result")).textContent();
  if (!status.includes("pass")) throw new Error("reuse should pass, got " + status);
  await shot("08-kyc-reused");
  log("second order verified from the account, no uploads");

  for (let i = 0; i < 30; i++) {
    const { orders } = await call("GET", `/tracker/orders?leadId=${encodeURIComponent(lead2)}`);
    if (orders[0] && orders[0].status !== "quote_accepted" && orders[0].status !== "kyc_pending") { log("order 2 moved to", orders[0].status); break; }
    if (i === 29) throw new Error("order 2 did not move past KYC");
    await sleep(1000);
  }
  log("MOBILE ACCOUNT E2E OK");
} finally {
  await browser.close();
  server?.close();
}
