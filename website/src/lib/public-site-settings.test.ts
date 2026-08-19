import { afterEach, describe, expect, test } from "bun:test";
import {
  DOCS_LINK_REVALIDATE_SECONDS,
  DOCS_LINK_TIMEOUT_MS,
  getDocsUrl,
  getPublicSiteSettings,
  normalizeDocsUrl,
} from "./public-site-settings";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("normalizeDocsUrl", () => {
  test("accepts trimmed HTTP and HTTPS URLs", () => {
    expect(normalizeDocsUrl("  https://docs.example.com/guide  ")).toBe("https://docs.example.com/guide");
    expect(normalizeDocsUrl("http://docs.example.com")).toBe("http://docs.example.com/");
  });

  test("rejects empty, non-string, relative, malformed, and unsafe URLs", () => {
    for (const value of [
      undefined,
      null,
      42,
      "",
      "   ",
      "/docs",
      "not a url",
      "javascript:alert(1)",
      "data:text/plain,test",
    ]) {
      expect(normalizeDocsUrl(value)).toBeNull();
    }
  });
});

describe("getDocsUrl", () => {
  test("reads docs_link from the public status response with bounded caching", async () => {
    let input: RequestInfo | URL | undefined;
    let init: (RequestInit & { next?: { revalidate?: number } }) | undefined;
    globalThis.fetch = ((requestInput: RequestInfo | URL, requestInit?: RequestInit) => {
      input = requestInput;
      init = requestInit;
      return Promise.resolve(
        new Response(
          JSON.stringify({
            success: true,
            data: { docs_link: "https://docs.example.com/start" },
          })
        )
      );
    }) as typeof fetch;

    await expect(getDocsUrl()).resolves.toBe("https://docs.example.com/start");
    expect(String(input)).toBe("https://console.flatkey.ai/api/status");
    expect(init?.headers).toEqual({ accept: "application/json" });
    expect(init?.next?.revalidate).toBe(DOCS_LINK_REVALIDATE_SECONDS);
    expect(init?.signal).toBeInstanceOf(AbortSignal);
    expect(DOCS_LINK_REVALIDATE_SECONDS).toBe(60);
    expect(DOCS_LINK_TIMEOUT_MS).toBe(3000);
  });

  test("reads Google One Tap settings from the public status response", async () => {
    globalThis.fetch = (() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            success: true,
            data: {
              docs_link: "https://docs.example.com/start",
              google_client_id: " google-client.apps.googleusercontent.com ",
              google_oauth: true,
              official_website_banner_content: "Official launch credits are live.",
              official_website_banner_enabled: true,
              official_website_banner_href: "https://console.example.com/sign-up",
              official_website_banner_icon: "data:image/png;base64,iVBORw0KGgo=",
            },
          }),
        ),
      )) as typeof fetch;

    await expect(getPublicSiteSettings()).resolves.toEqual({
      docsUrl: "https://docs.example.com/start",
      googleOneTap: {
        clientId: "google-client.apps.googleusercontent.com",
        enabled: true,
      },
      promoBanner: {
        content: "Official launch credits are live.",
        enabled: true,
        href: "https://console.example.com/sign-up",
        icon: "data:image/png;base64,iVBORw0KGgo=",
      },
    });
  });

  test("reads official website banner settings from the public status response", async () => {
    globalThis.fetch = (() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            success: true,
            data: {
              official_website_banner_content: " Join the launch event. ",
              official_website_banner_enabled: false,
              official_website_banner_href: " /campaigns/launch ",
              official_website_banner_icon: " data:image/webp;base64,UklGRg== ",
            },
          }),
        ),
      )) as typeof fetch;

    await expect(getPublicSiteSettings()).resolves.toEqual({
      docsUrl: null,
      googleOneTap: {
        clientId: null,
        enabled: false,
      },
      promoBanner: {
        content: "Join the launch event.",
        enabled: false,
        href: "/campaigns/launch",
        icon: "data:image/webp;base64,UklGRg==",
      },
    });
  });

  test("disables Google One Tap when OAuth is off or the client id is missing", async () => {
    for (const data of [
      {
        google_client_id: "google-client.apps.googleusercontent.com",
        google_oauth: false,
      },
      { google_client_id: "", google_oauth: true },
      { google_oauth: true },
    ]) {
      globalThis.fetch = (() =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              success: true,
              data,
            }),
          ),
        )) as typeof fetch;

      await expect(getPublicSiteSettings()).resolves.toEqual({
        docsUrl: null,
        googleOneTap: {
          clientId: data.google_client_id?.trim() || null,
          enabled: false,
        },
        promoBanner: {
          content:
            "Seedance is 15% off for a limited time. Join our Discord to get $5 in free credit.",
          enabled: true,
          href: "/models/seedance-api",
          icon: "/assets/logos/bytedance.svg",
        },
      });
    }
  });

  test("returns null for non-2xx, failed envelopes, invalid payloads, and request errors", async () => {
    const responseFactories: Array<() => Promise<Response>> = [
      () => Promise.resolve(new Response("{}", { status: 503 })),
      () =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              success: false,
              data: { docs_link: "https://docs.example.com" },
            })
          )
        ),
      () =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              success: true,
              data: { docs_link: "javascript:alert(1)" },
            })
          )
        ),
      () => Promise.reject(new DOMException("Timed out", "AbortError")),
    ];

    for (const responseFactory of responseFactories) {
      globalThis.fetch = (() => responseFactory()) as typeof fetch;
      await expect(getDocsUrl()).resolves.toBeNull();
    }
  });
});
