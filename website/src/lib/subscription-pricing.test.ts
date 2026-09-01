import { describe, expect, test } from "bun:test";
import { formatUsd, STANDARD_SUBSCRIPTION_LIMITS } from "./subscription-pricing";

describe("standard subscription pricing", () => {
  test("keeps the approved USD limits for every standard tier", () => {
    expect(STANDARD_SUBSCRIPTION_LIMITS).toEqual({
      go: { priceUsd: 10, fiveHourUsd: 8, sevenDayUsd: 12, monthlyUsd: 25 },
      pro: { priceUsd: 30, fiveHourUsd: 18, sevenDayUsd: 45, monthlyUsd: 90 },
      max: { priceUsd: 100, fiveHourUsd: 78, sevenDayUsd: 220, monthlyUsd: 450 },
    });
  });

  test("formats contract values consistently for public copy", () => {
    expect(formatUsd(STANDARD_SUBSCRIPTION_LIMITS.go.monthlyUsd)).toBe("$25");
    expect(formatUsd(STANDARD_SUBSCRIPTION_LIMITS.max.monthlyUsd)).toBe("$450");
    expect(formatUsd(12.5)).toBe("$12.5");
  });
});
