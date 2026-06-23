"use client";

import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { RunResultPanel } from "@/components/console/run-result-panel";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/ui/badge";
import { formatDateTime } from "@/lib/format";
import { isDemoMode } from "@/api/config";
import { useRun, useRunEvents } from "@/api/run-history";

export function RunDetailScreen({ id }: { id: string }) {
  const run = useRun(id);
  const events = useRunEvents(id);
  return (
    <PageShell
      breadcrumbs={[
        { label: "Console", href: "/console" },
        { label: "Runs", href: "/console/runs" },
        { label: id },
      ]}
      description="Inspect one Gateway-observed run without exposing local paths or secrets."
      kicker="Run detail"
      title="Full run"
    >
      {run.isLoading ? <LoadingPanel /> : null}
      {run.error ? (
        <Alert title="Run unavailable" tone="danger">
          {run.error.message}
        </Alert>
      ) : null}
      {run.data ? (
        <div className="grid min-w-0 gap-lg">
          {isDemoMode() ? <DemoModeNotice /> : null}
          <Card>
            <CardHeader className="flex min-w-0 flex-wrap items-start justify-between gap-md">
              <div className="min-w-0">
                <CardTitle>
                  {run.data.run.title || run.data.run.runId}
                </CardTitle>
                {run.data.run.goal ? (
                  <p className="m-0 mt-xs min-w-0 break-words text-body-sm text-ink-secondary">
                    {run.data.run.goal}
                  </p>
                ) : null}
              </div>
              <StatusBadge status={run.data.run.status || "unknown"} />
            </CardHeader>
            <CardContent>
              <KeyValueList
                items={[
                  {
                    label: "Project",
                    value: run.data.run.projectName || run.data.run.projectId,
                  },
                  {
                    label: "Executor",
                    value: run.data.run.executorProfile || "n/a",
                  },
                  { label: "Run ID", value: run.data.run.runId || "n/a" },
                  { label: "Step ID", value: run.data.run.stepId || "n/a" },
                  {
                    label: "Relay",
                    value: run.data.run.relayProfileId || "n/a",
                  },
                  {
                    label: "Connector",
                    value: run.data.run.connectorId || "n/a",
                  },
                  {
                    label: "Machine",
                    value:
                      run.data.run.hostLabel || run.data.run.machineId || "n/a",
                  },
                  { label: "Run mode", value: run.data.run.mode || "task" },
                  {
                    label: "Report",
                    value: run.data.run.reportStatus || "n/a",
                  },
                  {
                    label: "Started",
                    value: run.data.run.startedAt
                      ? formatDateTime(run.data.run.startedAt)
                      : "n/a",
                  },
                  {
                    label: "Completed",
                    value: run.data.run.completedAt
                      ? formatDateTime(run.data.run.completedAt)
                      : "n/a",
                  },
                ]}
              />
            </CardContent>
          </Card>
          <RunResultPanel run={run.data.run} />
          <Card>
            <CardHeader>
              <CardTitle>Safe artifacts and logs</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="m-0 min-w-0 whitespace-pre-wrap break-words text-body-sm text-ink-secondary">
                {run.data.run.resultDetails ||
                  run.data.run.resultSummary ||
                  "No safe artifact or log summary is available yet."}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Event timeline</CardTitle>
            </CardHeader>
            <CardContent>
              {events.isLoading ? <LoadingPanel /> : null}
              {events.error ? (
                <Alert title="Run events unavailable" tone="warning">
                  {events.error.message}
                </Alert>
              ) : null}
              {events.data ? (
                events.data.events.length === 0 ? (
                  <p className="m-0 text-body-sm text-ink-secondary">
                    No audit events are linked to this run yet.
                  </p>
                ) : (
                  <AuditEventTimeline events={events.data.events} />
                )
              ) : null}
            </CardContent>
          </Card>
        </div>
      ) : null}
    </PageShell>
  );
}
