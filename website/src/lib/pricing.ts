import { APP_CONSOLE_ORIGIN } from "@/lib/origins";

export const API_BASE_URL = APP_CONSOLE_ORIGIN;
export const WEBSITE_PUBLIC_PRICING_GROUP = "plg";
// Health metrics scope: every active group, merged server-side (whole-platform traffic).
export const PERF_METRICS_ALL_GROUPS = "all";

export type PricingVendor = {
  id: number;
  name: string;
  icon?: string;
  description?: string;
};

export type PricingModel = {
  id?: number;
  model_name: string;
  description?: string;
  icon?: string;
  vendor_id?: number;
  vendor_name?: string;
  vendor_icon?: string;
  vendor_description?: string;
  quota_type: number;
  model_ratio: number;
  completion_ratio: number;
  model_price?: number;
  cache_ratio?: number | null;
  create_cache_ratio?: number | null;
  image_ratio?: number | null;
  audio_ratio?: number | null;
  audio_completion_ratio?: number | null;
  enable_groups?: string[];
  tags?: string;
  supported_endpoint_types?: string[];
  group_ratio?: Record<string, number>;
  group_model_ratio?: Record<string, number>;
  billing_mode?: string;
  billing_expr?: string;
  pricing_version?: string;
  availability_status?: string;
  availability_reason?: string;
  availability_detected_at?: number;
  availability_checked_at?: number;
  display_prices?: WebsiteDisplayPrices;
  effective_group_ratio?: string;
  ratio_source?: "group" | "group_group" | "group_model";
  display_label?: string;
};

export type WebsitePricePair = {
  configured: string;
  plg: string;
};

export type WebsiteDisplayPrices = {
  input: WebsitePricePair | null;
  output: WebsitePricePair | null;
  cache: WebsitePricePair | null;
  cache_creation: WebsitePricePair | null;
  image: WebsitePricePair | null;
  audio_input: WebsitePricePair | null;
  audio_output: WebsitePricePair | null;
  request: WebsitePricePair | null;
};

type PricingApiResponse = {
  success: boolean;
  message?: string;
  data?: PricingModel[];
  vendors?: PricingVendor[];
  group_ratio?: Record<string, number>;
  group_model_ratio?: GroupModelRatio;
  usable_group?: Record<string, string>;
  supported_endpoint?: Record<string, string>;
  auto_groups?: string[];
};

type WebsitePricingV2Response = {
  success: boolean;
  schema_version: string;
  group: string;
  generated_at: number;
  models: Array<{
    model_name: string;
    description?: string;
    icon?: string;
    tags?: string;
    vendor_id?: number;
    quota_type: number;
    enable_groups: string[];
    supported_endpoint_types: string[];
    billing_kind: string;
    display_label?: string;
    effective_group_ratio: string;
    ratio_source: "group" | "group_group" | "group_model";
    prices: WebsiteDisplayPrices;
    availability_status?: string;
    availability_reason?: string;
    availability_detected_at?: number;
    availability_checked_at?: number;
  }>;
  vendors?: PricingVendor[];
  supported_endpoint?: Record<string, unknown>;
  auto_groups?: string[];
};

const WEBSITE_DISPLAY_PRICE_KEYS = [
  "input",
  "output",
  "cache",
  "cache_creation",
  "image",
  "audio_input",
  "audio_output",
  "request",
] as const satisfies ReadonlyArray<keyof WebsiteDisplayPrices>;

export type GroupModelRatio = Record<string, Record<string, number>>;

export type PricingData = {
  models: PricingModel[];
  vendors: PricingVendor[];
  groupRatio: Record<string, number>;
  groupModelRatio: GroupModelRatio;
  usableGroup: Record<string, string>;
  supportedEndpoint: Record<string, unknown>;
  autoGroups: string[];
  pricingAvailable?: boolean;
};

export type PricingSearch = {
  q?: string;
  vendor?: string;
  endpoint?: string;
  quota?: string;
};

const QUOTA_TYPE_TOKEN = 0;
const QUOTA_TYPE_REQUEST = 1;
const VENDOR_SORT_PRIORITY: Record<string, number> = {
  openai: 0,
  anthropic: 1,
  google: 2,
  gemini: 2,
};

export function publicPricingUrl(apiBaseUrl = API_BASE_URL, group?: string): string {
  const url = new URL("/api/website/pricing", apiBaseUrl);
  if (group) url.searchParams.set("group", group);
  return url.toString();
}

