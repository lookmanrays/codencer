import { z } from "zod";
import { collectionField } from "@/schemas/collections";

export const AuditEventSchema = z.object({
  actor: z.string(),
  createdAt: z.string(),
  id: z.string(),
  metadata: z.record(z.string(), z.unknown()).optional(),
  runHistoryId: z.string().optional(),
  severity: z.enum(["info", "warning", "error"]),
  summary: z.string(),
  type: z.string(),
});

const RawAuditEventSchema = z.object({
  actor_user_id: z.string().optional(),
  created_at: z.string(),
  id: z.string(),
  metadata: z.record(z.string(), z.unknown()).optional(),
  summary: z.string(),
  type: z.string(),
});

export const PaginationSchema = z.object({
  has_more: z.boolean().optional().default(false),
  limit: z.number().optional().default(100),
  next_offset: z.number().optional(),
  offset: z.number().optional().default(0),
});

const AuditEventGroupSchema = z
  .object({
    event_count: z.number(),
    first_event_at: z.string(),
    id: z.string(),
    last_event_at: z.string(),
    project_id: z.string().optional(),
    run_history_id: z.string(),
    run_id: z.string().optional(),
    summary: z.string(),
    types: collectionField(z.string()),
  })
  .transform((group) => ({
    eventCount: group.event_count,
    firstEventAt: group.first_event_at,
    id: group.id,
    lastEventAt: group.last_event_at,
    projectId: group.project_id,
    runHistoryId: group.run_history_id,
    runId: group.run_id,
    summary: group.summary,
    types: group.types,
  }));

export const AuditEventListResponseSchema = z
  .object({
    audit_events: collectionField(RawAuditEventSchema).optional(),
    events: collectionField(RawAuditEventSchema).optional(),
    groups: collectionField(AuditEventGroupSchema).optional(),
    pagination: PaginationSchema.optional(),
  })
  .transform(({ audit_events, events, groups, pagination }) => ({
    auditEvents: (audit_events ?? events ?? []).map((event) =>
      AuditEventSchema.parse({
        actor: event.actor_user_id || "gateway",
        createdAt: event.created_at,
        id: event.id,
        metadata: event.metadata,
        runHistoryId:
          typeof event.metadata?.run_history_id === "string"
            ? event.metadata.run_history_id
            : undefined,
        severity: event.type.includes("blocker") ? "warning" : "info",
        summary: event.summary,
        type: event.type,
      }),
    ),
    groups: groups ?? [],
    pagination: pagination ?? PaginationSchema.parse({}),
  }));

export type AuditEvent = z.infer<typeof AuditEventSchema>;
export type AuditEventGroup = z.infer<typeof AuditEventGroupSchema>;
export type Pagination = z.infer<typeof PaginationSchema>;
