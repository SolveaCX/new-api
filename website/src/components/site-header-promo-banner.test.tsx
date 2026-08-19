import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SiteConfigProvider } from "./site-config-provider";
import { SiteHeader } from "./site-header";

describe("SiteHeader promo banner", () => {
  test("renders the default Seedance promo banner at the top of the site", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={null}>
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).toContain("Seedance is 15% off for a limited time.");
    expect(html).toContain("Join our Discord to get $5 in free credit.");
    expect(html).toContain(">Learn more →<");
    expect(html).toContain('aria-label="Dismiss website banner"');
    expect(html).toContain('href="/models/seedance-api"');
    expect(html.indexOf("Seedance is 15% off for a limited time.")).toBeLessThan(
      html.indexOf('aria-label="Dismiss website banner"'),
    );
  });

  test("renders configured promo banner content, link, and uploaded icon", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: "Black Friday credits are live.",
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
    expect(html).not.toContain("Seedance is 15% off for a limited time.");
    expect(html).not.toContain("/assets/logos/bytedance.svg");
  });

  test("localizes configured relative banner links", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: "加入活动领取额度。",
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

  test("hides the promo banner when it is disabled by site settings", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: "Hidden campaign",
          enabled: false,
          href: "/hidden",
          icon: "data:image/png;base64,iVBORw0KGgo=",
        }}
      >
        <SiteHeader locale="en" pathname="/" />
      </SiteConfigProvider>,
    );

    expect(html).not.toContain("Hidden campaign");
    expect(html).not.toContain("Seedance is 15% off for a limited time.");
    expect(html).toContain("top-[72px] max-h-[calc(100dvh-72px)]");
  });
});
