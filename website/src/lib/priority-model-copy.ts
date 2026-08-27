/**
 * Locale-aware editorial copy for the five priority model pages.
 *
 * `model-landing.ts` owns the shared page shape and the English source copy.
 * This module deliberately stays independent so that the page config can be
 * wired to it without duplicating a second layout.  Technical literals (model
 * IDs, endpoint paths, parameter names, units and numeric rates) are kept
 * unchanged in every language; the surrounding explanation is translated.
 */

import type { Locale } from "./locales";
import { LOCALES } from "./locales";
import type { ModelLandingContent } from "./model-landing";

export const PRIORITY_MODEL_SLUGS = [
  "gpt-5-6-sol",
  "gpt-image-2",
  "kimi-k3",
  "deepseek-v4-pro",
  "minimax-h3",
] as const;

export type PriorityModelSlug = (typeof PRIORITY_MODEL_SLUGS)[number];

/** Canonical route/model aliases used by the live catalog and page config. */
const PRIORITY_SLUG_ALIASES: Record<string, PriorityModelSlug> = {
  "gpt-5.6-sol": "gpt-5-6-sol",
  "gpt-5-6-sol": "gpt-5-6-sol",
  "gpt-image-2": "gpt-image-2",
  "kimi-k3": "kimi-k3",
  "deepseek-v4-pro": "deepseek-v4-pro",
  "minimax-h3": "minimax-h3",
  "MiniMax-H3": "minimax-h3",
};

function normalizePrioritySlug(value: string): PriorityModelSlug | null {
  return PRIORITY_SLUG_ALIASES[value] ?? null;
}

export type PriorityModelSeo = {
  title: string;
  description: string;
};

export type PriorityModelLocalizedCopy = {
  seo: PriorityModelSeo;
  landingContent: ModelLandingContent;
};

type Facts = {
  slug: PriorityModelSlug;
  name: string;
  id: string;
  vendor: string;
  kind: "text" | "image" | "video";
  endpoint: string;
  secondEndpoint?: string;
  context?: string;
  modalities: string;
  rates: Array<{ label: string; value: string; detail: string }>;
};

const FACTS: Record<PriorityModelSlug, Facts> = {
  "gpt-5-6-sol": {
    slug: "gpt-5-6-sol",
    name: "GPT-5.6 Sol",
    id: "gpt-5.6-sol",
    vendor: "OpenAI",
    kind: "text",
    endpoint: "/v1/chat/completions",
    context: "1,048,576 tokens",
    modalities: "text, image, and file",
    rates: [
      { label: "Input", value: "$4.00", detail: "per 1M tokens" },
      { label: "Output", value: "$24.00", detail: "per 1M tokens" },
      { label: "Cache read", value: "$0.40", detail: "per 1M tokens" },
      { label: "Cache creation", value: "$5.00", detail: "per 1M tokens" },
    ],
  },
  "gpt-image-2": {
    slug: "gpt-image-2",
    name: "GPT Image 2",
    id: "gpt-image-2",
    vendor: "OpenAI",
    kind: "image",
    endpoint: "/v1/images/generations",
    modalities: "image generation",
    rates: [
      { label: "Input tokens", value: "$4.00", detail: "per 1M units" },
      { label: "Output tokens", value: "$24.00", detail: "per 1M units" },
      { label: "Cache tokens", value: "$1.00", detail: "per 1M units" },
      { label: "Image dimensions", value: "$6.40", detail: "per 1M units" },
    ],
  },
  "kimi-k3": {
    slug: "kimi-k3",
    name: "Kimi K3",
    id: "kimi-k3",
    vendor: "Moonshot AI",
    kind: "text",
    endpoint: "/v1/chat/completions",
    secondEndpoint: "/v1/messages",
    context: "1,048,576 tokens",
    modalities: "text and file",
    rates: [
      { label: "Input", value: "$2.40", detail: "per 1M tokens" },
      { label: "Output", value: "$12.00", detail: "per 1M tokens" },
      { label: "Cache", value: "$0.24", detail: "per 1M tokens" },
    ],
  },
  "deepseek-v4-pro": {
    slug: "deepseek-v4-pro",
    name: "DeepSeek V4 Pro",
    id: "deepseek-v4-pro",
    vendor: "DeepSeek",
    kind: "text",
    endpoint: "/v1/chat/completions",
    secondEndpoint: "/v1/messages",
    context: "1,048,576 tokens",
    modalities: "text and file",
    rates: [
      { label: "Peak (UTC)", value: "$1.32 / $0.044 / $3.96", detail: "input / cache read / output per 1M tokens" },
      { label: "Off-peak (UTC)", value: "$0.66 / $0.022 / $1.98", detail: "input / cache read / output per 1M tokens" },
    ],
  },
  "minimax-h3": {
    slug: "minimax-h3",
    name: "MiniMax-H3",
    id: "MiniMax-H3",
    vendor: "MiniMax",
    kind: "video",
    endpoint: "/v1/videos",
    modalities: "video generation",
    rates: [
      { label: "Catalog base", value: "$0.08", detail: "per second" },
      { label: "768P / 2K", value: "Live estimate", detail: "resolution-dependent" },
      { label: "Reference video", value: "Live estimate", detail: "input-video seconds and resolution" },
      { label: "Input image", value: "Check catalog", detail: "free allowance and later images may differ" },
    ],
  },
};

type LanguagePack = {
  apiPricingDetails: string;
  apiPricingAndModel: string;
  imageApiPricing: string;
  videoApiPricing: string;
  pricing: string;
  pricingByToken: string;
  pricingByTokenAndDimensions: string;
  pricingByUtc: string;
  pricingCurrent: string;
  pricingNoteToken: string;
  pricingNoteImage: string;
  pricingNoteUtc: string;
  pricingNoteVideo: string;
  capabilities: string;
  contextModalitiesEndpoint: string;
  imageControls: string;
  fileEndpoints: string;
  videoControls: string;
  documentedFields: string;
  compareFields: string;
  hostedApiFacts: string;
  migrationFields: string;
  videoFields: string;
  api: string;
  callWithApi: string;
  imageApiSetup: string;
  textApiSetup: string;
  videoApiSetup: string;
  faqDescription: string;
  unknown: string;
  notAsserted: string;
  notVerified: string;
  perMillionTokens: string;
  perMillionUnits: string;
  perSecond: string;
  input: string;
  output: string;
  cacheRead: string;
  cacheCreation: string;
  cache: string;
  inputTokens: string;
  outputTokens: string;
  cacheTokens: string;
  imageDimensions: string;
  peakUtc: string;
  offPeakUtc: string;
  endpoint: string;
  modelId: string;
  modalities: string;
  context: string;
  formats: string;
  sizes: string;
  quality: string;
  controls: string;
  access: string;
  boundary: string;
  hostedEndpoint: string;
  inputModality: string;
  downloadableWeights: string;
  localRuntime: string;
  freeAccess: string;
  resolution: string;
  duration: string;
  ratio: string;
  watermark: string;
  fields: string;
  result: string;
  whatIs: (name: string) => string;
  howUse: (name: string) => string;
  apiModelId: (name: string) => string;
  cost: (name: string) => string;
  contextInputs: (name: string) => string;
  release: (name: string) => string;
  accessQuestion: (name: string) => string;
  endpointQuestion: (name: string) => string;
  sizeFormatQuestion: (name: string) => string;
  transparentQuestion: string;
  freeSignupQuestion: (name: string) => string;
  freeQuestion: (name: string) => string;
  localQuestion: (name: string) => string;
  vendorQuestion: (name: string) => string;
  priceUtcQuestion: (name: string) => string;
  visionQuestion: (name: string) => string;
  qualityQuestion: (name: string) => string;
  settingsQuestion: (name: string) => string;
  configureQuestion: (name: string) => string;
  generationQuestion: string;
  modelStatement: (name: string, vendor: string, endpoint: string, facts: string) => string;
  imageStatement: (name: string, endpoint: string) => string;
  kimiStatement: (name: string, endpoint: string, secondEndpoint: string, context: string) => string;
  deepseekStatement: (name: string, endpoint: string, secondEndpoint: string, context: string) => string;
  minimaxStatement: (name: string, endpoint: string) => string;
  contextBody: (context: string) => string;
  modalitiesBody: (modalities: string) => string;
  endpointBody: (endpoint: string, id: string) => string;
  keyBillingBody: string;
  imageCountBody: string;
  imageSizesBody: string;
  imageQualityBody: string;
  imageFormatsBody: string;
  imageBackgroundBody: string;
  twoRoutesBody: (endpoint: string, secondEndpoint: string) => string;
  fileBody: string;
  knowledgeBody: string;
  routeBody: (endpoint: string) => string;
  distillableBody: string;
  resolutionBody: string;
  durationBody: string;
  ratioBody: string;
  watermarkBody: string;
  apiEndpointDetail: (endpoint: string) => string;
  apiModelDetail: (id: string) => string;
  apiModalitiesDetail: (modalities: string) => string;
  apiCompatibilityDetail: string;
  apiImageControlsDetail: string;
  apiAccessDetail: string;
  apiBoundaryDetail: string;
  apiResultDetail: string;
  costAnswer: (name: string, rates: string) => string;
  contextAnswer: (context: string, modalities: string) => string;
  releaseAnswer: (date: string) => string;
  imageAccessAnswer: string;
  imageEndpointAnswer: (id: string, endpoint: string) => string;
  imageCostAnswer: (rates: string) => string;
  imageSizeAnswer: string;
  transparentAnswer: string;
  freeSignupAnswer: string;
  kimiUseAnswer: (id: string, endpoint: string, secondEndpoint: string) => string;
  kimiIdAnswer: (id: string, endpoint: string, secondEndpoint: string) => string;
  kimiCostAnswer: (rates: string) => string;
  kimiFreeAnswer: string;
  kimiLocalAnswer: string;
  kimiVendorAnswer: string;
  deepseekUseAnswer: (id: string, endpoint: string, secondEndpoint: string) => string;
  deepseekIdAnswer: (id: string, endpoint: string, secondEndpoint: string) => string;
  deepseekCostAnswer: string;
  deepseekLocalAnswer: string;
  deepseekVisionAnswer: string;
  deepseekQualityAnswer: string;
  minimaxWhatAnswer: (endpoint: string) => string;
  minimaxUseAnswer: string;
  minimaxCostAnswer: string;
  minimaxFieldsAnswer: string;
  minimaxGenerationAnswer: string;
  minimaxLocalAnswer: string;
};

/*
 * The language packs keep sentence grammar in one place while the model facts
 * below remain literal.  This makes it possible to audit every locale without
 * maintaining five mostly-identical object trees by hand.
 */
