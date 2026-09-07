"use client";

import { Check, ChevronRight, Copy, Filter, Search, SlidersHorizontal, Sparkles } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { SiteShell } from "@/components/site-shell";
import { localizePath, type Locale } from "@/lib/locales";
import type { PromptArtifact, PromptItem } from "@/lib/prompt-library";

type Props = { locale: Locale; items: PromptItem[]; initialSearch?: Record<string, string | string[] | undefined> };
type Sort = "updated" | "oldest" | "complete";

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

export function PromptDirectoryPage({ locale, items, initialSearch }: Props) {
  const text = copy[locale];
  const initial = (key: string) => {
    const value = initialSearch?.[key];
    return Array.isArray(value) ? value[0] ?? "" : value ?? "";
  };
  const [query, setQuery] = useState(initial("q"));
  const [type, setType] = useState(initial("type"));
  const [model, setModel] = useState(initial("model"));
  const [useCase, setUseCase] = useState(initial("useCase"));
  const [source, setSource] = useState(initial("source"));
  const [sort, setSort] = useState<Sort>((initial("sort") as Sort) || "updated");
  const [mobileOpen, setMobileOpen] = useState(false);
  const [copied, setCopied] = useState("");

  const facets = useMemo(() => {
    const models = Array.from(new Set(items.map((item) => item.model).filter(Boolean))).sort();
    const uses = Array.from(new Set(items.flatMap((item) => item.tags))).filter(isUsefulFacetTag).sort();
    const sources = Array.from(new Set(items.map((item) => item.source.platform))).sort();
    return { models, uses, sources };
  }, [items]);

  const matched = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return items
      .filter((item) => !type || item.category === type)
      .filter((item) => !model || item.model === model)
      .filter((item) => !useCase || item.tags.includes(useCase))
      .filter((item) => !source || item.source.platform === source)
      .filter((item) => {
        if (!needle) return true;
        const title = item.title[locale] ?? item.title.en;
        const summary = item.summary[locale] ?? item.summary.en;
        return [title, summary, item.prompt, item.model, ...item.tags].join(" ").toLowerCase().includes(needle);
      })
      .sort((a, b) => {
        if (sort === "oldest") return Date.parse(a.updatedAt) - Date.parse(b.updatedAt);
        if (sort === "complete") return artifactScore(b.artifact) - artifactScore(a.artifact);
        return Date.parse(b.updatedAt) - Date.parse(a.updatedAt);
      });
  }, [items, locale, model, query, sort, source, type, useCase]);

  useEffect(() => {
    const params = new URLSearchParams();
    if (query) params.set("q", query);
    if (type) params.set("type", type);
    if (model) params.set("model", model);
    if (useCase) params.set("useCase", useCase);
    if (source) params.set("source", source);
    if (sort !== "updated") params.set("sort", sort);
    window.history.replaceState(null, "", `${localizePath("/prompts", locale)}${params.toString() ? `?${params}` : ""}`);
  }, [locale, model, query, sort, source, type, useCase]);

  const featured = matched[0] ?? items[0];
  const clear = () => { setQuery(""); setType(""); setModel(""); setUseCase(""); setSource(""); setSort("updated"); };
  const copyPrompt = async (item: PromptItem) => {
    await navigator.clipboard.writeText(item.prompt);
    setCopied(item.slug);
    window.setTimeout(() => setCopied(""), 1600);
  };

  const sidebar = (
    <div className="space-y-6">
      <div className="flex items-center justify-between"><h2 className="text-sm font-black text-[#0B0B0F]">{text.filter}</h2><button type="button" onClick={clear} className="text-xs font-bold text-violet-700 hover:text-violet-900">{text.reset}</button></div>
      <Facet title={text.type} values={Object.keys(categoryLabels[locale])} labels={categoryLabels[locale]} active={type} onChange={setType} />
      <Facet title={text.model} values={facets.models} active={model} onChange={setModel} />
      <Facet title={text.useCase} values={facets.uses.slice(0, 12)} active={useCase} onChange={setUseCase} />
      <Facet title={text.source} values={facets.sources} labels={facets.sources.reduce<Record<string, string>>((map, value) => { map[value] = sourceLabel(locale, value); return map; }, {})} active={source} onChange={setSource} />
    </div>
  );

  return (
    <SiteShell locale={locale} pathname="/prompts">
      <main className="min-h-screen bg-white text-[#171a21]">
        <section className="px-6 pt-10 pb-8 sm:px-8 md:pt-14 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            <nav aria-label="Breadcrumb" className="mb-6 flex items-center gap-1 text-xs text-[#6B6475]"><Link href={localizePath("/", locale)} className="hover:text-violet-700">Flatkey</Link><ChevronRight className="size-3" /><span className="font-semibold text-[#0B0B0F]">Prompts</span></nav>
            <div className="flex flex-wrap items-end justify-between gap-4"><div><p className="mb-3 text-xs font-black tracking-[0.16em] text-violet-600 uppercase">Flatkey Prompt Library</p><h1 className="text-[clamp(2.25rem,5vw,3.8rem)] leading-none font-extrabold tracking-tight">{text.title}</h1><p className="mt-4 max-w-2xl text-base leading-7 text-[#62626D]">{text.description}</p></div><div className="rounded-xl border border-[#E7E4EC] bg-[#FBFAFC] px-4 py-3 text-right"><p className="text-2xl font-black text-[#0B0B0F]">{items.length}</p><p className="text-xs font-semibold text-[#6B7280]">{text.found}</p></div></div>
          </div>
        </section>

        {featured ? <section className="px-6 pb-8 sm:px-8 lg:px-10"><div className="mx-auto grid max-w-[1280px] overflow-hidden rounded-2xl border border-[#E7E4EC] bg-[#F5F1FF] shadow-[0_20px_70px_-50px_rgba(76,29,149,.35)] lg:grid-cols-[1fr_1.05fr]"><div className="flex flex-col justify-center p-7 md:p-10"><p className="text-xs font-black tracking-[0.16em] text-violet-700 uppercase">{text.featured}</p><h2 className="mt-3 text-3xl font-black tracking-tight text-[#0B0B0F] md:text-4xl">{featured.title[locale] ?? featured.title.en}</h2><p className="mt-3 max-w-xl text-sm leading-6 text-[#5D566A]">{featured.summary[locale] ?? featured.summary.en}</p><div className="mt-5 flex flex-wrap items-center gap-2"><span className="rounded-full bg-white px-3 py-1 text-xs font-bold text-violet-700">{featured.model || categoryLabels[locale][featured.category]}</span>{visibleTags(featured).slice(0, 2).map((tag) => <span key={tag} className="rounded-full border border-violet-200 bg-white/60 px-3 py-1 text-xs font-semibold text-[#5D566A]">{tag}</span>)}</div><Link href={promptHref(featured, locale)} className="mt-7 inline-flex h-10 w-fit items-center gap-2 rounded-xl bg-[#6D28D9] px-4 text-sm font-bold text-white shadow-[0_12px_28px_-16px_rgba(91,33,182,.8)] hover:bg-[#5B21B6]">{text.view}<ChevronRight className="size-4" /></Link></div><Link href={promptHref(featured, locale)} className="relative min-h-64 bg-[#E9E1FF] lg:min-h-[300px]"><ArtifactPreview artifact={featured.artifact} title={featured.title[locale] ?? featured.title.en} variant="hero" /></Link></div></section> : null}

        <section className="border-y border-[#E7E4EC] bg-[#F8F6FC] px-6 py-8 sm:px-8 md:py-10 lg:px-10"><div className="mx-auto grid max-w-[1280px] gap-4 xl:grid-cols-[280px_minmax(0,1fr)]"><aside className="hidden rounded-2xl border border-[#E7E4EC] bg-white p-5 xl:block">{sidebar}</aside><div className="min-w-0 space-y-4"><div className="rounded-2xl border border-[#E7E4EC] bg-white p-4 shadow-sm"><div className="flex flex-col gap-3 lg:flex-row lg:items-center"><div className="relative min-w-0 flex-1"><Search className="absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-[#9CA3AF]" /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={text.search} aria-label={text.search} className="h-10 w-full rounded-xl border border-[#E7E4EC] bg-[#FBFAFC] px-4 pl-10 text-sm outline-none focus:border-[#C9B8FF] focus:ring-4 focus:ring-[#C9B8FF]/25" /></div><button type="button" onClick={() => setMobileOpen((value) => !value)} className="inline-flex h-10 items-center justify-center gap-2 rounded-xl border border-[#E7E4EC] px-4 text-xs font-bold xl:hidden"><Filter className="size-4" />{text.filter}</button><label className="inline-flex h-10 items-center gap-2 rounded-xl border border-[#E7E4EC] bg-white px-3 text-sm font-semibold text-[#45414C]"><SlidersHorizontal className="size-4 text-[#9CA3AF]" /><span className="sr-only">{text.sort}</span><select value={sort} onChange={(event) => setSort(event.target.value as Sort)} className="bg-transparent outline-none"><option value="updated">{text.updated}</option><option value="oldest">{text.oldest}</option><option value="complete">{text.complete}</option></select></label></div><div className="mt-3 flex items-center gap-3 border-t border-[#EFECF3] pt-3"><h2 className="text-[17px] font-black tracking-tight text-[#0B0B0F]">{text.all}</h2><span className="text-[13px] text-[#6B7280]">{matched.length} {text.found}</span></div></div>{mobileOpen ? <div className="rounded-2xl border border-[#E7E4EC] bg-white p-5 xl:hidden">{sidebar}</div> : null}{matched.length ? <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">{matched.map((item) => <PromptDirectoryCard key={item.slug} item={item} locale={locale} text={text} copied={copied === item.slug} onCopy={() => copyPrompt(item)} />)}</div> : <div className="flex min-h-64 flex-col items-center justify-center rounded-2xl border border-[#E7E4EC] bg-white px-6 py-14 text-center"><h3 className="text-lg font-bold text-[#0B0B0F]">{text.noResults}</h3><p className="mt-2 text-sm text-[#6B7280]">{text.noResultsHint}</p><button type="button" onClick={clear} className="mt-4 inline-flex h-9 items-center rounded-xl bg-[#6D28D9] px-4 text-xs font-bold text-white">{text.reset}</button></div>}</div></div></section>
      </main>
    </SiteShell>
  );
}

function Facet({ title, values, labels, active, onChange }: { title: string; values: string[]; labels?: Record<string, string>; active: string; onChange: (value: string) => void }) {
  return <div><p className="mb-2 text-xs font-black tracking-[0.12em] text-[#6B6475] uppercase">{title}</p><div className="space-y-1">{values.map((value) => <button type="button" key={value} onClick={() => onChange(active === value ? "" : value)} className={`flex w-full items-center justify-between rounded-lg px-2.5 py-2 text-left text-sm font-semibold ${active === value ? "bg-[#F1EAFE] text-[#5B21B6]" : "text-[#45414C] hover:bg-[#FBFAFC]"}`}><span className="truncate">{labels?.[value] ?? value}</span>{active === value ? <Check className="size-4" /> : null}</button>)}</div></div>;
}

function PromptDirectoryCard({ item, locale, text, copied, onCopy }: { item: PromptItem; locale: Locale; text: Copy; copied: boolean; onCopy: () => void }) {
  const title = item.title[locale] ?? item.title.en;
  const summary = item.summary[locale] ?? item.summary.en;
  return <article className="group overflow-hidden rounded-2xl border border-[#E7E4EC] bg-white shadow-[0_12px_32px_-26px_rgba(24,14,38,.2)] transition hover:-translate-y-0.5 hover:border-violet-300 hover:shadow-[0_20px_40px_-28px_rgba(91,33,182,.35)]"><Link href={promptHref(item, locale)} className="block"><ArtifactPreview artifact={item.artifact} title={title} /></Link><div className="p-4"><div className="mb-2 flex flex-wrap items-center gap-2"><span className="rounded-full bg-[#F1EAFE] px-2.5 py-1 text-[11px] font-bold text-[#5B21B6]">{item.model || categoryLabels[locale][item.category]}</span><span className="text-[11px] font-semibold text-[#8A8493]">{item.updatedAt}</span></div><Link href={promptHref(item, locale)}><h3 className="line-clamp-2 min-h-10 text-base font-black leading-5 text-[#0B0B0F]">{title}</h3></Link><p className="mt-2 line-clamp-2 text-sm leading-5 text-[#6B6475]">{summary}</p><div className="mt-3 flex flex-wrap gap-1.5">{visibleTags(item).slice(0, 3).map((tag) => <span key={tag} className="rounded-md bg-[#F8F7FA] px-2 py-1 text-[10px] font-semibold text-[#77727F]">{tag}</span>)}</div><div className="mt-4 flex items-center gap-2"><Link href={promptHref(item, locale)} className="inline-flex h-9 flex-1 items-center justify-center rounded-lg bg-[#6D28D9] px-3 text-xs font-bold text-white hover:bg-[#5B21B6]">{text.view}</Link><button type="button" onClick={onCopy} aria-label={text.copy} className="inline-flex h-9 items-center gap-1.5 rounded-lg border border-[#E7E4EC] px-3 text-xs font-bold text-[#514A5D] hover:border-violet-300 hover:text-[#5B21B6]"><Copy className="size-3.5" />{copied ? text.copied : text.copy}</button></div></div></article>;
}

function promptHref(item: PromptItem, locale: Locale) {
  const base = item.category === "video" ? `/prompts/video/${item.slug}` : item.category === "image" ? `/prompts/image/${item.slug}` : `/prompts?type=${encodeURIComponent(item.category)}`;
  return localizePath(base, locale);
}

function artifactScore(artifact: PromptArtifact) {
  if (artifact.kind === "image" || artifact.kind === "video") return 3;
  if (artifact.kind === "storyboard") return 2;
  return 1;
}

function isUsefulFacetTag(tag: string) {
  return !["image", "video", "audio", "text", "agent", "cli", "solvea", "github", "awesome-images"].includes(tag.toLowerCase());
}

function visibleTags(item: PromptItem) {
  return item.tags.filter(isUsefulFacetTag);
}

function sourceLabel(locale: Locale, value: string) {
  if (value === "GitHub") return "GitHub";
  if (value === "Local migration") {
    const labels: Partial<Record<Locale, string>> = { zh: "自有生产素材", ja: "自社制作素材", fr: "Production Flatkey", de: "Eigene Produktion" };
    return labels[locale] ?? "Owned production";
  }
  return value;
}

function ArtifactPreview({ artifact, title, variant = "card" }: { artifact: PromptArtifact; title: string; variant?: "card" | "hero" }) {
  const ratio = variant === "hero" ? "aspect-[16/9] h-full" : artifact.kind === "video" ? "aspect-[16/9]" : "aspect-[4/3]";
  if (artifact.kind === "image") return <div className={`relative overflow-hidden bg-[#EEE8FF] ${ratio}`}><Image src={artifact.url} alt={artifact.alt || title} fill sizes={variant === "hero" ? "(max-width: 1024px) 100vw, 640px" : "(max-width: 768px) 100vw, 33vw"} className="object-cover transition duration-500 group-hover:scale-[1.02]" unoptimized /></div>;
  if (artifact.kind === "video") return <div className={`relative overflow-hidden bg-[#E4DDF5] ${ratio}`}><Image src={artifact.poster || artifact.url} alt={artifact.alt || title} fill sizes="(max-width: 768px) 100vw, 50vw" className="object-cover" unoptimized /><span className="absolute inset-0 m-auto flex size-11 items-center justify-center rounded-full bg-white/90 text-violet-700 shadow-lg"><Sparkles className="size-4" /></span></div>;
  return <div className={`flex items-center justify-center bg-gradient-to-br from-[#F1EAFE] to-[#E8E4EE] p-6 text-center text-sm font-semibold text-[#5B21B6] ${ratio}`}><span>{title}</span></div>;
}
