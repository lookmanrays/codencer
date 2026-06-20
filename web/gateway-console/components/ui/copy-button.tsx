"use client";

import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { IconButton } from "@/components/ui/icon-button";

type CopyButtonProps = {
  value: string;
  label?: string;
};

export function CopyButton({ label = "Copy", value }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  }

  return <IconButton icon={copied ? Check : Copy} label={copied ? "Copied" : label} onClick={copy} />;
}
