import type { Metadata } from "next";
import { DEFAULT_LOCALE, LOCALES, type Locale, localeAlternates, localizePath } from "./locales";

export const SITE_ORIGIN = "https://flatkey.ai";
export const SITE_NAME = "flatkey.ai";

export type SeoInput = {
  title: string;
  description: string;
  pathname: string;
  locale?: Locale;
  image?: string;
  noIndex?: boolean;
  /** Locales the page actually exists in. Defaults to all locales; pass ["en"] for English-only pages so hreflang never points at 404s. */
  locales?: readonly Locale[];
};

export function buildMetadata(input: SeoInput): Metadata {
  const locale = input.locale ?? DEFAULT_LOCALE;
  const canonicalPath = localizePath(input.pathname, locale);
  const canonical = `${SITE_ORIGIN}${canonicalPath}`;
  const title = input.title;

  return {
    title,
    description: input.description,
    metadataBase: new URL(SITE_ORIGIN),
    alternates: {
      canonical,
      languages: {
        ...(input.locales && input.locales.length < LOCALES.length
          ? Object.fromEntries(
              input.locales.map((altLocale) => [altLocale, `${SITE_ORIGIN}${localizePath(input.pathname, altLocale)}`])
            )
          : localeAlternates(input.pathname)),
        "x-default": `${SITE_ORIGIN}${localizePath(input.pathname, DEFAULT_LOCALE)}`,
      },
    },
    robots: input.noIndex
      ? { index: false, follow: false }
      : { index: true, follow: true },
    openGraph: {
      title,
      description: input.description,
      url: canonical,
      siteName: SITE_NAME,
      type: "website",
      images: input.image ? [{ url: input.image }] : undefined,
    },
    twitter: {
      card: "summary_large_image",
      title,
      description: input.description,
      images: input.image ? [input.image] : undefined,
    },
  };
}
