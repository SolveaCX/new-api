import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, ArrowRight, BookOpen, CalendarDays, Search, X } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { formatBlogCopy, type BlogCopy } from "@/lib/blog-copy";
import {
  BLOG_PAGE_SIZE,
  formatBlogDate,
  getBlogCategories,
  getBlogPost,
  getBlogPosts,
  getBlogToc,
  sanitizeBlogHtml,
  type BlogPost,
} from "@/lib/blog";
import { getCopy } from "@/lib/copy";
import type { Locale } from "@/lib/locales";
import { localizePath } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";
import {
  buildBlogArticleSchema,
  buildBlogCategorySchema,
  buildBlogIndexSchema,
  stringifyJsonLd,
  type JsonLdGraph,
} from "@/lib/schema";
import { cn } from "@/lib/utils";

type BlogSearchState = {
  page?: number;
  q?: string;
};

type Props = {
  locale: Locale;
};

const blogGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(91,33,182,0.065)_1px,transparent_1px),linear-gradient(to_bottom,rgba(91,33,182,0.055)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-55 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-35";
const blogWashClass = "pointer-events-none absolute inset-x-0 top-0 h-[34rem] fk-hero-wash";
const blogCardClass =
  "rounded-[16px] border border-[#0B0B0F14] bg-white/94 shadow-[0_18px_48px_-30px_rgba(46,16,101,0.22)] backdrop-blur-sm dark:border-white/14 dark:bg-white/[0.06] dark:shadow-none";
const blogMutedClass = "text-[#43434C] dark:text-white/62";
const blogContainerClass = "mx-auto w-full max-w-[var(--fk-site-container)] px-[var(--fk-site-gutter)]";

function JsonLdScript(props: { data: JsonLdGraph }) {
  return <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: stringifyJsonLd(props.data) }} />;
}

function Badge(props: { children: React.ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border border-[#0B0B0F14] bg-[#F0EBFA] px-3 py-1.5 font-mono text-[11px] font-bold uppercase text-[#4C1D95] dark:border-white/14 dark:bg-white/10 dark:text-white",
        props.className
      )}
    >
      {props.children}
    </span>
  );
}

function buttonClass(variant: "primary" | "outline" | "ghost" = "primary") {
  if (variant === "outline") {
    return "flatkey-cta-secondary inline-flex h-10 items-center justify-center gap-2 px-4 text-sm";
  }
  if (variant === "ghost") {
    return "fk-button-motion inline-flex h-10 items-center justify-center gap-2 rounded-[10px] border border-[#0B0B0F14] bg-white px-4 text-sm font-semibold text-[#0B0B0F] shadow-[0_1px_2px_rgba(11,11,15,0.06)] hover:border-[#5B21B6]/35 hover:bg-[#F0EBFA] dark:border-white/14 dark:bg-white/8 dark:text-white";
  }
  return "flatkey-cta-primary inline-flex h-10 items-center justify-center gap-2 px-4 text-sm";
}

function buildQuery(search?: BlogSearchState) {
  const params = new URLSearchParams();
  if (search?.page && search.page > 1) params.set("page", String(search.page));
  if (search?.q) params.set("q", search.q);
  const query = params.toString();
  return query ? `?${query}` : "";
}

function parsePage(value: string | string[] | undefined): number {
  const raw = Array.isArray(value) ? value[0] : value;
  const page = Number(raw);
  return Number.isFinite(page) && page > 0 ? Math.floor(page) : 1;
}

function parseQuery(value: string | string[] | undefined): string | undefined {
  const raw = Array.isArray(value) ? value[0] : value;
  const query = raw?.trim();
  return query || undefined;
}

export function parseBlogSearch(searchParams?: Record<string, string | string[] | undefined>): BlogSearchState {
  return {
    page: parsePage(searchParams?.page),
    q: parseQuery(searchParams?.q),
  };
}