const PACKS: Record<Locale, LanguagePack> = {
  en: {
    apiPricingDetails: "API, pricing, and model details",
    apiPricingAndModel: "API and pricing",
    imageApiPricing: "API and image generation",
    videoApiPricing: "video generator API and pricing",
    pricing: "pricing",
    pricingByToken: "pricing by token dimension",
    pricingByTokenAndDimensions: "pricing by token and image dimensions",
    pricingByUtc: "pricing by UTC time tier",
    pricingCurrent: "Current Flatkey catalog rates",
    pricingNoteToken: "Rates are per 1M tokens; check the dated catalog block for current values.",
    pricingNoteImage: "The catalog exposes token-dimension pricing; the final amount depends on request and usage fields.",
    pricingNoteUtc: "Keep peak and off-peak token dimensions together; no blended always-active price is implied.",
    pricingNoteVideo: "Current catalog rows are per-second values; review reference-video and input-image rows separately.",
    capabilities: "capabilities",
    contextModalitiesEndpoint: "context, modalities, and endpoint",
    imageControls: "sizes, quality, formats, and controls",
    fileEndpoints: "context, file input, and compatible endpoints",
    videoControls: "resolution, duration, ratio, and watermark",
    documentedFields: "Documented integration fields are shown below; no benchmark or quality promise is inferred.",
    compareFields: "Compare documented fields",
    hostedApiFacts: "Hosted API facts",
    migrationFields: "Migration fields",
    videoFields: "Video controls",
    api: "API",
    callWithApi: "API access for",
    imageApiSetup: "API endpoint and request setup",
    textApiSetup: "API and model ID",
    videoApiSetup: "video API setup",
    faqDescription: "Pricing, compatibility, limits, and model-specific request details.",
    unknown: "Unknown",
    notAsserted: "Not asserted",
    notVerified: "Unknown / not verified",
    perMillionTokens: "per 1M tokens",
    perMillionUnits: "per 1M units",
    perSecond: "per second",
    input: "Input",
    output: "Output",
    cacheRead: "Cache read",
    cacheCreation: "Cache creation",
    cache: "Cache",
    inputTokens: "Input tokens",
    outputTokens: "Output tokens",
    cacheTokens: "Cache tokens",
    imageDimensions: "Image dimensions",
    peakUtc: "Peak (UTC)",
    offPeakUtc: "Off-peak (UTC)",
    endpoint: "Endpoint",
    modelId: "Model ID",
    modalities: "Modalities",
    context: "Context",
    formats: "Formats",
    sizes: "Sizes",
    quality: "Quality",
    controls: "Controls",
    access: "Access",
    boundary: "Boundary",
    hostedEndpoint: "Hosted endpoint",
    inputModality: "Input modality",
    downloadableWeights: "Downloadable weights",
    localRuntime: "Local runtime or hardware",
    freeAccess: "Free access",
    resolution: "Resolution",
    duration: "Duration",
    ratio: "Ratio",
    watermark: "AIGC watermark",
    fields: "Fields",
    result: "Result",
    whatIs: (name) => `What is ${name}?`,
    howUse: (name) => `How do I use ${name}?`,
    apiModelId: (name) => `What is the ${name} API model ID?`,
    cost: (name) => `How much does ${name} cost?`,
    contextInputs: (name) => `What context window and inputs does ${name} support?`,
    release: (name) => `When was ${name} released?`,
    accessQuestion: (name) => `How do I access ${name}?`,
    endpointQuestion: (name) => `What is the ${name} API endpoint?`,
    sizeFormatQuestion: (name) => `What sizes and formats are supported by ${name}?`,
    transparentQuestion: "Can GPT Image 2 create transparent backgrounds?",
    freeSignupQuestion: (name) => `Is ${name} free or available without signup?`,
    freeQuestion: (name) => `Is ${name} free?`,
    localQuestion: (name) => `Is ${name} open source or available locally?`,
    vendorQuestion: (name) => `Who makes ${name}?`,
    priceUtcQuestion: (name) => `How is ${name} priced?`,
    visionQuestion: (name) => `Does ${name} support vision or multimodal input?`,
    qualityQuestion: (name) => `Is ${name} better for coding than V4 Flash?`,
    settingsQuestion: (name) => `Which ${name} settings can I configure here?`,
    configureQuestion: (name) => `How do I configure a ${name} request?`,
    generationQuestion: "Does this start a real generation?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} is an ${vendor} catalog model available through Flatkey's ${endpoint} endpoint. The verified catalog records ${facts}. This page keeps the model ID, current rates, and request path visible before production traffic.`,
    imageStatement: (name, endpoint) => `${name} is the OpenAI image model exposed in Flatkey's image-generation flow. Use ${endpoint} and configure count, size, quality, format, background, and moderation fields before sending a request.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} is the Moonshot AI catalog model available through Flatkey's compatible chat and messages routes. The verified catalog records a ${context.replace(/\s+tokens?$/i, "-token")} context and file input; documented paths are ${endpoint} and ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} is the DeepSeek catalog model documented with text and file modalities, a ${context.replace(/\s+tokens?$/i, "-token")} context, and two compatible API paths: ${endpoint} and ${secondEndpoint}. Pricing is time-tiered in UTC.`,
    minimaxStatement: (name, endpoint) => `${name} is available through Flatkey's asynchronous ${endpoint} flow. Configure 768P or 2K resolution, 4–15 seconds, a supported ratio, and the AIGC watermark field before opening the console.`,
    contextBody: (context) => `The catalog records a ${context.replace(/\s+tokens?$/i, "-token")} context window.`,
    modalitiesBody: (modalities) => `Catalog metadata lists ${modalities}.`,
    endpointBody: (endpoint, id) => `Requests use ${endpoint} with model ID ${id}.`,
    keyBillingBody: "Flatkey supplies the API-key and billing layer with endpoint/request compatibility.",
    imageCountBody: "n accepts 1–10 in the page generator.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536, or auto.",
    imageQualityBody: "auto, high, medium, or low.",
    imageFormatsBody: "PNG, JPEG, or WebP.",
    imageBackgroundBody: "background is opaque or auto; moderation is auto or low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} and ${secondEndpoint} are the recorded compatible endpoint paths.`,
    fileBody: "File modality is listed in the catalog; do not extrapolate additional media types.",
    knowledgeBody: "Coding, document, and research examples are editorial use cases, not benchmark claims.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "A distillable metadata flag is not a claim that the model is open source, downloadable, or locally runnable.",
    resolutionBody: "Choose 768P or 2K.",
    durationBody: "Configure a 4–15 second request.",
    ratioBody: "Use a supported fixed ratio or adaptive.",
    watermarkBody: "Set the AIGC watermark boolean explicitly.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Catalog metadata lists ${modalities}.`,
    apiCompatibilityDetail: "Use compatible request fields; do not infer unsupported consumer features.",
    apiImageControlsDetail: "n, size, quality, format, background, and moderation.",
    apiAccessDetail: "Use the normal Flatkey account and API-key flow; no free or no-signup promise is made.",
    apiBoundaryDetail: "Compatible routes do not prove local or open-source support.",
    apiResultDetail: "Use the returned task ID with the content endpoint.",
    costAnswer: (name, rates) => `Current catalog rates for ${name} are ${rates}. Check the dated pricing block before running production traffic.`,
    contextAnswer: (context, modalities) => `The catalog records ${context} and ${modalities}. This page does not infer additional modalities.`,
    releaseAnswer: (date) => `The catalog snapshot records ${date} as the release date; treat this as catalog metadata rather than a separate launch announcement.`,
    imageAccessAnswer: "Configure a request, then use the normal Flatkey account and API-key flow. The page does not promise no-signup generation.",
    imageEndpointAnswer: (id, endpoint) => `Use ${endpoint} with model ID ${id} and the supported fields listed in the API section.`,
    imageCostAnswer: (rates) => `The catalog lists ${rates} per 1M units; it is not a single fixed per-image price.`,
    imageSizeAnswer: "Sizes are 1024x1024, 1536x1024, 1024x1536, and auto. Formats are PNG, JPEG, and WebP; quality is auto/high/medium/low.",
    transparentAnswer: "The verified field is background: opaque | auto; this page does not promise transparent output.",
    freeSignupAnswer: "Free or no-signup generation is not verified. Use the normal Flatkey access flow and check the dated pricing block.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Create a Flatkey API key, choose ${endpoint} or ${secondEndpoint} as documented by your client, and set model to ${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `The verified model ID is ${id}; the documented compatible routes are ${endpoint} and ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `Current catalog rates are ${rates} per 1M tokens.`,
    kimiFreeAnswer: "The current catalog shows paid token rates. Do not promise free access; check the account and pricing UI for current availability.",
    kimiLocalAnswer: "The verified catalog does not confirm downloadable weights, local deployment, or hardware/VRAM requirements. Do not infer those properties from metadata.",
    kimiVendorAnswer: "The catalog vendor is Moonshot AI; Flatkey provides routing and billing.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Send a request to ${endpoint} or ${secondEndpoint}, use model ID ${id}, and follow the request shape for the selected compatible client.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `The verified model ID is ${id}; the documented paths are ${endpoint} and ${secondEndpoint}.`,
    deepseekCostAnswer: "Pricing is time-tiered in UTC: peak input/cache-read/output are $1.32/$0.044/$3.96 and off-peak are $0.66/$0.022/$1.98 per 1M tokens.",
    deepseekLocalAnswer: "The current catalog does not verify downloadable weights or local deployment. Do not infer those claims from a distillable metadata flag.",
    deepseekVisionAnswer: "Only text and file modalities are verified in this fact set. Do not claim vision or broader multimodal support until an authoritative model document is added.",
    deepseekQualityAnswer: "This page does not publish a benchmark or quality ranking. Compare documented model IDs, endpoints, context, modalities, and current price fields instead.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 is a video model available through Flatkey's ${endpoint} flow.`,
    minimaxUseAnswer: "Save the task ID returned by the asynchronous request and retrieve the result when ready.",
    minimaxCostAnswer: "The current public catalog lists a $0.08 per-second base. Resolution, reference-video seconds, and input-image allowance are shown separately and should be checked in the live estimate.",
    minimaxFieldsAnswer: "Configure resolution, duration, ratio, and AIGC watermark before opening the console.",
    minimaxGenerationAnswer: "The public page saves your draft settings first. Sign up or open the console to run the request with an API key.",
    minimaxLocalAnswer: "ComfyUI, local weights, and other deployment claims are not verified by this catalog record; do not infer them from search snippets.",
  },
  zh: {
    apiPricingDetails: "API、价格与模型详情",
    apiPricingAndModel: "API 与价格",
    imageApiPricing: "API 与图像生成",
    videoApiPricing: "视频生成器 API 与价格",
    pricing: "价格",
    pricingByToken: "按 token 维度计价",
    pricingByTokenAndDimensions: "按 token 与图像尺寸计价",
    pricingByUtc: "按 UTC 时段计价",
    pricingCurrent: "Flatkey 当前目录价格",
    pricingNoteToken: "费率按每 100 万 token 展示；当前值请以带日期的目录区块为准。",
    pricingNoteImage: "目录提供 token 维度价格；最终金额取决于请求与用量字段。",
    pricingNoteUtc: "请同时查看高峰与非高峰 token 维度；这里不暗示一个始终有效的混合价格。",
    pricingNoteVideo: "当前目录行按秒计价；参考视频和输入图片请分别查看。",
    capabilities: "能力",
    contextModalitiesEndpoint: "上下文、模态与 endpoint",
    imageControls: "尺寸、质量、格式与控制项",
    fileEndpoints: "上下文、文件输入与兼容 endpoint",
    videoControls: "分辨率、时长、比例与水印",
    documentedFields: "下方只展示已记录的集成字段，不推断基准测试或质量承诺。",
    compareFields: "已记录字段对比",
    hostedApiFacts: "托管 API 事实",
    migrationFields: "迁移字段",
    videoFields: "视频控制项",
    api: "API",
    callWithApi: "API 调用：",
    imageApiSetup: "API endpoint 与请求设置",
    textApiSetup: "API 与模型 ID",
    videoApiSetup: "视频 API 设置",
    faqDescription: "价格、兼容性、限制与模型专属请求细节。",
    unknown: "未知",
    notAsserted: "未声明",
    notVerified: "未知 / 未核实",
    perMillionTokens: "每 100 万 token",
    perMillionUnits: "每 100 万单位",
    perSecond: "每秒",
    input: "输入",
    output: "输出",
    cacheRead: "缓存读取",
    cacheCreation: "缓存创建",
    cache: "缓存",
    inputTokens: "输入 token",
    outputTokens: "输出 token",
    cacheTokens: "缓存 token",
    imageDimensions: "图像尺寸",
    peakUtc: "高峰（UTC）",
    offPeakUtc: "非高峰（UTC）",
    endpoint: "Endpoint",
    modelId: "模型 ID",
    modalities: "模态",
    context: "上下文",
    formats: "格式",
    sizes: "尺寸",
    quality: "质量",
    controls: "控制项",
    access: "访问方式",
    boundary: "边界",
    hostedEndpoint: "托管 endpoint",
    inputModality: "输入模态",
    downloadableWeights: "可下载权重",
    localRuntime: "本地运行时或硬件",
    freeAccess: "免费访问",
    resolution: "分辨率",
    duration: "时长",
    ratio: "比例",
    watermark: "AIGC 水印",
    fields: "字段",
    result: "结果",
    whatIs: (name) => `什么是 ${name}？`,
    howUse: (name) => `如何使用 ${name}？`,
    apiModelId: (name) => `${name} 的 API 模型 ID 是什么？`,
    cost: (name) => `${name} 的价格是多少？`,
    contextInputs: (name) => `${name} 支持什么上下文窗口和输入？`,
    release: (name) => `${name} 何时发布？`,
    accessQuestion: (name) => `如何访问 ${name}？`,
    endpointQuestion: (name) => `${name} 的 API endpoint 是什么？`,
    sizeFormatQuestion: (name) => `${name} 支持哪些尺寸和格式？`,
    transparentQuestion: "GPT Image 2 能生成透明背景吗？",
    freeSignupQuestion: (name) => `${name} 免费或无需注册就能使用吗？`,
    freeQuestion: (name) => `${name} 免费吗？`,
    localQuestion: (name) => `${name} 开源或可本地运行吗？`,
    vendorQuestion: (name) => `${name} 由谁提供？`,
    priceUtcQuestion: (name) => `${name} 如何计价？`,
    visionQuestion: (name) => `${name} 支持视觉或多模态输入吗？`,
    qualityQuestion: (name) => `${name} 比 V4 Flash 更适合编程吗？`,
    settingsQuestion: (name) => `这里可以配置哪些 ${name} 设置？`,
    configureQuestion: (name) => `如何配置 ${name} 请求？`,
    generationQuestion: "这里会立即开始真实生成吗？",
    modelStatement: (name, vendor, endpoint, facts) => `${name} 是 ${vendor} 目录中的模型，可通过 Flatkey 的 ${endpoint} endpoint 使用。已核实的目录字段包括：${facts}。本页会在发送生产流量前展示准确的模型 ID、当前费率和请求路径。`,
    imageStatement: (name, endpoint) => `${name} 是 OpenAI 的图像模型，通过 Flatkey 的图像生成流程提供。使用 ${endpoint}，发送前配置数量、尺寸、质量、格式、背景和 moderation 字段。`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} 是 Moonshot AI 目录模型，可通过 Flatkey 的兼容 chat 和 messages 路由使用。已核实目录记录 ${context} 上下文和文件输入；文档化路径为 ${endpoint} 与 ${secondEndpoint}。`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} 是 DeepSeek 目录模型，已记录文本和文件模态、${context} 上下文，以及两个兼容 API 路径：${endpoint} 和 ${secondEndpoint}。价格按 UTC 时段变化。`,
    minimaxStatement: (name, endpoint) => `${name} 可通过 Flatkey 异步 ${endpoint} 流程使用。打开控制台前可配置 768P 或 2K 分辨率、4–15 秒、支持的比例和 AIGC 水印字段。`,
    contextBody: (context) => `目录记录的上下文窗口为 ${context}。`,
    modalitiesBody: (modalities) => `目录元数据列出 ${modalities}。`,
    endpointBody: (endpoint, id) => `请求使用 ${endpoint}，模型 ID 为 ${id}。`,
    keyBillingBody: "Flatkey 提供 API Key 与计费层，并保持 endpoint/请求兼容性。",
    imageCountBody: "页面生成器中的 n 支持 1–10。",
    imageSizesBody: "1024x1024、1536x1024、1024x1536 或 auto。",
    imageQualityBody: "auto、high、medium 或 low。",
    imageFormatsBody: "PNG、JPEG 或 WebP。",
    imageBackgroundBody: "background 字段接受 opaque 或 auto；moderation 字段接受 auto 或 low。",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} 与 ${secondEndpoint} 是已记录的兼容 endpoint。`,
    fileBody: "目录列出 file 模态；不要外推其他媒体类型。",
    knowledgeBody: "编程、文档和研究示例是编辑性使用场景，不是基准测试结论。",
    routeBody: (endpoint) => `${endpoint}。`,
    distillableBody: "distillable 元数据标记不等于开源、可下载或可本地运行。",
    resolutionBody: "选择 768P 或 2K。",
    durationBody: "配置 4–15 秒的请求。",
    ratioBody: "使用支持的固定比例或 adaptive。",
    watermarkBody: "明确设置 AIGC 水印布尔值。",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `目录元数据列出 ${modalities}。`,
    apiCompatibilityDetail: "使用兼容请求字段；不要推断未核实的消费者功能。",
    apiImageControlsDetail: "n、size、quality、format、background 和 moderation。",
    apiAccessDetail: "使用标准 Flatkey 账号和 API Key 流程；不承诺免费或免注册生成。",
    apiBoundaryDetail: "兼容路由不证明本地或开源支持。",
    apiResultDetail: "使用返回的 task ID 调用 content endpoint。",
    costAnswer: (name, rates) => `${name} 当前目录费率为 ${rates}。生产流量前请查看带日期的价格区块。`,
    contextAnswer: (context, modalities) => `目录记录 ${context} 和 ${modalities}。本页不推断额外模态。`,
    releaseAnswer: (date) => `目录快照记录发布日期为 ${date}；这是目录元数据，不等同于独立的发布公告。`,
    imageAccessAnswer: "先配置请求，再使用标准 Flatkey 账号和 API Key 流程。本页不承诺免注册生成。",
    imageEndpointAnswer: (id, endpoint) => `使用 ${endpoint}、模型 ID ${id} 及 API 区块列出的支持字段。`,
    imageCostAnswer: (rates) => `目录列出的费率为每 100 万单位 ${rates}；不是统一的单张图片价格。`,
    imageSizeAnswer: "尺寸为 1024x1024、1536x1024、1024x1536 和 auto；格式为 PNG、JPEG、WebP，质量为 auto/high/medium/low。",
    transparentAnswer: "已核实字段为 background: opaque | auto；本页不承诺透明输出。",
    freeSignupAnswer: "研究未核实免费或免注册生成。请使用标准 Flatkey 访问流程并查看带日期的价格区块。",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `创建 Flatkey API Key，根据客户端选择 ${endpoint} 或 ${secondEndpoint}，并将 model 设为 ${id}。`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `已核实模型 ID 为 ${id}；文档化兼容路由为 ${endpoint} 和 ${secondEndpoint}。`,
    kimiCostAnswer: (rates) => `当前目录费率为每 100 万 token ${rates}。`,
    kimiFreeAnswer: "当前目录显示按 token 付费；不要承诺免费访问，请以账号和价格界面为准。",
    kimiLocalAnswer: "已核实目录没有确认可下载权重、本地部署或硬件/VRAM 要求；不要从元数据推断这些属性。",
    kimiVendorAnswer: "目录供应商为 Moonshot AI；Flatkey 提供路由与计费。",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `向 ${endpoint} 或 ${secondEndpoint} 发送请求，使用模型 ID ${id}，并遵循所选兼容客户端的请求格式。`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `已核实模型 ID 为 ${id}；文档化路径为 ${endpoint} 和 ${secondEndpoint}。`,
    deepseekCostAnswer: "价格按 UTC 时段计费：高峰输入/缓存读取/输出为 $1.32/$0.044/$3.96，非高峰为 $0.66/$0.022/$1.98，均按每 100 万 token。",
    deepseekLocalAnswer: "当前目录没有核实可下载权重或本地部署；不要从 distillable 元数据标记推断这些结论。",
    deepseekVisionAnswer: "本事实集仅核实文本和文件模态；在加入权威模型文档前，不要声称支持视觉或更广泛的多模态输入。",
    deepseekQualityAnswer: "本页不发布基准或质量排名；请比较已记录的模型 ID、endpoint、上下文、模态和当前价格字段。",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 是可通过 Flatkey ${endpoint} 流程使用的视频模型。`,
    minimaxUseAnswer: "保存异步请求返回的 task ID，准备好后再获取结果。",
    minimaxCostAnswer: "当前公开目录列出每秒 $0.08 的基础值。分辨率、参考视频秒数和输入图片额度会单独展示，请以实时估算为准。",
    minimaxFieldsAnswer: "打开控制台前配置分辨率、时长、比例和 AIGC 水印。",
    minimaxGenerationAnswer: "公开页先保存你的草稿设置；注册或打开控制台后，使用 API Key 执行请求。",
    minimaxLocalAnswer: "目录记录未核实 ComfyUI、本地权重或其他部署结论；不要从搜索摘要推断。",
  },
  es: {
    apiPricingDetails: "API, precios y detalles del modelo",
    apiPricingAndModel: "API y precios",
    imageApiPricing: "API y generación de imágenes",
    videoApiPricing: "API y precios del generador de vídeo",
    pricing: "precios",
    pricingByToken: "precios por dimensión de token",
    pricingByTokenAndDimensions: "precios por token y dimensiones de imagen",
    pricingByUtc: "precios por franja horaria UTC",
    pricingCurrent: "Tarifas actuales del catálogo de Flatkey",
    pricingNoteToken: "Las tarifas son por 1 M de tokens; consulta el bloque fechado del catálogo para conocer los valores actuales.",
    pricingNoteImage: "El catálogo expone precios por dimensión de token; el importe final depende de los campos de solicitud y uso.",
    pricingNoteUtc: "Mantén juntas las dimensiones de tokens de hora punta y fuera de punta; no se indica un precio combinado permanente.",
    pricingNoteVideo: "Las filas actuales del catálogo son por segundo; revisa por separado el vídeo de referencia y la imagen de entrada.",
    capabilities: "capacidades",
    contextModalitiesEndpoint: "contexto, modalidades y endpoint",
    imageControls: "tamaños, calidad, formatos y controles",
    fileEndpoints: "contexto, entrada de archivos y endpoints compatibles",
    videoControls: "resolución, duración, proporción y marca de agua",
    documentedFields: "A continuación se muestran campos de integración documentados; no se infiere ninguna promesa de benchmark o calidad.",
    compareFields: "Comparar campos documentados",
    hostedApiFacts: "Datos de la API alojada",
    migrationFields: "Campos de migración",
    videoFields: "Controles de vídeo",
    api: "API",
    callWithApi: "API para",
    imageApiSetup: "endpoint de API y configuración de la solicitud",
    textApiSetup: "API e ID del modelo",
    videoApiSetup: "configuración de la API de vídeo",
    faqDescription: "Precios, compatibilidad, límites y detalles de solicitud específicos del modelo.",
    unknown: "Desconocido",
    notAsserted: "No afirmado",
    notVerified: "Desconocido / no verificado",
    perMillionTokens: "por 1 M de tokens",
    perMillionUnits: "por 1 M de unidades",
    perSecond: "por segundo",
    input: "Entrada",
    output: "Salida",
    cacheRead: "Lectura de caché",
    cacheCreation: "Creación de caché",
    cache: "Caché",
    inputTokens: "Tokens de entrada",
    outputTokens: "Tokens de salida",
    cacheTokens: "Tokens de caché",
    imageDimensions: "Dimensiones de imagen",
    peakUtc: "Punta (UTC)",
    offPeakUtc: "Fuera de punta (UTC)",
    endpoint: "Endpoint",
    modelId: "ID del modelo",
    modalities: "Modalidades",
    context: "Contexto",
    formats: "Formatos",
    sizes: "Tamaños",
    quality: "Calidad",
    controls: "Controles",
    access: "Acceso",
    boundary: "Límite",
    hostedEndpoint: "Endpoint alojado",
    inputModality: "Modalidad de entrada",
    downloadableWeights: "Pesos descargables",
    localRuntime: "Runtime local o hardware",
    freeAccess: "Acceso gratuito",
    resolution: "Resolución",
    duration: "Duración",
    ratio: "Proporción",
    watermark: "Marca de agua AIGC",
    fields: "Campos",
    result: "Resultado",
    whatIs: (name) => `¿Qué es ${name}?`,
    howUse: (name) => `¿Cómo uso ${name}?`,
    apiModelId: (name) => `¿Cuál es el ID de modelo de API de ${name}?`,
    cost: (name) => `¿Cuánto cuesta ${name}?`,
    contextInputs: (name) => `¿Qué ventana de contexto y entradas admite ${name}?`,
    release: (name) => `¿Cuándo se lanzó ${name}?`,
    accessQuestion: (name) => `¿Cómo accedo a ${name}?`,
    endpointQuestion: (name) => `¿Cuál es el endpoint de API de ${name}?`,
    sizeFormatQuestion: (name) => `¿Qué tamaños y formatos admite ${name}?`,
    transparentQuestion: "¿GPT Image 2 puede crear fondos transparentes?",
    freeSignupQuestion: (name) => `¿${name} es gratis o está disponible sin registrarse?`,
    freeQuestion: (name) => `¿${name} es gratis?`,
    localQuestion: (name) => `¿${name} es de código abierto o está disponible localmente?`,
    vendorQuestion: (name) => `¿Quién crea ${name}?`,
    priceUtcQuestion: (name) => `¿Cómo se calcula el precio de ${name}?`,
    visionQuestion: (name) => `¿${name} admite entrada visual o multimodal?`,
    qualityQuestion: (name) => `¿${name} es mejor para programar que V4 Flash?`,
    settingsQuestion: (name) => `¿Qué ajustes de ${name} puedo configurar aquí?`,
    configureQuestion: (name) => `¿Cómo configuro una solicitud de ${name}?`,
    generationQuestion: "¿Esto inicia una generación real?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} es un modelo del catálogo de ${vendor} disponible mediante el endpoint ${endpoint} de Flatkey. El catálogo verificado registra ${facts}. Esta página muestra el ID exacto, las tarifas actuales y la ruta de solicitud antes del tráfico de producción.`,
    imageStatement: (name, endpoint) => `${name} es el modelo de imágenes de OpenAI expuesto en el flujo de generación de imágenes de Flatkey. Usa ${endpoint} y configura cantidad, tamaño, calidad, formato, fondo y moderación antes de enviar la solicitud.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} es el modelo del catálogo de Moonshot AI disponible mediante las rutas compatibles de chat y mensajes de Flatkey. El catálogo verificado registra un contexto de ${context} y entrada de archivos; las rutas documentadas son ${endpoint} y ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} es el modelo del catálogo de DeepSeek documentado con modalidades de texto y archivo, un contexto de ${context} y dos rutas de API compatibles: ${endpoint} y ${secondEndpoint}. El precio varía por franja UTC.`,
    minimaxStatement: (name, endpoint) => `${name} está disponible mediante el flujo asíncrono ${endpoint} de Flatkey. Configura resolución 768P o 2K, 4–15 segundos, una proporción compatible y el campo de marca de agua AIGC antes de abrir la consola.`,
    contextBody: (context) => `El catálogo registra una ventana de contexto de ${context}.`,
    modalitiesBody: (modalities) => `Los metadatos del catálogo enumeran ${modalities}.`,
    endpointBody: (endpoint, id) => `Las solicitudes usan ${endpoint} con el ID de modelo ${id}.`,
    keyBillingBody: "Flatkey proporciona la capa de API key y facturación con compatibilidad de endpoint y solicitud.",
    imageCountBody: "n acepta 1–10 en el generador de la página.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536 o auto.",
    imageQualityBody: "auto, high, medium o low.",
    imageFormatsBody: "PNG, JPEG o WebP.",
    imageBackgroundBody: "El campo background admite opaque o auto; moderation admite auto o low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} y ${secondEndpoint} son las rutas de endpoint compatibles registradas.`,
    fileBody: "La modalidad de archivo figura en el catálogo; no extrapoles otros tipos de medios.",
    knowledgeBody: "Los ejemplos de código, documentos e investigación son casos editoriales, no resultados de benchmarks.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "Una marca de metadatos distillable no demuestra que el modelo sea de código abierto, descargable o ejecutable localmente.",
    resolutionBody: "Elige 768P o 2K.",
    durationBody: "Configura una solicitud de 4–15 segundos.",
    ratioBody: "Usa una proporción fija compatible o adaptive.",
    watermarkBody: "Configura explícitamente el booleano de marca de agua AIGC.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Los metadatos del catálogo enumeran ${modalities}.`,
    apiCompatibilityDetail: "Usa los campos de solicitud compatibles; no infieras funciones de consumidor no verificadas.",
    apiImageControlsDetail: "n, size, quality, format, background y moderation.",
    apiAccessDetail: "Usa el flujo normal de cuenta y API key de Flatkey; no se promete generación gratuita o sin registro.",
    apiBoundaryDetail: "Las rutas compatibles no demuestran soporte local o de código abierto.",
    apiResultDetail: "Usa el task ID devuelto con el endpoint de contenido.",
    costAnswer: (name, rates) => `Las tarifas actuales del catálogo para ${name} son ${rates}. Consulta el bloque de precios fechado antes del tráfico de producción.`,
    contextAnswer: (context, modalities) => `El catálogo registra ${context} y ${modalities}. Esta página no infiere modalidades adicionales.`,
    releaseAnswer: (date) => `La instantánea del catálogo registra ${date} como fecha de lanzamiento; trátala como metadato del catálogo, no como un anuncio independiente.`,
    imageAccessAnswer: "Configura una solicitud y usa el flujo normal de cuenta y API key de Flatkey. La página no promete generación sin registro.",
    imageEndpointAnswer: (id, endpoint) => `Usa ${endpoint} con el ID ${id} y los campos compatibles indicados en la sección API.`,
    imageCostAnswer: (rates) => `El catálogo indica ${rates} por 1 M de unidades; no es un precio fijo por imagen.`,
    imageSizeAnswer: "Los tamaños son 1024x1024, 1536x1024, 1024x1536 y auto. Los formatos son PNG, JPEG y WebP; la calidad es auto/high/medium/low.",
    transparentAnswer: "El campo verificado es background: opaque | auto; esta página no promete salida transparente.",
    freeSignupAnswer: "La investigación no verifica generación gratuita o sin registro. Usa el flujo normal de Flatkey y consulta el bloque de precios fechado.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Crea una API key de Flatkey, elige ${endpoint} o ${secondEndpoint} según tu cliente y establece model en ${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `El ID verificado es ${id}; las rutas compatibles documentadas son ${endpoint} y ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `Las tarifas actuales del catálogo son ${rates} por 1 M de tokens.`,
    kimiFreeAnswer: "El catálogo actual muestra tarifas por token. No prometas acceso gratuito; comprueba la cuenta y la interfaz de precios.",
    kimiLocalAnswer: "El catálogo verificado no confirma pesos descargables, despliegue local ni requisitos de hardware/VRAM. No infieras esas propiedades de los metadatos.",
    kimiVendorAnswer: "El proveedor del catálogo es Moonshot AI; Flatkey proporciona el enrutamiento y la facturación.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Envía una solicitud a ${endpoint} o ${secondEndpoint}, usa el ID ${id} y sigue el formato del cliente compatible elegido.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `El ID verificado es ${id}; las rutas documentadas son ${endpoint} y ${secondEndpoint}.`,
    deepseekCostAnswer: "El precio depende de la franja UTC: en punta, entrada/caché/salida son $1.32/$0.044/$3.96; fuera de punta, $0.66/$0.022/$1.98 por 1 M de tokens.",
    deepseekLocalAnswer: "El catálogo actual no verifica pesos descargables ni despliegue local. No infieras esas afirmaciones de una marca de metadatos distillable.",
    deepseekVisionAnswer: "Solo se verifican modalidades de texto y archivo. No afirmes soporte visual o multimodal más amplio sin un documento autorizado.",
    deepseekQualityAnswer: "Esta página no publica un ranking de benchmark o calidad. Compara los IDs, endpoints, contexto, modalidades y campos de precio documentados.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 es un modelo de vídeo disponible mediante el flujo ${endpoint} de Flatkey.`,
    minimaxUseAnswer: "Guarda el task ID de la solicitud asíncrona y recupera el resultado cuando esté listo.",
    minimaxCostAnswer: "El catálogo público actual muestra una base de $0.08 por segundo. La resolución, los segundos de vídeo de referencia y la asignación de imágenes de entrada se muestran por separado en la estimación en vivo.",
    minimaxFieldsAnswer: "Configura resolución, duración, proporción y marca de agua AIGC antes de abrir la consola.",
    minimaxGenerationAnswer: "La página pública guarda primero el borrador. Regístrate o abre la consola para ejecutar la solicitud con una API key.",
    minimaxLocalAnswer: "El registro del catálogo no verifica ComfyUI, pesos locales ni otros despliegues; no los infieras de fragmentos de búsqueda.",
  },
  fr: {
    apiPricingDetails: "API, tarifs et détails du modèle",
    apiPricingAndModel: "API et tarifs",
    imageApiPricing: "API et génération d'images",
    videoApiPricing: "API et tarifs du générateur vidéo",
    pricing: "tarification",
    pricingByToken: "tarification par dimension de token",
    pricingByTokenAndDimensions: "tarification par token et dimensions d'image",
    pricingByUtc: "tarification par plage horaire UTC",
    pricingCurrent: "Tarifs actuels du catalogue Flatkey",
    pricingNoteToken: "Les tarifs sont indiqués pour 1 M de tokens ; consultez le bloc daté du catalogue pour les valeurs actuelles.",
    pricingNoteImage: "Le catalogue expose une tarification par dimension de token ; le montant final dépend des champs de requête et d'utilisation.",
    pricingNoteUtc: "Conservez ensemble les dimensions de tokens en période de pointe et hors pointe ; aucun prix moyen permanent n'est sous-entendu.",
    pricingNoteVideo: "Les lignes actuelles du catalogue sont facturées à la seconde ; vérifiez séparément la vidéo de référence et l'image d'entrée.",
    capabilities: "capacités",
    contextModalitiesEndpoint: "contexte, modalités et endpoint",
    imageControls: "tailles, qualité, formats et contrôles",
    fileEndpoints: "contexte, entrée de fichiers et endpoints compatibles",
    videoControls: "résolution, durée, ratio et filigrane",
    documentedFields: "Les champs d'intégration documentés sont présentés ci-dessous ; aucune promesse de benchmark ou de qualité n'est déduite.",
    compareFields: "Comparer les champs documentés",
    hostedApiFacts: "Faits sur l'API hébergée",
    migrationFields: "Champs de migration",
    videoFields: "Contrôles vidéo",
    api: "API",
    callWithApi: "API pour",
    imageApiSetup: "endpoint API et configuration de requête",
    textApiSetup: "API et ID du modèle",
    videoApiSetup: "configuration de l'API vidéo",
    faqDescription: "Tarifs, compatibilité, limites et détails de requête propres au modèle.",
    unknown: "Inconnu",
    notAsserted: "Non affirmé",
    notVerified: "Inconnu / non vérifié",
    perMillionTokens: "par 1 M de tokens",
    perMillionUnits: "par 1 M d'unités",
    perSecond: "par seconde",
    input: "Entrée",
    output: "Sortie",
    cacheRead: "Lecture du cache",
    cacheCreation: "Création du cache",
    cache: "Cache",
    inputTokens: "Tokens d'entrée",
    outputTokens: "Tokens de sortie",
    cacheTokens: "Tokens du cache",
    imageDimensions: "Dimensions de l'image",
    peakUtc: "Pointe (UTC)",
    offPeakUtc: "Hors pointe (UTC)",
    endpoint: "Endpoint",
    modelId: "ID du modèle",
    modalities: "Modalités",
    context: "Contexte",
    formats: "Formats",
    sizes: "Tailles",
    quality: "Qualité",
    controls: "Contrôles",
    access: "Accès",
    boundary: "Limite",
    hostedEndpoint: "Endpoint hébergé",
    inputModality: "Modalité d'entrée",
    downloadableWeights: "Poids téléchargeables",
    localRuntime: "Runtime local ou matériel",
    freeAccess: "Accès gratuit",
    resolution: "Résolution",
    duration: "Durée",
    ratio: "Ratio",
    watermark: "Filigrane AIGC",
    fields: "Champs",
    result: "Résultat",
    whatIs: (name) => `Qu'est-ce que ${name} ?`,
    howUse: (name) => `Comment utiliser ${name} ?`,
    apiModelId: (name) => `Quel est l'ID d'API du modèle ${name} ?`,
    cost: (name) => `Combien coûte ${name} ?`,
    contextInputs: (name) => `Quelle fenêtre de contexte et quelles entrées ${name} prend-il en charge ?`,
    release: (name) => `Quand ${name} est-il sorti ?`,
    accessQuestion: (name) => `Comment accéder à ${name} ?`,
    endpointQuestion: (name) => `Quel est l'endpoint API de ${name} ?`,
    sizeFormatQuestion: (name) => `Quelles tailles et quels formats ${name} prend-il en charge ?`,
    transparentQuestion: "GPT Image 2 peut-il créer des arrière-plans transparents ?",
    freeSignupQuestion: (name) => `${name} est-il gratuit ou disponible sans inscription ?`,
    freeQuestion: (name) => `${name} est-il gratuit ?`,
    localQuestion: (name) => `${name} est-il open source ou disponible localement ?`,
    vendorQuestion: (name) => `Qui crée ${name} ?`,
    priceUtcQuestion: (name) => `Comment ${name} est-il tarifé ?`,
    visionQuestion: (name) => `${name} prend-il en charge la vision ou les entrées multimodales ?`,
    qualityQuestion: (name) => `${name} est-il meilleur pour le code que V4 Flash ?`,
    settingsQuestion: (name) => `Quels réglages de ${name} puis-je configurer ici ?`,
    configureQuestion: (name) => `Comment configurer une requête ${name} ?`,
    generationQuestion: "Cela lance-t-il une génération réelle ?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} est un modèle du catalogue ${vendor}, disponible via l'endpoint ${endpoint} de Flatkey. Le catalogue vérifié indique ${facts}. Cette page affiche l'ID exact, les tarifs actuels et le chemin de requête avant le trafic de production.`,
    imageStatement: (name, endpoint) => `${name} est le modèle d'image d'OpenAI exposé dans le flux de génération d'images Flatkey. Utilisez ${endpoint} et configurez le nombre, la taille, la qualité, le format, l'arrière-plan et la modération avant l'envoi.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} est le modèle du catalogue Moonshot AI disponible via les routes chat et messages compatibles de Flatkey. Le catalogue vérifié indique un contexte de ${context} et une entrée fichier ; les chemins documentés sont ${endpoint} et ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} est le modèle du catalogue DeepSeek documenté avec des modalités texte et fichier, un contexte de ${context} et deux chemins API compatibles : ${endpoint} et ${secondEndpoint}. Le tarif dépend de l'heure UTC.`,
    minimaxStatement: (name, endpoint) => `${name} est disponible via le flux asynchrone ${endpoint} de Flatkey. Configurez une résolution 768P ou 2K, 4–15 secondes, un ratio pris en charge et le champ de filigrane AIGC avant d'ouvrir la console.`,
    contextBody: (context) => `Le catalogue indique une fenêtre de contexte de ${context}.`,
    modalitiesBody: (modalities) => `Les métadonnées du catalogue listent ${modalities}.`,
    endpointBody: (endpoint, id) => `Les requêtes utilisent ${endpoint} avec l'ID ${id}.`,
    keyBillingBody: "Flatkey fournit la couche de clé API et de facturation, avec compatibilité d'endpoint et de requête.",
    imageCountBody: "n accepte 1–10 dans le générateur de la page.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536 ou auto.",
    imageQualityBody: "auto, high, medium ou low.",
    imageFormatsBody: "PNG, JPEG ou WebP.",
    imageBackgroundBody: "Le champ background accepte opaque ou auto ; moderation accepte auto ou low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} et ${secondEndpoint} sont les endpoints compatibles enregistrés.`,
    fileBody: "La modalité fichier est listée dans le catalogue ; n'extrapolez pas d'autres médias.",
    knowledgeBody: "Les exemples de code, de documents et de recherche sont des cas d'usage éditoriaux, pas des benchmarks.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "Un indicateur de métadonnées distillable ne prouve pas que le modèle est open source, téléchargeable ou exécutable localement.",
    resolutionBody: "Choisissez 768P ou 2K.",
    durationBody: "Configurez une requête de 4–15 secondes.",
    ratioBody: "Utilisez un ratio fixe pris en charge ou adaptive.",
    watermarkBody: "Définissez explicitement le booléen de filigrane AIGC.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Les métadonnées du catalogue listent ${modalities}.`,
    apiCompatibilityDetail: "Utilisez les champs de requête compatibles ; n'inférez pas de fonctions grand public non vérifiées.",
    apiImageControlsDetail: "n, size, quality, format, background et moderation.",
    apiAccessDetail: "Utilisez le flux normal de compte et de clé API Flatkey ; aucune génération gratuite ou sans inscription n'est promise.",
    apiBoundaryDetail: "Les routes compatibles ne prouvent pas la prise en charge locale ou open source.",
    apiResultDetail: "Utilisez l'ID de tâche renvoyé avec l'endpoint content.",
    costAnswer: (name, rates) => `Les tarifs actuels du catalogue pour ${name} sont ${rates}. Consultez le bloc daté avant le trafic de production.`,
    contextAnswer: (context, modalities) => `Le catalogue indique ${context} et ${modalities}. Cette page n'infère pas de modalité supplémentaire.`,
    releaseAnswer: (date) => `L'instantané du catalogue indique ${date} comme date de sortie ; il s'agit d'une métadonnée et non d'une annonce indépendante.`,
    imageAccessAnswer: "Configurez une requête, puis utilisez le flux normal de compte et de clé API Flatkey. La page ne promet pas de génération sans inscription.",
    imageEndpointAnswer: (id, endpoint) => `Utilisez ${endpoint} avec l'ID ${id} et les champs pris en charge indiqués dans la section API.`,
    imageCostAnswer: (rates) => `Le catalogue indique ${rates} par 1 M d'unités ; ce n'est pas un prix fixe par image.`,
    imageSizeAnswer: "Les tailles sont 1024x1024, 1536x1024, 1024x1536 et auto. Les formats sont PNG, JPEG et WebP ; la qualité est auto/high/medium/low.",
    transparentAnswer: "Le champ vérifié est background: opaque | auto ; cette page ne promet pas de sortie transparente.",
    freeSignupAnswer: "La recherche ne vérifie pas une génération gratuite ou sans inscription. Utilisez le flux Flatkey normal et consultez le bloc de tarifs daté.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Créez une clé API Flatkey, choisissez ${endpoint} ou ${secondEndpoint} selon votre client et définissez model sur ${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `L'ID vérifié est ${id} ; les routes compatibles documentées sont ${endpoint} et ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `Les tarifs actuels du catalogue sont ${rates} par 1 M de tokens.`,
    kimiFreeAnswer: "Le catalogue actuel affiche des tarifs par token. Ne promettez pas d'accès gratuit ; vérifiez le compte et l'interface de tarification.",
    kimiLocalAnswer: "Le catalogue vérifié ne confirme ni poids téléchargeables, ni déploiement local, ni exigences matérielles/VRAM. N'inférez pas ces propriétés des métadonnées.",
    kimiVendorAnswer: "Le fournisseur du catalogue est Moonshot AI ; Flatkey fournit le routage et la facturation.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Envoyez une requête à ${endpoint} ou ${secondEndpoint}, utilisez l'ID ${id} et suivez le format du client compatible choisi.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `L'ID vérifié est ${id} ; les chemins documentés sont ${endpoint} et ${secondEndpoint}.`,
    deepseekCostAnswer: "Le tarif dépend de l'heure UTC : en pointe, entrée/cache/sortie $1.32/$0.044/$3.96 ; hors pointe, $0.66/$0.022/$1.98 par 1 M de tokens.",
    deepseekLocalAnswer: "Le catalogue actuel ne vérifie ni poids téléchargeables ni déploiement local. N'inférez pas ces affirmations d'un indicateur distillable.",
    deepseekVisionAnswer: "Seules les modalités texte et fichier sont vérifiées. Ne revendiquez pas la vision ou un multimodal plus large sans document d'autorité.",
    deepseekQualityAnswer: "Cette page ne publie aucun classement de benchmark ou de qualité. Comparez plutôt les IDs, endpoints, contexte, modalités et champs de prix documentés.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 est un modèle vidéo disponible via le flux ${endpoint} de Flatkey.`,
    minimaxUseAnswer: "Conservez l'ID de tâche renvoyé par la requête asynchrone et récupérez le résultat lorsqu'il est prêt.",
    minimaxCostAnswer: "Le catalogue public actuel indique une base de $0.08 par seconde. La résolution, les secondes de vidéo de référence et le quota d’images d’entrée sont séparés et doivent être vérifiés dans l’estimation en direct.",
    minimaxFieldsAnswer: "Configurez la résolution, la durée, le ratio et le filigrane AIGC avant d'ouvrir la console.",
    minimaxGenerationAnswer: "La page publique enregistre d'abord votre brouillon. Inscrivez-vous ou ouvrez la console pour exécuter la requête avec une clé API.",
    minimaxLocalAnswer: "Le catalogue ne vérifie pas ComfyUI, les poids locaux ou d'autres déploiements ; ne les déduisez pas d'extraits de recherche.",
  },
  pt: {
    apiPricingDetails: "API, preços e detalhes do modelo",
    apiPricingAndModel: "API e preços",
    imageApiPricing: "API e geração de imagens",
    videoApiPricing: "API e preços do gerador de vídeo",
    pricing: "preços",
    pricingByToken: "preços por dimensão de token",
    pricingByTokenAndDimensions: "preços por token e dimensões da imagem",
    pricingByUtc: "preços por faixa de horário UTC",
    pricingCurrent: "Tarifas atuais do catálogo Flatkey",
    pricingNoteToken: "As tarifas são por 1 milhão de tokens; consulte o bloco datado do catálogo para os valores atuais.",
    pricingNoteImage: "O catálogo expõe preços por dimensão de token; o valor final depende dos campos de solicitação e uso.",
    pricingNoteUtc: "Mantenha juntas as dimensões de tokens de pico e fora de pico; nenhum preço médio permanente é implícito.",
    pricingNoteVideo: "As linhas atuais do catálogo são por segundo; verifique separadamente o vídeo de referência e a imagem de entrada.",
    capabilities: "capacidades",
    contextModalitiesEndpoint: "contexto, modalidades e endpoint",
    imageControls: "tamanhos, qualidade, formatos e controles",
    fileEndpoints: "contexto, entrada de arquivos e endpoints compatíveis",
    videoControls: "resolução, duração, proporção e marca-d'água",
    documentedFields: "Os campos de integração documentados aparecem abaixo; nenhuma promessa de benchmark ou qualidade é inferida.",
    compareFields: "Comparar campos documentados",
    hostedApiFacts: "Fatos da API hospedada",
    migrationFields: "Campos de migração",
    videoFields: "Controles de vídeo",
    api: "API",
    callWithApi: "API para",
    imageApiSetup: "endpoint da API e configuração da solicitação",
    textApiSetup: "API e ID do modelo",
    videoApiSetup: "configuração da API de vídeo",
    faqDescription: "Preços, compatibilidade, limites e detalhes de solicitação específicos do modelo.",
    unknown: "Desconhecido",
    notAsserted: "Não declarado",
    notVerified: "Desconhecido / não verificado",
    perMillionTokens: "por 1 milhão de tokens",
    perMillionUnits: "por 1 milhão de unidades",
    perSecond: "por segundo",
    input: "Entrada",
    output: "Saída",
    cacheRead: "Leitura do cache",
    cacheCreation: "Criação do cache",
    cache: "Cache",
    inputTokens: "Tokens de entrada",
    outputTokens: "Tokens de saída",
    cacheTokens: "Tokens de cache",
    imageDimensions: "Dimensões da imagem",
    peakUtc: "Pico (UTC)",
    offPeakUtc: "Fora de pico (UTC)",
    endpoint: "Endpoint",
    modelId: "ID do modelo",
    modalities: "Modalidades",
    context: "Contexto",
    formats: "Formatos",
    sizes: "Tamanhos",
    quality: "Qualidade",
    controls: "Controles",
    access: "Acesso",
    boundary: "Limite",
    hostedEndpoint: "Endpoint hospedado",
    inputModality: "Modalidade de entrada",
    downloadableWeights: "Pesos para download",
    localRuntime: "Runtime local ou hardware",
    freeAccess: "Acesso gratuito",
    resolution: "Resolução",
    duration: "Duração",
    ratio: "Proporção",
    watermark: "Marca-d'água AIGC",
    fields: "Campos",
    result: "Resultado",
    whatIs: (name) => `O que é ${name}?`,
    howUse: (name) => `Como usar ${name}?`,
    apiModelId: (name) => `Qual é o ID do modelo da API ${name}?`,
    cost: (name) => `Quanto custa ${name}?`,
    contextInputs: (name) => `Qual janela de contexto e quais entradas ${name} aceita?`,
    release: (name) => `Quando ${name} foi lançado?`,
    accessQuestion: (name) => `Como acessar ${name}?`,
    endpointQuestion: (name) => `Qual é o endpoint da API ${name}?`,
    sizeFormatQuestion: (name) => `Quais tamanhos e formatos ${name} aceita?`,
    transparentQuestion: "O GPT Image 2 cria fundos transparentes?",
    freeSignupQuestion: (name) => `${name} é gratuito ou está disponível sem cadastro?`,
    freeQuestion: (name) => `${name} é gratuito?`,
    localQuestion: (name) => `${name} é open source ou pode ser usado localmente?`,
    vendorQuestion: (name) => `Quem fornece ${name}?`,
    priceUtcQuestion: (name) => `Como ${name} é cobrado?`,
    visionQuestion: (name) => `${name} aceita entrada visual ou multimodal?`,
    qualityQuestion: (name) => `${name} é melhor para programação do que o V4 Flash?`,
    settingsQuestion: (name) => `Quais configurações de ${name} posso ajustar aqui?`,
    configureQuestion: (name) => `Como configurar uma solicitação de ${name}?`,
    generationQuestion: "Isso inicia uma geração real?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} é um modelo do catálogo da ${vendor}, disponível pelo endpoint ${endpoint} da Flatkey. O catálogo verificado registra ${facts}. Esta página mostra o ID exato, as tarifas atuais e o caminho da solicitação antes do tráfego de produção.`,
    imageStatement: (name, endpoint) => `${name} é o modelo de imagem da OpenAI exposto no fluxo de geração da Flatkey. Use ${endpoint} e configure quantidade, tamanho, qualidade, formato, fundo e moderação antes de enviar a solicitação.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} é o modelo do catálogo da Moonshot AI disponível pelas rotas compatíveis de chat e mensagens da Flatkey. O catálogo verificado registra contexto de ${context} e entrada de arquivos; os caminhos documentados são ${endpoint} e ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} é o modelo do catálogo da DeepSeek documentado com modalidades de texto e arquivo, contexto de ${context} e dois caminhos de API compatíveis: ${endpoint} e ${secondEndpoint}. O preço varia conforme a faixa UTC.`,
    minimaxStatement: (name, endpoint) => `${name} está disponível pelo fluxo assíncrono ${endpoint} da Flatkey. Configure resolução 768P ou 2K, 4–15 segundos, uma proporção compatível e o campo de marca-d'água AIGC antes de abrir o console.`,
    contextBody: (context) => `O catálogo registra uma janela de contexto de ${context}.`,
    modalitiesBody: (modalities) => `Os metadados do catálogo listam ${modalities}.`,
    endpointBody: (endpoint, id) => `As solicitações usam ${endpoint} com o ID de modelo ${id}.`,
    keyBillingBody: "A Flatkey fornece a camada de API key e cobrança com compatibilidade de endpoint e solicitação.",
    imageCountBody: "n aceita 1–10 no gerador da página.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536 ou auto.",
    imageQualityBody: "auto, high, medium ou low.",
    imageFormatsBody: "PNG, JPEG ou WebP.",
    imageBackgroundBody: "O campo background aceita opaque ou auto; moderation aceita auto ou low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} e ${secondEndpoint} são os endpoints compatíveis registrados.`,
    fileBody: "A modalidade de arquivo aparece no catálogo; não extrapole outros tipos de mídia.",
    knowledgeBody: "Os exemplos de código, documentos e pesquisa são casos de uso editoriais, não resultados de benchmark.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "Uma marca de metadados distillable não prova que o modelo seja open source, baixável ou executável localmente.",
    resolutionBody: "Escolha 768P ou 2K.",
    durationBody: "Configure uma solicitação de 4–15 segundos.",
    ratioBody: "Use uma proporção fixa compatível ou adaptive.",
    watermarkBody: "Defina explicitamente o booleano da marca-d'água AIGC.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Os metadados do catálogo listam ${modalities}.`,
    apiCompatibilityDetail: "Use campos de solicitação compatíveis; não infira recursos de consumidor não verificados.",
    apiImageControlsDetail: "n, size, quality, format, background e moderation.",
    apiAccessDetail: "Use o fluxo normal de conta e API key da Flatkey; não há promessa de geração gratuita ou sem cadastro.",
    apiBoundaryDetail: "Rotas compatíveis não comprovam suporte local ou open source.",
    apiResultDetail: "Use o task ID retornado com o endpoint de conteúdo.",
    costAnswer: (name, rates) => `As tarifas atuais do catálogo para ${name} são ${rates}. Consulte o bloco de preços datado antes do tráfego de produção.`,
    contextAnswer: (context, modalities) => `O catálogo registra ${context} e ${modalities}. Esta página não infere modalidades adicionais.`,
    releaseAnswer: (date) => `O snapshot do catálogo registra ${date} como data de lançamento; trate isso como metadado do catálogo, não como anúncio separado.`,
    imageAccessAnswer: "Configure uma solicitação e use o fluxo normal de conta e API key da Flatkey. A página não promete geração sem cadastro.",
    imageEndpointAnswer: (id, endpoint) => `Use ${endpoint} com o ID ${id} e os campos compatíveis listados na seção de API.`,
    imageCostAnswer: (rates) => `O catálogo lista ${rates} por 1 milhão de unidades; não é um preço fixo por imagem.`,
    imageSizeAnswer: "Os tamanhos são 1024x1024, 1536x1024, 1024x1536 e auto. Os formatos são PNG, JPEG e WebP; a qualidade é auto/high/medium/low.",
    transparentAnswer: "O campo verificado é background: opaque | auto; esta página não promete saída transparente.",
    freeSignupAnswer: "A pesquisa não verifica geração gratuita ou sem cadastro. Use o fluxo normal da Flatkey e confira o bloco de preços datado.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Crie uma API key Flatkey, escolha ${endpoint} ou ${secondEndpoint} conforme o cliente e defina model como ${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `O ID verificado é ${id}; as rotas compatíveis documentadas são ${endpoint} e ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `As tarifas atuais do catálogo são ${rates} por 1 milhão de tokens.`,
    kimiFreeAnswer: "O catálogo atual mostra tarifas por token. Não prometa acesso gratuito; confira a conta e a interface de preços.",
    kimiLocalAnswer: "O catálogo verificado não confirma pesos para download, implantação local ou requisitos de hardware/VRAM. Não deduza essas propriedades dos metadados.",
    kimiVendorAnswer: "O fornecedor do catálogo é a Moonshot AI; a Flatkey fornece roteamento e cobrança.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Envie uma solicitação para ${endpoint} ou ${secondEndpoint}, use o ID ${id} e siga o formato do cliente compatível escolhido.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `O ID verificado é ${id}; os caminhos documentados são ${endpoint} e ${secondEndpoint}.`,
    deepseekCostAnswer: "O preço é por faixa UTC: no pico, entrada/cache/saída são $1.32/$0.044/$3.96; fora do pico, $0.66/$0.022/$1.98 por 1 milhão de tokens.",
    deepseekLocalAnswer: "O catálogo atual não verifica pesos para download nem implantação local. Não deduza essas afirmações de uma marca de metadados distillable.",
    deepseekVisionAnswer: "Apenas modalidades de texto e arquivo são verificadas. Não afirme suporte visual ou multimodal mais amplo sem documentação autorizada.",
    deepseekQualityAnswer: "Esta página não publica ranking de benchmark ou qualidade. Compare IDs, endpoints, contexto, modalidades e campos de preço documentados.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 é um modelo de vídeo disponível pelo fluxo ${endpoint} da Flatkey.`,
    minimaxUseAnswer: "Guarde o task ID retornado pela solicitação assíncrona e recupere o resultado quando estiver pronto.",
    minimaxCostAnswer: "O catálogo público atual mostra uma base de $0.08 por segundo. A resolução, os segundos de vídeo de referência e a franquia de imagens de entrada aparecem separadamente na estimativa ao vivo.",
    minimaxFieldsAnswer: "Configure resolução, duração, proporção e marca-d'água AIGC antes de abrir o console.",
    minimaxGenerationAnswer: "A página pública salva o rascunho primeiro. Cadastre-se ou abra o console para executar a solicitação com uma API key.",
    minimaxLocalAnswer: "O registro do catálogo não verifica ComfyUI, pesos locais ou outros meios de implantação; não deduza isso de snippets de busca.",
  },
  ru: {
    apiPricingDetails: "API, цены и сведения о модели",
    apiPricingAndModel: "API и цены",
    imageApiPricing: "API и генерация изображений",
    videoApiPricing: "API и цены видеогенератора",
    pricing: "цены",
    pricingByToken: "цены по измерению токенов",
    pricingByTokenAndDimensions: "цены по токенам и размерам изображения",
    pricingByUtc: "цены по часовым зонам UTC",
    pricingCurrent: "Текущие тарифы каталога Flatkey",
    pricingNoteToken: "Тарифы указаны за 1 млн токенов; актуальные значения смотрите в датированном блоке каталога.",
    pricingNoteImage: "Каталог показывает цены по измерениям токенов; итоговая сумма зависит от полей запроса и использования.",
    pricingNoteUtc: "Пиковые и непиковые измерения токенов нужно рассматривать вместе; единая постоянная цена не подразумевается.",
    pricingNoteVideo: "Текущие строки каталога указаны за секунду; тарифы для референсного видео и входного изображения смотрите отдельно.",
    capabilities: "возможности",
    contextModalitiesEndpoint: "контекст, модальности и endpoint",
    imageControls: "размеры, качество, форматы и параметры",
    fileEndpoints: "контекст, ввод файлов и совместимые endpoint",
    videoControls: "разрешение, длительность, соотношение и водяной знак",
    documentedFields: "Ниже приведены документированные поля интеграции; обещания о бенчмарках или качестве не делаются.",
    compareFields: "Сравнение документированных полей",
    hostedApiFacts: "Факты о размещённом API",
    migrationFields: "Поля миграции",
    videoFields: "Параметры видео",
    api: "API",
    callWithApi: "API для",
    imageApiSetup: "endpoint API и настройка запроса",
    textApiSetup: "API и ID модели",
    videoApiSetup: "настройка видео API",
    faqDescription: "Цены, совместимость, ограничения и детали запросов для этой модели.",
    unknown: "Неизвестно",
    notAsserted: "Не заявлено",
    notVerified: "Неизвестно / не проверено",
    perMillionTokens: "за 1 млн токенов",
    perMillionUnits: "за 1 млн единиц",
    perSecond: "за секунду",
    input: "Ввод",
    output: "Вывод",
    cacheRead: "Чтение кэша",
    cacheCreation: "Создание кэша",
    cache: "Кэш",
    inputTokens: "Входные токены",
    outputTokens: "Выходные токены",
    cacheTokens: "Токены кэша",
    imageDimensions: "Размеры изображения",
    peakUtc: "Пик (UTC)",
    offPeakUtc: "Вне пика (UTC)",
    endpoint: "Endpoint",
    modelId: "ID модели",
    modalities: "Модальности",
    context: "Контекст",
    formats: "Форматы",
    sizes: "Размеры",
    quality: "Качество",
    controls: "Параметры",
    access: "Доступ",
    boundary: "Граница утверждений",
    hostedEndpoint: "Размещённый endpoint",
    inputModality: "Модальность ввода",
    downloadableWeights: "Загружаемые веса",
    localRuntime: "Локальная среда или оборудование",
    freeAccess: "Бесплатный доступ",
    resolution: "Разрешение",
    duration: "Длительность",
    ratio: "Соотношение",
    watermark: "Водяной знак AIGC",
    fields: "Поля",
    result: "Результат",
    whatIs: (name) => `Что такое ${name}?`,
    howUse: (name) => `Как использовать ${name}?`,
    apiModelId: (name) => `Какой API ID у модели ${name}?`,
    cost: (name) => `Сколько стоит ${name}?`,
    contextInputs: (name) => `Какое контекстное окно и ввод поддерживает ${name}?`,
    release: (name) => `Когда выпущена ${name}?`,
    accessQuestion: (name) => `Как получить доступ к ${name}?`,
    endpointQuestion: (name) => `Какой API endpoint у ${name}?`,
    sizeFormatQuestion: (name) => `Какие размеры и форматы поддерживает ${name}?`,
    transparentQuestion: "Может ли GPT Image 2 создавать прозрачный фон?",
    freeSignupQuestion: (name) => `${name} бесплатна или доступна без регистрации?`,
    freeQuestion: (name) => `${name} бесплатна?`,
    localQuestion: (name) => `${name} имеет открытый исходный код или доступна локально?`,
    vendorQuestion: (name) => `Кто выпускает ${name}?`,
    priceUtcQuestion: (name) => `Как рассчитывается цена ${name}?`,
    visionQuestion: (name) => `Поддерживает ли ${name} зрение или мультимодальный ввод?`,
    qualityQuestion: (name) => `Лучше ли ${name} подходит для кода, чем V4 Flash?`,
    settingsQuestion: (name) => `Какие параметры ${name} можно настроить здесь?`,
    configureQuestion: (name) => `Как настроить запрос ${name}?`,
    generationQuestion: "Это запускает настоящую генерацию?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} — модель из каталога ${vendor}, доступная через endpoint ${endpoint} Flatkey. Проверенный каталог указывает: ${facts}. На странице показаны точный ID модели, текущие тарифы и путь запроса до запуска рабочего трафика.`,
    imageStatement: (name, endpoint) => `${name} — модель изображений OpenAI в потоке генерации Flatkey. Используйте ${endpoint} и перед отправкой настройте количество, размер, качество, формат, фон и модерацию.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} — модель Moonshot AI из каталога, доступная через совместимые маршруты chat и messages Flatkey. Проверенный каталог указывает контекст ${context} и ввод файлов; документированные пути: ${endpoint} и ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} — модель DeepSeek из каталога с текстовой и файловой модальностями, контекстом ${context} и двумя совместимыми API-путями: ${endpoint} и ${secondEndpoint}. Цена зависит от времени UTC.`,
    minimaxStatement: (name, endpoint) => `${name} доступна через асинхронный поток ${endpoint} Flatkey. Перед открытием консоли настройте разрешение 768P или 2K, 4–15 секунд, соотношение и поле водяного знака AIGC.`,
    contextBody: (context) => `Каталог указывает контекстное окно ${context}.`,
    modalitiesBody: (modalities) => `Метаданные каталога перечисляют: ${modalities}.`,
    endpointBody: (endpoint, id) => `Запросы используют ${endpoint} с ID модели ${id}.`,
    keyBillingBody: "Flatkey предоставляет уровень API-ключа и биллинга с совместимостью endpoint и запроса.",
    imageCountBody: "В генераторе страницы n принимает значения 1–10.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536 или auto.",
    imageQualityBody: "auto, high, medium или low.",
    imageFormatsBody: "PNG, JPEG или WebP.",
    imageBackgroundBody: "Поле background принимает opaque или auto; moderation — auto или low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} и ${secondEndpoint} — зарегистрированные совместимые endpoint.`,
    fileBody: "Файловая модальность указана в каталоге; не следует выводить другие типы медиа.",
    knowledgeBody: "Примеры для кода, документов и исследований — редакционные сценарии, а не результаты бенчмарков.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "Метаданный флаг distillable не означает открытый исходный код, скачиваемые веса или локальный запуск.",
    resolutionBody: "Выберите 768P или 2K.",
    durationBody: "Настройте запрос длительностью 4–15 секунд.",
    ratioBody: "Используйте поддерживаемое фиксированное соотношение или adaptive.",
    watermarkBody: "Явно задайте логическое поле водяного знака AIGC.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Метаданные каталога перечисляют: ${modalities}.`,
    apiCompatibilityDetail: "Используйте совместимые поля запроса; не делайте выводов о непроверенных потребительских функциях.",
    apiImageControlsDetail: "n, size, quality, format, background и moderation.",
    apiAccessDetail: "Используйте обычный поток аккаунта и API-ключа Flatkey; бесплатный доступ или генерация без регистрации не обещаются.",
    apiBoundaryDetail: "Совместимые маршруты не доказывают локальную или open-source поддержку.",
    apiResultDetail: "Используйте возвращённый task ID с content endpoint.",
    costAnswer: (name, rates) => `Текущие тарифы каталога для ${name}: ${rates}. Перед запуском рабочего трафика проверьте датированный блок цен.`,
    contextAnswer: (context, modalities) => `Каталог указывает ${context} и ${modalities}. Дополнительные модальности на этой странице не предполагаются.`,
    releaseAnswer: (date) => `Снимок каталога указывает дату ${date}; это метаданные каталога, а не отдельное объявление запуска.`,
    imageAccessAnswer: "Настройте запрос, затем используйте обычный поток аккаунта и API-ключа Flatkey. Страница не обещает генерацию без регистрации.",
    imageEndpointAnswer: (id, endpoint) => `Используйте ${endpoint} с ID ${id} и поддерживаемыми полями из раздела API.`,
    imageCostAnswer: (rates) => `Каталог указывает ${rates} за 1 млн единиц; это не единая цена за изображение.`,
    imageSizeAnswer: "Размеры: 1024x1024, 1536x1024, 1024x1536 и auto. Форматы: PNG, JPEG и WebP; качество: auto/high/medium/low.",
    transparentAnswer: "Проверенное поле: background: opaque | auto; прозрачный вывод на этой странице не обещается.",
    freeSignupAnswer: "Исследование не подтверждает бесплатную генерацию или работу без регистрации. Используйте обычный поток Flatkey и датированный блок цен.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Создайте API-ключ Flatkey, выберите ${endpoint} или ${secondEndpoint} согласно клиенту и задайте model=${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `Проверенный ID модели: ${id}; документированные совместимые маршруты: ${endpoint} и ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `Текущие тарифы каталога: ${rates} за 1 млн токенов.`,
    kimiFreeAnswer: "Текущий каталог показывает платные тарифы за токены. Не обещайте бесплатный доступ; проверьте аккаунт и интерфейс цен.",
    kimiLocalAnswer: "Проверенный каталог не подтверждает скачиваемые веса, локальное развертывание или требования к hardware/VRAM. Не выводите это из метаданных.",
    kimiVendorAnswer: "Поставщик каталога — Moonshot AI; Flatkey отвечает за маршрутизацию и биллинг.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Отправьте запрос на ${endpoint} или ${secondEndpoint}, используйте ID ${id} и формат выбранного совместимого клиента.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `Проверенный ID модели: ${id}; документированные пути: ${endpoint} и ${secondEndpoint}.`,
    deepseekCostAnswer: "Цена зависит от времени UTC: пик input/cache-read/output — $1.32/$0.044/$3.96, вне пика — $0.66/$0.022/$1.98 за 1 млн токенов.",
    deepseekLocalAnswer: "Текущий каталог не подтверждает скачиваемые веса или локальное развертывание. Не выводите это из флага distillable.",
    deepseekVisionAnswer: "Проверены только текстовая и файловая модальности. Не заявляйте vision или более широкую мультимодальность без авторитетной документации.",
    deepseekQualityAnswer: "Страница не публикует рейтинг бенчмарков или качества. Сравнивайте документированные ID, endpoint, контекст, модальности и цены.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 — видеомодель, доступная через поток ${endpoint} Flatkey.`,
    minimaxUseAnswer: "Сохраните task ID асинхронного запроса и получите результат после готовности.",
    minimaxCostAnswer: "В текущем публичном каталоге указана базовая ставка $0.08 за секунду. Разрешение, секунды референсного видео и лимит входных изображений показываются отдельно в актуальной оценке.",
    minimaxFieldsAnswer: "Перед открытием консоли настройте разрешение, длительность, соотношение и водяной знак AIGC.",
    minimaxGenerationAnswer: "Публичная страница сначала сохраняет черновик. Зарегистрируйтесь или откройте консоль, чтобы выполнить запрос с API-ключом.",
    minimaxLocalAnswer: "Запись каталога не подтверждает ComfyUI, локальные веса или другие способы развёртывания; не делайте выводов из поисковых фрагментов.",
  },
  ja: {
    apiPricingDetails: "API・料金・モデル詳細",
    apiPricingAndModel: "API と料金",
    imageApiPricing: "API と画像生成",
    videoApiPricing: "動画生成 API と料金",
    pricing: "料金",
    pricingByToken: "トークン単位の料金",
    pricingByTokenAndDimensions: "トークンと画像サイズ別の料金",
    pricingByUtc: "UTC 時間帯別の料金",
    pricingCurrent: "Flatkey カタログの現在の料金",
    pricingNoteToken: "料金は 100 万トークン単位です。現在値は日付付きのカタログ欄を確認してください。",
    pricingNoteImage: "カタログではトークン単位の料金を示しています。最終額はリクエストと使用量の項目で変わります。",
    pricingNoteUtc: "ピークとオフピークのトークン項目を併記しています。常時適用される平均料金を意味しません。",
    pricingNoteVideo: "現在のカタログ行は秒単位です。参照動画と入力画像の料金は分けて確認してください。",
    capabilities: "機能",
    contextModalitiesEndpoint: "コンテキスト・モダリティ・エンドポイント",
    imageControls: "サイズ・品質・形式・制御項目",
    fileEndpoints: "コンテキスト・ファイル入力・互換エンドポイント",
    videoControls: "解像度・長さ・比率・透かし",
    documentedFields: "以下は文書化された統合項目です。ベンチマークや品質の約束は推測していません。",
    compareFields: "文書化された項目を比較",
    hostedApiFacts: "ホスト型 API の事実",
    migrationFields: "移行項目",
    videoFields: "動画制御",
    api: "API",
    callWithApi: "API の利用：",
    imageApiSetup: "API エンドポイントとリクエスト設定",
    textApiSetup: "API とモデル ID",
    videoApiSetup: "動画 API の設定",
    faqDescription: "料金、互換性、制限、モデル固有のリクエスト情報。",
    unknown: "不明",
    notAsserted: "主張なし",
    notVerified: "不明 / 未確認",
    perMillionTokens: "100 万トークンあたり",
    perMillionUnits: "100 万単位あたり",
    perSecond: "秒あたり",
    input: "入力",
    output: "出力",
    cacheRead: "キャッシュ読み取り",
    cacheCreation: "キャッシュ作成",
    cache: "キャッシュ",
    inputTokens: "入力トークン",
    outputTokens: "出力トークン",
    cacheTokens: "キャッシュトークン",
    imageDimensions: "画像サイズ",
    peakUtc: "ピーク（UTC）",
    offPeakUtc: "オフピーク（UTC）",
    endpoint: "エンドポイント",
    modelId: "モデル ID",
    modalities: "モダリティ",
    context: "コンテキスト",
    formats: "形式",
    sizes: "サイズ",
    quality: "品質",
    controls: "制御項目",
    access: "アクセス",
    boundary: "確認範囲",
    hostedEndpoint: "ホスト型エンドポイント",
    inputModality: "入力モダリティ",
    downloadableWeights: "ダウンロード可能な重み",
    localRuntime: "ローカル実行環境またはハードウェア",
    freeAccess: "無料アクセス",
    resolution: "解像度",
    duration: "長さ",
    ratio: "アスペクト比",
    watermark: "AIGC 透かし",
    fields: "項目",
    result: "結果",
    whatIs: (name) => `${name}とは？`,
    howUse: (name) => `${name}の使い方は？`,
    apiModelId: (name) => `${name} API のモデル ID は？`,
    cost: (name) => `${name}の料金は？`,
    contextInputs: (name) => `${name}のコンテキスト長と入力形式は？`,
    release: (name) => `${name}のリリース日は？`,
    accessQuestion: (name) => `${name}にアクセスするには？`,
    endpointQuestion: (name) => `${name} API のエンドポイントは？`,
    sizeFormatQuestion: (name) => `${name}が対応するサイズと形式は？`,
    transparentQuestion: "GPT Image 2 は透明背景を作成できますか？",
    freeSignupQuestion: (name) => `${name}は無料、または登録なしで利用できますか？`,
    freeQuestion: (name) => `${name}は無料ですか？`,
    localQuestion: (name) => `${name}はオープンソース、またはローカルで利用できますか？`,
    vendorQuestion: (name) => `${name}の提供元は？`,
    priceUtcQuestion: (name) => `${name}の料金体系は？`,
    visionQuestion: (name) => `${name}は画像・マルチモーダル入力に対応しますか？`,
    qualityQuestion: (name) => `${name}は V4 Flash よりコーディングに適していますか？`,
    settingsQuestion: (name) => `ここで設定できる ${name} の項目は？`,
    configureQuestion: (name) => `${name} のリクエストを設定するには？`,
    generationQuestion: "ここで実際の生成が開始されますか？",
    modelStatement: (name, vendor, endpoint, facts) => `${name} は ${vendor} のカタログモデルで、Flatkey の ${endpoint} エンドポイントから利用できます。確認済みのカタログ情報は ${facts} です。本番トラフィックの前に、モデル ID、現在の料金、リクエスト経路を確認できます。`,
    imageStatement: (name, endpoint) => `${name} は Flatkey の画像生成フローで提供される OpenAI の画像モデルです。${endpoint} を使い、送信前に枚数、サイズ、品質、形式、背景、モデレーションを設定します。`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} は Moonshot AI のカタログモデルで、Flatkey の chat / messages 互換ルートから利用できます。確認済みのコンテキストは ${context}、ファイル入力に対応し、文書化されたパスは ${endpoint} と ${secondEndpoint} です。`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} は DeepSeek のカタログモデルで、テキスト・ファイルモダリティ、${context} のコンテキスト、${endpoint} と ${secondEndpoint} の互換 API パスが記録されています。料金は UTC の時間帯で変わります。`,
    minimaxStatement: (name, endpoint) => `${name} は Flatkey の非同期 ${endpoint} フローで利用できます。コンソールを開く前に、768P/2K、4–15 秒、対応比率、AIGC 透かしを設定できます。`,
    contextBody: (context) => `カタログのコンテキストウィンドウは ${context} です。`,
    modalitiesBody: (modalities) => `カタログのメタデータには ${modalities} が記載されています。`,
    endpointBody: (endpoint, id) => `リクエストは ${endpoint} をモデル ID ${id} で使用します。`,
    keyBillingBody: "Flatkey は API キーと請求のレイヤーを提供し、エンドポイントとリクエストの互換性を保ちます。",
    imageCountBody: "ページのジェネレーターでは n に 1–10 を指定できます。",
    imageSizesBody: "1024x1024、1536x1024、1024x1536、または auto。",
    imageQualityBody: "auto、high、medium、または low。",
    imageFormatsBody: "PNG、JPEG、または WebP。",
    imageBackgroundBody: "background には opaque または auto、moderation には auto または low を指定します。",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} と ${secondEndpoint} が記録済みの互換パスです。`,
    fileBody: "ファイルモダリティはカタログに記載されています。追加のメディア形式を推測しないでください。",
    knowledgeBody: "コード、文書、調査の例は編集上の用途であり、ベンチマーク結果ではありません。",
    routeBody: (endpoint) => `${endpoint}。`,
    distillableBody: "distillable というメタデータは、オープンソース、ダウンロード可能、ローカル実行可能であることを意味しません。",
    resolutionBody: "768P または 2K を選択します。",
    durationBody: "4–15 秒のリクエストを設定します。",
    ratioBody: "対応する固定比率または adaptive を使用します。",
    watermarkBody: "AIGC 透かしの boolean を明示的に設定します。",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `カタログのメタデータには ${modalities} が記載されています。`,
    apiCompatibilityDetail: "互換リクエスト項目を使い、未確認のコンシューマー向け機能を推測しないでください。",
    apiImageControlsDetail: "n、size、quality、format、background、moderation。",
    apiAccessDetail: "通常の Flatkey アカウントと API キーフローを使用します。無料または登録不要の生成は約束されません。",
    apiBoundaryDetail: "互換ルートはローカルまたはオープンソース対応を証明しません。",
    apiResultDetail: "返された task ID を content エンドポイントで使用します。",
    costAnswer: (name, rates) => `${name} の現在のカタログ料金は ${rates} です。本番トラフィックの前に日付付き料金欄を確認してください。`,
    contextAnswer: (context, modalities) => `カタログには ${context} と ${modalities} が記録されています。追加のモダリティは推測していません。`,
    releaseAnswer: (date) => `カタログスナップショットのリリース日は ${date} です。独立した発表日ではなく、カタログメタデータとして扱ってください。`,
    imageAccessAnswer: "リクエストを設定し、通常の Flatkey アカウントと API キーフローを使用します。登録不要の生成は約束されません。",
    imageEndpointAnswer: (id, endpoint) => `${endpoint} にモデル ID ${id} と API セクションの対応項目を指定します。`,
    imageCostAnswer: (rates) => `カタログの料金は 100 万単位あたり ${rates} です。画像 1 枚の固定料金ではありません。`,
    imageSizeAnswer: "サイズは 1024x1024、1536x1024、1024x1536、auto。形式は PNG、JPEG、WebP、品質は auto/high/medium/low です。",
    transparentAnswer: "確認済みの項目は background: opaque | auto です。透明出力は約束していません。",
    freeSignupAnswer: "無料または登録不要の生成は確認されていません。通常の Flatkey アクセスフローと日付付き料金欄を確認してください。",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Flatkey API キーを作成し、クライアントに応じて ${endpoint} または ${secondEndpoint} を選び、model を ${id} に設定します。`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `確認済みモデル ID は ${id}。文書化された互換ルートは ${endpoint} と ${secondEndpoint} です。`,
    kimiCostAnswer: (rates) => `現在のカタログ料金は 100 万トークンあたり ${rates} です。`,
    kimiFreeAnswer: "現在のカタログには有料トークン料金が表示されています。無料アクセスを約束せず、アカウントと料金画面を確認してください。",
    kimiLocalAnswer: "確認済みカタログでは、ダウンロード可能な重み、ローカル実行、ハードウェア/VRAM 要件を確認できません。メタデータから推測しないでください。",
    kimiVendorAnswer: "カタログのプロバイダーは Moonshot AI です。ルーティングと請求は Flatkey が提供します。",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `${endpoint} または ${secondEndpoint} にリクエストを送り、モデル ID ${id} と選択した互換クライアントの形式を使います。`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `確認済みモデル ID は ${id}。文書化されたパスは ${endpoint} と ${secondEndpoint} です。`,
    deepseekCostAnswer: "料金は UTC 時間帯で変わります。ピークの入力/キャッシュ読み取り/出力は $1.32/$0.044/$3.96、オフピークは $0.66/$0.022/$1.98（100 万トークンあたり）です。",
    deepseekLocalAnswer: "現在のカタログではダウンロード可能な重みやローカル実行を確認できません。distillable のメタデータから推測しないでください。",
    deepseekVisionAnswer: "確認済みなのはテキストとファイルモダリティのみです。権威ある資料が追加されるまで、vision や広いマルチモーダル対応を主張しないでください。",
    deepseekQualityAnswer: "このページではベンチマークや品質ランキングを公開していません。モデル ID、endpoint、コンテキスト、モダリティ、料金項目を比較してください。",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 は Flatkey の ${endpoint} フローで利用できる動画モデルです。`,
    minimaxUseAnswer: "非同期リクエストから返された task ID を保存し、準備ができたら結果を取得します。",
    minimaxCostAnswer: "現在の公開カタログには 1 秒あたり $0.08 の基本値が記載されています。解像度、参照動画の秒数、入力画像の枠は別表示なので、最新の見積もりを確認してください。",
    minimaxFieldsAnswer: "コンソールを開く前に、解像度、長さ、比率、AIGC 透かしを設定します。",
    minimaxGenerationAnswer: "公開ページはまず下書き設定を保存します。登録後にコンソールを開き、API キーで実行してください。",
    minimaxLocalAnswer: "カタログ記録では ComfyUI、ローカル重み、その他のデプロイ方法を確認できません。検索スニペットから推測しないでください。",
  },
  vi: {
    apiPricingDetails: "API, giá và chi tiết mô hình",
    apiPricingAndModel: "API và giá",
    imageApiPricing: "API và tạo ảnh",
    videoApiPricing: "API và giá trình tạo video",
    pricing: "giá",
    pricingByToken: "giá theo đơn vị token",
    pricingByTokenAndDimensions: "giá theo token và kích thước ảnh",
    pricingByUtc: "giá theo khung giờ UTC",
    pricingCurrent: "Mức giá hiện tại trong danh mục Flatkey",
    pricingNoteToken: "Mức giá tính trên 1 triệu token; xem khối danh mục có ngày cập nhật để biết giá hiện tại.",
    pricingNoteImage: "Danh mục cung cấp giá theo đơn vị token; số tiền cuối cùng phụ thuộc vào trường yêu cầu và mức sử dụng.",
    pricingNoteUtc: "Giữ các đơn vị token giờ cao điểm và ngoài cao điểm cùng nhau; không ngụ ý một mức giá trung bình luôn áp dụng.",
    pricingNoteVideo: "Các dòng hiện tại tính theo giây; hãy xem riêng mức giá cho video tham chiếu và ảnh đầu vào.",
    capabilities: "khả năng",
    contextModalitiesEndpoint: "ngữ cảnh, modality và endpoint",
    imageControls: "kích thước, chất lượng, định dạng và điều khiển",
    fileEndpoints: "ngữ cảnh, đầu vào tệp và endpoint tương thích",
    videoControls: "độ phân giải, thời lượng, tỷ lệ và watermark",
    documentedFields: "Các trường tích hợp được ghi nhận hiển thị bên dưới; không suy diễn cam kết benchmark hay chất lượng.",
    compareFields: "So sánh các trường đã ghi nhận",
    hostedApiFacts: "Thông tin API được lưu trữ",
    migrationFields: "Trường di chuyển",
    videoFields: "Điều khiển video",
    api: "API",
    callWithApi: "API cho",
    imageApiSetup: "endpoint API và thiết lập yêu cầu",
    textApiSetup: "API và ID mô hình",
    videoApiSetup: "thiết lập API video",
    faqDescription: "Giá, khả năng tương thích, giới hạn và chi tiết yêu cầu riêng của mô hình.",
    unknown: "Chưa biết",
    notAsserted: "Không khẳng định",
    notVerified: "Chưa biết / chưa xác minh",
    perMillionTokens: "trên 1 triệu token",
    perMillionUnits: "trên 1 triệu đơn vị",
    perSecond: "mỗi giây",
    input: "Đầu vào",
    output: "Đầu ra",
    cacheRead: "Đọc bộ nhớ đệm",
    cacheCreation: "Tạo bộ nhớ đệm",
    cache: "Bộ nhớ đệm",
    inputTokens: "Token đầu vào",
    outputTokens: "Token đầu ra",
    cacheTokens: "Token bộ nhớ đệm",
    imageDimensions: "Kích thước ảnh",
    peakUtc: "Cao điểm (UTC)",
    offPeakUtc: "Ngoài cao điểm (UTC)",
    endpoint: "Endpoint",
    modelId: "ID mô hình",
    modalities: "Modality",
    context: "Ngữ cảnh",
    formats: "Định dạng",
    sizes: "Kích thước",
    quality: "Chất lượng",
    controls: "Điều khiển",
    access: "Truy cập",
    boundary: "Phạm vi xác minh",
    hostedEndpoint: "Endpoint lưu trữ",
    inputModality: "Modality đầu vào",
    downloadableWeights: "Trọng số có thể tải",
    localRuntime: "Runtime hoặc phần cứng cục bộ",
    freeAccess: "Truy cập miễn phí",
    resolution: "Độ phân giải",
    duration: "Thời lượng",
    ratio: "Tỷ lệ",
    watermark: "Watermark AIGC",
    fields: "Trường",
    result: "Kết quả",
    whatIs: (name) => `${name} là gì?`,
    howUse: (name) => `Dùng ${name} như thế nào?`,
    apiModelId: (name) => `ID mô hình API của ${name} là gì?`,
    cost: (name) => `${name} có giá bao nhiêu?`,
    contextInputs: (name) => `${name} hỗ trợ cửa sổ ngữ cảnh và đầu vào nào?`,
    release: (name) => `${name} được phát hành khi nào?`,
    accessQuestion: (name) => `Truy cập ${name} bằng cách nào?`,
    endpointQuestion: (name) => `Endpoint API của ${name} là gì?`,
    sizeFormatQuestion: (name) => `${name} hỗ trợ kích thước và định dạng nào?`,
    transparentQuestion: "GPT Image 2 có tạo nền trong suốt không?",
    freeSignupQuestion: (name) => `${name} có miễn phí hoặc dùng được không cần đăng ký không?`,
    freeQuestion: (name) => `${name} có miễn phí không?`,
    localQuestion: (name) => `${name} có mã nguồn mở hoặc chạy cục bộ không?`,
    vendorQuestion: (name) => `Ai cung cấp ${name}?`,
    priceUtcQuestion: (name) => `${name} được tính giá như thế nào?`,
    visionQuestion: (name) => `${name} có hỗ trợ đầu vào hình ảnh hoặc đa phương thức không?`,
    qualityQuestion: (name) => `${name} có tốt hơn V4 Flash cho lập trình không?`,
    settingsQuestion: (name) => `Có thể cấu hình những cài đặt ${name} nào ở đây?`,
    configureQuestion: (name) => `Cấu hình yêu cầu ${name} như thế nào?`,
    generationQuestion: "Thao tác này có bắt đầu tạo thật không?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} là mô hình trong danh mục ${vendor}, có thể dùng qua endpoint ${endpoint} của Flatkey. Danh mục đã xác minh ghi nhận ${facts}. Trang này hiển thị ID mô hình, mức giá hiện tại và đường dẫn yêu cầu trước khi chạy lưu lượng sản xuất.`,
    imageStatement: (name, endpoint) => `${name} là mô hình ảnh của OpenAI trong luồng tạo ảnh Flatkey. Dùng ${endpoint} và cấu hình số lượng, kích thước, chất lượng, định dạng, nền và moderation trước khi gửi.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} là mô hình Moonshot AI trong danh mục, dùng được qua các route chat và messages tương thích của Flatkey. Danh mục xác minh ngữ cảnh ${context} và đầu vào tệp; đường dẫn được ghi nhận là ${endpoint} và ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} là mô hình DeepSeek trong danh mục, được ghi nhận với modality văn bản/tệp, ngữ cảnh ${context} và hai đường dẫn API tương thích: ${endpoint} và ${secondEndpoint}. Giá thay đổi theo giờ UTC.`,
    minimaxStatement: (name, endpoint) => `${name} có thể dùng qua luồng ${endpoint} bất đồng bộ của Flatkey. Cấu hình độ phân giải 768P hoặc 2K, 4–15 giây, tỷ lệ được hỗ trợ và trường watermark AIGC trước khi mở console.`,
    contextBody: (context) => `Danh mục ghi nhận cửa sổ ngữ cảnh ${context}.`,
    modalitiesBody: (modalities) => `Metadata danh mục liệt kê ${modalities}.`,
    endpointBody: (endpoint, id) => `Yêu cầu dùng ${endpoint} với ID mô hình ${id}.`,
    keyBillingBody: "Flatkey cung cấp lớp API key và thanh toán với khả năng tương thích endpoint/yêu cầu.",
    imageCountBody: "n nhận 1–10 trong trình tạo của trang.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536 hoặc auto.",
    imageQualityBody: "auto, high, medium hoặc low.",
    imageFormatsBody: "PNG, JPEG hoặc WebP.",
    imageBackgroundBody: "Trường background nhận opaque hoặc auto; moderation nhận auto hoặc low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} và ${secondEndpoint} là các endpoint tương thích đã ghi nhận.`,
    fileBody: "Modality tệp được liệt kê trong danh mục; không suy rộng thêm loại media.",
    knowledgeBody: "Ví dụ về code, tài liệu và nghiên cứu là use case biên tập, không phải kết quả benchmark.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "Cờ metadata distillable không chứng minh mô hình mã nguồn mở, tải xuống được hoặc chạy cục bộ.",
    resolutionBody: "Chọn 768P hoặc 2K.",
    durationBody: "Cấu hình yêu cầu 4–15 giây.",
    ratioBody: "Dùng tỷ lệ cố định được hỗ trợ hoặc adaptive.",
    watermarkBody: "Đặt rõ giá trị boolean của watermark AIGC.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Metadata danh mục liệt kê ${modalities}.`,
    apiCompatibilityDetail: "Dùng các trường yêu cầu tương thích; không suy đoán tính năng người dùng chưa xác minh.",
    apiImageControlsDetail: "n, size, quality, format, background và moderation.",
    apiAccessDetail: "Dùng luồng tài khoản và API key thông thường của Flatkey; không cam kết tạo miễn phí hoặc không đăng ký.",
    apiBoundaryDetail: "Route tương thích không chứng minh hỗ trợ cục bộ hoặc mã nguồn mở.",
    apiResultDetail: "Dùng task ID trả về với content endpoint.",
    costAnswer: (name, rates) => `Mức giá danh mục hiện tại cho ${name} là ${rates}. Kiểm tra khối giá có ngày trước khi chạy trong môi trường thực tế.`,
    contextAnswer: (context, modalities) => `Danh mục ghi nhận ${context} và ${modalities}. Trang này không suy đoán modality bổ sung.`,
    releaseAnswer: (date) => `Snapshot danh mục ghi nhận ngày phát hành ${date}; đây là metadata danh mục, không phải thông báo ra mắt riêng.`,
    imageAccessAnswer: "Cấu hình yêu cầu rồi dùng luồng tài khoản và API key thông thường của Flatkey. Trang không cam kết tạo không cần đăng ký.",
    imageEndpointAnswer: (id, endpoint) => `Dùng ${endpoint} với ID ${id} và các trường được hỗ trợ trong phần API.`,
    imageCostAnswer: (rates) => `Danh mục ghi nhận ${rates} trên 1 triệu đơn vị; không phải một giá cố định cho mỗi ảnh.`,
    imageSizeAnswer: "Kích thước: 1024x1024, 1536x1024, 1024x1536 và auto. Định dạng: PNG, JPEG, WebP; chất lượng: auto/high/medium/low.",
    transparentAnswer: "Trường đã xác minh là background: opaque | auto; trang này không cam kết đầu ra trong suốt.",
    freeSignupAnswer: "Nghiên cứu không xác minh việc tạo miễn phí hoặc không đăng ký. Dùng luồng Flatkey thông thường và xem khối giá có ngày.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Tạo API key Flatkey, chọn ${endpoint} hoặc ${secondEndpoint} theo client và đặt model là ${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `ID mô hình đã xác minh là ${id}; route tương thích được ghi nhận là ${endpoint} và ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `Mức giá danh mục hiện tại là ${rates} trên 1 triệu token.`,
    kimiFreeAnswer: "Danh mục hiện tại hiển thị mức giá token có tính phí. Không hứa hẹn truy cập miễn phí; hãy kiểm tra tài khoản và giao diện giá.",
    kimiLocalAnswer: "Danh mục đã xác minh không xác nhận trọng số tải xuống, triển khai cục bộ hoặc yêu cầu phần cứng/VRAM. Không suy ra các thuộc tính đó từ metadata.",
    kimiVendorAnswer: "Nhà cung cấp trong danh mục là Moonshot AI; Flatkey cung cấp định tuyến và thanh toán.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Gửi yêu cầu tới ${endpoint} hoặc ${secondEndpoint}, dùng ID ${id} và định dạng của client tương thích đã chọn.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `ID mô hình đã xác minh là ${id}; đường dẫn được ghi nhận là ${endpoint} và ${secondEndpoint}.`,
    deepseekCostAnswer: "Giá theo giờ UTC: cao điểm input/cache-read/output là $1.32/$0.044/$3.96; ngoài cao điểm là $0.66/$0.022/$1.98 trên 1 triệu token.",
    deepseekLocalAnswer: "Danh mục hiện tại không xác minh trọng số tải xuống hoặc triển khai cục bộ. Không suy luận từ cờ metadata distillable.",
    deepseekVisionAnswer: "Chỉ modality văn bản và tệp được xác minh. Không tuyên bố hỗ trợ vision hoặc đa phương thức rộng hơn nếu chưa có tài liệu có thẩm quyền.",
    deepseekQualityAnswer: "Trang này không công bố xếp hạng benchmark hay chất lượng. Hãy so sánh ID, endpoint, ngữ cảnh, modality và trường giá đã ghi nhận.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 là mô hình video có thể dùng qua luồng ${endpoint} của Flatkey.`,
    minimaxUseAnswer: "Lưu task ID từ yêu cầu bất đồng bộ và lấy kết quả khi sẵn sàng.",
    minimaxCostAnswer: "Danh mục công khai hiện tại ghi nhận mức cơ bản $0.08 mỗi giây. Độ phân giải, số giây video tham chiếu và hạn mức ảnh đầu vào được tách riêng trong ước tính trực tiếp.",
    minimaxFieldsAnswer: "Cấu hình độ phân giải, thời lượng, tỷ lệ và watermark AIGC trước khi mở console.",
    minimaxGenerationAnswer: "Trang công khai lưu bản nháp trước. Đăng ký hoặc mở console để chạy yêu cầu bằng API key.",
    minimaxLocalAnswer: "Bản ghi danh mục không xác minh ComfyUI, trọng số cục bộ hay cách triển khai khác; không suy ra từ đoạn trích tìm kiếm.",
  },
  de: {
    apiPricingDetails: "API, Preise und Modelldetails",
    apiPricingAndModel: "API und Preise",
    imageApiPricing: "API und Bildgenerierung",
    videoApiPricing: "Video-Generator-API und Preise",
    pricing: "Preise",
    pricingByToken: "Preise nach Token-Dimension",
    pricingByTokenAndDimensions: "Preise nach Token und Bildabmessungen",
    pricingByUtc: "Preise nach UTC-Zeittarif",
    pricingCurrent: "Aktuelle Flatkey-Katalogpreise",
    pricingNoteToken: "Die Preise gelten pro 1 Mio. Tokens; aktuelle Werte stehen im datierten Katalogblock.",
    pricingNoteImage: "Der Katalog weist Token-Dimensionspreise aus; der Endbetrag hängt von Anfrage- und Nutzungsfeldern ab.",
    pricingNoteUtc: "Führe Token-Dimensionen für Spitzen- und Nebenzeiten gemeinsam auf; ein dauerhaft gemittelter Preis ist nicht gemeint.",
    pricingNoteVideo: "Die aktuellen Katalogzeilen gelten pro Sekunde; Referenzvideo und Eingabebild separat prüfen.",
    capabilities: "Funktionen",
    contextModalitiesEndpoint: "Kontext, Modalitäten und Endpoint",
    imageControls: "Größen, Qualität, Formate und Einstellungen",
    fileEndpoints: "Kontext, Dateieingabe und kompatible Endpoints",
    videoControls: "Auflösung, Dauer, Seitenverhältnis und Wasserzeichen",
    documentedFields: "Unten stehen dokumentierte Integrationsfelder; Benchmark- oder Qualitätsversprechen werden nicht abgeleitet.",
    compareFields: "Dokumentierte Felder vergleichen",
    hostedApiFacts: "Fakten zur gehosteten API",
    migrationFields: "Migrationsfelder",
    videoFields: "Videoeinstellungen",
    api: "API",
    callWithApi: "API-Zugriff für",
    imageApiSetup: "API-Endpoint und Anfrageeinrichtung",
    textApiSetup: "API und Modell-ID",
    videoApiSetup: "Video-API einrichten",
    faqDescription: "Preise, Kompatibilität, Limits und modellspezifische Anfrageinformationen.",
    unknown: "Unbekannt",
    notAsserted: "Nicht behauptet",
    notVerified: "Unbekannt / nicht verifiziert",
    perMillionTokens: "pro 1 Mio. Tokens",
    perMillionUnits: "pro 1 Mio. Einheiten",
    perSecond: "pro Sekunde",
    input: "Eingabe",
    output: "Ausgabe",
    cacheRead: "Cache-Lesen",
    cacheCreation: "Cache-Erstellung",
    cache: "Cache",
    inputTokens: "Eingabe-Tokens",
    outputTokens: "Ausgabe-Tokens",
    cacheTokens: "Cache-Tokens",
    imageDimensions: "Bildabmessungen",
    peakUtc: "Spitzenzeit (UTC)",
    offPeakUtc: "Nebenzeit (UTC)",
    endpoint: "Endpoint",
    modelId: "Modell-ID",
    modalities: "Modalitäten",
    context: "Kontext",
    formats: "Formate",
    sizes: "Größen",
    quality: "Qualität",
    controls: "Einstellungen",
    access: "Zugriff",
    boundary: "Verifizierungsgrenze",
    hostedEndpoint: "Gehosteter Endpoint",
    inputModality: "Eingabemodalität",
    downloadableWeights: "Herunterladbare Gewichte",
    localRuntime: "Lokale Laufzeit oder Hardware",
    freeAccess: "Kostenloser Zugriff",
    resolution: "Auflösung",
    duration: "Dauer",
    ratio: "Seitenverhältnis",
    watermark: "AIGC-Wasserzeichen",
    fields: "Felder",
    result: "Ergebnis",
    whatIs: (name) => `Was ist ${name}?`,
    howUse: (name) => `Wie verwende ich ${name}?`,
    apiModelId: (name) => `Wie lautet die API-Modell-ID von ${name}?`,
    cost: (name) => `Was kostet ${name}?`,
    contextInputs: (name) => `Welches Kontextfenster und welche Eingaben unterstützt ${name}?`,
    release: (name) => `Wann wurde ${name} veröffentlicht?`,
    accessQuestion: (name) => `Wie erhalte ich Zugriff auf ${name}?`,
    endpointQuestion: (name) => `Welcher API-Endpoint gehört zu ${name}?`,
    sizeFormatQuestion: (name) => `Welche Größen und Formate unterstützt ${name}?`,
    transparentQuestion: "Kann GPT Image 2 transparente Hintergründe erzeugen?",
    freeSignupQuestion: (name) => `Ist ${name} kostenlos oder ohne Anmeldung verfügbar?`,
    freeQuestion: (name) => `Ist ${name} kostenlos?`,
    localQuestion: (name) => `Ist ${name} Open Source oder lokal verfügbar?`,
    vendorQuestion: (name) => `Wer stellt ${name} bereit?`,
    priceUtcQuestion: (name) => `Wie wird ${name} abgerechnet?`,
    visionQuestion: (name) => `Unterstützt ${name} Vision oder multimodale Eingaben?`,
    qualityQuestion: (name) => `Ist ${name} besser fürs Programmieren als V4 Flash?`,
    settingsQuestion: (name) => `Welche ${name}-Einstellungen kann ich hier konfigurieren?`,
    configureQuestion: (name) => `Wie konfiguriere ich eine ${name}-Anfrage?`,
    generationQuestion: "Startet dies eine echte Generierung?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} ist ein Katalogmodell von ${vendor}, verfügbar über den Flatkey-Endpoint ${endpoint}. Der verifizierte Katalog nennt ${facts}. Diese Seite zeigt Modell-ID, aktuelle Preise und Anfragepfad vor Produktionsverkehr.`,
    imageStatement: (name, endpoint) => `${name} ist das OpenAI-Bildmodell im Flatkey-Bildgenerierungsfluss. Verwende ${endpoint} und konfiguriere Anzahl, Größe, Qualität, Format, Hintergrund und Moderation vor dem Senden.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} ist das Moonshot-AI-Katalogmodell, verfügbar über kompatible Chat- und Messages-Routen von Flatkey. Der verifizierte Katalog nennt einen ${context}-Kontext und Dateieingabe; dokumentierte Pfade sind ${endpoint} und ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} ist das DeepSeek-Katalogmodell mit dokumentierten Text-/Dateimodalitäten, ${context} Kontext und zwei kompatiblen API-Pfaden: ${endpoint} und ${secondEndpoint}. Der Preis richtet sich nach der UTC-Zeit.`,
    minimaxStatement: (name, endpoint) => `${name} ist über den asynchronen Flatkey-Fluss ${endpoint} verfügbar. Konfiguriere 768P oder 2K, 4–15 Sekunden, ein unterstütztes Verhältnis und das AIGC-Wasserzeichen, bevor du die Konsole öffnest.`,
    contextBody: (context) => `Der Katalog nennt ein Kontextfenster von ${context}.`,
    modalitiesBody: (modalities) => `Die Katalogmetadaten führen ${modalities} auf.`,
    endpointBody: (endpoint, id) => `Anfragen verwenden ${endpoint} mit der Modell-ID ${id}.`,
    keyBillingBody: "Flatkey stellt API-Key- und Abrechnungsebene mit Endpoint-/Anfragekompatibilität bereit.",
    imageCountBody: "n akzeptiert im Seitengenerator 1–10.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536 oder auto.",
    imageQualityBody: "auto, high, medium oder low.",
    imageFormatsBody: "PNG, JPEG oder WebP.",
    imageBackgroundBody: "Das Feld background akzeptiert opaque oder auto; moderation akzeptiert auto oder low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} und ${secondEndpoint} sind die registrierten kompatiblen Endpoints.`,
    fileBody: "Dateimodalität ist im Katalog aufgeführt; leite keine weiteren Medientypen ab.",
    knowledgeBody: "Code-, Dokument- und Recherchebeispiele sind redaktionelle Anwendungsfälle, keine Benchmark-Ergebnisse.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "Ein distillable-Metadatenflag beweist weder Open Source noch herunterladbare Gewichte oder lokale Ausführung.",
    resolutionBody: "Wähle 768P oder 2K.",
    durationBody: "Konfiguriere eine 4–15-Sekunden-Anfrage.",
    ratioBody: "Verwende ein unterstütztes festes Verhältnis oder adaptive.",
    watermarkBody: "Setze das AIGC-Wasserzeichen-Boolean ausdrücklich.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Die Katalogmetadaten führen ${modalities} auf.`,
    apiCompatibilityDetail: "Verwende kompatible Anfragefelder; leite keine nicht verifizierten Verbraucherfunktionen ab.",
    apiImageControlsDetail: "n, size, quality, format, background und moderation.",
    apiAccessDetail: "Nutze den normalen Flatkey-Konto- und API-Key-Fluss; kostenlose oder anmeldefreie Generierung wird nicht versprochen.",
    apiBoundaryDetail: "Kompatible Routen beweisen keine lokale oder Open-Source-Unterstützung.",
    apiResultDetail: "Verwende die zurückgegebene Task-ID mit dem Content-Endpoint.",
    costAnswer: (name, rates) => `Die aktuellen Katalogtarife für ${name} sind ${rates}. Prüfe den datierten Preisblock vor Produktionsverkehr.`,
    contextAnswer: (context, modalities) => `Der Katalog nennt ${context} und ${modalities}. Weitere Modalitäten werden hier nicht abgeleitet.`,
    releaseAnswer: (date) => `Der Katalog-Snapshot nennt ${date} als Veröffentlichungsdatum; dies ist Katalogmetadatum, keine separate Launch-Ankündigung.`,
    imageAccessAnswer: "Konfiguriere eine Anfrage und nutze den normalen Flatkey-Konto- und API-Key-Fluss. Die Seite verspricht keine Generierung ohne Anmeldung.",
    imageEndpointAnswer: (id, endpoint) => `Verwende ${endpoint} mit Modell-ID ${id} und den im API-Abschnitt genannten Feldern.`,
    imageCostAnswer: (rates) => `Der Katalog nennt ${rates} pro 1 Mio. Einheiten; es ist kein einheitlicher Preis pro Bild.`,
    imageSizeAnswer: "Größen: 1024x1024, 1536x1024, 1024x1536 und auto. Formate: PNG, JPEG, WebP; Qualität: auto/high/medium/low.",
    transparentAnswer: "Das verifizierte Feld ist background: opaque | auto; transparente Ausgabe wird hier nicht versprochen.",
    freeSignupAnswer: "Kostenlose oder anmeldefreie Generierung ist nicht verifiziert. Nutze den normalen Flatkey-Zugriff und den datierten Preisblock.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Erstelle einen Flatkey-API-Key, wähle ${endpoint} oder ${secondEndpoint} gemäß deinem Client und setze model auf ${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `Die verifizierte Modell-ID lautet ${id}; dokumentierte kompatible Routen sind ${endpoint} und ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `Aktuelle Katalogtarife: ${rates} pro 1 Mio. Tokens.`,
    kimiFreeAnswer: "Der aktuelle Katalog zeigt kostenpflichtige Token-Tarife. Versprich keinen kostenlosen Zugriff; prüfe Konto und Preisoberfläche.",
    kimiLocalAnswer: "Der verifizierte Katalog bestätigt weder herunterladbare Gewichte noch lokale Bereitstellung oder Hardware/VRAM-Anforderungen. Leite dies nicht aus Metadaten ab.",
    kimiVendorAnswer: "Der Kataloganbieter ist Moonshot AI; Flatkey übernimmt Routing und Abrechnung.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Sende eine Anfrage an ${endpoint} oder ${secondEndpoint}, nutze Modell-ID ${id} und das Format des gewählten kompatiblen Clients.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `Die verifizierte Modell-ID lautet ${id}; dokumentierte Pfade sind ${endpoint} und ${secondEndpoint}.`,
    deepseekCostAnswer: "Die Preise hängen von UTC ab: Spitzenzeit input/cache-read/output $1.32/$0.044/$3.96, Nebenzeit $0.66/$0.022/$1.98 pro 1 Mio. Tokens.",
    deepseekLocalAnswer: "Der aktuelle Katalog verifiziert keine herunterladbaren Gewichte oder lokale Bereitstellung. Leite dies nicht aus dem distillable-Flag ab.",
    deepseekVisionAnswer: "Verifiziert sind nur Text- und Dateimodalitäten. Behaupte keine Vision- oder breitere Multimodalität ohne autorisierte Dokumentation.",
    deepseekQualityAnswer: "Diese Seite veröffentlicht kein Benchmark- oder Qualitätsranking. Vergleiche dokumentierte IDs, Endpoints, Kontext, Modalitäten und Preisfelder.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 ist ein Videomodell über den Flatkey-Fluss ${endpoint}.`,
    minimaxUseAnswer: "Speichere die Task-ID der asynchronen Anfrage und rufe das Ergebnis ab, sobald es bereit ist.",
    minimaxCostAnswer: "Der aktuelle öffentliche Katalog nennt einen Basiswert von $0.08 pro Sekunde. Auflösung, Referenzvideo-Sekunden und das Kontingent für Eingabebilder werden in der Live-Schätzung getrennt ausgewiesen.",
    minimaxFieldsAnswer: "Konfiguriere Auflösung, Dauer, Seitenverhältnis und AIGC-Wasserzeichen vor dem Öffnen der Konsole.",
    minimaxGenerationAnswer: "Die öffentliche Seite speichert zuerst den Entwurf. Melde dich an oder öffne die Konsole, um die Anfrage mit API-Key auszuführen.",
    minimaxLocalAnswer: "Der Katalogeintrag verifiziert ComfyUI, lokale Gewichte oder andere Deployments nicht; leite sie nicht aus Such-Snippets ab.",
  },
  id: {
    apiPricingDetails: "API, harga, dan detail model",
    apiPricingAndModel: "API dan harga",
    imageApiPricing: "API dan pembuatan gambar",
    videoApiPricing: "API dan harga generator video",
    pricing: "harga",
    pricingByToken: "harga berdasarkan dimensi token",
    pricingByTokenAndDimensions: "harga berdasarkan token dan dimensi gambar",
    pricingByUtc: "harga berdasarkan zona waktu UTC",
    pricingCurrent: "Harga katalog Flatkey saat ini",
    pricingNoteToken: "Tarif dihitung per 1 juta token; lihat blok katalog bertanggal untuk nilai terbaru.",
    pricingNoteImage: "Katalog menampilkan harga berdasarkan dimensi token; jumlah akhir bergantung pada bidang permintaan dan penggunaan.",
    pricingNoteUtc: "Tampilkan dimensi token jam sibuk dan di luar jam sibuk bersama-sama; tidak ada harga rata-rata yang selalu berlaku.",
    pricingNoteVideo: "Baris katalog saat ini dihitung per detik; periksa tarif video referensi dan gambar input secara terpisah.",
    capabilities: "kemampuan",
    contextModalitiesEndpoint: "konteks, modalitas, dan endpoint",
    imageControls: "ukuran, kualitas, format, dan kontrol",
    fileEndpoints: "konteks, input file, dan endpoint yang kompatibel",
    videoControls: "resolusi, durasi, rasio, dan watermark",
    documentedFields: "Bidang integrasi yang terdokumentasi ditampilkan di bawah; tidak ada klaim benchmark atau kualitas yang disimpulkan.",
    compareFields: "Bandingkan bidang terdokumentasi",
    hostedApiFacts: "Fakta API ter-host",
    migrationFields: "Bidang migrasi",
    videoFields: "Kontrol video",
    api: "API",
    callWithApi: "Akses API untuk",
    imageApiSetup: "endpoint API dan pengaturan permintaan",
    textApiSetup: "API dan ID model",
    videoApiSetup: "pengaturan API video",
    faqDescription: "Harga, kompatibilitas, batas, dan detail permintaan khusus model.",
    unknown: "Tidak diketahui",
    notAsserted: "Tidak diklaim",
    notVerified: "Tidak diketahui / belum diverifikasi",
    perMillionTokens: "per 1 juta token",
    perMillionUnits: "per 1 juta unit",
    perSecond: "per detik",
    input: "Input",
    output: "Output",
    cacheRead: "Baca cache",
    cacheCreation: "Pembuatan cache",
    cache: "Cache",
    inputTokens: "Token input",
    outputTokens: "Token output",
    cacheTokens: "Token cache",
    imageDimensions: "Dimensi gambar",
    peakUtc: "Puncak (UTC)",
    offPeakUtc: "Di luar puncak (UTC)",
    endpoint: "Endpoint",
    modelId: "ID model",
    modalities: "Modalitas",
    context: "Konteks",
    formats: "Format",
    sizes: "Ukuran",
    quality: "Kualitas",
    controls: "Kontrol",
    access: "Akses",
    boundary: "Batas verifikasi",
    hostedEndpoint: "Endpoint ter-host",
    inputModality: "Modalitas input",
    downloadableWeights: "Bobot yang dapat diunduh",
    localRuntime: "Runtime atau perangkat keras lokal",
    freeAccess: "Akses gratis",
    resolution: "Resolusi",
    duration: "Durasi",
    ratio: "Rasio",
    watermark: "Watermark AIGC",
    fields: "Bidang",
    result: "Hasil",
    whatIs: (name) => `Apa itu ${name}?`,
    howUse: (name) => `Bagaimana cara menggunakan ${name}?`,
    apiModelId: (name) => `Apa ID model API ${name}?`,
    cost: (name) => `Berapa biaya ${name}?`,
    contextInputs: (name) => `Jendela konteks dan input apa yang didukung ${name}?`,
    release: (name) => `Kapan ${name} dirilis?`,
    accessQuestion: (name) => `Bagaimana cara mengakses ${name}?`,
    endpointQuestion: (name) => `Apa endpoint API untuk ${name}?`,
    sizeFormatQuestion: (name) => `Ukuran dan format apa yang didukung ${name}?`,
    transparentQuestion: "Apakah GPT Image 2 dapat membuat latar transparan?",
    freeSignupQuestion: (name) => `Apakah ${name} gratis atau tersedia tanpa pendaftaran?`,
    freeQuestion: (name) => `Apakah ${name} gratis?`,
    localQuestion: (name) => `Apakah ${name} open source atau tersedia secara lokal?`,
    vendorQuestion: (name) => `Siapa pembuat ${name}?`,
    priceUtcQuestion: (name) => `Bagaimana harga ${name} dihitung?`,
    visionQuestion: (name) => `Apakah ${name} mendukung input visual atau multimodal?`,
    qualityQuestion: (name) => `Apakah ${name} lebih baik untuk coding daripada V4 Flash?`,
    settingsQuestion: (name) => `Pengaturan ${name} apa yang dapat dikonfigurasi di sini?`,
    configureQuestion: (name) => `Bagaimana mengonfigurasi permintaan ${name}?`,
    generationQuestion: "Apakah ini memulai pembuatan yang sebenarnya?",
    modelStatement: (name, vendor, endpoint, facts) => `${name} adalah model katalog ${vendor} yang tersedia melalui endpoint ${endpoint} Flatkey. Katalog terverifikasi mencatat ${facts}. Halaman ini menampilkan ID model, tarif saat ini, dan jalur permintaan sebelum trafik produksi.`,
    imageStatement: (name, endpoint) => `${name} adalah model gambar OpenAI dalam alur pembuatan gambar Flatkey. Gunakan ${endpoint} dan atur jumlah, ukuran, kualitas, format, latar, serta moderasi sebelum mengirim permintaan.`,
    kimiStatement: (name, endpoint, secondEndpoint, context) => `${name} adalah model katalog Moonshot AI yang tersedia melalui rute chat dan messages kompatibel Flatkey. Katalog terverifikasi mencatat konteks ${context} dan input file; jalur yang terdokumentasi adalah ${endpoint} dan ${secondEndpoint}.`,
    deepseekStatement: (name, endpoint, secondEndpoint, context) => `${name} adalah model katalog DeepSeek dengan modalitas teks dan file, konteks ${context}, serta dua jalur API kompatibel: ${endpoint} dan ${secondEndpoint}. Harga berubah berdasarkan waktu UTC.`,
    minimaxStatement: (name, endpoint) => `${name} tersedia melalui alur asinkron ${endpoint} Flatkey. Atur resolusi 768P atau 2K, 4–15 detik, rasio yang didukung, dan bidang watermark AIGC sebelum membuka konsol.`,
    contextBody: (context) => `Katalog mencatat jendela konteks ${context}.`,
    modalitiesBody: (modalities) => `Metadata katalog mencantumkan ${modalities}.`,
    endpointBody: (endpoint, id) => `Permintaan menggunakan ${endpoint} dengan ID model ${id}.`,
    keyBillingBody: "Flatkey menyediakan lapisan API key dan penagihan dengan kompatibilitas endpoint dan permintaan.",
    imageCountBody: "n menerima 1–10 pada generator halaman.",
    imageSizesBody: "1024x1024, 1536x1024, 1024x1536, atau auto.",
    imageQualityBody: "auto, high, medium, atau low.",
    imageFormatsBody: "PNG, JPEG, atau WebP.",
    imageBackgroundBody: "Bidang background menerima opaque atau auto; moderation menerima auto atau low.",
    twoRoutesBody: (endpoint, secondEndpoint) => `${endpoint} dan ${secondEndpoint} adalah endpoint kompatibel yang tercatat.`,
    fileBody: "Modalitas file tercantum di katalog; jangan menyimpulkan jenis media tambahan.",
    knowledgeBody: "Contoh coding, dokumen, dan riset adalah use case editorial, bukan hasil benchmark.",
    routeBody: (endpoint) => `${endpoint}.`,
    distillableBody: "Flag metadata distillable tidak membuktikan bahwa model open source, dapat diunduh, atau dapat dijalankan lokal.",
    resolutionBody: "Pilih 768P atau 2K.",
    durationBody: "Konfigurasikan permintaan 4–15 detik.",
    ratioBody: "Gunakan rasio tetap yang didukung atau adaptive.",
    watermarkBody: "Tetapkan boolean watermark AIGC secara eksplisit.",
    apiEndpointDetail: (endpoint) => `POST ${endpoint}`,
    apiModelDetail: (id) => id,
    apiModalitiesDetail: (modalities) => `Metadata katalog mencantumkan ${modalities}.`,
    apiCompatibilityDetail: "Gunakan bidang permintaan yang kompatibel; jangan menyimpulkan fitur konsumen yang belum diverifikasi.",
    apiImageControlsDetail: "n, size, quality, format, background, dan moderation.",
    apiAccessDetail: "Gunakan alur akun dan API key Flatkey biasa; tidak ada janji pembuatan gratis atau tanpa pendaftaran.",
    apiBoundaryDetail: "Rute kompatibel tidak membuktikan dukungan lokal atau open source.",
    apiResultDetail: "Gunakan task ID yang dikembalikan dengan endpoint konten.",
    costAnswer: (name, rates) => `Tarif katalog saat ini untuk ${name} adalah ${rates}. Periksa blok harga bertanggal sebelum trafik produksi.`,
    contextAnswer: (context, modalities) => `Katalog mencatat ${context} dan ${modalities}. Halaman ini tidak menyimpulkan modalitas tambahan.`,
    releaseAnswer: (date) => `Snapshot katalog mencatat ${date} sebagai tanggal rilis; ini adalah metadata katalog, bukan pengumuman peluncuran terpisah.`,
    imageAccessAnswer: "Konfigurasikan permintaan, lalu gunakan alur akun dan API key Flatkey biasa. Halaman ini tidak menjanjikan pembuatan tanpa pendaftaran.",
    imageEndpointAnswer: (id, endpoint) => `Gunakan ${endpoint} dengan ID ${id} dan bidang yang didukung di bagian API.`,
    imageCostAnswer: (rates) => `Katalog mencantumkan ${rates} per 1 juta unit; ini bukan harga tetap per gambar.`,
    imageSizeAnswer: "Ukuran: 1024x1024, 1536x1024, 1024x1536, auto. Format: PNG, JPEG, WebP; kualitas: auto/high/medium/low.",
    transparentAnswer: "Bidang terverifikasi adalah background: opaque | auto; halaman ini tidak menjanjikan output transparan.",
    freeSignupAnswer: "Riset tidak memverifikasi pembuatan gratis atau tanpa pendaftaran. Gunakan alur Flatkey biasa dan periksa blok harga bertanggal.",
    kimiUseAnswer: (id, endpoint, secondEndpoint) => `Buat API key Flatkey, pilih ${endpoint} atau ${secondEndpoint} sesuai klien, lalu tetapkan model ke ${id}.`,
    kimiIdAnswer: (id, endpoint, secondEndpoint) => `ID model terverifikasi adalah ${id}; rute kompatibel yang terdokumentasi adalah ${endpoint} dan ${secondEndpoint}.`,
    kimiCostAnswer: (rates) => `Tarif katalog saat ini adalah ${rates} per 1 juta token.`,
    kimiFreeAnswer: "Katalog saat ini menunjukkan tarif token berbayar. Jangan menjanjikan akses gratis; periksa akun dan antarmuka harga.",
    kimiLocalAnswer: "Katalog terverifikasi tidak mengonfirmasi bobot yang dapat diunduh, deployment lokal, atau kebutuhan hardware/VRAM. Jangan menyimpulkannya dari metadata.",
    kimiVendorAnswer: "Vendor katalog adalah Moonshot AI; Flatkey menyediakan routing dan penagihan.",
    deepseekUseAnswer: (id, endpoint, secondEndpoint) => `Kirim permintaan ke ${endpoint} atau ${secondEndpoint}, gunakan ID ${id}, dan ikuti format klien kompatibel yang dipilih.`,
    deepseekIdAnswer: (id, endpoint, secondEndpoint) => `ID model terverifikasi adalah ${id}; jalur terdokumentasi adalah ${endpoint} dan ${secondEndpoint}.`,
    deepseekCostAnswer: "Harga bertingkat menurut waktu UTC: puncak input/cache-read/output $1.32/$0.044/$3.96; di luar puncak $0.66/$0.022/$1.98 per 1 juta token.",
    deepseekLocalAnswer: "Katalog saat ini tidak memverifikasi bobot yang dapat diunduh atau deployment lokal. Jangan menyimpulkan dari flag distillable.",
    deepseekVisionAnswer: "Hanya modalitas teks dan file yang terverifikasi. Jangan mengklaim dukungan vision atau multimodal yang lebih luas tanpa dokumen berwenang.",
    deepseekQualityAnswer: "Halaman ini tidak menerbitkan peringkat benchmark atau kualitas. Bandingkan ID, endpoint, konteks, modalitas, dan bidang harga yang terdokumentasi.",
    minimaxWhatAnswer: (endpoint) => `MiniMax-H3 adalah model video yang tersedia melalui alur ${endpoint} Flatkey.`,
    minimaxUseAnswer: "Simpan task ID dari permintaan asinkron dan ambil hasil saat siap.",
    minimaxCostAnswer: "Katalog publik saat ini mencantumkan nilai dasar $0.08 per detik. Resolusi, detik video referensi, dan kuota gambar input ditampilkan terpisah dalam estimasi langsung.",
    minimaxFieldsAnswer: "Atur resolusi, durasi, rasio, dan watermark AIGC sebelum membuka konsol.",
    minimaxGenerationAnswer: "Halaman publik menyimpan draf terlebih dahulu. Daftar atau buka konsol untuk menjalankan permintaan dengan API key.",
    minimaxLocalAnswer: "Catatan katalog tidak memverifikasi ComfyUI, bobot lokal, atau deployment lain; jangan menyimpulkannya dari cuplikan pencarian.",
  },
};

