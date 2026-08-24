import type { ModelLandingKey } from "./model-landing";

/**
 * Per-model media for the landing pages.
 *
 * Before this table every model page rendered the same five seedance clips as
 * its "prompt library" -- so gpt-image-2, an image model, advertised video
 * output it does not produce, and ten video models showed identical footage.
 * Each model now points at its own generated sample and its own cover.
 *
 * How the assets are produced: `node scripts/build-model-media.mjs`. Samples
 * are real generations from the model itself. Covers are a generated sci-fi
 * backdrop with the model id composited on afterwards -- image models render
 * text unreliably, and a cover that misspells the model it labels is worse
 * than none.
 *
 * Assets live on the CDN, never in the repo: the clips are multi-megabyte and
 * would ship in the deployment image otherwise.
 */

const CDN_BASE = "https://cdn.shulex-voc.com/flatkey/model-media";

export type ModelMediaSample = {
  /** Asset slug: <slug>.png, plus <slug>.mp4 for video models. */
  slug: string;
  kind: "image" | "video";
  /** Caption for the sample, translated through the model-landing copy maps. */
  label: ModelLandingKey;
  /** The prompt that produced this exact asset. */
  prompt: string;
};

export type ModelMedia = {
  /** Cover still, model name composited on. */
  coverSlug: string;
  /**
   * Examples for the workbench picker and for the prompt library, kept as two
   * disjoint sets.
   *
   * They serve different questions -- the workbench answers "what request do I
   * send", the library answers "what output is worth copying" -- and filling
   * both from one list rendered the same four pictures twice on one page.
   */
  workbench: ModelMediaSample[];
  library: ModelMediaSample[];
};

// Keep the payload and network work bounded on the landing page.  A model
// needs one representative playground asset; the prompt library can expose
// up to five scene cards, but never ships a second workbench set.
function normalizeModelMedia(media: ModelMedia): ModelMedia {
  const workbench = media.workbench.slice(0, 1);
  const library = [...media.library];
  return {
    ...media,
    workbench,
    library,
  };
}

