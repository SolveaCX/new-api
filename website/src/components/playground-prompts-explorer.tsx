"use client";

import {
  ArrowRight,
  BookOpen,
  Boxes,
  CheckCircle2,
  Clipboard,
  Copy,
  FileText,
  Code2,
  Image as ImageIcon,
  KeyRound,
  Layers3,
  type LucideIcon,
  Play,
  Search,
  Sparkles,
  Video,
} from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useMemo, useState } from "react";
import { CLI_IMAGE_PATH, CLI_VIDEO_PATH } from "@/lib/cli-landing";
import { type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";
import { type PromptArtifact, promptLibraryCopy, type PromptItem } from "@/lib/prompt-library";

type Category = PromptItem["category"];

type PlaygroundExplorerCopy = {
  all: string;
  artifactPairs: string;
  copied: string;
  copyPrompt: string;
  empty: string;
  featured: string;
  heroGalleryLabel: string;
  imageDatasetStat: string;
  loadMore: string;
  model: string;
  openConsole: string;
  prompt: string;
  promptCount: string;
  promptRowsStat: string;
  run: string;
  searchPlaceholder: string;
  weeklyHot: string;
};

const copyByLocale: Record<Locale, PlaygroundExplorerCopy> = withIdFallback({
  en: {
    all: "All",
    artifactPairs: "prompt-output pairs",
    copied: "Copied",
    copyPrompt: "Copy",
    empty: "No matching prompts yet.",
    featured: "Featured",
    heroGalleryLabel: "Visible outputs",
    imageDatasetStat: "DiffusionDB image pairs screened",
    loadMore: "Load more",
    model: "Model",
    openConsole: "Open console",
    prompt: "Prompt",
    promptCount: "free prompts",
    promptRowsStat: "prompts.chat CC0 rows sampled",
    run: "Run with Flatkey",
    searchPlaceholder: "Search prompt, model, tag, source...",
    weeklyHot: "Weekly hot",
  },
  zh: {
    all: "全部",
    artifactPairs: "提示词-产物配对",
    copied: "已复制",
    copyPrompt: "复制",
    empty: "暂时没有匹配的提示词。",
    featured: "精选",
    heroGalleryLabel: "可见产物",
    imageDatasetStat: "已筛选 DiffusionDB 图像配对",
    loadMore: "加载更多",
    model: "模型",
    openConsole: "打开控制台",
    prompt: "提示词",
    promptCount: "免费提示词",
    promptRowsStat: "prompts.chat CC0 行抽样",
    run: "用 Flatkey 运行",
    searchPlaceholder: "搜索提示词、模型、标签、来源...",
    weeklyHot: "每周热门",
  },
  es: {
    all: "Todo",
    artifactPairs: "pares prompt-resultado",
    copied: "Copiado",
    copyPrompt: "Copiar",
    empty: "Aun no hay prompts coincidentes.",
    featured: "Destacado",
    heroGalleryLabel: "Resultados visibles",
    imageDatasetStat: "pares de imagen DiffusionDB filtrados",
    loadMore: "Cargar mas",
    model: "Modelo",
    openConsole: "Abrir consola",
    prompt: "Prompt",
    promptCount: "prompts gratis",
    promptRowsStat: "filas CC0 de prompts.chat muestreadas",
    run: "Ejecutar con Flatkey",
    searchPlaceholder: "Buscar prompt, modelo, etiqueta, fuente...",
    weeklyHot: "Populares",
  },
  fr: {
    all: "Tout",
    artifactPairs: "paires prompt-resultat",
    copied: "Copie",
    copyPrompt: "Copier",
    empty: "Aucun prompt correspondant.",
    featured: "Selection",
    heroGalleryLabel: "Resultats visibles",
    imageDatasetStat: "paires image DiffusionDB filtrees",
    loadMore: "Charger plus",
    model: "Modele",
    openConsole: "Ouvrir la console",
    prompt: "Prompt",
    promptCount: "prompts gratuits",
    promptRowsStat: "lignes CC0 prompts.chat echantillonnees",
    run: "Lancer avec Flatkey",
    searchPlaceholder: "Rechercher prompt, modele, tag, source...",
    weeklyHot: "Populaires",
  },
  pt: {
    all: "Tudo",
    artifactPairs: "pares prompt-resultado",
    copied: "Copiado",
    copyPrompt: "Copiar",
    empty: "Nenhum prompt encontrado.",
    featured: "Destaque",
    heroGalleryLabel: "Resultados visiveis",
    imageDatasetStat: "pares de imagem DiffusionDB filtrados",
    loadMore: "Carregar mais",
    model: "Modelo",
    openConsole: "Abrir console",
    prompt: "Prompt",
    promptCount: "prompts gratis",
    promptRowsStat: "linhas CC0 do prompts.chat amostradas",
    run: "Executar com Flatkey",
    searchPlaceholder: "Buscar prompt, modelo, tag, fonte...",
    weeklyHot: "Populares",
  },
  ru: {
    all: "Все",
    artifactPairs: "пары prompt-output",
    copied: "Скопировано",
    copyPrompt: "Копировать",
    empty: "Подходящих prompts нет.",
    featured: "Избранное",
    heroGalleryLabel: "Видимые результаты",
    imageDatasetStat: "пары изображений DiffusionDB отфильтрованы",
    loadMore: "Показать ещё",
    model: "Модель",
    openConsole: "Открыть консоль",
    prompt: "Prompt",
    promptCount: "бесплатных prompts",
    promptRowsStat: "выборка строк prompts.chat CC0",
    run: "Запустить через Flatkey",
    searchPlaceholder: "Искать prompt, model, tag, source...",
    weeklyHot: "Популярное",
  },
  ja: {
    all: "すべて",
    artifactPairs: "prompt-output ペア",
    copied: "コピー済み",
    copyPrompt: "コピー",
    empty: "一致するプロンプトはありません。",
    featured: "注目",
    heroGalleryLabel: "見える成果物",
    imageDatasetStat: "DiffusionDB 画像ペアを選別",
    loadMore: "さらに表示",
    model: "モデル",
    openConsole: "コンソールを開く",
    prompt: "Prompt",
    promptCount: "無料プロンプト",
    promptRowsStat: "prompts.chat CC0 行をサンプリング",
    run: "Flatkeyで実行",
    searchPlaceholder: "prompt、model、tag、sourceを検索...",
    weeklyHot: "人気",
  },
  vi: {
    all: "Tat ca",
    artifactPairs: "cap prompt-output",
    copied: "Da copy",
    copyPrompt: "Copy",
    empty: "Chua co prompt phu hop.",
    featured: "Noi bat",
    heroGalleryLabel: "Ket qua hien thi",
    imageDatasetStat: "cap anh DiffusionDB da loc",
    loadMore: "Tai them",
    model: "Model",
    openConsole: "Mo console",
    prompt: "Prompt",
    promptCount: "prompt mien phi",
    promptRowsStat: "dong prompts.chat CC0 da lay mau",
    run: "Chay voi Flatkey",
    searchPlaceholder: "Tim prompt, model, tag, source...",
    weeklyHot: "Hot trong tuan",
  },
  de: {
    all: "Alle",
    artifactPairs: "Prompt-Output-Paare",
    copied: "Kopiert",
    copyPrompt: "Kopieren",
    empty: "Keine passenden Prompts.",
    featured: "Featured",
    heroGalleryLabel: "Sichtbare Ergebnisse",
    imageDatasetStat: "DiffusionDB-Bildpaare gefiltert",
    loadMore: "Mehr laden",
    model: "Modell",
    openConsole: "Konsole offnen",
    prompt: "Prompt",
    promptCount: "kostenlose Prompts",
    promptRowsStat: "prompts.chat CC0-Zeilen gesampelt",
    run: "Mit Flatkey ausfuhren",
    searchPlaceholder: "Prompt, Modell, Tag, Quelle suchen...",
    weeklyHot: "Beliebt",
  },
});

const categoryOrder: Category[] = ["image", "video", "text", "agent", "audio"];

const categoryIcon: Record<Category, LucideIcon> = {
  image: ImageIcon,
  video: Video,
  audio: Layers3,
  text: FileText,
  agent: Code2,
};

const initialVisibleCount = 60;
const visibleCountStep = 60;

export function PlaygroundPromptsExplorer(props: { items: PromptItem[]; locale: Locale }) {
  const [activeCategory, setActiveCategory] = useState<Category | "all">("all");
  const [query, setQuery] = useState("");
  const [copiedSlug, setCopiedSlug] = useState<string | null>(null);
  const [visibleCount, setVisibleCount] = useState(initialVisibleCount);
  const libraryCopy = promptLibraryCopy[props.locale] ?? promptLibraryCopy.en;
  const copy = copyByLocale[props.locale] ?? copyByLocale.en;
  const items = useMemo(() => sortPromptItems(props.items.filter(hasArtifact)), [props.items]);
  const categories = categoryOrder.filter((category) => items.some((item) => item.category === category));
  const categoryCounts = new Map<Category, number>();

  for (const item of items) {
    categoryCounts.set(item.category, (categoryCounts.get(item.category) ?? 0) + 1);
  }

  const filteredItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return items.filter((item) => {
      if (activeCategory !== "all" && item.category !== activeCategory) return false;
      if (!normalizedQuery) return true;
      return searchableText(item, props.locale).includes(normalizedQuery);
    });
  }, [activeCategory, items, props.locale, query]);

  const weeklyItems = filteredItems.slice(0, 8);
  const visibleFilteredItems = filteredItems.slice(0, visibleCount);

  async function copyPrompt(item: PromptItem) {
    try {
      await navigator.clipboard.writeText(item.prompt);
      setCopiedSlug(item.slug);
      window.setTimeout(() => setCopiedSlug(null), 1600);
    } catch {
      setCopiedSlug(null);
    }
  }

  return (
    <>
      <style>{`
        .playgroundHero{position:relative;overflow:hidden;border-bottom:1px solid var(--line);background:linear-gradient(180deg,#ffffff 0%,#f7f5fd 100%)}
        .playgroundHero:before{content:"";position:absolute;inset:0;pointer-events:none;background-image:linear-gradient(to right,rgba(124,58,237,.055) 1px,transparent 1px),linear-gradient(to bottom,rgba(124,58,237,.055) 1px,transparent 1px);background-size:72px 72px;mask-image:linear-gradient(to bottom,rgba(0,0,0,.72),transparent 88%)}
        .playgroundHeroIn{position:relative;z-index:1;width:100%;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:92px var(--fk-site-gutter) 60px;display:grid;grid-template-columns:minmax(0,1fr) minmax(440px,520px);gap:40px;align-items:center}
        .playgroundHeroCopy{max-width:720px}
        .playgroundEyebrow{display:inline-flex;align-items:center;gap:8px;padding:7px 14px;border-radius:999px;background:var(--violet-tint);color:var(--violet-deep);font:600 12px/1 var(--mono);letter-spacing:.04em;text-transform:none}
        .playgroundHeroTitle{max-width:720px;margin:18px 0 0;color:var(--ink);font-family:var(--disp);font-size:clamp(42px,4.5vw,62px);line-height:1.02;font-weight:700;letter-spacing:-.055em;text-wrap:balance}
        .playgroundHeroBody{max-width:560px;margin-top:18px;color:#615b64;font-size:17px;line-height:1.65}
        .playgroundHeroCtas{display:flex;gap:12px;flex-wrap:wrap;margin-top:28px}
        .playgroundHeroCtas .btn{min-height:42px;border-radius:8px;font-size:14px;font-weight:750}
        .playgroundOutputRail{width:100%;min-width:0}
        .playgroundHeroMedia{display:block;min-width:0;overflow:hidden;border:1px solid #e7e2f1;border-radius:18px;background:#f7f4fc;color:var(--ink);box-shadow:0 26px 72px -50px rgba(46,16,101,.28);text-decoration:none}
        .playgroundHeroMediaFrame{position:relative;aspect-ratio:16/9;overflow:hidden}
        .playgroundHeroMediaFrame img{display:block;width:100%;height:100%;object-fit:contain;transition:filter .35s ease}
        .playgroundHeroMedia:hover .playgroundHeroMediaFrame img{filter:saturate(1.02)}
        .playgroundBand{position:relative;overflow:hidden;border-bottom:1px solid var(--line);background:#fff}
        .playgroundBand.soft{background:var(--home-surface)}
        .playgroundBand.gradient{background:linear-gradient(180deg,#f7f5fd 0%,#fcfbff 100%)}
        .playgroundIn{width:100%;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:68px var(--fk-site-gutter);position:relative;z-index:1}
        .playgroundSectionHead{display:grid;grid-template-columns:minmax(0,1fr) minmax(280px,.56fr);gap:34px;align-items:end;margin-bottom:28px}
        .playgroundSectionHead p{color:#615b64;font-size:15.5px;line-height:1.65}
        .playgroundKicker{display:inline-flex;align-items:center;gap:8px;padding:7px 13px;border-radius:999px;background:var(--violet-tint);color:var(--violet-deep);font:600 12px/1 var(--mono);letter-spacing:.04em;text-transform:none}
        .playgroundSectionTitle{margin-top:15px;font-family:var(--disp);font-size:clamp(34px,3.8vw,52px);line-height:1.02;font-weight:700;letter-spacing:-.055em;text-wrap:balance}
        .promptToolbar{position:sticky;top:0;z-index:8;border-bottom:1px solid var(--line);background:rgba(255,255,255,.92);backdrop-filter:blur(10px)}
        .promptToolbarIn{width:100%;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:15px var(--fk-site-gutter);display:grid;grid-template-columns:minmax(0,1fr) auto;gap:16px;align-items:center}
        .promptSearch{position:relative}
        .promptSearch svg{position:absolute;left:15px;top:50%;transform:translateY(-50%);color:#6f6873}
        .promptSearch input{width:100%;height:46px;border:1px solid var(--line);border-radius:8px;background:#fff;padding:0 16px 0 46px;color:var(--ink);font:650 14px/1 var(--sans);outline:none;box-shadow:0 18px 50px -44px rgba(30,22,40,.48)}
        .promptSearch input:focus{border-color:#8e75c6}
        .categoryTabs{display:flex;gap:8px;overflow-x:auto}
        .categoryTab{height:40px;display:inline-flex;align-items:center;gap:8px;flex:none;border:1px solid var(--line);border-radius:999px;background:#fff;color:#625c66;padding:0 12px;font:750 12px/1 var(--sans);cursor:pointer;transition:background .18s ease,border-color .18s ease,color .18s ease}
        .categoryTab.on{background:#111014;border-color:#111014;color:#fff}
        .categoryTab em{font-style:normal;border-radius:5px;background:rgba(95,39,194,.08);padding:4px 6px;color:inherit;font:750 10px/1 var(--mono)}
        .categoryTab.on em{background:rgba(255,255,255,.16)}
        .promptGrid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px}
        .promptGrid.full{grid-template-columns:repeat(3,minmax(0,1fr))}
        .promptCard{overflow:hidden;border:1px solid var(--line);border-radius:8px;background:#fff;color:var(--ink);box-shadow:0 24px 66px -54px rgba(22,16,30,.52);transition:transform .18s ease,border-color .18s ease}
        .promptCard:hover{transform:translateY(-2px);border-color:#c6bbd6}
        .promptCardMediaLink{display:block;text-decoration:none;color:inherit}
        .promptCardBody{padding:17px}
        .promptMeta{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-bottom:13px}
        .promptBadge{display:inline-flex;align-items:center;border-radius:999px;background:var(--violet-tint);padding:6px 8px;color:var(--violet-deep);font:850 10px/1 var(--mono);text-transform:uppercase}
        .promptModel{display:inline-flex;border-radius:999px;background:#f0ecf8;padding:6px 8px;color:#5f27c2;font:800 10px/1 var(--mono)}
        .promptTitle{font-family:var(--disp);font-size:18px;line-height:1.16}
        .promptSummary{margin-top:9px;display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden;color:#625c66;font-size:13px;line-height:1.55}
        .promptPreview{margin-top:15px;overflow:hidden;border:1px solid var(--line);border-radius:12px;background:#faf8fc;color:var(--ink)}
        .promptPreviewHead{height:36px;display:flex;align-items:center;justify-content:space-between;border-bottom:1px solid var(--line);padding:0 9px 0 12px;background:#fff}
        .promptPreviewHead span{color:#7b7480;font:750 10px/1 var(--mono);text-transform:uppercase}
        .copyIconButton{width:30px;height:30px;display:grid;place-items:center;border:0;border-radius:6px;background:transparent;color:#6f6873;cursor:pointer}
        .copyIconButton:hover{background:rgba(124,58,237,.08);color:#4f2bb4}
        .promptPreview pre{max-height:128px;overflow:auto;margin:0;padding:12px;white-space:pre-wrap;color:var(--ink);font:500 11.5px/1.55 var(--mono)}
        .promptActions{display:flex;flex-wrap:wrap;gap:8px;margin-top:15px}
        .promptActions .btn{min-height:38px;border-radius:8px;padding:8px 11px;font-size:12.5px}
        .promptCopied{display:inline-flex;align-items:center;min-height:38px;border-radius:8px;background:#f0ecf8;padding:0 11px;color:#5f27c2;font-size:12px;font-weight:800}
        .artifactSurface{position:relative;display:flex;align-items:center;justify-content:center;overflow:hidden;background:#f2f0f7}
        .artifactSurface.hero{aspect-ratio:16/10}
        .artifactSurface.compact{aspect-ratio:4/3}
        .artifactSurface.default{aspect-ratio:16/10}
        .artifactSurface img,.artifactSurface video{display:block;height:100%;width:100%;max-width:100%;object-fit:contain}
        .videoPlay{position:absolute;right:14px;bottom:14px;width:38px;height:38px;display:grid;place-items:center;border-radius:999px;border:1px solid rgba(255,255,255,.32);background:rgba(255,255,255,.92);color:#101014}
        .codeArtifact,.storyArtifact,.textArtifact{min-height:180px;overflow:visible;padding:18px}
        .codeArtifact{background:#111014;color:#f4f0ff}
        .artifactLang{display:flex;align-items:center;gap:8px;margin-bottom:12px;color:#d9ef6e;font:800 11px/1 var(--mono);text-transform:uppercase}
        .codeArtifact pre{overflow:visible;margin:0;white-space:pre-wrap;font:500 11px/1.6 var(--mono)}
        .storyArtifact{display:grid;grid-template-columns:repeat(auto-fit,minmax(92px,1fr));gap:6px;background:#12121a}
        .storyFrame{overflow:visible;border:1px solid rgba(255,255,255,.12);border-radius:6px;background:rgba(255,255,255,.08);padding:8px;color:#f4f0ff;font-size:10px;line-height:1.35}
        .storyFrame b{display:block;margin-bottom:4px;color:#d9ef6e;font:800 10px/1 var(--mono)}
        .textArtifact{background:#f7f7f2}
        .textArtifactIcon{width:40px;height:40px;display:grid;place-items:center;border-radius:8px;background:#fff;color:#5f27c2;box-shadow:0 16px 34px -28px rgba(36,25,50,.55)}
        .textArtifact b{display:block;margin-top:14px;font-size:13px}
        .textArtifact p{margin-top:10px;overflow:visible;color:#625c66;font-size:12px;line-height:1.55;white-space:pre-line}
        .emptyState{border:1px dashed rgba(11,11,15,.18);border-radius:16px;background:#fff;padding:42px;text-align:center;color:#625c66;font-weight:700}
        .loadMoreRow{display:flex;justify-content:center;margin-top:28px}
        .loadMoreRow .btn{min-height:44px;border-radius:8px}
        @media(max-width:1050px){.playgroundHeroIn,.playgroundSectionHead,.promptToolbarIn{grid-template-columns:1fr}.playgroundHeroTitle{font-size:60px}.playgroundOutputRail{max-width:760px}.promptGrid,.promptGrid.full{grid-template-columns:repeat(2,minmax(0,1fr))}.categoryTabs{padding-bottom:2px}}
        @media(max-width:700px){.playgroundHeroIn{width:100%;max-width:100vw;grid-template-columns:minmax(0,1fr);padding:72px var(--fk-site-gutter) 52px;gap:34px;min-width:0;overflow:hidden}.playgroundHeroCopy,.playgroundOutputRail{width:100%;max-width:100%;inline-size:100%;max-inline-size:100%;min-width:0}.playgroundHeroTitle{width:100%;max-width:100%;font-size:42px;line-height:1.05;overflow-wrap:anywhere;word-break:normal;text-wrap:wrap}.playgroundHeroBody{width:100%;max-width:100%;font-size:15.5px;overflow-wrap:anywhere}.playgroundHeroCtas{width:100%;flex-direction:column}.playgroundHeroCtas .btn{width:100%}.playgroundIn{padding:56px var(--fk-site-gutter)}.promptToolbarIn{padding:14px var(--fk-site-gutter)}.promptGrid,.promptGrid.full{grid-template-columns:1fr}.playgroundSectionTitle{font-size:36px}.promptPreview pre{max-height:116px}}
        @media(max-width:480px){.playgroundHeroTitle{font-size:39px}.playgroundHeroBody{font-size:15px}}
      `}</style>
      <header className="hero heroUnified playgroundHero">
        <div className="playgroundHeroIn">
          <div className="playgroundHeroCopy">
            <span className="playgroundEyebrow">
              <Sparkles size={14} />
              {libraryCopy.heroBadge}
            </span>
            <h1 className="playgroundHeroTitle">{heroTitle(libraryCopy.heroTitle)}</h1>
            <p className="playgroundHeroBody">{libraryCopy.heroBody}</p>
            <div className="playgroundHeroCtas">
              <a className="btn black" href={consoleUrl("/playground", `lng=${props.locale}`)}>
                {copy.openConsole}
                <ArrowRight size={16} />
              </a>
              <Link className="btn white" href={localizePath(CLI_IMAGE_PATH, props.locale)}>
                <BookOpen size={16} />
                {libraryCopy.categories.image}
              </Link>
            </div>
          </div>
          <aside className="playgroundOutputRail" aria-label={copy.heroGalleryLabel}>
            <Link className="playgroundHeroMedia" href={localizePath(CLI_IMAGE_PATH, props.locale)}>
              <div className="playgroundHeroMediaFrame">
                <Image
                  priority
                  src="/assets/cli/campaign-hero.png"
                  alt="Campaign hero image showing a product bottle on a stone pedestal"
                  fill
                  sizes="(min-width: 1024px) 48vw, 100vw"
                />
              </div>
            </Link>
          </aside>
        </div>
      </header>

      <main>
        <div className="promptToolbar">
          <div className="promptToolbarIn">
            <label className="promptSearch">
              <Search size={18} />
              <input
                onChange={(event) => {
                  setQuery(event.target.value);
                  setVisibleCount(initialVisibleCount);
                }}
                placeholder={copy.searchPlaceholder}
                value={query}
              />
            </label>
            <div className="categoryTabs">
              <button
                className={`categoryTab${activeCategory === "all" ? " on" : ""}`}
                onClick={() => {
                  setActiveCategory("all");
                  setVisibleCount(initialVisibleCount);
                }}
                type="button"
              >
                <Boxes size={16} />
                {copy.all}
                <em>{items.length}</em>
              </button>
              {categories.map((category) => {
                const Icon = categoryIcon[category];
                const active = activeCategory === category;
                return (
                  <button
                    className={`categoryTab${active ? " on" : ""}`}
                    key={category}
                    onClick={() => {
                      setActiveCategory(category);
                      setVisibleCount(initialVisibleCount);
                    }}
                    type="button"
                  >
                    <Icon size={16} />
                    {libraryCopy.categories[category]}
                    <em>{categoryCounts.get(category) ?? 0}</em>
                  </button>
                );
              })}
            </div>
          </div>
        </div>

        <section className="playgroundBand">
          <div className="playgroundIn">
            <div className="playgroundSectionHead">
              <div>
                <div className="playgroundKicker">{copy.weeklyHot}</div>
                <h2 className="playgroundSectionTitle">{copy.featured}</h2>
              </div>
              <p>{libraryCopy.heroBody}</p>
            </div>
            {weeklyItems.length > 0 ? (
              <div className="promptGrid">
                {weeklyItems.map((item) => (
                  <PromptCard copied={copiedSlug === item.slug} copy={copy} item={item} key={item.slug} locale={props.locale} onCopy={copyPrompt} />
                ))}
              </div>
            ) : (
              <EmptyState copy={copy} />
            )}
          </div>
        </section>

        <section className="playgroundBand soft">
          <div className="playgroundIn">
            <div className="playgroundSectionHead">
              <div>
                <div className="playgroundKicker">{copy.featured}</div>
                <h2 className="playgroundSectionTitle">{`${filteredItems.length} ${copy.artifactPairs}`}</h2>
              </div>
              <p>{libraryCopy.heroBody}</p>
            </div>
            {filteredItems.length > 0 ? (
              <>
                <div className="promptGrid full">
                  {visibleFilteredItems.map((item) => (
                    <PromptCard copied={copiedSlug === item.slug} copy={copy} item={item} key={item.slug} locale={props.locale} onCopy={copyPrompt} variant="full" />
                  ))}
                </div>
                {visibleFilteredItems.length < filteredItems.length ? (
                  <div className="loadMoreRow">
                    <button className="btn black" onClick={() => setVisibleCount((count) => count + visibleCountStep)} type="button">
                      {copy.loadMore}
                      <ArrowRight size={15} />
                    </button>
                  </div>
                ) : null}
              </>
            ) : (
              <EmptyState copy={copy} />
            )}
          </div>
        </section>
      </main>
    </>
  );
}

