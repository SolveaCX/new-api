import { describe, expect, test } from "bun:test";
import {
  buildDirectoryRow,
  facetCount,
  filterDirectoryRows,
  hasActiveFilters,
  sortDirectoryRows,
  EMPTY_DIRECTORY_FILTERS,
  type DirectoryFilters,
} from "./model-directory-filters";
import { ageBandFor, formatContextTokens, getModelMeta, priceBandFor, seriesForModels } from "./model-directory-meta";
import { MODEL_DIRECTORY_META } from "./model-directory-meta-data";
import { directoryHref, directorySearchQuery, parseDirectorySearch, toggleDirectoryFilter } from "./model-directory-url";

const NOW = new Date("2026-08-19T00:00:00Z");

function rows(
  names: Array<{ name: string; vendor: string; inputUsd?: number; outputUsd?: number; officialUsd?: number }>
) {
  return names.map((input) => buildDirectoryRow(input, NOW));
}

const SAMPLE = rows([
  { name: "claude-opus-5", vendor: "Anthropic", inputUsd: 5, outputUsd: 25 },
  { name: "gpt-5.6-sol", vendor: "OpenAI", inputUsd: 5, outputUsd: 7 },
  { name: "gpt-4o-mini", vendor: "OpenAI", inputUsd: 0.15, outputUsd: 0.6 },
  { name: "deepseek-v4-pro", vendor: "DeepSeek", inputUsd: 1.32, outputUsd: 2 },
  { name: "seedance-2.5", vendor: "ByteDance", inputUsd: 0.14 },
  { name: "gemini-2.5-flash", vendor: "Google", inputUsd: 0.3, outputUsd: 2.5 },
]);

function withFilters(overrides: Partial<DirectoryFilters>): DirectoryFilters {
  return { ...EMPTY_DIRECTORY_FILTERS, ...overrides };
}

describe("price banding", () => {
  test("buckets on half-open ranges so boundaries land in exactly one band", () => {
    expect(priceBandFor(0.49)).toBe("lt-0.5");
    expect(priceBandFor(0.5)).toBe("0.5-1");
    expect(priceBandFor(1)).toBe("1-2");
    expect(priceBandFor(5)).toBe("5-10");
    expect(priceBandFor(25)).toBe("10+");
  });

  test("treats missing and non-positive prices as unpriced, not cheapest", () => {
    expect(priceBandFor(undefined)).toBeUndefined();
    expect(priceBandFor(0)).toBeUndefined();
    expect(priceBandFor(Number.NaN)).toBeUndefined();
  });
});

describe("age banding", () => {
  test("recomputes the band against the current date", () => {
    expect(ageBandFor("2026-08-04", NOW)).toBe("new");
    expect(ageBandFor("2026-06-20", NOW)).toBe("1-3m");
    expect(ageBandFor("2026-04-06", NOW)).toBe("3-6m");
    expect(ageBandFor("2025-11-22", NOW)).toBe("6-12m");
    expect(ageBandFor("2025-05-26", NOW)).toBe("12m+");
  });

  test("a date that was New last year reads as older today", () => {
    expect(ageBandFor("2026-08-04", new Date("2027-08-19T00:00:00Z"))).toBe("12m+");
  });

  test("returns undefined for unknown dates rather than defaulting into a band", () => {
    expect(ageBandFor(null, NOW)).toBeUndefined();
    expect(ageBandFor("not-a-date", NOW)).toBeUndefined();
  });
});

