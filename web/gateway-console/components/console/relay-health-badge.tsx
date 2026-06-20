import { StatusBadge } from "@/components/ui/badge";
import type { RelayProfile } from "@/schemas/console";

export function RelayHealthBadge({ relay }: { relay: RelayProfile }) {
  return <StatusBadge status={relay.enabled ? relay.status : "disabled"} />;
}
