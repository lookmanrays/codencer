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
    executorProfile: z.string().trim().optional(),
    hostLabel: z.string().optional(),
    machineId: z.string().optional(),
    timeoutSeconds: z.coerce.number().int().min(5).max(900).default(120),
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
  .transform(({ project_id, result }) => {
    const payload = asRecord(result);
    const task = asRecord(payload.task);
    const run = asRecord(payload.run);
    const blocker = asRecord(payload.blocker);
    return RunSubmitResultSchema.parse({
      blockerType: stringValue(blocker.type),
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
      summary: stringValue(payload.summary) || stringValue(task.summary),
    });
  });

export const RunSubmitResultSchema = z.object({
  blockerType: z.string().optional(),
  ok: z.boolean(),
  executorProfile: z.string().optional(),
  projectId: z.string().optional(),
  raw: z.record(z.string(), z.unknown()),
  relayProfileId: z.string().optional(),
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

export type TaskRunInput = z.input<typeof TaskRunInputSchema>;
export type ParsedTaskRunInput = z.output<typeof TaskRunInputSchema>;
export type RunSubmitResult = z.infer<typeof RunSubmitResultSchema>;
