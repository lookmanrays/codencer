import * as LabelPrimitive from "@radix-ui/react-label";
import * as React from "react";
import { cn } from "@/lib/cn";

export const Label = React.forwardRef<
  React.ElementRef<typeof LabelPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root>
>(({ className, ...props }, ref) => (
  <LabelPrimitive.Root
    className={cn("text-kicker font-bold uppercase tracking-[0.12em] text-ink-primary", className)}
    ref={ref}
    {...props}
  />
));
Label.displayName = LabelPrimitive.Root.displayName;
