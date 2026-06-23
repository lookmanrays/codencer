import { Suspense } from "react";
import { OAuthConsentPanel } from "@/components/console/oauth-consent-panel";
import { AuthShell } from "@/components/layout/auth-shell";
import { LoadingPanel } from "@/components/ui/skeleton";

export default function OAuthAuthorizePage() {
  return (
    <AuthShell
      description="Review client, workspace, resource, and scopes before consenting to Gateway MCP access."
      kicker="OAuth dev consent"
      title="Authorize Gateway MCP access"
    >
      <Suspense fallback={<LoadingPanel />}>
        <OAuthConsentPanel />
      </Suspense>
    </AuthShell>
  );
}