export function publicPricingV2Url(apiBaseUrl = API_BASE_URL): string {
  const url = new URL("/api/website/pricing/v2", apiBaseUrl);
  url.searchParams.set("group", WEBSITE_PUBLIC_PRICING_GROUP);
  return url.toString();
}

export async function getPricingData(group?: string): Promise<PricingData> {
  if (group === WEBSITE_PUBLIC_PRICING_GROUP) {
    return getWebsitePricingV2Data();
  }
  try {
    const response = await fetch(publicPricingUrl(API_BASE_URL, group), {
      cache: "no-store",
      headers: { accept: "application/json" },
    });
    if (!response.ok) return emptyPricingData();
    const payload = (await response.json()) as PricingApiResponse;
    if (!payload.success) return emptyPricingData();
    return {
      models: payload.data ?? [],
      vendors: payload.vendors ?? [],
      groupRatio: payload.group_ratio ?? {},
      groupModelRatio: payload.group_model_ratio ?? {},
      usableGroup: payload.usable_group ?? {},
      supportedEndpoint: payload.supported_endpoint ?? {},
      autoGroups: payload.auto_groups ?? [],
      pricingAvailable: true,
    };
  } catch {
    return emptyPricingData();
  }
}

async function getWebsitePricingV2Data(): Promise<PricingData> {
  try {
    const response = await fetch(publicPricingV2Url(), {
      next: { revalidate: 60 },
      headers: { accept: "application/json" },
    });
    if (!response.ok) return emptyPricingData();
    return parseWebsitePricingV2((await response.json()) as unknown) ?? emptyPricingData();
  } catch {
    return emptyPricingData();
  }
}

function parseWebsitePricingV2(value: unknown): PricingData | null {
  const payload = value as WebsitePricingV2Response;
  if (
    !payload ||
    payload.success !== true ||
    payload.schema_version !== "website-public-plg-v2" ||
    payload.group !== WEBSITE_PUBLIC_PRICING_GROUP ||
    !Number.isInteger(payload.generated_at) ||
    !Array.isArray(payload.models) ||
    (payload.vendors !== undefined && !Array.isArray(payload.vendors)) ||
    (payload.auto_groups !== undefined && !isStringArray(payload.auto_groups)) ||
    (payload.supported_endpoint !== undefined && !isRecord(payload.supported_endpoint))
  ) {
    return null;
  }

  const seen = new Set<string>();
  const models: PricingModel[] = [];
  for (const item of payload.models) {
    if (
      !item ||
      typeof item.model_name !== "string" ||
      item.model_name.length === 0 ||
      seen.has(item.model_name) ||
      (item.quota_type !== QUOTA_TYPE_TOKEN && item.quota_type !== QUOTA_TYPE_REQUEST) ||
      !isStringArray(item.enable_groups) ||
      !item.enable_groups.includes(WEBSITE_PUBLIC_PRICING_GROUP) ||
      !isStringArray(item.supported_endpoint_types) ||
      !["token_ratio", "request_base", "tiered_expr"].includes(item.billing_kind) ||
      !validPriceString(item.effective_group_ratio) ||
      !validWebsiteDisplayPrices(item.prices) ||
      !["group", "group_group", "group_model"].includes(item.ratio_source)
    ) {
      return null;
    }
    seen.add(item.model_name);
    models.push({
      model_name: item.model_name,
      description: item.description,
      icon: item.icon,
      tags: item.tags,
      vendor_id: item.vendor_id,
      quota_type: item.quota_type,
      model_ratio: 0,
      completion_ratio: 0,
      model_price: 0,
      enable_groups: [WEBSITE_PUBLIC_PRICING_GROUP],
      supported_endpoint_types: item.supported_endpoint_types ?? [],
      billing_mode: item.billing_kind,
      display_label: item.display_label,
      effective_group_ratio: item.effective_group_ratio,
      ratio_source: item.ratio_source,
      display_prices: item.prices,
      availability_status: item.availability_status,
      availability_reason: item.availability_reason,
      availability_detected_at: item.availability_detected_at,
      availability_checked_at: item.availability_checked_at,
    });
  }

  return {
    models,
    vendors: payload.vendors ?? [],
    groupRatio: { [WEBSITE_PUBLIC_PRICING_GROUP]: 1 },
    groupModelRatio: {},
    usableGroup: { [WEBSITE_PUBLIC_PRICING_GROUP]: WEBSITE_PUBLIC_PRICING_GROUP },
    supportedEndpoint: payload.supported_endpoint ?? {},
    autoGroups: payload.auto_groups ?? [],
    pricingAvailable: true,
  };
}

