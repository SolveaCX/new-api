import { describe, expect, test } from "bun:test";
import {
  getImagePlaygroundExample,
  getImagePromptTemplate,
  getImagePromptTemplateFallbackPosters,
  getImagePromptTemplates,
  localizeImagePromptText,
  IMAGE_PROMPT_TEMPLATES,
} from "./image-prompt-templates";
import { LOCALES } from "./locales";

describe("image prompt templates", () => {
  test("covers the six reviewed image scenes with ready-to-run English prompts", () => {
    expect(IMAGE_PROMPT_TEMPLATES).toHaveLength(6);

    const ids = new Set(IMAGE_PROMPT_TEMPLATES.map((template) => template.id));
    expect(ids.size).toBe(IMAGE_PROMPT_TEMPLATES.length);
    const prompts = new Set(IMAGE_PROMPT_TEMPLATES.map((template) => template.prompt));
    expect(prompts.size).toBe(IMAGE_PROMPT_TEMPLATES.length);
    const posters = new Set(IMAGE_PROMPT_TEMPLATES.map((template) => template.poster));
    expect(posters.size).toBe(IMAGE_PROMPT_TEMPLATES.length);

    for (const template of IMAGE_PROMPT_TEMPLATES) {
      expect(template.prompt.length).toBeGreaterThan(120);
      expect(template.prompt).not.toMatch(/\[[^\]]+\]/);
      expect(template.poster).toMatch(/^(\/assets\/|\/use-case\/image-buddy\/|https:\/\/cdn\.shulex-voc\.com\/flatkey\/model-media\/)/);
      expect(template.poster).not.toMatch(/creator|portrait|ugc|medical|developer|terminal|fitness-app|streetwear/i);
      expect(template.ratio).toMatch(/^\d+:\d+$/);
      expect(template.tags.length).toBeGreaterThan(0);
    }

    expect(IMAGE_PROMPT_TEMPLATES[0].prompt).toMatch(/sci-fi game loadout|glowing violet rifle/i);
    expect(IMAGE_PROMPT_TEMPLATES[1].prompt).toMatch(/rainy night soccer|stadium floodlights/i);
    expect(IMAGE_PROMPT_TEMPLATES[2].prompt).toMatch(/matte-black square smartwatch|wet glossy black tabletop/i);
    expect(IMAGE_PROMPT_TEMPLATES[3].prompt).toMatch(/rain-soaked observatory|large telescope/i);
    expect(IMAGE_PROMPT_TEMPLATES[4].prompt).toMatch(/surprised cook|pancakes/i);
    expect(IMAGE_PROMPT_TEMPLATES[5].prompt).toMatch(/period harbor platform|vintage green tram/i);
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
    expect(first[0].prompt).not.toBe(second[0].prompt);

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

  test("keeps each model's six prompt posters bound to its six scene prompts", () => {
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

    for (const modelId of modelIds) {
      const templates = getImagePromptTemplates(modelId);
      const posters = getImagePromptTemplateFallbackPosters(modelId);
      expect(templates.map((template) => template.poster)).toEqual(posters);
      expect(templates.map((template) => template.prompt)).toHaveLength(IMAGE_PROMPT_TEMPLATES.length);
      expect(templates.every((template) => template.prompt.length > 120)).toBe(true);
      expect(templates.every((template) => !/\[[^\]]+\]/.test(template.prompt))).toBe(true);
    }
  });

  test("uses a fact-based English prompt set for every canonical image model", () => {
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
    const expectedSceneTokens = [
      /game|loadout|character|controller|camera/i,
      /soccer|basketball|swimmer|volleyball|hockey|race|sport/i,
      /product|watch|shoe|bottle|camera|perfume|espresso|controller|earbuds/i,
      /observatory|train|city|astronaut|coast|desert|stage|mountain|theater|traveler|cinematic|storyboard/i,
      /kitchen|laundry|beach|dog|theater|boxes|pancake|towel|physical|comedy|park|umbrella/i,
      /historical|period|vintage|archive|weaving|camel|harbor|market|school|train|boat|dock|riverside/i,
    ];

    const promptSets = modelIds.map((modelId) => getImagePromptTemplates(modelId).map((template) => template.prompt));
    expect(new Set(promptSets.map((prompts) => prompts.join("\n"))).size).toBe(modelIds.length);

    for (const prompts of promptSets) {
      expect(prompts).toHaveLength(6);
      prompts.forEach((prompt, index) => {
        expect(prompt).toMatch(expectedSceneTokens[index]);
        expect(prompt).not.toMatch(/use seedance|high-quality|production-ready framing/i);
        expect(prompt).not.toMatch(/\[[^\]]+\]/);
      });
    }
  });

  test("describes the visible content of the four previously mismatched image assets", () => {
    const flashCards = getImagePromptTemplates("gemini-3.1-flash-image");
    expect(flashCards[4].prompt).toMatch(/coral-red raincoat/i);
    expect(flashCards[4].prompt).toMatch(/inverted rainbow umbrella/i);
    expect(flashCards[4].prompt).toMatch(/wet green park/i);
    expect(flashCards[5].prompt).toMatch(/workers.*repairing.*wooden boat/i);
    expect(flashCards[5].prompt).toMatch(/riverside boatyard/i);

    const flashLiteCards = getImagePromptTemplates("gemini-3.1-flash-lite-image");
    expect(flashLiteCards[0].prompt).toMatch(/white robot/i);
    expect(flashLiteCards[0].prompt).toMatch(/floating-island garden/i);
    expect(flashLiteCards[0].prompt).toMatch(/plant blaster/i);
    expect(flashLiteCards[0].prompt).toMatch(/equipment wheel/i);

    const qualityCards = getImagePromptTemplates("grok-imagine-image-quality");
    expect(qualityCards[0].prompt).toMatch(/cloaked wizard/i);
    expect(qualityCards[0].prompt).toMatch(/crystal equipment wheel/i);
    expect(qualityCards[0].prompt).toMatch(/pink crystal/i);
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

    const sceneTokens = [
      /dark-haired profile silhouette|mountain temple city/i,
      /three-day Chengdu|Anshun Bridge/i,
      /Ming-dynasty hanfu|exploded garment/i,
      /luxury skincare serum|frosted dropper/i,
      /10-by-10 numbered storyboard|100 panels/i,
      /open-world game livestream|palm-lined coastal street/i,
      /golden retriever|pet-food brand/i,
      /fantasy-and-science-fiction book cover|vaulted library/i,
      /high-end interior render|boucle sofa/i,
    ];
    examples.forEach((example, index) => {
      expect(example?.prompt).toMatch(sceneTokens[index]);
      expect(example?.prompt).not.toMatch(/\[[^\]]+\]/);
    });
  });

  test("resolves image model aliases used by the pricing catalog", () => {
    expect(getImagePlaygroundExample("gemini-3.1-flash-image-preview")?.poster).toContain("high-end-skincare-product-poster");
    expect(getImagePlaygroundExample("gemini_2_5_flash_image_preview")?.poster).toContain("three-day-travel-guide-card");
    expect(getImagePromptTemplates("gemini-3.1-flash-image-preview").map((template) => template.poster)).toEqual(
      getImagePromptTemplates("gemini-3.1-flash-image").map((template) => template.poster),
    );
    expect(getImagePromptTemplates("gemini_2_5_flash_image_preview").map((template) => template.poster)).toEqual(
      getImagePromptTemplates("gemini-2.5-flash-image").map((template) => template.poster),
    );
    expect(getImagePlaygroundExample("unknown-image-model")).toBeUndefined();
  });

  test("localizes prompt bodies on every locale page", () => {
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

    for (const modelId of modelIds) {
      const englishCards = getImagePromptTemplates(modelId, "en");
      const englishStarter = getImagePlaygroundExample(modelId, "en");
      expect(englishStarter).toBeDefined();
      for (const locale of LOCALES) {
        const cards = getImagePromptTemplates(modelId, locale);
        const starter = getImagePlaygroundExample(modelId, locale);
        expect(cards).toHaveLength(IMAGE_PROMPT_TEMPLATES.length);
        expect(starter).toBeDefined();
        expect(cards.every((card) => card.label.length > 0 && card.prompt.length > 80)).toBe(true);
        expect(starter?.prompt.length).toBeGreaterThan(80);
        if (locale === "en") {
          expect(cards.map((card) => card.prompt)).toEqual(englishCards.map((card) => card.prompt));
          expect(starter?.prompt).toBe(englishStarter?.prompt);
        } else {
          expect(cards.map((card) => card.prompt)).not.toEqual(englishCards.map((card) => card.prompt));
          expect(starter?.prompt).not.toBe(englishStarter?.prompt);
        }
      }
    }
  });

  test("localizes the generic image starter when a catalog page has no curated example", () => {
    const source = "Create a high-quality product image with catalog-model: clean composition, precise lighting, strong subject focus, and realistic detail.";
    expect(localizeImagePromptText(source, "zh")).toBe("使用 catalog-model 制作高质量产品图片：构图简洁、光线精准、主体突出，并保留真实细节。");
    expect(localizeImagePromptText(source, "en")).toBe(source);
  });
});
