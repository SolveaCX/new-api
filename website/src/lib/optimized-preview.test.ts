import { describe, expect, test } from "bun:test";
import { optimizedPreviewSrc } from "./optimized-preview";

describe("oversized public preview optimization", () => {
  test("uses the existing optimizer only for the three verified large assets", () => {
    for (const src of [
      "/assets/prompts/selected-playground/book-cover.png",
      "/assets/prompts/selected-playground/ximen-qing-100-panel-storyboard.jpg",
      "/media/website-featured/de31c7e32d1280c2586d8f36b69bee1a83a2f48c4241bfc5fc790d75cdc2660f.png",
    ]) {
      const url = new URL(optimizedPreviewSrc(src), "https://example.test");
      expect(url.pathname).toBe("/_next/image");
      expect(url.searchParams.get("url")).toBe(src);
      expect(url.searchParams.get("w")).toBe("1200");
      expect(url.searchParams.get("q")).toBe("75");
    }
  });
  test("preserves unrelated, remote, uploaded and already optimized sources", () => {
    for (const src of ["/logo.svg", "/assets/other.jpg", "https://cdn.example.test/image.png", "blob:upload", "data:image/png;base64,abc", "/_next/image?url=x&w=1200&q=75"]) {
      expect(optimizedPreviewSrc(src)).toBe(src);
    }
  });
});
