import type { Locale } from "./locales";

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

type LocalizedImagePromptCopy = {
  labels: Partial<Record<ImagePromptTemplate["id"], string>>;
  prompts: Partial<Record<ImagePromptTemplate["id"], string>>;
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

const IMAGE_PROMPT_LOCAL_FALLBACK_LANES = [
  "game-ui-equipment",
  "sports-broadcast",
  "brand-tvc-ecommerce",
  "cinematic-storyboard",
  "comedy-physical",
  "historical-revival",
] as const;

function normalizeImageModelId(modelId: string): string {
  return modelId
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function resolveImagePosterModelId(modelId: string): string | undefined {
  const normalized = normalizeImageModelId(modelId);
  if (IMAGE_MODEL_POSTER_SETS[normalized]) return normalized;
  return Object.keys(IMAGE_MODEL_POSTER_SETS).find(
    (candidate) => normalized.startsWith(`${candidate}-`) || candidate.startsWith(`${normalized}-`),
  );
}

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
      "Create a vertical illustrated fantasy narrative poster with a large dark-haired profile silhouette on the left, a layered mountain temple city in the center, a red torii gate, drifting cherry blossoms, and small travelers along the bottom edge. Use a cream paper texture with ink-and-wash detail, a restrained charcoal, vermilion, and muted violet palette, and reserve a clean right margin for the title and credits.",
  },
  "gemini-2-5-flash-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-2-5-flash-image"],
    prompt:
      "Create a tall editorial travel-guide infographic for a three-day Chengdu trip. Combine friendly panda illustrations, a city skyline, Anshun Bridge and old-street scenes, local food icons, a day-one/day-two/day-three route layout, small maps, and budget-and-tips blocks. Use a clear green, orange, and cream information hierarchy with compact but legible Chinese annotations and consistent card spacing.",
  },
  "gemini-3-pro-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-3-pro-image"],
    prompt:
      "Create a vertical museum-catalog infographic dissecting a Ming-dynasty hanfu. Place a central woman in a pale robe, exploded garment components and construction views on the left, fabric, weave, and pattern swatches on the right, and a step-by-step dressing sequence along the bottom. Use a restrained cream, navy, muted red, and ink palette with precise Chinese annotation hierarchy and generous archival margins.",
  },
  "gemini-3-1-flash-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-3-1-flash-image"],
    prompt:
      "Create a two-panel luxury skincare serum advertising poster. Show a frosted dropper bottle against warm ivory and champagne-gold liquid, with fine water droplets, translucent molecule diagrams, and carefully aligned areas for benefits, ingredients, and price. Keep the bottle proportions and reflections identical across both panels, use soft cream-and-gold lighting, and leave the typography zones clean for editorial copy.",
  },
  "gemini-3-1-flash-lite-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["gemini-3-1-flash-lite-image"],
    prompt:
      "Create a 10-by-10 numbered storyboard contact sheet containing 100 panels from a period Chinese drama. Show recurring male and female characters in historical robes across courtyards, interiors, markets, meals, letters, costume changes, and night scenes. Keep faces, wardrobe continuity, props, and screen direction consistent from panel to panel; use warm lantern light, varied shot sizes, and small readable panel numbers.",
  },
  "grok-imagine-image": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["grok-imagine-image"],
    prompt:
      "Create a 16:9 fictional open-world game livestream screenshot on a bright palm-lined coastal street. Show a pink sports car in the roadway, a third-person player avatar, a streamer facecam in the lower-left corner, a vertical chat column on the right, and a clear HUD with minimap and status panels. Use a lively neon-sunset palette, crisp game-rendered depth, and distinct overlay zones for the broadcast interface.",
  },
  "grok-imagine-image-pro": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["grok-imagine-image-pro"],
    prompt:
      "Create a vertical photoreal pet-food brand hero with a happy golden retriever sitting in a warm, sunlit kitchen. Place a ceramic bowl of kibble in the foreground and a kraft pet-food bag beside the dog, with a blank front panel reserved for later branding. Use soft window light, natural wood and cream tones, a shallow depth of field, and a friendly eye-level composition.",
  },
  "grok-imagine-image-quality": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["grok-imagine-image-quality"],
    prompt:
      "Create a vertical fantasy-and-science-fiction book cover set inside a vaulted library. Fill the scene with tall bookshelves, floating pages, a glowing cyan circuit-like path leading through luminous trees and flowers, and layered violet-blue light. Keep a strong central perspective, rich paper-and-ink texture, and a clean upper area for the title and author name.",
  },
  "nano-banana-pro-preview": {
    industry: "ecommerce-retail",
    poster: IMAGE_PLAYGROUND_SELECTED_POSTERS["nano-banana-pro-preview"],
    prompt:
      "Create a vertical high-end interior render with a white boucle sofa in the foreground, walnut cabinetry and a flush door, a sculptural wavy floor lamp, and a potted olive tree. Use floor-to-ceiling windows to cast strong warm afternoon sunlight and long shadows across the room, with balanced architectural perspective, quiet neutral materials, and a clean uncluttered composition.",
  },
};

/**
 * Canonical English prompts for the reviewed image references.  Each prompt
 * describes the exact scene shown by the corresponding CDN poster; it is
 * intentionally ready to run and contains no profession brief or fill-in
 * placeholder.
 */
export const IMAGE_PROMPT_TEMPLATES: readonly ImagePromptTemplate[] = [
  {
    // Keep the legacy id so previously saved prompt-library links continue to resolve.
    id: "product-hero",
    label: "Game UI interaction and equipment switching",
    prompt:
      "A 16:9 sci-fi game loadout screen in a blue-purple spaceship hangar. A female armored character stands centered with a glowing violet rifle; armor slots line the left, three weapon slots line the right, and the lower slot is highlighted. Keep the HUD hierarchy, pose, lighting, and equipment geometry fixed for an exact equipment-switch frame. No readable words, logos, invented stats, or watermark.",
    ratio: "16:9",
    poster: `${GAME_UI_EQUIPMENT_CDN_BASE}/gpt-image-2.png`,
    tags: ["game", "ui", "equipment", "animation"],
  },
  {
    id: "social-ad",
    label: "Live sports broadcast simulation",
    prompt:
      "A rainy night soccer broadcast frame under bright stadium floodlights. A black-uniform player strikes the ball while two white-uniform defenders close in, with the goal and a blurred crowd behind. Keep the broadcast camera angle, blank blue score bars, spray, and motion blur consistent; leave overlays abstract and text-free. No real teams, athletes, logos, or watermark.",
    ratio: "16:9",
    poster: `${SPORTS_BROADCAST_CDN_BASE}/gpt-image-2.png`,
    tags: ["sports", "broadcast", "live-event", "television"],
  },
  {
    id: "catalog-variant",
    label: "Brand TVC and seamless ecommerce showcase",
    prompt:
      "A closed matte-black square smartwatch with a black strap rests on a wet glossy black tabletop. Water droplets catch a cool blue rim light and a soft reflection sits beneath the watch. Use a low three-quarter product camera and preserve the case, strap, highlights, and empty dark background. No readable branding, extra products, hands, or watermark.",
    ratio: "16:9",
    poster: `${BRAND_TVC_ECOMMERCE_CDN_BASE}/gpt-image-2.png`,
    tags: ["brand", "tvc", "ecommerce", "product"],
  },
  {
    id: "editorial-portrait",
    label: "Cinematic character and storyboard direction",
    prompt:
      "On a rain-soaked observatory roof at sunrise, a lone figure in a long dark coat walks toward a large telescope beside an open dome. Wet stone reflects the warm doorway light; cloud-covered mountains sit beyond. Hold the screen direction and end on the telescope and figure in the same wide composition. No readable text, logos, or watermark.",
    ratio: "16:9",
    poster: `${CINEMATIC_STORYBOARD_CDN_BASE}/gpt-image-2.png`,
    tags: ["film", "character", "storyboard", "cinematic"],
  },
  {
    id: "product-ui",
    label: "Comedy sketch and physical storytelling",
    prompt:
      "In a bright home kitchen, a surprised cook in a blue shirt and cream apron reaches toward pancakes, a frying pan, flour, bowl, and whisk suspended midair. Freeze the comic cause-and-effect moment with believable weight, warm daylight, and a clear path for each object. No injury, logos, readable words, or watermark.",
    ratio: "16:9",
    poster: `${COMEDY_PHYSICAL_CDN_BASE}/gpt-image-2.png`,
    tags: ["comedy", "physical", "micro-drama", "storytelling"],
  },
  {
    id: "food-editorial",
    label: "Historical photo restoration and revival",
    prompt:
      "A rainy period harbor platform shelters people in era-appropriate coats and umbrellas as a vintage green tram arrives on wet tracks. Wooden waterfront buildings and misty mountains sit behind the reflections. Preserve the historical clothing, tram shape, rain direction, and stable wide composition; no readable signage, logos, or watermark.",
    ratio: "16:9",
    poster: `${HISTORICAL_REVIVAL_CDN_BASE}/gpt-image-2.png`,
    tags: ["historical", "restoration", "humanities", "documentary"],
  },
];

type ImagePromptSceneId =
  | "product-hero"
  | "social-ad"
  | "catalog-variant"
  | "editorial-portrait"
  | "product-ui"
  | "food-editorial";

type ImagePromptSet = Record<ImagePromptSceneId, string>;

const IMAGE_PROMPT_BASE_BY_ID: ImagePromptSet = {
  "product-hero": IMAGE_PROMPT_TEMPLATES[0].prompt,
  "social-ad": IMAGE_PROMPT_TEMPLATES[1].prompt,
  "catalog-variant": IMAGE_PROMPT_TEMPLATES[2].prompt,
  "editorial-portrait": IMAGE_PROMPT_TEMPLATES[3].prompt,
  "product-ui": IMAGE_PROMPT_TEMPLATES[4].prompt,
  "food-editorial": IMAGE_PROMPT_TEMPLATES[5].prompt,
};

/**
 * Fact-based prompts for each reviewed model poster. The six keys intentionally
 * match the six poster lanes above; keeping this as one model-keyed registry
 * prevents a model page from pairing a real output with a generic prompt from
 * another model. Translated routes use the locale-specific prompt bodies.
 */
