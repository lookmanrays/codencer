"use client";

import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/lib/cn";

export function DataTable<TData>({
  columns,
  data,
  emptyDescription = "No records match this view.",
  emptyTitle = "No records",
}: {
  columns: ColumnDef<TData>[];
  data: TData[];
  emptyTitle?: string;
  emptyDescription?: string;
}) {
  const table = useReactTable({ columns, data, getCoreRowModel: getCoreRowModel() });
  if (data.length === 0) {
    return <EmptyState description={emptyDescription} title={emptyTitle} />;
  }

  return (
    <div className="overflow-x-auto rounded-[var(--radius-card)] border border-border bg-paper-strong">
      <table className="w-full min-w-[760px] border-collapse text-left">
        <thead className="border-b border-border bg-paper-tinted">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <th
                  className="px-md py-sm font-mono text-mono uppercase tracking-[0.12em] text-ink-muted"
                  key={header.id}
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                </th>
              ))}
            </tr>
          ))}
        </thead>
        <tbody>
          {table.getRowModel().rows.map((row) => (
            <tr className={cn("border-b border-border last:border-b-0")} key={row.id}>
              {row.getVisibleCells().map((cell) => (
                <td className="px-md py-sm align-top text-body-sm" key={cell.id}>
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
