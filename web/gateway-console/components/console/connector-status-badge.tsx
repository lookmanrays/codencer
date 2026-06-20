import { StatusBadge } from "@/components/ui/badge";
import type { Connector } from "@/schemas/console";

export function ConnectorStatusBadge({ connector }: { connector: Connector }) {
  return <StatusBadge status={connector.status} />;
}
