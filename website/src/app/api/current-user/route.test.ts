import { describe, expect, test } from "bun:test";
import { NextRequest } from "next/server";
import { GET } from "./route";

describe("current user API", () => {
  test("proxies session cookies to the console identity endpoint", async () => {
    const originalFetch = globalThis.fetch;
    const requests: Array<{ url: string; headers: Headers }> = [];
    globalThis.fetch = ((url: string | URL, init?: RequestInit) => {
      const headers = new Headers(init?.headers);
      requests.push({ url: String(url), headers });
      if (String(url).includes("/phone-status")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              success: true,
              data: {
                phone_bound: false,
                verification_required: true,
                sms_verification_enabled: true,
              },
            }),
            { status: 200, headers: { "content-type": "application/json" } },
          ),
        );
      }
      return Promise.resolve(
        new Response(
          JSON.stringify({ success: true, data: { id: 123, role: 1 } }),
          {
            status: 200,
            headers: { "content-type": "application/json" },
          },
        ),
      );
    }) as typeof fetch;

    try {
      const request = new NextRequest("https://flatkey.ai/api/current-user", {
        headers: { cookie: "session=abc" },
      });
      const response = await GET(request);

      expect(response.status).toBe(200);
      expect(response.headers.get("cache-control")).toBe("no-store");
      expect(requests.map((entry) => entry.url)).toEqual([
        expect.stringContaining("/api/user/analytics-self"),
        expect.stringContaining("/api/user/self/phone-status"),
      ]);
      expect(requests[0]?.headers.get("cookie")).toBe("session=abc");
      expect(requests[1]?.headers.get("cookie")).toBe("session=abc");
      expect(requests[1]?.headers.get("New-Api-User")).toBe("123");
      expect(await response.json()).toMatchObject({
        success: true,
        data: {
          id: 123,
          role: 1,
          phone_bound: false,
          verification_required: true,
        },
      });
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("does not request phone status for administrators", async () => {
    const originalFetch = globalThis.fetch;
    let requestCount = 0;
    globalThis.fetch = (() => {
      requestCount += 1;
      return Promise.resolve(
        new Response(
          JSON.stringify({ success: true, data: { id: 9, role: 10 } }),
          { status: 200, headers: { "content-type": "application/json" } },
        ),
      );
    }) as typeof fetch;

    try {
      const response = await GET(
        new NextRequest("https://flatkey.ai/api/current-user", {
          headers: { cookie: "session=admin" },
        }),
      );

      expect(requestCount).toBe(1);
      expect(await response.json()).toMatchObject({
        data: { id: 9, role: 10, verification_required: false },
      });
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
