import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { KeyValueList } from "@/components/ui/key-value-list";
import type { ConsoleSnapshot } from "@/schemas/console";

export function WorkspaceSummaryCard({ snapshot }: { snapshot: ConsoleSnapshot }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{snapshot.workspace.name}</CardTitle>
      </CardHeader>
      <CardContent>
        <KeyValueList
          items={[
            { label: "Mode", value: snapshot.workspace.mode.replaceAll("_", " ") },
            { label: "User", value: snapshot.user.email },
            { label: "Relays", value: snapshot.relays.length },
            { label: "Connectors", value: snapshot.connectors.length },
            { label: "Projects", value: snapshot.projects.length },
          ]}
        />
      </CardContent>
    </Card>
  );
}
