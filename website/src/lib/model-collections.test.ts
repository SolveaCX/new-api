import { describe, expect, test } from "bun:test";
import { LOCALES } from "@/lib/locales";
import { MODEL_COLLECTION_COPY } from "@/lib/model-collections-copy";
import type { PricingModel } from "@/lib/pricing";
import {
  getModelCollectionCopy, getModelCollectionPathnames, getAvailableModelCollections,
  getModelCollection, getLegacyCollectionDestination, getModelCollectionsSeoCopy,
  getModelCollectionSeoDescription, selectCollectionModels, modelCardData, MODEL_COLLECTIONS,
} from "@/lib/model-collections";

const model = (name: string, extra: Partial<PricingModel> = {}): PricingModel => ({ model_name: name, quota_type: 0, model_ratio: 1, completion_ratio: 1, ...extra });
const membership = (item: PricingModel) => MODEL_COLLECTIONS.filter((c) => c.matches(item)).map((c) => c.slug);
const meta = { author: "Test", providers: [], context_tokens: null, series: "Test", released_at: "2026-01-01", distillable: false };

describe("model collections", () => {
  test("keeps the eight agreed pages in order, including during empty or small catalogs", () => {
    const expected = ["discounted-models", "coding", "roleplay-creative-writing", "image-generation", "video-generation", "audio-generation-models", "vision-models", "text-to-speech-models"];
    expect(MODEL_COLLECTIONS.map((c) => c.slug)).toEqual(expected);
    expect(getAvailableModelCollections([]).map((c) => c.slug)).toEqual(expected);
    expect(getModelCollectionPathnames([])).toEqual(expected.map((slug) => `/collections/${slug}`));
    const tts = getModelCollection("text-to-speech-models")!;
    expect(selectCollectionModels(tts, [model("gemini-2.5-flash-tts")])).toHaveLength(1);
    expect(getAvailableModelCollections([model("gemini-2.5-flash-tts")])).toContain(tts);
  });

  test("has separate translated metadata, headings and card copy in every locale", () => {
    for (const locale of LOCALES) {
      const overview = MODEL_COLLECTION_COPY[locale].index;
      expect(getModelCollectionsSeoCopy(locale)).toEqual({ title: overview.seoTitle, description: overview.seoDescription });
      for (const collection of MODEL_COLLECTIONS) {
        const copy = getModelCollectionCopy(collection, locale);
        for (const field of ["title", "shortDescription", "slogan", "intro", "seoTitle", "seoDescription", "empty"] as const) {
          expect(copy[field].length).toBeGreaterThan(0);
          if (locale !== "en") expect(copy[field]).not.toBe(collection.copy.en[field]);
        }
        expect(copy.seoTitle).toEndWith("| Flatkey");
        expect(getModelCollectionSeoDescription(collection, locale)).toBe(copy.seoDescription);
      }
    }
    expect(getModelCollection("coding")!.copy.en.seoTitle).toBe("LLMs for Coding: Compare Models & API Prices | Flatkey");
    expect(getModelCollection("text-to-speech-models")!.copy.en.slogan).toBe("Give your words a voice");
  });

  test("keeps image and video understanding separate from generation", () => {
    const gemini = model("gemini-2.5-flash", { supported_endpoint_types: ["gemini", "openai"], directory_metadata: { ...meta, modalities: ["text", "image", "audio", "video"], categories: ["Programming", "Roleplay"] } });
    expect(membership(gemini)).toEqual(["coding", "roleplay-creative-writing", "vision-models"]);
    const video = model("MiniMax-H3", { supported_endpoint_types: ["video"], directory_metadata: { ...meta, modalities: ["text", "image", "audio", "video"], categories: ["Programming", "Roleplay"] } });
    expect(membership(video)).toEqual(["video-generation"]);
    expect(membership(model("veo-3.1-generate-preview", { supported_endpoint_types: ["openai-video", "gemini"] }))).toEqual(["video-generation"]);
    expect(membership(model("bytedance/seedance-2.0"))).toEqual(["video-generation"]);
    expect(membership(model("gpt-image-2", { directory_metadata: { ...meta, categories: [], modalities: ["text", "image"] } }))).toEqual(["image-generation"]);
    expect(membership(model("gemini-3.1-flash-image"))).toEqual(["image-generation"]);
  });

  test("audio covers TTS and video music, not transcription or audio input", () => {
    expect(membership(model("sonilo-video-to-music", { supported_endpoint_types: ["video-to-music"], directory_metadata: { ...meta, categories: [], modalities: ["text", "video"] } }))).toEqual(["audio-generation-models"]);
    expect(membership(model("gemini-2.5-flash-tts"))).toEqual(["audio-generation-models", "text-to-speech-models"]);
    expect(membership(model("gemini-2.5-pro-preview-tts"))).toEqual(["audio-generation-models", "text-to-speech-models"]);
    expect(membership(model("whisper-1", { description: "audio speech transcription" }))).toEqual([]);
    expect(membership(model("gemini-embedding-001"))).toEqual([]);
    expect(membership(model("unknown-model"))).toEqual([]);
  });

  test("keeps all matching models and supports explicit limits", () => {
    const coding = getModelCollection("coding")!;
    const models = Array.from({ length: 24 }, (_, i) => model(`coding-model-${i}`));
    expect(selectCollectionModels(coding, models)).toHaveLength(24);
    expect(selectCollectionModels(coding, models, 3)).toHaveLength(3);
  });

  test("only uses finite, comparable public/reference prices as discount evidence", () => {
    const discounted = getModelCollection("discounted-models")!;
    expect(discounted.matches(model("discount", { display_pricing: { billing_kind: "token", prices: { input: { configured: 5, plg: 3 } } } }))).toBe(true);
    for (const pair of [{ configured: 5, plg: 5 }, { configured: 2, plg: 3 }, { configured: 5 }, { plg: 3 }, { configured: 5, plg: -1 }, { configured: Infinity, plg: 3 }, { configured: 5, plg: NaN }]) {
      expect(discounted.matches(model("invalid", { display_pricing: { billing_kind: "token", prices: { input: pair } } }))).toBe(false);
    }
    expect(discounted.matches(model("cheap", { model_ratio: 0.1, group_ratio: { plg: 0.5 } }))).toBe(false);
  });

  test("preserves known legacy links without inventing a catch-all collection", () => {
    expect(getLegacyCollectionDestination("general-purpose-models")).toBe("/models");
    expect(getLegacyCollectionDestination("text-embedding-models")).toBe("/collections");
    expect(getLegacyCollectionDestination("not-a-category")).toBeNull();
    expect(getModelCollection("general-purpose-models")).toBeNull();
    expect(getLegacyCollectionDestination("coding")).toBeNull();
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
