import Link from "next/link";
import { ArrowRight, BarChart3, Check, Layers3, Sparkles } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import {
  getModelCollectionCopy,
  modelCardData,
  MODEL_COLLECTIONS,
  type ModelCollectionDefinition,
  selectCollectionModels,
} from "@/lib/model-collections";
import { localizePath, type Locale } from "@/lib/locales";
import { displayTokens, type RankingsData } from "@/lib/rankings-live";
import {
  getPricingData,
  WEBSITE_PUBLIC_PRICING_GROUP,
  type PricingData,
} from "@/lib/pricing";
import {
  AGE_BANDS,
  categoriesForModels,
  CONTEXT_BUCKETS,
  formatContextTokens,
  MODALITIES,
  PRICE_BANDS,
  providersForModels,
  seriesForModels,
  vendorsForModels,
} from "@/lib/model-directory-meta";
import { AGE_BAND_LABELS, categoryLabel, getDirectoryCopy, MODALITY_LABELS } from "@/lib/model-directory-copy";
import { directoryHref } from "@/lib/model-directory-url";
import { EMPTY_DIRECTORY_FILTERS, type DirectoryFilters, type DirectoryFilterKey } from "@/lib/model-directory-filters";

const shellClass = "fk-site-frame";

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
    browseByFilter: "Browse by filter",
    browseByFilterDescription: "Start with the same filters used in the model directory, then compare the matching models.",
    viewAll: "View all",
    more: "More",
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
    browseByFilter: "按条件浏览",
    browseByFilterDescription: "使用模型目录中的相同筛选条件，快速找到并比较符合条件的模型。",
    viewAll: "查看全部",
    more: "更多",
  },
  es: { collections: "Colecciones", heroTitle: "Encuentra el modelo de IA adecuado para cada tarea.", heroDescription: "Explora colecciones de modelos para programación, imágenes, vídeo, herramientas y más, con detalles, uso y precios públicos.", updated: "Actualizado con datos del catálogo", browse: "Ver colección", allModels: "Ver todos los modelos", compare: "Comparar modelos", featured: "Modelos destacados", usage: "uso semanal", context: "contexto", from: "desde", explore: "Explora más colecciones", criteria: "Cómo se crea esta colección", capabilities: "Capacidades del catálogo", signals: "Señales de uso y precio", details: "Ver detalles del modelo", browseByFilter: "Explorar por filtro", browseByFilterDescription: "Usa los mismos filtros del directorio para encontrar y comparar modelos.", viewAll: "Ver todos", more: "Más" },
  fr: { collections: "Collections", heroTitle: "Trouvez le modèle IA adapté à chaque tâche.", heroDescription: "Explorez des collections pour le code, l’image, la vidéo, les outils et plus, avec détails, usage et tarifs publics.", updated: "Mis à jour avec les données du catalogue", browse: "Voir la collection", allModels: "Voir tous les modèles", compare: "Comparer les modèles", featured: "Modèles sélectionnés", usage: "utilisation hebdomadaire", context: "contexte", from: "à partir de", explore: "Explorer d’autres collections", criteria: "Comment cette collection est créée", capabilities: "Capacités du catalogue", signals: "Signaux d’usage et de prix", details: "Voir les détails du modèle", browseByFilter: "Parcourir par filtre", browseByFilterDescription: "Utilisez les mêmes filtres que l’annuaire pour trouver et comparer les modèles.", viewAll: "Tout voir", more: "Plus" },
  pt: { collections: "Coleções", heroTitle: "Encontre o modelo de IA certo para cada tarefa.", heroDescription: "Explore coleções para programação, imagens, vídeo, ferramentas e muito mais, com detalhes, uso e preços públicos.", updated: "Atualizado com dados do catálogo", browse: "Ver coleção", allModels: "Ver todos os modelos", compare: "Comparar modelos", featured: "Modelos em destaque", usage: "uso semanal", context: "contexto", from: "a partir de", explore: "Explore mais coleções", criteria: "Como esta coleção é criada", capabilities: "Capacidades do catálogo", signals: "Sinais de uso e preço", details: "Ver detalhes do modelo", browseByFilter: "Explorar por filtro", browseByFilterDescription: "Use os mesmos filtros do diretório para encontrar e comparar modelos.", viewAll: "Ver todos", more: "Mais" },
  ru: { collections: "Подборки", heroTitle: "Найдите подходящую ИИ-модель для каждой задачи.", heroDescription: "Изучайте подборки для программирования, изображений, видео, инструментов и других сценариев с актуальными ценами и данными.", updated: "Обновляется по данным каталога", browse: "Открыть подборку", allModels: "Все модели", compare: "Сравнить модели", featured: "Избранные модели", usage: "за неделю", context: "контекст", from: "от", explore: "Другие подборки", criteria: "Как формируется подборка", capabilities: "Возможности каталога", signals: "Сигналы использования и цены", details: "Подробнее о модели", browseByFilter: "По фильтрам", browseByFilterDescription: "Используйте те же фильтры каталога, чтобы находить и сравнивать модели.", viewAll: "Все модели", more: "Ещё" },
  ja: { collections: "コレクション", heroTitle: "タスクに合った AI モデルを見つけましょう。", heroDescription: "コーディング、画像、動画、ツール呼び出しなどのモデルを、詳細・利用量・公開価格とともに比較できます。", updated: "最新のカタログデータで更新", browse: "コレクションを見る", allModels: "すべてのモデル", compare: "モデルを比較", featured: "注目のモデル", usage: "週間利用量", context: "コンテキスト", from: "から", explore: "他のコレクション", criteria: "このコレクションの基準", capabilities: "カタログの機能", signals: "利用量と価格のシグナル", details: "モデルの詳細を見る", browseByFilter: "フィルターから探す", browseByFilterDescription: "モデル一覧と同じフィルターでモデルを検索・比較できます。", viewAll: "すべて見る", more: "もっと見る" },
  vi: { collections: "Bộ sưu tập", heroTitle: "Tìm mô hình AI phù hợp cho từng công việc.", heroDescription: "Khám phá bộ sưu tập cho lập trình, hình ảnh, video, gọi công cụ và hơn thế nữa với dữ liệu, lượt dùng và giá công khai.", updated: "Cập nhật theo catalog trực tiếp", browse: "Xem bộ sưu tập", allModels: "Xem tất cả mô hình", compare: "So sánh mô hình", featured: "Mô hình nổi bật", usage: "lượt dùng mỗi tuần", context: "ngữ cảnh", from: "từ", explore: "Khám phá bộ sưu tập khác", criteria: "Cách tạo bộ sưu tập", capabilities: "Khả năng trong catalog", signals: "Tín hiệu sử dụng và giá", details: "Xem chi tiết mô hình", browseByFilter: "Duyệt theo bộ lọc", browseByFilterDescription: "Dùng cùng bộ lọc của danh mục để tìm và so sánh mô hình.", viewAll: "Xem tất cả", more: "Thêm" },
  de: { collections: "Sammlungen", heroTitle: "Finden Sie das passende KI-Modell für jede Aufgabe.", heroDescription: "Entdecken Sie Sammlungen für Code, Bilder, Video, Tool-Calling und mehr mit aktuellen Details, Nutzung und öffentlichen Preisen.", updated: "Mit aktuellen Katalogdaten aktualisiert", browse: "Sammlung öffnen", allModels: "Alle Modelle", compare: "Modelle vergleichen", featured: "Ausgewählte Modelle", usage: "Nutzung pro Woche", context: "Kontext", from: "ab", explore: "Weitere Sammlungen", criteria: "So entsteht diese Sammlung", capabilities: "Katalogfunktionen", signals: "Nutzungs- und Preissignale", details: "Modelldetails ansehen", browseByFilter: "Nach Filter durchsuchen", browseByFilterDescription: "Nutzen Sie dieselben Verzeichnisfilter, um Modelle zu finden und zu vergleichen.", viewAll: "Alle anzeigen", more: "Mehr" },
  id: { collections: "Koleksi", heroTitle: "Temukan model AI yang tepat untuk setiap tugas.", heroDescription: "Jelajahi koleksi untuk coding, gambar, video, pemanggilan alat, dan lainnya dengan detail, penggunaan, serta harga publik.", updated: "Diperbarui dengan data katalog langsung", browse: "Lihat koleksi", allModels: "Lihat semua model", compare: "Bandingkan model", featured: "Model pilihan", usage: "penggunaan mingguan", context: "konteks", from: "mulai", explore: "Jelajahi koleksi lain", criteria: "Cara koleksi ini dibuat", capabilities: "Kemampuan katalog", signals: "Sinyal penggunaan dan harga", details: "Lihat detail model", browseByFilter: "Jelajahi berdasarkan filter", browseByFilterDescription: "Gunakan filter direktori yang sama untuk menemukan dan membandingkan model.", viewAll: "Lihat semua", more: "Lainnya" },
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

