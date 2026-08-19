import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import {
  DEFAULT_PROMO_BANNER_CONTENT,
  DEFAULT_PROMO_BANNER_HREF,
  DEFAULT_PROMO_BANNER_ICON,
} from "@/lib/promo-banner";
import { SiteConfigProvider } from "./site-config-provider";
import { SiteHeader } from "./site-header";

describe("SiteHeader promo banner", () => {
  test("renders the built-in default promo banner at the top of the site", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={null}>
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).toContain(DEFAULT_PROMO_BANNER_CONTENT.en);
    expect(html).toContain(">Learn more →<");
    expect(html).toContain('aria-label="Dismiss website banner"');
    expect(html).toContain(`href="${DEFAULT_PROMO_BANNER_HREF}"`);
    expect(html.indexOf(DEFAULT_PROMO_BANNER_CONTENT.en)).toBeLessThan(
      html.indexOf('aria-label="Dismiss website banner"'),
    );
  });

  test("renders configured promo banner content, link, and uploaded icon", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: { en: "Black Friday credits are live." },
          enabled: true,
          href: "https://console.example.com/sign-up",
          icon: "data:image/png;base64,iVBORw0KGgo=",
        }}
      >
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).toContain("Black Friday credits are live.");
    expect(html).toContain(">Learn more →<");
    expect(html).toContain('href="https://console.example.com/sign-up"');
    expect(html).toContain('src="data:image/png;base64,iVBORw0KGgo="');
    expect(html).not.toContain(DEFAULT_PROMO_BANNER_CONTENT.en);
    expect(html).not.toContain(DEFAULT_PROMO_BANNER_ICON);
  });

  test("localizes configured relative banner links", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: {
            en: "Join the campaign for free credit.",
            zh: "加入活动领取额度。",
          },
          enabled: true,
          href: "/campaigns/summer",
          icon: "",
        }}
      >
        <SiteHeader locale="zh" pathname="/zh" />
      </SiteConfigProvider>,
    );

    expect(html).toContain("加入活动领取额度。");
    expect(html).toContain(">了解更多 →<");
    expect(html).toContain('aria-label="关闭官网横幅"');
    expect(html).toContain('href="/zh/campaigns/summer"');
  });

  test("falls back to the configured english copy for unconfigured locales", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: {
            en: "Join the campaign for free credit.",
            zh: "加入活动领取额度。",
          },
          enabled: true,
          href: "/campaigns/summer",
          icon: "",
        }}
      >
        <SiteHeader locale="ja" pathname="/ja" />
      </SiteConfigProvider>,
    );

    expect(html).toContain("Join the campaign for free credit.");
    expect(html).toContain(">詳細を見る →<");
    expect(html).not.toContain("加入活动领取额度。");
  });

  test("renders a copy-only banner when no link is configured", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: { en: "Scheduled maintenance this Sunday." },
          enabled: true,
          href: "",
          icon: "",
        }}
      >
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).toContain("Scheduled maintenance this Sunday.");
    expect(html).toContain('aria-label="Dismiss website banner"');
    expect(html).not.toContain(">Learn more →<");
  });

  test("hides the promo banner when it is disabled by site settings", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: { en: "Hidden campaign" },
          enabled: false,
          href: "/hidden",
          icon: "data:image/png;base64,iVBORw0KGgo=",
        }}
      >
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).not.toContain("Hidden campaign");
    expect(html).not.toContain(DEFAULT_PROMO_BANNER_CONTENT.en);
    expect(html).toContain("top-[72px] max-h-[calc(100dvh-72px)]");
  });

  test("hides the promo banner when the configured copy is empty", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: {},
          enabled: true,
          href: "/campaigns/summer",
          icon: "",
        }}
      >
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).not.toContain('aria-label="Dismiss website banner"');
    expect(html).toContain("top-[72px] max-h-[calc(100dvh-72px)]");
  });
});
