"use client";

import { ArrowRight, Bot, ChevronRight, Copy, ImageIcon, Sparkles, Type, Video, type LucideIcon } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useMemo, useState, type ReactNode } from "react";
import { SiteShell } from "@/components/site-shell";
import { localizePath, type Locale } from "@/lib/locales";
import type { PromptArtifact, PromptItem } from "@/lib/prompt-library";

type Props = { locale: Locale; items: PromptItem[]; initialSearch?: Record<string, string | string[] | undefined> };

type Copy = {
  title: string; description: string; featured: string; all: string; found: string; search: string; filter: string; reset: string;
  type: string; model: string; useCase: string; source: string; sort: string; updated: string; oldest: string; complete: string;
  view: string; copy: string; copied: string; noResults: string; noResultsHint: string; browse: string;
};

const copy: Record<Locale, Copy> = {
  en: { title: "Prompt directory", description: "Production-ready prompts paired with real outputs, models, and provenance.", featured: "Featured prompt", all: "All prompts", found: "prompts found", search: "Search prompts, models, use cases or tags…", filter: "Filters", reset: "Reset", type: "Type", model: "Model", useCase: "Use case", source: "Source", sort: "Sort", updated: "Recently updated", oldest: "Oldest first", complete: "Most complete output", view: "View details", copy: "Copy prompt", copied: "Copied", noResults: "No prompts found", noResultsHint: "Try clearing a filter or using a broader search.", browse: "Browse library" },
  zh: { title: "提示词目录", description: "每条提示词都绑定真实产物、适用模型和可追溯来源。", featured: "精选提示词", all: "全部提示词", found: "条提示词", search: "搜索提示词、模型、场景或标签…", filter: "筛选", reset: "重置", type: "类型", model: "模型", useCase: "适用场景", source: "来源", sort: "排序", updated: "最近更新", oldest: "最早更新", complete: "产物最完整", view: "查看详情", copy: "复制提示词", copied: "已复制", noResults: "没有找到提示词", noResultsHint: "试试清除筛选条件或扩大搜索范围。", browse: "浏览提示词库" },
  es: { title: "Directorio de prompts", description: "Prompts listos para producción con resultados, modelos y procedencia reales.", featured: "Prompt destacado", all: "Todos los prompts", found: "prompts encontrados", search: "Buscar prompts, modelos, usos o etiquetas…", filter: "Filtros", reset: "Restablecer", type: "Tipo", model: "Modelo", useCase: "Uso", source: "Fuente", sort: "Ordenar", updated: "Actualizados recientemente", oldest: "Más antiguos", complete: "Resultado más completo", view: "Ver detalles", copy: "Copiar prompt", copied: "Copiado", noResults: "No se encontraron prompts", noResultsHint: "Prueba a borrar un filtro o ampliar la búsqueda.", browse: "Explorar biblioteca" },
  fr: { title: "Répertoire de prompts", description: "Des prompts prêts pour la production avec résultats, modèles et sources vérifiables.", featured: "Prompt à la une", all: "Tous les prompts", found: "prompts trouvés", search: "Rechercher des prompts, modèles, usages ou tags…", filter: "Filtres", reset: "Réinitialiser", type: "Type", model: "Modèle", useCase: "Usage", source: "Source", sort: "Trier", updated: "Récemment mis à jour", oldest: "Plus anciens", complete: "Résultat le plus complet", view: "Voir les détails", copy: "Copier le prompt", copied: "Copié", noResults: "Aucun prompt trouvé", noResultsHint: "Supprimez un filtre ou élargissez la recherche.", browse: "Parcourir la bibliothèque" },
  pt: { title: "Diretório de prompts", description: "Prompts prontos para produção com resultados, modelos e fontes verificáveis.", featured: "Prompt em destaque", all: "Todos os prompts", found: "prompts encontrados", search: "Buscar prompts, modelos, usos ou tags…", filter: "Filtros", reset: "Redefinir", type: "Tipo", model: "Modelo", useCase: "Uso", source: "Fonte", sort: "Ordenar", updated: "Atualizados recentemente", oldest: "Mais antigos", complete: "Resultado mais completo", view: "Ver detalhes", copy: "Copiar prompt", copied: "Copiado", noResults: "Nenhum prompt encontrado", noResultsHint: "Limpe um filtro ou amplie a busca.", browse: "Explorar biblioteca" },
  ru: { title: "Каталог промптов", description: "Готовые к продакшену промпты с реальными результатами, моделями и источниками.", featured: "Избранный промпт", all: "Все промпты", found: "промптов найдено", search: "Поиск по промптам, моделям, сценариям и тегам…", filter: "Фильтры", reset: "Сбросить", type: "Тип", model: "Модель", useCase: "Сценарий", source: "Источник", sort: "Сортировка", updated: "Недавно обновлённые", oldest: "Сначала старые", complete: "Самый полный результат", view: "Подробнее", copy: "Копировать промпт", copied: "Скопировано", noResults: "Промпты не найдены", noResultsHint: "Очистите фильтр или расширьте поиск.", browse: "Открыть библиотеку" },
  ja: { title: "プロンプトディレクトリ", description: "実際の成果物、モデル、出典を確認できる本番向けプロンプト。", featured: "注目のプロンプト", all: "すべてのプロンプト", found: "件", search: "プロンプト、モデル、用途、タグを検索…", filter: "フィルター", reset: "リセット", type: "種類", model: "モデル", useCase: "用途", source: "出典", sort: "並び替え", updated: "最近更新", oldest: "古い順", complete: "成果物が充実", view: "詳細を見る", copy: "プロンプトをコピー", copied: "コピーしました", noResults: "プロンプトが見つかりません", noResultsHint: "フィルターを解除するか検索範囲を広げてください。", browse: "ライブラリを見る" },
  vi: { title: "Thư mục prompt", description: "Prompt sẵn sàng cho sản xuất, kèm đầu ra, model và nguồn xác thực.", featured: "Prompt nổi bật", all: "Tất cả prompt", found: "prompt", search: "Tìm prompt, model, mục đích hoặc thẻ…", filter: "Bộ lọc", reset: "Đặt lại", type: "Loại", model: "Model", useCase: "Mục đích", source: "Nguồn", sort: "Sắp xếp", updated: "Mới cập nhật", oldest: "Cũ nhất", complete: "Đầu ra đầy đủ nhất", view: "Xem chi tiết", copy: "Sao chép prompt", copied: "Đã sao chép", noResults: "Không tìm thấy prompt", noResultsHint: "Hãy xóa bộ lọc hoặc mở rộng tìm kiếm.", browse: "Duyệt thư viện" },
  de: { title: "Prompt-Verzeichnis", description: "Produktionsreife Prompts mit echten Ergebnissen, Modellen und nachvollziehbaren Quellen.", featured: "Ausgewählter Prompt", all: "Alle Prompts", found: "Prompts gefunden", search: "Prompts, Modelle, Anwendungsfälle oder Tags suchen…", filter: "Filter", reset: "Zurücksetzen", type: "Typ", model: "Modell", useCase: "Anwendungsfall", source: "Quelle", sort: "Sortieren", updated: "Zuletzt aktualisiert", oldest: "Älteste zuerst", complete: "Vollständigstes Ergebnis", view: "Details ansehen", copy: "Prompt kopieren", copied: "Kopiert", noResults: "Keine Prompts gefunden", noResultsHint: "Filter löschen oder Suche erweitern.", browse: "Bibliothek durchsuchen" },
  id: { title: "Direktori prompt", description: "Prompt siap produksi dengan hasil, model, dan sumber yang dapat ditelusuri.", featured: "Prompt pilihan", all: "Semua prompt", found: "prompt ditemukan", search: "Cari prompt, model, penggunaan, atau tag…", filter: "Filter", reset: "Atur ulang", type: "Jenis", model: "Model", useCase: "Penggunaan", source: "Sumber", sort: "Urutkan", updated: "Baru diperbarui", oldest: "Terlama", complete: "Hasil terlengkap", view: "Lihat detail", copy: "Salin prompt", copied: "Tersalin", noResults: "Prompt tidak ditemukan", noResultsHint: "Hapus filter atau perluas pencarian.", browse: "Jelajahi pustaka" },
};

