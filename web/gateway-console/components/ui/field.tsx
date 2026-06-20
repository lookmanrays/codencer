import type { ReactNode } from "react";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/cn";

type FieldProps = {
  id: string;
  label: string;
  children: ReactNode;
  description?: string;
  error?: string;
  className?: string;
};

export function Field({
  children,
  className,
  description,
  error,
  id,
  label,
}: FieldProps) {
  return (
    <div className={cn("flex min-w-0 flex-col gap-[6px]", className)}>
      <Label htmlFor={id}>{label}</Label>
      {children}
      {description && !error ? (
        <p className="m-0 min-w-0 break-words text-body-sm text-ink-secondary">
          {description}
        </p>
      ) : null}
      {error ? <FormMessage id={`${id}-message`}>{error}</FormMessage> : null}
    </div>
  );
}

export function FormMessage({
  className,
  ...props
}: React.HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p
      className={cn(
        "m-0 font-mono text-mono tracking-[0.04em] text-danger",
        className,
      )}
      {...props}
    />
  );
}
