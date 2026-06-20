import * as React from "react";
import { cn } from "@/lib/cn";

export const Input = React.forwardRef<
  HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>
>(({ className, type = "text", ...props }, ref) => (
  <input
    className={cn(
      "min-h-10 w-full rounded-[var(--radius-card)] border border-border-strong bg-paper-strong px-3.5 py-2 text-body text-ink-primary transition-colors placeholder:text-ink-muted hover:border-ink-primary disabled:cursor-not-allowed disabled:opacity-50",
      className,
    )}
    ref={ref}
    type={type}
    {...props}
  />
));
Input.displayName = "Input";
