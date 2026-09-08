"use client";

import { ArrowRight, Check, ChevronRight, Copy, ImageIcon, Sparkles, Video, type LucideIcon } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useMemo, useState, type ReactNode } from "react";
import { PromptFreeCta } from "@/components/prompt-free-cta";
import { SiteShell } from "@/components/site-shell";
import { localizePath, type Locale } from "@/lib/locales";
import type { PromptArtifact, PromptItem } from "@/lib/prompt-library";
import { getPromptDisplayCopy, localizePromptTag } from "@/lib/prompt-display-copy";
import { consoleUrl } from "@/lib/origins";

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

const discoveryCopy: Record<Locale, { slogan: string; imageModel: string; videoModel: string }> = {
  en: { slogan: "From idea to final frame, find prompts that already work.", imageModel: "An image model for posters, product visuals, and production-ready creative work.", videoModel: "A video model for ads, product stories, and motion-led creative work." },
  zh: { slogan: "从灵感到成片，找到真正能用的提示词。", imageModel: "适合海报、商品图与成片级视觉创意的图像生成模型。", videoModel: "适合广告短片、产品叙事与动态创意的视频生成模型。" },
  es: { slogan: "De la idea al resultado final: encuentra prompts que ya funcionan.", imageModel: "Un modelo de imagen para carteles, producto y piezas visuales listas para producción.", videoModel: "Un modelo de vídeo para anuncios, historias de producto y creatividad en movimiento." },
  fr: { slogan: "De l’idée au rendu final, trouvez des prompts qui fonctionnent déjà.", imageModel: "Un modèle d’image pour les affiches, les visuels produit et les créations prêtes à produire.", videoModel: "Un modèle vidéo pour les publicités, les récits produit et les créations en mouvement." },
  pt: { slogan: "Da ideia ao resultado final, encontre prompts que já funcionam.", imageModel: "Um modelo de imagem para pôsteres, produtos e peças visuais prontas para produção.", videoModel: "Um modelo de vídeo para anúncios, histórias de produto e criações em movimento." },
  ru: { slogan: "От идеи до готового кадра — найдите промпты, которые уже работают.", imageModel: "Модель изображений для постеров, товарной визуализации и готовых к выпуску креативов.", videoModel: "Видеомодель для рекламы, историй о продукте и динамичных креативов." },
  ja: { slogan: "アイデアから完成作品まで、実際に使えるプロンプトが見つかります。", imageModel: "ポスター、商品ビジュアル、制作向けクリエイティブに適した画像モデル。", videoModel: "広告、商品ストーリー、動きのあるクリエイティブに適した動画モデル。" },
  vi: { slogan: "Từ ý tưởng đến thành phẩm, khám phá những prompt đã được chứng minh hiệu quả.", imageModel: "Mô hình ảnh dành cho áp phích, hình sản phẩm và nội dung sáng tạo sẵn sàng sản xuất.", videoModel: "Mô hình video dành cho quảng cáo, câu chuyện sản phẩm và nội dung chuyển động." },
  de: { slogan: "Von der Idee bis zum fertigen Ergebnis: Finden Sie Prompts, die bereits funktionieren.", imageModel: "Ein Bildmodell für Poster, Produktvisuals und produktionsreife Kreativinhalte.", videoModel: "Ein Videomodell für Werbung, Produktgeschichten und bewegte Kreativinhalte." },
  id: { slogan: "Dari ide hingga hasil akhir, temukan prompt yang sudah terbukti efektif.", imageModel: "Model gambar untuk poster, visual produk, dan materi kreatif siap produksi.", videoModel: "Model video untuk iklan, cerita produk, dan materi kreatif berbasis gerak." },
};

