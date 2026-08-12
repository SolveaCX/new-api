import Link from "next/link";
import { HomePriceHealthScore } from "@/components/home-price-health-score";
import { ModelLogo } from "@/components/pricing-model-browser";
import type { HomeCopy } from "@/lib/home-copy";
import { buildRowsForModels, modelIconKey, type HomePricedModel } from "@/lib/home-models";
import type { Locale } from "@/lib/locales";
import { localizePath, withIdFallback } from "@/lib/locales";
import { modelPublicPath } from "@/lib/model-public";
import {
  buildEffectiveGroupRatio,
  discountedPriceUsd,
  getAvailableGroups,
  getBestGroupRatio,
  getGroupModelRatioForModel,
  getOfficialPriceUsd,
  getVendorName,
  type PricingData,
  type PricingModel,
} from "@/lib/pricing";

type PriceComparisonCopy = {
  modelTags: string[];
  featuredModelsCta: string;
  officialPriceLabel: string;
  flatkeyPriceLabel: string;
  priceEyebrow: string;
  priceTitle: string;
  priceDescription: string;
  priceTableNote: string;
};

const HOME_PRICE_COMPARISON_COPY: Record<Locale, PriceComparisonCopy> = withIdFallback({
  en: {
    modelTags: ["GPT-5 text & code", "Claude agents", "Seedance video", "ElevenLabs voice", "DeepSeek reasoning", "Kimi long context", "GLM stack"],
    featuredModelsCta: "Explore all models",
    officialPriceLabel: "Official",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Official model price comparison",
    priceTitle: "Model price comparison",
    priceDescription: "Compare official model prices with flatkey's after-bonus price across text, image, video, voice, and reasoning calls.",
    priceTableNote: "After-bonus pricing combines supported model discounts with prepaid recharge credit.",
  },
  zh: {
    modelTags: ["GPT-5 文本与代码", "Claude Agent", "Seedance 视频", "ElevenLabs 语音", "DeepSeek 推理", "Kimi 长上下文", "GLM 模型栈"],
    featuredModelsCta: "探索全部模型",
    officialPriceLabel: "官网",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "官方模型价格对比",
    priceTitle: "模型与官网价格对比",
    priceDescription: "对比文本、图像、视频、语音、推理模型的官网价格与 flatkey 充值后的实际价格。",
    priceTableNote: "充值后价格会叠加支持模型的折扣与预付充值赠送额度。",
  },
  es: {
    modelTags: ["GPT-5 texto y código", "agentes Claude", "video Seedance", "voz ElevenLabs", "razonamiento DeepSeek", "contexto largo Kimi", "stack GLM"],
    featuredModelsCta: "Explorar todos los modelos",
    officialPriceLabel: "Oficial",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Comparación con precio oficial",
    priceTitle: "Comparación de precios de modelos",
    priceDescription: "Compara precios oficiales con el precio after-bonus de flatkey en texto, imagen, video, voz y razonamiento.",
    priceTableNote: "El precio after-bonus combina descuentos compatibles y crédito de recarga prepago.",
  },
  fr: {
    modelTags: ["GPT-5 texte et code", "agents Claude", "vidéo Seedance", "voix ElevenLabs", "raisonnement DeepSeek", "long contexte Kimi", "stack GLM"],
    featuredModelsCta: "Explorer tous les modèles",
    officialPriceLabel: "Officiel",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Comparaison avec les prix officiels",
    priceTitle: "Comparaison des prix modèles",
    priceDescription: "Comparez les prix officiels avec le prix after-bonus flatkey pour texte, image, vidéo, voix et raisonnement.",
    priceTableNote: "Le prix after-bonus combine remises modèles et crédit de recharge prépayé.",
  },
  pt: {
    modelTags: ["GPT-5 texto e código", "agentes Claude", "vídeo Seedance", "voz ElevenLabs", "raciocínio DeepSeek", "contexto longo Kimi", "stack GLM"],
    featuredModelsCta: "Explorar todos os modelos",
    officialPriceLabel: "Oficial",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Comparação com preço oficial",
    priceTitle: "Comparação de preços dos modelos",
    priceDescription: "Compare preços oficiais com o preço after-bonus da flatkey em texto, imagem, vídeo, voz e raciocínio.",
    priceTableNote: "O preço after-bonus combina descontos de modelo e crédito pré-pago.",
  },
  ru: {
    modelTags: ["GPT-5 text & code", "Claude agents", "Seedance video", "ElevenLabs voice", "DeepSeek reasoning", "Kimi long context", "GLM stack"],
    featuredModelsCta: "Смотреть все модели",
    officialPriceLabel: "Официально",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Сравнение с официальными ценами",
    priceTitle: "Сравнение цен моделей",
    priceDescription: "Сравните официальные цены и after-bonus цену flatkey для text, image, video, voice и reasoning вызовов.",
    priceTableNote: "After-bonus цена сочетает скидки моделей и prepaid recharge credit.",
  },
  ja: {
    modelTags: ["GPT-5 テキストとコード", "Claude エージェント", "Seedance 動画", "ElevenLabs 音声", "DeepSeek 推論", "Kimi 長文脈", "GLM スタック"],
    featuredModelsCta: "すべてのモデルを見る",
    officialPriceLabel: "公式",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "公式価格との比較",
    priceTitle: "モデル価格比較",
    priceDescription: "テキスト、画像、動画、音声、推論の公式価格と flatkey の after-bonus 価格を比較します。",
    priceTableNote: "after-bonus 価格はモデル割引とプリペイド特典を組み合わせます。",
  },
  vi: {
    modelTags: ["GPT-5 text & code", "Claude agents", "Seedance video", "ElevenLabs voice", "DeepSeek reasoning", "Kimi long context", "GLM stack"],
    featuredModelsCta: "Khám phá tất cả model",
    officialPriceLabel: "Chính thức",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "So sánh với giá chính thức",
    priceTitle: "So sánh giá model",
    priceDescription: "So sánh giá chính thức với giá after-bonus của flatkey cho text, image, video, voice và reasoning.",
    priceTableNote: "Giá after-bonus kết hợp giảm giá model và credit nạp trả trước.",
  },
  de: {
    modelTags: ["GPT-5 Text & Code", "Claude Agents", "Seedance Video", "ElevenLabs Voice", "DeepSeek Reasoning", "Kimi Long Context", "GLM Stack"],
    featuredModelsCta: "Alle Modelle ansehen",
    officialPriceLabel: "Offiziell",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Vergleich mit offiziellen Preisen",
    priceTitle: "Modellpreisvergleich",
    priceDescription: "Vergleiche offizielle Preise mit flatkeys after-bonus Preis für Text-, Bild-, Video-, Voice- und Reasoning-Aufrufe.",
    priceTableNote: "After-bonus Preise kombinieren Modellrabatte mit Prepaid-Aufladeguthaben.",
  },
  id: {
    modelTags: ["GPT-5 teks & kode", "Agen Claude", "Video Seedance", "Voice ElevenLabs", "Reasoning DeepSeek", "Kimi konteks panjang", "Stack GLM"],
    featuredModelsCta: "Lihat semua model",
    officialPriceLabel: "Resmi",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Perbandingan harga model resmi",
    priceTitle: "Perbandingan harga model",
    priceDescription: "Bandingkan harga resmi dengan harga after-bonus flatkey untuk panggilan teks, gambar, video, suara, dan reasoning.",
    priceTableNote: "Harga after-bonus menggabungkan diskon model yang didukung dengan kredit top up prabayar.",
  },
});

