import { APP_CONSOLE_ORIGIN } from "@/lib/origins";

export type RankingPeriod = "week" | "month";

export type WebsiteRankedModel = {
  rank: number;
  previous_rank?: number;
  model_name: string;
  vendor: string;
  vendor_icon?: string;
  category: string;
  share: number;
  growth_pct?: number;
};

export type WebsiteRankedVendor = {
  rank: number;
  vendor: string;
  vendor_icon?: string;
  share: number;
  growth_pct?: number;
  models_count: number;
  top_model?: string;
};

export type WebsiteRankingMover = {
  model_name: string;
  vendor: string;
  vendor_icon?: string;
  rank_delta: number;
  current_rank: number;
  growth_pct?: number;
};

export type WebsiteRankingsData = {
  period: RankingPeriod;
  models: WebsiteRankedModel[];
  vendors: WebsiteRankedVendor[];
  top_movers: WebsiteRankingMover[];
  top_droppers: WebsiteRankingMover[];
};

type WebsiteRankingsResponse = {
  success: boolean;
  message?: string;
  data?: WebsiteRankingsData;
};

export const RANKING_PERIODS: RankingPeriod[] = ["week", "month"];

const PERIODS = new Set<RankingPeriod>(RANKING_PERIODS);

export function normalizeRankingPeriod(period: string | string[] | undefined): RankingPeriod {
  const value = Array.isArray(period) ? period[0] : period;
  return value && PERIODS.has(value as RankingPeriod) ? (value as RankingPeriod) : "week";
}

export function publicRankingsUrl(period: string | undefined = "week", apiBaseUrl = APP_CONSOLE_ORIGIN): string {
  const target = new URL("/api/website/rankings", apiBaseUrl);
  target.searchParams.set("period", normalizeRankingPeriod(period));
  return target.toString();
}

export async function getWebsiteRankingsData(period: RankingPeriod = "week"): Promise<WebsiteRankingsData> {
  try {
    const response = await fetch(publicRankingsUrl(period), {
      next: { revalidate: 300 },
      headers: { accept: "application/json" },
    });
    if (!response.ok) return emptyRankingsData(period);
    const payload = (await response.json()) as WebsiteRankingsResponse;
    if (!payload.success || !payload.data) return emptyRankingsData(period);
    return payload.data;
  } catch {
    return emptyRankingsData(period);
  }
}

export function emptyRankingsData(period: RankingPeriod): WebsiteRankingsData {
  return {
    period,
    models: [],
    vendors: [],
    top_movers: [],
    top_droppers: [],
  };
}

export function formatShare(share: number): string {
  return `${Math.round(Math.max(0, share) * 1000) / 10}%`;
}

export function formatGrowth(growth: number | null | undefined, fallback = "N/A"): string {
  if (growth == null || !Number.isFinite(growth)) return fallback;
  const rounded = Math.round(growth * 10) / 10;
  return `${rounded > 0 ? "+" : ""}${rounded}%`;
}