const heroCopy: Record<Locale, { eyebrow: string; title: string; body: string; primary: string; secondary: string; cards: [string, string, string, string, string, string] }> = {
  en: { eyebrow: "PROMPT LIBRARY", title: "Test the prompt before production.", body: "Explore image and video prompts with real outputs, model context, and sources you can verify before you generate.", primary: "Browse featured prompts", secondary: "Open Playground", cards: ["Real outputs", "See the finished result", "Model context", "Know the model and source", "Free to try", "Take any prompt into Playground"] },
  zh: { eyebrow: "提示词库", title: "接生产前，先用真实提示词试出结果。", body: "浏览带真实产物、模型信息和可追溯来源的图片与视频提示词，确认效果后再开始生成。", primary: "浏览精选提示词", secondary: "进入 Playground", cards: ["真实产物", "先看最终效果", "模型与来源", "知道用的是什么、从哪里来", "免费试用", "把任意提示词带入 Playground"] },
  es: { eyebrow: "BIBLIOTECA DE PROMPTS", title: "Prueba el prompt antes de llevarlo a producción.", body: "Explora prompts de imagen y vídeo con resultados reales, contexto del modelo y fuentes verificables antes de generar.", primary: "Ver prompts destacados", secondary: "Abrir Playground", cards: ["Resultados reales", "Mira el resultado final", "Modelo y fuente", "Conoce el modelo y su origen", "Gratis para probar", "Lleva cualquier prompt al Playground"] },
  fr: { eyebrow: "BIBLIOTHÈQUE DE PROMPTS", title: "Testez le prompt avant de passer en production.", body: "Explorez des prompts image et vidéo accompagnés de résultats réels, du contexte du modèle et de sources vérifiables.", primary: "Voir les prompts à la une", secondary: "Ouvrir le Playground", cards: ["Résultats réels", "Voyez le rendu final", "Modèle et source", "Connaissez le modèle et son origine", "Essai gratuit", "Envoyez n’importe quel prompt dans le Playground"] },
  pt: { eyebrow: "BIBLIOTECA DE PROMPTS", title: "Teste o prompt antes de levar para produção.", body: "Explore prompts de imagem e vídeo com resultados reais, contexto do modelo e fontes verificáveis antes de gerar.", primary: "Ver prompts em destaque", secondary: "Abrir o Playground", cards: ["Resultados reais", "Veja o resultado final", "Modelo e fonte", "Saiba qual modelo foi usado e de onde veio", "Grátis para testar", "Leve qualquer prompt para o Playground"] },
  ru: { eyebrow: "БИБЛИОТЕКА ПРОМПТОВ", title: "Проверьте промпт до запуска в продакшен.", body: "Изучайте промпты для изображений и видео с реальными результатами, данными о модели и проверяемыми источниками.", primary: "Смотреть избранные промпты", secondary: "Открыть Playground", cards: ["Реальные результаты", "Сначала оцените готовый кадр", "Модель и источник", "Знайте, какая модель использовалась и откуда пример", "Бесплатная проба", "Откройте любой промпт в Playground"] },
  ja: { eyebrow: "プロンプトライブラリ", title: "本番投入の前に、プロンプトを試す。", body: "実際の成果物、モデル情報、確認できる出典付きの画像・動画プロンプトを見てから生成を始められます。", primary: "注目のプロンプトを見る", secondary: "Playground を開く", cards: ["実際の成果物", "完成結果を先に確認", "モデルと出典", "使われたモデルと出典が分かる", "無料で試せる", "どのプロンプトでも Playground へ送れる"] },
  vi: { eyebrow: "THƯ VIỆN PROMPT", title: "Thử prompt trước khi đưa vào sản xuất.", body: "Khám phá prompt hình ảnh và video có kết quả thực tế, thông tin model và nguồn có thể kiểm tra trước khi tạo.", primary: "Xem prompt nổi bật", secondary: "Mở Playground", cards: ["Kết quả thực tế", "Xem thành phẩm trước", "Model và nguồn", "Biết model nào đã dùng và ví dụ đến từ đâu", "Dùng thử miễn phí", "Đưa mọi prompt vào Playground"] },
  de: { eyebrow: "PROMPT-BIBLIOTHEK", title: "Teste den Prompt, bevor er in Produktion geht.", body: "Entdecken Sie Bild- und Video-Prompts mit echten Ergebnissen, Modellkontext und überprüfbaren Quellen vor der Generierung.", primary: "Ausgewählte Prompts ansehen", secondary: "Playground öffnen", cards: ["Echte Ergebnisse", "Das fertige Ergebnis zuerst sehen", "Modell und Quelle", "Modell und Herkunft des Beispiels kennen", "Kostenlos testen", "Jeden Prompt im Playground öffnen"] },
  id: { eyebrow: "PUSTAKA PROMPT", title: "Uji prompt sebelum masuk produksi.", body: "Jelajahi prompt gambar dan video dengan hasil nyata, konteks model, dan sumber yang dapat diverifikasi sebelum membuat.", primary: "Lihat prompt pilihan", secondary: "Buka Playground", cards: ["Hasil nyata", "Lihat hasil akhir lebih dulu", "Model dan sumber", "Ketahui model dan asal contohnya", "Gratis untuk dicoba", "Bawa prompt apa pun ke Playground"] },
};

