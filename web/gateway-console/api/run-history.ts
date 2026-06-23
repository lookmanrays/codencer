"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { demoSnapshot } from "@/api/demo-data";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import {
  RunDetailResponseSchema,
  RunEventsResponseSchema,
  RunListResponseSchema,
} from "@/schemas/run-history";

export async function listRuns() {
  if (isDemoMode()) {
    return { runs: demoSnapshot.runs };
  }
  return gatewayJSON("/runs", RunListResponseSchema);
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
    };
  }
  return gatewayJSON(
    `/runs/${encodeURIComponent(id)}/events`,
    RunEventsResponseSchema,
  );
}

export function useRuns() {
  return useQuery({
    queryKey: queryKeys.runs,
    queryFn: listRuns,
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
