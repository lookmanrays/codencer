import { z } from "zod";
import { collectionField } from "@/schemas/collections";

export const RelayProfileSchema = z.object({
  enabled: z.boolean(),
  id: z.string(),
  name: z.string(),
  status: z.enum(["available", "unavailable", "disabled", "checking"]),
  tokenConfigured: z.boolean(),
  tokenRef: z.string(),
  type: z.enum(["managed", "self_host"]),
  url: z.string().url(),
});

export const RelayProfileResponseSchema = z
  .object({
    relay: z.object({
      enabled: z.boolean(),
      id: z.string(),
      name: z.string(),
      status: z.string().optional(),
      token_configured: z.boolean().optional(),
      type: z.string(),
      url: z.string().url(),
    }),
  })
  .transform(({ relay }) => ({
    relay: normalizeRelayProfile(relay),
  }));

export const RelayListResponseSchema = z
  .object({
    relays: collectionField(
      z.object({
        enabled: z.boolean(),
        id: z.string(),
        name: z.string(),
        status: z.string().optional(),
        token_configured: z.boolean().optional(),
        type: z.string(),
        url: z.string().url(),
      }),
    ),
  })
  .transform(({ relays }) => ({
    relays: relays.map(normalizeRelayProfile),
  }));

export const CreateRelayProfileInputSchema = z.object({
  enabled: z.boolean().default(true),
  name: z.string().min(2, "Name is required"),
  tokenEnv: z.string().min(2, "Use an environment variable reference"),
  url: z.string().url("Use a valid Relay URL"),
});

export const DeleteRelayProfileResponseSchema = z.object({
  ok: z.boolean(),
  relay_profile_id: z.string(),
});

function normalizeRelayProfile(relay: {
  enabled: boolean;
  id: string;
  name: string;
  status?: string;
  token_configured?: boolean;
  type: string;
  url: string;
}) {
  const enabled = relay.enabled;
  return RelayProfileSchema.parse({
    enabled,
    id: relay.id,
    name: relay.name,
    status: enabled
      ? relay.status === "available" || relay.status === "unavailable"
        ? relay.status
        : "checking"
      : "disabled",
    tokenConfigured: relay.token_configured ?? false,
    tokenRef: relay.token_configured ? "server-side" : "not configured",
    type: relay.type === "managed" ? "managed" : "self_host",
    url: relay.url,
  });
}

export type RelayProfile = z.infer<typeof RelayProfileSchema>;
export type CreateRelayProfileInput = z.input<
  typeof CreateRelayProfileInputSchema
>;
