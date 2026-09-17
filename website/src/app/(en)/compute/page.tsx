import { ComputeMarketPage } from "@/components/compute-market-page";
import { getComputeMarketCopy } from "@/lib/compute-market-copy";
import { staticFeaturePages } from "@/lib/static-feature-pages";
import { buildMetadata } from "@/lib/seo";

const page = staticFeaturePages.compute;

export const metadata = buildMetadata({
  title: getComputeMarketCopy("en").metaTitle,
  description: getComputeMarketCopy("en").metaDescription,
  pathname: page.pathname,
});

export default function Page() {
  return <ComputeMarketPage locale="en" />;
}
