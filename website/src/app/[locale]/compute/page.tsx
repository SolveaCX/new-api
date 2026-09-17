import { notFound } from "next/navigation";
import { ComputeMarketPage } from "@/components/compute-market-page";
import { getComputeMarketCopy } from "@/lib/compute-market-copy";
import { isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";
import { staticFeaturePages } from "@/lib/static-feature-pages";

type Props = {
  params: Promise<{ locale: string }>;
};

const page = staticFeaturePages.compute;

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const copy = getComputeMarketCopy(params.locale);
  return buildMetadata({ title: copy.metaTitle, description: copy.metaDescription, pathname: page.pathname, locale: params.locale });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  return <ComputeMarketPage locale={params.locale} />;
}