function BlogHero(props: {
  locale: Locale;
  title: string;
  description: string;
  copy: BlogCopy;
  query?: string;
  categorySlug?: string;
}) {
  const action = props.categorySlug ? `/blog/category/${props.categorySlug}` : "/blog";

  return (
    <section className="relative z-10 border-b border-[#0B0B0F14] pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-14 text-center dark:border-white/12">
      <div className={blogContainerClass}>
        <Badge className="mb-5">
          <BookOpen className="size-3.5" />
          flatkey.ai
        </Badge>
        <h1 className="text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.98] font-semibold tracking-normal text-balance text-[#0B0B0F] dark:text-white">
          {props.title}
        </h1>
        <p className={`mx-auto mt-6 max-w-2xl text-base leading-7 text-balance md:text-lg ${blogMutedClass}`}>
          {props.description}
        </p>
        <form className="mx-auto mt-8 flex max-w-2xl flex-col gap-3 sm:flex-row" action={localizePath(action, props.locale)}>
          <div className="relative flex-1">
            <Search className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-[#83838E] dark:text-white/62" />
            <input
              name="q"
              defaultValue={props.query ?? ""}
              placeholder={props.copy.searchPlaceholder}
              className="h-11 w-full rounded-[10px] border border-[#0B0B0F14] bg-white px-4 pl-10 text-sm font-medium text-[#0B0B0F] shadow-[0_1px_2px_rgba(11,11,15,0.06)] outline-none transition placeholder:text-[#83838E] focus:border-[#5B21B6]/35 focus:ring-3 focus:ring-[#5B21B6]/10 dark:border-white/14 dark:bg-white/8 dark:text-white dark:placeholder:text-white/46"
              type="search"
            />
          </div>
          <button className={cn(buttonClass(), "h-11 px-5")} type="submit">
            <Search className="size-4" />
            {props.copy.search}
          </button>
          {props.query ? (
            <Link className={cn(buttonClass("outline"), "h-11 px-5")} href={localizePath(action, props.locale)}>
              <X className="size-4" />
              {props.copy.clear}
            </Link>
          ) : null}
        </form>
      </div>
    </section>
  );
}

async function BlogCategories(props: { locale: Locale }) {
  const categories = await getBlogCategories();
  const copy = getCopy(props.locale).blog;

  if (categories.length === 0) return null;

  return (
    <div className="mt-10 grid gap-4 text-left sm:grid-cols-2 lg:grid-cols-4">
      {categories.map((category) => (
        <Link
          key={category.slug}
          href={localizePath(`/blog/category/${category.slug}`, props.locale)}
          className={`fk-card-motion block ${blogCardClass} p-5`}
        >
          <h2 className="font-semibold text-[#0B0B0F] dark:text-white">{category.name}</h2>
          <p className={`mt-2 line-clamp-3 text-sm leading-6 ${blogMutedClass}`}>
            {category.description || formatBlogCopy(copy.latestInCategory, { category: category.name })}
          </p>
          <span className="mt-4 inline-flex items-center gap-1 text-sm font-semibold text-[#5B21B6] dark:text-[#C8A8FF]">
            {copy.readMore}
            <ArrowRight className="size-3.5" />
          </span>
        </Link>
      ))}
    </div>
  );
}

function BlogCard(props: { post: BlogPost; locale: Locale; compact?: boolean }) {
  const date = formatBlogDate(props.post.date, "short", props.locale);

  return (
    <Link
      href={localizePath(`/blog/${props.post.slug}`, props.locale)}
      className={`fk-card-motion group flex min-h-full flex-col overflow-hidden ${blogCardClass}`}
    >
      {props.post.cover ? (
        <div className="aspect-[16/9] overflow-hidden border-b border-[#0B0B0F14] bg-[#F0EBFA] dark:border-white/12">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={props.post.cover}
            alt={props.post.title}
            loading="lazy"
            decoding="async"
            className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.03]"
          />
        </div>
      ) : (
        <div className="aspect-[16/9] border-b border-[#0B0B0F14] bg-[#F0EBFA] dark:border-white/12 dark:bg-white/8" />
      )}
      <div className={cn("flex flex-1 flex-col p-5", props.compact && "p-4")}>
        {props.post.categoryName ? <Badge className="mb-3 max-w-fit">{props.post.categoryName}</Badge> : null}
        <h2
          className={cn(
            "line-clamp-2 font-semibold text-[#0B0B0F] transition-colors group-hover:text-[#5B21B6] dark:text-white dark:group-hover:text-[#C8A8FF]",
            props.compact ? "text-sm leading-snug" : "text-base leading-snug"
          )}
        >
          {props.post.title}
        </h2>
        {props.post.summary && !props.compact ? (
          <p className={`mt-3 line-clamp-3 flex-1 text-sm leading-6 ${blogMutedClass}`}>{props.post.summary}</p>
        ) : null}
        <div className={`mt-5 flex flex-wrap items-center gap-2 font-mono text-xs font-bold ${blogMutedClass}`}>
          {date ? (
            <span className="inline-flex items-center gap-1.5">
              <CalendarDays className="size-3.5" />
              {date}
            </span>
          ) : null}
          {props.post.author ? <span>{props.post.author}</span> : null}
        </div>
      </div>
    </Link>
  );
}

