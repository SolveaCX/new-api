import type { MetadataRoute } from "next";
import { getAllBlogPosts, getBlogCategories } from "@/lib/blog";
import { CLI_IMAGE_PATH, CLI_LANDING_PATH, CLI_VIDEO_PATH, HIGGSFIELD_ALTERNATIVE_PATH } from "@/lib/cli-landing";
import { LOCALES, type Locale, localeLanguageTag, localizePath } from "@/lib/locales";
import { getMarketPathnames } from "@/lib/market-landing";
import { getModelLandingConfigForPricingModel, getModelLandingPathnames } from "@/lib/model-landing";
import { seriesForModels } from "@/lib/model-directory-meta";
import { seoIndexableLocales } from "@/lib/seo";
import { getSkagLandingLocales, SKAG_LANDING_SLUGS, skagLandingPath } from "@/lib/skag-landing";
import { getToolsAdLandingPathnames } from "@/lib/tools-ad-landing";
import { TOOLS_LANDING_PATH } from "@/lib/tools-landing";
import { APIFY_ALTERNATIVE_PATH } from "@/lib/tools-conquest-landing";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { getCliMediaPromptItems } from "@/lib/prompt-library";
import { SITE_ORIGIN } from "@/lib/origins";

// The model list comes from the live public catalog. Do not prerender this
// route during a website build where the console API may be unavailable.
export const dynamic = "force-dynamic";

const base = SITE_ORIGIN;
const REDIRECT_MODEL_LANDING_PATHS = new Set([
  "/models/gpt-api",
  "/models/claude-api",
  "/models/seedance-2-5",
]);

function entry(
  pathname: string,
  priority: number,
  changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"],
  locales: readonly Locale[] = LOCALES
) {
  const indexableLocales = seoIndexableLocales(pathname, locales);
  return indexableLocales.map((locale) => ({
    url: `${base}${localizePath(pathname, locale)}`,
    lastModified: new Date(),
    changeFrequency,
    priority,
    alternates: {
      languages: Object.fromEntries(
        indexableLocales.map((locale) => [localeLanguageTag(locale), `${base}${localizePath(pathname, locale)}`])
      ),
    },
  }));
}

