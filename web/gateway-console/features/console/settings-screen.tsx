"use client";

import { ThemeToggle } from "@/components/layout/theme-toggle";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { isDemoMode } from "@/api/config";
import { useWorkspace } from "@/api/workspace";
import { DemoModeNotice } from "@/components/console/mode-notices";

export function SettingsScreen() {
  const workspace = useWorkspace();
  return (
    <PageShell
      breadcrumbs={[
        { label: "Console", href: "/console" },
        { label: "Settings" },
      ]}
      description="Workspace metadata, endpoints, theme, and future token-management placeholders."
      kicker="Settings"
      title="Console settings"
    >
      {workspace.isLoading ? <LoadingPanel /> : null}
      {workspace.error ? (
        <Alert title="Workspace API unavailable" tone="danger">
          {workspace.error.message}
        </Alert>
      ) : null}
      {workspace.data ? (
        <div className="grid min-w-0 max-w-full gap-lg">
          {isDemoMode() ? <DemoModeNotice /> : null}
          <Card>
            <CardHeader>
              <CardTitle>Workspace</CardTitle>
            </CardHeader>
            <CardContent>
              <KeyValueList
                items={[
                  { label: "Workspace", value: workspace.data.workspace.name },
                  { label: "MCP endpoint", value: workspace.data.mcpEndpoint },
                  {
                    label: "Mode",
                    value: workspace.data.workspace.mode,
                  },
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
              <span className="text-body-sm text-ink-secondary">
                Toggle light/dark console theme.
              </span>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Token revocation</CardTitle>
            </CardHeader>
            <CardContent className="flex items-center justify-between gap-md">
              <span className="text-body-sm text-ink-secondary">
                Placeholder only. Production token revocation UI/API is
                future/private.
              </span>
              <Switch disabled />
            </CardContent>
          </Card>
          <Alert title="Future private Cloud features" tone="warning">
            Billing, team invites, support/admin console, hosted provider login,
            KMS/Vault, and managed runners are intentionally not implemented
            here.
          </Alert>
        </div>
      ) : null}
    </PageShell>
  );
}
