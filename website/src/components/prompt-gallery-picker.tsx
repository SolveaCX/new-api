"use client";

import { Check, Clipboard, ExternalLink, Filter, ImageIcon, Search, Sparkles, X } from "lucide-react";
import Image from "next/image";
import { useEffect, useMemo, useState } from "react";
import { PromptCopyButton } from "@/components/prompt-copy-button";
import { SiteShell } from "@/components/site-shell";
import type { Locale } from "@/lib/locales";
import { getPromptGalleryPickerCopy } from "@/lib/prompt-gallery-picker-copy";
import type { PromptGalleryItem } from "@/lib/prompt-gallery-api";

type Props = {
  error?: string;
  items: PromptGalleryItem[];
  locale: Locale;
  total: number;
};

function uniqueInOrder(values: readonly string[]) {
  return Array.from(new Set(values.filter(Boolean)));
}

function formatCount(value: number) {
  return new Intl.NumberFormat().format(value);
}

export function PromptGalleryPicker({ error, items, locale, total }: Props) {
  const copy = getPromptGalleryPickerCopy(locale);
  const [query, setQuery] = useState("");
  const [model, setModel] = useState("all");
  const [tag, setTag] = useState("all");
  const [selectedSlugs, setSelectedSlugs] = useState<Set<string>>(() => new Set());
  const [selectionCopied, setSelectionCopied] = useState(false);

  const models = useMemo(() => uniqueInOrder(items.map((item) => item.model)), [items]);
  const tags = useMemo(
    () => uniqueInOrder(items.flatMap((item) => item.tags)).sort((left, right) => left.localeCompare(right)),
    [items],
  );
  const filteredItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return items.filter((item) => {
      if (model !== "all" && item.model !== model) return false;
      if (tag !== "all" && !item.tags.includes(tag)) return false;
      if (!needle) return true;
      return [item.title, item.prompt, item.model, item.slug, item.tags.join(" "), item.source?.label]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(needle);
    });
  }, [items, model, query, tag]);
  const selectedItems = useMemo(
    () => items.filter((item) => selectedSlugs.has(item.slug)),
    [items, selectedSlugs],
  );

  useEffect(() => {
    if (!selectionCopied) return;
    const timer = window.setTimeout(() => setSelectionCopied(false), 1600);
    return () => window.clearTimeout(timer);
  }, [selectionCopied]);

  const toggleSelection = (slug: string) => {
    setSelectedSlugs((current) => {
      const next = new Set(current);
      if (next.has(slug)) next.delete(slug);
      else next.add(slug);
      return next;
    });
  };

  const clearFilters = () => {
    setQuery("");
    setModel("all");
    setTag("all");
  };

  const copySelection = async () => {
    if (selectedItems.length === 0) return;
    try {
      await navigator.clipboard.writeText(selectedItems.map((item) => item.slug).join("\n"));
      setSelectionCopied(true);
    } catch {
      setSelectionCopied(false);
    }
  };

  return (
    <SiteShell locale={locale} pathname="/prompt-picker">
      <main
        className="relative min-h-screen overflow-hidden bg-[#070812] text-white"
        data-testid="prompt-gallery-picker"
      >
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_12%_8%,rgba(91,73,255,0.32),transparent_33%),radial-gradient(circle_at_88%_18%,rgba(0,213,255,0.18),transparent_28%),linear-gradient(180deg,#090a18_0%,#070812_45%,#05060c_100%)]"
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 opacity-20 [background-image:linear-gradient(rgba(255,255,255,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.08)_1px,transparent_1px)] [background-size:56px_56px] [mask-image:linear-gradient(to_bottom,black,transparent_76%)]"
        />

        <section className="relative mx-auto max-w-[1440px] px-5 pb-20 pt-20 sm:px-8 lg:px-12">
          <header className="mx-auto max-w-4xl text-center">
            <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1.5 text-[11px] font-semibold tracking-[0.16em] text-cyan-100 uppercase">
              <Sparkles className="size-3.5" aria-hidden="true" />
              {copy.eyebrow}
            </div>
            <h1 className="text-balance text-[clamp(2.25rem,5vw,4.8rem)] leading-[0.98] font-black tracking-[-0.06em] text-white">
              {copy.title}
            </h1>
            <p className="mx-auto mt-6 max-w-2xl text-sm leading-7 text-white/62 sm:text-base">
              {copy.description}
            </p>
          </header>

          <div className="mx-auto mt-10 max-w-7xl rounded-[28px] border border-white/12 bg-white/[0.055] p-3 shadow-[0_30px_100px_-46px_rgba(0,215,255,0.5)] backdrop-blur-xl sm:p-4">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
              <label className="relative min-w-0 flex-1">
                <span className="sr-only">{copy.searchPlaceholder}</span>
                <Search className="pointer-events-none absolute top-1/2 left-4 size-4 -translate-y-1/2 text-white/42" aria-hidden="true" />
                <input
                  type="search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={copy.searchPlaceholder}
                  className="h-12 w-full rounded-2xl border border-white/10 bg-black/20 pr-4 pl-11 text-sm text-white outline-none placeholder:text-white/35 focus:border-cyan-300/55 focus:ring-2 focus:ring-cyan-300/15"
                />
              </label>
              <div className="flex flex-wrap items-center gap-2">
                <div className="flex items-center gap-2 px-1 text-xs font-semibold text-white/48">
                  <Filter className="size-3.5" aria-hidden="true" />
                  {copy.modelLabel}
                </div>
                <select
                  value={model}
                  onChange={(event) => setModel(event.target.value)}
                  className="h-11 max-w-[220px] rounded-xl border border-white/10 bg-[#111326] px-3 text-sm text-white outline-none focus:border-cyan-300/55"
                  aria-label={copy.modelLabel}
                >
                  <option value="all">{copy.allModels}</option>
                  {models.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
                <div className="flex items-center gap-2 px-1 text-xs font-semibold text-white/48">
                  {copy.tagLabel}
                </div>
                <select
                  value={tag}
                  onChange={(event) => setTag(event.target.value)}
                  className="h-11 max-w-[220px] rounded-xl border border-white/10 bg-[#111326] px-3 text-sm text-white outline-none focus:border-cyan-300/55"
                  aria-label={copy.tagLabel}
                >
                  <option value="all">{copy.allTags}</option>
                  {tags.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
                {(query || model !== "all" || tag !== "all") ? (
                  <button
                    type="button"
                    onClick={clearFilters}
                    className="inline-flex h-11 items-center gap-1.5 rounded-xl border border-white/10 px-3 text-xs font-semibold text-white/65 transition hover:border-white/25 hover:bg-white/8 hover:text-white"
                  >
                    <X className="size-3.5" aria-hidden="true" />
                    {copy.clearFilters}
                  </button>
                ) : null}
              </div>
            </div>

            <div className="mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-white/8 px-1 pt-3 text-xs text-white/48">
              <div className="flex flex-wrap gap-x-4 gap-y-1">
                <span><strong className="text-white/90">{formatCount(total)}</strong> {copy.total}</span>
                <span><strong className="text-white/90">{formatCount(filteredItems.length)}</strong> {copy.visible}</span>
                <span><strong className="text-cyan-200">{formatCount(selectedItems.length)}</strong> {copy.selected}</span>
              </div>
              <button
                type="button"
                onClick={copySelection}
                disabled={selectedItems.length === 0}
                className="inline-flex h-9 items-center gap-2 rounded-lg border border-cyan-200/25 bg-cyan-200/10 px-3 font-semibold text-cyan-100 transition hover:border-cyan-200/45 hover:bg-cyan-200/16 disabled:cursor-not-allowed disabled:opacity-35"
              >
                {selectionCopied ? <Check className="size-3.5" /> : <Clipboard className="size-3.5" />}
                {selectionCopied ? copy.copiedSelection : copy.copySelection}
              </button>
            </div>
          </div>

          {error ? (
            <div role="alert" className="mx-auto mt-6 max-w-7xl rounded-2xl border border-amber-300/25 bg-amber-300/10 px-5 py-4 text-sm text-amber-100">
              <strong className="block text-base">{copy.errorTitle}</strong>
              <span className="mt-1 block text-amber-100/70">{copy.errorBody}</span>
            </div>
          ) : null}

          {selectedItems.length > 0 ? (
            <div className="mx-auto mt-6 max-w-7xl rounded-2xl border border-cyan-200/18 bg-cyan-200/[0.055] px-4 py-3 text-xs text-cyan-100/75">
              <span className="font-semibold text-cyan-100">{copy.selected}:</span>{" "}
              {selectedItems.map((item) => item.slug).join(" · ")}
            </div>
          ) : null}

          {filteredItems.length > 0 ? (
            <div className="mx-auto mt-8 grid max-w-7xl gap-5 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
              {filteredItems.map((item) => {
                const isSelected = selectedSlugs.has(item.slug);
                return (
                  <article
                    className={`group overflow-hidden rounded-[24px] border bg-white/[0.055] shadow-[0_22px_60px_-42px_rgba(0,0,0,0.9)] transition duration-300 hover:-translate-y-1 hover:bg-white/[0.09] ${isSelected ? "border-cyan-200/70 ring-2 ring-cyan-200/20" : "border-white/10"}`}
                    data-prompt-slug={item.slug}
                    data-selected={isSelected ? "true" : "false"}
                    key={item.slug}
                  >
                    <div className="relative aspect-[4/3] overflow-hidden bg-[#111326]">
                      {item.artifact?.url ? (
                        <Image
                          src={item.artifact.url}
                          alt={item.artifact.alt || item.title}
                          fill
                          unoptimized
                          sizes="(min-width: 1536px) 25vw, (min-width: 1280px) 33vw, (min-width: 640px) 50vw, 100vw"
                          className="object-cover transition duration-700 group-hover:scale-[1.04]"
                        />
                      ) : (
                        <div className="grid size-full place-items-center text-white/25">
                          <ImageIcon className="size-10" aria-hidden="true" />
                        </div>
                      )}
                      <div aria-hidden="true" className="pointer-events-none absolute inset-0 bg-gradient-to-t from-[#070812]/80 via-transparent to-[#070812]/12" />
                      <div className="absolute top-3 left-3 rounded-full border border-white/15 bg-black/35 px-2.5 py-1 text-[10px] font-bold tracking-[0.08em] text-white/80 uppercase backdrop-blur-md">
                        {item.model}
                      </div>
                    </div>
                    <div className="p-4">
                      <div className="flex items-start justify-between gap-3">
                        <h2 className="line-clamp-2 min-h-12 text-base leading-6 font-bold text-white">{item.title}</h2>
                        <span className="shrink-0 rounded-full border border-white/10 px-2 py-1 text-[10px] font-semibold text-white/42">{item.category}</span>
                      </div>
                      <p className="mt-3 line-clamp-4 text-sm leading-6 text-white/62">{item.prompt}</p>
                      {item.outputTranslation ? (
                        <p className="mt-3 rounded-xl border border-white/8 bg-black/15 p-3 text-xs leading-5 text-white/48">
                          <span className="mb-1 block font-semibold text-white/68">{copy.translationLabel}</span>
                          {item.outputTranslation}
                        </p>
                      ) : null}
                      {item.tags.length > 0 ? (
                        <div className="mt-3 flex flex-wrap gap-1.5">
                          {item.tags.slice(0, 5).map((itemTag) => (
                            <button
                              type="button"
                              className="rounded-full border border-violet-200/15 bg-violet-200/8 px-2 py-1 text-[10px] font-semibold text-violet-100/70 transition hover:border-violet-200/35 hover:bg-violet-200/15"
                              key={itemTag}
                              onClick={() => setTag(itemTag)}
                            >
                              #{itemTag}
                            </button>
                          ))}
                        </div>
                      ) : null}
                      <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-white/8 pt-3">
                        <div className="flex min-w-0 items-center gap-2">
                          {item.source?.url ? (
                            <a
                              href={item.source.url}
                              target="_blank"
                              rel="noreferrer"
                              className="inline-flex max-w-[150px] items-center gap-1 truncate text-xs font-semibold text-white/48 transition hover:text-cyan-100"
                            >
                              {copy.sourceLabel} {item.source.label}
                              <ExternalLink className="size-3 shrink-0" aria-hidden="true" />
                            </a>
                          ) : <span className="text-xs text-white/38">{item.source?.label ?? "—"}</span>}
                        </div>
                        <div className="flex items-center gap-2">
                          <PromptCopyButton
                            className="!h-8 !rounded-lg !border-white/15 !bg-white/8 !px-2.5 !text-xs !text-white/80 hover:!bg-white/14"
                            copiedLabel={copy.copiedPrompt}
                            prompt={item.prompt}
                            promptLabel={copy.copyPrompt}
                          />
                          <button
                            type="button"
                            aria-pressed={isSelected}
                            onClick={() => toggleSelection(item.slug)}
                            className={`inline-flex h-8 items-center gap-1.5 rounded-lg px-2.5 text-xs font-bold transition ${isSelected ? "bg-cyan-200 text-[#071018]" : "border border-cyan-200/30 bg-cyan-200/10 text-cyan-100 hover:bg-cyan-200/18"}`}
                          >
                            {isSelected ? <Check className="size-3.5" /> : null}
                            {isSelected ? copy.deselect : copy.select}
                          </button>
                        </div>
                      </div>
                    </div>
                  </article>
                );
              })}
            </div>
          ) : (
            <div className="mx-auto mt-12 max-w-xl rounded-3xl border border-white/10 bg-white/[0.055] px-6 py-14 text-center">
              <ImageIcon className="mx-auto size-10 text-white/30" aria-hidden="true" />
              <h2 className="mt-4 text-xl font-bold text-white">{copy.emptyTitle}</h2>
              <p className="mt-2 text-sm leading-6 text-white/52">{copy.emptyBody}</p>
            </div>
          )}
        </section>
      </main>
    </SiteShell>
  );
}
