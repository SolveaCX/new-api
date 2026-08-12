import { type Locale, withIdFallback } from "./locales";

export type PromptArtifact =
  | {
      kind: "image";
      alt: string;
      url: string;
    }
  | {
      kind: "video";
      alt: string;
      poster: string;
      url: string;
    }
  | {
      kind: "text";
      title: string;
      body: string;
    }
  | {
      kind: "code";
      language: string;
      code: string;
    }
  | {
      kind: "storyboard";
      frames: string[];
    };

export type PromptSource = {
  label: string;
  platform: "GitHub" | "Hugging Face" | "Social" | "Official docs" | "Flatkey generated" | "Local migration" | "External";
  url: string;
  capturedAt: string;
};

export type PromptItem = {
  artifact: PromptArtifact;
  category: "image" | "video" | "audio" | "text" | "agent";
  model: string;
  output: {
    label: Record<Locale, string>;
    ratio: "1:1" | "3:2" | "4:3" | "9:16" | "16:9" | "3x3";
  };
  prompt: string;
  slug: string;
  source: PromptSource;
  summary: Record<Locale, string>;
  tags: string[];
  title: Record<Locale, string>;
  updatedAt: string;
};

export type PromptLibraryCopy = {
  artifactLabel: string;
  categories: Record<PromptItem["category"], string>;
  copyPrompt: string;
  detailBack: string;
  detailCta: string;
  detailSource: string;
  detailUseWith: string;
  empty: string;
  heroBadge: string;
  heroBody: string;
  heroTitle: string;
  metaDescription: string;
  metaTitle: string;
  promptLabel: string;
  sourceLabel: string;
};

const today = new Date().toISOString().slice(0, 10);
const flatkeyGithubBase = "https://github.com/flatkey-ai";
const solveaSourceUrl = "https://mkt-video-proxy-528088078482.us-central1.run.app/characters";
const diffusionDbSourceUrl = "https://huggingface.co/datasets/anothy1/image-gen-instruct-360K";

function promptText(en: string, zh: string): Record<Locale, string> {
  return withIdFallback({
    en,
    zh,
    es: en,
    fr: en,
    pt: en,
    ru: en,
    ja: en,
    vi: en,
    de: en,
  });
}

function outputLabel(en: string, zh: string, ratio: PromptItem["output"]["ratio"]): PromptItem["output"] {
  return {
    label: promptText(en, zh),
    ratio,
  };
}

const categoriesEn: PromptLibraryCopy["categories"] = {
  image: "Image",
  video: "Video",
  audio: "Audio",
  text: "Text",
  agent: "Agent",
};

function solveaOwnedSource(): PromptSource {
  return {
    capturedAt: today,
    label: "Owned production asset",
    platform: "Local migration",
    url: solveaSourceUrl,
  };
}

