"use client";

import { RelayProfileCard } from "@/components/console/relay-profile-card";
import { RelayProfileForm } from "@/components/console/relay-profile-form";
import { OfficialGatewayNotice } from "@/components/console/mode-notices";
import { PageShell } from "@/components/layout/page-shell";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ConsoleData } from "@/features/console/use-console-data";

export function RelaysScreen() {
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
      <ConsoleData
        emptyDescription="Add a backend Relay profile to route projects."
        emptyTitle="No Relay profiles"
      >
        {(snapshot) => (
          <div className="grid gap-lg">
            <OfficialGatewayNotice />
            <div className="grid gap-md lg:grid-cols-2">
              {snapshot.relays.map((relay) => (
                <RelayProfileCard key={relay.id} relay={relay} />
              ))}
            </div>
            <Card>
              <CardHeader>
                <CardTitle>Add self-host Relay profile</CardTitle>
              </CardHeader>
              <CardContent>
                <RelayProfileForm />
              </CardContent>
            </Card>
          </div>
        )}
      </ConsoleData>
    </PageShell>
  );
}
