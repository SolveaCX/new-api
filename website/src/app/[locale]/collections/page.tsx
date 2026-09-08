import { notFound } from "next/navigation";
import { ModelCollectionsIndex } from "@/components/model-collections-page";
import { getModelCollectionsSeoCopy } from "@/lib/model-collections";
import { isLocale, type Locale, LOCALES } from "@/lib/locales";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ locale: string }> };

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") return {};
  const seoCopy = getModelCollectionsSeoCopy(params.locale as Locale);
  return buildMetadata({ title: seoCopy.title, description: seoCopy.description, pathname: "/collections", locale: params.locale as Locale, absoluteTitle: true });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const pricing = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  return <ModelCollectionsIndex locale={params.locale as Locale} pricing={pricing} />;
}
