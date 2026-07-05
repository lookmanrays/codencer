"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState, type ReactNode } from "react";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { RunResultPanel } from "@/components/console/run-result-panel";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Badge, StatusBadge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { DataTable } from "@/components/ui/data-table";
import { Field } from "@/components/ui/field";
import { LoadingPanel } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { formatDateTime } from "@/lib/format";
import { isDemoMode } from "@/api/config";
import {
  useRespondToHumanInterrupt,
  useRun,
  useRunEvents,
} from "@/api/run-history";
import type { AuditEvent } from "@/schemas/audit";
import type { RunRecord } from "@/schemas/run-history";

export function RunDetailScreen({ id }: { id: string }) {
  const run = useRun(id);
  const events = useRunEvents(id);
  const runRecord = run.data?.run;
  const attempts = useMemo(
    () => (runRecord ? attemptsFromRun(runRecord) : []),
    [runRecord],
  );
  const artifacts = useMemo(
    () => (runRecord ? artifactsFromRun(runRecord) : []),
    [runRecord],
  );
  const attemptColumns = useMemo<ColumnDef<AttemptRow>[]>(
    () => [
      { header: "Attempt", accessorKey: "attempt" },
      { header: "Adapter", accessorKey: "adapter" },
      { header: "Executor profile", accessorKey: "executorProfile" },
      { header: "State", accessorKey: "state" },
      { header: "Started", accessorKey: "started" },
      { header: "Completed", accessorKey: "completed" },
      { header: "Result", accessorKey: "result" },
      { header: "Actions", accessorKey: "action" },
    ],
    [],
  );
  const artifactColumns = useMemo<ColumnDef<ArtifactRow>[]>(
    () => [
      { header: "Name", accessorKey: "name" },
      { header: "Type", accessorKey: "type" },
      { header: "Size", accessorKey: "size" },
      { header: "Created", accessorKey: "created" },
      { header: "Action", accessorKey: "action" },
    ],
    [],
  );
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
              <CompactMetadataGrid
                items={[
                  {
                    label: "Status",
                    value: run.data.run.status || "unknown",
                  },
                  { label: "Run ID", value: run.data.run.runId || "n/a" },
                  {
                    label: "Project",
                    value: run.data.run.projectName || run.data.run.projectId,
                  },
                  {
                    label: "Adapter",
                    value: run.data.run.executorAdapter || "n/a",
                  },
                  {
                    label: "Executor profile",
                    value: run.data.run.executorProfile || "n/a",
                  },
                  {
                    label: "Execution",
                    value: (
                      <Badge
                        variant={
                          executionModeDisplay(run.data.run.executionMode)
                            .variant
                        }
                      >
                        {executionModeDisplay(run.data.run.executionMode).label}
                      </Badge>
                    ),
                  },
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
                    label: "Scope",
                    value: scopeLabel(run.data.run.scope),
                  },
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
                      ? `${formatDateTime(run.data.run.completedAt)}${durationLabel(run.data.run) ? ` · ${durationLabel(run.data.run)}` : ""}`
                      : "n/a",
                  },
                ]}
              />
            </CardContent>
          </Card>
          <RunResultPanel mode="detail" run={run.data.run} />
          <HumanInterruptResponsePanel
            events={events.data?.events ?? []}
            run={run.data.run}
          />
          <Card>
            <CardHeader>
              <CardTitle>Attempts</CardTitle>
            </CardHeader>
            <CardContent>
              <DataTable
                columns={attemptColumns}
                data={attempts}
                emptyDescription="No attempt metadata is available yet."
                emptyTitle="No attempts"
              />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Safe artifacts and logs</CardTitle>
            </CardHeader>
            <CardContent>
              <DataTable
                columns={artifactColumns}
                data={artifacts}
                emptyDescription="No safe artifact or log summary is available yet."
                emptyTitle="No artifacts"
              />
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
                  <AuditEventTimeline
                    events={compactReportReads(events.data.events)}
                  />
                )
              ) : null}
            </CardContent>
          </Card>
        </div>
      ) : null}
    </PageShell>
  );
}

