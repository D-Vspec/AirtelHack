import type { BaseChatModel } from "@langchain/core/language_models/chat_models";

export type Provider = "gemini" | "anthropic" | "openai";
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
    default:
      throw new Error(`Unknown LLM_PROVIDER "${provider}" (use gemini | anthropic | openai)`);
  }
}
