import {
  ArrowRight,
  BookOpen,
  Boxes,
  Code2,
  FileText,
  Image as ImageIcon,
  KeyRound,
  Layers3,
  Play,
  Search,
  Sparkles,
  type LucideIcon,
  Video,
} from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import type { ReactNode } from "react";
import { CLI_IMAGE_PATH, CLI_VIDEO_PATH } from "@/lib/cli-landing";
import { type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";
import {
  getCliMediaPromptItems,
  type PromptArtifact,
  promptLibraryCopy,
  type PromptItem,
} from "@/lib/prompt-library";

type MediaKind = "image" | "video";
type VisualArtifact = Extract<PromptArtifact, { kind: "image" | "video" }>;
type VisualPromptItem = PromptItem & { artifact: VisualArtifact };

type CollectionCopy = {
  title: string;
  body: string;
  meta: string;
};

type MediaCardCopy = {
  body: string;
  keywords: string[];
  label: string;
};

type ModelCardCopy = {
  body: string;
  media: string;
};

type KeywordGroupCopy = {
  label: string;
  words: string[];
};

type PlaygroundIndexCopy = {
  aboutBody: string;
  aboutTitle: string;
  categoryTitle: string;
  collectionEyebrow: string;
  collectionTitle: string;
  collections: CollectionCopy[];
  finalCtaBody: string;
  finalCtaButton: string;
  finalCtaTitle: string;
  heroAccent: string;
  heroBadge: string;
  heroBody: string;
  heroStats: string[];
  heroTitle: string;
  imageIndexTitle: string;
  mediaCards: Record<MediaKind, MediaCardCopy>;
  mediaTitle: string;
  modelCards: ModelCardCopy[];
  modelHint: string;
  modelTitle: string;
  moreTitle: string;
  openPrompt: string;
  promptUnit: string;
  searchBody: string;
  searchTitle: string;
  secondaryFeatureBody: string;
  secondaryFeatureTitle: string;
  smallFeatureLinks: string[];
  tagUnit: string;
  videoIndexTitle: string;
  weeklyHint: string;
  weeklyTitle: string;
  keywordGroups: Record<MediaKind, KeywordGroupCopy[]>;
};

const copyByLocale: Record<Locale, PlaygroundIndexCopy> = withIdFallback({
  en: {
    aboutBody:
      "Flatkey keeps reusable image and video prompts together with the output they produced. Use the library to compare keywords, models, and finished artifacts before you spend credits in the playground.",
    aboutTitle: "What is the Flatkey AI prompt library?",
    categoryTitle: "Browse by category",
    collectionEyebrow: "Curated sets",
    collectionTitle: "Featured collections",
    collections: [
      {
        title: "Product image starter pack",
        body: "Marketplace, app-store, poster, and product hero prompts that already have usable image outputs.",
        meta: "Image prompts",
      },
      {
        title: "Seedance video production set",
        body: "Short-form video prompts for product reveal, UGC ads, localization, and reference motion tests.",
        meta: "Video prompts",
      },
      {
        title: "Character and scene boards",
        body: "Prompt-output pairs for consistent character sheets, props, scene bibles, and storyboard panels.",
        meta: "Image plus video planning",
      },
    ],
    finalCtaBody:
      "Open the playground with a starter key, then run any prompt against Flatkey's image and video routes.",
    finalCtaButton: "Start in Flatkey",
    finalCtaTitle: "Free for creators, forever.",
    heroAccent: "library",
    heroBadge: "20,000+ prompts - image and video",
    heroBody:
      "A public prompt index for creators who want examples with visible outputs. Start from a collection, browse by media, or jump into a model card.",
    heroStats: ["20,000+ prompt rows", "Image and video only", "Source and output paired"],
    heroTitle: "AI prompt",
    imageIndexTitle: "Image prompt index",
    mediaCards: {
      image: {
        body: "Stable image prompts for ecommerce, posters, character sheets, UI shots, and campaign assets.",
        keywords: ["product", "poster", "avatar", "storyboard"],
        label: "Image",
      },
      video: {
        body: "Video prompts for UGC clips, product reveal, localized variants, image-to-video, and motion tests.",
        keywords: ["UGC", "reveal", "i2v", "motion"],
        label: "Video",
      },
    },
    mediaTitle: "Browse by media",
    modelCards: [
      { body: "High-fidelity commercial images and editable product compositions.", media: "Image" },
      { body: "Fast creative image drafts for character, product, and poster exploration.", media: "Image" },
      { body: "Balanced image generation for ads, concept boards, and localized visuals.", media: "Image" },
      { body: "Classic OpenAI image route for stable prompt migration tests.", media: "Image" },
      { body: "Image-to-video and character motion prompts with stricter identity control.", media: "Video" },
      { body: "Seedance prompt patterns for short ads, reveal shots, and storyboard motion.", media: "Video" },
      { body: "Exploratory image prompts for social, meme, and fast iteration workflows.", media: "Image" },
      { body: "Gemini prompt templates for multimodal planning and creative variants.", media: "Image" },
    ],
    modelHint: "Temporary model cards while the live prompt catalog is being wired in.",
    modelTitle: "Browse by model",
    moreTitle: "More prompt tools",
    openPrompt: "Open prompt",
    promptUnit: "prompts",
    searchBody:
      "Search prompts by task, model, output type, or visual keyword, then send the exact prompt into the Flatkey console.",
    searchTitle: "AI prompt search",
    secondaryFeatureBody:
      "Drop in a reference image and turn visual intent into a reusable prompt for the next image or video run.",
    secondaryFeatureTitle: "Image to prompt",
    smallFeatureLinks: ["GPT Image prompts", "Seedance prompts", "Image prompt editor", "Video prompt editor", "Prompt collections", "AI tools"],
    tagUnit: "keywords",
    videoIndexTitle: "Video prompt index",
    weeklyHint: "A rotating sample of prompt-output pairs from the current local library.",
    weeklyTitle: "Weekly hot",
    keywordGroups: {
      image: [
        { label: "Use case", words: ["ecommerce main image", "ad poster", "app screenshot", "character sheet"] },
        { label: "Style", words: ["clean product photo", "liquid glass", "editorial collage", "concept board"] },
        { label: "Subject", words: ["skincare bottle", "sports shoe", "avatar portrait", "game prop"] },
      ],
      video: [
        { label: "Use case", words: ["UGC ad", "product reveal", "localized launch", "reference motion"] },
        { label: "Motion", words: ["slow push-in", "handheld pickup", "parallax", "single take"] },
        { label: "Subject", words: ["consumer product", "character", "market variant", "clip poster"] },
      ],
    },
  },
  zh: {
    aboutBody:
      "Flatkey 把可复用的图片、视频提示词和它们真实产出的结果放在一起。你可以先比较关键词、模型和成片效果，再进入 playground 消耗额度生成。",
    aboutTitle: "Flatkey AI 提示词库是什么？",
    categoryTitle: "按分类浏览",
    collectionEyebrow: "精选合集",
    collectionTitle: "精选合集",
    collections: [
      {
        title: "电商图片素材包",
        body: "覆盖平台主图、应用商店海报、活动主视觉和产品首图，全部带可见图片产物。",
        meta: "图片提示词",
      },
      {
        title: "Seedance 视频制作合集",
        body: "整理产品揭幕、UGC 广告、本地化变体、参考图动效等短视频提示词。",
        meta: "视频提示词",
      },
      {
        title: "角色与场景设定合集",
        body: "包含角色设定、道具图、场景圣经和分镜板，适合做连续内容的视觉基准。",
        meta: "图片 + 视频规划",
      },
    ],
    finalCtaBody:
      "进入 playground，选择图片或视频模型，把提示词直接带到 Flatkey 控制台里运行。",
    finalCtaButton: "开始使用 Flatkey",
    finalCtaTitle: "为创作者而生，永久免费。",
    heroAccent: "资料库",
    heroBadge: "20,000+ 提示词 - 图片与视频",
    heroBody:
      "面向创作者的公开提示词索引。你可以从合集开始，按图片/视频媒介浏览，也可以先按模型查看示例。",
    heroStats: ["20,000+ 提示词行", "只保留图片和视频", "来源与产物配对"],
    heroTitle: "AI 提示词",
    imageIndexTitle: "图片提示词索引",
    mediaCards: {
      image: {
        body: "适合电商主图、海报、角色设定、界面截图和营销素材的稳定图片提示词。",
        keywords: ["产品", "海报", "头像", "分镜"],
        label: "图片",
      },
      video: {
        body: "适合 UGC 短片、产品揭幕、本地化变体、图生视频和动作测试的视频提示词。",
        keywords: ["UGC", "揭幕", "图生视频", "动作"],
        label: "视频",
      },
    },
    mediaTitle: "按媒介浏览",
    modelCards: [
      { body: "高保真商业图片、可编辑产品构图和广告素材生成。", media: "图片" },
      { body: "快速探索角色、产品、海报方向的图片草稿模型。", media: "图片" },
      { body: "适合广告、概念板和本地化视觉的均衡图片模型。", media: "图片" },
      { body: "用于迁移老图片提示词的 OpenAI 经典图片路由。", media: "图片" },
      { body: "面向图生视频和角色动作，强调身份一致性的提示词。", media: "视频" },
      { body: "适合短广告、揭幕镜头和分镜动效的 Seedance 模式。", media: "视频" },
      { body: "面向社媒、梗图和快速迭代的探索型图片提示词。", media: "图片" },
      { body: "用于多模态规划和创意变体的 Gemini 提示词模板。", media: "图片" },
    ],
    modelHint: "这里先放临时模型卡片，后续接入真实提示词目录。",
    modelTitle: "按模型浏览",
    moreTitle: "更多提示词功能",
    openPrompt: "查看提示词",
    promptUnit: "个提示词",
    searchBody:
      "按任务、模型、产物类型或视觉关键词搜索提示词，再把完整提示词带入 Flatkey 控制台。",
    searchTitle: "AI 搜索提示词",
    secondaryFeatureBody:
      "上传参考图，把画面意图整理成可复用提示词，再用于下一次图片或视频生成。",
    secondaryFeatureTitle: "图片转提示词",
    smallFeatureLinks: ["GPT Image 提示词", "Seedance 提示词", "图片提示词编辑器", "视频提示词编辑器", "提示词合集", "AI 工具"],
    tagUnit: "个关键词",
    videoIndexTitle: "视频提示词索引",
    weeklyHint: "从当前本地提示词库中抽取的提示词-产物配对样例。",
    weeklyTitle: "每周最热",
    keywordGroups: {
      image: [
        { label: "使用场景", words: ["电商主图", "广告海报", "App 截图", "角色设定"] },
        { label: "风格", words: ["干净产品摄影", "液态玻璃", "杂志拼贴", "概念设定板"] },
        { label: "主体", words: ["护肤瓶", "运动鞋", "头像肖像", "游戏道具"] },
      ],
      video: [
        { label: "使用场景", words: ["UGC 广告", "产品揭幕", "本地化发布", "参考图动作"] },
        { label: "镜头运动", words: ["慢推镜头", "手持拿起", "视差动效", "单镜到底"] },
        { label: "主体", words: ["消费品", "角色", "市场变体", "短片封面"] },
      ],
    },
  },
  es: {
    aboutBody:
      "Flatkey guarda prompts reutilizables de imagen y video junto al resultado que produjeron. Compara palabras clave, modelos y artefactos antes de gastar créditos en el playground.",
    aboutTitle: "¿Qué es la biblioteca de prompts de Flatkey?",
    categoryTitle: "Explorar por categoría",
    collectionEyebrow: "Colecciones",
    collectionTitle: "Colecciones destacadas",
    collections: [
      { title: "Pack inicial de imagen de producto", body: "Prompts para marketplace, app-store, póster y hero de producto con salidas de imagen utilizables.", meta: "Prompts de imagen" },
      { title: "Set de producción Seedance", body: "Prompts de video corto para reveal, UGC ads, localización y pruebas de movimiento por referencia.", meta: "Prompts de video" },
      { title: "Personajes y tableros de escena", body: "Pares prompt-resultado para personajes, props, escenas y paneles de storyboard.", meta: "Imagen y planificación de video" },
    ],
    finalCtaBody: "Abre el playground con una key inicial y ejecuta cualquier prompt en rutas de imagen o video.",
    finalCtaButton: "Empezar en Flatkey",
    finalCtaTitle: "Gratis para creadores, para siempre.",
    heroAccent: "biblioteca",
    heroBadge: "20.000+ prompts - imagen y video",
    heroBody: "Índice público de prompts para creadores que quieren ejemplos con resultados visibles.",
    heroStats: ["20.000+ filas", "Solo imagen y video", "Fuente y resultado unidos"],
    heroTitle: "Biblioteca de prompts IA",
    imageIndexTitle: "Índice de prompts de imagen",
    mediaCards: {
      image: { body: "Prompts de imagen para ecommerce, pósters, personajes, UI y campañas.", keywords: ["producto", "póster", "avatar", "storyboard"], label: "Imagen" },
      video: { body: "Prompts de video para UGC, reveal, variantes locales, image-to-video y movimiento.", keywords: ["UGC", "reveal", "i2v", "motion"], label: "Video" },
    },
    mediaTitle: "Explorar por medio",
    modelCards: [
      { body: "Imágenes comerciales fieles y composiciones de producto editables.", media: "Imagen" },
      { body: "Borradores rápidos para personajes, productos y pósters.", media: "Imagen" },
      { body: "Generación equilibrada para anuncios, moodboards y visuales localizados.", media: "Imagen" },
      { body: "Ruta clásica para migrar prompts de imagen existentes.", media: "Imagen" },
      { body: "Image-to-video y movimiento de personajes con control de identidad.", media: "Video" },
      { body: "Patrones Seedance para anuncios cortos, reveals y storyboard en movimiento.", media: "Video" },
      { body: "Prompts exploratorios para social, memes e iteración rápida.", media: "Imagen" },
      { body: "Plantillas Gemini para planificación multimodal y variantes creativas.", media: "Imagen" },
    ],
    modelHint: "Tarjetas temporales mientras conectamos el catálogo real.",
    modelTitle: "Explorar por modelo",
    moreTitle: "Más herramientas de prompt",
    openPrompt: "Abrir prompt",
    promptUnit: "prompts",
    searchBody: "Busca por tarea, modelo, tipo de salida o palabra visual y envía el prompt a la consola Flatkey.",
    searchTitle: "Búsqueda de prompts IA",
    secondaryFeatureBody: "Convierte una imagen de referencia en un prompt reutilizable para la siguiente generación.",
    secondaryFeatureTitle: "Imagen a prompt",
    smallFeatureLinks: ["Prompts GPT Image", "Prompts Seedance", "Editor de imagen", "Editor de video", "Colecciones", "Herramientas IA"],
    tagUnit: "palabras clave",
    videoIndexTitle: "Índice de prompts de video",
    weeklyHint: "Muestra rotativa de pares prompt-resultado del catálogo local.",
    weeklyTitle: "Popular de la semana",
    keywordGroups: {
      image: [
        { label: "Caso de uso", words: ["imagen ecommerce", "póster publicitario", "captura de app", "hoja de personaje"] },
        { label: "Estilo", words: ["foto limpia", "liquid glass", "collage editorial", "concept board"] },
        { label: "Sujeto", words: ["frasco skincare", "zapatilla", "retrato avatar", "prop de juego"] },
      ],
      video: [
        { label: "Caso de uso", words: ["UGC ad", "reveal producto", "lanzamiento local", "movimiento referencia"] },
        { label: "Movimiento", words: ["push-in lento", "handheld", "parallax", "toma única"] },
        { label: "Sujeto", words: ["producto", "personaje", "variante mercado", "poster de clip"] },
      ],
    },
  },
  fr: {
    aboutBody:
      "Flatkey conserve les prompts image et vidéo réutilisables avec le résultat produit. Comparez mots-clés, modèles et artefacts avant de dépenser des crédits.",
    aboutTitle: "Qu'est-ce que la bibliothèque de prompts Flatkey ?",
    categoryTitle: "Explorer par catégorie",
    collectionEyebrow: "Sélections",
    collectionTitle: "Collections à la une",
    collections: [
      { title: "Pack image produit", body: "Prompts marketplace, app-store, affiche et hero produit avec sorties image exploitables.", meta: "Prompts image" },
      { title: "Set vidéo Seedance", body: "Prompts vidéo courte pour reveal, UGC ads, localisation et tests de mouvement.", meta: "Prompts vidéo" },
      { title: "Personnages et scènes", body: "Paires prompt-résultat pour fiches personnage, props, scènes et storyboards.", meta: "Image et plan vidéo" },
    ],
    finalCtaBody: "Ouvrez le playground puis lancez vos prompts sur les routes image et vidéo Flatkey.",
    finalCtaButton: "Commencer sur Flatkey",
    finalCtaTitle: "Gratuit pour les créateurs, pour toujours.",
    heroAccent: "bibliothèque",
    heroBadge: "20 000+ prompts - image et vidéo",
    heroBody: "Index public de prompts pour créateurs avec exemples et résultats visibles.",
    heroStats: ["20 000+ lignes", "Image et vidéo seulement", "Source et résultat liés"],
    heroTitle: "Bibliothèque de prompts IA",
    imageIndexTitle: "Index des prompts image",
    mediaCards: {
      image: { body: "Prompts image pour ecommerce, affiches, personnages, UI et campagnes.", keywords: ["produit", "affiche", "avatar", "storyboard"], label: "Image" },
      video: { body: "Prompts vidéo pour UGC, reveal, variantes locales, image-to-video et motion.", keywords: ["UGC", "reveal", "i2v", "motion"], label: "Vidéo" },
    },
    mediaTitle: "Explorer par média",
    modelCards: [
      { body: "Images commerciales fidèles et compositions produit éditables.", media: "Image" },
      { body: "Brouillons rapides pour personnages, produits et affiches.", media: "Image" },
      { body: "Génération équilibrée pour annonces, concept boards et visuels localisés.", media: "Image" },
      { body: "Route OpenAI classique pour migrer des prompts image.", media: "Image" },
      { body: "Image-to-video et mouvement de personnage avec identité contrôlée.", media: "Vidéo" },
      { body: "Patterns Seedance pour ads courtes, reveal et storyboard animé.", media: "Vidéo" },
      { body: "Prompts exploratoires pour social, memes et itérations rapides.", media: "Image" },
      { body: "Templates Gemini pour planification multimodale et variantes.", media: "Image" },
    ],
    modelHint: "Cartes temporaires en attendant le catalogue réel.",
    modelTitle: "Explorer par modèle",
    moreTitle: "Plus d'outils de prompt",
    openPrompt: "Ouvrir",
    promptUnit: "prompts",
    searchBody: "Recherchez par tâche, modèle, sortie ou mot visuel puis envoyez le prompt vers la console Flatkey.",
    searchTitle: "Recherche de prompts IA",
    secondaryFeatureBody: "Transformez une image de référence en prompt réutilisable pour la prochaine génération.",
    secondaryFeatureTitle: "Image vers prompt",
    smallFeatureLinks: ["Prompts GPT Image", "Prompts Seedance", "Éditeur image", "Éditeur vidéo", "Collections", "Outils IA"],
    tagUnit: "mots-clés",
    videoIndexTitle: "Index des prompts vidéo",
    weeklyHint: "Exemples tournants issus de la bibliothèque locale.",
    weeklyTitle: "Tendances de la semaine",
    keywordGroups: {
      image: [
        { label: "Usage", words: ["image ecommerce", "affiche pub", "capture app", "fiche personnage"] },
        { label: "Style", words: ["photo produit", "liquid glass", "collage éditorial", "concept board"] },
        { label: "Sujet", words: ["flacon skincare", "chaussure", "portrait avatar", "prop de jeu"] },
      ],
      video: [
        { label: "Usage", words: ["UGC ad", "reveal produit", "lancement local", "mouvement référence"] },
        { label: "Mouvement", words: ["push-in lent", "handheld", "parallax", "plan unique"] },
        { label: "Sujet", words: ["produit", "personnage", "variante marché", "poster clip"] },
      ],
    },
  },
  pt: {
    aboutBody:
      "A Flatkey mantém prompts reutilizáveis de imagem e vídeo junto do resultado produzido. Compare palavras-chave, modelos e artefatos antes de gastar créditos.",
    aboutTitle: "O que é a biblioteca de prompts da Flatkey?",
    categoryTitle: "Explorar por categoria",
    collectionEyebrow: "Curadoria",
    collectionTitle: "Coleções em destaque",
    collections: [
      { title: "Pacote inicial de imagem de produto", body: "Prompts para marketplace, app-store, pôster e hero de produto com imagens utilizáveis.", meta: "Prompts de imagem" },
      { title: "Set de produção Seedance", body: "Prompts de vídeo curto para reveal, UGC ads, localização e movimento por referência.", meta: "Prompts de vídeo" },
      { title: "Personagens e cenas", body: "Pares prompt-resultado para fichas, props, cenas e storyboards.", meta: "Imagem e plano de vídeo" },
    ],
    finalCtaBody: "Abra o playground e rode qualquer prompt nas rotas de imagem e vídeo da Flatkey.",
    finalCtaButton: "Começar na Flatkey",
    finalCtaTitle: "Grátis para criadores, para sempre.",
    heroAccent: "biblioteca",
    heroBadge: "20.000+ prompts - imagem e vídeo",
    heroBody: "Índice público de prompts para criadores que querem exemplos com resultados visíveis.",
    heroStats: ["20.000+ linhas", "Só imagem e vídeo", "Fonte e resultado juntos"],
    heroTitle: "Biblioteca de prompts IA",
    imageIndexTitle: "Índice de prompts de imagem",
    mediaCards: {
      image: { body: "Prompts de imagem para ecommerce, pôsteres, personagens, UI e campanhas.", keywords: ["produto", "pôster", "avatar", "storyboard"], label: "Imagem" },
      video: { body: "Prompts de vídeo para UGC, reveal, variantes locais, image-to-video e motion.", keywords: ["UGC", "reveal", "i2v", "motion"], label: "Vídeo" },
    },
    mediaTitle: "Explorar por mídia",
    modelCards: [
      { body: "Imagens comerciais fiéis e composições de produto editáveis.", media: "Imagem" },
      { body: "Rascunhos rápidos para personagens, produtos e pôsteres.", media: "Imagem" },
      { body: "Geração equilibrada para anúncios, concept boards e visuais localizados.", media: "Imagem" },
      { body: "Rota clássica para migrar prompts de imagem.", media: "Imagem" },
      { body: "Image-to-video e movimento de personagem com controle de identidade.", media: "Vídeo" },
      { body: "Padrões Seedance para ads curtos, reveals e storyboard em movimento.", media: "Vídeo" },
      { body: "Prompts exploratórios para social, memes e iteração rápida.", media: "Imagem" },
      { body: "Templates Gemini para planejamento multimodal e variantes criativas.", media: "Imagem" },
    ],
    modelHint: "Cards temporários enquanto conectamos o catálogo real.",
    modelTitle: "Explorar por modelo",
    moreTitle: "Mais ferramentas de prompt",
    openPrompt: "Abrir prompt",
    promptUnit: "prompts",
    searchBody: "Pesquise por tarefa, modelo, saída ou palavra visual e envie o prompt para o console Flatkey.",
    searchTitle: "Busca de prompts IA",
    secondaryFeatureBody: "Transforme uma imagem de referência em prompt reutilizável para a próxima geração.",
    secondaryFeatureTitle: "Imagem para prompt",
    smallFeatureLinks: ["Prompts GPT Image", "Prompts Seedance", "Editor de imagem", "Editor de vídeo", "Coleções", "Ferramentas IA"],
    tagUnit: "palavras-chave",
    videoIndexTitle: "Índice de prompts de vídeo",
    weeklyHint: "Amostra rotativa de pares prompt-resultado da biblioteca local.",
    weeklyTitle: "Populares da semana",
    keywordGroups: {
      image: [
        { label: "Uso", words: ["imagem ecommerce", "pôster de anúncio", "screenshot app", "ficha personagem"] },
        { label: "Estilo", words: ["foto limpa", "liquid glass", "colagem editorial", "concept board"] },
        { label: "Assunto", words: ["frasco skincare", "tênis", "retrato avatar", "prop de jogo"] },
      ],
      video: [
        { label: "Uso", words: ["UGC ad", "reveal produto", "lançamento local", "movimento referência"] },
        { label: "Movimento", words: ["push-in lento", "handheld", "parallax", "take único"] },
        { label: "Assunto", words: ["produto", "personagem", "variante mercado", "poster de clip"] },
      ],
    },
  },
  ru: {
    aboutBody:
      "Flatkey хранит reusable image и video prompts вместе с готовым результатом. Сравните keywords, models и outputs перед запуском в playground.",
    aboutTitle: "Что такое prompt library Flatkey?",
    categoryTitle: "По категориям",
    collectionEyebrow: "Подборки",
    collectionTitle: "Избранные коллекции",
    collections: [
      { title: "Product image starter pack", body: "Prompts для marketplace, app-store, posters и product hero с готовыми image outputs.", meta: "Image prompts" },
      { title: "Seedance video production set", body: "Short video prompts для reveal, UGC ads, localization и reference motion tests.", meta: "Video prompts" },
      { title: "Character and scene boards", body: "Prompt-output пары для character sheets, props, scene bibles и storyboards.", meta: "Image + video planning" },
    ],
    finalCtaBody: "Откройте playground и запустите prompts через image и video routes Flatkey.",
    finalCtaButton: "Начать в Flatkey",
    finalCtaTitle: "Бесплатно для creators, навсегда.",
    heroAccent: "library",
    heroBadge: "20 000+ prompts - image и video",
    heroBody: "Публичный prompt index для creators, которым нужны примеры с видимыми outputs.",
    heroStats: ["20 000+ prompt rows", "Только image и video", "Source и output вместе"],
    heroTitle: "AI prompt",
    imageIndexTitle: "Image prompt index",
    mediaCards: {
      image: { body: "Image prompts для ecommerce, posters, character sheets, UI и campaigns.", keywords: ["product", "poster", "avatar", "storyboard"], label: "Image" },
      video: { body: "Video prompts для UGC clips, product reveal, localized variants, image-to-video и motion.", keywords: ["UGC", "reveal", "i2v", "motion"], label: "Video" },
    },
    mediaTitle: "По медиа",
    modelCards: [
      { body: "Commercial image generation с faithful product compositions.", media: "Image" },
      { body: "Быстрые drafts для characters, products и posters.", media: "Image" },
      { body: "Balanced image generation для ads, boards и localized visuals.", media: "Image" },
      { body: "Classic OpenAI image route для migration tests.", media: "Image" },
      { body: "Image-to-video и character motion с identity control.", media: "Video" },
      { body: "Seedance patterns для short ads, reveal shots и storyboard motion.", media: "Video" },
      { body: "Exploratory image prompts для social и fast iteration.", media: "Image" },
      { body: "Gemini templates для multimodal planning и variants.", media: "Image" },
    ],
    modelHint: "Временные model cards до подключения live catalog.",
    modelTitle: "По моделям",
    moreTitle: "Другие prompt tools",
    openPrompt: "Открыть prompt",
    promptUnit: "prompts",
    searchBody: "Ищите prompts по задаче, model, output type или visual keyword и отправляйте их в Flatkey console.",
    searchTitle: "AI prompt search",
    secondaryFeatureBody: "Превратите reference image в reusable prompt для следующей генерации.",
    secondaryFeatureTitle: "Image to prompt",
    smallFeatureLinks: ["GPT Image prompts", "Seedance prompts", "Image editor", "Video editor", "Prompt collections", "AI tools"],
    tagUnit: "keywords",
    videoIndexTitle: "Video prompt index",
    weeklyHint: "Ротационная выборка prompt-output pairs из local library.",
    weeklyTitle: "Популярное за неделю",
    keywordGroups: {
      image: [
        { label: "Use case", words: ["ecommerce image", "ad poster", "app screenshot", "character sheet"] },
        { label: "Style", words: ["clean product photo", "liquid glass", "editorial collage", "concept board"] },
        { label: "Subject", words: ["skincare bottle", "sports shoe", "avatar portrait", "game prop"] },
      ],
      video: [
        { label: "Use case", words: ["UGC ad", "product reveal", "localized launch", "reference motion"] },
        { label: "Motion", words: ["slow push-in", "handheld pickup", "parallax", "single take"] },
        { label: "Subject", words: ["consumer product", "character", "market variant", "clip poster"] },
      ],
    },
  },
  ja: {
    aboutBody:
      "Flatkey は再利用できる画像・動画プロンプトを、実際に生成された出力と一緒に保存します。playground でクレジットを使う前に、キーワード、モデル、成果物を比較できます。",
    aboutTitle: "Flatkey AI プロンプトライブラリとは？",
    categoryTitle: "カテゴリ別に見る",
    collectionEyebrow: "注目セット",
    collectionTitle: "注目コレクション",
    collections: [
      { title: "商品画像スターターパック", body: "マーケットプレイス、アプリストア、ポスター、商品 hero 向けの画像プロンプトと出力例。", meta: "画像プロンプト" },
      { title: "Seedance 動画制作セット", body: "商品リビール、UGC 広告、ローカライズ、参照画像モーション向けの短尺動画プロンプト。", meta: "動画プロンプト" },
      { title: "キャラクター・シーンボード", body: "キャラクター設定、props、scene bible、storyboard に使える prompt-output ペア。", meta: "画像 + 動画設計" },
    ],
    finalCtaBody: "playground を開き、Flatkey の画像・動画ルートでプロンプトを実行できます。",
    finalCtaButton: "Flatkey を始める",
    finalCtaTitle: "クリエイターには永久無料。",
    heroAccent: "ライブラリ",
    heroBadge: "20,000+ プロンプト - 画像と動画",
    heroBody: "成果物を見ながら選べる、クリエイター向けの公開プロンプト索引です。",
    heroStats: ["20,000+ 行", "画像と動画のみ", "出典と出力を表示"],
    heroTitle: "AI プロンプト",
    imageIndexTitle: "画像プロンプト索引",
    mediaCards: {
      image: { body: "EC、ポスター、キャラクター設定、UI、キャンペーン素材向けの画像プロンプト。", keywords: ["商品", "ポスター", "アバター", "storyboard"], label: "画像" },
      video: { body: "UGC、商品リビール、ローカライズ、image-to-video、motion test 向けの動画プロンプト。", keywords: ["UGC", "reveal", "i2v", "motion"], label: "動画" },
    },
    mediaTitle: "メディア別に見る",
    modelCards: [
      { body: "商用画像と編集しやすい商品構図を高精度に生成。", media: "画像" },
      { body: "キャラクター、商品、ポスターの方向出しに使える高速ドラフト。", media: "画像" },
      { body: "広告、concept board、ローカライズ素材向けのバランス型画像生成。", media: "画像" },
      { body: "既存の画像プロンプト移行テスト向け OpenAI ルート。", media: "画像" },
      { body: "image-to-video とキャラクターモーションの identity control。", media: "動画" },
      { body: "短尺広告、reveal shot、storyboard motion 向け Seedance パターン。", media: "動画" },
      { body: "SNS、ミーム、短時間の探索向け画像プロンプト。", media: "画像" },
      { body: "マルチモーダル計画と創意バリエーション向け Gemini テンプレート。", media: "画像" },
    ],
    modelHint: "実カタログ接続までの一時的なモデルカードです。",
    modelTitle: "モデル別に見る",
    moreTitle: "その他のプロンプト機能",
    openPrompt: "開く",
    promptUnit: "プロンプト",
    searchBody: "タスク、モデル、出力タイプ、視覚キーワードで検索し、Flatkey コンソールへ送れます。",
    searchTitle: "AI プロンプト検索",
    secondaryFeatureBody: "参照画像から、次の画像・動画生成で使える再利用プロンプトを作ります。",
    secondaryFeatureTitle: "画像からプロンプト",
    smallFeatureLinks: ["GPT Image プロンプト", "Seedance プロンプト", "画像プロンプト編集", "動画プロンプト編集", "コレクション", "AI ツール"],
    tagUnit: "キーワード",
    videoIndexTitle: "動画プロンプト索引",
    weeklyHint: "ローカルライブラリから選んだ prompt-output ペアのサンプルです。",
    weeklyTitle: "今週の人気",
    keywordGroups: {
      image: [
        { label: "用途", words: ["EC 商品画像", "広告ポスター", "アプリ画面", "キャラクター設定"] },
        { label: "スタイル", words: ["商品写真", "liquid glass", "編集風コラージュ", "concept board"] },
        { label: "主体", words: ["スキンケア瓶", "スポーツシューズ", "アバター肖像", "ゲーム prop"] },
      ],
      video: [
        { label: "用途", words: ["UGC 広告", "商品 reveal", "ローカライズ", "参照画像モーション"] },
        { label: "動き", words: ["slow push-in", "handheld", "parallax", "single take"] },
        { label: "主体", words: ["商品", "キャラクター", "市場別 variant", "clip poster"] },
      ],
    },
  },
  vi: {
    aboutBody:
      "Flatkey lưu prompt hình ảnh và video cùng output đã tạo. Bạn có thể so sánh keyword, model và artifact trước khi dùng credit trong playground.",
    aboutTitle: "Thư viện prompt AI của Flatkey là gì?",
    categoryTitle: "Duyệt theo danh mục",
    collectionEyebrow: "Bộ chọn lọc",
    collectionTitle: "Bộ sưu tập nổi bật",
    collections: [
      { title: "Bộ prompt ảnh sản phẩm", body: "Prompt marketplace, app-store, poster và product hero kèm output ảnh dùng được.", meta: "Prompt hình ảnh" },
      { title: "Bộ sản xuất video Seedance", body: "Prompt video ngắn cho product reveal, UGC ads, bản địa hóa và motion test.", meta: "Prompt video" },
      { title: "Nhân vật và scene board", body: "Cặp prompt-output cho character sheet, prop, scene bible và storyboard.", meta: "Ảnh + kế hoạch video" },
    ],
    finalCtaBody: "Mở playground và chạy prompt qua route hình ảnh hoặc video của Flatkey.",
    finalCtaButton: "Bắt đầu với Flatkey",
    finalCtaTitle: "Miễn phí mãi mãi cho creator.",
    heroAccent: "library",
    heroBadge: "20.000+ prompt - hình ảnh và video",
    heroBody: "Chỉ mục prompt công khai cho creator cần ví dụ có output nhìn thấy được.",
    heroStats: ["20.000+ dòng prompt", "Chỉ ảnh và video", "Nguồn kèm output"],
    heroTitle: "AI prompt",
    imageIndexTitle: "Chỉ mục prompt hình ảnh",
    mediaCards: {
      image: { body: "Prompt ảnh cho ecommerce, poster, character sheet, UI và campaign.", keywords: ["product", "poster", "avatar", "storyboard"], label: "Hình ảnh" },
      video: { body: "Prompt video cho UGC, reveal, local variant, image-to-video và motion.", keywords: ["UGC", "reveal", "i2v", "motion"], label: "Video" },
    },
    mediaTitle: "Duyệt theo media",
    modelCards: [
      { body: "Ảnh thương mại độ trung thực cao và bố cục sản phẩm dễ chỉnh.", media: "Hình ảnh" },
      { body: "Draft nhanh cho nhân vật, sản phẩm và poster.", media: "Hình ảnh" },
      { body: "Tạo ảnh cân bằng cho ads, concept board và visual bản địa hóa.", media: "Hình ảnh" },
      { body: "Route OpenAI cổ điển để test migration prompt ảnh.", media: "Hình ảnh" },
      { body: "Image-to-video và character motion với kiểm soát identity.", media: "Video" },
      { body: "Pattern Seedance cho ads ngắn, reveal shot và storyboard motion.", media: "Video" },
      { body: "Prompt ảnh khám phá cho social, meme và iteration nhanh.", media: "Hình ảnh" },
      { body: "Template Gemini cho planning đa phương thức và creative variant.", media: "Hình ảnh" },
    ],
    modelHint: "Model card tạm thời trong lúc nối catalog thật.",
    modelTitle: "Duyệt theo model",
    moreTitle: "Công cụ prompt khác",
    openPrompt: "Mở prompt",
    promptUnit: "prompt",
    searchBody: "Tìm theo task, model, loại output hoặc visual keyword rồi gửi prompt vào console Flatkey.",
    searchTitle: "Tìm prompt AI",
    secondaryFeatureBody: "Biến ảnh tham chiếu thành prompt tái sử dụng cho lần tạo ảnh hoặc video tiếp theo.",
    secondaryFeatureTitle: "Ảnh sang prompt",
    smallFeatureLinks: ["Prompt GPT Image", "Prompt Seedance", "Editor ảnh", "Editor video", "Bộ prompt", "AI tools"],
    tagUnit: "keyword",
    videoIndexTitle: "Chỉ mục prompt video",
    weeklyHint: "Mẫu prompt-output lấy từ thư viện local hiện tại.",
    weeklyTitle: "Hot trong tuần",
    keywordGroups: {
      image: [
        { label: "Use case", words: ["ảnh ecommerce", "poster quảng cáo", "app screenshot", "character sheet"] },
        { label: "Style", words: ["product photo", "liquid glass", "editorial collage", "concept board"] },
        { label: "Subject", words: ["chai skincare", "giày thể thao", "avatar portrait", "game prop"] },
      ],
      video: [
        { label: "Use case", words: ["UGC ad", "product reveal", "localized launch", "reference motion"] },
        { label: "Motion", words: ["slow push-in", "handheld pickup", "parallax", "single take"] },
        { label: "Subject", words: ["consumer product", "character", "market variant", "clip poster"] },
      ],
    },
  },
  de: {
    aboutBody:
      "Flatkey speichert wiederverwendbare Bild- und Video-Prompts zusammen mit dem erzeugten Ergebnis. Vergleiche Keywords, Modelle und Outputs vor dem Credit-Verbrauch.",
    aboutTitle: "Was ist die Flatkey AI Prompt Library?",
    categoryTitle: "Nach Kategorie",
    collectionEyebrow: "Kuratierte Sets",
    collectionTitle: "Featured Collections",
    collections: [
      { title: "Product Image Starter Pack", body: "Marketplace-, App-Store-, Poster- und Product-Hero-Prompts mit nutzbaren Bild-Outputs.", meta: "Bild-Prompts" },
      { title: "Seedance Video Production Set", body: "Kurzvideo-Prompts für Reveal, UGC Ads, Lokalisierung und Referenzbewegung.", meta: "Video-Prompts" },
      { title: "Character und Scene Boards", body: "Prompt-Output-Paare für Character Sheets, Props, Scene Bibles und Storyboards.", meta: "Bild + Video-Planung" },
    ],
    finalCtaBody: "Öffne den Playground und führe Prompts über Flatkeys Bild- und Video-Routen aus.",
    finalCtaButton: "Mit Flatkey starten",
    finalCtaTitle: "Für Creator kostenlos, dauerhaft.",
    heroAccent: "Library",
    heroBadge: "20.000+ Prompts - Bild und Video",
    heroBody: "Ein öffentlicher Prompt-Index für Creator, die Beispiele mit sichtbaren Ergebnissen brauchen.",
    heroStats: ["20.000+ Prompt-Zeilen", "Nur Bild und Video", "Quelle und Output gepaart"],
    heroTitle: "AI Prompt",
    imageIndexTitle: "Bild-Prompt-Index",
    mediaCards: {
      image: { body: "Bild-Prompts für Ecommerce, Poster, Character Sheets, UI und Kampagnen.", keywords: ["Produkt", "Poster", "Avatar", "Storyboard"], label: "Bild" },
      video: { body: "Video-Prompts für UGC, Reveal, lokale Varianten, Image-to-Video und Motion.", keywords: ["UGC", "Reveal", "i2v", "Motion"], label: "Video" },
    },
    mediaTitle: "Nach Medium",
    modelCards: [
      { body: "Hochwertige kommerzielle Bilder und editierbare Produktkompositionen.", media: "Bild" },
      { body: "Schnelle Entwürfe für Characters, Produkte und Poster.", media: "Bild" },
      { body: "Ausgewogene Bildgenerierung für Ads, Concept Boards und Lokalisierung.", media: "Bild" },
      { body: "Klassische OpenAI-Bildroute für Migrationstests.", media: "Bild" },
      { body: "Image-to-Video und Character Motion mit Identity Control.", media: "Video" },
      { body: "Seedance Patterns für Short Ads, Reveal Shots und Storyboard Motion.", media: "Video" },
      { body: "Explorative Bild-Prompts für Social, Memes und schnelle Iteration.", media: "Bild" },
      { body: "Gemini Templates für multimodale Planung und Creative Variants.", media: "Bild" },
    ],
    modelHint: "Temporäre Model Cards, bis der Live-Katalog angebunden ist.",
    modelTitle: "Nach Modell",
    moreTitle: "Mehr Prompt Tools",
    openPrompt: "Prompt öffnen",
    promptUnit: "Prompts",
    searchBody: "Suche nach Aufgabe, Modell, Output-Typ oder visuellem Keyword und sende den Prompt an die Flatkey-Konsole.",
    searchTitle: "AI Prompt Search",
    secondaryFeatureBody: "Wandle ein Referenzbild in einen wiederverwendbaren Prompt für den nächsten Lauf um.",
    secondaryFeatureTitle: "Image to Prompt",
    smallFeatureLinks: ["GPT Image Prompts", "Seedance Prompts", "Bild-Editor", "Video-Editor", "Prompt Collections", "AI Tools"],
    tagUnit: "Keywords",
    videoIndexTitle: "Video-Prompt-Index",
    weeklyHint: "Rotierende Auswahl von Prompt-Output-Paaren aus der lokalen Library.",
    weeklyTitle: "Weekly Hot",
    keywordGroups: {
      image: [
        { label: "Use Case", words: ["Ecommerce-Bild", "Ad Poster", "App Screenshot", "Character Sheet"] },
        { label: "Stil", words: ["Produktfoto", "Liquid Glass", "Editorial Collage", "Concept Board"] },
        { label: "Motiv", words: ["Skincare Bottle", "Sportschuh", "Avatar Portrait", "Game Prop"] },
      ],
      video: [
        { label: "Use Case", words: ["UGC Ad", "Product Reveal", "Localized Launch", "Reference Motion"] },
        { label: "Motion", words: ["Slow Push-in", "Handheld Pickup", "Parallax", "Single Take"] },
        { label: "Motiv", words: ["Produkt", "Character", "Market Variant", "Clip Poster"] },
      ],
    },
  },
});

const modelNames = [
  "GPT Image 2",
  "Nano Banana Pro",
  "Seedream 4.5",
  "GPT Image 1.5",
  "Seedance 2.5",
  "Seedance 2.0",
  "Grok Imagine",
  "Gemini 3 Pro",
];

const modelIconByName: Record<string, string> = {
  "GPT Image 1.5": "/logos/openai.svg",
  "GPT Image 2": "/logos/openai.svg",
  "Gemini 3 Pro": "/logos/googlegemini.svg",
  "Grok Imagine": "/assets/logos/x.svg",
  "Nano Banana Pro": "/logos/googlegemini.svg",
  "Seedance 2.0": "/logos/bytedance.svg",
  "Seedance 2.5": "/logos/bytedance.svg",
  "Seedream 4.5": "/logos/bytedance.svg",
};

const modelAccentByIndex = ["#5B21B6", "#7C3AED", "#4C1D95", "#0B0B0F", "#15803D", "#5B21B6", "#83838E", "#7C3AED"];

export function PlaygroundPromptsExplorer(props: { items: PromptItem[]; locale: Locale }) {
  const copy = copyByLocale[props.locale] ?? copyByLocale.en;
  const libraryCopy = promptLibraryCopy[props.locale] ?? promptLibraryCopy.en;
  const items = mergePromptItems([
    ...props.items,
    ...getCliMediaPromptItems("image"),
    ...getCliMediaPromptItems("video"),
  ]);
  const localVisualItems = sortPromptItems(items.filter(hasLocalVisualMedia));
  const imageItems = localVisualItems.filter((item) => item.category === "image");
  const videoItems = localVisualItems.filter((item) => item.category === "video");
  const heroItem = imageItems.find((item) => item.slug === "awesome-images-saas-hero-phone") ?? imageItems[0] ?? localVisualItems[0];
  const weeklyItems = preferSlugs(localVisualItems, [
    "awesome-images-marketplace-main-image",
    "ugc-paid-social-product-clip",
    "awesome-images-liquid-bento-infographic",
    "cinematic-product-reveal-video",
    "awesome-images-consistent-avatar",
    "localized-market-product-variant-video",
    "awesome-images-event-poster-key-visual",
    "flatkey-image-to-video-product-scene",
  ]).slice(0, 8);
  const collectionItems = [
    imageItems.filter((item) => item.tags.includes("ecommerce") || item.tags.includes("product")).slice(0, 3),
    videoItems.slice(0, 3),
    imageItems.filter((item) => item.tags.includes("character") || item.tags.includes("storyboard") || item.tags.includes("prop")).slice(0, 3),
  ];
  const mediaCounts: Record<MediaKind, number> = {
    image: imageItems.length,
    video: videoItems.length,
  };

  return (
    <>
      <style>{`
        .promptIndexPage{--pi-ink:var(--ink,#0b0b0f);--pi-muted:var(--ink2,#43434c);--pi-faint:var(--ink3,#83838e);--pi-paper:var(--paper,#fff);--pi-paper2:var(--paper2,#f7f6fb);--pi-line:var(--line,#0b0b0f14);--pi-line2:var(--line2,#0b0b0f0a);--pi-violet:var(--violet,#5b21b6);--pi-violet-deep:var(--violet-deep,#4c1d95);--pi-violet-hi:var(--violet-hi,#7c3aed);--pi-violet-soft:var(--violet-tint,#f0ebfa);--pi-green:var(--green,#15803d);--pi-green-soft:var(--green-tint,#e7f4ec);--pi-dark:var(--dark,#0a0a10);--pi-shadow:0 26px 70px -50px rgba(46,16,101,.36);background:var(--pi-paper);color:var(--pi-ink);letter-spacing:0}
        .promptIndexPage *{letter-spacing:0}
        .promptIndexPage .btn.black{background:var(--pi-violet);color:#fff;box-shadow:0 18px 42px -28px rgba(91,33,182,.72)}
        .promptIndexPage .btn.white{background:#fff;color:var(--pi-ink);box-shadow:inset 0 0 0 1px var(--pi-line),0 16px 36px -30px rgba(30,27,75,.28)}
        .promptIndexHero{position:relative;overflow:hidden;border-bottom:1px solid var(--pi-line);background:linear-gradient(180deg,#fff 0%,var(--pi-paper2) 100%)}
        .promptIndexHero:before{content:"";position:absolute;inset:0;pointer-events:none;background-image:linear-gradient(to right,rgba(91,33,182,.07) 1px,transparent 1px),linear-gradient(to bottom,rgba(91,33,182,.06) 1px,transparent 1px);background-size:56px 56px;mask-image:linear-gradient(to bottom,rgba(0,0,0,.7),transparent 88%)}
        .promptIndexHeroIn{position:relative;z-index:1;display:grid;grid-template-columns:minmax(0,1fr) minmax(340px,520px);gap:44px;align-items:center;width:100%;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:82px var(--fk-site-gutter) 70px}
        .promptIndexEyebrow{display:inline-flex;align-items:center;gap:8px;border:1px solid rgba(91,33,182,.16);background:rgba(255,255,255,.82);border-radius:999px;padding:7px 12px;color:var(--pi-violet-deep);font:800 12px/1 var(--mono);box-shadow:0 14px 34px -28px rgba(76,29,149,.62);backdrop-filter:blur(12px)}
        .promptIndexTitle{margin-top:18px;max-width:680px;font-family:var(--disp);font-size:clamp(50px,6vw,86px);line-height:.98;font-weight:900;text-wrap:balance}
        .promptIndexTitle span{display:block;color:var(--pi-violet)}
        .promptIndexBody{max-width:620px;margin-top:20px;color:var(--pi-muted);font-size:16.5px;line-height:1.72}
        .promptIndexStats{display:flex;flex-wrap:wrap;gap:9px;margin-top:26px}
        .promptIndexStats span{display:inline-flex;align-items:center;min-height:31px;border:1px solid var(--pi-line);border-radius:999px;background:#fff;padding:0 12px;color:var(--pi-muted);font:800 11px/1 var(--mono);box-shadow:0 12px 28px -26px rgba(46,16,101,.34)}
        .promptIndexCtas{display:flex;flex-wrap:wrap;gap:12px;margin-top:30px}
        .promptIndexCtas .btn{border-radius:8px}
        .promptIndexHeroArt{position:relative;border:1px solid rgba(91,33,182,.18);border-radius:8px;background:#fff;box-shadow:0 30px 78px -48px rgba(46,16,101,.52);overflow:hidden}
        .promptIndexHeroFrame{position:relative;aspect-ratio:16/11;background:var(--pi-violet-soft)}
        .promptIndexHeroFrame img,.promptIndexHeroFrame video{display:block;width:100%;height:100%;object-fit:cover}
        .promptIndexHeroMeta{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px;align-items:center;border-top:1px solid var(--pi-line);padding:13px 14px;background:#fff}
        .promptIndexHeroMeta b{font-size:13.5px;line-height:1.2}
        .promptIndexHeroMeta span{border:1px solid rgba(91,33,182,.16);border-radius:6px;background:var(--pi-violet-soft);padding:5px 8px;color:var(--pi-violet-deep);font:850 10px/1 var(--mono)}
        .promptIndexBand{border-bottom:1px solid var(--pi-line);background:#fff}
        .promptIndexBand.soft{background:var(--pi-paper2)}
        .promptIndexBand.mint{background:linear-gradient(180deg,var(--pi-paper2) 0%,#fff 100%)}
        .promptIndexBand.cta{background:radial-gradient(120% 160% at 50% -20%,var(--pi-violet) 0%,#3b0fa0 45%,#2e1065 100%);color:#fff}
        .promptIndexIn{width:100%;max-width:var(--fk-site-frame-max-width);margin:0 auto;padding:62px var(--fk-site-gutter)}
        .promptIndexHead{display:flex;align-items:end;justify-content:space-between;gap:24px;margin-bottom:24px}
        .promptIndexHead h2{font-family:var(--disp);font-size:30px;line-height:1.1;font-weight:900}
        .promptIndexHead p{max-width:440px;color:var(--pi-muted);font-size:13.5px;line-height:1.6;text-align:right}
        .promptIndexKicker{display:inline-flex;align-items:center;gap:7px;margin-bottom:9px;color:var(--pi-violet-deep);font:900 12px/1 var(--mono)}
        .collectionGrid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}
        .collectionCard{display:flex;flex-direction:column;min-height:310px;border:1px solid var(--pi-line);border-radius:8px;background:#fff;color:var(--pi-ink);text-decoration:none;box-shadow:var(--pi-shadow);overflow:hidden;transition:transform .16s ease,border-color .16s ease,box-shadow .16s ease}
        .collectionCard:hover,.weeklyCard:hover,.mediaBrowseCard:hover,.modelBrowseCard:hover{transform:translateY(-2px)}
        .collectionCard:hover,.weeklyCard:hover,.mediaBrowseCard:hover,.modelBrowseCard:hover{border-color:rgba(91,33,182,.32);box-shadow:0 30px 78px -48px rgba(46,16,101,.62)}
        .collectionTop{display:flex;align-items:center;justify-content:space-between;gap:12px;border-bottom:1px solid var(--pi-line);padding:13px 14px}
        .collectionTop span{color:var(--pi-violet-deep);font:800 10px/1 var(--mono)}
        .collectionPreview{display:grid;grid-template-columns:1.1fr .9fr;gap:8px;padding:12px;background:var(--pi-paper2)}
        .collectionThumb{position:relative;overflow:hidden;border:1px solid rgba(91,33,182,.13);border-radius:6px;background:#fff;aspect-ratio:4/3}
        .collectionThumb.tall{grid-row:span 2;aspect-ratio:3/4}
        .collectionThumb img,.collectionThumb video{display:block;width:100%;height:100%;object-fit:cover}
        .collectionBody{padding:16px 16px 18px}
        .collectionBody h3{font-family:var(--disp);font-size:19px;line-height:1.18;font-weight:900}
        .collectionBody p{margin-top:8px;color:var(--pi-muted);font-size:13px;line-height:1.55}
        .collectionFoot{display:flex;align-items:center;justify-content:space-between;margin-top:14px;color:var(--pi-violet-deep);font:850 11px/1 var(--mono)}
        .weeklyGrid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}
        .weeklyCard{display:block;overflow:hidden;border:1px solid var(--pi-line);border-radius:8px;background:#fff;color:var(--pi-ink);text-decoration:none;box-shadow:0 24px 66px -52px rgba(46,16,101,.48);transition:transform .16s ease,border-color .16s ease,box-shadow .16s ease}
        .weeklyMedia{position:relative;aspect-ratio:4/3;background:var(--pi-violet-soft);overflow:hidden}
        .weeklyMedia img,.weeklyMedia video{display:block;width:100%;height:100%;object-fit:cover}
        .weeklyBody{border-top:1px solid var(--pi-line);padding:10px 11px 12px}
        .weeklyBody h3{font-size:13.5px;line-height:1.25;font-weight:900}
        .weeklyMeta{display:flex;justify-content:space-between;gap:8px;margin-top:7px;color:var(--pi-faint);font:800 10px/1 var(--mono)}
        .videoChip{position:absolute;right:9px;bottom:9px;display:grid;place-items:center;width:31px;height:31px;border:1px solid rgba(255,255,255,.62);border-radius:999px;background:rgba(255,255,255,.92);color:var(--pi-violet);box-shadow:0 12px 30px -20px rgba(0,0,0,.45);backdrop-filter:blur(8px)}
        .mediaBrowseGrid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}
        .mediaBrowseCard{display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:16px;align-items:center;min-height:132px;border:1px solid var(--pi-line);border-radius:8px;background:#fff;color:var(--pi-ink);text-decoration:none;padding:18px;box-shadow:var(--pi-shadow);transition:transform .16s ease,border-color .16s ease,box-shadow .16s ease}
        .mediaIcon{display:grid;place-items:center;width:46px;height:46px;border:1px solid rgba(91,33,182,.18);border-radius:8px;background:var(--pi-violet-soft);color:var(--pi-violet)}
        .mediaBrowseCard.video .mediaIcon{border-color:rgba(21,128,61,.18);background:var(--pi-green-soft);color:var(--pi-green)}
        .mediaBrowseCard h3{font-family:var(--disp);font-size:25px;line-height:1.05;font-weight:900}
        .mediaBrowseCard p{margin-top:7px;color:var(--pi-muted);font-size:13px;line-height:1.5}
        .mediaTags{display:flex;flex-wrap:wrap;gap:5px;margin-top:10px}
        .mediaTags span{border:1px solid var(--pi-line);border-radius:999px;background:var(--pi-paper2);padding:4px 7px;color:var(--pi-muted);font:800 10px/1 var(--mono)}
        .mediaArrow{display:grid;place-items:center;width:36px;height:36px;border-radius:999px;background:var(--pi-violet);color:#fff}
        .mediaBrowseCard.video .mediaArrow{background:var(--pi-green)}
        .modelGrid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}
        .modelBrowseCard{min-height:156px;border:1px solid var(--pi-line);border-radius:8px;background:#fff;color:var(--pi-ink);text-decoration:none;padding:16px;box-shadow:0 24px 66px -52px rgba(46,16,101,.48);transition:transform .16s ease,border-color .16s ease,box-shadow .16s ease}
        .modelIconRow{display:flex;align-items:center;justify-content:space-between;gap:12px}
        .modelIconBox{display:grid;place-items:center;width:39px;height:39px;border:1px solid rgba(91,33,182,.18);border-radius:8px;background:#fff}
        .modelIconBox img{display:block;width:22px;height:22px;object-fit:contain}
        .modelBadge{border-radius:999px;background:var(--pi-green-soft);padding:5px 7px;color:var(--pi-green);font:850 9px/1 var(--mono)}
        .modelBrowseCard h3{margin-top:13px;font-size:15px;font-weight:950;line-height:1.2}
        .modelBrowseCard p{margin-top:7px;color:var(--pi-muted);font-size:12.2px;line-height:1.48}
        .modelFoot{display:flex;align-items:center;justify-content:space-between;margin-top:13px;color:var(--pi-violet-deep);font:850 10px/1 var(--mono)}
        .keywordGrid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18px}
        .keywordPanel{border:1px solid var(--pi-line);border-radius:8px;background:#fff;box-shadow:var(--pi-shadow);overflow:hidden}
        .keywordPanelHead{display:flex;align-items:center;gap:9px;border-bottom:1px solid var(--pi-line);padding:14px 16px}
        .keywordDot{width:8px;height:8px;border-radius:999px;background:var(--pi-violet)}
        .keywordPanel.video .keywordDot{background:var(--pi-green)}
        .keywordPanelHead h3{font-family:var(--disp);font-size:18px;font-weight:900}
        .keywordColumns{display:grid;grid-template-columns:repeat(3,minmax(0,1fr))}
        .keywordCol{min-height:170px;padding:18px 16px;border-right:1px solid var(--pi-line)}
        .keywordCol:last-child{border-right:0}
        .keywordCol b{display:block;color:#d8cdf2;font:900 28px/1 var(--mono)}
        .keywordCol h4{margin-top:12px;font-size:13px;font-weight:950}
        .keywordCol ul{list-style:none;margin-top:9px;display:grid;gap:6px}
        .keywordCol li{color:var(--pi-muted);font-size:12px;line-height:1.35}
        .aboutGrid{display:grid;grid-template-columns:minmax(260px,.6fr) minmax(0,1fr);gap:44px;align-items:start}
        .aboutGrid h2{font-family:var(--disp);font-size:27px;line-height:1.15;font-weight:950}
        .aboutGrid p{color:var(--pi-muted);font-size:14.5px;line-height:1.75}
        .featureBox{border:1px solid var(--pi-line);border-radius:8px;background:#fff;box-shadow:var(--pi-shadow);padding:22px}
        .featureRows{display:grid;gap:22px}
        .featureRow{display:grid;grid-template-columns:minmax(0,1fr) minmax(220px,.62fr);gap:24px;align-items:center;border-bottom:1px dashed rgba(91,33,182,.18);padding-bottom:22px}
        .featureRow:last-child{border-bottom:0;padding-bottom:0}
        .featureRow h3{font-family:var(--disp);font-size:28px;line-height:1.1;font-weight:950}
        .featureRow p{margin-top:9px;color:var(--pi-muted);font-size:13.5px;line-height:1.62}
        .featureMock{min-height:132px;border:1px solid rgba(91,33,182,.16);border-radius:8px;background:var(--pi-violet-soft);padding:16px}
        .searchBarMock{display:flex;align-items:center;gap:10px;height:42px;border:1px solid rgba(91,33,182,.16);border-radius:8px;background:#fff;padding:0 12px;color:var(--pi-muted);font:800 11px/1 var(--mono)}
        .featureSwatch{height:94px;border:1px solid rgba(91,33,182,.14);border-radius:8px;background:linear-gradient(135deg,var(--pi-dark) 0%,#1b1033 42%,var(--pi-violet) 74%,var(--pi-green-soft) 100%)}
        .smallFeatureGrid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:9px}
        .smallFeatureGrid a{display:flex;align-items:center;justify-content:space-between;gap:8px;border-bottom:1px solid var(--pi-line);padding:11px 2px;color:var(--pi-ink);text-decoration:none;font:850 12px/1.2 var(--sans)}
        .smallFeatureGrid a:hover{color:var(--pi-violet-deep)}
        .finalCta{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:24px;align-items:center}
        .finalCta h2{font-family:var(--disp);font-size:37px;line-height:1.08;font-weight:950}
        .finalCta p{margin-top:8px;max-width:680px;color:#ddd3f9;font-size:14px;line-height:1.6}
        .finalCta .btn.black{background:#fff;color:var(--pi-violet-deep);box-shadow:0 18px 42px -28px rgba(0,0,0,.72)}
        @media(max-width:1080px){.promptIndexHeroIn{grid-template-columns:1fr}.promptIndexHeroArt{max-width:680px}.collectionGrid,.modelGrid,.weeklyGrid{grid-template-columns:repeat(2,minmax(0,1fr))}.aboutGrid,.featureRow{grid-template-columns:1fr}}
        @media(max-width:760px){.promptIndexHeroIn{padding:58px var(--fk-site-gutter) 52px}.promptIndexTitle{font-size:44px;line-height:1.04}.promptIndexBody{font-size:15px}.promptIndexHead{display:block}.promptIndexHead p{text-align:left;margin-top:8px}.collectionGrid,.weeklyGrid,.mediaBrowseGrid,.modelGrid,.keywordGrid{grid-template-columns:1fr}.mediaBrowseCard{grid-template-columns:auto minmax(0,1fr);align-items:start}.mediaArrow{grid-column:2;justify-self:start}.keywordColumns{grid-template-columns:1fr}.keywordCol{border-right:0;border-bottom:1px solid var(--pi-line)}.keywordCol:last-child{border-bottom:0}.featureBox{padding:17px}.finalCta{grid-template-columns:1fr}.finalCta h2{font-size:30px}.promptIndexCtas{flex-direction:column}.promptIndexCtas .btn{width:100%;white-space:normal;text-align:center}.smallFeatureGrid{grid-template-columns:1fr}}
      `}</style>
      <div className="promptIndexPage">
        <header className="promptIndexHero">
          <div className="promptIndexHeroIn">
            <div>
              <span className="promptIndexEyebrow">
                <Sparkles size={14} />
                {copy.heroBadge}
              </span>
              <h1 className="promptIndexTitle">
                {copy.heroTitle}
                <span>{copy.heroAccent}</span>
              </h1>
              <p className="promptIndexBody">{copy.heroBody}</p>
              <div className="promptIndexStats">
                {copy.heroStats.map((stat) => (
                  <span key={stat}>{stat}</span>
                ))}
              </div>
              <div className="promptIndexCtas">
                <Link className="btn black" href={localizePath(CLI_IMAGE_PATH, props.locale)}>
                  <ImageIcon size={16} />
                  {copy.mediaCards.image.label}
                </Link>
                <a className="btn white" href={consoleUrl("/playground", `lng=${props.locale}`)}>
                  <KeyRound size={16} />
                  {libraryCopy.detailCta}
                </a>
              </div>
            </div>
            {heroItem ? <HeroArtwork copy={copy} item={heroItem} locale={props.locale} /> : null}
          </div>
        </header>

        <main>
          <section className="promptIndexBand">
            <div className="promptIndexIn">
              <SectionHead eyebrow={copy.collectionEyebrow} icon={BookOpen} title={copy.collectionTitle} />
              <div className="collectionGrid">
                {copy.collections.map((collection, index) => (
                  <CollectionCard
                    collection={collection}
                    href={collectionHref(index, props.locale)}
                    items={collectionItems[index] ?? []}
                    key={collection.title}
                    promptUnit={copy.promptUnit}
                  />
                ))}
              </div>
            </div>
          </section>

          <section className="promptIndexBand soft">
            <div className="promptIndexIn">
              <SectionHead eyebrow={copy.weeklyTitle} hint={copy.weeklyHint} icon={Sparkles} title={copy.weeklyTitle} />
              <div className="weeklyGrid">
                {weeklyItems.map((item) => (
                  <WeeklyCard copy={copy} item={item} key={item.slug} locale={props.locale} />
                ))}
              </div>
            </div>
          </section>

          <section className="promptIndexBand">
            <div className="promptIndexIn">
              <SectionHead icon={Boxes} title={copy.mediaTitle} />
              <div className="mediaBrowseGrid">
                <MediaBrowseCard copy={copy} count={mediaCounts.image} kind="image" locale={props.locale} />
                <MediaBrowseCard copy={copy} count={mediaCounts.video} kind="video" locale={props.locale} />
              </div>
            </div>
          </section>

          <section className="promptIndexBand mint">
            <div className="promptIndexIn">
              <SectionHead hint={copy.modelHint} icon={Layers3} title={copy.modelTitle} />
              <div className="modelGrid">
                {modelNames.map((modelName, index) => (
                  <ModelBrowseCard copy={copy} index={index} key={modelName} locale={props.locale} modelName={modelName} />
                ))}
              </div>
            </div>
          </section>

          <section className="promptIndexBand soft">
            <div className="promptIndexIn">
              <SectionHead icon={FileText} title={copy.categoryTitle} />
              <div className="keywordGrid">
                <KeywordPanel copy={copy} kind="image" title={copy.imageIndexTitle} />
                <KeywordPanel copy={copy} kind="video" title={copy.videoIndexTitle} />
              </div>
            </div>
          </section>

          <section className="promptIndexBand">
            <div className="promptIndexIn aboutGrid">
              <h2>{copy.aboutTitle}</h2>
              <p>{copy.aboutBody}</p>
            </div>
          </section>

          <section className="promptIndexBand soft">
            <div className="promptIndexIn">
              <SectionHead icon={Code2} title={copy.moreTitle} />
              <div className="featureBox">
                <div className="featureRows">
                  <FeatureRow body={copy.searchBody} icon={Search} title={copy.searchTitle}>
                    <div className="searchBarMock">
                      <Search size={16} />
                      {copy.mediaCards.image.keywords.join(" / ")}
                    </div>
                  </FeatureRow>
                  <FeatureRow body={copy.secondaryFeatureBody} icon={ImageIcon} title={copy.secondaryFeatureTitle}>
                    <div className="featureSwatch" />
                  </FeatureRow>
                  <div className="smallFeatureGrid">
                    {copy.smallFeatureLinks.map((label, index) => (
                      <Link href={localizePath(smallFeaturePath(label, index), props.locale)} key={label}>
                        {label}
                        <ArrowRight size={14} />
                      </Link>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className="promptIndexBand cta">
            <div className="promptIndexIn finalCta">
              <div>
                <h2>{copy.finalCtaTitle}</h2>
                <p>{copy.finalCtaBody}</p>
              </div>
              <a className="btn black big" href={consoleUrl("/playground", `lng=${props.locale}`)}>
                {copy.finalCtaButton}
                <ArrowRight size={16} />
              </a>
            </div>
          </section>
        </main>
      </div>
    </>
  );
}

function SectionHead(props: { eyebrow?: string; hint?: string; icon: LucideIcon; title: string }) {
  const Icon = props.icon;
  return (
    <div className="promptIndexHead">
      <div>
        {props.eyebrow ? (
          <div className="promptIndexKicker">
            <Icon size={14} />
            {props.eyebrow}
          </div>
        ) : null}
        <h2>{props.title}</h2>
      </div>
      {props.hint ? <p>{props.hint}</p> : null}
    </div>
  );
}

function HeroArtwork(props: { copy: PlaygroundIndexCopy; item: VisualPromptItem; locale: Locale }) {
  const title = localized(props.item.title, props.locale);
  return (
    <Link className="promptIndexHeroArt" href={itemHref(props.item, props.locale)}>
      <div className="promptIndexHeroFrame">
        <ArtifactMedia artifact={props.item.artifact} priority title={title} />
      </div>
      <div className="promptIndexHeroMeta">
        <b>{title}</b>
        <span>{props.copy.openPrompt}</span>
      </div>
    </Link>
  );
}

function CollectionCard(props: {
  collection: CollectionCopy;
  href: string;
  items: VisualPromptItem[];
  promptUnit: string;
}) {
  return (
    <Link className="collectionCard" href={props.href}>
      <div className="collectionTop">
        <span>{props.collection.meta}</span>
        <ArrowRight size={16} />
      </div>
      <div className="collectionPreview">
        {(props.items.length > 0 ? props.items : fallbackVisualItems()).slice(0, 3).map((item, index) => (
          <div className={`collectionThumb${index === 0 ? " tall" : ""}`} key={`${item.slug}-${index}`}>
            <ArtifactMedia artifact={item.artifact} title={item.artifact.alt} />
          </div>
        ))}
      </div>
      <div className="collectionBody">
        <h3>{props.collection.title}</h3>
        <p>{props.collection.body}</p>
        <div className="collectionFoot">
          <span>{`${Math.max(props.items.length, 3)} ${props.promptUnit}`}</span>
          <ArrowRight size={15} />
        </div>
      </div>
    </Link>
  );
}

function WeeklyCard(props: { copy: PlaygroundIndexCopy; item: VisualPromptItem; locale: Locale }) {
  const title = localized(props.item.title, props.locale);
  const mediaCopy = props.item.category === "video" ? props.copy.mediaCards.video : props.copy.mediaCards.image;
  return (
    <Link className="weeklyCard" href={itemHref(props.item, props.locale)}>
      <div className="weeklyMedia">
        <ArtifactMedia artifact={props.item.artifact} title={title} />
      </div>
      <div className="weeklyBody">
        <h3>{title}</h3>
        <div className="weeklyMeta">
          <span>{mediaCopy.label}</span>
          <span>{props.item.model}</span>
        </div>
      </div>
    </Link>
  );
}

function MediaBrowseCard(props: { copy: PlaygroundIndexCopy; count: number; kind: MediaKind; locale: Locale }) {
  const mediaCopy = props.copy.mediaCards[props.kind];
  const Icon = props.kind === "image" ? ImageIcon : Video;
  return (
    <Link className={`mediaBrowseCard ${props.kind}`} href={localizePath(props.kind === "image" ? CLI_IMAGE_PATH : CLI_VIDEO_PATH, props.locale)}>
      <span className="mediaIcon">
        <Icon size={24} />
      </span>
      <div className="mediaCopy">
        <h3>{mediaCopy.label}</h3>
        <p>{mediaCopy.body}</p>
        <span className="mediaTags">
          <span>{`${props.count} ${props.copy.promptUnit}`}</span>
          <span>{`${mediaCopy.keywords.length} ${props.copy.tagUnit}`}</span>
        </span>
      </div>
      <span className="mediaArrow">
        <ArrowRight size={17} />
      </span>
    </Link>
  );
}

function ModelBrowseCard(props: {
  copy: PlaygroundIndexCopy;
  index: number;
  locale: Locale;
  modelName: string;
}) {
  const modelCopy = props.copy.modelCards[props.index] ?? props.copy.modelCards[0];
  const params = new URLSearchParams({
    lng: props.locale,
    model: props.modelName.toLowerCase().replaceAll(" ", "-"),
    source: "flatkey-prompt-index",
  });
  return (
    <a className="modelBrowseCard" href={consoleUrl("/playground", params.toString())}>
      <div className="modelIconRow">
        <span className="modelIconBox" style={{ borderColor: modelAccentByIndex[props.index] }}>
          <Image alt="" height={22} src={modelIconByName[props.modelName]} width={22} />
        </span>
        {props.index === 0 || props.index === 5 ? <span className="modelBadge">NEW</span> : null}
      </div>
      <h3>{props.modelName}</h3>
      <p>{modelCopy.body}</p>
      <div className="modelFoot">
        <span>{modelCopy.media}</span>
        <ArrowRight size={14} />
      </div>
    </a>
  );
}

function KeywordPanel(props: { copy: PlaygroundIndexCopy; kind: MediaKind; title: string }) {
  return (
    <div className={`keywordPanel ${props.kind}`}>
      <div className="keywordPanelHead">
        <span className="keywordDot" />
        <h3>{props.title}</h3>
      </div>
      <div className="keywordColumns">
        {props.copy.keywordGroups[props.kind].map((group, index) => (
          <div className="keywordCol" key={group.label}>
            <b>{String(index + 1).padStart(2, "0")}</b>
            <h4>{group.label}</h4>
            <ul>
              {group.words.map((word) => (
                <li key={word}>{word}</li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </div>
  );
}

function FeatureRow(props: {
  body: string;
  children: ReactNode;
  icon: LucideIcon;
  title: string;
}) {
  const Icon = props.icon;
  return (
    <div className="featureRow">
      <div>
        <div className="promptIndexKicker">
          <Icon size={14} />
          {props.title}
        </div>
        <h3>{props.title}</h3>
        <p>{props.body}</p>
      </div>
      <div className="featureMock">{props.children}</div>
    </div>
  );
}

function ArtifactMedia(props: { artifact: VisualArtifact; priority?: boolean; title: string }) {
  if (props.artifact.kind === "video") {
    return (
      <>
        <video aria-label={props.artifact.alt} autoPlay loop muted playsInline poster={props.artifact.poster} preload="metadata">
          <source src={props.artifact.url} type="video/mp4" />
        </video>
        <span className="videoChip" aria-hidden>
          <Play size={14} fill="currentColor" />
        </span>
      </>
    );
  }

  return (
    <Image
      alt={props.artifact.alt || props.title}
      fill
      priority={props.priority}
      sizes="(min-width: 1180px) 28vw, (min-width: 760px) 46vw, 100vw"
      src={props.artifact.url}
    />
  );
}

function collectionHref(index: number, locale: Locale) {
  if (index === 1) return localizePath(CLI_VIDEO_PATH, locale);
  return localizePath(CLI_IMAGE_PATH, locale);
}

function itemHref(item: VisualPromptItem, locale: Locale) {
  if (item.category === "video") return localizePath(`${CLI_VIDEO_PATH}/${item.slug}`, locale);
  return localizePath(`${CLI_IMAGE_PATH}/${item.slug}`, locale);
}

function smallFeaturePath(label: string, index: number) {
  const normalized = label.toLowerCase();
  if (index === 1 || index === 3 || normalized.includes("video") || normalized.includes("seedance") || normalized.includes("视频")) {
    return CLI_VIDEO_PATH;
  }
  return CLI_IMAGE_PATH;
}

function localized(value: Record<Locale, string>, locale: Locale) {
  return value[locale] ?? value.en;
}

function mergePromptItems(items: PromptItem[]) {
  const bySlug = new Map<string, PromptItem>();
  for (const item of items) bySlug.set(item.slug, item);
  return Array.from(bySlug.values());
}

function hasLocalVisualMedia(item: PromptItem): item is VisualPromptItem {
  if (item.artifact.kind === "image") return item.artifact.url.startsWith("/");
  if (item.artifact.kind === "video") {
    return item.artifact.url.startsWith("/") && item.artifact.poster.startsWith("/");
  }
  return false;
}

function sortPromptItems(items: VisualPromptItem[]) {
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
  else if (item.tags.includes("diffusiondb")) rank = 65;
  else if (item.source.platform === "Hugging Face") rank = 90;

  if (item.artifact.kind === "image") rank -= 6;
  if (item.artifact.kind === "video") rank -= 4;

  return rank;
}

function preferSlugs(items: VisualPromptItem[], slugs: string[]) {
  const bySlug = new Map(items.map((item) => [item.slug, item]));
  const preferred = slugs.flatMap((slug) => {
    const item = bySlug.get(slug);
    return item ? [item] : [];
  });
  const preferredSet = new Set(preferred.map((item) => item.slug));
  return [...preferred, ...items.filter((item) => !preferredSet.has(item.slug))];
}

function fallbackVisualItems(): VisualPromptItem[] {
  return [
    {
      artifact: {
        alt: "Flatkey campaign prompt preview",
        kind: "image",
        url: "/assets/cli/campaign-hero.png",
      },
      category: "image",
      model: "gpt-image-2",
      output: {
        label: withIdFallback({
          en: "Preview image",
          zh: "预览图",
          es: "Imagen de vista previa",
          fr: "Image d'aperçu",
          pt: "Imagem de prévia",
          ru: "Preview image",
          ja: "プレビュー画像",
          vi: "Ảnh xem trước",
          de: "Vorschaubild",
        }),
        ratio: "16:9",
      },
      prompt: "",
      slug: "fallback-campaign-preview",
      source: {
        capturedAt: "2026-08-13",
        label: "Flatkey",
        platform: "Flatkey generated",
        url: "/playground",
      },
      summary: withIdFallback({
        en: "Fallback campaign prompt preview.",
        zh: "兜底活动提示词预览图。",
        es: "Vista previa de campaña.",
        fr: "Aperçu de campagne.",
        pt: "Prévia de campanha.",
        ru: "Campaign preview.",
        ja: "キャンペーンプレビュー。",
        vi: "Ảnh campaign dự phòng.",
        de: "Kampagnenvorschau.",
      }),
      tags: ["image"],
      title: withIdFallback({
        en: "Campaign prompt preview",
        zh: "活动提示词预览",
        es: "Vista previa de prompt",
        fr: "Aperçu de prompt",
        pt: "Prévia de prompt",
        ru: "Prompt preview",
        ja: "プロンプトプレビュー",
        vi: "Preview prompt",
        de: "Prompt-Vorschau",
      }),
      updatedAt: "2026-08-13",
    },
  ];
}