function CollectionCard(props: { collection: ModelCollectionDefinition; locale: Locale }) {
  const copy = getModelCollectionCopy(props.collection, props.locale);
  const ui = getUiCopy(props.locale);
  return (
    <Link
      href={localizePath(`/collections/${props.collection.slug}`, props.locale)}
      className="group rounded-2xl border border-[#E8E5EF] bg-white p-6 shadow-[0_8px_30px_-26px_rgba(36,20,64,.45)] transition hover:-translate-y-0.5 hover:border-[#A78BFA] hover:shadow-[0_18px_40px_-28px_rgba(76,29,149,.5)]"
    >
      <div className="flex items-start justify-between gap-5">
        <span className="flex size-10 items-center justify-center rounded-xl bg-[#F2ECFF] text-lg text-[#6D28D9]">{props.collection.icon}</span>
        <ArrowRight className="mt-1 size-5 text-[#A78BFA] transition group-hover:translate-x-1" aria-hidden="true" />
      </div>
      <h2 className="mt-5 text-xl font-semibold tracking-[-0.02em] text-[#16151B]">{copy.title}</h2>
      <p className="mt-2 text-sm leading-6 text-[#65616F]">{copy.shortDescription}</p>
      <span className="mt-5 inline-flex text-sm font-semibold text-[#6D28D9]">{ui.browse}</span>
    </Link>
  );
}

