"use client";

import Link from "next/link";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { DataTable } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
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
  const columns: ColumnDef<RunRecord>[] = [
    {
      header: "Status",
      cell: ({ row }) => (
        <StatusBadge status={row.original.status || "unknown"} />
      ),
    },
    {
      header: "Title",
      cell: ({ row }) => (
        <div className="min-w-0">
          <p className="m-0 min-w-0 break-words font-semibold">
            {row.original.title || row.original.runId || row.original.id}
          </p>
          {row.original.resultSummary ? (
            <p className="m-0 mt-xs line-clamp-1 min-w-0 break-words text-body-sm text-ink-secondary">
              {row.original.resultSummary}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      header: "Executor profile",
      cell: ({ row }) => row.original.executorProfile || "n/a",
    },
    {
      header: "Project",
      cell: ({ row }) => row.original.projectName || row.original.projectId,
    },
    {
      header: "Run ID",
      cell: ({ row }) => row.original.runId || "n/a",
    },
    {
      header: "Execution",
      cell: ({ row }) => {
        const execution = executionModeDisplay(row.original.executionMode);
        return <Badge variant={execution.variant}>{execution.label}</Badge>;
      },
    },
    {
      header: "Machine",
      cell: ({ row }) =>
        row.original.hostLabel || row.original.machineId || "n/a",
    },
    {
      header: "Connector",
      cell: ({ row }) => row.original.connectorId || "n/a",
    },
    {
      header: "Started",
      cell: ({ row }) =>
        row.original.startedAt ? formatDateTime(row.original.startedAt) : "n/a",
    },
    {
      header: "Completed",
      cell: ({ row }) =>
        row.original.completedAt
          ? `${formatDateTime(row.original.completedAt)}${durationLabel(row.original) ? ` · ${durationLabel(row.original)}` : ""}`
          : "n/a",
    },
    {
      header: "Actions",
      cell: ({ row }) => (
        <Button asChild size="sm" variant="secondary">
          <Link href={`/console/runs/${row.original.id}`}>View details</Link>
        </Button>
      ),
    },
  ];
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
            <DataTable columns={columns} data={runs.data.runs} />
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
