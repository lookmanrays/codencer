import { Alert } from "@/components/ui/alert";

export function OfficialGatewayNotice() {
  return (
    <Alert title="Official connector path" tone="brand">
      AI clients point to Gateway at <code>https://mcp.codencer.dev/mcp</code>.
      Gateway routes to the default managed Relay or to user-added self-host
      Relay profiles.
    </Alert>
  );
}

export function SelfHostModeNotice() {
  return (
    <Alert title="Self-host boundary">
      Direct self-host Relay MCP remains available for personal, corporate, and
      debug use. It is not the primary official connector endpoint.
    </Alert>
  );
}

export function DemoModeNotice() {
  return (
    <Alert title="Demo data mode" tone="info">
      This console is rendering explicit demo data because{" "}
      <code>NEXT_PUBLIC_CODENCER_CONSOLE_MODE=demo</code> is set. Live mode is
      the default and never falls back to demo fixtures.
    </Alert>
  );
}
