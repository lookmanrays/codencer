"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { TaskRunForm } from "@/components/console/task-run-form";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";
import { isDemoMode } from "@/api/config";
import { useProjects } from "@/api/projects";
import type { Project, ProjectLocation } from "@/schemas/projects";

type LocationSelection = ProjectLocation & {
  projectName: string;
};

export function ProjectRunScreen() {
  const params = useSearchParams();
  const projects = useProjects();
  const requested = {
    hostLabel: params.get("host_label") ?? "",
    machineId: params.get("machine_id") ?? "",
    projectId: params.get("project_id") ?? "",
    relayProfileId: params.get("relay_profile_id") ?? "",
  };

  const selected = projects.data
    ? selectLocation(projects.data.projects, requested)
    : undefined;
  const hasParams = Boolean(
    requested.projectId || requested.relayProfileId || requested.machineId,
  );
  const invalidParams = projects.data && hasParams && !selected;

  return (
    <PageShell
      breadcrumbs={[
        { label: "Console", href: "/console" },
        { label: "Projects", href: "/console/projects" },
        { label: "Run" },
      ]}
      description="Submit a task against one selected project location."
      kicker="Projects"
      title="Run project task"
    >
      {projects.isLoading ? <LoadingPanel /> : null}
      {projects.error ? (
        <Alert title="Project API unavailable" tone="danger">
          {projects.error.message}
        </Alert>
      ) : null}
      {projects.data ? (
        <div className="grid min-w-0 max-w-full gap-lg">
          {isDemoMode() ? <DemoModeNotice /> : null}
          {invalidParams ? (
            <Alert title="Project location not found" tone="warning">
              The requested route parameters did not match an online project
              location. Select another project location below or return to the
              project inventory.
            </Alert>
          ) : null}
          {projects.data.projects.length === 0 ? (
            <EmptyState
              description="Share a project from a connector before submitting a task."
              title="No projects"
            />
          ) : (
            <div className="grid min-w-0 gap-lg xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
              <div className="grid min-w-0 gap-md self-start">
                <Card>
                  <CardHeader>
                    <CardTitle>Selected location</CardTitle>
                  </CardHeader>
                  <CardContent>
                    {selected ? (
                      <KeyValueList
                        items={[
                          { label: "Project", value: selected.projectName },
                          { label: "Project ID", value: selected.projectId },
                          { label: "Relay", value: selected.relayProfileId },
                          {
                            label: "Machine",
                            value: selected.hostLabel || selected.machineId,
                          },
                          { label: "Repo", value: selected.repoLabel },
                          { label: "Status", value: selected.status },
                        ]}
                      />
                    ) : (
                      <p className="m-0 text-body-sm text-ink-secondary">
                        Choose a project location in the form. The inventory
                        page links here with route parameters for direct
                        selection.
                      </p>
                    )}
                    <div className="mt-md">
                      <Link
                        className={buttonVariants({
                          size: "sm",
                          variant: "secondary",
                        })}
                        href="/console/projects"
                      >
                        Back to inventory
                      </Link>
                    </div>
                  </CardContent>
                </Card>
              </div>
              <TaskRunForm
                projects={projects.data.projects}
                selectedLocationId={selected?.id}
              />
            </div>
          )}
          {projects.data.relayErrors.length > 0 ? (
            <Alert title="Some Relay profiles are unavailable" tone="warning">
              Gateway returned explicit Relay errors for this project listing.
            </Alert>
          ) : null}
        </div>
      ) : null}
    </PageShell>
  );
}

function selectLocation(
  projects: Project[],
  requested: {
    hostLabel: string;
    machineId: string;
    projectId: string;
    relayProfileId: string;
  },
): LocationSelection | undefined {
  const locations = projects.flatMap((project) =>
    project.locations.map((location) => ({
      ...location,
      projectName: project.name,
    })),
  );
  if (
    !requested.projectId &&
    !requested.relayProfileId &&
    !requested.machineId
  ) {
    return locations.find((location) => location.status === "online");
  }
  return locations.find((location) => {
    if (requested.projectId && location.projectId !== requested.projectId) {
      return false;
    }
    if (
      requested.relayProfileId &&
      location.relayProfileId !== requested.relayProfileId
    ) {
      return false;
    }
    if (requested.machineId && location.machineId !== requested.machineId) {
      return false;
    }
    if (requested.hostLabel && location.hostLabel !== requested.hostLabel) {
      return false;
    }
    return location.status === "online";
  });
}
