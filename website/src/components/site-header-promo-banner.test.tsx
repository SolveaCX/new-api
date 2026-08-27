import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { LOCALES, localizePath } from "@/lib/locales";
import { SiteConfigProvider } from "./site-config-provider";
import { SiteHeader } from "./site-header";

describe("SiteHeader promo banner", () => {
  test("renders the DeepSeek V4 announcement at the top of the site", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={null}>
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).toContain(
      "DeepSeek V4 is here. Join our Discord get $5 free credits.",
    );
    expect(html).toContain(">Learn more →<");
    expect(html).toContain('aria-label="Dismiss DeepSeek V4 announcement"');
    expect(html).toContain('href="/blog/deepseek-v4-pro-vs-flash"');
    expect(html).toContain("DeepSeek V4");
    expect(html).not.toContain("Seedance");
    expect(
      html.indexOf(
        "DeepSeek V4 is here. Join our Discord get $5 free credits.",
      ),
    ).toBeLessThan(
      html.indexOf('aria-label="Dismiss DeepSeek V4 announcement"'),
    );
  });

  test("uses configured ads below the navigation and exposes carousel controls", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        announcements={[
          {
            id: 1,
            content: "Seedance 2.5 is available",
            extra: "Video models",
            link: "/models/seedance-2-5",
          },
          {
            id: 2,
            content: "New model pricing",
            extra: "Pricing update",
          },
        ]}
      >
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).toContain("Seedance 2.5 is available");
    expect(html).toContain("Video models");
    expect(html).toContain('href="/models/seedance-2-5"');
    expect(html).not.toContain("Learn more →");
    expect(html).toContain('data-promo-dots="true"');
    expect(html).not.toContain('aria-label="Previous advertisement"');
    expect(html).not.toContain('aria-label="Next advertisement"');
    expect(html).not.toContain('data-promo-progress="true"');
    expect(html).not.toMatch(/>\s*\d+\s*\/\s*\d+\s*</);
    expect(html.indexOf("Product")).toBeLessThan(
      html.indexOf("Seedance 2.5 is available"),
    );
  });

  test("renders localized intro, logo, content, and CTA in order", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        announcements={[
          {
            id: 10,
            content: "Legacy content",
            content_i18n: { en: "English content", zh: "中文内容" },
            extra: "Legacy intro",
            intro_i18n: { en: "English intro", zh: "中文简介" },
            link: "/pricing?source=announcement#plans",
            link_label: "Legacy CTA",
            link_label_i18n: { en: "View pricing", zh: "查看价格" },
            logo: "/assets/logos/deepseek.svg",
          },
        ]}
      >
        <SiteHeader locale="zh" pathname="/zh" />
      </SiteConfigProvider>,
    );

    expect(html).toContain("中文简介");
    expect(html).toContain("中文内容");
    expect(html).toContain("查看价格");
    expect(html).toContain('data-promo-cta="true"');
    expect(html).toContain('src="/assets/logos/deepseek.svg"');
    expect(html).toContain('href="/zh/pricing?source=announcement#plans"');
    expect(html.indexOf('data-promo-intro="true"')).toBeLessThan(
      html.indexOf('data-promo-logo="true"'),
    );
    expect(html.indexOf('data-promo-logo="true"')).toBeLessThan(
      html.indexOf("中文内容"),
    );
    expect(html).not.toContain("English content");
    expect(html).not.toContain("View pricing");
  });

  test("hides the banner when the backend has no configured ads", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={null} announcements={[]}>
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).not.toContain("DeepSeek V4 is here");
    expect(html).not.toContain("Next advertisement");
  });

  test("renders the localized DeepSeek V4 announcement for zh visitors", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={null}>
        <SiteHeader locale="zh" pathname="/zh" />
      </SiteConfigProvider>,
    );

    expect(html).toContain(
      "DeepSeek V4 来了。加入我们的 Discord，领取 5 美元免费额度。",
    );
    expect(html).toContain(">了解更多 →<");
    expect(html).toContain('aria-label="关闭 DeepSeek V4 公告"');
    expect(html).toContain('href="/zh/blog/deepseek-v4-pro-vs-flash"');
  });

  test("links every localized announcement to the matching article", () => {
    for (const locale of LOCALES) {
      const pathname = localizePath("/", locale);
      const html = renderToStaticMarkup(
        <SiteConfigProvider docsUrl={null}>
          <SiteHeader locale={locale} pathname={pathname} />
        </SiteConfigProvider>,
      );

      expect(html).toContain(
        `href="${localizePath("/blog/deepseek-v4-pro-vs-flash", locale)}"`,
      );
      expect(html).toContain("DeepSeek V4");
      expect(html).not.toContain("Seedance");
    }
  });
});
