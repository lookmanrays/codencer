import { StatusBadge } from "@/components/ui/badge";
import type { RelayProfile } from "@/schemas/relays";

export function RelayHealthBadge({ relay }: { relay: RelayProfile }) {
  return <StatusBadge status={relay.enabled ? relay.status : "disabled"} />;
}
