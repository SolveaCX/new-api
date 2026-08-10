"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { DailyHealthBars } from "@/components/home-health-bars";
import { ModelLogo } from "@/components/pricing-model-browser";
import type { HomeCopy } from "@/lib/home-copy";
import {
  fetchHealthSummary,
  fetchModelTrend,
  formatLatencyMs,
  formatSuccessRate,
  trendAvgTtftMs,
  type HomePerfSummary,
  type HomeTrendPoint,
} from "@/lib/home-live";
import type { HomePricedModel } from "@/lib/home-models";
import { localizePath, type Locale } from "@/lib/locales";
import { modelPublicPath } from "@/lib/model-public";

type Props = {
  copy: HomeCopy["table"];
  rows: HomePricedModel[];
  locale?: Locale;
};

// /models directory: every priced model as one row — official price struck
// through vs the after-bonus price (the hero number), TTFT latency, and a
// 30-day health bar wall. Health series load lazily as rows scroll into view
// so 40+ rows do not fan out 40+ upfront requests.
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

  return (
    <div className="fk-model-table overflow-x-auto rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]">
      <table className="w-full min-w-[760px] border-collapse text-sm">
        <thead>
          <tr className="border-b-2 border-[#101014] bg-[#F7F4EC]/72 text-left text-[11px] font-black tracking-normal text-[#5C5861] uppercase dark:border-white/20 dark:bg-white/[0.045] dark:text-white/62">
            <th className="px-5 py-3.5 font-bold">{props.copy.colModel}</th>
            <th className="px-3 py-3.5 text-right font-bold">
              {props.copy.colOfficial}
              <span className="block text-[9px] font-semibold text-[#6D6A72] normal-case dark:text-white/42">{props.copy.perMillion}</span>
            </th>
            <th className="px-3 py-3.5 text-right font-black text-[#7C3AED] dark:text-[#C8A8FF]">
              {props.copy.colFlatkey}
              <span className="block text-[9px] font-semibold text-[#6D6A72] normal-case dark:text-white/42">{props.copy.perMillion}</span>
            </th>
            <th className="px-3 py-3.5 text-right font-bold">{props.copy.colLatency}</th>
            <th className="w-[220px] px-5 py-3.5 text-left font-bold">{props.copy.colHealth}</th>
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => (
            <DirectoryRow
              key={row.name}
              row={row}
              perf={summary[row.name]}
              trend={trends[row.name] ?? []}
              healthLabel={props.copy.colHealth}
              locale={props.locale}
              onVisible={() => loadTrend(row.name)}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DirectoryRow(props: {
  row: HomePricedModel;
  perf: HomePerfSummary | undefined;
  trend: HomeTrendPoint[];
  healthLabel: string;
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
  return (
    <tr ref={ref} className="fk-price-row border-b border-[#101014]/10 transition-colors last:border-b-0 hover:bg-[#F9F871]/18 dark:border-white/10 dark:hover:bg-white/[0.06]">
      <td className="max-w-[280px] px-5 py-3">
        {props.locale ? (
          <Link
            href={localizePath(modelPublicPath(row.name), props.locale)}
            className="flex min-w-0 items-center gap-2.5 hover:opacity-90"
          >
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border-2 border-[#101014] bg-white shadow-[2px_2px_0_#101014] dark:border-white/24 dark:bg-white/8 dark:shadow-[2px_2px_0_rgba(255,255,255,0.16)]">
              <ModelLogo iconKey={row.iconKey} fallback={row.name.charAt(0).toUpperCase()} size={18} />
            </span>
            <span className="min-w-0">
              <span className="fk-price-row-text block truncate font-mono text-[13px] font-black tracking-normal text-[#101014] underline-offset-2 hover:underline dark:text-white">
                {row.name}
              </span>
              <span className="block text-[11px] font-semibold text-[#6D6A72] dark:text-white/50">{row.vendor}</span>
            </span>
          </Link>
        ) : (
          <div className="flex items-center gap-2.5">
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border-2 border-[#101014] bg-white shadow-[2px_2px_0_#101014] dark:border-white/24 dark:bg-white/8 dark:shadow-[2px_2px_0_rgba(255,255,255,0.16)]">
              <ModelLogo iconKey={row.iconKey} fallback={row.name.charAt(0).toUpperCase()} size={18} />
            </span>
            <span className="min-w-0">
              <span className="fk-price-row-text block truncate font-mono text-[13px] font-black tracking-normal text-[#101014] dark:text-white">{row.name}</span>
              <span className="block text-[11px] font-semibold text-[#6D6A72] dark:text-white/50">{row.vendor}</span>
            </span>
          </div>
        )}
      </td>
      <td className="px-3 py-3 text-right font-mono text-[13px] text-[#6D6A72] line-through dark:text-white/48">{row.official}</td>
      <td className="fk-price-row-strong px-3 py-3 text-right font-mono text-[13px] font-black text-emerald-600 dark:text-emerald-400">{row.discounted}</td>
      <td className="px-3 py-3 text-right font-mono text-[13px] font-semibold text-[#101014] dark:text-white/82">{formatLatencyMs(perf?.avg_ttft_ms || trendAvgTtftMs(trend))}</td>
      <td className="px-5 py-3">
        <div className="flex items-center gap-3">
          <div className="h-7 w-[140px]">
            {trend.length > 1 ? <DailyHealthBars points={trend} label={props.healthLabel} heightPx={28} /> : null}
          </div>
          <span className="font-mono text-[13px] font-black text-emerald-600 dark:text-emerald-400">
            {formatSuccessRate(perf?.success_rate)}
          </span>
        </div>
      </td>
    </tr>
  );
}
