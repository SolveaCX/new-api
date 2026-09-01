import { APP_CONSOLE_ORIGIN } from "@/lib/origins";
import {
  mergeTrend,
  type HomeModelHealth,
  type HomePerfSummary,
  type HomeTrendPoint,
} from "@/lib/home-live";

const THIRTY_DAYS_HOURS = 720;

type SummaryPayload = {
  success?: boolean;
  data?: { models?: HomePerfSummary[] };
};

type TrendPayload = {
  success?: boolean;
  data?: {
    groups?: Array<{
      group: string;
      series: Array<{ ts: number; success_rate: number; avg_ttft_ms: number }>;
    }>;
  };
};

/**
 * Seed model detail pages with telemetry during SSR. The client keeps its
 * existing refresh requests, but the first paint now contains the same live
 * values instead of placeholder dashes.
 */
export async function fetchModelHealthData(modelName: string): Promise<HomeModelHealth> {
  const model = modelName.trim();
  if (!model) return { model: "", trend: [] };

  const summaryUrl = new URL("/api/perf-metrics/summary", APP_CONSOLE_ORIGIN);
  summaryUrl.searchParams.set("hours", String(THIRTY_DAYS_HOURS));
  summaryUrl.searchParams.set("model", model);
  const trendUrl = new URL("/api/perf-metrics", APP_CONSOLE_ORIGIN);
  trendUrl.searchParams.set("hours", String(THIRTY_DAYS_HOURS));
  trendUrl.searchParams.set("model", model);

  const [summaryResponse, trendResponse] = await Promise.all([
    fetch(summaryUrl, { cache: "no-store", headers: { accept: "application/json" } }).catch(() => null),
    fetch(trendUrl, { cache: "no-store", headers: { accept: "application/json" } }).catch(() => null),
  ]);

  let summary: HomePerfSummary | undefined;
  if (summaryResponse?.ok) {
    const payload = await summaryResponse.json().catch(() => null) as SummaryPayload | null;
    if (payload?.success) {
      summary = payload.data?.models?.find((row) => row.model_name === model);
    }
  }

  let trend: HomeTrendPoint[] = [];
  if (trendResponse?.ok) {
    const payload = await trendResponse.json().catch(() => null) as TrendPayload | null;
    if (payload?.success) trend = mergeTrend(payload.data?.groups ?? []);
  }

  return { model, trend, summary };
}
