import { cn } from "@/lib/cn";

export function Kicker({
  className,
  label,
}: {
  className?: string;
  label: string;
}) {
  return (
    <div className={cn("inline-flex items-center gap-sm font-sans", className)}>
      <span aria-hidden="true" className="h-[1.5px] w-6 flex-none bg-accent" />
      <span className="text-kicker font-bold uppercase tracking-[0.12em]">
        {label}
      </span>
    </div>
  );
}
