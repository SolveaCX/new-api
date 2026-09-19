// Demand pools: many small GPU requests rolled up per model so they read as
// one large order. Tiers mirror the backend (model/compute_demand_lead.go).

export const POOL_TIERS: ReadonlyArray<{ gpus: number; discount: number }> = [
  { gpus: 64, discount: 5 },
  { gpus: 256, discount: 10 },
  { gpus: 1024, discount: 18 },
  { gpus: 4096, discount: 25 },
];

export type DemandPool = {
  gpu_model: string;
  gpus: number;
  requests: number;
  leads: number;
  tier: number;
  next_tier_gpus: number;
  discount_pct: number;
};

export const POOL_GPU_MODELS = ["B300", "B200", "H200", "H100", "A100", "L40S", "RTX 5090", "RTX 4090", "M3 Ultra"] as const;

export function poolTier(gpus: number): { tier: number; next: number; discount: number } {
  let tier = 0;
  let discount = 0;
  for (let i = 0; i < POOL_TIERS.length; i += 1) {
    const t = POOL_TIERS[i];
    if (gpus >= t.gpus) {
      tier = i + 1;
      discount = t.discount;
    } else {
      return { tier, next: t.gpus, discount };
    }
  }
  return { tier, next: 0, discount };
}

/** Progress (0..1) toward the next tier, for the pool bar. */
export function poolProgress(gpus: number, next: number): number {
  if (next <= 0) return 1;
  const prev = POOL_TIERS.filter((t) => t.gpus < next).map((t) => t.gpus).pop() ?? 0;
  return Math.max(0.04, Math.min(1, (gpus - prev) / (next - prev)));
}

/** Fallback pools shown when the live API is unreachable at render time. */
export function fallbackPools(): DemandPool[] {
  return [
    ["B300", 296, 4],
    ["H200", 168, 5],
    ["B200", 96, 3],
    ["H100", 72, 3],
  ].map(([gpu_model, gpus, requests]) => {
    const t = poolTier(gpus as number);
    return {
      gpu_model: gpu_model as string,
      gpus: gpus as number,
      requests: requests as number,
      leads: 0,
      tier: t.tier,
      next_tier_gpus: t.next,
      discount_pct: t.discount,
    };
  });
}

export async function fetchDemandPools(origin: string): Promise<DemandPool[] | null> {
  try {
    const res = await fetch(new URL("/api/compute/market/public/pools", origin), {
      cache: "no-store",
      headers: { accept: "application/json" },
      signal: AbortSignal.timeout(2500),
    });
    if (!res.ok) return null;
    const json = (await res.json()) as { success?: boolean; data?: { pools?: DemandPool[] } };
    if (!json.success || !Array.isArray(json.data?.pools)) return null;
    return json.data.pools;
  } catch {
    return null;
  }
}
