import { afterEach, describe, expect, test } from "bun:test";

const originalFetch = globalThis.fetch;
const originalKey = process.env.BLOGGER_ACCESS_KEY;

afterEach(() => {
  globalThis.fetch = originalFetch;
  if (originalKey === undefined) delete process.env.BLOGGER_ACCESS_KEY;
  else process.env.BLOGGER_ACCESS_KEY = originalKey;
});

// Each module instance captures its own environment, independent of test order.
let moduleId = 0;
async function loadBlog(enabled = true): Promise<typeof import("./blog")> {
  process.env.BLOGGER_ACCESS_KEY = enabled ? "test-only-key" : "";
  return import(`./blog.ts?fetch-regression=${moduleId++}`);
}

function post(index: number, language = "en") {
  return {
    id: String(index), title: `Article ${index}`, slug: `article-${index}`,
    language, html_content: "<p>Body</p>", author: { nickname: "Writer" },
    updated_at: "2026-09-10T00:00:00Z",
  };
}

describe("Blogger SEO data fetching", () => {
  test("renders only published internal links and leaves links intact during a catalog outage", async () => {
    const blog = await loadBlog();
    const content = '<a href="/blog/article-1">Translated</a><a href="/blog/article-2">English</a><a href="/blog/missing">Missing</a>';
    globalThis.fetch = (async (input) => {
      const language = new URL(String(input)).searchParams.get("language")!;
      return Response.json(language === "en" ? [post(1), post(2)] : [post(1, language)]);
    }) as typeof fetch;
    const html = await blog.renderBlogHtml(content, "ja");
    expect(html).toContain('href="/ja/blog/article-1"');
    expect(html).toContain('href="/blog/article-2"');
    expect(html).not.toContain('href="/ja/blog/missing"');
    globalThis.fetch = (async () => new Response("Unavailable", { status: 503 })) as typeof fetch;
    expect(await blog.renderBlogHtml(content, "ja")).toContain('href="/ja/blog/missing"');
  });

  test("paginates cache-sized batches without losing the final page", async () => {
    const blog = await loadBlog();
    const posts = Array.from({ length: 41 }, (_, index) => post(index));
    const offsets: number[] = [];
    globalThis.fetch = (async (input, init) => {
      const url = new URL(String(input));
      const limit = Number(url.searchParams.get("limit"));
      const offset = Number(url.searchParams.get("offset"));
      expect(limit).toBe(20);
      expect(url.searchParams.get("language")).toBe("en");
      expect((init as RequestInit & { next?: { revalidate: number } })?.next?.revalidate).toBe(300);
      offsets.push(offset);
      return Response.json(posts.slice(offset, offset + limit));
    }) as typeof fetch;
    expect((await blog.getAllBlogPosts("en")).map((item) => item.slug)).toEqual(posts.map((item) => item.slug));
    expect(offsets).toEqual([0, 20, 40]);
  });

  test("checks only article details and excludes missing, mismatched and noindex locales", async () => {
    const blog = await loadBlog();
    const requests: URL[] = [];
    globalThis.fetch = (async (input) => {
      const url = new URL(String(input));
      requests.push(url);
      expect(url.pathname.endsWith("/posts/article-1")).toBe(true);
      expect(url.searchParams.has("limit")).toBe(false);
      const language = url.searchParams.get("language")!;
      if (language === "en" || language === "zh") return Response.json(post(1, language));
      if (language === "fr") return Response.json(post(1, "en"));
      if (language === "de") return Response.json(post(2, "de"));
      return new Response("Not found", { status: 404 });
    }) as typeof fetch;
    expect(await blog.getBlogPostLocales("article-1")).toEqual(["en", "zh"]);
    expect(requests).toHaveLength(9);
    expect(requests.some((url) => url.searchParams.get("language") === "id")).toBe(false);
  });

  test("does not scan lists or invent hreflang targets on upstream errors", async () => {
    const blog = await loadBlog();
    let calls = 0;
    globalThis.fetch = (async () => {
      calls++;
      return new Response("Unavailable", { status: 503 });
    }) as typeof fetch;
    expect(await blog.getBlogPostLocales("article-1")).toEqual([]);
    expect(calls).toBe(9);
  });

  test("encodes detail slugs without injecting query parameters", async () => {
    const blog = await loadBlog();
    globalThis.fetch = (async (input) => {
      const url = new URL(String(input));
      expect(url.pathname.endsWith("/posts/article%2F%3F%26")).toBe(true);
      expect([...url.searchParams.keys()]).toEqual(["language"]);
      return new Response("Not found", { status: 404 });
    }) as typeof fetch;
    expect(await blog.getBlogPostLocales("article/?&")).toEqual([]);
  });

  test("preserves English-only legacy discovery when Blogger is not configured", async () => {
    const blog = await loadBlog(false);
    let calls = 0;
    globalThis.fetch = (async (input) => {
      calls++;
      expect(String(input)).toContain("/api/blog/list?");
      return Response.json({ success: true, data: { list: [post(1)] } });
    }) as typeof fetch;
    expect(await blog.getBlogPostLocales("article-1")).toEqual(["en"]);
    expect(calls).toBe(1);
  });
});
