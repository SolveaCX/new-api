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
const AWESOME_IMAGE_ASSET_BASE = "/assets/prompts/awesome-images";

/**
 * Poster variants stay inside the same industry lane as their prompt. The
 * model id only chooses between compatible references; it never turns a food
 * brief into a random medical or sci-fi thumbnail.
 */
const IMAGE_TEMPLATE_POSTER_VARIANTS: Record<string, readonly string[]> = {
  "product-hero": [
    `${AWESOME_IMAGE_ASSET_BASE}/ecommerce-skincare.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/skincare.png`,
  ],
  "social-ad": [
    `${AWESOME_IMAGE_ASSET_BASE}/ugc-coffee-ad.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-creator.png`,
  ],
  "catalog-variant": [
    `${AWESOME_IMAGE_ASSET_BASE}/sports-shoe.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/sports.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/streetwear-lookbook.png`,
  ],
  "editorial-portrait": [
    `${IMAGE_TEMPLATE_ASSET_BASE}/portrait.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/flatkey-image2-creator.png`,
  ],
  "product-ui": [
    `${AWESOME_IMAGE_ASSET_BASE}/fitness-app.png`,
    `${IMAGE_TEMPLATE_ASSET_BASE}/saas.png`,
  ],
  "food-editorial": [
    `${IMAGE_TEMPLATE_ASSET_BASE}/coffee.png`,
    `${AWESOME_IMAGE_ASSET_BASE}/ugc-coffee-ad.png`,
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
      "For consumer brands and growth teams, create a 9:16 UGC ad cover for [product] aimed at [audience] on TikTok or Reels. Show a real creator using it in [home, cafe, or everyday setting], with natural hands and skin, authentic phone-camera framing, and the product clearly visible. Keep the top 18% safe for a headline and CTA. No generated text, invented logos, exaggerated claims, plastic skin, or extra products.",
    ratio: "9:16",
    poster: `${AWESOME_IMAGE_ASSET_BASE}/ugc-coffee-ad.png`,
    tags: ["social", "campaign", "variants"],
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
    label: "Content creators & knowledge streamers",
    prompt:
      "For creator, community, and customer-facing teams, create a 4:5 professional avatar for [person or role] used on [profile, support, or about page]. Keep the face natural, eyes clear, skin and clothing texture realistic, and the background uncluttered with one subtle identity cue. Use soft key light and a centered chest-up crop. Preserve supplied identity details; no invented names, logos, text, or identifying information.",
    ratio: "4:5",
    poster: `${AWESOME_IMAGE_ASSET_BASE}/cyber-portrait.png`,
    tags: ["portrait", "editorial", "people"],
  },
  {
    id: "product-ui",
    label: "Apps",
    prompt:
      "For SaaS and mobile-product teams, create a 16:9 product-launch visual for [app name] showing [core workflow] on a realistic phone or laptop. Place the supplied UI in a clean branded scene, preserve its hierarchy and supplied text, use a restrained [brand palette], and leave the right side open for headline copy. No developer-tool interface, invented logo, readable code, fake metrics, or tiny unreadable interface text.",
    ratio: "16:9",
    poster: `${AWESOME_IMAGE_ASSET_BASE}/fitness-app.png`,
    tags: ["app", "product", "launch"],
  },
  {
    id: "food-editorial",
    label: "Food and beverage",
    prompt:
      "For restaurants and beverage brands, create a 4:5 menu and delivery-platform hero for [dish or drink] served by [restaurant type]. Show the requested portion and ingredients with believable texture, plated on [surface] from a top-down or three-quarter angle, with warm directional light and a clean area for dish name and price. Do not add unrequested ingredients, utensils, labels, text, or props.",
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

/**
 * Pick deterministic, industry-compatible posters for one model's template
 * cards. The model id chooses a compatible variant so two model pages do not
 * look like a copy-paste, while the scenario-to-industry relationship stays
 * stable (for example, a food brief always gets a food image).
 */
export function getImagePromptTemplateFallbackPosters(modelId = ""): string[] {
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
