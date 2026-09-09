import Link from "next/link";
import { ArrowRight, Sparkles } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { ModelLogo } from "@/components/pricing-model-browser";
import {
  getModelCollectionCopy,
  getAvailableModelCollections,
  modelCardData,
  type ModelCollectionDefinition,
  selectCollectionModels,
} from "@/lib/model-collections";
import { localizePath, type Locale } from "@/lib/locales";
import { displayTokens, type RankingsData } from "@/lib/rankings-live";
import { type PricingData } from "@/lib/pricing";
import { buildCollectionDetailSchema, buildCollectionsIndexSchema, stringifyJsonLd } from "@/lib/schema";

const shellClass = "fk-site-frame";
const detailShellClass = "fk-site-frame max-w-[1160px]";
const COLLECTION_DISPLAY_ORDER = [
  "image-generation",
  "free-models",
  "discounted-models",
  "tool-calling",
  "coding",
  "roleplay-creative-writing",
  "vision-models",
  "openclaw-models",
  "text-embedding-models",
  "video-generation",
  "audio-generation-models",
  "text-to-speech-models",
  "speech-to-text-models",
  "rerank-models",
  "general-purpose-models",
] as const;

const uiCopy = {
  en: {
    collections: "Collections",
    heroTitle: "Find the right AI model for the job.",
    heroDescription: "Browse curated collections of models for coding, image generation, video, tool calling, and more. Each collection links to live model details, usage, and public pricing.",
    updated: "Updated with live catalog data",
    browse: "Browse collection",
    allModels: "Browse all models",
    compare: "Compare models",
    featured: "Featured models",
    usage: "weekly usage",
    context: "context",
    from: "from",
    explore: "Explore more collections",
    criteria: "How this collection is built",
    capabilities: "Live catalog capabilities",
    signals: "Usage and pricing signals",
    details: "View model details",
    openConsole: "Open console",
    browsePricing: "Browse pricing",
  },
  zh: {
    collections: "模型集合",
    heroTitle: "为每项任务找到合适的 AI 模型。",
    heroDescription: "浏览适合编程、图像生成、视频、工具调用等场景的模型集合，直接查看实时模型详情、调用量和公开价格。",
    updated: "根据实时模型目录更新",
    browse: "查看集合",
    allModels: "浏览全部模型",
    compare: "比较模型",
    featured: "精选模型",
    usage: "周调用量",
    context: "上下文",
    from: "起",
    explore: "探索更多集合",
    criteria: "集合如何生成",
    capabilities: "实时目录能力",
    signals: "调用量与价格信号",
    details: "查看模型详情",
    openConsole: "打开控制台",
    browsePricing: "浏览价格",
  },
  es: { collections: "Colecciones", heroTitle: "Encuentra el modelo de IA adecuado para cada tarea.", heroDescription: "Explora colecciones de modelos para programación, imágenes, vídeo, herramientas y más, con detalles, uso y precios públicos.", updated: "Actualizado con datos del catálogo", browse: "Ver colección", allModels: "Ver todos los modelos", compare: "Comparar modelos", featured: "Modelos destacados", usage: "uso semanal", context: "contexto", from: "desde", explore: "Explora más colecciones", criteria: "Cómo se crea esta colección", capabilities: "Capacidades del catálogo", signals: "Señales de uso y precio", details: "Ver detalles del modelo", openConsole: "Abrir consola", browsePricing: "Ver precios" },
  fr: { collections: "Collections", heroTitle: "Trouvez le modèle IA adapté à chaque tâche.", heroDescription: "Explorez des collections pour le code, l’image, la vidéo, les outils et plus, avec détails, usage et tarifs publics.", updated: "Mis à jour avec les données du catalogue", browse: "Voir la collection", allModels: "Voir tous les modèles", compare: "Comparer les modèles", featured: "Modèles sélectionnés", usage: "utilisation hebdomadaire", context: "contexte", from: "à partir de", explore: "Explorer d’autres collections", criteria: "Comment cette collection est créée", capabilities: "Capacités du catalogue", signals: "Signaux d’usage et de prix", details: "Voir les détails du modèle", openConsole: "Ouvrir la console", browsePricing: "Voir les tarifs" },
  pt: { collections: "Coleções", heroTitle: "Encontre o modelo de IA certo para cada tarefa.", heroDescription: "Explore coleções para programação, imagens, vídeo, ferramentas e muito mais, com detalhes, uso e preços públicos.", updated: "Atualizado com dados do catálogo", browse: "Ver coleção", allModels: "Ver todos os modelos", compare: "Comparar modelos", featured: "Modelos em destaque", usage: "uso semanal", context: "contexto", from: "a partir de", explore: "Explore mais coleções", criteria: "Como esta coleção é criada", capabilities: "Capacidades do catálogo", signals: "Sinais de uso e preço", details: "Ver detalhes do modelo", openConsole: "Abrir console", browsePricing: "Ver preços" },
  ru: { collections: "Подборки", heroTitle: "Найдите подходящую ИИ-модель для каждой задачи.", heroDescription: "Изучайте подборки для программирования, изображений, видео, инструментов и других сценариев с актуальными ценами и данными.", updated: "Обновляется по данным каталога", browse: "Открыть подборку", allModels: "Все модели", compare: "Сравнить модели", featured: "Избранные модели", usage: "за неделю", context: "контекст", from: "от", explore: "Другие подборки", criteria: "Как формируется подборка", capabilities: "Возможности каталога", signals: "Сигналы использования и цены", details: "Подробнее о модели", openConsole: "Открыть консоль", browsePricing: "Цены" },
  ja: { collections: "コレクション", heroTitle: "タスクに合った AI モデルを見つけましょう。", heroDescription: "コーディング、画像、動画、ツール呼び出しなどのモデルを、詳細・利用量・公開価格とともに比較できます。", updated: "最新のカタログデータで更新", browse: "コレクションを見る", allModels: "すべてのモデル", compare: "モデルを比較", featured: "注目のモデル", usage: "週間利用量", context: "コンテキスト", from: "から", explore: "他のコレクション", criteria: "このコレクションの基準", capabilities: "カタログの機能", signals: "利用量と価格のシグナル", details: "モデルの詳細を見る", openConsole: "コンソールを開く", browsePricing: "料金を見る" },
  vi: { collections: "Bộ sưu tập", heroTitle: "Tìm mô hình AI phù hợp cho từng công việc.", heroDescription: "Khám phá bộ sưu tập cho lập trình, hình ảnh, video, gọi công cụ và hơn thế nữa với dữ liệu, lượt dùng và giá công khai.", updated: "Cập nhật theo catalog trực tiếp", browse: "Xem bộ sưu tập", allModels: "Xem tất cả mô hình", compare: "So sánh mô hình", featured: "Mô hình nổi bật", usage: "lượt dùng mỗi tuần", context: "ngữ cảnh", from: "từ", explore: "Khám phá bộ sưu tập khác", criteria: "Cách tạo bộ sưu tập", capabilities: "Khả năng trong catalog", signals: "Tín hiệu sử dụng và giá", details: "Xem chi tiết mô hình", openConsole: "Mở console", browsePricing: "Xem giá" },
  de: { collections: "Sammlungen", heroTitle: "Finden Sie das passende KI-Modell für jede Aufgabe.", heroDescription: "Entdecken Sie Sammlungen für Code, Bilder, Video, Tool-Calling und mehr mit aktuellen Details, Nutzung und öffentlichen Preisen.", updated: "Mit aktuellen Katalogdaten aktualisiert", browse: "Sammlung öffnen", allModels: "Alle Modelle", compare: "Modelle vergleichen", featured: "Ausgewählte Modelle", usage: "Nutzung pro Woche", context: "Kontext", from: "ab", explore: "Weitere Sammlungen", criteria: "So entsteht diese Sammlung", capabilities: "Katalogfunktionen", signals: "Nutzungs- und Preissignale", details: "Modelldetails ansehen", openConsole: "Konsole öffnen", browsePricing: "Preise ansehen" },
  id: { collections: "Koleksi", heroTitle: "Temukan model AI yang tepat untuk setiap tugas.", heroDescription: "Jelajahi koleksi untuk coding, gambar, video, pemanggilan alat, dan lainnya dengan detail, penggunaan, serta harga publik.", updated: "Diperbarui dengan data katalog langsung", browse: "Lihat koleksi", allModels: "Lihat semua model", compare: "Bandingkan model", featured: "Model pilihan", usage: "penggunaan mingguan", context: "konteks", from: "mulai", explore: "Jelajahi koleksi lain", criteria: "Cara koleksi ini dibuat", capabilities: "Kemampuan katalog", signals: "Sinyal penggunaan dan harga", details: "Lihat detail model", openConsole: "Buka konsol", browsePricing: "Lihat harga" },
} as const;

