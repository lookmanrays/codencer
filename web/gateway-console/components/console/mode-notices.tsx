import { Alert } from "@/components/ui/alert";

export function OfficialGatewayNotice() {
  return (
    <Alert title="Gateway MCP path" tone="brand">
      AI clients point to Gateway at <code>http://127.0.0.1:19090/mcp</code>.
      Gateway routes to the default self-host Relay or to user-added Relay
      profiles.
    </Alert>
  );
}

export function SelfHostModeNotice() {
  return (
    <Alert title="Self-host boundary">
      Direct self-host Relay MCP remains available for personal, corporate, and
      debug use. Gateway MCP is the primary public self-host client endpoint.
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
