import { describe, expect, test } from "bun:test";
import sitemap from "@/app/sitemap";
import { LOCALES } from "@/lib/locales";
import {
  COMPUTE_LANDING_PATH,
  getComputeLandingCtaUrl,
  getComputeLandingMetadataInput,
  getComputeLandingPageCopy,
} from "./compute-landing";

describe("Compute landing page", () => {
  test("uses the approved route and conversion CTA", () => {
    expect(COMPUTE_LANDING_PATH).toBe("/compute");
    expect(getComputeLandingCtaUrl("https://console.example.test")).toBe(
      "https://console.example.test/sign-up?redirect=/keys"
    );
  });

  test("has complete localized copy for every supported website locale", () => {
    for (const locale of LOCALES) {
      const copy = getComputeLandingPageCopy(locale);

      expect(copy.hero.subtitle.length).toBeGreaterThan(60);
      expect(copy.products).toHaveLength(3);
      expect(copy.unifiedPoints).toHaveLength(3);
      expect(copy.pricingRows).toHaveLength(3);
      expect(copy.whoFor).toHaveLength(3);
      expect(copy.faqs).toHaveLength(5);
      for (const product of copy.products) {
        expect(product.name.length).toBeGreaterThan(2);
        expect(product.price.length).toBeGreaterThan(2);
      }
    }
  });

  test("ships real Chinese copy alongside English", () => {
    const chinese = getComputeLandingPageCopy("zh");
    const english = getComputeLandingPageCopy("en");

    expect(chinese.hero.highlight).not.toBe(english.hero.highlight);
    expect(chinese.hero.primaryCta).toBe("打开 Compute");
    expect(chinese.badge).toContain("Compute");
  });

  test("keeps the flatkey-compute-fast serverless model in metadata and copy", () => {
    const en = getComputeLandingMetadataInput("en");
    expect(en.pathname).toBe(COMPUTE_LANDING_PATH);
    const copy = getComputeLandingPageCopy("en");
    expect(copy.products[0]?.price).toContain("flatkey-compute-fast");
  });

  test("adds the Compute page to the sitemap", async () => {
    const entries = await sitemap();

    expect(entries.some((entry) => entry.url === "https://flatkey.ai/compute")).toBe(true);
    expect(entries.some((entry) => entry.url === "https://flatkey.ai/zh/compute")).toBe(true);
  });
});
