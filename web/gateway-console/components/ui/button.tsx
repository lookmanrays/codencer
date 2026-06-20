import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/cn";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-sm rounded-[var(--radius-card)] font-semibold no-underline transition-[background,color,border-color] duration-150 ease-[ease] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:cursor-not-allowed disabled:opacity-40",
  {
    variants: {
      variant: {
        primary:
          "border border-accent bg-accent text-accent-contrast hover:border-accent-hover hover:bg-accent-hover",
        secondary:
          "border border-ink-primary bg-transparent text-ink-primary hover:bg-ink-primary hover:text-paper",
        quiet:
          "border border-border bg-paper-strong text-ink-primary hover:border-ink-primary",
        danger:
          "border border-error bg-transparent text-error hover:bg-error hover:text-paper",
        link: "border-0 bg-transparent p-0 font-mono font-normal tracking-[0.04em] text-ink-primary hover:text-accent",
      },
      size: {
        sm: "min-h-8 px-3 py-1.5 text-body-sm",
        md: "min-h-10 px-4 py-2 text-body-sm",
        lg: "min-h-12 px-5 py-3 text-body-sm",
        icon: "h-9 w-9 p-0",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  },
);

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  };

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ asChild = false, className, size, variant, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        className={cn(buttonVariants({ size, variant }), className)}
        ref={ref}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";

export { buttonVariants };
