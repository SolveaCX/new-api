import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ModelLandingPage } from "./model-landing-page";
import {
  DEEPSEEK_CONFIG,
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
    expect(url.searchParams.get("prompt")).toBe(GPT_IMAGE_2_CONFIG.examplePrompt);
    expect(url.searchParams.has("redirect")).toBe(false);
  });

  test("routes the top Get API Key action to the console overview", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(hrefBeforeText(html, "Get API Key")).toBe(
      "https://console.flatkey.ai/dashboard",
    );
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
    expect(html).toContain("Request preview");
    expect(html).toContain("$0.047 / second");
    expect(html).toContain("Capabilities");
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

  test("renders back and playground actions on localized media model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={MINIMAX_H3_CONFIG} locale="zh" liveModels={[]} />
    );

    expect(html).toContain('href="/zh/models"');
    expect(html).toContain("返回模型列表");
    expect(html).toContain("在 Playground 打开");
    expect(html).toContain("获取 API Key");
    expect(html).toContain("https://console.flatkey.ai/playground");
    expect(html).toContain("model=MiniMax-H3");
  });

  test("renders Flatkey sections and related model links on localized Sonilo model pages", () => {
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
    expect(html).not.toContain('id="workbench"');
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

  test("uses a local default cover when a catalog icon key is not a real CDN asset", () => {
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
    expect(relatedSection).toContain("ai-agent-poster.png");
    expect(relatedSection).not.toContain("cdn.jsdelivr.net/npm/@lobehub/icons-static-svg@latest/icons/ai.svg");
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
    expect(relatedSection).toContain("cdn.jsdelivr.net/npm/@lobehub/icons-static-svg@latest/icons/openai.svg");
    expect(relatedSection).not.toContain("related-card-top");
  });

  test("filters related models by modality and uses catalog CDN covers", () => {
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
    expect(relatedSection).toContain("cdn.example.test");
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
    const html = renderToStaticMarkup(
      <ModelLandingPage
        config={DEEPSEEK_CONFIG}
        locale="en"
        liveModels={[]}
        initialHealth={{
          model: "deepseek-v4-flash",
          trend: [{ ts: 1, success_rate: 99.25, avg_ttft_ms: 7619 }],
          summary: {
            model_name: "deepseek-v4-flash",
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
    expect(html).toContain("deepseek-v3.2");
    expect(html).toContain("What changed from the previous generation");
    expect(html).not.toContain("Pricing vs official");

    const performance = html.slice(html.indexOf('id="performance"'), html.indexOf('id="activity"'));
    expect((performance.match(/stroke="#EDEFF2"/g) ?? []).length).toBe(3);
    expect(performance).toContain('fill="#6B38E6"');
  });

  test("shows token input and output prices in the model hero", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(html).toContain('class="model-stat-label">Input /M</div>');
    expect(html).toContain('class="model-stat-label">Output /M</div>');
    expect(html).toContain("$0.83");
    expect(html).toContain("$1.25");
    expect(html).not.toContain('id="pricing"');
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

    expect(imageHtml).toContain("GPT-image-1");
    expect(imageHtml).not.toContain("Pricing vs official");
    expect(imageHtml).toContain("$0.04 / image");
    expect(imageHtml).toContain("$0.06 / image");
    expect(videoHtml).toContain("$0.14 / second");
    expect(videoHtml).not.toContain("$0.14 / request");
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

    expect(html).toContain("$0.32 / request");
    expect(html).toContain("$0.4 / request");
    expect(html).not.toContain("$0.32 / second");
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
    expect(videoHtml).toContain('data-model-kind="video"');
    expect(videoHtml).toContain('id="prompt-library"');
    expect(videoHtml).toContain("Prompt library");
    expect(videoHtml).toContain("/v1/videos");
    expect(videoHtml).toContain('class="preview-media"');
    expect(videoHtml).toContain("/assets/cli/product-reveal.mp4");
    expect(videoHtml).toContain("/assets/cli/ugc-ad-clips.mp4");
    expect(videoHtml).toContain("/assets/cli/localized-variants.mp4");
    expect(videoHtml).toContain("/assets/cli/product-reveal.mp4");
    expect(videoHtml).toContain("campaign-hero.png");
    expect(videoHtml).toContain("storyboard-motion.png");
    expect(videoHtml).toContain("thumbnail-test-set.png");
    expect(videoHtml).not.toContain("prompt-high-speed-action");
    expect(videoHtml).not.toContain("formula car at speed");
    expect(videoHtml).not.toContain("upload-example");
    expect(videoHtml).toContain("Upload or drag and drop");
    expect(videoHtml).toContain("0 / 30");
    expect(videoHtml).toContain("0 / 10");
  });

  test("keeps audio model detail pages free of the public playground", () => {
    const audioHtml = renderToStaticMarkup(
      <ModelLandingPage config={SONILO_VIDEO_TO_MUSIC_CONFIG} locale="en" liveModels={[]} />
    );

    expect(audioHtml).toContain('data-model-kind="audio"');
    expect(audioHtml).not.toContain('id="workbench"');
    expect(audioHtml).not.toContain('href="#workbench"');
    expect(audioHtml).not.toContain("Start generating");
    expect(audioHtml).not.toContain('id="prompt-library"');
    expect(audioHtml).not.toContain("Prompt library");
  });
});
