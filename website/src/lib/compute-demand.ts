/**
 * Seeded, deterministic compute-demand snapshot used by the marketing site
 * (homepage ticker + /compute board) until real RFQs flow in from the
 * console Compute Market. Deterministic so server and client render the same
 * rows (no hydration mismatch); the client may append live rows after mount.
 *
 * Price bands are USD per GPU-hour, anchored to Sept 2026 public quotes
 * (OCPI H100 settle ≈ $2.5, on-demand median ≈ $3.4).
 */

export type DemandStatus = "matching" | "matched" | "delivered";

export type DemandRow = {
  id: number;
  gpu: string;
  nodes: number;
  gpusPerNode: number;
  termMonths: number;
  region: string;
  ceiling: number;
  lowest: number;
  status: DemandStatus;
  bids: number;
  nickname: string;
  kind: string;
  delivery: string;
  ageMinutes: number;
  remainingMinutes: number;
  featured?: boolean;
  paymentTerms?: string;
};

export const GPU_PRICE_BANDS: ReadonlyArray<readonly [string, number, number]> = [
  ["B300", 4.5, 6.0],
  ["B200", 3.6, 5.0],
  ["H200", 2.6, 3.6],
  ["H100", 1.9, 3.2],
  ["H100", 1.9, 3.2],
  ["A100", 1.0, 1.5],
  ["L40S", 0.7, 1.1],
  ["RTX 4090", 0.35, 0.6],
];

export const DEMAND_REGIONS = [
  "JP · Tokyo",
  "SG",
  "US-WEST",
  "US-EAST",
  "EU · Frankfurt",
  "KR · Seoul",
  "TW · Taipei",
  "IN · Mumbai",
  "JP · Osaka",
  "AU · Sydney",
] as const;

const ADJECTIVES = ["Quiet", "Amber", "Silent", "Brisk", "Cobalt", "Velvet", "Copper", "Lunar", "Hollow", "Rapid", "Ivory", "Cedar", "Pale", "Nimble", "Ember", "Slate"];
const ANIMALS = ["Falcon", "Otter", "Heron", "Lynx", "Koi", "Badger", "Marten", "Ibis", "Puffin", "Yak", "Civet", "Orca", "Finch", "Tapir", "Bison", "Moth"];
const KINDS = ["AI video startup", "LLM lab", "Agent platform", "Robotics · VLA", "Fintech AI", "Game studio", "Research institute", "Medical imaging", "Voice AI", "Search startup", "Ads-tech", "E-commerce AI"];
const DELIVERY = ["bare metal · IB 400G", "bare metal · IB 200G", "VM · 100G", "Slurm managed", "k8s managed"];
const NODES = [2, 4, 4, 8, 8, 12, 16, 20, 24, 32, 48, 64];
const TERMS = [1, 3, 3, 6, 6, 12, 12, 12, 24];

/** Small LCG so the snapshot is stable across renders and environments. */
export function createSeededRandom(seed: number) {
  let state = seed % 233280;
  return () => {
    state = (state * 9301 + 49297) % 233280;
    return state / 233280;
  };
}

const roundNickel = (value: number) => Math.round(value * 20) / 20;

export function generateDemandRow(index: number, random: () => number): DemandRow {
  const pick = <T,>(list: ReadonlyArray<T>): T => list[Math.floor(random() * list.length)];
  const band = pick(GPU_PRICE_BANDS);
  const ceiling = roundNickel(band[1] + (band[2] - band[1]) * (0.55 + random() * 0.45));
  const roll = random();
  const status: DemandStatus = roll < 0.64 ? "matching" : roll < 0.9 ? "matched" : "delivered";
  const bids = status === "matching" ? Math.floor(random() * 8) : Math.floor(2 + random() * 6);
  const lowest = bids ? roundNickel(ceiling * (0.78 + random() * 0.17)) : 0;
  const remainingMinutes = status === "matching" ? (random() < 0.2 ? Math.floor(random() * 55 + 3) : Math.floor(random() * 22 * 60 + 60)) : 0;
  return {
    id: 2000 + index,
    gpu: band[0],
    nodes: pick(NODES),
    gpusPerNode: 8,
    termMonths: pick(TERMS),
    region: pick(DEMAND_REGIONS),
    ceiling,
    lowest,
    status,
    bids,
    nickname: `${pick(ADJECTIVES)} ${pick(ANIMALS)}`,
    kind: pick(KINDS),
    delivery: pick(DELIVERY),
    ageMinutes: Math.floor(random() * 600) + 1,
    remainingMinutes,
  };
}

/** Ops-curated featured request, pinned to the top of every board. */
export const FEATURED_DEMAND: DemandRow = {
  id: 1901,
  gpu: "B300",
  nodes: 16,
  gpusPerNode: 8,
  termMonths: 36,
  region: "JP · SG · US-WEST",
  ceiling: 4.5,
  lowest: 0,
  status: "matching",
  bids: 0,
  nickname: "Cedar Orca",
  kind: "LLM lab",
  delivery: "bare metal · IB 400G",
  ageMinutes: 12,
  remainingMinutes: 22 * 60,
  featured: true,
  paymentTerms: "3-year contract · 15% down payment",
};

/** Deterministic snapshot, featured first then newest. */
export function generateDemandSnapshot(count = 40, seed = 7): DemandRow[] {
  const random = createSeededRandom(seed);
  const rows: DemandRow[] = [];
  for (let i = 0; i < count; i += 1) rows.push(generateDemandRow(i, random));
  rows.sort((a, b) => a.ageMinutes - b.ageMinutes);
  return [FEATURED_DEMAND, ...rows];
}

export const totalGpus = (row: Pick<DemandRow, "nodes" | "gpusPerNode">) => row.nodes * row.gpusPerNode;

/** Annual contract value at the lowest bid (or the ceiling before any bid). */
export const annualValue = (row: DemandRow) => totalGpus(row) * (row.lowest || row.ceiling) * 8760;

export function formatUsdCompact(value: number) {
  if (value >= 1_000_000) return `$${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `$${Math.round(value / 1_000)}K`;
  return `$${Math.round(value)}`;
}

export function formatRemaining(minutes: number) {
  if (minutes >= 60) return `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, "0")}m`;
  return `${minutes}m`;
}

export type DemandStats = {
  openValue: number;
  matching: number;
  bids: number;
  matched: number;
  endingSoon: number;
};

export function summarizeDemand(rows: DemandRow[]): DemandStats {
  const open = rows.filter((r) => r.status === "matching");
  return {
    openValue: open.reduce((sum, r) => sum + annualValue(r), 0),
    matching: open.length,
    bids: rows.reduce((sum, r) => sum + r.bids, 0),
    matched: rows.filter((r) => r.status === "matched").length,
    endingSoon: open.filter((r) => r.remainingMinutes < 60).length,
  };
}

/** Reference index quotes shown under the hero (USD / GPU-hour). */
export const DEMAND_INDEX = [
  ["H100", 2.53],
  ["H200", 3.1],
  ["B200", 4.4],
  ["B300", 5.2],
  ["A100", 1.25],
] as const;
