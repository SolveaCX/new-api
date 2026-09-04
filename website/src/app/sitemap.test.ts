import { describe, expect, test } from "bun:test";

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
        return Promise.resolve(new Response("not found", { status: 404 }));
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
