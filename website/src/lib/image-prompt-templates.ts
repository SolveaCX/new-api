/**
 * A prompt that can be copied into any image model's request editor.
 *
 * These are intentionally workflow-led rather than style-led. Each card tells
 * a concrete team what deliverable to make, where it will be used, and which
 * production constraints matter. Any real model-generated media remains owned
 * by `model-media.ts`; reviewed template posters may be served from the local
 * asset bundle or the public CDN.
 */
export type ImagePromptTemplate = {
  id: string;
  label: string;
  prompt: string;
  ratio: string;
  poster: string;
  tags: readonly string[];
};

/**
 * The first image a visitor sees in an image model's public Playground.
 *
 * Playground starters are deliberately scoped to one business scenario for
 * this release (ecommerce and retail), while each model gets its own product
 * brief and its own local poster.  These are starter references, not claims
 * that the current model generated the poster.
 */
export type ImagePlaygroundExample = {
  industry: "ecommerce-retail";
  prompt: string;
  poster: string;
};

const SELECTED_PLAYGROUND_ASSET_BASE = "/assets/prompts/selected-playground";
/**
 * Asset binding contract: these reviewed CDN images are the canonical media
 * for the six profession directions. The model Playground starter and prompt
 * library variants must keep using this registry; local artwork is permitted
 * only as an explicit load-error fallback, never as a data replacement.
 */
const GAME_UI_EQUIPMENT_CDN_BASE =
  "https://cdn.shulex-voc.com/flatkey/model-media/prompt-library/game-ui-equipment";
const SPORTS_BROADCAST_CDN_BASE =
  "https://cdn.shulex-voc.com/flatkey/model-media/prompt-library/sports-broadcast";
const BRAND_TVC_ECOMMERCE_CDN_BASE =
  "https://cdn.shulex-voc.com/flatkey/model-media/prompt-library/brand-tvc-ecommerce";
const CINEMATIC_STORYBOARD_CDN_BASE =
  "https://cdn.shulex-voc.com/flatkey/model-media/prompt-library/cinematic-storyboard";
const COMEDY_PHYSICAL_CDN_BASE =
  "https://cdn.shulex-voc.com/flatkey/model-media/prompt-library/comedy-physical";
const HISTORICAL_REVIVAL_CDN_BASE =
  "https://cdn.shulex-voc.com/flatkey/model-media/prompt-library/historical-revival";

/**
 * Poster variants stay inside the same workflow lane as their prompt. Every
 * reference in these pools is a reviewed visual reference — never a real
 * person, creator, model, portrait, hand, or workstation. The model id only
 * chooses between compatible references; it never turns a focused brief into a
 * random unrelated thumbnail.
 */