/** Normalizes a model id to its asset slug: gpt-image-2, seedance-2-5, ... */
export function modelMediaSlug(modelId: string): string {
  return modelId
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export function modelCoverUrl(slug: string): string {
  return `${CDN_BASE}/cover/${slug}.png`;
}

export function modelSampleImageUrl(slug: string): string {
  return `${CDN_BASE}/sample/${slug}.png`;
}

export function modelSampleVideoUrl(slug: string): string {
  return `${CDN_BASE}/sample/${slug}.mp4`;
}

/** Development fallback while the CDN batch is being published. */
export function localModelSampleUrl(slug: string, extension: "png" | "mp4"): string {
  return `/assets/model-media/sample/${slug}.${extension}`;
}

// Keep in sync with MODELS in scripts/build-model-media.mjs: the prompts here
// must be the ones the assets were generated from, or the page claims a prompt
// produced something it did not.
const MODEL_MEDIA: Record<string, ModelMedia> = {
  "seedance-2-0": {
    coverSlug: "seedance-2-0",
    workbench: [
      { slug: "seedance-2-0-w1", kind: "video", label: "Walker crossing", prompt: "A survey walker crossing a violet salt flat at dusk, twin moons low on the horizon, dust curling off each footfall, slow tracking shot from the side, cinematic sci-fi realism." },
      { slug: "seedance-2-0-w2", kind: "video", label: "Dust wake", prompt: "Low camera behind a survey walker on a salt flat, dust streaming off its rear legs into low sun, indigo and amber palette, cinematic sci-fi realism." },
      { slug: "seedance-2-0-w3", kind: "video", label: "Horizon pan", prompt: "Slow pan across a violet salt flat at dusk, a survey walker entering frame from the left, twin moons rising, cinematic sci-fi realism." },
      { slug: "seedance-2-0-w4", kind: "video", label: "Leg detail", prompt: "Close tracking shot of a survey walker's articulated leg striking salt crust, dust bursting outward, running lights glowing, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "seedance-2-0-l1", kind: "video", label: "Wide landscape motion", prompt: "A survey walker crossing a violet salt flat at dusk, twin moons low on the horizon, dust curling off each footfall, indigo and amber palette, slow tracking shot from the side, cinematic sci-fi realism." },
      { slug: "seedance-2-0-l2", kind: "video", label: "Night crossing", prompt: "A survey walker moving across a dark salt flat at night, its running lights the only illumination, stars dense overhead, slow lateral tracking, cinematic sci-fi realism." },
      { slug: "seedance-2-0-l3", kind: "video", label: "Storm approach", prompt: "A survey walker halting as a dust wall approaches across a salt flat, light going brown, camera holding wide, cinematic sci-fi realism." },
      { slug: "seedance-2-0-l4", kind: "video", label: "Reflection crossing", prompt: "A survey walker crossing a thin layer of standing water on a salt flat, its shape mirrored below, twin moons reflected, cinematic sci-fi realism." },
    ],
  },
  "seedance-2-0-pro": {
    coverSlug: "seedance-2-0-pro",
    workbench: [
      { slug: "seedance-2-0-pro-w1", kind: "video", label: "Molten pour", prompt: "Inside a deep-space foundry, a robotic arm pours molten alloy into a mould, sparks arcing through indigo shadow, slow orbiting camera, cinematic sci-fi realism." },
      { slug: "seedance-2-0-pro-w2", kind: "video", label: "Gantry sweep", prompt: "Camera sweeping along an overhead gantry in a deep-space foundry, machinery passing beneath, amber glow from the pour below, cinematic sci-fi realism." },
      { slug: "seedance-2-0-pro-w3", kind: "video", label: "Spark shower", prompt: "Close shot of sparks showering off a cutting head in a foundry, indigo shadow behind, embers drifting toward camera, cinematic sci-fi realism." },
      { slug: "seedance-2-0-pro-w4", kind: "video", label: "Cooling rack", prompt: "Slow dolly past a rack of cooling alloy billets in a foundry, heat shimmer rising, amber fading to grey, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "seedance-2-0-pro-l1", kind: "video", label: "Industrial interior", prompt: "Inside a deep-space foundry, a robotic arm pours molten alloy into a mould, sparks arcing through indigo shadow, heat shimmer distorting the machinery behind, slow orbiting camera, cinematic sci-fi realism." },
      { slug: "seedance-2-0-pro-l2", kind: "video", label: "Arm choreography", prompt: "Three robotic arms working in sequence over a foundry mould, precise synchronised motion, amber light on their joints, cinematic sci-fi realism." },
      { slug: "seedance-2-0-pro-l3", kind: "video", label: "Furnace door", prompt: "A foundry furnace door opening, white-hot interior flaring into a dark hall, camera pushing slowly toward the opening, cinematic sci-fi realism." },
      { slug: "seedance-2-0-pro-l4", kind: "video", label: "Worker scale", prompt: "A worker in heat gear walking a foundry floor, dwarfed by the machinery above, amber and indigo palette, cinematic sci-fi realism." },
    ],
  },
  "seedance-2-0-fast": {
    coverSlug: "seedance-2-0-fast",
    workbench: [
      { slug: "seedance-2-0-fast-w1", kind: "video", label: "Canyon run", prompt: "A racing skiff banking hard through a neon canyon between floating platforms, magenta and cyan light trails streaking past, chase camera close behind, cinematic sci-fi realism." },
      { slug: "seedance-2-0-fast-w2", kind: "video", label: "Overtake", prompt: "Two racing skiffs trading places through a neon canyon, motion blur on the walls, magenta and cyan trails crossing, cinematic sci-fi realism." },
      { slug: "seedance-2-0-fast-w3", kind: "video", label: "Underpass", prompt: "A racing skiff dropping under a floating platform, shadow then neon light washing over the hull, fast camera follow, cinematic sci-fi realism." },
      { slug: "seedance-2-0-fast-w4", kind: "video", label: "Cockpit view", prompt: "Cockpit view from a racing skiff threading a neon canyon at speed, instrument glow in the foreground, walls blurring past, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "seedance-2-0-fast-l1", kind: "video", label: "High-speed chase", prompt: "A racing skiff banking hard through a neon canyon between floating platforms, magenta and cyan light trails streaking past, heavy motion blur on the canyon walls, chase camera close behind, cinematic sci-fi realism." },
      { slug: "seedance-2-0-fast-l2", kind: "video", label: "Rain race", prompt: "A racing skiff cutting through a neon canyon in heavy rain, spray fanning off the hull, reflections doubling the light trails, cinematic sci-fi realism." },
      { slug: "seedance-2-0-fast-l3", kind: "video", label: "Slow-motion bank", prompt: "Slow motion as a racing skiff banks around a neon pylon, hull plates flexing, light trails smearing, cinematic sci-fi realism." },
      { slug: "seedance-2-0-fast-l4", kind: "video", label: "Finish gate", prompt: "A racing skiff crossing a lit finish gate, magenta light flaring across the lens, crowd platforms blurred behind, cinematic sci-fi realism." },
    ],
  },
  "seedance-2-0-mini": {
    coverSlug: "seedance-2-0-mini",
    workbench: [
      { slug: "seedance-2-0-mini-w1", kind: "video", label: "Drone hover", prompt: "A small reconnaissance drone hovering in a shaft of light inside an abandoned station corridor, sensor ring glowing violet, dust drifting through the beam, cinematic sci-fi realism." },
      { slug: "seedance-2-0-mini-w2", kind: "video", label: "Corridor advance", prompt: "A reconnaissance drone advancing down a dark station corridor, its lamp sweeping the walls, debris on the floor, cinematic sci-fi realism." },
      { slug: "seedance-2-0-mini-w3", kind: "video", label: "Sensor spin", prompt: "Close shot of a reconnaissance drone rotating its sensor ring, violet light sweeping across its shell, dust suspended around it, cinematic sci-fi realism." },
      { slug: "seedance-2-0-mini-w4", kind: "video", label: "Doorway pass", prompt: "A reconnaissance drone slipping through a half-open bulkhead door, light spilling from the room beyond, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "seedance-2-0-mini-l1", kind: "video", label: "Close-up subject", prompt: "A small reconnaissance drone hovering in a shaft of light inside an abandoned station corridor, its sensor ring glowing violet, dust drifting through the beam, the drone rotating slowly to scan, cinematic sci-fi realism." },
      { slug: "seedance-2-0-mini-l2", kind: "video", label: "Dust disturbance", prompt: "A reconnaissance drone's downwash stirring settled dust off a station floor, particles swirling up into its light, cinematic sci-fi realism." },
      { slug: "seedance-2-0-mini-l3", kind: "video", label: "Reflected shell", prompt: "A reconnaissance drone passing a cracked mirror panel in a station corridor, its violet glow doubled in the reflection, cinematic sci-fi realism." },
      { slug: "seedance-2-0-mini-l4", kind: "video", label: "Power down", prompt: "A reconnaissance drone settling to a station floor, its sensor ring dimming, dust resettling around it, cinematic sci-fi realism." },
    ],
  },
  "minimax-h3": {
    coverSlug: "minimax-h3",
    workbench: [
      { slug: "minimax-h3-w1", kind: "video", label: "Reactor catwalk", prompt: "A technician walking a catwalk across a crimson-lit reactor hall, containment core pulsing behind them, slow crane shot rising, cinematic sci-fi realism." },
      { slug: "minimax-h3-w2", kind: "video", label: "Core pulse", prompt: "The spherical containment core of a reactor pulsing crimson, light washing across the surrounding gantries in rhythm, cinematic sci-fi realism." },
      { slug: "minimax-h3-w3", kind: "video", label: "Gantry descent", prompt: "Camera descending past three levels of reactor gantries, crimson light strobing between the decks, cinematic sci-fi realism." },
      { slug: "minimax-h3-w4", kind: "video", label: "Control mezzanine", prompt: "A crimson-lit control mezzanine overlooking a reactor hall, operators at consoles, warning lights sweeping, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "minimax-h3-l1", kind: "video", label: "Scale and atmosphere", prompt: "A technician walking a catwalk across a crimson-lit reactor hall, the spherical containment core pulsing behind them, red volumetric light through the gantries, slow crane shot rising, cinematic sci-fi realism." },
      { slug: "minimax-h3-l2", kind: "video", label: "Alarm state", prompt: "A reactor hall shifting to alarm state, crimson strobes sweeping the gantries, steam venting from floor ports, cinematic sci-fi realism." },
      { slug: "minimax-h3-l3", kind: "video", label: "Core close", prompt: "Slow push toward a reactor containment sphere, its surface panels glowing along the seams, heat distortion rising, cinematic sci-fi realism." },
      { slug: "minimax-h3-l4", kind: "video", label: "Silhouette walk", prompt: "A lone figure silhouetted against a pulsing reactor core, walking away from camera down a long catwalk, cinematic sci-fi realism." },
    ],
  },
  "grok-imagine-video": {
    coverSlug: "grok-imagine-video",
    workbench: [
      { slug: "grok-imagine-video-w1", kind: "video", label: "Spire crane", prompt: "Camera craning up a monolithic black spire emerging from a dawn fog bank, thin white light seams tracing its edges, cold monochrome palette, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-w2", kind: "video", label: "Fog roll", prompt: "Dense fog rolling past the base of a black spire at dawn, its surface catching thin white light, slow static shot, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-w3", kind: "video", label: "Seam detail", prompt: "Close tracking along a glowing white seam in black monolithic stone, fog drifting across the frame, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-w4", kind: "video", label: "Aerial orbit", prompt: "Aerial orbit around the top of a monolithic black spire above a fog sea at dawn, cold monochrome palette, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "grok-imagine-video-l1", kind: "video", label: "Architectural reveal", prompt: "Camera craning up a monolithic black spire emerging from a dawn fog bank, thin white light seams tracing its edges, fog rolling past the base, cold monochrome palette, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-l2", kind: "video", label: "Storm light", prompt: "A black spire under storm light, white seams flaring as lightning passes behind it, fog torn by wind, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-l3", kind: "video", label: "Ground approach", prompt: "Walking approach toward the base of a black spire, its scale growing in frame, fog thinning, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-l4", kind: "video", label: "Night seams", prompt: "A monolithic black spire at night, its light seams the only illumination, stars faint above, slow vertical tilt, cinematic sci-fi realism." },
    ],
  },
  "grok-imagine-video-1-5": {
    coverSlug: "grok-imagine-video-1-5",
    workbench: [
      { slug: "grok-imagine-video-1-5-w1", kind: "video", label: "Hangar dolly", prompt: "A sleek matte-black craft on its cradle in a night hangar, cool white floods raking across the fuselage as the camera dollies along it, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-1-5-w2", kind: "video", label: "Canopy open", prompt: "The canopy of a matte-black craft rising open in a night hangar, interior lights coming up, steam venting from the grating, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-1-5-w3", kind: "video", label: "Panel detail", prompt: "Close tracking across the panel lines of a matte-black fuselage, cool floods sliding over the surface, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-1-5-w4", kind: "video", label: "Crew walkaround", prompt: "A crew member walking a preflight circuit around a matte-black craft in a night hangar, torch beam crossing the hull, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "grok-imagine-video-1-5-l1", kind: "video", label: "Hard-surface detail", prompt: "A sleek matte-black craft on its cradle in a night hangar, cool white floods raking slowly across the fuselage as the camera dollies along it, steam venting from the floor grating, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-1-5-l2", kind: "video", label: "Bay doors", prompt: "Hangar bay doors parting in front of a matte-black craft, night sky and runway lights beyond, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-1-5-l3", kind: "video", label: "Reflected floods", prompt: "A matte-black craft mirrored in wet hangar decking, floods doubling in the reflection, slow lateral track, cinematic sci-fi realism." },
      { slug: "grok-imagine-video-1-5-l4", kind: "video", label: "Engine spool", prompt: "The engine of a matte-black craft spooling up in a hangar, intake vortex forming, heat haze behind the nozzle, cinematic sci-fi realism." },
    ],
  },
  "veo-3-1-generate-preview": {
    coverSlug: "veo-3-1-generate-preview",
    workbench: [
      { slug: "veo-3-1-generate-preview-w1", kind: "video", label: "Array aerial", prompt: "A terraforming array on a red desert plateau at golden hour, condenser towers venting white vapour, slow aerial push toward the towers, cinematic sci-fi realism." },
      { slug: "veo-3-1-generate-preview-w2", kind: "video", label: "Vapour drift", prompt: "White vapour drifting off terraforming condensers across a red plateau, golden light behind it, static wide shot, cinematic sci-fi realism." },
      { slug: "veo-3-1-generate-preview-w3", kind: "video", label: "Tower base", prompt: "Camera tracking past the base of a terraforming tower, red dust blowing across its footings, golden hour light, cinematic sci-fi realism." },
      { slug: "veo-3-1-generate-preview-w4", kind: "video", label: "Shadow lengthen", prompt: "Time passing over a terraforming array as shadows lengthen across red sand, vapour catching the last light, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "veo-3-1-generate-preview-l1", kind: "video", label: "Golden-hour aerial", prompt: "A terraforming array on a red desert plateau at golden hour, condenser towers venting white vapour that drifts across the frame, long shadows over the sand, slow aerial push toward the towers, cinematic sci-fi realism." },
      { slug: "veo-3-1-generate-preview-l2", kind: "video", label: "Dust devil", prompt: "A dust devil crossing a terraforming array's field, vapour and red dust mixing, camera holding steady, cinematic sci-fi realism." },
      { slug: "veo-3-1-generate-preview-l3", kind: "video", label: "Night array", prompt: "A terraforming array at night, condenser towers lit from below, vapour glowing against a star field, cinematic sci-fi realism." },
      { slug: "veo-3-1-generate-preview-l4", kind: "video", label: "Maintenance flyby", prompt: "A maintenance craft flying low past terraforming towers at golden hour, its shadow racing over the sand, cinematic sci-fi realism." },
    ],
  },
  "veo-3-1-fast-generate-preview": {
    coverSlug: "veo-3-1-fast-generate-preview",
    workbench: [
      { slug: "veo-3-1-fast-generate-preview-w1", kind: "video", label: "Dome drift", prompt: "Inside a bioluminescent greenhouse dome on an ice world, emerald plant light glowing through frosted glass, camera drifting between the planting beds, cinematic sci-fi realism." },
      { slug: "veo-3-1-fast-generate-preview-w2", kind: "video", label: "Aurora overhead", prompt: "An aurora shifting above a greenhouse dome on an ice world, emerald light from within, slow upward tilt, cinematic sci-fi realism." },
      { slug: "veo-3-1-fast-generate-preview-w3", kind: "video", label: "Leaf glow", prompt: "Close shot of bioluminescent leaves pulsing emerald in a greenhouse bed, frost on the glass beyond, cinematic sci-fi realism." },
      { slug: "veo-3-1-fast-generate-preview-w4", kind: "video", label: "Airlock entry", prompt: "An airlock cycling open into a bioluminescent greenhouse, cold vapour spilling in, emerald light beyond, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "veo-3-1-fast-generate-preview-l1", kind: "video", label: "Atmospheric interior", prompt: "Inside a bioluminescent greenhouse dome on an ice world, emerald plant light glowing through frosted glass panels, an aurora shifting overhead, camera drifting between the planting beds, cinematic sci-fi realism." },
      { slug: "veo-3-1-fast-generate-preview-l2", kind: "video", label: "Exterior night", prompt: "A bioluminescent greenhouse dome seen from the ice outside at night, emerald glow through frost, aurora above, cinematic sci-fi realism." },
      { slug: "veo-3-1-fast-generate-preview-l3", kind: "video", label: "Condensation run", prompt: "Condensation running down the inside of a greenhouse dome panel, emerald light refracting through each droplet, cinematic sci-fi realism." },
      { slug: "veo-3-1-fast-generate-preview-l4", kind: "video", label: "Tender at work", prompt: "A botanist moving between glowing planting beds in an ice-world greenhouse, emerald light on their coat, cinematic sci-fi realism." },
    ],
  },
  "gpt-image-2": {
    coverSlug: "gpt-image-2",
    workbench: [
      { slug: "gpt-image-2-w1", kind: "image", label: "Archive hall", prompt: "A vast archive hall of glowing data monoliths receding into darkness, teal light spilling from seams in each slab, polished floor reflections, volumetric haze, cinematic sci-fi realism." },
      { slug: "gpt-image-2-w2", kind: "image", label: "Terminal alcove", prompt: "A single curved terminal alcove inside a data vault, teal glyphs scrolling across a concave screen, an empty operator chair, cold rim light, cinematic sci-fi realism." },
      { slug: "gpt-image-2-w3", kind: "image", label: "Cable underlevel", prompt: "The underlevel of a server cathedral, thick bundled cables sweeping overhead like roots, teal service lamps at intervals, standing water on the floor, cinematic sci-fi realism." },
      { slug: "gpt-image-2-w4", kind: "image", label: "Index chamber", prompt: "A spherical index chamber lined with rotating data rings, a technician on a catwalk at its equator, teal and white light, extreme scale, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "gpt-image-2-l1", kind: "image", label: "Scale and depth", prompt: "A vast archive hall of glowing data monoliths receding into darkness, teal light spilling from seams in each slab, a single figure walking between them for scale, polished floor reflections, volumetric haze, cinematic sci-fi realism." },
      { slug: "gpt-image-2-l2", kind: "image", label: "Reflected light", prompt: "A flooded server floor after a coolant leak, monoliths mirrored in still water, teal glow doubled in the reflection, silent and abandoned, cinematic sci-fi realism." },
      { slug: "gpt-image-2-l3", kind: "image", label: "Close detail", prompt: "Macro view of a data monolith's surface, etched circuitry glowing teal beneath frosted glass, condensation beading along the seam, shallow depth of field, cinematic sci-fi realism." },
      { slug: "gpt-image-2-l4", kind: "image", label: "Overhead geometry", prompt: "Overhead view down a data vault's central aisle, monoliths in strict rows forming a receding grid, one lit differently from the rest, teal palette, cinematic sci-fi realism." },
    ],
  },
  "gemini-3-pro-image": {
    coverSlug: "gemini-3-pro-image",
    workbench: [
      { slug: "gemini-3-pro-image-w1", kind: "image", label: "Cliff observatory", prompt: "An observatory carved into a cliff face above a cloud sea at dawn, a great lens array tilted at the sky, blue and gold light, cinematic sci-fi realism." },
      { slug: "gemini-3-pro-image-w2", kind: "image", label: "Lens assembly", prompt: "The segmented primary lens of a cliff observatory seen head on, each panel catching a different band of dawn light, brass framework, cinematic sci-fi realism." },
      { slug: "gemini-3-pro-image-w3", kind: "image", label: "Control gallery", prompt: "The control gallery of a cliff observatory, curved glass overlooking a cloud sea, analogue dials and warm lamp light inside, dawn breaking outside, cinematic sci-fi realism." },
      { slug: "gemini-3-pro-image-w4", kind: "image", label: "Approach stair", prompt: "A narrow stair cut into a cliff leading up to an observatory door, cloud spilling over the steps, cold blue shadow and a single warm lamp, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "gemini-3-pro-image-l1", kind: "image", label: "Detail and light", prompt: "An observatory carved into a cliff face above a cloud sea at dawn, its great lens array tilted toward the sky, blue and gold light breaking over the rim, birds crossing far below, cinematic sci-fi realism, ultra detailed." },
      { slug: "gemini-3-pro-image-l2", kind: "image", label: "Night operation", prompt: "A cliff observatory at night, its lens array open to a dense star field, warm interior light spilling from the dome seam, cloud sea black below, cinematic sci-fi realism." },
      { slug: "gemini-3-pro-image-l3", kind: "image", label: "Weather front", prompt: "A cliff observatory as a storm front arrives, cloud sea churning below the rim, the lens array closing, dramatic side light, cinematic sci-fi realism." },
      { slug: "gemini-3-pro-image-l4", kind: "image", label: "Instrument macro", prompt: "Macro of an observatory's brass focus mechanism, precisely machined gear teeth, dawn light raking across the metal, shallow depth of field, cinematic sci-fi realism." },
    ],
  },
  "gemini-2-5-flash-image": {
    coverSlug: "gemini-2-5-flash-image",
    workbench: [
      { slug: "gemini-2-5-flash-image-w1", kind: "image", label: "Sky platform", prompt: "A transit platform suspended between sky towers at blue hour, cool blue guide lights along the edge, distant traffic streams, cinematic sci-fi realism." },
      { slug: "gemini-2-5-flash-image-w2", kind: "image", label: "Tower canyon", prompt: "Looking up a canyon between two sky towers at blue hour, transit lines crossing overhead, rain beginning, cool blue palette, cinematic sci-fi realism." },
      { slug: "gemini-2-5-flash-image-w3", kind: "image", label: "Platform interior", prompt: "The waiting hall of a sky transit platform, curved benches and glass walls, blue hour light outside, one traveller with luggage, cinematic sci-fi realism." },
      { slug: "gemini-2-5-flash-image-w4", kind: "image", label: "Arrival lights", prompt: "A transit car arriving at a sky platform, its headlights flaring across wet decking, blue hour city beyond, motion blur on the car, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "gemini-2-5-flash-image-l1", kind: "image", label: "Blue-hour cityscape", prompt: "A transit platform suspended between sky towers at blue hour, cool guide lights running along its edge, streams of distant traffic threading between the towers, one waiting figure in silhouette, cinematic sci-fi realism." },
      { slug: "gemini-2-5-flash-image-l2", kind: "image", label: "Rain on glass", prompt: "A sky platform seen through a rain-streaked glass wall at blue hour, city lights refracted into soft discs, a silhouette close to the glass, cinematic sci-fi realism." },
      { slug: "gemini-2-5-flash-image-l3", kind: "image", label: "Aerial grid", prompt: "Aerial view of a sky city at blue hour, transit platforms strung between towers like a lit lattice, traffic threading below, cinematic sci-fi realism." },
      { slug: "gemini-2-5-flash-image-l4", kind: "image", label: "Edge detail", prompt: "The edge of a sky transit platform, blue guide lights embedded in wet decking, a drop into city haze beyond the rail, shallow depth of field, cinematic sci-fi realism." },
    ],
  },
  "gemini-3-1-flash-image": {
    coverSlug: "gemini-3-1-flash-image",
    workbench: [
      { slug: "gemini-3-1-flash-image-w1", kind: "image", label: "Ice shelf station", prompt: "A subsurface ocean station under an ice shelf, floodlights cutting through dark water, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-image-w2", kind: "image", label: "Moon pool", prompt: "The moon pool of an under-ice station, black water lit from beneath, a submersible on its cradle above, cold interior light, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-image-w3", kind: "image", label: "Ice ceiling", prompt: "Looking up at the underside of an ice shelf from deep water, station floodlights illuminating the blue-white ceiling, particulate drifting, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-image-w4", kind: "image", label: "Submersible bay", prompt: "A submersible bay inside an under-ice station, the craft dripping seawater, technicians in cold-weather gear, harsh work lighting, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "gemini-3-1-flash-image-l1", kind: "image", label: "Underwater atmosphere", prompt: "A subsurface ocean station under an ice shelf, floodlights cutting cones through dark water, a vast silhouette passing beyond the light, particulate drifting in the beams, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-image-l2", kind: "image", label: "Beam and particulate", prompt: "A single floodlight beam in deep water under ice, dense particulate suspended in the cone, darkness pressing in at the edges, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-image-l3", kind: "image", label: "Hull frost", prompt: "Macro of an under-ice station's hull, frost crystals spreading across riveted steel, a porthole glowing warm from within, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-image-l4", kind: "image", label: "Descent view", prompt: "View from a descending submersible, the under-ice station's lights appearing below as scattered points in black water, cinematic sci-fi realism." },
    ],
  },
  "gemini-3-1-flash-lite-image": {
    coverSlug: "gemini-3-1-flash-lite-image",
    workbench: [
      { slug: "gemini-3-1-flash-lite-image-w1", kind: "image", label: "Ridge and giant", prompt: "A single explorer standing on a wind-carved ridge facing a ringed gas giant at dusk, thin atmosphere, pale cyan light, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-lite-image-w2", kind: "image", label: "Ring shadow", prompt: "A wind-carved plain under a ringed gas giant, the ring casting a hard shadow band across the ground, pale cyan light, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-lite-image-w3", kind: "image", label: "Camp at dusk", prompt: "A small survey camp on a rocky plain beneath a ringed gas giant, one lit tent, equipment cases, cyan dusk light, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-lite-image-w4", kind: "image", label: "Rock formation", prompt: "Towering wind-carved rock spires under a ringed gas giant, dust streaming between them, pale cyan and grey palette, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "gemini-3-1-flash-lite-image-l1", kind: "image", label: "Lone figure, big sky", prompt: "A single explorer on a wind-carved ridge facing a ringed gas giant low on the horizon at dusk, thin atmosphere, pale cyan light across the rock, dust streaming off the ridge line, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-lite-image-l2", kind: "image", label: "Long shadow", prompt: "An explorer's long shadow thrown across pale rock by a ringed gas giant near the horizon, footprints trailing behind, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-lite-image-l3", kind: "image", label: "Suit detail", prompt: "Close view of a survey suit's shoulder and chest panel, dust in the fabric weave, a ringed gas giant reflected in the visor edge, cinematic sci-fi realism." },
      { slug: "gemini-3-1-flash-lite-image-l4", kind: "image", label: "Night ring", prompt: "A ringed gas giant at full night over a dark plain, the ring a bright arc across the sky, faint cyan light on the rocks below, cinematic sci-fi realism." },
    ],
  },
  "grok-imagine-image": {
    coverSlug: "grok-imagine-image",
    workbench: [
      { slug: "grok-imagine-image-w1", kind: "image", label: "Derelict hull", prompt: "A derelict generation ship drifting against a starfield, hull breached and ribbed with structure, cold rim light from a distant sun, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-w2", kind: "image", label: "Breach interior", prompt: "Inside the breached hull of a derelict ship, structural ribs open to vacuum, starlight cutting through the gap, debris hanging motionless, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-w3", kind: "image", label: "Bridge remains", prompt: "The abandoned bridge of a derelict generation ship, consoles dark and frosted, a viewport open to stars, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-w4", kind: "image", label: "Debris field", prompt: "A debris field around a derelict ship, fragments of hull plating tumbling slowly, cold rim light from a distant sun, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "grok-imagine-image-l1", kind: "image", label: "Deep-space still", prompt: "A derelict generation ship drifting against a dense starfield, its breached hull showing ribbed internal structure, cold rim light from a distant sun, debris suspended around it, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-l2", kind: "image", label: "Silhouette pass", prompt: "A derelict ship silhouetted against a bright nebula, its broken outline black against the glow, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-l3", kind: "image", label: "Hull macro", prompt: "Macro of a derelict ship's hull plating, micrometeorite pitting and faded registry markings, hard vacuum light, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-l4", kind: "image", label: "Approach angle", prompt: "A survey craft approaching a derelict generation ship, tiny against the wreck's scale, its running lights the only warm colour, cinematic sci-fi realism." },
    ],
  },
  "grok-imagine-image-pro": {
    coverSlug: "grok-imagine-image-pro",
    workbench: [
      { slug: "grok-imagine-image-pro-w1", kind: "image", label: "Helmet portrait", prompt: "A close portrait of an exploration helmet, visor reflecting a red planet horizon, scuffed white composite shell, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-pro-w2", kind: "image", label: "Visor reflection", prompt: "Extreme close-up of an exploration visor, a red planet's horizon and a distant rover mirrored in the curved glass, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-pro-w3", kind: "image", label: "Suit seals", prompt: "Close view of an exploration suit's neck seal and locking ring, red dust worked into every groove, hard sunlight, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-pro-w4", kind: "image", label: "Helmet on bench", prompt: "An exploration helmet resting on a workshop bench, red dust on the shell, tools and a service light beside it, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "grok-imagine-image-pro-l1", kind: "image", label: "Portrait and reflection", prompt: "Close portrait of an exploration helmet, its visor reflecting a red planet horizon and the figure's own gloved hand, scuffed white composite shell, dust caught in the seals, cinematic sci-fi realism, ultra detailed." },
      { slug: "grok-imagine-image-pro-l2", kind: "image", label: "Backlit rim", prompt: "An explorer's helmet backlit by a low red sun, rim light tracing the shell's edge, face in shadow behind the visor, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-pro-l3", kind: "image", label: "Two figures", prompt: "Two explorers in white suits facing each other on a red plain, helmets reflecting one another, long shadows, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-pro-l4", kind: "image", label: "Dust and scratch", prompt: "Macro of a scratched helmet visor, red dust in the scratches, sunlight scattering across the damage, shallow depth of field, cinematic sci-fi realism." },
    ],
  },
  "grok-imagine-image-quality": {
    coverSlug: "grok-imagine-image-quality",
    workbench: [
      { slug: "grok-imagine-image-quality-w1", kind: "image", label: "Engine cutaway", prompt: "A cutaway of a starship engine bay as a technical illustration, copper coils and cooling fins, warm amber glow against gunmetal, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-quality-w2", kind: "image", label: "Coil array", prompt: "A dense array of copper induction coils inside an engine bay, amber light between the windings, precise machining, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-quality-w3", kind: "image", label: "Coolant lines", prompt: "Coolant lines running along an engine bay ceiling, condensation beading and dripping, amber warning lamps, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-quality-w4", kind: "image", label: "Service hatch", prompt: "An open service hatch on a starship engine, an engineer's tools laid out on a mat, amber interior glow, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "grok-imagine-image-quality-l1", kind: "image", label: "Mechanical detail", prompt: "A starship engine bay in cutaway, copper induction coils and cooling fins in precise detail, warm amber glow against gunmetal plating, condensation on the coolant lines, cinematic sci-fi realism, ultra detailed." },
      { slug: "grok-imagine-image-quality-l2", kind: "image", label: "Thrust bell", prompt: "The interior of a thrust bell seen from below, regenerative cooling channels spiralling up its throat, scorched metal, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-quality-l3", kind: "image", label: "Diagnostic glow", prompt: "An engine diagnostic panel lit amber in a dark bay, reflections in polished deck plating, cinematic sci-fi realism." },
      { slug: "grok-imagine-image-quality-l4", kind: "image", label: "Assembly scale", prompt: "An engine assembly suspended in a gantry, engineers dwarfed beneath it, work lights raking across the machinery, cinematic sci-fi realism." },
    ],
  },
  "nano-banana-pro-preview": {
    coverSlug: "nano-banana-pro-preview",
    workbench: [
      { slug: "nano-banana-pro-preview-w1", kind: "image", label: "Relay outpost", prompt: "A desert relay outpost at sunset, amber dish arrays tracking the sky, heat shimmer over the sand, long shadows, cinematic sci-fi realism." },
      { slug: "nano-banana-pro-preview-w2", kind: "image", label: "Dish array", prompt: "A cluster of relay dishes against a sunset sky, their surfaces catching amber light, support trusses in silhouette, cinematic sci-fi realism." },
      { slug: "nano-banana-pro-preview-w3", kind: "image", label: "Service vehicle", prompt: "A dust-covered service vehicle parked beside a desert relay mast at sunset, tracks trailing behind it across the sand, cinematic sci-fi realism." },
      { slug: "nano-banana-pro-preview-w4", kind: "image", label: "Control shack", prompt: "The interior of a desert relay control shack at sunset, amber light through slatted blinds, worn instrument panels, cinematic sci-fi realism." },
    ],
    library: [
      { slug: "nano-banana-pro-preview-l1", kind: "image", label: "Warm desert light", prompt: "A desert relay outpost at sunset, amber dish arrays tracking across the sky, heat shimmer rippling over the sand, long shadows from the mast structures, a service vehicle parked in the foreground, cinematic sci-fi realism." },
      { slug: "nano-banana-pro-preview-l2", kind: "image", label: "Dust storm edge", prompt: "A desert relay outpost as a dust wall approaches, dishes turning to stow, amber light going brown, cinematic sci-fi realism." },
      { slug: "nano-banana-pro-preview-l3", kind: "image", label: "Night watch", prompt: "A desert relay outpost under a dense night sky, dish arrays lit by amber ground lamps, the milky way overhead, cinematic sci-fi realism." },
      { slug: "nano-banana-pro-preview-l4", kind: "image", label: "Mast climb", prompt: "A technician climbing a relay mast at sunset, amber light on the ladder rungs, the desert flat far below, cinematic sci-fi realism." },
    ],
  },
};

/**
 * seedance-2.5 keeps its original hand-curated assets.
 *
 * Its workbench examples (model-examples/) and prompt library
 * (model-showcase/) are real Seedance generations produced for that page, each
 * with the reference image and parameter set behind it. The generated media in
 * this table cannot match that -- the video samples here were rendered by a
 * different model, since seedance is not reachable on this account's channel
 * group -- so the page is left exactly as it was.
 */
const KEEPS_ORIGINAL_ASSETS = new Set(["seedance-2-5"]);

export function getModelMedia(modelId: string): ModelMedia | null {
  const slug = modelMediaSlug(modelId);
  if (KEEPS_ORIGINAL_ASSETS.has(slug)) return null;
  const media = MODEL_MEDIA[slug];
  if (media) return normalizeModelMedia(media);

  // The catalog is larger than the set of generated samples. Reuse the
  // existing, already-uploaded representative assets instead of generating a
  // new image/video for every newly-added model. The model page still keeps
  // the correct modality and prompt-library shape; only the visual sample is
  // shared.
  const isVideo = /(^|-)(video|seedance|kling|sora|veo|wan|minimax-h3)(-|$)/.test(slug);
  const isImage = /(^|-)(image|imagen|flux|banana|dall-e|gpt-image)(-|$)/.test(slug);
  if (!isVideo && !isImage) return null;
  const shared: ModelMedia = isVideo
    ? {
        coverSlug: "seedance-2-0",
        workbench: [{ slug: "seedance-2-0-w1", kind: "video" as const, label: "Walker crossing" as ModelLandingKey, prompt: "A survey walker crossing a violet salt flat at dusk, twin moons low on the horizon, dust curling off each footfall, slow tracking shot from the side, cinematic sci-fi realism." }],
        library: [
          { slug: "seedance-2-0-l1", kind: "video" as const, label: "Wide landscape motion" as ModelLandingKey, prompt: "A survey walker crossing a violet salt flat at dusk, twin moons low on the horizon, dust curling off each footfall, indigo and amber palette, cinematic sci-fi realism." },
          { slug: "seedance-2-0-l2", kind: "video" as const, label: "Night crossing" as ModelLandingKey, prompt: "A survey walker moving across a dark salt flat at night, its running lights the only illumination, stars dense overhead, slow lateral tracking, cinematic sci-fi realism." },
          { slug: "seedance-2-0-l3", kind: "video" as const, label: "Storm approach" as ModelLandingKey, prompt: "A survey walker halting as a dust wall approaches across a salt flat, light going brown, camera holding wide, cinematic sci-fi realism." },
          { slug: "seedance-2-0-l4", kind: "video" as const, label: "Reflection crossing" as ModelLandingKey, prompt: "A survey walker crossing a thin layer of standing water on a salt flat, its shape mirrored below, twin moons reflected, cinematic sci-fi realism." },
        ],
      }
    : {
        coverSlug: "gpt-image-2",
        workbench: [{ slug: "gpt-image-2-w1", kind: "image" as const, label: "Archive hall" as ModelLandingKey, prompt: "A vast archive hall of glowing data monoliths receding into darkness, teal light spilling from seams in each slab, polished floor reflections, volumetric haze, cinematic sci-fi realism." }],
        library: [
          { slug: "gpt-image-2-l1", kind: "image" as const, label: "Scale and depth" as ModelLandingKey, prompt: "A vast archive hall of glowing data monoliths receding into darkness, teal light spilling from seams in each slab, a single figure walking between them for scale, polished floor reflections, volumetric haze, cinematic sci-fi realism." },
          { slug: "gpt-image-2-l2", kind: "image" as const, label: "Reflected light" as ModelLandingKey, prompt: "A flooded server floor after a coolant leak, monoliths mirrored in still water, teal glow doubled in the reflection, silent and abandoned, cinematic sci-fi realism." },
          { slug: "gpt-image-2-l3", kind: "image" as const, label: "Close detail" as ModelLandingKey, prompt: "Macro view of a data monolith's surface, etched circuitry glowing teal beneath frosted glass, condensation beading along the seam, shallow depth of field, cinematic sci-fi realism." },
          { slug: "gpt-image-2-l4", kind: "image" as const, label: "Overhead geometry" as ModelLandingKey, prompt: "Overhead view down a data vault's central aisle, monoliths in strict rows forming a receding grid, one lit differently from the rest, teal palette, cinematic sci-fi realism." },
        ],
      };
  return normalizeModelMedia(shared);
}

/**
 * Cover URL for a model's social card, or undefined when the model has no
 * cover -- callers fall back to the site-wide OG image rather than linking a
 * 404.
 */
export function modelCoverImage(modelId: string): string | undefined {
  const media = getModelMedia(modelId);
  return media ? modelCoverUrl(media.coverSlug) : undefined;
}
