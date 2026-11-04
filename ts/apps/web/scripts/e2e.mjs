// End-to-end demo path in a real browser (the installed Microsoft Edge or Chrome via playwright-core;
// no browser download). Needs the stack (make up seed), an LLM key for the advisor, and the web
// app on WEB_URL. Saves screenshots to docs/screens.
//
//   node scripts/e2e.mjs                 # phone (390x844) clinic path + desktop no-fibre path + admin
//   E2E_ONLY=phone node scripts/e2e.mjs
import { chromium } from "playwright-core";
import { mkdirSync } from "node:fs";
import { resolve } from "node:path";

const WEB = process.env.WEB_URL ?? "http://localhost:3000";
const OUT = resolve(import.meta.dirname, "../../../../docs/screens");
const CHANNEL = process.env.E2E_BROWSER ?? "msedge"; // or "chrome"
mkdirSync(OUT, { recursive: true });

const log = (...a) => console.log(new Date().toISOString().slice(11, 19), ...a);

async function shot(page, name) {
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${OUT}/${name}.png`, fullPage: true });
  log("screenshot", name);
}

/** Chat until the advisor quotes (it may ask one or two questions first). */
async function chatToQuote(page, followUps) {
  await page.getByTestId("send").click();
  for (let turn = 0; turn <= followUps.length; turn++) {
    const quoted = await page.getByTestId("see-quote").waitFor({ timeout: 120_000 }).then(() => true).catch(() => false);
    if (quoted) return;
    if (turn === followUps.length) throw new Error("advisor did not quote");
    log("advisor asked a question; answering:", followUps[turn]);
    await page.locator("#msg").fill(followUps[turn]);
    await page.getByTestId("send").click();
  }
}

async function journey(page, { owner, prefix, followUps, expectKyc, waitFor }) {
  await page.goto(WEB);
  await shot(page, `${prefix}-01-home`);
  await page.getByTestId(`demo-${owner}`).click();
  await page.waitForURL("**/chat");
  await page.waitForFunction(() => document.querySelector("#msg")?.value.length > 0);
  await chatToQuote(page, followUps);
  await shot(page, `${prefix}-02-chat`);

  await page.getByTestId("see-quote").click();
  await page.waitForURL("**/quote");
  await page.getByTestId("accept").waitFor();
  await shot(page, `${prefix}-03-quote`);
  await page.getByTestId("accept").click();

  await page.waitForURL("**/consent");
  await shot(page, `${prefix}-04-consent`);
  await page.getByTestId("agree").click();

  await page.waitForURL("**/kyc");
  await page.getByTestId("use-sample").click();
  await page.waitForFunction(() => {
    const b = document.querySelector('[data-testid="check-docs"]');
    return b && !b.disabled;
  });
  await page.getByTestId("check-docs").click();
  const result = page.getByTestId("kyc-result");
  await result.waitFor({ timeout: 120_000 });
  const status = await result.getAttribute("data-status");
  log("KYC", status);
  await shot(page, `${prefix}-05-kyc`);
  if (status !== expectKyc) throw new Error(`KYC ${status}, expected ${expectKyc}`);

  await page.getByTestId("go-track").click();
  await page.waitForURL("**/track");
  await waitFor(page);
}

async function waitStatus(page, status, timeout = 120_000) {
  await page.locator(`[data-testid="order-status"][data-status="${status}"]`).waitFor({ timeout });
  log("order", status);
}

const browser = await chromium.launch({ channel: CHANNEL, headless: true });
try {
  if (process.env.E2E_ONLY !== "desktop") {
    // Phone: Tamil dental clinic (scenario 2, specimen K03) all the way to activation.
    const phone = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
    const page = await phone.newPage();
    await journey(page, {
      owner: "clinic-tamil", prefix: "phone", expectKyc: "pass",
      followUps: ["ஒரே கிளை, 6 பேர், GSTIN உள்ளது", "சரி"],
      waitFor: async (p) => {
        await waitStatus(p, "kyc_passed", 60_000).catch(() => waitStatus(p, "install_scheduled"));
        await shot(p, "phone-06-track");
        await waitStatus(p, "activated");
        await p.waitForTimeout(3500); // let the message list catch up
        await shot(p, "phone-07-activated");
      },
    });
    await phone.close();
  }

  if (process.env.E2E_ONLY !== "phone") {
    // Desktop: fibre not feasible (scenario 19, specimen K09): delay, 5G offer, accept, activation.
    const desk = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await desk.newPage();
    await journey(page, {
      owner: "no-fibre", prefix: "desktop", expectKyc: "pass",
      followUps: ["One branch, 5 staff, we have GSTIN", "Go ahead"],
      waitFor: async (p) => {
        await p.getByTestId("fallback-accept").waitFor({ timeout: 120_000 });
        await shot(p, "desktop-06-fallback-offer");
        await p.getByTestId("fallback-accept").click();
        await waitStatus(p, "activated");
        await shot(p, "desktop-07-activated-5g");
      },
    });
    await page.evaluate(() => localStorage.setItem("udyam.adminToken", "demo-admin-token"));
    for (const [path, name] of [["/admin/leads", "admin-leads"], ["/admin/handoff", "admin-handoff"], ["/admin", "admin-orders"], ["/admin/reviews", "admin-reviews"], ["/admin/messages", "admin-messages"], ["/admin/status", "admin-status"]]) {
      await page.goto(WEB + path);
      await page.waitForTimeout(1500);
      await shot(page, `desktop-${name}`);
    }
    await desk.close();
  }
  log("E2E OK");
} finally {
  await browser.close();
}
