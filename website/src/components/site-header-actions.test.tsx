import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SiteHeaderDesktopActions } from "./site-header";

function anchorMarkupForHref(html: string, href: string) {
  const hrefIndex = html.indexOf(`href="${href}"`);
  expect(hrefIndex).toBeGreaterThanOrEqual(0);

  const anchorStart = html.lastIndexOf("<a", hrefIndex);
  const anchorEnd = html.indexOf("</a>", hrefIndex);
  expect(anchorStart).toBeGreaterThanOrEqual(0);
  expect(anchorEnd).toBeGreaterThanOrEqual(0);

  return html.slice(anchorStart, anchorEnd);
}

describe("SiteHeaderDesktopActions", () => {
  test("renders authenticated Console as secondary and Contact sales as primary", () => {
    const html = renderToStaticMarkup(
      <SiteHeaderDesktopActions
        accountHref="https://console.flatkey.ai/dashboard"
        accountLabel="Console"
        contactSalesHref="/contact"
        contactSalesLabel="Contact sales"
        consoleSessionActive={true}
        signUpHref="https://console.flatkey.ai/sign-up?lng=en"
        startFreeLabel="Start Free"
      />,
    );
    const consoleButton = anchorMarkupForHref(
      html,
      "https://console.flatkey.ai/dashboard",
    );
    const contactSalesButton = anchorMarkupForHref(html, "/contact");

    expect(html).toContain('href="https://console.flatkey.ai/dashboard"');
    expect(html).toContain(">Console<");
    expect(html).toContain('href="/contact"');
    expect(html).toContain(">Contact sales<");
    expect(html.indexOf(">Console<")).toBeLessThan(
      html.indexOf(">Contact sales<"),
    );
    expect(consoleButton).toContain("bg-white");
    expect(contactSalesButton).toContain("bg-[#070707]");
    expect(html).not.toContain(">Start Free<");
  });
});
