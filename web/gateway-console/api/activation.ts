"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import { ActivationCommandListResponseSchema } from "@/schemas/activation";

export async function listActivationCommands() {
  if (isDemoMode()) {
    return { activationCommands: demoSnapshot.activationCommands };
  }
  return gatewayJSON(
    "/activation/commands",
    ActivationCommandListResponseSchema,
  );
}

export function useActivationCommands() {
  return useQuery({
    queryKey: queryKeys.activationCommands,
    queryFn: listActivationCommands,
  });
}
