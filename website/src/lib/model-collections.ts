import { classifyPublicModel, modelPublicPath } from "@/lib/model-public";
import { modelIconKey } from "@/lib/home-models";
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

export type ModelCollectionsSeoCopy = { title: string; description: string };

export const MIN_COLLECTION_MODELS = 5;

const MODEL_COLLECTIONS_SEO_COPY: Record<Locale, ModelCollectionsSeoCopy> = {
  en: {
    title: "AI Model Collections: Image, Video, Coding & More | Flatkey",
    description: "Explore curated AI model collections for image generation, video, coding, tool calling, embeddings, speech, and search. Compare live model capabilities and public API pricing on Flatkey.",
  },
  zh: {
    title: "AI 模型集合：图像、视频、编程模型 | Flatkey",
    description: "浏览图像生成、视频、编程、工具调用、嵌入、语音和搜索等 AI 模型集合，在 Flatkey 比较实时能力与公开 API 价格。",
  },
  es: {
    title: "Colecciones de modelos de IA: imágenes, vídeo, código y más | Flatkey",
    description: "Explora colecciones de IA para imágenes, vídeo, programación, herramientas, embeddings, voz y búsqueda. Compara capacidades y precios públicos en Flatkey.",
  },
  fr: {
    title: "Collections de modèles IA : image, vidéo, code et plus | Flatkey",
    description: "Découvrez des collections pour l’image, la vidéo, le code, les outils, les embeddings, la voix et la recherche. Comparez capacités et tarifs API publics sur Flatkey.",
  },
  pt: {
    title: "Coleções de modelos de IA: imagem, vídeo, código e mais | Flatkey",
    description: "Explore coleções para imagem, vídeo, programação, ferramentas, embeddings, voz e pesquisa. Compare capacidades e preços públicos de API na Flatkey.",
  },
  ru: {
    title: "Подборки ИИ-моделей: изображения, видео, код и другое | Flatkey",
    description: "Изучайте подборки моделей для изображений, видео, программирования, инструментов, embeddings, речи и поиска. Сравнивайте возможности и публичные цены API в Flatkey.",
  },
  ja: {
    title: "AI モデルコレクション：画像・動画・コーディングなど | Flatkey",
    description: "画像生成、動画、コーディング、ツール呼び出し、埋め込み、音声、検索向けの AI モデルを集めました。Flatkey で機能と公開 API 料金を比較できます。",
  },
  vi: {
    title: "Bộ sưu tập mô hình AI: hình ảnh, video, coding và hơn thế | Flatkey",
    description: "Khám phá bộ sưu tập cho tạo ảnh, video, lập trình, gọi công cụ, embedding, giọng nói và tìm kiếm. So sánh khả năng và giá API công khai trên Flatkey.",
  },
  de: {
    title: "KI-Modellsammlungen: Bild, Video, Code und mehr | Flatkey",
    description: "Entdecken Sie Sammlungen für Bildgenerierung, Video, Programmierung, Tool-Calling, Embeddings, Sprache und Suche. Vergleichen Sie Funktionen und öffentliche API-Preise bei Flatkey.",
  },
  id: {
    title: "Koleksi Model AI: Gambar, Video, Coding & Lainnya | Flatkey",
    description: "Jelajahi koleksi model AI untuk pembuatan gambar, video, coding, pemanggilan alat, embedding, suara, dan pencarian. Bandingkan kemampuan serta harga API publik di Flatkey.",
  },
};

export function getModelCollectionsSeoCopy(locale: Locale): ModelCollectionsSeoCopy {
  return MODEL_COLLECTIONS_SEO_COPY[locale] ?? MODEL_COLLECTIONS_SEO_COPY.en;
}