describe("filter semantics", () => {
  test("within a group is OR", () => {
    const result = filterDirectoryRows(SAMPLE, withFilters({ series: ["GPT", "Claude"] }));
    expect(result.map((row) => row.name).sort()).toEqual(["claude-opus-5", "gpt-4o-mini", "gpt-5.6-sol"]);
  });

  test("across groups is AND", () => {
    const result = filterDirectoryRows(SAMPLE, withFilters({ series: ["GPT"], modalities: ["image"] }));
    expect(result.map((row) => row.name).sort()).toEqual(["gpt-4o-mini", "gpt-5.6-sol"]);

    const narrowed = filterDirectoryRows(SAMPLE, withFilters({ series: ["GPT"], modalities: ["video"] }));
    expect(narrowed).toHaveLength(0);
  });

  test("context length is a >= filter using the smallest selected bound", () => {
    const oneMillion = filterDirectoryRows(SAMPLE, withFilters({ context: [1048576] }));
    expect(oneMillion.map((row) => row.name).sort()).toEqual([
      "claude-opus-5",
      "deepseek-v4-pro",
      "gemini-2.5-flash",
      "gpt-5.6-sol",
    ]);

    const mixed = filterDirectoryRows(SAMPLE, withFilters({ context: [1048576, 128000] }));
    expect(mixed.map((row) => row.name)).toContain("gpt-4o-mini");
  });

  test("models with no context window are excluded from context filters", () => {
    const result = filterDirectoryRows(SAMPLE, withFilters({ context: [8192] }));
    expect(result.map((row) => row.name)).not.toContain("seedance-2.5");
  });

  test("price bands filter on the live figure, not stored metadata", () => {
    const cheap = filterDirectoryRows(SAMPLE, withFilters({ inputPrice: ["lt-0.5"] }));
    expect(cheap.map((row) => row.name).sort()).toEqual(["gemini-2.5-flash", "gpt-4o-mini", "seedance-2.5"]);

    const repriced = rows([{ name: "gpt-4o-mini", vendor: "OpenAI", inputUsd: 6 }]);
    expect(filterDirectoryRows(repriced, withFilters({ inputPrice: ["lt-0.5"] }))).toHaveLength(0);
    expect(filterDirectoryRows(repriced, withFilters({ inputPrice: ["5-10"] }))).toHaveLength(1);
  });

  test("search matches every term across name, vendor, series and categories", () => {
    expect(filterDirectoryRows(SAMPLE, withFilters({ q: "openai mini" })).map((row) => row.name)).toEqual(["gpt-4o-mini"]);
    expect(filterDirectoryRows(SAMPLE, withFilters({ q: "anthropic" })).map((row) => row.name)).toEqual(["claude-opus-5"]);
    expect(filterDirectoryRows(SAMPLE, withFilters({ q: "nonexistent" }))).toHaveLength(0);
  });

  test("vendor filter from existing sitemap links still applies", () => {
    expect(filterDirectoryRows(SAMPLE, withFilters({ vendor: "OpenAI" }))).toHaveLength(2);
    expect(filterDirectoryRows(SAMPLE, withFilters({ vendor: "all" }))).toHaveLength(SAMPLE.length);
  });

  test("the model-authors group is OR within itself", () => {
    const result = filterDirectoryRows(SAMPLE, withFilters({ vendors: ["OpenAI", "Anthropic"] }));
    expect(result.map((row) => row.name).sort()).toEqual(["claude-opus-5", "gpt-4o-mini", "gpt-5.6-sol"]);
  });

  test("model authors AND across other groups", () => {
    const result = filterDirectoryRows(SAMPLE, withFilters({ vendors: ["OpenAI"], inputPrice: ["lt-0.5"] }));
    expect(result.map((row) => row.name)).toEqual(["gpt-4o-mini"]);
  });

  test("the legacy single-vendor param narrows alongside the group", () => {
    // ?vendor=Google plus a group selection that excludes Google yields nothing,
    // rather than one silently overriding the other.
    expect(filterDirectoryRows(SAMPLE, withFilters({ vendor: "Google", vendors: ["OpenAI"] }))).toHaveLength(0);
    expect(filterDirectoryRows(SAMPLE, withFilters({ vendor: "OpenAI", vendors: ["OpenAI"] }))).toHaveLength(2);
  });
});

describe("facet counts", () => {
  test("ignore other selections in the same group so siblings never zero out", () => {
    const filters = withFilters({ series: ["GPT"] });
    expect(facetCount(SAMPLE, filters, "series", "Claude")).toBe(1);
    expect(facetCount(SAMPLE, filters, "series", "DeepSeek")).toBe(1);
  });

  test("respect selections in other groups", () => {
    const filters = withFilters({ modalities: ["video"] });
    expect(facetCount(SAMPLE, filters, "series", "GPT")).toBe(0);
    expect(facetCount(SAMPLE, filters, "series", "Seedance")).toBe(1);
  });
});

