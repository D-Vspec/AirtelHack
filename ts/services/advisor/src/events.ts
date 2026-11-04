import { randomUUID } from "node:crypto";
import { Kafka, logLevel, type Producer } from "kafkajs";
import type { EventEnvelope } from "@udyam/shared";
import type { Emit } from "./advisor.ts";

export function envelope(type: string, leadId: string, payload: unknown, correlationId = randomUUID()): EventEnvelope {
  return { id: randomUUID(), type, version: 1, occurredAt: new Date().toISOString(), leadId, correlationId, payload: payload as Record<string, unknown> };
}

/**
 * Kafka emitter keyed by leadId (same envelope as go/pkg/events). With no KAFKA_BROKERS it logs instead.
 * ponytail: fire-and-forget publish after the turn; Phase 5 adds the outbox for guaranteed delivery.
 */
export async function kafkaEmitter(brokers: string | undefined, log: (msg: string, err?: unknown) => void): Promise<{ emit: Emit; close: () => Promise<void> }> {
  if (!brokers) return { emit: async (topic) => log(`kafka disabled, dropping ${topic}`), close: async () => {} };
  const kafka = new Kafka({ clientId: "advisor", brokers: brokers.split(","), logLevel: logLevel.WARN });
  const producer: Producer = kafka.producer({ allowAutoTopicCreation: false });
  await producer.connect();
  return {
    emit: async (topic, leadId, payload) => {
      const env = envelope(topic, leadId, payload);
      try {
        await producer.send({ topic, messages: [{ key: leadId, value: JSON.stringify(env) }] });
      } catch (err) {
        log(`publish ${topic} failed`, err);
      }
    },
    close: () => producer.disconnect(),
  };
}