type FilterLinkGroup = {
  key: DirectoryFilterKey;
  label: string;
  links: Array<{ label: string; href: string }>;
};

function priceBandLabel(band: (typeof PRICE_BANDS)[number]): string {
  const min = "min" in band ? band.min : undefined;
  const max = "max" in band ? band.max : undefined;
  if (min == null && max != null) return `< $${max}`;
  if (min != null && max == null) return `$${min}+`;
  return `$${min}–$${max}`;
}

function filterHref(locale: Locale, key: DirectoryFilterKey, value: string | number | boolean): string {
  const filters: DirectoryFilters = { ...EMPTY_DIRECTORY_FILTERS, [key]: [value] } as DirectoryFilters;
  return directoryHref(locale, filters);
}

function buildFilterLinkGroups(locale: Locale, pricing: PricingData): FilterLinkGroup[] {
  const copy = getDirectoryCopy(locale);
  const metadataRows = pricing.models.map((model) => model.directory_metadata);
  const limit = (values: string[], max = 12) => values.slice(0, max);
  const groups: FilterLinkGroup[] = [
    {
      key: "modalities",
      label: copy.groupModalities,
      links: MODALITIES.map((value) => ({ label: MODALITY_LABELS[locale][value], href: filterHref(locale, "modalities", value) })),
    },
    {
      key: "context",
      label: copy.groupContext,
      links: CONTEXT_BUCKETS.map((value, index) => ({
        label: index === CONTEXT_BUCKETS.length - 1 ? `${formatContextTokens(value)}` : `${formatContextTokens(value)}+`,
        href: filterHref(locale, "context", value),
      })),
    },
    {
      key: "inputPrice",
      label: copy.groupInputPrice,
      links: PRICE_BANDS.map((band) => ({ label: priceBandLabel(band), href: filterHref(locale, "inputPrice", band.id) })),
    },
    {
      key: "outputPrice",
      label: copy.groupOutputPrice,
      links: PRICE_BANDS.map((band) => ({ label: priceBandLabel(band), href: filterHref(locale, "outputPrice", band.id) })),
    },
    {
      key: "categories",
      label: copy.groupCategories,
      links: limit(categoriesForModels(metadataRows)).map((value) => ({ label: categoryLabel(locale, value), href: filterHref(locale, "categories", value) })),
    },
    {
      key: "series",
      label: copy.groupSeries,
      links: limit(seriesForModels(metadataRows)).map((value) => ({ label: value, href: filterHref(locale, "series", value) })),
    },
    {
      key: "providers",
      label: copy.groupProviders,
      links: limit(providersForModels(metadataRows)).map((value) => ({ label: value, href: filterHref(locale, "providers", value) })),
    },
    {
      key: "vendors",
      label: copy.groupVendors,
      links: limit(vendorsForModels(metadataRows)).map((value) => ({ label: value, href: filterHref(locale, "vendors", value) })),
    },
    {
      key: "age",
      label: copy.groupAge,
      links: AGE_BANDS.map((value) => ({ label: AGE_BAND_LABELS[locale][value], href: filterHref(locale, "age", value) })),
    },
    {
      key: "distillable",
      label: copy.groupDistillable,
      links: [true, false].map((value) => ({ label: value ? copy.yes : copy.no, href: filterHref(locale, "distillable", value) })),
    },
  ];
  return groups.filter((group) => group.links.length > 0);
}