type HomePriceComparisonRow = {
  discountLabel: string;
} & HomePricedModel;

type HomePriceSeedRow = {
  discountLabel: string;
  flatkeyPrice: string;
  iconKey?: string;
  model: string;
  officialPrice: string;
  vendor: string;
};

const HOME_PRICE_MODEL_ROWS: HomePriceSeedRow[] = [
  { model: "MiniMax-H3", vendor: "MiniMax", flatkeyPrice: "$0.08 / req", officialPrice: "$0.08 / req", discountLabel: "0%" },
  { model: "Seedance2.0", vendor: "ByteDance", flatkeyPrice: "$6.3 / billing unit", officialPrice: "$7 / billing unit", discountLabel: "10%" },
  { model: "kimi-k3", vendor: "Moonshot", flatkeyPrice: "in $1.8 / out $9 / 1M tokens", officialPrice: "in $3 / out $15 / 1M tokens", discountLabel: "40%" },
  { model: "gpt-5.6-sol", vendor: "OpenAI", flatkeyPrice: "in $1.5 / out $9 / 1M tokens", officialPrice: "in $5 / out $30 / 1M tokens", discountLabel: "70%" },
  { model: "gpt-4o-mini", vendor: "OpenAI", flatkeyPrice: "in $0.135 / out $0.54 / 1M tokens", officialPrice: "in $0.15 / out $0.6 / 1M tokens", discountLabel: "10%" },
  { model: "claude-opus-5", vendor: "Anthropic", flatkeyPrice: "in $4.5 / out $22.5 / 1M tokens", officialPrice: "in $5 / out $25 / 1M tokens", discountLabel: "10%" },
  { model: "deepseek-v4-flash", vendor: "DeepSeek", flatkeyPrice: "in $0.084 / out $0.168 / 1M tokens", officialPrice: "in $0.14 / out $0.28 / 1M tokens", discountLabel: "40%" },
  { model: "claude-opus-4-6", vendor: "Anthropic", flatkeyPrice: "in $4.5 / out $22.5 / 1M tokens", officialPrice: "in $5 / out $25 / 1M tokens", discountLabel: "10%" },
  { model: "gpt-5.6-luna", vendor: "OpenAI", flatkeyPrice: "in $0.06 / out $0.36 / 1M tokens", officialPrice: "in $0.2 / out $1.2 / 1M tokens", discountLabel: "70%" },
  { model: "claude-sonnet-5", vendor: "Anthropic", flatkeyPrice: "in $1.8 / out $9 / 1M tokens", officialPrice: "in $2 / out $10 / 1M tokens", discountLabel: "10%" },
  { model: "claude-opus-4-8", vendor: "Anthropic", flatkeyPrice: "in $4.5 / out $22.5 / 1M tokens", officialPrice: "in $5 / out $25 / 1M tokens", discountLabel: "10%" },
  { model: "gemini-3.6-flash", vendor: "Google", flatkeyPrice: "in $1.35 / out $6.75 / 1M tokens", officialPrice: "in $1.5 / out $7.5 / 1M tokens", discountLabel: "10%" },
  { model: "gpt-5.4", vendor: "OpenAI", flatkeyPrice: "in $0.75 / out $4.5 / 1M tokens", officialPrice: "in $2.5 / out $15 / 1M tokens", discountLabel: "70%" },
  { model: "glm-5.2", vendor: "Z.ai", flatkeyPrice: "in $0.84 / out $2.64 / 1M tokens", officialPrice: "in $1.4 / out $4.4 / 1M tokens", discountLabel: "40%" },
];

