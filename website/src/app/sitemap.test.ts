import { describe, expect, mock, test } from "bun:test";

// Exercise generation without a Next request context. Actual cache behavior is
// covered by the production-mode HTTP regression, not by these unit doubles.
mock.module("next/server", () => ({ connection: async () => {} }));
mock.module("next/cache", () => ({ unstable_cache: (fn: () => unknown) => fn }));

describe("sitemap", () => {
  test("omits non-canonical query pages and redirect-only model aliases", async () => {
    const originalFetch = globalThis.fetch;
    const pricingRequests: string[] = [];
    try {
      globalThis.fetch = ((input: RequestInfo | URL) => {
        if (String(input).includes("/api/website/pricing")) {
          pricingRequests.push(String(input));
          return Promise.resolve(
            new Response(
              JSON.stringify({
                success: true,
                data: [
                  {
                    model_name: "gpt-5",
                    vendor_name: "OpenAI",
                    quota_type: 0,
                    model_ratio: 1,
                    completion_ratio: 1,
                  },
                  {
                    model_name: "MiniMax-H3",
                    quota_type: 1,
                    model_ratio: 1,
                    completion_ratio: 1,
                  },
                  {
                    model_name: "seedance-2-5",
                    quota_type: 1,
                    model_ratio: 1,
                    completion_ratio: 1,
                  },
                ],
                vendors: [],
              }),
              { status: 200 }
            )
          );
        }
        return Promise.resolve(Response.json({ success: true, data: String(input).includes("/categories") ? [] : { list: [], total: 0 } }));
      }) as typeof fetch;

      const { default: sitemap } = await import("./sitemap");
      const entries = await sitemap();
      const urls = entries.map((entry) => entry.url);
      const promoEntry = entries.find((entry) => entry.url === "https://flatkey.ai/pt/5-credit-promo");

      expect(urls.some((url) => url.includes("?vendor="))).toBe(false);
      expect(urls).not.toContain("https://flatkey.ai/models/gpt-api");
      expect(urls).not.toContain("https://flatkey.ai/models/claude-api");
      expect(urls).toContain("https://flatkey.ai/gpt-api");
      expect(urls).toContain("https://flatkey.ai/claude-api");
      expect(urls).toContain("https://flatkey.ai/models/minimax-h3");
      expect(urls).not.toContain("https://flatkey.ai/models/MiniMax-H3");
      expect(urls).toContain("https://flatkey.ai/models/seedance-2.5");
      expect(urls).not.toContain("https://flatkey.ai/models/seedance-2-5");
      expect(pricingRequests).toEqual(["https://console.flatkey.ai/api/website/pricing?group=plg"]);
      expect(urls).toContain("https://flatkey.ai/pt/5-credit-promo");
      expect(urls).not.toContain("https://flatkey.ai/5-credit-promo");
      expect(urls.some((url) => url.startsWith("https://flatkey.ai/cli/image/"))).toBe(true);
      expect(urls.some((url) => url.startsWith("https://flatkey.ai/cli/video/"))).toBe(true);
      expect(urls).not.toContain("https://flatkey.ai/id/about");
      expect(urls).not.toContain("https://flatkey.ai/id/docs");
      expect(urls).not.toContain("https://flatkey.ai/id/playground");
      expect(urls).not.toContain("https://flatkey.ai/pt/playground");
      expect(urls.some((url) => url.includes("/models?series=") && url.includes("/id/"))).toBe(false);
      expect(urls).not.toContain("https://flatkey.ai/id/privacy");
      expect(urls).not.toContain("https://flatkey.ai/id/refund-policy");
      expect(urls).not.toContain("https://flatkey.ai/id/sla");
      expect(urls).not.toContain("https://flatkey.ai/id/terms");
      expect(urls).not.toContain("https://flatkey.ai/id/usecases");
      expect(urls).not.toContain("https://flatkey.ai/id/blog/claude-api-proxy-vs-router");
      const aboutEntry = entries.find((entry) => entry.url === "https://flatkey.ai/about");
      expect(aboutEntry?.alternates?.languages).not.toHaveProperty("id-ID");
      const playgroundEntry = entries.find((entry) => entry.url === "https://flatkey.ai/playground");
      expect(playgroundEntry?.alternates?.languages).not.toHaveProperty("id-ID");
      expect(playgroundEntry?.alternates?.languages).not.toHaveProperty("pt-BR");
      const seriesEntry = entries.find((entry) => entry.url === "https://flatkey.ai/models?series=Claude");
      expect(seriesEntry?.alternates?.languages).not.toHaveProperty("id-ID");
      expect(promoEntry?.alternates?.languages).toMatchObject({
        "pt-BR": "https://flatkey.ai/pt/5-credit-promo",
      });
      expect(promoEntry?.alternates?.languages).not.toHaveProperty("pt-PT");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("fails closed when the pricing catalog is unavailable", async () => {
    const originalFetch = globalThis.fetch;
    try {
      globalThis.fetch = (() => Promise.resolve(new Response("unavailable", { status: 503 }))) as typeof fetch;

      const { default: sitemap } = await import("./sitemap");
      await expect(sitemap()).rejects.toThrow("Sitemap pricing catalog is unavailable");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