function BlogPagination(props: { locale: Locale; pageNo: number; totalPages: number; query?: string; categorySlug?: string }) {
  if (props.totalPages <= 1) return null;
  const copy = getCopy(props.locale).blog;
  const basePath = props.categorySlug ? `/blog/category/${props.categorySlug}` : "/blog";
  const prevPage = props.pageNo - 1;
  const nextPage = props.pageNo + 1;

  return (
    <nav className="mt-14 flex flex-wrap items-center justify-center gap-3">
      {props.pageNo > 1 ? (
        <Link
          className={buttonClass("outline")}
          href={`${localizePath(basePath, props.locale)}${buildQuery({ page: prevPage, q: props.query })}`}
        >
          <ArrowLeft className="size-4" />
          {copy.previous}
        </Link>
      ) : null}
      <span className={`font-mono text-sm font-bold ${blogMutedClass}`}>
        {formatBlogCopy(copy.pageOf, { page: props.pageNo, total: props.totalPages })}
      </span>
      {props.pageNo < props.totalPages ? (
        <Link
          className={buttonClass("outline")}
          href={`${localizePath(basePath, props.locale)}${buildQuery({ page: nextPage, q: props.query })}`}
        >
          {copy.next}
          <ArrowRight className="size-4" />
        </Link>
      ) : null}
    </nav>
  );
}

function BlogCTA(props: { locale: Locale }) {
  const copy = getCopy(props.locale).blog;

  return (
    <section className="mt-20 rounded-[18px] bg-[radial-gradient(120%_160%_at_50%_-20%,#5B21B6_0%,#3B0FA0_45%,#2E1065_100%)] px-6 py-12 text-center text-white shadow-[0_24px_60px_-18px_rgba(46,16,101,0.18)] sm:px-10">
      <h2 className="text-2xl font-semibold">{copy.ctaTitle}</h2>
      <p className="mx-auto mt-3 max-w-2xl text-sm leading-6 text-white/72">
        {copy.ctaDescription}
      </p>
      <Link className={cn("flatkey-cta-inverse mt-7 inline-flex h-10 items-center justify-center gap-2 px-4 text-sm")} href={consoleUrl("/sign-up")}>
        {copy.ctaButton}
      </Link>
    </section>
  );
}

function EmptyBlogState(props: { locale: Locale }) {
  const copy = getCopy(props.locale).blog;

  return (
    <div className={`flex min-h-64 flex-col items-center justify-center px-6 py-14 text-center ${blogCardClass}`}>
      <BookOpen className="size-10 text-[#7C3AED] dark:text-[#C8A8FF]" />
      <h2 className="mt-4 text-lg font-semibold text-[#0B0B0F] dark:text-white">{copy.emptyTitle}</h2>
      <p className={`mt-2 max-w-md text-sm ${blogMutedClass}`}>{copy.emptyDescription}</p>
    </div>
  );
}