function FilterBrowseSection(props: { locale: Locale; pricing: PricingData }) {
  const ui = getUiCopy(props.locale);
  const groups = buildFilterLinkGroups(props.locale, props.pricing);
  return (
    <section className="border-t border-[#ECEAF1] bg-[#FBFAFE] py-14 sm:py-20">
      <div className={`${shellClass}`}>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 className="text-2xl font-semibold tracking-[-0.03em] text-[#201D28] sm:text-3xl">{ui.browseByFilter}</h2>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-[#65616F]">{ui.browseByFilterDescription}</p>
          </div>
          <Link href={localizePath("/models", props.locale)} className="inline-flex items-center gap-1 text-sm font-semibold text-[#6D28D9] hover:underline">
            {ui.viewAll}<ArrowRight className="size-4" aria-hidden="true" />
          </Link>
        </div>
        <div className="mt-8 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {groups.map((group) => (
            <div key={group.key} className="rounded-2xl border border-[#E7E4EC] bg-white p-5 shadow-[0_8px_30px_-28px_rgba(36,20,64,.4)]">
              <h3 className="text-sm font-semibold text-[#3C3548]">{group.label}</h3>
              <div className="mt-3 flex flex-wrap gap-2">
                {group.links.map((link) => (
                  <Link key={link.href} href={link.href} className="inline-flex items-center rounded-lg border border-[#E7E4EC] bg-white px-2.5 py-1.5 text-xs font-semibold text-[#45414C] transition hover:-translate-y-px hover:border-[#C9B8FF] hover:bg-[#F8F4FF] hover:text-[#5B21B6]">
                    {link.label}
                  </Link>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export async function ModelCollectionsIndex(props: { locale: Locale }) {
  const ui = getUiCopy(props.locale);
  const pricing = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  return (
    <SiteShell locale={props.locale} pathname="/collections">
      <main className="model-square-page relative min-h-screen overflow-x-hidden bg-[#FAFAFC]">
      <section className="py-16 sm:py-24">
        <div className={`${shellClass} pt-8 pb-8 sm:pt-10 sm:pb-10`}>
          <div className="max-w-3xl">
            <p className="text-sm font-semibold uppercase tracking-[0.18em] text-[#7C3AED]">{ui.collections}</p>
            <h1 className="mt-4 text-4xl font-semibold tracking-[-0.045em] text-[#16151B] sm:text-6xl">{ui.heroTitle}</h1>
            <p className="mt-5 max-w-2xl text-lg leading-8 text-[#5F5A68]">{ui.heroDescription}</p>
          </div>
          <div className="mt-12 grid gap-4 sm:grid-cols-2">
            {MODEL_COLLECTIONS.map((collection) => <CollectionCard key={collection.slug} collection={collection} locale={props.locale} />)}
          </div>
        </div>
      </section>
      <FilterBrowseSection locale={props.locale} pricing={pricing} />
      </main>
    </SiteShell>
  );
}

function ModelRow(props: { model: ReturnType<typeof modelCardData> & { rawName: string }; usage?: number; locale: Locale }) {
  const ui = getUiCopy(props.locale);
  return (
    <article className="border-t border-[#ECEAF1] py-6 first:border-t-0 first:pt-0 last:pb-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <Link href={localizePath(props.model.href, props.locale)} className="text-base font-semibold text-[#201D28] hover:text-[#6D28D9]">{props.model.name}</Link>
          <p className="mt-1 text-sm text-[#777180]">{props.model.vendor}</p>
        </div>
        {props.usage != null ? <span className="text-sm font-medium text-[#777180]">{displayTokens(props.usage).toLocaleString()} {ui.usage}</span> : null}
      </div>
      {props.model.description ? <p className="mt-3 line-clamp-2 text-sm leading-6 text-[#5F5A68]">{props.model.description}</p> : null}
      <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-xs font-medium text-[#777180]">
        {formatContext(props.model.context) ? <span>{formatContext(props.model.context)} {ui.context}</span> : null}
        {props.model.price ? <span>{props.model.price}</span> : null}
        <Link href={localizePath(props.model.href, props.locale)} className="text-[#6D28D9] hover:underline">{ui.details}</Link>
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
            <div className="flex size-12 items-center justify-center rounded-2xl bg-[#F2ECFF] text-xl text-[#6D28D9]">{props.collection.icon}</div>
            <h1 className="mt-6 text-4xl font-semibold tracking-[-0.045em] text-[#16151B] sm:text-6xl">{copy.title}</h1>
            <p className="mt-4 text-sm font-medium text-[#777180]">{ui.updated}</p>
            <p className="mt-6 max-w-3xl text-lg leading-8 text-[#5F5A68]">{copy.intro}</p>
            <div className="mt-7 flex flex-wrap gap-3">
              <Link href={localizePath("/models", props.locale)} className="inline-flex items-center gap-2 rounded-lg bg-[#6D28D9] px-4 py-2.5 text-sm font-semibold text-white hover:bg-[#5B21B6]">{ui.allModels}<ArrowRight className="size-4" /></Link>
              <Link href={localizePath("/models?view=compare", props.locale)} className="inline-flex items-center gap-2 rounded-lg border border-[#D8D2E4] bg-white px-4 py-2.5 text-sm font-semibold text-[#3C3548] hover:border-[#A78BFA]">{ui.compare}</Link>
            </div>
          </div>
        </div>
      </section>

      <section className="bg-white py-14 sm:py-20">
        <div className="model-container">
          <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_300px]">
            <div>
              <div className="mb-8 flex items-center gap-3"><Sparkles className="size-5 text-[#7C3AED]" /><h2 className="text-2xl font-semibold tracking-[-0.03em] text-[#201D28]">{ui.featured}</h2></div>
              <div className="rounded-2xl border border-[#E8E5EF] bg-white p-6 sm:p-8">
                {cards.length ? cards.map((model) => <ModelRow key={model.href} model={model} locale={props.locale} usage={usageByName.get(model.rawName)} />) : <p className="text-sm text-[#777180]">{copy.empty}</p>}
              </div>
            </div>
            <aside className="h-fit rounded-2xl border border-[#E8E5EF] bg-[#FBFAFE] p-6">
              <div className="flex items-center gap-2 text-sm font-semibold text-[#3C3548]"><Layers3 className="size-4 text-[#7C3AED]" />{ui.criteria}</div>
              <p className="mt-4 text-sm leading-6 text-[#65616F]">{copy.criteria}</p>
              <div className="mt-6 grid gap-3 text-sm text-[#4C4657]"><div className="flex gap-2"><Check className="mt-0.5 size-4 shrink-0 text-[#7C3AED]" />{ui.capabilities}</div><div className="flex gap-2"><BarChart3 className="mt-0.5 size-4 shrink-0 text-[#7C3AED]" />{ui.signals}</div></div>
            </aside>
          </div>
        </div>
      </section>

      <section className="bg-[#FBFAFE] py-14 sm:py-20"><div className="model-container"><h2 className="text-2xl font-semibold tracking-[-0.03em] text-[#201D28]">{ui.explore}</h2><div className="mt-6 flex flex-wrap gap-3">{related.map((collection) => <Link key={collection.slug} href={localizePath(`/collections/${collection.slug}`, props.locale)} className="rounded-full border border-[#DDD7E8] bg-white px-4 py-2 text-sm font-medium text-[#4C4657] hover:border-[#A78BFA] hover:text-[#6D28D9]">{getModelCollectionCopy(collection, props.locale).title}</Link>)}</div></div></section>
      </main>
    </SiteShell>
  );
}
