"use client";

import { ArrowRight, Check, Copy, ExternalLink, Search } from "lucide-react";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import type {
  PromptCategory,
  PromptLibraryExample,
  PromptLibraryPageCopy,
  PromptPageType,
} from "@/lib/prompt-library-public";
import {
  filterPromptLibraryExamples,
  getPromptLibraryFilterOptions,
} from "@/lib/prompt-library-public";
import { cn } from "@/lib/utils";

type Props = {
  copy: PromptLibraryPageCopy;
  examples: readonly PromptLibraryExample[];
};

type CategoryFilter = PromptCategory | "all";
type PageTypeFilter = PromptPageType | "all";
type ModelFilter = string | "all";

export function PromptLibraryBrowser({ copy, examples }: Props) {
  const [activeCategory, setActiveCategory] = useState<CategoryFilter>("all");
  const [activePageType, setActivePageType] = useState<PageTypeFilter>("all");
  const [activeModel, setActiveModel] = useState<ModelFilter>("all");
  const [copiedSlug, setCopiedSlug] = useState<string | null>(null);
  const [query, setQuery] = useState("");

  const categories = useMemo(
    () => Array.from(new Set(examples.map((item) => item.category))),
    [examples],
  );
  const filterOptions = useMemo(
    () => getPromptLibraryFilterOptions(examples),
    [examples],
  );
  const filteredExamples = useMemo(() => {
    return filterPromptLibraryExamples(examples, {
      category: activeCategory,
      model: activeModel,
      pageType: activePageType,
      query,
    });
  }, [activeCategory, activeModel, activePageType, examples, query]);

  useEffect(() => {
    if (!copiedSlug) return;
    const timer = window.setTimeout(() => setCopiedSlug(null), 1400);
    return () => window.clearTimeout(timer);
  }, [copiedSlug]);

  const handleCopy = async (item: PromptLibraryExample) => {
    try {
      await navigator.clipboard.writeText(item.prompt);
      setCopiedSlug(item.slug);
    } catch {
      setCopiedSlug(null);
    }
  };

  return (
    <section className="relative z-10 px-6 py-12 lg:py-16">
      <div className="mx-auto max-w-6xl">
        <div className="rounded-2xl border border-violet-500/16 bg-white/72 p-5 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm lg:p-6 dark:bg-white/[0.04]">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div className="min-w-0 max-w-3xl">
              <p className="inline-flex items-center gap-1.5 text-[11px] font-medium tracking-widest text-violet-700 uppercase dark:text-violet-300">
                <span className="relative flex size-1.5">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-violet-400 opacity-75" />
                  <span className="relative inline-flex size-1.5 rounded-full bg-violet-500" />
                </span>
                {copy.filterLabel}
              </p>
              <h2 className="mt-3 text-2xl leading-tight font-bold tracking-tight md:text-3xl">
                {copy.featuredLabel}
              </h2>
              <p className="text-muted-foreground/80 mt-3 max-w-2xl text-sm leading-relaxed sm:text-base">
                {copy.sourceNote}
              </p>
            </div>
            <label className="relative block w-full max-w-xl">
              <span className="sr-only">{copy.searchPlaceholder}</span>
              <Search
                className="pointer-events-none absolute top-1/2 left-4 size-4 -translate-y-1/2 text-violet-600/65"
                aria-hidden="true"
              />
              <input
                className="h-12 w-full rounded-lg border border-violet-500/16 bg-white/78 pr-4 pl-11 text-sm font-semibold outline-none transition placeholder:text-slate-400 focus:border-violet-500/45 focus:shadow-[0_0_0_3px_rgba(124,58,237,.12)] dark:bg-white/[0.05]"
                placeholder={copy.searchPlaceholder}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
            </label>
          </div>

          <div className="mt-6 grid gap-4">
            <FilterGroup label={copy.pageTypeFilterLabel}>
              <FilterChip
                active={activePageType === "all"}
                label={copy.allPageTypes}
                onClick={() => setActivePageType("all")}
              />
              {filterOptions.pageTypes.map((pageType) => (
                <FilterChip
                  active={activePageType === pageType}
                  key={pageType}
                  label={copy.pageTypes[pageType]}
                  onClick={() => setActivePageType(pageType)}
                />
              ))}
            </FilterGroup>

            <FilterGroup label={copy.modelFilterLabel}>
              <FilterChip
                active={activeModel === "all"}
                label={copy.allModels}
                onClick={() => setActiveModel("all")}
              />
              {filterOptions.models.map((model) => (
                <FilterChip
                  active={activeModel === model}
                  key={model}
                  label={model}
                  onClick={() => setActiveModel(model)}
                />
              ))}
            </FilterGroup>

            <FilterGroup label={copy.filterLabel}>
              <FilterChip
                active={activeCategory === "all"}
                label={copy.allCategories}
                onClick={() => setActiveCategory("all")}
              />
              {categories.map((category) => (
                <FilterChip
                  active={activeCategory === category}
                  key={category}
                  label={copy.categories[category]}
                  onClick={() => setActiveCategory(category)}
                />
              ))}
            </FilterGroup>
          </div>
        </div>

        <div className="mt-6">
          {filteredExamples.length === 0 ? (
            <div className="rounded-2xl border border-violet-500/16 bg-white/62 p-8 text-muted-foreground shadow-[0_18px_48px_-36px_rgba(124,58,237,.55)] dark:bg-white/[0.04]">
              <h3 className="text-xl font-semibold text-foreground">
                {copy.emptyTitle}
              </h3>
              <p className="mt-2 max-w-2xl text-sm leading-6">
                {copy.emptyBody}
              </p>
            </div>
          ) : (
            <div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
              {filteredExamples.map((item) => (
                <PromptExampleCard
                  copied={copiedSlug === item.slug}
                  copy={copy}
                  item={item}
                  key={item.slug}
                  onCopy={() => void handleCopy(item)}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

function FilterGroup(props: {
  children: ReactNode;
  label: string;
}) {
  return (
    <div className="grid gap-2 sm:grid-cols-[8.5rem_minmax(0,1fr)] sm:items-start">
      <div className="pt-2 text-[11px] font-medium tracking-widest text-muted-foreground uppercase">
        {props.label}
      </div>
      <div className="flex flex-wrap gap-2">{props.children}</div>
    </div>
  );
}

function FilterChip(props: {
  active: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={cn(
        "inline-flex h-10 items-center rounded-lg border px-4 text-sm font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 focus-visible:ring-offset-2",
        props.active
          ? "flatkey-hero-cta border-violet-500 text-white shadow-[0_16px_34px_-18px_rgba(124,58,237,0.85)]"
          : "border-violet-500/18 bg-white/68 text-[#3f3f46] hover:-translate-y-0.5 hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-white/78",
      )}
      onClick={props.onClick}
      aria-pressed={props.active}
    >
      {props.label}
    </button>
  );
}

function PromptExampleCard(props: {
  copied: boolean;
  copy: PromptLibraryPageCopy;
  item: PromptLibraryExample;
  onCopy: () => void;
}) {
  return (
    <article className="group overflow-hidden rounded-2xl border border-violet-500/16 bg-white/72 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm transition duration-200 hover:-translate-y-1 hover:border-violet-500/28 hover:bg-white/82 dark:bg-white/[0.04] dark:hover:bg-white/[0.06]">
      <a
        className="block"
        href={props.item.source.url}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={`${props.copy.viewSource}: ${props.item.title}`}
      >
        <div
          className="relative aspect-[16/10] overflow-hidden bg-[#f4f0ff]"
          role="img"
          aria-label={props.item.previewImage.alt}
        >
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_18%_22%,rgba(124,58,237,.16),transparent_28%),radial-gradient(circle_at_78%_72%,rgba(217,70,239,.14),transparent_36%),linear-gradient(135deg,#fbfaff_0%,#f4f0ff_52%,#ffffff_100%)]" />
          <div className="absolute top-8 left-6 h-2 w-24 rounded-full bg-violet-500/25" />
          <div className="absolute top-14 left-6 h-2 w-36 rounded-full bg-violet-500/12" />
          <div className="absolute right-6 bottom-7 h-14 w-20 rounded-lg border border-violet-500/15 bg-white/35" />
          <div
            className="absolute inset-0 bg-cover bg-center opacity-90 transition duration-300 group-hover:scale-[1.03]"
            style={{ backgroundImage: `url("${props.item.previewImage.src}")` }}
          />
          <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0)_52%,rgba(244,240,255,.45)_100%)] ring-1 ring-inset ring-violet-500/14" />
          <div className="absolute top-3 left-3 rounded-full border border-violet-500/16 bg-white/75 px-2.5 py-1 text-[10px] font-bold tracking-widest text-violet-700 uppercase shadow-sm backdrop-blur-sm">
            {props.copy.pageTypes[props.item.pageType]}
          </div>
        </div>
      </a>
      <div className="border-t border-violet-500/10 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="text-[11px] font-medium tracking-widest text-violet-700 uppercase dark:text-violet-300">
              {props.copy.categories[props.item.category]}
            </p>
            <h3 className="mt-1 line-clamp-2 min-h-12 text-xl leading-6 font-semibold tracking-tight text-[#0B0B0F] dark:text-white">
              {props.item.title}
            </h3>
          </div>
          <span className="shrink-0 rounded-lg border border-violet-500/16 bg-white/65 px-2 py-1 text-[11px] font-bold text-muted-foreground dark:bg-white/[0.05]">
            {props.item.ratio}
          </span>
        </div>

        <div className="mt-3 flex flex-wrap gap-2 text-[11px] font-semibold text-muted-foreground">
          <span>
            {props.copy.pageTypeLabel}: {props.copy.pageTypes[props.item.pageType]}
          </span>
          <span aria-hidden="true">/</span>
          <span>
            {props.copy.modelLabel}: {props.item.model}
          </span>
          <span aria-hidden="true">/</span>
          <span>
            {props.copy.sourceLabel}: {props.item.source.label}
          </span>
        </div>

        <div className="mt-4 rounded-xl border border-violet-500/12 bg-white/60 dark:bg-white/[0.035]">
          <div className="flex items-center justify-between border-b border-violet-500/10 px-3 py-2">
            <span className="text-[11px] font-medium tracking-widest text-violet-700 uppercase dark:text-violet-300">
              {props.copy.promptLabel}
            </span>
            <span className="text-[10px] font-semibold tracking-widest text-muted-foreground uppercase">
              {props.item.updatedAt}
            </span>
          </div>
          <pre className="max-h-44 overflow-auto p-3 text-[12px] leading-5 whitespace-pre-wrap break-words text-[#34323a] dark:text-white/78">
            <code
              className="block"
              style={{ overflowWrap: "anywhere", wordBreak: "break-word" }}
            >
              {props.item.prompt}
            </code>
          </pre>
        </div>

        <div className="mt-4 flex flex-wrap items-center gap-2">
          <button
            type="button"
            className="flatkey-hero-cta inline-flex h-10 items-center gap-2 rounded-lg px-3 text-sm font-semibold shadow-[0_16px_34px_-18px_rgba(124,58,237,0.85)] transition hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 focus-visible:ring-offset-2"
            onClick={props.onCopy}
          >
            {props.copied ? <Check className="size-4" /> : <Copy className="size-4" />}
            {props.copied ? props.copy.copiedPrompt : props.copy.copyPrompt}
          </button>
          <a
            className="inline-flex h-10 items-center gap-2 rounded-lg border border-violet-500/18 bg-white/65 px-3 text-sm font-semibold text-[#3f3f46] transition hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-white/78"
            href={props.item.source.url}
            target="_blank"
            rel="noopener noreferrer"
          >
            <ExternalLink className="size-4" />
            {props.copy.viewSource}
          </a>
          <a
            className="ml-auto inline-flex h-10 items-center gap-1.5 rounded-lg px-2 text-xs font-semibold text-violet-700 transition hover:text-violet-900 dark:text-violet-300 dark:hover:text-violet-100"
            href={props.item.source.repositoryUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            {props.copy.repositoryLabel}
            <ArrowRight className="size-3.5" />
          </a>
        </div>
      </div>
    </article>
  );
}
