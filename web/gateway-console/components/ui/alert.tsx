import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/cn";

type AlertProps = {
  title: string;
  children: ReactNode;
  tone?: "neutral" | "info" | "success" | "warning" | "danger" | "brand";
  className?: string;
};

const tones = {
  neutral: "border-border bg-paper-strong",
  info: "border-info/50 bg-info-soft",
  success: "border-success/50 bg-success-soft",
  warning: "border-warning/50 bg-warning-soft",
  danger: "border-danger/50 bg-danger-soft",
  brand: "border-accent/50 bg-accent-tint-bg",
};

const toneBadges = {
  neutral: "neutral",
  info: "info",
  success: "success",
  warning: "warning",
  danger: "danger",
  brand: "brand",
} as const;

export function Alert({
  children,
  className,
  title,
  tone = "neutral",
}: AlertProps) {
  return (
    <div
      className={cn(
        "min-w-0 max-w-full rounded-[var(--radius-card)] border p-md",
        tones[tone],
        className,
      )}
    >
      <Badge variant={toneBadges[tone]}>{title}</Badge>
      <div className="mt-sm min-w-0 break-words text-body-sm text-ink-secondary">
        {children}
      </div>
    </div>
  );
}
