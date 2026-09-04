import {
  discountedPriceUsd,
  formatUsdPrice,
  getBestGroupRatio,
  getOfficialPriceUsd,
  getVendorName,
  buildEffectiveGroupRatio,
  isTokenBasedModel,
  parseTags,
  resolveModelDisplayPrice,
  sortPricingModelsBySeries,
  type GroupModelRatio,
  type PricingData,
  type ModelDirectoryMetadata,
  type PricingModel,
  type ResolvedModelDisplayPrice,
} from "./pricing";

export type HomePricedModel = {
  name: string;
  vendor: string;
  /** Tags configured on model metadata in the console. */
  tags?: string[];
  displayWeight?: number;
  official: string;
  discounted: string;
  officialUsd: number;
  discountedUsd: number;
  priceUnit?: string;
  pricePrefix?: string;
  billingUnit?: "token" | "request" | "second";
  inputFilterUsd?: number;
  outputFilterUsd?: number;
  input?: string;
  inputOfficial?: string;
  output?: string;
  outputOfficial?: string;
  cache?: string;
  cacheOfficial?: string;
  billing?: string;
  capabilities?: string[];
  endpointTypes?: string[];
  // Directory-only metadata, sourced from model-directory-meta rather than the
  // pricing payload. Optional so home/pricing callers are unaffected.
  series?: string;
  contextTokens?: number | null;
  top10?: number;
  directoryMetadata?: ModelDirectoryMetadata;
  // Lobehub static-svg icon key rendered by ModelLogo; derived from the model
  // name because the pricing payload's icon fields are empty in production.
  iconKey: string;
};

const ICON_KEY_PATTERNS: Array<[RegExp, string]> = [
  [/^(gpt|o\d|dall-e|sora|codex)/i, "openai"],
  [/^claude/i, "claude-color"],
  [/^(gemini|imagen|veo)/i, "gemini-color"],
  [/^deepseek/i, "deepseek-color"],
  [/^qwen/i, "qwen-color"],
  [/^glm|^chatglm/i, "chatglm-color"],
  [/^kimi|^moonshot/i, "kimi-color"],
  [/^grok/i, "grok"],
  [/^llama/i, "meta-color"],
  [/^mistral/i, "mistral-color"],
  [/^doubao/i, "doubao-color"],
  [/^seedance/i, "bytedance-color"],
  [/^minimax/i, "minimax-color"],
];

export function modelIconKey(modelName: string, vendor: string): string {
  for (const [pattern, key] of ICON_KEY_PATTERNS) {
    if (pattern.test(modelName)) return key;
  }
  return vendor.toLowerCase();
}

// One flagship per vendor for the hero price-comparison card — spans Western +
// Chinese vendors so it reads "many models", not just OpenAI/Anthropic. Vendor-
// driven (not name-regex) so it stays robust as model names churn.
const FLAGSHIP_VENDORS: Array<{ label: string; match: RegExp }> = [
  { label: "OpenAI", match: /openai/i },
  { label: "Anthropic", match: /anthropic|claude/i },
  { label: "Google", match: /google|gemini/i },
  { label: "DeepSeek", match: /deepseek/i },
  { label: "Qwen", match: /qwen|alibaba|阿里|通义/i },
  { label: "Zhipu", match: /zhipu|智谱|glm/i },
  { label: "xAI", match: /xai|grok/i },
];
// Variants that never read as "the flagship" of a family.
const NON_FLAGSHIP =
  /[-_.](mini|nano|lite|flash|haiku|preview|codex|image|audio|realtime|embedding|turbo|thinking|exp|deepsearch|tts|ocr)/i;

export function pickFlagshipModels(data: PricingData, limit = 7): HomePricedModel[] {
  const priced = pricedTokenModels(data);
  const rows: HomePricedModel[] = [];
  const seenVendors = new Set<string>();
  for (const vendor of FLAGSHIP_VENDORS) {
    const forVendor = priced.filter((model) => {
      const name = model.vendor_name ?? getVendorName(model, data.vendors);
      return vendor.match.test(name) || vendor.match.test(model.model_name);
    });
    if (forVendor.length === 0) continue;
    // Prefer a "real" flagship (drop mini/lite/preview/etc), highest official
    // price first — but ignore placeholder-priced outliers (some models carry a
    // sentinel ~$75 list price). Fall back to any priced model from the vendor.
    const SANE_MAX = 12; // official $/1M input; filters sentinel pricing
    const clean = forVendor.filter((model) => !NON_FLAGSHIP.test(model.model_name));
    const byPriceDesc = (a: PricingModel, b: PricingModel) => getOfficialPriceUsd(b) - getOfficialPriceUsd(a);
    const flagship =
      clean.filter((model) => getOfficialPriceUsd(model) <= SANE_MAX).sort(byPriceDesc)[0] ??
      clean.sort(byPriceDesc)[0] ??
      forVendor.filter((model) => getOfficialPriceUsd(model) <= SANE_MAX).sort(byPriceDesc)[0] ??
      forVendor.sort(byPriceDesc)[0];
    if (!flagship || seenVendors.has(vendor.label)) continue;
    seenVendors.add(vendor.label);
    rows.push(toHomeRow(flagship, data));
    if (rows.length >= limit) break;
  }
  return rows;
}

