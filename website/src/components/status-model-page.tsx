import { SiteShell } from "@/components/site-shell";
import { StatusHistoryBars } from "@/components/status-history-bars";
import { StatusIndicator, formatMicros, formatTimestamp } from "@/components/status-page";
import type { Locale } from "@/lib/locales";
import { localizePath } from "@/lib/locales";
import { getStatusCopy, getStatusPresentation, type StatusFreshness } from "@/lib/status-copy";
import type { StatusComponentHistoryData, StatusHistoryRange, StatusIncident } from "@/lib/status";

interface StatusModelPageProps {
  locale: Locale;
  history: StatusComponentHistoryData;
  freshness: StatusFreshness;
  incidents: StatusIncident[];
  selectedRange: StatusHistoryRange;
}

export function StatusModelPage({ locale, history, freshness, incidents, selectedRange }: StatusModelPageProps) {
  const copy = getStatusCopy(locale);
  const component = history.component;
  const relatedIncidents = incidents.filter((incident) => incident.component_ids.includes(component.id));
  const current = getStatusPresentation({ locale, status: component.status, freshness, lifecycle: "active" });
  const lifecycle = component.lifecycle === "retired"
    ? getStatusPresentation({ locale, status: component.status, freshness, lifecycle: component.lifecycle })
    : null;
  const overviewPath = localizePath("/status", locale);
  const pagePath = localizePath(`/status/models/${encodeURIComponent(component.slug)}`, locale);

  return (
    <SiteShell locale={locale} pathname={pagePath}>
      <div className="min-h-screen bg-slate-50 py-12 dark:bg-slate-950 sm:py-16">
        <div className="mx-auto w-full max-w-5xl space-y-8 px-4 sm:px-6 lg:px-8">
          <header>
            <a href={overviewPath} className="text-sm font-semibold text-blue-700 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-blue-300">← {copy.model.backLabel}</a>
            <div className="mt-5 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <h1 className="text-4xl font-black tracking-tight text-slate-950 dark:text-white sm:text-5xl">{component.display_name}</h1>
                {component.capability ? <p className="mt-2 text-slate-600 dark:text-slate-300">{component.capability}</p> : null}
              </div>
              <div className="flex flex-wrap gap-2">
                <StatusIndicator presentation={current} />
                {lifecycle ? <StatusIndicator presentation={lifecycle} /> : null}
              </div>
            </div>
          </header>

          <section aria-labelledby="model-evidence-title" className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-6">
            <h2 id="model-evidence-title" className="text-xl font-bold text-slate-950 dark:text-white">{copy.model.evidenceLabel}</h2>
            <dl className="mt-5 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
              <Metric term={copy.model.currentStatusLabel} detail={current.text} />
              <Metric term={copy.history.availabilityLabel} detail={formatMicros(history.availability.availability_micros)} />
              <Metric term={copy.history.coverageLabel} detail={formatMicros(history.availability.coverage_micros)} />
              <Metric term={copy.history.incidentCountLabel} detail={String(relatedIncidents.length)} />
            </dl>
            <p className="mt-5 text-sm text-slate-500 dark:text-slate-400">{copy.lastTrustworthyUpdateLabel}: {formatTimestamp(history.last_trustworthy_update_at, locale)}</p>
          </section>

          <StatusHistoryBars locale={locale} componentSlug={component.slug} periods={history.periods} selectedRange={selectedRange} endAt={history.generated_at} />

          <section aria-labelledby="model-incident-title" className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-6">
            <h2 id="model-incident-title" className="text-xl font-bold text-slate-950 dark:text-white">{copy.incidents.title}</h2>
            {relatedIncidents.length === 0 ? <p className="mt-4 text-slate-600 dark:text-slate-300">{copy.incidents.empty}</p> : (
              <ol className="mt-5 space-y-6 border-l-2 border-slate-200 pl-5 dark:border-slate-700">
                {relatedIncidents.map((incident) => (
                  <li key={incident.id}>
                    <h3 className="font-bold text-slate-950 dark:text-white">{incident.title}</h3>
                    <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{formatTimestamp(incident.updated_at, locale)}</p>
                    <ol className="mt-3 space-y-3">
                      {incident.updates.map((update, index) => (
                        <li key={`${incident.id}-${update.published_at}-${index}`} className="rounded-lg bg-slate-50 p-3 dark:bg-slate-950">
                          <p className="text-xs font-bold uppercase tracking-wide text-slate-500 dark:text-slate-400">{update.state}</p>
                          <p className="mt-1 text-sm text-slate-700 dark:text-slate-200">{update.body}</p>
                          <time dateTime={new Date(update.published_at * 1_000).toISOString()} className="mt-1 block text-xs text-slate-500">{formatTimestamp(update.published_at, locale)}</time>
                        </li>
                      ))}
                    </ol>
                  </li>
                ))}
              </ol>
            )}
          </section>
        </div>
      </div>
    </SiteShell>
  );
}

function Metric({ term, detail }: { term: string; detail: string }) {
  return <div><dt className="text-sm font-semibold text-slate-500 dark:text-slate-400">{term}</dt><dd className="mt-2 text-lg font-bold text-slate-950 dark:text-white">{detail}</dd></div>;
}
