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

  test("renders the CTA as a solid pill on the brand gradient, not an inline link", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={null}>
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    // Deep-purple gradient band with white copy, kept to a single slim row.
    expect(html).toContain("linear-gradient(90deg,#4c1d95_0%,#5b21b6_45%,#7c3aed_100%)");
    expect(html).toContain("font-semibold");
    expect(html).not.toContain("flex-wrap");
    // The CTA is a filled white pill, not the old underlined text link.
    expect(html).toContain("rounded-full bg-white");
    expect(html).toContain("text-[#4c1d95]");
    expect(html).not.toContain("underline decoration-[#AAA7B0]");
  });

  test("anchors the mobile menu below the header instead of a hardcoded offset", () => {
    // The banner grows to two lines on narrow screens, so a fixed pixel offset
    // would drift. The menu tracks the header box itself.
    for (const banner of [
      undefined,
      {
        content: { en: "Hidden campaign" },
        enabled: false,
        href: "/hidden",
        icon: "",
      },
    ]) {
      const html = renderToStaticMarkup(
        <SiteConfigProvider docsUrl={null} promoBanner={banner}>
          <SiteHeader locale="en" pathname="/" />
        </SiteConfigProvider>,
      );

      expect(html).toContain("absolute inset-x-0 top-full");
      expect(html).not.toContain("top-[112px]");
      expect(html).not.toContain("top-[136px]");
    }
  });

  test("collapses the CTA to an arrow on mobile so the message keeps its width", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={null}>
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    // Circular arrow under 700px, labelled pill from 700px up.
    expect(html).toContain("size-[26px]");
    expect(html).toContain("min-[700px]:w-auto");
    expect(html).toContain("lucide-arrow-right size-3.5 min-[700px]:hidden");
    expect(html).toContain('class="hidden min-[700px]:inline"');
    // Message wraps on mobile rather than being cut off, truncates on desktop.
    expect(html).toContain("min-[700px]:truncate");
    // Room reserved on the right so the CTA never collides with the dismiss button.
    expect(html).toContain("pr-14");
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
  });
});