const IMAGE_MODEL_ENGLISH_PROMPTS: Record<string, ImagePromptSet> = {
  "gpt-image-2": IMAGE_PROMPT_BASE_BY_ID,
  "gemini-2-5-flash-image": {
    "product-hero":
      "A 16:9 fantasy game loadout screen in mossy forest ruins. A hooded green archer stands centered with a bow; a circular equipment wheel sits around the character and alternate equipment silhouettes occupy the right panel. Keep the green-and-gold HUD, pose, ruins, and prop geometry fixed for an exact equipment-switch frame. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "A live basketball broadcast frame catches a blue-uniform player airborne for a dunk in a packed arena. Show the hoop, backboard, court lights, crowd depth, and an abstract blue-and-gold lower-third without readable text. Keep the camera angle, ball position, jersey colors, and contact-free action consistent; no real teams, athletes, logos, or watermark.",
    "catalog-variant":
      "A gray-and-black running shoe hangs at a three-quarter angle above a wet track. Its orange sole glows against a dark background while orange light trails echo the tread and the surface holds a restrained reflection. Preserve the shoe shape, lace pattern, sole, lighting, and empty copy space. No readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "A lone astronaut stands inside a glass biodome on a red desert planet, seen from behind among glowing bioluminescent plants. A hazy red landscape and distant mountains show through the curved glass. Hold the character silhouette, plant positions, cool interior light, and wide screen direction for storyboard continuity. No readable text, logos, or watermark.",
    "product-ui":
      "In a bright modern kitchen, a surprised man in a light green overshirt holds an orange while paper sheets and cups burst through the air around him. Freeze the harmless cause-and-effect gag with believable trajectories, warm daylight, and clear spacing between the objects. Keep the pose and prop positions stable for a physical-comedy frame; no injury, logos, readable words, or watermark.",
    "food-editorial":
      "A restored black-and-white market street shows period vendors beside baskets of fruit and folded fabric, with stone buildings and mountains behind them. Preserve the era-appropriate clothing, stalls, architecture, film grain, and stable documentary composition while repairing scratches and contrast. Do not add modern objects, readable signs, logos, identifiable public figures, or watermark.",
  },
  "gemini-3-pro-image": {
    "product-hero":
      "A 16:9 underwater game loadout screen on a sea floor. An armored diver stands on a circular pedestal holding a glowing cyan sword, with submerged ruins behind and a weapon-swap panel on the right. Lock the diver silhouette, sword, aquatic lighting, pedestal, and interface hierarchy for an exact equipment-switch frame. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "A televised basketball frame shows a red-uniform player taking a jump shot while a white-uniform defender contests it in an indoor arena. Include the hoop, overhead lights, crowd blur, and abstract red-and-gray lower-third panels without readable text. Keep the broadcast angle, ball arc, uniforms, and player spacing coherent; no real teams, athletes, logos, or watermark.",
    "catalog-variant":
      "A clear glass dropper bottle filled with pale gold serum stands on a white stone block in a warm cream room. Soft sunlight and large leaf shadows fall across the wall and the liquid, glass, metal collar, and surface catch subtle reflections. Preserve the bottle proportions and generous blank space; no readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "At a rain-wet historic train platform, a young traveler in a dark cap and coat holds a brown suitcase while a steam train waits under a long canopy. Warm lamps recede into the blue night and reflect on the platform. Hold the traveler profile, suitcase, train direction, and wide composition for a continuous storyboard. No readable signs, logos, or watermark.",
    "product-ui":
      "On a small theater stage, a startled magician in black pulls a long rainbow cloth from a tall black trunk wrapped in more cloth. The audience is dim behind him, with red curtains and colored spotlights. Freeze the harmless reveal at the moment the fabric stretches between hand and trunk; preserve object weight, pose, and stage spacing. No readable words, logos, injury, or watermark.",
    "food-editorial":
      "A sepia archival photograph shows four workers weaving cloth on wooden looms inside a rustic workshop, with yarn baskets and bright hillside scenery through the windows. Restore the paper texture, faces without sharpening them into modern portraits, loom construction, period clothing, and gentle film wear. No modern objects, readable signage, logos, captions, or watermark.",
  },
  "gemini-3-1-flash-image": {
    "product-hero":
      "A 16:9 futuristic game loadout frame shows a helmeted rider beside a black-and-orange motorcycle in a rainy industrial hangar. Blue-and-orange interface controls and a circular part-selection panel sit around the vehicle. Keep the rider, motorcycle geometry, wet floor reflections, lighting direction, and selected-part position fixed for a clean equipment swap. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "A poolside sports broadcast catches a swimmer in a blue cap during freestyle, with one arm entering the water and a sharp splash across the lane. Use a low pool-deck camera, blue water, lane markings, and abstract blue lower-third graphics without readable text. Keep swimmer direction, splash shape, and framing consistent; no real athletes, teams, logos, or watermark.",
    "catalog-variant":
      "A metallic wireless-earbud charging case floats open above a glossy black surface while the two matching earbuds hover above their slots. Purple and cyan rim lights define the rounded metal, black interior, reflections, and empty dark background. Preserve the case, earbuds, hinge, highlights, and three-quarter camera angle. No readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "A cloaked figure crouches on a rain-slick elevated walkway beneath massive futuristic bridges. Cyan fog, magenta tower lights, railings, and wet reflections lead toward the distant city. Hold the figure's silhouette, facing direction, cloak edge, camera height, and neon light placement as a cinematic storyboard keyframe. No readable signs, logos, or watermark.",
    "product-ui":
      "In a wet green park, a man in a coral-red raincoat and yellow boots struggles with an inverted rainbow umbrella in a gust of wind. His arms brace against the bent canopy while rain streaks through the trees and puddles reflect the bright colors. Keep the umbrella shape, body pose, wind direction, and safe footing clear; no injury, readable text, logos, or watermark.",
    "food-editorial":
      "A softly faded historical dock scene shows several workers repairing a large wooden boat at a riverside boatyard. Timber ribs, scaffolding, hand tools, smaller boats, and weathered work clothes fill the wide documentary frame. Restore the wood grain, riverbank structures, faces, and natural color aging without modern objects, readable signs, logos, captions, or watermark.",
  },
  "gemini-3-1-flash-lite-image": {
    "product-hero":
      "A 16:9 whimsical game UI shows a small white robot standing in a floating-island garden while holding a green plant blaster or seed tool. A circular equipment wheel surrounds the selected tool, with tiny garden plots, paths, clouds, and floating terrain behind. Lock the robot pose, tool shape, wheel position, and bright pastel lighting for an equipment-switch frame. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "An aerial broadcast-style road-race frame follows a cyclist and a motorcycle on a winding coastal road above blue water and steep cliffs. Leave an abstract lower-third area free of readable text and keep the road curve, vehicle spacing, sea horizon, and sun direction consistent. No real teams, athletes, logos, or watermark.",
    "catalog-variant":
      "A tall matte black bottle stands on a sunlit mountain rock with a hazy valley and layered peaks behind it. Use a warm sunrise rim on the right and soft atmospheric depth while preserving the plain cap, bottle silhouette, rock texture, and empty sky for later copy. No readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "At sunset on a sea cliff, a lone helmeted traveler stands in silhouette holding a helmet while a red warning flag fills the near right foreground. The ocean, distant headland, lighthouse, and orange-to-blue sky form a wide establishing frame. Keep the traveler orientation, flag motion direction, horizon, and light transition stable for storyboard use. No readable signs, logos, or watermark.",
    "product-ui":
      "In a mint laundry room, a surprised man in a coral sweatshirt leans back as a turquoise laundry basket throws a colorful cloud of striped socks into the air. Washing machines, folded towels, and clean daylight establish the setting. Preserve the basket tilt, sock trajectories, body pose, and safe spacing for a physical-comedy frame; no injury, readable text, logos, or watermark.",
    "food-editorial":
      "A lightly colorized vintage schoolyard photograph shows children in period sweaters and skirts playing jump rope in front of a brick building. Restore the worn border, faded colors, rope arc, clothing details, and original wide framing without introducing modern objects. No readable signs, logos, captions, or watermark.",
  },
  "grok-imagine-image": {
    "product-hero":
      "A 16:9 cyberpunk game equipment screen shows a masked figure crouched on a neon rooftop above a dense night city. A black-and-lime HUD frames the character, with weapon and tool slots on the right including a glowing pickaxe. Hold the crouch, rooftop line, neon rim light, and selected slot fixed for an equipment-switch frame. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "A sunset beach-volleyball broadcast frame captures a player spiking over the net as sand sprays from the jump. Show the opposing court, hazy beach crowd, warm sky, and a simple teal-and-white lower-third made from abstract blocks. Keep the net, player position, ball path, and camera angle coherent; no real athletes, teams, logos, or watermark.",
    "catalog-variant":
      "A vintage black camera with a large textured lens rests on a black pedestal against a deep green-black background. Warm highlights describe the metal dials, leather grip, lens rings, and small empty space to the left. Preserve the camera body, lens perspective, pedestal edge, and controlled studio lighting. No readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "In a warmly lit theater dressing room, a curly-haired actor in a cream shirt and dark vest looks into a large aged mirror beside costumes and glowing round bulbs. The reflected face and three-quarter profile form a character continuity frame. Keep the mirror geometry, wardrobe, eye line, and amber lighting consistent. No readable signage, logos, or watermark.",
    "product-ui":
      "On a beach at sunset, a man in a light shirt pulls a yellow-striped picnic cloth as a sandwich and two white plates lift into the air above a wooden table. Freeze the harmless physical gag with visible cloth tension, believable object paths, warm sea light, and clear spacing. No injury, readable words, logos, or watermark.",
    "food-editorial":
      "A warm faded archival photograph shows a family having a picnic in a grassy field: one adult pours from a thermos, another spreads food, and a child sits beside a bicycle. Preserve the period clothing, enamel cups, bicycle, hills, film grain, and soft light-leak edge. No modern branding, readable text, logos, or watermark.",
  },
  "grok-imagine-image-pro": {
    "product-hero":
      "A 16:9 desert game equipment screen shows a large rusted mech in a sunlit hangar. A physical hammer stands beside it while a holographic drill-arm replacement and blueprint equipment panel occupy the workbench and right side. Lock the mech silhouette, replacement part positions, warm dust light, and panel hierarchy for a clear equipment swap. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "A motorsport broadcast frame shows a silver open-wheel race car sweeping around a coastal circuit, with pale mountains and ocean beyond. Leave a clean blank scoreboard area at the left and use realistic track reflections and speed perspective. Keep the car proportions, camera height, road curve, and horizon stable; no real teams, drivers, logos, or watermark.",
    "catalog-variant":
      "A clear rectangular perfume bottle with a black cap stands on rippled black satin. A warm amber key light outlines the glass edges and liquid while the background falls into deep shadow. Preserve the square bottle, cap, satin folds, reflections, and generous dark copy space. No readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "Under a canvas camp in a desert ruin, an explorer studies a large hand-drawn map with a brass compass while carved stone structures sit beyond the table. Use an over-shoulder composition with tools, crates, sand, and hard afternoon light. Hold the explorer's hands, map orientation, ruins, and camera angle for a storyboard keyframe. No readable labels, logos, or watermark.",
    "product-ui":
      "In a warm apartment, a startled man in a blue shirt tries to balance a tall stack of taped cardboard boxes while more boxes sit open around him. Freeze the harmless tipping moment with clear weight, contact points, soft window light, and a stable wide camera. No injury, readable shipping text, logos, or watermark.",
    "food-editorial":
      "A monochrome archival railway photograph shows travelers in long coats and hats waiting with suitcases on a rain-dark platform as a steam train approaches. Preserve the canopy, wet pavement, smoke, period clothing, film scratches, and deep perspective while restoring contrast. No readable station signs, logos, modern objects, or watermark.",
  },
  "grok-imagine-image-quality": {
    "product-hero":
      "A 16:9 fantasy game UI shows a cloaked wizard standing on a glowing circular platform before a large crystal equipment wheel. Faceted blue crystals ring the interface around a bright pink crystal at the center, with violet mist and stone architecture behind. Lock the wizard silhouette, wheel geometry, selected crystal, and lighting for an equipment-switch frame. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "An indoor ice-hockey broadcast frame catches an orange-uniform skater driving toward the goal while a goalie guards the blue crease. Show the rink boards, arena haze, ice reflections, and a blank blue panel without readable text. Keep puck direction, player spacing, camera angle, and goal geometry coherent; no real teams, athletes, logos, or watermark.",
    "catalog-variant":
      "A black game controller floats above a dark angular pedestal in a blue-and-violet studio. Soft rim lights define its grips, directional pad, sticks, and buttons while the background remains free for copy. Preserve the controller geometry, centered perspective, highlights, and pedestal edges. No readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "Inside a glass mountain cable-car station, an older man in a dark coat reaches toward a woman as they meet on an icy platform. Snowy peaks, cables, and gondolas fill the background through the windows. Hold both figures' eye lines, walking direction, cold daylight, and wide spatial blocking for a cinematic storyboard. No readable signs, logos, or watermark.",
    "product-ui":
      "In a bright tiled bathroom, a wet shaggy dog bounds forward as a mint towel, soap bubbles, and water droplets fly around it; a surprised groomer in an apron reacts behind. Freeze the safe splash moment with believable arcs, clean separation of dog and person, and soft daylight. No injury, readable text, logos, or watermark.",
    "food-editorial":
      "A sepia archival scene shows a camel caravan resting beside a small oasis, with travelers in wrapped period clothing, baskets, palms, and rocky desert hills. Restore the old paper grain, camel tack, clothing folds, water edge, and stable documentary framing. Do not add modern objects, readable signs, logos, captions, or watermark.",
  },
  "nano-banana-pro-preview": {
    "product-hero":
      "A 16:9 post-apocalyptic game equipment screen shows a rugged survivor with a backpack and rifle inside an overgrown greenhouse. A gear board displays a rifle, axe, and item slots with one orange slot highlighted. Lock the survivor silhouette, weapon geometry, greenhouse light, and board hierarchy for a precise equipment-switch frame. No readable words, logos, invented stats, or watermark.",
    "social-ad":
      "An indoor volleyball broadcast frame shows a blue-uniform attacker rising for a spike against two yellow blockers. Include the net, court lights, crowd blur, a simple top-left scoreboard block, and a lower stats bar without relying on readable claims. Keep the ball path, player spacing, and camera angle coherent; no real teams, athletes, logos, or watermark.",
    "catalog-variant":
      "A black-and-silver espresso machine pours a thin stream of coffee into a cream cup on a pale kitchen counter. Warm window light, a small plant, the portafilter, steam wand, and clean backsplash form a calm product frame. Preserve the machine proportions, cup position, coffee stream, and empty counter space. No readable branding, extra products, hands, or watermark.",
    "editorial-portrait":
      "Inside a wooden ship at night, a carved sailor puppet grips the wheel beside a round porthole showing moonlit waves. Rope coils, timber walls, a lantern, and the blue sea create a tactile character scene. Keep the puppet's face, hands, wheel, porthole, and warm-to-cool lighting fixed for storyboard continuity. No readable text, logos, or watermark.",
    "product-ui":
      "On a warmly lit film set, a bespectacled cameraman in suspenders stumbles beside a vintage camera and tripod while a cream backdrop tangles around his legs. Freeze the harmless physical gag with visible fabric tension, stable tripod contact, and clear studio lighting. No injury, readable words, logos, or watermark.",
    "food-editorial":
      "A monochrome archival dressing-room photograph shows period performers changing clothes beside trunks, ropes, fabric, and a large window opening onto countryside. Restore the film grain, era-appropriate garments, wooden stage equipment, and natural light without adding modern objects. No readable labels, logos, captions, or watermark.",
  },
};

