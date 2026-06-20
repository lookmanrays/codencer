import { CopyButton } from "@/components/ui/copy-button";
import { sanitizeForDisplay } from "@/lib/redaction";
import { cn } from "@/lib/cn";

type CodeBlockProps = {
  code: string;
  language?: string;
  copyValue?: string;
  className?: string;
  variant?: "code" | "terminal";
};

export function CodeBlock({
  className,
  code,
  copyValue,
  language = "text",
  variant = "code",
}: CodeBlockProps) {
  const safeCode = sanitizeForDisplay(code);
  const terminal = variant === "terminal";
  return (
    <div
      className={cn(
        "min-w-0 max-w-full overflow-hidden rounded-[var(--radius-card)] border border-border text-code-fg shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]",
        terminal ? "bg-[#0f1419] text-[#e8e6e1]" : "bg-code-bg",
        className,
      )}
    >
      <div
        className={cn(
          "flex min-w-0 items-center justify-between gap-sm border-b border-border-strong px-md py-sm",
          terminal ? "bg-white/5" : "bg-paper-tinted/70",
        )}
      >
        <span
          className={cn(
            "min-w-0 truncate font-mono text-mono uppercase tracking-[0.12em]",
            terminal ? "text-[#c9c9ce]" : "text-ink-muted",
          )}
        >
          {language}
        </span>
        <CopyButton label="Copy code" value={copyValue ?? safeCode} />
      </div>
      <pre className="m-0 min-w-0 max-w-full overflow-x-auto p-sm font-mono text-mono leading-[1.65] md:p-md">
        <code className="block min-w-0">{safeCode}</code>
      </pre>
    </div>
  );
}

export function CommandBlock({
  command,
  title,
  variant = "code",
}: {
  command: string;
  title?: string;
  variant?: "code" | "terminal";
}) {
  return (
    <div className="min-w-0 max-w-full">
      {title ? (
        <p className="mb-xs mt-0 min-w-0 break-words font-mono text-mono tracking-[0.04em] text-ink-muted">
          {title}
        </p>
      ) : null}
      <CodeBlock
        code={command}
        copyValue={command}
        language="shell"
        variant={variant}
      />
    </div>
  );
}
