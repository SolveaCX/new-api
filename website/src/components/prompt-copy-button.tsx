"use client";

import { Check, Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { cn } from "@/lib/utils";

type Props = {
  className?: string;
  copiedLabel: string;
  prompt: string;
  promptLabel: string;
};

export function PromptCopyButton(props: Props) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const handleClick = async () => {
    try {
      await navigator.clipboard.writeText(props.prompt);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  return (
    <button
      type="button"
      className={cn(
        "flatkey-hero-cta inline-flex h-10 items-center gap-2 rounded-lg px-3 text-sm font-semibold shadow-[0_16px_34px_-18px_rgba(124,58,237,0.85)] transition hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 focus-visible:ring-offset-2",
        props.className,
      )}
      onClick={handleClick}
    >
      {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
      {copied ? props.copiedLabel : props.promptLabel}
    </button>
  );
}
