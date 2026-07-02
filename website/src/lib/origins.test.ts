import { describe, expect, test } from "bun:test";
import { ROUTER_ORIGIN, SITE_ORIGIN, buildConsoleUrl } from "./origins";
import { SITE_ORIGIN as SEO_SITE_ORIGIN, buildMetadata } from "./seo";

describe("buildConsoleUrl", () => {
  test("builds a console URL from an origin with trailing slash", () => {
    expect(buildConsoleUrl("/dashboard", "https://console.flatkey.ai/")).toBe("https://console.flatkey.ai/dashboard");
  });

  test("normalizes paths without a leading slash", () => {
    expect(buildConsoleUrl("dashboard", "https://console.flatkey.ai")).toBe("https://console.flatkey.ai/dashboard");
  });

  test("preserves search params when provided", () => {
    expect(buildConsoleUrl("/sign-up", "https://console.flatkey.ai", "?next=%2Fdashboard&utm_source=home")).toBe(
      "https://console.flatkey.ai/sign-up?next=%2Fdashboard&utm_source=home"
    );
  });
});

describe("ROUTER_ORIGIN", () => {
  test("defaults model invocation examples to the router host", () => {
    expect(ROUTER_ORIGIN).toBe("https://router.flatkey.ai");
  });
});

describe("SITE_ORIGIN", () => {
  test("keeps SEO metadata origin aligned with sitemap origin", () => {
    const metadata = buildMetadata({
      title: "flatkey.ai",
      description: "AI API gateway",
      pathname: "/rankings",
    });

    expect(SEO_SITE_ORIGIN).toBe(SITE_ORIGIN);
    expect(metadata.metadataBase?.toString()).toBe(`${SITE_ORIGIN}/`);
    expect(metadata.alternates?.languages?.en).toBe(`${SITE_ORIGIN}/rankings`);
    expect(metadata.alternates?.languages?.["x-default"]).toBe(`${SITE_ORIGIN}/rankings`);
  });
});
