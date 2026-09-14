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
      expect(asset).toStartWith("/assets/model-regeneration/");
      expect(sha256(readFileSync(new URL(`../../public${asset}`, import.meta.url)))).toBe(entry.assetSha256);
      expect(readFileSync(new URL(`../../public${entry.poster}`, import.meta.url)).length).toBeGreaterThan(0);
      if (entry.kind === "video") {
        const video = getVideoPromptTemplates(entry.model).find((item) => item.professionId === entry.scene)!;
        expect(video.video).toBe(asset);
        expect(getVideoPromptTemplateLocalFallbackPoster(entry.model, video.professionId)).toBe(entry.poster);
      } else if (entry.scene !== "playground") {
        expect(getImagePromptTemplateLocalFallbackPoster(entry.model, entry.poster)).toBe(entry.poster);
      }
    }
  });
});