function validWebsiteDisplayPrices(prices: WebsiteDisplayPrices | undefined): prices is WebsiteDisplayPrices {
  if (!prices || typeof prices !== "object") return false;
  return WEBSITE_DISPLAY_PRICE_KEYS.every(
    (key) => Object.hasOwn(prices, key) && (prices[key] === null || validWebsitePricePair(prices[key]))
  );
}

function validWebsitePricePair(pair: WebsitePricePair): boolean {
  return validPriceString(pair.configured) && validPriceString(pair.plg);
}

function validPriceString(value: string): boolean {
  if (typeof value !== "string" || value.trim() === "") return false;
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0;
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

export function buildEffectiveGroupRatio(
  model: Pick<PricingModel, "model_name" | "group_ratio">,
  fallbackGroupRatio: Record<string, number>,
  groupModelRatio: GroupModelRatio = {}
): Record<string, number> {
  const effective = { ...fallbackGroupRatio, ...(model.group_ratio ?? {}) };
  const matchedModelName = formatMatchingModelName(model.model_name);
  for (const [group, modelRatios] of Object.entries(groupModelRatio)) {
    const ratio = modelRatios?.[matchedModelName];
    if (typeof ratio === "number" && Number.isFinite(ratio)) {
      effective[group] = ratio;
    }
  }
  return effective;
}

export function getGroupModelRatioForModel(modelName: string, groupModelRatio: GroupModelRatio = {}): Record<string, number> {
  const result: Record<string, number> = {};
  const matchedModelName = formatMatchingModelName(modelName);
  for (const [group, modelRatios] of Object.entries(groupModelRatio)) {
    const ratio = modelRatios?.[matchedModelName];
    if (typeof ratio === "number" && Number.isFinite(ratio)) {
      result[group] = ratio;
    }
  }
  return result;
}

export function formatMatchingModelName(modelName: string): string {
  let name = modelName;
  if (name.startsWith("gemini-2.5-flash-lite")) {
    name = handleThinkingBudgetModel(name, "gemini-2.5-flash-lite", "gemini-2.5-flash-lite-thinking-*");
  } else if (name.startsWith("gemini-2.5-flash")) {
    name = handleThinkingBudgetModel(name, "gemini-2.5-flash", "gemini-2.5-flash-thinking-*");
  } else if (name.startsWith("gemini-2.5-pro")) {
    name = handleThinkingBudgetModel(name, "gemini-2.5-pro", "gemini-2.5-pro-thinking-*");
  }

  if (name.startsWith("gpt-4-gizmo")) {
    return "gpt-4-gizmo-*";
  }
  if (name.startsWith("gpt-4o-gizmo")) {
    return "gpt-4o-gizmo-*";
  }
  return name;
}

function handleThinkingBudgetModel(modelName: string, prefix: string, wildcard: string): string {
  return modelName.startsWith(prefix) && modelName.includes("-thinking-") ? wildcard : modelName;
}

export function filterPricingModels(models: PricingModel[], search: PricingSearch): PricingModel[] {
  const query = search.q?.trim().toLowerCase();
  const vendor = search.vendor?.trim().toLowerCase();
  const endpoint = search.endpoint?.trim().toLowerCase();
  const quota = search.quota?.trim().toLowerCase();

  return models.filter((model) => {
    if (query) {
      const haystack = [
        model.model_name,
        model.description,
        model.vendor_name,
        model.tags,
        ...(model.supported_endpoint_types ?? []),
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      if (!haystack.includes(query)) return false;
    }

    if (vendor && vendor !== "all" && model.vendor_name?.toLowerCase() !== vendor) return false;
    if (endpoint && endpoint !== "all" && !(model.supported_endpoint_types ?? []).some((item) => item.toLowerCase() === endpoint)) {
      return false;
    }
    if (quota === "token" && model.quota_type !== QUOTA_TYPE_TOKEN) return false;
    if (quota === "request" && model.quota_type !== QUOTA_TYPE_REQUEST) return false;

    return true;
  });
}

export function sortPricingModelsBySeries(models: PricingModel[]): PricingModel[] {
  return [...models].sort((a, b) => {
    const vendorCompare = getVendorSortKey(a).localeCompare(getVendorSortKey(b), "en", { numeric: true });
    if (vendorCompare !== 0) return vendorCompare;

    const familyCompare = getModelFamilyKey(a.model_name).localeCompare(getModelFamilyKey(b.model_name), "en", { numeric: true });
    if (familyCompare !== 0) return familyCompare;

    return getModelVersionSortKey(a.model_name).localeCompare(getModelVersionSortKey(b.model_name), "en", { numeric: true });
  });
}

export function getVendorName(model: PricingModel, vendors: PricingVendor[]): string {
  return model.vendor_name ?? vendors.find((vendor) => vendor.id === model.vendor_id)?.name ?? "AI";
}

export function getTopVendors(models: PricingModel[], limit = 10): string[] {
  const counts = new Map<string, number>();
  for (const model of models) {
    if (!model.vendor_name) continue;
    counts.set(model.vendor_name, (counts.get(model.vendor_name) ?? 0) + 1);
  }
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, limit)
    .map(([vendor]) => vendor);
}

export function getTopEndpoints(models: PricingModel[], limit = 8): string[] {
  const counts = new Map<string, number>();
  for (const model of models) {
    for (const endpoint of model.supported_endpoint_types ?? []) {
      counts.set(endpoint, (counts.get(endpoint) ?? 0) + 1);
    }
  }
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .slice(0, limit)
    .map(([endpoint]) => endpoint);
}

export function isTokenBasedModel(model: PricingModel): boolean {
  return model.quota_type === QUOTA_TYPE_TOKEN;
}

export function formatModelPrice(model: PricingModel, type: "input" | "output" | "cache" = "input"): string {
  if (model.display_prices) {
    const pair = model.quota_type === QUOTA_TYPE_REQUEST ? model.display_prices.request : model.display_prices[type];
    return pair ? formatUsd(Number(pair.plg)) : model.display_label ?? "-";
  }
  if (!isTokenBasedModel(model)) {
    return formatUsd(model.model_price ?? 0);
  }

  const base = Number(model.model_ratio ?? 0) * 2 * getMinGroupRatio(model);
  const price =
    type === "output"
      ? base * Number(model.completion_ratio ?? 1)
      : type === "cache" && model.cache_ratio != null
        ? base * Number(model.cache_ratio)
        : base;

  return formatUsd(price);
}

// Official vendor list price per 1M tokens (ratio convention: model_ratio × $2,
// calibrated to the vendor's published price), before any group discount.
export function getOfficialPriceUsd(model: PricingModel, type: "input" | "output" = "input"): number {
  if (model.display_prices) {
    const pair = model.quota_type === QUOTA_TYPE_REQUEST ? model.display_prices.request : model.display_prices[type];
    return pair ? Number(pair.configured) : 0;
  }
  if (!isTokenBasedModel(model)) return Number(model.model_price ?? 0);
  const base = Number(model.model_ratio ?? 0) * 2;
  return type === "output" ? base * Number(model.completion_ratio ?? 1) : base;
}

// Cheapest visible group ratio for the model — the "60-90% of official" layer.
// Group ratios live in the pricing payload's top-level group_ratio map, keyed
// by the model's enable_groups.
export function getBestGroupRatio(model: PricingModel, fallbackGroupRatio: Record<string, number>): number {
  if (model.display_prices) {
    const pair = model.quota_type === QUOTA_TYPE_REQUEST ? model.display_prices.request : model.display_prices.input;
    if (!pair) return 1;
    const configured = Number(pair.configured);
    const plg = Number(pair.plg);
    return configured > 0 && Number.isFinite(plg) ? plg / configured : 1;
  }
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups.filter(isVisibleGroup) : [];
  const names = groups.includes("all") ? Object.keys(fallbackGroupRatio).filter(isVisibleGroup) : groups;
  const ratios = names
    .map((group) => model.group_ratio?.[group] ?? fallbackGroupRatio[group])
    .filter((ratio): ratio is number => typeof ratio === "number" && Number.isFinite(ratio) && ratio > 0);
  return ratios.length > 0 ? Math.min(...ratios) : 1;
}

// Effective price after the best top-up bonus tier ($200 + $100 → 2/3 of list).
export function discountedPriceUsd(value: number): number {
  return (value * 2) / 3;
}

export function formatUsdPrice(value: number): string {
  return formatUsd(value);
}

export function getAvailableGroups(
  model: PricingModel,
  fallbackGroupRatio: Record<string, number> = {},
  usableGroup: Record<string, string> = {}
): string[] {
  const usableGroups = Object.keys(usableGroup).filter(isVisibleGroup);
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups.filter(isVisibleGroup) : [];
  const ratioGroups = Object.keys(model.group_ratio ?? fallbackGroupRatio).filter(isVisibleGroup);
  if (groups.includes("all")) return usableGroups;
  const modelSpecificGroups = Object.keys(model.group_model_ratio ?? {}).filter(isVisibleGroup);
  const candidateGroups = Array.from(new Set(groups.length > 0 ? [...groups, ...modelSpecificGroups] : ratioGroups));
  if (candidateGroups.length > 0 && usableGroups.length > 0) {
    const usableSet = new Set(usableGroups);
    return candidateGroups.filter((group) => usableSet.has(group));
  }
  if (candidateGroups.length > 0) return candidateGroups;
  return ratioGroups;
}

export function formatGroupTokenPrice(
  model: PricingModel,
  group: string,
  groupRatio: Record<string, number>,
  type: "input" | "output" | "cache" | "create_cache" | "image" | "audio_input" | "audio_output"
): string {
  if (!isTokenBasedModel(model)) return "-";
  if (model.display_prices && group === WEBSITE_PUBLIC_PRICING_GROUP) {
    const pair = getWebsiteDisplayPricePair(model.display_prices, type);
    return pair ? formatUsd(Number(pair.plg)) : "-";
  }
  const ratio = getGroupRatio(model, group, groupRatio);
  const base = Number(model.model_ratio ?? 0) * 2 * ratio;
  const price = calculateTokenPrice(model, base, type);
  return Number.isFinite(price) ? formatUsd(price) : "-";
}

export function formatGroupRequestPrice(model: PricingModel, group: string, groupRatio: Record<string, number>): string {
  if (isTokenBasedModel(model)) return "-";
  if (model.display_prices && group === WEBSITE_PUBLIC_PRICING_GROUP) {
    const pair = model.display_prices.request;
    return pair ? formatUsd(Number(pair.plg)) : model.display_label ?? "-";
  }
  const ratio = getGroupRatio(model, group, groupRatio);
  return formatUsd(Number(model.model_price ?? 0) * ratio);
}

export function getPublicPriceUsd(model: PricingModel): number | null {
  if (!model.display_prices) return null;
  const pair = model.quota_type === QUOTA_TYPE_REQUEST ? model.display_prices.request : model.display_prices.input;
  return pair ? Number(pair.plg) : null;
}

function getWebsiteDisplayPricePair(
  prices: WebsiteDisplayPrices,
  type: "input" | "output" | "cache" | "create_cache" | "image" | "audio_input" | "audio_output"
): WebsitePricePair | null {
  if (type === "create_cache") return prices.cache_creation;
  return prices[type];
}

export function formatRatio(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(Number(value))) return "-";
  return new Intl.NumberFormat("en-US", {
    maximumFractionDigits: 8,
  }).format(Number(value));
}