function solveaImagePromptItem(props: {
  alt: string;
  outputEn: string;
  outputRatio: PromptItem["output"]["ratio"];
  outputZh: string;
  prompt: string;
  slug: string;
  summaryEn: string;
  summaryZh: string;
  tags: string[];
  titleEn: string;
  titleZh: string;
  url: string;
}): PromptItem {
  return {
    artifact: {
      alt: props.alt,
      kind: "image",
      url: props.url,
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel(props.outputEn, props.outputZh, props.outputRatio),
    prompt: props.prompt,
    slug: props.slug,
    source: solveaOwnedSource(),
    summary: promptText(props.summaryEn, props.summaryZh),
    tags: ["solvea", ...props.tags, "image"],
    title: promptText(props.titleEn, props.titleZh),
    updatedAt: today,
  };
}

function awesomeImagesSource(path = "README.md"): PromptSource {
  return {
    capturedAt: today,
    label: "flatkey-ai/awesome-images",
    platform: "GitHub",
    url: `${flatkeyGithubBase}/awesome-images/blob/main/${path}`,
  };
}

function githubImagePromptItem(props: {
  alt: string;
  outputEn: string;
  outputRatio: PromptItem["output"]["ratio"];
  outputZh: string;
  prompt: string;
  slug: string;
  summaryEn: string;
  summaryZh: string;
  tags: string[];
  titleEn: string;
  titleZh: string;
  url: string;
}): PromptItem {
  return {
    artifact: {
      alt: props.alt,
      kind: "image",
      url: props.url,
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel(props.outputEn, props.outputZh, props.outputRatio),
    prompt: props.prompt,
    slug: props.slug,
    source: awesomeImagesSource("src/prompts.js"),
    summary: promptText(props.summaryEn, props.summaryZh),
    tags: ["github", "awesome-images", ...props.tags, "image"],
    title: promptText(props.titleEn, props.titleZh),
    updatedAt: today,
  };
}

function diffusionDbSource(): PromptSource {
  return {
    capturedAt: today,
    label: "DiffusionDB image-gen-instruct-360K",
    platform: "Hugging Face",
    url: diffusionDbSourceUrl,
  };
}

function diffusionDbImagePromptItem(props: {
  alt: string;
  imageId: number;
  imageNsfw: number;
  outputEn: string;
  outputRatio: PromptItem["output"]["ratio"];
  outputZh: string;
  prompt: string;
  promptNsfw: number;
  row: number;
  slug: string;
  summaryEn: string;
  summaryZh: string;
  tags: string[];
  titleEn: string;
  titleZh: string;
  url: string;
}): PromptItem {
  const nsfwTag = `nsfw<=${Math.max(props.imageNsfw, props.promptNsfw).toFixed(2)}`;

  return {
    artifact: {
      alt: props.alt,
      kind: "image",
      url: props.url,
    },
    category: "image",
    model: "stable-diffusion",
    output: outputLabel(props.outputEn, props.outputZh, props.outputRatio),
    prompt: props.prompt,
    slug: props.slug,
    source: diffusionDbSource(),
    summary: promptText(
      `${props.summaryEn} Source row ${props.row}, image id ${props.imageId}; image and prompt NSFW scores are below 0.15.`,
      `${props.summaryZh} 来源 row ${props.row}，image id ${props.imageId}；图片与提示词 NSFW 分数均低于 0.15。`,
    ),
    tags: ["free", "diffusiondb", "hugging-face", nsfwTag, ...props.tags, "image"],
    title: promptText(props.titleEn, props.titleZh),
    updatedAt: today,
  };
}

function sourceSignal(props: {
  label: string;
  platform: PromptSource["platform"];
  url: string;
}): PromptSource {
  return {
    capturedAt: today,
    label: props.label,
    platform: props.platform,
    url: props.url,
  };
}

function textPromptItem(props: {
  artifactBody: string;
  artifactTitle: string;
  category?: PromptItem["category"];
  model: string;
  outputEn: string;
  outputZh: string;
  prompt: string;
  slug: string;
  source: PromptSource;
  summaryEn: string;
  summaryZh: string;
  tags: string[];
  titleEn: string;
  titleZh: string;
}): PromptItem {
  const category = props.category ?? "text";
  return {
    artifact: {
      body: props.artifactBody,
      kind: "text",
      title: props.artifactTitle,
    },
    category,
    model: props.model,
    output: outputLabel(props.outputEn, props.outputZh, "1:1"),
    prompt: props.prompt,
    slug: props.slug,
    source: props.source,
    summary: promptText(props.summaryEn, props.summaryZh),
    tags: [...props.tags, category],
    title: promptText(props.titleEn, props.titleZh),
    updatedAt: today,
  };
}

function codePromptItem(props: {
  code: string;
  language: string;
  model: string;
  outputEn: string;
  outputZh: string;
  prompt: string;
  slug: string;
  source: PromptSource;
  summaryEn: string;
  summaryZh: string;
  tags: string[];
  titleEn: string;
  titleZh: string;
}): PromptItem {
  return {
    artifact: {
      code: props.code,
      kind: "code",
      language: props.language,
    },
    category: "agent",
    model: props.model,
    output: outputLabel(props.outputEn, props.outputZh, "1:1"),
    prompt: props.prompt,
    slug: props.slug,
    source: props.source,
    summary: promptText(props.summaryEn, props.summaryZh),
    tags: [...props.tags, "agent"],
    title: promptText(props.titleEn, props.titleZh),
    updatedAt: today,
  };
}

function storyboardPromptItem(props: {
  frames: string[];
  model: string;
  outputEn: string;
  outputRatio: PromptItem["output"]["ratio"];
  outputZh: string;
  prompt: string;
  slug: string;
  source: PromptSource;
  summaryEn: string;
  summaryZh: string;
  tags: string[];
  titleEn: string;
  titleZh: string;
}): PromptItem {
  return {
    artifact: {
      frames: props.frames,
      kind: "storyboard",
    },
    category: "video",
    model: props.model,
    output: outputLabel(props.outputEn, props.outputZh, props.outputRatio),
    prompt: props.prompt,
    slug: props.slug,
    source: props.source,
    summary: promptText(props.summaryEn, props.summaryZh),
    tags: [...props.tags, "video", "storyboard"],
    title: promptText(props.titleEn, props.titleZh),
    updatedAt: today,
  };
}

export const promptLibraryCopy: Record<Locale, PromptLibraryCopy> = withIdFallback({
  en: {
    artifactLabel: "Produced artifact",
    categories: categoriesEn,
    copyPrompt: "Copy prompt",
    detailBack: "Back to prompts",
    detailCta: "Run with Flatkey",
    detailSource: "Source and provenance",
    detailUseWith: "Use with",
    empty: "No prompts matched this source.",
    heroBadge: "Prompt library",
    heroBody:
      "Prompts are stored in Flatkey's prompt database alongside the artifact they produced. The playground surfaces strong image, video, text, and agent examples with local assets and direct actions.",
    heroTitle: "Prompts that already shipped an output.",
    metaDescription:
      "A daily refreshed prompt library for Flatkey users. Browse image, video, audio, text and agent prompts with source provenance and produced artifacts.",
    metaTitle: "Flatkey Prompts — daily sourced AI prompt library",
    promptLabel: "Prompt",
    sourceLabel: "Source",
  },
  zh: {
    artifactLabel: "对应产物",
    categories: { image: "图像", video: "视频", audio: "音频", text: "文本", agent: "Agent" },
    copyPrompt: "复制提示词",
    detailBack: "返回提示词库",
    detailCta: "用 Flatkey 运行",
    detailSource: "来源与出处",
    detailUseWith: "适用模型",
    empty: "没有匹配这个来源的提示词。",
    heroBadge: "提示词库",
    heroBody:
      "提示词和对应产物一起存入 Flatkey 数据库。这里优先展示图像、视频、文本和 Agent 的高质量示例，并保持本地图片可用。",
    heroTitle: "不只给提示词，也给已经产出的结果。",
    metaDescription:
      "Flatkey 用户可用的每日更新提示词库，覆盖图像、视频、音频、文本和 Agent 提示词，每条都有来源和产物。",
    metaTitle: "Flatkey Prompts — 每日更新 AI 提示词库",
    promptLabel: "提示词",
    sourceLabel: "来源",
  },
  es: {
    artifactLabel: "Resultado producido",
    categories: { image: "Imagen", video: "Video", audio: "Audio", text: "Texto", agent: "Agente" },
    copyPrompt: "Copiar prompt",
    detailBack: "Volver a prompts",
    detailCta: "Ejecutar con Flatkey",
    detailSource: "Fuente y procedencia",
    detailUseWith: "Usar con",
    empty: "No hay prompts para esta fuente.",
    heroBadge: "Biblioteca diaria de prompts",
    heroBody:
      "Recolectamos primero proyectos de Flatkey en GitHub y después fuentes públicas de prompt engineering. Cada entrada incluye fuente y resultado.",
    heroTitle: "Prompts que ya tienen un resultado.",
    metaDescription:
      "Biblioteca diaria de prompts para usuarios de Flatkey, con fuentes y resultados para imagen, video, audio, texto y agentes.",
    metaTitle: "Flatkey Prompts — biblioteca diaria de prompts IA",
    promptLabel: "Prompt",
    sourceLabel: "Fuente",
  },
  fr: {
    artifactLabel: "Résultat produit",
    categories: { image: "Image", video: "Vidéo", audio: "Audio", text: "Texte", agent: "Agent" },
    copyPrompt: "Copier le prompt",
    detailBack: "Retour aux prompts",
    detailCta: "Lancer avec Flatkey",
    detailSource: "Source et provenance",
    detailUseWith: "Utiliser avec",
    empty: "Aucun prompt pour cette source.",
    heroBadge: "Bibliothèque de prompts quotidienne",
    heroBody:
      "Nous collectons d'abord les projets Flatkey sur GitHub, puis des sources publiques de prompt engineering. Chaque entrée affiche sa source et son résultat.",
    heroTitle: "Des prompts avec leur résultat produit.",
    metaDescription:
      "Bibliothèque de prompts actualisée chaque jour pour Flatkey, avec provenance et résultats pour image, vidéo, audio, texte et agents.",
    metaTitle: "Flatkey Prompts — bibliothèque quotidienne de prompts IA",
    promptLabel: "Prompt",
    sourceLabel: "Source",
  },
  pt: {
    artifactLabel: "Resultado produzido",
    categories: { image: "Imagem", video: "Vídeo", audio: "Áudio", text: "Texto", agent: "Agente" },
    copyPrompt: "Copiar prompt",
    detailBack: "Voltar aos prompts",
    detailCta: "Executar com Flatkey",
    detailSource: "Fonte e procedência",
    detailUseWith: "Usar com",
    empty: "Nenhum prompt corresponde a esta fonte.",
    heroBadge: "Biblioteca diária de prompts",
    heroBody:
      "Coletamos primeiro projetos Flatkey no GitHub e depois fontes públicas de engenharia de prompts. Cada item inclui fonte e resultado.",
    heroTitle: "Prompts que já têm resultado.",
    metaDescription:
      "Biblioteca diária de prompts para usuários Flatkey, com fontes e resultados para imagem, vídeo, áudio, texto e agentes.",
    metaTitle: "Flatkey Prompts — biblioteca diária de prompts de IA",
    promptLabel: "Prompt",
    sourceLabel: "Fonte",
  },
  ru: {
    artifactLabel: "Готовый результат",
    categories: { image: "Изображение", video: "Видео", audio: "Аудио", text: "Текст", agent: "Агент" },
    copyPrompt: "Скопировать prompt",
    detailBack: "Назад к prompts",
    detailCta: "Запустить через Flatkey",
    detailSource: "Источник",
    detailUseWith: "Использовать с",
    empty: "Для этого источника prompt не найден.",
    heroBadge: "Ежедневная библиотека prompts",
    heroBody:
      "Сначала собираем материалы из проектов Flatkey на GitHub, затем из публичных источников prompt engineering. У каждой записи есть источник и результат.",
    heroTitle: "Prompts с уже готовым результатом.",
    metaDescription:
      "Ежедневно обновляемая библиотека prompts для Flatkey: изображения, видео, аудио, текст и агенты с источниками и результатами.",
    metaTitle: "Flatkey Prompts — ежедневная библиотека AI prompts",
    promptLabel: "Prompt",
    sourceLabel: "Источник",
  },
  ja: {
    artifactLabel: "生成済み成果物",
    categories: { image: "画像", video: "動画", audio: "音声", text: "テキスト", agent: "Agent" },
    copyPrompt: "プロンプトをコピー",
    detailBack: "プロンプト一覧へ戻る",
    detailCta: "Flatkey で実行",
    detailSource: "出典",
    detailUseWith: "利用モデル",
    empty: "このソースに一致するプロンプトはありません。",
    heroBadge: "毎日更新のプロンプト集",
    heroBody:
      "Flatkey の GitHub プロジェクトを優先して収集し、公開プロンプトエンジニアリング情報も追加します。各項目に出典と成果物を表示します。",
    heroTitle: "成果物まで確認できるプロンプト。",
    metaDescription:
      "Flatkey ユーザー向けの毎日更新プロンプト集。画像、動画、音声、テキスト、Agent の出典と成果物を掲載。",
    metaTitle: "Flatkey Prompts — 毎日更新 AI プロンプト集",
    promptLabel: "プロンプト",
    sourceLabel: "出典",
  },
  vi: {
    artifactLabel: "Sản phẩm đầu ra",
    categories: { image: "Hình ảnh", video: "Video", audio: "Âm thanh", text: "Văn bản", agent: "Agent" },
    copyPrompt: "Sao chép prompt",
    detailBack: "Quay lại prompts",
    detailCta: "Chạy với Flatkey",
    detailSource: "Nguồn",
    detailUseWith: "Dùng với",
    empty: "Không có prompt nào khớp nguồn này.",
    heroBadge: "Thư viện prompt cập nhật hằng ngày",
    heroBody:
      "Chúng tôi lấy trước từ các dự án Flatkey trên GitHub, rồi bổ sung nguồn prompt engineering công khai. Mỗi mục có nguồn và sản phẩm đầu ra.",
    heroTitle: "Prompt có sẵn kết quả đầu ra.",
    metaDescription:
      "Thư viện prompt cập nhật hằng ngày cho Flatkey, có nguồn và kết quả cho hình ảnh, video, âm thanh, văn bản và agent.",
    metaTitle: "Flatkey Prompts — thư viện prompt AI hằng ngày",
    promptLabel: "Prompt",
    sourceLabel: "Nguồn",
  },
  de: {
    artifactLabel: "Erzeugtes Ergebnis",
    categories: { image: "Bild", video: "Video", audio: "Audio", text: "Text", agent: "Agent" },
    copyPrompt: "Prompt kopieren",
    detailBack: "Zurück zu Prompts",
    detailCta: "Mit Flatkey ausführen",
    detailSource: "Quelle",
    detailUseWith: "Verwenden mit",
    empty: "Keine Prompts für diese Quelle.",
    heroBadge: "Täglich aktualisierte Prompt-Bibliothek",
    heroBody:
      "Wir sammeln zuerst aus Flatkey-GitHub-Projekten und ergänzen öffentliche Prompt-Engineering-Quellen. Jeder Eintrag zeigt Quelle und Ergebnis.",
    heroTitle: "Prompts mit fertigem Ergebnis.",
    metaDescription:
      "Täglich aktualisierte Prompt-Bibliothek für Flatkey mit Quellen und Ergebnissen für Bild, Video, Audio, Text und Agenten.",
    metaTitle: "Flatkey Prompts — tägliche KI-Prompt-Bibliothek",
    promptLabel: "Prompt",
    sourceLabel: "Quelle",
  },
});

export const staticPromptItems: PromptItem[] = [
  diffusionDbImagePromptItem({
    alt: "Silhouette photographer with colored studio bokeh",
    imageId: 45,
    imageNsfw: 0.0983564555644989,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Create a silhouette illustration of a photographer taking a picture with a close-up telephoto lens. Use blue and red studio lighting, soft bokeh, a subtle vignette, and bluish darks.",
    promptNsfw: 0.0009590951376594603,
    row: 23,
    slug: "diffusiondb-silhouette-photographer",
    summaryEn: "A public DiffusionDB derivative prompt-output pair localized into the Flatkey prompt library.",
    summaryZh: "从公开 DiffusionDB 派生数据集中筛出的提示词-图片产物配对，并本地化到 Flatkey 提示词库。",
    tags: ["photography", "lighting", "studio"],
    titleEn: "Silhouette photographer lighting study",
    titleZh: "摄影剪影灯光练习",
    url: "/assets/prompts/diffusiondb/silhouette-photographer.jpg",
  }),
  diffusionDbImagePromptItem({
    alt: "Luminescent fantasy tree with glowing canopy",
    imageId: 46,
    imageNsfw: 0.09993492066860199,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Create an ethereal fantasy tree called the Tree of Luminescence. Make the trunk intricate, the canopy glowing, and the surrounding grove misty, detailed, and luminous.",
    promptNsfw: 0.0002848550211638212,
    row: 24,
    slug: "diffusiondb-luminescent-fantasy-tree",
    summaryEn: "A CC0 DiffusionDB image pair with a normalized visual prompt and a locally hosted output image.",
    summaryZh: "CC0 DiffusionDB 图片配对，已整理提示词并将产物图片转为本地托管。",
    tags: ["fantasy", "environment", "glow"],
    titleEn: "Luminescent fantasy tree",
    titleZh: "发光幻想树",
    url: "/assets/prompts/diffusiondb/luminescent-fantasy-tree.jpg",
  }),
  diffusionDbImagePromptItem({
    alt: "Wooden chain swing in a quiet forest clearing",
    imageId: 146,
    imageNsfw: 0.05225906893610954,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Illustrate a rugged wooden swing supported by two chain sets in a quiet forest clearing. Keep the object centered, with natural daylight, realistic wood grain, and a simple background.",
    promptNsfw: 0.01392119936645031,
    row: 100,
    slug: "diffusiondb-wooden-chain-swing",
    summaryEn: "A simple object-and-environment prompt from the filtered image-gen-instruct subset.",
    summaryZh: "来自过滤后 image-gen-instruct 子集的物体与环境类提示词。",
    tags: ["object", "outdoor", "reference"],
    titleEn: "Wooden chain swing",
    titleZh: "木质链条秋千",
    url: "/assets/prompts/diffusiondb/wooden-chain-swing.jpg",
  }),
  diffusionDbImagePromptItem({
    alt: "Black red and green aurora over a mountain lake",
    imageId: 728,
    imageNsfw: 0.08734456449747086,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Create a dramatic aurora borealis landscape using a black, white, red, and green color scheme. Add mountain silhouettes, water reflection, and a clean square composition.",
    promptNsfw: 0.00725916214287281,
    row: 500,
    slug: "diffusiondb-black-red-aurora",
    summaryEn: "A public landscape prompt-output pair selected for clear composition and safe NSFW scores.",
    summaryZh: "筛选自公开风景类提示词-产物配对，构图清晰且 NSFW 分数安全。",
    tags: ["landscape", "color", "nature"],
    titleEn: "Black-red aurora landscape",
    titleZh: "黑红极光风景",
    url: "/assets/prompts/diffusiondb/black-red-aurora.jpg",
  }),
  diffusionDbImagePromptItem({
    alt: "Domed building in Vienna under clear blue sky",
    imageId: 732,
    imageNsfw: 0.09363918751478195,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Create a daylight architectural photograph of a domed building in Vienna. Use a clean blue sky, crisp facade detail, and a centered travel-photography composition.",
    promptNsfw: 0.0019991325680166483,
    row: 503,
    slug: "diffusiondb-vienna-domed-building",
    summaryEn: "A public architectural image pair normalized into a reusable travel and location prompt.",
    summaryZh: "公开建筑图片配对，已整理成可复用的旅行与地点类提示词。",
    tags: ["architecture", "travel", "photo"],
    titleEn: "Vienna domed building photo",
    titleZh: "维也纳穹顶建筑照片",
    url: "/assets/prompts/diffusiondb/vienna-domed-building.jpg",
  }),
  diffusionDbImagePromptItem({
    alt: "Tall fantasy tower in a lush valley with a river",
    imageId: 333332,
    imageNsfw: 0.06427015364170074,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Create a photorealistic fantasy tower in a lush valley with a river flowing through it. Use misty mountains, dense greenery, and a high vantage point.",
    promptNsfw: 0.00040953917778097093,
    row: 20000,
    slug: "diffusiondb-fantasy-tower-valley",
    summaryEn: "A filtered DiffusionDB environment prompt paired with a locally hosted generated image.",
    summaryZh: "过滤后的 DiffusionDB 环境类提示词，并配有本地托管生成图。",
    tags: ["environment", "fantasy", "landscape"],
    titleEn: "Fantasy tower valley",
    titleZh: "幻想高塔山谷",
    url: "/assets/prompts/diffusiondb/wizard-tower-valley.jpg",
  }),
  diffusionDbImagePromptItem({
    alt: "Orchard street art scene with small house and blooming trees",
    imageId: 174228,
    imageNsfw: 0.05742084980010986,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Create a street-art inspired farm scene with a small house, a tidy orchard, blooming trees, and soft surreal lighting. Keep the composition calm and graphic.",
    promptNsfw: 0.0024766039568930864,
    row: 40004,
    slug: "diffusiondb-orchard-street-art",
    summaryEn: "A public scene prompt-output pair rewritten to remove named-style dependencies while keeping the visual intent.",
    summaryZh: "公开场景提示词-产物配对，已去掉命名风格依赖并保留视觉意图。",
    tags: ["street-art", "scene", "farm"],
    titleEn: "Orchard street-art scene",
    titleZh: "果园街头艺术场景",
    url: "/assets/prompts/diffusiondb/orchard-street-art.jpg",
  }),
  diffusionDbImagePromptItem({
    alt: "Private archive room in a vinyl record store",
    imageId: 47198,
    imageNsfw: 0.032407861202955246,
    outputEn: "1:1 generated image",
    outputRatio: "1:1",
    outputZh: "1:1 生成图",
    prompt:
      "Create an illustration of a private archive room inside a vinyl record store, packed with shelves of valuable records. Make it nostalgic, textured, and warmly lit.",
    promptNsfw: 0.0018022130243480206,
    row: 240007,
    slug: "diffusiondb-vinyl-record-room",
    summaryEn: "A safe public image pair selected for interior-detail prompting and visible artifact quality.",
    summaryZh: "安全公开图片配对，适合作为室内细节提示词样例，且产物质量可见。",
    tags: ["interior", "archive", "retail"],
    titleEn: "Vinyl record archive room",
    titleZh: "黑胶唱片档案室",
    url: "/assets/prompts/diffusiondb/vinyl-record-room.jpg",
  }),
  {
    artifact: {
      alt: "D-17 delivery robot identity sheet",
      kind: "image",
      url: "/assets/prompts/solvea/d17_delivery_robot.png",
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel("4:3 character sheet", "4:3 角色设定图", "4:3"),
    prompt:
      "Original warm-core wasteland 3D animated micro-film character concept art. Create the main protagonist: D-17, an old delivery robot with a compact rounded box-shaped body, chest delivery compartment, two small articulated arms, large expressive LED eyes, short wheel-feet hybrid base, one antenna, scuffed cream and faded orange paint. Full-body front three-quarter character concept on a neutral dusty background, no weapons, no readable text.",
    slug: "solvea-d17-delivery-robot",
    source: {
      capturedAt: today,
      label: "Owned production asset",
      platform: "Local migration",
      url: solveaSourceUrl,
    },
    summary: promptText(
      "Migrated from owned production assets with the character prompt and generated image already paired.",
      "从自有生产素材迁移，角色提示词和生成图片已经明确对应。",
    ),
    tags: ["solvea", "character", "image"],
    title: promptText("D-17 delivery robot identity sheet", "D-17 / 小递角色身份图"),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Package 0001 Hope Heart prop",
      kind: "image",
      url: "/assets/prompts/solvea/package_0001_hope_heart.png",
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel("4:3 prop concept", "4:3 道具概念图", "4:3"),
    prompt:
      "Create the core prop: Package 0001, a small metal delivery parcel that becomes the Hope Heart. Rectangular case with rounded corners, worn white-gray shell, faded orange sealing band, small electronic lock with warm golden glow, scratches, dust, taped edges, blank label area. Three-quarter prop concept plus a small open-compartment inset showing warm golden light, no typography.",
    slug: "solvea-package-0001-hope-heart",
    source: {
      capturedAt: today,
      label: "Owned production asset",
      platform: "Local migration",
      url: solveaSourceUrl,
    },
    summary: promptText(
      "A verified owned prop prompt paired with the generated Package 0001 image.",
      "已验证的自有道具提示词，并配有 Package 0001 生成图。",
    ),
    tags: ["solvea", "prop", "image"],
    title: promptText("Package 0001 Hope Heart prop", "包裹0001 / 希望之心道具图"),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Mia child owner character",
      kind: "image",
      url: "/assets/prompts/solvea/mia_child_owner.png",
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel("4:3 character sheet", "4:3 角色设定图", "4:3"),
    prompt:
      "Create Mia, the first owner, a young foreign girl around 8 years old with a warm expressive face, freckles, and short brown curly hair. She wears a yellow raincoat, small backpack, red star sticker in her hand, worn sneakers, hopeful but scared expression. Full-body front three-quarter character concept, simple ruined apartment hallway hint, warm-core wasteland 3D animated micro-film style, no readable text.",
    slug: "solvea-mia-child-owner",
    source: {
      capturedAt: today,
      label: "Owned production asset",
      platform: "Local migration",
      url: solveaSourceUrl,
    },
    summary: promptText(
      "An owned character prompt with its finished Mia concept image attached.",
      "自有角色提示词，已附上 Mia 的完成概念图。",
    ),
    tags: ["solvea", "character", "image"],
    title: promptText("Mia child owner character", "Mia / 米娅儿童角色图"),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Episode 01 delivery center scene bible",
      kind: "image",
      url: "/assets/prompts/solvea/scene_bible_ep01_delivery_center_3x3.png",
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel("3x3 scene board", "3x3 场景板", "3x3"),
    prompt:
      "Create a 3x3 scene bible sheet for the abandoned delivery center in Episode 01. Panels: blacked-out sorting hall, stopped conveyor belts, dormant delivery robots, package storage racks, task chute with warm lock light, narrow maintenance tunnel, dented rolling shutter with cold green leak, half-stuck fire side door, exterior threshold with dust light. Stylized 3D animation environment, no readable signs.",
    slug: "solvea-ep01-delivery-center-scene-bible",
    source: {
      capturedAt: today,
      label: "Owned production asset",
      platform: "Local migration",
      url: solveaSourceUrl,
    },
    summary: promptText(
      "A production scene-bible prompt paired with the generated 3x3 delivery center board.",
      "生产用场景圣经提示词，配有配送中心 3x3 生成板。",
    ),
    tags: ["solvea", "scene-bible", "image"],
    title: promptText("Episode 01 delivery center scene bible", "EP01 配送中心场景圣经"),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Episode 01 delivery center motion storyboard",
      kind: "image",
      url: "/assets/prompts/solvea/storyboard_ep01_delivery_center_motion_3x3.png",
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel("3x3 motion storyboard", "3x3 镜头分镜板", "3x3"),
    prompt:
      "Create a 3x3 action storyboard bible sheet for Episode 01, matching D-17, Package 0001, and the delivery center style. Panel order: warehouse power dies; D-17 LED eyes wake; package pops halfway from chute; D-17 catches package; chest compartment locks; conveyor belts restart; rolling shutter dents from outside; D-17 squeezes through fire door; ruined city exterior reveal. Use one D-17 only, no text labels.",
    slug: "solvea-ep01-delivery-center-storyboard",
    source: {
      capturedAt: today,
      label: "Owned production asset",
      platform: "Local migration",
      url: solveaSourceUrl,
    },
    summary: promptText(
      "An owned storyboard prompt paired with the completed Episode 01 motion board.",
      "自有分镜提示词，已附 EP01 镜头走势成图。",
    ),
    tags: ["solvea", "storyboard", "image"],
    title: promptText("Episode 01 delivery center motion storyboard", "EP01 配送中心镜头走势图"),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Commercial image prompt result from the Flatkey Image Buddy gallery",
      kind: "image",
      url: "/assets/cli/product-reveal.png",
    },
    category: "image",
    model: "gpt-image-2",
    output: outputLabel("16:9 hero visual", "16:9 首图视觉", "16:9"),
    prompt:
      "Create a premium product hero image for {{product_name}}. Show the product as the first-viewport signal, clean SaaS lighting, high-detail commercial realism, one clear benefit visible in the scene, no crowded typography.",
    slug: "premium-product-hero-image",
    source: {
      capturedAt: today,
      label: "flatkey-ai/awesome-images",
      platform: "GitHub",
      url: `${flatkeyGithubBase}/awesome-images`,
    },
    summary: withIdFallback({
      en: "A commercial product hero prompt adapted from Flatkey Image Buddy templates, with a real gallery output attached.",
      zh: "来自 Flatkey Image Buddy 模板的商业产品首图提示词，并附带真实图库产物。",
      es: "Prompt comercial para hero de producto adaptado de Image Buddy, con resultado de galería.",
      fr: "Prompt de visuel produit issu d'Image Buddy, avec résultat de galerie.",
      pt: "Prompt comercial de hero de produto adaptado do Image Buddy, com resultado de galeria.",
      ru: "Коммерческий prompt для hero-изображения продукта из Image Buddy, с готовым результатом.",
      ja: "Image Buddy テンプレート由来の商品ヒーロー画像プロンプト。ギャラリー成果物付き。",
      vi: "Prompt ảnh hero sản phẩm từ Image Buddy, có sản phẩm gallery đi kèm.",
      de: "Kommerzieller Produkt-Hero-Prompt aus Image Buddy mit Galerie-Ergebnis.",
    }),
    tags: ["commerce", "image", "hero"],
    title: withIdFallback({
      en: "Premium product hero visual",
      zh: "高级产品首图",
      es: "Hero visual de producto premium",
      fr: "Visuel produit premium",
      pt: "Hero visual premium de produto",
      ru: "Премиальный product hero",
      ja: "プレミアム商品ヒーロー画像",
      vi: "Ảnh hero sản phẩm cao cấp",
      de: "Premium-Produkt-Hero",
    }),
    updatedAt: today,
  },
  solveaImagePromptItem({
    alt: "D-17 four-view model sheet",
    outputEn: "4-view model sheet",
    outputRatio: "4:3",
    outputZh: "四视图角色设定",
    prompt:
      "Create a clean four-view model sheet for D-17, the old delivery robot protagonist. Show front, side, rear, and three-quarter views with consistent rounded box body, chest delivery compartment, short wheel-feet base, one antenna, cream and faded orange worn paint, expressive LED eyes, warm-core wasteland 3D animated style, neutral background, no labels or readable text.",
    slug: "solvea-d17-four-view-model-sheet",
    summaryEn: "Owned D-17 consistency prompt paired with a finished four-view model sheet.",
    summaryZh: "自有 D-17 一致性提示词，配有完成的四视图角色设定图。",
    tags: ["character", "model-sheet"],
    titleEn: "D-17 four-view model sheet",
    titleZh: "D-17 四视图角色设定",
    url: "/assets/prompts/solvea/d17_model_sheet_4view.png",
  }),
  solveaImagePromptItem({
    alt: "Lev wasteland driver character sheet",
    outputEn: "4:3 character sheet",
    outputRatio: "4:3",
    outputZh: "4:3 角色设定图",
    prompt:
      "Create Lev, a lean wasteland driver in his late 30s with sunburned skin, tired eyes, patched mechanic jacket, faded scarf, fingerless gloves, utility belt, dusty boots, and a guarded but kind expression. Full-body front three-quarter character concept, warm-core wasteland 3D animated micro-film style, neutral dusty background, no readable text.",
    slug: "solvea-lev-wasteland-driver",
    summaryEn: "Owned supporting-character prompt with Lev's completed production concept image.",
    summaryZh: "自有配角提示词，已配 Lev 的生产概念图。",
    tags: ["character"],
    titleEn: "Lev wasteland driver character",
    titleZh: "Lev / 荒原司机角色图",
    url: "/assets/prompts/solvea/lev_wasteland_driver.png",
  }),
  solveaImagePromptItem({
    alt: "Vera wasteland doctor character sheet",
    outputEn: "4:3 character sheet",
    outputRatio: "4:3",
    outputZh: "4:3 角色设定图",
    prompt:
      "Create Vera, a calm wasteland doctor in her 40s with practical short hair, weathered medical coat layered over travel clothes, canvas satchel, patched sleeves, gentle determined eyes, and small improvised medical tools. Full-body front three-quarter character concept, warm-core wasteland 3D animated style, neutral dusty background, no readable text.",
    slug: "solvea-vera-wasteland-doctor",
    summaryEn: "Owned character prompt paired with Vera's finished concept sheet.",
    summaryZh: "自有角色提示词，配有 Vera 的完成概念图。",
    tags: ["character"],
    titleEn: "Vera wasteland doctor character",
    titleZh: "Vera / 荒原医生角色图",
    url: "/assets/prompts/solvea/vera_wasteland_doctor.png",
  }),
  solveaImagePromptItem({
    alt: "Cole retired soldier character sheet",
    outputEn: "4:3 character sheet",
    outputRatio: "4:3",
    outputZh: "4:3 角色设定图",
    prompt:
      "Create Cole, a retired soldier survivor with broad shoulders, gray stubble, patched field coat, old protective vest, worn cargo pants, calm protective stance, and a softened expression that suggests guilt and responsibility. Full-body front three-quarter character concept, warm-core wasteland 3D animated micro-film style, no weapons, neutral background, no readable text.",
    slug: "solvea-cole-retired-soldier",
    summaryEn: "Owned supporting-character prompt with Cole's finished visual artifact.",
    summaryZh: "自有配角提示词，配有 Cole 的完成视觉产物。",
    tags: ["character"],
    titleEn: "Cole retired soldier character",
    titleZh: "Cole / 退役士兵角色图",
    url: "/assets/prompts/solvea/cole_retired_soldier.png",
  }),
  solveaImagePromptItem({
    alt: "Milo greenhouse keeper character sheet",
    outputEn: "4:3 character sheet",
    outputRatio: "4:3",
    outputZh: "4:3 角色设定图",
    prompt:
      "Create Milo, a greenhouse keeper teenager with curly hair, oversized patched sweater, apron pockets filled with seed packets and small tools, round glasses, muddy sneakers, and an anxious hopeful smile. Full-body front three-quarter character concept, warm-core wasteland 3D animated style, soft plant-life color accents, neutral background, no readable text.",
    slug: "solvea-milo-greenhouse-keeper",
    summaryEn: "Owned character prompt paired with Milo's generated concept image.",
    summaryZh: "自有角色提示词，配有 Milo 的生成概念图。",
    tags: ["character"],
    titleEn: "Milo greenhouse keeper character",
    titleZh: "Milo / 温室守护者角色图",
    url: "/assets/prompts/solvea/milo_greenhouse_keeper.png",
  }),
  solveaImagePromptItem({
    alt: "Lin Cher researcher character sheet",
    outputEn: "4:3 character sheet",
    outputRatio: "4:3",
    outputZh: "4:3 角色设定图",
    prompt:
      "Create Lin Cher, a thoughtful researcher with tidy dark hair, practical lab coat under a travel cloak, cracked tablet device, field notebook, careful posture, and a conflicted expression. Full-body front three-quarter character concept, warm-core wasteland 3D animated micro-film style, subtle science-survivor details, neutral background, no readable text.",
    slug: "solvea-lin-cher-researcher",
    summaryEn: "Owned researcher prompt paired with Lin Cher's finished character image.",
    summaryZh: "自有研究员提示词，配有 Lin Cher 的完成角色图。",
    tags: ["character"],
    titleEn: "Lin Cher researcher character",
    titleZh: "Lin Cher / 研究员角色图",
    url: "/assets/prompts/solvea/lin_cher_researcher.png",
  }),
  solveaImagePromptItem({
    alt: "Standard infected silhouette sheet",
    outputEn: "4:3 creature sheet",
    outputRatio: "4:3",
    outputZh: "4:3 生物设定图",
    prompt:
      "Create a standard infected silhouette design for a family-friendly wasteland animated micro-film. Thin unstable body shape, torn everyday clothes, awkward off-balance posture, slightly glowing eyes, readable as threatening from shape language but not graphic or gory. Full-body front three-quarter creature concept, neutral dusty background, no text.",
    slug: "solvea-standard-infected-silhouette",
    summaryEn: "Owned non-graphic creature prompt paired with a finished infected silhouette sheet.",
    summaryZh: "自有非血腥生物提示词，配有感染者剪影设定图。",
    tags: ["creature"],
    titleEn: "Standard infected silhouette",
    titleZh: "标准感染者剪影设定",
    url: "/assets/prompts/solvea/standard_infected_silhouette.png",
  }),
  solveaImagePromptItem({
    alt: "School side gate hallway scene bible",
    outputEn: "3x3 scene board",
    outputRatio: "3x3",
    outputZh: "3x3 场景板",
    prompt:
      "Create a 3x3 scene bible sheet for Episode 01 school side gate and hallway. Panels include bent side gate, quiet entrance path, abandoned hallway, classroom doorway, raincoat hook detail, dusty floor footprints, broken window light, storage corner, and emergency exit view. Warm-core wasteland 3D animation environment, no readable signs.",
    slug: "solvea-ep01-school-side-gate-hallway",
    summaryEn: "Owned school-route scene prompt paired with a generated 3x3 environment board.",
    summaryZh: "自有学校路线场景提示词，配有生成的 3x3 环境板。",
    tags: ["scene-bible"],
    titleEn: "EP01 school side gate hallway scene bible",
    titleZh: "EP01 学校侧门走廊场景圣经",
    url: "/assets/prompts/solvea/scene_bible_ep01_school_side_gate_hallway_3x3.png",
  }),
  solveaImagePromptItem({
    alt: "Wasteland city scene bible",
    outputEn: "3x3 scene board",
    outputRatio: "3x3",
    outputZh: "3x3 场景板",
    prompt:
      "Create a 3x3 scene bible sheet for Episode 02 wasteland city route. Panels include cracked avenue, leaning streetlights, collapsed storefront, overgrown median, delivery drone wreck, dusty underpass, rooftop water tank, distant greenhouse glow, and sunset skyline. Stylized warm-core 3D animated environment, no readable text.",
    slug: "solvea-ep02-wasteland-city-scene-bible",
    summaryEn: "Owned Episode 02 city-route prompt with a completed 3x3 scene board.",
    summaryZh: "自有 EP02 城市路线提示词，配有完成的 3x3 场景板。",
    tags: ["scene-bible"],
    titleEn: "EP02 wasteland city scene bible",
    titleZh: "EP02 荒原城市场景圣经",
    url: "/assets/prompts/solvea/scene_bible_ep02_wasteland_city_3x3.png",
  }),
  solveaImagePromptItem({
    alt: "Residential alley scene bible",
    outputEn: "3x3 scene board",
    outputRatio: "3x3",
    outputZh: "3x3 场景板",
    prompt:
      "Create a 3x3 scene bible sheet for Episode 03 residential alley choice. Panels include narrow alley entrance, stacked balconies, hanging cables, improvised barricade, hidden clinic door, water puddle reflection, rooftop escape ladder, warm window light, and alley exit into open dust. Warm-core wasteland 3D animated style, no readable text.",
    slug: "solvea-ep03-residential-alley-scene-bible",
    summaryEn: "Owned Episode 03 alley prompt paired with the completed residential scene board.",
    summaryZh: "自有 EP03 巷道提示词，配有完成的居民巷场景板。",
    tags: ["scene-bible"],
    titleEn: "EP03 residential alley scene bible",
    titleZh: "EP03 居民巷场景圣经",
    url: "/assets/prompts/solvea/scene_bible_ep03_residential_alley_3x3.png",
  }),
  solveaImagePromptItem({
    alt: "Episode 02 city route motion storyboard",
    outputEn: "3x3 motion storyboard",
    outputRatio: "3x3",
    outputZh: "3x3 镜头分镜板",
    prompt:
      "Create a 3x3 action storyboard for Episode 02 city route. Panel order: D-17 exits ruined avenue; package lock glows; wheel base slips on dust; Lev's vehicle appears; city silhouettes move in distance; shortcut through underpass; package almost falls; D-17 chooses safer route; greenhouse glow appears ahead. Keep D-17 consistent, no text labels.",
    slug: "solvea-ep02-city-route-storyboard",
    summaryEn: "Owned Episode 02 motion prompt paired with a finished 3x3 storyboard.",
    summaryZh: "自有 EP02 镜头提示词，配有完成的 3x3 分镜板。",
    tags: ["storyboard"],
    titleEn: "EP02 city route motion storyboard",
    titleZh: "EP02 城市路线镜头走势图",
    url: "/assets/prompts/solvea/storyboard_ep02_city_route_motion_3x3.png",
  }),
  solveaImagePromptItem({
    alt: "Episode 03 alley choice motion storyboard",
    outputEn: "3x3 motion storyboard",
    outputRatio: "3x3",
    outputZh: "3x3 镜头分镜板",
    prompt:
      "Create a 3x3 action storyboard for Episode 03 alley choice. Panel order: D-17 enters narrow alley; Mia hears the package chime; Vera blocks an unsafe route; infected silhouettes pass by; D-17 hides behind crates; package glow reveals a side door; Cole holds the barricade; D-17 squeezes through; warm safe room reveal. No readable text labels.",
    slug: "solvea-ep03-alley-choice-storyboard",
    summaryEn: "Owned Episode 03 action prompt with a completed alley-choice storyboard.",
    summaryZh: "自有 EP03 动作提示词，配有完成的巷道选择分镜板。",
    tags: ["storyboard"],
    titleEn: "EP03 alley choice motion storyboard",
    titleZh: "EP03 巷道选择镜头走势图",
    url: "/assets/prompts/solvea/storyboard_ep03_alley_choice_motion_3x3.png",
  }),
  solveaImagePromptItem({
    alt: "Heart of Hope vertical cover",
    outputEn: "9:16 cover art",
    outputRatio: "9:16",
    outputZh: "9:16 竖版封面",
    prompt:
      "Create a vertical 9:16 cover image for an original warm-core wasteland animated short called Heart of Hope. D-17 stands in the foreground holding Package 0001 with warm golden glow; ruined city behind, Mia silhouette nearby, hopeful sunrise dust light, cinematic poster composition, no readable text or title typography.",
    slug: "solvea-heart-of-hope-vertical-cover",
    summaryEn: "Owned cover prompt paired with a finished vertical key-art image.",
    summaryZh: "自有封面提示词，配有完成的竖版主视觉图。",
    tags: ["cover", "poster"],
    titleEn: "Heart of Hope vertical cover",
    titleZh: "希望之心竖版封面",
    url: "/assets/prompts/solvea/heart-of-hope-cover-vertical-v5-h3.png",
  }),
  solveaImagePromptItem({
    alt: "Comedic infected 3x3 creature variation sheet",
    outputEn: "3x3 creature board",
    outputRatio: "3x3",
    outputZh: "3x3 生物变化板",
    prompt:
      "Create a 3x3 creature variation sheet for family-friendly comedic infected characters in Heart of Hope. Nine awkward silhouettes with different broken clothing shapes, clumsy body language, mild glowing eyes, expressive non-gory designs, warm-core 3D animated style, dusty neutral background, no horror gore, no text labels.",
    slug: "solvea-comedic-infected-variation-board",
    summaryEn: "Owned creature-variation prompt paired with a completed 3x3 infected board.",
    summaryZh: "自有生物变化提示词，配有完成的 3x3 感染者变化板。",
    tags: ["creature", "variation"],
    titleEn: "Comedic infected variation board",
    titleZh: "喜剧感染者变化板",
    url: "/assets/prompts/solvea/heart-of-hope-comedic-zombie-3x3.png",
  }),
  githubImagePromptItem({
    alt: "Marketplace skincare ecommerce main image",
    outputEn: "1:1 ecommerce main image",
    outputRatio: "1:1",
    outputZh: "1:1 电商主图",
    prompt:
      "Create a marketplace main image for {{product}} on a pure white background with a front three-quarter angle and the product filling 82%-88% of the frame. Accurately show {{material}}, edge structure, and true proportions. If {{accessories}} are included, place them neatly at the lower right while the main product remains the visual center. Use even lighting, very soft shadows, clean commercial photography, no scene props, no people, no text, no border, and no watermark. Output a 1:1 square image suitable for Amazon, Shopify, and TikTok Shop listings.",
    slug: "awesome-images-marketplace-main-image",
    summaryEn: "GitHub Image Buddy ecommerce template paired with its skincare main-image demo output.",
    summaryZh: "GitHub Image Buddy 电商主图模板，配有护肤品主图 demo 产物。",
    tags: ["ecommerce", "product"],
    titleEn: "Marketplace ecommerce main image",
    titleZh: "电商平台白底主图",
    url: "/assets/prompts/awesome-images/ecommerce-skincare.png",
  }),
  githubImagePromptItem({
    alt: "UGC coffee ad cover frame",
    outputEn: "9:16 UGC ad frame",
    outputRatio: "9:16",
    outputZh: "9:16 UGC 广告首帧",
    prompt:
      "Create a 9:16 UGC ad cover frame. Show {{audience}} naturally using {{product}} in {{scene}}, with the relaxed expression that comes after solving {{pain point}}. Frame it like a phone-shot clip, but keep the image professionally sharp, with realistic hand motion, a clearly visible product, and small lifestyle details in the environment. Leave 18% of the top area open for copy space without generating any text. Keep the colors bright without heavy filters, preserve natural skin texture, add slight depth of field, and make it suitable for TikTok and Reels first frames. Avoid plastic surfaces, over-smoothing, garbled text, and extra brand logos.",
    slug: "awesome-images-ugc-ad-still",
    summaryEn: "GitHub UGC ad cover-frame prompt paired with the coffee ad still artifact.",
    summaryZh: "GitHub UGC 广告封面帧提示词，配有咖啡广告首帧产物。",
    tags: ["ugc", "ads"],
    titleEn: "UGC ad cover frame",
    titleZh: "UGC 广告封面帧",
    url: "/assets/prompts/awesome-images/ugc-coffee-ad.png",
  }),
  githubImagePromptItem({
    alt: "Liquid glass bento infographic",
    outputEn: "16:9 bento infographic",
    outputRatio: "16:9",
    outputZh: "16:9 Bento 信息图",
    prompt:
      "Create a 16:9 bento infographic for {{topic}} in {{language}}. Use a premium liquid-glass interface style with a softly blurred abstract background in {{primary color}}. Build 8 foreground modules with translucent glass cards, thin borders, and realistic shadows. M1 should present the main visual, M2 the four core benefits, M3 the four-step workflow, M4 key data such as {{data point}}, M5 the target audience, M6 important notes, M7 quick reference info, and M8 a small surprise fact. Keep all text crisp and well organized, with no garbled characters. Use a consistent line-icon style and generous whitespace. Make it suitable for article covers and product education pages.",
    slug: "awesome-images-liquid-bento-infographic",
    summaryEn: "GitHub bento infographic template paired with a generated liquid-glass demo.",
    summaryZh: "GitHub Bento 信息图模板，配有液态玻璃 demo 产物。",
    tags: ["infographic", "bento"],
    titleEn: "Liquid glass bento infographic",
    titleZh: "液态玻璃 Bento 信息图",
    url: "/assets/prompts/awesome-images/liquid-bento.png",
  }),
  githubImagePromptItem({
    alt: "Consistent cyber portrait avatar",
    outputEn: "1:1 avatar",
    outputRatio: "1:1",
    outputZh: "1:1 头像",
    prompt:
      "Create a premium 1:1 avatar for {{character}}. Their role is {{profession}} and the visual style is {{style}}. Keep the face natural, the gaze clear, and the shoulders proportionate. Add a small amount of {{background element}} as an identity cue. Use a bust composition with the head centered and a background that stays simple. Light it softly with a clean rim light so it works as an avatar for product communities, support accounts, and creator profiles. Render at high resolution and avoid exaggerated expressions, distorted facial features, text, watermarks, or extra decoration.",
    slug: "awesome-images-consistent-avatar",
    summaryEn: "GitHub avatar template paired with a finished cyber portrait demo image.",
    summaryZh: "GitHub 头像模板，配有完成的 cyber portrait demo 图。",
    tags: ["avatar", "portrait"],
    titleEn: "Consistent avatar portrait",
    titleZh: "统一风格头像",
    url: "/assets/prompts/awesome-images/cyber-portrait.png",
  }),
  githubImagePromptItem({
    alt: "Fitness app store screenshot poster",
    outputEn: "9:16 app screenshot poster",
    outputRatio: "9:16",
    outputZh: "9:16 App 截图海报",
    prompt:
      "Create a 9:16 App Store screenshot poster for {{app name}}. The core feature is {{core feature}} and the interface type is {{interface type}}. Place a realistic phone in the scene and show a credible product UI on the screen: top navigation, key data areas, primary action buttons, and list or card content. Use a clean gradient in {{brand color}} with lightweight product symbols in the background, and leave room around the phone for title and benefit copy without generating long text. The UI must feel like a real product, with no garbled text, no unreadably small copy, and no overdone 3D cards.",
    slug: "awesome-images-app-store-screenshot",
    summaryEn: "GitHub mobile app screenshot template paired with a finished fitness app demo.",
    summaryZh: "GitHub 移动应用截图模板，配有健身 App demo 产物。",
    tags: ["app", "ui"],
    titleEn: "App Store screenshot poster",
    titleZh: "App Store 截图海报",
    url: "/assets/prompts/awesome-images/fitness-app.png",
  }),
  githubImagePromptItem({
    alt: "AI agent event poster key visual",
    outputEn: "3:4 event poster",
    outputRatio: "4:3",
    outputZh: "活动主视觉",
    prompt:
      "Create a 3:4 key visual for {{event theme}}. The target audience is {{target audience}} and the visual metaphor is {{visual metaphor}}. Build the scene with a clear main object, strong foreground layering, and usable background space. Leave the top area open for the title, place the main visual in the middle, and reserve the bottom for date, speaker, and registration information without generating literal copy. Keep the style modern and commercial, suitable for official account covers and offline rollups. Use dramatic but clean lighting and avoid garbled text, watermarks, and QR codes.",
    slug: "awesome-images-event-poster-key-visual",
    summaryEn: "GitHub event-poster prompt paired with the AI agent poster demo artifact.",
    summaryZh: "GitHub 活动海报提示词，配有 AI Agent 海报 demo 产物。",
    tags: ["poster", "event"],
    titleEn: "Event poster key visual",
    titleZh: "活动海报主 KV",
    url: "/assets/prompts/awesome-images/ai-agent-poster.png",
  }),
  githubImagePromptItem({
    alt: "Crystal game prop concept sheet",
    outputEn: "4:3 game prop sheet",
    outputRatio: "4:3",
    outputZh: "4:3 游戏道具设定",
    prompt:
      "Create a game prop concept sheet for {{prop type}}. The world setting is {{world}}, the main material is {{material}}, and the rarity level is {{rarity}}. Include 6 variants arranged on a clean design-sheet canvas, with consistent angles, clear silhouettes, and separate cutout potential for each prop. You may include a very short label area at the bottom, but do not generate unreadably small text. Keep the style unified, the lighting consistent, and the details suitable for later 3D modeling or 2D sprite production. Avoid merged duplicates, broken perspective, heavy backgrounds, and watermarks.",
    slug: "awesome-images-game-prop-sheet",
    summaryEn: "GitHub game prop template paired with a crystal prop concept output.",
    summaryZh: "GitHub 游戏道具模板，配有水晶道具概念图产物。",
    tags: ["game", "prop"],
    titleEn: "Game prop concept sheet",
    titleZh: "游戏道具设定图",
    url: "/assets/prompts/awesome-images/game-prop-crystal.png",
  }),
  githubImagePromptItem({
    alt: "Streetwear fashion lookbook collage",
    outputEn: "4:5 fashion lookbook",
    outputRatio: "4:3",
    outputZh: "服装 Lookbook",
    prompt:
      "Create a premium 4:5 lookbook collage poster for {{garment}}. The model attitude is {{model mood}}, the setting is {{scene}}, and the brand inspiration is close to {{brand inspiration}}. Show the same model in 3 poses: full body, half body, and detail close-up, arranged in a layered layout without feeling crowded. Keep the fabric, cut, folds, and accessories clearly visible, with an editorial magazine feel in the background. Leave a small amount of copy space without generating literal text. Avoid distorted fingers, incorrect limbs, garbled text, and extra logos.",
    slug: "awesome-images-fashion-lookbook",
    summaryEn: "GitHub fashion lookbook template paired with a streetwear collage demo.",
    summaryZh: "GitHub 服装 Lookbook 模板，配有街头服饰拼贴 demo。",
    tags: ["fashion", "ecommerce"],
    titleEn: "Fashion lookbook collage",
    titleZh: "服装 Lookbook 拼贴",
    url: "/assets/prompts/awesome-images/streetwear-lookbook.png",
  }),
  githubImagePromptItem({
    alt: "SaaS hero phone product visual",
    outputEn: "16:9 SaaS hero visual",
    outputRatio: "16:9",
    outputZh: "16:9 SaaS 首图",
    prompt:
      "Create a premium commercial hero visual for {{product name}}. The brand tone is {{brand tone}} and the core benefit is {{core benefit}}. Show the product itself in the center with a true proportion, crisp edges, and a surface that can withstand zoom inspection. Use {{primary color}} as the main visual clue in the background and add light scene elements related to the benefit without covering the product. Compose the image in 16:9 landscape, with the product taking 42% of the frame and clean negative space on the right for marketing copy. Use a large softbox plus one rim light, sharp detail, premium ecommerce photography, realistic shadows, and 8k quality. Avoid watermarks, garbled text, and fake logos.",
    slug: "awesome-images-saas-hero-phone",
    summaryEn: "GitHub premium product hero template paired with a SaaS phone hero demo.",
    summaryZh: "GitHub 高端产品主视觉模板，配有 SaaS 手机首图 demo。",
    tags: ["product", "hero"],
    titleEn: "SaaS product hero phone",
    titleZh: "SaaS 产品手机首图",
    url: "/assets/prompts/awesome-images/saas-hero-phone.png",
  }),
  githubImagePromptItem({
    alt: "Sports shoe ecommerce product image",
    outputEn: "1:1 ecommerce product image",
    outputRatio: "1:1",
    outputZh: "1:1 电商产品图",
    prompt:
      "Create a marketplace main image for {{product}} on a pure white background with a front three-quarter angle and the product filling 82%-88% of the frame. Accurately show {{material}}, edge structure, and true proportions. If {{accessories}} are included, place them neatly at the lower right while the main product remains the visual center. Use even lighting, very soft shadows, clean commercial photography, no scene props, no people, no text, no border, and no watermark. Output a 1:1 square image suitable for Amazon, Shopify, and TikTok Shop listings.",
    slug: "awesome-images-sports-shoe-product",
    summaryEn: "GitHub ecommerce template reused with a finished sports shoe product artifact.",
    summaryZh: "GitHub 电商主图模板复用，配有运动鞋产品图产物。",
    tags: ["ecommerce", "product"],
    titleEn: "Sports shoe ecommerce image",
    titleZh: "运动鞋电商产品图",
    url: "/assets/prompts/awesome-images/sports-shoe.png",
  }),
  {
    artifact: {
      alt: "9:16 UGC ad clip produced by Flatkey CLI",
      kind: "video",
      poster: "/assets/cli/ugc-ad-clips.png",
      url: "/assets/cli/ugc-ad-clips.mp4",
    },
    category: "video",
    model: "seedance-2.0",
    output: outputLabel("9:16 short video", "9:16 竖屏短片", "9:16"),
    prompt:
      "Create a 9:16 UGC-style paid social video for a compact consumer product. Start with a handheld first-person product pickup, cut to a close product use moment, then a clean benefit reveal. Natural indoor daylight, casual creator framing, no readable text, no logos, edit-ready pacing.",
    slug: "ugc-paid-social-product-clip",
    source: {
      capturedAt: today,
      label: "flatkey-ai/flatkey-cli",
      platform: "GitHub",
      url: `${flatkeyGithubBase}/flatkey-cli`,
    },
    summary: withIdFallback({
      en: "A CLI-ready UGC video prompt paired with an existing rendered social ad clip.",
      zh: "可直接用于 CLI 的 UGC 视频提示词，并配有现成社交广告短片产物。",
      es: "Prompt UGC para CLI con clip social ya renderizado.",
      fr: "Prompt UGC prêt pour le CLI avec clip social rendu.",
      pt: "Prompt UGC para CLI com clipe social já renderizado.",
      ru: "UGC video prompt для CLI с готовым рекламным клипом.",
      ja: "CLI 向け UGC 動画プロンプト。生成済み広告クリップ付き。",
      vi: "Prompt video UGC cho CLI, kèm clip social đã render.",
      de: "CLI-tauglicher UGC-Video-Prompt mit fertigem Social-Ad-Clip.",
    }),
    tags: ["video", "ugc", "ads"],
    title: withIdFallback({
      en: "9:16 UGC product ad clip",
      zh: "9:16 UGC 产品广告短片",
      es: "Clip UGC 9:16 de producto",
      fr: "Clip UGC produit 9:16",
      pt: "Clipe UGC 9:16 de produto",
      ru: "UGC product ad 9:16",
      ja: "9:16 UGC 商品広告クリップ",
      vi: "Clip quảng cáo sản phẩm UGC 9:16",
      de: "9:16 UGC Produktclip",
    }),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Localized market variant video produced by Flatkey CLI",
      kind: "video",
      poster: "/assets/cli/localized-variants.png",
      url: "/assets/cli/localized-variants.mp4",
    },
    category: "video",
    model: "seedance-2.0",
    output: outputLabel("9:16 localized video", "9:16 本地化短片", "9:16"),
    prompt:
      "Create a short localized market variant video for a product launch. Use bright everyday lifestyle framing, culturally neutral props, a clear product-in-use moment, warm natural light, and space for localized captions added later. Keep the product consistent, no readable text, no brand logos.",
    slug: "localized-market-product-variant-video",
    source: {
      capturedAt: today,
      label: "flatkey-ai/flatkey-cli",
      platform: "GitHub",
      url: `${flatkeyGithubBase}/flatkey-cli`,
    },
    summary: withIdFallback({
      en: "A localization-focused video prompt paired with an existing variant clip artifact.",
      zh: "面向市场本地化的视频提示词，并配有现成变体短片产物。",
      es: "Prompt de video localizado con clip de variante existente.",
      fr: "Prompt vidéo de localisation avec clip variant existant.",
      pt: "Prompt de vídeo localizado com clipe variante existente.",
      ru: "Prompt для localized video с готовым вариантом клипа.",
      ja: "市場別ローカライズ動画プロンプト。生成済みクリップ付き。",
      vi: "Prompt video bản địa hóa, kèm clip biến thể có sẵn.",
      de: "Lokalisierungs-Video-Prompt mit vorhandenem Variant-Clip.",
    }),
    tags: ["video", "localization", "launch"],
    title: withIdFallback({
      en: "Localized product launch variant",
      zh: "产品发布本地化视频变体",
      es: "Variante localizada de lanzamiento",
      fr: "Variante localisée de lancement",
      pt: "Variante localizada de lançamento",
      ru: "Localized launch variant",
      ja: "ローカライズ版ローンチ動画",
      vi: "Biến thể video ra mắt bản địa hóa",
      de: "Lokalisierte Launch-Variante",
    }),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Cinematic product reveal video produced by Flatkey CLI",
      kind: "video",
      poster: "/assets/cli/product-reveal.png",
      url: "/assets/cli/product-reveal.mp4",
    },
    category: "video",
    model: "seedance-2.0",
    output: outputLabel("16:9 production storyboard", "16:9 成片分镜", "16:9"),
    prompt:
      "8 second cinematic product reveal, glossy black background, controlled reflections, slow camera push-in, product remains sharp and centered, no floating text, one continuous shot.",
    slug: "cinematic-product-reveal-video",
    source: {
      capturedAt: today,
      label: "flatkey-ai/flatkey-cli",
      platform: "GitHub",
      url: `${flatkeyGithubBase}/flatkey-cli`,
    },
    summary: withIdFallback({
      en: "A repeatable video prompt from Flatkey CLI usage patterns, rendered here as a production storyboard artifact.",
      zh: "来自 Flatkey CLI 用法的视频提示词，这里生成了可执行的成片分镜产物。",
      es: "Prompt de video basado en Flatkey CLI, con storyboard como resultado.",
      fr: "Prompt vidéo inspiré du CLI Flatkey, avec storyboard de production.",
      pt: "Prompt de vídeo do fluxo Flatkey CLI, com storyboard de produção.",
      ru: "Видео prompt из Flatkey CLI, с раскадровкой как результатом.",
      ja: "Flatkey CLI の動画生成例をもとにしたプロンプト。成果物は制作分镜。",
      vi: "Prompt video từ Flatkey CLI, có storyboard sản xuất đi kèm.",
      de: "Video-Prompt aus Flatkey-CLI-Mustern mit Storyboard-Ergebnis.",
    }),
    tags: ["video", "seedance", "product"],
    title: withIdFallback({
      en: "Cinematic product reveal",
      zh: "电影感产品揭幕视频",
      es: "Revelación cinematográfica de producto",
      fr: "Révélation produit cinématique",
      pt: "Reveal cinematográfico de produto",
      ru: "Кинематографичный product reveal",
      ja: "映画風プロダクトリビール",
      vi: "Video reveal sản phẩm điện ảnh",
      de: "Cinematic Product Reveal",
    }),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Seedance 2.5 image-to-video sample clip",
      kind: "video",
      poster: "/assets/video/v1.1.jpg",
      url: "/assets/video/v1.1.mp4",
    },
    category: "video",
    model: "seedance-2.5",
    output: outputLabel("16:9 image-to-video clip", "16:9 图生视频短片", "16:9"),
    prompt:
      "Create a polished image-to-video product scene from a provided still. Keep the product identity locked, use a slow camera push with subtle parallax, controlled reflections, and clean commercial lighting. Avoid text, logos, flicker, identity drift, and unnecessary camera shake.",
    slug: "flatkey-image-to-video-product-scene",
    source: solveaOwnedSource(),
    summary: promptText(
      "Owned video sample paired with a rendered image-to-video product clip.",
      "自有视频样例，配有已经渲染完成的图生视频产品短片。",
    ),
    tags: ["video", "i2v", "product"],
    title: promptText("Image-to-video product scene", "图生视频产品场景"),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Character motion video sample clip",
      kind: "video",
      poster: "/assets/video/v1.2.jpg",
      url: "/assets/video/v1.2.mp4",
    },
    category: "video",
    model: "seedance-2.5",
    output: outputLabel("9:16 character motion clip", "9:16 角色动作短片", "9:16"),
    prompt:
      "Create a short character motion clip from a reference image. Preserve face, outfit, silhouette, and color palette. Add a simple natural gesture, soft environmental movement, stable camera, and warm practical light. No readable text, no extra characters, no identity morphing.",
    slug: "flatkey-character-reference-motion-clip",
    source: solveaOwnedSource(),
    summary: promptText(
      "Owned character-motion prompt paired with a finished reference-to-video clip.",
      "自有角色动作提示词，配有完成的参考图转视频短片。",
    ),
    tags: ["video", "character", "i2v"],
    title: promptText("Character reference motion clip", "角色参考动作短片"),
    updatedAt: today,
  },
  {
    artifact: {
      alt: "Text-to-video cinematic test clip",
      kind: "video",
      poster: "/assets/video/v1.3.jpg",
      url: "/assets/video/v1.3.mp4",
    },
    category: "video",
    model: "veo-3.1-fast-generate-preview",
    output: outputLabel("16:9 text-to-video clip", "16:9 文生视频短片", "16:9"),
    prompt:
      "Generate a clean cinematic product reveal from text only. One continuous shot, product centered, slow dolly-in, soft rim light, realistic reflections, shallow depth of field, edit-ready pacing. Avoid floating captions, watermarks, extra logos, and sudden scene changes.",
    slug: "flatkey-text-to-video-cinematic-test",
    source: solveaOwnedSource(),
    summary: promptText(
      "Owned text-to-video sample with a completed cinematic test clip.",
      "自有文生视频样例，配有完成的电影感测试短片。",
    ),
    tags: ["video", "t2v", "product"],
    title: promptText("Text-to-video cinematic test", "文生视频电影感测试"),
    updatedAt: today,
  },
  {
    artifact: {
      body:
        "Headline set:\n1. One key for every model your product ships.\n2. Stop rebuilding provider billing twice.\n3. Route, price, and observe AI calls in one place.\n4. Give agents tools without handing them every vendor key.\n5. Launch model features with a ledger already attached.",
      kind: "text",
      title: "Generated launch headlines",
    },
    category: "text",
    model: "gpt-5",
    output: outputLabel("Text output", "文本产物", "1:1"),
    prompt:
      "Write 5 sharp launch headlines for a developer tool that unifies AI model routing, tool calls, billing, and observability. Keep each under 14 words, avoid hype, and make the buyer value concrete.",
    slug: "developer-tool-launch-headlines",
    source: {
      capturedAt: today,
      label: "flatkey-ai/how-to-use-flatkey",
      platform: "GitHub",
      url: `${flatkeyGithubBase}/how-to-use-flatkey`,
    },
    summary: withIdFallback({
      en: "A text-generation prompt shaped for OpenAI-compatible Flatkey samples, with finished headline output.",
      zh: "面向 Flatkey OpenAI 兼容示例的文本生成提示词，并附带已生成标题产物。",
      es: "Prompt de texto para ejemplos Flatkey compatibles con OpenAI, con titulares terminados.",
      fr: "Prompt texte pour exemples Flatkey compatibles OpenAI, avec titres produits.",
      pt: "Prompt de texto para exemplos Flatkey compatíveis com OpenAI, com títulos prontos.",
      ru: "Текстовый prompt для OpenAI-совместимых примеров Flatkey, с готовыми заголовками.",
      ja: "OpenAI 互換 Flatkey サンプル向けのテキスト生成プロンプト。生成済み見出し付き。",
      vi: "Prompt tạo văn bản cho mẫu Flatkey tương thích OpenAI, có headline hoàn chỉnh.",
      de: "Text-Prompt für OpenAI-kompatible Flatkey-Beispiele mit fertigen Headlines.",
    }),
    tags: ["copywriting", "developer", "launch"],
    title: withIdFallback({
      en: "Developer tool launch headlines",
      zh: "开发者工具发布标题",
      es: "Titulares de lanzamiento para herramienta dev",
      fr: "Titres de lancement pour outil dev",
      pt: "Headlines de lançamento para ferramenta dev",
      ru: "Заголовки запуска dev tool",
      ja: "開発者ツールのローンチ見出し",
      vi: "Headline ra mắt công cụ developer",
      de: "Launch-Headlines für Developer-Tool",
    }),
    updatedAt: today,
  },
  {
    artifact: {
      code: `flatkey video generate \\
  --model seedance-2.0 \\
  --prompt "8 second cinematic product reveal, glossy black background" \\
  --ratio 16:9 \\
  --resolution 720p \\
  --dry-run \\
  --json`,
      kind: "code",
      language: "bash",
    },
    category: "agent",
    model: "gpt-5",
    output: outputLabel("CLI command", "CLI 命令", "1:1"),
    prompt:
      "Before spending credits, inspect the request shape for a media generation job. Use JSON output, keep the prompt unchanged, and return model, ratio, resolution, estimated route, and the final command to run.",
    slug: "agent-dry-run-before-spend",
    source: {
      capturedAt: today,
      label: "flatkey-ai/flatkey-cli agent protocol",
      platform: "GitHub",
      url: `${flatkeyGithubBase}/flatkey-cli`,
    },
    summary: withIdFallback({
      en: "An agent prompt for checking Flatkey CLI request shape before spending credits, with the command artifact included.",
      zh: "让 Agent 在消耗余额前检查 Flatkey CLI 请求形状的提示词，并附带命令产物。",
      es: "Prompt de agente para revisar una petición Flatkey CLI antes de gastar créditos.",
      fr: "Prompt agent pour inspecter une requête Flatkey CLI avant dépense.",
      pt: "Prompt de agente para validar uma chamada Flatkey CLI antes de gastar créditos.",
      ru: "Agent prompt для проверки запроса Flatkey CLI перед списанием кредитов.",
      ja: "クレジット消費前に Flatkey CLI リクエストを確認する Agent プロンプト。",
      vi: "Prompt agent kiểm tra request Flatkey CLI trước khi tiêu credit.",
      de: "Agent-Prompt zur Prüfung eines Flatkey-CLI-Requests vor Kosten.",
    }),
    tags: ["agent", "cli", "cost-control"],
    title: withIdFallback({
      en: "Agent dry run before spend",
      zh: "Agent 消费前 dry run",
      es: "Dry run de agente antes de gastar",
      fr: "Dry run agent avant dépense",
      pt: "Dry run de agente antes do gasto",
      ru: "Agent dry run перед расходом",
      ja: "消費前の Agent dry run",
      vi: "Agent dry run trước khi tiêu phí",
      de: "Agent Dry Run vor Kosten",
    }),
    updatedAt: today,
  },
  textPromptItem({
    artifactBody:
      "Executive brief\nMarket motion: a new self-serve tier is likely aimed at smaller teams.\nEvidence to collect: launch post, pricing page diff, changelog, customer quotes, and hiring signals.\nNext move: update battlecard, run pricing comparison, and prepare two objection-handling notes.",
    artifactTitle: "Competitor launch brief",
    model: "gpt-5",
    outputEn: "Research memo",
    outputZh: "研究简报",
    prompt:
      "Build a concise competitor launch brief from public web evidence. Separate facts from inference, cite each source URL, summarize positioning, pricing change, target customer, distribution channel, and the likely counter-message for our sales team. Keep the final memo under 350 words.",
    slug: "web-competitor-launch-brief",
    source: sourceSignal({
      label: "OpenAI prompt engineering guidance signal",
      platform: "Official docs",
      url: "https://platform.openai.com/docs/guides/prompt-engineering",
    }),
    summaryEn: "Original web-research prompt paired with a structured competitive-intelligence memo output.",
    summaryZh: "原创网页研究提示词，配有结构化竞品情报简报产物。",
    tags: ["research", "competitive", "gtm", "web"],
    titleEn: "Web competitor launch brief",
    titleZh: "网页竞品发布简报",
  }),
  textPromptItem({
    artifactBody:
      "Persona: VP Engineering at a 200-person SaaS company.\nPain: AI feature spend is split across provider dashboards.\nMessage: one API key, route-level governance, and one ledger for finance.\nCTA: migrate the first agent workflow behind Flatkey this week.",
    artifactTitle: "Sales account brief",
    model: "claude-sonnet-5",
    outputEn: "Account brief",
    outputZh: "客户简报",
    prompt:
      "Create an enterprise account brief from a company website, hiring page, pricing page, and three recent public posts. Return buyer persona, likely pain, current trigger, relevant proof, and a 5-sentence first-touch email. Mark any unsupported claim as inference.",
    slug: "enterprise-account-brief-from-web",
    source: sourceSignal({
      label: "Anthropic prompt engineering overview signal",
      platform: "Official docs",
      url: "https://docs.anthropic.com/en/docs/build-with-claude/prompt-engineering/overview",
    }),
    summaryEn: "Original sales-research prompt with a finished account brief artifact.",
    summaryZh: "原创销售研究提示词，附带完成的客户简报产物。",
    tags: ["sales", "research", "web", "enterprise"],
    titleEn: "Enterprise account brief from web signals",
    titleZh: "基于网页信号的企业客户简报",
  }),
  textPromptItem({
    artifactBody:
      "Support cluster\nTop issue: users cannot map model names to billing rows.\nSuggested fix: add model aliases in docs and invoice exports.\nMacro reply: include exact model, timestamp, request id, and current pricing link.",
    artifactTitle: "Support triage report",
    model: "gpt-5-mini",
    outputEn: "Support triage",
    outputZh: "支持工单归类",
    prompt:
      "Cluster 40 support tickets into user-intent buckets. For each bucket, name the root cause hypothesis, impacted workflow, severity, fastest product fix, and a customer support macro. Do not merge tickets that have different failure points.",
    slug: "support-ticket-intent-clustering",
    source: sourceSignal({
      label: "Flatkey generated support workflow",
      platform: "Flatkey generated",
      url: `${flatkeyGithubBase}/how-to-use-flatkey`,
    }),
    summaryEn: "A support operations prompt with a paired triage report artifact.",
    summaryZh: "支持运营提示词，配有工单归类报告产物。",
    tags: ["support", "ops", "classification"],
    titleEn: "Support ticket intent clustering",
    titleZh: "支持工单意图聚类",
  }),
  textPromptItem({
    artifactBody:
      "PRD slice\nProblem: users can test prompts but cannot estimate production cost before copying code.\nRequirement: show model, context window, input/output estimate, and fallback route note.\nAcceptance: the estimate survives refresh and exports into snippet metadata.",
    artifactTitle: "PRD acceptance slice",
    model: "gpt-5",
    outputEn: "Product requirements",
    outputZh: "产品需求",
    prompt:
      "Turn a rough product idea into a one-page PRD. Include problem, user segment, job-to-be-done, non-goals, UX states, data contract, risks, launch guardrails, and five testable acceptance criteria. Keep decisions crisp and avoid roadmap filler.",
    slug: "one-page-prd-from-rough-idea",
    source: sourceSignal({
      label: "Google Gemini prompting strategies signal",
      platform: "Official docs",
      url: "https://ai.google.dev/gemini-api/docs/prompting-strategies",
    }),
    summaryEn: "Original product-management prompt paired with a concrete PRD slice.",
    summaryZh: "原创产品管理提示词，配有具体 PRD 切片产物。",
    tags: ["product", "prd", "planning"],
    titleEn: "One-page PRD from a rough idea",
    titleZh: "粗略想法转一页 PRD",
  }),
  textPromptItem({
    artifactBody:
      "Localization pack\nUS: Lead with time saved.\nJapan: Lead with operational reliability and support path.\nBrazil: Lead with prepaid control and local payment clarity.\nVariant rule: keep the same product fact, change proof order and objection handling.",
    artifactTitle: "Localized ad variants",
    model: "gemini-3.6-flash",
    outputEn: "Ad variant pack",
    outputZh: "广告变体包",
    prompt:
      "Generate localized ad message variants for three markets. Keep product claims identical, adapt proof order and objection handling, and output headline, 25-word body copy, CTA, required disclaimer, and one creative direction per market.",
    slug: "localized-ad-message-variants",
    source: sourceSignal({
      label: "Microsoft Copilot prompt gallery signal",
      platform: "External",
      url: "https://adoption.microsoft.com/en-us/copilot/prompt-gallery/",
    }),
    summaryEn: "Original localization prompt paired with an ad message variant pack.",
    summaryZh: "原创本地化提示词，配有广告消息变体包产物。",
    tags: ["ads", "localization", "copywriting"],
    titleEn: "Localized ad message variants",
    titleZh: "本地化广告消息变体",
  }),
  textPromptItem({
    artifactBody:
      "Eval rubric\nAccuracy: rejects unsupported claims.\nCompleteness: covers all required fields.\nDecision quality: names tradeoffs and gives a recommended next step.\nFailure mode: confident answer without source boundary.",
    artifactTitle: "Prompt evaluation rubric",
    model: "claude-opus-4-8",
    outputEn: "Evaluation rubric",
    outputZh: "评估量表",
    prompt:
      "Design a compact evaluation rubric for a research assistant prompt. Include scoring dimensions, pass/fail thresholds, examples of high-risk hallucination, and a reviewer checklist that a non-technical operator can use in under two minutes.",
    slug: "research-assistant-eval-rubric",
    source: sourceSignal({
      label: "Anthropic evaluation workflow signal",
      platform: "Official docs",
      url: "https://docs.anthropic.com/en/docs/test-and-evaluate/define-success",
    }),
    summaryEn: "A prompt-quality evaluation prompt paired with a practical reviewer rubric.",
    summaryZh: "提示词质量评估提示词，配有可执行评审量表产物。",
    tags: ["evaluation", "qa", "research"],
    titleEn: "Research assistant eval rubric",
    titleZh: "研究助手评估量表",
  }),
  codePromptItem({
    code: `{
  "type": "object",
  "required": ["company", "signal", "evidence_url", "confidence"],
  "properties": {
    "company": { "type": "string" },
    "signal": { "enum": ["pricing", "hiring", "launch", "partnership", "customer_quote"] },
    "evidence_url": { "type": "string", "format": "uri" },
    "confidence": { "type": "number", "minimum": 0, "maximum": 1 }
  }
}`,
    language: "json",
    model: "gpt-5",
    outputEn: "JSON schema",
    outputZh: "JSON Schema",
    prompt:
      "Design a strict JSON schema for extracting competitive market signals from web pages. The schema must preserve source URL, confidence, timestamp, and whether the claim is fact or inference. Include only fields that can be validated downstream.",
    slug: "market-signal-json-schema",
    source: sourceSignal({
      label: "OpenAI structured outputs signal",
      platform: "Official docs",
      url: "https://platform.openai.com/docs/guides/structured-outputs",
    }),
    summaryEn: "Original extraction prompt paired with a downstream-safe JSON schema artifact.",
    summaryZh: "原创抽取提示词，配有适合下游校验的 JSON Schema 产物。",
    tags: ["json", "extraction", "web", "agent"],
    titleEn: "Market signal JSON schema",
    titleZh: "市场信号 JSON Schema",
  }),
  codePromptItem({
    code: `flatkey chat completions \\
  --model claude-sonnet-5 \\
  --prompt-file ./prompts/repo-audit.md \\
  --max-tokens 1600 \\
  --metadata task=repo-audit,env=staging`,
    language: "bash",
    model: "claude-sonnet-5",
    outputEn: "CLI command",
    outputZh: "CLI 命令",
    prompt:
      "Prepare a coding-agent prompt that audits a repository before a release. Require impact surface, risky files, migration concerns, test gaps, deployment targets, and a final go/no-go recommendation. Keep it usable as a CLI prompt file.",
    slug: "coding-agent-release-audit-command",
    source: sourceSignal({
      label: "Flatkey CLI release workflow",
      platform: "Flatkey generated",
      url: `${flatkeyGithubBase}/flatkey-cli`,
    }),
    summaryEn: "A release-audit agent prompt paired with a runnable Flatkey CLI command.",
    summaryZh: "发布审计 Agent 提示词，配有可运行 Flatkey CLI 命令产物。",
    tags: ["code", "release", "audit", "cli"],
    titleEn: "Coding agent release audit command",
    titleZh: "编程 Agent 发布审计命令",
  }),
  codePromptItem({
    code: `const response = await client.responses.create({
  model: "gpt-5",
  input: prompt,
  text: { format: marketSignalSchema },
  metadata: { workflow: "source-review" }
});`,
    language: "typescript",
    model: "gpt-5",
    outputEn: "TypeScript snippet",
    outputZh: "TypeScript 片段",
    prompt:
      "Write a minimal TypeScript snippet that sends a prompt to an OpenAI-compatible model endpoint and requests structured output for source review. Keep credentials external, include metadata, and avoid framework-specific code.",
    slug: "structured-output-source-review-snippet",
    source: sourceSignal({
      label: "OpenAI API structured output pattern",
      platform: "Official docs",
      url: "https://platform.openai.com/docs/guides/structured-outputs",
    }),
    summaryEn: "A developer prompt paired with a compact structured-output TypeScript artifact.",
    summaryZh: "开发者提示词，配有简洁结构化输出 TypeScript 产物。",
    tags: ["typescript", "structured-output", "developer"],
    titleEn: "Structured output source review snippet",
    titleZh: "结构化来源评审代码片段",
  }),
  storyboardPromptItem({
    frames: [
      "Phone screen opens to a crowded ad dashboard.",
      "Creator drags product still into the generator.",
      "Prompt library card expands with model and ratio.",
      "Seedance route preview shows cost before run.",
      "Product rotates under soft window light.",
      "Caption-safe frame leaves top space blank.",
      "Three localized variants appear side by side.",
      "Approved clip exports to the launch folder.",
      "Ledger row records model, seconds, and cost.",
    ],
    model: "seedance-2.0",
    outputEn: "9-frame video plan",
    outputRatio: "3x3",
    outputZh: "9 格视频计划",
    prompt:
      "Create a 9-frame storyboard for a 15-second product launch workflow video. Show the operator moving from prompt library to generated localized clips, with cost preview and final ledger record. Keep frames production-specific and avoid readable UI text.",
    slug: "prompt-library-to-video-launch-storyboard",
    source: sourceSignal({
      label: "Flatkey generated video workflow",
      platform: "Flatkey generated",
      url: `${flatkeyGithubBase}/flatkey-cli`,
    }),
    summaryEn: "A video workflow prompt paired with a nine-frame storyboard artifact.",
    summaryZh: "视频工作流提示词，配有九格分镜产物。",
    tags: ["video", "workflow", "launch"],
    titleEn: "Prompt library to video launch storyboard",
    titleZh: "提示词库到视频发布分镜",
  }),
  storyboardPromptItem({
    frames: [
      "Customer review sentence appears as a floating source card.",
      "Agent extracts pain, product detail, and sentiment.",
      "RAG answer drafts a claim with citation boundary.",
      "Evaluator rejects an unsupported comparison.",
      "Model retries with narrower evidence.",
      "Final answer highlights the exact cited fact.",
      "Human reviewer approves the short response.",
      "Snippet moves into help-center draft.",
      "Analytics marks source-reviewed completion.",
    ],
    model: "gpt-5",
    outputEn: "Agent workflow board",
    outputRatio: "3x3",
    outputZh: "Agent 工作流板",
    prompt:
      "Storyboard an evidence-grounded support-answer workflow for an AI agent. Each frame should show one state transition from raw customer text to approved cited answer. Include rejection and retry when evidence is weak.",
    slug: "evidence-grounded-support-agent-board",
    source: sourceSignal({
      label: "OpenAI eval and agent workflow signal",
      platform: "Official docs",
      url: "https://platform.openai.com/docs/guides/evals",
    }),
    summaryEn: "An agent workflow prompt paired with a storyboard showing evaluation and retry states.",
    summaryZh: "Agent 工作流提示词，配有展示评估与重试状态的分镜产物。",
    tags: ["agent", "support", "eval", "rag"],
    titleEn: "Evidence-grounded support agent board",
    titleZh: "有证据边界的支持 Agent 工作流板",
  }),
  textPromptItem({
    artifactBody:
      "Answer quality review\nGrounding: pass, two citations attached.\nInstruction fit: partial, missing concise summary.\nRisk: medium, one pricing inference needs source.\nRevision: replace the inference with a current pricing link and timestamp.",
    artifactTitle: "RAG answer review",
    model: "deepseek-v4-flash",
    outputEn: "QA review",
    outputZh: "质检评审",
    prompt:
      "Review a RAG answer against retrieved evidence. Score grounding, instruction fit, completeness, and unsupported claims. Return the minimum edit needed to make the answer publishable, not a full rewrite.",
    slug: "rag-answer-minimum-edit-review",
    source: sourceSignal({
      label: "Google Vertex AI prompt design signal",
      platform: "Official docs",
      url: "https://cloud.google.com/vertex-ai/generative-ai/docs/learn/prompts/introduction-prompt-design",
    }),
    summaryEn: "A RAG quality prompt paired with a minimum-edit review artifact.",
    summaryZh: "RAG 质量提示词，配有最小修改评审产物。",
    tags: ["rag", "evaluation", "qa"],
    titleEn: "RAG answer minimum-edit review",
    titleZh: "RAG 答案最小修改评审",
  }),
  textPromptItem({
    artifactBody:
      "Audio brief\nVoice: calm product narrator, medium pace.\nScene bed: soft office ambience, low keyboard taps.\nTiming: 0-2s hook, 2-7s workflow, 7-10s proof.\nMix note: leave room for captions and UI sounds.",
    artifactTitle: "Voiceover production brief",
    category: "audio",
    model: "gpt-5",
    outputEn: "Audio brief",
    outputZh: "音频简报",
    prompt:
      "Write an audio production brief for a 10-second product workflow ad. Define voice, pace, ambience, beat timing, mix constraints, and one safety note that prevents the narration from overpromising product capability.",
    slug: "product-workflow-audio-brief",
    source: sourceSignal({
      label: "Flatkey generated audio workflow",
      platform: "Flatkey generated",
      url: `${flatkeyGithubBase}/flatkey-cli`,
    }),
    summaryEn: "An audio prompt paired with a production-ready voiceover brief.",
    summaryZh: "音频提示词，配有可执行旁白制作简报产物。",
    tags: ["audio", "voiceover", "ads"],
    titleEn: "Product workflow audio brief",
    titleZh: "产品工作流音频简报",
  }),
  textPromptItem({
    artifactBody:
      "Decision note\nUse gpt-5 for first-pass synthesis when evidence is messy.\nUse claude-sonnet-5 for long policy drafts.\nUse deepseek-v4-flash for low-cost classification.\nEscalate to human review for legal, billing, or health claims.",
    artifactTitle: "Model-routing decision note",
    category: "agent",
    model: "gpt-5",
    outputEn: "Routing policy",
    outputZh: "路由策略",
    prompt:
      "Create a model-routing decision note for an agent that handles research, summarization, extraction, and policy-sensitive answers. Include when to use a fast model, when to use a stronger model, and when to stop for human review.",
    slug: "agent-model-routing-decision-note",
    source: sourceSignal({
      label: "Flatkey generated routing workflow",
      platform: "Flatkey generated",
      url: `${flatkeyGithubBase}/how-to-use-flatkey`,
    }),
    summaryEn: "A routing-policy prompt paired with a practical agent decision note.",
    summaryZh: "路由策略提示词，配有实用 Agent 决策说明产物。",
    tags: ["agent", "routing", "policy"],
    titleEn: "Agent model-routing decision note",
    titleZh: "Agent 模型路由决策说明",
  }),
  textPromptItem({
    artifactBody:
      "Incident summary\nImpact: elevated latency on image prompt previews.\nKnown good: text routes and billing ledger unaffected.\nCustomer message: acknowledge latency, give workaround, update every 30 minutes.\nPostmortem seed: compare provider status and internal queue depth.",
    artifactTitle: "Incident communication draft",
    model: "claude-sonnet-5",
    outputEn: "Incident draft",
    outputZh: "故障沟通草稿",
    prompt:
      "Draft a customer-facing incident update from raw engineering notes. Separate impact, current status, workaround, next update time, and what is still unknown. Do not blame providers unless the evidence is explicit.",
    slug: "customer-incident-update-from-engineering-notes",
    source: sourceSignal({
      label: "Flatkey generated operations workflow",
      platform: "Flatkey generated",
      url: `${flatkeyGithubBase}/how-to-use-flatkey`,
    }),
    summaryEn: "An operations prompt paired with a concise customer incident update artifact.",
    summaryZh: "运营提示词，配有简洁客户故障沟通产物。",
    tags: ["ops", "incident", "support"],
    titleEn: "Customer incident update from engineering notes",
    titleZh: "工程笔记转客户故障更新",
  }),
  textPromptItem({
    artifactBody:
      "PRD skeleton\nProblem: teams test prompts but cannot estimate cost before shipping.\nUsers: PMs and engineers validating model workflows.\nAcceptance: cost estimate, model route, prompt state, and fallback note persist after refresh.",
    artifactTitle: "PRD outline",
    model: "gpt-5",
    outputEn: "Product requirements",
    outputZh: "产品需求",
    prompt:
      "Act as a product manager. When I provide a feature idea, create a concise PRD with subject, problem statement, goals, user stories, technical requirements, KPIs, risks, and acceptance criteria. Ask one clarifying question only when the feature is ambiguous.",
    slug: "free-product-manager-prd-outline",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from the free prompts.chat Product Manager role prompt and paired with a PRD outline artifact.",
    summaryZh: "改写自 prompts.chat 免费 Product Manager 角色提示词，并配有 PRD 大纲产物。",
    tags: ["free", "prompts-chat", "product", "prd"],
    titleEn: "Product manager PRD outline",
    titleZh: "产品经理 PRD 大纲",
  }),
  textPromptItem({
    artifactBody:
      "| Idea | Persona | Pain | MVP | Validation |\n| Micro SaaS cost guard | AI builders | surprise model spend | route preview | 10 interviews + landing page |\n| Review-to-FAQ agent | support teams | repeated tickets | cited answer drafts | ticket deflection test |",
    artifactTitle: "Startup validation table",
    model: "claude-sonnet-5",
    outputEn: "Startup idea table",
    outputZh: "创业想法表",
    prompt:
      "Generate digital startup ideas from a user wish. Return a markdown table with idea name, one-line pitch, target persona, pain point, value proposition, MVP scope, validation steps, first-year operating cost estimate, and key risks.",
    slug: "free-startup-idea-validation-table",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free startup-idea prompt and paired with a validation table output.",
    summaryZh: "改写自免费创业想法提示词，并配有验证表格产物。",
    tags: ["free", "prompts-chat", "startup", "growth"],
    titleEn: "Startup idea validation table",
    titleZh: "创业想法验证表",
  }),
  textPromptItem({
    artifactBody:
      "Code review\nRisk: error path returns 200 with failed payload.\nFix: propagate typed error and add regression test.\nAlternative: keep controller thin and move retry policy into service.\nDeployment: console only unless shared API handler changes.",
    artifactTitle: "Code review notes",
    category: "agent",
    model: "gpt-5",
    outputEn: "Review report",
    outputZh: "审查报告",
    prompt:
      "Act as a senior code reviewer. Review the provided code for correctness, security, maintainability, performance, and missing tests. Lead with findings ordered by severity, include file or function references, explain impact, and propose the smallest safe fix.",
    slug: "free-code-review-actionable-findings",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free code reviewer prompt with a production-style review artifact.",
    summaryZh: "改写自免费代码审查提示词，并配有生产风格审查产物。",
    tags: ["free", "prompts-chat", "code", "review"],
    titleEn: "Actionable code review findings",
    titleZh: "可执行代码审查结论",
  }),
  codePromptItem({
    code: `SELECT customer_id, COUNT(*) AS orders
FROM orders
WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY customer_id
ORDER BY orders DESC
LIMIT 10;`,
    language: "sql",
    model: "gpt-5-mini",
    outputEn: "SQL answer",
    outputZh: "SQL 答案",
    prompt:
      "Act as a SQL terminal for an example commerce database with Products, Users, Orders, and Suppliers tables. When I type a query request, return one SQL block and a compact result table. Do not add unrelated explanation.",
    slug: "free-sql-terminal-commerce-query",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from the free SQL Terminal prompt and paired with a query artifact.",
    summaryZh: "改写自免费 SQL Terminal 提示词，并配有查询产物。",
    tags: ["free", "prompts-chat", "sql", "developer"],
    titleEn: "Commerce SQL terminal query",
    titleZh: "电商 SQL 终端查询",
  }),
  codePromptItem({
    code: `> console.log(["flatkey", "router"].join("-"))
flatkey-router`,
    language: "javascript",
    model: "gpt-5-mini",
    outputEn: "Console output",
    outputZh: "控制台输出",
    prompt:
      "Act as a JavaScript console. I will type JavaScript expressions or short snippets, and you will return only the console output inside one code block unless I explicitly ask for explanation.",
    slug: "free-javascript-console-session",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from the free JavaScript Console prompt and paired with a console transcript artifact.",
    summaryZh: "改写自免费 JavaScript Console 提示词，并配有控制台转录产物。",
    tags: ["free", "prompts-chat", "javascript", "developer"],
    titleEn: "JavaScript console session",
    titleZh: "JavaScript 控制台会话",
  }),
  codePromptItem({
    code: `contract Messenger {
  string public message;
  address public owner;
  uint256 public updateCount;

  modifier onlyOwner() {
    require(msg.sender == owner, "not owner");
    _;
  }
}`,
    language: "solidity",
    model: "gpt-5",
    outputEn: "Contract scaffold",
    outputZh: "合约骨架",
    prompt:
      "Act as an experienced Ethereum developer. Create a minimal Solidity smart contract for a public blockchain message that only the deployer can update. Include state variables, access control, update counting, and a short explanation of security considerations.",
    slug: "free-ethereum-messenger-contract",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from the free Ethereum Developer prompt and paired with a smart-contract scaffold.",
    summaryZh: "改写自免费 Ethereum Developer 提示词，并配有智能合约骨架。",
    tags: ["free", "prompts-chat", "ethereum", "developer"],
    titleEn: "Ethereum messenger contract",
    titleZh: "以太坊留言合约",
  }),
  textPromptItem({
    artifactBody:
      "UX plan\nNavigation: group setup, usage, billing, and support.\nPrototype: two task paths, key creation and spend review.\nValidation: first-click test, keyboard pass, and mobile breakpoint scan.",
    artifactTitle: "UX improvement plan",
    model: "claude-sonnet-5",
    outputEn: "UX plan",
    outputZh: "UX 方案",
    prompt:
      "Act as a UX/UI developer. Given product details and audience, propose a navigation structure, primary screens, interaction states, accessibility checks, and validation plan. Keep recommendations tied to the business goal.",
    slug: "free-ux-navigation-improvement-plan",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free UX/UI Developer prompt with a navigation improvement artifact.",
    summaryZh: "改写自免费 UX/UI Developer 提示词，并配有导航优化产物。",
    tags: ["free", "prompts-chat", "ux", "design"],
    titleEn: "UX navigation improvement plan",
    titleZh: "UX 导航优化方案",
  }),
  textPromptItem({
    artifactBody:
      "Ecommerce IA\nTop nav: Shop, Collections, Reviews, Support.\nProduct card: clear price, delivery promise, trust proof.\nCheckout: guest path, wallet options, and abandoned-cart recovery.",
    artifactTitle: "Web design plan",
    model: "gpt-5-mini",
    outputEn: "Design plan",
    outputZh: "设计方案",
    prompt:
      "Act as a web design consultant. For the provided organization and goal, recommend information architecture, layout, interaction patterns, trust signals, content sections, and measurable conversion improvements.",
    slug: "free-web-design-ecommerce-plan",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free web design consultant prompt and paired with an ecommerce IA artifact.",
    summaryZh: "改写自免费网站设计顾问提示词，并配有电商信息架构产物。",
    tags: ["free", "prompts-chat", "web-design", "commerce"],
    titleEn: "Ecommerce web design plan",
    titleZh: "电商网站设计方案",
  }),
  textPromptItem({
    artifactBody:
      "Accessibility findings\nKeyboard: filter tabs need visible focus.\nScreen reader: source cards need clearer labels.\nContrast: secondary gray text passes on white, fails on violet tint.\nFix first: focus ring and aria labels.",
    artifactTitle: "Accessibility audit",
    model: "gpt-5",
    outputEn: "Audit report",
    outputZh: "审计报告",
    prompt:
      "Act as a web accessibility auditor. Review a page or component against WCAG 2.2 and Section 508. Focus on keyboard navigation, screen reader labels, color contrast, focus order, motion, and actionable fixes.",
    slug: "free-accessibility-audit-checklist",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Accessibility Auditor prompt with a concise audit artifact.",
    summaryZh: "改写自免费无障碍审计提示词，并配有精简审计产物。",
    tags: ["free", "prompts-chat", "accessibility", "qa"],
    titleEn: "Accessibility audit checklist",
    titleZh: "无障碍审计清单",
  }),
  textPromptItem({
    artifactBody:
      "Security plan\nData: classify secrets, customer data, and logs.\nControls: encryption at rest, scoped API keys, rotation, rate limits.\nDetection: suspicious usage alerts and audit trails.\nReview: quarterly access recertification.",
    artifactTitle: "Security strategy",
    category: "agent",
    model: "claude-opus-4-8",
    outputEn: "Security plan",
    outputZh: "安全方案",
    prompt:
      "Act as a cybersecurity specialist. Given a system description, propose a practical data-protection strategy covering threat model, sensitive data, encryption, access control, logging, monitoring, incident response, and residual risks.",
    slug: "free-cybersecurity-data-protection-plan",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Cyber Security Specialist prompt with a security-plan artifact.",
    summaryZh: "改写自免费网络安全专家提示词，并配有数据保护方案产物。",
    tags: ["free", "prompts-chat", "security", "ops"],
    titleEn: "Cybersecurity data-protection plan",
    titleZh: "网络安全数据保护方案",
  }),
  textPromptItem({
    artifactBody:
      "DevRel review\nPackage: express.\nSignals: docs breadth, issue velocity, StackOverflow footprint, examples, migration guides.\nGap: add production deployment examples and troubleshooting recipes.\nAudience: backend beginners and framework migrators.",
    artifactTitle: "Developer relations review",
    model: "gpt-5",
    outputEn: "DevRel review",
    outputZh: "DevRel 评审",
    prompt:
      "Act as a Developer Relations consultant. Research a software package and its docs. Summarize adoption signals, documentation gaps, examples to add, competitor comparison, and concrete content recommendations. Say when data is unavailable.",
    slug: "free-devrel-package-documentation-review",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Developer Relations prompt with a package documentation review artifact.",
    summaryZh: "改写自免费 DevRel 提示词，并配有包文档评审产物。",
    tags: ["free", "prompts-chat", "developer", "docs"],
    titleEn: "Package documentation DevRel review",
    titleZh: "软件包文档 DevRel 评审",
  }),
  textPromptItem({
    artifactBody:
      "Paper Q&A\nConcept: retrieval-augmented generation.\nPurpose: ground model responses in external evidence.\nProcedure: retrieve, rank, synthesize, cite.\nRisk: stale retrieval and unsupported synthesis.",
    artifactTitle: "Research Q&A",
    model: "claude-sonnet-5",
    outputEn: "Research answer",
    outputZh: "研究回答",
    prompt:
      "Act as an expert LLM researcher. When I provide a paper, excerpt, or concept, answer my questions with the reason, procedure, purpose, limitations, and references when available. Clearly separate source facts from inference.",
    slug: "free-llm-researcher-paper-qa",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free LLM Researcher prompt and paired with a paper Q&A artifact.",
    summaryZh: "改写自免费 LLM Researcher 提示词，并配有论文问答产物。",
    tags: ["free", "prompts-chat", "research", "llm"],
    titleEn: "LLM researcher paper Q&A",
    titleZh: "LLM 研究员论文问答",
  }),
  textPromptItem({
    artifactBody:
      "Insight brief\nMetric: week-2 retention fell from 31% to 24%.\nSegment: new mobile users from paid social.\nHypothesis: onboarding prompt length creates drop-off.\nNext analysis: compare completion by device and acquisition channel.",
    artifactTitle: "Data insight brief",
    model: "gpt-5-mini",
    outputEn: "Analysis brief",
    outputZh: "分析简报",
    prompt:
      "Act as a data analyst. Given a dataset summary or table, identify useful patterns, anomalies, likely causes, business impact, and the next analysis to run. Return actionable recommendations and avoid unsupported certainty.",
    slug: "free-data-analyst-insight-brief",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Data Analyst prompt with a retention insight artifact.",
    summaryZh: "改写自免费数据分析师提示词，并配有留存洞察产物。",
    tags: ["free", "prompts-chat", "data", "analytics"],
    titleEn: "Data analyst insight brief",
    titleZh: "数据分析师洞察简报",
  }),
  textPromptItem({
    artifactBody:
      "Enhanced prompt\nWrite a source-grounded launch brief from five URLs. Separate facts, quotes, and inference. Include target customer, pricing change, distribution channel, and a counter-message. Keep the memo under 350 words.",
    artifactTitle: "Prompt rewrite",
    category: "agent",
    model: "gpt-5",
    outputEn: "Optimized prompt",
    outputZh: "优化提示词",
    prompt:
      "Act as a prompt enhancer. Rewrite the user's rough prompt into a precise, testable prompt with role, task, context, constraints, output format, and quality checks. Put the rewritten prompt first, then list the main improvements.",
    slug: "free-prompt-enhancer-rewrite-flow",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Prompt Enhancer prompt and paired with an optimized prompt artifact.",
    summaryZh: "改写自免费 Prompt Enhancer 提示词，并配有优化后提示词产物。",
    tags: ["free", "prompts-chat", "prompt-engineering", "agent"],
    titleEn: "Prompt enhancer rewrite flow",
    titleZh: "提示词增强改写流程",
  }),
  textPromptItem({
    artifactBody:
      "SEO plan\nKeyword: best AI prompt library.\nIntent: discovery and comparison.\nPage structure: H1, source methodology, category pages, prompt examples, FAQ.\nRisk: avoid fake freshness; show captured dates.",
    artifactTitle: "SEO action plan",
    model: "gpt-5-mini",
    outputEn: "SEO plan",
    outputZh: "SEO 方案",
    prompt:
      "Act as an SEO specialist. For a keyword or page scenario, return search intent, page structure, title options, internal links, FAQ ideas, measurement plan, and risks. Stay focused on SEO rather than general marketing advice.",
    slug: "free-seo-specialist-page-plan",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free SEO Specialist prompt with a page optimization artifact.",
    summaryZh: "改写自免费 SEO Specialist 提示词，并配有页面优化产物。",
    tags: ["free", "prompts-chat", "seo", "growth"],
    titleEn: "SEO specialist page plan",
    titleZh: "SEO 页面方案",
  }),
  textPromptItem({
    artifactBody:
      "Campaign calendar\nWeek 1: founder story + product proof.\nWeek 2: customer objection series.\nWeek 3: short demo clips.\nWeek 4: sourced case study and retargeting hooks.\nMeasurement: saves, replies, demos booked.",
    artifactTitle: "Social campaign plan",
    model: "gemini-3.6-flash",
    outputEn: "Campaign plan",
    outputZh: "活动计划",
    prompt:
      "Act as a social media manager. Create a campaign plan for the provided organization, audience, and goal. Include platform focus, content pillars, posting cadence, engagement workflow, measurement metrics, and a weekly calendar.",
    slug: "free-social-media-campaign-calendar",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Social Media Manager prompt with a campaign calendar artifact.",
    summaryZh: "改写自免费社媒经理提示词，并配有活动日历产物。",
    tags: ["free", "prompts-chat", "social", "growth"],
    titleEn: "Social media campaign calendar",
    titleZh: "社媒活动日历",
  }),
  textPromptItem({
    artifactBody:
      "Recruiting plan\nRole: API partnerships AE.\nSignals: AI infra sales, developer platform deals, usage-based pricing.\nChannels: LinkedIn search, community posts, referral asks.\nScreen: demo ownership and technical buyer mapping.",
    artifactTitle: "Recruiting sourcing plan",
    model: "claude-sonnet-5",
    outputEn: "Sourcing plan",
    outputZh: "招聘寻源方案",
    prompt:
      "Act as a recruiter. Given a role, market, and must-have criteria, build a sourcing plan with target profiles, search strings, outreach message, screening questions, and a shortlist scoring rubric.",
    slug: "free-recruiter-sourcing-plan",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Recruiter prompt with a candidate sourcing artifact.",
    summaryZh: "改写自免费招聘提示词，并配有候选人寻源产物。",
    tags: ["free", "prompts-chat", "recruiting", "sales"],
    titleEn: "Recruiter sourcing plan",
    titleZh: "招聘寻源方案",
  }),
  textPromptItem({
    artifactBody:
      "Nearby itinerary\nAnchor: Istanbul Beyoglu.\nTheme: museums only.\nRoute: Pera Museum, Istanbul Modern, Galata Mevlevi Museum.\nNotes: walking order, opening-hour check, cafe stop, and transit backup.",
    artifactTitle: "Travel itinerary",
    model: "gpt-5-mini",
    outputEn: "Itinerary",
    outputZh: "行程方案",
    prompt:
      "Act as a travel guide. When I provide a location and preferred place type, suggest nearby places, explain why each fits, propose a practical route, and include timing, transit, and backup considerations.",
    slug: "free-travel-guide-nearby-itinerary",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Travel Guide prompt and paired with a nearby itinerary artifact.",
    summaryZh: "改写自免费旅行向导提示词，并配有附近行程产物。",
    tags: ["free", "prompts-chat", "travel", "planning"],
    titleEn: "Nearby travel itinerary",
    titleZh: "附近旅行行程",
  }),
  textPromptItem({
    artifactBody:
      "Book summary structure\n1. Core thesis\n2. Major concepts\n3. Examples and applications\n4. Key takeaways\n5. Questions for discussion\n6. What to verify against the original text",
    artifactTitle: "Book summary framework",
    model: "gpt-5-mini",
    outputEn: "Summary framework",
    outputZh: "摘要框架",
    prompt:
      "Act as a book summarizer. Summarize the provided book or chapter with major topics, examples, applications, key takeaways, and discussion questions. Keep the structure easy to scan and mark uncertain details.",
    slug: "free-book-summary-framework",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free Book Summarizer prompt with a reusable summary framework.",
    summaryZh: "改写自免费图书摘要提示词，并配有可复用摘要框架。",
    tags: ["free", "prompts-chat", "writing", "summary"],
    titleEn: "Book summary framework",
    titleZh: "图书摘要框架",
  }),
  textPromptItem({
    artifactBody:
      "Sales funnel\nURL input: product homepage.\nStages: visitor, lead, activated trial, paid account.\nAssets: landing page, demo email, objection FAQ, retargeting ad.\nMeasurement: page CVR, demo rate, activation, paid conversion.",
    artifactTitle: "Sales funnel plan",
    model: "gpt-5",
    outputEn: "Funnel plan",
    outputZh: "漏斗方案",
    prompt:
      "Act as a sales funnel architect. Given a product URL and target customer, design a funnel with stages, page sections, lead magnet, email sequence, objection handling, retargeting hooks, and conversion metrics.",
    slug: "free-sales-funnel-from-url",
    source: sourceSignal({
      label: "prompts.chat CC0 dataset (117k+ rows)",
      platform: "GitHub",
      url: "https://github.com/f/prompts.chat/blob/main/prompts.csv",
    }),
    summaryEn: "Adapted from a free sales-funnel prompt and paired with a funnel-plan artifact.",
    summaryZh: "改写自免费销售漏斗提示词，并配有漏斗方案产物。",
    tags: ["free", "prompts-chat", "sales", "growth"],
    titleEn: "Sales funnel from URL",
    titleZh: "基于 URL 的销售漏斗",
  }),
  textPromptItem({
    artifactBody:
      "Code hallucination guard\nScope: use one function at a time.\nEvidence: paste official docs or examples for niche libraries.\nConstraint: avoid undocumented packages and unsupported APIs.\nVerification: run the snippet, paste errors back, and revise only the failing part.",
    artifactTitle: "Coding prompt guardrail",
    category: "agent",
    model: "gpt-5",
    outputEn: "Guardrail prompt",
    outputZh: "防幻觉提示词",
    prompt:
      "Write code one small unit at a time. Before using a library API, state whether it comes from supplied docs, an official package page, or an assumption. Use only documented APIs. If context is missing, ask for the relevant docs or propose a standard-library alternative. Return runnable code, verification steps, and likely failure points.",
    slug: "social-hn-code-hallucination-guard",
    source: sourceSignal({
      label: "Hacker News discussion: avoiding code hallucinations",
      platform: "Social",
      url: "https://news.ycombinator.com/item?id=40479500",
    }),
    summaryEn: "A coding-agent prompt rewritten from public HN discussion signals about avoiding fabricated libraries and APIs.",
    summaryZh: "根据 HN 公开讨论中关于避免编造库和 API 的经验改写成编程 Agent 提示词。",
    tags: ["free", "social", "hacker-news", "code", "hallucination"],
    titleEn: "Code hallucination guard",
    titleZh: "代码幻觉防护提示词",
  }),
  textPromptItem({
    artifactBody:
      "Prompt experiment matrix\nMetric: answer correctness and edit distance.\nVariants: direct prompt, example-driven prompt, structured prompt.\nRun: isolate context, score outputs, keep the shortest prompt that passes.\nDecision: iterate rather than over-template.",
    artifactTitle: "Prompt test plan",
    category: "agent",
    model: "claude-sonnet-5",
    outputEn: "Experiment plan",
    outputZh: "实验计划",
    prompt:
      "Design a prompt experiment for a task. Create three prompt variants, define the success metric, isolate test context, specify how to score outputs, and recommend which variant to keep. Prefer simple direct instructions unless structured output measurably improves results.",
    slug: "social-hn-prompt-experiment-matrix",
    source: sourceSignal({
      label: "Hacker News discussion: learning prompt engineering",
      platform: "Social",
      url: "https://news.ycombinator.com/item?id=38588127",
    }),
    summaryEn: "A prompt-testing workflow derived from public HN discussion about measuring what works instead of over-templating.",
    summaryZh: "根据 HN 公开讨论中“用指标验证提示词效果”的观点改写成实验工作流。",
    tags: ["free", "social", "hacker-news", "prompt-engineering", "evaluation"],
    titleEn: "Prompt experiment matrix",
    titleZh: "提示词实验矩阵",
  }),
  textPromptItem({
    artifactBody:
      "Prompt library routing note\nContext cards: tech stack, writing style, database schema, product constraints.\nRouting rule: apply the smallest relevant context card, show what was applied, and let the user reject it.\nRisk: hidden context can surprise users.",
    artifactTitle: "Prompt routing note",
    category: "agent",
    model: "gpt-5",
    outputEn: "Routing note",
    outputZh: "路由说明",
    prompt:
      "Create a prompt-library routing policy for an AI workspace. Define reusable context cards, matching rules, user visibility, approval controls, and failure modes. The policy should help the assistant apply relevant prompts automatically without hiding important context from the user.",
    slug: "social-hn-prompt-library-routing-policy",
    source: sourceSignal({
      label: "Hacker News discussion: prompt libraries and non-linear AI UI",
      platform: "Social",
      url: "https://news.ycombinator.com/item?id=40300126",
    }),
    summaryEn: "A routing-policy prompt adapted from public HN discussion about prompt libraries, BYO keys, and non-linear AI workflows.",
    summaryZh: "根据 HN 关于提示词库、BYO key 和非线性 AI 工作流的公开讨论改写成路由策略提示词。",
    tags: ["free", "social", "hacker-news", "prompt-library", "agent"],
    titleEn: "Prompt library routing policy",
    titleZh: "提示词库路由策略",
  }),
];

