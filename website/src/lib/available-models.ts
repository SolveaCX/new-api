import { APP_CONSOLE_ORIGIN } from './origins';
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP, type PricingData, type PricingModel } from './pricing';

type PublicCatalogModel = {
  id: string;
  availability_status: string;
  supported_endpoint_types: string[];
};

// Model membership comes from the same resolver as the console's Available
// Models page. Pricing only enriches those entries; it cannot add models.
export async function getAvailableModelPricingData(): Promise<PricingData> {
  const empty: PricingData = { models: [], vendors: [], groupRatio: {}, groupModelRatio: {}, usableGroup: {}, supportedEndpoint: {}, autoGroups: [] };
  try {
    const [response, pricing] = await Promise.all([
      fetch(new URL('/api/website/model-access', APP_CONSOLE_ORIGIN), {
        cache: 'no-store',
        signal: AbortSignal.timeout(10_000),
        headers: { accept: 'application/json' },
      }),
      getPricingData(WEBSITE_PUBLIC_PRICING_GROUP),
    ]);
    if (!response.ok) return empty;
    const payload = await response.json() as { success: boolean; data?: { models?: PublicCatalogModel[] } };
    if (!payload.success || !Array.isArray(payload.data?.models)) return empty;
    const catalog = new Map(payload.data.models
      .filter((model) => model.availability_status === 'available' || model.availability_status === 'unknown')
      .map((model) => [model.id, model]));
    // Preserve configured website ordering and its richer directory metadata.
    const models: PricingModel[] = pricing.models.flatMap((model) => {
      const available = catalog.get(model.model_name);
      return available ? [{ ...model, availability_status: available.availability_status, supported_endpoint_types: available.supported_endpoint_types }] : [];
    });
    return { ...pricing, models };
  } catch {
    return empty;
  }
}
