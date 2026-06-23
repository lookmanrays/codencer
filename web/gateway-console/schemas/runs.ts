import { z } from "zod";

const UnsafeOutputPattern =
  /\/Users\/|\/tmp\/|\/var\/folders\/|CODENCER_HOME|\.codencer-live-test|report_path|logs_ref|normalized_task_ref|original_input_ref|"path"/;

export const TaskRunInputSchema = z
  .object({
    goal: z.string().min(3),
    locationId: z.string().min(1),
    manifestText: z.string().optional(),
    mode: z.enum(["task", "manifest"]).default("task"),
    projectId: z.string().min(1),
    relayProfileId: z.string().min(1),
    dangerousExecutorConfirmed: z.boolean().optional(),
    executorProfile: z.string().trim().min(1),
    hostLabel: z.string().optional(),
    machineId: z.string().optional(),
    manualExecutorProfile: z.string().trim().optional(),
    timeoutSeconds: z.coerce.number().int().min(5).max(900).default(300),
    title: z.string().min(3),
  })
  .refine((value) => value.mode !== "manifest" || value.manifestText, {
    message: "Manifest text is required",
    path: ["manifestText"],
  });

export const RunSubmitResponseSchema = z
  .object({
    ok: z.boolean().optional(),
    project_id: z.string().optional(),
    run_history_id: z.string().optional(),
    result: z.unknown(),
  })
  .superRefine((value, ctx) => {
    const encoded = JSON.stringify(value);
    if (UnsafeOutputPattern.test(encoded)) {
      ctx.addIssue({
        code: "custom",
        message: "Gateway run output contained unsafe local path data",
      });
    }
  })
  .transform(({ project_id, result, run_history_id }) => {
    const payload = asRecord(result);
    const task = asRecord(payload.task);
    const run = asRecord(payload.run);
    const blocker = asRecord(payload.blocker);
    return RunSubmitResultSchema.parse({
      blockerType: stringValue(blocker.type),
      details: extractResultDetails(payload),
      ok: payload.ok === true,
      projectId: project_id ?? stringValue(payload.project_id),
      raw: payload,
      relayProfileId: stringValue(payload.relay_profile_id),
      executorProfile:
        stringValue(payload.executor_profile) ||
        stringValue(payload.profile) ||
        stringValue(payload.adapter_profile),
      runId:
        stringValue(payload.run_id) ||
        stringValue(run.id) ||
        stringValue(task.run_id),
      status: stringValue(payload.status) || stringValue(task.status),
      stepId: stringValue(payload.step_id) || stringValue(task.step_id),
      summary: extractResultSummary(payload),
      runHistoryId: run_history_id || stringValue(payload.run_history_id),
    });
  });

export const RunSubmitResultSchema = z.object({
  blockerType: z.string().optional(),
  details: z.string().optional(),
  ok: z.boolean(),
  executorProfile: z.string().optional(),
  projectId: z.string().optional(),
  raw: z.record(z.string(), z.unknown()),
  relayProfileId: z.string().optional(),
  runHistoryId: z.string().optional(),
  runId: z.string().optional(),
  status: z.string().optional(),
  stepId: z.string().optional(),
  summary: z.string().optional(),
});

export const RunReportResponseSchema = RunSubmitResponseSchema;

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function stringValue(value: unknown) {
  return typeof value === "string" ? value : "";
}

function extractResultSummary(payload: Record<string, unknown>) {
  const task = asRecord(payload.task);
  const firstTask = Array.isArray(payload.tasks)
    ? asRecord(payload.tasks[0])
    : {};
  const candidates = [
    stringValue(payload.summary),
    stringValue(task.summary),
    stringValue(firstTask.summary),
    stringValue(asRecord(asRecord(payload.evidence).result).summary),
    stringValue(asRecord(asRecord(task.evidence).result).summary),
    stringValue(asRecord(asRecord(firstTask.evidence).result).summary),
    stringValue(asRecord(payload.blocker).message),
    stringValue(asRecord(task.blocker).message),
    stringValue(asRecord(firstTask.blocker).message),
    stringValue(asRecord(asRecord(payload.evidence).result).raw_output),
    stringValue(asRecord(asRecord(task.evidence).result).raw_output),
    stringValue(asRecord(asRecord(firstTask.evidence).result).raw_output),
    artifactSummary(payload),
  ];
  return candidates.find((value) => value.trim().length > 0)?.trim();
}

function extractResultDetails(payload: Record<string, unknown>) {
  const task = asRecord(payload.task);
  const firstTask = Array.isArray(payload.tasks)
    ? asRecord(payload.tasks[0])
    : {};
  const rawOutput =
    stringValue(asRecord(asRecord(payload.evidence).result).raw_output) ||
    stringValue(asRecord(asRecord(task.evidence).result).raw_output) ||
    stringValue(asRecord(asRecord(firstTask.evidence).result).raw_output);
  return (
    rawOutput.trim() ||
    artifactSummary(payload) ||
    extractResultSummary(payload) ||
    undefined
  );
}

function artifactSummary(payload: Record<string, unknown>) {
  const names = new Set<string>();
  const visit = (value: unknown) => {
    if (Array.isArray(value)) {
      value.forEach(visit);
      return;
    }
    const record = asRecord(value);
    if (!Object.keys(record).length) return;
    const name = stringValue(record.name).trim();
    if (name) names.add(name);
    Object.values(record).forEach(visit);
  };
  visit(payload.artifacts);
  visit(asRecord(payload.evidence).artifacts);
  visit(asRecord(asRecord(payload.task).evidence).artifacts);
  visit(payload.tasks);
  return names.size > 0 ? `Artifacts: ${Array.from(names).join(", ")}` : "";
}

export type TaskRunInput = z.input<typeof TaskRunInputSchema>;
export type ParsedTaskRunInput = z.output<typeof TaskRunInputSchema>;
export type RunSubmitResult = z.infer<typeof RunSubmitResultSchema>;
