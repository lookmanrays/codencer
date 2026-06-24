"use client";

import Link from "next/link";
import { FileText } from "lucide-react";
import { useState } from "react";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";
import { Badge, StatusBadge } from "@/components/ui/badge";
import { formatDateTime } from "@/lib/format";
import { isDemoMode } from "@/api/config";
import { useRuns } from "@/api/run-history";
import type { RunRecord } from "@/schemas/run-history";

const PAGE_SIZE = 25;

export function RunsScreen() {
  const [offset, setOffset] = useState(0);
  const runs = useRuns({ limit: PAGE_SIZE, offset });
  return (
    <PageShell
      breadcrumbs={[{ label: "Console", href: "/console" }, { label: "Runs" }]}
      description="Review Gateway-observed execution history and open full run results."
      kicker="Runs"
      title="Run history"
    >
      {runs.isLoading ? <LoadingPanel /> : null}
      {runs.error ? (
        <Alert title="Run history unavailable" tone="danger">
          {runs.error.message}
        </Alert>
      ) : null}
      {runs.data ? (
        <div className="grid min-w-0 gap-md">
          {isDemoMode() ? <DemoModeNotice /> : null}
          {runs.data.runs.length === 0 ? (
            <EmptyState
              description="Submit a project task through Gateway Console to create a history record."
              title="No runs yet"
            />
          ) : (
            <div className="grid min-w-0 gap-md">
              {runs.data.runs.map((run) => (
                <RunHistoryCard key={run.id} run={run} />
              ))}
            </div>
          )}
          <div className="flex min-w-0 flex-wrap items-center justify-between gap-sm text-body-sm text-ink-secondary">
            <span>
              Showing {runs.data.runs.length} runs from offset{" "}
              {runs.data.pagination.offset}
            </span>
            <div className="flex gap-sm">
              <Button
                disabled={offset === 0 || runs.isFetching}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                size="sm"
                type="button"
                variant="quiet"
              >
                Previous
              </Button>
              <Button
                disabled={!runs.data.pagination.has_more || runs.isFetching}
                onClick={() =>
                  setOffset(runs.data?.pagination.next_offset ?? offset)
                }
                size="sm"
                type="button"
                variant="quiet"
              >
                Next
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </PageShell>
  );
}

function RunHistoryCard({ run }: { run: RunRecord }) {
  const execution = executionModeDisplay(run.executionMode);
  return (
    <Card>
      <CardHeader className="flex min-w-0 flex-wrap items-start justify-between gap-md">
        <div className="min-w-0">
          <CardTitle>{run.title || run.runId || run.id}</CardTitle>
          {run.goal ? (
            <p className="m-0 mt-xs min-w-0 break-words text-body-sm text-ink-secondary">
              {run.goal}
            </p>
          ) : null}
        </div>
        <StatusBadge status={run.status || "unknown"} />
      </CardHeader>
      <CardContent className="grid gap-md">
        <KeyValueList
          items={[
            { label: "Project", value: run.projectName || run.projectId },
            { label: "Executor", value: run.executorProfile || "n/a" },
            {
              label: "Execution",
              value: (
                <Badge variant={execution.variant}>{execution.label}</Badge>
              ),
            },
            { label: "Run ID", value: run.runId || "n/a" },
            { label: "Scope", value: scopeLabel(run.scope) },
            {
              label: "Machine",
              value: run.hostLabel || run.machineId || "n/a",
            },
            { label: "Connector", value: run.connectorId || "n/a" },
            {
              label: "Started",
              value: run.startedAt ? formatDateTime(run.startedAt) : "n/a",
            },
            {
              label: "Completed",
              value: run.completedAt ? formatDateTime(run.completedAt) : "n/a",
            },
          ]}
        />
        <div className="min-w-0">
          <p className="m-0 font-semibold">Result</p>
          <p className="m-0 mt-xs min-w-0 whitespace-pre-wrap break-words text-body-sm text-ink-secondary">
            {run.resultSummary ||
              run.resultDetails ||
              "No result text has been recorded for this run yet."}
          </p>
        </div>
        <div>
          <Button asChild size="sm" variant="secondary">
            <Link href={`/console/runs/${run.id}`}>
              <FileText aria-hidden="true" className="h-4 w-4" />
              View details
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function scopeLabel(scope?: string) {
  if (scope === "gateway_submitted") return "Gateway-submitted";
  if (scope === "synced") return "Synced";
  if (scope === "local") return "Local";
  return scope || "n/a";
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
