import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { withIdFallback } from "@/lib/locales";
import type { PromptItem } from "@/lib/prompt-library";
import { PromptDirectoryPage } from "./prompt-directory";

function localized(en: string, zh: string) {
  return withIdFallback({ en, zh, es: en, fr: en, pt: en, ru: en, ja: en, vi: en, de: en });
}

function promptItem(props: { category: PromptItem["category"]; model: string; slug: string; tag: string; title: string }): PromptItem {
  return {
    artifact: { kind: "text", title: props.title, body: "Produced output" },
    category: props.category,
    model: props.model,
    output: { label: localized("Output", "产物"), ratio: "16:9" },
    prompt: `Create ${props.title}`,
    slug: props.slug,
    source: { label: "Test", platform: "Local migration", url: "https://example.com", capturedAt: "2026-09-07" },
    summary: localized(`${props.title} summary`, `${props.title}简介`),
    tags: [props.category, props.tag],
    title: localized(props.title, props.title),
    updatedAt: "2026-09-07",
  };
}

const items: PromptItem[] = [
  promptItem({ category: "image", model: "gpt-image-2", slug: "image-ad", tag: "ads", title: "Image ad" }),
  promptItem({ category: "video", model: "seedance-2.0", slug: "video-ad", tag: "ads", title: "Video ad" }),
  promptItem({ category: "text", model: "gpt-5", slug: "launch-copy", tag: "copywriting", title: "Launch copy" }),
];

describe("PromptDirectoryPage", () => {
  test("uses browseable content sections instead of filter controls", () => {
    const html = renderToStaticMarkup(<PromptDirectoryPage locale="zh" items={items} />);

    expect(html).toContain("按媒介浏览");
    expect(html).toContain("按模型浏览");
    expect(html).toContain("按场景浏览");
    expect(html).toContain('href="/zh/prompts/image"');
    expect(html).toContain('href="/zh/prompts/video"');
    expect(html).toContain('href="/zh/prompts?model=gpt-image-2#prompt-collection"');
    expect(html).toContain('href="/zh/prompts?useCase=ads#prompt-collection"');
    expect(html).not.toContain("<input");
    expect(html).not.toContain("<select");
    expect(html).not.toContain(">筛选<");
    expect(html).not.toContain(">重置<");
  });

  test("turns model query links into a dedicated content collection", () => {
    const html = renderToStaticMarkup(
      <PromptDirectoryPage locale="zh" items={items} initialSearch={{ model: "seedance-2.0" }} />,
    );

    expect(html).toContain('id="prompt-collection"');
    expect(html).toContain(">seedance-2.0<");
    expect(html).toContain("1 条提示词");
    expect(html).toContain('href="/zh/prompts#prompt-collection"');
  });
});
