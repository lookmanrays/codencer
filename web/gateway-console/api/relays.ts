"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import {
  CreateRelayProfileInputSchema,
  DeleteRelayProfileResponseSchema,
  RelayListResponseSchema,
  RelayProfileResponseSchema,
  type CreateRelayProfileInput,
} from "@/schemas/relays";

export async function listRelayProfiles() {
  if (isDemoMode()) {
    return { relays: demoSnapshot.relays };
  }
  return gatewayJSON("/relays", RelayListResponseSchema);
}

export async function getRelayProfile(relayId: string) {
  if (isDemoMode()) {
    const relay = demoSnapshot.relays.find((item) => item.id === relayId);
    if (!relay) throw new Error(`Demo relay ${relayId} not found`);
    return { relay };
  }
  return gatewayJSON(
    `/relays/${encodeURIComponent(relayId)}`,
    RelayProfileResponseSchema,
  );
}

export async function createRelayProfile(input: CreateRelayProfileInput) {
  const values = CreateRelayProfileInputSchema.parse(input);
  if (isDemoMode()) {
    return {
      relay: {
        enabled: values.enabled,
        id: values.name.toLowerCase().replaceAll(/[^a-z0-9_-]+/g, "-"),
        name: values.name,
        status: "checking" as const,
        tokenConfigured: true,
        tokenRef: "server-side",
        type: "self_host" as const,
        url: values.url,
      },
    };
  }
  return gatewayJSON("/relays", RelayProfileResponseSchema, {
    body: JSON.stringify({
      enabled: values.enabled,
      name: values.name,
      token_env: values.tokenEnv,
      type: "self_host",
      url: values.url,
    }),
    method: "POST",
  });
}

export async function deleteRelayProfile(relayId: string) {
  if (isDemoMode()) {
    return { ok: true, relay_profile_id: relayId };
  }
  return gatewayJSON(
    `/relays/${encodeURIComponent(relayId)}`,
    DeleteRelayProfileResponseSchema,
    { method: "DELETE" },
  );
}

export function useRelayProfiles() {
  return useQuery({
    queryKey: queryKeys.relays,
    queryFn: listRelayProfiles,
  });
}

export function useRelayProfile(relayId: string) {
  return useQuery({
    queryKey: queryKeys.relay(relayId),
    queryFn: () => getRelayProfile(relayId),
    enabled: Boolean(relayId),
  });
}

export function useCreateRelayProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: createRelayProfile,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.relays });
      await queryClient.invalidateQueries({ queryKey: queryKeys.auditEvents });
    },
  });
}

export function useDeleteRelayProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: deleteRelayProfile,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.relays });
      await queryClient.invalidateQueries({ queryKey: queryKeys.auditEvents });
    },
  });
}
