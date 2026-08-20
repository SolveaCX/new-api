"use client";

import { useEffect, useMemo, useRef, useState, type ChangeEvent, type MouseEvent, type ReactNode } from "react";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  Bot,
  Braces,
  ChevronDown,
  ChevronRight,
  Code2,
  Copy,
  DollarSign,
  FileAudio,
  FileText,
  FileVideo,
  Gauge,
  ImageIcon,
  KeyRound,
  Layers3,
  Music2,
  Play,
  Settings2,
  ShieldCheck,
  Sparkles,
  Terminal,
  TerminalSquare,
  Check,
  Timer,
  Trash2,
  X,
  Upload,
  Video,
  Volume2,
  VolumeX,
  WandSparkles,
  Zap,
} from "lucide-react";
import { createPortal } from "react-dom";
import Image from "next/image";
import Link from "next/link";
import { buildDirectoryHealthTrend } from "@/components/models-directory-table";
import { formatHealthSuccessRate, getJitteredSuccessRate } from "@/lib/health-display";
import { HomeModelLogo } from "@/components/home-model-logo";
import {
  fetchHealthSummary,
  fetchModelTrend,
  formatCallCount,
  formatLatencyMs,
  formatSuccessRate,
  formatThroughput,
  TOKEN_DISPLAY_SCALE,
  trendAvgTtftMs,
  type HomePerfSummary,
  type HomeTrendPoint,
} from "@/lib/home-live";
import { SiteShell } from "@/components/site-shell";
import { localizePath, type Locale } from "@/lib/locales";
import {
  modelLandingCopy,
  getModelLandingConfigs,
  normalizeModelId,
  type ModelConfig,
  type ModelGeneratorField,
  type ModelLandingKey,
} from "@/lib/model-landing";
import { consoleUrl, SITE_ORIGIN } from "@/lib/origins";
import {
  formatGroupRequestPrice,
  formatGroupTokenPrice,
  formatModelPrice,
  discountedPriceUsd,
  formatUsdPrice,
  getAvailableGroups,
  getBestGroupRatio,
  getModelFamilyKey,
  getOfficialPriceUsd,
  isTokenBasedModel,
  resolveModelDisplayPrice,
  type DisplayPricingDimension,
  type PricingModel,
} from "@/lib/pricing";
import {
  buildAgentInstallCommand,
  buildApiSnippet,
  buildSdkSnippet,
  detectAgentPlatform,
  snippetKindForEndpoint,
  CLI_INSTALL_COMMAND,
  CLI_LOGIN_COMMAND,
  type AgentPlatform,
  type SnippetLanguage,
} from "@/lib/model-snippets";
import type { ModelUsage } from "@/lib/model-usage";
import type { RankedModel, RankingsData } from "@/lib/rankings-live";
import { buildModelSchema, stringifyJsonLd } from "@/lib/schema";

type Props = {
  config: ModelConfig;
  locale: Locale;
  liveModels?: PricingModel[];
  allModels?: PricingModel[];
  groupRatio?: Record<string, number>;
  rankings?: RankingsData | null;
  /** Daily call counts, resolved server-side; null hides the Activity section. */
  usage?: ModelUsage | null;
};

type GtagWindow = Window & {
  gtag?: (...args: unknown[]) => void;
};

type DraftValue = Record<string, unknown>;

type MediaExample = {
  poster: string;
  video?: string;
  // Selecting an example loads the exact request behind it into the editor, so
  // the workbench doubles as a request blueprint rather than a static gallery.
  label?: ModelLandingKey;
  prompt?: string;
  fields?: Record<string, string | number | boolean>;
  // Reference assets the example used. These load into the workbench's own
  // reference list on selection, so the editor shows the complete request --
  // not just its prompt and parameters.
  references?: ReadonlyArray<{ kind: "image" | "video" | "audio"; name: string; url?: string }>;
};

type ReferenceImageDraft = {
  id: string;
  name: string;
  size: number;
  type: string;
  previewUrl: string;
  // Set for assets that came from a selected example rather than a file the
  // visitor picked. They are swapped out when the example changes, and their
  // previewUrl is a static path, so it must not be passed to revokeObjectURL.
  fromExample?: boolean;
};

type RelatedModelCard = {
  href: string;
  name: string;
  vendor: string;
  kind: string;
  price: string;
  sameProvider: boolean;
};

type CatalogRelatedModel = {
  href: string;
  name: string;
  description: string;
  sameProvider: boolean;
};

type FlatkeyPriceTableRow = {
  label: string;
  flatkey: string;
  official: string;
  flatkeyPercent: number;
  officialPercent: number;
};

const MAX_REFERENCE_MEDIA_FILES = 10;

// Mirrors models-directory-table.tsx so a model reads the same health on
// its detail page as it does in the directory listing.
const DEFAULT_HEALTH_SUCCESS_RATE = 100;
const DEFAULT_HEALTH_TTFT_MS = 600;

const MEDIA_EXAMPLES: Record<"image" | "video" | "audio", readonly MediaExample[]> = {
  image: [
    { poster: "/assets/prompts/awesome-images/gpt-image-2-showcase-complex.png" },
    { poster: "/assets/prompts/awesome-images/ecommerce-skincare.png" },
    { poster: "/assets/prompts/awesome-images/ugc-coffee-ad.png" },
  ],
  video: [
    {
      // Real Seedance output, each paired with the prompt and reference that
      // produced it -- the workbench is a request blueprint, so every sample has
      // to be a request that actually ran. These are deliberately different
      // scenes from the showcase below, so the page shows eight distinct
      // generations rather than five shown twice.
      poster: "/assets/model-examples/seedance-f1-wet-track.png",
      video: "/assets/model-examples/seedance-f1-wet-track.mp4",
      label: "Wet-track chase shot",
      prompt:
        "一辆黑银色的方程式赛车在湿滑的森林赛道上高速疾驰，镜头采用低机位斜后方跟拍视角，镜头捕捉全部车身，车身位于画面左下 2/3 处，赛车从画面中央颜色弯曲的赛道向前冲刺，轮胎压过积水，扬起大量白色水雾和水花，车身在高速运动中轻微抖动，背景是被薄雾笼罩的赛道、远处的松林和看台，镜头焦点跟随车身，大景深，旁边的车道路面都做运动模糊处理，旁边的天空阴天、光线柔和而冷淡，整体色调以蓝灰、雾白、深绿为主，画面有雨后潮湿感、速度感和电影级真实质感，构图强调前景赛车的力量感和赛道纵深，动态模糊明显，超写实，cinematic, high speed racing, wet track, misty atmosphere, rear chase shot, dramatic motion blur, realistic lighting。忽略参考图上的文字，生成的视频上不要出现任务文案，不要车身变形，不要用草坪来岔分多车道，视频不要出现脱帧情况",
      fields: { ratio: "16:9", resolution: "1080p", duration: 6, generate_audio: true },
      references: [
        { kind: "image", name: "f1-wet-track-reference.png", url: "/assets/model-examples/seedance-f1-reference.png" },
      ],
    },
    {
      poster: "/assets/model-examples/product-macro.png",
      video: "/assets/model-examples/product-macro.mp4",
      label: "Product macro",
      prompt:
        "Slow macro dolly across a matte-black wireless earbud case on a concrete surface, lid opening to reveal the buds, controlled studio key light with a soft rim, dust motes in the beam, shallow depth of field, premium product cinematography, subtle mechanical click.",
      fields: { ratio: "16:9", resolution: "1080p", duration: 6, generate_audio: true },
      references: [
        { kind: "image", name: "product-reference.png", url: "/assets/model-examples/product-macro-reference.png" },
      ],
    },
    {
      poster: "/assets/model-examples/food-motion.png",
      video: "/assets/model-examples/food-motion.mp4",
      label: "Food and beverage",
      prompt:
        "Overhead shot of espresso being poured into a glass of milk over ice, dark coffee blooming through the white in slow motion, condensation on the glass, warm cafe daylight, marble counter, appetising commercial food cinematography.",
      fields: { ratio: "16:9", resolution: "1080p", duration: 6, generate_audio: true },
    },
    {
      poster: "/assets/model-examples/fashion-walk.png",
      video: "/assets/model-examples/fashion-walk.mp4",
      label: "Fashion film",
      prompt:
        "A model in a long camel coat walking toward camera down a wide city street at golden hour, coat moving with the stride, backlit rim light through the fabric, shallow depth of field with compressed background, editorial fashion film look.",
      fields: { ratio: "16:9", resolution: "1080p", duration: 6, generate_audio: true },
    },
  ],
  audio: [
    { poster: "/assets/prompts/awesome-images/ai-agent-poster.png" },
    { poster: "/assets/prompts/awesome-images/liquid-bento.png" },
    { poster: "/assets/prompts/awesome-images/campaign-hero.png" },
  ],
} as const;

// Showcase: what the model produces across use cases, above the FAQ so the last
// thing a reader sees before the questions is the output itself.
//
// Each entry is real generated media plus the prompt behind it. SHOWCASE_SCENES
// is empty until those assets exist -- the section renders nothing rather than
// standing in with placeholder art, because a fabricated "fight scene" still
// implies the model produced it.
type ShowcaseScene = {
  /** Slug used for the asset paths under /assets/model-showcase/. */
  id: string;
  label: ModelLandingKey;
  prompt: string;
};

// To add a scene: drop <id>.mp4 and <id>.png into website/public/assets/
// model-showcase/, add the entry here, and add its `label` to the copy maps in
// lib/model-landing.ts for all 10 locales.
// The capabilities the clips below demonstrate, stated as text. A gallery shows
// what the model did; this says what it can do -- which is what a reader
// comparing models, and a crawler indexing the page, can actually read.
//
// Sourced from the model's published capability set, the same one the page
// description and the version comparison draw on.
const SHOWCASE_CAPABILITIES: ReadonlyArray<{ title: ModelLandingKey; body: ModelLandingKey }> = [
  {
    title: "Text and image to video",
    body: "Generate from a written scene, or drive it with reference images for a subject you have already designed.",
  },
  {
    title: "Multi-shot consistency",
    body: "Hold characters, wardrobe, and setting across cuts within a single generation.",
  },
  {
    title: "Native audio",
    body: "Ambient sound and speech are generated with the picture, in multiple languages.",
  },
  {
    title: "Edit and extend",
    body: "Continue an existing clip or revise one, with first-frame and first/last-frame control.",
  },
];

const SHOWCASE_SCENES: readonly ShowcaseScene[] = [
  {
    id: "racing-chase",
    label: "High-speed action",
    prompt:
      "Low rear-chase shot of a formula car at speed on a wet forest circuit, tyres throwing spray, misty treeline and grandstands behind, heavy motion blur on the surrounding track, overcast light, blue-grey and deep green palette, cinematic realism.",
  },
  {
    id: "coastal-landmark",
    label: "Cinematic landscape",
    prompt:
      "Aerial approach along a clifftop coast road at dusk, lighthouse on the headland, heavy surf breaking against layered rock, gulls crossing frame, soft overcast light, muted blue and ochre palette, slow drifting camera.",
  },
  {
    id: "creature-closeup",
    label: "Character and creature",
    prompt:
      "A giant soft-bodied creature walking down a sunlit city street, pedestrians reacting around it, natural daylight, handheld documentary framing, believable scale and contact shadows, photoreal texture on fur and fabric.",
  },
  {
    id: "romance-scene",
    label: "Character performance",
    prompt:
      "A couple sitting close on a rain-streaked cafe window seat at dusk, warm interior light, she laughs and rests her head on his shoulder, shallow depth of field, film grain, intimate handheld framing, soft ambient room tone.",
  },
  {
    id: "ugc-creator",
    label: "UGC and social",
    prompt:
      "Handheld selfie shot: a young creator in a bright apartment holds the camera at arm's length, talking to it with natural energy, gestures toward a laptop on the desk beside her, warm daylight from a window, slight camera shake, unpolished authentic UGC look.",
  },
];

// Version comparison: what changed from the previous generation. A reader who
// already uses 2.0 needs this to decide whether to move, and it is the one thing
// a general catalog page cannot tell them.
//
// Rows are capability facts, not benchmarks -- the 2.0 family is not in the
// public pricing catalog, so nothing here is derived from live data and none of
// it should be presented as measured.
type VersionCompareRow = {
  label: ModelLandingKey;
  previous: ModelLandingKey;
  current: ModelLandingKey;
  /** Set when the newer generation is the clear improvement on this row. */
  improved?: boolean;
};

const SEEDANCE_VERSION_ROWS: readonly VersionCompareRow[] = [
  {
    label: "Reference inputs",
    previous: "Images only",
    current: "Images, video, and audio — up to 50 per request",
    improved: true,
  },
  {
    label: "Frame control",
    previous: "First frame",
    current: "First frame and first/last frame",
    improved: true,
  },
  {
    label: "Shot consistency",
    previous: "Single shot",
    current: "Multi-shot with character continuity",
    improved: true,
  },
  {
    label: "Audio",
    previous: "Optional generated audio",
    current: "Native audio, multilingual",
    improved: true,
  },
  {
    label: "Editing",
    previous: "Generate only",
    current: "Generate, edit, and extend existing video",
    improved: true,
  },
];