/**
 * A few field values are deliberately shared by the model builders (for
 * example, the two compatible providers or the comparison baseline).  Keep
 * those labels localized as well instead of letting a sentence translate
 * while a table heading silently remains English.
 */
const LITERAL_LABELS: Partial<Record<Locale, Record<string, string>>> = {
  zh: {
    "Knowledge work": "知识工作",
    "OpenAI-compatible": "OpenAI 兼容",
    "Anthropic-compatible": "Anthropic 兼容",
    "Distillable metadata": "Distillable 元数据",
    "Relative quality/benchmark": "相对质量/基准测试",
    "Local / open-source assumptions": "本地/开源假设",
    "Text, image, file": "文本、图片、文件",
    "Text/file fields": "文本/文件字段",
    "Not promised; paid token rates apply": "未承诺；按 token 付费",
    "Verify before publishing": "发布前核实",
    "Benchmark/coding ranking": "基准/编程排名",
    Compatibility: "兼容性",
    "Use the exact model ID and documented endpoint in your existing client.": "在现有客户端中使用准确的模型 ID 和文档化 endpoint。",
    "Background / moderation": "背景 / moderation",
    "Quality/background/moderation": "质量/背景/moderation",
    "Image quality ranking": "图像质量排名",
    "Documented above": "见上方文档",
    "Default": "默认",
    "6 seconds": "6 秒",
    "Explicit boolean": "明确的布尔值",
    "output fields": "输出字段",
    "resolution, duration, ratio, and aigc_watermark.": "resolution、duration、ratio 和 aigc_watermark。",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 是 OpenAI 的图像模型，可通过 /v1/images/generations 使用。",
  },
  es: {
    "Knowledge work": "Trabajo de conocimiento",
    "OpenAI-compatible": "Compatible con OpenAI",
    "Anthropic-compatible": "Compatible con Anthropic",
    "Distillable metadata": "Metadatos distillable",
    "Relative quality/benchmark": "Calidad/benchmark relativo",
    "Local / open-source assumptions": "Suposiciones locales/open source",
    "Text, image, file": "Texto, imagen y archivo",
    "Text/file fields": "Campos de texto/archivo",
    "Not promised; paid token rates apply": "No prometido; se aplican tarifas por token",
    "Verify before publishing": "Verificar antes de publicar",
    "Benchmark/coding ranking": "Ranking de benchmark/código",
    Compatibility: "Compatibilidad",
    "Use the exact model ID and documented endpoint in your existing client.": "Usa el ID exacto del modelo y el endpoint documentado en tu cliente actual.",
    "Background / moderation": "Fondo / moderación",
    "Quality/background/moderation": "Calidad/fondo/moderación",
    "Image quality ranking": "Ranking de calidad de imagen",
    "Documented above": "Documentado arriba",
    "Default": "Predeterminado",
    "6 seconds": "6 segundos",
    "Explicit boolean": "Booleano explícito",
    "output fields": "campos de salida",
    "resolution, duration, ratio, and aigc_watermark.": "resolution, duration, ratio y aigc_watermark.",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 es el modelo de imágenes de OpenAI disponible mediante /v1/images/generations.",
  },
  fr: {
    "Knowledge work": "Travail de connaissance",
    "OpenAI-compatible": "Compatible OpenAI",
    "Anthropic-compatible": "Compatible Anthropic",
    "Distillable metadata": "Métadonnées distillable",
    "Relative quality/benchmark": "Qualité/benchmark relatif",
    "Local / open-source assumptions": "Hypothèses locales/open source",
    "Text, image, file": "Texte, image et fichier",
    "Text/file fields": "Champs texte/fichier",
    "Not promised; paid token rates apply": "Non promis ; tarifs par token appliqués",
    "Verify before publishing": "Vérifier avant publication",
    "Benchmark/coding ranking": "Classement benchmark/code",
    Compatibility: "Compatibilité",
    "Use the exact model ID and documented endpoint in your existing client.": "Utilisez l'ID exact du modèle et l'endpoint documenté dans votre client existant.",
    "Background / moderation": "Arrière-plan / modération",
    "Quality/background/moderation": "Qualité/arrière-plan/modération",
    "Image quality ranking": "Classement de qualité d'image",
    "Documented above": "Documenté ci-dessus",
    "Default": "Par défaut",
    "6 seconds": "6 secondes",
    "Explicit boolean": "Booléen explicite",
    "output fields": "champs de sortie",
    "resolution, duration, ratio, and aigc_watermark.": "resolution, duration, ratio et aigc_watermark.",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 est le modèle d'images OpenAI disponible via /v1/images/generations.",
  },
  pt: {
    "Knowledge work": "Trabalho de conhecimento",
    "OpenAI-compatible": "Compatível com OpenAI",
    "Anthropic-compatible": "Compatível com Anthropic",
    "Distillable metadata": "Metadados distillable",
    "Relative quality/benchmark": "Qualidade/benchmark relativo",
    "Local / open-source assumptions": "Suposições locais/open source",
    "Text, image, file": "Texto, imagem e arquivo",
    "Text/file fields": "Campos de texto/arquivo",
    "Not promised; paid token rates apply": "Não prometido; aplicam-se tarifas por token",
    "Verify before publishing": "Verifique antes de publicar",
    "Benchmark/coding ranking": "Ranking de benchmark/código",
    Compatibility: "Compatibilidade",
    "Use the exact model ID and documented endpoint in your existing client.": "Use o ID exato do modelo e o endpoint documentado no seu cliente atual.",
    "Background / moderation": "Fundo / moderação",
    "Quality/background/moderation": "Qualidade/fundo/moderação",
    "Image quality ranking": "Ranking de qualidade da imagem",
    "Documented above": "Documentado acima",
    "Default": "Padrão",
    "6 seconds": "6 segundos",
    "Explicit boolean": "Booleano explícito",
    "output fields": "campos de saída",
    "resolution, duration, ratio, and aigc_watermark.": "resolution, duration, ratio e aigc_watermark.",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 é o modelo de imagem da OpenAI disponível em /v1/images/generations.",
  },
  ru: {
    "Knowledge work": "Работа со знаниями",
    "OpenAI-compatible": "Совместимый с OpenAI",
    "Anthropic-compatible": "Совместимый с Anthropic",
    "Distillable metadata": "Метаданные distillable",
    "Relative quality/benchmark": "Относительное качество/бенчмарк",
    "Local / open-source assumptions": "Предположения о локальном/open-source запуске",
    "Text, image, file": "Текст, изображения и файлы",
    "Text/file fields": "Поля текста/файла",
    "Not promised; paid token rates apply": "Не обещается; действуют тарифы за токены",
    "Verify before publishing": "Проверьте перед публикацией",
    "Benchmark/coding ranking": "Рейтинг бенчмарка/кода",
    Compatibility: "Совместимость",
    "Use the exact model ID and documented endpoint in your existing client.": "Используйте точный ID модели и документированный endpoint в существующем клиенте.",
    "Background / moderation": "Фон / модерация",
    "Quality/background/moderation": "Качество/фон/модерация",
    "Image quality ranking": "Рейтинг качества изображения",
    "Documented above": "Указано выше",
    "Default": "По умолчанию",
    "6 seconds": "6 секунд",
    "Explicit boolean": "Явное логическое значение",
    "output fields": "поля вывода",
    "resolution, duration, ratio, and aigc_watermark.": "resolution, duration, ratio и aigc_watermark.",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 — модель изображений OpenAI, доступная через /v1/images/generations.",
  },
  ja: {
    "Knowledge work": "ナレッジワーク",
    "OpenAI-compatible": "OpenAI 互換",
    "Anthropic-compatible": "Anthropic 互換",
    "Distillable metadata": "Distillable メタデータ",
    "Relative quality/benchmark": "相対的な品質/ベンチマーク",
    "Local / open-source assumptions": "ローカル/オープンソースの仮定",
    "Text, image, file": "テキスト・画像・ファイル",
    "Text/file fields": "テキスト/ファイル項目",
    "Not promised; paid token rates apply": "保証なし。トークン料金が適用されます",
    "Verify before publishing": "公開前に確認",
    "Benchmark/coding ranking": "ベンチマーク/コーディング順位",
    Compatibility: "互換性",
    "Use the exact model ID and documented endpoint in your existing client.": "既存のクライアントで正確なモデル ID と文書化されたエンドポイントを使用します。",
    "Background / moderation": "背景 / モデレーション",
    "Quality/background/moderation": "品質/背景/モデレーション",
    "Image quality ranking": "画像品質の順位",
    "Documented above": "上記に記載",
    "Default": "デフォルト",
    "6 seconds": "6 秒",
    "Explicit boolean": "明示的な boolean",
    "output fields": "出力項目",
    "resolution, duration, ratio, and aigc_watermark.": "resolution、duration、ratio、aigc_watermark。",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 は /v1/images/generations で利用できる OpenAI の画像モデルです。",
  },
  vi: {
    "Knowledge work": "Công việc tri thức",
    "OpenAI-compatible": "Tương thích OpenAI",
    "Anthropic-compatible": "Tương thích Anthropic",
    "Distillable metadata": "Metadata distillable",
    "Relative quality/benchmark": "Chất lượng/benchmark tương đối",
    "Local / open-source assumptions": "Giả định cục bộ/mã nguồn mở",
    "Text, image, file": "Văn bản, hình ảnh và tệp",
    "Text/file fields": "Trường văn bản/tệp",
    "Not promised; paid token rates apply": "Không cam kết; áp dụng giá theo token",
    "Verify before publishing": "Xác minh trước khi xuất bản",
    "Benchmark/coding ranking": "Xếp hạng benchmark/lập trình",
    Compatibility: "Khả năng tương thích",
    "Use the exact model ID and documented endpoint in your existing client.": "Dùng đúng ID mô hình và endpoint được ghi nhận trong client hiện có.",
    "Background / moderation": "Nền / moderation",
    "Quality/background/moderation": "Chất lượng/nền/moderation",
    "Image quality ranking": "Xếp hạng chất lượng ảnh",
    "Documented above": "Đã ghi nhận ở trên",
    "Default": "Mặc định",
    "6 seconds": "6 giây",
    "Explicit boolean": "Boolean rõ ràng",
    "output fields": "trường đầu ra",
    "resolution, duration, ratio, and aigc_watermark.": "resolution, duration, ratio và aigc_watermark.",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 là mô hình hình ảnh OpenAI có thể dùng qua /v1/images/generations.",
  },
  de: {
    "Knowledge work": "Wissensarbeit",
    "OpenAI-compatible": "OpenAI-kompatibel",
    "Anthropic-compatible": "Anthropic-kompatibel",
    "Distillable metadata": "Distillable-Metadaten",
    "Relative quality/benchmark": "Relative Qualität/Benchmark",
    "Local / open-source assumptions": "Annahmen zu lokal/Open Source",
    "Text, image, file": "Text, Bild und Datei",
    "Text/file fields": "Text-/Dateifelder",
    "Not promised; paid token rates apply": "Nicht versprochen; Token-Tarife gelten",
    "Verify before publishing": "Vor Veröffentlichung prüfen",
    "Benchmark/coding ranking": "Benchmark-/Coding-Ranking",
    Compatibility: "Kompatibilität",
    "Use the exact model ID and documented endpoint in your existing client.": "Verwende in deinem bestehenden Client die exakte Modell-ID und den dokumentierten Endpoint.",
    "Background / moderation": "Hintergrund / Moderation",
    "Quality/background/moderation": "Qualität/Hintergrund/Moderation",
    "Image quality ranking": "Bildqualitäts-Ranking",
    "Documented above": "Oben dokumentiert",
    "Default": "Standard",
    "6 seconds": "6 Sekunden",
    "Explicit boolean": "Expliziter Boolean",
    "output fields": "Ausgabefelder",
    "resolution, duration, ratio, and aigc_watermark.": "resolution, duration, ratio und aigc_watermark.",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 ist das OpenAI-Bildmodell über /v1/images/generations.",
  },
  id: {
    "Knowledge work": "Pekerjaan pengetahuan",
    "OpenAI-compatible": "Kompatibel dengan OpenAI",
    "Anthropic-compatible": "Kompatibel dengan Anthropic",
    "Distillable metadata": "Metadata distillable",
    "Relative quality/benchmark": "Kualitas/benchmark relatif",
    "Local / open-source assumptions": "Asumsi lokal/open source",
    "Text, image, file": "Teks, gambar, dan file",
    "Text/file fields": "Bidang teks/file",
    "Not promised; paid token rates apply": "Tidak dijanjikan; tarif token berlaku",
    "Verify before publishing": "Verifikasi sebelum dipublikasikan",
    "Benchmark/coding ranking": "Peringkat benchmark/coding",
    Compatibility: "Kompatibilitas",
    "Use the exact model ID and documented endpoint in your existing client.": "Gunakan ID model yang tepat dan endpoint terdokumentasi pada klien yang ada.",
    "Background / moderation": "Latar / moderasi",
    "Quality/background/moderation": "Kualitas/latar/moderasi",
    "Image quality ranking": "Peringkat kualitas gambar",
    "Documented above": "Didokumentasikan di atas",
    "Default": "Default",
    "6 seconds": "6 detik",
    "Explicit boolean": "Boolean eksplisit",
    "output fields": "bidang output",
    "resolution, duration, ratio, and aigc_watermark.": "resolution, duration, ratio, dan aigc_watermark.",
    "GPT Image 2 is the OpenAI image model available through /v1/images/generations.": "GPT Image 2 adalah model gambar OpenAI yang tersedia melalui /v1/images/generations.",
  },
};

