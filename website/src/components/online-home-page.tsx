import Link from "next/link";
import type { CSSProperties, ReactNode } from "react";
import { getHomeCopy } from "@/lib/home-copy";
import { type Locale, localizePath } from "@/lib/locales";
import {
  getOnlineHomeToolText,
  getOnlineStaticCopy,
  getOnlineStaticText,
} from "@/lib/online-static-copy";
import { consoleUrl } from "@/lib/origins";
import { OnlineStaticShell } from "./online-static-shell";
import { ModelStripCarousel } from "./model-strip-carousel";
import { IntelligenceVideo } from "./intelligence-video";

const providers = [
  ["logos/claude.svg", "Claude", "reasoning + coding"],
  ["logos/openai.svg", "GPT", "language + multimodal"],
  ["logos/bytedance.svg", "Seedance 2.0", "video generation"],
  ["logos/zai.svg", "GLM 5.2", "reasoning + agents"],
  ["logos/moonshotai.svg", "Kimi", "long context + agents"],
  ["logos/googlegemini.svg", "Gemini", "multimodal intelligence"],
  ["logos/minimax.svg", "MiniMax", "image + music"],
  ["logos/deepseek.svg", "DeepSeek", "reasoning + coding"],
  ["tools/x.svg", "X / Twitter", "posts + profiles"],
  ["tools/linkedin.svg", "LinkedIn", "people + companies"],
  ["tools/tiktok.svg", "TikTok", "videos + trends"],
  ["tools/apify.png", "Apify", "actors + crawlers"],
  ["tools/browserbase.png", "Browserbase", "browser automation"],
  ["tools/exa.png", "Exa", "neural web search"],
  ["tools/apollo.svg", "Apollo", "sales intelligence"],
  ["tools/pdl.png", "People Data Labs", "people enrichment"],
];

const providerRows = [
  [
    ["logos/claude.svg", "Claude", "reasoning + coding"],
    ["logos/openai.svg", "GPT", "language + multimodal"],
    ["logos/bytedance.svg", "Seedance 2.0", "video generation"],
    ["logos/zai.svg", "GLM 5.2", "reasoning + agents"],
    ["logos/moonshotai.svg", "Kimi", "long context + agents"],
    ["logos/googlegemini.svg", "Gemini", "multimodal intelligence"],
    ["logos/minimax.svg", "MiniMax", "image + music"],
    ["logos/deepseek.svg", "DeepSeek", "reasoning + coding"],
  ],
  [
    ["tools/x.svg", "X / Twitter", "posts + profiles"],
    ["tools/linkedin.svg", "LinkedIn", "people + companies"],
    ["tools/tiktok.svg", "TikTok", "videos + trends"],
    ["tools/instagram.svg", "Instagram", "posts + creators"],
    ["tools/youtube.svg", "YouTube", "videos + transcripts"],
    ["tools/reddit.svg", "Reddit", "communities + intent"],
  ],
  [
    ["tools/apify.png", "Apify", "actors + crawlers"],
    ["tools/browserbase.png", "Browserbase", "browser automation"],
    ["tools/exa.png", "Exa", "neural web search"],
    ["tools/google.png", "Google Reviews", "local review data"],
    ["tools/amazon.svg", "Amazon", "products + reviews"],
  ],
  [
    ["tools/apollo.svg", "Apollo", "sales intelligence"],
    ["tools/pdl.png", "People Data Labs", "people enrichment"],
    ["tools/semrush.svg", "Semrush", "search intelligence"],
    ["tools/wokelo.png", "Wokelo", "private markets"],
    ["tools/saperly.png", "Saperly", "agent phone"],
    ["tools/openweather.svg", "OpenWeather", "real-time signals"],
  ],
];

const modelTiles = [
  ["gpt-5.5", "openai · 1M", "$3.33/M · 67% of list", false, true],
  ["claude-opus-4-8", "anthropic · 500K", "$3.33/M · 67% of list", false, true],
  ["claude-opus-5", "anthropic · 1M", "$4.50/M · 90% of list", false, true],
  ["gemini-3.2-pro", "google · 2M", "$1.50/M", false, true],
  ["deepseek-v4", "deepseek · 256K", "$0.17/M", false, true],
  ["qwen3-max", "alibaba", "$0.72/M", false, false],
  ["glm-5.2", "zhipu · 200K", "$0.56/M", false, true],
] as const;

const videoCards = [
  [
    "v1.1",
    "Seedance 2.5",
    "Image to Video",
    "bytedance · 1080p · t2v + i2v",
    "Early access",
    "/playground?model=seedance-2.5",
    true,
  ],
  [
    "v1.2",
    "Seedance 2.0",
    "Image to Video",
    "bytedance · i2v",
    "Usage-based",
    "/playground?model=seedance-2.0-i2v",
    false,
  ],
  [
    "v1.3",
    "Veo 3.1",
    "Text to Video",
    "google · t2v + audio",
    "Coming soon",
    "/playground?model=veo-3.1",
    false,
  ],
] as const;

const upcomingVideoModels = [
  ["sora-2", "openai · t2v"],
  ["kling-2.5-pro", "kuaishou · i2v"],
  ["wan-2.7", "alibaba · t2v"],
  ["hailuo-02", "minimax · i2v"],
  ["pixverse-v6", "pixverse · t2v"],
  ["seedance-2.0-lite", "bytedance · t2v"],
  ["ltx-2.3", "lightricks · i2v"],
] as const;

const modelWallRows = [
  modelTiles,
  [
    [
      "seedance-2.5",
      "bytedance · video · t2v + i2v",
      "Early access",
      true,
      false,
    ],
    ["glm-5.2", "z.ai", "$0.56/M", false, false],
    ["claude-sonnet-5", "anthropic · 1M", "$1.33/M", false, true],
    ["gpt-5.4-mini", "openai · 400K", "$0.50/M", false, true],
    ["gemini-3-flash", "google", "$0.047/M", false, false],
    ["deepseek-v4-flash", "deepseek", "$0.056/M", false, false],
    ["qwen3.7-max", "alibaba", "$1.00/M", false, false],
  ],
  [
    ["flatkey-auto", "routes itself", "no routing fee", true, false],
    ["minimax-m3", "minimax", "$0.18/M", false, false],
    ["grok-4.2", "xai", "$1.80/M", false, false],
    ["step-3.7-flash", "stepfun", "$0.16/M", false, false],
    ["hunyuan-2.0", "tencent", "$0.40/M", false, false],
    ["mimo-v2.5", "xiaomi", "$0.01/M", false, false],
  ],
] as const;

function createPixelRandom(initialSeed: number) {
  let seed = initialSeed;
  return function rnd() {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff;
    return seed / 0x7fffffff;
  };
}

function PixelGrid(props: {
  accent?: string;
  cell: number;
  colors?: string[];
  cols: number;
  n: number;
  rows: number;
  seed: number;
  style?: CSSProperties;
}) {
  const colors = props.colors ?? ["#DDD1F6", "#C4B5FD", "#A78BFA"];
  const accent = props.accent ?? "#15803D";
  const rnd = createPixelRandom(props.seed);

  const used = new Set<string>();
  const pixels = [];
  for (let index = 0; index < props.n; index += 1) {
    let x = 0;
    let y = 0;
    let key = "";
    let tries = 0;
    do {
      x = Math.floor(rnd() * props.cols);
      y = Math.floor(rnd() * props.rows);
      key = `${x}_${y}`;
      tries += 1;
    } while (used.has(key) && tries < 40);
    used.add(key);

    const background =
      rnd() < 0.07 ? accent : colors[Math.floor(rnd() * colors.length)];
    pixels.push(
      <i
        key={`${key}-${index}`}
        style={{
          animationDelay: `${(rnd() * 7).toFixed(2)}s`,
          animationDuration: `${(5 + rnd() * 7).toFixed(2)}s`,
          background,
          height: props.cell,
          left: x * props.cell,
          top: y * props.cell,
          width: props.cell,
        }}
      />,
    );
  }

  return (
    <div
      className="pxgrid"
      style={{
        height: props.rows * props.cell,
        width: props.cols * props.cell,
        ...props.style,
      }}
    >
      {pixels}
    </div>
  );
}

function renderLineBreakText(value: string) {
  return value.split(/<br\s*\/?>/i).map((part, index) => (
    <span key={`${part}-${index}`}>
      {index > 0 && <br />}
      {part}
    </span>
  ));
}

function renderHeroTitle(locale: Locale, fallback: ReactNode) {
  if (locale === "en") {
    return (
      <>
        <span className="hero-title-lead">
          One key
          <img
            className="hero-title-mark"
            src="/assets/flatkey-mark.svg"
            alt=""
            aria-hidden="true"
          />
        </span>
        <br />
        <span className="hero-title-phrase">
          More <span className="hero-highlight hero-highlight-models">models</span>, more tools, <span className="hero-highlight hero-highlight-cost">lower cost</span>
        </span>
      </>
    );
  }
  const value = getOnlineStaticText(locale, "hero.h1", "");
  if (!value) return fallback;
  const match = value.match(
    /^(.*?)<br><span class="price">(.*?)<span class="toolLine">(.*?)<\/span><span class="costLine">(.*?)<\/span><\/span>$/,
  );
  if (!match) return renderLineBreakText(value.replace(/<[^>]+>/g, ""));
  const separator = locale === "zh" ? "，" : ", ";
  return (
    <>
      <span className="hero-title-lead">
        {match[1].trim()}
        <img
          className="hero-title-mark"
          src="/assets/flatkey-mark.svg"
          alt=""
          aria-hidden="true"
        />
      </span>
      <br />
      <span className="hero-title-phrase">
        <span className="hero-highlight hero-highlight-models">{match[2].trim()}</span>
        {separator}
        {match[3].trim()}
        {separator}
        <span className="hero-highlight hero-highlight-cost">{match[4].trim()}</span>
      </span>
    </>
  );
}

function renderHeroSub(locale: Locale, value: string) {
  if (locale !== "en") return value;
  const breakAt = value.indexOf(" subscriptions");
  if (breakAt < 0) return value;
  return (
    <>
      {value.slice(0, breakAt)}
      <br className="hero-sub-break" />
      {value.slice(breakAt + 1)}
    </>
  );
}

const FEATURED_MODEL_STRIP = [
  ["deepseek.svg", "deepseek-v4-pro", "DeepSeek · reasoning model"],
  ["openai.svg", "openai/gpt-5.6-sol", "GPT · frontier model"],
  ["bytedance.svg", "seedance-2.5", "ByteDance · video model"],
  ["zai.svg", "glm-5.3", "GLM · coding model"],
  ["claude.svg", "claude-opus-5", "Claude · reasoning model"],
] as const;

const IMAGE_SELECTED_MODEL = "gpt-image-2";
const VIDEO_SELECTED_MODEL = "seedance-2.5";
const VIDEO_RESULT_ASSET = {
  poster: "https://cdn.shulex-voc.com/flatkey/model-examples/seedance-f1-wet-track.png",
  video: "https://cdn.shulex-voc.com/flatkey/model-examples/seedance-f1-wet-track.mp4",
} as const;

type IntelligenceCopy = {
  title: string;
  eyebrow: string;
  tabs: [string, string, string];
  promptTitle: string;
  promptBody: string;
  attachment: string;
  actionTitle: string;
  checklist: [string, string, string, string];
  recordsLabel: string;
  recordsValue: string;
  processedLabel: string;
  processedValue: string;
  costLabel: string;
  costValue: string;
  footer: string;
};

