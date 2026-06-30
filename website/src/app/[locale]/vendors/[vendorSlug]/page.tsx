import { notFound } from "next/navigation";
import { VendorSeoPage, getVendorSeoPageData } from "@/components/pricing-seo-pages";
import { isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";
import { buildPricingSeoIndex, buildVendorSeoDescription, buildVendorSeoTitle } from "@/lib/pricing-seo";
import { getPricingData } from "@/lib/pricing";

type Props = {
  params: Promise<{ locale: string; vendorSlug: string }>;
};

export async function generateStaticParams() {
  const index = buildPricingSeoIndex(await getPricingData());
  return LOCALES.filter((locale) => locale !== "en").flatMap((locale) =>
    index.vendors.map((vendor) => ({ locale, vendorSlug: vendor.slug }))
  );
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const data = await getVendorSeoPageData(params.vendorSlug);
  if (!data.found) return {};
  return buildMetadata({
    title: buildVendorSeoTitle(data.entry),
    description: buildVendorSeoDescription(data.entry),
    pathname: `/vendors/${data.entry.slug}`,
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const data = await getVendorSeoPageData(params.vendorSlug);
  if (!data.found) notFound();
  return <VendorSeoPage locale={params.locale} entry={data.entry} />;
}
