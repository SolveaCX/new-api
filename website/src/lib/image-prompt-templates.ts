import type { ModelLandingKey } from "./model-landing";

/**
 * A prompt that can be copied into any image model's request editor.
 *
 * These are intentionally industry-led rather than style-led. Each card tells
 * a concrete team what deliverable to make, where it will be used, and which
 * production constraints matter. Any real model-generated media remains owned
 * by `model-media.ts`; these local posters are clearly marked as templates.
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
const MODEL_EXAMPLES_ASSET_BASE = "/assets/model-examples";
const MODEL_PAGES_ASSET_BASE = "/assets/model-pages";
const AWESOME_IMAGE_ASSET_BASE = "/assets/prompts/awesome-images";
const CLI_ASSET_BASE = "/assets/cli";
const IMAGE_BUDDY_ASSET_BASE = "/use-case/image-buddy";

/**
 * Poster variants stay inside the same industry lane as their prompt. Every
 * reference in these pools is a product, space, interface, food, or packaging
 * still — never a real person, creator, model, portrait, hand, or workstation.
 * The model id only chooses between compatible references; it never turns a
 * food brief into a random medical or sci-fi thumbnail.
 */
const IMAGE_TEMPLATE_POSTER_VARIANTS: Record<string, readonly string[]> = {
  "product-hero": [
    `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${CLI_ASSET_BASE}/product-reveal.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro-reference.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
    `${CLI_ASSET_BASE}/localized-variants.png`,
  ],
  "social-ad": [
    `${CLI_ASSET_BASE}/localized-variants.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${CLI_ASSET_BASE}/product-reveal.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
    `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro.png`,
    `${MODEL_PAGES_ASSET_BASE}/gpt-image-2-hero.png`,
  ],
  "catalog-variant": [
    `${AWESOME_IMAGE_ASSET_BASE}/sports-shoe.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/sports.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro-reference.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro.png`,
    `${CLI_ASSET_BASE}/localized-variants.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${MODEL_PAGES_ASSET_BASE}/image-api-hero.png`,
  ],
  "editorial-portrait": [
    `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-hotel.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/coffee.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/food-motion.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${CLI_ASSET_BASE}/product-reveal.png`,
    `${MODEL_PAGES_ASSET_BASE}/gpt-image-2-hero.png`,
    `${MODEL_PAGES_ASSET_BASE}/image-api-hero.png`,
  ],
  "product-ui": [
    `${MODEL_PAGES_ASSET_BASE}/image-api-hero.png`,
    `${MODEL_PAGES_ASSET_BASE}/gpt-image-2-hero.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/liquid-bento.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/ai-agent-poster.png`,
    `${MODEL_PAGES_ASSET_BASE}/gemini-api-hero.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro-reference.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
  ],
  "food-editorial": [
    `${MODEL_EXAMPLES_ASSET_BASE}/food-motion.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/coffee.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-hotel.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${CLI_ASSET_BASE}/localized-variants.png`,
    `${CLI_ASSET_BASE}/product-reveal.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
  ],
};

/**
 * Canonical image pages get a curated six-poster set instead of a shared
 * random-looking rotation. The six positions line up with the six industry
 * templates above, and each model's set is intentionally different.
 */
