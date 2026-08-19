"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { DailyHealthBars } from "@/components/home-health-bars";
import { HomeModelLogo } from "@/components/home-model-logo";
import { formatHealthSuccessRate, getJitteredSuccessRate } from "@/lib/health-display";
import type { HomeCopy } from "@/lib/home-copy";
import {
  fetchHealthSummary,
  fetchModelTrend,
  formatLatencyMs,
  trendAvgTtftMs,
  type HomePerfSummary,
  type HomeTrendPoint,
} from "@/lib/home-live";
import type { HomePricedModel } from "@/lib/home-models";
import { localizePath, type Locale } from "@/lib/locales";
import { modelPublicPath } from "@/lib/model-public";

export type ModelsDirectoryTableCopy = HomeCopy["table"] & {
  colProvider: string;
  colInput: string;
  colOutput: string;
  colCapabilities: string;
  colBilling: string;
  noCapabilities: string;
  latencyLabel: string;
};

type Props = {
  copy: ModelsDirectoryTableCopy;
  rows: HomePricedModel[];
  locale?: Locale;
  view?: "list" | "cards";
};

const DEFAULT_TTFT_MS = 600;
const DEFAULT_HEALTH_SUCCESS_RATE = 100;
const HEALTH_BAR_COUNT = 15;
const DAY_SECONDS = 24 * 60 * 60;

