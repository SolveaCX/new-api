import { afterEach, describe, expect, mock, test } from "bun:test";
import { fetchCliMediaPromptItems } from "./prompt-library";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function apiItem(overrides: Record<string, unknown> = {}) {
  return {
    artifact: { alt: "Result", kind: "image", url: "https://cdn.example.com/result.png" },
    category: "image",
    model: "gpt-image-2",
    output: { label: { en: "Image" }, ratio: "4:5" },
    prompt: "Create a product portrait.",
    slug: "product-portrait",
    source: { label: "Example", platform: "External", url: "https://example.com/source" },
    summary: { en: "A product portrait prompt." },
    tags: ["product"],
    title: { en: "Product portrait" },
    updatedAt: "2026-09-07",
    ...overrides,
  };
}

function mockPromptApi(items: unknown[]) {
  globalThis.fetch = mock(async () =>
    new Response(JSON.stringify({ data: { items } }), {
      headers: { "content-type": "application/json" },
      status: 200,
    })) as typeof fetch;
}

describe("prompt library API normalization", () => {
  test("preserves explicit upstream output ratios", async () => {
    mockPromptApi([apiItem()]);

    const items = await fetchCliMediaPromptItems("image");

    expect(items).toHaveLength(1);
    expect(items[0]?.output.ratio).toBe("4:5");
  });

  test("rejects non-http source URLs before they can reach links", async () => {
    mockPromptApi([
      apiItem({ slug: "unsafe-source", source: { label: "Unsafe", platform: "External", url: "javascript:alert(1)" } }),
      apiItem({ slug: "safe-source" }),
    ]);

    const items = await fetchCliMediaPromptItems("image");

    expect(items.map((item) => item.slug)).toEqual(["safe-source"]);
    expect(items[0]?.source.url).toBe("https://example.com/source");
  });
});
