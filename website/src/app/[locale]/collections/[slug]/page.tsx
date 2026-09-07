import { notFound } from "next/navigation";
import { ModelCollectionDetail } from "@/components/model-collections-page";
import { getModelCollection, getModelCollectionSeoDescription, getModelCollectionCopy, MODEL_COLLECTIONS, selectCollectionModels } from "@/lib/model-collections";
import { isLocale, type Locale, LOCALES } from "@/lib/locales";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { fetchRankingsData } from "@/lib/rankings-live";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ locale: string; slug: string }> };

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").flatMap((locale) => MODEL_COLLECTIONS.map((collection) => ({ locale, slug: collection.slug })));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") return {};
  const collection = getModelCollection(params.slug);
  if (!collection) return {};
  const copy = getModelCollectionCopy(collection, params.locale as Locale);
  return buildMetadata({ title: `${copy.title} | Flatkey`, description: getModelCollectionSeoDescription(collection, params.locale as Locale), pathname: `/collections/${collection.slug}`, locale: params.locale as Locale, absoluteTitle: true });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const collection = getModelCollection(params.slug);
  if (!collection) notFound();
  const [pricing, rankings] = await Promise.all([getPricingData(WEBSITE_PUBLIC_PRICING_GROUP), fetchRankingsData()]);
  if (selectCollectionModels(collection, pricing.models, 1).length === 0) notFound();
  return <ModelCollectionDetail locale={params.locale as Locale} collection={collection} pricing={pricing} rankings={rankings} />;
}
