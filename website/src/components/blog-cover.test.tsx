import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { BlogCover, blogCoverSource, DEFAULT_BLOG_COVER } from "./blog-cover";

describe("blog covers", () => {
  test("missing and malformed covers use the local branded cover", () => {
    for (const cover of [undefined, "", "  ", "javascript:alert(1)", "//example.com/a.png", 'https://example.com/a.png" onerror="alert(3)', "not-a-url"]) {
      expect(blogCoverSource(cover)).toBe(DEFAULT_BLOG_COVER);
    }
  });
  test("valid local and remote covers are preserved", () => {
    for (const cover of ["/assets/cover.png", "https://example.com/cover.png", "https://example.com/a%20b.png"]) {
      expect(blogCoverSource(cover)).toBe(cover);
    }
  });
  test("fallback is server rendered and decorative beside the card title", () => {
    const html = renderToStaticMarkup(<BlogCover title="Article" />);
    expect(html).toContain(`src="${DEFAULT_BLOG_COVER}"`);
    expect(html).toContain('alt=""');
    expect(html).toContain('loading="lazy"');
  });
  test("remote cover titles remain escaped", () => {
    const html = renderToStaticMarkup(<BlogCover cover="https://example.com/a.png" title={'Hello "><script>alert(1)</script>'} />);
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });
});