const IMAGE_MODEL_POSTER_SETS: Record<string, readonly string[]> = {
  "gpt-image-2": [
    `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    `${CLI_ASSET_BASE}/localized-variants.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/sports-shoe.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-hotel.png`,
    `${MODEL_PAGES_ASSET_BASE}/image-api-hero.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/food-motion.png`,
  ],
  "gemini-2-5-flash-image": [
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/sports.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
    `${MODEL_PAGES_ASSET_BASE}/gpt-image-2-hero.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/coffee.png`,
  ],
  "gemini-3-pro-image": [
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
    `${CLI_ASSET_BASE}/product-reveal.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro-reference.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/coffee.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/liquid-bento.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
  ],
  "gemini-3-1-flash-image": [
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro.png`,
    `${MODEL_EXAMPLES_ASSET_BASE}/food-motion.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/ai-agent-poster.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-hotel.png`,
  ],
  "gemini-3-1-flash-lite-image": [
    `${CLI_ASSET_BASE}/product-reveal.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    `${CLI_ASSET_BASE}/localized-variants.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
    `${MODEL_PAGES_ASSET_BASE}/gemini-api-hero.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
  ],
  "grok-imagine-image": [
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/sports.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/liquid-bento.png`,
    `${CLI_ASSET_BASE}/localized-variants.png`,
  ],
  "grok-imagine-image-pro": [
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro-reference.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
    `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    `${CLI_ASSET_BASE}/product-reveal.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/ai-agent-poster.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/coffee.png`,
  ],
  "grok-imagine-image-quality": [
    `${IMAGE_BUDDY_ASSET_BASE}/premium-product-hero.jpg`,
    `${MODEL_EXAMPLES_ASSET_BASE}/product-macro.png`,
    `${CLI_ASSET_BASE}/campaign-hero.png`,
    `${MODEL_PAGES_ASSET_BASE}/gpt-image-2-hero.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/ai-agent-poster.png`,
    `${IMAGE_BUDDY_ASSET_BASE}/marketplace-main-image.jpg`,
  ],
  "nano-banana-pro-preview": [
    `${CLI_ASSET_BASE}/localized-variants.png`,
    `${MODEL_PAGES_ASSET_BASE}/gpt-image-2-hero.png`,
    `${MODEL_PAGES_ASSET_BASE}/image-api-hero.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-hotel.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/liquid-bento.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
  ],
};

/**
 * Industry prompts for the image prompt library. Bracketed values are
 * deliberate fill-in slots: a visitor can replace them without rewriting the
 * composition, lighting, and delivery constraints that make a prompt useful.
 */
export const IMAGE_PROMPT_TEMPLATES: readonly ImagePromptTemplate[] = [
  {
    id: "product-hero",
    label: "Product mockups",
    prompt:
      "For ecommerce and retail teams, create a marketplace hero image for [product] sold through [Amazon, Shopify, or store]. Show the supplied product exactly, preserving packaging, materials, proportions, and supplied brand marks. Use a clean [surface], a balanced three-quarter view, soft studio light, and 4:5 or 1:1 framing with safe space for price and CTA copy. No invented text, claims, accessories, watermark, or extra products.",
    ratio: "4:5",
    poster: `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    tags: ["product", "ecommerce", "hero"],
  },
  {
    id: "social-ad",
    label: "Ad creatives",
    prompt:
      "For consumer brands and growth teams, create a 9:16 product-first social ad for [product] aimed at [audience] on TikTok or Reels. Build a clear still-life scene with the supplied packaging, a bold [background color], controlled shadow, and a clean product silhouette that reads at mobile size. Keep the top 18% safe for a headline and CTA. No people, hands, faces, invented logos, exaggerated claims, generated text, or extra products.",
    ratio: "9:16",
    poster: `${CLI_ASSET_BASE}/localized-variants.png`,
    tags: ["social", "campaign", "product-still-life"],
  },
  {
    id: "catalog-variant",
    label: "Ecommerce images",
    prompt:
      "For fashion and sports retailers, create a consistent catalog set for [product line] with [number] colorways. Lock camera height, lens, background tone, crop, and shadow direction across every variant; change only [color or configuration]. Preserve exact proportions, sole or fabric texture, and marketplace-safe margins. No extra accessories, invented text, or drifting product identity.",
    ratio: "1:1",
    poster: `${AWESOME_IMAGE_ASSET_BASE}/sports-shoe.png`,
    tags: ["catalog", "consistency", "marketplace"],
  },
  {
    id: "editorial-portrait",
    label: "Hospitality and travel",
    prompt:
      "For hospitality and travel teams, create a 4:5 room or destination listing image for [hotel, resort, or rental]. Show the supplied interior or space with accurate architecture, materials, linens, and daylight direction; stage one clear focal area and leave safe space for room type and booking copy. No people, silhouettes, hands, invented signage, logos, text, or extra rooms.",
    ratio: "4:5",
    poster: `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-hotel.png`,
    tags: ["hospitality", "interior", "travel"],
  },
  {
    id: "product-ui",
    label: "Apps",
    prompt:
      "For SaaS and mobile-product teams, create a 16:9 product-launch visual for [app name] showing [core workflow] in a clean interface composition. Place the supplied UI in a restrained [brand palette], preserve its hierarchy and supplied labels, use clear cards and generous spacing, and leave the right side open for headline copy. No people, hands, faces, developer terminal, readable code, invented logo, fake metrics, or tiny unreadable interface text.",
    ratio: "16:9",
    poster: `${MODEL_PAGES_ASSET_BASE}/image-api-hero.png`,
    tags: ["app", "product", "launch"],
  },
  {
    id: "food-editorial",
    label: "Food and beverage",
    prompt:
      "For restaurants and beverage brands, create a 4:5 menu and delivery-platform hero for [dish or drink] served by [restaurant type]. Show the requested portion and ingredients with believable texture, plated on [surface] from a top-down or three-quarter angle, with warm directional light and a clean area for dish name and price. No people, hands, invented labels, text, unrequested ingredients, utensils, or props.",
    ratio: "4:5",
    poster: `${MODEL_EXAMPLES_ASSET_BASE}/food-motion.png`,
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

/**
 * Pick deterministic, industry-compatible posters for one model's template
 * cards. The model id chooses a compatible variant so two model pages do not
 * look like a copy-paste, while the scenario-to-industry relationship stays
 * stable (for example, a food brief always gets a food image).
 */
export function getImagePromptTemplateFallbackPosters(modelId = ""): string[] {
  const normalizedModelId = modelId
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  const curated = IMAGE_MODEL_POSTER_SETS[normalizedModelId];
  if (curated && curated.length >= IMAGE_PROMPT_TEMPLATES.length) {
    return curated.slice(0, IMAGE_PROMPT_TEMPLATES.length);
  }

  const hashFor = (value: string) => {
    let hash = 0;
    for (const character of value.trim().toLowerCase()) {
      hash = (hash * 31 + character.charCodeAt(0)) >>> 0;
    }
    return hash;
  };

  const used = new Set<string>();
  return IMAGE_PROMPT_TEMPLATES.map((template) => {
    const variants = IMAGE_TEMPLATE_POSTER_VARIANTS[template.id] ?? [template.poster];
    const start = hashFor(`${modelId}:${template.id}`) % variants.length;
    for (let offset = 0; offset < variants.length; offset += 1) {
      const candidate = variants[(start + offset) % variants.length];
      if (candidate && !used.has(candidate)) {
        used.add(candidate);
        return candidate;
      }
    }
    return variants[start] ?? template.poster;
  });
}

export function getImagePromptTemplate(templateId: string): ImagePromptTemplate | undefined {
  return IMAGE_PROMPT_TEMPLATES.find((template) => template.id === templateId);
}