const EXTRA_LITERAL_LABELS: Record<Locale, Record<string, string>> = {
  en: {},
  zh: {
    "image after free tier": "免费额度后的图片",
    "Catalog base": "目录基础值",
    "Reference video": "参考视频",
    "Input image": "输入图片",
    "resolution-dependent": "取决于分辨率",
    "input-video seconds and resolution": "输入视频秒数和分辨率",
    "free allowance and later images may differ": "免费额度和后续图片可能不同",
    "per image": "每张图片",
    "Verified inputs": "已核实输入",
    Off: "关闭",
    "Text, image, and file": "文本、图片和文件",
    "text, image, and file": "文本、图片和文件",
    "text and file": "文本和文件",
    "ComfyUI or local weights": "ComfyUI 或本地权重",
    "Not verified": "未核实",
    "Not verified in Flatkey catalog": "Flatkey 目录未核实",
    Verified: "已核实",
    "Unknown here": "此处未知",
    "Live estimate": "实时估算",
    "Check catalog": "请查看目录",
  },
  es: {
    "image after free tier": "imagen tras el nivel gratuito",
    "Catalog base": "Base del catálogo",
    "Reference video": "Vídeo de referencia",
    "Input image": "Imagen de entrada",
    "resolution-dependent": "depende de la resolución",
    "input-video seconds and resolution": "segundos del vídeo de entrada y resolución",
    "free allowance and later images may differ": "la cuota gratuita y las imágenes posteriores pueden variar",
    "per image": "por imagen",
    "Verified inputs": "Entradas verificadas",
    Off: "Desactivado",
    "Text, image, and file": "Texto, imagen y archivo",
    "text, image, and file": "texto, imagen y archivo",
    "text and file": "texto y archivo",
    "ComfyUI or local weights": "ComfyUI o pesos locales",
    "Not verified": "No verificado",
    "Not verified in Flatkey catalog": "No verificado en el catálogo de Flatkey",
    Verified: "Verificado",
    "Unknown here": "Desconocido aquí",
    "Live estimate": "Estimación en tiempo real",
    "Check catalog": "Consultar el catálogo",
  },
  fr: {
    "image after free tier": "image après le quota gratuit",
    "Catalog base": "Base du catalogue",
    "Reference video": "Vidéo de référence",
    "Input image": "Image d'entrée",
    "resolution-dependent": "selon la résolution",
    "input-video seconds and resolution": "secondes de vidéo d'entrée et résolution",
    "free allowance and later images may differ": "le quota gratuit et les images suivantes peuvent varier",
    "per image": "par image",
    "Verified inputs": "Entrées vérifiées",
    Off: "Désactivé",
    "Text, image, and file": "Texte, image et fichier",
    "text, image, and file": "texte, image et fichier",
    "text and file": "texte et fichier",
    "ComfyUI or local weights": "ComfyUI ou poids locaux",
    "Not verified": "Non vérifié",
    "Not verified in Flatkey catalog": "Non vérifié dans le catalogue Flatkey",
    Verified: "Vérifié",
    "Unknown here": "Inconnu ici",
    "Live estimate": "Estimation en temps réel",
    "Check catalog": "Consulter le catalogue",
  },
  pt: {
    "image after free tier": "imagem após a franquia gratuita",
    "Catalog base": "Base do catálogo",
    "Reference video": "Vídeo de referência",
    "Input image": "Imagem de entrada",
    "resolution-dependent": "depende da resolução",
    "input-video seconds and resolution": "segundos do vídeo de entrada e resolução",
    "free allowance and later images may differ": "a franquia gratuita e as imagens seguintes podem variar",
    "per image": "por imagem",
    "Verified inputs": "Entradas verificadas",
    Off: "Desativado",
    "Text, image, and file": "Texto, imagem e arquivo",
    "text, image, and file": "texto, imagem e arquivo",
    "text and file": "texto e arquivo",
    "ComfyUI or local weights": "ComfyUI ou pesos locais",
    "Not verified": "Não verificado",
    "Not verified in Flatkey catalog": "Não verificado no catálogo Flatkey",
    Verified: "Verificado",
    "Unknown here": "Desconhecido aqui",
    "Live estimate": "Estimativa em tempo real",
    "Check catalog": "Consultar o catálogo",
  },
  ru: {
    "image after free tier": "изображение после бесплатной квоты",
    "Catalog base": "Базовое значение каталога",
    "Reference video": "Референсное видео",
    "Input image": "Входное изображение",
    "resolution-dependent": "зависит от разрешения",
    "input-video seconds and resolution": "секунды входного видео и разрешение",
    "free allowance and later images may differ": "бесплатная квота и последующие изображения могут отличаться",
    "per image": "за изображение",
    "Verified inputs": "Подтверждённые входы",
    Off: "Выкл.",
    "Text, image, and file": "Текст, изображения и файлы",
    "text, image, and file": "текст, изображения и файлы",
    "text and file": "текст и файлы",
    "ComfyUI or local weights": "ComfyUI или локальные веса",
    "Not verified": "Не проверено",
    "Not verified in Flatkey catalog": "Не проверено в каталоге Flatkey",
    Verified: "Проверено",
    "Unknown here": "Неизвестно здесь",
    "Live estimate": "Текущая оценка",
    "Check catalog": "Проверьте каталог",
  },
  ja: {
    "image after free tier": "無料枠後の画像",
    "Catalog base": "カタログ基準値",
    "Reference video": "参照動画",
    "Input image": "入力画像",
    "resolution-dependent": "解像度に依存",
    "input-video seconds and resolution": "入力動画の秒数と解像度",
    "free allowance and later images may differ": "無料枠と後続画像は異なる場合があります",
    "per image": "画像あたり",
    "Verified inputs": "確認済み入力",
    Off: "オフ",
    "Text, image, and file": "テキスト・画像・ファイル",
    "text, image, and file": "テキスト・画像・ファイル",
    "text and file": "テキスト・ファイル",
    "ComfyUI or local weights": "ComfyUI またはローカル重み",
    "Not verified": "未確認",
    "Not verified in Flatkey catalog": "Flatkey カタログで未確認",
    Verified: "確認済み",
    "Unknown here": "ここでは不明",
    "Live estimate": "リアルタイム見積もり",
    "Check catalog": "カタログを確認",
  },
  vi: {
    "image after free tier": "hình ảnh sau hạn mức miễn phí",
    "Catalog base": "Mức cơ bản trong catalog",
    "Reference video": "Video tham chiếu",
    "Input image": "Ảnh đầu vào",
    "resolution-dependent": "phụ thuộc độ phân giải",
    "input-video seconds and resolution": "số giây video đầu vào và độ phân giải",
    "free allowance and later images may differ": "hạn mức miễn phí và ảnh tiếp theo có thể khác",
    "per image": "mỗi ảnh",
    "Verified inputs": "Đầu vào đã xác minh",
    Off: "Tắt",
    "Text, image, and file": "Văn bản, hình ảnh và tệp",
    "text, image, and file": "văn bản, hình ảnh và tệp",
    "text and file": "văn bản và tệp",
    "ComfyUI or local weights": "ComfyUI hoặc trọng số cục bộ",
    "Not verified": "Chưa xác minh",
    "Not verified in Flatkey catalog": "Chưa xác minh trong catalog Flatkey",
    Verified: "Đã xác minh",
    "Unknown here": "Chưa rõ tại đây",
    "Live estimate": "Ước tính trực tiếp",
    "Check catalog": "Xem catalog",
  },
  de: {
    "image after free tier": "Bild nach dem kostenlosen Kontingent",
    "Catalog base": "Katalogbasis",
    "Reference video": "Referenzvideo",
    "Input image": "Eingabebild",
    "resolution-dependent": "auflösungsabhängig",
    "input-video seconds and resolution": "Sekunden des Eingabevideos und Auflösung",
    "free allowance and later images may differ": "kostenloses Kontingent und spätere Bilder können abweichen",
    "per image": "pro Bild",
    "Verified inputs": "Verifizierte Eingaben",
    Off: "Aus",
    "Text, image, and file": "Text, Bild und Datei",
    "text, image, and file": "Text, Bild und Datei",
    "text and file": "Text und Datei",
    "ComfyUI or local weights": "ComfyUI oder lokale Gewichte",
    "Not verified": "Nicht verifiziert",
    "Not verified in Flatkey catalog": "Im Flatkey-Katalog nicht verifiziert",
    Verified: "Verifiziert",
    "Unknown here": "Hier unbekannt",
    "Live estimate": "Live-Schätzung",
    "Check catalog": "Katalog prüfen",
  },
  id: {
    "image after free tier": "gambar setelah kuota gratis",
    "Catalog base": "Nilai dasar katalog",
    "Reference video": "Video referensi",
    "Input image": "Gambar input",
    "resolution-dependent": "bergantung pada resolusi",
    "input-video seconds and resolution": "detik video input dan resolusi",
    "free allowance and later images may differ": "kuota gratis dan gambar berikutnya dapat berbeda",
    "per image": "per gambar",
    "Verified inputs": "Input terverifikasi",
    Off: "Nonaktif",
    "Text, image, and file": "Teks, gambar, dan file",
    "text, image, and file": "teks, gambar, dan file",
    "text and file": "teks dan file",
    "ComfyUI or local weights": "ComfyUI atau bobot lokal",
    "Not verified": "Belum terverifikasi",
    "Not verified in Flatkey catalog": "Belum terverifikasi di katalog Flatkey",
    Verified: "Terverifikasi",
    "Unknown here": "Tidak diketahui di sini",
    "Live estimate": "Estimasi langsung",
    "Check catalog": "Periksa katalog",
  },
};

