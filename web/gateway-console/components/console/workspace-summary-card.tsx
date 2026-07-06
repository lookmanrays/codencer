import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KeyValueList } from "@/components/ui/key-value-list";
import type { User, Workspace } from "@/schemas/workspace";

export function WorkspaceSummaryCard({
  connectorCount,
  projectCount,
  relayCount,
  user,
  workspace,
}: {
  connectorCount: number;
  projectCount: number;
  relayCount: number;
  user: User;
  workspace: Workspace;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{workspace.name}</CardTitle>
      </CardHeader>
      <CardContent>
        <KeyValueList
          items={[
            {
              label: "Mode",
              value: workspace.mode,
            },
            { label: "User", value: user.email },
            { label: "Relays", value: relayCount },
            { label: "Connectors", value: connectorCount },
            { label: "Projects", value: projectCount },
          ]}
        />
      </CardContent>
    </Card>
  );
}
