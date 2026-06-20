"use client";

import { ActivationCommandPanel } from "@/components/console/activation-command-panel";
import { OfficialGatewayNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { ConsoleData } from "@/features/console/use-console-data";

export function ActivationScreen() {
  return (
    <PageShell
      breadcrumbs={[
        { label: "Console", href: "/console" },
        { label: "Activation" },
      ]}
      description="Copy safe Gateway-first commands for CLI login, connector binding, project sharing, MCP setup, and smoke checks."
      kicker="Activation"
      title="Gateway-first setup"
    >
      <ConsoleData
        emptyDescription="Activation commands are generated from the console data layer."
        emptyTitle="No activation commands"
      >
        {(snapshot) => (
          <div className="grid gap-lg">
            <OfficialGatewayNotice />
            <ActivationCommandPanel commands={snapshot.activationCommands} />
          </div>
        )}
      </ConsoleData>
    </PageShell>
  );
}
