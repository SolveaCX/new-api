import { describe, expect, test } from "bun:test";
import {
  getImagePromptTemplate,
  getImagePromptTemplateFallbackPosters,
  getImagePromptTemplates,
  IMAGE_PROMPT_TEMPLATES,
} from "./image-prompt-templates";

describe("image prompt templates", () => {
  test("covers the main image-production scenarios", () => {
    expect(IMAGE_PROMPT_TEMPLATES).toHaveLength(6);

    const ids = new Set(IMAGE_PROMPT_TEMPLATES.map((template) => template.id));
    expect(ids.size).toBe(IMAGE_PROMPT_TEMPLATES.length);
    const prompts = new Set(IMAGE_PROMPT_TEMPLATES.map((template) => template.prompt));
    expect(prompts.size).toBe(IMAGE_PROMPT_TEMPLATES.length);
    const posters = new Set(IMAGE_PROMPT_TEMPLATES.map((template) => template.poster));
    expect(posters.size).toBe(IMAGE_PROMPT_TEMPLATES.length);

    for (const template of IMAGE_PROMPT_TEMPLATES) {
      expect(template.prompt.length).toBeGreaterThan(120);
      expect(template.prompt).toContain("[");
      expect(template.poster).toMatch(/^\/assets\/(model-examples\/image2|prompts\/awesome-images)\//);
      expect(template.ratio).toMatch(/^\d+:\d+$/);
      expect(template.tags.length).toBeGreaterThan(0);
    }

    expect(IMAGE_PROMPT_TEMPLATES[0].prompt).toMatch(/ecommerce and retail teams/i);
    expect(IMAGE_PROMPT_TEMPLATES[1].prompt).toMatch(/consumer brands and growth teams/i);
    expect(IMAGE_PROMPT_TEMPLATES[2].prompt).toMatch(/fashion and sports retailers/i);
    expect(IMAGE_PROMPT_TEMPLATES[3].prompt).toMatch(/creator, community, and customer-facing teams/i);
    expect(IMAGE_PROMPT_TEMPLATES[4].prompt).toMatch(/SaaS and mobile-product teams/i);
    expect(IMAGE_PROMPT_TEMPLATES[5].prompt).toMatch(/restaurants and beverage brands/i);
  });

  test("returns an independent catalog for every image model", () => {
    const first = getImagePromptTemplates("gpt-image-2");
    const second = getImagePromptTemplates("gemini-3.1-flash-image-preview");

    expect(first).toHaveLength(IMAGE_PROMPT_TEMPLATES.length);
    expect(second).toHaveLength(IMAGE_PROMPT_TEMPLATES.length);
    expect(first).not.toBe(IMAGE_PROMPT_TEMPLATES);
    expect(first[0]).not.toBe(IMAGE_PROMPT_TEMPLATES[0]);
    expect(first[0].prompt).toBe(second[0].prompt);

    first[0].tags.push("test-only");
    expect(second[0].tags).not.toContain("test-only");
  });

  test("resolves a template by its stable id", () => {
    expect(getImagePromptTemplate("product-hero")?.label).toBe("Product mockups");
    expect(getImagePromptTemplate("missing-template")).toBeUndefined();
  });

  test("chooses industry-compatible poster variants per model", () => {
    const first = getImagePromptTemplateFallbackPosters("qwen-image-2512");
    const second = getImagePromptTemplateFallbackPosters("flux-image-1");

    expect(first).toHaveLength(IMAGE_PROMPT_TEMPLATES.length);
    expect(new Set(first).size).toBe(IMAGE_PROMPT_TEMPLATES.length);
    expect(second).toHaveLength(IMAGE_PROMPT_TEMPLATES.length);
    expect(new Set(second).size).toBe(IMAGE_PROMPT_TEMPLATES.length);
    expect(first).not.toEqual(second);
    expect(first[0]).toMatch(/ecommerce-skincare|skincare/);
    expect(first[1]).toMatch(/ugc-coffee-ad|flatkey-image2-creator/);
    expect(first[5]).toMatch(/coffee|ugc-coffee-ad/);
    expect(first.some((poster) => poster.includes("developer"))).toBe(false);
  });
});
