import { getLocalizedModelLandingConfig, getModelLandingConfigForPricingModel } from "../src/lib/model-landing";
import { LOCALES } from "../src/lib/locales";
import type { PricingModel } from "../src/lib/pricing";
import { getImagePromptTemplates, localizeImagePromptText } from "../src/lib/image-prompt-templates";
import { getVideoPromptTemplates, localizeVideoPromptText } from "../src/lib/video-prompt-templates";

type PricingPayload = { success?: boolean; data?: PricingModel[] };

const origin = process.env.APP_CONSOLE_ORIGIN ?? "https://console.flatkey.ai";
const group = process.env.MODEL_DETAIL_AUDIT_GROUP;
const url = new URL("/api/website/pricing", origin);
if (group) url.searchParams.set("group", group);

const response = await fetch(url, { headers: { accept: "application/json" } });
if (!response.ok) throw new Error(`pricing fetch failed: ${response.status}`);
const payload = (await response.json()) as PricingPayload;
if (payload.success !== true || !Array.isArray(payload.data)) throw new Error("pricing payload invalid");

const locales = LOCALES.filter((locale) => locale !== "en");
const fields = [
  ["hero.title", (config: ReturnType<typeof getModelLandingConfigForPricingModel>) => config.landingContent?.hero?.title],
  ["capabilitiesTitle", (config: ReturnType<typeof getModelLandingConfigForPricingModel>) => config.landingContent?.capabilitiesTitle],
  ["faq.question", (config: ReturnType<typeof getModelLandingConfigForPricingModel>) => config.landingContent?.faq?.[0]?.question],
  ["faq.answer", (config: ReturnType<typeof getModelLandingConfigForPricingModel>) => config.landingContent?.faq?.[0]?.answer],
] as const;

const issues: string[] = [];
let checks = 0;

function visibleConfig(config: ReturnType<typeof getModelLandingConfigForPricingModel>) {
  const content = config.landingContent;
  return {
    positioning: config.positioning,
    useCases: config.useCases,
    rows: config.rows,
    faq: config.faq,
    hero: content?.hero && { description: content.hero.description, provider: content.hero.provider, referencePrice: content.hero.referencePrice },
    performance: content?.performance,
    activity: content?.activity,
    pricing: content?.pricing,
    capabilities: content?.capabilities,
    capabilitiesEyebrow: content?.capabilitiesEyebrow,
    capabilitiesTitle: content?.capabilitiesTitle,
    capabilitiesDescription: content?.capabilitiesDescription,
    comparison: content?.comparison,
    promptLibraryTitle: content?.promptLibraryTitle,
    promptLibraryDescription: content?.promptLibraryDescription,
    why: content?.why,
    api: content?.api,
    related: content?.related,
    faqTitle: content?.faqTitle,
    faqDescription: content?.faqDescription,
  };
}

function flattenStrings(value: unknown, path = "", output: Array<[string, string]> = []) {
  if (typeof value === "string") output.push([path, value]);
  else if (Array.isArray(value)) value.forEach((item, index) => flattenStrings(item, `${path}[${index}]`, output));
  else if (value && typeof value === "object") Object.entries(value).forEach(([key, item]) => flattenStrings(item, `${path}.${key}`, output));
  return output;
}

function isEnglishSentence(value: string) {
  if (!/^[\x00-\x7F]+$/.test(value) || value.trim().split(/\s+/).length < 4) return false;
  if (/^[\d$.,%/ ()·×–-]+$/.test(value)) return false;
  if (/^(GPT|Gemini|Claude|DeepSeek|Kimi|Seedance|Flatkey|OpenAI|Anthropic|Google|Alibaba|MiniMax|Qwen|Sonilo|[\w.-]+ API)$/i.test(value.trim())) return false;
  return true;
}

for (const model of payload.data) {
  const source = getModelLandingConfigForPricingModel(model);
  const sourceStrings = new Map(flattenStrings(visibleConfig(source)));
  for (const locale of locales) {
    const localized = getLocalizedModelLandingConfig(source, locale);
    for (const [name, read] of fields) {
      const original = read(source);
      const translated = read(localized);
      checks += 1;
      if (original && original === translated && isEnglishSentence(original)) {
        issues.push(`${model.model_name} [${locale}] ${name}: ${original}`);
      }
    }
    for (const [path, value] of flattenStrings(visibleConfig(localized))) {
      checks += 1;
      if (value === sourceStrings.get(path) && !path.includes(".comparison.rows") && isEnglishSentence(value)) {
        issues.push(`${model.model_name} [${locale}] ${path}: ${value}`);
      }
    }
    const kind = source.generator?.kind;
    if (kind === "image" || kind === "video") {
      const reviewed = kind === "image" ? getImagePromptTemplates(source.modelId, locale) : getVideoPromptTemplates(source.modelId, locale);
      const englishReviewed = kind === "image" ? getImagePromptTemplates(source.modelId, "en") : getVideoPromptTemplates(source.modelId, "en");
      const configured = source.landingContent?.promptLibrary ?? [];
      const prompts = reviewed.length > 0
        ? reviewed.map((item, index) => ({ localized: item.prompt, source: englishReviewed[index]?.prompt }))
        : configured.map((item) => ({ localized: kind === "image" ? localizeImagePromptText(item.prompt, locale) : localizeVideoPromptText(item.prompt, locale), source: item.prompt }));
      checks += prompts.length;
      if (prompts.some(({ localized, source }) => source && localized === source && isEnglishSentence(source))) {
        issues.push(`${model.model_name} [${locale}] contains an untranslated media prompt`);
      }
    }
  }
}

console.log(`Audited ${payload.data.length} models × ${locales.length} non-English locales (${checks} checks).`);
if (issues.length > 0) {
  console.error(`Found ${issues.length} localization issues:`);
  for (const issue of issues) console.error(`- ${issue}`);
  process.exitCode = 1;
} else {
  console.log("No model-detail localization issues found.");
}
