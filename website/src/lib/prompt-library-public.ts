import { type Locale, withIdFallback } from "./locales";
import { PROMPTS_PATH } from "./prompt-library-path";
export { PROMPTS_PATH } from "./prompt-library-path";

export type PromptMediaType = "image" | "video" | "audio";
export type PromptCategory = "photography" | "infographic" | "character" | "editing";
export type PromptPageType =
  | "landing-page"
  | "product-page"
  | "ad-creative"
  | "infographic-page"
  | "character-reference"
  | "editing-workflow";

export type PromptLibrarySource = {
  label: string;
  repository: "ZeroLu/awesome-gpt-image";
  repositoryUrl: string;
  section: string;
  url: string;
};

export type PromptLibraryExample = {
  category: PromptCategory;
  mediaType: PromptMediaType;
  model: string;
  pageType: PromptPageType;
  previewImage: {
    alt: string;
    src: string;
  };
  prompt: string;
  ratio: string;
  slug: string;
  source: PromptLibrarySource;
  tags: string[];
  title: string;
  updatedAt: string;
};

export type PromptLibraryPageCopy = {
  allMediaTypes: string;
  allCategories: string;
  allModels: string;
  allPageTypes: string;
  categories: Record<PromptCategory, string>;
  copiedPrompt: string;
  copyPrompt: string;
  emptyBody: string;
  emptyTitle: string;
  exampleCountLabel: string;
  featuredLabel: string;
  filterLabel: string;
  heroBadge: string;
  heroBody: string;
  heroTitle: string;
  metaDescription: string;
  metaTitle: string;
  modelFilterLabel: string;
  modelBrowseTitle: string;
  modelBrowseBody: string;
  modelLabel: string;
  mediaBrowseTitle: string;
  mediaBrowseBody: string;
  mediaTypeFilterLabel: string;
  mediaTypeLabel: string;
  mediaTypes: Record<PromptMediaType, string>;
  mediaTypeDescriptions: Record<PromptMediaType, string>;
  pageTypeFilterLabel: string;
  pageTypeLabel: string;
  pageTypes: Record<PromptPageType, string>;
  promptLabel: string;
  repositoryLabel: string;
  searchPlaceholder: string;
  sourceLabel: string;
  sourceNote: string;
  weeklyHotTitle: string;
  weeklyHotBody: string;
  viewSource: string;
};

export type PromptLibraryFilters = {
  category?: PromptCategory | "all";
  mediaType?: PromptMediaType | "all";
  model?: string | "all";
  pageType?: PromptPageType | "all";
  query?: string;
};

export type PromptLibraryMediaSummary = {
  count: number;
  description: string;
  href: string;
  modelCount: number;
  type: PromptMediaType;
};

export type PromptLibraryModelSummary = {
  count: number;
  displayName: string;
  href: string;
  mediaType: PromptMediaType;
  slug: string;
};

const REPOSITORY = "ZeroLu/awesome-gpt-image" as const;
const REPOSITORY_URL = "https://github.com/ZeroLu/awesome-gpt-image";
const UPDATED_AT = "2026-08-18";

const MEDIA_TYPES: PromptMediaType[] = ["image", "video", "audio"];

const EMPTY_AUDIO_MODELS = [
  {
    displayName: "sonilo-video-to-music",
    mediaType: "audio" as const,
    slug: "sonilo-video-to-music",
  },
];

const MODEL_DISPLAY_NAMES: Record<string, string> = {
  "gpt-image-2": "GPT Image 2",
  "seedance-2.0": "Seedance 2.0",
  "seedance-2.5": "Seedance 2.5",
  "veo-3.1-fast-generate-preview": "Veo 3.1 Fast",
  "sonilo-video-to-music": "Sonilo Video to Music",
};

const MODEL_MEDIA_TYPES: Record<string, PromptMediaType> = {
  "gpt-image-2": "image",
  "seedance-2.0": "video",
  "seedance-2.5": "video",
  "veo-3.1-fast-generate-preview": "video",
  "sonilo-video-to-music": "audio",
};

const categoryLabels: Record<Locale, Record<PromptCategory, string>> = withIdFallback({
  en: {
    photography: "Photography",
    infographic: "Infographics",
    character: "Character",
    editing: "Editing",
  },
  zh: {
    photography: "摄影",
    infographic: "信息图",
    character: "角色",
    editing: "图像编辑",
  },
  es: {
    photography: "Fotografía",
    infographic: "Infografías",
    character: "Personajes",
    editing: "Edición",
  },
  fr: {
    photography: "Photographie",
    infographic: "Infographies",
    character: "Personnage",
    editing: "Retouche",
  },
  pt: {
    photography: "Fotografia",
    infographic: "Infográficos",
    character: "Personagem",
    editing: "Edição",
  },
  ru: {
    photography: "Фотография",
    infographic: "Инфографика",
    character: "Персонажи",
    editing: "Обработка",
  },
  ja: {
    photography: "写真",
    infographic: "インフォグラフィック",
    character: "キャラクター",
    editing: "編集",
  },
  vi: {
    photography: "Nhiếp ảnh",
    infographic: "Infographic",
    character: "Nhân vật",
    editing: "Chỉnh sửa",
  },
  de: {
    photography: "Fotografie",
    infographic: "Infografiken",
    character: "Figur",
    editing: "Bearbeitung",
  },
  id: {
    photography: "Fotografi",
    infographic: "Infografik",
    character: "Karakter",
    editing: "Penyuntingan",
  },
});

