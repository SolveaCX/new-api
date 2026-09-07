import { describe, expect, test } from "bun:test";
import { formatUsd, STANDARD_SUBSCRIPTION_LIMITS } from "./subscription-pricing";

describe("standard subscription pricing", () => {
  test("keeps the approved USD limits for every standard tier", () => {
    expect(STANDARD_SUBSCRIPTION_LIMITS).toEqual({
      go: { priceUsd: 10, monthlyUsd: 13 },
      pro: { priceUsd: 30, monthlyUsd: 45 },
      max: { priceUsd: 100, monthlyUsd: 170 },
    });
  });

  test("formats contract values consistently for public copy", () => {
    expect(formatUsd(STANDARD_SUBSCRIPTION_LIMITS.go.monthlyUsd)).toBe("$13");
    expect(formatUsd(STANDARD_SUBSCRIPTION_LIMITS.max.monthlyUsd)).toBe("$170");
    expect(formatUsd(12.5)).toBe("$12.5");
  });
});
