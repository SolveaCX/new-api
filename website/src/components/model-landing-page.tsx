"use client";

import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type DragEvent, type MouseEvent, type ReactNode } from "react";
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  ChevronRight,
  Code2,
  Copy,
  FileText,
  Gauge,
  ImageIcon,
  KeyRound,
  Layers3,
  Music2,
  Play,
  Settings2,
  ShieldCheck,
  Sparkles,
  Timer,
  Trash2,
  Upload,
  Video,
  WandSparkles,
  Zap,
} from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { DailyHealthBars } from "@/components/home-health-bars";
import { HomeModelLogo } from "@/components/home-model-logo";
import { CdnFallbackImage, CdnFallbackVideo } from "@/components/cdn-media";
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
  type HomeModelHealth,
  type HomeTrendPoint,
} from "@/lib/home-live";
import { SiteShell } from "@/components/site-shell";
import { modelIconKey } from "@/lib/home-models";
import { localizePath, type Locale } from "@/lib/locales";
import {
  getImagePlaygroundExample,
  getImagePromptTemplateLocalFallbackPoster,
  getImagePromptTemplateFallbackPosters,
  getImagePromptTemplates,
  localizeImagePromptText,
  type ImagePlaygroundExample,
} from "@/lib/image-prompt-templates";
import {
  getVideoPlaygroundPrompt,
  getVideoPromptTemplateLocalFallbackPoster,
  getVideoPromptTemplates,
  localizeVideoPromptText,
  VIDEO_PROMPT_TEMPLATES,
} from "@/lib/video-prompt-templates";
import {
  modelLandingCopy,
  getModelVideoModeLabel,
  getModelVideoModeUiCopy,
  getLocalizedModelLandingConfig,
  getModelLandingConfigs,
  normalizeModelId,
  type ModelConfig,
  type ModelGeneratorConfig,
  type ModelGeneratorField,
  type ModelGeneratorProtocol,
  type ModelLandingKey,
  type ModelVideoMode,
  type ModelVideoModeOption,
} from "@/lib/model-landing";
import { navigateToHref } from "@/lib/link-navigation";
import { consoleUrl } from "@/lib/origins";
import {
  formatModelPrice,
  buildEffectiveGroupRatio,
  discountedPriceUsd,
  formatUsdPrice,
  getBestGroupRatio,
  getModelFamilyKey,
  getOfficialPriceUsd,
  isTokenBasedModel,
  resolveModelDisplayPrice,
  type GroupModelRatio,
  type PricingModel,
} from "@/lib/pricing";
import type { RankedModel, RankingsData } from "@/lib/rankings-live";
import { buildModelSchema, stringifyJsonLd } from "@/lib/schema";

type Props = {
  config: ModelConfig;
  locale: Locale;
  liveModels?: PricingModel[];
  allModels?: PricingModel[];
  groupRatio?: Record<string, number>;
  groupModelRatio?: GroupModelRatio;
  rankings?: RankingsData | null;
  initialHealth?: HomeModelHealth;
};

type GtagWindow = Window & {
  gtag?: (...args: unknown[]) => void;
};

type DraftValue = Record<string, unknown>;

type MediaExample = {
  poster: string;
  video?: string;
  /** Packaged same-source poster used only if the primary media fails. */
  fallbackPoster?: string;
  /** Original bundled clip used when a CDN clip cannot be loaded. */
  fallbackVideo?: string;
};

const isProfessionVideo = (video?: string) => Boolean(video?.includes("/model-showcase/video-profession-"));

type ReferenceImageDraft = {
  id: string;
  name: string;
  size: number;
  type: string;
  previewUrl: string;
};

type MediaUploadKind = "image" | "video" | "audio";
type MediaUploadCounts = Record<MediaUploadKind, number>;

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
  asset?: string;
};

type FlatkeyPriceTableRow = {
  label: string;
  flatkey: string;
  official: string;
  flatkeyPercent: number;
  officialPercent: number;
};

type StarterCopy = Record<Locale, (modelName: string) => string>;

// The editor prompt is user-facing copy. Keep it separate from
// `config.examplePrompt`, which remains the canonical English request used in
// code/request previews, and provide a complete starter for every locale.
const MODEL_STARTER_COPY: Record<"text" | "image" | "audio", StarterCopy> = {
  text: {
    en: (name) => `You are a senior backend engineer. In 3 sentences, explain why developers should use ${name} through an LLM gateway instead of calling each official API directly.`,
    zh: (name) => `请让 ${name} 用三句话说明：与其分别调用各家官方 API，开发者为什么应该通过 LLM 网关使用该模型。`,
    es: (name) => `Pide a ${name} que explique en tres frases por qué los desarrolladores deberían usarlo mediante una pasarela LLM en vez de llamar directamente a cada API oficial.`,
    fr: (name) => `Demandez à ${name} d’expliquer en trois phrases pourquoi les développeurs devraient passer par une passerelle LLM plutôt que d’appeler chaque API officielle directement.`,
    pt: (name) => `Peça ao ${name} que explique em três frases por que desenvolvedores devem usá-lo por um gateway de LLM em vez de chamar cada API oficial diretamente.`,
    ru: (name) => `Попросите ${name} в трёх предложениях объяснить, почему разработчикам стоит использовать модель через шлюз LLM, а не напрямую вызывать каждый официальный API.`,
    ja: (name) => `${name}に、各公式APIを個別に呼び出すのではなくLLMゲートウェイを使うべき理由を3文で説明させる。`,
    vi: (name) => `Yêu cầu ${name} giải thích trong 3 câu vì sao nhà phát triển nên dùng mô hình qua cổng LLM thay vì gọi trực tiếp từng API chính thức.`,
    de: (name) => `Bitte ${name} in drei Sätzen erklären lassen, warum Entwickler das Modell über ein LLM-Gateway statt über direkte Aufrufe jeder offiziellen API nutzen sollten.`,
    id: (name) => `Minta ${name} menjelaskan dalam 3 kalimat mengapa pengembang sebaiknya menggunakan model melalui gateway LLM, bukan memanggil setiap API resmi secara langsung.`,
  },
  image: {
    en: (name) => `Create a high-quality product image with ${name}: clean composition, precise lighting, strong subject focus, and realistic detail.`,
    zh: (name) => `使用 ${name} 制作高质量产品图：构图干净、光线精准、主体突出、细节真实。`,
    es: (name) => `Crea con ${name} una imagen de producto de alta calidad: composición limpia, iluminación precisa, sujeto destacado y detalle realista.`,
    fr: (name) => `Créez avec ${name} une image produit de haute qualité : composition nette, lumière précise, sujet mis en valeur et détails réalistes.`,
    pt: (name) => `Crie com ${name} uma imagem de produto de alta qualidade: composição limpa, iluminação precisa, foco forte no produto e detalhes realistas.`,
    ru: (name) => `Создайте с помощью ${name} качественное изображение продукта: чистая композиция, точный свет, акцент на объекте и реалистичные детали.`,
    ja: (name) => `${name}で高品質な商品画像を作成：整理された構図、正確な照明、被写体への強いフォーカス、リアルなディテール。`,
    vi: (name) => `Tạo ảnh sản phẩm chất lượng cao bằng ${name}: bố cục sạch, ánh sáng chính xác, tập trung rõ vào chủ thể và chi tiết chân thực.`,
    de: (name) => `Erstelle mit ${name} ein hochwertiges Produktbild: klare Komposition, präzises Licht, starker Motivfokus und realistische Details.`,
    id: (name) => `Buat gambar produk berkualitas tinggi dengan ${name}: komposisi bersih, pencahayaan presisi, fokus kuat pada subjek, dan detail realistis.`,
  },
  audio: {
    en: (name) => `Create a polished music bed with ${name} for a short product video: keep the video timing, preserve important speech, use a warm electronic style, and deliver a clean loopable ending.`,
    zh: (name) => `使用 ${name} 为短产品视频制作精致的音乐铺底：保持视频节奏，保留重要语音，采用温暖电子风格，并以干净、可循环的结尾收束。`,
    es: (name) => `Crea con ${name} una base musical pulida para un vídeo corto de producto: conserva el ritmo del vídeo y la voz importante, usa un estilo electrónico cálido y termina con un cierre limpio y repetible.`,
    fr: (name) => `Créez avec ${name} un habillage musical soigné pour une courte vidéo produit : respectez le rythme, préservez les paroles importantes, adoptez un style électronique chaleureux et terminez par une boucle nette.`,
    pt: (name) => `Crie com ${name} uma base musical refinada para um vídeo curto de produto: mantenha o ritmo, preserve a fala importante, use um estilo eletrônico acolhedor e entregue um final limpo e repetível.`,
    ru: (name) => `Создайте с помощью ${name} выверенную музыкальную подложку для короткого видео продукта: сохраните темп, важную речь и тёплый электронный стиль, завершив чистым зацикливаемым финалом.`,
    ja: (name) => `${name}で短い商品動画用の洗練された音楽ベッドを作成：映像のタイミングと重要な話し声を保ち、温かいエレクトロニックスタイルで、きれいにループできる終わりにする。`,
    vi: (name) => `Tạo nền nhạc hoàn chỉnh bằng ${name} cho video sản phẩm ngắn: giữ nhịp video, bảo toàn lời thoại quan trọng, dùng phong cách điện tử ấm áp và kết thúc sạch để lặp.`,
    de: (name) => `Erstelle mit ${name} ein ausgearbeitetes Musikbett für ein kurzes Produktvideo: Timing und wichtige Sprache erhalten, einen warmen elektronischen Stil nutzen und sauber loopbar enden.`,
    id: (name) => `Buat musik latar yang rapi dengan ${name} untuk video produk singkat: pertahankan timing video dan ucapan penting, gunakan gaya elektronik hangat, lalu akhiri dengan penutup bersih yang dapat diulang.`,
  },
};

function getModelStarterPrompt(
  config: ModelConfig,
  locale: Locale,
  imageExample?: ImagePlaygroundExample,
): string {
  const kind = config.generator?.kind;
  if (kind === "image" && imageExample) return imageExample.prompt;
  if (kind === "video") return getVideoPlaygroundPrompt(config.modelId, locale, config.displayName);
  if (kind === "image" || kind === "audio") return MODEL_STARTER_COPY[kind][locale](config.displayName);
  return MODEL_STARTER_COPY.text[locale](config.displayName);
}

function localizeConfiguredPrompt(
  prompt: string,
  kind: ModelGeneratorConfig["kind"] | "text" | undefined,
  locale: Locale,
): string {
  if (locale === "en") return prompt;
  if (kind === "image") return localizeImagePromptText(prompt, locale);
  if (kind === "video") return localizeVideoPromptText(prompt, locale);
  return prompt;
}

const MEDIA_EXAMPLES: Record<"image" | "video" | "audio", readonly MediaExample[]> = {
  image: [
    { poster: "/assets/prompts/awesome-images/gpt-image-2-showcase-complex.png" },
    { poster: "/assets/prompts/awesome-images/ecommerce-skincare.png" },
    { poster: "/assets/prompts/awesome-images/ugc-coffee-ad.png" },
  ],
  video: [
    { poster: "/assets/cli/product-reveal.png", video: "/assets/cli/product-reveal.mp4" },
    { poster: "/assets/cli/ugc-ad-clips.png", video: "/assets/cli/ugc-ad-clips.mp4" },
    { poster: "/assets/cli/localized-variants.png", video: "/assets/cli/localized-variants.mp4" },
  ],
  audio: [
    { poster: "/assets/prompts/awesome-images/ai-agent-poster.png" },
    { poster: "/assets/prompts/awesome-images/liquid-bento.png" },
    { poster: "/assets/cli/campaign-hero.png" },
  ],
} as const;

function isRemoteMedia(src: string) {
  return /^https?:\/\//i.test(src);
}

