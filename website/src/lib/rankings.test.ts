import { describe, expect, test } from "bun:test";
import { formatGrowth, normalizeRankingPeriod, publicRankingsUrl } from "./rankings";

describe("publicRankingsUrl", () => {
  test("points website rankings at the public website API", () => {
    expect(publicRankingsUrl("month", "https://console.flatkey.ai")).toBe(
      "https://console.flatkey.ai/api/website/rankings?period=month"
    );
  });

  test("defaults to weekly rankings", () => {
    expect(publicRankingsUrl(undefined, "https://console.flatkey.ai")).toBe(
      "https://console.flatkey.ai/api/website/rankings?period=week"
    );
  });

  test("normalizes non-public periods back to weekly rankings", () => {
    expect(normalizeRankingPeriod("today")).toBe("week");
    expect(normalizeRankingPeriod("year")).toBe("week");
    expect(normalizeRankingPeriod("all")).toBe("week");
    expect(publicRankingsUrl("all", "https://console.flatkey.ai")).toBe(
      "https://console.flatkey.ai/api/website/rankings?period=week"
    );
  });

  test("formats suppressed growth with a fallback label", () => {
    expect(formatGrowth(undefined, "sample limited")).toBe("sample limited");
    expect(formatGrowth(10)).toBe("+10%");
  });
});
