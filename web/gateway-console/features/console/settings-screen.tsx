"use client";

import { ThemeToggle } from "@/components/layout/theme-toggle";
import { PageShell } from "@/components/layout/page-shell";
import { Alert } from "@/components/ui/alert";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KeyValueList } from "@/components/ui/key-value-list";
import { LoadingPanel } from "@/components/ui/skeleton";
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
      description="Workspace metadata, runtime endpoints, and local Console preferences."
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
                  { label: "Type", value: workspace.data.workspace.kind },
                  { label: "MCP endpoint", value: workspace.data.mcpEndpoint },
                  {
                    label: "Public base",
                    value: workspace.data.publicBaseURL,
                  },
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
        </div>
      ) : null}
    </PageShell>
  );
}
