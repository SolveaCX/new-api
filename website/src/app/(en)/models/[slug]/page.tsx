import { notFound, redirect } from "next/navigation";
import { ModelLandingPage } from "@/components/model-landing-page";
import {
  getModelLandingConfig,
  getModelLandingConfigForPricingModel,
  getModelLandingConfigs,
  getLocalizedModelLandingConfig,
  buildModelLandingMetadata,
  limitSeoDescription,
  resolveModelLandingModels,
} from "@/lib/model-landing";
import { resolvePublicModel } from "@/lib/model-public";
import { getPricingData, getVendorName, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { fetchRankingsData } from "@/lib/rankings-live";
import { fetchModelHealthData } from "@/lib/model-health-server";
import { buildMetadata } from "@/lib/seo";
import { getSkagLandingMetadataInput } from "@/lib/skag-landing";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return getModelLandingConfigs().map((config) => ({ slug: config.slug }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (params.slug === "gpt-api") {
    return buildMetadata(getSkagLandingMetadataInput("gpt-api"));
  }
  if (params.slug === "claude-api") {
    return buildMetadata(getSkagLandingMetadataInput("claude-api"));
  }
  const config = getModelLandingConfig(params.slug);
  const pricing = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  if (config) {
    const liveModel = pricing.models.find((model) => model.model_name === config.modelId)
      ?? resolveModelLandingModels(config, pricing.models)[0];
    if (liveModel) {
      const modelWithVendor = {
        ...liveModel,
        vendor_name: liveModel.vendor_name ?? getVendorName(liveModel, pricing.vendors),
      };
      const dynamic = buildModelLandingMetadata(modelWithVendor, {
        displayName: config.displayName,
        pathname: `/models/${config.slug}`,
        task: config.generator?.kind === "image"
          ? "image generation"
          : config.generator?.kind === "video"
            ? "video generation"
            : config.generator?.kind === "audio"
              ? "audio"
              : undefined,
      });
      return buildMetadata(dynamic);
    }
    return buildMetadata({
      title: config.seoByLocale?.en?.title ?? config.seo.title,
      description: limitSeoDescription(config.seoByLocale?.en?.description ?? config.seo.description),
      pathname: `/models/${config.slug}`,
    });
  }
  const model = resolvePublicModel(pricing.models, params.slug);
  if (!model) return {};
  const modelWithVendor = {
    ...model,
    vendor_name: model.vendor_name ?? getVendorName(model, pricing.vendors),
  };
  const modelSpecificConfig = getModelLandingConfigForPricingModel(modelWithVendor);
  return buildMetadata(buildModelLandingMetadata(modelWithVendor, {
    pathname: `/models/${modelSpecificConfig.slug}`,
    task: modelSpecificConfig.generator?.kind === "image"
      ? "image generation"
      : modelSpecificConfig.generator?.kind === "video"
        ? "video generation"
        : modelSpecificConfig.generator?.kind === "audio"
          ? "audio"
          : undefined,
  }));
}

export default async function Page(props: Props) {
  const params = await props.params;
  // Model names in the live catalog preserve vendor casing, but the curated
  // landing route is lowercase. Redirect casing-only aliases so they cannot
  // render a second generic page with a competing canonical URL.
  if (params.slug !== "minimax-h3" && params.slug.toLowerCase() === "minimax-h3") {
    redirect("/models/minimax-h3");
  }
  if (params.slug === "seedance-2-5") redirect("/models/seedance-2.5");
  if (params.slug === "gpt-api") redirect("/gpt-api");
  if (params.slug === "claude-api") redirect("/claude-api");

  const config = getModelLandingConfig(params.slug);
  const [pricing, rankings] = await Promise.all([getPricingData(WEBSITE_PUBLIC_PRICING_GROUP), fetchRankingsData()]);
  const models = pricing.models.map((model) => ({
    ...model,
    vendor_name: model.vendor_name ?? getVendorName(model, pricing.vendors),
  }));

  if (config) {
    const resolvedModels = resolveModelLandingModels(config, models);
    // Single-model legacy routes (for example Sonilo and gpt-4.1-mini) must
    // use the same model-specific editorial pack as fallback model URLs. Keep
    // multi-model API family pages unchanged.
    const effectiveConfig = resolvedModels.length === 1
      ? getModelLandingConfigForPricingModel(resolvedModels[0])
      : config;
    const localizedConfig = getLocalizedModelLandingConfig(effectiveConfig, "en");
    const initialHealth = await fetchModelHealthData(resolvedModels[0]?.model_name ?? localizedConfig.modelId);
    return (
      <ModelLandingPage
        config={localizedConfig}
        locale="en"
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
  const localizedConfig = getLocalizedModelLandingConfig(modelSpecificConfig, "en");
  const initialHealth = await fetchModelHealthData(modelWithVendor.model_name);
  return (
    <ModelLandingPage
      config={localizedConfig}
      locale="en"
      liveModels={resolveModelLandingModels(localizedConfig, [modelWithVendor])}
    allModels={models}
    groupRatio={pricing.groupRatio}
    groupModelRatio={pricing.groupModelRatio}
    rankings={rankings}
    initialHealth={initialHealth}
  />
  );
}
