import { describe, expect, test } from "bun:test";
import { formatUsd, STANDARD_SUBSCRIPTION_LIMITS } from "./subscription-pricing";

describe("standard subscription pricing", () => {
  test("keeps the approved USD limits for every standard tier", () => {
    expect(STANDARD_SUBSCRIPTION_LIMITS).toEqual({
      go: { priceUsd: 10, fiveHourUsd: 10, sevenDayUsd: 18, monthlyUsd: 45 },
      pro: { priceUsd: 30, fiveHourUsd: 30, sevenDayUsd: 60, monthlyUsd: 90 },
      max: { priceUsd: 100, fiveHourUsd: 80, sevenDayUsd: 240, monthlyUsd: 300 },
    });
  });

  test("formats contract values consistently for public copy", () => {
    expect(formatUsd(STANDARD_SUBSCRIPTION_LIMITS.go.monthlyUsd)).toBe("$45");
    expect(formatUsd(STANDARD_SUBSCRIPTION_LIMITS.max.monthlyUsd)).toBe("$300");
    expect(formatUsd(12.5)).toBe("$12.5");
  });
});
