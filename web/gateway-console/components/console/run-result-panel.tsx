import Link from "next/link";
import { FileText } from "lucide-react";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { KeyValueList } from "@/components/ui/key-value-list";
import type { RunSubmitResult } from "@/schemas/runs";
import type { RunRecord } from "@/schemas/run-history";

export function RunResultPanel({
  result,
  run,
}: {
  result?: RunSubmitResult;
  run?: RunRecord;
}) {
  const status = run?.status || result?.status || "completed";
  const pending = isPendingRunStatus(status);
  const summary =
    run?.resultSummary ||
    result?.summary ||
    run?.resultDetails ||
    result?.details ||
    (pending
      ? "Run submitted; waiting for executor result."
      : "Run completed, but no executor result text was returned.");
  const details = run?.resultDetails || result?.details || summary;
  const runHistoryId = run?.id || result?.runHistoryId;
  const runId = run?.runId || result?.runId || "n/a";
  const executor = run?.executorProfile || result?.executorProfile || "n/a";
  const executionMode =
    run?.executionMode || result?.executionMode || "unknown";
  const reportStatus = run?.reportStatus || (pending ? "pending" : "completed");
  const execution = executionModeDisplay(executionMode);
  return (
    <Alert
      title="Result"
      tone={executionMode === "simulation" ? "warning" : "brand"}
    >
      <div className="grid min-w-0 gap-md">
        <KeyValueList
          items={[
            { label: "Status", value: status },
            { label: "Run ID", value: runId },
            { label: "Executor", value: executor },
            {
              label: "Execution",
              value: (
                <Badge variant={execution.variant}>{execution.label}</Badge>
              ),
            },
            { label: "Report", value: reportStatus },
          ]}
        />
        <section className="min-w-0">
          <h4 className="m-0 text-body font-semibold text-ink-primary">
            Summary
          </h4>
          <p className="m-0 mt-xs min-w-0 whitespace-pre-wrap break-words text-body text-ink-primary">
            {summary}
          </p>
        </section>
        {details && details !== summary ? (
          <section className="min-w-0">
            <h4 className="m-0 text-body font-semibold text-ink-primary">
              Details
            </h4>
            <p className="m-0 mt-xs min-w-0 whitespace-pre-wrap break-words text-body-sm text-ink-secondary">
              {details}
            </p>
          </section>
        ) : null}
        {runHistoryId ? (
          <div>
            <Link
              className={buttonVariants({ size: "sm", variant: "secondary" })}
              href={`/console/runs/${runHistoryId}`}
            >
              <FileText aria-hidden="true" className="h-4 w-4" />
              View full run
            </Link>
          </div>
        ) : null}
      </div>
    </Alert>
  );
}

function isPendingRunStatus(status?: string) {
  const normalized = (status ?? "").trim().toLowerCase();
  return [
    "collecting_artifacts",
    "dispatching",
    "in_progress",
    "pending",
    "queued",
    "running",
    "started",
    "starting",
    "submitted",
    "validating",
  ].includes(normalized);
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
