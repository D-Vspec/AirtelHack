import { randomUUID } from "node:crypto";
import Fastify from "fastify";
import { TOPICS } from "@udyam/shared";
import { newSession, runTurn, type SessionState } from "./advisor.ts";
import { kafkaEmitter } from "./events.ts";
import { advisorLlm, createChatModel } from "./llm.ts";
import { httpServices } from "./services.ts";

const app = Fastify({ logger: { level: process.env.LOG_LEVEL ?? "info" } });
const services = httpServices();
const { emit, close } = await kafkaEmitter(process.env.KAFKA_BROKERS, (m, e) => app.log.warn({ err: e }, m));
// ponytail: sessions in memory, lost on restart; Phase 5 (resume links) persists them in the advisor schema.
const sessions = new Map<string, SessionState>();
let llm: ReturnType<typeof advisorLlm> | undefined;

app.get("/healthz", async () => ({ status: "ok", service: "advisor" }));
app.get("/ping", async () => ({ service: "advisor", time: new Date().toISOString() }));

app.post<{ Body: { sessionId?: string; leadId?: string; message?: string } }>("/chat", {
  schema: {
    body: {
      type: "object", required: ["message"], additionalProperties: false,
      properties: { sessionId: { type: "string", maxLength: 64 }, leadId: { type: "string", maxLength: 64 }, message: { type: "string", minLength: 1, maxLength: 2000 } },
    },
  },
}, async (req, reply) => {
  try {
    llm ??= advisorLlm(await createChatModel());
  } catch (e) {
    reply.code(503);
    return { code: "LLM_NOT_CONFIGURED", message: String((e as Error).message) };
  }
  const { sessionId, leadId, message } = req.body;
  let state = sessionId ? sessions.get(sessionId) : undefined;
  if (!state) {
    state = newSession(sessionId ?? randomUUID(), leadId);
    sessions.set(state.sessionId, state);
    await emit(TOPICS.LEAD_CREATED, state.leadId, { sessionId: state.sessionId, channel: "advisor" });
  }
  const r = await runTurn(state, message!, { llm, services, emit });
  return {
    sessionId: state.sessionId, leadId: state.leadId, turn: state.turn, reply: r.reply, kind: r.brief.kind,
    language: state.language, romanised: state.romanised, profile: state.profile, quote: state.quote ?? null,
  };
});

app.get<{ Params: { id: string } }>("/sessions/:id", async (req, reply) => {
  const s = sessions.get(req.params.id);
  if (!s) { reply.code(404); return { code: "SESSION_NOT_FOUND" }; }
  return s;
});

const port = Number(process.env.PORT ?? 8090);
app.addHook("onClose", async () => close());
app.listen({ port, host: "0.0.0.0" }).catch((err) => {
  app.log.error(err);
  process.exit(1);
});
