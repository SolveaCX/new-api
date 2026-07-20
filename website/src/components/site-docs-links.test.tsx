import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SiteConfigProvider } from "./site-config-provider";
import { SiteFooter } from "./site-footer";
import { SiteHeader } from "./site-header";

const DOCS_URL = "https://docs.example.com/start";

function renderHeader(docsUrl: string | null) {
  return renderToStaticMarkup(
    <SiteConfigProvider docsUrl={docsUrl}>
      <SiteHeader locale="en" pathname="/" />
    </SiteConfigProvider>
  );
}

function renderFooter(docsUrl: string | null) {
  return renderToStaticMarkup(
    <SiteConfigProvider docsUrl={docsUrl}>
      <SiteFooter locale="en" />
    </SiteConfigProvider>
  );
}

describe("website documentation links", () => {
  test("renders safe desktop and mobile header links after Models and before Use Case", () => {
    const html = renderHeader(DOCS_URL);
    const docsAnchors = html.match(/<a[^>]+href="https:\/\/docs\.example\.com\/start"[^>]*>/g) ?? [];

    expect(docsAnchors).toHaveLength(2);
    for (const anchor of docsAnchors) {
      expect(anchor).toContain('target="_blank"');
      expect(anchor).toContain('rel="noopener noreferrer"');
    }
    expect(html.indexOf(">Models<")).toBeLessThan(html.indexOf(">Documentation<"));
    expect(html.indexOf(">Documentation<")).toBeLessThan(html.indexOf(">Use Case<"));
  });

  test("renders a safe footer link before legal links", () => {
    const html = renderFooter(DOCS_URL);
    const docsAnchor = html.match(/<a[^>]+href="https:\/\/docs\.example\.com\/start"[^>]*>/)?.[0];

    expect(docsAnchor).toContain('target="_blank"');
    expect(docsAnchor).toContain('rel="noopener noreferrer"');
    expect(html.indexOf(">Documentation<")).toBeLessThan(html.indexOf(">Terms of Service<"));
  });

  test("hides header and footer entries when the setting is unavailable", () => {
    expect(renderHeader(null)).not.toContain("Documentation");
    expect(renderFooter(null)).not.toContain("Documentation");
  });
});
