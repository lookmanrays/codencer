"use client";

import type { ColumnDef } from "@tanstack/react-table";
import { Plus } from "lucide-react";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
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
    {
      header: "Name",
      cell: ({ row }) => (
        <span className="block min-w-0 truncate" title={row.original.id}>
          {row.original.name}
        </span>
      ),
    },
    {
      header: "Type",
      cell: ({ row }) => <Badge>{row.original.type}</Badge>,
    },
    {
      header: "URL",
      cell: ({ row }) => (
        <span className="block max-w-[320px] truncate" title={row.original.url}>
          {row.original.url}
        </span>
      ),
    },
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
        row.original.tokenConfigured ? (
          <span
            className="block max-w-[240px] truncate"
            title={row.original.tokenRef}
          >
            {row.original.tokenRef}
          </span>
        ) : (
          "not configured"
        ),
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
          <Card>
            <CardHeader>
              <div className="flex min-w-0 flex-wrap items-center justify-between gap-md">
                <CardTitle>Relay profiles</CardTitle>
                <Dialog>
                  <DialogTrigger asChild>
                    <Button size="sm">
                      <Plus aria-hidden="true" className="h-4 w-4" />
                      Add relay profile
                    </Button>
                  </DialogTrigger>
                  <DialogContent>
                    <DialogTitle>Add self-host Relay profile</DialogTitle>
                    <DialogDescription className="mt-xs block text-body-sm text-ink-secondary">
                      Store planner token material on the Gateway host. The
                      Console only sends a token environment variable reference.
                    </DialogDescription>
                    <div className="mt-md">
                      <RelayProfileForm />
                    </div>
                  </DialogContent>
                </Dialog>
              </div>
            </CardHeader>
            <CardContent>
              {relays.data.relays.length === 0 ? (
                <EmptyState
                  description="Add a backend Relay profile to route projects."
                  title="No Relay profiles"
                />
              ) : (
                <DataTable
                  columns={columns}
                  data={relays.data.relays}
                  density="compact"
                  minWidth="700px"
                />
              )}
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Relay token handling</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="m-0 text-body text-ink-secondary">
                Token values stay server-side. This page shows only masked token
                state or token environment variable references.
              </p>
            </CardContent>
          </Card>
        </div>
      ) : null}
    </PageShell>
  );
}
