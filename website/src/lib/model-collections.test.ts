import { describe, expect, test } from "bun:test";
import { LOCALES } from "@/lib/locales";
import {
  getModelCollectionCopy,
  getModelCollectionPathnames,
  getAvailableModelCollections,
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
      { model_name: "text-embedding-3-small", quota_type: 0, model_ratio: 1, completion_ratio: 1 },
      { model_name: "cohere-rerank-v3", quota_type: 0, model_ratio: 1, completion_ratio: 1 },
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
  });
});
