import { z } from "zod";
import { collectionField } from "@/schemas/collections";
import {
  AuditEventListResponseSchema,
  PaginationSchema,
} from "@/schemas/audit";
import { executionModeFromPayload } from "@/schemas/runs";

const UnknownRecord = z.record(z.string(), z.unknown());

export const RunRecordSchema = z
  .object({
    completed_at: z.string().optional(),
    connector_id: z.string().optional(),
    created_at: z.string(),
    executor_profile: z.string().optional(),
    goal: z.string().optional(),
    host_label: z.string().optional(),
    id: z.string(),
    machine_id: z.string().optional(),
    mode: z.string().optional(),
    project_id: z.string(),
    project_name: z.string().optional(),
    relay_profile_id: z.string().optional(),
    report: UnknownRecord.optional(),
    report_status: z.string().optional(),
    result_details: z.string().optional(),
    result_summary: z.string().optional(),
    run_id: z.string().optional(),
    scope: z.string().optional(),
    started_at: z.string().optional(),
    status: z.string().optional(),
    step_id: z.string().optional(),
    title: z.string().optional(),
    updated_at: z.string(),
  })
  .transform((run) => ({
    completedAt: run.completed_at,
    connectorId: run.connector_id,
    createdAt: run.created_at,
    executorProfile: run.executor_profile,
    executionMode: executionModeFromPayload(run.report ?? {}),
    goal: run.goal,
    hostLabel: run.host_label,
    id: run.id,
    machineId: run.machine_id,
    mode: run.mode,
    projectId: run.project_id,
    projectName: run.project_name,
    relayProfileId: run.relay_profile_id,
    report: run.report ?? {},
    reportStatus: run.report_status,
    resultDetails: run.result_details,
    resultSummary: run.result_summary,
    runId: run.run_id,
    scope: run.scope,
    startedAt: run.started_at,
    status: run.status,
    stepId: run.step_id,
    title: run.title,
    updatedAt: run.updated_at,
  }));

export const RunListResponseSchema = z
  .object({
    pagination: PaginationSchema.optional(),
    runs: collectionField(RunRecordSchema),
  })
  .transform(({ pagination, runs }) => ({
    pagination: pagination ?? PaginationSchema.parse({}),
    runs,
  }));

export const RunDetailResponseSchema = z.object({
  run: RunRecordSchema,
});

export const RunEventsResponseSchema = AuditEventListResponseSchema.transform(
  ({ auditEvents, groups, pagination }) => ({
    events: auditEvents,
    groups,
    pagination,
  }),
);

export const HumanInterruptResponseSchema = z
  .object({
    follow_up: z.string().optional(),
    follow_up_result: UnknownRecord.optional(),
    next_actions: UnknownRecord.optional(),
    ok: z.boolean(),
    project_id: z.string(),
    reason: z.string().optional(),
    response: z
      .object({
        operator_response: z.string(),
        type: z.string(),
      })
      .optional(),
    run_history_id: z.string(),
    run_id: z.string().optional(),
    status: z.string(),
  })
  .transform((payload) => ({
    followUp: payload.follow_up,
    followUpResult: payload.follow_up_result ?? {},
    nextActions: payload.next_actions ?? {},
    ok: payload.ok,
    projectId: payload.project_id,
    reason: payload.reason,
    response: payload.response
      ? {
          operatorResponse: payload.response.operator_response,
          type: payload.response.type,
        }
      : undefined,
    runHistoryId: payload.run_history_id,
    runId: payload.run_id,
    status: payload.status,
  }));

export type RunRecord = z.infer<typeof RunRecordSchema>;
export type HumanInterruptResponse = z.infer<
  typeof HumanInterruptResponseSchema
>;
