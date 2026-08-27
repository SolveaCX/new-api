import { afterEach, describe, expect, test } from "bun:test";
import {
  ANNOUNCEMENT_LOGO_MAX_LENGTH,
  DOCS_LINK_REVALIDATE_SECONDS,
  DOCS_LINK_TIMEOUT_MS,
  getDocsUrl,
  getPublicSiteSettings,
  localizeAnnouncementLink,
  normalizeAnnouncement,
  resolveAnnouncementContent,
  resolveAnnouncementIntro,
  resolveAnnouncementLinkLabel,
  normalizeDocsUrl,
} from "./public-site-settings";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("normalizeDocsUrl", () => {
  test("accepts trimmed HTTP and HTTPS URLs", () => {
    expect(normalizeDocsUrl("  https://docs.example.com/guide  ")).toBe(
      "https://docs.example.com/guide",
    );
    expect(normalizeDocsUrl("http://docs.example.com")).toBe(
      "http://docs.example.com/",
    );
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
    globalThis.fetch = ((
      requestInput: RequestInfo | URL,
      requestInit?: RequestInit,
    ) => {
      input = requestInput;
      init = requestInit;
      return Promise.resolve(
        new Response(
          JSON.stringify({
            success: true,
            data: { docs_link: "https://docs.example.com/start" },
          }),
        ),
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
      announcements: [],
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
        announcements: [],
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
            }),
          ),
        ),
      () =>
        Promise.resolve(
          new Response(
            JSON.stringify({
              success: true,
              data: { docs_link: "javascript:alert(1)" },
            }),
          ),
        ),
      () => Promise.reject(new DOMException("Timed out", "AbortError")),
    ];

    for (const responseFactory of responseFactories) {
      globalThis.fetch = (() => responseFactory()) as typeof fetch;
      await expect(getDocsUrl()).resolves.toBeNull();
    }
  });
});

describe("announcement normalization", () => {
  const logo = "data:image/png;base64,iVBORw0KGgo=";

  test("normalizes canonical localized fields and legacy aliases", () => {
    const announcement = normalizeAnnouncement({
      id: 7,
      content: "Legacy content",
      content_i18n: { en: "English content", zh: "中文内容" },
      extra: "Legacy intro",
      intro_i18n: { en: "English intro", zh: "中文简介" },
      href: " /pricing?source=announcement#plans ",
      link_label_i18n: { en: "View pricing", zh: "查看价格" },
      icon: logo,
      publishDate: "2026-08-27T00:00:00Z",
      type: "success",
    });

    expect(announcement).toMatchObject({
      id: 7,
      content: "English content",
      content_i18n: { en: "English content", zh: "中文内容" },
      intro: "English intro",
      extra: "English intro",
      intro_i18n: { en: "English intro", zh: "中文简介" },
      link: "/pricing?source=announcement#plans",
      link_label: "View pricing",
      link_label_i18n: { en: "View pricing", zh: "查看价格" },
      logo,
    });
  });

  test("keeps scalar announcements usable and ignores unsafe or oversized logos", () => {
    const legacy = normalizeAnnouncement({
      content: "Legacy content",
      extra: "Legacy intro",
      link: "/pricing",
    });
    expect(legacy).toMatchObject({
      content: "Legacy content",
      extra: "Legacy intro",
      intro: "Legacy intro",
      link: "/pricing",
    });

    expect(
      normalizeAnnouncement({
        content: "Unsafe logo",
        logo: "javascript:alert(1)",
      }),
    ).toEqual({ content: "Unsafe logo" });
    expect(
      normalizeAnnouncement({
        content: "Oversized logo",
        logo: `data:image/png;base64,${"a".repeat(ANNOUNCEMENT_LOGO_MAX_LENGTH)}`,
      }),
    ).toEqual({ content: "Oversized logo" });
    expect(
      normalizeAnnouncement({
        content: "Unsupported relative logo",
        logo: "/uploads/logo.svg",
      }),
    ).toEqual({ content: "Unsupported relative logo" });
    expect(
      normalizeAnnouncement({
        content: "Traversal logo",
        logo: "/assets/logos/../openai.svg",
      }),
    ).toEqual({ content: "Traversal logo" });
    expect(
      normalizeAnnouncement({
        content: "Built-in logo",
        logo: "/assets/logos/openai.svg",
      }),
    ).toMatchObject({
      content: "Built-in logo",
      logo: "/assets/logos/openai.svg",
    });
  });

  test("resolves locale, English, and scalar fallbacks in order", () => {
    const announcement = normalizeAnnouncement({
      content: "Legacy content",
      content_i18n: { en: "English content", zh: "中文内容" },
      extra: "Legacy intro",
      intro_i18n: { en: "English intro" },
      link_label: "Legacy CTA",
      link_label_i18n: { en: "English CTA" },
    });
    expect(announcement).not.toBeNull();
    expect(resolveAnnouncementContent(announcement!, "zh")).toBe("中文内容");
    expect(resolveAnnouncementContent(announcement!, "fr")).toBe(
      "English content",
    );
    expect(resolveAnnouncementIntro(announcement!, "zh")).toBe("English intro");
    expect(resolveAnnouncementLinkLabel(announcement!, "fr")).toBe(
      "English CTA",
    );
  });

  test("localizes relative links while preserving query/hash and external URLs", () => {
    expect(localizeAnnouncementLink("/pricing?source=ad#plans", "zh")).toBe(
      "/zh/pricing?source=ad#plans",
    );
    expect(localizeAnnouncementLink("/zh/pricing", "zh")).toBe("/zh/pricing");
    expect(localizeAnnouncementLink("https://example.com/pricing", "zh")).toBe(
      "https://example.com/pricing",
    );
    expect(localizeAnnouncementLink("//example.com/pricing", "zh")).toBe(
      undefined,
    );
  });
});
