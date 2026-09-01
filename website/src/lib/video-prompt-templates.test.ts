import { describe, expect, test } from "bun:test";
import {
  VIDEO_MODEL_IDS,
  VIDEO_PROFESSION_MODEL_IDS,
  VIDEO_PROFESSION_IDS,
  VIDEO_PROMPT_TEMPLATES,
  getVideoPlaygroundPrompt,
  getVideoPromptTemplateFallbackPosters,
  getVideoPromptTemplates,
  localizeVideoPromptText,
} from "./video-prompt-templates";
import { LOCALES } from "./locales";

describe("video profession prompt templates", () => {
  test("defines six ready-to-run English scene briefs", () => {
    expect(VIDEO_PROMPT_TEMPLATES).toHaveLength(6);
    expect(new Set(VIDEO_PROMPT_TEMPLATES.map((template) => template.id)).size).toBe(6);
    expect(VIDEO_PROMPT_TEMPLATES.map((template) => template.professionId)).toEqual([...VIDEO_PROFESSION_IDS]);
    expect(VIDEO_PROMPT_TEMPLATES.map((template) => template.label)).toEqual([
      "Micro-drama and comic creators",
      "Advertising and ecommerce teams",
      "Film concept and production teams",
      "Game art and animation teams",
      "Creator and explainer channels",
      "Music producers and visual artists",
    ]);
    expect(new Set(VIDEO_PROMPT_TEMPLATES.map((template) => template.professionId)).size).toBe(6);
    expect(new Set(VIDEO_PROMPT_TEMPLATES.map((template) => template.prompt)).size).toBe(6);

    for (const template of VIDEO_PROMPT_TEMPLATES) {
      expect(template.prompt.length).toBeGreaterThan(120);
      expect(template.prompt).not.toMatch(/\[[^\]]+\]/);
      expect(template.ratio).toMatch(/^\d+:\d+$/);
      expect(template.duration).toBeGreaterThan(0);
      expect(template.tags.length).toBeGreaterThan(0);
      expect(template.video).toMatch(/\.mp4$/);
    }

    expect(VIDEO_PROMPT_TEMPLATES[0].prompt).toMatch(/manga|courier|station/i);
    expect(VIDEO_PROMPT_TEMPLATES[1].prompt).toMatch(/travel kettle|steam/i);
    expect(VIDEO_PROMPT_TEMPLATES[2].prompt).toMatch(/sci-fi|command deck/i);
    expect(VIDEO_PROMPT_TEMPLATES[3].prompt).toMatch(/open-world|luminous ridge/i);
    expect(VIDEO_PROMPT_TEMPLATES[4].prompt).toMatch(/space-science|planet model/i);
    expect(VIDEO_PROMPT_TEMPLATES[5].prompt).toMatch(/stage-projection|performer silhouette/i);
  });

  test("binds every dedicated model clip to the matching profession ID", () => {
    const dedicatedModels = VIDEO_PROFESSION_MODEL_IDS;
    for (const modelId of dedicatedModels) {
      const cards = getVideoPromptTemplates(modelId, "en");
      expect(cards).toHaveLength(VIDEO_PROFESSION_IDS.length);
      cards.forEach((card, index) => {
        expect(card.professionId).toBe(VIDEO_PROFESSION_IDS[index]);
        expect(card.video).toContain(`/video-profession-${String(index + 1).padStart(2, "0")}/`);
        expect(card.prompt).not.toMatch(/\[[^\]]+\]/);
      });
    }

    // MiniMax-H3 uses the six clips reviewed with Seedance 2.0. The stable
    // profession IDs keep those clips attached to the right card.
    const minimaxCards = getVideoPromptTemplates("minimax-h3", "en");
    expect(minimaxCards).toHaveLength(VIDEO_PROFESSION_IDS.length);
    expect(minimaxCards.map((card) => card.professionId)).toEqual([...VIDEO_PROFESSION_IDS]);
    expect(minimaxCards[0].video).toBe(
      "https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-01/minimax-h3-seedance-2-0.mp4",
    );
  });

  test("keeps the legacy model registry separate from profession media", () => {
    expect(VIDEO_MODEL_IDS).toHaveLength(10);
    expect(VIDEO_PROFESSION_MODEL_IDS).toHaveLength(10);
    expect(getVideoPromptTemplateFallbackPosters("seedance-2.5")).toEqual([]);
    expect(getVideoPromptTemplateFallbackPosters("minimax-h3")).toEqual([
      "https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-01/minimax-h3-seedance-2-0.jpg",
      "https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-02/minimax-h3-seedance-advertising-ecommerce.jpg",
      "https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-03/minimax-h3-seedance-film-concept-production.jpg",
      "https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-04/minimax-h3-seedance-game-art-animation.jpg",
      "https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-05/minimax-h3-seedance-creator-explainer.jpg",
      "https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-06/minimax-h3-seedance-music-visual-art.jpg",
    ]);
  });

  test("returns one model-specific template set and keeps unknown models empty", () => {
    const seedanceTemplates = getVideoPromptTemplates("seedance-2.5");
    const minimaxTemplates = getVideoPromptTemplates("MiniMax-H3");
    expect(seedanceTemplates).toHaveLength(6);
    expect(seedanceTemplates[0].poster).toBe("");
    expect(seedanceTemplates.every((template) => template.video.endsWith(".mp4"))).toBe(true);
    expect(minimaxTemplates).toHaveLength(6);
    expect(minimaxTemplates[0].poster).toContain("minimax-h3-seedance-2-0.jpg");
    expect(minimaxTemplates.every((template) => template.video.includes("minimax-h3-seedance"))).toBe(true);
    expect(getVideoPromptTemplates("unknown-video-model")).toEqual([]);
  });

  test("keeps prompt bodies in English on every locale page and syncs the starter", () => {
    for (const modelId of VIDEO_PROFESSION_MODEL_IDS) {
      const englishCards = getVideoPromptTemplates(modelId, "en");
      const englishStarter = getVideoPlaygroundPrompt(modelId, "en", modelId);
      expect(englishStarter).toBe(englishCards[0].prompt);
      for (const locale of LOCALES) {
        const cards = getVideoPromptTemplates(modelId, locale);
        const starter = getVideoPlaygroundPrompt(modelId, locale, modelId);
        expect(cards).toHaveLength(VIDEO_PROMPT_TEMPLATES.length);
        expect(cards.every((card) => card.label.length > 0 && card.prompt.length > 100)).toBe(true);
        expect(cards.map((card) => card.professionId)).toEqual([...VIDEO_PROFESSION_IDS]);
        expect(starter.length).toBeGreaterThan(20);
        expect(cards.map((card) => card.prompt)).toEqual(englishCards.map((card) => card.prompt));
        expect(starter).toBe(englishStarter);
      }
    }
  });

  test("keeps a generic configured video starter in English", () => {
    const source = "Create a short product video with catalog-model: clear subject motion, realistic lighting, stable camera, and production-ready framing.";
    expect(localizeVideoPromptText(source, "zh")).toBe("Create a short video with catalog-model: clear subject motion, realistic lighting, stable camera, and production-ready framing.");
    expect(localizeVideoPromptText(source, "en")).toBe("Create a short video with catalog-model: clear subject motion, realistic lighting, stable camera, and production-ready framing.");
  });
});
