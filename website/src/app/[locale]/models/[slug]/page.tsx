import { notFound } from "next/navigation";
import { ModelLandingPage } from "@/components/model-landing-page";
import { ModelSeoPage, getModelSeoPageData } from "@/components/pricing-seo-pages";
import { isLocale, LOCALES } from "@/lib/locales";
import {
  getModelLandingConfig,
  getModelLandingConfigs,
  resolveModelLandingModels,
} from "@/lib/model-landing";
import { getPricingData, resolveVendorName } from "@/lib/pricing";
import { buildModelSeoDescription, buildModelSeoTitle, buildPricingSeoIndex } from "@/lib/pricing-seo";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string; slug: string }>;
};

export async function generateStaticParams() {
  const index = buildPricingSeoIndex(await getPricingData());
  const slugs = new Set<string>();
  getModelLandingConfigs().forEach((config) => slugs.add(config.slug));
  index.models.forEach((model) => slugs.add(model.slug));

  return LOCALES
    .filter((locale) => locale !== "en")
    .flatMap((locale) => Array.from(slugs).map((slug) => ({ locale, slug })));
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

  const data = await getModelSeoPageData(params.slug);
  if (!data.found) return {};
  return buildMetadata({
    title: buildModelSeoTitle(data.entry),
    description: buildModelSeoDescription(data.entry),
    pathname: `/models/${data.entry.slug}`,
    locale: params.locale,
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

  const data = await getModelSeoPageData(params.slug);
  if (!data.found) notFound();
  return <ModelSeoPage locale={params.locale} entry={data.entry} />;
}