/**
 * Prompt-card labels and bodies are localized for translated model-detail
 * routes. English retains the reviewed model-specific scene briefs.
 */
const IMAGE_PROMPT_LOCALE_COPY: Partial<Record<Locale, LocalizedImagePromptCopy>> = {
  zh: {
    labels: {
      "product-hero": "游戏 UI 交互与装备动态切换",
      "social-ad": "高逼真电视/体育赛事直播模拟",
      "catalog-variant": "品牌商业 TVC 与电商产品无缝展示",
      "editorial-portrait": "影视级剧本角色与多视角分镜演播",
      "product-ui": "喜剧段子与物理剧情演播",
      "food-editorial": "老旧照片修复与历史/人文动态复活",
    },
    prompts: {
      "product-hero": "面向游戏 UI 与交互设计师，为[原游戏与角色]制作 16:9 静态游戏关键帧，用于装备切换转场。展示同一角色装备[武器或道具]，安排清晰的 HUD/装备栏，并为下一帧留出独立的替换装备槽。锁定镜头、姿势、服装轮廓、光线方向和道具几何，让 Seedance 能顺滑完成换装、技能释放或菜单交互。保持游戏美术与界面层级清晰，标签后期添加；不要现有 IP、真实 logo、可读文字、虚构属性或水印。",
      "social-ad": "面向体育转播、现场导演和体育营销团队，在[场馆]为[运动项目与关键瞬间]制作 16:9 高逼真电视转播画面。使用转播机位、自然运动模糊、可信碰撞或球路、观众纵深和轻微编码压缩质感，捕捉真实物理瞬间。为通用比分板、时间、字幕条和回放标记留出干净区域，只用抽象形状与简单数字，最终文案后期添加。保持球衣、器材、灯光和镜头方向一致，便于 Seedance 延展为带镜头抖动和场馆氛围的短集锦；不要真实联赛、球队、运动员、赞助商、logo、可读文字、虚构声明或水印。",
      "catalog-variant": "面向品牌、广告和电商团队，在[场景或环境]中为[产品]制作 16:9 商业产品主视觉。锁定产品轮廓、比例、材质、包装细节及品牌安全留白，并从[微距细节、三分之四主视图或 360 度旋转]等明确起始帧开始。使用精致 TVC 灯光、反射和干净台面，让画面可无缝延展为产品揭示或电商变体。为标题和 CTA 留出空间，文案后期添加；不要真实 logo、可读文字、虚构功效、额外产品、人物、手或水印。",
      "editorial-portrait": "面向电影导演、分镜师和角色设计师，为[角色]在[场景]中制作 16:9 电影感关键帧，采用明确的[远景、中景、近景或过肩]角度。保持身份特征、服装、道具、屏幕方向、光线方向和空间关系一致，使画面可作为多机位与连续镜头的参考。安排清晰的前景、中景和背景，动作自然有表现力；对白和字幕后期添加。不要现有 IP、真实人物、logo、可读文字、虚构演职员名单或水印。",
      "product-ui": "面向短剧编剧、喜剧创作者和物理分镜团队，在[地点]把[喜剧设定]制作成 16:9 电影感定格画面。明确展示因果链——[物体]从[起点]移动到[落点]——配合夸张但自然的反应、可信重量、安全间距和易读笑点的机位。保持角色、道具、服装和屏幕方向一致，便于制作物理喜剧短片；对白或字幕后期添加。不要伤害、危险特技、真实品牌、可读文字、现有 IP 或水印。",
      "food-editorial": "面向档案馆、博物馆、历史学者和纪录片创作者，修复并重构一张来自[年份或时代]、记录[普通人物与活动]于[地点]的 16:9 历史照片。还原符合时代的服装、建筑、工具与光线，保留自然胶片颗粒和少量修复痕迹，同时恢复纹理但不让人物可识别。构图保持稳定，方便后续添加轻微呼吸、眼神或环境运动。不要真实公众人物、政治符号、可读招牌、现代物件、虚构字幕、logo 或水印。",
    },
  },
  es: {
    labels: {
      "product-hero": "Interacción de UI de juego y cambio de equipamiento",
      "social-ad": "Simulación de retransmisión deportiva en TV",
      "catalog-variant": "TVC de marca y escaparate de ecommerce",
      "editorial-portrait": "Personaje cinematográfico y dirección de storyboard",
      "product-ui": "Gag cómico y narración física",
      "food-editorial": "Restauración de fotos históricas y memoria viva",
    },
    prompts: {
      "product-hero": "Para diseñadores de UI e interacción de juegos, crea un fotograma clave estático 16:9 de [juego y personaje originales] para una transición de cambio de equipo. Muestra al mismo personaje con [arma u objeto] equipado, una composición HUD/loadout clara y una ranura alternativa separada para el siguiente fotograma. Bloquea cámara, pose, silueta del vestuario, dirección de luz y geometría para que Seedance anime un cambio, habilidad o menú fluido. Mantén legibles el arte y la jerarquía de interfaz; añade etiquetas en postproducción. Sin IP existente, logos reales, texto legible, estadísticas inventadas ni marca de agua.",
      "social-ad": "Para equipos de retransmisión y marketing deportivo, crea un fotograma televisivo 16:9 de alta fidelidad de [deporte y momento decisivo] en [estadio]. Captura un instante físicamente creíble con perspectiva de cámara de emisión, desenfoque natural, colisiones o trayectoria de balón realistas, profundidad de público y ligera compresión de códec. Reserva zonas limpias para marcador, reloj, rótulo y repetición genéricos usando formas abstractas y números simples; añade el texto final en post. Mantén uniformes, equipo, luz y dirección de cámara para que Seedance extienda el cuadro con vibración y ambiente del estadio. Sin ligas, equipos, atletas, patrocinadores, logos, texto legible, afirmaciones inventadas ni marca de agua.",
      "catalog-variant": "Para equipos de marca, publicidad y ecommerce, crea un hero comercial 16:9 de [producto] en [escenario]. Fija silueta, proporciones, materiales, detalles del envase y áreas vacías seguras para la marca; parte de [detalle macro, hero en tres cuartos o giro de 360 grados]. Usa luz TVC pulida, reflejos y una superficie limpia para prolongar la imagen como revelado de producto o variante de tienda. Deja espacio para titular y CTA, que se añadirán después. Sin logos reales, palabras legibles, promesas inventadas, productos extra, personas, manos ni marca de agua.",
      "editorial-portrait": "Para directores, artistas de storyboard y diseñadores de personajes, crea un fotograma cinematográfico 16:9 de [personaje] en [escena] desde un ángulo [abierto, medio, cercano o sobre el hombro]. Conserva identidad, vestuario, utilería, dirección de pantalla, luz y relaciones espaciales para usarlo como referencia de otros ángulos y continuidad. Construye primer plano, plano medio y fondo claros con acción expresiva y natural; deja diálogos y subtítulos para post. Sin IP existente, personas reales, logos, texto legible, créditos inventados ni marca de agua.",
      "product-ui": "Para guionistas de microdramas, creadores de comedia y equipos de storyboard físico, crea un fotograma congelado cinematográfico 16:9 de [situación cómica] en [lugar]. Muestra la cadena causa-efecto —[objeto] va de [inicio] a [aterrizaje]— con reacciones expresivas, peso creíble, espacio seguro y un ángulo que haga legible el gag. Mantén personajes, utilería, vestuario y dirección de pantalla para un clip de comedia física; añade diálogo o subtítulos en post. Sin lesiones, acrobacias peligrosas, marcas reales, texto legible, IP existente ni marca de agua.",
      "food-editorial": "Para archivistas, museos, historiadores y documentalistas, restaura y recrea una foto histórica 16:9 de [año o época] con [personas comunes y actividad] en [lugar]. Reconstruye ropa, arquitectura, herramientas y luz de época; conserva grano de película y algunas marcas reparadas mientras recuperas textura sin identificar a las personas. Compón un cuadro estable para animar después respiración, mirada o ambiente sutil. Sin figuras públicas reales, símbolos políticos, letreros legibles, objetos modernos, pies inventados, logos ni marca de agua.",
    },
  },
  fr: {
    labels: {
      "product-hero": "Interaction UI de jeu et changement d’équipement",
      "social-ad": "Simulation de retransmission sportive télévisée",
      "catalog-variant": "TVC de marque et vitrine e-commerce",
      "editorial-portrait": "Personnage cinéma et direction de storyboard",
      "product-ui": "Gag comique et narration physique",
      "food-editorial": "Restauration photo historique et résurrection documentaire",
    },
    prompts: {
      "product-hero": "Pour les designers d’UI et d’interaction de jeu, créez une image clé statique 16:9 de [jeu et personnage originaux] pour une transition de changement d’équipement. Montrez le même personnage avec [arme ou objet] équipé, une composition HUD/équipement claire et un emplacement alternatif séparé pour l’image suivante. Verrouillez caméra, pose, silhouette du costume, direction de la lumière et géométrie afin que Seedance anime un échange, une compétence ou un menu fluide. Gardez l’art et la hiérarchie de l’interface lisibles; ajoutez les libellés en postproduction. Sans IP existante, logos réels, texte lisible, statistiques inventées ni filigrane.",
      "social-ad": "Pour les équipes de diffusion et de marketing sportif, créez une image TV 16:9 très réaliste de [sport et moment décisif] dans [stade]. Saisissez un instant physiquement crédible avec perspective de caméra de direct, flou naturel, collisions ou trajectoire de balle réalistes, profondeur du public et légère compression codec. Réservez des zones propres pour un score, une horloge, un bandeau et des marqueurs de replay génériques, avec formes abstraites et chiffres simples; ajoutez le texte final en postproduction. Conservez tenues, équipement, lumière et axe caméra pour que Seedance prolonge l’image avec secousses et ambiance d’arène. Sans ligues, équipes, athlètes, sponsors, logos, texte lisible, affirmations inventées ni filigrane.",
      "catalog-variant": "Pour les équipes de marque, publicité et e-commerce, créez un visuel produit commercial 16:9 de [produit] dans [décor]. Verrouillez silhouette, proportions, matières, détails du packaging et espaces neutres; partez d’un [détail macro, hero trois-quarts ou tour à 360 degrés]. Utilisez une lumière TVC soignée, des reflets maîtrisés et une surface nette pour prolonger l’image en révélation produit ou variante e-commerce. Laissez de la place au titre et au CTA ajoutés en postproduction. Sans vrais logos, mots lisibles, promesses inventées, produits supplémentaires, personnes, mains ni filigrane.",
      "editorial-portrait": "Pour les réalisateurs, artistes storyboard et designers de personnages, créez une image clé cinématographique 16:9 de [personnage] dans [scène], depuis un angle [large, moyen, rapproché ou par-dessus l’épaule]. Préservez identité, costume, accessoires, axe écran, direction de lumière et relations spatiales afin de servir de référence à d’autres angles et à un plan continu. Construisez premier plan, plan intermédiaire et arrière-plan lisibles avec une action naturelle; gardez dialogues et sous-titres pour la postproduction. Sans IP existante, personnes réelles, logos, texte lisible, générique inventé ni filigrane.",
      "product-ui": "Pour les auteurs de micro-dramas, créateurs de comédie et équipes de storyboard physique, créez une image figée cinématographique 16:9 de [situation comique] dans [lieu]. Montrez la chaîne cause-effet —[objet] va de [départ] à [atterrissage]— avec réactions expressives, poids crédible, espace sûr et angle lisible. Gardez personnages, accessoires, costumes et axe écran cohérents pour un clip de comédie physique; ajoutez dialogues ou sous-titres en postproduction. Sans blessure, cascade dangereuse, marque réelle, texte lisible, IP existante ni filigrane.",
      "food-editorial": "Pour les archivistes, musées, historiens et documentaristes, restaurez et recomposez une photo historique 16:9 de [année ou époque] montrant [personnes ordinaires et activité] à [lieu]. Reconstituez vêtements, architecture, outils et lumière d’époque; conservez le grain argentique et quelques traces réparées tout en récupérant les textures sans rendre les personnes identifiables. Composez un cadre stable pour animer ensuite respiration, regard ou mouvement ambiant subtil. Sans personnalités publiques réelles, symboles politiques, enseignes lisibles, objets modernes, légendes inventées, logos ni filigrane.",
    },
  },
  pt: {
    labels: {
      "product-hero": "Interação de UI de jogo e troca de equipamento",
      "social-ad": "Simulação de transmissão esportiva em TV",
      "catalog-variant": "TVC de marca e vitrine de e-commerce",
      "editorial-portrait": "Personagem cinematográfico e direção de storyboard",
      "product-ui": "Esquete cômica e narrativa física",
      "food-editorial": "Restauração de foto histórica e memória documental",
    },
    prompts: {
      "product-hero": "Para designers de UI e interação de jogos, crie um quadro-chave estático 16:9 de [jogo e personagem originais] para uma transição de troca de equipamento. Mostre o mesmo personagem com [arma ou item] equipado, uma composição clara de HUD/loadout e um slot alternativo separado para o próximo quadro. Trave câmera, pose, silhueta do figurino, direção da luz e geometria para que o Seedance anime uma troca, habilidade ou menu contínuo. Mantenha a arte e a hierarquia da interface legíveis; adicione rótulos na pós-produção. Sem IP existente, logos reais, texto legível, estatísticas inventadas ou marca-d’água.",
      "social-ad": "Para equipes de transmissão e marketing esportivo, crie um quadro televisivo 16:9 de alta fidelidade de [esporte e momento decisivo] em [estádio]. Capture um instante fisicamente crível com perspectiva de câmera de transmissão, desfoque natural, colisões ou trajetória de bola realistas, profundidade da torcida e leve textura de compressão. Reserve áreas limpas para placar, relógio, tarja e marcadores de replay genéricos usando formas abstratas e números simples; adicione o texto final depois. Mantenha uniformes, equipamento, iluminação e direção de câmera para o Seedance estender o quadro com trepidação e ambiência da arena. Sem ligas, equipes, atletas, patrocinadores, logos, texto legível, alegações inventadas ou marca-d’água.",
      "catalog-variant": "Para equipes de marca, publicidade e e-commerce, crie um hero de produto comercial 16:9 de [produto] em [cenário]. Trave silhueta, proporções, materiais, detalhes da embalagem e áreas neutras seguras; comece por [detalhe macro, hero em três quartos ou giro de 360 graus]. Use luz de TVC refinada, reflexos controlados e superfície limpa para prolongar a imagem em revelação de produto ou variante de loja. Deixe espaço para título e CTA adicionados na pós-produção. Sem logos reais, palavras legíveis, promessas inventadas, produtos extras, pessoas, mãos ou marca-d’água.",
      "editorial-portrait": "Para diretores, artistas de storyboard e designers de personagens, crie um quadro-chave cinematográfico 16:9 de [personagem] em [cena], visto de um ângulo [aberto, médio, próximo ou sobre o ombro]. Preserve identidade, figurino, objetos, direção de tela, direção da luz e relações espaciais para servir de referência a outros ângulos e a um plano contínuo. Construa primeiro plano, meio e fundo claros com ação natural; deixe diálogos e legendas para a pós-produção. Sem IP existente, pessoas reais, logos, texto legível, créditos inventados ou marca-d’água.",
      "product-ui": "Para roteiristas de microdrama, criadores de comédia e equipes de storyboard físico, crie um quadro congelado cinematográfico 16:9 de [situação cômica] em [local]. Mostre a cadeia de causa e efeito —[objeto] vai de [início] a [queda]— com reações expressivas, peso crível, espaço seguro e um ângulo que deixe a piada clara. Mantenha personagens, objetos, figurino e direção de tela consistentes para um clipe de comédia física; adicione diálogos ou legendas depois. Sem ferimentos, acrobacias perigosas, marcas reais, texto legível, IP existente ou marca-d’água.",
      "food-editorial": "Para arquivistas, museus, historiadores e documentaristas, restaure e reencene uma foto histórica 16:9 de [ano ou época] mostrando [pessoas comuns e atividade] em [local]. Reconstrua roupas, arquitetura, ferramentas e luz da época; preserve o grão natural e algumas marcas reparadas enquanto recupera a textura sem identificar pessoas. Componha um quadro estável para depois animar respiração, olhar ou movimento ambiental sutil. Sem figuras públicas reais, símbolos políticos, placas legíveis, objetos modernos, legendas inventadas, logos ou marca-d’água.",
    },
  },
  ru: {
    labels: {
      "product-hero": "Игровой UI и динамическая смена снаряжения",
      "social-ad": "Симуляция телевизионной спортивной трансляции",
      "catalog-variant": "Брендовый TVC и бесшовная витрина e-commerce",
      "editorial-portrait": "Кинематографический персонаж и режиссура сториборда",
      "product-ui": "Комедийная сценка и физическое повествование",
      "food-editorial": "Реставрация старых фото и оживление истории",
    },
    prompts: {
      "product-hero": "Для дизайнеров игрового UI и интеракций создайте статичный ключевой кадр 16:9 для [оригинальная игра и персонаж], который можно сопоставить со сменой снаряжения. Покажите того же персонажа с [оружие или предмет], понятной композицией HUD/экипировки и отдельным слотом альтернативного предмета для следующего кадра. Зафиксируйте камеру, позу, силуэт костюма, направление света и геометрию, чтобы Seedance плавно анимировал смену, способность или меню. Сохраните читаемость игрового арта и иерархии интерфейса; подписи добавьте после. Без существующих IP, реальных логотипов, читаемого текста, выдуманных характеристик и водяных знаков.",
      "social-ad": "Для команд спортивных трансляций и маркетинга создайте реалистичный телевизионный кадр 16:9 с [вид спорта и решающий момент] на [арена]. Передайте физически правдоподобный миг: ракурс вещательной камеры, естественный motion blur, реальные столкновения или траекторию мяча, глубину трибун и лёгкую компрессию кодека. Оставьте чистые зоны под абстрактный счёт, часы, нижнюю плашку и повторы, используя простые формы и цифры; финальные надписи добавьте после. Сохраните форму, экипировку, свет и направление камеры, чтобы Seedance продолжил кадр тряской и атмосферой стадиона. Без настоящих лиг, команд, спортсменов, спонсоров, логотипов, читаемых слов, выдуманных заявлений и водяного знака.",
      "catalog-variant": "Для брендовых, рекламных и e-commerce команд создайте коммерческий продуктовый кадр 16:9 для [продукт] в [сцена или окружение]. Зафиксируйте силуэт, пропорции, материалы, детали упаковки и безопасные пустые области бренда; начните с [макро-деталь, ракурс три четверти или оборот 360 градусов]. Используйте выверенный TVC-свет, контролируемые отражения и чистую поверхность, чтобы продолжить кадр в раскрытие продукта или вариант каталога. Оставьте место для заголовка и CTA, добавляемых после. Без реальных логотипов, читаемых слов, выдуманных обещаний, лишних продуктов, людей, рук и водяного знака.",
      "editorial-portrait": "Для кинорежиссёров, художников сториборда и дизайнеров персонажей создайте кинематографический ключевой кадр 16:9 с [персонаж] в [сцена] с осознанного [общий, средний, крупный или через плечо] ракурса. Сохраните черты личности, одежду, реквизит, направление экрана, свет и пространственные связи, чтобы использовать кадр для других ракурсов и непрерывного плана. Разделите передний, средний и задний планы, оставьте действие выразительным и естественным; диалоги и субтитры добавьте после. Без существующих IP, реальных людей, логотипов, читаемых слов, выдуманных титров и водяного знака.",
      "product-ui": "Для авторов коротких драм, комедийных создателей и команд физических сторибордов создайте кинематографичный стоп-кадр 16:9 с [комедийная завязка] в [место]. Покажите цепочку причины и следствия —[объект] движется от [старт] к [приземление]— с выразительной реакцией, правдоподобным весом, безопасной дистанцией и ракурсом, который делает шутку понятной. Сохраните персонажей, реквизит, одежду и направление экрана для короткой физической комедии; диалог и подписи добавьте после. Без травм, опасных трюков, реальных брендов, читаемых слов, существующих IP и водяного знака.",
      "food-editorial": "Для архивистов, музеев, историков и документалистов восстановите и воссоздайте историческую фотографию 16:9 из [год или эпоха] с [обычные люди и действие] в [место]. Воссоздайте одежду, архитектуру, инструменты и свет эпохи; сохраните естественное зерно плёнки и несколько следов ремонта, восстановив фактуру без идентификации людей. Создайте устойчивую композицию для последующей анимации дыхания, взгляда или лёгкого движения среды. Без реальных публичных лиц, политических символов, читаемых вывесок, современных предметов, придуманных подписей, логотипов и водяных знаков.",
    },
  },
  ja: {
    labels: {
      "product-hero": "ゲーム UI 操作と装備の動的切り替え",
      "social-ad": "高精細なテレビ／スポーツ中継シミュレーション",
      "catalog-variant": "ブランド TVC とシームレスな EC 商品展示",
      "editorial-portrait": "映画級キャラクターと多視点ストーリーボード演出",
      "product-ui": "コメディと物理演出のストーリーテリング",
      "food-editorial": "古写真の修復と歴史・人文シーンの蘇生",
    },
    prompts: {
      "product-hero": "ゲーム UI／インタラクションデザイナー向けに、[オリジナルのゲームとキャラクター]の装備切り替えに対応する16:9の静止キーフレームを作成。［武器またはアイテム］を装備した同じキャラクター、明確な HUD／ロードアウト、次フレーム用に分離した代替装備スロットを配置する。カメラ、ポーズ、衣装シルエット、光の方向、プロップ形状を固定し、Seedanceで交換・スキル発動・メニュー操作を滑らかにアニメーションできるようにする。ゲームアートとUI階層を読みやすく保ち、ラベルは後編集。既存IP、実在ロゴ、読める文字、架空ステータス、透かしは入れない。",
      "social-ad": "スポーツ中継・ライブ演出・スポーツマーケティング向けに、[競技と決定的瞬間]を[会場]で捉えた16:9の高精細テレビ中継フレームを作成。中継カメラの視点、自然なモーションブラー、物理的に自然な接触やボール軌道、観客の奥行き、軽いコーデック圧縮感を表現する。汎用スコア、時計、テロップ、リプレイ印用の余白を抽象形状と簡単な数字だけで確保し、最終文字は後編集。ユニフォーム、機材、照明、カメラ方向を固定して、Seedanceで手ブレと会場音のある短いハイライトへ拡張できるようにする。実在リーグ、チーム、選手、スポンサー、ロゴ、読める文字、架空の主張、透かしは不可。",
      "catalog-variant": "ブランド・広告・ECチーム向けに、[商品]を[セットまたは環境]で見せる16:9の商用プロダクトヒーローを作成。商品シルエット、比率、素材、パッケージ細部、ブランド用の安全な余白を固定し、[マクロ詳細、3/4ヒーロー、360度回転]のいずれかを開始フレームにする。洗練されたTVC照明、反射、清潔な面を使い、商品リビールやECバリエーションへ自然に繋げる。見出しとCTAの余白を残し、文言は後編集。実在ロゴ、読める単語、架空の効能、余分な商品、人、手、透かしは不可。",
      "editorial-portrait": "映画監督・ストーリーボードアーティスト・キャラクターデザイナー向けに、[キャラクター]を[シーン]で捉えた16:9の映画的キーフレームを、[ワイド／ミディアム／クローズ／肩越し]の意図したアングルから作成。別アングルや連続ショットの参照にできるよう、人物の特徴、衣装、プロップ、画面方向、光の方向、空間関係を維持する。前景・中景・背景を明確にし、動きは自然で表情豊かに。台詞と字幕は後編集。既存IP、実在人物、ロゴ、読める文字、架空クレジット、透かしは不可。",
      "product-ui": "短編ドラマ脚本家・コメディ制作者・物理ストーリーボードチーム向けに、[場所]の[コメディ設定]を16:9の映画的な静止フレームにする。[物体]が[開始]から[着地点]へ動く因果の流れを、表情豊かな反応、自然な重量感、安全な間隔、ギャグが伝わるカメラ角度で示す。短いフィジカルコメディに使えるよう、人物、プロップ、衣装、画面方向を統一し、台詞や字幕は後編集。けが、危険なスタント、実在ブランド、読める文字、既存IP、透かしは不可。",
      "food-editorial": "アーキビスト・博物館・歴史研究者・ドキュメンタリー制作者向けに、[年代または時代]の[普通の人々と活動]を[場所]で記録した16:9の歴史写真を修復・再構成する。時代に合う衣服、建築、道具、光を再現し、自然なフィルム粒子と少数の修復痕を残しながら質感を回復する。ただし人物を特定可能にしない。呼吸、視線、環境の風などを後で穏やかに動かせる安定した構図にする。実在の公人、政治記号、読める看板、現代物、架空キャプション、ロゴ、透かしは不可。",
    },
  },
  vi: {
    labels: {
      "product-hero": "Tương tác UI game và chuyển đổi trang bị",
      "social-ad": "Mô phỏng truyền hình trực tiếp sự kiện thể thao",
      "catalog-variant": "TVC thương hiệu và trưng bày thương mại điện tử liền mạch",
      "editorial-portrait": "Nhân vật điện ảnh và đạo diễn storyboard đa góc",
      "product-ui": "Tiểu phẩm hài và kể chuyện vật lý",
      "food-editorial": "Phục hồi ảnh cũ và hồi sinh lịch sử – nhân văn",
    },
    prompts: {
      "product-hero": "Dành cho nhà thiết kế UI và tương tác game: tạo khung hình tĩnh 16:9 cho [game và nhân vật gốc], dùng được cho chuyển cảnh đổi trang bị. Giữ cùng nhân vật với [vũ khí hoặc vật phẩm] đang trang bị, bố cục HUD/loadout rõ ràng và một ô trang bị thay thế tách biệt cho khung tiếp theo. Khóa máy quay, tư thế, dáng trang phục, hướng sáng và hình học đạo cụ để Seedance có thể hoạt ảnh hóa việc đổi đồ, tung kỹ năng hoặc thao tác menu liền mạch. Giữ thứ bậc giao diện dễ đọc, thêm nhãn ở hậu kỳ. Không dùng IP có sẵn, logo thật, chữ đọc được, chỉ số bịa đặt hoặc watermark.",
      "social-ad": "Dành cho đội phát sóng và marketing thể thao: tạo khung truyền hình 16:9 độ chân thực cao về [môn thể thao và khoảnh khắc quyết định] tại [sân]. Ghi lại khoảnh khắc hợp lý về vật lý với góc máy phát sóng, nhòe chuyển động tự nhiên, va chạm hoặc quỹ đạo bóng đáng tin, chiều sâu khán giả và chút nén codec. Chừa vùng sạch cho bảng điểm, đồng hồ, lower-third và dấu replay chung bằng hình trừu tượng cùng chữ số đơn giản; thêm nội dung cuối ở hậu kỳ. Giữ đồng phục, thiết bị, ánh sáng và hướng máy để Seedance kéo dài thành highlight có rung máy và âm thanh khán đài. Không dùng giải đấu, đội, vận động viên, nhà tài trợ, logo, chữ đọc được, tuyên bố bịa đặt hoặc watermark.",
      "catalog-variant": "Dành cho đội thương hiệu, quảng cáo và thương mại điện tử: tạo hero sản phẩm thương mại 16:9 cho [sản phẩm] trong [bối cảnh]. Khóa dáng, tỷ lệ, vật liệu, chi tiết bao bì và khoảng trống an toàn cho thương hiệu; bắt đầu bằng [chi tiết macro, hero ba phần tư hoặc xoay 360 độ]. Dùng ánh sáng TVC tinh gọn, phản xạ có kiểm soát và bề mặt sạch để nối thành cảnh reveal hoặc biến thể sản phẩm. Chừa chỗ cho tiêu đề và CTA thêm sau. Không có logo thật, từ đọc được, công dụng bịa đặt, sản phẩm thừa, người, tay hoặc watermark.",
      "editorial-portrait": "Dành cho đạo diễn, họa sĩ storyboard và nhà thiết kế nhân vật: tạo keyframe điện ảnh 16:9 của [nhân vật] trong [cảnh] từ góc [toàn, trung, cận hoặc qua vai]. Giữ dấu hiệu nhận diện, trang phục, đạo cụ, hướng màn hình, hướng sáng và quan hệ không gian để dùng làm khung tham chiếu cho góc máy khác và cảnh liên tục. Tách tiền cảnh, trung cảnh, hậu cảnh rõ ràng với hành động tự nhiên; để lời thoại và phụ đề cho hậu kỳ. Không dùng IP có sẵn, người thật, logo, chữ đọc được, credit bịa đặt hoặc watermark.",
      "product-ui": "Dành cho biên kịch micro-drama, người làm hài và đội storyboard vật lý: tạo khung hình đóng băng điện ảnh 16:9 của [tình huống hài] tại [địa điểm]. Thể hiện chuỗi nguyên nhân–kết quả —[vật thể] đi từ [điểm bắt đầu] đến [điểm rơi]— với phản ứng biểu cảm, trọng lượng đáng tin, khoảng cách an toàn và góc máy làm trò đùa dễ hiểu. Giữ nhân vật, đạo cụ, trang phục và hướng màn hình nhất quán cho clip hài vật lý; thêm lời thoại hoặc phụ đề ở hậu kỳ. Không chấn thương, pha nguy hiểm, thương hiệu thật, chữ đọc được, IP có sẵn hoặc watermark.",
      "food-editorial": "Dành cho lưu trữ, bảo tàng, sử gia và nhà làm phim tài liệu: phục hồi và tái dựng ảnh lịch sử 16:9 từ [năm hoặc thời kỳ], ghi lại [người bình thường và hoạt động] ở [địa điểm]. Khôi phục quần áo, kiến trúc, công cụ và ánh sáng đúng thời kỳ; giữ hạt phim tự nhiên và vài dấu vết sửa chữa, phục hồi chi tiết nhưng không làm người trong ảnh có thể nhận dạng. Bố cục ổn định để sau đó thêm hơi thở, chuyển mắt hoặc gió môi trường nhẹ. Không nhân vật công chúng thật, biểu tượng chính trị, biển hiệu đọc được, vật hiện đại, chú thích bịa đặt, logo hoặc watermark.",
    },
  },
  de: {
    labels: {
      "product-hero": "Spiel-UI und dynamischer Ausrüstungswechsel",
      "social-ad": "Hochrealistische TV-Sportsendungssimulation",
      "catalog-variant": "Marken-TVC und nahtlose E-Commerce-Präsentation",
      "editorial-portrait": "Filmfigur und Storyboard-Regie aus mehreren Perspektiven",
      "product-ui": "Komödienszene und physisches Storytelling",
      "food-editorial": "Restaurierung alter Fotos und historische Wiederbelebung",
    },
    prompts: {
      "product-hero": "Für Game-UI- und Interaktionsdesigner: Erstelle ein statisches 16:9-Keyframe für [originales Spiel und Figur], das zu einem Ausrüstungswechsel passt. Zeige dieselbe Figur mit [Waffe oder Gegenstand], ein klares HUD/Loadout und einen getrennten alternativen Slot für das nächste Bild. Fixiere Kamera, Pose, Kostümsilhouette, Lichtrichtung und Objektgeometrie, damit Seedance Wechsel, Fähigkeit oder Menü flüssig animieren kann. Halte Spielgrafik und UI-Hierarchie lesbar; Beschriftungen später ergänzen. Keine bestehende IP, echten Logos, lesbaren Wörter, erfundenen Werte oder Wasserzeichen.",
      "social-ad": "Für Sportübertragungs-, Live-Regie- und Sportmarketingteams: Erstelle ein hochrealistisches 16:9-TV-Bild von [Sport und entscheidendem Moment] in [Stadion]. Zeige einen physikalisch glaubwürdigen Augenblick mit Broadcast-Perspektive, natürlicher Bewegungsunschärfe, realistischen Kollisionen oder Ballflug, Zuschauertiefe und leichter Codec-Kompression. Lasse saubere Flächen für generische Anzeige, Uhr, Bauchbinde und Replay-Marker mit abstrakten Formen und einfachen Ziffern; finalen Text später setzen. Halte Trikots, Ausrüstung, Licht und Kamerarichtung konstant, damit Seedance einen kurzen Highlight-Clip mit Wackler und Arena-Atmosphäre fortsetzen kann. Keine echten Ligen, Teams, Sportler, Sponsoren, Logos, lesbaren Wörter, erfundenen Behauptungen oder Wasserzeichen.",
      "catalog-variant": "Für Marken-, Werbe- und E-Commerce-Teams: Erstelle ein kommerzielles 16:9-Produktmotiv für [Produkt] in [Set oder Umgebung]. Fixiere Silhouette, Proportionen, Materialien, Verpackungsdetails und markensichere Freiflächen; beginne mit [Makrodetail, Dreiviertel-Hero oder 360-Grad-Drehung]. Nutze kontrolliertes TVC-Licht, Reflexionen und eine saubere Oberfläche, damit das Motiv nahtlos zu Produkt-Reveal oder Shop-Variante erweitert werden kann. Platz für Überschrift und CTA lassen, Text später ergänzen. Keine echten Logos, lesbaren Wörter, erfundenen Versprechen, Zusatzprodukte, Personen, Hände oder Wasserzeichen.",
      "editorial-portrait": "Für Filmregisseure, Storyboard-Künstler und Charakterdesigner: Erstelle ein filmisches 16:9-Keyframe von [Figur] in [Szene] aus einer bewussten [Totalen-, mittleren, nahen oder Over-Shoulder]-Perspektive. Bewahre Identität, Kleidung, Requisiten, Bildschirmrichtung, Lichtrichtung und räumliche Beziehungen, damit das Bild als Referenz für andere Winkel und einen durchgehenden Shot dient. Gestalte klaren Vorder-, Mittel- und Hintergrund mit natürlicher Aktion; Dialoge und Untertitel später ergänzen. Keine bestehende IP, echten Personen, Logos, lesbaren Wörter, erfundenen Credits oder Wasserzeichen.",
      "product-ui": "Für Kurzdrama-Autoren, Comedy-Creator und Teams für physische Storyboards: Erstelle ein filmisches 16:9-Standbild von [komödiantischer Ausgangslage] in [Ort]. Zeige die Ursache-Wirkungs-Kette —[Objekt] bewegt sich von [Start] zu [Landung]— mit ausdrucksstarken Reaktionen, glaubwürdigem Gewicht, sicherem Abstand und einem verständlichen Gag-Winkel. Halte Figuren, Requisiten, Kleidung und Bildschirmrichtung für einen Physical-Comedy-Clip konstant; Dialoge oder Untertitel später ergänzen. Keine Verletzungen, gefährlichen Stunts, echten Marken, lesbaren Wörter, bestehender IP oder Wasserzeichen.",
      "food-editorial": "Für Archive, Museen, Historiker und Dokumentarfilmer: Restauriere und rekonstruiere ein historisches 16:9-Foto aus [Jahr oder Epoche] mit [gewöhnlichen Menschen und Aktivität] an [Ort]. Stelle zeittypische Kleidung, Architektur, Werkzeuge und Licht wieder her; bewahre Filmkorn und einige Reparaturspuren, verbessere die Textur aber mache Personen nicht identifizierbar. Erzeuge ein stabiles Bild für spätere leichte Animation von Atmung, Blick oder Umgebung. Keine realen Persönlichkeiten, politischen Symbole, lesbaren Schilder, modernen Objekte, erfundenen Bildunterschriften, Logos oder Wasserzeichen.",
    },
  },
  id: {
    labels: {
      "product-hero": "Interaksi UI game dan pergantian perlengkapan",
      "social-ad": "Simulasi siaran TV pertandingan olahraga",
      "catalog-variant": "TVC merek dan tampilan e-commerce tanpa jeda",
      "editorial-portrait": "Karakter sinematik dan pengarahan storyboard",
      "product-ui": "Sketsa komedi dan penceritaan fisik",
      "food-editorial": "Pemulihan foto lama dan penghidupan sejarah",
    },
    prompts: {
      "product-hero": "Untuk desainer UI dan interaksi game, buat keyframe game statis 16:9 untuk [game dan karakter orisinal] yang dapat dicocokkan dalam transisi pergantian perlengkapan. Tampilkan karakter yang sama dengan [senjata atau item] terpasang, komposisi HUD/loadout yang jelas, serta slot perlengkapan alternatif yang terpisah untuk frame berikutnya. Kunci kamera, pose, siluet kostum, arah cahaya, dan geometri properti agar Seedance dapat menganimasikan pergantian, pelepasan skill, atau interaksi menu dengan mulus. Jaga hierarki game art dan antarmuka tetap terbaca; tambahkan label di pascaproduksi. Tanpa IP yang ada, logo nyata, kata yang terbaca, statistik rekaan, atau watermark.",
      "social-ad": "Untuk tim siaran dan pemasaran olahraga, buat frame siaran televisi 16:9 yang sangat realistis tentang [olahraga dan momen penentu] di [venue]. Tangkap momen yang masuk akal secara fisik dengan perspektif kamera siaran, motion blur alami, tabrakan atau lintasan bola yang realistis, kedalaman penonton, dan sedikit tekstur kompresi codec. Sisakan area bersih untuk skor, jam, lower-third, dan penanda replay generik memakai bentuk abstrak serta angka sederhana; tambahkan teks akhir di pascaproduksi. Pertahankan seragam, peralatan, cahaya, dan arah kamera agar Seedance dapat memperpanjangnya menjadi highlight dengan guncangan kamera dan suasana arena. Tanpa liga, tim, atlet, sponsor, logo, kata terbaca, klaim rekaan, atau watermark.",
      "catalog-variant": "Untuk tim merek, iklan, dan e-commerce, buat hero produk komersial 16:9 untuk [produk] di [set atau lingkungan]. Kunci siluet, proporsi, bahan, detail kemasan, dan ruang kosong yang aman untuk merek; mulai dari [detail makro, hero tiga perempat, atau putaran 360 derajat]. Gunakan pencahayaan TVC yang rapi, pantulan terkontrol, dan permukaan bersih agar gambar dapat berlanjut menjadi reveal produk atau varian e-commerce. Sisakan ruang untuk judul dan CTA yang ditambahkan nanti. Tanpa logo nyata, kata terbaca, klaim rekaan, produk tambahan, orang, tangan, atau watermark.",
      "editorial-portrait": "Untuk sutradara film, seniman storyboard, dan desainer karakter, buat keyframe sinematik 16:9 tentang [karakter] di [adegan] dari sudut [lebar, sedang, dekat, atau over-the-shoulder]. Pertahankan ciri identitas, pakaian, properti, arah layar, arah cahaya, dan hubungan ruang agar gambar dapat menjadi referensi sudut lain dan shot berkelanjutan. Bangun latar depan, tengah, dan belakang yang jelas dengan aksi natural; tambahkan dialog dan caption di pascaproduksi. Tanpa IP yang ada, orang nyata, logo, kata terbaca, kredit rekaan, atau watermark.",
      "product-ui": "Untuk penulis micro-drama, kreator komedi, dan tim storyboard fisik, buat freeze-frame sinematik 16:9 tentang [situasi komedi] di [lokasi]. Tampilkan rantai sebab-akibat—[objek] bergerak dari [awal] ke [pendaratan]—dengan reaksi ekspresif, bobot yang masuk akal, jarak aman, dan sudut kamera yang membuat lelucon mudah dipahami. Pertahankan karakter, properti, pakaian, dan arah layar untuk klip komedi fisik; tambahkan dialog atau caption nanti. Tanpa cedera, aksi berbahaya, merek nyata, kata terbaca, IP yang ada, atau watermark.",
      "food-editorial": "Untuk arsiparis, museum, sejarawan, dan pembuat dokumenter, pulihkan dan tata ulang foto sejarah 16:9 dari [tahun atau era] yang menampilkan [orang biasa dan aktivitas] di [tempat]. Rekonstruksi pakaian, arsitektur, alat, dan cahaya sesuai zaman; pertahankan grain film alami dan beberapa bekas perbaikan sambil memulihkan tekstur tanpa membuat orang dapat dikenali. Susun frame stabil untuk animasi napas, gerak mata, atau angin lingkungan yang halus. Tanpa tokoh publik nyata, simbol politik, papan terbaca, benda modern, caption rekaan, logo, atau watermark.",
    },
  },
};

