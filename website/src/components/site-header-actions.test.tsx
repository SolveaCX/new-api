import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SiteHeaderDesktopActions } from "./site-header";

function anchorForText(html: string, text: string): string {
  const textIndex = html.indexOf(`>${text}<`);
  expect(textIndex).toBeGreaterThanOrEqual(0);

  const anchorStart = html.lastIndexOf("<a", textIndex);
  const anchorEnd = html.indexOf("</a>", textIndex);
  expect(anchorStart).toBeGreaterThanOrEqual(0);
  expect(anchorEnd).toBeGreaterThanOrEqual(0);

  return html.slice(anchorStart, anchorEnd + "</a>".length);
}

describe("SiteHeaderDesktopActions", () => {
  test("renders Contact sales next to Console for authenticated visitors", () => {
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

    expect(html).toContain('href="https://console.flatkey.ai/dashboard"');
    expect(html).toContain(">Console<");
    expect(html).toContain('href="/contact"');
    expect(html).toContain(">Contact sales<");
    expect(html.indexOf(">Console<")).toBeLessThan(
      html.indexOf(">Contact sales<"),
    );
    expect(html).not.toContain(">Start Free<");

    const consoleAnchor = anchorForText(html, "Console");
    const contactSalesAnchor = anchorForText(html, "Contact sales");
    expect(consoleAnchor).toContain("bg-white");
    expect(consoleAnchor).not.toContain("bg-[#070707]");
    expect(contactSalesAnchor).toContain("bg-[#070707]");
    expect(contactSalesAnchor).toContain("text-white");
  });
});
