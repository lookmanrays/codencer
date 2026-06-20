"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import { ConnectorListResponseSchema } from "@/schemas/connectors";

export async function listConnectors() {
  if (isDemoMode()) {
    return { connectors: demoSnapshot.connectors };
  }
  return gatewayJSON("/connectors", ConnectorListResponseSchema);
}

export function useConnectors() {
  return useQuery({
    queryKey: queryKeys.connectors,
    queryFn: listConnectors,
  });
}