function literal(locale: Locale, value: string): string {
  return LITERAL_LABELS[locale]?.[value] ?? EXTRA_LITERAL_LABELS[locale]?.[value] ?? value;
}

/**
 * The five priority pages share the same shell, but their editorial proof
 * points are intentionally different.  These helpers keep the `why` and
 * prompt-library sections locale-aware without introducing a second page
 * component.  Technical values (IDs, paths, dimensions and rates) are passed
 * through unchanged; only the surrounding prose is translated.
 */
type AddonCardKind = "routing" | "pricing" | "workflow" | "boundary";
type WhySlot = AddonCardKind | "controls" | "clients" | "utc" | "textBoundary" | "facts" | "account" | "prompting" | "async";

type AddonPack = {
  whyEyebrow: string;
  whyTitle: (name: string) => string;
  whyDescription: (name: string) => string;
  cardTitle: (kind: AddonCardKind) => string;
  cardBody: (kind: AddonCardKind, facts: Facts) => string;
  promptTitle: (name: string, kind: Facts["kind"]) => string;
  promptDescription: string;
  promptLabels: Record<string, string>;
  promptTexts: Record<string, string>;
  promptAlts: Record<string, string>;
};

const PROMPT_MEDIA: Record<string, { poster: string; video?: string }> = {
  "gpt-image-2-product": { poster: "/assets/prompts/awesome-images/ecommerce-skincare.png" },
  "gpt-image-2-ad": { poster: "/assets/prompts/awesome-images/ugc-coffee-ad.png" },
  "gpt-image-2-storyboard": { poster: "/assets/prompts/awesome-images/gpt-image-2-showcase-complex.png" },
  "minimax-h3-product": { poster: "/assets/cli/product-reveal.png", video: "/assets/cli/product-reveal.mp4" },
  "minimax-h3-ugc": { poster: "/assets/cli/ugc-ad-clips.png", video: "/assets/cli/ugc-ad-clips.mp4" },
  "minimax-h3-storyboard": { poster: "/assets/cli/localized-variants.png", video: "/assets/cli/localized-variants.mp4" },
};

/** Source prompts are deliberately concrete and model-specific. */
const PROMPT_SOURCE: Record<string, { label: string; prompt: string; alt: string }> = {
  "gpt-image-2-product": {
    label: "Product mockup",
    prompt: "Studio product mockup of a matte black reusable bottle on a pale stone plinth, one soft side light, clean shadow, no text, leave negative space on the right for a headline.",
    alt: "GPT Image 2 product mockup prompt example",
  },
  "gpt-image-2-ad": {
    label: "Ad creative",
    prompt: "Square social ad still for a citrus skincare launch: glass dropper bottle, sliced bergamot, warm cream background, crisp condensation, editorial daylight, no logo or legible text.",
    alt: "GPT Image 2 advertising prompt example",
  },
  "gpt-image-2-storyboard": {
    label: "Storyboard frame",
    prompt: "Wide storyboard frame of a cyclist entering a rain-lit city tunnel at blue hour, camera low behind the wheel, reflective pavement, clear subject silhouette, cinematic but physically plausible lighting.",
    alt: "GPT Image 2 storyboard prompt example",
  },
  "minimax-h3-product": {
    label: "Product motion",
    prompt: "Six-second product shot: a matte black bottle rotates slowly on a wet stone pedestal, one controlled push-in, small water highlights, uncluttered background, end on a steady hero frame.",
    alt: "MiniMax-H3 product motion prompt example",
  },
  "minimax-h3-ugc": {
    label: "UGC ad clip",
    prompt: "Eight-second vertical UGC-style clip: a creator lifts a compact coffee maker, points to the front control, then smiles to camera; handheld but stable, natural window light, leave the spoken words to the audio track.",
    alt: "MiniMax-H3 UGC ad prompt example",
  },
  "minimax-h3-storyboard": {
    label: "Storyboard shot",
    prompt: "Ten-second wide establishing shot: a courier crosses a rain-soaked plaza toward a lit station, a slow lateral camera move follows, reflections remain consistent, finish with the subject centered under the sign.",
    alt: "MiniMax-H3 storyboard prompt example",
  },
};

