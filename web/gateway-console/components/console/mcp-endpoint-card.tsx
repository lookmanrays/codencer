import { Cable } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { CopyButton } from "@/components/ui/copy-button";
import { KeyValueList } from "@/components/ui/key-value-list";

export function McpEndpointCard({ endpoint }: { endpoint: string }) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-start justify-between gap-md">
          <div>
            <CardTitle className="flex items-center gap-sm">
              <Cable aria-hidden="true" className="h-5 w-5 text-accent" />
              MCP endpoint
            </CardTitle>
          </div>
          <CopyButton label="Copy MCP endpoint" value={endpoint} />
        </div>
      </CardHeader>
      <CardContent>
        <KeyValueList
          items={[
            { label: "Endpoint", value: <code>{endpoint}</code> },
            {
              label: "Client rule",
              value: "ChatGPT, Claude Code, and Codex connect to Gateway.",
            },
            {
              label: "Relay token",
              value: "Resolved server-side; never shown to AI clients.",
            },
          ]}
        />
      </CardContent>
    </Card>
  );
}
