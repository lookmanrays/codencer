import { z } from "zod";

export const ActivationCommandSchema = z.object({
  command: z.string(),
  description: z.string(),
  id: z.string(),
  target: z.enum(["gateway", "local", "client"]),
  title: z.string(),
});

export const ActivationCommandListResponseSchema = z
  .object({
    activation_commands: z.array(ActivationCommandSchema),
  })
  .transform(({ activation_commands }) => ({
    activationCommands: activation_commands,
  }));

export type ActivationCommand = z.infer<typeof ActivationCommandSchema>;
