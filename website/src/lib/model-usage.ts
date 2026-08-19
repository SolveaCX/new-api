import { APP_CONSOLE_ORIGIN } from "./origins";

export type ModelUsagePoint = {
  /** Unix seconds at UTC day start. */
  date: number;
  count: number;
};

export type ModelUsage = {
  model: string;
  total: number;
  points: ModelUsagePoint[];
};

type ModelUsagePayload = {
  success?: boolean;
  model?: string;
  total?: number;
  points?: { date?: number; count?: number }[];
};

/**
 * Daily call counts for one model, from the Go app's keyed website feed.
 *
 * The console's own /api/data routes are admin/user scoped, so the public site
 * cannot read them. This uses /api/website/model-usage with WEBSITE_METRICS_KEY,
 * which returns call counts only — no quota, users, or channels.
 *
 * Server-only: the key must never reach the browser, so this is called from a
 * server component and the value is passed down as props.
 *
 * Returns null when the key is unset, the request fails, or the model has no
 * recorded traffic. Callers hide the section rather than render an empty chart.
 */
export async function fetchModelUsage(modelName: string, days = 30): Promise<ModelUsage | null> {
  const key = process.env.WEBSITE_METRICS_KEY?.trim();
  if (!key) return null;

  try {
    const target = new URL("/api/website/model-usage", APP_CONSOLE_ORIGIN);
    target.searchParams.set("model", modelName);
    target.searchParams.set("days", String(days));

    const response = await fetch(target, {
      headers: { accept: "application/json", "X-Website-Metrics-Key": key },
      // Matches the endpoint's own cache header; the series only moves hourly.
      next: { revalidate: 300 },
    });
    if (!response.ok) return null;

    const payload = (await response.json()) as ModelUsagePayload;
    if (!payload.success) return null;

    const points = (payload.points ?? [])
      .map((point) => ({ date: Number(point.date), count: Number(point.count) }))
      .filter((point) => Number.isFinite(point.date) && Number.isFinite(point.count) && point.count >= 0)
      .sort((a, b) => a.date - b.date);

    // No traffic is not an error, but there is nothing worth charting.
    if (points.length === 0) return null;

    return {
      model: payload.model ?? modelName,
      total: Number(payload.total ?? points.reduce((sum, point) => sum + point.count, 0)),
      points,
    };
  } catch {
    return null;
  }
}