const HOME_PRICE_TABLE_COPY = withIdFallback({
  en: {
    discount: "discount",
    health: "Health Score",
    save: "SAVE",
  },
  zh: {
    discount: "折扣",
    health: "健康评分",
    save: "节省",
  },
  es: {
    discount: "descuento",
    health: "Puntuación de salud",
    save: "AHORRA",
  },
  fr: {
    discount: "remise",
    health: "Score de santé",
    save: "ÉCO",
  },
  pt: {
    discount: "desconto",
    health: "Pontuação de saúde",
    save: "POUPE",
  },
  ru: {
    discount: "скидка",
    health: "Оценка здоровья",
    save: "SAVE",
  },
  ja: {
    discount: "割引",
    health: "健全性スコア",
    save: "節約",
  },
  vi: {
    discount: "ưu đãi",
    health: "Điểm sức khỏe",
    save: "TIẾT KIỆM",
  },
  de: {
    discount: "Rabatt",
    health: "Gesundheitswert",
    save: "SPAREN",
  },
  id: {
    discount: "diskon",
    health: "Skor kesehatan",
    save: "HEMAT",
  },
});

export function buildHomePriceComparisonRows(data: PricingData, limit = HOME_PRICE_MODEL_ROWS.length): HomePriceComparisonRow[] {
  const models = selectHomePriceModels(prepareHomePriceModels(data));
  const modelsByName = new Map(models.map((model) => [normalizeModelName(model.model_name), model]));
  const rowsByName = new Map(buildRowsForModels(models, data.vendors, data.groupRatio).map((row) => [row.name, row]));

  return HOME_PRICE_MODEL_ROWS.slice(0, limit)
    .map((seed) => {
      const model = modelsByName.get(normalizeModelName(seed.model));
      const liveRow = model ? rowsByName.get(model.model_name) : undefined;
      return {
        name: seed.model,
        vendor: liveRow?.vendor ?? seed.vendor,
        official: liveRow?.official ?? seed.officialPrice,
        discounted: liveRow?.discounted ?? seed.flatkeyPrice,
        iconKey: liveRow?.iconKey ?? seed.iconKey ?? modelIconKey(seed.model, seed.vendor),
        discountLabel: model ? formatHomePriceDiscount(model, data.groupRatio) : seed.discountLabel,
      };
    })
    .filter((row) => row.official !== "-" && row.discounted !== "-");
}

