import type { ReactNode } from "react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/cn";

type AlertProps = {
  title: string;
  children: ReactNode;
  tone?: "neutral" | "accent" | "warning" | "error";
  className?: string;
};

const tones = {
  neutral: "border-border bg-paper-strong",
  accent: "border-accent bg-accent-tint-bg",
  warning: "border-warning bg-paper-strong",
  error: "border-error bg-paper-strong",
};

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
      <Badge
        variant={
          tone === "error"
            ? "error"
            : tone === "warning"
              ? "warning"
              : tone === "accent"
                ? "accent"
                : "neutral"
        }
      >
        {title}
      </Badge>
      <div className="mt-sm min-w-0 break-words text-body-sm text-ink-secondary">
        {children}
      </div>
    </div>
  );
}