const pageTypeLabels: Record<Locale, Record<PromptPageType, string>> = withIdFallback({
  en: {
    "landing-page": "Landing page",
    "product-page": "Product page",
    "ad-creative": "Ad creative",
    "infographic-page": "Infographic page",
    "character-reference": "Character sheet",
    "editing-workflow": "Editing workflow",
  },
  zh: {
    "landing-page": "落地页",
    "product-page": "产品页",
    "ad-creative": "广告创意",
    "infographic-page": "信息图页面",
    "character-reference": "角色设定页",
    "editing-workflow": "编辑工作流",
  },
  es: {
    "landing-page": "Landing page",
    "product-page": "Página de producto",
    "ad-creative": "Creatividad publicitaria",
    "infographic-page": "Página infográfica",
    "character-reference": "Ficha de personaje",
    "editing-workflow": "Flujo de edición",
  },
  fr: {
    "landing-page": "Page d'atterrissage",
    "product-page": "Page produit",
    "ad-creative": "Créa publicitaire",
    "infographic-page": "Page infographique",
    "character-reference": "Fiche personnage",
    "editing-workflow": "Flux de retouche",
  },
  pt: {
    "landing-page": "Landing page",
    "product-page": "Página de produto",
    "ad-creative": "Criativo de anúncio",
    "infographic-page": "Página infográfica",
    "character-reference": "Ficha de personagem",
    "editing-workflow": "Fluxo de edição",
  },
  ru: {
    "landing-page": "Лендинг",
    "product-page": "Страница продукта",
    "ad-creative": "Рекламный креатив",
    "infographic-page": "Инфографика",
    "character-reference": "Лист персонажа",
    "editing-workflow": "Редактирование",
  },
  ja: {
    "landing-page": "ランディングページ",
    "product-page": "商品ページ",
    "ad-creative": "広告クリエイティブ",
    "infographic-page": "インフォグラフィックページ",
    "character-reference": "キャラクター設定",
    "editing-workflow": "編集ワークフロー",
  },
  vi: {
    "landing-page": "Trang đích",
    "product-page": "Trang sản phẩm",
    "ad-creative": "Mẫu quảng cáo",
    "infographic-page": "Trang infographic",
    "character-reference": "Hồ sơ nhân vật",
    "editing-workflow": "Quy trình chỉnh sửa",
  },
  de: {
    "landing-page": "Landingpage",
    "product-page": "Produktseite",
    "ad-creative": "Anzeigenmotiv",
    "infographic-page": "Infografik-Seite",
    "character-reference": "Figuren-Sheet",
    "editing-workflow": "Bearbeitungsworkflow",
  },
  id: {
    "landing-page": "Landing page",
    "product-page": "Halaman produk",
    "ad-creative": "Materi iklan",
    "infographic-page": "Halaman infografik",
    "character-reference": "Lembar karakter",
    "editing-workflow": "Alur penyuntingan",
  },
});

const mediaTypeLabels: Record<Locale, Record<PromptMediaType, string>> = withIdFallback({
  en: {
    image: "Image",
    video: "Video",
    audio: "Audio",
  },
  zh: {
    image: "图片",
    video: "视频",
    audio: "音频",
  },
  es: {
    image: "Imagen",
    video: "Video",
    audio: "Audio",
  },
  fr: {
    image: "Image",
    video: "Vidéo",
    audio: "Audio",
  },
  pt: {
    image: "Imagem",
    video: "Vídeo",
    audio: "Áudio",
  },
  ru: {
    image: "Изображение",
    video: "Видео",
    audio: "Аудио",
  },
  ja: {
    image: "画像",
    video: "動画",
    audio: "音声",
  },
  vi: {
    image: "Hình ảnh",
    video: "Video",
    audio: "Âm thanh",
  },
  de: {
    image: "Bild",
    video: "Video",
    audio: "Audio",
  },
  id: {
    image: "Gambar",
    video: "Video",
    audio: "Audio",
  },
});

const mediaTypeDescriptions: Record<Locale, Record<PromptMediaType, string>> = withIdFallback({
  en: {
    image: "Photo, product, poster, character sheet, and layout prompts for image models.",
    video: "Text-to-video and image-to-video prompts for clips, ads, and motion tests.",
    audio: "Music, narration, and video-to-music prompts ready for the future API feed.",
  },
  zh: {
    image: "照片、产品、海报、角色设定和版式类图片模型提示词。",
    video: "面向短片、广告和动态测试的文生视频、图生视频提示词。",
    audio: "音乐、旁白和视频转音乐提示词，先留好接口数据位。",
  },
  es: {
    image: "Prompts para foto, producto, póster, personajes y layouts de modelos de imagen.",
    video: "Prompts de texto a video e imagen a video para clips, anuncios y pruebas de movimiento.",
    audio: "Prompts de música, narración y video a música preparados para el futuro feed API.",
  },
  fr: {
    image: "Prompts photo, produit, affiche, fiche personnage et mise en page pour modèles image.",
    video: "Prompts texte-vers-vidéo et image-vers-vidéo pour clips, pubs et tests de mouvement.",
    audio: "Prompts musique, narration et vidéo-vers-musique prêts pour le futur flux API.",
  },
  pt: {
    image: "Prompts de foto, produto, pôster, personagem e layout para modelos de imagem.",
    video: "Prompts de texto para vídeo e imagem para vídeo para clipes, anúncios e testes.",
    audio: "Prompts de música, narração e vídeo para música preparados para o feed API futuro.",
  },
  ru: {
    image: "Prompts для фото, продукта, постера, персонажей и layout у image-моделей.",
    video: "Prompts text-to-video и image-to-video для клипов, рекламы и motion-тестов.",
    audio: "Prompts для музыки, narration и video-to-music под будущий API-feed.",
  },
  ja: {
    image: "写真、商品、ポスター、キャラクター設定、レイアウト向け画像モデルプロンプト。",
    video: "クリップ、広告、モーション検証向けのテキスト/画像から動画プロンプト。",
    audio: "音楽、ナレーション、動画から音楽へのプロンプト。今後の API フィード用です。",
  },
  vi: {
    image: "Prompt ảnh cho ảnh chụp, sản phẩm, poster, nhân vật và bố cục.",
    video: "Prompt text-to-video và image-to-video cho clip, quảng cáo và thử nghiệm chuyển động.",
    audio: "Prompt nhạc, thuyết minh và video-to-music sẵn cho feed API sau này.",
  },
  de: {
    image: "Prompts für Foto, Produkt, Poster, Character Sheets und Layouts bei Bildmodellen.",
    video: "Text-to-Video- und Image-to-Video-Prompts für Clips, Ads und Motion-Tests.",
    audio: "Musik-, Narration- und Video-to-Music-Prompts für den künftigen API-Feed.",
  },
  id: {
    image: "Prompt gambar untuk foto, produk, poster, karakter, dan layout.",
    video: "Prompt text-to-video dan image-to-video untuk klip, iklan, dan uji gerak.",
    audio: "Prompt musik, narasi, dan video-to-music untuk feed API mendatang.",
  },
});

type PromptLibraryPageCopyBase = Omit<PromptLibraryPageCopy, "categories" | "pageTypes">;

