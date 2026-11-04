import { test } from "node:test";
import assert from "node:assert/strict";
import { LANGUAGES, STRINGS, STRINGS_EN, t, inr } from "../src/i18n.ts";
import { KYC_FINDINGS } from "../src/kyc-findings.ts";
import { KYC_MESSAGES, KYC_MESSAGE_LANGUAGES } from "../src/kyc-messages.ts";
import { DEMO_OWNERS } from "../src/demo.ts";

test("every language has every UI string with the same placeholders", () => {
  const ph = (s: string) => (s.match(/\{[a-z]+\}/g) ?? []).sort().join(",");
  for (const { code } of LANGUAGES) {
    for (const key of Object.keys(STRINGS_EN) as (keyof typeof STRINGS_EN)[]) {
      const s = STRINGS[code][key];
      assert.ok(s && s.trim(), `${code}.${key} missing`);
      assert.equal(ph(s), ph(STRINGS_EN[key]), `${code}.${key} placeholders`);
    }
  }
});

test("every KYC finding code has a message and next step in every UI language", () => {
  for (const { code } of LANGUAGES) assert.ok((KYC_MESSAGE_LANGUAGES as readonly string[]).includes(code), code);
  for (const finding of Object.keys(KYC_FINDINGS)) {
    for (const lang of KYC_MESSAGE_LANGUAGES) {
      const m = KYC_MESSAGES[finding]?.[lang];
      assert.ok(m?.message && m?.action, `${finding}/${lang}`);
    }
  }
});

test("t fills params and inr groups the Indian way", () => {
  assert.equal(t("en", "totalContract", { months: 12 }), "Total for 12 months");
  assert.equal(t("hi", "expectedBy", { date: "2026-08-05" }), "2026-08-05 तक चालू होने की उम्मीद");
  assert.equal(inr(3933), "₹3,933");
  assert.equal(inr(123456), "₹1,23,456");
  assert.equal(inr(12345678), "₹1,23,45,678");
  assert.equal(inr(999), "₹999");
});

test("demo owners use real specimens", () => {
  for (const o of DEMO_OWNERS) assert.match(o.specimen, /^K(0[1-9]|1[0-2])$/);
});

test("every language labels every order status", async () => {
  const { ORDER_STATUS_LABELS } = await import("../src/i18n.ts");
  const { ORDER_HAPPY_PATH } = await import("../src/orders.ts");
  for (const { code } of LANGUAGES) {
    for (const s of [...ORDER_HAPPY_PATH, "delayed", "failed"]) assert.ok(ORDER_STATUS_LABELS[code][s], `${code}/${s}`);
  }
});