const categoryIcons: Partial<Record<PromptItem["category"], LucideIcon>> = {
  image: ImageIcon,
  video: Video,
};

type Collection = { key: string; count: number; sample: PromptItem; items: PromptItem[] };

const modelPresentation: Record<string, { title: string; logo: string }> = {
  "gpt-image-2": { title: "GPT Image 2", logo: "/assets/logos/openai.svg" },
  "jimeng-image-4.5": { title: "Jimeng Image 4.5", logo: "/assets/logos/bytedance.svg" },
  "seedance-2.0": { title: "Seedance 2.0", logo: "/assets/logos/bytedance.svg" },
  "seedance-2.5": { title: "Seedance 2.5", logo: "/assets/logos/bytedance.svg" },
  "veo-3.1-fast-generate-preview": { title: "Veo 3.1 Fast", logo: "/assets/logos/googlegemini.svg" },
};

export function PromptDirectoryPage({ locale, items, initialSearch }: Props) {
  const text = copy[locale];
  const sections = sectionCopy[locale];
  const selectedType = firstSearchValue(initialSearch?.type);
  const selectedModel = firstSearchValue(initialSearch?.model);
  const selectedUseCase = firstSearchValue(initialSearch?.useCase);
  const [copied, setCopied] = useState("");
  const [showAll, setShowAll] = useState(false);

  const sortedItems = useMemo(
    () => [...items].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)),
    [items],
  );
  const mediaItems = useMemo(
    () => sortedItems.filter((item) => item.category === "image" || item.category === "video"),
    [sortedItems],
  );
  const mediaCollections = useMemo(() => {
    const order: PromptItem["category"][] = ["image", "video"];
    return order.flatMap((category) => {
      const categoryItems = mediaItems.filter((item) => item.category === category);
      const sample = categoryItems[0];
      return sample ? [{ category, count: categoryItems.length, sample }] : [];
    });
  }, [mediaItems]);
  const modelCollections = useMemo(() => buildModelCollections(mediaItems).slice(0, 8), [mediaItems]);
  const topicCollections = useMemo(() => buildTopicCollections(mediaItems).slice(0, 9), [mediaItems]);
  const collectionItems = useMemo(
    () => mediaItems.filter((item) => {
      if (selectedType && item.category !== selectedType) return false;
      if (selectedModel && item.model !== selectedModel) return false;
      if (selectedUseCase && !item.tags.includes(selectedUseCase)) return false;
      return true;
    }),
    [mediaItems, selectedModel, selectedType, selectedUseCase],
  );
  const collectionTitle = selectedModel
    || (selectedType ? categoryLabels[locale][selectedType] : "")
    || (selectedUseCase ? localizePromptTag(selectedUseCase, locale) : "")
    || sections.latest;
  const hasSelectedCollection = Boolean(selectedType || selectedModel || selectedUseCase);
  const displayedItems = hasSelectedCollection || showAll ? collectionItems : collectionItems.slice(0, 15);
  const playgroundHref = consoleUrl("/playground", new URLSearchParams({ lng: locale, source: "prompt-library" }).toString());
  const copyPrompt = async (item: PromptItem) => {
    await navigator.clipboard.writeText(item.prompt);
    setCopied(item.slug);
    window.setTimeout(() => setCopied(""), 1600);
  };

  return (
    <SiteShell locale={locale} pathname="/prompts">
      <main className="min-h-screen bg-white text-[#171a21]">
        <section className="border-b border-[#0B0B0F14] bg-[#F8F6FC] px-6 pt-10 pb-10 sm:px-8 md:pt-14 md:pb-14 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            <nav aria-label="Breadcrumb" className="mb-7 flex items-center gap-1 text-xs text-[#6B6475]">
              <Link href={localizePath("/", locale)} className="hover:text-violet-700">Flatkey</Link>
              <ChevronRight className="size-3" aria-hidden="true" />
              <span className="font-semibold text-[#0B0B0F]">{text.title}</span>
            </nav>
            <div className="mx-auto max-w-5xl px-2 py-10 text-center md:py-16">
              <p className="text-xs font-black tracking-[0.18em] text-[#6D28D9] uppercase">{heroCopy[locale].eyebrow}</p>
              <h1 className="mx-auto mt-6 max-w-5xl text-[clamp(2.8rem,7vw,6.8rem)] leading-[0.94] font-extrabold tracking-[-0.065em] text-[#0B0B0F]">{heroCopy[locale].title}</h1>
              <p className="mx-auto mt-7 max-w-3xl text-base leading-7 text-[#5F5B66] md:text-xl md:leading-8">{heroCopy[locale].body}</p>
              <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
                <Link href="#prompt-collection" className="inline-flex h-12 items-center gap-2 rounded-xl bg-[#070707] px-6 text-sm font-extrabold !text-white shadow-[0_18px_36px_-20px_rgba(11,11,15,.55)] transition-colors hover:bg-[#1a1a1d]">
                  {heroCopy[locale].primary}<ArrowRight className="size-4" aria-hidden="true" />
                </Link>
                <a href={playgroundHref} className="inline-flex h-12 items-center gap-2 rounded-xl border border-[#0B0B0F16] bg-white px-6 text-sm font-extrabold text-[#242129] shadow-[0_14px_28px_-22px_rgba(11,11,15,.3)] transition-colors hover:border-[#7C3AED45] hover:text-[#4C1D95]">
                  {heroCopy[locale].secondary}
                </a>
              </div>
            </div>
            <div className="grid gap-4 border-t border-[#0B0B0F10] pt-7 md:grid-cols-3">
              {[0, 1, 2].map((index) => {
                const offset = index * 2;
                return <div key={heroCopy[locale].cards[offset]} className="rounded-2xl border border-[#0B0B0F12] bg-white px-6 py-5 text-left shadow-[0_18px_48px_-38px_rgba(46,16,101,.28)]"><h2 className="text-xl font-black tracking-tight text-[#0B0B0F]">{heroCopy[locale].cards[offset]}</h2><p className="mt-2 text-sm font-semibold leading-6 text-[#77727F]">{heroCopy[locale].cards[offset + 1]}</p></div>;
              })}
            </div>
          </div>
        </section>

        <DirectorySection eyebrow={text.type} title={sections.media} tone="white">
          <div className="grid gap-4 md:grid-cols-2">
            {mediaCollections.map((collection) => <MediaCollectionCard key={collection.category} collection={collection} locale={locale} text={text} />)}
          </div>
        </DirectorySection>

        <DirectorySection eyebrow={text.model} title={sections.models} tone="tinted">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {modelCollections.map((collection) => <ModelCollectionCard key={collection.key} collection={collection} locale={locale} text={text} />)}
          </div>
        </DirectorySection>

        <DirectorySection eyebrow={text.useCase} title={sections.topics} tone="white">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {topicCollections.map((collection) => (
              <Link key={collection.key} href={collectionHref(locale, "useCase", collection.key)} className="group overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_24px_70px_-46px_rgba(46,16,101,.16)] transition hover:-translate-y-px hover:border-[#7C3AED35] hover:shadow-[0_24px_70px_-42px_rgba(46,16,101,.24)]">
                <div className="aspect-[16/9] overflow-hidden bg-[#211C2D]"><ArtifactPreview artifact={collection.sample.artifact} title={getPromptDisplayCopy(collection.sample, locale).title} variant="hero" /></div>
                <div className="flex items-center gap-3 p-4"><div className="min-w-0 flex-1"><h3 className="truncate text-sm font-black text-[#0B0B0F]">{localizePromptTag(collection.key, locale)}</h3><p className="mt-1 text-xs font-semibold text-[#77727F]">{collection.count} {text.found}</p></div><ArrowRight className="size-4 shrink-0 text-[#A29CAB] transition group-hover:translate-x-0.5 group-hover:text-[#5B21B6]" aria-hidden="true" /></div>
              </Link>
            ))}
          </div>
        </DirectorySection>

        <section id="prompt-collection" className="scroll-mt-28 border-y border-[#E7E4EC] bg-[#F8F6FC] px-6 py-10 sm:px-8 md:py-14 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
              <div><p className="text-xs font-black tracking-[0.14em] text-violet-600 uppercase">{hasSelectedCollection ? text.browse : text.featured}</p><h2 className="mt-2 text-2xl font-black tracking-tight text-[#0B0B0F] md:text-3xl">{collectionTitle}</h2><p className="mt-1 text-sm text-[#6B7280]">{collectionItems.length} {text.found}</p></div>
              {hasSelectedCollection ? <Link href={`${localizePath("/prompts", locale)}#prompt-collection`} className="inline-flex h-9 items-center rounded-full border border-[#0B0B0F14] bg-white px-4 text-xs font-extrabold text-[#403A48] shadow-[0_12px_28px_-22px_rgba(11,11,15,.32)] transition-colors hover:border-[#7C3AED35] hover:text-[#4C1D95]">{text.all}</Link> : null}
            </div>
            {collectionItems.length ? <><div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">{displayedItems.map((item) => <PromptDirectoryCard key={item.slug} item={item} locale={locale} text={text} copied={copied === item.slug} onCopy={() => copyPrompt(item)} />)}</div>{!hasSelectedCollection && !showAll && collectionItems.length > 15 ? <div className="mt-8 flex justify-center"><button type="button" onClick={() => setShowAll(true)} className="inline-flex h-11 items-center gap-2 rounded-full border border-[#0B0B0F14] bg-white px-5 text-sm font-extrabold text-[#28232E] shadow-[0_16px_34px_-24px_rgba(11,11,15,.32)] transition-colors hover:border-[#7C3AED35] hover:text-[#4C1D95]">{text.all}<ArrowRight className="size-4" aria-hidden="true" /></button></div> : null}</> : <div className="flex min-h-56 flex-col items-center justify-center rounded-2xl border border-[#0B0B0F14] bg-white px-6 py-14 text-center shadow-[0_24px_70px_-46px_rgba(46,16,101,.16)]"><h3 className="text-lg font-bold text-[#0B0B0F]">{text.noResults}</h3><p className="mt-2 text-sm text-[#6B7280]">{text.noResultsHint}</p><Link href={localizePath("/prompts", locale)} className="mt-5 inline-flex h-10 items-center rounded-full bg-[#070707] px-5 text-xs font-extrabold !text-white shadow-[0_16px_34px_-22px_rgba(11,11,15,.55)] hover:bg-[#1a1a1d]">{text.all}</Link></div>}
          </div>
        </section>
        <PromptFreeCta locale={locale} />
      </main>
    </SiteShell>
  );
}

function DirectorySection({ eyebrow, title, tone, children }: { eyebrow: string; title: string; tone: "white" | "tinted"; children: ReactNode }) {
  return <section className={`border-t border-[#E7E4EC] px-6 py-10 sm:px-8 md:py-14 lg:px-10 ${tone === "tinted" ? "bg-[#F8F6FC]" : "bg-white"}`}><div className="mx-auto max-w-[1280px]"><div className="mb-6"><p className="text-xs font-black tracking-[0.14em] text-violet-600 uppercase">{eyebrow}</p><h2 className="mt-2 text-2xl font-black tracking-tight text-[#0B0B0F] md:text-3xl">{title}</h2></div>{children}</div></section>;
}

function MediaCollectionCard({ collection, locale, text }: { collection: { category: PromptItem["category"]; count: number; sample: PromptItem }; locale: Locale; text: Copy }) {
  const Icon = categoryIcons[collection.category] ?? Sparkles;
  return <Link href={mediaCollectionHref(locale, collection.category)} className="group overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_24px_70px_-46px_rgba(46,16,101,.16)] transition hover:-translate-y-px hover:border-[#7C3AED35] hover:shadow-[0_24px_70px_-42px_rgba(46,16,101,.24)]"><div className="relative aspect-[16/9] overflow-hidden bg-[#EEE8FF]"><ArtifactPreview artifact={collection.sample.artifact} title={getPromptDisplayCopy(collection.sample, locale).title} variant="hero" /><span className="absolute top-3 left-3 flex size-9 items-center justify-center rounded-full border border-white/80 bg-white/92 text-[#5B21B6] shadow-[0_10px_24px_-16px_rgba(11,11,15,.4)] backdrop-blur"><Icon className="size-4" aria-hidden="true" /></span></div><div className="flex items-center justify-between gap-3 p-4"><div><h3 className="text-base font-black text-[#0B0B0F]">{categoryLabels[locale][collection.category]}</h3><p className="mt-1 text-xs font-semibold text-[#77727F]">{collection.count} {text.found}</p></div><ArrowRight className="size-4 text-[#A29CAB] transition group-hover:translate-x-0.5 group-hover:text-[#5B21B6]" aria-hidden="true" /></div></Link>;
}

function ModelCollectionCard({ collection, locale, text }: { collection: Collection; locale: Locale; text: Copy }) {
  const presentation = modelPresentation[collection.key] ?? { title: humanizeModelName(collection.key), logo: modelLogo(collection.key) };
  const description = collection.sample.category === "video" ? discoveryCopy[locale].videoModel : discoveryCopy[locale].imageModel;
  const tags = [...new Set(collection.items.flatMap(visibleTags))].slice(0, 3);
  return (
    <Link href={collectionHref(locale, "model", collection.key)} className="group flex min-h-60 flex-col rounded-2xl border border-[#0B0B0F14] bg-white p-5 shadow-[0_24px_70px_-46px_rgba(46,16,101,.16)] transition hover:-translate-y-px hover:border-[#7C3AED35] hover:shadow-[0_24px_70px_-42px_rgba(46,16,101,.24)]">
      <div className="flex items-start justify-between gap-4">
        <span className="grid size-11 place-items-center rounded-full border border-[#0B0B0F12] bg-[#F8F6FC] shadow-[0_10px_26px_-20px_rgba(11,11,15,.35)]">
          <Image src={presentation.logo} alt="" width={24} height={24} className="size-6 object-contain" aria-hidden="true" />
        </span>
        <span className="inline-flex items-center gap-2 text-xs font-bold text-[#77727F]">{collection.count} {text.found}<ArrowRight className="size-4 text-[#A29CAB] transition group-hover:translate-x-0.5 group-hover:text-[#5B21B6]" aria-hidden="true" /></span>
      </div>
      <h3 className="mt-5 text-lg font-black tracking-tight text-[#0B0B0F]">{presentation.title}</h3>
      <p className="mt-2 line-clamp-2 text-sm leading-6 text-[#68616F]">{description}</p>
      <div className="mt-auto flex flex-wrap gap-1.5 pt-5">
        {tags.map((tag) => <span key={tag} className="rounded-full bg-[#F1EAFE] px-2.5 py-1 text-[10px] font-bold text-[#5B21B6]">{localizePromptTag(tag, locale)}</span>)}
      </div>
    </Link>
  );
}

function PromptDirectoryCard({ item, locale, text, copied, onCopy }: { item: PromptItem; locale: Locale; text: Copy; copied: boolean; onCopy: () => void }) {
  const { title, summary } = getPromptDisplayCopy(item, locale);
  return (
    <article className="group overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_24px_70px_-46px_rgba(46,16,101,.16)] transition hover:-translate-y-px hover:border-[#7C3AED35] hover:shadow-[0_24px_70px_-42px_rgba(46,16,101,.24)]">
      <Link href={promptHref(item, locale)} className="block"><ArtifactPreview artifact={item.artifact} title={title} /></Link>
      <div className="p-4">
        <div className="mb-2 flex flex-wrap items-center gap-2"><span className="rounded-full bg-[#F1EAFE] px-2.5 py-1 text-[11px] font-bold text-[#5B21B6]">{item.model || categoryLabels[locale][item.category]}</span><span className="text-[11px] font-semibold text-[#8A8493]">{item.updatedAt}</span></div>
        <Link href={promptHref(item, locale)}><h3 className="line-clamp-2 min-h-10 text-base font-black leading-5 text-[#0B0B0F] transition-colors group-hover:text-[#32145F]">{title}</h3></Link>
        <p className="mt-2 line-clamp-2 text-sm leading-5 text-[#6B6475]">{summary}</p>
        <div className="mt-3 flex flex-wrap gap-1.5">{visibleTags(item).slice(0, 3).map((tag) => <span key={tag} className="rounded-md bg-[#F8F7FA] px-2 py-1 text-[10px] font-semibold text-[#77727F]">{localizePromptTag(tag, locale)}</span>)}</div>
        <div className="mt-4 flex items-center justify-between gap-3 border-t border-[#0B0B0F0F] pt-3"><Link href={promptHref(item, locale)} className="inline-flex h-8 items-center gap-1.5 rounded-full bg-[#070707] px-3.5 text-[11px] font-extrabold !text-white shadow-[0_14px_28px_-20px_rgba(11,11,15,.55)] transition-colors hover:bg-[#1a1a1d]">{text.view}<ArrowRight className="size-3.5" aria-hidden="true" /></Link><button type="button" onClick={onCopy} aria-label={copied ? text.copied : text.copy} title={copied ? text.copied : text.copy} aria-live="polite" className={`inline-flex size-8 items-center justify-center rounded-full border transition-colors ${copied ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-[#0B0B0F14] bg-white text-[#68616F] hover:border-[#7C3AED35] hover:bg-[#F8F6FC] hover:text-[#4C1D95]"}`}>{copied ? <Check className="size-3.5" aria-hidden="true" /> : <Copy className="size-3.5" aria-hidden="true" />}</button></div>
      </div>
    </article>
  );
}

function buildModelCollections(items: PromptItem[]): Collection[] {
  const groups = new Map<string, Collection>();
  for (const item of items) {
    if (!item.model) continue;
    const current = groups.get(item.model);
    if (current) {
      current.count += 1;
      current.items.push(item);
    } else groups.set(item.model, { key: item.model, count: 1, sample: item, items: [item] });
  }
  return [...groups.values()].sort((a, b) => b.count - a.count || a.key.localeCompare(b.key));
}

function buildTopicCollections(items: PromptItem[]): Collection[] {
  const groups = new Map<string, Collection>();
  for (const item of items) {
    for (const tag of visibleTags(item)) {
      const current = groups.get(tag);
      if (current) {
        current.count += 1;
        current.items.push(item);
      } else groups.set(tag, { key: tag, count: 1, sample: item, items: [item] });
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

function humanizeModelName(value: string) {
  return value.split(/[-_]+/).map((part) => part.length <= 3 ? part.toUpperCase() : `${part[0]?.toUpperCase() ?? ""}${part.slice(1)}`).join(" ");
}

function modelLogo(value: string) {
  const normalized = value.toLowerCase();
  if (normalized.includes("gpt") || normalized.includes("openai")) return "/assets/logos/openai.svg";
  if (normalized.includes("seedance") || normalized.includes("jimeng")) return "/assets/logos/bytedance.svg";
  if (normalized.includes("veo") || normalized.includes("gemini")) return "/assets/logos/googlegemini.svg";
  return "/assets/logos/flatkey-mark-dark.svg";
}

function ArtifactPreview({ artifact, title, variant = "card" }: { artifact: PromptArtifact; title: string; variant?: "card" | "hero" }) {
  const ratio = variant === "hero" ? "aspect-[16/9] h-full" : artifact.kind === "video" ? "aspect-[16/9]" : "aspect-[4/3]";
  if (artifact.kind === "image") return <div className={`relative overflow-hidden bg-[#EEE8FF] ${ratio}`}><Image src={artifact.url} alt={artifact.alt || title} fill sizes={variant === "hero" ? "(max-width: 1024px) 100vw, 640px" : "(max-width: 768px) 100vw, 33vw"} className="object-cover transition duration-500 group-hover:scale-[1.02]" unoptimized /></div>;
  if (artifact.kind === "video") return <div className={`relative overflow-hidden bg-[#211C2D] ${ratio}`}><video src={artifact.url} poster={artifact.poster} aria-label={artifact.alt || title} autoPlay muted loop playsInline preload={variant === "hero" ? "auto" : "metadata"} className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.02]" /></div>;
  return <div className={`flex items-center justify-center bg-gradient-to-br from-[#F1EAFE] to-[#E8E4EE] p-6 text-center text-sm font-semibold text-[#5B21B6] ${ratio}`}><span>{title}</span></div>;
}
