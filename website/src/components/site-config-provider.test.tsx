import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SiteConfigProvider, useSiteConfig } from "./site-config-provider";

function ConfigProbe() {
  const { docsUrl, promoBanner } = useSiteConfig();
  return (
    <span>
      {docsUrl ?? "hidden"}|{promoBanner.content}|{promoBanner.href}
    </span>
  );
}

describe("SiteConfigProvider", () => {
  test("provides one documentation URL to descendant chrome", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl="https://docs.example.com/start">
        <ConfigProbe />
      </SiteConfigProvider>
    );

    expect(html).toContain("https://docs.example.com/start");
    expect(html).toContain("Seedance is 15% off for a limited time.");
  });

  test("provides configured promo banner settings to descendant chrome", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider
        docsUrl={null}
        promoBanner={{
          content: "Custom event credits",
          enabled: true,
          href: "/campaigns/custom",
          icon: "data:image/png;base64,iVBORw0KGgo=",
        }}
      >
        <ConfigProbe />
      </SiteConfigProvider>
    );

    expect(html).toContain("Custom event credits");
    expect(html).toContain("/campaigns/custom");
  });

  test("defaults to a hidden documentation entry without a provider", () => {
    expect(renderToStaticMarkup(<ConfigProbe />)).toContain("hidden");
    expect(renderToStaticMarkup(<ConfigProbe />)).toContain(
      "Seedance is 15% off for a limited time.",
    );
  });
});
