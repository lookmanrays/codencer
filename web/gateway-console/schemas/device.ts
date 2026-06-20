import { z } from "zod";

export const DeviceApprovalInputSchema = z.object({
  userCode: z.string().min(4, "Enter a device code like ABCD-EFGH"),
});

export const DeviceApprovalResponseSchema = z.object({
  ok: z.boolean(),
  user: z.object({
    email: z.string().email(),
    id: z.string(),
  }),
  workspace: z.object({
    id: z.string(),
    name: z.string(),
  }),
});

export type DeviceApprovalInput = z.infer<typeof DeviceApprovalInputSchema>;
export type DeviceApprovalResponse = z.infer<
  typeof DeviceApprovalResponseSchema
>;
