import { getImagePromptTemplateLocalFallbackPoster, getImagePromptTemplates } from "./image-prompt-templates";
import type { Locale } from "./locales";
import {
  getModelLandingConfig,
  getModelLandingConfigForModel,
  getModelLandingConfigForPricingModel,
  type ModelConfig,
  type ModelGeneratorConfig,
} from "./model-landing";
import { resolvePublicModel } from "./model-public";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "./pricing";
import { getVideoPromptTemplateLocalFallbackPoster, getVideoPromptTemplates } from "./video-prompt-templates";

export type PromptDetail = {
  id: string;
  modelSlug: string;
  modelId: string;
  modelName: string;
  kind: "image" | "video";
  generator: ModelGeneratorConfig;
  label: string;
  prompt: string;
  ratio: string;
  duration?: number;
  poster: string;
  fallbackPoster?: string;
  video?: string;
  siblings: readonly { id: string; label: string; prompt: string; poster: string; fallbackPoster?: string; video?: string }[];
};

/** Only publish prompt URLs backed by the same templates as model detail cards. */
export function getPromptDetailPathnames(configs: readonly ModelConfig[]): string[] {
  const paths = new Set<string>();
  for (const config of configs) {
    const kind = config.generator?.kind;
    const templates = kind === "video"
      ? getVideoPromptTemplates(config.modelId)
      : kind === "image"
        ? getImagePromptTemplates(config.modelId)
        : [];
    for (const template of templates) {
      const id = "professionId" in template ? template.professionId : template.id;
      paths.add(`/models/${config.slug}/prompts/${id}`);
    }
  }
  return [...paths];
}

/** Resolve the same template and media pair used by the model's library card. */
export async function getPromptDetail(modelSlug: string, promptId: string, locale: Locale): Promise<PromptDetail | null> {
  const pricing = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  const model = resolvePublicModel(pricing.models, modelSlug);
  const config = model
    ? getModelLandingConfigForPricingModel(model)
    : getModelLandingConfig(modelSlug) ?? getModelLandingConfigForModel(modelSlug);
  const kind = config?.generator?.kind;
  if (!config || (kind !== "image" && kind !== "video")) return null;

  const modelId = config.modelId || modelSlug;
  const canonicalSlug = config.slug === "seedance-api" ? "seedance-2.0" : config.slug;
  const base = { modelSlug: canonicalSlug, modelId, modelName: config.displayName, kind, generator: config.generator! };
  if (kind === "image") {
    const templates = getImagePromptTemplates(modelId, locale);
    const template = templates.find((item) => item.id === promptId);
    if (!template) return null;
    return {
      ...base,
      id: template.id,
      label: template.label,
      prompt: template.prompt,
      ratio: template.ratio,
      poster: template.poster,
      fallbackPoster: getImagePromptTemplateLocalFallbackPoster(modelId, template.poster),
      siblings: templates.map((item) => ({
        id: item.id,
        label: item.label,
        prompt: item.prompt,
        poster: item.poster,
        fallbackPoster: getImagePromptTemplateLocalFallbackPoster(modelId, item.poster),
      })),
    };
  }

  const templates = getVideoPromptTemplates(modelId, locale);
  const template = templates.find((item) => item.professionId === promptId);
  if (!template) return null;
  return {
    ...base,
    id: template.professionId,
    label: template.label,
    prompt: template.prompt,
    ratio: template.ratio,
    duration: template.duration,
    poster: template.poster,
    fallbackPoster: getVideoPromptTemplateLocalFallbackPoster(modelId, template.professionId),
    video: template.video,
    siblings: templates.map((item) => ({
      id: item.professionId,
      label: item.label,
      prompt: item.prompt,
      poster: item.poster || getVideoPromptTemplateLocalFallbackPoster(modelId, item.professionId) || "",
      fallbackPoster: getVideoPromptTemplateLocalFallbackPoster(modelId, item.professionId),
      video: item.video,
    })),
  };
}
