import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { CollectionDiscountPrices } from "./collection-discount-prices";
import type { PricingModel } from "@/lib/pricing";

describe("collection discount comparisons", () => {
  const model: PricingModel = { model_name: "test", quota_type: 0, model_ratio: 1, completion_ratio: 1, display_pricing: { billing_kind: "token", prices: { input: { configured: 5, plg: 3 }, output: { configured: 20, plg: 20 } } } };
  test("shows like-for-like public and configured prices without claiming an official list price", () => {
    const html = renderToStaticMarkup(<CollectionDiscountPrices model={model} locale="en" />);
    expect(html).toContain("Configured reference");
    expect(html).toContain("Current public price");
    expect(html).toContain("$5");
    expect(html).toContain("$3");
    expect(html).not.toContain("Output");
    expect(html).not.toContain("official");
  });
  test("does not manufacture savings when no comparable price exists", () => {
    expect(renderToStaticMarkup(<CollectionDiscountPrices model={{ ...model, display_pricing: undefined }} locale="en" />)).toBe("");
  });
  test("localizes discount labels", () => {
    expect(renderToStaticMarkup(<CollectionDiscountPrices model={model} locale="zh" />)).toContain("配置参考价");
    expect(renderToStaticMarkup(<CollectionDiscountPrices model={model} locale="id" />)).toContain("Harga publik saat ini");
  });
});