function heroTitle(value: string) {
  const [first, ...rest] = value.split(/，|, /);
  if (rest.length === 0) return value;
  return (
    <>
      {first}
      <br />
      <em>{rest.join("，")}</em>
    </>
  );
}

function PromptCard(props: {
  copied: boolean;
  copy: PlaygroundExplorerCopy;
  item: PromptItem;
  locale: Locale;
  onCopy: (item: PromptItem) => void;
  variant?: "full";
}) {
  const title = props.item.title.en;
  const summary = props.item.summary.en;
  const libraryCopy = promptLibraryCopy[props.locale] ?? promptLibraryCopy.en;
  const href = itemHref(props.item, props.locale);
  const preview = <ArtifactPreview artifact={props.item.artifact} title={title} variant={props.variant === "full" ? undefined : "compact"} />;

  return (
    <article className="promptCard">
      {isExternalHref(href) ? (
        <a aria-label={title} className="promptCardMediaLink" href={href} target="_blank" rel="noopener noreferrer">
          {preview}
        </a>
      ) : (
        <Link aria-label={title} className="promptCardMediaLink" href={href}>
          {preview}
        </Link>
      )}
      <div className="promptCardBody">
        <div className="promptMeta">
          <span className="promptBadge">{libraryCopy.categories[props.item.category]}</span>
          <span className="promptModel">{props.item.model}</span>
        </div>
        <h3 className="promptTitle">{title}</h3>
        <p className="promptSummary">{summary}</p>
        <div className="promptPreview">
          <div className="promptPreviewHead">
            <span>{props.copy.prompt}</span>
            <button
              className="copyIconButton"
              onClick={(event) => {
                event.preventDefault();
                props.onCopy(props.item);
              }}
              title={props.copy.copyPrompt}
              type="button"
            >
              {props.copied ? <CheckCircle2 size={16} color="#D9EF6E" /> : <Copy size={16} />}
            </button>
          </div>
          <pre><code>{props.item.prompt}</code></pre>
        </div>
        <div className="promptActions">
          <a className="btn black" href={runHref(props.item, props.locale)}>
            <KeyRound size={15} />
            {props.copy.run}
          </a>
          {props.copied ? <span className="promptCopied">{props.copy.copied}</span> : null}
        </div>
      </div>
    </article>
  );
}

