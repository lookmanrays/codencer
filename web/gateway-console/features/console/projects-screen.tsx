"use client";

import { ProjectLocationsTable } from "@/components/console/project-locations-table";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ConsoleData } from "@/features/console/use-console-data";

export function ProjectsScreen() {
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
      <ConsoleData
        emptyDescription="Share a project from a connector to advertise it."
        emptyTitle="No projects"
      >
        {(snapshot) => (
          <div className="grid gap-lg">
            <Alert title="Routing rule" tone="warning">
              When the same project is available from multiple Relay profiles or
              machines, execution requires <code>relay_profile_id</code>,{" "}
              <code>machine_id</code>, or <code>host_label</code>.
            </Alert>
            <ProjectLocationsTable projects={snapshot.projects} />
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
        )}
      </ConsoleData>
    </PageShell>
  );
}
