import { notFound } from "next/navigation";
import { StatusModelPage } from "@/components/status-model-page";
import { isLocale, LOCALES } from "@/lib/locales";
import { getStatusCopy } from "@/lib/status-copy";
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
  params: Promise<{ locale: string; slug: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const { locale, slug } = await props.params;
  if (!isLocale(locale) || locale === "en") return {};
  const history = await fetchStatusComponentHistory(slug, "90d");
  const displayName = history.data?.component.display_name ?? displayNameFromSlug(slug);
  const copy = getStatusCopy(locale);
  return buildMetadata({
    title: `${displayName} — ${copy.title}`,
    description: copy.description,
    pathname: `/status/models/${encodeURIComponent(slug)}`,
    locale,
  });
}

export default async function Page(props: Props) {
  const [{ locale, slug }, searchParams] = await Promise.all([props.params, props.searchParams]);
  if (!isLocale(locale) || locale === "en") notFound();
  const range = parseRange(first(searchParams?.range));
  const [history, incidents] = await Promise.all([
    fetchStatusComponentHistory(slug, range),
    fetchStatusIncidents(),
  ]);
  if (history.state === "not-found") notFound();

  return (
    <StatusModelPage
      locale={locale}
      history={history.data ?? unavailableHistory(slug, range)}
      freshness={history.state}
      incidents={incidents.data?.incidents ?? []}
      incidentFreshness={incidents.state === "not-found" ? "monitoring-unavailable" : incidents.state}
      selectedRange={range}
    />
  );
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
