import Link from "next/link";
import { ArrowRight, Sparkles } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { ModelLogo } from "@/components/pricing-model-browser";
import {
  getModelCollectionCopy,
  modelCardData,
  MODEL_COLLECTIONS,
  type ModelCollectionDefinition,
  selectCollectionModels,
} from "@/lib/model-collections";
import { localizePath, type Locale } from "@/lib/locales";
import { displayTokens, type RankingsData } from "@/lib/rankings-live";
import { type PricingData } from "@/lib/pricing";
import { consoleUrl } from "@/lib/origins";

const shellClass = "fk-site-frame";
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
      className="landing-animate-fade-up group flex min-h-[168px] flex-col rounded-lg border border-[#E8E5EF] bg-[#FBFBFD] p-5 opacity-0 transition hover:border-[#C9B8FF] hover:bg-white hover:shadow-[0_12px_30px_-24px_rgba(76,29,149,.35)] sm:p-6"
      style={{ animationDelay: `${120 + props.index * 55}ms` }}
    >
      <h2 className="text-lg font-semibold tracking-[-0.02em] text-[#16151B] sm:text-xl">{copy.title}</h2>
      <p className="mt-2 text-sm leading-6 text-[#65616F]">{copy.shortDescription}</p>
      <span className="mt-auto inline-flex items-center gap-1 pt-5 text-sm font-semibold text-[#7C3AED]">{ui.browse}<ArrowRight className="size-4 transition group-hover:translate-x-1" aria-hidden="true" /></span>
    </Link>
  );
}

export function ModelCollectionsIndex(props: { locale: Locale }) {
  const ui = getUiCopy(props.locale);
  return (
    <SiteShell locale={props.locale} pathname="/collections">
      <main className="model-square-page relative min-h-screen overflow-x-hidden bg-[#FAFAFC]">
      <section className="border-b border-[#ECEAF1] bg-gradient-to-b from-[#F8F6FF] via-[#FBFAFD] to-[#FAFAFC] py-20 sm:py-28">
        <div className={`${shellClass} text-center`}>
          <p className="landing-animate-fade-up text-sm font-bold uppercase tracking-[0.16em] text-[#6D28D9] opacity-0">{ui.collections}</p>
          <h1 className="landing-animate-fade-up mx-auto mt-5 max-w-5xl text-[clamp(2.75rem,7vw,6.5rem)] font-semibold leading-[1.04] tracking-[-0.065em] text-[#0B0B0F] opacity-0" style={{ animationDelay: "60ms" }}>{ui.heroTitle}</h1>
          <p className="landing-animate-fade-up mx-auto mt-7 max-w-4xl text-lg leading-8 text-[#5F5A68] opacity-0 sm:text-xl" style={{ animationDelay: "110ms" }}>{ui.heroDescription}</p>
          <div className="landing-animate-fade-up mt-9 flex flex-wrap justify-center gap-3 opacity-0" style={{ animationDelay: "160ms" }}>
            <a href={consoleUrl("/dashboard")} className="inline-flex h-12 items-center justify-center gap-2 rounded-xl bg-black px-7 text-sm font-semibold !text-white transition hover:-translate-y-0.5 hover:bg-[#202024] hover:shadow-[0_12px_24px_-14px_rgba(0,0,0,.65)]">{ui.openConsole}<ArrowRight className="size-4" aria-hidden="true" /></a>
            <Link href={localizePath("/pricing", props.locale)} className="inline-flex h-12 items-center justify-center rounded-xl border border-[#E3E0E8] bg-white px-7 text-sm font-semibold text-[#25232B] transition hover:-translate-y-0.5 hover:border-[#C9B8FF] hover:bg-[#FCFAFF] hover:shadow-[0_12px_24px_-14px_rgba(76,29,149,.35)]">{ui.browsePricing}</Link>
          </div>
        </div>
      </section>
      <section className="bg-[#FAFAFC] py-14 sm:py-20">
        <div className={`${shellClass}`}>
          <div className="grid gap-4 sm:grid-cols-2">
            {[...MODEL_COLLECTIONS]
              .sort((a, b) => COLLECTION_DISPLAY_ORDER.indexOf(a.slug as (typeof COLLECTION_DISPLAY_ORDER)[number]) - COLLECTION_DISPLAY_ORDER.indexOf(b.slug as (typeof COLLECTION_DISPLAY_ORDER)[number]))
              .map((collection, index) => <CollectionCard key={collection.slug} collection={collection} locale={props.locale} index={index} />)}
          </div>
        </div>
      </section>
      </main>
    </SiteShell>
  );
}

