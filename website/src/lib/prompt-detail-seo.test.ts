import { describe, expect, test } from "bun:test";
import { LOCALES } from "./locales";
import type { PromptDetail } from "./prompt-detail-data";
import { buildPromptDetailMetadata, getPromptDetailSeoText } from "./prompt-detail-seo";

const detail = {
  id: "advertising-ecommerce",
  modelSlug: "seedance-2.0",
  modelId: "seedance-2.0",
  modelName: "seedance-2.0",
  kind: "video",
  label: "广告与电商营销团队",
  poster: "/assets/model-regeneration/example.jpg",
  prompt: "Creative direction: A product video.",
  ratio: "16:9",
  siblings: [],
} as PromptDetail;

describe("prompt detail SEO", () => {
  test("gives each locale a concise model and media specific title", () => {
    for (const locale of LOCALES) {
      const seo = getPromptDetailSeoText(detail, locale);
      expect(seo.title).toContain("seedance-2.0");
      expect(seo.title).toContain("Flatkey");
      expect(seo.title.length).toBeLessThanOrEqual(60);
      expect(seo.description).toContain("seedance-2.0");
    }
    expect(getPromptDetailSeoText(detail, "zh").title).toContain("视频提示词");
    expect(getPromptDetailSeoText({ ...detail, kind: "image" }, "zh").title).toContain("图片提示词");
  });

  test("sets a canonical, hreflang alternates and the matching example image", () => {
    const metadata = buildPromptDetailMetadata(detail, "zh");
    expect(metadata.alternates?.canonical).toBe("https://flatkey.ai/zh/models/seedance-2.0/prompts/advertising-ecommerce");
    expect(metadata.alternates?.languages).toHaveProperty("en-US");
    expect(metadata.alternates?.languages).not.toHaveProperty("id-ID");
    expect(metadata.description?.length).toBeLessThanOrEqual(155);
    expect(metadata.openGraph?.images).toEqual([{ url: "https://flatkey.ai/assets/model-regeneration/example.jpg" }]);
  });
});