export function HomePriceComparisonSection(props: {
  home: HomeCopy;
  locale: Locale;
  rows: HomePriceComparisonRow[];
}) {
  const copy = HOME_PRICE_COMPARISON_COPY[props.locale] ?? HOME_PRICE_COMPARISON_COPY.en;
  const tableCopy = HOME_PRICE_TABLE_COPY[props.locale] ?? HOME_PRICE_TABLE_COPY.en;

  if (props.rows.length === 0) return null;

  return (
    <section className="fk-home-price-section" aria-labelledby="home-price-comparison-title">
      <style>{HOME_PRICE_COMPARISON_STYLES}</style>
      <div aria-hidden className="fk-home-price-grid" />
      <div className="fk-home-price-inner">
        <div className="fk-home-price-head">
          <div className="fk-home-price-title-block">
            <p className="fk-home-price-eyebrow">{copy.priceEyebrow}</p>
            <h2 id="home-price-comparison-title">{copy.priceTitle}</h2>
          </div>
          <p className="fk-home-price-description">{copy.priceDescription}</p>
        </div>

        <div className="fk-home-price-tags">
          {copy.modelTags.map((tag) => (
            <span key={tag}>{tag}</span>
          ))}
        </div>

        <div className="fk-home-price-body">
          <aside className="fk-home-price-spotlight">
            <p>{props.home.compare.title}</p>
            <strong>{props.home.compare.badge}</strong>
            <span>{props.home.compare.save}</span>
          </aside>

          <div className="fk-home-price-board">
            <div className="fk-home-price-board-head">
              <span>{copy.priceTitle}</span>
              <span>{copy.flatkeyPriceLabel}<small>{props.home.table.perMillion}</small></span>
              <span>{copy.officialPriceLabel}<small>{props.home.table.perMillion}</small></span>
              <span>{tableCopy.discount}</span>
              <span>{tableCopy.health}</span>
            </div>
            {props.rows.map((row) => (
              <FeaturedModelRow
                key={row.name}
                row={row}
                locale={props.locale}
                tableCopy={tableCopy}
                flatkeyPriceLabel={copy.flatkeyPriceLabel}
                officialPriceLabel={copy.officialPriceLabel}
              />
            ))}
          </div>
        </div>

        <div className="fk-home-price-foot">
          <p>{copy.priceTableNote}</p>
          <Link href={localizePath("/models", props.locale)}>
            {copy.featuredModelsCta}
            <span aria-hidden="true">→</span>
          </Link>
        </div>
      </div>
    </section>
  );
}

function FeaturedModelRow(props: {
  row: HomePriceComparisonRow;
  locale: Locale;
  tableCopy: (typeof HOME_PRICE_TABLE_COPY)[Locale];
  flatkeyPriceLabel: string;
  officialPriceLabel: string;
}) {
  const href = localizePath(modelPublicPath(props.row.name), props.locale);

  return (
    <Link
      href={href}
      className="fk-home-price-row"
      aria-label={`Open ${props.row.name} model page`}
    >
      <div className="fk-home-price-model">
        <ModelLogoSurface iconKey={props.row.iconKey} fallback={props.row.name.charAt(0).toUpperCase()} />
        <div>
          <strong>{props.row.name}</strong>
          <span>{props.row.vendor}</span>
        </div>
      </div>
      <span className="fk-home-price-flat" data-label={props.flatkeyPriceLabel}>{props.row.discounted}</span>
      <span className="fk-home-price-official" data-label={props.officialPriceLabel}>{props.row.official}</span>
      <span className="fk-home-price-discount" data-label={props.tableCopy.discount}>
        <span>{props.tableCopy.save}</span>
        {props.row.discountLabel}
      </span>
      <span className="fk-home-price-health-cell" data-label={props.tableCopy.health}>
        <HomePriceHealthScore label={props.tableCopy.health} modelName={props.row.name} />
      </span>
    </Link>
  );
}