const DETAIL_SEO_SUFFIX: Record<Locale, string> = {
  en: " Compare capabilities, context, usage, and public API pricing on Flatkey.",
  zh: " 在 Flatkey 比较模型能力、上下文、调用量和公开 API 价格。",
  es: " Compara capacidades, contexto, uso y precios públicos de API en Flatkey.",
  fr: " Comparez les capacités, le contexte, l’usage et les tarifs API publics sur Flatkey.",
  pt: " Compare capacidades, contexto, uso e preços públicos de API na Flatkey.",
  ru: " Сравните возможности, контекст, использование и публичные цены API в Flatkey.",
  ja: " Flatkey で機能、コンテキスト、利用量、公開 API 料金を比較できます。",
  vi: " So sánh khả năng, ngữ cảnh, lượt dùng và giá API công khai trên Flatkey.",
  de: " Vergleichen Sie Funktionen, Kontext, Nutzung und öffentliche API-Preise bei Flatkey.",
  id: " Bandingkan kemampuan, konteks, penggunaan, dan harga API publik di Flatkey.",
};

export function getModelCollectionSeoDescription(collection: ModelCollectionDefinition, locale: Locale): string {
  const collectionCopy = getModelCollectionCopy(collection, locale);
  return `${collectionCopy.shortDescription}${DETAIL_SEO_SUFFIX[locale] ?? DETAIL_SEO_SUFFIX.en}`;
}