export function parseTags(tags?: string): string[] {
  if (!tags) return [];
  return tags
    .split(/[,\s]+/)
    .map((tag) => tag.trim())
    .filter(Boolean)
    .slice(0, 4);
}

function getMinGroupRatio(model: PricingModel): number {
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups : [];
  if (groups.length === 0) return 1;
  const ratios = groups
    .map((group) => model.group_ratio?.[group])
    .filter((ratio): ratio is number => typeof ratio === "number" && Number.isFinite(ratio));
  return ratios.length > 0 ? Math.min(...ratios) : 1;
}

function getGroupRatio(model: PricingModel, group: string, fallbackGroupRatio: Record<string, number>): number {
  const modelRatio = model.group_ratio?.[group];
  if (typeof modelRatio === "number" && Number.isFinite(modelRatio)) return modelRatio;
  const fallbackRatio = fallbackGroupRatio[group];
  if (typeof fallbackRatio === "number" && Number.isFinite(fallbackRatio)) return fallbackRatio;
  return 1;
}

function isVisibleGroup(group: string): boolean {
  return group !== "" && group !== "auto";
}

function getVendorSortKey(model: PricingModel): string {
  const vendor = (model.vendor_name || model.vendor_icon || "zz-provider").toLowerCase();
  const priority = VENDOR_SORT_PRIORITY[vendor] ?? 50;
  return `${priority}:${vendor}`;
}

