import { z } from "zod";
import { collectionField } from "@/schemas/collections";

export const AuditEventSchema = z.object({
  actor: z.string(),
  createdAt: z.string(),
  id: z.string(),
  severity: z.enum(["info", "warning", "error"]),
  summary: z.string(),
  type: z.string(),
});

const RawAuditEventSchema = z.object({
  actor_user_id: z.string().optional(),
  created_at: z.string(),
  id: z.string(),
  summary: z.string(),
  type: z.string(),
});

export const AuditEventListResponseSchema = z
  .object({
    audit_events: collectionField(RawAuditEventSchema).optional(),
    events: collectionField(RawAuditEventSchema).optional(),
  })
  .transform(({ audit_events, events }) => ({
    auditEvents: (audit_events ?? events ?? []).map((event) =>
      AuditEventSchema.parse({
        actor: event.actor_user_id || "gateway",
        createdAt: event.created_at,
        id: event.id,
        severity: event.type.includes("blocker") ? "warning" : "info",
        summary: event.summary,
        type: event.type,
      }),
    ),
  }));

export type AuditEvent = z.infer<typeof AuditEventSchema>;
