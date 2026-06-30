import { notFound } from "next/navigation";
import { ModelLandingPage } from "@/components/model-landing-page";
import { ModelSeoPage, getModelSeoPageData } from "@/components/pricing-seo-pages";
import {
  getModelLandingConfig,
  getModelLandingConfigs,
  resolveModelLandingModels,
} from "@/lib/model-landing";
import { getPricingData, getVendorName } from "@/lib/pricing";
import { buildModelSeoDescription, buildModelSeoTitle, buildPricingSeoIndex } from "@/lib/pricing-seo";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ slug: string }>;
};

export async function generateStaticParams() {
  const index = buildPricingSeoIndex(await getPricingData());
  const slugs = new Set<string>();
  getModelLandingConfigs().forEach((config) => slugs.add(config.slug));
  index.models.forEach((model) => slugs.add(model.slug));
  return Array.from(slugs).map((slug) => ({ slug }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  const config = getModelLandingConfig(params.slug);
  if (config) {
    return buildMetadata({
      title: config.seo.title,
      description: config.seo.description,
      pathname: `/models/${config.slug}`,
    });
  }

  const data = await getModelSeoPageData(params.slug);
  if (!data.found) return {};
  return buildMetadata({
    title: buildModelSeoTitle(data.entry),
    description: buildModelSeoDescription(data.entry),
    pathname: `/models/${data.entry.slug}`,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  const config = getModelLandingConfig(params.slug);

  if (config) {
    const pricing = await getPricingData();
    const models = pricing.models.map((model) => ({
      ...model,
      vendor_name: model.vendor_name ?? getVendorName(model, pricing.vendors),
    }));

    return (
      <ModelLandingPage
        config={config}
        locale="en"
        liveModels={resolveModelLandingModels(config, models)}
      />
    );
  }

  const data = await getModelSeoPageData(params.slug);
  if (!data.found) notFound();
  return <ModelSeoPage locale="en" entry={data.entry} />;
}
