"use client";

import { useEffect, useMemo, useState } from "react";
import {
  ProjectLocationsTable,
  projectLocationRows,
  type ProjectLocationRow,
} from "@/components/console/project-locations-table";
import { TaskRunForm } from "@/components/console/task-run-form";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { LoadingPanel } from "@/components/ui/skeleton";
import { isDemoMode } from "@/api/config";
import { useProjects } from "@/api/projects";

export function ProjectsScreen() {
  const projects = useProjects();
  const [selectedLocationId, setSelectedLocationId] = useState("");
  const [query, setQuery] = useState("");
  const [machine, setMachine] = useState("all");
  const [relay, setRelay] = useState("all");
  const [status, setStatus] = useState("all");
  const [ambiguity, setAmbiguity] = useState("all");
  const rows = useMemo(
    () => (projects.data ? projectLocationRows(projects.data.projects) : []),
    [projects.data],
  );
  const filteredRows = useMemo(
    () =>
      rows.filter((row) => {
        const haystack = [
          row.projectName,
          row.projectId,
          row.repoLabel,
          row.repoHash,
          row.hostLabel,
          row.hostname,
          row.machineId,
          row.relayProfileId,
        ]
          .join(" ")
          .toLowerCase();
        const textMatch =
          !query.trim() || haystack.includes(query.trim().toLowerCase());
        const machineMatch =
          machine === "all" || machine === (row.hostLabel || row.machineId);
        const relayMatch = relay === "all" || relay === row.relayProfileId;
        const statusMatch = status === "all" || status === row.status;
        const ambiguityMatch =
          ambiguity === "all" || ambiguity === row.ambiguity;
        return (
          textMatch &&
          machineMatch &&
          relayMatch &&
          statusMatch &&
          ambiguityMatch
        );
      }),
    [ambiguity, machine, query, relay, rows, status],
  );
  const selectedLocation =
    rows.find((row) => row.id === selectedLocationId) ??
    rows.find((row) => row.status === "online") ??
    rows[0];
  useEffect(() => {
    if (selectedLocationId || !selectedLocation) return;
    setSelectedLocationId(selectedLocation.id);
  }, [selectedLocation, selectedLocationId]);
  const machines = uniqueOptions(
    rows.map((row) => row.hostLabel || row.machineId).filter(Boolean),
  );
  const relays = uniqueOptions(rows.map((row) => row.relayProfileId));
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
            <div className="grid min-w-0 gap-lg xl:grid-cols-[minmax(0,1fr)_420px]">
              <div className="grid min-w-0 gap-md">
                <Card>
                  <CardHeader>
                    <CardTitle>Project and location inventory</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <div className="grid min-w-0 gap-md">
                      <div className="grid min-w-0 gap-sm md:grid-cols-[minmax(180px,1fr)_repeat(4,minmax(130px,160px))]">
                        <Field id="project-search" label="Search">
                          <Input
                            id="project-search"
                            onChange={(event) => setQuery(event.target.value)}
                            placeholder="Project, repo, machine"
                            value={query}
                          />
                        </Field>
                        <CompactFilter
                          id="project-machine-filter"
                          label="Machine"
                          onChange={setMachine}
                          options={machines}
                          value={machine}
                        />
                        <CompactFilter
                          id="project-relay-filter"
                          label="Relay"
                          onChange={setRelay}
                          options={relays}
                          value={relay}
                        />
                        <CompactFilter
                          id="project-status-filter"
                          label="Status"
                          onChange={setStatus}
                          options={uniqueOptions(rows.map((row) => row.status))}
                          value={status}
                        />
                        <CompactFilter
                          id="project-ambiguity-filter"
                          label="Ambiguity"
                          onChange={setAmbiguity}
                          options={uniqueOptions(
                            rows.map((row) => row.ambiguity),
                          )}
                          value={ambiguity}
                        />
                      </div>
                      <ProjectLocationsTable
                        onRun={(row: ProjectLocationRow) =>
                          setSelectedLocationId(row.id)
                        }
                        rows={filteredRows}
                        selectedLocationId={selectedLocation?.id}
                      />
                    </div>
                  </CardContent>
                </Card>
              </div>
              <aside className="min-w-0">
                <div className="sticky top-[88px] grid min-w-0 gap-md">
                  {selectedLocation ? (
                    <TaskRunForm
                      projects={projects.data.projects}
                      selectedLocationId={selectedLocation.id}
                    />
                  ) : (
                    <Card>
                      <CardHeader>
                        <CardTitle>Select a route</CardTitle>
                      </CardHeader>
                      <CardContent>
                        <p className="m-0 text-body-sm text-ink-secondary">
                          Choose an online project location to open the task
                          submission panel.
                        </p>
                      </CardContent>
                    </Card>
                  )}
                </div>
              </aside>
            </div>
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

function CompactFilter({
  id,
  label,
  onChange,
  options,
  value,
}: {
  id: string;
  label: string;
  onChange: (value: string) => void;
  options: string[];
  value: string;
}) {
  return (
    <Field id={id} label={label}>
      <Select onValueChange={onChange} value={value}>
        <SelectTrigger aria-label={label} id={id}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">All</SelectItem>
          {options.map((option) => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  );
}

function uniqueOptions(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) =>
    a.localeCompare(b),
  );
}