export function getModelFamilyKey(modelName: string): string {
  const name = modelName.toLowerCase();
  const normalized = name
    .replace(/\b(20\d{2}[-_]?\d{2}[-_]?\d{2}|\d{8})\b/g, "")
    .replace(/[-_](latest|preview|beta|stable|turbo|instruct|chat|online|fk|br|cc|compact)$/g, "")
    .replace(/[-_]+/g, "-");

  const familyPatterns: Array<[RegExp, string]> = [
    [/^gpt-5(?:[.-]\d+)?(?:-|$)/, "gpt-5"],
    [/^gpt-4(?:[.-]\d+)?(?:-|$)/, "gpt-4"],
    [/^gpt-3(?:[.-]\d+)?(?:-|$)/, "gpt-3"],
    [/^o[1-4](?:-|$)/, "openai-o"],
    [/^claude-(opus|sonnet|haiku|fable)-(\d+)/, "claude-$1-$2"],
    [/^gemini-(\d+(?:\.\d+)?)/, "gemini-$1"],
    [/^qwen(?:\d+|[-_]\d+)?/, "qwen"],
    [/^deepseek[-_]?r/, "deepseek-r"],
    [/^deepseek/, "deepseek"],
    [/^doubao/, "doubao"],
    [/^hunyuan/, "hunyuan"],
    [/^glm[-_]?/, "glm"],
    [/^mistral/, "mistral"],
    [/^llama[-_]?\d+/, "llama"],
    [/^grok[-_]?/, "grok"],
    [/^kimi[-_]?/, "kimi"],
    [/^abab[-_]?/, "abab"],
    [/^seedance[-_]?/, "seedance"],
    [/^kling[-_]?/, "kling"],
    [/^veo[-_]?/, "veo"],
    [/^imagen[-_]?/, "imagen"],
    [/^sora[-_]?/, "sora"],
    [/^dall[-_]?e/, "dalle"],
    [/^text-embedding/, "text-embedding"],
  ];

  for (const [pattern, key] of familyPatterns) {
    const match = normalized.match(pattern);
    if (match) return key.replace(/\$(\d+)/g, (_, index: string) => match[Number(index)] ?? "");
  }

  return normalized.split("-").slice(0, 2).join("-") || normalized;
}

