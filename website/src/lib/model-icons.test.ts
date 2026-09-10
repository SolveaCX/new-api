import { describe, expect, test } from "bun:test";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { getLobeStaticSvgUrl, getLocalLogoUrl } from "./model-icons";

const repairedIcons = [
  ["DeepSeek.Color", "deepseek-color"],
  ["VeniceAI", "venice"],
  ["TogetherAI.Color", "together-color"],
  ["Moonshot.Color", "moonshot"],
  ["VertexAI.Color", "vertexai-color"],
  ["ByteDance", "bytedance-color"],
  ["Vercel.Color", "vercel"],
  ["OpenCode", "opencode"],
] as const;

describe("model icon assets", () => {
  test.each(repairedIcons)("maps %s to a verified pinned asset and a bundled fallback", (input, filename) => {
    expect(getLobeStaticSvgUrl(input)).toBe(
      `https://cdn.jsdelivr.net/npm/@lobehub/icons-static-svg@1.95.0/icons/${filename}.svg`
    );
    const fallback = getLocalLogoUrl(input);
    expect(fallback).not.toBeNull();
    expect(existsSync(join(import.meta.dir, "../../public", fallback!))).toBe(true);
  });

  test("retains existing provider keys and ignores icon options", () => {
    expect(getLobeStaticSvgUrl("OpenAI")).toEndWith("/openai.svg");
    expect(getLobeStaticSvgUrl("Gemini.Color.size=24")).toEndWith("/gemini-color.svg");
    expect(getLobeStaticSvgUrl("Qwen.Color")).toEndWith("/qwen-color.svg");
    expect(getLocalLogoUrl("Mistral")).toBe("/assets/logos/mistralai.svg");
  });

  test.each([undefined, "", "ai", "not-a-real-provider", "https://example.com/logo.svg"])(
    "does not turn unknown metadata into a failing CDN request: %s", (input) => {
      expect(getLobeStaticSvgUrl(input)).toBeNull();
      expect(getLocalLogoUrl(input)).toBeNull();
    }
  );
});
