import { ArrowRight, ExternalLink, Sparkles } from "lucide-react";
import Link from "next/link";
import { PromptCopyButton } from "@/components/prompt-copy-button";
import { SiteShell } from "@/components/site-shell";
import type { Locale } from "@/lib/locales";
import { localizePath } from "@/lib/locales";
import { cn } from "@/lib/utils";
import {
  PROMPTS_PATH,
  getPromptLibraryExampleBySlug,
  getPromptLibraryExamples,
  getPromptLibraryExamplesByMediaType,
  getPromptLibraryExamplesByModelSlug,
  getPromptLibraryMediaSummaries,
  getPromptLibraryModelDisplayName,
  getPromptLibraryModelPath,
  getPromptLibraryModelSlug,
  getPromptLibraryModelSummaries,
  getPromptLibraryPageCopy,
  getPromptLibraryPromptPath,
  getPromptLibraryTypePath,
  type PromptLibraryExample,
  type PromptLibraryPageCopy,
  type PromptMediaType,
  type PromptPageType,
} from "@/lib/prompt-library-public";

type PromptCardProps = {
  copy: PromptLibraryPageCopy;
  href: string;
  item: PromptLibraryExample;
  large?: boolean;
  locale: Locale;
  title: string;
  body: string;
};

type SectionHeadingProps = {
  body?: string;
  eyebrow: string;
  title: string;
};

type HeroPreviewProps = {
  copy: PromptLibraryPageCopy;
  items: readonly PromptLibraryExample[];
  locale: Locale;
  paths: string[];
};

