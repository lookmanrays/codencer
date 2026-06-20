import { cn } from "@/lib/cn";

export type KeyValueItem = {
  label: string;
  value: React.ReactNode;
};

export function KeyValueList({ className, items }: { className?: string; items: KeyValueItem[] }) {
  return (
    <dl className={cn("grid gap-sm", className)}>
      {items.map((item) => (
        <div className="grid grid-cols-[160px_1fr] gap-md border-t border-border py-sm first:border-t-0 max-sm:grid-cols-1 max-sm:gap-xs" key={item.label}>
          <dt className="font-mono text-mono uppercase tracking-[0.12em] text-ink-muted">{item.label}</dt>
          <dd className="m-0 text-body-sm text-ink-primary">{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