const promptLibraryPageCopyBase: Record<Locale, PromptLibraryPageCopyBase> = withIdFallback({
  en: {
    allMediaTypes: "All media",
    allCategories: "All",
    allModels: "All models",
    allPageTypes: "All page types",
    copiedPrompt: "Copied",
    copyPrompt: "Copy prompt",
    emptyBody: "Try another keyword or clear the category filter.",
    emptyTitle: "No matching prompts",
    exampleCountLabel: "example prompts",
    featuredLabel: "Featured example",
    filterLabel: "Filter",
    mediaBrowseBody:
      "Browse image, video, and audio prompt collections by type, then jump into the individual model pages.",
    mediaBrowseTitle: "Browse by media",
    heroBadge: "Repo-seeded sample set",
    heroBody:
      "A small prompt library seeded from ZeroLu/awesome-gpt-image and restyled to match Flatkey's own homepage language: purple-black accents, dense cards, and quick source traceability until the API feed arrives.",
    heroTitle: "Copy the prompt. Keep the source. Ship the image.",
    metaDescription:
      "Browse a small Flatkey prompt library seeded from ZeroLu/awesome-gpt-image, with model, category, source section, and one-click prompt copy.",
    metaTitle: "Flatkey Prompts — GPT Image 2 prompt examples",
    modelBrowseBody:
      "Each model gets its own page so the prompt set can grow into a real per-model reference later.",
    modelBrowseTitle: "Browse by model",
    modelFilterLabel: "Model",
    modelLabel: "Model",
    mediaTypeFilterLabel: "Media type",
    mediaTypeLabel: "Media type",
    mediaTypes: mediaTypeLabels.en,
    mediaTypeDescriptions: mediaTypeDescriptions.en,
    pageTypeFilterLabel: "Page type",
    pageTypeLabel: "Page type",
    promptLabel: "Prompt",
    repositoryLabel: "GitHub README",
    searchPlaceholder: "Search prompts, tags, or source...",
    sourceLabel: "Source",
    sourceNote: "These examples keep the repo source, section, prompt text, and image references visible so the future API can drop in without changing the UI.",
    weeklyHotBody:
      "The most viewed prompts in each media group, arranged as a weekly hot strip.",
    weeklyHotTitle: "Weekly hot",
    viewSource: "View source",
  },
  zh: {
    allMediaTypes: "全部媒介",
    allCategories: "全部",
    allModels: "全部模型",
    allPageTypes: "全部页面类型",
    copiedPrompt: "已复制",
    copyPrompt: "复制提示词",
    emptyBody: "换个关键词，或清空分类筛选再试。",
    emptyTitle: "没有匹配的提示词",
    exampleCountLabel: "条示例提示词",
    featuredLabel: "精选示例",
    filterLabel: "筛选",
    mediaBrowseBody:
      "先按图片、视频、音频浏览，再进入对应的单独模型页。",
    mediaBrowseTitle: "按媒介浏览",
    heroBadge: "GitHub 种子样本",
    heroBody:
      "这个提示词库先用 ZeroLu/awesome-gpt-image 里现成的内容起步，并改成更接近 Flatkey 首页的语言：紫黑强调、密信息卡片、清晰的来源追踪，等 API 进来后也能直接替换。",
    heroTitle: "复制提示词，保留来源，直接出图。",
    metaDescription:
      "浏览一个由 ZeroLu/awesome-gpt-image 种子的 Flatkey 提示词库，包含模型、分类、来源分区与一键复制。",
    metaTitle: "Flatkey Prompts — GPT Image 2 提示词示例",
    modelBrowseBody:
      "每个模型都有自己的页面，后面数据继续扩展时也能直接挂上去。",
    modelBrowseTitle: "按模型浏览",
    modelFilterLabel: "模型",
    modelLabel: "模型",
    mediaTypeFilterLabel: "媒介类型",
    mediaTypeLabel: "媒介类型",
    mediaTypes: mediaTypeLabels.zh,
    mediaTypeDescriptions: mediaTypeDescriptions.zh,
    pageTypeFilterLabel: "页面类型",
    pageTypeLabel: "页面类型",
    promptLabel: "提示词",
    repositoryLabel: "GitHub README",
    searchPlaceholder: "搜索提示词、标签或来源...",
    sourceLabel: "来源",
    sourceNote: "这些示例保留仓库来源、分区、提示词正文和图片引用，后续 API 接入时 UI 不用重做。",
    weeklyHotBody: "每个媒介里当前最热的几个提示词，先做成一条热榜带。",
    weeklyHotTitle: "每周最热",
    viewSource: "查看来源",
  },
  es: {
    allMediaTypes: "Todos los medios",
    allCategories: "Todo",
    allModels: "Todos los modelos",
    allPageTypes: "Todos los tipos de página",
    copiedPrompt: "Copiado",
    copyPrompt: "Copiar prompt",
    emptyBody: "Prueba otra palabra clave o limpia el filtro.",
    emptyTitle: "No hay prompts coincidentes",
    exampleCountLabel: "prompts de ejemplo",
    featuredLabel: "Ejemplo destacado",
    filterLabel: "Filtro",
    mediaBrowseBody:
      "Explora colecciones de prompts de imagen, video y audio por tipo, luego entra a las páginas individuales de cada modelo.",
    mediaBrowseTitle: "Explorar por medio",
    heroBadge: "Muestra semilla desde GitHub",
    heroBody:
      "Una biblioteca pequeña de prompts sembrada desde ZeroLu/awesome-gpt-image y reajustada al lenguaje visual de la página principal de Flatkey: acentos morado-negro, tarjetas densas y trazabilidad clara de la fuente hasta que llegue el feed API.",
    heroTitle: "Copia el prompt. Conserva la fuente. Entrega la imagen.",
    metaDescription:
      "Explora una pequeña biblioteca de prompts de Flatkey sembrada desde ZeroLu/awesome-gpt-image, con modelo, categoría, sección de origen y copia en un clic.",
    metaTitle: "Flatkey Prompts — ejemplos de prompts GPT Image 2",
    modelBrowseBody:
      "Cada modelo tiene su propia página para que el conjunto de prompts pueda crecer como referencia real por modelo.",
    modelBrowseTitle: "Explorar por modelo",
    modelFilterLabel: "Modelo",
    modelLabel: "Modelo",
    mediaTypeFilterLabel: "Tipo de medio",
    mediaTypeLabel: "Tipo de medio",
    mediaTypes: mediaTypeLabels.es,
    mediaTypeDescriptions: mediaTypeDescriptions.es,
    pageTypeFilterLabel: "Tipo de página",
    pageTypeLabel: "Tipo de página",
    promptLabel: "Prompt",
    repositoryLabel: "README de GitHub",
    searchPlaceholder: "Buscar prompts, etiquetas o fuente...",
    sourceLabel: "Fuente",
    sourceNote: "Estos ejemplos conservan la fuente del repositorio, la sección, el texto del prompt y la referencia de imagen para que la API futura encaje sin rehacer la UI.",
    weeklyHotBody: "Los prompts más vistos de cada grupo de medios, organizados como una franja semanal.",
    weeklyHotTitle: "Más populares de la semana",
    viewSource: "Ver fuente",
  },
  fr: {
    allMediaTypes: "Tous les médias",
    allCategories: "Tout",
    allModels: "Tous les modèles",
    allPageTypes: "Tous les types de page",
    copiedPrompt: "Copié",
    copyPrompt: "Copier le prompt",
    emptyBody: "Essayez un autre mot-clé ou réinitialisez le filtre.",
    emptyTitle: "Aucun prompt correspondant",
    exampleCountLabel: "prompts exemples",
    featuredLabel: "Exemple en avant",
    filterLabel: "Filtre",
    mediaBrowseBody:
      "Parcourez les collections de prompts image, vidéo et audio par type, puis ouvrez chaque page de modèle.",
    mediaBrowseTitle: "Parcourir par média",
    heroBadge: "Échantillons issus du dépôt",
    heroBody:
      "Une petite bibliothèque de prompts amorcée depuis ZeroLu/awesome-gpt-image et remise au goût visuel de la page d'accueil Flatkey : accents violet-noir, cartes denses et traçabilité claire de la source en attendant le flux API.",
    heroTitle: "Copiez le prompt. Gardez la source. Livrez l'image.",
    metaDescription:
      "Parcourez une petite bibliothèque de prompts Flatkey amorcée depuis ZeroLu/awesome-gpt-image, avec modèle, catégorie, section source et copie en un clic.",
    metaTitle: "Flatkey Prompts — exemples de prompts GPT Image 2",
    modelBrowseBody:
      "Chaque modèle aura sa propre page pour que la bibliothèque puisse grandir comme une vraie référence par modèle.",
    modelBrowseTitle: "Parcourir par modèle",
    modelFilterLabel: "Modèle",
    modelLabel: "Modèle",
    mediaTypeFilterLabel: "Type de média",
    mediaTypeLabel: "Type de média",
    mediaTypes: mediaTypeLabels.fr,
    mediaTypeDescriptions: mediaTypeDescriptions.fr,
    pageTypeFilterLabel: "Type de page",
    pageTypeLabel: "Type de page",
    promptLabel: "Prompt",
    repositoryLabel: "README GitHub",
    searchPlaceholder: "Rechercher prompts, tags ou source...",
    sourceLabel: "Source",
    sourceNote: "Ces exemples conservent la source du dépôt, la section, le texte du prompt et la référence d'image pour que l'API future se branche sans refaire l'UI.",
    weeklyHotBody: "Les prompts les plus vus de chaque groupe média, rangés en bande hebdo.",
    weeklyHotTitle: "Les plus chauds de la semaine",
    viewSource: "Voir la source",
  },
  pt: {
    allMediaTypes: "Todos os meios",
    allCategories: "Tudo",
    allModels: "Todos os modelos",
    allPageTypes: "Todos os tipos de página",
    copiedPrompt: "Copiado",
    copyPrompt: "Copiar prompt",
    emptyBody: "Tente outra palavra-chave ou limpe o filtro.",
    emptyTitle: "Nenhum prompt encontrado",
    exampleCountLabel: "prompts de exemplo",
    featuredLabel: "Exemplo em destaque",
    filterLabel: "Filtro",
    mediaBrowseBody:
      "Navegue pelas coleções de prompts de imagem, vídeo e áudio por tipo e depois entre nas páginas individuais de cada modelo.",
    mediaBrowseTitle: "Navegar por meio",
    heroBadge: "Amostras iniciadas no GitHub",
    heroBody:
      "Uma biblioteca pequena de prompts iniciada em ZeroLu/awesome-gpt-image e redesenhada para falar a mesma língua visual da home da Flatkey: acentos roxo-preto, cartões densos e rastreamento claro da fonte até o feed API chegar.",
    heroTitle: "Copie o prompt. Guarde a fonte. Entregue a imagem.",
    metaDescription:
      "Explore uma pequena biblioteca de prompts da Flatkey iniciada em ZeroLu/awesome-gpt-image, com modelo, categoria, seção de origem e cópia em um clique.",
    metaTitle: "Flatkey Prompts — exemplos de prompts GPT Image 2",
    modelBrowseBody:
      "Cada modelo terá sua própria página para a biblioteca crescer como referência real por modelo.",
    modelBrowseTitle: "Navegar por modelo",
    modelFilterLabel: "Modelo",
    modelLabel: "Modelo",
    mediaTypeFilterLabel: "Tipo de meio",
    mediaTypeLabel: "Tipo de meio",
    mediaTypes: mediaTypeLabels.pt,
    mediaTypeDescriptions: mediaTypeDescriptions.pt,
    pageTypeFilterLabel: "Tipo de página",
    pageTypeLabel: "Tipo de página",
    promptLabel: "Prompt",
    repositoryLabel: "README do GitHub",
    searchPlaceholder: "Buscar prompts, tags ou fonte...",
    sourceLabel: "Fonte",
    sourceNote: "Esses exemplos mantêm a fonte do repositório, a seção, o texto do prompt e a referência da imagem visíveis para que a API futura entre sem refazer a UI.",
    weeklyHotBody: "Os prompts mais vistos de cada grupo de mídia, organizados em uma faixa semanal.",
    weeklyHotTitle: "Mais quentes da semana",
    viewSource: "Ver fonte",
  },
  ru: {
    allMediaTypes: "Все медиа",
    allCategories: "Все",
    allModels: "Все модели",
    allPageTypes: "Все типы страниц",
    copiedPrompt: "Скопировано",
    copyPrompt: "Скопировать prompt",
    emptyBody: "Попробуйте другой запрос или сбросьте фильтр.",
    emptyTitle: "Prompt не найден",
    exampleCountLabel: "примеров prompt",
    featuredLabel: "Избранный пример",
    filterLabel: "Фильтр",
    mediaBrowseBody:
      "Просматривайте коллекции image, video и audio prompt'ов по типам, а затем переходите на отдельные страницы моделей.",
    mediaBrowseTitle: "Просмотр по медиа",
    heroBadge: "Образцы из репозитория",
    heroBody:
      "Небольшая библиотека prompt'ов, собранная из ZeroLu/awesome-gpt-image и приведённая к языку главной страницы Flatkey: фиолетово-чёрные акценты, плотные карточки и понятная трассировка источника до появления API-ленты.",
    heroTitle: "Скопируйте prompt. Сохраните источник. Выпустите изображение.",
    metaDescription:
      "Смотрите небольшую библиотеку prompt'ов Flatkey, собранную из ZeroLu/awesome-gpt-image: модель, категория, секция источника и копирование в один клик.",
    metaTitle: "Flatkey Prompts — примеры prompts GPT Image 2",
    modelBrowseBody:
      "У каждой модели будет своя страница, чтобы библиотека росла как реальный справочник по моделям.",
    modelBrowseTitle: "Просмотр по моделям",
    modelFilterLabel: "Модель",
    modelLabel: "Модель",
    mediaTypeFilterLabel: "Тип медиа",
    mediaTypeLabel: "Тип медиа",
    mediaTypes: mediaTypeLabels.ru,
    mediaTypeDescriptions: mediaTypeDescriptions.ru,
    pageTypeFilterLabel: "Тип страницы",
    pageTypeLabel: "Тип страницы",
    promptLabel: "Prompt",
    repositoryLabel: "GitHub README",
    searchPlaceholder: "Искать prompts, теги или источник...",
    sourceLabel: "Источник",
    sourceNote: "В этих примерах видны источник репозитория, секция, текст prompt'а и ссылка на изображение, чтобы будущий API подключился без переделки UI.",
    weeklyHotBody: "Самые популярные prompts в каждом медиа-группировании, собранные в недельную ленту.",
    weeklyHotTitle: "Самое горячее недели",
    viewSource: "Открыть источник",
  },
  ja: {
    allMediaTypes: "すべてのメディア",
    allCategories: "すべて",
    allModels: "すべてのモデル",
    allPageTypes: "すべてのページタイプ",
    copiedPrompt: "コピー済み",
    copyPrompt: "プロンプトをコピー",
    emptyBody: "別のキーワードを試すか、カテゴリをリセットしてください。",
    emptyTitle: "一致するプロンプトはありません",
    exampleCountLabel: "件のサンプルプロンプト",
    featuredLabel: "注目サンプル",
    filterLabel: "フィルター",
    mediaBrowseBody:
      "画像、動画、音声のプロンプト集をタイプごとに辿り、各モデルページへ進めます。",
    mediaBrowseTitle: "メディア別に見る",
    heroBadge: "リポジトリ由来のサンプル",
    heroBody:
      "ZeroLu/awesome-gpt-image から集めた小さなプロンプト集を、Flatkey のホームページに寄せた紫黒のアクセント、密度の高いカード、出典追跡のしやすさで組み直しました。API フィードが来るまでの仮置きです。",
    heroTitle: "コピーして、出典を残して、そのまま使えるプロンプト。",
    metaDescription:
      "ZeroLu/awesome-gpt-image 由来の Flatkey プロンプト集を、モデル、カテゴリ、出典セクション、ワンクリックコピー付きで閲覧できます。",
    metaTitle: "Flatkey Prompts — GPT Image 2 プロンプト例",
    modelBrowseBody:
      "各モデルに専用ページを用意し、後から本格的なモデル別リファレンスに育てられるようにします。",
    modelBrowseTitle: "モデル別に見る",
    modelFilterLabel: "モデル",
    modelLabel: "モデル",
    mediaTypeFilterLabel: "メディア種別",
    mediaTypeLabel: "メディア種別",
    mediaTypes: mediaTypeLabels.ja,
    mediaTypeDescriptions: mediaTypeDescriptions.ja,
    pageTypeFilterLabel: "ページタイプ",
    pageTypeLabel: "ページタイプ",
    promptLabel: "プロンプト",
    repositoryLabel: "GitHub README",
    searchPlaceholder: "プロンプト、タグ、出典を検索...",
    sourceLabel: "出典",
    sourceNote: "これらの例では、リポジトリの出典、セクション、プロンプト本文、画像参照をそのまま見せているので、後続の API を UI 変更なしで差し込めます。",
    weeklyHotBody: "各メディア群で今よく見られているプロンプトを、週次のホット欄として並べます。",
    weeklyHotTitle: "今週の人気",
    viewSource: "出典を見る",
  },
  vi: {
    allMediaTypes: "Tất cả media",
    allCategories: "Tất cả",
    allModels: "Tất cả model",
    allPageTypes: "Tất cả loại trang",
    copiedPrompt: "Đã sao chép",
    copyPrompt: "Sao chép prompt",
    emptyBody: "Thử từ khóa khác hoặc bỏ lọc danh mục.",
    emptyTitle: "Không có prompt phù hợp",
    exampleCountLabel: "prompt mẫu",
    featuredLabel: "Ví dụ nổi bật",
    filterLabel: "Bộ lọc",
    mediaBrowseBody:
      "Duyệt bộ prompt hình ảnh, video và âm thanh theo từng loại, rồi vào từng trang model riêng.",
    mediaBrowseTitle: "Duyệt theo media",
    heroBadge: "Mẫu lấy từ repo",
    heroBody:
      "Một thư viện prompt nhỏ được gieo từ ZeroLu/awesome-gpt-image và làm lại theo ngôn ngữ trang chủ của Flatkey: nhấn mạnh tím-đen, thẻ dày thông tin và truy nguồn rõ ràng cho tới khi feed API sẵn sàng.",
    heroTitle: "Sao chép prompt. Giữ nguồn. Xuất ảnh ngay.",
    metaDescription:
      "Duyệt một thư viện prompt nhỏ của Flatkey được gieo từ ZeroLu/awesome-gpt-image, kèm model, danh mục, mục nguồn và nút sao chép.",
    metaTitle: "Flatkey Prompts — ví dụ prompt GPT Image 2",
    modelBrowseBody:
      "Mỗi model có một trang riêng để sau này bộ prompt có thể lớn lên thành tài liệu tham khảo thực sự.",
    modelBrowseTitle: "Duyệt theo model",
    modelFilterLabel: "Model",
    modelLabel: "Model",
    mediaTypeFilterLabel: "Loại media",
    mediaTypeLabel: "Loại media",
    mediaTypes: mediaTypeLabels.vi,
    mediaTypeDescriptions: mediaTypeDescriptions.vi,
    pageTypeFilterLabel: "Loại trang",
    pageTypeLabel: "Loại trang",
    promptLabel: "Prompt",
    repositoryLabel: "README GitHub",
    searchPlaceholder: "Tìm prompt, tag hoặc nguồn...",
    sourceLabel: "Nguồn",
    sourceNote: "Các ví dụ này giữ nguyên nguồn repo, phần mục, nội dung prompt và tham chiếu ảnh để API sau này cắm vào mà không phải làm lại UI.",
    weeklyHotBody: "Các prompt được xem nhiều nhất trong từng nhóm media, xếp thành một dải nóng theo tuần.",
    weeklyHotTitle: "Nóng nhất tuần",
    viewSource: "Xem nguồn",
  },
  de: {
    allMediaTypes: "Alle Medien",
    allCategories: "Alle",
    allModels: "Alle Modelle",
    allPageTypes: "Alle Seitentypen",
    copiedPrompt: "Kopiert",
    copyPrompt: "Prompt kopieren",
    emptyBody: "Versuche ein anderes Stichwort oder lösche den Filter.",
    emptyTitle: "Keine passenden Prompts",
    exampleCountLabel: "Beispiel-Prompts",
    featuredLabel: "Ausgewähltes Beispiel",
    filterLabel: "Filter",
    mediaBrowseBody:
      "Durchsuche Bild-, Video- und Audio-Prompt-Sammlungen nach Typ und gehe dann zu den einzelnen Modellseiten.",
    mediaBrowseTitle: "Nach Medium durchsuchen",
    heroBadge: "Mustersatz aus dem Repo",
    heroBody:
      "Eine kleine Prompt-Bibliothek, die aus ZeroLu/awesome-gpt-image gespeist und in die visuelle Sprache der Flatkey-Startseite übersetzt wurde: violett-schwarze Akzente, dichte Karten und klare Quellennachverfolgung bis der API-Feed kommt.",
    heroTitle: "Prompt kopieren. Quelle behalten. Bild ausliefern.",
    metaDescription:
      "Durchsuche eine kleine Flatkey-Prompt-Bibliothek, gespeist aus ZeroLu/awesome-gpt-image, mit Modell, Kategorie, Quellabschnitt und Ein-Klick-Kopie.",
    metaTitle: "Flatkey Prompts — GPT Image 2 Prompt-Beispiele",
    modelBrowseBody:
      "Jedes Modell bekommt eine eigene Seite, damit die Sammlung später als echte modellbezogene Referenz wachsen kann.",
    modelBrowseTitle: "Nach Modell durchsuchen",
    modelFilterLabel: "Modell",
    modelLabel: "Modell",
    mediaTypeFilterLabel: "Medientyp",
    mediaTypeLabel: "Medientyp",
    mediaTypes: mediaTypeLabels.de,
    mediaTypeDescriptions: mediaTypeDescriptions.de,
    pageTypeFilterLabel: "Seitentyp",
    pageTypeLabel: "Seitentyp",
    promptLabel: "Prompt",
    repositoryLabel: "GitHub README",
    searchPlaceholder: "Prompts, Tags oder Quelle suchen...",
    sourceLabel: "Quelle",
    sourceNote: "Diese Beispiele halten Repo-Quelle, Abschnitt, Prompt-Text und Bildreferenz sichtbar, damit die spätere API ohne UI-Umbau einsetzen kann.",
    weeklyHotBody: "Die meistgesehenen Prompts pro Mediengruppe, als Wochenleiste sortiert.",
    weeklyHotTitle: "Wöchentliche Top-Prompts",
    viewSource: "Quelle ansehen",
  },
  id: {
    allMediaTypes: "Semua media",
    allCategories: "Semua",
    allModels: "Semua model",
    allPageTypes: "Semua tipe halaman",
    copiedPrompt: "Disalin",
    copyPrompt: "Salin prompt",
    emptyBody: "Coba kata kunci lain atau kosongkan filter kategori.",
    emptyTitle: "Tidak ada prompt yang cocok",
    exampleCountLabel: "prompt contoh",
    featuredLabel: "Contoh unggulan",
    filterLabel: "Filter",
    mediaBrowseBody:
      "Telusuri koleksi prompt gambar, video, dan audio per jenis, lalu buka halaman model masing-masing.",
    mediaBrowseTitle: "Telusuri berdasarkan media",
    heroBadge: "Contoh dari repo",
    heroBody:
      "Perpustakaan prompt kecil yang diambil dari ZeroLu/awesome-gpt-image lalu disesuaikan dengan bahasa visual beranda Flatkey: aksen ungu-hitam, kartu padat, dan jejak sumber yang jelas sampai feed API siap.",
    heroTitle: "Salin prompt. Simpan sumber. Kirim gambar.",
    metaDescription:
      "Jelajahi perpustakaan prompt kecil Flatkey yang diambil dari ZeroLu/awesome-gpt-image, lengkap dengan model, kategori, bagian sumber, dan salin sekali klik.",
    metaTitle: "Flatkey Prompts — contoh prompt GPT Image 2",
    modelBrowseBody:
      "Setiap model punya halaman sendiri supaya koleksinya bisa tumbuh jadi referensi nyata per model.",
    modelBrowseTitle: "Telusuri berdasarkan model",
    modelFilterLabel: "Model",
    modelLabel: "Model",
    mediaTypeFilterLabel: "Jenis media",
    mediaTypeLabel: "Jenis media",
    mediaTypes: mediaTypeLabels.id,
    mediaTypeDescriptions: mediaTypeDescriptions.id,
    pageTypeFilterLabel: "Tipe halaman",
    pageTypeLabel: "Tipe halaman",
    promptLabel: "Prompt",
    repositoryLabel: "README GitHub",
    searchPlaceholder: "Cari prompt, tag, atau sumber...",
    sourceLabel: "Sumber",
    sourceNote: "Contoh-contoh ini tetap menampilkan sumber repo, bagian, isi prompt, dan referensi gambar supaya API nanti bisa masuk tanpa ubah UI.",
    weeklyHotBody: "Prompt yang paling sering dilihat di tiap grup media, disusun sebagai strip mingguan.",
    weeklyHotTitle: "Paling panas minggu ini",
    viewSource: "Lihat sumber",
  },
});