const INTELLIGENCE_COPY: Record<Locale, IntelligenceCopy> = {
  en: {
    title: "One key, multi model run",
    eyebrow: "Text Intelligence",
    tabs: ["Text Intelligence", "Image Generation", "Video generation"],
    promptTitle: "Test Prompt",
    promptBody: "Analyze the data and source material, identify key patterns and trends, summarize the findings, and turn them into clear recommendations.",
    attachment: "Attach source material",
    actionTitle: "AI analysis in action",
    checklist: ["Structured data analyzed", "Key patterns and trends identified", "Long-form text summarized", "Insights turned into recommendations"],
    recordsLabel: "Records analyzed",
    recordsValue: "1,284",
    processedLabel: "Text processed",
    processedValue: "4.8 s",
    costLabel: "Total cost",
    costValue: "$0.02",
    footer: "One key · More intelligence · Lower cost",
  },
  zh: {
    title: "一个 key，多模型运行",
    eyebrow: "文本智能",
    tabs: ["文本智能", "图片生成", "视频生成"],
    promptTitle: "测试提示词",
    promptBody: "分析数据和源材料，识别关键模式与趋势，总结发现，并将其转化为清晰的建议。",
    attachment: "上传源材料",
    actionTitle: "AI 分析进行中",
    checklist: ["已分析结构化数据", "已识别关键模式与趋势", "已总结长篇文本", "已将洞察转化为建议"],
    recordsLabel: "已分析记录",
    recordsValue: "1,284",
    processedLabel: "文本处理耗时",
    processedValue: "4.8 秒",
    costLabel: "总成本",
    costValue: "$0.02",
    footer: "一个 key · 更多智能 · 更低成本",
  },
  es: {
    title: "Una key, múltiples modelos",
    eyebrow: "Inteligencia de texto",
    tabs: ["Inteligencia de texto", "Generación de imágenes", "Generación de vídeo"],
    promptTitle: "Prompt de prueba",
    promptBody: "Analiza los datos y el material de origen, identifica patrones y tendencias clave, resume los hallazgos y conviértelos en recomendaciones claras.",
    attachment: "Adjuntar material de origen",
    actionTitle: "Análisis de IA en acción",
    checklist: ["Datos estructurados analizados", "Patrones y tendencias identificados", "Texto extenso resumido", "Insights convertidos en recomendaciones"],
    recordsLabel: "Registros analizados",
    recordsValue: "1.284",
    processedLabel: "Texto procesado",
    processedValue: "4,8 s",
    costLabel: "Coste total",
    costValue: "$0,02",
    footer: "Una key · Más inteligencia · Menor coste",
  },
  fr: {
    title: "Une key, plusieurs modèles",
    eyebrow: "Intelligence textuelle",
    tabs: ["Intelligence textuelle", "Génération d’images", "Génération vidéo"],
    promptTitle: "Prompt de test",
    promptBody: "Analysez les données et les sources, identifiez les tendances et schémas clés, résumez les résultats et transformez-les en recommandations claires.",
    attachment: "Joindre les sources",
    actionTitle: "Analyse IA en action",
    checklist: ["Données structurées analysées", "Schémas et tendances identifiés", "Texte long résumé", "Insights transformés en recommandations"],
    recordsLabel: "Enregistrements analysés",
    recordsValue: "1 284",
    processedLabel: "Texte traité",
    processedValue: "4,8 s",
    costLabel: "Coût total",
    costValue: "$0,02",
    footer: "Une key · Plus d’intelligence · Moindre coût",
  },
  pt: {
    title: "Uma key, vários modelos",
    eyebrow: "Inteligência de texto",
    tabs: ["Inteligência de texto", "Geração de imagens", "Geração de vídeo"],
    promptTitle: "Prompt de teste",
    promptBody: "Analise os dados e as fontes, identifique padrões e tendências, resuma as descobertas e transforme tudo em recomendações claras.",
    attachment: "Anexar fontes",
    actionTitle: "Análise de IA em ação",
    checklist: ["Dados estruturados analisados", "Padrões e tendências identificados", "Texto longo resumido", "Insights transformados em recomendações"],
    recordsLabel: "Registros analisados",
    recordsValue: "1.284",
    processedLabel: "Texto processado",
    processedValue: "4,8 s",
    costLabel: "Custo total",
    costValue: "$0,02",
    footer: "Uma key · Mais inteligência · Menor custo",
  },
  ru: {
    title: "Один key, несколько моделей",
    eyebrow: "Интеллект для текста",
    tabs: ["Интеллект для текста", "Генерация изображений", "Генерация видео"],
    promptTitle: "Тестовый промпт",
    promptBody: "Проанализируйте данные и исходные материалы, найдите ключевые закономерности и тренды, обобщите результаты и превратите их в понятные рекомендации.",
    attachment: "Прикрепить материалы",
    actionTitle: "ИИ-анализ в действии",
    checklist: ["Структурированные данные проанализированы", "Ключевые закономерности и тренды найдены", "Длинный текст обобщён", "Инсайты превращены в рекомендации"],
    recordsLabel: "Записей проанализировано",
    recordsValue: "1 284",
    processedLabel: "Обработано текста",
    processedValue: "4,8 с",
    costLabel: "Общая стоимость",
    costValue: "$0,02",
    footer: "Один key · Больше интеллекта · Меньше затрат",
  },
  ja: {
    title: "1 つの key、複数モデルを実行",
    eyebrow: "テキストインテリジェンス",
    tabs: ["テキストインテリジェンス", "画像生成", "動画生成"],
    promptTitle: "テストプロンプト",
    promptBody: "データとソース資料を分析し、重要なパターンと傾向を特定して、結果を要約し、明確な提案に変換します。",
    attachment: "ソース資料を添付",
    actionTitle: "AI 分析を実行中",
    checklist: ["構造化データを分析", "重要なパターンと傾向を特定", "長文を要約", "インサイトを提案に変換"],
    recordsLabel: "分析したレコード",
    recordsValue: "1,284",
    processedLabel: "処理時間",
    processedValue: "4.8 秒",
    costLabel: "合計コスト",
    costValue: "$0.02",
    footer: "1 つの key · より賢く · より低コスト",
  },
  vi: {
    title: "Một key, chạy nhiều model",
    eyebrow: "Trí tuệ văn bản",
    tabs: ["Trí tuệ văn bản", "Tạo hình ảnh", "Tạo video"],
    promptTitle: "Prompt thử nghiệm",
    promptBody: "Phân tích dữ liệu và tài liệu nguồn, xác định các mẫu và xu hướng chính, tóm tắt phát hiện rồi chuyển thành đề xuất rõ ràng.",
    attachment: "Đính kèm tài liệu nguồn",
    actionTitle: "AI phân tích trong thực tế",
    checklist: ["Đã phân tích dữ liệu có cấu trúc", "Đã xác định mẫu và xu hướng chính", "Đã tóm tắt văn bản dài", "Đã chuyển insight thành đề xuất"],
    recordsLabel: "Bản ghi đã phân tích",
    recordsValue: "1.284",
    processedLabel: "Văn bản đã xử lý",
    processedValue: "4,8 giây",
    costLabel: "Tổng chi phí",
    costValue: "$0,02",
    footer: "Một key · Nhiều trí tuệ hơn · Chi phí thấp hơn",
  },
  de: {
    title: "Ein Key, mehrere Modelle",
    eyebrow: "Textintelligenz",
    tabs: ["Textintelligenz", "Bildgenerierung", "Videogenerierung"],
    promptTitle: "Test-Prompt",
    promptBody: "Analysiere Daten und Quellen, erkenne wichtige Muster und Trends, fasse die Ergebnisse zusammen und mache daraus klare Empfehlungen.",
    attachment: "Quellen anhängen",
    actionTitle: "KI-Analyse in Aktion",
    checklist: ["Strukturierte Daten analysiert", "Wichtige Muster und Trends erkannt", "Langer Text zusammengefasst", "Erkenntnisse in Empfehlungen verwandelt"],
    recordsLabel: "Analysierte Datensätze",
    recordsValue: "1.284",
    processedLabel: "Verarbeiteter Text",
    processedValue: "4,8 s",
    costLabel: "Gesamtkosten",
    costValue: "$0,02",
    footer: "Ein Key · Mehr Intelligenz · Geringere Kosten",
  },
  id: {
    title: "Satu key, banyak model",
    eyebrow: "Intelijen teks",
    tabs: ["Intelijen teks", "Pembuatan gambar", "Pembuatan video"],
    promptTitle: "Prompt uji",
    promptBody: "Analisis data dan materi sumber, temukan pola serta tren utama, rangkum hasilnya, lalu ubah menjadi rekomendasi yang jelas.",
    attachment: "Lampirkan materi sumber",
    actionTitle: "Analisis AI sedang berjalan",
    checklist: ["Data terstruktur dianalisis", "Pola dan tren utama ditemukan", "Teks panjang diringkas", "Insight diubah menjadi rekomendasi"],
    recordsLabel: "Catatan dianalisis",
    recordsValue: "1.284",
    processedLabel: "Teks diproses",
    processedValue: "4,8 dtk",
    costLabel: "Total biaya",
    costValue: "$0,02",
    footer: "Satu key · Lebih cerdas · Biaya lebih rendah",
  },
};

function renderToolsTitle(locale: Locale, fallback: ReactNode) {
  const value = getOnlineHomeToolText(locale, "tools.intro.title", "");
  if (!value) return fallback;
  const match = value.match(
    /^<span class="tools-title-line">(.*?)<\/span><br>(.*)$/,
  );
  if (!match) return renderLineBreakText(value.replace(/<[^>]+>/g, ""));
  return (
    <>
      <span className="tools-title-line">{match[1]}</span>
      <br />
      {match[2]}
    </>
  );
}

function renderEmStart(value: string) {
  const match = value.match(/^<em>(.*?)<\/em>(.*)$/);
  if (!match) return value;
  return (
    <>
      <em>{match[1]}</em>
      {match[2]}
    </>
  );
}

function renderInlineEm(value: string) {
  const match = value.match(/^(.*?)<em>(.*?)<\/em>(.*)$/);
  if (!match) return renderLineBreakText(value);
  return (
    <>
      {renderLineBreakText(match[1])}
      <em>{match[2]}</em>
      {renderLineBreakText(match[3])}
    </>
  );
}

type OnlineHomePageProps = {
  /** A browser hint is not a verified console session; CTA auth state is verified client-side. */
  hasConsoleSessionHint?: boolean;
  locale: Locale;
};

