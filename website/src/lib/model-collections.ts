import { classifyPublicModel, modelPublicPath } from "@/lib/model-public";
import {
  formatResolvedModelDisplayPrice,
  getVendorName,
  resolveModelDisplayPrice,
  type PricingData,
  type PricingModel,
} from "@/lib/pricing";
import { LOCALES, type Locale } from "@/lib/locales";

export type ModelCollectionCopy = {
  title: string;
  shortDescription: string;
  intro: string;
  criteria: string;
  empty: string;
};

export type ModelCollectionDefinition = {
  slug: string;
  icon: string;
  copy: Record<Locale, ModelCollectionCopy>;
  matches: (model: PricingModel) => boolean;
};

const copy = (values: Partial<Record<Locale, ModelCollectionCopy>>): Record<Locale, ModelCollectionCopy> => {
  const fallback: ModelCollectionCopy = {
      title: "AI Model Collection",
      shortDescription: "Compare models for your next AI workflow.",
      intro: "Browse current models available through Flatkey and compare the capabilities, usage, and public pricing that matter for this workflow.",
      criteria: "Models are grouped using catalog capabilities and live availability. Pricing and usage can change as providers update their offers.",
      empty: "No models are available in this collection yet.",
  };
  const english = values.en ?? fallback;
  return Object.fromEntries(LOCALES.map((locale) => [locale, values[locale] ?? english])) as Record<Locale, ModelCollectionCopy>;
};

const textOf = (model: PricingModel) =>
  [model.model_name, model.description, model.tags, ...(model.directory_metadata?.categories ?? [])]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();

const hasModality = (model: PricingModel, modality: string) =>
  model.directory_metadata?.modalities?.includes(modality as never) ||
  (model.supported_endpoint_types ?? []).some((endpoint) => endpoint.toLowerCase().includes(modality));