const IMAGE_TEMPLATE_POSTER_VARIANTS: Record<string, readonly string[]> = {
  "product-hero": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gpt-image-2.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-3-pro-image.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/grok-imagine-image.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/grok-imagine-image-pro.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/grok-imagine-image-quality.png`,
    `${GAME_UI_EQUIPMENT_CDN_BASE}/nano-banana-pro-preview.png`,
  ],
  "social-ad": [
    `${SPORTS_BROADCAST_CDN_BASE}/gpt-image-2.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-3-pro-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/grok-imagine-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/grok-imagine-image-pro.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/grok-imagine-image-quality.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/nano-banana-pro-preview.png`,
  ],
  "catalog-variant": [
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gpt-image-2.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-3-pro-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/grok-imagine-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/grok-imagine-image-pro.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/grok-imagine-image-quality.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/nano-banana-pro-preview.png`,
  ],
  "editorial-portrait": [
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gpt-image-2.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-3-pro-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/grok-imagine-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/grok-imagine-image-pro.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/grok-imagine-image-quality.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/nano-banana-pro-preview.png`,
  ],
  "product-ui": [
    `${COMEDY_PHYSICAL_CDN_BASE}/gpt-image-2.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-3-pro-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/grok-imagine-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/grok-imagine-image-pro.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/grok-imagine-image-quality.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/nano-banana-pro-preview.png`,
  ],
  "food-editorial": [
    `${HISTORICAL_REVIVAL_CDN_BASE}/gpt-image-2.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-3-pro-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/grok-imagine-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/grok-imagine-image-pro.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/grok-imagine-image-quality.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/nano-banana-pro-preview.png`,
  ],
};

/**
 * Canonical image pages get a curated six-poster set instead of a shared
 * random-looking rotation. The six positions line up with the six prompt
 * templates above, and each model's set is intentionally different.
 */
const IMAGE_MODEL_POSTER_SETS: Record<string, readonly string[]> = {
  "gpt-image-2": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gpt-image-2.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gpt-image-2.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gpt-image-2.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gpt-image-2.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gpt-image-2.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gpt-image-2.png`,
  ],
  "gemini-2-5-flash-image": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-2-5-flash-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-2-5-flash-image.png`,
  ],
  "gemini-3-pro-image": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-3-pro-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-3-pro-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-3-pro-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-3-pro-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-3-pro-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-3-pro-image.png`,
  ],
  "gemini-3-1-flash-image": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-3-1-flash-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-3-1-flash-image.png`,
  ],
  "gemini-3-1-flash-lite-image": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/gemini-3-1-flash-lite-image.png`,
  ],
  "grok-imagine-image": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/grok-imagine-image.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/grok-imagine-image.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/grok-imagine-image.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/grok-imagine-image.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/grok-imagine-image.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/grok-imagine-image.png`,
  ],
  "grok-imagine-image-pro": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/grok-imagine-image-pro.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/grok-imagine-image-pro.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/grok-imagine-image-pro.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/grok-imagine-image-pro.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/grok-imagine-image-pro.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/grok-imagine-image-pro.png`,
  ],
  "grok-imagine-image-quality": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/grok-imagine-image-quality.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/grok-imagine-image-quality.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/grok-imagine-image-quality.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/grok-imagine-image-quality.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/grok-imagine-image-quality.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/grok-imagine-image-quality.png`,
  ],
  "nano-banana-pro-preview": [
    `${GAME_UI_EQUIPMENT_CDN_BASE}/nano-banana-pro-preview.png`,
    `${SPORTS_BROADCAST_CDN_BASE}/nano-banana-pro-preview.png`,
    `${BRAND_TVC_ECOMMERCE_CDN_BASE}/nano-banana-pro-preview.png`,
    `${CINEMATIC_STORYBOARD_CDN_BASE}/nano-banana-pro-preview.png`,
    `${COMEDY_PHYSICAL_CDN_BASE}/nano-banana-pro-preview.png`,
    `${HISTORICAL_REVIVAL_CDN_BASE}/nano-banana-pro-preview.png`,
  ],
};

/**
 * The temporary picker selection for the nine canonical image model pages.
 * The order was supplied by the product review flow, so it intentionally
 * overrides the source feed's `model` field (the feed has only two source
 * model values). These are local copies so the detail pages do not depend on
 * remote image URLs at render time.
 */
const IMAGE_PLAYGROUND_SELECTED_POSTERS: Record<string, string> = {
  "gpt-image-2": `${SELECTED_PLAYGROUND_ASSET_BASE}/silhouette-universe-narrative-poster.jpg`,
  "gemini-2-5-flash-image": `${SELECTED_PLAYGROUND_ASSET_BASE}/three-day-travel-guide-card.jpg`,
  "gemini-3-pro-image": `${SELECTED_PLAYGROUND_ASSET_BASE}/museum-catalog-style-chinese-disassembly-infographic.jpg`,
  "gemini-3-1-flash-image": `${SELECTED_PLAYGROUND_ASSET_BASE}/high-end-skincare-product-poster.png`,
  "gemini-3-1-flash-lite-image": `${SELECTED_PLAYGROUND_ASSET_BASE}/ximen-qing-100-panel-storyboard.jpg`,
  "grok-imagine-image": `${SELECTED_PLAYGROUND_ASSET_BASE}/gta-6-livestream-gameplay-screenshot.jpg`,
  "grok-imagine-image-pro": `${SELECTED_PLAYGROUND_ASSET_BASE}/pet-brand.png`,
  "grok-imagine-image-quality": `${SELECTED_PLAYGROUND_ASSET_BASE}/book-cover.png`,
  "nano-banana-pro-preview": `${SELECTED_PLAYGROUND_ASSET_BASE}/real-estate-interior.png`,
};

/**
 * One distinct ecommerce/retail starter for each canonical image page. The
 * object keys use the same punctuation-free form as the model ids returned by
 * the pricing catalog (for example, `gemini-3.1-*` becomes `gemini-3-1-*`).
 */
const IMAGE_PLAYGROUND_EXAMPLES: Record<string, ImagePlaygroundExample> = {
  "gpt-image-2": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gpt-image-2"],
    prompt:
      "For an ecommerce skincare listing, create a 4:5 premium hero image of a frosted glass serum dropper on a pale aqua stone surface with fine water droplets. Keep the bottle proportions and cap shape exact, use soft daylight and a clean reflection, leave generous negative space for price and CTA copy, and deliver a product-only composition with no readable text, invented logo, extra products, hands, or watermark.",
  },
  "gemini-2-5-flash-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-2-5-flash-image"],
    prompt:
      "For a beauty marketplace listing, create a warm 1:1 hero still of a matte skincare bottle and matching cream jar on a travertine shelf. Preserve the supplied packaging, cap geometry, materials, and neutral palette; use soft window shadows, a clear front-facing silhouette, and safe margins for listing controls. No people, hands, invented labels, extra products, claims, or watermark.",
  },
  "gemini-3-pro-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-3-pro-image"],
    prompt:
      "For a sustainable retail catalog, create a clean 1:1 marketplace product photo of a brushed stainless-steel bottle centered on seamless white. Show the exact cylindrical body, lid, and metal grain with a soft grounded shadow, neutral color balance, and enough empty margin for catalog overlays. No text, logos, accessories, reflections of people, or additional objects.",
  },
  "gemini-3-1-flash-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-3-1-flash-image"],
    prompt:
      "For a premium skincare launch, create a 4:5 retail campaign image of a deep-green glass pump bottle on sculptural white stone. Keep the pump, bottle proportions, and glass reflections consistent; use directional botanical shadows, bright natural daylight, and a quiet upper-left area for campaign copy. No readable text, invented branding, people, hands, extra products, or watermark.",
  },
  "gemini-3-1-flash-lite-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-3-1-flash-lite-image"],
    prompt:
      "For a beauty brand product reveal, create a square ecommerce hero of a translucent emerald pump bottle on a dark graphite pedestal. Use a controlled rim light, crisp silhouette, subtle contact shadow, and premium contrast that survives a small mobile thumbnail. Preserve the product shape and color; no text, logo changes, props, people, or watermark.",
  },
  "grok-imagine-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["grok-imagine-image"],
    prompt:
      "For an electronics retailer, create a 16:9 product-detail hero of an open matte-black wireless earbud case on dark slate. Show both earbuds seated correctly, crisp hinge and material texture, a low three-quarter camera, and a single soft key light with a controlled cast shadow. No hands, people, readable text, invented logo, extra accessories, or watermark.",
  },
  "grok-imagine-image-pro": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["grok-imagine-image-pro"],
    prompt:
      "For a consumer-electronics catalog, create a 1:1 front-facing listing image of a closed matte-black wireless earbud charging case on a light concrete surface. Preserve the rounded lid, seam, indicator light, and subtle brand mark exactly; use diffuse studio light and a soft natural shadow with marketplace-safe margins. No invented text, extra products, hands, or watermark.",
  },
  "grok-imagine-image-quality": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["grok-imagine-image-quality"],
    prompt:
      "For a lifestyle retail shop, create a 4:5 hero image of a handmade ceramic travel mug on a sunlit coastal stone ledge. Keep the glaze pattern, handle, and proportions stable; use a softly blurred ocean background, warm morning light, and clear negative space for product title and price. No people, hands, text, invented logo, extra props, or watermark.",
  },
  "nano-banana-pro-preview": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["nano-banana-pro-preview"],
    prompt:
      "For a beauty marketplace variant set, create three coordinated 9:16 product panels for the same pump bottle in magenta, sky blue, and amber colorways. Lock camera height, bottle proportions, cap geometry, lighting direction, and crop; change only the liquid and background color. Keep every panel text-free with no invented logos, extra products, hands, or watermark.",
  },
};

/**
 * Workflow prompts for the image prompt library. Bracketed values are
 * deliberate fill-in slots: a visitor can replace them without rewriting the
 * composition, lighting, and delivery constraints that make a prompt useful.
 */
export const IMAGE_PROMPT_TEMPLATES: readonly ImagePromptTemplate[] = [
  {
    // Keep the legacy id so previously saved prompt-library links continue to resolve.
    id: "product-hero",
    label: "Game UI interaction and equipment switching",
    prompt:
      "For game UI and interaction designers, create a 16:9 static game keyframe for [original game and character] that can be matched across an equipment-switch transition. Show the same character with [weapon or item] equipped, a deliberate HUD/loadout composition, and a clearly separated alternate equipment slot for the next frame. Lock camera, pose, costume silhouette, lighting direction, and prop geometry so Seedance can animate a seamless swap, skill release, or menu interaction. Keep the game art and interface hierarchy legible; add labels in post. No existing IP, real logos, readable words, invented stats, or watermark.",
    ratio: "16:9",
    poster: `${GAME_UI_EQUIPMENT_CDN_BASE}/gpt-image-2.png`,
    tags: ["game", "ui", "equipment", "animation"],
  },
  {
    id: "social-ad",
    label: "Live sports broadcast simulation",
    prompt:
      "For sports broadcasters, live-event directors, and sports marketing teams, create a 16:9 high-fidelity television broadcast frame of [sport and decisive moment] in [venue]. Capture a physically believable instant with broadcast-camera perspective, natural motion blur, realistic collisions or ball trajectory, crowd depth, and mild codec/compression texture. Reserve clean areas for a generic scoreboard, clock, lower-third, and replay markers using abstract shapes and simple numerals only; add final copy in post. Keep uniforms, equipment, lighting, and camera direction consistent so Seedance can extend the frame into a short highlight with camera shake and arena ambience. No real leagues, teams, athletes, sponsors, logos, readable words, invented claims, or watermark.",
    ratio: "16:9",
    poster: `${SPORTS_BROADCAST_CDN_BASE}/gpt-image-2.png`,
    tags: ["sports", "broadcast", "live-event", "television"],
  },
  {
    id: "catalog-variant",
    label: "Brand TVC and seamless ecommerce showcase",
    prompt:
      "For brand, advertising, and ecommerce teams, create a 16:9 commercial product hero for [product] in [set or environment]. Lock the product silhouette, proportions, materials, packaging details, and brand-safe blank areas while using a deliberate starting frame such as [macro detail, three-quarter hero, or 360-degree turn]. Build polished TVC lighting, reflections, and a clean surface that can extend into a seamless product reveal or ecommerce variant. Leave room for headline and CTA copy to be added in post. No real logos, readable words, invented claims, extra products, people, hands, or watermark.",
    ratio: "16:9",
    poster: `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gpt-image-2.png`,
    tags: ["brand", "tvc", "ecommerce", "product"],
  },
  {
    id: "editorial-portrait",
    label: "Cinematic character and storyboard direction",
    prompt:
      "For film directors, storyboard artists, and character designers, create a 16:9 cinematic keyframe of [character] in [scene] from a deliberate [wide, medium, close, or over-the-shoulder] angle. Preserve identity cues, wardrobe, props, screen direction, lighting direction, and spatial relationships so the image can serve as a reference frame for alternate camera views and a continuous shot. Build a clear foreground, midground, and background with expressive but natural action; leave dialogue and captions for post. No existing IP, real people, logos, readable words, invented credits, or watermark.",
    ratio: "16:9",
    poster: `${CINEMATIC_STORYBOARD_CDN_BASE}/gpt-image-2.png`,
    tags: ["film", "character", "storyboard", "cinematic"],
  },
  {
    id: "product-ui",
    label: "Comedy sketch and physical storytelling",
    prompt:
      "For short-drama writers, comedy creators, and physical-storyboard teams, create a 16:9 cinematic freeze-frame of [comic setup] in [location]. Show a clear cause-and-effect chain—[object] moves from [start] to [landing]—with expressive reactions, believable weight, safe spacing, and a camera angle that makes the gag readable. Keep the same characters, props, wardrobe, and screen direction suitable for a short physical-comedy clip; add dialogue or captions in post. No injury, dangerous stunts, real brands, readable words, existing IP, or watermark.",
    ratio: "16:9",
    poster: `${COMEDY_PHYSICAL_CDN_BASE}/gpt-image-2.png`,
    tags: ["comedy", "physical", "micro-drama", "storytelling"],
  },
  {
    id: "food-editorial",
    label: "Historical photo restoration and revival",
    prompt:
      "For archivists, museums, historians, and documentary creators, restore and re-stage a 16:9 historical photo from [year or era] showing [ordinary people and activity] in [place]. Reconstruct period-accurate clothing, architecture, tools, and light; retain natural film grain and a few subtle repaired marks while recovering texture without making people identifiable. Compose a stable frame that can later animate gentle breathing, eye movement, or environmental motion. No real public figures, political symbols, readable signage, modern objects, invented captions, logos, or watermark.",
    ratio: "16:9",
    poster: `${HISTORICAL_REVIVAL_CDN_BASE}/gpt-image-2.png`,
    tags: ["historical", "restoration", "humanities", "documentary"],
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
 * Pick deterministic, profession-compatible posters for one model's template
 * cards. The model id chooses a compatible variant so two model pages do not
 * look like a copy-paste, while the profession-to-scenario relationship stays
 * stable (for example, a physical-comedy brief always gets a physical-comedy image).
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

/**
 * Resolve a model-specific Playground starter, including catalog aliases such
 * as `gemini-3.1-flash-image-preview`. A fresh object keeps callers from
 * accidentally mutating the shared catalog entry.
 */
export function getImagePlaygroundExample(modelId = ""): ImagePlaygroundExample | undefined {
  const normalizedModelId = modelId
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  const exact = IMAGE_PLAYGROUND_EXAMPLES[normalizedModelId];
  if (exact) return { ...exact };

  const alias = Object.keys(IMAGE_PLAYGROUND_EXAMPLES).find(
    (candidate) => normalizedModelId.startsWith(`${candidate}-`) || candidate.startsWith(`${normalizedModelId}-`)
  );
  return alias ? { ...IMAGE_PLAYGROUND_EXAMPLES[alias] } : undefined;
}

export function getImagePromptTemplate(templateId: string): ImagePromptTemplate | undefined {
  return IMAGE_PROMPT_TEMPLATES.find((template) => template.id === templateId);
}
