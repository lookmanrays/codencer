"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import { WorkspaceResponseSchema } from "@/schemas/workspace";

export async function getWorkspace() {
  if (isDemoMode()) {
    return {
      mcpEndpoint: demoSnapshot.mcpEndpoint,
      publicBaseURL: "http://127.0.0.1:19090",
      user: demoSnapshot.user,
      workspace: {
        ...demoSnapshot.workspace,
        kind: "demo",
        mode: "demo" as const,
      },
    };
  }
  return gatewayJSON("/workspace", WorkspaceResponseSchema);
}

export function useWorkspace() {
  return useQuery({
    queryKey: queryKeys.workspace,
    queryFn: getWorkspace,
  });
}
