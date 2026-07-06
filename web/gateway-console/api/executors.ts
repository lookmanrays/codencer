"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { demoSnapshot } from "@/api/demo-data";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { ExecutorListResponseSchema } from "@/schemas/executors";

export async function listExecutors() {
  if (isDemoMode()) {
    return { executors: demoSnapshot.executors };
  }
  return gatewayJSON("/executors", ExecutorListResponseSchema);
}

export function useExecutors() {
  return useQuery({
    queryKey: queryKeys.executors,
    queryFn: listExecutors,
  });
}