const examples: PromptLibraryExample[] = [
  {
    category: "photography",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "landing-page",
    previewImage: {
      alt: "Convenience store night scene sample",
      src: "https://github.com/user-attachments/assets/91c95d69-1094-472e-9410-8a86bad9b086",
    },
    prompt:
      "Create an ultra-realistic urban street group photo at a convenience store entrance at 10 PM summer night. 3-4 young people briefly chatting at the entrance, someone holding drinks, someone sitting on plastic outdoor chairs, someone standing looking at their phone. Bright white light streaming through the glass doors and windows, warm yellow street lights and distant car headlights outside. Characters wearing everyday clothes: T-shirts, shirts, shorts, jeans, sneakers. No internet celebrity styling. Faces and postures must look like real pedestrians, not overly polished. Environment must include real convenience store elements: freezer stickers, promotional posters, trash cans, entrance mats, glass reflections, shared bikes on roadside, water droplets from drink bottles on ground. The image should look like a very authentic life slice captured by a photographer in the city.",
    ratio: "16:9",
    slug: "convenience-store-night-scene",
    source: {
      label: "卡尔的AI沃茨",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Photography & Photorealism",
      url: "https://mp.weixin.qq.com/s/ASxig6mFVYxrIE8-8Fthew",
    },
    tags: ["night", "street", "photorealistic", "group"],
    title: "Convenience Store Night Scene",
    updatedAt: UPDATED_AT,
  },
  {
    category: "photography",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "landing-page",
    previewImage: {
      alt: "RAW iPhone subway station sample",
      src: "https://pbs.twimg.com/media/HFPFe0VbkAYeflZ?format=jpg&name=large",
    },
    prompt:
      "Create a completely RAW quality, unprocessed, unedited image with full iPhone camera quality. A subway station in USA, a momentary blur. The subway is in motion. In front of the subway, there is an elderly woman and man.",
    ratio: "3:2",
    slug: "raw-iphone-subway-station",
    source: {
      label: "@WolfRiccardo",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Photography & Photorealism",
      url: "https://x.com/WolfRiccardo/status/2041192232623972441",
    },
    tags: ["raw", "iphone", "subway", "motion"],
    title: "RAW iPhone Quality - Subway Station",
    updatedAt: UPDATED_AT,
  },
  {
    category: "photography",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "ad-creative",
    previewImage: {
      alt: "90s point-and-shoot camera quality sample",
      src: "https://pbs.twimg.com/media/HGlDZasaMAAtLeg?format=jpg&name=large",
    },
    prompt: "90s + point-and-shoot camera quality",
    ratio: "4:3",
    slug: "90s-point-and-shoot-aesthetic",
    source: {
      label: "@sunyunran",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Photography & Photorealism",
      url: "https://x.com/sunyunran/status/2047241649957503308",
    },
    tags: ["film", "retro", "camera"],
    title: "90s Point-and-Shoot Aesthetic",
    updatedAt: UPDATED_AT,
  },
  {
    category: "photography",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "landing-page",
    previewImage: {
      alt: "Apple Park keynote crowd shot sample",
      src: "https://raw.githubusercontent.com/ZeroLu/awesome-gpt-image/main/assets/opennana/apple-park-tim-cook-keynote.jpg",
    },
    prompt:
      "Apple Park keynote crowd shot",
    ratio: "16:9",
    slug: "apple-park-keynote-crowd-shot",
    source: {
      label: "OpenNana | @patrickassale",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Photography & Photorealism",
      url: "https://opennana.com/awesome-prompt-gallery/apple-park-tim-cook-keynote",
    },
    tags: ["keynote", "crowd", "apple", "photography"],
    title: "Apple Park Keynote Crowd Shot",
    updatedAt: UPDATED_AT,
  },
  {
    category: "infographic",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "infographic-page",
    previewImage: {
      alt: "Three-day travel guide card sample",
      src: "https://pbs.twimg.com/media/HGa2KbFXMAAv9Wh?format=jpg&name=large",
    },
    prompt: "Generate a three-day travel guide image for [city]",
    ratio: "9:16",
    slug: "three-day-travel-guide-card",
    source: {
      label: "@MrLarus",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Infographics, Education & Documents",
      url: "https://x.com/MrLarus/status/2046523494003851300",
    },
    tags: ["travel", "guide", "card", "layout"],
    title: "Three-Day Travel Guide Card",
    updatedAt: UPDATED_AT,
  },
  {
    category: "infographic",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "infographic-page",
    previewImage: {
      alt: "World time analog clock wall sample",
      src: "https://pbs.twimg.com/media/HGYD-Y4bMAA0KxJ?format=jpg&name=large",
    },
    prompt: "World time analog clock wall",
    ratio: "4:5",
    slug: "world-time-analog-clock-wall",
    source: {
      label: "@Angaisb_",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Infographics, Education & Documents",
      url: "https://x.com/Angaisb_/status/2046666389734179018",
    },
    tags: ["clock", "time", "infographic", "layout"],
    title: "World Time Analog Clock Wall",
    updatedAt: UPDATED_AT,
  },
  {
    category: "character",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "character-reference",
    previewImage: {
      alt: "Official character reference sheet sample",
      src: "https://raw.githubusercontent.com/ZeroLu/awesome-gpt-image/main/assets/opennana/official-character-reference-sheet.jpeg",
    },
    prompt:
      "Based on this character and background, please create a character reference sheet similar to official setting materials.\n- Includes three-view drawings: front view, side view, and back view\n- Add variations of the character's facial expressions\n- Break down and display detailed parts of the clothing and equipment\n- Add a color palette\n- Include a brief explanation of the worldview setting\n- Overall, use an organized layout (white background, illustration style)",
    ratio: "4:3",
    slug: "official-character-reference-sheet",
    source: {
      label: "OpenNana",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Character & Consistency",
      url: "https://opennana.com/awesome-prompt-gallery/official-character-reference-sheet",
    },
    tags: ["character", "reference", "consistency", "sheet"],
    title: "Official Character Reference Sheet",
    updatedAt: UPDATED_AT,
  },
  {
    category: "editing",
    mediaType: "image",
    model: "GPT Image 2",
    pageType: "product-page",
    previewImage: {
      alt: "Professional Instagram photo enhancement sample",
      src: "https://pbs.twimg.com/media/HGkEbUzbMAANbU9?format=jpg&name=large",
    },
    prompt:
      "Enhance this iPhone photo with ChatGPT so it looks like a professional photographer and designer worked on it.",
    ratio: "4:5",
    slug: "pro-instagram-photo-enhancement",
    source: {
      label: "@dezainaz_ceo",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Image Editing & Style Transfer",
      url: "https://x.com/dezainaz_ceo/status/2047172512551977162",
    },
    tags: ["editing", "instagram", "photo", "enhancement", "product"],
    title: "Pro Instagram Photo Enhancement",
    updatedAt: UPDATED_AT,
  },
  {
    category: "editing",
    mediaType: "video",
    model: "Seedance 2.0",
    pageType: "ad-creative",
    previewImage: {
      alt: "Product reveal video thumbnail",
      src: "/assets/cli/product-reveal.png",
    },
    prompt:
      "Create a polished cinematic product reveal clip with a slow push-in, a clean hero object, subtle parallax, and a premium commercial finish. Keep the motion controlled, the scene uncluttered, and the pacing edit-ready for a social ad.",
    ratio: "16:9",
    slug: "cinematic-product-reveal-clip",
    source: {
      label: "Flatkey CLI",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Video prompt library",
      url: "/cli/video/flatkey-text-to-video-cinematic-test",
    },
    tags: ["video", "product", "ad"],
    title: "Cinematic Product Reveal Clip",
    updatedAt: UPDATED_AT,
  },
  {
    category: "editing",
    mediaType: "video",
    model: "Seedance 2.5",
    pageType: "landing-page",
    previewImage: {
      alt: "UGC ad video thumbnail",
      src: "/assets/cli/ugc-ad-clips.png",
    },
    prompt:
      "Create a vertical UGC-style ad clip from a product demo. Start with a handheld hook, cut to a clear use moment, then end on a benefit reveal. Keep the framing casual, the motion natural, and the video ready for short-form platforms.",
    ratio: "9:16",
    slug: "ugc-style-ad-clip",
    source: {
      label: "Flatkey CLI",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Video prompt library",
      url: "/cli/video/flatkey-image-to-video-product-scene",
    },
    tags: ["video", "ugc", "ads"],
    title: "UGC-Style Ad Clip",
    updatedAt: UPDATED_AT,
  },
  {
    category: "editing",
    mediaType: "audio",
    model: "sonilo-video-to-music",
    pageType: "editing-workflow",
    previewImage: {
      alt: "Audio workflow thumbnail",
      src: "/assets/model-pages/sonilo-video-to-music-hero.png",
    },
    prompt:
      "Generate a warm, cinematic music bed for a product video. Keep the rhythm aligned to the edit, preserve any spoken line, and leave a clean loopable ending for social playback.",
    ratio: "1:1",
    slug: "video-to-music-warm-cinematic-bed",
    source: {
      label: "Flatkey model page",
      repository: REPOSITORY,
      repositoryUrl: REPOSITORY_URL,
      section: "Audio prompt library",
      url: "/models/sonilo-video-to-music",
    },
    tags: ["audio", "video-to-music", "music"],
    title: "Video-to-Music Warm Cinematic Bed",
    updatedAt: UPDATED_AT,
  },
];

