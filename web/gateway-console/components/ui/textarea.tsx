import * as React from "react";
import { cn } from "@/lib/cn";

export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea
    className={cn(
      "min-h-[112px] w-full rounded-[var(--radius-card)] border border-border-strong bg-paper-strong px-3.5 py-3 text-body text-ink-primary transition-colors placeholder:text-ink-muted hover:border-ink-primary disabled:cursor-not-allowed disabled:opacity-50",
      className,
    )}
    ref={ref}
    {...props}
  />
));
Textarea.displayName = "Textarea";