function getModelVersionSortKey(modelName: string): string {
  const name = modelName.toLowerCase();
  const tier =
    /latest|preview/.test(name) ? "0" :
    /mini|small|haiku|flash|nano/.test(name) ? "2" :
    /pro|sonnet|medium/.test(name) ? "1" :
    /opus|ultra|large|max/.test(name) ? "0" :
    "1";
  return `${tier}:${name}`;
}

function calculateTokenPrice(
  model: PricingModel,
  base: number,
  type: "input" | "output" | "cache" | "create_cache" | "image" | "audio_input" | "audio_output"
): number {
  switch (type) {
    case "input":
      return base;
    case "output":
      return base * Number(model.completion_ratio ?? 1);
    case "cache":
      return model.cache_ratio == null ? Number.NaN : base * Number(model.cache_ratio);
    case "create_cache":
      return model.create_cache_ratio == null ? Number.NaN : base * Number(model.create_cache_ratio);
    case "image":
      return model.image_ratio == null ? Number.NaN : base * Number(model.image_ratio);
    case "audio_input":
      return model.audio_ratio == null ? Number.NaN : base * Number(model.audio_ratio);
    case "audio_output":
      return model.audio_ratio == null || model.audio_completion_ratio == null
        ? Number.NaN
        : base * Number(model.audio_ratio) * Number(model.audio_completion_ratio);
  }
}

function formatUsd(value: number): string {
  if (!Number.isFinite(value)) return "-";
  const digits = Math.abs(value) >= 1 ? 4 : 6;
  const formatted = new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: digits,
  }).format(value);
  return formatted.replace(/(\.\d*?[1-9])0+$/, "$1").replace(/\.0+$/, "");
}

function emptyPricingData(): PricingData {
  return {
    models: [],
    vendors: [],
    groupRatio: {},
    groupModelRatio: {},
    usableGroup: {},
    supportedEndpoint: {},
    autoGroups: [],
    pricingAvailable: false,
  };
}
