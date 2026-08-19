import { describe, expect, test } from "bun:test";
import type { ComponentProps } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ModelsDirectoryTable, buildDirectoryHealthTrend } from "./models-directory-table";
import { getModelsDirectoryTableCopy } from "./pricing-explorer";

describe("ModelsDirectoryTable", () => {
  test("renders model directory columns for input, output, and capabilities", () => {
    const rows = [
      {
        name: "gpt-5-mini",
        vendor: "OpenAI",
        official: "$0.5",
        discounted: "$0.2",
        officialUsd: 0.5,
        discountedUsd: 0.2,
        output: "$0.8",
        billing: "Token",
        capabilities: ["Chat", "Tools", "Vision"],
        iconKey: "openai",
        priceUnit: "per 1M tokens",
      },
    ] as unknown as ComponentProps<typeof ModelsDirectoryTable>["rows"];

    const html = renderToStaticMarkup(
      <ModelsDirectoryTable locale="en" copy={getModelsDirectoryTableCopy("en")} rows={rows} />
    );

    expect(html).toContain("Provider");
    expect(html).toContain("Input");
    expect(html).toContain("Output");
    expect(html).toContain("Capabilities");
    expect(html).toContain("Tools");
    expect(html).toContain("Vision");
    expect(html).toContain("$0.8");
  });

  test("renders the directory as cards when card view is selected", () => {
    const rows = [
      {
        name: "gpt-5-mini",
        vendor: "OpenAI",
        official: "$0.5",
        discounted: "$0.2",
        officialUsd: 0.5,
        discountedUsd: 0.2,
        output: "$0.8",
        billing: "Token",
        capabilities: ["Chat", "Tools", "Vision"],
        iconKey: "openai",
        priceUnit: "per 1M tokens",
      },
    ] as unknown as ComponentProps<typeof ModelsDirectoryTable>["rows"];

    const html = renderToStaticMarkup(
      <ModelsDirectoryTable locale="en" copy={getModelsDirectoryTableCopy("en")} rows={rows} view="cards" />
    );

    expect(html).toContain('data-local-models-card-grid="true"');
    expect(html).toContain('data-local-model-card="gpt-5-mini"');
    expect(html).toContain('data-local-model-card-metrics="true"');
    expect(html).toContain('data-local-card-health-metric="true"');
    expect(html).toContain('data-local-card-title-row="true"');
    expect(html).toContain('data-local-card-divider="true"');
    expect(html).toContain('data-local-card-vendor-row="true"');
    expect(html).toContain('data-local-card-capabilities="true"');
    expect(html).toContain("OpenAI");
    expect(html).toContain("Health");
    expect(html).toContain("$0.8");
    expect(html.indexOf("gpt-5-mini")).toBeLessThan(html.indexOf("Chat"));
    expect(html.indexOf("Chat")).toBeLessThan(html.indexOf("OpenAI"));
    expect(html).not.toContain("<table");
    expect(html).not.toContain('data-local-card-health-row="true"');
    expect(html).not.toContain("min-h-7");
  });

  test("uses model and vendor identity to render provider logos", () => {
    const rows = [
      {
        name: "deepseek-v4-flash",
        vendor: "DeepSeek",
        official: "$0.5",
        discounted: "$0.2",
        officialUsd: 0.5,
        discountedUsd: 0.2,
        iconKey: "openai",
      },
    ] as unknown as ComponentProps<typeof ModelsDirectoryTable>["rows"];

    const html = renderToStaticMarkup(
      <ModelsDirectoryTable locale="en" copy={getModelsDirectoryTableCopy("en")} rows={rows} />
    );

    expect(html).toContain("/assets/logos/deepseek.svg");
    expect(html).not.toContain("icons/openai.svg");
  });

  test("uses default latency and fifteen healthy bars when health data is missing", () => {
    const html = renderToStaticMarkup(
      <ModelsDirectoryTable
        locale="en"
        copy={getModelsDirectoryTableCopy("en")}
        rows={[
          {
            name: "gpt-5-mini",
            vendor: "OpenAI",
            official: "$0.5",
            discounted: "$0.2",
            officialUsd: 0.5,
            discountedUsd: 0.2,
            iconKey: "openai",
          },
        ]}
      />
    );

    expect(html).toContain("Input");
    expect(html).toContain("Output");
    expect(html).toContain("Health");
    expect(html).not.toContain("After bonus");
    expect(html).not.toContain("30-day health");
    expect(html).toContain("600ms");
    expect(html).toContain(">100%</span>");
    expect(html.match(/title="[^"]* · 100\.00%"/g)?.length).toBe(15);
  });

  test("pads short health trends to fifteen daily points with healthy defaults", () => {
    const day = 24 * 60 * 60;
    const first = Date.UTC(2026, 7, 16) / 1000;
    const second = first + day;

    const points = buildDirectoryHealthTrend([
      { ts: first, success_rate: 97, avg_ttft_ms: 540 },
      { ts: second, success_rate: 92, avg_ttft_ms: 810 },
    ]);

    expect(points).toHaveLength(15);
    expect(points.slice(0, 13)).toEqual(
      Array.from({ length: 13 }, (_, index) => ({
        ts: first - day * (13 - index),
        success_rate: 100,
        avg_ttft_ms: 600,
      }))
    );
    expect(points.slice(13)).toEqual([
      { ts: first, success_rate: 97, avg_ttft_ms: 540 },
      { ts: second, success_rate: 92, avg_ttft_ms: 810 },
    ]);
  });

  test("renders row-specific pricing units and from text", () => {
    const html = renderToStaticMarkup(
      <ModelsDirectoryTable
        locale="en"
        copy={getModelsDirectoryTableCopy("en")}
        rows={[
          {
            name: "gpt-5-mini",
            vendor: "OpenAI",
            official: "$0.5",
            discounted: "$0.2",
            officialUsd: 0.5,
            discountedUsd: 0.2,
            iconKey: "openai",
            priceUnit: "per 1M tokens",
          },
          {
            name: "video-request-model",
            vendor: "VideoAI",
            official: "$1",
            discounted: "$0.9",
            officialUsd: 1,
            discountedUsd: 0.9,
            iconKey: "videoai",
            priceUnit: "per request",
          },
          {
            name: "video-second-model",
            vendor: "VideoAI",
            official: "$0.08",
            discounted: "$0.072",
            officialUsd: 0.08,
            discountedUsd: 0.072,
            iconKey: "videoai",
            priceUnit: "per second",
            pricePrefix: "from",
          },
        ]}
      />
    );

    expect(html).toContain("per 1M tokens");
    expect(html).toContain("per request");
    expect(html).toContain("per second");
    expect(html).toContain("from");
    expect(html).toContain("from</span>$0.072");
    expect(html).not.toContain("$1 /req");
  });

  test("localizes second and request units on the Chinese models page", () => {
    const html = renderToStaticMarkup(
      <ModelsDirectoryTable
        locale="zh"
        copy={getModelsDirectoryTableCopy("zh")}
        rows={[
          {
            name: "video-second-model",
            vendor: "VideoAI",
            official: "$0.08",
            discounted: "$0.072",
            officialUsd: 0.08,
            discountedUsd: 0.072,
            iconKey: "videoai",
            priceUnit: "per second",
          },
          {
            name: "image-request-model",
            vendor: "ImageAI",
            official: "$0.04",
            discounted: "$0.036",
            officialUsd: 0.04,
            discountedUsd: 0.036,
            iconKey: "imageai",
            priceUnit: "per request",
          },
          {
            name: "token-model",
            vendor: "OpenAI",
            official: "$1",
            discounted: "$0.9",
            officialUsd: 1,
            discountedUsd: 0.9,
            iconKey: "openai",
            priceUnit: "per 1M tokens",
          },
        ]}
      />
    );

    expect(html).toContain("/ 秒");
    expect(html).toContain("/ 次");
    expect(html).toContain("/ 1M tokens");
  });
});
