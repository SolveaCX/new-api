import { describe, expect, test } from "bun:test";
import { buildMetadata } from "./seo";

describe("explicit empty hreflang sets", () => {
  test("does not invent x-default when a caller disables alternates", () => {
    for (const noIndex of [false, true]) {
      const metadata = buildMetadata({title: "Careers", description: "Careers", pathname: "/careers", locale: "de", locales: [], noIndex});
      expect(metadata.alternates).toEqual({canonical: "https://flatkey.ai/de/careers"});
      expect(metadata.robots).toEqual({index: !noIndex, follow: !noIndex});
    }
  });
  test("preserves nonempty language clusters and their x-default", () => {
    const metadata = buildMetadata({title: "Careers", description: "Careers", pathname: "/careers", locale: "zh", locales: ["en", "zh"]});
    expect(metadata.alternates?.languages).toEqual({"en-US": "https://flatkey.ai/careers", "zh-CN": "https://flatkey.ai/zh/careers", "x-default": "https://flatkey.ai/careers"});
    expect(metadata.robots).toEqual({index: true, follow: true});
  });
  test("default language discovery is unchanged", () => {
    const metadata = buildMetadata({title: "Pricing", description: "Pricing", pathname: "/pricing"});
    expect(Object.keys(metadata.alternates?.languages ?? {})).toHaveLength(10);
    expect(metadata.alternates?.languages?.["x-default"]).toBe("https://flatkey.ai/pricing");
    expect(metadata.alternates?.languages).not.toHaveProperty("id-ID");
  });
});