export function buildHomeModelRows(data: PricingData): HomePricedModel[] {
  return sortPricingModelsBySeries(pricedTokenModels(data)).map((model) => toHomeRow(model, data));
}

export function finalHomePricedRowsByName(rows: HomePricedModel[]): HomePricedModel[] {
  return [...new Map(rows.map((row) => [row.name, row])).values()];
}

// Rows for an externally filtered/sorted model list (the /models directory).
// Includes per-request and display-priced models; rows carry unit metadata so
// the table can mix token, request, and second billing without a global suffix.
export function buildRowsForModels(
  models: PricingModel[],
  vendors: PricingData["vendors"],
  groupRatio: Record<string, number>,
  groupModelRatio: GroupModelRatio = {}
): HomePricedModel[] {
  return models
    .filter((model) => getOfficialPriceUsd(model) > 0 || resolveModelDisplayPrice(model, undefined, "plg", groupRatio) != null)
    .map((model) => {
      const imageGeneration = isImageGenerationModel(model);
      const official = getOfficialPriceUsd(model);
      // Per-model overrides in group_model_ratio beat the flat group ratio
      // during billing, so the quoted price has to apply them too — otherwise a
      // model priced below its group is advertised higher than it is charged.
      // The model's own group_model_ratio is the fallback, so a caller that
      // omits the payload-level map still quotes the billed rate.
      const overrides =
        Object.keys(groupModelRatio).length > 0 ? groupModelRatio : modelScopedGroupModelRatio(model);
      const effectiveGroupRatio = buildEffectiveGroupRatio(model, groupRatio, overrides);
      const bestGroupRatio = getBestGroupRatio(model, effectiveGroupRatio);
      const safeGroupRatio = bestGroupRatio > 0 ? bestGroupRatio : 1;
      const listed = official * safeGroupRatio;
      const vendor = model.vendor_name ?? getVendorName(model, vendors);
      const rawDisplayPrice = resolveDisplayPriceForModel(model, imageGeneration, effectiveGroupRatio);
      const imageDisplayPrice = imageGeneration ? resolveImageDisplayPrice(model, effectiveGroupRatio) : null;
      const displayPrice = imageDisplayPrice ?? (rawDisplayPrice && rawDisplayPrice.value > 0
        ? rawDisplayPrice
        : isTokenBasedModel(model) ? resolveUsableTokenDisplayPrice(model, "input", effectiveGroupRatio) : rawDisplayPrice);
      const officialDisplayPrice = displayPrice
        ? resolveModelDisplayPrice(model, displayPrice.dimension, "configured", effectiveGroupRatio) ??
          configuredDisplayPriceFromResolved(displayPrice)
        : null;
      const hasOfficialDisplayPrice = Boolean(officialDisplayPrice && officialDisplayPrice.value > 0);
      const billingUnit = modelBillingUnit(model, displayPrice?.unit);
      // Image models with no explicit per-image contract retain their native
      // token dimensions so the directory can show input/output/cache per 1M
      // tokens instead of mislabeling an input token amount as per-image.
      const exposeTokenDimensions = billingUnit === "token" && (!imageGeneration || !imageDisplayPrice);
      const inputPrice = exposeTokenDimensions ? resolveUsableTokenDisplayPrice(model, "input", effectiveGroupRatio) : null;
      const officialInputPrice = inputPrice ? resolveModelDisplayPrice(model, "input", "configured", effectiveGroupRatio) : null;
      const outputPrice = exposeTokenDimensions ? resolveUsableTokenDisplayPrice(model, "output", effectiveGroupRatio) : null;
      const officialOutputPrice = outputPrice ? resolveModelDisplayPrice(model, "output", "configured", effectiveGroupRatio) : null;
      const cachePrice = exposeTokenDimensions
        ? resolveUsableTokenDisplayPrice(model, "cache", effectiveGroupRatio)
        : null;
      const officialCachePrice = cachePrice ? resolveModelDisplayPrice(model, "cache", "configured", effectiveGroupRatio) : null;
      const usesParsedDisplayPrice = displayPrice?.source === "display";
      const discountedUsd = usesParsedDisplayPrice ? displayPrice.value : discountedPriceUsd(listed);
      // Video models are billed per second rather than by input/output tokens.
      // Their display contract therefore has no `input`/`output` dimensions;
      // expose the billed per-second rate as our output price so the directory
      // table does not render an empty output column for video rows.
      const displayedOutputPrice = billingUnit === "second" ? displayPrice : outputPrice;
      const displayedOfficialOutputPrice = billingUnit === "second" ? officialDisplayPrice : officialOutputPrice;
      const inputFilterUsd = billingUnit === "token" ? inputPrice?.value : discountedUsd;
      const outputFilterUsd = billingUnit === "token" ? outputPrice?.value : discountedUsd;
      const directoryMeta = model.directory_metadata;
      return {
        name: model.model_name,
        tags: parseTags(model.tags),
        displayWeight: model.display_weight ?? 0,
        // The metadata table is authoritative for the author: the payload
        // leaves vendor_id empty for some models (Macaron, Veo, Gemma) and
        // would otherwise fall back to the literal "AI".
        vendor: directoryMeta?.author ?? vendor,
        official: usesParsedDisplayPrice && hasOfficialDisplayPrice ? officialDisplayPrice!.text : formatUsdPrice(official),
        discounted: usesParsedDisplayPrice ? displayPrice.text : formatUsdPrice(discountedUsd),
        officialUsd: usesParsedDisplayPrice && hasOfficialDisplayPrice ? officialDisplayPrice!.value : official,
        discountedUsd,
        input: inputPrice?.text,
        inputOfficial: officialInputPrice?.text,
        output: displayedOutputPrice?.text,
        outputOfficial: displayedOfficialOutputPrice?.text,
        cache: cachePrice?.text,
        cacheOfficial: officialCachePrice?.text,
        billingUnit,
        inputFilterUsd,
        outputFilterUsd,
        billing: modelBillingLabel(model, displayPrice?.unit),
        capabilities: modelCapabilities(model),
        endpointTypes: model.supported_endpoint_types ?? [],
        priceUnit: displayPrice ? normalizeDisplayUnit(displayPrice.unit) : isTokenBasedModel(model) ? "per 1M tokens" : "per request",
        pricePrefix: displayPrice?.from ? "from" : undefined,
        series: directoryMeta?.series,
        contextTokens: directoryMeta?.context_tokens ?? null,
        top10: directoryMeta?.top_ten_rank,
        directoryMetadata: directoryMeta,
        iconKey: model.icon || model.vendor_icon || modelIconKey(model.model_name, vendor),
      };
    });
}