export async function OnlineHomePage(props: OnlineHomePageProps) {
  const copy = getOnlineStaticCopy(props.locale);
  const home = getHomeCopy(props.locale);
  const t = (key: string, fallback: string) =>
    getOnlineStaticText(props.locale, key, fallback);
  const ht = (key: string, fallback: string) =>
    getOnlineHomeToolText(props.locale, key, fallback);
  const videoStatus = (status: string) => {
    if (status === "Early access") return status;
    if (status === "Usage-based") return t("md.usage", status);
    if (status === "Coming soon") return t("md.soon", status);
    return status;
  };
  const modelPrice = (price: string) =>
    price === "Early access" ? t("md.early", price) : price;
  const authActionHref = consoleUrl("/sign-up", `lng=${props.locale}`);
  // The free-credits CTA is the fast path to a first API call, so land on the console
  // dashboard overview once the visitor is authenticated -- it carries the key picker
  // and the copy-paste integration examples in one screen (the console honors
  // ?redirect= after sign-up, after switching to sign-in, and skips the form entirely
  // when already logged in).
  const overviewActionHref = consoleUrl(
    "/sign-up",
    `redirect=${encodeURIComponent("/dashboard/overview")}&lng=${props.locale}`,
  );
  const authActionLabel = copy.home.ctaKey;
  const finalCtaLabel = t("cta.b1", "Get started");

  return (
    <OnlineStaticShell locale={props.locale} pathname="/">
      <style>{`
        .online-static-page:has(> header.hero.heroUnified) .heroUnified h1.display{max-width:min(100%,720px);font-size:58px;line-height:1.04;letter-spacing:0;overflow-wrap:anywhere;text-wrap:balance}
        .online-static-page:has(> header.hero.heroUnified) .heroUnified h1 .price{display:block;max-width:100%;overflow-wrap:anywhere}
        .online-static-page:has(> header.hero.heroUnified) .heroUnified .heroCopy{min-width:0}
        .online-static-page:has(> header.hero.heroUnified) .heroUnified .saveRow small{white-space:normal;text-align:right;line-height:1.25}
        @media(max-width:1000px){.online-static-page:has(> header.hero.heroUnified) .heroUnified h1.display{font-size:52px;max-width:760px}}
        @media(max-width:620px){.online-static-page:has(> header.hero.heroUnified) .heroUnified h1.display{font-size:38px;line-height:1.06;letter-spacing:0}.online-static-page:has(> header.hero.heroUnified) .heroUnified h1 .price{margin-top:8px}.online-static-page:has(> header.hero.heroUnified) .heroUnified .heroCtas .btn{white-space:normal;text-align:center}}
        @media(max-width:380px){.online-static-page:has(> header.hero.heroUnified) .heroUnified h1.display{font-size:34px}}
        .compactHero{min-height:540px!important;background:#fff!important;border-bottom:0!important;color:#09090b!important}
        .compactHero .heroGrid{display:block!important;max-width:none!important;padding:98px 24px 86px!important;text-align:center!important}
        .compactHero .heroCopy{max-width:1280px!important;margin:0 auto!important;align-items:center!important}
        .compactHero .eyebrow,.compactHero .heroSavings{display:none!important}
        .compactHero h1.display{max-width:none!important;margin:0!important;color:#050505!important;font-size:clamp(48px,5.2vw,80px)!important;line-height:1.12!important;letter-spacing:-.055em!important;font-weight:650!important;text-wrap:balance!important}
        .compactHero .hero-title-lead{display:inline-flex;align-items:center;justify-content:center;gap:18px;white-space:nowrap}
        .compactHero .hero-title-mark{width:48px;height:48px;object-fit:contain;vertical-align:middle}
        .compactHero .hero-title-phrase{display:inline-block;margin-top:8px;white-space:nowrap}
        .compactHero:not(.compactHero-en) h1.display{max-width:min(100%,1600px)!important;font-size:clamp(40px,3.7vw,64px)!important;letter-spacing:-.04em!important}
        .compactHero:not(.compactHero-en) .hero-title-phrase{max-width:100%;white-space:normal;overflow-wrap:break-word;word-break:normal}
        .compactHero .hero-highlight{display:inline;padding:0 .08em .03em}
        .compactHero .hero-highlight-models{background:linear-gradient(180deg,transparent 26%,#e7d8ff 26%,#e7d8ff 88%,transparent 88%)}
        .compactHero .hero-highlight-cost{background:linear-gradient(180deg,transparent 72%,#fff58a 72%,#fff58a 94%,transparent 94%)}
        .compactHero .sub{max-width:1200px!important;margin:22px auto 0!important;color:#202124!important;font-size:20px!important;line-height:1.5!important;text-wrap:balance}
        .compactHero .hero-sub-break{display:block}
        .compactHero .heroCtas{justify-content:center!important;gap:16px!important;margin-top:44px!important}
        .compactHero .heroCtas .btn{min-width:0!important;min-height:58px!important;padding:0 30px!important;border-radius:999px!important;font-size:17px!important;font-weight:650!important;letter-spacing:-.01em}
        .compactHero .heroPrimary{background:#050505!important;color:#fff!important;box-shadow:none!important}
        .compactHero .heroPrimary::after{content:"↗";display:inline-block;margin-left:10px;font-size:1.15em;line-height:1;transform:translateY(-1px)}
        .compactHero .heroSecondary{border:1.5px solid #444!important;background:#fff!important;color:#111!important;box-shadow:none!important}
        .modelStrip{background:#f5f6f7;border-top:1px solid #f0f1f2;border-bottom:1px solid #eceef0}
        .modelStripInner{min-height:84px;padding:0 300px}
        .modelStripCarousel{display:block}
        .modelStripViewport{overflow:hidden}
        .modelStripTrack{display:flex;width:max-content;align-items:center;animation:modelStripMarquee 32s linear infinite;will-change:transform}
        .modelStripSet{display:flex;flex:none;align-items:center}
        .modelStripSetClone{display:flex}
        .modelStripItem{display:flex;align-items:center;justify-content:center;gap:14px;width:250px;min-width:0;height:84px;flex:none;padding:0}
        .modelStripItem img{width:28px;height:28px;flex:none;object-fit:contain}
        .modelStripItem div{min-width:0}
        .modelStripItem strong{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#27292d;font-size:15px;font-weight:500;line-height:1.25;letter-spacing:-.01em}
        .modelStripItem span{display:block;margin-top:5px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#92969c;font-size:13px;line-height:1.2}
        .modelStripDots{display:none}
        @keyframes modelStripMarquee{from{transform:translateX(0)}to{transform:translateX(-50%)}}
        @media(max-width:1200px){.modelStripInner{padding:0 80px}}
        @media(max-width:1100px){.compactHero h1.display{font-size:clamp(44px,6.3vw,68px)!important}.compactHero:not(.compactHero-en) h1.display{font-size:clamp(38px,5.2vw,58px)!important}.modelStripItem{gap:10px}.modelStripItem strong{font-size:13px}.modelStripItem span{font-size:11px}}
        @media(max-width:700px){.compactHero{min-height:auto!important}.compactHero .heroGrid{padding:78px 20px 68px!important}.compactHero h1.display{font-size:clamp(38px,10.5vw,58px)!important;line-height:1.08!important}.compactHero:not(.compactHero-en) h1.display{max-width:100%!important;font-size:clamp(32px,9.4vw,48px)!important;letter-spacing:-.035em!important}.compactHero .hero-title-lead{gap:10px}.compactHero .hero-title-mark{width:34px;height:34px}.compactHero .hero-title-phrase{margin-top:12px;white-space:normal}.compactHero .sub{font-size:15px!important;max-width:100%!important;overflow-wrap:anywhere}.compactHero .hero-sub-break{display:none}.compactHero .heroCtas{width:100%;flex-direction:column;margin-top:32px!important}.compactHero .heroCtas .btn{width:100%;font-size:15px!important}.modelStripInner{padding:0 20px}.modelStripTrack{animation-duration:28s;touch-action:pan-x}.modelStripItem{width:220px}.modelStripItem strong{font-size:13px}}
        @media(prefers-reduced-motion:reduce){.modelStripTrack{animation:none}}
        .intelligence-section{background:#fff;padding:92px var(--fk-site-gutter) 100px;border-bottom:1px solid #eeeaf6}
        .tools-intro{display:none!important}
        .intelligence-wrap{max-width:1408px;margin:0 auto}
        .intelligence-heading{text-align:center}
        .intelligence-heading h2{margin:0;font-family:var(--disp);font-size:clamp(44px,5vw,66px);line-height:1.05;letter-spacing:-.055em;color:#09090b;font-weight:700}
        .intelligence-heading p{margin:19px 0 0;color:#4a4650;font-size:18px;line-height:1.5}
        .intelligence-tabs{display:grid;grid-template-columns:repeat(3,1fr);gap:26px;margin-top:66px}
        .intelligence-tab-input{position:absolute;opacity:0;pointer-events:none}
        .intelligence-tab{display:flex;align-items:center;justify-content:center;gap:11px;padding:0 0 19px;border-bottom:4px solid #eee8fb;color:#5a565f;font-size:17px;font-weight:600;cursor:pointer;transition:color .2s,border-color .2s}
        .intelligence-tab-input:checked + .intelligence-tab{border-color:#7538f2;color:#26232a}
        .intelligence-tab svg{display:block;flex:none;width:auto;height:auto;stroke:none;fill:none}
        .intelligence-panel{--intelligence-card-gap:clamp(32px,calc((100vw - var(--fk-site-gutter) - var(--fk-site-gutter) - 1187px)/2),111px);--intelligence-connector-width:var(--intelligence-card-gap);--intelligence-panel-pad-right:52px;--intelligence-model-column:320px;position:relative;display:grid;grid-column:1/-1;grid-template-columns:320px minmax(0,443px) var(--intelligence-model-column);justify-content:space-between;gap:0;min-height:614px;margin-top:29px;padding:34px 52px 38px;border-radius:28px;background:radial-gradient(circle at 18% 12%,rgba(235,222,255,.86),transparent 43%),linear-gradient(135deg,#f2eaff 0%,#f8f1ff 43%,#fffafc 100%);overflow:hidden}
        .intelligence-panel-image,.intelligence-panel-video{display:none;background:linear-gradient(180deg,rgba(202,179,253,.4) 0%,rgba(251,224,240,.4) 50%,rgba(250,248,249,.4) 100%)}
        .intelligence-panel-media{--intelligence-card-gap:clamp(32px,calc((100vw - var(--fk-site-gutter) - var(--fk-site-gutter) - 1179px)/2),115px);--intelligence-connector-width:var(--intelligence-card-gap);--intelligence-panel-pad-right:48px;min-height:575px;padding:32px 48px;border-radius:24px}
        .intelligence-wrap:has(#intelligence-tab-1:checked) .intelligence-panel-text,.intelligence-wrap:has(#intelligence-tab-2:checked) .intelligence-panel-text{display:none}
        .intelligence-wrap:has(#intelligence-tab-1:checked) .intelligence-panel-image,.intelligence-wrap:has(#intelligence-tab-2:checked) .intelligence-panel-video{display:grid}
        .intelligence-panel:before{content:"";position:absolute;inset:0;background-image:linear-gradient(90deg,rgba(137,89,227,.07) 1px,transparent 1px),linear-gradient(rgba(137,89,227,.07) 1px,transparent 1px);background-size:72px 72px;mask-image:linear-gradient(135deg,rgba(0,0,0,.62),transparent 70%);pointer-events:none}
        .intelligence-panel:after{display:none}
        .intelligence-prompt,.intelligence-analysis{position:relative;z-index:1}
        .intelligence-prompt{align-self:center;min-height:454px;border:1px solid #e4dfe8;border-radius:14px;background:#fff;box-shadow:0 18px 35px -27px rgba(55,32,76,.35);overflow:visible}
        .intelligence-card-head{display:flex;align-items:center;gap:11px;padding:20px 24px 17px;border-bottom:1px solid #ece9ef;font-size:18px;font-weight:700;letter-spacing:-.025em}
        .intelligence-card-head span{display:grid;place-items:center;width:22px;height:22px;color:#bf3ee8;font-size:18px;line-height:1}
        .pixel-mark{position:relative;display:block!important;width:20px!important;height:20px!important;background:radial-gradient(circle,#c84ae9 2.5px,transparent 3px);background-size:7px 7px;background-position:0 0}
        .intelligence-prompt-body{padding:23px 24px;color:#28242d;font-size:16px;line-height:1.55}
        .intelligence-attachment{display:flex;align-items:center;gap:8px;margin:28px 20px 20px;padding:13px 14px;border:1px dashed #d8d0e4;border-radius:11px;color:#8d8498;font-size:13px;background:#fcfbfd}
        .intelligence-attachment span{display:grid;place-items:center;width:23px;height:23px;border-radius:6px;background:#f0e9ff;color:#8c46dd;font-size:18px;font-weight:600}
        .intelligence-analysis{align-self:center;max-width:760px;padding:18px 0 0}
        .intelligence-analysis h3{margin:0;font-family:var(--disp);font-size:31px;line-height:1.1;letter-spacing:-.045em;color:#1d1824}
        .intelligence-checklist{display:grid;gap:0;margin:25px 0 0;padding:0;list-style:none;border:1px solid rgba(196,181,213,.56);border-radius:13px;background:rgba(255,255,255,.62);overflow:hidden}
        .intelligence-checklist li{display:flex;align-items:center;gap:13px;padding:16px 19px;border-bottom:1px solid rgba(219,211,226,.72);color:#37323c;font-size:16px;line-height:1.3}
        .intelligence-checklist li:last-child{border-bottom:0}
        .intelligence-checklist li:before{content:"✓";display:grid;place-items:center;width:22px;height:22px;flex:none;border-radius:50%;background:#d8f7dc;color:#20a94b;font-size:14px;font-weight:800}
        .intelligence-metrics{display:grid;grid-template-columns:repeat(3,1fr);gap:11px;margin-top:14px}
        .intelligence-metric{padding:17px 16px 15px;border-radius:11px;background:rgba(255,255,255,.74);box-shadow:0 9px 24px -24px rgba(49,24,65,.5)}
        .intelligence-metric span{display:block;color:#88808d;font-size:12px;line-height:1.2}
        .intelligence-metric strong{display:block;margin-top:8px;color:#29222f;font-family:var(--mono);font-size:21px;line-height:1.1;letter-spacing:-.04em}
        .intelligence-footer{margin:24px 0 0;text-align:center;color:#7551bb;font-size:15px;font-weight:650;letter-spacing:.01em}
        .intelligence-analysis{display:none!important}
        .intelligence-criteria{position:relative;z-index:1;align-self:center;border:1px solid #e6e0e9;border-radius:18px;background:#fff;box-shadow:0 18px 35px -27px rgba(55,32,76,.35);overflow:hidden}
        .criteria-head{display:flex;align-items:center;gap:12px;padding:22px 25px 18px;border-bottom:1px solid #ece9ef;font-size:20px;font-weight:700;letter-spacing:-.025em}
        .criteria-head .criteria-icon{display:block;width:25px;height:25px;background:radial-gradient(circle,#44bc68 3px,transparent 3.5px);background-size:8px 8px;background-position:0 0}
        .criteria-head .criteria-shield{margin-left:auto;width:27px;height:27px;object-fit:contain}
        .criteria-row{padding:19px 25px;border-bottom:1px solid #ece9ef;color:#2d2931;font-size:16px;line-height:1.35}
        .criteria-row.success{color:#37b968;font-weight:700}
        .criteria-metrics{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;padding:18px 25px;border-bottom:1px solid #ece9ef}
        .criteria-metric{min-height:91px;padding:17px 16px;border-radius:14px;background:#f7f5f8}
        .criteria-metric span{display:block;color:#88818d;font-size:13px;line-height:1.25}
        .criteria-metric strong{display:block;margin-top:10px;color:#241f28;font-family:var(--mono);font-size:19px;line-height:1.1;letter-spacing:-.04em}
        .criteria-pay{margin:17px 25px 20px;padding:13px 15px;border-radius:9px;background:#9c65ec;color:#fff;text-align:center;font-size:16px;font-weight:700;line-height:1.25}
        .intelligence-media-card{position:relative;z-index:1;align-self:center;border:1px solid #e6e0e9;border-radius:18px;background:#fff;box-shadow:0 18px 35px -27px rgba(55,32,76,.35);overflow:hidden}
        .intelligence-media-body{padding:17px 25px 0}
        .intelligence-result-media{display:block;width:100%;aspect-ratio:1.975/1;border-radius:14px;object-fit:cover}
        .media-metrics{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;padding:18px 0}
        .media-metric{min-height:91px;padding:17px 16px;border-radius:14px;background:#f7f5f8}
        .media-metric span{display:block;color:#88818d;font-size:13px;line-height:1.25}
        .media-metric strong{display:block;margin-top:10px;color:#241f28;font-family:var(--mono);font-size:19px;line-height:1.1;letter-spacing:-.04em}
        .media-pay{padding:0 25px 20px;border-top:1px solid #ece9ef}
        .media-pay div{margin-top:17px;padding:13px 15px;border-radius:9px;background:#9c65ec;color:#fff;text-align:center;font-size:16px;font-weight:700;line-height:1.25}
        .prompt-reference{display:block;width:60px;height:60px;margin:25px 24px 0;border-radius:10px;object-fit:cover}
        .intelligence-panel-media .intelligence-prompt-body{padding-top:14px;font-size:17px;line-height:1.52}
        .intelligence-panel-media .intelligence-models{gap:20px}
        .intelligence-panel-media .model-card{min-height:70px}
        .intelligence-panel-media .model-selected-label{margin:0 0 -10px}
        .intelligence-models{position:relative;z-index:1;align-self:center;display:grid;gap:24px;min-width:0}
        .model-card{min-width:0;min-height:74px;display:flex;align-items:center;gap:14px;padding:15px 18px;border:1px solid rgba(225,218,232,.9);border-radius:13px;background:rgba(255,255,255,.78);box-shadow:0 13px 28px -26px rgba(55,32,76,.38)}
        .model-card.selected{min-height:76px;background:#fff;border-color:#e3dce8;box-shadow:0 16px 30px -23px rgba(55,32,76,.4)}
        .model-card img{width:32px;height:32px;flex:none;object-fit:contain}
        .model-card-copy{min-width:0}
        .model-card-copy strong{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#34303a;font-size:16px;line-height:1.25}
        .model-card-copy span{display:block;margin-top:4px;color:#9a939e;font-size:13px;line-height:1.2}
        .model-card-price{min-width:max-content;max-width:none;margin-left:auto;flex:none;overflow:visible;text-overflow:clip;padding:8px 10px;border-radius:10px;background:#f5f3f6;color:#8c8790;font-size:12px;white-space:nowrap}
        .model-selected-label{margin:2px 0 -12px;color:#3bb969;font-size:14px;font-weight:600}
        .model-placeholder{width:34px;height:34px;display:grid;place-items:center;flex:none;border-radius:50%;background:#e7e5e8;color:#818087;font-size:22px;font-weight:700}
        .intelligence-connectors{position:absolute;z-index:2;top:calc(50% + 16px);right:calc(var(--intelligence-panel-pad-right) + var(--intelligence-model-column));width:var(--intelligence-connector-width);height:200px;transform:translateY(-50%);pointer-events:none;overflow:visible}
        .intelligence-connectors line,.intelligence-connectors path{fill:none;stroke-width:1.5;vector-effect:non-scaling-stroke;stroke-linecap:round;stroke-linejoin:round}
        .intelligence-prompt{min-height:454px;display:flex;flex-direction:column}
        .intelligence-prompt:after{content:"";position:absolute;z-index:2;left:100%;top:50%;width:var(--intelligence-card-gap);height:1px;background:#b79af0;pointer-events:none}
        .intelligence-prompt-body{max-width:290px}
        .intelligence-prompt-input{display:flex;align-items:center;gap:11px;margin:auto 16px 17px;padding:0 10px;height:39px;border:1px solid #e7e4e9;border-radius:999px}
        .intelligence-prompt-input span{flex:1}
        .intelligence-prompt-input b{display:grid;place-items:center;width:30px;height:30px;border-radius:50%;background:#d4d4d5;color:#fff;font-size:20px;line-height:1}
        @media(max-width:1450px) and (min-width:901px){.intelligence-panel{--intelligence-card-gap:clamp(14px,2.5vw,32px);--intelligence-connector-width:var(--intelligence-card-gap);--intelligence-panel-pad-right:clamp(24px,4vw,52px);--intelligence-model-column:clamp(220px,28vw,320px);grid-template-columns:var(--intelligence-model-column) minmax(0,1fr) var(--intelligence-model-column);gap:var(--intelligence-card-gap);padding-inline:var(--intelligence-panel-pad-right)}.intelligence-panel-media{--intelligence-panel-pad-right:clamp(24px,4vw,48px);padding-inline:var(--intelligence-panel-pad-right)}.model-card-price{min-width:0;max-width:44%;flex:0 1 auto;overflow:hidden;text-overflow:ellipsis;padding-inline:8px;font-size:10px}}
        @media(max-width:1100px) and (min-width:901px){.intelligence-panel{--intelligence-card-gap:16px;--intelligence-panel-pad-right:24px;--intelligence-model-column:clamp(188px,22vw,240px);padding-inline:24px}.intelligence-panel-media{--intelligence-panel-pad-right:20px;padding-inline:20px}.intelligence-panel-media .model-card-price{display:none}}
        @media(max-width:900px){.intelligence-section{padding:72px 20px 78px}.intelligence-tabs{gap:10px;margin-top:47px}.intelligence-tab{font-size:14px}.intelligence-panel{grid-template-columns:1fr;gap:25px;min-height:0;padding:26px 24px 30px}.intelligence-panel:after{display:none}.intelligence-prompt{min-height:0}.intelligence-prompt:after{display:none}.intelligence-prompt-body{max-width:none}.intelligence-criteria,.intelligence-media-card,.intelligence-models{align-self:stretch;width:100%}.intelligence-connectors{display:none}}
        @media(max-width:560px){.intelligence-heading h2{font-size:42px}.intelligence-heading p{font-size:16px}.intelligence-tabs{grid-template-columns:1fr;gap:8px}.intelligence-tab{justify-content:flex-start;padding:13px 8px;border-bottom-width:2px}.intelligence-panel{margin-top:18px;padding:18px 14px 22px;border-radius:20px}.intelligence-card-head{padding:17px 18px 14px;font-size:16px}.intelligence-prompt-body{padding:19px 18px;font-size:15px}.intelligence-attachment{margin:23px 14px 14px}.intelligence-checklist li{padding:14px 14px;font-size:14px}.intelligence-metrics{grid-template-columns:1fr}.intelligence-metric{padding:14px}.intelligence-footer{font-size:13px}}
        @media(max-width:420px){.intelligence-heading h2{font-size:36px}.intelligence-heading p{font-size:14px;line-height:1.45}.intelligence-panel{gap:20px;padding-inline:12px}.intelligence-panel-media{padding-inline:12px}.criteria-metrics,.media-metrics{grid-template-columns:1fr;gap:10px}.criteria-metric,.media-metric{min-height:0;padding:13px 14px}.criteria-head{padding-inline:18px}.model-card{padding-inline:14px}.model-card-price{padding:7px 8px;font-size:11px}}
        .hero{background:linear-gradient(180deg,#FFFFFF 0%,#F7F5FD 100%);border-bottom:1px solid var(--line)}.heroGrid{max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:56px var(--fk-site-gutter) 52px;display:grid;grid-template-columns:minmax(0,1fr) 560px;gap:48px;align-items:start}.eyebrow{display:inline-flex;align-items:center;gap:8px;font-family:var(--mono);font-size:12px;letter-spacing:.6px;color:var(--violet-deep);background:var(--violet-tint);border-radius:999px;padding:7px 14px;font-weight:600;margin-bottom:22px}.hero h1 .price{color:var(--violet)}.hero h1 .toolLine,.hero h1 .costLine{display:block}.hero .sub{margin-top:18px;max-width:560px}.heroCtas{display:flex;gap:12px;margin-top:26px}.heroSavings{background:#fff;border:1px solid var(--line);border-radius:18px;padding:22px;box-shadow:0 24px 60px -18px rgba(46,16,101,.18)}.saveRow{display:flex;justify-content:space-between;gap:18px;border-bottom:1px solid var(--line2);padding:14px 0}.saveRow s{color:var(--ink3);font-weight:700}.saveRow small{font-family:var(--mono);color:var(--ink2)}.balanceCard{margin-top:18px;background:var(--violet-tint);border-radius:14px;padding:22px}.balanceCard span{font-family:var(--mono);font-size:11px;color:var(--violet-deep);font-weight:700;letter-spacing:.8px}.balanceCard strong{display:block;margin-top:8px;font-size:28px;letter-spacing:-1px}.balanceCard p{color:var(--ink2);margin-top:4px}.mini{margin-top:18px}.mbar{display:grid;grid-template-columns:66px 1fr 54px;align-items:center;gap:10px;font:600 11px/1 var(--mono);margin-top:10px}.mtrack{height:8px;border-radius:999px;background:#eee;overflow:hidden}.mfill{height:100%}.mcap{font-size:11px!important;margin-top:12px!important}.labs,.chips{display:flex;flex-wrap:wrap;gap:8px;margin-top:18px}.lab{width:72px;height:54px;border-radius:12px;color:#fff;display:grid;place-items:center;font-weight:800}.lab span{display:block;font:500 8px/1 var(--mono)}.chips span{background:#f4f0ff;border:1px solid #ded6f4;border-radius:999px;padding:8px 10px;font-size:12px}.wcode{margin-top:16px;background:#0d0d10;color:#eee;border-radius:12px;padding:16px;font:500 12px/1.65 var(--mono);white-space:pre-wrap}.wcode span{color:#67e8f9}.wcode em{font-style:normal;color:#c4b5fd}.home-cta-link{display:inline-block;margin-top:24px;color:#fff;font-weight:800}.vcard video{display:block}.home-section-actions{display:flex;gap:12px;margin-top:24px}.home-section-actions a{text-decoration:none}
        .price-proof{position:relative;overflow:hidden;background:#fff;border-bottom:1px solid var(--line)}.price-proof:before{content:"";position:absolute;inset:0;background:linear-gradient(to right,rgba(124,58,237,.055) 1px,transparent 1px),linear-gradient(to bottom,rgba(124,58,237,.05) 1px,transparent 1px);background-size:72px 72px;mask-image:linear-gradient(180deg,rgba(0,0,0,.78),transparent 86%);pointer-events:none}.price-proof-in{position:relative;z-index:1;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:76px var(--fk-site-gutter);display:grid;grid-template-columns:minmax(0,.86fr) minmax(420px,1fr);gap:56px;align-items:center}.price-proof-copy{max-width:520px}.price-proof-copy .kick2{margin-bottom:16px}.price-proof-copy h2{font-family:var(--disp);font-size:clamp(36px,4vw,58px);line-height:.98;letter-spacing:-.055em;font-weight:700;text-wrap:balance}.price-proof-copy p{margin-top:18px;color:var(--ink2);font-size:16.5px;line-height:1.7}.price-proof-list{display:grid;gap:10px;margin-top:26px}.price-proof-list li{list-style:none;display:flex;align-items:flex-start;gap:10px;color:var(--ink2);font-size:14px;line-height:1.55}.price-proof-list li:before{content:"";margin-top:7px;width:8px;height:8px;flex:none;border-radius:999px;background:linear-gradient(135deg,var(--violet-hi),#c026d3);box-shadow:0 0 0 4px rgba(124,58,237,.09)}.price-board{position:relative;border:1px solid rgba(76,29,149,.14);border-radius:20px;background:rgba(255,255,255,.94);box-shadow:0 34px 90px -56px rgba(46,16,101,.48);overflow:hidden}.price-board-head{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:20px 22px;border-bottom:1px solid var(--line);background:linear-gradient(135deg,#fbfaff,#f4edff)}.price-board-head span{font-family:var(--mono);font-size:11px;letter-spacing:.1em;text-transform:uppercase;color:var(--violet-deep);font-weight:700}.price-board-head b{display:inline-flex;align-items:center;white-space:nowrap;border-radius:999px;background:#111;color:#dff36e;padding:8px 12px;font:800 12px/1 var(--mono);letter-spacing:.04em}.price-board-body{padding:24px 24px 22px}.price-bars{display:grid;gap:18px}.price-bar-row{display:grid;grid-template-columns:112px minmax(0,1fr) 72px;align-items:center;gap:14px}.price-bar-row span{font-size:13px;font-weight:750;color:var(--ink2)}.price-bar-row strong{font-family:var(--mono);font-size:13px;text-align:right;color:var(--ink)}.price-track{position:relative;height:20px;border-radius:999px;background:#eeeaf7;overflow:hidden}.price-fill{position:absolute;inset:0 auto 0 0;border-radius:999px}.price-fill.official{width:100%;background:#c9c4d3}.price-fill.flatkey{width:50%;background:linear-gradient(90deg,var(--violet-hi),#c026d3)}.price-save{margin-top:22px;border-radius:16px;background:#111;color:#fff;padding:20px}.price-save small{display:block;font-family:var(--mono);font-size:10.5px;letter-spacing:.09em;text-transform:uppercase;color:#dff36e;font-weight:750}.price-save strong{display:block;margin-top:9px;font-size:22px;line-height:1.16;letter-spacing:-.03em}.price-board a{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:15px 24px;border-top:1px solid var(--line);color:var(--violet-deep);font-size:13px;font-weight:800;text-decoration:none}.price-board a:after{content:"→";font-family:var(--mono)}
        .why{background:var(--home-surface);border-bottom:1px solid var(--line);position:relative;overflow:hidden}.whyIn{max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:80px var(--fk-site-gutter);display:grid;grid-template-columns:300px 1fr;gap:48px;position:relative;z-index:1}.whyHead{position:sticky;top:40px;align-self:start}.whyGrid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.why .wcard{background:var(--paper2);border:1px solid var(--line);border-radius:16px;padding:26px 28px}.why .wcard h3{font-family:var(--disp);font-size:20px;letter-spacing:-.5px;font-weight:700;margin-bottom:10px}.why .wcard p{font-size:13.5px;color:var(--ink2);line-height:1.6}.why .wcard.dark{background:var(--dark);border-color:transparent}.why .wcard.dark h3{color:#fff}.why .wcard.dark p{color:#B9B9C6}.why .mini{margin-top:20px}.why .mbar{display:grid;grid-template-columns:56px 1fr 52px;gap:10px;align-items:center;margin-bottom:8px;margin-top:0;font:inherit}.why .mbar span{font-family:var(--mono);font-size:11px;color:var(--ink3)}.why .mbar b{font-family:var(--mono);font-size:12px;text-align:right}.why .mtrack{height:20px;border-radius:5px;background:var(--line2);position:relative;overflow:visible}.why .mfill{position:absolute;inset:0 auto 0 0;border-radius:5px;height:auto}.why .mcap{font-size:11px!important;color:var(--ink3)!important;margin-top:10px!important}.why .labs{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:20px}.why .lab{width:auto;height:auto;border-radius:10px;color:#fff;font-family:var(--disp);font-weight:700;font-size:20px;padding:14px 14px 10px;display:flex;flex-direction:column;gap:6px;place-items:initial}.why .lab span{font-family:var(--mono);font-size:9.5px;font-weight:400;color:#ffffffcc;letter-spacing:.4px;line-height:1}.why .wcode{margin-top:18px;font-family:var(--mono);font-size:11.5px;line-height:1.75;color:#C9E8D4;background:#ffffff0d;border-radius:10px;padding:14px 16px;overflow-x:auto;white-space:pre-wrap}.why .wcode span{color:#A7F3C8}.why .wcode em{color:#8E8E9C;font-style:normal}.why .chips{display:flex;flex-wrap:wrap;gap:8px;margin-top:18px}.why .chips span{font-size:12px;font-weight:650;border:1px solid var(--line);background:#fff;border-radius:999px;padding:6px 12px;color:var(--ink2)}
        section.v{position:relative;min-height:680px;overflow:hidden;display:flex;flex-direction:column;justify-content:center;color:#F4F4F0}section.v .in{position:relative;z-index:2;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:150px var(--fk-site-gutter);width:100%}.bgv{position:absolute;inset:0;z-index:0}.shade{position:absolute;inset:0;z-index:1;background:linear-gradient(90deg,rgba(8,8,14,.9) 0%,rgba(8,8,14,.6) 48%,rgba(8,8,14,.2) 100%)}.kick{font-family:var(--mono);font-size:12.5px;letter-spacing:2.5px;color:#B7A3F0;margin-bottom:20px;font-weight:600}h2.big{font-family:var(--disp);font-size:72px;line-height:1;letter-spacing:-2.8px;font-weight:600;color:#fff;text-wrap:balance;max-width:900px}h2.big em{font-style:normal;color:var(--violet-hi)}p.d{font-size:17.5px;line-height:1.65;color:#C9C9CF;margin-top:20px;max-width:620px}.vtag{position:absolute;left:40px;bottom:26px;z-index:3;font-family:var(--mono);font-size:11.5px;color:#8E8E9C;display:flex;gap:10px;align-items:center}.vtag .p{width:24px;height:24px;border-radius:50%;background:#ffffff1f;display:flex;align-items:center;justify-content:center;color:#fff;font-size:9px}.badges{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px;margin-top:30px;max-width:960px}.badge{border:1px solid #ffffff2e;border-radius:10px;padding:14px 16px;background:#ffffff0d;backdrop-filter:blur(4px)}.badge b{display:block;font-size:14px;font-weight:650;color:#fff;letter-spacing:-.2px}.badge span{display:block;font-size:11px;color:#9a9aa6;font-family:var(--mono);margin-top:4px;line-height:1.5}.ledger{position:absolute;inset:0;background:linear-gradient(180deg,#0A0A12,#0D0B18);overflow:hidden}.lcol{position:absolute;top:-40%;width:300px;font-family:var(--mono);font-size:11px;color:#7C5CF755;line-height:2.6;white-space:nowrap;animation:fall 16s linear infinite}@keyframes fall{to{transform:translateY(45%)}}.wall{position:absolute;inset:0;background:#08080E;display:flex;flex-direction:column;justify-content:center;gap:14px;overflow:hidden}.lane{display:flex;gap:14px;animation:slide 30s linear infinite;width:max-content}.lane.r{animation-direction:reverse}.tile{width:186px;height:100px;border-radius:10px;background:#121220;border:1px solid #ffffff12;padding:14px 16px;flex:none}.tile b{font-size:13.5px;color:#EDEDEF;display:block;font-weight:650;letter-spacing:-.2px}.tile span{font-family:var(--mono);font-size:10px;color:#77778A}.tile .pr{font-family:var(--mono);font-size:11px;color:#B7A3F0;margin-top:11px;display:block}@keyframes slide{to{transform:translateX(-50%)}}@media(max-width:900px){.home-section-actions{flex-direction:column}.heroCtas{flex-wrap:wrap}section.v .in{padding:96px 24px}.badges{grid-template-columns:1fr!important}h2.big{font-size:54px;letter-spacing:-1.8px}.vtag{left:24px}}@media(max-width:620px){h2.big{font-size:42px;letter-spacing:-1.2px}.vtag{font-size:10px}}
        :root{--tool-acid:#dff36e;--tool-violet:#7627df;--tool-ink:#111014;--tool-muted:#706a74;--tool-line:#ded7e7;--tool-soft:#faf9fb;--home-surface:#f8f6fc;--home-gradient:radial-gradient(ellipse 62% 52% at 50% 8%,rgba(124,58,237,.18),transparent 72%),linear-gradient(135deg,#fbfaff 0 56%,#f0e8ff 56% 100%)}
        .tools-intro,.tool-universe{background:var(--home-gradient)}
        .tools-intro{position:relative;overflow:hidden;border-bottom:1px solid var(--tool-line)}
        .tools-intro:before{content:"";position:absolute;inset:0;pointer-events:none;background-image:linear-gradient(to right,rgba(124,58,237,.065) 1px,transparent 1px),linear-gradient(to bottom,rgba(124,58,237,.065) 1px,transparent 1px);background-size:72px 72px;mask-image:linear-gradient(to bottom,rgba(0,0,0,.78),transparent 88%)}
        .tools-intro:after{display:none}.tools-intro-inner{position:relative;z-index:2;max-width:var(--fk-site-frame-max-width);min-height:inherit;margin:0 auto;padding:150px var(--fk-site-gutter);display:grid;grid-template-columns:minmax(0,.94fr) minmax(520px,1.06fr);gap:76px;align-items:center}.tools-kicker{display:flex;align-items:center;gap:13px;color:#4d49ff;font:500 12px/1.4 var(--mono);letter-spacing:.04em}.tools-kicker:before{content:"";width:28px;height:2px;background:#706aff}.tools-title{margin:27px 0 0;max-width:700px;font-family:var(--disp);font-size:clamp(58px,6vw,94px);line-height:1.02;font-weight:700;letter-spacing:-.065em}.tools-title-line{white-space:nowrap}html:not(:lang(en)) .tools-title{font-size:clamp(48px,3.8vw,62px)}.tools-copy{max-width:690px;margin:30px 0 0;color:#615b64;font-size:18px;line-height:1.7}.agent-label{display:flex;align-items:center;gap:8px;margin-top:30px;font-size:12px;font-weight:700}.agent-marks{display:flex;margin-left:auto;gap:5px}.agent-mark{width:25px;height:25px;border-radius:7px;display:grid;place-items:center;color:#fff;font:700 10px/1 var(--mono)}.agent-mark.openai{background:#4e4652}.agent-mark.claude{background:#dc866c}.agent-mark.codex{background:#111}.agent-mark.more{width:auto;padding:0 8px;background:#f0edf2;color:#78717c}.agent-command{width:100%;display:flex;align-items:center;gap:14px;min-height:76px;margin-top:14px;padding:12px 18px 12px 24px;border:0;border-radius:14px;background:#0d0d10;color:#eee;text-align:left;font:500 13px/1.5 var(--mono);box-shadow:0 26px 60px -34px #000;cursor:pointer}.agent-command .prompt{color:#827b87}.agent-command code{flex:1;white-space:normal}.copy-state{min-width:72px;flex:none;display:flex;align-items:center;justify-content:flex-end;gap:8px;color:#aaa4ae;font-size:10px}.copy-icon-wrap{position:relative;width:20px;height:20px;display:grid;place-items:center}.copy-icon-wrap svg{position:absolute;width:16px;height:16px}.check-icon{color:#4ade80;opacity:0}.copy-label{min-width:38px;text-align:right}.agent-note{margin-top:13px;color:#99929d;font:400 10px/1.5 var(--mono)}.agent-links{display:flex;gap:34px;margin-top:30px;color:#4b454e;font-size:13px;font-weight:600}.terminal{position:relative;border:1px solid #dadfe8;border-radius:16px;background:rgba(255,255,255,.94);box-shadow:0 36px 90px -48px rgba(37,21,54,.45);overflow:hidden}.terminal-head{height:58px;display:flex;align-items:center;gap:9px;padding:0 18px;background:#f4f7fa;color:#708096;font:500 11px/1 var(--mono)}.terminal-dot{width:11px;height:11px;border-radius:50%}.terminal-dot.red{background:#ff675f}.terminal-dot.yellow{background:#ffbd2e}.terminal-dot.green{background:#29c940}.terminal-head span:last-child{margin-left:7px}.terminal-body{min-height:485px;padding:28px 29px 34px;color:#4e5c71;font:400 13px/1.72 var(--mono)}.term-brand{display:flex;align-items:center;gap:15px;margin:6px 0 25px}.term-bot{width:55px;height:39px;position:relative;background:#7627df;clip-path:polygon(16% 0,40% 0,40% 18%,60% 18%,60% 0,84% 0,84% 18%,100% 18%,100% 82%,84% 82%,84% 100%,68% 100%,68% 82%,32% 82%,32% 100%,16% 100%,16% 82%,0 82%,0 18%,16% 18%)}.term-brand b{display:block;color:#2c3543;font-size:13px}.term-brand span{display:block;color:#7e8ba0;font-size:11px}.term-command{color:#354356;margin-bottom:20px}.term-line{margin:7px 0;color:#718097}.term-line.ok{color:#1ba850}.term-line.bill{color:#6f22cf;font-weight:600}.term-divider{height:1px;margin:22px 0;background:#e3e7ec}.term-summary{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}.term-stat{border:1px solid #e1e5eb;border-radius:10px;padding:11px}.term-stat small{display:block;color:#8b96a6;font-size:8px;text-transform:uppercase;letter-spacing:.08em}.term-stat b{display:block;color:#354152;font-size:13px;margin-top:5px}
        .tool-universe{position:relative;min-height:100svh;overflow:hidden;border-bottom:1px solid var(--tool-line)}.tool-universe:before{content:"";position:absolute;inset:0;background-image:linear-gradient(to right,rgba(124,58,237,.05) 1px,transparent 1px),linear-gradient(to bottom,rgba(124,58,237,.05) 1px,transparent 1px);background-size:72px 72px;mask-image:linear-gradient(to bottom,rgba(0,0,0,.64),transparent 94%);pointer-events:none}.universe-inner{position:relative;z-index:1;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:90px var(--fk-site-gutter) 82px;text-align:center}.universe-title{font-family:var(--disp);font-size:clamp(46px,5vw,76px);line-height:1.03;font-weight:700;letter-spacing:-.055em}.universe-copy{max-width:790px;margin:20px auto 0;color:#6d6770;font-size:17px;line-height:1.65}.hub{position:relative;height:168px;margin-top:34px;display:block}.hub-core{position:absolute;z-index:3;top:8px;left:50%;transform:translateX(-50%);width:460px;padding:16px 18px 14px;border:1px solid #d8cee2;border-radius:19px;background:rgba(255,255,255,.96);box-shadow:0 28px 70px -42px rgba(65,24,98,.6),0 0 0 7px rgba(118,39,223,.035);text-align:left}.hub-core:before{content:"";position:absolute;width:1px;height:22px;left:50%;top:-22px;background:linear-gradient(#dcd6de,#a56ee6)}.hub-core:after{content:"";position:absolute;width:1px;height:34px;left:50%;bottom:-34px;background:linear-gradient(#a56ee6,#e4dfe6)}.hub-brand{display:flex;align-items:center;gap:12px}.hub-logo{width:48px;height:48px;flex:none;filter:drop-shadow(0 10px 14px rgba(109,40,217,.18))}.hub-brand-copy{min-width:0;margin-right:0;text-align:left}.hub-brand-copy strong{display:block;font-size:23px;line-height:1;font-weight:800;letter-spacing:-1px}.hub-brand-copy span{display:block;margin-top:5px;color:#796f7e;font:500 8.5px/1.3 var(--mono);letter-spacing:.08em;text-transform:uppercase}.hub-live{margin-left:auto;display:inline-flex;align-items:center;gap:6px;padding:7px 9px;border:1px solid #d9ecd9;border-radius:999px;background:#f3fbf3;color:#277748;font:600 8px/1 var(--mono);letter-spacing:.05em}.hub-live:before{content:"";width:6px;height:6px;border-radius:50%;background:#2db76b;box-shadow:0 0 0 4px rgba(45,183,107,.1)}.hub-metrics{display:grid;grid-template-columns:repeat(3,1fr);gap:1px;margin-top:13px;border:1px solid #e7e0eb;border-radius:10px;overflow:hidden;background:#e7e0eb}.hub-metric{display:flex;align-items:baseline;gap:6px;justify-content:center;padding:10px 8px;background:#faf8fc}.hub-metric b{color:#5f1db5;font:700 13px/1 var(--mono)}.hub-metric small{color:#827a86;font:500 7.5px/1 var(--mono);text-transform:uppercase;letter-spacing:.05em}.capability-map{position:relative;z-index:2;display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin-top:2px}.capability-row,.provider-cards{display:contents}.provider-card{position:relative;min-width:0;height:84px;display:flex;align-items:center;gap:12px;padding:14px 15px;border:1px solid #e2dce4;border-radius:13px;background:rgba(255,255,255,.96);text-align:left;box-shadow:0 12px 30px -29px rgba(33,19,42,.5);transition:transform .2s ease,border-color .2s ease,box-shadow .2s ease}.provider-card:hover{transform:translateY(-2px);border-color:#d5c8db;box-shadow:0 20px 40px -31px rgba(33,19,42,.6)}.provider-icon{width:38px;height:38px;flex:none;border-radius:9px;display:grid;place-items:center;background:#fff}.provider-icon img{display:block;width:32px;height:32px;object-fit:contain}.provider-icon.voc-mark{background:#111;color:#fff;border-radius:11px;box-shadow:inset 0 0 0 1px rgba(255,255,255,.15)}.provider-icon.voc-mark span{margin:0;color:#fff;font:800 10px/1 var(--mono);letter-spacing:-.05em}.provider-card>div{min-width:0}.provider-card b{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;line-height:1.2}.provider-card div>span{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin-top:5px;color:#8d8591;font:400 8.3px/1.3 var(--mono)}.provider-card.more-card{overflow:hidden;border-color:#d7c9e8;background:linear-gradient(135deg,rgba(255,255,255,.98),rgba(246,240,255,.98));color:#17131c;text-decoration:none;box-shadow:0 16px 36px -28px rgba(82,32,137,.48)}.provider-card.more-card:after{content:"↗";position:absolute;right:15px;top:50%;transform:translateY(-50%);color:#7340ac;font:600 15px/1 var(--mono)}.more-icon{position:relative;overflow:visible;background:linear-gradient(145deg,#22172e,#7933c5);box-shadow:0 8px 20px -10px rgba(81,25,143,.72)}.more-icon:before,.more-icon:after,.more-icon span{content:"";position:absolute;width:13px;height:13px;border:1px solid rgba(255,255,255,.72);border-radius:4px;background:rgba(255,255,255,.14);box-shadow:0 3px 8px rgba(27,11,41,.15)}.more-icon:before{left:7px;top:7px}.more-icon span{left:17px;top:11px}.more-icon:after{left:11px;top:19px}.more-card.tools-more .more-icon{background:linear-gradient(145deg,#3b2451,#9b58e3)}.more-card b em{font-style:normal;color:#6c2bad}.tool-values{display:grid;grid-template-columns:repeat(4,1fr);gap:1px;margin-top:22px;border:1px solid #e2dce4;background:#e2dce4;border-radius:14px;overflow:hidden}.tool-value{background:#fff;padding:21px;text-align:left;border:0;border-radius:0}.tool-value small{color:#6f27c4;font:600 9px/1 var(--mono);letter-spacing:.08em}.tool-value b{display:block;margin-top:15px;font-size:14px}.tool-value p{margin:7px 0 0;color:#78717b;font-size:11px;line-height:1.55}.rel,.why,.media,.steps,.support,.ctaWrap{background-color:var(--home-surface)}
        @media(max-width:1050px){.price-proof-in,.tools-intro-inner{grid-template-columns:1fr;padding:78px var(--fk-site-gutter);gap:50px}.tools-intro-inner{padding:120px var(--fk-site-gutter)}.price-proof-copy{max-width:760px}.tools-title{max-width:850px}.terminal{max-width:780px}.hub{height:164px}.hub-core{top:4px}.tool-values{grid-template-columns:repeat(2,1fr)}}@media(max-width:700px){.price-proof-in,.tools-intro-inner{padding:64px var(--fk-site-gutter)}.tools-intro-inner{padding:88px var(--fk-site-gutter)}.price-board-head{align-items:flex-start;flex-direction:column}.price-board-body{padding:20px}.price-bar-row{grid-template-columns:1fr;gap:8px}.price-bar-row strong{text-align:left}.tools-title{font-size:clamp(39px,11.2vw,49px)}.tools-copy{font-size:15.5px}html:not(:lang(en)) .tools-title{font-size:clamp(34px,9vw,43px)}.agent-marks{display:none}.agent-command{font-size:10px;padding:14px}.copy-label{display:none}.copy-state{min-width:24px}.terminal-body{min-height:430px;padding:20px 17px;font-size:10px}.term-summary{grid-template-columns:1fr}.universe-inner{padding:68px var(--fk-site-gutter)}.universe-title{font-size:43px}.universe-copy{font-size:14px}.provider-card{height:82px;padding:13px 12px}.provider-icon{width:30px;height:30px}.provider-card b{font-size:11px}.capability-map{grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.tool-values{grid-template-columns:1fr}.hub{height:166px}.hub-core{width:100%;padding:14px}.hub-logo{width:42px;height:42px}.hub-brand-copy strong{font-size:20px}}
      `}</style>
      <header className={`hero heroUnified compactHero compactHero-${props.locale}`}>
        <div className="heroGrid">
          <div className="heroCopy">
            <h1 className="display">
              {renderHeroTitle(props.locale, copy.home.heroTitle)}
            </h1>
            <p className="sub">{renderHeroSub(props.locale, copy.home.sub)}</p>
            <div className="heroCtas">
              <Link className="btn big heroPrimary" href={overviewActionHref}>
                {authActionLabel}
              </Link>
              <Link
                className="btn big heroSecondary"
                href={localizePath("/models", props.locale)}
              >
                {copy.home.ctaModels}
              </Link>
            </div>
          </div>
        </div>
      </header>
      <section className="modelStrip" aria-label="Featured models">
        <div className="modelStripInner">
          <ModelStripCarousel items={FEATURED_MODEL_STRIP} />
        </div>
      </section>
      <section className="intelligence-section" aria-labelledby="intelligence-heading">
        <div className="intelligence-wrap">
          {(() => {
            const intelligence = INTELLIGENCE_COPY[props.locale];
            return (
              <>
                <div className="intelligence-heading">
                  <h2 id="intelligence-heading">{intelligence.title}</h2>
                  <p>Just use one Key — our system intelligently picks the best model/tools for every task, input, and scenario.</p>
                </div>
                <div className="intelligence-tabs" role="tablist" aria-label={intelligence.eyebrow}>
                  {intelligence.tabs.map((tab, index) => (
                    <div key={tab}>
                      <input className="intelligence-tab-input" type="radio" name="intelligence-tab" id={`intelligence-tab-${index}`} defaultChecked={index === 0} />
                      <label className="intelligence-tab" htmlFor={`intelligence-tab-${index}`} role="tab">
                        <svg width={index === 1 ? 18 : 20} height={index === 1 ? 15 : 20} viewBox={index === 1 ? "0 0 18 15" : "0 0 20 20"} fill="none" aria-hidden="true">
                          {index === 0 ? <>
                            <path d="M12.4998 9.79163C12.9601 9.79163 13.3332 10.1647 13.3332 10.625V15.625C13.3332 17.4659 11.8408 18.9583 9.99984 18.9583C8.1589 18.9583 6.66652 17.4659 6.6665 15.625V15.2083C6.6665 14.7481 7.0396 14.375 7.49984 14.375C7.96007 14.375 8.33317 14.7481 8.33317 15.2083V15.625C8.33319 16.5454 9.07938 17.2916 9.99984 17.2916C10.9203 17.2916 11.6665 16.5454 11.6665 15.625V10.625C11.6665 10.1647 12.0396 9.79163 12.4998 9.79163Z" fill="black" />
                            <path d="M4.7915 6.66663C5.25174 6.66663 5.62484 7.03972 5.62484 7.49996C5.62484 7.9602 5.25174 8.33329 4.7915 8.33329H4.36833C3.45358 8.33329 2.70817 9.07741 2.70817 9.99996C2.70817 10.9225 3.45358 11.6666 4.36833 11.6666H9.37484C9.83507 11.6666 10.2082 12.0397 10.2082 12.5C10.2082 12.9602 9.83507 13.3333 9.37484 13.3333H4.36833C2.52894 13.3333 1.0415 11.8388 1.0415 9.99996C1.0415 8.16109 2.52894 6.66663 4.36833 6.66663H4.7915Z" fill="black" />
                            <path d="M15.62 6.66663C17.462 6.66663 18.9582 8.15755 18.9582 9.99996C18.9582 11.8424 17.462 13.3333 15.62 13.3333H15.2082C14.7479 13.3333 14.3748 12.9602 14.3748 12.5C14.3748 12.0397 14.7479 11.6666 15.2082 11.6666H15.62C16.5445 11.6666 17.2915 10.919 17.2915 9.99996C17.2915 9.08095 16.5445 8.33329 15.62 8.33329H10.6248C10.1646 8.33329 9.7915 7.9602 9.7915 7.49996C9.7915 7.03972 10.1646 6.66663 10.6248 6.66663H15.62Z" fill="black" />
                            <path d="M9.99984 1.04163C11.8408 1.04163 13.3332 2.53401 13.3332 4.37496V4.79163C13.3332 5.25186 12.9601 5.62496 12.4998 5.62496C12.0396 5.62496 11.6665 5.25186 11.6665 4.79163V4.37496C11.6665 3.45448 10.9203 2.70829 9.99984 2.70829C9.07936 2.70829 8.33317 3.45448 8.33317 4.37496V9.37496C8.33317 9.8352 7.96007 10.2083 7.49984 10.2083C7.0396 10.2083 6.6665 9.8352 6.6665 9.37496V4.37496C6.6665 2.53401 8.15889 1.04163 9.99984 1.04163Z" fill="black" />
                          </> : index === 1 ? <>
                            <path fillRule="evenodd" clipRule="evenodd" d="M4.79167 2.91667C5.59707 2.91667 6.25 3.5696 6.25 4.375C6.25 5.1804 5.59707 5.83333 4.79167 5.83333C3.98626 5.83333 3.33333 5.1804 3.33333 4.375C3.33333 3.5696 3.98626 2.91667 4.79167 2.91667ZM4.79167 4.16667C4.6766 4.16667 4.58333 4.25993 4.58333 4.375C4.58333 4.49007 4.6766 4.58333 4.79167 4.58333C4.90674 4.58333 5 4.49007 5 4.375C5 4.25993 4.90674 4.16667 4.79167 4.16667Z" fill="#454545" />
                            <path fillRule="evenodd" clipRule="evenodd" d="M15.8333 0C16.7538 0 17.5 0.7462 17.5 1.66667V13.3333C17.5 14.2538 16.7538 15 15.8333 15H1.66667C0.7462 15 0 14.2538 0 13.3333V1.66667C0 0.746192 0.746192 0 1.66667 0H15.8333ZM7.71566 9.70866C7.42329 10.0498 6.9133 10.0984 6.5625 9.81771L5.06104 8.61654L1.66667 12.0117V13.3333H15.8333V12.0776L9.70215 7.39014L7.71566 9.70866ZM1.66667 9.65495L4.41081 6.91081L4.4694 6.85791C4.77053 6.60864 5.21001 6.6003 5.52083 6.84896L6.9751 8.0127L8.95101 5.70801L9.00716 5.64779C9.3005 5.36657 9.7603 5.33663 10.0895 5.58838L15.8333 9.98047V1.66667H1.66667V9.65495Z" fill="#454545" />
                          </> : <>
                            <path fillRule="evenodd" clipRule="evenodd" d="M7.29199 6.03023C7.54972 5.88153 7.86759 5.88153 8.12533 6.03023L13.7503 9.27812C14.008 9.42693 14.1669 9.70238 14.167 9.99996C14.1669 10.2975 14.0079 10.5721 13.7503 10.721L8.12533 13.9689C7.86749 14.1177 7.54983 14.1177 7.29199 13.9689C7.03425 13.82 6.87533 13.5447 6.87533 13.247V6.75208C6.87543 6.45447 7.03425 6.17904 7.29199 6.03023ZM8.54199 11.8033L11.667 9.99915L8.54199 8.19495V11.8033Z" fill="#454545" />
                            <path fillRule="evenodd" clipRule="evenodd" d="M16.2503 1.66663C17.4009 1.66663 18.3337 2.59937 18.3337 3.74996V16.25C18.3337 17.4006 17.4009 18.3333 16.2503 18.3333H3.75033C2.59973 18.3333 1.66699 17.4006 1.66699 16.25V3.74996C1.66699 2.59937 2.59973 1.66663 3.75033 1.66663H16.2503ZM3.75033 3.33329C3.52021 3.33329 3.33366 3.51984 3.33366 3.74996V16.25C3.33366 16.4801 3.52021 16.6666 3.75033 16.6666H16.2503C16.4804 16.6666 16.667 16.4801 16.667 16.25V3.74996C16.667 3.51984 16.4804 3.33329 16.2503 3.33329H3.75033Z" fill="#454545" />
                          </>}
                        </svg>
                        {tab}
                      </label>
                    </div>
                  ))}
                </div>
                <div className="intelligence-panel intelligence-panel-text" role="tabpanel">
                  <div className="intelligence-prompt">
                    <div className="intelligence-card-head"><span className="pixel-mark" aria-hidden="true" />{intelligence.promptTitle}</div>
                    <p className="intelligence-prompt-body">{intelligence.promptBody}</p>
                    <div className="intelligence-prompt-input" aria-hidden="true"><span /><b>↑</b></div>
                  </div>
                  <div className="intelligence-analysis">
                    <h3>{intelligence.actionTitle}</h3>
                    <ul className="intelligence-checklist">{intelligence.checklist.map((item) => <li key={item}>{item}</li>)}</ul>
                    <div className="intelligence-metrics">
                      <div className="intelligence-metric"><span>{intelligence.recordsLabel}</span><strong>{intelligence.recordsValue}</strong></div>
                      <div className="intelligence-metric"><span>{intelligence.processedLabel}</span><strong>{intelligence.processedValue}</strong></div>
                      <div className="intelligence-metric"><span>{intelligence.costLabel}</span><strong>{intelligence.costValue}</strong></div>
                    </div>
                    <p className="intelligence-footer">{intelligence.footer}</p>
                  </div>
                  <div className="intelligence-criteria">
                    <div className="criteria-head"><span className="criteria-icon" aria-hidden="true" /><span>{intelligence.actionTitle}</span><img className="criteria-shield" src="/assets/flatkey-mark.svg" alt="" /></div>
                    {intelligence.checklist.map((item, index) => <div className={`criteria-row${index > 1 ? " success" : ""}`} key={item}>{item}</div>)}
                    <div className="criteria-metrics">
                      <div className="criteria-metric"><span>{intelligence.recordsLabel}</span><strong>{intelligence.recordsValue}</strong></div>
                      <div className="criteria-metric"><span>{intelligence.processedLabel}</span><strong>{intelligence.processedValue}</strong></div>
                      <div className="criteria-metric"><span>{intelligence.costLabel}</span><strong>{intelligence.costValue}</strong></div>
                    </div>
                    <div className="criteria-pay">{intelligence.footer}</div>
                  </div>
                  <div className="intelligence-models">
                    <div className="model-card"><img src="/assets/logos/openai.svg" alt="" /><div className="model-card-copy"><strong>openai/gpt-5.6-sol</strong><span>GPT · frontier model</span></div></div>
                    <div className="model-card"><img src="/assets/logos/deepseek.svg" alt="" /><div className="model-card-copy"><strong>deepseek-v4-flash</strong><span>DeepSeek · reasoning model</span></div></div>
                    <div className="model-selected-label">Selected model</div>
                    <div className="model-card selected"><img src="/assets/logos/claude.svg" alt="" /><div className="model-card-copy"><strong>claude-opus-5</strong><span>Claude · reasoning model</span></div><span className="model-card-price">$4.5 / 1M tokens</span></div>
                    <div className="model-card"><img src="/assets/logos/googlegemini.svg" alt="" /><div className="model-card-copy"><strong>google/gemini-3.7-flash</strong><span>Gemini · multimodal model</span></div></div>
                    <div className="model-card"><img src="/assets/logos/zai.svg" alt="" /><div className="model-card-copy"><strong>z-ai/glm-5.3-flash</strong><span>dal modelGLM · multim</span></div></div>
                  </div>
                  <svg className="intelligence-connectors" width="80" height="200" viewBox="0 0 80 200" fill="none" aria-hidden="true">
                    <line y1="100" x2="80" y2="100" stroke="url(#criteria-line)" />
                    <path d="M0 49.5C12.4079 49.5 23.9061 56.0101 30.2899 66.6499L41.2609 84.9349C46.6826 93.971 56.4478 99.5 66.9857 99.5H80" stroke="url(#criteria-top)" />
                    <path d="M0 149.5C12.4079 149.5 23.9061 142.99 30.2899 132.35L41.2609 114.065C46.6826 105.029 56.4478 99.5 66.9857 99.5H80" stroke="url(#criteria-bottom)" />
                    <defs>
                      <linearGradient id="criteria-line" x1="0" y1="101" x2="80" y2="101" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                      <linearGradient id="criteria-top" x1="0" y1="74.5" x2="80" y2="74.5" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                      <linearGradient id="criteria-bottom" x1="0" y1="124.5" x2="80" y2="124.5" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                    </defs>
                  </svg>
                </div>
                <div className="intelligence-panel intelligence-panel-media intelligence-panel-image" role="tabpanel">
                  <div className="intelligence-prompt">
                    <div className="intelligence-card-head"><span className="pixel-mark" aria-hidden="true" />Test Prompt</div>
                    <img className="prompt-reference" src="/assets/home-tabs/headphones-reference.png" alt="Rose-gold wireless headphones reference" />
                    <p className="intelligence-prompt-body">Generate a fashion ad-style product scene for these headphones. A young female model wears rose-gold wireless over-ear headphones in a bright outdoor garden with oversized artistic flowers in orange-red, pink, lavender, and cream—a dreamlike floral backdrop.</p>
                    <div className="intelligence-prompt-input" aria-hidden="true"><span /><b>↑</b></div>
                  </div>
                  <div className="intelligence-media-card">
                    <div className="criteria-head"><span className="criteria-icon" aria-hidden="true" /><span>Criteria</span><img className="criteria-shield" src="/assets/flatkey-mark.svg" alt="" /></div>
                    <div className="intelligence-media-body">
                      <img className="intelligence-result-media" src="/assets/home-tabs/image-fashion-result.png" alt="Fashion image generated from the headphones reference" />
                      <div className="media-metrics">
                        <div className="media-metric"><span>Model</span><strong>{IMAGE_SELECTED_MODEL}</strong></div>
                        <div className="media-metric"><span>Total runtime</span><strong>18.4 sec</strong></div>
                        <div className="media-metric"><span>One invoice</span><strong>$0.06</strong></div>
                      </div>
                    </div>
                    <div className="media-pay"><div>Pay per successful call&nbsp; · &nbsp;failed calls $0.00</div></div>
                  </div>
                  <div className="intelligence-models">
                    <div className="model-card"><img src="/assets/logos/googlegemini.svg" alt="" /><div className="model-card-copy"><strong>nano-banana-pro-preview</strong><span>Google · image model</span></div></div>
                    <div className="model-card"><img src="/assets/logos/googlegemini.svg" alt="" /><div className="model-card-copy"><strong>gemini-3.1-flash-image</strong><span>Google · image model</span></div></div>
                    <div className="model-selected-label">Selected model</div>
                    <div className="model-card selected"><img src="/assets/logos/openai.svg" alt="" /><div className="model-card-copy"><strong>{IMAGE_SELECTED_MODEL}</strong><span>OpenAI · image model</span></div><span className="model-card-price">$4 / 1M tokens</span></div>
                    <div className="model-card"><img src="/assets/logos/googlegemini.svg" alt="" /><div className="model-card-copy"><strong>imagen-4.0-ultra-generate-001</strong><span>Google · image model</span></div></div>
                    <div className="model-card"><span className="model-placeholder" aria-hidden="true">F</span><div className="model-card-copy"><strong>flux-2-pro</strong><span>Black Forest Labs · image model</span></div></div>
                  </div>
                  <svg className="intelligence-connectors" width="80" height="200" viewBox="0 0 80 200" fill="none" aria-hidden="true">
                    <line y1="100" x2="80" y2="100" stroke="url(#image-line)" />
                    <path d="M0 49.5C12.4079 49.5 23.9061 56.0101 30.2899 66.6499L41.2609 84.9349C46.6826 93.971 56.4478 99.5 66.9857 99.5H80" stroke="url(#image-top)" />
                    <path d="M0 149.5C12.4079 149.5 23.9061 142.99 30.2899 132.35L41.2609 114.065C46.6826 105.029 56.4478 99.5 66.9857 99.5H80" stroke="url(#image-bottom)" />
                    <defs>
                      <linearGradient id="image-line" x1="0" y1="101" x2="80" y2="101" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                      <linearGradient id="image-top" x1="0" y1="74.5" x2="80" y2="74.5" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                      <linearGradient id="image-bottom" x1="0" y1="124.5" x2="80" y2="124.5" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                    </defs>
                  </svg>
                </div>
                <div className="intelligence-panel intelligence-panel-media intelligence-panel-video" role="tabpanel">
                  <div className="intelligence-prompt">
                    <div className="intelligence-card-head"><span className="pixel-mark" aria-hidden="true" />Test Prompt</div>
                    <p className="intelligence-prompt-body">Black-and-silver F1 car tearing through a wet forest track, low rear follow-cam. Tires throw white rooster tails of spray; body trembles at speed. Misty pines and faint grandstands in background. Overcast, cool light. Blue-gray, mist-white, deep green tones. Rainy, fast, cinematic.</p>
                    <div className="intelligence-prompt-input" aria-hidden="true"><span /><b>↑</b></div>
                  </div>
                  <div className="intelligence-media-card">
                    <div className="criteria-head"><span className="criteria-icon" aria-hidden="true" /><span>Criteria</span><img className="criteria-shield" src="/assets/flatkey-mark.svg" alt="" /></div>
                    <div className="intelligence-media-body">
                      <IntelligenceVideo
                        className="intelligence-result-media"
                        poster={VIDEO_RESULT_ASSET.poster}
                        src={VIDEO_RESULT_ASSET.video}
                        ariaLabel="F1 car racing on a wet forest track"
                      />
                      <div className="media-metrics">
                        <div className="media-metric"><span>Model</span><strong>{VIDEO_SELECTED_MODEL}</strong></div>
                        <div className="media-metric"><span>Total runtime</span><strong>18.4 sec</strong></div>
                        <div className="media-metric"><span>One invoice</span><strong>$0.16</strong></div>
                      </div>
                    </div>
                    <div className="media-pay"><div>Pay per successful call&nbsp; · &nbsp;failed calls $0.00</div></div>
                  </div>
                  <div className="intelligence-models">
                    <div className="model-card"><img src="/assets/logos/openai.svg" alt="" /><div className="model-card-copy"><strong>sora-2</strong><span>OpenAI · video model</span></div></div>
                    <div className="model-card"><img src="/assets/logos/minimax.svg" alt="" /><div className="model-card-copy"><strong>MiniMax-H3</strong><span>MiniMax · video model</span></div></div>
                    <div className="model-selected-label">Selected model</div>
                    <div className="model-card selected"><img src="/assets/logos/bytedance.svg" alt="" /><div className="model-card-copy"><strong>{VIDEO_SELECTED_MODEL}</strong><span>ByteDance · video model</span></div><span className="model-card-price">$0.084 / 1M</span></div>
                    <div className="model-card"><img src="/assets/logos/googlegemini.svg" alt="" /><div className="model-card-copy"><strong>veo-3.1-generate-preview</strong><span>Google · video model</span></div></div>
                    <div className="model-card"><img src="/assets/logos/kuaishou.svg" alt="" /><div className="model-card-copy"><strong>kling-2.5-pro</strong><span>Kuaishou · video model</span></div></div>
                  </div>
                  <svg className="intelligence-connectors" width="80" height="200" viewBox="0 0 80 200" fill="none" aria-hidden="true">
                    <line y1="100" x2="80" y2="100" stroke="url(#video-line)" />
                    <path d="M0 49.5C12.4079 49.5 23.9061 56.0101 30.2899 66.6499L41.2609 84.9349C46.6826 93.971 56.4478 99.5 66.9857 99.5H80" stroke="url(#video-top)" />
                    <path d="M0 149.5C12.4079 149.5 23.9061 142.99 30.2899 132.35L41.2609 114.065C46.6826 105.029 56.4478 99.5 66.9857 99.5H80" stroke="url(#video-bottom)" />
                    <defs>
                      <linearGradient id="video-line" x1="0" y1="101" x2="80" y2="101" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                      <linearGradient id="video-top" x1="0" y1="74.5" x2="80" y2="74.5" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                      <linearGradient id="video-bottom" x1="0" y1="124.5" x2="80" y2="124.5" gradientUnits="userSpaceOnUse"><stop stopColor="#9377DB" /><stop offset="1" stopColor="#9377DB" stopOpacity="0.2" /></linearGradient>
                    </defs>
                  </svg>
                </div>
              </>
            );
          })()}
        </div>
      </section>
      <section className="tools-intro">
        <div className="tools-intro-inner">
          <div>
            <div className="tools-kicker">{copy.home.toolsKicker}</div>
            <h2 className="tools-title">
              {renderToolsTitle(props.locale, copy.home.toolsTitle)}
            </h2>
            <p className="tools-copy">{copy.home.toolsCopy}</p>
            <div className="agent-label">
              <span>{ht("tools.intro.agent", "Give this to your agent")}</span>
              <div className="agent-marks">
                <span className="agent-mark openai">AI</span>
                <span className="agent-mark claude">C</span>
                <span className="agent-mark codex">⌘</span>
                <span className="agent-mark more">
                  {ht("tools.intro.more", "+ more")}
                </span>
              </div>
            </div>
            <button
              className="agent-command"
              id="copy-flatkey-setup"
              type="button"
              data-action="copy-flatkey-setup"
              aria-label="Copy Flatkey setup prompt"
              data-copy="Set up Flatkey from https://flatkey.ai/SKILL.md"
            >
              <span className="prompt" aria-hidden="true">
                &gt;
              </span>
              <code>{copy.home.toolsCommand}</code>
              <span className="copy-state" aria-hidden="true">
                <span className="copy-label">
                  {ht("tools.intro.copyButton", "Copy")}
                </span>
                <span className="copy-icon-wrap">
                  <svg
                    className="copy-icon"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <rect width="14" height="14" x="8" y="8" rx="2" />
                    <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
                  </svg>
                  <svg
                    className="check-icon"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M20 6 9 17l-5-5" />
                  </svg>
                </span>
              </span>
            </button>
            <div className="agent-note">
              {ht(
                "tools.intro.note",
                "No new seat. No separate provider keys. Every model and tool call lands on the same ledger.",
              )}
            </div>
            <div className="agent-links">
              <a href={consoleUrl("/api-marketplace")}>
                {ht("tools.intro.explore", "Explore tools →")}
              </a>
              <a
                href="https://docs.flatkey.ai/"
                target="_blank"
                rel="noopener noreferrer"
              >
                {ht("tools.intro.docs", "Read the docs")}
              </a>
            </div>
          </div>
          <div className="terminal">
            <div className="terminal-head">
              <i className="terminal-dot red" />
              <i className="terminal-dot yellow" />
              <i className="terminal-dot green" />
              <span>{copy.home.terminal.title}</span>
            </div>
            <div className="terminal-body">
              <div className="term-brand">
                <div className="term-bot" />
                <div>
                  <b>flatkey tools</b>
                  <span>
                    {ht("tools.terminal.tagline", "one key · models + tools")}
                  </span>
                </div>
              </div>
              <div className="term-command">
                {ht(
                  "tools.terminal.command",
                  "❯ Find 14 competitors, research their launches, identify decision makers, waterfall-enrich contacts, and prepare a sourced GTM brief.",
                )}
              </div>
              <div className="term-line">{copy.home.terminal.competitors}</div>
              <div className="term-line">{copy.home.terminal.scanned}</div>
              <div className="term-line ok">{copy.home.terminal.contacts}</div>
              <div className="term-line ok">{copy.home.terminal.persona}</div>
              <div className="term-line bill">{copy.home.terminal.billed}</div>
              <div className="term-divider" />
              <div className="term-summary">
                <div className="term-stat">
                  <small>{copy.home.terminal.successfulLabel}</small>
                  <b>{copy.home.terminal.successfulValue}</b>
                </div>
                <div className="term-stat">
                  <small>{copy.home.terminal.runtimeLabel}</small>
                  <b>{copy.home.terminal.runtimeValue}</b>
                </div>
                <div className="term-stat">
                  <small>{copy.home.terminal.invoiceLabel}</small>
                  <b>{copy.home.terminal.invoiceValue}</b>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
      <section className="tool-universe">
        <div className="universe-inner">
          <div className="tools-kicker" style={{ justifyContent: "center" }}>
            {copy.home.universeKicker}
          </div>
          <h2 className="universe-title" style={{ marginTop: 20 }}>
            {renderLineBreakText(
              ht(
                "tools.universe.title",
                "Every model. Every tool.<br>One key.",
              ),
            )}
          </h2>
          <p className="universe-copy">{copy.home.universeCopy}</p>
          <div className="hub" aria-hidden="true">
            <div className="hub-core">
              <div className="hub-brand">
                <img
                  className="hub-logo"
                  src="/assets/flatkey-mark.svg"
                  alt=""
                />
                <div className="hub-brand-copy">
                  <strong>flatkey</strong>
                  <span>
                    {ht("tools.universe.router", "Unified model + tool router")}
                  </span>
                </div>
                <span className="hub-live">
                  {ht("tools.universe.live", "ROUTING LIVE")}
                </span>
              </div>
              <div className="hub-metrics">
                <span className="hub-metric">
                  <b>100+</b>
                  <small>
                    {ht("tools.universe.models", "official models")}
                  </small>
                </span>
                <span className="hub-metric">
                  <b>1,000+</b>
                  <small>{ht("tools.universe.tools", "AI tools")}</small>
                </span>
                <span className="hub-metric">
                  <b>1</b>
                  <small>
                    {ht("tools.universe.balance", "shared balance")}
                  </small>
                </span>
              </div>
            </div>
          </div>
          <div className="capability-map">
            {providerRows.map((row, rowIndex) => (
              <div
                className={`capability-row ${["models", "social", "crawl", "gtm"][rowIndex]}`}
                data-section={["models", "social", "crawl", "gtm"][rowIndex]}
                key={rowIndex}
              >
                <div className="provider-cards">
                  {row.map(([src, name, desc]) => (
                    <article className="provider-card" key={name}>
                      <i className="provider-icon">
                        <img src={`/assets/${src}`} alt={name} />
                      </i>
                      <div>
                        <b>{name}</b>
                        <span>{desc}</span>
                      </div>
                    </article>
                  ))}
                  {rowIndex === 2 && (
                    <article className="provider-card">
                      <i className="provider-icon voc-mark">
                        <span>VOC</span>
                      </i>
                      <div>
                        <b>VOC AI</b>
                        <span>voice of customer</span>
                      </div>
                    </article>
                  )}
                  {rowIndex === 3 && (
                    <>
                      <Link
                        className="provider-card more-card models-more"
                        href={localizePath("/models", props.locale)}
                        aria-label="Explore more than 100 official AI models"
                      >
                        <i
                          className="provider-icon more-icon"
                          aria-hidden="true"
                        >
                          <span />
                        </i>
                        <div>
                          <b>
                            {renderEmStart(
                              ht(
                                "tools.universe.moreModels",
                                "<em>More</em> models",
                              ),
                            )}
                          </b>
                          <span>
                            {ht(
                              "tools.universe.moreModelsCount",
                              "100+ official models",
                            )}
                          </span>
                        </div>
                      </Link>
                      <a
                        className="provider-card more-card tools-more"
                        href={consoleUrl("/api-marketplace")}
                        aria-label="Explore more than 1,000 AI tools"
                      >
                        <i
                          className="provider-icon more-icon"
                          aria-hidden="true"
                        >
                          <span />
                        </i>
                        <div>
                          <b>
                            {renderEmStart(
                              ht(
                                "tools.universe.moreTools",
                                "<em>More</em> tools",
                              ),
                            )}
                          </b>
                          <span>
                            {ht(
                              "tools.universe.moreToolsCount",
                              "1,000+ available",
                            )}
                          </span>
                        </div>
                      </a>
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
          <div className="tool-values">
            <article className="tool-value">
              <small>{ht("tools.value.authEye", "ONE AUTH")}</small>
              <b>{ht("tools.value.authTitle", "One Flatkey key")}</b>
              <p>
                {ht(
                  "tools.value.authCopy",
                  "No provider credentials scattered across agents, repos, and environments.",
                )}
              </p>
            </article>
            <article className="tool-value">
              <small>{ht("tools.value.contractEye", "ONE CONTRACT")}</small>
              <b>
                {ht(
                  "tools.value.contractTitle",
                  "Normalized calls and results",
                )}
              </b>
              <p>
                {ht(
                  "tools.value.contractCopy",
                  "Consistent schemas, error envelopes, request IDs, and usage records.",
                )}
              </p>
            </article>
            <article className="tool-value">
              <small>{ht("tools.value.policyEye", "ONE POLICY")}</small>
              <b>{ht("tools.value.policyTitle", "Budgets and allowlists")}</b>
              <p>
                {ht(
                  "tools.value.policyCopy",
                  "Control which agents can call which tools and how much they may spend.",
                )}
              </p>
            </article>
            <article className="tool-value">
              <small>{ht("tools.value.billEye", "ONE BILL")}</small>
              <b>{ht("tools.value.billTitle", "Pay for successful work")}</b>
              <p>
                {ht(
                  "tools.value.billCopy",
                  "Models and tools share one balance; failed Flatkey-side calls cost nothing.",
                )}
              </p>
            </article>
          </div>
        </div>
      </section>
      <section className="rel" id="reliability">
        <PixelGrid
          cell={18}
          cols={8}
          n={9}
          rows={3}
          seed={139}
          style={{ right: 38, top: 20 }}
        />
        <div className="relIn">
          <div>
            <div className="kick2">
              {t("rel.kick", "RELIABILITY · SLA, IN WRITING")}
            </div>
            <h2 className="display">
              {renderLineBreakText(
                t("rel.h2", "Stability you can<br>hold us to."),
              )}
            </h2>
            <p className="sub" style={{ marginTop: 14, maxWidth: 440 }}>
              {t(
                "rel.p",
                "One request, five upstream channel classes. When a channel degrades, the router reroutes automatically — and the SLA is a public document, not a sales promise.",
              )}
            </p>
            <div className="relStats">
              <div>
                <b className="num">99.5%</b>
                <span>
                  {renderLineBreakText(
                    t("rel.s1", "signed SLA — public terms →")
                      .replace(/<a[^>]*>/g, "")
                      .replace(/<\/a>/g, ""),
                  )}
                </span>
              </div>
              <div>
                <b className="num">5</b>
                <span>
                  {t("rel.s2", "channel classes, automatic failover")}
                </span>
              </div>
              <div>
                <b className="num">$0</b>
                <span>
                  {t("rel.s3", "balance consumed by flatkey-side errors")}
                </span>
              </div>
            </div>
            <div className="home-section-actions">
              <Link
                className="btn black big"
                href={localizePath("/status", props.locale)}
              >
                {t("rel.cta1", "Live status →")}
              </Link>
              <Link
                className="btn white big"
                href={localizePath("/sla", props.locale)}
              >
                {t("rel.cta2", "SLA & service credits")}
              </Link>
            </div>
          </div>
          <div className="relChart">
            <div className="rcHead">
              <span>
                {t("rel.ct", "UPTIME · SINGLE CHANNEL VS FLATKEY MESH · LIVE")}
              </span>
              <span className="pill mint">
                <span className="dot" />
                LIVE
              </span>
            </div>
            <svg viewBox="0 0 560 220" preserveAspectRatio="none">
              <line x1="0" y1="30" x2="560" y2="30" stroke="#0B0B0F14" />
              <line x1="0" y1="90" x2="560" y2="90" stroke="#0B0B0F14" />
              <line x1="0" y1="150" x2="560" y2="150" stroke="#0B0B0F14" />
              <path
                d="M0,34 L60,32 L90,36 L120,120 L150,140 L170,60 L220,38 L260,34 L300,110 L330,170 L360,150 L390,44 L440,36 L480,100 L510,60 L560,38"
                fill="none"
                stroke="#D97706"
                strokeWidth="2.2"
                strokeLinejoin="round"
              />
              <path
                d="M0,26 L80,24 L160,27 L240,24 L320,28 L400,24 L480,26 L560,24"
                fill="none"
                stroke="#15803D"
                strokeWidth="2.8"
                strokeLinejoin="round"
              />
              <text
                x="8"
                y="20"
                fontSize="11"
                fill="#15803D"
                fontWeight="700"
                fontFamily="monospace"
              >
                flatkey mesh 99.98%
              </text>
              <text
                x="300"
                y="196"
                fontSize="11"
                fill="#D97706"
                fontWeight="700"
                fontFamily="monospace"
              >
                least stable single channel
              </text>
            </svg>
            <p className="rcCap">
              {t(
                "rel.cap",
                "When one upstream errors, the router recovers on the next-best official channel — you see one flat line.",
              )}
            </p>
          </div>
        </div>
      </section>
      <section className="why" id="why">
        <PixelGrid
          cell={18}
          cols={7}
          n={9}
          rows={4}
          seed={113}
          style={{ bottom: 30, left: 34 }}
        />
        <div className="whyIn">
          <div className="whyHead">
            <h2 className="display" style={{ fontSize: 56 }}>
              {renderLineBreakText(t("why.h2", "Why choose<br>flatkey?"))}
            </h2>
          </div>
          <div className="whyGrid">
            <div className="wcard">
              <h3>{t("why.c1h", "As low as 40% of list")}</h3>
              <p>
                {t(
                  "why.c1p",
                  "National-lab models from 60% of list, stacking with the top-up bonus to 40%; flagships bill at list with the same bonus. One prepaid balance across every model.",
                )}
              </p>
              <div className="mini">
                <div className="mbar">
                  <span>official</span>
                  <div className="mtrack">
                    <div
                      className="mfill"
                      style={{ width: "100%", background: "#C9C9D2" }}
                    />
                  </div>
                  <b>$5.00</b>
                </div>
                <div className="mbar">
                  <span>flatkey</span>
                  <div className="mtrack">
                    <div
                      className="mfill"
                      style={{
                        width: "53%",
                        background:
                          "linear-gradient(90deg,var(--violet-hi),var(--violet-deep))",
                      }}
                    />
                  </div>
                  <b style={{ color: "var(--violet-deep)" }}>$2.67</b>
                </div>
                <p className="mcap">
                  claude-opus-4-8 · $/1M input tokens, after bonus
                </p>
              </div>
            </div>
            <div className="wcard">
              <h3>{t("why.c2h", "Official endpoints, verified hourly")}</h3>
              <p>
                {t(
                  "why.c2p",
                  "Every request hits the real lab API — no fp8 re-serves, no silent swaps. 100+ models probed against official fingerprints, on a public log.",
                )}
              </p>
              <div className="labs">
                {[
                  ["G", "OpenAI"],
                  ["C", "Anthropic"],
                  ["G", "Google"],
                  ["D", "DeepSeek"],
                  ["Q", "Alibaba"],
                  ["Z", "Z.ai"],
                ].map(([letter, lab]) => (
                  <div
                    className="lab"
                    key={lab}
                    style={{
                      background:
                        lab === "Anthropic"
                          ? "#B45309"
                          : lab === "Google"
                            ? "#2563EB"
                            : lab === "DeepSeek"
                              ? "#3B52D4"
                              : lab === "Alibaba"
                                ? "var(--violet-deep)"
                                : lab === "Z.ai"
                                  ? "#0B0B0F"
                                  : "#0E8A6C",
                    }}
                  >
                    {letter}
                    <span>{lab}</span>
                  </div>
                ))}
              </div>
            </div>
            <div className="wcard dark">
              <h3>{t("why.c3h", "CLI-native. Powered by SKILL.md.")}</h3>
              <p>
                {t(
                  "why.c3p",
                  "Point Codex, Claude Code, OpenClaw, or any agent CLI at Flatkey’s SKILL.md. It learns how to discover and call 100+ official models and 1,000+ pay-per-call tools with one key.",
                )}
              </p>
              <pre className="wcode">
                <span>$</span> codex{" "}
                <em>{`"Set up Flatkey from\n  https://flatkey.ai/SKILL.md"`}</em>
                {"\n\n"}
                <span>✓</span> 100+ official models connected{"\n"}
                <span>✓</span> 1,000+ tools discovered{"\n"}
                <span>✓</span> one key · one balance
              </pre>
            </div>
            <div className="wcard">
              <h3>{t("why.c4h", "Enterprise-grade governance")}</h3>
              <p>
                {t(
                  "why.c4p",
                  "Tree-structured sub-keys with budgets and model allowlists, a per-request ledger API, invoices in 48h — on audited infrastructure.",
                )}
              </p>
              <div className="chips">
                {[
                  "Sub-key caps",
                  "Model allowlists",
                  "Ledger API",
                  "Invoices 48h",
                  "SOC 2 Type II",
                  "ISO 27001",
                  "GDPR",
                  "Zero retention",
                ].map((chip) => (
                  <span key={chip}>{chip}</span>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>
      <section className="v" id="compliance">
        <PixelGrid
          accent="#22C55E"
          cell={24}
          colors={["#7C3AED66", "#8B5CF64D", "#A78BFA59"]}
          cols={12}
          n={20}
          rows={6}
          seed={43}
          style={{ bottom: 70, right: 34, zIndex: 1 }}
        />
        <div className="bgv ledger">
          <div className="lcol" style={{ left: "56%" }}>
            req_8f3a2e91 · in 1,204 tok · out 388 tok · cached 61% · $0.0041
            <br />
            req_c11977ab · in 640 tok · out 1,022 tok · cached 38% · $0.0009
            <br />
            req_40deb0c2 · in 8,441 tok · out 96 tok · cached 95% · $0.0002
            <br />
            invoice #2026-0714 · VAT · issued 48h
            <br />
            req_77ab40de · 3DS ✓ · stripe checkout · BRL R$55,00
            <br />
            audit_export.csv · 128,441 rows · sha256 ✓
          </div>
          <div className="lcol" style={{ animationDelay: "-8s", left: "75%" }}>
            subkey cline-agent · cap $50/mo · model allowlist ✓<br />
            subkey prod · cap $500/mo · alert 80% ✓<br />
            SLA 99.9% · failover 3-region · breaker 99.95%
            <br />
            gdpr export · user 1835 · 2.1MB · done
            <br />
            req_b0c240de · in 96 tok · out 2,048 tok · $0.0031
          </div>
        </div>
        <div className="shade" />
        <div className="in">
          <div className="kick">
            {t("compl.kick", "COMPLIANCE · ENTERPRISE-GRADE AUDITABILITY")}
          </div>
          <h2 className="big">
            {renderInlineEm(
              t("compl.h2", "Every token,<br>on the record<em>.</em>"),
            )}
          </h2>
          <p className="d">
            {t(
              "compl.p",
              "Per-request ledger with input, output and cached tokens — exportable, API-accessible. Invoices in 48h (VAT fapiao supported), 3DS payments, sub-key caps and model allowlists for teams.",
            )}
          </p>
          <div className="badges">
            <div className="badge">
              <b className="num">99.5% SLA</b>
              <span>multi-provider failover · signed</span>
            </div>
            <div className="badge">
              <b>3DS · Stripe</b>
              <span>fraud-screened payments</span>
            </div>
            <div className="badge">
              <b>Invoices 48h</b>
              <span>enterprise invoicing API</span>
            </div>
            <div className="badge">
              <b>Ledger API</b>
              <span>every request, every token</span>
            </div>
            <div className="badge">
              <b>Sub-key governance</b>
              <span>caps · allowlists · alerts</span>
            </div>
          </div>
        </div>
      </section>
      <section className="v" id="models">
        <PixelGrid
          accent="#22C55E"
          cell={24}
          colors={["#7C3AED66", "#8B5CF64D", "#A78BFA59"]}
          cols={10}
          n={16}
          rows={5}
          seed={47}
          style={{ bottom: 56, right: 60, zIndex: 1 }}
        />
        <div className="bgv wall">
          {modelWallRows.map((row, rowIndex) => (
            <div className={`lane${rowIndex === 1 ? " r" : ""}`} key={rowIndex}>
              {[...row, ...row].map(
                ([name, meta, price, highlighted, verified], index) => (
                  <div
                    className="tile"
                    key={`${name}-${index}`}
                    style={
                      highlighted ? { borderColor: "#8B5CF666" } : undefined
                    }
                  >
                    <b style={highlighted ? { color: "#B7A3F0" } : undefined}>
                      {name}
                    </b>
                    <span>
                      {verified ? (
                        <>
                          <i
                            style={{
                              color: "var(--green)",
                              fontStyle: "normal",
                            }}
                          >
                            ✓ verified
                          </i>{" "}
                          · {meta}
                        </>
                      ) : (
                        meta
                      )}
                    </span>
                    <span className="pr">{modelPrice(price)}</span>
                  </div>
                ),
              )}
            </div>
          ))}
        </div>
        <div className="shade" />
        <div className="in">
          <div className="kick">
            {t("mdl.kick", "MODELS · 100+ OFFICIAL MODELS, ONE KEY")}
          </div>
          <h2 className="big">
            {renderInlineEm(
              t("mdl.h2", "Every frontier lab<em>.</em><br>One key."),
            )}
          </h2>
          <p className="d">
            {t(
              "mdl.p",
              "OpenAI, Anthropic, Google, DeepSeek, Alibaba, Zhipu, Moonshot, ByteDance… new models live within 24h of official release — always the real endpoints, never quantized re-serves.",
            )}
          </p>
        </div>
      </section>
      <section className="media" id="video">
        <PixelGrid
          cell={20}
          cols={10}
          n={11}
          rows={3}
          seed={103}
          style={{ right: 36, top: 14 }}
        />
        <div className="mediaIn">
          <div className="kick2">
            {t("vid.kick", "VIDEO MODELS · SAME KEY, SAME BALANCE")}
          </div>
          <h2 className="display">
            {t("vid.h2", "Video generation, official endpoints.")}
          </h2>
          <p className="sub" style={{ marginTop: 12, maxWidth: 620 }}>
            {t(
              "vid.sub",
              "Text-to-video and image-to-video from every major lab — metered per second on the same prepaid balance as your text models.",
            )}
          </p>
          <div className="vgal">
            {videoCards.map(
              ([asset, title, subtitle, meta, status, href, featured]) => (
                <Link
                  className={`vcard${featured ? " big" : ""}`}
                  href={localizePath(href, props.locale)}
                  key={asset}
                >
                  <video
                    autoPlay
                    muted
                    loop
                    playsInline
                    preload="auto"
                    poster={`/assets/video/${asset}.jpg`}
                    src={`/assets/video/${asset}.mp4`}
                  />
                  <div className="vshade" />
                  {featured && (
                    <span className="fbadge">
                      {t("vid.badge", "FEATURED · NEW")}
                    </span>
                  )}
                  <div className="vmeta">
                    <h3>
                      {title}
                      <br />
                      {subtitle}
                    </h3>
                    <p>
                      <span>{meta}</span>
                      <i>{videoStatus(status)}</i>
                    </p>
                  </div>
                </Link>
              ),
            )}
          </div>
          <div className="mgrid" style={{ marginTop: 16 }}>
            {upcomingVideoModels.map(([model, meta]) => (
              <div className="mcard" key={model}>
                <b>{model}</b>
                <span>{meta}</span>
                <i>{t("md.soon", "Coming soon")}</i>
              </div>
            ))}
            <div className="mcard more">
              <Link href={localizePath("/models", props.locale)}>
                {t("vid.more", "+ 20 more video models →")}
              </Link>
            </div>
          </div>
        </div>
      </section>
      <section className="support" id="support">
        <PixelGrid
          cell={18}
          cols={7}
          n={8}
          rows={3}
          seed={97}
          style={{ bottom: 20, left: 40 }}
        />
        <div className="supportIn">
          <div>
            <div className="kick2">
              {t("sup.kick", "SUPPORT · HUMANS ON CALL")}
            </div>
            <h2 className="display">{t("sup.h2", "Questions? Talk to us.")}</h2>
            <p className="sub" style={{ marginTop: 14, maxWidth: 420 }}>
              {t(
                "sup.sub",
                "Integration, model choice, or billing — online support and usage consultation for everything on flatkey.",
              )}
            </p>
          </div>
          <div className="chGrid">
            <a
              className="ch"
              href="https://discord.gg/VrbZFDXj5g"
              style={{
                gridColumn: "1/-1",
                background: "linear-gradient(120deg,#5865F2 0%,#4752C4 100%)",
                borderColor: "transparent",
              }}
            >
              <b style={{ color: "#fff" }}>
                {t("sup.dc", "Discord community")}
              </b>
              <span style={{ color: "#E4E6FF" }}>discord.gg/VrbZFDXj5g</span>
              <i style={{ color: "#C7CCFF" }}>
                {t(
                  "sup.dc.i",
                  "Builders answering builders — the team is in the channels daily",
                )}
              </i>
            </a>
            <a className="ch" href="mailto:support@flatkey.ai">
              <b>{t("sup.email", "Email")}</b>
              <span>support@flatkey.ai</span>
              <i>{t("sup.email.i", "Reply within 1 business day")}</i>
            </a>
            <Link className="ch" href={localizePath("/contact", props.locale)}>
              <b>{t("sup.chat", "Live chat")}</b>
              <span>{t("sup.chat.s", "Start chatting →")}</span>
              <i>{t("sup.chat.i", "Fastest answer, on the site")}</i>
            </Link>
            <a className="ch" href="https://x.com/flatkey101">
              <b>{t("sup.x", "X (Twitter)")}</b>
              <span>@flatkey101</span>
              <i>{t("sup.x.i", "Follow or DM")}</i>
            </a>
          </div>
        </div>
      </section>
      <section className="ctaWrap">
        <div className="ctaBanner">
          <PixelGrid
            accent="#67E8F9"
            cell={26}
            colors={["#7C3AED", "#A78BFA", "#6D28D9"]}
            cols={9}
            n={26}
            rows={7}
            seed={163}
            style={{ left: -30, opacity: 0.9, top: -20 }}
          />
          <PixelGrid
            accent="#67E8F9"
            cell={26}
            colors={["#A78BFA", "#C4B5FD", "#7C3AED"]}
            cols={9}
            n={26}
            rows={7}
            seed={167}
            style={{ bottom: -20, opacity: 0.9, right: -30 }}
          />
          <div className="ctaIn">
            <h2>
              {renderLineBreakText(
                t("cta.h", "More AI, less cost —<br>on every official model"),
              )}
            </h2>
            <p>
              {t(
                "cta.sub",
                "Whether you ship an agent today or route billions of tokens a month — one key, a signed SLA, and prices that drop as you grow.",
              )}
            </p>
            <div className="ctaBtns">
              <Link className="btn white big" href={authActionHref}>
                {finalCtaLabel}
              </Link>
              <Link
                className="btn black big"
                href={localizePath("/contact", props.locale)}
              >
                {t("cta.b2", "Contact Sales")}
              </Link>
            </div>
          </div>
        </div>
      </section>
    </OnlineStaticShell>
  );
}
