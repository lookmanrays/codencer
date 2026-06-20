"use client";

import { RelayProfileCard } from "@/components/console/relay-profile-card";
import { RelayProfileForm } from "@/components/console/relay-profile-form";
import {
  DemoModeNotice,
  OfficialGatewayNotice,
} from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { LoadingPanel } from "@/components/ui/skeleton";
import { isDemoMode } from "@/api/config";
import { useRelayProfiles } from "@/api/relays";

export function RelaysScreen() {
  const relays = useRelayProfiles();
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
      {relays.error ? (
        <Alert title="Relay API unavailable" tone="danger">
          {relays.error.message}
        </Alert>
      ) : null}
      {relays.data ? (
        <div className="grid min-w-0 max-w-full gap-lg">
          {isDemoMode() ? <DemoModeNotice /> : null}
          <OfficialGatewayNotice />
          {relays.data.relays.length === 0 ? (
            <EmptyState
              description="Add a backend Relay profile to route projects."
              title="No Relay profiles"
            />
          ) : (
            <div className="grid min-w-0 gap-md lg:grid-cols-2">
              {relays.data.relays.map((relay) => (
                <RelayProfileCard key={relay.id} relay={relay} />
              ))}
            </div>
          )}
          <Card>
            <CardHeader>
              <CardTitle>Add self-host Relay profile</CardTitle>
            </CardHeader>
            <CardContent>
              <RelayProfileForm />
            </CardContent>
          </Card>
        </div>
      ) : null}
    </PageShell>
  );
}
