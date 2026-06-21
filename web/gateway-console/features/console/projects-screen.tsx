"use client";

import { ProjectLocationsTable } from "@/components/console/project-locations-table";
import { TaskRunForm } from "@/components/console/task-run-form";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { LoadingPanel } from "@/components/ui/skeleton";
import { isDemoMode } from "@/api/config";
import { useProjects } from "@/api/projects";

export function ProjectsScreen() {
  const projects = useProjects();
  return (
    <PageShell
      breadcrumbs={[
        { label: "Console", href: "/console" },
        { label: "Projects" },
      ]}
      description="View safe project location metadata across Relay profiles and machines."
      kicker="Projects"
      title="Project locations"
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
          <Alert title="Routing rule" tone="warning">
            When the same project is available from multiple Relay profiles or
            machines, execution requires <code>relay_profile_id</code>,{" "}
            <code>machine_id</code>, or <code>host_label</code>.
          </Alert>
          {projects.data.projects.length === 0 ? (
            <EmptyState
              description="Share a project from a connector to advertise it."
              title="No projects"
            />
          ) : (
            <>
              <TaskRunForm projects={projects.data.projects} />
              <ProjectLocationsTable projects={projects.data.projects} />
            </>
          )}
          {projects.data.relayErrors.length > 0 ? (
            <Alert title="Some Relay profiles are unavailable" tone="warning">
              Gateway returned explicit Relay errors for this project listing.
            </Alert>
          ) : null}
          <Card>
            <CardHeader>
              <CardTitle>Visible metadata</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="m-0 text-body text-ink-secondary">
                This view deliberately shows repo labels and hashes only.
                Absolute local paths, daemon URLs, and connector private keys
                are not UI data.
              </p>
            </CardContent>
          </Card>
        </div>
      ) : null}
    </PageShell>
  );
}
