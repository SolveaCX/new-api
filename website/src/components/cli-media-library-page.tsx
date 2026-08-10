import { ArrowLeft, ArrowRight, ExternalLink, KeyRound, Play, Sparkles } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { CliPromptActionPanel } from "@/components/cli-prompt-action-panel";
import { SiteShell } from "@/components/site-shell";
import { CLI_IMAGE_PATH, CLI_LANDING_PATH, CLI_VIDEO_PATH } from "@/lib/cli-landing";
import { getCliMediaPromptItem, getCliMediaPromptItems, type PromptArtifact, type PromptItem } from "@/lib/prompt-library";
import { type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type MediaKind = "image" | "video";

type CliMediaCopy = {
  artifact: string;
  back: string;
  browseImage: string;
  browseVideo: string;
  createKey: string;
  empty: string;
  filters: string[];
  heroBadge: string;
  heroBody: string;
  heroTitle: string;
  model: string;
  prompt: string;
  source: string;
  viewSource: string;
};

const copyByLocale: Record<Locale, Record<MediaKind, CliMediaCopy>> = withIdFallback({
  en: {
    image: {
      artifact: "Produced image",
      back: "Back to CLI",
      browseImage: "Image",
      browseVideo: "Video",
      createKey: "Create API key",
      empty: "No image prompts with paired artifacts yet.",
      filters: ["Characters", "Product visuals", "Storyboards", "Campaign assets"],
      heroBadge: "CLI image prompt library",
      heroBody: "Image prompts collected from Flatkey projects and owned production assets. Every card includes the prompt, model, provenance, and finished image.",
      heroTitle: "Image prompts with output already attached.",
      model: "Model",
      prompt: "Prompt",
      source: "Source",
      viewSource: "View source",
    },
    video: {
      artifact: "Produced video artifact",
      back: "Back to CLI",
      browseImage: "Image",
      browseVideo: "Video",
      createKey: "Create API key",
      empty: "No video prompts with paired artifacts yet.",
      filters: ["UGC ads", "Product reveal", "Localization", "Storyboard to motion"],
      heroBadge: "CLI video prompt library",
      heroBody: "Video prompts shaped for Flatkey CLI runs. Only entries with an existing clip, poster, or production storyboard are shown.",
      heroTitle: "Video prompts paired with usable outputs.",
      model: "Model",
      prompt: "Prompt",
      source: "Source",
      viewSource: "View source",
    },
  },
  zh: {
    image: {
      artifact: "图像产物",
      back: "返回 CLI",
      browseImage: "图像",
      browseVideo: "视频",
      createKey: "创建 API key",
      empty: "还没有已配对产物的图像提示词。",
      filters: ["角色", "产品视觉", "分镜", "活动素材"],
      heroBadge: "CLI 图像提示词库",
      heroBody: "图像提示词来自 Flatkey 项目和自有生产素材迁移。每张卡都包含提示词、模型、归属和完成图。",
      heroTitle: "图像提示词必须带着产物一起展示。",
      model: "模型",
      prompt: "提示词",
      source: "来源",
      viewSource: "查看来源",
    },
    video: {
      artifact: "视频产物",
      back: "返回 CLI",
      browseImage: "图像",
      browseVideo: "视频",
      createKey: "创建 API key",
      empty: "还没有已配对产物的视频提示词。",
      filters: ["UGC 广告", "产品揭幕", "本地化", "分镜转视频"],
      heroBadge: "CLI 视频提示词库",
      heroBody: "视频提示词面向 Flatkey CLI 生产流程整理。这里只展示已经有短片、封面或生产分镜产物的条目。",
      heroTitle: "视频提示词必须对应可用产物。",
      model: "模型",
      prompt: "提示词",
      source: "来源",
      viewSource: "查看来源",
    },
  },
  es: {
    image: {
      artifact: "Imagen producida",
      back: "Volver a CLI",
      browseImage: "Imagen",
      browseVideo: "Video",
      createKey: "Crear API key",
      empty: "Aun no hay prompts de imagen con resultados.",
      filters: ["Personajes", "Producto", "Storyboards", "Campanas"],
      heroBadge: "Prompts de imagen para CLI",
      heroBody: "Prompts de imagen de proyectos Flatkey y assets propios migrados. Cada tarjeta incluye prompt, modelo, procedencia e imagen.",
      heroTitle: "Prompts de imagen con resultado incluido.",
      model: "Modelo",
      prompt: "Prompt",
      source: "Fuente",
      viewSource: "Ver fuente",
    },
    video: {
      artifact: "Artefacto de video",
      back: "Volver a CLI",
      browseImage: "Imagen",
      browseVideo: "Video",
      createKey: "Crear API key",
      empty: "Aun no hay prompts de video con resultados.",
      filters: ["UGC ads", "Reveal", "Localizacion", "Storyboard"],
      heroBadge: "Prompts de video para CLI",
      heroBody: "Prompts de video para flujos Flatkey CLI. Solo mostramos entradas con clip, poster o storyboard.",
      heroTitle: "Prompts de video con salidas utilizables.",
      model: "Modelo",
      prompt: "Prompt",
      source: "Fuente",
      viewSource: "Ver fuente",
    },
  },
  fr: {
    image: { artifact: "Image produite", back: "Retour au CLI", browseImage: "Image", browseVideo: "Video", createKey: "Creer API key", empty: "Aucun prompt image avec resultat.", filters: ["Personnages", "Produit", "Storyboards", "Campagnes"], heroBadge: "Prompts image CLI", heroBody: "Prompts image issus de projets Flatkey et d'assets propres migres, avec prompt, modele, source et image.", heroTitle: "Prompts image avec resultat attache.", model: "Modele", prompt: "Prompt", source: "Source", viewSource: "Voir source" },
    video: { artifact: "Artefact video", back: "Retour au CLI", browseImage: "Image", browseVideo: "Video", createKey: "Creer API key", empty: "Aucun prompt video avec resultat.", filters: ["UGC ads", "Reveal", "Localisation", "Storyboard"], heroBadge: "Prompts video CLI", heroBody: "Prompts video pour Flatkey CLI. Seules les entrees avec clip, poster ou storyboard sont affichees.", heroTitle: "Prompts video avec sorties utilisables.", model: "Modele", prompt: "Prompt", source: "Source", viewSource: "Voir source" },
  },
  pt: {
    image: { artifact: "Imagem gerada", back: "Voltar ao CLI", browseImage: "Imagem", browseVideo: "Video", createKey: "Criar API key", empty: "Nenhum prompt de imagem com resultado.", filters: ["Personagens", "Produto", "Storyboards", "Campanhas"], heroBadge: "Prompts de imagem CLI", heroBody: "Prompts de imagem de projetos Flatkey e assets próprios migrados, com prompt, modelo, fonte e imagem.", heroTitle: "Prompts de imagem com resultado.", model: "Modelo", prompt: "Prompt", source: "Fonte", viewSource: "Ver fonte" },
    video: { artifact: "Artefato de video", back: "Voltar ao CLI", browseImage: "Imagem", browseVideo: "Video", createKey: "Criar API key", empty: "Nenhum prompt de video com resultado.", filters: ["UGC ads", "Reveal", "Localizacao", "Storyboard"], heroBadge: "Prompts de video CLI", heroBody: "Prompts de video para Flatkey CLI. So entradas com clipe, poster ou storyboard aparecem.", heroTitle: "Prompts de video com saidas usaveis.", model: "Modelo", prompt: "Prompt", source: "Fonte", viewSource: "Ver fonte" },
  },
  ru: {
    image: { artifact: "Готовое изображение", back: "Назад к CLI", browseImage: "Image", browseVideo: "Video", createKey: "Create API key", empty: "Нет image prompts с результатом.", filters: ["Characters", "Product", "Storyboards", "Campaigns"], heroBadge: "CLI image prompts", heroBody: "Image prompts из Flatkey проектов и owned assets, с prompt, model, source и готовым изображением.", heroTitle: "Image prompts с готовым output.", model: "Model", prompt: "Prompt", source: "Source", viewSource: "View source" },
    video: { artifact: "Video artifact", back: "Назад к CLI", browseImage: "Image", browseVideo: "Video", createKey: "Create API key", empty: "Нет video prompts с результатом.", filters: ["UGC ads", "Reveal", "Localization", "Storyboard"], heroBadge: "CLI video prompts", heroBody: "Video prompts для Flatkey CLI. Показываем только записи с clip, poster или storyboard.", heroTitle: "Video prompts с usable outputs.", model: "Model", prompt: "Prompt", source: "Source", viewSource: "View source" },
  },
  ja: {
    image: { artifact: "生成済み画像", back: "CLIへ戻る", browseImage: "画像", browseVideo: "動画", createKey: "API keyを作成", empty: "成果物付き画像プロンプトはまだありません。", filters: ["キャラクター", "商品画像", "Storyboard", "Campaign"], heroBadge: "CLI画像プロンプト", heroBody: "Flatkeyプロジェクトと自社制作素材から整理した画像プロンプト。prompt、model、source、画像を表示します。", heroTitle: "成果物付き画像プロンプト。", model: "Model", prompt: "Prompt", source: "Source", viewSource: "Sourceを見る" },
    video: { artifact: "動画成果物", back: "CLIへ戻る", browseImage: "画像", browseVideo: "動画", createKey: "API keyを作成", empty: "成果物付き動画プロンプトはまだありません。", filters: ["UGC ads", "Reveal", "Localization", "Storyboard"], heroBadge: "CLI動画プロンプト", heroBody: "Flatkey CLI向け動画プロンプト。clip、poster、storyboardのあるものだけを表示します。", heroTitle: "使える出力付き動画プロンプト。", model: "Model", prompt: "Prompt", source: "Source", viewSource: "Sourceを見る" },
  },
  vi: {
    image: { artifact: "Anh da tao", back: "Quay lai CLI", browseImage: "Hinh anh", browseVideo: "Video", createKey: "Tao API key", empty: "Chua co prompt anh kem ket qua.", filters: ["Nhan vat", "San pham", "Storyboard", "Campaign"], heroBadge: "Prompt anh CLI", heroBody: "Prompt anh tu du an Flatkey va owned assets, kem prompt, model, source va anh.", heroTitle: "Prompt anh co san output.", model: "Model", prompt: "Prompt", source: "Source", viewSource: "View source" },
    video: { artifact: "Video artifact", back: "Quay lai CLI", browseImage: "Hinh anh", browseVideo: "Video", createKey: "Tao API key", empty: "Chua co prompt video kem ket qua.", filters: ["UGC ads", "Reveal", "Localization", "Storyboard"], heroBadge: "Prompt video CLI", heroBody: "Prompt video cho Flatkey CLI. Chi hien entry co clip, poster hoac storyboard.", heroTitle: "Prompt video kem output dung duoc.", model: "Model", prompt: "Prompt", source: "Source", viewSource: "View source" },
  },
  de: {
    image: { artifact: "Erzeugtes Bild", back: "Zuruck zur CLI", browseImage: "Bild", browseVideo: "Video", createKey: "API key erstellen", empty: "Keine Bild-Prompts mit Ergebnis.", filters: ["Characters", "Produkt", "Storyboards", "Campaigns"], heroBadge: "CLI Bild-Prompts", heroBody: "Bild-Prompts aus Flatkey-Projekten und eigenen Assets, mit Prompt, Modell, Quelle und Bild.", heroTitle: "Bild-Prompts mit Ergebnis.", model: "Modell", prompt: "Prompt", source: "Quelle", viewSource: "Quelle ansehen" },
    video: { artifact: "Video-Artefakt", back: "Zuruck zur CLI", browseImage: "Bild", browseVideo: "Video", createKey: "API key erstellen", empty: "Keine Video-Prompts mit Ergebnis.", filters: ["UGC ads", "Reveal", "Localization", "Storyboard"], heroBadge: "CLI Video-Prompts", heroBody: "Video-Prompts fur Flatkey CLI. Nur Eintrage mit Clip, Poster oder Storyboard werden gezeigt.", heroTitle: "Video-Prompts mit nutzbaren Outputs.", model: "Modell", prompt: "Prompt", source: "Quelle", viewSource: "Quelle ansehen" },
  },
});

const cliMediaGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const cliMediaCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const cliMediaMutedClass = "text-[#5C5861] dark:text-white/62";

export function cliMediaPath(kind: MediaKind) {
  return kind === "image" ? CLI_IMAGE_PATH : CLI_VIDEO_PATH;
}

export function cliMediaDetailPath(kind: MediaKind, slug: string) {
  return `${cliMediaPath(kind)}/${slug}`;
}

export function getCliMediaMetadata(kind: MediaKind, locale: Locale) {
  const copy = copyByLocale[locale][kind];
  const label = kind === "image" ? copy.browseImage : copy.browseVideo;
  return {
    title: `Flatkey CLI ${label} prompts`,
    description: copy.heroBody,
    pathname: cliMediaPath(kind),
  };
}

export function getCliMediaDetailMetadata(kind: MediaKind, slug: string, locale: Locale) {
  const item = getCliMediaPromptItem(kind, slug);
  if (!item) return undefined;
  const title = item.title[locale] ?? item.title.en;
  const summary = item.summary[locale] ?? item.summary.en;
  return {
    title: `${title} - Flatkey CLI ${kind} prompt`,
    description: summary,
    pathname: cliMediaDetailPath(kind, slug),
  };
}

export function CliMediaLibraryPage(props: { kind: MediaKind; locale: Locale }) {
  const copy = copyByLocale[props.locale][props.kind];
  const items = getCliMediaPromptItems(props.kind);
  const featuredItem = items[0];
  const weeklyItems = items.slice(0, 4);
  const curatedItems = items.slice(0, 8);
  const keyUrl = consoleUrl("/keys", `lng=${props.locale}`);
  const currentPath = cliMediaPath(props.kind);
  const displayName = props.kind === "image" ? copy.browseImage : copy.browseVideo;
  const isVideo = props.kind === "video";

  return (
    <SiteShell locale={props.locale} pathname={currentPath}>
      <main className="cli-media-page fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] px-4 pt-[var(--fk-header-safe-area)] pb-20 text-[#101014] antialiased sm:px-6 dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={cliMediaGridClass} />
        <section className="relative z-10 border-b-2 border-[#101014] py-10 md:py-14 dark:border-white/20">
          <div className="mx-auto max-w-[2160px]">
            <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
              <Link href={localizePath(CLI_LANDING_PATH, props.locale)} className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-white px-3 py-1.5 text-sm font-black text-[#101014] shadow-[3px_3px_0_#101014] hover:bg-[#F9F871] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                <ArrowLeft className="size-4" />
                {copy.back}
              </Link>
            </div>
            <div className={`grid gap-8 lg:items-center ${isVideo ? "lg:grid-cols-[0.7fr_1.3fr]" : "lg:grid-cols-[0.78fr_1.22fr]"}`}>
              <div>
                <p className="mb-4 inline-flex rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">{copy.heroBadge}</p>
                <h1 className="max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
                  {displayName}
                  <span className="block text-[#5852FF] dark:text-[#C8A8FF]">{props.locale === "zh" ? "提示词" : "Prompts"}</span>
                </h1>
                <p className={`mt-5 max-w-2xl text-sm leading-6 md:text-base ${cliMediaMutedClass}`}>{copy.heroBody}</p>
                <div className="mt-5 flex flex-wrap gap-2">
                  {copy.filters.map((filter) => (
                    <span key={filter} className="rounded-full border border-[#101014]/18 bg-white/72 px-3 py-1.5 text-xs font-black text-[#43434C] shadow-[2px_2px_0_rgba(16,16,20,0.12)] dark:border-white/14 dark:bg-white/8 dark:text-white/70">{filter}</span>
                  ))}
                </div>
              </div>
              {featuredItem ? <FeaturedPreview copy={copy} item={featuredItem} kind={props.kind} locale={props.locale} /> : null}
            </div>
          </div>
        </section>

        <section className="relative z-10 py-12">
          <div className="mx-auto max-w-[2160px]">
            {items.length === 0 ? (
              <div className={`${cliMediaCardClass} p-8 text-sm ${cliMediaMutedClass}`}>{copy.empty}</div>
            ) : (
              <>
                <SectionHeading eyebrow={props.locale === "zh" ? "每周热门" : "Weekly Hot"} title={props.locale === "zh" ? `热门${displayName}提示词` : `Popular ${displayName.toLowerCase()} prompts`} />
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  {weeklyItems.map((item) => (
                    <CompactPromptCard item={item} key={item.slug} kind={props.kind} locale={props.locale} />
                  ))}
                </div>

                <div className="mt-14">
                  <SectionHeading eyebrow={props.locale === "zh" ? "精选案例" : "Curated"} title={props.locale === "zh" ? `适合真实产物的${displayName}提示词` : `${displayName} prompts with production-ready outputs`} />
                  <div className={isVideo ? "grid gap-5 md:grid-cols-2 xl:grid-cols-3" : "columns-1 gap-5 md:columns-2 xl:columns-3"}>
                    {curatedItems.map((item) => (
                      <PromptCard copy={copy} item={item} key={item.slug} keyUrl={keyUrl} kind={props.kind} locale={props.locale} />
                    ))}
                  </div>
                </div>
              </>
            )}
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

export function CliMediaPromptDetailPage(props: { kind: MediaKind; locale: Locale; slug: string }) {
  const copy = copyByLocale[props.locale][props.kind];
  const item = getCliMediaPromptItem(props.kind, props.slug);
  const keyUrl = consoleUrl("/keys", `lng=${props.locale}`);

  if (!item) return null;

  const title = item.title[props.locale] ?? item.title.en;
  const summary = item.summary[props.locale] ?? item.summary.en;
  const currentPath = cliMediaDetailPath(props.kind, item.slug);
  const listPath = localizePath(cliMediaPath(props.kind), props.locale);
  const relatedItems = getCliMediaPromptItems(props.kind).filter((candidate) => candidate.slug !== item.slug).slice(0, 3);
  const isVideo = props.kind === "video";
  const generateParams = new URLSearchParams({
    generate: props.kind,
    lng: props.locale,
    prompt: item.prompt,
    source: `flatkey-cli-${props.kind}-prompt`,
    slug: item.slug,
  });
  const generateUrl = consoleUrl("/playground", generateParams.toString());

  return (
    <SiteShell locale={props.locale} pathname={currentPath}>
      <main className="cli-media-detail-page fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] px-4 pt-[var(--fk-header-safe-area)] pb-20 text-[#101014] antialiased sm:px-6 dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={cliMediaGridClass} />
        <section className="relative z-10 border-b-2 border-[#101014] py-10 md:py-14 dark:border-white/20">
          <div className="mx-auto max-w-[2160px]">
            <Link href={listPath} className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-white px-3 py-1.5 text-sm font-black text-[#101014] shadow-[3px_3px_0_#101014] hover:bg-[#F9F871] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
              <ArrowLeft className="size-4" />
              {props.locale === "zh" ? "返回列表" : copy.back}
            </Link>
            <div className="mt-6 max-w-4xl">
              <div className="mb-4 flex flex-wrap gap-2">
                <span className="inline-flex items-center gap-1 rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1 text-xs font-black text-[#101014] shadow-[2px_2px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white"><Sparkles className="size-3.5" />{copy.artifact}</span>
                <span className="rounded-full border border-[#101014]/16 bg-white/72 px-3 py-1 text-xs font-black text-[#5C5861] dark:border-white/14 dark:bg-white/8 dark:text-white/62">{item.updatedAt}</span>
              </div>
              <h1 className="text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">{title}</h1>
              <p className={`mt-5 max-w-2xl text-base leading-7 ${cliMediaMutedClass}`}>{summary}</p>
            </div>
            <div className={`mt-8 grid gap-6 lg:items-start ${isVideo ? "lg:grid-cols-[minmax(0,1.25fr)_300px]" : "lg:grid-cols-[minmax(0,1fr)_320px]"}`}>
              <article className={`${cliMediaCardClass} overflow-hidden`}>
                <ArtifactPreview artifact={item.artifact} title={title} variant="detail" />
              </article>
              <aside className="space-y-4">
                <div className={`${cliMediaCardClass} p-5`}>
                  <div className="grid grid-cols-2 gap-3">
                    <DetailMetric label={props.locale === "zh" ? "类型" : "Type"} value={props.kind === "image" ? copy.browseImage : copy.browseVideo} />
                    <DetailMetric label={props.locale === "zh" ? "更新" : "Updated"} value={item.updatedAt} />
                  </div>
                  <div className="mt-4 grid gap-4 border-t border-[#0B0B0F10] pt-4">
                    <SourceInfo copy={copy} item={item} locale={props.locale} />
                  </div>
                  <div className="mt-5 flex flex-wrap gap-2">
                    {visibleTags(item).map((tag) => (
                      <span key={tag} className="rounded-full border border-[#101014]/16 bg-white/72 px-3 py-1 text-xs font-black text-[#43434C] dark:border-white/14 dark:bg-white/8 dark:text-white/70">{tag}</span>
                    ))}
                  </div>
                </div>
                <a className="flatkey-primary-cta flex items-center justify-between px-4 py-3 text-sm" href={keyUrl}>
                  <span className="inline-flex items-center gap-2"><KeyRound className="size-4" />{copy.createKey}</span>
                  <ArrowRight className="size-4" />
                </a>
              </aside>
            </div>
          </div>
        </section>

        <section className="relative z-10 py-12">
          <div className="mx-auto grid max-w-[2160px] gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
            <CliPromptActionPanel
              defaultPrompt={item.prompt}
              generateUrl={generateUrl}
              kind={props.kind}
              locale={props.locale}
              model={item.model}
              ratio={item.output.ratio}
              title={copy.prompt}
            />
            <aside className="space-y-4">
              <SourcePanel copy={copy} item={item} locale={props.locale} />
            </aside>
          </div>
        </section>

        {relatedItems.length > 0 ? (
          <section className="relative z-10 pb-16">
            <div className="mx-auto max-w-[2160px]">
              <SectionHeading eyebrow={props.locale === "zh" ? "继续浏览" : "More"} title={props.locale === "zh" ? "相关提示词产物" : "Related prompt outputs"} />
              <div className="grid gap-4 md:grid-cols-3">
                {relatedItems.map((related) => (
                  <CompactPromptCard item={related} key={related.slug} kind={props.kind} locale={props.locale} />
                ))}
              </div>
            </div>
          </section>
        ) : null}
      </main>
    </SiteShell>
  );
}

function SectionHeading(props: { eyebrow: string; title: string }) {
  return (
    <div className="mb-5 flex items-end justify-between gap-4">
      <div>
        <p className="font-mono text-xs font-black tracking-normal text-[#7C3AED] uppercase dark:text-[#C8A8FF]">{props.eyebrow}</p>
        <h2 className="mt-1 text-3xl font-black tracking-normal">{props.title}</h2>
      </div>
    </div>
  );
}

function FeaturedPreview(props: { copy: CliMediaCopy; item: PromptItem; kind: MediaKind; locale: Locale }) {
  const title = props.item.title[props.locale] ?? props.item.title.en;
  const href = localizePath(cliMediaDetailPath(props.kind, props.item.slug), props.locale);
  return (
    <article className={`${cliMediaCardClass} overflow-hidden lg:ml-auto lg:w-full`}>
      <Link aria-label={title} href={href}>
        <ArtifactPreview artifact={props.item.artifact} title={title} variant="hero" />
      </Link>
      <div className="flex items-center justify-between gap-3 border-t-2 border-[#101014] px-4 py-3 dark:border-white/20">
        <div className="min-w-0">
          <p className="truncate text-sm font-black">{title}</p>
        </div>
        <span className="rounded-full border border-[#101014]/16 bg-[#EEE4FF] px-2 py-1 text-[11px] font-black text-[#7C3AED] dark:border-white/14 dark:bg-white/8 dark:text-[#C8A8FF]">{props.copy.artifact}</span>
      </div>
    </article>
  );
}

function PromptCard(props: { copy: CliMediaCopy; item: PromptItem; keyUrl: string; kind: MediaKind; locale: Locale }) {
  const title = props.item.title[props.locale] ?? props.item.title.en;
  const summary = props.item.summary[props.locale] ?? props.item.summary.en;
  const href = localizePath(cliMediaDetailPath(props.kind, props.item.slug), props.locale);

  return (
    <article className={`${cliMediaCardClass} mb-5 break-inside-avoid overflow-hidden transition-transform hover:-translate-y-0.5 ${props.kind === "video" ? "md:mb-6" : ""}`}>
      <Link aria-label={title} href={href}>
        <ArtifactPreview artifact={props.item.artifact} title={title} />
      </Link>
      <div className="p-5">
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1 rounded-full border border-[#101014]/16 bg-[#EEE4FF] px-2.5 py-1 text-[11px] font-black text-[#7C3AED] dark:border-white/14 dark:bg-white/8 dark:text-[#C8A8FF]"><Sparkles className="size-3" />{props.copy.artifact}</span>
          <span className="rounded-full border border-[#101014]/12 bg-white/62 px-2.5 py-1 text-[11px] font-black text-[#5C5861] dark:border-white/12 dark:bg-white/8 dark:text-white/62">{props.item.updatedAt}</span>
        </div>
        <h2 className="text-xl font-black tracking-normal">{title}</h2>
        <p className={`mt-2 text-sm leading-6 ${cliMediaMutedClass}`}>{summary}</p>
        <div className="mt-5 overflow-hidden rounded-[1rem] border-2 border-[#101014] bg-[#101014] dark:border-white/18">
          <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
            <span className="text-xs font-semibold text-white/60">{props.copy.prompt}</span>
          </div>
          <pre className="max-h-48 overflow-auto p-4 text-[12px] leading-6 whitespace-pre-wrap text-[#EEE4FF]"><code>{props.item.prompt}</code></pre>
        </div>
        <div className="mt-5 flex flex-wrap gap-2">
          {!isOwnedSource(props.item) ? (
            <a className="flatkey-cta-secondary inline-flex h-9 items-center gap-2 px-3 text-sm" href={props.item.source.url} target="_blank" rel="noopener noreferrer">
              <ExternalLink className="size-4" />
              {props.copy.viewSource}
            </a>
          ) : null}
          <a className="flatkey-primary-cta inline-flex h-9 items-center gap-2 px-3 text-sm" href={props.keyUrl}>
            <KeyRound className="size-4" />
            {props.copy.createKey}
          </a>
        </div>
      </div>
    </article>
  );
}

function CompactPromptCard(props: { item: PromptItem; kind: MediaKind; locale: Locale }) {
  const title = props.item.title[props.locale] ?? props.item.title.en;
  const href = localizePath(cliMediaDetailPath(props.kind, props.item.slug), props.locale);
  return (
    <article className={`${cliMediaCardClass} overflow-hidden transition-transform hover:-translate-y-0.5`}>
      <Link aria-label={title} href={href}>
        <ArtifactPreview artifact={props.item.artifact} title={title} variant="compact" />
      </Link>
      <div className="border-t-2 border-[#101014] p-3 dark:border-white/20">
        <h3 className="line-clamp-2 min-h-10 text-sm font-black leading-5">{title}</h3>
        <div className="mt-3 flex items-center justify-end gap-2">
          <span className="rounded-full border border-[#101014]/14 bg-white/62 px-2 py-0.5 text-[11px] font-black text-[#5C5861] dark:border-white/12 dark:bg-white/8 dark:text-white/62">{props.item.updatedAt}</span>
        </div>
      </div>
    </article>
  );
}

function Info(props: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[11px] font-bold tracking-[0.12em] text-[#62626D] uppercase">{props.label}</p>
      <p className="mt-1 text-sm font-semibold text-[#0B0B0F] dark:text-white">{props.value}</p>
    </div>
  );
}

function DetailMetric(props: { label: string; value: string }) {
  return (
    <div className="rounded-[1rem] border border-[#101014]/14 bg-white/62 p-3 dark:border-white/12 dark:bg-white/8">
      <p className="font-mono text-[11px] font-bold tracking-normal text-[#5C5861] uppercase dark:text-white/62">{props.label}</p>
      <p className="mt-1 truncate text-sm font-black text-[#0B0B0F] dark:text-white">{props.value}</p>
    </div>
  );
}

function visibleTags(item: PromptItem) {
  return item.tags.filter((tag) => !(isOwnedSource(item) && tag.toLowerCase() === "solvea"));
}

function isOwnedSource(item: PromptItem) {
  return item.source.platform === "Local migration" || item.source.label.toLowerCase().includes("owned");
}

function ownedSourceLabel(locale: Locale) {
  return locale === "zh" ? "自有素材" : "Owned asset";
}

function sourceDisplayLabel(item: PromptItem, locale: Locale) {
  return isOwnedSource(item) ? ownedSourceLabel(locale) : item.source.label;
}

function SourceInfo(props: { copy: CliMediaCopy; item: PromptItem; locale: Locale }) {
  const label = sourceDisplayLabel(props.item, props.locale);
  return (
    <div>
      <p className="font-mono text-[11px] font-bold tracking-normal text-[#5C5861] uppercase dark:text-white/62">{props.copy.source}</p>
      {isOwnedSource(props.item) ? (
        <p className="mt-1 text-sm font-semibold text-[#0B0B0F] dark:text-white">{label}</p>
      ) : (
        <a className="mt-1 inline-flex items-center gap-1 text-sm font-black text-[#7C3AED] hover:text-[#0B0B0F] dark:text-[#C8A8FF] dark:hover:text-white" href={props.item.source.url} target="_blank" rel="noopener noreferrer">
          {label}
          <ExternalLink className="size-3.5" />
        </a>
      )}
    </div>
  );
}

function SourcePanel(props: { copy: CliMediaCopy; item: PromptItem; locale: Locale }) {
  const label = sourceDisplayLabel(props.item, props.locale);
  return (
    <div className={`${cliMediaCardClass} p-5`}>
      <p className="font-mono text-xs font-black tracking-normal text-[#7C3AED] uppercase dark:text-[#C8A8FF]">{props.locale === "zh" ? "归属" : "Provenance"}</p>
      <p className="mt-3 text-sm font-semibold text-[#0B0B0F] dark:text-white">{label}</p>
      {isOwnedSource(props.item) ? (
        <p className={`mt-1 text-sm ${cliMediaMutedClass}`}>{props.locale === "zh" ? "Flatkey 自有迁移产物" : "Flatkey owned migrated artifact"}</p>
      ) : (
        <>
          <p className={`mt-1 text-sm ${cliMediaMutedClass}`}>{props.item.source.platform}</p>
          <a className="flatkey-cta-secondary mt-4 inline-flex h-10 items-center gap-2 px-3 text-sm" href={props.item.source.url} target="_blank" rel="noopener noreferrer">
            <ExternalLink className="size-4" />
            {props.copy.viewSource}
          </a>
        </>
      )}
    </div>
  );
}

function ArtifactPreview(props: { artifact: PromptArtifact; title: string; variant?: "compact" | "detail" | "hero" | "tile" }) {
  const aspect = props.variant === "compact" ? "aspect-[4/3]" : props.variant === "hero" || props.variant === "detail" ? "aspect-[16/9]" : "aspect-[16/10]";
  const mediaFillClass = "mx-auto h-full w-auto max-w-none";
  if (props.artifact.kind === "video") {
    return (
      <div className={`relative flex ${aspect} items-center justify-center overflow-hidden bg-[#EEE4FF] dark:bg-[#111116]`}>
        {isVideoFile(props.artifact.url) ? (
          <video
            aria-label={props.artifact.alt}
            autoPlay
            className={mediaFillClass}
            loop
            muted
            playsInline
            poster={props.artifact.poster}
            preload={props.variant === "hero" ? "auto" : "metadata"}
          >
            <source src={props.artifact.url} type={videoMimeType(props.artifact.url)} />
          </video>
        ) : (
          <Image src={props.artifact.poster} alt={props.artifact.alt} width={1600} height={900} sizes="(min-width: 1024px) 50vw, 100vw" className={mediaFillClass} />
        )}
        <div className="pointer-events-none absolute inset-0 ring-1 ring-inset ring-white/10" />
        {!isVideoFile(props.artifact.url) ? (
          <div className="pointer-events-none absolute right-3 bottom-3 flex h-9 w-9 items-center justify-center rounded-full border-2 border-[#101014] bg-white text-[#101014] shadow-[3px_3px_0_#101014]">
            <Play className="ml-0.5 size-4 fill-current" />
          </div>
        ) : null}
      </div>
    );
  }

  if (props.artifact.kind === "image") {
    return (
      <div className={`relative flex ${aspect} items-center justify-center overflow-hidden bg-[#EEE4FF] dark:bg-[#111116]`}>
        <Image src={props.artifact.url} alt={props.artifact.alt} width={1600} height={1200} sizes="(min-width: 1024px) 50vw, 100vw" className={mediaFillClass} />
      </div>
    );
  }

  if (props.artifact.kind === "storyboard") {
    return (
      <div className={`grid ${aspect} grid-cols-3 gap-1 bg-[#161020] p-2`}>
        {props.artifact.frames.map((frame, index) => (
          <div key={frame} className="rounded-md border border-white/10 bg-white/8 p-2 text-[11px] leading-4 text-[#EEE4FF]">
            <span className="mb-1 block font-semibold text-[#C8A8FF]">{index + 1}</span>
            {frame}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className={`${aspect} bg-[#101014] p-5 text-[#EEE4FF]`}>
      <p className="text-sm font-semibold">{props.title}</p>
    </div>
  );
}

function isVideoFile(url: string) {
  return /\.(mp4|webm|mov)(\?|$)/i.test(url);
}

function videoMimeType(url: string) {
  if (/\.webm(\?|$)/i.test(url)) return "video/webm";
  if (/\.mov(\?|$)/i.test(url)) return "video/quicktime";
  return "video/mp4";
}