type ImagePlaygroundScenario = Record<Locale, string>;

/**
 * The nine curated Playground starters keep their own subject while the
 * surrounding instruction is translated. This avoids replacing a reviewed
 * model-specific image brief with one generic sentence merely to localize it.
 */
const IMAGE_PLAYGROUND_SCENARIOS: Record<string, ImagePlaygroundScenario> = {
  "gpt-image-2": {
    en: "a frosted glass serum dropper on a pale aqua stone surface with fine water droplets",
    zh: "浅水绿色石面上的磨砂玻璃精华滴管与细小水珠",
    es: "un gotero de sérum de vidrio esmerilado sobre piedra aqua pálida con finas gotas de agua",
    fr: "un flacon compte-gouttes en verre dépoli sur une pierre bleu aqua pâle, couvert de fines gouttes",
    pt: "um frasco conta-gotas de vidro fosco sobre pedra azul-clara com pequenas gotas de água",
    ru: "матовый стеклянный флакон-капельница на светло-бирюзовой каменной поверхности с каплями воды",
    ja: "淡いアクア色の石面に置いたすりガラスの美容液スポイトと細かな水滴",
    vi: "lọ serum thủy tinh mờ trên mặt đá xanh nhạt với những giọt nước nhỏ",
    de: "einen Tropfer aus Milchglas auf einer hell aqua­farbenen Steinfläche mit feinen Wassertropfen",
    id: "botol penetes serum kaca buram di atas batu aqua pucat dengan butiran air halus",
  },
  "gemini-2-5-flash-image": {
    en: "a matte skincare bottle and matching cream jar on a travertine shelf",
    zh: "洞石架上的哑光护肤瓶与配套面霜罐",
    es: "una botella mate de cuidado facial y un tarro de crema a juego sobre una repisa de travertino",
    fr: "un flacon de soin mat et son pot de crème assorti sur une étagère en travertin",
    pt: "um frasco fosco de skincare e um pote de creme combinando em uma prateleira de travertino",
    ru: "матовый флакон косметики и подходящая баночка крема на полке из травертина",
    ja: "トラバーチン棚に置いたマットなスキンケアボトルと同系色のクリーム容器",
    vi: "chai chăm sóc da mờ và hũ kem đồng bộ trên kệ travertine",
    de: "eine matte Hautpflegeflasche mit passendem Cremetiegel auf einem Travertinregal",
    id: "botol skincare matte dan stoples krim serasi di rak travertine",
  },
  "gemini-3-pro-image": {
    en: "a brushed stainless-steel bottle centered on seamless white",
    zh: "无缝白背景中央的拉丝不锈钢水瓶",
    es: "una botella de acero inoxidable cepillado centrada sobre un fondo blanco continuo",
    fr: "une bouteille en acier inoxydable brossé centrée sur un fond blanc uniforme",
    pt: "uma garrafa de aço inoxidável escovado centralizada em um fundo branco contínuo",
    ru: "бутылка из шлифованной нержавеющей стали по центру бесшовного белого фона",
    ja: "継ぎ目のない白背景の中央に置いたヘアライン仕上げのステンレスボトル",
    vi: "chai thép không gỉ xước đặt giữa nền trắng liền mạch",
    de: "eine gebürstete Edelstahlflasche mittig auf nahtlosem Weiß",
    id: "botol baja tahan karat bertekstur sikat di tengah latar putih tanpa sambungan",
  },
  "gemini-3-1-flash-image": {
    en: "a deep-green glass pump bottle on sculptural white stone",
    zh: "雕塑感白石上的深绿色玻璃按压瓶",
    es: "un frasco con dispensador de vidrio verde oscuro sobre piedra blanca escultórica",
    fr: "un flacon-pompe en verre vert profond sur une pierre blanche sculpturale",
    pt: "um frasco pump de vidro verde-escuro sobre pedra branca escultural",
    ru: "флакон с помпой из тёмно-зелёного стекла на скульптурном белом камне",
    ja: "彫刻的な白い石の上に置いた深緑色ガラスのポンプボトル",
    vi: "chai bơm bằng kính xanh đậm trên khối đá trắng tạo hình",
    de: "eine dunkelgrüne Glasflasche mit Pumpe auf skulpturalem weißen Stein",
    id: "botol pompa kaca hijau tua di atas batu putih berukir",
  },
  "gemini-3-1-flash-lite-image": {
    en: "a translucent emerald pump bottle on a dark graphite pedestal",
    zh: "深石墨台座上的半透明祖母绿按压瓶",
    es: "un frasco dispensador esmeralda translúcido sobre un pedestal de grafito oscuro",
    fr: "un flacon-pompe émeraude translucide sur un socle en graphite sombre",
    pt: "um frasco pump esmeralda translúcido sobre um pedestal de grafite escuro",
    ru: "полупрозрачный изумрудный флакон с помпой на тёмно-графитовом постаменте",
    ja: "濃いグラファイト台座に置いた半透明エメラルド色のポンプボトル",
    vi: "chai bơm màu ngọc lục bảo bán trong suốt trên bệ than chì tối",
    de: "eine durchscheinende smaragdgrüne Pumpflasche auf einem dunklen Graphitsockel",
    id: "botol pompa zamrud tembus pandang di atas pedestal grafit gelap",
  },
  "grok-imagine-image": {
    en: "an open matte-black wireless earbud case on dark slate",
    zh: "深色板岩上的打开状态哑光黑无线耳机盒",
    es: "un estuche abierto de auriculares inalámbricos negro mate sobre pizarra oscura",
    fr: "un boîtier ouvert d’écouteurs sans fil noir mat sur une ardoise sombre",
    pt: "um estojo aberto de fones sem fio preto fosco sobre ardósia escura",
    ru: "открытый матовый чёрный футляр беспроводных наушников на тёмном сланце",
    ja: "ダークスレート上に置いた、開いたマットブラックのワイヤレスイヤホンケース",
    vi: "hộp tai nghe không dây đen mờ đang mở trên nền đá phiến tối",
    de: "ein geöffnetes matt-schwarzes Ladeetui für kabellose Ohrhörer auf dunklem Schiefer",
    id: "casing earbud nirkabel hitam matte yang terbuka di atas batu sabak gelap",
  },
  "grok-imagine-image-pro": {
    en: "a closed matte-black wireless earbud charging case on light concrete",
    zh: "浅色混凝土表面上的闭合哑光黑无线耳机充电盒",
    es: "un estuche de carga cerrado de auriculares inalámbricos negro mate sobre hormigón claro",
    fr: "un boîtier de charge fermé pour écouteurs sans fil noir mat sur du béton clair",
    pt: "um estojo de carregamento fechado de fones sem fio preto fosco sobre concreto claro",
    ru: "закрытый матовый чёрный зарядный футляр беспроводных наушников на светлом бетоне",
    ja: "明るいコンクリート面に置いた、閉じたマットブラックのワイヤレスイヤホン充電ケース",
    vi: "hộp sạc tai nghe không dây đen mờ đóng kín trên bề mặt bê tông sáng",
    de: "ein geschlossenes matt-schwarzes Ladeetui für kabellose Ohrhörer auf hellem Beton",
    id: "casing pengisi daya earbud nirkabel hitam matte yang tertutup di atas beton terang",
  },
  "grok-imagine-image-quality": {
    en: "a handmade ceramic travel mug on a sunlit coastal stone ledge",
    zh: "阳光海岸石台上的手工陶瓷旅行杯",
    es: "una taza de viaje de cerámica artesanal sobre un saliente de piedra costera iluminado por el sol",
    fr: "un mug de voyage en céramique artisanale sur une corniche côtière baignée de soleil",
    pt: "uma caneca de viagem de cerâmica artesanal em uma borda de pedra costeira ensolarada",
    ru: "керамическая дорожная кружка ручной работы на залитом солнцем прибрежном каменном уступе",
    ja: "陽光が差す海岸の石棚に置いた手作り陶器のトラベルマグ",
    vi: "cốc du lịch gốm thủ công trên bậc đá ven biển ngập nắng",
    de: "einen handgefertigten Keramikbecher auf einer sonnenbeschienenen Küstensteinkante",
    id: "mug perjalanan keramik buatan tangan di tepian batu pantai yang terkena sinar matahari",
  },
  "nano-banana-pro-preview": {
    en: "the same pump bottle in magenta, sky blue, and amber colorways",
    zh: "同一按压瓶的洋红、天蓝和琥珀三种配色",
    es: "el mismo frasco con dispensador en variantes magenta, azul cielo y ámbar",
    fr: "le même flacon-pompe en variantes magenta, bleu ciel et ambre",
    pt: "o mesmo frasco pump nas cores magenta, azul-céu e âmbar",
    ru: "один и тот же флакон с помпой в пурпурном, небесно-синем и янтарном цветах",
    ja: "同じポンプボトルをマゼンタ、スカイブルー、アンバーの3色で展開",
    vi: "cùng một chai bơm với ba biến thể màu hồng magenta, xanh da trời và hổ phách",
    de: "dieselbe Pumpflasche in Magenta, Himmelblau und Bernstein",
    id: "botol pompa yang sama dalam varian magenta, biru langit, dan amber",
  },
};

