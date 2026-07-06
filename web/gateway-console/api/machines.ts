"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import { MachineListResponseSchema } from "@/schemas/machines";

export async function listMachines() {
  if (isDemoMode()) {
    return { machines: demoSnapshot.machines };
  }
  return gatewayJSON("/machines", MachineListResponseSchema);
}

export function useMachines() {
  return useQuery({
    queryKey: queryKeys.machines,
    queryFn: listMachines,
  });
}
