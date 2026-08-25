import type { ModelLandingKey } from "./model-landing";

/**
 * A prompt that can be copied into any image model's request editor.
 *
 * These are intentionally model-neutral.  The same scenario vocabulary makes
 * the image model pages comparable, while the model-specific output preview
 * remains owned by `model-media.ts` when one is available.
 */
export type ImagePromptTemplate = {
  id: string;
  label: ModelLandingKey;
  prompt: string;
  ratio: string;
  poster: string;
  tags: readonly string[];
};

const IMAGE_TEMPLATE_ASSET_BASE = "/assets/model-examples/image2";

/**
 * Use-case prompts for the image prompt library. Bracketed values are
 * deliberate fill-in slots: a visitor can replace them without rewriting the
 * composition, lighting, and delivery constraints that make a prompt useful.
 */
export const IMAGE_PROMPT_TEMPLATES: readonly ImagePromptTemplate[] = [
  {
    id: "product-hero",
    label: "Product mockups",
    prompt:
      "Create a premium ecommerce hero image for [product name]. Place the product on a clean [surface] with [brand colors] as subtle accents, soft directional daylight, accurate materials and packaging details, a balanced three-quarter camera angle, and generous negative space for a headline and call to action. No logos or readable text unless supplied in the reference image.",
    ratio: "4:5",
    poster: `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
    tags: ["product", "ecommerce", "hero"],
  },
  {
    id: "social-ad",
    label: "Ad creatives",
    prompt:
      "Design a scroll-stopping social ad for [product or offer] aimed at [audience]. Show one clear benefit in a natural, believable scene, use [brand colors] as a restrained accent, keep the subject large and legible at mobile size, leave safe space for a short headline and CTA, and return three visual variants with the same product identity. Avoid invented claims and tiny unreadable copy.",
    ratio: "1:1",
    poster: `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-creator.png`,
    tags: ["social", "campaign", "variants"],
  },
  {
    id: "catalog-variant",
    label: "Ecommerce images",
    prompt:
      "Create a consistent catalog image set for [product line]. Keep the camera height, focal length, background tone, and shadow direction fixed across [number] variants; change only [color or configuration]. Show the full product, preserve exact proportions and surface texture, use a neutral studio background, and leave clean margins for marketplace cropping. No extra accessories or text.",
    ratio: "1:1",
    poster: `${IMAGE_TEMPLATE_ASSET_BASE}/sports.png`,
    tags: ["catalog", "consistency", "marketplace"],
  },
  {
    id: "editorial-portrait",
    label: "Portrait",
    prompt:
      "Create an editorial portrait of [person or role] in [location]. Use soft window light from camera left, a natural expression, realistic skin texture, an uncluttered background, and wardrobe in [color palette]. Frame from chest up with a 4:5 composition, keep hands and facial features anatomically correct, and remove identifying details that were not provided.",
    ratio: "4:5",
    poster: `${IMAGE_TEMPLATE_ASSET_BASE}/portrait.png`,
    tags: ["portrait", "editorial", "people"],
  },
  {
    id: "product-ui",
    label: "Apps",
    prompt:
      "Create a polished launch visual for [app name], showing [core workflow] on a realistic device at a developer workstation. Use a dark neutral desk, focused monitor glow, subtle reflections, and a clear visual hierarchy; keep interface text abstract or supplied by the reference, with no invented logos or readable code. Leave the upper-right area open for launch copy.",
    ratio: "16:10",
    poster: `${IMAGE_TEMPLATE_ASSET_BASE}/saas.png`,
    tags: ["app", "product", "launch"],
  },
  {
    id: "food-editorial",
    label: "Food and beverage",
    prompt:
      "Create an editorial menu image for [dish or drink] served in [setting]. Show the hero item at a natural three-quarter angle with believable texture, controlled highlights, supporting ingredients used sparingly, warm directional light, and a clean area for menu copy. Keep the portion and colors appetizing, avoid invented labels, and do not add utensils or props that were not requested.",
    ratio: "4:5",
    poster: `${IMAGE_TEMPLATE_ASSET_BASE}/coffee.png`,
    tags: ["food", "menu", "editorial"],
  },
];

/**
 * Return a fresh array so callers can safely annotate examples for a specific
 * model without mutating the shared catalog.  `modelId` is accepted as part of
 * the public API to leave room for model-family ordering while keeping today's
 * scenario set consistent across every image model.
 */
export function getImagePromptTemplates(_modelId?: string): ImagePromptTemplate[] {
  return IMAGE_PROMPT_TEMPLATES.map((template) => ({
    ...template,
    tags: [...template.tags],
  }));
}

export function getImagePromptTemplate(templateId: string): ImagePromptTemplate | undefined {
  return IMAGE_PROMPT_TEMPLATES.find((template) => template.id === templateId);
}
