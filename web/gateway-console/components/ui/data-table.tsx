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
  const table = useReactTable({
    columns,
    data,
    getCoreRowModel: getCoreRowModel(),
  });
  if (data.length === 0) {
    return <EmptyState description={emptyDescription} title={emptyTitle} />;
  }

  return (
    <>
      <div className="grid min-w-0 max-w-full gap-sm md:hidden">
        {table.getRowModel().rows.map((row) => (
          <article
            className="min-w-0 max-w-full rounded-[var(--radius-card)] border border-border bg-paper-strong p-md"
            key={row.id}
          >
            <dl className="m-0 grid min-w-0 gap-sm">
              {row.getVisibleCells().map((cell) => (
                <div
                  className="grid min-w-0 gap-xs border-t border-border py-sm first:border-t-0 first:pt-0 last:pb-0"
                  key={cell.id}
                >
                  <dt className="min-w-0 break-words font-mono text-mono uppercase tracking-[0.12em] text-ink-muted">
                    {headerText(cell.column.columnDef.header, cell.column.id)}
                  </dt>
                  <dd className="m-0 min-w-0 break-words text-body-sm text-ink-primary">
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </dd>
                </div>
              ))}
            </dl>
          </article>
        ))}
      </div>
      <div className="hidden min-w-0 max-w-full overflow-x-auto rounded-[var(--radius-card)] border border-border bg-paper-strong md:block">
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
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext(),
                        )}
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <tr
                className={cn("border-b border-border last:border-b-0")}
                key={row.id}
              >
                {row.getVisibleCells().map((cell) => (
                  <td
                    className="px-md py-sm align-top text-body-sm"
                    key={cell.id}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function headerText(header: unknown, fallback: string) {
  return typeof header === "string" ? header : fallback;
}
