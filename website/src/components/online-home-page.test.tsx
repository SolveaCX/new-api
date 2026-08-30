import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

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

    expect(hrefBeforeText(html, "Get Up to $40 in Free Credits")).toBe(
      overviewHref,
    );
    expect(hrefBeforeText(html, "Get started")).toBe(signupHref);
  });

  test("does not treat a console session hint as verified auth for home CTAs", async () => {
    const { OnlineHomePage } = await import("./online-home-page");
    const html = renderToStaticMarkup(
      await OnlineHomePage({ locale: "en", hasConsoleSessionHint: true }),
    );

    expect(hrefBeforeText(html, "Get Up to $40 in Free Credits")).toBe(
      overviewHref,
    );
    expect(hrefBeforeText(html, "Get started")).toBe(signupHref);
  });

  test("keeps the free-credits CTA pointed at the console overview in a non-English locale", async () => {
    const { OnlineHomePage } = await import("./online-home-page");
    const html = renderToStaticMarkup(
      await OnlineHomePage({ locale: "zh" }),
    );

    expect(hrefBeforeText(html, "最高领取 $40 免费额度")).toBe(
      "https://console.flatkey.ai/sign-up?redirect=%2Fdashboard%2Foverview&lng=zh",
    );
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
