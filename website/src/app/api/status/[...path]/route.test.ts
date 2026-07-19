import { describe, expect, test } from "bun:test";
import { NextRequest } from "next/server";
import { APP_CONSOLE_ORIGIN } from "@/lib/origins";
import { GET, POST } from "./route";

type RouteContext = { params: Promise<{ path: string[] }> };

function context(...path: string[]): RouteContext {
  return { params: Promise.resolve({ path }) };
}

describe("public status proxy", () => {
  test("allowlists every documented GET shape and preserves query strings", async () => {
    const originalFetch = globalThis.fetch;
    const requested: string[] = [];
    globalThis.fetch = ((input: RequestInfo | URL) => {
      requested.push(String(input));
      return Promise.resolve(new Response("{}", { headers: { "content-type": "application/json" } }));
    }) as typeof fetch;

    const cases = [
      ["summary"],
      ["components"],
      ["components", "gpt-5"],
      ["components", "模型-gpt"],
      ["components", "gpt-5", "history"],
      ["incidents"],
      ["incidents", "inc_public"],
      ["maintenance"],
      ["subscriptions", "verify"],
      ["subscriptions", "unsubscribe"],
    ];

    try {
      for (const path of cases) {
        const query = path.at(-1) === "history" ? "?range=7d" : path.at(-1) === "verify" ? "?token=a%2Bb" : "";
        const request = new NextRequest(`https://flatkey.ai/api/status/${path.join("/")}${query}`);
        expect((await GET(request, context(...path))).status).toBe(200);
      }

      expect(requested).toHaveLength(cases.length);
      expect(requested.every((url) => url.startsWith(`${APP_CONSOLE_ORIGIN}/api/status/`))).toBe(true);
      expect(requested).toContain(`${APP_CONSOLE_ORIGIN}/api/status/components/gpt-5/history?range=7d`);
      expect(requested).toContain(`${APP_CONSOLE_ORIGIN}/api/status/components/%E6%A8%A1%E5%9E%8B-gpt`);
      expect(requested).toContain(`${APP_CONSOLE_ORIGIN}/api/status/subscriptions/verify?token=a%2Bb`);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("forwards only safe GET headers and preserves upstream status and public cache headers", async () => {
    const originalFetch = globalThis.fetch;
    let forwarded: RequestInit | undefined;
    globalThis.fetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
      forwarded = init;
      return Promise.resolve(
        new Response('{"success":true}', {
          status: 206,
          headers: {
            "content-type": "application/status+json",
            "cache-control": "public, max-age=30",
            etag: '"status-v1"',
            "set-cookie": "session=upstream-secret",
          },
        })
      );
    }) as typeof fetch;

    try {
      const request = new NextRequest("https://flatkey.ai/api/status/summary", {
        headers: {
          accept: "application/json",
          authorization: "Bearer secret",
          cookie: "session=secret",
          "if-none-match": '"status-v0"',
          "x-api-key": "credential",
        },
      });
      const response = await GET(request, context("summary"));
      const headers = new Headers(forwarded?.headers);

      expect(forwarded?.method).toBe("GET");
      expect((forwarded as RequestInit & { next?: { revalidate?: number } })?.next?.revalidate).toBe(60);
      expect(headers.get("accept")).toBe("application/json");
      expect(headers.get("if-none-match")).toBe('"status-v0"');
      expect(headers.get("authorization")).toBeNull();
      expect(headers.get("cookie")).toBeNull();
      expect(headers.get("x-api-key")).toBeNull();
      expect(response.status).toBe(206);
      expect(response.headers.get("content-type")).toBe("application/status+json");
      expect(response.headers.get("cache-control")).toBe("public, max-age=30");
      expect(response.headers.get("etag")).toBe('"status-v1"');
      expect(response.headers.get("set-cookie")).toBeNull();
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("forces upstream GET errors to no-store even when upstream marks them cacheable", async () => {
    const originalFetch = globalThis.fetch;
    const upstreamResponses = [
      new Response('{"success":false}', { status: 404 }),
      new Response('{"success":false}', { status: 503, headers: { "cache-control": "public, max-age=30" } }),
    ];
    globalThis.fetch = (() => Promise.resolve(upstreamResponses.shift()!)) as typeof fetch;

    try {
      for (const expectedStatus of [404, 503]) {
        const response = await GET(new NextRequest("https://flatkey.ai/api/status/summary"), context("summary"));
        expect(response.status).toBe(expectedStatus);
        expect(response.headers.get("cache-control")).toBe("no-store");
      }
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("preserves successful conditional GET cache headers and ETags", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => Promise.resolve(
      new Response(null, {
        status: 304,
        headers: { "cache-control": "public, max-age=30", etag: '"status-v2"' },
      })
    )) as typeof fetch;

    try {
      const response = await GET(new NextRequest("https://flatkey.ai/api/status/summary"), context("summary"));
      expect(response.status).toBe(304);
      expect(response.headers.get("cache-control")).toBe("public, max-age=30");
      expect(response.headers.get("etag")).toBe('"status-v2"');
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("rejects admin, traversal, encoded, extra-segment, and method-confused requests without proxying", async () => {
    const originalFetch = globalThis.fetch;
    let calls = 0;
    globalThis.fetch = (() => {
      calls += 1;
      return Promise.resolve(new Response("{}"));
    }) as typeof fetch;

    try {
      const rejectedGets = [
        ["admin", "components"],
        ["components", "..", "admin"],
        ["components", "gpt%2Fadmin"],
        ["components", "gpt-5", "extra"],
        ["subscriptions"],
      ];
      for (const path of rejectedGets) {
        const response = await GET(new NextRequest(`https://flatkey.ai/api/status/${path.join("/")}`), context(...path));
        expect([404, 405]).toContain(response.status);
      }
      const response = await POST(
        new NextRequest("https://flatkey.ai/api/status/summary", { method: "POST", body: "{}" }),
        context("summary")
      );
      expect(response.status).toBe(405);
      expect(calls).toBe(0);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("forwards bounded raw POST bodies without credentials and disables caching", async () => {
    const originalFetch = globalThis.fetch;
    let forwarded: RequestInit | undefined;
    globalThis.fetch = ((_input: RequestInfo | URL, init?: RequestInit) => {
      forwarded = init;
      return Promise.resolve(new Response('{"success":true}', { status: 202, headers: { "content-type": "application/json" } }));
    }) as typeof fetch;

    try {
      const rawBody = '{ "email" : "reader@example.com", "component_ids" : [1] }';
      const request = new NextRequest("https://flatkey.ai/api/status/subscriptions", {
        method: "POST",
        body: rawBody,
        headers: {
          accept: "application/json",
          "content-type": "application/json; charset=utf-8",
          authorization: "Bearer secret",
          cookie: "session=secret",
        },
      });
      const response = await POST(request, context("subscriptions"));
      const headers = new Headers(forwarded?.headers);

      expect(forwarded?.method).toBe("POST");
      expect(forwarded?.cache).toBe("no-store");
      expect((forwarded as RequestInit & { next?: unknown })?.next).toBeUndefined();
      expect(headers.get("accept")).toBe("application/json");
      expect(headers.get("content-type")).toBe("application/json; charset=utf-8");
      expect(headers.get("authorization")).toBeNull();
      expect(headers.get("cookie")).toBeNull();
      expect(new TextDecoder().decode(forwarded?.body as ArrayBuffer)).toBe(rawBody);
      expect(response.status).toBe(202);
      expect(response.headers.get("cache-control")).toBe("no-store");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("allowlists subscription creation and unsubscribe POST shapes", async () => {
    const originalFetch = globalThis.fetch;
    const requested: string[] = [];
    globalThis.fetch = ((input: RequestInfo | URL) => {
      requested.push(String(input));
      return Promise.resolve(Response.json({ success: true, data: { message: "generic" } }));
    }) as typeof fetch;

    try {
      for (const path of [["subscriptions"], ["subscriptions", "unsubscribe"]]) {
        const request = new NextRequest(`https://flatkey.ai/api/status/${path.join("/")}`, { method: "POST", body: "{}" });
        expect((await POST(request, context(...path))).status).toBe(200);
      }
      expect(requested).toEqual([
        `${APP_CONSOLE_ORIGIN}/api/status/subscriptions`,
        `${APP_CONSOLE_ORIGIN}/api/status/subscriptions/unsubscribe`,
      ]);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("rejects oversized subscription bodies without proxying", async () => {
    const originalFetch = globalThis.fetch;
    let called = false;
    globalThis.fetch = (() => {
      called = true;
      return Promise.resolve(new Response("{}"));
    }) as typeof fetch;

    try {
      const response = await POST(
        new NextRequest("https://flatkey.ai/api/status/subscriptions", { method: "POST", body: "x".repeat(4097) }),
        context("subscriptions")
      );
      expect(response.status).toBe(413);
      expect(called).toBe(false);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("returns bounded 502 JSON without leaking network internals", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => Promise.reject(new Error("dial tcp db.internal: password=hunter2"))) as typeof fetch;

    try {
      const response = await GET(new NextRequest("https://flatkey.ai/api/status/summary"), context("summary"));
      const body = await response.text();
      expect(response.status).toBe(502);
      expect(response.headers.get("content-type")).toContain("application/json");
      expect(response.headers.get("cache-control")).toBe("no-store");
      expect(body.length).toBeLessThan(256);
      expect(body).not.toContain("db.internal");
      expect(body).not.toContain("hunter2");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
