import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SiteConfigProvider } from "./site-config-provider";
import { SiteFooter } from "./site-footer";
import { SiteHeader } from "./site-header";
import { SiteShell } from "./site-shell";

const DOCS_URL = "https://docs.example.com/start";

function renderHeader(docsUrl: string | null, pathname = "/") {
  return renderToStaticMarkup(
    <SiteConfigProvider docsUrl={docsUrl}>
      <SiteHeader locale="en" pathname={pathname} />
    </SiteConfigProvider>,
  );
}

function renderFooter(docsUrl: string | null) {
  return renderToStaticMarkup(
    <SiteConfigProvider docsUrl={docsUrl}>
      <SiteFooter locale="en" />
    </SiteConfigProvider>,
  );
}

function findDesktopGroupTrigger(html: string, label: string) {
  const labelIndex = html.indexOf(`>${label}<`);
  const buttonStart = html.lastIndexOf("<button", labelIndex);
  const buttonEnd = html.indexOf("</button>", labelIndex);
  if (labelIndex < 0 || buttonStart < 0 || buttonEnd < 0) return undefined;

  return html.slice(buttonStart, buttonEnd + "</button>".length);
}

function findButtonByAriaLabel(html: string, label: string) {
  const labelIndex = html.indexOf(`aria-label="${label}"`);
  const buttonStart = html.lastIndexOf("<button", labelIndex);
  const buttonEnd = html.indexOf(">", labelIndex);
  if (labelIndex < 0 || buttonStart < 0 || buttonEnd < 0) return undefined;

  return html.slice(buttonStart, buttonEnd + 1);
}

function findAnchorByHref(html: string, href: string) {
  return html.match(
    new RegExp(`<a[^>]+href="${href.replace("/", "\\/")}"[^>]*>[\\s\\S]*?<\\/a>`),
  )?.[0];
}

