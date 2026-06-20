import { z } from "zod";

export const ConnectorSchema = z.object({
  id: z.string(),
  label: z.string(),
  lastSeen: z.string(),
  machineId: z.string(),
  relayProfileId: z.string(),
  status: z.enum(["online", "offline", "pending"]),
});

export const ConnectorListResponseSchema = z
  .object({
    connectors: z.array(
      z.object({
        id: z.string(),
        last_seen_at: z.string().optional(),
        machine_id: z.string(),
        relay_profile_id: z.string(),
        status: z.string().optional(),
      }),
    ),
  })
  .transform(({ connectors }) => ({
    connectors: connectors.map((connector) =>
      ConnectorSchema.parse({
        id: connector.id,
        label: connector.id,
        lastSeen: connector.last_seen_at ?? "",
        machineId: connector.machine_id,
        relayProfileId: connector.relay_profile_id,
        status:
          connector.status === "online"
            ? "online"
            : connector.status === "pending"
              ? "pending"
              : "offline",
      }),
    ),
  }));

export type Connector = z.infer<typeof ConnectorSchema>;
