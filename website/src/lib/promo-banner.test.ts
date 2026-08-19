import { describe, expect, test } from "bun:test";
import {
  DEFAULT_PROMO_BANNER_CONTENT,
  DEFAULT_PROMO_BANNER_HREF,
  DEFAULT_PROMO_BANNER_ICON,
  normalizePromoBannerSettings,
  promoBannerCopyForLocale,
} from "./promo-banner";

describe("normalizePromoBannerSettings", () => {
  test("falls back to the built-in banner when no copy is configured", () => {
    for (const data of [
      {},
      { official_website_banner_content: null },
      { official_website_banner_content: {} },
      { official_website_banner_content: "Launch credits are live." },
      { official_website_banner_content: ["Launch credits are live."] },
      { official_website_banner_content: { zh: "上线额度已开放。" } },
      { official_website_banner_content: { en: "   " } },
    ]) {
      expect(normalizePromoBannerSettings(data)).toEqual({
        content: DEFAULT_PROMO_BANNER_CONTENT,
        enabled: true,
        href: DEFAULT_PROMO_BANNER_HREF,
        icon: DEFAULT_PROMO_BANNER_ICON,
      });
    }
  });

  test("keeps the whole configured banner once english copy is present", () => {
    expect(
      normalizePromoBannerSettings({
        official_website_banner_content: {
          en: "  Launch credits are live.  ",
          zh: "上线额度已开放。",
          klingon: "nuqneH",
          de: "   ",
        },
        official_website_banner_enabled: true,
        official_website_banner_href: "  /campaigns/launch  ",
        official_website_banner_icon: "  data:image/png;base64,iVBORw0KGgo=  ",
      }),
    ).toEqual({
      content: { en: "Launch credits are live.", zh: "上线额度已开放。" },
      enabled: true,
      href: "/campaigns/launch",
      icon: "data:image/png;base64,iVBORw0KGgo=",
    });
  });

  test("hides the banner only when the console explicitly disables it", () => {
    expect(
      normalizePromoBannerSettings({ official_website_banner_enabled: false })
        .enabled,
    ).toBe(false);
    expect(
      normalizePromoBannerSettings({
        official_website_banner_content: { en: "Launch credits are live." },
        official_website_banner_enabled: false,
      }).enabled,
    ).toBe(false);
    expect(normalizePromoBannerSettings({}).enabled).toBe(true);
  });

  test("drops unsafe links and icons instead of rendering them", () => {
    for (const href of [
      "javascript:alert(1)",
      "data:text/html;base64,PHNjcmlwdD4=",
      "//evil.example.com",
      "campaigns/launch",
      "not a url",
      42,
    ]) {
      expect(
        normalizePromoBannerSettings({
          official_website_banner_content: { en: "Launch credits are live." },
          official_website_banner_href: href,
        }).href,
      ).toBe("");
    }

    for (const icon of [
      "https://cdn.example.com/icon.png",
      "data:text/html;base64,PHNjcmlwdD4=",
      "data:image/png,iVBORw0KGgo=",
      "data:image/svg+xml;utf8,<svg onload=alert(1)></svg>",
      42,
    ]) {
      expect(
        normalizePromoBannerSettings({
          official_website_banner_content: { en: "Launch credits are live." },
          official_website_banner_icon: icon,
        }).icon,
      ).toBe("");
    }
  });

  test("accepts absolute http and https links", () => {
    expect(
      normalizePromoBannerSettings({
        official_website_banner_content: { en: "Launch credits are live." },
        official_website_banner_href: "https://console.example.com/sign-up",
      }).href,
    ).toBe("https://console.example.com/sign-up");
  });
});

describe("promoBannerCopyForLocale", () => {
  test("prefers the requested locale and falls back to english", () => {
    const content = { en: "Launch credits are live.", zh: "上线额度已开放。" };
    expect(promoBannerCopyForLocale(content, "zh")).toBe("上线额度已开放。");
    expect(promoBannerCopyForLocale(content, "ja")).toBe(
      "Launch credits are live.",
    );
    expect(promoBannerCopyForLocale({}, "en")).toBe("");
  });
});