export const MODEL_COLLECTIONS: ModelCollectionDefinition[] = [
  {
    slug: "image-generation",
    icon: "✦",
    copy: copy({
      en: {
        title: "Best AI Models for Image Generation",
        shortDescription: "Compare image generation and editing models available through one API.",
        intro: "Find image models for text-to-image generation, editing, image understanding, and production workflows. Compare current capabilities, context, and public pricing before you build.",
        criteria: "This collection prioritizes models that advertise image output or image-generation endpoints in the live catalog.",
        empty: "Image generation models are being added to the catalog.",
      },
      zh: {
        title: "最佳 AI 图像生成模型",
        shortDescription: "比较可通过统一 API 调用的图像生成与编辑模型。",
        intro: "查找适合文生图、图片编辑和视觉工作流的模型，在接入前比较能力、上下文和公开价格。",
        criteria: "优先展示实时模型目录中标记为图像输出或图像生成接口的模型。",
        empty: "图像生成模型正在加入目录。",
      },
      es: { title: "Mejores modelos de IA para generar imágenes", shortDescription: "Compara modelos de generación y edición de imágenes con una sola API.", intro: "Encuentra modelos para texto a imagen, edición y flujos visuales, y compara sus capacidades y precios públicos.", criteria: "Priorizamos modelos con salida de imagen o endpoints de generación en el catálogo.", empty: "Aún no hay modelos de imagen en el catálogo." },
      fr: { title: "Meilleurs modèles IA pour générer des images", shortDescription: "Comparez les modèles d’image disponibles via une seule API.", intro: "Trouvez des modèles pour la génération et l’édition d’images, puis comparez leurs capacités et tarifs publics.", criteria: "La sélection repose sur les modalités et endpoints d’image du catalogue.", empty: "Les modèles d’image seront bientôt disponibles." },
      pt: { title: "Melhores modelos de IA para geração de imagens", shortDescription: "Compare modelos de imagem e edição em uma única API.", intro: "Encontre modelos para texto-imagem, edição e fluxos visuais e compare capacidades e preços públicos.", criteria: "Priorizamos modelos com saída de imagem ou endpoints de geração no catálogo.", empty: "Ainda não há modelos de imagem no catálogo." },
      ru: { title: "Лучшие ИИ-модели для генерации изображений", shortDescription: "Сравните модели генерации и редактирования изображений через единый API.", intro: "Выберите модели для генерации и редактирования изображений и сравните их возможности и публичные цены.", criteria: "В подборку попадают модели с изображениями в возможностях или endpoint каталога.", empty: "Модели изображений пока добавляются в каталог." },
      ja: { title: "画像生成に最適な AI モデル", shortDescription: "1 つの API で使える画像生成・編集モデルを比較します。", intro: "テキストからの画像生成や編集に使えるモデルを、機能と公開価格で比較できます。", criteria: "カタログで画像出力または画像生成エンドポイントが確認できるモデルを掲載します。", empty: "画像モデルは準備中です。" },
      vi: { title: "Mô hình AI tốt nhất để tạo ảnh", shortDescription: "So sánh các mô hình tạo và chỉnh sửa ảnh qua một API.", intro: "Tìm mô hình cho text-to-image, chỉnh sửa và quy trình hình ảnh, rồi so sánh khả năng và giá công khai.", criteria: "Bộ sưu tập ưu tiên mô hình có đầu ra ảnh hoặc endpoint tạo ảnh trong catalog.", empty: "Catalog chưa có mô hình hình ảnh." },
      de: { title: "Beste KI-Modelle für Bildgenerierung", shortDescription: "Vergleichen Sie Bild- und Bearbeitungsmodelle über eine API.", intro: "Finden Sie Modelle für Text-zu-Bild, Bearbeitung und visuelle Workflows und vergleichen Sie Funktionen und öffentliche Preise.", criteria: "Aufgenommen werden Modelle mit Bildausgabe oder Bild-Endpoint im Katalog.", empty: "Noch sind keine Bildmodelle verfügbar." },
    }),
    matches: (model) => classifyPublicModel(model) === "image" || /imagen|banana|flux|seedream|dall-e/i.test(model.model_name),
  },
  {
    slug: "coding",
    icon: "⌘",
    copy: copy({
      en: { title: "Best AI Models for Coding", shortDescription: "Compare models for code generation, debugging, and coding agents.", intro: "Explore models that developers use for code generation, debugging, refactoring, and agentic coding workflows. Use the live catalog to compare context and public pricing.", criteria: "Models are selected from catalog categories, descriptions, and names that identify coding or software-engineering workflows.", empty: "Coding models are being added to the catalog." },
      zh: { title: "最佳 AI 编程模型", shortDescription: "比较适合代码生成、调试和编程 Agent 的模型。", intro: "查找适合代码生成、调试、重构和编程 Agent 工作流的模型，比较上下文和公开价格。", criteria: "根据目录中的编程分类、描述和模型名称筛选。", empty: "编程模型正在加入目录。" },
      es: { title: "Mejores modelos de IA para programación", shortDescription: "Compara modelos para generar y depurar código.", intro: "Explora modelos para generación, depuración, refactorización y agentes de programación.", criteria: "Usamos categorías, descripciones y nombres del catálogo relacionados con software.", empty: "Los modelos de programación se añadirán pronto." },
      fr: { title: "Meilleurs modèles IA pour le code", shortDescription: "Comparez les modèles pour générer et déboguer du code.", intro: "Explorez des modèles pour la génération, le débogage, le refactoring et les agents de code.", criteria: "La sélection utilise les catégories et descriptions de développement du catalogue.", empty: "Les modèles de code seront bientôt disponibles." },
      pt: { title: "Melhores modelos de IA para programação", shortDescription: "Compare modelos para geração e depuração de código.", intro: "Explore modelos para gerar, depurar, refatorar código e criar agentes de programação.", criteria: "Usamos categorias, descrições e nomes de modelos ligados a software.", empty: "Os modelos de programação serão adicionados em breve." },
      ru: { title: "Лучшие ИИ-модели для программирования", shortDescription: "Сравните модели для генерации и отладки кода.", intro: "Изучите модели для генерации, отладки, рефакторинга и программных агентов.", criteria: "Используются категории и описания каталога, связанные с разработкой.", empty: "Модели для программирования пока добавляются." },
      ja: { title: "コーディングに最適な AI モデル", shortDescription: "コード生成・デバッグ・コーディングエージェント向けモデルを比較します。", intro: "コード生成、デバッグ、リファクタリング、エージェント開発向けのモデルを比較できます。", criteria: "カタログの開発関連カテゴリ、説明、モデル名を基準に選定します。", empty: "コーディングモデルは準備中です。" },
      vi: { title: "Mô hình AI tốt nhất cho lập trình", shortDescription: "So sánh mô hình tạo và gỡ lỗi mã.", intro: "Khám phá mô hình cho tạo mã, gỡ lỗi, refactor và tác vụ coding agent.", criteria: "Lựa chọn dựa trên danh mục và mô tả liên quan đến phát triển phần mềm.", empty: "Mô hình lập trình sẽ sớm được bổ sung." },
      de: { title: "Beste KI-Modelle fürs Programmieren", shortDescription: "Vergleichen Sie Modelle für Code und Debugging.", intro: "Entdecken Sie Modelle für Codegenerierung, Debugging, Refactoring und Coding-Agenten.", criteria: "Die Auswahl basiert auf Entwicklungs-Kategorien, Beschreibungen und Namen im Katalog.", empty: "Programmiermodelle werden bald ergänzt." },
    }),
    matches: (model) => /coding|code|program|developer|software|coder|codex|codestral/i.test(textOf(model)),
  },
  {
    slug: "video-generation",
    icon: "▶",
    copy: copy({
      en: { title: "Best AI Models for Video Generation", shortDescription: "Compare video models for text, image, and creative workflows.", intro: "Browse video generation models for text-to-video, image-to-video, and creative production workflows through a single API.", criteria: "Models are selected when the catalog reports video modalities, video endpoints, or a video-generation description.", empty: "Video models are being added to the catalog." },
      zh: { title: "最佳 AI 视频生成模型", shortDescription: "比较适合文生视频、图生视频和创作工作流的模型。", intro: "浏览适合文生视频、图生视频和创意生产的模型，通过统一 API 接入。", criteria: "优先展示目录中标记为视频模态、视频接口或视频生成能力的模型。", empty: "视频生成模型正在加入目录。" },
      es: { title: "Mejores modelos de IA para generar vídeos", shortDescription: "Compara modelos de vídeo para texto e imagen.", intro: "Explora modelos para texto a vídeo, imagen a vídeo y producción creativa mediante una sola API.", criteria: "Incluimos modelos con modalidad, endpoint o descripción de vídeo.", empty: "Los modelos de vídeo se añadirán pronto." },
      fr: { title: "Meilleurs modèles IA pour générer des vidéos", shortDescription: "Comparez les modèles vidéo pour le texte et l’image.", intro: "Parcourez les modèles texte-vidéo et image-vidéo accessibles via une seule API.", criteria: "La sélection repose sur les modalités, endpoints et descriptions vidéo.", empty: "Les modèles vidéo seront bientôt disponibles." },
      pt: { title: "Melhores modelos de IA para geração de vídeo", shortDescription: "Compare modelos de vídeo para texto e imagem.", intro: "Explore modelos de texto para vídeo, imagem para vídeo e produção criativa por uma única API.", criteria: "Incluímos modelos com modalidade, endpoint ou descrição de vídeo.", empty: "Os modelos de vídeo serão adicionados em breve." },
      ru: { title: "Лучшие ИИ-модели для генерации видео", shortDescription: "Сравните модели для текста, изображений и видео.", intro: "Изучите модели text-to-video и image-to-video для творческих задач через единый API.", criteria: "В подборку входят модели с видео-модальностью, endpoint или описанием.", empty: "Видео-модели пока добавляются." },
      ja: { title: "動画生成に最適な AI モデル", shortDescription: "テキスト・画像から動画を作るモデルを比較します。", intro: "text-to-video、image-to-video、クリエイティブ制作向けのモデルを比較できます。", criteria: "カタログで動画モダリティや動画エンドポイントが確認できるモデルを掲載します。", empty: "動画モデルは準備中です。" },
      vi: { title: "Mô hình AI tốt nhất để tạo video", shortDescription: "So sánh mô hình video từ văn bản và hình ảnh.", intro: "Duyệt mô hình text-to-video, image-to-video và quy trình sáng tạo qua một API.", criteria: "Chọn mô hình có modality, endpoint hoặc mô tả về video trong catalog.", empty: "Mô hình video sẽ sớm được bổ sung." },
      de: { title: "Beste KI-Modelle für Videogenerierung", shortDescription: "Vergleichen Sie Video-Modelle für Text und Bild.", intro: "Entdecken Sie Text-zu-Video- und Bild-zu-Video-Modelle für kreative Workflows über eine API.", criteria: "Aufgenommen werden Modelle mit Video-Modality, Endpoint oder Beschreibung.", empty: "Videomodelle werden bald ergänzt." },
    }),
    matches: (model) =>
      model.directory_metadata?.modalities?.includes("video") === true ||
      (model.supported_endpoint_types ?? []).some((endpoint) => /video|video-generation/i.test(endpoint)) ||
      /video|veo|kling|seedance|sora|runway|hailuo/i.test(model.model_name),
  },
  {
    slug: "tool-calling",
    icon: "⚙",
    copy: copy({
      en: { title: "AI Models with Tool Calling", shortDescription: "Find models for agents, functions, and automated workflows.", intro: "Compare models that can call tools and functions in agentic workflows, including APIs, databases, and external services.", criteria: "The collection uses catalog tags, supported parameters, and descriptions that mention tool or function calling.", empty: "Tool-calling models are being added to the catalog." },
      zh: { title: "支持工具调用的 AI 模型", shortDescription: "查找适合 Agent、函数调用和自动化工作流的模型。", intro: "比较能够调用工具和函数的模型，适合连接 API、数据库和外部服务。", criteria: "根据目录标签、支持参数和工具调用描述筛选。", empty: "工具调用模型正在加入目录。" },
      es: { title: "Modelos de IA con uso de herramientas", shortDescription: "Encuentra modelos para agentes y funciones.", intro: "Compara modelos capaces de llamar herramientas y funciones en flujos automatizados.", criteria: "Usamos etiquetas y descripciones del catálogo sobre herramientas o funciones.", empty: "Los modelos con herramientas se añadirán pronto." },
      fr: { title: "Modèles IA avec appel d’outils", shortDescription: "Trouvez des modèles pour les agents et fonctions.", intro: "Comparez les modèles capables d’appeler des outils et des fonctions dans des workflows automatisés.", criteria: "La sélection s’appuie sur les paramètres et descriptions du catalogue.", empty: "Les modèles avec outils seront bientôt disponibles." },
      pt: { title: "Modelos de IA com chamadas de ferramentas", shortDescription: "Encontre modelos para agentes e funções.", intro: "Compare modelos que chamam ferramentas e funções em fluxos automatizados.", criteria: "Usamos tags, parâmetros e descrições do catálogo.", empty: "Os modelos com ferramentas serão adicionados em breve." },
      ru: { title: "ИИ-модели с вызовом инструментов", shortDescription: "Найдите модели для агентов и функций.", intro: "Сравните модели, которые вызывают инструменты и функции в автоматизированных сценариях.", criteria: "Используются теги, параметры и описания каталога.", empty: "Модели с инструментами пока добавляются." },
      ja: { title: "ツール呼び出しに対応した AI モデル", shortDescription: "エージェントや関数呼び出し向けのモデルを探せます。", intro: "API、データベース、外部サービスと連携するエージェント向けモデルを比較します。", criteria: "カタログのタグ、対応パラメータ、説明を基準にします。", empty: "ツール対応モデルは準備中です。" },
      vi: { title: "Mô hình AI hỗ trợ gọi công cụ", shortDescription: "Tìm mô hình cho agent và function calling.", intro: "So sánh mô hình có thể gọi công cụ và hàm trong quy trình tự động.", criteria: "Dựa trên tag, tham số và mô tả về tool hoặc function calling.", empty: "Mô hình hỗ trợ công cụ sẽ sớm được bổ sung." },
      de: { title: "KI-Modelle mit Tool-Calling", shortDescription: "Finden Sie Modelle für Agenten und Funktionen.", intro: "Vergleichen Sie Modelle, die Tools und Funktionen in automatisierten Workflows aufrufen können.", criteria: "Die Auswahl basiert auf Tags, Parametern und Beschreibungen im Katalog.", empty: "Tool-fähige Modelle werden bald ergänzt." },
    }),
    matches: (model) => /tool.?calling|function.?calling|function call|agentic|agent workflow/i.test(textOf(model)),
  },
];

export function getModelCollection(slug: string): ModelCollectionDefinition | null {
  return MODEL_COLLECTIONS.find((collection) => collection.slug === slug) ?? null;
}

export function getModelCollectionPathnames(): string[] {
  return MODEL_COLLECTIONS.map((collection) => `/collections/${collection.slug}`);
}

export function getModelCollectionCopy(collection: ModelCollectionDefinition, locale: Locale): ModelCollectionCopy {
  return collection.copy[locale] ?? collection.copy.en;
}

export function selectCollectionModels(collection: ModelCollectionDefinition, models: PricingModel[], limit = 18): PricingModel[] {
  const matched = models.filter(collection.matches);
  return (matched.length ? matched : models).slice(0, limit);
}

export function modelCardData(model: PricingModel, pricing: PricingData) {
  const vendor = model.vendor_name ?? getVendorName(model, pricing.vendors);
  const price = resolveModelDisplayPrice(model, undefined, "plg", pricing.groupRatio);
  return {
    href: modelPublicPath(model.model_name),
    name: model.featured_config?.display_name || model.model_name,
    vendor,
    description: model.description || "",
    context: model.directory_metadata?.context_tokens,
    price: price ? formatResolvedModelDisplayPrice(price) : null,
  };
}
