import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

mock.module("server-only", () => ({}));

describe("OnlinePricingPage", () => {
  test("renders subscribe and contact sales actions above the plan prices", async () => {
    const { OnlinePricingPage } = await import("./online-pricing-page");
    const html = renderToStaticMarkup(<OnlinePricingPage locale="en" />);

    const goCta = html.indexOf("Subscribe");
    const goPrice = html.indexOf("$10/mo");
    const enterpriseCta = html.indexOf("Contact sales");
    const enterpriseCustom = html.indexOf(">Custom<");

    expect(goCta).toBeGreaterThanOrEqual(0);
    expect(goPrice).toBeGreaterThan(goCta);
    expect(enterpriseCta).toBeGreaterThanOrEqual(0);
    expect(enterpriseCustom).toBeGreaterThan(enterpriseCta);
  });
});
