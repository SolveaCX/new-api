import type { Metadata } from "next";
import { DEFAULT_LOCALE, LOCALES, type Locale, localeAlternates, localeLanguageTag, localizePath } from "./locales";
import { SITE_ORIGIN } from "./origins";

// Keep the historic seo.ts export stable for schema and route consumers while
// sourcing the value from the environment-aware origin module.
export { SITE_ORIGIN } from "./origins";

export const SITE_NAME = "flatkey.ai";
export const DEFAULT_SOCIAL_IMAGE = `${SITE_ORIGIN}/flatkey-logo-light.png`;
export const HOMEPAGE_SOCIAL_IMAGE = `${SITE_ORIGIN}/assets/og-image.png`;

export type SeoInput = {
  title: string;
  /** Keep a page-specific brand suffix from being processed by the root title template. */
  absoluteTitle?: boolean;
  description: string;
  pathname: string;
  locale?: Locale;
  image?: string;
  noIndex?: boolean;
  /** Locales the page actually exists in. Defaults to all locales; pass ["en"] for English-only pages so hreflang never points at 404s. */
  locales?: readonly Locale[];
  /**
   * Physical single-locale routes (e.g. market pages /br, /id-market) that only
   * exist at their literal pathname: canonical is the raw pathname (never
   * locale-prefixed) and no hreflang alternates are emitted.
   */
  unlocalized?: boolean;
};

// Keep localized routes with English fallback copy out of the index until
// their content is genuinely localized. The pages remain accessible for
// users, but should not compete with their English canonical counterparts.
const FALLBACK_LOCALE_NOINDEX_PATHS: Partial<Record<Locale, readonly string[]>> = {
  id: ["/about", "/docs", "/playground", "/privacy", "/refund-policy", "/sla", "/terms", "/usecases"],
  pt: ["/playground"],
};

// Indonesian copy is still English fallback across the public site. Keep all
// `/id/*` variants out of the index until each page has reviewed Indonesian
// content; otherwise Google may select the English URL as the canonical.
const FALLBACK_LOCALE_NOINDEX_LOCALES: readonly Locale[] = ["id"];

function normalizeSeoPathname(pathname: string): string {
  const withoutQuery = pathname.split(/[?#]/, 1)[0] ?? pathname;
  if (withoutQuery === "/") return "/";
  return withoutQuery.replace(/\/+$/, "") || "/";
}

export function isFallbackLocaleNoIndex(pathname: string, locale: Locale): boolean {
  return (
    FALLBACK_LOCALE_NOINDEX_LOCALES.includes(locale) ||
    FALLBACK_LOCALE_NOINDEX_PATHS[locale]?.includes(normalizeSeoPathname(pathname)) === true
  );
}

export function seoIndexableLocales(pathname: string, locales: readonly Locale[] = LOCALES): Locale[] {
  const normalized = normalizeSeoPathname(pathname);
  return locales.filter((locale) => !isFallbackLocaleNoIndex(normalized, locale));
}

export function getSeoLocaleOptions(pathname: string, locale: Locale): Pick<SeoInput, "locales" | "noIndex"> {
  const noIndex = isFallbackLocaleNoIndex(pathname, locale);
  return {
    locales: noIndex ? [] : seoIndexableLocales(pathname),
    noIndex,
  };
}

export function buildMetadata(input: SeoInput): Metadata {
  const locale = input.locale ?? DEFAULT_LOCALE;
  const fallbackLocaleNoIndex = isFallbackLocaleNoIndex(input.pathname, locale);
  const noIndex = input.noIndex === true || fallbackLocaleNoIndex;
  const locales = input.locales ?? (fallbackLocaleNoIndex ? [] : seoIndexableLocales(input.pathname));
  const canonicalPath = input.unlocalized ? input.pathname : localizePath(input.pathname, locale);
  const canonical = `${SITE_ORIGIN}${canonicalPath}`;
  const title = input.title;
  const socialImage = input.image ?? DEFAULT_SOCIAL_IMAGE;

  return {
    title: input.absoluteTitle ? { absolute: title } : title,
    description: input.description,
    metadataBase: new URL(SITE_ORIGIN),
    alternates: {
      canonical,
      ...(input.unlocalized || fallbackLocaleNoIndex
        ? {}
        : {
            languages: {
              ...(locales && locales.length < LOCALES.length
                ? Object.fromEntries(
                    locales.map((altLocale) => [
                      localeLanguageTag(altLocale),
                      `${SITE_ORIGIN}${localizePath(input.pathname, altLocale)}`,
                    ])
                  )
                : localeAlternates(input.pathname)),
              "x-default": `${SITE_ORIGIN}${localizePath(
                input.pathname,
                locales?.includes(DEFAULT_LOCALE) ? DEFAULT_LOCALE : (locales?.[0] ?? DEFAULT_LOCALE),
              )}`,
            },
          }),
    },
    robots: noIndex
      ? { index: false, follow: false }
      : { index: true, follow: true },
    openGraph: {
      title,
      description: input.description,
      url: canonical,
      siteName: SITE_NAME,
      type: "website",
      images: [{ url: socialImage }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description: input.description,
      images: [socialImage],
    },
  };
}
