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
  const audit = useAuditEvents({ limit: PAGE_SIZE, offset });
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
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="runs">Runs</SelectItem>
            <SelectItem value="connector">Connector</SelectItem>
            <SelectItem value="relay">Relay</SelectItem>
            <SelectItem value="auth">Auth/device</SelectItem>
            <SelectItem value="errors">Errors only</SelectItem>
            <SelectItem value="raw">Raw/debug</SelectItem>
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
              filter={filter}
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
  filter,
  groups,
}: {
  events: AuditEvent[];
  filter: string;
  groups: AuditEventGroup[];
}) {
  const filteredEvents = filterAuditEvents(events, filter);
  if (filter === "raw") {
    return <AuditEventTimeline events={compactReportReads(filteredEvents)} />;
  }
  const runEvents = filteredEvents.filter((event) => event.runHistoryId);
  const runIDs = new Set(runEvents.map((event) => event.runHistoryId));
  const visibleGroups =
    filter === "connector" || filter === "relay" || filter === "auth"
      ? []
      : groups.filter((group) => runIDs.has(group.runHistoryId));
  const otherEvents = filteredEvents.filter((event) => !event.runHistoryId);
  return (
    <div className="grid gap-md">
      {visibleGroups.length > 0 ? (
        <AuditGroups events={runEvents} groups={visibleGroups} />
      ) : null}
      {otherEvents.length > 0 ? (
        <div className="grid gap-sm">
          <p className="m-0 text-label font-semibold uppercase text-ink-secondary">
            Other workspace events
          </p>
          <AuditEventTimeline events={compactReportReads(otherEvents)} />
        </div>
      ) : null}
    </div>
  );
}

function AuditGroups({
  events,
  groups,
}: {
  events: AuditEvent[];
  groups: AuditEventGroup[];
}) {
  return (
    <div className="grid gap-sm">
      <p className="m-0 text-label font-semibold uppercase text-ink-secondary">
        Run lifecycle
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
              {summarizeEventTypes(eventsForGroup(events, group)).map(
                ({ count, type }) => (
                  <Badge key={type} variant="neutral">
                    {count > 1 ? `${type} x ${count}` : type}
                  </Badge>
                ),
              )}
            </div>
            <details className="min-w-0">
              <summary className="cursor-pointer text-body-sm font-semibold text-ink-primary">
                Lifecycle events
              </summary>
              <div className="mt-sm">
                <AuditEventTimeline
                  events={compactReportReads(eventsForGroup(events, group))}
                />
              </div>
            </details>
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

function filterAuditEvents(events: AuditEvent[], filter: string) {
  if (filter === "runs") {
    return events.filter((event) => Boolean(event.runHistoryId));
  }
  if (filter === "connector") {
    return events.filter((event) => event.type.startsWith("connector"));
  }
  if (filter === "relay") {
    return events.filter((event) => event.type.startsWith("relay"));
  }
  if (filter === "auth") {
    return events.filter(
      (event) =>
        event.type.startsWith("device") ||
        event.type.startsWith("oauth") ||
        event.type.includes("login"),
    );
  }
  if (filter === "errors") {
    return events.filter(
      (event) =>
        event.severity === "error" ||
        event.severity === "warning" ||
        event.type.includes("failed") ||
        event.type.includes("blocker"),
    );
  }
  return events;
}

function eventsForGroup(events: AuditEvent[], group: AuditEventGroup) {
  return events.filter((event) => event.runHistoryId === group.runHistoryId);
}

function summarizeEventTypes(events: AuditEvent[]) {
  const counts = new Map<string, number>();
  for (const event of events) {
    counts.set(event.type, (counts.get(event.type) ?? 0) + 1);
  }
  return Array.from(counts, ([type, count]) => ({ count, type }));
}

function compactReportReads(events: AuditEvent[]) {
  const out: AuditEvent[] = [];
  let reportReads: AuditEvent[] = [];
  for (const event of events) {
    if (event.type === "report_read") {
      reportReads.push(event);
      continue;
    }
    flushReportReads();
    out.push(event);
  }
  flushReportReads();
  return out;

  function flushReportReads() {
    if (reportReads.length === 0) return;
    if (reportReads.length === 1) {
      out.push(reportReads[0]);
    } else {
      const first = reportReads[0];
      out.push({
        ...first,
        id: `${first.id}:collapsed`,
        summary: `report_read x ${reportReads.length}`,
      });
    }
    reportReads = [];
  }
}
