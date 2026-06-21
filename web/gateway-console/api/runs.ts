"use client";

import { useMutation } from "@tanstack/react-query";
import { isDemoMode } from "@/api/config";
import { gatewayJSON } from "@/api/http";
import {
  RunSubmitResponseSchema,
  type ParsedTaskRunInput,
} from "@/schemas/runs";

export async function submitProjectRun(input: ParsedTaskRunInput) {
  if (isDemoMode()) {
    return RunSubmitResponseSchema.parse({
      ok: true,
      project_id: input.projectId,
      result: {
        ok: true,
        project_id: input.projectId,
        relay_profile_id: input.relayProfileId,
        run_id: "run_demo_console",
        status: "completed",
        step_id: "step_demo_console",
        summary: "Demo task completed through mock Gateway data.",
      },
    });
  }
  const body =
    input.mode === "manifest"
      ? {
          host_label: input.hostLabel,
          machine_id: input.machineId,
          manifest_name: input.title,
          manifest_text: input.manifestText,
          mode: "manifest",
          relay_profile_id: input.relayProfileId,
          wait: true,
        }
      : {
          goal: input.goal,
          host_label: input.hostLabel,
          machine_id: input.machineId,
          mode: "task",
          relay_profile_id: input.relayProfileId,
          timeout_seconds: input.timeoutSeconds,
          title: input.title,
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
