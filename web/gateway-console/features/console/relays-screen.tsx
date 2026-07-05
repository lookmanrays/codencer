"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { RelayProfileForm } from "@/components/console/relay-profile-form";
import {
  DemoModeNotice,
  OfficialGatewayNotice,
} from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Badge, StatusBadge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { DataTable } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { LoadingPanel } from "@/components/ui/skeleton";
import { isDemoMode } from "@/api/config";
import { useDeleteRelayProfile, useRelayProfiles } from "@/api/relays";
import { useWorkspace } from "@/api/workspace";
import type { RelayProfile } from "@/schemas/relays";

export function RelaysScreen() {
  const relays = useRelayProfiles();
  const workspace = useWorkspace();
  const deleteRelay = useDeleteRelayProfile();
  const columns: ColumnDef<RelayProfile>[] = [
    { header: "ID", accessorKey: "id" },
    { header: "Name", accessorKey: "name" },
    {
      header: "Type",
      cell: ({ row }) => <Badge>{row.original.type}</Badge>,
    },
    { header: "URL", accessorKey: "url" },
    {
      header: "Status",
      cell: ({ row }) => <StatusBadge status={row.original.status} />,
    },
    {
      header: "Enabled",
      cell: ({ row }) => (row.original.enabled ? "yes" : "no"),
    },
    {
      header: "Token reference",
      cell: ({ row }) =>
        row.original.tokenConfigured ? row.original.tokenRef : "not configured",
    },
    {
      header: "Actions",
      cell: ({ row }) => (
        <Button
          disabled={deleteRelay.isPending || row.original.id === "default"}
          onClick={() => deleteRelay.mutate(row.original.id)}
          size="sm"
          type="button"
          variant="quiet"
        >
          Remove
        </Button>
      ),
    },
  ];
  return (
    <PageShell
      breadcrumbs={[
        { label: "Console", href: "/console" },
        { label: "Relays" },
      ]}
      description="Manage Gateway backend Relay profiles without exposing Relay bearer tokens to AI clients."
      kicker="Relay profiles"
      title="Gateway routing backends"
    >
      {relays.isLoading ? <LoadingPanel /> : null}
      {workspace.isLoading ? <LoadingPanel /> : null}
      {relays.error ? (
        <Alert title="Relay API unavailable" tone="danger">
          {relays.error.message}
        </Alert>
      ) : null}
      {workspace.error ? (
        <Alert title="Workspace API unavailable" tone="danger">
          {workspace.error.message}
        </Alert>
      ) : null}
      {relays.data && workspace.data ? (
        <div className="grid min-w-0 max-w-full gap-lg">
          {isDemoMode() ? <DemoModeNotice /> : null}
          <OfficialGatewayNotice mcpEndpoint={workspace.data.mcpEndpoint} />
          <div className="grid min-w-0 gap-lg xl:grid-cols-[minmax(0,1fr)_360px]">
            <div className="min-w-0">
              {relays.data.relays.length === 0 ? (
                <EmptyState
                  description="Add a backend Relay profile to route projects."
                  title="No Relay profiles"
                />
              ) : (
                <DataTable columns={columns} data={relays.data.relays} />
              )}
            </div>
            <Card className="self-start">
              <CardHeader>
                <CardTitle>Add self-host Relay profile</CardTitle>
              </CardHeader>
              <CardContent>
                <RelayProfileForm />
              </CardContent>
            </Card>
          </div>
        </div>
      ) : null}
    </PageShell>
  );
}