describe("sorting", () => {
  test("most popular leads with the board order, then overall rank", () => {
    const sorted = sortDirectoryRows(SAMPLE, "rank");
    expect(sorted[0].name).toBe("gpt-5.6-sol"); // TOP 2 — the lowest board position present
    expect(sorted.map((row) => row.name).slice(0, 3)).toEqual(["gpt-5.6-sol", "deepseek-v4-pro", "seedance-2.5"]);
  });

  test("longest context first, unknown context last", () => {
    const sorted = sortDirectoryRows(SAMPLE, "ctxDesc");
    expect(sorted[sorted.length - 1].name).toBe("seedance-2.5");
  });

  test("newest orders by band and leaves unknown ages last", () => {
    const withUnknown = [...SAMPLE, buildDirectoryRow({ name: "totally-unknown-model", vendor: "Nobody" }, NOW)];
    const sorted = sortDirectoryRows(withUnknown, "newest");
    expect(sorted[0].age).toBe("new");
    expect(sorted[sorted.length - 1].name).toBe("totally-unknown-model");
  });

  test("name sorts alphabetically", () => {
    expect(sortDirectoryRows(SAMPLE, "name")[0].name).toBe("claude-opus-5");
  });

  test("biggest discount first, computed from live prices", () => {
    const priced = rows([
      { name: "gpt-5.6-sol", vendor: "OpenAI", officialUsd: 5, inputUsd: 1.5 }, // 70%
      { name: "claude-opus-5", vendor: "Anthropic", officialUsd: 5, inputUsd: 4.5 }, // 10%
      { name: "deepseek-v4-pro", vendor: "DeepSeek", officialUsd: 1.32, inputUsd: 1.122 }, // 15%
    ]);
    expect(sortDirectoryRows(priced, "discount").map((row) => row.name)).toEqual([
      "gpt-5.6-sol",
      "deepseek-v4-pro",
      "claude-opus-5",
    ]);
  });

  test("rows with no comparable price sort last, not as a 0% discount", () => {
    const mixed = rows([
      { name: "claude-opus-5", vendor: "Anthropic", officialUsd: 5, inputUsd: 5 }, // real 0%
      { name: "seedance-2.5", vendor: "ByteDance", inputUsd: 0.14 }, // no official price
      { name: "gpt-5.6-sol", vendor: "OpenAI", officialUsd: 5, inputUsd: 4 }, // 20%
    ]);
    const sorted = sortDirectoryRows(mixed, "discount").map((row) => row.name);
    expect(sorted[0]).toBe("gpt-5.6-sol");
    expect(sorted[sorted.length - 1]).toBe("seedance-2.5");
  });

  test("a reprice changes the discount order without touching metadata", () => {
    const before = rows([
      { name: "gpt-4o-mini", vendor: "OpenAI", officialUsd: 1, inputUsd: 0.9 }, // 10%
      { name: "claude-opus-5", vendor: "Anthropic", officialUsd: 1, inputUsd: 0.5 }, // 50%
    ]);
    expect(sortDirectoryRows(before, "discount")[0].name).toBe("claude-opus-5");

    const after = rows([
      { name: "gpt-4o-mini", vendor: "OpenAI", officialUsd: 1, inputUsd: 0.1 }, // now 90%
      { name: "claude-opus-5", vendor: "Anthropic", officialUsd: 1, inputUsd: 0.5 },
    ]);
    expect(sortDirectoryRows(after, "discount")[0].name).toBe("gpt-4o-mini");
  });
});

describe("unknown models degrade gracefully", () => {
  const unknown = buildDirectoryRow({ name: "brand-new-model-2027", vendor: "NewCo", inputUsd: 2 }, NOW);

  test("still render and stay searchable", () => {
    expect(filterDirectoryRows([unknown], EMPTY_DIRECTORY_FILTERS)).toHaveLength(1);
    expect(filterDirectoryRows([unknown], withFilters({ q: "newco" }))).toHaveLength(1);
  });

  test("drop out of metadata-driven filters but keep live price filters", () => {
    expect(filterDirectoryRows([unknown], withFilters({ modalities: ["text"] }))).toHaveLength(0);
    expect(filterDirectoryRows([unknown], withFilters({ age: ["new"] }))).toHaveLength(0);
    expect(filterDirectoryRows([unknown], withFilters({ inputPrice: ["2-5"] }))).toHaveLength(1);
  });

  test("infer a series from the model name when the table has no entry", () => {
    const inferred = buildDirectoryRow({ name: "claude-opus-9-future", vendor: "Anthropic" }, NOW);
    expect(inferred.series).toBe("Claude");
    expect(filterDirectoryRows([inferred], withFilters({ series: ["Claude"] }))).toHaveLength(1);
  });

  test("infer a series through a vendor/model namespace", () => {
    // The catalogue serves some models as `vendor/model`; the series patterns
    // anchor at the start, so the namespace has to be stripped first.
    expect(buildDirectoryRow({ name: "bytedance/seedance-2.0-fast", vendor: "AI" }, NOW).series).toBe("Seedance");
    expect(buildDirectoryRow({ name: "openai/gpt-9", vendor: "AI" }, NOW).series).toBe("GPT");
  });
});

