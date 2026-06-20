"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import { RelayHealthResponseSchema } from "@/schemas/relay-health";

export async function getRelayHealth(relayId: string) {
  if (isDemoMode()) {
    const health = demoSnapshot.relayHealth.find(
      (item) => item.relayProfileId === relayId,
    );
    if (!health) {
      return {
        health: {
          checkedAt: new Date().toISOString(),
          latencyMs: null,
          relayProfileId: relayId,
          status: "checking" as const,
        },
      };
    }
    return { health };
  }
  return gatewayJSON(
    `/relays/${encodeURIComponent(relayId)}/health`,
    RelayHealthResponseSchema,
  );
}

export function useRelayHealth(relayId: string) {
  return useQuery({
    queryKey: queryKeys.relayHealth(relayId),
    queryFn: () => getRelayHealth(relayId),
    enabled: Boolean(relayId),
  });
}
