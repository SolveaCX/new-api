import { getLocalizedModelLandingConfig, getModelLandingConfigForPricingModel } from "../src/lib/model-landing";
import { LOCALES } from "../src/lib/locales";
import type { PricingModel } from "../src/lib/pricing";

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
for (const model of payload.data) {
  const source = getModelLandingConfigForPricingModel(model);
  for (const locale of locales) {
    const localized = getLocalizedModelLandingConfig(source, locale);
    for (const [name, read] of fields) {
      const original = read(source);
      const translated = read(localized);
      checks += 1;
      if (original && original === translated && /[A-Za-z]{3,}/.test(original)) {
        issues.push(`${model.model_name} [${locale}] ${name}: ${original}`);
      }
    }
    checks += 1;
    const landingText = JSON.stringify(localized.landingContent ?? "");
    if (/What is .* used for\?|Core capabilities and practical engineering value|The model is listed in Flatkey's live pricing catalog/i.test(landingText)) {
      issues.push(`${model.model_name} [${locale}] contains generated English landing copy`);
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