const LOCALIZED_COLLECTION_TITLES: Record<string, Partial<Record<Locale, string>>> = {
  "Best AI Models for Image Generation": { id: "Model AI terbaik untuk pembuatan gambar" },
  "Best AI Models for Coding": { id: "Model AI terbaik untuk coding" },
  "Best AI Models for Video Generation": { id: "Model AI terbaik untuk pembuatan video" },
  "AI Models with Tool Calling": { id: "Model AI dengan pemanggilan alat" },
  "Best Free AI Models on Flatkey": { es: "Mejores modelos de IA gratuitos en Flatkey", fr: "Meilleurs modèles IA gratuits sur Flatkey", pt: "Melhores modelos de IA gratuitos na Flatkey", ru: "Лучшие бесплатные ИИ-модели в Flatkey", ja: "Flatkey で使えるおすすめの無料 AI モデル", vi: "Mô hình AI miễn phí tốt nhất trên Flatkey", de: "Beste kostenlose KI-Modelle bei Flatkey", id: "Model AI gratis terbaik di Flatkey" },
  "Discounted AI Models on Flatkey": { es: "Modelos de IA con descuento en Flatkey", fr: "Modèles IA remisés sur Flatkey", pt: "Modelos de IA com desconto na Flatkey", ru: "ИИ-модели со скидкой в Flatkey", ja: "Flatkey の割引 AI モデル", vi: "Mô hình AI giảm giá trên Flatkey", de: "Vergünstigte KI-Modelle auf Flatkey", id: "Model AI diskon di Flatkey" },
  "Best AI Models for Roleplay and Creative Writing": { es: "Mejores modelos de IA para roleplay y escritura creativa", fr: "Meilleurs modèles IA pour le roleplay et l’écriture créative", pt: "Melhores modelos de IA para roleplay e escrita criativa", ru: "Лучшие ИИ-модели для ролевых игр и творчества", ja: "ロールプレイと創作に最適な AI モデル", vi: "Mô hình AI tốt nhất cho nhập vai và sáng tác", de: "Beste KI-Modelle für Rollenspiel und kreatives Schreiben", id: "Model AI terbaik untuk roleplay dan penulisan kreatif" },
  "AI Models with Vision": { es: "Modelos de IA con visión", fr: "Modèles IA avec vision", pt: "Modelos de IA com visão", ru: "ИИ-модели с компьютерным зрением", ja: "画像理解に対応した AI モデル", vi: "Mô hình AI hỗ trợ thị giác", de: "KI-Modelle mit Bildverständnis", id: "Model AI dengan kemampuan vision" },
  "Top AI Models Used by OpenClaw": { es: "Modelos de IA más usados por OpenClaw", fr: "Modèles IA populaires avec OpenClaw", pt: "Principais modelos de IA usados pelo OpenClaw", ru: "Популярные ИИ-модели для OpenClaw", ja: "OpenClaw で使われる AI モデル", vi: "Mô hình AI phổ biến cho OpenClaw", de: "Top-KI-Modelle für OpenClaw", id: "Model AI teratas yang digunakan OpenClaw" },
  "Text Embedding Models": { es: "Modelos de embeddings de texto", fr: "Modèles d’embeddings de texte", pt: "Modelos de embedding de texto", ru: "Модели текстовых embeddings", ja: "テキスト埋め込みモデル", vi: "Mô hình embedding văn bản", de: "Text-Embedding-Modelle", id: "Model embedding teks" },
  "Best Audio Generation Models": { es: "Mejores modelos de IA para generar audio", fr: "Meilleurs modèles IA pour générer de l’audio", pt: "Melhores modelos de IA para geração de áudio", ru: "Лучшие ИИ-модели для генерации аудио", ja: "音声生成に最適な AI モデル", vi: "Mô hình AI tốt nhất để tạo âm thanh", de: "Beste KI-Modelle für Audiogenerierung", id: "Model AI terbaik untuk menghasilkan audio" },
  "Best Text-to-Speech Models": { es: "Mejores modelos de texto a voz", fr: "Meilleurs modèles de synthèse vocale", pt: "Melhores modelos de texto para fala", ru: "Лучшие модели синтеза речи", ja: "音声合成に最適な AI モデル", vi: "Mô hình chuyển văn bản thành giọng nói", de: "Beste Text-to-Speech-Modelle", id: "Model text-to-speech terbaik" },
  "Best Speech-to-Text and Transcription Models": { es: "Mejores modelos de voz a texto y transcripción", fr: "Meilleurs modèles de reconnaissance et transcription vocales", pt: "Melhores modelos de fala para texto e transcrição", ru: "Лучшие модели распознавания и транскрибации речи", ja: "音声認識と文字起こしに最適な AI モデル", vi: "Mô hình chuyển giọng nói thành văn bản tốt nhất", de: "Beste Sprach-zu-Text- und Transkriptionsmodelle", id: "Model speech-to-text dan transkripsi terbaik" },
  "Best Rerank Models for Search and RAG": { es: "Mejores modelos de reranking para búsqueda y RAG", fr: "Meilleurs modèles de reranking pour la recherche et le RAG", pt: "Melhores modelos de rerank para pesquisa e RAG", ru: "Лучшие rerank-модели для поиска и RAG", ja: "検索と RAG に最適なリランキングモデル", vi: "Mô hình rerank tốt nhất cho tìm kiếm và RAG", de: "Beste Rerank-Modelle für Suche und RAG", id: "Model rerank terbaik untuk pencarian dan RAG" },
};

