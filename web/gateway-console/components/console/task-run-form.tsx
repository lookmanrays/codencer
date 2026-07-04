"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Play, RotateCcw } from "lucide-react";
import { Controller, useForm } from "react-hook-form";
import { useCallback, useEffect, useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useExecutors } from "@/api/executors";
import { queryKeys } from "@/api/query-keys";
import { useProjectRunReport, useSubmitProjectRun } from "@/api/runs";
import { RunResultPanel } from "@/components/console/run-result-panel";
import type { ExecutorProfile } from "@/schemas/executors";
import type { Project } from "@/schemas/projects";
import {
  TaskRunInputSchema,
  type ParsedTaskRunInput,
  type TaskRunInput,
} from "@/schemas/runs";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { KeyValueList } from "@/components/ui/key-value-list";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

export function TaskRunForm({ projects }: { projects: Project[] }) {
  const queryClient = useQueryClient();
  const executors = useExecutors();
  const submitRun = useSubmitProjectRun();
  const locations = useMemo(
    () =>
      projects.flatMap((project) =>
        project.locations
          .filter((location) => location.status === "online")
          .map((location) => ({ ...location, projectName: project.name })),
      ),
    [projects],
  );
  const executorList = useMemo(
    () => executors.data?.executors ?? [],
    [executors.data?.executors],
  );
  const executorByID = useMemo(
    () => new Map(executorList.map((executor) => [executor.id, executor])),
    [executorList],
  );
  const firstLocation = locations[0];
  const firstProject = projects.find(
    (project) => project.id === firstLocation?.projectId,
  );
  const initialExecutor = resolvedProjectExecutor(firstProject);
  const initialDefaults = taskDefaultsForExecutor(
    initialExecutor,
    firstLocation?.projectId ?? "codencer",
    executorByID.get(initialExecutor),
  );
  const form = useForm<TaskRunInput, unknown, ParsedTaskRunInput>({
    resolver: zodResolver(TaskRunInputSchema),
    defaultValues: {
      dangerousExecutorConfirmed: false,
      executorProfile: initialExecutor,
      goal: initialDefaults.goal,
      locationId: firstLocation?.id ?? "",
      manifestText: initialDefaults.manifestText,
      manualExecutorProfile: "",
      mode: "task",
      projectId: firstLocation?.projectId ?? "",
      relayProfileId: firstLocation?.relayProfileId ?? "",
      hostLabel: firstLocation?.hostLabel ?? "",
      machineId: firstLocation?.machineId ?? "",
      timeoutSeconds: initialDefaults.timeoutSeconds,
      title: initialDefaults.title,
    },
  });
  const mode = form.watch("mode");
  const locationId = form.watch("locationId");
  const executorProfile = form.watch("executorProfile");
  const manualExecutorProfile = form.watch("manualExecutorProfile")?.trim();
  const timeoutSeconds = form.watch("timeoutSeconds");
  const dangerousExecutorConfirmed = form.watch("dangerousExecutorConfirmed");
  const selectedLocation = useMemo(
    () => locations.find((location) => location.id === locationId),
    [locationId, locations],
  );
  const selectedProject = useMemo(
    () =>
      projects.find((project) => project.id === selectedLocation?.projectId),
    [projects, selectedLocation?.projectId],
  );
  const effectiveExecutorID = manualExecutorProfile || executorProfile;
  const selectedExecutor = executorByID.get(effectiveExecutorID);
  const executorRequiresConfirmation =
    selectedExecutor?.id === "codex-full" ||
    selectedExecutor?.id === "codex-danger-bypass" ||
    selectedExecutor?.dangerous_bypass_approvals_and_sandbox === true ||
    selectedExecutor?.requires_explicit_allow_dangerous_bypass === true;
  const executorError =
    executors.isLoading || executors.isPending
      ? "Executor profiles are loading."
      : executors.error
        ? "Executor profiles are unavailable."
        : effectiveExecutorID && !selectedExecutor
          ? `Unknown executor profile: ${effectiveExecutorID}`
          : executorRequiresConfirmation && !dangerousExecutorConfirmed
            ? `${effectiveExecutorID} requires explicit confirmation.`
            : "";
  const report = useProjectRunReport({
    enabled: Boolean(submitRun.data?.runId),
    hostLabel: selectedLocation?.hostLabel,
    machineId: selectedLocation?.machineId,
    projectId: submitRun.data?.projectId ?? selectedProject?.id,
    relayProfileId:
      submitRun.data?.relayProfileId ?? selectedLocation?.relayProfileId,
    runId: submitRun.data?.runId,
  });

  const applyExecutorDefaults = useCallback(
    (executorID: string, projectID: string, executor?: ExecutorProfile) => {
      const defaults = taskDefaultsForExecutor(executorID, projectID, executor);
      const dirty = form.formState.dirtyFields;
      if (!dirty.title) {
        form.setValue("title", defaults.title, { shouldValidate: true });
      }
      if (!dirty.goal) {
        form.setValue("goal", defaults.goal, { shouldValidate: true });
      }
      if (!dirty.timeoutSeconds) {
        form.setValue("timeoutSeconds", defaults.timeoutSeconds, {
          shouldValidate: true,
        });
      }
      if (!dirty.manifestText) {
        form.setValue("manifestText", defaults.manifestText, {
          shouldValidate: true,
        });
      }
    },
    [form],
  );

  const applyExecutor = useCallback(
    (executorID: string) => {
      form.setValue("executorProfile", executorID, { shouldValidate: true });
      form.setValue("manualExecutorProfile", "", { shouldValidate: true });
      form.setValue("dangerousExecutorConfirmed", false);
      applyExecutorDefaults(
        executorID,
        selectedProject?.id ?? form.getValues("projectId"),
        executorByID.get(executorID),
      );
    },
    [applyExecutorDefaults, executorByID, form, selectedProject?.id],
  );

  const applyLocation = useCallback(
    (nextLocationId: string) => {
      const location = locations.find((item) => item.id === nextLocationId);
      if (!location) return;
      const project = projects.find((item) => item.id === location.projectId);
      const nextExecutor = resolvedProjectExecutor(project);
      form.setValue("locationId", location.id, { shouldValidate: true });
      form.setValue("projectId", location.projectId, { shouldValidate: true });
      form.setValue("relayProfileId", location.relayProfileId, {
        shouldValidate: true,
      });
      form.setValue("hostLabel", location.hostLabel);
      form.setValue("machineId", location.machineId);
      form.setValue("executorProfile", nextExecutor, { shouldValidate: true });
      form.setValue("manualExecutorProfile", "", { shouldValidate: true });
      form.setValue("dangerousExecutorConfirmed", false);
      applyExecutorDefaults(
        nextExecutor,
        location.projectId,
        executorByID.get(nextExecutor),
      );
    },
    [applyExecutorDefaults, executorByID, form, locations, projects],
  );

  useEffect(() => {
    if (!firstLocation || form.getValues("locationId")) return;
    applyLocation(firstLocation.id);
  }, [applyLocation, firstLocation, form]);

  useEffect(() => {
    if (!submitRun.isSuccess) return;
    void queryClient.invalidateQueries({ queryKey: queryKeys.auditEvents });
    void queryClient.invalidateQueries({ queryKey: queryKeys.runs });
  }, [queryClient, submitRun.isSuccess]);

  useEffect(() => {
    if (!report.dataUpdatedAt) return;
    void queryClient.invalidateQueries({ queryKey: queryKeys.auditEvents });
    void queryClient.invalidateQueries({ queryKey: queryKeys.runs });
    if (submitRun.data?.runHistoryId) {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.run(submitRun.data.runHistoryId),
      });
      void queryClient.invalidateQueries({
        queryKey: queryKeys.runEvents(submitRun.data.runHistoryId),
      });
    }
  }, [queryClient, report.dataUpdatedAt, submitRun.data?.runHistoryId]);

  const submitDisabled =
    submitRun.isPending || locations.length === 0 || Boolean(executorError);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Submit task</CardTitle>
      </CardHeader>
      <CardContent>
        <form
          className="grid min-w-0 gap-md"
          onSubmit={form.handleSubmit(async (values) => {
            if (executorError) return;
            const effectiveID =
              values.manualExecutorProfile?.trim() || values.executorProfile;
            await submitRun.mutateAsync({
              ...values,
              executorAdapter: executorByID.get(effectiveID)?.adapter,
            });
            await queryClient.invalidateQueries({
              queryKey: queryKeys.auditEvents,
            });
            await queryClient.invalidateQueries({ queryKey: queryKeys.runs });
          })}
        >
          {executors.error ? (
            <Alert title="Executor profiles unavailable" tone="danger">
              {executors.error.message}
            </Alert>
          ) : null}
          {submitRun.error ? (
            <Alert title="Run submission failed" tone="danger">
              {submitRun.error.message}
            </Alert>
          ) : null}
          {submitRun.data ? (
            <RunResultPanel result={report.data ?? submitRun.data} />
          ) : null}
          <div className="grid min-w-0 gap-md md:grid-cols-2">
            <Field
              error={form.formState.errors.locationId?.message}
              id="run-location"
              label="Project location"
            >
              <Controller
                control={form.control}
                name="locationId"
                render={({ field }) => (
                  <Select
                    disabled={locations.length === 0}
                    onValueChange={applyLocation}
                    value={field.value}
                  >
                    <SelectTrigger
                      aria-label="Project location"
                      id="run-location"
                    >
                      <SelectValue placeholder="Select project location" />
                    </SelectTrigger>
                    <SelectContent>
                      {locations.map((location) => (
                        <SelectItem key={location.id} value={location.id}>
                          {location.projectName} / {location.relayProfileId} /{" "}
                          {location.hostLabel || location.machineId}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
            <Field
              error={form.formState.errors.executorProfile?.message}
              id="run-executor"
              label="Executor"
            >
              <Controller
                control={form.control}
                name="executorProfile"
                render={({ field }) => (
                  <Select
                    disabled={executors.isLoading || executorList.length === 0}
                    onValueChange={applyExecutor}
                    value={field.value}
                  >
                    <SelectTrigger aria-label="Executor" id="run-executor">
                      <SelectValue placeholder="Select executor" />
                    </SelectTrigger>
                    <SelectContent>
                      {executorList.map((executor) => (
                        <SelectItem key={executor.id} value={executor.id}>
                          {executor.id} / {executor.adapter}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
          </div>
          {executorError ? (
            <Alert title="Executor selection required" tone="warning">
              {executorError}
            </Alert>
          ) : null}
          {executorRequiresConfirmation ? (
            <label className="flex min-w-0 items-start gap-sm text-body-sm text-ink-secondary">
              <Controller
                control={form.control}
                name="dangerousExecutorConfirmed"
                render={({ field }) => (
                  <Checkbox
                    aria-label="Confirm elevated executor"
                    checked={Boolean(field.value)}
                    onCheckedChange={(value) => field.onChange(value === true)}
                  />
                )}
              />
              <span className="min-w-0 break-words">
                I understand {effectiveExecutorID} has elevated executor access
                and should only be used for an approved, isolated run.
              </span>
            </label>
          ) : null}
          <Alert title="Route preview" tone="info">
            <KeyValueList
              items={[
                { label: "Project", value: selectedProject?.name ?? "n/a" },
                {
                  label: "Relay",
                  value: selectedLocation?.relayProfileId ?? "n/a",
                },
                {
                  label: "Connector",
                  value: selectedLocation?.connectorId || "n/a",
                },
                {
                  label: "Machine",
                  value:
                    selectedLocation?.hostLabel ||
                    selectedLocation?.machineId ||
                    "n/a",
                },
                { label: "Executor", value: effectiveExecutorID || "n/a" },
                {
                  label: "Execution",
                  value: mode === "manifest" ? "manifest" : "task",
                },
                { label: "Timeout", value: `${timeoutSeconds || 0}s` },
              ]}
            />
          </Alert>
          <div className="grid min-w-0 gap-md md:grid-cols-[1fr_160px]">
            <Field
              error={form.formState.errors.title?.message}
              id="run-title"
              label="Title"
            >
              <Input id="run-title" {...form.register("title")} />
            </Field>
            <Field
              error={form.formState.errors.timeoutSeconds?.message}
              id="run-timeout"
              label="Timeout seconds"
            >
              <Input
                id="run-timeout"
                min={5}
                type="number"
                {...form.register("timeoutSeconds")}
              />
            </Field>
          </div>
          {mode === "task" ? (
            <Field
              error={form.formState.errors.goal?.message}
              id="run-goal"
              label="Goal"
            >
              <Textarea id="run-goal" {...form.register("goal")} />
            </Field>
          ) : null}
          <Accordion collapsible type="single">
            <AccordionItem value="advanced">
              <AccordionTrigger>Advanced</AccordionTrigger>
              <AccordionContent>
                <div className="grid min-w-0 gap-md">
                  <Field
                    error={form.formState.errors.manualExecutorProfile?.message}
                    id="run-manual-executor-profile"
                    label="Manual executor profile override"
                  >
                    <Input
                      id="run-manual-executor-profile"
                      placeholder="Known executor profile id"
                      {...form.register("manualExecutorProfile")}
                    />
                  </Field>
                  <Field
                    error={form.formState.errors.mode?.message}
                    id="run-mode"
                    label="Execution mode"
                  >
                    <Controller
                      control={form.control}
                      name="mode"
                      render={({ field }) => (
                        <Select
                          onValueChange={(value) => {
                            field.onChange(value);
                            form.setValue(
                              "manifestText",
                              defaultManifest(
                                selectedProject?.id ??
                                  form.getValues("projectId") ??
                                  "codencer",
                                effectiveExecutorID,
                                selectedExecutor,
                              ),
                              { shouldValidate: true },
                            );
                          }}
                          value={field.value}
                        >
                          <SelectTrigger
                            aria-label="Execution mode"
                            id="run-mode"
                          >
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="task">Simple task</SelectItem>
                            <SelectItem value="manifest">
                              Manifest / run plan
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      )}
                    />
                  </Field>
                  {mode === "manifest" ? (
                    <Alert title="Manifest schema help" tone="warning">
                      Required YAML fields: version, kind, metadata.name,
                      project.id, execution.profile, tasks[].id, tasks[].title,
                      tasks[].goal.
                    </Alert>
                  ) : null}
                  {mode === "manifest" ? (
                    <Field
                      error={form.formState.errors.manifestText?.message}
                      id="run-manifest"
                      label="Manifest / run plan"
                    >
                      <Textarea
                        className="min-h-[260px] font-mono text-mono"
                        id="run-manifest"
                        {...form.register("manifestText")}
                      />
                    </Field>
                  ) : null}
                </div>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
          <div className="flex flex-wrap gap-sm">
            <Button disabled={submitDisabled}>
              <Play aria-hidden="true" className="h-4 w-4" />
              {submitRun.isPending ? "Submitting..." : "Submit"}
            </Button>
            <Button
              onClick={() => submitRun.reset()}
              type="button"
              variant="quiet"
            >
              <RotateCcw aria-hidden="true" className="h-4 w-4" />
              Reset result
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}

function resolvedProjectExecutor(project?: Project) {
  return project?.profile || project?.adapter || "";
}

function taskDefaultsForExecutor(
  executorID: string,
  projectID: string,
  executor?: ExecutorProfile,
) {
  const normalized = executorID.trim();
  let title = "Gateway Console fake-safe task";
  let goal = "Run a deterministic fake-safe task from Gateway Console.";
  let timeoutSeconds = 120;
  if (normalized.startsWith("codex")) {
    title =
      normalized === "codex-workspace"
        ? "Codex workspace smoke task"
        : "Codex executor smoke task";
    goal =
      "Inspect the project README and return a short summary. Do not modify files.";
    timeoutSeconds = 300;
  } else if (normalized.startsWith("claude")) {
    title = "Claude CLI smoke task";
    goal =
      "Inspect the project README and return a short summary. Do not modify files.";
    timeoutSeconds = 300;
  } else if (normalized.startsWith("antigravity")) {
    title = "Antigravity smoke task";
    goal =
      "Inspect the project README and return a short summary. Do not modify files.";
    timeoutSeconds = 300;
  }
  return {
    goal,
    manifestText: defaultManifest(projectID, normalized, executor),
    timeoutSeconds,
    title,
  };
}

function defaultManifest(
  projectId: string,
  executorID: string,
  executor?: ExecutorProfile,
) {
  const profile = executorID || "fake-success";
  const adapter = executor?.adapter || profile.split("-")[0] || "fake";
  const defaults = taskDefaultsForExecutorTitle(profile);
  return `version: codencer.io/v1alpha1
kind: RunManifest
metadata:
  name: gateway-console-run
project:
  id: ${projectId}
execution:
  adapter: ${adapter}
  profile: ${profile}
policy:
  stop_on_blocker: true
  stop_on_failure: true
tasks:
  - id: gateway-console-task
    title: ${defaults.title}
    goal: ${defaults.goal}
`;
}

function taskDefaultsForExecutorTitle(executorID: string) {
  if (executorID.startsWith("codex")) {
    return {
      title: "Codex workspace smoke task",
      goal: "Inspect the project README and return a short summary. Do not modify files.",
    };
  }
  if (executorID.startsWith("claude")) {
    return {
      title: "Claude CLI smoke task",
      goal: "Inspect the project README and return a short summary. Do not modify files.",
    };
  }
  if (executorID.startsWith("antigravity")) {
    return {
      title: "Antigravity smoke task",
      goal: "Inspect the project README and return a short summary. Do not modify files.",
    };
  }
  return {
    title: "Gateway Console fake-safe task",
    goal: "Complete a deterministic fake success task.",
  };
}
