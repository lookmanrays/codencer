import type { LucideIcon } from "lucide-react";
import { Box } from "lucide-react";
import { cn } from "@/lib/cn";

type EmptyStateProps = {
  title: string;
  description: string;
  icon?: LucideIcon;
  action?: React.ReactNode;
  className?: string;
};

export function EmptyState({
  action,
  className,
  description,
  icon: Icon = Box,
  title,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex min-h-[220px] flex-col items-start justify-center rounded-[var(--radius-card)] border border-dashed border-border-strong bg-paper-tinted p-lg",
        className,
      )}
      data-testid="empty-state"
    >
      <Icon aria-hidden="true" className="h-6 w-6 text-accent" />
      <h3 className="mb-0 mt-md text-h3 font-bold">{title}</h3>
      <p className="mb-0 mt-sm max-w-[60ch] text-body text-ink-secondary">
        {description}
      </p>
      {action ? <div className="mt-md">{action}</div> : null}
    </div>
  );
}
