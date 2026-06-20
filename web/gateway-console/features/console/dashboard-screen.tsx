"use client";

import { Boxes, Cable, MonitorCog, Route } from "lucide-react";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { McpEndpointCard } from "@/components/console/mcp-endpoint-card";
import {
  DemoModeNotice,
  OfficialGatewayNotice,
  SelfHostModeNotice,
} from "@/components/console/mode-notices";
import { WorkspaceSummaryCard } from "@/components/console/workspace-summary-card";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CommandBlock } from "@/components/ui/code-block";
import { LoadingPanel } from "@/components/ui/skeleton";
import { StatCard } from "@/components/ui/stat-card";
import { countBy } from "@/lib/format";
import { isDemoMode } from "@/api/config";
import { useActivationCommands } from "@/api/activation";
import { useAuditEvents } from "@/api/audit";
import { useConnectors } from "@/api/connectors";
import { useMachines } from "@/api/machines";
import { useProjects } from "@/api/projects";
import { useRelayProfiles } from "@/api/relays";
import { useWorkspace } from "@/api/workspace";

export function DashboardScreen() {
  const workspace = useWorkspace();
  const relays = useRelayProfiles();
  const machines = useMachines();
  const connectors = useConnectors();
  const projects = useProjects();
  const audit = useAuditEvents();
  const activation = useActivationCommands();
  const queries = [
    workspace,
    relays,
    machines,
    connectors,
    projects,
    audit,
    activation,
  ];
  const firstError = queries.find((query) => query.error)?.error;
  if (queries.some((query) => query.isLoading))
    return (
      <DashboardShell>
        <LoadingPanel />
      </DashboardShell>
    );
  if (firstError) {
    return (
      <DashboardShell>
        <Alert title="Gateway Console live data unavailable" tone="error">
          {firstError.message}
        </Alert>
      </DashboardShell>
    );
  }
  const workspaceData = workspace.data!;
  const relayData = relays.data!.relays;
  const machineData = machines.data!.machines;
  const connectorData = connectors.data!.connectors;
  const projectData = projects.data!.projects;
  const auditData = audit.data!.auditEvents;
  const activationData = activation.data!.activationCommands;
  return (
    <DashboardShell>
      <div className="grid min-w-0 max-w-full gap-lg">
        {isDemoMode() ? <DemoModeNotice /> : null}
        <div className="grid min-w-0 gap-md lg:grid-cols-3">
          <OfficialGatewayNotice />
          <SelfHostModeNotice />
          <McpEndpointCard endpoint={workspaceData.mcpEndpoint} />
        </div>
        <div className="grid min-w-0 gap-md md:grid-cols-2 xl:grid-cols-4">
          <StatCard
            description="Enabled Gateway backend profiles."
            icon={Cable}
            label="Relays"
            value={countBy(relayData, (relay) => relay.enabled)}
          />
          <StatCard
            description="Machines with explicit connector bindings."
            icon={MonitorCog}
            label="Connectors"
            value={connectorData.length}
          />
          <StatCard
            description="Shared projects visible through Relay profiles."
            icon={Boxes}
            label="Projects"
            value={projectData.length}
          />
          <StatCard
            description="Locations that need deterministic selectors."
            icon={Route}
            label="Ambiguities"
            value={
              projectData
                .flatMap((p) => p.locations)
                .filter((loc) => loc.ambiguity !== "none").length
            }
          />
        </div>
        <div className="grid min-w-0 gap-lg xl:grid-cols-[1fr_1.2fr]">
          <WorkspaceSummaryCard
            connectorCount={connectorData.length}
            projectCount={projectData.length}
            relayCount={relayData.length}
            user={workspaceData.user}
            workspace={workspaceData.workspace}
          />
          <Card>
            <CardHeader>
              <CardTitle>Next activation step</CardTitle>
            </CardHeader>
            <CardContent>
              <CommandBlock
                command={
                  activationData[0]?.command ??
                  `codencer login --gateway ${workspaceData.publicBaseURL}`
                }
              />
            </CardContent>
          </Card>
        </div>
        <Card>
          <CardHeader>
            <CardTitle>Recent audit</CardTitle>
          </CardHeader>
          <CardContent>
            <AuditEventTimeline events={auditData.slice(0, 3)} />
          </CardContent>
        </Card>
        <span className="sr-only">{machineData.length} machines loaded</span>
      </div>
    </DashboardShell>
  );
}

function DashboardShell({ children }: { children: React.ReactNode }) {
  return (
    <PageShell
      breadcrumbs={[{ label: "Console" }]}
      description="Operational overview for the public Gateway Console foundation."
      kicker="Gateway Console"
      title="Self-host bridge status"
    >
      {children}
    </PageShell>
  );
}
