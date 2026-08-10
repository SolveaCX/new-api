"use client";

import { Check, Copy, Terminal } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  CLAUDE_CODE_INSTALL_COMMANDS,
  detectClaudeCodeInstallTab,
  type ClaudeCodeInstallTab,
} from "@/lib/claude-code-use-case";
import { type Locale, withIdFallback } from "@/lib/locales";
import { cn } from "@/lib/utils";

const tabs: Array<{ id: ClaudeCodeInstallTab; label: string; hint: string }> = [
  { id: "macos", label: "macOS", hint: "Terminal" },
  { id: "linux", label: "Linux", hint: "Shell" },
  { id: "windows", label: "Windows", hint: "PowerShell" },
];

const tabCopy: Record<Locale, { aria: string; oneLiner: string; copy: string; copied: string; srTitle: string }> =withIdFallback({
  en: { aria: "Install command by operating system", oneLiner: "one-liner", copy: "Copy", copied: "Copied", srTitle: "Flatkey one-line install commands" },
  zh: { aria: "按操作系统选择安装命令", oneLiner: "一行命令", copy: "复制", copied: "已复制", srTitle: "Flatkey 一行安装命令" },
  es: { aria: "Comando de instalación por sistema operativo", oneLiner: "one-liner", copy: "Copiar", copied: "Copiado", srTitle: "Comandos de instalación Flatkey" },
  fr: { aria: "Commande d'installation par système", oneLiner: "one-liner", copy: "Copier", copied: "Copié", srTitle: "Commandes d'installation Flatkey" },
  pt: { aria: "Comando de instalação por sistema operacional", oneLiner: "one-liner", copy: "Copiar", copied: "Copiado", srTitle: "Comandos de instalação Flatkey" },
  ru: { aria: "Команда установки по ОС", oneLiner: "one-liner", copy: "Копировать", copied: "Скопировано", srTitle: "Команды установки Flatkey" },
  ja: { aria: "OS 別インストールコマンド", oneLiner: "one-liner", copy: "コピー", copied: "コピー済み", srTitle: "Flatkey インストールコマンド" },
  vi: { aria: "Lệnh cài đặt theo hệ điều hành", oneLiner: "one-liner", copy: "Sao chép", copied: "Đã sao chép", srTitle: "Lệnh cài đặt Flatkey" },
  de: { aria: "Installationsbefehl nach Betriebssystem", oneLiner: "one-liner", copy: "Kopieren", copied: "Kopiert", srTitle: "Flatkey Installationsbefehle" },
});

type Props = {
  locale: Locale;
};

export function ClaudeCodeInstallTabs({ locale }: Props) {
  const [active, setActive] = useState<ClaudeCodeInstallTab>("macos");
  const [copied, setCopied] = useState(false);
  const copy = tabCopy[locale] ?? tabCopy.en;
  const activeCommand = CLAUDE_CODE_INSTALL_COMMANDS[active];

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setActive(detectClaudeCodeInstallTab(navigator.platform || navigator.userAgent));
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const activeLabel = useMemo(() => tabs.find((tab) => tab.id === active)?.label ?? "macOS", [active]);

  const copyCommand = async () => {
    await navigator.clipboard.writeText(activeCommand);
    setCopied(true);
  };

  return (
    <div className="min-w-0 max-w-full rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 p-3 shadow-[5px_5px_0_#101014] sm:p-4 dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]">
      <div className="mb-4 grid min-w-0 grid-cols-2 gap-2 sm:flex sm:flex-wrap" role="tablist" aria-label={copy.aria}>
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={active === tab.id}
            aria-controls={`claude-code-${tab.id}-command`}
            className={cn(
              "inline-flex h-10 min-w-0 items-center justify-center gap-2 rounded-full border-2 px-3 text-sm font-black transition-colors sm:px-4",
              active === tab.id
                ? "border-[#101014] bg-[#5852FF] text-white shadow-[2px_2px_0_#101014] dark:border-white/24 dark:bg-white dark:text-[#101014] dark:shadow-[2px_2px_0_rgba(255,255,255,0.16)]"
                : "border-[#101014]/16 bg-white/68 text-[#5C5861] hover:border-[#101014] hover:bg-[#F9F871] hover:text-[#101014] dark:border-white/14 dark:bg-white/[0.06] dark:text-white/62 dark:hover:border-white/24 dark:hover:bg-white/12 dark:hover:text-white"
            )}
            onClick={() => setActive(tab.id)}
          >
            <Terminal className="size-4 shrink-0" />
            <span className="truncate">{tab.label}</span>
          </button>
        ))}
      </div>

      <div
        id={`claude-code-${active}-command`}
        role="tabpanel"
        aria-label={`${activeLabel} ${copy.oneLiner}`}
        className="min-w-0 overflow-hidden rounded-[1rem] border-2 border-[#101014] bg-zinc-950 dark:border-white/18"
      >
        <div className="flex min-w-0 items-center justify-between gap-2 border-b border-white/10 px-3 py-2 sm:gap-3 sm:px-4">
          <span className="min-w-0 truncate text-xs font-semibold text-zinc-400">{activeLabel} {copy.oneLiner}</span>
          <button
            type="button"
            onClick={copyCommand}
            className="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg bg-white/10 px-2.5 text-xs font-semibold text-white transition-colors hover:bg-white/16"
          >
            {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
            {copied ? copy.copied : copy.copy}
          </button>
        </div>
        <pre className="max-w-full overflow-x-auto p-3 font-mono text-[13px] leading-6 text-zinc-100 sm:p-4">{activeCommand}</pre>
      </div>

      <div className="sr-only">
        <h2>{copy.srTitle}</h2>
        {tabs.map((tab) => (
          <p key={tab.id}>
            {tab.label}: {CLAUDE_CODE_INSTALL_COMMANDS[tab.id]}
          </p>
        ))}
      </div>
    </div>
  );
}
