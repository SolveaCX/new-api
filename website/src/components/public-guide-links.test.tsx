import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { LOCALES } from "@/lib/locales";
import { PublicGuideLinks } from "./public-guide-links";
import { StaticFeaturePage } from "./static-feature-page";

describe("single-language guide discovery", () => {
  test("links all eleven reported orphan pages from every locale without inventing translated routes", () => {
    for (const locale of LOCALES) {
      const html = renderToStaticMarkup(<PublicGuideLinks locale={locale} />);
      for (const path of ["/gateway", "/chinese-ai", "/tools/web-scraping-api", "/in", "/gpt-api-alternative", "/apify-alternative", "/pt/5-credit-promo", "/id-market", "/tools/google-search-api", "/br", "/openai-compatible"]) {
        expect(html).toContain(`href="${path}"`);
      }
      expect(html.match(/<a /g)).toHaveLength(11);
      expect(html).not.toContain('rel="nofollow"');
    }
  });
  test("adds visible guides only to usecases and preserves other feature pages", () => {
    expect(renderToStaticMarkup(<StaticFeaturePage pageKey="usecases" locale="en" />)).toContain('id="public-guides-heading"');
    expect(renderToStaticMarkup(<StaticFeaturePage pageKey="compute" locale="en" />)).not.toContain('id="public-guides-heading"');
  });
});