const modelFallbackCopy: Record<Locale, (name: string, vendor: string, collection: string) => string> = {
  en: (name, vendor, collection) => `${name} by ${vendor} is available through the Flatkey unified API and is included in our ${collection} collection. Review its current pricing, context, and availability before integrating it.`,
  zh: (name, vendor, collection) => `${name} 是由 ${vendor} 提供、可通过 Flatkey 统一 API 调用的模型，并收录在“${collection}”集合中。接入前可查看其当前价格、上下文和可用状态。`,
  es: (name, vendor, collection) => `${name}, de ${vendor}, está disponible mediante la API unificada de Flatkey y forma parte de «${collection}». Consulta su precio, contexto y disponibilidad antes de integrarlo.`,
  fr: (name, vendor, collection) => `${name}, proposé par ${vendor}, est accessible via l’API unifiée Flatkey et figure dans la collection « ${collection} ». Vérifiez son tarif, son contexte et sa disponibilité avant intégration.`,
  pt: (name, vendor, collection) => `${name}, da ${vendor}, está disponível pela API unificada da Flatkey e faz parte da coleção “${collection}”. Consulte preço, contexto e disponibilidade antes da integração.`,
  ru: (name, vendor, collection) => `${name} от ${vendor} доступна через единый API Flatkey и входит в подборку «${collection}». Перед интеграцией проверьте актуальные цены, контекст и доступность.`,
  ja: (name, vendor, collection) => `${vendor} の ${name} は Flatkey の統合 API から利用でき、「${collection}」に掲載されています。導入前に現在の料金、コンテキスト、提供状況を確認できます。`,
  vi: (name, vendor, collection) => `${name} của ${vendor} có thể dùng qua API hợp nhất Flatkey và nằm trong bộ sưu tập “${collection}”. Hãy xem giá, ngữ cảnh và tình trạng khả dụng hiện tại trước khi tích hợp.`,
  de: (name, vendor, collection) => `${name} von ${vendor} ist über die einheitliche Flatkey-API verfügbar und Teil der Sammlung „${collection}“. Prüfen Sie vor der Integration aktuelle Preise, Kontext und Verfügbarkeit.`,
  id: (name, vendor, collection) => `${name} dari ${vendor} tersedia melalui API terpadu Flatkey dan termasuk dalam koleksi “${collection}”. Periksa harga, konteks, dan ketersediaan terbaru sebelum integrasi.`,
};

