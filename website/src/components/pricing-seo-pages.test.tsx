import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ModelSeoPage, VendorSeoPage } from "./pricing-seo-pages";
import type { PricingSeoModel, PricingSeoVendor } from "@/lib/pricing-seo";

const openAiVendor: PricingSeoVendor = {
  id: 1,
  name: "OpenAI",
  slug: "openai",
  displayName: "OpenAI",
  icon: "openai",
  models: [
    {
      model_name: "gpt-4o-mini",
      model_slug: "gpt-4o-mini",
      vendor_id: 1,
      vendor_name: "OpenAI",
      vendor_slug: "openai",
      quota_type: 0,
      model_ratio: 0.15,
      completion_ratio: 4,
      supported_endpoint_types: ["openai"],
      description: "Fast multimodal model for production API usage.",
      enable_groups: ["Standard"],
      group_ratio: { Standard: 1 },
    },
  ],
};

const modelEntry: PricingSeoModel = {
  slug: "gpt-4o-mini",
  model: openAiVendor.models[0],
  vendor: openAiVendor,
};

describe("pricing SEO pages", () => {
  test("keeps vendor SEO copy and links on the existing model directory surface", () => {
    const html = renderToStaticMarkup(<VendorSeoPage locale="en" entry={openAiVendor} />);

    expect(html).toContain("model-square-page");
    expect(html).toContain("Compare OpenAI models on flatkey.ai");
    expect(html).toContain("/pricing?vendor=openai");
    expect(html).not.toContain("/models/gpt-4o-mini");
    expect(html).not.toContain("bg-[linear-gradient(180deg,#f7f4ff_0%,#ffffff_44%,#f3f8ff_100%)]");
  });

  test("keeps model SEO copy and pricing links on the existing model directory surface", () => {
    const html = renderToStaticMarkup(<ModelSeoPage locale="en" entry={modelEntry} />);

    expect(html).toContain("model-square-page");
    expect(html).toContain("Use gpt-4o-mini from OpenAI through flatkey.ai");
    expect(html).toContain("/pricing?vendor=openai");
    expect(html).toContain("Input");
    expect(html).not.toContain("bg-[linear-gradient(180deg,#f7f4ff_0%,#ffffff_44%,#f3f8ff_100%)]");
  });
});
