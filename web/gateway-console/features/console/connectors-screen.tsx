"use client";

import { MachineConnectorTable } from "@/components/console/machine-connector-table";
import { PageShell } from "@/components/layout/page-shell";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KeyValueList } from "@/components/ui/key-value-list";
import { ConsoleData } from "@/features/console/use-console-data";

export function ConnectorsScreen() {
  return (
    <PageShell
      breadcrumbs={[
        { label: "Console", href: "/console" },
        { label: "Machines" },
      ]}
      description="Inspect machine identity, host labels, connector bindings, and last-seen status."
      kicker="Machines / connectors"
      title="Local execution endpoints"
    >
      <ConsoleData
        emptyDescription="Run connector login to bind a machine."
        emptyTitle="No connectors"
      >
        {(snapshot) => (
          <div className="grid min-w-0 max-w-full gap-lg">
            <MachineConnectorTable
              connectors={snapshot.connectors}
              machines={snapshot.machines}
            />
            <Card>
              <CardHeader>
                <CardTitle>Machine identity rule</CardTitle>
              </CardHeader>
              <CardContent>
                <KeyValueList
                  items={[
                    {
                      label: "machine_id",
                      value:
                        "Stable local identity stored under CODENCER_HOME.",
                    },
                    {
                      label: "host_label",
                      value: "Editable human selector used for routing.",
                    },
                    {
                      label: "repo paths",
                      value: "Never exposed; only labels and hashes are shown.",
                    },
                  ]}
                />
              </CardContent>
            </Card>
          </div>
        )}
      </ConsoleData>
    </PageShell>
  );
}