function ModelLogoSurface(props: { iconKey?: string; fallback: string }) {
  return (
    <span className="fk-home-price-logo">
      <ModelLogo iconKey={props.iconKey} fallback={props.fallback} size={28} />
    </span>
  );
}

function prepareHomePriceModels(data: PricingData): PricingModel[] {
  return data.models.map((model) => {
    const effectiveGroupRatio = buildEffectiveGroupRatio(model, data.groupRatio, data.groupModelRatio);
    const enrichedModel = {
      ...model,
      vendor_name: getVendorName(model, data.vendors),
      vendor_icon: model.vendor_icon ?? data.vendors.find((vendor) => vendor.id === model.vendor_id)?.icon,
      vendor_description: model.vendor_description ?? data.vendors.find((vendor) => vendor.id === model.vendor_id)?.description,
      group_ratio: effectiveGroupRatio,
      group_model_ratio: getGroupModelRatioForModel(model.model_name, data.groupModelRatio),
    };
    return {
      ...enrichedModel,
      enable_groups: getAvailableGroups(enrichedModel, data.groupRatio, data.usableGroup),
    };
  });
}

function selectHomePriceModels(models: PricingModel[]): PricingModel[] {
  const priced = models.filter((model) => getOfficialPriceUsd(model) > 0);
  const byName = new Map(priced.map((model) => [model.model_name.toLowerCase(), model]));
  const selected: PricingModel[] = [];
  const seen = new Set<string>();

  for (const { model: modelName } of HOME_PRICE_MODEL_ROWS) {
    const model = byName.get(modelName.toLowerCase());
    if (!model || seen.has(model.model_name)) continue;
    selected.push(model);
    seen.add(model.model_name);
  }

  return selected;
}

function formatHomePriceDiscount(model: PricingModel, groupRatio: Record<string, number>): string {
  const official = getOfficialPriceUsd(model);
  if (!Number.isFinite(official) || official <= 0) return "0%";
  const listed = official * getBestGroupRatio(model, groupRatio);
  const discounted = discountedPriceUsd(listed);
  const percent = Math.max(0, Math.round((1 - discounted / official) * 100));
  return `${percent}%`;
}

function normalizeModelName(value: string): string {
  return value.toLowerCase();
}

