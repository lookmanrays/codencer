import { Timeline } from "@/components/ui/timeline";
import { formatDateTime } from "@/lib/format";
import type { AuditEvent } from "@/schemas/console";

export function AuditEventTimeline({ events }: { events: AuditEvent[] }) {
  return (
    <Timeline
      items={events.map((event) => ({
        id: event.id,
        title: event.type,
        description: event.summary,
        status: event.severity,
        time: `${formatDateTime(event.createdAt)} · ${event.actor}`,
      }))}
    />
  );
}
