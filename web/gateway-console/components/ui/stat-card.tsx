import type { LucideIcon } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/cn";

type StatCardProps = {
  label: string;
  value: string | number;
  description: string;
  icon?: LucideIcon;
  className?: string;
};

export function StatCard({ className, description, icon: Icon, label, value }: StatCardProps) {
  return (
    <Card className={cn("border-l-[3px] border-l-accent", className)}>
      <CardContent>
        <div className="flex items-start justify-between gap-md">
          <div>
            <p className="m-0 font-mono text-mono uppercase tracking-[0.12em] text-ink-muted">
              {label}
            </p>
            <p className="m-0 mt-sm text-h2 font-bold leading-tight">{value}</p>
          </div>
          {Icon ? <Icon aria-hidden="true" className="h-5 w-5 text-accent" /> : null}
        </div>
        <p className="mb-0 mt-md text-body-sm text-ink-secondary">{description}</p>
      </CardContent>
    </Card>
  );
}