// /models directory: every priced model as one row with input/output pricing,
// capability badges, TTFT latency, and compact health bars. Health series load
// lazily as rows scroll into view so the directory does not fan out requests.
export function ModelsDirectoryTable(props: Props) {
  const [summary, setSummary] = useState<Record<string, HomePerfSummary>>({});
  const [trends, setTrends] = useState<Record<string, HomeTrendPoint[]>>({});
  const requested = useRef(new Set<string>());

  useEffect(() => {
    let cancelled = false;
    fetchHealthSummary().then((data) => {
      if (!cancelled) setSummary(data);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const loadTrend = (model: string) => {
    if (requested.current.has(model)) return;
    requested.current.add(model);
    fetchModelTrend(model).then((points) => {
      if (points.length > 0) setTrends((current) => ({ ...current, [model]: points }));
    });
  };

  if (props.rows.length === 0) return null;

  if (props.view === "cards") {
    return (
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3" data-local-models-card-grid="true">
        {props.rows.map((row) => (
          <DirectoryCard
            key={row.name}
            copy={props.copy}
            row={row}
            perf={summary[row.name]}
            trend={trends[row.name] ?? []}
            locale={props.locale}
            onVisible={() => loadTrend(row.name)}
          />
        ))}
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-2xl border border-violet-500/16 bg-white/72 shadow-lg backdrop-blur-sm dark:border-white/10 dark:bg-white/[0.04] dark:shadow-violet-950/10">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[940px] border-collapse text-sm">
          <thead>
            <tr className="border-b border-violet-500/12 bg-violet-500/[0.035] text-left text-[11px] font-bold tracking-[0.08em] text-slate-500 uppercase dark:border-white/10 dark:bg-white/[0.035] dark:text-slate-400">
              <th className="px-5 py-4 font-bold">{props.copy.colModel}</th>
              <th className="px-4 py-4 font-bold">{props.copy.colProvider}</th>
              <th className="px-4 py-4 text-right font-bold">{props.copy.colInput}</th>
              <th className="px-4 py-4 text-right font-bold">{props.copy.colOutput}</th>
              <th className="px-4 py-4 font-bold">{props.copy.colCapabilities}</th>
              <th className="w-[240px] px-5 py-4 text-left font-bold">{props.copy.colHealth}</th>
            </tr>
          </thead>
          <tbody>
            {props.rows.map((row) => (
              <DirectoryRow
                key={row.name}
                copy={props.copy}
                row={row}
                perf={summary[row.name]}
                trend={trends[row.name] ?? []}
                locale={props.locale}
                onVisible={() => loadTrend(row.name)}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function DirectoryCard(props: {
  copy: ModelsDirectoryTableCopy;
  row: HomePricedModel;
  perf: HomePerfSummary | undefined;
  trend: HomeTrendPoint[];
  locale?: Locale;
  onVisible: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const { onVisible } = props;

  useEffect(() => {
    const node = ref.current;
    if (!node || typeof IntersectionObserver === "undefined") {
      onVisible();
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          onVisible();
          observer.disconnect();
        }
      },
      { rootMargin: "200px" }
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [onVisible]);

  const { row, perf, trend } = props;
  const latencyMs = perf?.avg_ttft_ms && perf.avg_ttft_ms > 0 ? perf.avg_ttft_ms : trendAvgTtftMs(trend) || DEFAULT_TTFT_MS;
  const measuredSuccessRate = validSuccessRate(perf?.success_rate) ?? trendAvgSuccessRate(trend);
  const displaySuccessRate =
    measuredSuccessRate == null
      ? DEFAULT_HEALTH_SUCCESS_RATE
      : getJitteredSuccessRate(measuredSuccessRate, row.name) ?? measuredSuccessRate;
  const formattedSuccessRate = measuredSuccessRate == null ? "100%" : formatHealthSuccessRate(displaySuccessRate);
  const healthTrend = buildDirectoryHealthTrend(trend);
  const unit = localizePriceUnit(row.priceUnit, props.locale);
  const capabilities = row.capabilities ?? [];
  const primaryCapability = pickPrimaryCapability(capabilities);
  const extraCapabilities = capabilities.filter((capability) => capability !== primaryCapability);
  const modelHref = props.locale ? localizePath(modelPublicPath(row.name), props.locale) : undefined;
  const modelTitle = (
    <span className="block truncate font-mono text-[1.08rem] font-black tracking-tight text-slate-950 underline-offset-2 group-hover/model:underline dark:text-white">
      {row.name}
    </span>
  );

  return (
    <article
      ref={ref}
      className="min-h-[214px] rounded-2xl border border-[#E1DAEC]/85 bg-white/90 p-5 shadow-[0_18px_55px_-48px_rgba(61,48,105,0.55)] transition-colors hover:border-[#C7B8F3] hover:bg-white dark:border-white/10 dark:bg-white/[0.045] dark:hover:border-violet-200/30"
      data-local-model-card={row.name}
    >
      <div className="min-w-0">
        <div className="flex min-w-0 flex-wrap items-center gap-2" data-local-card-title-row="true">
          {modelHref ? (
            <Link href={modelHref} className="group/model block min-w-0">
              {modelTitle}
            </Link>
          ) : (
            <div className="min-w-0">{modelTitle}</div>
          )}
          {primaryCapability ? <CapabilityBadge label={primaryCapability} compact /> : null}
        </div>

        <div className="mt-3 h-px bg-slate-200/80 dark:bg-white/10" data-local-card-divider="true" />

        <div className="mt-3 flex min-w-0 flex-wrap items-center gap-2" data-local-card-vendor-row="true">
          <HomeModelLogo
            iconKey={row.iconKey}
            modelName={row.vendor}
            vendor={row.vendor}
            fallback={row.vendor.charAt(0).toUpperCase()}
            surfaceSize={24}
            imageSize={14}
          />
          <span className="min-w-0 truncate text-[11px] font-black tracking-[0.11em] text-slate-500 uppercase dark:text-slate-400">
            {row.vendor}
          </span>
          <span className="shrink-0 rounded-md bg-[#EEF2FF] px-2 py-0.5 text-[11px] font-black text-[#4F46E5] dark:bg-violet-300/14 dark:text-violet-100">
            {localizedBilling(row.billing, props.locale)}
          </span>
        </div>

        <div className="mt-2 flex flex-wrap gap-1.5" data-local-card-capabilities="true">
          {extraCapabilities.length > 0 ? (
            extraCapabilities.map((capability) => <CapabilityBadge key={capability} label={capability} />)
          ) : capabilities.length === 0 ? (
            <span className="rounded-md border border-slate-200/80 bg-slate-50 px-2 py-1 text-[10px] font-bold leading-none text-slate-500 dark:border-white/10 dark:bg-white/[0.045] dark:text-slate-300">
              {props.copy.noCapabilities}
            </span>
          ) : null}
        </div>
      </div>

      <div className="mt-5 grid grid-cols-3 gap-4" data-local-model-card-metrics="true">
        <div className="min-w-0" data-local-card-health-metric="true">
          <div className="text-[11px] font-black tracking-[0.1em] text-slate-400 uppercase dark:text-slate-500">
            {props.copy.colHealth}
          </div>
          <div className="mt-2 flex min-w-0 items-center gap-2.5">
            <span className="font-mono text-sm font-semibold text-emerald-600 dark:text-emerald-400">
              {formattedSuccessRate}
            </span>
            <div className="h-6 min-w-0 flex-1">
              <DailyHealthBars points={healthTrend} label={props.copy.colHealth} heightPx={24} maxDays={HEALTH_BAR_COUNT} />
            </div>
          </div>
          <div className="mt-1 font-mono text-[10px] text-slate-400 dark:text-slate-500">
            {props.copy.latencyLabel} {formatLatencyMs(latencyMs)}
          </div>
        </div>
        <div className="min-w-0">
          <div className="text-[11px] font-black tracking-[0.1em] text-slate-400 uppercase dark:text-slate-500">
            {props.copy.colOutput}
          </div>
          <div className="mt-2 font-mono text-sm">
            <PriceCell price={row.output ?? "-"} official={row.outputOfficial} unit={row.output ? unit : undefined} align="start" />
          </div>
        </div>
        <div className="min-w-0">
          <div className="text-[11px] font-black tracking-[0.1em] text-slate-400 uppercase dark:text-slate-500">
            {props.copy.colInput}
          </div>
          <div className="mt-2 font-mono text-sm">
            <PriceCell
              price={row.input ?? row.discounted}
              official={row.inputOfficial}
              unit={unit}
              prefix={row.pricePrefix}
              align="start"
            />
          </div>
        </div>
      </div>
    </article>
  );
}

function DirectoryRow(props: {
  copy: ModelsDirectoryTableCopy;
  row: HomePricedModel;
  perf: HomePerfSummary | undefined;
  trend: HomeTrendPoint[];
  locale?: Locale;
  onVisible: () => void;
}) {
  const ref = useRef<HTMLTableRowElement>(null);
  const { onVisible } = props;

  useEffect(() => {
    const node = ref.current;
    if (!node || typeof IntersectionObserver === "undefined") {
      onVisible();
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          onVisible();
          observer.disconnect();
        }
      },
      { rootMargin: "200px" }
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [onVisible]);

  const { row, perf, trend } = props;
  const latencyMs = perf?.avg_ttft_ms && perf.avg_ttft_ms > 0 ? perf.avg_ttft_ms : trendAvgTtftMs(trend) || DEFAULT_TTFT_MS;
  const measuredSuccessRate = validSuccessRate(perf?.success_rate) ?? trendAvgSuccessRate(trend);
  const displaySuccessRate =
    measuredSuccessRate == null
      ? DEFAULT_HEALTH_SUCCESS_RATE
      : getJitteredSuccessRate(measuredSuccessRate, row.name) ?? measuredSuccessRate;
  const formattedSuccessRate = measuredSuccessRate == null ? "100%" : formatHealthSuccessRate(displaySuccessRate);
  const healthTrend = buildDirectoryHealthTrend(trend);
  const unit = localizePriceUnit(row.priceUnit, props.locale);
  const capabilities = row.capabilities ?? [];

  return (
    <tr ref={ref} className="border-b border-violet-500/8 transition-colors last:border-b-0 hover:bg-violet-500/4 dark:border-white/[0.055] dark:hover:bg-white/[0.035]">
      <td className="max-w-[320px] px-5 py-[18px]">
        {props.locale ? (
          <Link
            href={localizePath(modelPublicPath(row.name), props.locale)}
            className="flex items-center gap-2.5 hover:opacity-80"
          >
            <HomeModelLogo
              iconKey={row.iconKey}
              modelName={row.name}
              vendor={row.vendor}
              fallback={row.name.charAt(0).toUpperCase()}
              surfaceSize={28}
              imageSize={18}
            />
            <span className="min-w-0">
              <span className="block truncate font-mono text-[13px] font-semibold tracking-tight underline-offset-2 hover:underline">
                {row.name}
              </span>
            </span>
          </Link>
        ) : (
          <div className="flex items-center gap-2.5">
            <HomeModelLogo
              iconKey={row.iconKey}
              modelName={row.name}
              vendor={row.vendor}
              fallback={row.name.charAt(0).toUpperCase()}
              surfaceSize={28}
              imageSize={18}
            />
            <span className="min-w-0">
              <span className="block truncate font-mono text-[13px] font-semibold tracking-tight">{row.name}</span>
            </span>
          </div>
        )}
      </td>
      <td className="px-4 py-[18px]">
        <div className="flex min-w-[150px] items-center gap-2.5">
          <HomeModelLogo
            iconKey={row.iconKey}
            modelName={row.vendor}
            vendor={row.vendor}
            fallback={row.vendor.charAt(0).toUpperCase()}
            surfaceSize={26}
            imageSize={16}
          />
          <span className="min-w-0">
            <span className="block truncate text-sm font-bold text-slate-800 dark:text-slate-100">{row.vendor}</span>
          <span className="rounded-md border border-slate-200/80 bg-slate-50 px-2 py-0.5 text-[10px] font-bold tracking-[0.05em] text-slate-500 uppercase dark:border-white/10 dark:bg-white/[0.04] dark:text-slate-400">
            {localizedBilling(row.billing, props.locale)}
          </span>
          </span>
        </div>
      </td>
      <td className="px-4 py-[18px] text-right font-mono text-[13px]">
        <PriceCell
          price={row.input ?? row.discounted}
          official={row.inputOfficial}
          unit={unit}
          prefix={row.pricePrefix}
        />
      </td>
      <td className="px-4 py-[18px] text-right font-mono text-[13px]">
        <PriceCell price={row.output ?? "-"} official={row.outputOfficial} unit={row.output ? unit : undefined} />
      </td>
      <td className="px-4 py-[18px]">
        {capabilities.length > 0 ? (
          <div className="flex max-w-[220px] flex-wrap gap-1.5">
            {capabilities.map((capability) => (
              <CapabilityBadge key={capability} label={capability} />
            ))}
          </div>
        ) : (
          <span className="text-xs text-slate-400 dark:text-slate-500">{props.copy.noCapabilities}</span>
        )}
      </td>
      <td className="px-5 py-[18px]">
        <div className="flex items-center gap-3.5">
          <div className="h-7 w-[172px]">
            <DailyHealthBars points={healthTrend} label={props.copy.colHealth} heightPx={28} maxDays={HEALTH_BAR_COUNT} />
          </div>
          <span className="inline-flex min-w-[62px] flex-col items-end">
            <span className="font-mono text-[13px] font-semibold text-emerald-600 dark:text-emerald-400">
              {formattedSuccessRate}
            </span>
            <span className="font-mono text-[10px] text-slate-400 dark:text-slate-500">
              {props.copy.latencyLabel} {formatLatencyMs(latencyMs)}
            </span>
          </span>
        </div>
      </td>
    </tr>
  );
}

function PriceCell(props: { price: string; official?: string; unit?: string; prefix?: string; align?: "start" | "end" }) {
  const showOfficial = props.official && props.official !== props.price && props.official !== "-";
  const alignment = props.align === "start" ? "items-start" : "items-end";
  return (
    <span className={`inline-flex min-w-[76px] flex-col ${alignment} gap-0.5`}>
      <span className={props.price === "-" ? "text-slate-400 dark:text-slate-500" : "font-bold text-slate-900 tabular-nums dark:text-white"}>
        {props.prefix ? <span className="mr-1 font-sans text-[11px] font-semibold">{props.prefix}</span> : null}
        {props.price}
      </span>
      {showOfficial ? <span className="text-[10px] text-slate-400 line-through dark:text-slate-500">{props.official}</span> : null}
      {props.unit ? <span className="font-sans text-[9px] font-medium text-slate-400 normal-case dark:text-slate-500">{props.unit}</span> : null}
    </span>
  );
}

function localizePriceUnit(unit: string | undefined, locale: Locale | undefined): string | undefined {
  if (!unit || locale !== "zh") return unit;
  if (unit === "per second") return "/ 秒";
  if (unit === "per request") return "/ 次";
  if (unit === "per 1M tokens") return "/ 1M tokens";
  return unit;
}

function localizedBilling(value: string | undefined, locale: Locale | undefined): string {
  if (locale !== "zh") return value ?? "API";
  if (value === "Token") return "Token";
  if (value === "Second") return "按秒";
  if (value === "Request") return "请求";
  return value ?? "API";
}

function CapabilityBadge(props: { label: string; compact?: boolean }) {
  const tone = capabilityTone(props.label);
  return (
    <span
      className={`rounded-md border px-2 text-[10px] font-bold leading-none ${props.compact ? "py-0.5" : "py-1"} ${tone}`}
      title={props.label}
    >
      {props.label}
    </span>
  );
}

function pickPrimaryCapability(capabilities: string[]): string | undefined {
  const priority = ["Chat", "Image", "Video", "Audio", "Reasoning", "Tools", "Vision", "Responses", "Code", "API"];
  for (const label of priority) {
    if (capabilities.includes(label)) return label;
  }
  return capabilities[0];
}

function capabilityTone(label: string): string {
  const key = label.toLowerCase();
  if (key.includes("vision") || key.includes("image") || key.includes("video")) {
    return "border-sky-200/80 bg-sky-50 text-sky-700 dark:border-sky-400/20 dark:bg-sky-400/10 dark:text-sky-300";
  }
  if (key.includes("tool") || key.includes("web") || key.includes("response")) {
    return "border-teal-200/80 bg-teal-50 text-teal-700 dark:border-teal-400/20 dark:bg-teal-400/10 dark:text-teal-300";
  }
  if (key.includes("think") || key.includes("reason")) {
    return "border-amber-200/80 bg-amber-50 text-amber-700 dark:border-amber-400/20 dark:bg-amber-400/10 dark:text-amber-300";
  }
  if (key === "json" || key.includes("code")) {
    return "border-indigo-200/80 bg-indigo-50 text-indigo-700 dark:border-indigo-400/20 dark:bg-indigo-400/10 dark:text-indigo-300";
  }
  return "border-slate-200/80 bg-slate-50 text-slate-600 dark:border-white/10 dark:bg-white/[0.045] dark:text-slate-300";
}

export function buildDirectoryHealthTrend(points: HomeTrendPoint[], fallbackEndTs = currentUtcDayStartSeconds()): HomeTrendPoint[] {
  const realDays = bucketDirectoryHealthDays(points).slice(-HEALTH_BAR_COUNT);
  if (realDays.length === 0) {
    const endTs = utcDayStartSeconds(fallbackEndTs);
    return Array.from({ length: HEALTH_BAR_COUNT }, (_, index) =>
      defaultHealthPoint(endTs - (HEALTH_BAR_COUNT - 1 - index) * DAY_SECONDS)
    );
  }
  if (realDays.length >= HEALTH_BAR_COUNT) return realDays;

  const missing = HEALTH_BAR_COUNT - realDays.length;
  const firstTs = realDays[0].ts;
  const padding = Array.from({ length: missing }, (_, index) =>
    defaultHealthPoint(firstTs - (missing - index) * DAY_SECONDS)
  );
  return [...padding, ...realDays];
}

function bucketDirectoryHealthDays(points: HomeTrendPoint[]): HomeTrendPoint[] {
  const byDay = new Map<number, { successRates: number[]; ttfts: number[] }>();
  for (const point of points) {
    if (!Number.isFinite(point.ts)) continue;
    const dayTs = utcDayStartSeconds(point.ts);
    const bucket = byDay.get(dayTs) ?? { successRates: [], ttfts: [] };
    if (validSuccessRate(point.success_rate) != null) bucket.successRates.push(point.success_rate);
    if (Number.isFinite(point.avg_ttft_ms) && point.avg_ttft_ms > 0) bucket.ttfts.push(point.avg_ttft_ms);
    byDay.set(dayTs, bucket);
  }

  return [...byDay.entries()]
    .sort(([a], [b]) => a - b)
    .filter(([, bucket]) => bucket.successRates.length > 0)
    .map(([ts, bucket]) => ({
      ts,
      success_rate: average(bucket.successRates),
      avg_ttft_ms: bucket.ttfts.length > 0 ? average(bucket.ttfts) : DEFAULT_TTFT_MS,
    }));
}

function defaultHealthPoint(ts: number): HomeTrendPoint {
  return { ts, success_rate: DEFAULT_HEALTH_SUCCESS_RATE, avg_ttft_ms: DEFAULT_TTFT_MS };
}

function currentUtcDayStartSeconds(): number {
  return utcDayStartSeconds(Date.now() / 1000);
}

function utcDayStartSeconds(ts: number): number {
  return Math.floor(ts / DAY_SECONDS) * DAY_SECONDS;
}

function average(values: number[]): number {
  return values.length > 0 ? values.reduce((sum, value) => sum + value, 0) / values.length : 0;
}

function trendAvgSuccessRate(points: HomeTrendPoint[]): number | undefined {
  const values = points.map((point) => point.success_rate).filter((value) => validSuccessRate(value) != null);
  return values.length > 0 ? average(values) : undefined;
}

function validSuccessRate(value: number | undefined): number | undefined {
  return value != null && Number.isFinite(value) && value >= 0 ? value : undefined;
}
