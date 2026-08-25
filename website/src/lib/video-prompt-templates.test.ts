import { describe, expect, test } from "bun:test";
import {
  VIDEO_MODEL_IDS,
  VIDEO_PROMPT_TEMPLATES,
  getVideoPromptTemplateFallbackPosters,
  getVideoPromptTemplates,
} from "./video-prompt-templates";

describe("video industry prompt templates", () => {
  test("defines six distinct industry briefs with fill-in slots", () => {
    expect(VIDEO_PROMPT_TEMPLATES).toHaveLength(6);
    expect(new Set(VIDEO_PROMPT_TEMPLATES.map((template) => template.id)).size).toBe(6);
    expect(new Set(VIDEO_PROMPT_TEMPLATES.map((template) => template.prompt)).size).toBe(6);

    for (const template of VIDEO_PROMPT_TEMPLATES) {
      expect(template.prompt.length).toBeGreaterThan(240);
      expect(template.prompt).toMatch(/\[[^\]]+\]/);
      expect(template.ratio).toMatch(/^\d+:\d+$/);
      expect(template.duration).toBeGreaterThan(0);
      expect(template.tags.length).toBeGreaterThan(0);
      expect(template.video).toMatch(/\.mp4$/);
      expect(template.prompt).not.toMatch(/\b(?:UGC|creator|presenter|portrait|selfie)\b/i);
    }
  });

  test("gives every dedicated video model six independent playable references", () => {
    const sets = VIDEO_MODEL_IDS.map((modelId) => getVideoPromptTemplateFallbackPosters(modelId));

    expect(VIDEO_MODEL_IDS).toHaveLength(10);
    for (const posters of sets) {
      expect(posters).toHaveLength(6);
      expect(new Set(posters).size).toBe(6);
      expect(posters.every((poster) => poster.startsWith("/assets/"))).toBe(true);
      expect(posters.every((poster) => !/ugc|creator|portrait|fashion-walk|v1\.[123]/i.test(poster))).toBe(true);
    }

    expect(new Set(sets.map((posters) => posters.join("|"))).size).toBe(10);
  });

  test("returns one model-specific template set and keeps unknown models empty", () => {
    const seedanceTemplates = getVideoPromptTemplates("seedance-2.5");
    const minimaxTemplates = getVideoPromptTemplates("MiniMax-H3");
    expect(seedanceTemplates).toHaveLength(6);
    expect(seedanceTemplates[0].poster).toBe("/assets/model-examples/product-macro.png");
    expect(seedanceTemplates.every((template) => template.video.endsWith(".mp4"))).toBe(true);
    expect(seedanceTemplates[0].prompt).not.toBe(minimaxTemplates[0].prompt);
    expect(seedanceTemplates.map((template) => template.video)).not.toEqual(minimaxTemplates.map((template) => template.video));
    expect(getVideoPromptTemplates("unknown-video-model")).toEqual([]);
  });
});
