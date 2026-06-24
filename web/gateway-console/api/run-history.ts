"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { demoSnapshot } from "@/api/demo-data";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import {
  HumanInterruptResponseSchema,
  RunDetailResponseSchema,
  RunEventsResponseSchema,
  RunListResponseSchema,
} from "@/schemas/run-history";

export type RunListParams = {
  limit?: number;
  offset?: number;
  projectId?: string;
  scope?: string;
  status?: string;
};

export type HumanInterruptResponseInput = {
  followUp?: string;
  reason?: string;
  response: string;
  responseType: string;
  runHistoryId: string;
};

function runListQuery(params: RunListParams = {}) {
  const search = new URLSearchParams();
  if (params.limit) search.set("limit", String(params.limit));
  if (params.offset) search.set("offset", String(params.offset));
  if (params.projectId) search.set("project_id", params.projectId);
  if (params.scope) search.set("scope", params.scope);
  if (params.status) search.set("status", params.status);
  const query = search.toString();
  return query ? `/runs?${query}` : "/runs";
}

export async function listRuns(params: RunListParams = {}) {
  if (isDemoMode()) {
    return {
      pagination: {
        has_more: false,
        limit: params.limit ?? 100,
        offset: params.offset ?? 0,
      },
      runs: demoSnapshot.runs,
    };
  }
  return gatewayJSON(runListQuery(params), RunListResponseSchema);
}

export async function getRun(id: string) {
  if (isDemoMode()) {
    const run = demoSnapshot.runs.find((item) => item.id === id);
    if (!run) {
      throw new Error(`Run not found: ${id}`);
    }
    return { run };
  }
  return gatewayJSON(
    `/runs/${encodeURIComponent(id)}`,
    RunDetailResponseSchema,
  );
}

export async function getRunEvents(id: string) {
  if (isDemoMode()) {
    return {
      events: demoSnapshot.auditEvents.filter(
        (event) => event.runHistoryId === id,
      ),
      groups: [],
      pagination: { has_more: false, limit: 100, offset: 0 },
    };
  }
  return gatewayJSON(
    `/runs/${encodeURIComponent(id)}/events`,
    RunEventsResponseSchema,
  );
}

export async function respondToHumanInterrupt(
  input: HumanInterruptResponseInput,
) {
  if (isDemoMode()) {
    return HumanInterruptResponseSchema.parse({
      follow_up: input.followUp,
      follow_up_result:
        input.followUp === "resume"
          ? {
              ok: true,
              result: { events: [{ type: "run_resumed" }] },
              status: "running",
            }
          : undefined,
      next_actions: {
        cancel_operation: "codencer.cancel_project_run",
        cancel_supported: true,
        resume_attempted: input.followUp === "resume",
        resume_operation: "codencer.resume_project_run",
        resume_supported: input.followUp === "resume",
      },
      ok: true,
      project_id: "codencer",
      response: {
        operator_response: input.response,
        type: input.responseType,
      },
      run_history_id: input.runHistoryId,
      run_id: "run_demo_blocked",
      status: "human_interrupt_response_recorded",
    });
  }
  return gatewayJSON(
    `/runs/${encodeURIComponent(input.runHistoryId)}/human-interrupts/respond`,
    HumanInterruptResponseSchema,
    {
      body: JSON.stringify({
        follow_up: input.followUp,
        reason: input.reason,
        response: input.response,
        response_type: input.responseType,
      }),
      method: "POST",
    },
  );
}

export function useRuns(params: RunListParams = {}) {
  return useQuery({
    queryKey: queryKeys.runsPage(params),
    queryFn: () => listRuns(params),
  });
}

export function useRun(id: string) {
  return useQuery({
    enabled: Boolean(id),
    queryKey: queryKeys.run(id),
    queryFn: () => getRun(id),
  });
}

export function useRunEvents(id: string) {
  return useQuery({
    enabled: Boolean(id),
    queryKey: queryKeys.runEvents(id),
    queryFn: () => getRunEvents(id),
  });
}

export function useRespondToHumanInterrupt() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: respondToHumanInterrupt,
    onSuccess: async (_data, input) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.auditEvents }),
        queryClient.invalidateQueries({ queryKey: queryKeys.runs }),
        queryClient.invalidateQueries({
          queryKey: queryKeys.run(input.runHistoryId),
        }),
        queryClient.invalidateQueries({
          queryKey: queryKeys.runEvents(input.runHistoryId),
        }),
      ]);
    },
  });
}
