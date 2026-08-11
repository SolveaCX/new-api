import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { PricingPlansGrid } from "./pricing-plans-grid";
import { getPricingPlans } from "./pricing-page";

describe("PricingPlansGrid", () => {
  test("links the enterprise plan to the contact page instead of mounting a form modal", () => {
    const html = renderToStaticMarkup(<PricingPlansGrid plans={getPricingPlans("en")} locale="en" />);

    expect(html).not.toContain("data-tally-src=");
    expect(html).toContain("href=\"/contact\"");
    expect(html).toContain("redirect=%2Fwallet%3Famount%3D10%26currency%3DUSD");
    expect(html).not.toContain("mailto:support@flatkey.ai");
    expect(html).not.toContain("support@flatkey.ai");
  });

  test("styles the localized enterprise price from the plan action", () => {
    const plans = getPricingPlans("pt");
    const contactPlan = plans.find((plan) => plan.action === "contact");
    const html = renderToStaticMarkup(<PricingPlansGrid plans={plans} locale="pt" />);

    expect(contactPlan?.price).not.toBe("Custom");
    expect(html).toContain(
      `<span class="text-4xl font-black tracking-normal text-[#101014] dark:text-white">${contactPlan?.price}</span>`
    );
  });
});
