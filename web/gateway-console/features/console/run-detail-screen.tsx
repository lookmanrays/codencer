"use client";

import { useMemo, useState } from "react";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { RunResultPanel } from "@/components/console/run-result-panel";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/ui/badge";
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
                      ? formatDateTime(run.data.run.completedAt)
                      : "n/a",
                  },
                ]}
              />
            </CardContent>
          </Card>
          <RunResultPanel run={run.data.run} />
          <HumanInterruptResponsePanel
            events={events.data?.events ?? []}
            run={run.data.run}
          />
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
                  {respond.data.followUp === "resume" ? (
                    <span className="mt-xs block text-xs text-muted">
                      Resume follow-up requested; check the audit timeline for
                      the resumed or blocked outcome.
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
