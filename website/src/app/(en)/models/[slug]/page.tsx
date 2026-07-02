import { notFound } from "next/navigation";
import { ModelDetailPage } from "@/components/model-detail-page";
import { ModelLandingPage } from "@/components/model-landing-page";
import {
  getIndexableRelatedModels,
  getModelDetailPathname,
  getRoutableModelDetailPathnames,
  isModelDetailIndexable,
  isModelDetailRoutable,
  modelDetailMetadataCopy,
  resolveModelBySlug,
} from "@/lib/model-detail";
import {
  getModelLandingConfig,
  getModelLandingConfigs,
  resolveModelLandingModels,
} from "@/lib/model-landing";
import { getPricingData, resolveVendorName } from "@/lib/pricing";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ slug: string }>;
};

export async function generateStaticParams() {
  const pricing = await getPricingData();
  const models = pricing.models.map((model) => ({
    ...model,
    vendor_name: resolveVendorName(model, pricing.vendors),
  }));
  const modelParams = getRoutableModelDetailPathnames(models).map((pathname) => ({
    slug: pathname.split("/").pop() ?? "",
  }));
  const configuredParams = getModelLandingConfigs().map((config) => ({ slug: config.slug }));
  return Array.from(
    new Map([...configuredParams, ...modelParams].map((item) => [item.slug, item])).values()
  );
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

  const pricing = await getPricingData();
  const models = pricing.models.map((model) => ({
    ...model,
    vendor_name: resolveVendorName(model, pricing.vendors),
  }));
  const model = resolveModelBySlug(models, params.slug);
  if (!model) return {};
  if (!isModelDetailRoutable(model, models)) return {};
  const indexable = isModelDetailIndexable(model, models);
  const metadataCopy = modelDetailMetadataCopy("en", model);
  return buildMetadata({
    title: metadataCopy.title,
    description: metadataCopy.description,
    pathname: getModelDetailPathname(model),
    noIndex: !indexable,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  const config = getModelLandingConfig(params.slug);

  const pricing = await getPricingData();
  const models = pricing.models.map((model) => ({
    ...model,
    vendor_name: resolveVendorName(model, pricing.vendors),
  }));

  if (config) {
    return (
      <ModelLandingPage
        config={config}
        locale="en"
        liveModels={resolveModelLandingModels(config, models)}
      />
    );
  }

  const model = resolveModelBySlug(models, params.slug);
  if (!model) notFound();
  if (!isModelDetailRoutable(model, models)) notFound();
  const relatedModels = getIndexableRelatedModels(model, models);

  return <ModelDetailPage locale="en" model={model} relatedModels={relatedModels} />;
}
