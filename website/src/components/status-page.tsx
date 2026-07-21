import { AlertTriangle, Archive, CheckCircle2, CircleHelp, Wrench, XCircle, type LucideIcon } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { StatusSubscribe } from "@/components/status-subscribe";
import type { Locale } from "@/lib/locales";
import { localizePath } from "@/lib/locales";
import {
  getStatusCopy,
  getStatusPresentation as buildStatusPresentation,
  type StatusFreshness,
  type StatusPresentation,
  type StatusPresentationInput,
} from "@/lib/status-copy";
import type { StatusComponent, StatusIncident, StatusOverallValue, StatusSummary, StatusValue } from "@/lib/status";

export interface StatusPageFilters {
  query?: string;
  capability?: string;
  status?: StatusValue | "";
}

interface StatusPageProps {
  locale: Locale;
  summary: StatusSummary;
  freshness: StatusFreshness;
  incidents: StatusIncident[];
  incidentFreshness?: StatusFreshness;
  maintenance: StatusIncident[];
  maintenanceFreshness?: StatusFreshness;
  filters?: StatusPageFilters;
}

const ICONS: Record<StatusPresentation["icon"], LucideIcon> = {
  check: CheckCircle2,
  warning: AlertTriangle,
  outage: XCircle,
  unknown: CircleHelp,
  maintenance: Wrench,
  retired: Archive,
};

export const getStatusPresentation = buildStatusPresentation;

export function filterStatusComponents(components: StatusComponent[], filters: StatusPageFilters): StatusComponent[] {
  const query = filters.query?.trim().toLocaleLowerCase() ?? "";
  return sortStatusComponents(components.filter((component) => {
    if (query && !`${component.display_name} ${component.slug}`.toLocaleLowerCase().includes(query)) return false;
    if (filters.capability && component.capability !== filters.capability) return false;
    if (filters.status && component.status !== filters.status) return false;
    return true;
  }));
}

export function sortStatusComponents(components: StatusComponent[]): StatusComponent[] {
  return [...components].sort((left, right) => {
    const leftRouter = left.kind === "router" || left.slug.toLocaleLowerCase() === "router";
    const rightRouter = right.kind === "router" || right.slug.toLocaleLowerCase() === "router";
    if (leftRouter !== rightRouter) return leftRouter ? -1 : 1;
    return left.display_name.localeCompare(right.display_name);
  });
}

export function StatusPage({
  locale,
  summary,
  freshness,
  incidents,
  incidentFreshness = "fresh",
  maintenance,
  maintenanceFreshness = "fresh",
  filters = {},
}: StatusPageProps) {
  const copy = getStatusCopy(locale);
  const components = filterStatusComponents(summary.components, filters);
  const capabilities = Array.from(new Set(summary.components.map((component) => component.capability).filter(Boolean) as string[])).sort();
  const overall = buildStatusPresentation({
    locale,
    freshness,
    lifecycle: "active",
    status: overallComponentStatus(summary.status),
  });

  return (
    <SiteShell locale={locale} pathname={localizePath("/status", locale)}>
      <div className="min-h-screen bg-slate-50 py-12 dark:bg-slate-950 sm:py-16">
        <div className="mx-auto w-full max-w-6xl space-y-8 px-4 sm:px-6 lg:px-8">
          <header className="space-y-4">
            <h1 className="text-4xl font-black tracking-tight text-slate-950 dark:text-white sm:text-5xl">{copy.title}</h1>
            <p className="max-w-3xl text-lg leading-8 text-slate-600 dark:text-slate-300">{copy.description}</p>
          </header>

          <section aria-labelledby="overall-status-title" className="grid gap-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:grid-cols-3 sm:p-6">
            <div>
              <h2 id="overall-status-title" className="text-sm font-semibold text-slate-500 dark:text-slate-400">{copy.overallLabel}</h2>
              <div className="mt-2"><StatusIndicator presentation={overall} /></div>
            </div>
            <Metric label={copy.coverageLabel} value={formatMicros(summary.coverage)} />
            <Metric label={copy.lastTrustworthyUpdateLabel} value={formatTimestamp(summary.last_trustworthy_update_at, locale)} />
          </section>

          <section aria-labelledby="status-components-title" className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-6">
            <h2 id="status-components-title" className="text-2xl font-bold text-slate-950 dark:text-white">{copy.componentsTitle}</h2>
            <form method="get" className="mt-5 grid gap-4 md:grid-cols-[2fr_1fr_1fr_auto] md:items-end">
              <label className="grid gap-1.5 text-sm font-semibold text-slate-700 dark:text-slate-200">
                {copy.filters.nameLabel}
                <input name="query" defaultValue={filters.query} placeholder={copy.filters.namePlaceholder} maxLength={100} className="rounded-lg border border-slate-300 bg-white px-3 py-2 font-normal text-slate-950 outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-slate-700 dark:bg-slate-950 dark:text-white" />
              </label>
              <label className="grid gap-1.5 text-sm font-semibold text-slate-700 dark:text-slate-200">
                {copy.filters.capabilityLabel}
                <select name="capability" defaultValue={filters.capability ?? ""} className="rounded-lg border border-slate-300 bg-white px-3 py-2 font-normal text-slate-950 outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-slate-700 dark:bg-slate-950 dark:text-white">
                  <option value="">{copy.filters.allCapabilities}</option>
                  {capabilities.map((capability) => <option key={capability} value={capability}>{capability}</option>)}
                </select>
              </label>
              <label className="grid gap-1.5 text-sm font-semibold text-slate-700 dark:text-slate-200">
                {copy.filters.statusLabel}
                <select name="status" defaultValue={filters.status ?? ""} className="rounded-lg border border-slate-300 bg-white px-3 py-2 font-normal text-slate-950 outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-slate-700 dark:bg-slate-950 dark:text-white">
                  <option value="">{copy.filters.allStatuses}</option>
                  {(Object.keys(copy.states) as StatusValue[]).map((status) => <option key={status} value={status}>{copy.states[status]}</option>)}
                </select>
              </label>
              <button type="submit" className="rounded-lg bg-slate-950 px-4 py-2 font-semibold text-white outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:bg-white dark:text-slate-950">{copy.filters.applyLabel}</button>
            </form>

            <ul className="mt-6 divide-y divide-slate-200 dark:divide-slate-800">
              {components.map((component) => {
                const presentation = buildStatusPresentation({ locale, status: component.status, freshness, lifecycle: component.lifecycle });
                return (
                  <li key={component.id} className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <a href={localizePath(`/status/models/${encodeURIComponent(component.slug)}`, locale)} className="font-bold text-slate-950 outline-none hover:text-blue-700 focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-white dark:hover:text-blue-300">{component.display_name}</a>
                      {component.capability ? <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{component.capability}</p> : null}
                    </div>
                    <StatusIndicator presentation={presentation} />
                  </li>
                );
              })}
            </ul>
          </section>

          <div className="grid gap-6 lg:grid-cols-2">
            <IncidentList title={copy.incidents.title} empty={copy.incidents.empty} incidents={incidents} freshness={incidentFreshness} locale={locale} />
            <IncidentList title={copy.maintenance.title} empty={copy.maintenance.empty} incidents={maintenance} freshness={maintenanceFreshness} locale={locale} />
          </div>

          <StatusSubscribe locale={locale} components={sortStatusComponents(summary.components)} />
        </div>
      </div>
    </SiteShell>
  );
}

