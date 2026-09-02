import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { getImagePromptTemplates } from "../src/lib/image-prompt-templates";
import { getVideoPromptTemplates, VIDEO_PROFESSION_MODEL_IDS } from "../src/lib/video-prompt-templates";

const imageOutput = process.env.IMAGE_CATALOG_OUTPUT;
const videoOutput = process.env.VIDEO_CATALOG_OUTPUT;
const sourceCommit = process.env.MODEL_DETAIL_SOURCE_COMMIT ?? "origin/main";
const capturedAt = process.env.MODEL_DETAIL_CAPTURED_AT ?? new Date().toISOString().slice(0, 10);
const imageModelIds = ["gpt-image-2", "gemini-2-5-flash-image", "gemini-3-pro-image", "gemini-3-1-flash-image", "gemini-3-1-flash-lite-image", "grok-imagine-image", "grok-imagine-image-pro", "grok-imagine-image-quality", "nano-banana-pro-preview"];
const imageIndustries: Record<string, string> = { "product-hero": "gaming", "social-ad": "sports-fitness", "catalog-variant": "ecommerce-retail", "editorial-portrait": "media-entertainment", "product-ui": "marketing-advertising", "food-editorial": "media-entertainment" };
const imageCategories: Record<string, string> = { "product-hero": "game-ui", "social-ad": "sports-broadcast", "catalog-variant": "commercial", "editorial-portrait": "cinematic-storyboard", "product-ui": "physical-storytelling", "food-editorial": "historical-documentary" };
const videoIndustries: Record<string, string> = { "micro-drama-comic": "media-entertainment", "advertising-ecommerce": "ecommerce-retail", "film-concept-production": "media-entertainment", "game-art-animation": "gaming", "creator-explainer": "creator-social", "music-visual-art": "media-entertainment" };
const videoCategories: Record<string, string> = { "micro-drama-comic": "storyboard", "advertising-ecommerce": "commercial", "film-concept-production": "cinematic", "game-art-animation": "reference-driven", "creator-explainer": "pov-fpv", "music-visual-art": "music-video" };
const source = (model: string) => ({ label: "Flatkey model detail", platform: "Flatkey generated", url: `https://flatkey.ai/models/${model}`, license: "Flatkey-owned", captured_at: capturedAt, model_detail_source: sourceCommit });

function imageEntries() {
  return imageModelIds.flatMap((model) => getImagePromptTemplates(model, "en").map((template) => {
    const assetId = `model-detail-image-${model}-${template.id}`;
    return { slug: assetId, assetId, model, industry: imageIndustries[template.id] ?? "marketing-advertising", category: imageCategories[template.id] ?? "commercial", title: { en: `${model} — ${template.label}` }, description: { en: `Reviewed ${template.label.toLowerCase()} output from the ${model} model detail page.` }, prompt: template.prompt, tags: ["model-detail", "reviewed", ...template.tags], artifact: { assetId, kind: "image", url: template.poster, ratio: template.ratio, source: "model-detail" }, source: source(model), status: "published" };
  }));
}

function videoEntries() {
  return VIDEO_PROFESSION_MODEL_IDS.flatMap((model) => getVideoPromptTemplates(model, "en").map((template) => {
    const assetId = `model-detail-video-${model}-${template.professionId}`;
    return { slug: assetId, assetId, model, industry: videoIndustries[template.professionId] ?? "media-entertainment", category: videoCategories[template.professionId] ?? "cinematic", title: { en: `${model} — ${template.label}` }, description: { en: `Reviewed ${template.label.toLowerCase()} clip from the ${model} model detail page.` }, prompt: template.prompt, tags: ["model-detail", "reviewed", ...template.tags], artifact: { assetId, kind: "video", url: template.video, ...(template.poster ? { poster: template.poster } : {}), ratio: template.ratio, duration: template.duration, source: "model-detail" }, source: source(model), status: "published" };
  }));
}

async function writeCatalog(output: string | undefined, kind: "image" | "video", entries: unknown[]) {
  if (!output) return;
  await mkdir(path.dirname(output), { recursive: true });
  await writeFile(output, `${JSON.stringify({ schemaVersion: "1.0", kind: `${kind}-model-detail-assets`, repository: kind === "image" ? "https://github.com/flatkey-ai/awesome-images" : "https://github.com/flatkey-ai/awesome-seedance-prompts", generatedFrom: `new-api:${sourceCommit}:website/src/lib/${kind}-prompt-templates.ts`, capturedAt, entries }, null, 2)}\n`);
  console.log(`wrote ${entries.length} ${kind} model-detail entries to ${output}`);
}

await writeCatalog(imageOutput, "image", imageEntries());
await writeCatalog(videoOutput, "video", videoEntries());
