import { afterEach, describe, expect, mock, test } from "bun:test";
import { fetchCliMediaPromptItems, getCliMediaPromptItems } from "./prompt-library";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

function apiImage(slug: string) {
  return {
    artifact: { alt: "API image", kind: "image", url: "/api-image.png" },
    category: "image",
    model: "gpt-image-2",
    output: { label: { en: "Image", zh: "图像" }, ratio: "1:1" },
    prompt: "Create the API image.",
    slug,
    source: { captured_at: "2026-09-07", label: "Flatkey", platform: "Flatkey generated", url: "" },
    summary: { en: "API summary", zh: "接口摘要" },
    tags: ["product"],
    title: { en: "API prompt", zh: "接口提示词" },
    updatedAt: "2026-09-07",
  };
}

describe("fetchCliMediaPromptItems", () => {
  test("requests the complete public page and keeps missing categories on static fallback", async () => {
    const requested: string[] = [];
    globalThis.fetch = ((input: RequestInfo | URL) => {
      requested.push(String(input));
      return Promise.resolve(new Response(JSON.stringify({
        success: true,
        data: { items: [apiImage("api-image")], page: 1, page_size: 100, total: 1 },
      }), { status: 200 }));
    }) as typeof fetch;

    const items = await fetchCliMediaPromptItems();

    expect(requested).toHaveLength(1);
    expect(requested[0]).toContain("/api/prompt-library?page=1&size=100");
    expect(items.some((item) => item.slug === "api-image")).toBe(true);
    expect(items.some((item) => item.category === "video")).toBe(true);
    expect(items.some((item) => item.category === "text")).toBe(true);
    expect(items.some((item) => item.category === "agent")).toBe(true);
    expect(items.some((item) => item.category === "image" && item.slug !== "api-image")).toBe(false);
  });

  test("uses the checked-in video library when the API video category is empty", async () => {
    globalThis.fetch = (() => Promise.resolve(new Response(JSON.stringify({
      success: true,
      data: { items: [], page: 1, page_size: 100, total: 0 },
    }), { status: 200 }))) as typeof fetch;

    const items = await fetchCliMediaPromptItems("video");

    expect(items).toEqual(getCliMediaPromptItems("video"));
    expect(items.length).toBeGreaterThan(0);
  });
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
