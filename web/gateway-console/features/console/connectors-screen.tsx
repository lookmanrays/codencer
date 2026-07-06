"use client";

import { MachineConnectorTable } from "@/components/console/machine-connector-table";
import { DemoModeNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";
import { isDemoMode } from "@/api/config";
import { useConnectors } from "@/api/connectors";
import { useMachines } from "@/api/machines";

export function ConnectorsScreen() {
  const machines = useMachines();
  const connectors = useConnectors();
  const error = machines.error ?? connectors.error;
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
      {machines.isLoading || connectors.isLoading ? <LoadingPanel /> : null}
      {error ? (
        <Alert title="Connector API unavailable" tone="danger">
          {error.message}
        </Alert>
      ) : null}
      {machines.data && connectors.data ? (
        <div className="grid min-w-0 max-w-full gap-lg">
          {isDemoMode() ? <DemoModeNotice /> : null}
          {connectors.data.connectors.length === 0 ? (
            <EmptyState
              description="Run connector login to bind a machine."
              title="No connectors"
            />
          ) : (
            <MachineConnectorTable
              connectors={connectors.data.connectors}
              machines={machines.data.machines}
            />
          )}
          <Card>
            <CardHeader>
              <CardTitle>Machine identity rule</CardTitle>
            </CardHeader>
            <CardContent>
              <KeyValueList
                items={[
                  {
                    label: "machine_id",
                    value: "Stable local identity stored under CODENCER_HOME.",
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
      ) : null}
    </PageShell>
  );
}