const LOCALIZED_FALLBACK_FIELDS: Record<Locale, Omit<ModelCollectionCopy, "title">> = {
  en: { shortDescription: "Compare models for this AI workflow through one API.", intro: "Browse current models for this workflow and compare capabilities, usage, and public pricing before you build.", criteria: "Models are grouped using live catalog capabilities and availability.", empty: "Models are being added to this collection." },
  zh: { shortDescription: "通过统一 API 比较适合该 AI 工作流的模型。", intro: "浏览适合该工作流的实时模型，在接入前比较能力、调用量和公开价格。", criteria: "根据实时模型目录中的能力与可用性进行归类。", empty: "相关模型正在加入目录。" },
  es: { shortDescription: "Compara modelos para este flujo de IA con una sola API.", intro: "Explora modelos actuales para este flujo y compara capacidades, uso y precios públicos.", criteria: "Agrupamos modelos según las capacidades y disponibilidad del catálogo.", empty: "Los modelos se añadirán a esta colección." },
  fr: { shortDescription: "Comparez les modèles de ce workflow IA via une seule API.", intro: "Parcourez les modèles disponibles et comparez capacités, usage et tarifs publics.", criteria: "Les modèles sont regroupés selon les capacités et la disponibilité du catalogue.", empty: "Les modèles seront bientôt ajoutés à cette collection." },
  pt: { shortDescription: "Compare modelos para este fluxo de IA em uma única API.", intro: "Explore modelos atuais para o fluxo e compare capacidades, uso e preços públicos.", criteria: "Os modelos são agrupados pelas capacidades e disponibilidade do catálogo.", empty: "Modelos serão adicionados a esta coleção." },
  ru: { shortDescription: "Сравните модели для этого ИИ-сценария через единый API.", intro: "Изучите доступные модели и сравните возможности, использование и публичные цены.", criteria: "Модели группируются по возможностям и доступности в каталоге.", empty: "Модели для этой подборки скоро появятся." },
  ja: { shortDescription: "1 つの API でこの AI ワークフロー向けモデルを比較できます。", intro: "現在利用できるモデルを確認し、機能・利用量・公開料金を比較できます。", criteria: "カタログの機能と提供状況を基準にモデルを分類します。", empty: "このコレクションのモデルは準備中です。" },
  vi: { shortDescription: "So sánh mô hình cho quy trình AI này qua một API.", intro: "Khám phá mô hình hiện có và so sánh khả năng, lượt dùng cùng giá công khai.", criteria: "Mô hình được nhóm theo khả năng và tình trạng sẵn có trong catalog.", empty: "Mô hình cho bộ sưu tập này sẽ sớm được bổ sung." },
  de: { shortDescription: "Vergleichen Sie Modelle für diesen KI-Workflow über eine API.", intro: "Entdecken Sie verfügbare Modelle und vergleichen Sie Funktionen, Nutzung und öffentliche Preise.", criteria: "Die Gruppierung basiert auf Funktionen und Verfügbarkeit im Katalog.", empty: "Modelle für diese Sammlung werden bald ergänzt." },
  id: { shortDescription: "Bandingkan model untuk alur kerja AI ini melalui satu API.", intro: "Jelajahi model yang tersedia dan bandingkan kemampuan, penggunaan, serta harga publik.", criteria: "Model dikelompokkan berdasarkan kemampuan dan ketersediaan di katalog.", empty: "Model untuk koleksi ini akan segera ditambahkan." },
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
  return Object.fromEntries(
    LOCALES.map((locale) => {
      if (values[locale]) return [locale, values[locale]];
      const localizedFields = LOCALIZED_FALLBACK_FIELDS[locale] ?? LOCALIZED_FALLBACK_FIELDS.en;
      const localizedTitle = LOCALIZED_COLLECTION_TITLES[english.title]?.[locale] ?? (locale === "en" ? english.title : `${localizedFields.shortDescription.split(" ")[0]} AI Model Collection`);
      return [locale, { title: localizedTitle, ...localizedFields }];
    }),
  ) as Record<Locale, ModelCollectionCopy>;
};

const textOf = (model: PricingModel) =>
  [model.model_name, model.description, model.tags, ...(model.directory_metadata?.categories ?? [])]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();

const hasModality = (model: PricingModel, modality: string) =>
  model.directory_metadata?.modalities?.includes(modality as never) ||
  (model.supported_endpoint_types ?? []).some((endpoint) => endpoint.toLowerCase().includes(modality));

const hasActiveDiscount = (model: PricingModel) =>
  Object.values(model.display_pricing?.prices ?? {}).some((price) => {
    const configured = price?.configured;
    const plg = price?.plg;
    return (
      typeof configured === "number" &&
      Number.isFinite(configured) &&
      configured >= 0 &&
      typeof plg === "number" &&
      Number.isFinite(plg) &&
      plg >= 0 &&
      plg < configured
    );
  });

