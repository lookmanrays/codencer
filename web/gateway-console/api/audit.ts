"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import { AuditEventListResponseSchema } from "@/schemas/audit";

export type AuditEventListParams = {
  limit?: number;
  offset?: number;
  projectId?: string;
  runHistoryId?: string;
  runId?: string;
  type?: string;
};

function auditQuery(params: AuditEventListParams = {}) {
  const search = new URLSearchParams();
  if (params.limit) search.set("limit", String(params.limit));
  if (params.offset) search.set("offset", String(params.offset));
  if (params.projectId) search.set("project_id", params.projectId);
  if (params.runHistoryId) search.set("run_history_id", params.runHistoryId);
  if (params.runId) search.set("run_id", params.runId);
  if (params.type) search.set("type", params.type);
  const query = search.toString();
  return query ? `/audit-events?${query}` : "/audit-events";
}

export async function listAuditEvents(params: AuditEventListParams = {}) {
  if (isDemoMode()) {
    const filtered = demoSnapshot.auditEvents.filter((event) => {
      if (params.type && event.type !== params.type) return false;
      if (params.runHistoryId && event.runHistoryId !== params.runHistoryId) {
        return false;
      }
      if (
        params.projectId &&
        (typeof event.metadata?.project_id !== "string" ||
          event.metadata.project_id !== params.projectId)
      ) {
        return false;
      }
      if (
        params.runId &&
        (typeof event.metadata?.run_id !== "string" ||
          event.metadata.run_id !== params.runId)
      ) {
        return false;
      }
      return true;
    });
    return AuditEventListResponseSchema.parse({
      events: filtered.map((event) => ({
        actor_user_id: event.actor,
        created_at: event.createdAt,
        id: event.id,
        metadata: event.metadata,
        summary: event.summary,
        type: event.type,
      })),
      groups: demoAuditGroups(filtered),
      pagination: {
        has_more: false,
        limit: params.limit ?? 100,
        offset: params.offset ?? 0,
      },
    });
  }
  return gatewayJSON(auditQuery(params), AuditEventListResponseSchema);
}

function demoAuditGroups(events: typeof demoSnapshot.auditEvents) {
  const groups = new Map<
    string,
    {
      event_count: number;
      first_event_at: string;
      id: string;
      last_event_at: string;
      project_id?: string;
      run_history_id: string;
      run_id?: string;
      summary: string;
      types: string[];
    }
  >();
  for (const event of events) {
    const runHistoryId = event.runHistoryId;
    if (!runHistoryId) continue;
    const existing = groups.get(runHistoryId) ?? {
      event_count: 0,
      first_event_at: event.createdAt,
      id: `run:${runHistoryId}`,
      last_event_at: event.createdAt,
      project_id:
        typeof event.metadata?.project_id === "string"
          ? event.metadata.project_id
          : undefined,
      run_history_id: runHistoryId,
      run_id:
        typeof event.metadata?.run_id === "string"
          ? event.metadata.run_id
          : undefined,
      summary: "",
      types: [],
    };
    existing.event_count += 1;
    if (event.createdAt < existing.first_event_at) {
      existing.first_event_at = event.createdAt;
    }
    if (event.createdAt > existing.last_event_at) {
      existing.last_event_at = event.createdAt;
    }
    if (!existing.types.includes(event.type)) {
      existing.types.push(event.type);
    }
    existing.summary = `${existing.event_count} lifecycle events for run ${
      existing.run_id ?? runHistoryId
    }`;
    groups.set(runHistoryId, existing);
  }
  return Array.from(groups.values()).map((group) => ({
    ...group,
    types: group.types.sort(),
  }));
}

export function useAuditEvents(params: AuditEventListParams = {}) {
  return useQuery({
    queryKey: queryKeys.auditEventsPage(params),
    queryFn: () => listAuditEvents(params),
  });
}
