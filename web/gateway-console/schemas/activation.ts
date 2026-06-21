import { z } from "zod";
import { collectionField } from "@/schemas/collections";

export const ActivationCommandSchema = z.object({
  command: z.string(),
  description: z.string(),
  id: z.string(),
  target: z.enum(["gateway", "local", "client"]),
  title: z.string(),
});

export const ActivationCommandListResponseSchema = z
  .object({
    activation_commands: collectionField(ActivationCommandSchema).optional(),
    commands: collectionField(ActivationCommandSchema).optional(),
  })
  .transform(({ activation_commands, commands }) => ({
    activationCommands: activation_commands ?? commands ?? [],
  }));

export type ActivationCommand = z.infer<typeof ActivationCommandSchema>;
