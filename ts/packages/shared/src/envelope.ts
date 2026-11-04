import { z } from "zod";

/** Mirrors contracts/events.schema.json and go/pkg/events/envelope.go */
export const EventEnvelope = z.object({
  id: z.string().min(1),
  type: z.string().min(1),
  version: z.number().int().min(1),
  occurredAt: z.string(),
  leadId: z.string().optional(),
  correlationId: z.string().min(1),
  payload: z.record(z.string(), z.unknown()),
});
export type EventEnvelope = z.infer<typeof EventEnvelope>;
