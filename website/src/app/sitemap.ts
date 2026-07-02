import type { MetadataRoute } from "next";
import { getAllBlogPosts, getBlogCategories } from "@/lib/blog";
import { LOCALES, type Locale, localizePath } from "@/lib/locales";
import { getModelLandingPathnames } from "@/lib/model-landing";
import { getPricingData } from "@/lib/pricing";
import { buildPricingSeoIndex } from "@/lib/pricing-seo";
import { SITE_ORIGIN } from "@/lib/origins";

const base = SITE_ORIGIN;

function entry(
  pathname: string,
  priority: number,
  changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"],
  locales: readonly Locale[] = LOCALES
) {
  return locales.map((locale) => ({
    url: `${base}${localizePath(pathname, locale)}`,
    lastModified: new Date(),
    changeFrequency,
    priority,
    alternates: {
      languages: Object.fromEntries(locales.map((locale) => [locale, `${base}${localizePath(pathname, locale)}`])),
    },
  }));
}

function queryEntry(
  pathname: string,
  query: string,
  priority: number,
  changeFrequency: MetadataRoute.Sitemap[number]["changeFrequency"]
) {
  return LOCALES.map((locale) => ({
    url: `${base}${localizePath(pathname, locale)}?${query}`,
    lastModified: new Date(),
    changeFrequency,
    priority,
    alternates: {
      languages: Object.fromEntries(LOCALES.map((locale) => [locale, `${base}${localizePath(pathname, locale)}?${query}`])),
    },
  }));
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const [localizedPosts, categories, pricing] = await Promise.all([
    Promise.all(LOCALES.map(async (locale) => ({ locale, posts: await getAllBlogPosts(locale) }))),
    getBlogCategories(),
    getPricingData(),
  ]);
  const staticEntries = [
    ...entry("/", 1, "daily"),
    ...entry("/pricing", 0.8, "daily"),
    ...entry("/models", 0.82, "daily"),
    ...entry("/use-case/codex", 0.84, "weekly"),
    ...entry("/use-case/claude-code", 0.84, "weekly"),
    ...entry("/use-case/image-buddy", 0.84, "weekly"),
    ...entry("/rankings", 0.7, "daily"),
    ...entry("/about", 0.5, "monthly"),
    ...entry("/blog", 0.9, "daily"),
    ...entry("/terms", 0.3, "yearly"),
    ...entry("/privacy", 0.3, "yearly"),
    ...entry("/sla", 0.3, "yearly"),
    ...entry("/refund-policy", 0.3, "yearly"),
  ];
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
    const availableLocales = LOCALES.filter((locale) => locales[locale]);
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
              availableLocale,
              `${base}${localizePath(`/blog/${slug}`, availableLocale)}`,
            ])
          ),
        },
      };
    });
  });
  const pricingIndex = buildPricingSeoIndex(pricing);
  const modelLandingEntries = getModelLandingPathnames().flatMap((pathname) => entry(pathname, 0.82, "daily"));
  const vendorPricingEntries = pricingIndex.vendors.slice(0, 80).flatMap((vendor) =>
    queryEntry("/pricing", `vendor=${encodeURIComponent(vendor.slug)}`, 0.72, "daily")
  );
  const vendorPageEntries = pricingIndex.vendors.flatMap((vendor) => entry(`/vendors/${vendor.slug}`, 0.74, "daily"));
  const modelPageEntries = pricingIndex.models.flatMap((model) => entry(`/models/${model.slug}`, 0.76, "daily"));

  return [
    ...staticEntries,
    ...modelLandingEntries,
    ...vendorPricingEntries,
    ...vendorPageEntries,
    ...modelPageEntries,
    ...categoryEntries,
    ...postEntries,
  ];
}
