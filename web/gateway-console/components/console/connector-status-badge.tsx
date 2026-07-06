import { StatusBadge } from "@/components/ui/badge";
import type { Connector } from "@/schemas/connectors";

export function ConnectorStatusBadge({ connector }: { connector: Connector }) {
  return <StatusBadge status={connector.status} />;
}
