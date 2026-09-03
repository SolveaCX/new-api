import { describe, expect, test } from "bun:test";
import { LOCALES } from "@/lib/locales";
import {
  getModelCollectionCopy,
  getModelCollectionPathnames,
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
      "/collections/distillable-models",
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
});
