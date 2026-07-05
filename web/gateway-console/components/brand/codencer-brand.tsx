import { cn } from "@/lib/cn";

const bars = [
  { x: 0, width: 60, lit: true },
  { x: 82, width: 30, lit: false },
  { x: 134, width: 46, lit: true },
  { x: 202, width: 16, lit: false },
] as const;

export function CodencerMark({
  className,
  onDark = false,
}: {
  className?: string;
  onDark?: boolean;
}) {
  const accent = onDark ? "#FF6A2C" : "#FF5A1F";
  const ink = onDark ? "#EDEDE6" : "var(--color-ink-primary)";
  return (
    <span
      aria-label="Codencer"
      className={cn("relative block h-[18px] w-[48px] shrink-0", className)}
      role="img"
    >
      {bars.map((bar, index) => (
        <span
          aria-hidden="true"
          className="absolute top-0 h-full rounded-[2px]"
          key={index}
          style={{
            backgroundColor: bar.lit ? accent : ink,
            left: `${(bar.x / 218) * 100}%`,
            width: `${(bar.width / 218) * 100}%`,
          }}
        />
      ))}
    </span>
  );
}

export function CodencerLockup({ className }: { className?: string }) {
  return (
    <span className={cn("flex min-w-0 items-center gap-sm", className)}>
      <CodencerMark className="h-[20px] w-[53px]" />
      <span className="min-w-0 truncate font-mono text-h3 font-semibold leading-none text-ink-primary">
        codencer
      </span>
    </span>
  );
}
