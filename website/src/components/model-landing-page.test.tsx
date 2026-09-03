import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ModelLandingPage } from "./model-landing-page";
import {
  GPT_CONFIG,
  GPT_IMAGE_2_CONFIG,
  MINIMAX_H3_CONFIG,
  SONILO_VIDEO_TO_MUSIC_CONFIG,
  SEEDANCE_25_CONFIG,
  SEEDANCE_CONFIG,
  getModelLandingConfigForPricingModel,
} from "@/lib/model-landing";
import type { PricingModel } from "@/lib/pricing";
import type { RankingsData } from "@/lib/rankings-live";
import { getImagePlaygroundExample } from "@/lib/image-prompt-templates";

const gptFamilyModels: PricingModel[] = [
  {
    model_name: "gpt-5.5",
    vendor_name: "OpenAI",
    quota_type: 0,
    model_ratio: 0.5,
    completion_ratio: 8,
    description: "Flagship GPT model for coding and agent workflows.",
    supported_endpoint_types: ["openai"],
    tags: "256K context",
  },
  {
    model_name: "gpt-5",
    vendor_name: "OpenAI",
    quota_type: 0,
    model_ratio: 0.35,
    completion_ratio: 8,
    supported_endpoint_types: ["openai"],
  },
  {
    model_name: "gpt-5-mini",
    vendor_name: "OpenAI",
    quota_type: 0,
    model_ratio: 0.1,
    completion_ratio: 1,
    supported_endpoint_types: ["openai"],
  },
  {
    model_name: "gpt-4o",
    vendor_name: "OpenAI",
    quota_type: 0,
    model_ratio: 0.25,
    completion_ratio: 4,
    supported_endpoint_types: ["openai"],
  },
  {
    model_name: "gpt-4.1",
    vendor_name: "OpenAI",
    quota_type: 0,
    model_ratio: 0.3,
    completion_ratio: 5,
    supported_endpoint_types: ["openai"],
  },
];

const gptRankings: RankingsData = {
  models: [
    { rank: 1, model_name: "gpt-5.5", vendor: "OpenAI", total_tokens: 1200, share: 0.42 },
  ],
  usage: {
    series: ["gpt-5.5"],
    total: 120000,
    days: [
      { label: "Aug 11", total: 1000, values: [1000] },
      { label: "Aug 12", total: 1400, values: [1400] },
      { label: "Aug 13", total: 1800, values: [1800] },
    ],
  },
};

function hrefBeforeText(html: string, text: string): string {
  const textIndex = html.indexOf(`>${text}<`);
  expect(textIndex).toBeGreaterThanOrEqual(0);
  const matches = [...html.slice(0, textIndex).matchAll(/href="([^"]+)"/g)];
  expect(matches.length).toBeGreaterThan(0);
  return matches[matches.length - 1][1].replaceAll("&amp;", "&");
}

