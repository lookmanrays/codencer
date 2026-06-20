"use client";

import { useMemo, useState } from "react";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { EmptyState } from "@/components/ui/empty-state";
import { LoadingPanel } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { isDemoMode } from "@/api/config";
import { useAuditEvents } from "@/api/audit";
import type { AuditEvent } from "@/schemas/audit";

export function AuditScreen() {
  const [filter, setFilter] = useState("all");
  const audit = useAuditEvents();
  return (
    <PageShell
      actions={
        <Select onValueChange={setFilter} value={filter}>
          <SelectTrigger className="w-[220px]">
            <SelectValue placeholder="Event type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All events</SelectItem>
            <SelectItem value="relay.add">Relay changes</SelectItem>
            <SelectItem value="connector.login">Connector login</SelectItem>
            <SelectItem value="routing.blocker">Routing blockers</SelectItem>
          </SelectContent>
        </Select>
      }
      breadcrumbs={[{ label: "Console", href: "/console" }, { label: "Audit" }]}
      description="Review recent Gateway workspace activity without exposing secrets."
      kicker="Audit / events"
      title="Workspace event stream"
    >
      {audit.isLoading ? <LoadingPanel /> : null}
      {audit.error ? (
        <Alert title="Audit API unavailable" tone="error">
          {audit.error.message}
        </Alert>
      ) : null}
      {audit.data ? (
        <div className="grid gap-md">
          {isDemoMode() ? <DemoModeNotice /> : null}
          {audit.data.auditEvents.length === 0 ? (
            <EmptyState
              description="No events have been recorded."
              title="No audit events"
            />
          ) : (
            <AuditContent events={audit.data.auditEvents} filter={filter} />
          )}
        </div>
      ) : null}
    </PageShell>
  );
}

function AuditContent({
  events,
  filter,
}: {
  events: AuditEvent[];
  filter: string;
}) {
  const filtered = useMemo(
    () =>
      filter === "all"
        ? events
        : events.filter((event) => event.type === filter),
    [events, filter],
  );
  return <AuditEventTimeline events={filtered} />;
}
