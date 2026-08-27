import { notFound, redirect } from "next/navigation";
import { ModelLandingPage } from "@/components/model-landing-page";
import { isLocale, LOCALES, localizePath } from "@/lib/locales";
import {
  getModelLandingConfig,
  getModelLandingConfigForPricingModel,
  getModelLandingConfigs,
  getLocalizedModelLandingConfig,
  resolveModelLandingModels,
} from "@/lib/model-landing";
import { resolvePublicModel } from "@/lib/model-public";
import { getPricingData, getVendorName, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { fetchRankingsData } from "@/lib/rankings-live";
import { fetchModelHealthData } from "@/lib/model-health-server";
import { buildMetadata } from "@/lib/seo";
import { getSkagLandingMetadataInput } from "@/lib/skag-landing";

type Props = {
  params: Promise<{ locale: string; slug: string }>;
};

export function generateStaticParams() {
  return LOCALES
    .filter((locale) => locale !== "en")
    .flatMap((locale) => getModelLandingConfigs().map((config) => ({ locale, slug: config.slug })));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  if (params.slug === "gpt-api") {
    return buildMetadata(getSkagLandingMetadataInput("gpt-api", params.locale));
  }
  if (params.slug === "claude-api") {
    return buildMetadata(getSkagLandingMetadataInput("claude-api", params.locale));
  }
  const config = getModelLandingConfig(params.slug);
  if (config) {
    const localizedSeo = config.seoByLocale?.[params.locale] ?? config.seo;
    return buildMetadata({
      title: localizedSeo.title,
      description: localizedSeo.description,
      pathname: `/models/${config.slug}`,
      locale: params.locale,
    });
  }
  const pricing = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  const model = resolvePublicModel(pricing.models, params.slug);
  if (!model) return {};
  const modelWithVendor = {
    ...model,
    vendor_name: model.vendor_name ?? getVendorName(model, pricing.vendors),
  };
  const modelSpecificConfig = getModelLandingConfigForPricingModel(modelWithVendor);
  const localizedSeo = modelSpecificConfig.seoByLocale?.[params.locale] ?? modelSpecificConfig.seoByLocale?.en ?? modelSpecificConfig.seo;
  return buildMetadata({
    title: localizedSeo.title,
    description: localizedSeo.description,
    pathname: `/models/${modelSpecificConfig.slug}`,
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  // Keep the localized landing URL canonical when a catalog model name is
  // requested with vendor casing (for example /models/MiniMax-H3).
  if (params.slug !== "minimax-h3" && params.slug.toLowerCase() === "minimax-h3") {
    redirect(localizePath("/models/minimax-h3", params.locale));
  }
  if (params.slug === "seedance-2-5") redirect(localizePath("/models/seedance-2.5", params.locale));
  if (params.slug === "gpt-api") redirect(localizePath("/gpt-api", params.locale));
  if (params.slug === "claude-api") redirect(localizePath("/claude-api", params.locale));

  const config = getModelLandingConfig(params.slug);
  const [pricing, rankings] = await Promise.all([getPricingData(WEBSITE_PUBLIC_PRICING_GROUP), fetchRankingsData()]);
  const models = pricing.models.map((model) => ({
    ...model,
    vendor_name: model.vendor_name ?? getVendorName(model, pricing.vendors),
  }));

  if (config) {
    const localizedConfig = getLocalizedModelLandingConfig(config, params.locale);
    const resolvedModels = resolveModelLandingModels(localizedConfig, models);
    const initialHealth = await fetchModelHealthData(resolvedModels[0]?.model_name ?? localizedConfig.modelId);
    return (
      <ModelLandingPage
        config={localizedConfig}
        locale={params.locale}
        liveModels={resolvedModels}
        allModels={models}
        groupRatio={pricing.groupRatio}
        groupModelRatio={pricing.groupModelRatio}
        rankings={rankings}
        initialHealth={initialHealth}
      />
    );
  }

  // Generic public model page: rankings / directory click-through target.
  const model = resolvePublicModel(models, params.slug);
  if (!model) notFound();
  const modelWithVendor = {
    ...model,
    vendor_name: model.vendor_name ?? getVendorName(model, pricing.vendors),
  };
  const modelSpecificConfig = getModelLandingConfigForPricingModel(modelWithVendor);
  const localizedConfig = getLocalizedModelLandingConfig(modelSpecificConfig, params.locale);
  const initialHealth = await fetchModelHealthData(modelWithVendor.model_name);
  return (
    <ModelLandingPage
      config={localizedConfig}
      locale={params.locale}
      liveModels={resolveModelLandingModels(localizedConfig, [modelWithVendor])}
    allModels={models}
    groupRatio={pricing.groupRatio}
    groupModelRatio={pricing.groupModelRatio}
    rankings={rankings}
    initialHealth={initialHealth}
  />
  );
}
