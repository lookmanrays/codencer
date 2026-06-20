import { z } from "zod";

export const MachineSchema = z.object({
  arch: z.string(),
  hostLabel: z.string(),
  hostname: z.string(),
  id: z.string(),
  os: z.string(),
  status: z.enum(["online", "offline", "unknown"]),
});

export const MachineListResponseSchema = z
  .object({
    machines: z.array(
      z.object({
        arch: z.string().optional(),
        host_label: z.string().optional(),
        hostname: z.string().optional(),
        id: z.string(),
        os: z.string().optional(),
        status: z.string().optional(),
      }),
    ),
  })
  .transform(({ machines }) => ({
    machines: machines.map((machine) =>
      MachineSchema.parse({
        arch: machine.arch ?? "unknown",
        hostLabel: machine.host_label ?? machine.id,
        hostname: machine.hostname ?? "",
        id: machine.id,
        os: machine.os ?? "unknown",
        status: machine.status === "active" ? "online" : "unknown",
      }),
    ),
  }));

export type Machine = z.infer<typeof MachineSchema>;
