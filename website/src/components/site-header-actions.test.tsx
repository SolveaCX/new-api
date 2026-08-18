import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SiteHeaderDesktopActions } from "./site-header";

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

    const consoleButton = html.slice(
      html.indexOf('href="https://console.flatkey.ai/dashboard"'),
      html.indexOf(
        "</a>",
        html.indexOf('href="https://console.flatkey.ai/dashboard"'),
      ),
    );
    const contactSalesButton = html.slice(
      html.indexOf('href="/contact"'),
      html.indexOf("</a>", html.indexOf('href="/contact"')),
    );

    expect(html).toContain(">Console<");
    expect(html).toContain(">Contact sales<");
    expect(html.indexOf(">Console<")).toBeLessThan(
      html.indexOf(">Contact sales<"),
    );
    expect(consoleButton).toContain("bg-white");
    expect(contactSalesButton).toContain("bg-[#070707]");
    expect(html).not.toContain(">Start Free<");
  });
});