const hasZeroPublicPrice = (model: PricingModel) => {
  const prices = Object.values(model.display_pricing?.prices ?? {})
    .map((price) => {
      if (typeof price?.plg === "number" && Number.isFinite(price.plg)) return price.plg;
      if (typeof price?.configured === "number" && Number.isFinite(price.configured)) return price.configured;
      return null;
    })
    .filter((price): price is number => price !== null);
  return prices.length > 0 && prices.every((price) => price === 0);
};

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
    matches: (model) => /tool.?calling|function.?calling|function call|custom.?tools|agentic|agent workflow/i.test(textOf(model)),
  },
  {
    slug: "free-models",
    icon: "◇",
    copy: copy({
      en: { title: "Best Free AI Models on Flatkey", shortDescription: "Explore AI models with zero token pricing through one API.", intro: "Compare free AI models available through Flatkey for experiments, prototypes, and everyday workloads.", criteria: "Models whose current input ratio or request price is zero in the live catalog.", empty: "Free models are being added to the catalog." },
      zh: { title: "Flatkey 上的最佳免费 AI 模型", shortDescription: "通过统一 API 体验当前价格为零的 AI 模型。", intro: "浏览适合实验、原型和日常任务的免费 AI 模型。", criteria: "实时目录中输入倍率或请求价格当前为零的模型。", empty: "免费模型正在加入目录。" },
    }),
    matches: hasZeroPublicPrice,
  },
  {
    slug: "discounted-models",
    icon: "%",
    copy: copy({
      en: { title: "Discounted AI Models on Flatkey", shortDescription: "Find models with promotional pricing and lower-cost access.", intro: "Discover models with an active discount or promotional rate in the public catalog.", criteria: "Models whose public rate is below the configured reference rate.", empty: "Discounted models are being added to the catalog." },
      zh: { title: "Flatkey 上的折扣 AI 模型", shortDescription: "查找有促销价格、调用成本更低的模型。", intro: "发现公开目录中正在提供折扣或促销价格的模型。", criteria: "公开价格低于配置参考价格的模型。", empty: "折扣模型正在加入目录。" },
    }),
    matches: hasActiveDiscount,
  },
  {
    slug: "roleplay-creative-writing",
    icon: "✎",
    copy: copy({
      en: { title: "Best AI Models for Roleplay and Creative Writing", shortDescription: "Compare models for character chat, roleplay, and imaginative writing.", intro: "Find models suited to character conversations, roleplay, storytelling, and creative writing.", criteria: "Models whose names, tags, categories, or descriptions indicate roleplay or creative writing.", empty: "Roleplay models are being added to the catalog." },
      zh: { title: "角色扮演与创意写作 AI 模型", shortDescription: "比较适合角色聊天、角色扮演和创意写作的模型。", intro: "查找适合角色对话、故事创作和创意写作的模型。", criteria: "名称、标签、分类或描述中包含角色扮演与创意写作信号的模型。", empty: "角色扮演模型正在加入目录。" },
    }),
    matches: (model) => /roleplay|creative writing|character chat|sillytavern|janitor|creative/i.test(textOf(model)),
  },
  {
    slug: "vision-models",
    icon: "◉",
    copy: copy({
      en: { title: "AI Models with Vision", shortDescription: "Compare multimodal models for image understanding and visual questions.", intro: "Explore multimodal language models that can read images, charts, screenshots, and other visual content.", criteria: "Models with image input metadata that are not primarily image-generation models.", empty: "Vision models are being added to the catalog." },
      zh: { title: "支持视觉理解的多模态 AI 模型", shortDescription: "比较可理解图片和视觉内容的多模态模型。", intro: "浏览能够读取图片、图表、截图并回答视觉问题的多模态模型。", criteria: "具有图像输入能力且主要用途不是图像生成的模型。", empty: "视觉模型正在加入目录。" },
    }),
    matches: (model) => hasModality(model, "image") && !/image-generation|text-to-image|dall.?e|imagen|flux|seedream|banana/i.test(textOf(model)),
  },
  {
    slug: "openclaw-models",
    icon: "⌁",
    copy: copy({
      en: { title: "Top AI Models Used by OpenClaw", shortDescription: "See popular models for autonomous agent workflows and tool use.", intro: "Browse models suited to OpenClaw-style autonomous agents, ranked with live usage signals.", criteria: "Models that support agentic workflows and are ranked by the live usage data shown on this page.", empty: "OpenClaw models are being added to the catalog." },
      zh: { title: "OpenClaw 常用的 AI 模型", shortDescription: "查看适合自主 Agent 工作流和工具调用的热门模型。", intro: "浏览适合 OpenClaw 类自主 Agent 的模型，并参考实时调用量。", criteria: "支持 Agent 工作流，并按本页实时调用数据排序的模型。", empty: "OpenClaw 模型正在加入目录。" },
    }),
    matches: (model) => /openclaw|agentic|agent|tool.?calling|function.?calling/i.test(textOf(model)),
  },
  {
    slug: "text-embedding-models",
    icon: "≋",
    copy: copy({
      en: { title: "Text Embedding Models", shortDescription: "Find embedding APIs for semantic search, RAG, and clustering.", intro: "Compare text embedding models for semantic search, retrieval pipelines, clustering, and similarity matching.", criteria: "Models identified as embeddings by catalog metadata, endpoint, name, or description.", empty: "Embedding models are being added to the catalog." },
      zh: { title: "文本嵌入模型", shortDescription: "查找适合语义搜索、RAG 和聚类的嵌入 API。", intro: "比较适合语义搜索、检索增强、聚类和相似度匹配的文本嵌入模型。", criteria: "目录元数据、接口、名称或描述中标记为嵌入模型的模型。", empty: "嵌入模型正在加入目录。" },
    }),
    matches: (model) => /embedding|embed|bge-|e5-|gte-|text-embedding| jina-embeddings/i.test(textOf(model)),
  },
  {
    slug: "audio-generation-models",
    icon: "♫",
    copy: copy({
      en: { title: "Best Audio Generation Models", shortDescription: "Compare models for music, sound, and audio-output applications.", intro: "Explore models for music generation, sound effects, and other audio-output workflows.", criteria: "Models with audio output or audio-generation signals in the catalog.", empty: "Audio generation models are being added to the catalog." },
      zh: { title: "最佳音频生成模型", shortDescription: "比较适合音乐、声音和音频输出的模型。", intro: "浏览适合音乐生成、音效和其他音频输出工作流的模型。", criteria: "目录中具有音频输出或音频生成能力信号的模型。", empty: "音频生成模型正在加入目录。" },
    }),
    matches: (model) =>
      (hasModality(model, "audio") && /audio|music|sound|tts|speech|voice/i.test(textOf(model))) ||
      /audio|music|sound|tts|speech|voice|sonilo/i.test(model.model_name),
  },
  {
    slug: "text-to-speech-models",
    icon: "◖",
    copy: copy({
      en: { title: "Best Text-to-Speech Models", shortDescription: "Compare TTS models for voice generation and narration.", intro: "Find text-to-speech models for voice generation, narration, accessibility, and audio apps.", criteria: "Models identified as text-to-speech by catalog metadata or descriptions.", empty: "Text-to-speech models are being added to the catalog." },
      zh: { title: "最佳文本转语音模型", shortDescription: "比较适合语音生成、旁白和无障碍应用的 TTS 模型。", intro: "查找用于语音生成、旁白、无障碍和音频应用的文本转语音模型。", criteria: "目录元数据或描述中标记为文本转语音的模型。", empty: "文本转语音模型正在加入目录。" },
    }),
    matches: (model) => /text.?to.?speech|tts|speech synthesis|voice generation/i.test(textOf(model)),
  },
  {
    slug: "speech-to-text-models",
    icon: "◗",
    copy: copy({
      en: { title: "Best Speech-to-Text and Transcription Models", shortDescription: "Find models for transcription, captions, meetings, and speech recognition.", intro: "Compare speech-to-text and transcription models for meetings, calls, captions, and voice interfaces.", criteria: "Models identified as speech recognition, transcription, or speech-to-text in the catalog.", empty: "Speech-to-text models are being added to the catalog." },
      zh: { title: "最佳语音转文字与转录模型", shortDescription: "查找适合转录、字幕、会议和语音识别的模型。", intro: "比较适合会议、通话、字幕和语音交互的语音转文字模型。", criteria: "目录中标记为语音识别、转录或语音转文字的模型。", empty: "语音转文字模型正在加入目录。" },
    }),
    matches: (model) => /speech.?to.?text|transcri|automatic speech recognition|whisper/i.test(textOf(model)),
  },
  {
    slug: "rerank-models",
    icon: "⇅",
    copy: copy({
      en: { title: "Best Rerank Models for Search and RAG", shortDescription: "Compare rerankers for semantic search and retrieval quality.", intro: "Explore reranking models for semantic search, RAG pipelines, and recommendation systems.", criteria: "Models identified as rerank or reranker models in the catalog.", empty: "Rerank models are being added to the catalog." },
      zh: { title: "搜索与 RAG 最佳重排模型", shortDescription: "比较用于语义搜索和检索质量优化的重排模型。", intro: "浏览适合语义搜索、RAG 流程和推荐系统的重排模型。", criteria: "目录中标记为 rerank 或 reranker 的模型。", empty: "重排模型正在加入目录。" },
    }),
    matches: (model) => /rerank|reranker|re-rank/i.test(textOf(model)),
  },
  {
    slug: "general-purpose-models",
    icon: "✧",
    copy: copy({
      en: { title: "General-Purpose AI Models", shortDescription: "Browse dependable models for everyday AI tasks through one API.", intro: "This broad collection keeps every live catalog model discoverable, including models that do not yet have a dedicated capability category.", criteria: "Every model in the live public catalog is included so no model detail page is isolated from the collection directory.", empty: "General-purpose models are being added to the catalog." },
      zh: { title: "通用 AI 模型", shortDescription: "通过统一 API 浏览适合日常 AI 任务的模型。", intro: "这个通用集合确保实时目录中的每个模型都能被发现，即使它暂时没有专门的能力分类。", criteria: "收录实时公开目录中的全部模型，确保每个模型详情页都至少属于一个集合。", empty: "通用模型正在加入目录。" },
      es: { title: "Modelos de IA de propósito general", shortDescription: "Explora modelos fiables para tareas habituales de IA mediante una API.", intro: "Esta colección mantiene visibles todos los modelos del catálogo, incluidos los que aún no tienen una categoría específica.", criteria: "Incluye todos los modelos del catálogo público para que ningún detalle quede aislado.", empty: "Los modelos generales se añadirán al catálogo." },
      fr: { title: "Modèles IA généralistes", shortDescription: "Découvrez des modèles fiables pour les tâches IA courantes via une API.", intro: "Cette collection rend chaque modèle du catalogue visible, même sans catégorie spécialisée.", criteria: "Tous les modèles publics sont inclus afin qu’aucune fiche ne reste isolée.", empty: "Les modèles généralistes seront bientôt disponibles." },
      pt: { title: "Modelos de IA de uso geral", shortDescription: "Explore modelos para tarefas comuns de IA em uma única API.", intro: "Esta coleção mantém visíveis todos os modelos do catálogo, inclusive os que ainda não têm categoria própria.", criteria: "Inclui todos os modelos públicos para que nenhuma página de detalhes fique isolada.", empty: "Os modelos gerais serão adicionados ao catálogo." },
      ru: { title: "Универсальные ИИ-модели", shortDescription: "Изучайте надёжные модели для повседневных задач через единый API.", intro: "Эта подборка делает видимой каждую модель каталога, даже если для неё ещё нет отдельной категории.", criteria: "Включены все модели публичного каталога, чтобы ни одна страница модели не оставалась изолированной.", empty: "Универсальные модели скоро появятся в каталоге." },
      ja: { title: "汎用 AI モデル", shortDescription: "日常的な AI タスクに使えるモデルを 1 つの API で探せます。", intro: "専用カテゴリがまだないモデルを含め、公開カタログのすべてのモデルを見つけられるコレクションです。", criteria: "モデル詳細ページが孤立しないよう、公開カタログの全モデルを掲載します。", empty: "汎用モデルは準備中です。" },
      vi: { title: "Mô hình AI đa dụng", shortDescription: "Khám phá mô hình đáng tin cậy cho tác vụ AI hằng ngày qua một API.", intro: "Bộ sưu tập này giúp tìm thấy mọi mô hình trong catalog, kể cả mô hình chưa có danh mục chuyên biệt.", criteria: "Bao gồm toàn bộ mô hình công khai để không trang chi tiết nào bị bỏ riêng lẻ.", empty: "Mô hình đa dụng sẽ sớm được bổ sung." },
      de: { title: "Allgemeine KI-Modelle", shortDescription: "Entdecken Sie zuverlässige Modelle für alltägliche KI-Aufgaben über eine API.", intro: "Diese Sammlung macht jedes Katalogmodell auffindbar, auch ohne eigene Spezialkategorie.", criteria: "Alle öffentlichen Katalogmodelle werden aufgenommen, damit keine Detailseite isoliert bleibt.", empty: "Allgemeine Modelle werden bald ergänzt." },
      id: { title: "Model AI serbaguna", shortDescription: "Jelajahi model tepercaya untuk tugas AI sehari-hari melalui satu API.", intro: "Koleksi ini membuat semua model di katalog dapat ditemukan, termasuk model yang belum memiliki kategori khusus.", criteria: "Semua model publik disertakan agar tidak ada halaman detail yang berdiri sendiri.", empty: "Model serbaguna akan segera ditambahkan." },
    }),
    matches: () => true,
  },
];

