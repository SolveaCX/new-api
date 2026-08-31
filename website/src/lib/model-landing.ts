import { LOCALES, type Locale } from "./locales";
import { withIdFallback } from "@/lib/locales";
import { formatUsdPrice, type PricingModel } from "./pricing";
import {
  getPriorityModelCopy,
  getPriorityModelTranslationMapForSource,
  getPriorityModelTranslations,
} from "./priority-model-copy";

export type ModelPriceRow = {
  label: string;
  flatkey: string;
  official?: string;
  value?: string;
};

export type ModelGeneratorField = {
  name: string;
  label: string;
  type: "select" | "number" | "text" | "boolean";
  defaultValue: string | number | boolean;
  options?: string[];
  min?: number;
  max?: number;
  help?: string;
};

export type ModelGeneratorProtocol =
  | "openai-image"
  | "gemini-image"
  | "seedance-video"
  | "veo-video"
  | "grok-video"
  | "audio";

/**
 * Input workflows exposed by the documented Seedance content[] contract.
 *
 * This is deliberately opt-in metadata rather than something inferred from
 * `protocol`: other video adapters use different request shapes and action
 * support.  A mode that is not supported by the selected route stays visible
 * in the Playground as disabled, but is never serialized as a made-up API
 * field.
 */
export type ModelVideoMode =
  | "text-to-video"
  | "image-to-video"
  | "reference-to-video"
  | "video-edit"
  | "video-extend";

export type ModelVideoModeOption = {
  value: ModelVideoMode;
  supported: boolean;
};

export const SEEDANCE_VIDEO_MODE_OPTIONS: readonly ModelVideoModeOption[] = [
  { value: "text-to-video", supported: true },
  { value: "image-to-video", supported: true },
  { value: "reference-to-video", supported: true },
  { value: "video-edit", supported: false },
  { value: "video-extend", supported: false },
];

type ModelVideoModeUiCopy = {
  label: string;
  helper: string;
  unavailable: string;
};

/**
 * Keep these labels identical across locales: they are Seedance's documented
 * workflow names and are also used when users map a Playground mode to the
 * provider contract.
 */
const MODEL_VIDEO_MODE_ENGLISH_LABELS: Record<ModelVideoMode, string> = {
  "text-to-video": "Text-to-Video",
  "image-to-video": "Image-to-Video",
  "reference-to-video": "Reference-to-Video",
  "video-edit": "Video Edit",
  "video-extend": "Video Extend",
};

const MODEL_VIDEO_MODE_UI_COPY: Record<Locale, ModelVideoModeUiCopy> = {
  en: {
    label: "Video mode",
    helper: "Choose the documented input workflow. The request keeps Seedance's content[] contract.",
    unavailable: "Not available on this route",
  },
  zh: {
    label: "视频模式",
    helper: "选择文档化的输入工作流，请求仍使用 Seedance 的 content[] 契约。",
    unavailable: "当前路由不可用",
  },
  es: {
    label: "Modo de vídeo",
    helper: "Elige el flujo de entrada documentado. La solicitud mantiene el contrato content[] de Seedance.",
    unavailable: "No disponible en esta ruta",
  },
  fr: {
    label: "Mode vidéo",
    helper: "Choisissez le flux d’entrée documenté. La requête conserve le contrat content[] de Seedance.",
    unavailable: "Indisponible sur cette route",
  },
  pt: {
    label: "Modo de vídeo",
    helper: "Escolha o fluxo de entrada documentado. A solicitação mantém o contrato content[] do Seedance.",
    unavailable: "Indisponível nesta rota",
  },
  ru: {
    label: "Режим видео",
    helper: "Выберите документированный способ ввода. Запрос сохраняет контракт Seedance content[].",
    unavailable: "Недоступно для этого маршрута",
  },
  ja: {
    label: "ビデオモード",
    helper: "ドキュメント化された入力ワークフローを選択します。リクエストはSeedanceのcontent[]契約を保持します。",
    unavailable: "このルートでは利用できません",
  },
  vi: {
    label: "Chế độ video",
    helper: "Chọn quy trình đầu vào đã được tài liệu hóa. Yêu cầu vẫn giữ hợp đồng content[] của Seedance.",
    unavailable: "Không khả dụng trên tuyến này",
  },
  de: {
    label: "Videomodus",
    helper: "Wählen Sie den dokumentierten Eingabeworkflow. Die Anfrage behält den Seedance-content[]-Vertrag bei.",
    unavailable: "Auf dieser Route nicht verfügbar",
  },
  id: {
    label: "Mode video",
    helper: "Pilih alur input yang terdokumentasi. Permintaan tetap menggunakan kontrak content[] Seedance.",
    unavailable: "Tidak tersedia di rute ini",
  },
};

export function getModelVideoModeLabel(_locale: Locale, mode: ModelVideoMode): string {
  return MODEL_VIDEO_MODE_ENGLISH_LABELS[mode];
}

export function getModelVideoModeUiCopy(locale: Locale): ModelVideoModeUiCopy {
  return MODEL_VIDEO_MODE_UI_COPY[locale] ?? MODEL_VIDEO_MODE_UI_COPY.en;
}

export type ModelReferenceLimits = Partial<Record<"image" | "video" | "audio", number>>;

export type ModelGeneratorConfig = {
  kind: "image" | "video" | "audio";
  endpoint: string;
  storageKey: string;
  fields: ModelGeneratorField[];
  /** Wire contract used by the Playground request preview and builder. */
  protocol?: ModelGeneratorProtocol;
  /** Maximum reference files accepted by this model's documented route. */
  referenceLimits?: ModelReferenceLimits;
  /**
   * Optional, route-specific video workflows. Do not infer this from the
   * protocol: a shared protocol name does not imply shared action support.
   */
  videoModes?: readonly ModelVideoModeOption[];
  /** Default UI workflow for an explicitly configured video mode selector. */
  defaultVideoMode?: ModelVideoMode;
};

/**
 * Optional content overrides for the shared model-detail shell.  The shell is
 * intentionally data-driven: the approved Seedance prototype can define its
 * comparison, prompt, API, and FAQ content without changing the layout used
 * by text, image, and other video model pages.
 */
export type ModelLandingContent = {
  hero?: {
    title?: string;
    description?: string;
    logo?: string;
    breadcrumb?: string[];
    actionLabel?: string;
    provider?: string;
    flatkeyPrice?: string;
    referencePrice?: string;
  };
  performance?: {
    eyebrow?: string;
    title: string;
    description: string;
    metrics?: Array<{ label: string; value?: string; note: string; icon: "latency" | "requests" | "uptime" }>;
  };
  activity?: {
    eyebrow?: string;
    title: string;
    description: string;
    stats?: Array<{ label: string; value: string; unit?: string }>;
    sampleChart?: boolean;
  };
  /**
   * Optional editorial pricing block. The shared shell still receives the
   * live pricing rows from the catalog; this metadata only supplies the
   * model-specific heading, explanation, and (when needed) audited formula
   * rows. Keeping the presentation data-driven avoids another page layout.
   */
  pricing?: {
    eyebrow?: string;
    title: string;
    description: string;
    note?: string;
    rows?: Array<{ label: string; value: string; detail?: string }>;
  };
  capabilities?: Array<{ title: string; body: string }>;
  capabilitiesEyebrow?: string;
  capabilitiesTitle?: string;
  capabilitiesDescription?: string;
  comparison?: {
    eyebrow: string;
    title: string;
    description: string;
    baselineLabel: string;
    currentLabel: string;
    rows: Array<{
      label: string;
      baseline: string;
      current: string;
    }>;
  };
  promptLibrary?: Array<{
    key: string;
    label: string;
    prompt: string;
    poster: string;
    video?: string;
    alt: string;
  }>;
  promptLibraryTitle?: string;
  promptLibraryDescription?: string;
  why?: {
    eyebrow: string;
    title: string;
    description: string;
    cards: Array<{ title: string; body: string }>;
  };
  api?: {
    eyebrow?: string;
    title: string;
    description: string;
    items: Array<{ title: string; detail: string }>;
  };
  related?: {
    eyebrow?: string;
    title: string;
    description?: string;
    cards: Array<{ href: string; name: string; description: string; asset: string }>;
  };
  faq?: Array<{ question: string; answer: string }>;
  faqTitle?: { beforeBreak: string; afterBreak: string };
  faqDescription?: string;
};

export type ModelConfig = {
  slug: string;
  modelIds: string[];
  displayName: string;
  modelId: string;
  generator?: ModelGeneratorConfig;
  officialName: string;
  officialPrice: string;
  flatkeyPrice: string;
  estFlatkey: string;
  estOfficial: string;
  examplePrompt: string;
  priceUnit: ModelLandingKey;
  rows: ModelPriceRow[];
  seo: {
    title: string;
    description: string;
  };
  /** Localized metadata for markets where the page has a translated search intent. */
  seoByLocale?: Partial<Record<Locale, { title: string; description: string }>>;
  positioning: ModelLandingKey;
  useCases: ModelLandingKey[];
  faq: Array<{ question: ModelLandingKey; answer: ModelLandingKey }>;
  landingContent?: ModelLandingContent;
};

const COVERAGE = "GPT · Gemini · Claude · DeepSeek · Kimi · Seedance";

export const CLAUDE_CONFIG: ModelConfig = {
  slug: "claude-api",
  modelIds: ["claude-opus-4", "claude-sonnet-4", "claude-haiku"],
  displayName: "Claude Opus 4",
  modelId: "claude-opus-4",
  officialName: "Anthropic",
  officialPrice: "$15.00",
  flatkeyPrice: "$10.00",
  estFlatkey: "$0.005",
  estOfficial: "$0.008",
  examplePrompt:
    "You are a senior backend engineer. In 3 sentences, explain why developers should use an LLM gateway instead of calling each official API directly.",
  priceUnit: "/ million output tokens",
  rows: [
    { label: "Opus 4 output", flatkey: "$10.0", official: "$15" },
    { label: "Sonnet 4 output", flatkey: "$10.0", official: "$15" },
    { label: "Haiku output", flatkey: "$2.7", official: "$4" },
    { label: "Cache reads", flatkey: "", value: "50% off" },
    { label: "Coverage", flatkey: "", value: COVERAGE },
  ],
  seo: {
    title: "Claude API pricing with one OpenAI-compatible key",
    description: "Use Claude through flatkey.ai with OpenAI-compatible routing, lower token costs, one API key, and unified billing.",
  },
  positioning: "Best for long-context reasoning, coding agents, and production assistants",
  useCases: ["Coding agents", "Support automation", "Long document analysis"],
  faq: [
    {
      question: "Does this use the same model id in my SDK?",
      answer: "Yes. Keep your SDK and switch base_url plus api_key.",
    },
    {
      question: "Can I control usage before scaling?",
      answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.",
    },
  ],
};

export const GPT_CONFIG: ModelConfig = {
  slug: "gpt-api",
  modelIds: ["gpt-5.6-sol", "gpt-5.5", "gpt-5", "gpt-5-mini", "gpt-4o", "gpt-4.1"],
  displayName: "GPT-5",
  modelId: "gpt-5",
  officialName: "OpenAI",
  officialPrice: "$10.00",
  flatkeyPrice: "$6.67",
  estFlatkey: "$0.004",
  estOfficial: "$0.006",
  examplePrompt:
    "You are a senior backend engineer. In 3 sentences, explain why developers should use an LLM gateway instead of calling each official API directly.",
  priceUnit: "/ million output tokens",
  rows: [
    { label: "GPT-5 output", flatkey: "$6.7", official: "$10" },
    { label: "GPT-5 mini output", flatkey: "$1.3", official: "$2" },
    { label: "GPT-5 input", flatkey: "$0.83", official: "$1.25" },
    { label: "Cache reads", flatkey: "", value: "50% off" },
    { label: "Coverage", flatkey: "", value: COVERAGE },
  ],
  seo: {
    title: "GPT API pricing with one OpenAI-compatible key",
    description: "Use GPT models through flatkey.ai with OpenAI-compatible routing, lower token costs, one API key, and unified billing.",
  },
  positioning: "Best for general AI apps, agents, search, and high-volume API workloads",
  useCases: ["AI app backends", "Agent workflows", "Batch content generation"],
  faq: [
    {
      question: "Does this use the same model id in my SDK?",
      answer: "Yes. Keep your SDK and switch base_url plus api_key.",
    },
    {
      question: "Can I control usage before scaling?",
      answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.",
    },
  ],
};

export const GEMINI_CONFIG: ModelConfig = {
  slug: "gemini-api",
  modelIds: ["gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"],
  displayName: "Gemini API",
  modelId: "gemini-2.5-pro",
  officialName: "Google Gemini",
  officialPrice: "$10.00",
  flatkeyPrice: "$6.67",
  estFlatkey: "$0.004",
  estOfficial: "$0.006",
  examplePrompt:
    "You are a senior backend engineer. In 3 sentences, explain why developers should use an LLM gateway instead of calling each official API directly.",
  priceUnit: "/ million output tokens",
  rows: [
    { label: "Gemini 2.5 Pro output", flatkey: "$6.67", official: "$10" },
    { label: "Gemini 2.5 Flash output", flatkey: "$1.67", official: "$2.50" },
    { label: "Gemini 2.5 Pro input", flatkey: "$0.83", official: "$1.25" },
    { label: "Cache reads", flatkey: "", value: "up to 50% off" },
    { label: "Coverage", flatkey: "", value: COVERAGE },
  ],
  seo: {
    title: "Gemini API without GCP setup — one OpenAI-compatible key",
    description:
      "Call Gemini 2.5 Pro and Flash through flatkey.ai with no Google Cloud project, billing account, or vendor SDK — one OpenAI-compatible key, lower token costs, unified billing.",
  },
  positioning: "Best for general AI apps, agents, search, and high-volume API workloads",
  useCases: ["AI app backends", "Agent workflows", "Batch content generation"],
  faq: [
    { question: "Does this use the same model id in my SDK?", answer: "Yes. Keep your SDK and switch base_url plus api_key." },
    { question: "Can I control usage before scaling?", answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded." },
  ],
};

export const DEEPSEEK_CONFIG: ModelConfig = {
  slug: "deepseek-api",
  modelIds: ["deepseek-v4-flash", "deepseek-v4-pro", "deepseek-v3", "deepseek-v3.1", "deepseek-v3.2"],
  displayName: "DeepSeek API",
  modelId: "deepseek-v4-flash",
  officialName: "DeepSeek",
  officialPrice: "$0.14",
  flatkeyPrice: "$0.074667",
  estFlatkey: "$0.001",
  estOfficial: "$0.002",
  examplePrompt: "Compare two API gateway designs for reliability, cost control, and failover in three concise bullets.",
  priceUnit: "/ million output tokens",
  rows: [
    { label: "Coverage", flatkey: "", value: "DeepSeek V3 · V3.2 · V4 Flash · V4 Pro" },
    { label: "Cache reads", flatkey: "", value: "up to 50% off" },
  ],
  seo: {
    title: "DeepSeek API pricing — OpenAI-compatible access",
    description: "Call DeepSeek V3 and V4 models through flatkey.ai with live pricing, health metrics, one API key, and OpenAI-compatible code.",
  },
  positioning: "Best for general AI apps, agents, search, and high-volume API workloads",
  useCases: ["AI app backends", "Agent workflows", "Batch content generation"],
  faq: [
    { question: "Does this use the same model id in my SDK?", answer: "Yes. Keep your SDK and switch base_url plus api_key." },
    { question: "Can I control usage before scaling?", answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded." },
  ],
};

export const QWEN_CONFIG: ModelConfig = {
  slug: "qwen-api",
  modelIds: ["qwen3.7-plus", "qwen3.7-max", "qwen3.6-plus", "qwen3.5-plus", "qwen3.5-flash"],
  displayName: "Qwen API",
  modelId: "qwen3.7-plus",
  officialName: "Alibaba Qwen",
  officialPrice: "$0.40",
  flatkeyPrice: "$0.24",
  estFlatkey: "$0.002",
  estOfficial: "$0.004",
  examplePrompt: "Design a multilingual support-agent workflow and return the architecture in three concise bullets.",
  priceUnit: "/ million output tokens",
  rows: [
    { label: "Coverage", flatkey: "", value: "Qwen 3.5 · 3.6 · 3.7 · Max · Plus" },
    { label: "Cache reads", flatkey: "", value: "up to 50% off" },
  ],
  seo: {
    title: "Qwen API pricing — one OpenAI-compatible key",
    description: "Use Qwen 3.5, 3.6, and 3.7 models through flatkey.ai with live pricing, one API key, and OpenAI-compatible routing.",
  },
  positioning: "Best for general AI apps, agents, search, and high-volume API workloads",
  useCases: ["AI app backends", "Agent workflows", "Batch content generation"],
  faq: [
    { question: "Does this use the same model id in my SDK?", answer: "Yes. Keep your SDK and switch base_url plus api_key." },
    { question: "Can I control usage before scaling?", answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded." },
  ],
};

export const GLM_API_CONFIG: ModelConfig = {
  slug: "glm-api",
  modelIds: ["glm-5.2", "glm-5-turbo", "glm-4.7"],
  displayName: "GLM API",
  modelId: "glm-5.2",
  officialName: "Z.ai",
  officialPrice: "$1.40",
  flatkeyPrice: "$0.56",
  estFlatkey: "$0.003",
  estOfficial: "$0.006",
  examplePrompt: "Review this API migration plan for cost, latency, and rollback risk in three concise bullets.",
  priceUnit: "/ million output tokens",
  rows: [
    { label: "Coverage", flatkey: "", value: "GLM 4.7 · GLM 5 Turbo · GLM 5.2" },
    { label: "Cache reads", flatkey: "", value: "up to 50% off" },
  ],
  seo: {
    title: "GLM API pricing — GLM 5.2 and Z.ai models",
    description: "Call GLM 4.7, GLM 5 Turbo, and GLM 5.2 through flatkey.ai with live pricing, one API key, and OpenAI-compatible routing.",
  },
  positioning: "Best for general AI apps, agents, search, and high-volume API workloads",
  useCases: ["AI app backends", "Agent workflows", "Batch content generation"],
  faq: [
    { question: "Does this use the same model id in my SDK?", answer: "Yes. Keep your SDK and switch base_url plus api_key." },
    { question: "Can I control usage before scaling?", answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded." },
  ],
};

export const SEEDANCE_CONFIG: ModelConfig = {
  slug: "seedance-api",
  modelIds: ["seedance-2-0", "seedance-2.0", "seedance"],
  displayName: "Seedance 2.0",
  modelId: "seedance-2-0",
  generator: {
    kind: "video",
    endpoint: "/v1/videos",
    storageKey: "flatkey:model-generator-draft:seedance-2-0",
    protocol: "seedance-video",
    referenceLimits: { image: 30, video: 10, audio: 10 },
    videoModes: SEEDANCE_VIDEO_MODE_OPTIONS,
    defaultVideoMode: "text-to-video",
    fields: [
      { name: "resolution", label: "Resolution", type: "select", defaultValue: "1080p", options: ["720p", "1080p"] },
      { name: "ratio", label: "Aspect ratio", type: "select", defaultValue: "16:9", options: ["16:9", "9:16", "1:1", "4:3", "3:4"] },
      { name: "duration", label: "Duration", type: "select", defaultValue: 5, options: ["5", "10"] },
      { name: "frames", label: "Frames", type: "number", defaultValue: 0, min: 0, max: 240, help: "Optional frame count override" },
      { name: "camera_fixed", label: "Camera fixed", type: "boolean", defaultValue: false },
      { name: "generate_audio", label: "Generate audio", type: "boolean", defaultValue: false },
      { name: "return_last_frame", label: "Return last frame", type: "boolean", defaultValue: false },
      { name: "seed", label: "Seed", type: "number", defaultValue: 0, min: 0, max: 2147483647, help: "0 means random" },
    ],
  },
  officialName: "fal.ai",
  officialPrice: "$0.07",
  flatkeyPrice: "$0.047",
  estFlatkey: "$0.23",
  estOfficial: "$0.35",
  examplePrompt:
    "A cinematic drone shot flying over a neon-lit Tokyo street at night, rain reflections, 5 seconds.",
  priceUnit: "/ second",
  rows: [
    { label: "Seedance video / sec", flatkey: "$0.047", official: "$0.07" },
    { label: "Image-to-video / sec", flatkey: "$0.053", official: "$0.08" },
    { label: "1080p / sec", flatkey: "$0.067", official: "$0.10" },
    { label: "Coverage", flatkey: "", value: "Seedance · Kling · Veo · Sora · GPT · Claude" },
  ],
  seo: {
    title: "Seedance video API — cheaper than official, one API key",
    description: "Generate Seedance text/image-to-video through flatkey.ai at lower per-second cost, with one API key and unified billing.",
  },
  positioning: "Best for product videos, ad creative, and image-to-video production",
  useCases: ["UGC ad clips", "Product motion", "Social video variants"],
  faq: [
    {
      question: "Does this use the same model id in my SDK?",
      answer: "Yes. Keep your SDK and switch base_url plus api_key.",
    },
    {
      question: "Can I control usage before scaling?",
      answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.",
    },
  ],
};

/**
 * Seedance 2.5 is a separate public model family from the legacy 2.0
 * landing. Keep its contract explicit here so a live `seedance-2.5` pricing
 * entry cannot fall through to the generic video defaults (1080p/16:9).
 */
export const SEEDANCE_25_CONFIG: ModelConfig = {
  slug: "seedance-2.5",
  modelIds: ["seedance-2.5", "seedance-2-5"],
  displayName: "Seedance 2.5",
  modelId: "seedance-2.5",
  generator: {
    kind: "video",
    endpoint: "/v1/videos",
    storageKey: "flatkey:model-generator-draft:seedance-2-5",
    protocol: "seedance-video",
    referenceLimits: { image: 30, video: 10, audio: 10 },
    videoModes: SEEDANCE_VIDEO_MODE_OPTIONS,
    defaultVideoMode: "text-to-video",
    fields: [
      { name: "resolution", label: "Resolution", type: "select", defaultValue: "720p", options: ["480p", "720p"] },
      { name: "ratio", label: "Aspect ratio", type: "select", defaultValue: "adaptive", options: ["adaptive", "16:9", "4:3", "1:1", "3:4", "9:16"] },
      { name: "duration", label: "Duration", type: "number", defaultValue: 5, min: 4, max: 30 },
      { name: "generate_audio", label: "Generate audio", type: "boolean", defaultValue: true },
    ],
  },
  officialName: "ByteDance",
  // The catalog's model_price is a base value used by the billing resolver;
  // it is not a universal per-second retail price. The request formula also
  // depends on resolution and whether a video reference is supplied.
  officialPrice: "$0.14 base",
  flatkeyPrice: "$0.14 base",
  estFlatkey: "$0.14 base",
  estOfficial: "$0.14 base",
  examplePrompt:
    "A cinematic product shot of a sports car on a wet track, soft studio lighting, high detail.",
  priceUnit: "Pricing",
  rows: [
    { label: "Request price", flatkey: "$0.140 × duration", official: "See request formula" },
    { label: "Request price", flatkey: "$0.314 × duration", official: "See request formula" },
    { label: "Video reference input", flatkey: "$0.084–$0.188 × video seconds", official: "See request formula" },
    { label: "Coverage", flatkey: "", value: "Seedance 2.5 · Seedance 2.0 · Kling · Veo · Sora" },
  ],
  seo: {
    title: "Seedance 2.5 AI video generator — API and pricing | Flatkey",
    description:
      "Use ByteDance Seedance 2.5 as an AI video generator through the Flatkey API: text-to-video, image-to-video, 4–30 second clips, reference media, and 480p/720p pricing.",
  },
  seoByLocale: {
    en: {
      title: "Seedance 2.5 AI video generator — API and pricing | Flatkey",
      description: "Use ByteDance Seedance 2.5 as an AI video generator through the Flatkey API: text-to-video, image-to-video, 4–30 second clips, reference media, and 480p/720p pricing.",
    },
    pt: {
      title: "Gerador de vídeo IA Seedance 2.5 e preços da API | Flatkey",
      description: "Use o Seedance 2.5 da ByteDance como gerador de vídeo por IA na API Flatkey: texto para vídeo, imagem para vídeo, clipes de 4–30 segundos e preços para 480p/720p.",
    },
    zh: {
      title: "Seedance 2.5 AI 视频生成器与 API 价格 | Flatkey",
      description: "通过 Flatkey API 使用 ByteDance Seedance 2.5：支持文生视频、图生视频、4–30 秒片段、参考素材，以及 480p/720p 定价。",
    },
    es: {
      title: "Generador de vídeo IA Seedance 2.5 y precios de API | Flatkey",
      description: "Usa ByteDance Seedance 2.5 como generador de vídeo IA con la API de Flatkey: texto a vídeo, imagen a vídeo, clips de 4–30 segundos y precios 480p/720p.",
    },
    fr: {
      title: "Générateur vidéo IA Seedance 2.5 et tarifs API | Flatkey",
      description: "Utilisez ByteDance Seedance 2.5 comme générateur vidéo IA via l’API Flatkey : texte-vers-vidéo, image-vers-vidéo, clips de 4 à 30 secondes et tarifs 480p/720p.",
    },
    ru: {
      title: "ИИ-генератор видео Seedance 2.5 и цены API | Flatkey",
      description: "Используйте ByteDance Seedance 2.5 через API Flatkey: текст-видео, изображение-видео, клипы 4–30 секунд, референсы и цены 480p/720p.",
    },
    ja: {
      title: "Seedance 2.5 AI動画生成とAPI料金 | Flatkey",
      description: "Flatkey APIでByteDance Seedance 2.5を利用。テキストから動画、画像から動画、4～30秒のクリップ、参照素材、480p/720p料金に対応します。",
    },
    vi: {
      title: "Trình tạo video AI Seedance 2.5 và giá API | Flatkey",
      description: "Dùng ByteDance Seedance 2.5 qua API Flatkey: văn bản thành video, ảnh thành video, clip 4–30 giây, media tham chiếu và giá 480p/720p.",
    },
    de: {
      title: "Seedance 2.5 KI-Videogenerator und API-Preise | Flatkey",
      description: "Nutzen Sie ByteDance Seedance 2.5 über die Flatkey-API: Text-zu-Video, Bild-zu-Video, 4–30-Sekunden-Clips, Referenzmedien und 480p/720p-Preise.",
    },
    id: {
      title: "Generator Video AI Seedance 2.5 dan harga API | Flatkey",
      description: "Gunakan ByteDance Seedance 2.5 melalui API Flatkey: teks menjadi video, gambar menjadi video, klip 4–30 detik, media referensi, serta harga 480p/720p.",
    },
  },
  positioning: "Best for product videos, ad creative, and image-to-video production",
  useCases: ["UGC ad clips", "Product motion", "Social video variants"],
  faq: [
    {
      question: "Does this use the same model id in my SDK?",
      answer: "Yes. Keep your SDK and switch base_url plus api_key.",
    },
    {
      question: "Can I control usage before scaling?",
      answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.",
    },
  ],
  landingContent: {
    hero: {
      title: "Seedance 2.5 AI video generator and API",
      description: "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.",
      logo: "/logos/seedance.png",
      breadcrumb: ["Models", "Video generation", "Seedance 2.5 API"],
      actionLabel: "Quick Start",
      provider: "ByteDance",
      flatkeyPrice: "$0.140 × duration",
      referencePrice: "See request formula",
    },
    performance: {
      eyebrow: "Performance",
      title: "Seedance 2.5 AI video API performance and uptime",
      description: "Live request telemetry appears here when enough Flatkey traffic is available.",
    },
    activity: {
      eyebrow: "Activity",
      title: "Seedance 2.5 video API usage activity",
      description: "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.",
      sampleChart: false,
    },
    capabilities: [
      { title: "Text-to-video and image-to-video", body: "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed." },
      { title: "Reference media and frame control", body: "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows." },
      { title: "Audio-video generation", body: "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document." },
      { title: "Duration and output controls", body: "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task." },
    ],
    capabilitiesEyebrow: "Seedance 2.5 features",
    capabilitiesTitle: "Seedance 2.5 AI video generator features for references and audio",
    capabilitiesDescription: "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.",
    comparison: {
      eyebrow: "Compare",
      title: "Compare Seedance 2.5 with Seedance 2.0: documented video fields",
      description: "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.",
      baselineLabel: "Seedance 2.0 (not re-audited)",
      currentLabel: "Seedance 2.5",
      rows: [
        { label: "Clip length", baseline: "Not verified in this audit", current: "4–30 seconds per request" },
        { label: "Reference inputs", baseline: "Not verified in this audit", current: "Up to 50 total: 30 images, 10 videos, and 10 audio" },
        { label: "Frame control", baseline: "Not verified in this audit", current: "First-frame and last-frame roles are supported" },
        { label: "Audio", baseline: "Not verified in this audit", current: "Audio generation can be enabled per request" },
        { label: "Editing workflow", baseline: "Not verified in this audit", current: "Reference-guided and first/last-frame workflows" },
      ],
    },
    // Keep the model page tied to workflow-specific examples used by the
    // prompt library. They are starting points, not claims that every card is
    // a live generation from this model.
    promptLibrary: [
      {
        key: "micro-drama-comic-storyboard",
        label: "Micro-drama storyboard",
        prompt: "Create a 9:16 micro-drama storyboard from a three-beat comic script: a courier notices a torn envelope, follows a red umbrella through a crowded station, and stops at a silent platform. Keep the same two characters, wardrobe, props, and rainy evening light across the beats; use clear eyelines and no readable text.",
        poster: "/assets/cli/ugc-ad-clips.png",
        video: "/assets/cli/ugc-ad-clips.mp4",
        alt: "Micro-drama storyboard video example",
      },
      {
        key: "ecommerce-ugc-product-video",
        label: "E-commerce UGC",
        prompt: "Create a 9:16 e-commerce UGC video for a reusable travel bottle. Show an adult creator opening the package, filling the bottle, and placing it in a tote bag. Keep the bottle shape and lid color consistent, use a small apartment kitchen, leave space for Portuguese captions added in post, and do not invent logos or readable label text.",
        poster: "/assets/cli/localized-variants.png",
        video: "/assets/cli/localized-variants.mp4",
        alt: "E-commerce UGC product video example",
      },
      {
        key: "film-previz-camera-blocking",
        label: "Film previsualization",
        prompt: "Create a 12-second film previsualization of a detective entering an empty observatory at dawn. Start with a wide establishing view, track behind the character, then pan to the telescope and hold on the doorway. Keep screen direction, blocking, and the warm-to-cool light transition consistent; no dialogue text or logos.",
        poster: "/assets/cli/product-reveal.png",
        video: "/assets/cli/product-reveal.mp4",
        alt: "Film previsualization camera blocking example",
      },
      {
        key: "game-cinematic-reference-shot",
        label: "Game cinematic",
        prompt: "Animate a game-cinematic reference board into an 8-second shot: a masked pilot walks across a hangar while a grounded shuttle powers up behind them. Preserve the pilot silhouette, helmet markings, shuttle geometry, and camera-left-to-right movement; use practical hangar lights and restrained smoke, with no new text or logos.",
        poster: "/assets/cli/campaign-hero.png",
        alt: "Game cinematic reference shot example",
      },
      {
        key: "creator-social-explainer-video",
        label: "Creator explainer",
        prompt: "Create a 4:5 creator explainer about a compact microphone for an independent video maker. Begin with a talking-head medium shot, cut to a close-up of the cable connection, then return to the same framing for a practical tip. Keep the speaker, microphone, and room layout consistent; leave clean caption space and avoid readable brand text.",
        poster: "/assets/cli/storyboard-motion.png",
        alt: "Creator microphone explainer video example",
      },
      {
        key: "market-research-creative-variant",
        label: "Market-research variant",
        prompt: "Create three short, clearly distinct creative variants for a market-research test of a sunscreen product: beach morning, city commute, and family picnic. Keep the same bottle proportions and cap, change only setting and opening action, use neutral props, and leave all claims and captions for post-production. Do not add medical promises or invented labels.",
        poster: "/assets/cli/thumbnail-test-set.png",
        alt: "Market-research creative variant example",
      },
    ],
    promptLibraryTitle: "Seedance 2.5 prompt guide for text-to-video and image-to-video",
    promptLibraryDescription: "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.",
    why: {
      eyebrow: "Why Flatkey",
      title: "Use Seedance 2.5 through a unified video API",
      description: "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.",
      cards: [
        { title: "One key for the model catalog", body: "Use the same Flatkey account and API key across video, image, audio, and text workloads." },
        { title: "Documented video contract", body: "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration." },
        { title: "Pricing follows the request", body: "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise." },
        { title: "Live data only when available", body: "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported." },
      ],
    },
    api: {
      eyebrow: "API",
      title: "Use the Seedance 2.5 API at /v1/videos",
      description: "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.",
      items: [
        { title: "POST /v1/videos", detail: "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields." },
        { title: "Async task result", detail: "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready." },
        { title: "Reference limits", detail: "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total." },
        { title: "Output controls", detail: "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio." },
      ],
    },
    related: {
      eyebrow: "Related Models",
      title: "Other Seedance and AI video generator APIs",
      // Related cards are populated from the live model catalog. Keeping this
      // list empty prevents a stale hand-authored list from mixing text and
      // video models when the catalog changes.
      cards: [],
    },
    faq: [
      { question: "What is Seedance 2.5?", answer: "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls." },
      { question: "How much does Seedance 2.5 cost?", answer: "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution." },
      { question: "What can I use it for?", answer: "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests." },
      { question: "How do I use the model in my app?", answer: "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content." },
      { question: "Can I control output features?", answer: "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields." },
      { question: "Is the Flatkey API OpenAI compatible?", answer: "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract." },
      { question: "What limits apply?", answer: "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change." },
      { question: "What is the Seedance 2.5 release date?", answer: "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch." },
      { question: "Is Seedance 2.5 free?", answer: "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs." },
    ],
    faqTitle: {
      beforeBreak: "Seedance 2.5",
      afterBreak: "API, pricing, and release-date questions",
    },
  },
};

export const GPT_IMAGE_2_CONFIG: ModelConfig = {
  slug: "gpt-image-2",
  modelIds: ["gpt-image-2"],
  displayName: "GPT-image-2",
  modelId: "gpt-image-2",
  generator: {
    kind: "image",
    endpoint: "/v1/images/generations",
    storageKey: "flatkey:model-generator-draft:gpt-image-2",
    protocol: "openai-image",
    referenceLimits: { image: 4 },
    fields: [
      { name: "n", label: "Images", type: "number", defaultValue: 1, min: 1, max: 10 },
      { name: "size", label: "Size", type: "select", defaultValue: "1024x1024", options: ["1024x1024", "1536x1024", "1024x1536", "auto"] },
      { name: "quality", label: "Quality", type: "select", defaultValue: "high", options: ["auto", "high", "medium", "low"] },
      { name: "output_format", label: "Output format", type: "select", defaultValue: "png", options: ["png", "jpeg", "webp"] },
      { name: "background", label: "Background", type: "select", defaultValue: "opaque", options: ["opaque", "auto"] },
      { name: "moderation", label: "Moderation", type: "select", defaultValue: "auto", options: ["auto", "low"] },
    ],
  },
  officialName: "OpenAI",
  // Image pricing is dimension/modality based; these fields are only
  // fallbacks for the shared card when the live catalog is unavailable.
  // Keep the actual catalog rows in the editorial pricing block below.
  officialPrice: "Varies by modality/batch",
  flatkeyPrice: "$4.00–$24.00 / 1M catalog units",
  estFlatkey: "$4.00–$24.00 / 1M catalog units",
  estOfficial: "Varies by modality/batch",
  examplePrompt:
    "A complex AI image generation mood wall photographed as one premium studio scene: a large violet-and-gold abstract floral artwork surrounded by pinned botanical sketches, macro insect study, translucent vellum flower sheets, black-and-white portrait, crystal minerals, perfume product render, architectural arch and staircase studies, film strips, fabric swatches, tape, brass pins, graphite notes, and warm spotlights on a charcoal wall.",
  priceUnit: "/ image",
  rows: [
    { label: "GPT Image 2 catalog dimensions", flatkey: "$4.00–$24.00 / 1M units", official: "Varies by modality/batch" },
    { label: "Square output", flatkey: "", value: "1024 × 1024" },
    { label: "Fast product mockups", flatkey: "", value: "image, ads, ecommerce" },
    { label: "Coverage", flatkey: "", value: "GPT-image-2 · Nano Banana Pro · Imagen · Qwen Image" },
  ],
  seo: {
    title: "GPT-image-2 image generator — configure prompts before signup",
    description:
      "Try a GPT-image-2 style image generator landing page, save your prompt settings locally, then continue to Flatkey signup or the console.",
  },
  positioning: "Best for product images, ad creatives, and ecommerce visual variants",
  useCases: ["Product mockups", "Ad creatives", "Ecommerce images"],
  faq: [
    {
      question: "Does this start a real generation?",
      answer: "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.",
    },
    {
      question: "Where are my edited prompt settings stored?",
      answer: "They are stored in this browser's localStorage so the draft survives the signup handoff.",
    },
  ],
};

export const KIMI_K3_CONFIG: ModelConfig = {
  slug: "kimi-k3",
  modelIds: ["kimi-k3"],
  displayName: "Kimi K3",
  modelId: "kimi-k3",
  officialName: "Moonshot AI",
  officialPrice: "$15.00",
  flatkeyPrice: "$12.00",
  estFlatkey: "$12.00",
  estOfficial: "$15.00",
  examplePrompt: "Extract the action items from this document and return owners, deadlines, and open questions.",
  priceUnit: "/ million output tokens",
  rows: [
    { label: "Kimi K3 input", flatkey: "$2.40", official: "$3.00" },
    { label: "Kimi K3 output", flatkey: "$12.00", official: "$15.00" },
    { label: "Cache-hit input", flatkey: "$0.24", official: "$0.30" },
    { label: "Context", flatkey: "", value: "1,048,576 tokens" },
    { label: "Modalities", flatkey: "", value: "Text · image · file (upstream vision; Flatkey fields vary)" },
  ],
  seo: {
    title: "Kimi K3 API and pricing | Flatkey",
    description: "Use Moonshot AI's Kimi K3 through OpenAI- and Anthropic-compatible endpoints with a 1,048,576-token context, file input, and current token pricing.",
  },
  positioning: "Best for long-context reasoning, coding agents, and production assistants",
  useCases: ["Long document analysis", "Coding agents", "Agent workflows"],
  faq: [
    { question: "Does this use the same model id in my SDK?", answer: "Yes. Keep your SDK and switch base_url plus api_key." },
    { question: "Can I control usage before scaling?", answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded." },
  ],
};

export const MINIMAX_H3_CONFIG: ModelConfig = {
  slug: "minimax-h3",
  modelIds: ["MiniMax-H3"],
  displayName: "MiniMax-H3",
  modelId: "MiniMax-H3",
  generator: {
    kind: "video",
    endpoint: "/v1/videos",
    storageKey: "flatkey:model-generator-draft:minimax-h3",
    protocol: "seedance-video",
    referenceLimits: { image: 9, video: 3, audio: 3 },
    fields: [
      { name: "resolution", label: "Resolution", type: "select", defaultValue: "768P", options: ["768P", "2K"] },
      { name: "duration", label: "Duration", type: "number", defaultValue: 6, min: 4, max: 15 },
      { name: "ratio", label: "Aspect ratio", type: "select", defaultValue: "16:9", options: ["21:9", "16:9", "4:3", "1:1", "3:4", "9:16", "adaptive"] },
      { name: "aigc_watermark", label: "AIGC watermark", type: "boolean", defaultValue: false },
    ],
  },
  officialName: "MiniMax",
  officialPrice: "$0.08",
  flatkeyPrice: "$0.08",
  estFlatkey: "$0.08",
  estOfficial: "$0.08",
  examplePrompt:
    "A paper boat crosses a rain puddle at street level, cinematic macro shot, soft reflections, 6 seconds.",
  priceUnit: "/ second",
  rows: [
    { label: "MiniMax-H3 catalog base / sec", flatkey: "$0.08", official: "$0.08" },
    { label: "Resolution", flatkey: "", value: "768P · 2K (live estimate)" },
    { label: "Reference video input", flatkey: "", value: "resolution- and input-second-dependent" },
    { label: "Input image", flatkey: "", value: "check current catalog allowance" },
  ],
  seo: {
    title: "MiniMax-H3 video generator — configure prompts before signup",
    description:
      "Configure MiniMax-H3 video requests on flatkey.ai, save prompt settings locally, then continue to signup or the console.",
  },
  positioning: "Best for product videos, ad creative, and image-to-video production",
  useCases: ["UGC ad clips", "Product motion", "Social video variants"],
  faq: [
    {
      question: "Which MiniMax-H3 fields can I configure here?",
      answer: "Configure resolution, duration, ratio, and AIGC watermark before opening the console.",
    },
    {
      question: "Does this start a real generation?",
      answer: "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.",
    },
  ],
};

/** Editorial, model-specific copy for high-priority catalog IDs.  These are
 * merged only when the live pricing model matches the exact normalized ID;
 * the shared config (including Seedance) remains the fallback for every other
 * model in the family.
 */
const PRIORITY_MODEL_OVERRIDES: Record<string, Partial<ModelConfig>> = {
  "gpt-5-6-sol": {
    seo: {
      title: "GPT-5.6 Sol API and pricing for developers | Flatkey",
      description: "Build with GPT-5.6 Sol through Flatkey: OpenAI-compatible API access, a long context window, image input, current Flatkey token rates, and one API key.",
    },
    seoByLocale: {
      en: { title: "GPT-5.6 Sol API and pricing for developers | Flatkey", description: "Build with GPT-5.6 Sol through Flatkey: OpenAI-compatible API access, a long context window, image input, current Flatkey token rates, and one API key." },
    },
    rows: [
      { label: "GPT-5.6 Sol input", flatkey: "$4.00", official: "$4.00" },
      { label: "GPT-5.6 Sol output", flatkey: "$24.00", official: "$20.00" },
      { label: "Cached input", flatkey: "$0.40", official: "$0.40" },
      { label: "Cache writes (Flatkey catalog)", flatkey: "$5.00", official: "$5.00" },
      { label: "Context", flatkey: "", value: "1,048,576 tokens" },
      { label: "Modalities", flatkey: "", value: "Text · image (file tools are route-specific)" },
    ],
    landingContent: {
      hero: { title: "GPT-5.6 Sol API for long-context work", description: "GPT-5.6 Sol is OpenAI's GPT-5.6 catalog model. OpenAI documents a roughly 1.05M-token context, text output, image input, and Chat Completions and Responses access; Flatkey exposes the gpt-5.6-sol route and its current catalog rates below." },
      performance: { eyebrow: "Performance", title: "GPT-5.6 Sol API performance and availability", description: "Live Flatkey request telemetry appears here when enough traffic is available; no benchmark score is inferred." },
      activity: { eyebrow: "Activity", title: "GPT-5.6 Sol API usage and request activity", description: "This chart uses live Flatkey request data for the selected model and stays unreported until enough traffic is collected." },
      pricing: { title: "GPT-5.6 Sol API pricing by token", description: "These are current Flatkey catalog rates, not a universal OpenAI direct price. The live catalog and account group determine the amount you pay.", note: "Rates are shown per 1M tokens. OpenAI's standard reference is $4 input, $0.40 cached input, $5 cache writes, and $20 output; inputs above 272K tokens use the higher long-context tier. Flatkey's catalog row remains the settlement source.", rows: [
        { label: "Flatkey input", value: "$4.00", detail: "per 1M tokens" }, { label: "Flatkey output", value: "$24.00", detail: "per 1M tokens" }, { label: "Flatkey cache read", value: "$0.40", detail: "per 1M tokens" }, { label: "Flatkey cache creation", value: "$5.00", detail: "per 1M tokens; catalog-specific" },
      ] },
      capabilitiesEyebrow: "GPT-5.6 Sol capabilities",
      capabilitiesTitle: "GPT-5.6 Sol capabilities for documents, code, and agents",
      capabilitiesDescription: "These cards describe practical workflows; exact context, modality, and route fields remain in the API section below.",
      capabilities: [
        { title: "Long-context document and code work", body: "Use the model to plan, summarize, transform, and reason across large documents, codebases, and research notes." },
        { title: "Text and image understanding", body: "Ground answers in written material and supported image input when a workflow needs more than text alone." },
        { title: "Structured agent workflows", body: "Connect reasoning, structured output, tools, and streaming to assistants and application backends." },
        { title: "Production integration", body: "Keep one stable model identity and account surface as experiments become repeatable product workflows." },
      ],
      comparison: { eyebrow: "Compare fields", title: "GPT-5.6 Sol vs GPT-5.5, GPT-5.6 Terra, and GPT-5.6 Luna", description: "This table compares documented integration fields only; no performance ranking is asserted.", baselineLabel: "GPT-5.5 / GPT-5.6 Terra / GPT-5.6 Luna", currentLabel: "GPT-5.6 Sol", rows: [
        { label: "Catalog model ID", baseline: "Unknown in this verified fact set", current: "gpt-5.6-sol" },
        { label: "Endpoint", baseline: "Unknown", current: "/v1/chat/completions; /v1/responses" },
        { label: "Context", baseline: "Unknown", current: "About 1.05M tokens; 128K max output" },
        { label: "Modalities", baseline: "Unknown", current: "Text input/output; image input" },
        { label: "Relative quality/benchmark", baseline: "Not asserted", current: "Not asserted" },
      ] },
      api: { eyebrow: "API", title: "Call the GPT-5.6 Sol API from your existing client", description: "Use the exact model ID with Flatkey's Chat Completions or Responses endpoint in an OpenAI-shaped client.", items: [
        { title: "Endpoint", detail: "POST /v1/chat/completions" },
        { title: "Responses endpoint", detail: "POST /v1/responses" },
        { title: "Model ID", detail: "gpt-5.6-sol" },
        { title: "Modalities", detail: "Text input/output and image input are documented by OpenAI; Flatkey route fields may vary." },
        { title: "Compatibility", detail: "Use OpenAI-compatible request fields; do not infer Codex, streaming, or ChatGPT availability." },
      ] },
      why: { eyebrow: "Why use Flatkey for GPT-5.6 Sol?", title: "A practical GPT-5.6 Sol API workflow", description: "Keep the model ID, endpoint, context, and token dimensions visible while you move from a test request to production traffic.", cards: [
        { title: "Exact model routing", body: "Set gpt-5.6-sol explicitly and keep the /v1/chat/completions path in your client configuration." },
        { title: "Token-dimension visibility", body: "Review input, output, cache-read, and cache-creation rates instead of treating one headline price as universal." },
        { title: "Long-context workflows", body: "Use the documented long context for document, code, and agent workflow planning, then verify your account limits." },
        { title: "One account for model testing", body: "Keep API keys, limits, usage, and adjacent model experiments in the same Flatkey workspace." },
      ] },
      related: { eyebrow: "Related models", title: "More OpenAI GPT models and API options", description: "Compare adjacent OpenAI routes when you need a different context, modality, or price point.", cards: [] },
      faqTitle: { beforeBreak: "GPT-5.6 Sol API", afterBreak: "questions and pricing" },
      faqDescription: "Answers about the GPT-5.6 Sol API model ID, endpoint, token pricing, context, inputs, and catalog release metadata.",
      faq: [
        { question: "What is GPT-5.6 Sol?", answer: "GPT-5.6 Sol is OpenAI's GPT-5.6 catalog model. OpenAI documents a roughly 1.05M-token context, text output, and image input; Flatkey routes the model through /v1/chat/completions or /v1/responses." },
        { question: "How do I use GPT-5.6 Sol?", answer: "Create a Flatkey API key, send a request to /v1/chat/completions or /v1/responses, and set model to gpt-5.6-sol." },
        { question: "What is the GPT-5.6 Sol API model ID?", answer: "The verified model ID is gpt-5.6-sol; documented Flatkey endpoints are /v1/chat/completions and /v1/responses." },
        { question: "How much does GPT-5.6 Sol cost?", answer: "The current Flatkey catalog lists $4 input, $24 output, $0.40 cache read, and $5 cache creation per 1M tokens. OpenAI's standard reference lists $4 input, $5 cache writes, $0.40 cached input, and $20 output, with a higher tier above 272K input tokens; these are separate price sources, so check the dated Flatkey block before scaling." },
        { question: "What context window and inputs does GPT-5.6 Sol support?", answer: "OpenAI documents about 1.05M context tokens, text input/output, and image input. File tools and account limits remain route-specific, so check the Flatkey request contract." },
        { question: "When was GPT-5.6 Sol released?", answer: "The catalog snapshot records 2026-06-20 as the release date; treat this as catalog metadata rather than a launch announcement." },
      ],
    },
  },
  "gpt-image-2": {
    seo: { title: "GPT Image 2 image generator — API and pricing | Flatkey", description: "Create and edit images with GPT Image 2 through Flatkey: image API access, current catalog pricing, size and quality controls, formats, background, and moderation settings." },
    rows: [
      { label: "Input tokens", flatkey: "$4.00", official: "Provider table varies by modality/batch" },
      { label: "Output tokens", flatkey: "$24.00", official: "Provider table varies by modality/batch" },
      { label: "Cache tokens", flatkey: "$1.00", official: "Provider table varies by modality/batch" },
      { label: "Image input tokens", flatkey: "$6.40", official: "Provider table varies by modality/batch" },
    ],
    landingContent: {
      hero: { title: "GPT Image 2 AI image generator and API", description: "GPT Image 2 is OpenAI's image-generation model. OpenAI documents image generation and editing with flexible sizes; Flatkey exposes the image routes and request controls shown below." },
      performance: { eyebrow: "Performance", title: "GPT Image 2 API performance and availability", description: "Live Flatkey request telemetry appears here when enough image-generation traffic is available." },
      activity: { eyebrow: "Activity", title: "GPT Image 2 API usage and generation activity", description: "This chart reflects live Flatkey generation requests and remains unreported until enough traffic is collected." },
      pricing: { title: "GPT Image 2 API pricing by token and image input", description: "These are current Flatkey catalog rates. Image cost depends on request fields and usage; they are not a universal OpenAI direct price.", note: "Flatkey rows are shown per 1M catalog units; the image-input row is a catalog input-token dimension, not a separate fee for pixel dimensions. OpenAI's current reference separates text input ($5/M standard), image input ($8/M), cached image input ($2/M), and image output ($30/M), with Batch rates of $2.50/$4/$1/$15; verify the applicable modality and dated Flatkey block.", rows: [
        { label: "Flatkey input tokens", value: "$4.00", detail: "per 1M units" }, { label: "Flatkey output tokens", value: "$24.00", detail: "per 1M units" }, { label: "Flatkey cache tokens", value: "$1.00", detail: "per 1M units" }, { label: "Flatkey image input tokens", value: "$6.40", detail: "per 1M units; not a pixel-size fee" },
      ] },
      capabilitiesEyebrow: "GPT Image 2 capabilities", capabilitiesTitle: "GPT Image 2 image generation and editing capabilities", capabilitiesDescription: "Use the documented image workflows for new visuals, reference-led edits, and production-ready creative variants.",
      capabilities: [
        { title: "Text-to-image creation", body: "Generate new product, editorial, and campaign visuals from a structured natural-language prompt." },
        { title: "Reference-based editing", body: "Supply an image input to revise an existing visual while preserving important subject and composition details." },
        { title: "Flexible image composition", body: "Create square, portrait, or landscape assets with flexible image sizes for different placements and channels." },
        { title: "High-fidelity visual input", body: "Combine text and image context in one workflow for richer visual direction; audio and video inputs are not supported by this model." },
      ],
      comparison: { eyebrow: "Migration fields", title: "GPT Image 2 and GPT Image 1: image API controls", description: "GPT Image 1 values are not verified in this fact set, so only documented GPT Image 2 fields are shown.", baselineLabel: "GPT Image 1", currentLabel: "GPT Image 2", rows: [
        { label: "Endpoints", baseline: "Unknown here", current: "/v1/images/generations; /v1/images/edits" },
        { label: "Sizes", baseline: "Unknown here", current: "1024x1024, 1536x1024, 1024x1536, auto" },
        { label: "Formats", baseline: "Unknown here", current: "PNG, JPEG, WebP" },
        { label: "Quality/background/moderation", baseline: "Unknown here", current: "Documented above" },
        { label: "Image quality ranking", baseline: "Not asserted", current: "Not asserted" },
      ] },
      api: { eyebrow: "API", title: "Use the GPT Image 2 API for generation and edits", description: "Send the model ID with the supported image controls through the documented generation or edits route.", items: [
        { title: "Generation endpoint", detail: "POST /v1/images/generations" },
        { title: "Edits endpoint", detail: "POST /v1/images/edits" },
        { title: "Model ID", detail: "gpt-image-2" },
        { title: "Controls", detail: "n, size, quality, format, background, and moderation." },
        { title: "Access", detail: "Use the normal Flatkey account and API-key flow; no free or no-signup promise is made." },
      ] },
      promptLibraryTitle: "GPT Image 2 prompt examples for product and marketing visuals",
      promptLibraryDescription: "Start with a concrete subject, composition, output size, and brand constraint; adjust the request fields separately.",
      promptLibrary: [
        { key: "gpt-image-2-product", label: "Product mockup", prompt: "Studio product mockup of a matte black reusable bottle on a pale stone plinth, one soft side light, clean shadow, no text, leave negative space on the right for a headline.", poster: "/assets/prompts/awesome-images/ecommerce-skincare.png", alt: "GPT Image 2 product mockup prompt example" },
        { key: "gpt-image-2-ad", label: "Ad creative", prompt: "Square social ad still for a citrus skincare launch: glass dropper bottle, sliced bergamot, warm cream background, crisp condensation, editorial daylight, no logo or legible text.", poster: "/assets/prompts/awesome-images/ugc-coffee-ad.png", alt: "GPT Image 2 advertising prompt example" },
        { key: "gpt-image-2-storyboard", label: "Storyboard frame", prompt: "Wide storyboard frame of a cyclist entering a rain-lit city tunnel at blue hour, camera low behind the wheel, reflective pavement, clear subject silhouette, cinematic but physically plausible lighting.", poster: "/assets/prompts/awesome-images/gpt-image-2-showcase-complex.png", alt: "GPT Image 2 storyboard prompt example" },
      ],
      why: { eyebrow: "Why use Flatkey for GPT Image 2?", title: "A repeatable GPT Image 2 workflow for product images", description: "Keep image controls and image-input token dimensions together so a prompt test can become a reproducible API request.", cards: [
        { title: "All documented controls in one place", body: "Set n, size, quality, output format, background, and moderation before you hand the request to the console." },
        { title: "Image input stays explicit", body: "The catalog exposes token and image-input dimensions; this is not a separate fee for output pixel size, and the final amount depends on request and usage fields." },
        { title: "Marketing-ready starting points", body: "Use product, ad, and storyboard examples as editable starting prompts rather than generic image filler." },
        { title: "No unsupported promises", body: "The page does not promise transparent output, free generation, or a fixed per-image price when those facts are not verified." },
      ] },
      related: { eyebrow: "Related models", title: "More GPT Image 2 alternatives and image generator APIs", description: "Explore other image routes when you need a different generation or editing workflow.", cards: [] },
      faqTitle: { beforeBreak: "GPT Image 2 API", afterBreak: "pricing and prompt questions" },
      faqDescription: "Answers about GPT Image 2 API access, image-generation fields, token and image-input pricing, formats, and background behavior.",
      faq: [
        { question: "What is GPT Image 2?", answer: "GPT Image 2 is the OpenAI image model available through /v1/images/generations." },
        { question: "How do I access GPT Image 2?", answer: "Configure a request, then use the normal Flatkey account and API-key flow." },
        { question: "What are the GPT Image 2 API endpoints?", answer: "Use /v1/images/generations for new images or /v1/images/edits for image edits with model ID gpt-image-2." },
        { question: "How much does GPT Image 2 cost?", answer: "The Flatkey catalog lists $4 input tokens, $24 output tokens, $1 cache tokens, and $6.40 image-input tokens per 1M units; this is not one fixed per-image price or a separate pixel-size fee. OpenAI's standard image reference is $8/M image input, $2/M cached image input, and $30/M image output, while Batch uses $4/$1/$15." },
        { question: "What sizes and formats are supported?", answer: "Flatkey presets are 1024x1024, 1536x1024, 1024x1536, and auto. OpenAI documents a wider size range; PNG, JPEG, and WebP are listed here, with transparent output requiring PNG or WebP upstream." },
        { question: "Can GPT Image 2 create transparent backgrounds?", answer: "Yes, OpenAI's image guide documents gpt-image-2 preview support for background=transparent with PNG or WebP. The Flatkey route may expose only the controls shown in its current request form, so verify the live request before relying on transparency." },
        { question: "Is GPT Image 2 free or available without signup?", answer: "Free or no-signup generation is not verified. Use the normal Flatkey access flow and dated pricing block." },
      ],
    },
  },
  "kimi-k3": {
    seo: { title: "Kimi K3 API and pricing for long-context workflows | Flatkey", description: "Use Moonshot AI's Kimi K3 through Flatkey's compatible API routes for long-horizon coding and knowledge work, with a 1M-token context, native vision, and current catalog pricing." },
    seoByLocale: {
      en: { title: "Kimi K3 API and pricing for long-context workflows | Flatkey", description: "Use Moonshot AI's Kimi K3 through Flatkey's compatible API routes for long-horizon coding and knowledge work, with a 1M-token context, native vision upstream, file fields on Flatkey, and current catalog pricing." },
    },
    landingContent: {
      hero: { title: "Kimi K3 API for long-context coding and research", description: "Kimi K3 is Moonshot AI's flagship model for long-horizon coding and knowledge work. Moonshot documents a 1M-token context, native visual understanding, and open-weight availability; Flatkey exposes compatible chat and messages routes with the catalog rates below." },
      performance: { eyebrow: "Performance", title: "Kimi K3 API performance and availability", description: "Live Flatkey request telemetry appears here when enough Kimi K3 traffic is available; no benchmark ranking is inferred." },
      activity: { eyebrow: "Activity", title: "Kimi K3 API usage and request activity", description: "This chart uses live Flatkey request data for Kimi K3 and stays unreported until enough traffic is collected." },
      pricing: { title: "Kimi K3 API pricing for long-context requests", description: "These are current Flatkey catalog token rates; they are not a consumer subscription price or a promise of free access.", note: "Rates are shown per 1M tokens; verify the dated Flatkey catalog block before scaling.", rows: [
        { label: "Flatkey input", value: "$2.40", detail: "per 1M tokens" }, { label: "Flatkey output", value: "$12.00", detail: "per 1M tokens" }, { label: "Flatkey cache", value: "$0.24", detail: "per 1M tokens" },
      ] },
      capabilitiesEyebrow: "Kimi K3 capabilities", capabilitiesTitle: "Kimi K3 capabilities for coding, files, and research", capabilitiesDescription: "These cards describe practical workflows; upstream facts and Flatkey route fields remain separated below.",
      capabilities: [
        { title: "Long-context knowledge work", body: "Work across long documents, codebases, and research notes while keeping the relevant material in one workflow." },
        { title: "Vision and file-aware analysis", body: "Use visual understanding upstream and the file inputs listed for the hosted route when a task needs richer context." },
        { title: "Coding and research assistance", body: "Draft, inspect, explain, and transform technical material for engineering and knowledge teams." },
        { title: "Flexible client integration", body: "Bring the model into compatible chat or messages clients while keeping the same model identity and account controls." },
      ],
      comparison: { eyebrow: "Hosted API facts", title: "Kimi K3 hosted API vs open-weight and local options", description: "Moonshot documents open-weight availability upstream; this page describes Flatkey's hosted API route and does not promise native local execution inside Flatkey.", baselineLabel: "Moonshot open-weight / local option", currentLabel: "Verified Flatkey hosted API", rows: [
        { label: "Hosted endpoint", baseline: "Unknown", current: "/v1/chat/completions and /v1/messages" },
        { label: "Context", baseline: "Unknown", current: "1,048,576 tokens" },
        { label: "Input modality", baseline: "Native vision plus documented upstream inputs", current: "Text/file fields in Flatkey catalog" },
        { label: "Downloadable weights", baseline: "Documented by Moonshot upstream", current: "Flatkey local delivery not verified" },
        { label: "Free access", baseline: "Unknown", current: "Not promised; paid token rates apply" },
        { label: "Upstream vs Flatkey", baseline: "Moonshot open-weight release and local use are upstream options", current: "Flatkey routes and bills hosted API access" },
      ] },
      api: { eyebrow: "API", title: "Use the Kimi K3 API with OpenAI or Anthropic clients", description: "Choose the compatible route documented by your client and set model to kimi-k3; Moonshot upstream access and Flatkey hosted routing are separate services.", items: [
        { title: "Model ID", detail: "kimi-k3" },
        { title: "OpenAI-compatible", detail: "POST /v1/chat/completions" },
        { title: "Anthropic-compatible", detail: "POST /v1/messages" },
        { title: "Boundary", detail: "Compatible routes do not prove local or open-source support." },
      ] },
      why: { eyebrow: "Why use Flatkey for Kimi K3?", title: "A long-context Kimi K3 workflow for coding and research", description: "Make the two compatible routes, file input, and token dimensions visible before you build an agent or knowledge workflow.", cards: [
        { title: "Two client paths", body: "Choose /v1/chat/completions for an OpenAI-shaped client or /v1/messages for an Anthropic-shaped client." },
        { title: "Document-first use cases", body: "Use the verified file modality and 1,048,576-token context for long documents, codebases, and research notes." },
        { title: "Hosted boundary is clear", body: "Moonshot's open-weight release is an upstream option; this page only promises Flatkey hosted API access, not native local hardware support." },
        { title: "Predictable token accounting", body: "See input, output, and cache rates in the pricing block before scaling a workflow." },
      ] },
      related: { eyebrow: "Related models", title: "More Moonshot AI and long-context API models", description: "Compare adjacent text models when your workload needs a different context or endpoint.", cards: [] },
      faqTitle: { beforeBreak: "Kimi K3 API", afterBreak: "pricing and local-use questions" },
      faqDescription: "Answers about Kimi K3 API routes, model ID, long context, file input, pricing, hosted access, and local-deployment boundaries.",
      faq: [
        { question: "What is Kimi K3?", answer: "Kimi K3 is Moonshot AI's long-context model with a documented 1,048,576-token context and native visual understanding. Flatkey exposes a hosted compatible API route." },
        { question: "How do I use Kimi K3?", answer: "Create a Flatkey API key, choose /v1/chat/completions or /v1/messages, and set model to kimi-k3." },
        { question: "What is the Kimi K3 API model ID?", answer: "The verified model ID is kimi-k3." },
        { question: "How much does Kimi K3 cost?", answer: "Flatkey's current catalog rates are $2.40 input, $12 output, and $0.24 cache per 1M tokens. Moonshot's direct API reference is $3 input, $0.30 cache-hit input, and $15 output; check the applicable account and route." },
        { question: "Is Kimi K3 free?", answer: "The current catalog shows paid token rates; do not promise free access." },
        { question: "Is Kimi K3 open source or available locally?", answer: "Moonshot documents an open-weight release that can be downloaded upstream. Flatkey's page provides hosted API access; native Flatkey local deployment and hardware requirements are not promised." },
        { question: "Who makes Kimi K3?", answer: "The catalog vendor is Moonshot AI; Flatkey provides routing and billing." },
      ],
    },
  },
  "deepseek-v4-pro": {
    seo: { title: "DeepSeek V4 Pro API and pricing | Flatkey", description: "Use DeepSeek V4 Pro through Flatkey's OpenAI- or Anthropic-compatible API routes for reasoning and coding workflows, with a 1M-token context and UTC time-tiered catalog pricing." },
    seoByLocale: {
      en: { title: "DeepSeek V4 Pro API and pricing | Flatkey", description: "Use DeepSeek V4 Pro through Flatkey's OpenAI- or Anthropic-compatible API routes for reasoning and coding workflows, with a 1M-token context and UTC time-tiered catalog pricing." },
    },
    rows: [
      { label: "Peak UTC cache-miss / cache-hit / output", flatkey: "$1.32 / $0.044 / $3.96", official: "DeepSeek direct reference" },
      { label: "Off-peak UTC cache-miss / cache-hit / output", flatkey: "$0.66 / $0.022 / $1.98", official: "DeepSeek direct reference" },
      { label: "Context", flatkey: "", value: "1,048,576 tokens" },
      { label: "Modalities", flatkey: "", value: "Text · file" },
    ],
    landingContent: {
      hero: { title: "DeepSeek V4 Pro API for reasoning and coding", description: "DeepSeek V4 Pro is the documented model ID for Flatkey's compatible chat and messages routes. DeepSeek documents a 1M-token context and up to 384K output tokens; Flatkey's current catalog applies UTC time-tiered rates below." },
      performance: { eyebrow: "Performance", title: "DeepSeek V4 Pro API performance and availability", description: "Live Flatkey request telemetry appears here when enough DeepSeek V4 Pro traffic is available; no coding benchmark is inferred." },
      activity: { eyebrow: "Activity", title: "DeepSeek V4 Pro API usage and request activity", description: "This chart uses live Flatkey request data for DeepSeek V4 Pro and stays unreported until enough traffic is collected." },
      pricing: { title: "DeepSeek V4 Pro API pricing by UTC tier", description: "These are Flatkey's current UTC catalog tiers. Keep peak and off-peak token dimensions together; they are not a single official direct rate.", note: "Rates are shown per 1M tokens. DeepSeek's direct reference uses cache-miss input / cache-hit input / output and peak UTC windows (01–04 and 06–10 Monday–Friday); verify the dated Flatkey catalog block before scaling.", rows: [
        { label: "Peak (UTC)", value: "$1.32 / $0.044 / $3.96", detail: "cache-miss input / cache-hit input / output per 1M tokens" }, { label: "Off-peak (UTC)", value: "$0.66 / $0.022 / $1.98", detail: "cache-miss input / cache-hit input / output per 1M tokens" },
      ] },
      capabilitiesEyebrow: "DeepSeek V4 Pro capabilities", capabilitiesTitle: "DeepSeek V4 Pro capabilities for reasoning and coding", capabilitiesDescription: "These cards describe practical workflows; exact route, context, and billing fields remain in the API and comparison sections.",
      capabilities: [
        { title: "Long-context reasoning", body: "Plan, compare, and transform large technical materials within the documented context available to the model route." },
        { title: "Text and file workflows", body: "Use the verified text and file inputs for coding, document analysis, and research tasks on the hosted route." },
        { title: "Engineering and coding assistance", body: "Turn technical prompts into explanations, implementation drafts, reviews, and structured next steps." },
        { title: "Compatible application integration", body: "Connect the model to OpenAI-shaped or Anthropic-shaped clients while keeping routing and account controls in Flatkey." },
      ],
      comparison: { eyebrow: "Compare documented fields", title: "DeepSeek V4 Pro vs V4 Flash: API and context fields", description: "No quality or coding-performance ranking is asserted.", baselineLabel: "DeepSeek V4 Flash", currentLabel: "DeepSeek V4 Pro", rows: [
        { label: "Model ID", baseline: "deepseek-v4-flash", current: "deepseek-v4-pro" },
        { label: "Endpoint paths", baseline: "Verify before publishing", current: "/v1/chat/completions, /v1/messages" },
        { label: "Context", baseline: "Unknown", current: "1M tokens; up to 384K output" },
        { label: "Modalities", baseline: "Verify the selected V4 Flash route", current: "Text/file fields verified for Flatkey" },
        { label: "Benchmark/coding ranking", baseline: "Not asserted", current: "Not asserted" },
      ] },
      api: { eyebrow: "API", title: "Use the DeepSeek V4 Pro API with compatible clients", description: "Set the exact model ID and choose the compatible path selected by your client.", items: [
        { title: "Model ID", detail: "deepseek-v4-pro" },
        { title: "OpenAI-compatible", detail: "POST /v1/chat/completions" },
        { title: "Anthropic-compatible", detail: "POST /v1/messages" },
        { title: "Verified inputs", detail: "Text and file fields for this Flatkey route; provider vision/file support is endpoint-specific." },
      ] },
      why: { eyebrow: "Why use Flatkey for DeepSeek V4 Pro?", title: "A clear DeepSeek V4 Pro API workflow for coding", description: "The page makes the time tier, endpoint choice, context field, and deployment boundaries explicit for production planning.", cards: [
        { title: "UTC-aware billing", body: "Keep peak and off-peak expressions together; do not replace the catalog rule with one blended rate." },
        { title: "OpenAI or Anthropic route", body: "Use /v1/chat/completions or /v1/messages with the exact deepseek-v4-pro model ID." },
        { title: "Text and file boundary", body: "The verified catalog lists text and file modalities; vision and other inputs are not inferred." },
        { title: "Model facts over rankings", body: "Compare integration fields and context data without publishing an unsupported benchmark or coding-superiority claim." },
      ] },
      related: { eyebrow: "Related models", title: "More DeepSeek API models and pricing", description: "Compare adjacent DeepSeek routes when you need another context or billing profile.", cards: [] },
      faqTitle: { beforeBreak: "DeepSeek V4 Pro API", afterBreak: "pricing and local-use questions" },
      faqDescription: "Answers about DeepSeek V4 Pro API paths, UTC peak and off-peak pricing, context, verified inputs, and deployment boundaries.",
      faq: [
        { question: "What is DeepSeek V4 Pro?", answer: "DeepSeek V4 Pro is documented with a 1M-token context and up to 384K output tokens. The Flatkey page exposes its compatible API routes and catalog pricing." },
        { question: "How do I use DeepSeek V4 Pro?", answer: "Send a request to /v1/chat/completions or /v1/messages with model ID deepseek-v4-pro." },
        { question: "What is the DeepSeek V4 Pro API model name?", answer: "The verified model ID is deepseek-v4-pro." },
        { question: "How is DeepSeek V4 Pro priced?", answer: "Flatkey's current catalog lists UTC peak cache-miss/cache-hit/output at $1.32/$0.044/$3.96 and off-peak at $0.66/$0.022/$1.98 per 1M tokens. DeepSeek's direct reference uses the same dimensions and publishes peak windows at 01–04 and 06–10 UTC Monday–Friday; direct account terms can differ." },
        { question: "Does DeepSeek V4 Pro support local download or open-source use?", answer: "DeepSeek has documented an open-source V4 Preview upstream. This Flatkey page provides hosted API routing; local packaging and hardware requirements are not promised here." },
        { question: "Does DeepSeek V4 Pro support vision or multimodal input?", answer: "Text and file fields are verified for this Flatkey route. Do not infer native V4 Pro vision support from the separate V4 Flash Vision Files API documentation." },
        { question: "Is DeepSeek V4 Pro better for coding than V4 Flash?", answer: "This page does not publish a benchmark or quality ranking." },
      ],
    },
  },
  "minimax-h3": {
    seo: { title: "MiniMax H3 video model — API, prompting, and pricing | Flatkey", description: "Create multimodal videos with MiniMax-H3 through Flatkey: 768P or 2K output, 4–15 seconds, supported ratios, prompting examples, and catalog pricing." },
    landingContent: {
      hero: { title: "MiniMax H3 AI video generator and API", description: "MiniMax-H3 is MiniMax's multimodal video model. Official documentation lists text, image, video, and audio references, 768P or 2K output, and 4–15-second clips; Flatkey exposes the asynchronous /v1/videos flow." },
      performance: { eyebrow: "Performance", title: "MiniMax-H3 video API performance and availability", description: "Live Flatkey request telemetry appears here when enough MiniMax-H3 traffic is available; no quality ranking is inferred." },
      activity: { eyebrow: "Activity", title: "MiniMax-H3 video API usage and generation activity", description: "This chart uses live Flatkey generation requests and stays unreported until enough traffic is collected." },
      pricing: { title: "MiniMax H3 video API pricing by resolution and duration", description: "Flatkey's catalog base is $0.08 per second at the 768P reference. Official MiniMax pay-as-you-go rates differ by resolution, so review the live estimate for your request.", note: "Use the dated Flatkey catalog estimate for settlement. MiniMax documents $0.08/sec at 768P and $0.13/sec at 2K; the first five input images are free, later images are $0.04 each, and video input is billed from input seconds and output resolution.", rows: [
        { label: "Flatkey catalog base (768P reference)", value: "$0.08", detail: "per second" }, { label: "Official 768P reference", value: "$0.08", detail: "per second; verify current provider terms" }, { label: "Official 2K reference", value: "$0.13", detail: "per second; verify current provider terms" }, { label: "Reference video", value: "Live estimate", detail: "input-video seconds and resolution" },
      ] },
      capabilitiesEyebrow: "MiniMax-H3 video capabilities", capabilitiesTitle: "MiniMax H3 video generation and production capabilities", capabilitiesDescription: "Use these documented capabilities to plan a production workflow; request fields remain in the API section below.",
      capabilities: [
        { title: "Text-to-video and image-to-video", body: "Start a scene from a written brief or a designed frame and develop it into a short clip." },
        { title: "Reference-led motion", body: "Use text, image, video, or audio references to keep the subject and creative direction coherent." },
        { title: "Camera and pacing direction", body: "Shape how the camera, subject movement, duration, and framing progress through the shot." },
        { title: "Production-ready variants", body: "Create product, UGC, and storyboard versions that can move from testing into an editing workflow." },
      ],
      comparison: { eyebrow: "Hosted API facts", title: "MiniMax H3 hosted API vs open-weight or local deployment", description: "Compare documented hosted request fields with open-weight or local assumptions; MiniMax upstream terms and Flatkey routing are separate.", baselineLabel: "Open-weight / local deployment", currentLabel: "Verified hosted API", rows: [
        { label: "Resolution", baseline: "768P base; 2K via hosted regeneration", current: "768P or 2K" },
        { label: "Duration", baseline: "4–15 seconds", current: "4–15 seconds" },
        { label: "Ratio", baseline: "Text-to-video requires a fixed ratio; adaptive is for reference routes", current: "Route-specific fixed/adaptive ratio rules" },
        { label: "AIGC watermark", baseline: "Not verified", current: "Flatkey route field; availability varies" },
        { label: "ComfyUI or local weights", baseline: "MiniMax documents open-source weights and ComfyUI options upstream", current: "Native Flatkey local/ComfyUI delivery not verified" },
        { label: "Upstream vs Flatkey", baseline: "Open-source/local use follows MiniMax upstream documentation", current: "Flatkey routes and bills hosted API access" },
      ] },
      api: { eyebrow: "API", title: "Use the MiniMax H3 video API with an async task", description: "MiniMax's upstream API and Flatkey's hosted route use different paths; save the returned task ID and retrieve the result when ready.", items: [
        { title: "MiniMax upstream", detail: "POST /v2/video_generation" },
        { title: "Flatkey hosted route", detail: "POST /v1/videos" },
        { title: "Model ID", detail: "MiniMax-H3" },
        { title: "Fields", detail: "content[] with a non-empty text item, resolution, duration, and ratio; aigc_watermark is a Flatkey route field whose availability is route-specific." },
        { title: "Result", detail: "Use the returned task ID with the content endpoint." },
      ] },
      promptLibraryTitle: "MiniMax-H3 prompting guide: product, UGC, and storyboard shots",
      promptLibraryDescription: "Describe the subject, action, camera position, duration, and output ratio; keep the request fields separate from the scene prompt.",
      promptLibrary: [
        { key: "minimax-h3-product", label: "Product motion", prompt: "Six-second product shot: a matte black bottle rotates slowly on a wet stone pedestal, one controlled push-in, small water highlights, uncluttered background, end on a steady hero frame.", poster: "/assets/cli/product-reveal.png", video: "/assets/cli/product-reveal.mp4", alt: "MiniMax-H3 product motion prompt example" },
        { key: "minimax-h3-ugc", label: "UGC ad clip", prompt: "Eight-second vertical UGC-style clip: a creator lifts a compact coffee maker, points to the front control, then smiles to camera; handheld but stable, natural window light, leave the spoken words to the audio track.", poster: "/assets/cli/ugc-ad-clips.png", video: "/assets/cli/ugc-ad-clips.mp4", alt: "MiniMax-H3 UGC ad prompt example" },
        { key: "minimax-h3-storyboard", label: "Storyboard shot", prompt: "Ten-second wide establishing shot: a courier crosses a rain-soaked plaza toward a lit station, a slow lateral camera move follows, reflections remain consistent, finish with the subject centered under the sign.", poster: "/assets/cli/localized-variants.png", video: "/assets/cli/localized-variants.mp4", alt: "MiniMax-H3 storyboard prompt example" },
      ],
      why: { eyebrow: "Why use Flatkey for MiniMax-H3?", title: "A practical MiniMax H3 video API workflow", description: "Keep video settings, asynchronous task handling, and pricing boundaries visible while you move from a prompt draft to a real request.", cards: [
        { title: "Video fields are explicit", body: "Choose 768P or 2K, 4–15 seconds, a supported ratio, and the AIGC watermark boolean." },
        { title: "Prompting guide included", body: "Product, UGC, and storyboard examples describe subject, action, camera, and ending rather than repeating a generic cinematic prompt." },
        { title: "Async result path", body: "Save the task ID returned by POST /v1/videos and retrieve the generated content when the task is ready." },
        { title: "ComfyUI/local boundary", body: "MiniMax documents open-source H3 weights and ComfyUI/local options upstream; this page only promises Flatkey hosted delivery, not native local packaging." },
      ] },
      related: { eyebrow: "Related models", title: "More MiniMax H3 alternatives and AI video generator APIs", description: "Explore other video routes when you need different durations, ratios, or audio controls.", cards: [] },
      faqTitle: { beforeBreak: "MiniMax H3 video API", afterBreak: "pricing and ComfyUI questions" },
      faqDescription: "Answers about MiniMax-H3 video API fields, prompting, per-second pricing, asynchronous tasks, ComfyUI/local boundaries, and moderation claims.",
      faq: [
        { question: "Which MiniMax-H3 fields can I configure here?", answer: "Configure resolution, duration, ratio, and AIGC watermark before opening the console." },
        { question: "What is MiniMax-H3?", answer: "MiniMax-H3 is MiniMax's multimodal video model, with documented text, image, video, and audio references and 768P or 2K output up to 15 seconds." },
        { question: "How much does MiniMax-H3 cost?", answer: "Flatkey's catalog base is $0.08 per second at the 768P reference. MiniMax's published pay-as-you-go references are $0.08/sec at 768P and $0.13/sec at 2K; the first five input images are free, later images are $0.04 each, and final Flatkey estimates can vary with request inputs." },
        { question: "How do I use the MiniMax-H3 video API?", answer: "Configure the fields, create a Flatkey API key, send POST /v1/videos with model MiniMax-H3, then keep the asynchronous task ID for the content lookup." },
        { question: "Does MiniMax-H3 have a prompting guide?", answer: "Use the page examples as a starting point: name the subject, action, camera movement, duration, and ending, then set ratio and resolution as request fields." },
        { question: "Does Flatkey provide native MiniMax-H3 ComfyUI or local installation?", answer: "MiniMax documents open-source H3 weights and ComfyUI/local options upstream. Flatkey's hosted route is available here; native Flatkey ComfyUI delivery and local hardware packaging are not promised." },
        { question: "Is MiniMax-H3 censored or unrestricted?", answer: "The current catalog does not publish a moderation policy for this model, so the page makes no unrestricted-use claim." },
        { question: "Are MiniMax-H3 prompt drafts executed on this public page?", answer: "The public page saves the video settings and prompt draft first. Sign up or open the console to run POST /v1/videos with an API key." },
      ],
    },
  },
};

// These are the only priority IDs that have a dedicated public landing URL.
// Keep the canonical slugs stable even when the live catalog preserves vendor
// casing (notably `MiniMax-H3`).
const PRIORITY_CANONICAL_SLUGS: Record<string, string> = {
  "gpt-5-6-sol": "gpt-5.6-sol",
  "gpt-image-2": "gpt-image-2",
  "kimi-k3": "kimi-k3",
  "deepseek-v4-pro": "deepseek-v4-pro",
  "minimax-h3": "minimax-h3",
};

/**
 * Search metadata is localized independently from the editorial body.  Keep
 * every supported locale explicit here so a high-priority page never falls
 * back to an English title/description simply because the live catalog uses a
 * vendor-specific model id.  Product names, endpoint paths, model ids, and
 * currency values intentionally remain literal; the surrounding intent is
 * translated for the locale.
 */
const PRIORITY_SEO_BY_LOCALE: Record<string, Partial<Record<Locale, { title: string; description: string }>>> = {
  "gpt-5-6-sol": {
    en: {
      title: "GPT-5.6 Sol API and pricing for developers | Flatkey",
      description: "Build with GPT-5.6 Sol through Flatkey: OpenAI-compatible API access, a long context window, image input, current Flatkey token rates, and one API key.",
    },
    zh: {
      title: "GPT-5.6 Sol API 与价格 | Flatkey",
      description: "通过 Flatkey 使用 GPT-5.6 Sol，获得 OpenAI 兼容 API、1,048,576 token 上下文、文本/图片/文件输入、当前 token 价格和一个 API Key。",
    },
    es: {
      title: "API y precios de GPT-5.6 Sol | Flatkey",
      description: "Usa GPT-5.6 Sol con Flatkey mediante una API compatible con OpenAI, contexto de 1.048.576 tokens, entradas de texto, imagen y archivo, precios actuales y una sola API key.",
    },
    fr: {
      title: "API et tarifs de GPT-5.6 Sol | Flatkey",
      description: "Utilisez GPT-5.6 Sol avec Flatkey via une API compatible OpenAI, un contexte de 1 048 576 tokens, des entrées texte, image et fichier, les tarifs actuels et une seule clé API.",
    },
    pt: {
      title: "API e preços do GPT-5.6 Sol para desenvolvedores | Flatkey",
      description: "Crie com o GPT-5.6 Sol pela Flatkey: API compatível com OpenAI, contexto longo, entrada de imagens, preços atuais do catálogo e uma única chave de API.",
    },
    ru: {
      title: "API и цены GPT-5.6 Sol | Flatkey",
      description: "Используйте GPT-5.6 Sol через Flatkey с OpenAI-совместимым API, контекстом 1 048 576 токенов, текстовыми, графическими и файловыми входами, актуальными тарифами и одним API-ключом.",
    },
    ja: {
      title: "GPT-5.6 Sol API と料金 | Flatkey",
      description: "FlatkeyでGPT-5.6 Solを利用。OpenAI互換API、1,048,576トークンのコンテキスト、テキスト・画像・ファイル入力、最新のトークン料金、1つのAPIキーに対応します。",
    },
    vi: {
      title: "API và bảng giá GPT-5.6 Sol | Flatkey",
      description: "Dùng GPT-5.6 Sol qua Flatkey với API tương thích OpenAI, ngữ cảnh 1.048.576 token, đầu vào văn bản, hình ảnh và tệp, giá token hiện tại và một khóa API.",
    },
    de: {
      title: "GPT-5.6 Sol API und Preise | Flatkey",
      description: "Nutzen Sie GPT-5.6 Sol über Flatkey mit OpenAI-kompatibler API, einem Kontext von 1.048.576 Tokens, Text-, Bild- und Dateieingaben, aktuellen Tokenpreisen und einem API-Schlüssel.",
    },
    id: {
      title: "API dan harga GPT-5.6 Sol | Flatkey",
      description: "Gunakan GPT-5.6 Sol melalui Flatkey dengan API yang kompatibel dengan OpenAI, konteks 1.048.576 token, input teks, gambar, dan file, harga token terbaru, serta satu kunci API.",
    },
  },
  "gpt-image-2": {
    en: {
      title: "GPT Image 2 image generator — API and pricing | Flatkey",
      description: "Create and edit images with GPT Image 2 through Flatkey: image API access, current catalog pricing, size and quality controls, formats, background, and moderation settings.",
    },
    zh: {
      title: "GPT Image 2 API 与图像生成器 | Flatkey",
      description: "通过 Flatkey 准备 GPT Image 2 请求，使用图像 API，并配置当前 token/图像尺寸价格、尺寸、质量、格式、背景和审核设置。",
    },
    es: {
      title: "API y generador de imágenes GPT Image 2 | Flatkey",
      description: "Prepara solicitudes de GPT Image 2 con Flatkey: acceso a la API de imágenes, precios por tokens y dimensiones, controles de tamaño y calidad, formatos, fondo y moderación.",
    },
    fr: {
      title: "API et générateur d’images GPT Image 2 | Flatkey",
      description: "Préparez des requêtes GPT Image 2 avec Flatkey : accès à l’API d’image, tarifs par tokens et dimensions, contrôles de taille et de qualité, formats, arrière-plan et modération.",
    },
    pt: {
      title: "Gerador de imagens GPT Image 2 — API e preços | Flatkey",
      description: "Crie e edite imagens com o GPT Image 2 pela Flatkey: acesso à API de imagens, preços do catálogo, controles de tamanho e qualidade, formatos, fundo e moderação.",
    },
    ru: {
      title: "API и генератор изображений GPT Image 2 | Flatkey",
      description: "Подготовьте запросы GPT Image 2 через Flatkey: доступ к API изображений, цены за токены и размеры, настройки разрешения и качества, формата, фона и модерации.",
    },
    ja: {
      title: "GPT Image 2 API と画像生成 | Flatkey",
      description: "FlatkeyでGPT Image 2のリクエストを準備。画像API、トークン・画像サイズ別料金、サイズと品質、形式、背景、モデレーションを設定できます。",
    },
    vi: {
      title: "API và trình tạo ảnh GPT Image 2 | Flatkey",
      description: "Chuẩn bị yêu cầu GPT Image 2 qua Flatkey với API hình ảnh, giá theo token và kích thước, tùy chọn kích thước, chất lượng, định dạng, nền và kiểm duyệt.",
    },
    de: {
      title: "GPT Image 2 API und Bildgenerator | Flatkey",
      description: "Bereiten Sie GPT Image 2-Anfragen über Flatkey vor: Bild-API, aktuelle Token- und Dimensionspreise sowie Einstellungen für Größe, Qualität, Format, Hintergrund und Moderation.",
    },
    id: {
      title: "API dan generator gambar GPT Image 2 | Flatkey",
      description: "Siapkan permintaan GPT Image 2 melalui Flatkey dengan akses API gambar, harga token dan dimensi saat ini, serta pengaturan ukuran, kualitas, format, latar, dan moderasi.",
    },
  },
  "kimi-k3": {
    en: {
      title: "Kimi K3 API and pricing for long-context workflows | Flatkey",
      description: "Use Moonshot AI's Kimi K3 through Flatkey's compatible API routes for long-horizon coding and knowledge work, with a 1M-token context, file input, and current catalog pricing.",
    },
    zh: {
      title: "Kimi K3 API 与价格 | Flatkey",
      description: "通过 Flatkey 使用 Moonshot AI 的 Kimi K3，支持 OpenAI 和 Anthropic 兼容端点、1,048,576 token 上下文、文件输入及当前 token 价格。",
    },
    es: {
      title: "API y precios de Kimi K3 | Flatkey",
      description: "Usa Kimi K3 de Moonshot AI con Flatkey mediante endpoints compatibles con OpenAI y Anthropic, contexto de 1.048.576 tokens, entrada de archivos y precios actuales.",
    },
    fr: {
      title: "API et tarifs de Kimi K3 | Flatkey",
      description: "Utilisez Kimi K3 de Moonshot AI avec Flatkey via des endpoints compatibles OpenAI et Anthropic, un contexte de 1 048 576 tokens, l’entrée de fichiers et les tarifs actuels.",
    },
    pt: {
      title: "API e preços do Kimi K3 para contexto longo | Flatkey",
      description: "Use o Kimi K3 da Moonshot AI pela Flatkey em fluxos de programação e conhecimento de longo prazo, com contexto de 1 milhão de tokens, arquivos e preços atuais do catálogo.",
    },
    ru: {
      title: "API и цены Kimi K3 | Flatkey",
      description: "Используйте Kimi K3 от Moonshot AI через Flatkey с OpenAI- и Anthropic-совместимыми endpoint, контекстом 1 048 576 токенов, загрузкой файлов и актуальными тарифами.",
    },
    ja: {
      title: "Kimi K3 API と料金 | Flatkey",
      description: "FlatkeyでMoonshot AIのKimi K3を利用。OpenAI・Anthropic互換エンドポイント、1,048,576トークンのコンテキスト、ファイル入力、最新料金に対応します。",
    },
    vi: {
      title: "API và bảng giá Kimi K3 | Flatkey",
      description: "Dùng Kimi K3 của Moonshot AI qua Flatkey với endpoint tương thích OpenAI và Anthropic, ngữ cảnh 1.048.576 token, đầu vào tệp và giá hiện tại.",
    },
    de: {
      title: "Kimi K3 API und Preise | Flatkey",
      description: "Nutzen Sie Kimi K3 von Moonshot AI über Flatkey mit OpenAI- und Anthropic-kompatiblen Endpunkten, 1.048.576-Token-Kontext, Dateieingabe und aktuellen Tokenpreisen.",
    },
    id: {
      title: "API dan harga Kimi K3 | Flatkey",
      description: "Gunakan Kimi K3 dari Moonshot AI melalui Flatkey dengan endpoint yang kompatibel dengan OpenAI dan Anthropic, konteks 1.048.576 token, input file, dan harga token terbaru.",
    },
  },
  "deepseek-v4-pro": {
    en: {
      title: "DeepSeek V4 Pro API and pricing | Flatkey",
      description: "Use DeepSeek V4 Pro through Flatkey's OpenAI- or Anthropic-compatible API routes for reasoning and coding workflows, with a 1M-token context and UTC time-tiered catalog pricing.",
    },
    zh: {
      title: "DeepSeek V4 Pro API 与动态价格 | Flatkey",
      description: "通过 Flatkey 调用 DeepSeek V4 Pro，支持 OpenAI 或 Anthropic 兼容端点、1,048,576 token 上下文、文件输入及按 UTC 时段变化的价格。",
    },
    es: {
      title: "API y precios dinámicos de DeepSeek V4 Pro | Flatkey",
      description: "Llama a DeepSeek V4 Pro con Flatkey mediante endpoints compatibles con OpenAI o Anthropic, contexto de 1.048.576 tokens, entrada de archivos y precios por franjas UTC.",
    },
    fr: {
      title: "API et tarifs dynamiques de DeepSeek V4 Pro | Flatkey",
      description: "Appelez DeepSeek V4 Pro via Flatkey avec des endpoints compatibles OpenAI ou Anthropic, un contexte de 1 048 576 tokens, l’entrée de fichiers et des tarifs horaires en UTC.",
    },
    pt: {
      title: "API e preços do DeepSeek V4 Pro | Flatkey",
      description: "Use o DeepSeek V4 Pro pela Flatkey em fluxos de raciocínio e programação, com endpoints compatíveis com OpenAI ou Anthropic, contexto de 1 milhão de tokens e preços do catálogo por faixa UTC.",
    },
    ru: {
      title: "API и динамические цены DeepSeek V4 Pro | Flatkey",
      description: "Вызывайте DeepSeek V4 Pro через Flatkey с OpenAI- или Anthropic-совместимыми endpoint, контекстом 1 048 576 токенов, загрузкой файлов и тарифами по времени UTC.",
    },
    ja: {
      title: "DeepSeek V4 Pro API と動的料金 | Flatkey",
      description: "FlatkeyでDeepSeek V4 Proを呼び出し。OpenAI・Anthropic互換エンドポイント、1,048,576トークンのコンテキスト、ファイル入力、UTC時間帯別料金に対応します。",
    },
    vi: {
      title: "API và giá theo thời điểm của DeepSeek V4 Pro | Flatkey",
      description: "Gọi DeepSeek V4 Pro qua Flatkey với endpoint tương thích OpenAI hoặc Anthropic, ngữ cảnh 1.048.576 token, đầu vào tệp và giá phân theo khung giờ UTC.",
    },
    de: {
      title: "DeepSeek V4 Pro API und dynamische Preise | Flatkey",
      description: "Rufen Sie DeepSeek V4 Pro über Flatkey mit OpenAI- oder Anthropic-kompatiblen Endpunkten, 1.048.576-Token-Kontext, Dateieingabe und UTC-Zeitstaffelpreisen auf.",
    },
    id: {
      title: "API dan harga dinamis DeepSeek V4 Pro | Flatkey",
      description: "Panggil DeepSeek V4 Pro melalui Flatkey dengan endpoint yang kompatibel dengan OpenAI atau Anthropic, konteks 1.048.576 token, input file, dan harga berdasarkan waktu UTC.",
    },
  },
  "minimax-h3": {
    en: {
      title: "MiniMax H3 video model — API, prompting, and pricing | Flatkey",
      description: "Create multimodal videos with MiniMax-H3 through Flatkey: 768P or 2K output, 4–15 seconds, supported ratios, prompting examples, and catalog pricing.",
    },
    zh: {
      title: "MiniMax H3 视频 API 与价格 | Flatkey",
      description: "通过 Flatkey 的 /v1/videos 使用 MiniMax H3，配置 768P 或 2K、4–15 秒片段、画面比例及当前每秒价格。",
    },
    es: {
      title: "API de vídeo y precios de MiniMax H3 | Flatkey",
      description: "Usa MiniMax H3 con el endpoint /v1/videos de Flatkey: configura 768P o 2K, clips de 4–15 segundos, proporción y precios actuales por segundo.",
    },
    fr: {
      title: "API vidéo et tarifs de MiniMax H3 | Flatkey",
      description: "Utilisez MiniMax H3 via l’endpoint /v1/videos de Flatkey avec des réglages 768P ou 2K, des clips de 4 à 15 secondes, le ratio et les tarifs actuels par seconde.",
    },
    pt: {
      title: "Modelo de vídeo MiniMax H3 — API, prompts e preços | Flatkey",
      description: "Crie vídeos multimodais com o MiniMax-H3 pela Flatkey: saída 768P ou 2K, clipes de 4–15 segundos, proporções compatíveis, exemplos de prompts e preços do catálogo.",
    },
    ru: {
      title: "Видео API и цены MiniMax H3 | Flatkey",
      description: "Используйте MiniMax H3 через endpoint /v1/videos Flatkey: настройки 768P или 2K, клипы 4–15 секунд, соотношение сторон и актуальная цена за секунду.",
    },
    ja: {
      title: "MiniMax H3 動画API と料金 | Flatkey",
      description: "Flatkeyの/v1/videosでMiniMax H3を利用。768Pまたは2K、4～15秒のクリップ、アスペクト比、最新の秒単価を設定できます。",
    },
    vi: {
      title: "API video và bảng giá MiniMax H3 | Flatkey",
      description: "Dùng MiniMax H3 qua endpoint /v1/videos của Flatkey với tùy chọn 768P hoặc 2K, clip 4–15 giây, tỷ lệ khung hình và giá theo giây hiện tại.",
    },
    de: {
      title: "MiniMax H3 Video-API und Preise | Flatkey",
      description: "Nutzen Sie MiniMax H3 über den Flatkey-Endpunkt /v1/videos mit 768P- oder 2K-Einstellungen, 4–15-Sekunden-Clips, Seitenverhältnis und aktuellen Preisen pro Sekunde.",
    },
    id: {
      title: "API video dan harga MiniMax H3 | Flatkey",
      description: "Gunakan MiniMax H3 melalui endpoint /v1/videos Flatkey dengan pengaturan 768P atau 2K, klip 4–15 detik, rasio, dan harga per detik terbaru.",
    },
  },
};

for (const [modelKey, seoByLocale] of Object.entries(PRIORITY_SEO_BY_LOCALE)) {
  const override = PRIORITY_MODEL_OVERRIDES[modelKey];
  if (override) {
    const mergedSeo = { ...seoByLocale, ...override.seoByLocale };
    override.seoByLocale = Object.fromEntries(
      Object.entries(mergedSeo).map(([locale, value]) => [
        locale,
        value ? { ...value, description: limitSeoDescription(value.description) } : value,
      ]),
    ) as Partial<Record<Locale, { title: string; description: string }>>;
  }
}

// The editorial overrides above are intentionally authored in English so the
// catalog/config layer remains locale-neutral.  Build a locale map against
// those exact source objects rather than relying on a flattened generated
// copy list: this preserves the target-specific wording (including prompts,
// comparison rows, and FAQ answers) when the shared shell calls `t()`.
const prioritySourceTranslationCache: Partial<Record<Locale, Record<string, string>>> = {};
function getPrioritySourceTranslations(locale: Locale): Record<string, string> {
  const cached = prioritySourceTranslationCache[locale];
  if (cached) return cached;

  const merged = { ...getPriorityModelTranslations(locale) };
  for (const [modelKey, override] of Object.entries(PRIORITY_MODEL_OVERRIDES)) {
    if (!override.landingContent) continue;
    Object.assign(
      merged,
      getPriorityModelTranslationMapForSource(modelKey, locale, override.landingContent),
    );
  }
  prioritySourceTranslationCache[locale] = merged;
  return merged;
}

// Image and MiniMax pages have static route configs as well as live pricing
// model routes, so apply their editorial overrides to the exported constants
// before MODEL_CONFIGS is assembled.  This leaves Seedance untouched.
Object.assign(GPT_IMAGE_2_CONFIG, PRIORITY_MODEL_OVERRIDES["gpt-image-2"]);
Object.assign(KIMI_K3_CONFIG, PRIORITY_MODEL_OVERRIDES["kimi-k3"]);
Object.assign(MINIMAX_H3_CONFIG, PRIORITY_MODEL_OVERRIDES["minimax-h3"]);

export const GPT_4_1_MINI_CONFIG: ModelConfig = {
  ...GPT_CONFIG,
  slug: "gpt-4.1-mini",
  modelIds: ["gpt-4.1-mini"],
  displayName: "gpt-4.1-mini",
  modelId: "gpt-4.1-mini",
  officialPrice: "$1.60",
  flatkeyPrice: "$1.07",
  estFlatkey: "$0.001",
  estOfficial: "$0.002",
  rows: [
    { label: "Live flatkey pricing", flatkey: "$1.07", official: "$1.60" },
    { label: "GPT-5 input", flatkey: "$0.27", official: "$0.40" },
    { label: "Cache reads", flatkey: "", value: "up to 50% off" },
    { label: "Coverage", flatkey: "", value: COVERAGE },
  ],
  seo: {
    title: "gpt-4.1-mini API pricing — one OpenAI-compatible key",
    description: "Use gpt-4.1-mini through flatkey.ai with OpenAI-compatible routing, lower token costs, one API key, and unified billing.",
  },
};

export const SONILO_VIDEO_TO_MUSIC_CONFIG: ModelConfig = {
  slug: "sonilo-video-to-music",
  modelIds: ["sonilo-video-to-music"],
  displayName: "sonilo-video-to-music",
  modelId: "sonilo-video-to-music",
  generator: {
    kind: "audio",
    endpoint: "/v1/video-to-music",
    storageKey: "flatkey:model-generator-draft:sonilo-video-to-music",
    protocol: "audio",
    fields: [
      { name: "video_url", label: "Video URL", type: "text", defaultValue: "" },
      { name: "duration_seconds", label: "Duration", type: "number", defaultValue: 30, min: 5, max: 300 },
      { name: "output_format", label: "Output format", type: "select", defaultValue: "mp3", options: ["mp3", "m4a", "wav"] },
      { name: "variants_num", label: "Outputs", type: "number", defaultValue: 1, min: 1, max: 10 },
      { name: "preserve_speech", label: "Preserve speech", type: "boolean", defaultValue: true },
    ],
  },
  officialName: "Sonilo",
  officialPrice: "$0.009",
  flatkeyPrice: "$0.0048",
  estFlatkey: "$0.0048",
  estOfficial: "$0.009",
  examplePrompt:
    "Create a polished music bed for a short product video: keep the video timing, preserve important speech, use a warm electronic style, and deliver a clean loopable ending.",
  priceUnit: "/ request",
  rows: [
    { label: "Live flatkey pricing", flatkey: "$0.0048", official: "$0.009" },
    { label: "Live model data from pricing API", flatkey: "", value: "Sonilo" },
    { label: "Coverage", flatkey: "", value: "Video-to-music · audio · sound" },
  ],
  seo: {
    title: "sonilo-video-to-music API — configure prompts before signup",
    description:
      "Configure sonilo-video-to-music requests on flatkey.ai, save prompt settings locally, then continue to signup or the console.",
  },
  positioning: "Best for video soundtracks, speech-preserving edits, and audio production",
  useCases: ["Audio generation", "Speech preservation", "Audio variants"],
  faq: [
    {
      question: "Does this start a real generation?",
      answer: "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.",
    },
    {
      question: "Where are my edited prompt settings stored?",
      answer: "They are stored in this browser's localStorage so the draft survives the signup handoff.",
    },
  ],
};

export const MODEL_CONFIGS: Record<string, ModelConfig> = {
  [CLAUDE_CONFIG.slug]: CLAUDE_CONFIG,
  [DEEPSEEK_CONFIG.slug]: DEEPSEEK_CONFIG,
  [GEMINI_CONFIG.slug]: GEMINI_CONFIG,
  [GPT_4_1_MINI_CONFIG.slug]: GPT_4_1_MINI_CONFIG,
  [GPT_IMAGE_2_CONFIG.slug]: GPT_IMAGE_2_CONFIG,
  [GLM_API_CONFIG.slug]: GLM_API_CONFIG,
  [GPT_CONFIG.slug]: GPT_CONFIG,
  [KIMI_K3_CONFIG.slug]: KIMI_K3_CONFIG,
  [MINIMAX_H3_CONFIG.slug]: MINIMAX_H3_CONFIG,
  [QWEN_CONFIG.slug]: QWEN_CONFIG,
  [SEEDANCE_25_CONFIG.slug]: SEEDANCE_25_CONFIG,
  [SEEDANCE_CONFIG.slug]: SEEDANCE_CONFIG,
  [SONILO_VIDEO_TO_MUSIC_CONFIG.slug]: SONILO_VIDEO_TO_MUSIC_CONFIG,
};

const GENERIC_MEDIA_PRICE_BY_KIND: Record<ModelGeneratorConfig["kind"], { priceUnit: ModelLandingKey; flatkey: string; official: string }> = {
  image: { priceUnit: "/ image", flatkey: "$0.04", official: "$0.06" },
  video: { priceUnit: "/ second", flatkey: "$0.047", official: "$0.07" },
  audio: { priceUnit: "/ request", flatkey: "$0.0048", official: "$0.009" },
};

const GENERIC_MEDIA_FIELDS: Record<ModelGeneratorConfig["kind"], ModelGeneratorField[]> = {
  image: [
    { name: "n", label: "Images", type: "number", defaultValue: 1, min: 1, max: 10 },
    { name: "size", label: "Size", type: "select", defaultValue: "1024x1024", options: ["1024x1024", "1536x1024", "1024x1536", "auto"] },
    { name: "quality", label: "Quality", type: "select", defaultValue: "high", options: ["auto", "high", "medium", "low"] },
    { name: "output_format", label: "Output format", type: "select", defaultValue: "png", options: ["png", "jpeg", "webp"] },
  ],
  video: [
    { name: "resolution", label: "Resolution", type: "select", defaultValue: "1080p", options: ["720p", "1080p"] },
    { name: "ratio", label: "Aspect ratio", type: "select", defaultValue: "16:9", options: ["16:9", "9:16", "1:1", "4:3", "3:4"] },
    { name: "duration", label: "Duration", type: "number", defaultValue: 5, min: 1, max: 30 },
    { name: "generate_audio", label: "Generate audio", type: "boolean", defaultValue: false },
  ],
  audio: [
    { name: "input_video_url", label: "Video URL", type: "text", defaultValue: "" },
    { name: "duration", label: "Duration", type: "number", defaultValue: 30, min: 5, max: 300 },
    { name: "output_format", label: "Output format", type: "select", defaultValue: "mp3", options: ["mp3", "m4a", "wav"] },
    { name: "variants", label: "Outputs", type: "number", defaultValue: 1, min: 1, max: 10 },
    { name: "preserve_speech", label: "Preserve speech", type: "boolean", defaultValue: true },
  ],
};

// Catalog models do not share one universal media request. These profiles
// mirror the gateway adapters/docs so the public Playground never presents a
// control that the selected route silently ignores.
const GROK_IMAGE_FIELDS: ModelGeneratorField[] = [
  { name: "n", label: "Images", type: "number", defaultValue: 1, min: 1, max: 10 },
  { name: "resolution", label: "Resolution", type: "select", defaultValue: "1k", options: ["1k", "2k"] },
  { name: "quality", label: "Quality", type: "select", defaultValue: "medium", options: ["low", "medium"] },
  { name: "aspect_ratio", label: "Aspect ratio", type: "select", defaultValue: "auto", options: ["auto", "1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20"] },
  { name: "response_format", label: "Output format", type: "select", defaultValue: "url", options: ["url", "b64_json"] },
];

const GEMINI_IMAGE_BASE_FIELDS: ModelGeneratorField[] = [
  { name: "aspect_ratio", label: "Aspect ratio", type: "select", defaultValue: "1:1", options: ["1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"] },
];

const GROK_VIDEO_FIELDS: ModelGeneratorField[] = [
  { name: "duration", label: "Duration", type: "number", defaultValue: 5, min: 1, max: 15 },
];

const VEO_VIDEO_FIELDS: ModelGeneratorField[] = [
  { name: "size", label: "Size", type: "select", defaultValue: "1280x720", options: ["1280x720", "1920x1080", "3840x2160"] },
  { name: "duration", label: "Duration", type: "select", defaultValue: "8", options: ["4", "6", "8"] },
];

function isGeminiImageModel(modelName: string) {
  const normalized = normalizeModelId(modelName);
  return normalized.startsWith("gemini-2-5-flash-image") ||
    normalized.startsWith("gemini-3-pro-image") ||
    normalized.startsWith("gemini-3-1-flash-image") ||
    normalized.startsWith("gemini-3-1-flash-lite-image") ||
    normalized.startsWith("nano-banana-pro-preview");
}

function getGenericMediaProfile(model: PricingModel, kind: ModelGeneratorConfig["kind"]): Pick<ModelGeneratorConfig, "endpoint" | "fields" | "protocol" | "referenceLimits"> {
  const normalized = normalizeModelId(model.model_name);
  if (kind === "video" && normalized.startsWith("grok-imagine-video")) {
    return { endpoint: "/v1/videos", fields: GROK_VIDEO_FIELDS, protocol: "grok-video", referenceLimits: { image: 1 } };
  }
  if (kind === "video" && normalized.startsWith("veo-")) {
    return { endpoint: "/v1/videos", fields: VEO_VIDEO_FIELDS, protocol: "veo-video", referenceLimits: { image: 1 } };
  }
  if (kind === "image" && isGeminiImageModel(model.model_name)) {
    const includeSize = !normalized.startsWith("gemini-2-5-flash-image");
    return {
      endpoint: `/v1beta/models/${model.model_name}:generateContent`,
      fields: includeSize
        ? [...GEMINI_IMAGE_BASE_FIELDS, { name: "image_size", label: "Size", type: "select", defaultValue: "1K", options: ["1K", "2K", "4K"] }]
        : GEMINI_IMAGE_BASE_FIELDS,
      protocol: "gemini-image",
      referenceLimits: { image: 4 },
    };
  }
  if (kind === "image" && normalized.startsWith("grok-imagine-image")) {
    return { endpoint: "/v1/images/generations", fields: GROK_IMAGE_FIELDS, protocol: "openai-image", referenceLimits: { image: 3 } };
  }
  return {
    endpoint: mediaEndpointForModel(kind, model),
    fields: GENERIC_MEDIA_FIELDS[kind],
    protocol: kind === "image" ? "openai-image" : kind === "video" ? "seedance-video" : "audio",
    referenceLimits: kind === "image" ? { image: 4 } : kind === "video" ? { image: 30, video: 10, audio: 10 } : {},
  };
}

export type ModelLandingKey =
  | "All models"
  | "Back to Models"
  | "View Pricing"
  | "View API"
  | "Docs"
  | "Rankings"
  | "Related pages"
  | "Keep exploring Flatkey"
  | "Related models"
  | "More models from {{provider}}"
  | "Swipe or scroll to compare"
  | "Previous example"
  | "Next example"
  | "Example {{index}} of {{total}}"
  | "Generated with {{model}}"
  | "Live catalog model"
  | "Catalog data unavailable"
  | "Providers"
  | "Provider"
  | "API"
  | "Pricing"
  | "Price type"
  | "Price / image"
  | "Previous generation"
  | "Breadcrumb"
  | "Model sections"
  | "Sample usage trend"
  | "Usage trend"
  | "Upload or drag and drop"
  | "Prompt"
  | "Images"
  | "Video URL"
  | "Preserve speech"
  | "Remove reference image"
  | "Uploaded {{count}} files"
  | "Remove {{name}}"
  | "Model price comparison"
  | "After bonus"
  | "Pricing data unavailable"
  | "Performance"
  | "Benchmarks"
  | "Apps"
  | "Activity"
  | "FAQ"
  | "Playground"
  | "Parameters"
  | "Prompt library"
  | "Modalities"
  | "In / Out price"
  | "Context"
  | "Released"
  | "Knowledge cutoff"
  | "Input /M"
  | "Output /M"
  | "Latency"
  | "Uptime"
  | "Live model health"
  | "30-day health, measured on real traffic"
  | "Available catalog entries"
  | "Status"
  | "Throughput"
  | "Successful inference trend"
  | "Not enough data yet"
  | "Avg. provider uptime"
  | "Requests"
  | "Frequently asked questions"
  | "Reliability Index"
  | "Throughput Index"
  | "Latency Index"
  | "Prompt testing and request handoff."
  | "Monthly token share from Flatkey rankings."
  | "Model catalog"
  | "Related models from the pricing catalog."
  | "API Gateway"
  | "Supported endpoint coverage in our pricing API."
  | "Weighted avg input price"
  | "Weighted avg output price"
  | "Cache read"
  | "Cache write"
  | "Request price"
  | "Reference price"
  | "Flatkey routes your request to available upstream channels for this model and keeps billing under one account."
  | "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API."
  | "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available."
  | "Token volume and request traffic for this model over time."
  | "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links."
  | "What is {{model}}?"
  | "How much does {{model}} cost?"
  | "Which providers serve {{model}}?"
  | "Use the pricing section above for current Flatkey prices from our pricing API."
  | "The providers section shows the upstream provider names available in our model catalog."
  | "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price"
  | "▶ Sign in to run"
  | "Start generating"
  | "Generator setup"
  | "Saved before signup"
  | "Public demo"
  | "Size"
  | "Quality"
  | "Outputs"
  | "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup."
  | "(flatkey · official ≈ {{price}})"
  | "{{model}} · OpenAI-compatible · one key, all models"
  | "* Illustrative pricing — see flatkey pricing page"
  | "/ million output tokens"
  | "/ image"
  | "/ second"
  | "/ request"
  | "# Your existing OpenAI code:"
  | "up to 50% off"
  | "covers every model"
  | "Est. this run"
  | "One subscription"
  | "Google / GitHub one-click · no credit card to start"
  | "migrate.py — change one line"
  | "Text, image and video in one plan · overage billed as you go · cancel anytime"
  | "Playground (edit before sign-up)"
  | "Pricing vs official"
  | "Compare"
  | "Compare the current model with the previous generation before you migrate."
  | "Best for"
  | "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready."
  | "See plans →"
  | "Starter / individual"
  | "Team / high-volume"
  | "The same {{model}},"
  | "You pay"
  | "per month on the Go plan"
  | "You get"
  | "of monthly model usage — 4.5× the price"
  | "from $10/month"
  | "Pro — $30/mo, up to $90 usage"
  | "Most popular"
  | "Go — $10/mo, up to $25 usage"
  | "Max — $100/mo, up to $450 usage"
  | "Opus 4 output"
  | "Sonnet 4 output"
  | "Haiku output"
  | "GPT-5 output"
  | "GPT-5 mini output"
  | "GPT-5 input"
  | "GPT-image-2 image"
  | "Square output"
  | "Fast product mockups"
  | "Gemini 2.5 Pro output"
  | "Gemini 2.5 Flash output"
  | "Gemini 2.5 Pro input"
  | "Seedance video / sec"
  | "Image-to-video / sec"
  | "1080p / sec"
  | "MiniMax-H3 768P / sec"
  | "MiniMax-H3 2K / sec"
  | "Reference video input"
  | "Input image after free tier"
  | "same per-second rate"
  | "AIGC watermark"
  | "Cache reads"
  | "Coverage"
  | "AI app backends"
  | "Agent workflows"
  | "Batch content generation"
  | "Best for general AI apps, agents, search, and high-volume API workloads"
  | "Best for long-context reasoning, coding agents, and production assistants"
  | "Best for product videos, ad creative, and image-to-video production"
  | "Best for video soundtracks, speech-preserving edits, and audio production"
  | "Can I control usage before scaling?"
  | "Coding agents"
  | "Product mockups"
  | "Ad creatives"
  | "Ecommerce images"
  | "Best for product images, ad creatives, and ecommerce visual variants"
  | "Does this start a real generation?"
  | "The public page saves your draft settings first. Sign up or open the console to run the request with an API key."
  | "Which MiniMax-H3 fields can I configure here?"
  | "Configure resolution, duration, ratio, and AIGC watermark before opening the console."
  | "Where are my edited prompt settings stored?"
  | "They are stored in this browser's localStorage so the draft survives the signup handoff."
  | "Does this use the same model id in my SDK?"
  | "Live flatkey pricing"
  | "Live model data from pricing API"
  | "Long document analysis"
  | "Matched live models"
  | "Product motion"
  | "Social video variants"
  | "Support automation"
  | "UGC ad clips"
  | "Yes. Keep your SDK and switch base_url plus api_key."
  | "Yes. Plan limits, usage analytics, and one invoice keep spend bounded."
  | "50% off"
  | "Capabilities"
  | "Use one account and API key across text, image, video, and audio models."
  | "Keep prompts, quotas, and model routing in one place."
  | "Reference image"
  | "Reference videos"
  | "Reference Audios"
  | "Video preview"
  | "Image preview"
  | "Audio preview"
  | "Request summary"
  | "Preview only. Sign in to submit this request to Flatkey."
  | "Image to Video"
  | "Reference-guided Video"
  | "Short-form Video"
  | "Text to Image"
  | "Reference-guided Image"
  | "Image Editing"
  | "Text to Audio"
  | "Video to Audio"
  | "Speech synthesis"
  | "Audio generation"
  | "Speech preservation"
  | "Audio variants"
  | "Create music, sound beds, and polished audio tracks from a media-aware brief."
  | "Keep important dialogue and speech intelligible while adding a new audio layer."
  | "Generate multiple delivery options for edits, localization, and production handoff."
  | "Chat and coding"
  | "Long context"
  | "Tool workflows"
  | "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog."
  | "SDK for developers"
  | "Use your existing SDK and set its base URL to the Flatkey router origin."
  | "Flatkey CLI"
  | "Keep prompts and request files in your terminal workflow with the Flatkey CLI."
  | "Codex & Claude Code"
  | "Use the same key with your coding agent and route model jobs from a script."
  | "Price / second"
  | "generate"
  | "30-day window"
  | "last 30 days"
  | "Endpoint"
  | "Aspect ratio"
  | "On"
  | "Off"
  | "Reference media"
  | "None"
  // Seedance 2.5 prototype copy. Keep model names and API identifiers intact
  // in localized values while translating the surrounding product language.
  | "Models"
  | "Video generation"
  | "Image editing"
  | "Reference-guided motion"
  | "Camera and pacing"
  | "Generate product-style shots, merchandising scenes, and reference-guided variations."
  | "Produce campaign concepts, thumbnails, posters, and localized variants."
  | "Refine existing images, swap details, and create consistent variations for each channel."
  | "Add async generation for agents, CMS tools, and batch creative systems."
  | "Create product shots, social clips, and story scenes with controlled motion and framing."
  | "Use image, video, and audio references to keep subjects and creative direction consistent."
  | "Configure ratio, resolution, duration, and sound before handing the request to the API."
  | "seedance-2.5 API"
  | "Seedance-2.5 API"
  | "Quick Start"
  | "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence."
  | "Reliability over the last 30 days"
  | "Measured on real Flatkey traffic, with production monitoring."
  | "Average latency"
  | "Task accepted; generation continues asynchronously"
  | "Last 30 days"
  | "Usage"
  | "Daily seedance-2.5 requests on Flatkey"
  | "Sample shape shown while live telemetry is being connected."
  | "Total Requests"
  | "Daily Average"
  | "Busiest Day"
  | "What seedance-2.5 can do"
  | "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching."
  | "Text and image to video"
  | "Generate from a written scene, or drive it with reference images for a subject you have already designed."
  | "Multi-shot consistency"
  | "Hold characters, wardrobe, and setting across cuts within a single generation."
  | "Native audio"
  | "Ambient sound and speech are generated with the picture, in multiple languages."
  | "Edit and extend"
  | "Continue an existing clip or revise one, with first-frame and first/last-frame control."
  | "Capability"
  | "What changed from the previous generation, so you can tell whether it is worth switching."
  | "Seedance 2.0"
  | "Seedance 2.5"
  | "Clip length"
  | "Short clips, stitched for longer runs"
  | "Up to 30s in a single continuous shot"
  | "Reference inputs"
  | "Image references"
  | "Up to 50 per request — 30 images, 10 videos, 10 audio"
  | "Motion control"
  | "Text prompt only"
  | "Structured motion paths, green-screen and white-model references"
  | "Audio"
  | "Generated audio"
  | "Native audio in 10+ languages"
  | "Editing"
  | "Regenerate to change a clip"
  | "Edit and extend an existing clip in place"
  | "High-Speed Action"
  | "Seedance-2.5 prompts that work"
  | "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there."
  | "Copy Prompt"
  | "Make one like this"
  | "Why Flatkey"
  | "Why run seedance-2.5 through Flatkey"
  | "One key, one balance, and the same upstream model you would call directly."
  | "10% below list price"
  | "The same upstream model, billed from one balance that also covers text, image, and audio models."
  | "OpenAI-compatible from day one"
  | "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code."
  | "Swap models without a new integration"
  | "Move between ByteDance and every other model in the catalog by changing one string."
  | "Routing across upstream channels"
  | "Requests are spread across the channels serving this model, with 30-day uptime published above."
  | "How to call the seedance-2.5 API"
  | "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example."
  | "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language."
  | "Use the OpenAI client and set baseURL to your Flatkey router origin."
  | "Keep prompts and reference files in your terminal workflow with the Flatkey CLI."
  | "Codex&Claude Code"
  | "Use the same key with your coding agent and route video jobs from a script."
  | "Related Models"
  | "Other video generation models"
  | "Seedance 2.5"
  | "Text-to-video · image-to-video"
  | "DeepSeek"
  | "OpenAI-compatible text model"
  | "MiniMax"
  | "Creative video generation"
  | "Veo 3.1"
  | "High-fidelity video generation"
  | "Kling"
  | "Motion control and references"
  | "Sora"
  | "Story and scene generation"
  | "Wan"
  | "Fast creative variants"
  | "Hailuo"
  | "Social-ready clips"
  | "What is seedance-2.5?"
  | "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls."
  | "How much does seedance-2.5 cost?"
  | "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings."
  | "What can I use it for?"
  | "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments."
  | "How do I use the model in my app?"
  | "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration."
  | "Can I control output features?"
  | "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them."
  | "Is the Flatkey API OpenAI compatible?"
  | "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format."
  | "What limits apply?"
  | "Rate limits and available model IDs depend on your account and current upstream availability."
  | "What happens to my prompts and generated files?"
  | "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready."
  | "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow."
  | "API–frequently asked questions"
  | "Seedance-2.5 API–frequently asked questions"
  | "Pricing, compatibility, limits, and how your prompts and generated files are handled.";

export function getModelLandingConfig(slug: string): ModelConfig | null {
  return MODEL_CONFIGS[slug] ?? null;
}

export type ModelLandingMetadataOptions = {
  displayName?: string;
  pathname?: string;
  locale?: Locale;
  /** Prefer the resolved landing kind when the catalog endpoint label is ambiguous. */
  task?: string;
};

export type ModelLandingMetadata = {
  title: string;
  description: string;
  pathname: string;
  locale?: Locale;
};

/** Build short, live-data metadata for any catalog model (including models
 * without a curated landing config). Curated copy remains the fallback when
 * the pricing endpoint has no matching model.
 */
export function buildModelLandingMetadata(
  model: PricingModel,
  options: ModelLandingMetadataOptions = {},
): ModelLandingMetadata {
  const name = options.displayName?.trim() || metadataDisplayName(model.model_name);
  const provider = model.vendor_name?.trim() || model.directory_metadata?.author?.trim() || "AI";
  const task = options.task ?? modelTaskLabel(model);
  const pricing = modelPricingSummary(model, task);
  const context = model.directory_metadata?.context_tokens;
  const locale = options.locale ?? "en";
  const title = buildMetadataTitle(name, task, locale);
  const description = limitSeoDescription(buildMetadataDescription({
    name,
    provider,
    task,
    pricing: pricing[locale] ?? pricing.en,
    context,
    locale,
  }));
  return { title, description, pathname: options.pathname ?? `/models/${encodeURIComponent(model.model_name)}`, ...(options.locale ? { locale: options.locale } : {}) };
}

const PRIORITY_METADATA_DISPLAY_NAMES: Record<string, string> = {
  "gpt-5-6-sol": "GPT-5.6 Sol",
  "gpt-image-2": "GPT Image 2",
  "kimi-k3": "Kimi K3",
  "deepseek-v4-pro": "DeepSeek V4 Pro",
  "minimax-h3": "MiniMax H3",
};

function metadataDisplayName(modelId: string): string {
  return PRIORITY_METADATA_DISPLAY_NAMES[normalizeModelId(modelId)] ?? modelId;
}

function modelTaskLabel(model: PricingModel): string {
  const endpoints = (model.supported_endpoint_types ?? []).map(normalizeModelId);
  const modalities = (model.directory_metadata?.modalities ?? []).map((item) => item.toLowerCase());
  const has = (...needles: string[]) => needles.some((needle) => endpoints.some((endpoint) => endpoint.includes(needle)));
  if (has("video-to-music")) return "video-to-music";
  if (has("image-generation")) return "image generation";
  if (has("openai-video", "video") || modalities.includes("video")) return "video generation";
  if (has("embedding") || /(^|-)embedding(s)?(-|$)/.test(normalizeModelId(model.model_name))) return "embedding";
  if (has("rerank") || /(^|-)rerank(-|$)/.test(normalizeModelId(model.model_name))) return "reranking";
  if (has("audio", "music", "sound", "tts") || modalities.includes("audio")) return "audio";
  if (
    has("openai", "anthropic", "gemini", "response", "chat", "completion", "message", "text") ||
    modalities.includes("text")
  ) return "chat/completions";
  return "AI model";
}

const METADATA_TASK_LABELS: Record<string, Partial<Record<Locale, string>>> = {
  "image generation": { en: "image generation", pt: "geração de imagens", zh: "图像生成", es: "generación de imágenes", fr: "génération d’images", ru: "генерация изображений", ja: "画像生成", vi: "tạo ảnh", de: "Bildgenerierung", id: "generasi gambar" },
  "video generation": { en: "video generation", pt: "geração de vídeo", zh: "视频生成", es: "generación de vídeo", fr: "génération vidéo", ru: "генерация видео", ja: "動画生成", vi: "tạo video", de: "Videogenerierung", id: "generasi video" },
  "video-to-music": { en: "video-to-music", pt: "vídeo para música", zh: "视频转音乐", es: "vídeo a música", fr: "vidéo vers musique", ru: "видео в музыку", ja: "動画から音楽", vi: "video thành nhạc", de: "Video-zu-Musik", id: "video ke musik" },
  embedding: { en: "embedding", pt: "embeddings", zh: "向量嵌入", es: "embeddings", fr: "embeddings", ru: "эмбеддинги", ja: "埋め込み", vi: "embedding", de: "Embeddings", id: "embedding" },
  reranking: { en: "reranking", pt: "reranking", zh: "重排序", es: "reranking", fr: "reclassement", ru: "reranking", ja: "リランキング", vi: "xếp hạng lại", de: "Reranking", id: "reranking" },
  audio: { en: "audio", pt: "áudio", zh: "音频", es: "audio", fr: "audio", ru: "аудио", ja: "音声", vi: "âm thanh", de: "Audio", id: "audio" },
  "chat/completions": { en: "text and chat", pt: "texto e chat", zh: "文本与对话", es: "texto y chat", fr: "texte et chat", ru: "текст и чат", ja: "テキストとチャット", vi: "văn bản và chat", de: "Text und Chat", id: "teks dan chat" },
  "AI model": { en: "AI model", pt: "modelo de IA", zh: "AI 模型", es: "modelo de IA", fr: "modèle d’IA", ru: "модель ИИ", ja: "AIモデル", vi: "mô hình AI", de: "KI-Modell", id: "model AI" },
};

function taskLabel(task: string, locale: Locale): string {
  return METADATA_TASK_LABELS[task]?.[locale] ?? METADATA_TASK_LABELS[task]?.en ?? task;
}

function buildMetadataTitle(name: string, task: string, locale: Locale): string {
  const apiName = name.endsWith(" API") ? name : `${name} API`;
  if (task === "chat/completions") {
    const endpoint = " (chat/completions)";
    const suffix: Record<Locale, string> = { en: ", pricing & FAQs | Flatkey", pt: ", preços e FAQs | Flatkey", zh: "、价格与常见问题 | Flatkey", es: ", precios y FAQ | Flatkey", fr: ", tarifs et FAQ | Flatkey", ru: ", цены и FAQ | Flatkey", ja: "・料金・FAQ | Flatkey", vi: ", giá và FAQ | Flatkey", de: ", Preise & FAQs | Flatkey", id: ", harga & FAQ | Flatkey" };
    return `${apiName}${endpoint}${suffix[locale]}`;
  }
  const taskName = taskLabel(task, locale);
  const suffix: Record<Locale, string> = { en: ", pricing & FAQs | Flatkey", pt: ", preços e FAQs | Flatkey", zh: "、价格与常见问题 | Flatkey", es: ", precios y FAQ | Flatkey", fr: ", tarifs et FAQ | Flatkey", ru: ", цены и FAQ | Flatkey", ja: "・料金・FAQ | Flatkey", vi: ", giá và FAQ | Flatkey", de: ", Preise & FAQs | Flatkey", id: ", harga & FAQ | Flatkey" };
  const prefix: Record<Locale, string> = { en: `${name} ${taskName} API`, pt: `${name} API de ${taskName}`, zh: `${name}${taskName} API`, es: `${name} API de ${taskName}`, fr: `${name} API de ${taskName}`, ru: `${name}: API для ${taskName}`, ja: `${name} ${taskName} API`, vi: `${name} API ${taskName}`, de: `${name} ${taskName}-API`, id: `${name} API ${taskName}` };
  return `${prefix[locale]}${suffix[locale]}`;
}

function buildMetadataDescription(input: { name: string; provider: string; task: string; pricing: string; context: number | null | undefined; locale: Locale }): string {
  const taskName = taskLabel(input.task, input.locale);
  const context = input.context && input.context > 0 ? formatLocalizedContext(input.context, input.locale) : null;
  const facts = (providerLabel: string) => [providerLabel, input.pricing, context].filter(Boolean).join("; ");
  switch (input.locale) {
    case "pt": return `${input.name}: API de ${taskName} pela Flatkey; ${facts(`modelo ${input.provider}`)}.`;
    case "zh": return `${input.name}提供${taskName} API，可通过 Flatkey 使用；${facts(`${input.provider} 模型`)}。`;
    case "es": return `${input.name}: ${taskName} mediante la API de Flatkey; ${facts(`modelo de ${input.provider}`)}.`;
    case "fr": return `${input.name} : ${taskName} via l’API Flatkey ; ${facts(`modèle ${input.provider}`)}.`;
    case "ru": return `${input.name}: ${taskName} через API Flatkey; ${facts(`модель ${input.provider}`)}.`;
    case "ja": return `${input.name}の${taskName}をFlatkey APIで利用。${facts(`${input.provider}モデル`)}。`;
    case "vi": return `${input.name}: ${taskName} qua API Flatkey; ${facts(`mô hình ${input.provider}`)}.`;
    case "de": return `${input.name}: ${taskName} über die Flatkey-API; ${facts(`${input.provider}-Modell`)}.`;
    case "id": return `${input.name}: ${taskName} melalui API Flatkey; ${facts(`model ${input.provider}`)}.`;
    default: return `${input.name} ${taskName} via Flatkey; ${facts(`${input.provider} model`)}.`;
  }
}

function formatLocalizedContext(tokens: number, locale: Locale): string {
  const value = formatContext(tokens);
  switch (locale) {
    case "en": return `${value} context`;
    case "pt": return `contexto de ${value.replace("-token", " tokens")}`;
    case "zh": return `${value.replace("-token", " token")} 上下文`;
    case "es": return `contexto de ${value.replace("-token", " tokens")}`;
    case "fr": return `contexte de ${value.replace("-token", " tokens")}`;
    case "ru": return `контекст ${value.replace("-token", " токенов")}`;
    case "ja": return `${value.replace("-token", "トークン")}のコンテキスト`;
    case "vi": return `ngữ cảnh ${value.replace("-token", " token")}`;
    case "de": return `${value.replace("-token", "-Token")}-Kontext`;
    case "id": return `konteks ${value.replace("-token", " token")}`;
  }
}

function modelPricingSummary(model: PricingModel, task: string): { en: string; pt: string } & Partial<Record<Locale, string>> {
  const kind = model.display_pricing?.billing_kind;
  const prices = model.display_pricing?.prices ?? {};
  const valueFor = (dimension: keyof typeof prices): number | undefined => {
    const pair = prices[dimension];
    return pair?.plg ?? pair?.configured;
  };
  const valueText = (value: number | undefined): string | null => value == null || !Number.isFinite(value) ? null : formatUsdPrice(value);
  // Image-generation and audio-generation routes can be token-billed with a
  // dedicated media dimension. Prefer that dimension when it is present so
  // metadata describes the actual unit instead of incorrectly saying only
  // "token pricing".
  if (task === "image generation") {
    const value = valueText(valueFor("image"));
    if (value) return mediaPriceCopy(value, "image");
  }
  if (task === "audio") {
    const value = valueText(valueFor("audio_output")) ?? valueText(valueFor("audio_input"));
    if (value) return mediaPriceCopy(value, "audio");
  }
  if (kind === "per_second") {
    const value = valueText(valueFor("second"));
    return value ? secondPriceCopy(value) : genericPriceCopy("per-second pricing", "preço por segundo");
  }
  if (kind === "request") {
    const value = valueText(valueFor("request"));
    return value ? requestPriceCopy(value) : genericPriceCopy("per-request pricing", "preço por solicitação");
  }
  if (kind === "token" || kind === "tiered_expr" || model.quota_type === 0) {
    const input = valueText(valueFor("input"));
    const output = valueText(valueFor("output"));
    if (input && output) return tokenPriceCopy(`${input} input/${output} output per 1M tokens`, `${input} entrada/${output} saída por 1M tokens`, input, output);
    if (input) return tokenPriceCopy(`${input} input per 1M tokens`, `${input} entrada por 1M tokens`, input);
    return kind === "tiered_expr" ? genericPriceCopy("time-tiered token pricing", "preço de tokens por faixa de horário") : genericPriceCopy("token pricing", "preço por tokens");
  }
  const request = valueText(valueFor("request")) ?? valueText(model.model_price);
  return request ? requestPriceCopy(request) : genericPriceCopy("request pricing", "preço por solicitação");
}

function genericPriceCopy(en: string, pt: string): { en: string; pt: string } & Partial<Record<Locale, string>> {
  return { en, pt, zh: en === "token pricing" ? "按 token 计价" : "按请求计价", es: en.replace("pricing", "precios"), fr: en.replace("pricing", "tarifs"), ru: "цена зависит от тарификации", ja: "料金は課金単位によります", vi: "giá theo đơn vị tính", de: "Preis nach Abrechnungseinheit", id: "harga berdasarkan unit penagihan" };
}

function mediaPriceCopy(value: string, unit: "image" | "audio"): { en: string; pt: string } & Partial<Record<Locale, string>> {
  const units = { image: { pt: "imagem", zh: "图像", es: "imagen", fr: "image", ru: "изображение", ja: "画像", vi: "ảnh", de: "Bild", id: "gambar" }, audio: { pt: "áudio", zh: "音频", es: "audio", fr: "audio", ru: "аудио", ja: "音声", vi: "âm thanh", de: "Audio", id: "audio" } }[unit];
  return { en: `from ${value}/${unit} pricing`, pt: `preço a partir de ${value}/${units.pt}`, zh: `价格从 ${value}/${units.zh}起`, es: `precios desde ${value}/${units.es}`, fr: `tarifs à partir de ${value}/${units.fr}`, ru: `цена от ${value} за ${units.ru}`, ja: `${value}/${units.ja}から`, vi: `giá từ ${value}/${units.vi}`, de: `ab ${value}/${units.de}`, id: `mulai ${value}/${units.id}` };
}

function secondPriceCopy(value: string): { en: string; pt: string } & Partial<Record<Locale, string>> {
  return { en: `from ${value}/second pricing`, pt: `preço a partir de ${value}/segundo`, zh: `价格从 ${value}/秒起`, es: `precios desde ${value}/segundo`, fr: `tarifs à partir de ${value}/seconde`, ru: `цена от ${value} за секунду`, ja: `${value}/秒から`, vi: `giá từ ${value}/giây`, de: `ab ${value}/Sekunde`, id: `mulai ${value}/detik` };
}

function requestPriceCopy(value: string): { en: string; pt: string } & Partial<Record<Locale, string>> {
  return { en: `from ${value}/request pricing`, pt: `preço a partir de ${value}/solicitação`, zh: `价格从 ${value}/请求起`, es: `precios desde ${value}/solicitud`, fr: `tarifs à partir de ${value}/requête`, ru: `цена от ${value} за запрос`, ja: `${value}/リクエストから`, vi: `giá từ ${value}/yêu cầu`, de: `ab ${value}/Anfrage`, id: `mulai ${value}/permintaan` };
}

function tokenPriceCopy(en: string, pt: string, input: string, output?: string): { en: string; pt: string } & Partial<Record<Locale, string>> {
  const zh = output ? `${input} 输入/${output} 输出，每 100 万 token` : `${input} 输入，每 100 万 token`;
  const es = output ? `${input} entrada/${output} salida por 1M tokens` : `${input} entrada por 1M tokens`;
  const fr = output ? `${input} entrée/${output} sortie par 1M de tokens` : `${input} entrée par 1M de tokens`;
  const ru = output ? `${input} ввод/${output} вывод за 1 млн токенов` : `${input} ввод за 1 млн токенов`;
  const ja = output ? `入力${input}/出力${output}（100万トークンあたり）` : `入力${input}（100万トークンあたり）`;
  const vi = output ? `đầu vào ${input}/đầu ra ${output} mỗi 1M token` : `đầu vào ${input} mỗi 1M token`;
  const de = output ? `${input} Eingabe/${output} Ausgabe je 1M Token` : `${input} Eingabe je 1M Token`;
  const id = output ? `input ${input}/output ${output} per 1M token` : `input ${input} per 1M token`;
  return { en, pt, zh, es, fr, ru, ja, vi, de, id };
}

function formatContext(tokens: number): string {
  return tokens >= 1_000_000 && tokens % 1_000_000 === 0 ? `${tokens / 1_000_000}M-token` : `${tokens.toLocaleString("en-US")}-token`;
}

export function limitSeoDescription(value: string): string {
  const normalized = value.replace(/\s+/g, " ").trim();
  if (normalized.length <= 160) return normalized;
  const clipped = normalized.slice(0, 159).replace(/\s+\S*$/, "").trim();
  return `${clipped || normalized.slice(0, 159).trim()}…`;
}

export function getModelLandingConfigForModel(modelId: string): ModelConfig | null {
  const normalized = normalizeModelId(modelId);
  const config = getModelLandingConfigs().find((candidate) =>
    candidate.modelIds.some((configuredId) => matchesModelId(normalized, configuredId))
  ) ?? null;
  const priorityOverride = PRIORITY_MODEL_OVERRIDES[normalized];
  if (!config || !priorityOverride) return config;
  return {
    ...config,
    ...priorityOverride,
    slug: PRIORITY_CANONICAL_SLUGS[normalized] ?? config.slug,
    modelIds: [modelId, ...config.modelIds],
    displayName: modelId,
    modelId,
    ...(priorityOverride.seo
      ? { seo: { ...priorityOverride.seo, description: limitSeoDescription(priorityOverride.seo.description) } }
      : {}),
    landingContent: priorityOverride.landingContent
      ? { ...config.landingContent, ...priorityOverride.landingContent }
      : config.landingContent,
  };
}

export function getModelLandingConfigForPricingModel(model: PricingModel): ModelConfig {
  const explicitConfig = getModelLandingConfigForModel(model.model_name);
  if (explicitConfig) {
    // A family config can match a more specific media model by prefix (for
    // example `gemini-2.5-flash-image` matches the Gemini chat family). Let the
    // model name/endpoint classification win when it disagrees with that
    // broad family config, otherwise the image page loses its generator and
    // prompt-library assets entirely.
    const inferredKind = inferMediaKind(model);
    if (inferredKind && explicitConfig.generator?.kind !== inferredKind) {
      return buildGenericMediaLandingConfig(model) ?? buildGenericTextLandingConfig(model);
    }
    return modelLandingConfigForModel(explicitConfig, model);
  }
  return buildGenericMediaLandingConfig(model) ?? buildGenericTextLandingConfig(model);
}

export function modelLandingConfigForModel(config: ModelConfig, model: PricingModel): ModelConfig {
  const normalizedModelId = normalizeModelId(model.model_name);
  const priorityOverride = PRIORITY_MODEL_OVERRIDES[normalizedModelId];
  const editorialContent = priorityOverride?.landingContent
    ? { ...config.landingContent, ...priorityOverride.landingContent }
    : config.landingContent ?? buildGenericLandingContent(model, config.generator?.kind ?? inferMediaKind(model) ?? "text");
  const metadataTask = config.generator?.kind ? metadataTaskForLandingKind(config.generator.kind) : undefined;
  const dynamicMetadata = priorityOverride?.seo
    ? null
    : buildModelLandingMetadata(model, { locale: "en", task: metadataTask });
  return {
    ...config,
    ...priorityOverride,
    slug: PRIORITY_CANONICAL_SLUGS[normalizedModelId] ?? encodeURIComponent(model.model_name),
    modelIds: [model.model_name, ...config.modelIds],
    displayName: model.model_name,
    modelId: model.model_name,
    officialName: model.vendor_name ?? config.officialName,
    ...(priorityOverride?.seo
      ? { seo: { ...priorityOverride.seo, description: limitSeoDescription(priorityOverride.seo.description) } }
      : {}),
    ...(dynamicMetadata
      ? {
          seo: { title: dynamicMetadata.title, description: dynamicMetadata.description },
          seoByLocale: buildModelSeoByLocale(model, metadataTask),
        }
      : {}),
    landingContent: editorialContent,
  };
}

/**
 * Apply a complete, model-specific editorial pack for a non-English page.
 *
 * The shared shell still owns the layout and the English override remains the
 * source of truth for the default route.  For translated routes we replace
 * the editorial fields as one coherent object instead of translating each
 * English string independently: priority packs contain different section
 * counts and ordering, so positional string pairing can silently attach a
 * Kimi explanation to a pricing heading or a MiniMax comparison title.
 */
export function getLocalizedModelLandingConfig(config: ModelConfig, locale: Locale): ModelConfig {
  if (locale === "en") return config;
  const localized = getPriorityModelCopy(config.modelId || config.slug, locale);
  if (!localized) {
    // Hand-audited packs (notably Seedance 2.5) already carry their own
    // localized content map. Only the generated catalog fallback should be
    // rebuilt here; preserving object identity keeps curated packs immutable.
    const isGeneratedGeneric = config.landingContent?.faq?.some(
      (item) => item.question === `What is ${config.displayName} used for?`,
    );
    if (!isGeneratedGeneric) return config;
    const localizedSeo = config.seoByLocale?.[locale];
    return {
      ...config,
      ...(localizedSeo ? { seo: localizedSeo } : {}),
      landingContent: localizeGenericLandingContent(
        config.landingContent!,
        locale,
        config.displayName,
        config.generator?.kind,
      ),
    };
  }

  const sourceContent = config.landingContent;
  const localizedContent = localized.landingContent;
  return {
    ...config,
    landingContent: {
      ...sourceContent,
      ...localizedContent,
      // Preserve non-editorial source fields (logo, breadcrumb, and action
      // label) while replacing the localized title/description.
      hero: sourceContent?.hero || localizedContent.hero
        ? { ...sourceContent?.hero, ...localizedContent.hero }
        : undefined,
    },
  };
}

export function getModelLandingConfigs(): ModelConfig[] {
  return Object.values(MODEL_CONFIGS);
}

export function getModelLandingPathnames(): string[] {
  return getModelLandingConfigs().map((config) => `/models/${config.slug}`);
}

/**
 * Canonical paths for priority models whose live catalog entry is resolved at
 * request time rather than represented by a static family config (for example
 * GPT-5.6 Sol and DeepSeek V4 Pro).  Keep these separate from the static list
 * so generateStaticParams does not accidentally force a pricing fetch for
 * every locale during a build.
 */
export function getPriorityModelLandingPathnames(): string[] {
  return [...new Set(Object.values(PRIORITY_CANONICAL_SLUGS).map((slug) => `/models/${slug}`))];
}

export function resolveModelLandingModels(config: ModelConfig, models: PricingModel[]): PricingModel[] {
  return models.filter((model) => {
    const normalized = normalizeModelId(model.model_name);
    return config.modelIds.some((configuredId) => matchesModelId(normalized, configuredId));
  });
}

export function normalizeModelId(modelId: string): string {
  return modelId.trim().toLowerCase().replace(/[_.\s]+/g, "-");
}

function matchesModelId(normalizedModelId: string, configuredId: string): boolean {
  const normalizedConfiguredId = normalizeModelId(configuredId);
  return (
    normalizedModelId === normalizedConfiguredId ||
    normalizedModelId.startsWith(`${normalizedConfiguredId}-`)
  );
}

function buildGenericMediaLandingConfig(model: PricingModel): ModelConfig | null {
  const kind = inferMediaKind(model);
  if (!kind) return null;
  const price = GENERIC_MEDIA_PRICE_BY_KIND[kind];
  const displayName = model.model_name;
  const officialName = model.vendor_name ?? "Provider";
  const liveSecond = model.display_pricing?.prices.second?.plg ?? model.display_pricing?.prices.second?.configured;
  const liveOfficialSecond = model.display_pricing?.prices.second?.configured ?? liveSecond;
  const liveFlatkeyPrice = liveSecond != null ? formatPriceLiteral(liveSecond) : price.flatkey;
  const liveOfficialPrice = liveOfficialSecond != null ? formatPriceLiteral(liveOfficialSecond) : price.official;
  const landingContent = buildGenericLandingContent(model, kind);
  const mediaProfile = getGenericMediaProfile(model, kind);
  const metadata = buildModelLandingMetadata(model, { locale: "en", task: metadataTaskForLandingKind(kind) });
  return {
    slug: encodeURIComponent(model.model_name),
    modelIds: [model.model_name],
    displayName,
    modelId: model.model_name,
    generator: {
      kind,
      endpoint: mediaProfile.endpoint,
      storageKey: `flatkey:model-generator-draft:${normalizeModelId(model.model_name)}`,
      fields: mediaProfile.fields,
      protocol: mediaProfile.protocol,
      referenceLimits: mediaProfile.referenceLimits,
    },
    officialName,
    officialPrice: liveOfficialPrice,
    flatkeyPrice: liveFlatkeyPrice,
    estFlatkey: liveFlatkeyPrice,
    estOfficial: liveOfficialPrice,
    examplePrompt: examplePromptForMediaKind(kind, displayName),
    priceUnit: price.priceUnit,
    rows: [
      { label: "Live flatkey pricing", flatkey: liveFlatkeyPrice, official: liveOfficialPrice },
      { label: "Live model data from pricing API", flatkey: "", value: officialName },
      { label: "Coverage", flatkey: "", value: "Text · image · video · audio" },
    ],
    seo: { title: metadata.title, description: metadata.description },
    seoByLocale: buildModelSeoByLocale(model, metadataTaskForLandingKind(kind)),
    positioning: kind === "image"
      ? "Best for product images, ad creatives, and ecommerce visual variants"
      : kind === "audio"
        ? "Best for video soundtracks, speech-preserving edits, and audio production"
        : "Best for product videos, ad creative, and image-to-video production",
    useCases: kind === "image"
      ? ["Product mockups", "Ad creatives", "Ecommerce images"]
      : kind === "audio"
        ? ["Audio generation", "Speech preservation", "Audio variants"]
        : ["UGC ad clips", "Product motion", "Social video variants"],
    faq: [
      {
        question: "Does this start a real generation?",
        answer: "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.",
      },
      {
        question: "Where are my edited prompt settings stored?",
        answer: "They are stored in this browser's localStorage so the draft survives the signup handoff.",
      },
    ],
    landingContent,
  };
}

function buildGenericTextLandingConfig(model: PricingModel): ModelConfig {
  const displayName = model.model_name;
  const officialName = model.vendor_name ?? "Provider";
  const tokenBased = model.quota_type === 0;
  const officialUnitPrice = tokenBased
    ? Number(model.model_ratio ?? 0) * 2 * Number(model.completion_ratio ?? 1)
    : Number(model.model_price ?? 0);
  const flatkeyUnitPrice = officialUnitPrice * 0.67;
  const officialPrice = officialUnitPrice > 0 ? formatPriceLiteral(officialUnitPrice) : "$0";
  const flatkeyPrice = flatkeyUnitPrice > 0 ? formatPriceLiteral(flatkeyUnitPrice) : "$0";
  const landingContent = buildGenericLandingContent(model, "text");
  const metadata = buildModelLandingMetadata(model, { locale: "en" });

  return {
    slug: encodeURIComponent(model.model_name),
    modelIds: [model.model_name],
    displayName,
    modelId: model.model_name,
    officialName,
    officialPrice,
    flatkeyPrice,
    estFlatkey: flatkeyPrice,
    estOfficial: officialPrice,
    examplePrompt:
      "You are a senior backend engineer. In 3 sentences, explain why developers should use an LLM gateway instead of calling each official API directly.",
    priceUnit: tokenBased ? "/ million output tokens" : "/ request",
    rows: [
      { label: "Live flatkey pricing", flatkey: flatkeyPrice, official: officialPrice },
      { label: "Live model data from pricing API", flatkey: "", value: officialName },
      { label: "Coverage", flatkey: "", value: COVERAGE },
    ],
    seo: { title: metadata.title, description: metadata.description },
    seoByLocale: buildModelSeoByLocale(model),
    positioning: "Best for general AI apps, agents, search, and high-volume API workloads",
    useCases: ["AI app backends", "Agent workflows", "Batch content generation"],
    faq: [
      {
        question: "Does this use the same model id in my SDK?",
        answer: "Yes. Keep your SDK and switch base_url plus api_key.",
      },
      {
        question: "Can I control usage before scaling?",
        answer: "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.",
      },
    ],
    landingContent,
  };
}

/**
 * Build a distinct editorial pack for catalog models that do not have a
 * hand-written priority brief. The page shell stays shared, but every H2,
 * pricing explanation, capability card and FAQ is anchored to the actual
 * model id, provider, endpoint, modalities and catalog billing dimensions.
 * Unknown upstream facts are deliberately described as catalog facts rather
 * than guessed as official product claims.
 */
function buildGenericLandingContent(
  model: PricingModel,
  kind: ModelGeneratorConfig["kind"] | "text",
): ModelLandingContent {
  const name = model.model_name;
  const provider = model.vendor_name?.trim() || model.directory_metadata?.author?.trim() || "Provider";
  const endpoint = kind === "text" ? textEndpointForModel(model) : mediaEndpointForModel(kind, model);
  const task = modelTaskLabel(model);
  const taskPhrase = kind === "image"
    ? "AI image generator and API"
    : kind === "video"
      ? "AI video generator and API"
      : kind === "audio"
        ? "audio generation API"
        : "AI model API";
  const keywordPhrase = kind === "image"
    ? "image generation"
    : kind === "video"
      ? "video generation"
      : kind === "audio"
        ? "audio generation"
        : "chat and coding";
  const modalities = model.directory_metadata?.modalities?.join(" · ") || "Not listed in the catalog";
  const context = model.directory_metadata?.context_tokens && model.directory_metadata.context_tokens > 0
    ? formatContext(model.directory_metadata.context_tokens)
    : "Context window not listed in the catalog";
  const pricing = modelPricingSummary(model, task).en;
  const categories = model.directory_metadata?.categories?.filter(Boolean).slice(0, 3).join(", ");
  const categoryLine = categories ? `Catalog categories: ${categories}.` : "The catalog does not list a category for this model.";
  const endpointLine = `Flatkey routes this model through ${endpoint}.`;

  const capabilities = kind === "image"
    ? [
        { title: `${name} visual creation`, body: `Turn a written brief into product, editorial, or campaign imagery through ${endpoint}.` },
        { title: `${name} reference-led editing`, body: `Use an existing image as creative direction for revisions and consistent visual variants when the route accepts references.` },
        { title: `${name} channel-ready variants`, body: `Adapt one concept into the image compositions your storefront, campaign, or content pipeline needs.` },
        { title: `${name} creative production workflow`, body: `Move an approved visual from prompt exploration into a repeatable Flatkey generation workflow.` },
      ]
    : kind === "video"
      ? [
          { title: `${name} scene creation`, body: `Turn a written brief into short-form scenes through ${endpoint}, ready for creative iteration.` },
          { title: `${name} reference-led motion`, body: `Animate a designed frame, product, character, or storyboard when the route accepts reference media.` },
          { title: `${name} camera and action direction`, body: `Shape how the camera, subject, pacing, and framing develop into a coherent clip.` },
          { title: `${name} production handoff`, body: `Move a tested concept from prompt exploration into an authenticated Flatkey video workflow.` },
        ]
      : kind === "audio"
        ? [
            { title: `${name} soundtrack and sound design`, body: `Create music, ambience, speech layers, or other audio outputs through ${endpoint}.` },
            { title: `${name} media-aware audio work`, body: `Use the listed modalities (${modalities}) to plan soundtrack, speech, or video-to-audio workflows.` },
            { title: `${name} creator and localization use`, body: `Connect the model to creator tools, dubbing, localization, or batch-production pipelines.` },
            { title: `${name} repeatable delivery`, body: `Move a tested audio brief into an authenticated Flatkey workflow with predictable account controls.` },
          ]
        : [
            { title: `${name} chat and coding API`, body: `Call ${name} through ${endpoint} for chat, coding or agent workflows supported by the model's listed modalities.` },
            { title: `${name} long-context work`, body: `${name} is listed with ${context}; use that catalog value as an integration planning reference and validate limits for your account.` },
            { title: `${name} tool and agent workflows`, body: `Keep tool calls, structured output and streaming behavior aligned with the endpoint contract instead of assuming every provider feature is interchangeable.` },
            { title: `${name} production routing`, body: `Use one Flatkey key for ${name}, usage controls and model routing while retaining the exact model id in your SDK configuration.` },
          ];

  const useCases = kind === "image"
    ? ["Product and ecommerce images", "Advertising creative variants", "CMS and batch image jobs"]
    : kind === "video"
      ? ["Short-form creator clips", "Product and UGC video variants", "Storyboard and previsualization tests"]
      : kind === "audio"
        ? ["Soundtrack and audio beds", "Speech or localization workflows", "Batch audio production"]
        : ["AI application backends", "Coding and agent workflows", "Document and research automation"];

  const content: ModelLandingContent = {
    hero: {
      title: `${name} ${taskPhrase}`,
      description: `${provider} ${name} for ${keywordPhrase}; ${endpointLine} ${pricing}${context.startsWith("Context window") ? "" : ` ${context} context window.`}`,
      provider,
    },
    performance: {
      eyebrow: `${name} API performance`,
      title: `${name} API reliability and uptime`,
      description: `Live Flatkey request telemetry for ${name} appears here when enough production traffic is available.`,
    },
    activity: {
      eyebrow: `${name} API activity`,
      title: `${name} API usage and request activity`,
      description: `Track request volume and successful inference activity for ${name} over the latest reporting window.`,
    },
    pricing: {
      eyebrow: `${name} pricing`,
      title: `${name} API pricing and billing`,
      description: `${pricing}. Rates below come from the live Flatkey catalog; they are not a promise about a provider's direct public price.`,
      note: `Billing follows the selected ${name} route and account group. Recheck the live estimate before running production volume.`,
      rows: buildGenericPricingRows(model, kind),
    },
    capabilities,
    capabilitiesTitle: `${name} ${keywordPhrase}: core capabilities`,
    capabilitiesDescription: `${provider}'s ${name} route supports ${modalities}. ${categoryLine}`,
    comparison: {
      eyebrow: "Compare",
      title: `${name} API comparison: capabilities and access`,
      description: `Compare the catalog facts for ${name} with the previous-generation baseline before changing your integration.`,
      baselineLabel: "Previous generation",
      currentLabel: name,
      rows: [
        { label: "Provider", baseline: "Not verified in this catalog snapshot", current: provider },
        { label: "Modalities", baseline: "Not verified in this catalog snapshot", current: modalities },
        { label: "Context", baseline: "Not verified in this catalog snapshot", current: context },
        { label: "Endpoint", baseline: "Not verified in this catalog snapshot", current: endpoint },
        { label: "Billing", baseline: "Not verified in this catalog snapshot", current: pricing },
      ],
    },
    promptLibraryTitle: `${name} ${keywordPhrase} prompt examples`,
    promptLibraryDescription: kind === "audio"
      ? `Review ${name} API examples and billing dimensions, then use the documented endpoint for authenticated requests.`
      : `Start with a ${name} prompt for ${useCases[0].toLowerCase()}, then adjust the request fields shown in the playground.`,
    why: {
      eyebrow: `Why ${name} API`,
      title: `Why use ${name} through Flatkey?`,
      description: `Use one gateway for ${name}, account controls and the rest of your model catalog.`,
      cards: [
        { title: `${name} in one API`, body: `Keep the ${name} model id and endpoint explicit while using the same Flatkey key as other workloads.` },
        { title: "Live catalog pricing", body: `See the current ${name} billing dimensions before a request is sent, then confirm the final estimate in your account.` },
        {
          title: kind === "audio" ? "API integration" : "A practical model handoff",
          body: kind === "audio"
            ? `Use ${endpoint} with a Flatkey API key in your server or agent.`
            : `Test a ${name} prompt in the public playground and carry the same settings into an authenticated integration.`,
        },
        { title: "Usage and routing controls", body: `Centralize keys, quotas and routing for ${name} without changing your application’s provider-facing workflow.` },
      ],
    },
    api: {
      eyebrow: `${name} API`,
      title: `${name} API integration guide`,
      description: `Use the model id above with ${endpoint}; keep the request fields and billing unit documented for your route.`,
      items: [
        { title: `Call ${endpoint}`, detail: `Send an authenticated request to ${endpoint} with model set to ${name}.` },
        { title: "Keep the model id stable", detail: `Use ${name} in your SDK configuration so routing and usage reports map to the intended catalog entry.` },
        { title: "Check the live estimate", detail: `Review ${pricing} and account limits before scaling ${name} requests.` },
        {
          title: kind === "audio" ? "Integrate with the API" : "Move from test to production",
          detail: kind === "audio"
            ? `Use ${endpoint} with a Flatkey API key in your server or agent.`
            : `Start in the playground, then reuse the request shape with a Flatkey API key in your server or agent.`,
        },
      ],
    },
    related: {
      eyebrow: "Related model APIs",
      title: `Related ${keywordPhrase} models`,
      description: `Explore other ${keywordPhrase} routes in the Flatkey catalog.`,
      cards: [],
    },
    faqTitle: { beforeBreak: `${name} API`, afterBreak: "pricing, features, and usage FAQs" },
    faqDescription: `Answers about ${name} pricing, capabilities, endpoint access and catalog limits.`,
    faq: [
      { question: `What is ${name} used for?`, answer: `${name} is listed by ${provider} for ${keywordPhrase}; the catalog lists these modalities: ${modalities}.` },
      { question: `How is ${name} priced?`, answer: `${name} currently shows ${pricing}. The applicable rate can vary by route, account group and request settings.` },
      { question: `Which API endpoint calls ${name}?`, answer: `${endpointLine} Set the model field to ${name} and follow the fields supported by that endpoint.` },
      { question: `What context or input limits does ${name} have?`, answer: `${name} is listed with ${context}. Other limits depend on the route and current account availability, so verify them before production use.` },
    ],
  };
  return content;
}

function textEndpointForModel(model: PricingModel): string {
  const endpoint = (model.supported_endpoint_types ?? []).map(normalizeModelId).find((item) =>
    /chat|completion|response|message|anthropic|gemini|openai/.test(item),
  );
  return endpoint?.includes("anthropic") ? "/v1/messages" : "/v1/chat/completions";
}

function buildGenericPricingRows(
  model: PricingModel,
  kind: ModelGeneratorConfig["kind"] | "text",
): Array<{ label: string; value: string; detail?: string }> {
  const pricing = model.display_pricing;
  const rows: Array<{ label: string; value: string; detail?: string }> = [];
  const add = (label: string, dimension: keyof NonNullable<PricingModel["display_pricing"]>["prices"], detail: string) => {
    const pair = pricing?.prices?.[dimension];
    const value = pair?.plg ?? pair?.configured;
    if (value != null && Number.isFinite(value)) rows.push({ label, value: `${formatUsdPrice(value)} / ${kind === "text" ? "1M tokens" : dimension === "second" ? "second" : dimension}`, detail });
  };
  if (pricing?.billing_kind === "token" || pricing?.billing_kind === "tiered_expr" || model.quota_type === 0) {
    add("Input tokens", "input", "Catalog input rate");
    add("Output tokens", "output", "Catalog output rate");
    add("Cached input", "cache", "Catalog cache rate");
  } else if (pricing?.billing_kind === "per_second") {
    add("Generation seconds", "second", "Rate follows generated seconds");
  } else {
    add(kind === "image" ? "Generated image" : kind === "audio" ? "Audio request" : kind === "video" ? "Generated video" : "API request", "request", "Catalog request rate");
  }
  if (rows.length === 0) rows.push({ label: "Live catalog pricing", value: "See current estimate", detail: "The catalog did not expose a numeric rate in this snapshot" });
  return rows;
}

function buildModelSeoByLocale(model: PricingModel, task?: string): Partial<Record<Locale, { title: string; description: string }>> {
  return Object.fromEntries(LOCALES.map((locale) => {
    const metadata = buildModelLandingMetadata(model, { locale, task });
    return [locale, { title: metadata.title, description: metadata.description }];
  })) as Partial<Record<Locale, { title: string; description: string }>>;
}

function metadataTaskForLandingKind(kind: ModelGeneratorConfig["kind"]): string {
  return kind === "image" ? "image generation" : kind === "video" ? "video generation" : "audio";
}

type GenericLocaleCopy = {
  task: { image: string; video: string; audio: string; text: string };
  performance: string;
  performanceTitle: string;
  activity: string;
  activityTitle: string;
  pricing: string;
  pricingTitle: string;
  capabilitiesTitle: string;
  comparisonTitle: string;
  whyTitle: string;
  apiTitle: string;
  relatedTitle: string;
  faqAfter: string;
  faqDescription: string;
  faqQuestions: string[];
  faqAnswers: string[];
};

const GENERIC_LOCALE_COPY: Record<Locale, GenericLocaleCopy> = {
  en: {
    task: { image: "AI image generator and API", video: "AI video generator and API", audio: "audio generation API", text: "AI model API" },
    performance: "API performance", performanceTitle: "API reliability and uptime", activity: "API activity", activityTitle: "API usage and request activity", pricing: "pricing", pricingTitle: "API pricing and billing", capabilitiesTitle: "core capabilities", comparisonTitle: "API comparison: capabilities and access", whyTitle: "Why use this model through Flatkey?", apiTitle: "API integration guide", relatedTitle: "Related model APIs", faqAfter: "pricing, features, and usage FAQs", faqDescription: "Answers about pricing, capabilities, endpoint access and catalog limits.",
    faqQuestions: ["What is this model used for?", "How is this model priced?", "Which API endpoint calls this model?", "What context or input limits apply?"],
    faqAnswers: ["The model is listed in the Flatkey catalog for the modalities shown on this page.", "The current rate is shown in the live Flatkey pricing table and can vary by route and account group.", "Use the endpoint and model id shown in the API section for this catalog entry.", "Context and input limits are catalog or route facts; verify them before production use."],
  },
  zh: {
    task: { image: "AI 图像生成器与 API", video: "AI 视频生成器与 API", audio: "音频生成 API", text: "AI 模型 API" },
    performance: "API 性能", performanceTitle: "API 可靠性与可用性", activity: "API 活动", activityTitle: "API 使用量与请求活动", pricing: "价格", pricingTitle: "API 价格与计费", capabilitiesTitle: "核心能力", comparisonTitle: "API 对比：能力与访问方式", whyTitle: "为什么通过 Flatkey 使用此模型？", apiTitle: "API 集成指南", relatedTitle: "相关模型 API", faqAfter: "价格、功能与使用常见问题", faqDescription: "关于价格、能力、端点访问和目录限制的答案。",
    faqQuestions: ["此模型用于什么任务？", "此模型如何计费？", "调用此模型使用哪个 API 端点？", "此模型有哪些上下文或输入限制？"],
    faqAnswers: ["该模型已列入 Flatkey 目录，支持的模态见本页说明。", "当前费率显示在 Flatkey 实时价格表中，可能因路由和账户组而变化。", "请使用 API 区域显示的端点和模型 ID 调用此目录条目。", "上下文和输入限制以目录或路由事实为准，生产使用前请再次确认。"],
  },
  pt: {
    task: { image: "gerador de imagens por IA e API", video: "gerador de vídeo por IA e API", audio: "API de geração de áudio", text: "API de modelo de IA" },
    performance: "Desempenho da API", performanceTitle: "Confiabilidade e disponibilidade da API", activity: "Atividade da API", activityTitle: "Uso e atividade de solicitações da API", pricing: "preços", pricingTitle: "Preços e cobrança da API", capabilitiesTitle: "principais capacidades", comparisonTitle: "Comparação de API: capacidades e acesso", whyTitle: "Por que usar este modelo pela Flatkey?", apiTitle: "Guia de integração da API", relatedTitle: "APIs de modelos relacionados", faqAfter: "perguntas sobre preço, recursos e uso", faqDescription: "Respostas sobre preço, capacidades, endpoints e limites do catálogo.",
    faqQuestions: ["Para que este modelo é usado?", "Como este modelo é cobrado?", "Qual endpoint da API chama este modelo?", "Quais limites de contexto ou entrada se aplicam?"],
    faqAnswers: ["O modelo está no catálogo Flatkey com as modalidades indicadas nesta página.", "A tarifa atual aparece na tabela de preços ao vivo e pode variar por rota e grupo da conta.", "Use o endpoint e o ID do modelo mostrados na seção API para esta entrada do catálogo.", "Os limites de contexto e entrada são fatos do catálogo ou da rota; confirme antes de usar em produção."],
  },
  es: {
    task: { image: "generador de imágenes IA y API", video: "generador de vídeo IA y API", audio: "API de generación de audio", text: "API de modelo de IA" },
    performance: "Rendimiento de la API", performanceTitle: "Fiabilidad y disponibilidad de la API", activity: "Actividad de la API", activityTitle: "Uso y actividad de solicitudes de la API", pricing: "precios", pricingTitle: "Precios y facturación de la API", capabilitiesTitle: "capacidades principales", comparisonTitle: "Comparación de API: capacidades y acceso", whyTitle: "¿Por qué usar este modelo mediante Flatkey?", apiTitle: "Guía de integración de la API", relatedTitle: "APIs de modelos relacionados", faqAfter: "preguntas sobre precios, funciones y uso", faqDescription: "Respuestas sobre precios, capacidades, endpoints y límites del catálogo.",
    faqQuestions: ["¿Para qué se usa este modelo?", "¿Cómo se factura este modelo?", "¿Qué endpoint de API llama a este modelo?", "¿Qué límites de contexto o entrada se aplican?"],
    faqAnswers: ["El modelo aparece en el catálogo Flatkey con las modalidades indicadas en esta página.", "La tarifa actual aparece en la tabla de precios de Flatkey y puede variar según la ruta y la cuenta.", "Usa el endpoint y el ID de modelo mostrados en la sección API.", "Los límites de contexto y entrada son datos del catálogo o de la ruta; verifícalos antes de producción."],
  },
  fr: {
    task: { image: "générateur d’images IA et API", video: "générateur vidéo IA et API", audio: "API de génération audio", text: "API de modèle IA" },
    performance: "Performances de l’API", performanceTitle: "Fiabilité et disponibilité de l’API", activity: "Activité de l’API", activityTitle: "Utilisation et activité des requêtes API", pricing: "tarifs", pricingTitle: "Tarifs et facturation de l’API", capabilitiesTitle: "capacités principales", comparisonTitle: "Comparaison API : capacités et accès", whyTitle: "Pourquoi utiliser ce modèle via Flatkey ?", apiTitle: "Guide d’intégration API", relatedTitle: "API de modèles associés", faqAfter: "questions sur les tarifs, fonctions et l’usage", faqDescription: "Réponses sur les tarifs, capacités, endpoints et limites du catalogue.",
    faqQuestions: ["À quoi sert ce modèle ?", "Comment ce modèle est-il facturé ?", "Quel endpoint API appelle ce modèle ?", "Quelles limites de contexte ou d’entrée s’appliquent ?"],
    faqAnswers: ["Le modèle figure dans le catalogue Flatkey avec les modalités indiquées sur cette page.", "Le tarif actuel apparaît dans le tableau Flatkey et peut varier selon la route et le compte.", "Utilisez l’endpoint et l’identifiant du modèle indiqués dans la section API.", "Les limites de contexte et d’entrée sont des faits du catalogue ou de la route ; vérifiez-les avant la production."],
  },
  ru: {
    task: { image: "ИИ-генератор изображений и API", video: "ИИ-генератор видео и API", audio: "API генерации аудио", text: "API модели ИИ" },
    performance: "Производительность API", performanceTitle: "Надёжность и доступность API", activity: "Активность API", activityTitle: "Использование и активность запросов API", pricing: "цены", pricingTitle: "Цены и расчёты API", capabilitiesTitle: "основные возможности", comparisonTitle: "Сравнение API: возможности и доступ", whyTitle: "Зачем использовать эту модель через Flatkey?", apiTitle: "Руководство по интеграции API", relatedTitle: "API похожих моделей", faqAfter: "вопросы о ценах, функциях и использовании", faqDescription: "Ответы о ценах, возможностях, конечных точках и лимитах каталога.",
    faqQuestions: ["Для чего используется эта модель?", "Как тарифицируется эта модель?", "Какой API endpoint вызывает эту модель?", "Какие ограничения контекста или входных данных действуют?"],
    faqAnswers: ["Модель есть в каталоге Flatkey с модальностями, указанными на этой странице.", "Текущая ставка указана в таблице цен Flatkey и может зависеть от маршрута и аккаунта.", "Используйте endpoint и ID модели из раздела API.", "Ограничения контекста и входа зависят от каталога или маршрута; проверьте их перед запуском в продакшене."],
  },
  ja: {
    task: { image: "AI画像生成とAPI", video: "AI動画生成とAPI", audio: "音声生成API", text: "AIモデルAPI" },
    performance: "APIパフォーマンス", performanceTitle: "APIの信頼性と可用性", activity: "APIアクティビティ", activityTitle: "APIの利用状況とリクエスト", pricing: "料金", pricingTitle: "API料金と課金", capabilitiesTitle: "主な機能", comparisonTitle: "API比較：機能とアクセス", whyTitle: "Flatkey経由でこのモデルを使う理由", apiTitle: "API統合ガイド", relatedTitle: "関連モデルAPI", faqAfter: "料金・機能・利用に関するFAQ", faqDescription: "料金、機能、エンドポイント、カタログ制限に関する回答です。",
    faqQuestions: ["このモデルは何に使えますか？", "このモデルの料金体系は？", "どのAPIエンドポイントで呼び出せますか？", "コンテキストや入力の制限は？"],
    faqAnswers: ["このモデルはページに示すモダリティでFlatkeyカタログに掲載されています。", "現在の料金はFlatkeyのライブ料金表に表示され、経路やアカウントで変わる場合があります。", "APIセクションに示すエンドポイントとモデルIDを使用してください。", "コンテキストと入力制限はカタログまたは経路の事実を確認してください。"],
  },
  vi: {
    task: { image: "trình tạo ảnh AI và API", video: "trình tạo video AI và API", audio: "API tạo âm thanh", text: "API mô hình AI" },
    performance: "Hiệu suất API", performanceTitle: "Độ tin cậy và khả dụng của API", activity: "Hoạt động API", activityTitle: "Mức sử dụng và hoạt động yêu cầu API", pricing: "giá", pricingTitle: "Giá và thanh toán API", capabilitiesTitle: "năng lực chính", comparisonTitle: "So sánh API: năng lực và quyền truy cập", whyTitle: "Vì sao dùng mô hình này qua Flatkey?", apiTitle: "Hướng dẫn tích hợp API", relatedTitle: "API mô hình liên quan", faqAfter: "câu hỏi về giá, tính năng và cách dùng", faqDescription: "Giải đáp về giá, năng lực, endpoint và giới hạn danh mục.",
    faqQuestions: ["Mô hình này dùng để làm gì?", "Mô hình này được tính giá thế nào?", "Endpoint API nào gọi mô hình này?", "Giới hạn ngữ cảnh hoặc đầu vào là gì?"],
    faqAnswers: ["Mô hình có trong danh mục Flatkey với các phương thức được nêu trên trang.", "Giá hiện tại nằm trong bảng giá trực tiếp của Flatkey và có thể đổi theo tuyến hoặc tài khoản.", "Dùng endpoint và model ID trong phần API.", "Giới hạn ngữ cảnh và đầu vào là dữ kiện của danh mục hoặc tuyến; hãy kiểm tra trước khi chạy production."],
  },
  de: {
    task: { image: "KI-Bildgenerator und API", video: "KI-Videogenerator und API", audio: "Audio-Generierungs-API", text: "KI-Modell-API" },
    performance: "API-Leistung", performanceTitle: "Zuverlässigkeit und Verfügbarkeit der API", activity: "API-Aktivität", activityTitle: "API-Nutzung und Anfrageaktivität", pricing: "Preise", pricingTitle: "API-Preise und Abrechnung", capabilitiesTitle: "zentrale Funktionen", comparisonTitle: "API-Vergleich: Funktionen und Zugriff", whyTitle: "Warum dieses Modell über Flatkey nutzen?", apiTitle: "API-Integrationsleitfaden", relatedTitle: "Verwandte Modell-APIs", faqAfter: "Fragen zu Preisen, Funktionen und Nutzung", faqDescription: "Antworten zu Preisen, Funktionen, Endpunkten und Kataloglimits.",
    faqQuestions: ["Wofür wird dieses Modell verwendet?", "Wie wird dieses Modell abgerechnet?", "Welcher API-Endpunkt ruft dieses Modell auf?", "Welche Kontext- oder Eingabelimits gelten?"],
    faqAnswers: ["Das Modell ist mit den auf dieser Seite genannten Modalitäten im Flatkey-Katalog aufgeführt.", "Der aktuelle Tarif steht in der Live-Preistabelle und kann je nach Route und Konto variieren.", "Verwende den im API-Bereich genannten Endpunkt und die Modell-ID.", "Kontext- und Eingabelimits sind Katalog- oder Routenfakten; prüfe sie vor dem Produktionseinsatz."],
  },
  id: {
    task: { image: "generator gambar AI dan API", video: "generator video AI dan API", audio: "API generasi audio", text: "API model AI" },
    performance: "Performa API", performanceTitle: "Keandalan dan ketersediaan API", activity: "Aktivitas API", activityTitle: "Penggunaan dan aktivitas permintaan API", pricing: "harga", pricingTitle: "Harga dan penagihan API", capabilitiesTitle: "kemampuan utama", comparisonTitle: "Perbandingan API: kemampuan dan akses", whyTitle: "Mengapa memakai model ini melalui Flatkey?", apiTitle: "Panduan integrasi API", relatedTitle: "API model terkait", faqAfter: "pertanyaan harga, fitur, dan penggunaan", faqDescription: "Jawaban tentang harga, kemampuan, endpoint, dan batas katalog.",
    faqQuestions: ["Untuk apa model ini digunakan?", "Bagaimana harga model ini?", "Endpoint API mana yang memanggil model ini?", "Apa batas konteks atau inputnya?"],
    faqAnswers: ["Model ini tercantum di katalog Flatkey dengan modalitas yang ditampilkan di halaman ini.", "Tarif saat ini ada di tabel harga live Flatkey dan dapat berbeda menurut rute dan akun.", "Gunakan endpoint dan ID model pada bagian API.", "Batas konteks dan input adalah fakta katalog atau rute; verifikasi sebelum produksi."],
  },
};

function localizeGenericLandingContent(
  source: ModelLandingContent,
  locale: Locale,
  name: string,
  forcedKind?: ModelGeneratorConfig["kind"],
): ModelLandingContent {
  const copy = GENERIC_LOCALE_COPY[locale] ?? GENERIC_LOCALE_COPY.en;
  // Prefer the resolved generator kind. Model names such as
  // `sonilo-video-to-music` contain the word "video" even though the route is
  // audio, so inferring from a title alone can localize the page incorrectly.
  const kind = forcedKind ?? (source.hero?.title?.includes("image") || source.capabilitiesTitle?.includes("image")
    ? "image"
    : source.hero?.title?.includes("video") || source.capabilitiesTitle?.includes("video")
      ? "video"
      : source.hero?.title?.includes("audio") || source.capabilitiesTitle?.includes("audio")
        ? "audio"
        : "text");
  const endpoint = source.api?.items?.[0]?.title ?? "/v1/chat/completions";
  const task = copy.task[kind];
  const sourceFacts = source.hero?.description?.split(";").slice(1).join(";").trim() ?? "";
  const heroDescription = locale === "zh"
    ? `${name} 可通过 Flatkey 使用；${sourceFacts}`.trim()
    : `${name} ${task} via Flatkey. ${sourceFacts}`.trim();
  return {
    ...source,
    hero: source.hero ? { ...source.hero, title: `${name} ${task}`, description: heroDescription } : source.hero,
    performance: source.performance ? { ...source.performance, eyebrow: copy.performance, title: `${name} ${copy.performanceTitle}` } : source.performance,
    activity: source.activity ? { ...source.activity, eyebrow: copy.activity, title: `${name} ${copy.activityTitle}` } : source.activity,
    pricing: source.pricing ? { ...source.pricing, eyebrow: `${name} ${copy.pricing}`, title: `${name} ${copy.pricingTitle}` } : source.pricing,
    capabilitiesTitle: `${name} ${task}: ${copy.capabilitiesTitle}`,
    comparison: source.comparison ? { ...source.comparison, title: `${name} ${copy.comparisonTitle}` } : source.comparison,
    promptLibraryTitle: source.promptLibraryTitle ? `${name} ${task} prompt examples` : source.promptLibraryTitle,
    why: source.why ? { ...source.why, eyebrow: `Why ${name} API`, title: `${copy.whyTitle.replace("this model", name).replace("ce modèle", name).replace("dieses Modell", name)}` } : source.why,
    api: source.api ? { ...source.api, eyebrow: `${name} API`, title: `${name} ${copy.apiTitle}`, items: source.api.items.map((item, index) => index === 0 ? { ...item, title: endpoint } : item) } : source.api,
    related: source.related ? { ...source.related, eyebrow: copy.relatedTitle, title: `${name} ${copy.relatedTitle}` } : source.related,
    faqTitle: source.faqTitle ? { beforeBreak: `${name} API`, afterBreak: copy.faqAfter } : source.faqTitle,
    faqDescription: copy.faqDescription,
    faq: source.faq?.map((item, index) => ({ question: copy.faqQuestions[index] ?? item.question, answer: copy.faqAnswers[index] ?? item.answer })),
  };
}

function inferMediaKind(model: PricingModel): ModelGeneratorConfig["kind"] | null {
  const name = normalizeModelId(model.model_name);
  const endpoints = (model.supported_endpoint_types ?? []).map((endpoint) => normalizeModelId(endpoint));
  const endpointText = endpoints.join("-");
  if (
    endpointText.includes("video-to-music") ||
    endpointText.includes("audio") ||
    endpointText.includes("music") ||
    endpointText.includes("sound") ||
    /(^|-)(audio|music|voice|tts|sfx|sound|sonilo|suno|lyrics)(-|$)/.test(name)
  ) {
    return "audio";
  }
  if (
    endpointText.includes("image-generation") ||
    /(^|-)(image|imagen|banana|flux|ideogram)(-|$)/.test(name)
  ) {
    return "image";
  }
  if (
    endpointText.includes("video") ||
    /(^|-)(video|sora|veo|kling|seedance|hailuo|runway|wan)(-|$)/.test(name)
  ) {
    return "video";
  }
  return null;
}

function mediaEndpointForModel(kind: ModelGeneratorConfig["kind"], model: PricingModel): string {
  const endpoints = (model.supported_endpoint_types ?? []).map((endpoint) => normalizeModelId(endpoint));
  if (endpoints.some((endpoint) => endpoint.includes("video-to-music"))) return "/v1/video-to-music";
  if (endpoints.some((endpoint) => endpoint.includes("sound"))) return "/v1/sound-generation";
  if (endpoints.some((endpoint) => endpoint.includes("music"))) return "/v1/music";
  if (kind === "image") return "/v1/images/generations";
  if (kind === "video") return "/v1/videos";
  return "/v1/music";
}

function examplePromptForMediaKind(kind: ModelGeneratorConfig["kind"], modelName: string): string {
  if (kind === "audio") {
    return `Create a polished music bed for ${modelName}: keep the video timing, preserve important speech, use a warm electronic style, and deliver a clean loopable ending.`;
  }
  if (kind === "video") {
    return `Create a short product video with ${modelName}: clear subject motion, realistic lighting, stable camera, and production-ready framing.`;
  }
  return `Create a high-quality product image with ${modelName}: clean composition, precise lighting, strong subject focus, and realistic detail.`;
}

function formatPriceLiteral(value: number): string {
  return `$${Number(value.toFixed(6)).toString()}`;
}

const en: Record<ModelLandingKey, string> = {
  "All models": "All models",
  "Back to Models": "Back to Models",
  "View Pricing": "View Pricing",
  "View API": "View API",
  Docs: "Docs",
  Rankings: "Rankings",
  "Related pages": "Related pages",
  "Keep exploring Flatkey": "Keep exploring Flatkey",
  "Related models": "Related models",
  "More models from {{provider}}": "More models from {{provider}}",
  "Swipe or scroll to compare": "Swipe or scroll to compare",
  "Previous example": "Previous example",
  "Next example": "Next example",
  "Example {{index}} of {{total}}": "Example {{index}} of {{total}}",
  "Generated with {{model}}": "Generated with {{model}}",
  "Live catalog model": "Live catalog model",
  "Catalog data unavailable": "Catalog data unavailable",
  Providers: "Providers",
  Provider: "Provider",
  API: "API",
  Pricing: "Pricing",
  "Price type": "Price type",
  "Price / image": "Price / image",
  "Previous generation": "Previous generation",
  Breadcrumb: "Breadcrumb",
  "Model sections": "Model sections",
  "Sample usage trend": "Sample usage trend",
  "Usage trend": "Usage trend",
  "Upload or drag and drop": "Upload or drag and drop",
  Prompt: "Prompt",
  Images: "Images",
  "Video URL": "Video URL",
  "Preserve speech": "Preserve speech",
  "Remove reference image": "Remove reference image",
  "Uploaded {{count}} files": "{{count}} uploaded files",
  "Remove {{name}}": "Remove {{name}}",
  "Model price comparison": "Model price comparison",
  "After bonus": "After bonus",
  "Pricing data unavailable": "Pricing data unavailable",
  Performance: "Performance",
  Benchmarks: "Benchmarks",
  Apps: "Apps",
  Activity: "Activity",
  FAQ: "FAQ",
  Compare: "Compare",
  Playground: "Playground",
  Parameters: "Parameters",
  "Prompt library": "Prompt library",
  Modalities: "Modalities",
  "In / Out price": "In / Out price",
  Context: "Context",
  Released: "Released",
  "Knowledge cutoff": "Knowledge cutoff",
  "Input /M": "Input /M",
  "Output /M": "Output /M",
  Latency: "Latency",
  Uptime: "Uptime",
  "Live model health": "Live model health",
  "30-day health, measured on real traffic": "30-day health, measured on real traffic",
  "Available catalog entries": "Available catalog entries",
  Status: "Status",
  Throughput: "Throughput",
  "Successful inference trend": "Successful inference trend",
  "Not enough data yet": "Not enough data yet",
  "Avg. provider uptime": "Avg. provider uptime",
  Requests: "Requests",
  "Frequently asked questions": "Frequently asked questions",
  "Reliability Index": "Reliability Index",
  "Throughput Index": "Throughput Index",
  "Latency Index": "Latency Index",
  "Prompt testing and request handoff.": "Prompt testing and request handoff.",
  "Monthly token share from Flatkey rankings.": "Monthly token share from Flatkey rankings.",
  "Model catalog": "Model catalog",
  "Related models from the pricing catalog.": "Related models from the pricing catalog.",
  "API Gateway": "API Gateway",
  "Supported endpoint coverage in our pricing API.": "Supported endpoint coverage in our pricing API.",
  "Weighted avg input price": "Weighted avg input price",
  "Weighted avg output price": "Weighted avg output price",
  "Cache read": "Cache read",
  "Cache write": "Cache write",
  "Request price": "Request price",
  "Reference price": "Reference price",
  "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.",
  "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.",
  "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.",
  "Token volume and request traffic for this model over time.": "Token volume and request traffic for this model over time.",
  "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.",
  "What is {{model}}?": "What is {{model}}?",
  "How much does {{model}} cost?": "How much does {{model}} cost?",
  "Which providers serve {{model}}?": "Which providers serve {{model}}?",
  "Use the pricing section above for current Flatkey prices from our pricing API.": "Use the pricing section above for current Flatkey prices from our pricing API.",
  "The providers section shows the upstream provider names available in our model catalog.": "The providers section shows the upstream provider names available in our model catalog.",
  "You pay": "You pay",
  "per month on the Go plan": "per month on the Go plan",
  "You get": "You get",
  "of monthly model usage — 4.5× the price": "of monthly model usage — 4.5× the price",
  "from $10/month": "from $10/month",
  "Pro — $30/mo, up to $90 usage": "Pro — $30/mo, up to $90 usage",
  "Most popular": "Most popular",
  "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price",
  "▶ Sign in to run": "▶ Sign in to run",
  "Start generating": "Start generating",
  "Generator setup": "Generator setup",
  "Saved before signup": "Saved before signup",
  "Public demo": "Public demo",
  Size: "Size",
  Quality: "Quality",
  Outputs: "Outputs",
  "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.",
  "(flatkey · official ≈ {{price}})": "(flatkey · official ≈ {{price}})",
  "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · OpenAI-compatible · one key, all models",
  "* Illustrative pricing — see flatkey pricing page": "* Illustrative pricing — see flatkey pricing page",
  "/ million output tokens": "/ million output tokens",
  "/ second": "/ second",
  "# Your existing OpenAI code:": "# Your existing OpenAI code:",
  "up to 50% off": "up to 50% off",
  "covers every model": "covers every model",
  "Est. this run": "Est. this run",
  "One subscription": "One subscription",
  "Google / GitHub one-click · no credit card to start": "Google / GitHub one-click · no credit card to start",
  "migrate.py — change one line": "migrate.py — change one line",
  "Text, image and video in one plan · overage billed as you go · cancel anytime": "Text, image and video in one plan · overage billed as you go · cancel anytime",
  "Playground (edit before sign-up)": "Playground (edit before sign-up)",
  "Pricing vs official": "Pricing vs official",
  "Compare the current model with the previous generation before you migrate.": "Compare the current model with the previous generation before you migrate.",
  "Best for": "Best for",
  "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.",
  "See plans →": "See plans →",
  "Starter / individual": "Starter / individual",
  "Team / high-volume": "Team / high-volume",
  "The same {{model}},": "The same {{model}},",
  "Go — $10/mo, up to $25 usage": "Go — $10/mo, up to $25 usage",
  "Max — $100/mo, up to $450 usage": "Max — $100/mo, up to $450 usage",
  "/ image": "/ image",
  "/ request": "/ request",
  "Opus 4 output": "Opus 4 output",
  "Sonnet 4 output": "Sonnet 4 output",
  "Haiku output": "Haiku output",
  "GPT-5 output": "GPT-5 output",
  "GPT-5 mini output": "GPT-5 mini output",
  "GPT-5 input": "GPT-5 input",
  "GPT-image-2 image": "GPT-image-2 image",
  "Square output": "Square output",
  "Fast product mockups": "Fast product mockups",
  "Gemini 2.5 Pro output": "Gemini 2.5 Pro output",
  "Gemini 2.5 Flash output": "Gemini 2.5 Flash output",
  "Gemini 2.5 Pro input": "Gemini 2.5 Pro input",
  "Seedance video / sec": "Seedance video / sec",
  "Image-to-video / sec": "Image-to-video / sec",
  "1080p / sec": "1080p / sec",
  "MiniMax-H3 768P / sec": "MiniMax-H3 768P / sec",
  "MiniMax-H3 2K / sec": "MiniMax-H3 2K / sec",
  "Reference video input": "Reference video input",
  "Input image after free tier": "Input image after free tier",
  "same per-second rate": "same per-second rate",
  "AIGC watermark": "AIGC watermark",
  "Cache reads": "Cache reads",
  Coverage: "Coverage",
  "AI app backends": "AI app backends",
  "Agent workflows": "Agent workflows",
  "Batch content generation": "Batch content generation",
  "Best for general AI apps, agents, search, and high-volume API workloads": "Best for general AI apps, agents, search, and high-volume API workloads",
  "Best for long-context reasoning, coding agents, and production assistants": "Best for long-context reasoning, coding agents, and production assistants",
  "Best for product videos, ad creative, and image-to-video production": "Best for product videos, ad creative, and image-to-video production",
  "Best for video soundtracks, speech-preserving edits, and audio production": "Best for video soundtracks, speech-preserving edits, and audio production",
  "Can I control usage before scaling?": "Can I control usage before scaling?",
  "Coding agents": "Coding agents",
  "Product mockups": "Product mockups",
  "Ad creatives": "Ad creatives",
  "Ecommerce images": "Ecommerce images",
  "Best for product images, ad creatives, and ecommerce visual variants": "Best for product images, ad creatives, and ecommerce visual variants",
  "Does this start a real generation?": "Does this start a real generation?",
  "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.",
  "Which MiniMax-H3 fields can I configure here?": "Which MiniMax-H3 fields can I configure here?",
  "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "Configure resolution, duration, ratio, and AIGC watermark before opening the console.",
  "Where are my edited prompt settings stored?": "Where are my edited prompt settings stored?",
  "They are stored in this browser's localStorage so the draft survives the signup handoff.": "They are stored in this browser's localStorage so the draft survives the signup handoff.",
  "Does this use the same model id in my SDK?": "Does this use the same model id in my SDK?",
  "Live flatkey pricing": "Live flatkey pricing",
  "Live model data from pricing API": "Live model data from pricing API",
  "Long document analysis": "Long document analysis",
  "Matched live models": "Matched live models",
  "Product motion": "Product motion",
  "Social video variants": "Social video variants",
  "Support automation": "Support automation",
  "UGC ad clips": "UGC ad clips",
  "Yes. Keep your SDK and switch base_url plus api_key.": "Yes. Keep your SDK and switch base_url plus api_key.",
  "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.",
  "50% off": "50% off",
  Capabilities: "Capabilities",
  "Use one account and API key across text, image, video, and audio models.": "Use one account and API key across text, image, video, and audio models.",
  "Keep prompts, quotas, and model routing in one place.": "Keep prompts, quotas, and model routing in one place.",
  "Reference image": "Reference image",
  "Reference videos": "Reference videos",
  "Reference Audios": "Reference Audios",
  "Video preview": "Video preview",
  "Image preview": "Image preview",
  "Audio preview": "Audio preview",
  "Request summary": "Request summary",
  "Preview only. Sign in to submit this request to Flatkey.": "Preview only. Sign in to submit this request to Flatkey.",
  "Image to Video": "Image to Video",
  "Reference-guided Video": "Reference-guided Video",
  "Short-form Video": "Short-form Video",
  "Text to Image": "Text to Image",
  "Reference-guided Image": "Reference-guided Image",
  "Image Editing": "Image Editing",
  "Text to Audio": "Text to Audio",
  "Video to Audio": "Video to Audio",
  "Speech synthesis": "Speech synthesis",
  "Audio generation": "Audio generation",
  "Speech preservation": "Speech preservation",
  "Audio variants": "Audio variants",
  "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Create music, sound beds, and polished audio tracks from a media-aware brief.",
  "Keep important dialogue and speech intelligible while adding a new audio layer.": "Keep important dialogue and speech intelligible while adding a new audio layer.",
  "Generate multiple delivery options for edits, localization, and production handoff.": "Generate multiple delivery options for edits, localization, and production handoff.",
  "Image editing": "Image editing",
  "Reference-guided motion": "Reference-guided motion",
  "Camera and pacing": "Camera and pacing",
  "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Generate product-style shots, merchandising scenes, and reference-guided variations.",
  "Produce campaign concepts, thumbnails, posters, and localized variants.": "Produce campaign concepts, thumbnails, posters, and localized variants.",
  "Refine existing images, swap details, and create consistent variations for each channel.": "Refine existing images, swap details, and create consistent variations for each channel.",
  "Add async generation for agents, CMS tools, and batch creative systems.": "Add async generation for agents, CMS tools, and batch creative systems.",
  "Create product shots, social clips, and story scenes with controlled motion and framing.": "Create product shots, social clips, and story scenes with controlled motion and framing.",
  "Use image, video, and audio references to keep subjects and creative direction consistent.": "Use image, video, and audio references to keep subjects and creative direction consistent.",
  "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Configure ratio, resolution, duration, and sound before handing the request to the API.",
  "Chat and coding": "Chat and coding",
  "Long context": "Long context",
  "Tool workflows": "Tool workflows",
  "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.",
  "SDK for developers": "SDK for developers",
  "Use your existing SDK and set its base URL to the Flatkey router origin.": "Use your existing SDK and set its base URL to the Flatkey router origin.",
  "Flatkey CLI": "Flatkey CLI",
  "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Keep prompts and request files in your terminal workflow with the Flatkey CLI.",
  "Codex & Claude Code": "Codex & Claude Code",
  "Use the same key with your coding agent and route model jobs from a script.": "Use the same key with your coding agent and route model jobs from a script.",
  "Price / second": "Price / second",
  generate: "generate",
  "30-day window": "30-day window",
  "last 30 days": "last 30 days",
  Endpoint: "Endpoint",
  "Aspect ratio": "Aspect ratio",
  On: "On",
  Off: "Off",
  "Reference media": "Reference media",
  Models: "Models",
  "Video generation": "Video generation",
  "seedance-2.5 API": "seedance-2.5 API",
  "Seedance-2.5 API": "Seedance-2.5 API",
  "Quick Start": "Quick Start",
  "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.",
  "Reliability over the last 30 days": "Reliability over the last 30 days",
  "Measured on real Flatkey traffic, with production monitoring.": "Measured on real Flatkey traffic, with production monitoring.",
  "Average latency": "Average latency",
  "Task accepted; generation continues asynchronously": "Task accepted; generation continues asynchronously",
  "Last 30 days": "Last 30 days",
  Usage: "Usage",
  "Daily seedance-2.5 requests on Flatkey": "Daily seedance-2.5 requests on Flatkey",
  "Sample shape shown while live telemetry is being connected.": "Sample shape shown while live telemetry is being connected.",
  "Total Requests": "Total Requests",
  "Daily Average": "Daily Average",
  "Busiest Day": "Busiest Day",
  "What seedance-2.5 can do": "What seedance-2.5 can do",
  "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.",
  "Text and image to video": "Text and image to video",
  "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Generate from a written scene, or drive it with reference images for a subject you have already designed.",
  "Multi-shot consistency": "Multi-shot consistency",
  "Hold characters, wardrobe, and setting across cuts within a single generation.": "Hold characters, wardrobe, and setting across cuts within a single generation.",
  "Native audio": "Native audio",
  "Ambient sound and speech are generated with the picture, in multiple languages.": "Ambient sound and speech are generated with the picture, in multiple languages.",
  "Edit and extend": "Edit and extend",
  "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Continue an existing clip or revise one, with first-frame and first/last-frame control.",
  Capability: "Capability",
  "What changed from the previous generation, so you can tell whether it is worth switching.": "What changed from the previous generation, so you can tell whether it is worth switching.",
  "Seedance 2.0": "Seedance 2.0",
  "Seedance 2.5": "Seedance 2.5",
  "Clip length": "Clip length",
  "Short clips, stitched for longer runs": "Short clips, stitched for longer runs",
  "Up to 30s in a single continuous shot": "Up to 30s in a single continuous shot",
  "Reference inputs": "Reference inputs",
  "Image references": "Image references",
  "Up to 50 per request — 30 images, 10 videos, 10 audio": "Up to 50 per request — 30 images, 10 videos, 10 audio",
  "Motion control": "Motion control",
  "Text prompt only": "Text prompt only",
  "Structured motion paths, green-screen and white-model references": "Structured motion paths, green-screen and white-model references",
  Audio: "Audio",
  "Generated audio": "Generated audio",
  "Native audio in 10+ languages": "Native audio in 10+ languages",
  Editing: "Editing",
  "Regenerate to change a clip": "Regenerate to change a clip",
  "Edit and extend an existing clip in place": "Edit and extend an existing clip in place",
  "High-Speed Action": "High-Speed Action",
  "Seedance-2.5 prompts that work": "Seedance-2.5 prompts that work",
  "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.",
  "Copy Prompt": "Copy Prompt",
  "Make one like this": "Make one like this",
  "Why Flatkey": "Why Flatkey",
  "Why run seedance-2.5 through Flatkey": "Why run seedance-2.5 through Flatkey",
  "One key, one balance, and the same upstream model you would call directly.": "One key, one balance, and the same upstream model you would call directly.",
  "10% below list price": "10% below list price",
  "The same upstream model, billed from one balance that also covers text, image, and audio models.": "The same upstream model, billed from one balance that also covers text, image, and audio models.",
  "OpenAI-compatible from day one": "OpenAI-compatible from day one",
  "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.",
  "Swap models without a new integration": "Swap models without a new integration",
  "Move between ByteDance and every other model in the catalog by changing one string.": "Move between ByteDance and every other model in the catalog by changing one string.",
  "Routing across upstream channels": "Routing across upstream channels",
  "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Requests are spread across the channels serving this model, with 30-day uptime published above.",
  "How to call the seedance-2.5 API": "How to call the seedance-2.5 API",
  "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.",
  "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.",
  "Use the OpenAI client and set baseURL to your Flatkey router origin.": "Use the OpenAI client and set baseURL to your Flatkey router origin.",
  "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.",
  "Codex&Claude Code": "Codex&Claude Code",
  "Use the same key with your coding agent and route video jobs from a script.": "Use the same key with your coding agent and route video jobs from a script.",
  "Related Models": "Related Models",
  "Other video generation models": "Other video generation models",
  "Text-to-video · image-to-video": "Text-to-video · image-to-video",
  DeepSeek: "DeepSeek",
  "OpenAI-compatible text model": "OpenAI-compatible text model",
  MiniMax: "MiniMax",
  "Creative video generation": "Creative video generation",
  "Veo 3.1": "Veo 3.1",
  "High-fidelity video generation": "High-fidelity video generation",
  Kling: "Kling",
  "Motion control and references": "Motion control and references",
  Sora: "Sora",
  "Story and scene generation": "Story and scene generation",
  Wan: "Wan",
  "Fast creative variants": "Fast creative variants",
  Hailuo: "Hailuo",
  "Social-ready clips": "Social-ready clips",
  "What is seedance-2.5?": "What is seedance-2.5?",
  "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.",
  "How much does seedance-2.5 cost?": "How much does seedance-2.5 cost?",
  "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.",
  "What can I use it for?": "What can I use it for?",
  "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.",
  "How do I use the model in my app?": "How do I use the model in my app?",
  "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.",
  "Can I control output features?": "Can I control output features?",
  "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.",
  "Is the Flatkey API OpenAI compatible?": "Is the Flatkey API OpenAI compatible?",
  "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.",
  "What limits apply?": "What limits apply?",
  "Rate limits and available model IDs depend on your account and current upstream availability.": "Rate limits and available model IDs depend on your account and current upstream availability.",
  "What happens to my prompts and generated files?": "What happens to my prompts and generated files?",
  "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.",
  "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.",
  "API–frequently asked questions": "API–frequently asked questions",
  "Seedance-2.5 API–frequently asked questions": "Seedance-2.5 API–frequently asked questions",
  "Pricing, compatibility, limits, and how your prompts and generated files are handled.": "Pricing, compatibility, limits, and how your prompts and generated files are handled.",
  None: "None",
};

const translations: Record<Locale, Record<string, string>> = withIdFallback<Record<string, string>>({
  en,
  zh: {
    "All models": "全部模型",
    "Back to Models": "返回模型列表",
    "Copy page": "复制页面",
    "Copy model id": "复制模型 ID",
    "Flatkey Router": "Flatkey Router",
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "在公开页配置 {{model}} 请求。Flatkey 会先把草稿保存到本地，然后打开控制台，让你用自己的账号和 API Key 运行。",
    "Model Type": "模型类型",
    "Text to Video": "文生视频",
    Audio: "音频",
    "Image to Image": "图像生成",
    API: "API",
    Pricing: "价格",
    "Model ID": "模型 ID",
    Playground: "Playground",
    Examples: "示例",
    Similar: "相似模型",
    README: "README",
    Input: "输入",
    Form: "表单",
    "Join and run": "注册后运行",
    Output: "输出",
    Preview: "预览",
    "View API Docs": "查看 API 文档",
    "Get API Key": "获取 API Key",
    "Images generated": "已生成图片",
    "Videos generated": "已生成视频",
    "Avg. response time": "平均响应时间",
    Uptime: "可用性",
    "Ready for production": "可用于生产",
    "Generated Examples": "生成示例",
    "Explore what {{model}} can create": "看看 {{model}} 可以生成什么",
    "Create with this model": "用这个模型创建",
    "Generated with {{model}}": "使用 {{model}} 生成",
    "Example {{index}} of {{total}}": "示例 {{index}} / {{total}}",
    "Previous example": "上一个示例",
    "Next example": "下一个示例",
    "Live catalog model": "实时目录模型",
    "Catalog data unavailable": "目录数据暂不可用",
    "Related pages": "相关页面",
    "Keep exploring Flatkey": "继续浏览 Flatkey",
    "Related models": "相关模型",
    "More models from {{provider}}": "更多 {{provider}} 模型",
    "Swipe or scroll to compare": "滑动或滚动查看",
    Providers: "供应商",
    Provider: "供应商",
    Performance: "性能",
    "Model price comparison": "模型价格对比",
    "After bonus": "充值后",
    "Pricing data unavailable": "价格数据暂不可用",
    Benchmarks: "基准测试",
    Apps: "应用",
    Activity: "活动",
    FAQ: "FAQ",
    Modalities: "模态",
    "In / Out price": "输入 / 输出价格",
    Context: "上下文",
    Released: "发布时间",
    "Knowledge cutoff": "知识截止",
    "Input /M": "输入 /M",
    "Output /M": "输出 /M",
    Latency: "延迟",
    "Live model health": "实时模型健康",
    "30-day health, measured on real traffic": "基于真实流量的 30 天健康度",
    "Available catalog entries": "可用目录条目",
    Status: "状态",
    Throughput: "吞吐",
    "Successful inference trend": "成功推理趋势",
    "Not enough data yet": "数据积累中",
    "Avg. provider uptime": "供应商平均可用性",
    Requests: "请求量",
    "Frequently asked questions": "常见问题",
    "Reliability Index": "可靠性指数",
    "Throughput Index": "吞吐指数",
    "Latency Index": "延迟指数",
    "Prompt testing and request handoff.": "Prompt 测试和请求接力。",
    "Monthly token share from Flatkey rankings.": "来自 Flatkey 排行榜的月度 token 占比。",
    "Model catalog": "模型目录",
    "Related models from the pricing catalog.": "来自价格目录的相关模型。",
    "API Gateway": "API 网关",
    "Supported endpoint coverage in our pricing API.": "定价 API 中支持的 endpoint 覆盖。",
    "Weighted avg input price": "加权平均输入价格",
    "Weighted avg output price": "加权平均输出价格",
    "Cache read": "缓存读取",
    "Cache write": "缓存写入",
    "Request price": "请求价格",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey 会把请求路由到该模型可用的上游通道，并在一个账号下统一计费。",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "下方价格来自当前模型的 Flatkey 定价数据，以及定价 API 返回的可见分组。",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "当流量足够时，性能数据使用 Flatkey 近 30 天请求遥测。",
    "Token volume and request traffic for this model over time.": "该模型随时间变化的 token 用量和请求流量。",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} 可通过 Flatkey 使用，并提供实时价格、供应商路由、生成示例、API 接力和相关模型内链。",
    "What is {{model}}?": "{{model}} 是什么？",
    "How much does {{model}} cost?": "{{model}} 多少钱？",
    "Which providers serve {{model}}?": "哪些供应商提供 {{model}}？",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "请查看上方价格区，那里展示了定价 API 返回的当前 Flatkey 价格。",
    "The providers section shows the upstream provider names available in our model catalog.": "供应商区展示了我们模型目录中该模型可用的上游供应商。",
    Rankings: "排行",
    "Transparent Pricing": "透明价格",
    "Flatkey {{model}} usage pricing": "Flatkey {{model}} 用量价格",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "图像、视频、音频和文本模型共用同一个 Flatkey 余额和 API Key。",
    "Open wallet": "打开钱包",
    "Flatkey price": "Flatkey 价格",
    "Reference price": "官方价格",
    "Shared balance": "共用余额",
    "What seedance-2.5 can do": "seedance-2.5 能做什么",
    "Why Flatkey": "为什么选择 Flatkey",
    "Why use Flatkey for {{model}}?": "为什么用 Flatkey 调用 {{model}}？",
    "Lower generation pricing": "更低的生成成本",
    "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "媒体生成任务走 Flatkey，扩量前的 prompt 测试成本更可控。",
    "Draft handoff": "草稿接力",
    "The public page stores prompt settings locally before sending the user into Flatkey.": "公开页会先在本地保存 prompt 和参数，再把用户带到 Flatkey。",
    "Unified API access": "统一 API 入口",
    "Use one account and API key across image, video, audio, and language models.": "一个账号和 API Key 覆盖图像、视频、音频和语言模型。",
    "Start generating in three steps": "三步开始生成",
    "Try a prompt": "先试 prompt",
    "Use the playground to validate quality and style fit.": "在 playground 中验证质量、风格和参数是否合适。",
    "Create an API key": "创建 API Key",
    "Sign up, open Dashboard, and create a token for this model.": "注册后进入控制台，为这个模型创建 token。",
    "Ship your workflow": "接入你的工作流",
    "Call the same endpoint, then top up credits as usage grows.": "调用同一个 endpoint，并随用量增长充值额度。",
    "Built for generation teams": "为生成团队准备",
    "Ads and social creative": "广告与社媒创意",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "生成活动概念图、缩略图、海报和本地化版本。",
    "Product visuals": "产品视觉",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "生成产品图、陈列场景和参考图引导的变体。",
    "Developer pipelines": "开发者流水线",
    "Add async generation for agents, CMS tools, and batch creative systems.": "为 Agent、CMS 工具和批量创意系统接入异步生成。",
    "{{model}} pricing FAQ": "{{model}} 价格 FAQ",
    "Generate your first {{model}} on Flatkey": "在 Flatkey 生成第一个 {{model}} 请求",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "先保存草稿；未登录会进入注册，已登录则直接打开控制台继续。",
    "Model Guide": "模型指南",
    "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} 是适合聊天、代码、长上下文推理和工具工作流的生产级文本模型，可通过 Flatkey 兼容 API 访问。",
    Vendor: "供应商",
    Text: "文本",
    Price: "价格",
    Updated: "更新日期",
    "Open in Playground": "在 Playground 打开",
    Docs: "文档",
    "Model Overview": "模型概览",
    "Quick Answer": "快速结论",
    "Best for chat, code generation, agent workflows, and production assistants.": "适合聊天、代码生成、Agent 工作流和生产级助手。",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "当你需要兼容 OpenAI 的路由、统一账单和可复用 API Key 时，可以使用 Flatkey。",
    "Start with the default parameters, then tune max tokens and temperature for your workload.": "先使用默认参数，再按任务调整 max_tokens 和 temperature。",
    "{{model}} Model Features": "{{model}} 模型能力",
    "Core capabilities and practical engineering value": "核心能力与工程价值",
    "How to Use {{model}} API": "如何使用 {{model}} API",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "创建 API Key，并设置 Authorization: Bearer <YOUR_API_KEY>。",
    "POST to /v1/chat/completions with at least model and messages.": "向 /v1/chat/completions 发起 POST 请求，至少包含 model 和 messages。",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "根据任务复杂度调整 max_tokens、temperature 和 top_p。",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "聊天 UI、终端助手和 Agent 工作流可以启用 streaming。",
    "Use logs and retries to refine prompts before broader rollout.": "扩量前通过日志和重试优化 prompt。",
    "Common Errors": "常见错误",
    "Missing required fields, malformed messages, or unsupported parameter values.": "缺少必填字段、messages 格式错误，或参数值不支持。",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "缺少 Authorization 头、Bearer token 格式错误，或 API Key 无效。",
    "Request rate, concurrency, or quota is above current account limits.": "请求速率、并发或额度超过当前账号限制。",
    "Transient upstream instability, tool execution failure, or processing issue.": "上游短暂不稳定、工具执行失败或处理异常。",
    "Ready to unify your AI model access?": "准备统一你的 AI 模型访问了吗？",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "用一个 Flatkey 账号测试 prompt、比较模型，并把保存的请求带入控制台。",
    "View Pricing": "查看价格",
    Prompt: "Prompt",
    "Quick Prompts": "快捷 Prompt",
    "Product Reveal": "产品展示",
    "UGC Ad": "UGC 广告",
    "Cinematic Scene": "电影感场景",
    "Social Clip": "社媒短片",
    "Product Photo": "产品摄影",
    "Anime Portrait": "动漫头像",
    "Realistic Human": "真人写实",
    "YouTube Thumbnail": "YouTube 缩略图",
    "Fantasy Landscape": "奇幻风景",
    "Advanced Options": "高级选项",
    "Reference Images": "参考图片",
    "Upload reference": "上传参考",
    "Example output": "示例输出",
    "Request preview": "请求预览",
    "Copy request": "复制请求",
    "Output format": "输出格式",
    Background: "背景",
    Moderation: "审核强度",
    Resolution: "分辨率",
    "Aspect ratio": "画面比例",
    Duration: "时长",
    Frames: "帧数",
    "Optional frame count override": "可选的帧数覆盖",
    "Camera fixed": "固定镜头",
    "Generate audio": "生成音频",
    "Return last frame": "返回尾帧",
    Seed: "随机种子",
    "0 means random": "0 表示随机",
    "OpenAI-compatible migration path": "兼容 OpenAI 的迁移路径",
    "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Chat Completions 风格请求体能降低从现有模型栈迁移的成本。",
    "Structured and tool-based output": "结构化与工具输出",
    "Use structured JSON, tools, and code-generation flows for agentic workflows.": "可在 Agent 工作流中使用结构化 JSON、工具和代码生成流程。",
    "Streaming interaction": "流式交互",
    "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "流式输出适合聊天 UI、终端助手和渐进式渲染。",
    "Production routing": "生产路由",
    "Keep usage, keys, quotas, and model routing in one Flatkey account.": "在一个 Flatkey 账号中管理用量、Key、额度和模型路由。",
    "Long-context work": "长上下文任务",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "适合文档总结、代码库分析和知识工作流。",
    "Coding and technical generation": "代码与技术生成",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "适合代码解释、测试、重构、SDK 封装和技术草稿。",
    "You pay": "你只付",
    "per month on the Go plan": "每月 · Go 套餐",
    "You get": "你获得",
    "of monthly model usage — 4.5× the price": "每月模型可用量 —— 套餐价的 4.5 倍",
    "from $10/month": "只需 $10/月起",
    "Pro — $30/mo, up to $90 usage": "Pro —— $30/月，可用 $90",
    "Most popular": "最受欢迎",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 每月 —— 可用量最高达套餐价 4.5 倍",
    "▶ Sign in to run": "▶ 登录即可运行",
    "Start generating": "开始生成",
    "Generator setup": "生成器配置",
    "Saved before signup": "注册前保存",
    "Public demo": "公开演示",
    Size: "尺寸",
    Quality: "质量",
    Outputs: "输出数量",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "在这里编辑 prompt 和参数。我们会先在本地保存草稿，然后打开 Flatkey，注册后即可运行。",
    "(flatkey · official ≈ {{price}})": "(flatkey · 官方 ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · 兼容 OpenAI · 一个密钥，全部模型",
    "* Illustrative pricing — see flatkey pricing page": "* 示例价格 — 详见 flatkey 定价页",
    "/ million output tokens": "/ 百万输出 token",
    "/ second": "/ 秒",
    "# Your existing OpenAI code:": "# 你现有的 OpenAI 代码：",
    "up to 50% off": "最低 5 折",
    "covers every model": "覆盖全部模型",
    "Est. this run": "本次预估",
    "One subscription": "一份订阅",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub 一键登录 · 无需信用卡即可开始",
    "migrate.py — change one line": "migrate.py — 改一行即可",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "文本·图像·视频一个套餐 · 超量按量计费 · 随时取消",
    "Playground (edit before sign-up)": "Playground（注册前可编辑）",
    "Pricing vs official": "与官方价格对比",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "同款 {{official}} 上游，同等质量 —— $10/月起的套餐包含全部前沿模型，每月可用量最高达套餐价的 4.5 倍。只改一行 base_url，现有 OpenAI SDK 直接可用。先在下方试用，准备好再登录。",
    "See plans →": "查看套餐 →",
    "Starter / individual": "入门 / 个人",
    "Team / high-volume": "团队 / 大用量",
    "The same {{model}},": "同样的 {{model}}，",
    "Go — $10/mo, up to $25 usage": "Go —— $10/月，可用 $25",
    "Max — $100/mo, up to $450 usage": "Max —— $100/月，可用 $450",
    "/ image": "/ 张图片",
    "Opus 4 output": "Opus 4 输出",
    "Sonnet 4 output": "Sonnet 4 输出",
    "Haiku output": "Haiku 输出",
    "GPT-5 output": "GPT-5 输出",
    "GPT-5 mini output": "GPT-5 mini 输出",
    "GPT-5 input": "GPT-5 输入",
    "Gemini 2.5 Pro output": "Gemini 2.5 Pro 输出",
    "Gemini 2.5 Flash output": "Gemini 2.5 Flash 输出",
    "Gemini 2.5 Pro input": "Gemini 2.5 Pro 输入",
    "GPT-image-2 image": "GPT-image-2 图片",
    "Square output": "方图输出",
    "Fast product mockups": "快速产品样机",
    "Seedance video / sec": "Seedance 视频/秒",
    "Image-to-video / sec": "图生视频/秒",
    "1080p / sec": "1080p/秒",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/秒",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/秒",
    "Reference video input": "参考视频输入",
    "Input image after free tier": "超出免费档的输入图片",
    "same per-second rate": "同输出秒价",
    "AIGC watermark": "AIGC 水印",
    "Cache reads": "缓存读取",
    Coverage: "覆盖范围",
    "AI app backends": "AI 应用后端",
    "Agent workflows": "Agent 工作流",
    "Batch content generation": "批量内容生成",
    "Best for general AI apps, agents, search, and high-volume API workloads": "适合通用 AI 应用、Agent、搜索和高用量 API 场景",
    "Best for long-context reasoning, coding agents, and production assistants": "适合长上下文推理、编程 Agent 和生产级助手",
    "Best for product videos, ad creative, and image-to-video production": "适合产品视频、广告创意和图生视频生产",
    "Can I control usage before scaling?": "扩量前可以控制用量吗？",
    "Coding agents": "编程 Agent",
    "Product mockups": "产品样机",
    "Ad creatives": "广告素材",
    "Ecommerce images": "电商图片",
    "Best for product images, ad creatives, and ecommerce visual variants": "适合产品图、广告素材和电商视觉变体",
    "Does this start a real generation?": "这里会真的开始生成吗？",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "公开页面会先保存你的草稿配置。注册或进入控制台后，再用 API 密钥真正运行请求。",
    "Which MiniMax-H3 fields can I configure here?": "这里可以配置哪些 MiniMax-H3 字段？",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "进入控制台前可以先配置分辨率、时长、画面比例和 AIGC 水印。",
    "Where are my edited prompt settings stored?": "我编辑过的 prompt 配置存在哪里？",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "会保存在当前浏览器的 localStorage 中，因此注册跳转后草稿仍可保留。",
    "Does this use the same model id in my SDK?": "我的 SDK 里还能用同一个模型 ID 吗？",
    "Live flatkey pricing": "flatkey 实时价格",
    "Live model data from pricing API": "来自定价 API 的实时模型数据",
    "Long document analysis": "长文档分析",
    "Matched live models": "匹配到的实时模型",
    "Product motion": "产品动态视频",
    "Social video variants": "社媒视频变体",
    "Support automation": "客服自动化",
    "UGC ad clips": "UGC 广告短片",
    "Yes. Keep your SDK and switch base_url plus api_key.": "可以。保留现有 SDK，只切换 base_url 和 api_key。",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "可以。套餐上限、用量分析和统一账单让支出可控。",
    "50% off": "5 折",
  },
  es: {
    "All models": "Todos los modelos",
    "Back to Models": "Volver a modelos",
    "View Pricing": "Ver precios",
    Docs: "Documentación",
    Rankings: "Rankings",
    "Related pages": "Páginas relacionadas",
    "Keep exploring Flatkey": "Seguir explorando Flatkey",
    "Related models": "Modelos relacionados",
    "More models from {{provider}}": "Más modelos de {{provider}}",
    "Swipe or scroll to compare": "Desliza o desplázate para comparar",
    "Previous example": "Ejemplo anterior",
    "Next example": "Ejemplo siguiente",
    "Example {{index}} of {{total}}": "Ejemplo {{index}} de {{total}}",
    "Generated with {{model}}": "Generado con {{model}}",
    "You pay": "Pagas",
    "per month on the Go plan": "al mes en el plan Go",
    "You get": "Recibes",
    "of monthly model usage — 4.5× the price": "de uso mensual de modelos — 4.5× el precio",
    "from $10/month": "desde $10/mes",
    "Pro — $30/mo, up to $90 usage": "Pro — $30/mes, hasta $90 de uso",
    "Most popular": "Más popular",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 al mes — uso de hasta 4.5× el precio",
    "▶ Sign in to run": "▶ Inicia sesión para ejecutar",
    "Start generating": "Empezar a generar",
    "Generator setup": "Configuración del generador",
    "Saved before signup": "Guardado antes del registro",
    "Public demo": "Demo pública",
    Size: "Tamaño",
    Quality: "Calidad",
    Outputs: "Salidas",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "Edita el prompt y los ajustes aquí. Guardamos el borrador localmente y luego abrimos Flatkey para que puedas ejecutarlo tras registrarte.",
    "(flatkey · official ≈ {{price}})": "(flatkey · oficial ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · compatible con OpenAI · una clave, todos los modelos",
    "* Illustrative pricing — see flatkey pricing page": "* Precios ilustrativos — consulta la página de precios de flatkey",
    "/ million output tokens": "/ millón de tokens de salida",
    "/ image": "/ imagen",
    "/ second": "/ segundo",
    "# Your existing OpenAI code:": "# Tu código OpenAI actual:",
    "up to 50% off": "hasta 50% menos",
    "covers every model": "cubre todos los modelos",
    "Est. this run": "Est. esta ejecución",
    "One subscription": "Una suscripción",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub con un clic · sin tarjeta para empezar",
    "migrate.py — change one line": "migrate.py — cambia una línea",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "Texto, imagen y vídeo en un plan · el exceso se cobra por uso · cancela cuando quieras",
    "Playground (edit before sign-up)": "Playground (edita antes de registrarte)",
    "Pricing vs official": "Precios vs oficial",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "Mismo upstream de {{official}}, misma calidad — los planes desde $10/mes incluyen todos los modelos frontera, con un uso mensual de hasta 4.5× lo que pagas. Cambia una línea de base_url y tu SDK de OpenAI existente simplemente funciona. Pruébalo abajo e inicia sesión cuando estés listo.",
    "See plans →": "Ver planes →",
    "Starter / individual": "Inicial / individual",
    "Team / high-volume": "Equipo / alto volumen",
    "The same {{model}},": "El mismo {{model}},",
    "Go — $10/mo, up to $25 usage": "Go — $10/mes, hasta $25 de uso",
    "Max — $100/mo, up to $450 usage": "Max — $100/mes, hasta $450 de uso",
    "Opus 4 output": "Salida de Opus 4",
    "Sonnet 4 output": "Salida de Sonnet 4",
    "Haiku output": "Salida de Haiku",
    "GPT-5 output": "Salida de GPT-5",
    "GPT-5 mini output": "Salida de GPT-5 mini",
    "GPT-5 input": "Entrada de GPT-5",
    "GPT-image-2 image": "Imagen GPT-image-2",
    "Square output": "Salida cuadrada",
    "Fast product mockups": "Mockups de producto rápidos",
    "Gemini 2.5 Pro output": "Salida de Gemini 2.5 Pro",
    "Gemini 2.5 Flash output": "Salida de Gemini 2.5 Flash",
    "Gemini 2.5 Pro input": "Entrada de Gemini 2.5 Pro",
    "Seedance video / sec": "Vídeo Seedance/seg",
    "Image-to-video / sec": "Imagen a vídeo/seg",
    "1080p / sec": "1080p/seg",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/seg",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/seg",
    "Reference video input": "Vídeo de referencia",
    "Input image after free tier": "Imagen de entrada tras el tramo gratis",
    "same per-second rate": "misma tarifa por segundo",
    "AIGC watermark": "Marca de agua AIGC",
    "Cache reads": "Lecturas de caché",
    Coverage: "Cobertura",
    "AI app backends": "Backends de apps de IA",
    "Agent workflows": "Flujos de agentes",
    "Batch content generation": "Generación de contenido por lotes",
    "Best for general AI apps, agents, search, and high-volume API workloads": "Ideal para apps de IA generales, agentes, búsqueda y cargas API de alto volumen",
    "Best for long-context reasoning, coding agents, and production assistants": "Ideal para razonamiento de contexto largo, agentes de código y asistentes en producción",
    "Best for product videos, ad creative, and image-to-video production": "Ideal para videos de producto, creatividades publicitarias y producción imagen-a-video",
    "Can I control usage before scaling?": "¿Puedo controlar el uso antes de escalar?",
    "Coding agents": "Agentes de código",
    "Product mockups": "Mockups de producto",
    "Ad creatives": "Creatividades publicitarias",
    "Ecommerce images": "Imágenes de ecommerce",
    "Best for product images, ad creatives, and ecommerce visual variants": "Ideal para imágenes de producto, creatividades publicitarias y variantes visuales de ecommerce",
    "Does this start a real generation?": "¿Esto inicia una generación real?",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "La página pública primero guarda tus ajustes de borrador. Regístrate o abre la consola para ejecutar la solicitud con una clave API.",
    "Which MiniMax-H3 fields can I configure here?": "¿Qué campos de MiniMax-H3 puedo configurar aquí?",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "Configura resolución, duración, relación de aspecto y marca de agua AIGC antes de abrir la consola.",
    "Where are my edited prompt settings stored?": "¿Dónde se guardan mis ajustes editados del prompt?",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "Se guardan en el localStorage de este navegador para que el borrador sobreviva al paso de registro.",
    "Does this use the same model id in my SDK?": "¿Uso el mismo id de modelo en mi SDK?",
    "Live flatkey pricing": "Precio en vivo de flatkey",
    "Live model data from pricing API": "Datos del modelo en vivo desde la API de precios",
    "Long document analysis": "Análisis de documentos largos",
    "Matched live models": "Modelos en vivo coincidentes",
    "Product motion": "Movimiento de producto",
    "Social video variants": "Variantes de video social",
    "Support automation": "Automatización de soporte",
    "UGC ad clips": "Clips publicitarios UGC",
    "Yes. Keep your SDK and switch base_url plus api_key.": "Sí. Mantén tu SDK y cambia base_url más api_key.",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Sí. Los límites del plan, la analítica de uso y una única factura mantienen el gasto acotado.",
    "50% off": "50% de descuento",
  },
  fr: {
    "All models": "Tous les modèles",
    "Back to Models": "Retour aux modèles",
    "View Pricing": "Voir les tarifs",
    Docs: "Documentation",
    Rankings: "Classements",
    "Related pages": "Pages associées",
    "Keep exploring Flatkey": "Continuer avec Flatkey",
    "Related models": "Modèles associés",
    "More models from {{provider}}": "Plus de modèles de {{provider}}",
    "Swipe or scroll to compare": "Faites défiler pour comparer",
    "Previous example": "Exemple précédent",
    "Next example": "Exemple suivant",
    "Example {{index}} of {{total}}": "Exemple {{index}} sur {{total}}",
    "Generated with {{model}}": "Généré avec {{model}}",
    "You pay": "Vous payez",
    "per month on the Go plan": "par mois avec le plan Go",
    "You get": "Vous recevez",
    "of monthly model usage — 4.5× the price": "d'usage mensuel des modèles — 4,5× le prix",
    "from $10/month": "dès $10/mois",
    "Pro — $30/mo, up to $90 usage": "Pro — $30/mois, jusqu'à $90 d'usage",
    "Most popular": "Le plus populaire",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 par mois — usage jusqu'à 4,5× le prix",
    "▶ Sign in to run": "▶ Connectez-vous pour exécuter",
    "Start generating": "Commencer la génération",
    "Generator setup": "Configuration du générateur",
    "Saved before signup": "Enregistré avant l'inscription",
    "Public demo": "Démo publique",
    Size: "Taille",
    Quality: "Qualité",
    Outputs: "Sorties",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "Modifiez le prompt et les paramètres ici. Nous enregistrons le brouillon localement, puis ouvrons Flatkey pour que vous puissiez l'exécuter après inscription.",
    "(flatkey · official ≈ {{price}})": "(flatkey · officiel ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · compatible OpenAI · une clé, tous les modèles",
    "* Illustrative pricing — see flatkey pricing page": "* Tarifs indicatifs — voir la page tarifs de flatkey",
    "/ million output tokens": "/ million de tokens de sortie",
    "/ image": "/ image",
    "/ second": "/ seconde",
    "# Your existing OpenAI code:": "# Votre code OpenAI actuel :",
    "up to 50% off": "jusqu'à -50 %",
    "covers every model": "couvre tous les modèles",
    "Est. this run": "Est. pour cette exécution",
    "One subscription": "Un seul abonnement",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub en un clic · sans carte bancaire pour commencer",
    "migrate.py — change one line": "migrate.py — changez une ligne",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "Texte, image et vidéo dans un plan · dépassement facturé à l'usage · annulable à tout moment",
    "Playground (edit before sign-up)": "Playground (modifiez avant l'inscription)",
    "Pricing vs official": "Tarifs vs officiel",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "Même upstream {{official}}, même qualité — les plans dès $10/mois incluent tous les modèles frontière, avec un usage mensuel valant jusqu'à 4,5× le prix. Changez une ligne de base_url et votre SDK OpenAI existant fonctionne tel quel. Essayez ci-dessous, connectez-vous quand vous êtes prêt.",
    "See plans →": "Voir les plans →",
    "Starter / individual": "Débutant / individuel",
    "Team / high-volume": "Équipe / gros volume",
    "The same {{model}},": "Le même {{model}},",
    "Go — $10/mo, up to $25 usage": "Go — $10/mois, jusqu'à $25 d'usage",
    "Max — $100/mo, up to $450 usage": "Max — $100/mois, jusqu'à $450 d'usage",
    "Opus 4 output": "Sortie Opus 4",
    "Sonnet 4 output": "Sortie Sonnet 4",
    "Haiku output": "Sortie Haiku",
    "GPT-5 output": "Sortie GPT-5",
    "GPT-5 mini output": "Sortie GPT-5 mini",
    "GPT-5 input": "Entrée GPT-5",
    "GPT-image-2 image": "Image GPT-image-2",
    "Square output": "Sortie carrée",
    "Fast product mockups": "Mockups produit rapides",
    "Gemini 2.5 Pro output": "Sortie Gemini 2.5 Pro",
    "Gemini 2.5 Flash output": "Sortie Gemini 2.5 Flash",
    "Gemini 2.5 Pro input": "Entrée Gemini 2.5 Pro",
    "Seedance video / sec": "Vidéo Seedance/s",
    "Image-to-video / sec": "Image vers vidéo/s",
    "1080p / sec": "1080p/s",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/s",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/s",
    "Reference video input": "Vidéo de référence",
    "Input image after free tier": "Image d'entrée après le palier gratuit",
    "same per-second rate": "même tarif par seconde",
    "AIGC watermark": "Filigrane AIGC",
    "Cache reads": "Lectures de cache",
    Coverage: "Couverture",
    "AI app backends": "Backends d'apps IA",
    "Agent workflows": "Workflows d'agents",
    "Batch content generation": "Génération de contenu par lot",
    "Best for general AI apps, agents, search, and high-volume API workloads": "Idéal pour apps IA généralistes, agents, recherche et charges API à fort volume",
    "Best for long-context reasoning, coding agents, and production assistants": "Idéal pour raisonnement long contexte, agents de code et assistants en production",
    "Best for product videos, ad creative, and image-to-video production": "Idéal pour vidéos produit, créations publicitaires et production image-vers-vidéo",
    "Can I control usage before scaling?": "Puis-je contrôler l'usage avant de passer à l'échelle ?",
    "Coding agents": "Agents de code",
    "Product mockups": "Mockups produit",
    "Ad creatives": "Créations publicitaires",
    "Ecommerce images": "Images e-commerce",
    "Best for product images, ad creatives, and ecommerce visual variants": "Idéal pour les images produit, les créations publicitaires et les variantes visuelles e-commerce",
    "Does this start a real generation?": "Cela lance-t-il une vraie génération ?",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "La page publique enregistre d'abord vos paramètres de brouillon. Inscrivez-vous ou ouvrez la console pour exécuter la requête avec une clé API.",
    "Which MiniMax-H3 fields can I configure here?": "Quels champs MiniMax-H3 puis-je configurer ici ?",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "Configurez la résolution, la durée, le ratio et le filigrane AIGC avant d'ouvrir la console.",
    "Where are my edited prompt settings stored?": "Où sont stockés mes paramètres de prompt modifiés ?",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "Ils sont stockés dans le localStorage de ce navigateur afin que le brouillon survive au passage par l'inscription.",
    "Does this use the same model id in my SDK?": "Puis-je garder le même id de modèle dans mon SDK ?",
    "Live flatkey pricing": "Tarifs flatkey en direct",
    "Live model data from pricing API": "Données modèle en direct depuis l'API tarifs",
    "Long document analysis": "Analyse de longs documents",
    "Matched live models": "Modèles en direct correspondants",
    "Product motion": "Animation produit",
    "Social video variants": "Variantes vidéo sociales",
    "Support automation": "Automatisation du support",
    "UGC ad clips": "Clips publicitaires UGC",
    "Yes. Keep your SDK and switch base_url plus api_key.": "Oui. Gardez votre SDK et changez base_url ainsi que api_key.",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Oui. Les limites du plan, l'analyse d'usage et une facture unique gardent les dépenses maîtrisées.",
    "50% off": "50% de réduction",
  },
  pt: {
    "All models": "Todos os modelos",
    "Back to Models": "Voltar aos modelos",
    "View Pricing": "Ver preços",
    Docs: "Documentação",
    Rankings: "Rankings",
    "Related pages": "Páginas relacionadas",
    "Keep exploring Flatkey": "Continue explorando o Flatkey",
    "Related models": "Modelos relacionados",
    "More models from {{provider}}": "Mais modelos da {{provider}}",
    "Swipe or scroll to compare": "Deslize ou role para comparar",
    "Previous example": "Exemplo anterior",
    "Next example": "Próximo exemplo",
    "Example {{index}} of {{total}}": "Exemplo {{index}} de {{total}}",
    "Generated with {{model}}": "Gerado com {{model}}",
    "You pay": "Você paga",
    "per month on the Go plan": "por mês no plano Go",
    "You get": "Você recebe",
    "of monthly model usage — 4.5× the price": "de uso mensal de modelos — 4,5× o preço",
    "from $10/month": "a partir de $10/mês",
    "Pro — $30/mo, up to $90 usage": "Pro — $30/mês, até $90 de uso",
    "Most popular": "Mais popular",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 por mês — uso de até 4,5× o preço",
    "▶ Sign in to run": "▶ Entrar para executar",
    "Start generating": "Começar a gerar",
    "Generator setup": "Configuração do gerador",
    "Saved before signup": "Salvo antes do cadastro",
    "Public demo": "Demo pública",
    Size: "Tamanho",
    Quality: "Qualidade",
    Outputs: "Saídas",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "Edite o prompt e as configurações aqui. Salvamos o rascunho localmente e depois abrimos o Flatkey para você executar após cadastrar.",
    "(flatkey · official ≈ {{price}})": "(flatkey · oficial ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · compatível com OpenAI · uma chave, todos os modelos",
    "* Illustrative pricing — see flatkey pricing page": "* Preços ilustrativos — veja a página de preços do flatkey",
    "/ million output tokens": "/ milhão de tokens de saída",
    "/ image": "/ imagem",
    "/ second": "/ segundo",
    "# Your existing OpenAI code:": "# Seu código OpenAI atual:",
    "up to 50% off": "até 50% de desconto",
    "covers every model": "cobre todos os modelos",
    "Est. this run": "Est. desta execução",
    "One subscription": "Uma assinatura",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub com um clique · sem cartão de crédito para começar",
    "migrate.py — change one line": "migrate.py — mude uma linha",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "Texto, imagem e vídeo em um plano · excedente cobrado por uso · cancele quando quiser",
    "Playground (edit before sign-up)": "Playground (edite antes de cadastrar)",
    "Pricing vs official": "Preços vs oficial",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "Mesmo upstream da {{official}}, mesma qualidade — planos a partir de $10/mês incluem todos os modelos de fronteira, com uso mensal de até 4,5× o preço. Mude uma linha de base_url e seu SDK OpenAI existente simplesmente funciona. Teste abaixo e entre quando estiver pronto.",
    "See plans →": "Ver planos →",
    "Starter / individual": "Inicial / individual",
    "Team / high-volume": "Equipe / alto volume",
    "The same {{model}},": "O mesmo {{model}},",
    "Go — $10/mo, up to $25 usage": "Go — $10/mês, até $25 de uso",
    "Max — $100/mo, up to $450 usage": "Max — $100/mês, até $450 de uso",
    "Opus 4 output": "Saída do Opus 4",
    "Sonnet 4 output": "Saída do Sonnet 4",
    "Haiku output": "Saída do Haiku",
    "GPT-5 output": "Saída do GPT-5",
    "GPT-5 mini output": "Saída do GPT-5 mini",
    "GPT-5 input": "Entrada do GPT-5",
    "GPT-image-2 image": "Imagem GPT-image-2",
    "Square output": "Saída quadrada",
    "Fast product mockups": "Mockups de produto rápidos",
    "Gemini 2.5 Pro output": "Saída do Gemini 2.5 Pro",
    "Gemini 2.5 Flash output": "Saída do Gemini 2.5 Flash",
    "Gemini 2.5 Pro input": "Entrada do Gemini 2.5 Pro",
    "Seedance video / sec": "Vídeo Seedance/seg",
    "Image-to-video / sec": "Imagem-para-vídeo/seg",
    "1080p / sec": "1080p/seg",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/seg",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/seg",
    "Reference video input": "Vídeo de referência",
    "Input image after free tier": "Imagem de entrada após a faixa grátis",
    "same per-second rate": "mesma tarifa por segundo",
    "AIGC watermark": "Marca d'água AIGC",
    "Cache reads": "Leituras de cache",
    Coverage: "Cobertura",
    "AI app backends": "Backends de apps de IA",
    "Agent workflows": "Fluxos de agentes",
    "Batch content generation": "Geração de conteúdo em lote",
    "Best for general AI apps, agents, search, and high-volume API workloads": "Ideal para apps de IA, agentes, busca e cargas API de alto volume",
    "Best for long-context reasoning, coding agents, and production assistants": "Ideal para raciocínio de contexto longo, agentes de código e assistentes em produção",
    "Best for product videos, ad creative, and image-to-video production": "Ideal para vídeos de produto, criativos de anúncio e produção imagem-para-vídeo",
    "Can I control usage before scaling?": "Posso controlar o uso antes de escalar?",
    "Coding agents": "Agentes de código",
    "Product mockups": "Mockups de produto",
    "Ad creatives": "Criativos de anúncio",
    "Ecommerce images": "Imagens de ecommerce",
    "Best for product images, ad creatives, and ecommerce visual variants": "Ideal para imagens de produto, criativos de anúncio e variações visuais de ecommerce",
    "Does this start a real generation?": "Isso inicia uma geração real?",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "A página pública salva primeiro suas configurações de rascunho. Cadastre-se ou abra o console para executar a solicitação com uma chave API.",
    "Which MiniMax-H3 fields can I configure here?": "Quais campos do MiniMax-H3 posso configurar aqui?",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "Configure resolução, duração, proporção e marca d'água AIGC antes de abrir o console.",
    "Where are my edited prompt settings stored?": "Onde meus ajustes editados do prompt são armazenados?",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "Eles ficam no localStorage deste navegador para que o rascunho sobreviva ao fluxo de cadastro.",
    "Does this use the same model id in my SDK?": "Uso o mesmo id de modelo no meu SDK?",
    "Live flatkey pricing": "Preço em tempo real da flatkey",
    "Live model data from pricing API": "Dados do modelo em tempo real da API de preços",
    "Long document analysis": "Análise de documentos longos",
    "Matched live models": "Modelos em tempo real correspondentes",
    "Product motion": "Movimento de produto",
    "Social video variants": "Variações de vídeo social",
    "Support automation": "Automação de suporte",
    "UGC ad clips": "Clipes de anúncio UGC",
    "Yes. Keep your SDK and switch base_url plus api_key.": "Sim. Mantenha seu SDK e troque base_url e api_key.",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Sim. Limites do plano, análise de uso e uma única fatura mantêm o gasto sob controle.",
    "50% off": "50% de desconto",
  },
  ru: {
    "All models": "Все модели",
    "Back to Models": "Назад к моделям",
    "View Pricing": "Смотреть цены",
    Docs: "Документация",
    Rankings: "Рейтинги",
    "Related pages": "Связанные страницы",
    "Keep exploring Flatkey": "Продолжить изучать Flatkey",
    "Related models": "Похожие модели",
    "More models from {{provider}}": "Больше моделей от {{provider}}",
    "Swipe or scroll to compare": "Прокрутите, чтобы сравнить",
    "Previous example": "Предыдущий пример",
    "Next example": "Следующий пример",
    "Example {{index}} of {{total}}": "Пример {{index}} из {{total}}",
    "Generated with {{model}}": "Сгенерировано с {{model}}",
    "You pay": "Вы платите",
    "per month on the Go plan": "в месяц на плане Go",
    "You get": "Вы получаете",
    "of monthly model usage — 4.5× the price": "месячного использования моделей — 4,5× цены",
    "from $10/month": "от $10/мес",
    "Pro — $30/mo, up to $90 usage": "Pro — $30/мес, до $90 использования",
    "Most popular": "Самый популярный",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 в месяц — использование до 4,5× цены",
    "▶ Sign in to run": "▶ Войдите, чтобы запустить",
    "Start generating": "Начать генерацию",
    "Generator setup": "Настройки генератора",
    "Saved before signup": "Сохранено до регистрации",
    "Public demo": "Публичная демо-страница",
    Size: "Размер",
    Quality: "Качество",
    Outputs: "Выходы",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "Редактируйте prompt и параметры здесь. Мы сохраним черновик локально, затем откроем Flatkey, чтобы вы могли запустить его после регистрации.",
    "(flatkey · official ≈ {{price}})": "(flatkey · официальный ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · совместим с OpenAI · один ключ, все модели",
    "* Illustrative pricing — see flatkey pricing page": "* Ориентировочные цены — см. страницу тарифов flatkey",
    "/ million output tokens": "/ млн выходных токенов",
    "/ image": "/ изображение",
    "/ second": "/ секунду",
    "# Your existing OpenAI code:": "# Ваш текущий код OpenAI:",
    "up to 50% off": "до 50% дешевле",
    "covers every model": "покрывает все модели",
    "Est. this run": "Оценка за этот запуск",
    "One subscription": "Одна подписка",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub в один клик · без карты для старта",
    "migrate.py — change one line": "migrate.py — измените одну строку",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "Текст, изображения и видео в одном плане · сверх лимита — по факту · отмена в любой момент",
    "Playground (edit before sign-up)": "Playground (правьте до регистрации)",
    "Pricing vs official": "Цены против официальных",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "Тот же upstream {{official}}, то же качество — планы от $10/мес включают все передовые модели, с месячным использованием до 4,5× цены. Измените одну строку base_url — и ваш существующий OpenAI SDK просто работает. Попробуйте ниже, войдите, когда будете готовы.",
    "See plans →": "Смотреть планы →",
    "Starter / individual": "Начальный / индивидуальный",
    "Team / high-volume": "Команда / большой объём",
    "The same {{model}},": "Та же {{model}},",
    "Go — $10/mo, up to $25 usage": "Go — $10/мес, до $25 использования",
    "Max — $100/mo, up to $450 usage": "Max — $100/мес, до $450 использования",
    "Opus 4 output": "Вывод Opus 4",
    "Sonnet 4 output": "Вывод Sonnet 4",
    "Haiku output": "Вывод Haiku",
    "GPT-5 output": "Вывод GPT-5",
    "GPT-5 mini output": "Вывод GPT-5 mini",
    "GPT-5 input": "Ввод GPT-5",
    "GPT-image-2 image": "Изображение GPT-image-2",
    "Square output": "Квадратный вывод",
    "Fast product mockups": "Быстрые продуктовые макеты",
    "Gemini 2.5 Pro output": "Вывод Gemini 2.5 Pro",
    "Gemini 2.5 Flash output": "Вывод Gemini 2.5 Flash",
    "Gemini 2.5 Pro input": "Ввод Gemini 2.5 Pro",
    "Seedance video / sec": "Видео Seedance/сек",
    "Image-to-video / sec": "Изображение в видео/сек",
    "1080p / sec": "1080p/сек",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/сек",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/сек",
    "Reference video input": "Входное референс-видео",
    "Input image after free tier": "Входное изображение после бесплатного лимита",
    "same per-second rate": "та же посекундная ставка",
    "AIGC watermark": "Водяной знак AIGC",
    "Cache reads": "Чтения из кэша",
    Coverage: "Покрытие",
    "AI app backends": "Бэкенды AI-приложений",
    "Agent workflows": "Agent workflow",
    "Batch content generation": "Пакетная генерация контента",
    "Best for general AI apps, agents, search, and high-volume API workloads": "Подходит для AI-приложений, агентов, поиска и больших API-нагрузок",
    "Best for long-context reasoning, coding agents, and production assistants": "Подходит для длинного контекста, кодовых агентов и production-ассистентов",
    "Best for product videos, ad creative, and image-to-video production": "Подходит для продуктовых видео, рекламы и image-to-video производства",
    "Can I control usage before scaling?": "Можно ли контролировать расход до масштабирования?",
    "Coding agents": "Кодовые агенты",
    "Product mockups": "Продуктовые макеты",
    "Ad creatives": "Рекламные креативы",
    "Ecommerce images": "Изображения для e-commerce",
    "Best for product images, ad creatives, and ecommerce visual variants": "Подходит для продуктовых изображений, рекламных креативов и визуальных вариантов для e-commerce",
    "Does this start a real generation?": "Это запускает реальную генерацию?",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "Публичная страница сначала сохраняет настройки черновика. Зарегистрируйтесь или откройте консоль, чтобы запустить запрос с API-ключом.",
    "Which MiniMax-H3 fields can I configure here?": "Какие поля MiniMax-H3 можно настроить здесь?",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "Настройте разрешение, длительность, соотношение сторон и водяной знак AIGC перед открытием консоли.",
    "Where are my edited prompt settings stored?": "Где хранятся отредактированные настройки prompt?",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "Они хранятся в localStorage этого браузера, чтобы черновик сохранился при переходе к регистрации.",
    "Does this use the same model id in my SDK?": "Можно ли использовать тот же model id в SDK?",
    "Live flatkey pricing": "Актуальные цены flatkey",
    "Live model data from pricing API": "Живые данные модели из pricing API",
    "Long document analysis": "Анализ длинных документов",
    "Matched live models": "Найденные живые модели",
    "Product motion": "Product motion",
    "Social video variants": "Варианты видео для соцсетей",
    "Support automation": "Автоматизация поддержки",
    "UGC ad clips": "UGC рекламные клипы",
    "Yes. Keep your SDK and switch base_url plus api_key.": "Да. Оставьте SDK и смените base_url вместе с api_key.",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Да. Лимиты плана, аналитика использования и один счёт держат расходы под контролем.",
    "50% off": "скидка 50%",
  },
  ja: {
    "All models": "すべてのモデル",
    "Back to Models": "モデル一覧に戻る",
    "View Pricing": "料金を見る",
    Docs: "ドキュメント",
    Rankings: "ランキング",
    "Related pages": "関連ページ",
    "Keep exploring Flatkey": "Flatkey を続けて見る",
    "Related models": "関連モデル",
    "More models from {{provider}}": "{{provider}} の他のモデル",
    "Swipe or scroll to compare": "横にスクロールして比較",
    "Previous example": "前の例",
    "Next example": "次の例",
    "Example {{index}} of {{total}}": "例 {{index}} / {{total}}",
    "Generated with {{model}}": "{{model}} で生成",
    "You pay": "支払うのは",
    "per month on the Go plan": "／月（Go プラン）",
    "You get": "使えるのは",
    "of monthly model usage — 4.5× the price": "月間モデル利用枠——料金の 4.5 倍",
    "from $10/month": "月額 $10 から",
    "Pro — $30/mo, up to $90 usage": "Pro — $30/月、利用枠 $90",
    "Most popular": "一番人気",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 /月——利用枠は料金の最大 4.5 倍",
    "▶ Sign in to run": "▶ サインインして実行",
    "Start generating": "生成を開始",
    "Generator setup": "生成設定",
    "Saved before signup": "登録前に保存",
    "Public demo": "公開デモ",
    Size: "サイズ",
    Quality: "品質",
    Outputs: "出力数",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "ここでプロンプトと設定を編集します。下書きはローカルに保存され、その後 Flatkey を開いて登録後に実行できます。",
    "(flatkey · official ≈ {{price}})": "(flatkey · 公式 ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · OpenAI 互換 · 1つのキーで全モデル",
    "* Illustrative pricing — see flatkey pricing page": "* 参考価格 — flatkey の料金ページをご覧ください",
    "/ million output tokens": "/ 出力トークン100万あたり",
    "/ image": "/ 画像",
    "/ second": "/ 秒",
    "# Your existing OpenAI code:": "# 既存の OpenAI コード:",
    "up to 50% off": "最大 50% オフ",
    "covers every model": "全モデルをカバー",
    "Est. this run": "今回の概算",
    "One subscription": "1 つのサブスクで",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub ワンクリック · クレジットカード不要で開始",
    "migrate.py — change one line": "migrate.py — 1行変更するだけ",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "テキスト・画像・動画を 1 プランで · 超過分は従量課金 · いつでも解約可",
    "Playground (edit before sign-up)": "プレイグラウンド（登録前に編集可）",
    "Pricing vs official": "公式との価格比較",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "同じ {{official}} アップストリーム、同じ品質——月額 $10 からのプランで全フロンティアモデルが使え、月間利用枠は料金の最大 4.5 倍。base_url を 1 行変えるだけで、既存の OpenAI SDK がそのまま動きます。まず下で試して、準備ができたらログインを。",
    "See plans →": "プランを見る →",
    "Starter / individual": "スターター / 個人",
    "Team / high-volume": "チーム / 大量利用",
    "The same {{model}},": "同じ {{model}}、",
    "Go — $10/mo, up to $25 usage": "Go — $10/月、利用枠 $25",
    "Max — $100/mo, up to $450 usage": "Max — $100/月、利用枠 $450",
    "Opus 4 output": "Opus 4 出力",
    "Sonnet 4 output": "Sonnet 4 出力",
    "Haiku output": "Haiku 出力",
    "GPT-5 output": "GPT-5 出力",
    "GPT-5 mini output": "GPT-5 mini 出力",
    "GPT-5 input": "GPT-5 入力",
    "GPT-image-2 image": "GPT-image-2 画像",
    "Square output": "正方形出力",
    "Fast product mockups": "高速な商品モックアップ",
    "Gemini 2.5 Pro output": "Gemini 2.5 Pro 出力",
    "Gemini 2.5 Flash output": "Gemini 2.5 Flash 出力",
    "Gemini 2.5 Pro input": "Gemini 2.5 Pro 入力",
    "Seedance video / sec": "Seedance 動画/秒",
    "Image-to-video / sec": "画像から動画/秒",
    "1080p / sec": "1080p/秒",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/秒",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/秒",
    "Reference video input": "参照動画入力",
    "Input image after free tier": "無料枠後の入力画像",
    "same per-second rate": "同じ秒単価",
    "AIGC watermark": "AIGC ウォーターマーク",
    "Cache reads": "キャッシュ読み取り",
    Coverage: "対応モデル",
    "AI app backends": "AI アプリのバックエンド",
    "Agent workflows": "Agent ワークフロー",
    "Batch content generation": "一括コンテンツ生成",
    "Best for general AI apps, agents, search, and high-volume API workloads": "汎用 AI アプリ、Agent、検索、高負荷 API ワークロードに最適",
    "Best for long-context reasoning, coding agents, and production assistants": "長文脈推論、コーディング Agent、本番アシスタントに最適",
    "Best for product videos, ad creative, and image-to-video production": "商品動画、広告クリエイティブ、画像から動画制作に最適",
    "Can I control usage before scaling?": "拡張前に使用量を管理できますか？",
    "Coding agents": "コーディング Agent",
    "Product mockups": "商品モックアップ",
    "Ad creatives": "広告クリエイティブ",
    "Ecommerce images": "EC 画像",
    "Best for product images, ad creatives, and ecommerce visual variants": "商品画像、広告クリエイティブ、EC 向けビジュアルバリエーションに最適",
    "Does this start a real generation?": "ここで実際に生成が始まりますか？",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "公開ページではまず下書き設定を保存します。登録するかコンソールを開き、API キーでリクエストを実行してください。",
    "Which MiniMax-H3 fields can I configure here?": "ここで設定できる MiniMax-H3 の項目は何ですか？",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "コンソールを開く前に、解像度、長さ、アスペクト比、AIGC ウォーターマークを設定できます。",
    "Where are my edited prompt settings stored?": "編集したプロンプト設定はどこに保存されますか？",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "このブラウザの localStorage に保存されるため、登録への受け渡し後も下書きが残ります。",
    "Does this use the same model id in my SDK?": "SDK で同じモデル ID を使えますか？",
    "Live flatkey pricing": "flatkey のライブ料金",
    "Live model data from pricing API": "料金 API からのライブモデルデータ",
    "Long document analysis": "長文書分析",
    "Matched live models": "一致したライブモデル",
    "Product motion": "商品モーション",
    "Social video variants": "SNS 動画バリエーション",
    "Support automation": "サポート自動化",
    "UGC ad clips": "UGC 広告クリップ",
    "Yes. Keep your SDK and switch base_url plus api_key.": "はい。SDK はそのまま、base_url と api_key だけ変更します。",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "はい。プラン上限・利用分析・一括請求で支出を抑えられます。",
    "50% off": "50% オフ",
  },
  vi: {
    "All models": "Tất cả mô hình",
    "Back to Models": "Quay lại danh sách mô hình",
    "View Pricing": "Xem giá",
    Docs: "Tài liệu",
    Rankings: "Xếp hạng",
    "Related pages": "Trang liên quan",
    "Keep exploring Flatkey": "Tiếp tục khám phá Flatkey",
    "Related models": "Mô hình liên quan",
    "More models from {{provider}}": "Thêm mô hình từ {{provider}}",
    "Swipe or scroll to compare": "Vuốt hoặc cuộn để so sánh",
    "Previous example": "Ví dụ trước",
    "Next example": "Ví dụ tiếp theo",
    "Example {{index}} of {{total}}": "Ví dụ {{index}} / {{total}}",
    "Generated with {{model}}": "Tạo bằng {{model}}",
    "You pay": "Bạn trả",
    "per month on the Go plan": "mỗi tháng với gói Go",
    "You get": "Bạn nhận",
    "of monthly model usage — 4.5× the price": "mức dùng model hằng tháng — 4,5× giá",
    "from $10/month": "từ $10/tháng",
    "Pro — $30/mo, up to $90 usage": "Pro — $30/tháng, dùng tới $90",
    "Most popular": "Phổ biến nhất",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 mỗi tháng — mức dùng tới 4,5× giá",
    "▶ Sign in to run": "▶ Đăng nhập để chạy",
    "Start generating": "Bắt đầu tạo",
    "Generator setup": "Thiết lập trình tạo",
    "Saved before signup": "Đã lưu trước khi đăng ký",
    "Public demo": "Demo công khai",
    Size: "Kích thước",
    Quality: "Chất lượng",
    Outputs: "Số đầu ra",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "Chỉnh prompt và cài đặt tại đây. Chúng tôi lưu bản nháp cục bộ rồi mở Flatkey để bạn chạy sau khi đăng ký.",
    "(flatkey · official ≈ {{price}})": "(flatkey · chính thức ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · tương thích OpenAI · một khóa, mọi mô hình",
    "* Illustrative pricing — see flatkey pricing page": "* Giá minh họa — xem trang giá của flatkey",
    "/ million output tokens": "/ triệu token đầu ra",
    "/ image": "/ ảnh",
    "/ second": "/ giây",
    "# Your existing OpenAI code:": "# Mã OpenAI hiện có của bạn:",
    "up to 50% off": "rẻ hơn tới 50%",
    "covers every model": "bao trọn mọi model",
    "Est. this run": "Ước tính lần chạy này",
    "One subscription": "Một gói thuê bao",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub một chạm · không cần thẻ tín dụng để bắt đầu",
    "migrate.py — change one line": "migrate.py — đổi một dòng",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "Văn bản, ảnh và video trong một gói · vượt hạn mức tính theo dùng · hủy bất cứ lúc nào",
    "Playground (edit before sign-up)": "Playground (chỉnh sửa trước khi đăng ký)",
    "Pricing vs official": "Giá so với chính thức",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "Cùng upstream {{official}}, cùng chất lượng — các gói từ $10/tháng bao gồm mọi model tiên phong, với mức dùng hằng tháng lên tới 4,5× giá gói. Đổi một dòng base_url và SDK OpenAI hiện có của bạn chạy ngay. Thử bên dưới, đăng nhập khi sẵn sàng.",
    "See plans →": "Xem các gói →",
    "Starter / individual": "Khởi đầu / cá nhân",
    "Team / high-volume": "Nhóm / khối lượng lớn",
    "The same {{model}},": "Cùng {{model}},",
    "Go — $10/mo, up to $25 usage": "Go — $10/tháng, dùng tới $25",
    "Max — $100/mo, up to $450 usage": "Max — $100/tháng, dùng tới $450",
    "Opus 4 output": "Đầu ra Opus 4",
    "Sonnet 4 output": "Đầu ra Sonnet 4",
    "Haiku output": "Đầu ra Haiku",
    "GPT-5 output": "Đầu ra GPT-5",
    "GPT-5 mini output": "Đầu ra GPT-5 mini",
    "GPT-5 input": "Đầu vào GPT-5",
    "GPT-image-2 image": "Ảnh GPT-image-2",
    "Square output": "Đầu ra vuông",
    "Fast product mockups": "Mockup sản phẩm nhanh",
    "Gemini 2.5 Pro output": "Đầu ra Gemini 2.5 Pro",
    "Gemini 2.5 Flash output": "Đầu ra Gemini 2.5 Flash",
    "Gemini 2.5 Pro input": "Đầu vào Gemini 2.5 Pro",
    "Seedance video / sec": "Video Seedance/giây",
    "Image-to-video / sec": "Ảnh thành video/giây",
    "1080p / sec": "1080p/giây",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/giây",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/giây",
    "Reference video input": "Video tham chiếu đầu vào",
    "Input image after free tier": "Ảnh đầu vào sau mức miễn phí",
    "same per-second rate": "cùng đơn giá mỗi giây",
    "AIGC watermark": "Watermark AIGC",
    "Cache reads": "Đọc bộ nhớ đệm",
    Coverage: "Phạm vi hỗ trợ",
    "AI app backends": "Backend ứng dụng AI",
    "Agent workflows": "Quy trình agent",
    "Batch content generation": "Tạo nội dung hàng loạt",
    "Best for general AI apps, agents, search, and high-volume API workloads": "Phù hợp cho ứng dụng AI phổ thông, agent, tìm kiếm và tải API lớn",
    "Best for long-context reasoning, coding agents, and production assistants": "Phù hợp cho suy luận ngữ cảnh dài, agent lập trình và trợ lý production",
    "Best for product videos, ad creative, and image-to-video production": "Phù hợp cho video sản phẩm, quảng cáo và sản xuất ảnh-thành-video",
    "Can I control usage before scaling?": "Tôi có thể kiểm soát mức dùng trước khi mở rộng không?",
    "Coding agents": "Agent lập trình",
    "Product mockups": "Mockup sản phẩm",
    "Ad creatives": "Creative quảng cáo",
    "Ecommerce images": "Ảnh thương mại điện tử",
    "Best for product images, ad creatives, and ecommerce visual variants": "Phù hợp cho ảnh sản phẩm, creative quảng cáo và biến thể hình ảnh thương mại điện tử",
    "Does this start a real generation?": "Thao tác này có bắt đầu tạo thật không?",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "Trang công khai trước hết lưu cấu hình bản nháp. Đăng ký hoặc mở console để chạy request bằng API key.",
    "Which MiniMax-H3 fields can I configure here?": "Tôi có thể cấu hình trường MiniMax-H3 nào ở đây?",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "Cấu hình độ phân giải, thời lượng, tỷ lệ khung hình và watermark AIGC trước khi mở console.",
    "Where are my edited prompt settings stored?": "Cài đặt prompt đã chỉnh sửa được lưu ở đâu?",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "Chúng được lưu trong localStorage của trình duyệt này để bản nháp vẫn còn sau bước đăng ký.",
    "Does this use the same model id in my SDK?": "SDK của tôi có dùng cùng model id không?",
    "Live flatkey pricing": "Giá flatkey trực tiếp",
    "Live model data from pricing API": "Dữ liệu mô hình trực tiếp từ API giá",
    "Long document analysis": "Phân tích tài liệu dài",
    "Matched live models": "Mô hình trực tiếp đã khớp",
    "Product motion": "Chuyển động sản phẩm",
    "Social video variants": "Biến thể video mạng xã hội",
    "Support automation": "Tự động hóa hỗ trợ",
    "UGC ad clips": "Clip quảng cáo UGC",
    "Yes. Keep your SDK and switch base_url plus api_key.": "Có. Giữ SDK, chỉ đổi base_url và api_key.",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Có. Hạn mức gói, phân tích mức dùng và một hóa đơn duy nhất giữ chi tiêu trong tầm kiểm soát.",
    "50% off": "giảm 50%",
  },
  de: {
    "All models": "Alle Modelle",
    "Back to Models": "Zurück zu den Modellen",
    "View Pricing": "Preise ansehen",
    Docs: "Dokumentation",
    Rankings: "Rankings",
    "Related pages": "Verwandte Seiten",
    "Keep exploring Flatkey": "Flatkey weiter erkunden",
    "Related models": "Verwandte Modelle",
    "More models from {{provider}}": "Weitere Modelle von {{provider}}",
    "Swipe or scroll to compare": "Zum Vergleichen wischen oder scrollen",
    "Previous example": "Vorheriges Beispiel",
    "Next example": "Nächstes Beispiel",
    "Example {{index}} of {{total}}": "Beispiel {{index}} von {{total}}",
    "Generated with {{model}}": "Generiert mit {{model}}",
    "You pay": "Sie zahlen",
    "per month on the Go plan": "pro Monat im Go-Plan",
    "You get": "Sie erhalten",
    "of monthly model usage — 4.5× the price": "monatliche Modellnutzung — das 4,5-Fache des Preises",
    "from $10/month": "ab $10/Monat",
    "Pro — $30/mo, up to $90 usage": "Pro — $30/Monat, bis zu $90 Nutzung",
    "Most popular": "Am beliebtesten",
    "↓ Go $10 · Pro $30 · Max $100 per month — usage worth up to 4.5× the price": "↓ Go $10 · Pro $30 · Max $100 pro Monat — Nutzung bis zum 4,5-Fachen des Preises",
    "▶ Sign in to run": "▶ Zum Ausführen anmelden",
    "Start generating": "Generierung starten",
    "Generator setup": "Generator-Einstellungen",
    "Saved before signup": "Vor der Anmeldung gespeichert",
    "Public demo": "Öffentliche Demo",
    Size: "Größe",
    Quality: "Qualität",
    Outputs: "Ausgaben",
    "Edit the prompt and settings here. We save the draft locally, then open Flatkey so you can run it after signup.": "Bearbeiten Sie Prompt und Einstellungen hier. Wir speichern den Entwurf lokal und öffnen dann Flatkey, damit Sie ihn nach der Anmeldung ausführen können.",
    "(flatkey · official ≈ {{price}})": "(flatkey · offiziell ≈ {{price}})",
    "{{model}} · OpenAI-compatible · one key, all models": "{{model}} · OpenAI-kompatibel · ein Schlüssel, alle Modelle",
    "* Illustrative pricing — see flatkey pricing page": "* Beispielpreise — siehe flatkey-Preisseite",
    "/ million output tokens": "/ Million Output-Tokens",
    "/ image": "/ Bild",
    "/ second": "/ Sekunde",
    "# Your existing OpenAI code:": "# Dein vorhandener OpenAI-Code:",
    "up to 50% off": "bis zu 50% günstiger",
    "covers every model": "deckt alle Modelle ab",
    "Est. this run": "Schätzung für diesen Lauf",
    "One subscription": "Ein Abo",
    "Google / GitHub one-click · no credit card to start": "Google / GitHub mit einem Klick · keine Kreditkarte nötig zum Start",
    "migrate.py — change one line": "migrate.py — eine Zeile ändern",
    "Text, image and video in one plan · overage billed as you go · cancel anytime": "Text, Bild und Video in einem Plan · Mehrverbrauch nach Verbrauch · jederzeit kündbar",
    "Playground (edit before sign-up)": "Playground (vor der Anmeldung bearbeiten)",
    "Pricing vs official": "Preise im Vergleich zum offiziellen Anbieter",
    "Same {{official}} upstream, same quality — plans from $10/month include every frontier model, with monthly usage worth up to 4.5× the price. Change one line of base_url and your existing OpenAI SDK just works. Try it below, sign in when you are ready.": "Gleicher {{official}}-Upstream, gleiche Qualität — Pläne ab $10/Monat enthalten alle Frontier-Modelle, mit monatlichem Nutzungswert bis zum 4,5-Fachen des Preises. Ändern Sie eine Zeile base_url und Ihr bestehendes OpenAI SDK funktioniert einfach. Unten ausprobieren, anmelden, wenn Sie bereit sind.",
    "See plans →": "Pläne ansehen →",
    "Starter / individual": "Starter / Einzelperson",
    "Team / high-volume": "Team / hohes Volumen",
    "The same {{model}},": "Das gleiche {{model}},",
    "Go — $10/mo, up to $25 usage": "Go — $10/Monat, bis zu $25 Nutzung",
    "Max — $100/mo, up to $450 usage": "Max — $100/Monat, bis zu $450 Nutzung",
    "Opus 4 output": "Opus 4 Output",
    "Sonnet 4 output": "Sonnet 4 Output",
    "Haiku output": "Haiku Output",
    "GPT-5 output": "GPT-5 Output",
    "GPT-5 mini output": "GPT-5 mini Output",
    "GPT-5 input": "GPT-5 Input",
    "GPT-image-2 image": "GPT-image-2-Bild",
    "Square output": "Quadratische Ausgabe",
    "Fast product mockups": "Schnelle Produkt-Mockups",
    "Gemini 2.5 Pro output": "Gemini 2.5 Pro Output",
    "Gemini 2.5 Flash output": "Gemini 2.5 Flash Output",
    "Gemini 2.5 Pro input": "Gemini 2.5 Pro Input",
    "Seedance video / sec": "Seedance-Video/Sek.",
    "Image-to-video / sec": "Bild-zu-Video/Sek.",
    "1080p / sec": "1080p/Sek.",
    "MiniMax-H3 768P / sec": "MiniMax-H3 768P/Sek.",
    "MiniMax-H3 2K / sec": "MiniMax-H3 2K/Sek.",
    "Reference video input": "Referenzvideo-Eingabe",
    "Input image after free tier": "Eingabebild nach dem kostenlosen Kontingent",
    "same per-second rate": "gleicher Sekundenpreis",
    "AIGC watermark": "AIGC-Wasserzeichen",
    "Cache reads": "Cache-Lesevorgänge",
    Coverage: "Abdeckung",
    "AI app backends": "Backends für AI-Apps",
    "Agent workflows": "Agent-Workflows",
    "Batch content generation": "Batch-Content-Erstellung",
    "Best for general AI apps, agents, search, and high-volume API workloads": "Ideal für allgemeine AI-Apps, Agents, Suche und API-Workloads mit hohem Volumen",
    "Best for long-context reasoning, coding agents, and production assistants": "Ideal für Long-Context-Reasoning, Coding-Agents und Produktionsassistenten",
    "Best for product videos, ad creative, and image-to-video production": "Ideal für Produktvideos, Anzeigen-Creatives und Bild-zu-Video-Produktion",
    "Can I control usage before scaling?": "Kann ich die Nutzung vor dem Skalieren kontrollieren?",
    "Coding agents": "Coding-Agents",
    "Product mockups": "Produkt-Mockups",
    "Ad creatives": "Anzeigen-Creatives",
    "Ecommerce images": "E-Commerce-Bilder",
    "Best for product images, ad creatives, and ecommerce visual variants": "Ideal für Produktbilder, Anzeigen-Creatives und visuelle Varianten für E-Commerce",
    "Does this start a real generation?": "Startet das eine echte Generierung?",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "Die öffentliche Seite speichert zuerst Ihre Entwurfseinstellungen. Registrieren Sie sich oder öffnen Sie die Konsole, um die Anfrage mit einem API-Schlüssel auszuführen.",
    "Which MiniMax-H3 fields can I configure here?": "Welche MiniMax-H3-Felder kann ich hier konfigurieren?",
    "Configure resolution, duration, ratio, and AIGC watermark before opening the console.": "Konfigurieren Sie Auflösung, Dauer, Seitenverhältnis und AIGC-Wasserzeichen, bevor Sie die Konsole öffnen.",
    "Where are my edited prompt settings stored?": "Wo werden meine bearbeiteten Prompt-Einstellungen gespeichert?",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "Sie werden im localStorage dieses Browsers gespeichert, damit der Entwurf die Anmeldung übersteht.",
    "Does this use the same model id in my SDK?": "Nutze ich dieselbe Modell-ID in meinem SDK?",
    "Live flatkey pricing": "Live-Preise von flatkey",
    "Live model data from pricing API": "Live-Modelldaten aus der Pricing API",
    "Long document analysis": "Analyse langer Dokumente",
    "Matched live models": "Passende Live-Modelle",
    "Product motion": "Produktbewegung",
    "Social video variants": "Varianten für Social Videos",
    "Support automation": "Support-Automatisierung",
    "UGC ad clips": "UGC-Anzeigenclips",
    "Yes. Keep your SDK and switch base_url plus api_key.": "Ja. Behalte dein SDK und ändere base_url plus api_key.",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Ja. Planlimits, Nutzungsanalysen und eine Rechnung halten die Ausgaben im Rahmen.",
    "50% off": "50% Rabatt",
  },
});

/* Prototype-only labels that are shared by the refreshed model detail shell.
 * Keep these locale-specific so a newly added visual surface does not silently
 * fall back to English on localized model pages. */
const modelDetailCommonCopy: Partial<Record<Locale, Record<string, string>>> = {
  zh: {
    Endpoint: "Endpoint",
    "Aspect ratio": "画面比例",
    On: "开",
    Off: "关",
    Prompt: "提示词",
    Images: "图片数量",
    "Video URL": "视频 URL",
    "Preserve speech": "保留人声",
    "Reference media": "参考媒体",
    None: "无",
    "Reliability over the last 30 days": "最近 30 天的可靠性",
    "Measured on real Flatkey traffic, with production monitoring.": "基于 Flatkey 真实流量测量，并持续进行生产监控。",
    "Routing across upstream channels": "上游通道路由",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "请求会分配到服务该模型的可用上游通道，页面上方展示近 30 天的可用性。",
    "Speech preservation": "人声保留",
    "Video to Audio": "视频转音频",
    "Audio variants": "音频变体",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "根据媒体感知的描述生成音乐、声音底床和精细音轨。",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "添加新的音频层时，保持重要对白和人声清晰可辨。",
    "Generate multiple delivery options for edits, localization, and production handoff.": "为剪辑、本地化和制作交付生成多个版本。",
    "Best for video soundtracks, speech-preserving edits, and audio production": "适合视频配乐、保留人声的编辑和音频制作",
    "Image editing": "图片编辑",
    "Reference-guided motion": "参考引导的运动",
    "Camera and pacing": "镜头与节奏",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "生成产品图、陈列场景和参考图引导的变体。",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "生成活动概念图、缩略图、海报和本地化版本。",
    "Refine existing images, swap details, and create consistent variations for each channel.": "优化现有图片、替换细节，并为每个渠道生成一致的变体。",
    "Add async generation for agents, CMS tools, and batch creative systems.": "为 Agent、CMS 工具和批量创意系统接入异步生成。",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "创建产品镜头、社媒短片和故事场景，并控制运动与构图。",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "使用图片、视频和音频参考，保持主体与创意方向一致。",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "在交给 API 前配置画面比例、分辨率、时长和声音。",
  },
  es: {
    Endpoint: "Endpoint",
    "Aspect ratio": "Relación de aspecto",
    On: "Activado",
    Off: "Desactivado",
    Prompt: "Prompt",
    Images: "Imágenes",
    "Video URL": "URL del vídeo",
    "Preserve speech": "Conservar el habla",
    "Reference media": "Medios de referencia",
    None: "Ninguno",
    "Reliability over the last 30 days": "Fiabilidad de los últimos 30 días",
    "Measured on real Flatkey traffic, with production monitoring.": "Medido con tráfico real de Flatkey y monitorización de producción.",
    "Routing across upstream channels": "Enrutamiento entre canales upstream",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Las solicitudes se distribuyen entre los canales disponibles para este modelo; arriba se muestra la disponibilidad de los últimos 30 días.",
    "Speech preservation": "Conservación del habla",
    "Video to Audio": "Vídeo a audio",
    "Audio variants": "Variantes de audio",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Crea música, fondos sonoros y pistas de audio pulidas a partir de una indicación consciente del medio.",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "Mantén inteligibles los diálogos y la voz importantes mientras añades una nueva capa de audio.",
    "Generate multiple delivery options for edits, localization, and production handoff.": "Genera varias opciones de entrega para edición, localización y traspaso a producción.",
    "Best for video soundtracks, speech-preserving edits, and audio production": "Ideal para bandas sonoras de vídeo, ediciones que conservan la voz y producción de audio",
    "Image editing": "Edición de imágenes",
    "Reference-guided motion": "Movimiento guiado por referencias",
    "Camera and pacing": "Cámara y ritmo",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Genera tomas de producto, escenas de merchandising y variaciones guiadas por referencias.",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "Produce conceptos de campaña, miniaturas, pósteres y variantes localizadas.",
    "Refine existing images, swap details, and create consistent variations for each channel.": "Refina imágenes existentes, cambia detalles y crea variaciones coherentes para cada canal.",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Añade generación asíncrona para agentes, herramientas CMS y sistemas creativos por lotes.",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "Crea tomas de producto, clips sociales y escenas narrativas con movimiento y encuadre controlados.",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "Usa referencias de imagen, vídeo y audio para mantener la coherencia del sujeto y la dirección creativa.",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Configura relación de aspecto, resolución, duración y sonido antes de enviar la solicitud a la API.",
  },
  fr: {
    Endpoint: "Endpoint",
    "Aspect ratio": "Format",
    On: "Activé",
    Off: "Désactivé",
    Prompt: "Prompt",
    Images: "Images",
    "Video URL": "URL vidéo",
    "Preserve speech": "Conserver la parole",
    "Reference media": "Média de référence",
    None: "Aucun",
    "Reliability over the last 30 days": "Fiabilité sur les 30 derniers jours",
    "Measured on real Flatkey traffic, with production monitoring.": "Mesuré sur le trafic réel de Flatkey, avec supervision en production.",
    "Routing across upstream channels": "Routage entre canaux amont",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Les requêtes sont réparties entre les canaux disponibles pour ce modèle ; la disponibilité des 30 derniers jours est affichée ci-dessus.",
    "Speech preservation": "Préservation de la parole",
    "Video to Audio": "Vidéo vers audio",
    "Audio variants": "Variantes audio",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Créez de la musique, des fonds sonores et des pistes audio soignées à partir d’un brief adapté au média.",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "Gardez les dialogues et la parole importants intelligibles tout en ajoutant une nouvelle couche audio.",
    "Generate multiple delivery options for edits, localization, and production handoff.": "Générez plusieurs options de livraison pour le montage, la localisation et le passage en production.",
    "Best for video soundtracks, speech-preserving edits, and audio production": "Idéal pour les bandes-son vidéo, le montage préservant la parole et la production audio",
    "Image editing": "Retouche d’image",
    "Reference-guided motion": "Mouvement guidé par référence",
    "Camera and pacing": "Caméra et rythme",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Générez des plans produit, des scènes de merchandising et des variantes guidées par référence.",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "Produisez des concepts de campagne, des vignettes, des affiches et des variantes localisées.",
    "Refine existing images, swap details, and create consistent variations for each channel.": "Affinez les images existantes, remplacez des détails et créez des variantes cohérentes pour chaque canal.",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Ajoutez la génération asynchrone pour les agents, outils CMS et systèmes créatifs par lots.",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "Créez des plans produit, clips sociaux et scènes narratives avec mouvement et cadrage contrôlés.",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "Utilisez des références image, vidéo et audio pour garder le sujet et la direction créative cohérents.",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Configurez le format, la résolution, la durée et le son avant de transmettre la requête à l’API.",
  },
  pt: {
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "Configure uma solicitação de {{model}} na página pública. A Flatkey salva o rascunho localmente e abre o console para que você possa executá-la com sua conta e chave de API.",
    "Model Type": "Tipo de modelo",
    Form: "Formulário",
    "Join and run": "Cadastre-se e execute",
    "View API Docs": "Ver documentação da API",
    "Avg. response time": "Tempo médio de resposta",
    "Ready for production": "Pronto para produção",
    "Generated Examples": "Exemplos gerados",
    "Explore what {{model}} can create": "Explore o que {{model}} pode criar",
    "Create with this model": "Criar com este modelo",
    "Transparent Pricing": "Preços transparentes",
    "Flatkey {{model}} usage pricing": "Preços de uso do {{model}} na Flatkey",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "Use o mesmo saldo e a mesma chave de API Flatkey para modelos de imagem, vídeo, áudio e texto.",
    "Open wallet": "Abrir carteira",
    "Start generating in three steps": "Comece a gerar em três etapas",
    "Try a prompt": "Teste um prompt",
    "Use the playground to validate quality and style fit.": "Use o Playground para validar a qualidade e a adequação do estilo.",
    "Create an API key": "Criar uma chave de API",
    "Sign up, open Dashboard, and create a token for this model.": "Cadastre-se, abra o Dashboard e crie um token para este modelo.",
    "Ship your workflow": "Publique seu fluxo",
    "Call the same endpoint, then top up credits as usage grows.": "Chame o mesmo endpoint e recarregue créditos conforme o uso crescer.",
    "Built for generation teams": "Feito para equipes de geração",
    "{{model}} pricing FAQ": "FAQ de preços do {{model}}",
    "Generate your first {{model}} on Flatkey": "Gere seu primeiro {{model}} na Flatkey",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "Salve o rascunho, cadastre-se se necessário ou abra o console diretamente se já estiver conectado.",
    "Model Guide": "Guia do modelo",
    Updated: "Atualizado",
    "Model Overview": "Visão geral do modelo",
    "Best for chat, code generation, agent workflows, and production assistants.": "Ideal para chat, geração de código, fluxos de agentes e assistentes de produção.",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "Use a Flatkey quando quiser roteamento compatível com OpenAI, cobrança unificada e chaves de API reutilizáveis.",
    "How to Use {{model}} API": "Como usar a API do {{model}}",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "Crie uma chave de API e defina Authorization: Bearer <YOUR_API_KEY>.",
    "POST to /v1/chat/completions with at least model and messages.": "Envie um POST para /v1/chat/completions com pelo menos model e messages.",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "Ajuste max_tokens, temperature e top_p conforme a complexidade da tarefa.",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "Ative o streaming para interfaces de chat, assistentes de terminal e fluxos de agentes.",
    "Use logs and retries to refine prompts before broader rollout.": "Use logs e novas tentativas para refinar prompts antes de ampliar a implantação.",
    "Common Errors": "Erros comuns",
    "Missing required fields, malformed messages, or unsupported parameter values.": "Campos obrigatórios ausentes, mensagens malformadas ou valores de parâmetros não suportados.",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "Cabeçalho Authorization ausente, token Bearer malformado ou chave de API inválida.",
    "Request rate, concurrency, or quota is above current account limits.": "A taxa de solicitações, a concorrência ou a cota ultrapassa os limites atuais da conta.",
    "Ready to unify your AI model access?": "Pronto para unificar o acesso aos seus modelos de IA?",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Use uma conta Flatkey para testar prompts, comparar modelos e levar a solicitação salva para o console.",
    Endpoint: "Endpoint",
    "Aspect ratio": "Proporção",
    On: "Ativado",
    Off: "Desativado",
    Prompt: "Prompt",
    Images: "Imagens",
    "Video URL": "URL do vídeo",
    "Preserve speech": "Preservar fala",
    "Reference media": "Mídia de referência",
    None: "Nenhum",
    "Reliability over the last 30 days": "Confiabilidade nos últimos 30 dias",
    "Measured on real Flatkey traffic, with production monitoring.": "Medido com tráfego real da Flatkey e monitoramento de produção.",
    "Routing across upstream channels": "Roteamento entre canais upstream",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "As solicitações são distribuídas entre os canais disponíveis para este modelo; a disponibilidade dos últimos 30 dias aparece acima.",
    "Speech preservation": "Preservação da fala",
    "Video to Audio": "Vídeo para áudio",
    "Audio variants": "Variantes de áudio",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Crie músicas, bases sonoras e faixas de áudio refinadas a partir de um briefing que considera a mídia.",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "Mantenha diálogos e falas importantes inteligíveis ao adicionar uma nova camada de áudio.",
    "Generate multiple delivery options for edits, localization, and production handoff.": "Gere várias opções de entrega para edição, localização e passagem à produção.",
    "Best for video soundtracks, speech-preserving edits, and audio production": "Ideal para trilhas de vídeo, edições que preservam a fala e produção de áudio",
    "Image editing": "Edição de imagens",
    "Reference-guided motion": "Movimento guiado por referência",
    "Camera and pacing": "Câmera e ritmo",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Gere imagens de produto, cenas de merchandising e variações guiadas por referências.",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "Produza conceitos de campanha, miniaturas, pôsteres e variantes localizadas.",
    "Refine existing images, swap details, and create consistent variations for each channel.": "Aprimore imagens existentes, troque detalhes e crie variações consistentes para cada canal.",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Adicione geração assíncrona para agentes, ferramentas CMS e sistemas criativos em lote.",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "Crie cenas de produto, clipes sociais e cenas narrativas com movimento e enquadramento controlados.",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "Use referências de imagem, vídeo e áudio para manter o sujeito e a direção criativa consistentes.",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Configure proporção, resolução, duração e som antes de enviar a solicitação à API.",
  },
  ru: {
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "Настройте запрос {{model}} на публичной странице. Flatkey сохранит черновик локально и откроет консоль, чтобы вы могли запустить его со своим аккаунтом и API-ключом.",
    "Model Type": "Тип модели",
    Form: "Форма",
    "Join and run": "Зарегистрироваться и запустить",
    "View API Docs": "Открыть документацию API",
    "Avg. response time": "Среднее время ответа",
    "Ready for production": "Готово для продакшена",
    "Generated Examples": "Сгенерированные примеры",
    "Explore what {{model}} can create": "Узнайте, что может создать {{model}}",
    "Create with this model": "Создать с этой моделью",
    "Transparent Pricing": "Прозрачные цены",
    "Flatkey {{model}} usage pricing": "Стоимость использования {{model}} в Flatkey",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "Используйте один баланс Flatkey и API-ключ для моделей изображений, видео, аудио и текста.",
    "Open wallet": "Открыть кошелёк",
    "Start generating in three steps": "Начать генерацию за три шага",
    "Try a prompt": "Попробовать промпт",
    "Use the playground to validate quality and style fit.": "Проверьте качество и соответствие стилю в Playground.",
    "Create an API key": "Создать API-ключ",
    "Sign up, open Dashboard, and create a token for this model.": "Зарегистрируйтесь, откройте Dashboard и создайте токен для этой модели.",
    "Ship your workflow": "Запустить рабочий процесс",
    "Call the same endpoint, then top up credits as usage grows.": "Вызывайте тот же endpoint и пополняйте кредит по мере роста использования.",
    "Built for generation teams": "Для команд генерации",
    "{{model}} pricing FAQ": "FAQ по стоимости {{model}}",
    "Generate your first {{model}} on Flatkey": "Создайте свой первый {{model}} в Flatkey",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "Сохраните черновик, зарегистрируйтесь при необходимости или сразу откройте консоль, если уже вошли.",
    "Model Guide": "Руководство по модели",
    Updated: "Обновлено",
    "Model Overview": "Обзор модели",
    "Best for chat, code generation, agent workflows, and production assistants.": "Подходит для чата, генерации кода, рабочих процессов агентов и продакшен-ассистентов.",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "Используйте Flatkey для маршрутизации, совместимой с OpenAI, единого биллинга и повторно используемых API-ключей.",
    "How to Use {{model}} API": "Как использовать API {{model}}",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "Создайте API-ключ и задайте Authorization: Bearer <YOUR_API_KEY>.",
    "POST to /v1/chat/completions with at least model and messages.": "Отправьте POST на /v1/chat/completions как минимум с полями model и messages.",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "Настройте max_tokens, temperature и top_p с учётом сложности задачи.",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "Включите streaming для чат-интерфейсов, терминальных ассистентов и рабочих процессов агентов.",
    "Use logs and retries to refine prompts before broader rollout.": "Используйте логи и повторные попытки, чтобы улучшить промпты до масштабного запуска.",
    "Common Errors": "Распространённые ошибки",
    "Missing required fields, malformed messages, or unsupported parameter values.": "Отсутствуют обязательные поля, сообщения имеют неверный формат или значения параметров не поддерживаются.",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "Отсутствует заголовок Authorization, токен Bearer имеет неверный формат или API-ключ недействителен.",
    "Request rate, concurrency, or quota is above current account limits.": "Частота запросов, параллелизм или квота превышают текущие лимиты аккаунта.",
    "Ready to unify your AI model access?": "Готовы объединить доступ к моделям ИИ?",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Используйте один аккаунт Flatkey, чтобы тестировать промпты, сравнивать модели и переносить сохранённый запрос в консоль.",
    Endpoint: "Эндпоинт",
    "Aspect ratio": "Соотношение сторон",
    On: "Вкл.",
    Off: "Выкл.",
    Prompt: "Промпт",
    Images: "Изображения",
    "Video URL": "URL видео",
    "Preserve speech": "Сохранять речь",
    "Reference media": "Медиа-референс",
    None: "Нет",
    "Reliability over the last 30 days": "Надёжность за последние 30 дней",
    "Measured on real Flatkey traffic, with production monitoring.": "Измерено на реальном трафике Flatkey при производственном мониторинге.",
    "Routing across upstream channels": "Маршрутизация через каналы провайдеров",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Запросы распределяются между доступными каналами этой модели; доступность за последние 30 дней показана выше.",
    "Speech preservation": "Сохранение речи",
    "Video to Audio": "Видео в аудио",
    "Audio variants": "Варианты аудио",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Создавайте музыку, звуковые подложки и готовые аудиодорожки по описанию с учётом медиаконтекста.",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "Сохраняйте разборчивость важных диалогов и речи при добавлении нового аудиослоя.",
    "Generate multiple delivery options for edits, localization, and production handoff.": "Создавайте несколько вариантов для монтажа, локализации и передачи в производство.",
    "Best for video soundtracks, speech-preserving edits, and audio production": "Подходит для саундтреков к видео, монтажа с сохранением речи и производства аудио",
    "Image editing": "Редактирование изображений",
    "Reference-guided motion": "Движение по референсам",
    "Camera and pacing": "Камера и темп",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Создавайте товарные кадры, мерчандайзинговые сцены и варианты по референсам.",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "Создавайте идеи кампаний, миниатюры, постеры и локализованные варианты.",
    "Refine existing images, swap details, and create consistent variations for each channel.": "Уточняйте существующие изображения, меняйте детали и создавайте согласованные варианты для каждого канала.",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Добавляйте асинхронную генерацию для агентов, CMS и пакетных креативных систем.",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "Создавайте товарные кадры, социальные клипы и сюжетные сцены с контролируемым движением и композицией.",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "Используйте референсы изображений, видео и аудио, чтобы сохранять единый объект и творческое направление.",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Настройте соотношение сторон, разрешение, длительность и звук перед передачей запроса в API.",
  },
  ja: {
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "公開ページで{{model}}リクエストを設定します。Flatkeyが下書きをローカルに保存し、コンソールを開くので、アカウントとAPIキーで実行できます。",
    "Model Type": "モデルタイプ",
    Form: "フォーム",
    "Join and run": "登録して実行",
    "View API Docs": "APIドキュメントを見る",
    "Avg. response time": "平均応答時間",
    "Ready for production": "本番環境に対応",
    "Generated Examples": "生成例",
    "Explore what {{model}} can create": "{{model}}で作成できるものを見る",
    "Create with this model": "このモデルで作成",
    "Transparent Pricing": "透明な料金",
    "Flatkey {{model}} usage pricing": "Flatkeyの{{model}}利用料金",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "画像・動画・音声・テキストモデルで同じFlatkey残高とAPIキーを使えます。",
    "Open wallet": "ウォレットを開く",
    "Start generating in three steps": "3ステップで生成を開始",
    "Try a prompt": "プロンプトを試す",
    "Use the playground to validate quality and style fit.": "Playgroundで品質とスタイルの適合性を確認します。",
    "Create an API key": "APIキーを作成",
    "Sign up, open Dashboard, and create a token for this model.": "登録してDashboardを開き、このモデル用のトークンを作成します。",
    "Ship your workflow": "ワークフローを本番投入",
    "Call the same endpoint, then top up credits as usage grows.": "同じエンドポイントを呼び出し、利用量に応じてクレジットを追加します。",
    "Built for generation teams": "生成チーム向け",
    "{{model}} pricing FAQ": "{{model}}の料金に関するFAQ",
    "Generate your first {{model}} on Flatkey": "Flatkeyで最初の{{model}}を生成",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "下書きを保存し、必要なら登録へ進み、ログイン済みならコンソールを直接開きます。",
    "Model Guide": "モデルガイド",
    Updated: "更新日",
    "Model Overview": "モデル概要",
    "Best for chat, code generation, agent workflows, and production assistants.": "チャット、コード生成、エージェントワークフロー、本番アシスタントに最適です。",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "OpenAI互換ルーティング、統合請求、再利用可能なAPIキーが必要ならFlatkeyを使用します。",
    "How to Use {{model}} API": "{{model}} APIの使い方",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "APIキーを作成し、Authorization: Bearer <YOUR_API_KEY>を設定します。",
    "POST to /v1/chat/completions with at least model and messages.": "/v1/chat/completionsへmodelとmessagesを含むPOSTを送信します。",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "タスクの複雑さに応じてmax_tokens、temperature、top_pを調整します。",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "チャットUI、ターミナルアシスタント、エージェントワークフローではストリーミングを有効にします。",
    "Use logs and retries to refine prompts before broader rollout.": "ログと再試行を使い、広範な展開の前にプロンプトを改善します。",
    "Common Errors": "よくあるエラー",
    "Missing required fields, malformed messages, or unsupported parameter values.": "必須フィールドの不足、メッセージ形式の誤り、またはサポートされていないパラメータ値です。",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "Authorizationヘッダーの不足、Bearerトークンの形式エラー、または無効なAPIキーです。",
    "Request rate, concurrency, or quota is above current account limits.": "リクエストレート、同時実行数、またはクォータが現在のアカウント上限を超えています。",
    "Ready to unify your AI model access?": "AIモデルへのアクセスを統合する準備はできましたか？",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Flatkeyの1つのアカウントでプロンプトを試し、モデルを比較し、保存したリクエストをコンソールへ移行できます。",
    Endpoint: "エンドポイント",
    "Aspect ratio": "アスペクト比",
    On: "オン",
    Off: "オフ",
    Prompt: "プロンプト",
    Images: "画像数",
    "Video URL": "動画 URL",
    "Preserve speech": "音声を保持",
    "Reference media": "参照メディア",
    None: "なし",
    "Reliability over the last 30 days": "過去30日間の信頼性",
    "Measured on real Flatkey traffic, with production monitoring.": "実際の Flatkey トラフィックを本番監視で計測しています。",
    "Routing across upstream channels": "上流チャネル間のルーティング",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "リクエストはこのモデルを提供する利用可能な上流チャネルに分散され、過去30日間の稼働率を上に表示しています。",
    "Speech preservation": "音声の保持",
    "Video to Audio": "動画から音声",
    "Audio variants": "音声バリエーション",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "メディアに合わせた指示から、音楽やサウンドベッド、仕上がりのよい音声トラックを作成します。",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "新しい音声レイヤーを加えながら、重要な会話や話し声を明瞭に保ちます。",
    "Generate multiple delivery options for edits, localization, and production handoff.": "編集、ローカライズ、制作引き継ぎ向けに複数の納品バリエーションを生成します。",
    "Best for video soundtracks, speech-preserving edits, and audio production": "動画のサウンドトラック、音声を保つ編集、音声制作に最適です",
    "Image editing": "画像編集",
    "Reference-guided motion": "参照ガイドのモーション",
    "Camera and pacing": "カメラとテンポ",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "商品カット、マーチャンダイジングシーン、参照画像に沿ったバリエーションを作成します。",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "キャンペーン案、サムネイル、ポスター、ローカライズ版を作成します。",
    "Refine existing images, swap details, and create consistent variations for each channel.": "既存画像を調整し、細部を差し替え、各チャネル向けに一貫したバリエーションを作成します。",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Agent、CMS ツール、バッチ制作システムに非同期生成を追加します。",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "動きと構図を制御した商品カット、ソーシャル動画、ストーリーシーンを作成します。",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "画像・動画・音声の参照を使い、被写体とクリエイティブの方向性を一貫させます。",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "API に渡す前に、比率、解像度、長さ、サウンドを設定します。",
  },
  vi: {
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "Định cấu hình yêu cầu {{model}} trên trang công khai. Flatkey lưu bản nháp cục bộ rồi mở console để bạn chạy bằng tài khoản và API key.",
    "Model Type": "Loại mô hình",
    Form: "Biểu mẫu",
    "Join and run": "Đăng ký và chạy",
    "View API Docs": "Xem tài liệu API",
    "Avg. response time": "Thời gian phản hồi trung bình",
    "Ready for production": "Sẵn sàng cho production",
    "Generated Examples": "Ví dụ đã tạo",
    "Explore what {{model}} can create": "Khám phá {{model}} có thể tạo gì",
    "Create with this model": "Tạo bằng mô hình này",
    "Transparent Pricing": "Giá minh bạch",
    "Flatkey {{model}} usage pricing": "Giá sử dụng {{model}} trên Flatkey",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "Dùng cùng số dư và API key Flatkey cho mô hình hình ảnh, video, âm thanh và văn bản.",
    "Open wallet": "Mở ví",
    "Start generating in three steps": "Bắt đầu tạo trong ba bước",
    "Try a prompt": "Thử prompt",
    "Use the playground to validate quality and style fit.": "Dùng playground để kiểm tra chất lượng và độ phù hợp phong cách.",
    "Create an API key": "Tạo API key",
    "Sign up, open Dashboard, and create a token for this model.": "Đăng ký, mở Dashboard và tạo token cho mô hình này.",
    "Ship your workflow": "Đưa workflow vào vận hành",
    "Call the same endpoint, then top up credits as usage grows.": "Gọi cùng endpoint rồi nạp thêm credit khi mức sử dụng tăng.",
    "Built for generation teams": "Dành cho đội ngũ tạo nội dung",
    "{{model}} pricing FAQ": "FAQ về giá {{model}}",
    "Generate your first {{model}} on Flatkey": "Tạo {{model}} đầu tiên trên Flatkey",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "Lưu bản nháp, đăng ký nếu cần hoặc mở console trực tiếp nếu bạn đã đăng nhập.",
    "Model Guide": "Hướng dẫn mô hình",
    Updated: "Đã cập nhật",
    "Model Overview": "Tổng quan mô hình",
    "Best for chat, code generation, agent workflows, and production assistants.": "Phù hợp cho chat, tạo mã, workflow agent và trợ lý production.",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "Dùng Flatkey khi cần định tuyến tương thích OpenAI, thanh toán hợp nhất và API key có thể tái sử dụng.",
    "How to Use {{model}} API": "Cách dùng API {{model}}",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "Tạo API key và đặt Authorization: Bearer <YOUR_API_KEY>.",
    "POST to /v1/chat/completions with at least model and messages.": "Gửi POST tới /v1/chat/completions với ít nhất model và messages.",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "Điều chỉnh max_tokens, temperature và top_p theo độ phức tạp của tác vụ.",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "Bật streaming cho giao diện chat, trợ lý terminal và workflow agent.",
    "Use logs and retries to refine prompts before broader rollout.": "Dùng nhật ký và thử lại để tinh chỉnh prompt trước khi triển khai rộng hơn.",
    "Common Errors": "Lỗi thường gặp",
    "Missing required fields, malformed messages, or unsupported parameter values.": "Thiếu trường bắt buộc, messages sai định dạng hoặc giá trị tham số không được hỗ trợ.",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "Thiếu header Authorization, token Bearer sai định dạng hoặc API key không hợp lệ.",
    "Request rate, concurrency, or quota is above current account limits.": "Tốc độ yêu cầu, mức đồng thời hoặc hạn ngạch vượt giới hạn tài khoản hiện tại.",
    "Ready to unify your AI model access?": "Bạn đã sẵn sàng hợp nhất quyền truy cập mô hình AI chưa?",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Dùng một tài khoản Flatkey để thử prompt, so sánh mô hình và chuyển yêu cầu đã lưu vào console.",
    Endpoint: "Endpoint",
    "Aspect ratio": "Tỷ lệ khung hình",
    On: "Bật",
    Off: "Tắt",
    Prompt: "Prompt",
    Images: "Số lượng ảnh",
    "Video URL": "URL video",
    "Preserve speech": "Giữ giọng nói",
    "Reference media": "Media tham chiếu",
    None: "Không có",
    "Reliability over the last 30 days": "Độ tin cậy trong 30 ngày qua",
    "Measured on real Flatkey traffic, with production monitoring.": "Được đo trên lưu lượng Flatkey thực tế cùng giám sát production.",
    "Routing across upstream channels": "Định tuyến qua các kênh upstream",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Yêu cầu được phân bổ qua các kênh khả dụng cho model này; độ khả dụng 30 ngày được hiển thị ở trên.",
    "Speech preservation": "Bảo toàn lời nói",
    "Video to Audio": "Video sang âm thanh",
    "Audio variants": "Biến thể âm thanh",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Tạo nhạc, nền âm thanh và các bản nhạc hoàn thiện từ mô tả phù hợp với nội dung media.",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "Giữ lời thoại và giọng nói quan trọng rõ ràng khi thêm lớp âm thanh mới.",
    "Generate multiple delivery options for edits, localization, and production handoff.": "Tạo nhiều phương án bàn giao cho biên tập, bản địa hóa và sản xuất.",
    "Best for video soundtracks, speech-preserving edits, and audio production": "Phù hợp cho nhạc nền video, biên tập giữ lời thoại và sản xuất âm thanh",
    "Image editing": "Chỉnh sửa hình ảnh",
    "Reference-guided motion": "Chuyển động theo tham chiếu",
    "Camera and pacing": "Góc máy và nhịp độ",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Tạo cảnh sản phẩm, cảnh trưng bày và biến thể theo hình ảnh tham chiếu.",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "Tạo ý tưởng chiến dịch, thumbnail, poster và biến thể bản địa hóa.",
    "Refine existing images, swap details, and create consistent variations for each channel.": "Tinh chỉnh ảnh có sẵn, thay đổi chi tiết và tạo biến thể nhất quán cho từng kênh.",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Thêm khả năng tạo bất đồng bộ cho agent, công cụ CMS và hệ thống sáng tạo theo lô.",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "Tạo cảnh sản phẩm, clip mạng xã hội và cảnh kể chuyện với chuyển động, khung hình được kiểm soát.",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "Dùng tham chiếu ảnh, video và âm thanh để giữ chủ thể và định hướng sáng tạo nhất quán.",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Cấu hình tỷ lệ, độ phân giải, thời lượng và âm thanh trước khi gửi yêu cầu đến API.",
  },
  de: {
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "Konfigurieren Sie eine {{model}}-Anfrage auf der öffentlichen Seite. Flatkey speichert den Entwurf lokal und öffnet die Konsole, damit Sie ihn mit Ihrem Konto und API-Schlüssel ausführen können.",
    "Model Type": "Modelltyp",
    Form: "Formular",
    "Join and run": "Registrieren und ausführen",
    "View API Docs": "API-Dokumentation anzeigen",
    "Avg. response time": "Durchschnittliche Antwortzeit",
    "Ready for production": "Bereit für den Produktionseinsatz",
    "Generated Examples": "Generierte Beispiele",
    "Explore what {{model}} can create": "Entdecken Sie, was {{model}} erstellen kann",
    "Create with this model": "Mit diesem Modell erstellen",
    "Transparent Pricing": "Transparente Preise",
    "Flatkey {{model}} usage pricing": "Flatkey-Nutzungspreise für {{model}}",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "Verwenden Sie dasselbe Flatkey-Guthaben und denselben API-Schlüssel für Bild-, Video-, Audio- und Textmodelle.",
    "Open wallet": "Wallet öffnen",
    "Start generating in three steps": "In drei Schritten starten",
    "Try a prompt": "Einen Prompt testen",
    "Use the playground to validate quality and style fit.": "Validieren Sie Qualität und Stileignung im Playground.",
    "Create an API key": "API-Schlüssel erstellen",
    "Sign up, open Dashboard, and create a token for this model.": "Registrieren Sie sich, öffnen Sie das Dashboard und erstellen Sie ein Token für dieses Modell.",
    "Ship your workflow": "Workflow ausliefern",
    "Call the same endpoint, then top up credits as usage grows.": "Rufen Sie denselben Endpunkt auf und laden Sie bei wachsender Nutzung Guthaben nach.",
    "Built for generation teams": "Für Generierungsteams entwickelt",
    "{{model}} pricing FAQ": "{{model}}-Preis-FAQ",
    "Generate your first {{model}} on Flatkey": "Ihren ersten {{model}} auf Flatkey generieren",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "Speichern Sie den Entwurf; registrieren Sie sich bei Bedarf oder öffnen Sie direkt die Konsole, wenn Sie bereits angemeldet sind.",
    "Model Guide": "Modellleitfaden",
    Updated: "Aktualisiert",
    "Model Overview": "Modellübersicht",
    "Best for chat, code generation, agent workflows, and production assistants.": "Ideal für Chat, Codegenerierung, Agenten-Workflows und produktive Assistenten.",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "Nutzen Sie Flatkey für OpenAI-kompatibles Routing, einheitliche Abrechnung und wiederverwendbare API-Schlüssel.",
    "How to Use {{model}} API": "{{model}}-API verwenden",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "Erstellen Sie einen API-Schlüssel und setzen Sie Authorization: Bearer <YOUR_API_KEY>.",
    "POST to /v1/chat/completions with at least model and messages.": "Senden Sie einen POST an /v1/chat/completions mit mindestens model und messages.",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "Passen Sie max_tokens, temperature und top_p an die Aufgabenkomplexität an.",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "Aktivieren Sie Streaming für Chat-Oberflächen, Terminal-Assistenten und Agenten-Workflows.",
    "Use logs and retries to refine prompts before broader rollout.": "Verfeinern Sie Prompts mit Logs und Wiederholungsversuchen vor einem breiteren Rollout.",
    "Common Errors": "Häufige Fehler",
    "Missing required fields, malformed messages, or unsupported parameter values.": "Pflichtfelder fehlen, Nachrichten sind fehlerhaft formatiert oder Parameterwerte werden nicht unterstützt.",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "Authorization-Header fehlt, Bearer-Token ist fehlerhaft formatiert oder der API-Schlüssel ist ungültig.",
    "Request rate, concurrency, or quota is above current account limits.": "Anfragerate, Parallelität oder Kontingent überschreiten die aktuellen Kontolimits.",
    "Ready to unify your AI model access?": "Bereit, Ihren Zugriff auf KI-Modelle zu vereinheitlichen?",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Nutzen Sie ein Flatkey-Konto, um Prompts zu testen, Modelle zu vergleichen und die gespeicherte Anfrage in die Konsole zu übernehmen.",
    Endpoint: "Endpunkt",
    "Aspect ratio": "Seitenverhältnis",
    On: "An",
    Off: "Aus",
    Prompt: "Prompt",
    Images: "Bilder",
    "Video URL": "Video-URL",
    "Preserve speech": "Sprache erhalten",
    "Reference media": "Referenzmedien",
    None: "Keine",
    "Reliability over the last 30 days": "Zuverlässigkeit der letzten 30 Tage",
    "Measured on real Flatkey traffic, with production monitoring.": "Auf echtem Flatkey-Traffic mit Produktionsmonitoring gemessen.",
    "Routing across upstream channels": "Routing über Upstream-Kanäle",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Anfragen werden auf verfügbare Kanäle für dieses Modell verteilt; die Verfügbarkeit der letzten 30 Tage steht oben.",
    "Speech preservation": "Spracherhalt",
    "Video to Audio": "Video zu Audio",
    "Audio variants": "Audio-Varianten",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Erstellen Sie Musik, Klangbetten und ausgearbeitete Audiospuren aus einem medienbewussten Briefing.",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "Halten Sie wichtige Dialoge und Sprache verständlich, während Sie eine neue Audioschicht hinzufügen.",
    "Generate multiple delivery options for edits, localization, and production handoff.": "Erstellen Sie mehrere Ausgabeoptionen für Schnitt, Lokalisierung und Produktionsübergabe.",
    "Best for video soundtracks, speech-preserving edits, and audio production": "Ideal für Video-Soundtracks, sprachbewahrenden Schnitt und Audioproduktion",
    "Image editing": "Bildbearbeitung",
    "Reference-guided motion": "Referenzgesteuerte Bewegung",
    "Camera and pacing": "Kamera und Tempo",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Erstellen Sie Produktaufnahmen, Merchandising-Szenen und referenzgesteuerte Varianten.",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "Erstellen Sie Kampagnenkonzepte, Vorschaubilder, Poster und lokalisierte Varianten.",
    "Refine existing images, swap details, and create consistent variations for each channel.": "Überarbeiten Sie vorhandene Bilder, tauschen Sie Details aus und erstellen Sie konsistente Varianten für jeden Kanal.",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Fügen Sie asynchrone Generierung für Agents, CMS-Tools und kreative Batch-Systeme hinzu.",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "Erstellen Sie Produktaufnahmen, Social-Clips und Story-Szenen mit kontrollierter Bewegung und Bildausschnitt.",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "Nutzen Sie Bild-, Video- und Audioreferenzen, um Motiv und kreative Richtung konsistent zu halten.",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Konfigurieren Sie Seitenverhältnis, Auflösung, Dauer und Ton, bevor Sie die Anfrage an die API übergeben.",
  },
  id: {
    Input: "Masukan",
    Output: "Keluaran",
    "Flatkey Router": "Router Flatkey",
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "Konfigurasikan permintaan {{model}} di halaman publik. Flatkey menyimpan draf secara lokal, lalu membuka konsol agar Anda dapat menjalankannya dengan akun dan kunci API.",
    "Model Type": "Jenis model",
    Form: "Formulir",
    "Join and run": "Daftar dan jalankan",
    "View API Docs": "Lihat dokumentasi API",
    "Avg. response time": "Waktu respons rata-rata",
    "Ready for production": "Siap untuk produksi",
    "Generated Examples": "Contoh yang dibuat",
    "Explore what {{model}} can create": "Lihat apa yang dapat dibuat {{model}}",
    "Create with this model": "Buat dengan model ini",
    "Transparent Pricing": "Harga transparan",
    "Flatkey {{model}} usage pricing": "Harga penggunaan {{model}} di Flatkey",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "Gunakan saldo dan kunci API Flatkey yang sama untuk model gambar, video, audio, dan teks.",
    "Open wallet": "Buka dompet",
    "Pricing vs official": "Perbandingan dengan harga resmi",
    "Start generating in three steps": "Mulai membuat dalam tiga langkah",
    "Try a prompt": "Coba prompt",
    "Use the playground to validate quality and style fit.": "Gunakan playground untuk memeriksa kualitas dan kesesuaian gaya.",
    "Create an API key": "Buat kunci API",
    "Sign up, open Dashboard, and create a token for this model.": "Daftar, buka Dashboard, lalu buat token untuk model ini.",
    "Ship your workflow": "Luncurkan workflow Anda",
    "Call the same endpoint, then top up credits as usage grows.": "Panggil endpoint yang sama, lalu isi ulang kredit saat penggunaan meningkat.",
    "Built for generation teams": "Dibuat untuk tim generasi",
    "{{model}} pricing FAQ": "FAQ harga {{model}}",
    "Generate your first {{model}} on Flatkey": "Buat {{model}} pertama Anda di Flatkey",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "Simpan draf, lanjutkan pendaftaran jika perlu, atau buka konsol langsung jika Anda sudah masuk.",
    "Model Guide": "Panduan model",
    Updated: "Diperbarui",
    "Model Overview": "Ikhtisar model",
    "Best for chat, code generation, agent workflows, and production assistants.": "Cocok untuk chat, pembuatan kode, workflow agent, dan asisten produksi.",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "Gunakan Flatkey untuk routing yang kompatibel dengan OpenAI, penagihan terpadu, dan kunci API yang dapat digunakan kembali.",
    "How to Use {{model}} API": "Cara menggunakan API {{model}}",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "Buat kunci API dan atur Authorization: Bearer <YOUR_API_KEY>.",
    "POST to /v1/chat/completions with at least model and messages.": "Kirim POST ke /v1/chat/completions dengan setidaknya model dan messages.",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "Sesuaikan max_tokens, temperature, dan top_p berdasarkan kompleksitas tugas.",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "Aktifkan streaming untuk UI chat, asisten terminal, dan workflow agent.",
    "Use logs and retries to refine prompts before broader rollout.": "Gunakan log dan percobaan ulang untuk menyempurnakan prompt sebelum peluncuran yang lebih luas.",
    "Common Errors": "Kesalahan umum",
    "Missing required fields, malformed messages, or unsupported parameter values.": "Kolom wajib tidak ada, format pesan salah, atau nilai parameter tidak didukung.",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "Header Authorization tidak ada, token Bearer salah format, atau kunci API tidak valid.",
    "Request rate, concurrency, or quota is above current account limits.": "Laju permintaan, konkurensi, atau kuota melebihi batas akun saat ini.",
    "Ready to unify your AI model access?": "Siap menyatukan akses ke model AI Anda?",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Gunakan satu akun Flatkey untuk menguji prompt, membandingkan model, dan memindahkan permintaan tersimpan ke konsol.",
    Endpoint: "Endpoint",
    "Aspect ratio": "Rasio aspek",
    On: "Aktif",
    Off: "Nonaktif",
    Prompt: "Prompt",
    Images: "Jumlah gambar",
    "Video URL": "URL video",
    "Preserve speech": "Pertahankan ucapan",
    "Reference media": "Media referensi",
    None: "Tidak ada",
    "Reliability over the last 30 days": "Keandalan 30 hari terakhir",
    "Measured on real Flatkey traffic, with production monitoring.": "Diukur pada trafik Flatkey nyata dengan pemantauan produksi.",
    "Routing across upstream channels": "Routing melalui kanal upstream",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Permintaan didistribusikan ke kanal yang tersedia untuk model ini; uptime 30 hari ditampilkan di atas.",
    "Speech preservation": "Pelestarian ucapan",
    "Video to Audio": "Video ke audio",
    "Audio variants": "Varian audio",
    "Create music, sound beds, and polished audio tracks from a media-aware brief.": "Buat musik, lapisan suara, dan trek audio yang rapi dari brief yang memahami media.",
    "Keep important dialogue and speech intelligible while adding a new audio layer.": "Pertahankan dialog dan ucapan penting agar tetap jelas saat menambahkan lapisan audio baru.",
    "Generate multiple delivery options for edits, localization, and production handoff.": "Hasilkan beberapa opsi keluaran untuk penyuntingan, lokalisasi, dan serah terima produksi.",
    "Best for video soundtracks, speech-preserving edits, and audio production": "Cocok untuk soundtrack video, penyuntingan yang mempertahankan ucapan, dan produksi audio",
    "Image editing": "Pengeditan gambar",
    "Reference-guided motion": "Gerakan terpandu referensi",
    "Camera and pacing": "Kamera dan tempo",
    "Generate product-style shots, merchandising scenes, and reference-guided variations.": "Buat visual produk, adegan merchandising, dan variasi terpandu referensi.",
    "Produce campaign concepts, thumbnails, posters, and localized variants.": "Hasilkan konsep kampanye, thumbnail, poster, dan varian terlokalisasi.",
    "Refine existing images, swap details, and create consistent variations for each channel.": "Sempurnakan gambar yang ada, ganti detail, dan buat varian konsisten untuk setiap kanal.",
    "Add async generation for agents, CMS tools, and batch creative systems.": "Tambahkan generasi asinkron untuk agent, alat CMS, dan sistem kreatif batch.",
    "Create product shots, social clips, and story scenes with controlled motion and framing.": "Buat visual produk, klip sosial, dan adegan cerita dengan gerakan serta framing terkontrol.",
    "Use image, video, and audio references to keep subjects and creative direction consistent.": "Gunakan referensi gambar, video, dan audio agar subjek dan arah kreatif tetap konsisten.",
    "Configure ratio, resolution, duration, and sound before handing the request to the API.": "Atur rasio, resolusi, durasi, dan suara sebelum meneruskan permintaan ke API.",
  },
};

const modelDetailPrototypeCopy: Partial<Record<Locale, Partial<Record<ModelLandingKey, string>>>> = {
  zh: {
    Capabilities: "能力",
    "Use one account and API key across text, image, video, and audio models.": "一个账号和 API Key 覆盖文本、图片、视频和音频模型。",
    "Keep prompts, quotas, and model routing in one place.": "在一个地方管理 prompt、额度和模型路由。",
    "Reference image": "参考图片",
    "Reference videos": "参考视频",
    "Reference Audios": "参考音频",
    "Video preview": "视频预览",
    "Image preview": "图片预览",
    "Audio preview": "音频预览",
    "Request summary": "请求摘要",
    "Preview only. Sign in to submit this request to Flatkey.": "仅供预览。登录后即可向 Flatkey 提交此请求。",
    "Image to Video": "图生视频",
    "Reference-guided Video": "参考引导视频",
    "Short-form Video": "短视频",
    "Text to Image": "文生图",
    "Reference-guided Image": "参考图引导",
    "Image Editing": "图片编辑",
    "Text to Audio": "文生音频",
    "Speech synthesis": "语音合成",
    "Audio generation": "音频生成",
    "Chat and coding": "聊天与编程",
    "Long context": "长上下文",
    "Tool workflows": "工具工作流",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "通过与 Flatkey 目录其他模型相同的 OpenAI 兼容路由和 API Key 调用此模型。",
    "SDK for developers": "开发者 SDK",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "继续使用现有 SDK，将 base URL 设置为 Flatkey 路由地址。",
    "Flatkey CLI": "Flatkey CLI",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "使用 Flatkey CLI 在终端工作流中管理 prompt 和请求文件。",
    "Codex & Claude Code": "Codex 与 Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "在编码 Agent 中复用同一个 Key，并通过脚本路由模型任务。",
    "Price / second": "价格 / 秒",
    generate: "生成",
    "30-day window": "30 天窗口",
    "last 30 days": "最近 30 天",
  },
  es: {
    Capabilities: "Capacidades",
    "Use one account and API key across text, image, video, and audio models.": "Usa una cuenta y una API key para modelos de texto, imagen, vídeo y audio.",
    "Keep prompts, quotas, and model routing in one place.": "Gestiona prompts, cuotas y enrutamiento en un solo lugar.",
    "Reference image": "Imagen de referencia",
    "Reference videos": "Vídeos de referencia",
    "Reference Audios": "Audios de referencia",
    "Video preview": "Vista previa de vídeo",
    "Image preview": "Vista previa de imagen",
    "Audio preview": "Vista previa de audio",
    "Request summary": "Resumen de la solicitud",
    "Preview only. Sign in to submit this request to Flatkey.": "Solo vista previa. Inicia sesión para enviar esta solicitud a Flatkey.",
    "Image to Video": "Imagen a vídeo",
    "Reference-guided Video": "Vídeo guiado por referencia",
    "Short-form Video": "Vídeo corto",
    "Text to Image": "Texto a imagen",
    "Reference-guided Image": "Imagen guiada por referencia",
    "Image Editing": "Edición de imágenes",
    "Text to Audio": "Texto a audio",
    "Speech synthesis": "Síntesis de voz",
    "Audio generation": "Generación de audio",
    "Chat and coding": "Chat y código",
    "Long context": "Contexto largo",
    "Tool workflows": "Flujos con herramientas",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Llama a este modelo con el mismo router compatible con OpenAI y la misma API key que el resto del catálogo Flatkey.",
    "SDK for developers": "SDK para desarrolladores",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "Usa tu SDK actual y establece su base URL en el router de Flatkey.",
    "Flatkey CLI": "CLI de Flatkey",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Mantén prompts y archivos de solicitudes en tu flujo de terminal con la CLI de Flatkey.",
    "Codex & Claude Code": "Codex y Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "Usa la misma clave con tu agente de código y enruta trabajos desde un script.",
    "Price / second": "Precio / segundo",
    generate: "generar",
    "30-day window": "Ventana de 30 días",
    "last 30 days": "últimos 30 días",
  },
  fr: {
    Capabilities: "Capacités",
    "Use one account and API key across text, image, video, and audio models.": "Utilisez un compte et une clé API pour les modèles texte, image, vidéo et audio.",
    "Keep prompts, quotas, and model routing in one place.": "Gérez les prompts, quotas et routage au même endroit.",
    "Reference image": "Image de référence",
    "Reference videos": "Vidéos de référence",
    "Reference Audios": "Audios de référence",
    "Video preview": "Aperçu vidéo",
    "Image preview": "Aperçu image",
    "Audio preview": "Aperçu audio",
    "Request summary": "Résumé de la requête",
    "Preview only. Sign in to submit this request to Flatkey.": "Aperçu uniquement. Connectez-vous pour envoyer cette requête à Flatkey.",
    "Image to Video": "Image vers vidéo",
    "Reference-guided Video": "Vidéo guidée par référence",
    "Short-form Video": "Vidéo courte",
    "Text to Image": "Texte vers image",
    "Reference-guided Image": "Image guidée par référence",
    "Image Editing": "Édition d’image",
    "Text to Audio": "Texte vers audio",
    "Speech synthesis": "Synthèse vocale",
    "Audio generation": "Génération audio",
    "Chat and coding": "Chat et code",
    "Long context": "Contexte long",
    "Tool workflows": "Workflows avec outils",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Appelez ce modèle avec le même routeur compatible OpenAI et la même clé API que le reste du catalogue Flatkey.",
    "SDK for developers": "SDK pour développeurs",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "Utilisez votre SDK existant et définissez sa base URL sur le routeur Flatkey.",
    "Flatkey CLI": "CLI Flatkey",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Gérez prompts et fichiers de requêtes dans votre terminal avec la CLI Flatkey.",
    "Codex & Claude Code": "Codex et Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "Utilisez la même clé avec votre agent de code et routez les tâches depuis un script.",
    "Price / second": "Prix / seconde",
    generate: "générer",
    "30-day window": "Fenêtre de 30 jours",
    "last 30 days": "30 derniers jours",
  },
  pt: {
    Capabilities: "Capacidades",
    "Use one account and API key across text, image, video, and audio models.": "Use uma conta e uma chave de API para modelos de texto, imagem, vídeo e áudio.",
    "Keep prompts, quotas, and model routing in one place.": "Gerencie prompts, cotas e roteamento em um só lugar.",
    "Reference image": "Imagem de referência",
    "Reference videos": "Vídeos de referência",
    "Reference Audios": "Áudios de referência",
    "Video preview": "Prévia de vídeo",
    "Image preview": "Prévia de imagem",
    "Audio preview": "Prévia de áudio",
    "Request summary": "Resumo da solicitação",
    "Preview only. Sign in to submit this request to Flatkey.": "Somente prévia. Entre para enviar esta solicitação à Flatkey.",
    "Image to Video": "Imagem para vídeo",
    "Reference-guided Video": "Vídeo guiado por referência",
    "Short-form Video": "Vídeo curto",
    "Text to Image": "Texto para imagem",
    "Reference-guided Image": "Imagem guiada por referência",
    "Image Editing": "Edição de imagem",
    "Text to Audio": "Texto para áudio",
    "Speech synthesis": "Síntese de fala",
    "Audio generation": "Geração de áudio",
    "Chat and coding": "Chat e código",
    "Long context": "Contexto longo",
    "Tool workflows": "Fluxos com ferramentas",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Chame este modelo pelo mesmo roteador compatível com OpenAI e pela mesma chave de API do restante do catálogo Flatkey.",
    "SDK for developers": "SDK para desenvolvedores",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "Use seu SDK atual e defina a base URL para o roteador Flatkey.",
    "Flatkey CLI": "CLI da Flatkey",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Mantenha prompts e arquivos de solicitação no fluxo do terminal com a CLI da Flatkey.",
    "Codex & Claude Code": "Codex e Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "Use a mesma chave com seu agente de código e roteie tarefas por um script.",
    "Price / second": "Preço / segundo",
    generate: "gerar",
    "30-day window": "Janela de 30 dias",
    "last 30 days": "últimos 30 dias",
  },
  ru: {
    Capabilities: "Возможности",
    "Use one account and API key across text, image, video, and audio models.": "Один аккаунт и API-ключ для текстовых, графических, видео- и аудиомоделей.",
    "Keep prompts, quotas, and model routing in one place.": "Управляйте промптами, квотами и маршрутизацией в одном месте.",
    "Reference image": "Эталонное изображение",
    "Reference videos": "Эталонные видео",
    "Reference Audios": "Эталонное аудио",
    "Video preview": "Предпросмотр видео",
    "Image preview": "Предпросмотр изображения",
    "Audio preview": "Предпросмотр аудио",
    "Request summary": "Сводка запроса",
    "Preview only. Sign in to submit this request to Flatkey.": "Только предпросмотр. Войдите, чтобы отправить запрос в Flatkey.",
    "Image to Video": "Изображение в видео",
    "Reference-guided Video": "Видео по референсу",
    "Short-form Video": "Короткое видео",
    "Text to Image": "Текст в изображение",
    "Reference-guided Image": "Изображение по референсу",
    "Image Editing": "Редактирование изображений",
    "Text to Audio": "Текст в аудио",
    "Speech synthesis": "Синтез речи",
    "Audio generation": "Генерация аудио",
    "Chat and coding": "Чат и код",
    "Long context": "Длинный контекст",
    "Tool workflows": "Рабочие процессы с инструментами",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Вызывайте эту модель через тот же OpenAI-совместимый роутер и API-ключ, что и остальные модели Flatkey.",
    "SDK for developers": "SDK для разработчиков",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "Используйте текущий SDK и укажите в base URL роутер Flatkey.",
    "Flatkey CLI": "Flatkey CLI",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Работайте с промптами и файлами запросов в терминале через Flatkey CLI.",
    "Codex & Claude Code": "Codex и Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "Используйте тот же ключ в кодинг-агенте и направляйте задачи скриптом.",
    "Price / second": "Цена / секунда",
    generate: "сгенерировать",
    "30-day window": "Период 30 дней",
    "last 30 days": "последние 30 дней",
  },
  ja: {
    Capabilities: "機能",
    "Use one account and API key across text, image, video, and audio models.": "1つのアカウントとAPIキーで、テキスト・画像・動画・音声モデルを利用できます。",
    "Keep prompts, quotas, and model routing in one place.": "プロンプト、クォータ、モデルルーティングを一元管理できます。",
    "Reference image": "参照画像",
    "Reference videos": "参照動画",
    "Reference Audios": "参照音声",
    "Video preview": "動画プレビュー",
    "Image preview": "画像プレビュー",
    "Audio preview": "音声プレビュー",
    "Request summary": "リクエスト概要",
    "Preview only. Sign in to submit this request to Flatkey.": "プレビューのみです。Flatkey に送信するにはログインしてください。",
    "Image to Video": "画像から動画",
    "Reference-guided Video": "参照画像ガイド動画",
    "Short-form Video": "ショート動画",
    "Text to Image": "テキストから画像",
    "Reference-guided Image": "参照画像ガイド生成",
    "Image Editing": "画像編集",
    "Text to Audio": "テキストから音声",
    "Speech synthesis": "音声合成",
    "Audio generation": "音声生成",
    "Chat and coding": "チャットとコーディング",
    "Long context": "長いコンテキスト",
    "Tool workflows": "ツールワークフロー",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Flatkey カタログの他のモデルと同じ OpenAI 互換ルーターと API キーでこのモデルを呼び出せます。",
    "SDK for developers": "開発者向け SDK",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "既存の SDK を使い、base URL を Flatkey ルーターに設定してください。",
    "Flatkey CLI": "Flatkey CLI",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Flatkey CLI でプロンプトとリクエストファイルをターミナルから管理できます。",
    "Codex & Claude Code": "Codex と Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "同じキーをコーディングエージェントで使い、スクリプトからモデル処理を実行できます。",
    "Price / second": "価格 / 秒",
    generate: "生成",
    "30-day window": "30日間",
    "last 30 days": "過去30日",
  },
  vi: {
    Capabilities: "Khả năng",
    "Use one account and API key across text, image, video, and audio models.": "Dùng một tài khoản và API key cho các model văn bản, hình ảnh, video và âm thanh.",
    "Keep prompts, quotas, and model routing in one place.": "Quản lý prompt, hạn mức và định tuyến model tại một nơi.",
    "Reference image": "Ảnh tham chiếu",
    "Reference videos": "Video tham chiếu",
    "Reference Audios": "Âm thanh tham chiếu",
    "Video preview": "Xem trước video",
    "Image preview": "Xem trước hình ảnh",
    "Audio preview": "Xem trước âm thanh",
    "Request summary": "Tóm tắt yêu cầu",
    "Preview only. Sign in to submit this request to Flatkey.": "Chỉ xem trước. Đăng nhập để gửi yêu cầu đến Flatkey.",
    "Image to Video": "Ảnh thành video",
    "Reference-guided Video": "Video theo ảnh tham chiếu",
    "Short-form Video": "Video ngắn",
    "Text to Image": "Văn bản thành ảnh",
    "Reference-guided Image": "Ảnh theo tham chiếu",
    "Image Editing": "Chỉnh sửa ảnh",
    "Text to Audio": "Văn bản thành âm thanh",
    "Speech synthesis": "Tổng hợp giọng nói",
    "Audio generation": "Tạo âm thanh",
    "Chat and coding": "Trò chuyện và lập trình",
    "Long context": "Ngữ cảnh dài",
    "Tool workflows": "Quy trình với công cụ",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Gọi model này qua router tương thích OpenAI và API key giống các model khác trong danh mục Flatkey.",
    "SDK for developers": "SDK cho nhà phát triển",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "Dùng SDK hiện có và đặt base URL tới router Flatkey.",
    "Flatkey CLI": "Flatkey CLI",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Quản lý prompt và file yêu cầu trong terminal bằng Flatkey CLI.",
    "Codex & Claude Code": "Codex và Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "Dùng cùng key cho coding agent và định tuyến tác vụ model từ script.",
    "Price / second": "Giá / giây",
    generate: "tạo",
    "30-day window": "Khung 30 ngày",
    "last 30 days": "30 ngày qua",
  },
  de: {
    Capabilities: "Funktionen",
    "Use one account and API key across text, image, video, and audio models.": "Ein Konto und ein API-Schlüssel für Text-, Bild-, Video- und Audiomodelle.",
    "Keep prompts, quotas, and model routing in one place.": "Prompts, Quoten und Modellrouting an einem Ort verwalten.",
    "Reference image": "Referenzbild",
    "Reference videos": "Referenzvideos",
    "Reference Audios": "Referenzaudio",
    "Video preview": "Videovorschau",
    "Image preview": "Bildvorschau",
    "Audio preview": "Audiovorschau",
    "Request summary": "Anfrageübersicht",
    "Preview only. Sign in to submit this request to Flatkey.": "Nur Vorschau. Melden Sie sich an, um diese Anfrage an Flatkey zu senden.",
    "Image to Video": "Bild zu Video",
    "Reference-guided Video": "Referenzgesteuertes Video",
    "Short-form Video": "Kurzvideo",
    "Text to Image": "Text zu Bild",
    "Reference-guided Image": "Referenzgesteuertes Bild",
    "Image Editing": "Bildbearbeitung",
    "Text to Audio": "Text zu Audio",
    "Speech synthesis": "Sprachsynthese",
    "Audio generation": "Audiogenerierung",
    "Chat and coding": "Chat und Coding",
    "Long context": "Langer Kontext",
    "Tool workflows": "Tool-Workflows",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Rufen Sie dieses Modell über denselben OpenAI-kompatiblen Router und API-Schlüssel wie den restlichen Flatkey-Katalog auf.",
    "SDK for developers": "SDK für Entwickler",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "Verwenden Sie Ihr bestehendes SDK und setzen Sie die base URL auf den Flatkey-Router.",
    "Flatkey CLI": "Flatkey CLI",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Verwalten Sie Prompts und Anfrage-Dateien mit der Flatkey CLI im Terminal.",
    "Codex & Claude Code": "Codex und Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "Verwenden Sie denselben Schlüssel mit Ihrem Coding-Agent und routen Sie Aufgaben per Skript.",
    "Price / second": "Preis / Sekunde",
    generate: "generieren",
    "30-day window": "30-Tage-Fenster",
    "last 30 days": "letzte 30 Tage",
  },
  id: {
    Capabilities: "Kemampuan",
    "Use one account and API key across text, image, video, and audio models.": "Gunakan satu akun dan API key untuk model teks, gambar, video, dan audio.",
    "Keep prompts, quotas, and model routing in one place.": "Kelola prompt, kuota, dan routing model di satu tempat.",
    "Reference image": "Gambar referensi",
    "Reference videos": "Video referensi",
    "Reference Audios": "Audio referensi",
    "Video preview": "Pratinjau video",
    "Image preview": "Pratinjau gambar",
    "Audio preview": "Pratinjau audio",
    "Request summary": "Ringkasan permintaan",
    "Preview only. Sign in to submit this request to Flatkey.": "Hanya pratinjau. Masuk untuk mengirim permintaan ini ke Flatkey.",
    "Image to Video": "Gambar ke video",
    "Reference-guided Video": "Video berbasis referensi",
    "Short-form Video": "Video pendek",
    "Text to Image": "Teks ke gambar",
    "Reference-guided Image": "Gambar berbasis referensi",
    "Image Editing": "Pengeditan gambar",
    "Text to Audio": "Teks ke audio",
    "Speech synthesis": "Sintesis suara",
    "Audio generation": "Pembuatan audio",
    "Chat and coding": "Chat dan coding",
    "Long context": "Konteks panjang",
    "Tool workflows": "Alur kerja tool",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Panggil model ini melalui router kompatibel OpenAI dan API key yang sama seperti katalog Flatkey lainnya.",
    "SDK for developers": "SDK untuk developer",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "Gunakan SDK yang ada dan atur base URL ke router Flatkey.",
    "Flatkey CLI": "Flatkey CLI",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Kelola prompt dan file permintaan dalam alur terminal dengan Flatkey CLI.",
    "Codex & Claude Code": "Codex & Claude Code",
    "Use the same key with your coding agent and route model jobs from a script.": "Gunakan key yang sama dengan coding agent dan arahkan tugas model dari script.",
    "Price / second": "Harga / detik",
    generate: "buat",
    "30-day window": "Jendela 30 hari",
    "last 30 days": "30 hari terakhir",
  },
};

const supplementalModelLandingCopy: Partial<Record<Locale, Partial<Record<ModelLandingKey, string>>>> = {
  zh: {
    Playground: "Playground",
    Parameters: "参数",
    "Prompt library": "提示词库",
    "Live catalog model": "实时目录模型",
    "Catalog data unavailable": "目录数据暂不可用",
    "Model price comparison": "模型价格对比",
    "After bonus": "充值后",
    "Pricing data unavailable": "价格数据暂不可用",
    "Live model health": "实时模型健康",
    "30-day health, measured on real traffic": "基于真实流量的 30 天健康度",
    "Available catalog entries": "可用目录条目",
    Status: "状态",
    Throughput: "吞吐",
    "/ request": "/ 请求",
  },
  es: {
    Playground: "Playground",
    Parameters: "Parámetros",
    "Prompt library": "Biblioteca de prompts",
    "Live catalog model": "Modelo activo del catálogo",
    "Catalog data unavailable": "Datos del catálogo no disponibles",
    Providers: "Proveedores",
    Provider: "Proveedor",
    API: "API",
    Pricing: "Precios",
    "Model price comparison": "Comparación de precios del modelo",
    "After bonus": "Después del bono",
    "Pricing data unavailable": "Datos de precios no disponibles",
    Performance: "Rendimiento",
    Benchmarks: "Benchmarks",
    Apps: "Apps",
    Activity: "Actividad",
    FAQ: "Preguntas frecuentes",
    Modalities: "Modalidades",
    "In / Out price": "Precio entrada / salida",
    Context: "Contexto",
    Released: "Lanzamiento",
    "Knowledge cutoff": "Corte de conocimiento",
    "Input /M": "Entrada /M",
    "Output /M": "Salida /M",
    Latency: "Latencia",
    Uptime: "Disponibilidad",
    "Live model health": "Salud del modelo en vivo",
    "30-day health, measured on real traffic": "Salud de 30 días medida con tráfico real",
    "Available catalog entries": "Entradas disponibles del catálogo",
    Status: "Estado",
    Throughput: "Throughput",
    "Successful inference trend": "Tendencia de inferencias correctas",
    "Not enough data yet": "Aún no hay datos suficientes",
    "Avg. provider uptime": "Disponibilidad media del proveedor",
    Requests: "Solicitudes",
    "Frequently asked questions": "Preguntas frecuentes",
    "Reliability Index": "Índice de fiabilidad",
    "Throughput Index": "Índice de throughput",
    "Latency Index": "Índice de latencia",
    "Prompt testing and request handoff.": "Pruebas de prompts y traspaso de solicitudes.",
    "Monthly token share from Flatkey rankings.": "Cuota mensual de tokens desde los rankings de Flatkey.",
    "Model catalog": "Catálogo de modelos",
    "Related models from the pricing catalog.": "Modelos relacionados del catálogo de precios.",
    "API Gateway": "API Gateway",
    "Supported endpoint coverage in our pricing API.": "Cobertura de endpoints soportados en nuestra API de precios.",
    "Weighted avg input price": "Precio medio ponderado de entrada",
    "Weighted avg output price": "Precio medio ponderado de salida",
    "Cache read": "Lectura de caché",
    "Cache write": "Escritura de caché",
    "Request price": "Precio por solicitud",
    "Reference price": "Precio de referencia",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey enruta tu solicitud a los canales upstream disponibles para este modelo y mantiene la facturación en una sola cuenta.",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Los precios siguientes se calculan con los datos de precios de Flatkey para este modelo y los grupos visibles que devuelve nuestra API de precios.",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "El rendimiento usa telemetría de solicitudes de Flatkey de los últimos 30 días cuando hay suficiente tráfico.",
    "Token volume and request traffic for this model over time.": "Volumen de tokens y tráfico de solicitudes de este modelo a lo largo del tiempo.",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} está disponible en Flatkey con precios en vivo, routing de proveedor, ejemplos de generación, traspaso de API y enlaces a modelos relacionados.",
    "What is {{model}}?": "¿Qué es {{model}}?",
    "How much does {{model}} cost?": "¿Cuánto cuesta {{model}}?",
    "Which providers serve {{model}}?": "¿Qué proveedores sirven {{model}}?",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "Usa la sección de precios anterior para ver los precios actuales de Flatkey desde nuestra API de precios.",
    "The providers section shows the upstream provider names available in our model catalog.": "La sección de proveedores muestra los nombres upstream disponibles en nuestro catálogo de modelos.",
    "/ request": "/ solicitud",
  },
  fr: {
    Playground: "Playground",
    Parameters: "Paramètres",
    "Prompt library": "Bibliothèque de prompts",
    "Live catalog model": "Modèle actif du catalogue",
    "Catalog data unavailable": "Données du catalogue indisponibles",
    Providers: "Fournisseurs",
    Provider: "Fournisseur",
    API: "API",
    Pricing: "Tarifs",
    "Model price comparison": "Comparaison des prix du modèle",
    "After bonus": "Après bonus",
    "Pricing data unavailable": "Données tarifaires indisponibles",
    Performance: "Performance",
    Benchmarks: "Benchmarks",
    Apps: "Apps",
    Activity: "Activité",
    FAQ: "FAQ",
    Modalities: "Modalités",
    "In / Out price": "Prix entrée / sortie",
    Context: "Contexte",
    Released: "Sortie",
    "Knowledge cutoff": "Date limite de connaissance",
    "Input /M": "Entrée /M",
    "Output /M": "Sortie /M",
    Latency: "Latence",
    Uptime: "Disponibilité",
    "Live model health": "Santé du modèle en direct",
    "30-day health, measured on real traffic": "Santé sur 30 jours mesurée sur trafic réel",
    "Available catalog entries": "Entrées disponibles du catalogue",
    Status: "Statut",
    Throughput: "Débit",
    "Successful inference trend": "Tendance des inférences réussies",
    "Not enough data yet": "Pas encore assez de données",
    "Avg. provider uptime": "Disponibilité moyenne du fournisseur",
    Requests: "Requêtes",
    "Frequently asked questions": "Questions fréquentes",
    "Reliability Index": "Indice de fiabilité",
    "Throughput Index": "Indice de débit",
    "Latency Index": "Indice de latence",
    "Prompt testing and request handoff.": "Tests de prompts et passage de requête.",
    "Monthly token share from Flatkey rankings.": "Part mensuelle de tokens issue des classements Flatkey.",
    "Model catalog": "Catalogue de modèles",
    "Related models from the pricing catalog.": "Modèles associés du catalogue tarifaire.",
    "API Gateway": "Passerelle API",
    "Supported endpoint coverage in our pricing API.": "Couverture des endpoints pris en charge dans notre API de tarifs.",
    "Weighted avg input price": "Prix d'entrée moyen pondéré",
    "Weighted avg output price": "Prix de sortie moyen pondéré",
    "Cache read": "Lecture cache",
    "Cache write": "Écriture cache",
    "Request price": "Prix par requête",
    "Reference price": "Prix de référence",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey route votre requête vers les canaux upstream disponibles pour ce modèle et regroupe la facturation dans un seul compte.",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Les prix ci-dessous sont calculés à partir des données tarifaires Flatkey pour ce modèle et des groupes visibles renvoyés par notre API de tarifs.",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "La performance utilise la télémétrie des requêtes Flatkey des 30 derniers jours lorsqu'il y a assez de trafic.",
    "Token volume and request traffic for this model over time.": "Volume de tokens et trafic de requêtes pour ce modèle dans le temps.",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} est disponible via Flatkey avec tarifs en direct, routage fournisseur, exemples de génération, passage API et liens vers des modèles associés.",
    "What is {{model}}?": "Qu'est-ce que {{model}} ?",
    "How much does {{model}} cost?": "Combien coûte {{model}} ?",
    "Which providers serve {{model}}?": "Quels fournisseurs servent {{model}} ?",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "Consultez la section tarifs ci-dessus pour les prix Flatkey actuels issus de notre API de tarifs.",
    "The providers section shows the upstream provider names available in our model catalog.": "La section fournisseurs affiche les fournisseurs upstream disponibles dans notre catalogue de modèles.",
    "/ request": "/ requête",
  },
  pt: {
    Playground: "Playground",
    Parameters: "Parâmetros",
    "Prompt library": "Biblioteca de prompts",
    "Live catalog model": "Modelo ativo do catálogo",
    "Catalog data unavailable": "Dados do catálogo indisponíveis",
    Providers: "Provedores",
    Provider: "Provedor",
    API: "API",
    Pricing: "Preços",
    "Model price comparison": "Comparação de preços do modelo",
    "After bonus": "Depois do bônus",
    "Pricing data unavailable": "Dados de preço indisponíveis",
    Performance: "Desempenho",
    Benchmarks: "Benchmarks",
    Apps: "Apps",
    Activity: "Atividade",
    FAQ: "FAQ",
    Modalities: "Modalidades",
    "In / Out price": "Preço entrada / saída",
    Context: "Contexto",
    Released: "Lançamento",
    "Knowledge cutoff": "Corte de conhecimento",
    "Input /M": "Entrada /M",
    "Output /M": "Saída /M",
    Latency: "Latência",
    Uptime: "Disponibilidade",
    "Live model health": "Saúde do modelo em tempo real",
    "30-day health, measured on real traffic": "Saúde de 30 dias medida com tráfego real",
    "Available catalog entries": "Entradas disponíveis no catálogo",
    Status: "Status",
    Throughput: "Throughput",
    "Successful inference trend": "Tendência de inferências bem-sucedidas",
    "Not enough data yet": "Ainda não há dados suficientes",
    "Avg. provider uptime": "Disponibilidade média do provedor",
    Requests: "Requisições",
    "Frequently asked questions": "Perguntas frequentes",
    "Reliability Index": "Índice de confiabilidade",
    "Throughput Index": "Índice de throughput",
    "Latency Index": "Índice de latência",
    "Prompt testing and request handoff.": "Teste de prompts e handoff de requisições.",
    "Monthly token share from Flatkey rankings.": "Participação mensal de tokens nos rankings da Flatkey.",
    "Model catalog": "Catálogo de modelos",
    "Related models from the pricing catalog.": "Modelos relacionados do catálogo de preços.",
    "API Gateway": "API Gateway",
    "Supported endpoint coverage in our pricing API.": "Cobertura de endpoints suportados na nossa API de preços.",
    "Weighted avg input price": "Preço médio ponderado de entrada",
    "Weighted avg output price": "Preço médio ponderado de saída",
    "Cache read": "Leitura de cache",
    "Cache write": "Escrita de cache",
    "Request price": "Preço por requisição",
    "Reference price": "Preço de referência",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "A Flatkey roteia sua requisição para canais upstream disponíveis para este modelo e mantém a cobrança em uma conta.",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Os preços abaixo são calculados com os dados de preços da Flatkey para este modelo e os grupos visíveis retornados pela nossa API de preços.",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "O desempenho usa telemetria de requisições da Flatkey dos últimos 30 dias quando há tráfego suficiente.",
    "Token volume and request traffic for this model over time.": "Volume de tokens e tráfego de requisições deste modelo ao longo do tempo.",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} está disponível pela Flatkey com preços em tempo real, roteamento de provedor, exemplos de geração, handoff de API e links para modelos relacionados.",
    "What is {{model}}?": "O que é {{model}}?",
    "How much does {{model}} cost?": "Quanto custa {{model}}?",
    "Which providers serve {{model}}?": "Quais provedores servem {{model}}?",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "Use a seção de preços acima para ver os preços atuais da Flatkey retornados pela nossa API de preços.",
    "The providers section shows the upstream provider names available in our model catalog.": "A seção de provedores mostra os upstreams disponíveis no nosso catálogo de modelos.",
    "/ request": "/ requisição",
  },
  ru: {
    Playground: "Playground",
    Parameters: "Параметры",
    "Prompt library": "Библиотека промптов",
    "Live catalog model": "Активная модель каталога",
    "Catalog data unavailable": "Данные каталога недоступны",
    Providers: "Провайдеры",
    Provider: "Провайдер",
    API: "API",
    Pricing: "Цены",
    "Model price comparison": "Сравнение цен модели",
    "After bonus": "После бонуса",
    "Pricing data unavailable": "Данные цен недоступны",
    Performance: "Производительность",
    Benchmarks: "Бенчмарки",
    Apps: "Приложения",
    Activity: "Активность",
    FAQ: "FAQ",
    Modalities: "Модальности",
    "In / Out price": "Цена ввода / вывода",
    Context: "Контекст",
    Released: "Дата выхода",
    "Knowledge cutoff": "Срез знаний",
    "Input /M": "Ввод /M",
    "Output /M": "Вывод /M",
    Latency: "Задержка",
    Uptime: "Доступность",
    "Live model health": "Текущее состояние модели",
    "30-day health, measured on real traffic": "Состояние за 30 дней на реальном трафике",
    "Available catalog entries": "Доступные записи каталога",
    Status: "Статус",
    Throughput: "Пропускная способность",
    "Successful inference trend": "Тренд успешных инференсов",
    "Not enough data yet": "Пока недостаточно данных",
    "Avg. provider uptime": "Средняя доступность провайдера",
    Requests: "Запросы",
    "Frequently asked questions": "Частые вопросы",
    "Reliability Index": "Индекс надежности",
    "Throughput Index": "Индекс пропускной способности",
    "Latency Index": "Индекс задержки",
    "Prompt testing and request handoff.": "Тестирование prompt и передача запроса.",
    "Monthly token share from Flatkey rankings.": "Месячная доля токенов из рейтингов Flatkey.",
    "Model catalog": "Каталог моделей",
    "Related models from the pricing catalog.": "Связанные модели из ценового каталога.",
    "API Gateway": "API Gateway",
    "Supported endpoint coverage in our pricing API.": "Покрытие поддерживаемых endpoints в нашем pricing API.",
    "Weighted avg input price": "Средневзвешенная цена ввода",
    "Weighted avg output price": "Средневзвешенная цена вывода",
    "Cache read": "Чтение кэша",
    "Cache write": "Запись кэша",
    "Request price": "Цена запроса",
    "Reference price": "Справочная цена",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey маршрутизирует запрос в доступные upstream-каналы для этой модели и ведет оплату в одном аккаунте.",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Цены ниже рассчитываются из данных Flatkey для этой модели и видимых групп, которые возвращает наш pricing API.",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Производительность использует телеметрию запросов Flatkey за последние 30 дней, если трафика достаточно.",
    "Token volume and request traffic for this model over time.": "Объем токенов и трафик запросов для этой модели во времени.",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} доступна через Flatkey с актуальными ценами, маршрутизацией провайдеров, примерами генерации, передачей API и ссылками на связанные модели.",
    "What is {{model}}?": "Что такое {{model}}?",
    "How much does {{model}} cost?": "Сколько стоит {{model}}?",
    "Which providers serve {{model}}?": "Какие провайдеры обслуживают {{model}}?",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "Актуальные цены Flatkey из нашего pricing API смотрите в разделе цен выше.",
    "The providers section shows the upstream provider names available in our model catalog.": "Раздел провайдеров показывает upstream-провайдеров, доступных в нашем каталоге моделей.",
    "/ request": "/ запрос",
  },
  ja: {
    Playground: "Playground",
    Parameters: "パラメータ",
    "Prompt library": "プロンプトライブラリ",
    "Live catalog model": "ライブカタログモデル",
    "Catalog data unavailable": "カタログデータは利用できません",
    Providers: "プロバイダー",
    Provider: "プロバイダー",
    API: "API",
    Pricing: "料金",
    "Model price comparison": "モデル料金比較",
    "After bonus": "ボーナス適用後",
    "Pricing data unavailable": "料金データは利用できません",
    Performance: "パフォーマンス",
    Benchmarks: "ベンチマーク",
    Apps: "アプリ",
    Activity: "アクティビティ",
    FAQ: "FAQ",
    Modalities: "モダリティ",
    "In / Out price": "入力 / 出力料金",
    Context: "コンテキスト",
    Released: "公開日",
    "Knowledge cutoff": "知識カットオフ",
    "Input /M": "入力 /M",
    "Output /M": "出力 /M",
    Latency: "レイテンシ",
    Uptime: "稼働率",
    "Live model health": "ライブモデル健全性",
    "30-day health, measured on real traffic": "実トラフィックで測定した30日間の健全性",
    "Available catalog entries": "利用可能なカタログ項目",
    Status: "ステータス",
    Throughput: "スループット",
    "Successful inference trend": "成功した推論の推移",
    "Not enough data yet": "まだ十分なデータがありません",
    "Avg. provider uptime": "平均プロバイダー稼働率",
    Requests: "リクエスト",
    "Frequently asked questions": "よくある質問",
    "Reliability Index": "信頼性指数",
    "Throughput Index": "スループット指数",
    "Latency Index": "レイテンシ指数",
    "Prompt testing and request handoff.": "プロンプトテストとリクエスト引き継ぎ。",
    "Monthly token share from Flatkey rankings.": "Flatkey ランキングに基づく月間 token シェア。",
    "Model catalog": "モデルカタログ",
    "Related models from the pricing catalog.": "料金カタログ内の関連モデル。",
    "API Gateway": "API Gateway",
    "Supported endpoint coverage in our pricing API.": "料金 API で対応している endpoint の範囲。",
    "Weighted avg input price": "加重平均入力料金",
    "Weighted avg output price": "加重平均出力料金",
    "Cache read": "キャッシュ読み取り",
    "Cache write": "キャッシュ書き込み",
    "Request price": "リクエスト料金",
    "Reference price": "参考料金",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey はこのモデルで利用可能な upstream チャンネルへリクエストをルーティングし、請求を 1 つのアカウントにまとめます。",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "以下の料金は、このモデルの Flatkey 料金データと料金 API が返す表示可能グループから計算されます。",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "十分なトラフィックがある場合、パフォーマンスは過去 30 日の Flatkey リクエストテレメトリを使用します。",
    "Token volume and request traffic for this model over time.": "このモデルの token 量とリクエストトラフィックの推移。",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} は Flatkey で利用でき、ライブ料金、プロバイダールーティング、生成例、API 引き継ぎ、関連モデルリンクを提供します。",
    "What is {{model}}?": "{{model}} とは何ですか？",
    "How much does {{model}} cost?": "{{model}} の料金はいくらですか？",
    "Which providers serve {{model}}?": "{{model}} を提供するプロバイダーはどれですか？",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "現在の Flatkey 料金は、上の料金セクションで料金 API から確認できます。",
    "The providers section shows the upstream provider names available in our model catalog.": "プロバイダーセクションには、モデルカタログで利用可能な upstream プロバイダー名が表示されます。",
    "/ request": "/ リクエスト",
  },
  vi: {
    Playground: "Playground",
    Parameters: "Tham số",
    "Prompt library": "Thư viện prompt",
    "Live catalog model": "Model trực tiếp trong danh mục",
    "Catalog data unavailable": "Chưa có dữ liệu danh mục",
    Providers: "Nhà cung cấp",
    Provider: "Nhà cung cấp",
    API: "API",
    Pricing: "Giá",
    "Model price comparison": "So sánh giá model",
    "After bonus": "Sau ưu đãi nạp",
    "Pricing data unavailable": "Chưa có dữ liệu giá",
    Performance: "Hiệu năng",
    Benchmarks: "Benchmark",
    Apps: "Ứng dụng",
    Activity: "Hoạt động",
    FAQ: "FAQ",
    Modalities: "Phương thức",
    "In / Out price": "Giá đầu vào / đầu ra",
    Context: "Ngữ cảnh",
    Released: "Ngày phát hành",
    "Knowledge cutoff": "Mốc kiến thức",
    "Input /M": "Đầu vào /M",
    "Output /M": "Đầu ra /M",
    Latency: "Độ trễ",
    Uptime: "Uptime",
    "Live model health": "Tình trạng model trực tiếp",
    "30-day health, measured on real traffic": "Sức khỏe 30 ngày đo trên traffic thật",
    "Available catalog entries": "Mục danh mục khả dụng",
    Status: "Trạng thái",
    Throughput: "Throughput",
    "Successful inference trend": "Xu hướng suy luận thành công",
    "Not enough data yet": "Chưa đủ dữ liệu",
    "Avg. provider uptime": "Uptime trung bình của nhà cung cấp",
    Requests: "Lượt gọi",
    "Frequently asked questions": "Câu hỏi thường gặp",
    "Reliability Index": "Chỉ số độ tin cậy",
    "Throughput Index": "Chỉ số throughput",
    "Latency Index": "Chỉ số độ trễ",
    "Prompt testing and request handoff.": "Kiểm thử prompt và bàn giao request.",
    "Monthly token share from Flatkey rankings.": "Tỷ trọng token hằng tháng từ bảng xếp hạng Flatkey.",
    "Model catalog": "Danh mục mô hình",
    "Related models from the pricing catalog.": "Mô hình liên quan từ danh mục giá.",
    "API Gateway": "API Gateway",
    "Supported endpoint coverage in our pricing API.": "Phạm vi endpoint được hỗ trợ trong API giá của chúng tôi.",
    "Weighted avg input price": "Giá đầu vào bình quân gia quyền",
    "Weighted avg output price": "Giá đầu ra bình quân gia quyền",
    "Cache read": "Đọc cache",
    "Cache write": "Ghi cache",
    "Request price": "Giá mỗi request",
    "Reference price": "Giá tham chiếu",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey định tuyến request tới các kênh upstream khả dụng cho mô hình này và gom thanh toán trong một tài khoản.",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Giá bên dưới được tính từ dữ liệu giá Flatkey cho mô hình này và các nhóm hiển thị do API giá trả về.",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Hiệu năng dùng telemetry request Flatkey trong 30 ngày gần nhất khi có đủ lưu lượng.",
    "Token volume and request traffic for this model over time.": "Khối lượng token và lưu lượng request của mô hình này theo thời gian.",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} có trên Flatkey với giá trực tiếp, định tuyến nhà cung cấp, ví dụ tạo nội dung, handoff API và liên kết mô hình liên quan.",
    "What is {{model}}?": "{{model}} là gì?",
    "How much does {{model}} cost?": "{{model}} có giá bao nhiêu?",
    "Which providers serve {{model}}?": "Nhà cung cấp nào phục vụ {{model}}?",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "Xem phần giá bên trên để biết giá Flatkey hiện tại từ API giá của chúng tôi.",
    "The providers section shows the upstream provider names available in our model catalog.": "Phần nhà cung cấp hiển thị tên upstream có trong danh mục mô hình của chúng tôi.",
    "/ request": "/ request",
  },
  de: {
    Playground: "Playground",
    Parameters: "Parameter",
    "Prompt library": "Prompt-Bibliothek",
    "Live catalog model": "Live-Modell im Katalog",
    "Catalog data unavailable": "Katalogdaten nicht verfügbar",
    Providers: "Anbieter",
    Provider: "Anbieter",
    API: "API",
    Pricing: "Preise",
    "Model price comparison": "Modellpreisvergleich",
    "After bonus": "Nach Bonus",
    "Pricing data unavailable": "Preisdaten nicht verfügbar",
    Performance: "Performance",
    Benchmarks: "Benchmarks",
    Apps: "Apps",
    Activity: "Aktivität",
    FAQ: "FAQ",
    Modalities: "Modalitäten",
    "In / Out price": "Preis Eingabe / Ausgabe",
    Context: "Kontext",
    Released: "Veröffentlicht",
    "Knowledge cutoff": "Wissensstand",
    "Input /M": "Eingabe /M",
    "Output /M": "Ausgabe /M",
    Latency: "Latenz",
    Uptime: "Verfügbarkeit",
    "Live model health": "Live-Modellzustand",
    "30-day health, measured on real traffic": "30-Tage-Zustand anhand realen Traffics",
    "Available catalog entries": "Verfügbare Katalogeinträge",
    Status: "Status",
    Throughput: "Durchsatz",
    "Successful inference trend": "Trend erfolgreicher Inferenzen",
    "Not enough data yet": "Noch nicht genug Daten",
    "Avg. provider uptime": "Durchschn. Anbieter-Verfügbarkeit",
    Requests: "Anfragen",
    "Frequently asked questions": "Häufige Fragen",
    "Reliability Index": "Zuverlässigkeitsindex",
    "Throughput Index": "Durchsatzindex",
    "Latency Index": "Latenzindex",
    "Prompt testing and request handoff.": "Prompt-Tests und Übergabe von Anfragen.",
    "Monthly token share from Flatkey rankings.": "Monatlicher Token-Anteil aus den Flatkey-Rankings.",
    "Model catalog": "Modellkatalog",
    "Related models from the pricing catalog.": "Verwandte Modelle aus dem Preiskatalog.",
    "API Gateway": "API Gateway",
    "Supported endpoint coverage in our pricing API.": "Abdeckung unterstützter Endpoints in unserer Pricing API.",
    "Weighted avg input price": "Gewichteter Durchschnittspreis für Eingabe",
    "Weighted avg output price": "Gewichteter Durchschnittspreis für Ausgabe",
    "Cache read": "Cache-Lesen",
    "Cache write": "Cache-Schreiben",
    "Request price": "Preis pro Anfrage",
    "Reference price": "Referenzpreis",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey leitet Ihre Anfrage an verfügbare Upstream-Kanäle für dieses Modell weiter und bündelt die Abrechnung in einem Konto.",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Die Preise unten werden aus den Flatkey-Preisdaten für dieses Modell und den sichtbaren Gruppen berechnet, die unsere Pricing API zurückgibt.",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Performance nutzt Flatkey-Anfragetelemetrie der letzten 30 Tage, wenn genügend Traffic vorhanden ist.",
    "Token volume and request traffic for this model over time.": "Token-Volumen und Anfrage-Traffic für dieses Modell im Zeitverlauf.",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} ist über Flatkey mit Live-Preisen, Anbieter-Routing, Generierungsbeispielen, API-Übergabe und Links zu verwandten Modellen verfügbar.",
    "What is {{model}}?": "Was ist {{model}}?",
    "How much does {{model}} cost?": "Was kostet {{model}}?",
    "Which providers serve {{model}}?": "Welche Anbieter stellen {{model}} bereit?",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "Die aktuellen Flatkey-Preise aus unserer Pricing API finden Sie im Preisbereich oben.",
    "The providers section shows the upstream provider names available in our model catalog.": "Der Anbieterbereich zeigt die Upstream-Anbieter, die in unserem Modellkatalog verfügbar sind.",
    "/ request": "/ Anfrage",
  },
  id: {
    Playground: "Playground",
    Parameters: "Parameter",
    "Prompt library": "Pustaka prompt",
    "Live catalog model": "Model katalog live",
    "Catalog data unavailable": "Data katalog tidak tersedia",
    Providers: "Penyedia",
    Provider: "Penyedia",
    API: "API",
    Pricing: "Harga",
    "Model price comparison": "Perbandingan harga model",
    "After bonus": "Setelah bonus",
    "Pricing data unavailable": "Data harga tidak tersedia",
    Performance: "Performa",
    Benchmarks: "Benchmark",
    Apps: "Aplikasi",
    Activity: "Aktivitas",
    FAQ: "FAQ",
    Modalities: "Modalitas",
    "In / Out price": "Harga input / output",
    Context: "Konteks",
    Released: "Dirilis",
    "Knowledge cutoff": "Batas pengetahuan",
    "Input /M": "Input /M",
    "Output /M": "Output /M",
    Latency: "Latensi",
    Uptime: "Uptime",
    "Live model health": "Kesehatan model live",
    "30-day health, measured on real traffic": "Kesehatan 30 hari diukur dari traffic nyata",
    "Available catalog entries": "Entri katalog tersedia",
    Status: "Status",
    Throughput: "Throughput",
    "Successful inference trend": "Tren inferensi berhasil",
    "Not enough data yet": "Data belum cukup",
    "Avg. provider uptime": "Rata-rata uptime penyedia",
    Requests: "Request",
    "Frequently asked questions": "Pertanyaan umum",
    "Reliability Index": "Indeks reliabilitas",
    "Throughput Index": "Indeks throughput",
    "Latency Index": "Indeks latensi",
    "Prompt testing and request handoff.": "Pengujian prompt dan handoff request.",
    "Monthly token share from Flatkey rankings.": "Porsi token bulanan dari peringkat Flatkey.",
    "Model catalog": "Katalog model",
    "Related models from the pricing catalog.": "Model terkait dari katalog harga.",
    "API Gateway": "API Gateway",
    "Supported endpoint coverage in our pricing API.": "Cakupan endpoint yang didukung di API harga kami.",
    "Weighted avg input price": "Harga input rata-rata tertimbang",
    "Weighted avg output price": "Harga output rata-rata tertimbang",
    "Cache read": "Baca cache",
    "Cache write": "Tulis cache",
    "Request price": "Harga per request",
    "Reference price": "Harga referensi",
    "Flatkey routes your request to available upstream channels for this model and keeps billing under one account.": "Flatkey merutekan request Anda ke kanal upstream yang tersedia untuk model ini dan menyatukan penagihan dalam satu akun.",
    "Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.": "Harga di bawah dihitung dari data harga Flatkey untuk model ini dan grup terlihat yang saat ini dikembalikan oleh API harga kami.",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Performa menggunakan telemetri request Flatkey dari 30 hari terakhir saat traffic cukup.",
    "Token volume and request traffic for this model over time.": "Volume token dan traffic request untuk model ini dari waktu ke waktu.",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} tersedia melalui Flatkey dengan harga live, routing penyedia, contoh generasi, handoff API, dan tautan model terkait.",
    "What is {{model}}?": "Apa itu {{model}}?",
    "How much does {{model}} cost?": "Berapa biaya {{model}}?",
    "Which providers serve {{model}}?": "Penyedia mana yang melayani {{model}}?",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "Gunakan bagian harga di atas untuk melihat harga Flatkey terbaru dari API harga kami.",
    "The providers section shows the upstream provider names available in our model catalog.": "Bagian penyedia menampilkan nama upstream yang tersedia di katalog model kami.",
    "/ request": "/ request",
  },
};

const modelComparisonCopy: Record<Locale, Partial<Record<ModelLandingKey, string>>> = {
  en: {
    "Compare the current model with the previous generation before you migrate.": "Compare the current model with the previous generation before you migrate.",
    "Best for": "Best for",
    Capability: "Capability",
    "Price type": "Price type",
    "Price / image": "Price / image",
    "Previous generation": "Previous generation",
    Breadcrumb: "Breadcrumb",
    "Model sections": "Model sections",
    "Sample usage trend": "Sample usage trend",
    "Usage trend": "Usage trend",
    "Upload or drag and drop": "Upload or drag and drop",
    "Remove reference image": "Remove reference image",
    "Uploaded {{count}} files": "{{count}} uploaded files",
    "Remove {{name}}": "Remove {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "What changed from the previous generation, so you can tell whether it is worth switching.",
  },
  zh: {
    "Compare the current model with the previous generation before you migrate.": "迁移前，对比当前模型与上一代模型。",
    "Best for": "适用场景",
    Capability: "能力",
    "Price type": "价格类型",
    "Price / image": "价格 / 图片",
    "Previous generation": "上一代",
    Breadcrumb: "面包屑导航",
    "Model sections": "模型板块",
    "Sample usage trend": "示例使用趋势",
    "Usage trend": "使用趋势",
    "Upload or drag and drop": "上传或拖拽文件",
    "Remove reference image": "移除参考图片",
    "Uploaded {{count}} files": "已上传 {{count}} 个文件",
    "Remove {{name}}": "移除 {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "看看当前版本相比上一代有哪些变化，再决定是否迁移。",
  },
  es: {
    "Compare the current model with the previous generation before you migrate.": "Compara el modelo actual con la generación anterior antes de migrar.",
    "Best for": "Ideal para",
    Capability: "Capacidad",
    "Price type": "Tipo de precio",
    "Price / image": "Precio / imagen",
    "Previous generation": "Generación anterior",
    Breadcrumb: "Ruta de navegación",
    "Model sections": "Secciones del modelo",
    "Sample usage trend": "Tendencia de uso de ejemplo",
    "Usage trend": "Tendencia de uso",
    "Upload or drag and drop": "Sube o arrastra y suelta",
    "Remove reference image": "Eliminar imagen de referencia",
    "Uploaded {{count}} files": "{{count}} archivos subidos",
    "Remove {{name}}": "Eliminar {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "Consulta qué cambió frente a la generación anterior antes de cambiar.",
  },
  fr: {
    "Compare the current model with the previous generation before you migrate.": "Comparez le modèle actuel à la génération précédente avant de migrer.",
    "Best for": "Idéal pour",
    Capability: "Capacité",
    "Price type": "Type de tarif",
    "Price / image": "Prix / image",
    "Previous generation": "Génération précédente",
    Breadcrumb: "Fil d’Ariane",
    "Model sections": "Sections du modèle",
    "Sample usage trend": "Tendance d’utilisation d’exemple",
    "Usage trend": "Tendance d’utilisation",
    "Upload or drag and drop": "Téléversez ou glissez-déposez",
    "Remove reference image": "Supprimer l’image de référence",
    "Uploaded {{count}} files": "{{count}} fichiers téléversés",
    "Remove {{name}}": "Supprimer {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "Découvrez ce qui change par rapport à la génération précédente avant de migrer.",
  },
  pt: {
    "Compare the current model with the previous generation before you migrate.": "Compare o modelo atual com a geração anterior antes de migrar.",
    "Best for": "Ideal para",
    Capability: "Capacidade",
    "Price type": "Tipo de preço",
    "Price / image": "Preço / imagem",
    "Previous generation": "Geração anterior",
    Breadcrumb: "Trilha de navegação",
    "Model sections": "Seções do modelo",
    "Sample usage trend": "Tendência de uso de exemplo",
    "Usage trend": "Tendência de uso",
    "Upload or drag and drop": "Envie ou arraste e solte",
    "Remove reference image": "Remover imagem de referência",
    "Uploaded {{count}} files": "{{count}} arquivos enviados",
    "Remove {{name}}": "Remover {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "Veja o que mudou em relação à geração anterior antes de migrar.",
  },
  ru: {
    "Compare the current model with the previous generation before you migrate.": "Сравните текущую модель с предыдущим поколением перед миграцией.",
    "Best for": "Лучше всего для",
    Capability: "Возможность",
    "Price type": "Тип цены",
    "Price / image": "Цена / изображение",
    "Previous generation": "Предыдущее поколение",
    Breadcrumb: "Хлебные крошки",
    "Model sections": "Разделы модели",
    "Sample usage trend": "Пример динамики использования",
    "Usage trend": "Динамика использования",
    "Upload or drag and drop": "Загрузите или перетащите файл",
    "Remove reference image": "Удалить эталонное изображение",
    "Uploaded {{count}} files": "Загружено файлов: {{count}}",
    "Remove {{name}}": "Удалить {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "Узнайте, что изменилось по сравнению с предыдущим поколением, прежде чем переходить.",
  },
  ja: {
    "Compare the current model with the previous generation before you migrate.": "移行前に、現行モデルと前世代モデルを比較できます。",
    "Best for": "おすすめ用途",
    Capability: "機能",
    "Price type": "料金種別",
    "Price / image": "画像あたりの料金",
    "Previous generation": "前世代",
    Breadcrumb: "パンくずリスト",
    "Model sections": "モデルのセクション",
    "Sample usage trend": "サンプル利用傾向",
    "Usage trend": "利用傾向",
    "Upload or drag and drop": "アップロードまたはドラッグ＆ドロップ",
    "Remove reference image": "参照画像を削除",
    "Uploaded {{count}} files": "{{count}} 件のファイルをアップロード済み",
    "Remove {{name}}": "{{name}} を削除",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "移行前に、前世代から何が変わったかを確認できます。",
  },
  vi: {
    "Compare the current model with the previous generation before you migrate.": "So sánh model hiện tại với thế hệ trước trước khi di chuyển.",
    "Best for": "Phù hợp nhất cho",
    Capability: "Khả năng",
    "Price type": "Loại giá",
    "Price / image": "Giá / ảnh",
    "Previous generation": "Thế hệ trước",
    Breadcrumb: "Đường dẫn",
    "Model sections": "Các phần của model",
    "Sample usage trend": "Xu hướng sử dụng mẫu",
    "Usage trend": "Xu hướng sử dụng",
    "Upload or drag and drop": "Tải lên hoặc kéo thả",
    "Remove reference image": "Xóa ảnh tham chiếu",
    "Uploaded {{count}} files": "Đã tải lên {{count}} tệp",
    "Remove {{name}}": "Xóa {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "Xem model hiện tại đã thay đổi gì so với thế hệ trước trước khi chuyển đổi.",
  },
  de: {
    "Compare the current model with the previous generation before you migrate.": "Vergleichen Sie das aktuelle Modell vor der Migration mit der vorherigen Generation.",
    "Best for": "Am besten für",
    Capability: "Fähigkeit",
    "Price type": "Preisart",
    "Price / image": "Preis / Bild",
    "Previous generation": "Vorherige Generation",
    Breadcrumb: "Brotkrümelnavigation",
    "Model sections": "Modellbereiche",
    "Sample usage trend": "Beispiel-Nutzungstrend",
    "Usage trend": "Nutzungstrend",
    "Upload or drag and drop": "Hochladen oder per Drag-and-drop ablegen",
    "Remove reference image": "Referenzbild entfernen",
    "Uploaded {{count}} files": "{{count}} Dateien hochgeladen",
    "Remove {{name}}": "{{name}} entfernen",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "Sehen Sie, was sich gegenüber der vorherigen Generation geändert hat, bevor Sie wechseln.",
  },
  id: {
    "Compare the current model with the previous generation before you migrate.": "Bandingkan model saat ini dengan generasi sebelumnya sebelum migrasi.",
    "Best for": "Terbaik untuk",
    Capability: "Kapabilitas",
    "Price type": "Jenis harga",
    "Price / image": "Harga / gambar",
    "Previous generation": "Generasi sebelumnya",
    Breadcrumb: "Breadcrumb",
    "Model sections": "Bagian model",
    "Sample usage trend": "Tren penggunaan contoh",
    "Usage trend": "Tren penggunaan",
    "Upload or drag and drop": "Unggah atau seret dan lepas",
    "Remove reference image": "Hapus gambar referensi",
    "Uploaded {{count}} files": "{{count}} file diunggah",
    "Remove {{name}}": "Hapus {{name}}",
    "What changed from the previous generation, so you can tell whether it is worth switching.": "Lihat perubahan dari generasi sebelumnya sebelum beralih.",
  },
};

/**
 * Shared labels used by the current model-detail shell.  The older catalog
 * copy table predates the refreshed shell and only covered the English/Chinese
 * variants, so keeping these labels in a dedicated map prevents localized
 * pages from quietly rendering a second language in the generator, telemetry,
 * comparison, and API sections.
 */
const modelDetailUiCopy: Partial<Record<Locale, Record<string, string>>> = {
  zh: {
    "Back to Models": "返回模型列表",
    "Copy model id": "复制模型 ID",
    "Get API Key": "获取 API Key",
    "Flatkey price": "Flatkey 价格",
    "Reference price": "官方价格",
    Provider: "供应商",
    Input: "输入",
    Output: "输出",
    Preview: "预览",
    "Open in Playground": "在 Playground 打开",
    "Generator setup": "生成器配置",
    "Playground (edit before sign-up)": "Playground（注册前可编辑）",
    "Start generating": "开始生成",
    "Request summary": "请求摘要",
    "Preview only. Sign in to submit this request to Flatkey.": "仅供预览。登录后即可向 Flatkey 提交此请求。",
    "Quick Prompts": "快捷 Prompt",
    "Advanced Options": "高级选项",
    "Upload or drag and drop": "上传或拖拽文件",
    "Reference image": "参考图片",
    "Reference videos": "参考视频",
    "Reference Audios": "参考音频",
    "Reference Images": "参考图片",
    "Upload reference": "上传参考",
    Images: "图片数量",
    Size: "尺寸",
    Quality: "质量",
    Background: "背景",
    Moderation: "审核强度",
    Resolution: "分辨率",
    "Aspect ratio": "画面比例",
    Duration: "时长",
    "Output format": "输出格式",
    Outputs: "输出数量",
    "Video URL": "视频 URL",
    "Preserve speech": "保留人声",
    "Generate audio": "生成音频",
    "AIGC watermark": "AIGC 水印",
    Frames: "帧数",
    "Camera fixed": "固定镜头",
    "Return last frame": "返回尾帧",
    Seed: "随机种子",
    "Optional frame count override": "可选的帧数覆盖",
    "0 means random": "0 表示随机",
    On: "开",
    Off: "关",
    None: "无",
    "Model ID": "模型 ID",
    Capability: "能力",
    Modalities: "模态",
    Context: "上下文",
    "Best for": "适合场景",
    Text: "文本",
    "Text to Video": "文生视频",
    "Text to Image": "文生图",
    "Image to Video": "图生视频",
    "Reference-guided Video": "参考引导视频",
    "Short-form Video": "短视频",
    Audio: "音频",
    "Image to Image": "图像生成",
    "Reference-guided Image": "参考图引导",
    "Image Editing": "图片编辑",
    "Video to Audio": "视频转音频",
    "Speech preservation": "人声保留",
    "Speech synthesis": "语音合成",
    "Audio generation": "音频生成",
    "Chat and coding": "聊天与编程",
    "Long context": "长上下文",
    "Tool workflows": "工具工作流",
    Performance: "性能",
    Activity: "活动",
    FAQ: "FAQ",
    Playground: "Playground",
    Capabilities: "能力",
    Related: "相关模型",
    "Related models": "相关模型",
    "Related models from the pricing catalog.": "来自价格目录的相关模型。",
    "Keep exploring Flatkey": "继续浏览 Flatkey",
    "More models from {{provider}}": "更多 {{provider}} 模型",
    "Swipe or scroll to compare": "滑动或滚动查看",
    "Live catalog model": "实时目录模型",
    "Frequently asked questions": "常见问题",
    "Use the pricing section above for current Flatkey prices from our pricing API.": "请查看上方价格区，那里展示了定价 API 返回的当前 Flatkey 价格。",
    "The providers section shows the upstream provider names available in our model catalog.": "供应商区展示了我们模型目录中该模型可用的上游供应商。",
    "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} 可通过 Flatkey 使用，并提供实时价格、供应商路由、生成示例、API 接力和相关模型内链。",
    "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} 是适合聊天、代码、长上下文推理和工具工作流的生产级文本模型，可通过 Flatkey 兼容 API 访问。",
    "Reliability over the last 30 days": "最近 30 天的可靠性",
    "Measured on real Flatkey traffic, with production monitoring.": "基于 Flatkey 真实流量测量，并持续进行生产监控。",
    "Token volume and request traffic for this model over time.": "该模型随时间变化的 token 用量和请求流量。",
    "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "当流量足够时，性能数据使用 Flatkey 近 30 天请求遥测。",
    "Avg. provider uptime": "供应商平均可用性",
    Latency: "延迟",
    Requests: "请求量",
    Uptime: "可用性",
    "30-day window": "30 天窗口",
    "last 30 days": "最近 30 天",
    "Successful inference trend": "成功推理趋势",
    "Not enough data yet": "数据积累中",
    "API Gateway": "API 网关",
    "Supported endpoint coverage in our pricing API.": "定价 API 中支持的 endpoint 覆盖。",
    Docs: "文档",
    "SDK for developers": "开发者 SDK",
    "Flatkey CLI": "Flatkey CLI",
    "Codex & Claude Code": "Codex 与 Claude Code",
    "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "通过与 Flatkey 目录其他模型相同的 OpenAI 兼容路由和 API Key 调用此模型。",
    "Use your existing SDK and set its base URL to the Flatkey router origin.": "继续使用现有 SDK，将 base URL 设置为 Flatkey 路由地址。",
    "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "使用 Flatkey CLI 在终端工作流中管理 prompt 和请求文件。",
    "Use the same key with your coding agent and route model jobs from a script.": "在编码 Agent 中复用同一个 Key，并通过脚本路由模型任务。",
    "Request preview": "请求预览",
    "Copy request": "复制请求",
    "Why Flatkey": "为什么选择 Flatkey",
    "Why use Flatkey for {{model}}?": "为什么用 Flatkey 调用 {{model}}？",
    "Lower generation pricing": "更低的生成成本",
    "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "媒体生成任务走 Flatkey，扩量前的 prompt 测试成本更可控。",
    "Draft handoff": "草稿接力",
    "The public page stores prompt settings locally before sending the user into Flatkey.": "公开页会先在本地保存 prompt 和参数，再把用户带到 Flatkey。",
    "Unified API access": "统一 API 入口",
    "Keep usage, keys, quotas, and model routing in one Flatkey account.": "在一个 Flatkey 账号中管理用量、Key、额度和模型路由。",
    "Routing across upstream channels": "上游通道路由",
    "Requests are spread across the channels serving this model, with 30-day uptime published above.": "请求会分配到服务该模型的可用上游通道，页面上方展示近 30 天的可用性。",
    "Core capabilities and practical engineering value": "核心能力与工程价值",
    "OpenAI-compatible migration path": "兼容 OpenAI 的迁移路径",
    "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Chat Completions 风格请求体能降低从现有模型栈迁移的成本。",
    "Structured and tool-based output": "结构化与工具输出",
    "Use structured JSON, tools, and code-generation flows for agentic workflows.": "可在 Agent 工作流中使用结构化 JSON、工具和代码生成流程。",
    "Streaming interaction": "流式交互",
    "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "流式输出适合聊天 UI、终端助手和渐进式渲染。",
    "Production routing": "生产路由",
    "Price / image": "价格 / 张图片",
    "Price / second": "价格 / 秒",
    "Request price": "请求价格",
    "/ image": "/ 张图片",
    "/ second": "/ 秒",
    "/ request": "/ 请求",
  },
  es: {
    "Back to Models": "Volver a modelos", "Copy model id": "Copiar ID del modelo", "Get API Key": "Obtener clave API", "Flatkey price": "Precio de Flatkey", "Reference price": "Precio de referencia", Provider: "Proveedor", Input: "Entrada", Output: "Salida", Preview: "Vista previa", "Open in Playground": "Abrir en Playground", "Generator setup": "Configuración del generador", "Playground (edit before sign-up)": "Playground (edita antes de registrarte)", "Start generating": "Empezar a generar", "Request summary": "Resumen de la solicitud", "Preview only. Sign in to submit this request to Flatkey.": "Solo vista previa. Inicia sesión para enviar esta solicitud a Flatkey.", "Quick Prompts": "Prompts rápidos", "Advanced Options": "Opciones avanzadas", "Upload or drag and drop": "Sube o arrastra y suelta", "Reference image": "Imagen de referencia", "Reference videos": "Vídeos de referencia", "Reference Audios": "Audios de referencia", "Reference Images": "Imágenes de referencia", "Upload reference": "Subir referencia", Images: "Imágenes", Size: "Tamaño", Quality: "Calidad", Background: "Fondo", Moderation: "Moderación", Resolution: "Resolución", "Aspect ratio": "Relación de aspecto", Duration: "Duración", "Output format": "Formato de salida", Outputs: "Salidas", "Video URL": "URL del vídeo", "Preserve speech": "Conservar el habla", "Generate audio": "Generar audio", "AIGC watermark": "Marca de agua AIGC", Frames: "Fotogramas", "Camera fixed": "Cámara fija", "Return last frame": "Devolver el último fotograma", Seed: "Semilla", "Optional frame count override": "Recuento de fotogramas opcional", "0 means random": "0 significa aleatorio", On: "Activado", Off: "Desactivado", None: "Ninguno", "Model ID": "ID del modelo", Capability: "Capacidad", Modalities: "Modalidades", Context: "Contexto", "Best for": "Ideal para", Text: "Texto", "Text to Video": "Texto a vídeo", "Text to Image": "Texto a imagen", "Image to Video": "Imagen a vídeo", "Reference-guided Video": "Vídeo guiado por referencia", "Short-form Video": "Vídeo corto", Audio: "Audio", "Image to Image": "Imagen a imagen", "Reference-guided Image": "Imagen guiada por referencia", "Image Editing": "Edición de imágenes", "Video to Audio": "Vídeo a audio", "Speech preservation": "Conservación del habla", "Speech synthesis": "Síntesis de voz", "Audio generation": "Generación de audio", "Chat and coding": "Chat y código", "Long context": "Contexto largo", "Tool workflows": "Flujos con herramientas", Performance: "Rendimiento", Activity: "Actividad", FAQ: "Preguntas frecuentes", Playground: "Playground", Capabilities: "Capacidades", "Related models": "Modelos relacionados", "Related models from the pricing catalog.": "Modelos relacionados del catálogo de precios.", "Keep exploring Flatkey": "Seguir explorando Flatkey", "More models from {{provider}}": "Más modelos de {{provider}}", "Swipe or scroll to compare": "Desliza o desplázate para comparar", "Live catalog model": "Modelo del catálogo en vivo", "Frequently asked questions": "Preguntas frecuentes", "Use the pricing section above for current Flatkey prices from our pricing API.": "Usa la sección de precios anterior para ver los precios actuales de Flatkey desde nuestra API de precios.", "The providers section shows the upstream provider names available in our model catalog.": "La sección de proveedores muestra los nombres upstream disponibles en nuestro catálogo de modelos.", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} está disponible en Flatkey con precios en vivo, enrutamiento de proveedores, ejemplos de generación, traspaso de API y enlaces a modelos relacionados.", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} es un modelo de texto para producción con chat, código, contexto largo y flujos con herramientas mediante la API compatible con Flatkey.", "Reliability over the last 30 days": "Fiabilidad de los últimos 30 días", "Measured on real Flatkey traffic, with production monitoring.": "Medido con tráfico real de Flatkey y monitorización de producción.", "Token volume and request traffic for this model over time.": "Volumen de tokens y tráfico de solicitudes de este modelo a lo largo del tiempo.", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "El rendimiento usa telemetría de solicitudes de Flatkey de los últimos 30 días cuando hay tráfico suficiente.", "Avg. provider uptime": "Disponibilidad media del proveedor", Latency: "Latencia", Requests: "Solicitudes", Uptime: "Disponibilidad", "30-day window": "Ventana de 30 días", "last 30 days": "últimos 30 días", "Successful inference trend": "Tendencia de inferencias exitosas", "Not enough data yet": "Aún no hay datos suficientes", "API Gateway": "Pasarela API", "Supported endpoint coverage in our pricing API.": "Cobertura de endpoints compatible con nuestra API de precios.", Docs: "Documentación", "SDK for developers": "SDK para desarrolladores", "Flatkey CLI": "CLI de Flatkey", "Codex & Claude Code": "Codex y Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Llama a este modelo con el mismo router compatible con OpenAI y la misma clave API que el resto del catálogo Flatkey.", "Use your existing SDK and set its base URL to the Flatkey router origin.": "Usa tu SDK actual y establece su URL base en el router de Flatkey.", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Mantén prompts y archivos de solicitudes en tu flujo de terminal con la CLI de Flatkey.", "Use the same key with your coding agent and route model jobs from a script.": "Usa la misma clave con tu agente de código y enruta trabajos desde un script.", "Request preview": "Vista previa de la solicitud", "Copy request": "Copiar solicitud", "Why Flatkey": "Por qué Flatkey", "Why use Flatkey for {{model}}?": "¿Por qué usar Flatkey para {{model}}?", "Lower generation pricing": "Menor precio de generación", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "Enruta cargas multimedia por Flatkey y mantén más baratos los tests de prompts antes de escalar.", "Draft handoff": "Traspaso del borrador", "The public page stores prompt settings locally before sending the user into Flatkey.": "La página pública guarda localmente la configuración del prompt antes de llevarte a Flatkey.", "Unified API access": "Acceso API unificado", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "Gestiona uso, claves, cuotas y enrutamiento en una sola cuenta de Flatkey.", "Routing across upstream channels": "Enrutamiento entre canales upstream", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Las solicitudes se distribuyen entre los canales disponibles para este modelo; arriba se muestra la disponibilidad de los últimos 30 días.", "Core capabilities and practical engineering value": "Capacidades principales y valor práctico de ingeniería", "OpenAI-compatible migration path": "Ruta de migración compatible con OpenAI", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Los cuerpos estilo Chat Completions reducen la fricción al migrar desde stacks existentes.", "Structured and tool-based output": "Salida estructurada y basada en herramientas", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "Usa JSON estructurado, herramientas y flujos de generación de código para agentes.", "Streaming interaction": "Interacción en streaming", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "El streaming permite interfaces de chat, asistentes de terminal y renderizado progresivo.", "Production routing": "Enrutamiento de producción", "Price / image": "Precio / imagen", "Price / second": "Precio / segundo", "Request price": "Precio por solicitud", "/ image": "/ imagen", "/ second": "/ segundo", "/ request": "/ solicitud",
  },
  fr: {
    "Back to Models": "Retour aux modèles", "Copy model id": "Copier l’identifiant du modèle", "Get API Key": "Obtenir une clé API", "Flatkey price": "Prix Flatkey", "Reference price": "Prix de référence", Provider: "Fournisseur", Input: "Entrée", Output: "Sortie", Preview: "Aperçu", "Open in Playground": "Ouvrir dans le Playground", "Generator setup": "Configuration du générateur", "Playground (edit before sign-up)": "Playground (modifiez avant l’inscription)", "Start generating": "Commencer la génération", "Request summary": "Résumé de la requête", "Preview only. Sign in to submit this request to Flatkey.": "Aperçu uniquement. Connectez-vous pour envoyer cette requête à Flatkey.", "Quick Prompts": "Prompts rapides", "Advanced Options": "Options avancées", "Upload or drag and drop": "Téléversez ou glissez-déposez", "Reference image": "Image de référence", "Reference videos": "Vidéos de référence", "Reference Audios": "Audios de référence", "Reference Images": "Images de référence", "Upload reference": "Téléverser une référence", Images: "Images", Size: "Taille", Quality: "Qualité", Background: "Arrière-plan", Moderation: "Modération", Resolution: "Résolution", "Aspect ratio": "Format", Duration: "Durée", "Output format": "Format de sortie", Outputs: "Sorties", "Video URL": "URL vidéo", "Preserve speech": "Préserver la parole", "Generate audio": "Générer de l’audio", "AIGC watermark": "Filigrane AIGC", Frames: "Images", "Camera fixed": "Caméra fixe", "Return last frame": "Retourner la dernière image", Seed: "Graine", "Optional frame count override": "Nombre d’images facultatif", "0 means random": "0 signifie aléatoire", On: "Activé", Off: "Désactivé", None: "Aucun", "Model ID": "ID du modèle", Capability: "Capacité", Modalities: "Modalités", Context: "Contexte", "Best for": "Idéal pour", Text: "Texte", "Text to Video": "Texte vers vidéo", "Text to Image": "Texte vers image", "Image to Video": "Image vers vidéo", "Reference-guided Video": "Vidéo guidée par référence", "Short-form Video": "Vidéo courte", Audio: "Audio", "Image to Image": "Image vers image", "Reference-guided Image": "Image guidée par référence", "Image Editing": "Édition d’image", "Video to Audio": "Vidéo vers audio", "Speech preservation": "Préservation de la parole", "Speech synthesis": "Synthèse vocale", "Audio generation": "Génération audio", "Chat and coding": "Chat et code", "Long context": "Contexte long", "Tool workflows": "Workflows avec outils", Performance: "Performance", Activity: "Activité", FAQ: "FAQ", Playground: "Playground", Capabilities: "Capacités", "Related models": "Modèles associés", "Related models from the pricing catalog.": "Modèles associés du catalogue tarifaire.", "Keep exploring Flatkey": "Continuer avec Flatkey", "More models from {{provider}}": "Autres modèles de {{provider}}", "Swipe or scroll to compare": "Faites glisser ou défilez pour comparer", "Live catalog model": "Modèle du catalogue en direct", "Frequently asked questions": "Questions fréquentes", "Use the pricing section above for current Flatkey prices from our pricing API.": "Consultez la section tarifs ci-dessus pour les prix Flatkey actuels issus de notre API.", "The providers section shows the upstream provider names available in our model catalog.": "La section fournisseurs affiche les fournisseurs upstream disponibles dans notre catalogue.", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} est disponible via Flatkey avec tarifs en direct, routage fournisseur, exemples de génération, passage API et liens vers des modèles associés.", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} est un modèle texte de production pour le chat, le code, le raisonnement long contexte et les workflows avec outils via l’API compatible Flatkey.", "Reliability over the last 30 days": "Fiabilité sur les 30 derniers jours", "Measured on real Flatkey traffic, with production monitoring.": "Mesuré sur le trafic réel de Flatkey, avec supervision en production.", "Token volume and request traffic for this model over time.": "Volume de tokens et trafic de requêtes de ce modèle dans le temps.", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "La performance utilise la télémétrie des requêtes Flatkey des 30 derniers jours lorsque le trafic est suffisant.", "Avg. provider uptime": "Disponibilité moyenne du fournisseur", Latency: "Latence", Requests: "Requêtes", Uptime: "Disponibilité", "30-day window": "Fenêtre de 30 jours", "last 30 days": "30 derniers jours", "Successful inference trend": "Tendance des inférences réussies", "Not enough data yet": "Pas encore assez de données", "API Gateway": "Passerelle API", "Supported endpoint coverage in our pricing API.": "Couverture des endpoints pris en charge par notre API tarifaire.", Docs: "Documentation", "SDK for developers": "SDK pour développeurs", "Flatkey CLI": "CLI Flatkey", "Codex & Claude Code": "Codex et Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Appelez ce modèle avec le même routeur compatible OpenAI et la même clé API que le reste du catalogue Flatkey.", "Use your existing SDK and set its base URL to the Flatkey router origin.": "Utilisez votre SDK existant et définissez sa base URL sur le routeur Flatkey.", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Gérez prompts et fichiers de requêtes dans votre terminal avec la CLI Flatkey.", "Use the same key with your coding agent and route model jobs from a script.": "Utilisez la même clé avec votre agent de code et routez les tâches depuis un script.", "Request preview": "Aperçu de la requête", "Copy request": "Copier la requête", "Why Flatkey": "Pourquoi Flatkey", "Why use Flatkey for {{model}}?": "Pourquoi utiliser Flatkey pour {{model}} ?", "Lower generation pricing": "Un prix de génération plus bas", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "Routez les charges média via Flatkey et réduisez le coût des tests de prompts avant de passer à l’échelle.", "Draft handoff": "Transmission du brouillon", "The public page stores prompt settings locally before sending the user into Flatkey.": "La page publique enregistre localement les réglages du prompt avant de vous diriger vers Flatkey.", "Unified API access": "Accès API unifié", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "Gérez l’usage, les clés, les quotas et le routage dans un seul compte Flatkey.", "Routing across upstream channels": "Routage entre canaux amont", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Les requêtes sont réparties entre les canaux disponibles pour ce modèle ; la disponibilité des 30 derniers jours est affichée ci-dessus.", "Core capabilities and practical engineering value": "Capacités clés et valeur pratique pour l’ingénierie", "OpenAI-compatible migration path": "Parcours de migration compatible OpenAI", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Les charges utiles de type Chat Completions réduisent la friction lors de la migration.", "Structured and tool-based output": "Sortie structurée et outillée", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "Utilisez du JSON structuré, des outils et des workflows de génération de code pour les agents.", "Streaming interaction": "Interaction en streaming", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "Le streaming prend en charge les interfaces de chat, assistants de terminal et rendu progressif.", "Production routing": "Routage de production", "Price / image": "Prix / image", "Price / second": "Prix / seconde", "Request price": "Prix par requête", "/ image": "/ image", "/ second": "/ seconde", "/ request": "/ requête",
  },
  pt: {
    "Back to Models": "Voltar aos modelos", "Copy model id": "Copiar ID do modelo", "Get API Key": "Obter chave de API", "Flatkey price": "Preço Flatkey", "Reference price": "Preço de referência", Provider: "Provedor", Input: "Entrada", Output: "Saída", Preview: "Prévia", "Open in Playground": "Abrir no Playground", "Generator setup": "Configuração do gerador", "Playground (edit before sign-up)": "Playground (edite antes de cadastrar)", "Start generating": "Começar a gerar", "Request summary": "Resumo da solicitação", "Preview only. Sign in to submit this request to Flatkey.": "Somente prévia. Entre para enviar esta solicitação à Flatkey.", "Quick Prompts": "Prompts rápidos", "Advanced Options": "Opções avançadas", "Upload or drag and drop": "Envie ou arraste e solte", "Reference image": "Imagem de referência", "Reference videos": "Vídeos de referência", "Reference Audios": "Áudios de referência", "Reference Images": "Imagens de referência", "Upload reference": "Enviar referência", Images: "Imagens", Size: "Tamanho", Quality: "Qualidade", Background: "Fundo", Moderation: "Moderação", Resolution: "Resolução", "Aspect ratio": "Proporção", Duration: "Duração", "Output format": "Formato de saída", Outputs: "Saídas", "Video URL": "URL do vídeo", "Preserve speech": "Preservar fala", "Generate audio": "Gerar áudio", "AIGC watermark": "Marca d’água AIGC", Frames: "Quadros", "Camera fixed": "Câmera fixa", "Return last frame": "Retornar último quadro", Seed: "Semente", "Optional frame count override": "Quantidade opcional de quadros", "0 means random": "0 significa aleatório", On: "Ativado", Off: "Desativado", None: "Nenhum", "Model ID": "ID do modelo", Capability: "Capacidade", Modalities: "Modalidades", Context: "Contexto", "Best for": "Ideal para", Text: "Texto", "Text to Video": "Texto para vídeo", "Text to Image": "Texto para imagem", "Image to Video": "Imagem para vídeo", "Reference-guided Video": "Vídeo guiado por referência", "Short-form Video": "Vídeo curto", Audio: "Áudio", "Image to Image": "Imagem para imagem", "Reference-guided Image": "Imagem guiada por referência", "Image Editing": "Edição de imagem", "Video to Audio": "Vídeo para áudio", "Speech preservation": "Preservação da fala", "Speech synthesis": "Síntese de fala", "Audio generation": "Geração de áudio", "Chat and coding": "Chat e código", "Long context": "Contexto longo", "Tool workflows": "Fluxos com ferramentas", Performance: "Desempenho", Activity: "Atividade", FAQ: "Perguntas frequentes", Playground: "Playground", Capabilities: "Capacidades", "Related models": "Modelos relacionados", "Related models from the pricing catalog.": "Modelos relacionados do catálogo de preços.", "Keep exploring Flatkey": "Continuar explorando a Flatkey", "More models from {{provider}}": "Mais modelos da {{provider}}", "Swipe or scroll to compare": "Deslize ou role para comparar", "Live catalog model": "Modelo do catálogo ao vivo", "Frequently asked questions": "Perguntas frequentes", "Use the pricing section above for current Flatkey prices from our pricing API.": "Use a seção de preços acima para ver os preços atuais da Flatkey retornados pela nossa API.", "The providers section shows the upstream provider names available in our model catalog.": "A seção de provedores mostra os upstreams disponíveis no catálogo de modelos.", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} está disponível pela Flatkey com preços em tempo real, roteamento de provedores, exemplos de geração, handoff de API e links para modelos relacionados.", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} é um modelo de texto para produção com chat, código, contexto longo e fluxos com ferramentas pela API compatível com Flatkey.", "Reliability over the last 30 days": "Confiabilidade nos últimos 30 dias", "Measured on real Flatkey traffic, with production monitoring.": "Medido com tráfego real da Flatkey e monitoramento de produção.", "Token volume and request traffic for this model over time.": "Volume de tokens e tráfego de solicitações deste modelo ao longo do tempo.", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "O desempenho usa a telemetria de solicitações da Flatkey dos últimos 30 dias quando há tráfego suficiente.", "Avg. provider uptime": "Disponibilidade média do provedor", Latency: "Latência", Requests: "Solicitações", Uptime: "Disponibilidade", "30-day window": "Janela de 30 dias", "last 30 days": "últimos 30 dias", "Successful inference trend": "Tendência de inferências bem-sucedidas", "Not enough data yet": "Ainda não há dados suficientes", "API Gateway": "Gateway de API", "Supported endpoint coverage in our pricing API.": "Cobertura de endpoints compatíveis na nossa API de preços.", Docs: "Documentação", "SDK for developers": "SDK para desenvolvedores", "Flatkey CLI": "CLI da Flatkey", "Codex & Claude Code": "Codex e Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Chame este modelo pelo mesmo roteador compatível com OpenAI e pela mesma chave de API do restante do catálogo Flatkey.", "Use your existing SDK and set its base URL to the Flatkey router origin.": "Use seu SDK atual e defina a base URL para o roteador Flatkey.", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Mantenha prompts e arquivos de solicitação no fluxo do terminal com a CLI Flatkey.", "Use the same key with your coding agent and route model jobs from a script.": "Use a mesma chave com seu agente de código e roteie tarefas por um script.", "Request preview": "Prévia da solicitação", "Copy request": "Copiar solicitação", "Why Flatkey": "Por que Flatkey", "Why use Flatkey for {{model}}?": "Por que usar a Flatkey para {{model}}?", "Lower generation pricing": "Menor preço de geração", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "Roteie cargas de mídia pela Flatkey e mantenha os testes de prompts mais baratos antes de escalar.", "Draft handoff": "Transferência do rascunho", "The public page stores prompt settings locally before sending the user into Flatkey.": "A página pública salva as configurações do prompt localmente antes de levar você à Flatkey.", "Unified API access": "Acesso unificado à API", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "Gerencie uso, chaves, cotas e roteamento em uma única conta Flatkey.", "Routing across upstream channels": "Roteamento entre canais upstream", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "As solicitações são distribuídas entre os canais disponíveis para este modelo; a disponibilidade dos últimos 30 dias aparece acima.", "Core capabilities and practical engineering value": "Principais capacidades e valor prático para engenharia", "OpenAI-compatible migration path": "Caminho de migração compatível com OpenAI", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Payloads no estilo Chat Completions reduzem o atrito ao migrar de stacks existentes.", "Structured and tool-based output": "Saída estruturada e baseada em ferramentas", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "Use JSON estruturado, ferramentas e fluxos de geração de código para agentes.", "Streaming interaction": "Interação em streaming", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "O streaming atende a UIs de chat, assistentes de terminal e renderização progressiva.", "Production routing": "Roteamento de produção", "Price / image": "Preço / imagem", "Price / second": "Preço / segundo", "Request price": "Preço por solicitação", "/ image": "/ imagem", "/ second": "/ segundo", "/ request": "/ solicitação",
  },
  ru: {
    "Back to Models": "Назад к моделям", "Copy model id": "Скопировать ID модели", "Get API Key": "Получить API-ключ", "Flatkey price": "Цена Flatkey", "Reference price": "Справочная цена", Provider: "Провайдер", Input: "Ввод", Output: "Вывод", Preview: "Предпросмотр", "Open in Playground": "Открыть в Playground", "Generator setup": "Настройки генератора", "Playground (edit before sign-up)": "Playground (измените до регистрации)", "Start generating": "Начать генерацию", "Request summary": "Сводка запроса", "Preview only. Sign in to submit this request to Flatkey.": "Только предпросмотр. Войдите, чтобы отправить запрос в Flatkey.", "Quick Prompts": "Быстрые промпты", "Advanced Options": "Расширенные настройки", "Upload or drag and drop": "Загрузите или перетащите файл", "Reference image": "Эталонное изображение", "Reference videos": "Эталонные видео", "Reference Audios": "Эталонное аудио", "Reference Images": "Эталонные изображения", "Upload reference": "Загрузить референс", Images: "Изображения", Size: "Размер", Quality: "Качество", Background: "Фон", Moderation: "Модерация", Resolution: "Разрешение", "Aspect ratio": "Соотношение сторон", Duration: "Длительность", "Output format": "Формат вывода", Outputs: "Выходы", "Video URL": "URL видео", "Preserve speech": "Сохранять речь", "Generate audio": "Генерировать аудио", "AIGC watermark": "Водяной знак AIGC", Frames: "Кадры", "Camera fixed": "Фиксированная камера", "Return last frame": "Вернуть последний кадр", Seed: "Семя", "Optional frame count override": "Необязательное число кадров", "0 means random": "0 означает случайное значение", On: "Вкл.", Off: "Выкл.", None: "Нет", "Model ID": "ID модели", Capability: "Возможность", Modalities: "Модальности", Context: "Контекст", "Best for": "Лучше всего для", Text: "Текст", "Text to Video": "Текст в видео", "Text to Image": "Текст в изображение", "Image to Video": "Изображение в видео", "Reference-guided Video": "Видео по референсу", "Short-form Video": "Короткое видео", Audio: "Аудио", "Image to Image": "Изображение в изображение", "Reference-guided Image": "Изображение по референсу", "Image Editing": "Редактирование изображений", "Video to Audio": "Видео в аудио", "Speech preservation": "Сохранение речи", "Speech synthesis": "Синтез речи", "Audio generation": "Генерация аудио", "Chat and coding": "Чат и код", "Long context": "Длинный контекст", "Tool workflows": "Рабочие процессы с инструментами", Performance: "Производительность", Activity: "Активность", FAQ: "Частые вопросы", Playground: "Playground", Capabilities: "Возможности", "Related models": "Связанные модели", "Related models from the pricing catalog.": "Связанные модели из каталога цен.", "Keep exploring Flatkey": "Продолжить изучать Flatkey", "More models from {{provider}}": "Другие модели {{provider}}", "Swipe or scroll to compare": "Смахните или прокрутите для сравнения", "Live catalog model": "Модель из живого каталога", "Frequently asked questions": "Часто задаваемые вопросы", "Use the pricing section above for current Flatkey prices from our pricing API.": "Актуальные цены Flatkey из pricing API смотрите в разделе цен выше.", "The providers section shows the upstream provider names available in our model catalog.": "Раздел провайдеров показывает upstream-провайдеров, доступных в нашем каталоге.", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} доступна через Flatkey с актуальными ценами, маршрутизацией провайдеров, примерами генерации, передачей API и ссылками на связанные модели.", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} — производственная текстовая модель для чата, кода, длинного контекста и рабочих процессов с инструментами через совместимый с Flatkey API.", "Reliability over the last 30 days": "Надёжность за последние 30 дней", "Measured on real Flatkey traffic, with production monitoring.": "Измерено на реальном трафике Flatkey при производственном мониторинге.", "Token volume and request traffic for this model over time.": "Объём токенов и трафик запросов этой модели во времени.", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Производительность использует телеметрию запросов Flatkey за последние 30 дней, когда трафика достаточно.", "Avg. provider uptime": "Средняя доступность провайдера", Latency: "Задержка", Requests: "Запросы", Uptime: "Доступность", "30-day window": "Период 30 дней", "last 30 days": "последние 30 дней", "Successful inference trend": "Тренд успешных инференсов", "Not enough data yet": "Пока недостаточно данных", "API Gateway": "API-шлюз", "Supported endpoint coverage in our pricing API.": "Поддерживаемые эндпоинты в нашем pricing API.", Docs: "Документация", "SDK for developers": "SDK для разработчиков", "Flatkey CLI": "Flatkey CLI", "Codex & Claude Code": "Codex и Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Вызывайте эту модель через тот же совместимый с OpenAI роутер и API-ключ, что и остальные модели Flatkey.", "Use your existing SDK and set its base URL to the Flatkey router origin.": "Используйте существующий SDK и укажите в нём базовый URL роутера Flatkey.", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Управляйте промптами и файлами запросов в терминале с помощью Flatkey CLI.", "Use the same key with your coding agent and route model jobs from a script.": "Используйте тот же ключ в coding-агенте и маршрутизируйте задачи скриптом.", "Request preview": "Предпросмотр запроса", "Copy request": "Скопировать запрос", "Why Flatkey": "Почему Flatkey", "Why use Flatkey for {{model}}?": "Зачем использовать Flatkey для {{model}}?", "Lower generation pricing": "Более низкая цена генерации", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "Маршрутизируйте медиазадачи через Flatkey и снизьте стоимость тестирования промптов до масштабирования.", "Draft handoff": "Передача черновика", "The public page stores prompt settings locally before sending the user into Flatkey.": "Публичная страница сохраняет настройки промпта локально перед переходом в Flatkey.", "Unified API access": "Единый доступ к API", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "Управляйте расходом, ключами, квотами и маршрутизацией в одном аккаунте Flatkey.", "Routing across upstream channels": "Маршрутизация через upstream-каналы", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Запросы распределяются между каналами этой модели; доступность за последние 30 дней показана выше.", "Core capabilities and practical engineering value": "Ключевые возможности и практическая инженерная ценность", "OpenAI-compatible migration path": "Путь миграции, совместимый с OpenAI", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Тела запросов в стиле Chat Completions упрощают переход с существующего стека.", "Structured and tool-based output": "Структурированный вывод и инструменты", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "Используйте структурированный JSON, инструменты и генерацию кода в агентских процессах.", "Streaming interaction": "Потоковое взаимодействие", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "Потоковая выдача подходит для чатов, терминальных помощников и постепенного рендеринга.", "Production routing": "Маршрутизация в продакшене", "Price / image": "Цена / изображение", "Price / second": "Цена / секунда", "Request price": "Цена запроса", "/ image": "/ изображение", "/ second": "/ секунда", "/ request": "/ запрос",
  },
  ja: {
    "Back to Models": "モデル一覧に戻る", "Copy model id": "モデル ID をコピー", "Get API Key": "API キーを取得", "Flatkey price": "Flatkey 料金", "Reference price": "参考料金", Provider: "プロバイダー", Input: "入力", Output: "出力", Preview: "プレビュー", "Open in Playground": "Playground で開く", "Generator setup": "生成設定", "Playground (edit before sign-up)": "Playground（登録前に編集可）", "Start generating": "生成を開始", "Request summary": "リクエスト概要", "Preview only. Sign in to submit this request to Flatkey.": "プレビューのみです。Flatkey に送信するにはログインしてください。", "Quick Prompts": "クイックプロンプト", "Advanced Options": "詳細設定", "Upload or drag and drop": "アップロードまたはドラッグ＆ドロップ", "Reference image": "参照画像", "Reference videos": "参照動画", "Reference Audios": "参照音声", "Reference Images": "参照画像", "Upload reference": "参照をアップロード", Images: "画像数", Size: "サイズ", Quality: "品質", Background: "背景", Moderation: "モデレーション", Resolution: "解像度", "Aspect ratio": "アスペクト比", Duration: "長さ", "Output format": "出力形式", Outputs: "出力数", "Video URL": "動画 URL", "Preserve speech": "音声を保持", "Generate audio": "音声を生成", "AIGC watermark": "AIGC ウォーターマーク", Frames: "フレーム数", "Camera fixed": "カメラ固定", "Return last frame": "最後のフレームを返す", Seed: "シード", "Optional frame count override": "任意のフレーム数上書き", "0 means random": "0 はランダム", On: "オン", Off: "オフ", None: "なし", "Model ID": "モデル ID", Capability: "機能", Modalities: "モダリティ", Context: "コンテキスト", "Best for": "おすすめ用途", Text: "テキスト", "Text to Video": "テキストから動画", "Text to Image": "テキストから画像", "Image to Video": "画像から動画", "Reference-guided Video": "参照画像ガイド動画", "Short-form Video": "短尺動画", Audio: "音声", "Image to Image": "画像から画像", "Reference-guided Image": "参照画像ガイド", "Image Editing": "画像編集", "Video to Audio": "動画から音声", "Speech preservation": "音声の保持", "Speech synthesis": "音声合成", "Audio generation": "音声生成", "Chat and coding": "チャットとコーディング", "Long context": "長いコンテキスト", "Tool workflows": "ツールワークフロー", Performance: "パフォーマンス", Activity: "アクティビティ", FAQ: "FAQ", Playground: "Playground", Capabilities: "機能", "Related models": "関連モデル", "Related models from the pricing catalog.": "料金カタログの関連モデル。", "Keep exploring Flatkey": "Flatkey をさらに見る", "More models from {{provider}}": "{{provider}} の他のモデル", "Swipe or scroll to compare": "スワイプまたはスクロールして比較", "Live catalog model": "ライブカタログモデル", "Frequently asked questions": "よくある質問", "Use the pricing section above for current Flatkey prices from our pricing API.": "現在の Flatkey 料金は、上の料金セクションで料金 API から確認できます。", "The providers section shows the upstream provider names available in our model catalog.": "プロバイダーセクションには、モデルカタログで利用可能な upstream プロバイダー名が表示されます。", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} は Flatkey で利用でき、ライブ料金、プロバイダールーティング、生成例、API 引き継ぎ、関連モデルリンクを提供します。", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} はチャット、コーディング、長いコンテキスト推論、ツールワークフローに対応する本番向けテキストモデルです。", "Reliability over the last 30 days": "過去30日間の信頼性", "Measured on real Flatkey traffic, with production monitoring.": "実際の Flatkey トラフィックを本番監視で計測しています。", "Token volume and request traffic for this model over time.": "このモデルの token 量とリクエストトラフィックの推移。", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "十分なトラフィックがある場合、パフォーマンスは過去 30 日の Flatkey リクエストテレメトリを使用します。", "Avg. provider uptime": "平均プロバイダー稼働率", Latency: "レイテンシ", Requests: "リクエスト", Uptime: "稼働率", "30-day window": "30日間", "last 30 days": "過去30日", "Successful inference trend": "成功した推論の推移", "Not enough data yet": "まだ十分なデータがありません", "API Gateway": "API ゲートウェイ", "Supported endpoint coverage in our pricing API.": "料金 API でサポートされるエンドポイント範囲。", Docs: "ドキュメント", "SDK for developers": "開発者向け SDK", "Flatkey CLI": "Flatkey CLI", "Codex & Claude Code": "Codex と Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Flatkey カタログの他モデルと同じ OpenAI 互換ルーターと API キーで呼び出せます。", "Use your existing SDK and set its base URL to the Flatkey router origin.": "既存の SDK を使い、base URL を Flatkey ルーターのオリジンに設定してください。", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Flatkey CLI でプロンプトとリクエストファイルをターミナルで管理できます。", "Use the same key with your coding agent and route model jobs from a script.": "同じキーをコーディングエージェントで使い、スクリプトからモデル処理をルーティングできます。", "Request preview": "リクエストプレビュー", "Copy request": "リクエストをコピー", "Why Flatkey": "Flatkey を選ぶ理由", "Why use Flatkey for {{model}}?": "なぜ {{model}} に Flatkey を使うのですか？", "Lower generation pricing": "低い生成料金", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "メディア処理を Flatkey 経由にして、拡張前のプロンプトテスト費用を抑えます。", "Draft handoff": "下書きの引き継ぎ", "The public page stores prompt settings locally before sending the user into Flatkey.": "公開ページは Flatkey へ移動する前にプロンプト設定をローカル保存します。", "Unified API access": "統合 API アクセス", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "使用量、キー、クォータ、モデルルーティングを 1 つの Flatkey アカウントで管理できます。", "Routing across upstream channels": "アップストリームチャネルのルーティング", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "リクエストはこのモデルを提供する利用可能なチャネルに分散され、過去 30 日の稼働率を上に表示します。", "Core capabilities and practical engineering value": "主な機能と実践的なエンジニアリング価値", "OpenAI-compatible migration path": "OpenAI 互換の移行パス", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Chat Completions 形式のペイロードで既存のモデルスタックから移行しやすくなります。", "Structured and tool-based output": "構造化出力とツール連携", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "構造化 JSON、ツール、コード生成フローをエージェントワークフローで利用できます。", "Streaming interaction": "ストリーミング操作", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "ストリーミングはチャット UI、ターミナルアシスタント、段階的なレンダリングに対応します。", "Production routing": "本番ルーティング", "Price / image": "料金 / 画像", "Price / second": "料金 / 秒", "Request price": "リクエスト料金", "/ image": "/ 画像", "/ second": "/ 秒", "/ request": "/ リクエスト",
  },
  vi: {
    "Back to Models": "Quay lại danh sách mô hình", "Copy model id": "Sao chép ID model", "Get API Key": "Lấy khóa API", "Flatkey price": "Giá Flatkey", "Reference price": "Giá tham chiếu", Provider: "Nhà cung cấp", Input: "Đầu vào", Output: "Đầu ra", Preview: "Xem trước", "Open in Playground": "Mở trong Playground", "Generator setup": "Thiết lập trình tạo", "Playground (edit before sign-up)": "Playground (chỉnh sửa trước khi đăng ký)", "Start generating": "Bắt đầu tạo", "Request summary": "Tóm tắt yêu cầu", "Preview only. Sign in to submit this request to Flatkey.": "Chỉ xem trước. Đăng nhập để gửi yêu cầu đến Flatkey.", "Quick Prompts": "Prompt nhanh", "Advanced Options": "Tùy chọn nâng cao", "Upload or drag and drop": "Tải lên hoặc kéo thả", "Reference image": "Ảnh tham chiếu", "Reference videos": "Video tham chiếu", "Reference Audios": "Âm thanh tham chiếu", "Reference Images": "Ảnh tham chiếu", "Upload reference": "Tải tham chiếu lên", Images: "Số ảnh", Size: "Kích thước", Quality: "Chất lượng", Background: "Nền", Moderation: "Kiểm duyệt", Resolution: "Độ phân giải", "Aspect ratio": "Tỷ lệ khung hình", Duration: "Thời lượng", "Output format": "Định dạng đầu ra", Outputs: "Số đầu ra", "Video URL": "URL video", "Preserve speech": "Giữ giọng nói", "Generate audio": "Tạo âm thanh", "AIGC watermark": "Watermark AIGC", Frames: "Số khung hình", "Camera fixed": "Cố định camera", "Return last frame": "Trả về khung hình cuối", Seed: "Seed", "Optional frame count override": "Ghi đè số khung hình tùy chọn", "0 means random": "0 nghĩa là ngẫu nhiên", On: "Bật", Off: "Tắt", None: "Không có", "Model ID": "ID model", Capability: "Khả năng", Modalities: "Phương thức", Context: "Ngữ cảnh", "Best for": "Phù hợp nhất cho", Text: "Văn bản", "Text to Video": "Văn bản thành video", "Text to Image": "Văn bản thành ảnh", "Image to Video": "Ảnh thành video", "Reference-guided Video": "Video theo tham chiếu", "Short-form Video": "Video ngắn", Audio: "Âm thanh", "Image to Image": "Ảnh thành ảnh", "Reference-guided Image": "Ảnh theo tham chiếu", "Image Editing": "Chỉnh sửa ảnh", "Video to Audio": "Video sang âm thanh", "Speech preservation": "Bảo toàn lời nói", "Speech synthesis": "Tổng hợp giọng nói", "Audio generation": "Tạo âm thanh", "Chat and coding": "Trò chuyện và lập trình", "Long context": "Ngữ cảnh dài", "Tool workflows": "Quy trình với công cụ", Performance: "Hiệu năng", Activity: "Hoạt động", FAQ: "Câu hỏi thường gặp", Playground: "Playground", Capabilities: "Khả năng", "Related models": "Model liên quan", "Related models from the pricing catalog.": "Model liên quan từ danh mục giá.", "Keep exploring Flatkey": "Tiếp tục khám phá Flatkey", "More models from {{provider}}": "Các model khác của {{provider}}", "Swipe or scroll to compare": "Vuốt hoặc cuộn để so sánh", "Live catalog model": "Model trong danh mục trực tiếp", "Frequently asked questions": "Câu hỏi thường gặp", "Use the pricing section above for current Flatkey prices from our pricing API.": "Xem phần giá bên trên để biết giá Flatkey hiện tại từ API giá.", "The providers section shows the upstream provider names available in our model catalog.": "Phần nhà cung cấp hiển thị tên upstream có trong danh mục model.", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} có trên Flatkey với giá trực tiếp, định tuyến nhà cung cấp, ví dụ tạo nội dung, handoff API và liên kết model liên quan.", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} là model văn bản production cho trò chuyện, lập trình, suy luận ngữ cảnh dài và quy trình với công cụ qua API tương thích Flatkey.", "Reliability over the last 30 days": "Độ tin cậy trong 30 ngày qua", "Measured on real Flatkey traffic, with production monitoring.": "Được đo trên lưu lượng Flatkey thực tế cùng giám sát production.", "Token volume and request traffic for this model over time.": "Khối lượng token và lưu lượng request của model này theo thời gian.", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Hiệu năng dùng telemetry request Flatkey trong 30 ngày gần nhất khi có đủ lưu lượng.", "Avg. provider uptime": "Uptime trung bình của nhà cung cấp", Latency: "Độ trễ", Requests: "Lượt gọi", Uptime: "Uptime", "30-day window": "Khung 30 ngày", "last 30 days": "30 ngày qua", "Successful inference trend": "Xu hướng suy luận thành công", "Not enough data yet": "Chưa đủ dữ liệu", "API Gateway": "Cổng API", "Supported endpoint coverage in our pricing API.": "Phạm vi endpoint được hỗ trợ trong API giá.", Docs: "Tài liệu", "SDK for developers": "SDK cho nhà phát triển", "Flatkey CLI": "Flatkey CLI", "Codex & Claude Code": "Codex và Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Gọi model này qua router tương thích OpenAI và khóa API giống các model khác trong danh mục Flatkey.", "Use your existing SDK and set its base URL to the Flatkey router origin.": "Dùng SDK hiện có và đặt base URL tới router Flatkey.", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Quản lý prompt và file request trong terminal bằng Flatkey CLI.", "Use the same key with your coding agent and route model jobs from a script.": "Dùng cùng khóa với coding agent và định tuyến tác vụ model từ script.", "Request preview": "Xem trước yêu cầu", "Copy request": "Sao chép yêu cầu", "Why Flatkey": "Vì sao chọn Flatkey", "Why use Flatkey for {{model}}?": "Vì sao dùng Flatkey cho {{model}}?", "Lower generation pricing": "Giá tạo thấp hơn", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "Định tuyến workload media qua Flatkey để giảm chi phí thử prompt trước khi mở rộng.", "Draft handoff": "Bàn giao bản nháp", "The public page stores prompt settings locally before sending the user into Flatkey.": "Trang công khai lưu cài đặt prompt cục bộ trước khi đưa bạn sang Flatkey.", "Unified API access": "Truy cập API thống nhất", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "Quản lý usage, khóa, quota và định tuyến model trong một tài khoản Flatkey.", "Routing across upstream channels": "Định tuyến qua các kênh upstream", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Request được phân bổ qua các kênh phục vụ model này; uptime 30 ngày hiển thị ở trên.", "Core capabilities and practical engineering value": "Năng lực cốt lõi và giá trị kỹ thuật thực tế", "OpenAI-compatible migration path": "Lộ trình chuyển đổi tương thích OpenAI", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Payload kiểu Chat Completions giảm ma sát khi chuyển từ stack hiện có.", "Structured and tool-based output": "Đầu ra có cấu trúc và dựa trên công cụ", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "Dùng JSON có cấu trúc, công cụ và quy trình tạo mã cho agent.", "Streaming interaction": "Tương tác streaming", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "Streaming hỗ trợ UI chat, trợ lý terminal và render từng phần.", "Production routing": "Định tuyến production", "Price / image": "Giá / ảnh", "Price / second": "Giá / giây", "Request price": "Giá mỗi request", "/ image": "/ ảnh", "/ second": "/ giây", "/ request": "/ request",
  },
  de: {
    "Back to Models": "Zurück zu den Modellen", "Copy model id": "Modell-ID kopieren", "Get API Key": "API-Key abrufen", "Flatkey price": "Flatkey-Preis", "Reference price": "Referenzpreis", Provider: "Anbieter", Input: "Eingabe", Output: "Ausgabe", Preview: "Vorschau", "Open in Playground": "Im Playground öffnen", "Generator setup": "Generator-Einstellungen", "Playground (edit before sign-up)": "Playground (vor der Anmeldung bearbeiten)", "Start generating": "Generierung starten", "Request summary": "Anfrageübersicht", "Preview only. Sign in to submit this request to Flatkey.": "Nur Vorschau. Melden Sie sich an, um diese Anfrage an Flatkey zu senden.", "Quick Prompts": "Schnell-Prompts", "Advanced Options": "Erweiterte Optionen", "Upload or drag and drop": "Hochladen oder per Drag-and-drop ablegen", "Reference image": "Referenzbild", "Reference videos": "Referenzvideos", "Reference Audios": "Referenzaudio", "Reference Images": "Referenzbilder", "Upload reference": "Referenz hochladen", Images: "Bilder", Size: "Größe", Quality: "Qualität", Background: "Hintergrund", Moderation: "Moderation", Resolution: "Auflösung", "Aspect ratio": "Seitenverhältnis", Duration: "Dauer", "Output format": "Ausgabeformat", Outputs: "Ausgaben", "Video URL": "Video-URL", "Preserve speech": "Sprache erhalten", "Generate audio": "Audio generieren", "AIGC watermark": "AIGC-Wasserzeichen", Frames: "Frames", "Camera fixed": "Kamera fixieren", "Return last frame": "Letzten Frame zurückgeben", Seed: "Seed", "Optional frame count override": "Optionale Frame-Anzahl", "0 means random": "0 bedeutet zufällig", On: "An", Off: "Aus", None: "Keine", "Model ID": "Modell-ID", Capability: "Fähigkeit", Modalities: "Modalitäten", Context: "Kontext", "Best for": "Ideal für", Text: "Text", "Text to Video": "Text zu Video", "Text to Image": "Text zu Bild", "Image to Video": "Bild zu Video", "Reference-guided Video": "Referenzgesteuertes Video", "Short-form Video": "Kurzvideo", Audio: "Audio", "Image to Image": "Bild zu Bild", "Reference-guided Image": "Referenzgesteuertes Bild", "Image Editing": "Bildbearbeitung", "Video to Audio": "Video zu Audio", "Speech preservation": "Spracherhalt", "Speech synthesis": "Sprachsynthese", "Audio generation": "Audiogenerierung", "Chat and coding": "Chat und Coding", "Long context": "Langer Kontext", "Tool workflows": "Tool-Workflows", Performance: "Performance", Activity: "Aktivität", FAQ: "Häufige Fragen", Playground: "Playground", Capabilities: "Funktionen", "Related models": "Verwandte Modelle", "Related models from the pricing catalog.": "Verwandte Modelle aus dem Preiskatalog.", "Keep exploring Flatkey": "Flatkey weiter erkunden", "More models from {{provider}}": "Weitere Modelle von {{provider}}", "Swipe or scroll to compare": "Zum Vergleichen wischen oder scrollen", "Live catalog model": "Live-Katalogmodell", "Frequently asked questions": "Häufig gestellte Fragen", "Use the pricing section above for current Flatkey prices from our pricing API.": "Die aktuellen Flatkey-Preise aus unserer Pricing API finden Sie im Preisbereich oben.", "The providers section shows the upstream provider names available in our model catalog.": "Der Anbieterbereich zeigt die Upstream-Anbieter, die in unserem Modellkatalog verfügbar sind.", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} ist über Flatkey mit Live-Preisen, Anbieter-Routing, Generierungsbeispielen, API-Übergabe und Links zu verwandten Modellen verfügbar.", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} ist ein produktives Textmodell für Chat, Coding, Langkontext-Reasoning und Tool-Workflows über die Flatkey-kompatible API.", "Reliability over the last 30 days": "Zuverlässigkeit der letzten 30 Tage", "Measured on real Flatkey traffic, with production monitoring.": "Auf echtem Flatkey-Traffic mit Produktionsmonitoring gemessen.", "Token volume and request traffic for this model over time.": "Token-Volumen und Anfrage-Traffic dieses Modells im Zeitverlauf.", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Performance nutzt die Flatkey-Anfragetelemetrie der letzten 30 Tage, wenn genügend Traffic vorhanden ist.", "Avg. provider uptime": "Durchschn. Anbieter-Verfügbarkeit", Latency: "Latenz", Requests: "Anfragen", Uptime: "Verfügbarkeit", "30-day window": "30-Tage-Fenster", "last 30 days": "letzte 30 Tage", "Successful inference trend": "Trend erfolgreicher Inferenzen", "Not enough data yet": "Noch nicht genug Daten", "API Gateway": "API-Gateway", "Supported endpoint coverage in our pricing API.": "Unterstützte Endpoints in unserer Pricing API.", Docs: "Dokumentation", "SDK for developers": "SDK für Entwickler", "Flatkey CLI": "Flatkey CLI", "Codex & Claude Code": "Codex und Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Rufen Sie dieses Modell über denselben OpenAI-kompatiblen Router und API-Key wie den restlichen Flatkey-Katalog auf.", "Use your existing SDK and set its base URL to the Flatkey router origin.": "Verwenden Sie Ihr vorhandenes SDK und setzen Sie dessen Basis-URL auf den Flatkey-Router.", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Verwalten Sie Prompts und Anfrage-Dateien mit der Flatkey CLI im Terminal.", "Use the same key with your coding agent and route model jobs from a script.": "Verwenden Sie denselben Schlüssel mit Ihrem Coding-Agent und routen Sie Modellaufgaben per Skript.", "Request preview": "Anfragevorschau", "Copy request": "Anfrage kopieren", "Why Flatkey": "Warum Flatkey", "Why use Flatkey for {{model}}?": "Warum Flatkey für {{model}} nutzen?", "Lower generation pricing": "Niedrigere Generierungskosten", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "Routen Sie Medien-Workloads über Flatkey und halten Sie Prompt-Tests vor der Skalierung günstiger.", "Draft handoff": "Entwurfsübergabe", "The public page stores prompt settings locally before sending the user into Flatkey.": "Die öffentliche Seite speichert Prompt-Einstellungen lokal, bevor sie zu Flatkey weiterleitet.", "Unified API access": "Einheitlicher API-Zugriff", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "Verwalten Sie Nutzung, Schlüssel, Quoten und Modellrouting in einem Flatkey-Konto.", "Routing across upstream channels": "Routing über Upstream-Kanäle", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Anfragen werden auf verfügbare Kanäle für dieses Modell verteilt; die Verfügbarkeit der letzten 30 Tage steht oben.", "Core capabilities and practical engineering value": "Kernfunktionen und praktischer Engineering-Nutzen", "OpenAI-compatible migration path": "OpenAI-kompatibler Migrationspfad", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Payloads im Chat-Completions-Stil erleichtern die Migration aus bestehenden Model-Stacks.", "Structured and tool-based output": "Strukturierte und toolbasierte Ausgabe", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "Nutzen Sie strukturiertes JSON, Tools und Code-Generierungsabläufe für Agent-Workflows.", "Streaming interaction": "Streaming-Interaktion", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "Streaming unterstützt Chat-UIs, Terminal-Assistenten und progressives Rendering.", "Production routing": "Produktionsrouting", "Price / image": "Preis / Bild", "Price / second": "Preis / Sekunde", "Request price": "Preis pro Anfrage", "/ image": "/ Bild", "/ second": "/ Sekunde", "/ request": "/ Anfrage",
  },
  id: {
    "Back to Models": "Kembali ke model", "Copy model id": "Salin ID model", "Get API Key": "Dapatkan kunci API", "Flatkey price": "Harga Flatkey", "Reference price": "Harga referensi", Provider: "Penyedia", Input: "Input", Output: "Output", Preview: "Pratinjau", "Open in Playground": "Buka di Playground", "Generator setup": "Penyiapan generator", "Playground (edit before sign-up)": "Playground (edit sebelum mendaftar)", "Start generating": "Mulai membuat", "Request summary": "Ringkasan permintaan", "Preview only. Sign in to submit this request to Flatkey.": "Hanya pratinjau. Masuk untuk mengirim permintaan ini ke Flatkey.", "Quick Prompts": "Prompt cepat", "Advanced Options": "Opsi lanjutan", "Upload or drag and drop": "Unggah atau seret dan lepas", "Reference image": "Gambar referensi", "Reference videos": "Video referensi", "Reference Audios": "Audio referensi", "Reference Images": "Gambar referensi", "Upload reference": "Unggah referensi", Images: "Jumlah gambar", Size: "Ukuran", Quality: "Kualitas", Background: "Latar", Moderation: "Moderasi", Resolution: "Resolusi", "Aspect ratio": "Rasio aspek", Duration: "Durasi", "Output format": "Format output", Outputs: "Output", "Video URL": "URL video", "Preserve speech": "Pertahankan ucapan", "Generate audio": "Buat audio", "AIGC watermark": "Watermark AIGC", Frames: "Frame", "Camera fixed": "Kamera tetap", "Return last frame": "Kembalikan frame terakhir", Seed: "Seed", "Optional frame count override": "Jumlah frame opsional", "0 means random": "0 berarti acak", On: "Aktif", Off: "Nonaktif", None: "Tidak ada", "Model ID": "ID model", Capability: "Kemampuan", Modalities: "Modalitas", Context: "Konteks", "Best for": "Cocok untuk", Text: "Teks", "Text to Video": "Teks menjadi video", "Text to Image": "Teks menjadi gambar", "Image to Video": "Gambar menjadi video", "Reference-guided Video": "Video terpandu referensi", "Short-form Video": "Video pendek", Audio: "Audio", "Image to Image": "Gambar menjadi gambar", "Reference-guided Image": "Gambar terpandu referensi", "Image Editing": "Pengeditan gambar", "Video to Audio": "Video ke audio", "Speech preservation": "Pelestarian ucapan", "Speech synthesis": "Sintesis ucapan", "Audio generation": "Pembuatan audio", "Chat and coding": "Chat dan coding", "Long context": "Konteks panjang", "Tool workflows": "Alur kerja tool", Performance: "Performa", Activity: "Aktivitas", FAQ: "FAQ", Playground: "Playground", Capabilities: "Kemampuan", "Related models": "Model terkait", "Related models from the pricing catalog.": "Model terkait dari katalog harga.", "Keep exploring Flatkey": "Jelajahi Flatkey", "More models from {{provider}}": "Model lain dari {{provider}}", "Swipe or scroll to compare": "Geser atau gulir untuk membandingkan", "Live catalog model": "Model dari katalog langsung", "Frequently asked questions": "Pertanyaan umum", "Use the pricing section above for current Flatkey prices from our pricing API.": "Gunakan bagian harga di atas untuk melihat harga Flatkey terbaru dari API harga kami.", "The providers section shows the upstream provider names available in our model catalog.": "Bagian penyedia menampilkan nama upstream yang tersedia di katalog model kami.", "{{model}} is available through Flatkey with live pricing, provider routing, generation examples, API handoff, and related model links.": "{{model}} tersedia melalui Flatkey dengan harga live, routing penyedia, contoh generasi, handoff API, dan tautan model terkait.", "{{model}} is a production text model for chat, coding, long-context reasoning, and tool-enabled workflows through Flatkey-compatible API access.": "{{model}} adalah model teks produksi untuk chat, coding, penalaran konteks panjang, dan alur kerja tool melalui API yang kompatibel dengan Flatkey.", "Reliability over the last 30 days": "Keandalan 30 hari terakhir", "Measured on real Flatkey traffic, with production monitoring.": "Diukur pada trafik Flatkey nyata dengan pemantauan produksi.", "Token volume and request traffic for this model over time.": "Volume token dan trafik permintaan untuk model ini dari waktu ke waktu.", "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.": "Performa menggunakan telemetri permintaan Flatkey dari 30 hari terakhir saat trafik cukup.", "Avg. provider uptime": "Rata-rata uptime penyedia", Latency: "Latensi", Requests: "Permintaan", Uptime: "Uptime", "30-day window": "Jendela 30 hari", "last 30 days": "30 hari terakhir", "Successful inference trend": "Tren inferensi berhasil", "Not enough data yet": "Data belum cukup", "API Gateway": "Gateway API", "Supported endpoint coverage in our pricing API.": "Cakupan endpoint yang didukung di API harga kami.", Docs: "Dokumentasi", "SDK for developers": "SDK untuk pengembang", "Flatkey CLI": "Flatkey CLI", "Codex & Claude Code": "Codex dan Claude Code", "Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.": "Panggil model ini melalui router kompatibel OpenAI dan kunci API yang sama seperti katalog Flatkey lainnya.", "Use your existing SDK and set its base URL to the Flatkey router origin.": "Gunakan SDK yang ada dan atur base URL ke origin router Flatkey.", "Keep prompts and request files in your terminal workflow with the Flatkey CLI.": "Kelola prompt dan file permintaan di terminal dengan Flatkey CLI.", "Use the same key with your coding agent and route model jobs from a script.": "Gunakan kunci yang sama dengan coding agent dan rutekan tugas model dari skrip.", "Request preview": "Pratinjau permintaan", "Copy request": "Salin permintaan", "Why Flatkey": "Mengapa Flatkey", "Why use Flatkey for {{model}}?": "Mengapa menggunakan Flatkey untuk {{model}}?", "Lower generation pricing": "Harga generasi lebih rendah", "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.": "Rutekan workload media melalui Flatkey agar pengujian prompt tetap lebih murah sebelum penskalaan.", "Draft handoff": "Serah terima draf", "The public page stores prompt settings locally before sending the user into Flatkey.": "Halaman publik menyimpan pengaturan prompt secara lokal sebelum mengarahkan Anda ke Flatkey.", "Unified API access": "Akses API terpadu", "Keep usage, keys, quotas, and model routing in one Flatkey account.": "Kelola penggunaan, kunci, kuota, dan routing model dalam satu akun Flatkey.", "Routing across upstream channels": "Routing melalui kanal upstream", "Requests are spread across the channels serving this model, with 30-day uptime published above.": "Permintaan didistribusikan ke kanal yang melayani model ini; uptime 30 hari ditampilkan di atas.", "Core capabilities and practical engineering value": "Kemampuan inti dan nilai engineering praktis", "OpenAI-compatible migration path": "Jalur migrasi kompatibel OpenAI", "Chat Completions-style payloads reduce switching friction from existing model stacks.": "Payload bergaya Chat Completions mengurangi hambatan saat berpindah dari stack model yang ada.", "Structured and tool-based output": "Output terstruktur dan berbasis tool", "Use structured JSON, tools, and code-generation flows for agentic workflows.": "Gunakan JSON terstruktur, tool, dan alur pembuatan kode untuk workflow agent.", "Streaming interaction": "Interaksi streaming", "Streaming supports chat UIs, terminal assistants, and progressive rendering.": "Streaming mendukung UI chat, asisten terminal, dan rendering progresif.", "Production routing": "Routing produksi", "Price / image": "Harga / gambar", "Price / second": "Harga / detik", "Request price": "Harga per permintaan", "/ image": "/ gambar", "/ second": "/ detik", "/ request": "/ permintaan",
  },
};

// A small set of labels is shared by the media prompt editor and request
// preview, but does not belong to the legacy catalog copy table. Keep these
// values here so quick prompts and action labels never fall back to English
// on localized model pages.
const modelDetailUiAdditions: Partial<Record<Locale, Record<string, string>>> = {
  en: {
    "Add credits": "Add credits",
  },
  zh: {
    "Add credits": "充值余额",
    vs: "对比",
    "Flatkey Router": "Flatkey 路由器",
    Endpoint: "接口",
    Prompt: "提示词",
    "Product Reveal": "产品展示",
    "UGC Ad": "UGC 广告",
    "Cinematic Scene": "电影感场景",
    "Social Clip": "社媒短片",
    "Product Photo": "产品摄影",
    "Anime Portrait": "动漫头像",
    "Realistic Human": "真人写实",
    "YouTube Thumbnail": "YouTube 缩略图",
    "Fantasy Landscape": "奇幻风景",
    "Copy Prompt": "复制提示词",
    "Make one like this": "做一个类似的",
    "Long-context work": "长上下文任务",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "适合文档总结、代码库分析和知识工作流。",
    "Coding and technical generation": "代码与技术生成",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "适合代码解释、测试、重构、SDK 封装和技术文档初稿。",
    "Use one account and API key across image, video, audio, and language models.": "一个账号和 API Key 覆盖图片、视频、音频和语言模型。",
    "Product visuals": "产品视觉",
    "Ads and social creative": "广告与社媒创意",
    "Developer pipelines": "开发者流水线",
  },
  es: {
    "Add credits": "Añadir crédito",
    vs: "frente a",
    "Flatkey Router": "Router de Flatkey",
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "Configura una solicitud de {{model}} en la página pública. Flatkey guarda el borrador localmente y abre la consola para que puedas ejecutarla con tu cuenta y clave API.",
    "Model Type": "Tipo de modelo",
    Form: "Formulario",
    "Join and run": "Regístrate y ejecuta",
    "View API Docs": "Ver documentación de API",
    "Avg. response time": "Tiempo de respuesta medio",
    "Ready for production": "Lista para producción",
    "Generated Examples": "Ejemplos generados",
    "Explore what {{model}} can create": "Descubre qué puede crear {{model}}",
    "Create with this model": "Crear con este modelo",
    "Transparent Pricing": "Precios transparentes",
    "Flatkey {{model}} usage pricing": "Precios de uso de {{model}} en Flatkey",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "Usa el mismo saldo y la misma clave API de Flatkey para modelos de imagen, vídeo, audio y texto.",
    "Open wallet": "Abrir cartera",
    "Start generating in three steps": "Empieza a generar en tres pasos",
    "Try a prompt": "Prueba un prompt",
    "Use the playground to validate quality and style fit.": "Usa el playground para validar la calidad y el encaje del estilo.",
    "Create an API key": "Crea una clave API",
    "Sign up, open Dashboard, and create a token for this model.": "Regístrate, abre el Dashboard y crea un token para este modelo.",
    "Ship your workflow": "Pon tu flujo en producción",
    "Call the same endpoint, then top up credits as usage grows.": "Llama al mismo endpoint y añade crédito a medida que crezca el uso.",
    "Built for generation teams": "Diseñado para equipos de generación",
    "{{model}} pricing FAQ": "Preguntas frecuentes sobre precios de {{model}}",
    "Generate your first {{model}} on Flatkey": "Genera tu primer {{model}} en Flatkey",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "Guarda el borrador; regístrate si hace falta o abre directamente la consola si ya has iniciado sesión.",
    "Model Guide": "Guía del modelo",
    Updated: "Actualizado",
    "Model Overview": "Descripción general del modelo",
    "Best for chat, code generation, agent workflows, and production assistants.": "Ideal para chat, generación de código, flujos con agentes y asistentes de producción.",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "Usa Flatkey para obtener enrutamiento compatible con OpenAI, facturación unificada y claves API reutilizables.",
    "How to Use {{model}} API": "Cómo usar la API de {{model}}",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "Crea una clave API y establece Authorization: Bearer <YOUR_API_KEY>.",
    "POST to /v1/chat/completions with at least model and messages.": "Envía un POST a /v1/chat/completions con al menos model y messages.",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "Ajusta max_tokens, temperature y top_p según la complejidad de la tarea.",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "Activa el streaming para interfaces de chat, asistentes de terminal y flujos de agentes.",
    "Use logs and retries to refine prompts before broader rollout.": "Usa registros y reintentos para perfeccionar los prompts antes de ampliar el despliegue.",
    "Common Errors": "Errores comunes",
    "Missing required fields, malformed messages, or unsupported parameter values.": "Faltan campos obligatorios, los mensajes tienen un formato incorrecto o hay parámetros no compatibles.",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "Falta el encabezado Authorization, el token Bearer tiene un formato incorrecto o la clave API no es válida.",
    "Request rate, concurrency, or quota is above current account limits.": "La tasa de solicitudes, la concurrencia o la cuota supera los límites actuales de la cuenta.",
    "Ready to unify your AI model access?": "¿Listo para unificar el acceso a tus modelos de IA?",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Usa una cuenta de Flatkey para probar prompts, comparar modelos y pasar la solicitud guardada a la consola.",
    Endpoint: "Punto final",
    Prompt: "Prompt",
    "Product Reveal": "Presentación de producto",
    "UGC Ad": "Anuncio UGC",
    "Cinematic Scene": "Escena cinematográfica",
    "Social Clip": "Clip social",
    "Product Photo": "Fotografía de producto",
    "Anime Portrait": "Retrato anime",
    "Realistic Human": "Persona realista",
    "YouTube Thumbnail": "Miniatura de YouTube",
    "Fantasy Landscape": "Paisaje fantástico",
    "Copy Prompt": "Copiar prompt",
    "Make one like this": "Crear uno similar",
    "Long-context work": "Trabajo con contexto largo",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "Útil para resumir documentos, analizar bases de código y trabajar con conocimiento.",
    "Coding and technical generation": "Generación de código y contenido técnico",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "Útil para explicar código, crear pruebas, refactorizar, generar wrappers de SDK y preparar borradores técnicos.",
    "Use one account and API key across image, video, audio, and language models.": "Usa una cuenta y una clave API para todos los modelos de imagen, vídeo, audio y lenguaje.",
    "Product visuals": "Visuales de producto",
    "Ads and social creative": "Creatividades para anuncios y redes sociales",
    "Developer pipelines": "Pipelines para desarrolladores",
  },
  fr: {
    "Add credits": "Ajouter des crédits",
    vs: "vs",
    "Flatkey Router": "Routeur Flatkey",
    "/ image": "/ image",
    "Configure a {{model}} request on the public page. Flatkey saves the draft locally, then opens the console so you can run it with your account and API key.": "Configurez une requête {{model}} sur la page publique. Flatkey enregistre le brouillon localement puis ouvre la console pour que vous puissiez l'exécuter avec votre compte et votre clé API.",
    "Model Type": "Type de modèle",
    Form: "Formulaire",
    "Join and run": "S'inscrire et exécuter",
    "View API Docs": "Voir la documentation API",
    "Avg. response time": "Temps de réponse moyen",
    "Ready for production": "Prêt pour la production",
    "Generated Examples": "Exemples générés",
    "Explore what {{model}} can create": "Découvrez ce que {{model}} peut créer",
    "Create with this model": "Créer avec ce modèle",
    "Transparent Pricing": "Tarification transparente",
    "Flatkey {{model}} usage pricing": "Tarification à l'usage de {{model}} sur Flatkey",
    "Use the same Flatkey balance and API key across image, video, audio, and text models.": "Utilisez le même solde et la même clé API Flatkey pour les modèles image, vidéo, audio et texte.",
    "Open wallet": "Ouvrir le portefeuille",
    "Start generating in three steps": "Commencer à générer en trois étapes",
    "Try a prompt": "Essayer un prompt",
    "Use the playground to validate quality and style fit.": "Utilisez le Playground pour valider la qualité et l'adéquation du style.",
    "Create an API key": "Créer une clé API",
    "Sign up, open Dashboard, and create a token for this model.": "Inscrivez-vous, ouvrez le Dashboard et créez un jeton pour ce modèle.",
    "Ship your workflow": "Mettre votre workflow en production",
    "Call the same endpoint, then top up credits as usage grows.": "Appelez le même endpoint et rechargez des crédits à mesure que l'usage augmente.",
    "Built for generation teams": "Conçu pour les équipes de génération",
    "{{model}} pricing FAQ": "FAQ sur les tarifs de {{model}}",
    "Generate your first {{model}} on Flatkey": "Générez votre premier {{model}} sur Flatkey",
    "Save the draft, continue to signup if needed, or open the console directly when already logged in.": "Enregistrez le brouillon, inscrivez-vous si nécessaire ou ouvrez directement la console si vous êtes déjà connecté.",
    "Model Guide": "Guide du modèle",
    Updated: "Mis à jour",
    "Model Overview": "Vue d'ensemble du modèle",
    "Best for chat, code generation, agent workflows, and production assistants.": "Idéal pour le chat, la génération de code, les workflows d'agents et les assistants de production.",
    "Use Flatkey when you want OpenAI-compatible routing, unified billing, and reusable API keys.": "Utilisez Flatkey pour un routage compatible OpenAI, une facturation unifiée et des clés API réutilisables.",
    "How to Use {{model}} API": "Comment utiliser l'API {{model}}",
    "Create an API key and set Authorization: Bearer <YOUR_API_KEY>.": "Créez une clé API et définissez Authorization: Bearer <YOUR_API_KEY>.",
    "POST to /v1/chat/completions with at least model and messages.": "Envoyez une requête POST à /v1/chat/completions contenant au moins model et messages.",
    "Tune max_tokens, temperature, and top_p based on task complexity.": "Ajustez max_tokens, temperature et top_p selon la complexité de la tâche.",
    "Enable streaming for chat UIs, terminal assistants, and agent workflows.": "Activez le streaming pour les interfaces de chat, les assistants de terminal et les workflows d'agents.",
    "Use logs and retries to refine prompts before broader rollout.": "Utilisez les journaux et les nouvelles tentatives pour affiner les prompts avant un déploiement plus large.",
    "Common Errors": "Erreurs courantes",
    "Missing required fields, malformed messages, or unsupported parameter values.": "Champs obligatoires manquants, messages mal formés ou valeurs de paramètres non prises en charge.",
    "Missing Authorization header, malformed bearer token, or invalid API key.": "En-tête Authorization manquant, jeton Bearer mal formé ou clé API invalide.",
    "Request rate, concurrency, or quota is above current account limits.": "Le débit des requêtes, la concurrence ou le quota dépasse les limites actuelles du compte.",
    "Ready to unify your AI model access?": "Prêt à unifier l'accès à vos modèles d'IA ?",
    "Use one Flatkey account to test prompts, compare models, and move the saved request into the console.": "Utilisez un compte Flatkey pour tester des prompts, comparer les modèles et transférer la requête enregistrée dans la console.",
    Endpoint: "Point de terminaison",
    Prompt: "Prompt",
    "Product Reveal": "Présentation du produit",
    "UGC Ad": "Publicité UGC",
    "Cinematic Scene": "Scène cinématographique",
    "Social Clip": "Clip social",
    "Product Photo": "Photo de produit",
    "Anime Portrait": "Portrait anime",
    "Realistic Human": "Personne réaliste",
    "YouTube Thumbnail": "Vignette YouTube",
    "Fantasy Landscape": "Paysage fantastique",
    "Copy Prompt": "Copier le prompt",
    "Make one like this": "En créer un similaire",
    "Long-context work": "Travail sur de longs contextes",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "Utile pour résumer des documents, analyser des bases de code et gérer des workflows de connaissance.",
    "Coding and technical generation": "Génération de code et contenu technique",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "Utile pour expliquer du code, créer des tests, refactorer, produire des wrappers SDK et rédiger des brouillons techniques.",
    "Use one account and API key across image, video, audio, and language models.": "Utilisez un seul compte et une clé API pour les modèles image, vidéo, audio et langage.",
    "Product visuals": "Visuels produit",
    "Ads and social creative": "Créations publicitaires et sociales",
    "Developer pipelines": "Pipelines développeur",
  },
  pt: {
    "Add credits": "Adicionar créditos",
    vs: "vs",
    "Flatkey Router": "Roteador Flatkey",
    Endpoint: "Endpoint",
    Prompt: "Prompt",
    "Product Reveal": "Revelação de produto",
    "UGC Ad": "Anúncio UGC",
    "Cinematic Scene": "Cena cinematográfica",
    "Social Clip": "Clipe social",
    "Product Photo": "Foto de produto",
    "Anime Portrait": "Retrato de anime",
    "Realistic Human": "Pessoa realista",
    "YouTube Thumbnail": "Miniatura do YouTube",
    "Fantasy Landscape": "Paisagem fantástica",
    "Copy Prompt": "Copiar prompt",
    "Make one like this": "Criar um parecido",
    "Long-context work": "Trabalho com contexto longo",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "Útil para resumir documentos, analisar bases de código e trabalhar com conhecimento.",
    "Coding and technical generation": "Geração de código e conteúdo técnico",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "Útil para explicar código, criar testes, fazer refatorações, gerar wrappers de SDK e preparar rascunhos técnicos.",
    "Use one account and API key across image, video, audio, and language models.": "Use uma conta e uma chave API para modelos de imagem, vídeo, áudio e linguagem.",
    "Product visuals": "Visuais de produto",
    "Ads and social creative": "Criativos para anúncios e redes sociais",
    "Developer pipelines": "Pipelines para desenvolvedores",
  },
  ru: {
    "Add credits": "Пополнить баланс",
    vs: "против",
    "Flatkey Router": "Роутер Flatkey",
    Endpoint: "Эндпоинт",
    Prompt: "Промпт",
    "Product Reveal": "Демонстрация продукта",
    "UGC Ad": "UGC-реклама",
    "Cinematic Scene": "Кинематографичная сцена",
    "Social Clip": "Социальный клип",
    "Product Photo": "Предметная съёмка",
    "Anime Portrait": "Аниме-портрет",
    "Realistic Human": "Реалистичный человек",
    "YouTube Thumbnail": "Миниатюра YouTube",
    "Fantasy Landscape": "Фантастический пейзаж",
    "Copy Prompt": "Скопировать промпт",
    "Make one like this": "Создать похожий",
    "Long-context work": "Работа с длинным контекстом",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "Подходит для суммаризации документов, анализа кодовой базы и работы со знаниями.",
    "Coding and technical generation": "Генерация кода и технического контента",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "Подходит для объяснения кода, тестов, рефакторинга, SDK-обёрток и технических черновиков.",
    "Use one account and API key across image, video, audio, and language models.": "Используйте один аккаунт и API-ключ для моделей изображений, видео, аудио и текста.",
    "Product visuals": "Визуальные материалы для продуктов",
    "Ads and social creative": "Рекламные и социальные креативы",
    "Developer pipelines": "Пайплайны для разработчиков",
  },
  ja: {
    "Add credits": "クレジットを追加",
    "Flatkey Router": "Flatkeyルーター",
    vs: "対",
    Endpoint: "エンドポイント",
    Prompt: "プロンプト",
    "Product Reveal": "商品紹介",
    "UGC Ad": "UGC 広告",
    "Cinematic Scene": "シネマティックシーン",
    "Social Clip": "ソーシャルクリップ",
    "Product Photo": "商品写真",
    "Anime Portrait": "アニメポートレート",
    "Realistic Human": "リアルな人物",
    "YouTube Thumbnail": "YouTube サムネイル",
    "Fantasy Landscape": "ファンタジー風景",
    "Copy Prompt": "プロンプトをコピー",
    "Make one like this": "似たものを作る",
    "Long-context work": "長いコンテキストの作業",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "ドキュメント要約、コードベース分析、ナレッジワークフローに役立ちます。",
    "Coding and technical generation": "コーディングと技術コンテンツ生成",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "コード解説、テスト、リファクタリング、SDK ラッパー、技術文書の下書きに役立ちます。",
    "Use one account and API key across image, video, audio, and language models.": "画像・動画・音声・言語モデルを1つのアカウントと API キーで利用できます。",
    "Product visuals": "商品ビジュアル",
    "Ads and social creative": "広告・ソーシャル向けクリエイティブ",
    "Developer pipelines": "開発者パイプライン",
  },
  vi: {
    "Add credits": "Nạp thêm tín dụng",
    vs: "so với",
    "Flatkey Router": "Router Flatkey",
    "/ request": "/ yêu cầu",
    Uptime: "Thời gian hoạt động",
    Endpoint: "Endpoint",
    Prompt: "Prompt",
    "Product Reveal": "Giới thiệu sản phẩm",
    "UGC Ad": "Quảng cáo UGC",
    "Cinematic Scene": "Cảnh điện ảnh",
    "Social Clip": "Clip mạng xã hội",
    "Product Photo": "Ảnh sản phẩm",
    "Anime Portrait": "Chân dung anime",
    "Realistic Human": "Người chân thực",
    "YouTube Thumbnail": "Thumbnail YouTube",
    "Fantasy Landscape": "Phong cảnh kỳ ảo",
    "Copy Prompt": "Sao chép prompt",
    "Make one like this": "Tạo nội dung tương tự",
    "Long-context work": "Xử lý ngữ cảnh dài",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "Hữu ích cho việc tóm tắt tài liệu, phân tích codebase và các quy trình tri thức.",
    "Coding and technical generation": "Tạo mã và nội dung kỹ thuật",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "Hữu ích cho giải thích mã, viết test, refactor, wrapper SDK và bản nháp kỹ thuật.",
    "Use one account and API key across image, video, audio, and language models.": "Dùng một tài khoản và key API cho các mô hình ảnh, video, âm thanh và ngôn ngữ.",
    "Product visuals": "Hình ảnh sản phẩm",
    "Ads and social creative": "Nội dung quảng cáo và mạng xã hội",
    "Developer pipelines": "Quy trình cho nhà phát triển",
  },
  de: {
    "Add credits": "Guthaben hinzufügen",
    vs: "gegenüber",
    "Flatkey Router": "Flatkey-Router",
    Endpoint: "Endpunkt",
    Prompt: "Prompt",
    "Product Reveal": "Produktpräsentation",
    "UGC Ad": "UGC-Anzeige",
    "Cinematic Scene": "Kinematografische Szene",
    "Social Clip": "Social-Clip",
    "Product Photo": "Produktfoto",
    "Anime Portrait": "Anime-Porträt",
    "Realistic Human": "Realistische Person",
    "YouTube Thumbnail": "YouTube-Vorschaubild",
    "Fantasy Landscape": "Fantasielandschaft",
    "Copy Prompt": "Prompt kopieren",
    "Make one like this": "Ähnliches erstellen",
    "Long-context work": "Arbeiten mit langem Kontext",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "Geeignet für Dokumentzusammenfassungen, Codebase-Analysen und Wissens-Workflows.",
    "Coding and technical generation": "Coding und technische Generierung",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "Geeignet für Code-Erklärungen, Tests, Refactorings, SDK-Wrapper und technische Entwürfe.",
    "Use one account and API key across image, video, audio, and language models.": "Nutzen Sie ein Konto und einen API-Schlüssel für Bild-, Video-, Audio- und Sprachmodelle.",
    "Product visuals": "Produktvisuals",
    "Ads and social creative": "Werbe- und Social-Creatives",
    "Developer pipelines": "Entwickler-Pipelines",
  },
  id: {
    "Add credits": "Tambah kredit",
    Input: "Masukan",
    Output: "Keluaran",
    vs: "vs",
    "Flatkey Router": "Router Flatkey",
    "Input /M": "Masukan /M",
    "Output /M": "Keluaran /M",
    Uptime: "Waktu aktif",
    "View Pricing": "Lihat harga",
    Breadcrumb: "Jejak navigasi",
    "All models": "Semua model",
    "Example {{index}} of {{total}}": "Contoh {{index}} dari {{total}}",
    "Previous example": "Contoh sebelumnya",
    "Next example": "Contoh berikutnya",
    "Generated with {{model}}": "Dibuat dengan {{model}}",
    Endpoint: "Endpoint",
    Prompt: "Prompt",
    "Product Reveal": "Perkenalan produk",
    "UGC Ad": "Iklan UGC",
    "Cinematic Scene": "Adegan sinematik",
    "Social Clip": "Klip sosial",
    "Product Photo": "Foto produk",
    "Anime Portrait": "Potret anime",
    "Realistic Human": "Manusia realistis",
    "YouTube Thumbnail": "Thumbnail YouTube",
    "Fantasy Landscape": "Lanskap fantasi",
    "Copy Prompt": "Salin prompt",
    "Make one like this": "Buat yang serupa",
    "Long-context work": "Pekerjaan dengan konteks panjang",
    "Useful for document summarization, codebase analysis, and knowledge workflows.": "Berguna untuk merangkum dokumen, menganalisis codebase, dan alur kerja pengetahuan.",
    "Coding and technical generation": "Pembuatan kode dan konten teknis",
    "Useful for code explanation, tests, refactors, SDK wrappers, and technical drafts.": "Berguna untuk menjelaskan kode, membuat pengujian, refactor, wrapper SDK, dan draf teknis.",
    "Use one account and API key across image, video, audio, and language models.": "Gunakan satu akun dan kunci API untuk model gambar, video, audio, dan bahasa.",
    "Product visuals": "Visual produk",
    "Ads and social creative": "Kreatif iklan dan media sosial",
    "Developer pipelines": "Pipeline pengembang",
    "Does this use the same model id in my SDK?": "Apakah ini menggunakan model ID yang sama di SDK saya?",
    "Yes. Keep your SDK and switch base_url plus api_key.": "Ya. Pertahankan SDK Anda dan ganti base_url serta api_key.",
    "Can I control usage before scaling?": "Bisakah saya mengontrol penggunaan sebelum melakukan penskalaan?",
    "Yes. Plan limits, usage analytics, and one invoice keep spend bounded.": "Ya. Batas paket, analitik penggunaan, dan satu tagihan membantu menjaga pengeluaran tetap terkendali.",
    "Does this start a real generation?": "Apakah ini memulai generasi sungguhan?",
    "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.": "Halaman publik menyimpan pengaturan draf terlebih dahulu. Daftar atau buka konsol untuk menjalankan permintaan dengan API key.",
    "Where are my edited prompt settings stored?": "Di mana pengaturan prompt yang saya edit disimpan?",
    "They are stored in this browser's localStorage so the draft survives the signup handoff.": "Pengaturan disimpan di localStorage browser ini agar draf tetap tersedia saat proses pendaftaran.",
  },
};

/**
 * Seedance 2.5 uses the same refreshed shell as every other model page, but
 * it also ships a richer set of editorial sections (comparison, prompt
 * library, API guide, and FAQ). Keep those strings in the model-detail copy
 * table so every supported locale renders a complete page instead of falling
 * back to the English prototype text.
 */
const seedanceModelCopy: Partial<Record<Locale, Partial<Record<ModelLandingKey, string>>>> = {
  zh: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 是一款高质量视频生成模型，支持文生视频和图生视频工作流，可生成逼真、电影感的片段，并提供原生音频和出色的提示词遵循能力。",
    "What seedance-2.5 can do": "seedance-2.5 能做什么",
    "Why Flatkey": "为什么选择 Flatkey",
    Models: "模型",
    "Video generation": "视频生成",
    "Quick Start": "快速开始",
    "Average latency": "平均延迟",
    "Task accepted; generation continues asynchronously": "任务已接收，生成将在后台异步进行",
    "Last 30 days": "最近 30 天",
    "Daily seedance-2.5 requests on Flatkey": "Flatkey 上 seedance-2.5 的每日请求",
    "Sample shape shown while live telemetry is being connected.": "实时监控接入前显示示例趋势。",
    "Total Requests": "请求总量",
    "Daily Average": "日均请求",
    "Busiest Day": "最繁忙日期",
    "Text and image to video": "文生视频与图生视频",
    "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "可以从文字场景生成，也可以用参考图驱动已经设计好的主体。",
    "Multi-shot consistency": "多镜头一致性",
    "Hold characters, wardrobe, and setting across cuts within a single generation.": "在一次生成中保持角色、服装和场景跨镜头一致。",
    "Native audio": "原生音频",
    "Ambient sound and speech are generated with the picture, in multiple languages.": "画面会同步生成环境声和多语言语音。",
    "Edit and extend": "编辑与延展",
    "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "可延续或修改已有片段，并控制首帧及首尾帧。",
    "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "了解它的能力以及相较 Seedance 2.0 的变化，再决定是否迁移。",
    "Clip length": "片段时长",
    "Short clips, stitched for longer runs": "短片段拼接成长内容",
    "Up to 30s in a single continuous shot": "单次连续镜头最长 30 秒",
    "Reference inputs": "参考输入",
    "Image references": "图片参考",
    "Up to 50 per request — 30 images, 10 videos, 10 audio": "每次请求最多 50 个：30 张图片、10 个视频、10 个音频",
    "Motion control": "运动控制",
    "Text prompt only": "仅支持文字提示词",
    "Structured motion paths, green-screen and white-model references": "支持结构化运动路径、绿幕和白模参考",
    "Generated audio": "生成音频",
    "Native audio in 10+ languages": "支持 10 多种语言的原生音频",
    "Regenerate to change a clip": "重新生成以修改片段",
    "Edit and extend an existing clip in place": "直接编辑并延展已有片段",
    "Seedance-2.5 prompts that work": "实测有效的 Seedance-2.5 提示词",
    "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "每个片段都来自真实生成。复制提示词，或载入 Playground 后继续编辑。",
    "Why run seedance-2.5 through Flatkey": "为什么通过 Flatkey 使用 seedance-2.5",
    "One key, one balance, and the same upstream model you would call directly.": "一个 Key、一个余额，调用的仍是同一个上游模型。",
    "The same upstream model, billed from one balance that also covers text, image, and audio models.": "同一个上游模型，从统一余额扣费，文本、图片和音频模型也可共用。",
    "OpenAI-compatible from day one": "从第一天起兼容 OpenAI",
    "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "将 base_url 指向 Flatkey，即可保留现有 SDK、请求结构和流式代码。",
    "Swap models without a new integration": "无需重新集成即可切换模型",
    "Move between ByteDance and every other model in the catalog by changing one string.": "只需修改一个字符串，即可在字节跳动及目录中的其他模型间切换。",
    "How to call the seedance-2.5 API": "如何调用 seedance-2.5 API",
    "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "四种接入方式共用同一个 Key 和模型目录，选择一种查看可运行示例。",
    "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "通过兼容 OpenAI 的 API 调用任意模型，复制适合你的模型和语言的示例。",
    "Use the OpenAI client and set baseURL to your Flatkey router origin.": "使用 OpenAI 客户端，并将 baseURL 设置为 Flatkey 路由地址。",
    "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "使用 Flatkey CLI 在终端工作流中管理提示词和参考文件。",
    "Codex&Claude Code": "Codex 与 Claude Code",
    "Use the same key with your coding agent and route video jobs from a script.": "在编码 Agent 中复用同一个 Key，并通过脚本路由视频任务。",
    "Related Models": "相关模型",
    "Other video generation models": "其他视频生成模型",
    "What is seedance-2.5?": "什么是 seedance-2.5？",
    "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 是一款支持文生视频和图生视频的模型，具备多镜头一致性、原生音频和编辑控制。",
    "How much does seedance-2.5 cost?": "seedance-2.5 的价格是多少？",
    "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "上方实时价格来自 Flatkey 当前价格目录，可能因账户组和输出设置而变化。",
    "What can I use it for?": "它适合用来做什么？",
    "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "可用于产品演示、社交短片、广告变体、分镜和短场景实验。",
    "How do I use the model in my app?": "如何在应用中使用该模型？",
    "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "使用与你的 Flatkey 集成其余部分相同的 API Key 和模型目录，向 /v1/videos 发送视频请求。",
    "Can I control output features?": "可以控制输出特性吗？",
    "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "可以。所选路由支持时，请求可控制画面比例、分辨率、时长、音频、参考媒体及首尾帧。",
    "Is the Flatkey API OpenAI compatible?": "Flatkey API 兼容 OpenAI 吗？",
    "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "鉴权和共享模型目录遵循 OpenAI 兼容网关模式，视频专属字段遵循 Seedance content 格式。",
    "What limits apply?": "有哪些限制？",
    "Rate limits and available model IDs depend on your account and current upstream availability.": "速率限制和可用模型 ID 取决于你的账户及当前上游可用性。",
    "What happens to my prompts and generated files?": "我的提示词和生成文件会怎样处理？",
    "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "请求会异步处理。请保存响应中的 task id，完成后从 content 接口获取结果。",
    "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "在你喜欢的编码 Agent 工作流中，复用代码示例里的环境变量和 API Key。",
    "API–frequently asked questions": "API 常见问题",
  },
  es: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 es un modelo de generación de vídeo de alta calidad para flujos de texto a vídeo e imagen a vídeo, con clips realistas, audio nativo y gran fidelidad al prompt.",
    "What seedance-2.5 can do": "Qué puede hacer seedance-2.5",
    "Why Flatkey": "Por qué Flatkey",
    Models: "Modelos", "Video generation": "Generación de vídeo", "Quick Start": "Inicio rápido", "Average latency": "Latencia media", "Task accepted; generation continues asynchronously": "Tarea aceptada; la generación continúa de forma asíncrona", "Last 30 days": "Últimos 30 días", "Daily seedance-2.5 requests on Flatkey": "Solicitudes diarias de seedance-2.5 en Flatkey", "Sample shape shown while live telemetry is being connected.": "Se muestra una tendencia de ejemplo mientras se conecta la telemetría.", "Total Requests": "Solicitudes totales", "Daily Average": "Media diaria", "Busiest Day": "Día con más actividad", "Text and image to video": "Texto e imagen a vídeo", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Genera desde una escena escrita o usa imágenes de referencia para un sujeto ya diseñado.", "Multi-shot consistency": "Consistencia entre planos", "Hold characters, wardrobe, and setting across cuts within a single generation.": "Mantén personajes, vestuario y escenario coherentes entre cortes en una sola generación.", "Native audio": "Audio nativo", "Ambient sound and speech are generated with the picture, in multiple languages.": "El ambiente y el habla se generan junto con la imagen en varios idiomas.", "Edit and extend": "Editar y ampliar", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Continúa o revisa un clip existente con control del primer fotograma y de los fotogramas inicial y final.", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Conoce sus capacidades y los cambios frente a Seedance 2.0 antes de decidir si migrar.", "Clip length": "Duración del clip", "Short clips, stitched for longer runs": "Clips cortos unidos para recorridos largos", "Up to 30s in a single continuous shot": "Hasta 30 s en un plano continuo", "Reference inputs": "Entradas de referencia", "Image references": "Imágenes de referencia", "Up to 50 per request — 30 images, 10 videos, 10 audio": "Hasta 50 por solicitud: 30 imágenes, 10 vídeos y 10 audios", "Motion control": "Control del movimiento", "Text prompt only": "Solo prompt de texto", "Structured motion paths, green-screen and white-model references": "Trayectorias estructuradas, referencias de croma verde y modelo blanco", "Generated audio": "Audio generado", "Native audio in 10+ languages": "Audio nativo en más de 10 idiomas", "Regenerate to change a clip": "Regenerar para cambiar un clip", "Edit and extend an existing clip in place": "Editar y ampliar un clip existente directamente", "Seedance-2.5 prompts that work": "Prompts de Seedance-2.5 que funcionan", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Cada clip es una generación real. Copia su prompt o cárgalo en el Playground para editarlo.", "Why run seedance-2.5 through Flatkey": "Por qué usar seedance-2.5 a través de Flatkey", "One key, one balance, and the same upstream model you would call directly.": "Una clave, un saldo y el mismo modelo upstream que llamarías directamente.", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "El mismo modelo upstream, facturado desde un saldo que también cubre modelos de texto, imagen y audio.", "OpenAI-compatible from day one": "Compatible con OpenAI desde el primer día", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "Apunta base_url a Flatkey y conserva tu SDK, estructuras de solicitud y código de streaming.", "Swap models without a new integration": "Cambia de modelo sin una nueva integración", "Move between ByteDance and every other model in the catalog by changing one string.": "Cambia entre ByteDance y los demás modelos del catálogo modificando una sola cadena.", "How to call the seedance-2.5 API": "Cómo llamar a la API de seedance-2.5", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Cuatro formas de acceder, con la misma clave y catálogo. Elige una para ver un ejemplo ejecutable.", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Llama a cualquier modelo con una API compatible con OpenAI y copia un ejemplo listo para tu modelo y lenguaje.", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "Usa el cliente de OpenAI y establece baseURL en el origen del router Flatkey.", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Gestiona prompts y archivos de referencia en tu terminal con la CLI de Flatkey.", "Codex&Claude Code": "Codex y Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "Usa la misma clave con tu agente de código y enruta trabajos de vídeo desde un script.", "Related Models": "Modelos relacionados", "Other video generation models": "Otros modelos de generación de vídeo", "What is seedance-2.5?": "¿Qué es seedance-2.5?", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 es un modelo de texto e imagen a vídeo con consistencia entre planos, audio nativo y controles de edición.", "How much does seedance-2.5 cost?": "¿Cuánto cuesta seedance-2.5?", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "El precio mostrado se calcula con el catálogo actual de Flatkey y puede variar según el grupo de cuenta y la salida.", "What can I use it for?": "¿Para qué puedo usarlo?", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Úsalo para demos de producto, clips sociales, variantes publicitarias, storyboards y escenas cortas.", "How do I use the model in my app?": "¿Cómo uso el modelo en mi aplicación?", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Envía una solicitud de vídeo a /v1/videos con la misma clave y catálogo que el resto de tu integración Flatkey.", "Can I control output features?": "¿Puedo controlar las funciones de salida?", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Sí. Cuando la ruta lo admite, puedes controlar relación, resolución, duración, audio, medios de referencia y fotogramas inicial/final.", "Is the Flatkey API OpenAI compatible?": "¿La API de Flatkey es compatible con OpenAI?", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "La autenticación y el catálogo siguen el patrón de gateway compatible con OpenAI; los campos de vídeo siguen el formato de contenido Seedance.", "What limits apply?": "¿Qué límites se aplican?", "Rate limits and available model IDs depend on your account and current upstream availability.": "Los límites y los ID disponibles dependen de tu cuenta y de la disponibilidad upstream.", "What happens to my prompts and generated files?": "¿Qué ocurre con mis prompts y archivos generados?", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "Las solicitudes se procesan de forma asíncrona. Conserva el task id y recupera el resultado desde el endpoint de contenido.", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Usa las mismas variables de entorno y clave API del ejemplo en tu flujo de agente de código.", "API–frequently asked questions": "Preguntas frecuentes de la API",
  },
  fr: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 est un modèle vidéo haute qualité pour les workflows texte-vers-vidéo et image-vers-vidéo, avec des clips réalistes, un audio natif et une excellente fidélité aux prompts.",
    "What seedance-2.5 can do": "Ce que seedance-2.5 peut faire",
    "Why Flatkey": "Pourquoi Flatkey",
    Models: "Modèles", "Video generation": "Génération vidéo", "Quick Start": "Démarrage rapide", "Average latency": "Latence moyenne", "Task accepted; generation continues asynchronously": "Tâche acceptée ; la génération continue en arrière-plan", "Last 30 days": "30 derniers jours", "Daily seedance-2.5 requests on Flatkey": "Requêtes quotidiennes de seedance-2.5 sur Flatkey", "Sample shape shown while live telemetry is being connected.": "Exemple de tendance affiché pendant la connexion de la télémétrie.", "Total Requests": "Requêtes totales", "Daily Average": "Moyenne quotidienne", "Busiest Day": "Jour le plus actif", "Text and image to video": "Texte et image vers vidéo", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Générez depuis une scène écrite ou utilisez des images de référence pour un sujet déjà conçu.", "Multi-shot consistency": "Cohérence entre les plans", "Hold characters, wardrobe, and setting across cuts within a single generation.": "Conservez les personnages, les costumes et le décor entre les plans d’une même génération.", "Native audio": "Audio natif", "Ambient sound and speech are generated with the picture, in multiple languages.": "Les sons d’ambiance et la parole sont générés avec l’image, en plusieurs langues.", "Edit and extend": "Modifier et prolonger", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Prolongez ou modifiez un clip avec contrôle de la première image et des images de début/fin.", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Découvrez ses capacités et les changements par rapport à Seedance 2.0 avant de migrer.", "Clip length": "Durée du clip", "Short clips, stitched for longer runs": "Courts clips assemblés pour des séquences longues", "Up to 30s in a single continuous shot": "Jusqu’à 30 s en un seul plan continu", "Reference inputs": "Entrées de référence", "Image references": "Images de référence", "Up to 50 per request — 30 images, 10 videos, 10 audio": "Jusqu’à 50 par requête : 30 images, 10 vidéos et 10 audios", "Motion control": "Contrôle du mouvement", "Text prompt only": "Prompt texte uniquement", "Structured motion paths, green-screen and white-model references": "Trajectoires structurées, références sur fond vert et modèle blanc", "Generated audio": "Audio généré", "Native audio in 10+ languages": "Audio natif dans plus de 10 langues", "Regenerate to change a clip": "Régénérer pour modifier un clip", "Edit and extend an existing clip in place": "Modifier et prolonger un clip existant sur place", "Seedance-2.5 prompts that work": "Prompts Seedance-2.5 efficaces", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Chaque clip est une vraie génération. Copiez son prompt ou chargez-le dans le Playground pour le modifier.", "Why run seedance-2.5 through Flatkey": "Pourquoi utiliser seedance-2.5 via Flatkey", "One key, one balance, and the same upstream model you would call directly.": "Une clé, un solde et le même modèle upstream que vous appelleriez directement.", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "Le même modèle upstream, facturé depuis un solde commun aux modèles texte, image et audio.", "OpenAI-compatible from day one": "Compatible OpenAI dès le premier jour", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "Pointez base_url vers Flatkey et conservez votre SDK, vos requêtes et votre code de streaming.", "Swap models without a new integration": "Changez de modèle sans nouvelle intégration", "Move between ByteDance and every other model in the catalog by changing one string.": "Passez de ByteDance aux autres modèles du catalogue en modifiant une seule chaîne.", "How to call the seedance-2.5 API": "Comment appeler l’API seedance-2.5", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Quatre méthodes avec la même clé et le même catalogue. Choisissez-en une pour voir un exemple exécutable.", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Appelez n’importe quel modèle avec une API compatible OpenAI et copiez un exemple prêt à l’emploi.", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "Utilisez le client OpenAI et définissez baseURL sur l’origine du routeur Flatkey.", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Gérez prompts et fichiers de référence dans votre terminal avec la CLI Flatkey.", "Codex&Claude Code": "Codex et Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "Utilisez la même clé avec votre agent de code et routez les tâches vidéo depuis un script.", "Related Models": "Modèles associés", "Other video generation models": "Autres modèles de génération vidéo", "What is seedance-2.5?": "Qu’est-ce que seedance-2.5 ?", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 est un modèle texte/image vers vidéo avec cohérence multi-plans, audio natif et contrôles d’édition.", "How much does seedance-2.5 cost?": "Combien coûte seedance-2.5 ?", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "Le prix affiché est calculé depuis le catalogue Flatkey actuel et peut varier selon le groupe de compte et les réglages de sortie.", "What can I use it for?": "À quoi puis-je l’utiliser ?", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Utilisez-le pour des démonstrations produit, clips sociaux, variantes publicitaires, storyboards et scènes courtes.", "How do I use the model in my app?": "Comment utiliser le modèle dans mon application ?", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Envoyez une requête vidéo à /v1/videos avec la même clé et le même catalogue que le reste de votre intégration Flatkey.", "Can I control output features?": "Puis-je contrôler les options de sortie ?", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Oui. Si la route le permet, la requête accepte ratio, résolution, durée, audio, médias de référence et images de début/fin.", "Is the Flatkey API OpenAI compatible?": "L’API Flatkey est-elle compatible OpenAI ?", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "L’authentification et le catalogue suivent le modèle de passerelle compatible OpenAI ; les champs vidéo suivent le format Seedance.", "What limits apply?": "Quelles limites s’appliquent ?", "Rate limits and available model IDs depend on your account and current upstream availability.": "Les limites et les ID disponibles dépendent de votre compte et de la disponibilité upstream.", "What happens to my prompts and generated files?": "Que deviennent mes prompts et fichiers générés ?", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "Les requêtes sont traitées de façon asynchrone. Conservez le task id et récupérez le résultat via l’endpoint de contenu.", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Réutilisez les variables d’environnement et la clé API de l’exemple dans votre workflow d’agent de code.", "API–frequently asked questions": "Questions fréquentes sur l’API",
  },
  pt: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 é um modelo de geração de vídeo de alta qualidade para fluxos de texto e imagem para vídeo, com clipes realistas, áudio nativo e forte aderência ao prompt.",
    "What seedance-2.5 can do": "O que o seedance-2.5 pode fazer",
    "Why Flatkey": "Por que Flatkey",
    Models: "Modelos", "Video generation": "Geração de vídeo", "Quick Start": "Início rápido", "Average latency": "Latência média", "Task accepted; generation continues asynchronously": "Tarefa aceita; a geração continua de forma assíncrona", "Last 30 days": "Últimos 30 dias", "Daily seedance-2.5 requests on Flatkey": "Solicitações diárias de seedance-2.5 na Flatkey", "Sample shape shown while live telemetry is being connected.": "Exibimos uma tendência de exemplo enquanto a telemetria é conectada.", "Total Requests": "Solicitações totais", "Daily Average": "Média diária", "Busiest Day": "Dia mais movimentado", "Text and image to video": "Texto e imagem para vídeo", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Gere a partir de uma cena escrita ou use imagens de referência para um sujeito já criado.", "Multi-shot consistency": "Consistência entre planos", "Hold characters, wardrobe, and setting across cuts within a single generation.": "Mantenha personagens, figurino e cenário consistentes entre cortes na mesma geração.", "Native audio": "Áudio nativo", "Ambient sound and speech are generated with the picture, in multiple languages.": "Som ambiente e fala são gerados com a imagem em vários idiomas.", "Edit and extend": "Editar e estender", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Continue ou revise um clipe existente com controle do primeiro e do primeiro/último quadro.", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Conheça as capacidades e as mudanças em relação ao Seedance 2.0 antes de migrar.", "Clip length": "Duração do clipe", "Short clips, stitched for longer runs": "Clipes curtos unidos para sequências longas", "Up to 30s in a single continuous shot": "Até 30 s em um único plano contínuo", "Reference inputs": "Entradas de referência", "Image references": "Imagens de referência", "Up to 50 per request — 30 images, 10 videos, 10 audio": "Até 50 por solicitação: 30 imagens, 10 vídeos e 10 áudios", "Motion control": "Controle de movimento", "Text prompt only": "Somente prompt de texto", "Structured motion paths, green-screen and white-model references": "Caminhos estruturados, referências de tela verde e modelo branco", "Generated audio": "Áudio gerado", "Native audio in 10+ languages": "Áudio nativo em mais de 10 idiomas", "Regenerate to change a clip": "Gere novamente para alterar um clipe", "Edit and extend an existing clip in place": "Edite e estenda um clipe existente no local", "Seedance-2.5 prompts that work": "Prompts de Seedance-2.5 que funcionam", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Cada clipe é uma geração real. Copie o prompt ou carregue-o no Playground para editar.", "Why run seedance-2.5 through Flatkey": "Por que usar seedance-2.5 pela Flatkey", "One key, one balance, and the same upstream model you would call directly.": "Uma chave, um saldo e o mesmo modelo upstream que você chamaria diretamente.", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "O mesmo modelo upstream, cobrado de um saldo que também cobre modelos de texto, imagem e áudio.", "OpenAI-compatible from day one": "Compatível com OpenAI desde o primeiro dia", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "Aponte base_url para a Flatkey e mantenha seu SDK, formatos de solicitação e código de streaming.", "Swap models without a new integration": "Troque de modelo sem uma nova integração", "Move between ByteDance and every other model in the catalog by changing one string.": "Alterne entre ByteDance e os demais modelos do catálogo alterando uma única string.", "How to call the seedance-2.5 API": "Como chamar a API do seedance-2.5", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Quatro formas de entrar, com a mesma chave e catálogo. Escolha uma para ver um exemplo executável.", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Chame qualquer modelo com uma API compatível com OpenAI e copie um exemplo pronto para seu modelo e idioma.", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "Use o cliente OpenAI e defina baseURL para a origem do roteador Flatkey.", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Mantenha prompts e arquivos de referência no fluxo do terminal com a CLI Flatkey.", "Codex&Claude Code": "Codex e Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "Use a mesma chave com seu agente de código e roteie tarefas de vídeo por um script.", "Related Models": "Modelos relacionados", "Other video generation models": "Outros modelos de geração de vídeo", "What is seedance-2.5?": "O que é seedance-2.5?", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 é um modelo de texto e imagem para vídeo com consistência entre planos, áudio nativo e controles de edição.", "How much does seedance-2.5 cost?": "Quanto custa o seedance-2.5?", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "O preço acima é calculado pelo catálogo atual da Flatkey e pode variar por grupo de conta e configurações de saída.", "What can I use it for?": "Para que posso usá-lo?", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Use para demonstrações de produto, clipes sociais, variações de anúncios, storyboards e cenas curtas.", "How do I use the model in my app?": "Como uso o modelo no meu app?", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Envie uma solicitação de vídeo para /v1/videos usando a mesma chave e catálogo do restante da integração Flatkey.", "Can I control output features?": "Posso controlar os recursos de saída?", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Sim. Quando a rota permite, a solicitação aceita proporção, resolução, duração, áudio, mídia de referência e quadros inicial/final.", "Is the Flatkey API OpenAI compatible?": "A API Flatkey é compatível com OpenAI?", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "A autenticação e o catálogo seguem o padrão de gateway compatível com OpenAI; os campos de vídeo seguem o formato Seedance.", "What limits apply?": "Quais limites se aplicam?", "Rate limits and available model IDs depend on your account and current upstream availability.": "Os limites e IDs disponíveis dependem da sua conta e da disponibilidade upstream atual.", "What happens to my prompts and generated files?": "O que acontece com meus prompts e arquivos gerados?", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "As solicitações são processadas de forma assíncrona. Guarde o task id e busque o resultado no endpoint de conteúdo.", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Use as mesmas variáveis de ambiente e chave API do exemplo no seu fluxo de agente de código.", "API–frequently asked questions": "Perguntas frequentes da API",
  },
  ru: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 — высококачественная модель генерации видео из текста и изображений с реалистичными кинематографичными клипами, нативным звуком и точным следованием промпту.",
    "What seedance-2.5 can do": "Что умеет seedance-2.5",
    "Why Flatkey": "Почему Flatkey",
    Models: "Модели", "Video generation": "Генерация видео", "Quick Start": "Быстрый старт", "Average latency": "Средняя задержка", "Task accepted; generation continues asynchronously": "Задача принята; генерация продолжается асинхронно", "Last 30 days": "Последние 30 дней", "Daily seedance-2.5 requests on Flatkey": "Ежедневные запросы seedance-2.5 в Flatkey", "Sample shape shown while live telemetry is being connected.": "Показан пример динамики на время подключения телеметрии.", "Total Requests": "Всего запросов", "Daily Average": "Среднее в день", "Busiest Day": "Самый загруженный день", "Text and image to video": "Текст и изображение в видео", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Создавайте видео по сценарию или используйте референсы уже разработанного объекта.", "Multi-shot consistency": "Согласованность кадров", "Hold characters, wardrobe, and setting across cuts within a single generation.": "Сохраняйте персонажей, одежду и сцену согласованными во всех кадрах одной генерации.", "Native audio": "Нативный звук", "Ambient sound and speech are generated with the picture, in multiple languages.": "Окружение и речь генерируются вместе с изображением на разных языках.", "Edit and extend": "Редактирование и продолжение", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Продолжайте или изменяйте существующий клип с контролем первого и начального/конечного кадров.", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Узнайте о возможностях и изменениях по сравнению с Seedance 2.0 перед переходом.", "Clip length": "Длина клипа", "Short clips, stitched for longer runs": "Короткие клипы, объединённые в длинные последовательности", "Up to 30s in a single continuous shot": "До 30 с одним непрерывным кадром", "Reference inputs": "Референсные входы", "Image references": "Референсные изображения", "Up to 50 per request — 30 images, 10 videos, 10 audio": "До 50 на запрос: 30 изображений, 10 видео и 10 аудио", "Motion control": "Управление движением", "Text prompt only": "Только текстовый промпт", "Structured motion paths, green-screen and white-model references": "Структурированные траектории, референсы зелёного экрана и белой модели", "Generated audio": "Сгенерированный звук", "Native audio in 10+ languages": "Нативный звук на 10+ языках", "Regenerate to change a clip": "Перегенерируйте клип для изменений", "Edit and extend an existing clip in place": "Редактируйте и продолжайте существующий клип на месте", "Seedance-2.5 prompts that work": "Рабочие промпты Seedance-2.5", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Каждый клип создан реально. Скопируйте промпт или загрузите его в Playground для редактирования.", "Why run seedance-2.5 through Flatkey": "Зачем запускать seedance-2.5 через Flatkey", "One key, one balance, and the same upstream model you would call directly.": "Один ключ, один баланс и та же upstream-модель, которую вы вызываете напрямую.", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "Та же upstream-модель оплачивается из единого баланса для текстовых, графических и аудиомоделей.", "OpenAI-compatible from day one": "Совместимость с OpenAI с первого дня", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "Укажите base_url Flatkey и сохраните SDK, формы запросов и потоковый код.", "Swap models without a new integration": "Меняйте модели без новой интеграции", "Move between ByteDance and every other model in the catalog by changing one string.": "Переключайтесь между ByteDance и другими моделями каталога, изменив одну строку.", "How to call the seedance-2.5 API": "Как вызвать API seedance-2.5", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Четыре способа с одним ключом и каталогом. Выберите вариант, чтобы увидеть рабочий пример.", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Вызывайте любую модель через совместимый с OpenAI API и копируйте готовый пример.", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "Используйте клиент OpenAI и задайте baseURL для маршрутизатора Flatkey.", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Управляйте промптами и референсами в терминале с Flatkey CLI.", "Codex&Claude Code": "Codex и Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "Используйте тот же ключ в coding agent и направляйте видеозадачи из скрипта.", "Related Models": "Связанные модели", "Other video generation models": "Другие модели генерации видео", "What is seedance-2.5?": "Что такое seedance-2.5?", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 — модель текста и изображения в видео с согласованностью кадров, нативным звуком и редактированием.", "How much does seedance-2.5 cost?": "Сколько стоит seedance-2.5?", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "Цена выше рассчитана по текущему каталогу Flatkey и может зависеть от группы аккаунта и настроек вывода.", "What can I use it for?": "Для чего его можно использовать?", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Используйте для демонстраций продуктов, соцсетей, рекламных вариантов, раскадровок и коротких сцен.", "How do I use the model in my app?": "Как использовать модель в приложении?", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Отправьте запрос видео на /v1/videos с тем же ключом и каталогом, что и остальная интеграция Flatkey.", "Can I control output features?": "Можно ли управлять параметрами вывода?", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Да. Если маршрут поддерживает, доступны соотношение, разрешение, длительность, звук, референсы и первый/последний кадр.", "Is the Flatkey API OpenAI compatible?": "API Flatkey совместим с OpenAI?", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "Аутентификация и общий каталог следуют совместимому с OpenAI шлюзу, а поля видео — формату Seedance.", "What limits apply?": "Какие ограничения действуют?", "Rate limits and available model IDs depend on your account and current upstream availability.": "Лимиты и доступные ID зависят от аккаунта и текущей доступности upstream.", "What happens to my prompts and generated files?": "Что происходит с промптами и файлами?", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "Запросы обрабатываются асинхронно. Сохраните task id и получите результат через content endpoint.", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Используйте те же переменные окружения и API-ключ из примера в своём workflow.", "API–frequently asked questions": "Частые вопросы об API",
  },
  ja: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 はテキスト・画像から動画を生成する高品質モデルです。リアルでシネマティックな映像、ネイティブ音声、優れたプロンプト追従を実現します。",
    "What seedance-2.5 can do": "seedance-2.5 でできること",
    "Why Flatkey": "Flatkey を選ぶ理由",
    Models: "モデル", "Video generation": "動画生成", "Quick Start": "クイックスタート", "Average latency": "平均レイテンシ", "Task accepted; generation continues asynchronously": "タスクを受け付けました。生成は非同期で続行されます", "Last 30 days": "過去30日間", "Daily seedance-2.5 requests on Flatkey": "Flatkey の seedance-2.5 日次リクエスト", "Sample shape shown while live telemetry is being connected.": "ライブテレメトリ接続中はサンプル推移を表示します。", "Total Requests": "総リクエスト数", "Daily Average": "日平均", "Busiest Day": "最多の日", "Text and image to video": "テキスト・画像から動画", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "文章のシーンから生成するか、設計済みの被写体を参照画像で動かせます。", "Multi-shot consistency": "マルチショットの一貫性", "Hold characters, wardrobe, and setting across cuts within a single generation.": "1回の生成でカットをまたいだキャラクター、衣装、背景を維持します。", "Native audio": "ネイティブ音声", "Ambient sound and speech are generated with the picture, in multiple languages.": "映像と同時に環境音と音声を複数言語で生成します。", "Edit and extend": "編集と延長", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "既存クリップを続行・修正し、最初のフレームや開始/終了フレームを制御できます。", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "機能と Seedance 2.0 からの変更点を確認してから移行できます。", "Clip length": "クリップ長", "Short clips, stitched for longer runs": "短いクリップをつないで長いシーケンスに", "Up to 30s in a single continuous shot": "1つの連続ショットで最大30秒", "Reference inputs": "参照入力", "Image references": "参照画像", "Up to 50 per request — 30 images, 10 videos, 10 audio": "1リクエスト最大50個（画像30、動画10、音声10）", "Motion control": "モーション制御", "Text prompt only": "テキストプロンプトのみ", "Structured motion paths, green-screen and white-model references": "構造化モーション、グリーンバック、白モデル参照に対応", "Generated audio": "生成音声", "Native audio in 10+ languages": "10以上の言語のネイティブ音声", "Regenerate to change a clip": "再生成してクリップを変更", "Edit and extend an existing clip in place": "既存クリップをその場で編集・延長", "Seedance-2.5 prompts that work": "実際に使える Seedance-2.5 プロンプト", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "各クリップは実際の生成結果です。プロンプトをコピーするか、Playground で編集できます。", "Why run seedance-2.5 through Flatkey": "seedance-2.5 を Flatkey 経由で使う理由", "One key, one balance, and the same upstream model you would call directly.": "1つのキーと残高で、直接呼び出すものと同じ upstream モデルを利用できます。", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "同じ upstream モデルを、テキスト・画像・音声モデルと共通の残高で利用できます。", "OpenAI-compatible from day one": "初日から OpenAI 互換", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "base_url を Flatkey に向けるだけで既存の SDK、リクエスト形式、ストリーミングコードを維持できます。", "Swap models without a new integration": "新しい統合なしでモデルを切り替え", "Move between ByteDance and every other model in the catalog by changing one string.": "1つの文字列を変えるだけで ByteDance とカタログ内のモデルを切り替えられます。", "How to call the seedance-2.5 API": "seedance-2.5 API の呼び出し方", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "同じキーとモデルカタログで4つの方法から選べます。実行可能な例を確認してください。", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "OpenAI 互換 API で任意のモデルを呼び出し、モデルと言語に合う例をコピーできます。", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "OpenAI クライアントを使い、baseURL を Flatkey ルーターのオリジンに設定します。", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Flatkey CLI でプロンプトと参照ファイルをターミナルで管理します。", "Codex&Claude Code": "Codex と Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "同じキーをコーディングエージェントで使い、スクリプトから動画ジョブをルーティングできます。", "Related Models": "関連モデル", "Other video generation models": "その他の動画生成モデル", "What is seedance-2.5?": "seedance-2.5 とは？", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 はマルチショット一貫性、ネイティブ音声、編集機能を備えたテキスト・画像から動画モデルです。", "How much does seedance-2.5 cost?": "seedance-2.5 の料金は？", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "上の価格は Flatkey の現在の価格カタログに基づき、アカウントグループや出力設定で変わる場合があります。", "What can I use it for?": "何に使えますか？", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "製品デモ、ソーシャル動画、広告バリエーション、絵コンテ、短いシーンの検証に使えます。", "How do I use the model in my app?": "アプリでモデルを使うには？", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Flatkey 統合の他の部分と同じ API キーとモデルカタログで /v1/videos にリクエストします。", "Can I control output features?": "出力機能を制御できますか？", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "はい。選択したルートが対応していれば、比率、解像度、長さ、音声、参照メディア、開始/終了フレームを指定できます。", "Is the Flatkey API OpenAI compatible?": "Flatkey API は OpenAI 互換ですか？", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "認証と共有カタログは OpenAI 互換ゲートウェイに従い、動画固有フィールドは Seedance content 形式に従います。", "What limits apply?": "制限はありますか？", "Rate limits and available model IDs depend on your account and current upstream availability.": "レート制限と利用可能なモデル ID はアカウントと upstream の状況によって異なります。", "What happens to my prompts and generated files?": "プロンプトと生成ファイルはどうなりますか？", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "リクエストは非同期処理されます。レスポンスの task id を保存し、完了後に content エンドポイントから取得してください。", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "コード例と同じ環境変数・API キーを好みのコーディングエージェントで使えます。", "API–frequently asked questions": "API に関するよくある質問",
  },
  vi: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 là mô hình tạo video chất lượng cao từ văn bản và hình ảnh, tạo clip chân thực, điện ảnh với âm thanh gốc và khả năng bám sát prompt.",
    "What seedance-2.5 can do": "seedance-2.5 có thể làm gì",
    "Why Flatkey": "Vì sao chọn Flatkey",
    Models: "Mô hình", "Video generation": "Tạo video", "Quick Start": "Bắt đầu nhanh", "Average latency": "Độ trễ trung bình", "Task accepted; generation continues asynchronously": "Đã nhận tác vụ; quá trình tạo tiếp tục bất đồng bộ", "Last 30 days": "30 ngày qua", "Daily seedance-2.5 requests on Flatkey": "Số yêu cầu seedance-2.5 hằng ngày trên Flatkey", "Sample shape shown while live telemetry is being connected.": "Hiển thị xu hướng mẫu trong khi kết nối dữ liệu giám sát trực tiếp.", "Total Requests": "Tổng số yêu cầu", "Daily Average": "Trung bình hằng ngày", "Busiest Day": "Ngày bận rộn nhất", "Text and image to video": "Văn bản và hình ảnh thành video", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Tạo từ cảnh viết sẵn hoặc dùng ảnh tham chiếu để điều khiển chủ thể đã thiết kế.", "Multi-shot consistency": "Nhất quán đa cảnh", "Hold characters, wardrobe, and setting across cuts within a single generation.": "Giữ nhân vật, trang phục và bối cảnh nhất quán qua các cảnh trong một lần tạo.", "Native audio": "Âm thanh gốc", "Ambient sound and speech are generated with the picture, in multiple languages.": "Âm thanh môi trường và lời nói được tạo cùng hình ảnh bằng nhiều ngôn ngữ.", "Edit and extend": "Chỉnh sửa và kéo dài", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Tiếp tục hoặc sửa clip hiện có, kiểm soát khung đầu và khung đầu/cuối.", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Xem khả năng và thay đổi so với Seedance 2.0 trước khi chuyển đổi.", "Clip length": "Độ dài clip", "Short clips, stitched for longer runs": "Ghép các clip ngắn thành chuỗi dài", "Up to 30s in a single continuous shot": "Tối đa 30 giây trong một cảnh liên tục", "Reference inputs": "Đầu vào tham chiếu", "Image references": "Ảnh tham chiếu", "Up to 50 per request — 30 images, 10 videos, 10 audio": "Tối đa 50 mỗi yêu cầu: 30 ảnh, 10 video, 10 âm thanh", "Motion control": "Điều khiển chuyển động", "Text prompt only": "Chỉ prompt văn bản", "Structured motion paths, green-screen and white-model references": "Đường chuyển động có cấu trúc, tham chiếu phông xanh và mô hình trắng", "Generated audio": "Âm thanh được tạo", "Native audio in 10+ languages": "Âm thanh gốc hơn 10 ngôn ngữ", "Regenerate to change a clip": "Tạo lại để thay đổi clip", "Edit and extend an existing clip in place": "Chỉnh sửa và kéo dài clip hiện có ngay tại chỗ", "Seedance-2.5 prompts that work": "Prompt Seedance-2.5 hiệu quả", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Mỗi clip là kết quả tạo thật. Sao chép prompt hoặc tải vào Playground để chỉnh sửa.", "Why run seedance-2.5 through Flatkey": "Vì sao chạy seedance-2.5 qua Flatkey", "One key, one balance, and the same upstream model you would call directly.": "Một key, một số dư và cùng mô hình upstream bạn gọi trực tiếp.", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "Cùng mô hình upstream, tính phí từ một số dư dùng chung cho mô hình văn bản, ảnh và âm thanh.", "OpenAI-compatible from day one": "Tương thích OpenAI ngay từ đầu", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "Trỏ base_url đến Flatkey để giữ SDK, định dạng yêu cầu và mã streaming hiện có.", "Swap models without a new integration": "Đổi mô hình không cần tích hợp mới", "Move between ByteDance and every other model in the catalog by changing one string.": "Chuyển giữa ByteDance và các mô hình khác trong danh mục chỉ bằng một chuỗi.", "How to call the seedance-2.5 API": "Cách gọi API seedance-2.5", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Bốn cách truy cập với cùng key và danh mục mô hình. Chọn một cách để xem ví dụ chạy được.", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Gọi mọi mô hình bằng API tương thích OpenAI và sao chép ví dụ sẵn sàng cho mô hình/ngôn ngữ của bạn.", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "Dùng OpenAI client và đặt baseURL tới router Flatkey.", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Quản lý prompt và tệp tham chiếu trong terminal bằng Flatkey CLI.", "Codex&Claude Code": "Codex và Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "Dùng cùng key với coding agent và định tuyến tác vụ video từ script.", "Related Models": "Mô hình liên quan", "Other video generation models": "Mô hình tạo video khác", "What is seedance-2.5?": "seedance-2.5 là gì?", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 là mô hình văn bản và hình ảnh thành video với nhất quán đa cảnh, âm thanh gốc và điều khiển chỉnh sửa.", "How much does seedance-2.5 cost?": "seedance-2.5 có giá bao nhiêu?", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "Giá trực tiếp ở trên lấy từ danh mục Flatkey hiện tại và có thể thay đổi theo nhóm tài khoản và cài đặt đầu ra.", "What can I use it for?": "Có thể dùng cho việc gì?", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Dùng cho demo sản phẩm, clip mạng xã hội, biến thể quảng cáo, storyboard và thử nghiệm cảnh ngắn.", "How do I use the model in my app?": "Dùng mô hình trong ứng dụng thế nào?", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Gửi yêu cầu video tới /v1/videos bằng API key và danh mục mô hình như các phần khác của tích hợp Flatkey.", "Can I control output features?": "Có thể kiểm soát tính năng đầu ra không?", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Có. Khi route hỗ trợ, yêu cầu cho phép đặt tỷ lệ, độ phân giải, thời lượng, âm thanh, media tham chiếu và khung đầu/cuối.", "Is the Flatkey API OpenAI compatible?": "API Flatkey có tương thích OpenAI không?", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "Xác thực và danh mục dùng chung theo gateway tương thích OpenAI; trường video theo định dạng nội dung Seedance.", "What limits apply?": "Có giới hạn nào?", "Rate limits and available model IDs depend on your account and current upstream availability.": "Giới hạn tốc độ và model ID khả dụng tùy tài khoản và tình trạng upstream.", "What happens to my prompts and generated files?": "Prompt và tệp đã tạo được xử lý thế nào?", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "Yêu cầu được xử lý bất đồng bộ. Lưu task id trong phản hồi và lấy kết quả từ content endpoint khi hoàn tất.", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Dùng cùng biến môi trường và API key trong ví dụ ở workflow coding agent của bạn.", "API–frequently asked questions": "Câu hỏi thường gặp về API",
  },
  de: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 ist ein hochwertiges Text- und Bild-zu-Video-Modell für realistische, kinematografische Clips mit nativem Audio und hoher Prompt-Treue.",
    "What seedance-2.5 can do": "Was seedance-2.5 kann",
    "Why Flatkey": "Warum Flatkey",
    Models: "Modelle", "Video generation": "Videogenerierung", "Quick Start": "Schnellstart", "Average latency": "Durchschnittliche Latenz", "Task accepted; generation continues asynchronously": "Aufgabe angenommen; die Generierung läuft asynchron weiter", "Last 30 days": "Letzte 30 Tage", "Daily seedance-2.5 requests on Flatkey": "Tägliche seedance-2.5-Anfragen auf Flatkey", "Sample shape shown while live telemetry is being connected.": "Beispieltrend während der Verbindung der Live-Telemetrie.", "Total Requests": "Anfragen gesamt", "Daily Average": "Tagesdurchschnitt", "Busiest Day": "Aktivster Tag", "Text and image to video": "Text und Bild zu Video", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Aus einer beschriebenen Szene generieren oder ein bereits gestaltetes Motiv mit Referenzbildern steuern.", "Multi-shot consistency": "Konsistenz über mehrere Einstellungen", "Hold characters, wardrobe, and setting across cuts within a single generation.": "Figuren, Kleidung und Umgebung über Schnitte innerhalb einer Generierung konsistent halten.", "Native audio": "Native Audio", "Ambient sound and speech are generated with the picture, in multiple languages.": "Umgebungsgeräusche und Sprache werden zusammen mit dem Bild in mehreren Sprachen erzeugt.", "Edit and extend": "Bearbeiten und erweitern", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Bestehende Clips fortsetzen oder ändern, mit Kontrolle über erstes sowie erstes/letztes Bild.", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Funktionen und Änderungen gegenüber Seedance 2.0 vor dem Wechsel prüfen.", "Clip length": "Clip-Länge", "Short clips, stitched for longer runs": "Kurze Clips zu längeren Sequenzen verbinden", "Up to 30s in a single continuous shot": "Bis zu 30 s in einer durchgehenden Einstellung", "Reference inputs": "Referenzeingaben", "Image references": "Bildreferenzen", "Up to 50 per request — 30 images, 10 videos, 10 audio": "Bis zu 50 pro Anfrage: 30 Bilder, 10 Videos und 10 Audios", "Motion control": "Bewegungssteuerung", "Text prompt only": "Nur Text-Prompt", "Structured motion paths, green-screen and white-model references": "Strukturierte Bewegungspfade sowie Greenscreen- und Weißmodell-Referenzen", "Generated audio": "Generiertes Audio", "Native audio in 10+ languages": "Native Audio in über 10 Sprachen", "Regenerate to change a clip": "Clip zur Änderung neu generieren", "Edit and extend an existing clip in place": "Bestehenden Clip direkt bearbeiten und erweitern", "Seedance-2.5 prompts that work": "Funktionierende Seedance-2.5-Prompts", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Jeder Clip ist echt generiert. Prompt kopieren oder im Playground laden und bearbeiten.", "Why run seedance-2.5 through Flatkey": "Warum seedance-2.5 über Flatkey ausführen", "One key, one balance, and the same upstream model you would call directly.": "Ein Schlüssel, ein Guthaben und dasselbe Upstream-Modell wie beim direkten Aufruf.", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "Dasselbe Upstream-Modell, abgerechnet über ein Guthaben für Text-, Bild- und Audiomodelle.", "OpenAI-compatible from day one": "Von Anfang an OpenAI-kompatibel", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "base_url auf Flatkey setzen und vorhandenes SDK, Anfrageformate und Streaming-Code behalten.", "Swap models without a new integration": "Modelle ohne neue Integration wechseln", "Move between ByteDance and every other model in the catalog by changing one string.": "Zwischen ByteDance und allen anderen Katalogmodellen mit einer Zeichenkette wechseln.", "How to call the seedance-2.5 API": "Die seedance-2.5-API aufrufen", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Vier Wege mit demselben Schlüssel und Katalog. Wählen Sie einen ausführbaren Beispielweg.", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Beliebige Modelle über eine OpenAI-kompatible API aufrufen und ein passendes Beispiel kopieren.", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "OpenAI-Client verwenden und baseURL auf den Flatkey-Router setzen.", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Prompts und Referenzdateien mit der Flatkey CLI im Terminal verwalten.", "Codex&Claude Code": "Codex und Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "Denselben Schlüssel mit dem Coding-Agent verwenden und Videojobs per Skript routen.", "Related Models": "Verwandte Modelle", "Other video generation models": "Weitere Videogenerierungsmodelle", "What is seedance-2.5?": "Was ist seedance-2.5?", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 ist ein Text- und Bild-zu-Video-Modell mit Multi-Shot-Konsistenz, nativem Audio und Bearbeitungssteuerung.", "How much does seedance-2.5 cost?": "Was kostet seedance-2.5?", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "Der aktuelle Preis oben stammt aus dem Flatkey-Katalog und kann je nach Kontogruppe und Ausgabeoptionen variieren.", "What can I use it for?": "Wofür kann ich es verwenden?", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Für Produktdemos, Social Clips, Werbevarianten, Storyboards und kurze Szenenexperimente.", "How do I use the model in my app?": "Wie nutze ich das Modell in meiner App?", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Vide Anfrage an /v1/videos mit demselben Schlüssel und Katalog wie in der übrigen Flatkey-Integration senden.", "Can I control output features?": "Kann ich Ausgabeoptionen steuern?", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Ja. Wenn die Route es unterstützt, können Seitenverhältnis, Auflösung, Dauer, Audio, Referenzen und erstes/letztes Bild gesteuert werden.", "Is the Flatkey API OpenAI compatible?": "Ist die Flatkey-API OpenAI-kompatibel?", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "Authentifizierung und Katalog folgen dem OpenAI-kompatiblen Gateway-Muster; Videospezifisches folgt dem Seedance-Content-Format.", "What limits apply?": "Welche Limits gelten?", "Rate limits and available model IDs depend on your account and current upstream availability.": "Ratenlimits und verfügbare Modell-IDs hängen von Konto und Upstream-Verfügbarkeit ab.", "What happens to my prompts and generated files?": "Was geschieht mit Prompts und generierten Dateien?", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "Anfragen werden asynchron verarbeitet. task id speichern und das Ergebnis später am Content-Endpunkt abrufen.", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Dieselben Umgebungsvariablen und den API-Schlüssel aus dem Beispiel im Coding-Agent-Workflow verwenden.", "API–frequently asked questions": "Häufige Fragen zur API",
  },
  id: {
    "Seedance 2.5 is a high-quality video generation model for text-to-video and image-to-video workflows. Generate realistic, cinematic clips with native audio and strong prompt adherence.": "Seedance 2.5 adalah model pembuatan video berkualitas tinggi dari teks dan gambar, dengan klip realistis sinematik, audio bawaan, dan kepatuhan prompt yang kuat.",
    "What seedance-2.5 can do": "Apa yang dapat dilakukan seedance-2.5",
    "Why Flatkey": "Mengapa Flatkey",
    Models: "Model", "Video generation": "Pembuatan video", "Quick Start": "Mulai cepat", "Average latency": "Latensi rata-rata", "Task accepted; generation continues asynchronously": "Tugas diterima; pembuatan berlanjut secara asinkron", "Last 30 days": "30 hari terakhir", "Daily seedance-2.5 requests on Flatkey": "Permintaan seedance-2.5 harian di Flatkey", "Sample shape shown while live telemetry is being connected.": "Tren contoh ditampilkan saat telemetri langsung disambungkan.", "Total Requests": "Total permintaan", "Daily Average": "Rata-rata harian", "Busiest Day": "Hari tersibuk", "Text and image to video": "Teks dan gambar menjadi video", "Generate from a written scene, or drive it with reference images for a subject you have already designed.": "Buat dari adegan tertulis atau gunakan gambar referensi untuk subjek yang sudah dirancang.", "Multi-shot consistency": "Konsistensi multi-shot", "Hold characters, wardrobe, and setting across cuts within a single generation.": "Pertahankan karakter, busana, dan latar tetap konsisten di seluruh potongan dalam satu generasi.", "Native audio": "Audio bawaan", "Ambient sound and speech are generated with the picture, in multiple languages.": "Suara lingkungan dan ucapan dibuat bersama gambar dalam berbagai bahasa.", "Edit and extend": "Edit dan perluas", "Continue an existing clip or revise one, with first-frame and first/last-frame control.": "Lanjutkan atau revisi klip yang ada dengan kontrol frame pertama dan awal/akhir.", "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching.": "Pelajari kemampuan dan perubahan dari Seedance 2.0 sebelum beralih.", "Clip length": "Durasi klip", "Short clips, stitched for longer runs": "Gabungkan klip pendek menjadi rangkaian panjang", "Up to 30s in a single continuous shot": "Hingga 30 detik dalam satu shot berkelanjutan", "Reference inputs": "Input referensi", "Image references": "Gambar referensi", "Up to 50 per request — 30 images, 10 videos, 10 audio": "Hingga 50 per permintaan: 30 gambar, 10 video, 10 audio", "Motion control": "Kontrol gerakan", "Text prompt only": "Hanya prompt teks", "Structured motion paths, green-screen and white-model references": "Jalur gerak terstruktur, referensi green-screen dan model putih", "Generated audio": "Audio yang dibuat", "Native audio in 10+ languages": "Audio bawaan dalam 10+ bahasa", "Regenerate to change a clip": "Buat ulang untuk mengubah klip", "Edit and extend an existing clip in place": "Edit dan perluas klip yang ada langsung di tempat", "Seedance-2.5 prompts that work": "Prompt Seedance-2.5 yang berhasil", "Each clip is a real generation. Copy its prompt, or load it into the playground and edit from there.": "Setiap klip adalah hasil generasi nyata. Salin prompt atau muat ke Playground untuk mengedit.", "Why run seedance-2.5 through Flatkey": "Mengapa menjalankan seedance-2.5 lewat Flatkey", "One key, one balance, and the same upstream model you would call directly.": "Satu key, satu saldo, dan model upstream yang sama seperti panggilan langsung.", "The same upstream model, billed from one balance that also covers text, image, and audio models.": "Model upstream yang sama ditagihkan dari satu saldo yang juga mencakup model teks, gambar, dan audio.", "OpenAI-compatible from day one": "Kompatibel dengan OpenAI sejak awal", "Point base_url at Flatkey and keep your existing SDK, request shapes, and streaming code.": "Arahkan base_url ke Flatkey dan pertahankan SDK, bentuk permintaan, serta kode streaming yang ada.", "Swap models without a new integration": "Ganti model tanpa integrasi baru", "Move between ByteDance and every other model in the catalog by changing one string.": "Beralih antara ByteDance dan model lain di katalog dengan mengubah satu string.", "How to call the seedance-2.5 API": "Cara memanggil API seedance-2.5", "Four ways in, all on the same key and the same model catalog. Pick one to see a runnable example.": "Empat cara dengan key dan katalog yang sama. Pilih satu untuk melihat contoh yang dapat dijalankan.", "Call any model with an OpenAI-compatible API. Copy a ready-to-run example for your model and language.": "Panggil model apa pun dengan API kompatibel OpenAI dan salin contoh siap pakai.", "Use the OpenAI client and set baseURL to your Flatkey router origin.": "Gunakan klien OpenAI dan atur baseURL ke origin router Flatkey.", "Keep prompts and reference files in your terminal workflow with the Flatkey CLI.": "Kelola prompt dan file referensi di terminal dengan Flatkey CLI.", "Codex&Claude Code": "Codex dan Claude Code", "Use the same key with your coding agent and route video jobs from a script.": "Gunakan key yang sama dengan coding agent dan rutekan tugas video dari skrip.", "Related Models": "Model terkait", "Other video generation models": "Model pembuatan video lainnya", "What is seedance-2.5?": "Apa itu seedance-2.5?", "Seedance-2.5 is a text-to-video and image-to-video model with multi-shot consistency, native audio, and editing controls.": "Seedance-2.5 adalah model teks dan gambar menjadi video dengan konsistensi multi-shot, audio bawaan, dan kontrol edit.", "How much does seedance-2.5 cost?": "Berapa biaya seedance-2.5?", "The live price above is calculated from Flatkey's current pricing catalog and may vary by account group and output settings.": "Harga langsung di atas dihitung dari katalog Flatkey saat ini dan dapat berubah menurut grup akun serta pengaturan output.", "What can I use it for?": "Untuk apa model ini digunakan?", "Use it for product demos, social clips, ad variations, storyboards, and short-form scene experiments.": "Gunakan untuk demo produk, klip sosial, variasi iklan, storyboard, dan eksperimen adegan pendek.", "How do I use the model in my app?": "Bagaimana memakai model di aplikasi?", "Send a video request to /v1/videos using the same API key and model catalog as the rest of your Flatkey integration.": "Kirim permintaan video ke /v1/videos dengan key API dan katalog model yang sama seperti integrasi Flatkey lainnya.", "Can I control output features?": "Bisakah fitur output dikontrol?", "Yes. The request supports ratio, resolution, duration, audio, reference media, and first/last-frame options when the selected route supports them.": "Ya. Jika rute mendukung, permintaan dapat mengatur rasio, resolusi, durasi, audio, media referensi, dan frame awal/akhir.", "Is the Flatkey API OpenAI compatible?": "Apakah API Flatkey kompatibel dengan OpenAI?", "Authentication and the shared catalog follow the OpenAI-compatible gateway pattern, while video-specific fields follow the Seedance content format.": "Autentikasi dan katalog bersama mengikuti pola gateway kompatibel OpenAI; kolom video mengikuti format konten Seedance.", "What limits apply?": "Batas apa yang berlaku?", "Rate limits and available model IDs depend on your account and current upstream availability.": "Batas laju dan ID model yang tersedia bergantung pada akun serta ketersediaan upstream saat ini.", "What happens to my prompts and generated files?": "Apa yang terjadi pada prompt dan file yang dibuat?", "Requests are processed asynchronously. Keep the task id from the response and fetch the result from the content endpoint when ready.": "Permintaan diproses secara asinkron. Simpan task id dan ambil hasil dari endpoint konten saat siap.", "Use the same environment variables and API key from the code sample in your preferred coding-agent workflow.": "Gunakan variabel lingkungan dan key API yang sama dari contoh dalam workflow coding agent pilihan Anda.", "API–frequently asked questions": "Pertanyaan umum tentang API",
  },
};

// Editorial Seedance 2.5 copy added during the factual re-audit.  Keep these
// strings separate from the older prototype table so a stale translation can
// never silently re-introduce an unsupported claim. English is the global
// source copy; every supported locale below carries the same audited key set.
const seedanceFactCopy: Partial<Record<Locale, Record<string, string>>> = {
  en: {
    "Seedance 2.5 AI Video Generator & API": "Seedance 2.5 AI Video Generator & API",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "Live request telemetry appears here when enough Flatkey traffic is available.",
    "Seedance 2.5 usage activity": "Seedance 2.5 usage activity",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.",
    "Text-to-video and image-to-video": "Text-to-video and image-to-video",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.",
    "Reference media and frame control": "Reference media and frame control",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.",
    "Audio-video generation": "Audio-video generation",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.",
    "Duration and output controls": "Duration and output controls",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.",
    "Seedance 2.5 features": "Seedance 2.5 features",
    "What is Seedance 2.5? Features for AI video generation": "What is Seedance 2.5? Features for AI video generation",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.",
    "Seedance 2.5 features: references, audio, and 30-second video": "Seedance 2.5 features: references, audio, and 30-second video",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0 (not re-audited)",
    "Not verified in this audit": "Not verified in this audit",
    "4–30 seconds per request": "4–30 seconds per request",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "Up to 50 total: 30 images, 10 videos, and 10 audio",
    "First-frame and last-frame roles are supported": "First-frame and last-frame roles are supported",
    "Audio generation can be enabled per request": "Audio generation can be enabled per request",
    "Reference-guided and first/last-frame workflows": "Reference-guided and first/last-frame workflows",
    "Seedance 2.5 prompt guide: six workflows": "Seedance 2.5 prompt guide: six workflows",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.",
    "Why use Seedance 2.5 through Flatkey?": "Why use Seedance 2.5 through Flatkey?",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.",
    "One key for the model catalog": "One key for the model catalog",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Use the same Flatkey account and API key across video, image, audio, and text workloads.",
    "Documented video contract": "Documented video contract",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.",
    "Pricing follows the request": "Pricing follows the request",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.",
    "Live data only when available": "Live data only when available",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.",
    "Seedance 2.5 API: how to use /v1/videos": "Seedance 2.5 API: how to use /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.",
    "Async task result": "Async task result",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.",
    "Reference limits": "Reference limits",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.",
    "Output controls": "Output controls",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.",
    "480p · no video reference": "480p · no video reference",
    "720p · no video reference": "720p · no video reference",
    "Catalog formula": "Catalog formula",
    "Video reference input": "Video reference input",
    "Depends on resolution": "Depends on resolution",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.",
    "What is the Seedance 2.5 release date?": "What is the Seedance 2.5 release date?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.",
    "Is Seedance 2.5 free?": "Is Seedance 2.5 free?",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Seedance 2.5 pricing: 480p, 720p, and video references",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.",
    "Seedance 2.5 request pricing formulas": "Seedance 2.5 request pricing formulas",
    "Scenario": "Scenario",
    "Flatkey formula": "Flatkey formula",
    "Billing basis": "Billing basis",
    "Total input-video seconds": "Total input-video seconds",
    "Output duration": "Output duration",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.",
  },
  es: {
    "Seedance 2.5 AI Video Generator & API": "Generador de vídeo con IA y API de Seedance 2.5",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "Seedance 2.5 de ByteDance es un modelo de generación audiovisual para flujos de trabajo de texto a vídeo e imagen a vídeo. Usa medios de referencia, control del primer/último fotograma, solicitudes de 4–30 segundos y audio opcional mediante el endpoint /v1/videos de Flatkey.",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "La telemetría de solicitudes en directo aparece aquí cuando hay suficiente tráfico de Flatkey.",
    "Seedance 2.5 usage activity": "Actividad de uso de Seedance 2.5",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Aquí solo se muestran datos de solicitudes en directo de Flatkey; el gráfico aparece después de recopilar suficiente tráfico.",
    "Text-to-video and image-to-video": "Texto a vídeo e imagen a vídeo",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Empieza con una escena escrita o proporciona imágenes de referencia de un sujeto, producto o storyboard que ya hayas diseñado.",
    "Reference media and frame control": "Medios de referencia y control de fotogramas",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "La solicitud puede incluir referencias de imagen, vídeo y audio, además de los roles de primer y último fotograma para flujos guiados por referencias.",
    "Audio-video generation": "Generación audiovisual",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Activa la generación de audio como una opción explícita de la solicitud; no supongas una lista de idiomas ni un comportamiento de audio que la ruta seleccionada no documente.",
    "Duration and output controls": "Controles de duración y salida",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Elige 4–30 segundos, 480p o 720p, una relación adaptativa o fija compatible y la configuración de audio antes de enviar la tarea.",
    "Seedance 2.5 features": "Funciones de Seedance 2.5",
    "What is Seedance 2.5? Features for AI video generation": "¿Qué es Seedance 2.5? Funciones para generar vídeos con IA",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "El contrato documentado cubre entradas de texto a vídeo e imagen a vídeo, medios de referencia, audio opcional y ajustes de salida acotados.",
    "Seedance 2.5 features: references, audio, and 30-second video": "Funciones de Seedance 2.5: referencias, audio y vídeo de 30 segundos",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "Esta comparación registra el comportamiento documentado de Seedance 2.5. Los valores de Seedance 2.0 se marcan como no verificados en lugar de inferirse.",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0 (sin una nueva auditoría)",
    "Not verified in this audit": "No verificado en esta auditoría",
    "4–30 seconds per request": "4–30 segundos por solicitud",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "Hasta 50 en total: 30 imágenes, 10 vídeos y 10 audios",
    "First-frame and last-frame roles are supported": "Se admiten los roles de primer y último fotograma",
    "Audio generation can be enabled per request": "La generación de audio se puede activar en cada solicitud",
    "Reference-guided and first/last-frame workflows": "Flujos guiados por referencias y por primer/último fotograma",
    "Seedance 2.5 prompt guide: six workflows": "Guía de prompts de Seedance 2.5: seis flujos de trabajo",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Usa estos prompts específicos de cada flujo como punto de partida. La salida depende de las referencias proporcionadas y de la configuración de la solicitud.",
    "Why use Seedance 2.5 through Flatkey?": "¿Por qué usar Seedance 2.5 con Flatkey?",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Usa una sola clave para el catálogo de modelos, revisa el contrato de la solicitud y vincula el precio a la configuración elegida.",
    "One key for the model catalog": "Una clave para el catálogo de modelos",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Usa la misma cuenta y clave API de Flatkey para trabajos de vídeo, imagen, audio y texto.",
    "Documented video contract": "Contrato de vídeo documentado",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "Mantén explícitos en tu integración la solicitud content[] de Seedance, el endpoint /v1/videos y el flujo de tareas asíncrono.",
    "Pricing follows the request": "El precio depende de la solicitud",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "La resolución, la duración y la entrada de referencia de vídeo cambian la fórmula; el valor del catálogo no es una tarifa universal por segundo.",
    "Live data only when available": "Datos en directo solo cuando están disponibles",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Las tarjetas de rendimiento y actividad muestran telemetría cuando hay suficiente tráfico real de Flatkey; de lo contrario quedan sin datos.",
    "Seedance 2.5 API: how to use /v1/videos": "API de Seedance 2.5: cómo usar /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Envía los elementos content[] de Seedance, conserva el task id y obtén el resultado desde /v1/videos/{task_id}/content.",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Envía el ID del modelo, un array content[] y los campos compatibles de duración, resolución, relación y audio.",
    "Async task result": "Resultado de tarea asíncrona",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Guarda el task id devuelto y recupera el archivo generado desde el endpoint de contenido cuando esté listo.",
    "Reference limits": "Límites de referencias",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "El adaptador acepta hasta 30 imágenes, 10 vídeos y 10 referencias de audio, con 50 en total.",
    "Output controls": "Controles de salida",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Elige 480p o 720p, una relación compatible, 4–30 segundos y si quieres generar audio.",
    "480p · no video reference": "480p · sin referencia de vídeo",
    "720p · no video reference": "720p · sin referencia de vídeo",
    "Catalog formula": "Fórmula del catálogo",
    "Video reference input": "Entrada de referencia de vídeo",
    "Depends on resolution": "Depende de la resolución",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "El precio de Seedance 2.5 varía según la resolución, la duración y la entrada de referencia de vídeo; la base del catálogo no es una tarifa universal por segundo.",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 es el modelo de generación audiovisual de ByteDance para solicitudes de texto a vídeo e imagen a vídeo, con medios de referencia y controles de audio opcionales.",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "La base del catálogo es $0.14, pero la fórmula de la solicitud varía: 480p sin entrada de vídeo cuesta $0.140 × duration; 720p cuesta $0.314 × duration; las fórmulas con referencia de vídeo usan los segundos totales de vídeo y la resolución.",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Úsalo para storyboards de microdramas y cómics, variantes de productos y UGC, previsualización cinematográfica, cinemáticas de juegos, clips de creadores y pruebas creativas de investigación de mercado.",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "Envía un POST a /v1/videos con el formato content[] de Seedance, conserva el task id asíncrono y recupera el resultado desde /v1/videos/{task_id}/content.",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Sí. Configura 480p o 720p, 4–30 segundos, una relación compatible, generate_audio y los campos documentados de referencia/fotograma.",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "La autenticación de Flatkey y el catálogo compartido siguen el patrón de gateway, mientras que las solicitudes de vídeo de Seedance usan content[] y el contrato asíncrono /v1/videos.",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "Una solicitud puede incluir hasta 30 imágenes, 10 vídeos y 10 referencias de audio, con 50 referencias en total; los límites de la cuenta y la disponibilidad pueden cambiar.",
    "What is the Seedance 2.5 release date?": "¿Cuál es la fecha de lanzamiento de Seedance 2.5?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "El artículo oficial de ByteDance se publicó el 2026-07-31; el catálogo de Flatkey indica released_at como 2026-08-04. Son metadatos distintos, por lo que ninguna fecha por sí sola representa todos los lanzamientos.",
    "Is Seedance 2.5 free?": "¿Seedance 2.5 es gratis?",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "Esta página no promete acceso gratuito ni ilimitado. Consulta los precios en directo de Flatkey y los límites de tu cuenta antes de ejecutar tareas.",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Precios de Seedance 2.5: 480p, 720p y referencias de vídeo",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "La base del catálogo es $0.14; la fórmula de la solicitud depende de la resolución de salida, la duración y la entrada de referencia de vídeo.",
    "Seedance 2.5 request pricing formulas": "Fórmulas de precios por solicitud de Seedance 2.5",
    "Scenario": "Escenario",
    "Flatkey formula": "Fórmula de Flatkey",
    "Billing basis": "Base de facturación",
    "Total input-video seconds": "Segundos totales del vídeo de entrada",
    "Output duration": "Duración de salida",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "La base del catálogo y la fórmula de la solicitud se muestran por separado; la liquidación final sigue la estimación de la tarea y los límites de la cuenta.",
  },
  fr: {
    "Seedance 2.5 AI Video Generator & API": "Générateur vidéo IA Seedance 2.5 et API",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "Seedance 2.5 de ByteDance est un modèle de génération audiovisuelle pour les flux texte-vers-vidéo et image-vers-vidéo. Utilisez des médias de référence, le contrôle des première et dernière images, des requêtes de 4 à 30 secondes et l'audio facultatif via l'endpoint /v1/videos de Flatkey.",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "La télémétrie des requêtes en direct apparaîtra ici lorsque le trafic Flatkey sera suffisant.",
    "Seedance 2.5 usage activity": "Activité d'utilisation de Seedance 2.5",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Seules les données réelles des requêtes Flatkey sont affichées ici ; le graphique apparaît après la collecte de suffisamment de trafic.",
    "Text-to-video and image-to-video": "Texte-vers-vidéo et image-vers-vidéo",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Commencez par une scène écrite ou fournissez des images de référence pour un sujet, un produit ou un storyboard déjà conçu.",
    "Reference media and frame control": "Médias de référence et contrôle des images",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "La requête peut inclure des références d'image, de vidéo et d'audio, ainsi que des rôles de première et dernière image pour les flux guidés par référence.",
    "Audio-video generation": "Génération audiovisuelle",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Activez la génération audio comme option explicite de la requête ; ne supposez pas une liste de langues ou un comportement audio que la route sélectionnée ne documente pas.",
    "Duration and output controls": "Contrôles de durée et de sortie",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Choisissez 4 à 30 secondes, 480p ou 720p, un format adaptatif ou fixe compatible et le réglage audio avant d'envoyer la tâche.",
    "Seedance 2.5 features": "Fonctionnalités de Seedance 2.5",
    "What is Seedance 2.5? Features for AI video generation": "Qu'est-ce que Seedance 2.5 ? Fonctionnalités de génération vidéo IA",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "Le contrat documenté couvre les entrées texte-vers-vidéo et image-vers-vidéo, les médias de référence, l'audio facultatif et des réglages de sortie limités.",
    "Seedance 2.5 features: references, audio, and 30-second video": "Fonctionnalités de Seedance 2.5 : références, audio et vidéo de 30 secondes",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "Cette comparaison consigne le comportement documenté de Seedance 2.5. Les valeurs de Seedance 2.0 sont indiquées comme non vérifiées plutôt que déduites.",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0 (non réaudité)",
    "Not verified in this audit": "Non vérifié dans cet audit",
    "4–30 seconds per request": "4 à 30 secondes par requête",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "Jusqu'à 50 au total : 30 images, 10 vidéos et 10 fichiers audio",
    "First-frame and last-frame roles are supported": "Les rôles de première et dernière image sont pris en charge",
    "Audio generation can be enabled per request": "La génération audio peut être activée pour chaque requête",
    "Reference-guided and first/last-frame workflows": "Flux guidés par référence et par première/dernière image",
    "Seedance 2.5 prompt guide: six workflows": "Guide de prompts Seedance 2.5 : six flux",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Utilisez ces prompts propres à chaque flux comme points de départ. Le résultat dépend des références fournies et des réglages de la requête.",
    "Why use Seedance 2.5 through Flatkey?": "Pourquoi utiliser Seedance 2.5 avec Flatkey ?",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Utilisez une clé pour le catalogue de modèles, consultez le contrat de requête et liez le prix aux réglages sélectionnés.",
    "One key for the model catalog": "Une clé pour le catalogue de modèles",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Utilisez le même compte Flatkey et la même clé API pour les tâches vidéo, image, audio et texte.",
    "Documented video contract": "Contrat vidéo documenté",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "Gardez explicites dans votre intégration la requête content[] de Seedance, l'endpoint /v1/videos et le flux de tâches asynchrone.",
    "Pricing follows the request": "Le prix suit la requête",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "La résolution, la durée et l'entrée de référence vidéo modifient la formule ; la valeur du catalogue ne constitue pas un tarif universel par seconde.",
    "Live data only when available": "Données en direct lorsqu'elles sont disponibles",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Les cartes de performance et d'activité affichent la télémétrie lorsqu'il existe assez de trafic Flatkey réel ; sinon, elles restent sans données.",
    "Seedance 2.5 API: how to use /v1/videos": "API Seedance 2.5 : comment utiliser /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Envoyez les éléments content[] de Seedance, conservez l'identifiant de tâche et récupérez le résultat via /v1/videos/{task_id}/content.",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Envoyez l'identifiant du modèle, un tableau content[] et les champs compatibles de durée, résolution, format et audio.",
    "Async task result": "Résultat de tâche asynchrone",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Enregistrez l'identifiant de tâche renvoyé, puis récupérez le fichier généré via l'endpoint de contenu lorsqu'il est prêt.",
    "Reference limits": "Limites des références",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "L'adaptateur accepte jusqu'à 30 images, 10 vidéos et 10 références audio, soit 50 au total.",
    "Output controls": "Contrôles de sortie",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Choisissez 480p ou 720p, un format compatible, 4 à 30 secondes et l'activation ou non de la génération audio.",
    "480p · no video reference": "480p · sans référence vidéo",
    "720p · no video reference": "720p · sans référence vidéo",
    "Catalog formula": "Formule du catalogue",
    "Video reference input": "Entrée de référence vidéo",
    "Depends on resolution": "Dépend de la résolution",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Le prix de Seedance 2.5 varie selon la résolution, la durée et l'entrée de référence vidéo ; la base du catalogue n'est pas un tarif universel par seconde.",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 est le modèle de génération audiovisuelle de ByteDance pour les requêtes texte-vers-vidéo et image-vers-vidéo, avec médias de référence et contrôles audio facultatifs.",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "La base du catalogue est de 0,14 $, mais la formule de requête varie : en 480p sans entrée vidéo, elle est de 0,140 $ × durée ; en 720p, de 0,314 $ × durée ; les formules avec référence vidéo utilisent le nombre total de secondes vidéo et la résolution.",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Utilisez-le pour les storyboards de micro-dramas et de bandes dessinées, les variantes de produits et d'UGC, la prévisualisation de films, les cinématiques de jeux, les clips de créateurs et les tests créatifs d'études de marché.",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "Effectuez un POST vers /v1/videos au format content[] de Seedance, conservez l'identifiant de tâche asynchrone et récupérez le résultat via /v1/videos/{task_id}/content.",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Oui. Définissez 480p ou 720p, 4 à 30 secondes, un format compatible, generate_audio et les champs de référence/image documentés.",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "L'authentification Flatkey et le catalogue partagé utilisent le modèle de passerelle, tandis que les requêtes vidéo Seedance utilisent content[] et le contrat asynchrone /v1/videos.",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "Une requête peut inclure jusqu'à 30 images, 10 vidéos et 10 références audio, soit 50 références au total ; les limites et la disponibilité du compte peuvent changer.",
    "What is the Seedance 2.5 release date?": "Quelle est la date de sortie de Seedance 2.5 ?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "L'article officiel de ByteDance a été publié le 31/07/2026 ; le catalogue Flatkey indique released_at au 04/08/2026. Il s'agit de champs de métadonnées différents : aucune date ne représente à elle seule chaque lancement.",
    "Is Seedance 2.5 free?": "Seedance 2.5 est-il gratuit ?",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "Cette page ne promet ni accès gratuit ni quota illimité. Consultez les tarifs Flatkey en direct et les limites de votre compte avant de lancer des tâches.",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Tarifs Seedance 2.5 : 480p, 720p et références vidéo",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "La base du catalogue est de 0,14 $ ; la formule de la requête dépend de la résolution de sortie, de la durée et de l'entrée de référence vidéo.",
    "Seedance 2.5 request pricing formulas": "Formules tarifaires des requêtes Seedance 2.5",
    "Scenario": "Scénario",
    "Flatkey formula": "Formule Flatkey",
    "Billing basis": "Base de facturation",
    "Total input-video seconds": "Nombre total de secondes vidéo en entrée",
    "Output duration": "Durée de sortie",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "La base du catalogue et la formule de requête sont affichées séparément ; le règlement final suit l'estimation de la tâche et les limites du compte.",
  },
  ru: {
    "Seedance 2.5 AI Video Generator & API": "Генератор видео с ИИ и API Seedance 2.5",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "Seedance 2.5 от ByteDance — модель генерации аудио и видео для сценариев text-to-video и image-to-video. Используйте референсные материалы, управление первым/последним кадром, запросы длительностью 4–30 секунд и необязательную генерацию аудио через endpoint /v1/videos Flatkey.",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "Телеметрия запросов в реальном времени появится здесь, когда трафика Flatkey будет достаточно.",
    "Seedance 2.5 usage activity": "Активность использования Seedance 2.5",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Здесь показываются только актуальные данные запросов Flatkey; график появится после накопления достаточного трафика.",
    "Text-to-video and image-to-video": "Текст-в-видео и изображение-в-видео",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Начните с текстового описания сцены или добавьте референсные изображения уже спроектированного объекта, продукта или раскадровки.",
    "Reference media and frame control": "Референсные материалы и управление кадрами",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "Запрос может включать референсы изображений, видео и аудио, а также роли первого и последнего кадра для workflows на основе референсов.",
    "Audio-video generation": "Генерация аудио и видео",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Включайте генерацию аудио отдельной опцией запроса; не предполагайте список языков или поведение аудио, если выбранный маршрут этого не документирует.",
    "Duration and output controls": "Управление длительностью и выходом",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Перед отправкой задачи выберите 4–30 секунд, 480p или 720p, адаптивное либо поддерживаемое фиксированное соотношение сторон и настройку аудио.",
    "Seedance 2.5 features": "Возможности Seedance 2.5",
    "What is Seedance 2.5? Features for AI video generation": "Что такое Seedance 2.5? Возможности генерации видео с ИИ",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "Документированный контракт охватывает входы text-to-video и image-to-video, референсные материалы, необязательное аудио и ограниченные настройки выхода.",
    "Seedance 2.5 features: references, audio, and 30-second video": "Возможности Seedance 2.5: референсы, аудио и видео длительностью 30 секунд",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "В этом сравнении отражено документированное поведение Seedance 2.5. Значения Seedance 2.0 помечены как непроверенные, а не выведены предположительно.",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0 (не проходил повторный аудит)",
    "Not verified in this audit": "Не проверено в рамках этого аудита",
    "4–30 seconds per request": "4–30 секунд на запрос",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "Всего до 50: 30 изображений, 10 видео и 10 аудиофайлов",
    "First-frame and last-frame roles are supported": "Поддерживаются роли первого и последнего кадра",
    "Audio generation can be enabled per request": "Генерацию аудио можно включить для каждого запроса",
    "Reference-guided and first/last-frame workflows": "Рабочие процессы с референсами и первым/последним кадром",
    "Seedance 2.5 prompt guide: six workflows": "Руководство по промптам Seedance 2.5: шесть workflows",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Используйте эти промпты для конкретных workflows как отправную точку. Результат зависит от переданных референсов и настроек запроса.",
    "Why use Seedance 2.5 through Flatkey?": "Зачем использовать Seedance 2.5 через Flatkey?",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Используйте один ключ для каталога моделей, проверяйте контракт запроса и связывайте цену с выбранными настройками.",
    "One key for the model catalog": "Один ключ для каталога моделей",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Используйте один аккаунт и API-ключ Flatkey для задач с видео, изображениями, аудио и текстом.",
    "Documented video contract": "Документированный видеоконтракт",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "В интеграции явно укажите запрос Seedance в формате content[], endpoint /v1/videos и асинхронный процесс выполнения задачи.",
    "Pricing follows the request": "Цена зависит от запроса",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "Разрешение, длительность и входной видео-референс меняют формулу; значение каталога не является универсальной ценой за секунду.",
    "Live data only when available": "Данные в реальном времени — только при наличии",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Карточки производительности и активности показывают телеметрию при достаточном реальном трафике Flatkey; иначе данные не отображаются.",
    "Seedance 2.5 API: how to use /v1/videos": "API Seedance 2.5: как использовать /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Отправьте элементы content[] Seedance, сохраните task id и получите результат через /v1/videos/{task_id}/content.",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Отправьте идентификатор модели, массив content[] и поддерживаемые поля длительности, разрешения, соотношения сторон и аудио.",
    "Async task result": "Результат асинхронной задачи",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Сохраните возвращённый task id, а затем получите созданный файл через content endpoint после готовности.",
    "Reference limits": "Ограничения референсов",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "Адаптер принимает до 30 изображений, 10 видео и 10 аудиореференсов, всего до 50.",
    "Output controls": "Настройки выхода",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Выберите 480p или 720p, поддерживаемое соотношение сторон, 4–30 секунд и необходимость генерации аудио.",
    "480p · no video reference": "480p · без видео-референса",
    "720p · no video reference": "720p · без видео-референса",
    "Catalog formula": "Формула каталога",
    "Video reference input": "Входной видео-референс",
    "Depends on resolution": "Зависит от разрешения",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Цена Seedance 2.5 зависит от разрешения, длительности и входного видео-референса; база каталога не является универсальной ставкой за секунду.",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 — аудио-видеомодель ByteDance для запросов text-to-video и image-to-video с референсными материалами и необязательными настройками аудио.",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "База каталога — $0.14, но формула запроса различается: без видеовхода 480p стоит $0.140 × duration; 720p — $0.314 × duration; формулы с видео-референсом используют общее число видеосекунд и разрешение.",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Используйте модель для раскадровок микродрам и комиксов, вариантов продуктов и UGC, превизуализации фильмов, игровых синематиков, роликов авторов и креативных тестов для маркетинговых исследований.",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "Отправьте POST на /v1/videos в формате content[] Seedance, сохраните асинхронный task id и получите результат через /v1/videos/{task_id}/content.",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Да. Укажите 480p или 720p, 4–30 секунд, поддерживаемое соотношение сторон, generate_audio и документированные поля референсов/кадров.",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "Аутентификация Flatkey и общий каталог используют шаблон gateway, а видеозапросы Seedance — content[] и асинхронный контракт /v1/videos.",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "Запрос может содержать до 30 изображений, 10 видео и 10 аудиореференсов, всего 50; лимиты аккаунта и доступность могут меняться.",
    "What is the Seedance 2.5 release date?": "Какова дата выхода Seedance 2.5?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "Официальная статья ByteDance опубликована 2026-07-31; в каталоге Flatkey указано released_at: 2026-08-04. Это разные метаданные, поэтому ни одна дата сама по себе не обозначает все запуски.",
    "Is Seedance 2.5 free?": "Seedance 2.5 бесплатен?",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "Эта страница не обещает бесплатный или безлимитный доступ. Перед запуском задач проверьте актуальные цены Flatkey и ограничения аккаунта.",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Цены Seedance 2.5: 480p, 720p и видео-референсы",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "База каталога составляет $0.14; формула запроса зависит от выходного разрешения, длительности и входного видео-референса.",
    "Seedance 2.5 request pricing formulas": "Формулы оплаты запросов Seedance 2.5",
    "Scenario": "Сценарий",
    "Flatkey formula": "Формула Flatkey",
    "Billing basis": "Основа расчёта",
    "Total input-video seconds": "Общее число секунд входного видео",
    "Output duration": "Длительность выхода",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "База каталога и формула запроса показываются отдельно; итоговое списание зависит от оценки задачи и лимитов аккаунта.",
  },
  ja: {
    "Seedance 2.5 AI Video Generator & API": "Seedance 2.5 AI動画ジェネレーター＆API",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "ByteDanceのSeedance 2.5は、text-to-videoとimage-to-videoワークフロー向けの音声・動画生成モデルです。参照メディア、最初/最後のフレーム制御、4–30秒のリクエスト、オプションの音声生成をFlatkeyの/v1/videosエンドポイントから利用できます。",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "Flatkeyのトラフィックが十分になると、ここにライブリクエストテレメトリが表示されます。",
    "Seedance 2.5 usage activity": "Seedance 2.5の利用状況",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "ここにはライブのFlatkeyリクエストデータのみを表示します。十分なトラフィックが集まるとグラフが表示されます。",
    "Text-to-video and image-to-video": "テキストから動画・画像から動画",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "文章でシーンを指定するか、設計済みの被写体・商品・絵コンテの参照画像を指定して開始します。",
    "Reference media and frame control": "参照メディアとフレーム制御",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "リクエストには画像・動画・音声の参照を含められ、参照ベースのワークフローでは最初のフレームと最後のフレームの役割も指定できます。",
    "Audio-video generation": "音声・動画生成",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "音声生成はリクエストの明示的なオプションとして有効にしてください。選択したルートが文書化していない言語一覧や音声動作を想定しないでください。",
    "Duration and output controls": "長さと出力の制御",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "タスク送信前に4–30秒、480pまたは720p、適応または対応する固定アスペクト比、音声設定を選択します。",
    "Seedance 2.5 features": "Seedance 2.5の機能",
    "What is Seedance 2.5? Features for AI video generation": "Seedance 2.5とは？ AI動画生成の機能",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "文書化された契約では、text-to-videoとimage-to-video入力、参照メディア、オプションの音声、範囲が定められた出力設定に対応します。",
    "Seedance 2.5 features: references, audio, and 30-second video": "Seedance 2.5の機能：参照、音声、30秒動画",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "この比較は文書化されたSeedance 2.5の動作を記録しています。Seedance 2.0の値は推測せず、未検証として表示します。",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0（再監査なし）",
    "Not verified in this audit": "この監査では未検証",
    "4–30 seconds per request": "1リクエストあたり4–30秒",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "合計最大50件：画像30件、動画10件、音声10件",
    "First-frame and last-frame roles are supported": "最初のフレームと最後のフレームの役割に対応",
    "Audio generation can be enabled per request": "リクエストごとに音声生成を有効化できます",
    "Reference-guided and first/last-frame workflows": "参照ベースおよび最初/最後のフレームのワークフロー",
    "Seedance 2.5 prompt guide: six workflows": "Seedance 2.5プロンプトガイド：6つのワークフロー",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "これらのワークフロー別プロンプトを出発点として使用してください。出力は提供した参照とリクエスト設定によって変わります。",
    "Why use Seedance 2.5 through Flatkey?": "Seedance 2.5をFlatkeyで使う理由",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "モデルカタログに1つのキーを使い、リクエスト契約を確認し、選択した設定に応じた料金を維持します。",
    "One key for the model catalog": "モデルカタログ用の1つのキー",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "動画・画像・音声・テキストの処理で同じFlatkeyアカウントとAPIキーを使えます。",
    "Documented video contract": "文書化された動画契約",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "統合ではSeedanceのcontent[]リクエスト、/v1/videosエンドポイント、非同期タスクフローを明示してください。",
    "Pricing follows the request": "料金はリクエストに応じて変わります",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "解像度、長さ、動画参照入力によって計算式が変わります。カタログの値は一律の秒単価ではありません。",
    "Live data only when available": "利用可能な場合のみライブデータを表示",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "十分な実Flatkeyトラフィックがある場合だけパフォーマンスとアクティビティのカードにテレメトリを表示し、それ以外は未報告のままにします。",
    "Seedance 2.5 API: how to use /v1/videos": "Seedance 2.5 API：/v1/videosの使い方",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Seedanceのcontent[]要素を送信し、task idを保存して、/v1/videos/{task_id}/contentから結果を取得します。",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "model id、content[]配列、対応する長さ・解像度・比率・音声フィールドを送信します。",
    "Async task result": "非同期タスクの結果",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "リクエストが返したtask idを保存し、準備ができたらcontent endpointから生成ファイルを取得します。",
    "Reference limits": "参照の上限",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "アダプターは画像30件、動画10件、音声参照10件まで、合計50件を受け付けます。",
    "Output controls": "出力制御",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "480pまたは720p、対応する比率、4–30秒、音声を生成するかどうかを選択します。",
    "480p · no video reference": "480p · 動画参照なし",
    "720p · no video reference": "720p · 動画参照なし",
    "Catalog formula": "カタログの計算式",
    "Video reference input": "動画参照入力",
    "Depends on resolution": "解像度に依存",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Seedance 2.5の料金は解像度、長さ、動画参照入力で変わります。カタログの基準値は一律の秒単価ではありません。",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5はByteDanceの音声・動画生成モデルで、参照メディアとオプションの音声制御を使ったtext-to-videoおよびimage-to-videoリクエストに対応します。",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "カタログ基準値は$0.14ですが、リクエスト式は異なります。動画入力なしの480pは$0.140 × duration、720pは$0.314 × durationです。動画参照の式では動画の合計秒数と解像度を使います。",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "マイクロドラマやコミックの絵コンテ、商品・UGCのバリエーション、映画のプリビズ、ゲームのシネマティクス、クリエイター動画、マーケットリサーチ用クリエイティブテストに利用できます。",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "Seedanceのcontent[]形式で/v1/videosへPOSTし、非同期task idを保存して、/v1/videos/{task_id}/contentから結果を取得します。",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "はい。480pまたは720p、4–30秒、対応する比率、generate_audio、文書化された参照/フレームフィールドを設定します。",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "Flatkeyの認証と共有カタログはゲートウェイ方式を使用し、Seedanceの動画リクエストはcontent[]と非同期の/v1/videos契約を使用します。",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "1回のリクエストには画像30件、動画10件、音声参照10件まで、合計50件を含められます。アカウントのレート制限と可用性は変わる場合があります。",
    "What is the Seedance 2.5 release date?": "Seedance 2.5のリリース日はいつですか？",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "ByteDanceの公式記事は2026-07-31に公開され、Flatkeyのカタログではreleased_atが2026-08-04と記載されています。これは別々のメタデータであり、どちらか一方だけで全てのリリース日を示すものではありません。",
    "Is Seedance 2.5 free?": "Seedance 2.5は無料ですか？",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "このページでは無料または無制限の利用を約束していません。ジョブを実行する前に、Flatkeyのライブ料金とアカウント制限を確認してください。",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Seedance 2.5の料金：480p、720p、動画参照",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "カタログ基準値は$0.14ですが、リクエスト式は出力解像度、長さ、動画参照入力によって変わります。",
    "Seedance 2.5 request pricing formulas": "Seedance 2.5のリクエスト料金計算式",
    "Scenario": "シナリオ",
    "Flatkey formula": "Flatkeyの計算式",
    "Billing basis": "課金基準",
    "Total input-video seconds": "入力動画の合計秒数",
    "Output duration": "出力の長さ",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "カタログ基準値とリクエスト式は別々に表示されます。最終的な精算はタスク見積もりとアカウント制限に従います。",
  },
  vi: {
    "Seedance 2.5 AI Video Generator & API": "Trình tạo video AI Seedance 2.5 & API",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "ByteDance Seedance 2.5 là mô hình tạo video kèm âm thanh cho quy trình văn bản thành video và hình ảnh thành video. Sử dụng media tham chiếu, điều khiển khung đầu/cuối, yêu cầu 4–30 giây và âm thanh tùy chọn qua endpoint /v1/videos của Flatkey.",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "Dữ liệu giám sát yêu cầu trực tiếp sẽ hiển thị ở đây khi Flatkey có đủ lưu lượng.",
    "Seedance 2.5 usage activity": "Hoạt động sử dụng Seedance 2.5",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Chỉ hiển thị dữ liệu yêu cầu thực tế của Flatkey; biểu đồ xuất hiện sau khi thu thập đủ lưu lượng.",
    "Text-to-video and image-to-video": "Văn bản thành video và hình ảnh thành video",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Bắt đầu từ một cảnh viết sẵn hoặc cung cấp ảnh tham chiếu cho chủ thể, sản phẩm hay storyboard bạn đã thiết kế.",
    "Reference media and frame control": "Media tham chiếu và điều khiển khung hình",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "Yêu cầu có thể bao gồm tham chiếu ảnh, video và âm thanh, cùng vai trò khung đầu và khung cuối cho quy trình dựa trên tham chiếu.",
    "Audio-video generation": "Tạo video kèm âm thanh",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Bật tạo âm thanh như một tùy chọn rõ ràng trong yêu cầu; không giả định danh sách ngôn ngữ hoặc hành vi âm thanh mà route đã chọn không ghi rõ.",
    "Duration and output controls": "Điều khiển thời lượng và đầu ra",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Chọn 4–30 giây, 480p hoặc 720p, tỷ lệ thích ứng hoặc tỷ lệ cố định được hỗ trợ và cài đặt âm thanh trước khi gửi tác vụ.",
    "Seedance 2.5 features": "Tính năng Seedance 2.5",
    "What is Seedance 2.5? Features for AI video generation": "Seedance 2.5 là gì? Tính năng tạo video AI",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "Hợp đồng được tài liệu hóa bao gồm đầu vào văn bản thành video và hình ảnh thành video, media tham chiếu, âm thanh tùy chọn và các cài đặt đầu ra giới hạn.",
    "Seedance 2.5 features: references, audio, and 30-second video": "Tính năng Seedance 2.5: tham chiếu, âm thanh và video 30 giây",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "Phần so sánh này ghi lại hành vi Seedance 2.5 đã được tài liệu hóa. Các giá trị của Seedance 2.0 được đánh dấu chưa xác minh thay vì suy đoán.",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0 (chưa kiểm tra lại)",
    "Not verified in this audit": "Chưa xác minh trong lần kiểm tra này",
    "4–30 seconds per request": "4–30 giây mỗi yêu cầu",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "Tối đa 50 mục: 30 ảnh, 10 video và 10 âm thanh",
    "First-frame and last-frame roles are supported": "Hỗ trợ vai trò khung đầu và khung cuối",
    "Audio generation can be enabled per request": "Có thể bật tạo âm thanh cho từng yêu cầu",
    "Reference-guided and first/last-frame workflows": "Quy trình dựa trên tham chiếu và khung đầu/cuối",
    "Seedance 2.5 prompt guide: six workflows": "Hướng dẫn prompt Seedance 2.5: sáu quy trình",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Dùng các prompt theo từng quy trình này làm điểm bắt đầu. Kết quả phụ thuộc vào tham chiếu và cài đặt yêu cầu được cung cấp.",
    "Why use Seedance 2.5 through Flatkey?": "Tại sao dùng Seedance 2.5 qua Flatkey?",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Dùng một key cho danh mục mô hình, kiểm tra hợp đồng yêu cầu và gắn giá với các cài đặt đã chọn.",
    "One key for the model catalog": "Một key cho danh mục mô hình",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Dùng cùng tài khoản Flatkey và API key cho các tác vụ video, ảnh, âm thanh và văn bản.",
    "Documented video contract": "Hợp đồng video được tài liệu hóa",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "Trong tích hợp, hãy nêu rõ yêu cầu content[] của Seedance, endpoint /v1/videos và quy trình tác vụ bất đồng bộ.",
    "Pricing follows the request": "Giá phụ thuộc vào yêu cầu",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "Độ phân giải, thời lượng và đầu vào tham chiếu video thay đổi công thức; giá trị trong danh mục không phải cam kết giá cố định cho mỗi giây.",
    "Live data only when available": "Chỉ hiển thị dữ liệu trực tiếp khi có",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Thẻ hiệu suất và hoạt động hiển thị dữ liệu giám sát khi có đủ lưu lượng Flatkey thực tế; nếu không, chúng giữ trạng thái chưa báo cáo.",
    "Seedance 2.5 API: how to use /v1/videos": "API Seedance 2.5: cách dùng /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Gửi các mục content[] của Seedance, lưu task id và lấy kết quả từ /v1/videos/{task_id}/content.",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Gửi model id, mảng content[] và các trường thời lượng, độ phân giải, tỷ lệ và âm thanh được hỗ trợ.",
    "Async task result": "Kết quả tác vụ bất đồng bộ",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Lưu task id do yêu cầu trả về, sau đó lấy tệp đã tạo từ content endpoint khi sẵn sàng.",
    "Reference limits": "Giới hạn tham chiếu",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "Adapter chấp nhận tối đa 30 ảnh, 10 video và 10 tham chiếu âm thanh, tổng cộng 50.",
    "Output controls": "Điều khiển đầu ra",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Chọn 480p hoặc 720p, tỷ lệ được hỗ trợ, 4–30 giây và có tạo âm thanh hay không.",
    "480p · no video reference": "480p · không có tham chiếu video",
    "720p · no video reference": "720p · không có tham chiếu video",
    "Catalog formula": "Công thức danh mục",
    "Video reference input": "Đầu vào tham chiếu video",
    "Depends on resolution": "Phụ thuộc vào độ phân giải",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Giá Seedance 2.5 thay đổi theo độ phân giải, thời lượng và đầu vào tham chiếu video; giá cơ sở trong danh mục không phải mức giá cố định cho mỗi giây.",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 là mô hình tạo video kèm âm thanh của ByteDance cho các yêu cầu văn bản thành video và hình ảnh thành video, với media tham chiếu và điều khiển âm thanh tùy chọn.",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "Giá cơ sở trong danh mục là $0.14, nhưng công thức yêu cầu thay đổi: 480p không có đầu vào video là $0.140 × thời lượng; 720p là $0.314 × thời lượng; công thức có tham chiếu video sử dụng tổng số giây video và độ phân giải.",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Dùng cho storyboard phim ngắn và truyện tranh, biến thể sản phẩm và UGC, tiền kỳ phim, cảnh điện ảnh trò chơi, clip nhà sáng tạo và thử nghiệm ý tưởng cho nghiên cứu thị trường.",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "POST tới /v1/videos theo định dạng content[] của Seedance, giữ task id bất đồng bộ và lấy kết quả từ /v1/videos/{task_id}/content.",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Có. Đặt 480p hoặc 720p, 4–30 giây, tỷ lệ được hỗ trợ, generate_audio và các trường tham chiếu/khung hình đã được tài liệu hóa.",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "Xác thực Flatkey và danh mục dùng chung theo mô hình gateway; yêu cầu video Seedance dùng content[] và hợp đồng /v1/videos bất đồng bộ.",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "Một yêu cầu có thể chứa tối đa 30 ảnh, 10 video và 10 tham chiếu âm thanh, tổng cộng 50; giới hạn tốc độ và khả dụng của tài khoản có thể thay đổi.",
    "What is the Seedance 2.5 release date?": "Ngày phát hành Seedance 2.5 là khi nào?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "Bài viết chính thức của ByteDance được đăng ngày 2026-07-31; danh mục Flatkey ghi released_at là 2026-08-04. Đây là hai trường siêu dữ liệu khác nhau, vì vậy không ngày nào tự nó đại diện cho mọi lần ra mắt.",
    "Is Seedance 2.5 free?": "Seedance 2.5 có miễn phí không?",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "Trang này không cam kết quyền truy cập miễn phí hoặc không giới hạn. Hãy xem dữ liệu giá Flatkey trực tiếp và giới hạn tài khoản trước khi chạy tác vụ.",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Giá Seedance 2.5: 480p, 720p và tham chiếu video",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "Giá cơ sở danh mục là $0.14; công thức yêu cầu phụ thuộc vào độ phân giải đầu ra, thời lượng và đầu vào tham chiếu video.",
    "Seedance 2.5 request pricing formulas": "Công thức tính giá yêu cầu Seedance 2.5",
    "Scenario": "Tình huống",
    "Flatkey formula": "Công thức Flatkey",
    "Billing basis": "Cơ sở tính phí",
    "Total input-video seconds": "Tổng số giây video đầu vào",
    "Output duration": "Thời lượng đầu ra",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "Giá cơ sở danh mục và công thức yêu cầu được hiển thị riêng; quyết toán cuối cùng tuân theo ước tính tác vụ và giới hạn tài khoản.",
  },
  de: {
      "Seedance 2.5 AI Video Generator & API": "Seedance 2.5 KI-Videogenerator & API",
      "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "ByteDance Seedance 2.5 ist ein Modell zur Audio- und Videogenerierung für Text-zu-Video- und Bild-zu-Video-Workflows. Verwende Referenzmedien, Erst-/Letztframe-Steuerung, Anfragen mit 4–30 Sekunden und optionales Audio über den Flatkey-Endpunkt /v1/videos.",
      "Live request telemetry appears here when enough Flatkey traffic is available.": "Live-Anfrage-Telemetrie wird hier angezeigt, sobald genügend Flatkey-Traffic verfügbar ist.",
      "Seedance 2.5 usage activity": "Nutzungsaktivität von Seedance 2.5",
      "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Hier werden nur Live-Daten von Flatkey-Anfragen angezeigt; ein Diagramm erscheint, sobald genügend Traffic gesammelt wurde.",
      "Text-to-video and image-to-video": "Text-zu-Video und Bild-zu-Video",
      "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Beginne mit einer schriftlich beschriebenen Szene oder stelle Referenzbilder für ein bereits entworfenes Motiv, Produkt oder Storyboard bereit.",
      "Reference media and frame control": "Referenzmedien und Frame-Steuerung",
      "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "Die Anfrage kann Bild-, Video- und Audio-Referenzen sowie die Rollen für erstes und letztes Frame enthalten, um referenzbasierte Workflows zu unterstützen.",
      "Audio-video generation": "Audio- und Videogenerierung",
      "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Aktiviere die Audiogenerierung als ausdrückliche Anfrageoption; nimm keine Sprachliste oder kein Audioverhalten an, das die gewählte Route nicht dokumentiert.",
      "Duration and output controls": "Steuerung von Dauer und Ausgabe",
      "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Wähle vor dem Absenden der Aufgabe 4–30 Sekunden, 480p oder 720p, adaptive oder unterstützte feste Seitenverhältnisse sowie die Audioeinstellung.",
      "Seedance 2.5 features": "Funktionen von Seedance 2.5",
      "What is Seedance 2.5? Features for AI video generation": "Was ist Seedance 2.5? Funktionen für KI-Videogenerierung",
      "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "Der dokumentierte Vertrag umfasst Text-zu-Video- und Bild-zu-Video-Eingaben, Referenzmedien, optionales Audio und begrenzte Ausgabeeinstellungen.",
      "Seedance 2.5 features: references, audio, and 30-second video": "Funktionen von Seedance 2.5: Referenzen, Audio und 30-Sekunden-Video",
      "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "Dieser Vergleich hält das dokumentierte Verhalten von Seedance 2.5 fest. Werte für Seedance 2.0 sind als nicht verifiziert gekennzeichnet und werden nicht abgeleitet.",
      "Seedance 2.0 (not re-audited)": "Seedance 2.0 (nicht erneut geprüft)",
      "Not verified in this audit": "In diesem Audit nicht verifiziert",
      "4–30 seconds per request": "4–30 Sekunden pro Anfrage",
      "Up to 50 total: 30 images, 10 videos, and 10 audio": "Insgesamt bis zu 50: 30 Bilder, 10 Videos und 10 Audiodateien",
      "First-frame and last-frame roles are supported": "Rollen für erstes und letztes Frame werden unterstützt",
      "Audio generation can be enabled per request": "Die Audiogenerierung kann pro Anfrage aktiviert werden",
      "Reference-guided and first/last-frame workflows": "Referenzgeführte und Erst-/Letztframe-Workflows",
      "Seedance 2.5 prompt guide: six workflows": "Seedance 2.5 Prompt-Leitfaden: sechs Workflows",
      "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Verwende diese workflowspezifischen Prompts als Ausgangspunkt. Die Ausgabe hängt von den bereitgestellten Referenzen und den Anfrageeinstellungen ab.",
      "Why use Seedance 2.5 through Flatkey?": "Warum Seedance 2.5 über Flatkey verwenden?",
      "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Verwende einen Schlüssel für den Modellkatalog, prüfe den Anfragevertrag und richte die Preisberechnung an den ausgewählten Einstellungen aus.",
      "One key for the model catalog": "Ein Schlüssel für den Modellkatalog",
      "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Verwende dasselbe Flatkey-Konto und denselben API-Schlüssel für Video-, Bild-, Audio- und Textaufgaben.",
      "Documented video contract": "Dokumentierter Videovertrag",
      "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "Halte in deiner Integration die Seedance-Anfrage im Format content[], den Endpunkt /v1/videos und den asynchronen Aufgabenablauf ausdrücklich fest.",
      "Pricing follows the request": "Die Preisberechnung richtet sich nach der Anfrage",
      "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "Auflösung, Dauer und Video-Referenzeingabe ändern die Formel; der Katalogwert ist kein allgemeingültiger Preis pro Sekunde.",
      "Live data only when available": "Live-Daten nur bei Verfügbarkeit",
      "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Leistungs- und Aktivitätskarten zeigen Telemetrie, sobald genügend echter Flatkey-Traffic vorhanden ist; andernfalls werden keine Daten gemeldet.",
      "Seedance 2.5 API: how to use /v1/videos": "Seedance 2.5 API: So verwendest du /v1/videos",
      "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Sende content[]-Elemente für Seedance, bewahre die task id auf und rufe das Ergebnis unter /v1/videos/{task_id}/content ab.",
      "POST /v1/videos": "POST /v1/videos",
      "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Sende die Modell-ID, ein content[]-Array sowie unterstützte Felder für Dauer, Auflösung, Seitenverhältnis und Audio.",
      "Async task result": "Asynchrones Aufgabenergebnis",
      "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Speichere die von der Anfrage zurückgegebene task id und rufe die erzeugte Datei über den Content-Endpunkt ab, sobald sie bereitsteht.",
      "Reference limits": "Referenzlimits",
      "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "Der Adapter akzeptiert bis zu 30 Bildreferenzen, 10 Videoreferenzen und 10 Audioreferenzen, insgesamt 50.",
      "Output controls": "Ausgabesteuerung",
      "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Wähle 480p oder 720p, ein unterstütztes Seitenverhältnis, 4–30 Sekunden und ob Audio generiert werden soll.",
      "480p · no video reference": "480p · ohne Videoreferenz",
      "720p · no video reference": "720p · ohne Videoreferenz",
      "Catalog formula": "Katalogformel",
      "Video reference input": "Videoreferenzeingabe",
      "Depends on resolution": "Abhängig von der Auflösung",
      "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Die Preise für Seedance 2.5 variieren je nach Auflösung, Dauer und Video-Referenzeingabe; der Katalogbasiswert ist kein allgemeingültiger Preis pro Sekunde.",
      "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 ist das Audio- und Videogenerierungsmodell von ByteDance für Text-zu-Video- und Bild-zu-Video-Anfragen mit Referenzmedien und optionaler Audiosteuerung.",
      "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "Der Katalogbasiswert beträgt $0.14, aber die Anfrageformel variiert: 480p ohne Videoeingabe entspricht $0.140 × duration; 720p entspricht $0.314 × duration; Formeln mit Videoreferenz verwenden die gesamte Videodauer in Sekunden und die Auflösung.",
      "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Verwende es für Mikrodrama- und Comic-Storyboards, Produkt- und UGC-Varianten, Film-Previsualisierung, Game-Cinematics, Creator-Clips und kreative Tests in der Marktforschung.",
      "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "Führe einen POST an /v1/videos im Seedance-content[]-Format aus, bewahre die asynchrone task id auf und rufe das Ergebnis unter /v1/videos/{task_id}/content ab.",
      "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Ja. Setze 480p oder 720p, 4–30 Sekunden, ein unterstütztes Seitenverhältnis, generate_audio sowie die dokumentierten Referenz-/Frame-Felder.",
      "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "Die Flatkey-Authentifizierung und der gemeinsame Katalog verwenden das Gateway-Muster, während Seedance-Videoanfragen content[] und den asynchronen Vertrag für /v1/videos verwenden.",
      "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "Eine Anfrage kann bis zu 30 Bilder, 10 Videos und 10 Audio-Referenzen enthalten, insgesamt 50 Referenzen; Kontolimits für Anfragen und Verfügbarkeit können sich ändern.",
      "What is the Seedance 2.5 release date?": "Was ist das Veröffentlichungsdatum von Seedance 2.5?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "Der offizielle ByteDance-Artikel wurde am 31.07.2026 veröffentlicht; der Flatkey-Katalog führt released_at als 04.08.2026. Dies sind unterschiedliche Metadatenfelder, daher steht kein Datum allein für jede Markteinführung.",
      "Is Seedance 2.5 free?": "Ist Seedance 2.5 kostenlos?",
      "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "Auf dieser Seite wird kein kostenloser oder unbegrenzter Zugang zugesichert. Prüfe vor dem Ausführen von Jobs die aktuellen Flatkey-Preisdaten und deine Kontolimits.",
      "Seedance 2.5 pricing: 480p, 720p, and video references": "Preise für Seedance 2.5: 480p, 720p und Videoreferenzen",
      "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "Der Katalogbasiswert beträgt $0.14; die Anfrageformel hängt von Ausgabeauflösung, Dauer und Video-Referenzeingabe ab.",
      "Seedance 2.5 request pricing formulas": "Preisformeln für Anfragen mit Seedance 2.5",
      "Scenario": "Szenario",
      "Flatkey formula": "Flatkey-Formel",
      "Billing basis": "Abrechnungsgrundlage",
      "Total input-video seconds": "Gesamte Sekunden der Eingangsvideos",
      "Output duration": "Ausgabedauer",
      "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "Katalogbasiswert und Anfrageformel werden getrennt angezeigt; die Endabrechnung richtet sich nach der Aufgabenschätzung und den Kontolimits.",
    },
  id: {
    "Seedance 2.5 AI Video Generator & API": "Generator Video AI & API Seedance 2.5",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "Seedance 2.5 dari ByteDance adalah model generasi audio-video untuk alur kerja teks-ke-video dan gambar-ke-video. Gunakan media referensi, kontrol frame pertama/terakhir, permintaan 4–30 detik, dan audio opsional melalui endpoint /v1/videos Flatkey.",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "Telemetri permintaan langsung akan muncul di sini jika trafik Flatkey mencukupi.",
    "Seedance 2.5 usage activity": "Aktivitas penggunaan Seedance 2.5",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Hanya data permintaan langsung Flatkey yang ditampilkan di sini; grafik akan muncul setelah trafik yang cukup terkumpul.",
    "Text-to-video and image-to-video": "Teks-ke-video dan gambar-ke-video",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Mulai dari adegan tertulis atau berikan gambar referensi untuk subjek, produk, atau storyboard yang sudah Anda rancang.",
    "Reference media and frame control": "Media referensi dan kontrol frame",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "Permintaan dapat menyertakan referensi gambar, video, dan audio, serta peran frame pertama dan terakhir untuk alur kerja berbasis referensi.",
    "Audio-video generation": "Generasi audio-video",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Aktifkan generasi audio sebagai opsi permintaan yang eksplisit; jangan mengasumsikan daftar bahasa atau perilaku audio yang tidak didokumentasikan oleh rute terpilih.",
    "Duration and output controls": "Kontrol durasi dan output",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Pilih 4–30 detik, 480p atau 720p, rasio adaptif atau rasio tetap yang didukung, serta pengaturan audio sebelum mengirim tugas.",
    "Seedance 2.5 features": "Fitur Seedance 2.5",
    "What is Seedance 2.5? Features for AI video generation": "Apa itu Seedance 2.5? Fitur untuk generasi video AI",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "Kontrak yang terdokumentasi mencakup input teks-ke-video dan gambar-ke-video, media referensi, audio opsional, serta pengaturan output yang dibatasi.",
    "Seedance 2.5 features: references, audio, and 30-second video": "Fitur Seedance 2.5: referensi, audio, dan video 30 detik",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "Perbandingan ini mencatat perilaku Seedance 2.5 yang terdokumentasi. Nilai Seedance 2.0 ditandai belum terverifikasi, bukan disimpulkan.",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0 (belum diaudit ulang)",
    "Not verified in this audit": "Belum terverifikasi dalam audit ini",
    "4–30 seconds per request": "4–30 detik per permintaan",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "Maksimal 50 total: 30 gambar, 10 video, dan 10 audio",
    "First-frame and last-frame roles are supported": "Peran frame pertama dan terakhir didukung",
    "Audio generation can be enabled per request": "Generasi audio dapat diaktifkan untuk setiap permintaan",
    "Reference-guided and first/last-frame workflows": "Alur kerja berbasis referensi dan frame pertama/terakhir",
    "Seedance 2.5 prompt guide: six workflows": "Panduan prompt Seedance 2.5: enam alur kerja",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Gunakan prompt khusus alur kerja ini sebagai titik awal. Output bergantung pada referensi dan pengaturan permintaan yang diberikan.",
    "Why use Seedance 2.5 through Flatkey?": "Mengapa menggunakan Seedance 2.5 melalui Flatkey?",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Gunakan satu kunci untuk katalog model, periksa kontrak permintaan, dan kaitkan harga dengan pengaturan yang dipilih.",
    "One key for the model catalog": "Satu kunci untuk katalog model",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Gunakan akun dan kunci API Flatkey yang sama untuk pekerjaan video, gambar, audio, dan teks.",
    "Documented video contract": "Kontrak video yang terdokumentasi",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "Jadikan permintaan content[] Seedance, endpoint /v1/videos, dan alur tugas asinkron eksplisit dalam integrasi Anda.",
    "Pricing follows the request": "Harga mengikuti permintaan",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "Resolusi, durasi, dan input referensi video mengubah rumus; nilai katalog bukan janji tarif universal per detik.",
    "Live data only when available": "Data langsung hanya saat tersedia",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Kartu performa dan aktivitas menampilkan telemetri jika trafik Flatkey nyata mencukupi; jika tidak, kartu tetap tanpa data.",
    "Seedance 2.5 API: how to use /v1/videos": "API Seedance 2.5: cara menggunakan /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Kirim item content[] Seedance, simpan ID tugas, lalu ambil hasil dari /v1/videos/{task_id}/content.",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Kirim ID model, array content[], serta field durasi, resolusi, rasio, dan audio yang didukung.",
    "Async task result": "Hasil tugas asinkron",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Simpan ID tugas yang dikembalikan permintaan, lalu ambil file yang dihasilkan dari endpoint content saat sudah siap.",
    "Reference limits": "Batas referensi",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "Adaptor menerima hingga 30 gambar, 10 video, dan 10 referensi audio, dengan total 50.",
    "Output controls": "Kontrol output",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Pilih 480p atau 720p, rasio yang didukung, 4–30 detik, dan apakah audio akan dibuat.",
    "480p · no video reference": "480p · tanpa referensi video",
    "720p · no video reference": "720p · tanpa referensi video",
    "Catalog formula": "Rumus katalog",
    "Video reference input": "Input referensi video",
    "Depends on resolution": "Bergantung pada resolusi",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Harga Seedance 2.5 bervariasi menurut resolusi, durasi, dan input referensi video; dasar katalog bukan tarif universal per detik.",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 adalah model generasi audio-video ByteDance untuk permintaan teks-ke-video dan gambar-ke-video, dengan media referensi dan kontrol audio opsional.",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "Dasar katalog adalah $0.14, tetapi rumus permintaan bervariasi: 480p tanpa input video adalah $0.140 × durasi; 720p adalah $0.314 × durasi; rumus dengan referensi video menggunakan total detik video dan resolusi.",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Gunakan untuk storyboard mikro-drama dan komik, variasi produk dan UGC, pravisualisasi film, sinematik game, klip kreator, dan uji kreatif riset pasar.",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "Lakukan POST ke /v1/videos dengan format content[] Seedance, simpan ID tugas asinkron, lalu ambil hasil dari /v1/videos/{task_id}/content.",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Ya. Atur 480p atau 720p, 4–30 detik, rasio yang didukung, generate_audio, dan field referensi/frame yang terdokumentasi.",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "Autentikasi Flatkey dan katalog bersama menggunakan pola gateway, sedangkan permintaan video Seedance menggunakan content[] dan kontrak asinkron /v1/videos.",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "Satu permintaan dapat menyertakan hingga 30 gambar, 10 video, dan 10 referensi audio, dengan total 50 referensi; batas laju akun dan ketersediaan dapat berubah.",
    "What is the Seedance 2.5 release date?": "Kapan Seedance 2.5 dirilis?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "Artikel resmi ByteDance diterbitkan pada 2026-07-31; katalog Flatkey mencantumkan released_at sebagai 2026-08-04. Ini adalah bidang metadata yang berbeda, jadi tidak satu pun tanggal tersebut mewakili semua peluncuran.",
    "Is Seedance 2.5 free?": "Apakah Seedance 2.5 gratis?",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "Halaman ini tidak menjanjikan akses gratis atau tanpa batas. Gunakan data harga Flatkey langsung dan batas akun Anda sebelum menjalankan tugas.",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Harga Seedance 2.5: 480p, 720p, dan referensi video",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "Dasar katalog adalah $0.14; rumus permintaan bergantung pada resolusi output, durasi, dan input referensi video.",
    "Seedance 2.5 request pricing formulas": "Rumus harga permintaan Seedance 2.5",
    "Scenario": "Skenario",
    "Flatkey formula": "Rumus Flatkey",
    "Billing basis": "Dasar penagihan",
    "Total input-video seconds": "Total detik video input",
    "Output duration": "Durasi output",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "Dasar katalog dan rumus permintaan ditampilkan secara terpisah; penyelesaian akhir mengikuti estimasi tugas dan batas akun.",
  },
  pt: {
    "Seedance 2.5 AI Video Generator & API": "Gerador de vídeo IA Seedance 2.5 e API",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "O Seedance 2.5 da ByteDance é um modelo de geração audiovisual para fluxos de texto para vídeo e imagem para vídeo. Use mídia de referência, controle do primeiro/último quadro, solicitações de 4–30 segundos e áudio opcional pelo endpoint /v1/videos da Flatkey.",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "A telemetria de solicitações ao vivo aparece aqui quando houver tráfego suficiente na Flatkey.",
    "Seedance 2.5 usage activity": "Atividade de uso do Seedance 2.5",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "Somente dados reais de solicitações da Flatkey são exibidos; o gráfico aparece quando houver dados suficientes.",
    "Text-to-video and image-to-video": "Texto para vídeo e imagem para vídeo",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "Comece com uma cena escrita ou forneça imagens de referência de um sujeito, produto ou storyboard já criado.",
    "Reference media and frame control": "Mídia de referência e controle de quadros",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "A solicitação pode incluir referências de imagem, vídeo e áudio, além das funções de primeiro e último quadro.",
    "Audio-video generation": "Geração audiovisual",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "Ative a geração de áudio como opção explícita; não presuma idiomas ou comportamentos que a rota selecionada não documente.",
    "Duration and output controls": "Controles de duração e saída",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "Escolha 4–30 segundos, 480p ou 720p, proporção adaptativa ou fixa compatível e a opção de áudio antes de enviar a tarefa.",
    "Seedance 2.5 features": "Recursos do Seedance 2.5",
    "What is Seedance 2.5? Features for AI video generation": "O que é o Seedance 2.5? Recursos para geração de vídeo por IA",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "O contrato documentado cobre entradas de texto para vídeo e imagem para vídeo, mídia de referência, áudio opcional e configurações de saída limitadas.",
    "Seedance 2.5 features: references, audio, and 30-second video": "Recursos do Seedance 2.5: referências, áudio e vídeo de 30 segundos",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "Esta comparação registra o comportamento documentado do Seedance 2.5. Valores do Seedance 2.0 não verificados são marcados como tal.",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0 (não reauditado)",
    "Not verified in this audit": "Não verificado nesta auditoria",
    "4–30 seconds per request": "4–30 segundos por solicitação",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "Até 50 no total: 30 imagens, 10 vídeos e 10 áudios",
    "First-frame and last-frame roles are supported": "Funções de primeiro e último quadro são compatíveis",
    "Audio generation can be enabled per request": "A geração de áudio pode ser ativada por solicitação",
    "Reference-guided and first/last-frame workflows": "Fluxos guiados por referência e pelo primeiro/último quadro",
    "Seedance 2.5 prompt guide: six workflows": "Guia de prompts do Seedance 2.5: seis fluxos",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "Use estes prompts específicos de cada fluxo como ponto de partida. A saída depende das referências e configurações enviadas.",
    "Why use Seedance 2.5 through Flatkey?": "Por que usar o Seedance 2.5 pela Flatkey?",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "Use uma chave para o catálogo, confira o contrato da solicitação e mantenha o preço ligado às configurações escolhidas.",
    "One key for the model catalog": "Uma chave para o catálogo de modelos",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "Use a mesma conta e chave de API Flatkey para tarefas de vídeo, imagem, áudio e texto.",
    "Documented video contract": "Contrato de vídeo documentado",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "Mantenha explícitos na integração a solicitação content[] do Seedance, o endpoint /v1/videos e o fluxo assíncrono.",
    "Pricing follows the request": "O preço segue a solicitação",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "Resolução, duração e entrada de referência em vídeo alteram a fórmula; o valor do catálogo não é uma tarifa universal por segundo.",
    "Live data only when available": "Dados ao vivo somente quando disponíveis",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "Os cartões exibem telemetria quando há tráfego real suficiente; caso contrário, os dados ficam sem relatório.",
    "Seedance 2.5 API: how to use /v1/videos": "API do Seedance 2.5: como usar /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "Envie itens content[] do Seedance, guarde o task id e busque o resultado em /v1/videos/{task_id}/content.",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "Envie o ID do modelo, um array content[] e os campos compatíveis de duração, resolução, proporção e áudio.",
    "Async task result": "Resultado da tarefa assíncrona",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "Salve o task id retornado e recupere o arquivo gerado no endpoint de conteúdo quando estiver pronto.",
    "Reference limits": "Limites de referências",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "O adaptador aceita até 30 imagens, 10 vídeos e 10 referências de áudio, com 50 no total.",
    "Output controls": "Controles de saída",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "Escolha 480p ou 720p, uma proporção compatível, 4–30 segundos e se deseja gerar áudio.",
    "480p · no video reference": "480p · sem referência de vídeo",
    "720p · no video reference": "720p · sem referência de vídeo",
    "Catalog formula": "Fórmula do catálogo",
    "Video reference input": "Entrada de referência de vídeo",
    "Depends on resolution": "Depende da resolução",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "O preço do Seedance 2.5 varia por resolução, duração e entrada de referência de vídeo; a base do catálogo não é uma tarifa universal por segundo.",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "O Seedance 2.5 é o modelo audiovisual da ByteDance para solicitações de texto para vídeo e imagem para vídeo, com mídia de referência e controles de áudio opcionais.",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "A base do catálogo é US$ 0,14, mas a fórmula varia: 480p sem entrada de vídeo é US$ 0,140 × duração; 720p é US$ 0,314 × duração; referências de vídeo usam os segundos totais e a resolução.",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "Use para storyboards de microdramas e quadrinhos, variações de produto e UGC, pré-visualização de filmes, cinemáticas de jogos, clipes de criadores e testes de pesquisa de mercado.",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "Faça POST em /v1/videos no formato content[] do Seedance, guarde o task id assíncrono e busque o resultado em /v1/videos/{task_id}/content.",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "Sim. Defina 480p ou 720p, 4–30 segundos, uma proporção compatível, generate_audio e os campos documentados de referência/quadro.",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "A autenticação e o catálogo compartilhado da Flatkey seguem o padrão de gateway; as solicitações de vídeo usam content[] e o contrato assíncrono /v1/videos do Seedance.",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "Uma solicitação pode incluir até 30 imagens, 10 vídeos e 10 referências de áudio, com 50 referências no total; limites e disponibilidade podem mudar.",
    "What is the Seedance 2.5 release date?": "Qual é a data de lançamento do Seedance 2.5?",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "O artigo oficial da ByteDance foi publicado em 2026-07-31; o catálogo da Flatkey lista released_at como 2026-08-04. São metadados diferentes, portanto nenhuma das datas, isoladamente, representa todos os lançamentos.",
    "Is Seedance 2.5 free?": "O Seedance 2.5 é gratuito?",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "Esta página não promete acesso gratuito ou ilimitado. Confira os preços ao vivo da Flatkey e os limites da conta antes de executar tarefas.",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Preços do Seedance 2.5: 480p, 720p e referências de vídeo",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "A base do catálogo é US$ 0,14; a fórmula da solicitação depende da resolução de saída, da duração e da entrada de referência de vídeo.",
    "Seedance 2.5 request pricing formulas": "Fórmulas de preço por solicitação do Seedance 2.5",
    "Scenario": "Cenário",
    "Flatkey formula": "Fórmula Flatkey",
    "Billing basis": "Base de cobrança",
    "Total input-video seconds": "Total de segundos dos vídeos de entrada",
    "Output duration": "Duração da saída",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "A base do catálogo e a fórmula da solicitação são mostradas separadamente; a cobrança final segue a estimativa da tarefa e os limites da conta.",
  },
  zh: {
    "Seedance 2.5 AI Video Generator & API": "Seedance 2.5 AI 视频生成器与 API",
    "ByteDance Seedance 2.5 is an audio-video generation model for text-to-video and image-to-video workflows. Use reference media, first/last-frame controls, 4–30-second requests, and optional audio through Flatkey's /v1/videos endpoint.": "ByteDance Seedance 2.5 是一款音视频生成模型，支持文生视频和图生视频工作流。可通过 Flatkey 的 /v1/videos 使用参考素材、首尾帧控制、4–30 秒请求和可选音频。",
    "Live request telemetry appears here when enough Flatkey traffic is available.": "当 Flatkey 积累足够请求流量后，这里会显示实时请求遥测数据。",
    "Seedance 2.5 usage activity": "Seedance 2.5 使用活动",
    "Only live Flatkey request data is shown here; a chart appears after enough traffic is collected.": "这里只展示真实的 Flatkey 请求数据；积累足够流量后才会显示图表。",
    "Text-to-video and image-to-video": "文生视频与图生视频",
    "Start from a written scene or supply reference images for a subject, product, or storyboard you have already designed.": "可以从文字场景开始，也可以为已设计的主体、产品或分镜提供参考图。",
    "Reference media and frame control": "参考素材与帧控制",
    "The request can include image, video, and audio references, plus first-frame and last-frame roles for reference-led workflows.": "请求可包含图片、视频和音频参考素材，并支持在参考驱动工作流中指定首帧和尾帧角色。",
    "Audio-video generation": "音视频生成",
    "Enable audio generation as an explicit request option; do not assume a language list or audio behavior that the selected route does not document.": "将音频生成作为明确的请求选项；不要假设所选路由未文档化的语言列表或音频行为。",
    "Duration and output controls": "时长与输出控制",
    "Choose 4–30 seconds, 480p or 720p, adaptive or supported fixed ratios, and the audio setting before submitting the task.": "提交任务前可选择 4–30 秒、480p 或 720p、自适应或支持的固定比例，以及音频设置。",
    "Seedance 2.5 features": "Seedance 2.5 功能",
    "What is Seedance 2.5? Features for AI video generation": "什么是 Seedance 2.5？AI 视频生成能力",
    "The documented contract covers text-to-video and image-to-video inputs, reference media, optional audio, and bounded output settings.": "文档化契约覆盖文生视频和图生视频输入、参考素材、可选音频及受限输出设置。",
    "Seedance 2.5 features: references, audio, and 30-second video": "Seedance 2.5 功能：参考素材、音频与 30 秒视频",
    "This comparison records documented Seedance 2.5 behavior. Seedance 2.0 values are marked as not verified rather than inferred.": "此对比只记录已文档化的 Seedance 2.5 行为；Seedance 2.0 的未核实值不会被推测。",
    "Seedance 2.0 (not re-audited)": "Seedance 2.0（未重新审计）",
    "Not verified in this audit": "本次审计未核实",
    "4–30 seconds per request": "每次请求 4–30 秒",
    "Up to 50 total: 30 images, 10 videos, and 10 audio": "总计最多 50 个：30 张图片、10 个视频和 10 个音频",
    "First-frame and last-frame roles are supported": "支持首帧和尾帧角色",
    "Audio generation can be enabled per request": "可在每次请求中启用音频生成",
    "Reference-guided and first/last-frame workflows": "参考驱动及首尾帧工作流",
    "Seedance 2.5 prompt guide: six workflows": "Seedance 2.5 提示词指南：六种工作流",
    "Use these workflow-specific prompts as starting points. Output depends on the supplied references and request settings.": "这些按工作流编写的提示词可作为起点；输出取决于参考素材和请求设置。",
    "Why use Seedance 2.5 through Flatkey?": "为什么通过 Flatkey 使用 Seedance 2.5？",
    "Use one key for the model catalog, inspect the request contract, and keep pricing tied to the selected settings.": "一个 Key 覆盖模型目录，清楚查看请求契约，并让价格与所选设置保持一致。",
    "One key for the model catalog": "一个 Key 覆盖模型目录",
    "Use the same Flatkey account and API key across video, image, audio, and text workloads.": "视频、图片、音频和文本任务共用同一个 Flatkey 账号与 API Key。",
    "Documented video contract": "文档化视频契约",
    "Keep Seedance's content[] request, /v1/videos endpoint, and asynchronous task flow explicit in your integration.": "在集成中明确 Seedance 的 content[] 请求、/v1/videos endpoint 和异步任务流程。",
    "Pricing follows the request": "价格随请求变化",
    "Resolution, duration, and video-reference input change the formula; the catalog value is not a universal per-second promise.": "分辨率、时长和视频参考输入会改变公式；目录值不是统一的每秒价格承诺。",
    "Live data only when available": "仅在有数据时显示实时指标",
    "Performance and activity cards show telemetry when enough real Flatkey traffic exists, otherwise they stay unreported.": "只有积累足够真实 Flatkey 流量时才显示遥测卡片，否则保持未报告状态。",
    "Seedance 2.5 API: how to use /v1/videos": "Seedance 2.5 API：如何使用 /v1/videos",
    "Send Seedance content[] items, keep the task id, and fetch the result from /v1/videos/{task_id}/content.": "发送 Seedance content[] 项，保存 task id，并从 /v1/videos/{task_id}/content 获取结果。",
    "POST /v1/videos": "POST /v1/videos",
    "Send the model id, a content[] array, and supported duration, resolution, ratio, and audio fields.": "发送模型 ID、content[] 数组，以及支持的时长、分辨率、比例和音频字段。",
    "Async task result": "异步任务结果",
    "Save the task id returned by the request, then retrieve the generated file from the content endpoint when ready.": "保存请求返回的 task id，任务完成后从 content endpoint 获取生成文件。",
    "Reference limits": "参考素材限制",
    "The adapter accepts up to 30 images, 10 videos, and 10 audio references, with 50 total.": "适配器最多接受 30 张图片、10 个视频和 10 个音频参考素材，总数 50 个。",
    "Output controls": "输出控制",
    "Choose 480p or 720p, a supported ratio, 4–30 seconds, and whether to generate audio.": "选择 480p 或 720p、支持的比例、4–30 秒，以及是否生成音频。",
    "480p · no video reference": "480p · 无视频参考",
    "720p · no video reference": "720p · 无视频参考",
    "Catalog formula": "目录公式",
    "Video reference input": "视频参考输入",
    "Depends on resolution": "取决于分辨率",
    "Seedance 2.5 pricing varies by resolution, duration, and video-reference input; the catalog base is not a universal per-second rate.": "Seedance 2.5 价格取决于分辨率、时长和视频参考输入；目录基础值不是统一的每秒费率。",
    "Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.": "Seedance 2.5 是 ByteDance 的音视频生成模型，支持文生视频、图生视频、参考素材和可选音频控制。",
    "The catalog base is $0.14, but the request formula varies: 480p without video input is $0.140 × duration; 720p is $0.314 × duration; video-reference formulas use total video seconds and resolution.": "目录基础值为 $0.14，但请求公式会变化：无视频输入的 480p 为 $0.140 × 时长，720p 为 $0.314 × 时长；视频参考公式还取决于视频总秒数和分辨率。",
    "Use it for micro-drama and comic storyboards, product and UGC variants, film previsualization, game cinematics, creator clips, and market-research creative tests.": "可用于微短剧和漫画分镜、产品与 UGC 变体、电影预演、游戏过场、创作者短片及市场调研创意测试。",
    "POST to /v1/videos with the Seedance content[] format, retain the asynchronous task id, and fetch the result from /v1/videos/{task_id}/content.": "使用 Seedance content[] 格式向 /v1/videos 发起 POST，保留异步 task id，并从 /v1/videos/{task_id}/content 获取结果。",
    "Yes. Set 480p or 720p, 4–30 seconds, a supported ratio, generate_audio, and the documented reference/frame fields.": "可以。设置 480p 或 720p、4–30 秒、支持的比例、generate_audio，以及文档化的参考/帧字段。",
    "Flatkey authentication and the shared catalog use the gateway pattern, while Seedance video requests use content[] and the asynchronous /v1/videos contract.": "Flatkey 鉴权和共享目录采用网关模式；Seedance 视频请求使用 content[] 和异步 /v1/videos 契约。",
    "A request can include up to 30 images, 10 videos, and 10 audio references, with 50 references total; account rate limits and availability can change.": "一次请求最多可包含 30 张图片、10 个视频和 10 个音频参考素材，总计 50 个；账户限流和可用性可能变化。",
    "What is the Seedance 2.5 release date?": "Seedance 2.5 的发布日期是什么？",
    "The official ByteDance article was published on 2026-07-31; Flatkey's catalog lists released_at as 2026-08-04. These are different metadata fields, so neither date alone represents every launch.": "ByteDance 官方文章发表于 2026-07-31；Flatkey 目录将 released_at 记录为 2026-08-04。两者是不同的元数据字段，不能把任一日期单独理解为所有发布渠道的发布日期。",
    "Is Seedance 2.5 free?": "Seedance 2.5 免费吗？",
    "No free or unlimited entitlement is promised on this page. Use the live Flatkey pricing data and your account limits before running jobs.": "本页不承诺免费或无限额度；运行任务前请查看 Flatkey 实时价格和账户限制。",
    "Seedance 2.5 pricing: 480p, 720p, and video references": "Seedance 2.5 价格：480p、720p 与视频参考输入",
    "The catalog base is $0.14; the request formula depends on output resolution, duration, and video-reference input.": "目录基础值为 $0.14；实际请求公式取决于输出分辨率、时长和视频参考输入。",
    "Seedance 2.5 request pricing formulas": "Seedance 2.5 请求计价公式",
    "Scenario": "场景",
    "Flatkey formula": "Flatkey 公式",
    "Billing basis": "计费依据",
    "Total input-video seconds": "输入视频总秒数",
    "Output duration": "输出时长",
    "The catalog base and request formula are shown separately; final settlement follows the task estimate and account limits.": "目录基础值与请求公式会分开显示；最终结算以任务估算和账户限制为准。",
  },
};

// The Seedance 2.5 editorial block uses a few deliberately natural headings
// whose English source wording differs from the older Seedance translation
// keys. Keep those headings localized instead of letting the shared fallback
// return an English H2 on /pt, /es, or another translated route. For the
// remaining locales, aliases reuse the audited translations of the equivalent
// Seedance key so the page stays complete without duplicating the whole copy
// tree.
const seedance25SourceAliases: Record<string, string> = {
  "Seedance 2.5 AI video generator and API": "Seedance 2.5 AI Video Generator & API",
  "Seedance 2.5 AI video API performance and uptime": "Seedance 2.5 usage activity",
  "Seedance 2.5 video API usage activity": "Seedance 2.5 usage activity",
  "Seedance 2.5 AI video generator features for references and audio": "Seedance 2.5 features",
  "Seedance 2.5 features: 30-second video, references, and audio": "Seedance 2.5 features: references, audio, and 30-second video",
  "Seedance 2.5 prompt guide for text-to-video and image-to-video": "Seedance 2.5 prompt guide: six workflows",
  "Use Seedance 2.5 through a unified video API": "Why use Seedance 2.5 through Flatkey?",
  "Use the Seedance 2.5 API at /v1/videos": "Seedance 2.5 API: how to use /v1/videos",
  "Other Seedance and AI video generator APIs": "Other video generation models",
  "API, pricing, and release-date questions": "API–frequently asked questions",
};

const seedance25DirectTranslations: Partial<Record<Locale, Record<string, string>>> = {
  en: {
    "Seedance 2.5 AI video generator and API": "Seedance 2.5 AI video generator and API",
    "Seedance 2.5 AI video API performance and uptime": "Seedance 2.5 AI video API performance and availability",
    "Seedance 2.5 video API usage activity": "Seedance 2.5 video API usage activity",
    "Seedance 2.5 AI video generator features for references and audio": "Seedance 2.5 AI video generator features for references and audio",
    "Seedance 2.5 features: 30-second video, references, and audio": "Seedance 2.5 features: 30-second video, references, and audio",
    "Seedance 2.5 prompt guide for text-to-video and image-to-video": "Seedance 2.5 prompt guide for text-to-video and image-to-video",
    "Use Seedance 2.5 through a unified video API": "Use Seedance 2.5 through a unified video API",
    "Use the Seedance 2.5 API at /v1/videos": "Use the Seedance 2.5 API at /v1/videos",
    "Other Seedance and AI video generator APIs": "Other Seedance and AI video generator APIs",
    "API, pricing, and release-date questions": "API, pricing, and release-date questions",
  },
  pt: {
    "Seedance 2.5 AI video generator and API": "Gerador de vídeo IA Seedance 2.5 e API",
    "Seedance 2.5 AI video API performance and uptime": "Desempenho e disponibilidade da API de vídeo IA Seedance 2.5",
    "Seedance 2.5 video API usage activity": "Uso da API de vídeo Seedance 2.5 e atividade das solicitações",
    "Seedance 2.5 AI video generator features for references and audio": "Recursos do gerador de vídeo IA Seedance 2.5 para referências e áudio",
    "Seedance 2.5 features: 30-second video, references, and audio": "Recursos do Seedance 2.5: vídeo de 30 segundos, referências e áudio",
    "Seedance 2.5 prompt guide for text-to-video and image-to-video": "Guia de prompts do Seedance 2.5 para texto em vídeo e imagem em vídeo",
    "Use Seedance 2.5 through a unified video API": "Use o Seedance 2.5 por uma API de vídeo unificada",
    "Use the Seedance 2.5 API at /v1/videos": "Use a API do Seedance 2.5 em /v1/videos",
    "Other Seedance and AI video generator APIs": "Outras APIs do Seedance e de geração de vídeo com IA",
    "API, pricing, and release-date questions": "Perguntas sobre a API, preços e data de lançamento",
  },
};

// Keep the hero's API jump action translated on every localized model route.
const modelViewApiCopy: Record<Locale, string> = {
  en: "View API",
  zh: "查看 API",
  es: "Ver API",
  fr: "Voir l’API",
  pt: "Ver API",
  ru: "Открыть API",
  ja: "APIを見る",
  vi: "Xem API",
  de: "API anzeigen",
  id: "Lihat API",
};

function seedanceSourceCopy(locale: Locale, key: string): string | undefined {
  const direct = seedance25DirectTranslations[locale]?.[key];
  if (direct) return direct;
  const alias = seedance25SourceAliases[key];
  if (!alias) return undefined;
  return seedanceFactCopy[locale]?.[alias] ?? seedanceFactCopy.en?.[alias];
}

export function modelLandingCopy(locale: Locale, key: ModelLandingKey, vars: Record<string, string> = {}) {
  // Priority pages have a dedicated editorial copy pack.  Resolve those
  // strings only after the established Seedance/shared maps so a priority
  // phrase cannot change legacy pages that happen to reuse a short label
  // such as "Input" or "Context".
  // English is the source language for the editorial overrides.  Resolving
  // the generated priority map for `en` would translate a hand-written source
  // heading back into an older generated variant (for example, swapping the
  // natural H1 order to "API, pricing, and model details").  Non-English
  // locales still use the coherent priority map below.
  const priorityTranslations = locale === "en" ? {} : getPrioritySourceTranslations(locale);
  let value = (key === "View API" ? modelViewApiCopy[locale] : undefined) ?? seedanceFactCopy[locale]?.[key] ?? seedanceFactCopy.en?.[key] ?? seedanceSourceCopy(locale, key) ?? seedanceModelCopy[locale]?.[key] ?? modelDetailUiAdditions[locale]?.[key] ?? modelDetailUiCopy[locale]?.[key] ?? modelDetailPrototypeCopy[locale]?.[key] ?? modelDetailCommonCopy[locale]?.[key] ?? supplementalModelLandingCopy[locale]?.[key] ?? modelComparisonCopy[locale]?.[key] ?? priorityTranslations[key] ?? translations[locale][key] ?? translations.en[key] ?? key;
  for (const [name, replacement] of Object.entries(vars)) {
    value = value.replaceAll(`{{${name}}}`, replacement);
  }
  return value;
}
