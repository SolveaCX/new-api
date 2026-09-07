import { ModelCollectionsIndex } from "@/components/model-collections-page";
import { getModelCollectionsSeoCopy } from "@/lib/model-collections";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { buildMetadata } from "@/lib/seo";

const seoCopy = getModelCollectionsSeoCopy("en");

export const metadata = buildMetadata({
  title: seoCopy.title,
  description: seoCopy.description,
  pathname: "/collections",
  absoluteTitle: true,
});

export default async function Page() {
  const pricing = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  return <ModelCollectionsIndex locale="en" pricing={pricing} />;
}
