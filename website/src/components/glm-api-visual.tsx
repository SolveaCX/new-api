"use client";

import { useMemo, useState } from "react";
import { Code2 } from "lucide-react";
import type { GlmLandingPageCopy } from "@/lib/glm-landing";

type ApiVisualTab = "openai" | "claude" | "glm";

type Props = {
  copy: GlmLandingPageCopy;
};

const CODE_BLOCK_LINE_COUNT = 14;

function padCodeBlock(code: string): string {
  const lines = code.split("\n");
  if (lines.length >= CODE_BLOCK_LINE_COUNT) return code;
  return [...lines, ...Array(CODE_BLOCK_LINE_COUNT - lines.length).fill("")].join("\n");
}

export function GlmApiVisual({ copy }: Props) {
  const tabs = useMemo(
    () =>
      [
        {
          id: "claude" as const,
          label: copy.visual.tabs[1],
          endpoint: "https://router.flatkey.ai/v1/messages",
          status: copy.visual.status.claude,
          code: padCodeBlock(`# ~/.claude/settings.json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://router.flatkey.ai",
    "ANTHROPIC_AUTH_TOKEN": "YOUR_FLATKEY_KEY",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.2",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.2",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.2",
    "CLAUDE_CODE_AUTO_COMPACT_WINDOW": "1000000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "API_TIMEOUT_MS": "3000000"
  }
}`),
        },
        {
          id: "openai" as const,
          label: copy.visual.tabs[0],
          endpoint: "https://router.flatkey.ai/v1/chat/completions",
          status: copy.visual.status.openai,
          code: padCodeBlock(`from openai import OpenAI

client = OpenAI(
    base_url="https://router.flatkey.ai/v1",
    api_key="YOUR_FLATKEY_KEY",
)

client.chat.completions.create(
    model="${copy.code.model}",
    messages=[...]
)`),
        },
        {
          id: "glm" as const,
          label: copy.visual.tabs[2],
          endpoint: "https://router.flatkey.ai/v1/chat/completions",
          status: copy.visual.status.curl,
          code: padCodeBlock(`curl https://router.flatkey.ai/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_FLATKEY_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "glm-5.2",
    "messages": [{"role":"user","content":"..."}]
  }'`),
        },
      ],
    [copy]
  );
  const [activeTab, setActiveTab] = useState<ApiVisualTab>("claude");
  const active = tabs.find((tab) => tab.id === activeTab) ?? tabs[0];

  return (
    <div className="relative mx-auto w-full max-w-3xl">
      <div className="relative overflow-hidden rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]">
        <div className="flex items-center gap-1 border-b-2 border-[#101014] bg-white/72 px-3 dark:border-white/18 dark:bg-white/[0.06]">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={[
                "relative -mb-0.5 min-h-12 border-b-2 px-3 text-left text-xs font-black transition-colors",
                active.id === tab.id
                  ? "border-[#101014] text-[#101014] dark:border-white dark:text-white"
                  : "border-transparent text-[#5C5861] hover:text-[#101014] dark:text-white/52 dark:hover:text-white",
              ].join(" ")}
            >
              {tab.label}
            </button>
          ))}
          <div className="ml-auto flex items-center gap-2 pr-2">
            <span className="inline-block size-1.5 rounded-full bg-emerald-500 shadow-[0_0_10px_rgba(16,185,129,0.55)] dark:bg-emerald-300 dark:shadow-[0_0_10px_rgba(52,211,153,0.75)]" />
            <span className="font-mono text-[10px] font-black uppercase text-[#5C5861] dark:text-white/52">200 ok</span>
          </div>
        </div>

        <div className="flex min-w-0 items-center gap-2.5 border-b-2 border-[#101014] bg-[#EEE4FF]/75 px-5 py-3 dark:border-white/18 dark:bg-white/[0.06]">
          <span className="rounded-md border border-[#101014]/16 bg-[#F9F871] px-1.5 py-0.5 font-mono text-[10px] font-black text-[#101014] dark:border-white/16 dark:bg-white/10 dark:text-white">POST</span>
          <code className="truncate font-mono text-[12.5px] text-[#4D4D56] dark:text-white/70">{active.endpoint}</code>
        </div>

        <div className="grid min-h-[390px] grid-rows-[1fr_auto] font-mono text-[12.5px] leading-[1.65]">
          <div className="p-5">
            <div className="mb-4 flex items-center gap-2 text-[#5C5861] dark:text-white/52">
              <Code2 className="size-4 text-emerald-600 dark:text-emerald-300" />
              <span className="font-sans text-[10px] font-black uppercase">{active.status}</span>
            </div>

            <pre className="min-h-[20rem] overflow-x-auto rounded-[1rem] border-2 border-[#101014] bg-[#060912] p-4 text-slate-100 dark:border-white/10 dark:bg-[#060912]/80 dark:text-slate-300">
              <code>{active.code}</code>
            </pre>
          </div>

          <div className="border-t-2 border-[#101014] bg-[#FFFDF6] px-5 py-4 dark:border-white/18 dark:bg-white/[0.06]">
            <div className="flex flex-col gap-3 text-[#5C5861] sm:flex-row sm:items-center sm:justify-between dark:text-white/62">
              <span>{copy.visual.compatibility}</span>
              <span className="font-mono text-xs font-bold text-emerald-600 dark:text-emerald-300">{copy.visual.priceSignal}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