export function ModelLandingPage({ config: inputConfig, locale, liveModels = [], allModels = [], groupRatio = {}, groupModelRatio = {}, rankings = null, initialHealth }: Props) {
  // Priority pages ship a complete editorial pack per locale. Resolve it once
  // at the page boundary so the shared shell never mixes an English source
  // field with a translated field from a different section.
  const config = useMemo(
    () => getLocalizedModelLandingConfig(inputConfig, locale),
    [inputConfig, locale],
  );

  // Next can preserve the previous directory scroll position while navigating
  // between client-rendered routes. A model detail page is always entered at
  // its hero, so reset the document position whenever the model or locale
  // changes (including client-side navigation from the model list).
  useEffect(() => {
    window.scrollTo({ top: 0, left: 0, behavior: "auto" });
  }, [config.slug, locale]);

  const imagePlaygroundExample = config.generator?.kind === "image"
    ? getImagePlaygroundExample(config.modelId, locale)
    : undefined;
  const initialPrompt = getModelStarterPrompt(config, locale, imagePlaygroundExample);
  const [prompt, setPrompt] = useState(() => initialPrompt);
  const [fieldValues, setFieldValues] = useState<Record<string, string | number | boolean>>(() =>
    buildInitialGeneratorValues(config)
  );
  const [referenceImages, setReferenceImages] = useState<ReferenceImageDraft[]>([]);
  const [mediaUploadCounts, setMediaUploadCounts] = useState<MediaUploadCounts>({ image: 0, video: 0, audio: 0 });
  const onMediaUploadCountChange = useCallback((kind: MediaUploadKind, count: number) => {
    setMediaUploadCounts((current) => (current[kind] === count ? current : { ...current, [kind]: count }));
  }, []);
  const generator = config.generator;
  const mediaKind = generator?.kind ?? "text";
  const t = useCallback(
    (key: string, vars?: Record<string, string>) => modelLandingCopy(locale, key as ModelLandingKey, vars),
    [locale]
  );
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

  const buildDraft = (): DraftValue => ({
    source: "model_landing",
    model: config.modelId,
    slug: config.slug,
    mediaKind,
    endpoint: generator?.endpoint ?? "/v1/chat/completions",
    storageKey: generator?.storageKey ?? "flatkey:model-generator-draft",
    prompt,
    fields: fieldValues,
    referenceImages: referenceImages.map(({ name, size, type }) => ({ name, size, type })),
    request: buildGeneratorRequest(config, prompt, fieldValues, referenceImages),
    locale,
    savedAt: new Date().toISOString(),
  });

  const onRunClick = (event: MouseEvent<HTMLAnchorElement>) => {
    (window as GtagWindow).gtag?.("event", "flatkey_sign_in_to_run_click", {
      model: config.slug,
      media_kind: mediaKind,
    });
    const draft = buildDraft();
    window.localStorage.setItem(generator?.storageKey ?? "flatkey:model-generator-draft", JSON.stringify(draft));
    event.currentTarget.href = withCurrentSearch(buildRunHref(config, locale, prompt, draft));
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
      mediaUploadCounts={mediaUploadCounts}
      onMediaUploadCountChange={onMediaUploadCountChange}
      onRunClick={onRunClick}
      primaryLiveModel={primaryLiveModel}
      liveModels={liveModels}
      allModels={allModels}
      groupRatio={groupRatio}
      groupModelRatio={groupModelRatio}
      rankings={rankings}
      initialHealth={initialHealth}
      imagePlaygroundExample={imagePlaygroundExample}
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
  mediaUploadCounts: MediaUploadCounts;
  onMediaUploadCountChange: (kind: MediaUploadKind, count: number) => void;
  onPromptChange: (prompt: string) => void;
  onFieldChange: (name: string, value: string | number | boolean) => void;
  onReferenceImagesChange: (images: ReferenceImageDraft[]) => void;
  onRunClick: (event: MouseEvent<HTMLAnchorElement>) => void;
  primaryLiveModel: PricingModel | null;
  liveModels: PricingModel[];
  allModels: PricingModel[];
  groupRatio: Record<string, number>;
  groupModelRatio: GroupModelRatio;
  rankings: RankingsData | null;
  initialHealth?: HomeModelHealth;
  imagePlaygroundExample?: ImagePlaygroundExample;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const runHref = buildRunHref(props.config, props.locale, props.prompt, {
    model: props.config.modelId,
    prompt: props.prompt,
    fields: props.fieldValues,
  });
  const model = props.primaryLiveModel;
  const providerName = model?.vendor_name ?? props.config.officialName;
  const effectiveGroupRatio = model
    ? buildEffectiveGroupRatio(model, props.groupRatio, props.groupModelRatio)
    : props.groupRatio;
  const relatedModels = buildCatalogRelatedModels(props.config, props.locale, props.allModels, props.t);
  const priceRows = buildFlatkeyPriceRows(props.config, model, effectiveGroupRatio, props.t);
  const configuredGenerator = props.config.generator;
  const isAudioModel = configuredGenerator?.kind === "audio";
  // Audio model pages expose API and pricing details only. They do not have a
  // public prompt playground, so keep the configured kind for page metadata
  // while disabling the shared workbench/navigation entry below.
  const generator = isAudioModel ? undefined : configuredGenerator;
  const mediaKind = configuredGenerator?.kind ?? "text";
  const mediaReferenceCount = configuredGenerator?.kind === "video"
    ? Object.values(props.mediaUploadCounts).reduce((total, count) => total + count, 0)
    : configuredGenerator?.kind === "image"
      ? props.referenceImages.length
      : 0;
  const hasPromptLibrary = mediaKind === "image" || mediaKind === "video";
  const examples = configuredGenerator ? MEDIA_EXAMPLES[configuredGenerator.kind] : [];
  const modelDescription = buildModelDescription(props.config, model, props.t, props.locale);
  const faqItems = buildModelFaq(props.config, props.t).map((item) =>
    /cost|price|pricing|多少钱|价格|料金|preço|tarif|цена|giá|Preis|harga/i.test(item.question)
      ? { ...item, answer: props.t("Use the pricing section above for current Flatkey prices from our pricing API.") }
      : item,
  );
  const schema = buildModelSchema({
    locale: props.locale,
    modelName: props.config.displayName,
    vendorName: providerName,
    description: modelDescription,
    // Seedance 2.5 has a resolution/duration/input-dependent formula, while
    // the other audited priority pages expose multiple token, image, or UTC
    // dimensions. Do not turn any of those editorial tables into a misleading
    // single Product Offer. Their exact dimensions remain visible below.
    // Product offers are only emitted when a live catalog model exists. Never
    // turn a curated config amount into structured pricing data.
    // Structured offers are defined as the effective input-token rate. Resolve
    // that value from the same live display contract used by the visible price
    // rows. Request/per-second models (and tiered models without a single rate)
    // must not be represented as a token offer, and model-specific group
    // ratios must be applied before falling back to legacy ratio math.
    inputPriceUsd: model && model.billing_mode !== "tiered_expr"
      ? resolveModelDisplayPrice(model, "input", "plg", effectiveGroupRatio)?.value ?? Number.NaN
      : Number.NaN,
    pagePath: localizePath(`/models/${props.config.slug}`, props.locale),
    faq: faqItems.map((item) => ({ q: item.question, a: item.answer })),
  });
  const rankingRow = findRankingRow(props.rankings?.models ?? [], props.config.modelId);

  const [health, setHealth] = useState<HomeModelHealth>(props.initialHealth ?? { model: "", trend: [] });

  useEffect(() => {
    let cancelled = false;
    // The pricing catalog contains the canonical upstream model name. Use it
    // when available because telemetry is keyed by that exact name (the
    // display/config id can be an alias, especially on family landing pages).
    const modelName = props.primaryLiveModel?.model_name ?? props.config.modelId;
    const applySummary = (summaries: Record<string, HomePerfSummary>) => {
      if (cancelled) return;
      const normalized = normalizeModelId(modelName);
      const summary =
        summaries[modelName] ??
        Object.values(summaries).find((row) => {
          const rowModel = normalizeModelId(row.model_name);
          return rowModel === normalized || rowModel.startsWith(`${normalized}-`) || normalized.startsWith(`${rowModel}-`);
        });
      setHealth((current) => ({ model: modelName, trend: current.model === modelName ? current.trend : [], summary }));
    };
    const applyTrend = (nextTrend: HomeTrendPoint[]) => {
      if (cancelled) return;
      setHealth((current) => ({ model: modelName, trend: nextTrend, summary: current.model === modelName ? current.summary : undefined }));
    };
    // Keep the two telemetry requests independent. A slow or unavailable trend
    // endpoint should not prevent the summary cards from showing live values.
    // Refresh once a minute so Activity and Performance do not remain stuck at
    // the values from the original page load while the tab stays open.
    const refreshHealth = () => {
      fetchHealthSummary(undefined, modelName).then(applySummary);
      fetchModelTrend(modelName).then(applyTrend);
    };
    refreshHealth();
    const refreshTimer = window.setInterval(refreshHealth, 60_000);
    return () => {
      cancelled = true;
      window.clearInterval(refreshTimer);
    };
  }, [props.config.modelId, props.primaryLiveModel?.model_name]);

  const healthModelName = props.primaryLiveModel?.model_name ?? props.config.modelId;
  const healthReady = normalizeModelId(health.model) === normalizeModelId(healthModelName);
  const trend = healthReady ? health.trend : [];
  const summary = healthReady ? health.summary : undefined;
  const hasLiveHealthData = Boolean(summary || trend.length > 0);
  const trendSuccess = averageFinite(trend.map((point) => point.success_rate));
  const successRate = summary?.success_rate ?? trendSuccess;
  const ttft = summary?.avg_ttft_ms ?? trendAvgTtftMs(trend);
  const unavailablePrice = props.t("Pricing data unavailable");
  const heroTags = buildHeroTags(props.config, model, props.t);
  const landingContent = props.config.landingContent;
  const heroContent = landingContent?.hero;
  const heroProvider = heroContent?.provider ?? providerName;
  const heroTitle = buildHumanReadableModelTitle(props.config, props.locale);
  const pricingContent = landingContent?.pricing ?? {
    eyebrow: props.t("Pricing"),
    title: `${props.config.displayName} ${props.t("Pricing")}`,
    description: props.t("Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API."),
    note: props.t("Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API."),
    rows: [],
  };
  const formatHeroPriceRows = (field: "flatkey" | "official") => priceRows.rows
    .slice(0, 3)
    .map((row) => {
      const resolution = /^(\d+p) ·/.exec(row.label)?.[1];
      return { resolution, value: row[field] };
    });
  const heroOfficialPriceRows = model ? formatHeroPriceRows("official") : [];
  const heroFlatkeyPriceRows = model ? formatHeroPriceRows("flatkey") : [];
  const isImageCatalogFallback = props.config.slug === "gpt-image-2" && !model;
  const isMiniMaxCatalogFallback = props.config.slug === "minimax-h3" && !model;
  const inputPriceRow = priceRows.rows.find((row) => row.label === props.t("Input /M"));
  const cachePriceRow = priceRows.rows.find((row) => row.label === props.t("Cache /M"));
  const outputPriceRow = priceRows.rows.find((row) => row.label === props.t("Output /M"));
  const showsTokenPriceBreakdown = Boolean(inputPriceRow && outputPriceRow);
  const tokenPriceRows = [inputPriceRow, cachePriceRow, outputPriceRow].filter(
    (row): row is FlatkeyPriceTableRow => Boolean(row)
  );
  const dashboardHref = consoleUrl("/dashboard");

  return (
    <SiteShell locale={props.locale} pathname={`/models/${props.config.slug}`}>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: stringifyJsonLd(schema) }} />
      <main data-model-kind={mediaKind} className="model-detail-page model-prototype home-landing relative overflow-x-hidden bg-white text-[#171a21] dark:bg-[#050507] dark:text-white">
        <section className="model-hero" id="overview">
          <div className="model-container">
            <div className="model-hero-top">
              <ModelLandingBreadcrumb
                locale={props.locale}
                modelName={props.config.modelId}
                t={props.t}
                className="model-breadcrumb"
              />
              <div className="model-hero-actions">
                <a href={localizePath("/models", props.locale)} className="model-back-link">
                  <ArrowLeft aria-hidden="true" />
                  {props.t("Back to Models")}
                </a>
                {generator ? (
                  <a
                    href={runHref}
                    onClick={props.onRunClick}
                    className="model-view-api model-playground-hero-link"
                  >
                    <Play className="size-4" aria-hidden="true" />
                    {props.t("Open in Playground")}
                  </a>
                ) : null}
                <a
                  href={dashboardHref}
                  onClick={navigateToHref}
                  className="flatkey-hero-cta inline-flex h-10 items-center gap-2 px-4 text-sm font-medium shadow-[0_16px_34px_-18px_rgba(124,58,237,0.85)]"
                  style={{ borderRadius: "0.5rem" }}
                >
                  {props.t(heroContent?.actionLabel ?? "Get started")}
                </a>
                <a href="#api" className="model-view-api">
                  {props.t("View API")}
                </a>
              </div>
            </div>

            <div className="model-hero-row">
              <div className="model-hero-copy-block">
                <div className="model-title-line">
                  {heroContent?.logo ? (
                    <Image
                      src={heroContent.logo}
                      alt={props.config.displayName}
                      width={40}
                      height={40}
                      className="model-custom-logo block shrink-0 object-contain object-center"
                    />
                  ) : (
                    <HomeModelLogo
                      iconKey={model?.icon ?? model?.vendor_icon}
                      modelName={props.config.modelId}
                      vendor={providerName}
                      fallback={props.config.modelId.slice(0, 1)}
                      surfaceSize={40}
                      imageSize={28}
                    />
                  )}
                  <h1>{heroTitle}</h1>
                </div>
                <div className="model-id-line">
                  <Link href={localizePath(`/models/${props.config.slug}`, props.locale)}>{props.config.modelId}</Link>
                  <button
                    type="button"
                    onClick={() => navigator.clipboard?.writeText(props.config.modelId).catch(() => undefined)}
                    aria-label={props.t("Copy model id")}
                  >
                    <Copy aria-hidden="true" />
                  </button>
                </div>
                <p className="model-hero-copy">{heroContent?.description ? props.t(heroContent.description) : modelDescription}</p>
                {heroTags.length > 0 ? (
                  <div className="model-hero-capabilities">
                    <div className="model-hero-tags">
                      {heroTags.map((tag) => <span className="model-tag" key={tag}>{tag}</span>)}
                    </div>
                  </div>
                ) : null}
              </div>

              <div className="model-hero-stats">
                {showsTokenPriceBreakdown ? (
                  <>
                    {tokenPriceRows.map((row) => (
                      <div className="model-stat-card" key={row.label}>
                        <div className="model-stat-label">{props.t(row.label)}</div>
                        <div className="model-stat-reference">
                          {props.t("Reference price")}: <s>{row.official}</s>
                        </div>
                        <div className="model-stat-value">{row.flatkey}</div>
                      </div>
                    ))}
                    <div className="model-stat-card">
                      <div className="model-stat-label">{props.t("Provider")}</div>
                      <div className="model-stat-value">{heroProvider}</div>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="model-stat-card">
                      <div className="model-stat-label">{props.t("Provider")}</div>
                      <div className="model-stat-value">{heroProvider}</div>
                    </div>
                    <div className="model-stat-card">
                      <div className="model-stat-label">{isMiniMaxCatalogFallback ? props.t("MiniMax-H3 768P / sec") : props.t("Reference price")}</div>
                      <div className="model-stat-value">
                        {isImageCatalogFallback || isMiniMaxCatalogFallback || !model
                          ? unavailablePrice
                          : <HeroPriceBreakdown rows={heroOfficialPriceRows} reference />}
                      </div>
                    </div>
                    <div className="model-stat-card">
                      <div className="model-stat-label">{isMiniMaxCatalogFallback ? props.t("MiniMax-H3 768P / sec") : props.t("Flatkey price")}</div>
                      <div className="model-stat-value">
                        {isImageCatalogFallback || isMiniMaxCatalogFallback || !model
                          ? unavailablePrice
                          : <HeroPriceBreakdown rows={heroFlatkeyPriceRows} />}
                      </div>
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
        </section>

        <ModelSectionNav generator={generator} showPricing t={props.t} />

        {generator ? (
          <section id="workbench" className="model-section model-playground playground">
            <div className="model-container">
              <span className="sr-only">
                {props.t("Generator setup")} · {props.t("Playground (edit before sign-up)")}
              </span>
              <div className="workbench">
                <div id="parameters" className="panel model-panel">
                  <PanelHeader title={props.t("Input")} right="" />
                  <MediaPromptEditor
                    generator={generator}
                    modelId={props.config.modelId}
                    locale={props.locale}
                    prompt={props.prompt}
                    fieldValues={props.fieldValues}
                    referenceImages={props.referenceImages}
                    onMediaUploadCountChange={props.onMediaUploadCountChange}
                    onPromptChange={props.onPromptChange}
                    onFieldChange={props.onFieldChange}
                    onReferenceImagesChange={props.onReferenceImagesChange}
                    t={props.t}
                  />
                  <a href={runHref} onClick={props.onRunClick} className="generate-button">
                    {props.t("Start generating")}
                  </a>
                </div>

                <div className="panel model-panel">
                  <PanelHeader title={props.t("Output")} right={props.t("Preview")} />
                  <OutputPreview
                    modelName={props.config.displayName}
                    modelId={props.config.modelId}
                    locale={props.locale}
                    fallbackVideo={props.config.landingContent?.promptLibrary?.find((item) => Boolean(item.video))}
                    prompt={props.prompt}
                    kind={generator.kind}
                    protocol={generator.protocol}
                    endpoint={generator.endpoint}
                    fieldValues={props.fieldValues}
                    referenceCount={mediaReferenceCount}
                    t={props.t}
                  />
                  <div className="model-output-actions">
                    <a
                      href={runHref}
                      onClick={props.onRunClick}
                      className="outline-button"
                    >
                      <Play className="size-4" />
                      {props.t("Open in Playground")}
                    </a>
                    <a
                      href={runHref}
                      onClick={props.onRunClick}
                      className="outline-button"
                    >
                      <KeyRound className="size-4" />
                      {props.t("Get started")}
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
        ) : null}

        <div id="health" className="model-anchor-alias" aria-hidden="true" />
        <section id="performance" className="model-section performance">
          <div className="model-container">
            <FlatkeySectionHeading
              eyebrow={props.t(landingContent?.performance?.eyebrow ?? "Performance")}
              title={props.t(landingContent?.performance?.title ?? "Reliability over the last 30 days")}
              description={props.t(landingContent?.performance?.description ?? "Measured on real Flatkey traffic, with production monitoring.")}
            />
            <div className="metrics">
              {landingContent?.performance?.metrics ? (
                landingContent.performance.metrics.map((metric, index) => {
                  const liveValue = metric.icon === "latency"
                    ? formatLatencyMs(ttft)
                    : metric.icon === "requests"
                      ? formatCallCount(summary?.request_count)
                      : formatSuccessRate(successRate);
                  // Prototype metrics are useful before telemetry has enough
                  // samples, but must never hide a live API value.
                  const value = hasLiveHealthData && liveValue !== "—" ? liveValue : metric.value ?? liveValue;
                  const icon = <PerformanceMetricIcon name={metric.icon} />;
                  return (
                    <FlatkeyMetricCard
                      key={`${metric.label}-${index}`}
                      label={props.t(metric.label)}
                      value={value}
                      note={props.t(metric.note)}
                      icon={icon}
                    />
                  );
                })
              ) : (
                <>
                  <FlatkeyMetricCard label={props.t("Avg. provider uptime")} value={formatSuccessRate(successRate)} note={props.t("last 30 days")} icon={<PerformanceMetricIcon name="uptime" />} />
                  <FlatkeyMetricCard label={props.t("Latency")} value={formatLatencyMs(ttft)} note={props.t("last 30 days")} icon={<PerformanceMetricIcon name="latency" />} />
                  <FlatkeyMetricCard label={props.t("Requests")} value={formatCallCount(summary?.request_count)} note={props.t("30-day window")} icon={<PerformanceMetricIcon name="requests" />} />
                </>
              )}
            </div>
            <div className="performance-chart">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <div className="text-sm font-semibold">{props.t("Successful inference trend")}</div>
                {rankingRow ? (
                  <div className="rounded-full bg-violet-500/10 px-3 py-1 text-xs font-semibold text-violet-700">
                    #{rankingRow.rank} · {formatCallCount(displayRankingTokens(rankingRow.total_tokens))}
                  </div>
                ) : null}
              </div>
              <div className="h-24">
                {trend.length > 0 ? (
                  <DailyHealthBars points={trend} label={props.t("Uptime")} heightPx={96} />
                ) : (
                  <div className="flex h-full items-center justify-center rounded-xl bg-violet-500/5 text-sm text-muted-foreground">
                    {props.t("Not enough data yet")}
                  </div>
                )}
              </div>
            </div>
          </div>
        </section>

        <ModelActivitySection config={props.config} trend={trend} summary={summary} t={props.t} />

        {props.config.slug === "seedance-2.5" ? (
          <SeedancePricingSection config={props.config} rows={priceRows.rows} note={priceRows.note} t={props.t} />
        ) : (
          <ModelPricingSection
            config={props.config}
            content={pricingContent}
            liveRows={priceRows.rows}
            liveNote={priceRows.note}
            allowPlayground={Boolean(generator)}
            t={props.t}
          />
        )}

        <ModelCapabilitiesSection config={props.config} t={props.t} />

        <ModelComparisonSection config={props.config} t={props.t} />

        {hasPromptLibrary ? (
          <PromptLibrarySection
            config={props.config}
            locale={props.locale}
            examples={examples}
            t={props.t}
          />
        ) : null}

        <ModelWhySection config={props.config} allowPlayground={Boolean(generator)} t={props.t} />

        <ModelApiSection config={props.config} allowPlayground={Boolean(generator)} locale={props.locale} t={props.t} />

        <section id="related" className="model-section related">
          <div className="model-container">
            <FlatkeySectionHeading
              eyebrow={props.t(landingContent?.related?.eyebrow ?? "Related models")}
              title={landingContent?.related?.title ? props.t(landingContent.related.title) : relatedModels.title}
              description={landingContent?.related?.description
                ? props.t(landingContent.related.description)
                : landingContent?.related
                  ? undefined
                  : props.t("Related models from the pricing catalog.")}
            />
            <div className="related-grid">
              {relatedModels.models.map((related) => (
                <Link
                  key={related.href}
                  href={related.href}
                  className="related-card group"
                >
                  {related.asset ? (
                    <RelatedModelCover
                      src={related.asset}
                      alt={related.name}
                      sizes="(min-width: 901px) 25vw, (min-width: 621px) 50vw, 100vw"
                    />
                  ) : null}
                  {!related.asset ? (
                    <div className="related-card-top">
                      <HomeModelLogo
                        modelName={related.name}
                        vendor={related.description}
                        fallback={related.name.charAt(0)}
                        surfaceSize={34}
                        imageSize={22}
                      />
                      <ArrowRight className="related-arrow" />
                    </div>
                  ) : null}
                  <div className="related-copy">
                    <strong>{related.name}</strong>
                    <span>{props.t(related.description)}</span>
                  </div>
                </Link>
              ))}
            </div>
          </div>
        </section>

        <section id="faq" className="model-section faq">
          <div className="model-container faq-layout">
            <div className="faq-intro">
              <FlatkeySectionHeading
                eyebrow={props.t("FAQ")}
                title={props.t(landingContent?.faqTitle ? landingContent.faqTitle.beforeBreak : "Frequently asked questions")}
                titleNode={landingContent?.faqTitle ? (
                  <>
                    {props.t(landingContent.faqTitle.beforeBreak)}
                    <br />
                    {" "}
                    {props.t(landingContent.faqTitle.afterBreak)}
                  </>
                ) : undefined}
                description={props.t(landingContent?.faqDescription ?? (landingContent?.faq
                  ? "Pricing, compatibility, limits, and how your prompts and generated files are handled."
                  : "Use the pricing section above for current Flatkey prices from our pricing API."))}
              />
            </div>
            <div className="faq-list">
              {faqItems.map((item) => (
                <details key={item.question} className="faq-item">
                  <summary className="faq-toggle">
                    <span>{item.question}</span>
                    <span aria-hidden="true">›</span>
                  </summary>
                  <div className="faq-answer">{item.answer}</div>
                </details>
              ))}
            </div>
          </div>
        </section>
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
                  {buildHumanReadableModelTitle(props.config, props.locale)}
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
                <Pill label={props.t("Pricing")} value={props.primaryLiveModel ? props.t("Live catalog model") : props.t("Pricing data unavailable")} />
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
                  locale={props.locale}
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
                  modelId={props.config.modelId}
                  locale={props.locale}
                  fallbackVideo={props.config.landingContent?.promptLibrary?.find((item) => Boolean(item.video))}
                  prompt={props.prompt}
                  kind={generator.kind}
                  protocol={generator.protocol}
                  endpoint={generator.endpoint}
                  fieldValues={props.fieldValues}
                  referenceCount={props.referenceImages.length}
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
                    {props.t("Get started")}
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
                    <span className="mt-1 block text-[10px] tracking-[0.14em] text-emerald-600/70 uppercase">{props.t("vs")} {props.config.officialName}</span>
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
            <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">{props.t("FAQ")}</div>
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
                  {buildHumanReadableModelTitle(props.config, props.locale)}
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
              <div className="text-xs font-extrabold tracking-[0.16em] text-[#7c3aed] uppercase">{props.t("FAQ")}</div>
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
      aria-label={props.t("Breadcrumb")}
      className={`flex min-w-0 flex-wrap items-center gap-1 text-xs text-[#6B6475] dark:text-slate-300/72 ${props.className ?? ""}`}
    >
      <Link href={localizePath("/", props.locale)} className="hover:text-[#0B0B0F] dark:hover:text-white">
        Flatkey
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

function MediaPromptEditor(props: {
  generator: NonNullable<ModelConfig["generator"]>;
  modelId: string;
  locale: Locale;
  prompt: string;
  fieldValues: Record<string, string | number | boolean>;
  referenceImages: ReferenceImageDraft[];
  onMediaUploadCountChange?: (kind: MediaUploadKind, count: number) => void;
  onPromptChange: (prompt: string) => void;
  onFieldChange: (name: string, value: string | number | boolean) => void;
  onReferenceImagesChange: (images: ReferenceImageDraft[]) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const fields = useMemo(() => {
    if (props.generator.kind !== "video") return props.generator.fields;

    // Keep the controls in the same reading order as the approved prototype:
    // ratio, resolution/size, duration, then audio. Generator configs are shared
    // with the request builder and historically put resolution first, so sort
    // only the presentation list instead of changing request semantics.
    const prototypeOrder = ["ratio", "resolution", "size", "duration", "generate_audio"];
    return [...props.generator.fields].sort((left, right) => {
      const leftIndex = prototypeOrder.indexOf(left.name);
      const rightIndex = prototypeOrder.indexOf(right.name);
      return (leftIndex < 0 ? prototypeOrder.length : leftIndex) - (rightIndex < 0 ? prototypeOrder.length : rightIndex);
    });
  }, [props.generator.kind, props.generator.fields]);
  const fieldRows = useMemo(() => {
    if (props.generator.kind !== "video") return [fields];
    const leading = fields.filter((field) => field.name === "ratio" || field.name === "resolution" || field.name === "size");
    const secondary = fields.filter((field) => field.name === "duration" || field.name === "generate_audio");
    const rest = fields.filter((field) => !leading.includes(field) && !secondary.includes(field));
    const rows: ModelGeneratorField[][] = [];
    if (leading.length > 0) rows.push(leading);
    if (secondary.length > 0) rows.push(secondary);
    for (let index = 0; index < rest.length; index += 2) rows.push(rest.slice(index, index + 2));
    return rows;
  }, [fields, props.generator.kind]);
  const quickPrompts = props.generator.kind === "video"
    ? ["Product Reveal", "UGC Ad", "Cinematic Scene", "Social Clip"]
    : ["Product Photo", "Anime Portrait", "Realistic Human", "YouTube Thumbnail", "Fantasy Landscape"];
  const videoModeOptions = props.generator.kind === "video" ? (props.generator.videoModes ?? []) : [];
  const selectedVideoMode = videoModeOptions.find((option) => option.value === props.fieldValues.video_mode)?.value
    ?? videoModeOptions.find((option) => option.value === props.generator.defaultVideoMode)?.value
    ?? videoModeOptions.find((option) => option.supported)?.value;
  const referenceImageLimit = props.generator.referenceLimits?.image ?? (props.generator.kind === "image" ? 4 : 0);
  const supportsReferenceImages = props.generator.kind === "image" && referenceImageLimit > 0;

  const onReferenceInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.currentTarget.files ?? []);
    if (files.length === 0) return;
    const remainingSlots = Math.max(0, referenceImageLimit - props.referenceImages.length);
    const nextImages = files.slice(0, remainingSlots).map((file) => ({
      id: `${file.name}-${file.size}-${file.lastModified}`,
      name: file.name,
      size: file.size,
      type: file.type || "image",
      previewUrl: URL.createObjectURL(file),
    }));
    props.onReferenceImagesChange([...props.referenceImages, ...nextImages]);
    event.currentTarget.value = "";
  };

  const removeReferenceImage = (image: ReferenceImageDraft) => {
    URL.revokeObjectURL(image.previewUrl);
    props.onReferenceImagesChange(props.referenceImages.filter((item) => item.id !== image.id));
  };

  return (
    <div className="input-fields">
      {selectedVideoMode && videoModeOptions.length > 0 ? (
        <VideoModeSelector
          locale={props.locale}
          options={videoModeOptions}
          value={selectedVideoMode}
          onChange={(value) => props.onFieldChange("video_mode", value)}
        />
      ) : null}
      <label className="field prompt-field block text-sm font-semibold text-[#2c2d33] dark:text-white/88">
        <span className="field-label">
          <span>{props.t("Prompt")}</span>
        </span>
        <span className="prompt-control">
          <textarea
            value={props.prompt}
            onChange={(event) => props.onPromptChange(event.target.value)}
            className="mt-2 h-[100px] min-h-[100px] w-full resize-y rounded-[1.1rem] border border-[#ded8ea] bg-[#fcfbff] p-4 font-mono text-sm leading-6 font-medium text-[#20222a] shadow-[0_10px_28px_-26px_rgba(76,29,149,.72)] outline-none transition focus:border-[#7c3aed] focus:bg-white focus:ring-4 focus:ring-[#7c3aed]/10"
          />
          <span className="counter" aria-live="polite">{props.prompt.length} / 10000</span>
        </span>
      </label>
      <div className="quick-prompts mt-5">
        <div className="mb-3 text-sm font-semibold text-[#2c2d33] dark:text-white/88">{props.t("Quick Prompts")}</div>
        <div className="flex flex-wrap gap-2.5">
          {quickPrompts.map((item) => (
            <button
              key={item}
              type="button"
              onClick={() => props.onPromptChange(buildQuickPrompt(props.t(item), props.generator.kind, props.locale))}
              className="rounded-xl border border-[#e4deed] bg-[#fcfbff] px-3.5 py-2 text-[13px] font-bold text-[#4f4d56] shadow-[0_10px_20px_-18px_rgba(76,29,149,.45)] transition hover:border-[#7c3aed]/45 hover:bg-white hover:text-[#4c1d95]"
            >
              {props.t(item)}
            </button>
          ))}
        </div>
      </div>
      {props.generator.kind === "video" ? (
        <div className="input-reference-fields mt-6">
          {([
            ["image", props.t("Reference image"), "image/*"],
            ["video", props.t("Reference videos"), "video/*"],
            ["audio", props.t("Reference Audios"), "audio/*"],
          ] as const).map(([kind, label, accept]) => {
            const maxFiles = props.generator.referenceLimits?.[kind] ?? 0;
            if (maxFiles <= 0) return null;
            return (
              <MediaUploadField
                key={kind}
                label={label}
                accept={accept}
                kind={kind}
                maxFiles={maxFiles}
                onCountChange={props.onMediaUploadCountChange}
                t={props.t}
              />
            );
          })}
        </div>
      ) : null}
      <div className="advanced-options mt-6">
        <div className="rounded-[1.35rem] border border-[#e2dbea] bg-[linear-gradient(180deg,#ffffff_0%,#fbf9ff_100%)] p-4 shadow-[0_18px_38px_-32px_rgba(76,29,149,.55)] sm:p-5">
          {fieldRows.map((row, rowIndex) => (
            <div className="field-grid grid grid-cols-1 gap-3.5 sm:grid-cols-6" key={"field-row-" + rowIndex}>
              {row.map((field) => (
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
          ))}
        </div>
      </div>
      {supportsReferenceImages ? (
        <div className="reference-images-field mt-6">
          <div className="mb-3 flex items-center justify-between gap-3">
            <div className="text-sm font-semibold text-[#2c2d33] dark:text-white/88">{props.t("Reference Images")}</div>
            <label className="inline-flex h-10 cursor-pointer items-center gap-2 rounded-lg border border-violet-500/16 bg-white/70 px-4 text-sm font-semibold text-[#4f4d56] hover:border-violet-500/35 hover:bg-violet-500/8 hover:text-violet-700">
              <Upload className="size-3.5" />
              {props.t("Upload reference")}
              <input type="file" accept="image/*" multiple className="sr-only" onChange={onReferenceInputChange} />
            </label>
          </div>
          {props.referenceImages.length > 0 ? (
            <div className="grid gap-3 sm:grid-cols-2">
              {props.referenceImages.map((image) => (
                <div key={image.id} className="grid grid-cols-[3.5rem_1fr_auto] items-center gap-3 rounded-xl border border-violet-500/12 bg-white/70 p-3">
                  <div className="relative aspect-square overflow-hidden rounded-lg bg-white">
                    <Image src={image.previewUrl} alt="" fill sizes="52px" className="object-cover" unoptimized />
                  </div>
                  <div className="min-w-0">
                    <div className="truncate text-xs font-extrabold text-[#2c2d33]">{image.name}</div>
                    <div className="mt-0.5 text-[10px] font-medium text-[#8b8891]">{formatUploadedSize(image.size)}</div>
                  </div>
                  <button
                    type="button"
                    onClick={() => removeReferenceImage(image)}
                    className="grid size-8 place-items-center rounded-lg text-[#706a74] hover:bg-violet-500/8 hover:text-violet-700"
                    aria-label={props.t("Remove reference image")}
                  >
                    <Trash2 className="size-3.5" />
                  </button>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function VideoModeSelector(props: {
  locale: Locale;
  options: readonly ModelVideoModeOption[];
  value: ModelVideoMode;
  onChange: (value: ModelVideoMode) => void;
}) {
  const copy = getModelVideoModeUiCopy(props.locale);
  const selectedOption = props.options.find((option) => option.value === props.value) ?? props.options[0];

  if (!selectedOption) return null;

  return (
    <label
      data-video-mode-selector
      data-video-mode-value={selectedOption.value}
      className="grid w-full min-w-0 gap-1.5 text-[11px] leading-5 font-extrabold tracking-normal text-[#77717f] uppercase"
    >
      <span>{copy.label}</span>
      <select
        value={selectedOption.value}
        aria-describedby="seedance-video-mode-help"
        data-video-mode-options
        onChange={(event) => {
          const nextMode = props.options.find((option) => option.value === event.currentTarget.value);
          if (nextMode?.supported) props.onChange(nextMode.value);
        }}
        data-video-mode-control
        className="h-9 w-full min-w-0 appearance-none rounded-lg border border-[#ded8ea] bg-white px-3.5 pr-9 text-sm font-bold tracking-normal text-[#20222a] shadow-[0_10px_22px_-20px_rgba(76,29,149,.5)] outline-none transition focus:border-[#7c3aed] focus:ring-4 focus:ring-[#7c3aed]/10"
      >
        {props.options.map((option) => {
          const label = getModelVideoModeLabel(props.locale, option.value);
          return (
            <option
              key={option.value}
              disabled={!option.supported}
              data-video-mode={option.value}
              data-video-mode-supported={option.supported}
            >{option.supported ? label : `${label} — ${copy.unavailable}`}</option>
          );
        })}
      </select>
      <span id="seedance-video-mode-help" className="sr-only">{copy.helper}</span>
    </label>
  );
}

type PlaygroundUploadExample = {
  label: string;
  poster: string;
  alt: string;
  video?: string;
};

type PlaygroundUpload = {
  id: string;
  name: string;
  size: number;
  type: string;
  previewUrl: string;
};

function MediaUploadField(props: {
  label: string;
  accept: string;
  kind: "image" | "video" | "audio";
  maxFiles: number;
  example?: PlaygroundUploadExample;
  onCountChange?: (kind: MediaUploadKind, count: number) => void;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const [uploads, setUploads] = useState<PlaygroundUpload[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const uploadsRef = useRef(uploads);
  const { kind, onCountChange } = props;

  useEffect(() => {
    uploadsRef.current = uploads;
  }, [uploads]);

  useEffect(() => {
    onCountChange?.(kind, uploads.length);
  }, [kind, onCountChange, uploads.length]);

  useEffect(() => () => {
    uploadsRef.current.forEach((upload) => URL.revokeObjectURL(upload.previewUrl));
  }, []);

  const addFiles = (files: File[]) => {
    if (files.length === 0) return;
    setUploads((current) => {
      const remaining = Math.max(0, props.maxFiles - current.length);
      if (remaining === 0) return current;
      const next = files.slice(0, remaining).map((file, index) => ({
        id: `${file.name}-${file.size}-${file.lastModified}-${Date.now()}-${index}`,
        name: file.name,
        size: file.size,
        type: file.type || props.kind,
        previewUrl: URL.createObjectURL(file),
      }));
      return [...current, ...next];
    });
  };

  const onUploadChange = (event: ChangeEvent<HTMLInputElement>) => {
    addFiles(Array.from(event.currentTarget.files ?? []));
    event.currentTarget.value = "";
  };

  const onDragOver = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    if (event.dataTransfer.types.includes("Files")) {
      event.dataTransfer.dropEffect = "copy";
      setIsDragging(true);
    }
  };

  const onDragLeave = (event: DragEvent<HTMLLabelElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setIsDragging(false);
  };

  const onDrop = (event: DragEvent<HTMLLabelElement>) => {
    event.preventDefault();
    setIsDragging(false);
    addFiles(Array.from(event.dataTransfer.files));
  };

  const removeUpload = (upload: PlaygroundUpload) => {
    URL.revokeObjectURL(upload.previewUrl);
    setUploads((current) => current.filter((item) => item.id !== upload.id));
  };

  return (
    <div className="field upload-field">
      <div className="field-label">
        <span>{props.label}</span>
        <span className="upload-limit" aria-live="polite">{uploads.length} / {props.maxFiles}</span>
      </div>
      <label
        className={`upload-zone${uploads.length > 0 ? " has-uploads" : ""}${isDragging ? " is-dragging" : ""}`}
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
      >
        <input type="file" accept={props.accept} multiple={props.maxFiles > 1} onChange={onUploadChange} />
        {uploads.length === 0 && props.example ? (
          <span className="upload-example" aria-hidden="true">
            {props.example.video ? (
              <video
                className="upload-example-media"
                src={props.example.video}
                poster={props.example.poster}
                muted
                loop
                autoPlay
                playsInline
              />
            ) : (
              <Image
                src={props.example.poster}
                alt={props.example.alt}
                width={58}
                height={44}
                className="upload-example-media"
              />
            )}
            <span className="upload-example-copy">
              <strong>{props.example.label}</strong>
              <span>{props.t("Upload or drag and drop")}</span>
            </span>
          </span>
        ) : uploads.length === 0 ? (
          <span className="upload-copy">{props.t("Upload or drag and drop")}</span>
        ) : (
          <span className="upload-filled">
            <span className="upload-file-list" aria-label={props.t("Uploaded {{count}} files", { count: String(uploads.length) })}>
              {uploads.map((upload) => (
                <span className="upload-file-item" key={upload.id}>
                  <span className="upload-file-preview">
                    {props.kind === "video" ? (
                      <video src={upload.previewUrl} muted loop autoPlay playsInline />
                    ) : props.kind === "image" ? (
                      // Blob URLs are client-local preview sources, not remote
                      // image assets. Use a native image element here so the
                      // preview renders immediately after the file input
                      // changes, without going through Next's image loader.
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={upload.previewUrl} alt="" className="object-cover" />
                    ) : (
                      <Music2 aria-hidden="true" className="size-4" />
                    )}
                  </span>
                  <span className="upload-file-info">
                    <strong title={upload.name}>{upload.name}</strong>
                    <span>{formatUploadedSize(upload.size)}</span>
                  </span>
                  <span
                    role="button"
                    tabIndex={0}
                    className="upload-file-remove"
                    onClick={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      removeUpload(upload);
                    }}
                    onKeyDown={(event) => {
                      if (event.key !== "Enter" && event.key !== " ") return;
                      event.preventDefault();
                      event.stopPropagation();
                      removeUpload(upload);
                    }}
                    aria-label={props.t("Remove {{name}}", { name: upload.name })}
                  >
                    <Trash2 aria-hidden="true" className="size-3.5" />
                  </span>
                </span>
              ))}
            </span>
            <span className="upload-add-copy">
              <Upload aria-hidden="true" className="size-3.5" />
              <span>{props.t("Upload or drag and drop")}</span>
            </span>
          </span>
        )}
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
  // The approved playground uses compact native selects for model options.
  // Keeping the controls native preserves the two-column field rhythm instead
  // of squeezing a long aspect-ratio list into pills.
  const canUseSegmented = false;

  if (props.field.type === "boolean" && props.kind === "video") {
    const enabled = Boolean(props.value);
    return (
      <label className="grid min-w-0 gap-2 text-[11px] font-extrabold tracking-normal text-[#77717f] uppercase">
        <span>{props.t(props.field.label)}</span>
        <span className="relative block">
          <select
            value={enabled ? "on" : "off"}
            onChange={(event) => props.onChange(event.target.value === "on")}
            className="h-11 w-full min-w-0 appearance-none rounded-[0.95rem] border border-[#ded8ea] bg-white px-3.5 pr-9 text-sm font-bold tracking-normal text-[#20222a] shadow-[0_10px_22px_-20px_rgba(76,29,149,.5)] outline-none transition focus:border-[#7c3aed] focus:ring-4 focus:ring-[#7c3aed]/10"
          >
            <option value="on">{props.t("On")}</option>
            <option value="off">{props.t("Off")}</option>
          </select>
        </span>
      </label>
    );
  }

  if (props.field.type === "boolean") {
    return (
      <label className="flex min-h-[4.55rem] min-w-0 cursor-pointer items-center justify-between gap-4 rounded-[1.05rem] border border-[#ded8ea] bg-white px-4 py-3 text-sm font-extrabold text-[#5d5b64] shadow-[0_10px_22px_-20px_rgba(76,29,149,.5)] transition hover:border-[#7c3aed]/35 hover:bg-[#fdfcff]">
        <span className="min-w-0 leading-5">{props.t(props.field.label)}</span>
        <span className="relative inline-flex h-7 w-12 shrink-0 items-center">
          <input
            type="checkbox"
            checked={Boolean(props.value)}
            onChange={(event) => props.onChange(event.target.checked)}
            className="peer sr-only"
          />
          <span className="absolute inset-0 rounded-full bg-violet-500/14 transition-colors peer-checked:bg-violet-600" />
          <span className="absolute left-1 size-5 rounded-full bg-white shadow-sm transition-transform peer-checked:translate-x-5" />
        </span>
      </label>
    );
  }

  if (props.field.type === "number" && props.field.name === "n") {
    const min = props.field.min ?? 1;
    const max = props.field.max ?? 10;

    return (
      <label className="grid min-w-0 gap-2 text-[11px] font-extrabold tracking-normal text-[#77717f] uppercase">
        <span>{props.t(props.field.label)}</span>
        <input
          type="number"
          min={min}
          max={max}
          value={String(props.value)}
          onChange={(event) => props.onChange(coerceGeneratorValue(props.field, event.target.value))}
          className="h-9 w-full min-w-0 appearance-none rounded-[0.95rem] border border-[#ded8ea] bg-white px-3.5 text-sm font-bold tracking-normal text-[#20222a] shadow-[0_10px_22px_-20px_rgba(76,29,149,.5)] outline-none transition focus:border-[#7c3aed] focus:ring-4 focus:ring-[#7c3aed]/10"
        />
      </label>
    );
  }

  if (canUseSegmented) {
    return (
      <div className="grid min-w-0 gap-2 text-[11px] font-extrabold tracking-normal text-[#77717f] uppercase">
        <span>{props.t(props.field.label)}</span>
        <div className={`${segmentedGridClass(props.field)} min-h-11 gap-1.5 rounded-[0.95rem] border border-[#ded8ea] bg-[#f3f0f9] p-1 shadow-[inset_0_1px_0_rgba(255,255,255,.8)]`}>
          {(props.field.options ?? []).map((item) => {
            const active = String(props.value) === item;
            return (
              <button
                key={item}
                type="button"
                onClick={() => props.onChange(coerceGeneratorValue(props.field, item))}
                className={`min-h-9 min-w-0 rounded-[0.72rem] px-3 py-2 text-[13px] leading-5 font-extrabold tracking-normal transition ${
                  active
                    ? "bg-white text-[#4c1d95] shadow-[0_8px_18px_-12px_rgba(76,29,149,.85)] ring-1 ring-[#7c3aed]/20"
                    : "text-[#5d5b64] hover:bg-white/75 hover:text-[#3f236b]"
                }`}
              >
                <span className="block">{item}</span>
              </button>
            );
          })}
        </div>
        {props.field.help ? <span className="text-[10px] font-medium tracking-normal text-[#8b8891] normal-case">{props.t(props.field.help)}</span> : null}
      </div>
    );
  }

  return (
    <label className="grid min-w-0 gap-2 text-[11px] font-extrabold tracking-normal text-[#77717f] uppercase">
      <span>{props.t(props.field.label)}</span>
      {props.field.type === "select" || (props.kind === "video" && props.field.name === "duration") ? (
        <span className="relative block">
          <select
            value={String(props.value)}
            onChange={(event) => props.onChange(coerceGeneratorValue(props.field, event.target.value))}
            className="h-11 w-full min-w-0 appearance-none rounded-[0.95rem] border border-[#ded8ea] bg-white px-3.5 pr-9 text-sm font-bold tracking-normal text-[#20222a] shadow-[0_10px_22px_-20px_rgba(76,29,149,.5)] outline-none transition focus:border-[#7c3aed] focus:ring-4 focus:ring-[#7c3aed]/10"
          >
            {(props.field.options ?? (props.kind === "video" && props.field.name === "duration"
              ? Array.from(
                  { length: (props.field.max ?? 30) - (props.field.min ?? 1) + 1 },
                  (_, index) => String((props.field.min ?? 1) + index)
                )
              : [])).map((item) => (
              <option key={item} value={item}>{item}</option>
            ))}
          </select>
        </span>
      ) : (
        <input
          type={props.field.type === "number" ? "number" : "text"}
          min={props.field.min}
          max={props.field.max}
          value={String(props.value)}
          onChange={(event) => props.onChange(coerceGeneratorValue(props.field, event.target.value))}
          className="h-11 w-full min-w-0 rounded-[0.95rem] border border-[#ded8ea] bg-white px-3.5 text-sm font-bold tracking-normal text-[#20222a] shadow-[0_10px_22px_-20px_rgba(76,29,149,.5)] outline-none transition focus:border-[#7c3aed] focus:ring-4 focus:ring-[#7c3aed]/10"
        />
      )}
      {props.field.help ? <span className="text-[10px] font-medium tracking-normal text-[#8b8891] normal-case">{props.t(props.field.help)}</span> : null}
    </label>
  );
}

function generatorFieldColumnClass(kind: NonNullable<ModelConfig["generator"]>["kind"], field: ModelGeneratorField) {
  if (kind === "image") {
    if (field.name === "n") return "sm:col-span-2";
    if (field.name === "size") return "sm:col-span-6";
    return "sm:col-span-3";
  }
  if (kind === "video") {
    if (field.name === "ratio") return "sm:col-span-6";
    if (field.name === "resolution" || field.name === "duration" || field.name === "frames" || field.name === "seed") {
      return "sm:col-span-3";
    }
    return "sm:col-span-2";
  }
  return "sm:col-span-3";
}

function segmentedGridClass(field: ModelGeneratorField) {
  if (field.name === "ratio") {
    const optionCount = field.options?.length ?? 0;
    if (optionCount >= 7) return "grid grid-cols-2 min-[900px]:grid-cols-4 min-[1280px]:grid-cols-7";
    if (optionCount === 5) return "grid grid-cols-2 min-[900px]:grid-cols-3 min-[1280px]:grid-cols-5";
  }
  if (field.name === "size") return "grid grid-cols-2 min-[1280px]:grid-cols-4";
  if ((field.options?.length ?? 0) === 4) return "grid grid-cols-2";
  if ((field.options?.length ?? 0) === 3) return "grid grid-cols-3";
  return "grid grid-cols-2";
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

function OutputPreview(props: {
  modelName: string;
  modelId: string;
  locale: Locale;
  fallbackVideo?: MediaExample;
  prompt: string;
  kind: "image" | "video" | "audio";
  protocol?: ModelGeneratorProtocol;
  endpoint: string;
  fieldValues: Record<string, string | number | boolean>;
  referenceCount: number;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const endpoint = props.endpoint;
  const protocol = props.protocol ?? (props.kind === "image" ? "openai-image" : props.kind === "video" ? "seedance-video" : "audio");
  // Models without a reviewed profession batch keep their own configured
  // sample. Prefer it over the global media sample so one model never shows
  // another model's asset in the Playground preview.
  const videoTemplates = props.kind === "video" ? getVideoPromptTemplates(props.modelId, props.locale) : [];
  const videoExample = props.kind === "video"
    ? videoTemplates[0] ?? props.fallbackVideo ?? MEDIA_EXAMPLES.video[0]
    : undefined;
  const originalVideoExample = videoTemplates[0]
    ? VIDEO_PROMPT_TEMPLATES.find((template) => template.professionId === videoTemplates[0]?.professionId)
    : undefined;
  const localVideoExample = originalVideoExample ?? (props.fallbackVideo?.video && !isRemoteMedia(props.fallbackVideo.video)
    ? props.fallbackVideo
    : MEDIA_EXAMPLES.video.find((example) => example.video && !isRemoteMedia(example.video)));
  const videoFallbackSrc = videoExample?.video && localVideoExample?.video !== videoExample.video
    ? localVideoExample?.video
    : undefined;
  const videoFallbackPoster = props.kind === "video"
    ? getVideoPromptTemplateLocalFallbackPoster(props.modelId, videoExample?.professionId) ?? props.fallbackVideo?.fallbackPoster ?? localVideoExample?.poster
    : undefined;
  const imageExample = props.kind === "image" ? getImagePlaygroundExample(props.modelId, props.locale) : undefined;
  const field = (name: string, fallback: string | number | boolean) => props.fieldValues[name] ?? fallback;
  const rows = props.kind === "video"
    ? [
        [props.t("Endpoint"), `POST ${endpoint}`],
        [props.t("Model ID"), props.modelName],
        ...(protocol === "grok-video"
          ? []
          : protocol === "veo-video"
            ? [[props.t("Size"), String(field("size", "1280x720"))]]
            : [
                [props.t("Aspect ratio"), String(field("ratio", "adaptive"))],
                [props.t("Resolution"), String(field("resolution", "720p"))],
              ]),
        [props.t("Duration"), `${field("duration", protocol === "veo-video" ? "8" : 5)}s`],
        ...(protocol === "seedance-video"
          ? [[props.t("Generate audio"), field("generate_audio", true) ? props.t("On") : props.t("Off")]]
          : []),
        [props.t("Reference media"), props.referenceCount > 0 ? `${props.referenceCount}` : props.t("None")],
      ]
    : props.kind === "image"
      ? [
          [props.t("Endpoint"), `POST ${endpoint}`],
          [props.t("Model ID"), props.modelName],
          ...(protocol === "gemini-image"
            ? [
                [props.t("Aspect ratio"), String(field("aspect_ratio", "1:1"))],
                ...(props.fieldValues.image_size ? [[props.t("Size"), String(field("image_size", "1K"))]] : []),
              ]
            : protocol === "openai-image" && props.fieldValues.resolution
              ? [
                  [props.t("Resolution"), String(field("resolution", "1k"))],
                  [props.t("Quality"), String(field("quality", "medium"))],
                  [props.t("Aspect ratio"), String(field("aspect_ratio", "auto"))],
                ]
              : [
                  [props.t("Size"), String(field("size", "1024x1024"))],
                  [props.t("Quality"), String(field("quality", "standard"))],
                ]),
          ...(protocol === "openai-image" && props.fieldValues.resolution
            ? [[props.t("Output format"), String(field("response_format", "url"))]]
            : []),
          ...(protocol === "gemini-image" ? [] : [[props.t("Outputs"), String(field("n", 1))]]),
        ]
      : [
          [props.t("Endpoint"), `POST ${endpoint}`],
          [props.t("Model ID"), props.modelName],
          [props.t("Duration"), `${field(endpoint.includes("video-to-music") ? "duration_seconds" : "duration", 30)}s`],
          [props.t("Output format"), String(field("output_format", "mp3"))],
          [props.t("Outputs"), String(field(endpoint.includes("video-to-music") ? "variants_num" : "variants", 1))],
        ];
  return (
    <div className="output-preview">
      <div className={`video-preview ${props.kind === "image" ? "image-preview" : ""}`}>
        {videoExample?.video ? (
          <VideoPreviewMedia
            key={videoExample.video}
            src={videoExample.video}
            poster={videoExample.poster || videoFallbackPoster || undefined}
            fallbackSrc={videoFallbackSrc}
            fallbackPoster={videoFallbackPoster}
            alt={props.t("Video preview")}
          />
        ) : imageExample ? (
          <Image
            src={imageExample.poster}
            alt={props.t("Image preview")}
            fill
            sizes="(min-width: 1024px) 40vw, 100vw"
            className="preview-media object-cover"
            unoptimized
          />
        ) : (
          <span className="preview-label">
            {props.kind === "image" ? props.t("Image preview") : props.t("Audio preview")}
          </span>
        )}
      </div>
      <div className="summary">
        <h3>{props.t("Request summary")}</h3>
        <dl className="summary-list">
          {rows.map(([label, value]) => <Fragment key={label}><dt>{label}</dt><dd>{value}</dd></Fragment>)}
        </dl>
        <p className="summary-help">{props.t("Preview only. Sign in to submit this request to Flatkey.")}</p>
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
            <CdnFallbackVideo
              key={activeExample.video}
              className="h-full w-full object-cover"
              autoPlay
              controls
              loop
              muted
              playsInline
              poster={activeExample.poster}
              preload="metadata"
              src={activeExample.video}
              fallbackPoster={activeExample.fallbackPoster}
              fallbackAlt={props.t("Video preview")}
            />
          ) : (
            <CdnFallbackImage
              key={activeExample.poster}
              src={activeExample.poster}
              alt=""
              fallbackSrc={activeExample.fallbackPoster}
              className="absolute inset-0 size-full object-cover"
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
              <CdnFallbackImage
                key={example.poster}
                src={example.poster}
                alt=""
                fallbackSrc={example.fallbackPoster}
                className="absolute inset-0 size-full object-cover"
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
            </div>
            <div className="mt-4 min-w-0">
              <h4 className="model-related-name text-base font-extrabold text-[#17151d]">{model.name}</h4>
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
    return t("More models from {{provider}}", { provider: config.officialName });
  }
  return t("Keep exploring Flatkey");
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

/**
 * Keep the compact hero tags useful without presenting a generic
 * "Capabilities" heading.  For live catalog models, only show modality and
 * control tags that are backed by directory metadata, endpoint names, or the
 * configured generator fields.  Dedicated editorial pages can therefore
 * expose their documented reference/audio controls while generic pages do not
 * inherit the same three video labels indiscriminately.
 */
function buildHeroTags(
  config: ModelConfig,
  model: PricingModel | null,
  t: (key: string, vars?: Record<string, string>) => string
) {
  const generator = config.generator;
  const fields = new Set((generator?.fields ?? []).map((field) => field.name.toLowerCase()));
  const endpoints = (model?.supported_endpoint_types ?? []).map((endpoint) => normalizeModelId(endpoint));
  const modalities = new Set((model?.directory_metadata?.modalities ?? []).map((modality) => modality.toLowerCase()));
  const modelName = normalizeModelId(model?.model_name ?? config.modelId);
  const hasEndpoint = (...needles: string[]) => needles.some((needle) => endpoints.some((endpoint) => endpoint.includes(needle)));
  const hasField = (...needles: string[]) => needles.some((needle) => Array.from(fields).some((field) => field.includes(needle)));
  const translateUnique = (keys: string[]) => Array.from(new Set(keys)).map((key) => t(key));

  if (generator?.kind === "video") {
    const tags: string[] = [];
    const knownReferenceModel = /seedance|minimax|kling|sora|veo/.test(modelName);
    if (modalities.has("image") || hasField("image", "reference") || knownReferenceModel) tags.push("Image to Video");
    if (modalities.has("video") || modalities.has("audio") || hasField("reference", "frame") || knownReferenceModel) {
      tags.push("Reference-guided Video");
    }
    if (hasField("audio") || modalities.has("audio")) tags.push("Audio generation");
    if (hasField("duration")) tags.push("Short-form Video");
    if (tags.length === 0 || (tags.length === 1 && !modalities.has("image"))) tags.unshift("Video generation");
    return translateUnique(tags);
  }
  if (generator?.kind === "image") {
    const tags = ["Text to Image"];
    if (modalities.has("image") || hasField("reference", "image", "input")) tags.push("Reference-guided Image");
    if (hasEndpoint("image-generation") || hasField("edit", "mask", "background")) tags.push("Image Editing");
    return translateUnique(tags);
  }
  if (generator?.kind === "audio") {
    const isVideoToAudio = generator.endpoint.includes("video-to-music") || hasEndpoint("video-to-music");
    return translateUnique(isVideoToAudio
      ? ["Video to Audio", "Speech preservation", "Audio generation"]
      : ["Text to Audio", "Speech synthesis", "Audio generation"]);
  }

  const tags = ["Chat and coding"];
  if ((model?.directory_metadata?.context_tokens ?? 0) > 0 || /context|long/.test(modelName)) tags.push("Long context");
  if (hasEndpoint("response", "chat", "completion", "message") || modalities.has("text")) tags.push("Tool workflows");
  return translateUnique(tags);
}

function FlatkeySectionHeading(props: { eyebrow?: string; title: string; titleNode?: ReactNode; description?: string }) {
  return (
    <div className="section-head">
      {props.eyebrow ? <p className="eyebrow">{props.eyebrow}</p> : null}
      <h2 className="section-title">{props.titleNode ?? props.title}</h2>
      {props.description ? (
        <p className="section-copy">{props.description}</p>
      ) : null}
    </div>
  );
}

type ModelTabIconName = "playground" | "performance" | "parameters" | "activity" | "api" | "faq";

const MODEL_TAB_ICONS: Record<ModelTabIconName, ReactNode> = {
  playground: <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false"><path fillRule="evenodd" clipRule="evenodd" d="M3.79166 1.02081C3.40436 1.02081 2.97161 1.04749 2.60494 1.07753C1.78842 1.14442 1.14443 1.78841 1.07754 2.60492C1.04751 2.97159 1.02083 3.40435 1.02083 3.79165C1.02083 4.17894 1.04751 4.6117 1.07754 4.97837C1.14443 5.79489 1.78842 6.43888 2.60494 6.50576C2.97161 6.5358 3.40436 6.56248 3.79166 6.56248C4.17896 6.56248 4.61171 6.5358 4.97839 6.50576C5.7949 6.43888 6.43889 5.79489 6.50578 4.97837C6.53582 4.6117 6.56249 4.17894 6.56249 3.79165C6.56249 3.40435 6.53582 2.9716 6.50578 2.60492C6.43889 1.78841 5.7949 1.14442 4.97839 1.07753C4.61171 1.04749 4.17896 1.02081 3.79166 1.02081ZM2.70019 2.2403C3.05375 2.21134 3.45051 2.18748 3.79166 2.18748C4.13281 2.18748 4.52958 2.21134 4.88313 2.2403C5.1305 2.26057 5.32274 2.45281 5.34301 2.70017C5.37197 3.05373 5.39583 3.4505 5.39583 3.79165C5.39583 4.1328 5.37197 4.52956 5.34301 4.88312C5.32274 5.13049 5.1305 5.32273 4.88313 5.34299C4.52958 5.37195 4.13281 5.39581 3.79166 5.39581C3.45051 5.39581 3.05375 5.37195 2.70019 5.34299C2.45282 5.32273 2.26058 5.13049 2.24032 4.88312C2.21135 4.52956 2.18749 4.1328 2.18749 3.79165C2.18749 3.4505 2.21135 3.05373 2.24032 2.70017C2.26058 2.45281 2.45282 2.26057 2.70019 2.2403Z" fill="currentColor"/><path fillRule="evenodd" clipRule="evenodd" d="M3.79166 7.43748C3.40436 7.43748 2.97161 7.46416 2.60494 7.4942C1.78842 7.56108 1.14443 8.20507 1.07754 9.02159C1.04751 9.38826 1.02083 9.82102 1.02083 10.2083C1.02083 10.5956 1.04751 11.0284 1.07754 11.395C1.14443 12.2116 1.78842 12.8555 2.60494 12.9224C2.97161 12.9525 3.40436 12.9791 3.79166 12.9791C4.17896 12.9791 4.61171 12.9525 4.97839 12.9224C5.7949 12.8555 6.43889 12.2116 6.50578 11.395C6.53582 11.0284 6.56249 10.5956 6.56249 10.2083C6.56249 9.82102 6.53582 9.38826 6.50578 9.02159C6.43889 8.20507 5.7949 7.56108 4.97839 7.4942C4.61171 7.46416 4.17896 7.43748 3.79166 7.43748ZM2.70019 8.65697C3.05375 8.628 3.45051 8.60415 3.79166 8.60415C4.13281 8.60415 4.52958 8.628 4.88313 8.65697C5.1305 8.67723 5.32274 8.86947 5.34301 9.11684C5.37197 9.4704 5.39583 9.86716 5.39583 10.2083C5.39583 10.5495 5.37197 10.9462 5.34301 11.2998C5.32274 11.5472 5.1305 11.7394 4.88313 11.7597C4.52958 11.7886 4.13281 11.8125 3.79166 11.8125C3.45051 11.8125 3.05375 11.7886 2.70019 11.7597C2.45282 11.7394 2.26058 11.5472 2.24032 11.2998C2.21135 10.9462 2.18749 10.5495 2.18749 10.2083C2.18749 9.86716 2.21135 9.4704 2.24032 9.11684C2.26058 8.86947 2.45282 8.67723 2.70019 8.65697Z" fill="currentColor"/><path fillRule="evenodd" clipRule="evenodd" d="M9.0216 1.07753C9.38828 1.04749 9.82103 1.02081 10.2083 1.02081C10.5956 1.02081 11.0284 1.04749 11.3951 1.07753C12.2116 1.14442 12.8556 1.78841 12.9224 2.60492C12.9525 2.9716 12.9792 3.40435 12.9792 3.79165C12.9792 4.17894 12.9525 4.6117 12.9224 4.97837C12.8556 5.79489 12.2116 6.43888 11.3951 6.50576C11.0284 6.5358 10.5956 6.56248 10.2083 6.56248C9.82103 6.56248 9.38828 6.5358 9.0216 6.50576C8.20509 6.43888 7.5611 5.79489 7.49421 4.97837C7.46417 4.6117 7.43749 4.17894 7.43749 3.79165C7.43749 3.40435 7.46417 2.97159 7.49421 2.60492C7.5611 1.78841 8.20509 1.14442 9.0216 1.07753ZM10.2083 2.18748C9.86718 2.18748 9.47041 2.21134 9.11686 2.2403C8.86949 2.26057 8.67725 2.45281 8.65698 2.70017C8.62802 3.05373 8.60416 3.4505 8.60416 3.79165C8.60416 4.1328 8.62802 4.52956 8.65698 4.88312C8.67725 5.13049 8.86949 5.32273 9.11686 5.34299C9.47041 5.37195 9.86718 5.39581 10.2083 5.39581C10.5495 5.39581 10.9462 5.37195 11.2998 5.34299C11.5472 5.32273 11.7394 5.13049 11.7597 4.88312C11.7886 4.52956 11.8125 4.1328 11.8125 3.79165C11.8125 3.4505 11.7886 3.05373 11.7597 2.70017C11.7394 2.45281 11.5472 2.26057 11.2998 2.2403C10.9462 2.21134 10.5495 2.18748 10.2083 2.18748Z" fill="currentColor"/><path fillRule="evenodd" clipRule="evenodd" d="M10.2083 7.43748C9.82103 7.43748 9.38828 7.46416 9.0216 7.4942C8.20509 7.56108 7.5611 8.20507 7.49421 9.02159C7.46417 9.38826 7.43749 9.82102 7.43749 10.2083C7.43749 10.5956 7.46417 11.0284 7.49421 11.395C7.5611 12.2116 8.20509 12.8555 9.0216 12.9224C9.38828 12.9525 9.82103 12.9791 10.2083 12.9791C10.5956 12.9791 11.0284 12.9525 11.3951 12.9224C12.2116 12.8555 12.8556 12.2116 12.9224 11.395C12.9525 11.0284 12.9792 10.5956 12.9792 10.2083C12.9792 9.82102 12.9525 9.38826 12.9224 9.02159C12.8556 8.20507 12.2116 7.56108 11.3951 7.4942C11.0284 7.46416 10.5956 7.43748 10.2083 7.43748ZM9.11686 8.65697C9.47041 8.628 9.86718 8.60415 10.2083 8.60415C10.5495 8.60415 10.9462 8.628 11.2998 8.65697C11.5472 8.67723 11.7394 8.86947 11.7597 9.11684C11.7886 9.4704 11.8125 9.86716 11.8125 10.2083C11.8125 10.5495 11.7886 10.9462 11.7597 11.2998C11.7394 11.5472 11.5472 11.7394 11.2998 11.7597C10.9462 11.7886 10.5495 11.8125 10.2083 11.8125C9.86718 11.8125 9.47041 11.7886 9.11686 11.7597C8.86949 11.7394 8.67725 11.5472 8.65698 11.2998C8.62802 10.9462 8.60416 10.5495 8.60416 10.2083C8.60416 9.86716 8.62802 9.4704 8.65698 9.11684C8.67725 8.86947 8.86949 8.67723 9.11686 8.65697Z" fill="currentColor"/></svg>,
  performance: <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false"><path fillRule="evenodd" clipRule="evenodd" d="M7.00001 0.583313C10.5438 0.583313 13.4166 3.45618 13.4167 6.99998C13.4167 8.00866 13.1831 8.96264 12.7684 9.81183C12.756 9.84274 12.7402 9.87242 12.7223 9.90127C11.6628 11.9867 9.49933 13.4166 7.00001 13.4166C3.45619 13.4166 0.58335 10.5438 0.583344 6.99998C0.583382 3.45618 3.45621 0.583313 7.00001 0.583313ZM9.0206 2.29002C9.04494 2.42538 9.084 2.47069 9.10377 2.49111C9.14428 2.53293 9.18529 2.55555 9.33049 2.64321C9.44896 2.71473 9.6748 2.85184 9.82097 3.11489C9.97222 3.38727 9.98979 3.69775 9.93889 4.0235C9.88439 4.37206 9.74642 4.68372 9.4655 4.8928C9.20852 5.08401 8.91515 5.12124 8.73463 5.13833C8.29823 5.1796 8.14648 5.16462 7.97413 5.31435C7.96651 5.32098 7.93264 5.34581 7.92571 5.49721C7.91786 5.67022 7.94838 5.86375 7.98382 6.1603C8.0146 6.41799 8.05021 6.76738 7.97812 7.10309C7.8977 7.47712 7.68555 7.82811 7.27003 8.06012C6.84645 8.29656 6.41949 8.28142 6.05665 8.17405C5.71532 8.07299 5.40162 7.88201 5.15943 7.72744C4.8865 7.55324 4.70786 7.42949 4.54192 7.35488C4.49567 7.33409 4.46379 7.3242 4.44394 7.31899C4.42956 7.33556 4.4093 7.36655 4.38811 7.4278C4.34683 7.54727 4.32352 7.70986 4.30209 7.93878C4.28325 8.14018 4.26448 8.41953 4.21494 8.66225C4.16549 8.90426 4.065 9.22732 3.80079 9.46832C3.45867 9.78027 3.03601 9.87746 2.58855 9.84544C3.5235 11.292 5.14947 12.25 7.00001 12.25C7.15011 12.25 7.29858 12.2418 7.44549 12.2295C7.24062 11.8144 7.17088 11.3693 7.31788 10.9073C7.46985 10.4301 7.68835 10.1789 7.88583 9.99355C8.0709 9.81986 8.07686 9.82116 8.1006 9.75714C8.10174 9.75054 8.10683 9.71647 8.10344 9.61416C8.10021 9.51668 8.08519 9.31417 8.10857 9.11627C8.13463 8.89603 8.20735 8.65115 8.38201 8.41217C8.55234 8.17923 8.79544 7.9853 9.11004 7.82029C9.66109 7.53135 10.2667 7.59063 10.8458 7.8465C11.2185 8.01117 11.6037 8.26642 12.0016 8.59788C12.1625 8.09399 12.25 7.5572 12.25 6.99998C12.25 4.8071 10.9051 2.92912 8.99553 2.14362C9.00295 2.1895 9.01129 2.23826 9.0206 2.29002ZM10.3747 8.91347C10.0051 8.75019 9.79072 8.78081 9.65178 8.85366C9.44969 8.95969 9.36266 9.04839 9.32423 9.10089C9.2903 9.14736 9.27458 9.19169 9.26726 9.25356C9.25736 9.33766 9.26354 9.41187 9.26897 9.57542C9.27282 9.69125 9.27787 9.87903 9.22226 10.0773L9.19435 10.1627C9.053 10.544 8.78659 10.7477 8.68393 10.8441C8.59349 10.9289 8.50738 11.0177 8.42986 11.261C8.38425 11.4045 8.39037 11.632 8.65829 11.9811C9.85406 11.5832 10.8588 10.7681 11.4986 9.70701C11.0576 9.30626 10.6836 9.05003 10.3747 8.91347ZM7.00001 1.74998C4.10054 1.74998 1.75005 4.10052 1.75001 6.99998C1.75001 7.51978 1.82634 8.02178 1.96705 8.49591C2.69986 8.78977 2.94579 8.66907 3.01466 8.60642C3.00395 8.61625 3.03858 8.5904 3.07162 8.42869C3.10464 8.26692 3.11615 8.0847 3.13998 7.82998C3.16126 7.60268 3.19323 7.31298 3.28525 7.04669C3.38155 6.7681 3.56144 6.46388 3.90732 6.27537L3.98593 6.23663C4.38006 6.06147 4.76592 6.17665 5.01987 6.29075C5.2874 6.41102 5.57528 6.60837 5.7872 6.74363C6.02992 6.89855 6.22029 7.00626 6.38762 7.05581C6.5334 7.09894 6.62029 7.0869 6.70151 7.04156C6.79066 6.99176 6.81933 6.94283 6.83766 6.85756C6.86411 6.73395 6.8567 6.56307 6.82512 6.29873C6.79817 6.0732 6.74661 5.74123 6.76018 5.44366C6.77479 5.12448 6.86587 4.73218 7.20851 4.43422C7.73675 3.97495 8.39149 3.9989 8.62525 3.97679C8.69661 3.97002 8.7367 3.96139 8.75742 3.95628C8.76503 3.93676 8.77731 3.90198 8.78647 3.84349C8.79922 3.76183 8.79812 3.71282 8.79615 3.6874C8.78323 3.67816 8.7622 3.6626 8.72779 3.64183C8.64567 3.59226 8.43814 3.48077 8.2658 3.30288C8.07269 3.10349 7.93435 2.84249 7.87216 2.49681C7.82539 2.23677 7.7938 2.00381 7.78216 1.80809C7.52697 1.76997 7.26581 1.74998 7.00001 1.74998Z" fill="currentColor"/></svg>,
  parameters: <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false"><path d="M4.66668 4.08333C4.34451 4.08333 4.08334 4.3445 4.08334 4.66667C4.08334 4.98883 4.34451 5.25 4.66668 5.25H6.41668C6.73884 5.25 7.00001 4.98883 7.00001 4.66667C7.00001 4.3445 6.73884 4.08333 6.41668 4.08333H4.66668Z" fill="currentColor"/><path d="M4.08334 7C4.08334 6.67783 4.34451 6.41667 4.66668 6.41667H9.33334C9.65551 6.41667 9.91668 6.67783 9.91668 7C9.91668 7.32217 9.65551 7.58333 9.33334 7.58333H4.66668C4.34451 7.58333 4.08334 7.32217 4.08334 7Z" fill="currentColor"/><path d="M4.66668 8.75C4.34451 8.75 4.08334 9.01117 4.08334 9.33333C4.08334 9.6555 4.34451 9.91667 4.66668 9.91667H9.33334C9.65551 9.91667 9.91668 9.6555 9.91668 9.33333C9.91668 9.01117 9.65551 8.75 9.33334 8.75H4.66668Z" fill="currentColor"/><path fillRule="evenodd" clipRule="evenodd" d="M7.00001 0.875C5.39638 0.875 4.2197 0.947073 3.43485 1.02094C2.45135 1.11351 1.69794 1.87097 1.61056 2.85491C1.5346 3.71028 1.45834 5.05358 1.45834 7C1.45834 8.94642 1.5346 10.2897 1.61056 11.1451C1.69794 12.129 2.45134 12.8865 3.43485 12.9791C4.2197 13.0529 5.39638 13.125 7.00001 13.125C8.60364 13.125 9.78032 13.0529 10.5652 12.9791C11.5487 12.8865 12.3021 12.129 12.3895 11.1451C12.4654 10.2897 12.5417 8.94642 12.5417 7C12.5417 6.38613 12.5341 5.83228 12.5213 5.33483C12.5061 4.74228 12.2596 4.18483 11.8468 3.77071L9.6489 1.56544C9.23384 1.149 8.67258 0.899294 8.07494 0.886344C7.7406 0.879099 7.38244 0.875 7.00001 0.875ZM3.54417 2.18248C4.29119 2.11217 5.43135 2.04167 7.00001 2.04167C7.37411 2.04167 7.72383 2.04568 8.04967 2.05274C8.33629 2.05895 8.61326 2.17902 8.82257 2.38903L11.0205 4.59429C11.2283 4.80281 11.3477 5.07851 11.355 5.36476C11.3675 5.85187 11.375 6.39573 11.375 7C11.375 8.91627 11.2999 10.2249 11.2274 11.0419C11.1898 11.4654 10.8791 11.7777 10.4559 11.8175C9.70883 11.8878 8.56867 11.9583 7.00001 11.9583C5.43135 11.9583 4.29119 11.8878 3.54417 11.8175C3.12094 11.7777 2.81026 11.4654 2.77265 11.0419C2.7001 10.2249 2.62501 8.91627 2.62501 7C2.62501 5.08373 2.7001 3.77513 2.77265 2.9581C2.81026 2.53463 3.12094 2.22231 3.54417 2.18248Z" fill="currentColor"/></svg>,
  activity: <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false"><path d="M4.37502 0.583313C4.69719 0.583313 4.95835 0.84448 4.95835 1.16665V12.8333C4.95835 13.1555 4.69719 13.4166 4.37502 13.4166C4.05285 13.4166 3.79169 13.1555 3.79169 12.8333V1.16665C3.79169 0.84448 4.05285 0.583313 4.37502 0.583313Z" fill="currentColor"/><path d="M9.62502 2.62498C9.94719 2.62498 10.2084 2.88615 10.2084 3.20831V10.7916C10.2084 11.1138 9.94719 11.375 9.62502 11.375C9.30285 11.375 9.04169 11.1138 9.04169 10.7916V3.20831C9.04169 2.88615 9.30285 2.62498 9.62502 2.62498Z" fill="currentColor"/><path d="M1.75002 4.37498C2.07219 4.37498 2.33335 4.63615 2.33335 4.95831V9.04165C2.33335 9.36381 2.07219 9.62498 1.75002 9.62498C1.42785 9.62498 1.16669 9.36381 1.16669 9.04165V4.95831C1.16669 4.63615 1.42785 4.37498 1.75002 4.37498Z" fill="currentColor"/><path d="M7.00002 4.37498C7.32219 4.37498 7.58335 4.63615 7.58335 4.95831V9.04165C7.58335 9.36381 7.32219 9.62498 7.00002 9.62498C6.67785 9.62498 6.41669 9.36381 6.41669 9.04165V4.95831C6.41669 4.63615 6.67785 4.37498 7.00002 4.37498Z" fill="currentColor"/><path d="M12.25 4.66665C12.5722 4.66665 12.8334 4.92781 12.8334 5.24998V8.74998C12.8334 9.07215 12.5722 9.33331 12.25 9.33331C11.9279 9.33331 11.6667 9.07215 11.6667 8.74998V5.24998C11.6667 4.92781 11.9279 4.66665 12.25 4.66665Z" fill="currentColor"/></svg>,
  api: <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false"><path d="M8.75002 6.85419C9.07219 6.85419 9.33335 7.11535 9.33335 7.43752V10.9375C9.33334 12.2262 8.28867 13.2709 7.00002 13.2709C5.71137 13.2709 4.6667 12.2262 4.66669 10.9375V10.6459C4.66669 10.3237 4.92785 10.0625 5.25002 10.0625C5.57219 10.0625 5.83335 10.3237 5.83335 10.6459V10.9375C5.83337 11.5818 6.3557 12.1042 7.00002 12.1042C7.64434 12.1042 8.16667 11.5818 8.16669 10.9375V7.43752C8.16669 7.11535 8.42785 6.85419 8.75002 6.85419Z" fill="currentColor"/><path d="M3.35419 4.66669C3.67635 4.66669 3.93752 4.92785 3.93752 5.25002C3.93752 5.57219 3.67635 5.83335 3.35419 5.83335H3.05796C2.41764 5.83335 1.89585 6.35423 1.89585 7.00002C1.89585 7.64581 2.41764 8.16669 3.05796 8.16669H6.56252C6.88469 8.16669 7.14585 8.42785 7.14585 8.75002C7.14585 9.07219 6.88469 9.33335 6.56252 9.33335H3.05796C1.7704 9.33335 0.729187 8.28723 0.729187 7.00002C0.729187 5.71282 1.7704 4.66669 3.05796 4.66669H3.35419Z" fill="currentColor"/><path d="M10.9341 4.66669C12.2235 4.66669 13.2709 5.71033 13.2709 7.00002C13.2709 8.28971 12.2235 9.33335 10.9341 9.33335H10.6459C10.3237 9.33335 10.0625 9.07219 10.0625 8.75002C10.0625 8.42785 10.3237 8.16669 10.6459 8.16669H10.9341C11.5813 8.16669 12.1042 7.64333 12.1042 7.00002C12.1042 6.35672 11.5813 5.83335 10.9341 5.83335H7.43752C7.11535 5.83335 6.85419 5.57219 6.85419 5.25002C6.85419 4.92785 7.11535 4.66669 7.43752 4.66669H10.9341Z" fill="currentColor"/><path d="M7.00002 0.729187C8.28868 0.729187 9.33335 1.77385 9.33335 3.06252V3.35419C9.33335 3.67635 9.07219 3.93752 8.75002 3.93752C8.42785 3.93752 8.16669 3.67635 8.16669 3.35419V3.06252C8.16669 2.41819 7.64435 1.89585 7.00002 1.89585C6.35569 1.89585 5.83335 2.41819 5.83335 3.06252V6.56252C5.83335 6.88469 5.57219 7.14585 5.25002 7.14585C4.92785 7.14585 4.66669 6.88469 4.66669 6.56252V3.06252C4.66669 1.77385 5.71136 0.729187 7.00002 0.729187Z" fill="currentColor"/></svg>,
  faq: <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false"><path d="M8.16667 10.5C8.48877 10.5001 8.75 10.7612 8.75 11.0833C8.74988 11.4053 8.4887 11.6666 8.16667 11.6666H5.83333C5.51124 11.6666 5.25012 11.4054 5.25 11.0833C5.25 10.7611 5.51117 10.5 5.83333 10.5H8.16667Z" fill="currentColor"/><path d="M7.58333 2.33331C7.90543 2.33331 8.16655 2.59458 8.16667 2.91665C8.16667 3.23881 7.9055 3.49998 7.58333 3.49998H6.41667C6.09457 3.4999 5.83333 3.23876 5.83333 2.91665C5.83345 2.59463 6.09464 2.33339 6.41667 2.33331H7.58333Z" fill="currentColor"/><path fillRule="evenodd" clipRule="evenodd" d="M9.91667 0.583313C10.7219 0.58339 11.3749 1.23638 11.375 2.04165V11.9583C11.375 12.7637 10.722 13.4166 9.91667 13.4166H4.08333C3.27792 13.4166 2.625 12.7637 2.625 11.9583V2.04165C2.62512 1.23633 3.27799 0.583313 4.08333 0.583313H9.91667ZM4.08333 1.74998C3.92232 1.74998 3.79178 1.88066 3.79167 2.04165V11.9583C3.79167 12.1194 3.92225 12.25 4.08333 12.25H9.91667C10.0777 12.2499 10.2083 12.1193 10.2083 11.9583V2.04165C10.2082 1.88071 10.0776 1.75006 9.91667 1.74998H4.08333Z" fill="currentColor"/></svg>,
};

function ModelTabIcon({ name }: { name: ModelTabIconName }) {
  return MODEL_TAB_ICONS[name];
}

function VideoPreviewMedia(props: {
  src: string;
  poster?: string;
  fallbackSrc?: string;
  fallbackPoster?: string;
  alt: string;
}) {
  const [currentSrc, setCurrentSrc] = useState(props.src);
  const [hasFailed, setHasFailed] = useState(false);

  if (hasFailed && props.fallbackPoster) {
    return (
      <Image
        src={props.fallbackPoster}
        alt={props.alt}
        fill
        sizes="(min-width: 1024px) 40vw, 100vw"
        className="preview-media object-cover"
        unoptimized
      />
    );
  }

  return (
    <video
      key={currentSrc}
      className="preview-media"
      src={currentSrc}
      // Reviewed profession clips provide a same-source first frame;
      // use it immediately so the preview never flashes a blank panel
      // while the remote video is buffering.
      poster={currentSrc === props.fallbackSrc ? props.fallbackPoster ?? props.poster : props.poster}
      autoPlay
      muted
      loop
      playsInline
      onError={() => {
        if (props.fallbackSrc && currentSrc !== props.fallbackSrc) {
          setCurrentSrc(props.fallbackSrc);
          return;
        }
        setHasFailed(true);
      }}
      aria-label={props.alt}
    />
  );
}

function ModelSectionNav(props: {
  generator?: ModelConfig["generator"];
  showPricing?: boolean;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const { generator, showPricing = false, t } = props;
  const items = useMemo<Array<{ href: string; label: string; icon: ModelTabIconName }>>(
    () => [
      ...(generator ? [{ href: "#workbench", label: t("Playground"), icon: "playground" as const }] : []),
      { href: "#performance", label: t("Performance"), icon: "performance" },
      { href: "#activity", label: t("Activity"), icon: "activity" },
      ...(showPricing ? [{ href: "#pricing", label: t("Pricing"), icon: "api" as const }] : []),
      { href: "#api", label: t("API"), icon: "api" },
      { href: "#faq", label: t("FAQ"), icon: "faq" },
    ],
    [generator, showPricing, t]
  );
  const [activeHref, setActiveHref] = useState(() => {
    const hash = typeof window !== "undefined" ? window.location.hash : "";
    const defaultHref = generator ? "#workbench" : "#performance";
    return items.some((item) => item.href === hash) ? hash : defaultHref;
  });
  useEffect(() => {
    const updateHeaderOffset = () => {
      const header = document.querySelector<HTMLElement>(".fk-site-header");
      if (!header) return;
      const height = Math.max(0, Math.round(header.getBoundingClientRect().height));
      document.documentElement.style.setProperty("--model-site-header-height", `${height}px`);
    };
    const header = document.querySelector<HTMLElement>(".fk-site-header");
    const headerResizeObserver = header && typeof ResizeObserver !== "undefined"
      ? new ResizeObserver(updateHeaderOffset)
      : null;
    if (headerResizeObserver && header) headerResizeObserver.observe(header);
    updateHeaderOffset();

    const syncActiveDom = (href: string) => {
      document.querySelectorAll<HTMLAnchorElement>(".model-anchor-link").forEach((link) => {
        const active = link.getAttribute("href") === href;
        link.classList.toggle("active", active);
        if (active) link.setAttribute("aria-current", "page");
        else link.removeAttribute("aria-current");
      });
    };
    const updateActiveSection = () => {
      const isCompact = window.matchMedia("(max-width: 900px)").matches;
      const fallbackHeaderHeight = isCompact ? 132 : 126;
      const siteHeaderHeight = document.querySelector<HTMLElement>(".fk-site-header")?.getBoundingClientRect().height ?? fallbackHeaderHeight;
      const marker = window.scrollY + siteHeaderHeight + 48;
      const sections = items
        .map((item) => {
          const element = document.querySelector<HTMLElement>(item.href);
          if (!element) return null;
          const rect = element.getBoundingClientRect();
          return {
            href: item.href,
            top: rect.top + window.scrollY,
            bottom: rect.bottom + window.scrollY,
          };
        })
        .filter((section): section is { href: string; top: number; bottom: number } => section !== null)
        .sort((left, right) => left.top - right.top);
      let nextHref = sections[0]?.href ?? items[0]?.href ?? "";
      const hash = window.location.hash;
      const hashedSection = sections.find((section) => section.href === hash);
      // A hash is a navigation hint, not a permanent active-tab override. It
      // should win while its target is entering or sitting near the sticky
      // rail, but the visible section must take over again after the user
      // scrolls back to the top. This prevents a stale #api hash from leaving
      // API selected while the hero/workbench is on screen.
      const hashIsNearMarker = hashedSection
        ? hashedSection.top <= marker + 360 && hashedSection.bottom >= marker - 360
        : false;
      if (hashedSection && hashIsNearMarker) {
        nextHref = hashedSection.href;
      } else {
        for (const section of sections) {
          if (section.top <= marker) nextHref = section.href;
        }
      }
      setActiveHref((current) => (current === nextHref ? current : nextHref));
      syncActiveDom(nextHref);
    };

    updateActiveSection();
    window.addEventListener("scroll", updateActiveSection, { passive: true });
    document.addEventListener("scroll", updateActiveSection, { passive: true, capture: true });
    window.addEventListener("resize", updateActiveSection);
    window.addEventListener("resize", updateHeaderOffset);
    const updateHash = () => updateActiveSection();
    window.addEventListener("hashchange", updateHash);
    // Hash navigation can complete after the first effect pass (especially
    // when the page is restored from a deep link). Recalculate from the
    // target's actual position instead of forcing the hash tab unconditionally.
    updateHash();
    const activeSectionTimer = window.setInterval(updateActiveSection, 250);
    return () => {
      window.removeEventListener("scroll", updateActiveSection);
      document.removeEventListener("scroll", updateActiveSection, { capture: true });
      window.removeEventListener("resize", updateActiveSection);
      window.removeEventListener("resize", updateHeaderOffset);
      window.removeEventListener("hashchange", updateHash);
      window.clearInterval(activeSectionTimer);
      headerResizeObserver?.disconnect();
    };
  }, [items]);

  return (
    <nav aria-label={t("Model sections")} className="model-anchor-bar">
      <div className="model-container model-anchor-inner">
        <div className="model-anchor-links">
          {items.map(({ href, label, icon }) => (
            <a
              key={href}
              href={href}
              onClick={() => {
                setActiveHref(href);
                document.querySelectorAll<HTMLAnchorElement>(".model-anchor-link").forEach((link) => {
                  const active = link.getAttribute("href") === href;
                  link.classList.toggle("active", active);
                  if (active) link.setAttribute("aria-current", "page");
                  else link.removeAttribute("aria-current");
                });
              }}
              className={`model-anchor-link ${activeHref === href ? "active" : ""}`}
              aria-current={activeHref === href ? "page" : undefined}
            >
              <span className="model-tab-icon" aria-hidden="true"><ModelTabIcon name={icon} /></span>
              {label}
            </a>
          ))}
        </div>
      </div>
    </nav>
  );
}

const MODEL_CAPABILITY_ICON_STYLE = { width: 36, height: 36, display: "block", flex: "none" } as const;

function ModelCapabilityIcon({ index }: { index: number }) {
  switch (index) {
    case 0:
      return (
        <svg width="36" height="36" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" style={MODEL_CAPABILITY_ICON_STYLE}>
          <rect width="36" height="36" rx="8" fill="white" fillOpacity="0.4" />
          <path d="M20.8451 9.07755C21.1705 8.75213 21.698 8.75216 22.0234 9.07755L23.791 10.8451C24.1165 11.1706 24.1164 11.6981 23.791 12.0235L22.0234 13.7919C21.698 14.1169 21.1704 14.117 20.8451 13.7919C20.5197 13.4665 20.5198 12.9381 20.8451 12.6127L21.291 12.1667H11.75C11.5199 12.1667 11.3333 12.3533 11.3333 12.5834V24.2501C11.3334 24.4801 11.5199 24.6667 11.75 24.6667H24.25C24.4801 24.6667 24.6666 24.4802 24.6667 24.2501V15.5001C24.6667 15.0398 25.0398 14.6667 25.5 14.6667C25.9602 14.6667 26.3333 15.0398 26.3333 15.5001V24.2501C26.3333 25.4006 25.4006 26.3334 24.25 26.3334H11.75C10.5995 26.3334 9.66674 25.4006 9.66667 24.2501V12.5834C9.66667 11.4328 10.5994 10.5001 11.75 10.5001H21.0892L20.8451 10.2559C20.5197 9.93055 20.5198 9.40298 20.8451 9.07755Z" fill="#171A21" />
          <path d="M18.8333 21.3334C19.2936 21.3334 19.6667 21.7065 19.6667 22.1667C19.6666 22.6269 19.2935 23.0001 18.8333 23.0001H14.6667C14.2065 23.0001 13.8334 22.6269 13.8333 22.1667C13.8333 21.7065 14.2064 21.3334 14.6667 21.3334H18.8333Z" fill="#171A21" />
          <path d="M21.3333 18.0001C21.7936 18.0001 22.1667 18.3732 22.1667 18.8334C22.1666 19.2936 21.7935 19.6667 21.3333 19.6667H14.6667C14.2065 19.6667 13.8334 19.2936 13.8333 18.8334C13.8333 18.3732 14.2064 18.0001 14.6667 18.0001H21.3333Z" fill="#171A21" />
          <path d="M18.8333 14.6667C19.2936 14.6667 19.6667 15.0398 19.6667 15.5001C19.6666 15.9603 19.2935 16.3334 18.8333 16.3334H14.6667C14.2065 16.3334 13.8334 15.9603 13.8333 15.5001C13.8333 15.0398 14.2064 14.6667 14.6667 14.6667H18.8333Z" fill="#171A21" />
        </svg>
      );
    case 1:
      return (
        <svg width="36" height="36" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" style={MODEL_CAPABILITY_ICON_STYLE}>
          <rect width="36" height="36" rx="8" fill="white" fillOpacity="0.4" />
          <path d="M10.5 20.5001C10.9603 20.5001 11.3334 20.8732 11.3334 21.3334V24.6667H14.6667C15.1269 24.6667 15.5 25.0398 15.5 25.5001C15.5 25.9603 15.1269 26.3334 14.6667 26.3334H11.3334C10.4129 26.3334 9.66669 25.5872 9.66669 24.6667V21.3334C9.66669 20.8732 10.0398 20.5001 10.5 20.5001Z" fill="#171A21" />
          <path d="M25.5 20.5001C25.9603 20.5001 26.3334 20.8732 26.3334 21.3334V24.6667C26.3334 25.5872 25.5872 26.3334 24.6667 26.3334H21.3334C20.8731 26.3334 20.5 25.9603 20.5 25.5001C20.5 25.0398 20.8731 24.6667 21.3334 24.6667H24.6667V21.3334C24.6667 20.8732 25.0398 20.5001 25.5 20.5001Z" fill="#171A21" />
          <path d="M18 16.7501C18.6904 16.7501 19.25 17.3097 19.25 18.0001C19.25 18.6904 18.6904 19.2501 18 19.2501C17.3097 19.2501 16.75 18.6904 16.75 18.0001C16.75 17.3097 17.3097 16.7501 18 16.7501Z" fill="#171A21" />
          <path fillRule="evenodd" clipRule="evenodd" d="M18 13.0001C20.7614 13.0001 23 15.2387 23 18.0001C23 20.7615 20.7614 23.0001 18 23.0001C15.2386 23.0001 13 20.7615 13 18.0001C13 15.2387 15.2386 13.0001 18 13.0001ZM18 14.6667C16.1591 14.6667 14.6667 16.1591 14.6667 18.0001C14.6667 19.841 16.1591 21.3334 18 21.3334C19.841 21.3334 21.3334 19.841 21.3334 18.0001C21.3334 16.1591 19.841 14.6667 18 14.6667Z" fill="#171A21" />
          <path d="M14.6667 9.66675C15.1269 9.66675 15.5 10.0398 15.5 10.5001C15.5 10.9603 15.1269 11.3334 14.6667 11.3334H11.3334V14.6667C11.3334 15.127 10.9603 15.5001 10.5 15.5001C10.0398 15.5001 9.66669 15.127 9.66669 14.6667V11.3334C9.66669 10.4129 10.4129 9.66675 11.3334 9.66675H14.6667Z" fill="#171A21" />
          <path d="M24.6667 9.66675C25.5872 9.66675 26.3334 10.4129 26.3334 11.3334V14.6667C26.3334 15.127 25.9603 15.5001 25.5 15.5001C25.0398 15.5001 24.6667 15.127 24.6667 14.6667V11.3334H21.3334C20.8731 11.3334 20.5 10.9603 20.5 10.5001C20.5 10.0398 20.8731 9.66675 21.3334 9.66675H24.6667Z" fill="#171A21" />
        </svg>
      );
    case 2:
      return (
        <svg width="36" height="36" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" style={MODEL_CAPABILITY_ICON_STYLE}>
          <rect width="36" height="36" rx="8" fill="white" fillOpacity="0.4" />
          <path d="M14.25 8.83325C14.7103 8.83325 15.0834 9.20635 15.0834 9.66659V26.3333C15.0834 26.7935 14.7103 27.1666 14.25 27.1666C13.7898 27.1666 13.4167 26.7935 13.4167 26.3333V9.66659C13.4167 9.20635 13.7898 8.83325 14.25 8.83325Z" fill="#171A21" />
          <path d="M21.75 11.7499C22.2103 11.7499 22.5834 12.123 22.5834 12.5833V23.4166C22.5834 23.8768 22.2103 24.2499 21.75 24.2499C21.2898 24.2499 20.9167 23.8768 20.9167 23.4166V12.5833C20.9167 12.123 21.2898 11.7499 21.75 11.7499Z" fill="#171A21" />
          <path d="M10.5 14.2499C10.9603 14.2499 11.3334 14.623 11.3334 15.0833V20.9166C11.3334 21.3768 10.9603 21.7499 10.5 21.7499C10.0398 21.7499 9.66669 21.3768 9.66669 20.9166V15.0833C9.66669 14.623 10.0398 14.2499 10.5 14.2499Z" fill="#171A21" />
          <path d="M18 14.2499C18.4603 14.2499 18.8334 14.623 18.8334 15.0833V20.9166C18.8334 21.3768 18.4603 21.7499 18 21.7499C17.5398 21.7499 17.1667 21.3768 17.1667 20.9166V15.0833C17.1667 14.623 17.5398 14.2499 18 14.2499Z" fill="#171A21" />
          <path d="M25.5 14.6666C25.9603 14.6666 26.3334 15.0397 26.3334 15.4999V20.4999C26.3334 20.9602 25.9603 21.3333 25.5 21.3333C25.0398 21.3333 24.6667 20.9602 24.6667 20.4999V15.4999C24.6667 15.0397 25.0398 14.6666 25.5 14.6666Z" fill="#171A21" />
        </svg>
      );
    case 3:
      return (
        <svg width="36" height="36" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" style={MODEL_CAPABILITY_ICON_STYLE}>
          <rect width="36" height="36" rx="8" fill="white" fillOpacity="0.4" />
          <path d="M13 20.6726C13 20.3014 13.4487 20.1155 13.7112 20.378L15.622 22.2888C15.8845 22.5513 15.6986 23 15.3274 23.0001H13.4166C13.1865 23.0001 13 22.8135 13 22.5834V20.6726Z" fill="#171A21" />
          <path d="M22.2887 20.378C22.5512 20.1155 22.9999 20.3014 23 20.6726V22.5834C22.9999 22.8135 22.8134 23.0001 22.5833 23.0001H20.6725C20.3013 23 20.1155 22.5513 20.3779 22.2888L22.2887 20.378Z" fill="#171A21" />
          <path d="M15.3274 13.0001C15.6986 13.0001 15.8845 13.4489 15.622 13.7113L13.7112 15.6222C13.4487 15.8846 13 15.6987 13 15.3276V13.4167C13 13.1866 13.1865 13.0001 13.4166 13.0001H15.3274Z" fill="#171A21" />
          <path d="M22.5833 13.0001C22.8134 13.0001 23 13.1866 23 13.4167V15.3276C22.9999 15.6987 22.5512 15.8846 22.2887 15.6222L20.3779 13.7113C20.1154 13.4489 20.3013 13.0001 20.6725 13.0001H22.5833Z" fill="#171A21" />
          <path fillRule="evenodd" clipRule="evenodd" d="M24.25 9.66675C25.4006 9.66675 26.3333 10.5995 26.3333 11.7501V24.2501C26.3333 25.4007 25.4006 26.3334 24.25 26.3334H11.75C10.5994 26.3334 9.66663 25.4007 9.66663 24.2501V11.7501C9.66663 10.5995 10.5994 9.66675 11.75 9.66675H24.25ZM11.75 11.3334C11.5198 11.3334 11.3333 11.52 11.3333 11.7501V24.2501C11.3333 24.4802 11.5198 24.6667 11.75 24.6667H24.25C24.4801 24.6667 24.6666 24.4802 24.6666 24.2501V11.7501C24.6666 11.52 24.4801 11.3334 24.25 11.3334H11.75Z" fill="#171A21" />
        </svg>
      );
    default:
      return null;
  }
}

type CapabilityCard = {
  title: string;
  body: string;
  Icon: typeof Code2;
};

function ModelCapabilitiesSection(props: {
  config: ModelConfig;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const customCards = props.config.landingContent?.capabilities;
  const cards = customCards
    ? customCards.map(({ title, body }) => ({ title, body, Icon: Code2 }))
    : buildCapabilityCards(props.config);
  const isSeedance25 = props.config.slug === "seedance-2.5";
  return (
    <section id="capabilities" className="model-section capabilities">
      <div className="model-container">
        <FlatkeySectionHeading
          // The model-specific H2 carries the search phrase; do not render a
          // generic “Capabilities” eyebrow above it.
          eyebrow={undefined}
          title={props.t(props.config.landingContent?.capabilitiesTitle ?? (isSeedance25 ? "What seedance-2.5 can do" : "Core capabilities and practical engineering value"))}
          description={props.t(props.config.landingContent?.capabilitiesDescription ?? (isSeedance25
            ? "Its capabilities, and what changed from Seedance 2.0 — so you can tell whether it is worth switching."
            : props.config.positioning))}
        />
        <div className="capability-grid">
          {cards.map(({ title, body }, index) => (
            <article key={title} className="capability-card">
              <div className="card-icon" style={{ padding: 0, background: "transparent" }} aria-hidden="true">
                <ModelCapabilityIcon index={index} />
              </div>
              <div className="capability-copy">
                <h3>{props.t(title)}</h3>
                <p>{props.t(body)}</p>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function buildCapabilityCards(config: ModelConfig): CapabilityCard[] {
  if (config.generator?.kind === "image") {
    return [
      { title: "Product visuals", body: "Generate product-style shots, merchandising scenes, and reference-guided variations.", Icon: ImageIcon },
      { title: "Ads and social creative", body: "Produce campaign concepts, thumbnails, posters, and localized variants.", Icon: Sparkles },
      { title: "Image editing", body: "Refine existing images, swap details, and create consistent variations for each channel.", Icon: WandSparkles },
      { title: "Developer pipelines", body: "Add async generation for agents, CMS tools, and batch creative systems.", Icon: Layers3 },
    ];
  }
  if (config.generator?.kind === "video") {
    return [
      { title: "Text to Video", body: "Create product shots, social clips, and story scenes with controlled motion and framing.", Icon: ImageIcon },
      { title: "Reference-guided motion", body: "Use image, video, and audio references to keep subjects and creative direction consistent.", Icon: Sparkles },
      { title: "Camera and pacing", body: "Configure ratio, resolution, duration, and sound before handing the request to the API.", Icon: Layers3 },
      { title: "Production routing", body: "Keep usage, keys, quotas, and model routing in one Flatkey account.", Icon: ShieldCheck },
    ];
  }
  if (config.generator?.kind === "audio") {
    return [
      { title: "Audio generation", body: "Create music, sound beds, and polished audio tracks from a media-aware brief.", Icon: ImageIcon },
      { title: "Speech preservation", body: "Keep important dialogue and speech intelligible while adding a new audio layer.", Icon: Sparkles },
      { title: "Audio variants", body: "Generate multiple delivery options for edits, localization, and production handoff.", Icon: Layers3 },
      { title: "Production routing", body: "Keep usage, keys, quotas, and model routing in one Flatkey account.", Icon: ShieldCheck },
    ];
  }
  return [
    { title: "OpenAI-compatible migration path", body: "Chat Completions-style payloads reduce switching friction from existing model stacks.", Icon: Code2 },
    { title: "Structured and tool-based output", body: "Use structured JSON, tools, and code-generation flows for agentic workflows.", Icon: Layers3 },
    { title: "Streaming interaction", body: "Streaming supports chat UIs, terminal assistants, and progressive rendering.", Icon: Zap },
    { title: "Production routing", body: "Keep usage, keys, quotas, and model routing in one Flatkey account.", Icon: ShieldCheck },
  ];
}

function ModelComparisonSection(props: {
  config: ModelConfig;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const content = props.config.landingContent?.comparison ?? buildPreviousGenerationComparison(props.config, props.t);
  return (
    <section id="comparison" className="model-section capabilities-comparison">
      <div className="model-container">
        <div className="comparison-block">
          <FlatkeySectionHeading
            // This block renders a side-by-side table. Keep its semantic
            // label as Compare even when older model configs called the
            // eyebrow “Features”, “Capabilities”, or “Hosted API facts”.
            // The model-specific title and rows carry the verified fields;
            // the shared label should never suggest that a generic feature
            // card is a comparison.
            eyebrow={props.t("Compare")}
            title={props.t(content.title)}
            description={props.t(content.description)}
          />
          <div className="comparison">
            <table>
              <thead>
                <tr>
                  <th>{props.t("Capability")}</th>
                  <th>{props.t(content.baselineLabel)}</th>
                  <th>{props.t(content.currentLabel)}</th>
                </tr>
              </thead>
              <tbody>
                {content.rows.map((row) => (
                  <tr key={row.label}>
                    <td>{props.t(row.label)}</td>
                    <td>{props.t(row.baseline)}</td>
                    <td><span className="check">{props.t(row.current)}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </section>
  );
}

const PREVIOUS_GENERATION_BY_SLUG: Record<string, string> = {
  "claude-api": "Claude 3.7 Sonnet",
  "gpt-api": "GPT-4.1",
  "gemini-api": "Gemini 2.0 Pro",
  "qwen-api": "Qwen 3.5",
  "glm-api": "GLM-4.7",
  "seedance-api": "Seedance 1.0",
  "gpt-image-2": "GPT-image-1",
  "minimax-h3": "MiniMax-H2",
  "gpt-4.1-mini": "GPT-4o mini",
  "sonilo-video-to-music": "sonilo-video-to-music-v1",
};

function buildPreviousGenerationComparison(
  config: ModelConfig,
  t: (key: string, vars?: Record<string, string>) => string
): NonNullable<NonNullable<ModelConfig["landingContent"]>["comparison"]> {
  const previous = PREVIOUS_GENERATION_BY_SLUG[config.slug] ?? config.modelIds.find((id) => id !== config.modelId) ?? t("Previous generation");
  const modality = config.generator?.kind === "image"
    ? "Text to Image"
    : config.generator?.kind === "video"
      ? "Text to Video"
      : config.generator?.kind === "audio"
        ? config.generator.endpoint.includes("video-to-music") ? "Video to Audio" : "Text to Audio"
        : "Chat and coding";
  const context = config.generator ? "Reference media" : "Long context";
  return {
    eyebrow: "Compare",
    title: "What changed from the previous generation, so you can tell whether it is worth switching.",
    description: "Compare the current model with the previous generation before you migrate.",
    baselineLabel: previous,
    currentLabel: config.modelId,
    rows: [
      { label: "Model ID", baseline: previous, current: config.modelId },
      { label: "Modalities", baseline: modality, current: modality },
      { label: "Context", baseline: context, current: context },
      { label: "API", baseline: "OpenAI-compatible migration path", current: "OpenAI-compatible migration path" },
      { label: "Best for", baseline: config.positioning, current: config.positioning },
    ],
  };
}

function ModelWhySection(props: {
  config: ModelConfig;
  allowPlayground?: boolean;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const allowPlayground = props.allowPlayground ?? Boolean(props.config.generator && props.config.generator.kind !== "audio");
  const configuredCustom = props.config.landingContent?.why;
  const custom = configuredCustom && (allowPlayground || !/playground/i.test(JSON.stringify(configuredCustom)))
    ? configuredCustom
    : undefined;
  const customCards = custom?.cards.filter((card) =>
    allowPlayground || !/playground/i.test(`${card.title} ${card.body}`)
  );
  const cards = customCards && customCards.length > 0
    ? customCards.map((card) => ({ ...card, Icon: ModelWhyIcon }))
    : (props.config.generator?.kind === "image" || props.config.generator?.kind === "video"
    ? [
        { title: "Lower generation pricing", body: "Route media workloads through Flatkey and keep prompt tests cheaper before scaling.", Icon: Zap },
        { title: "Draft handoff", body: "The public page stores prompt settings locally before sending the user into Flatkey.", Icon: Sparkles },
        { title: "Unified API access", body: "Use one account and API key across image, video, audio, and language models.", Icon: Code2 },
        { title: "Routing across upstream channels", body: "Requests are spread across the channels serving this model, with 30-day uptime published above.", Icon: ShieldCheck },
      ]
    : [
        { title: "OpenAI-compatible migration path", body: "Chat Completions-style payloads reduce switching friction from existing model stacks.", Icon: Code2 },
        { title: "Production routing", body: "Keep usage, keys, quotas, and model routing in one Flatkey account.", Icon: ShieldCheck },
        { title: "Unified API access", body: "Use one account and API key across image, video, audio, and language models.", Icon: Layers3 },
        { title: "Routing across upstream channels", body: "Requests are spread across the channels serving this model, with 30-day uptime published above.", Icon: ShieldCheck },
      ]);
  return (
    <section id="why" className="model-section why">
      <div className="model-container">
        <FlatkeySectionHeading
          eyebrow={props.t(custom?.eyebrow ?? "Why Flatkey")}
          title={props.t(custom?.title ?? "Why use Flatkey for {{model}}?", { model: props.config.displayName })}
          description={custom ? props.t(custom.description) : undefined}
        />
        <div className="why-grid">
          {cards.map(({ title, body, Icon }) => (
            <article key={title} className="why-card">
              <div className="card-icon"><Icon aria-hidden /></div>
              <h3>{props.t(title)}</h3>
              <p>{props.t(body)}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function ModelWhyIcon(props: { "aria-hidden"?: boolean }) {
  return (
    <svg viewBox="0 0 20 20" focusable="false" aria-hidden={props["aria-hidden"]}>
      <path d="M3.5 2.5h7.5l5.5 5.5v9.5h-13zM11 2.8V8h5.2M6 11h4.5M6 14h3M11.5 11.5h4m0 0-1.8-1.8M15.5 11.5l-1.8 1.8" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" />
    </svg>
  );
}

type PromptLibraryItem = {
  key: string;
  label: string;
  prompt: string;
  example: MediaExample;
  alt?: string;
};

/**
 * Local fallbacks stay paired with the workflow that owns the card. Do not
 * rotate these by card index: the six profession sets have different orders
 * on image and video pages, and index-based fallbacks make a food card show a
 * shoe or a game card show an unrelated map when a remote object fails.
 */
const PROMPT_POSTER_FALLBACKS: Record<string, string> = {
  // Image profession directions.
  "product-hero": "/assets/prompts/awesome-images/saas-hero-phone.png",
  "social-ad": "/assets/prompts/awesome-images/sports-shoe.png",
  "catalog-variant": "/assets/prompts/awesome-images/ecommerce-skincare.png",
  "editorial-portrait": "/assets/prompts/awesome-images/gpt-image-2-showcase-complex.png",
  "product-ui": "/assets/prompts/awesome-images/liquid-bento.png",
  "food-editorial": "/assets/cli/campaign-hero.png",
  // Video workflow templates.
  "product-launch": "/assets/model-examples/product-macro.png",
  "food-beverage-loop": "/assets/model-examples/food-motion.png",
  "hospitality-walkthrough": "/assets/model-examples/image2/flatkey-image2-hotel.png",
  "mobility-launch": "/assets/model-examples/seedance-f1-wet-track.png",
  "saas-product-demo": "/assets/prompts/awesome-images/saas-hero-phone.png",
  "architecture-reveal": "/assets/model-showcase/coastal-landmark.png",
};

function getPromptPosterFallback(item: PromptLibraryItem): string {
  return PROMPT_POSTER_FALLBACKS[item.key] ?? (item.example.poster || "/assets/prompts/awesome-images/playground-starter-product.png");
}

/**
 * Load a prompt-library clip when it is close to the viewport. The first card
 * is eager, while the remaining cards start the exact CDN asset as soon as
 * they become visible. This keeps the page responsive without substituting an
 * older poster for a reviewed generated clip.
 */
function PromptLibraryVideo(props: {
  src: string;
  fallbackSrc?: string;
  poster?: string;
  fallbackPoster?: string;
  priority: boolean;
  className: string;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [isVisible, setIsVisible] = useState(props.priority);
  const [currentSrc, setCurrentSrc] = useState(props.src);
  const [hasFailed, setHasFailed] = useState(false);

  useEffect(() => {
    if (props.priority || isVisible) return;
    const video = videoRef.current;
    if (!video || typeof IntersectionObserver === "undefined") {
      setIsVisible(true);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setIsVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "240px 0px" },
    );
    observer.observe(video);
    return () => observer.disconnect();
  }, [isVisible, props.priority]);

  useEffect(() => {
    if (!isVisible) return;
    const playPromise = videoRef.current?.play();
    playPromise?.catch(() => undefined);
  }, [isVisible]);

  if (hasFailed && props.fallbackPoster) {
    return (
      <Image
        src={props.fallbackPoster}
        alt=""
        fill
        sizes="(min-width: 1024px) 33vw, 100vw"
        className={props.className}
        unoptimized
      />
    );
  }

  return (
    <video
      ref={videoRef}
      key={currentSrc}
      className={props.className}
      src={currentSrc}
      poster={currentSrc === props.fallbackSrc ? props.fallbackPoster || props.poster : props.poster || props.fallbackPoster}
      muted
      loop
      autoPlay={isVisible}
      playsInline
      preload={isVisible ? "auto" : "none"}
      onError={() => {
        if (props.fallbackSrc && currentSrc !== props.fallbackSrc) {
          setCurrentSrc(props.fallbackSrc);
          return;
        }
        setHasFailed(true);
      }}
    />
  );
}

function PromptLibrarySection(props: {
  config: ModelConfig;
  locale: Locale;
  examples: readonly MediaExample[];
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const [failedPosters, setFailedPosters] = useState<Record<string, boolean>>({});
  const kind = props.config.generator?.kind;
  if (kind !== "image" && kind !== "video") return null;
  const items = buildPromptLibraryItems(props.config, props.examples, props.t, props.locale);
  const isPrototypeContent = Boolean(props.config.landingContent?.promptLibrary);

  return (
    <section id="prompt-library" className="model-section prompt-library">
      <div className="model-container">
        <div className="prompt-library-head">
          <FlatkeySectionHeading
            eyebrow={props.t("Prompt library")}
            title={props.t(props.config.landingContent?.promptLibraryTitle ?? "Explore what {{model}} can create", { model: props.config.displayName })}
            description={props.t(props.config.landingContent?.promptLibraryDescription ?? "Prompt testing and request handoff.")}
          />
        </div>
        <div className="prompt-grid">
          {items.map((item, index) => {
            const isPriorityMedia = index === 0;
            // Generated profession clips are the canonical asset for video
            // cards. Do not paint an older local poster behind them while the
            // CDN clip is loading; the same clip is also used by the
            // playground preview below.
            const usesGeneratedVideo = isProfessionVideo(item.example.video);
            const posterFallback = item.example.fallbackPoster ?? (usesGeneratedVideo ? "" : getPromptPosterFallback(item));
            const posterSource = failedPosters[item.example.poster]
              ? posterFallback
              : item.example.poster;
            return (
            <article key={item.key} className="prompt-card">
              <div
                className="prompt-media"
                style={{
                  ...(posterFallback ? { backgroundImage: `url("${posterFallback}")` } : {}),
                  backgroundPosition: "center",
                  backgroundSize: "cover",
                }}
              >
                {item.example.video ? (
                  <PromptLibraryVideo
                    key={item.example.video}
                    className="prompt-image h-full w-full object-cover"
                    src={item.example.video}
                    fallbackSrc={item.example.fallbackVideo}
                    poster={item.example.poster || posterFallback || undefined}
                    fallbackPoster={posterFallback || undefined}
                    priority={isPriorityMedia}
                  />
                ) : (
                  <Image
                    src={posterSource}
                    alt={item.alt ?? item.label}
                    fill
                    sizes="(min-width: 1024px) 33vw, (min-width: 768px) 50vw, 100vw"
                    className="prompt-image object-cover"
                    priority={isPriorityMedia}
                    // Prompt posters are the primary content of this section.
                    // Eager loading prevents visible cards from remaining as
                    // empty placeholders when the browser's lazy threshold
                    // does not account for the tall prompt-card layout.
                    loading="eager"
                    onError={() => setFailedPosters((current) => ({ ...current, [item.example.poster]: true }))}
                    // The reviewed prompt posters live on the public CDN. Bypass
                    // Next's server-side optimizer so a slow/large remote object
                    // cannot leave the card stuck on a broken image placeholder.
                    unoptimized
                  />
                )}
                <div className="prompt-badge">{item.label}</div>
              </div>
              <div className="prompt-body">
                <p className="prompt-text">{item.prompt}</p>
                <div className="prompt-actions">
                  <button
                    type="button"
                    onClick={() => navigator.clipboard?.writeText(item.prompt).catch(() => undefined)}
                    className="outline-button"
                  >
                    {props.t(isPrototypeContent ? "Copy Prompt" : "Copy request")}
                  </button>
                  <a
                    href={consoleUrl("/dashboard/overview")}
                    className="dark-button"
                  >
                    {props.t("Make one like this")}
                  </a>
                </div>
              </div>
            </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function buildPromptLibraryItems(
  config: ModelConfig,
  examples: readonly MediaExample[],
  t: (key: string, vars?: Record<string, string>) => string,
  locale: Locale,
): PromptLibraryItem[] {
  let hasReviewedVideoTemplates = false;
  if (config.generator?.kind === "image") {
    const templates = getImagePromptTemplates(config.modelId, locale);
    const posters = getImagePromptTemplateFallbackPosters(config.modelId);
    if (templates.length > 0) {
      return templates.slice(0, 6).map((template, index) => ({
        key: template.id,
        label: t(template.label),
        prompt: template.prompt,
        alt: t(template.label),
        example: {
          poster: posters[index] ?? template.poster,
          fallbackPoster: getImagePromptTemplateLocalFallbackPoster(config.modelId, template.poster),
        },
      }));
    }
  }

  if (config.generator?.kind === "video") {
    const templates = getVideoPromptTemplates(config.modelId, locale);
    hasReviewedVideoTemplates = templates.length > 0;
    if (templates.length > 0) {
      return templates.slice(0, 6).map((template) => ({
        // The profession is the semantic identity of a card. The legacy
        // template id is retained inside the template for serialized prompts,
        // but must not be used as the media join key.
        key: template.professionId,
        label: t(template.label),
        prompt: template.prompt,
        alt: t(template.label),
        // Keep the generated profession clip as the canonical card asset. If
        // it fails, the component-level fallback is still resolved by the
        // same profession-bound template rather than by card index.
        example: {
          // Keep the generated CDN clip and its same-source poster together.
          // Older reviewed sets may not have a poster; they remain video-only
          // rather than inheriting an unrelated local industry image.
          poster: template.poster || (isProfessionVideo(template.video) ? "" : (PROMPT_POSTER_FALLBACKS[template.id] ?? template.poster)),
          video: template.video,
          fallbackPoster: getVideoPromptTemplateLocalFallbackPoster(config.modelId, template.professionId),
          fallbackVideo: VIDEO_PROMPT_TEMPLATES.find((candidate) => candidate.professionId === template.professionId)?.video,
        },
      }));
    }
  }

  const configured = config.landingContent?.promptLibrary;
  const labels = config.generator?.kind === "video"
    ? ["UGC ad clips", "Product motion", "Social video variants"]
    : ["Product mockups", "Ad creatives", "Ecommerce images"];
  const fallbackItems = labels.flatMap((label) => [0, 1].map((copyIndex) => ({
    key: `${label}-${copyIndex}`,
    label: t(label),
    prompt: `${t(label)} — ${getModelStarterPrompt(config, locale)}`,
    example: examples[(copyIndex + labels.indexOf(label)) % Math.max(1, examples.length)] ?? { poster: "/assets/prompts/awesome-images/ai-agent-poster.png" },
  })));

  if (configured && configured.length > 0) {
    const configuredItems = configured.slice(0, 6).map((item) => ({
      key: item.key,
      label: t(item.label),
      prompt: localizeConfiguredPrompt(item.prompt, config.generator?.kind, locale),
      alt: t(item.alt),
      example: {
        poster: item.poster,
        video: item.video,
        fallbackVideo: item.video && isRemoteMedia(item.video) ? MEDIA_EXAMPLES.video[0]?.video : undefined,
      },
    }));
    // For a model without a reviewed six-profession batch, its configured
    // examples are authoritative. Do not append generic fallback cards that
    // could make a legacy product/UGC/storyboard clip look like a profession
    // mapping or change the model's existing content.
    if (config.generator?.kind === "video" && !hasReviewedVideoTemplates) {
      return configuredItems.slice(0, 6);
    }
    const configuredKeys = new Set(configuredItems.map((item) => item.key));
    const supplementalItems = fallbackItems.filter((item) => !configuredKeys.has(item.key));
    return [...configuredItems, ...supplementalItems].slice(0, 6);
  }
  return fallbackItems.slice(0, 6);
}

const MODEL_USAGE_SAMPLE_POINTS = [0.26, 0.42, 0.33, 0.48, 0.38, 0.52, 0.31, 0.35, 0.59, 0.44, 0.72, 0.49, 0.66, 0.38, 0.55, 0.45, 0.72, 0.53, 0.81, 0.47, 0.62, 0.43, 0.68, 0.54, 0.77, 0.52, 0.69, 0.48, 0.74, 0.59, 0.78, 0.57, 0.68, 0.51, 0.75, 0.6];

function buildSmoothUsagePath(points: readonly number[], width: number, top: number, lineHeight: number) {
  const coords = points.map((value, index) => ({
    x: (index / (points.length - 1)) * width,
    y: top + (1 - value) * lineHeight,
  }));
  if (coords.length === 0) return "";
  let path = `M ${coords[0].x} ${coords[0].y}`;
  for (let index = 0; index < coords.length - 1; index += 1) {
    const p0 = coords[index - 1] ?? coords[index];
    const p1 = coords[index];
    const p2 = coords[index + 1];
    const p3 = coords[index + 2] ?? p2;
    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;
    path += ` C ${cp1x} ${cp1y}, ${cp2x} ${cp2y}, ${p2.x} ${p2.y}`;
  }
  return path;
}

function UsageSampleChart({ t }: { t: (key: string, vars?: Record<string, string>) => string }) {
  const linePath = buildSmoothUsagePath(MODEL_USAGE_SAMPLE_POINTS, 1000, 25, 61);
  const areaPath = `${linePath} L 1000 125 L 0 125 Z`;
  return (
    <svg className="usage-sample-chart" viewBox="0 0 1000 125" preserveAspectRatio="none" aria-label={t("Sample usage trend")} role="img">
      <defs>
        <linearGradient id="model-usage-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#6b38e6" stopOpacity="0.1" />
          <stop offset="1" stopColor="#6b38e6" stopOpacity="0.02" />
        </linearGradient>
      </defs>
      <path d={areaPath} fill="url(#model-usage-fill)" />
      <path d={linePath} fill="none" stroke="#6b38e6" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

function UsageTrendChart({ points, t }: { points: HomeTrendPoint[]; t: (key: string, vars?: Record<string, string>) => string }) {
  const values = points.map((point) => point.success_rate).filter((value) => Number.isFinite(value));
  if (values.length === 0) return null;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min;
  const normalized = values.map((value) => (range > 0 ? (value - min) / range : 0.6));
  const linePath = buildSmoothUsagePath(normalized, 1000, 25, 61);
  const areaPath = `${linePath} L 1000 125 L 0 125 Z`;
  return (
    <svg className="usage-sample-chart" viewBox="0 0 1000 125" preserveAspectRatio="none" aria-label={t("Usage trend")} role="img">
      <defs>
        <linearGradient id="model-live-usage-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#6b38e6" stopOpacity="0.1" />
          <stop offset="1" stopColor="#6b38e6" stopOpacity="0.02" />
        </linearGradient>
      </defs>
      <path d={areaPath} fill="url(#model-live-usage-fill)" />
      <path d={linePath} fill="none" stroke="#6b38e6" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
    </svg>
  );
}

function UsageChartAxis({ points, sample }: { points: HomeTrendPoint[]; sample: boolean }) {
  const formatDate = (ts: number | undefined, fallback: string) => {
    if (!ts) return fallback;
    const date = new Date(ts * 1000);
    return `${date.getMonth() + 1}/${date.getDate()}`;
  };
  return (
    <div className="usage-chart-axis" aria-hidden="true">
      <span>{formatDate(points[0]?.ts, "7/26")}</span>
      <span>{formatDate(sample ? undefined : points.at(-1)?.ts, "8/24")}</span>
    </div>
  );
}

function ModelActivitySection(props: {
  config: ModelConfig;
  trend: HomeTrendPoint[];
  summary?: HomePerfSummary;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const content = props.config.landingContent?.activity;
  const configuredStats = content?.stats;
  const hasLiveData = Boolean(props.summary || props.trend.length > 0);
  const showSampleChart = !hasLiveData && Boolean(content?.sampleChart);
  return (
    <section id="activity" className="model-section usage">
      <div className="model-container">
        <FlatkeySectionHeading
          eyebrow={props.t(content?.eyebrow ?? "Activity")}
          title={props.t(content?.title ?? "Token volume and request traffic for this model over time.")}
          description={props.t(content?.description ?? "Performance uses Flatkey request telemetry from the last 30 days when enough traffic is available.")}
        />
        <div className="chart-card">
          <div className="chart-stats">
            {hasLiveData ? (
              <>
                <FlatkeyMetricCard label={props.t("Requests")} value={formatCallCount(props.summary?.request_count)} note={props.t("30-day window")} icon={<PerformanceMetricIcon name="requests" />} />
                <FlatkeyMetricCard label={props.t("Latency")} value={formatLatencyMs(props.summary?.avg_ttft_ms)} note={props.t("last 30 days")} icon={<PerformanceMetricIcon name="latency" />} />
                <FlatkeyMetricCard label={props.t("Uptime")} value={formatSuccessRate(props.summary?.success_rate)} note={props.t("last 30 days")} icon={<PerformanceMetricIcon name="uptime" />} />
              </>
            ) : configuredStats ? configuredStats.map((stat, index) => (
              <FlatkeyMetricCard
                key={`${stat.label}-${index}`}
                label={props.t(stat.label)}
                value={stat.value}
                valueNode={stat.unit ? <><span>{stat.value}</span><span className="chart-stat-unit">{props.t(stat.unit)}</span></> : undefined}
                note=""
              />
            )) : (
              <>
                <FlatkeyMetricCard label={props.t("Requests")} value={formatCallCount(props.summary?.request_count)} note={props.t("30-day window")} icon={<PerformanceMetricIcon name="requests" />} />
                <FlatkeyMetricCard label={props.t("Latency")} value={formatLatencyMs(props.summary?.avg_ttft_ms)} note={props.t("last 30 days")} icon={<PerformanceMetricIcon name="latency" />} />
                <FlatkeyMetricCard label={props.t("Uptime")} value={formatSuccessRate(props.summary?.success_rate)} note={props.t("last 30 days")} icon={<PerformanceMetricIcon name="uptime" />} />
              </>
            )}
          </div>
          <div className="chart-wrap">
            <div className="chart-grid" aria-hidden="true" />
            <div className="usage-chart-content">
              <div className="mb-4 flex items-center justify-between gap-3 text-sm font-semibold">
                <span>{props.t("Successful inference trend")}</span>
                <span className="text-xs font-medium text-muted-foreground">{props.t("Activity")}</span>
              </div>
              <div className="h-24">
                {props.trend.length > 0 ? (
                  <UsageTrendChart points={props.trend} t={props.t} />
                ) : showSampleChart ? (
                  <UsageSampleChart t={props.t} />
                ) : (
                  <div className="flex h-full items-center justify-center rounded-xl bg-violet-500/5 text-sm text-muted-foreground">{props.t("Not enough data yet")}</div>
                )}
              </div>
              {props.trend.length > 0 || showSampleChart ? <UsageChartAxis points={props.trend} sample={showSampleChart} /> : null}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

function SeedancePricingSection(props: {
  config: ModelConfig;
  rows: FlatkeyPriceTableRow[];
  note: string;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const displayRows = props.rows;
  const featured = displayRows[0];
  const duration = props.config.generator?.fields.find((field) => field.name === "duration");
  const pricingBasis = getPricingBasisLabel(props.config.generator?.kind, props.t);
  const productFacts = [
    { label: "Provider", value: props.config.officialName },
    { label: "API", value: props.config.generator?.endpoint ?? "/v1/videos" },
    { label: "Duration", value: duration ? `${duration.min ?? ""}–${duration.max ?? ""}s` : "4–30s" },
    { label: "Billing basis", value: pricingBasis },
  ];
  return (
    <section id="pricing" className="model-section model-pricing">
      <div className="model-container">
        <div className="pricing-block">
          <FlatkeySectionHeading
            eyebrow={props.t("Pricing")}
            title={`${props.config.displayName} API ${props.t("Pricing")}`}
            description={props.note}
          />
          <div className="pricing-conversion-grid">
            <article className="pricing-feature-card">
              <div className="pricing-feature-head">
                <div>
                  <span className="pricing-kicker">{props.t("Flatkey price")}</span>
                  <h3>{props.t("Price / second")}</h3>
                </div>
                <span className="pricing-live-badge">{props.t("Live catalog model")}</span>
              </div>
              <div className="pricing-feature-value">{featured?.flatkey ?? "—"}</div>
              <PricingFeatureExamples rows={displayRows.map((row) => ({ label: row.label, value: row.flatkey }))} />
              <a href="#workbench" className="pricing-feature-action">{props.t("Try a prompt")} <span aria-hidden="true">↗</span></a>
            </article>
            <PricingWalletCard facts={productFacts} t={props.t} />
          </div>
        </div>
      </div>
    </section>
  );
}

function getPricingBasisLabel(
  kind: ModelGeneratorConfig["kind"] | "text" | undefined,
  t: (key: string, vars?: Record<string, string>) => string,
): string {
  if (kind === "video") return t("/ second").replace(/^\/\s*/, "");
  if (kind === "image") return t("/ image").replace(/^\/\s*/, "");
  return "1M tokens";
}

function PricingFeatureExamples(props: { rows: Array<{ label: string; value: string }> }) {
  if (props.rows.length === 0) return null;
  return (
    <div className="pricing-example-list">
      {props.rows.map((row) => (
        <div className="pricing-example-row" key={`${row.label}-${row.value}`}>
          <span>{row.label}</span>
          <strong>{row.value}</strong>
        </div>
      ))}
    </div>
  );
}

function PricingWalletCard(props: {
  facts: Array<{ label: string; value: string }>;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const walletHref = consoleUrl("/wallet");
  const creditOptions = [10, 20, 50];
  return (
    <div className="pricing-facts-card pricing-wallet-card">
      <div className="pricing-facts-head">
        <div>
          <span className="pricing-kicker">{props.t("Shared balance")}</span>
          <h3>{props.t("Add credits")}</h3>
        </div>
        <a className="pricing-wallet-action" href={walletHref}>
          {props.t("Add credits")} <span aria-hidden="true">↗</span>
        </a>
      </div>
      <p className="pricing-wallet-copy">
        {props.t("Use the same Flatkey balance and API key across image, video, audio, and text models.")}
      </p>
      <div className="pricing-topup-options" aria-label={props.t("Add credits")}>
        {creditOptions.map((amount) => (
          <a
            aria-label={`${props.t("Add credits")}: $${amount}`}
            className="pricing-topup-option"
            href={walletHref}
            key={amount}
          >
            <strong>${amount}</strong>
            <span>{props.t("Open wallet")}</span>
          </a>
        ))}
      </div>
      <div className="pricing-wallet-facts-label">{props.t("Model catalog")}</div>
      <dl className="pricing-facts-list">
        {props.facts.map((fact) => (
          <div key={`${fact.label}-${fact.value}`}>
            <dt>{props.t(fact.label)}</dt>
            <dd>{props.t(fact.value)}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

/**
 * Render an audited, model-specific pricing block inside the shared detail
 * page. `liveRows` always come from the pricing API; optional editorial rows
 * are reserved for dimensions (for example image units or UTC tiers) that
 * the compact comparison card cannot infer safely.
 */
function ModelPricingSection(props: {
  config: ModelConfig;
  content: NonNullable<NonNullable<ModelConfig["landingContent"]>["pricing"]>;
  liveRows: FlatkeyPriceTableRow[];
  liveNote: string;
  allowPlayground?: boolean;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  // Monetary values must always come from the pricing API. Curated editorial
  // rows may describe dimensions, but their amounts can become stale; use the
  // live rows for both the feature card and the breakdown table.
  const rows = props.liveRows.map((row) => ({ label: row.label, value: row.flatkey, detail: row.official }));
  const featured = rows[0];
  const productFacts = buildPricingProductFacts(props.config, props.t);
  const allowPlayground = props.allowPlayground ?? Boolean(props.config.generator && props.config.generator.kind !== "audio");
  const actionHref = allowPlayground ? "#workbench" : "#api";
  const actionLabel = allowPlayground ? props.t("Try a prompt") : props.t("View API");
  const featureExamples = (props.config.generator ? rows : rows.slice(0, 3)).map((row) => ({
    label: props.t(row.label),
    value: props.t(row.value),
  }));
  return (
    <section id="pricing" className="model-section model-pricing">
      <div className="model-container">
        <div className="pricing-block">
          <FlatkeySectionHeading
            eyebrow={props.t(props.content.eyebrow ?? "Pricing")}
            title={props.t(props.content.title)}
            description={props.liveNote}
          />
          <div className="pricing-conversion-grid">
            <article className="pricing-feature-card">
              <div className="pricing-feature-head">
                <div>
                  <span className="pricing-kicker">{props.t("Flatkey price")}</span>
                  <h3>{featured ? props.t(featured.label) : props.t("Pricing")}</h3>
                </div>
                <span className="pricing-live-badge">{props.t("Live catalog model")}</span>
              </div>
              <div className="pricing-feature-value">{featured ? props.t(featured.value) : "—"}</div>
              <PricingFeatureExamples rows={featureExamples} />
              <a href={actionHref} className="pricing-feature-action">{actionLabel} <span aria-hidden="true">↗</span></a>
            </article>
            <PricingWalletCard facts={productFacts} t={props.t} />
          </div>
        </div>
      </div>
    </section>
  );
}

function buildPricingProductFacts(
  config: ModelConfig,
  t: (key: string, vars?: Record<string, string>) => string,
): Array<{ label: string; value: string }> {
  const generator = config.generator;
  if (generator?.kind === "image") {
    const outputCount = generator.fields.find((field) => field.name === "n");
    const size = generator.fields.find((field) => field.name === "size");
    const quality = generator.fields.find((field) => field.name === "quality");
    return [
      { label: "Model Type", value: "Text to Image" },
      { label: "API", value: generator.endpoint },
      { label: "Billing basis", value: getPricingBasisLabel("image", t) },
      { label: "Outputs", value: outputCount ? `${outputCount.min ?? 1}–${outputCount.max ?? 1}` : "1" },
      { label: "Size", value: size?.options?.join(" · ") ?? "auto" },
      ...(quality?.options ? [{ label: "Quality", value: quality.options.join(" · ") }] : []),
    ].slice(0, 4);
  }
  if (generator?.kind === "video") {
    const duration = generator.fields.find((field) => field.name === "duration");
    const ratio = generator.fields.find((field) => field.name === "ratio");
    return [
      { label: "Model Type", value: "Text to Video" },
      { label: "API", value: generator.endpoint },
      { label: "Billing basis", value: getPricingBasisLabel("video", t) },
      { label: "Duration", value: duration ? `${duration.min ?? ""}–${duration.max ?? ""}s` : "—" },
      { label: "Aspect ratio", value: ratio?.options?.join(" · ") ?? "adaptive" },
    ];
  }
  const context = config.rows.find((row) => /context/i.test(row.label) && row.value)?.value;
  const modalities = config.rows.find((row) => /modalit/i.test(row.label) && row.value)?.value;
  return [
    { label: "Model Type", value: "Text" },
    { label: "API", value: generator?.endpoint ?? "/v1/chat/completions" },
    { label: "Billing basis", value: getPricingBasisLabel("text", t) },
    ...(context ? [{ label: "Context", value: context }] : []),
    ...(modalities ? [{ label: "Modalities", value: modalities }] : []),
  ].slice(0, 4);
}

function ModelApiSection(props: {
  config: ModelConfig;
  allowPlayground?: boolean;
  locale: Locale;
  t: (key: string, vars?: Record<string, string>) => string;
}) {
  const allowPlayground = props.allowPlayground ?? Boolean(props.config.generator && props.config.generator.kind !== "audio");
  const configuredCustom = props.config.landingContent?.api;
  const custom = configuredCustom && (allowPlayground || !/playground/i.test(JSON.stringify(configuredCustom)))
    ? configuredCustom
    : undefined;
  const endpoint = props.config.generator?.endpoint ?? "/v1/chat/completions";
  // Keep the request shape and technical identifiers canonical, while using
  // the same locale-aware starter prompt shown in the playground. This avoids
  // an English prompt leaking into otherwise localized API examples.
  const apiPrompt =
    props.locale === "en"
      ? props.config.examplePrompt
      : getModelStarterPrompt(
          props.config,
          props.locale,
          props.config.generator?.kind === "image"
            ? getImagePlaygroundExample(props.config.modelId, props.locale)
            : undefined,
        );
  const request = buildPublicApiRequest(props.config, apiPrompt);
  const requestText = JSON.stringify(request, null, 2);
  const [activeTab, setActiveTab] = useState<"curl" | "node" | "python">("curl");
  const [openItem, setOpenItem] = useState(0);
  const languageSamples = {
    curl: `curl https://router.flatkey.ai${endpoint} \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer sk-..." \\\n  -d '${requestText.replaceAll("'", "\\'")}'`,
    node: `const response = await fetch("https://router.flatkey.ai${endpoint}", {\n  method: "POST",\n  headers: { Authorization: "Bearer " + process.env.FLATKEY_API_KEY },\n  body: JSON.stringify(${requestText}),\n});`,
    python: `import os\nimport requests\n\nresponse = requests.post(\n    "https://router.flatkey.ai${endpoint}",\n    headers={"Authorization": "Bearer " + os.environ["FLATKEY_API_KEY"]},\n    json=${requestText},\n)`,
  };
  const customItems = custom?.items?.filter((item) =>
    allowPlayground || !/playground/i.test(`${item.title} ${item.detail}`)
  );
  const apiItems = customItems && customItems.length > 0 ? customItems : [
    { title: props.t("API"), detail: props.t("Call this model through the same OpenAI-compatible router and API key as the rest of the Flatkey catalog.") },
    { title: props.t("SDK for developers"), detail: props.t("Use your existing SDK and set its base URL to the Flatkey router origin.") },
    { title: props.t("Flatkey CLI"), detail: props.t("Keep prompts and request files in your terminal workflow with the Flatkey CLI.") },
    { title: props.t("Codex & Claude Code"), detail: props.t("Use the same key with your coding agent and route model jobs from a script.") },
  ];
  return (
    <section id="api" className="model-section api-section">
      <div className="model-container">
        <FlatkeySectionHeading
          eyebrow={custom?.eyebrow ? props.t(custom.eyebrow) : undefined}
          title={props.t(custom?.title ?? "API Gateway")}
          description={props.t(custom?.description ?? "Supported endpoint coverage in our pricing API.")}
        />
        <div className="api-layout">
          <div className="api-list">
            {apiItems.map((item, index) => {
              const open = index === openItem;
              return (
                <div className={`api-item ${open ? "open" : ""}`} key={item.title}>
                  <button type="button" className="api-toggle" aria-expanded={open} onClick={() => setOpenItem(open ? -1 : index)}>
                    <span>{custom ? props.t(item.title) : item.title}</span><span aria-hidden="true">{open ? "−" : "＋"}</span>
                  </button>
                  <div className="api-detail">{custom ? props.t(item.detail) : item.detail}</div>
                </div>
              );
            })}
          </div>
          <div className="code-shell">
            <div className="code-tabs" role="tablist" aria-label={props.t("Request preview")}>
              <div className="code-tab-group">
                {(["curl", "node", "python"] as const).map((tab) => (
                  <button key={tab} type="button" role="tab" className={`code-tab ${activeTab === tab ? "active" : ""}`} onClick={() => setActiveTab(tab)} aria-selected={activeTab === tab}>
                    {tab === "curl" ? "cURL" : tab === "node" ? "Node.js" : "Python"}
                  </button>
                ))}
              </div>
              <button type="button" className="code-copy" onClick={() => navigator.clipboard?.writeText(languageSamples[activeTab]).catch(() => undefined)} aria-label={props.t("Copy request")}>
              </button>
            </div>
            <div className="code-panel active">
              <pre>{renderApiSample(languageSamples[activeTab], activeTab)}</pre>
            </div>
          </div>
        </div>
        <a className="api-docs-link" href={localizePath("/docs", props.locale)}><BookOpen aria-hidden="true" />{props.t("Docs")}</a>
      </div>
    </section>
  );
}

type ApiCodeTab = "curl" | "node" | "python";

/**
 * Keep the request preview as real text (so copy/paste remains lossless), but
 * add the small set of token classes used by the approved model-detail HTML.
 * This intentionally avoids innerHTML: model IDs and prompts are runtime data.
 */
function renderApiSample(source: string, tab: ApiCodeTab): ReactNode {
  const lines = source.split("\n");
  const payloadStartLine = tab === "curl" ? lines.findIndex((line) => line.includes("-d '")) : -1;
  const tokenPattern = tab === "curl"
    ? /https:\/\/[^\s"']+|("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')/g
    : /https:\/\/[^\s"']+|("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')|\b(const|await|new|import|from|return|async|fetch|requests|post)\b/g;

  const renderLine = (line: string, lineIndex: number) => {
    const isCurlHeader = tab === "curl" && line.trimStart().startsWith("-H");
    const isCurlPayload = tab === "curl" && payloadStartLine >= 0 && lineIndex >= payloadStartLine;
    const pieces: ReactNode[] = [];
    let cursor = 0;
    let match: RegExpExecArray | null;
    tokenPattern.lastIndex = 0;
    while ((match = tokenPattern.exec(line)) !== null) {
      if (match.index > cursor) {
        const plain = line.slice(cursor, match.index);
        pieces.push(isCurlPayload ? <span className="code-json" key={`${lineIndex}-${cursor}`}>{plain}</span> : plain);
      }
      const token = match[0];
      const tokenClass = token.startsWith("http")
        ? "code-url"
        : isCurlHeader
          ? "code-header"
          : isCurlPayload
            ? "code-json"
            : match[2]
              ? "code-keyword"
              : "code-string";
      pieces.push(<span className={tokenClass} key={`${lineIndex}-${match.index}`}>{token}</span>);
      cursor = match.index + token.length;
    }
    if (cursor < line.length) {
      const tail = line.slice(cursor);
      pieces.push(isCurlPayload ? <span className="code-json" key={`${lineIndex}-${cursor}`}>{tail}</span> : tail);
    }
    return pieces.length > 0 ? pieces : line;
  };

  return lines.map((line, index) => (
    <Fragment key={`line-${index}`}>
      {renderLine(line, index)}
      {index < lines.length - 1 ? "\n" : null}
    </Fragment>
  ));
}

function buildPublicApiRequest(config: ModelConfig, prompt = config.examplePrompt): Record<string, unknown> {
  if (config.generator?.kind === "video") {
    const defaults = Object.fromEntries(config.generator.fields.map((field) => [field.name, field.defaultValue]));
    if (config.generator.protocol === "grok-video") {
      return compactRequest({
        model: config.modelId,
        prompt,
        duration: defaults.duration ?? 5,
      });
    }
    if (config.generator.protocol === "veo-video") {
      return compactRequest({
        model: config.modelId,
        prompt,
        duration: Number(defaults.duration ?? 8),
        size: defaults.size ?? "1280x720",
      });
    }
    if (config.generator.protocol === "minimax-video") {
      return {
        model: config.modelId,
        content: [{ type: "text", text: prompt }],
        resolution: defaults.resolution ?? "768P",
        duration: defaults.duration ?? 6,
        ratio: defaults.ratio ?? "16:9",
        aigc_watermark: defaults.aigc_watermark ?? false,
      };
    }
    return {
      model: config.modelId,
      content: [{ type: "text", text: prompt }],
      ratio: defaults.ratio ?? "adaptive",
      resolution: defaults.resolution ?? "720p",
      duration: defaults.duration ?? 5,
      generate_audio: defaults.generate_audio ?? true,
    };
  }
  if (config.generator?.kind === "image") {
    const defaults = Object.fromEntries(config.generator.fields.map((field) => [field.name, field.defaultValue]));
    if (config.generator.protocol === "gemini-image") {
      const imageConfig = compactRequest({
        aspectRatio: defaults.aspect_ratio,
        imageSize: defaults.image_size,
      });
      return {
        model: config.modelId,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
        generationConfig: {
          responseModalities: ["TEXT", "IMAGE"],
          imageConfig,
        },
      };
    }
    return {
      model: config.modelId,
      prompt,
      ...defaults,
    };
  }
  if (config.generator?.kind === "audio") {
    const defaults = Object.fromEntries(config.generator.fields.map((field) => [field.name, field.defaultValue]));
    return {
      model: config.modelId,
      input: prompt,
      ...defaults,
    };
  }
  return {
    model: config.modelId,
    messages: [{ role: "user", content: prompt }],
  };
}

function FlatkeyHeroMetric(props: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-violet-500/16 bg-white/62 p-4 shadow-[0_18px_48px_-42px_rgba(91,33,182,0.72)] backdrop-blur-sm dark:bg-white/[0.04]">
      <div className="text-muted-foreground text-[11px] font-bold tracking-[0.1em] uppercase">{props.label}</div>
      <div className="mt-2 truncate font-mono text-sm font-semibold">{props.value}</div>
    </div>
  );
}

type PerformanceMetricIconName = "latency" | "requests" | "uptime";

/**
 * The three 36px performance icons from the approved model-detail prototype.
 * The frame is part of each supplied SVG, so the metric-icon wrapper removes
 * its own padding/border when one of these icons is present.
 */
function PerformanceMetricIcon({ name }: { name: PerformanceMetricIconName }) {
  if (name === "latency") {
    return (
      <svg width="36" height="36" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false" aria-hidden="true">
        <rect x="0.5" y="0.5" width="35" height="35" rx="7.5" stroke="#EDEFF2" />
        <path d="M13.9092 13.5H12.9686C14.2052 12.1183 16.0015 11.25 18 11.25C21.7279 11.25 24.75 14.2721 24.75 18C24.75 21.7279 21.7279 24.75 18 24.75C14.2721 24.75 11.25 21.7279 11.25 18C11.25 17.5858 10.9142 17.25 10.5 17.25C10.0858 17.25 9.75 17.5858 9.75 18C9.75 22.5564 13.4436 26.25 18 26.25C22.5564 26.25 26.25 22.5564 26.25 18C26.25 13.4436 22.5564 9.75 18 9.75C15.5994 9.75 13.4388 10.7756 11.9319 12.4108V11.5227C11.9319 11.1085 11.5961 10.7727 11.1819 10.7727C10.7677 10.7727 10.4319 11.1085 10.4319 11.5227V14.25C10.4319 14.6642 10.7677 15 11.1819 15H13.9092C14.3234 15 14.6592 14.6642 14.6592 14.25C14.6592 13.8358 14.3234 13.5 13.9092 13.5Z" fill="#6B38E6" />
        <path d="M18.7519 13.5001C18.752 13.0859 18.4162 12.75 18.002 12.75C17.5878 12.75 17.252 13.0857 17.2519 13.4999L17.2515 18.0032C17.2514 18.2022 17.3305 18.393 17.4711 18.5336L20.6509 21.7134C20.9438 22.0063 21.4187 22.0063 21.7116 21.7134C22.0045 21.4205 22.0045 20.9456 21.7116 20.6527L18.7515 17.6927L18.7519 13.5001Z" fill="#6B38E6" />
      </svg>
    );
  }

  if (name === "requests") {
    return (
      <svg width="36" height="36" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false" aria-hidden="true">
        <rect x="0.5" y="0.5" width="35" height="35" rx="7.5" stroke="#EDEFF2" />
        <path d="M21.9917 9.94644C22.2878 9.65691 22.7626 9.66212 23.0522 9.95816L24.5442 11.4831C24.8325 11.7778 24.829 12.2505 24.5361 12.5407L23.0442 14.018C22.7499 14.3095 22.2751 14.3071 21.9836 14.0128C21.6922 13.7186 21.6938 13.2438 21.988 12.9523L22.1821 12.7597H14.3181C13.9174 12.7597 13.3112 12.8773 12.8284 13.2123C12.3861 13.5193 12.0001 14.0365 12 14.9877C12.0001 15.9398 12.3872 16.4677 12.8335 16.7836C13.3187 17.1269 13.9248 17.2494 14.3181 17.2494H21.7478C22.3748 17.2494 23.2901 17.4245 24.0703 17.9855C24.8892 18.5744 25.4999 19.5503 25.5 20.9943C25.4999 22.4378 24.8898 23.4156 24.0725 24.0075C23.2932 24.5718 22.3776 24.7509 21.7478 24.7509H13.8274L14.0457 24.9669C14.3396 25.2584 14.3413 25.7333 14.05 26.0275C13.7586 26.3217 13.2838 26.3239 12.9895 26.0326L11.4976 24.5553C11.2046 24.2651 11.2011 23.7925 11.4895 23.4977L12.9814 21.9728C13.2711 21.6769 13.746 21.6715 14.042 21.9611C14.3378 22.2507 14.343 22.7256 14.0537 23.0216L13.8289 23.2509H21.7478C22.1406 23.2509 22.7261 23.1297 23.1921 22.7924C23.6201 22.4824 23.9999 21.9565 24 20.9943C23.9999 20.0324 23.6208 19.5102 23.1943 19.2035C22.7292 18.8692 22.1434 18.7494 21.7478 18.7494H14.3181C13.6888 18.7494 12.7602 18.5694 11.967 18.0082C11.1353 17.4196 10.5001 16.4411 10.5 14.9877C10.5001 13.533 11.1369 12.5607 11.9729 11.9804C12.7684 11.4283 13.6962 11.2597 14.3181 11.2597H22.2275L21.98 11.007C21.6904 10.7109 21.6956 10.2361 21.9917 9.94644Z" fill="#6B38E6" />
      </svg>
    );
  }

  return (
    <svg width="36" height="36" viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg" focusable="false" aria-hidden="true">
      <rect x="0.5" y="0.5" width="35" height="35" rx="7.5" stroke="#EDEFF2" />
      <path d="M19.2188 10.6875L16.7813 9.0625V9.89218C12.8314 10.4812 9.80212 13.8865 9.80212 18C9.80212 18.2762 9.81581 18.5494 9.84257 18.8189C9.88349 19.2311 10.2508 19.5321 10.663 19.4912C11.0752 19.4502 11.3762 19.0829 11.3352 18.6707C11.3133 18.4503 11.3021 18.2266 11.3021 18C11.3021 14.7172 13.6643 11.9854 16.7813 11.4126V12.3125L19.2188 10.6875Z" fill="#6B38E6" />
      <path d="M21.2795 10.4845C20.8999 10.3187 20.4578 10.4919 20.2919 10.8715C20.1261 11.2511 20.2994 11.6932 20.6789 11.8591C23.0461 12.8933 24.698 15.2546 24.698 18C24.698 18.865 24.5343 19.6907 24.2365 20.4486L23.5136 20.0313L23.7021 22.9547L26.3282 21.6563L25.5474 21.2055C25.9663 20.2203 26.198 19.1365 26.198 18C26.198 14.6371 24.1733 11.7489 21.2795 10.4845Z" fill="#6B38E6" />
      <path d="M18 13.5C18.4142 13.5 18.75 13.8358 18.75 14.25L18.75 21.75C18.75 22.1642 18.4142 22.5 18 22.5C17.5858 22.5 17.25 22.1642 17.25 21.75L17.25 14.25C17.25 13.8358 17.5858 13.5 18 13.5Z" fill="#6B38E6" />
      <path d="M15.75 15C16.1642 15 16.5 15.3358 16.5 15.75V20.25C16.5 20.6642 16.1642 21 15.75 21C15.3358 21 15 20.6642 15 20.25L15 15.75C15 15.3358 15.3358 15 15.75 15Z" fill="#6B38E6" />
      <path d="M21 15.75C21 15.3358 20.6642 15 20.25 15C19.8358 15 19.5 15.3358 19.5 15.75V20.25C19.5 20.6642 19.8358 21 20.25 21C20.6642 21 21 20.6642 21 20.25V15.75Z" fill="#6B38E6" />
      <path d="M22.5 16.3125C22.9142 16.3125 23.25 16.6483 23.25 17.0625V18.9375C23.25 19.3517 22.9142 19.6875 22.5 19.6875C22.0858 19.6875 21.75 19.3517 21.75 18.9375V17.0625C21.75 16.6483 22.0858 16.3125 22.5 16.3125Z" fill="#6B38E6" />
      <path d="M14.25 17.0625C14.25 16.6483 13.9142 16.3125 13.5 16.3125C13.0858 16.3125 12.75 16.6483 12.75 17.0625L12.75 18.9375C12.75 19.3517 13.0858 19.6875 13.5 19.6875C13.9142 19.6875 14.25 19.3517 14.25 18.9375V17.0625Z" fill="#6B38E6" />
      <path d="M10.6875 23.361L11.445 22.9237C12.94 24.911 15.3194 26.1979 18.0001 26.1979C19.8446 26.1979 21.5483 25.5878 22.9177 24.5648C23.2496 24.3169 23.3176 23.8469 23.0697 23.515C22.8218 23.1832 22.3518 23.1152 22.02 23.3631C20.8999 24.1999 19.5089 24.6979 18.0001 24.6979C15.8768 24.6979 13.9835 23.7102 12.7557 22.1669L13.5021 21.736L10.8761 20.4375L10.6875 23.361Z" fill="#6B38E6" />
    </svg>
  );
}

function FlatkeyMetricCard(props: { label: string; value: string; valueNode?: ReactNode; note?: string; icon?: ReactNode }) {
  return (
    <div className="metric-card">
      <div className="metric-main">
        <span className="metric-icon">{props.icon ?? <Gauge aria-hidden="true" />}</span>
        <div className="metric-copy">
          <div className="metric-label">{props.label}</div>
          <div className="metric-value">{props.valueNode ?? props.value}</div>
        </div>
      </div>
      {props.note ? <div className="metric-note">{props.note}</div> : null}
    </div>
  );
}

function HeroPriceBreakdown(props: { rows: Array<{ resolution?: string; value: string }>; reference?: boolean }) {
  if (props.rows.length === 0) return <>—</>;
  return (
    <span className="model-hero-price-breakdown">
      {props.rows.map((row) => (
        <span className="model-hero-price-item" key={`${row.resolution ?? "default"}-${row.value}`}>
          {row.resolution ? <span className="model-hero-price-resolution">{row.resolution}</span> : null}
          {(() => {
            const match = /^(from\s+)?(\$[\d.,]+)(.*)$/.exec(row.value);
            if (!props.reference || !match) {
              return <span className="model-hero-price-value">{row.value}</span>;
            }
            return (
              <span className="model-hero-price-value model-hero-price-value-reference">
                {match[1] ?? ""}<s>{match[2]}</s><span className="model-hero-price-suffix">{match[3]}</span>
              </span>
            );
          })()}
        </span>
      ))}
    </span>
  );
}

function FlatkeyPriceRow(props: { row: FlatkeyPriceTableRow; officialLabel: string; flatkeyLabel: string }) {
  return (
    <div className="rounded-xl border border-violet-500/12 bg-white/62 p-4 dark:bg-white/[0.03]">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <span className="font-mono text-sm font-semibold">{props.row.label}</span>
      </div>
      <div className="grid gap-2">
        <PriceTrack label={props.officialLabel} value={props.row.official} percent={props.row.officialPercent} kind="official" />
        <PriceTrack label={props.flatkeyLabel} value={props.row.flatkey} percent={props.row.flatkeyPercent} kind="flatkey" />
      </div>
    </div>
  );
}

function PriceTrack(props: { label: string; value: string; percent: number; kind: "flatkey" | "official" }) {
  return (
    <div className="price-track">
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

function formatDetailUsdPrice(value: number): string {
  if (!Number.isFinite(value)) return "-";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: 3,
    maximumFractionDigits: 3,
  }).format(value);
}

function buildFlatkeyPriceRows(
  config: ModelConfig,
  model: PricingModel | null,
  groupRatio: Record<string, number>,
  t: (key: string, vars?: Record<string, string>) => string
): { rows: FlatkeyPriceTableRow[]; note: string } {
  const note = t("Prices below are calculated from Flatkey pricing data for this model and the visible groups currently returned by our pricing API.");
  const unavailablePrice = t("Pricing data unavailable");
  if (!model) {
    return {
      note,
      rows: [{ label: t("Pricing"), flatkey: unavailablePrice, official: unavailablePrice, flatkeyPercent: 0, officialPercent: 0 }],
    };
  }

  if (config.generator?.kind === "video" && model.display_pricing?.second_by_resolution) {
    const resolutionRows = buildVideoResolutionPriceRows(config, model, t);
    if (resolutionRows.length > 0) return { note, rows: resolutionRows };
  }
  if (config.slug === "seedance-2.5") {
    const resolutionRows = buildSeedanceFallbackPriceRows(config, model, t, groupRatio);
    if (resolutionRows.length > 0) return { note, rows: resolutionRows };
  }

  const defaultDisplayPrice = resolveModelDisplayPrice(model, undefined, "plg", groupRatio);
  if (config.generator?.kind === "image") {
    const imagePrice = resolveModelDisplayPrice(model, "image", "plg", groupRatio);
    const officialImagePrice = resolveModelDisplayPrice(model, "image", "configured", groupRatio);
    if (imagePrice) {
      const official = officialImagePrice?.value ?? imagePrice.configured ?? imagePrice.value;
      return {
        note,
        rows: [{
          label: t("Price / image"),
          flatkey: `${imagePrice.from ? "from " : ""}${formatDetailUsdPrice(imagePrice.value)} ${t("/ image")}`,
          official: `${officialImagePrice?.from ? "from " : ""}${formatDetailUsdPrice(official)} ${t("/ image")}`,
          flatkeyPercent: pricePercent(imagePrice.value, official),
          officialPercent: 100,
        }],
      };
    }
  }
  // Use the same default display dimension as the model directory. Some image
  // models are billed through the token path in the live catalog, so forcing
  // an image-only price here would make the detail card disagree with the
  // list even though both values came from the same pricing payload.
  if (isTokenBasedModel(model) && defaultDisplayPrice?.unit === "/ 1M tokens") {
    return buildLiveTokenPriceRows(model, groupRatio, note, t, defaultDisplayPrice.from ? "from " : "");
  }

  if (config.generator) {
    const generatorKind = config.generator.kind;
    const dimension = generatorKind === "image" ? "image" : generatorKind === "audio" ? "request" : "second";
    // For generic catalog pages, the directory's default display dimension is
    // authoritative. A video endpoint can be request-priced (for example Veo)
    // and an image endpoint can be billed per request, so deriving the unit
    // from the generator kind would make this card disagree with the list.
    const perUnit = defaultDisplayPrice ?? resolveModelDisplayPrice(model, dimension, "plg", groupRatio);
    if (perUnit) {
      const officialDisplayPrice = perUnit.configured != null
        ? resolveModelDisplayPrice(model, perUnit.dimension, "configured", groupRatio)
        : null;
      const official = officialDisplayPrice?.value ?? perUnit.configured ?? perUnit.value;
      const flatkey = perUnit.value;
      const unitKey = perUnit.unit === "/ second"
        ? "/ second"
        : perUnit.unit === "/ request"
          ? "/ request"
          : generatorKind === "image"
            ? "/ image"
            : generatorKind === "audio"
              ? "/ request"
              : "/ second";
      const labelKey = unitKey === "/ second" ? "Price / second" : unitKey === "/ request" ? "Request price" : "Price / image";
      // Keep the catalog's `from` marker while using a fixed three-decimal
      // presentation on detail pages so small prices remain comparable.
      const displayPrice = (price: typeof perUnit, fallback: number) => {
        const prefix = price.from ? "from " : "";
        return `${prefix}${formatDetailUsdPrice(Number.isFinite(price.value) ? price.value : fallback)} ${t(unitKey)}`;
      };
      return {
        note,
        rows: [{
          label: t(labelKey),
          flatkey: displayPrice(perUnit, flatkey),
          official: officialDisplayPrice
            ? displayPrice(officialDisplayPrice, official)
            : `${formatDetailUsdPrice(official)} ${t(unitKey)}`,
          flatkeyPercent: pricePercent(flatkey, official),
          officialPercent: 100,
        }],
      };
    }
    return {
      note,
      rows: [{ label: t("Pricing"), flatkey: unavailablePrice, official: unavailablePrice, flatkeyPercent: 0, officialPercent: 0 }],
    };
  }

  if (!isTokenBasedModel(model)) {
    const requestPrice = resolveModelDisplayPrice(model, "request", "plg", groupRatio);
    const officialRequestPrice = resolveModelDisplayPrice(model, "request", "configured", groupRatio);
    if (!requestPrice) {
      return {
        note,
        rows: [{ label: t("Pricing"), flatkey: unavailablePrice, official: unavailablePrice, flatkeyPercent: 0, officialPercent: 0 }],
      };
    }
    const official = officialRequestPrice?.value ?? requestPrice.value;
    const flatkey = requestPrice.value;
    return {
      note,
      rows: [{
        label: t("Request price"),
        flatkey: `${formatUsdPrice(flatkey)} ${t("/ request")}`,
        official: `${formatUsdPrice(official)} ${t("/ request")}`,
        flatkeyPercent: pricePercent(flatkey, official),
        officialPercent: 100,
      }],
    };
  }

  return buildLiveTokenPriceRows(model, groupRatio, note, t);
}

function buildVideoResolutionPriceRows(
  config: ModelConfig,
  model: PricingModel,
  t: (key: string, vars?: Record<string, string>) => string,
): FlatkeyPriceTableRow[] {
  const entries = Object.entries(model.display_pricing?.second_by_resolution ?? {})
    .map(([resolution, pair]) => {
      const flatkey = pair.plg ?? pair.configured;
      const official = pair.configured ?? flatkey;
      if (flatkey == null || official == null || !Number.isFinite(flatkey) || !Number.isFinite(official)) return null;
      const minimum = config.generator?.fields.find((field) => field.name === "duration")?.min;
      const from = pair.from ? "from " : "";
      const duration = minimum != null ? ` · ${minimum}s min` : "";
      return {
        label: `${resolution} · ${t("Price / second")}`,
        flatkey: `${from}${formatDetailUsdPrice(flatkey)} ${t("/ second")}${duration}`,
        official: `${from}${formatDetailUsdPrice(official)} ${t("/ second")}${duration}`,
        flatkeyPercent: pricePercent(flatkey, official),
        officialPercent: 100,
      } satisfies FlatkeyPriceTableRow;
    })
    .filter((row): row is FlatkeyPriceTableRow => row != null);
  return entries.sort((a, b) => a.label.localeCompare(b.label, "en", { numeric: true }));
}

function buildSeedanceFallbackPriceRows(
  config: ModelConfig,
  model: PricingModel,
  t: (key: string, vars?: Record<string, string>) => string,
  groupRatio: Record<string, number>,
): FlatkeyPriceTableRow[] {
  // Audited Seedance 2.5 output-duration rates. The live API's aggregate
  // second price is retained for compatibility, but cannot identify the
  // resolution tier; use the visible group ratio to calculate our price until
  // the API returns second_by_resolution.
  const tiers = [
    ["480p", 0.14],
    ["720p", 0.314],
  ] as const;
  const ratio = getBestGroupRatio(model, groupRatio);
  const minimum = config.generator?.fields.find((field) => field.name === "duration")?.min;
  const duration = minimum != null ? ` · ${minimum}s min` : "";
  return tiers.map(([resolution, official]) => {
    const flatkey = official * ratio;
    return {
      label: `${resolution} · ${t("Price / second")}`,
      flatkey: `${formatDetailUsdPrice(flatkey)} ${t("/ second")}${duration}`,
      official: `${formatDetailUsdPrice(official)} ${t("/ second")}${duration}`,
      flatkeyPercent: pricePercent(flatkey, official),
      officialPercent: 100,
    };
  });
}

function buildLiveTokenPriceRows(
  model: PricingModel,
  groupRatio: Record<string, number>,
  note: string,
  t: (key: string, vars?: Record<string, string>) => string,
  prefix = ""
): { rows: FlatkeyPriceTableRow[]; note: string } {
  const dimensions = [
    ["input", "Input /M"],
    ["cache", "Cache /M"],
    ["output", "Output /M"],
  ] as const;
  return dimensions.flatMap(([type, label]) => {
    const plgPrice = resolveModelDisplayPrice(model, type, "plg", groupRatio);
    const configuredPrice = resolveModelDisplayPrice(model, type, "configured", groupRatio);
    // Cache is optional in the upstream catalog. Only add the row when the
    // live display contract or the legacy cache ratio exposes a real value.
    if (!plgPrice && !configuredPrice) return [];
    const official = configuredPrice?.value ?? getOfficialPriceUsd(model, type === "output" ? "output" : "input");
    const flatkey = plgPrice?.value ?? discountedPriceUsd(official * getBestGroupRatio(model, groupRatio));
    return [{
      label: t(label),
      flatkey: `${prefix}${formatUsdPrice(flatkey)}`,
      official: `${prefix}${formatUsdPrice(official)}`,
      flatkeyPercent: pricePercent(flatkey, official),
      officialPercent: 100,
    }];
  }).reduce<{ rows: FlatkeyPriceTableRow[]; note: string }>(
    (result, row) => ({ ...result, rows: [...result.rows, row] }),
    { rows: [], note }
  );
}

function buildCatalogRelatedModels(
  config: ModelConfig,
  locale: Locale,
  allModels: PricingModel[],
  t: (key: string, vars?: Record<string, string>) => string
) {
  const configured = config.landingContent?.related;
  if (configured?.cards.length) {
    return {
      title: t(configured.title),
      models: configured.cards.map((card) => ({
        href: localizePath(card.href, locale),
        name: t(card.name),
        description: t(card.description),
        sameProvider: false,
        asset: card.asset,
      })),
    };
  }
  const current = normalizeModelId(config.modelId);
  const provider = allModels.find((model) => normalizeModelId(model.model_name) === current)?.vendor_name ?? config.officialName;
  const family = getModelFamilyKey(config.modelId);
  const currentEndpoints = new Set((allModels.find((model) => normalizeModelId(model.model_name) === current)?.supported_endpoint_types ?? []).map(normalizeModelId));
  const currentModality = configModalityKey(config);
  const scoredRelated = allModels
    // A detail page should only recommend models that accept the same kind of
    // input/output. This keeps a video page from surfacing text or image
    // models simply because they share a provider or endpoint family.
    .filter((model) => normalizeModelId(model.model_name) !== current && modelModalityKey(model) === currentModality)
    .map((model) => {
      const endpointMatch = (model.supported_endpoint_types ?? []).some((endpoint) => currentEndpoints.has(normalizeModelId(endpoint)));
      const modalityMatch = modelModalityKey(model) === currentModality;
      const score =
        model.vendor_name === provider ? 0 :
        getModelFamilyKey(model.model_name) === family ? 1 :
        endpointMatch ? 2 :
        modalityMatch ? 3 :
        4;
      return { model, score };
    });
  const preferredRelated = scoredRelated.filter((item) => item.score < 4);
  const liveRelated = preferredRelated
    .sort((a, b) => a.score - b.score || a.model.model_name.localeCompare(b.model.model_name, "en", { numeric: true }))
    .slice(0, 8)
    .map(({ model }): CatalogRelatedModel => ({
      href: localizePath(`/models/${encodeURIComponent(model.model_name)}`, locale),
      name: model.model_name,
      // Catalog descriptions are provider-supplied and currently English.
      // Keep them on the English page, while localized pages use a stable,
      // translated label instead of mixing an English sentence into the card.
      description: locale === "en" && model.description
        ? model.description
        : t("Live catalog model"),
      sameProvider: model.vendor_name === provider,
      asset: relatedModelAsset(model, currentModality),
    }));

  if (liveRelated.length > 0) {
    return {
      title: configured
        ? t(configured.title)
        : liveRelated.every((model) => model.sameProvider)
        ? t("More models from {{provider}}", { provider })
        : t("Keep exploring Flatkey"),
      models: liveRelated,
    };
  }

  // An explicitly empty related list means this page is intentionally
  // catalog-driven. Do not fall back to the old hand-authored mixed list when
  // the live catalog has no same-modality entries yet.
  if (configured) {
    return { title: t(configured.title), models: [] };
  }

  return {
    title: t("Related models"),
    models: [],
  };
}

const FEATURED_MODEL_ASSETS = [
  { modelName: "deepseek-v4-pro", asset: "/assets/models-featured/deepseek.jpg" },
  { modelName: "kimi-k3", asset: "/assets/models-featured/moonshot.jpg" },
  { modelName: "minimax-h3", asset: "/assets/models-featured/minimax.jpg" },
  { modelName: "seedance-2-5", asset: "/assets/models-featured/bytedance.jpg" },
  { modelName: "gpt-5-6-sol", asset: "/assets/models-featured/gpt-5.6-sol.png" },
  { modelName: "glm-5-3", asset: "/assets/models-featured/zhipu.jpg" },
];

function RelatedModelCover({ src: _src, alt, sizes }: { src: string; alt: string; sizes: string }) {
  return (
    <div
      className="related-card-top"
      data-cover-sizes={sizes}
      aria-label={alt}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src="/flatkey-lockup-light.svg"
        alt="flatkey.ai"
        className="related-card-brand"
        loading="lazy"
      />
      <div className="related-card-logo-mark" aria-hidden="true">
        <HomeModelLogo
          modelName={alt}
          className="related-card-logo-bare"
          fallback={alt.charAt(0)}
          surfaceSize={150}
          imageSize={126}
        />
      </div>
    </div>
  );
}

function relatedModelAsset(model: PricingModel, modality: string) {
  const catalogAsset = [model.icon, model.vendor_icon].find((value) => /^https?:\/\//i.test(value ?? ""));
  if (catalogAsset) return catalogAsset;

  // The pricing payload normally stores a provider/model icon key rather than
  // a full URL. Resolve that key to the same public CDN used by the pricing
  // browser so related cards never depend on local placeholder photography.
  const iconKey = model.icon || model.vendor_icon || modelIconKey(model.model_name, model.vendor_name ?? "");
  const normalizedIconKey = normalizeRelatedIconKey(iconKey);
  return normalizedIconKey
    ? `https://cdn.jsdelivr.net/npm/@lobehub/icons-static-svg@latest/icons/${normalizedIconKey}.svg`
    : relatedModelAssetFromName(model.model_name, model.vendor_name, modality);
}

function normalizeRelatedIconKey(iconKey: string) {
  const known: Record<string, string> = {
    openai: "openai",
    "open-ai": "openai",
    anthropic: "anthropic",
    claude: "claude-color",
    google: "google-color",
    gemini: "gemini-color",
    deepseek: "deepseek-color",
    "deep-seek": "deepseek-color",
    qwen: "qwen-color",
    alibaba: "alibabacloud-color",
    "alibaba-cloud": "alibabacloud-color",
    mistral: "mistral-color",
    xai: "xai",
    grok: "grok",
    meta: "meta-color",
    llama: "meta-color",
    moonshot: "moonshot",
    kimi: "kimi-color",
    bytedance: "bytedance-color",
    seedance: "bytedance-color",
    minimax: "minimax-color",
    kuaishou: "kuaishou-color",
  };
  const normalized = iconKey
    .split(".")
    .filter((segment) => segment && !segment.includes("="))
    .join("-")
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  // Only use keys that are known to exist in the CDN icon set. Generic values
  // such as `ai` are often metadata, not an actual icon asset, and would
  // otherwise render as a broken image instead of using the local cover.
  return normalized ? known[normalized] ?? null : null;
}

function relatedModelAssetFromName(name: string, vendor: string | undefined, modality: string) {
  const modelKey = normalizeModelId(name);
  const vendorKey = normalizeModelId(vendor ?? "");
  const featured = FEATURED_MODEL_ASSETS.find(
    (slide) => modelKey === slide.modelName || modelKey.startsWith(`${slide.modelName}-`)
  );
  if (featured) return featured.asset;

  const familyKey = `${modelKey} ${vendorKey}`;
  if (familyKey.includes("deepseek")) return "/assets/models-featured/deepseek.jpg";
  if (familyKey.includes("kimi") || familyKey.includes("moonshot")) return "/assets/models-featured/moonshot.jpg";
  if (familyKey.includes("minimax")) return "/assets/models-featured/minimax.jpg";
  if (familyKey.includes("seedance") || familyKey.includes("bytedance") || familyKey.includes("doubao")) {
    return "/assets/models-featured/bytedance.jpg";
  }
  if (familyKey.includes("gpt") || familyKey.includes("openai")) return "/assets/models-featured/openai.jpg";
  if (familyKey.includes("glm") || familyKey.includes("zhipu")) return "/assets/models-featured/zhipu.jpg";
  if (modality === "video") return "/assets/video/v1.1.jpg";
  if (modality === "image") return "/assets/prompts/awesome-images/gpt-image-2-showcase-complex.png";
  if (modality === "audio") return "/assets/prompts/awesome-images/ai-agent-poster.png";
  return "/assets/prompts/awesome-images/saas-hero-phone.png";
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

function buildHumanReadableModelTitle(
  config: ModelConfig,
  locale: Locale,
): string {
  const name = config.displayName.replace(/\s+(?:AI\s+)?(?:video|image|audio)?\s*(?:generator\s+and\s+)?API$/i, "").trim();
  const kind = config.generator?.kind;
  const suffixes: Record<Locale, { text: string; image: string; video: string; audio: string }> = {
    en: { text: "API", image: "AI Image API", video: "AI Video API", audio: "Audio API" },
    zh: { text: "API", image: "AI 图像 API", video: "AI 视频 API", audio: "音频 API" },
    es: { text: "API", image: "API de imágenes con IA", video: "API de vídeo con IA", audio: "API de audio" },
    fr: { text: "API", image: "API d’images IA", video: "API vidéo IA", audio: "API audio" },
    pt: { text: "API", image: "API de imagem com IA", video: "API de vídeo com IA", audio: "API de áudio" },
    ru: { text: "API", image: "API генерации изображений", video: "API генерации видео", audio: "Аудио API" },
    ja: { text: "API", image: "AI画像API", video: "AI動画API", audio: "音声API" },
    vi: { text: "API", image: "API hình ảnh AI", video: "API video AI", audio: "API âm thanh" },
    de: { text: "API", image: "KI-Bild-API", video: "KI-Video-API", audio: "Audio-API" },
    id: { text: "API", image: "API gambar AI", video: "API video AI", audio: "API audio" },
  };
  const suffix = suffixes[locale] ?? suffixes.en;
  return `${name} ${kind === "image" ? suffix.image : kind === "video" ? suffix.video : kind === "audio" ? suffix.audio : suffix.text}`;
}

function buildModelDescription(
  config: ModelConfig,
  model: PricingModel | null,
  t: (key: string, vars?: Record<string, string>) => string,
  locale: Locale = "en"
) {
  // Seedance's catalog description is not the editorial source of truth for
  // this page. Keep the opening answer aligned with the audited contract in
  // every locale, including the JSON-LD description, instead of allowing a
  // stale live-catalog paragraph to reintroduce unsupported claims.
  if (config.slug === "seedance-2.5") {
    return t("Seedance 2.5 is ByteDance's audio-video generation model for text-to-video and image-to-video requests, with reference media and optional audio controls.");
  }
  // Priority pages carry an audited opening answer in landingContent. Keep it
  // as the source for both the visible hero and JSON-LD instead of allowing a
  // stale provider description to overwrite the target-specific facts.
  if (config.landingContent?.hero?.description) {
    return t(config.landingContent.hero.description);
  }
  // Live catalog descriptions are currently English. Keep them on the
  // English page, but use the localized shell copy on other locales so a
  // translated detail page does not mix an English paragraph into its hero.
  if (model?.description && locale === "en") return model.description;
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
  if (config.landingContent?.faq) {
    return config.landingContent.faq.map((item) => ({ question: t(item.question), answer: t(item.answer) }));
  }
  return [
    {
      question: t("What is {{model}}?", { model: config.displayName }),
      answer: buildModelDescription(config, null, t),
    },
    {
      question: t("How much does {{model}} cost?", { model: config.displayName }),
      answer: t("Use the pricing section above for current Flatkey prices from our pricing API."),
    },
    {
      question: t("Which providers serve {{model}}?", { model: config.displayName }),
      answer: t("The providers section shows the upstream provider names available in our model catalog."),
    },
    ...config.faq.map((item) => ({ question: t(item.question), answer: t(item.answer) })),
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

function pricePercent(value: number, official: number) {
  if (!Number.isFinite(value) || !Number.isFinite(official) || official <= 0 || value <= 0) return 0;
  return Math.round(Math.max(6, Math.min(100, (value / official) * 100)));
}

function formatModelDate(timestamp?: number) {
  if (!timestamp || !Number.isFinite(timestamp)) return "—";
  const millis = timestamp > 10_000_000_000 ? timestamp : timestamp * 1000;
  return new Date(millis).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
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
    <div className="request-preview mt-5 rounded-2xl border border-violet-500/16 bg-white/72 p-4 shadow-[0_24px_70px_-56px_rgba(91,33,182,0.72)] backdrop-blur-sm sm:p-5 dark:bg-white/[0.04]">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3 text-sm font-semibold text-[#2c2d33] dark:text-white/88">
        <span>{props.t("Request preview")}</span>
        <button
          type="button"
          onClick={() => navigator.clipboard?.writeText(JSON.stringify(request, null, 2)).catch(() => undefined)}
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
    <div className="panel-heading">
      <h3>{props.title}</h3>
      <span className="panel-note">
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
  const values: Record<string, string | number | boolean> = Object.fromEntries(
    (config.generator?.fields ?? []).map((field) => [field.name, field.defaultValue])
  );
  const videoModes = config.generator?.videoModes;
  if (config.generator?.kind === "video" && videoModes?.length) {
    values.video_mode = config.generator.defaultVideoMode
      ?? videoModes.find((option) => option.supported)?.value
      ?? videoModes[0].value;
  }
  return values;
}

function buildGeneratorRequest(
  config: ModelConfig,
  prompt: string,
  values: Record<string, string | number | boolean>,
  referenceImages: ReferenceImageDraft[] = []
) {
  // `video_mode` is Playground-only metadata. The gateway's documented
  // Seedance wire contract infers the workflow from content[] item roles, so
  // never leak this selector value as an unsupported API parameter.
  const { video_mode: _videoMode, ...requestValues } = values;
  if (config.generator?.kind === "video") {
    if (config.generator.protocol === "grok-video") {
      return compactRequest({ model: config.modelId, prompt, duration: values.duration ?? 5 });
    }
    if (config.generator.protocol === "veo-video") {
      return compactRequest({
        model: config.modelId,
        prompt,
        duration: Number(values.duration ?? 8),
        size: values.size ?? "1280x720",
      });
    }
    const content = [{ type: "text", text: prompt }];
    return compactRequest({ model: config.modelId, content, ...requestValues });
  }
  if (config.generator?.kind === "audio") {
    return compactRequest({ model: config.modelId, input: prompt, ...requestValues });
  }
  if (config.generator?.kind === "image") {
    if (config.generator.protocol === "gemini-image") {
      const imageConfig = compactRequest({
        aspectRatio: values.aspect_ratio,
        imageSize: values.image_size,
      });
      return {
        model: config.modelId,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
        generationConfig: {
          responseModalities: ["TEXT", "IMAGE"],
          imageConfig,
        },
      };
    }
    return compactRequest({
      model: config.modelId,
      prompt,
      ...requestValues,
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
    Object.entries(value).filter(([, entry]) => entry !== "" && entry !== 0 && entry !== undefined)
  );
}

function buildRunHref(
  config: ModelConfig,
  locale: Locale,
  prompt: string,
  draft: DraftValue
) {
  // Audio model detail pages are API-only. Keep accidental callers from
  // constructing a public Playground URL if an older landing component is
  // reintroduced or a new CTA forgets to apply the page-level gate.
  if (config.generator?.kind === "audio") return consoleUrl("/dashboard");
  const playgroundParams = new URLSearchParams({
    model: config.modelId,
    prompt,
    lng: locale,
    draft: JSON.stringify(draft),
  });
  if (config.generator?.kind === "image" || config.generator?.kind === "video") {
    playgroundParams.set("generate", config.generator.kind);
  }
  return consoleUrl("/playground", playgroundParams.toString());
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
  if (field.type !== "number" && typeof field.defaultValue !== "number") return raw;
  const value = Number(raw);
  if (!Number.isFinite(value)) return field.defaultValue;
  return Math.min(field.max ?? value, Math.max(field.min ?? value, value));
}

type QuickPromptKind = "image" | "video" | "audio";

const QUICK_PROMPT_COPY: Record<Locale, Record<QuickPromptKind, string>> = {
  en: {
    image: "{{label}}: a high-quality product visual with clean composition, precise lighting, strong subject focus, and realistic detail.",
    video: "{{label}}: a concise commercial video shot with clear subject motion, realistic lighting, stable camera, and production-ready framing.",
    audio: "{{label}}: a clean studio-quality audio generation brief with precise tone, pacing, ambience, and delivery notes.",
  },
  zh: {
    image: "{{label}}：高质量产品视觉，构图干净、光线精准、主体突出、细节真实。",
    video: "{{label}}：简洁的商业视频镜头，主体运动清晰、光线真实、镜头稳定，画面可直接用于制作。",
    audio: "{{label}}：清晰的录音棚级音频生成简报，明确音色、节奏、环境氛围和交付要求。",
  },
  es: {
    image: "{{label}}: una imagen de producto de alta calidad, con composición limpia, iluminación precisa, sujeto destacado y detalle realista.",
    video: "{{label}}: un plano comercial conciso con movimiento claro del sujeto, iluminación realista, cámara estable y encuadre listo para producción.",
    audio: "{{label}}: un briefing de generación de audio con calidad de estudio, tono, ritmo, ambiente y notas de entrega precisos.",
  },
  fr: {
    image: "{{label}} : un visuel produit de haute qualité, à la composition nette, lumière précise, sujet bien mis en avant et détails réalistes.",
    video: "{{label}} : un plan publicitaire concis avec un mouvement clair du sujet, une lumière réaliste, une caméra stable et un cadrage prêt pour la production.",
    audio: "{{label}} : un brief de génération audio de qualité studio, avec des indications précises sur le timbre, le rythme, l’ambiance et la livraison.",
  },
  pt: {
    image: "{{label}}: um visual de produto de alta qualidade, com composição limpa, iluminação precisa, foco no produto e detalhes realistas.",
    video: "{{label}}: um plano comercial conciso, com movimento claro do assunto, iluminação realista, câmera estável e enquadramento pronto para produção.",
    audio: "{{label}}: um briefing de geração de áudio com qualidade de estúdio, tom, ritmo, ambiente e instruções de entrega precisos.",
  },
  ru: {
    image: "{{label}}: качественный визуал продукта с чистой композицией, точным светом, акцентом на объекте и реалистичными деталями.",
    video: "{{label}}: короткий рекламный кадр с понятным движением объекта, реалистичным светом, стабильной камерой и готовой к производству композицией.",
    audio: "{{label}}: чёткое техническое задание на студийную генерацию аудио с указанием тембра, темпа, атмосферы и требований к выдаче.",
  },
  ja: {
    image: "{{label}}：クリーンな構図、正確な照明、被写体を際立たせるフォーカス、リアルなディテールを備えた高品質な商品ビジュアル。",
    video: "{{label}}：被写体の動きを明確にし、自然な照明、安定したカメラ、制作に使えるフレーミングでまとめた簡潔な広告ショット。",
    audio: "{{label}}：音色、テンポ、環境音、納品条件を明確にした、スタジオ品質の音声生成ブリーフ。",
  },
  vi: {
    image: "{{label}}: hình ảnh sản phẩm chất lượng cao với bố cục gọn, ánh sáng chính xác, chủ thể nổi bật và chi tiết chân thực.",
    video: "{{label}}: một cảnh quảng cáo ngắn với chuyển động chủ thể rõ ràng, ánh sáng thực tế, camera ổn định và khung hình sẵn sàng sản xuất.",
    audio: "{{label}}: bản mô tả tạo âm thanh chất lượng phòng thu, nêu rõ âm sắc, nhịp điệu, không gian và yêu cầu bàn giao.",
  },
  de: {
    image: "{{label}}: ein hochwertiges Produktmotiv mit klarer Komposition, präzisem Licht, starkem Motivfokus und realistischen Details.",
    video: "{{label}}: ein kurzer Werbeshot mit klarer Motivbewegung, realistischem Licht, stabiler Kamera und produktionsfertigem Bildausschnitt.",
    audio: "{{label}}: ein Studio-Audio-Briefing mit präzisen Angaben zu Klangfarbe, Tempo, Atmosphäre und Auslieferung.",
  },
  id: {
    image: "{{label}}: visual produk berkualitas tinggi dengan komposisi bersih, pencahayaan presisi, fokus subjek yang kuat, dan detail realistis.",
    video: "{{label}}: cuplikan komersial singkat dengan gerak subjek yang jelas, pencahayaan realistis, kamera stabil, dan framing siap produksi.",
    audio: "{{label}}: brief pembuatan audio berkualitas studio dengan nada, tempo, suasana, dan catatan pengiriman yang jelas.",
  },
};

function buildQuickPrompt(label: string, kind: QuickPromptKind, locale: Locale) {
  const template = QUICK_PROMPT_COPY[locale]?.[kind] ?? QUICK_PROMPT_COPY.en[kind];
  return template.replace("{{label}}", label);
}
