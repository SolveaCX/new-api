import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ModelLandingPage, animateScrollToTop, buildDraftFallbackRunHref } from "./model-landing-page";
import {
  GPT_CONFIG,
  GPT_IMAGE_2_CONFIG,
  MINIMAX_H3_CONFIG,
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

function sectionHtml(html: string, id: string, nextId: string): string {
  const start = html.indexOf(`<section id="${id}"`);
  const end = html.indexOf(`<section id="${nextId}"`, start + 1);
  expect(start).toBeGreaterThanOrEqual(0);
  expect(end).toBeGreaterThan(start);
  return html.slice(start, end);
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

  test("routes GPT-image-2 actions through console signup with a compact handoff entry", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="zh" liveModels={[]} />
    );
    const encodedHref = html.match(/href="([^"]*\/sign-up\?[^"]*redirect=[^"]*)"/)?.[1];

    expect(encodedHref).toBeDefined();
    const url = new URL(encodedHref!.replaceAll("&amp;", "&"));
    expect(url.origin).toBe("https://console.flatkey.ai");
    expect(url.pathname).toBe("/sign-up");
    expect(url.searchParams.get("lng")).toBe("zh");
    const redirect = new URL(url.searchParams.get("redirect")!, url.origin);
    expect(redirect.pathname).toBe("/playground");
    expect(redirect.searchParams.get("source")).toBe("model_landing");
    expect(redirect.searchParams.get("model")).toBe("gpt-image-2");
    expect(redirect.searchParams.get("media_kind")).toBe("image");
    expect(redirect.searchParams.get("prompt")).toBeNull();
    expect(redirect.searchParams.get("request")).toBeNull();
    expect(redirect.searchParams.get("size")).toBeNull();
  });

  test("builds a bounded draft fallback URL when model handoff creation fails", () => {
    const href = buildDraftFallbackRunHref(GPT_IMAGE_2_CONFIG, "zh", {
      source: "model_landing",
      model: "gpt-image-2",
      mediaKind: "image",
      storageKey: "flatkey:model-generator-draft:gpt-image-2",
      prompt: "Create a purple sneaker product shot",
      fields: { size: "1024x1024", quality: "high" },
      request: {
        model: "gpt-image-2",
        prompt: "Create a purple sneaker product shot",
        size: "1024x1024",
      },
    });

    const url = new URL(href);
    const redirect = new URL(url.searchParams.get("redirect")!, url.origin);
    const draft = JSON.parse(redirect.searchParams.get("draft")!) as { prompt?: string; request?: { size?: string } };

    expect(url.pathname).toBe("/sign-up");
    expect(url.searchParams.get("lng")).toBe("zh");
    expect(redirect.pathname).toBe("/playground");
    expect(redirect.searchParams.get("model")).toBe("gpt-image-2");
    expect(redirect.searchParams.get("media_kind")).toBe("image");
    expect(draft.prompt).toBe("Create a purple sneaker product shot");
    expect(draft.request?.size).toBe("1024x1024");
  });

  test("animates section scrolling across multiple frames", () => {
    const scrollCalls: number[] = [];
    const frameCallbacks: Array<(time: number) => void> = [];
    const fakeWindow = {
      scrollY: 120,
      scrollTo: ({ top }: { top: number }) => {
        scrollCalls.push(Math.round(top));
      },
      requestAnimationFrame: (callback: (time: number) => void) => {
        frameCallbacks.push(callback);
        return frameCallbacks.length;
      },
      cancelAnimationFrame: () => undefined,
      matchMedia: () => ({ matches: false }),
    } as unknown as Window;

    animateScrollToTop(fakeWindow, 420, { durationMs: 200 });

    expect(frameCallbacks.length).toBe(1);
    frameCallbacks.shift()?.(0);
    expect(scrollCalls.at(-1)).toBe(120);

    expect(frameCallbacks.length).toBe(1);
    frameCallbacks.shift()?.(100);
    expect(scrollCalls.at(-1)).toBeGreaterThan(120);

    expect(frameCallbacks.length).toBe(1);
    frameCallbacks.shift()?.(200);
    expect(scrollCalls.at(-1)).toBe(420);
    expect(scrollCalls.length).toBeGreaterThan(2);
  });

  test("routes the top Get API Key action to the console overview", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(hrefBeforeText(html, "Get API Key")).toBe(
      "https://console.flatkey.ai/dashboard",
    );
  });

  test("keeps the top Get API Key action group aligned to the right", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="en" liveModels={[]} />
    );

    expect(html).toContain("ml-auto flex flex-wrap items-center justify-end gap-2");
  });

  test("renders Flatkey homepage-style sections for video model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_CONFIG} locale="en" liveModels={[]} />
    );

    for (const id of ["workbench", "health", "providers", "related", "faq"]) {
      expect(html).toContain(`id="${id}"`);
    }
    expect(html).toContain("Generator setup");
    expect(html).toContain("Create with this model");
    expect(html).toContain("Start generating");
    expect(html).toContain("Reset");
    expect(html).not.toContain("Request preview");
    expect(html).toContain("Model price comparison");
    expect(html).toContain("Model catalog");
    expect(html).toContain("Related models");
    expect(html).toContain("Frequently asked questions");
    expect(html).toContain('type="application/ld+json"');
    expect(html).toContain('"@type":"Product"');
    expect(html).toContain('"@type":"FAQPage"');
  });

  test("renders a simplified brand-video generator workbench", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_CONFIG} locale="zh" liveModels={[]} />
    );
    const workbenchHtml = sectionHtml(html, "workbench", "related");

    expect(workbenchHtml).toContain("生成器配置");
    expect(workbenchHtml).not.toContain("Playground 预览");
    expect(workbenchHtml).not.toContain("在这里配置真实的");
    expect(workbenchHtml).toContain("Reset");
    expect(workbenchHtml).toContain("开始生成");
    expect(workbenchHtml).not.toContain("打开控制台");
    expect(workbenchHtml).not.toContain("获取 API Key");
    expect(workbenchHtml).not.toContain("查看 API 文档");
    expect(workbenchHtml).toContain("/assets/cli/ugc-ad-clips.mp4");
    expect(workbenchHtml).toContain('data-model-output-video="true"');
    expect(workbenchHtml).not.toMatch(/<video[^>]*\smuted(?:[\s=>]|$)/);
    expect(workbenchHtml).toContain("参考素材");
    expect(workbenchHtml).toContain("上传素材");
    expect(workbenchHtml).toContain("最多 10 个图片、视频或音频素材");
    expect(workbenchHtml).toContain('accept="image/*,video/*,audio/*"');
    expect(workbenchHtml).toContain('data-reference-media-limit="10"');
    expect(workbenchHtml).toContain("<select");
    expect(workbenchHtml).not.toContain('type="checkbox"');
    expect(workbenchHtml).not.toContain('type="number"');
    expect(workbenchHtml).not.toContain("Quick Prompts");
    expect(workbenchHtml).not.toContain("Advanced Options");
  });

  test("marks the current model section link as active", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="zh" liveModels={[]} />
    );
    const navStart = html.indexOf('aria-label="Model page sections"');
    const navHtml = html.slice(navStart - 500, navStart + 3000);

    expect(navHtml).toContain('data-section-id="workbench"');
    expect(navHtml).toContain('aria-current="true"');
    expect(navHtml).toContain("data-active-model-section");
  });

  test("renders the model section navigation as a responsive sticky subnav with smooth-scroll links", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="en" liveModels={[]} />
    );
    const navStart = html.indexOf('aria-label="Model page sections"');
    expect(navStart).toBeGreaterThanOrEqual(0);
    const navHtml = html.slice(navStart - 800, navStart + 3000);

    expect(navHtml).toContain("sticky z-30");
    expect(navHtml).toContain('style="top:var(--fk-model-sticky-offset, var(--fk-site-header-height))"');
    expect(navHtml).toContain("max-w-[var(--fk-site-frame-max-width)]");
    expect(navHtml).toContain("px-[var(--fk-site-gutter)]");
    expect(navHtml).toContain('data-model-section-link="true"');
    for (const href of ["#workbench", "#related", "#readme", "#providers"]) {
      expect(navHtml).toContain(`href="${href}"`);
    }
    expect(html).toContain("scroll-mt-[var(--fk-model-section-scroll-margin)]");
  });

  test("renders model-specific README content sections for media model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_CONFIG} locale="en" liveModels={[]} />
    );

    for (const id of ["readme", "capabilities", "access", "use-cases"]) {
      expect(html).toContain(`id="${id}"`);
    }
    expect(html).toContain("scroll-mt-[var(--fk-model-section-scroll-margin)]");
    expect(html).toContain("Key features of Seedance 2.0 API");
    expect(html).toContain("How to access Seedance 2.0 API on Flatkey");
    expect(html).toContain("What you can build with Seedance 2.0 API");
    expect(html).toContain("Reference-guided video generation");
  });

  test("keeps the model detail first screen compact with one model type and a row-style price comparison", () => {
    const liveModel: PricingModel = {
      model_name: "gpt-image-2",
      vendor_name: "OpenAI",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.06,
      completion_ratio: 0,
      supported_endpoint_types: ["image-generation"],
      enable_groups: ["standard"],
      group_ratio: { standard: 0.67 },
    };
    const html = renderToStaticMarkup(
      <ModelLandingPage
        config={GPT_IMAGE_2_CONFIG}
        locale="en"
        liveModels={[liveModel]}
        allModels={[liveModel]}
        groupRatio={{ standard: 0.67 }}
      />
    );
    const heroStart = html.indexOf('<main class="home-landing');
    const heroEnd = html.indexOf('aria-label="Model page sections"');
    expect(heroStart).toBeGreaterThan(0);
    expect(heroEnd).toBeGreaterThan(0);
    const heroHtml = html.slice(heroStart, heroEnd);

    expect(heroHtml).toContain('data-model-hero-price-row="true"');
    expect(heroHtml).toContain('data-model-price-logo-cell="true"');
    expect(heroHtml).toContain('data-model-health-cell="true"');
    expect(heroHtml).toContain("GPT-image-2");
    expect(heroHtml).toContain("OpenAI");
    expect(heroHtml).toContain("Flatkey price");
    expect(heroHtml).toContain("Reference price");
    expect(heroHtml).toContain("Live model health");
    expect(heroHtml).toContain("$0.0402");
    expect(heroHtml).toContain("$0.06");
    expect(heroHtml).toContain("Image");
    expect((heroHtml.match(/data-model-type-chip="true"/g) ?? []).length).toBe(1);
    expect(heroHtml).not.toContain("Context");
    expect(heroHtml).not.toContain("Released");
    expect(heroHtml).not.toContain("model-pages/image-api-hero");
    expect(heroHtml).not.toContain("%2Fassets%2Fmodel-pages%2Fimage-api-hero.png");
  });

  test("localizes the new video model detail copy on Chinese pages", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_CONFIG} locale="zh" liveModels={[]} />
    );

    expect(html).toContain("Seedance 2.0 是通过 Flatkey 提供的视频生成模型");
    expect(html).toContain("图生视频");
    expect(html).toContain("参考图引导视频");
    expect(html).toContain("短视频生成");
    expect(html).toContain("参考图 / 首帧");
    expect(html).toContain("用例和参数配置");
    expect(html).toContain("视频草稿的一次性控制台接力");
    expect(html).not.toContain("Configure a real Seedance 2.0 video request here");
    expect(html).not.toContain("Reference-guided Video");
  });

  test("localizes image, audio, and text model detail copy on Chinese pages", () => {
    const imageHtml = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="zh" liveModels={[]} />
    );
    const audioConfig = getModelLandingConfigForPricingModel({
      model_name: "sonilo-video-to-music",
      vendor_name: "Sonilo",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.009,
      completion_ratio: 0,
      supported_endpoint_types: ["video-to-music", "audio"],
    });
    const audioHtml = renderToStaticMarkup(
      <ModelLandingPage config={audioConfig} locale="zh" liveModels={[]} />
    );
    const textHtml = renderToStaticMarkup(
      <ModelLandingPage config={GPT_CONFIG} locale="zh" liveModels={[]} />
    );

    expect(imageHtml).toContain("GPT-image-2 是用于 prompt 驱动视觉");
    expect(imageHtml).toContain("文生图像");
    expect(imageHtml).toContain("参考图准备");
    expect(audioHtml).toContain("sonilo-video-to-music 是用于视频配乐");
    expect(audioHtml).toContain("视频转音乐");
    expect(audioHtml).toContain("语音、音乐和声音工作流控制");
    expect(textHtml).toContain("使用 GPT-5 API 构建的主要方式");
    expect(textHtml).toContain("聊天和 Agent 后端");
    expect(textHtml).toContain("生产控制");
    expect(imageHtml).not.toContain("Prepare a GPT-image-2 image request here");
    expect(audioHtml).not.toContain("Prepare an audio request for sonilo-video-to-music here");
    expect(textHtml).not.toContain("Main ways to build with GPT-5 API");
  });

  test("localizes new model detail README copy for non-English locales", () => {
    const spanishHtml = renderToStaticMarkup(
      <ModelLandingPage config={SEEDANCE_CONFIG} locale="es" liveModels={[]} />
    );
    const japaneseHtml = renderToStaticMarkup(
      <ModelLandingPage config={GPT_IMAGE_2_CONFIG} locale="ja" liveModels={[]} />
    );

    expect(spanishHtml).toContain("Funciones clave de Seedance 2.0 API");
    expect(spanishHtml).toContain("Generación de video guiada por referencias");
    expect(spanishHtml).not.toContain("Key features of Seedance 2.0 API");
    expect(spanishHtml).not.toContain("Reference-guided video generation");
    expect(japaneseHtml).toContain("GPT-image-2 API の主な機能");
    expect(japaneseHtml).toContain("参照画像の引き継ぎ");
    expect(japaneseHtml).not.toContain("Key features of GPT-image-2 API");
    expect(japaneseHtml).not.toContain("Reference image handoff");
  });

  test("renders audio model pages with audio-specific guidance instead of image guidance", () => {
    const sonilo: PricingModel = {
      model_name: "sonilo-video-to-music",
      vendor_name: "Sonilo",
      quota_type: 1,
      model_ratio: 0,
      model_price: 0.009,
      completion_ratio: 0,
      supported_endpoint_types: ["video-to-music", "audio"],
    };
    const config = getModelLandingConfigForPricingModel(sonilo);
    const html = renderToStaticMarkup(
      <ModelLandingPage config={config} locale="en" liveModels={[sonilo]} allModels={[sonilo]} />
    );

    expect(html).toContain("Key features of sonilo-video-to-music API");
    expect(html).toContain("Voice, music, and sound workflow control");
    expect(html).toContain("Studio-quality voiceover and narration");
    expect(html).not.toContain("Text to Image with GPT Image-2 API");
    expect(html).not.toContain("Image-to-video production");
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

  test("renders back and console actions on localized media model landings", () => {
    const html = renderToStaticMarkup(
      <ModelLandingPage config={MINIMAX_H3_CONFIG} locale="zh" liveModels={[]} />
    );

    expect(html).toContain('href="/zh/models"');
    expect(html).toContain("返回模型列表");
    expect(html).toContain("打开控制台");
    expect(html).toContain("获取 API Key");
    expect(html).toContain("https://console.flatkey.ai/sign-up");
    expect(html).toContain("redirect=%2Fplayground%3Fsource%3Dmodel_landing");
    expect(html).toContain("model%3DMiniMax-H3");
    expect(html).not.toContain("prompt%3DA%2Bpaper%2Bboat");
  });

  test("renders Flatkey sections and related model links on localized Sonilo model pages", () => {
    const sonilo: PricingModel = {
      model_name: "sonilo-video-to-music",
      vendor_name: "Sonilo",
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
    expect(html).toContain("模型价格对比");
    expect(html).toContain("实时模型健康");
    expect(html).toContain("生成器配置");
    expect(html).toContain("用这个模型创建");
    expect(html).toContain("打开控制台");
    expect(html).not.toContain("请求预览");
    expect(html).toContain("可用目录条目");
    expect(html).toContain("常见问题");
    expect(html).toContain("继续浏览 Flatkey");
    expect(html).toContain('href="/zh/models/gpt-image-2"');
    expect(html).toContain('href="/zh/models/seedance-api"');
    expect(html).toContain('"url":"https://flatkey.ai/zh/models/sonilo-video-to-music"');
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
    const relatedSection = sectionHtml(html, "related", "readme");
    expect(relatedSection).toContain('class="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] border-b border-slate-200 bg-[#f8fafc] px-6 py-10 dark:border-white/10 dark:bg-white/[0.02]"');
    expect(relatedSection).toContain("%2Fassets%2Fmodel-pages%2Ftext-api-hero.png");
    expect(relatedSection).toContain("line-clamp-2 text-xs leading-5 text-muted-foreground");
    expect(relatedSection).not.toContain("bg-white/72");
    expect(relatedSection).not.toContain("backdrop-blur-sm");
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
});