function isImageGenerationModel(model: PricingModel): boolean {
  const endpointTypes = model.supported_endpoint_types ?? [];
  return (
    endpointTypes.includes("image-generation") ||
    /(^|[-_.])(image|banana)/i.test(model.model_name)
  );
}

export function resolveImageDisplayPrice(
  model: PricingModel,
  groupRatio: Record<string, number>,
  variant: "plg" | "configured" = "plg",
): ResolvedModelDisplayPrice | null {
  // Tiered image models often have no parsed display_pricing entry. Their
  // billing expression still carries the canonical image-output coefficient
  // (for example `img_o * 120`, i.e. $120 per 1M image tokens). Image output
  // usage is normalized to 1,000 tokens per generated image for the public
  // catalog, so expose that coefficient as a comparable per-image rate.
  const tieredImagePrice = resolveTieredImageOutputPrice(model, groupRatio, variant);
  if (tieredImagePrice) return tieredImagePrice;

  const explicitImagePrice = resolveModelDisplayPrice(model, "image", variant, groupRatio);
  if (explicitImagePrice) return explicitImagePrice;

  const displayPrice = variant === "configured"
    ? resolveModelDisplayPrice(model, "image", "configured", groupRatio)
      ?? resolveModelDisplayPrice(model, "request", "configured", groupRatio)
      ?? resolveDisplayPriceForModel(model, true, groupRatio)
    : resolveDisplayPriceForModel(model, true, groupRatio);
  if (!displayPrice) return null;
  if (displayPrice.dimension === "image") return displayPrice;
  if (displayPrice.dimension === "request") {
    return { ...displayPrice, dimension: "image", unit: "/ image" };
  }
  // Token-priced image models are only treated as per-image when the payload
  // matches the known milli-dollar provider contract. Otherwise callers must
  // keep the native input/output/cache token dimensions visible.
  if (displayPrice.dimension !== "input" || !isMilliDollarImagePrice(model)) return null;
  const source = variant === "plg"
    ? displayPrice
    : resolveModelDisplayPrice(model, "input", "configured", groupRatio) ?? displayPrice;
  const configured = source.configured != null ? source.configured / 1000 : undefined;
  const plg = source.plg != null ? source.plg / 1000 : undefined;
  const value = source.value / 1000;
  return {
    ...source,
    text: formatUsdPrice(value),
    value,
    configuredValue: configured,
    configured,
    plg,
    dimension: "image",
    unit: "/ image",
  };
}