describe("url round-trip", () => {
  test("serializes and re-parses every group", () => {
    const filters = withFilters({
      modalities: ["image", "video"],
      context: [1048576],
      inputPrice: ["lt-0.5"],
      outputPrice: ["10+"],
      vendors: ["OpenAI", "Anthropic"],
      series: ["GPT", "Claude"],
      categories: ["Programming"],
      age: ["new"],
      distillable: [true],
      q: "vision",
    });

    const parsed = parseDirectorySearch(Object.fromEntries(new URLSearchParams(directorySearchQuery(filters, "newest"))));
    expect(parsed.modalities).toEqual(["image", "video"]);
    expect(parsed.context).toEqual([1048576]);
    expect(parsed.inputPrice).toEqual(["lt-0.5"]);
    expect(parsed.outputPrice).toEqual(["10+"]);
    expect(parsed.vendors).toEqual(["OpenAI", "Anthropic"]);
    expect(parsed.series).toEqual(["GPT", "Claude"]);
    expect(parsed.age).toEqual(["new"]);
    expect(parsed.distillable).toEqual([true]);
    expect(parsed.q).toBe("vision");
    expect(parsed.sort).toBe("newest");
  });

  test("omits the default sort and empty groups", () => {
    expect(directorySearchQuery(EMPTY_DIRECTORY_FILTERS, "rank")).toBe("");
    expect(directoryHref("en", EMPTY_DIRECTORY_FILTERS)).toBe("/models");
    expect(directoryHref("zh", withFilters({ series: ["GPT"] }))).toBe("/zh/models?series=GPT");
  });

  test("drops malformed values instead of throwing", () => {
    const parsed = parseDirectorySearch({
      modalities: "image,telepathy",
      context: "1048576,abc",
      age: "new,ancient",
      distillable: "maybe",
      sort: "bogus",
    });
    expect(parsed.modalities).toEqual(["image"]);
    expect(parsed.context).toEqual([1048576]);
    expect(parsed.age).toEqual(["new"]);
    expect(parsed.distillable).toEqual([]);
    expect(parsed.sort).toBe("rank");
  });

  test("toggling adds then removes a value", () => {
    const once = toggleDirectoryFilter(EMPTY_DIRECTORY_FILTERS, "series", "GPT");
    expect(once.series).toEqual(["GPT"]);
    expect(toggleDirectoryFilter(once, "series", "GPT").series).toEqual([]);
  });

  test("hasActiveFilters reflects search, vendor and groups", () => {
    expect(hasActiveFilters(EMPTY_DIRECTORY_FILTERS)).toBe(false);
    expect(hasActiveFilters(withFilters({ q: "gpt" }))).toBe(true);
    expect(hasActiveFilters(withFilters({ vendor: "OpenAI" }))).toBe(true);
    expect(hasActiveFilters(withFilters({ vendor: "all" }))).toBe(false);
    expect(hasActiveFilters(withFilters({ modalities: ["text"] }))).toBe(true);
  });
});

describe("metadata table", () => {
  test("context labels read compactly", () => {
    expect(formatContextTokens(1048576)).toBe("1M");
    expect(formatContextTokens(262144)).toBe("262K");
    expect(formatContextTokens(128000)).toBe("128K");
    expect(formatContextTokens(null)).toBeUndefined();
  });

  test("every entry carries the fields the filters depend on", () => {
    for (const [name, meta] of Object.entries(MODEL_DIRECTORY_META)) {
      expect(meta.series, `${name} series`).toBeTruthy();
      expect(Array.isArray(meta.modalities), `${name} modalities`).toBe(true);
      expect(Array.isArray(meta.categories), `${name} categories`).toBe(true);
      expect(typeof meta.distillable, `${name} distillable`).toBe("boolean");
      if (meta.releasedAt != null) {
        expect(meta.releasedAt, `${name} releasedAt`).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      }
      if (meta.contextTokens != null) expect(meta.contextTokens, `${name} contextTokens`).toBeGreaterThan(0);
    }
  });

  test("series listing is ordered by the best rank in each family", () => {
    const series = seriesForModels(["gpt-4o-mini", "deepseek-v4-flash", "claude-opus-5"]);
    expect(series).toEqual(["DeepSeek", "Claude", "GPT"]);
  });

  test("lookup is exact so a renamed model reports as missing rather than mismatched", () => {
    expect(getModelMeta("claude-opus-5")).toBeDefined();
    expect(getModelMeta("claude-opus-5 ")).toBeUndefined();
  });
});
