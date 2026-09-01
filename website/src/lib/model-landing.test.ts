import { describe, expect, test } from "bun:test";
import {
  CLAUDE_CONFIG,
  DEEPSEEK_CONFIG,
  GEMINI_CONFIG,
  GLM_API_CONFIG,
  GPT_CONFIG,
  GPT_4_1_MINI_CONFIG,
  GPT_IMAGE_2_CONFIG,
  MINIMAX_H3_CONFIG,
  QWEN_CONFIG,
  SEEDANCE_CONFIG,
  SEEDANCE_25_CONFIG,
  SONILO_VIDEO_TO_MUSIC_CONFIG,
  getModelLandingConfig,
  getModelLandingConfigForModel,
  getModelLandingConfigForPricingModel,
  buildModelLandingMetadata,
  getModelLandingPathnames,
  getPriorityModelLandingPathnames,
  getLocalizedModelLandingConfig,
  getLocalizedModelLandingSeo,
  resolveModelLandingModels,
  modelLandingCopy,
} from "./model-landing";
import { LOCALES } from "./locales";
import type { PricingModel } from "./pricing";

describe("model landing configuration", () => {
  test("builds live metadata for every catalog task type", () => {
    const cases: Array<[string, string[], string[]]> = [
      ["gpt-test", ["openai"], ["chat/completions"]],
      ["gpt-image-test", ["image-generation"], ["image generation"]],
      ["video-test", ["video"], ["video generation"]],
      ["tts-test", ["audio"], ["audio"]],
      ["gemini-embedding-001", ["embeddings"], ["embedding"]],
      ["rerank-test", ["jina-rerank"], ["reranking"]],
    ];
    for (const [modelName, endpoints, taskNeedles] of cases) {
      const metadata = buildModelLandingMetadata({
        model_name: modelName,
        vendor_name: "Live Provider",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 2,
        supported_endpoint_types: endpoints,
        display_pricing: { billing_kind: "token", prices: { input: { plg: 1.25 }, output: { plg: 2.5 } } },
        directory_metadata: {
          author: "Live Provider",
          providers: ["Live Provider"],
          modalities: ["text"],
          context_tokens: 128000,
          series: "test",
          categories: ["test"],
          released_at: "2026-01-01",
          distillable: false,
        },
      });
      expect(taskNeedles.some((needle) => metadata.title.toLowerCase().includes(needle))).toBe(true);
      expect(metadata.description).toContain("Live Provider");
      expect(metadata.description).toContain("$1.25");
      expect(metadata.description).toContain("128,000-token");
      expect(metadata.description.length).toBeLessThanOrEqual(160);
    }
  });

  test("uses Portuguese metadata wording and clamps long descriptions", () => {
    const metadata = buildModelLandingMetadata({
      model_name: "very-long-model-name-for-description-length-check",
      vendor_name: "Provedor ao Vivo",
      quota_type: 1,
      model_ratio: 0,
      completion_ratio: 0,
      model_price: 0.08,
      supported_endpoint_types: ["video"],
      directory_metadata: {
        author: "Provedor ao Vivo",
        providers: ["Provedor ao Vivo"],
        modalities: ["video"],
        context_tokens: null,
        series: "test",
        categories: ["test"],
        released_at: "2026-01-01",
        distillable: false,
      },
    }, { locale: "pt" });
    expect(metadata.description).toContain("API de geração de vídeo");
    expect(metadata.description).toContain("Provedor ao Vivo");
    expect(metadata.description.length).toBeLessThanOrEqual(160);
  });

  test("prefers the resolved landing kind for ambiguous provider endpoints", () => {
    const metadata = buildModelLandingMetadata({
      model_name: "gemini-3.1-flash-image",
      vendor_name: "Google",
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      supported_endpoint_types: ["gemini"],
      directory_metadata: { modalities: ["text", "image"] },
      display_pricing: { billing_kind: "token", prices: { input: { plg: 1 }, output: { plg: 2 } } },
    } as PricingModel, { task: "image generation" });
    expect(metadata.title).toContain("image generation API");
    expect(metadata.description).toContain("image generation");
  });

  test("localizes live metadata templates for every supported locale", () => {
    const model: PricingModel = {
      model_name: "locale-check-model",
      vendor_name: "Provider Global",
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 2,
      supported_endpoint_types: ["openai"],
      display_pricing: {
        billing_kind: "token",
        prices: { input: { plg: 1.25 }, output: { plg: 2.5 } },
      },
      directory_metadata: {
        author: "Provider Global",
        providers: ["Provider Global"],
        modalities: ["text"],
        context_tokens: 128000,
        series: "locale-test",
        categories: ["text"],
        released_at: "2026-01-01",
        distillable: false,
      },
    };

    const expectedTitleTerms: Record<string, string> = {
      en: "pricing",
      zh: "价格",
      es: "precios",
      fr: "tarifs",
      pt: "preços",
      ru: "цены",
      ja: "料金",
      vi: "giá",
      de: "Preise",
      id: "harga",
    };
    for (const locale of LOCALES) {
      const metadata = buildModelLandingMetadata(model, { locale });
      expect(metadata.title).toContain(expectedTitleTerms[locale]);
      expect(metadata.description).toContain("Provider Global");
      expect(metadata.description.length).toBeLessThanOrEqual(160);
      if (locale !== "en") expect(metadata.description).not.toContain("via Flatkey; Provider Global model");
    }
  });

  test("defines paid-search landing pages for DeepSeek, Qwen, and GLM APIs", () => {
    expect(getModelLandingConfig("deepseek-api")).toBe(DEEPSEEK_CONFIG);
    expect(getModelLandingConfig("qwen-api")).toBe(QWEN_CONFIG);
    expect(getModelLandingConfig("glm-api")).toBe(GLM_API_CONFIG);

    for (const config of [DEEPSEEK_CONFIG, QWEN_CONFIG, GLM_API_CONFIG]) {
      expect(config.modelIds.length).toBeGreaterThanOrEqual(3);
      expect(config.seo.title.toLowerCase()).toContain("api");
    }
  });

  test("defines a paid-search landing page for the Gemini API", () => {
    expect(getModelLandingConfig("gemini-api")).toBe(GEMINI_CONFIG);
    expect(GEMINI_CONFIG.modelIds.length).toBeGreaterThanOrEqual(3);
    expect(GEMINI_CONFIG.seo.title.toLowerCase()).toContain("gemini api");
    expect(GEMINI_CONFIG.seo.title.toLowerCase()).toContain("openai-compatible");
    expect(getModelLandingConfigForModel("gemini-2.5-pro")?.slug).toBe("gemini-api");
    expect(getModelLandingConfigForModel("gemini-2.5-flash-preview")?.slug).toBe("gemini-api");
  });

  test("resolves configured landing pages by slug", () => {
    expect(getModelLandingConfig("gpt-api")?.displayName).toBe("GPT-5");
    expect(getModelLandingConfig("gpt-image-2")).toBe(GPT_IMAGE_2_CONFIG);
    expect(getModelLandingConfig("gpt-4.1-mini")?.modelId).toBe("gpt-4.1-mini");
    expect(getModelLandingConfig("minimax-h3")).toBe(MINIMAX_H3_CONFIG);
    expect(getModelLandingConfig("sonilo-video-to-music")).toBe(SONILO_VIDEO_TO_MUSIC_CONFIG);
    expect(getModelLandingConfigForModel("gpt-image-2")?.generator?.kind).toBe("image");
    expect(getModelLandingConfigForModel("MiniMax-H3")?.generator?.kind).toBe("video");
    expect(getModelLandingConfigForModel("sonilo-video-to-music")?.generator?.kind).toBe("audio");
    expect(getModelLandingConfig("missing-model")).toBeNull();
  });

  test("exposes sitemap pathnames for configured model landing pages", () => {
    expect(getModelLandingPathnames()).toEqual([
      "/models/claude-api",
      "/models/deepseek-api",
      "/models/gemini-api",
      "/models/gpt-4.1-mini",
      "/models/gpt-image-2",
      "/models/glm-api",
      "/models/gpt-api",
      "/models/kimi-k3",
      "/models/minimax-h3",
      "/models/qwen-api",
      "/models/seedance-2.5",
      "/models/seedance-api",
      "/models/sonilo-video-to-music",
    ]);
  });

  test("exposes dynamic priority model paths for sitemap discovery", () => {
    expect(getPriorityModelLandingPathnames()).toEqual([
      "/models/gpt-5.6-sol",
      "/models/gpt-image-2",
      "/models/kimi-k3",
      "/models/deepseek-v4-pro",
      "/models/minimax-h3",
    ]);
  });

  test("matches live pricing models from configured model ids", () => {
    const liveModels: PricingModel[] = [
      {
        model_name: "gpt-5-2026-06-01",
        vendor_name: "OpenAI",
        quota_type: 0,
        model_ratio: 0.35,
        completion_ratio: 8,
      },
      {
        model_name: "seedance-2.0-pro",
        vendor_name: "OpenAI",
        quota_type: 0,
        model_ratio: 0.35,
        completion_ratio: 8,
      },
      {
        model_name: "claude-opus-4",
        vendor_name: "Anthropic",
        quota_type: 0,
        model_ratio: 3.75,
        completion_ratio: 5,
      },
    ];

    const config = getModelLandingConfig("gpt-api");

    expect(config?.modelIds).toContain("gpt-5");
    expect(resolveModelLandingModels(config!, liveModels).map((model) => model.model_name)).toEqual(["gpt-5-2026-06-01"]);
  });

  test("finds landing page config from a live pricing model name", () => {
    expect(getModelLandingConfigForModel("gpt-5-mini")?.slug).toBe("gpt-api");
    expect(getModelLandingConfigForModel("gpt-5.5-fk-cx")?.slug).toBe("gpt-api");
    expect(getModelLandingConfigForModel("gpt-5-2026-06-01")?.slug).toBe("gpt-api");
    expect(getModelLandingConfigForModel("MiniMax-H3")?.slug).toBe("minimax-h3");
    expect(getModelLandingConfigForModel("seedance-2.0-pro")?.slug).toBe("seedance-api");
    expect(getModelLandingConfigForModel("seedance-2.5")?.slug).toBe("seedance-2.5");
    expect(getModelLandingConfigForModel("seedance-2-5")?.slug).toBe("seedance-2.5");
    expect(getModelLandingConfigForModel("unknown-model")).toBeNull();
  });

  test("keeps Seedance 2.5 generation defaults separate from Seedance 2.0", () => {
    expect(getModelLandingConfig("seedance-2.5")).toBe(SEEDANCE_25_CONFIG);
    expect(SEEDANCE_25_CONFIG.modelId).toBe("seedance-2.5");
    expect(SEEDANCE_25_CONFIG.generator).toMatchObject({
      endpoint: "/v1/videos",
      storageKey: "flatkey:model-generator-draft:seedance-2-5",
    });
    expect(SEEDANCE_25_CONFIG.generator?.fields).toEqual([
      { name: "resolution", label: "Resolution", type: "select", defaultValue: "720p", options: ["480p", "720p"] },
      { name: "ratio", label: "Aspect ratio", type: "select", defaultValue: "adaptive", options: ["adaptive", "16:9", "4:3", "1:1", "3:4", "9:16"] },
      { name: "duration", label: "Duration", type: "number", defaultValue: 5, min: 4, max: 30 },
      { name: "generate_audio", label: "Generate audio", type: "boolean", defaultValue: true },
    ]);
    expect(getModelLandingConfigForModel("seedance-2.0-pro")).toBeDefined();
    expect(getModelLandingConfigForModel("seedance-2.0-pro")?.slug).toBe("seedance-api");
  });

  test("keeps MiniMax-H3 on its documented video contract", () => {
    expect(MINIMAX_H3_CONFIG.generator?.protocol).toBe("minimax-video");
    expect(MINIMAX_H3_CONFIG.generator?.fields).toEqual([
      { name: "resolution", label: "Resolution", type: "select", defaultValue: "768P", options: ["768P", "2K"] },
      { name: "duration", label: "Duration", type: "number", defaultValue: 6, min: 4, max: 15 },
      { name: "ratio", label: "Aspect ratio", type: "select", defaultValue: "16:9", options: ["21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "adaptive"] },
      { name: "aigc_watermark", label: "AIGC watermark", type: "boolean", defaultValue: false },
    ]);
  });

  test("builds media landing configs from live pricing endpoint types", () => {
    const sonilo: PricingModel = {
      model_name: "sonilo-video-to-music",
      vendor_name: "Sonilo",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.009,
      completion_ratio: 0,
      supported_endpoint_types: ["video-to-music"],
    };

    const config = getModelLandingConfigForPricingModel(sonilo);

    expect(config?.slug).toBe("sonilo-video-to-music");
    expect(config?.generator?.kind).toBe("audio");
    expect(config?.generator?.endpoint).toBe("/v1/video-to-music");
    expect(config?.generator?.storageKey).toBe("flatkey:model-generator-draft:sonilo-video-to-music");
    expect(config?.seo.title).toContain("audio API");
    expect(config?.seo.description).toContain("audio");
  });

  test("does not let a broad Gemini chat family hide a Gemini image generator", () => {
    const config = getModelLandingConfigForPricingModel({
      model_name: "gemini-2.5-flash-image",
      vendor_name: "Google",
      quota_type: 0,
      model_ratio: 0.3,
      completion_ratio: 6,
      supported_endpoint_types: ["gemini"],
      directory_metadata: { modalities: ["text", "image"] },
    });

    expect(config.generator?.kind).toBe("image");
    expect(config.generator?.endpoint).toBe("/v1beta/models/gemini-2.5-flash-image:generateContent");
    expect(config.generator?.protocol).toBe("gemini-image");
  });

  test("keeps dynamic media Playground fields aligned with each model contract", () => {
    const grok = getModelLandingConfigForPricingModel({
      model_name: "grok-imagine-video",
      vendor_name: "xAI",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.07,
      completion_ratio: 0,
      supported_endpoint_types: ["video"],
    });
    expect(grok?.generator).toMatchObject({
      endpoint: "/v1/videos",
      protocol: "grok-video",
      referenceLimits: { image: 1 },
    });
    expect(grok?.generator?.fields.map((field) => field.name)).toEqual(["duration"]);

    const veo = getModelLandingConfigForPricingModel({
      model_name: "veo-3.1-generate-preview",
      vendor_name: "Google",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.4,
      completion_ratio: 0,
      supported_endpoint_types: ["video"],
    });
    expect(veo?.generator).toMatchObject({ protocol: "veo-video", referenceLimits: { image: 1 } });
    expect(veo?.generator?.fields.map((field) => field.name)).toEqual(["size", "duration"]);
  });

  test("does not put Playground copy into generic audio landing content", () => {
    const config = getModelLandingConfigForPricingModel({
      model_name: "eleven_sound_v1",
      vendor_name: "ElevenLabs",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.01,
      completion_ratio: 0,
      supported_endpoint_types: ["audio"],
    });

    expect(config.generator?.kind).toBe("audio");
    const editorialText = JSON.stringify(config.landingContent);
    expect(editorialText).not.toMatch(/playground/i);
    expect(editorialText).not.toContain("Start in the playground");
    expect(editorialText).not.toContain("public playground");
  });

  test("builds text landing configs for generic live pricing models", () => {
    const kimi: PricingModel = {
      model_name: "kimi-k2.5",
      vendor_name: "Moonshot AI",
      quota_type: 0,
      model_ratio: 0.3,
      completion_ratio: 4,
      supported_endpoint_types: ["openai"],
    };

    const config = getModelLandingConfigForPricingModel(kimi);

    expect(config.slug).toBe("kimi-k2.5");
    expect(config.modelId).toBe("kimi-k2.5");
    expect(config.officialName).toBe("Moonshot AI");
    expect(config.generator).toBeUndefined();
    expect(config.seo.title).toContain("kimi-k2.5");
    expect(config.landingContent?.pricing?.title).toContain("kimi-k2.5");
    expect(config.landingContent?.capabilitiesTitle).toContain("kimi-k2.5");
    expect(config.landingContent?.comparison?.title).toContain("kimi-k2.5");
    expect(config.landingContent?.api?.title).toContain("kimi-k2.5");
    expect(config.landingContent?.faq?.length).toBe(4);
  });

  test("keeps the five priority pages on target-specific editorial configs", () => {
    const priorityModels: Array<{
      id: string;
      endpoints: string[];
      heroNeedle: string;
      pricingNeedles: string[];
    }> = [
      { id: "gpt-5.6-sol", endpoints: ["/v1/chat/completions"], heroNeedle: "GPT-5.6 Sol", pricingNeedles: ["$4.00", "$24.00", "$0.40", "$5.00"] },
      { id: "gpt-image-2", endpoints: ["/v1/images/generations"], heroNeedle: "GPT Image 2", pricingNeedles: ["$4.00", "$24.00", "$1.00", "$6.40"] },
      { id: "kimi-k3", endpoints: ["/v1/chat/completions", "/v1/messages"], heroNeedle: "Kimi K3", pricingNeedles: ["$2.40", "$12.00", "$0.24"] },
      { id: "deepseek-v4-pro", endpoints: ["/v1/chat/completions", "/v1/messages"], heroNeedle: "DeepSeek V4 Pro", pricingNeedles: ["$1.32", "$0.044", "$3.96", "$0.66", "$0.022", "$1.98"] },
      { id: "MiniMax-H3", endpoints: ["/v1/videos"], heroNeedle: "MiniMax-H3", pricingNeedles: ["$0.08"] },
    ];

    const configs = priorityModels.map(({ id }) =>
      getModelLandingConfigForPricingModel({
        model_name: id,
        vendor_name: "test vendor",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
      }),
    );

    expect(new Set(configs.map((config) => config.landingContent?.hero?.title)).size).toBe(priorityModels.length);
    expect(new Set(configs.map((config) => config.landingContent?.faq?.[0]?.question)).size).toBe(priorityModels.length);

    for (const [index, config] of configs.entries()) {
      const target = priorityModels[index];
      const content = config.landingContent;

      expect(content).toBeDefined();
      const normalizeHeading = (value: string) => value.trim().toLowerCase().replace(/[._\s]+/g, "-");
      expect(normalizeHeading(content?.hero?.title ?? "")).toContain(normalizeHeading(target.heroNeedle));
      expect(normalizeHeading(content?.hero?.description ?? "")).toContain(normalizeHeading(target.heroNeedle));
      // Prose may use a natural spacing variant (for example, "MiniMax H3")
      // while the model ID uses a hyphen ("MiniMax-H3").
      expect(normalizeHeading(content?.pricing?.title ?? "")).toContain(normalizeHeading(target.heroNeedle));
      expect(content?.pricing?.rows?.length ?? 0).toBeGreaterThan(0);
      const editorialText = JSON.stringify(content);
      for (const endpoint of target.endpoints) expect(editorialText).toContain(endpoint);
      for (const price of target.pricingNeedles) expect(editorialText).toContain(price);
      expect(content?.faq?.length ?? 0).toBeGreaterThanOrEqual(6);

      // A priority page must not silently fall back to the generic two-question
      // generator FAQ or the Seedance-specific editorial block. Even media
      // pages need model-specific wording in every FAQ entry.
      expect(content?.faq?.some((item) => item.question === "Does this start a real generation?")).toBe(false);
      expect(content).not.toBe(SEEDANCE_25_CONFIG.landingContent);
      expect(config.slug).not.toBe(SEEDANCE_25_CONFIG.slug);
      expect(content?.hero?.title).not.toContain("Seedance");
    }
  });

  test("adds keyword-led editorial sections to explicit catalog families without curated copy", () => {
    const config = getModelLandingConfigForPricingModel({
      model_name: "gpt-5.5",
      vendor_name: "OpenAI",
      quota_type: 0,
      model_ratio: 0.5,
      completion_ratio: 8,
      supported_endpoint_types: ["openai"],
      directory_metadata: {
        author: "OpenAI",
        providers: ["OpenAI"],
        modalities: ["text", "image"],
        context_tokens: 256000,
        series: "GPT-5",
        categories: ["reasoning"],
        released_at: "2026-01-01",
        distillable: false,
      },
    });
    expect(config.landingContent?.hero?.title).toContain("gpt-5.5");
    expect(config.landingContent?.pricing?.title).toContain("gpt-5.5");
    expect(config.landingContent?.faq?.length).toBe(4);
  });

  test("keeps priority metadata localized and distinct from the English fallback", () => {
    const priorityIds = ["gpt-5.6-sol", "gpt-image-2", "kimi-k3", "deepseek-v4-pro", "MiniMax-H3"];

    for (const id of priorityIds) {
      const config = getModelLandingConfigForPricingModel({
        model_name: id,
        vendor_name: "test vendor",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
      });

      for (const locale of LOCALES) {
        const localized = locale === "en" ? config.seoByLocale?.en ?? config.seo : config.seoByLocale?.[locale];
        expect(localized?.title).toBeTruthy();
        expect(localized?.description).toBeTruthy();
        if (locale !== "en") {
          expect(localized?.title).not.toBe(config.seo.title);
          expect(localized?.description).not.toBe(config.seo.description);
        }
      }
    }
  });

  test("uses a coherent localized editorial pack without mutating the English config", () => {
    const cases: Array<{ id: string; needle: string }> = [
      { id: "gpt-5.6-sol", needle: "/v1/chat/completions" },
      { id: "gpt-image-2", needle: "/v1/images/generations" },
      { id: "kimi-k3", needle: "/v1/messages" },
      { id: "deepseek-v4-pro", needle: "UTC" },
      { id: "MiniMax-H3", needle: "ComfyUI" },
    ];

    for (const { id, needle } of cases) {
      const source = getModelLandingConfigForPricingModel({
        model_name: id,
        vendor_name: "test vendor",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
      });
      const sourceTitle = source.landingContent?.hero?.title;
      const localized = getLocalizedModelLandingConfig(source, "pt");
      const localizedText = JSON.stringify(localized.landingContent);

      expect(localized).not.toBe(source);
      expect(localized.landingContent?.hero?.title).toBeTruthy();
      expect(localized.landingContent?.hero?.title).not.toBe(sourceTitle);
      expect(localizedText).toContain(needle);
      expect(source.landingContent?.hero?.title).toBe(sourceTitle);
    }

    // Seedance keeps its own audited editorial block, but the resolver still
    // returns a localized copy so consumers outside React cannot accidentally
    // expose the English source fields.
    const localizedSeedance = getLocalizedModelLandingConfig(SEEDANCE_25_CONFIG, "pt");
    expect(localizedSeedance).not.toBe(SEEDANCE_25_CONFIG);
    expect(localizedSeedance.landingContent?.comparison?.title).toContain("Seedance 2.5");
    expect(localizedSeedance.landingContent?.comparison?.title).not.toBe(
      SEEDANCE_25_CONFIG.landingContent?.comparison?.title,
    );
    expect(SEEDANCE_25_CONFIG.landingContent?.comparison?.title).toContain("documented video fields");
  });

  test("keeps refreshed model-detail UI labels translated in every locale", () => {
    const localizedKeys = [
      "Product Reveal",
      "UGC Ad",
      "Cinematic Scene",
      "Social Clip",
      "Product Photo",
      "Copy Prompt",
      "Make one like this",
      "Long-context work",
      "Useful for document summarization, codebase analysis, and knowledge workflows.",
      "Coding and technical generation",
      "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.",
    ] as const;

    for (const locale of LOCALES.filter((item) => item !== "en")) {
      for (const key of localizedKeys) {
        expect(modelLandingCopy(locale, key as never)).not.toBe(modelLandingCopy("en", key as never));
      }
    }
  });

  test("localizes static model metadata when a catalog model is unavailable", () => {
    const staticConfigs = [
      CLAUDE_CONFIG,
      GPT_CONFIG,
      GEMINI_CONFIG,
      DEEPSEEK_CONFIG,
      QWEN_CONFIG,
      GLM_API_CONFIG,
      GPT_4_1_MINI_CONFIG,
      SEEDANCE_CONFIG,
      SONILO_VIDEO_TO_MUSIC_CONFIG,
    ];

    for (const config of staticConfigs) {
      for (const locale of LOCALES.filter((item) => item !== "en")) {
        const seo = getLocalizedModelLandingSeo(config, locale);
        expect(seo.title).toBeTruthy();
        expect(seo.description).toBeTruthy();
        expect(seo.title).not.toBe(config.seo.title);
        expect(seo.description).not.toBe(config.seo.description);
        expect(getLocalizedModelLandingConfig(config, locale).seo).toEqual(seo);
      }
    }
  });

  test("does not leak English compact labels into translated model shells", () => {
    const labels = ["Prompt", "Performance", "Endpoint"] as const;
    for (const locale of LOCALES.filter((item) => item !== "en")) {
      for (const label of labels) {
        expect(modelLandingCopy(locale, label as never)).not.toBe(label);
      }
    }
  });

  test("translates shared model-detail labels and related cards", () => {
    const labels = [
      "Usage",
      "Editing",
      "High-Speed Action",
      "10% below list price",
      "Seedance-2.5 API–frequently asked questions",
      "OpenAI-compatible text model",
      "Creative video generation",
      "High-fidelity video generation",
      "Motion control and references",
      "Story and scene generation",
      "Fast creative variants",
      "Social-ready clips",
      "Videos generated",
      "Images generated",
      "Input",
      "Output",
    ] as const;
    for (const locale of LOCALES.filter((item) => item !== "en")) {
      for (const label of labels) {
        const value = modelLandingCopy(locale, label as never);
        expect(value).toBeTruthy();
        expect(value).not.toBe(modelLandingCopy("en", label as never));
      }
    }
  });

  test("keeps Seedance 2.5 audited copy localized across every supported locale", () => {
    const auditedKeys = [
      "Seedance 2.5 AI Video Generator & API",
      "Seedance 2.5 pricing: 480p, 720p, and video references",
      "What is Seedance 2.5? Features for AI video generation",
      "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.",
    ];

    for (const locale of LOCALES) {
      for (const key of auditedKeys) {
        const value = modelLandingCopy(locale, key as never);
        expect(value).toBeTruthy();
        if (locale !== "en") expect(value).not.toBe(modelLandingCopy("en", key as never));
      }
    }
  });
});