const IMAGE_PLAYGROUND_PROMPT_FRAME: Record<Locale, (modelName: string, subject: string) => string> = {
  en: (modelName, subject) => `Create a premium product image with ${modelName}: ${subject}. Preserve the supplied packaging, proportions, and materials; use controlled light, a clean grounded shadow, and safe margins for catalog copy. No readable text, invented branding, extra products, people, hands, or watermark.`,
  zh: (modelName, subject) => `使用 ${modelName} 制作高级产品图：${subject}。保持包装、比例和材质准确，使用受控光线、干净的接触阴影，并为商品文案留出安全边距。不要可读文字、虚构品牌、额外产品、人物、手或水印。`,
  es: (modelName, subject) => `Crea con ${modelName} una imagen de producto premium: ${subject}. Conserva el envase, las proporciones y los materiales; usa luz controlada, una sombra de contacto limpia y márgenes seguros para el texto del catálogo. Sin texto legible, marca inventada, productos extra, personas, manos ni marca de agua.`,
  fr: (modelName, subject) => `Créez avec ${modelName} une image produit haut de gamme : ${subject}. Préservez packaging, proportions et matières; utilisez une lumière maîtrisée, une ombre de contact nette et des marges sûres pour le texte du catalogue. Sans texte lisible, marque inventée, produits supplémentaires, personnes, mains ni filigrane.`,
  pt: (modelName, subject) => `Crie com ${modelName} uma imagem de produto premium: ${subject}. Preserve embalagem, proporções e materiais; use luz controlada, sombra de contato limpa e margens seguras para o texto do catálogo. Sem texto legível, marca inventada, produtos extras, pessoas, mãos ou marca-d’água.`,
  ru: (modelName, subject) => `Создайте с помощью ${modelName} премиальное изображение продукта: ${subject}. Сохраните упаковку, пропорции и материалы; используйте контролируемый свет, чистую контактную тень и безопасные поля для текста каталога. Без читаемого текста, выдуманного бренда, лишних товаров, людей, рук и водяного знака.`,
  ja: (modelName, subject) => `${modelName}でプレミアムな商品画像を作成：${subject}。パッケージ、比率、素材を正確に保ち、制御された光、自然な接地影、カタログ文言用の安全な余白を使う。読める文字、架空ブランド、余分な商品、人、手、透かしは入れない。`,
  vi: (modelName, subject) => `Tạo ảnh sản phẩm cao cấp bằng ${modelName}: ${subject}. Giữ nguyên bao bì, tỷ lệ và vật liệu; dùng ánh sáng có kiểm soát, bóng tiếp xúc sạch và lề an toàn cho nội dung catalog. Không chữ đọc được, thương hiệu bịa đặt, sản phẩm thừa, người, tay hoặc watermark.`,
  de: (modelName, subject) => `Erstelle mit ${modelName} ein hochwertiges Produktbild: ${subject}. Bewahre Verpackung, Proportionen und Materialien; nutze kontrolliertes Licht, einen sauberen Kontaktschatten und sichere Ränder für Katalogtext. Keine lesbaren Wörter, erfundenen Marken, Zusatzprodukte, Personen, Hände oder Wasserzeichen.`,
  id: (modelName, subject) => `Buat gambar produk premium dengan ${modelName}: ${subject}. Pertahankan kemasan, proporsi, dan material; gunakan cahaya terkontrol, bayangan kontak yang bersih, serta margin aman untuk teks katalog. Tanpa teks terbaca, merek rekaan, produk tambahan, orang, tangan, atau watermark.`,
};

