"use client";

import { useQuery } from "@tanstack/react-query";
import { getConsoleSnapshot } from "@/api/client";

export function useConsoleSnapshot() {
  return useQuery({
    queryKey: ["console-snapshot"],
    queryFn: getConsoleSnapshot,
  });
}
