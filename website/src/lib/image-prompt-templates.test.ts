import { describe, expect, test } from "bun:test";
import {
  getImagePlaygroundExample,
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
      expect(template.poster).toMatch(/^(\/assets\/|\/use-case\/image-buddy\/|https:\/\/cdn\.shulex-voc\.com\/flatkey\/model-media\/)/);
      expect(template.poster).not.toMatch(/creator|portrait|ugc|medical|developer|terminal|fitness-app|streetwear/i);
      expect(template.ratio).toMatch(/^\d+:\d+$/);
      expect(template.tags.length).toBeGreaterThan(0);
    }

    expect(IMAGE_PROMPT_TEMPLATES[0].prompt).toMatch(/game UI and interaction designers/i);
    expect(IMAGE_PROMPT_TEMPLATES[1].prompt).toMatch(/sports broadcasters, live-event directors/i);
    expect(IMAGE_PROMPT_TEMPLATES[2].prompt).toMatch(/brand, advertising, and ecommerce teams/i);
    expect(IMAGE_PROMPT_TEMPLATES[3].prompt).toMatch(/film directors, storyboard artists/i);
    expect(IMAGE_PROMPT_TEMPLATES[4].prompt).toMatch(/short-drama writers, comedy creators/i);
    expect(IMAGE_PROMPT_TEMPLATES[5].prompt).toMatch(/archivists, museums, historians/i);
    for (const template of IMAGE_PROMPT_TEMPLATES) {
      expect(template.prompt).not.toMatch(/real creator|professional avatar|person or role|natural hands and skin/i);
    }
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
    expect(getImagePromptTemplate("product-hero")?.label).toBe("Game UI interaction and equipment switching");
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
    expect(first[0]).toMatch(/game-ui-equipment/);
    expect(first[1]).toMatch(/sports-broadcast/);
    expect(first[2]).toMatch(/brand-tvc-ecommerce/);
    expect(first[3]).toMatch(/cinematic-storyboard/);
    expect(first[4]).toMatch(/comedy-physical/);
    expect(first[5]).toMatch(/historical-revival/);
    expect(first.some((poster) => /creator|portrait|ugc|medical|developer|terminal|fitness-app|streetwear/i.test(poster))).toBe(false);
  });

  test("gives every canonical image model a distinct non-human poster set", () => {
    const modelIds = [
      "gpt-image-2",
      "gemini-2.5-flash-image",
      "gemini-3-pro-image",
      "gemini-3.1-flash-image",
      "gemini-3.1-flash-lite-image",
      "grok-imagine-image",
      "grok-imagine-image-pro",
      "grok-imagine-image-quality",
      "nano-banana-pro-preview",
    ];
    const sets = modelIds.map((modelId) => getImagePromptTemplateFallbackPosters(modelId));
    const serialized = sets.map((posters) => posters.join("\n"));

    expect(new Set(serialized).size).toBe(modelIds.length);
    for (const posters of sets) {
      expect(posters).toHaveLength(IMAGE_PROMPT_TEMPLATES.length);
      expect(new Set(posters).size).toBe(posters.length);
      expect(posters.every((poster) => !/creator|portrait|ugc|medical|developer|terminal|fitness-app|streetwear/i.test(poster))).toBe(true);
    }
  });

  test("gives every canonical image model a distinct ecommerce Playground starter", () => {
    const modelIds = [
      "gpt-image-2",
      "gemini-2.5-flash-image",
      "gemini-3-pro-image",
      "gemini-3.1-flash-image",
      "gemini-3.1-flash-lite-image",
      "grok-imagine-image",
      "grok-imagine-image-pro",
      "grok-imagine-image-quality",
      "nano-banana-pro-preview",
    ];
    const examples = modelIds.map((modelId) => getImagePlaygroundExample(modelId));

    expect(examples.every(Boolean)).toBe(true);
    expect(new Set(examples.map((example) => example?.poster)).size).toBe(modelIds.length);
    expect(new Set(examples.map((example) => example?.prompt)).size).toBe(modelIds.length);
    expect(examples.every((example) => example?.industry === "ecommerce-retail")).toBe(true);
  });

  test("resolves image model aliases used by the pricing catalog", () => {
    expect(getImagePlaygroundExample("gemini-3.1-flash-image-preview")?.poster).toContain("high-end-skincare-product-poster");
    expect(getImagePlaygroundExample("gemini_2_5_flash_image_preview")?.poster).toContain("three-day-travel-guide-card");
    expect(getImagePlaygroundExample("unknown-image-model")).toBeUndefined();
  });
});
