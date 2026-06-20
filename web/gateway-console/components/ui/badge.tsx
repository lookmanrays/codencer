import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/cn";

const badgeVariants = cva(
  "inline-flex min-h-6 items-center gap-xs rounded-[2px] border px-2 py-0.5 font-mono text-mono tracking-[0.04em]",
  {
    variants: {
      variant: {
        neutral: "border-border bg-paper text-ink-secondary",
        accent: "border-accent bg-accent-tint-bg text-ink-primary",
        success: "border-success bg-paper text-success",
        warning: "border-warning bg-paper text-warning",
        error: "border-error bg-paper text-error",
        dark: "border-ink-primary bg-ink-primary text-paper",
      },
    },
    defaultVariants: { variant: "neutral" },
  },
);

export type BadgeProps = React.HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>;

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export function StatusBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase();
  const variant =
    normalized.includes("online") ||
    normalized.includes("active") ||
    normalized.includes("available") ||
    normalized.includes("healthy")
      ? "success"
      : normalized.includes("pending") || normalized.includes("warning")
        ? "warning"
        : normalized.includes("error") ||
            normalized.includes("down") ||
            normalized.includes("unavailable") ||
            normalized.includes("disabled")
          ? "error"
          : "neutral";
  return <Badge variant={variant}>{status}</Badge>;
}
