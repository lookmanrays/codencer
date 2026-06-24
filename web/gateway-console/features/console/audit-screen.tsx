"use client";

import Link from "next/link";
import { useState } from "react";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { formatDateTime } from "@/lib/format";
import type { AuditEvent, AuditEventGroup } from "@/schemas/audit";

const PAGE_SIZE = 50;

export function AuditScreen() {
  const [filter, setFilter] = useState("all");
  const [offset, setOffset] = useState(0);
  const eventType = filter === "all" ? undefined : filter;
  const audit = useAuditEvents({ limit: PAGE_SIZE, offset, type: eventType });
  return (
    <PageShell
      actions={
        <Select
          onValueChange={(value) => {
            setFilter(value);
            setOffset(0);
          }}
          value={filter}
        >
          <SelectTrigger className="w-[220px]">
            <SelectValue placeholder="Event type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All events</SelectItem>
            <SelectItem value="relay.add">Relay changes</SelectItem>
            <SelectItem value="connector.login">Connector login</SelectItem>
            <SelectItem value="task_submitted">Task submitted</SelectItem>
            <SelectItem value="run_completed">Run completed</SelectItem>
            <SelectItem value="report_read">Report read</SelectItem>
            <SelectItem value="blocker">Blockers</SelectItem>
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
        <Alert title="Audit API unavailable" tone="danger">
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
            <AuditContent
              events={audit.data.auditEvents}
              groups={audit.data.groups}
            />
          )}
          {audit.data.pagination ? (
            <div className="flex min-w-0 flex-wrap items-center justify-between gap-sm text-body-sm text-ink-secondary">
              <span>
                Showing {audit.data.auditEvents.length} events from offset{" "}
                {audit.data.pagination.offset}
              </span>
              <div className="flex gap-sm">
                <Button
                  disabled={offset === 0 || audit.isFetching}
                  onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                  size="sm"
                  type="button"
                  variant="quiet"
                >
                  Previous
                </Button>
                <Button
                  disabled={!audit.data.pagination.has_more || audit.isFetching}
                  onClick={() =>
                    setOffset(audit.data?.pagination.next_offset ?? offset)
                  }
                  size="sm"
                  type="button"
                  variant="quiet"
                >
                  Next
                </Button>
              </div>
            </div>
          ) : null}
        </div>
      ) : null}
    </PageShell>
  );
}

function AuditContent({
  events,
  groups,
}: {
  events: AuditEvent[];
  groups: AuditEventGroup[];
}) {
  return (
    <div className="grid gap-md">
      {groups.length > 0 ? <AuditGroups groups={groups} /> : null}
      <AuditEventTimeline events={events} />
    </div>
  );
}

function AuditGroups({ groups }: { groups: AuditEventGroup[] }) {
  return (
    <div className="grid gap-sm">
      <p className="m-0 text-label font-semibold uppercase text-ink-secondary">
        Grouped lifecycle
      </p>
      {groups.map((group) => (
        <Card key={group.id}>
          <CardHeader className="flex min-w-0 flex-wrap items-start justify-between gap-sm">
            <div className="min-w-0">
              <CardTitle>{group.runId || group.runHistoryId}</CardTitle>
              <p className="m-0 mt-xs min-w-0 break-words text-body-sm text-ink-secondary">
                {group.summary}
              </p>
            </div>
            <Badge variant="neutral">{group.eventCount} events</Badge>
          </CardHeader>
          <CardContent className="grid gap-sm">
            <div className="flex min-w-0 flex-wrap gap-xs">
              {group.types.map((type) => (
                <Badge key={type} variant="neutral">
                  {type}
                </Badge>
              ))}
            </div>
            <p className="m-0 text-body-sm text-ink-secondary">
              {formatDateTime(group.firstEventAt)} -{" "}
              {formatDateTime(group.lastEventAt)}
            </p>
            <div>
              <Button asChild size="sm" variant="secondary">
                <Link href={`/console/runs/${group.runHistoryId}`}>
                  View run
                </Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
