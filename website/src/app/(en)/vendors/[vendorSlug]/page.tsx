import { notFound } from "next/navigation";
import { VendorSeoPage, getVendorSeoPageData } from "@/components/pricing-seo-pages";
import { buildMetadata } from "@/lib/seo";
import { buildPricingSeoIndex, buildVendorSeoDescription, buildVendorSeoTitle } from "@/lib/pricing-seo";
import { getPricingData } from "@/lib/pricing";

type Props = {
  params: Promise<{ vendorSlug: string }>;
};

export async function generateStaticParams() {
  const index = buildPricingSeoIndex(await getPricingData());
  return index.vendors.map((vendor) => ({ vendorSlug: vendor.slug }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  const data = await getVendorSeoPageData(params.vendorSlug);
  if (!data.found) return {};
  return buildMetadata({
    title: buildVendorSeoTitle(data.entry),
    description: buildVendorSeoDescription(data.entry),
    pathname: `/vendors/${data.entry.slug}`,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  const data = await getVendorSeoPageData(params.vendorSlug);
  if (!data.found) notFound();
  return <VendorSeoPage locale="en" entry={data.entry} />;
}
