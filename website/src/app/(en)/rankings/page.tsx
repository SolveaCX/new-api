import { RankingsPage, rankingsMetadataCopy } from "@/components/rankings-page";
import { normalizeRankingPeriod, getWebsiteRankingsData } from "@/lib/rankings";
import { buildMetadata } from "@/lib/seo";

const metadataCopy = rankingsMetadataCopy("en");

export const metadata = buildMetadata({
  title: metadataCopy.title,
  description: metadataCopy.description,
  pathname: "/rankings",
});

type Props = { searchParams?: Promise<Record<string, string | string[] | undefined>> };

export default async function Page(props: Props) {
  const searchParams = await props.searchParams;
  const period = normalizeRankingPeriod(searchParams?.period);
  const rankings = await getWebsiteRankingsData(period);
  return <RankingsPage locale="en" rankings={rankings} />;
}
