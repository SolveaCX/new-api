import { StatusModelPage } from "@/components/status-model-page";
import { getStatusCopy, type StatusFreshness } from "@/lib/status-copy";
import {
  STATUS_REVALIDATE_SECONDS,
  fetchStatusComponentHistory,
  fetchStatusIncidents,
  type StatusComponentHistoryData,
  type StatusHistoryRange,
} from "@/lib/status";
import { buildMetadata } from "@/lib/seo";

export const revalidate = STATUS_REVALIDATE_SECONDS;

type Props = {
  params: Promise<{ slug: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export async function generateMetadata(props: Props) {
  const { slug } = await props.params;
  const history = await fetchStatusComponentHistory(slug, "90d");
  const displayName = history.data?.component.display_name ?? displayNameFromSlug(slug);
  const copy = getStatusCopy("en");
  return buildMetadata({
    title: `${displayName} — ${copy.title}`,
    description: copy.description,
    pathname: `/status/models/${encodeURIComponent(slug)}`,
  });
}

export default async function Page(props: Props) {
  const [{ slug }, searchParams] = await Promise.all([props.params, props.searchParams]);
  const range = parseRange(first(searchParams?.range));
  const [history, incidents] = await Promise.all([
    fetchStatusComponentHistory(slug, range),
    fetchStatusIncidents(),
  ]);
  const freshness = combineFreshness(history.state, incidents.state);

  return (
    <StatusModelPage
      locale="en"
      history={history.data ?? unavailableHistory(slug, range)}
      freshness={freshness}
      incidents={incidents.data?.incidents ?? []}
      selectedRange={range}
    />
  );
}

function combineFreshness(...states: StatusFreshness[]): StatusFreshness {
  if (states.includes("monitoring-unavailable")) return "monitoring-unavailable";
  return states.includes("stale") ? "stale" : "fresh";
}

function parseRange(value: string | undefined): StatusHistoryRange {
  return value === "24h" || value === "7d" || value === "30d" || value === "90d" ? value : "90d";
}

function first(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

function unavailableHistory(slug: string, range: StatusHistoryRange): StatusComponentHistoryData {
  return {
    generated_at: Math.floor(Date.now() / 1_000),
    last_trustworthy_update_at: 0,
    coverage: 0,
    range,
    component: {
      id: 0,
      slug,
      kind: "model",
      display_name: displayNameFromSlug(slug),
      lifecycle: "active",
      status: "unknown",
      last_trustworthy_update_at: 0,
      coverage: 0,
    },
    availability: {
      availability_micros: 0,
      coverage_micros: 0,
      known_bucket_count: 0,
      unknown_bucket_count: 0,
      maintenance_bucket_count: 0,
    },
    periods: [],
  };
}

function displayNameFromSlug(slug: string): string {
  return slug.replaceAll("-", " ");
}