export function getPromptLibraryExamples(): readonly PromptLibraryExample[] {
  return examples;
}

export function getPromptLibraryExampleBySlug(slug: string): PromptLibraryExample | undefined {
  return examples.find((item) => item.slug === slug);
}

export function getPromptLibraryExamplesByMediaType(mediaType: PromptMediaType) {
  return examples.filter((item) => item.mediaType === mediaType);
}

export function getPromptLibraryExamplesByModelSlug(modelSlug: string) {
  return examples.filter((item) => getPromptLibraryModelSlug(item.model) === modelSlug);
}

export function getPromptLibraryFilterOptions(
  items: readonly PromptLibraryExample[] = examples,
) {
  return {
    mediaTypes: uniqueInOrder(items.map((item) => item.mediaType)),
    models: uniqueInOrder(items.map((item) => item.model)),
    pageTypes: uniqueInOrder(items.map((item) => item.pageType)),
  };
}

export function filterPromptLibraryExamples(
  items: readonly PromptLibraryExample[],
  filters: PromptLibraryFilters = {},
) {
  const category = filters.category ?? "all";
  const mediaType = filters.mediaType ?? "all";
  const pageType = filters.pageType ?? "all";
  const model = filters.model ?? "all";
  const needle = filters.query?.trim().toLowerCase() ?? "";

  return items.filter((item) => {
    if (category !== "all" && item.category !== category) return false;
    if (mediaType !== "all" && item.mediaType !== mediaType) return false;
    if (pageType !== "all" && item.pageType !== pageType) return false;
    if (model !== "all" && item.model !== model) return false;
    if (!needle) return true;

    return [
      item.title,
      item.prompt,
      item.model,
      item.pageType,
      item.ratio,
      item.source.label,
      item.source.section,
      item.tags.join(" "),
    ]
      .join(" ")
      .toLowerCase()
      .includes(needle);
  });
}

