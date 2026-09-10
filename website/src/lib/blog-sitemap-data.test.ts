import { afterEach, describe, expect, test } from "bun:test";

const originalFetch = globalThis.fetch;
const originalKey = process.env.BLOGGER_ACCESS_KEY;
afterEach(() => {
  globalThis.fetch = originalFetch;
  if (originalKey === undefined) delete process.env.BLOGGER_ACCESS_KEY;
  else process.env.BLOGGER_ACCESS_KEY = originalKey;
});
let moduleId = 0;
async function loadBlog(enabled = true): Promise<typeof import("./blog")> {
  process.env.BLOGGER_ACCESS_KEY = enabled ? "test-only" : "";
  return import(`./blog.ts?sitemap-data-test=${moduleId++}`);
}

describe("complete blog sitemap data", () => {
  test("paginates every indexable locale and drops article bodies from the snapshot", async () => {
    const blog = await loadBlog();
    const languages = new Set<string>();
    globalThis.fetch = (async (input) => {
      const url = new URL(String(input));
      if (url.pathname.endsWith("/categories")) return Response.json([{ slug: "news", name: "News" }]);
      const language = url.searchParams.get("language")!;
      languages.add(language);
      const offset = Number(url.searchParams.get("offset"));
      return Response.json(Array.from({ length: offset === 0 ? 20 : 1 }, (_, i) => ({
        id: `${language}-${offset + i}`, slug: `article-${offset + i}`, language,
        html_content: "<p>Large body</p>", title: "Article", updated_at: "2026-09-10T00:00:00Z", author: {},
      })));
    }) as typeof fetch;
    const data = await blog.getBlogSitemapData();
    expect(languages.size).toBe(9);
    expect(languages.has("id")).toBe(false);
    expect(data?.localizedPosts.every(({ posts }) => posts.length === 21)).toBe(true);
    expect(data?.localizedPosts[0].posts[0]).toEqual({ slug: "article-0", date: "2026-09-10T00:00:00Z" });
    expect(data?.categories).toEqual([{ slug: "news" }]);
    expect(JSON.stringify(data)).not.toContain("Large body");
  });

  test("does not publish a partial snapshot or fall back to another backend on one-language failure", async () => {
    const blog = await loadBlog();
    globalThis.fetch = (async (input) => {
      const url = new URL(String(input));
      expect(url.pathname).toContain("/api/integration/");
      if (url.searchParams.get("language") === "zh") return new Response("Unavailable", { status: 503 });
      return Response.json([]);
    }) as typeof fetch;
    expect(await blog.getBlogSitemapData()).toBeNull();
  });

  test("distinguishes a legitimately empty catalog from category failure", async () => {
    const blog = await loadBlog();
    globalThis.fetch = (async () => Response.json([])) as typeof fetch;
    expect((await blog.getBlogSitemapData())?.localizedPosts).toHaveLength(9);
    globalThis.fetch = (async (input) => String(input).endsWith("/categories")
      ? new Response("Unavailable", { status: 503 }) : Response.json([])) as typeof fetch;
    expect(await blog.getBlogSitemapData()).toBeNull();
  });

  test("rejects malformed JSON shapes and network errors", async () => {
    const blog = await loadBlog();
    globalThis.fetch = (async () => Response.json({ error: "invalid" })) as typeof fetch;
    expect(await blog.getBlogSitemapData()).toBeNull();
    globalThis.fetch = (async (input) => String(input).includes("language=zh")
      ? Response.json({ error: "invalid locale list" }) : Response.json([])) as typeof fetch;
    expect(await blog.getBlogSitemapData()).toBeNull();
    globalThis.fetch = (async (input) => String(input).includes("language=zh")
      ? Response.json([{ slug: "wrong-language", language: "en" }]) : Response.json([])) as typeof fetch;
    expect(await blog.getBlogSitemapData()).toBeNull();
    globalThis.fetch = (async () => { throw new Error("network"); }) as typeof fetch;
    expect(await blog.getBlogSitemapData()).toBeNull();
  });

  test("does not silently truncate a legacy catalog larger than its first page", async () => {
    const blog = await loadBlog(false);
    globalThis.fetch = (async (input) => Response.json({ success: true, data:
      String(input).endsWith("/categories") ? [] : { list: [], total: 201 },
    })) as typeof fetch;
    expect(await blog.getBlogSitemapData()).toBeNull();
  });
});
