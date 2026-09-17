import { describe, expect, test } from "bun:test";
import manifest from "./model-media-generation-manifest.json";
import { getShowcaseGeneratorLabel } from "./showcase-generation";

describe("showcase generator attribution", () => {
  test("credits Image 2 on regenerated artwork regardless of the destination model page", () => {
    const regenerated = manifest.entries.filter((entry) => "generationModel" in entry && entry.generationModel === "gpt-image-2");
    expect(regenerated.length).toBeGreaterThan(0);
    for (const entry of regenerated) {
      expect(getShowcaseGeneratorLabel(entry.poster)).toBe("Image 2");
    }
    expect(regenerated.some((entry) => entry.model !== "gpt-image-2")).toBe(true);
  });

  test("credits Seedance 2.0 on every regenerated video page, including other model pages", () => {
    const videos = manifest.entries.filter((entry) => entry.kind === "video" && "generationModel" in entry);
    expect(videos).toHaveLength(60);
    for (const entry of videos) {
      expect(getShowcaseGeneratorLabel(entry.poster)).toBe("Seedance 2.0");
      expect(getShowcaseGeneratorLabel(entry.video)).toBe("Seedance 2.0");
    }
    expect(videos.some((entry) => entry.model !== "seedance-2.0")).toBe(true);
  });

  test("does not assign a generator to unknown assets or infer one from a filename", () => {
    expect(getShowcaseGeneratorLabel()).toBeUndefined();
    expect(getShowcaseGeneratorLabel("/assets/unregistered-gpt-image-2.webp")).toBeUndefined();
  });
});