function ModelRow(props: { model: ReturnType<typeof modelCardData> & { rawName: string }; usage?: number; locale: Locale }) {
  const ui = getUiCopy(props.locale);
  return (
    <article className="border-t border-[#ECEAF1] py-6 first:border-t-0 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
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
      {props.model.description ? <p className="mt-3 line-clamp-2 text-sm leading-6 text-[#5F5A68]">{props.model.description}</p> : null}
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
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
  const cards = models.map((model) => ({ ...modelCardData(model, props.pricing), rawName: model.model_name }));
  const related = MODEL_COLLECTIONS.filter((collection) => collection.slug !== props.collection.slug);

  return (
    <SiteShell locale={props.locale} pathname={`/collections/${props.collection.slug}`}>
      <main className="model-detail-page model-prototype relative overflow-x-hidden bg-white text-[#171a21]">
      <section className="model-hero">
        <div className="model-container">
          <nav className="text-sm text-[#777180]"><Link href={localizePath("/collections", props.locale)} className="hover:text-[#6D28D9]">{ui.collections}</Link><span className="mx-2">/</span><span>{copy.title}</span></nav>
          <div className="mt-8 max-w-4xl">
            <h1 className="mt-6 text-4xl font-semibold tracking-[-0.045em] text-[#16151B] sm:text-6xl">{copy.title}</h1>
            <p className="mt-4 text-sm font-medium text-[#777180]">{ui.updated}</p>
            <p className="mt-6 max-w-3xl text-lg leading-8 text-[#5F5A68]">{copy.intro}</p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Link href={localizePath("/models", props.locale)} className="inline-flex items-center gap-2 rounded-lg bg-[#6D28D9] px-4 py-2.5 text-sm font-semibold !text-white transition hover:-translate-y-0.5 hover:!bg-[#5B21B6] hover:shadow-[0_10px_20px_-14px_rgba(76,29,149,.8)]">{ui.allModels}<ArrowRight className="size-4" /></Link>
            </div>
          </div>
        </div>
      </section>

      <section className="bg-white py-14 sm:py-20">
        <div className="model-container">
          <div>
              <div className="mb-8 flex items-center gap-3"><Sparkles className="size-5 text-[#7C3AED]" /><h2 className="text-2xl font-semibold tracking-[-0.03em] text-[#201D28]">{ui.featured}</h2></div>
              <div className="rounded-2xl border border-[#E8E5EF] bg-white p-6 sm:p-8">
                {cards.length ? cards.map((model) => <ModelRow key={model.href} model={model} locale={props.locale} usage={usageByName.get(model.rawName)} />) : <p className="text-sm text-[#777180]">{copy.empty}</p>}
              </div>
          </div>
        </div>
      </section>

      <section className="bg-[#FBFAFE] py-14 sm:py-20"><div className="model-container"><h2 className="text-2xl font-semibold tracking-[-0.03em] text-[#201D28]">{ui.explore}</h2><div className="mt-6 flex flex-wrap gap-3">{related.map((collection) => <Link key={collection.slug} href={localizePath(`/collections/${collection.slug}`, props.locale)} className="rounded-full border border-[#DDD7E8] bg-white px-4 py-2 text-sm font-medium text-[#4C4657] hover:border-[#A78BFA] hover:text-[#6D28D9]">{getModelCollectionCopy(collection, props.locale).title}</Link>)}</div></div></section>
      </main>
    </SiteShell>
  );
}