const generatedPromptItems: PromptItem[] = [];
const curatedDatasetPromptItems = staticPromptItems.slice(0, 8);
const showcasePromptItems = staticPromptItems.slice(8);

export const playgroundPromptItems: PromptItem[] = [
  ...showcasePromptItems,
  ...curatedDatasetPromptItems,
  ...generatedPromptItems,
];

function hasArtifact(item: PromptItem): boolean {
  if (item.artifact.kind === "image") return Boolean(item.artifact.url);
  if (item.artifact.kind === "video") return Boolean(item.artifact.url);
  if (item.artifact.kind === "text") return Boolean(item.artifact.body);
  if (item.artifact.kind === "code") return Boolean(item.artifact.code);
  if (item.artifact.kind === "storyboard") return item.artifact.frames.length > 0;
  return false;
}

export function getCliMediaPromptItems(category?: "image" | "video"): PromptItem[] {
  const bySlug = new Map<string, PromptItem>();

  for (const item of staticPromptItems.filter(hasArtifact)) {
    if (category && item.category !== category) continue;
    bySlug.set(item.slug, item);
  }

  return Array.from(bySlug.values()).sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
}

export function getCliMediaPromptItem(category: "image" | "video", slug: string): PromptItem | undefined {
  return getCliMediaPromptItems(category).find((item) => item.slug === slug);
}
