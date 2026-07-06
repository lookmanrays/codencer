"use client";

import { useMutation } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import {
  DeviceApprovalInputSchema,
  DeviceApprovalResponseSchema,
  type DeviceApprovalInput,
} from "@/schemas/device";

export async function approveDevice(input: DeviceApprovalInput) {
  const values = DeviceApprovalInputSchema.parse(input);
  if (isDemoMode()) {
    return {
      ok: true,
      user: { email: "operator@example.com", id: "user_demo" },
      workspace: { id: "ws_personal", name: "Personal Gateway Workspace" },
    };
  }
  return gatewayJSON("/device/approve", DeviceApprovalResponseSchema, {
    body: JSON.stringify({ user_code: values.userCode }),
    method: "POST",
  });
}

export function useDeviceApproval() {
  return useMutation({
    mutationFn: approveDevice,
  });
}
