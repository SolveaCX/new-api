import { describe, expect, test } from "bun:test";
import { LOCALES } from "@/lib/locales";
import {
  getModelCollectionCopy,
  getModelCollectionPathnames,
  getAvailableModelCollections,
  MIN_COLLECTION_MODELS,
  selectCollectionModels,
  modelCardData,
  MODEL_COLLECTIONS,
} from "@/lib/model-collections";

describe("model collections", () => {
  test("exposes stable collection paths", () => {
    expect(getModelCollectionPathnames()).toEqual([
      "/collections/image-generation",
      "/collections/coding",
      "/collections/video-generation",
      "/collections/tool-calling",
      "/collections/free-models",
      "/collections/discounted-models",
      "/collections/roleplay-creative-writing",
      "/collections/vision-models",
      "/collections/openclaw-models",
      "/collections/text-embedding-models",
      "/collections/audio-generation-models",
      "/collections/text-to-speech-models",
      "/collections/speech-to-text-models",
      "/collections/rerank-models",
    ]);
  });

  test("has localized title and description copy for every locale", () => {
    for (const collection of MODEL_COLLECTIONS) {
      for (const locale of LOCALES) {
        const copy = getModelCollectionCopy(collection, locale);
        expect(copy.title.length).toBeGreaterThan(0);
        expect(copy.shortDescription.length).toBeGreaterThan(0);
        expect(copy.intro.length).toBeGreaterThan(0);
      }
    }
  });

  test("only exposes collections backed by current catalog models", () => {
    const models = [
      ...Array.from({ length: MIN_COLLECTION_MODELS }, (_, index) => ({ model_name: `text-embedding-${index}`, quota_type: 0, model_ratio: 1, completion_ratio: 1 })),
      ...Array.from({ length: MIN_COLLECTION_MODELS }, (_, index) => ({ model_name: `cohere-rerank-v3-${index}`, quota_type: 0, model_ratio: 1, completion_ratio: 1 })),
    ];

    expect(getAvailableModelCollections(models).map((collection) => collection.slug)).toEqual([
      "text-embedding-models",
      "rerank-models",
    ]);
    expect(getModelCollectionPathnames(models)).toEqual([
      "/collections/text-embedding-models",
      "/collections/rerank-models",
    ]);
  });

  test("only exposes collections with at least five matched models", () => {
    const coding = MODEL_COLLECTIONS.find((collection) => collection.slug === "coding");
    const models = Array.from({ length: MIN_COLLECTION_MODELS - 1 }, (_, index) => ({
      model_name: `coding-model-${index}`,
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      directory_metadata: { categories: ["Programming"] },
    }));
    expect(coding ? selectCollectionModels(coding, models, MIN_COLLECTION_MODELS) : []).toHaveLength(MIN_COLLECTION_MODELS - 1);
    expect(getAvailableModelCollections(models)).not.toContain(coding);
  });

  test("returns every matched model by default instead of truncating collections", () => {
    const coding = MODEL_COLLECTIONS.find((collection) => collection.slug === "coding");
    expect(coding).toBeDefined();
    const models = Array.from({ length: 24 }, (_, index) => ({
      model_name: `coding-model-${index}`,
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      directory_metadata: { categories: ["Programming"], modalities: ["text"], author: "Test", providers: [], context_tokens: null, series: "Test", released_at: "2026-01-01", distillable: false },
    }));
    expect(coding ? selectCollectionModels(coding, models) : []).toHaveLength(24);
  });

  test("recognizes custom tool and audio model signals", () => {
    const tools = MODEL_COLLECTIONS.find((collection) => collection.slug === "tool-calling");
    const audio = MODEL_COLLECTIONS.find((collection) => collection.slug === "audio-generation-models");
    expect(tools?.matches({ model_name: "gemini-custom-tools", quota_type: 0, model_ratio: 1, completion_ratio: 1 })).toBe(true);
    expect(audio?.matches({ model_name: "sonilo-video-to-music", quota_type: 0, model_ratio: 1, completion_ratio: 1 })).toBe(true);
  });

  test("detects discounts from the public and configured prices", () => {
    const discounted = MODEL_COLLECTIONS.find((collection) => collection.slug === "discounted-models");
    expect(discounted).toBeDefined();
    expect(
      discounted?.matches({
        model_name: "gpt-discounted",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
        display_pricing: {
          billing_kind: "token",
          prices: { input: { configured: 5, plg: 3 }, output: { configured: 30, plg: 18 } },
        },
      }),
    ).toBe(true);
    expect(
      discounted?.matches({
        model_name: "gpt-full-price",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
        display_pricing: {
          billing_kind: "token",
          prices: { input: { configured: 5, plg: 5 } },
        },
      }),
    ).toBe(false);
  });

  test("only treats models with verified zero public prices as free", () => {
    const free = MODEL_COLLECTIONS.find((collection) => collection.slug === "free-models");
    expect(free).toBeDefined();
    expect(
      free?.matches({
        model_name: "verified-free",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
        display_pricing: {
          billing_kind: "token",
          prices: { input: { configured: 2, plg: 0 }, output: { configured: 8, plg: 0 } },
        },
      }),
    ).toBe(true);
    expect(
      free?.matches({
        model_name: "legacy-zero-but-paid",
        quota_type: 0,
        model_ratio: 0,
        completion_ratio: 1,
        display_pricing: {
          billing_kind: "token",
          prices: { input: { configured: 4, plg: 4 } },
        },
      }),
    ).toBe(false);
    expect(
      free?.matches({
        model_name: "partially-free",
        quota_type: 0,
        model_ratio: 1,
        completion_ratio: 1,
        display_pricing: {
          billing_kind: "token",
          prices: { input: { configured: 2, plg: 0 }, output: { configured: 8, plg: 8 } },
        },
      }),
    ).toBe(false);
  });

  test("resolves official vendor logo keys when model icon metadata is missing", () => {
    const pricing = {
      models: [],
      vendors: [],
      groupRatio: {},
      groupModelRatio: {},
      usableGroup: {},
      supportedEndpoint: {},
      autoGroups: [],
    };
    expect(
      modelCardData(
        {
          model_name: "bytedance/seedance-2.0-fast",
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
          vendor_name: "ByteDance",
        },
        pricing,
      ).iconKey,
    ).toBe("bytedance");
    expect(
      modelCardData(
        {
          model_name: "claude-fable-5",
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
          vendor_name: "Anthropic",
        },
        pricing,
    ).iconKey,
    ).toBe("claude-color");
    expect(
      modelCardData(
        {
          model_name: "nano-banana-pro-preview",
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
        },
        pricing,
      ).iconKey,
    ).toBe("gemini-color");
  });
});
