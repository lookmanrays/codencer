"use client";

import { Eye, EyeOff } from "lucide-react";
import { useState } from "react";
import { IconButton } from "@/components/ui/icon-button";

export function SecretField({
  label,
  value = "redacted",
}: {
  label: string;
  value?: string;
}) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="flex items-center justify-between gap-sm rounded-[var(--radius-card)] border border-border bg-paper-strong p-sm">
      <div>
        <p className="m-0 font-mono text-mono uppercase tracking-[0.12em] text-ink-muted">
          {label}
        </p>
        <p className="m-0 font-mono text-mono text-ink-primary">
          {visible ? value.replace(/[A-Za-z0-9]/g, "*") : "••••••••••••"}
        </p>
      </div>
      <IconButton
        icon={visible ? EyeOff : Eye}
        label={visible ? "Hide redacted value" : "Show redacted value"}
        onClick={() => setVisible((current) => !current)}
      />
    </div>
  );
}
