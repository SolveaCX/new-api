import { describe, expect, test } from "bun:test";
import {
  GPU_PRICE_BANDS,
  annualValue,
  formatRemaining,
  formatUsdCompact,
  generateDemandSnapshot,
  summarizeDemand,
} from "./compute-demand";

describe("compute demand snapshot", () => {
  test("is deterministic for a given seed", () => {
    const a = generateDemandSnapshot(30, 7);
    const b = generateDemandSnapshot(30, 7);
    expect(a).toEqual(b);
    expect(generateDemandSnapshot(30, 8)).not.toEqual(a);
  });

  test("prices stay inside the market band for each GPU", () => {
    for (const row of generateDemandSnapshot(200, 3)) {
      const band = GPU_PRICE_BANDS.find((b) => b[0] === row.gpu);
      expect(band).toBeDefined();
      expect(row.ceiling).toBeGreaterThanOrEqual(band![1]);
      expect(row.ceiling).toBeLessThanOrEqual(band![2] + 1e-9);
      if (row.lowest) expect(row.lowest).toBeLessThanOrEqual(row.ceiling);
      expect(row.nickname).toMatch(/^[A-Z][a-z]+ [A-Z][a-z]+$/);
    }
  });

  test("summary counts and money helpers", () => {
    const rows = generateDemandSnapshot(40, 7);
    const stats = summarizeDemand(rows);
    expect(stats.matching).toBe(rows.filter((r) => r.status === "matching").length);
    expect(stats.openValue).toBeGreaterThan(1_000_000);
    expect(annualValue({ ...rows[0], nodes: 20, gpusPerNode: 8, lowest: 0, ceiling: 5 })).toBe(160 * 5 * 8760);
    expect(formatUsdCompact(7_008_000)).toBe("$7.0M");
    expect(formatUsdCompact(981_120)).toBe("$981K");
    expect(formatRemaining(372)).toBe("6h 12m");
    expect(formatRemaining(9)).toBe("9m");
  });
});