const categoryLabels: Record<Locale, Record<string, string>> = {
  en: { image: "Image", video: "Video", audio: "Audio", text: "Text", agent: "Agent" }, zh: { image: "图像", video: "视频", audio: "音频", text: "文本", agent: "Agent" }, es: { image: "Imagen", video: "Vídeo", audio: "Audio", text: "Texto", agent: "Agente" }, fr: { image: "Image", video: "Vidéo", audio: "Audio", text: "Texte", agent: "Agent" }, pt: { image: "Imagem", video: "Vídeo", audio: "Áudio", text: "Texto", agent: "Agente" }, ru: { image: "Изображение", video: "Видео", audio: "Аудио", text: "Текст", agent: "Агент" }, ja: { image: "画像", video: "動画", audio: "音声", text: "テキスト", agent: "Agent" }, vi: { image: "Hình ảnh", video: "Video", audio: "Âm thanh", text: "Văn bản", agent: "Agent" }, de: { image: "Bild", video: "Video", audio: "Audio", text: "Text", agent: "Agent" }, id: { image: "Gambar", video: "Video", audio: "Audio", text: "Teks", agent: "Agen" },
};

const sectionCopy: Record<Locale, { media: string; models: string; topics: string; latest: string }> = {
  en: { media: "Browse by media", models: "Browse by model", topics: "Browse by use case", latest: "Latest prompt examples" },
  zh: { media: "按媒介浏览", models: "按模型浏览", topics: "按场景浏览", latest: "最新提示词案例" },
  es: { media: "Explorar por medio", models: "Explorar por modelo", topics: "Explorar por uso", latest: "Últimos ejemplos de prompts" },
  fr: { media: "Parcourir par média", models: "Parcourir par modèle", topics: "Parcourir par usage", latest: "Derniers exemples de prompts" },
  pt: { media: "Explorar por mídia", models: "Explorar por modelo", topics: "Explorar por uso", latest: "Exemplos de prompts recentes" },
  ru: { media: "По типу контента", models: "По модели", topics: "По сценарию", latest: "Новые примеры промптов" },
  ja: { media: "メディアから探す", models: "モデルから探す", topics: "用途から探す", latest: "最新のプロンプト例" },
  vi: { media: "Duyệt theo nội dung", models: "Duyệt theo model", topics: "Duyệt theo mục đích", latest: "Ví dụ prompt mới nhất" },
  de: { media: "Nach Medium entdecken", models: "Nach Modell entdecken", topics: "Nach Anwendung entdecken", latest: "Neueste Prompt-Beispiele" },
  id: { media: "Jelajahi berdasarkan media", models: "Jelajahi berdasarkan model", topics: "Jelajahi berdasarkan penggunaan", latest: "Contoh prompt terbaru" },
};