function getImagePromptSet(modelId?: string): ImagePromptSet | undefined {
  if (!modelId) return undefined;
  const resolvedModelId = resolveImagePosterModelId(modelId);
  return resolvedModelId ? IMAGE_MODEL_ENGLISH_PROMPTS[resolvedModelId] : undefined;
}

/**
 * Return a fresh array so callers can safely annotate examples for a specific
 * model without mutating the shared catalog.  `modelId` is accepted as part of
 * the public API to leave room for model-family ordering while keeping today's
 * scenario set consistent across every image model.
 */
export function getImagePromptTemplates(modelId?: string, locale: Locale = "en"): ImagePromptTemplate[] {
  const copy = IMAGE_PROMPT_LOCALE_COPY[locale];
  // Resolve the model's poster set here, next to its prompt objects, so every
  // caller receives a complete scene-to-media binding. The landing page also
  // resolves this registry for its fallback handling, but callers should not
  // have to remember a second index-based join to avoid showing the gpt-image-2
  // poster on another model's card.
  const posters = modelId ? getImagePromptTemplateFallbackPosters(modelId) : undefined;
  const promptSet = getImagePromptSet(modelId);
  return IMAGE_PROMPT_TEMPLATES.map((template, index) => ({
    ...template,
    label: copy?.labels[template.id] ?? template.label,
    // English keeps model-specific reviewed scene briefs. Translated routes
    // use the complete locale prompt so visible prompt cards stay in-language.
    prompt: locale === "en"
      ? (promptSet?.[template.id as ImagePromptSceneId] ?? template.prompt)
      : (copy?.prompts?.[template.id as ImagePromptSceneId] ?? template.prompt),
    poster: posters?.[index] ?? template.poster,
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
  const normalizedModelId = normalizeImageModelId(modelId);
  const resolvedModelId = resolveImagePosterModelId(normalizedModelId);
  const curated = resolvedModelId ? IMAGE_MODEL_POSTER_SETS[resolvedModelId] : undefined;
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
 * Return packaged fallbacks for the same model and workflow lane as each
 * reviewed CDN poster. These files are derived from the corresponding CDN
 * output and are only used after the remote object fails; they must never
 * replace the primary CDN URL in the initial render.
 */
export function getImagePromptTemplateLocalFallbackPosters(modelId = ""): string[] {
  const resolvedModelId = resolveImagePosterModelId(modelId);
  if (!resolvedModelId) return [];
  return IMAGE_PROMPT_LOCAL_FALLBACK_LANES.map(
    (lane) => `/assets/model-fallback-audit/images/${lane}/${resolvedModelId}.webp`,
  );
}

/**
 * Resolve a model-specific Playground starter, including catalog aliases such
 * as `gemini-3.1-flash-image-preview`. A fresh object keeps callers from
 * accidentally mutating the shared catalog entry.
 */
export function getImagePlaygroundExample(modelId = "", locale: Locale = "en"): ImagePlaygroundExample | undefined {
  const normalizedModelId = modelId
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  const exact = IMAGE_PLAYGROUND_EXAMPLES[normalizedModelId];
  const resolvedId = exact ? normalizedModelId : Object.keys(IMAGE_PLAYGROUND_EXAMPLES).find(
    (candidate) => normalizedModelId.startsWith(`${candidate}-`) || candidate.startsWith(`${normalizedModelId}-`)
  );
  if (resolvedId && IMAGE_PLAYGROUND_EXAMPLES[resolvedId]) {
    const example = IMAGE_PLAYGROUND_EXAMPLES[resolvedId];
    // Playground prompt bodies follow the same locale contract as the
    // prompt-library cards.
    const localizedPrompt = locale === "en"
      ? example.prompt
      : IMAGE_PROMPT_LOCALE_COPY[locale]?.prompts["product-hero"] ?? localizeImagePromptText(example.prompt, locale);
    return { ...example, prompt: localizedPrompt };
  }

  return undefined;
}

export function getImagePromptTemplate(templateId: string, locale: Locale = "en"): ImagePromptTemplate | undefined {
  return getImagePromptTemplates(undefined, locale).find((template) => template.id === templateId);
}

/**
 * Localize a configured prompt-library string when it belongs to the shared
 * six-card image workflow. Unknown strings are returned untouched because
 * they may be user-authored prompts or provider-specific technical examples.
 */
export function localizeImagePromptText(prompt: string, locale: Locale): string {
  for (const promptSet of Object.values(IMAGE_MODEL_ENGLISH_PROMPTS)) {
    const entry = Object.entries(promptSet).find(([, value]) => value === prompt);
    if (entry) {
      return getImagePromptTemplate(entry[0], locale)?.prompt ?? prompt;
    }
  }

  const template = IMAGE_PROMPT_TEMPLATES.find((candidate) => candidate.prompt === prompt);
  if (template) return getImagePromptTemplate(template.id, locale)?.prompt ?? prompt;

  // Legacy serialized cards can still contain one of the former localized
  // prompt bodies. Resolve those values to the selected locale.
  for (const copy of Object.values(IMAGE_PROMPT_LOCALE_COPY)) {
    const entry = Object.entries(copy.prompts).find(([, value]) => value && prompt.startsWith(value));
    if (entry) {
      const canonical = IMAGE_PROMPT_TEMPLATES.find((candidate) => candidate.id === entry[0]);
      if (canonical) return getImagePromptTemplate(canonical.id, locale)?.prompt ?? canonical.prompt;
    }
  }

  const generic = prompt.match(/^Create a (?:high-quality|premium) product image with (.+?): *(.*)\.?$/i);
  if (generic) {
    const model = generic[1].trim();
    const starterCopy: Record<Locale, string> = {
      en: `Create a high-quality product image with ${model}: clean composition, precise lighting, strong subject focus, and realistic detail.`,
      zh: `使用 ${model} 制作高质量产品图片：构图简洁、光线精准、主体突出，并保留真实细节。`,
      es: `Crea una imagen de producto de alta calidad con ${model}: composición limpia, iluminación precisa, sujeto destacado y detalle realista.`,
      fr: `Créez une image produit de haute qualité avec ${model} : composition nette, éclairage précis, sujet bien défini et détails réalistes.`,
      pt: `Crie uma imagem de produto de alta qualidade com ${model}: composição limpa, iluminação precisa, foco forte no produto e detalhes realistas.`,
      ru: `Создайте качественное изображение продукта с помощью ${model}: чистая композиция, точный свет, акцент на объекте и реалистичные детали.`,
      ja: `${model}で高品質な商品画像を作成：すっきりした構図、正確な照明、明確な主役、リアルなディテールにする。`,
      vi: `Tạo hình ảnh sản phẩm chất lượng cao bằng ${model}: bố cục gọn, ánh sáng chính xác, chủ thể nổi bật và chi tiết chân thực.`,
      de: `Erstelle mit ${model} ein hochwertiges Produktbild: klare Komposition, präzise Beleuchtung, starker Fokus auf das Motiv und realistische Details.`,
      id: `Buat gambar produk berkualitas tinggi dengan ${model}: komposisi bersih, pencahayaan presisi, fokus subjek kuat, dan detail realistis.`,
    };
    return starterCopy[locale];
  }
  return prompt;
}
