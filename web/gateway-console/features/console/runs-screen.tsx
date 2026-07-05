"use client";

import Link from "next/link";
import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import { ExternalLink } from "lucide-react";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { DataTable } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { LoadingPanel } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/ui/badge";
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
      header: "Run",
      cell: ({ row }) => (
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-sm">
            <StatusBadge status={row.original.status || "unknown"} />
            <span
              className="min-w-0 truncate font-semibold"
              title={
                row.original.title || row.original.runId || row.original.id
              }
            >
              {row.original.title || row.original.runId || row.original.id}
            </span>
          </div>
          <p
            className="m-0 mt-xs truncate font-mono text-mono text-ink-muted"
            title={row.original.runId || row.original.id}
          >
            {shortID(row.original.runId || row.original.id)}
          </p>
          {row.original.resultSummary ? (
            <p
              className="m-0 mt-xs line-clamp-1 min-w-0 break-words text-body-sm text-ink-secondary"
              title={row.original.resultSummary}
            >
              {row.original.resultSummary}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      header: "Executor",
      cell: ({ row }) => (
        <div className="min-w-0">
          <p
            className="m-0 truncate font-semibold"
            title={row.original.executorProfile || "n/a"}
          >
            {row.original.executorProfile || "n/a"}
          </p>
          <p className="m-0 mt-xs flex min-w-0 items-center gap-xs font-mono text-mono text-ink-muted">
            <span>{row.original.executorAdapter || "n/a"}</span>
            <span aria-hidden="true">·</span>
            <span className="inline-flex items-center gap-[5px]">
              <span className="h-1.5 w-1.5 rounded-full bg-success" />
              {executionModeLabel(row.original.executionMode)}
            </span>
          </p>
        </div>
      ),
    },
    {
      header: "Project / Machine",
      cell: ({ row }) => (
        <div className="min-w-0">
          <p
            className="m-0 truncate"
            title={row.original.projectName || row.original.projectId}
          >
            {row.original.projectName || row.original.projectId}
          </p>
          <p
            className="m-0 mt-xs truncate font-mono text-mono text-ink-muted"
            title={row.original.machineId || row.original.hostLabel || "n/a"}
          >
            {row.original.hostLabel || shortID(row.original.machineId) || "n/a"}
          </p>
        </div>
      ),
    },
    {
      header: "Started / Duration",
      cell: ({ row }) => (
        <div className="min-w-0">
          <p
            className="m-0 truncate"
            title={
              row.original.startedAt
                ? formatDateTime(row.original.startedAt)
                : "n/a"
            }
          >
            {compactDate(row.original.startedAt)}
          </p>
          <p className="m-0 mt-xs font-mono text-mono text-ink-muted">
            {durationLabel(row.original) || "n/a"}
          </p>
        </div>
      ),
    },
    {
      header: "",
      id: "actions",
      cell: ({ row }) => (
        <Button
          asChild
          aria-label={`Open ${row.original.title || row.original.runId}`}
          size="icon"
          variant="quiet"
        >
          <Link href={`/console/runs/${row.original.id}`}>
            <ExternalLink aria-hidden="true" className="h-4 w-4" />
          </Link>
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
            <DataTable
              columns={columns}
              data={runs.data.runs}
              getRowHref={(run) => `/console/runs/${run.id}`}
            />
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

function executionModeLabel(mode: "real" | "simulation" | "unknown") {
  if (mode === "real") {
    return "real";
  }
  if (mode === "simulation") {
    return "simulation";
  }
  return "unknown";
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

function compactDate(value?: string) {
  if (!value) return "n/a";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "n/a";
  return new Intl.DateTimeFormat("en", {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "short",
  }).format(date);
}

function shortID(value?: string) {
  if (!value) return "";
  return value.length > 18
    ? `${value.slice(0, 10)}...${value.slice(-5)}`
    : value;
}