const categoryIcons: Partial<Record<PromptItem["category"], LucideIcon>> = {
  image: ImageIcon,
  video: Video,
  text: Type,
  agent: Bot,
};

type Collection = { key: string; count: number; sample: PromptItem };

export function PromptDirectoryPage({ locale, items, initialSearch }: Props) {
  const text = copy[locale];
  const sections = sectionCopy[locale];
  const selectedType = firstSearchValue(initialSearch?.type);
  const selectedModel = firstSearchValue(initialSearch?.model);
  const selectedUseCase = firstSearchValue(initialSearch?.useCase);
  const [copied, setCopied] = useState("");

  const sortedItems = useMemo(
    () => [...items].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)),
    [items],
  );
  const mediaCollections = useMemo(() => {
    const order: PromptItem["category"][] = ["image", "video", "text", "agent", "audio"];
    return order.flatMap((category) => {
      const categoryItems = sortedItems.filter((item) => item.category === category);
      const sample = categoryItems[0];
      return sample ? [{ category, count: categoryItems.length, sample }] : [];
    });
  }, [sortedItems]);
  const modelCollections = useMemo(() => buildModelCollections(sortedItems).slice(0, 8), [sortedItems]);
  const topicCollections = useMemo(() => buildTopicCollections(sortedItems).slice(0, 9), [sortedItems]);
  const collectionItems = useMemo(
    () => sortedItems.filter((item) => {
      if (selectedType && item.category !== selectedType) return false;
      if (selectedModel && item.model !== selectedModel) return false;
      if (selectedUseCase && !item.tags.includes(selectedUseCase)) return false;
      return true;
    }),
    [selectedModel, selectedType, selectedUseCase, sortedItems],
  );
  const collectionTitle = selectedModel
    || (selectedType ? categoryLabels[locale][selectedType] : "")
    || selectedUseCase
    || sections.latest;
  const hasSelectedCollection = Boolean(selectedType || selectedModel || selectedUseCase);
  const featured = collectionItems[0] ?? sortedItems[0];
  const copyPrompt = async (item: PromptItem) => {
    await navigator.clipboard.writeText(item.prompt);
    setCopied(item.slug);
    window.setTimeout(() => setCopied(""), 1600);
  };

  return (
    <SiteShell locale={locale} pathname="/prompts">
      <main className="min-h-screen bg-white text-[#171a21]">
        <section className="border-b border-[#E7E4EC] bg-white px-6 pt-10 pb-10 sm:px-8 md:pt-14 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            <nav aria-label="Breadcrumb" className="mb-7 flex items-center gap-1 text-xs text-[#6B6475]">
              <Link href={localizePath("/", locale)} className="hover:text-violet-700">Flatkey</Link>
              <ChevronRight className="size-3" aria-hidden="true" />
              <span className="font-semibold text-[#0B0B0F]">{text.title}</span>
            </nav>
            <div className="flex flex-wrap items-end justify-between gap-8">
              <div>
                <p className="mb-3 text-xs font-black tracking-[0.16em] text-violet-600 uppercase">{text.browse}</p>
                <h1 className="text-[clamp(2.25rem,5vw,3.8rem)] leading-none font-extrabold tracking-tight">{text.title}</h1>
                <p className="mt-4 max-w-2xl text-base leading-7 text-[#62626D]">{text.description}</p>
              </div>
              <div className="min-w-32 rounded-2xl border border-[#E7E4EC] bg-[#F8F6FC] px-5 py-4 text-right shadow-[0_10px_30px_-24px_rgba(24,14,38,.24)]">
                <p className="text-3xl font-black tabular-nums text-[#0B0B0F]">{items.length}</p>
                <p className="mt-1 text-xs font-semibold text-[#6B7280]">{text.found}</p>
              </div>
            </div>
          </div>
        </section>

        {featured ? (
          <section className="bg-[#F8F6FC] px-6 py-10 sm:px-8 md:py-12 lg:px-10">
            <div className="mx-auto grid max-w-[1280px] overflow-hidden rounded-[22px] border border-[#E7E4EC] bg-white shadow-[0_24px_70px_-50px_rgba(24,14,38,.35)] lg:grid-cols-[0.9fr_1.1fr]">
              <div className="flex flex-col justify-center p-7 md:p-10">
                <p className="text-xs font-black tracking-[0.16em] text-violet-700 uppercase">{text.featured}</p>
                <h2 className="mt-3 text-3xl font-black tracking-tight text-[#0B0B0F] md:text-4xl">{featured.title[locale] ?? featured.title.en}</h2>
                <p className="mt-3 max-w-xl text-sm leading-6 text-[#5D566A]">{featured.summary[locale] ?? featured.summary.en}</p>
                <div className="mt-5 flex flex-wrap items-center gap-2">
                  <span className="rounded-full bg-[#F1EAFE] px-3 py-1 text-xs font-bold text-[#5B21B6]">{featured.model || categoryLabels[locale][featured.category]}</span>
                  {visibleTags(featured).slice(0, 2).map((tag) => <span key={tag} className="rounded-full border border-[#E7E4EC] bg-white px-3 py-1 text-xs font-semibold text-[#5D566A]">{tag}</span>)}
                </div>
                <Link href={promptHref(featured, locale)} className="mt-7 inline-flex h-11 w-fit items-center gap-2 rounded-xl bg-[#0B0B0F] px-5 text-sm font-bold text-white shadow-[0_14px_30px_-20px_rgba(11,11,15,.75)] transition hover:-translate-y-px hover:bg-[#242329]">
                  {text.view}<ArrowRight className="size-4" aria-hidden="true" />
                </Link>
              </div>
              <Link href={promptHref(featured, locale)} className="relative min-h-64 border-t border-[#E7E4EC] bg-[#E9E1FF] lg:min-h-[320px] lg:border-t-0 lg:border-l">
                <ArtifactPreview artifact={featured.artifact} title={featured.title[locale] ?? featured.title.en} variant="hero" />
              </Link>
            </div>
          </section>
        ) : null}

        <DirectorySection eyebrow={text.browse} title={sections.media} tone="white">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            {mediaCollections.map((collection) => <MediaCollectionCard key={collection.category} collection={collection} locale={locale} text={text} />)}
          </div>
        </DirectorySection>

        <DirectorySection eyebrow={text.model} title={sections.models} tone="tinted">
          <div className="grid overflow-hidden rounded-2xl border border-[#E7E4EC] bg-white md:grid-cols-2 xl:grid-cols-4">
            {modelCollections.map((collection, index) => (
              <Link key={collection.key} href={collectionHref(locale, "model", collection.key)} className="group flex min-h-36 flex-col justify-between border-[#E7E4EC] p-5 transition hover:bg-[#F8F4FF] md:border-r md:border-b xl:[&:nth-child(4n)]:border-r-0 xl:[&:nth-last-child(-n+4)]:border-b-0">
                <div className="flex items-start justify-between gap-3"><span className="flex size-9 items-center justify-center rounded-xl bg-[#F1EAFE] text-sm font-black text-[#5B21B6]">{String(index + 1).padStart(2, "0")}</span><ArrowRight className="size-4 text-[#A29CAB] transition group-hover:translate-x-0.5 group-hover:text-[#5B21B6]" aria-hidden="true" /></div>
                <div><h3 className="mt-5 break-words text-base font-black text-[#0B0B0F]">{collection.key}</h3><p className="mt-1 text-xs font-semibold text-[#77727F]">{collection.count} {text.found}</p></div>
              </Link>
            ))}
          </div>
        </DirectorySection>

        <DirectorySection eyebrow={text.useCase} title={sections.topics} tone="white">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {topicCollections.map((collection, index) => (
              <Link key={collection.key} href={collectionHref(locale, "useCase", collection.key)} className="group flex min-h-24 items-center gap-4 rounded-2xl border border-[#E7E4EC] bg-white p-4 shadow-[0_12px_30px_-28px_rgba(24,14,38,.2)] transition hover:-translate-y-0.5 hover:border-[#C9B8FF] hover:shadow-[0_18px_36px_-26px_rgba(91,33,182,.3)]">
                <span className="text-2xl font-black text-[#D8D1E2]">{String(index + 1).padStart(2, "0")}</span>
                <div className="min-w-0 flex-1"><h3 className="truncate text-sm font-black text-[#0B0B0F]">{collection.key}</h3><p className="mt-1 text-xs font-semibold text-[#77727F]">{collection.count} {text.found}</p></div>
                <ArrowRight className="size-4 shrink-0 text-[#A29CAB] transition group-hover:translate-x-0.5 group-hover:text-[#5B21B6]" aria-hidden="true" />
              </Link>
            ))}
          </div>
        </DirectorySection>

        <section id="prompt-collection" className="scroll-mt-28 border-y border-[#E7E4EC] bg-[#F8F6FC] px-6 py-10 sm:px-8 md:py-14 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
              <div><p className="text-xs font-black tracking-[0.14em] text-violet-600 uppercase">{hasSelectedCollection ? text.browse : text.featured}</p><h2 className="mt-2 text-2xl font-black tracking-tight text-[#0B0B0F] md:text-3xl">{collectionTitle}</h2><p className="mt-1 text-sm text-[#6B7280]">{collectionItems.length} {text.found}</p></div>
              {hasSelectedCollection ? <Link href={`${localizePath("/prompts", locale)}#prompt-collection`} className="inline-flex h-9 items-center rounded-lg border border-[#D8D1E2] bg-white px-3 text-xs font-bold text-[#514A5D] transition hover:border-[#C9B8FF] hover:text-[#5B21B6]">{text.all}</Link> : null}
            </div>
            {collectionItems.length ? <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">{collectionItems.slice(0, hasSelectedCollection ? 18 : 12).map((item) => <PromptDirectoryCard key={item.slug} item={item} locale={locale} text={text} copied={copied === item.slug} onCopy={() => copyPrompt(item)} />)}</div> : <div className="flex min-h-56 flex-col items-center justify-center rounded-2xl border border-[#E7E4EC] bg-white px-6 py-14 text-center"><h3 className="text-lg font-bold text-[#0B0B0F]">{text.noResults}</h3><p className="mt-2 text-sm text-[#6B7280]">{text.noResultsHint}</p><Link href={localizePath("/prompts", locale)} className="mt-5 inline-flex h-10 items-center rounded-xl bg-[#0B0B0F] px-4 text-xs font-bold text-white">{text.all}</Link></div>}
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

function DirectorySection({ eyebrow, title, tone, children }: { eyebrow: string; title: string; tone: "white" | "tinted"; children: ReactNode }) {
  return <section className={`border-t border-[#E7E4EC] px-6 py-10 sm:px-8 md:py-14 lg:px-10 ${tone === "tinted" ? "bg-[#F8F6FC]" : "bg-white"}`}><div className="mx-auto max-w-[1280px]"><div className="mb-6"><p className="text-xs font-black tracking-[0.14em] text-violet-600 uppercase">{eyebrow}</p><h2 className="mt-2 text-2xl font-black tracking-tight text-[#0B0B0F] md:text-3xl">{title}</h2></div>{children}</div></section>;
}

function MediaCollectionCard({ collection, locale, text }: { collection: { category: PromptItem["category"]; count: number; sample: PromptItem }; locale: Locale; text: Copy }) {
  const Icon = categoryIcons[collection.category] ?? Sparkles;
  return <Link href={mediaCollectionHref(locale, collection.category)} className="group overflow-hidden rounded-2xl border border-[#E7E4EC] bg-white shadow-[0_14px_34px_-28px_rgba(24,14,38,.22)] transition hover:-translate-y-0.5 hover:border-[#C9B8FF] hover:shadow-[0_22px_44px_-30px_rgba(91,33,182,.35)]"><div className="relative aspect-[16/9] overflow-hidden bg-[#EEE8FF]"><ArtifactPreview artifact={collection.sample.artifact} title={collection.sample.title[locale] ?? collection.sample.title.en} variant="hero" /><span className="absolute top-3 left-3 flex size-9 items-center justify-center rounded-xl border border-white/80 bg-white/90 text-[#5B21B6] shadow-sm backdrop-blur"><Icon className="size-4" aria-hidden="true" /></span></div><div className="flex items-center justify-between gap-3 p-4"><div><h3 className="text-base font-black text-[#0B0B0F]">{categoryLabels[locale][collection.category]}</h3><p className="mt-1 text-xs font-semibold text-[#77727F]">{collection.count} {text.found}</p></div><ArrowRight className="size-4 text-[#A29CAB] transition group-hover:translate-x-0.5 group-hover:text-[#5B21B6]" aria-hidden="true" /></div></Link>;
}

function PromptDirectoryCard({ item, locale, text, copied, onCopy }: { item: PromptItem; locale: Locale; text: Copy; copied: boolean; onCopy: () => void }) {
  const title = item.title[locale] ?? item.title.en;
  const summary = item.summary[locale] ?? item.summary.en;
  return <article className="group overflow-hidden rounded-2xl border border-[#E7E4EC] bg-white shadow-[0_12px_32px_-26px_rgba(24,14,38,.2)] transition hover:-translate-y-0.5 hover:border-violet-300 hover:shadow-[0_20px_40px_-28px_rgba(91,33,182,.35)]"><Link href={promptHref(item, locale)} className="block"><ArtifactPreview artifact={item.artifact} title={title} /></Link><div className="p-4"><div className="mb-2 flex flex-wrap items-center gap-2"><span className="rounded-full bg-[#F1EAFE] px-2.5 py-1 text-[11px] font-bold text-[#5B21B6]">{item.model || categoryLabels[locale][item.category]}</span><span className="text-[11px] font-semibold text-[#8A8493]">{item.updatedAt}</span></div><Link href={promptHref(item, locale)}><h3 className="line-clamp-2 min-h-10 text-base font-black leading-5 text-[#0B0B0F]">{title}</h3></Link><p className="mt-2 line-clamp-2 text-sm leading-5 text-[#6B6475]">{summary}</p><div className="mt-3 flex flex-wrap gap-1.5">{visibleTags(item).slice(0, 3).map((tag) => <span key={tag} className="rounded-md bg-[#F8F7FA] px-2 py-1 text-[10px] font-semibold text-[#77727F]">{tag}</span>)}</div><div className="mt-4 flex items-center justify-between gap-3 border-t border-[#EFECF3] pt-3"><Link href={promptHref(item, locale)} className="inline-flex items-center gap-1.5 text-xs font-black text-[#0B0B0F] transition hover:text-[#5B21B6]">{text.view}<ArrowRight className="size-3.5" aria-hidden="true" /></Link><button type="button" onClick={onCopy} aria-label={text.copy} aria-live="polite" className="inline-flex h-8 items-center gap-1.5 rounded-lg border border-[#D8D1E2] bg-[#F8F7FA] px-2.5 text-[11px] font-bold text-[#514A5D] transition hover:border-[#C9B8FF] hover:bg-white hover:text-[#5B21B6]"><Copy className="size-3.5" aria-hidden="true" />{copied ? text.copied : text.copy}</button></div></div></article>;
}

function buildModelCollections(items: PromptItem[]): Collection[] {
  const groups = new Map<string, Collection>();
  for (const item of items) {
    if (!item.model) continue;
    const current = groups.get(item.model);
    if (current) current.count += 1;
    else groups.set(item.model, { key: item.model, count: 1, sample: item });
  }
  return [...groups.values()].sort((a, b) => b.count - a.count || a.key.localeCompare(b.key));
}

function buildTopicCollections(items: PromptItem[]): Collection[] {
  const groups = new Map<string, Collection>();
  for (const item of items) {
    for (const tag of visibleTags(item)) {
      const current = groups.get(tag);
      if (current) current.count += 1;
      else groups.set(tag, { key: tag, count: 1, sample: item });
    }
  }
  return [...groups.values()].sort((a, b) => b.count - a.count || a.key.localeCompare(b.key));
}

function firstSearchValue(value: string | string[] | undefined) {
  return Array.isArray(value) ? value[0] ?? "" : value ?? "";
}

function collectionHref(locale: Locale, key: "model" | "useCase", value: string) {
  return `${localizePath("/prompts", locale)}?${key}=${encodeURIComponent(value)}#prompt-collection`;
}

function mediaCollectionHref(locale: Locale, category: PromptItem["category"]) {
  if (category === "image" || category === "video") return localizePath(`/prompts/${category}`, locale);
  return `${localizePath("/prompts", locale)}?type=${encodeURIComponent(category)}#prompt-collection`;
}

function promptHref(item: PromptItem, locale: Locale) {
  const base = item.category === "video" ? `/prompts/video/${item.slug}` : item.category === "image" ? `/prompts/image/${item.slug}` : `/prompts?type=${encodeURIComponent(item.category)}`;
  return localizePath(base, locale);
}

function isUsefulFacetTag(tag: string) {
  return !["image", "video", "audio", "text", "agent", "cli", "solvea", "github", "awesome-images"].includes(tag.toLowerCase());
}

function visibleTags(item: PromptItem) {
  return item.tags.filter(isUsefulFacetTag);
}

function ArtifactPreview({ artifact, title, variant = "card" }: { artifact: PromptArtifact; title: string; variant?: "card" | "hero" }) {
  const ratio = variant === "hero" ? "aspect-[16/9] h-full" : artifact.kind === "video" ? "aspect-[16/9]" : "aspect-[4/3]";
  if (artifact.kind === "image") return <div className={`relative overflow-hidden bg-[#EEE8FF] ${ratio}`}><Image src={artifact.url} alt={artifact.alt || title} fill sizes={variant === "hero" ? "(max-width: 1024px) 100vw, 640px" : "(max-width: 768px) 100vw, 33vw"} className="object-cover transition duration-500 group-hover:scale-[1.02]" unoptimized /></div>;
  if (artifact.kind === "video") return <div className={`relative overflow-hidden bg-[#E4DDF5] ${ratio}`}><Image src={artifact.poster || artifact.url} alt={artifact.alt || title} fill sizes="(max-width: 768px) 100vw, 50vw" className="object-cover" unoptimized /><span className="absolute inset-0 m-auto flex size-11 items-center justify-center rounded-full bg-white/90 text-violet-700 shadow-lg"><Sparkles className="size-4" /></span></div>;
  return <div className={`flex items-center justify-center bg-gradient-to-br from-[#F1EAFE] to-[#E8E4EE] p-6 text-center text-sm font-semibold text-[#5B21B6] ${ratio}`}><span>{title}</span></div>;
}
