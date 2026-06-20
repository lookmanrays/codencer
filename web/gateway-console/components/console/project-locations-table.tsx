"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Badge, StatusBadge } from "@/components/ui/badge";
import { DataTable } from "@/components/ui/data-table";
import type { Project, ProjectLocation } from "@/schemas/console";

type Row = ProjectLocation & { projectName: string };

const columns: ColumnDef<Row>[] = [
  { header: "Project", accessorKey: "projectName" },
  { header: "Relay", accessorKey: "relayProfileId" },
  { header: "Machine", accessorKey: "hostLabel" },
  { header: "Repo", cell: ({ row }) => `${row.original.repoLabel} · ${row.original.repoHash}` },
  { header: "Status", cell: ({ row }) => <StatusBadge status={row.original.status} /> },
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

export function ProjectLocationsTable({ projects }: { projects: Project[] }) {
  const rows = projects.flatMap((project) =>
    project.locations.map((location) => ({ ...location, projectName: project.name })),
  );
  return (
    <DataTable
      columns={columns}
      data={rows}
      emptyDescription="Share a project through the connector to advertise safe location metadata."
      emptyTitle="No project locations"
    />
  );
}
