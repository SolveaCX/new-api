import { notFound } from "next/navigation";
import { ModelDetailPage } from "@/components/model-detail-page";
import { ModelLandingPage } from "@/components/model-landing-page";
import { isLocale, LOCALES } from "@/lib/locales";
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
  params: Promise<{ locale: string; slug: string }>;
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
  const slugs = Array.from(
    new Map([...configuredParams, ...modelParams].map((item) => [item.slug, item])).values()
  );
  return LOCALES
    .filter((locale) => locale !== "en")
    .flatMap((locale) => slugs.map((item) => ({ locale, slug: item.slug })));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const config = getModelLandingConfig(params.slug);
  if (config) {
    return buildMetadata({
      title: config.seo.title,
      description: config.seo.description,
      pathname: `/models/${config.slug}`,
      locale: params.locale,
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
  const metadataCopy = modelDetailMetadataCopy(params.locale, model);
  return buildMetadata({
    title: metadataCopy.title,
    description: metadataCopy.description,
    pathname: getModelDetailPathname(model),
    locale: params.locale,
    noIndex: !indexable,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();

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
        locale={params.locale}
        liveModels={resolveModelLandingModels(config, models)}
      />
    );
  }

  const model = resolveModelBySlug(models, params.slug);
  if (!model) notFound();
  if (!isModelDetailRoutable(model, models)) notFound();
  const relatedModels = getIndexableRelatedModels(model, models);

  return <ModelDetailPage locale={params.locale} model={model} relatedModels={relatedModels} />;
}