export function StatusIndicator({ presentation }: { presentation: StatusPresentation }) {
  const Icon = ICONS[presentation.icon];
  return (
    <span className={`inline-flex items-center gap-2 rounded-full border px-3 py-1 text-sm font-semibold ${presentation.colorClass}`}>
      <Icon aria-hidden="true" className="size-4" />
      <span>{presentation.text}</span>
    </span>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return <div><p className="text-sm font-semibold text-slate-500 dark:text-slate-400">{label}</p><p className="mt-2 text-lg font-bold text-slate-950 dark:text-white">{value}</p></div>;
}

function IncidentList({
  title,
  empty,
  incidents,
  freshness,
  locale,
}: {
  title: string;
  empty: string;
  incidents: StatusIncident[];
  freshness: StatusFreshness;
  locale: Locale;
}) {
  const freshnessMessage = freshness === "stale"
    ? getStatusCopy(locale).freshness.stale
    : getStatusCopy(locale).freshness.unavailable;
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-6">
      <h2 className="text-xl font-bold text-slate-950 dark:text-white">{title}</h2>
      {freshness !== "fresh" ? <p className="mt-4 text-slate-600 dark:text-slate-300">{freshnessMessage}</p> : null}
      {incidents.length === 0 ? (freshness === "fresh" ? <p className="mt-4 text-slate-600 dark:text-slate-300">{empty}</p> : null) : (
        <ul className="mt-4 space-y-4">
          {incidents.map((incident) => (
            <li key={incident.id} className="border-l-2 border-slate-300 pl-4 dark:border-slate-700">
              <h3 className="font-semibold text-slate-950 dark:text-white">{incident.title}</h3>
              <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{formatTimestamp(incident.updated_at, locale)}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function overallComponentStatus(status: StatusOverallValue): StatusValue {
  if (status === "major_outage") return "outage";
  if (status === "some_systems_affected" || status === "degraded_performance") return "degraded";
  if (status === "maintenance") return "maintenance";
  if (status === "all_systems_operational") return "operational";
  return "unknown";
}

export function formatMicros(value: number): string {
  return `${(value / 10_000).toFixed(2)}%`;
}

export function formatTimestamp(timestamp: number, locale: Locale): string {
  if (!timestamp) return getStatusCopy(locale).states.unknown;
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short", timeZone: "UTC" }).format(new Date(timestamp * 1_000));
}

export type { StatusPresentationInput };
