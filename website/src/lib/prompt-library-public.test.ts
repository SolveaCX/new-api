import { describe, expect, test } from "bun:test";
import { LOCALES } from "./locales";
import {
  PROMPTS_PATH,
  filterPromptLibraryExamples,
  getPromptLibraryFilterOptions,
  getPromptLibraryExamples,
  getPromptLibraryMediaSummaries,
  getPromptLibraryMediaSummary,
  getPromptLibraryExampleBySlug,
  getPromptLibraryExamplesByMediaType,
  getPromptLibraryExamplesByModelSlug,
  getPromptLibraryModelSummaries,
  getPromptLibraryModelSummary,
  getPromptLibraryPageCopy,
  getPromptLibraryPromptPath,
  getPromptLibraryStaticPathnames,
  getPromptLibraryTypePath,
  getPromptLibraryModelPath,
} from "./prompt-library-public";

describe("public prompt library", () => {
  test("exposes a small set of attributed examples until the API feed is ready", () => {
    const items = getPromptLibraryExamples();

    expect(PROMPTS_PATH).toBe("/prompts");
    expect(items.length).toBeGreaterThanOrEqual(4);
    expect(items.length).toBeLessThanOrEqual(8);
    expect(items.some((item) => item.title === "Convenience Store Night Scene")).toBe(true);

    for (const item of items) {
      expect(item.prompt.length).toBeGreaterThan(20);
      expect(item.category).toBeTruthy();
      expect(item.pageType).toBeTruthy();
      expect(item.source.label).toBeTruthy();
      expect(item.source.url).toMatch(/^https:\/\//);
      expect(item.source.repository).toBe("ZeroLu/awesome-gpt-image");
    }
  });

  test("filters examples by page type, model, and search text", () => {
    const items = getPromptLibraryExamples();
    const options = getPromptLibraryFilterOptions(items);

    expect(options.pageTypes.length).toBeGreaterThanOrEqual(3);
    expect(options.models).toContain("GPT Image 2");

    const productExamples = filterPromptLibraryExamples(items, {
      model: "GPT Image 2",
      pageType: "product-page",
      query: "product",
    });

    expect(productExamples.length).toBeGreaterThan(0);
    for (const item of productExamples) {
      expect(item.model).toBe("GPT Image 2");
      expect(item.pageType).toBe("product-page");
      expect(
        `${item.title} ${item.prompt} ${item.tags.join(" ")}`.toLowerCase(),
      ).toContain("product");
    }
  });

  test("builds stable public prompt paths for media, model, and detail pages", () => {
    const items = getPromptLibraryExamples();
    const first = items[0];

    expect(getPromptLibraryExampleBySlug(first.slug)).toBe(first);
    expect(getPromptLibraryExamplesByMediaType("image").length).toBeGreaterThan(0);
    expect(getPromptLibraryExamplesByModelSlug("gpt-image-2").length).toBeGreaterThan(0);
    expect(getPromptLibraryTypePath("image")).toBe("/prompts/image");
    expect(getPromptLibraryTypePath("video")).toBe("/prompts/video");
    expect(getPromptLibraryTypePath("audio")).toBe("/prompts/audio");
    expect(getPromptLibraryModelPath("GPT Image 2")).toBe("/prompts/models/gpt-image-2");
    expect(getPromptLibraryPromptPath(first)).toBe(`/prompts/${first.slug}`);

    const pathnames = getPromptLibraryStaticPathnames();
    expect(pathnames).toContain("/prompts");
    expect(pathnames).toContain("/prompts/image");
    expect(pathnames).toContain("/prompts/video");
    expect(pathnames).toContain("/prompts/audio");
    expect(pathnames).toContain("/prompts/models/gpt-image-2");
    expect(pathnames).toContain(`/prompts/${first.slug}`);
  });

  test("summarizes prompts by media type and model", () => {
    const media = getPromptLibraryMediaSummaries();
    const models = getPromptLibraryModelSummaries();

    expect(media.map((item) => item.type)).toEqual(["image", "video", "audio"]);
    expect(media.find((item) => item.type === "image")?.count).toBeGreaterThan(0);
    expect(media.find((item) => item.type === "video")?.count).toBeGreaterThan(0);
    expect(media.find((item) => item.type === "audio")?.count).toBeGreaterThan(0);
    expect(getPromptLibraryMediaSummary("image")?.href).toBe("/prompts/image");

    expect(models.some((model) => model.slug === "gpt-image-2")).toBe(true);
    expect(models.some((model) => model.mediaType === "image")).toBe(true);
    expect(models.some((model) => model.mediaType === "video")).toBe(true);
    expect(models.some((model) => model.mediaType === "audio")).toBe(true);
    expect(getPromptLibraryModelSummary("sonilo-video-to-music")?.displayName).toContain("Sonilo");
  });

  test("provides localized page chrome for every supported locale", () => {
    const english = getPromptLibraryPageCopy("en");

    for (const locale of LOCALES) {
      const copy = getPromptLibraryPageCopy(locale);

      expect(copy.metaTitle).toContain("Flatkey");
      expect(copy.heroTitle).toBeTruthy();
      expect(copy.searchPlaceholder).toBeTruthy();
      expect(copy.copyPrompt).toBeTruthy();
      expect(copy.pageTypeFilterLabel).toBeTruthy();
      expect(copy.modelFilterLabel).toBeTruthy();
      expect(copy.pageTypes["product-page"]).toBeTruthy();

      if (locale !== "en" && locale !== "id") {
        expect(copy.searchPlaceholder).not.toBe(english.searchPlaceholder);
      }
    }
  });
});