describe("website documentation links", () => {
  test("renders the public prompt library in the top navigation", () => {
    const enHtml = renderHeader(DOCS_URL);
    const zhHtml = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={DOCS_URL}>
        <SiteHeader locale="zh" pathname="/zh" />
      </SiteConfigProvider>,
    );

    expect(enHtml).toContain('href="/prompts"');
    expect(enHtml).toContain(">Prompts<");
    expect(zhHtml).toContain('href="/zh/prompts"');
    expect(zhHtml).toContain(">提示词<");
  });

  test("renders safe desktop and mobile resource links before Use Cases", () => {
    const html = renderHeader(DOCS_URL);
    const docsAnchors =
      html.match(/<a[^>]+href="https:\/\/docs\.example\.com\/start"[^>]*>/g) ??
      [];

    expect(docsAnchors.length).toBeGreaterThan(0);
    for (const docsAnchor of docsAnchors) {
      expect(docsAnchor).toContain('target="_blank"');
      expect(docsAnchor).toContain('rel="noopener noreferrer"');
    }
    expect(html.indexOf(">Ranking<")).toBeLessThan(html.indexOf(">Documentation<"));
    expect(html.indexOf(">Documentation<")).toBeLessThan(
      html.indexOf(">Use Cases<"),
    );
    expect(html).toContain('aria-label="Toggle navigation menu"');
    expect(html).not.toContain(">Menu<");
  });

  test("renders a safe footer link before legal links", () => {
    const html = renderFooter(DOCS_URL);
    const docsAnchor = html.match(
      /<a[^>]+href="https:\/\/docs\.example\.com\/start"[^>]*>/,
    )?.[0];

    expect(docsAnchor).toContain('target="_blank"');
    expect(docsAnchor).toContain('rel="noopener noreferrer"');
    expect(html.indexOf(">Documentation<")).toBeLessThan(
      html.indexOf(">Terms of Service<"),
    );
    expect(html).toContain('href="https://discord.gg/Xnm8Cc7JRD"');
  });

  test("hides header and footer entries when the setting is unavailable", () => {
    expect(renderHeader(null)).not.toContain("Documentation");
    expect(renderFooter(null)).not.toContain("Documentation");
  });

  test("marks desktop nav groups as dropdowns without decorating ordinary links", () => {
    const html = renderHeader(DOCS_URL);
    const productTrigger = findDesktopGroupTrigger(html, "Product");
    const resourceTrigger = findDesktopGroupTrigger(html, "Resource");
    const cliLink = findAnchorByHref(html, "/cli");
    const languageButton = findButtonByAriaLabel(html, "Change language");

    expect(productTrigger).toContain('aria-haspopup="menu"');
    expect(resourceTrigger).toContain('aria-haspopup="menu"');
    expect(productTrigger).toContain("lucide-chevron-down");
    expect(resourceTrigger).toContain("lucide-chevron-down");
    expect(productTrigger).toContain("border-0");
    expect(resourceTrigger).toContain("border-0");
    expect(productTrigger).toContain("bg-transparent");
    expect(resourceTrigger).toContain("bg-transparent");
    expect(productTrigger).toContain("text-[14px]");
    expect(cliLink).toContain("text-[14px]");
    expect(productTrigger).toContain("[font-family:inherit]");
    expect(cliLink).toContain("[font-family:inherit]");
    expect(productTrigger).toContain("text-[#0B0B0F]");
    expect(cliLink).toContain("text-[#0B0B0F]");
    expect(productTrigger).toContain("inline-flex h-10 shrink-0");
    expect(cliLink).toContain("inline-flex h-10 shrink-0");
    expect(productTrigger).toContain("items-center justify-center gap-1");
    expect(cliLink).toContain("items-center justify-center gap-1");
    expect(html).toContain(
      "relative mx-auto flex h-[72px] max-w-[var(--fk-site-frame-max-width)] items-center gap-1.5 px-[var(--fk-site-gutter)]",
    );
    expect(html).toContain(
      "hidden min-w-0 shrink-0 items-center gap-2.5 ml-6 min-[901px]:flex min-[1180px]:gap-3 min-[1360px]:gap-3.5",
    );
    expect(html).toContain(
      "ml-auto hidden shrink-0 items-center gap-1.5 min-[901px]:flex min-[1180px]:gap-2",
    );
    expect(productTrigger).not.toContain("w-[100px]");
    expect(cliLink).not.toContain("w-[100px]");
    expect(productTrigger).toContain("hover:-translate-y-px");
    expect(cliLink).toContain("hover:-translate-y-px");
    expect(productTrigger).not.toContain("after:");
    expect(cliLink).not.toContain("after:");
    expect(productTrigger).not.toContain("border-[#E7E4EC]");
    expect(resourceTrigger).not.toContain("border-[#E7E4EC]");
    expect(cliLink).not.toContain('aria-hidden="true"');
    expect(languageButton).toContain("cursor-pointer");
    expect(html).toContain("before:w-[256px]");
    expect(html).toContain("before:w-[178px]");
    expect(html).toContain("group-hover/language:pointer-events-auto");
  });

  test("renders active desktop nav without background, underline, or layout shift", () => {
    const modelsHtml = renderHeader(DOCS_URL, "/models");
    const productTrigger = findDesktopGroupTrigger(modelsHtml, "Product");
    const promptsHtml = renderHeader(DOCS_URL, "/prompts");
    const promptsLink = findAnchorByHref(promptsHtml, "/prompts");

    expect(productTrigger).toContain("text-[#050505]");
    expect(productTrigger).toContain("font-bold");
    expect(productTrigger).not.toContain("bg-[#F3EDFF]");
    expect(productTrigger).not.toContain("bg-[#F7F2FF]");
    expect(productTrigger).not.toContain("[font-weight:800]");
    expect(productTrigger).not.toContain("scale-[1.04]");
    expect(productTrigger).not.toContain("after:");

    expect(promptsLink).toContain("text-[#050505]");
    expect(promptsLink).toContain("font-bold");
    expect(promptsLink).not.toContain("bg-[#F3EDFF]");
    expect(promptsLink).not.toContain("bg-[#F7F2FF]");
    expect(promptsLink).not.toContain("[font-weight:800]");
    expect(promptsLink).not.toContain("scale-[1.04]");
    expect(promptsLink).not.toContain("after:");
  });

  test("defaults the public site sign-in link to the console sign-in page", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={DOCS_URL}>
        <SiteShell locale="en" pathname="/">
          <div>body</div>
        </SiteShell>
      </SiteConfigProvider>,
    );

    expect(html).toContain(
      'href="https://console.flatkey.ai/sign-in?lng=en"',
    );
  });

  test("renders baseline structured data for ordinary SiteShell pages", () => {
    const html = renderToStaticMarkup(
      <SiteConfigProvider docsUrl={DOCS_URL}>
        <SiteShell locale="zh" pathname="/models">
          <div>body</div>
        </SiteShell>
      </SiteConfigProvider>,
    );

    expect(html).toContain('data-sitewide-schema="true"');
    expect(html).toContain('type="application/ld+json"');
    expect(html).toContain('"@type":"WebPage"');
    expect(html).toContain('"url":"https://flatkey.ai/zh/models"');
  });
});