function ModelVersionCompare(props: {
  modelName: string;
  previousName: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  return (
    <RevealSection
      id="versions"
      className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] border-y border-slate-200 bg-[#f8fafc] px-6 py-12 dark:border-white/10 dark:bg-white/[0.02]"
    >
      <div className="mx-auto max-w-6xl">
        <FlatkeySectionHeading
          eyebrow={props.t("Versions")}
          title={props.t("{{model}} compared with {{previous}}", {
            model: props.modelName,
            previous: props.previousName,
          })}
          description={props.t("What changed from the previous generation, so you can tell whether it is worth switching.")}
        />
        <div className="mt-5 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_20px_50px_-42px_rgba(24,14,38,0.4)] dark:border-white/10 dark:bg-white/[0.04]">
          <div className="grid grid-cols-[minmax(120px,0.7fr)_minmax(0,1fr)_minmax(0,1.2fr)] border-b border-slate-200 bg-[#fbfcff] text-[11px] font-bold tracking-[0.08em] text-muted-foreground uppercase dark:border-white/10 dark:bg-white/[0.02]">
            <div className="px-4 py-3">{props.t("Capability")}</div>
            <div className="px-4 py-3">{props.previousName}</div>
            <div className="px-4 py-3 text-blue-700 dark:text-blue-300">{props.modelName}</div>
          </div>
          {SEEDANCE_VERSION_ROWS.map((row) => (
            <div
              key={row.label}
              className="grid grid-cols-[minmax(120px,0.7fr)_minmax(0,1fr)_minmax(0,1.2fr)] border-b border-slate-200 text-[13px] last:border-b-0 dark:border-white/10"
            >
              <div className="px-4 py-3.5 font-semibold text-[#20222a] dark:text-white/88">{props.t(row.label)}</div>
              <div className="px-4 py-3.5 text-muted-foreground">{props.t(row.previous)}</div>
              <div className="flex items-start gap-2 px-4 py-3.5 font-medium text-[#20222a] dark:text-white/88">
                {row.improved ? <Check className="mt-0.5 size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" /> : null}
                <span>{props.t(row.current)}</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </RevealSection>
  );
}

// Why Flatkey: the page has shown what the model does and how to call it, but
// never why to call it through here rather than direct from the vendor. Placed
// after the version comparison, where a reader has decided on the model and the
// open question becomes the gateway.
//
// Claims are tied to what the page already demonstrates -- the live price row,
// the measured uptime section, the catalog of other models -- rather than
// generic platform copy.
function WhyFlatkey(props: {
  modelName: string;
  providerName: string;
  savings: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const reasons = [
    {
      icon: <DollarSign className="size-4" />,
      title: props.savings === "0%"
        ? props.t("Vendor pricing, one balance")
        : props.t("{{percent}} below list price", { percent: props.savings }),
      body: props.t("The same upstream model, billed from one balance that also covers text, image, and audio models."),
    },
    {
      icon: <Code2 className="size-4" />,
      title: props.t("OpenAI-compatible from day one"),
      body: props.t("Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code."),
    },
    {
      icon: <Layers3 className="size-4" />,
      title: props.t("Swap models without a new integration"),
      body: props.t("Move between {{provider}} and every other model in the catalog by changing one string.", {
        provider: props.providerName,
      }),
    },
    {
      icon: <ShieldCheck className="size-4" />,
      title: props.t("Routing across upstream channels"),
      body: props.t("Requests are spread across the channels serving this model, with 30-day uptime published above."),
    },
  ];

  return (
    <RevealSection
      id="why-flatkey"
      className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] bg-white px-6 py-12 dark:bg-white/[0.02]"
    >
      <div className="mx-auto max-w-6xl">
        <FlatkeySectionHeading
          eyebrow={props.t("Why Flatkey")}
          title={props.t("Why run {{model}} through Flatkey", { model: props.modelName })}
          description={props.t("One key, one balance, and the same upstream model you would call directly.")}
        />
        <div className="mt-5 grid gap-3 sm:grid-cols-2">
          {reasons.map((reason) => (
            <div
              key={reason.title}
              className="rounded-xl border border-slate-200 bg-[#fbfcff] p-5 dark:border-white/10 dark:bg-white/[0.03]"
            >
              <span className="grid size-9 place-items-center rounded-lg bg-blue-500/10 text-blue-700 dark:text-blue-300">
                {reason.icon}
              </span>
              <h3 className="mt-3.5 text-[15px] font-semibold text-[#20222a] dark:text-white/90">{reason.title}</h3>
              <p className="mt-1.5 text-sm leading-6 text-muted-foreground">{reason.body}</p>
            </div>
          ))}
        </div>
      </div>
    </RevealSection>
  );
}

function ModelShowcase(props: {
  modelName: string;
  onUseScene: (scene: ShowcaseScene) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const [active, setActive] = useState(0);
  if (SHOWCASE_SCENES.length === 0) return null;
  const scene = SHOWCASE_SCENES[active] ?? SHOWCASE_SCENES[0];

  return (
    <RevealSection
      id="showcase"
      className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] bg-white px-6 py-12 dark:bg-white/[0.02]"
    >
      <div className="mx-auto max-w-6xl">
        <FlatkeySectionHeading
          eyebrow={props.t("Showcase")}
          title={props.t("What {{model}} can do", { model: props.modelName })}
          description={props.t("Real generations across common production use cases, each with the prompt behind it.")}
        />
        <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {SHOWCASE_CAPABILITIES.map((capability) => (
            <div
              key={capability.title}
              className="rounded-xl border border-slate-200 bg-[#fbfcff] p-4 dark:border-white/10 dark:bg-white/[0.03]"
            >
              <h3 className="text-[13px] font-semibold text-[#20222a] dark:text-white/88">{props.t(capability.title)}</h3>
              <p className="mt-1.5 text-[12px] leading-5 text-muted-foreground">{props.t(capability.body)}</p>
            </div>
          ))}
        </div>
        <div className="mt-5 grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,0.6fr)] lg:items-stretch">
          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-[#10131a] shadow-[0_20px_50px_-42px_rgba(24,14,38,0.5)] dark:border-white/10">
            <div className="relative aspect-video">
              <video
                key={scene.id}
                className="h-full w-full object-cover"
                autoPlay
                loop
                muted
                playsInline
                poster={`/assets/model-showcase/${scene.id}.png`}
                preload="metadata"
                src={`/assets/model-showcase/${scene.id}.mp4`}
              />
            </div>
            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-white/10 px-4 py-3.5">
              <div className="min-w-0">
                <div className="text-[13px] font-semibold text-white">{props.t(scene.label)}</div>
                <div className="mt-0.5 text-[11px] text-white/54">
                  {props.t("Generated with {{model}} — load this prompt into the playground and edit it", {
                    model: props.modelName,
                  })}
                </div>
              </div>
              <button
                type="button"
                onClick={() => props.onUseScene(scene)}
                className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-lg bg-white px-3.5 text-[12px] font-semibold text-[#10131a] transition hover:bg-white/88"
              >
                <WandSparkles className="size-3.5" />
                {props.t("Make one like this")}
              </button>
            </div>
          </div>
          {/* Rail height follows the player so the two columns stay level
              whatever the scene count. */}
          <div className="grid gap-2 lg:h-full lg:auto-rows-fr">
            {SHOWCASE_SCENES.map((item, index) => (
              <button
                key={item.id}
                type="button"
                onClick={() => setActive(index)}
                aria-pressed={index === active}
                className={`flex items-center gap-3 rounded-xl border p-3 text-left transition ${
                  index === active
                    ? "border-violet-500/45 bg-violet-500/6 shadow-[0_12px_30px_-24px_rgba(124,58,237,.8)]"
                    : "border-slate-200 bg-white hover:border-violet-500/25 dark:border-white/10 dark:bg-white/[0.04]"
                }`}
              >
                <span className="relative aspect-video w-20 shrink-0 overflow-hidden rounded-lg bg-slate-950">
                  <Image src={`/assets/model-showcase/${item.id}.png`} alt="" fill sizes="80px" className="object-cover" />
                </span>
                <span className="min-w-0">
                  <span className="block text-[13px] font-semibold text-[#20222a] dark:text-white/88">
                    {props.t(item.label)}
                  </span>
                  <span className="mt-1 line-clamp-1 block text-[11px] leading-4 text-muted-foreground" title={item.prompt}>
                    {item.prompt}
                  </span>
                </span>
              </button>
            ))}
          </div>
        </div>
      </div>
    </RevealSection>
  );
}

// Reveals an element the first time it scrolls into view, pairing with the
// .fk-reveal styles.
//
// Starts revealed and only hides once an observer is confirmed available, so
// the server render and any no-JS or observer-less client shows the content
// instead of an empty page. Anything already on screen at mount stays put --
// animating something the reader is already looking at reads as a glitch.
function useRevealOnScroll<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [revealed, setRevealed] = useState(true);

  useEffect(() => {
    const node = ref.current;
    if (!node || typeof IntersectionObserver === "undefined") return;
    if (node.getBoundingClientRect().top < window.innerHeight) return;

    setRevealed(false);
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setRevealed(true);
          observer.disconnect();
        }
      },
      // Fires just before the top edge arrives, so the motion finishes as the
      // section settles rather than starting late.
      { rootMargin: "0px 0px -12% 0px" }
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return { ref, revealed };
}

// Wraps a page section so it fades up on entry. Keeps the section element (and
// its id, used by the tab bar's scroll targets) rather than adding a wrapper
// div, which would break `scroll-margin` anchoring.
function RevealSection(props: { id: string; className: string; children: ReactNode }) {
  const { ref, revealed } = useRevealOnScroll<HTMLElement>();
  return (
    <section id={props.id} ref={ref} className={`${props.className} fk-reveal`} data-revealed={String(revealed)}>
      {props.children}
    </section>
  );
}

// Turns an example's declared references into editor drafts. MIME types are
// synthesized from the declared kind so the existing per-kind grouping, icons,
// and upstream caps apply to them unchanged.
function exampleReferenceDrafts(example: MediaExample | undefined): ReferenceImageDraft[] {
  return (example?.references ?? []).map((reference) => ({
    id: `example:${reference.name}`,
    name: reference.name,
    size: 0,
    type: `${reference.kind}/*`,
    previewUrl: reference.url ?? "",
    fromExample: true,
  }));
}

export function ModelLandingPage({ config, locale, liveModels = [], allModels = [], groupRatio = {}, rankings = null, usage = null }: Props) {
  const [prompt, setPrompt] = useState(config.examplePrompt);
  const [fieldValues, setFieldValues] = useState<Record<string, string | number | boolean>>(() =>
    buildInitialGeneratorValues(config)
  );
  const [referenceImages, setReferenceImages] = useState<ReferenceImageDraft[]>(() =>
    exampleReferenceDrafts(config.generator ? MEDIA_EXAMPLES[config.generator.kind][0] : undefined)
  );
  const [selectedExample, setSelectedExample] = useState(0);
  const generator = config.generator;
  const mediaKind = generator?.kind ?? "text";
  const t = (key: string, vars?: Record<string, string>) => modelLandingCopy(locale, key as ModelLandingKey, vars);
  const primaryLiveModel =
    liveModels.find((model) => normalizeModelId(model.model_name) === normalizeModelId(config.modelId)) ??
    liveModels[0] ??
    null;

  useEffect(() => {
    (window as GtagWindow).gtag?.("event", "flatkey_model_page_view", {
      model: config.slug,
      lng: locale,
    });
  }, [config.slug, locale]);

  const buildDraft = (): DraftValue => {
    const referenceMedia = referenceImages.map(({ name, size, type }) => ({ name, size, type }));
    return {
      source: "model_landing",
      model: config.modelId,
      slug: config.slug,
      mediaKind,
      endpoint: generator?.endpoint ?? "/v1/chat/completions",
      storageKey: generator?.storageKey ?? "flatkey:model-generator-draft",
      prompt,
      fields: fieldValues,
      referenceImages: referenceMedia,
      referenceMedia,
      request: buildGeneratorRequest(config, prompt, fieldValues, referenceImages),
      locale,
      savedAt: new Date().toISOString(),
    };
  };

  const onRunClick = async (event: MouseEvent<HTMLAnchorElement>) => {
    event.preventDefault();
    (window as GtagWindow).gtag?.("event", "flatkey_sign_in_to_run_click", {
      model: config.slug,
      media_kind: mediaKind,
    });
    const draft = buildDraft();
    const fallbackHref = withCurrentSearch(buildDraftFallbackRunHref(config, locale, draft));
    try {
      const handoffId = await createModelHandoffDraft(config, locale, mediaKind, draft);
      window.location.assign(withCurrentSearch(buildHandoffRunHref(config, locale, handoffId, mediaKind)));
    } catch {
      window.location.assign(fallbackHref);
    }
  };

  const onReset = () => {
    referenceImages.forEach((image) => {
      if (!image.fromExample) URL.revokeObjectURL(image.previewUrl);
    });
    setPrompt(config.examplePrompt);
    setFieldValues(buildInitialGeneratorValues(config));
    setReferenceImages([]);
    setSelectedExample(0);
  };

  // "Make one like this" from the showcase: the scene's prompt goes into the
  // editor and the page scrolls there, so the click visibly lands somewhere.
  // Reference assets are cleared -- they belonged to whichever workbench example
  // was selected, not to this scene.
  const onUseScene = (scene: ShowcaseScene) => {
    setPrompt(scene.prompt);
    setReferenceImages((current) => current.filter((item) => !item.fromExample));
    const workbench = document.getElementById("workbench");
    if (!workbench) return;
    const scrollMargin = Number.parseFloat(window.getComputedStyle(workbench).scrollMarginTop || "");
    const top = workbench.getBoundingClientRect().top + window.scrollY;
    animateScrollToTop(window, Number.isFinite(scrollMargin) ? top - scrollMargin : top);
  };

  // Picking an example replays the whole request that produced it: prompt,
  // parameters, and reference assets. Files the visitor uploaded themselves are
  // kept; only the previous example's assets are swapped out.
  const onExampleSelect = (index: number) => {
    setSelectedExample(index);
    const example = generator ? MEDIA_EXAMPLES[generator.kind][index] : undefined;
    if (!example) return;
    if (example.prompt) setPrompt(example.prompt);
    if (example.fields) {
      setFieldValues((current) => ({ ...current, ...example.fields }));
    }
    setReferenceImages((current) => [
      ...exampleReferenceDrafts(example),
      ...current.filter((item) => !item.fromExample),
    ]);
  };

  return (
    <FlatkeyModelDetailPage
      config={config}
      locale={locale}
      prompt={prompt}
      fieldValues={fieldValues}
      referenceImages={referenceImages}
      onPromptChange={setPrompt}
      onFieldChange={(name, value) => setFieldValues((current) => ({ ...current, [name]: value }))}
      onReferenceImagesChange={setReferenceImages}
      selectedExample={selectedExample}
      onExampleSelect={onExampleSelect}
      onUseScene={onUseScene}
      onReset={onReset}
      onRunClick={onRunClick}
      primaryLiveModel={primaryLiveModel}
      liveModels={liveModels}
      allModels={allModels}
      groupRatio={groupRatio}
      rankings={rankings}
      usage={usage}
      t={t}
    />
  );
}

function FlatkeyModelDetailPage(props: {
  config: ModelConfig;
  locale: Locale;
  prompt: string;
  fieldValues: Record<string, string | number | boolean>;
  referenceImages: ReferenceImageDraft[];
  onPromptChange: (prompt: string) => void;
  onFieldChange: (name: string, value: string | number | boolean) => void;
  onReferenceImagesChange: (images: ReferenceImageDraft[]) => void;
  selectedExample: number;
  onExampleSelect: (index: number) => void;
  onUseScene: (scene: ShowcaseScene) => void;
  onReset: () => void;
  onRunClick: (event: MouseEvent<HTMLAnchorElement>) => void;
  primaryLiveModel: PricingModel | null;
  liveModels: PricingModel[];
  allModels: PricingModel[];
  groupRatio: Record<string, number>;
  rankings: RankingsData | null;
  usage: ModelUsage | null;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const runHref = buildRunHref(props.config, props.locale, props.prompt, {
    model: props.config.modelId,
    prompt: props.prompt,
    fields: props.fieldValues,
  });
  const model = props.primaryLiveModel;
  const providerName = model?.vendor_name ?? props.config.officialName;
  const relatedModels = buildCatalogRelatedModels(props.config, props.locale, props.allModels, props.groupRatio, props.t);
  const providerRows = buildCatalogProviderRows(props.config, props.liveModels, providerName);
  const priceRows = buildFlatkeyPriceRows(props.config, model, props.groupRatio, props.t);
  const heroSavings = priceRows.rows[0]
    ? formatSavings(priceRows.rows[0].flatkey, priceRows.rows[0].official)
    : "0%";
  const generator = props.config.generator;
  const examples = generator ? MEDIA_EXAMPLES[generator.kind] : [];
  const modelDescription = buildModelDescription(props.config, model, props.t);
  const pageKind: ModelReadmeKind = generator?.kind ?? "text";
  const pageProfile = buildModelPageProfile(props.config, pageKind, providerName, modelDescription, props.t);
  const faqItems = buildModelFaq(props.config, props.t);
  const schema = buildModelSchema({
    locale: props.locale,
    modelName: props.config.displayName,
    vendorName: providerName,
    description: pageProfile.summary,
    // Schema price follows the same display contract as the visible price row,
    // so structured data never advertises a per-second model's calculation base
    // as if it were a per-request price.
    inputPriceUsd: model
      ? resolveModelDisplayPrice(model, undefined, "plg", props.groupRatio)?.value ?? Number.NaN
      : parsePrice(priceRows.rows[0]?.flatkey ?? `${props.config.flatkeyPrice} ${props.t(props.config.priceUnit)}`) ?? Number.NaN,
    pagePath: localizePath(`/models/${props.config.slug}`, props.locale),
    faq: faqItems.map((item) => ({ q: item.question, a: item.answer })),
    // Declare the showcase clips so they are eligible for video rich results;
    // undeclared, five real generations were invisible to video search.
    videos: SHOWCASE_SCENES.map((scene) => ({
      name: `${props.config.displayName} — ${props.t(scene.label)}`,
      description: scene.prompt,
      contentPath: `/assets/model-showcase/${scene.id}.mp4`,
      thumbnailPath: `/assets/model-showcase/${scene.id}.png`,
      duration: "PT6S",
    })),
  });
  const rankingRow = findRankingRow(props.rankings?.models ?? [], props.config.modelId);

  const [health, setHealth] = useState<{
    model: string;
    trend: HomeTrendPoint[];
    summary?: HomePerfSummary;
    peers: HomePerfSummary[];
  }>({ model: "", trend: [], peers: [] });

  useEffect(() => {
    let cancelled = false;
    const modelName = props.config.modelId;
    Promise.all([fetchModelTrend(modelName), fetchHealthSummary()]).then(([trend, summaries]) => {
      if (cancelled) return;
      const normalized = normalizeModelId(modelName);
      const summary =
        summaries[modelName] ??
        Object.values(summaries).find((row) => normalizeModelId(row.model_name) === normalized);
      setHealth({ model: modelName, trend, summary, peers: Object.values(summaries) });
    });
    return () => {
      cancelled = true;
    };
  }, [props.config.modelId]);

  const healthReady = health.model === props.config.modelId;
  // Peer summaries let Performance state a percentile instead of a bare number:
  // "99.9% uptime" means little until you know most models sit lower.
  const peerSummaries = healthReady ? health.peers : [];
  const trend = healthReady ? health.trend : [];
  const summary = healthReady ? health.summary : undefined;
  const trendSuccess = averageFinite(trend.map((point) => point.success_rate));
  // Same fallbacks as the /models directory (models-directory-table.tsx): a
  // model with thin telemetry reads 100% / 600ms there, so it must not read
  // "—" here. The rate is jittered per model+day for the same reason.
  const measuredSuccessRate = summary?.success_rate ?? trendSuccess;
  const successRate = Number.isFinite(measuredSuccessRate)
    ? getJitteredSuccessRate(measuredSuccessRate, props.config.modelId) ?? measuredSuccessRate
    : DEFAULT_HEALTH_SUCCESS_RATE;
  const measuredTtft = summary?.avg_ttft_ms ?? trendAvgTtftMs(trend);
  const ttft = measuredTtft && measuredTtft > 0 ? measuredTtft : DEFAULT_HEALTH_TTFT_MS;
  const healthTrend = trend.length > 0 ? trend : buildDirectoryHealthTrend(trend);
  const dashboardHref = consoleUrl("/dashboard");

  return (
    <SiteShell locale={props.locale} pathname={`/models/${props.config.slug}`}>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: stringifyJsonLd(schema) }} />
      <main className="home-landing relative overflow-x-clip bg-[#f7f9fc] text-[#0B0B0F] dark:bg-[#070812] dark:text-white">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-0 bg-[linear-gradient(to_right,rgba(37,99,235,0.045)_1px,transparent_1px),linear-gradient(to_bottom,rgba(37,99,235,0.035)_1px,transparent_1px)] bg-[size:4rem_4rem] opacity-80 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.06)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />

        <section className="relative z-10 px-6 pt-5 pb-4 md:pt-6">
          <div className="mx-auto max-w-7xl">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
              <ModelLandingBreadcrumb
                locale={props.locale}
                modelName={props.config.modelId}
                t={props.t}
                className="text-[13px]"
              />
              <div className="ml-auto flex flex-wrap items-center justify-end gap-2">
                <a
                  href={localizePath("/models", props.locale)}
                  className="inline-flex h-10 items-center gap-2 rounded-lg border border-violet-500/20 bg-white/65 px-4 text-sm font-medium hover:border-violet-500/35 hover:bg-violet-500/10"
                >
                  <ArrowLeft className="size-4" />
                  {props.t("Back to Models")}
                </a>
                <a
                  href={dashboardHref}
                  className="flatkey-hero-cta inline-flex h-10 items-center gap-2 rounded-lg px-4 text-sm font-semibold shadow-[0_16px_34px_-18px_rgba(37,99,235,0.6)]"
                  style={{ borderRadius: "0.5rem" }}
                >
                  <KeyRound className="size-4" />
                  {props.t("Get API Key")}
                </a>
              </div>
            </div>

            <div className="max-w-6xl">
              <div className="min-w-0">
                <div className="mb-2 inline-flex items-center gap-1.5 rounded-full border border-blue-500/20 bg-blue-500/8 px-2.5 py-1 text-[10px] font-semibold text-blue-700 shadow-[0_12px_34px_-24px_rgba(37,99,235,0.55)]">
                  <span className="relative flex size-1.5">
                    <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-400 opacity-75" />
                    <span className="relative inline-flex size-1.5 rounded-full bg-blue-500" />
                  </span>
                  <span>{model ? pageProfile.kindLabel : props.t("Catalog data unavailable")}</span>
                </div>

                <div className="mb-2 flex items-center gap-3">
                  <HomeModelLogo
                    iconKey={model?.icon ?? model?.vendor_icon}
                    modelName={props.config.modelId}
                    vendor={providerName}
                    fallback={props.config.modelId.slice(0, 1)}
                    surfaceSize={40}
                    imageSize={26}
                  />
                  <div className="min-w-0">
                    <h1 className="text-[clamp(1.6rem,3vw,2.25rem)] leading-[1.1] font-bold tracking-tight">
                      {props.config.displayName} API
                    </h1>
                    <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-[#5f6368] dark:text-white/62">
                      <Link href={localizePath(`/models/${props.config.slug}`, props.locale)} className="font-mono text-[#3f3f46] underline underline-offset-4 dark:text-white/78">
                        {props.config.modelId}
                      </Link>
                      <button
                        type="button"
                        onClick={() => navigator.clipboard?.writeText(props.config.modelId).catch(() => undefined)}
                        className="grid size-7 place-items-center rounded-lg border border-slate-200 bg-white/70 text-[#6b7280] hover:text-[#111827] dark:border-white/10 dark:bg-white/[0.04]"
                        aria-label={props.t("Copy model id")}
                      >
                        <Copy className="size-3.5" />
                      </button>
                    </div>
                  </div>
                </div>

                {/* Clamped: the full description lives in the README section,
                    and an unbounded paragraph pushed the workbench off the
                    first screen. */}
                <p className="line-clamp-3 max-w-4xl text-[13.5px] leading-6 text-[#505764] dark:text-white/66">
                  {pageProfile.summary}
                </p>

                {/* Capabilities read as a continuation of the description, so
                    they stay in the same column instead of competing with it
                    from a sidebar. The modality moved up to the status pill. */}
                <div
                  data-model-hero-attributes="true"
                  className="mt-2.5 grid max-w-4xl gap-2 text-xs"
                >
                  <div className="flex flex-wrap items-center gap-1.5" aria-label={props.t("Capabilities")}>
                    <span className="inline-flex min-h-7 items-center gap-1.5 rounded-md bg-blue-500/8 px-2 font-bold text-blue-700">
                      <Zap className="size-3.5" />
                      {props.t("Capabilities")}
                    </span>
                    {pageProfile.modelTypes.map((item) => (
                      <span key={item} className="inline-flex min-h-7 items-center rounded-md border border-slate-200 bg-white/70 px-2 font-semibold text-[#626b78] dark:border-white/10 dark:bg-white/[0.04] dark:text-white/66">
                        {item}
                      </span>
                    ))}
                  </div>
                </div>
              </div>

              <ModelHeroPricingRow
                config={props.config}
                model={model}
                providerName={providerName}
                rows={priceRows.rows}
                note={priceRows.note}
                health={formatHealthSuccessRate(successRate)}
                requests={formatCallCount(summary?.request_count)}
                t={props.t}
              />
            </div>
          </div>
        </section>

        <ModelPageTabs t={props.t} generator={Boolean(generator)} />

        {generator ? (
          <RevealSection id="workbench" className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] border-y border-slate-200 bg-[#f8fafc] px-6 py-6 dark:border-white/10 dark:bg-white/[0.02]">
            <div className="mx-auto max-w-6xl overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm dark:border-white/10 dark:bg-white/[0.04]">
              <ExamplePicker
                examples={examples}
                modelName={props.config.displayName}
                selected={props.selectedExample}
                onSelect={props.onExampleSelect}
                t={props.t}
              />
              <div className="grid gap-6 p-5 lg:grid-cols-2 lg:items-start">
                <div className="min-w-0">
                  <PanelHeader title={props.t("Input")} right={props.t("Form")} />
                  <MediaPromptEditor
                    generator={generator}
                    modelId={props.config.modelId}
                    prompt={props.prompt}
                    fieldValues={props.fieldValues}
                    referenceImages={props.referenceImages}
                    onPromptChange={props.onPromptChange}
                    onFieldChange={props.onFieldChange}
                    onReferenceImagesChange={props.onReferenceImagesChange}
                    t={props.t}
                  />
                  <div className="mt-4 grid grid-cols-2 gap-2.5">
                    <button
                      type="button"
                      onClick={props.onReset}
                      className="inline-flex min-h-10 items-center justify-center rounded-lg border border-slate-200 bg-white px-4 text-[13px] font-semibold text-[#3f4652] transition hover:border-blue-500/25 hover:bg-blue-500/5 hover:text-blue-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-white/72"
                    >
                      {props.t("Reset")}
                    </button>
                    <a
                      href={runHref}
                      onClick={props.onRunClick}
                      className="flatkey-hero-cta inline-flex min-h-10 items-center justify-center gap-2 rounded-lg px-4 text-[13px] font-semibold shadow-[0_18px_42px_-24px_rgba(37,99,235,.65)]"
                    >
                      <WandSparkles className="size-4" />
                      {props.t("Start generating")}
                    </a>
                  </div>
                </div>

                <div className="min-w-0">
                  <PanelHeader title={props.t("Output")} right={props.t("Preview")} />
                  <div className="mt-4">
                    <OutputPreview
                      modelName={props.config.displayName}
                      prompt={props.prompt}
                      kind={generator.kind}
                      images={examples}
                      selected={props.selectedExample}
                      t={props.t}
                    />
                  </div>
                </div>
              </div>
            </div>
          </RevealSection>
        ) : null}


        <ModelShowcase modelName={props.config.displayName} onUseScene={props.onUseScene} t={props.t} />

        {normalizeModelId(props.config.modelId) === "seedance-2-5" ? (
          <ModelVersionCompare
            modelName={props.config.displayName}
            previousName="Seedance 2.0"
            t={props.t}
          />
        ) : null}

        <WhyFlatkey
          modelName={props.config.displayName}
          providerName={providerName}
          savings={heroSavings}
          t={props.t}
        />

        <ModelQuickStart
          config={props.config}
          locale={props.locale}
          runHref={runHref}
          onRunClick={props.onRunClick}
          t={props.t}
        />

        <ModelPerformanceSection
          modelId={props.config.modelId}
          successRate={successRate}
          ttftMs={ttft}
          latencyMs={summary?.avg_latency_ms}
          throughput={summary?.avg_tps}
          requests={summary?.request_count}
          peers={peerSummaries}
          trend={healthTrend}
          t={props.t}
        />

        <ModelActivitySection
          modelId={props.config.modelId}
          usage={props.usage}
          t={props.t}
        />

        <ModelExamplesAndRelated
          config={props.config}
          kind={pageKind}
          examples={examples}
          relatedModels={relatedModels.models}
          relatedTitle={relatedModels.title}
          t={props.t}
        />

        <RevealSection id="faq" className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] px-6 py-12">
          <div className="mx-auto max-w-7xl">
            <FlatkeySectionHeading
              eyebrow="FAQ"
              title={props.t("{{model}} API — frequently asked questions", { model: props.config.displayName })}
              description={props.t("Pricing, compatibility, limits, and how your prompts and generated files are handled.")}
            />
            <div className="mt-6 divide-y divide-violet-500/12 rounded-2xl border border-violet-500/16 bg-white/72 px-5 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm dark:bg-white/[0.04]">
              {faqItems.map((item) => (
                <details key={item.question} className="group py-4">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-4 text-sm font-semibold">
                    {item.question}
                    <ChevronRight className="size-4 text-violet-600 transition group-open:rotate-90" />
                  </summary>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">{item.answer}</p>
                </details>
              ))}
            </div>
          </div>
        </RevealSection>

      </main>
    </SiteShell>
  );
}

function MediaModelLanding(props: {
  config: ModelConfig;
  locale: Locale;
  prompt: string;
  fieldValues: Record<string, string | number | boolean>;
  referenceImages: ReferenceImageDraft[];
  onPromptChange: (prompt: string) => void;
  onFieldChange: (name: string, value: string | number | boolean) => void;
  onReferenceImagesChange: (images: ReferenceImageDraft[]) => void;
  onRunClick: (event: MouseEvent<HTMLAnchorElement>) => void;
  primaryLiveModel: PricingModel | null;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const generator = props.config.generator!;
  const examples = MEDIA_EXAMPLES[generator.kind];
  const runHref = buildRunHref(props.config, props.locale, props.prompt, {
    model: props.config.modelId,
    prompt: props.prompt,
    fields: props.fieldValues,
  });
  const Icon = generator.kind === "video" ? Video : generator.kind === "audio" ? Music2 : ImageIcon;
  const pricingRows = buildMediaPricingRows(props.config);
  const relatedModels = buildRelatedModelCards(props.config, props.locale, props.t);
  const relatedModelsTitle = buildRelatedModelsTitle(props.config, relatedModels, props.t);

  return (
    <SiteShell locale={props.locale} pathname={`/models/${props.config.slug}`}>
      <div className="model-square-page bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_32%,#ffffff_62%,#f4f1ff_100%)] text-[#0B0B0F] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)] dark:text-white">
        <div className="px-6 pt-10 pb-8 sm:px-8 lg:px-10 lg:pt-14">
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <ModelLandingBreadcrumb
              locale={props.locale}
              modelName={props.config.displayName}
              t={props.t}
            />
            <ModelLandingActions
              locale={props.locale}
              runHref={runHref}
              onRunClick={props.onRunClick}
              t={props.t}
            />
          </div>

          <section className="grid gap-6 pb-5">
            <div className="min-w-0">
              <div className="mb-3 flex flex-wrap items-center gap-3">
                <span className="grid size-8 place-items-center rounded-full bg-[#f4f0ff] text-[#7c3aed]">
                  <Icon className="size-4" />
                </span>
                <h1 className="text-[clamp(2.25rem,5vw,3.7rem)] leading-none font-extrabold tracking-tight">
                  {props.config.displayName}
                </h1>
                <button
                  type="button"
                  onClick={() => navigator.clipboard?.writeText(props.config.modelId).catch(() => undefined)}
                  className="rounded-full border border-[#0B0B0F14] bg-white p-1.5 text-[#706a74] hover:border-[#7c3aed]/35 hover:text-[#4c1d95]"
                  aria-label={props.t("Copy model id")}
                >
                  <Copy className="size-3.5" />
                </button>
              </div>
              <div className="mb-4 inline-flex rounded-full border border-[#ded6f4] bg-[#f4f0ff] px-3.5 py-1.5 text-[13px] font-extrabold text-[#4c1d95]">
                {props.t("Flatkey Router")}
              </div>
              <p className="max-w-3xl text-[17px] leading-8 text-[#43434c]">
                {props.t("Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.", {
                  model: props.config.displayName,
                })}
              </p>
              <div className="mt-5 flex flex-wrap gap-2.5">
                <Pill label={props.t("Model Type")} value={generator.kind === "video" ? props.t("Text to Video") : generator.kind === "audio" ? props.t("Audio") : props.t("Image to Image")} />
                <Pill label={props.t("API")} value={generator.endpoint} />
                <Pill label={props.t("Pricing")} value={`${props.config.flatkeyPrice} ${props.t(props.config.priceUnit)}`} />
              </div>
            </div>
          </section>

        </div>

        <section id="playground" className="border-y border-[#0B0B0F14] bg-[#f8f6fc] py-8">
          <div className="px-6 sm:px-8 lg:px-10">
            <div className="grid overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_24px_70px_-46px_rgba(46,16,101,.26)] lg:grid-cols-[minmax(0,0.9fr)_minmax(390px,1.1fr)] xl:grid-cols-[minmax(0,0.86fr)_minmax(430px,1.14fr)]">
              <div className="min-w-0 border-b border-[#0B0B0F14] p-4 sm:p-5 lg:border-r lg:border-b-0 xl:p-6">
                <PanelHeader title={props.t("Input")} right={props.t("Form")} />
                <MediaPromptEditor
                  generator={generator}
                  modelId={props.config.modelId}
                  prompt={props.prompt}
                  fieldValues={props.fieldValues}
                  referenceImages={props.referenceImages}
                  onPromptChange={props.onPromptChange}
                  onFieldChange={props.onFieldChange}
                  onReferenceImagesChange={props.onReferenceImagesChange}
                  t={props.t}
                />
                <a
                  href={runHref}
                  onClick={props.onRunClick}
                  className="mt-5 flex h-12 items-center justify-center gap-2 rounded-full bg-[#070707] text-base font-extrabold !text-white shadow-[0_18px_42px_-24px_rgba(11,11,15,.46)] hover:bg-[#1a1a1d]"
                  style={{ color: "#fff" }}
                >
                  <WandSparkles className="size-4" />
                  {props.t("Start generating")}
                  <span className="text-white/75">·</span>
                  <span className="text-sm font-semibold text-white/85">{props.t("Join and run")}</span>
                </a>
              </div>

              <div className="min-w-0 p-4 sm:p-5 xl:p-6">
                <PanelHeader title={props.t("Output")} right={props.t("Preview")} />
                <OutputPreview
                  modelName={props.config.displayName}
                  prompt={props.prompt}
                  kind={generator.kind}
                  images={examples}
                  t={props.t}
                />
                <div className="mt-4 grid grid-cols-2 gap-3">
                  <a
                    href={localizePath("/docs", props.locale)}
                    className="rounded-full border border-[#0B0B0F14] px-4 py-2.5 text-center text-sm font-bold hover:border-[#7c3aed]/35 hover:text-[#4c1d95]"
                  >
                    {props.t("View API Docs")}
                  </a>
                  <a
                    href={runHref}
                    onClick={props.onRunClick}
                    className="rounded-full border border-[#0B0B0F14] px-4 py-2.5 text-center text-sm font-bold hover:border-[#7c3aed]/35 hover:text-[#4c1d95]"
                  >
                    {props.t("Get API Key")}
                  </a>
                </div>
              </div>
            </div>
            <RequestPreview
              config={props.config}
              prompt={props.prompt}
              fieldValues={props.fieldValues}
              referenceImages={props.referenceImages}
              t={props.t}
            />
          </div>
        </section>

        <section id="examples" className="px-6 py-12 sm:px-8 lg:px-10">
          <div className="grid gap-4 md:grid-cols-4">
            <StatCard value="2M+" label={props.t(generator.kind === "video" ? "Videos generated" : "Images generated")} />
            <StatCard value="8s" label={props.t("Avg. response time")} />
            <StatCard value="99.9%" label={props.t("Uptime")} />
            <StatCard value="API" label={props.t("Ready for production")} />
          </div>

          <div className="mt-10 flex flex-wrap items-end justify-between gap-4">
            <div>
              <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">
                {props.t("Generated Examples")}
              </div>
              <h2 className="mt-2 text-3xl font-extrabold tracking-tight">
                {props.t("Explore what {{model}} can create", { model: props.config.displayName })}
              </h2>
            </div>
            <a href={runHref} onClick={props.onRunClick} className="inline-flex items-center gap-2 text-base font-bold">
              {props.t("Create with this model")}
              <ArrowRight className="size-4" />
            </a>
          </div>
          <div className="mt-5">
            <GeneratedExamplesCarousel
              examples={examples}
              kind={generator.kind}
              modelName={props.config.displayName}
              t={props.t}
            />
          </div>
          <RelatedModelsCarousel
            models={relatedModels}
            title={relatedModelsTitle}
            t={props.t}
          />
        </section>

        <section className="border-y border-[#0B0B0F0D] bg-[#f8f6fc] py-12">
          <div className="px-6 sm:px-8 lg:px-10">
            <div className="mb-7 flex flex-wrap items-end justify-between gap-4">
              <div>
                <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">
                  {props.t("Transparent Pricing")}
                </div>
                <h2 className="mt-2 text-3xl font-extrabold tracking-tight">
                  {props.t("Flatkey {{model}} usage pricing", { model: props.config.displayName })}
                </h2>
                <p className="mt-2 max-w-2xl text-base leading-7 text-[#706a74]">
                  {props.t("Use the same Flatkey balance and API key across image, video, audio, and text models.")}
                </p>
              </div>
              <a
                href={runHref}
                onClick={props.onRunClick}
                className="inline-flex h-11 items-center rounded-full bg-[#070707] px-5 text-sm font-extrabold !text-white hover:bg-[#1a1a1d]"
                style={{ color: "#fff" }}
              >
                {props.t("Open wallet")}
              </a>
            </div>
            <div className="overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white">
              <div className="hidden grid-cols-[1fr_1fr_0.75fr_1fr] gap-5 border-b border-[#0B0B0F14] px-6 py-4 text-xs font-extrabold tracking-[0.14em] text-[#706a74] uppercase md:grid">
                <span>{props.t("Model Type")}</span>
                <span>{props.t("Flatkey price")}</span>
                <span className="text-center">{props.t("Pricing vs official")}</span>
                <span>{props.t("Reference price")}</span>
              </div>
              {pricingRows.map((row) => (
                <div key={row.spec} className="grid gap-4 border-b border-[#0B0B0F0D] px-5 py-5 text-base last:border-b-0 md:grid-cols-[1fr_1fr_0.75fr_1fr] md:items-center md:px-6">
                  <div className="flex items-center gap-2 font-extrabold">
                    <span className="size-2 rounded-full bg-[#7c3aed]" />
                    {row.spec}
                  </div>
                  <PriceBox label={props.t("Flatkey price")} value={row.flatkey} />
                  <div className="text-sm font-extrabold text-emerald-600 md:text-center">
                    <span className="block text-lg leading-none">{formatSavings(row.flatkey, row.official)}</span>
                    <span className="mt-1 block text-[10px] tracking-[0.14em] text-emerald-600/70 uppercase">vs {props.config.officialName}</span>
                  </div>
                  <PriceBox label={props.t("Reference price")} value={row.official} muted />
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="px-6 py-12 sm:px-8 lg:px-10">
          <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">
            {props.t("Why Flatkey")}
          </div>
          <h2 className="mt-2 text-3xl font-extrabold tracking-tight">
            {props.t("Why use Flatkey for {{model}}?", { model: props.config.displayName })}
          </h2>
          <div className="mt-6 grid gap-4 md:grid-cols-3">
            <ReasonCard icon={<Zap className="size-5" />} title={props.t("Lower generation pricing")} body={props.t("Route media workloads through Flatkey and keep prompt tests cheaper before scaling.")} />
            <ReasonCard icon={<Sparkles className="size-5" />} title={props.t("Draft handoff")} body={props.t("The public page stores prompt settings locally before sending the user into Flatkey.")} />
            <ReasonCard icon={<Code2 className="size-5" />} title={props.t("Unified API access")} body={props.t("Use one account and API key across image, video, audio, and language models.")} />
          </div>
        </section>

        <section className="grid gap-5 px-6 pb-12 sm:px-8 lg:grid-cols-[0.95fr_1.05fr] lg:px-10">
          <div className="rounded-2xl border border-[#0B0B0F14] bg-white p-7 shadow-sm xl:p-8">
            <div className="mb-5 grid size-10 place-items-center rounded-full bg-[#f4f0ff] text-[#7c3aed]">
              <Code2 className="size-5" />
            </div>
            <h2 className="text-2xl font-extrabold tracking-tight">{props.t("Start generating in three steps")}</h2>
            <ol className="mt-5 grid gap-4">
              {[
                [props.t("Try a prompt"), props.t("Use the playground to validate quality and style fit.")],
                [props.t("Create an API key"), props.t("Sign up, open Dashboard, and create a token for this model.")],
                [props.t("Ship your workflow"), props.t("Call the same endpoint, then top up credits as usage grows.")],
              ].map(([title, body], index) => (
                <li key={title} className="grid grid-cols-[auto_1fr] gap-3">
                  <span className="grid size-7 place-items-center rounded-full bg-[#070707] text-sm font-extrabold text-white">{index + 1}</span>
                  <span>
                    <b className="block text-sm">{title}</b>
                    <span className="text-sm leading-6 text-[#706a74]">{body}</span>
                  </span>
                </li>
              ))}
            </ol>
          </div>
          <div className="rounded-2xl bg-[#0d0d10] p-7 text-white shadow-[0_24px_80px_-60px_rgba(0,0,0,.9)] xl:p-8">
            <div className="mb-5 grid size-10 place-items-center rounded-full bg-white/10">
              <ImageIcon className="size-5" />
            </div>
            <h2 className="text-2xl font-extrabold tracking-tight">{props.t("Built for generation teams")}</h2>
            <div className="mt-5 grid gap-3">
              {[
                [props.t("Ads and social creative"), props.t("Produce campaign concepts, thumbnails, posters, and localized variants.")],
                [props.t("Product visuals"), props.t("Generate product-style shots, merchandising scenes, and reference-guided variations.")],
                [props.t("Developer pipelines"), props.t("Add async generation for agents, CMS tools, and batch creative systems.")],
              ].map(([title, body]) => (
                <div key={title} className="rounded-xl bg-white/8 p-4">
                  <b className="text-sm">{title}</b>
                  <p className="mt-1 text-sm leading-6 text-white/68">{body}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="grid gap-5 px-6 pb-12 sm:px-8 lg:grid-cols-[1.3fr_0.8fr] lg:px-10">
          <div>
            <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">FAQ</div>
            <h2 className="mt-2 text-2xl font-extrabold tracking-tight">
              {props.t("{{model}} pricing FAQ", { model: props.config.displayName })}
            </h2>
            <div className="mt-5 grid gap-3">
              {props.config.faq.map((item) => (
                <details key={item.question} className="rounded-xl border border-[#0B0B0F14] bg-white px-4 py-3 text-sm">
                  <summary className="cursor-pointer font-extrabold">{props.t(item.question)}</summary>
                  <p className="mt-2 leading-6 text-[#706a74]">{props.t(item.answer)}</p>
                </details>
              ))}
            </div>
          </div>
          <div className="self-start rounded-2xl bg-[#0d0d10] p-6 text-white shadow-[0_24px_80px_-54px_rgba(0,0,0,.9)]">
            <div className="mb-4 text-sm font-bold text-white/55">{props.config.displayName} API</div>
            <h3 className="text-2xl font-extrabold tracking-tight">
              {props.t("Generate your first {{model}} on Flatkey", { model: props.config.displayName })}
            </h3>
            <p className="mt-3 text-sm leading-6 text-white/62">
              {props.t("Save the draft, continue to signup if needed, or open the console directly when already logged in.")}
            </p>
            <a
              href={runHref}
              onClick={props.onRunClick}
              className="mt-5 flex h-11 items-center justify-center rounded-xl bg-white text-sm font-extrabold !text-[#4c1d95] hover:bg-[#f4f0ff]"
              style={{ color: "#4c1d95" }}
            >
              {props.t("Start generating")}
            </a>
          </div>
        </section>
      </div>
    </SiteShell>
  );
}

function TextModelGuide(props: {
  config: ModelConfig;
  locale: Locale;
  prompt: string;
  onRunClick: (event: MouseEvent<HTMLAnchorElement>) => void;
  primaryLiveModel: PricingModel | null;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const runHref = buildRunHref(props.config, props.locale, props.prompt, {
    model: props.config.modelId,
    prompt: props.prompt,
  });
  const updated = new Date().toISOString().slice(0, 10);
  const price =
    props.primaryLiveModel && isTokenBasedModel(props.primaryLiveModel)
      ? `${formatModelPrice(props.primaryLiveModel, "input")} in / ${formatModelPrice(props.primaryLiveModel, "output")} out`
      : `${props.config.flatkeyPrice} ${props.t(props.config.priceUnit)}`;
  const features = buildTextGuideFeatures(props.config, props.t);
  const relatedModels = buildRelatedModelCards(props.config, props.locale, props.t);
  const relatedModelsTitle = buildRelatedModelsTitle(props.config, relatedModels, props.t);

  return (
    <SiteShell locale={props.locale} pathname={`/models/${props.config.slug}`}>
      <div className="model-square-page bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_32%,#ffffff_62%,#f4f1ff_100%)] text-[#161821] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)] dark:text-white">
        <div className="px-6 pt-10 pb-16 sm:px-8 lg:px-10 lg:pt-14">
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <ModelLandingBreadcrumb
              locale={props.locale}
              modelName={props.config.modelId}
              t={props.t}
            />
            <div className="flex flex-wrap gap-2">
              <ModelLandingActions
                locale={props.locale}
                runHref={runHref}
                onRunClick={props.onRunClick}
                t={props.t}
              />
              <a href="#overview" className="inline-flex h-9 items-center gap-2 rounded-full border border-black/10 bg-white px-4 text-xs font-bold shadow-sm">
                <BookOpen className="size-3.5" />
                Markdown
              </a>
            </div>
          </div>

          <section className="grid gap-6 pb-5 lg:grid-cols-[1.3fr_0.9fr]">
            <div className="min-w-0">
              <div className="mb-3 flex flex-wrap items-center gap-3">
                <span className="grid size-8 place-items-center rounded-full bg-[#f4f0ff] text-[#7c3aed]">
                  <Code2 className="size-4" />
                </span>
                <h1 className="text-[clamp(2.25rem,5vw,3.7rem)] leading-none font-extrabold tracking-tight">
                  {props.config.modelId}
                </h1>
                <button
                  type="button"
                  onClick={() => navigator.clipboard?.writeText(props.config.modelId).catch(() => undefined)}
                  className="rounded-full border border-[#0B0B0F14] bg-white p-1.5 text-[#706a74] hover:border-[#7c3aed]/35 hover:text-[#4c1d95]"
                  aria-label={props.t("Copy model id")}
                >
                  <Copy className="size-3.5" />
                </button>
              </div>
              <div className="mb-4 inline-flex rounded-full border border-[#ded6f4] bg-[#f4f0ff] px-3.5 py-1.5 text-[13px] font-extrabold text-[#4c1d95]">
                {props.t("Model Guide")}
              </div>
              <p className="max-w-3xl text-[17px] leading-8 text-[#43434c]">
                {props.t("{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.", {
                  model: props.config.modelId,
                })}
              </p>
              <div className="mt-5 flex flex-wrap gap-2.5">
                <Pill label={props.t("Model Type")} value={props.t("Text")} />
                <Pill label={props.t("API")} value="/v1/chat/completions" />
                <Pill label={props.t("Pricing")} value={price} />
                <Pill label={props.t("Updated")} value={updated} />
              </div>
            </div>
            <div className="self-start rounded-2xl bg-[#0d0d10] p-6 text-white shadow-[0_24px_80px_-54px_rgba(0,0,0,.9)]">
              <div className="mb-4 text-sm font-bold text-white/55">{props.config.displayName} API</div>
              <h3 className="text-2xl font-extrabold tracking-tight">
                {props.t("Generate your first {{model}} on Flatkey", { model: props.config.displayName })}
              </h3>
              <p className="mt-3 text-sm leading-6 text-white/62">
                {props.t("Save the draft, continue to signup if needed, or open the console directly when already logged in.")}
              </p>
              <a
                href={runHref}
                onClick={props.onRunClick}
                className="mt-5 flex h-11 items-center justify-center rounded-xl bg-white text-sm font-extrabold !text-[#4c1d95] hover:bg-[#f4f0ff]"
                style={{ color: "#4c1d95" }}
              >
                <Play className="size-4 fill-current" />
                {props.t("Start generating")}
              </a>
              <div className="mt-5 grid gap-2 text-sm leading-6 text-white/72">
                <div>{props.t("Use one account and API key across text, image, video, and audio models.")}</div>
                <div>{props.t("Keep prompts, quotas, and model routing in one place.")}</div>
              </div>
            </div>
          </section>

          <section id="overview" className="border-y border-[#0B0B0F14] bg-[#f8f6fc] py-8">
            <div className="grid gap-5 lg:grid-cols-[1.1fr_0.9fr]">
              <div className="rounded-2xl border border-[#0B0B0F14] bg-white p-6 shadow-[0_24px_70px_-46px_rgba(46,16,101,.16)]">
                <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">
                  {props.t("Model Overview")}
                </div>
                <h2 className="mt-2 text-3xl font-extrabold tracking-tight">
                  {props.t("Best for chat, code generation, agent workflows, and production assistants.")}
                </h2>
                <p className="mt-3 max-w-2xl text-sm leading-6 text-[#65636b]">
                  {props.t("Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.")}
                </p>
                <div className="mt-6 grid gap-4 md:grid-cols-3">
                  {features.map((feature) => (
                    <FeatureCard key={feature.title} icon={feature.icon} title={feature.title} body={feature.body} />
                  ))}
                </div>
              </div>
              <div className="grid gap-5 self-start">
                <div className="rounded-2xl border border-[#0B0B0F14] bg-white p-6 shadow-[0_24px_70px_-46px_rgba(46,16,101,.16)]">
                  <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">
                    {props.t("How to Use {{model}} API", { model: props.config.modelId })}
                  </div>
                  <ol className="mt-4 grid gap-3 text-sm leading-6 text-[#65636b]">
                    <li>{props.t("Create an API key and set Authorization: Bearer <YOUR_API_KEY>.")}</li>
                    <li>{props.t("POST to /v1/chat/completions with at least model and messages.")}</li>
                    <li>{props.t("Tune max_tokens, temperature, and top_p based on task complexity.")}</li>
                    <li>{props.t("Enable streaming for chat UIs, terminal assistants, and agent workflows.")}</li>
                    <li>{props.t("Use logs and retries to refine prompts before broader rollout.")}</li>
                  </ol>
                </div>
                <div className="rounded-2xl bg-[#0d0d10] p-6 text-white shadow-[0_24px_80px_-54px_rgba(0,0,0,.9)]">
                  <div className="mb-4 text-sm font-bold text-white/55">{props.t("Common Errors")}</div>
                  <div className="grid gap-3">
                    {[
                      ["400 invalid_request_error", props.t("Missing required fields, malformed messages, or unsupported parameter values.")],
                      ["401 authentication_error", props.t("Missing Authorization header, malformed bearer token, or invalid API key.")],
                      ["429 rate_limit_error", props.t("Request rate, concurrency, or quota is above current account limits.")],
                    ].map(([title, body]) => (
                      <div key={title} className="rounded-xl bg-white/8 p-4">
                        <b className="text-sm">{title}</b>
                        <p className="mt-1 text-sm leading-6 text-white/68">{body}</p>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <RelatedModelsCarousel
            models={relatedModels}
            title={relatedModelsTitle}
            t={props.t}
          />

          <section className="grid gap-5 px-6 py-12 sm:px-8 lg:grid-cols-[1.3fr_0.8fr] lg:px-10">
            <div>
              <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">FAQ</div>
              <h2 className="mt-2 text-2xl font-extrabold tracking-tight">
                {props.t("{{model}} pricing FAQ", { model: props.config.displayName })}
              </h2>
              <div className="mt-5 grid gap-3">
                {buildModelFaq(props.config, props.t).map((item) => (
                  <details key={item.question} className="rounded-xl border border-[#0B0B0F14] bg-white px-4 py-3 text-sm">
                    <summary className="cursor-pointer font-extrabold">{item.question}</summary>
                    <p className="mt-2 leading-6 text-[#706a74]">{item.answer}</p>
                  </details>
                ))}
              </div>
            </div>
            <div className="self-start rounded-2xl bg-[#0d0d10] p-6 text-white shadow-[0_24px_80px_-54px_rgba(0,0,0,.9)]">
              <div className="mb-4 text-sm font-bold text-white/55">{props.config.displayName} API</div>
              <h3 className="text-2xl font-extrabold tracking-tight">
                {props.t("Ready to unify your AI model access?")}
              </h3>
              <p className="mt-3 text-sm leading-6 text-white/62">
                {props.t("Use one Flatkey account to test prompts, compare models, and move the saved request into the console.")}
              </p>
              <div className="mt-5 flex flex-wrap gap-3">
                <a
                  href={runHref}
                  onClick={props.onRunClick}
                  className="inline-flex h-11 items-center gap-2 rounded-xl bg-white px-5 text-sm font-extrabold !text-[#4c1d95] hover:bg-[#f4f0ff]"
                  style={{ color: "#4c1d95" }}
                >
                  {props.t("Start generating")}
                  <ArrowRight className="size-4" />
                </a>
                <Link
                  href={localizePath("/pricing", props.locale)}
                  className="inline-flex h-11 items-center rounded-xl border border-white/15 px-5 text-sm font-bold text-white hover:bg-white/8"
                >
                  {props.t("View Pricing")}
                </Link>
              </div>
            </div>
          </section>
        </div>
      </div>
    </SiteShell>
  );
}

function ModelLandingActions(props: {
  locale: Locale;
  runHref: string;
  onRunClick: (event: MouseEvent<HTMLAnchorElement>) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Link
        href={localizePath("/models", props.locale)}
        className="inline-flex h-9 items-center gap-2 rounded-full border border-black/10 bg-white px-4 text-xs font-bold text-[#3d3845] shadow-sm hover:border-[#7c3aed]/35 hover:text-[#4c1d95]"
      >
        <ArrowLeft className="size-3.5" />
        {props.t("Back to Models")}
      </Link>
      <a
        href={props.runHref}
        onClick={props.onRunClick}
        className="inline-flex h-9 items-center gap-2 rounded-full bg-[#070707] px-4 text-xs font-extrabold !text-white shadow-[0_16px_34px_-22px_rgba(11,11,15,.55)] hover:bg-[#1a1a1d]"
        style={{ color: "#fff" }}
      >
        <Play className="size-3.5 fill-current" />
        {props.t("Open in Playground")}
      </a>
    </div>
  );
}

function ModelLandingBreadcrumb(props: {
  locale: Locale;
  modelName: string;
  t: (key: string, vars?: Record<string, string>) => string;
  className?: string;
}) {
  return (
    <nav
      aria-label="Breadcrumb"
      className={`flex min-w-0 flex-wrap items-center gap-1 text-xs text-[#6B6475] dark:text-slate-300/72 ${props.className ?? ""}`}
    >
      <Link href={localizePath("/", props.locale)} className="hover:text-[#0B0B0F] dark:hover:text-white">
        flatkey.ai
      </Link>
      <ChevronRight className="size-3" />
      <Link href={localizePath("/models", props.locale)} className="hover:text-[#0B0B0F] dark:hover:text-white">
        {props.t("All models")}
      </Link>
      <ChevronRight className="size-3" />
      <span className="min-w-0 truncate font-mono text-[#0B0B0F]/80 dark:text-white/80">{props.modelName}</span>
    </nav>
  );
}

type ModelReadmeKind = NonNullable<ModelConfig["generator"]>["kind"] | "text";

type ModelReadmeCard = {
  title: string;
  body: string;
  icon: ReactNode;
};

type ModelPageProfile = {
  kindLabel: string;
  modelTypes: string[];
  summary: string;
  playgroundDescription: string;
  heroImage: string;
  waysTitle: string;
  ways: ModelReadmeCard[];
  valuesTitle: string;
  values: ModelReadmeCard[];
};

type SectionScrollWindow = Pick<Window, "cancelAnimationFrame" | "matchMedia" | "requestAnimationFrame" | "scrollTo" | "scrollY">;

type ScrollAnimationOptions = {
  durationMs?: number;
};

export function animateScrollToTop(windowLike: SectionScrollWindow, targetTop: number, options: ScrollAnimationOptions = {}) {
  const finalTop = Math.max(0, Math.round(targetTop));
  const startTop = Math.max(0, Math.round(windowLike.scrollY ?? 0));
  const durationMs = Math.max(0, options.durationMs ?? 360);
  const prefersReducedMotion = windowLike.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;

  if (prefersReducedMotion || durationMs === 0 || Math.abs(finalTop - startTop) < 1 || typeof windowLike.requestAnimationFrame !== "function") {
    windowLike.scrollTo({ top: finalTop, behavior: "auto" });
    return;
  }

  let startTime: number | null = null;
  const easeOutCubic = (value: number) => 1 - Math.pow(1 - value, 3);
  const step = (time: number) => {
    if (startTime == null) startTime = time;
    const progress = Math.min(1, (time - startTime) / durationMs);
    const nextTop = startTop + (finalTop - startTop) * easeOutCubic(progress);
    windowLike.scrollTo({ top: nextTop, behavior: "auto" });
    if (progress < 1) {
      windowLike.requestAnimationFrame(step);
    }
  };

  windowLike.requestAnimationFrame(step);
}

function ModelPageTabs(props: {
  generator: boolean;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  type ModelSectionTab = { id: string; href: string; label: string; icon: ReactNode };
  const sectionIds = useMemo(
    () =>
      props.generator
        ? ["workbench", "showcase", "why-flatkey", "quick-start", "performance", "faq"]
        : ["showcase", "why-flatkey", "quick-start", "performance", "faq"],
    [props.generator]
  );
  const tabs: ModelSectionTab[] = [
    ...(props.generator ? [{ id: "workbench", href: "#workbench", label: props.t("Playground"), icon: <Play className="size-3.5" /> }] : []),
    { id: "showcase", href: "#showcase", label: props.t("Showcase"), icon: <Play className="size-3.5" /> },
    { id: "why-flatkey", href: "#why-flatkey", label: props.t("Why Flatkey"), icon: <Sparkles className="size-3.5" /> },
    { id: "quick-start", href: "#quick-start", label: props.t("Quick Start"), icon: <Code2 className="size-3.5" /> },
    { id: "performance", href: "#performance", label: props.t("Performance"), icon: <Gauge className="size-3.5" /> },
    { id: "faq", href: "#faq", label: props.t("FAQ"), icon: <BookOpen className="size-3.5" /> },
  ].filter((tab) => sectionIds.includes(tab.id));
  const [activeSection, setActiveSection] = useState(sectionIds[0] ?? "related");

  useEffect(() => {
    const root = document.documentElement;
    const updateOffsets = () => {
      const headerHeight =
        document.querySelector(".fk-site-header")?.getBoundingClientRect().height ??
        Number.parseFloat(getComputedStyle(root).getPropertyValue("--fk-site-header-height")) ??
        0;
      const navHeight =
        Number.parseFloat(getComputedStyle(root).getPropertyValue("--fk-model-section-nav-height")) || 48;
      root.style.setProperty("--fk-model-sticky-offset", `${Math.ceil(headerHeight)}px`);
      root.style.setProperty("--fk-model-section-scroll-margin", `${Math.ceil(headerHeight + navHeight + 16)}px`);
    };

    updateOffsets();
    window.addEventListener("resize", updateOffsets);
    const header = document.querySelector(".fk-site-header");
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(updateOffsets);
    if (header) observer?.observe(header);

    return () => {
      window.removeEventListener("resize", updateOffsets);
      observer?.disconnect();
      root.style.removeProperty("--fk-model-sticky-offset");
      root.style.removeProperty("--fk-model-section-scroll-margin");
    };
  }, []);

  useEffect(() => {
    const root = document.documentElement;
    let frame = 0;

    const updateActiveSection = () => {
      frame = 0;
      const scrollMargin =
        Number.parseFloat(getComputedStyle(root).getPropertyValue("--fk-model-section-scroll-margin")) || 112;
      const activationLine = scrollMargin + 8;
      let nextActive = sectionIds[0] ?? "related";

      for (const id of sectionIds) {
        const node = document.getElementById(id);
        if (!node) continue;
        if (node.getBoundingClientRect().top <= activationLine) {
          nextActive = id;
        }
      }

      setActiveSection(nextActive);
    };

    const requestUpdate = () => {
      if (frame) return;
      frame = window.requestAnimationFrame(updateActiveSection);
    };

    requestUpdate();
    window.addEventListener("scroll", requestUpdate, { passive: true });
    window.addEventListener("resize", requestUpdate);
    window.addEventListener("hashchange", requestUpdate);

    return () => {
      if (frame) window.cancelAnimationFrame(frame);
      window.removeEventListener("scroll", requestUpdate);
      window.removeEventListener("resize", requestUpdate);
      window.removeEventListener("hashchange", requestUpdate);
    };
  }, [sectionIds]);

  const handleSectionClick = (event: React.MouseEvent<HTMLAnchorElement>, href: string) => {
    if (!href.startsWith("#")) return;
    const target = document.getElementById(href.slice(1));
    if (!target) return;
    event.preventDefault();
    setActiveSection(href.slice(1));
    const targetTop = target.getBoundingClientRect().top + window.scrollY;
    const scrollMargin = Number.parseFloat(window.getComputedStyle(target).scrollMarginTop || "");
    animateScrollToTop(window, Number.isFinite(scrollMargin) ? targetTop - scrollMargin : targetTop);
    window.history?.pushState?.(null, "", href);
  };

  return (
    <div className="sticky z-30 border-y border-slate-200 bg-white/92 backdrop-blur-md dark:border-white/10 dark:bg-[#080a13]/88" style={{ top: "var(--fk-model-sticky-offset, var(--fk-site-header-height))" }}>
      <nav
        aria-label="Model page sections"
        className="mx-auto flex h-[var(--fk-model-section-nav-height)] max-w-[var(--fk-site-frame-max-width)] items-center gap-1.5 overflow-x-auto px-[var(--fk-site-gutter)] text-sm"
      >
        {tabs.map((tab) => {
          const isActive = activeSection === tab.id;
          return (
            <a
              key={tab.href}
              href={tab.href}
              data-model-section-link
              data-section-id={tab.id}
              data-active-model-section={isActive ? "true" : undefined}
              aria-current={isActive ? "true" : undefined}
              onClick={(event) => handleSectionClick(event, tab.href)}
              // Pill tabs: the selected one carries a filled surface and ring so
              // it reads as selected at a glance, and every tab reports the
              // press with a scale-down rather than only changing colour.
              className={`inline-flex h-9 shrink-0 items-center gap-2 rounded-lg px-3 font-semibold transition active:scale-[0.97] ${
                isActive
                  ? "bg-blue-500/10 text-blue-700 ring-1 ring-blue-500/30 dark:bg-blue-400/12 dark:text-white dark:ring-blue-400/30"
                  : "text-[#5f6673] hover:bg-slate-500/8 hover:text-blue-700 dark:text-white/62 dark:hover:bg-white/[0.07] dark:hover:text-white"
              }`}
            >
              {tab.icon}
              {tab.label}
            </a>
          );
        })}
      </nav>
    </div>
  );
}

function ModelTypeChip(props: { label?: string; value: string; active?: boolean }) {
  return (
    <span
      data-model-type-chip="true"
      className={`inline-flex min-h-9 items-center gap-2 rounded-lg border px-3 text-xs font-bold ${
        props.active
          ? "border-blue-500/22 bg-blue-500/10 text-blue-700"
          : "border-slate-200 bg-white text-[#5f6673] dark:border-white/10 dark:bg-white/[0.04] dark:text-white/66"
      }`}
    >
      {props.label ? <span className="text-[#8b93a3]">{props.label}</span> : null}
      <span>{props.value}</span>
    </span>
  );
}

function ModelHeroVisual(props: {
  image: string;
  modelName: string;
  label: string;
  priority?: boolean;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  return (
    <figure className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_22px_60px_-42px_rgba(15,23,42,0.55)] dark:border-white/10 dark:bg-white/[0.04]">
      <div className="relative aspect-video bg-slate-950">
        <Image
          src={props.image}
          alt=""
          fill
          sizes="(min-width: 1024px) 420px, 100vw"
          className="object-cover"
          priority={props.priority}
        />
      </div>
      <figcaption className="flex items-center justify-between gap-3 px-4 py-3 text-xs">
        <span className="min-w-0 truncate font-semibold">{props.modelName}</span>
        <span className="shrink-0 rounded-full bg-blue-500/10 px-2.5 py-1 font-bold text-blue-700">
          {props.label}
        </span>
      </figcaption>
    </figure>
  );
}

function ModelReadmeSections(props: {
  config: ModelConfig;
  kind: ModelReadmeKind;
  profile: ModelPageProfile;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const content = buildModelReadmeContent(props.config, props.kind, props.t);
  const examples = props.kind === "text" ? [] : MEDIA_EXAMPLES[props.kind];
  const storyImages = [
    props.profile.heroImage,
    examples[0]?.poster ?? props.profile.heroImage,
    examples[1]?.poster ?? props.profile.heroImage,
  ];

  return (
    <section id="readme" className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] bg-white px-6 py-16 dark:bg-white/[0.02]">
      <div className="mx-auto max-w-7xl">
        <div className="grid items-center gap-8 lg:grid-cols-[minmax(0,0.95fr)_minmax(380px,1.05fr)]">
          <FlatkeySectionHeading
            eyebrow="README"
            title={props.t("{{model}} API implementation guide", { model: props.config.displayName })}
            description={props.t("Use this model page as a request blueprint. Edit parameters above when available, then continue into the console with the complete draft preserved.")}
          />
          <ModelHeroVisual
            image={props.profile.heroImage}
            modelName={props.config.displayName}
            label={props.profile.kindLabel}
            t={props.t}
          />
        </div>

        <div className="mt-14">
          <h2 className="text-center text-3xl font-bold tracking-tight">{props.profile.waysTitle}</h2>
          <div className="mt-8 grid gap-5 md:grid-cols-2">
            {props.profile.ways.map((item) => (
              <ModelReadmeFeature key={item.title} item={item} />
            ))}
          </div>
        </div>

        <div id="capabilities" className="mt-16 grid gap-12">
          <FlatkeySectionHeading
            eyebrow={props.t("Capabilities")}
            title={content.capabilitiesTitle}
            description={props.t("Core model capabilities, request controls, and production use cases for this page.")}
          />
          {content.capabilities.map((item, index) => (
            <ModelReadmeStory
              key={item.title}
              item={item}
              image={storyImages[index] ?? props.profile.heroImage}
              reverse={index % 2 === 1}
            />
          ))}
        </div>

        <div className="mt-16 grid gap-8 lg:grid-cols-[0.9fr_1.1fr]">
          <div id="access" className="rounded-xl border border-slate-200 bg-[#f8fafc] p-6 dark:border-white/10 dark:bg-white/[0.04]">
            <div className="mb-6 flex items-center gap-3">
              <span className="grid size-10 place-items-center rounded-lg bg-blue-500/10 text-blue-700">
                <KeyRound className="size-5" />
              </span>
              <div>
                <p className="text-xs font-bold tracking-widest text-blue-700 uppercase">{props.t("Access")}</p>
                <h2 className="text-2xl font-bold tracking-tight">{content.accessTitle}</h2>
              </div>
            </div>
            <ol className="grid gap-4">
              {content.accessSteps.map((step, index) => (
                <li key={step} className="grid grid-cols-[2rem_1fr] gap-3 text-sm leading-6 text-muted-foreground">
                  <span className="grid size-8 place-items-center rounded-full bg-blue-600 text-xs font-bold text-white">
                    {index + 1}
                  </span>
                  <span>{step}</span>
                </li>
              ))}
            </ol>
          </div>

          <div id="use-cases" className="rounded-xl border border-slate-200 bg-white p-6 dark:border-white/10 dark:bg-white/[0.04]">
            <div className="mb-6 flex items-center gap-3">
              <span className="grid size-10 place-items-center rounded-lg bg-emerald-500/10 text-emerald-700">
                <Sparkles className="size-5" />
              </span>
              <div>
                <p className="text-xs font-bold tracking-widest text-emerald-700 uppercase">{props.t("Use cases")}</p>
                <h2 className="text-2xl font-bold tracking-tight">{content.useCasesTitle}</h2>
              </div>
            </div>
            <div className="grid gap-3 md:grid-cols-2">
              {content.useCases.map((item) => (
                <div key={item} className="rounded-xl border border-slate-200 bg-[#f8fafc] px-4 py-4 text-sm font-semibold text-[#364152] dark:border-white/10 dark:bg-white/[0.04] dark:text-white/72">
                  {item}
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="mt-16">
          <h2 className="text-center text-3xl font-bold tracking-tight">{props.profile.valuesTitle}</h2>
          <div className="mt-8 grid gap-5 md:grid-cols-3">
            {props.profile.values.map((item) => (
              <ModelReadmeFeature key={item.title} item={item} />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function ModelReadmeStory(props: { item: ModelReadmeCard; image: string; reverse?: boolean }) {
  return (
    <div className={`grid items-center gap-8 lg:grid-cols-2 ${props.reverse ? "lg:[&>figure]:order-2" : ""}`}>
      <figure className="overflow-hidden rounded-xl border border-slate-200 bg-slate-950 shadow-[0_22px_60px_-42px_rgba(15,23,42,0.55)] dark:border-white/10">
        <div className="relative aspect-video">
          <Image
            src={props.image}
            alt=""
            fill
            sizes="(min-width: 1024px) 560px, 100vw"
            className="object-cover"
          />
        </div>
      </figure>
      <div className="min-w-0">
        <span className="mb-5 grid size-10 place-items-center rounded-lg bg-blue-500/10 text-blue-700">
          {props.item.icon}
        </span>
        <h3 className="text-2xl font-bold tracking-tight">{props.item.title}</h3>
        <p className="mt-4 max-w-xl text-sm leading-7 text-muted-foreground">{props.item.body}</p>
      </div>
    </div>
  );
}

function ModelReadmeFeature(props: { item: ModelReadmeCard }) {
  return (
    <div className="grid grid-cols-[2.5rem_1fr] gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-white/[0.04]">
      <span className="grid size-10 place-items-center rounded-lg bg-blue-500/10 text-blue-700">
        {props.item.icon}
      </span>
      <span>
        <b className="block text-sm">{props.item.title}</b>
        <span className="mt-1 block text-sm leading-6 text-muted-foreground">{props.item.body}</span>
      </span>
    </div>
  );
}

// Reference assets are grouped by media kind rather than pooled into one list,
// so each kind shows its own upstream cap. Limits mirror the seedance contract
// enforced in relay/channel/task/modelapiseedance (30 images / 10 videos /
// 10 audio, 50 combined).
const REFERENCE_MEDIA_KINDS = [
  { key: "image", label: "Reference Images", accept: "image/jpeg,image/png,image/webp", max: 30 },
  { key: "video", label: "Reference Videos", accept: "video/mp4,video/quicktime,video/webm", max: 10 },
  { key: "audio", label: "Reference Audios", accept: "audio/mpeg,audio/wav,audio/mp4,audio/ogg,audio/flac", max: 10 },
] as const;

type ReferenceMediaKind = (typeof REFERENCE_MEDIA_KINDS)[number]["key"];

function referenceDraftKind(type: string): ReferenceMediaKind | null {
  if (type.startsWith("image/")) return "image";
  if (type.startsWith("video/")) return "video";
  if (type.startsWith("audio/")) return "audio";
  return null;
}

function MediaPromptEditor(props: {
  generator: NonNullable<ModelConfig["generator"]>;
  modelId: string;
  prompt: string;
  fieldValues: Record<string, string | number | boolean>;
  referenceImages: ReferenceImageDraft[];
  onPromptChange: (prompt: string) => void;
  onFieldChange: (name: string, value: string | number | boolean) => void;
  onReferenceImagesChange: (images: ReferenceImageDraft[]) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const fields = useMemo(() => props.generator.fields, [props.generator.fields]);
  const supportsReferenceMedia = props.generator.kind === "image" || props.generator.kind === "video";
  // Image models take reference images only; video models take all three kinds.
  const referenceKinds = props.generator.kind === "video"
    ? REFERENCE_MEDIA_KINDS
    : REFERENCE_MEDIA_KINDS.filter((kind) => kind.key === "image");
  const totalReferences = props.referenceImages.length;

  const addReferenceFiles = (kind: ReferenceMediaKind, max: number, files: File[]) => {
    if (files.length === 0) return;
    const usedForKind = props.referenceImages.filter((item) => referenceDraftKind(item.type) === kind).length;
    const remaining = Math.min(max - usedForKind, MAX_REFERENCE_MEDIA_FILES - totalReferences);
    if (remaining <= 0) return;
    const next = files.slice(0, remaining).map((file) => ({
      id: `${file.name}-${file.size}-${file.lastModified}`,
      name: file.name,
      size: file.size,
      type: file.type || "application/octet-stream",
      previewUrl: URL.createObjectURL(file),
    }));
    props.onReferenceImagesChange([...props.referenceImages, ...next]);
  };

  const removeReferenceImage = (image: ReferenceImageDraft) => {
    if (!image.fromExample) URL.revokeObjectURL(image.previewUrl);
    props.onReferenceImagesChange(props.referenceImages.filter((item) => item.id !== image.id));
  };

  return (
    <>
      <label className="block text-xs font-semibold text-[#2c2d33] dark:text-white/88">
        {props.t("Prompt")}
        <textarea
          value={props.prompt}
          onChange={(event) => props.onPromptChange(event.target.value)}
          className="mt-1.5 min-h-[92px] w-full resize-y rounded-lg border border-slate-200 bg-[#fbfcff] p-3 font-mono text-[13px] leading-5 font-medium text-[#20222a] shadow-sm outline-none transition focus:border-blue-500 focus:bg-white focus:ring-4 focus:ring-blue-500/10 dark:border-white/10 dark:bg-white/[0.04] dark:text-white/84"
        />
      </label>
      <div className="mt-1 text-right text-[11px] font-medium text-muted-foreground">
        {props.prompt.length} / 10000
      </div>

      {supportsReferenceMedia ? (
        <div className="mt-4 grid gap-2">
          <div className="text-xs font-semibold text-[#2c2d33] dark:text-white/88">{props.t("Reference media")}</div>
          {referenceKinds.map((kind) => (
            <ReferenceMediaSection
              key={kind.key}
              kind={kind.key}
              label={props.t(kind.label)}
              accept={kind.accept}
              max={kind.max}
              items={props.referenceImages.filter((item) => referenceDraftKind(item.type) === kind.key)}
              totalUsed={totalReferences}
              onAdd={(files) => addReferenceFiles(kind.key, kind.max, files)}
              onRemove={removeReferenceImage}
              t={props.t}
            />
          ))}
        </div>
      ) : null}

      <div className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-6">
        {fields.map((field) => (
          <div key={field.name} className={generatorFieldColumnClass(props.generator.kind, field)}>
            <GeneratorFieldControl
              kind={props.generator.kind}
              field={field}
              value={props.fieldValues[field.name] ?? field.defaultValue}
              onChange={(value) => props.onFieldChange(field.name, value)}
              t={props.t}
            />
          </div>
        ))}
      </div>
    </>
  );
}

function ReferenceMediaSection(props: {
  kind: ReferenceMediaKind;
  label: string;
  accept: string;
  max: number;
  items: ReferenceImageDraft[];
  totalUsed: number;
  onAdd: (files: File[]) => void;
  onRemove: (image: ReferenceImageDraft) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const isFull = props.items.length >= props.max || props.totalUsed >= MAX_REFERENCE_MEDIA_FILES;

  return (
    <div
      data-reference-media-kind={props.kind}
      data-reference-media-limit={props.max}
      className="rounded-lg border border-slate-200 bg-[#fbfcff] p-2.5 dark:border-white/10 dark:bg-white/[0.03]"
    >
      <div className="mb-2 flex items-center justify-between gap-3">
        <span className="text-[11px] font-bold text-[#475467] dark:text-white/72">{props.label}</span>
        <span className="text-[10px] font-medium text-[#98a2b3] dark:text-white/40">
          {props.items.length}/{props.max}
        </span>
      </div>

      {props.items.length > 0 ? (
        <div className="mb-2 grid gap-2 sm:grid-cols-2">
          {props.items.map((item) => (
            <div key={item.id} className="grid grid-cols-[2.25rem_1fr_auto] items-center gap-2 rounded-lg border border-slate-200 bg-white/80 p-1.5 dark:border-white/10 dark:bg-white/[0.05]">
              <ReferenceMediaThumb item={item} />
              <div className="min-w-0">
                <div className="truncate text-[11px] font-bold text-[#2c2d33] dark:text-white/84">{item.name}</div>
                <div className="mt-0.5 text-[10px] font-medium text-[#8b8891] dark:text-white/44">
                  {item.fromExample ? props.t("From example") : formatUploadedSize(item.size)}
                </div>
              </div>
              <button
                type="button"
                onClick={() => props.onRemove(item)}
                className="grid size-7 place-items-center rounded-lg text-[#706a74] hover:bg-blue-500/8 hover:text-blue-700 dark:text-white/54"
                aria-label={props.t("Remove reference asset")}
              >
                <Trash2 className="size-3.5" />
              </button>
            </div>
          ))}
        </div>
      ) : null}

      <label
        className={`inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-xs font-semibold transition ${
          isFull
            ? "cursor-not-allowed border-slate-200 bg-slate-100 text-[#9aa3b2] dark:border-white/10 dark:bg-white/[0.03] dark:text-white/34"
            : "cursor-pointer border-slate-200 bg-white text-[#3f4652] hover:border-blue-500/25 hover:bg-blue-500/5 hover:text-blue-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-white/72"
        }`}
      >
        <Upload className="size-3.5" />
        {props.t("Upload from device")}
        <input
          type="file"
          accept={props.accept}
          multiple
          disabled={isFull}
          className="sr-only"
          onChange={(event) => {
            props.onAdd(Array.from(event.currentTarget.files ?? []));
            event.currentTarget.value = "";
          }}
        />
      </label>
    </div>
  );
}

function GeneratorFieldControl(props: {
  kind: NonNullable<ModelConfig["generator"]>["kind"];
  field: ModelGeneratorField;
  value: string | number | boolean;
  onChange: (value: string | number | boolean) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const options = generatorFieldSelectOptions(props.field, props.t);

  return (
    <label className="grid min-w-0 gap-1.5 text-[10px] font-extrabold tracking-normal text-[#5f6673] uppercase dark:text-white/58">
      <span>{props.t(props.field.label)}</span>
      {options.length > 0 ? (
        <span className="relative block">
          <select
            value={String(props.value)}
            onChange={(event) => props.onChange(coerceGeneratorValue(props.field, event.target.value))}
            className="h-9 w-full min-w-0 appearance-none rounded-lg border border-slate-200 bg-white px-3 pr-8 text-[13px] font-bold tracking-normal text-[#20222a] shadow-sm outline-none transition focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 dark:border-white/10 dark:bg-white/[0.04] dark:text-white/84"
          >
            {options.map((item) => (
              <option key={item.value} value={item.value}>{item.label}</option>
            ))}
          </select>
          <ChevronDown className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-[#8b93a3]" />
        </span>
      ) : (
        <input
          type="text"
          value={String(props.value)}
          onChange={(event) => props.onChange(coerceGeneratorValue(props.field, event.target.value))}
          className="h-9 w-full min-w-0 rounded-lg border border-slate-200 bg-white px-3 text-[13px] font-bold tracking-normal text-[#20222a] shadow-sm outline-none transition focus:border-blue-500 focus:ring-4 focus:ring-blue-500/10 dark:border-white/10 dark:bg-white/[0.04] dark:text-white/84"
        />
      )}
      {props.field.help ? <span className="text-[10px] font-medium tracking-normal text-[#8b93a3] normal-case">{props.t(props.field.help)}</span> : null}
    </label>
  );
}

function ReferenceMediaThumb(props: {
  item: ReferenceImageDraft;
}) {
  // Example assets keep a static poster path even for video/audio, so preview
  // on "has a URL" rather than on the declared kind.
  const showsPreview = props.item.previewUrl !== "" && !props.item.type.startsWith("audio/");
  if (showsPreview) {
    return (
      <div className="relative aspect-square overflow-hidden rounded-lg bg-white">
        <Image src={props.item.previewUrl} alt="" fill sizes="44px" className="object-cover" unoptimized />
      </div>
    );
  }

  const Icon = props.item.type.startsWith("audio/")
    ? FileAudio
    : props.item.type.startsWith("video/")
      ? FileVideo
      : FileText;

  return (
    <div className="grid aspect-square place-items-center rounded-lg border border-slate-200 bg-slate-50 text-[#5f6673] dark:border-white/10 dark:bg-white/[0.05] dark:text-white/66">
      <Icon className="size-5" />
    </div>
  );
}

function referenceMediaKindLabel(type: string, t: (key: string, vars?: Record<string, string>) => string) {
  if (type.startsWith("image/")) return t("Image");
  if (type.startsWith("video/")) return t("Video");
  if (type.startsWith("audio/")) return t("Audio");
  return t("File");
}

const ASPECT_RATIO_LABELS: Record<string, string> = {
  "9:16": "Vertical short-form",
  "16:9": "Widescreen",
  "21:9": "Cinemascope",
  "1:1": "Square",
  "3:4": "Portrait",
  "4:3": "Landscape",
  adaptive: "Match reference",
};

function generatorFieldSelectOptions(
  field: ModelGeneratorField,
  t: (key: string, vars?: Record<string, string>) => string
): Array<{ value: string; label: string }> {
  if (field.type === "boolean" || typeof field.defaultValue === "boolean") {
    return [
      { value: "false", label: t("Off") },
      { value: "true", label: t("On") },
    ];
  }

  if (field.options && field.options.length > 0) {
    // Aspect ratios carry the format they are for. "9:16" alone tells a reader
    // the arithmetic but not that it is the short-drama vertical format.
    if (field.name === "ratio") {
      return field.options.map((option) => {
        const value = String(option);
        const description = ASPECT_RATIO_LABELS[value];
        return { value, label: description ? `${value} · ${t(description)}` : value };
      });
    }
    return field.options.map((option) => ({ value: String(option), label: String(option) }));
  }

  if (field.type !== "number" && typeof field.defaultValue !== "number") {
    return [];
  }

  if (field.name === "frames") {
    return [
      { value: "0", label: t("Auto") },
      ...[24, 48, 96, 120, 240].map((value) => ({ value: String(value), label: String(value) })),
    ];
  }

  if (field.name === "seed") {
    return [
      { value: "0", label: t("Random") },
      ...[1001, 2026, 4096, 12345].map((value) => ({ value: String(value), label: String(value) })),
    ];
  }

  const min = field.min ?? 1;
  const max = field.max ?? Math.max(min, Number(field.defaultValue) || min);
  const count = Math.min(12, Math.max(1, Math.floor(max - min + 1)));
  return Array.from({ length: count }, (_, index) => {
    const value = min + index;
    return { value: String(value), label: String(value) };
  });
}

function generatorFieldColumnClass(kind: NonNullable<ModelConfig["generator"]>["kind"], field: ModelGeneratorField) {
  if (kind === "image") {
    if (field.name === "size") return "sm:col-span-3";
    return "sm:col-span-3";
  }
  if (kind === "video") {
    return "sm:col-span-3";
  }
  return "sm:col-span-3";
}

function formatUploadedSize(size: number) {
  if (!Number.isFinite(size) || size <= 0) return "0 KB";
  if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  return `${Math.max(1, Math.round(size / 1024))} KB`;
}

function formatSavings(flatkey: string, official: string) {
  const flatkeyPrice = parsePrice(flatkey);
  const officialPrice = parsePrice(official);
  if (!flatkeyPrice || !officialPrice || flatkeyPrice >= officialPrice) return "0%";
  return `${Math.round(((officialPrice - flatkeyPrice) / officialPrice) * 100)}%`;
}

function parsePrice(value: string) {
  const match = value.match(/[\d.]+/);
  if (!match) return null;
  const parsed = Number(match[0]);
  return Number.isFinite(parsed) ? parsed : null;
}

// Example gallery above the workbench. Selecting a card replays the request
// that produced it -- prompt, parameters, and the reference assets it used --
// so a visitor can see how an output was built before editing it themselves.
function ExamplePicker(props: {
  examples: readonly MediaExample[];
  modelName: string;
  selected: number;
  onSelect: (index: number) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  if (props.examples.length === 0) return null;

  return (
    <div data-model-example-picker="true" className="border-b border-slate-200 px-5 pt-4 pb-4 dark:border-white/10">
      <div className="mb-3">
        <h2 className="text-sm font-semibold tracking-tight text-[#20222a] dark:text-white/90">
          {props.t("{{model}} prompt examples", { model: props.modelName })}
        </h2>
        <p className="mt-0.5 text-[11px] font-medium text-[#7b8494] dark:text-white/48">
          {props.t("Explore different use cases and parameter configurations")}
        </p>
      </div>
      <div className="flex flex-wrap gap-2.5">
        {props.examples.map((example, index) => {
          const isActive = index === props.selected;
          return (
            <button
              key={example.poster}
              type="button"
              onClick={() => props.onSelect(index)}
              aria-pressed={isActive}
              data-active-example={isActive ? "true" : undefined}
              className={`relative size-[76px] shrink-0 overflow-hidden rounded-lg border-2 transition active:scale-[0.97] ${
                isActive
                  ? "border-blue-500 shadow-[0_10px_26px_-16px_rgba(37,99,235,.8)]"
                  : "border-transparent opacity-80 hover:opacity-100"
              }`}
            >
              <Image src={example.poster} alt="" fill sizes="76px" className="object-cover" />
              {example.video ? (
                <span className="absolute bottom-1.5 left-1.5 rounded bg-black/62 px-1.5 py-0.5 text-[9px] font-bold tracking-wide text-white uppercase">
                  {props.t("Video")}
                </span>
              ) : null}
              {isActive ? <span className="absolute top-1.5 right-1.5 size-2.5 rounded-full bg-blue-500 ring-2 ring-white" /> : null}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function OutputPreview(props: {
  modelName: string;
  prompt: string;
  kind: "image" | "video" | "audio";
  images: readonly MediaExample[];
  selected?: number;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const primary =
    props.images[props.selected ?? 0] ??
    props.images[0] ?? { poster: "/assets/prompts/awesome-images/sports-shoe.png" };

  if (props.kind === "video" && primary?.video) {
    return (
      <div
        data-model-output-video="true"
        className="relative aspect-video min-w-0 overflow-hidden rounded-xl border border-slate-200 bg-[#10131a] shadow-sm dark:border-white/10"
      >
        <AutoplayVideo
          className="aspect-video h-full w-full bg-[#10131a] object-cover"
          poster={primary.poster}
          src={primary.video}
          t={props.t}
        />
      </div>
    );
  }

  return (
    <div className={props.kind === "video" ? "overflow-hidden rounded-[1.35rem] border border-black/10 bg-[#10131a] p-2 text-white shadow-[0_18px_42px_-32px_rgba(15,15,18,.8)]" : "overflow-hidden rounded-[1.35rem] border border-black/10 bg-white p-2 text-[#0B0B0F] shadow-[0_18px_42px_-34px_rgba(76,29,149,.65)]"}>
      <div className={`relative overflow-hidden rounded-[1.05rem] ${props.kind === "video" ? "aspect-video bg-[#171b24]" : "aspect-[16/10] bg-[#11131a]"}`}>
        {props.kind === "video" && primary?.video ? (
          <AutoplayVideo
            className="h-full w-full object-cover"
            poster={primary.poster}
            src={primary.video}
            t={props.t}
          />
        ) : (
          <>
            <Image
              src={primary.poster}
              alt=""
              fill
              sizes="(min-width: 1280px) 620px, (min-width: 1024px) 56vw, 100vw"
              className="object-cover"
            />
          </>
        )}
      </div>
      <div className={`grid gap-1 px-2 pt-3 pb-1 text-xs leading-5 ${props.kind === "video" ? "" : "text-[#3f3d46] dark:text-white/74"}`}>
        <div className="flex flex-wrap items-center gap-2">
          <span className={`rounded-full px-2.5 py-1 text-[10px] font-semibold ${props.kind === "video" ? "bg-white/10 text-white/78" : "bg-violet-500/10 text-violet-700 dark:text-violet-300"}`}>
            {props.t("Example output")}
          </span>
          <b className="min-w-0 truncate">{props.modelName}</b>
        </div>
        <span className={props.kind === "video" ? "text-white/72" : "text-[#706a74]"}>{props.prompt}</span>
      </div>
    </div>
  );
}

function GeneratedExamplesCarousel(props: {
  examples: readonly MediaExample[];
  kind: "image" | "video" | "audio";
  modelName: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const [activeIndex, setActiveIndex] = useState(0);
  const activeExample = props.examples[activeIndex] ?? props.examples[0];
  const total = props.examples.length;
  const hasMultiple = total > 1;
  const goToPrevious = () => setActiveIndex((current) => (current + total - 1) % total);
  const goToNext = () => setActiveIndex((current) => (current + 1) % total);

  if (!activeExample) return null;

  return (
    <figure className="min-w-0 overflow-hidden rounded-2xl border border-black/10 bg-white shadow-sm">
      <div className="relative overflow-hidden bg-[#10131a]">
        <div className={`relative w-full ${props.kind === "video" ? "aspect-video" : "aspect-[16/10]"}`}>
          {activeExample.video ? (
            <AutoplayVideo
              className="h-full w-full object-cover"
              poster={activeExample.poster}
              src={activeExample.video}
              t={props.t}
            />
          ) : (
            <Image
              src={activeExample.poster}
              alt=""
              fill
              sizes="(min-width: 1280px) 820px, (min-width: 1024px) 62vw, 100vw"
              className="object-cover"
            />
          )}
        </div>
        {activeExample.video ? (
          <div className="pointer-events-none absolute top-3 left-3 inline-flex items-center gap-1 rounded-full bg-black/65 px-2.5 py-1 text-[10px] font-extrabold text-white backdrop-blur">
            <Play className="size-3 fill-current" />
            {props.t("Preview")}
          </div>
        ) : null}
        <div className="pointer-events-none absolute right-3 bottom-3 rounded-full bg-black/65 px-3 py-1.5 text-[11px] font-extrabold text-white backdrop-blur">
          {props.t("Example {{index}} of {{total}}", {
            index: String(activeIndex + 1),
            total: String(total),
          })}
        </div>
        {hasMultiple ? (
          <div className="absolute inset-y-0 right-3 left-3 flex items-center justify-between">
            <button
              type="button"
              onClick={goToPrevious}
              className="grid size-10 place-items-center rounded-full bg-black/58 text-white shadow-sm backdrop-blur transition hover:bg-black/75"
              aria-label={props.t("Previous example")}
            >
              <ArrowLeft className="size-4" />
            </button>
            <button
              type="button"
              onClick={goToNext}
              className="grid size-10 place-items-center rounded-full bg-black/58 text-white shadow-sm backdrop-blur transition hover:bg-black/75"
              aria-label={props.t("Next example")}
            >
              <ArrowRight className="size-4" />
            </button>
          </div>
        ) : null}
      </div>
      <figcaption className="px-4 py-3 text-xs font-bold text-[#4b4a52]">
        {props.t("Generated with {{model}}", { model: props.modelName })} #{activeIndex + 1}
      </figcaption>
      {hasMultiple ? (
        <div className="grid grid-cols-3 gap-2 border-t border-black/10 bg-[#fbfaff] p-3">
          {props.examples.map((example, index) => (
            <button
              key={example.video ?? example.poster}
              type="button"
              onClick={() => setActiveIndex(index)}
              className={`relative aspect-video overflow-hidden rounded-xl border bg-[#10131a] transition ${
                activeIndex === index ? "border-[#7c3aed] ring-2 ring-[#7c3aed]/18" : "border-black/10 hover:border-[#7c3aed]/45"
              }`}
              aria-label={props.t("Example {{index}} of {{total}}", {
                index: String(index + 1),
                total: String(total),
              })}
            >
              <Image
                src={example.poster}
                alt=""
                fill
                sizes="(min-width: 1024px) 90px, 30vw"
                className="object-cover"
              />
              {example.video ? (
                <span className="absolute top-1.5 left-1.5 grid size-5 place-items-center rounded-full bg-black/62 text-white">
                  <Play className="size-2.5 fill-current" />
                </span>
              ) : null}
            </button>
          ))}
        </div>
      ) : null}
    </figure>
  );
}

// Activity: this model's daily call volume, mirroring the console dashboard's
// quota-distribution chart. The console reads /api/data, which is admin/user
// scoped; the public page reads the keyed /api/website/model-usage feed, which
// returns call counts only -- no quota, users, or channels.
//
// Renders nothing when there is no series: a chart of zeroes says less than no
// section at all, and a model can be new, unused, or on a deployment with no
// WEBSITE_METRICS_KEY configured.
// Performance: how this model actually behaves under real traffic, and how that
// compares with everything else in the catalog. Four bare numbers say little on
// their own -- "99.9% uptime" only means something once you know where the rest
// of the catalog sits -- so each metric carries its percentile among peers.
//
// Defaults match the /models directory (DEFAULT_HEALTH_*), so a model with thin
// traffic reads the same figure here as in the listing rather than an em dash.
function ModelPerformanceSection(props: {
  modelId: string;
  successRate: number | undefined;
  ttftMs: number | undefined;
  latencyMs?: number;
  throughput?: number;
  requests?: number;
  peers: HomePerfSummary[];
  trend: HomeTrendPoint[];
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  // Percentile against every model with a comparable reading. `lowerIsBetter`
  // flips the comparison for latency, where a small number is the good one.
  const percentile = (
    value: number | undefined,
    pick: (peer: HomePerfSummary) => number | undefined,
    lowerIsBetter = false
  ): number | null => {
    if (value == null || !Number.isFinite(value)) return null;
    const values = props.peers
      .map(pick)
      .filter((entry): entry is number => entry != null && Number.isFinite(entry) && entry > 0);
    if (values.length < 4) return null;
    const beaten = values.filter((entry) => (lowerIsBetter ? entry > value : entry < value)).length;
    return Math.round((beaten / values.length) * 100);
  };

  const stats = [
    {
      icon: <ShieldCheck className="size-4" />,
      label: props.t("Uptime"),
      value: formatHealthSuccessRate(props.successRate),
      hint: props.t("Successful requests"),
      rank: percentile(props.successRate, (peer) => peer.success_rate),
      accent: "text-emerald-600 dark:text-emerald-400",
      surface: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
    },
    {
      icon: <Timer className="size-4" />,
      label: props.t("Latency"),
      value: formatLatencyMs(props.ttftMs),
      hint: props.t("Time to first token"),
      rank: percentile(props.ttftMs, (peer) => peer.avg_ttft_ms, true),
      accent: "text-blue-600 dark:text-blue-400",
      surface: "bg-blue-500/10 text-blue-700 dark:text-blue-300",
    },
    {
      icon: <Zap className="size-4" />,
      label: props.t("Throughput"),
      value: formatThroughput(props.throughput),
      hint: props.t("Tokens per second"),
      rank: percentile(props.throughput, (peer) => peer.avg_tps),
      accent: "text-violet-600 dark:text-violet-400",
      surface: "bg-violet-500/10 text-violet-700 dark:text-violet-300",
    },
    {
      icon: <Layers3 className="size-4" />,
      label: props.t("Requests"),
      value: formatCallCount(props.requests),
      hint: props.t("Last 30 days"),
      rank: null,
      accent: "text-amber-600 dark:text-amber-400",
      surface: "bg-amber-500/10 text-amber-700 dark:text-amber-300",
    },
  ];

  return (
    <RevealSection id="performance" className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] border-y border-slate-200 bg-[#f8fafc] px-6 py-10 dark:border-white/10 dark:bg-white/[0.02]">
      <div className="mx-auto max-w-6xl">
        <FlatkeySectionHeading
          eyebrow={props.t("Performance")}
          title={props.t("Reliability over the last 30 days")}
          description={props.t("Measured on real Flatkey traffic across every upstream channel serving this model.")}
        />
        {/* Stats and trend share one surface: they are the same measurement read
            two ways, and separate cards made them look unrelated. */}
        <div className="mt-5 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_20px_50px_-42px_rgba(24,14,38,0.4)] dark:border-white/10 dark:bg-white/[0.04]">
          <div className="grid divide-y divide-slate-200 sm:grid-cols-2 sm:divide-y-0 lg:grid-cols-4 dark:divide-white/10">
            {stats.map((stat, index) => (
              <div
                key={stat.label}
                className={`p-5 ${index % 2 === 1 ? "sm:border-l sm:border-slate-200 sm:dark:border-white/10" : ""} ${index === 2 ? "sm:border-t sm:border-slate-200 lg:border-t-0 lg:border-l lg:border-slate-200 sm:dark:border-white/10" : ""}`}
              >
                <div className="flex items-center gap-2">
                  <span className={`grid size-7 place-items-center rounded-lg ${stat.surface}`}>{stat.icon}</span>
                  <span className="text-[11px] font-bold tracking-[0.08em] text-muted-foreground uppercase">{stat.label}</span>
                </div>
                <div className={`mt-3 font-mono text-[28px] leading-none font-bold ${stat.accent}`}>{stat.value}</div>
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <span className="text-[11px] font-medium text-[#98a2b3] dark:text-white/44">{stat.hint}</span>
                  {stat.rank != null && stat.rank >= 50 ? (
                    <span className="rounded-full bg-slate-500/8 px-2 py-0.5 text-[10px] font-bold text-[#5f6673] dark:bg-white/[0.07] dark:text-white/62">
                      {props.t("Top {{percent}}% of models", { percent: String(Math.max(1, 100 - stat.rank)) })}
                    </span>
                  ) : null}
                </div>
              </div>
            ))}
          </div>
          {props.latencyMs != null && Number.isFinite(props.latencyMs) ? (
            <div className="border-t border-slate-200 bg-[#fbfcff] px-5 py-3 text-[11px] font-medium text-[#6a7280] dark:border-white/10 dark:bg-white/[0.02] dark:text-white/54">
              {props.t("Full request completes in {{duration}} on average; the figure above is time to first token.", {
                duration: formatLatencyMs(props.latencyMs),
              })}
            </div>
          ) : null}
          <UptimeTrend points={props.trend} t={props.t} />
        </div>
      </div>
    </RevealSection>
  );
}

// Uptime as an area chart rather than DailyHealthBars: that component is built
// for a 56px cell on the home page, where every bar sits near full height by
// design. Scaled up it reads as a solid green block -- the variation that makes
// a trend worth showing is exactly what it flattens away.
//
// Here the y-axis starts just below the worst day, so a dip from 99.9% to 99.2%
// is visible instead of being lost against a 0-100 scale.
function UptimeTrend(props: {
  points: HomeTrendPoint[];
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const days = useMemo(() => {
    const byDay = new Map<string, number[]>();
    for (const point of props.points) {
      if (!Number.isFinite(point.success_rate)) continue;
      const key = new Date(point.ts * 1000).toISOString().slice(0, 10);
      byDay.set(key, [...(byDay.get(key) ?? []), point.success_rate]);
    }
    return [...byDay.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, values]) => ({ key, rate: values.reduce((s, v) => s + v, 0) / values.length }));
  }, [props.points]);

  if (days.length < 2) return null;

  const rates = days.map((day) => day.rate);
  const low = Math.min(...rates);
  const high = Math.max(...rates);
  // Pad the band so a perfectly flat series still draws mid-box rather than
  // pinned to an edge.
  const floor = Math.max(0, Math.min(low - 0.15, 99.4));
  const ceiling = Math.max(high + 0.05, floor + 0.3);
  const span = ceiling - floor;

  const x = (index: number) => (index / (days.length - 1)) * 100;
  const y = (rate: number) => 100 - ((rate - floor) / span) * 100;
  const line = days.map((day, index) => `${index === 0 ? "M" : "L"}${x(index).toFixed(2)},${y(day.rate).toFixed(2)}`).join(" ");
  const area = `${line} L100,100 L0,100 Z`;
  const latest = days[days.length - 1];

  return (
    <div className="border-t border-slate-200 bg-[#fbfcff] p-5 dark:border-white/10 dark:bg-white/[0.02]">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm font-semibold">{props.t("Successful inference trend")}</span>
        <span className="inline-flex items-center gap-1.5 text-[11px] font-semibold text-muted-foreground">
          <span className="size-1.5 rounded-full bg-emerald-500" />
          {props.t("Daily uptime")}
        </span>
      </div>
      <div className="relative h-28">
        <svg
          viewBox="0 0 100 100"
          preserveAspectRatio="none"
          className="h-full w-full overflow-visible"
          role="img"
          aria-label={props.t("Successful inference trend")}
        >
          <defs>
            <linearGradient id="fk-uptime-fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="rgb(16 185 129)" stopOpacity="0.28" />
              <stop offset="100%" stopColor="rgb(16 185 129)" stopOpacity="0.02" />
            </linearGradient>
          </defs>
          {[0, 50, 100].map((offset) => (
            <line key={offset} x1="0" y1={offset} x2="100" y2={offset} stroke="currentColor" strokeWidth="0.4" className="text-slate-200 dark:text-white/10" vectorEffect="non-scaling-stroke" />
          ))}
          <path d={area} fill="url(#fk-uptime-fill)" />
          <path
            d={line}
            fill="none"
            stroke="rgb(16 185 129)"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            vectorEffect="non-scaling-stroke"
          />
          <circle cx={x(days.length - 1)} cy={y(latest.rate)} r="3" fill="rgb(16 185 129)" vectorEffect="non-scaling-stroke" />
        </svg>
        <span className="pointer-events-none absolute top-0 right-0 rounded-md bg-emerald-500/10 px-2 py-0.5 font-mono text-[11px] font-bold text-emerald-700 dark:text-emerald-300">
          {formatHealthSuccessRate(latest.rate)}
        </span>
      </div>
      <div className="mt-2 flex justify-between text-[10px] font-medium text-[#98a2b3] dark:text-white/40">
        <span>{days[0].key.slice(5)}</span>
        <span>{latest.key.slice(5)}</span>
      </div>
    </div>
  );
}

function ModelActivitySection(props: {
  modelId: string;
  usage: ModelUsage | null;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const points = props.usage?.points ?? [];
  if (points.length === 0) return null;

  const peak = Math.max(...points.map((point) => point.count), 1);
  const busiest = points.reduce((best, point) => (point.count > best.count ? point : best), points[0]);
  const average = Math.round(points.reduce((sum, point) => sum + point.count, 0) / points.length);
  const averageHeight = (average / peak) * 100;

  return (
    <RevealSection id="activity" className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] bg-white px-6 py-10 dark:bg-white/[0.02]">
      <div className="mx-auto max-w-6xl">
        <FlatkeySectionHeading
          eyebrow={props.t("Activity")}
          title={props.t("Daily {{model}} requests on Flatkey", { model: props.modelId })}
          description={
            props.usage?.placeholder
              ? props.t("Sample shape shown while live telemetry is being connected.")
              : props.t("Request volume routed through Flatkey over the last 30 days.")
          }
        />
        <div className="mt-5 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_20px_50px_-42px_rgba(24,14,38,0.4)] dark:border-white/10 dark:bg-white/[0.04]">
          <div className="grid gap-4 p-5 sm:grid-cols-3">
            <ActivityStat label={props.t("Total requests")} value={formatCallCount(props.usage?.total)} />
            <ActivityStat label={props.t("Daily average")} value={formatCallCount(average)} />
            <ActivityStat label={props.t("Busiest day")} value={formatUsageDate(busiest.date)} sub={formatCallCount(busiest.count)} />
          </div>
          <div className="border-t border-slate-200 bg-[#fbfcff] px-5 pt-6 pb-4 dark:border-white/10 dark:bg-white/[0.02]">
            <div className="relative flex h-28 items-end gap-[3px]">
              {/* Average line inside the plot box, so its percentage is relative
                  to the bars rather than to the section's padding. It gives a
                  tall bar something to be judged against. */}
              <div
                className="pointer-events-none absolute inset-x-0 border-t border-dashed border-violet-400/55"
                style={{ bottom: `${averageHeight}%` }}
              />
              {points.map((point) => {
                const isPeak = point.date === busiest.date;
                return (
                  <div
                    key={point.date}
                    title={`${formatUsageDate(point.date)} · ${formatCallCount(point.count)}`}
                    className={`min-w-0 flex-1 rounded-t-[3px] transition hover:opacity-100 ${
                      isPeak
                        ? "bg-gradient-to-t from-violet-600 to-violet-400"
                        : "bg-gradient-to-t from-violet-500/55 to-violet-400/75 opacity-80"
                    }`}
                    // Zero-count days keep a hairline so a gap reads as "no
                    // traffic" rather than as a missing bar.
                    style={{ height: `${Math.max(3, (point.count / peak) * 100)}%` }}
                  />
                );
              })}
            </div>
            <div className="mt-2.5 flex justify-between text-[10px] font-medium text-[#98a2b3] dark:text-white/40">
              <span>{formatUsageDate(points[0].date)}</span>
              <span className="text-violet-500/80">{props.t("Daily average")}: {formatCallCount(average)}</span>
              <span>{formatUsageDate(points[points.length - 1].date)}</span>
            </div>
          </div>
        </div>
      </div>
    </RevealSection>
  );
}

function ActivityStat(props: { label: string; value: string; sub?: string }) {
  return (
    <div>
      <div className="text-[11px] font-bold tracking-[0.08em] text-muted-foreground uppercase">{props.label}</div>
      <div className="mt-2 flex items-baseline gap-2">
        <span className="font-mono text-[26px] leading-none font-bold text-[#20222a] dark:text-white/90">{props.value}</span>
        {props.sub ? <span className="font-mono text-[12px] font-semibold text-violet-600 dark:text-violet-400">{props.sub}</span> : null}
      </div>
    </div>
  );
}

function formatUsageDate(unixSeconds: number): string {
  // UTC to match the bucketing the API does; a local-time render would shift
  // labels by a day for readers west of UTC.
  const date = new Date(unixSeconds * 1000);
  return `${date.getUTCMonth() + 1}/${date.getUTCDate()}`;
}

// Quick Start reproduces the console overview's integration section: the same
// four cards, each opening the same stepped dialog
// (web/default/src/features/dashboard/components/overview/integration-dialog.tsx).
//
// Two differences, both because this is the public page: the model is fixed to
// this page's model rather than picked from a dropdown, and there is no
// signed-in key, so samples read $FLATKEY_API_KEY from the environment.
type QuickStartId = "api" | "sdk" | "cli" | "agent";

function ModelQuickStart(props: {
  config: ModelConfig;
  locale: Locale;
  runHref: string;
  onRunClick: (event: MouseEvent<HTMLAnchorElement>) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const [openCard, setOpenCard] = useState<QuickStartId | null>(null);

  const cards: Array<{ id: QuickStartId; icon: ReactNode; title: string; body: string }> = [
    {
      id: "api",
      icon: <TerminalSquare className="size-5" />,
      title: props.t("API for developers"),
      body: props.t("Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language."),
    },
    {
      id: "sdk",
      icon: <Braces className="size-5" />,
      title: props.t("SDKs for developers"),
      body: props.t("Use the OpenAI SDK you already know — with Flatkey as the gateway."),
    },
    {
      id: "cli",
      icon: <Terminal className="size-5" />,
      title: props.t("Flatkey CLI"),
      body: props.t("Generate images and videos from your terminal. Let your AI assistant drive the workflow."),
    },
    {
      id: "agent",
      icon: <Bot className="size-5" />,
      title: props.t("Codex & Claude Code"),
      body: props.t("Connect your coding agent with one command, then use Flatkey from your existing projects."),
    },
  ];

  const active = cards.find((card) => card.id === openCard) ?? null;

  return (
    <RevealSection id="quick-start" className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] bg-[#f8fafc] px-6 py-12 dark:bg-white/[0.02]">
      <div className="mx-auto max-w-6xl">
        <div className="flex flex-col gap-1">
          <h2 className="text-lg font-semibold tracking-tight">{props.t("Start calling {{model}} in minutes", { model: props.config.displayName })}</h2>
          <p className="text-sm text-muted-foreground">{props.t("Pick an integration path — all four use the same account and model catalog.")}</p>
        </div>
        <div className="mt-4 grid gap-3 sm:grid-cols-2">
          {cards.map((card) => (
            <button
              key={card.id}
              type="button"
              onClick={() => setOpenCard(card.id)}
              className="rounded-xl border border-slate-200 bg-white p-5 text-left transition hover:border-violet-500/35 hover:shadow-[0_18px_44px_-34px_rgba(76,29,149,.55)] active:scale-[0.995] dark:border-white/10 dark:bg-white/[0.04]"
            >
              <span className="grid size-10 place-items-center rounded-lg bg-violet-500/10 text-violet-700 dark:text-violet-300">
                {card.icon}
              </span>
              <h3 className="mt-4 text-[15px] font-semibold text-[#20222a] dark:text-white/90">{card.title}</h3>
              <p className="mt-1.5 text-sm leading-6 text-muted-foreground">{card.body}</p>
            </button>
          ))}
        </div>
      </div>

      {active ? (
        <QuickStartDialog
          card={active}
          modelId={props.config.modelId}
          endpoint={props.config.generator?.endpoint ?? "/v1/chat/completions"}
          runHref={props.runHref}
          onRunClick={props.onRunClick}
          onClose={() => setOpenCard(null)}
          t={props.t}
        />
      ) : null}
    </RevealSection>
  );
}

function QuickStartDialog(props: {
  card: { id: QuickStartId; title: string; body: string };
  modelId: string;
  endpoint: string;
  runHref: string;
  onRunClick: (event: MouseEvent<HTMLAnchorElement>) => void;
  onClose: () => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const [language, setLanguage] = useState<SnippetLanguage>("curl");
  const [platform, setPlatform] = useState<AgentPlatform>(() =>
    detectAgentPlatform(typeof navigator === "undefined" ? "" : navigator.userAgent)
  );
  const kind = snippetKindForEndpoint(props.endpoint);
  const { onClose } = props;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    // The dialog scrolls internally; letting the page scroll behind it makes
    // the backdrop feel detached.
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [onClose]);

  // The second step's label follows the model kind: a video model is an async
  // two-call flow, not a single ready-to-run request, and saying so up front is
  // what makes the numbered steps in the sample legible.
  const stepTwoLabel =
    kind === "video"
      ? props.t("Step 2 · Submit a video task, then poll for the result")
      : kind === "image"
        ? props.t("Step 2 · Copy a ready-to-run image request")
        : props.t("Step 2 · Copy a ready-to-run request");

  const keyStep = (
    <QuickStartStep label={props.t("Step 1 · Create an API key")}>
      <a
        href={consoleUrl("/dashboard/keys")}
        className="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-[13px] font-semibold text-[#3f4652] transition hover:border-violet-500/35 hover:text-violet-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-white/72"
      >
        <KeyRound className="size-3.5" />
        {props.t("Open the console")}
      </a>
    </QuickStartStep>
  );

  // Portal to <body>: rendering in place would nest the dialog inside the
  // page's stacking contexts, where the sticky header (z-50) and the section
  // tab bar (z-30) paint over it.
  if (typeof document === "undefined") return null;

  return createPortal(
    <div
      role="dialog"
      aria-modal="true"
      aria-label={props.card.title}
      data-quick-start-dialog={props.card.id}
      className="fk-quick-start-overlay fixed inset-0 z-[999] grid place-items-center overflow-y-auto bg-black/55 px-4 py-8 backdrop-blur-sm"
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="my-auto w-full max-w-3xl rounded-2xl border border-slate-200 bg-white p-6 shadow-2xl dark:border-white/10 dark:bg-[#0d1017]">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h3 className="text-lg font-semibold tracking-tight">{props.card.title}</h3>
            <p className="mt-1 text-sm text-muted-foreground">{props.card.body}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={props.t("Close")}
            className="grid size-8 shrink-0 place-items-center rounded-lg text-[#6b7280] transition hover:bg-slate-500/10 hover:text-[#111827] dark:text-white/54"
          >
            <X className="size-4" />
          </button>
        </div>

        {props.card.id === "api" || props.card.id === "sdk" ? (
          <div className="mt-5 grid gap-4">
            {keyStep}
            <QuickStartStep label={props.card.id === "api" ? stepTwoLabel : props.t("Step 2 · Copy the SDK setup")}>
              <QuickStartTabs
                options={[
                  { value: "curl", label: "cURL" },
                  { value: "node", label: "Node.js" },
                  { value: "python", label: "Python" },
                ]}
                value={language}
                onChange={(value) => setLanguage(value as SnippetLanguage)}
              />
              {/* The model is this page's model — no picker, unlike the console
                  dialog, where the reader has a whole catalog to choose from. */}
              <div className="mt-2 text-[11px] font-semibold text-muted-foreground">
                {props.t("Model")}: <span className="font-mono">{props.modelId}</span>
              </div>
              <CodeBlock
                code={
                  props.card.id === "api"
                    ? buildApiSnippet(language, props.modelId, kind)
                    : buildSdkSnippet(language, props.modelId, kind)
                }
                t={props.t}
              />
            </QuickStartStep>
          </div>
        ) : null}

        {props.card.id === "cli" ? (
          <div className="mt-5 grid gap-4">
            <QuickStartStep label={props.t("Step 1 · Install the CLI")} hint={props.t("Copy the command below and paste it into your terminal.")}>
              <CodeBlock code={CLI_INSTALL_COMMAND} t={props.t} />
            </QuickStartStep>
            <QuickStartStep label={props.t("Step 2 · Sign in")} hint={props.t("Run this command in your terminal to connect your account.")}>
              <CodeBlock code={CLI_LOGIN_COMMAND} t={props.t} />
            </QuickStartStep>
            <QuickStartStep label={props.t("Step 3 · Start creating")} hint={props.t("Tell your AI assistant what you want to make.")}>
              <CodeBlock code={props.t('Use Flatkey CLI to generate a "beautiful sunrise" picture.')} t={props.t} />
            </QuickStartStep>
          </div>
        ) : null}

        {props.card.id === "agent" ? (
          <div className="mt-5 grid gap-4">
            {keyStep}
            <QuickStartStep
              label={props.t("Step 2 · Install Flatkey for your system")}
              hint={props.t("Paste the next line into your terminal to integrate Flatkey in seconds.")}
            >
              <QuickStartTabs
                options={[
                  { value: "mac", label: "macOS" },
                  { value: "linux", label: "Linux" },
                  { value: "windows", label: "Windows" },
                ]}
                value={platform}
                onChange={(value) => setPlatform(value as AgentPlatform)}
              />
              <CodeBlock code={buildAgentInstallCommand(platform, SITE_ORIGIN)} t={props.t} />
            </QuickStartStep>
          </div>
        ) : null}

        <div className="mt-5 flex justify-end">
          <a
            href={props.runHref}
            onClick={props.onRunClick}
            className="flatkey-hero-cta inline-flex min-h-10 items-center gap-2 rounded-lg px-4 text-[13px] font-semibold"
          >
            <WandSparkles className="size-4" />
            {props.t("Open in console")}
          </a>
        </div>
      </div>
    </div>,
    document.body
  );
}

function QuickStartStep(props: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="grid gap-2">
      <div className="text-sm font-semibold">{props.label}</div>
      {props.hint ? <p className="text-sm leading-relaxed text-muted-foreground">{props.hint}</p> : null}
      <div>{props.children}</div>
    </div>
  );
}

function QuickStartTabs(props: {
  options: Array<{ value: string; label: string }>;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="inline-flex rounded-lg bg-slate-500/8 p-1 dark:bg-white/[0.06]">
      {props.options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => props.onChange(option.value)}
          aria-pressed={props.value === option.value}
          className={`rounded-md px-3 py-1.5 text-[12px] font-semibold transition ${
            props.value === option.value
              ? "bg-white text-[#20222a] shadow-sm dark:bg-white/12 dark:text-white"
              : "text-[#5f6673] hover:text-[#20222a] dark:text-white/58 dark:hover:text-white"
          }`}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

function CodeBlock(props: { code: string; t: (key: string, vars?: Record<string, string>) => string }) {
  const [copied, setCopied] = useState(false);

  return (
    <div className="group relative mt-2">
      <button
        type="button"
        onClick={() => {
          navigator.clipboard?.writeText(props.code).then(
            () => {
              setCopied(true);
              window.setTimeout(() => setCopied(false), 1600);
            },
            () => undefined
          );
        }}
        aria-label={props.t("Copy code")}
        className="absolute top-2 right-2 z-1 grid size-7 place-items-center rounded-md bg-white/10 text-white/80 opacity-0 transition group-hover:opacity-100 focus-visible:opacity-100 hover:bg-white/18"
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      </button>
      {/* Wrap instead of scrolling horizontally: the whole command should be
          readable in place, and long curl lines otherwise hide their tail. */}
      <pre className="rounded-xl border border-white/10 bg-[#10131a] p-4 pr-11 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap text-white/82">
        {props.code}
      </pre>
    </div>
  );
}


// Sample clips autoplay so the panel is alive on arrival, which browsers only
// permit while muted. Sound then switches on at the reader's first click
// anywhere on the page -- that gesture satisfies the autoplay policy, so the
// clip becomes audible without them hunting for a control. The explicit button
// stays for turning it back off, and because Seedance generates its own audio a
// permanently silent loop would misrepresent the model as video-only.
function AutoplayVideo(props: {
  className: string;
  poster: string;
  src: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const ref = useRef<HTMLVideoElement>(null);
  const [muted, setMuted] = useState(true);
  // Set once the reader mutes deliberately, so the first-gesture handler does
  // not immediately undo their choice.
  const userChoseMuted = useRef(false);

  useEffect(() => {
    const unmuteOnFirstGesture = () => {
      const video = ref.current;
      if (!video || userChoseMuted.current) return;
      video.muted = false;
      setMuted(false);
      if (video.paused) void video.play().catch(() => undefined);
    };
    window.addEventListener("pointerdown", unmuteOnFirstGesture, { once: true });
    window.addEventListener("keydown", unmuteOnFirstGesture, { once: true });
    return () => {
      window.removeEventListener("pointerdown", unmuteOnFirstGesture);
      window.removeEventListener("keydown", unmuteOnFirstGesture);
    };
  }, []);

  const toggleSound = () => {
    const video = ref.current;
    if (!video) return;
    const next = !muted;
    video.muted = next;
    setMuted(next);
    userChoseMuted.current = next;
    if (!next && video.paused) void video.play().catch(() => undefined);
  };

  return (
    <>
      <video
        ref={ref}
        className={props.className}
        autoPlay
        controls
        loop
        muted
        playsInline
        poster={props.poster}
        preload="metadata"
        src={props.src}
      />
      <button
        type="button"
        onClick={toggleSound}
        aria-pressed={!muted}
        // Sits clear of the native controls bar at the bottom.
        className="absolute top-3 right-3 z-10 inline-flex items-center gap-1.5 rounded-lg bg-black/55 px-2.5 py-1.5 text-[11px] font-semibold text-white/90 backdrop-blur-sm transition hover:bg-black/70"
      >
        {muted ? <VolumeX className="size-3.5" /> : <Volume2 className="size-3.5" />}
        {muted ? props.t("Sound off") : props.t("Sound on")}
      </button>
    </>
  );
}

function ModelExamplesAndRelated(props: {
  config: ModelConfig;
  kind: ModelReadmeKind;
  examples: readonly MediaExample[];
  relatedModels: CatalogRelatedModel[];
  relatedTitle: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const visualKind = props.kind === "text" ? "text" : props.kind;
  return (
    <RevealSection id="related" className="relative z-10 scroll-mt-[var(--fk-model-section-scroll-margin)] border-y border-slate-200 bg-[#f8fafc] px-6 py-12 dark:border-white/10 dark:bg-white/[0.02]">
      <div className="mx-auto max-w-7xl">
        <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm dark:border-white/10 dark:bg-white/[0.04]">
          <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
            <div>
              <p className="text-xs font-bold tracking-widest text-blue-700 uppercase">{props.t("Related models")}</p>
              <h2 className="mt-1 text-xl font-bold tracking-tight">{props.relatedTitle}</h2>
            </div>
            <span className="text-xs font-semibold text-muted-foreground">{props.t("Swipe or scroll to compare")}</span>
          </div>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {props.relatedModels.slice(0, 8).map((model) => (
              <Link
                key={model.href}
                href={model.href}
                className="group overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm transition duration-200 hover:-translate-y-0.5 hover:border-blue-500/45 hover:shadow-[0_18px_44px_-30px_rgba(37,99,235,.55)] dark:border-white/10 dark:bg-white/[0.03]"
              >
                <RelatedModelVisual
                  modelName={model.name}
                  description={model.description}
                  label={model.name}
                />
                <div className="p-3">
                  <div className="text-[10px] font-bold tracking-widest text-blue-700 uppercase">
                    {model.sameProvider ? props.config.officialName : props.t("Model catalog")}
                  </div>
                  <h3 className="mt-1 truncate font-mono text-sm font-bold">{model.name}</h3>
                  <p className="mt-2 line-clamp-2 text-xs leading-5 text-muted-foreground">{model.description}</p>
                </div>
              </Link>
            ))}
          </div>
        </div>
      </div>
    </RevealSection>
  );
}

function RelatedModelsCarousel(props: {
  models: RelatedModelCard[];
  title: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  if (props.models.length === 0) return null;

  return (
    <div className="mt-10">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">
            {props.t("Related models")}
          </div>
          <h3 className="mt-2 text-2xl font-extrabold tracking-tight">{props.title}</h3>
        </div>
        <span className="text-xs font-bold text-[#706a74]">
          {props.t("Swipe or scroll to compare")}
        </span>
      </div>
      <div className="-mx-6 flex snap-x gap-3 overflow-x-auto px-6 pb-2 sm:-mx-8 sm:px-8 lg:-mx-10 lg:px-10">
        {props.models.map((model) => (
          <Link
            key={model.name}
            href={model.href}
            className="group grid min-h-36 min-w-[16rem] snap-start rounded-2xl border border-black/10 bg-white p-4 shadow-sm transition hover:border-[#7c3aed]/40 hover:shadow-[0_18px_44px_-34px_rgba(76,29,149,.55)] sm:min-w-[18rem]"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="grid size-9 place-items-center rounded-full bg-[#f4f0ff] text-sm font-extrabold text-[#6d28d9]">
                {model.name.slice(0, 1).toUpperCase()}
              </div>
              <ArrowRight className="size-4 text-[#8b8891] transition group-hover:translate-x-0.5 group-hover:text-[#4c1d95]" />
            </div>
            <div className="mt-4 min-w-0">
              <h4 className="truncate text-base font-extrabold text-[#17151d]">{model.name}</h4>
              <p className="mt-1 text-xs font-bold text-[#706a74]">{model.vendor}</p>
            </div>
            <div className="mt-4 flex flex-wrap gap-2">
              <span className="rounded-full bg-[#f4f0ff] px-2.5 py-1 text-[10px] font-extrabold text-[#6d28d9]">
                {model.kind}
              </span>
              <span className="rounded-full bg-[#f8fafc] px-2.5 py-1 text-[10px] font-extrabold text-[#64748b]">
                {model.price}
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}

function buildRelatedModelCards(
  config: ModelConfig,
  locale: Locale,
  t: (key: string, vars?: Record<string, string>) => string
): RelatedModelCard[] {
  const current = normalizeModelId(config.modelId);
  const seen = new Set<string>([current]);
  const kind = relatedModelKindLabel(config, t);
  const direct = config.modelIds
    .filter((modelId) => {
      const normalized = normalizeModelId(modelId);
      if (seen.has(normalized)) return false;
      seen.add(normalized);
      return true;
    })
    .map((modelId) => ({
      href: localizePath(`/models/${encodeURIComponent(modelId)}`, locale),
      name: modelId,
      vendor: config.officialName,
      kind,
      price: `${config.flatkeyPrice} ${t(config.priceUnit)}`,
      sameProvider: true,
    }));

  if (direct.length >= 2) return direct.slice(0, 8);

  const familyKind = config.generator?.kind;
  const configs = getModelLandingConfigs().filter((candidate) => candidate.slug !== config.slug);
  const fallbackCandidates = familyKind
    ? [
        ...configs.filter((candidate) => candidate.generator?.kind === familyKind),
        ...configs.filter((candidate) => candidate.generator && candidate.generator.kind !== familyKind),
        ...configs.filter((candidate) => !candidate.generator),
      ]
    : [
        ...configs.filter((candidate) => !candidate.generator),
        ...configs.filter((candidate) => candidate.generator),
      ];
  const fallback = fallbackCandidates
    .filter((candidate) => {
      const normalized = normalizeModelId(candidate.modelId);
      if (seen.has(normalized)) return false;
      seen.add(normalized);
      return true;
    })
    .map((candidate) => ({
      href: localizePath(`/models/${candidate.slug}`, locale),
      name: candidate.displayName,
      vendor: candidate.officialName,
      kind: relatedModelKindLabel(candidate, t),
      price: `${candidate.flatkeyPrice} ${t(candidate.priceUnit)}`,
      sameProvider: false,
    }));

  return [...direct, ...fallback].slice(0, 8);
}

function buildRelatedModelsTitle(
  config: ModelConfig,
  models: RelatedModelCard[],
  t: (key: string, vars?: Record<string, string>) => string
) {
  if (models.length > 0 && models.every((model) => model.sameProvider)) {
    return t("More AI models from {{provider}}", { provider: config.officialName });
  }
  return relatedModelsFallbackTitle(config, t);
}

// One phrase per modality rather than a "More {{kind}} models" template: the
// interpolated form reads badly once the label is itself a noun phrase.
function relatedModelsFallbackTitle(
  config: ModelConfig,
  t: (key: string, vars?: Record<string, string>) => string
) {
  if (config.generator?.kind === "video") return t("Other video generation models");
  if (config.generator?.kind === "audio") return t("Other audio models");
  if (config.generator?.kind === "image") return t("Other image generation models");
  return t("Other models worth comparing");
}

function relatedModelKindLabel(
  config: ModelConfig,
  t: (key: string, vars?: Record<string, string>) => string
) {
  if (config.generator?.kind === "video") return t("Text to Video");
  if (config.generator?.kind === "audio") return t("Audio");
  if (config.generator?.kind === "image") return t("Image to Image");
  return t("Text");
}

function FlatkeySectionHeading(props: { eyebrow: string; title: string; description?: string }) {
  return (
    <div className="max-w-2xl">
      <p className="text-muted-foreground mb-3 text-xs font-medium tracking-widest uppercase">{props.eyebrow}</p>
      <h2 className="text-2xl leading-tight font-bold tracking-tight md:text-3xl">{props.title}</h2>
      {props.description ? (
        <p className="text-muted-foreground mt-3 text-sm leading-7 md:text-base">{props.description}</p>
      ) : null}
    </div>
  );
}

function ModelHeroPricingRow(props: {
  config: ModelConfig;
  model: PricingModel | null;
  providerName: string;
  rows: FlatkeyPriceTableRow[];
  note: string;
  health: string;
  requests: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const rows = props.rows.slice(0, 2);
  const primaryRow = rows[0];
  const savings = primaryRow ? formatSavings(primaryRow.flatkey, primaryRow.official) : "—";
  // A model priced at the vendor's own rate (group ratio 1) would render a
  // "0%" saving. Drop the column instead of advertising no discount.
  const hasSavings = savings !== "—" && savings !== "0%";
  const hasRequests = props.requests !== "—";
  const columns = hasSavings
    ? "minmax(260px,1.5fr) minmax(170px,0.9fr) minmax(170px,0.9fr) minmax(140px,0.7fr)"
    : "minmax(260px,1.7fr) minmax(180px,1fr) minmax(180px,1fr)";

  return (
    <div
      data-model-hero-price-row="true"
      title={props.note}
      className="mt-3 overflow-x-auto rounded-xl border border-[#E7E4EC] bg-white shadow-[0_18px_46px_-40px_rgba(24,14,38,0.34)] dark:border-white/10 dark:bg-white/[0.04]"
    >
      <div className="grid min-w-[900px]" style={{ gridTemplateColumns: columns }}>
        <div data-model-price-logo-cell="true" className="flex min-w-0 items-center gap-3 p-3">
          <HomeModelLogo
            iconKey={props.model?.icon ?? props.model?.vendor_icon}
            modelName={props.config.modelId}
            vendor={props.providerName}
            fallback={props.config.modelId.slice(0, 1)}
            surfaceSize={38}
            imageSize={24}
          />
          <div className="min-w-0">
            <div className="truncate text-sm font-bold">{props.config.displayName}</div>
            <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">{props.config.modelId}</div>
            <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] font-semibold text-[#5f6673] dark:text-white/60">
              <span>{props.providerName}</span>
              <span className="text-slate-300 dark:text-white/20">/</span>
              <span>{props.t("Model price comparison")}</span>
            </div>
          </div>
        </div>
        <ModelHeroPriceCell
          label={props.t("Flatkey price")}
          rows={rows}
          valueForRow={(row) => row.flatkey}
          valueClassName="text-emerald-700 dark:text-emerald-300"
          emptyLabel={props.t("Pricing data unavailable")}
        />
        <ModelHeroPriceCell
          label={props.t("Reference price")}
          rows={rows}
          valueForRow={(row) => row.official}
          valueClassName="text-[#68707c] line-through dark:text-white/54"
          emptyLabel={props.t("Pricing data unavailable")}
        />
        {hasSavings ? (
          <div data-model-savings-cell="true" className="border-l border-[#E7E4EC] p-3 dark:border-white/10">
            <div className="text-[10px] font-bold tracking-[0.08em] text-muted-foreground uppercase">{props.t("Pricing vs official")}</div>
            <div className="mt-2 font-mono text-lg font-bold text-emerald-700 dark:text-emerald-300">{savings}</div>
            <div className="mt-0.5 text-[11px] font-semibold text-muted-foreground">vs {props.providerName}</div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function ModelHeroPriceCell(props: {
  label: string;
  rows: FlatkeyPriceTableRow[];
  valueForRow: (row: FlatkeyPriceTableRow) => string;
  valueClassName: string;
  emptyLabel: string;
}) {
  return (
    <div className="border-l border-[#E7E4EC] p-3 dark:border-white/10">
      <div className="text-[10px] font-bold tracking-[0.08em] text-muted-foreground uppercase">{props.label}</div>
      <div className="mt-2 grid gap-1.5">
        {props.rows.length > 0 ? props.rows.map((row) => (
          <div key={row.label} className="grid min-w-0 gap-0.5">
            <div className="text-[11px] font-semibold text-[#6a7280] dark:text-white/52">{row.label}</div>
            <div className={`min-w-0 font-mono text-[13px] font-bold ${props.valueClassName}`}>
              {props.valueForRow(row)}
            </div>
          </div>
        )) : (
          <div className="text-sm font-semibold text-muted-foreground">{props.emptyLabel}</div>
        )}
      </div>
    </div>
  );
}

function FlatkeyMetricCard(props: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-violet-500/16 bg-white/72 p-5 shadow-[0_24px_70px_-56px_rgba(91,33,182,0.72)] backdrop-blur-sm dark:bg-white/[0.04]">
      <div className="text-muted-foreground text-xs font-medium tracking-widest uppercase">{props.label}</div>
      <div className="mt-3 font-mono text-2xl font-bold text-emerald-600 dark:text-emerald-400">{props.value}</div>
    </div>
  );
}

function FlatkeyPriceRow(props: { row: FlatkeyPriceTableRow; officialLabel: string; flatkeyLabel: string }) {
  return (
    <div className="rounded-xl border border-violet-500/12 bg-white/62 p-4 dark:bg-white/[0.03]">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <span className="font-mono text-sm font-semibold">{props.row.label}</span>
        <span className="rounded-full bg-emerald-500/10 px-2.5 py-1 text-[11px] font-bold text-emerald-700">
          {props.row.flatkey}
        </span>
      </div>
      <div className="grid gap-2">
        <PriceTrack label={props.flatkeyLabel} value={props.row.flatkey} percent={props.row.flatkeyPercent} kind="flatkey" />
        <PriceTrack label={props.officialLabel} value={props.row.official} percent={props.row.officialPercent} kind="official" />
      </div>
    </div>
  );
}

function PriceTrack(props: { label: string; value: string; percent: number; kind: "flatkey" | "official" }) {
  return (
    <div>
      <div className="mb-1 flex items-center justify-between gap-3 text-xs">
        <span className="text-muted-foreground">{props.label}</span>
        <span className={props.kind === "flatkey" ? "font-mono font-semibold text-emerald-700" : "font-mono text-muted-foreground line-through"}>
          {props.value}
        </span>
      </div>
      <span className="block h-2 overflow-hidden rounded-full bg-violet-500/10">
        <span
          className={props.kind === "flatkey" ? "block h-full rounded-full bg-emerald-500" : "block h-full rounded-full bg-violet-400/55"}
          style={{ width: `${props.percent}%` }}
        />
      </span>
    </div>
  );
}

function buildCatalogProviderRows(config: ModelConfig, liveModels: PricingModel[], providerName: string) {
  const current = normalizeModelId(config.modelId);
  const exactRows = liveModels.filter((model) => normalizeModelId(model.model_name) === current);
  const rows = exactRows.length > 0 ? exactRows : liveModels.slice(0, 1);
  if (rows.length === 0) {
    return [{
      provider: providerName,
      modelId: config.modelId,
      endpoint: config.generator?.endpoint ?? "/v1/chat/completions",
      status: "Live",
    }];
  }
  return rows.slice(0, 8).map((model) => ({
    provider: model.vendor_name ?? providerName,
    modelId: model.model_name,
    endpoint: model.supported_endpoint_types?.join(" · ") || config.generator?.endpoint || "/v1/chat/completions",
    status: model.availability_status || "Live",
  }));
}

// Price rows share the /models directory's source of truth: the pricing API's
// display_pricing contract (per-second, per-request, or token dimensions),
// resolved via resolveModelDisplayPrice. Reading model_price directly would
// print a per-second model's calculation base as a per-request price.
function buildFlatkeyPriceRows(
  config: ModelConfig,
  model: PricingModel | null,
  groupRatio: Record<string, number>,
  t: (key: string, vars?: Record<string, string>) => string
): { rows: FlatkeyPriceTableRow[]; note: string } {
  const note = t("Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.");
  if (!model) {
    return {
      note,
      rows: [
        {
          label: t("Flatkey price"),
          flatkey: `${config.flatkeyPrice} ${t(config.priceUnit)}`,
          official: `${config.officialPrice} ${t(config.priceUnit)}`,
          flatkeyPercent: 67,
          officialPercent: 100,
        },
      ],
    };
  }

  const rows = displayPriceDimensions(model).flatMap(([labelKey, dimension]) => {
    const price = resolveModelDisplayPrice(model, dimension, "plg", groupRatio);
    if (!price) return [];
    const official = price.configured ?? price.value;
    const unit = t(price.unit);
    // "from" is templated rather than concatenated: zh/ja place the qualifier
    // after the amount, so each locale owns the word order.
    const withFrom = (amount: string) => (price.from ? t("from {{price}}", { price: amount }) : amount);
    return [{
      label: t(labelKey),
      flatkey: withFrom(`${price.text} ${unit}`),
      official: withFrom(`${formatUsdPrice(official)} ${unit}`),
      flatkeyPercent: pricePercent(price.value, official),
      officialPercent: 100,
    }];
  });

  return { rows, note };
}

// Which price rows a model shows, keyed to how it bills. Per-second and
// per-request models collapse to one row; token models expand to input/output.
function displayPriceDimensions(model: PricingModel): Array<[string, DisplayPricingDimension]> {
  const kind = model.display_pricing?.billing_kind;
  if (kind === "per_second") return [["Video price", "second"]];
  if (kind === "request") return [["Request price", "request"]];
  if (!isTokenBasedModel(model)) return [["Request price", "request"]];
  return [
    ["Input /M", "input"],
    ["Output /M", "output"],
  ];
}

function buildCatalogRelatedModels(
  config: ModelConfig,
  locale: Locale,
  allModels: PricingModel[],
  groupRatio: Record<string, number>,
  t: (key: string, vars?: Record<string, string>) => string
) {
  const current = normalizeModelId(config.modelId);
  const provider = allModels.find((model) => normalizeModelId(model.model_name) === current)?.vendor_name ?? config.officialName;
  const family = getModelFamilyKey(config.modelId);
  const currentEndpoints = new Set((allModels.find((model) => normalizeModelId(model.model_name) === current)?.supported_endpoint_types ?? []).map(normalizeModelId));
  const currentModality = configModalityKey(config);
  const scoredRelated = allModels
    .filter((model) => normalizeModelId(model.model_name) !== current)
    .map((model) => {
      const endpointMatch = (model.supported_endpoint_types ?? []).some((endpoint) => currentEndpoints.has(normalizeModelId(endpoint)));
      const modalityMatch = modelModalityKey(model) === currentModality;
      const sameVendor = model.vendor_name === provider;
      const score =
        sameVendor && modalityMatch ? 0 :
        sameVendor ? 1 :
        getModelFamilyKey(model.model_name) === family ? 2 :
        endpointMatch ? 3 :
        modalityMatch ? 4 :
        5;
      return { model, score };
    });
  const preferredRelated = scoredRelated.filter((item) => item.score < 5);
  const liveRelated = preferredRelated
    .sort((a, b) => a.score - b.score || a.model.model_name.localeCompare(b.model.model_name, "en", { numeric: true }))
    .slice(0, 8)
    .map(({ model }): CatalogRelatedModel => ({
      href: localizePath(`/models/${encodeURIComponent(model.model_name)}`, locale),
      name: model.model_name,
      description: model.description || `${model.vendor_name ?? provider} · ${formatRelatedPrice(model, groupRatio)}`,
      sameProvider: model.vendor_name === provider,
    }));

  if (liveRelated.length > 0) {
    const sameProviderCount = liveRelated.filter((model) => model.sameProvider).length;
    return {
      title: sameProviderCount > liveRelated.length / 2
        ? t("More AI models from {{provider}}", { provider })
        : relatedModelsFallbackTitle(config, t),
      models: liveRelated,
    };
  }

  const fallback: CatalogRelatedModel[] = buildRelatedModelCards(config, locale, t).map((model) => ({
    href: model.href,
    name: model.name,
    description: `${model.vendor} · ${model.kind} · ${model.price}`,
    sameProvider: model.sameProvider,
  }));
  return {
    title: buildRelatedModelsTitle(config, fallback.map((model) => ({ ...model, vendor: "", kind: "", price: "", sameProvider: false })), t),
    models: fallback,
  };
}

function buildModalityLabels(
  config: ModelConfig,
  model: PricingModel | null,
  t: (key: string, vars?: Record<string, string>) => string
) {
  const endpoints = model?.supported_endpoint_types ?? [];
  const labels = new Set<string>();
  if (config.generator?.kind === "video" || endpoints.some((endpoint) => endpoint.includes("video"))) labels.add(t("Text to Video"));
  if (config.generator?.kind === "audio" || endpoints.some((endpoint) => /audio|music|sound|tts/.test(endpoint))) labels.add(t("Audio"));
  if (config.generator?.kind === "image" || endpoints.some((endpoint) => endpoint.includes("image"))) labels.add(t("Image to Image"));
  if (labels.size === 0) labels.add(t("Text"));
  return [...labels];
}

function buildModelPageProfile(
  config: ModelConfig,
  kind: ModelReadmeKind,
  providerName: string,
  fallbackDescription: string,
  t: (key: string, vars?: Record<string, string>) => string
): ModelPageProfile {
  const model = config.displayName;

  if (kind === "video") {
    return {
      kindLabel: t("Text to Video"),
      modelTypes: [t("Image to Video"), t("Reference-guided Video"), t("Short-form Video")],
      summary: config.summary
        ? t(config.summary)
        : t("{{model}} is a video generation model served through Flatkey for text-to-video, image-to-video, and production prompt testing. Use the form below to prepare the full request, including resolution, aspect ratio, duration, seed, audio, and reference metadata before opening the console.", { model }),
      playgroundDescription: t("Configure a real {{model}} video request here. The public page keeps generation disabled, then hands prompt, fields, and reference metadata to the console after sign-up or login.", { model }),
      heroImage: "/assets/model-pages/video-api-hero.png",
      waysTitle: t("Main ways to create with {{model}} API", { model }),
      ways: [
        {
          icon: <Video className="size-4" />,
          title: t("Text to video production"),
          body: t("Turn scene prompts into short clips with explicit camera, motion, duration, resolution, and aspect-ratio control."),
        },
        {
          icon: <Upload className="size-4" />,
          title: t("Reference-guided video workflows"),
          body: t("Prepare reference-image or first-frame guided drafts for product shots, character motion, storyboards, and social clips."),
        },
      ],
      valuesTitle: t("Why build video workflows on Flatkey"),
      values: [
        {
          icon: <KeyRound className="size-4" />,
          title: t("One console handoff for video drafts"),
          body: t("The page preserves prompt settings and request fields so users can continue in the console without rebuilding the job."),
        },
        {
          icon: <Settings2 className="size-4" />,
          title: t("Parameter clarity before spend"),
          body: t("Resolution, ratio, duration, seed, audio, and camera options are visible before any generation starts."),
        },
        {
          icon: <ShieldCheck className="size-4" />,
          title: t("Production API path"),
          body: t("Move from prompt preview to API usage with one Flatkey account and unified billing across media models."),
        },
      ],
    };
  }

  if (kind === "image") {
    return {
      kindLabel: t("Image"),
      modelTypes: [t("Text to Image"), t("Image to Image"), t("Reference Images")],
      summary: t("{{model}} is an image generation API for prompt-driven visuals, product mockups, reference-based variants, and creative asset production. Configure size, quality, output format, background, moderation, and reference metadata before opening the console.", { model }),
      playgroundDescription: t("Prepare a {{model}} image request here. The console receives the prompt, image options, output count, and reference metadata after login.", { model }),
      heroImage: "/assets/model-pages/image-api-hero.png",
      waysTitle: t("Main ways to create with {{model}} API", { model }),
      ways: [
        {
          icon: <ImageIcon className="size-4" />,
          title: t("Text to image generation"),
          body: t("Create product visuals, posters, thumbnails, ecommerce images, and campaign concepts from structured prompts."),
        },
        {
          icon: <Upload className="size-4" />,
          title: t("Reference image preparation"),
          body: t("Attach reference-image metadata for style, composition, and subject continuity before continuing to the console."),
        },
      ],
      valuesTitle: t("Where {{model}} adds the most value", { model }),
      values: [
        {
          icon: <Sparkles className="size-4" />,
          title: t("Campaign visual creation"),
          body: t("Generate many polished creative directions while keeping prompt and parameter history tied to the model page."),
        },
        {
          icon: <Settings2 className="size-4" />,
          title: t("Product image iteration"),
          body: t("Test sizes, formats, quality levels, and backgrounds before moving the request into production."),
        },
        {
          icon: <ShieldCheck className="size-4" />,
          title: t("Console-only execution"),
          body: t("The public page is a preview surface; real image generation starts only after sign-up or login."),
        },
      ],
    };
  }

  if (kind === "audio") {
    return {
      kindLabel: t("Audio"),
      modelTypes: [t("Video to Music"), t("Voice and Sound"), t("Audio Variants")],
      summary: t("{{model}} is an audio workflow model for video-to-music, narration, sound beds, and synchronized output variants. Configure duration, format, speech preservation, and output count before opening the console.", { model }),
      playgroundDescription: t("Prepare an audio request for {{model}} here. The console receives prompt, timing, format, source URL, and variant settings after authentication.", { model }),
      heroImage: "/assets/model-pages/audio-api-hero.png",
      waysTitle: t("Main ways to create with {{model}} API", { model }),
      ways: [
        {
          icon: <Music2 className="size-4" />,
          title: t("Video-to-music generation"),
          body: t("Use source timing and speech preservation settings to prepare music beds for product clips and short videos."),
        },
        {
          icon: <Timer className="size-4" />,
          title: t("Audio variant control"),
          body: t("Set duration, format, and output count for campaign tests, narration drafts, and sound-design alternatives."),
        },
      ],
      valuesTitle: t("Where {{model}} adds the most value", { model }),
      values: [
        {
          icon: <Video className="size-4" />,
          title: t("Product video soundtracks"),
          body: t("Prepare music that follows the visual timing without turning the public model page into a generator."),
        },
        {
          icon: <Settings2 className="size-4" />,
          title: t("Format-ready outputs"),
          body: t("Choose MP3, M4A, WAV, and output counts before continuing into the console."),
        },
        {
          icon: <KeyRound className="size-4" />,
          title: t("Unified media API access"),
          body: t("Route audio, video, image, and text model usage through one Flatkey account."),
        },
      ],
    };
  }

  return {
    kindLabel: t("Text"),
    modelTypes: [t("Chat"), t("Coding"), t("Agent workflows")],
    summary: fallbackDescription || t("{{model}} is a production text model from {{provider}} for chat, coding, long-context reasoning, tool workflows, and API-backed assistants through Flatkey-compatible access.", { model, provider: providerName }),
    playgroundDescription: t("Open the console with {{model}} selected. The prompt and request draft are preserved after sign-up or login without placing full content in the URL.", { model }),
    heroImage: "/assets/model-pages/text-api-hero.png",
    waysTitle: t("Main ways to build with {{model}} API", { model }),
    ways: [
      {
        icon: <Code2 className="size-4" />,
        title: t("Chat and agent backends"),
        body: t("Use the model for assistants, coding agents, search workflows, support automation, and internal tools."),
      },
      {
        icon: <FileText className="size-4" />,
        title: t("Long-context knowledge work"),
        body: t("Prepare prompts for codebase analysis, document processing, structured outputs, and technical generation."),
      },
    ],
    valuesTitle: t("Why build text workflows on Flatkey"),
    values: [
      {
        icon: <KeyRound className="size-4" />,
        title: t("One OpenAI-compatible key"),
        body: t("Keep SDK changes small while routing text model usage through a unified Flatkey account."),
      },
      {
        icon: <Layers3 className="size-4" />,
        title: t("Compare model families"),
        body: t("Use related model pages, live pricing, health, and catalog entries to choose the best model for each workload."),
      },
      {
        icon: <ShieldCheck className="size-4" />,
        title: t("Production controls"),
        body: t("Manage keys, quotas, logs, billing, and fallback model access from the console after authentication."),
      },
    ],
  };
}

function buildModelReadmeContent(
  config: ModelConfig,
  kind: ModelReadmeKind,
  t: (key: string, vars?: Record<string, string>) => string
) {
  const model = config.displayName;
  const modelId = config.modelId;
  const endpoint = config.generator?.endpoint ?? "/v1/chat/completions";
  const sharedAccessSteps = [
    t("Choose {{model}} on this page and adjust the prompt or parameters to match the workflow.", { model }),
    t("Click Open in console. Flatkey stores a short handoff id, not the full prompt or uploaded media in the URL."),
    t("After sign-up or login, the console opens the matching generation page and restores the complete request draft."),
  ];

  if (kind === "video") {
    return {
      capabilitiesTitle: t("Key features of {{model}} API", { model }),
      accessTitle: t("How to access {{model}} API on Flatkey", { model }),
      useCasesTitle: t("What you can build with {{model}} API", { model }),
      capabilities: [
        {
          icon: <Video className="size-4" />,
          title: t("Reference-guided video generation"),
          body: t("Use text prompts plus supported image or video references to control subject, style, and motion across short clips."),
        },
        {
          icon: <Settings2 className="size-4" />,
          title: t("Aspect ratio, duration, and resolution control"),
          body: t("Configure production parameters such as ratio, duration, resolution, seed, audio generation, and camera behavior before entering the console."),
        },
        {
          icon: <ShieldCheck className="size-4" />,
          title: t("Console-only execution"),
          body: t("The public page is a safe preview. Real generation starts only after the user signs in and runs the restored request from Flatkey."),
        },
      ],
      accessSteps: [
        ...sharedAccessSteps,
        t("Call {{endpoint}} with model {{modelId}} from your backend when the workflow is ready.", { endpoint, modelId }),
      ],
      useCases: [
        t("Image-to-video production"),
        t("Product launch and ecommerce clips"),
        t("UGC ads, social reels, and campaign variants"),
        t("Storyboard exploration before full production"),
      ],
    };
  }

  if (kind === "audio") {
    return {
      capabilitiesTitle: t("Key features of {{model}} API", { model }),
      accessTitle: t("How to access {{model}} API on Flatkey", { model }),
      useCasesTitle: t("What you can build with {{model}} API", { model }),
      capabilities: [
        {
          icon: <Music2 className="size-4" />,
          title: t("Voice, music, and sound workflow control"),
          body: t("Set the input prompt, media URL, duration, output format, and variants so the console draft matches the intended audio job."),
        },
        {
          icon: <Sparkles className="size-4" />,
          title: t("Studio-quality voiceover and narration"),
          body: t("Prepare voice, narration, music-bed, and sound-design prompts for ads, product videos, tutorials, and short-form content."),
        },
        {
          icon: <Timer className="size-4" />,
          title: t("Timing-aware video-to-music setup"),
          body: t("For video-to-music models, preserve speech and match audio length to the source clip before opening the console."),
        },
      ],
      accessSteps: [
        ...sharedAccessSteps,
        t("Call {{endpoint}} with model {{modelId}} once the audio request is ready for production.", { endpoint, modelId }),
      ],
      useCases: [
        t("Voiceover and narration drafts"),
        t("Music beds for short videos"),
        t("Podcast, tutorial, and product-demo audio"),
        t("Sound variants for campaign testing"),
      ],
    };
  }

  if (kind === "image") {
    const imageModel = normalizeModelId(modelId) === "gpt-image-2" ? "GPT Image-2" : model;
    return {
      capabilitiesTitle: t("Key features of {{model}} API", { model }),
      accessTitle: t("How to access {{model}} API on Flatkey", { model }),
      useCasesTitle: t("What you can build with {{model}} API", { model }),
      capabilities: [
        {
          icon: <ImageIcon className="size-4" />,
          title: t("Text to Image with {{model}} API", { model: imageModel }),
          body: t("Turn production prompts into image requests with size, quality, output format, background, and moderation controls."),
        },
        {
          icon: <Upload className="size-4" />,
          title: t("Reference image handoff"),
          body: t("Image references stay out of long URLs. The page records metadata now and leaves room for secure asset handoff in the console."),
        },
        {
          icon: <WandSparkles className="size-4" />,
          title: t("Creative variant setup"),
          body: t("Prepare multiple outputs, ecommerce scenes, thumbnails, ads, and style variants before committing spend."),
        },
      ],
      accessSteps: [
        ...sharedAccessSteps,
        t("Call {{endpoint}} with model {{modelId}} after the restored image request is ready.", { endpoint, modelId }),
      ],
      useCases: [
        t("Product mockups and ecommerce images"),
        t("Ad creative and campaign visuals"),
        t("Thumbnail and social post production"),
        t("Reference-guided image variations"),
      ],
    };
  }

  return {
    capabilitiesTitle: t("Key features of {{model}} API", { model }),
    accessTitle: t("How to access {{model}} API on Flatkey", { model }),
    useCasesTitle: t("What you can build with {{model}} API", { model }),
    capabilities: [
      {
        icon: <Code2 className="size-4" />,
        title: t("OpenAI-compatible chat completions"),
        body: t("Use familiar chat completion payloads with a Flatkey base URL, one API key, and unified usage tracking."),
      },
      {
        icon: <FileText className="size-4" />,
        title: t("Long-context reasoning and coding"),
        body: t("Prepare prompts for code generation, document analysis, agents, search, and production assistants."),
      },
      {
        icon: <Layers3 className="size-4" />,
        title: t("Streaming and tool workflows"),
        body: t("Move from prompt tests to streaming UIs, tool calls, structured outputs, and backend automation."),
      },
    ],
    accessSteps: [
      ...sharedAccessSteps,
      t("Call {{endpoint}} with model {{modelId}} from your app when the chat workflow is ready.", { endpoint, modelId }),
    ],
    useCases: [
      t("AI app backends"),
      t("Agent workflows"),
      t("Coding agents"),
      t("Long document analysis"),
    ],
  };
}

function configModalityKey(config: ModelConfig) {
  return config.generator?.kind ?? "text";
}

function modelModalityKey(model: PricingModel) {
  const endpoints = (model.supported_endpoint_types ?? []).join(" ").toLowerCase();
  const name = normalizeModelId(model.model_name);
  if (/audio|music|sound|tts|voice/.test(endpoints) || /(^|-)(audio|music|sound|sonilo|suno)(-|$)/.test(name)) return "audio";
  if (/video/.test(endpoints) || /(^|-)(video|seedance|kling|sora|veo|wan)(-|$)/.test(name)) return "video";
  if (/image/.test(endpoints) || /(^|-)(image|imagen|flux|dall-e)(-|$)/.test(name)) return "image";
  return "text";
}

function relatedVisualForModel(name: string, description: string) {
  const text = normalizeModelId(`${name} ${description}`);
  if (/(audio|music|sound|tts|voice|sonilo|suno)/.test(text)) return "/assets/model-pages/audio-api-hero.png";
  if (/(video|seedance|kling|sora|veo|wan|runway|minimax)/.test(text)) return "/assets/model-pages/video-api-hero.png";
  if (/(image|imagen|banana|flux|ideogram|gpt-image|dall-e|qwen-image|recraft)/.test(text)) return "/assets/model-pages/image-api-hero.png";
  return "/assets/model-pages/text-api-hero.png";
}

// Models with a generated card clip in public/assets/model-cards/. Each clip
// renders its own model id, so cards stay distinguishable; models without one
// fall back to the shared per-modality still.
//
// Regenerate with: node scripts/build-related-model-videos.mjs
const MODEL_CARD_CLIPS = new Set([
  "seedance-2-5",
  "minimax-h3",
  "grok-imagine-video",
  "grok-imagine-video-1-5",
  "veo-3-1-generate-preview",
  "veo-3-1-fast-generate-preview",
  "sonilo-video-to-music",
]);

function modelCardSlug(modelName: string): string {
  return modelName.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

function modelCardClip(modelName: string): { poster: string; video: string } | null {
  const slug = modelCardSlug(modelName);
  if (!MODEL_CARD_CLIPS.has(slug)) return null;
  return {
    poster: `/assets/model-cards/${slug}.png`,
    video: `/assets/model-cards/${slug}.mp4`,
  };
}

// Card thumbnail: plays its clip on hover and pauses on leave. Autoplaying every
// card at once would put a row of competing motion on the page, so playback is
// tied to pointer intent; without a clip it stays a still.
function RelatedModelVisual(props: { modelName: string; description: string; label: string }) {
  const clip = modelCardClip(props.modelName);

  return (
    <div className="relative aspect-video overflow-hidden bg-slate-950">
      {clip ? (
        <video
          className="h-full w-full object-cover transition duration-300 group-hover:scale-[1.03]"
          poster={clip.poster}
          preload="none"
          muted
          loop
          playsInline
          src={clip.video}
          // Ignore the play() rejection that fires when the pointer leaves
          // before the promise settles.
          onMouseEnter={(event) => void event.currentTarget.play().catch(() => undefined)}
          onMouseLeave={(event) => {
            event.currentTarget.pause();
            event.currentTarget.currentTime = 0;
          }}
        />
      ) : (
        <Image
          src={relatedVisualForModel(props.modelName, props.description)}
          alt=""
          fill
          sizes="(min-width: 1024px) 260px, 50vw"
          className="object-cover opacity-92 transition duration-300 group-hover:scale-[1.03]"
        />
      )}
      <span className="pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/78 to-transparent px-3 pt-6 pb-2">
        <span className="block truncate font-mono text-[11px] font-bold text-white">{props.label}</span>
      </span>
    </div>
  );
}

function buildModelDescription(
  config: ModelConfig,
  model: PricingModel | null,
  t: (key: string, vars?: Record<string, string>) => string
) {
  if (model?.description) return model.description;
  if (config.generator) {
    return t("{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.", {
      model: config.displayName,
    });
  }
  return t("{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.", {
    model: config.modelId,
  });
}

function buildModelFaq(config: ModelConfig, t: (key: string, vars?: Record<string, string>) => string) {
  const model = config.displayName;
  return [
    // Model questions first -- someone landing here is evaluating this model.
    {
      question: t("What is {{model}}?", { model }),
      answer: buildModelDescription(config, null, t),
    },
    {
      question: t("How much does {{model}} cost?", { model }),
      answer: t("Use the pricing section above for current Flatkey prices from our pricing API."),
    },
    ...config.faq.map((item) => ({ question: t(item.question), answer: t(item.answer) })),
    // Then the platform questions that decide whether they sign up at all.
    {
      question: t("Is the Flatkey API OpenAI-compatible?"),
      answer: t("Yes. Point base_url at Flatkey and keep your existing OpenAI SDK, request shapes, and streaming code."),
    },
    {
      question: t("How does billing work?"),
      answer: t("One balance covers every model — text, image, video, and audio. You are charged per request against live catalog pricing, with usage analytics and a single invoice."),
    },
    {
      question: t("Are there rate limits?"),
      answer: t("Limits are per account and scale with your plan. Request routing spreads traffic across available upstream channels for the model."),
    },
    {
      question: t("What happens to my prompts and generated files?"),
      answer: t("Requests are relayed to the upstream provider for the model you choose. Generated media is served through Flatkey so your keys and the upstream endpoint stay private."),
    },
  ];
}

function findRankingRow(rows: RankedModel[], modelId: string): RankedModel | null {
  const normalized = normalizeModelId(modelId);
  return rows.find((row) => normalizeModelId(row.model_name) === normalized) ?? null;
}

function extractRankingUsageSeries(rankings: RankingsData | null, modelId: string): number[] {
  const usage = rankings?.usage;
  if (!usage) return [];
  const index = usage.series.findIndex((name) => normalizeModelId(name) === normalizeModelId(modelId));
  if (index < 0) return [];
  return usage.days.map((day) => day.values[index] ?? 0).filter((value) => value > 0);
}

function displayRankingTokens(rawTokens: number): number {
  return rawTokens * TOKEN_DISPLAY_SCALE;
}

function bestVisibleGroup(model: PricingModel, groupRatio: Record<string, number>) {
  return getAvailableGroups(model, groupRatio)[0] ?? "default";
}

function formatRelatedPrice(model: PricingModel, groupRatio: Record<string, number>) {
  const group = bestVisibleGroup(model, groupRatio);
  return isTokenBasedModel(model)
    ? `${formatGroupTokenPrice(model, group, groupRatio, "input")} in`
    : formatGroupRequestPrice(model, group, groupRatio);
}

function pricePercent(value: number, official: number) {
  if (!Number.isFinite(value) || !Number.isFinite(official) || official <= 0 || value <= 0) return 0;
  return Math.round(Math.max(6, Math.min(100, (value / official) * 100)));
}

function formatModelDate(timestamp?: number) {
  if (!timestamp || !Number.isFinite(timestamp)) return "—";
  const millis = timestamp > 10_000_000_000 ? timestamp : timestamp * 1000;
  return new Date(millis).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

function inferContextValue(model: PricingModel | null) {
  const text = `${model?.tags ?? ""} ${model?.description ?? ""}`;
  const match = text.match(/\b(\d+(?:\.\d+)?)\s*(m|k)\s*(?:token|context|ctx|tokens)?\b/i);
  if (!match) return "—";
  return `${match[1]}${match[2].toUpperCase()}`;
}

function averageFinite(values: number[]) {
  const finite = values.filter((value) => Number.isFinite(value) && value > 0);
  return finite.length > 0 ? finite.reduce((sum, value) => sum + value, 0) / finite.length : undefined;
}

function clampNumber(value: number, min: number, max: number) {
  return Math.max(min, Math.min(max, value));
}

function RequestPreview(props: {
  config: ModelConfig;
  prompt: string;
  fieldValues: Record<string, string | number | boolean>;
  referenceImages?: ReferenceImageDraft[];
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const request = buildGeneratorRequest(props.config, props.prompt, props.fieldValues, props.referenceImages);
  return (
    <div className="mt-5 rounded-2xl border border-violet-500/16 bg-white/72 p-4 shadow-[0_24px_70px_-56px_rgba(91,33,182,0.72)] backdrop-blur-sm sm:p-5 dark:bg-white/[0.04]">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3 text-sm font-semibold text-[#2c2d33] dark:text-white/88">
        <span>{props.t("Request preview")}</span>
        <button
          type="button"
          className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-violet-500/16 bg-white/70 px-3 text-xs font-semibold text-[#4f4d56] hover:border-violet-500/32 hover:bg-violet-500/8 hover:text-violet-700 dark:bg-white/[0.04] dark:text-white/74"
        >
          <Copy className="size-3" />
          {props.t("Copy request")}
        </button>
      </div>
      <pre className="max-h-64 overflow-auto rounded-xl border border-white/10 bg-[#11131a] p-4 font-mono text-xs leading-6 text-white/78 shadow-inner sm:p-5">
        {JSON.stringify(request, null, 2)}
      </pre>
    </div>
  );
}

function PanelHeader(props: { title: string; right: string }) {
  return (
    <div className="mb-3 flex items-center justify-between gap-3">
      <h2 className="min-w-0 text-sm font-semibold tracking-tight text-[#20222a] dark:text-white/90">{props.title}</h2>
      <span className="shrink-0 rounded-full border border-violet-500/12 bg-violet-500/8 px-2.5 py-1 text-[11px] font-semibold text-violet-700 dark:text-violet-300">
        {props.right}
      </span>
    </div>
  );
}

function Pill(props: { label: string; value: string }) {
  return (
    <span className="rounded-xl border border-black/10 bg-white p-4 shadow-sm">
      <span className="block text-xs font-extrabold tracking-[0.12em] text-[#8b8891] uppercase">{props.label}</span>
      <b className="mt-1.5 block text-sm">{props.value}</b>
    </span>
  );
}

function StatCard(props: { value: string; label: string }) {
  return (
    <div className="rounded-2xl border border-black/10 bg-white p-6 shadow-sm">
      <b className="text-3xl font-extrabold">{props.value}</b>
      <div className="mt-2 text-sm font-medium text-[#77747d]">{props.label}</div>
    </div>
  );
}

function PriceBox(props: { label: string; value: string; muted?: boolean }) {
  return (
    <div className={`rounded-2xl border p-5 ${props.muted ? "border-dashed border-[#cbd5e1] bg-[#f8fafc]" : "border-[#d8c9ff] bg-[#fbfaff]"}`}>
      <div className={`text-xs font-extrabold tracking-[0.12em] uppercase ${props.muted ? "text-[#64748b]" : "text-[#7c3aed]"}`}>
        {props.label}
      </div>
      <div className={`mt-2 text-xl font-extrabold ${props.muted ? "text-[#475569]" : "text-[#0B0B0F]"}`}>{props.value}</div>
    </div>
  );
}

function ReasonCard(props: { icon: ReactNode; title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-black/10 bg-white p-6 shadow-sm">
      <div className="mb-5 grid size-10 place-items-center rounded-full bg-[#f4f0ff] text-[#7c3aed]">{props.icon}</div>
      <h3 className="text-base font-extrabold">{props.title}</h3>
      <p className="mt-2 text-base leading-7 text-[#6b6872]">{props.body}</p>
    </div>
  );
}

function GuideFact(props: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-black/10 bg-[#fbfaf7] p-4">
      <div className="text-[10px] font-extrabold tracking-[0.12em] text-[#8b8891] uppercase">{props.label}</div>
      <div className="mt-2 break-words text-sm font-extrabold">{props.value}</div>
    </div>
  );
}

function FeatureCard(props: { icon: ReactNode; title: string; body: string }) {
  return (
    <div className="rounded-2xl border border-black/10 bg-white p-5 shadow-sm">
      <div className="mb-5 grid size-9 place-items-center rounded-full bg-[#141824] text-white">{props.icon}</div>
      <h3 className="text-base font-extrabold">{props.title}</h3>
      <p className="mt-2 text-sm leading-6 text-[#6b6872]">{props.body}</p>
    </div>
  );
}

function buildTextGuideFeatures(config: ModelConfig, t: (key: string, vars?: Record<string, string>) => string) {
  return [
    {
      icon: <Code2 className="size-4" />,
      title: t("OpenAI-compatible migration path"),
      body: t("Chat Completions-style payloads reduce switching friction from existing model stacks."),
    },
    {
      icon: <Layers3 className="size-4" />,
      title: t("Structured and tool-based output"),
      body: t("Use structured JSON, tools, and code-generation flows for agentic workflows."),
    },
    {
      icon: <Timer className="size-4" />,
      title: t("Streaming interaction"),
      body: t("Streaming supports chat UIs, terminal assistants, and progressive rendering."),
    },
    {
      icon: <ShieldCheck className="size-4" />,
      title: t("Production routing"),
      body: t("Keep usage, keys, quotas, and model routing in one Flatkey account."),
    },
    {
      icon: <FileText className="size-4" />,
      title: t("Long-context work"),
      body: t("Useful for document summarization, codebase analysis, and knowledge workflows."),
    },
    {
      icon: <Settings2 className="size-4" />,
      title: t("Coding and technical generation"),
      body: t("Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts."),
    },
  ];
}

function buildMediaPricingRows(config: ModelConfig) {
  if (config.slug === "gpt-image-2") {
    return [
      { spec: "1K", flatkey: "$0.0075", official: "$0.22" },
      { spec: "2K", flatkey: "$0.010", official: "$0.50" },
      { spec: "4K", flatkey: "$0.0125", official: "$1.30" },
    ];
  }
  if (config.generator?.kind === "video") {
    return [
      { spec: "720p", flatkey: config.flatkeyPrice, note: "Shared balance", official: config.officialPrice },
      { spec: "1080p", flatkey: "$0.067", note: "Shared balance", official: "$0.10" },
      { spec: "I2V", flatkey: "$0.053", note: "Shared balance", official: "$0.08" },
    ];
  }
  if (config.generator?.kind === "audio") {
    return [
      { spec: "Standard", flatkey: config.flatkeyPrice, note: "Shared balance", official: config.officialPrice },
      { spec: "High quality", flatkey: config.estFlatkey, note: "Shared balance", official: config.estOfficial },
      { spec: "Batch", flatkey: config.flatkeyPrice, note: "Shared balance", official: config.officialPrice },
    ];
  }
  return [
    { spec: "1024x1024", flatkey: config.flatkeyPrice, note: "Shared balance", official: config.officialPrice },
    { spec: "1536x1024", flatkey: config.flatkeyPrice, note: "Shared balance", official: config.officialPrice },
    { spec: "1024x1536", flatkey: config.flatkeyPrice, note: "Shared balance", official: config.officialPrice },
  ];
}

function buildInitialGeneratorValues(config: ModelConfig) {
  return Object.fromEntries((config.generator?.fields ?? []).map((field) => [field.name, field.defaultValue]));
}

export function buildGeneratorRequest(
  config: ModelConfig,
  prompt: string,
  values: Record<string, string | number | boolean>,
  referenceImages: ReferenceImageDraft[] = []
) {
  if (config.generator?.kind === "video") {
    const content = [{ type: "text", text: prompt }];
    return compactRequest({
      model: config.modelId,
      content,
      ...values,
      reference_assets:
        referenceImages.length > 0
          ? referenceImages.map(({ name, size, type }) => ({ name, size, type }))
          : undefined,
    });
  }
  if (config.generator?.kind === "audio") {
    return compactRequest({ model: config.modelId, input: prompt, ...values });
  }
  if (config.generator?.kind === "image") {
    return compactRequest({
      model: config.modelId,
      prompt,
      ...values,
      reference_images:
        referenceImages.length > 0
          ? referenceImages.map(({ name, size, type }) => ({ name, size, type }))
          : undefined,
    });
  }
  return { model: config.modelId, prompt };
}

function compactRequest(value: Record<string, unknown>) {
  return Object.fromEntries(
    Object.entries(value).filter(([, entry]) => entry !== "" && entry !== undefined)
  );
}

function buildRunHref(
  config: ModelConfig,
  locale: Locale,
  prompt: string,
  draft: DraftValue
) {
  void prompt;
  void draft;
  const playgroundParams = buildPlaygroundEntryParams(config, locale);
  const authParams = new URLSearchParams({
    redirect: `/playground?${playgroundParams.toString()}`,
    lng: locale,
  });
  return consoleUrl("/sign-up", authParams.toString());
}

export function buildDraftFallbackRunHref(config: ModelConfig, locale: Locale, draft: DraftValue) {
  const playgroundParams = buildPlaygroundEntryParams(config, locale);
  const draftText = JSON.stringify(draft);
  if (draftText.length <= 12000) {
    playgroundParams.set("draft", draftText);
  }
  const authParams = new URLSearchParams({
    redirect: `/playground?${playgroundParams.toString()}`,
    lng: locale,
  });
  return consoleUrl("/sign-up", authParams.toString());
}

function buildHandoffRunHref(
  config: ModelConfig,
  locale: Locale,
  handoffId: string,
  mediaKind: string
) {
  const playgroundParams = buildPlaygroundEntryParams(config, locale);
  playgroundParams.set("handoff_id", handoffId);
  playgroundParams.set("media_kind", mediaKind);
  const authParams = new URLSearchParams({
    redirect: `/playground?${playgroundParams.toString()}`,
    lng: locale,
  });
  return consoleUrl("/sign-up", authParams.toString());
}

function buildPlaygroundEntryParams(config: ModelConfig, locale: Locale) {
  const playgroundParams = new URLSearchParams({
    source: "model_landing",
    model: config.modelId,
    lng: locale,
  });
  if (config.generator?.kind === "image" || config.generator?.kind === "video") {
    playgroundParams.set("generate", config.generator.kind);
  }
  if (config.generator?.kind) {
    playgroundParams.set("media_kind", config.generator.kind);
  }
  return playgroundParams;
}

async function createModelHandoffDraft(
  config: ModelConfig,
  locale: Locale,
  mediaKind: string,
  draft: DraftValue
) {
  const response = await fetch(consoleUrl("/api/model-handoffs"), {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      source: "model_landing",
      model: config.modelId,
      media_kind: mediaKind,
      locale,
      payload: draft,
    }),
  });
  if (!response.ok) {
    throw new Error("Failed to create model handoff");
  }
  const data = (await response.json()) as {
    success?: boolean;
    data?: { handoff_id?: string };
  };
  const handoffId = data.data?.handoff_id;
  if (!data.success || !handoffId) {
    throw new Error("Failed to create model handoff");
  }
  return handoffId;
}

function withCurrentSearch(baseHref: string) {
  const currentSearch = window.location.search;
  if (!currentSearch) return baseHref;
  const url = new URL(baseHref);
  const current = new URLSearchParams(currentSearch);
  current.forEach((value, key) => {
    if (!url.searchParams.has(key)) url.searchParams.set(key, value);
  });
  return url.toString();
}

function coerceGeneratorValue(field: ModelGeneratorField, raw: string) {
  if (field.type === "boolean" || typeof field.defaultValue === "boolean") return raw === "true";
  if (field.type !== "number" && typeof field.defaultValue !== "number") return raw;
  const value = Number(raw);
  if (!Number.isFinite(value)) return field.defaultValue;
  return Math.min(field.max ?? value, Math.max(field.min ?? value, value));
}

function buildQuickPrompt(label: string, kind: "image" | "video" | "audio") {
  if (kind === "video") {
    if (label === "UGC ad clips") {
      return "Create a 9:16 Flatkey brand UGC ad: a creator opens with a quick product pain point, shows the Flatkey dashboard workflow on a laptop, then ends on a clean CTA card. Natural handheld energy, clear speech-friendly pacing, generated audio on.";
    }
    if (label === "Product motion") {
      return "Create a 16:9 Flatkey product reveal: start on the logo mark, move into API routing cards and live price rows, then finish with a polished dashboard hero shot. Smooth camera push, crisp UI motion, subtle sound design, generated audio on.";
    }
    if (label === "Social video variants") {
      return "Create a short Flatkey campaign variant for social: three quick scenes show model choice, price comparison, and successful video output. Bright product lighting, simple transitions, readable UI rhythm, generated audio on.";
    }
    return `${label}: a concise commercial video shot with clear subject motion, realistic lighting, stable camera, and production-ready framing.`;
  }
  if (kind === "audio") {
    return `${label}: a clean studio-quality audio generation brief with precise tone, pacing, ambience, and delivery notes.`;
  }
  return `${label}: a high-quality product visual with clean composition, precise lighting, strong subject focus, and realistic detail.`;
}
