import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import type { Locale } from "@/lib/locales";
import type { Locale } from "@/lib/locales";

mock.module("server-only", () => ({}));

function hrefBeforeText(html: string, text: string): string {
  const textIndex = html.indexOf(`>${text}<`);
  expect(textIndex).toBeGreaterThanOrEqual(0);
  const matches = [...html.slice(0, textIndex).matchAll(/href="([^"]+)"/g)];
  expect(matches.length).toBeGreaterThan(0);
  return matches[matches.length - 1][1].replaceAll("&amp;", "&");
}

function panelMarkup(html: string, panelClass: string, nextPanelClass?: string): string {
  const start = html.indexOf(panelClass);
  expect(start).toBeGreaterThanOrEqual(0);
  const end = nextPanelClass ? html.indexOf(nextPanelClass, start) : html.length;
  expect(end).toBeGreaterThan(start);
  return html.slice(start, end);
}

describe("OnlineHomePage", () => {
  const signupHref = "https://console.flatkey.ai/sign-up?lng=en";
  const overviewHref =
    "https://console.flatkey.ai/sign-up?redirect=%2Fdashboard%2Foverview&lng=en";

  test("routes home auth CTAs to console signup when no session hint exists", async () => {
    const { OnlineHomePage } = await import("./online-home-page");
    const html = renderToStaticMarkup(
      await OnlineHomePage({ locale: "en" }),
    );

    expect(hrefBeforeText(html, "Quick Start")).toBe(
      overviewHref,
    );
    expect(hrefBeforeText(html, "Get started")).toBe(signupHref);
  });

  test("does not treat a console session hint as verified auth for home CTAs", async () => {
    const { OnlineHomePage } = await import("./online-home-page");
    const html = renderToStaticMarkup(
      await OnlineHomePage({ locale: "en", hasConsoleSessionHint: true }),
    );

    expect(hrefBeforeText(html, "Quick Start")).toBe(
      overviewHref,
    );
    expect(hrefBeforeText(html, "Get started")).toBe(signupHref);
  });

  test("keeps the primary CTA pointed at the console overview in a non-English locale", async () => {
    const { OnlineHomePage } = await import("./online-home-page");
    const html = renderToStaticMarkup(
      await OnlineHomePage({ locale: "zh" }),
    );

    expect(hrefBeforeText(html, "快速开始")).toBe(
      "https://console.flatkey.ai/sign-up?redirect=%2Fdashboard%2Foverview&lng=zh",
    );
  });

  test("localizes the primary CTA across every homepage locale", async () => {
    const labels = {
      en: "Quick Start",
      zh: "快速开始",
      es: "Inicio rápido",
      fr: "Démarrage rapide",
      pt: "Início rápido",
      ru: "Быстрый старт",
      ja: "クイックスタート",
      vi: "Bắt đầu nhanh",
      de: "Schnellstart",
      id: "Mulai cepat",
    } as const;

    const { OnlineHomePage } = await import("./online-home-page");
    for (const [locale, label] of Object.entries(labels)) {
      const html = renderToStaticMarkup(
        await OnlineHomePage({ locale: locale as Locale }),
      );
      expect(hrefBeforeText(html, label)).toBe(
        `https://console.flatkey.ai/sign-up?redirect=%2Fdashboard%2Foverview&lng=${locale}`,
      );
    }
  });

  test("localizes the redesigned homepage panels for Chinese", async () => {
    const { OnlineHomePage } = await import("./online-home-page");
    const html = renderToStaticMarkup(await OnlineHomePage({ locale: "zh" }));

    expect(html).toContain("精选模型");
    expect(html).toContain("只需一个 key，系统会根据每个任务、输入和场景");
    expect(html).toContain("测试提示词");
    expect(html).toContain("生成标准");
    expect(html).toContain("已选模型");
    expect(html).toContain("仅成功调用才付费");
    expect(html).not.toContain("Test Prompt");
    expect(html).not.toContain("Selected model");
    expect(html).not.toContain("Total runtime");
    expect(html).not.toContain("Pay per successful call");
    expect(html).not.toContain("voice of customer");
  });

  test("localizes image and video generation tabs across supported locales", async () => {
    const expected = {
      zh: ["测试提示词", "生成标准", "18.4 秒"],
      es: ["Prompt de prueba", "Criterios", "18,4 s"],
      fr: ["Prompt de test", "Critères", "18,4 s"],
      pt: ["Prompt de teste", "Critérios", "18,4 s"],
      ru: ["Тестовый промпт", "Критерии", "18,4 с"],
      ja: ["テストプロンプト", "生成条件", "18.4 秒"],
      vi: ["Prompt thử nghiệm", "Tiêu chí", "18,4 giây"],
      de: ["Test-Prompt", "Kriterien", "18,4 s"],
      id: ["Prompt uji", "Kriteria", "18,4 dtk"],
    } as const;

    for (const [locale, [prompt, criteria, runtime]] of Object.entries(expected)) {
      const { OnlineHomePage } = await import("./online-home-page");
      const html = renderToStaticMarkup(await OnlineHomePage({ locale: locale as Locale }));
      const imagePanel = panelMarkup(
        html,
        '<div class="intelligence-panel intelligence-panel-media intelligence-panel-image"',
        '<div class="intelligence-panel intelligence-panel-media intelligence-panel-video"',
      );
      const videoPanel = panelMarkup(
        html,
        '<div class="intelligence-panel intelligence-panel-media intelligence-panel-video"',
        '<section class="tools-intro"',
      );

      for (const value of [prompt, criteria, runtime]) {
        expect(imagePanel).toContain(value);
        expect(videoPanel).toContain(value);
      }
      for (const value of ["Test Prompt", "Criteria", "Selected model", "Total runtime", "Pay per successful call", "18.4 sec"]) {
        expect(imagePanel).not.toContain(value);
        expect(videoPanel).not.toContain(value);
      }
    }
  });

  test("uses task-specific models and keeps each media result aligned with its selected model", async () => {
    const { OnlineHomePage } = await import("./online-home-page");
    const html = renderToStaticMarkup(
      await OnlineHomePage({ locale: "en" }),
    );
    const imagePanel = panelMarkup(
      html,
      '<div class="intelligence-panel intelligence-panel-media intelligence-panel-image"',
      '<div class="intelligence-panel intelligence-panel-media intelligence-panel-video"',
    );
    const videoPanel = panelMarkup(
      html,
      '<div class="intelligence-panel intelligence-panel-media intelligence-panel-video"',
      '<section class="tools-intro"',
    );

    expect(imagePanel).toContain("nano-banana-pro-preview");
    expect(imagePanel).toContain("gemini-3.1-flash-image");
    expect(imagePanel).toContain("imagen-4.0-ultra-generate-001");
    expect(imagePanel).toContain("flux-2-pro");
    expect(imagePanel.match(/gpt-image-2/g)?.length).toBe(2);
    expect(imagePanel).not.toContain("deepseek-v4-pro");
    expect(imagePanel).not.toContain("MiniMax-H3");

    expect(videoPanel).toContain("sora-2");
    expect(videoPanel).toContain("MiniMax-H3");
    expect(videoPanel).toContain("veo-3.1-generate-preview");
    expect(videoPanel).toContain("kling-2.5-pro");
    expect(videoPanel.match(/seedance-2.5/g)?.length).toBe(2);
    expect(videoPanel).not.toContain("google/gemini-3.7-flash");
    expect(videoPanel).not.toContain("qwen/qwen3.8-max");
  });
});
