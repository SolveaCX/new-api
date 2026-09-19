import { describe, expect, it } from "bun:test";
import { fallbackPools, poolProgress, poolTier } from "./compute-pools";

describe("compute demand pools", () => {
  it("maps pooled GPUs to tiers", () => {
    expect(poolTier(0)).toEqual({ tier: 0, next: 64, discount: 0 });
    expect(poolTier(64)).toEqual({ tier: 1, next: 256, discount: 5 });
    expect(poolTier(300)).toEqual({ tier: 2, next: 1024, discount: 10 });
    expect(poolTier(5000)).toEqual({ tier: 4, next: 0, discount: 25 });
  });
  it("reports progress toward the next tier", () => {
    expect(poolProgress(64, 256)).toBeCloseTo(0.04);
    expect(poolProgress(160, 256)).toBeCloseTo(0.5);
    expect(poolProgress(999, 0)).toBe(1);
  });
  it("falls back to a sorted non-empty pool list", () => {
    const pools = fallbackPools();
    expect(pools.length).toBeGreaterThan(2);
    expect(pools[0].gpus).toBeGreaterThanOrEqual(pools[1].gpus);
    expect(pools.every((p) => p.tier >= 1)).toBe(true);
  });
});
