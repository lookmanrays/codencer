import { CopyButton } from "@/components/ui/copy-button";
import { sanitizeForDisplay } from "@/lib/redaction";
import { cn } from "@/lib/cn";

type CodeBlockProps = {
  code: string;
  language?: string;
  copyValue?: string;
  className?: string;
};

export function CodeBlock({ className, code, copyValue, language = "text" }: CodeBlockProps) {
  const safeCode = sanitizeForDisplay(code);
  return (
    <div className={cn("overflow-hidden rounded-[var(--radius-card)] border border-border bg-code-bg text-code-fg", className)}>
      <div className="flex items-center justify-between border-b border-border-strong px-md py-sm">
        <span className="font-mono text-mono uppercase tracking-[0.12em] text-dark-muted">
          {language}
        </span>
        <CopyButton label="Copy code" value={copyValue ?? safeCode} />
      </div>
      <pre className="m-0 overflow-x-auto p-md font-mono text-mono leading-[1.7]">
        <code>{safeCode}</code>
      </pre>
    </div>
  );
}

export function CommandBlock({ command, title }: { command: string; title?: string }) {
  return (
    <div>
      {title ? <p className="mb-xs mt-0 font-mono text-mono tracking-[0.04em] text-ink-muted">{title}</p> : null}
      <CodeBlock code={command} copyValue={command} language="shell" />
    </div>
  );
}