const HOME_PRICE_COMPARISON_STYLES = `
.fk-home-price-section,
.fk-home-price-section * {
  box-sizing: border-box;
}

.fk-home-price-section {
  position: relative;
  z-index: 1;
  overflow: hidden;
  border-top: 1px solid var(--line, rgba(11, 11, 15, 0.08));
  border-bottom: 1px solid var(--line, rgba(11, 11, 15, 0.08));
  background: #fff;
  color: var(--ink, #0b0b0f);
}

.fk-home-price-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(to right, rgba(124, 58, 237, 0.055) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(124, 58, 237, 0.05) 1px, transparent 1px);
  background-size: 72px 72px;
  mask-image: linear-gradient(180deg, rgba(0, 0, 0, 0.78), transparent 88%);
}

.fk-home-price-inner {
  position: relative;
  z-index: 1;
  width: min(100%, 1240px);
  margin: 0 auto;
  padding: 76px 40px 82px;
}

.fk-home-price-head {
  display: grid;
  grid-template-columns: minmax(0, 0.9fr) minmax(360px, 1fr);
  gap: 40px;
  align-items: end;
}

.fk-home-price-title-block {
  min-width: 0;
}

.fk-home-price-eyebrow {
  margin: 0 0 14px;
  color: var(--violet-deep, #4c1d95);
  font: 700 12px/1.2 var(--mono, ui-monospace, monospace);
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.fk-home-price-title-block h2 {
  margin: 0;
  max-width: 660px;
  font-family: var(--disp, system-ui, sans-serif);
  font-size: clamp(38px, 4.2vw, 58px);
  font-weight: 750;
  letter-spacing: -0.055em;
  line-height: 0.98;
  text-wrap: balance;
}

.fk-home-price-description {
  margin: 0;
  max-width: 680px;
  color: var(--ink2, #43434c);
  font-size: 16.5px;
  font-weight: 500;
  line-height: 1.7;
}

.fk-home-price-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 24px;
}

.fk-home-price-tags span {
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  border: 1px solid rgba(16, 16, 20, 0.12);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.82);
  padding: 7px 12px;
  color: var(--ink2, #43434c);
  font-size: 12.5px;
  font-weight: 700;
  box-shadow: 0 10px 24px -22px rgba(46, 16, 101, 0.38);
}

.fk-home-price-body {
  display: grid;
  grid-template-columns: minmax(260px, 0.34fr) minmax(0, 1fr);
  gap: 18px;
  align-items: stretch;
  margin-top: 28px;
}

.fk-home-price-spotlight {
  min-width: 0;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 18px;
  background:
    radial-gradient(circle at 100% 0%, rgba(217, 239, 110, 0.18), transparent 36%),
    linear-gradient(145deg, #5b21b6 0%, #3b0764 100%);
  padding: 24px;
  color: #fff;
  box-shadow: 0 26px 62px -34px rgba(46, 16, 101, 0.58);
}

.fk-home-price-spotlight p {
  margin: 0;
  color: rgba(255, 255, 255, 0.68);
  font: 750 11px/1.2 var(--mono, ui-monospace, monospace);
  letter-spacing: 0.11em;
  text-transform: uppercase;
}

.fk-home-price-spotlight strong {
  display: block;
  margin-top: 18px;
  color: #fff;
  font-family: var(--disp, system-ui, sans-serif);
  font-size: clamp(42px, 4.4vw, 64px);
  font-weight: 760;
  letter-spacing: -0.055em;
  line-height: 0.9;
}

.fk-home-price-spotlight span {
  display: block;
  margin-top: 18px;
  color: rgba(255, 255, 255, 0.78);
  font-size: 13.5px;
  font-weight: 550;
  line-height: 1.65;
}

.fk-home-price-board {
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgba(11, 11, 15, 0.10);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 26px 68px -42px rgba(46, 16, 101, 0.38);
}

.fk-home-price-board-head,
.fk-home-price-row {
  display: grid;
  grid-template-columns: minmax(190px, 1.46fr) minmax(145px, 1fr) minmax(145px, 0.95fr) minmax(96px, 0.58fr) minmax(132px, 0.78fr);
  gap: 14px;
  align-items: center;
}

.fk-home-price-board-head {
  min-height: 42px;
  border-bottom: 1px solid rgba(11, 11, 15, 0.08);
  background: #f7f6fb;
  padding: 0 16px;
  color: var(--ink3, #83838e);
  font: 750 10.5px/1.25 var(--mono, ui-monospace, monospace);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.fk-home-price-board-head small {
  display: block;
  margin-top: 2px;
  color: rgba(131, 131, 142, 0.82);
  font-size: 9px;
  font-weight: 650;
  letter-spacing: 0.02em;
  text-transform: none;
}

.fk-home-price-row {
  min-height: 68px;
  border-bottom: 1px solid rgba(16, 16, 20, 0.08);
  padding: 12px 16px;
  color: inherit;
  text-decoration: none;
  transition: background-color 160ms ease, border-color 160ms ease;
}

.fk-home-price-row:last-child {
  border-bottom: 0;
}

.fk-home-price-row:hover {
  background: rgba(247, 246, 251, 0.92);
  border-color: rgba(124, 58, 237, 0.22);
}

.fk-home-price-model {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 12px;
}

.fk-home-price-model strong {
  display: block;
  min-width: 0;
  overflow: hidden;
  color: var(--ink, #0b0b0f);
  font-size: 15px;
  font-weight: 800;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fk-home-price-model span {
  display: block;
  margin-top: 3px;
  color: var(--ink3, #83838e);
  font: 700 10.5px/1.2 var(--mono, ui-monospace, monospace);
}

.fk-home-price-logo {
  display: inline-flex;
  flex: none;
  width: 42px;
  height: 42px;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(16, 16, 20, 0.1);
  border-radius: 11px;
  background: #fff;
  color: var(--violet-deep, #4c1d95);
  box-shadow: 0 10px 24px -18px rgba(16, 16, 20, 0.45);
}

.fk-home-price-logo img {
  display: block;
  width: 28px;
  height: 28px;
  object-fit: contain;
}

.fk-home-price-logo > span {
  font-family: var(--disp, system-ui, sans-serif);
  font-size: 15px;
  font-weight: 850;
}

.fk-home-price-flat,
.fk-home-price-official,
.fk-home-price-discount,
.fk-home-price-health-cell {
  min-width: 0;
  font-family: var(--mono, ui-monospace, monospace);
  font-variant-numeric: tabular-nums;
}

.fk-home-price-flat {
  color: #5852ff;
  font-size: 13.5px;
  font-weight: 800;
  line-height: 1.45;
}

.fk-home-price-official {
  color: #8c8c97;
  font-size: 12.5px;
  font-weight: 650;
  line-height: 1.45;
  text-decoration: line-through;
}

.fk-home-price-discount {
  display: inline-flex;
  width: fit-content;
  align-items: center;
  gap: 5px;
  border-radius: 999px;
  background: rgba(21, 128, 61, 0.08);
  padding: 5px 8px;
  color: var(--green, #15803d);
  font-size: 12.5px;
  font-weight: 900;
  white-space: nowrap;
}

.fk-home-price-discount span {
  color: rgba(21, 128, 61, 0.72);
  font-size: 9px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.fk-home-price-health-cell {
  display: flex;
  justify-content: flex-start;
}

.fk-home-health {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--green, #15803d);
  font-size: 12.5px;
  font-weight: 850;
}

.fk-home-health-bars {
  display: inline-flex;
  width: 58px;
  height: 27px;
  align-items: end;
  justify-content: center;
  gap: 4px;
  border: 1px solid rgba(16, 185, 129, 0.10);
  border-radius: 9px;
  background: linear-gradient(180deg, rgba(16, 185, 129, 0.08), rgba(16, 185, 129, 0.035));
  padding: 6px 7px;
}

.fk-home-health-bars span {
  display: block;
  width: 5px;
  border-radius: 2px;
  background: linear-gradient(180deg, #34d399 0%, #10b981 70%, #059669 100%);
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.22);
}

.fk-home-price-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-top: 22px;
}

.fk-home-price-foot p {
  margin: 0;
  max-width: 620px;
  color: #6a6a75;
  font-size: 13.5px;
  font-weight: 550;
  line-height: 1.65;
}

.fk-home-price-foot a {
  display: inline-flex;
  flex: none;
  min-height: 44px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: 10px;
  background: var(--violet, #5b21b6);
  padding: 0 20px;
  color: #fff;
  font-size: 13.5px;
  font-weight: 800;
  text-decoration: none;
  transition: transform 160ms ease, background-color 160ms ease;
}

.fk-home-price-foot a:hover {
  transform: translateY(-1px);
  background: var(--violet-deep, #4c1d95);
}

@media (max-width: 1050px) {
  .fk-home-price-head,
  .fk-home-price-body {
    grid-template-columns: 1fr;
  }

  .fk-home-price-description {
    max-width: 760px;
  }
}

@media (max-width: 760px) {
  .fk-home-price-inner {
    padding: 62px 20px 68px;
  }

  .fk-home-price-title-block h2 {
    font-size: clamp(34px, 9vw, 44px);
    letter-spacing: -0.045em;
  }

  .fk-home-price-description {
    font-size: 15px;
  }

  .fk-home-price-tags {
    gap: 7px;
  }

  .fk-home-price-tags span {
    min-height: 31px;
    padding: 6px 10px;
    font-size: 11.5px;
  }

  .fk-home-price-spotlight {
    padding: 21px 18px;
  }

  .fk-home-price-board-head {
    display: none;
  }

  .fk-home-price-row {
    grid-template-columns: minmax(0, 1fr);
    gap: 9px;
    min-height: 0;
    padding: 15px 16px;
  }

  .fk-home-price-flat,
  .fk-home-price-official,
  .fk-home-price-discount,
  .fk-home-price-health-cell {
    font-size: 12.5px;
    line-height: 1.45;
  }

  .fk-home-price-flat::before,
  .fk-home-price-official::before,
  .fk-home-price-discount::before,
  .fk-home-price-health-cell::before {
    display: block;
    margin-bottom: 2px;
    color: var(--ink3, #83838e);
    font: 750 10px/1.2 var(--mono, ui-monospace, monospace);
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .fk-home-price-flat::before {
    content: attr(data-label);
  }

  .fk-home-price-official::before {
    content: attr(data-label);
  }

  .fk-home-price-discount::before {
    content: attr(data-label);
  }

  .fk-home-price-health-cell {
    display: block;
  }

  .fk-home-price-health-cell::before {
    content: attr(data-label);
  }

  .fk-home-price-foot {
    align-items: stretch;
    flex-direction: column;
  }

  .fk-home-price-foot a {
    width: 100%;
  }
}
`;
