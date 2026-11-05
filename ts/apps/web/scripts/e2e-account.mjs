// Returning-customer path in a real browser (installed Edge/Chrome via playwright-core), phone size.
// No LLM needed: orders are created through the quote API, the screens are driven in the browser.
//   1. first order: accepted quote + consent + full KYC (specimen K03)
//   2. sign in on /account with mobile + demo code -> the order is listed -> Track opens it
//   3. "Start a new order" -> its quote is accepted -> KYC screen offers the verified documents ->
//      "Nothing has changed" -> verified without uploading -> the account lists two orders
//
//   node scripts/e2e-account.mjs     (needs make up seed and the web app on WEB_URL)
import { chromium } from "playwright-core";
import { resolve } from "node:path";

const WEB = process.env.WEB_URL ?? "http://localhost:3000";
const API = process.env.API_URL ?? "http://localhost:8080/api";
const OUT = resolve(import.meta.dirname, "../../../../docs/screens");
const log = (...a) => console.log(new Date().toISOString().slice(11, 19), ...a);

async function call(method, path, body) {
  const r = await fetch(API + path, { method, headers: { "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : undefined });
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
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error("no order for " + leadId);
}

const journey = (leadId) => JSON.stringify({ lang: "en", leadId, messages: [], profile, consented: true });
/** Put the order in progress in the browser, once the page has stopped saving its own (it may still be hydrating). */
async function setJourney(page, leadId) {
  await page.waitForLoadState("networkidle");
  for (let i = 0; i < 10; i++) {
    await page.evaluate((j) => localStorage.setItem("udyam.journey.v1", j), journey(leadId));
    await page.waitForTimeout(300);
    if (await page.evaluate(() => JSON.parse(localStorage.getItem("udyam.journey.v1")).leadId) === leadId) return;
  }
  throw new Error("the page kept replacing the journey");
}
const run = Date.now().toString(36);
const lead1 = `e2e-acct-${run}-1`;
const mobile = "9" + String(Date.now()).slice(-9);

const browser = await chromium.launch({ channel: process.env.E2E_BROWSER ?? "msedge" });
const page = await (await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 })).newPage();
page.on("pageerror", (e) => { throw e; });
try {
  // 1. first order with full KYC, as a guest
  await order(lead1);
  await call("POST", `/kyc/applications/${lead1}/specimens/K03`);
  const first = await call("POST", `/kyc/applications/${lead1}/check`);
  if (first.status !== "pass") throw new Error("K03 should pass, got " + first.status);
  log("first order verified", lead1);

  // 2. sign in; the order in progress joins the account
  await page.goto(WEB);
  await setJourney(page, lead1);
  await page.goto(WEB + "/account");
  await page.getByTestId("mobile").fill(mobile);
  await page.getByTestId("send-code").click();
  const code = (await page.getByTestId("demo-code").textContent()).trim();
  await page.getByTestId("otp").fill(code);
  await page.getByTestId("sign-in").click();
  await page.getByTestId("signed-in").waitFor();
  await page.getByTestId("order-row").first().waitFor({ timeout: 15_000 });
  await page.getByTestId("verified-business").waitFor();
  await page.screenshot({ path: `${OUT}/phone-08-account.png`, fullPage: true });
  log("signed in; order and verified business listed");

  await page.getByTestId("track-order").first().click();
  await page.waitForURL("**/track?lead=*");
  await page.getByTestId("order-status").waitFor({ timeout: 15_000 });
  log("track opens the past order:", await page.getByTestId("order-status").getAttribute("data-status"));

  // 3. new order: fresh lead, linked to the account by the journey
  await page.goto(WEB + "/account");
  await page.getByTestId("new-order").click();
  await page.waitForURL("**/chat");
  const lead2 = await page.evaluate(() => JSON.parse(localStorage.getItem("udyam.journey.v1")).leadId);
  if (lead2 === lead1) throw new Error("new order kept the old lead");
  await page.waitForTimeout(800); // the journey links the lead to the account
  await order(lead2); // stands in for chat + quote accept + consent
  await setJourney(page, lead2);
  await page.goto(WEB + "/kyc");
  await page.getByTestId("reuse-card").waitFor({ timeout: 15_000 });
  await page.screenshot({ path: `${OUT}/phone-09-kyc-reuse.png`, fullPage: true });
  if (!(await page.getByTestId("reuse-kyc").isDisabled())) throw new Error("reuse must wait for the 'nothing changed' tick");
  await page.getByTestId("unchanged").check();
  await page.getByTestId("reuse-kyc").click();
  await page.locator('[data-testid="kyc-result"][data-status="pass"]').waitFor({ timeout: 15_000 });
  log("second order verified from the account, no uploads");

  for (let i = 0; i < 30; i++) {
    const { orders } = await call("GET", `/tracker/orders?leadId=${encodeURIComponent(lead2)}`);
    if (orders[0] && orders[0].status !== "quote_accepted" && orders[0].status !== "kyc_pending") { log("order 2 moved to", orders[0].status); break; }
    if (i === 29) throw new Error("order 2 did not move past KYC");
    await new Promise((r) => setTimeout(r, 1000));
  }
  await page.goto(WEB + "/account");
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="order-row"]').length >= 2, null, { timeout: 15_000 });
  log("ACCOUNT E2E OK");
} finally {
  await browser.close();
}
