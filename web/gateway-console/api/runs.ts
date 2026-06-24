"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import { queryKeys } from "@/api/query-keys";
import {
  RunReportResponseSchema,
  RunSubmitResponseSchema,
  type ParsedTaskRunInput,
} from "@/schemas/runs";

export async function submitProjectRun(input: ParsedTaskRunInput) {
  const effectiveExecutorProfile =
    input.manualExecutorProfile?.trim() || input.executorProfile;
  if (isDemoMode()) {
    return RunSubmitResponseSchema.parse({
      ok: true,
      project_id: input.projectId,
      run_history_id: "runhist_demo_console",
      result: {
        ok: true,
        project_id: input.projectId,
        relay_profile_id: input.relayProfileId,
        run_id: "run_demo_console",
        status: "completed",
        step_id: "step_demo_console",
        executor_profile: effectiveExecutorProfile,
        summary:
          "Demo task completed through mock Gateway data. README summary: Codencer bridges planners to coding executors through local and self-host services.",
        evidence: {
          result: {
            adapter: "codex",
            is_simulation: false,
            raw_output:
              "Codencer is a local/self-host bridge between AI planners and coding executors. It records structured runs, results, blockers, and audit evidence.",
          },
        },
      },
    });
  }
  const executorFields = effectiveExecutorProfile
    ? {
        adapter_profile: effectiveExecutorProfile,
        profile: effectiveExecutorProfile,
      }
    : {};
  const body =
    input.mode === "manifest"
      ? {
          ...executorFields,
          host_label: input.hostLabel,
          machine_id: input.machineId,
          manifest_name: input.title,
          manifest_text: input.manifestText,
          mode: "manifest",
          relay_profile_id: input.relayProfileId,
          wait: true,
        }
      : {
          ...executorFields,
          goal: input.goal,
          host_label: input.hostLabel,
          machine_id: input.machineId,
          mode: "task",
          relay_profile_id: input.relayProfileId,
          timeout_seconds: input.timeoutSeconds,
          title: input.title,
          wait: false,
        };
  return gatewayJSON(
    `/projects/${encodeURIComponent(input.projectId)}/runs`,
    RunSubmitResponseSchema,
    {
      body: JSON.stringify(body),
      method: "POST",
    },
  );
}

export function useSubmitProjectRun() {
  return useMutation({
    mutationFn: submitProjectRun,
  });
}

export function useProjectRunReport(input: {
  enabled: boolean;
  hostLabel?: string;
  machineId?: string;
  projectId?: string;
  relayProfileId?: string;
  runId?: string;
}) {
  return useQuery({
    enabled: input.enabled && Boolean(input.projectId && input.runId),
    queryKey: queryKeys.projectRunReport(
      input.projectId ?? "",
      input.runId ?? "",
    ),
    queryFn: () => getProjectRunReport(input),
    refetchInterval: (query) =>
      shouldPollRunReport(query.state.data?.status) ? 2000 : false,
  });
}

async function getProjectRunReport(input: {
  hostLabel?: string;
  machineId?: string;
  projectId?: string;
  relayProfileId?: string;
  runId?: string;
}) {
  if (!input.projectId || !input.runId) {
    throw new Error("Project id and run id are required");
  }
  if (isDemoMode()) {
    return RunReportResponseSchema.parse({
      ok: true,
      project_id: input.projectId,
      run_id: input.runId,
      run_history_id: "runhist_demo_console",
      result: {
        ok: true,
        project_id: input.projectId,
        relay_profile_id: input.relayProfileId,
        run_id: input.runId,
        status: "completed",
        summary:
          "Demo report fetched through mock Gateway data. The executor returned a short README summary.",
        evidence: {
          result: {
            adapter: "codex",
            is_simulation: false,
            raw_output:
              "The README describes Codencer as an open-source local/self-host bridge. The task did not modify files.",
          },
        },
      },
    });
  }
  const params = new URLSearchParams();
  if (input.relayProfileId)
    params.set("relay_profile_id", input.relayProfileId);
  if (input.machineId) params.set("machine_id", input.machineId);
  if (input.hostLabel) params.set("host_label", input.hostLabel);
  const suffix = params.size > 0 ? `?${params.toString()}` : "";
  return gatewayJSON(
    `/projects/${encodeURIComponent(input.projectId)}/runs/${encodeURIComponent(input.runId)}/report${suffix}`,
    RunReportResponseSchema,
  );
}

function shouldPollRunReport(status?: string) {
  const normalized = (status ?? "").trim().toLowerCase();
  if (!normalized) return true;
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
