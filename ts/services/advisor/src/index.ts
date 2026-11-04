import { randomUUID } from "node:crypto";
import Fastify from "fastify";
import { TOPICS } from "@udyam/shared";
import { newSession, runTurn, type SessionState } from "./advisor.ts";
import { kafkaEmitter } from "./events.ts";
import { advisorLlm, createChatModel } from "./llm.ts";
import { httpServices } from "./services.ts";
import { HANDOFF_REASONS, handoffSummary, type HandoffReason } from "./handoff.ts";

const app = Fastify({ logger: { level: process.env.LOG_LEVEL ?? "info" } });
const services = httpServices();
const { emit, close } = await kafkaEmitter(process.env.KAFKA_BROKERS, (m, e) => app.log.warn({ err: e }, m));
// ponytail: sessions in memory, lost on restart; Phase 5 (resume links) persists them in the advisor schema.
const sessions = new Map<string, SessionState>();
let llm: ReturnType<typeof advisorLlm> | undefined;

app.get("/healthz", async () => ({ status: "ok", service: "advisor" }));

// Prometheus text, same metric names as the Go services (go/pkg/svc/metrics.go).
const started = Date.now();
const reqs = new Map<string, number>();
const lat = new Map<string, [number, number]>();
app.addHook("onResponse", async (req, reply) => {
  const route = req.routeOptions.url ?? "unknown";
  if (route === "/metrics" || route === "/healthz") return;
  const k = `${req.method}|${route}|${Math.floor(reply.statusCode / 100)}xx`;
  reqs.set(k, (reqs.get(k) ?? 0) + 1);
  const lk = `${req.method}|${route}`;
  const [s, c] = lat.get(lk) ?? [0, 0];
  lat.set(lk, [s + reply.elapsedTime / 1000, c + 1]);
});
app.get("/metrics", async (_req, reply) => {
  const lines = ["# TYPE udyam_http_requests_total counter"];
  for (const [k, v] of reqs) { const [m, r, st] = k.split("|"); lines.push(`udyam_http_requests_total{service="advisor",method="${m}",route="${r}",status="${st}"} ${v}`); }
  lines.push("# TYPE udyam_http_request_seconds summary");
  for (const [k, [s, c]] of lat) { const [m, r] = k.split("|"); lines.push(`udyam_http_request_seconds_sum{service="advisor",method="${m}",route="${r}"} ${s.toFixed(6)}`, `udyam_http_request_seconds_count{service="advisor",method="${m}",route="${r}"} ${c}`); }
  lines.push("# TYPE udyam_advisor_sessions gauge", `udyam_advisor_sessions{service="advisor"} ${sessions.size}`);
  lines.push("# TYPE udyam_uptime_seconds gauge", `udyam_uptime_seconds{service="advisor"} ${Math.round((Date.now() - started) / 1000)}`);
  reply.type("text/plain; version=0.0.4");
  return lines.join("\n") + "\n";
});
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

// Hand the lead to a partner (scenario 21): emits lead.unclosed with a deterministic summary and
// the transcript; the scoring service opens a partner task ranked by the lead's score.
app.post<{ Body: { sessionId: string; reason?: HandoffReason } }>("/handoff", {
  schema: {
    body: {
      type: "object", required: ["sessionId"], additionalProperties: false,
      properties: { sessionId: { type: "string", maxLength: 64 }, reason: { type: "string", enum: [...HANDOFF_REASONS] } },
    },
  },
}, async (req, reply) => {
  const s = sessions.get(req.body.sessionId);
  if (!s) { reply.code(404); return { code: "SESSION_NOT_FOUND", message: "no such advisor session" }; }
  const reason = req.body.reason ?? "owner_asked_for_person";
  const summary = handoffSummary(s, reason);
  await emit(TOPICS.LEAD_UNCLOSED, s.leadId, { sessionId: s.sessionId, leadId: s.leadId, reason, summary, profile: s.profile, transcript: s.history.slice(-12) });
  return { leadId: s.leadId, reason, summary };
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
