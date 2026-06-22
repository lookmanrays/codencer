"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { Play, RotateCcw } from "lucide-react";
import { Controller, useForm } from "react-hook-form";
import { useCallback, useEffect, useMemo } from "react";
import { useProjectRunReport, useSubmitProjectRun } from "@/api/runs";
import type { Project } from "@/schemas/projects";
import {
  TaskRunInputSchema,
  type ParsedTaskRunInput,
  type TaskRunInput,
} from "@/schemas/runs";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

export function TaskRunForm({ projects }: { projects: Project[] }) {
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
  const firstLocation = locations[0];
  const form = useForm<TaskRunInput, unknown, ParsedTaskRunInput>({
    resolver: zodResolver(TaskRunInputSchema),
    defaultValues: {
      goal: "Run a deterministic fake-safe task from Gateway Console.",
      locationId: firstLocation?.id ?? "",
      manifestText: defaultManifest(firstLocation?.projectId ?? "codencer"),
      mode: "task",
      projectId: firstLocation?.projectId ?? "",
      relayProfileId: firstLocation?.relayProfileId ?? "",
      hostLabel: firstLocation?.hostLabel ?? "",
      machineId: firstLocation?.machineId ?? "",
      executorProfile: "",
      timeoutSeconds: 120,
      title: "Gateway Console fake-safe task",
    },
  });
  const mode = form.watch("mode");
  const locationId = form.watch("locationId");
  const executorProfile = form.watch("executorProfile");
  const selectedLocation = useMemo(
    () => locations.find((location) => location.id === locationId),
    [locationId, locations],
  );
  const selectedProject = useMemo(
    () =>
      projects.find((project) => project.id === selectedLocation?.projectId),
    [projects, selectedLocation?.projectId],
  );
  const resolvedExecutor =
    executorProfile?.trim() ||
    selectedProject?.profile ||
    selectedProject?.adapter ||
    "project default";
  const report = useProjectRunReport({
    enabled: Boolean(submitRun.data?.runId),
    hostLabel: selectedLocation?.hostLabel,
    machineId: selectedLocation?.machineId,
    projectId: submitRun.data?.projectId ?? selectedProject?.id,
    relayProfileId:
      submitRun.data?.relayProfileId ?? selectedLocation?.relayProfileId,
    runId: submitRun.data?.runId,
  });

  const applyLocation = useCallback(
    (locationId: string) => {
      const location = locations.find((item) => item.id === locationId);
      if (!location) return;
      form.setValue("locationId", location.id, { shouldValidate: true });
      form.setValue("projectId", location.projectId, { shouldValidate: true });
      form.setValue("relayProfileId", location.relayProfileId, {
        shouldValidate: true,
      });
      form.setValue("hostLabel", location.hostLabel);
      form.setValue("machineId", location.machineId);
      form.setValue("manifestText", defaultManifest(location.projectId));
    },
    [form, locations],
  );

  useEffect(() => {
    if (!firstLocation || form.getValues("locationId")) return;
    applyLocation(firstLocation.id);
  }, [applyLocation, firstLocation, form]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Submit task</CardTitle>
      </CardHeader>
      <CardContent>
        <form
          className="grid min-w-0 gap-md"
          onSubmit={form.handleSubmit(async (values) => {
            await submitRun.mutateAsync(values);
          })}
        >
          {submitRun.error ? (
            <Alert title="Run submission failed" tone="danger">
              {submitRun.error.message}
            </Alert>
          ) : null}
          {submitRun.data ? (
            <Alert title="Run completed" tone="brand">
              <span className="block min-w-0 break-words">
                status={submitRun.data.status ?? "completed"} run_id=
                {submitRun.data.runId ?? "n/a"} step_id=
                {submitRun.data.stepId ?? "n/a"}
              </span>
              <span className="mt-xs block min-w-0 break-words">
                executor={submitRun.data.executorProfile ?? resolvedExecutor}
              </span>
              <span className="mt-xs block min-w-0 break-words">
                report=
                {report.isPending
                  ? "loading"
                  : report.data?.status
                    ? report.data.status
                    : report.error
                      ? "unavailable"
                      : "pending"}
              </span>
              {submitRun.data.summary ? (
                <span className="mt-xs block min-w-0 break-words">
                  {submitRun.data.summary}
                </span>
              ) : null}
              {submitRun.data.blockerType ? (
                <span className="mt-xs block min-w-0 break-words">
                  blocker={submitRun.data.blockerType}
                </span>
              ) : null}
            </Alert>
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
              error={form.formState.errors.mode?.message}
              id="run-mode"
              label="Execution mode"
            >
              <Controller
                control={form.control}
                name="mode"
                render={({ field }) => (
                  <Select onValueChange={field.onChange} value={field.value}>
                    <SelectTrigger aria-label="Execution mode" id="run-mode">
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
          </div>
          <Alert title="Executor routing" tone="info">
            <span className="block min-w-0 break-words">
              Codencer is not the agent. Codencer routes this task to the
              selected executor.
            </span>
            <span className="mt-xs block min-w-0 break-words">
              relay={selectedLocation?.relayProfileId ?? "n/a"} connector=
              {selectedLocation?.connectorId || "n/a"} machine=
              {selectedLocation?.hostLabel ||
                selectedLocation?.machineId ||
                "n/a"}{" "}
              executor={resolvedExecutor}
            </span>
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
          <Field
            error={form.formState.errors.executorProfile?.message}
            id="run-executor-profile"
            label="Executor profile override"
          >
            <Input
              id="run-executor-profile"
              placeholder={selectedProject?.profile ?? "project default"}
              {...form.register("executorProfile")}
            />
          </Field>
          {mode === "manifest" ? (
            <Alert title="Advanced execution type" tone="warning">
              Manifest / run plan mode expects a valid Codencer run manifest and
              should be used when a planner has already produced the task plan.
            </Alert>
          ) : null}
          {mode === "manifest" ? (
            <Field
              error={form.formState.errors.manifestText?.message}
              id="run-manifest"
              label="Manifest"
            >
              <Textarea
                className="min-h-[220px] font-mono text-mono"
                id="run-manifest"
                {...form.register("manifestText")}
              />
            </Field>
          ) : (
            <Field
              error={form.formState.errors.goal?.message}
              id="run-goal"
              label="Goal"
            >
              <Textarea id="run-goal" {...form.register("goal")} />
            </Field>
          )}
          <div className="flex flex-wrap gap-sm">
            <Button disabled={submitRun.isPending || locations.length === 0}>
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

function defaultManifest(projectId: string) {
  return `version: codencer.io/v1alpha1
kind: RunManifest
metadata:
  name: gateway-console-fake-success
project:
  id: ${projectId}
execution:
  adapter: fake
  profile: fake-success
policy:
  stop_on_blocker: true
  stop_on_failure: true
tasks:
  - id: gateway-console-fake-success
    title: Gateway Console fake-safe task
    goal: Complete a deterministic fake success task.
`;
}
