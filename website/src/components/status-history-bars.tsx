import type { Locale } from "@/lib/locales";
import { localizePath } from "@/lib/locales";
import { getStatusCopy, getStatusPresentation } from "@/lib/status-copy";
import type { StatusHistoryRange, StatusPeriod, StatusValue } from "@/lib/status";

const HOUR = 3_600;
const DAY = 86_400;
const RANGE_BUCKETS: Record<StatusHistoryRange, { count: number; seconds: number }> = {
  "24h": { count: 24, seconds: HOUR },
  "7d": { count: 7, seconds: DAY },
  "30d": { count: 30, seconds: DAY },
  "90d": { count: 90, seconds: DAY },
};
const WORST_STATE: Record<StatusValue, number> = {
  operational: 0,
  maintenance: 1,
  unknown: 2,
  degraded: 3,
  outage: 4,
};

export interface NormalizedStatusPeriod extends StatusPeriod {
  synthetic: boolean;
}

interface NormalizeOptions {
  days: number;
  endAt: number;
}

interface StatusHistoryBarsProps {
  locale: Locale;
  componentSlug: string;
  periods: StatusPeriod[];
  selectedRange: StatusHistoryRange;
  endAt: number;
}

export function normalizeDailyHistory(periods: StatusPeriod[], options: NormalizeOptions): NormalizedStatusPeriod[] {
  return normalizeHistory(periods, options.days, DAY, options.endAt);
}

export function normalizeStatusHistory(
  periods: StatusPeriod[],
  range: StatusHistoryRange,
  endAt: number
): NormalizedStatusPeriod[] {
  const shape = RANGE_BUCKETS[range];
  return normalizeHistory(periods, shape.count, shape.seconds, endAt);
}

export function StatusHistoryBars(props: StatusHistoryBarsProps) {
  const copy = getStatusCopy(props.locale);
  const normalized = normalizeStatusHistory(props.periods, props.selectedRange, props.endAt);
  const basePath = localizePath(`/status/models/${encodeURIComponent(props.componentSlug)}`, props.locale);

  return (
    <section aria-labelledby="status-history-title" className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950 sm:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 id="status-history-title" className="text-xl font-bold text-slate-950 dark:text-white">{copy.history.title}</h2>
        <nav aria-label={copy.history.title} className="flex flex-wrap gap-2">
          {(Object.keys(RANGE_BUCKETS) as StatusHistoryRange[]).map((range) => (
            <a
              key={range}
              href={`${basePath}?range=${range}`}
              aria-current={range === props.selectedRange ? "page" : undefined}
              className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-semibold text-slate-700 outline-none transition hover:border-slate-500 focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:border-slate-700 dark:text-slate-200"
            >
              {copy.history.ranges[range]}
            </a>
          ))}
        </nav>
      </div>

      <ol className="mt-6 flex h-28 items-end gap-0.5" aria-label={copy.history.title}>
        {normalized.map((period) => {
          const presentation = getStatusPresentation({ locale: props.locale, status: period.status, freshness: "fresh", lifecycle: "active" });
          const label = historyLabel(period, props.locale, presentation.text, copy.history.noEvidence, props.selectedRange === "24h");
          return (
            <li
              key={period.period_start}
              data-status-day={period.period_start}
              aria-label={label}
              title={label}
              className={`min-w-0 flex-1 rounded-sm ${presentation.barClass} focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600`}
              tabIndex={0}
            >
              <span className="sr-only">{label}</span>
            </li>
          );
        })}
      </ol>
    </section>
  );
}

function normalizeHistory(periods: StatusPeriod[], count: number, seconds: number, endAt: number): NormalizedStatusPeriod[] {
  const endBucket = Math.floor(endAt / seconds) * seconds;
  const byBucket = new Map<number, StatusPeriod>();

  for (const period of periods) {
    const bucket = Math.floor(period.period_start / seconds) * seconds;
    if (bucket > endBucket || bucket <= endBucket - count * seconds) continue;
    const current = byBucket.get(bucket);
    if (!current || WORST_STATE[period.status] > WORST_STATE[current.status]) {
      byBucket.set(bucket, { ...period, period_start: bucket });
    }
  }

  return Array.from({ length: count }, (_, index) => {
    const periodStart = endBucket - (count - 1 - index) * seconds;
    const period = byBucket.get(periodStart);
    return period
      ? { ...period, period_start: periodStart, synthetic: false }
      : { period_start: periodStart, availability: 0, coverage: 0, status: "unknown", synthetic: true };
  });
}

function historyLabel(period: NormalizedStatusPeriod, locale: Locale, state: string, noEvidence: string, hourly: boolean): string {
  const date = new Intl.DateTimeFormat(locale, hourly
    ? { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" }
    : { dateStyle: "medium", timeZone: "UTC" }
  ).format(new Date(period.period_start * 1_000));
  if (period.synthetic || period.status === "unknown") return `${date}: ${state}. ${noEvidence}`;
  return `${date}: ${state}. ${formatMicros(period.availability)} availability, ${formatMicros(period.coverage)} coverage`;
}

function formatMicros(value: number): string {
  return `${(value / 10_000).toFixed(2)}%`;
}