function resolveTieredImageOutputPrice(
  model: PricingModel,
  groupRatio: Record<string, number>,
  variant: "plg" | "configured",
): ResolvedModelDisplayPrice | null {
  if (model.billing_mode !== "tiered_expr" || !model.billing_expr) return null;
  const coefficients = [...model.billing_expr.matchAll(/(?:img_o\s*\*\s*(\d+(?:\.\d+)?)|(\d+(?:\.\d+)?)\s*\*\s*img_o)/gi)]
    .map((match) => Number(match[1] ?? match[2]))
    .filter((value) => Number.isFinite(value) && value > 0);
  if (coefficients.length === 0) return null;

  const uniqueCoefficients = [...new Set(coefficients)].sort((a, b) => a - b);
  const configured = uniqueCoefficients[0] / 1000;
  const ratio = variant === "plg"
    ? groupRatio.plg ?? model.group_ratio?.plg ?? 1
    : 1;
  if (!Number.isFinite(ratio) || ratio < 0) return null;
  const value = configured * ratio;
  return {
    text: formatUsdPrice(value),
    value,
    variant,
    dimension: "image",
    configuredValue: configured,
    configured,
    plg: configured * (groupRatio.plg ?? model.group_ratio?.plg ?? 1),
    unit: "/ image",
    from: uniqueCoefficients.length > 1,
    source: "display",
  };
}

function resolveDisplayPriceForModel(
  model: PricingModel,
  imageGeneration: boolean,
  groupRatio: Record<string, number>
): ResolvedModelDisplayPrice | null {
  const preferred = resolveModelDisplayPrice(model, imageGeneration ? "image" : undefined, "plg", groupRatio);
  if (!imageGeneration) return preferred;

  // Some image providers still publish their per-image amount through the
  // legacy request dimension. Reuse that amount but normalize the visible
  // dimension so the directory consistently reports "$… / image".
  const fallback =
    preferred ??
    resolveModelDisplayPrice(model, "request", "plg", groupRatio) ??
    resolveModelDisplayPrice(model, "input", "plg", groupRatio);
  if (!fallback) return null;
  if (fallback.dimension === "image") return fallback;
  if (fallback.dimension === "request") return { ...fallback, dimension: "image", unit: "/ image" };

  return fallback;
}

function configuredDisplayPriceFromResolved(price: ResolvedModelDisplayPrice): ResolvedModelDisplayPrice | null {
  if (price.configured == null || !Number.isFinite(price.configured)) return null;
  return {
    ...price,
    text: formatUsdPrice(price.configured),
    value: price.configured,
    variant: "configured",
  };
}

function isMilliDollarImagePrice(model: PricingModel): boolean {
  if (model.display_pricing?.billing_kind !== "token") return false;
  const input = model.display_pricing.prices.input;
  const output = model.display_pricing.prices.output;
  if (!input || !output || input.configured == null || output.configured == null) return false;
  return input.configured > 0 && output.configured / input.configured >= 50;
}

function pricedTokenModels(data: PricingData): PricingModel[] {
  const seen = new Set<string>();
  return data.models.filter((model) => {
    if (!isTokenBasedModel(model) || getOfficialPriceUsd(model) <= 0) return false;
    if (seen.has(model.model_name)) return false;
    seen.add(model.model_name);
    return true;
  });
}

