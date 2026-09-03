import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ModelsDirectoryTable, attributionLabel, buildDirectoryHealthTrend } from "./models-directory-table";
import { getModelsDirectoryTableCopy } from "./pricing-explorer";

describe("ModelsDirectoryTable", () => {
  test("uses default latency and a full healthy bar wall when health data is missing", () => {
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

    expect(html).toContain("Our price");
    expect(html).toContain("Health Score");
    expect(html).not.toContain("After bonus");
    expect(html).not.toContain("30-day health");
    expect(html).toContain("600ms");
    expect(html).toContain(">100%</span>");
    expect(html.match(/title="[^"]* · 100\.00%"/g)?.length).toBe(15);
  });

  test("pads short health trends up to the full daily window with healthy defaults", () => {
    const day = 24 * 60 * 60;
    const first = Date.UTC(2026, 7, 16) / 1000;
    const second = first + day;

    const points = buildDirectoryHealthTrend([
      { ts: first, success_rate: 97, avg_ttft_ms: 540 },
      { ts: second, success_rate: 92, avg_ttft_ms: 810 },
    ]);

    // Two real days, padded on the left to the 15-bar window.
    expect(points).toHaveLength(15);
    expect(points.slice(0, 3)).toEqual([
      { ts: first - day * 13, success_rate: 100, avg_ttft_ms: 600 },
      { ts: first - day * 12, success_rate: 100, avg_ttft_ms: 600 },
      { ts: first - day * 11, success_rate: 100, avg_ttft_ms: 600 },
    ]);
    expect(points.slice(-2)).toEqual([
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
    expect(html).toContain("from</span><span class=\"line-through\">$0.08</span>");
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

  test("places a per-second video rate in the directory's Our output column", () => {
    const html = renderToStaticMarkup(
      <ModelsDirectoryTable
        locale="en"
        copy={{ ...getModelsDirectoryTableCopy("en"), colInput: "Our input", colOutput: "Our output" }}
        hideOurPrice
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
            output: "$0.072",
          },
        ]}
      />
    );

    const body = html.split("<tbody>")[1]?.split("</tbody>")[0] ?? "";
    const cells = body.match(/<td[^>]*>[\s\S]*?<\/td>/g) ?? [];
    // Model, official, input, output, latency, health (Our price is hidden).
    expect(cells[2]).toContain("—");
    expect(cells[3]).toContain("$0.072");
  });

  test("renders cache pricing in the cache column", () => {
    const html = renderToStaticMarkup(
      <ModelsDirectoryTable
        locale="en"
        copy={{ ...getModelsDirectoryTableCopy("en"), colCache: "Our cache" }}
        hideOurPrice
        rows={[{
          name: "gpt-5-mini",
          vendor: "OpenAI",
          official: "$0.5",
          discounted: "$0.2",
          officialUsd: 0.5,
          discountedUsd: 0.2,
          iconKey: "openai",
          priceUnit: "per 1M tokens",
          cache: "$0.1",
        }]}
      />
    );
    expect(html).toContain("Our cache");
    expect(html).toContain("$0.1");
  });

  test("keeps every column available through horizontal scrolling", () => {
    const html = renderToStaticMarkup(
      <ModelsDirectoryTable
        locale="en"
        copy={{ ...getModelsDirectoryTableCopy("en"), colInput: "Our input", colOutput: "Our output" }}
        hideOurPrice
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

    expect(html).toContain("touch-pan-x overflow-x-auto overscroll-x-contain");
    expect(html).toContain("w-max min-w-full table-auto border-collapse text-sm");
    expect(html).toContain("sticky left-0 z-10 min-w-[320px]");
    expect(html).toContain("min-w-[132px] px-2 py-3.5 text-right text-[10px] font-bold leading-4 whitespace-normal");
    expect(html).not.toContain("hidden w-[11%]");
  });

  test("keeps the full model name above promotion badges", () => {
    const html = renderToStaticMarkup(
      <ModelsDirectoryTable
        locale="en"
        copy={getModelsDirectoryTableCopy("en")}
        rows={[
          {
            name: "glm-5.3-flash",
            vendor: "Z.ai",
            official: "$1",
            discounted: "$0.8",
            officialUsd: 1,
            discountedUsd: 0.8,
            iconKey: "zai",
            top10: 1,
            tags: ["Limited discount", "New release"],
          },
        ]}
      />
    );

    const nameIndex = html.indexOf(">glm-5.3-flash</span>");
    const attributionIndex = html.indexOf(">Z.ai</span>");
    const limitedIndex = html.indexOf(">Limited discount</span>");
    const newReleaseIndex = html.indexOf(">New release</span>");
    const modelCell = html.slice(html.indexOf("<td"), html.indexOf("</td>"));

    expect(nameIndex).toBeGreaterThanOrEqual(0);
    expect(attributionIndex).toBeGreaterThan(nameIndex);
    expect(limitedIndex).toBeGreaterThan(attributionIndex);
    expect(newReleaseIndex).toBeGreaterThan(limitedIndex);
    expect(modelCell).not.toContain("TOP");
    expect(modelCell).toContain('title="glm-5.3-flash"');
    expect(modelCell).toContain("shrink-0 whitespace-nowrap font-mono");
    expect(modelCell).not.toContain("truncate font-mono");
    expect(modelCell).toContain("text-muted-foreground/70 block truncate text-[11px]");
    expect(modelCell).toContain("mt-0.5 flex min-w-0 flex-wrap items-center gap-1.5");
  });
});

describe("attribution label", () => {
  test("shows vendor and series together when both are known", () => {
    expect(attributionLabel("Anthropic", "Claude")).toBe("Anthropic · Claude");
  });

  test("falls back to the series when the vendor is the AI placeholder", () => {
    // getVendorName yields "AI" for models the payload leaves without a vendor;
    // attributing the model to "AI" would invent an author that does not exist.
    expect(attributionLabel("AI", "Seedance")).toBe("Seedance");
    expect(attributionLabel("AI", undefined)).toBe("");
  });

  test("handles a missing vendor or series without stray separators", () => {
    expect(attributionLabel(undefined, "Claude")).toBe("Claude");
    expect(attributionLabel("Anthropic", undefined)).toBe("Anthropic");
    expect(attributionLabel(undefined, undefined)).toBe("");
  });

  test("a vendor that merely contains AI is still a real vendor", () => {
    expect(attributionLabel("Open AI Labs", "GPT")).toBe("Open AI Labs · GPT");
  });
});
