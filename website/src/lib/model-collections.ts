import { classifyPublicModel, modelPublicPath } from "@/lib/model-public";
import { modelIconKey } from "@/lib/home-models";
import { formatResolvedModelDisplayPrice, getVendorName, resolveModelDisplayPrice, type PricingData, type PricingModel } from "@/lib/pricing";
import { LOCALES, type Locale } from "@/lib/locales";
import { MODEL_COLLECTION_COPY, type CollectionCopyKey, type ModelCollectionCopy } from "@/lib/model-collections-copy";

export type { ModelCollectionCopy } from "@/lib/model-collections-copy";
export type ModelCollectionDefinition = {
  slug: Exclude<CollectionCopyKey, "index">;
  icon: string;
  copy: Record<Locale, ModelCollectionCopy>;
  matches: (model: PricingModel) => boolean;
};
export type ModelCollectionsSeoCopy = { title: string; description: string };

export function getModelCollectionsSeoCopy(locale: Locale): ModelCollectionsSeoCopy {
  const copy = MODEL_COLLECTION_COPY[locale].index;
  return { title: copy.seoTitle, description: copy.seoDescription };
}

export function getModelCollectionSeoDescription(collection: ModelCollectionDefinition, locale: Locale): string {
  return getModelCollectionCopy(collection, locale).seoDescription;
}

const textOf = (model: PricingModel) =>
  [model.model_name, model.description, model.tags, ...(model.directory_metadata?.categories ?? [])]
    .filter(Boolean).join(" ").toLowerCase();

const hasActiveDiscount = (model: PricingModel) =>
  Object.values(model.display_pricing?.prices ?? {}).some((price) => {
    const configured = price?.configured;
    const plg = price?.plg;
    return typeof configured === "number" && Number.isFinite(configured) && configured >= 0 &&
      typeof plg === "number" && Number.isFinite(plg) && plg >= 0 && plg < configured;
  });

// Modalities describe inputs, not generated outputs. Keep generation services
// out of coding/roleplay/vision even if imported catalog tags say Programming.
function isVideoMusic(model: PricingModel): boolean {
  return (model.supported_endpoint_types ?? []).includes("video-to-music") || /video.to.music/i.test(model.model_name);
}
function isVideoGeneration(model: PricingModel): boolean {
  return !isVideoMusic(model) && ((model.supported_endpoint_types ?? []).some((type) => ["video", "openai-video"].includes(type)) ||
    /seedance|veo[-.]|grok-imagine-video|sora[-.]|kling|hunyuan-video/i.test(model.model_name));
}
function isSpeechGeneration(model: PricingModel): boolean {
  return /(?:^|[-_/])tts(?:$|[-_/])|text.to.speech|speech synthesis|voice generation/i.test(textOf(model));
}
function isImageGeneration(model: PricingModel): boolean {
  return !isVideoGeneration(model) && !isVideoMusic(model) && !isSpeechGeneration(model) &&
    (classifyPublicModel(model) === "image" || /gemini.*image|grok-imagine-image|gpt-image/i.test(model.model_name));
}
function isTextGeneration(model: PricingModel): boolean {
  return !isVideoGeneration(model) && !isVideoMusic(model) && !isImageGeneration(model) && !isSpeechGeneration(model) &&
    !/embedding|rerank|whisper|speech.to.text|transcription/i.test(model.model_name);
}

function defineCollection(slug: ModelCollectionDefinition["slug"], icon: string, matches: ModelCollectionDefinition["matches"]): ModelCollectionDefinition {
  return { slug, icon, matches, copy: Object.fromEntries(LOCALES.map((locale) => [locale, MODEL_COLLECTION_COPY[locale][slug]])) as Record<Locale, ModelCollectionCopy> };
}

// This order is shared by the index, related links, static routes and sitemap.
export const MODEL_COLLECTIONS: ModelCollectionDefinition[] = [
  defineCollection("discounted-models", "%", hasActiveDiscount),
  defineCollection("coding", "⌘", (model) => isTextGeneration(model) && /coding|programming|code generation|code review|debug|coder/i.test(textOf(model))),
  defineCollection("roleplay-creative-writing", "✎", (model) => isTextGeneration(model) && /roleplay|creative writing|character chat|storytelling|creative/i.test(textOf(model))),
  defineCollection("image-generation", "✦", isImageGeneration),
  defineCollection("video-generation", "▷", isVideoGeneration),
  defineCollection("audio-generation-models", "♫", (model) => isSpeechGeneration(model) || isVideoMusic(model)),
  defineCollection("vision-models", "◉", (model) => isTextGeneration(model) && model.directory_metadata?.modalities?.includes("image") === true),
  defineCollection("text-to-speech-models", "◖", isSpeechGeneration),
];

// Preserve links from the previous taxonomy without presenting unsupported
// categories. General-purpose discovery belongs in the complete model directory.
export function getLegacyCollectionDestination(slug: string): string | null {
  if (slug === "general-purpose-models") return "/models";
  return ["tool-calling", "free-models", "openclaw-models", "text-embedding-models", "speech-to-text-models", "rerank-models"].includes(slug) ? "/collections" : null;
}

export function getModelCollection(slug: string): ModelCollectionDefinition | null {
  return MODEL_COLLECTIONS.find((collection) => collection.slug === slug) ?? null;
}

export function getAvailableModelCollections(models: PricingModel[]): ModelCollectionDefinition[] {
  // The eight editorial pages remain discoverable even during a catalog outage.
  void models;
  return [...MODEL_COLLECTIONS];
}

export function getModelCollectionPathnames(models?: PricingModel[]): string[] {
  const collections = models ? getAvailableModelCollections(models) : MODEL_COLLECTIONS;
  return collections.map((collection) => `/collections/${collection.slug}`);
}

export function getModelCollectionCopy(collection: ModelCollectionDefinition, locale: Locale): ModelCollectionCopy {
  return collection.copy[locale] ?? collection.copy.en;
}

export function selectCollectionModels(collection: ModelCollectionDefinition, models: PricingModel[], limit?: number): PricingModel[] {
  const matched = models.filter(collection.matches);
  return limit == null ? matched : matched.slice(0, limit);
}

export function modelCardData(model: PricingModel, pricing: PricingData, fallbackDescription = "") {
  const vendor = model.vendor_name ?? getVendorName(model, pricing.vendors);
  const price = resolveModelDisplayPrice(model, undefined, "plg", pricing.groupRatio);
  return {
    href: modelPublicPath(model.model_name),
    name: model.featured_config?.display_name || model.model_name,
    vendor,
    // The public pricing payload often leaves icon fields empty. Resolve the
    // official vendor mark from the model family/vendor instead of passing the
    // full model id to the logo component (which would fall back to initials).
    iconKey: model.icon || model.vendor_icon || modelIconKey(model.model_name, vendor),
    description: model.featured_config?.description || model.description || model.vendor_description || fallbackDescription,
    context: model.directory_metadata?.context_tokens,
    price: price ? formatResolvedModelDisplayPrice(price) : null,
  };
}