export async function BlogIndexPage(props: Props & { search?: BlogSearchState }) {
  const page = props.search?.page ?? 1;
  const query = props.search?.q;
  const posts = await getBlogPosts({ page, q: query }, props.locale);
  const totalPages = Math.ceil(posts.total / BLOG_PAGE_SIZE);
  const copy = getCopy(props.locale).blog;

  return (
    <SiteShell locale={props.locale} pathname="/blog">
      <JsonLdScript data={buildBlogIndexSchema({ locale: props.locale, title: copy.title, description: copy.description })} />
      <main className="fk-blog-page fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F6FB] text-[#0B0B0F] antialiased dark:bg-[#0A0A10] dark:text-[#F6F3EA]">
        <div aria-hidden className={blogGridClass} />
        <div aria-hidden className={blogWashClass} />
        <BlogHero
          locale={props.locale}
          title={copy.title}
          description={copy.description}
          copy={copy}
          query={query}
        />
        <section className={cn("relative z-10 py-14", blogContainerClass)}>
          <BlogCategories locale={props.locale} />
        </section>
        <section className={cn("relative z-10 pb-20", blogContainerClass)}>
          {posts.list.length === 0 ? (
            <EmptyBlogState locale={props.locale} />
          ) : (
            <>
              <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
                {posts.list.map((post) => (
                  <BlogCard key={post.id || post.slug} post={post} locale={props.locale} />
                ))}
              </div>
              <BlogPagination pageNo={page} totalPages={totalPages} query={query} locale={props.locale} />
              <BlogCTA locale={props.locale} />
            </>
          )}
        </section>
      </main>
    </SiteShell>
  );
}

export async function BlogArticlePage(props: Props & { slug: string }) {
  const post = await getBlogPost(props.slug, props.locale);
  if (!post) notFound();
  const currentPost = post;

  const relatedPosts = await getBlogPosts(
    { page: 1, categoryIds: currentPost.categoryId ? [currentPost.categoryId] : undefined },
    props.locale
  );
  const related = relatedPosts.list.filter((item) => item.slug !== props.slug).slice(0, 3);
  const html = sanitizeBlogHtml(currentPost.content ?? "", props.locale);
  const toc = getBlogToc(html);
  const copy = getCopy(props.locale).blog;

  return (
    <SiteShell locale={props.locale} pathname={`/blog/${props.slug}`}>
      <JsonLdScript data={buildBlogArticleSchema({ locale: props.locale, post })} />
      <main className="fk-blog-page fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F6FB] text-[#0B0B0F] antialiased dark:bg-[#0A0A10] dark:text-[#F6F3EA]">
        <div aria-hidden className={blogGridClass} />
        <div aria-hidden className={blogWashClass} />
        <section className="relative z-10 border-b border-[#0B0B0F14] pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-12 dark:border-white/12">
          <div className={blogContainerClass}>
            <div className="mx-auto max-w-4xl">
              <div className="mb-5 flex flex-wrap items-center gap-3">
                {currentPost.categoryName ? <Badge>{currentPost.categoryName}</Badge> : null}
                {currentPost.date ? <span className={`font-mono text-sm font-bold ${blogMutedClass}`}>{formatBlogDate(currentPost.date, "long", props.locale)}</span> : null}
                {currentPost.author ? <span className={`font-mono text-sm font-bold ${blogMutedClass}`}>{currentPost.author}</span> : null}
              </div>
              <h1 className="text-[clamp(2.4rem,6vw,5.4rem)] leading-[1] font-semibold tracking-normal text-balance text-[#0B0B0F] dark:text-white">
                {currentPost.title}
              </h1>
              {currentPost.summary ? (
                <p className={`mt-6 max-w-3xl text-base leading-7 text-balance md:text-lg ${blogMutedClass}`}>{currentPost.summary}</p>
              ) : null}
            </div>
          </div>
        </section>
        {currentPost.cover ? (
          <div className={cn("relative z-10 py-8", blogContainerClass)}>
            <div className="mx-auto max-w-4xl">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={currentPost.cover}
                alt={currentPost.title}
                className="aspect-[16/9] w-full rounded-[16px] border border-[#0B0B0F14] bg-[#F0EBFA] object-cover shadow-[0_18px_48px_-30px_rgba(46,16,101,0.22)] dark:border-white/14 dark:bg-white/8"
                loading="eager"
                decoding="async"
              />
            </div>
          </div>
        ) : null}
        <section className={cn("relative z-10 py-8", blogContainerClass)}>
          <div className="mx-auto grid max-w-5xl items-start gap-12 lg:grid-cols-[minmax(0,1fr)_240px]">
            <div className={`blog-content min-w-0 ${blogCardClass} p-6 md:p-9`} dangerouslySetInnerHTML={{ __html: html }} />
            {toc.length >= 2 ? (
              <aside className="hidden lg:block">
                <nav className="sticky top-24 text-sm">
                  <p className="mb-3 font-mono text-xs font-bold uppercase text-[#5B21B6] dark:text-[#C8A8FF]">{copy.onThisPage}</p>
                  <ul className="space-y-1.5">
                    {toc.map((item) => (
                      <li key={item.id}>
                        <a
                          href={`#${item.id}`}
                          className={cn(
                            `block rounded-[10px] px-3 py-2 leading-snug font-medium transition-colors hover:bg-[#F0EBFA] hover:text-[#5B21B6] dark:hover:bg-white/10 dark:hover:text-white ${blogMutedClass}`,
                            item.level === 3 && "pl-3"
                          )}
                        >
                          {item.text}
                        </a>
                      </li>
                    ))}
                  </ul>
                </nav>
              </aside>
            ) : null}
          </div>
        </section>
        {related.length > 0 ? (
          <section className="relative z-10 mt-10 border-t border-[#0B0B0F14] py-16 dark:border-white/12">
            <div className={blogContainerClass}>
              <div className="mx-auto max-w-5xl">
                <h2 className="text-xl font-semibold text-[#0B0B0F] dark:text-white">{copy.relatedArticles}</h2>
                <div className="mt-7 grid gap-5 sm:grid-cols-3">
                  {related.map((item) => (
                    <BlogCard key={item.id || item.slug} post={item} locale={props.locale} compact />
                  ))}
                </div>
              </div>
            </div>
          </section>
        ) : null}
        <div className={cn("relative z-10 pb-16", blogContainerClass)}>
          <div className="mx-auto max-w-5xl">
            <Link className={buttonClass("ghost")} href={localizePath("/blog", props.locale)}>
              <ArrowLeft className="size-4" />
              {copy.backToBlog}
            </Link>
          </div>
        </div>
      </main>
    </SiteShell>
  );
}

