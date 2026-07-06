import { StatusBadge } from "@/components/ui/badge";

export type TimelineItem = {
  href?: string;
  id: string;
  title: string;
  description: string;
  time: string;
  status: string;
};

export function Timeline({ items }: { items: TimelineItem[] }) {
  return (
    <ol className="m-0 grid min-w-0 list-none gap-md p-0">
      {items.map((item) => (
        <li
          className="grid min-w-0 grid-cols-[18px_minmax(0,1fr)] gap-md"
          key={item.id}
        >
          <span
            aria-hidden="true"
            className="mt-[0.55rem] h-2 w-2 rounded-full bg-accent"
          />
          <div className="min-w-0 border-b border-border pb-md">
            <div className="flex min-w-0 flex-wrap items-start justify-between gap-sm">
              <div className="min-w-0">
                <p className="m-0 font-semibold">
                  {item.href ? (
                    <a className="text-ink-primary underline" href={item.href}>
                      {item.title}
                    </a>
                  ) : (
                    item.title
                  )}
                </p>
                <p className="m-0 mt-xs text-body-sm text-ink-secondary">
                  {item.description}
                </p>
              </div>
              <StatusBadge status={item.status} />
            </div>
            <p className="m-0 mt-sm font-mono text-mono tracking-[0.04em] text-ink-muted">
              {item.time}
            </p>
          </div>
        </li>
      ))}
    </ol>
  );
}
