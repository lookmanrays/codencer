import { z } from "zod";

export const RelayHealthSchema = z.object({
  checkedAt: z.string(),
  latencyMs: z.number().nullable(),
  relayProfileId: z.string(),
  status: z.enum(["available", "unavailable", "disabled", "checking"]),
});

export const RelayHealthResponseSchema = z
  .object({
    health: z.object({
      checked_at: z.string(),
      latency_ms: z.number().nullable(),
      relay_profile_id: z.string(),
      status: z.string(),
    }),
  })
  .transform(({ health }) => ({
    health: RelayHealthSchema.parse({
      checkedAt: health.checked_at,
      latencyMs: health.latency_ms,
      relayProfileId: health.relay_profile_id,
      status:
        health.status === "available" ||
        health.status === "unavailable" ||
        health.status === "disabled"
          ? health.status
          : "checking",
    }),
  }));

export type RelayHealth = z.infer<typeof RelayHealthSchema>;
