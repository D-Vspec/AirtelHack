import Fastify from "fastify";
import { BusinessProfile } from "@udyam/shared";

const app = Fastify({ logger: { level: process.env.LOG_LEVEL ?? "info" } });

app.get("/healthz", async () => ({ status: "ok", service: "advisor" }));
app.get("/ping", async () => ({ service: "advisor", time: new Date().toISOString() }));

// Phase 3 replaces this stub with the LangChain.js conversation graph.
app.post("/chat", async (_req, reply) => {
  reply.code(501);
  return { error: "not_implemented", phase: 3, profileSchemaFields: Object.keys(BusinessProfile.shape).length };
});

const port = Number(process.env.PORT ?? 8090);
app.listen({ port, host: "0.0.0.0" }).catch((err) => {
  app.log.error(err);
  process.exit(1);
});
