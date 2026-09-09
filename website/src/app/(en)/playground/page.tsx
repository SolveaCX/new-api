import { StaticFeaturePage } from "@/components/static-feature-page";
import { buildMetadata, getSeoLocaleOptions } from "@/lib/seo";
import { staticFeaturePages } from "@/lib/static-feature-pages";

const page = staticFeaturePages.playground;

type Props = { searchParams?: Promise<Record<string, string | string[] | undefined>> };

export async function generateMetadata(props: Props) {
  const searchParams = await props.searchParams;
  return buildMetadata({
    title: page.metadataTitle,
    description: page.metadataDescription,
    pathname: page.pathname,
    ...getSeoLocaleOptions(page.pathname, "en"),
    noIndex: Object.keys(searchParams ?? {}).length > 0,
  });
}

export default function Page() {
  return <StaticFeaturePage pageKey="playground" locale="en" />;
}
