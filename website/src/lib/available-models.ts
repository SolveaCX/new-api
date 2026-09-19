import { APP_CONSOLE_ORIGIN } from './origins';
import { attachPricingPayloadData, getPricingData, WEBSITE_PUBLIC_PRICING_GROUP, type PricingData, type PricingModel, type PricingVendor } from './pricing';

type CatalogModel = {
  id: string;
  tags?: string;
  description?: string;
  display_weight?: number;
  vendor?: PricingVendor | null;
  availability_status: string;
  supported_endpoint_types: string[];
};

type Catalog = {
  scope_mode: string;
  account_model_ids?: string[];
  identity_model_ids?: string[];
  create_default_scope?: string | null;
  groups: { id: string; model_ids: string[] }[];
  models: CatalogModel[];
};

// Mirrors the console Available Models page's initial scope, not the union of
// every model present in the metadata payload.
export function getAvailableCatalogModels(access: Catalog): CatalogModel[] {
  const fixed = access.scope_mode === 'fixed_account';
  const scope = access.groups.find(group => group.id === access.create_default_scope) ?? access.groups[0];
  const ids = fixed ? access.account_model_ids : scope ? scope.model_ids : access.identity_model_ids;
  const allowed = new Set(ids ?? []);
  return access.models.filter(model => allowed.has(model.id) &&
    (model.availability_status === 'available' || model.availability_status === 'unknown'));
}

function emptyPricing(): PricingData {
  return { models: [], vendors: [], groupRatio: {}, groupModelRatio: {}, usableGroup: {}, supportedEndpoint: {}, autoGroups: [] };
}

async function fetchCatalogJson(path: string, headers: Record<string, string>) {
  const response = await fetch(new URL(path, APP_CONSOLE_ORIGIN), {
    cache: 'no-store', signal: AbortSignal.timeout(10_000), headers,
  });
  if (!response.ok) throw new Error(`Model catalog request failed (${response.status})`);
  const payload = await response.json();
  if (!payload.success || !payload.data) throw new Error('Model catalog response failed');
  return payload;
}

// The incoming cookie stays on the server and is forwarded only to the configured
// console. No authenticated catalog is cached or reused across visitors.
export async function getAvailableModelPricingData(cookie = ''): Promise<PricingData> {
  try {
    const requestHeaders: Record<string, string> = { accept: 'application/json' };
    let authenticated = false;
    if (cookie) {
      const identity = await fetch(new URL('/api/user/analytics-self', APP_CONSOLE_ORIGIN), {
        cache: 'no-store', signal: AbortSignal.timeout(10_000),
        headers: { ...requestHeaders, cookie },
      });
      if (identity.status !== 401) {
        if (!identity.ok) return emptyPricing();
        const payload = await identity.json();
        const id = payload.data?.id;
        if (!payload.success || !Number.isInteger(id) || id <= 0) return emptyPricing();
        requestHeaders.cookie = cookie;
        requestHeaders['New-Api-User'] = String(id);
        authenticated = true;
      }
    }

    const [catalogPayload, publicPricing] = await Promise.all([
      fetchCatalogJson(authenticated
        ? '/api/user/model-access?view=available_models'
        : '/api/website/model-access?view=available_models', requestHeaders),
      getPricingData(WEBSITE_PUBLIC_PRICING_GROUP),
    ]);
    const access = catalogPayload.data as Catalog;
    if (!Array.isArray(access.models) || !Array.isArray(access.groups)) return emptyPricing();
    const catalog = getAvailableCatalogModels(access);
    let pricing = publicPricing;
    if (authenticated) {
      // Same price endpoint as the console; failure must not remove authorized
      // models or expand membership to the public PLG catalog.
      const payload = await fetchCatalogJson('/api/pricing', requestHeaders).catch(() => null);
      if (payload && Array.isArray(payload.data)) {
        const publicRows = new Map(publicPricing.models.map(model => [model.model_name, model]));
        pricing = {
          models: payload.data.map((model: PricingModel) => attachPricingPayloadData({ ...publicRows.get(model.model_name), ...model }, {})),
          vendors: payload.vendors ?? [], groupRatio: payload.group_ratio ?? {},
          groupModelRatio: payload.group_model_ratio ?? {}, usableGroup: payload.usable_group ?? {},
          supportedEndpoint: payload.supported_endpoint ?? {}, autoGroups: payload.auto_groups ?? [],
        };
      }
    }
    const prices = new Map(pricing.models.map(model => [model.model_name, model]));
    const available = new Map(catalog.map(model => [model.id, model]));
    // Keep the existing directory order, then append available entries missing
    // from pricing. A price response never determines model membership.
    const ordered = [
      ...pricing.models.flatMap(model => available.has(model.model_name) ? [available.get(model.model_name)!] : []),
      ...catalog.filter(model => !prices.has(model.id)),
    ];
    return { ...pricing, models: ordered.map(model => ({
      model_name: model.id, quota_type: 0, model_ratio: Number.NaN, completion_ratio: Number.NaN,
      ...prices.get(model.id),
      tags: model.tags ?? '', display_weight: model.display_weight ?? 0,
      description: model.description ?? prices.get(model.id)?.description,
      vendor_name: model.vendor?.name ?? prices.get(model.id)?.vendor_name,
      vendor_icon: model.vendor?.icon ?? prices.get(model.id)?.vendor_icon,
      availability_status: model.availability_status, supported_endpoint_types: model.supported_endpoint_types,
    })) };
  } catch {
    // An authenticated failure must never fall back to a broader public list.
    return emptyPricing();
  }
}