const ADDON_PACKS: Record<Locale, AddonPack> = {
  en: {
    whyEyebrow: "Why Flatkey",
    whyTitle: (name) => `Why use ${name} through Flatkey?`,
    whyDescription: (name) => `Keep ${name} facts, request fields, and pricing boundaries visible while you move from a test request to production traffic.`,
    cardTitle: (kind) => ({
      routing: "Exact model routing",
      pricing: "Pricing stays explicit",
      workflow: "Workflow-specific starting points",
      boundary: "No unsupported promises",
    }[kind]),
    cardBody: (kind, facts) => addonBody("en", kind, facts),
    promptTitle: (name, kind) => kind === "image"
      ? `${name} prompt examples for product and marketing visuals`
      : `${name} prompting guide: product, UGC, and storyboard shots`,
    promptDescription: "Start with a concrete subject, action, camera or composition, and an explicit output setting; keep request fields separate from the prompt.",
    promptLabels: Object.fromEntries(Object.entries(PROMPT_SOURCE).map(([key, item]) => [key, item.label])),
    promptTexts: Object.fromEntries(Object.entries(PROMPT_SOURCE).map(([key, item]) => [key, item.prompt])),
    promptAlts: Object.fromEntries(Object.entries(PROMPT_SOURCE).map(([key, item]) => [key, item.alt])),
  },
  zh: {
    whyEyebrow: "为什么选择 Flatkey",
    whyTitle: (name) => `为什么通过 Flatkey 使用 ${name}？`,
    whyDescription: (name) => `在从测试请求走向生产流量时，清楚展示 ${name} 的事实、请求字段和价格边界。`,
    cardTitle: (kind) => ({ routing: "准确的模型路由", pricing: "价格维度清晰", workflow: "贴合工作流的起点", boundary: "不做未经核实的承诺" }[kind]),
    cardBody: (kind, facts) => addonBody("zh", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `${name} 产品与营销图像提示词示例` : `${name} 提示词指南：产品、UGC 与分镜`,
    promptDescription: "从具体主体、动作、镜头或构图和明确的输出设置开始；请求字段与提示词分开配置。",
    promptLabels: { "gpt-image-2-product": "产品样机", "gpt-image-2-ad": "广告创意", "gpt-image-2-storyboard": "分镜画面", "minimax-h3-product": "产品运动", "minimax-h3-ugc": "UGC 广告片段", "minimax-h3-storyboard": "分镜镜头" },
    promptTexts: {
      "gpt-image-2-product": "棚拍产品样机：哑光黑色可重复使用水瓶置于浅色石台上，一盏柔和侧光，干净阴影，不要文字，右侧为标题留出负空间。",
      "gpt-image-2-ad": "柑橘护肤品发布的方形社交广告静帧：玻璃滴管瓶、切开的佛手柑、暖奶油色背景、清晰水珠、编辑式日光，不要标志或可读文字。",
      "gpt-image-2-storyboard": "蓝调时刻，骑行者进入雨光映照的城市隧道的宽幅分镜：镜头低位跟随车轮，路面反光，主体轮廓清晰，电影感但光线符合物理。",
      "minimax-h3-product": "六秒产品镜头：哑光黑色瓶子在湿石台上缓慢旋转，镜头平稳推进，水面有细小高光，背景简洁，最后停在稳定的主视觉画面。",
      "minimax-h3-ugc": "八秒竖屏 UGC 风格片段：创作者拿起小型咖啡机，指向正面控制键，然后对镜头微笑；手持但稳定，自然窗光，口播内容交给音轨。",
      "minimax-h3-storyboard": "十秒宽幅建立镜头：快递员穿过雨后的广场走向亮灯车站，镜头缓慢横移跟随，反射保持一致，最后让主体位于站牌下方中央。",
    },
    promptAlts: { "gpt-image-2-product": "GPT Image 2 产品样机提示词示例", "gpt-image-2-ad": "GPT Image 2 广告提示词示例", "gpt-image-2-storyboard": "GPT Image 2 分镜提示词示例", "minimax-h3-product": "MiniMax-H3 产品运动提示词示例", "minimax-h3-ugc": "MiniMax-H3 UGC 广告提示词示例", "minimax-h3-storyboard": "MiniMax-H3 分镜提示词示例" },
  },
  es: {
    whyEyebrow: "Por qué Flatkey",
    whyTitle: (name) => `¿Por qué usar ${name} con Flatkey?`,
    whyDescription: (name) => `Mantén visibles los datos, campos de solicitud y límites de precio de ${name} al pasar de una prueba a producción.`,
    cardTitle: (kind) => ({ routing: "Enrutamiento exacto", pricing: "Precios explícitos", workflow: "Puntos de partida por flujo", boundary: "Sin promesas no verificadas" }[kind]),
    cardBody: (kind, facts) => addonBody("es", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `Ejemplos de prompts de ${name} para producto y marketing` : `Guía de prompts de ${name}: producto, UGC y storyboard`,
    promptDescription: "Empieza con un sujeto, acción, cámara o composición concretos y un ajuste de salida explícito; separa los campos de la solicitud del prompt.",
    promptLabels: { "gpt-image-2-product": "Maqueta de producto", "gpt-image-2-ad": "Creatividad publicitaria", "gpt-image-2-storyboard": "Plano de storyboard", "minimax-h3-product": "Movimiento de producto", "minimax-h3-ugc": "Clip publicitario UGC", "minimax-h3-storyboard": "Plano de storyboard" },
    promptTexts: {
      "gpt-image-2-product": "Maqueta de producto en estudio: botella reutilizable negra mate sobre pedestal de piedra clara, una luz lateral suave, sombra limpia, sin texto y espacio negativo a la derecha para un titular.",
      "gpt-image-2-ad": "Anuncio social cuadrado para el lanzamiento de un cosmético cítrico: frasco cuentagotas de vidrio, bergamota cortada, fondo crema cálido, condensación nítida, luz editorial, sin logotipo ni texto legible.",
      "gpt-image-2-storyboard": "Plano panorámico de storyboard de un ciclista entrando en un túnel urbano iluminado por la lluvia al anochecer, cámara baja detrás de la rueda, pavimento reflectante y luz físicamente plausible.",
      "minimax-h3-product": "Plano de producto de seis segundos: una botella negra mate gira lentamente sobre un pedestal de piedra mojada, pequeño acercamiento controlado, reflejos de agua y cierre en un encuadre estable.",
      "minimax-h3-ugc": "Clip UGC vertical de ocho segundos: un creador levanta una cafetera compacta, señala el control frontal y sonríe a cámara; cámara en mano estable, luz natural y palabras habladas en la pista de audio.",
      "minimax-h3-storyboard": "Plano general de diez segundos: un repartidor cruza una plaza mojada hacia una estación iluminada, un movimiento lateral lento lo sigue y termina centrado bajo el letrero.",
    },
    promptAlts: { "gpt-image-2-product": "Ejemplo de prompt de maqueta de producto con GPT Image 2", "gpt-image-2-ad": "Ejemplo de prompt publicitario con GPT Image 2", "gpt-image-2-storyboard": "Ejemplo de prompt de storyboard con GPT Image 2", "minimax-h3-product": "Ejemplo de prompt de movimiento de producto con MiniMax-H3", "minimax-h3-ugc": "Ejemplo de prompt UGC con MiniMax-H3", "minimax-h3-storyboard": "Ejemplo de prompt de storyboard con MiniMax-H3" },
  },
  fr: {
    whyEyebrow: "Pourquoi Flatkey",
    whyTitle: (name) => `Pourquoi utiliser ${name} avec Flatkey ?`,
    whyDescription: (name) => `Gardez visibles les faits, les champs de requête et les limites tarifaires de ${name} du test à la production.`,
    cardTitle: (kind) => ({ routing: "Routage précis", pricing: "Tarification explicite", workflow: "Départs adaptés au workflow", boundary: "Aucune promesse non vérifiée" }[kind]),
    cardBody: (kind, facts) => addonBody("fr", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `Exemples de prompts ${name} pour produits et marketing` : `Guide de prompts ${name} : produit, UGC et storyboard`,
    promptDescription: "Commencez par un sujet, une action, une caméra ou une composition précise et un réglage de sortie explicite ; séparez les champs de requête du prompt.",
    promptLabels: { "gpt-image-2-product": "Maquette produit", "gpt-image-2-ad": "Création publicitaire", "gpt-image-2-storyboard": "Image de storyboard", "minimax-h3-product": "Mouvement produit", "minimax-h3-ugc": "Clip publicitaire UGC", "minimax-h3-storyboard": "Plan de storyboard" },
    promptTexts: {
      "gpt-image-2-product": "Maquette produit en studio : bouteille réutilisable noire mate sur un socle en pierre claire, une lumière latérale douce, ombre nette, aucun texte et espace négatif à droite pour un titre.",
      "gpt-image-2-ad": "Visuel publicitaire carré pour le lancement d'un soin aux agrumes : flacon compte-gouttes en verre, bergamote coupée, fond crème chaud, condensation nette, lumière éditoriale, sans logo ni texte lisible.",
      "gpt-image-2-storyboard": "Plan large de storyboard d'un cycliste entrant dans un tunnel urbain éclairé par la pluie à l'heure bleue, caméra basse derrière la roue, chaussée réfléchissante et lumière physiquement plausible.",
      "minimax-h3-product": "Plan produit de six secondes : une bouteille noire mate tourne lentement sur un socle en pierre mouillée, léger travelling avant contrôlé, petits reflets d'eau, fin sur une image stable.",
      "minimax-h3-ugc": "Clip UGC vertical de huit secondes : un créateur soulève une cafetière compacte, montre la commande frontale puis sourit à la caméra ; caméra à main stable, lumière naturelle, paroles laissées à la piste audio.",
      "minimax-h3-storyboard": "Plan d'ensemble de dix secondes : un coursier traverse une place détrempée vers une gare éclairée, un mouvement latéral lent le suit et finit centré sous le panneau.",
    },
    promptAlts: { "gpt-image-2-product": "Exemple de prompt de maquette produit GPT Image 2", "gpt-image-2-ad": "Exemple de prompt publicitaire GPT Image 2", "gpt-image-2-storyboard": "Exemple de prompt storyboard GPT Image 2", "minimax-h3-product": "Exemple de prompt de mouvement produit MiniMax-H3", "minimax-h3-ugc": "Exemple de prompt UGC MiniMax-H3", "minimax-h3-storyboard": "Exemple de prompt storyboard MiniMax-H3" },
  },
  pt: {
    whyEyebrow: "Por que a Flatkey",
    whyTitle: (name) => `Por que usar ${name} pela Flatkey?`,
    whyDescription: (name) => `Mantenha visíveis os fatos, campos da solicitação e limites de preço do ${name} ao passar do teste para a produção.`,
    cardTitle: (kind) => ({ routing: "Roteamento exato", pricing: "Preço explícito", workflow: "Pontos de partida por fluxo", boundary: "Sem promessas não verificadas" }[kind]),
    cardBody: (kind, facts) => addonBody("pt", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `Exemplos de prompts do ${name} para produto e marketing` : `Guia de prompts do ${name}: produto, UGC e storyboard`,
    promptDescription: "Comece com um sujeito, ação, câmera ou composição concretos e uma configuração de saída explícita; mantenha os campos da solicitação separados do prompt.",
    promptLabels: { "gpt-image-2-product": "Mockup de produto", "gpt-image-2-ad": "Criativo de anúncio", "gpt-image-2-storyboard": "Quadro de storyboard", "minimax-h3-product": "Movimento de produto", "minimax-h3-ugc": "Clipe de anúncio UGC", "minimax-h3-storyboard": "Plano de storyboard" },
    promptTexts: {
      "gpt-image-2-product": "Mockup de produto em estúdio: garrafa reutilizável preta fosca sobre pedestal de pedra clara, uma luz lateral suave, sombra limpa, sem texto e espaço negativo à direita para um título.",
      "gpt-image-2-ad": "Imagem quadrada para anúncio social de um lançamento de skincare cítrico: frasco conta-gotas de vidro, bergamota cortada, fundo creme quente, condensação nítida, luz editorial, sem logotipo ou texto legível.",
      "gpt-image-2-storyboard": "Quadro panorâmico de storyboard de um ciclista entrando em um túnel urbano iluminado pela chuva na hora azul, câmera baixa atrás da roda, pavimento refletivo e luz fisicamente plausível.",
      "minimax-h3-product": "Plano de produto de seis segundos: uma garrafa preta fosca gira lentamente sobre um pedestal de pedra molhada, pequeno avanço controlado, reflexos de água e final em um quadro estável.",
      "minimax-h3-ugc": "Clipe vertical UGC de oito segundos: um criador levanta uma cafeteira compacta, aponta para o controle frontal e sorri para a câmera; câmera na mão estável, luz natural e falas deixadas para a faixa de áudio.",
      "minimax-h3-storyboard": "Plano geral de dez segundos: um entregador atravessa uma praça molhada em direção a uma estação iluminada, um movimento lateral lento o acompanha e termina centralizado sob a placa.",
    },
    promptAlts: { "gpt-image-2-product": "Exemplo de prompt de mockup de produto com GPT Image 2", "gpt-image-2-ad": "Exemplo de prompt de anúncio com GPT Image 2", "gpt-image-2-storyboard": "Exemplo de prompt de storyboard com GPT Image 2", "minimax-h3-product": "Exemplo de prompt de movimento de produto com MiniMax-H3", "minimax-h3-ugc": "Exemplo de prompt UGC com MiniMax-H3", "minimax-h3-storyboard": "Exemplo de prompt de storyboard com MiniMax-H3" },
  },
  ru: {
    whyEyebrow: "Почему Flatkey",
    whyTitle: (name) => `Зачем использовать ${name} через Flatkey?`,
    whyDescription: (name) => `При переходе от тестового запроса к продакшену держите на виду факты ${name}, поля запроса и границы тарификации.`,
    cardTitle: (kind) => ({ routing: "Точный роутинг модели", pricing: "Прозрачные измерения цены", workflow: "Стартовые сценарии по задачам", boundary: "Без неподтверждённых обещаний" }[kind]),
    cardBody: (kind, facts) => addonBody("ru", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `Примеры промптов ${name} для товаров и маркетинга` : `Гайд по промптам ${name}: товар, UGC и раскадровка`,
    promptDescription: "Начните с конкретного объекта, действия, камеры или композиции и явной настройки вывода; поля запроса задавайте отдельно от промпта.",
    promptLabels: { "gpt-image-2-product": "Макет продукта", "gpt-image-2-ad": "Рекламный креатив", "gpt-image-2-storyboard": "Кадр раскадровки", "minimax-h3-product": "Движение продукта", "minimax-h3-ugc": "UGC-рекламный клип", "minimax-h3-storyboard": "Кадр раскадровки" },
    promptTexts: {
      "gpt-image-2-product": "Студийный макет товара: матовая чёрная многоразовая бутылка на светлой каменной подставке, один мягкий боковой источник, чистая тень, без текста, справа оставить свободное место под заголовок.",
      "gpt-image-2-ad": "Квадратный кадр для соцсетей к запуску цитрусового ухода: стеклянный флакон с пипеткой, разрезанный бергамот, тёплый кремовый фон, заметный конденсат, редакционный дневной свет, без логотипа и читаемого текста.",
      "gpt-image-2-storyboard": "Широкий кадр раскадровки: велосипедист въезжает в городской туннель под дождём в «синий час», камера низко за колесом, отражающий асфальт, чёткий силуэт и физически правдоподобный свет.",
      "minimax-h3-product": "Шестисекундный товарный кадр: матовая чёрная бутылка медленно вращается на мокрой каменной тумбе, плавный наезд, небольшие блики воды, финал на устойчивом hero-кадре.",
      "minimax-h3-ugc": "Вертикальный UGC-клип на восемь секунд: автор поднимает компактную кофеварку, показывает передний регулятор и улыбается в камеру; стабильная ручная съёмка, естественный свет, слова оставить аудиодорожке.",
      "minimax-h3-storyboard": "Десятисекундный общий план: курьер пересекает мокрую площадь к освещённому вокзалу, медленное боковое движение следует за ним, финал — герой по центру под вывеской.",
    },
    promptAlts: { "gpt-image-2-product": "Пример промпта макета продукта GPT Image 2", "gpt-image-2-ad": "Пример рекламного промпта GPT Image 2", "gpt-image-2-storyboard": "Пример промпта раскадровки GPT Image 2", "minimax-h3-product": "Пример промпта движения продукта MiniMax-H3", "minimax-h3-ugc": "Пример UGC-промпта MiniMax-H3", "minimax-h3-storyboard": "Пример промпта раскадровки MiniMax-H3" },
  },
  ja: {
    whyEyebrow: "Flatkey を選ぶ理由",
    whyTitle: (name) => `${name} を Flatkey で使う理由`,
    whyDescription: (name) => `テストから本番へ移行する間も、${name} の事実、リクエスト項目、料金の境界を確認できます。`,
    cardTitle: (kind) => ({ routing: "正確なモデルルーティング", pricing: "料金項目を明確に表示", workflow: "用途別の出発点", boundary: "未確認の約束をしない" }[kind]),
    cardBody: (kind, facts) => addonBody("ja", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `${name} の商品・マーケティング向けプロンプト例` : `${name} プロンプトガイド：商品、UGC、絵コンテ`,
    promptDescription: "具体的な被写体、動作、カメラまたは構図と出力設定から始め、リクエスト項目はプロンプトとは別に指定します。",
    promptLabels: { "gpt-image-2-product": "商品モックアップ", "gpt-image-2-ad": "広告クリエイティブ", "gpt-image-2-storyboard": "絵コンテフレーム", "minimax-h3-product": "商品モーション", "minimax-h3-ugc": "UGC 広告クリップ", "minimax-h3-storyboard": "絵コンテショット" },
    promptTexts: {
      "gpt-image-2-product": "スタジオの商品モックアップ：淡い石の台に置いたマットブラックの再利用ボトル、柔らかなサイドライト一灯、きれいな影、文字なし、右側に見出し用の余白。",
      "gpt-image-2-ad": "柑橘系スキンケア発売の正方形ソーシャル広告：ガラスのスポイトボトル、カットしたベルガモット、暖かなクリーム色の背景、鮮明な水滴、編集的な昼光、ロゴや読める文字なし。",
      "gpt-image-2-storyboard": "ブルーアワーの雨に照らされた都市トンネルへ自転車が入るワイド絵コンテ、車輪の後ろから低いカメラ、反射する路面、明瞭なシルエット、物理的に自然な照明。",
      "minimax-h3-product": "6 秒の商品ショット：濡れた石の台でマットブラックのボトルがゆっくり回転し、制御されたプッシュイン、小さな水のハイライト、最後は安定したヒーローフレーム。",
      "minimax-h3-ugc": "8 秒の縦型 UGC クリップ：クリエイターが小型コーヒーメーカーを持ち上げ、前面の操作部を指してカメラに笑顔を向ける。安定した手持ち撮影と自然光、話す内容は音声トラックに任せる。",
      "minimax-h3-storyboard": "10 秒のワイドな導入ショット：配達員が雨に濡れた広場を照明のある駅へ進み、ゆっくり横移動するカメラが追い、看板の下で被写体を中央にして終える。",
    },
    promptAlts: { "gpt-image-2-product": "GPT Image 2 商品モックアップのプロンプト例", "gpt-image-2-ad": "GPT Image 2 広告プロンプト例", "gpt-image-2-storyboard": "GPT Image 2 絵コンテプロンプト例", "minimax-h3-product": "MiniMax-H3 商品モーションのプロンプト例", "minimax-h3-ugc": "MiniMax-H3 UGC 広告プロンプト例", "minimax-h3-storyboard": "MiniMax-H3 絵コンテプロンプト例" },
  },
  vi: {
    whyEyebrow: "Vì sao chọn Flatkey",
    whyTitle: (name) => `Vì sao dùng ${name} qua Flatkey?`,
    whyDescription: (name) => `Giữ rõ dữ kiện, trường yêu cầu và ranh giới giá của ${name} khi chuyển từ thử nghiệm sang môi trường thực tế.`,
    cardTitle: (kind) => ({ routing: "Định tuyến đúng model", pricing: "Minh bạch từng loại giá", workflow: "Điểm bắt đầu theo quy trình", boundary: "Không hứa điều chưa xác minh" }[kind]),
    cardBody: (kind, facts) => addonBody("vi", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `Ví dụ prompt ${name} cho sản phẩm và marketing` : `Hướng dẫn prompt ${name}: sản phẩm, UGC và storyboard`,
    promptDescription: "Bắt đầu bằng chủ thể, hành động, máy quay hoặc bố cục cụ thể cùng thiết lập đầu ra rõ ràng; tách trường yêu cầu khỏi prompt.",
    promptLabels: { "gpt-image-2-product": "Mockup sản phẩm", "gpt-image-2-ad": "Mẫu quảng cáo", "gpt-image-2-storyboard": "Khung storyboard", "minimax-h3-product": "Chuyển động sản phẩm", "minimax-h3-ugc": "Clip quảng cáo UGC", "minimax-h3-storyboard": "Cảnh storyboard" },
    promptTexts: {
      "gpt-image-2-product": "Mockup sản phẩm trong studio: chai tái sử dụng màu đen mờ trên bệ đá sáng, một đèn hắt mềm, bóng sạch, không chữ, chừa khoảng trống bên phải cho tiêu đề.",
      "gpt-image-2-ad": "Ảnh quảng cáo mạng xã hội vuông cho sản phẩm chăm sóc da cam quýt: chai nhỏ giọt thủy tinh, bergamot cắt lát, nền kem ấm, giọt nước rõ, ánh sáng biên tập, không logo hay chữ đọc được.",
      "gpt-image-2-storyboard": "Khung storyboard rộng: người đi xe đạp vào đường hầm thành phố dưới mưa lúc blue hour, máy quay thấp phía sau bánh xe, mặt đường phản chiếu, bóng chủ thể rõ và ánh sáng hợp lý.",
      "minimax-h3-product": "Cảnh sản phẩm sáu giây: chai đen mờ xoay chậm trên bệ đá ướt, máy quay tiến nhẹ có kiểm soát, điểm sáng nước nhỏ, kết thúc ở khung hero ổn định.",
      "minimax-h3-ugc": "Clip UGC dọc tám giây: người sáng tạo nâng máy pha cà phê nhỏ, chỉ vào nút điều khiển phía trước rồi mỉm cười với máy quay; cầm tay ổn định, ánh sáng cửa sổ tự nhiên, lời nói để ở track âm thanh.",
      "minimax-h3-storyboard": "Cảnh toàn rộng mười giây: người giao hàng băng qua quảng trường ướt mưa về phía ga sáng đèn, máy quay lia ngang chậm để bám theo, kết thúc khi chủ thể ở giữa dưới biển hiệu.",
    },
    promptAlts: { "gpt-image-2-product": "Ví dụ prompt mockup sản phẩm GPT Image 2", "gpt-image-2-ad": "Ví dụ prompt quảng cáo GPT Image 2", "gpt-image-2-storyboard": "Ví dụ prompt storyboard GPT Image 2", "minimax-h3-product": "Ví dụ prompt chuyển động sản phẩm MiniMax-H3", "minimax-h3-ugc": "Ví dụ prompt quảng cáo UGC MiniMax-H3", "minimax-h3-storyboard": "Ví dụ prompt storyboard MiniMax-H3" },
  },
  de: {
    whyEyebrow: "Warum Flatkey",
    whyTitle: (name) => `Warum ${name} über Flatkey nutzen?`,
    whyDescription: (name) => `Halte Fakten, Anfragefelder und Preisgrenzen von ${name} sichtbar, wenn du vom Test in den Produktivbetrieb wechselst.`,
    cardTitle: (kind) => ({ routing: "Exaktes Modell-Routing", pricing: "Preise bleiben nachvollziehbar", workflow: "Startpunkte für konkrete Workflows", boundary: "Keine unbestätigten Versprechen" }[kind]),
    cardBody: (kind, facts) => addonBody("de", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `${name}-Promptbeispiele für Produkt- und Marketingbilder` : `${name}-Promptleitfaden: Produkt, UGC und Storyboard`,
    promptDescription: "Beginne mit einem konkreten Motiv, einer Aktion, Kamera oder Komposition und einer klaren Ausgabeeinstellung; Anfragefelder getrennt vom Prompt setzen.",
    promptLabels: { "gpt-image-2-product": "Produkt-Mockup", "gpt-image-2-ad": "Werbemotiv", "gpt-image-2-storyboard": "Storyboard-Frame", "minimax-h3-product": "Produktbewegung", "minimax-h3-ugc": "UGC-Werbeclip", "minimax-h3-storyboard": "Storyboard-Aufnahme" },
    promptTexts: {
      "gpt-image-2-product": "Studio-Produkt-Mockup: matte schwarze Mehrwegflasche auf hellem Steinsockel, ein weiches Seitenlicht, sauberer Schatten, kein Text, rechts Freiraum für eine Überschrift.",
      "gpt-image-2-ad": "Quadratisches Social-Ad-Motiv für einen Zitruspflege-Launch: Glasflasche mit Pipette, aufgeschnittene Bergamotte, warmer cremefarbener Hintergrund, klare Kondensation, redaktionelles Tageslicht, kein Logo und kein lesbarer Text.",
      "gpt-image-2-storyboard": "Breiter Storyboard-Frame eines Radfahrers, der bei blauer Stunde in einen regennassen Stadttunnel fährt, niedrige Kamera hinter dem Rad, reflektierender Asphalt, klare Silhouette und physikalisch plausible Beleuchtung.",
      "minimax-h3-product": "Sechssekündige Produktaufnahme: Eine matte schwarze Flasche dreht sich langsam auf einem nassen Steinsockel, kontrollierter Push-in, kleine Wasserreflexe, Abschluss in einem stabilen Hero-Frame.",
      "minimax-h3-ugc": "Achtsekündiger vertikaler UGC-Clip: Ein Creator hebt eine kompakte Kaffeemaschine an, zeigt auf die Frontsteuerung und lächelt in die Kamera; stabile Handkamera, natürliches Fensterlicht, gesprochene Worte auf der Audiospur.",
      "minimax-h3-storyboard": "Zehnsekündige Totale: Ein Kurier überquert einen regennassen Platz zu einem beleuchteten Bahnhof, langsame seitliche Kamerafahrt, am Ende steht die Figur mittig unter dem Schild.",
    },
    promptAlts: { "gpt-image-2-product": "GPT Image 2 Produkt-Mockup-Promptbeispiel", "gpt-image-2-ad": "GPT Image 2 Werbe-Promptbeispiel", "gpt-image-2-storyboard": "GPT Image 2 Storyboard-Promptbeispiel", "minimax-h3-product": "MiniMax-H3 Produktbewegungs-Promptbeispiel", "minimax-h3-ugc": "MiniMax-H3 UGC-Werbe-Promptbeispiel", "minimax-h3-storyboard": "MiniMax-H3 Storyboard-Promptbeispiel" },
  },
  id: {
    whyEyebrow: "Mengapa Flatkey",
    whyTitle: (name) => `Mengapa menggunakan ${name} melalui Flatkey?`,
    whyDescription: (name) => `Tampilkan fakta, bidang permintaan, dan batas harga ${name} saat berpindah dari pengujian ke produksi.`,
    cardTitle: (kind) => ({ routing: "Routing model yang tepat", pricing: "Harga tetap jelas", workflow: "Titik awal sesuai alur kerja", boundary: "Tanpa janji yang belum terverifikasi" }[kind]),
    cardBody: (kind, facts) => addonBody("id", kind, facts),
    promptTitle: (name, kind) => kind === "image" ? `Contoh prompt ${name} untuk visual produk dan pemasaran` : `Panduan prompt ${name}: produk, UGC, dan storyboard`,
    promptDescription: "Mulai dengan subjek, aksi, kamera atau komposisi yang konkret serta pengaturan output yang jelas; pisahkan bidang permintaan dari prompt.",
    promptLabels: { "gpt-image-2-product": "Mockup produk", "gpt-image-2-ad": "Kreatif iklan", "gpt-image-2-storyboard": "Frame storyboard", "minimax-h3-product": "Gerak produk", "minimax-h3-ugc": "Klip iklan UGC", "minimax-h3-storyboard": "Shot storyboard" },
    promptTexts: {
      "gpt-image-2-product": "Mockup produk studio: botol pakai ulang hitam matte di atas pedestal batu pucat, satu cahaya samping lembut, bayangan bersih, tanpa teks, sisakan ruang kosong di kanan untuk judul.",
      "gpt-image-2-ad": "Visual iklan sosial persegi untuk peluncuran skincare sitrus: botol pipet kaca, bergamot iris, latar krim hangat, kondensasi tajam, cahaya editorial, tanpa logo atau teks terbaca.",
      "gpt-image-2-storyboard": "Frame storyboard lebar tentang pesepeda memasuki terowongan kota yang diterangi hujan saat blue hour, kamera rendah di belakang roda, aspal reflektif, siluet jelas, pencahayaan masuk akal secara fisik.",
      "minimax-h3-product": "Shot produk enam detik: botol hitam matte berputar perlahan di pedestal batu basah, push-in terkontrol, kilau air kecil, akhiri pada frame hero yang stabil.",
      "minimax-h3-ugc": "Klip UGC vertikal delapan detik: kreator mengangkat pembuat kopi ringkas, menunjuk kontrol depan, lalu tersenyum ke kamera; handheld stabil, cahaya jendela alami, kata-kata dibiarkan di trek audio.",
      "minimax-h3-storyboard": "Shot pembuka lebar sepuluh detik: kurir melintasi alun-alun basah menuju stasiun yang terang, kamera bergerak lateral perlahan, akhiri subjek di tengah bawah papan tanda.",
    },
    promptAlts: { "gpt-image-2-product": "Contoh prompt mockup produk GPT Image 2", "gpt-image-2-ad": "Contoh prompt iklan GPT Image 2", "gpt-image-2-storyboard": "Contoh prompt storyboard GPT Image 2", "minimax-h3-product": "Contoh prompt gerak produk MiniMax-H3", "minimax-h3-ugc": "Contoh prompt iklan UGC MiniMax-H3", "minimax-h3-storyboard": "Contoh prompt storyboard MiniMax-H3" },
  },
};

/** Model-specific body copy keeps the repeated shell from becoming duplicate prose. */
function addonBody(locale: Locale, kind: AddonCardKind, facts: Facts): string {
  const id = facts.id;
  const endpoint = facts.endpoint;
  const second = facts.secondEndpoint;
  const context = facts.context ?? "1,048,576 tokens";
  const contextToken = context.replace(/\s+tokens?$/i, "-token");
  const text = facts.modalities;
  const templates: Record<Locale, Record<AddonCardKind, (facts: Facts) => string>> = {
    en: {
      routing: (f) => f.kind === "video" ? `Set ${f.id} with ${f.endpoint} and keep the asynchronous task path in your client.` : f.kind === "image" ? `Send ${f.id} to ${f.endpoint} and keep image controls in the request.` : `Set ${f.id} explicitly and keep ${f.endpoint} in the client configuration.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Keep peak and off-peak UTC dimensions together; do not replace the catalog rule with one blended rate." : f.slug === "minimax-h3" ? "Review the $0.08 per-second catalog base alongside resolution and reference-input estimates." : `Review each documented input, output, and cache dimension for ${f.name} instead of treating one headline price as universal.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Use the ${contextToken} context field for document, code, and agent workflow planning.` : f.slug === "gpt-image-2" ? "Use product, ad, and storyboard examples as editable starting prompts." : f.slug === "kimi-k3" ? `Use the ${contextToken} context and file input for long documents, codebases, and research notes.` : f.slug === "deepseek-v4-pro" ? `Plan text and file workflows around the verified ${contextToken} context.` : "Use product, UGC, and storyboard examples with explicit video settings.",
      boundary: (f) => f.slug === "gpt-image-2" ? "The page does not promise transparent output, free generation, or a fixed per-image price when those facts are not verified." : f.slug === "minimax-h3" ? "The catalog does not verify native Flatkey ComfyUI delivery or local weights; this page does not imply either." : f.slug === "deepseek-v4-pro" ? "Compare integration fields without publishing an unsupported benchmark or coding-superiority claim." : f.slug === "kimi-k3" ? "The page separates hosted API access from unverified claims about downloadable weights or local hardware." : "Keep the catalog's documented fields separate from unsupported performance or availability claims.",
    },
    zh: {
      routing: (f) => f.kind === "video" ? `在 ${f.endpoint} 中设置 ${f.id}，并在客户端保留异步任务路径。` : f.kind === "image" ? `将 ${f.id} 发送到 ${f.endpoint}，并在请求中保留图像控制项。` : `明确设置 ${f.id}，并在客户端配置中保留 ${f.endpoint}。`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "同时保留 UTC 高峰与非高峰维度，不要把目录规则替换成一个混合费率。" : f.slug === "minimax-h3" ? "将每秒 $0.08 的目录基础值与分辨率、参考输入估算一起查看。" : `分别查看 ${f.name} 已记录的输入、输出和缓存维度，不要把一个标题价格当作统一价格。`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `利用 ${context} 上下文字段规划文档、代码和智能体工作流。` : f.slug === "gpt-image-2" ? "将产品、广告和分镜示例作为可编辑的提示词起点。" : f.slug === "kimi-k3" ? `利用 ${context} 上下文和文件输入处理长文档、代码库和研究笔记。` : f.slug === "deepseek-v4-pro" ? `围绕已核实的 ${context} 上下文规划文本和文件工作流。` : "配合明确的视频设置使用产品、UGC 和分镜示例。",
      boundary: (f) => f.slug === "gpt-image-2" ? "在事实未核实的情况下，本页不承诺透明输出、免费生成或固定单图价格。" : f.slug === "minimax-h3" ? "目录未核实 Flatkey 原生 ComfyUI 交付或本地权重；本页不作此暗示。" : f.slug === "deepseek-v4-pro" ? "比较集成字段，不发布未经支持的基准或编程优势结论。" : f.slug === "kimi-k3" ? "本页将托管 API 访问与可下载权重、本地硬件等未核实说法分开。" : "将目录已记录字段与未经支持的性能或可用性说法分开。",
    },
    es: {
      routing: (f) => f.kind === "video" ? `Configura ${f.id} con ${f.endpoint} y conserva el flujo de tarea asíncrona en tu cliente.` : f.kind === "image" ? `Envía ${f.id} a ${f.endpoint} y mantén los controles de imagen en la solicitud.` : `Define ${f.id} explícitamente y conserva ${f.endpoint} en la configuración del cliente.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Mantén juntas las dimensiones UTC de hora punta y fuera de punta; no sustituyas la regla del catálogo por una tarifa combinada." : f.slug === "minimax-h3" ? "Revisa la base de $0.08 por segundo junto con las estimaciones de resolución y referencias." : `Revisa cada dimensión documentada de entrada, salida y caché de ${f.name}; no trates un precio destacado como universal.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Usa el campo de contexto de ${context} para planificar documentos, código y agentes.` : f.slug === "gpt-image-2" ? "Usa los ejemplos de producto, anuncio y storyboard como prompts editables." : f.slug === "kimi-k3" ? `Usa el contexto de ${context} y la entrada de archivos para documentos largos, código e investigación.` : f.slug === "deepseek-v4-pro" ? `Planifica flujos de texto y archivos alrededor del contexto verificado de ${context}.` : "Usa ejemplos de producto, UGC y storyboard con ajustes de vídeo explícitos.",
      boundary: (f) => f.slug === "gpt-image-2" ? "La página no promete salida transparente, generación gratuita ni un precio fijo por imagen si esos hechos no están verificados." : f.slug === "minimax-h3" ? "El catálogo no verifica entrega nativa de ComfyUI ni pesos locales de Flatkey; la página no lo implica." : f.slug === "deepseek-v4-pro" ? "Compara campos de integración sin publicar un benchmark o una superioridad de programación no respaldados." : f.slug === "kimi-k3" ? "La página separa el acceso a la API alojada de afirmaciones no verificadas sobre pesos descargables o hardware local." : "Separa los campos documentados del catálogo de afirmaciones de rendimiento o disponibilidad no respaldadas.",
    },
    fr: {
      routing: (f) => f.kind === "video" ? `Définissez ${f.id} avec ${f.endpoint} et conservez le flux de tâche asynchrone dans votre client.` : f.kind === "image" ? `Envoyez ${f.id} à ${f.endpoint} et gardez les contrôles d'image dans la requête.` : `Définissez ${f.id} explicitement et gardez ${f.endpoint} dans la configuration du client.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Conservez les dimensions UTC de pointe et hors pointe ; ne remplacez pas la règle du catalogue par un tarif mixte." : f.slug === "minimax-h3" ? "Examinez la base de 0,08 $ par seconde avec les estimations de résolution et de références." : `Examinez chaque dimension documentée d'entrée, de sortie et de cache de ${f.name}, plutôt qu'un prix unique présenté comme universel.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Utilisez le champ de contexte de ${context} pour planifier documents, code et agents.` : f.slug === "gpt-image-2" ? "Utilisez les exemples produit, publicité et storyboard comme prompts modifiables." : f.slug === "kimi-k3" ? `Utilisez le contexte de ${context} et les fichiers pour les longs documents, dépôts de code et notes de recherche.` : f.slug === "deepseek-v4-pro" ? `Planifiez les flux texte et fichier autour du contexte vérifié de ${context}.` : "Utilisez les exemples produit, UGC et storyboard avec des réglages vidéo explicites.",
      boundary: (f) => f.slug === "gpt-image-2" ? "La page ne promet ni sortie transparente, ni génération gratuite, ni prix fixe par image lorsque ces faits ne sont pas vérifiés." : f.slug === "minimax-h3" ? "Le catalogue ne vérifie pas une livraison ComfyUI native ou des poids locaux Flatkey ; la page ne le suggère pas." : f.slug === "deepseek-v4-pro" ? "Comparez les champs d'intégration sans publier de benchmark ou de supériorité en programmation non étayés." : f.slug === "kimi-k3" ? "La page distingue l'accès à l'API hébergée des affirmations non vérifiées sur les poids téléchargeables ou le matériel local." : "Séparez les champs documentés du catalogue des promesses de performance ou de disponibilité non étayées.",
    },
    pt: {
      routing: (f) => f.kind === "video" ? `Defina ${f.id} com ${f.endpoint} e mantenha o fluxo assíncrono da tarefa no cliente.` : f.kind === "image" ? `Envie ${f.id} para ${f.endpoint} e mantenha os controles de imagem na solicitação.` : `Defina ${f.id} explicitamente e mantenha ${f.endpoint} na configuração do cliente.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Mantenha juntas as dimensões UTC de pico e fora de pico; não substitua a regra do catálogo por uma tarifa combinada." : f.slug === "minimax-h3" ? "Confira a base de US$ 0,08 por segundo junto com as estimativas de resolução e referências." : `Veja cada dimensão documentada de entrada, saída e cache do ${f.name}; um preço de destaque não é universal.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Use o campo de contexto de ${context} para planejar documentos, código e agentes.` : f.slug === "gpt-image-2" ? "Use exemplos de produto, anúncio e storyboard como prompts editáveis." : f.slug === "kimi-k3" ? `Use o contexto de ${context} e a entrada de arquivos para documentos longos, bases de código e pesquisa.` : f.slug === "deepseek-v4-pro" ? `Planeje fluxos de texto e arquivo em torno do contexto verificado de ${context}.` : "Use exemplos de produto, UGC e storyboard com configurações de vídeo explícitas.",
      boundary: (f) => f.slug === "gpt-image-2" ? "A página não promete saída transparente, geração gratuita ou preço fixo por imagem quando esses fatos não estão verificados." : f.slug === "minimax-h3" ? "O catálogo não verifica entrega nativa do ComfyUI ou pesos locais da Flatkey; a página não sugere isso." : f.slug === "deepseek-v4-pro" ? "Compare campos de integração sem publicar benchmark ou superioridade em código sem suporte." : f.slug === "kimi-k3" ? "A página separa o acesso à API hospedada de alegações não verificadas sobre pesos para download ou hardware local." : "Separe os campos documentados no catálogo de alegações de desempenho ou disponibilidade sem suporte.",
    },
    ru: {
      routing: (f) => f.kind === "video" ? `Укажите ${f.id} через ${f.endpoint} и сохраните асинхронный путь задачи в клиенте.` : f.kind === "image" ? `Отправляйте ${f.id} на ${f.endpoint}, сохраняя настройки изображения в запросе.` : `Явно задайте ${f.id} и оставьте ${f.endpoint} в конфигурации клиента.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Сохраняйте тарифные измерения UTC для пикового и непикового времени; не заменяйте правило каталога одной смешанной ставкой." : f.slug === "minimax-h3" ? "Сверяйте базовые $0.08 за секунду с оценками для разрешения и референсов." : `Проверяйте каждое документированное измерение входа, выхода и кэша ${f.name}, а не воспринимайте одну цену как универсальную.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Используйте поле контекста ${context} для планирования документов, кода и агентов.` : f.slug === "gpt-image-2" ? "Используйте примеры товара, рекламы и раскадровки как редактируемые промпты." : f.slug === "kimi-k3" ? `Используйте контекст ${context} и файлы для больших документов, кодовых баз и исследовательских заметок.` : f.slug === "deepseek-v4-pro" ? `Планируйте текстовые и файловые сценарии с учётом подтверждённого контекста ${context}.` : "Используйте примеры товара, UGC и раскадровки с явными настройками видео.",
      boundary: (f) => f.slug === "gpt-image-2" ? "Страница не обещает прозрачный результат, бесплатную генерацию или фиксированную цену изображения, если это не подтверждено." : f.slug === "minimax-h3" ? "Каталог не подтверждает нативную поставку Flatkey ComfyUI или локальные веса; страница этого не подразумевает." : f.slug === "deepseek-v4-pro" ? "Сравнивайте поля интеграции, не публикуя неподтверждённый бенчмарк или превосходство в программировании." : f.slug === "kimi-k3" ? "Страница отделяет доступ к размещённому API от неподтверждённых заявлений о скачиваемых весах или локальном оборудовании." : "Отделяйте документированные поля каталога от неподтверждённых заявлений о качестве или доступности.",
    },
    ja: {
      routing: (f) => f.kind === "video" ? `${f.endpoint} で ${f.id} を指定し、非同期タスクの流れをクライアントに保持します。` : f.kind === "image" ? `${f.endpoint} に ${f.id} を送り、画像設定をリクエストに含めます。` : `${f.id} を明示的に指定し、クライアント設定に ${f.endpoint} を残します。`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "UTC のピーク・オフピークの項目を併記し、カタログのルールを単一の混合料金に置き換えないでください。" : f.slug === "minimax-h3" ? "1 秒あたり $0.08 のカタログ基準値を、解像度と参照入力の見積もりと合わせて確認します。" : `${f.name} の入力・出力・キャッシュの各項目を確認し、見出しの価格を一律料金とみなさないでください。`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `${context} のコンテキスト項目を文書、コード、エージェントの計画に利用します。` : f.slug === "gpt-image-2" ? "商品、広告、絵コンテの例を編集可能なプロンプトの出発点として使います。" : f.slug === "kimi-k3" ? `${context} のコンテキストとファイル入力を長文書、コードベース、調査メモに利用します。` : f.slug === "deepseek-v4-pro" ? `確認済みの ${context} コンテキストを前提にテキストとファイルのワークフローを設計します。` : "商品、UGC、絵コンテの例を明示的な動画設定と組み合わせます。",
      boundary: (f) => f.slug === "gpt-image-2" ? "確認されていない事実について、透明な出力、無料生成、固定の画像単価を約束しません。" : f.slug === "minimax-h3" ? "カタログは Flatkey のネイティブ ComfyUI 配信やローカル重みを確認していません。" : f.slug === "deepseek-v4-pro" ? "統合項目を比較し、裏付けのないベンチマークやコーディング優位性を公開しません。" : f.slug === "kimi-k3" ? "ホスト型 API の利用と、ダウンロード可能な重みやローカル機器に関する未確認の主張を分けます。" : "カタログの確認済み項目と、裏付けのない性能・可用性の主張を分けます。",
    },
    vi: {
      routing: (f) => f.kind === "video" ? `Đặt ${f.id} với ${f.endpoint} và giữ luồng tác vụ bất đồng bộ trong client.` : f.kind === "image" ? `Gửi ${f.id} tới ${f.endpoint} và giữ các điều khiển hình ảnh trong request.` : `Đặt rõ ${f.id} và giữ ${f.endpoint} trong cấu hình client.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Giữ các mức UTC giờ cao điểm và ngoài cao điểm cùng nhau; không thay quy tắc catalog bằng một mức giá trộn." : f.slug === "minimax-h3" ? "Xem giá cơ bản $0.08 mỗi giây cùng ước tính theo độ phân giải và đầu vào tham chiếu." : `Xem từng chiều input, output và cache đã ghi nhận của ${f.name}, thay vì coi một giá nổi bật là giá chung.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Dùng trường ngữ cảnh ${context} để lập kế hoạch tài liệu, code và agent.` : f.slug === "gpt-image-2" ? "Dùng ví dụ sản phẩm, quảng cáo và storyboard làm prompt có thể chỉnh sửa." : f.slug === "kimi-k3" ? `Dùng ngữ cảnh ${context} và đầu vào file cho tài liệu dài, codebase và ghi chú nghiên cứu.` : f.slug === "deepseek-v4-pro" ? `Lập quy trình văn bản và file dựa trên ngữ cảnh ${context} đã xác minh.` : "Dùng ví dụ sản phẩm, UGC và storyboard cùng thiết lập video rõ ràng.",
      boundary: (f) => f.slug === "gpt-image-2" ? "Trang không hứa đầu ra nền trong suốt, tạo miễn phí hay giá cố định mỗi ảnh khi các dữ kiện đó chưa được xác minh." : f.slug === "minimax-h3" ? "Catalog chưa xác minh việc phân phối ComfyUI gốc hoặc trọng số cục bộ của Flatkey; trang không ngụ ý điều đó." : f.slug === "deepseek-v4-pro" ? "So sánh trường tích hợp mà không công bố benchmark hay ưu thế coding chưa có căn cứ." : f.slug === "kimi-k3" ? "Trang tách quyền truy cập API hosted khỏi tuyên bố chưa xác minh về trọng số tải xuống hoặc phần cứng cục bộ." : "Tách trường đã ghi nhận trong catalog khỏi tuyên bố hiệu năng hay khả dụng chưa có căn cứ.",
    },
    de: {
      routing: (f) => f.kind === "video" ? `Setze ${f.id} mit ${f.endpoint} und behalte den asynchronen Aufgabenpfad im Client.` : f.kind === "image" ? `Sende ${f.id} an ${f.endpoint} und führe die Bildsteuerungen in der Anfrage mit.` : `Setze ${f.id} explizit und behalte ${f.endpoint} in der Client-Konfiguration.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Halte UTC-Spitzen- und Nebenzeiten zusammen; ersetze die Katalogregel nicht durch einen gemischten Tarif." : f.slug === "minimax-h3" ? "Prüfe die Katalogbasis von 0,08 $ pro Sekunde zusammen mit Auflösungs- und Referenzschätzungen." : `Prüfe jede dokumentierte Eingabe-, Ausgabe- und Cache-Dimension von ${f.name}, statt einen Einzelpreis als universell zu behandeln.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Nutze das ${context}-Kontextfeld für die Planung von Dokumenten, Code und Agents.` : f.slug === "gpt-image-2" ? "Nutze Produkt-, Werbe- und Storyboard-Beispiele als editierbare Prompts." : f.slug === "kimi-k3" ? `Nutze den ${context}-Kontext und Dateieingaben für lange Dokumente, Codebasen und Forschungsnotizen.` : f.slug === "deepseek-v4-pro" ? `Plane Text- und Datei-Workflows mit dem verifizierten ${context}-Kontext.` : "Nutze Produkt-, UGC- und Storyboard-Beispiele mit expliziten Videoeinstellungen.",
      boundary: (f) => f.slug === "gpt-image-2" ? "Die Seite verspricht keine Transparenz, kostenlose Generierung oder einen festen Bildpreis, wenn diese Fakten nicht verifiziert sind." : f.slug === "minimax-h3" ? "Der Katalog bestätigt weder native Flatkey-ComfyUI-Bereitstellung noch lokale Gewichte; die Seite deutet dies nicht an." : f.slug === "deepseek-v4-pro" ? "Vergleiche Integrationsfelder, ohne ein unbelegtes Benchmark- oder Coding-Überlegenheitsversprechen zu veröffentlichen." : f.slug === "kimi-k3" ? "Die Seite trennt gehosteten API-Zugriff von unbestätigten Aussagen zu herunterladbaren Gewichten oder lokaler Hardware." : "Trenne dokumentierte Katalogfelder von unbelegten Aussagen zu Leistung oder Verfügbarkeit.",
    },
    id: {
      routing: (f) => f.kind === "video" ? `Tetapkan ${f.id} dengan ${f.endpoint} dan simpan alur tugas asinkron di klien.` : f.kind === "image" ? `Kirim ${f.id} ke ${f.endpoint} dan sertakan kontrol gambar dalam permintaan.` : `Tetapkan ${f.id} secara eksplisit dan pertahankan ${f.endpoint} dalam konfigurasi klien.`,
      pricing: (f) => f.slug === "deepseek-v4-pro" ? "Pertahankan dimensi UTC jam sibuk dan di luar jam sibuk; jangan mengganti aturan katalog dengan tarif campuran." : f.slug === "minimax-h3" ? "Periksa dasar $0,08 per detik bersama estimasi resolusi dan input referensi." : `Periksa setiap dimensi input, output, dan cache ${f.name} yang terdokumentasi; satu harga utama bukan harga universal.`,
      workflow: (f) => f.slug === "gpt-5-6-sol" ? `Gunakan bidang konteks ${context} untuk merencanakan dokumen, kode, dan alur agent.` : f.slug === "gpt-image-2" ? "Gunakan contoh produk, iklan, dan storyboard sebagai prompt yang dapat diedit." : f.slug === "kimi-k3" ? `Gunakan konteks ${context} dan input file untuk dokumen panjang, codebase, dan catatan riset.` : f.slug === "deepseek-v4-pro" ? `Rencanakan alur teks dan file berdasarkan konteks ${context} yang terverifikasi.` : "Gunakan contoh produk, UGC, dan storyboard dengan pengaturan video yang jelas.",
      boundary: (f) => f.slug === "gpt-image-2" ? "Halaman ini tidak menjanjikan output transparan, pembuatan gratis, atau harga tetap per gambar jika faktanya belum terverifikasi." : f.slug === "minimax-h3" ? "Katalog tidak memverifikasi pengiriman ComfyUI native atau bobot lokal Flatkey; halaman ini tidak menyiratkannya." : f.slug === "deepseek-v4-pro" ? "Bandingkan bidang integrasi tanpa menerbitkan klaim benchmark atau keunggulan coding yang tidak didukung." : f.slug === "kimi-k3" ? "Halaman memisahkan akses API hosted dari klaim belum terverifikasi tentang bobot unduhan atau hardware lokal." : "Pisahkan bidang katalog yang terdokumentasi dari klaim performa atau ketersediaan yang belum didukung.",
    },
  };
  // Keep these reads explicit: they document which factual fields each card is
  // allowed to mention and prevent accidental omission during refactors.
  void id; void endpoint; void second; void text;
  return templates[locale][kind](facts);
}

const WHY_SLOT_TITLES: Record<Locale, Record<WhySlot, string>> = {
  en: { routing: "Exact model routing", pricing: "Pricing stays explicit", workflow: "Workflow-specific starting points", boundary: "No unsupported promises", controls: "All documented controls", clients: "Two client paths", utc: "UTC-aware billing", textBoundary: "Text and file boundary", facts: "Model facts over rankings", account: "One account for model testing", prompting: "Prompting guide included", async: "Async result path" },
  zh: { routing: "准确的模型路由", pricing: "价格维度清晰", workflow: "贴合工作流的起点", boundary: "不做未经核实的承诺", controls: "集中展示已记录控制项", clients: "两种客户端路径", utc: "按 UTC 时段计费", textBoundary: "文本与文件边界", facts: "以模型事实为准", account: "一个账号测试模型", prompting: "附带提示词指南", async: "异步结果路径" },
  es: { routing: "Enrutamiento exacto", pricing: "Precios explícitos", workflow: "Puntos de partida por flujo", boundary: "Sin promesas no verificadas", controls: "Todos los controles documentados", clients: "Dos rutas de cliente", utc: "Facturación según UTC", textBoundary: "Límite de texto y archivos", facts: "Datos del modelo, no rankings", account: "Una cuenta para probar modelos", prompting: "Guía de prompts incluida", async: "Ruta del resultado asíncrono" },
  fr: { routing: "Routage précis", pricing: "Tarification explicite", workflow: "Départs adaptés au workflow", boundary: "Aucune promesse non vérifiée", controls: "Tous les contrôles documentés", clients: "Deux chemins client", utc: "Facturation selon UTC", textBoundary: "Limite texte et fichier", facts: "Faits du modèle, pas de classement", account: "Un compte pour tester les modèles", prompting: "Guide de prompts inclus", async: "Chemin du résultat asynchrone" },
  pt: { routing: "Roteamento exato", pricing: "Preço explícito", workflow: "Pontos de partida por fluxo", boundary: "Sem promessas não verificadas", controls: "Todos os controles documentados", clients: "Duas rotas de cliente", utc: "Cobrança por UTC", textBoundary: "Limite de texto e arquivo", facts: "Fatos do modelo, não rankings", account: "Uma conta para testar modelos", prompting: "Guia de prompts incluído", async: "Caminho do resultado assíncrono" },
  ru: { routing: "Точный роутинг модели", pricing: "Прозрачные измерения цены", workflow: "Стартовые сценарии по задачам", boundary: "Без неподтверждённых обещаний", controls: "Все документированные настройки", clients: "Два пути клиента", utc: "Тарификация по UTC", textBoundary: "Граница текста и файлов", facts: "Факты модели вместо рейтингов", account: "Один аккаунт для тестов", prompting: "Руководство по промптам", async: "Путь асинхронного результата" },
  ja: { routing: "正確なモデルルーティング", pricing: "料金項目を明確に表示", workflow: "用途別の出発点", boundary: "未確認の約束をしない", controls: "文書化された設定を集約", clients: "2 つのクライアント経路", utc: "UTC 時間帯別の課金", textBoundary: "テキストとファイルの範囲", facts: "ランキングではなく事実", account: "1 つのアカウントで検証", prompting: "プロンプトガイド付き", async: "非同期結果の経路" },
  vi: { routing: "Định tuyến đúng model", pricing: "Minh bạch từng loại giá", workflow: "Điểm bắt đầu theo quy trình", boundary: "Không hứa điều chưa xác minh", controls: "Đủ điều khiển đã ghi nhận", clients: "Hai đường dẫn client", utc: "Tính phí theo UTC", textBoundary: "Ranh giới văn bản và file", facts: "Dữ kiện model thay vì thứ hạng", account: "Một tài khoản để thử model", prompting: "Có sẵn hướng dẫn prompt", async: "Luồng kết quả bất đồng bộ" },
  de: { routing: "Exaktes Modell-Routing", pricing: "Preise bleiben nachvollziehbar", workflow: "Startpunkte für konkrete Workflows", boundary: "Keine unbestätigten Versprechen", controls: "Alle dokumentierten Steuerungen", clients: "Zwei Client-Pfade", utc: "UTC-basierte Abrechnung", textBoundary: "Grenze für Text und Dateien", facts: "Modellfakten statt Rankings", account: "Ein Konto zum Testen", prompting: "Promptleitfaden enthalten", async: "Pfad für asynchrones Ergebnis" },
  id: { routing: "Routing model yang tepat", pricing: "Harga tetap jelas", workflow: "Titik awal sesuai alur kerja", boundary: "Tanpa janji yang belum terverifikasi", controls: "Semua kontrol terdokumentasi", clients: "Dua jalur klien", utc: "Penagihan berbasis UTC", textBoundary: "Batas teks dan file", facts: "Fakta model, bukan peringkat", account: "Satu akun untuk menguji model", prompting: "Panduan prompt tersedia", async: "Alur hasil asinkron" },
};

function whySlotBody(locale: Locale, slot: WhySlot, facts: Facts): string {
  const context = facts.context ?? "1,048,576 tokens";
  const specialized: Record<Locale, Partial<Record<Exclude<WhySlot, AddonCardKind>, string>>> = {
    en: {
      controls: facts.slug === "gpt-image-2" ? "Set n, size, quality, output format, background, and moderation before you hand the request to the console." : "Choose 768P or 2K, 4–15 seconds, a supported ratio, and the AIGC watermark boolean.",
      clients: "Choose /v1/chat/completions for an OpenAI-shaped client or /v1/messages for an Anthropic-shaped client.",
      utc: "Keep peak and off-peak expressions together; do not replace the catalog rule with one blended rate.",
      textBoundary: "The verified catalog lists text and file modalities; vision and other inputs are not inferred.",
      facts: "Compare integration fields and context data without publishing an unsupported benchmark or coding-superiority claim.",
      account: "Keep API keys, limits, usage, and adjacent model experiments in the same Flatkey workspace.",
      prompting: "Product, UGC, and storyboard examples describe subject, action, camera, and ending rather than repeating a generic cinematic prompt.",
      async: "Save the task ID returned by POST /v1/videos and retrieve the generated content when the task is ready.",
    },
    zh: {
      controls: facts.slug === "gpt-image-2" ? "在请求交给控制台前，设置 n、size、quality、output format、background 和 moderation。" : "选择 768P 或 2K、4–15 秒、支持的比例，并明确设置 AIGC 水印布尔值。",
      clients: "OpenAI 形客户端选择 /v1/chat/completions，Anthropic 形客户端选择 /v1/messages。",
      utc: "同时保留高峰与非高峰表达式，不要把目录规则替换成一个混合费率。",
      textBoundary: "已核实目录列出文本和文件模态；不推断视觉或其他输入。",
      facts: "比较集成字段和上下文数据，不发布未经支持的基准或编程优势结论。",
      account: "在同一个 Flatkey 工作区管理 API Key、限制、用量和其他模型测试。",
      prompting: "产品、UGC 和分镜示例会说明主体、动作、镜头与结尾，不重复空泛的电影感提示词。",
      async: "保存 POST /v1/videos 返回的 task ID，任务完成后获取生成内容。",
    },
    es: {
      controls: facts.slug === "gpt-image-2" ? "Define n, size, quality, output format, background y moderation antes de entregar la solicitud a la consola." : "Elige 768P o 2K, 4–15 segundos, una proporción compatible y el booleano de marca de agua AIGC.",
      clients: "Elige /v1/chat/completions para un cliente con forma OpenAI o /v1/messages para uno con forma Anthropic.",
      utc: "Mantén juntas las expresiones de hora punta y fuera de punta; no sustituyas la regla del catálogo por una tarifa combinada.",
      textBoundary: "El catálogo verificado enumera texto y archivos; no se infieren visión ni otras entradas.",
      facts: "Compara campos de integración y contexto sin publicar un benchmark o una superioridad de código no respaldados.",
      account: "Mantén claves API, límites, uso y pruebas de otros modelos en el mismo espacio de Flatkey.",
      prompting: "Los ejemplos de producto, UGC y storyboard describen sujeto, acción, cámara y final, en lugar de repetir un prompt cinematográfico genérico.",
      async: "Guarda el task ID devuelto por POST /v1/videos y recupera el contenido cuando la tarea esté lista.",
    },
    fr: {
      controls: facts.slug === "gpt-image-2" ? "Définissez n, size, quality, output format, background et moderation avant de transmettre la requête à la console." : "Choisissez 768P ou 2K, 4–15 secondes, un ratio pris en charge et le booléen du filigrane AIGC.",
      clients: "Choisissez /v1/chat/completions pour un client de forme OpenAI ou /v1/messages pour un client de forme Anthropic.",
      utc: "Gardez ensemble les expressions de pointe et hors pointe ; ne remplacez pas la règle du catalogue par un tarif mixte.",
      textBoundary: "Le catalogue vérifié liste le texte et les fichiers ; la vision et les autres entrées ne sont pas déduites.",
      facts: "Comparez les champs d'intégration et le contexte sans publier de benchmark ou de supériorité en programmation non étayés.",
      account: "Conservez clés API, limites, usage et essais d'autres modèles dans le même espace Flatkey.",
      prompting: "Les exemples produit, UGC et storyboard décrivent sujet, action, caméra et fin au lieu de répéter un prompt cinématographique générique.",
      async: "Enregistrez l'ID de tâche renvoyé par POST /v1/videos et récupérez le contenu une fois la tâche prête.",
    },
    pt: {
      controls: facts.slug === "gpt-image-2" ? "Defina n, size, quality, output format, background e moderation antes de enviar a solicitação ao console." : "Escolha 768P ou 2K, 4–15 segundos, uma proporção compatível e o booleano de watermark AIGC.",
      clients: "Escolha /v1/chat/completions para um cliente no formato OpenAI ou /v1/messages para um cliente no formato Anthropic.",
      utc: "Mantenha juntas as expressões de pico e fora de pico; não substitua a regra do catálogo por uma tarifa combinada.",
      textBoundary: "O catálogo verificado lista texto e arquivos; visão e outras entradas não são inferidas.",
      facts: "Compare campos de integração e contexto sem publicar benchmark ou superioridade em código sem suporte.",
      account: "Mantenha chaves API, limites, uso e testes de outros modelos no mesmo workspace da Flatkey.",
      prompting: "Os exemplos de produto, UGC e storyboard descrevem sujeito, ação, câmera e final, sem repetir um prompt cinematográfico genérico.",
      async: "Salve o task ID retornado por POST /v1/videos e recupere o conteúdo quando a tarefa estiver pronta.",
    },
    ru: {
      controls: facts.slug === "gpt-image-2" ? "Перед передачей запроса в консоль задайте n, size, quality, output format, background и moderation." : "Выберите 768P или 2K, 4–15 секунд, поддерживаемое соотношение и явно задайте булево поле водяного знака AIGC.",
      clients: "Для клиента в формате OpenAI выберите /v1/chat/completions, для формата Anthropic — /v1/messages.",
      utc: "Сохраняйте выражения для пикового и непикового времени вместе; не заменяйте правило каталога смешанной ставкой.",
      textBoundary: "Проверенный каталог указывает текст и файлы; поддержка vision и других входов не выводится.",
      facts: "Сравнивайте поля интеграции и контекст, не публикуя неподтверждённый бенчмарк или превосходство в кодинге.",
      account: "Храните API-ключи, лимиты, использование и тесты соседних моделей в одном рабочем пространстве Flatkey.",
      prompting: "Примеры товара, UGC и раскадровки описывают объект, действие, камеру и финал, а не повторяют общий кинематографичный промпт.",
      async: "Сохраните task ID из POST /v1/videos и получите контент после готовности задачи.",
    },
    ja: {
      controls: facts.slug === "gpt-image-2" ? "コンソールに渡す前に n、size、quality、output format、background、moderation を設定します。" : "768P または 2K、4～15 秒、対応する比率、AIGC 透かしの boolean を指定します。",
      clients: "OpenAI 形式のクライアントは /v1/chat/completions、Anthropic 形式は /v1/messages を選びます。",
      utc: "ピークとオフピークの式を併記し、カタログのルールを混合料金に置き換えないでください。",
      textBoundary: "確認済みカタログが示すのはテキストとファイルです。vision や他の入力は推測しません。",
      facts: "統合項目とコンテキストを比較し、裏付けのないベンチマークやコーディング優位性を公開しません。",
      account: "API キー、制限、使用量、他モデルの検証を同じ Flatkey ワークスペースで管理できます。",
      prompting: "商品、UGC、絵コンテの例は被写体、動作、カメラ、終わり方を具体化し、一般的な映画風プロンプトを繰り返しません。",
      async: "POST /v1/videos が返す task ID を保存し、タスク完了後にコンテンツを取得します。",
    },
    vi: {
      controls: facts.slug === "gpt-image-2" ? "Đặt n, size, quality, output format, background và moderation trước khi chuyển request cho console." : "Chọn 768P hoặc 2K, 4–15 giây, tỷ lệ được hỗ trợ và đặt rõ boolean watermark AIGC.",
      clients: "Chọn /v1/chat/completions cho client dạng OpenAI hoặc /v1/messages cho client dạng Anthropic.",
      utc: "Giữ biểu thức giờ cao điểm và ngoài cao điểm cùng nhau; không thay quy tắc catalog bằng một mức giá trộn.",
      textBoundary: "Catalog đã xác minh chỉ liệt kê văn bản và file; không suy ra vision hay đầu vào khác.",
      facts: "So sánh trường tích hợp và ngữ cảnh mà không công bố benchmark hay ưu thế coding chưa có căn cứ.",
      account: "Giữ API key, giới hạn, mức dùng và thử nghiệm model khác trong cùng workspace Flatkey.",
      prompting: "Ví dụ sản phẩm, UGC và storyboard nêu rõ chủ thể, hành động, máy quay và kết thúc thay vì lặp lại prompt điện ảnh chung chung.",
      async: "Lưu task ID từ POST /v1/videos và lấy nội dung khi tác vụ hoàn tất.",
    },
    de: {
      controls: facts.slug === "gpt-image-2" ? "Setze n, size, quality, output format, background und moderation, bevor du die Anfrage an die Konsole übergibst." : "Wähle 768P oder 2K, 4–15 Sekunden, ein unterstütztes Verhältnis und das AIGC-Wasserzeichen-Boolean.",
      clients: "Wähle /v1/chat/completions für einen OpenAI-ähnlichen Client oder /v1/messages für einen Anthropic-ähnlichen Client.",
      utc: "Halte Spitzen- und Nebenzeit-Ausdrücke zusammen; ersetze die Katalogregel nicht durch einen gemischten Tarif.",
      textBoundary: "Der verifizierte Katalog nennt Text und Dateien; Vision und weitere Eingaben werden nicht abgeleitet.",
      facts: "Vergleiche Integrationsfelder und Kontext, ohne ein unbelegtes Benchmark- oder Coding-Überlegenheitsversprechen zu veröffentlichen.",
      account: "Verwalte API-Keys, Limits, Nutzung und Tests anderer Modelle im selben Flatkey-Arbeitsbereich.",
      prompting: "Produkt-, UGC- und Storyboard-Beispiele nennen Motiv, Aktion, Kamera und Schluss, statt einen allgemeinen Film-Prompt zu wiederholen.",
      async: "Speichere die von POST /v1/videos zurückgegebene Task-ID und rufe den Inhalt nach Abschluss ab.",
    },
    id: {
      controls: facts.slug === "gpt-image-2" ? "Atur n, size, quality, output format, background, dan moderation sebelum mengirim permintaan ke konsol." : "Pilih 768P atau 2K, 4–15 detik, rasio yang didukung, dan tetapkan boolean watermark AIGC.",
      clients: "Pilih /v1/chat/completions untuk klien bergaya OpenAI atau /v1/messages untuk klien bergaya Anthropic.",
      utc: "Pertahankan ekspresi jam sibuk dan di luar jam sibuk; jangan mengganti aturan katalog dengan tarif campuran.",
      textBoundary: "Katalog terverifikasi mencantumkan teks dan file; dukungan vision atau input lain tidak disimpulkan.",
      facts: "Bandingkan bidang integrasi dan konteks tanpa menerbitkan klaim benchmark atau keunggulan coding yang tidak didukung.",
      account: "Simpan API key, batas, penggunaan, dan eksperimen model lain di workspace Flatkey yang sama.",
      prompting: "Contoh produk, UGC, dan storyboard menjelaskan subjek, aksi, kamera, serta akhir, bukan mengulang prompt sinematik generik.",
      async: "Simpan task ID dari POST /v1/videos dan ambil konten setelah tugas siap.",
    },
  };
  if (slot in specialized[locale]) return specialized[locale][slot as Exclude<WhySlot, AddonCardKind>] ?? "";
  return addonBody(locale, slot as AddonCardKind, facts);
}

function buildWhy(facts: Facts, locale: Locale): NonNullable<ModelLandingContent["why"]> {
  const pack = ADDON_PACKS[locale] ?? ADDON_PACKS.en;
  // Match the editorial order used by the English priority overrides.  The
  // order is model-specific because each page answers a different search
  // intent (for example, UTC billing for DeepSeek and client paths for Kimi).
  const layout: Record<PriorityModelSlug, WhySlot[]> = {
    "gpt-5-6-sol": ["routing", "pricing", "workflow", "account"],
    "gpt-image-2": ["controls", "pricing", "workflow", "boundary"],
    "kimi-k3": ["clients", "workflow", "boundary", "pricing"],
    "deepseek-v4-pro": ["utc", "routing", "textBoundary", "facts"],
    "minimax-h3": ["controls", "prompting", "async", "boundary"],
  };
  const slots = layout[facts.slug];
  return {
    eyebrow: facts.slug === "gpt-5-6-sol" || facts.slug === "gpt-image-2" || facts.slug === "kimi-k3" || facts.slug === "deepseek-v4-pro" || facts.slug === "minimax-h3"
      ? pack.whyEyebrow
      : pack.whyEyebrow,
    title: pack.whyTitle(facts.name),
    description: pack.whyDescription(facts.name),
    cards: slots.map((slot) => ({ title: WHY_SLOT_TITLES[locale]?.[slot] ?? WHY_SLOT_TITLES.en[slot], body: whySlotBody(locale, slot, facts) })),
  };
}

function buildPromptLibrary(facts: Facts, locale: Locale): Pick<ModelLandingContent, "promptLibrary" | "promptLibraryTitle" | "promptLibraryDescription"> {
  const pack = ADDON_PACKS[locale] ?? ADDON_PACKS.en;
  const prefix = facts.slug === "gpt-image-2" ? "gpt-image-2-" : facts.slug === "minimax-h3" ? "minimax-h3-" : "";
  if (!prefix) return {};
  const keys = Object.keys(PROMPT_SOURCE).filter((key) => key.startsWith(prefix));
  return {
    promptLibraryTitle: pack.promptTitle(facts.name, facts.kind),
    promptLibraryDescription: pack.promptDescription,
    promptLibrary: keys.map((key) => ({
      key,
      label: pack.promptLabels[key] ?? PROMPT_SOURCE[key].label,
      prompt: pack.promptTexts[key] ?? PROMPT_SOURCE[key].prompt,
      poster: PROMPT_MEDIA[key].poster,
      ...(PROMPT_MEDIA[key].video ? { video: PROMPT_MEDIA[key].video } : {}),
      alt: pack.promptAlts[key] ?? PROMPT_SOURCE[key].alt,
    })),
  };
}

/** The split FAQ heading is rendered as two lines by the shared shell. */
function buildFaqTitle(facts: Facts, locale: Locale): NonNullable<ModelLandingContent["faqTitle"]> {
  const afterBreak: Record<Locale, string> = {
    en: "frequently asked questions",
    zh: "常见问题",
    es: "preguntas frecuentes",
    fr: "questions fréquentes",
    pt: "perguntas frequentes",
    ru: "часто задаваемые вопросы",
    ja: "よくある質問",
    vi: "câu hỏi thường gặp",
    de: "häufig gestellte Fragen",
    id: "pertanyaan umum",
  };
  const before = locale === "zh"
    ? `${facts.name} API`
    : locale === "es"
      ? `API de ${facts.name}`
      : locale === "fr"
        ? `API ${facts.name}`
        : locale === "pt"
          ? `API do ${facts.name}`
          : locale === "ru"
            ? `API ${facts.name}`
            : locale === "ja"
              ? `${facts.name} API`
              : locale === "vi"
                ? `API ${facts.name}`
                : locale === "de"
                  ? `${facts.name}-API`
                  : locale === "id"
                    ? `API ${facts.name}`
                    : facts.slug === "minimax-h3" ? `${facts.name} video API` : `${facts.name} API`;
  return { beforeBreak: before, afterBreak: afterBreak[locale] ?? afterBreak.en };
}

type MinimaxExtraFaq = {
  promptQuestion: string;
  promptAnswer: string;
  localQuestion: string;
  localAnswer: string;
  moderationQuestion: string;
  moderationAnswer: string;
  draftQuestion: string;
  draftAnswer: string;
};

/** Additional MiniMax-H3 questions present in the priority-page brief. */
const MINIMAX_EXTRA_FAQ: Record<Locale, MinimaxExtraFaq> = {
  en: {
    promptQuestion: "Does MiniMax-H3 have a prompting guide?",
    promptAnswer: "Use the page examples as a starting point: name the subject, action, camera movement, duration, and ending, then set ratio and resolution as request fields.",
    localQuestion: "Does Flatkey provide native MiniMax-H3 ComfyUI or local installation?",
    localAnswer: "Native Flatkey ComfyUI delivery, downloadable weights, and local hardware requirements are not verified in the current catalog.",
    moderationQuestion: "Is MiniMax-H3 censored or unrestricted?",
    moderationAnswer: "The current catalog does not publish a moderation policy for this model, so the page makes no unrestricted-use claim.",
    draftQuestion: "Are MiniMax-H3 prompt drafts executed on this public page?",
    draftAnswer: "The public page saves the video settings and prompt draft first. Sign up or open the console to run POST /v1/videos with an API key.",
  },
  zh: {
    promptQuestion: "MiniMax-H3 有提示词指南吗？",
    promptAnswer: "可将页面示例作为起点：写明主体、动作、镜头运动、时长和结尾，再把比例与分辨率作为请求字段设置。",
    localQuestion: "Flatkey 提供原生 MiniMax-H3 ComfyUI 或本地安装吗？",
    localAnswer: "当前目录未核实 Flatkey 原生 ComfyUI 交付、可下载权重或本地硬件要求。",
    moderationQuestion: "MiniMax-H3 是否受审查或不受限制？",
    moderationAnswer: "当前目录没有发布该模型的 moderation 政策，因此本页不作不受限制使用的声明。",
    draftQuestion: "MiniMax-H3 的提示词草稿会在公开页执行吗？",
    draftAnswer: "公开页会先保存视频设置和提示词草稿；注册或打开控制台后，使用 API Key 执行 POST /v1/videos。",
  },
  es: {
    promptQuestion: "¿MiniMax-H3 tiene una guía de prompts?",
    promptAnswer: "Usa los ejemplos de la página como punto de partida: indica sujeto, acción, movimiento de cámara, duración y final, y define proporción y resolución como campos de la solicitud.",
    localQuestion: "¿Flatkey ofrece ComfyUI nativo o instalación local de MiniMax-H3?",
    localAnswer: "El catálogo actual no verifica entrega nativa de ComfyUI, pesos descargables ni requisitos de hardware local.",
    moderationQuestion: "¿MiniMax-H3 está censurado o es sin restricciones?",
    moderationAnswer: "El catálogo no publica una política de moderación para este modelo, por lo que la página no afirma un uso sin restricciones.",
    draftQuestion: "¿Los borradores de prompts de MiniMax-H3 se ejecutan en esta página pública?",
    draftAnswer: "La página pública guarda primero los ajustes y el borrador; regístrate o abre la consola para ejecutar POST /v1/videos con una API key.",
  },
  fr: {
    promptQuestion: "MiniMax-H3 propose-t-il un guide de prompts ?",
    promptAnswer: "Utilisez les exemples comme point de départ : indiquez sujet, action, mouvement de caméra, durée et fin, puis définissez ratio et résolution dans les champs de requête.",
    localQuestion: "Flatkey fournit-il un ComfyUI natif ou une installation locale de MiniMax-H3 ?",
    localAnswer: "Le catalogue actuel ne vérifie ni livraison ComfyUI native, ni poids téléchargeables, ni exigences matérielles locales.",
    moderationQuestion: "MiniMax-H3 est-il censuré ou sans restriction ?",
    moderationAnswer: "Le catalogue ne publie pas de politique de modération pour ce modèle ; la page ne revendique donc pas un usage sans restriction.",
    draftQuestion: "Les brouillons de prompts MiniMax-H3 sont-ils exécutés sur cette page publique ?",
    draftAnswer: "La page publique enregistre d'abord les réglages et le brouillon ; inscrivez-vous ou ouvrez la console pour exécuter POST /v1/videos avec une clé API.",
  },
  pt: {
    promptQuestion: "O MiniMax-H3 tem um guia de prompts?",
    promptAnswer: "Use os exemplos da página como ponto de partida: descreva sujeito, ação, movimento de câmera, duração e final, e defina proporção e resolução nos campos da solicitação.",
    localQuestion: "A Flatkey oferece ComfyUI nativo ou instalação local do MiniMax-H3?",
    localAnswer: "O catálogo atual não verifica entrega nativa do ComfyUI, pesos para download ou requisitos de hardware local.",
    moderationQuestion: "O MiniMax-H3 é censurado ou sem restrições?",
    moderationAnswer: "O catálogo não publica uma política de moderação para este modelo; por isso a página não afirma uso sem restrições.",
    draftQuestion: "Os rascunhos de prompt do MiniMax-H3 são executados nesta página pública?",
    draftAnswer: "A página pública salva primeiro as configurações e o rascunho; cadastre-se ou abra o console para executar POST /v1/videos com uma API key.",
  },
  ru: {
    promptQuestion: "Есть ли у MiniMax-H3 руководство по промптам?",
    promptAnswer: "Используйте примеры страницы как основу: укажите объект, действие, движение камеры, длительность и финал, а соотношение и разрешение задайте полями запроса.",
    localQuestion: "Предоставляет ли Flatkey нативный ComfyUI или локальную установку MiniMax-H3?",
    localAnswer: "Текущий каталог не подтверждает нативную поставку ComfyUI, скачиваемые веса или требования к локальному оборудованию.",
    moderationQuestion: "MiniMax-H3 цензурируется или не имеет ограничений?",
    moderationAnswer: "Каталог не публикует политику модерации этой модели, поэтому страница не заявляет об использовании без ограничений.",
    draftQuestion: "Выполняются ли черновики промптов MiniMax-H3 на этой публичной странице?",
    draftAnswer: "Публичная страница сначала сохраняет настройки видео и черновик; зарегистрируйтесь или откройте консоль, чтобы выполнить POST /v1/videos с API-ключом.",
  },
  ja: {
    promptQuestion: "MiniMax-H3 のプロンプトガイドはありますか？",
    promptAnswer: "ページの例を出発点に、被写体、動作、カメラ移動、長さ、終わり方を記述し、比率と解像度はリクエスト項目で設定します。",
    localQuestion: "Flatkey は MiniMax-H3 のネイティブ ComfyUI やローカルインストールを提供しますか？",
    localAnswer: "現在のカタログでは、ネイティブ ComfyUI 配信、ダウンロード可能な重み、ローカル機器要件を確認できません。",
    moderationQuestion: "MiniMax-H3 は検閲対象ですか、それとも無制限ですか？",
    moderationAnswer: "カタログにこのモデルのモデレーション方針は掲載されていないため、無制限利用を主張しません。",
    draftQuestion: "MiniMax-H3 のプロンプト草稿は公開ページで実行されますか？",
    draftAnswer: "公開ページは動画設定とプロンプト草稿を先に保存します。登録またはコンソールを開き、API キーで POST /v1/videos を実行してください。",
  },
  vi: {
    promptQuestion: "MiniMax-H3 có hướng dẫn prompt không?",
    promptAnswer: "Dùng ví dụ trên trang làm điểm bắt đầu: nêu chủ thể, hành động, chuyển động máy quay, thời lượng và kết thúc, rồi đặt tỷ lệ và độ phân giải ở trường request.",
    localQuestion: "Flatkey có cung cấp ComfyUI gốc hoặc cài đặt MiniMax-H3 cục bộ không?",
    localAnswer: "Catalog hiện tại chưa xác minh việc phân phối ComfyUI gốc, trọng số tải xuống hay yêu cầu phần cứng cục bộ.",
    moderationQuestion: "MiniMax-H3 có bị kiểm duyệt hay không giới hạn không?",
    moderationAnswer: "Catalog không công bố chính sách moderation cho model này, nên trang không tuyên bố sử dụng không giới hạn.",
    draftQuestion: "Bản nháp prompt MiniMax-H3 có được chạy trên trang công khai này không?",
    draftAnswer: "Trang công khai chỉ lưu thiết lập video và bản nháp trước; hãy đăng ký hoặc mở console để chạy POST /v1/videos bằng API key.",
  },
  de: {
    promptQuestion: "Gibt es für MiniMax-H3 einen Prompt-Leitfaden?",
    promptAnswer: "Nutze die Beispiele als Ausgangspunkt: Motiv, Aktion, Kamerabewegung, Dauer und Schluss nennen, anschließend Verhältnis und Auflösung als Anfragefelder setzen.",
    localQuestion: "Bietet Flatkey natives MiniMax-H3-ComfyUI oder eine lokale Installation?",
    localAnswer: "Der aktuelle Katalog bestätigt weder native ComfyUI-Bereitstellung noch herunterladbare Gewichte oder lokale Hardwareanforderungen.",
    moderationQuestion: "Ist MiniMax-H3 zensiert oder uneingeschränkt?",
    moderationAnswer: "Der Katalog veröffentlicht keine Moderationsrichtlinie für dieses Modell; die Seite behauptet daher keine uneingeschränkte Nutzung.",
    draftQuestion: "Werden MiniMax-H3-Promptentwürfe auf dieser öffentlichen Seite ausgeführt?",
    draftAnswer: "Die öffentliche Seite speichert Videoeinstellungen und Promptentwurf zuerst. Registriere dich oder öffne die Konsole, um POST /v1/videos mit einem API-Key auszuführen.",
  },
  id: {
    promptQuestion: "Apakah MiniMax-H3 memiliki panduan prompt?",
    promptAnswer: "Gunakan contoh di halaman sebagai titik awal: sebutkan subjek, aksi, gerakan kamera, durasi, dan akhir, lalu atur rasio serta resolusi sebagai bidang permintaan.",
    localQuestion: "Apakah Flatkey menyediakan ComfyUI native atau instalasi lokal MiniMax-H3?",
    localAnswer: "Katalog saat ini tidak memverifikasi pengiriman ComfyUI native, bobot yang dapat diunduh, atau kebutuhan hardware lokal.",
    moderationQuestion: "Apakah MiniMax-H3 disensor atau tanpa batasan?",
    moderationAnswer: "Katalog tidak menerbitkan kebijakan moderasi model ini, sehingga halaman tidak mengklaim penggunaan tanpa batasan.",
    draftQuestion: "Apakah draf prompt MiniMax-H3 dijalankan di halaman publik ini?",
    draftAnswer: "Halaman publik terlebih dahulu menyimpan pengaturan video dan draf prompt. Daftar atau buka konsol untuk menjalankan POST /v1/videos dengan API key.",
  },
};

const SEO: Record<PriorityModelSlug, Record<Locale, PriorityModelSeo>> = {
  "gpt-5-6-sol": {
    en: { title: "GPT-5.6 Sol API and pricing | Flatkey", description: "Use GPT-5.6 Sol through Flatkey with OpenAI-compatible API access, a 1,048,576-token context, text/image/file modalities, current token pricing, and one API key." },
    zh: { title: "GPT-5.6 Sol API 与价格 | Flatkey", description: "通过 Flatkey 使用 GPT-5.6 Sol，获得 OpenAI 兼容 API、1,048,576 token 上下文、文本/图片/文件模态、当前 token 价格和一个 API Key。" },
    es: { title: "API y precios de GPT-5.6 Sol | Flatkey", description: "Usa GPT-5.6 Sol con la API compatible con OpenAI de Flatkey, contexto de 1.048.576 tokens, texto/imagen/archivo, precios actuales y una sola API key." },
    fr: { title: "API et tarifs de GPT-5.6 Sol | Flatkey", description: "Utilisez GPT-5.6 Sol via l'API compatible OpenAI de Flatkey, avec contexte de 1 048 576 tokens, texte/image/fichier, tarifs actuels et une seule clé API." },
    pt: { title: "API e preços do GPT-5.6 Sol | Flatkey", description: "Use o GPT-5.6 Sol pela API compatível com OpenAI da Flatkey, com contexto de 1.048.576 tokens, texto/imagem/arquivo, preços atuais e uma única API key." },
    ru: { title: "GPT-5.6 Sol: API и цены | Flatkey", description: "Используйте GPT-5.6 Sol через совместимый с OpenAI API Flatkey: контекст 1 048 576 токенов, текст/изображения/файлы, текущие цены и один API-ключ." },
    ja: { title: "GPT-5.6 Sol API と料金 | Flatkey", description: "Flatkey の OpenAI 互換 API で GPT-5.6 Sol を利用。1,048,576 トークンのコンテキスト、テキスト/画像/ファイル入力、現在の料金、1 つの API キーに対応。" },
    vi: { title: "API và giá GPT-5.6 Sol | Flatkey", description: "Dùng GPT-5.6 Sol qua API tương thích OpenAI của Flatkey với ngữ cảnh 1.048.576 token, text/image/file, giá token hiện tại và một API key." },
    de: { title: "GPT-5.6 Sol API und Preise | Flatkey", description: "Nutze GPT-5.6 Sol über die OpenAI-kompatible Flatkey-API mit 1.048.576-Token-Kontext, Text/Bild/Datei, aktuellen Tokenpreisen und einem API-Key." },
    id: { title: "API dan harga GPT-5.6 Sol | Flatkey", description: "Gunakan GPT-5.6 Sol melalui API kompatibel OpenAI Flatkey dengan konteks 1.048.576 token, modalitas teks/gambar/file, harga token terbaru, dan satu API key." },
  },
  "gpt-image-2": {
    en: { title: "GPT Image 2 API and image generator | Flatkey", description: "Prepare GPT Image 2 requests through Flatkey with image API access, current token-dimension pricing, size and quality controls, formats, background, and moderation settings." },
    zh: { title: "GPT Image 2 API 与图像生成器 | Flatkey", description: "通过 Flatkey 配置 GPT Image 2 请求，使用图像 API、当前 token 维度价格、尺寸与质量控制、格式、背景和 moderation 设置。" },
    es: { title: "API y generador de imágenes GPT Image 2 | Flatkey", description: "Prepara solicitudes de GPT Image 2 con Flatkey: API de imágenes, precios por dimensión de token, controles de tamaño y calidad, formatos, fondo y moderación." },
    fr: { title: "API et générateur d'images GPT Image 2 | Flatkey", description: "Préparez vos requêtes GPT Image 2 avec Flatkey : API d'images, tarifs par dimension de token, tailles, qualité, formats, arrière-plan et modération." },
    pt: { title: "API e gerador de imagens do GPT Image 2 | Flatkey", description: "Prepare solicitações do GPT Image 2 pela Flatkey com API de imagem, preços por dimensão de token, controles de tamanho e qualidade, formatos, fundo e moderação." },
    ru: { title: "GPT Image 2: API и генератор изображений | Flatkey", description: "Настройте запросы GPT Image 2 через Flatkey: API изображений, цены по измерениям токенов, размеры, качество, форматы, фон и модерация." },
    ja: { title: "GPT Image 2 API と画像ジェネレーター | Flatkey", description: "Flatkey で GPT Image 2 のリクエストを準備。画像 API、トークン単位の料金、サイズ・品質・形式、背景、モデレーションを設定できます。" },
    vi: { title: "API và trình tạo ảnh GPT Image 2 | Flatkey", description: "Chuẩn bị yêu cầu GPT Image 2 qua Flatkey với API hình ảnh, giá theo token, điều khiển kích thước/chất lượng, định dạng, nền và moderation." },
    de: { title: "GPT Image 2 API und Bildgenerator | Flatkey", description: "Bereite GPT Image 2-Anfragen über Flatkey vor: Bild-API, Token-Dimensionspreise, Größen- und Qualitätssteuerung, Formate, Hintergrund und Moderation." },
    id: { title: "API dan generator gambar GPT Image 2 | Flatkey", description: "Siapkan permintaan GPT Image 2 melalui Flatkey dengan API gambar, harga berdasarkan dimensi token, kontrol ukuran/kualitas, format, latar, dan moderasi." },
  },
  "kimi-k3": {
    en: { title: "Kimi K3 API and pricing | Flatkey", description: "Use Moonshot AI's Kimi K3 through OpenAI- and Anthropic-compatible endpoints with a 1,048,576-token context, file input, and current token pricing." },
    zh: { title: "Kimi K3 API 与价格 | Flatkey", description: "通过 Flatkey 的 OpenAI 与 Anthropic 兼容 endpoint 使用 Moonshot AI Kimi K3，支持 1,048,576 token 上下文、文件输入和当前 token 价格。" },
    es: { title: "API y precios de Kimi K3 | Flatkey", description: "Usa Kimi K3 de Moonshot AI mediante endpoints compatibles con OpenAI y Anthropic, contexto de 1.048.576 tokens, entrada de archivos y precios actuales." },
    fr: { title: "API et tarifs de Kimi K3 | Flatkey", description: "Utilisez Kimi K3 de Moonshot AI via des endpoints compatibles OpenAI et Anthropic, avec contexte de 1 048 576 tokens, fichiers et tarifs actuels." },
    pt: { title: "API e preços do Kimi K3 | Flatkey", description: "Use o Kimi K3 da Moonshot AI por endpoints compatíveis com OpenAI e Anthropic, com contexto de 1.048.576 tokens, entrada de arquivos e preços atuais." },
    ru: { title: "Kimi K3: API и цены | Flatkey", description: "Используйте Kimi K3 от Moonshot AI через совместимые с OpenAI и Anthropic endpoint: контекст 1 048 576 токенов, файлы и текущие цены." },
    ja: { title: "Kimi K3 API と料金 | Flatkey", description: "Moonshot AI の Kimi K3 を Flatkey の OpenAI/Anthropic 互換エンドポイントで利用。1,048,576 トークンのコンテキスト、ファイル入力、現在の料金に対応。" },
    vi: { title: "API và giá Kimi K3 | Flatkey", description: "Dùng Kimi K3 của Moonshot AI qua endpoint tương thích OpenAI và Anthropic với ngữ cảnh 1.048.576 token, đầu vào file và giá hiện tại." },
    de: { title: "Kimi K3 API und Preise | Flatkey", description: "Nutze Moonshot AIs Kimi K3 über OpenAI- und Anthropic-kompatible Endpoints mit 1.048.576-Token-Kontext, Dateieingabe und aktuellen Tokenpreisen." },
    id: { title: "API dan harga Kimi K3 | Flatkey", description: "Gunakan Kimi K3 dari Moonshot AI melalui endpoint kompatibel OpenAI dan Anthropic dengan konteks 1.048.576 token, input file, dan harga token terbaru." },
  },
  "deepseek-v4-pro": {
    en: { title: "DeepSeek V4 Pro API and dynamic pricing | Flatkey", description: "Call DeepSeek V4 Pro through OpenAI-compatible or Anthropic-compatible endpoints with a 1,048,576-token context, file input, and UTC time-tiered pricing." },
    zh: { title: "DeepSeek V4 Pro API 与动态价格 | Flatkey", description: "通过 Flatkey 的 OpenAI 或 Anthropic 兼容 endpoint 调用 DeepSeek V4 Pro，支持 1,048,576 token 上下文、文件输入和按 UTC 时段变化的价格。" },
    es: { title: "API y precios dinámicos de DeepSeek V4 Pro | Flatkey", description: "Llama a DeepSeek V4 Pro mediante endpoints compatibles con OpenAI o Anthropic, con contexto de 1.048.576 tokens, archivos y precios por franja UTC." },
    fr: { title: "API et tarification dynamique de DeepSeek V4 Pro | Flatkey", description: "Appelez DeepSeek V4 Pro via des endpoints compatibles OpenAI ou Anthropic, avec contexte de 1 048 576 tokens, fichiers et tarifs par heure UTC." },
    pt: { title: "API e preços dinâmicos do DeepSeek V4 Pro | Flatkey", description: "Chame o DeepSeek V4 Pro por endpoints compatíveis com OpenAI ou Anthropic, com contexto de 1.048.576 tokens, entrada de arquivos e preços por faixa UTC." },
    ru: { title: "DeepSeek V4 Pro: API и динамические цены | Flatkey", description: "Вызывайте DeepSeek V4 Pro через совместимые с OpenAI или Anthropic endpoint: контекст 1 048 576 токенов, файлы и тарифы по времени UTC." },
    ja: { title: "DeepSeek V4 Pro API と時間帯別料金 | Flatkey", description: "Flatkey の OpenAI/Anthropic 互換エンドポイントで DeepSeek V4 Pro を呼び出せます。1,048,576 トークンのコンテキスト、ファイル入力、UTC 時間帯別料金に対応。" },
    vi: { title: "API và giá theo thời gian DeepSeek V4 Pro | Flatkey", description: "Gọi DeepSeek V4 Pro qua endpoint tương thích OpenAI hoặc Anthropic với ngữ cảnh 1.048.576 token, đầu vào file và giá theo khung giờ UTC." },
    de: { title: "DeepSeek V4 Pro API und zeitabhängige Preise | Flatkey", description: "Rufe DeepSeek V4 Pro über OpenAI- oder Anthropic-kompatible Endpoints auf: 1.048.576-Token-Kontext, Dateieingabe und UTC-Zeittarife." },
    id: { title: "API dan harga dinamis DeepSeek V4 Pro | Flatkey", description: "Panggil DeepSeek V4 Pro melalui endpoint kompatibel OpenAI atau Anthropic dengan konteks 1.048.576 token, input file, dan harga bertingkat waktu UTC." },
  },
  "minimax-h3": {
    en: { title: "MiniMax-H3 video generator API and pricing | Flatkey", description: "Configure MiniMax-H3 video requests through Flatkey with 768P or 2K resolution, duration, ratio, AIGC watermark, and current per-second pricing." },
    zh: { title: "MiniMax-H3 视频生成器 API 与价格 | Flatkey", description: "通过 Flatkey 配置 MiniMax-H3 视频请求，支持 768P 或 2K 分辨率、时长、比例、AIGC 水印和当前每秒价格。" },
    es: { title: "API y precios del generador de vídeo MiniMax-H3 | Flatkey", description: "Configura solicitudes de vídeo MiniMax-H3 con Flatkey: resolución 768P o 2K, duración, proporción, marca de agua AIGC y precios por segundo." },
    fr: { title: "API et tarifs du générateur vidéo MiniMax-H3 | Flatkey", description: "Configurez les requêtes vidéo MiniMax-H3 avec Flatkey : résolution 768P ou 2K, durée, ratio, filigrane AIGC et tarifs actuels à la seconde." },
    pt: { title: "API e preços do gerador de vídeo MiniMax-H3 | Flatkey", description: "Configure solicitações de vídeo MiniMax-H3 pela Flatkey com resolução 768P ou 2K, duração, proporção, marca-d'água AIGC e preço por segundo." },
    ru: { title: "MiniMax-H3: API видеогенератора и цены | Flatkey", description: "Настройте видеозапросы MiniMax-H3 через Flatkey: разрешение 768P или 2K, длительность, соотношение, водяной знак AIGC и цены за секунду." },
    ja: { title: "MiniMax-H3 動画生成 API と料金 | Flatkey", description: "Flatkey で MiniMax-H3 の動画リクエストを設定。768P/2K、長さ、比率、AIGC 透かし、現在の秒単位料金を確認できます。" },
    vi: { title: "API và giá trình tạo video MiniMax-H3 | Flatkey", description: "Cấu hình yêu cầu video MiniMax-H3 qua Flatkey với độ phân giải 768P hoặc 2K, thời lượng, tỷ lệ, watermark AIGC và giá theo giây." },
    de: { title: "MiniMax-H3 Video-Generator-API und Preise | Flatkey", description: "Konfiguriere MiniMax-H3-Videoanfragen über Flatkey mit 768P oder 2K, Dauer, Seitenverhältnis, AIGC-Wasserzeichen und aktuellen Preisen pro Sekunde." },
    id: { title: "API dan harga generator video MiniMax-H3 | Flatkey", description: "Konfigurasikan permintaan video MiniMax-H3 melalui Flatkey dengan resolusi 768P atau 2K, durasi, rasio, watermark AIGC, dan harga per detik." },
  },
};

function localizedRateLabel(label: string, pack: LanguagePack, locale: Locale): string {
  const labelMap: Record<string, string> = {
    Input: pack.input,
    Output: pack.output,
    "Cache read": pack.cacheRead,
    "Cache creation": pack.cacheCreation,
    Cache: pack.cache,
    "Input tokens": pack.inputTokens,
    "Output tokens": pack.outputTokens,
    "Cache tokens": pack.cacheTokens,
    "Image dimensions": pack.imageDimensions,
    "Peak (UTC)": pack.peakUtc,
    "Off-peak (UTC)": pack.offPeakUtc,
    "Input image after free tier": `${pack.input} ${literal(locale, "image after free tier")}`,
    "Catalog base": literal(locale, "Catalog base"),
    "Reference video": literal(locale, "Reference video"),
    "Input image": literal(locale, "Input image"),
  };
  return labelMap[label] ?? label;
}

function ratesText(facts: Facts, pack: LanguagePack, locale: Locale): string {
  return facts.rates.map((rate) => `${localizedRateLabel(rate.label, pack, locale)} ${rate.value}`).join(", ");
}

/**
 * Translate the compound pricing dimension used by DeepSeek's tiered rows.
 * The slash-separated values are kept in the same order as the catalog, but
 * the labels and unit phrase should still read naturally in each locale.
 */
function localizedRateDetail(detail: string, pack: LanguagePack, locale: Locale): string {
  if (detail === "input / cache read / output per 1M tokens") {
    const dimensions = ["Input", "Cache read", "Output"]
      .map((label) => localizedRateLabel(label, pack, locale))
      .join(" / ");
    if (locale === "zh") return `${dimensions}，${pack.perMillionTokens}`;
    if (locale === "ja") return `${dimensions}（${pack.perMillionTokens}）`;
    return `${dimensions} ${pack.perMillionTokens}`;
  }
  return detail
    .replace("per 1M tokens", pack.perMillionTokens)
    .replace("per 1M units", pack.perMillionUnits)
    .replace("per second", pack.perSecond)
    .replace("per image", literal(locale, "per image"))
    .replace("resolution-dependent", literal(locale, "resolution-dependent"))
    .replace("input-video seconds and resolution", literal(locale, "input-video seconds and resolution"))
    .replace("free allowance and later images may differ", literal(locale, "free allowance and later images may differ"));
}

function localizedRateRows(facts: Facts, pack: LanguagePack, locale: Locale) {
  return facts.rates.map((rate) => ({
    label: localizedRateLabel(rate.label, pack, locale),
    value: localizedRateValue(rate.value, locale),
    detail: localizedRateDetail(rate.detail, pack, locale),
  }));
}

function localizedRoutePair(locale: Locale, endpoint: string, secondEndpoint: string): string {
  const conjunctions: Record<Locale, string> = {
    en: "and",
    zh: "与",
    es: "y",
    fr: "et",
    pt: "e",
    ru: "и",
    ja: "と",
    vi: "và",
    de: "und",
    id: "dan",
  };
  return `${endpoint} ${conjunctions[locale] ?? conjunctions.en} ${secondEndpoint}`;
}

function localizedModalities(locale: Locale, facts: Facts): string {
  if (facts.slug === "gpt-5-6-sol") return literal(locale, "text, image, and file");
  if (facts.slug === "kimi-k3" || facts.slug === "deepseek-v4-pro") return literal(locale, "text and file");
  return facts.modalities;
}

function localizedModalitiesTable(locale: Locale, facts: Facts): string {
  if (facts.slug === "gpt-5-6-sol") return literal(locale, "Text, image, file");
  if (facts.slug === "kimi-k3" || facts.slug === "deepseek-v4-pro") return literal(locale, "Text/file fields");
  return facts.modalities;
}

const MINIMAX_REFERENCE_BODY: Record<Locale, string> = {
  en: "Catalog metadata lists text, image, video, and audio modalities; use only reference fields accepted by the selected route.",
  zh: "目录元数据列出文本、图片、视频和音频模态；只使用所选路由接受的参考字段。",
  es: "Los metadatos del catálogo enumeran texto, imagen, vídeo y audio; usa solo los campos de referencia aceptados por la ruta elegida.",
  fr: "Les métadonnées du catalogue listent texte, image, vidéo et audio ; utilisez uniquement les champs de référence acceptés par la route choisie.",
  pt: "Os metadados do catálogo listam texto, imagem, vídeo e áudio; use apenas os campos de referência aceitos pela rota escolhida.",
  ru: "Метаданные каталога указывают текст, изображения, видео и аудио; используйте только референсные поля, поддерживаемые выбранным маршрутом.",
  ja: "カタログのメタデータにはテキスト・画像・動画・音声が記載されています。選択した経路が受け付ける参照項目だけを使用します。",
  vi: "Metadata catalog liệt kê văn bản, hình ảnh, video và âm thanh; chỉ dùng trường tham chiếu được route đã chọn chấp nhận.",
  de: "Die Katalogmetadaten nennen Text, Bild, Video und Audio; verwende nur Referenzfelder, die der gewählte Pfad akzeptiert.",
  id: "Metadata katalog mencantumkan teks, gambar, video, dan audio; gunakan hanya bidang referensi yang diterima rute terpilih.",
};

/**
 * MiniMax-H3's fifth capability is deliberately about reference-aware
 * requests, not a generic "input modality" card.  Keep the title and the
 * comparison labels aligned with the English editorial override so the
 * source→locale resolver never pairs an unrelated string by array index.
 */
const MINIMAX_REFERENCE_TITLE: Record<Locale, string> = {
  en: "Reference-aware requests",
  zh: "支持参考素材的请求",
  es: "Solicitudes con referencias",
  fr: "Requêtes avec références",
  pt: "Solicitações com referências",
  ru: "Запросы с референсами",
  ja: "参照素材を使うリクエスト",
  vi: "Yêu cầu có dữ liệu tham chiếu",
  de: "Anfragen mit Referenzmaterial",
  id: "Permintaan dengan referensi",
};

const MINIMAX_COMPARISON_BASELINE: Record<Locale, string> = {
  en: "Unverified local assumption",
  zh: "未核实的本地假设",
  es: "Supuesto local no verificado",
  fr: "Hypothèse locale non vérifiée",
  pt: "Suposição local não verificada",
  ru: "Неподтверждённое локальное предположение",
  ja: "未確認のローカル前提",
  vi: "Giả định cục bộ chưa xác minh",
  de: "Unbestätigte lokale Annahme",
  id: "Asumsi lokal yang belum diverifikasi",
};

const MINIMAX_COMPARISON_CURRENT: Record<Locale, string> = {
  en: "Verified hosted fields",
  zh: "已核实的托管字段",
  es: "Campos alojados verificados",
  fr: "Champs hébergés vérifiés",
  pt: "Campos hospedados verificados",
  ru: "Подтверждённые поля хостинга",
  ja: "確認済みのホスト項目",
  vi: "Trường hosted đã xác minh",
  de: "Verifizierte gehostete Felder",
  id: "Bidang hosted terverifikasi",
};

const MINIMAX_COMPARISON_TITLE: Record<Locale, string> = {
  en: "MiniMax-H3 ComfyUI and local setup: verified boundaries",
  zh: "MiniMax-H3 ComfyUI 与本地设置：已核实边界",
  es: "MiniMax-H3 ComfyUI y configuración local: límites verificados",
  fr: "MiniMax-H3 : ComfyUI et configuration locale, limites vérifiées",
  pt: "MiniMax-H3 ComfyUI e configuração local: limites verificados",
  ru: "MiniMax-H3: ComfyUI и локальная настройка — подтверждённые границы",
  ja: "MiniMax-H3 ComfyUI とローカル設定：確認済みの範囲",
  vi: "MiniMax-H3 ComfyUI và thiết lập cục bộ: ranh giới đã xác minh",
  de: "MiniMax-H3 ComfyUI und lokale Einrichtung: verifizierte Grenzen",
  id: "MiniMax-H3 ComfyUI dan penyiapan lokal: batas terverifikasi",
};

const MINIMAX_COMPARISON_RATIO: Record<Locale, string> = {
  en: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16, adaptive",
  zh: "21:9、16:9、4:3、1:1、3:4、9:16、自适应",
  es: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16 o adaptable",
  fr: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16 ou adaptatif",
  pt: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16 ou adaptável",
  ru: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16 или адаптивное",
  ja: "21:9、16:9、4:3、1:1、3:4、9:16、アダプティブ",
  vi: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16 hoặc thích ứng",
  de: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16 oder adaptiv",
  id: "21:9, 16:9, 4:3, 1:1, 3:4, 9:16, atau adaptif",
};

const MINIMAX_EXPLICIT_BOOLEAN: Record<Locale, string> = {
  en: "Explicit boolean",
  zh: "明确的布尔值",
  es: "Booleano explícito",
  fr: "Booléen explicite",
  pt: "Booleano explícito",
  ru: "Явное логическое значение",
  ja: "明示的な boolean",
  vi: "Boolean rõ ràng",
  de: "Expliziter Boolean",
  id: "Boolean eksplisit",
};

const MINIMAX_OFF: Record<Locale, string> = {
  en: "Off",
  zh: "关闭",
  es: "Desactivado",
  fr: "Désactivé",
  pt: "Desativado",
  ru: "Выкл.",
  ja: "オフ",
  vi: "Tắt",
  de: "Aus",
  id: "Nonaktif",
};

function localizedRateValue(value: string, locale: Locale): string {
  return literal(locale, value);
}

function localizedUnknownHere(locale: Locale, pack: LanguagePack): string {
  return literal(locale, "Unknown here") || `${pack.unknown}`;
}

function localizedGptFacts(locale: Locale, facts: Facts, modalities: string): string {
  const context = facts.context ?? "1,048,576 tokens";
  switch (locale) {
    case "zh": return `${context} 上下文和${modalities}模态`;
    case "es": return `un contexto de ${context} y modalidades de ${modalities}`;
    case "fr": return `un contexte de ${context} et les modalités ${modalities}`;
    case "pt": return `um contexto de ${context} e modalidades ${modalities}`;
    case "ru": return `контекст ${context} и модальности: ${modalities}`;
    case "ja": return `${context} のコンテキストと${modalities}のモダリティ`;
    case "vi": return `ngữ cảnh ${context} và các phương thức ${modalities}`;
    case "de": return `ein Kontext von ${context} sowie die Modalitäten ${modalities}`;
    case "id": return `konteks ${context} dan modalitas ${modalities}`;
    default: return `${context.replace(" tokens", "-token")} context and ${modalities} modalities`;
  }
}

function localizedImageApiDescription(locale: Locale): string {
  const descriptions: Record<Locale, string> = {
    en: "Send the model ID with the supported image controls.",
    zh: "将模型 ID 与支持的图像控制项一同发送。",
    es: "Envía el ID del modelo con los controles de imagen compatibles.",
    fr: "Envoyez l'ID du modèle avec les contrôles d'image pris en charge.",
    pt: "Envie o ID do modelo com os controles de imagem compatíveis.",
    ru: "Передавайте ID модели вместе с поддерживаемыми настройками изображения.",
    ja: "モデル ID と対応する画像設定を一緒に送信します。",
    vi: "Gửi ID model cùng các điều khiển hình ảnh được hỗ trợ.",
    de: "Sende die Modell-ID zusammen mit den unterstützten Bildsteuerungen.",
    id: "Kirim ID model bersama kontrol gambar yang didukung.",
  };
  return descriptions[locale] ?? descriptions.en;
}

/**
 * Build section headings around the keyword cluster without forcing English
 * word order onto translated pages.  The model name and API/pricing intent
 * stay contiguous, while each locale gets a heading a native reader would
 * actually use as a search result or scan target.
 */
function localizedTextApiTitle(facts: Facts, pack: LanguagePack, locale: Locale): string {
  switch (locale) {
    case "zh": return `调用 ${facts.name} API`;
    case "es": return `API de ${facts.name}`;
    case "fr": return `API de ${facts.name}`;
    case "pt": return `Chame a API do ${facts.name}`;
    case "ru": return `Вызов API ${facts.name}`;
    case "ja": return `${facts.name} API の利用`;
    case "vi": return `Gọi API ${facts.name}`;
    case "de": return `${facts.name}-API aufrufen`;
    case "id": return `Panggil API ${facts.name}`;
    default: return `Call the ${facts.name} API`;
  }
}

type PricingHeadingVariant = "standard" | "utc" | "tokenDimensions";

function localizedPricingTitle(facts: Facts, pack: LanguagePack, locale: Locale, variant: PricingHeadingVariant): string {
  if (locale === "pt") {
    if (variant === "utc") return `Preços do ${facts.name} por faixa de horário UTC`;
    if (variant === "tokenDimensions") return `Preços do ${facts.name} por token e dimensões da imagem`;
    return `Preços do ${facts.name}`;
  }
  if (locale === "es") {
    if (variant === "utc") return `Precios de ${facts.name} por franja UTC`;
    if (variant === "tokenDimensions") return `Precios de ${facts.name} por token y dimensiones de imagen`;
    return `Precios de ${facts.name}`;
  }
  if (locale === "fr") {
    if (variant === "utc") return `Tarifs de ${facts.name} par plage horaire UTC`;
    if (variant === "tokenDimensions") return `Tarifs de ${facts.name} par token et dimensions d'image`;
    return `Tarifs de ${facts.name}`;
  }
  if (locale === "de") {
    if (variant === "utc") return `${facts.name}-Preise nach UTC-Zeitfenster`;
    if (variant === "tokenDimensions") return `${facts.name}-Preise nach Token und Bilddimensionen`;
    return `${facts.name}-Preise`;
  }
  if (locale === "ru") {
    if (variant === "utc") return `Цены ${facts.name} по временным зонам UTC`;
    if (variant === "tokenDimensions") return `Цены ${facts.name} по токенам и размерам изображения`;
    return `Цены ${facts.name}`;
  }
  if (locale === "ja") {
    if (variant === "utc") return `${facts.name} の UTC 時間帯別料金`;
    if (variant === "tokenDimensions") return `${facts.name} のトークン・画像サイズ別料金`;
    return `${facts.name} の料金`;
  }
  if (locale === "vi") {
    if (variant === "utc") return `Giá ${facts.name} theo khung giờ UTC`;
    if (variant === "tokenDimensions") return `Giá ${facts.name} theo token và kích thước ảnh`;
    return `Giá ${facts.name}`;
  }
  if (locale === "id") {
    if (variant === "utc") return `Harga ${facts.name} berdasarkan waktu UTC`;
    if (variant === "tokenDimensions") return `Harga ${facts.name} berdasarkan token dan dimensi gambar`;
    return `Harga ${facts.name}`;
  }
  if (locale === "zh") {
    if (variant === "utc") return `${facts.name} 按 UTC 时段计价`;
    if (variant === "tokenDimensions") return `${facts.name} 按 token 与图像尺寸计价`;
    return `${facts.name} 价格`;
  }
  if (variant === "utc") return `${facts.name} ${pack.pricingByUtc}`;
  if (variant === "tokenDimensions") return `${facts.name} ${pack.pricingByTokenAndDimensions}`;
  return `${facts.name} ${pack.pricing}`;
}

function localizedImageApiTitle(facts: Facts, pack: LanguagePack, locale: Locale): string {
  switch (locale) {
    case "zh": return `${facts.name} API endpoint 与请求设置`;
    case "es": return `Endpoint de la API y configuración de la solicitud de ${facts.name}`;
    case "fr": return `Endpoint API et configuration de requête de ${facts.name}`;
    case "pt": return `Endpoint da API e configuração da solicitação do ${facts.name}`;
    case "ru": return `Endpoint API ${facts.name} и настройка запроса`;
    case "ja": return `${facts.name} API のエンドポイントとリクエスト設定`;
    case "vi": return `Endpoint API và cấu hình yêu cầu ${facts.name}`;
    case "de": return `${facts.name}-API: Endpoint und Anfragekonfiguration`;
    case "id": return `Endpoint API dan pengaturan permintaan ${facts.name}`;
    default: return `${facts.name} ${pack.imageApiSetup}`;
  }
}

function localizedVideoApiTitle(facts: Facts, pack: LanguagePack, locale: Locale): string {
  switch (locale) {
    case "zh": return `${facts.name} 视频 API 设置`;
    case "es": return `Configuración de la API de vídeo de ${facts.name}`;
    case "fr": return `Configuration de l'API vidéo de ${facts.name}`;
    case "pt": return `Configuração da API de vídeo do ${facts.name}`;
    case "ru": return `Настройка видео API ${facts.name}`;
    case "ja": return `${facts.name} 動画 API の設定`;
    case "vi": return `Cấu hình API video ${facts.name}`;
    case "de": return `${facts.name}-Video-API konfigurieren`;
    case "id": return `Konfigurasi API video ${facts.name}`;
    default: return `${facts.name} ${pack.videoApiSetup}`;
  }
}

/**
 * Keep capability headings grammatical in the locale that is currently the
 * Brazil focus.  The model name remains adjacent to the relevant intent
 * terms, but Portuguese uses a prepositional phrase rather than an English
 * noun-stack such as "GPT Image 2 tamanhos".
 */
function localizedCapabilitiesTitle(facts: Facts, pack: LanguagePack, locale: Locale): string {
  if (locale === "pt") {
    if (facts.kind === "image") return `Tamanhos, qualidade, formatos e controles do ${facts.name}`;
    if (facts.kind === "video") return `Resolução, duração, proporção e marca-d'água do ${facts.name}`;
    if (facts.slug === "gpt-5-6-sol") return `Contexto, modalidades e endpoint do ${facts.name}`;
    return `Contexto, entrada de arquivos e endpoints compatíveis do ${facts.name}`;
  }
  if (facts.kind === "image") return `${facts.name} ${pack.imageControls}`;
  if (facts.kind === "video") return `${facts.name} ${pack.videoControls}`;
  return `${facts.name} ${facts.slug === "gpt-5-6-sol" ? pack.contextModalitiesEndpoint : pack.fileEndpoints}`;
}

const MINIMAX_DURATION_RANGE: Record<Locale, string> = {
  en: "4–15 seconds",
  zh: "4–15 秒",
  es: "4–15 segundos",
  fr: "4–15 secondes",
  pt: "4–15 segundos",
  ru: "4–15 секунд",
  ja: "4～15 秒",
  vi: "4–15 giây",
  de: "4–15 Sekunden",
  id: "4–15 detik",
};

const MINIMAX_RESOLUTION_RANGE: Record<Locale, string> = {
  en: "768P or 2K",
  zh: "768P 或 2K",
  es: "768P o 2K",
  fr: "768P ou 2K",
  pt: "768P ou 2K",
  ru: "768P или 2K",
  ja: "768P または 2K",
  vi: "768P hoặc 2K",
  de: "768P oder 2K",
  id: "768P atau 2K",
};

function localizedComparisonTitle(locale: Locale, facts: Facts): string {
  const titles: Record<Locale, string> = {
    en: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} vs GPT-5.5, Terra, and Luna`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} compared with GPT Image 1`
        : facts.slug === "kimi-k3"
          ? `${facts.name} hosted API vs local or open-source assumptions`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} vs V4 Flash: compare documented fields`
            : MINIMAX_COMPARISON_TITLE.en,
    zh: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} 与 GPT-5.5、Terra、Luna 对比`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} 与 GPT Image 1 的迁移字段对比`
        : facts.slug === "kimi-k3"
          ? `${facts.name} 托管 API 与本地/开源假设对比`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} 与 V4 Flash 的已记录字段对比`
            : MINIMAX_COMPARISON_TITLE.zh,
    es: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} frente a GPT-5.5, Terra y Luna`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} frente a GPT Image 1: campos de migración`
        : facts.slug === "kimi-k3"
          ? `API alojada de ${facts.name} frente a supuestos locales/open source`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} frente a V4 Flash: campos documentados`
            : MINIMAX_COMPARISON_TITLE.es,
    fr: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} face à GPT-5.5, Terra et Luna`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} face à GPT Image 1 : champs de migration`
        : facts.slug === "kimi-k3"
          ? `API hébergée ${facts.name} face aux hypothèses locales/open source`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} face à V4 Flash : champs documentés`
            : MINIMAX_COMPARISON_TITLE.fr,
    pt: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} vs. GPT-5.5, Terra e Luna`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} comparado ao GPT Image 1: campos de migração`
        : facts.slug === "kimi-k3"
          ? `API hospedada do ${facts.name} vs. suposições locais/open source`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} vs. V4 Flash: campos documentados`
            : MINIMAX_COMPARISON_TITLE.pt,
    ru: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} и GPT-5.5, Terra, Luna: сравнение`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} и GPT Image 1: поля миграции`
        : facts.slug === "kimi-k3"
          ? `Размещённый API ${facts.name} и предположения о локальном/open-source запуске`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} и V4 Flash: сравнение документированных полей`
            : MINIMAX_COMPARISON_TITLE.ru,
    ja: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} と GPT-5.5、Terra、Luna の比較`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} と GPT Image 1：移行項目の比較`
        : facts.slug === "kimi-k3"
          ? `${facts.name} のホステッド API とローカル/オープンソースの仮定`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} と V4 Flash：文書化項目の比較`
            : MINIMAX_COMPARISON_TITLE.ja,
    vi: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} so với GPT-5.5, Terra và Luna`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} so với GPT Image 1: trường chuyển đổi`
        : facts.slug === "kimi-k3"
          ? `API hosted của ${facts.name} so với giả định local/mã nguồn mở`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} so với V4 Flash: so sánh trường đã ghi nhận`
            : MINIMAX_COMPARISON_TITLE.vi,
    de: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} im Vergleich zu GPT-5.5, Terra und Luna`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} im Vergleich zu GPT Image 1: Migrationsfelder`
        : facts.slug === "kimi-k3"
          ? `Gehostete ${facts.name}-API vs. lokale/Open-Source-Annahmen`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} vs. V4 Flash: dokumentierte Felder vergleichen`
            : MINIMAX_COMPARISON_TITLE.de,
    id: facts.slug === "gpt-5-6-sol"
      ? `${facts.name} dibandingkan dengan GPT-5.5, Terra, dan Luna`
      : facts.slug === "gpt-image-2"
        ? `${facts.name} dibandingkan dengan GPT Image 1: bidang migrasi`
        : facts.slug === "kimi-k3"
          ? `API hosted ${facts.name} dibandingkan dengan asumsi lokal/open source`
          : facts.slug === "deepseek-v4-pro"
            ? `${facts.name} dibandingkan dengan V4 Flash: bidang terdokumentasi`
            : MINIMAX_COMPARISON_TITLE.id,
  };
  return titles[locale] ?? titles.en;
}

function buildTextCopy(facts: Facts, pack: LanguagePack, locale: Locale): ModelLandingContent {
  const isGpt = facts.slug === "gpt-5-6-sol";
  const isKimi = facts.slug === "kimi-k3";
  const isDeepseek = facts.slug === "deepseek-v4-pro";
  const modalities = localizedModalities(locale, facts);
  const modalitiesTable = localizedModalitiesTable(locale, facts);
  const heroDescription = isGpt
    ? pack.modelStatement(facts.name, facts.vendor, facts.endpoint, localizedGptFacts(locale, facts, modalities))
    : isKimi
      ? pack.kimiStatement(facts.name, facts.endpoint, facts.secondEndpoint ?? "/v1/messages", facts.context ?? "1,048,576 tokens")
      : pack.deepseekStatement(facts.name, facts.endpoint, facts.secondEndpoint ?? "/v1/messages", facts.context ?? "1,048,576 tokens");
  const capabilityCards = isGpt
    ? [
        { title: pack.context, body: pack.contextBody(facts.context ?? "1,048,576 tokens") },
        { title: pack.modalities, body: pack.modalitiesBody(modalities) },
        { title: pack.endpoint, body: pack.endpointBody(facts.endpoint, facts.id) },
        { title: "Flatkey", body: pack.keyBillingBody },
      ]
    : isKimi
      ? [
          { title: pack.context, body: pack.contextBody(facts.context ?? "1,048,576 tokens") },
          { title: pack.inputModality, body: pack.fileBody },
          { title: pack.endpoint, body: pack.twoRoutesBody(facts.endpoint, facts.secondEndpoint ?? "/v1/messages") },
          { title: literal(locale, "Knowledge work"), body: pack.knowledgeBody },
        ]
      : [
          { title: pack.context, body: pack.contextBody(facts.context ?? "1,048,576 tokens") },
          { title: pack.inputModality, body: pack.modalitiesBody(modalities) },
          { title: literal(locale, "OpenAI-compatible"), body: pack.routeBody(facts.endpoint) },
          { title: literal(locale, "Anthropic-compatible"), body: pack.routeBody(facts.secondEndpoint ?? "/v1/messages") },
          { title: literal(locale, "Distillable metadata"), body: pack.distillableBody },
        ];
  const comparison = isGpt
    ? {
        eyebrow: pack.compareFields,
        title: localizedComparisonTitle(locale, facts),
        description: pack.documentedFields,
        baselineLabel: "GPT-5.5 / Terra / Luna",
        currentLabel: facts.name,
        rows: [
          { label: pack.modelId, baseline: pack.unknown, current: facts.id },
          { label: pack.endpoint, baseline: pack.unknown, current: facts.endpoint },
          { label: pack.context, baseline: pack.unknown, current: facts.context ?? "1,048,576 tokens" },
          { label: pack.modalities, baseline: pack.unknown, current: modalitiesTable },
          { label: literal(locale, "Relative quality/benchmark"), baseline: pack.notAsserted, current: pack.notAsserted },
        ],
      }
    : isKimi
      ? {
          eyebrow: pack.hostedApiFacts,
          title: localizedComparisonTitle(locale, facts),
          description: pack.kimiLocalAnswer,
          baselineLabel: literal(locale, "Local / open-source assumptions"),
          currentLabel: `${literal(locale, "Verified")} ${facts.name}`,
          rows: [
            { label: pack.hostedEndpoint, baseline: pack.unknown, current: localizedRoutePair(locale, facts.endpoint, facts.secondEndpoint ?? "/v1/messages") },
            { label: pack.context, baseline: pack.unknown, current: facts.context ?? "1,048,576 tokens" },
            { label: pack.inputModality, baseline: pack.unknown, current: modalitiesTable },
            { label: pack.downloadableWeights, baseline: pack.notVerified, current: pack.notVerified },
            { label: pack.localRuntime, baseline: pack.notVerified, current: pack.notVerified },
            { label: pack.freeAccess, baseline: pack.unknown, current: literal(locale, "Not promised; paid token rates apply") },
          ],
        }
      : {
          eyebrow: pack.compareFields,
          title: localizedComparisonTitle(locale, facts),
          description: pack.documentedFields,
        baselineLabel: "DeepSeek V4 Flash",
          currentLabel: facts.name,
          rows: [
            { label: pack.modelId, baseline: "deepseek-v4-flash", current: facts.id },
            { label: pack.endpoint, baseline: literal(locale, "Verify before publishing"), current: `${facts.endpoint}, ${facts.secondEndpoint}` },
            { label: pack.context, baseline: pack.unknown, current: facts.context ?? "1,048,576 tokens" },
            { label: pack.modalities, baseline: pack.unknown, current: modalitiesTable },
            { label: literal(locale, "Benchmark/coding ranking"), baseline: pack.notAsserted, current: pack.notAsserted },
          ],
        };
  const apiItems = isGpt
    ? [
        { title: pack.endpoint, detail: `POST ${facts.endpoint}` },
        { title: pack.modelId, detail: facts.id },
        { title: pack.modalities, detail: pack.apiModalitiesDetail(modalities) },
        { title: literal(locale, "Compatibility"), detail: pack.apiCompatibilityDetail },
      ]
    : isKimi
      ? [
          { title: pack.modelId, detail: facts.id },
          { title: literal(locale, "OpenAI-compatible"), detail: `POST ${facts.endpoint}` },
          { title: literal(locale, "Anthropic-compatible"), detail: `POST ${facts.secondEndpoint}` },
          { title: pack.boundary, detail: pack.apiBoundaryDetail },
        ]
      : [
          { title: pack.modelId, detail: facts.id },
          { title: literal(locale, "OpenAI-compatible"), detail: `POST ${facts.endpoint}` },
          { title: literal(locale, "Anthropic-compatible"), detail: `POST ${facts.secondEndpoint}` },
          { title: locale === "en" ? "Verified inputs" : literal(locale, "Verified inputs"), detail: pack.modalitiesBody(modalities) },
        ];
  const faq = isGpt
    ? [
        { question: pack.whatIs(facts.name), answer: heroDescription },
        { question: pack.howUse(facts.name), answer: locale === "en" ? `Create a Flatkey API key, send a request to ${facts.endpoint}, and set model to ${facts.id}.` : `${pack.api} ${facts.id}: ${pack.apiEndpointDetail(facts.endpoint)}; model=${facts.id}.` },
        { question: pack.apiModelId(facts.name), answer: locale === "en" ? `The verified model ID is ${facts.id}; the documented endpoint is ${facts.endpoint}.` : `${pack.modelId}: ${facts.id}; ${pack.endpoint}: ${facts.endpoint}.` },
        { question: pack.cost(facts.name), answer: pack.costAnswer(facts.name, ratesText(facts, pack, locale)) },
        { question: pack.contextInputs(facts.name), answer: pack.contextAnswer(facts.context ?? "1,048,576 tokens", modalities) },
        { question: pack.release(facts.name), answer: pack.releaseAnswer("2026-06-20") },
      ]
    : isKimi
      ? [
          { question: pack.whatIs(facts.name), answer: heroDescription },
          { question: pack.howUse(facts.name), answer: pack.kimiUseAnswer(facts.id, facts.endpoint, facts.secondEndpoint ?? "/v1/messages") },
          { question: pack.apiModelId(facts.name), answer: pack.kimiIdAnswer(facts.id, facts.endpoint, facts.secondEndpoint ?? "/v1/messages") },
          { question: pack.cost(facts.name), answer: pack.kimiCostAnswer(ratesText(facts, pack, locale)) },
          { question: pack.freeQuestion(facts.name), answer: pack.kimiFreeAnswer },
          { question: pack.localQuestion(facts.name), answer: pack.kimiLocalAnswer },
          { question: pack.vendorQuestion(facts.name), answer: pack.kimiVendorAnswer },
        ]
      : [
          { question: pack.whatIs(facts.name), answer: heroDescription },
          { question: pack.howUse(facts.name), answer: pack.deepseekUseAnswer(facts.id, facts.endpoint, facts.secondEndpoint ?? "/v1/messages") },
          { question: pack.apiModelId(facts.name), answer: pack.deepseekIdAnswer(facts.id, facts.endpoint, facts.secondEndpoint ?? "/v1/messages") },
          { question: pack.priceUtcQuestion(facts.name), answer: pack.deepseekCostAnswer },
          { question: pack.localQuestion(facts.name), answer: pack.deepseekLocalAnswer },
          { question: pack.visionQuestion(facts.name), answer: pack.deepseekVisionAnswer },
          { question: pack.qualityQuestion(facts.name), answer: pack.deepseekQualityAnswer },
        ];
  const pricingTitle = localizedPricingTitle(facts, pack, locale, isDeepseek ? "utc" : "standard");
  return {
    hero: { title: `${facts.name} ${pack.apiPricingDetails}`, description: heroDescription },
    pricing: {
      title: pricingTitle,
      description: isDeepseek ? pack.pricingNoteUtc : pack.pricingCurrent,
      note: isDeepseek ? pack.pricingNoteUtc : pack.pricingNoteToken,
      rows: localizedRateRows(facts, pack, locale),
    },
    capabilitiesEyebrow: `${facts.name} ${pack.capabilities}`,
    capabilitiesTitle: localizedCapabilitiesTitle(facts, pack, locale),
    capabilitiesDescription: pack.documentedFields,
    capabilities: capabilityCards,
    comparison,
    api: {
      eyebrow: pack.api,
      title: localizedTextApiTitle(facts, pack, locale),
      description: isGpt ? literal(locale, "Use the exact model ID and documented endpoint in your existing client.") : pack.textApiSetup,
      items: apiItems,
    },
    why: buildWhy(facts, locale),
    faqTitle: buildFaqTitle(facts, locale),
    faqDescription: pack.faqDescription,
    faq,
  };
}

function buildImageCopy(facts: Facts, pack: LanguagePack, locale: Locale): ModelLandingContent {
  return {
    hero: { title: `${facts.name} ${pack.apiPricingDetails}`, description: pack.imageStatement(facts.name, facts.endpoint) },
    pricing: {
      title: localizedPricingTitle(facts, pack, locale, "tokenDimensions"),
      description: pack.pricingNoteImage,
      note: pack.pricingNoteImage,
      rows: localizedRateRows(facts, pack, locale),
    },
    capabilitiesEyebrow: `${facts.name} ${pack.controls}`,
    capabilitiesTitle: localizedCapabilitiesTitle(facts, pack, locale),
    capabilitiesDescription: pack.documentedFields,
    capabilities: [
      { title: pack.input, body: pack.imageCountBody },
      { title: pack.sizes, body: pack.imageSizesBody },
      { title: pack.quality, body: pack.imageQualityBody },
      { title: pack.formats, body: pack.imageFormatsBody },
      { title: literal(locale, "Background / moderation"), body: pack.imageBackgroundBody },
    ],
    comparison: {
      eyebrow: pack.migrationFields,
      title: localizedComparisonTitle(locale, facts),
      description: pack.documentedFields,
      baselineLabel: "GPT Image 1",
      currentLabel: facts.name,
      rows: [
        { label: pack.endpoint, baseline: localizedUnknownHere(locale, pack), current: facts.endpoint },
        { label: pack.sizes, baseline: localizedUnknownHere(locale, pack), current: "1024x1024, 1536x1024, 1024x1536, auto" },
        { label: pack.formats, baseline: localizedUnknownHere(locale, pack), current: "PNG, JPEG, WebP" },
        { label: literal(locale, "Quality/background/moderation"), baseline: localizedUnknownHere(locale, pack), current: literal(locale, "Documented above") },
        { label: literal(locale, "Image quality ranking"), baseline: pack.notAsserted, current: pack.notAsserted },
      ],
    },
    api: {
      eyebrow: pack.api,
      title: localizedImageApiTitle(facts, pack, locale),
      description: localizedImageApiDescription(locale),
      items: [
        { title: pack.endpoint, detail: `POST ${facts.endpoint}` },
        { title: pack.modelId, detail: facts.id },
        { title: pack.controls, detail: pack.apiImageControlsDetail },
        { title: pack.access, detail: pack.apiAccessDetail },
      ],
    },
    ...buildPromptLibrary(facts, locale),
    why: buildWhy(facts, locale),
    faqTitle: buildFaqTitle(facts, locale),
    faqDescription: pack.faqDescription,
    faq: [
      { question: pack.whatIs(facts.name), answer: locale === "en" ? `GPT Image 2 is the OpenAI image model available through ${facts.endpoint}.` : pack.imageStatement(facts.name, facts.endpoint) },
      { question: pack.accessQuestion(facts.name), answer: pack.imageAccessAnswer },
      { question: pack.endpointQuestion(facts.name), answer: pack.imageEndpointAnswer(facts.id, facts.endpoint) },
      { question: pack.cost(facts.name), answer: pack.imageCostAnswer(ratesText(facts, pack, locale)) },
      { question: pack.sizeFormatQuestion(facts.name), answer: pack.imageSizeAnswer },
      { question: pack.transparentQuestion, answer: pack.transparentAnswer },
      { question: pack.freeSignupQuestion(facts.name), answer: pack.freeSignupAnswer },
    ],
  };
}

function buildVideoCopy(facts: Facts, pack: LanguagePack, locale: Locale): ModelLandingContent {
  return {
    hero: { title: `${facts.name} ${pack.videoApiPricing}`, description: pack.minimaxStatement(facts.name, facts.endpoint) },
    pricing: {
      title: localizedPricingTitle(facts, pack, locale, "standard"),
      description: pack.pricingNoteVideo,
      note: pack.pricingNoteVideo,
      rows: localizedRateRows(facts, pack, locale),
    },
    capabilitiesEyebrow: `${facts.name} ${pack.videoFields}`,
    capabilitiesTitle: localizedCapabilitiesTitle(facts, pack, locale),
    capabilitiesDescription: pack.documentedFields,
    capabilities: [
      { title: pack.resolution, body: pack.resolutionBody },
      { title: pack.duration, body: pack.durationBody },
      { title: pack.ratio, body: pack.ratioBody },
      { title: pack.watermark, body: pack.watermarkBody },
      { title: MINIMAX_REFERENCE_TITLE[locale] ?? MINIMAX_REFERENCE_TITLE.en, body: MINIMAX_REFERENCE_BODY[locale] ?? MINIMAX_REFERENCE_BODY.en },
    ],
    comparison: {
      eyebrow: pack.videoFields,
      title: localizedComparisonTitle(locale, facts),
      description: pack.documentedFields,
      baselineLabel: MINIMAX_COMPARISON_BASELINE[locale] ?? MINIMAX_COMPARISON_BASELINE.en,
      currentLabel: MINIMAX_COMPARISON_CURRENT[locale] ?? MINIMAX_COMPARISON_CURRENT.en,
      rows: [
        { label: pack.resolution, baseline: "768P", current: MINIMAX_RESOLUTION_RANGE[locale] ?? MINIMAX_RESOLUTION_RANGE.en },
        { label: pack.duration, baseline: literal(locale, "6 seconds"), current: MINIMAX_DURATION_RANGE[locale] ?? MINIMAX_DURATION_RANGE.en },
        { label: pack.ratio, baseline: "16:9", current: MINIMAX_COMPARISON_RATIO[locale] ?? MINIMAX_COMPARISON_RATIO.en },
        { label: pack.watermark, baseline: MINIMAX_OFF[locale] ?? MINIMAX_OFF.en, current: MINIMAX_EXPLICIT_BOOLEAN[locale] ?? MINIMAX_EXPLICIT_BOOLEAN.en },
        { label: literal(locale, "ComfyUI or local weights"), baseline: literal(locale, "Not verified"), current: literal(locale, "Not verified in Flatkey catalog") },
      ],
    },
    api: {
      eyebrow: pack.api,
      title: localizedVideoApiTitle(facts, pack, locale),
      description: pack.minimaxUseAnswer,
      items: [
        { title: pack.endpoint, detail: `POST ${facts.endpoint}` },
        { title: pack.modelId, detail: facts.id },
        { title: pack.fields, detail: literal(locale, "resolution, duration, ratio, and aigc_watermark.") },
        { title: pack.result, detail: pack.apiResultDetail },
      ],
    },
    ...buildPromptLibrary(facts, locale),
    why: buildWhy(facts, locale),
    faqTitle: buildFaqTitle(facts, locale),
    faqDescription: pack.faqDescription,
    faq: (() => {
      const extra = MINIMAX_EXTRA_FAQ[locale] ?? MINIMAX_EXTRA_FAQ.en;
      return [
        { question: pack.settingsQuestion(facts.name), answer: pack.minimaxFieldsAnswer },
        { question: pack.whatIs(facts.name), answer: pack.minimaxWhatAnswer(facts.endpoint) },
        { question: pack.cost(facts.name), answer: pack.minimaxCostAnswer },
        { question: pack.howUse(facts.name), answer: pack.minimaxUseAnswer },
        { question: extra.promptQuestion, answer: extra.promptAnswer },
        { question: extra.localQuestion, answer: extra.localAnswer },
        { question: extra.moderationQuestion, answer: extra.moderationAnswer },
        { question: extra.draftQuestion, answer: extra.draftAnswer },
      ];
    })(),
  };
}

function buildLandingContent(slug: PriorityModelSlug, locale: Locale): ModelLandingContent {
  const facts = FACTS[slug];
  const pack = PACKS[locale] ?? PACKS.en;
  if (facts.kind === "image") return buildImageCopy(facts, pack, locale);
  if (facts.kind === "video") return buildVideoCopy(facts, pack, locale);
  return buildTextCopy(facts, pack, locale);
}

/** Return the complete localized editorial block for a priority page. */
export function getPriorityModelCopy(slug: string, locale: Locale): PriorityModelLocalizedCopy | null {
  const prioritySlug = normalizePrioritySlug(slug);
  if (!prioritySlug) return null;
  return {
    seo: SEO[prioritySlug][locale] ?? SEO[prioritySlug].en,
    landingContent: buildLandingContent(prioritySlug, locale),
  };
}

/** Locale-aware metadata lookup for route-level `generateMetadata`. */
export function getPriorityModelSeo(slug: string, locale: Locale): PriorityModelSeo | null {
  const prioritySlug = normalizePrioritySlug(slug);
  if (!prioritySlug) return null;
  return SEO[prioritySlug][locale] ?? SEO[prioritySlug].en;
}

function collectStrings(value: unknown, output: string[] = []): string[] {
  if (typeof value === "string") output.push(value);
  else if (Array.isArray(value)) value.forEach((item) => collectStrings(item, output));
  else if (value && typeof value === "object") Object.values(value).forEach((item) => collectStrings(item, output));
  return output;
}

/**
 * Pair an existing English source object with the localized priority object.
 *
 * The model page keeps its shared shape in `model-landing.ts`; callers can
 * pass that exact `landingContent` object here and receive a flat map suitable
 * for the existing `modelLandingCopy(locale, key)` resolver.  Pairing by
 * structure (rather than by a global string list) avoids translating a phrase
 * in the wrong model when two sections happen to contain similar text.
 */
export function getPriorityModelTranslationMapForSource(
  slug: string,
  locale: Locale,
  source: ModelLandingContent,
): Record<string, string> {
  const copy = getPriorityModelCopy(slug, locale);
  if (!copy) return {};
  const map: Record<string, string> = {};
  const pair = (english: unknown, localized: unknown): void => {
    if (typeof english === "string" && typeof localized === "string") {
      if (english !== localized) map[english] = localized;
      return;
    }
    if (Array.isArray(english) && Array.isArray(localized)) {
      english.forEach((item, index) => pair(item, localized[index]));
      return;
    }
    if (english && typeof english === "object" && localized && typeof localized === "object") {
      Object.keys(english).forEach((key) => {
        pair((english as Record<string, unknown>)[key], (localized as Record<string, unknown>)[key]);
      });
    }
  };
  pair(source, copy.landingContent);
  return map;
}

/**
 * Build an English-keyed translation map for integration with the existing
 * `modelLandingCopy(locale, key)` resolver.  Only strings that are actually in
 * the priority editorial block are included, so the shared shell remains the
 * fallback for unrelated pages.
 */
export function getPriorityModelTranslationMap(slug: string, locale: Locale): Record<string, string> {
  const copy = getPriorityModelCopy(slug, locale);
  if (!copy) return {};
  const prioritySlug = normalizePrioritySlug(slug);
  if (!prioritySlug) return {};
  return getPriorityModelTranslationMapForSource(
    prioritySlug,
    locale,
    buildLandingContent(prioritySlug, "en"),
  );
}

/** Merge all priority-page strings for use by a locale-first translator. */
export function getPriorityModelTranslations(locale: Locale): Record<string, string> {
  return PRIORITY_MODEL_SLUGS.reduce<Record<string, string>>((merged, slug) => ({
    ...merged,
    ...getPriorityModelTranslationMap(slug, locale),
  }), {});
}

/** Direct lookup helper; returns the original key when no translation exists. */
export function translatePriorityModelString(slug: string, locale: Locale, key: string): string {
  return getPriorityModelTranslationMap(slug, locale)[key] ?? key;
}

/** A small invariant helper used by tests and implementation checks. */
export function isPriorityModelSlug(value: string): value is PriorityModelSlug {
  return normalizePrioritySlug(value) !== null;
}

// Keep this assertion close to the data: if a future locale is added to
// locales.ts, TypeScript forces the editorial pack and SEO metadata to be
// filled instead of silently falling back to English.
void LOCALES;