const topModelsSummaryCopy: Record<Locale, (names: string) => string> = {
  en: (names) => `Featured models include ${names}, ranked by live weekly usage when available.`,
  zh: (names) => `当前热门模型包括 ${names}；有数据时按实时周调用量排序。`,
  es: (names) => `Los modelos destacados incluyen ${names}, ordenados por uso semanal en vivo cuando está disponible.`,
  fr: (names) => `Les modèles mis en avant sont ${names}, classés selon l’usage hebdomadaire lorsqu’il est disponible.`,
  pt: (names) => `Os modelos em destaque incluem ${names}, ordenados pelo uso semanal quando disponível.`,
  ru: (names) => `В подборку входят ${names}; при наличии данных порядок учитывает использование за неделю.`,
  ja: (names) => `注目モデルは ${names} です。利用可能な場合は週間利用量で並べています。`,
  vi: (names) => `Các mô hình nổi bật gồm ${names}, được xếp theo lượt dùng hằng tuần khi có dữ liệu.`,
  de: (names) => `Zu den hervorgehobenen Modellen gehören ${names}; sofern verfügbar, werden sie nach der wöchentlichen Nutzung sortiert.`,
  id: (names) => `Model unggulan mencakup ${names}, diurutkan berdasarkan penggunaan mingguan bila tersedia.`,
};

