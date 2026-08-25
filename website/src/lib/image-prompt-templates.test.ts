import { describe, expect, test } from "bun:test";
import {
  getImagePromptTemplate,
  getImagePromptTemplates,
  IMAGE_PROMPT_TEMPLATES,
} from "./image-prompt-templates";

describe("image prompt templates", () => {
  test("covers the main image-production scenarios", () => {
    expect(IMAGE_PROMPT_TEMPLATES.length).toBeGreaterThanOrEqual(5);

    const ids = new Set(IMAGE_PROMPT_TEMPLATES.map((template) => template.id));
    expect(ids.size).toBe(IMAGE_PROMPT_TEMPLATES.length);

    for (const template of IMAGE_PROMPT_TEMPLATES) {
      expect(template.prompt.length).toBeGreaterThan(120);
      expect(template.prompt).toContain("[");
      expect(template.poster).toMatch(/^\/assets\/model-examples\/image2\//);
      expect(template.ratio).toMatch(/^\d+:\d+$/);
      expect(template.tags.length).toBeGreaterThan(0);
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
    expect(getImagePromptTemplate("product-hero")?.label).toBe("Product mockups");
    expect(getImagePromptTemplate("missing-template")).toBeUndefined();
  });
});
