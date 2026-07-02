import { notFound } from "next/navigation";
import { RankingsPage, rankingsMetadataCopy } from "@/components/rankings-page";
import { isLocale, LOCALES } from "@/lib/locales";
import { getWebsiteRankingsData, normalizeRankingPeriod } from "@/lib/rankings";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string }>;
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};
const pathname = "/rankings";

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const metadataCopy = rankingsMetadataCopy(params.locale);
  return buildMetadata({
    title: metadataCopy.title,
    description: metadataCopy.description,
    pathname,
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const searchParams = await props.searchParams;
  const period = normalizeRankingPeriod(searchParams?.period);
  const rankings = await getWebsiteRankingsData(period);
  return <RankingsPage locale={params.locale} rankings={rankings} />;
}
