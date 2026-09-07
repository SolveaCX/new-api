import { describe, expect, test } from "bun:test";
import { GET } from "./route";

const MEDIA_ID = `${"a".repeat(64)}.png`;

describe("website featured media proxy", () => {
  test("streams immutable media from the private console endpoint", async () => {
    const originalFetch = globalThis.fetch;
    let requestedUrl = "";
    globalThis.fetch = ((input: RequestInfo | URL) => {
      requestedUrl = String(input);
      return Promise.resolve(
        new Response("image-bytes", {
          status: 200,
          headers: {
            "content-type": "image/png",
            "content-length": "11",
            etag: '"asset-hash"',
          },
        }),
      );
    }) as typeof fetch;

    try {
      const response = await GET(
        new Request(`https://flatkey.ai/media/website-featured/${MEDIA_ID}`),
        { params: Promise.resolve({ id: MEDIA_ID }) },
      );

      expect(requestedUrl).toBe(
        `https://console.flatkey.ai/media/website-featured/${MEDIA_ID}`,
      );
      expect(response.status).toBe(200);
      expect(response.headers.get("content-type")).toBe("image/png");
      expect(response.headers.get("cache-control")).toBe(
        "public, max-age=31536000, immutable",
      );
      expect(await response.text()).toBe("image-bytes");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("rejects invalid media ids without calling the console", async () => {
    const originalFetch = globalThis.fetch;
    let called = false;
    globalThis.fetch = (() => {
      called = true;
      return Promise.reject(new Error("unexpected fetch"));
    }) as typeof fetch;

    try {
      const response = await GET(
        new Request("https://flatkey.ai/media/website-featured/not-valid.png"),
        { params: Promise.resolve({ id: "not-valid.png" }) },
      );
      expect(response.status).toBe(404);
      expect(called).toBe(false);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