export async function BlogCategoryPage(props: Props & { slug: string; search?: BlogSearchState }) {
  const categories = await getBlogCategories();
  const category = categories.find((item) => item.slug === props.slug);
  if (!category) notFound();
  const currentCategory = category;

  const page = props.search?.page ?? 1;
  const query = props.search?.q;
  const posts = await getBlogPosts({ page, q: query, categoryIds: [currentCategory.id] }, props.locale);
  const totalPages = Math.ceil(posts.total / BLOG_PAGE_SIZE);
  const copy = getCopy(props.locale).blog;
  const description = currentCategory.description || formatBlogCopy(copy.latestInCategory, { category: currentCategory.name });

  return (
    <SiteShell locale={props.locale} pathname={`/blog/category/${props.slug}`}>
      <JsonLdScript data={buildBlogCategorySchema({ locale: props.locale, slug: props.slug, name: category.name, description })} />
      <main className="fk-blog-page fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F6FB] text-[#0B0B0F] antialiased dark:bg-[#0A0A10] dark:text-[#F6F3EA]">
        <div aria-hidden className={blogGridClass} />
        <div aria-hidden className={blogWashClass} />
        <BlogHero locale={props.locale} title={currentCategory.name} description={description} copy={copy} query={query} categorySlug={props.slug} />
        <section className={cn("relative z-10 py-12", blogContainerClass)}>
          <Link className={buttonClass("ghost")} href={localizePath("/blog", props.locale)}>
            <ArrowLeft className="size-4" />
            {copy.backToBlog}
          </Link>
        </section>
        <section className={cn("relative z-10 pb-20", blogContainerClass)}>
          {posts.list.length === 0 ? (
            <EmptyBlogState locale={props.locale} />
          ) : (
            <>
              <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
                {posts.list.map((post) => (
                  <BlogCard key={post.id || post.slug} post={post} locale={props.locale} />
                ))}
              </div>
              <BlogPagination
                pageNo={page}
                totalPages={totalPages}
                query={query}
                categorySlug={props.slug}
                locale={props.locale}
              />
              <BlogCTA locale={props.locale} />
            </>
          )}
        </section>
      </main>
    </SiteShell>
  );
}
