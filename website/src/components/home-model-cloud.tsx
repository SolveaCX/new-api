import Link from "next/link";
import { ArrowRight, KeyRound } from "lucide-react";
import { ModelLogo } from "@/components/pricing-model-browser";
import type { HomeCopy } from "@/lib/home-copy";

type Props = {
  copy: HomeCopy["compare"];
  moreHref: string;
};

// Hero visual: a constellation of LLM vendor logos around a single "one key"
// core — conveys the breadth of 160+ models better than a plain price table.
// Icons resolve to lobehub static SVGs (graceful letter fallback on miss).
const LOGO_TILES: Array<{ iconKey: string; label: string }> = [
  { iconKey: "openai", label: "GPT" },
  { iconKey: "claude-color", label: "Claude" },
  { iconKey: "gemini-color", label: "Gemini" },
  { iconKey: "deepseek-color", label: "DeepSeek" },
  { iconKey: "qwen-color", label: "Qwen" },
  { iconKey: "grok", label: "Grok" },
  { iconKey: "mistral-color", label: "Mistral" },
  { iconKey: "meta-color", label: "Llama" },
  { iconKey: "kimi-color", label: "Kimi" },
  { iconKey: "zhipu-color", label: "GLM" },
  { iconKey: "minimax-color", label: "MiniMax" },
  { iconKey: "doubao-color", label: "Doubao" },
  { iconKey: "moonshot", label: "Moonshot" },
  { iconKey: "cohere-color", label: "Cohere" },
  { iconKey: "perplexity-color", label: "Perplexity" },
];

export function HomeModelCloud(props: Props) {
  return (
    <div className="relative w-full max-w-md">
      {/* soft glow behind the grid */}
      <div
        aria-hidden="true"
        className="absolute -inset-6 -z-10 bg-[radial-gradient(circle_at_50%_42%,rgba(124,58,237,0.22),transparent_62%)] blur-2xl"
      />
      <div className="rounded-2xl border border-violet-500/16 bg-white/78 p-6 shadow-[0_32px_90px_-52px_rgba(91,33,182,0.8)] backdrop-blur-sm dark:bg-white/[0.04]">
        <div className="flex items-center justify-between gap-3">
          <span className="inline-flex items-center gap-1.5 rounded-full border border-violet-500/20 bg-violet-500/8 px-2.5 py-1 text-[11px] font-bold tracking-wide text-violet-700 dark:text-violet-300">
            <KeyRound className="size-3.5" />
            160+
          </span>
          <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/25 bg-emerald-500/10 px-2.5 py-1 text-[11px] font-bold text-emerald-700 dark:text-emerald-300">
            {props.copy.badge}
          </span>
        </div>

        <div className="mt-5 grid grid-cols-5 gap-2.5">
          {LOGO_TILES.map((tile, i) => (
            <div
              key={tile.iconKey}
              title={tile.label}
              className="group flex aspect-square items-center justify-center rounded-xl border border-violet-500/12 bg-gradient-to-br from-white to-violet-500/[0.04] transition-transform duration-200 hover:-translate-y-0.5 hover:border-violet-500/30 dark:from-white/[0.06] dark:to-white/[0.02]"
              style={{ animationDelay: `${i * 40}ms` }}
            >
              <ModelLogo iconKey={tile.iconKey} fallback={tile.label.charAt(0)} size={26} />
            </div>
          ))}
          {/* the "one key" core tile — visually anchors the constellation */}
          <div className="flex aspect-square items-center justify-center rounded-xl border border-violet-500/30 bg-gradient-to-br from-violet-600 to-indigo-600 text-white shadow-[0_10px_30px_-12px_rgba(99,102,241,0.9)]">
            <KeyRound className="size-5" />
          </div>
        </div>

        <Link
          href={props.moreHref}
          className="mt-5 flex items-center justify-between border-t border-violet-500/10 pt-3 text-[12px] font-semibold text-violet-700 transition-colors hover:text-violet-500 dark:text-violet-300 dark:hover:text-violet-200"
        >
          <span>{props.copy.more}</span>
          <ArrowRight className="size-3.5" />
        </Link>
      </div>
    </div>
  );
}
