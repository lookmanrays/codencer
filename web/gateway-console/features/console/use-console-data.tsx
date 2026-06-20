"use client";

import { AsyncState } from "@/components/ui/async-state";
import { useConsoleSnapshot } from "@/api/hooks";
import type { ConsoleSnapshot } from "@/schemas/console";

export function ConsoleData({
  children,
  emptyDescription = "No console data is available.",
  emptyTitle = "No data",
}: {
  children: (snapshot: ConsoleSnapshot) => React.ReactNode;
  emptyTitle?: string;
  emptyDescription?: string;
}) {
  const query = useConsoleSnapshot();
  return (
    <AsyncState
      data={query.data}
      emptyDescription={emptyDescription}
      emptyTitle={emptyTitle}
      error={query.error}
      isEmpty={(data) => data.relays.length === 0 && data.projects.length === 0}
      isLoading={query.isLoading}
    >
      {children}
    </AsyncState>
  );
}
