"use client";

import { useEffect, useState } from "react";
import { fetchHealthSummary, type HomePerfSummary } from "@/lib/home-live";

const DEFAULT_SUCCESS_RATE = 100;
const HEALTH_SIGNAL_HEIGHTS = [7, 10, 13, 16, 19] as const;

let healthSummaryPromise: Promise<Record<string, HomePerfSummary>> | null = null;

function loadHealthSummary() {
  healthSummaryPromise ??= fetchHealthSummary();
  return healthSummaryPromise;
}

export function HomePriceHealthScore(props: { label: string; modelName: string }) {
  const [summary, setSummary] = useState<HomePerfSummary | undefined>();

  useEffect(() => {
    let cancelled = false;
    loadHealthSummary().then((data) => {
      if (!cancelled) setSummary(data[props.modelName]);
    });
    return () => {
      cancelled = true;
    };
  }, [props.modelName]);

  const successRate = validSuccessRate(summary?.success_rate) ?? DEFAULT_SUCCESS_RATE;
  const formatted = formatDirectorySuccessRate(successRate);

  return (
    <span
      className="fk-home-health"
      role="img"
      aria-label={`${props.label}: ${formatted}`}
      title={`${props.label}: ${formatted}`}
    >
      <span aria-hidden className="fk-home-health-bars">
        {HEALTH_SIGNAL_HEIGHTS.map((height, index) => (
          <span key={`health-signal-${index}`} style={{ height }} />
        ))}
      </span>
      <span>{formatted}</span>
    </span>
  );
}

function validSuccessRate(value: number | undefined): number | undefined {
  return value != null && Number.isFinite(value) && value > 0 ? value : undefined;
}

function formatDirectorySuccessRate(value: number): string {
  if (!Number.isFinite(value)) return "100%";
  if (value === DEFAULT_SUCCESS_RATE) return "100%";
  const digits = value >= 99.95 ? 1 : value >= 99 ? 2 : 1;
  return `${value.toFixed(digits)}%`;
}