export function getModelCollection(slug: string): ModelCollectionDefinition | null {
  return MODEL_COLLECTIONS.find((collection) => collection.slug === slug) ?? null;
}

export function getAvailableModelCollections(models: PricingModel[]): ModelCollectionDefinition[] {
  return MODEL_COLLECTIONS.filter((collection) => selectCollectionModels(collection, models, MIN_COLLECTION_MODELS).length >= MIN_COLLECTION_MODELS);
}

export function getModelCollectionPathnames(models?: PricingModel[]): string[] {
  const collections = models ? getAvailableModelCollections(models) : MODEL_COLLECTIONS;
  return collections.map((collection) => `/collections/${collection.slug}`);
}

export function getModelCollectionCopy(collection: ModelCollectionDefinition, locale: Locale): ModelCollectionCopy {
  return collection.copy[locale] ?? collection.copy.en;
}

export function selectCollectionModels(collection: ModelCollectionDefinition, models: PricingModel[], limit?: number): PricingModel[] {
  const matched = models.filter(collection.matches);
  return limit == null ? matched : matched.slice(0, limit);
}

export function modelCardData(model: PricingModel, pricing: PricingData, fallbackDescription = "") {
  const vendor = model.vendor_name ?? getVendorName(model, pricing.vendors);
  const price = resolveModelDisplayPrice(model, undefined, "plg", pricing.groupRatio);
  return {
    href: modelPublicPath(model.model_name),
    name: model.featured_config?.display_name || model.model_name,
    vendor,
    // The public pricing payload often leaves icon fields empty. Resolve the
    // official vendor mark from the model family/vendor instead of passing the
    // full model id to the logo component (which would fall back to initials).
    iconKey: model.icon || model.vendor_icon || modelIconKey(model.model_name, vendor),
    description: model.featured_config?.description || model.description || model.vendor_description || fallbackDescription,
    context: model.directory_metadata?.context_tokens,
    price: price ? formatResolvedModelDisplayPrice(price) : null,
  };
}
