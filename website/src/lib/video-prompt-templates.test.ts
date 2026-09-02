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

    expect(VIDEO_PROMPT_TEMPLATES[0].prompt).toMatch(/anime student|parchment map|library/i);
    expect(VIDEO_PROMPT_TEMPLATES[1].prompt).toMatch(/travel kettle|steam/i);
    expect(VIDEO_PROMPT_TEMPLATES[2].prompt).toMatch(/armored rover|treaded wheel|dust/i);
    expect(VIDEO_PROMPT_TEMPLATES[3].prompt).toMatch(/cloaked traveler|canyon ridge/i);
    expect(VIDEO_PROMPT_TEMPLATES[4].prompt).toMatch(/space-science|planet model/i);
    expect(VIDEO_PROMPT_TEMPLATES[5].prompt).toMatch(/stage-projection|performer silhouette/i);
  });

  test("describes the visible subject and action in every reviewed video clip", () => {
    const visibleContentSignatures: Record<string, RegExp[]> = {
      "seedance-2.5": [
        /student.*parchment map.*library/i,
        /kettle.*assembles.*steam/i,
        /armored.*rover.*treaded wheel.*dust/i,
        /cloaked traveler.*canyon.*sunset/i,
        /hand.*lavender planet.*moons/i,
        /silhouette.*teal.*amber.*light ribbons/i,
      ],
      "seedance-2.0": [
        /yellow raincoat.*railway platform.*letter/i,
        /coral.*bottle.*ice cubes/i,
        /observatory.*telescope.*dawn/i,
        /orange-suited.*hangar.*machine/i,
        /researcher.*old map.*brass/i,
        /musician.*keyboard.*blue.*purple/i,
      ],
      "seedance-2.0-pro": [
        /courier.*floating.*envelope.*rain/i,
        /rooftop.*parcel.*golden.*trail/i,
        /caped.*rooftop.*glowing.*lantern/i,
        /inventor.*orange cube.*robot/i,
        /desert explorer.*bronze.*disk.*blue/i,
        /rooftop.*glowing.*baton.*light trail/i,
      ],
      "seedance-2.0-fast": [
        /convenience store.*paper bag.*customer/i,
        /transparent.*lunchbox.*fruit.*lid/i,
        /stunt performer.*vaults.*warehouse/i,
        /runner.*neon corridor.*energy barrier/i,
        /glasses.*cylindrical.*device/i,
        /musician.*keyboard.*drum pad.*sampler/i,
      ],
      "seedance-2.0-mini": [
        /girl.*origami bird.*flies/i,
        /mint.*organizer.*pens.*paperclips/i,
        /miniature construction worker.*wall panels.*roof/i,
        /low-poly.*creature.*wing.*floats/i,
        /paper planet.*orange.*sun.*orbit/i,
        /circular base.*spiral maze/i,
      ],
      "minimax-h3": [
        /yellow raincoat.*station.*torn letter/i,
        /teal.*bottle.*canvas tote/i,
        /observatory.*telescope.*dawn/i,
        /fighter.*neon.*energy shield/i,
        /island.*lighthouse.*assembl/i,
        /chrome.*ring.*morph/i,
      ],
      "grok-imagine-video": [
        /mustard jacket.*stairwell.*envelope/i,
        /white travel mug.*blue lid.*steam/i,
        /industrial.*room.*ocean.*window/i,
        /man.*quadcopter.*circles/i,
        /presenter.*microphone.*audio cable/i,
        /gallery.*turquoise.*panel.*orange.*purple/i,
      ],
      "grok-imagine-video-1.5": [
        /woman.*library.*origami bird.*window/i,
        /woman.*lamp.*glowing panel.*notebook/i,
        /traveler.*bridge.*green map.*balloon/i,
        /ninja.*rooftop.*paper umbrella/i,
        /cards.*sprout.*tree/i,
        /musician.*keyboard.*red.*waveform/i,
      ],
      "veo-3.1-generate-preview": [
        /traveler.*platform.*package.*antique key/i,
        /amber serum.*water.*wraps/i,
        /oval module.*unfolds.*quadcopter/i,
        /blue droplet.*water creature.*ripple/i,
        /conservator.*library.*astronomical instrument/i,
        /performer.*particle.*rose.*halo/i,
      ],
      "veo-3.1-fast-generate-preview": [
        /lantern festival.*puppet.*audience/i,
        /running shoe.*assembles.*splash/i,
        /woman.*orange suit.*train platform/i,
        /hero.*stone golem.*cyan core/i,
        /boy.*blue ball.*wooden ramp/i,
        /performer.*cyan grid.*orange waveform/i,
      ],
    };

    for (const [modelId, signatures] of Object.entries(visibleContentSignatures)) {
      const cards = getVideoPromptTemplates(modelId, "en");
      expect(cards).toHaveLength(signatures.length);
      cards.forEach((card, index) => {
        const visibleTokens = signatures[index].source.split(".*");
        visibleTokens.forEach((token) => expect(card.prompt).toMatch(new RegExp(token, "i")));
      });
    }
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

  test("localizes prompt bodies and the starter on every locale page", () => {
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
        if (locale === "en") {
          expect(cards.map((card) => card.prompt)).toEqual(englishCards.map((card) => card.prompt));
          expect(starter).toBe(englishStarter);
        } else {
          expect(cards.map((card) => card.prompt)).not.toEqual(englishCards.map((card) => card.prompt));
          expect(starter).not.toBe(englishStarter);
        }
      }
    }
  });

  test("localizes a generic configured video starter", () => {
    const source = "Create a short product video with catalog-model: clear subject motion, realistic lighting, stable camera, and production-ready framing.";
    expect(localizeVideoPromptText(source, "zh")).toBe("使用 catalog-model 制作短视频：主体运动清晰、光线真实、镜头稳定，画面构图达到可制作标准。");
    expect(localizeVideoPromptText(source, "en")).toBe("Create a short video with catalog-model: clear subject motion, realistic lighting, stable camera, and production-ready framing.");
  });
});
