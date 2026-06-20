"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { ConnectorStatusBadge } from "@/components/console/connector-status-badge";
import { CommandBlock } from "@/components/ui/code-block";
import { DataTable } from "@/components/ui/data-table";
import { formatDateTime } from "@/lib/format";
import type { Connector } from "@/schemas/connectors";
import type { Machine } from "@/schemas/machines";

type Row = Connector & { machine?: Machine };

const columns: ColumnDef<Row>[] = [
  { header: "Connector", accessorKey: "label" },
  {
    header: "Machine",
    cell: ({ row }) =>
      row.original.machine?.hostLabel ?? row.original.machineId,
  },
  {
    header: "OS/Arch",
    cell: ({ row }) =>
      `${row.original.machine?.os ?? "unknown"}/${row.original.machine?.arch ?? "unknown"}`,
  },
  { header: "Relay", accessorKey: "relayProfileId" },
  {
    header: "Status",
    cell: ({ row }) => <ConnectorStatusBadge connector={row.original} />,
  },
  {
    header: "Last seen",
    cell: ({ row }) => formatDateTime(row.original.lastSeen),
  },
];

export function MachineConnectorTable({
  connectors,
  machines,
}: {
  connectors: Connector[];
  machines: Machine[];
}) {
  const rows = connectors.map((connector) => ({
    ...connector,
    machine: machines.find((machine) => machine.id === connector.machineId),
  }));
  return (
    <div className="grid min-w-0 max-w-full gap-md">
      <DataTable
        columns={columns}
        data={rows}
        emptyDescription="Run connector login to bind a local machine."
        emptyTitle="No connectors"
      />
      <CommandBlock
        command="codencer connector login --gateway http://127.0.0.1:19090 --relay default --json"
        title="Connector login command"
      />
    </div>
  );
}
