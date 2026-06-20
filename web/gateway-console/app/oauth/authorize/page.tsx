import { OAuthConsentPanel } from "@/components/console/oauth-consent-panel";
import { PageHeader } from "@/components/layout/page-header";

export default function OAuthAuthorizePage() {
  return (
    <main
      className="min-h-dvh bg-paper px-[var(--container-pad)] py-xl"
      id="main-content"
    >
      <PageHeader
        description="Review client, workspace, resource, and scopes before consent."
        kicker="OAuth dev consent"
        title="Authorize Gateway MCP access"
      />
      <OAuthConsentPanel />
    </main>
  );
}