function buildDetailedModelDescription(base: string | undefined, supplemental: string, locale: Locale): string {
  const description = base?.trim();
  if (!description) return supplemental;
  if (description.toLocaleLowerCase().includes(supplemental.toLocaleLowerCase())) return description;
  if (supplemental.toLocaleLowerCase().includes(description.toLocaleLowerCase())) return supplemental;
  const hasEndingPunctuation = /[.!?。！？]$/.test(description);
  const punctuation = locale === "zh" || locale === "ja" ? "。" : ".";
  return `${description}${hasEndingPunctuation ? "" : punctuation} ${supplemental}`;
}

function getUiCopy(locale: Locale) {
  return uiCopy[locale] ?? uiCopy.en;
}

function formatContext(value: number | null | undefined): string | null {
  if (!value) return null;
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(value % 1_000_000 ? 1 : 0)}M`;
  if (value >= 1_000) return `${Math.round(value / 1_000)}K`;
  return String(value);
}

function CollectionCard(props: { collection: ModelCollectionDefinition; locale: Locale; index: number }) {
  const copy = getModelCollectionCopy(props.collection, props.locale);
  const ui = getUiCopy(props.locale);
  return (
    <Link
      href={localizePath(`/collections/${props.collection.slug}`, props.locale)}
      className="landing-animate-fade-up group flex min-h-[184px] flex-col rounded-lg border border-[#E8E5EF] bg-[#F8F8FA] p-5 opacity-0 transition duration-300 hover:-translate-y-0.5 hover:border-[#C9B8FF] hover:bg-white hover:shadow-[0_16px_32px_-26px_rgba(76,29,149,.42)] sm:p-6"
      style={{ animationDelay: `${120 + props.index * 55}ms` }}
    >
      <h2 className="text-lg font-semibold tracking-[-0.02em] text-[#16151B] sm:text-xl">{copy.title}</h2>
      <p className="mt-2 text-sm leading-6 text-[#65616F]">{copy.shortDescription} {copy.intro}</p>
      <span className="mt-auto inline-flex items-center gap-1 pt-4 text-sm font-semibold text-[#7C3AED]">{ui.browse}<ArrowRight className="size-4 transition group-hover:translate-x-1" aria-hidden="true" /></span>
    </Link>
  );
}

export function ModelCollectionsIndex(props: { locale: Locale; pricing: PricingData }) {
  const ui = getUiCopy(props.locale);
  const orderedCollections = getAvailableModelCollections(props.pricing.models).sort(
    (a, b) => COLLECTION_DISPLAY_ORDER.indexOf(a.slug as (typeof COLLECTION_DISPLAY_ORDER)[number]) - COLLECTION_DISPLAY_ORDER.indexOf(b.slug as (typeof COLLECTION_DISPLAY_ORDER)[number]),
  );
  const schema = buildCollectionsIndexSchema({
    locale: props.locale,
    title: ui.collections,
    description: ui.heroDescription,
    collections: orderedCollections.map((collection) => ({
      name: getModelCollectionCopy(collection, props.locale).title,
      path: localizePath(`/collections/${collection.slug}`, props.locale),
    })),
  });
  return (
    <SiteShell locale={props.locale} pathname="/collections">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: stringifyJsonLd(schema) }} />
      <main className="model-square-page relative overflow-x-hidden bg-[#FAFAFC]">
      <section className="py-10 sm:py-14">
        <div className={`${shellClass} max-w-[1160px]`}>
          <div className="max-w-5xl">
            <h1 className="landing-animate-fade-up text-3xl font-semibold tracking-[-0.035em] text-[#16151B] opacity-0 sm:text-4xl" style={{ animationDelay: "40ms" }}>{ui.collections}</h1>
            <p className="landing-animate-fade-up mt-3 max-w-4xl text-base leading-7 text-[#5F5A68] opacity-0" style={{ animationDelay: "80ms" }}>{ui.heroDescription}</p>
          </div>
          <div className="mt-8 grid gap-5 sm:grid-cols-2">
            {orderedCollections.map((collection, index) => <CollectionCard key={collection.slug} collection={collection} locale={props.locale} index={index} />)}
          </div>
        </div>
      </section>
      </main>
    </SiteShell>
  );
}

function ModelRow(props: { model: ReturnType<typeof modelCardData> & { rawName: string }; rank: number; usage?: number; locale: Locale }) {
  const ui = getUiCopy(props.locale);
  return (
    <article className="-mx-3 flex min-h-[232px] flex-col rounded-xl border-t border-[#ECEAF1] px-3 py-6 transition duration-200 first:border-t-0 hover:bg-[#FBFAFE] sm:-mx-4 sm:px-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <span className="mt-2 w-5 shrink-0 text-right text-xs font-semibold tabular-nums text-[#9B95A3]">{props.rank}.</span>
          <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-xl border border-[#E7E4EC] bg-[#FBFAFC] text-[#5B21B6] shadow-[0_8px_18px_-14px_rgba(76,29,149,.5)]">
            <ModelLogo iconKey={props.model.iconKey} fallback={props.model.name.charAt(0).toUpperCase()} size={22} />
          </span>
          <div className="min-w-0">
          <Link href={localizePath(props.model.href, props.locale)} className="text-base font-semibold text-[#201D28] hover:text-[#6D28D9]">{props.model.name}</Link>
          <p className="mt-1 text-sm text-[#777180]">{props.model.vendor}</p>
          </div>
        </div>
        {props.usage != null ? <span className="text-sm font-medium text-[#777180]">{displayTokens(props.usage).toLocaleString()} {ui.usage}</span> : null}
      </div>
      {props.model.description ? <p className="mt-4 line-clamp-4 min-h-24 text-sm leading-6 text-[#5F5A68]">{props.model.description}</p> : null}
      <div className="mt-auto flex flex-wrap items-center justify-between gap-3 pt-4">
        <div className="flex flex-wrap gap-x-5 gap-y-2 text-xs font-medium text-[#777180]">
          {formatContext(props.model.context) ? <span>{formatContext(props.model.context)} {ui.context}</span> : null}
          {props.model.price ? <span>{props.model.price}</span> : null}
        </div>
        <Link href={localizePath(props.model.href, props.locale)} className="inline-flex shrink-0 items-center gap-1 rounded-md bg-[#F2ECFF] px-2.5 py-1.5 text-xs font-semibold !text-[#6D28D9] transition hover:-translate-y-px hover:bg-[#E9D5FF] hover:!text-[#5B21B6] hover:shadow-[0_6px_12px_-8px_rgba(76,29,149,.65)]">{ui.details}<ArrowRight className="size-3.5" aria-hidden="true" /></Link>
      </div>
    </article>
  );
}

export function ModelCollectionDetail(props: { locale: Locale; collection: ModelCollectionDefinition; pricing: PricingData; rankings: RankingsData | null }) {
  const ui = getUiCopy(props.locale);
  const copy = getModelCollectionCopy(props.collection, props.locale);
  const usageByName = new Map((props.rankings?.models ?? []).map((row) => [row.model_name, row.total_tokens]));
  const models = selectCollectionModels(props.collection, props.pricing.models).sort((a, b) => {
    const usageDelta = (usageByName.get(b.model_name) ?? 0) - (usageByName.get(a.model_name) ?? 0);
    return usageDelta || a.model_name.localeCompare(b.model_name);
  });
  const fallback = modelFallbackCopy[props.locale] ?? modelFallbackCopy.en;
  const cards = models.map((model) => {
    const card = modelCardData(model, props.pricing);
    const supplemental = fallback(card.name, card.vendor, copy.title);
    return {
      ...card,
      description: buildDetailedModelDescription(props.locale === "en" ? card.description : undefined, supplemental, props.locale),
      rawName: model.model_name,
    };
  });
  const topModelNames = new Intl.ListFormat(props.locale, { style: "long", type: "conjunction" }).format(cards.slice(0, 3).map((model) => model.name));
  const updatedMonth = new Intl.DateTimeFormat(props.locale, { month: "long", year: "numeric" }).format(new Date());
  const rankingHeading = props.locale === "en"
    ? `Top ${copy.title.replace(/^Best /, "").replace(/ on Flatkey$/, "")} on Flatkey`
    : `${ui.featured}: ${copy.title}`;
  const related = getAvailableModelCollections(props.pricing.models).filter((collection) => collection.slug !== props.collection.slug);
  const schema = buildCollectionDetailSchema({
    locale: props.locale,
    collectionsName: ui.collections,
    slug: props.collection.slug,
    title: copy.title,
    description: `${copy.shortDescription} ${copy.intro}`,
    models: cards.map((model, index) => ({
      name: model.name,
      path: localizePath(model.href, props.locale),
      position: index + 1,
      vendor: model.vendor,
      description: model.description || undefined,
    })),
  });

  return (
    <SiteShell locale={props.locale} pathname={`/collections/${props.collection.slug}`}>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: stringifyJsonLd(schema) }} />
      <main className="model-detail-page model-prototype relative overflow-x-hidden bg-white text-[#171a21]">
      <section className="bg-white py-8 sm:py-10">
        <div className={detailShellClass}>
          <nav className="text-sm text-[#777180]"><Link href={localizePath("/collections", props.locale)} className="hover:text-[#6D28D9]">{ui.collections}</Link><span className="mx-2">/</span><span>{copy.title}</span></nav>
          <div className="mt-4">
            <h1 className="text-3xl font-semibold tracking-[-0.035em] text-[#16151B] sm:text-4xl">{copy.title}</h1>
            <p className="mt-2 text-sm font-medium text-[#777180]">{ui.updated} · {updatedMonth}</p>
            <p className="mt-5 text-base leading-7 text-[#5F5A68]">{copy.intro}</p>
            <p className="mt-3 text-base leading-7 text-[#5F5A68]">{topModelsSummaryCopy[props.locale](topModelNames)}</p>
            <div className="mt-5 flex flex-wrap gap-3">
              <Link href={localizePath("/models", props.locale)} className="inline-flex items-center gap-2 rounded-lg bg-[#6D28D9] px-4 py-2.5 text-sm font-semibold !text-white transition hover:-translate-y-0.5 hover:!bg-[#5B21B6] hover:shadow-[0_10px_20px_-14px_rgba(76,29,149,.8)]">{ui.allModels}<ArrowRight className="size-4" /></Link>
            </div>
          </div>
        </div>
      </section>

      <section className="bg-white pb-12 sm:pb-16">
        <div className={detailShellClass}>
          <div>
              <div className="mb-2 flex items-center gap-3"><Sparkles className="size-5 text-[#7C3AED]" /><h2 className="text-2xl font-semibold tracking-[-0.03em] text-[#201D28]">{rankingHeading}</h2></div>
              <p className="mb-3 text-sm leading-6 text-[#777180]">{copy.criteria}</p>
              <div>
                {cards.length ? cards.map((model, index) => <ModelRow key={model.href} model={model} rank={index + 1} locale={props.locale} usage={usageByName.get(model.rawName)} />) : <p className="text-sm text-[#777180]">{copy.empty}</p>}
              </div>
          </div>
        </div>
      </section>

      <section className="bg-[#FBFAFE] py-12 sm:py-16"><div className={detailShellClass}><h2 className="text-2xl font-semibold tracking-[-0.03em] text-[#201D28]">{ui.explore}</h2><div className="mt-6 flex flex-wrap gap-3">{related.map((collection) => <Link key={collection.slug} href={localizePath(`/collections/${collection.slug}`, props.locale)} className="rounded-full border border-[#DDD7E8] bg-white px-4 py-2 text-sm font-medium text-[#4C4657] hover:border-[#A78BFA] hover:text-[#6D28D9]">{getModelCollectionCopy(collection, props.locale).title}</Link>)}</div></div></section>
      </main>
    </SiteShell>
  );
}
