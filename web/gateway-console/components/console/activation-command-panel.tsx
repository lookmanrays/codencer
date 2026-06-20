import { CommandBlock } from "@/components/ui/code-block";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { ActivationCommand } from "@/schemas/console";

export function ActivationCommandPanel({ commands }: { commands: ActivationCommand[] }) {
  const groups = {
    gateway: commands.filter((command) => command.target === "gateway"),
    local: commands.filter((command) => command.target === "local"),
    client: commands.filter((command) => command.target === "client"),
  };
  return (
    <Tabs defaultValue="gateway">
      <TabsList>
        <TabsTrigger value="gateway">Gateway</TabsTrigger>
        <TabsTrigger value="local">Local</TabsTrigger>
        <TabsTrigger value="client">MCP clients</TabsTrigger>
      </TabsList>
      {Object.entries(groups).map(([key, values]) => (
        <TabsContent className="grid gap-md" key={key} value={key}>
          {values.map((command) => (
            <CommandBlock command={command.command} key={command.id} title={command.title} />
          ))}
        </TabsContent>
      ))}
    </Tabs>
  );
}