// Strike-through = official vendor price; green = after the group ratio
// (i.e. 60-90% of official). The top-up bonus layer is retired.
function toHomeRow(model: PricingModel, data: PricingData): HomePricedModel {
  const official = getOfficialPriceUsd(model);
  const listed = official * getBestGroupRatio(model, data.groupRatio, data.groupModelRatio);
  const vendor = model.vendor_name ?? getVendorName(model, data.vendors);
  return {
    name: model.model_name,
    tags: parseTags(model.tags),
    displayWeight: model.display_weight ?? 0,
    vendor,
    official: formatUsdPrice(official),
    discounted: formatUsdPrice(discountedPriceUsd(listed)),
    officialUsd: official,
    discountedUsd: discountedPriceUsd(listed),
    iconKey: model.icon || model.vendor_icon || modelIconKey(model.model_name, vendor),
  };
}

/**
 * The per-model overrides carried on the model itself, reshaped like the
 * payload-level map. `enrichVendorNames` attaches these, so a caller holding an
 * enriched model but not the top-level map still resolves the billed ratio.
 */
function modelScopedGroupModelRatio(model: PricingModel): GroupModelRatio {
  const own = model.group_model_ratio;
  if (!own) return {};
  const scoped: GroupModelRatio = {};
  for (const [group, ratio] of Object.entries(own)) {
    if (typeof ratio === "number" && Number.isFinite(ratio)) scoped[group] = { [model.model_name]: ratio };
  }
  return scoped;
}

function normalizeDisplayUnit(unit: string): string {
  return unit.replace(/^\s*\/\s*/, "per ");
}

function modelBillingLabel(model: PricingModel, displayUnit?: string): string {
  const unit = modelBillingUnit(model, displayUnit);
  if (unit === "token") return "Token";
  if (unit === "second") return "Second";
  return "Request";
}

function modelBillingUnit(model: PricingModel, displayUnit?: string): "token" | "request" | "second" {
  if (displayUnit === "/ second") return "second";
  if (displayUnit === "/ request") return "request";
  if (isTokenBasedModel(model)) return "token";
  return "request";
}

function resolveUsableTokenDisplayPrice(
  model: PricingModel,
  dimension: "input" | "output" | "cache",
  groupRatio: Record<string, number>,
): ResolvedModelDisplayPrice | null {
  const parsed = resolveModelDisplayPrice(model, dimension, "plg", groupRatio);
  if (parsed && parsed.value >= 0) {
    const configuredValue = resolveModelDisplayPrice(model, dimension, "configured", groupRatio)?.value;
    if (parsed.value > 0 || (configuredValue != null && configuredValue > 0)) return parsed;
  }
  const configured = resolveModelDisplayPrice(model, dimension, "configured", groupRatio);
  if (!configured || configured.value <= 0) return null;
  const ratio = getBestGroupRatio(model, groupRatio);
  const value = configured.value * ratio;
  return {
    ...configured,
    text: formatUsdPrice(value),
    value,
    variant: "plg",
    plg: value,
  };
}

const CAPABILITY_LABELS: Record<string, string> = {
  audio: "Audio",
  api: "API",
  cache: "Cache",
  chat: "Chat",
  code: "Code",
  image: "Image",
  json: "JSON",
  reasoning: "Reasoning",
  realtime: "Realtime",
  response: "Responses",
  responses: "Responses",
  think: "Think",
  thinking: "Think",
  tool: "Tools",
  tools: "Tools",
  video: "Video",
  vision: "Vision",
  web: "Web",
};

function modelCapabilities(model: PricingModel): string[] {
  const labels: string[] = [];
  const add = (value: string | undefined) => {
    if (!value) return;
    const key = value.trim().toLowerCase();
    if (!key) return;
    const label = CAPABILITY_LABELS[key] ?? endpointCapabilityLabel(key);
    if (!label) return;
    if (!labels.some((existing) => existing.toLowerCase() === label.toLowerCase())) labels.push(label);
  };

  for (const tag of parseTags(model.tags)) add(tag);
  for (const endpoint of model.supported_endpoint_types ?? []) add(endpoint);
  if (labels.length === 0) add(isTokenBasedModel(model) ? "chat" : "api");
  return labels.slice(0, 3);
}

function endpointCapabilityLabel(value: string): string | undefined {
  if (value.includes("image")) return "Image";
  if (value.includes("video")) return "Video";
  if (value.includes("audio") || value.includes("tts") || value.includes("speech")) return "Audio";
  if (value.includes("embedding")) return "Embeddings";
  if (value.includes("realtime")) return "Realtime";
  if (value.includes("responses")) return "Responses";
  if (value.includes("chat")) return "Chat";
  return undefined;
}
