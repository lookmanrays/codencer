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
  density = "default",
  emptyDescription = "No records match this view.",
  emptyTitle = "No records",
  getRowHref,
  minWidth = "760px",
}: {
  columns: ColumnDef<TData>[];
  data: TData[];
  density?: "default" | "compact";
  emptyTitle?: string;
  emptyDescription?: string;
  getRowHref?: (row: TData) => string | undefined;
  minWidth?: string;
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
            className={cn(
              "min-w-0 max-w-full rounded-[var(--radius-card)] border border-border bg-paper-strong p-md",
              getRowHref &&
                "cursor-pointer transition-colors hover:bg-paper-tinted",
            )}
            key={row.id}
            onClick={(event) => {
              const href = getRowHref?.(row.original);
              if (!href || shouldIgnoreRowClick(event.target)) return;
              window.location.assign(href);
            }}
            onKeyDown={(event) => {
              const href = getRowHref?.(row.original);
              if (!href) return;
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                window.location.assign(href);
              }
            }}
            role={getRowHref ? "link" : undefined}
            tabIndex={getRowHref ? 0 : undefined}
          >
            <dl className="m-0 grid min-w-0 gap-sm">
              {row.getVisibleCells().map((cell) => (
                <div
                  className={cn(
                    "grid min-w-0 gap-xs border-t border-border first:border-t-0 first:pt-0 last:pb-0",
                    density === "compact" ? "py-xs" : "py-sm",
                  )}
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
        <table
          className="w-full border-collapse text-left"
          style={{ minWidth }}
        >
          <thead className="border-b border-border bg-paper-tinted">
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <th
                    className={cn(
                      "font-mono text-mono uppercase tracking-[0.12em] text-ink-muted",
                      density === "compact" ? "px-sm py-xs" : "px-md py-sm",
                    )}
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
                className={cn(
                  "border-b border-border last:border-b-0",
                  getRowHref &&
                    "cursor-pointer transition-colors hover:bg-paper-tinted",
                )}
                key={row.id}
                onClick={(event) => {
                  const href = getRowHref?.(row.original);
                  if (!href || shouldIgnoreRowClick(event.target)) return;
                  window.location.assign(href);
                }}
                onKeyDown={(event) => {
                  const href = getRowHref?.(row.original);
                  if (!href) return;
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    window.location.assign(href);
                  }
                }}
                role={getRowHref ? "link" : undefined}
                tabIndex={getRowHref ? 0 : undefined}
              >
                {row.getVisibleCells().map((cell) => (
                  <td
                    className={cn(
                      "align-top text-body-sm",
                      density === "compact" ? "px-sm py-xs" : "px-md py-sm",
                    )}
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

function shouldIgnoreRowClick(target: EventTarget | null) {
  return target instanceof Element
    ? Boolean(target.closest("a,button,input,select,textarea,[role='button']"))
    : false;
}