export function getPromptLibraryPageCopy(locale: Locale): PromptLibraryPageCopy {
  return {
    ...promptLibraryPageCopyBase[locale],
    categories: categoryLabels[locale],
    mediaTypeDescriptions: mediaTypeDescriptions[locale],
    mediaTypes: mediaTypeLabels[locale],
    pageTypes: pageTypeLabels[locale],
  };
}

export function getPromptLibraryTypePath(mediaType: PromptMediaType) {
  return `${PROMPTS_PATH}/${mediaType}`;
}

export function getPromptLibraryModelPath(modelSlug: string) {
  return `${PROMPTS_PATH}/models/${modelSlug}`;
}

export function getPromptLibraryPromptPath(item: PromptLibraryExample) {
  return `${PROMPTS_PATH}/${item.slug}`;
}

export function getPromptLibraryModelSlug(model: string) {
  return model.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "");
}

export function getPromptLibraryModelDisplayName(model: string) {
  return MODEL_DISPLAY_NAMES[model] ?? model;
}

export function getPromptLibraryModelMediaType(model: string): PromptMediaType {
  return MODEL_MEDIA_TYPES[model] ?? "image";
}

export function getPromptLibraryMediaSummaries(): PromptLibraryMediaSummary[] {
  return MEDIA_TYPES.map((type) => {
    const items = examples.filter((item) => item.mediaType === type);
    const models = uniqueInOrder(items.map((item) => item.model));
    return {
      count: items.length,
      description: mediaTypeDescriptions.en[type],
      href: getPromptLibraryTypePath(type),
      modelCount: models.length,
      type,
    };
  });
}

