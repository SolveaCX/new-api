import { describe, expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { PricingExplorer, getModelsDirectoryTableCopy, modelsHref, normalizePricingSearch } from "./pricing-explorer";

describe("model directory filter links", () => {
  test("builds crawlable /models filter URLs", () => {
    expect(modelsHref("en", { vendor: "Qwen", pricing: "token", purpose: "text", endpoint: "openai-chat" })).toBe(
      "/models?vendor=Qwen&pricing=token&purpose=text"
    );
    expect(modelsHref("zh", { vendor: "Qwen", pricing: "request", purpose: "image" })).toBe(
      "/zh/models?vendor=Qwen&pricing=request&purpose=image"
    );
  });

  test("omits all filters from model directory URLs", () => {
    expect(modelsHref("en", { vendor: "all", pricing: "all", endpoint: "all", purpose: "all", q: " " })).toBe("/models");
    expect(modelsHref("en", { vendor: "Qwen", q: "coder" })).toBe("/models?vendor=Qwen");
  });

  test("normalizes old quota filters into pricing filters", () => {
    expect(normalizePricingSearch({ vendor: " Qwen ", quota: " token " })).toEqual({
      vendor: "Qwen",
      pricing: "token",
      endpoint: "all",
      purpose: "all",
    });
  });

  test("renders the models controls as a header-aligned top workspace", () => {
    const html = renderToStaticMarkup(
      createElement(PricingExplorer, {
        locale: "en",
        models: [
          {
            model_name: "gpt-5-mini",
            vendor_name: "OpenAI",
            quota_type: 0,
            model_ratio: 0.25,
            completion_ratio: 4,
            supported_endpoint_types: ["openai-chat"],
            tags: "tools vision",
          },
          {
            model_name: "image-fast",
            vendor_name: "ImageAI",
            quota_type: 1,
            model_ratio: 0,
            completion_ratio: 1,
            model_price: 0.04,
            supported_endpoint_types: ["openai-images"],
            tags: "image",
          },
        ],
        vendors: [
          { id: 1, name: "OpenAI" },
          { id: 2, name: "ImageAI" },
        ],
        groupRatio: { plg: 0.8 },
        usableGroup: {},
        endpointMap: {},
        autoGroups: [],
      })
    );

    expect(html).toContain("Browse 100+ models");
    expect(html).toContain("Sort");
    expect(html).toContain('data-local-sort-dropdown="true"');
    expect(html).toContain('data-local-reset-filters="true"');
    expect(html).toContain('data-local-view-toggle="true"');
    expect(html).toContain('aria-label="List view"');
    expect(html).toContain('aria-label="Card view"');
    expect(html).toContain("<table");
    expect(html).not.toContain('data-local-models-card-grid="true"');
    expect(html).toContain('aria-label="List view" aria-pressed="true"');
    expect(html).toContain('aria-label="Card view" aria-pressed="false"');
    expect(html).toContain("Provider");
    expect(html).toContain('href="/models?vendor=OpenAI"');
    expect(html).toContain("Pricing Type");
    expect(html).toContain("Purpose");
    expect(html).toContain("Text");
    expect(html).toContain("Image");
    expect(html).toContain("File");
    expect(html).toContain("Audio");
    expect(html).toContain("Video");
    expect(html).not.toContain("Endpoint Type");
    expect(html).not.toContain("openai-chat");
    expect(html).not.toContain("<select");
    expect(html).not.toContain('data-local-filters-toggle="true"');
    expect(html).not.toContain('aria-expanded="true"');
    expect(html).not.toContain("Provider A-Z");
    expect(html).not.toContain("Highest input");
    expect(html).not.toContain("Showing 2 of 2");
    expect(html).not.toContain(">2</span> models");
    expect(html).not.toContain("shadow-[0_24px_70px_-58px");
    expect(html).not.toContain("purpose, or tag");
    expect(html).not.toContain("or tag...");
    expect(html).toContain('data-local-models-filter="true"');
    expect(html).toContain('data-local-models-toolbar="true"');
    expect(html).toContain('data-local-controls-row="true"');
    expect(html).toContain('data-local-filter-panel="true"');
    expect(html).toContain('data-local-filter-section="provider"');
    expect(html).toContain('data-local-filter-section="pricing"');
    expect(html).toContain('data-local-filter-section="purpose"');
    expect(html).toContain('data-local-filter-groups="true"');
    expect(html).toContain('data-local-reset-header="true"');
    expect(html).not.toContain('data-local-reset-footer="true"');
    expect(html).toContain('data-local-reset-filters="true"');
    expect(html).toContain("rounded-2xl border border-[#E6E0F0]/80 bg-white/88");
    expect(html).toContain("grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto_auto_auto]");
    expect(html).toContain('space-y-3" data-local-filter-panel="true"');
    expect(html).toContain('space-y-3" data-local-filter-groups="true"');
    expect(html).toContain("inline-flex h-7 max-w-[9.5rem]");
    expect(html).toContain("inline-flex h-7 max-w-full");
    expect(html).not.toContain('hidden=""');
    expect(html).not.toContain("grid gap-3 sm:grid-cols-2");
    expect(html).not.toContain("border border-[#E2DAEF]");
    expect(html).not.toContain("border border-[#EEE8F6]");
    expect(html).not.toContain("border-t border-[#ECE7F3]");
    expect(html).not.toContain("grid gap-2.5 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7");
    expect(html).not.toContain("xl:grid-cols-[minmax(220px,auto)_minmax(0,1fr)_minmax(190px,240px)_auto]");
    expect(html).not.toContain("min-h-[62px]");
    expect(html).not.toContain("border-slate-950 bg-slate-950 text-white");
    expect(html).not.toContain('data-local-toolbar-reset="true"');
    expect(html).not.toContain("sticky top-4 hidden");
    expect(html).not.toContain("xl:grid-cols-[330px_minmax(0,1fr)]");
    expect(html).not.toContain("Refine models by provider, type, and endpoint.");
    expect(html).not.toContain("bg-[#FBFAFE]/78 px-3 py-2.5");
    expect(html).not.toContain("bg-white/97");
    expect(html).not.toContain("shadow-[0_24px_70px_-38px");
  });

  test("localizes the model directory table labels for Indonesian", () => {
    expect(getModelsDirectoryTableCopy("id").colHealth).toBe("Kesehatan");
    expect(getModelsDirectoryTableCopy("id").colCapabilities).toBe("Kapabilitas");
  });

  test("localizes Chinese purpose filter labels", () => {
    const html = renderToStaticMarkup(
      createElement(PricingExplorer, {
        locale: "zh",
        models: [],
        vendors: [],
        groupRatio: {},
        usableGroup: {},
        endpointMap: {},
        autoGroups: [],
      })
    );

    expect(html).toContain(">文本<");
    expect(html).toContain(">图像<");
    expect(html).toContain(">文件<");
    expect(html).toContain(">音频<");
    expect(html).toContain(">视频<");
    expect(html).not.toContain(">Text<");
    expect(html).not.toContain(">Image<");
    expect(html).not.toContain(">File<");
  });

  test("keeps an active deep-linked provider visible outside the top vendor shortlist", () => {
    const models = Array.from({ length: 7 }, (_, index) => ({
      model_name: `model-${index}`,
      vendor_name: index === 6 ? "RareVendor" : `Vendor${index}`,
      quota_type: 0,
      model_ratio: 0.1 + index,
      completion_ratio: 1,
      supported_endpoint_types: ["openai-chat"],
    }));
    const html = renderToStaticMarkup(
      createElement(PricingExplorer, {
        locale: "en",
        models,
        vendors: models.map((model, index) => ({ id: index + 1, name: model.vendor_name })),
        groupRatio: {},
        usableGroup: {},
        endpointMap: {},
        autoGroups: [],
        initialSearch: { vendor: "RareVendor" },
      })
    );

    expect(html).toContain('data-local-provider-filter="RareVendor"');
    expect(html).toContain('href="/models?vendor=RareVendor"');
  });
});
