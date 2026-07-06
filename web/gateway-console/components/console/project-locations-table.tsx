"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Play } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";
import { Badge, StatusBadge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable } from "@/components/ui/data-table";
import type { Project, ProjectLocation } from "@/schemas/projects";

export type ProjectLocationRow = ProjectLocation & { projectName: string };

const baseColumns: ColumnDef<ProjectLocationRow>[] = [
  {
    header: "Project",
    cell: ({ row }) => (
      <div className="min-w-0">
        <p
          className="m-0 truncate font-semibold"
          title={row.original.projectName}
        >
          {row.original.projectName}
        </p>
        <p
          className="m-0 truncate font-mono text-mono text-ink-muted"
          title={row.original.projectId}
        >
          {row.original.projectId}
        </p>
      </div>
    ),
  },
  { header: "Relay", accessorKey: "relayProfileId" },
  {
    header: "Machine",
    cell: ({ row }) => (
      <span title={row.original.machineId}>
        {row.original.hostLabel || row.original.machineId || "n/a"}
      </span>
    ),
  },
  {
    header: "Repo",
    cell: ({ row }) => (
      <span title={`${row.original.repoLabel} ${row.original.repoHash}`}>
        {row.original.repoLabel} · {row.original.repoHash}
      </span>
    ),
  },
  {
    header: "Status",
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  },
  {
    header: "Ambiguity",
    cell: ({ row }) =>
      row.original.ambiguity === "none" ? (
        <Badge>none</Badge>
      ) : (
        <Badge variant="warning">{row.original.ambiguity}</Badge>
      ),
  },
];

export function projectLocationRows(projects: Project[]): ProjectLocationRow[] {
  return projects.flatMap((project) =>
    project.locations.map((location) => ({
      ...location,
      projectName: project.name,
    })),
  );
}

export function ProjectLocationsTable({
  getRunHref,
  onRun,
  rows,
  selectedLocationId,
}: {
  getRunHref?: (row: ProjectLocationRow) => string;
  onRun?: (row: ProjectLocationRow) => void;
  rows: ProjectLocationRow[];
  selectedLocationId?: string;
}) {
  const columns = useMemo<ColumnDef<ProjectLocationRow>[]>(() => {
    if (!onRun && !getRunHref) return baseColumns;
    return [
      ...baseColumns,
      {
        header: "Action",
        cell: ({ row }) => {
          const label = `Run task on ${row.original.projectName} ${
            row.original.hostLabel || row.original.machineId
          }`;
          const disabled = row.original.status !== "online";
          const variant =
            row.original.id === selectedLocationId ? "primary" : "secondary";
          if (getRunHref && !disabled) {
            return (
              <Button asChild size="sm" variant={variant}>
                <Link aria-label={label} href={getRunHref(row.original)}>
                  <Play aria-hidden="true" className="h-4 w-4" />
                  Run
                </Link>
              </Button>
            );
          }
          return (
            <Button
              aria-label={label}
              disabled={disabled}
              onClick={() => onRun?.(row.original)}
              size="sm"
              type="button"
              variant={variant}
            >
              <Play aria-hidden="true" className="h-4 w-4" />
              Run
            </Button>
          );
        },
      },
    ];
  }, [getRunHref, onRun, selectedLocationId]);
  return (
    <DataTable
      columns={columns}
      data={rows}
      density="compact"
      emptyDescription="Share a project through the connector to advertise safe location metadata."
      emptyTitle="No project locations"
      getRowHref={getRunHref}
      minWidth="680px"
    />
  );
}

export function projectRunHref(row: ProjectLocationRow) {
  const params = new URLSearchParams({
    machine_id: row.machineId,
    project_id: row.projectId,
    relay_profile_id: row.relayProfileId,
  });
  if (row.hostLabel) {
    params.set("host_label", row.hostLabel);
  }
  return `/console/projects/run?${params.toString()}`;
}
