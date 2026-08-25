import { describe, expect, test } from "bun:test";
import { getImageModelLibrarySamples, getImageModelWorkbenchSample } from "./model-media";

// These are the image rows currently staged in model-media.ts. The live
// catalog can expose a smaller subset; keeping the staged rows here prevents a
// newly wired model from silently losing its six-card contract.
const IMAGE_MODELS = [
  "gpt-image-2",
  "gemini-3-pro-image",
  "gemini-2.5-flash-image",
  "gemini-3.1-flash-image",
  "gemini-3.1-flash-lite-image",
  "grok-imagine-image",
  "grok-imagine-image-pro",
  "grok-imagine-image-quality",
  "nano-banana-pro-preview",
];

describe("model-specific image media", () => {
  test("provides six independent library samples for every staged image model", () => {
    const sampleSets = IMAGE_MODELS.map((modelId) => getImageModelLibrarySamples(modelId));

    for (const samples of sampleSets) {
      expect(samples).toHaveLength(6);
      expect(new Set(samples.map((sample) => sample.slug)).size).toBe(6);
      expect(samples.every((sample) => sample.kind === "image")).toBe(true);
      expect(samples.every((sample) => sample.prompt.length > 40)).toBe(true);
    }

    const allSlugs = sampleSets.flatMap((samples) => samples.map((sample) => sample.slug));
    expect(new Set(allSlugs).size).toBe(allSlugs.length);
  });

  test("keeps the playground on the model's first workbench sample", () => {
    expect(getImageModelWorkbenchSample("gpt-image-2")?.slug).toBe("gpt-image-2-w1");
    expect(getImageModelWorkbenchSample("gemini-2.5-flash-image")?.slug).toBe("gemini-2-5-flash-image-w1");
    expect(getImageModelLibrarySamples("gemini-3.1-flash-image-preview")).toHaveLength(6);
    expect(getImageModelWorkbenchSample("unknown-image-model")).toBeNull();
  });
});
