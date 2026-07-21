import { StatusPage, type StatusPageFilters } from "@/components/status-page";
import { getStatusCopy, type StatusFreshness } from "@/lib/status-copy";
import {
  STATUS_REVALIDATE_SECONDS,
  fetchStatusIncidents,
  fetchStatusMaintenance,
  fetchStatusSummary,
  type StatusValue,
} from "@/lib/status";
import { buildMetadata } from "@/lib/seo";

const copy = getStatusCopy("en");

export const revalidate = STATUS_REVALIDATE_SECONDS;
export const metadata = buildMetadata({
  title: copy.title,
  description: copy.description,
  pathname: "/status",
});

type Props = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function Page(props: Props) {
  const [searchParams, summary, incidents, maintenance] = await Promise.all([
    props.searchParams,
    fetchStatusSummary(),
    fetchStatusIncidents(),
    fetchStatusMaintenance(),
  ]);

  return (
    <StatusPage
      locale="en"
      summary={summary.data}
      freshness={combineFreshness(summary.state, incidents.state, maintenance.state)}
      incidents={incidents.data?.incidents ?? []}
      maintenance={maintenance.data?.maintenance ?? []}
      filters={parseFilters(searchParams)}
    />
  );
}

function combineFreshness(...states: StatusFreshness[]): StatusFreshness {
  if (states.includes("monitoring-unavailable")) return "monitoring-unavailable";
  return states.includes("stale") ? "stale" : "fresh";
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