describe("ModelLandingPage", () => {
  test("uses the exact configured model as the primary live model", () => {
    const liveModels: PricingModel[] = [
      {
        model_name: "gpt-5-mini",
        vendor_name: "Mini Vendor",
        quota_type: 0,
        model_ratio: 0.1,
        completion_ratio: 1,
      },
      {
        model_name: "gpt-5",
        vendor_name: "Primary Vendor",
        quota_type: 0,
        model_ratio: 0.35,
        completion_ratio: 8,
      },
    ];

    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={liveModels} allModels={liveModels} />
    );

    expect(html).toContain("$0.7");
    expect(html).toContain("$5.6");
    expect(html).not.toContain("$0.2 in / $0.2 out");
  });

  test("opens GPT-image-2 directly in Playground with its model and prompt", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="zh" liveModels={[]} />
    );
    const encodedHref = html.match(/href="([^"]*\/playground\?[^"]*)"/)?.[1];

    expect(encodedHref).toBeDefined();
    const url = new URL(encodedHref!.replaceAll("&amp;", "&"));
    expect(url.pathname).toBe("/playground");
    expect(url.searchParams.get("model")).toBe("gpt-image-2");
    expect(url.searchParams.get("prompt")).toBe(getImagePlaygroundExample("gpt-image-2", "zh")?.prompt);
    expect(url.searchParams.has("redirect")).toBe(false);
  });

  test("routes the top Get started action to the console overview", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(hrefBeforeText(html, "Get started")).toBe(
      "https://console.flatkey.ai/dashboard",
    );
  });

  test("adds a View API jump link beside the top quick-start action", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(hrefBeforeText(html, "View API")).toBe("#api");
    expect(html).toContain('href="https://console.flatkey.ai/dashboard"');
  });

  test("renders Flatkey homepage-style sections for video model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_CONFIG} locale="en" liveModels={[]} />
    );

    for (const id of ["workbench", "health", "related", "faq"]) {
      expect(html).toContain(`id="${id}"`);
    }
    expect(html).toContain("Playground (edit before sign-up)");
    expect(html).toContain("Generator setup");
    expect(html).toContain("Open in Playground");
    expect(html).not.toContain('class="prompt-library-link"');
    expect(html).toContain("Request preview");
    expect(html).toContain("Pricing data unavailable");
    expect(html).not.toContain('<p class="eyebrow">Capabilities</p>');
    expect(html).toContain("Related models");
    expect(html).toContain("Frequently asked questions");
    expect(html).toContain('type="application/ld+json"');
    expect(html).toContain('"@type":"Product"');
    expect(html).toContain('"@type":"FAQPage"');
  });

  test("renders breadcrumbs on media model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_CONFIG} locale="en" liveModels={[]} />
    );

    expect(html).toContain("aria-label=\"Breadcrumb\"");
    expect(html).toContain("href=\"/models\"");
    expect(html).toContain("All models");
    expect(html).toContain("Seedance 2.0");
  });

  test("renders the Seedance 2.5 contract in the request preview", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_25_CONFIG} locale="en" liveModels={[]} />
    );
    const requestPreview = html.replaceAll("&quot;", '"');

    expect(requestPreview).toContain('"model": "seedance-2.5"');
    expect(requestPreview).toContain('"resolution": "720p"');
    expect(requestPreview).toContain('"ratio": "adaptive"');
    expect(requestPreview).toContain('"duration": 5');
    expect(requestPreview).toContain('"generate_audio": true');
    expect(requestPreview).not.toContain('"resolution": "1080p"');
  });

  test("renders MiniMax-H3 fields without the unsupported Seedance audio option", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={MINIMAX_H3_CONFIG} locale="en" liveModels={[]} />
    );
    const requestPreview = html.replaceAll("&quot;", '"');

    expect(requestPreview).toContain('"model": "MiniMax-H3"');
    expect(requestPreview).toContain('"resolution": "768P"');
    expect(requestPreview).toContain('"ratio": "16:9"');
    expect(requestPreview).toContain('"duration": 6');
    expect(requestPreview).toContain('"aigc_watermark": false');
    expect(requestPreview).not.toContain('"generate_audio"');
    expect(html).not.toContain("Generate audio");
  });

  test("renders the documented Seedance video mode selector without a fake request field", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_25_CONFIG} locale="en" liveModels={[]} />
    );
    const requestPreview = html.replaceAll("&quot;", '"');

    expect(html).toContain("data-video-mode-selector");
    expect(html).toContain('data-video-mode-value="text-to-video"');
    expect(html).toContain("data-video-mode-control");
    expect(html).toContain("Video mode");
    expect(html).toContain("Text-to-Video");
    expect(html).toContain("Image-to-Video");
    expect(html).toContain("Reference-to-Video");
    expect(html).toContain("Video Edit");
    expect(html).toContain("Video Extend");
    expect(html).toContain("Not available on this route");
    expect(html).not.toContain(">Seedance</span>");
    const selectorStart = html.indexOf("data-video-mode-selector");
    const promptStart = html.indexOf('class="field prompt-field', selectorStart);
    expect(selectorStart).toBeGreaterThanOrEqual(0);
    expect(promptStart).toBeGreaterThan(selectorStart);
    expect(html.slice(selectorStart, promptStart)).not.toContain("$");
    expect(requestPreview).not.toContain('"video_mode"');
    expect(SEEDANCE_25_CONFIG.generator?.defaultVideoMode).toBe("text-to-video");
    expect(SEEDANCE_25_CONFIG.generator?.videoModes?.filter((option) => option.supported).map((option) => option.value)).toEqual([
      "text-to-video",
      "image-to-video",
      "reference-to-video",
    ]);
  });

  test("localizes the Seedance video mode selector", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_25_CONFIG} locale="zh" liveModels={[]} />
    );

    expect(html).toContain("视频模式");
    expect(html).toContain("Text-to-Video");
    expect(html).toContain("Image-to-Video");
    expect(html).toContain("Reference-to-Video");
    expect(html).toContain("Video Edit");
    expect(html).toContain("Video Extend");
    const selectorStart = html.indexOf("data-video-mode-selector");
    const promptStart = html.indexOf('class="field prompt-field', selectorStart);
    const selectorMarkup = html.slice(selectorStart, promptStart);
    expect(selectorMarkup).not.toContain("文生视频");
    expect(selectorMarkup).not.toContain("图生视频");
    expect(selectorMarkup).not.toContain("参考生视频");
    expect(html).toContain("当前路由不可用");
  });

  test("renders the Seedance 2.5 pricing evidence without a false fixed Product Offer", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_25_CONFIG} locale="en" liveModels={[]} />
    );
    const pricingSection = html.slice(html.indexOf('id="pricing"'), html.indexOf('id="capabilities"'));

    expect(html).toContain('href="#pricing"');
    expect(pricingSection).toContain("Seedance 2.5 API Pricing");
    expect(pricingSection).toContain("Pricing data unavailable");
    expect(pricingSection).toContain("Add credits");
    expect(pricingSection).toContain("$10");
    expect(pricingSection).toContain("$20");
    expect(pricingSection).toContain("$50");
    expect(pricingSection).toContain('href="https://console.flatkey.ai/wallet"');
    const heroStats = html.slice(html.indexOf('class="model-hero-stats"'), html.indexOf('</div></div></div></section>', html.indexOf('class="model-hero-stats"')));
    expect(heroStats).toContain("Pricing data unavailable");
    expect(pricingSection).not.toContain("480p");
    expect(pricingSection).not.toContain("720p");
    // A single Product Offer would imply that $0.14 is the price for every
    // request, which is not true for resolution/duration/reference variants.
    const schema = html.slice(html.indexOf('type="application/ld+json"'), html.indexOf('</script>'));
    expect(schema).not.toContain('"offers"');
  });

  test("renders back and playground actions on localized media model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={MINIMAX_H3_CONFIG} locale="zh" liveModels={[]} />
    );

    expect(html).toContain('href="/zh/models"');
    expect(html).toContain("返回模型列表");
    expect(html).toContain("在 Playground 打开");
    expect(html).toContain("开始使用");
    expect(html).toContain("https://console.flatkey.ai/playground");
    expect(html).toContain("model=MiniMax-H3");
  });

  test("renders Flatkey sections and related model links on localized Sonilo model pages without a playground", () => {
    const sonilo: PricingModel = {
      model_name: "sonilo-video-to-music",
      vendor_name: "Sonilo",
      description: "Generate production-ready music from any video with synchronized timing.",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.009,
      completion_ratio: 0,
      supported_endpoint_types: ["video-to-music"],
    };
    const relatedModels: PricingModel[] = [
      sonilo,
      {
        model_name: "gpt-image-2",
        vendor_name: "OpenAI",
        quota_type: 1,
        model_ratio: 0,
        model_price: 0.04,
        completion_ratio: 0,
        supported_endpoint_types: ["image-generation"],
      },
      {
        model_name: "seedance-2.0-pro",
        vendor_name: "ByteDance",
        quota_type: 1,
        model_ratio: 0,
        model_price: 0.047,
        completion_ratio: 0,
        supported_endpoint_types: ["video"],
      },
    ];
    const config = getModelLandingConfigForPricingModel(sonilo);
    const html = renderToStaticMarkup(
      <ModelLandingPage config={config} locale="zh" liveModels={[sonilo]} allModels={relatedModels} />
    );

    expect(html).toContain("供应商");
    expect(html).toContain("价格");
    expect(html).toContain("性能");
    expect(html).toContain("可用性");
    expect(html).toContain("/ 请求");
    expect(html).not.toContain("实时模型健康");
    expect(html).not.toContain("生成器配置");
    expect(html).not.toContain("Playground（注册前可编辑）");
    expect(html).not.toContain("在 Playground 打开");
    expect(html).toContain("请求预览");
    expect(html).toContain("常见问题");
    expect(html).toContain("sonilo-video-to-music 可通过 Flatkey 使用");
    expect(html).not.toContain("Generate production-ready music from any video with synchronized timing.");
    // Related models are restricted to the current modality. Sonilo is an
    // audio model, so the image and video entries in this mixed fixture must
    // not leak into its related section.
    expect(html).not.toContain("继续浏览 Flatkey");
    expect(html).not.toContain('href="/zh/models/gpt-image-2"');
    expect(html).not.toContain('href="/zh/models/seedance-api"');
    expect(html).toContain('"url":"https://flatkey.ai/zh/models/sonilo-video-to-music"');
  });

  test("uses a large model logo when a catalog icon key is not available", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage
        config={SONILO_VIDEO_TO_MUSIC_CONFIG}
        locale="zh"
        liveModels={[{
          model_name: "sonilo-video-to-music",
          vendor_name: "Sonilo",
          quota_type: 1,
          model_ratio: 0,
          model_price: 0.009,
          completion_ratio: 0,
          supported_endpoint_types: ["video-to-music"],
        }]}
        allModels={[
          {
            model_name: "sonilo-video-to-music",
            vendor_name: "Sonilo",
            quota_type: 1,
            model_ratio: 0,
            model_price: 0.009,
            completion_ratio: 0,
            supported_endpoint_types: ["video-to-music"],
          },
          {
            model_name: "eleven_sound_v1",
            vendor_name: "AI",
            icon: "ai",
            quota_type: 1,
            model_ratio: 0,
            model_price: 0.01,
            completion_ratio: 0,
            supported_endpoint_types: ["audio"],
          },
        ]}
      />
    );

    const relatedSection = html.slice(html.indexOf('id="related"'), html.indexOf('id="faq"'));
    expect(relatedSection).toContain("related-card-logo-mark");
    expect(relatedSection).toContain("related-card-logo-bare");
    expect(relatedSection).toContain("related-card-brand");
    expect(relatedSection).toContain("flatkey-lockup-light.svg");
    expect(relatedSection).toContain('style="width:150px;height:150px');
    expect(relatedSection).toContain('aria-label="Model"');
    expect(relatedSection).not.toContain("related-arrow");
    expect(relatedSection).not.toContain("ai-agent-poster.png");
    expect(relatedSection).not.toContain("related-image");
  });

  test("renders GPT-series related model internal links on GPT model pages", () => {
    const gpt55Config = { ...GPT_CONFIG, slug: "gpt-5.5", displayName: "gpt-5.5", modelId: "gpt-5.5" };
    const html = renderToStaticMarkup(
      <ModelLandingPage
        config={gpt55Config}
        locale="en"
        liveModels={[gptFamilyModels[0]]}
        allModels={gptFamilyModels}
        rankings={gptRankings}
      />
    );

    expect(html).toContain("More models from OpenAI");
    expect(html).toContain('href="/models/gpt-5"');
    expect(html).toContain('href="/models/gpt-5-mini"');
    expect(html).toContain('href="/models/gpt-4o"');
    expect(html).toContain("120K");
    const relatedSection = html.slice(html.indexOf('id="related"'), html.indexOf('id="faq"'));
    expect(relatedSection).toContain('class="model-section related"');
    expect(relatedSection).toContain('class="related-grid"');
    expect(relatedSection).toContain("/assets/logos/openai.svg");
    expect(relatedSection).not.toContain("related-image");
    expect(relatedSection).toContain("related-card-top");
  });

  test("filters related models by modality and uses the corresponding model logo", () => {
    const catalog: PricingModel[] = [
      {
        model_name: "seedance-2.5",
        vendor_name: "ByteDance",
        quota_type: 1,
        model_ratio: 0,
        model_price: 0.14,
        completion_ratio: 0,
        supported_endpoint_types: ["video"],
      },
      {
        model_name: "kling-2.1",
        vendor_name: "Kuaishou",
        icon: "https://cdn.example.test/models/kling-2.1.png",
        quota_type: 1,
        model_ratio: 0,
        model_price: 0.08,
        completion_ratio: 0,
        supported_endpoint_types: ["video"],
      },
      {
        model_name: "gpt-5.5",
        vendor_name: "OpenAI",
        icon: "https://cdn.example.test/models/gpt-5.5.png",
        quota_type: 0,
        model_ratio: 0.5,
        completion_ratio: 8,
        supported_endpoint_types: ["openai"],
      },
    ];
    const html = renderToStaticMarkup(
      <ModelLandingPage
        config={SEEDANCE_25_CONFIG}
        locale="en"
        liveModels={[catalog[0]]}
        allModels={catalog}
      />
    );
    const relatedSection = html.slice(html.indexOf('id="related"'), html.indexOf('id="faq"'));

    expect(relatedSection).toContain("kling-2.1");
    expect(relatedSection).toContain("/assets/logos/kuaishou.svg");
    expect(relatedSection).not.toContain("cdn.example.test");
    expect(relatedSection).not.toContain("related-image");
    expect(relatedSection).not.toContain("gpt-5.5");
  });

  test("renders breadcrumbs on text model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(html).toContain("aria-label=\"Breadcrumb\"");
    expect(html).toContain("href=\"/models\"");
    expect(html).toContain("All models");
    expect(html).toContain("gpt-5");
    expect(html).toContain('type="application/ld+json"');
    expect(html).toContain('"url":"https://flatkey.ai/models/gpt-api"');
    expect(html).not.toContain('id="workbench"');
  });

  test("renders shared text model sections without media-only blocks", () => {
    const deepseekV4Pro: PricingModel = {
      model_name: "deepseek-v4-pro",
      vendor_name: "DeepSeek",
      quota_type: 0,
      model_ratio: 0.66,
      completion_ratio: 3,
      cache_ratio: 0.033333333333,
      supported_endpoint_types: ["openai", "anthropic"],
      display_pricing: {
        billing_kind: "token",
        prices: {
          input: { configured: 1.65, plg: 1.32 },
          output: { configured: 4.95, plg: 3.96 },
          cache: { configured: 0.055, plg: 0.044 },
        },
      },
    };
    const deepseekV4ProConfig = getModelLandingConfigForPricingModel(deepseekV4Pro);
    const html = renderToStaticMarkup(
      <ModelLandingPage
        config={deepseekV4ProConfig}
        locale="en"
        liveModels={[deepseekV4Pro]}
        initialHealth={{
          model: "deepseek-v4-pro",
          trend: [{ ts: 1, success_rate: 99.25, avg_ttft_ms: 7619 }],
          summary: {
            model_name: "deepseek-v4-pro",
            avg_latency_ms: 22419,
            avg_ttft_ms: 7619,
            success_rate: 99.25,
            avg_tps: 71.32,
            request_count: 189734,
          },
        }}
      />
    );

    expect(html).toContain('data-model-kind="text"');
    expect(html).not.toContain('id="parameters"');
    expect(html).toContain('id="performance"');
    expect(html).toContain('class="eyebrow">Performance</p>');
    expect(html).not.toContain("Live model health");
    expect(html).toContain('id="activity"');
    expect(html).toContain('class="eyebrow">Activity</p>');
    expect(html).not.toContain('class="eyebrow">Usage</p>');
    expect(html).toContain("99.25%");
    expect(html).toContain("190K");
    expect(html).toContain("7.62s");
    expect(html).toContain('id="api"');
    expect(html).not.toContain('id="workbench"');
    expect(html).not.toContain("Prompt library");
    expect(html).toContain("deepseek-v4-pro");
    expect(html).toContain("DeepSeek V4 Pro vs V4 Flash");
    expect(html).toContain("/v1/chat/completions");
    expect(html).toContain("/v1/messages");
    expect(html).not.toContain("What changed from the previous generation");
    expect(html).not.toContain("Pricing vs official");

    const performance = html.slice(html.indexOf('id="performance"'), html.indexOf('id="activity"'));
    expect((performance.match(/stroke="#EDEFF2"/g) ?? []).length).toBe(3);
    expect(performance).toContain('fill="#6B38E6"');
  });

  test("shows token input and output prices in the model hero", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(html).toContain("Pricing data unavailable");
    expect(html).not.toContain('class="model-stat-label">Input /M</div>');
    expect(html).not.toContain('id="pricing"');
  });

  test("shows the live cache price in the model hero with the official price crossed out", () => {
    const model: PricingModel = {
      model_name: "gpt-5",
      vendor_name: "OpenAI",
      quota_type: 0,
      model_ratio: 0.5,
      completion_ratio: 2,
      display_pricing: {
        billing_kind: "token",
        prices: {
          input: { configured: 1, plg: 0.8 },
          cache: { configured: 0.2, plg: 0.1 },
          output: { configured: 2, plg: 1.6 },
        },
      },
    };
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[model]} allModels={[model]} />
    );

    expect(html).toContain('class="model-stat-label">Cache /M</div>');
    expect(html).toContain("$0.1");
    expect(html).toContain("<s>$0.2</s>");
    expect(html).not.toContain("Promotional pricing");
    expect(html).not.toContain("活动价格");
    expect(html.indexOf('class="model-stat-reference">Reference price: <s>$0.2</s>'))
      .toBeLessThan(html.indexOf('<div class="model-stat-value">$0.1</div>'));
  });

  test("places the official hero price before the Flatkey price", () => {
    const model: PricingModel = {
      model_name: "veo-3.1-generate-preview",
      vendor_name: "Google",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.4,
      completion_ratio: 0,
      supported_endpoint_types: ["video"],
      display_pricing: {
        billing_kind: "request",
        prices: { request: { configured: 0.4, plg: 0.32 } },
      },
    };
    const html = renderToStaticMarkup(
      <ModelLandingPage
        config={getModelLandingConfigForPricingModel(model)}
        locale="en"
        liveModels={[model]}
        allModels={[model]}
      />
    );
    const heroStats = html.slice(html.indexOf('class="model-hero-stats"'), html.indexOf('class="model-anchor-bar"'));

    expect(heroStats).toContain("$0.320 / request");
    expect(heroStats).toContain("$0.400 / request");
  });

  test("keeps image model hero prices aligned with the model directory", () => {
    const imageModel: PricingModel = {
      model_name: "gpt-image-2",
      vendor_name: "OpenAI",
      quota_type: 0,
      model_ratio: 2.5,
      completion_ratio: 6,
      display_pricing: {
        billing_kind: "token",
        prices: {
          input: { configured: 5, plg: 4 },
          output: { configured: 30, plg: 24 },
          image: { configured: 8, plg: 6.4 },
        },
      },
    };
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="en" liveModels={[imageModel]} allModels={[imageModel]} />
    );

    expect(html).toContain("Input /M");
    expect(html).toContain("$4");
    expect(html).toContain("$24");
    expect(html).not.toContain("$6.4 / image");
  });

  test("uses previous-generation capability comparison and type-specific media pricing", () => {
    const imageHtml = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="en" liveModels={[]} />
    );
    const videoHtml = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_25_CONFIG} locale="en" liveModels={[]} />
    );

    // The image landing now uses the target-specific comparison copy. Keep
    // this assertion focused on the previous-generation comparison while
    // matching the editorial title/casing used by GPT Image 2.
    expect(imageHtml).toContain("GPT Image 1");
    expect(imageHtml).toContain("GPT Image 2");
    expect(imageHtml).toContain("GPT Image 1");
    expect(imageHtml).not.toContain("Pricing vs official");
    // GPT Image 2 now uses the audited token-dimension catalog rows instead
    // of the legacy generic per-image examples.
    expect(imageHtml).toContain("Pricing data unavailable");
    expect(imageHtml).toContain("pricing-conversion-grid");
    expect(imageHtml).toContain('href="https://console.flatkey.ai/wallet"');
    expect(imageHtml).toContain("$10");
    expect(imageHtml).toContain("$20");
    expect(imageHtml).toContain("$50");
    expect(imageHtml).toContain('class="prompt-control"');
    const imageCapabilities = imageHtml.slice(imageHtml.indexOf('id="capabilities"'), imageHtml.indexOf('id="comparison"'));
    expect((imageCapabilities.match(/class="capability-card"/g) ?? []).length).toBe(4);
    expect(imageCapabilities).toContain("Text-to-image creation");
    expect(imageCapabilities).not.toContain("Image count");
    expect(videoHtml).toContain("Pricing data unavailable");
    const videoPricing = videoHtml.slice(videoHtml.indexOf('id="pricing"'), videoHtml.indexOf('id="capabilities"'));
    expect(videoPricing).toContain("pricing-conversion-grid");
    expect(videoPricing).not.toContain("480p");
    expect(videoPricing).not.toContain("720p");
    expect(videoHtml).not.toContain("$0.14 / second");
    expect(videoHtml).not.toContain("10% below list price");
  });

  test("keeps request-priced media pages aligned with the model directory", () => {
    const veoModel: PricingModel = {
      model_name: "veo-3.1-generate-preview",
      vendor_name: "Google",
      quota_type: 1,
      model_ratio: 0,
      completion_ratio: 0,
      model_price: 0.4,
      supported_endpoint_types: ["video"],
      display_pricing: {
        billing_kind: "request",
        prices: { request: { configured: 0.4, plg: 0.32 } },
      },
    };
    const config = getModelLandingConfigForPricingModel(veoModel);
    const html = renderToStaticMarkup(
      <ModelLandingPage config={config} locale="en" liveModels={[veoModel]} allModels={[veoModel]} />
    );

    expect(html).toContain("$0.320 / request");
    expect(html).toContain("$0.400 / request");
    expect(html).not.toContain("$0.32 / second");
  });

  test("uses the live effective input rate for schema offers and omits request offers", () => {
    const tokenHtml = renderToStaticMarkup(
      <ModelLandingPage
        config={GPT_CONFIG}
        locale="en"
        groupRatio={{ plg: 0.9 }}
        groupModelRatio={{ plg: { "gpt-5": 0.5 } }}
        liveModels={[{
          model_name: "gpt-5",
          vendor_name: "OpenAI",
          quota_type: 0,
          model_ratio: 0.5,
          completion_ratio: 2,
          enable_groups: ["plg"],
          display_pricing: {
            billing_kind: "token",
            prices: {
              input: { configured: 1, plg: 0.5 },
              output: { configured: 2, plg: 1 },
            },
          },
        }]}
      />,
    );
    const tokenSchema = tokenHtml.slice(tokenHtml.indexOf('type="application/ld+json"'));
    expect(tokenSchema).toContain('"price":0.5');

    const requestHtml = renderToStaticMarkup(
      <ModelLandingPage
        config={GPT_CONFIG}
        locale="en"
        liveModels={[{
          model_name: "request-model",
          vendor_name: "Provider",
          quota_type: 1,
          model_ratio: 0,
          completion_ratio: 1,
          model_price: 0.04,
          display_pricing: {
            billing_kind: "request",
            prices: { request: { configured: 0.04, plg: 0.02 } },
          },
        }]}
      />,
    );
    const requestSchema = requestHtml.slice(requestHtml.indexOf('type="application/ld+json"'));
    expect(requestSchema).not.toContain('"offers"');
  });

  test("renders media-only prompt libraries for image and video models", () => {
    const imageHtml = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="en" liveModels={[]} />
    );
    const videoHtml = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_25_CONFIG} locale="en" liveModels={[]} />
    );

    expect(imageHtml).toContain('data-model-kind="image"');
    expect(imageHtml).toContain('id="prompt-library"');
    expect(imageHtml).toContain("Prompt library");
    expect(imageHtml).toContain("Reference Images");
    expect(imageHtml).toContain("/v1/images/generations");
    expect(imageHtml).toContain('type="number" min="1" max="10"');
    expect(imageHtml).toContain('class="h-9 w-full min-w-0 appearance-none');
    expect(imageHtml).toContain("resize-y");
    expect((imageHtml.match(/class="prompt-card"/g) ?? []).length).toBe(6);
    expect(videoHtml).toContain('data-model-kind="video"');
    expect(videoHtml).toContain('id="prompt-library"');
    expect(videoHtml).toContain("Prompt library");
    expect(videoHtml).toContain("/v1/videos");
    expect(videoHtml).toContain('class="preview-media"');
    expect(videoHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-01/video-profession-01-manga-seedance-2-5.mp4");
    expect(videoHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-02/video-profession-02-seedance-2-5-tvc-kettle.mp4");
    expect(videoHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-03/video-profession-03-seedance-2-5-sci-fi-set-extension.mp4");
    expect(videoHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-04/video-profession-04-seedance-2-5-open-world-trailer.mp4");
    expect(videoHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-05/video-profession-05-seedance-2-5-space-science-explainer.mp4");
    expect(videoHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-06/video-profession-06-seedance-2-5-stage-projection.mp4");
    const promptLibraryHtml = videoHtml.slice(videoHtml.indexOf('id="prompt-library"'));
    expect(promptLibraryHtml).toContain('preload="auto"');
    expect(promptLibraryHtml).toContain('preload="none"');
    expect(promptLibraryHtml).not.toContain('poster="/assets/model-examples/product-macro.png"');
    expect(videoHtml).not.toContain("/assets/cli/ugc-ad-clips.mp4");
    expect(videoHtml).not.toContain("/assets/cli/localized-variants.mp4");
    expect(videoHtml).not.toContain("prompt-high-speed-action");
    expect(videoHtml).not.toContain("formula car at speed");
    expect(videoHtml).not.toContain("upload-example");
    expect(videoHtml).toContain("Upload or drag and drop");
    expect(videoHtml).toContain("0 / 30");
    expect(videoHtml).toContain("0 / 10");
    expect((videoHtml.match(/class="prompt-card"/g) ?? []).length).toBe(6);
  });

  test("binds MiniMax-H3's six profession cards to the reviewed Seedance media", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={MINIMAX_H3_CONFIG} locale="zh" liveModels={[]} />
    );
    const promptLibraryHtml = html.slice(html.indexOf('id="prompt-library"'));

    expect((promptLibraryHtml.match(/class="prompt-card"/g) ?? []).length).toBe(6);
    expect(promptLibraryHtml).toContain("微短剧与漫剧创作者");
    expect(promptLibraryHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-01/minimax-h3-seedance-2-0.mp4");
    expect(promptLibraryHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-01/minimax-h3-seedance-2-0.jpg");
    expect(promptLibraryHtml).toContain("https://cdn.shulex-voc.com/flatkey/model-showcase/video-profession-06/minimax-h3-seedance-music-visual-art.mp4");
    expect(promptLibraryHtml).not.toContain("/assets/cli/product-reveal.mp4");
  });

  test("sends prompt-library make-one-like-this actions to the console overview", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="zh" liveModels={[]} />
    );

    expect(hrefBeforeText(html, "做一个类似的")).toBe(
      "https://console.flatkey.ai/dashboard/overview",
    );
    const promptLibraryHtml = html.slice(html.indexOf('id="prompt-library"'));
    expect((promptLibraryHtml.match(/href="https:\/\/console\.flatkey\.ai\/dashboard\/overview"/g) ?? []).length).toBe(6);
    expect(promptLibraryHtml).not.toContain('href="#workbench"');
  });

  test("keeps audio model pages free of the public playground", () => {
    const audioHtml = renderToStaticMarkup(
      <ModelLandingPage config={SONILO_VIDEO_TO_MUSIC_CONFIG} locale="en" liveModels={[]} />
    );
    const audioPageHtml = audioHtml.slice(audioHtml.indexOf("<main"), audioHtml.indexOf("</main>"));

    expect(audioPageHtml).toContain('data-model-kind="audio"');
    expect(audioPageHtml).not.toContain('id="workbench"');
    expect(audioPageHtml).not.toContain('href="#workbench"');
    expect(audioPageHtml).not.toContain("Playground");
    expect(audioPageHtml).not.toContain("Start generating");
    expect(audioPageHtml).not.toContain('id="prompt-library"');
    expect(audioPageHtml).not.toContain("Prompt library");
  });

  test("keeps generic audio model pages free of playground links and copy", () => {
    const audioModels: PricingModel[] = [
      {
        model_name: "eleven_sound_v1",
        vendor_name: "ElevenLabs",
        quota_type: 1,
        model_ratio: 0,
        model_price: 0.01,
        completion_ratio: 0,
        supported_endpoint_types: ["audio"],
      },
      {
        model_name: "gemini-3.1-flash-tts-preview",
        vendor_name: "Google",
        quota_type: 1,
        model_ratio: 0,
        model_price: 0.01,
        completion_ratio: 0,
        supported_endpoint_types: ["audio"],
      },
    ];

    for (const model of audioModels) {
      const config = getModelLandingConfigForPricingModel(model);
      const html = renderToStaticMarkup(
        <ModelLandingPage config={config} locale="en" liveModels={[model]} allModels={[model]} />
      );
      const audioPageHtml = html.slice(html.indexOf("<main"), html.indexOf("</main>"));

      expect(config.generator?.kind).toBe("audio");
      expect(audioPageHtml).toContain('data-model-kind="audio"');
      expect(audioPageHtml).toContain('id="api"');
      expect(audioPageHtml).toContain('id="pricing"');
      expect(audioPageHtml).not.toContain('id="workbench"');
      expect(audioPageHtml).not.toContain('id="prompt-library"');
      expect(audioPageHtml).not.toMatch(/playground/i);
      expect(audioPageHtml).not.toContain("Try a prompt");
      expect(audioPageHtml).not.toContain("Start generating");
      expect(audioPageHtml).not.toContain('href="#workbench"');
      expect(audioPageHtml).not.toContain("/playground");
    }
  });
});
