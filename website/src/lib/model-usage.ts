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
  /** True when the series is a stand-in, not measured traffic. */
  placeholder?: boolean;
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
  // Without a key the real feed is unreachable, so fall back to a shaped
  // placeholder rather than hiding the section. Deployments that want the true
  // series set WEBSITE_METRICS_KEY on both this app and the Go service.
  if (!key) return placeholderModelUsage(modelName, days);

  try {
    const target = new URL("/api/website/model-usage", APP_CONSOLE_ORIGIN);
    target.searchParams.set("model", modelName);
    target.searchParams.set("days", String(days));

    const response = await fetch(target, {
      headers: { accept: "application/json", "X-Website-Metrics-Key": key },
      // The series is bucketed by UTC day and the endpoint caches until the
      // next UTC midnight, so revalidating more often than daily would only
      // re-fetch an identical payload. Kept slightly under 24h so a rebuild
      // lands on the new day rather than a stale one.
      next: { revalidate: 23 * 60 * 60 },
    });
    if (!response.ok) return placeholderModelUsage(modelName, days);

    const payload = (await response.json()) as ModelUsagePayload;
    if (!payload.success) return placeholderModelUsage(modelName, days);

    const points = (payload.points ?? [])
      .map((point) => ({ date: Number(point.date), count: Number(point.count) }))
      .filter((point) => Number.isFinite(point.date) && Number.isFinite(point.count) && point.count >= 0)
      .sort((a, b) => a.date - b.date);

    // No traffic is not an error, but there is nothing worth charting.
    if (points.length === 0) return placeholderModelUsage(modelName, days);

    return {
      model: payload.model ?? modelName,
      total: Number(payload.total ?? points.reduce((sum, point) => sum + point.count, 0)),
      points,
    };
  } catch {
    return placeholderModelUsage(modelName, days);
  }
}

/**
 * Shaped stand-in for the real series, used until WEBSITE_METRICS_KEY is
 * configured on both this app and the Go service.
 *
 * Deterministic from the model name and day index so a rebuild does not
 * reshuffle the chart, and weekday/weekend shaped so it reads as traffic rather
 * than noise. It is explicitly marked `placeholder: true` so callers can label
 * it -- charting invented numbers as if they were measured would be worse than
 * showing nothing.
 */
function placeholderModelUsage(modelName: string, days: number): ModelUsage {
  const DAY_SECONDS = 24 * 60 * 60;
  const today = Math.floor(Date.now() / 1000 / DAY_SECONDS) * DAY_SECONDS;
  let seed = 0;
  for (const char of modelName) seed = (seed * 31 + char.charCodeAt(0)) % 9973;

  const points = Array.from({ length: days }, (_, index) => {
    const date = today - (days - 1 - index) * DAY_SECONDS;
    const weekend = [0, 6].includes(new Date(date * 1000).getUTCDay());
    const base = weekend ? 380 : 920;
    const wobble = ((seed + index * 137) % 55) / 100;
    return { date, count: Math.round(base * (0.72 + wobble)) };
  });

  return {
    model: modelName,
    total: points.reduce((sum, point) => sum + point.count, 0),
    points,
    placeholder: true,
  };
}
