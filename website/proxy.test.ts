import { describe, expect, test } from "bun:test";
import { NextRequest } from "next/server";
import { proxy, resolveModelAliasRedirectPath, resolvePermanentSeoRedirectPath } from "./src/proxy";

function request(path: string, headers: Record<string, string> = {}) {
  return new NextRequest(`https://flatkey.ai${path}`, { headers });
}

describe("website proxy language redirects", () => {
  const originalCookieSessionDomain = process.env.COOKIE_SESSION_DOMAIN;

  function withCookieSessionDomain<T>(domain: string | undefined, callback: () => T): T {
    if (domain === undefined) {
      delete process.env.COOKIE_SESSION_DOMAIN;
    } else {
      process.env.COOKIE_SESSION_DOMAIN = domain;
    }

    try {
      return callback();
    } finally {
      if (originalCookieSessionDomain === undefined) {
        delete process.env.COOKIE_SESSION_DOMAIN;
      } else {
        process.env.COOKIE_SESSION_DOMAIN = originalCookieSessionDomain;
      }
    }
  }

  function setCookieHeaders(response: Response | undefined): string[] {
    return response?.headers.getSetCookie?.() ?? response?.headers.get("set-cookie")?.split(/,\s*(?=fk_locale=)/) ?? [];
  }

  test("redirects ordinary users and preserves query strings", async () => {
    const response = await proxy(request("/pricing?vendor=OpenAI", { "accept-language": "ja-JP,ja;q=0.9" }));

    expect(response?.status).toBe(307);
    expect(response?.headers.get("location")).toBe("https://flatkey.ai/ja/pricing?vendor=OpenAI");
  });

  test("redirects the www host to the canonical site origin", async () => {
    const response = await proxy(new NextRequest("https://www.flatkey.ai/pricing?vendor=OpenAI"));

    expect(response?.status).toBe(301);
    expect(response?.headers.get("location")).toBe("https://flatkey.ai/pricing?vendor=OpenAI");
  });

  test("redirects a reverse-proxied www host to the canonical site origin", async () => {
    const response = await proxy(
      new NextRequest("http://127.0.0.1:4000/pricing?vendor=OpenAI", {
        headers: { "x-forwarded-host": "www.flatkey.ai" },
      })
    );

    expect(response?.status).toBe(301);
    expect(response?.headers.get("location")).toBe("https://flatkey.ai/pricing?vendor=OpenAI");
  });

  test("resolves legacy legal and model casing paths to permanent canonical paths", () => {
    expect(resolvePermanentSeoRedirectPath("/privacy-policy")).toBe("/privacy");
    expect(resolvePermanentSeoRedirectPath("/zh/privacy-policy")).toBe("/zh/privacy");
    expect(resolvePermanentSeoRedirectPath("/zh/user-agreement")).toBe("/zh/terms");
    expect(resolvePermanentSeoRedirectPath("/en/user-agreement")).toBe("/terms");
    expect(resolvePermanentSeoRedirectPath("/id/models/MiniMax-H3")).toBe("/id/models/minimax-h3");
    expect(resolvePermanentSeoRedirectPath("/id/models/minimax-h3")).toBeNull();
  });

  test("permanently redirects a legacy legal path and preserves its query string", async () => {
    const response = await proxy(request("/zh/privacy-policy?source=gsc"));

    expect(response?.status).toBe(301);
    expect(response?.headers.get("location")).toBe("https://flatkey.ai/zh/privacy?source=gsc");
  });

  test("permanently redirects a model casing alias", async () => {
    const response = await proxy(request("/id/models/MiniMax-H3"));

    expect(response?.status).toBe(301);
    expect(response?.headers.get("location")).toBe("https://flatkey.ai/id/models/minimax-h3");
  });

  test("redirects a live vendor-prefixed model and preserves its query string", async () => {
    const originalFetch = globalThis.fetch;
    const pricingRequests: string[] = [];
    try {
      globalThis.fetch = ((input: RequestInfo | URL) => {
        pricingRequests.push(String(input));
        return Promise.resolve(
          new Response(
            JSON.stringify({
              success: true,
              data: [{ model_name: "gemini-2.5-flash" }],
            }),
            { status: 200 }
          )
        );
      }) as typeof fetch;

      const response = await proxy(request("/zh/models/google/gemini-2.5-flash?utm_source=legacy"));

      expect(response.status).toBe(301);
      expect(response.headers.get("location")).toBe(
        "https://flatkey.ai/zh/models/gemini-2.5-flash?utm_source=legacy"
      );
      expect(pricingRequests).toEqual(["https://console.flatkey.ai/api/website/pricing?group=plg"]);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("resolves aliases only when the target exists in the live catalog", () => {
    const models = ["gemini-2.5-flash", "qwen3.5-27b", "gemini-pro-latest", "MiniMax-H3"];

    expect(resolveModelAliasRedirectPath("/zh/models/google/gemini-2.5-flash", models)).toBe(
      "/zh/models/gemini-2.5-flash"
    );
    expect(resolveModelAliasRedirectPath("/models/qwen/qwen3.5-27b", models)).toBe("/models/qwen3.5-27b");
    expect(resolveModelAliasRedirectPath("/es/models/~google/gemini-pro-latest", models)).toBe(
      "/es/models/gemini-pro-latest"
    );
    expect(resolveModelAliasRedirectPath("/models/minimax/MiniMax-H3", models)).toBe("/models/minimax-h3");
    expect(resolveModelAliasRedirectPath("/models/qwen/retired-model", models)).toBeNull();
  });

  test("keeps an unknown nested model as a 404 candidate when the catalog is unavailable", async () => {
    const originalFetch = globalThis.fetch;
    try {
      globalThis.fetch = (() => Promise.resolve(new Response("unavailable", { status: 503 }))) as typeof fetch;

      const response = await proxy(
        request("/models/qwen/retired-model", {
          "user-agent": "Googlebot/2.1",
        })
      );

      expect(response.status).toBe(200);
      expect(response.headers.get("location")).toBeNull();
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("prefers a live slash-containing model id over a vendor alias", () => {
    expect(
      resolveModelAliasRedirectPath("/models/google/model-with-slash", [
        "google/model-with-slash",
        "model-with-slash",
      ])
    ).toBe("/models/google%2Fmodel-with-slash");
  });

  test("does not rewrite canonical model pages or unrelated nested paths", () => {
    const models = ["gemini-2.5-flash"];
    expect(resolveModelAliasRedirectPath("/zh/models/gemini-2.5-flash", models)).toBeNull();
    expect(resolveModelAliasRedirectPath("/zh/pricing/google/gemini-2.5-flash", models)).toBeNull();
    expect(resolveModelAliasRedirectPath("/blog/models/google/gemini-2.5-flash", models)).toBeNull();
    expect(resolveModelAliasRedirectPath("/en/models/google/gemini-2.5-flash", models)).toBeNull();
    expect(resolveModelAliasRedirectPath("/models/unknown/gemini-2.5-flash", models)).toBeNull();
    expect(resolveModelAliasRedirectPath("/models/google/%E0%A4%A", models)).toBeNull();
  });

  test("keeps the bare homepage on the default locale", async () => {
    const response = await proxy(request("/", { "accept-language": "ja-JP,ja;q=0.9", cookie: "fk_locale=ja" }));

    expect(response?.headers.get("location")).toBeNull();
  });

  test("keeps same-origin clicks from English pages on English routes", async () => {
    const response = await proxy(
      request("/pricing", {
        "accept-language": "zh-CN,zh;q=0.9",
        cookie: "fk_locale=zh",
        referer: "https://flatkey.ai/",
      })
    );

    expect(response?.headers.get("location")).toBeNull();
  });

  test("does not redirect declared AI crawlers", async () => {
    const response = await proxy(
      request("/pricing", {
        "accept-language": "ja-JP,ja;q=0.9",
        "user-agent": "OAI-SearchBot/1.0",
      })
    );

    expect(response?.headers.get("location")).toBeNull();
  });

  test("migrates an existing language cookie to the shared cookie domain", async () => {
    const response = await withCookieSessionDomain(".flatkey.ai", () =>
      proxy(request("/pricing", { cookie: "fk_locale=ja" }))
    );

    expect(setCookieHeaders(response)).toContain("fk_locale=; Path=/; Max-Age=0; SameSite=Lax");
    expect(setCookieHeaders(response)).toContain("fk_locale=ja; Path=/; Domain=.flatkey.ai; Max-Age=31536000; SameSite=Lax");
  });

  test("clears ambiguous duplicate language cookies without rewriting a stale value", async () => {
    const response = await withCookieSessionDomain(".flatkey.ai", () =>
      proxy(request("/pricing", { cookie: "fk_locale=en; fk_locale=ja" }))
    );

    expect(setCookieHeaders(response)).toEqual(["fk_locale=; Path=/; Max-Age=0; SameSite=Lax"]);
  });

  test("clears ambiguous duplicate language cookies even when the first value is invalid", async () => {
    const response = await withCookieSessionDomain(".flatkey.ai", () =>
      proxy(request("/pricing", { cookie: "fk_locale=xx; fk_locale=ja" }))
    );

    expect(setCookieHeaders(response)).toEqual(["fk_locale=; Path=/; Max-Age=0; SameSite=Lax"]);
  });

  test("does not migrate the language cookie when no shared cookie domain is configured", async () => {
    const response = await withCookieSessionDomain(undefined, () =>
      proxy(request("/pricing", { cookie: "fk_locale=ja" }))
    );

    expect(response?.headers.get("set-cookie")).toBeNull();
  });
});