export function getPromptLibraryModelSummaries(): PromptLibraryModelSummary[] {
  const seen = new Set<string>();
  const summaries = examples.reduce<PromptLibraryModelSummary[]>((acc, item) => {
    const slug = getPromptLibraryModelSlug(item.model);
    if (seen.has(slug)) return acc;
    seen.add(slug);
    const count = examples.filter((candidate) => getPromptLibraryModelSlug(candidate.model) === slug).length;
    acc.push({
      count,
      displayName: getPromptLibraryModelDisplayName(item.model),
      href: getPromptLibraryModelPath(slug),
      mediaType: item.mediaType,
      slug,
    });
    return acc;
  }, []);

  for (const fallback of EMPTY_AUDIO_MODELS) {
    if (seen.has(fallback.slug)) continue;
    seen.add(fallback.slug);
    summaries.push({
      count: 0,
      displayName: fallback.displayName,
      href: getPromptLibraryModelPath(fallback.slug),
      mediaType: fallback.mediaType,
      slug: fallback.slug,
    });
  }

  return summaries;
}

export function getPromptLibraryMediaSummary(type: PromptMediaType) {
  return getPromptLibraryMediaSummaries().find((item) => item.type === type);
}

export function getPromptLibraryModelSummary(modelSlug: string) {
  return getPromptLibraryModelSummaries().find((item) => item.slug === modelSlug);
}

export function getPromptLibraryStaticPathnames() {
  return [
    PROMPTS_PATH,
    ...MEDIA_TYPES.map((type) => getPromptLibraryTypePath(type)),
    ...getPromptLibraryModelSummaries().map((item) => item.href),
    ...examples.map((item) => getPromptLibraryPromptPath(item)),
  ];
}

function uniqueInOrder<T extends string>(values: readonly T[]): T[] {
  return Array.from(new Set(values));
}
