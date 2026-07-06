import { z } from "zod";
import { collectionField } from "@/schemas/collections";

export const ExecutorProfileSchema = z.object({
  adapter: z.string(),
  approval: z.string().optional(),
  daemon_adapter: z.string(),
  dangerous_bypass_approvals_and_sandbox: z.boolean().optional(),
  description: z.string(),
  id: z.string(),
  output_format: z.string().optional(),
  requires_explicit_allow_dangerous_bypass: z.boolean().optional(),
  sandbox: z.string().optional(),
});

export const ExecutorListResponseSchema = z.object({
  executors: collectionField(ExecutorProfileSchema),
});

export type ExecutorProfile = z.infer<typeof ExecutorProfileSchema>;
