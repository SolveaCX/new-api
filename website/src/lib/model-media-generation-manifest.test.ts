import { describe, expect, test } from "bun:test";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import manifest from "./model-media-generation-manifest.json";
import { getImagePlaygroundExample, getImagePromptTemplates, getImagePromptTemplateLocalFallbackPoster } from "./image-prompt-templates";
import { getVideoPromptTemplates, getVideoPromptTemplateLocalFallbackPoster } from "./video-prompt-templates";

const sha256 = (value: string | Buffer) => createHash("sha256").update(value).digest("hex");

describe("regenerated model showcase provenance", () => {
  test("binds each generated asset to the exact displayed model and English prompt", () => {
    expect(manifest.entries.length).toBeGreaterThan(0);
    const identities = new Set<string>();
    for (const entry of manifest.entries) {
      const identity = `${entry.model}:${entry.scene}`;
      expect(identities.has(identity)).toBe(false);
      identities.add(identity);
      const example = entry.kind === "video"
        ? getVideoPromptTemplates(entry.model).find((item) => item.professionId === entry.scene)
        : entry.scene === "playground"
          ? getImagePlaygroundExample(entry.model)
          : getImagePromptTemplates(entry.model).find((item) => item.id === entry.scene);
      expect(example).toBeDefined();
      expect(sha256(example!.prompt)).toBe(entry.promptSha256);
      expect(example!.poster).toBe(entry.poster);
      const asset = "video" in entry ? entry.video : entry.poster;
      expect(entry.poster).toStartWith("/assets/model-regeneration/");
      const posterBytes = readFileSync(new URL(`../../public${entry.poster}`, import.meta.url));
      expect(posterBytes.length).toBeGreaterThan(0);
      if (entry.kind === "video") {
        const video = getVideoPromptTemplates(entry.model).find((item) => item.professionId === entry.scene)!;
        expect(asset).toMatch(/^https:\/\/cdn\.shulex-voc\.com\/flatkey\/model-showcase\/video-profession-\d\d\/20260917-[\w-]+-[a-f0-9]{12}\.mp4$/);
        expect(entry.generationModel).toBe("seedance-2.0");
        expect(entry.requestedDuration).toBe(10);
        expect(entry.duration).toBeGreaterThanOrEqual(10);
        expect(entry.posterSha256).toBe(sha256(posterBytes));
        expect(entry.assetSha256).toMatch(/^[a-f0-9]{64}$/);
        expect(video.video).toBe(asset);
        expect(video.duration).toBe(10);
        expect(getVideoPromptTemplateLocalFallbackPoster(entry.model, video.professionId)).toBe(entry.poster);
      } else {
        expect(asset).toStartWith("/assets/model-regeneration/");
        expect(sha256(posterBytes)).toBe(entry.assetSha256);
        if (entry.scene !== "playground") {
          expect(getImagePromptTemplateLocalFallbackPoster(entry.model, entry.poster)).toBe(entry.poster);
        }
      }
    }
  });
});