/** Like `entry`, but for a filtered view whose state lives in the query string. */
function queryEntry(
  pathname: string,
  query: string,
  priority: number,
  changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"]
) {
  const indexableLocales = seoIndexableLocales(pathname);
  return indexableLocales.map((locale) => ({
    url: `${base}${localizePath(pathname, locale)}?${query}`,
    lastModified: new Date(),
    changeFrequency,
    priority,
    alternates: {
      languages: Object.fromEntries(
        indexableLocales.map((alternate) => [localeLanguageTag(alternate), `${base}${localizePath(pathname, alternate)}?${query}`])
      ),
    },
  }));
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const [localizedPosts, categories, pricing] = await Promise.all([
    Promise.all(LOCALES.map(async (locale) => ({ locale, posts: await getAllBlogPosts(locale) }))),
    getBlogCategories(),
    // Keep the sitemap model set identical to the model detail pages and the
    // public directory. The unscoped endpoint can expose models from groups
    // that are not available on the public (PLG) detail route.
    getPricingData(WEBSITE_PUBLIC_PRICING_GROUP),
  ]);
  // A transient pricing outage must not produce a successful but incomplete
  // sitemap. Google will retry a 5xx response; serving only static entries
  // would make the entire live model catalog disappear from discovery.
  if (pricing.models.length === 0) {
    throw new Error("Sitemap pricing catalog is unavailable");
  }
  const staticEntries = [
    ...entry("/", 1, "daily"),
    ...entry("/pricing", 0.8, "daily"),
    ...entry("/models", 0.82, "daily"),
    ...entry(TOOLS_LANDING_PATH, 0.9, "daily"),
    ...entry("/docs", 0.7, "weekly"),
    ...entry("/playground", 0.7, "weekly"),
    ...entry("/compute", 0.7, "weekly"),
    ...entry("/usecases", 0.7, "weekly"),
    ...entry("/status", 0.65, "daily"),
    ...entry(APIFY_ALTERNATIVE_PATH, 0.84, "weekly", ["en"]),
    ...entry(CLI_LANDING_PATH, 0.86, "weekly"),
    ...entry(HIGGSFIELD_ALTERNATIVE_PATH, 0.84, "weekly"),
    ...entry("/use-case/codex", 0.84, "weekly"),
    ...entry("/use-case/claude-code", 0.84, "weekly"),
    ...entry("/use-case/image-buddy", 0.84, "weekly"),
    ...entry("/glm-5-2", 0.86, "daily"),
    ...entry("/5-credit-promo", 0.8, "weekly", ["pt"]),
    ...entry("/rankings", 0.7, "daily"),
    ...entry("/about", 0.5, "monthly"),
    ...entry("/careers", 0.6, "monthly", ["en", "zh"]),
    ...entry("/careers/business-development-representative", 0.7, "monthly", ["en", "zh"]),
    ...entry("/contact", 0.5, "monthly"),
    ...entry("/blog", 0.9, "daily"),
    ...entry("/terms", 0.3, "yearly"),
    ...entry("/privacy", 0.3, "yearly"),
    ...entry("/sla", 0.3, "yearly"),
    ...entry("/refund-policy", 0.3, "yearly"),
  ];
  const modelLandingPathnames = getModelLandingPathnames()
    .filter((pathname) => !REDIRECT_MODEL_LANDING_PATHS.has(pathname))
  const modelLandingEntries = modelLandingPathnames.flatMap((pathname) => entry(pathname, 0.82, "daily"));
  const skagLandingEntries = SKAG_LANDING_SLUGS.flatMap((slug) =>
    entry(skagLandingPath(slug), 0.8, "weekly", getSkagLandingLocales(slug))
  );
  const toolsAdLandingEntries = getToolsAdLandingPathnames().flatMap((pathname) => entry(pathname, 0.8, "weekly", ["en"]));
  // Every live model gets its own public page (/models/<name>); include them so
  // search engines discover the full catalog, not just the curated landings.
  const landingPaths = new Set(modelLandingPathnames);
  const modelPublicEntries = pricing.models
    .flatMap((model) => {
      // Resolve the same canonical slug used by the detail route. This keeps
      // casing-only aliases (for example MiniMax-H3) out of the sitemap and
      // avoids duplicate entries for curated single-model landings.
      const canonicalPath = `/models/${getModelLandingConfigForPricingModel(model).slug}`;
      if (REDIRECT_MODEL_LANDING_PATHS.has(canonicalPath) || landingPaths.has(canonicalPath)) return [];
      return entry(canonicalPath, 0.6, "daily");
    });
  // Market acquisition pages are single-locale (no i18n alternates by design).
  const marketEntries = getMarketPathnames().map((pathname) => ({
    url: `${base}${pathname}`,
    lastModified: new Date(),
    changeFrequency: "weekly" as const,
    priority: 0.84,
  }));
  const categoryEntries = categories.flatMap((category) => entry(`/blog/category/${category.slug}`, 0.7, "weekly"));
  const postsBySlug = new Map<string, Partial<Record<Locale, { date?: string }>>>();

  for (const { locale, posts } of localizedPosts) {
    for (const post of posts) {
      const existing = postsBySlug.get(post.slug) ?? {};
      existing[locale] = { date: post.date };
      postsBySlug.set(post.slug, existing);
    }
  }

  const postEntries = Array.from(postsBySlug.entries()).flatMap(([slug, locales]) => {
    // Keep the sitemap in sync with route-level robots metadata. In
    // particular, fallback locales such as Indonesian are intentionally
    // noindex until their copy is reviewed and must not be advertised in the
    // sitemap or as hreflang alternates.
    const availableLocales = seoIndexableLocales(
      `/blog/${slug}`,
      LOCALES.filter((locale) => locales[locale]),
    );
    return availableLocales.map((locale) => {
      const localizedPost = locales[locale];
      return {
        url: `${base}${localizePath(`/blog/${slug}`, locale)}`,
        lastModified: localizedPost?.date ? new Date(localizedPost.date) : new Date(),
        changeFrequency: "monthly" as const,
        priority: 0.8,
        alternates: {
          languages: Object.fromEntries(
            availableLocales.map((availableLocale) => [
              localeLanguageTag(availableLocale),
              `${base}${localizePath(`/blog/${slug}`, availableLocale)}`,
            ])
          ),
        },
      };
    });
  });
  // Single-series directory views are indexable landing pages ("Claude API
  // pricing"), so they need to be discoverable. Any richer filter combination
  // is noindex — see model-directory-seo.ts — and is deliberately absent here.
  const seriesEntries = seriesForModels(pricing.models.map((model) => model.directory_metadata)).flatMap((series) =>
    queryEntry("/models", `series=${encodeURIComponent(series)}`, 0.72, "daily")
  );
  const cliMediaDetailEntries = [
    { kind: "image" as const, path: CLI_IMAGE_PATH },
    { kind: "video" as const, path: CLI_VIDEO_PATH },
  ].flatMap(({ kind, path }) =>
    getCliMediaPromptItems(kind).flatMap((item) => entry(`${path}/${item.slug}`, 0.65, "monthly"))
  );

  return [
    ...staticEntries,
    ...marketEntries,
    ...modelLandingEntries,
    ...skagLandingEntries,
    ...toolsAdLandingEntries,
    ...cliMediaDetailEntries,
    ...modelPublicEntries,
    ...seriesEntries,
    ...categoryEntries,
    ...postEntries,
  ];
}
