import { describe, expect, test } from "bun:test";
import { rewriteBlogHref, sanitizeBlogHtml } from "./blog";

describe("rewriteBlogHref", () => {
  test("localizes public blog paths for translated pages", () => {
    expect(rewriteBlogHref("/blog/ai-api-retry-strategy", "zh")).toBe("/zh/blog/ai-api-retry-strategy");
    expect(rewriteBlogHref("/blog/category/gateway-comparisons", "vi")).toBe("/vi/blog/category/gateway-comparisons");
  });

  test("localizes same-site absolute links and preserves query and hash", () => {
    expect(rewriteBlogHref("https://flatkey.ai/pricing?tab=image#units", "zh")).toBe(
      "https://flatkey.ai/zh/pricing?tab=image#units"
    );
  });

  test("rewrites console-owned paths to the console origin", () => {
    expect(rewriteBlogHref("/dashboard", "zh")).toBe("https://console.flatkey.ai/dashboard");
    expect(rewriteBlogHref("/login?redirect=%2Fdashboard#top", "zh")).toBe(
      "https://console.flatkey.ai/sign-in?redirect=%2Fdashboard#top"
    );
    expect(rewriteBlogHref("https://flatkey.ai/sign-up?invite=abc", "ja")).toBe(
      "https://console.flatkey.ai/sign-up?invite=abc"
    );
  });

  test("keeps external links and anchors unchanged", () => {
    expect(rewriteBlogHref("https://example.com/blog/post", "zh")).toBe("https://example.com/blog/post");
    expect(rewriteBlogHref("#section-1", "zh")).toBe("#section-1");
  });

  test("normalizes quoted internal and external links before rewriting", () => {
    expect(rewriteBlogHref('\\"/setup\\"', "ja")).toBe("https://console.flatkey.ai/sign-up?redirect=%2Fkeys");
    expect(rewriteBlogHref('\\"https://flatkey.ai/pricing?tab=image\\"', "vi")).toBe(
      "https://flatkey.ai/vi/pricing?tab=image"
    );
    expect(rewriteBlogHref('\\"https://ai.google.dev/gemini-api/docs/pricing\\"', "ja")).toBe(
      "https://ai.google.dev/gemini-api/docs/pricing"
    );
  });
});

describe("sanitizeBlogHtml", () => {
  test("uses published translations, falls back to English, and removes dead destinations without losing text", () => {
    const html = sanitizeBlogHtml(
      '<p><a href="/blog/translated">Translated</a> <a href="/blog/english-only?q=1#example">English</a> <a href="/blog/missing">Missing article</a> <a href="https://example.com/blog/missing">External</a> <a href="/blog/category/news">Category</a></p>',
      "ja",
      { local: new Set(["translated"]), english: new Set(["translated", "english-only"]) },
    );
    expect(html).toContain('href="/ja/blog/translated"');
    expect(html).toContain('href="/blog/english-only?q=1#example"');
    expect(html).not.toContain('href="/ja/blog/missing"');
    expect(html).toContain("Missing article</a>");
    expect(html).toContain('href="https://example.com/blog/missing"');
    expect(html).toContain('href="/ja/blog/category/news"');
  });

  test("an unavailable catalog preserves links; a confirmed empty catalog removes dead links", () => {
    const content = '<a href="/blog/article">Article</a>';
    expect(sanitizeBlogHtml(content, "en")).toContain('href="/blog/article"');
    expect(sanitizeBlogHtml(content, "en", { local: new Set(), english: new Set() })).not.toContain("href=");
  });

  test("omits placeholder and mixed-content images in server HTML while retaining valid images", () => {
    const html = sanitizeBlogHtml('<img src="x"><img src="/blog/x"><img src="http://staging-router.flatkey.ai/api/status"><img src="http://example.com/image.png"><img src="/assets/cover.png"><img src="https://cdn.example.com/render?id=1">');
    expect(html.match(/<img /g)).toHaveLength(2);
    expect(html).toContain('src="/assets/cover.png"');
    expect(html).toContain('src="https://cdn.example.com/render?id=1"');
    expect(html).not.toContain("staging-router");
  });

  test("rewrites internal marketing links during sanitization", () => {
    const html = sanitizeBlogHtml(
      '<p><a href="/pricing">Pricing</a> and <a href="https://flatkey.ai/sign-up">Get a key</a></p>',
      "zh"
    );

    expect(html).toContain('href="/zh/pricing"');
    expect(html).toContain('href="https://console.flatkey.ai/sign-up"');
  });

  test("normalizes malformed quoted href, rel, and target values", () => {
    const html = sanitizeBlogHtml(
      `<p><a href='\\"/sign-up\\"' rel='\\"noopener' target='\\"_blank\\"'>Start</a></p>
       <p><a href='\\"https://ai.google.dev/gemini-api/docs/image-generation\\"'>Docs</a></p>`,
      "vi"
    );

    expect(html).toContain('href="https://console.flatkey.ai/sign-up"');
    expect(html).toContain('rel="noopener"');
    expect(html).toContain('target="_blank"');
    expect(html).toContain('href="https://ai.google.dev/gemini-api/docs/image-generation"');
  });

  test("demotes article body h1 headings so the page keeps one H1", () => {
    const html = sanitizeBlogHtml("<h1>Imported article title</h1><h2>Section</h2>");

    expect(html).not.toContain("<h1");
    expect(html).toContain('<h2 id="imported-article-title">Imported article title</h2>');
    expect(html).toContain('<h2 id="section">Section</h2>');
  });

  test("adds alt text and canonicalizes same-site image URLs", () => {
    const html = sanitizeBlogHtml(
      '<p><img src="http://www.flatkey.ai/uploads/hero.png" /><img src="https://flatkey.ai/uploads/second.png" alt="Hero" /></p>'
    );

    expect(html).toContain('src="https://flatkey.ai/uploads/hero.png" alt=""');
    expect(html).toContain('src="https://flatkey.ai/uploads/second.png" alt="Hero"');
  });
});
