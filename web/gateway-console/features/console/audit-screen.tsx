"use client";

import { useMemo, useState } from "react";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { PageShell } from "@/components/layout/page-shell";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ConsoleData } from "@/features/console/use-console-data";
import type { AuditEvent } from "@/schemas/console";

export function AuditScreen() {
  const [filter, setFilter] = useState("all");
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
      <ConsoleData emptyDescription="No events have been recorded." emptyTitle="No audit events">
        {(snapshot) => <AuditContent events={snapshot.auditEvents} filter={filter} />}
      </ConsoleData>
    </PageShell>
  );
}

function AuditContent({ events, filter }: { events: AuditEvent[]; filter: string }) {
  const filtered = useMemo(
    () => (filter === "all" ? events : events.filter((event) => event.type === filter)),
    [events, filter],
  );
  return <AuditEventTimeline events={filtered} />;
}