type MetadataItem = {
  label: string;
  value: ReactNode;
};

type AttemptRow = {
  action: string;
  adapter: string;
  attempt: string;
  completed: string;
  executorProfile: string;
  result: string;
  started: string;
  state: string;
};

type ArtifactRow = {
  action: string;
  created: string;
  name: string;
  size: string;
  type: string;
};

function CompactMetadataGrid({ items }: { items: MetadataItem[] }) {
  return (
    <dl className="grid min-w-0 gap-sm sm:grid-cols-2 xl:grid-cols-3">
      {items.map((item) => (
        <div
          className="min-w-0 rounded-[var(--radius-card)] border border-border bg-paper px-md py-sm"
          key={item.label}
        >
          <dt className="min-w-0 break-words font-mono text-mono uppercase tracking-[0.12em] text-ink-muted">
            {item.label}
          </dt>
          <dd className="m-0 mt-xs min-w-0 break-words text-body-sm text-ink-primary">
            {item.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function attemptsFromRun(run: RunRecord): AttemptRow[] {
  const report = asRecord(run.report);
  const tasks = taskRecords(report);
  if (tasks.length === 0) {
    return [
      {
        action: "View result",
        adapter: run.executorAdapter || "n/a",
        attempt: run.stepId || run.runId || run.id,
        completed: run.completedAt ? formatDateTime(run.completedAt) : "n/a",
        executorProfile: run.executorProfile || "n/a",
        result: run.resultSummary || run.status || "n/a",
        started: run.startedAt ? formatDateTime(run.startedAt) : "n/a",
        state: run.status || "unknown",
      },
    ];
  }
  return tasks.map((task, index) => ({
    action: "View result",
    adapter: stringValue(task.adapter) || run.executorAdapter || "n/a",
    attempt:
      stringValue(task.attempt) ||
      stringValue(task.task_id) ||
      stringValue(task.id) ||
      stringValue(task.step_id) ||
      `attempt-${index + 1}`,
    completed: formatMaybeDate(
      stringValue(task.completed_at) || stringValue(task.updated_at),
    ),
    executorProfile:
      stringValue(task.profile) ||
      stringValue(task.executor_profile) ||
      stringValue(task.adapter_profile) ||
      run.executorProfile ||
      "n/a",
    result:
      safeText(
        stringValue(task.summary) ||
          nestedString(task, "evidence", "result", "summary") ||
          nestedString(task, "evidence", "result", "raw_output") ||
          nestedString(task, "blocker", "message"),
      ) || "n/a",
    started: formatMaybeDate(
      stringValue(task.started_at) || stringValue(task.created_at),
    ),
    state: stringValue(task.status) || stringValue(task.state) || "unknown",
  }));
}

function artifactsFromRun(run: RunRecord): ArtifactRow[] {
  const artifacts: ArtifactRow[] = [];
  const seen = new Set<string>();
  collectArtifacts(asRecord(run.report), artifacts, seen);
  return artifacts;
}

function taskRecords(report: Record<string, unknown>) {
  const out: Record<string, unknown>[] = [];
  const task = asRecord(report.task);
  if (Object.keys(task).length > 0) out.push(task);
  if (Array.isArray(report.tasks)) {
    for (const item of report.tasks) {
      const record = asRecord(item);
      if (Object.keys(record).length > 0) out.push(record);
    }
  }
  return out;
}

function collectArtifacts(
  value: unknown,
  out: ArtifactRow[],
  seen: Set<string>,
) {
  if (!value || typeof value !== "object") return;
  if (Array.isArray(value)) {
    value.forEach((item) => collectArtifacts(item, out, seen));
    return;
  }
  const record = value as Record<string, unknown>;
  if (Array.isArray(record.artifacts)) {
    record.artifacts.forEach((artifact) => {
      const item = asRecord(artifact);
      const name = safeText(stringValue(item.name) || stringValue(item.id));
      if (!name) return;
      const type =
        safeText(
          stringValue(item.type) ||
            stringValue(item.mime_type) ||
            stringValue(item.kind),
        ) || "artifact";
      const size = formatSize(item.size);
      const dedupeKey =
        stringValue(item.artifact_id) ||
        stringValue(item.id) ||
        stringValue(item.hash) ||
        `${name}:${type}:${size}`;
      if (seen.has(dedupeKey)) return;
      seen.add(dedupeKey);
      out.push({
        action: "Reference only",
        created: formatMaybeDate(
          stringValue(item.created_at) || stringValue(item.created),
        ),
        name,
        size,
        type,
      });
    });
  }
  Object.entries(record).forEach(([key, item]) => {
    if (key === "path" || key === "report_path" || key === "logs_ref") return;
    collectArtifacts(item, out, seen);
  });
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function nestedString(value: unknown, ...keys: string[]) {
  let current: unknown = value;
  for (const key of keys) {
    current = asRecord(current)[key];
  }
  return stringValue(current);
}

function safeText(value: string) {
  return value
    .replace(/\/Users\/[^\s"']+/g, "redacted-local-path")
    .replace(/\/home\/[^\s"']+/g, "redacted-local-path")
    .replace(/\/tmp\/[^\s"']+/g, "redacted-local-path")
    .replace(/\/var\/folders\/[^\s"']+/g, "redacted-local-path")
    .replace(/token=[^\s"']+/gi, "token=redacted")
    .slice(0, 400);
}

function formatMaybeDate(value: string) {
  return value ? formatDateTime(value) : "n/a";
}

function formatSize(value: unknown) {
  return typeof value === "number" && Number.isFinite(value)
    ? `${value} bytes`
    : "n/a";
}

function executionModeDisplay(mode: "real" | "simulation" | "unknown"): {
  label: string;
  variant: "success" | "warning" | "neutral";
} {
  if (mode === "real") {
    return { label: "Real executor", variant: "success" };
  }
  if (mode === "simulation") {
    return { label: "Simulation", variant: "warning" };
  }
  return { label: "Unknown", variant: "neutral" };
}

function durationLabel(run: RunRecord) {
  if (!run.startedAt || !run.completedAt) return "";
  const started = Date.parse(run.startedAt);
  const completed = Date.parse(run.completedAt);
  if (
    !Number.isFinite(started) ||
    !Number.isFinite(completed) ||
    completed < started
  ) {
    return "";
  }
  return `${Math.round((completed - started) / 1000)}s`;
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

function HumanInterruptResponsePanel({
  events,
  run,
}: {
  events: AuditEvent[];
  run: RunRecord;
}) {
  const state = useMemo(() => humanInterruptState(run, events), [events, run]);
  const respond = useRespondToHumanInterrupt();
  const [responseType, setResponseType] = useState("answer");
  const [followUp, setFollowUp] = useState("resume");
  const [response, setResponse] = useState("");
  if (!state.visible) return null;
  return (
    <Card>
      <CardHeader>
        <CardTitle>Human interrupt response</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid min-w-0 gap-md">
          {state.responseRecorded ? (
            <Alert title="Response recorded" tone="success">
              {state.operatorResponse ||
                "An operator response has been recorded in the audit trail."}
            </Alert>
          ) : (
            <Alert title="Waiting for human action" tone="warning">
              {state.prompt ||
                "The run is waiting for an explicit operator response."}
            </Alert>
          )}
          {state.responseRecorded ? null : (
            <form
              className="grid min-w-0 gap-md"
              onSubmit={async (event) => {
                event.preventDefault();
                if (!response.trim()) return;
                await respond.mutateAsync({
                  followUp: followUp === "none" ? undefined : followUp,
                  response: response.trim(),
                  responseType,
                  runHistoryId: run.id,
                });
              }}
            >
              {respond.error ? (
                <Alert title="Response was not recorded" tone="danger">
                  {respond.error.message}
                </Alert>
              ) : null}
              {respond.data ? (
                <Alert title="Response recorded" tone="success">
                  {respond.data.response?.operatorResponse ||
                    "The response was recorded and linked to this run."}
                  {respond.data.followUp === "resume" ||
                  respond.data.followUp === "cancel" ||
                  respond.data.followUp === "start_new_task" ? (
                    <span className="mt-xs block text-xs text-muted">
                      {respond.data.followUp === "cancel"
                        ? "Cancel follow-up requested; check the audit timeline for the cancelled or blocked outcome."
                        : respond.data.followUp === "start_new_task"
                          ? "Start-new-task follow-up requested; check the audit timeline and Runs page for the submitted or blocked outcome."
                          : "Resume follow-up requested; check the audit timeline for the resumed or blocked outcome."}
                    </span>
                  ) : null}
                </Alert>
              ) : null}
              <div className="grid min-w-0 gap-md md:grid-cols-2">
                <Field id="interrupt-response-type" label="Response type">
                  <Select onValueChange={setResponseType} value={responseType}>
                    <SelectTrigger
                      aria-label="Response type"
                      id="interrupt-response-type"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="answer">Answer</SelectItem>
                      <SelectItem value="approve">Approve</SelectItem>
                      <SelectItem value="deny">Deny</SelectItem>
                      <SelectItem value="confirm">Confirm</SelectItem>
                      <SelectItem value="retry">Retry</SelectItem>
                      <SelectItem value="decision">Decision</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
                <Field
                  description="Resume records intent only; true resume still returns an explicit capability result."
                  id="interrupt-follow-up"
                  label="Follow-up"
                >
                  <Select onValueChange={setFollowUp} value={followUp}>
                    <SelectTrigger
                      aria-label="Follow-up"
                      id="interrupt-follow-up"
                    >
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="resume">Resume</SelectItem>
                      <SelectItem value="cancel">Cancel</SelectItem>
                      <SelectItem value="start_new_task">
                        Start new task
                      </SelectItem>
                      <SelectItem value="none">None</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
              </div>
              <Field
                description="This response is sanitized and stored as Gateway audit metadata."
                id="operator-response"
                label="Operator response"
              >
                <Textarea
                  id="operator-response"
                  onChange={(event) => setResponse(event.target.value)}
                  placeholder="Answer the question or record the approval decision."
                  value={response}
                />
              </Field>
              <div className="flex min-w-0 flex-wrap gap-sm">
                <Button
                  disabled={respond.isPending || !response.trim()}
                  type="submit"
                >
                  Record response
                </Button>
              </div>
            </form>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

function humanInterruptState(run: RunRecord, events: AuditEvent[]) {
  const created = events.find(
    (event) => event.type === "human_interrupt_created",
  );
  const responded = events.find(
    (event) => event.type === "human_interrupt_responded",
  );
  const blockedStatus = ["blocked", "waiting_for_human", "question"].includes(
    (run.status || "").toLowerCase(),
  );
  const visible = Boolean(created || responded || blockedStatus);
  const prompt =
    stringMetadata(created, "prompt") ||
    stringMetadata(created, "requested_action") ||
    created?.summary;
  return {
    operatorResponse: stringMetadata(responded, "operator_response"),
    prompt,
    responseRecorded: Boolean(responded),
    visible,
  };
}

function stringMetadata(event: AuditEvent | undefined, key: string) {
  const value = event?.metadata?.[key];
  return typeof value === "string" ? value : "";
}

function scopeLabel(scope?: string) {
  if (scope === "gateway_submitted") return "Gateway-submitted";
  if (scope === "synced") return "Synced";
  if (scope === "local") return "Local";
  return scope || "n/a";
}