function ArtifactPreview(props: { artifact: PromptArtifact; title: string; variant?: "compact" | "hero" }) {
  const surfaceClass = `artifactSurface ${props.variant ?? "default"}`;

  if (props.artifact.kind === "image") {
    return (
      <div className={surfaceClass}>
        <Image src={props.artifact.url} alt={props.artifact.alt} width={1600} height={1200} sizes="(min-width: 1024px) 33vw, 100vw" />
      </div>
    );
  }

  if (props.artifact.kind === "video") {
    return (
      <div className={`${surfaceClass} videoArtifact`}>
        <video aria-label={props.artifact.alt} autoPlay loop muted playsInline poster={props.artifact.poster} preload="metadata">
          <source src={props.artifact.url} type="video/mp4" />
        </video>
        <span className="videoPlay">
          <Play size={16} fill="currentColor" />
        </span>
      </div>
    );
  }

  if (props.artifact.kind === "code") {
    return (
      <div className="codeArtifact">
        <div className="artifactLang">
          <Code2 size={16} />
          {props.artifact.language}
        </div>
        <pre><code>{props.artifact.code}</code></pre>
      </div>
    );
  }

  if (props.artifact.kind === "storyboard") {
    return (
      <div className="storyArtifact">
        {props.artifact.frames.map((frame, index) => (
          <div className="storyFrame" key={`${frame}-${index}`}>
            <b>{index + 1}</b>
            {frame}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="textArtifact">
      <div className="textArtifactIcon">
        <Clipboard size={20} />
      </div>
      <b>{props.artifact.title || props.title}</b>
      <p>{props.artifact.body}</p>
    </div>
  );
}

function EmptyState(props: { copy: PlaygroundExplorerCopy }) {
  return <div className="emptyState">{props.copy.empty}</div>;
}

function localized(value: Record<Locale, string>, locale: Locale) {
  return value[locale] ?? value.en;
}

function hasArtifact(item: PromptItem): boolean {
  if (item.artifact.kind === "image") return Boolean(item.artifact.url);
  if (item.artifact.kind === "video") return Boolean(item.artifact.url);
  if (item.artifact.kind === "text") return Boolean(item.artifact.body);
  if (item.artifact.kind === "code") return Boolean(item.artifact.code);
  if (item.artifact.kind === "storyboard") return item.artifact.frames.length > 0;
  return false;
}

function sortPromptItems(items: PromptItem[]) {
  return items
    .map((item, index) => ({ index, item }))
    .sort((a, b) => itemDisplayRank(a.item) - itemDisplayRank(b.item) || a.index - b.index)
    .map(({ item }) => item);
}

function itemDisplayRank(item: PromptItem) {
  const sourceLabel = item.source.label.toLowerCase();
  let rank = 100;

  if (sourceLabel.includes("awesome-images")) rank = 0;
  else if (item.category === "video") rank = 10;
  else if (item.source.platform === "Local migration") rank = 20;
  else if (item.tags.includes("prompts-chat")) rank = 35;
  else if (item.source.platform === "Official docs") rank = 45;
  else if (item.source.platform === "Social") rank = 55;
  else if (item.tags.includes("diffusiondb") && !isBulkImportedItem(item)) rank = 65;
  else if (item.source.platform === "Hugging Face") rank = 90;

  if (item.artifact.kind === "image") rank -= 6;
  if (item.artifact.kind === "video") rank -= 4;
  if (isBulkImportedItem(item)) rank += 90;

  return rank;
}

function searchableText(item: PromptItem, locale: Locale) {
  return [
    item.slug,
    item.model,
    item.prompt,
    item.source.label,
    item.source.platform,
    item.tags.join(" "),
    localized(item.title, locale),
    localized(item.summary, locale),
  ]
    .join(" ")
    .toLowerCase();
}

function itemHref(item: PromptItem, locale: Locale) {
  if (isBulkImportedItem(item)) return item.source.url;
  if (item.category === "image") return localizePath(`${CLI_IMAGE_PATH}/${item.slug}`, locale);
  if (item.category === "video") return localizePath(`${CLI_VIDEO_PATH}/${item.slug}`, locale);
  return runHref(item, locale);
}

function runHref(item: PromptItem, locale: Locale) {
  const params = new URLSearchParams({
    lng: locale,
    model: item.model,
    prompt: item.prompt,
    source: "flatkey-prompt-library",
    slug: item.slug,
  });
  return consoleUrl("/playground", params.toString());
}

function isBulkImportedItem(item: PromptItem) {
  return item.tags.includes("bulk-import");
}

function isExternalHref(href: string) {
  return href.startsWith("http://") || href.startsWith("https://");
}
