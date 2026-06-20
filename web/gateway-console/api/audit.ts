"use client";

import { useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import { demoSnapshot } from "@/api/demo-data";
import { AuditEventListResponseSchema } from "@/schemas/audit";

export async function listAuditEvents() {
  if (isDemoMode()) {
    return { auditEvents: demoSnapshot.auditEvents };
  }
  return gatewayJSON("/audit-events", AuditEventListResponseSchema);
}

export function useAuditEvents() {
  return useQuery({
    queryKey: queryKeys.auditEvents,
    queryFn: listAuditEvents,
  });
}
