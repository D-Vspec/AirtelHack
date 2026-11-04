import type { BaseChatModel } from "@langchain/core/language_models/chat_models";
import { HumanMessage, SystemMessage } from "@langchain/core/messages";
import { AdvisorExtraction } from "@udyam/shared";
import type { Llm } from "./advisor.ts";
import { extractionPrompt, writerPrompt } from "./prompts.ts";

/** Advisor LLM roles on top of any LangChain chat model: structured extraction and reply writing. */
export function advisorLlm(model: BaseChatModel, env: Env = process.env): Llm {
  // ChatOpenAI defaults to strict response_format json_schema: OpenAI rejects it (every key must be
  // required, ours are optional) and DeepSeek doesn't support it. Tool calling works for both.
  const method = ["openai", "deepseek"].includes(env.LLM_PROVIDER ?? "") ? "functionCalling" : undefined;
  const extractor = model.withStructuredOutput(AdvisorExtraction, { name: "extract_profile", method });
  return {
    async extract(input) {
      const { system, human } = extractionPrompt(input);
      return AdvisorExtraction.parse(await extractor.invoke([new SystemMessage(system), new HumanMessage(human)]));
    },
    async write(input) {
      const { system, human } = writerPrompt(input);
      const res = await model.invoke([new SystemMessage(system), new HumanMessage(human)]);
      return typeof res.content === "string"
        ? res.content.trim()
        : res.content.map((c) => ("text" in c && typeof c.text === "string" ? c.text : "")).join("").trim();
    },
  };
}

export type Provider = "gemini" | "anthropic" | "openai" | "deepseek";
type Env = Record<string, string | undefined>;

/**
 * Provider switch used by the advisor and the eval harness (Phase 3).
 * Gemini and OpenAI need an explicit LLM_MODEL (model names change too often to hard-code a guess).
 */
export async function createChatModel(env: Env = process.env): Promise<BaseChatModel> {
  const provider = (env.LLM_PROVIDER ?? "gemini") as Provider;
  const model = env.LLM_MODEL || undefined;
  switch (provider) {
    case "anthropic": {
      const { ChatAnthropic } = await import("@langchain/anthropic");
      return new ChatAnthropic({ model: model ?? "claude-sonnet-5-5", apiKey: env.ANTHROPIC_API_KEY, temperature: 0 });
    }
    case "gemini": {
      if (!model) throw new Error("Set LLM_MODEL for LLM_PROVIDER=gemini (a current Gemini model id)");
      const { ChatGoogleGenerativeAI } = await import("@langchain/google-genai");
      return new ChatGoogleGenerativeAI({ model, apiKey: env.GOOGLE_API_KEY, temperature: 0 });
    }
    case "openai": {
      if (!model) throw new Error("Set LLM_MODEL for LLM_PROVIDER=openai (a current OpenAI model id)");
      const { ChatOpenAI } = await import("@langchain/openai");
      return new ChatOpenAI({ model, apiKey: env.OPENAI_API_KEY, temperature: 0 });
    }
    case "deepseek": {
      // OpenAI-compatible API. Default deepseek-flash = DeepSeek-V4.1-Flash.
      const { ChatOpenAI } = await import("@langchain/openai");
      return new ChatOpenAI({ model: model ?? "deepseek-flash", apiKey: env.DEEPSEEK_API_KEY, temperature: 0, configuration: { baseURL: env.DEEPSEEK_BASE_URL || "https://api.deepseek.com" } });
    }
    default:
      throw new Error(`Unknown LLM_PROVIDER "${provider}" (use gemini | anthropic | openai | deepseek)`);
  }
}
