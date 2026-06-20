"use client";

import { Boxes, Cable, MonitorCog, Route } from "lucide-react";
import { AuditEventTimeline } from "@/components/console/audit-event-timeline";
import { McpEndpointCard } from "@/components/console/mcp-endpoint-card";
import { MockModeNotice, OfficialGatewayNotice, SelfHostModeNotice } from "@/components/console/mode-notices";
import { WorkspaceSummaryCard } from "@/components/console/workspace-summary-card";
import { PageShell } from "@/components/layout/page-shell";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CommandBlock } from "@/components/ui/code-block";
import { StatCard } from "@/components/ui/stat-card";
import { countBy } from "@/lib/format";
import { isMockMode } from "@/api/client";
import { ConsoleData } from "@/features/console/use-console-data";

export function DashboardScreen() {
  return (
    <PageShell
      breadcrumbs={[{ label: "Console" }]}
      description="Operational overview for the public Gateway Console foundation."
      kicker="Gateway Console"
      title="Self-host bridge status"
    >
      <ConsoleData>
        {(snapshot) => (
          <div className="grid gap-lg">
            {isMockMode() ? <MockModeNotice /> : null}
            <div className="grid gap-md lg:grid-cols-3">
              <OfficialGatewayNotice />
              <SelfHostModeNotice />
              <McpEndpointCard endpoint={snapshot.mcpEndpoint} />
            </div>
            <div className="grid gap-md md:grid-cols-2 xl:grid-cols-4">
              <StatCard description="Enabled Gateway backend profiles." icon={Cable} label="Relays" value={countBy(snapshot.relays, (relay) => relay.enabled)} />
              <StatCard description="Machines with explicit connector bindings." icon={MonitorCog} label="Connectors" value={snapshot.connectors.length} />
              <StatCard description="Shared projects visible through Relay profiles." icon={Boxes} label="Projects" value={snapshot.projects.length} />
              <StatCard description="Locations that need deterministic selectors." icon={Route} label="Ambiguities" value={snapshot.projects.flatMap((p) => p.locations).filter((loc) => loc.ambiguity !== "none").length} />
            </div>
            <div className="grid gap-lg xl:grid-cols-[1fr_1.2fr]">
              <WorkspaceSummaryCard snapshot={snapshot} />
              <Card>
                <CardHeader>
                  <CardTitle>Next activation step</CardTitle>
                </CardHeader>
                <CardContent>
                  <CommandBlock command={snapshot.activationCommands[0]?.command ?? "codencer login --gateway https://mcp.codencer.dev"} />
                </CardContent>
              </Card>
            </div>
            <Card>
              <CardHeader>
                <CardTitle>Recent audit</CardTitle>
              </CardHeader>
              <CardContent>
                <AuditEventTimeline events={snapshot.auditEvents.slice(0, 3)} />
              </CardContent>
            </Card>
          </div>
        )}
      </ConsoleData>
    </PageShell>
  );
}
