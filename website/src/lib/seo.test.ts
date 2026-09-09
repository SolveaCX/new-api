import { describe, expect, test } from "bun:test";
import {
  buildMetadata,
  getSeoLocaleOptions,
  HOMEPAGE_SOCIAL_IMAGE,
  isFallbackLocaleNoIndex,
  seoIndexableLocales,
} from "./seo";

describe("buildMetadata", () => {
  test("adds a default social image for Open Graph and Twitter metadata", () => {
    const metadata = buildMetadata({
      title: "flatkey.ai pricing",
      description: "One API key for every top AI model.",
      pathname: "/pricing",
    });

    expect(metadata.openGraph?.images).toEqual([
      { url: "https://flatkey.ai/flatkey-logo-light.png" },
    ]);
    expect(metadata.twitter?.images).toEqual([
      "https://flatkey.ai/flatkey-logo-light.png",
    ]);
  });

  test("keeps page-specific social images when provided", () => {
    const metadata = buildMetadata({
      title: "Blog post",
      description: "A specific article.",
      pathname: "/blog/post",
      image: "https://cdn.example.test/post.png",
    });

    expect(metadata.openGraph?.images).toEqual([
      { url: "https://cdn.example.test/post.png" },
    ]);
    expect(metadata.twitter?.images).toEqual([
      "https://cdn.example.test/post.png",
    ]);
  });

  test("uses the homepage social image when supplied by the homepage route", () => {
    const metadata = buildMetadata({
      title: "flatkey homepage",
      description: "One key for frontier models and tools.",
      pathname: "/",
      image: HOMEPAGE_SOCIAL_IMAGE,
    });

    expect(metadata.openGraph?.images).toEqual([
      { url: "https://flatkey.ai/assets/og-image.png" },
    ]);
    expect(metadata.twitter?.images).toEqual([
      "https://flatkey.ai/assets/og-image.png",
    ]);
  });
  test("supports absolute titles for model pages", () => {
    const metadata = buildMetadata({
      title: "Claude Fable 5.1 API | Flatkey",
      description: "A model detail page.",
      pathname: "/models/claude-fable-5.1",
      absoluteTitle: true,
    });

    expect(metadata.title).toEqual({ absolute: "Claude Fable 5.1 API | Flatkey" });
  });

  test("keeps x-default inside the advertised locale set", () => {
    const metadata = buildMetadata({
      title: "Careers",
      description: "Join flatkey.",
      pathname: "/careers",
      locale: "zh",
      locales: ["en", "zh"],
    });

    expect(metadata.alternates?.languages?.["x-default"]).toBe("https://flatkey.ai/careers");
  });

  test("uses the only available locale for x-default on localized-only pages", () => {
    const metadata = buildMetadata({
      title: "Portuguese promotion",
      description: "A localized offer.",
      pathname: "/5-credit-promo",
      locale: "pt",
      locales: ["pt"],
    });

    expect(metadata.alternates?.languages?.["x-default"]).toBe("https://flatkey.ai/pt/5-credit-promo");
  });

  test("keeps known English-fallback locale pages out of indexing", () => {
    expect(isFallbackLocaleNoIndex("/about", "id")).toBe(true);
    expect(isFallbackLocaleNoIndex("/about", "pt")).toBe(false);
    expect(isFallbackLocaleNoIndex("/playground", "pt")).toBe(true);
    expect(seoIndexableLocales("/about")).not.toContain("id");
    expect(seoIndexableLocales("/about")).toContain("pt");
    expect(seoIndexableLocales("/playground")).not.toContain("id");
    expect(seoIndexableLocales("/playground")).not.toContain("pt");
  });

  test("returns noindex metadata options only for fallback locale variants", () => {
    expect(getSeoLocaleOptions("/about", "id")).toEqual({ locales: [], noIndex: true });
    expect(getSeoLocaleOptions("/about", "zh")).toEqual({
      locales: ["en", "zh", "es", "fr", "pt", "ru", "ja", "vi", "de"],
      noIndex: false,
    });
  });

  test("automatically noindexes Indonesian fallback pages and removes hreflang", () => {
    const metadata = buildMetadata({
      title: "Models",
      description: "Browse models.",
      pathname: "/models",
      locale: "id",
    });

    expect(metadata.robots).toEqual({ index: false, follow: false });
    expect(metadata.alternates).toEqual({ canonical: "https://flatkey.ai/id/models" });
  });
});
