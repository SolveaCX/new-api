import { notFound } from "next/navigation";
import { StatusPage, type StatusPageFilters } from "@/components/status-page";
import { isLocale, LOCALES } from "@/lib/locales";
import { getStatusCopy } from "@/lib/status-copy";
import {
  STATUS_REVALIDATE_SECONDS,
  fetchStatusIncidents,
  fetchStatusMaintenance,
  fetchStatusSummary,
  type StatusValue,
} from "@/lib/status";
import { buildMetadata } from "@/lib/seo";

export const revalidate = STATUS_REVALIDATE_SECONDS;

type Props = {
  params: Promise<{ locale: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const { locale } = await props.params;
  if (!isLocale(locale) || locale === "en") return {};
  const copy = getStatusCopy(locale);
  return buildMetadata({ title: copy.title, description: copy.description, pathname: "/status", locale });
}

export default async function Page(props: Props) {
  const [{ locale }, searchParams, summary, incidents, maintenance] = await Promise.all([
    props.params,
    props.searchParams,
    fetchStatusSummary(),
    fetchStatusIncidents(),
    fetchStatusMaintenance(),
  ]);
  if (!isLocale(locale) || locale === "en") notFound();

  return (
    <StatusPage
      locale={locale}
      summary={summary.data}
      freshness={summary.state}
      incidents={incidents.data?.incidents ?? []}
      incidentFreshness={incidents.state === "not-found" ? "monitoring-unavailable" : incidents.state}
      maintenance={maintenance.data?.maintenance ?? []}
      maintenanceFreshness={maintenance.state === "not-found" ? "monitoring-unavailable" : maintenance.state}
      filters={parseFilters(searchParams)}
    />
  );
}

function parseFilters(searchParams?: Record<string, string | string[] | undefined>): StatusPageFilters {
  const status = first(searchParams?.status);
  return {
    query: first(searchParams?.query),
    capability: first(searchParams?.capability),
    status: isStatusValue(status) ? status : "",
  };
}

function first(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

function isStatusValue(value: string | undefined): value is StatusValue {
  return value === "operational" || value === "degraded" || value === "outage" || value === "unknown" || value === "maintenance";
}
