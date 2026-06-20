"use client";

import { ThemeToggle } from "@/components/layout/theme-toggle";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KeyValueList } from "@/components/ui/key-value-list";
import { Switch } from "@/components/ui/switch";
import { ConsoleData } from "@/features/console/use-console-data";

export function SettingsScreen() {
  return (
    <PageShell
      breadcrumbs={[{ label: "Console", href: "/console" }, { label: "Settings" }]}
      description="Workspace metadata, endpoints, theme, and future token-management placeholders."
      kicker="Settings"
      title="Console settings"
    >
      <ConsoleData>
        {(snapshot) => (
          <div className="grid gap-lg">
            <Card>
              <CardHeader>
                <CardTitle>Workspace</CardTitle>
              </CardHeader>
              <CardContent>
                <KeyValueList
                  items={[
                    { label: "Workspace", value: snapshot.workspace.name },
                    { label: "MCP endpoint", value: snapshot.mcpEndpoint },
                    { label: "Mode", value: snapshot.workspace.mode.replaceAll("_", " ") },
                  ]}
                />
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Theme</CardTitle>
              </CardHeader>
              <CardContent className="flex items-center gap-md">
                <ThemeToggle />
                <span className="text-body-sm text-ink-secondary">Toggle light/dark console theme.</span>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>Token revocation</CardTitle>
              </CardHeader>
              <CardContent className="flex items-center justify-between gap-md">
                <span className="text-body-sm text-ink-secondary">
                  Placeholder only. Production token revocation UI/API is future/private.
                </span>
                <Switch disabled />
              </CardContent>
            </Card>
            <Alert title="Future private Cloud features" tone="warning">
              Billing, team invites, support/admin console, hosted provider login, KMS/Vault, and
              managed runners are intentionally not implemented here.
            </Alert>
          </div>
        )}
      </ConsoleData>
    </PageShell>
  );
}