export function PromptLibraryPage({ locale }: { locale: Locale }) {
  const copy = getPromptLibraryPageCopy(locale);
  const examples = getPromptLibraryExamples();
  const mediaSummaries = getPromptLibraryMediaSummaries();
  const modelSummaries = getPromptLibraryModelSummaries();
  const hotItems = mediaSummaries
    .map((summary) => getPromptLibraryExamplesByMediaType(summary.type)[0])
    .filter((item): item is PromptLibraryExample => Boolean(item));
  const previewItems = hotItems.slice(0, 3);

  return (
    <SiteShell locale={locale} pathname={PROMPTS_PATH}>
      <main className="home-landing relative overflow-x-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_28%,#ffffff_58%,#f4f1ff_100%)] text-[#0B0B0F] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)] dark:text-white">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-0 bg-[linear-gradient(to_right,rgba(124,58,237,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.08)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />

        <section className="relative z-10 overflow-hidden px-6 pt-24 pb-14 md:pt-32 md:pb-20 lg:pt-36">
          <div
            aria-hidden
            className="home-hero-glow pointer-events-none absolute inset-0 -z-10 opacity-40 dark:opacity-55"
            style={{ background: "var(--home-hero-glow)" }}
          />
          <div
            aria-hidden
            className="absolute inset-0 -z-10 bg-[linear-gradient(to_right,rgba(124,58,237,0.16)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.14)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_64%_52%_at_50%_28%,black_20%,transparent_100%)] bg-[size:4rem_4rem] opacity-35 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.06)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.05)_1px,transparent_1px)] dark:opacity-40"
          />

          <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-12 lg:gap-8">
            <div className="flex flex-col items-start text-left lg:col-span-7">
              <div className="landing-animate-fade-up mb-5 inline-flex items-center gap-1.5 rounded-full border border-violet-500/25 bg-violet-500/10 px-3 py-1.5 text-[11px] font-medium text-violet-700 opacity-0 shadow-[0_12px_34px_-22px_rgba(124,58,237,0.75)] dark:text-violet-300">
                <span className="relative flex size-1.5">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-violet-400 opacity-75" />
                  <span className="relative inline-flex size-1.5 rounded-full bg-violet-500" />
                </span>
                <Sparkles className="size-3.5" aria-hidden="true" />
                {copy.heroBadge}
              </div>
              <h1
                className="landing-animate-fade-up text-[clamp(2.25rem,4.5vw,3.25rem)] leading-[1.15] font-bold tracking-tight opacity-0"
                style={{ animationDelay: "60ms", overflowWrap: "anywhere", textWrap: "balance" }}
              >
                {copy.heroTitle}
              </h1>
              <p
                className="text-muted-foreground/80 landing-animate-fade-up mt-5 max-w-xl text-base leading-relaxed opacity-0 md:text-[15px]"
                style={{ animationDelay: "120ms", overflowWrap: "anywhere" }}
              >
                {copy.heroBody}
              </p>
              <div
                className="landing-animate-fade-up mt-8 grid w-full max-w-2xl gap-3 opacity-0 sm:grid-cols-3"
                style={{ animationDelay: "180ms" }}
              >
                <HeroStat label={copy.exampleCountLabel} value={String(examples.length)} />
                <HeroStat label={copy.mediaTypeLabel} value={String(mediaSummaries.length)} />
                <HeroStat label={copy.modelLabel} value={String(modelSummaries.length)} />
              </div>
            </div>

            <div className="landing-animate-fade-up opacity-0 lg:col-span-5" style={{ animationDelay: "260ms" }}>
              {previewItems.length > 0 ? (
                <HeroPreviewStack
                  copy={copy}
                  items={previewItems}
                  locale={locale}
                  paths={previewItems.map((item) => getPromptLibraryPromptPath(item))}
                />
              ) : null}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading
              eyebrow={copy.weeklyHotTitle}
              title={copy.weeklyHotTitle}
              body={copy.weeklyHotBody}
            />
            <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
              {hotItems.map((item) => (
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryPromptPath(item)}
                  item={item}
                  locale={locale}
                  key={item.slug}
                  title={item.title}
                  body={item.prompt}
                />
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading
              eyebrow={copy.mediaBrowseTitle}
              title={copy.mediaBrowseTitle}
              body={copy.mediaBrowseBody}
            />
            <div className="mt-8 grid gap-5 md:grid-cols-3">
              {mediaSummaries.map((summary) => {
                const item = getPromptLibraryExamplesByMediaType(summary.type)[0] ?? examples[0];
                const mediaTitle = copy.mediaTypes[summary.type];
                const mediaBody = copy.mediaTypeDescriptions[summary.type];
                return (
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryTypePath(summary.type)}
                  item={item}
                  locale={locale}
                  key={summary.type}
                  title={mediaTitle}
                  body={mediaBody}
                  />
                );
              })}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading
              eyebrow={copy.modelBrowseTitle}
              title={copy.modelBrowseTitle}
              body={copy.modelBrowseBody}
            />
            <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
              {modelSummaries.map((summary) => {
                const item =
                  getPromptLibraryExamplesByModelSlug(summary.slug)[0] ??
                  getPromptLibraryExamples()[0];
                return (
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryModelPath(summary.slug)}
                  item={item}
                  locale={locale}
                  key={summary.slug}
                  title={summary.displayName}
                  body={`${copy.exampleCountLabel}: ${summary.count}`}
                  />
                );
              })}
            </div>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

export function PromptLibraryMediaPage({
  locale,
  mediaType,
}: {
  locale: Locale;
  mediaType: PromptMediaType;
}) {
  const copy = getPromptLibraryPageCopy(locale);
  const items = getPromptLibraryExamplesByMediaType(mediaType);
  const previewItems = items.slice(0, 3);
  const pageTypes = buildPageTypeGroups(items);
  const models = buildModelGroups(items);

  return (
    <SiteShell locale={locale} pathname={getPromptLibraryTypePath(mediaType)}>
      <main className="home-landing relative overflow-x-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_28%,#ffffff_58%,#f4f1ff_100%)] text-[#0B0B0F] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)] dark:text-white">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-0 bg-[linear-gradient(to_right,rgba(124,58,237,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.08)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />

        <section className="relative z-10 overflow-hidden px-6 pt-24 pb-14 md:pt-32 md:pb-20 lg:pt-36">
          <div
            aria-hidden
            className="home-hero-glow pointer-events-none absolute inset-0 -z-10 opacity-40 dark:opacity-55"
            style={{ background: "var(--home-hero-glow)" }}
          />
          <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-12 lg:gap-8">
            <div className="flex flex-col items-start text-left lg:col-span-7">
              <Link
                className="landing-animate-fade-up mb-5 inline-flex items-center gap-2 rounded-full border border-violet-500/20 bg-white/65 px-3 py-1.5 text-[11px] font-medium text-violet-700 opacity-0 transition hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-violet-300"
                href={localizePath(PROMPTS_PATH, locale)}
              >
                <ArrowRight className="size-3.5 rotate-180" aria-hidden="true" />
                {copy.allMediaTypes}
              </Link>
              <h1
                className="landing-animate-fade-up text-[clamp(2.25rem,4.5vw,3.25rem)] leading-[1.15] font-bold tracking-tight opacity-0"
                style={{ animationDelay: "60ms", overflowWrap: "anywhere", textWrap: "balance" }}
              >
                {copy.mediaTypes[mediaType]}
              </h1>
              <p
                className="text-muted-foreground/80 landing-animate-fade-up mt-5 max-w-xl text-base leading-relaxed opacity-0 md:text-[15px]"
                style={{ animationDelay: "120ms", overflowWrap: "anywhere" }}
              >
                {copy.mediaTypeDescriptions[mediaType]}
              </p>
              <div
                className="landing-animate-fade-up mt-8 grid w-full max-w-2xl gap-3 opacity-0 sm:grid-cols-3"
                style={{ animationDelay: "180ms" }}
              >
                <HeroStat label={copy.exampleCountLabel} value={String(items.length)} />
                <HeroStat label={copy.modelLabel} value={String(models.length)} />
                <HeroStat label={copy.pageTypeLabel} value={String(pageTypes.length)} />
              </div>
            </div>

            <div className="landing-animate-fade-up opacity-0 lg:col-span-5" style={{ animationDelay: "260ms" }}>
              {previewItems.length > 0 ? (
                <HeroPreviewStack
                  copy={copy}
                  items={previewItems}
                  locale={locale}
                  paths={previewItems.map((item) => getPromptLibraryPromptPath(item))}
                />
              ) : null}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading
              eyebrow={copy.weeklyHotTitle}
              title={copy.weeklyHotTitle}
              body={copy.weeklyHotBody}
            />
            <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
              {items.slice(0, 3).map((item) => (
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryPromptPath(item)}
                  item={item}
                  locale={locale}
                  key={item.slug}
                  title={item.title}
                  body={item.prompt}
                />
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading
              eyebrow={copy.pageTypeFilterLabel}
              title={copy.pageTypeFilterLabel}
            />
            <div className="mt-8 flex flex-wrap gap-3">
              {pageTypes.map((group) => (
                <FacetLink
                  href={group.href}
                  locale={locale}
                  key={group.pageType}
                  label={`${copy.pageTypes[group.pageType]} · ${group.count}`}
                />
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading
              eyebrow={copy.modelBrowseTitle}
              title={copy.modelBrowseTitle}
              body={copy.modelBrowseBody}
            />
            <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
              {models.map((group) => (
                <PromptCard
                  copy={copy}
                  href={group.href}
                  item={group.item}
                  locale={locale}
                  key={group.slug}
                  title={group.displayName}
                  body={`${copy.exampleCountLabel}: ${group.count}`}
                />
              ))}
            </div>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

export function PromptLibraryModelPage({
  locale,
  modelSlug,
}: {
  locale: Locale;
  modelSlug: string;
}) {
  const copy = getPromptLibraryPageCopy(locale);
  const modelSummary = getPromptLibraryModelSummaries().find((item) => item.slug === modelSlug);
  const items = getPromptLibraryExamplesByModelSlug(modelSlug);
  const previewItems = items.slice(0, 3);
  const pageTypes = buildPageTypeGroups(items);
  const mediaSummary = modelSummary ? getPromptLibraryMediaSummaries().find((item) => item.type === modelSummary.mediaType) : undefined;

  const displayName = modelSummary?.displayName ?? getPromptLibraryModelDisplayName(modelSlug);
  const mediaLabel = mediaSummary ? copy.mediaTypes[mediaSummary.type] : copy.mediaTypeLabel;

  return (
    <SiteShell locale={locale} pathname={getPromptLibraryModelPath(modelSlug)}>
      <main className="home-landing relative overflow-x-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_28%,#ffffff_58%,#f4f1ff_100%)] text-[#0B0B0F] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)] dark:text-white">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-0 bg-[linear-gradient(to_right,rgba(124,58,237,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.08)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />

        <section className="relative z-10 overflow-hidden px-6 pt-24 pb-14 md:pt-32 md:pb-20 lg:pt-36">
          <div
            aria-hidden
            className="home-hero-glow pointer-events-none absolute inset-0 -z-10 opacity-40 dark:opacity-55"
            style={{ background: "var(--home-hero-glow)" }}
          />
          <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-12 lg:gap-8">
            <div className="flex flex-col items-start text-left lg:col-span-7">
              <Link
                className="landing-animate-fade-up mb-5 inline-flex items-center gap-2 rounded-full border border-violet-500/20 bg-white/65 px-3 py-1.5 text-[11px] font-medium text-violet-700 opacity-0 transition hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-violet-300"
                href={localizePath(PROMPTS_PATH, locale)}
              >
                <ArrowRight className="size-3.5 rotate-180" aria-hidden="true" />
                {copy.allModels}
              </Link>
              <h1
                className="landing-animate-fade-up text-[clamp(2.25rem,4.5vw,3.25rem)] leading-[1.15] font-bold tracking-tight opacity-0"
                style={{ animationDelay: "60ms", overflowWrap: "anywhere", textWrap: "balance" }}
              >
                {displayName}
              </h1>
              <p
                className="text-muted-foreground/80 landing-animate-fade-up mt-5 max-w-xl text-base leading-relaxed opacity-0 md:text-[15px]"
                style={{ animationDelay: "120ms", overflowWrap: "anywhere" }}
              >
                {copy.modelBrowseBody}
              </p>
              <div
                className="landing-animate-fade-up mt-8 grid w-full max-w-2xl gap-3 opacity-0 sm:grid-cols-3"
                style={{ animationDelay: "180ms" }}
              >
                <HeroStat label={copy.exampleCountLabel} value={String(items.length)} />
                <HeroStat label={copy.mediaTypeLabel} value={mediaLabel} />
                <HeroStat label={copy.pageTypeLabel} value={String(pageTypes.length)} />
              </div>
            </div>

            <div className="landing-animate-fade-up opacity-0 lg:col-span-5" style={{ animationDelay: "260ms" }}>
              {previewItems.length > 0 ? (
                <HeroPreviewStack
                  copy={copy}
                  items={previewItems}
                  locale={locale}
                  paths={previewItems.map((item) => getPromptLibraryPromptPath(item))}
                />
              ) : null}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading
              eyebrow={copy.weeklyHotTitle}
              title={copy.weeklyHotTitle}
              body={copy.weeklyHotBody}
            />
            <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
              {items.slice(0, 3).map((item) => (
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryPromptPath(item)}
                  item={item}
                  locale={locale}
                  key={item.slug}
                  title={item.title}
                  body={item.prompt}
                />
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading eyebrow={copy.pageTypeFilterLabel} title={copy.pageTypeFilterLabel} />
            <div className="mt-8 flex flex-wrap gap-3">
              {pageTypes.map((group) => (
                <FacetLink
                  href={group.href}
                  locale={locale}
                  key={group.pageType}
                  label={`${copy.pageTypes[group.pageType]} · ${group.count}`}
                />
              ))}
            </div>
          </div>
        </section>

        {mediaSummary ? (
          <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
            <div className="mx-auto max-w-6xl">
              <SectionHeading eyebrow={copy.mediaBrowseTitle} title={copy.mediaBrowseTitle} />
              <div className="mt-8 max-w-xl">
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryTypePath(mediaSummary.type)}
                  item={items[0] ?? getPromptLibraryExamples()[0]}
                  locale={locale}
                  title={copy.mediaTypes[mediaSummary.type]}
                  body={copy.mediaTypeDescriptions[mediaSummary.type]}
                />
              </div>
            </div>
          </section>
        ) : null}
      </main>
    </SiteShell>
  );
}

export function PromptLibraryPromptPage({
  locale,
  slug,
}: {
  locale: Locale;
  slug: string;
}) {
  const copy = getPromptLibraryPageCopy(locale);
  const item = getPromptLibraryExampleBySlug(slug);
  if (!item) return null;

  const mediaItems = getPromptLibraryExamplesByMediaType(item.mediaType).filter((candidate) => candidate.slug !== item.slug);
  const modelItems = getPromptLibraryExamplesByModelSlug(getPromptLibraryModelSlug(item.model)).filter(
    (candidate) => candidate.slug !== item.slug,
  );
  const relatedMedia = mediaItems.slice(0, 3);
  const relatedModel = modelItems.slice(0, 3);

  return (
    <SiteShell locale={locale} pathname={getPromptLibraryPromptPath(item)}>
      <main className="home-landing relative overflow-x-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_28%,#ffffff_58%,#f4f1ff_100%)] text-[#0B0B0F] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)] dark:text-white">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-0 bg-[linear-gradient(to_right,rgba(124,58,237,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.08)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />

        <section className="relative z-10 overflow-hidden px-6 pt-24 pb-14 md:pt-32 md:pb-20 lg:pt-36">
          <div
            aria-hidden
            className="home-hero-glow pointer-events-none absolute inset-0 -z-10 opacity-40 dark:opacity-55"
            style={{ background: "var(--home-hero-glow)" }}
          />
          <div className="mx-auto grid max-w-6xl grid-cols-1 items-start gap-12 lg:grid-cols-12 lg:gap-8">
            <div className="flex flex-col items-start text-left lg:col-span-7">
              <Link
                className="landing-animate-fade-up mb-5 inline-flex items-center gap-2 rounded-full border border-violet-500/20 bg-white/65 px-3 py-1.5 text-[11px] font-medium text-violet-700 opacity-0 transition hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-violet-300"
                href={localizePath(getPromptLibraryTypePath(item.mediaType), locale)}
              >
                <ArrowRight className="size-3.5 rotate-180" aria-hidden="true" />
                {copy.mediaTypes[item.mediaType]}
              </Link>
              <h1
                className="landing-animate-fade-up text-[clamp(2.25rem,4.5vw,3.25rem)] leading-[1.15] font-bold tracking-tight opacity-0"
                style={{ animationDelay: "60ms", overflowWrap: "anywhere", textWrap: "balance" }}
              >
                {item.title}
              </h1>
              <p
                className="text-muted-foreground/80 landing-animate-fade-up mt-5 max-w-2xl text-base leading-relaxed opacity-0 md:text-[15px]"
                style={{ animationDelay: "120ms", overflowWrap: "anywhere" }}
              >
                {item.source.label} · {item.source.section}
              </p>
              <div
                className="landing-animate-fade-up mt-8 flex flex-wrap items-center gap-3 opacity-0"
                style={{ animationDelay: "180ms" }}
              >
                <PromptCopyButton
                  copiedLabel={copy.copiedPrompt}
                  prompt={item.prompt}
                  promptLabel={copy.copyPrompt}
                />
                <Link
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-violet-500/20 bg-white/65 px-3 text-sm font-semibold text-[#3f3f46] transition hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-white/78"
                  href={item.source.url}
                  rel="noopener noreferrer"
                  target="_blank"
                >
                  <ExternalLink className="size-4" />
                  {copy.viewSource}
                </Link>
                <Link
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-violet-500/20 bg-white/65 px-3 text-sm font-semibold text-[#3f3f46] transition hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-white/78"
                  href={localizePath(PROMPTS_PATH, locale)}
                >
                  {copy.allMediaTypes}
                </Link>
              </div>
              <div className="mt-8 grid w-full gap-3 sm:grid-cols-3">
                <HeroStat label={copy.mediaTypeLabel} value={copy.mediaTypes[item.mediaType]} />
                <HeroStat label={copy.modelLabel} value={getPromptLibraryModelDisplayName(item.model)} />
                <HeroStat label={copy.pageTypeLabel} value={copy.pageTypes[item.pageType]} />
              </div>
            </div>

            <div className="landing-animate-fade-up opacity-0 lg:col-span-5" style={{ animationDelay: "260ms" }}>
              <article className="overflow-hidden rounded-2xl border border-violet-500/16 bg-white/72 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm dark:bg-white/[0.04]">
                <PromptArtwork item={item} />
                <div className="border-t border-violet-500/10 p-4">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <HeroStat label={copy.mediaTypeLabel} value={copy.mediaTypes[item.mediaType]} />
                    <HeroStat label={copy.modelLabel} value={getPromptLibraryModelDisplayName(item.model)} />
                    <HeroStat label={copy.pageTypeLabel} value={copy.pageTypes[item.pageType]} />
                    <HeroStat label={copy.sourceLabel} value={item.source.label} />
                  </div>
                </div>
              </article>
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden px-6 py-16 md:py-20">
          <div className="mx-auto max-w-6xl">
            <SectionHeading eyebrow={copy.promptLabel} title={copy.promptLabel} />
            <div className="mt-8 overflow-hidden rounded-2xl border border-violet-500/16 bg-white/72 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm dark:bg-white/[0.04]">
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-violet-500/10 px-4 py-3">
                <div className="flex flex-wrap items-center gap-2 text-[11px] font-medium tracking-widest text-violet-700 uppercase dark:text-violet-300">
                  <span>{copy.mediaTypes[item.mediaType]}</span>
                  <span aria-hidden="true">/</span>
                  <span>{copy.pageTypes[item.pageType]}</span>
                  <span aria-hidden="true">/</span>
                  <span>{getPromptLibraryModelDisplayName(item.model)}</span>
                </div>
                <span className="text-[10px] font-semibold tracking-widest text-muted-foreground uppercase">
                  {item.updatedAt}
                </span>
              </div>
              <pre className="overflow-auto p-4 text-[13px] leading-6 whitespace-pre-wrap break-words text-[#34323a] dark:text-white/78">
                <code style={{ overflowWrap: "anywhere", wordBreak: "break-word" }}>{item.prompt}</code>
              </pre>
            </div>
          </div>
        </section>

        {relatedMedia.length > 0 ? (
          <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
            <div className="mx-auto max-w-6xl">
              <SectionHeading
                eyebrow={copy.mediaBrowseTitle}
                title={copy.mediaBrowseTitle}
                body={copy.mediaTypeDescriptions[item.mediaType]}
              />
              <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
                {relatedMedia.map((related) => (
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryPromptPath(related)}
                  item={related}
                  locale={locale}
                  key={related.slug}
                  title={related.title}
                  body={related.prompt}
                  />
                ))}
              </div>
            </div>
          </section>
        ) : null}

        {relatedModel.length > 0 ? (
          <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-16 md:py-20">
            <div className="mx-auto max-w-6xl">
              <SectionHeading
                eyebrow={copy.modelBrowseTitle}
                title={copy.modelBrowseTitle}
                body={copy.modelBrowseBody}
              />
              <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
                {relatedModel.map((related) => (
                <PromptCard
                  copy={copy}
                  href={getPromptLibraryPromptPath(related)}
                  item={related}
                  locale={locale}
                  key={related.slug}
                  title={related.title}
                  body={related.prompt}
                  />
                ))}
              </div>
            </div>
          </section>
        ) : null}
      </main>
    </SiteShell>
  );
}

function HeroPreviewStack(props: HeroPreviewProps) {
  const [featured, ...rest] = props.items;

  return (
    <div className="grid gap-4">
      {featured ? (
        <PromptCard
          copy={props.copy}
          href={props.paths[0] ?? getPromptLibraryPromptPath(featured)}
          item={featured}
          large
          locale={props.locale}
          title={featured.title}
          body={featured.prompt}
        />
      ) : null}
      {rest.length > 0 ? (
        <div className="grid gap-4 sm:grid-cols-2">
          {rest.map((item, index) => (
            <PromptCard
              copy={props.copy}
              href={props.paths[index + 1] ?? getPromptLibraryPromptPath(item)}
              item={item}
              locale={props.locale}
              key={item.slug}
              title={item.title}
              body={item.prompt}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function PromptCard(props: PromptCardProps) {
  return (
    <Link className="group block" href={localizePath(props.href, props.locale)}>
      <article className="overflow-hidden rounded-2xl border border-violet-500/16 bg-white/72 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] transition duration-200 hover:-translate-y-1 hover:border-violet-500/28 hover:bg-white/82 dark:bg-white/[0.04] dark:hover:bg-white/[0.06]">
        <PromptArtwork item={props.item} large={props.large} eyebrow={props.copy.mediaTypes[props.item.mediaType]} />
        <div className="p-4">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <h3 className="line-clamp-2 text-lg leading-6 font-semibold tracking-tight text-[#0B0B0F] dark:text-white">
                {props.title}
              </h3>
            </div>
            <span className="shrink-0 rounded-lg border border-violet-500/16 bg-white/65 px-2 py-1 text-[11px] font-bold text-muted-foreground dark:bg-white/[0.05]">
              {props.item.ratio}
            </span>
          </div>
          <p className="text-muted-foreground mt-2 line-clamp-3 text-sm leading-6">{props.body}</p>
          <div className="mt-3 flex flex-wrap gap-2 text-[11px] font-semibold text-muted-foreground">
            <span>
              {props.copy.modelLabel}: {getPromptLibraryModelDisplayName(props.item.model)}
            </span>
            <span aria-hidden="true">/</span>
            <span>
              {props.copy.pageTypeLabel}: {props.copy.pageTypes[props.item.pageType]}
            </span>
            <span aria-hidden="true">/</span>
            <span>
              {props.copy.sourceLabel}: {props.item.source.label}
            </span>
          </div>
        </div>
      </article>
    </Link>
  );
}

function PromptArtwork({
  item,
  large = false,
  eyebrow,
}: {
  eyebrow?: string;
  item: PromptLibraryExample;
  large?: boolean;
}) {
  return (
    <div
      className={cn(
        "relative overflow-hidden bg-[#f4f0ff]",
        large ? "aspect-[4/3]" : "aspect-[16/10]",
      )}
      role="img"
      aria-label={item.previewImage.alt}
    >
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_18%_22%,rgba(124,58,237,.16),transparent_28%),radial-gradient(circle_at_78%_72%,rgba(217,70,239,.14),transparent_36%),linear-gradient(135deg,#fbfaff_0%,#f4f0ff_52%,#ffffff_100%)]" />
      <div
        className="absolute inset-0 bg-cover bg-center opacity-90 transition duration-300 group-hover:scale-[1.03]"
        style={{ backgroundImage: `url("${item.previewImage.src}")` }}
      />
      <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(255,255,255,0)_52%,rgba(244,240,255,.45)_100%)] ring-1 ring-inset ring-violet-500/14" />
      {eyebrow ? (
        <div className="absolute top-3 left-3 rounded-full border border-violet-500/16 bg-white/78 px-2.5 py-1 text-[10px] font-bold tracking-widest text-violet-700 uppercase shadow-sm backdrop-blur-sm dark:bg-black/45 dark:text-violet-200">
          {eyebrow}
        </div>
      ) : null}
      <div className="absolute top-3 right-3 rounded-full border border-violet-500/16 bg-white/78 px-2.5 py-1 text-[10px] font-bold tracking-widest text-muted-foreground uppercase shadow-sm backdrop-blur-sm dark:bg-black/45 dark:text-white/78">
        {item.ratio}
      </div>
    </div>
  );
}

function SectionHeading(props: SectionHeadingProps) {
  return (
    <div className="max-w-2xl">
      <p className="text-muted-foreground mb-3 text-xs font-medium tracking-widest uppercase">
        {props.eyebrow}
      </p>
      <h2 className="text-2xl leading-tight font-bold tracking-tight md:text-3xl">
        {props.title}
      </h2>
      {props.body ? (
        <p className="text-muted-foreground/80 mt-3 text-sm leading-relaxed md:text-base">
          {props.body}
        </p>
      ) : null}
    </div>
  );
}

function HeroStat(props: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-violet-500/16 bg-white/62 p-4 shadow-[0_18px_48px_-36px_rgba(124,58,237,.55)] backdrop-blur-sm dark:bg-white/[0.04]">
      <p className="text-3xl font-bold">{props.value}</p>
      <p className="text-violet-700 dark:text-violet-300 mt-1 text-[10px] font-medium tracking-widest uppercase">
        {props.label}
      </p>
    </div>
  );
}

function FacetLink(props: { href: string; label: string; locale: Locale }) {
  return (
    <Link
      className="inline-flex h-10 items-center rounded-lg border border-violet-500/18 bg-white/68 px-4 text-sm font-semibold text-[#3f3f46] transition hover:-translate-y-0.5 hover:border-violet-500/35 hover:bg-violet-500/10 dark:bg-white/[0.04] dark:text-white/78"
      href={localizePath(props.href, props.locale)}
    >
      {props.label}
    </Link>
  );
}

function buildPageTypeGroups(items: readonly PromptLibraryExample[]) {
  const grouped = new Map<PromptPageType, PromptLibraryExample[]>();
  for (const item of items) {
    const current = grouped.get(item.pageType) ?? [];
    current.push(item);
    grouped.set(item.pageType, current);
  }

  return Array.from(grouped.entries()).map(([pageType, group]) => ({
    count: group.length,
    href: getPromptLibraryPromptPath(group[0]),
    pageType,
  }));
}

function buildModelGroups(items: readonly PromptLibraryExample[]) {
  const grouped = new Map<string, PromptLibraryExample[]>();
  for (const item of items) {
    const slug = getPromptLibraryModelSlug(item.model);
    const current = grouped.get(slug) ?? [];
    current.push(item);
    grouped.set(slug, current);
  }

  return Array.from(grouped.entries()).map(([slug, group]) => ({
    count: group.length,
    displayName: getPromptLibraryModelDisplayName(group[0].model),
    href: getPromptLibraryModelPath(slug),
    item: group[0],
    slug,
  }));
}
