import { notFound } from "next/navigation";
import { ModelCollectionDetail } from "@/components/model-collections-page";
import { getModelCollection, getModelCollectionSeoDescription, getModelCollectionCopy, MODEL_COLLECTIONS, selectCollectionModels } from "@/lib/model-collections";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { fetchRankingsData } from "@/lib/rankings-live";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ slug: string }> };

export function generateStaticParams() {
  return MODEL_COLLECTIONS.map((collection) => ({ slug: collection.slug }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  const collection = getModelCollection(params.slug);
  if (!collection) return {};
  const copy = getModelCollectionCopy(collection, "en");
  return buildMetadata({ title: `${copy.title} | Flatkey`, description: getModelCollectionSeoDescription(collection, "en"), pathname: `/collections/${collection.slug}`, absoluteTitle: true });
}

export default async function Page(props: Props) {
  const params = await props.params;
  const collection = getModelCollection(params.slug);
  if (!collection) notFound();
  const [pricing, rankings] = await Promise.all([getPricingData(WEBSITE_PUBLIC_PRICING_GROUP), fetchRankingsData()]);
  if (selectCollectionModels(collection, pricing.models, 1).length === 0) notFound();
  return <ModelCollectionDetail locale="en" collection={collection} pricing={pricing} rankings={rankings} />;
}
