import { test } from "node:test";
import assert from "node:assert/strict";
import { createChatModel } from "./llm.ts";

test("rejects unknown provider", async () => {
  await assert.rejects(() => createChatModel({ LLM_PROVIDER: "nope" }), /Unknown LLM_PROVIDER/);
});

test("gemini and openai require LLM_MODEL", async () => {
  await assert.rejects(() => createChatModel({ LLM_PROVIDER: "gemini" }), /LLM_MODEL/);
  await assert.rejects(() => createChatModel({ LLM_PROVIDER: "openai" }), /LLM_MODEL/);
});
