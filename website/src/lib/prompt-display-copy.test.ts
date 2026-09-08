import { describe, expect, test } from "bun:test";
import { getPromptDisplayCopy, localizePromptTag } from "./prompt-display-copy";
import type { PromptItem } from "./prompt-library";

const item: PromptItem = {
  artifact: { kind: "image", alt: "", url: "/image.png" },
  category: "image",
  model: "gpt-image-2",
  output: { label: { en: "Image", zh: "图像" } as PromptItem["output"]["label"], ratio: "1:1" },
  prompt: "make an image",
  slug: "sample",
  source: { capturedAt: "2026-09-07", label: "test", platform: "External", url: "https://example.com" },
  summary: { en: "English-only summary" } as PromptItem["summary"],
  tags: ["image-editing"],
  title: { en: "English title" } as PromptItem["title"],
  updatedAt: "2026-09-07",
};

describe("prompt display copy", () => {
  test("uses a localized fallback when API content has no translation", () => {
    expect(getPromptDisplayCopy(item, "zh").summary).toBe("真实图片案例，附有完整提示词、适用模型和来源。");
  });

  test("localizes common prompt tags", () => {
    expect(localizePromptTag("image-editing", "zh")).toBe("图像编辑");
    expect(localizePromptTag("image-editing", "en")).toBe("Image editing");
  });
});
