import Image from "next/image";
import Link from "next/link";
import type { CSSProperties } from "react";
import { buildRowsForModels } from "@/lib/home-models";
import { type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { modelPublicPath } from "@/lib/model-public";
import { consoleUrl } from "@/lib/origins";
import { sortPricingModelsBySeries, type PricingData, type PricingModel } from "@/lib/pricing";
import { OnlineHomeHeroCarousel, type HeroMode } from "./online-home-hero-carousel";
import { OnlineStaticShell } from "./online-static-shell";

type HomePriceComparison = {
  flatkey: string;
  href: string;
  image: string;
  model: string;
  official: string;
  policy: string;
  tag: string;
  vendor: string;
};

type HomeModelKind = HeroMode["kind"];
type HomeModelTags = Record<HomeModelKind, string[]>;

type HomePageCopy = {
  carousel: {
    aria: string;
    eyebrow: string;
    switchAria: string;
  };
  ctaConsole: string;
  contactSales: string;
  fallbackDirectory: string;
  hero: {
    all: Omit<HeroMode, "href" | "image" | "kind" | "metric" | "modelName" | "modelVendor" | "thumb"> & { fallbackCta: string; fallbackMetric: string; fallbackModelName: string; fallbackVendor: string };
    audio: Omit<HeroMode, "href" | "image" | "kind" | "metric" | "modelName" | "modelVendor" | "thumb"> & { fallbackCta: string; fallbackMetric: string; fallbackModelName: string; fallbackVendor: string };
    image: Omit<HeroMode, "href" | "image" | "kind" | "metric" | "modelName" | "modelVendor" | "thumb"> & { fallbackCta: string; fallbackMetric: string; fallbackModelName: string; fallbackVendor: string };
    text: Omit<HeroMode, "href" | "image" | "kind" | "metric" | "modelName" | "modelVendor" | "thumb"> & { fallbackCta: string; fallbackMetric: string; fallbackModelName: string; fallbackVendor: string };
    video: Omit<HeroMode, "href" | "image" | "kind" | "metric" | "modelName" | "modelVendor" | "thumb"> & { fallbackCta: string; fallbackMetric: string; fallbackModelName: string; fallbackVendor: string };
    useModel: (modelName: string) => string;
  };
  logo: {
    aria: string;
    connected: string;
  };
  modelFlow: {
    directoryFallback: string;
    kicker: string;
    proof: string[];
    sub: string;
    title: string;
    types: Record<HomeModelKind, { api: string; copy: string; cta: string; title: string }>;
  };
  price: {
    aria: string;
    cardPolicy: string;
    flatkeyFailure: string;
    flatkeyFailureShort: string;
    kicker: string;
    officialEndpoint: string;
    officialLabel: string;
    perMillionInput: string;
    perRequest: string;
    requestLedger: string;
    sharedBalance: string;
    test: string;
    title: string;
    sub: string;
    viewAll: string;
  };
  cli: {
    aria: string;
    checks: string[];
    docs: string;
    guide: string;
    kicker: string;
    range: string;
    stepsDone: string;
    steps: Array<{ body: string; code: string; no: string; title: string }>;
    sub: string;
    title: string;
  };
  why: {
    cards: Array<{ body: string; chips: string[]; metric: string; title: string }>;
    kicker: string;
    title: string;
  };
  voice: {
    aria: string;
    boardCopy: string;
    boardMetric: string;
    boardTitle: string;
    faqs: Array<[string, string]>;
    items: Array<{ metric: string; quote: string; role: string }>;
    kicker: string;
    signals: string[];
    sub: string;
    title: string;
  };
  final: {
    alt: string;
    kicker: string;
    launch: Array<[string, string, string]>;
    metrics: [string, string, string];
    sub: string;
    title: string;
    trustZeroRetention: string;
  };
};

const HERO_ART: Record<HomeModelKind, { card: string; wide: string }> = {
  all: {
    card: "/assets/generated/flatkey-art-card-all-01.png",
    wide: "/assets/generated/flatkey-story-all-models.png",
  },
  audio: {
    card: "/assets/generated/flatkey-art-card-audio-01.png",
    wide: "/assets/generated/flatkey-story-audio-models.png",
  },
  image: {
    card: "/assets/generated/flatkey-art-card-image-01.png",
    wide: "/assets/generated/flatkey-story-image-models.png",
  },
  text: {
    card: "/assets/generated/flatkey-art-card-text-01.png",
    wide: "/assets/generated/flatkey-story-text-models.png",
  },
  video: {
    card: "/assets/generated/flatkey-art-card-video-01.png",
    wide: "/assets/generated/flatkey-story-video-models.png",
  },
};

const PRICE_POSTERS = [
  "/assets/generated/flatkey-price-poster-01.png",
  "/assets/generated/flatkey-price-poster-02.png",
  "/assets/generated/flatkey-price-poster-03.png",
  "/assets/generated/flatkey-price-poster-04.png",
  "/assets/generated/flatkey-price-poster-05.png",
  "/assets/generated/flatkey-price-poster-06.png",
  "/assets/generated/flatkey-price-poster-07.png",
  "/assets/generated/flatkey-price-poster-08.png",
  "/assets/generated/flatkey-price-poster-09.png",
  "/assets/generated/flatkey-price-poster-10.png",
  "/assets/generated/flatkey-price-poster-11.png",
  "/assets/generated/flatkey-price-poster-12.png",
] as const;

const logoMarquee = [
  ["openai.svg", "OpenAI"],
  ["claude.svg", "Claude"],
  ["googlegemini.svg", "Gemini"],
  ["deepseek.svg", "DeepSeek"],
  ["qwen.svg", "Qwen"],
  ["zai.svg", "Z.ai"],
  ["moonshotai.svg", "Kimi"],
  ["bytedance.svg", "Seedance"],
  ["minimax.svg", "MiniMax"],
  ["mistralai.svg", "Mistral"],
  ["meta.svg", "Meta"],
  ["perplexity.svg", "Perplexity"],
] as const;

type HomeTranslatedLocale = Exclude<Locale, "en" | "zh">;

const HOME_TRANSLATION_BASE: Record<HomeTranslatedLocale, {
  all: string;
  allDir: string;
  audio: string;
  audioCopy: string;
  audioModels: string;
  audioSubline: string;
  audited: string;
  balance: string;
  billing: string;
  browseAll: string;
  carousel: string;
  cliTitle: string;
  cliSub: string;
  contact: string;
  cost: string;
  debug: string;
  directory: string;
  docs: string;
  endpoint: string;
  failure: string;
  finalKicker: string;
  finalTitle: string;
  finalSub: string;
  guide: string;
  image: string;
  imageCopy: string;
  imageModels: string;
  imageSubline: string;
  ledger: string;
  modelFlowKicker: string;
  modelFlowTitle: string;
  modelFlowSub: string;
  models: string;
  official: string;
  openAudio: string;
  openImage: string;
  openText: string;
  openVideo: string;
  priceKicker: string;
  priceTitle: string;
  priceSub: string;
  request: string;
  security: string;
  sharedBalance: string;
  signedSla: string;
  switchType: string;
  test: string;
  text: string;
  textCopy: string;
  textModels: string;
  textSubline: string;
  traceable: string;
  useModel: (modelName: string) => string;
  video: string;
  videoCopy: string;
  videoModels: string;
  videoSubline: string;
  viewAllPrices: string;
  voiceKicker: string;
  voiceTitle: string;
  voiceSub: string;
  whyKicker: string;
  whyTitle: string;
  zeroRetention: string;
}> = {
  es: {
    all: "Todos los modelos", allDir: "Directorio de modelos", audio: "Audio", audioCopy: "Reconocimiento, síntesis, comprensión y transcripción de voz bajo una sola política de presupuesto.", audioModels: "Modelos de audio", audioSubline: "Voz y comprensión", audited: "Control auditado", balance: "Saldo compartido", billing: "facturación observable", browseAll: "Ver todos los modelos", carousel: "Carrusel de modelos multimodales", cliTitle: "Conecta tu CLI en tres pasos.", cliSub: "Instala la CLI, inicia sesión con una key y deja que Codex o Claude Code llamen modelos con un mismo registro.", contact: "Contactar ventas", cost: "coste por solicitud", debug: "depuración", directory: "Directorio en vivo", docs: "Leer docs", endpoint: "endpoint oficial", failure: "sin cargo por fallos", finalKicker: "PRUEBA DE MARCA · BAY AREA", finalTitle: "Lleva IA multimodal a producción.", finalSub: "Acceso a modelos, precios, uso, SLA, facturas y permisos en un flujo de producción.", guide: "Abrir guía CLI", image: "Imagen", imageCopy: "Pósters, visuales de producto, ilustraciones y generación creativa por lotes.", imageModels: "Modelos de imagen", imageSubline: "Visuales de calidad", ledger: "Registro por solicitud", modelFlowKicker: "FLUJO DE MODELOS MULTIMODALES", modelFlowTitle: "Conecta vídeo, imagen, texto y audio en un flujo de producto.", modelFlowSub: "Elige cada tipo de modelo por separado y comparte key, saldo, permisos y registro de solicitudes.", models: "modelos", official: "Oficial", openAudio: "Abrir modelos de audio", openImage: "Abrir modelos de imagen", openText: "Abrir modelos de texto", openVideo: "Abrir modelos de vídeo", priceKicker: "RESUMEN DE PRECIOS", priceTitle: "Ve primero el coste real del modelo.", priceSub: "Modelos representativos del directorio en vivo, con precios oficiales de referencia y precios Flatkey.", request: " / solicitud", security: "Programa de seguridad", sharedBalance: "Saldo prepago compartido", signedSla: "SLA disponible", switchType: "Cambiar tipo de carrusel", test: "Probar", text: "Texto", textCopy: "Chat, razonamiento, agentes de código y contexto largo enrutados por política de equipo.", textModels: "Modelos de texto", textSubline: "Enrutamiento fiable", traceable: "Solicitudes trazables", useModel: (modelName) => `Usar ${modelName}`, video: "Vídeo", videoCopy: "Texto a vídeo, imagen a vídeo, clips publicitarios y activos en movimiento desde una API.", videoModels: "Modelos de vídeo", videoSubline: "Producción de movimiento", viewAllPrices: "Ver todos los precios", voiceKicker: "VOZ DEL CLIENTE", voiceTitle: "Lo que los equipos repiten.", voiceSub: "Los equipos valoran menos keys, gasto claro, migración rápida y depuración sencilla.", whyKicker: "POR QUÉ FLATKEY", whyTitle: "Los equipos necesitan una pasarela de modelos controlable.", zeroRetention: "Retención cero",
  },
  fr: {
    all: "Tous les modèles", allDir: "Catalogue de modèles", audio: "Audio", audioCopy: "Reconnaissance, synthèse, compréhension audio et transcription sous une seule politique budgétaire.", audioModels: "Modèles audio", audioSubline: "Voix et compréhension", audited: "Contrôle audité", balance: "Solde partagé", billing: "facturation observable", browseAll: "Voir tous les modèles", carousel: "Carrousel de modèles multimodaux", cliTitle: "Connectez votre CLI en trois étapes.", cliSub: "Installez la CLI, connectez-vous avec une clé, puis laissez Codex ou Claude Code appeler les modèles avec un registre partagé.", contact: "Contacter les ventes", cost: "coût par requête", debug: "débogage", directory: "Catalogue en direct", docs: "Lire la doc", endpoint: "endpoint officiel", failure: "aucun frais d'échec", finalKicker: "PREUVE DE MARQUE · BAY AREA", finalTitle: "Mettez l'IA multimodale en production.", finalSub: "Accès modèles, prix, usage, SLA, factures et permissions dans un seul flux de production.", guide: "Ouvrir le guide CLI", image: "Image", imageCopy: "Affiches, visuels produit, illustrations et génération créative en lot.", imageModels: "Modèles image", imageSubline: "Visuels de qualité", ledger: "Registre par requête", modelFlowKicker: "FLUX DE MODÈLES MULTIMODAUX", modelFlowTitle: "Reliez vidéo, image, texte et audio dans un flux produit.", modelFlowSub: "Choisissez chaque type de modèle séparément tout en partageant clé, solde, permissions et journal des requêtes.", models: "modèles", official: "Officiel", openAudio: "Ouvrir les modèles audio", openImage: "Ouvrir les modèles image", openText: "Ouvrir les modèles texte", openVideo: "Ouvrir les modèles vidéo", priceKicker: "APERÇU DES PRIX", priceTitle: "Voyez d'abord le coût réel du modèle.", priceSub: "Modèles représentatifs du catalogue en direct, avec prix officiels de référence et prix Flatkey.", request: " / requête", security: "Programme sécurité", sharedBalance: "Solde prépayé partagé", signedSla: "SLA disponible", switchType: "Changer le type du carrousel", test: "Tester", text: "Texte", textCopy: "Chat, raisonnement, agents de code et longs contextes routés par politique d'équipe.", textModels: "Modèles texte", textSubline: "Routage fiable", traceable: "Requêtes traçables", useModel: (modelName) => `Utiliser ${modelName}`, video: "Vidéo", videoCopy: "Texte-vers-vidéo, image-vers-vidéo, clips publicitaires et assets animés via une API.", videoModels: "Modèles vidéo", videoSubline: "Production de mouvement", viewAllPrices: "Voir tous les prix", voiceKicker: "VOIX CLIENT", voiceTitle: "Ce que les équipes répètent.", voiceSub: "Les équipes cherchent moins de clés, des dépenses lisibles, une migration rapide et un débogage simple.", whyKicker: "POURQUOI FLATKEY", whyTitle: "Les équipes ont besoin d'une passerelle de modèles contrôlable.", zeroRetention: "Zéro rétention",
  },
  pt: {
    all: "Todos os modelos", allDir: "Diretório de modelos", audio: "Áudio", audioCopy: "Reconhecimento, síntese, compreensão de áudio e transcrição sob uma única política de orçamento.", audioModels: "Modelos de áudio", audioSubline: "Voz e compreensão", audited: "Controlo auditado", balance: "Saldo partilhado", billing: "faturação observável", browseAll: "Ver todos os modelos", carousel: "Carrossel de modelos multimodais", cliTitle: "Ligue a sua CLI em três passos.", cliSub: "Instale a CLI, entre com uma key e deixe Codex ou Claude Code chamar modelos com um ledger partilhado.", contact: "Falar com vendas", cost: "custo por pedido", debug: "depuração", directory: "Diretório em direto", docs: "Ler docs", endpoint: "endpoint oficial", failure: "sem cobrança por falhas", finalKicker: "PROVA DE MARCA · BAY AREA", finalTitle: "Leve IA multimodal para produção.", finalSub: "Acesso a modelos, preços, uso, SLA, faturas e permissões num fluxo de produção.", guide: "Abrir guia CLI", image: "Imagem", imageCopy: "Posters, visuais de produto, ilustrações e geração criativa em lote.", imageModels: "Modelos de imagem", imageSubline: "Visuais de qualidade", ledger: "Ledger por pedido", modelFlowKicker: "FLUXO DE MODELOS MULTIMODAIS", modelFlowTitle: "Ligue vídeo, imagem, texto e áudio num fluxo de produto.", modelFlowSub: "Escolha cada tipo de modelo separadamente e partilhe key, saldo, permissões e registo de pedidos.", models: "modelos", official: "Oficial", openAudio: "Abrir modelos de áudio", openImage: "Abrir modelos de imagem", openText: "Abrir modelos de texto", openVideo: "Abrir modelos de vídeo", priceKicker: "RESUMO DE PREÇOS", priceTitle: "Veja primeiro o custo real do modelo.", priceSub: "Modelos representativos do diretório em direto, com preços oficiais de referência e preços Flatkey.", request: " / pedido", security: "Programa de segurança", sharedBalance: "Saldo pré-pago partilhado", signedSla: "SLA disponível", switchType: "Mudar tipo do carrossel", test: "Testar", text: "Texto", textCopy: "Chat, raciocínio, agentes de código e contexto longo roteados por política da equipa.", textModels: "Modelos de texto", textSubline: "Roteamento fiável", traceable: "Pedidos rastreáveis", useModel: (modelName) => `Usar ${modelName}`, video: "Vídeo", videoCopy: "Texto para vídeo, imagem para vídeo, clips de anúncio e assets animados numa API.", videoModels: "Modelos de vídeo", videoSubline: "Produção de movimento", viewAllPrices: "Ver todos os preços", voiceKicker: "VOZ DO CLIENTE", voiceTitle: "O que as equipas repetem.", voiceSub: "As equipas querem menos keys, gasto claro, migração rápida e depuração simples.", whyKicker: "PORQUÊ FLATKEY", whyTitle: "As equipas precisam de uma gateway de modelos controlável.", zeroRetention: "Retenção zero",
  },
  ru: {
    all: "Все модели", allDir: "Каталог моделей", audio: "Аудио", audioCopy: "Распознавание, синтез, понимание аудио и транскрибация в одной бюджетной политике.", audioModels: "Аудиомодели", audioSubline: "Речь и понимание", audited: "Аудированный контроль", balance: "Общий баланс", billing: "наблюдаемые счета", browseAll: "Смотреть все модели", carousel: "Карусель мультимодальных моделей", cliTitle: "Подключите CLI за три шага.", cliSub: "Установите CLI, войдите с одним ключом и дайте Codex или Claude Code вызывать модели с общим журналом.", contact: "Связаться с продажами", cost: "стоимость запроса", debug: "отладка", directory: "Живой каталог", docs: "Читать docs", endpoint: "официальный endpoint", failure: "без платы за сбои", finalKicker: "ДОВЕРИЕ БРЕНДА · BAY AREA", finalTitle: "Запустите мультимодальный ИИ в продакшн.", finalSub: "Доступ к моделям, цены, usage, SLA, счета и права в одном production-процессе.", guide: "Открыть CLI-гайд", image: "Изображения", imageCopy: "Постеры, продуктовые визуалы, иллюстрации и пакетная генерация креативов.", imageModels: "Модели изображений", imageSubline: "Качественные визуалы", ledger: "Журнал по запросам", modelFlowKicker: "ПОТОК МУЛЬТИМОДАЛЬНЫХ МОДЕЛЕЙ", modelFlowTitle: "Соедините видео, изображения, текст и аудио в один продуктовый поток.", modelFlowSub: "Выбирайте типы моделей отдельно, но используйте один ключ, баланс, права и журнал запросов.", models: "модели", official: "Официальная", openAudio: "Открыть аудиомодели", openImage: "Открыть модели изображений", openText: "Открыть текстовые модели", openVideo: "Открыть видеомодели", priceKicker: "СНИМОК ЦЕН", priceTitle: "Сначала оцените реальную стоимость модели.", priceSub: "Репрезентативные модели из живого каталога: официальные ориентиры и цены Flatkey.", request: " / запрос", security: "Программа безопасности", sharedBalance: "Общий предоплаченный баланс", signedSla: "SLA доступен", switchType: "Переключить тип карусели", test: "Тест", text: "Текст", textCopy: "Чат, рассуждение, code agents и длинный контекст маршрутизируются по политике команды.", textModels: "Текстовые модели", textSubline: "Надежный роутинг", traceable: "Трассируемые запросы", useModel: (modelName) => `Использовать ${modelName}`, video: "Видео", videoCopy: "Text-to-video, image-to-video, рекламные клипы и motion assets через один API.", videoModels: "Видеомодели", videoSubline: "Motion production", viewAllPrices: "Смотреть все цены", voiceKicker: "ОТЗЫВЫ КЛИЕНТОВ", voiceTitle: "Что команды повторяют чаще всего.", voiceSub: "Команды ценят меньше ключей, понятные траты, быструю миграцию и простую отладку.", whyKicker: "ПОЧЕМУ FLATKEY", whyTitle: "Командам нужен управляемый шлюз моделей.", zeroRetention: "Нулевая ретенция",
  },
  ja: {
    all: "すべてのモデル", allDir: "モデルディレクトリ", audio: "音声", audioCopy: "音声認識、合成、理解、文字起こしを 1 つの予算ポリシーで管理します。", audioModels: "音声モデル", audioSubline: "音声と理解", audited: "監査済み管理", balance: "共通残高", billing: "請求を可視化", browseAll: "すべてのモデルを見る", carousel: "マルチモーダルモデルのカルーセル", cliTitle: "CLI を 3 ステップで接続。", cliSub: "CLI をインストールし、1 つの key でログイン。Codex や Claude Code のモデル呼び出しを同じ台帳に記録します。", contact: "営業に問い合わせ", cost: "リクエスト単位のコスト", debug: "デバッグ", directory: "ライブディレクトリ", docs: "ドキュメントを読む", endpoint: "公式 endpoint", failure: "失敗時は課金なし", finalKicker: "ブランド実績 · BAY AREA", finalTitle: "マルチモーダル AI を本番へ。", finalSub: "モデルアクセス、料金、利用量、SLA、請求、権限を 1 つの本番ワークフローで管理します。", guide: "CLI ガイドを開く", image: "画像", imageCopy: "ポスター、商品画像、イラスト、量産クリエイティブを生成します。", imageModels: "画像モデル", imageSubline: "高品質ビジュアル", ledger: "リクエスト台帳", modelFlowKicker: "マルチモーダルモデルフロー", modelFlowTitle: "動画、画像、テキスト、音声を 1 つのプロダクトフローへ。", modelFlowSub: "モデル種別ごとに選びながら、key、残高、権限、リクエストログを共有できます。", models: "モデル", official: "公式", openAudio: "音声モデルを開く", openImage: "画像モデルを開く", openText: "テキストモデルを開く", openVideo: "動画モデルを開く", priceKicker: "モデル価格スナップショット", priceTitle: "まずモデルの実コストを確認。", priceSub: "ライブディレクトリから代表モデルを抽出し、公式参考価格と Flatkey 価格を表示します。", request: " / リクエスト", security: "セキュリティ体制", sharedBalance: "共通プリペイド残高", signedSla: "SLA 利用可", switchType: "カルーセル種別を切り替え", test: "試す", text: "テキスト", textCopy: "チャット、推論、コード Agent、長文コンテキストをチームポリシーでルーティングします。", textModels: "テキストモデル", textSubline: "安定ルーティング", traceable: "追跡可能なリクエスト", useModel: (modelName) => `${modelName} を使う`, video: "動画", videoCopy: "Text-to-video、image-to-video、広告クリップ、モーション素材を 1 つの API で扱えます。", videoModels: "動画モデル", videoSubline: "映像制作", viewAllPrices: "すべての価格を見る", voiceKicker: "顧客の声", voiceTitle: "チームが繰り返し語る価値。", voiceSub: "重要なのはモデル数だけでなく、key の少なさ、明確な支出、速い移行、簡単なデバッグです。", whyKicker: "FLATKEY が選ばれる理由", whyTitle: "チームには制御できるモデルゲートウェイが必要です。", zeroRetention: "リクエスト内容ゼロ保持",
  },
  vi: {
    all: "Tất cả model", allDir: "Thư mục model", audio: "Âm thanh", audioCopy: "Nhận dạng, tổng hợp, hiểu âm thanh và phiên âm trong một chính sách ngân sách.", audioModels: "Model âm thanh", audioSubline: "Giọng nói và hiểu biết", audited: "Kiểm soát đã audit", balance: "Số dư dùng chung", billing: "hóa đơn quan sát được", browseAll: "Xem tất cả model", carousel: "Carousel model đa phương thức", cliTitle: "Kết nối CLI trong ba bước.", cliSub: "Cài CLI, đăng nhập bằng một key, rồi để Codex hoặc Claude Code gọi model với một ledger chung.", contact: "Liên hệ bán hàng", cost: "chi phí mỗi request", debug: "debug", directory: "Thư mục live", docs: "Đọc docs", endpoint: "endpoint chính thức", failure: "không tính phí lỗi", finalKicker: "BẰNG CHỨNG THƯƠNG HIỆU · BAY AREA", finalTitle: "Đưa AI đa phương thức vào production.", finalSub: "Truy cập model, giá, usage, SLA, hóa đơn và quyền trong một workflow production.", guide: "Mở hướng dẫn CLI", image: "Hình ảnh", imageCopy: "Poster, ảnh sản phẩm, minh họa và tạo creative hàng loạt.", imageModels: "Model hình ảnh", imageSubline: "Visual chất lượng", ledger: "Ledger theo request", modelFlowKicker: "LUỒNG MODEL ĐA PHƯƠNG THỨC", modelFlowTitle: "Kết nối video, hình ảnh, văn bản và âm thanh vào một luồng sản phẩm.", modelFlowSub: "Chọn từng loại model riêng nhưng dùng chung key, số dư, quyền và log request.", models: "model", official: "Chính thức", openAudio: "Mở model âm thanh", openImage: "Mở model hình ảnh", openText: "Mở model văn bản", openVideo: "Mở model video", priceKicker: "ẢNH CHỤP GIÁ MODEL", priceTitle: "Xem chi phí model thật trước.", priceSub: "Các model đại diện từ thư mục live, với giá tham chiếu chính thức và giá Flatkey.", request: " / request", security: "Chương trình bảo mật", sharedBalance: "Số dư trả trước chung", signedSla: "Có SLA", switchType: "Đổi loại carousel", test: "Thử", text: "Văn bản", textCopy: "Chat, reasoning, coding agent và tác vụ context dài được route theo policy đội nhóm.", textModels: "Model văn bản", textSubline: "Routing ổn định", traceable: "Request truy vết được", useModel: (modelName) => `Dùng ${modelName}`, video: "Video", videoCopy: "Text-to-video, image-to-video, clip quảng cáo và motion asset qua một API.", videoModels: "Model video", videoSubline: "Sản xuất chuyển động", viewAllPrices: "Xem toàn bộ giá", voiceKicker: "TIẾNG NÓI KHÁCH HÀNG", voiceTitle: "Điều các đội nhắc lại.", voiceSub: "Đội nhóm quan tâm key ít hơn, chi phí rõ hơn, migrate nhanh hơn và debug dễ hơn.", whyKicker: "VÌ SAO CHỌN FLATKEY", whyTitle: "Đội nhóm cần một model gateway có thể kiểm soát.", zeroRetention: "Không lưu nội dung",
  },
  de: {
    all: "Alle Modelle", allDir: "Modellverzeichnis", audio: "Audio", audioCopy: "Spracherkennung, Synthese, Audio-Verständnis und Transkription unter einer Budget-Policy.", audioModels: "Audio-Modelle", audioSubline: "Sprache und Verständnis", audited: "Auditierte Kontrolle", balance: "Gemeinsames Guthaben", billing: "beobachtbare Abrechnung", browseAll: "Alle Modelle ansehen", carousel: "Multimodales Modellkarussell", cliTitle: "Verbinde deine CLI in drei Schritten.", cliSub: "CLI installieren, mit einem Key anmelden und Codex oder Claude Code Modelle mit gemeinsamem Ledger aufrufen lassen.", contact: "Vertrieb kontaktieren", cost: "Kosten pro Request", debug: "Debugging", directory: "Live-Verzeichnis", docs: "Docs lesen", endpoint: "offizieller Endpoint", failure: "keine Kosten bei Fehlern", finalKicker: "MARKENBELEG · BAY AREA", finalTitle: "Bring multimodale KI in Produktion.", finalSub: "Modellzugriff, Preise, Nutzung, SLA, Rechnungen und Rechte in einem Produktionsworkflow.", guide: "CLI-Guide öffnen", image: "Bild", imageCopy: "Poster, Produktvisuals, Illustrationen und kreative Batch-Generierung.", imageModels: "Bildmodelle", imageSubline: "Hochwertige Visuals", ledger: "Ledger pro Request", modelFlowKicker: "MULTIMODALER MODELLFLOW", modelFlowTitle: "Verbinde Video, Bild, Text und Audio in einem Produktflow.", modelFlowSub: "Wähle jeden Modelltyp separat und teile Key, Guthaben, Rechte und Request-Logs.", models: "Modelle", official: "Offiziell", openAudio: "Audio-Modelle öffnen", openImage: "Bildmodelle öffnen", openText: "Textmodelle öffnen", openVideo: "Videomodelle öffnen", priceKicker: "MODELLPREIS-SNAPSHOT", priceTitle: "Sieh zuerst die echten Modellkosten.", priceSub: "Repräsentative Modelle aus dem Live-Verzeichnis mit offiziellen Referenzpreisen und Flatkey-Preisen.", request: " / Request", security: "Security-Programm", sharedBalance: "Gemeinsames Prepaid-Guthaben", signedSla: "SLA verfügbar", switchType: "Karusselltyp wechseln", test: "Testen", text: "Text", textCopy: "Chat, Reasoning, Coding Agents und Long-Context-Aufgaben werden per Team-Policy geroutet.", textModels: "Textmodelle", textSubline: "Zuverlässiges Routing", traceable: "Nachverfolgbare Requests", useModel: (modelName) => `${modelName} nutzen`, video: "Video", videoCopy: "Text-to-Video, Image-to-Video, Anzeigenclips und Motion Assets über eine API.", videoModels: "Videomodelle", videoSubline: "Motion-Produktion", viewAllPrices: "Alle Preise ansehen", voiceKicker: "KUNDENSTIMMEN", voiceTitle: "Was Teams immer wieder sagen.", voiceSub: "Teams wollen weniger Keys, klare Kosten, schnellere Migration und einfacheres Debugging.", whyKicker: "WARUM FLATKEY", whyTitle: "Teams brauchen ein kontrollierbares Modell-Gateway.", zeroRetention: "Keine Speicherung",
  },
  id: {
    all: "Semua model", allDir: "Direktori model", audio: "Audio", audioCopy: "Pengenalan, sintesis, pemahaman audio, dan transkripsi dalam satu kebijakan anggaran.", audioModels: "Model audio", audioSubline: "Suara dan pemahaman", audited: "Kontrol teraudit", balance: "Saldo bersama", billing: "billing terpantau", browseAll: "Lihat semua model", carousel: "Carousel model multimodal", cliTitle: "Hubungkan CLI dalam tiga langkah.", cliSub: "Instal CLI, masuk dengan satu key, lalu biarkan Codex atau Claude Code memanggil model dengan ledger bersama.", contact: "Hubungi sales", cost: "biaya per request", debug: "debug", directory: "Direktori live", docs: "Baca docs", endpoint: "endpoint resmi", failure: "tanpa biaya kegagalan", finalKicker: "BUKTI BRAND · BAY AREA", finalTitle: "Bawa AI multimodal ke produksi.", finalSub: "Akses model, harga, penggunaan, SLA, invoice, dan izin dalam satu workflow produksi.", guide: "Buka panduan CLI", image: "Gambar", imageCopy: "Poster, visual produk, ilustrasi, dan creative generation massal.", imageModels: "Model gambar", imageSubline: "Visual berkualitas", ledger: "Ledger per request", modelFlowKicker: "ALUR MODEL MULTIMODAL", modelFlowTitle: "Hubungkan video, gambar, teks, dan audio ke satu alur produk.", modelFlowSub: "Pilih tiap tipe model secara terpisah sambil berbagi key, saldo, izin, dan log request.", models: "model", official: "Resmi", openAudio: "Buka model audio", openImage: "Buka model gambar", openText: "Buka model teks", openVideo: "Buka model video", priceKicker: "RINGKASAN HARGA MODEL", priceTitle: "Lihat biaya model sebenarnya lebih dulu.", priceSub: "Model representatif dari direktori live, dengan harga referensi resmi dan harga Flatkey.", request: " / request", security: "Program keamanan", sharedBalance: "Saldo prabayar bersama", signedSla: "SLA tersedia", switchType: "Ganti tipe carousel", test: "Tes", text: "Teks", textCopy: "Chat, reasoning, coding agent, dan konteks panjang dirutekan dengan policy tim.", textModels: "Model teks", textSubline: "Routing stabil", traceable: "Request dapat dilacak", useModel: (modelName) => `Gunakan ${modelName}`, video: "Video", videoCopy: "Text-to-video, image-to-video, klip iklan, dan motion asset lewat satu API.", videoModels: "Model video", videoSubline: "Produksi motion", viewAllPrices: "Lihat semua harga", voiceKicker: "SUARA PELANGGAN", voiceTitle: "Yang sering diulang tim.", voiceSub: "Tim peduli key lebih sedikit, biaya lebih jelas, migrasi lebih cepat, dan debug lebih mudah.", whyKicker: "MENGAPA FLATKEY", whyTitle: "Tim butuh gateway model yang dapat dikontrol.", zeroRetention: "Retensi nol",
  },
};

const HOME_TRANSLATED_SECTIONS: Record<HomeTranslatedLocale, Pick<HomePageCopy, "cli" | "voice" | "final"> & { oneGateway: string; typeEntry: string; whyCards: HomePageCopy["why"]["cards"] }> = {
  es: {
    oneGateway: "Una pasarela",
    typeEntry: "Entrada por tipo",
    cli: { aria: "Pasos rápidos de CLI", checks: ["Una API Key", "Saldo y registro compartidos", "Llamadas trazables"], docs: "Leer docs", guide: "Abrir guía CLI", kicker: "CLI QUICKSTART", range: "de instalación a primera llamada", stepsDone: "3 pasos", steps: [{ body: "Instala Flatkey CLI de forma global para usarla desde la terminal.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "Instalar CLI" }, { body: "Copia una API Key desde la consola y evita claves de proveedores en agentes locales.", code: "flatkey login", no: "02", title: "Iniciar sesión" }, { body: "Deja que Codex, Claude Code o tus scripts llamen modelos con el mismo registro.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Usar modelos" }], sub: "Instala la CLI, inicia sesión con una key y deja que Codex o Claude Code llamen modelos con un registro compartido.", title: "Conecta tu CLI en tres pasos." },
    whyCards: [{ body: "Los nombres, proveedores y precios salen del mismo directorio en vivo. Las llamadas van al endpoint configurado sin cambios silenciosos de modelo.", chips: ["Endpoints oficiales", "Directorio sincronizado", "Probable"], metric: "01", title: "Saber qué modelo se llama" }, { body: "Fallos, latencia, proveedor y decisiones de ruta quedan unidos al log de la solicitud.", chips: ["Failover", "Logs", "Estado"], metric: "02", title: "Depurar desde la solicitud" }, { body: "Un saldo prepago cubre todos los tipos de modelo y el uso queda registrado para conciliación.", chips: ["Saldo compartido", "Sin cargo por fallos", "Registro"], metric: "03", title: "Explicar el coste por solicitud" }, { body: "Las subkeys llevan presupuestos, listas permitidas, facturas, revisiones de seguridad y política de retención.", chips: ["Subkey", "Lista permitida", "Facturas"], metric: "04", title: "Gestionar acceso por equipo" }],
    voice: { aria: "Carrusel de testimonios", boardCopy: "Flatkey reúne llamadas, presupuestos, fallos y facturas para que el ROI sea más fácil de evaluar.", boardMetric: "Repetido por equipos", boardTitle: "Menos keys, menos incidencias, gasto claro", faqs: [["¿Son endpoints oficiales?", "Sí. Flatkey unifica routing, facturación y gobierno sin cambios silenciosos de modelo."], ["¿Cómo se prueban los modelos?", "Cada tarjeta abre la página del modelo con edición de prompt y registro para ejecutar."], ["¿De dónde sale el precio?", "La home toma filas representativas del mismo directorio de precios."], ["¿Se puede navegar por tipo?", "La home muestra entradas para vídeo, texto, audio e imagen."]], items: [{ metric: "Menos keys", quote: "Ya no guardamos claves separadas de proveedores dentro de cada agente.", role: "Equipo de apps IA" }, { metric: "Gasto claro", quote: "Vídeo, texto e imagen usan el mismo saldo y registro.", role: "Equipo de crecimiento" }, { metric: "Migración rápida", quote: "La interfaz compatible con OpenAI se mantiene, así que cambiar modelos no reescribe el producto.", role: "Equipo de plataforma" }, { metric: "Depuración rápida", quote: "Fallos, modelos, proveedores y registros de facturación viven juntos.", role: "Operaciones" }], kicker: "VOZ DEL CLIENTE", signals: ["Endpoints oficiales", "Saldo compartido", "Registro", "Sin cargo por fallos", "Lista permitida", "SLA"], sub: "Los equipos valoran menos keys, gasto claro, migración rápida y depuración sencilla.", title: "Lo que los equipos repiten." },
    final: { alt: "Anuncio exterior de Flatkey en Bay Area", kicker: "PRUEBA DE MARCA · BAY AREA", launch: [["01", "Crear API Key", "Genera keys de equipo y separa políticas por entorno."], ["02", "Elegir modelos", "Revisa endpoints, precios y grupos disponibles en el directorio."], ["03", "Observar producción", "Controla tráfico con logs, presupuestos y cobertura SLA."]], metrics: ["Control auditado", "Programa de seguridad", "SLA disponible"], sub: "Acceso a modelos, precios, uso, SLA, facturas y permisos en un flujo de producción.", title: "Lleva IA multimodal a producción.", trustZeroRetention: "Retención cero" },
  },
  fr: {
    oneGateway: "Une passerelle", typeEntry: "Entrée par type",
    cli: { aria: "Étapes rapides CLI", checks: ["Une API Key", "Solde et registre partagés", "Appels traçables"], docs: "Lire la doc", guide: "Ouvrir le guide CLI", kicker: "CLI QUICKSTART", range: "de l'installation au premier appel", stepsDone: "3 étapes", steps: [{ body: "Installez Flatkey CLI globalement pour l'utiliser depuis le terminal.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "Installer la CLI" }, { body: "Copiez une API Key depuis la console et gardez les clés fournisseurs hors des agents locaux.", code: "flatkey login", no: "02", title: "Se connecter" }, { body: "Laissez Codex, Claude Code ou vos scripts appeler les modèles avec le même registre.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Utiliser les modèles" }], sub: "Installez la CLI, connectez-vous avec une clé, puis laissez Codex ou Claude Code appeler les modèles avec un registre partagé.", title: "Connectez votre CLI en trois étapes." },
    whyCards: [{ body: "Noms, fournisseurs et prix viennent du même catalogue en direct. Les appels vont vers l'endpoint configuré sans remplacement silencieux.", chips: ["Endpoints officiels", "Catalogue synchronisé", "Testable"], metric: "01", title: "Savoir quel modèle est appelé" }, { body: "Échecs, latence, fournisseur et décisions de routage restent liés au journal de requête.", chips: ["Failover", "Journaux", "Statut"], metric: "02", title: "Déboguer depuis la requête" }, { body: "Un solde prépayé couvre tous les types de modèles et l'usage est enregistré pour rapprochement.", chips: ["Solde partagé", "Échec non facturé", "Registre"], metric: "03", title: "Expliquer le coût par requête" }, { body: "Les sous-clés portent budgets, listes autorisées, factures, revues sécurité et politique de rétention.", chips: ["Sous-clé", "Liste autorisée", "Factures"], metric: "04", title: "Gérer l'accès par équipe" }],
    voice: { aria: "Carrousel des retours clients", boardCopy: "Flatkey regroupe appels, budgets, échecs et factures pour évaluer le ROI plus vite.", boardMetric: "Répété par les équipes", boardTitle: "Moins de clés, moins d'incidents, dépenses claires", faqs: [["S'agit-il d'endpoints officiels ?", "Oui. Flatkey unifie routage, facturation et gouvernance sans remplacement silencieux."], ["Comment tester un modèle ?", "Chaque carte ouvre la page du modèle avec édition de prompt et inscription pour exécuter."], ["D'où viennent les prix ?", "La page prélève des lignes représentatives du même catalogue tarifaire."], ["Peut-on parcourir par type ?", "La page propose des entrées vidéo, texte, audio et image."]], items: [{ metric: "Moins de clés", quote: "Nous ne gardons plus des clés fournisseurs séparées dans chaque agent.", role: "Équipe apps IA" }, { metric: "Dépenses claires", quote: "Vidéo, texte et image passent par le même solde et registre.", role: "Équipe growth" }, { metric: "Migration rapide", quote: "L'interface compatible OpenAI reste stable, donc changer de modèle ne réécrit pas le produit.", role: "Équipe plateforme" }, { metric: "Débogage rapide", quote: "Échecs, modèles, fournisseurs et facturation sont au même endroit.", role: "Ops" }], kicker: "VOIX CLIENT", signals: ["Endpoints officiels", "Solde partagé", "Registre", "Échec non facturé", "Liste autorisée", "SLA"], sub: "Les équipes cherchent moins de clés, des dépenses lisibles, une migration rapide et un débogage simple.", title: "Ce que les équipes répètent." },
    final: { alt: "Panneau Flatkey dans la Bay Area", kicker: "PREUVE DE MARQUE · BAY AREA", launch: [["01", "Créer une API Key", "Générez des clés d'équipe et séparez les politiques par environnement."], ["02", "Choisir les modèles", "Vérifiez endpoints, prix et groupes disponibles dans le catalogue."], ["03", "Observer la production", "Contrôlez le trafic avec logs, budgets et couverture SLA."]], metrics: ["Contrôle audité", "Programme sécurité", "SLA disponible"], sub: "Accès modèles, prix, usage, SLA, factures et permissions dans un seul flux de production.", title: "Mettez l'IA multimodale en production.", trustZeroRetention: "Zéro rétention" },
  },
  pt: {
    oneGateway: "Uma gateway", typeEntry: "Entrada por tipo",
    cli: { aria: "Passos rápidos da CLI", checks: ["Uma API Key", "Saldo e ledger partilhados", "Chamadas rastreáveis"], docs: "Ler docs", guide: "Abrir guia CLI", kicker: "CLI QUICKSTART", range: "da instalação à primeira chamada", stepsDone: "3 passos", steps: [{ body: "Instale a Flatkey CLI globalmente para usar no terminal.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "Instalar CLI" }, { body: "Copie uma API Key da consola e mantenha chaves de fornecedores fora dos agentes locais.", code: "flatkey login", no: "02", title: "Entrar" }, { body: "Deixe Codex, Claude Code ou scripts chamar modelos com o mesmo ledger.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Usar modelos" }], sub: "Instale a CLI, entre com uma key e deixe Codex ou Claude Code chamar modelos com um ledger partilhado.", title: "Ligue a sua CLI em três passos." },
    whyCards: [{ body: "Nomes, fornecedores e preços vêm do mesmo diretório em direto. As chamadas vão para o endpoint configurado sem troca silenciosa.", chips: ["Endpoints oficiais", "Diretório sincronizado", "Testável"], metric: "01", title: "Saber que modelo é chamado" }, { body: "Falhas, latência, fornecedor e decisões de rota ficam ligados ao log do pedido.", chips: ["Failover", "Logs", "Status"], metric: "02", title: "Depurar a partir do pedido" }, { body: "Um saldo pré-pago cobre todos os tipos de modelos e o uso fica registado para reconciliação.", chips: ["Saldo partilhado", "Falha sem custo", "Ledger"], metric: "03", title: "Explicar custo por pedido" }, { body: "Subkeys carregam orçamentos, allowlists, faturas, reviews de segurança e política de retenção.", chips: ["Subkey", "Allowlist", "Faturas"], metric: "04", title: "Gerir acesso por equipa" }],
    voice: { aria: "Carrossel de feedback", boardCopy: "Flatkey reúne chamadas, orçamentos, falhas e faturas para avaliar ROI mais depressa.", boardMetric: "Repetido pelas equipas", boardTitle: "Menos keys, menos incidentes, gasto claro", faqs: [["São endpoints oficiais?", "Sim. Flatkey unifica routing, faturação e governance sem troca silenciosa de modelos."], ["Como testar modelos?", "Cada card abre a página do modelo com edição de prompt e registo para executar."], ["De onde vêm os preços?", "A homepage usa linhas representativas do mesmo diretório de preços."], ["Dá para navegar por tipo?", "A homepage mostra entradas de vídeo, texto, áudio e imagem."]], items: [{ metric: "Menos keys", quote: "Já não guardamos chaves de fornecedores em cada agente.", role: "Equipa de apps IA" }, { metric: "Gasto claro", quote: "Vídeo, texto e imagem usam o mesmo saldo e ledger.", role: "Equipa growth" }, { metric: "Migração rápida", quote: "A interface compatível com OpenAI mantém-se, por isso trocar modelos não reescreve o produto.", role: "Equipa plataforma" }, { metric: "Debug rápido", quote: "Falhas, modelos, fornecedores e faturação ficam juntos.", role: "Ops" }], kicker: "VOZ DO CLIENTE", signals: ["Endpoints oficiais", "Saldo partilhado", "Ledger", "Falha sem custo", "Allowlist", "SLA"], sub: "As equipas querem menos keys, gasto claro, migração rápida e depuração simples.", title: "O que as equipas repetem." },
    final: { alt: "Outdoor Flatkey na Bay Area", kicker: "PROVA DE MARCA · BAY AREA", launch: [["01", "Criar API Key", "Gere keys de equipa e separe políticas por ambiente."], ["02", "Escolher modelos", "Confirme endpoints, preços e grupos disponíveis no diretório."], ["03", "Observar produção", "Controle tráfego com logs, orçamentos e cobertura SLA."]], metrics: ["Controlo auditado", "Programa de segurança", "SLA disponível"], sub: "Acesso a modelos, preços, uso, SLA, faturas e permissões num fluxo de produção.", title: "Leve IA multimodal para produção.", trustZeroRetention: "Retenção zero" },
  },
  ru: {
    oneGateway: "Один шлюз", typeEntry: "Вход по типу",
    cli: { aria: "Быстрый старт CLI", checks: ["Один API Key", "Общий баланс и журнал", "Трассируемые вызовы"], docs: "Читать docs", guide: "Открыть CLI-гайд", kicker: "CLI QUICKSTART", range: "от установки до первого вызова", stepsDone: "3 шага", steps: [{ body: "Установите Flatkey CLI глобально для работы из терминала.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "Установить CLI" }, { body: "Скопируйте API Key из консоли и не храните ключи провайдеров в локальных агентах.", code: "flatkey login", no: "02", title: "Войти" }, { body: "Позвольте Codex, Claude Code или скриптам вызывать модели с одним журналом.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Использовать модели" }], sub: "Установите CLI, войдите с одним ключом и дайте Codex или Claude Code вызывать модели с общим журналом.", title: "Подключите CLI за три шага." },
    whyCards: [{ body: "Имена моделей, провайдеры и цены берутся из одного живого каталога. Вызовы идут на настроенный endpoint без скрытых замен.", chips: ["Официальные endpoints", "Синхронный каталог", "Можно тестировать"], metric: "01", title: "Понимайте, какая модель вызывается" }, { body: "Сбои, задержка, провайдер и решения маршрута остаются в журнале запроса.", chips: ["Failover", "Журналы", "Статус"], metric: "02", title: "Отлаживайте по запросу" }, { body: "Один предоплаченный баланс покрывает типы моделей, а usage записывается для сверки.", chips: ["Общий баланс", "Сбой без оплаты", "Журнал"], metric: "03", title: "Объясняйте стоимость запроса" }, { body: "Sub-keys несут бюджеты, allowlist моделей, счета, проверки безопасности и правила ретенции.", chips: ["Sub-key", "Allowlist", "Счета"], metric: "04", title: "Управляйте доступом команд" }],
    voice: { aria: "Карусель отзывов клиентов", boardCopy: "Flatkey объединяет вызовы, бюджеты, сбои и счета, чтобы быстрее оценивать ROI.", boardMetric: "Команды повторяют", boardTitle: "Меньше ключей, меньше инцидентов, понятные расходы", faqs: [["Это официальные endpoints?", "Да. Flatkey объединяет routing, billing и governance без скрытой замены моделей."], ["Как тестировать модели?", "Каждая карточка открывает страницу модели с prompt-редактором и регистрацией для запуска."], ["Откуда берутся цены?", "Главная берет репрезентативные строки из того же каталога цен."], ["Можно смотреть по типу модели?", "На главной есть входы для видео, текста, аудио и изображений."]], items: [{ metric: "Меньше ключей", quote: "Мы больше не держим отдельные ключи провайдеров в каждом агенте.", role: "Команда AI-приложений" }, { metric: "Понятные расходы", quote: "Видео, текст и изображения идут через один баланс и журнал.", role: "Growth-команда" }, { metric: "Быстрая миграция", quote: "OpenAI-compatible интерфейс сохраняется, поэтому смена моделей не переписывает продукт.", role: "Платформенная команда" }, { metric: "Быстрая отладка", quote: "Сбои, модели, провайдеры и billing-записи находятся вместе.", role: "Ops-команда" }], kicker: "ОТЗЫВЫ КЛИЕНТОВ", signals: ["Официальные endpoints", "Общий баланс", "Журнал запросов", "Сбой без оплаты", "Allowlist", "SLA"], sub: "Команды ценят меньше ключей, понятные траты, быструю миграцию и простую отладку.", title: "Что команды повторяют чаще всего." },
    final: { alt: "Наружная реклама Flatkey в Bay Area", kicker: "ДОВЕРИЕ БРЕНДА · BAY AREA", launch: [["01", "Создать API Key", "Создайте командные ключи и разделите политики по окружениям."], ["02", "Выбрать модели", "Проверьте endpoints, цены и доступные группы в каталоге."], ["03", "Наблюдать production", "Контролируйте трафик через логи, бюджеты и SLA."]], metrics: ["Аудированный контроль", "Программа безопасности", "SLA доступен"], sub: "Доступ к моделям, цены, usage, SLA, счета и права в одном production-процессе.", title: "Запустите мультимодальный ИИ в продакшн.", trustZeroRetention: "Нулевая ретенция" },
  },
  ja: {
    oneGateway: "1 つのゲートウェイ", typeEntry: "タイプ別入口",
    cli: { aria: "CLI クイックスタート手順", checks: ["1 つの API Key", "共通残高と台帳", "追跡可能なモデル呼び出し"], docs: "ドキュメントを読む", guide: "CLI ガイドを開く", kicker: "CLI QUICKSTART", range: "インストールから初回実行まで", stepsDone: "3 ステップ", steps: [{ body: "Flatkey CLI をグローバルにインストールし、ターミナルから使えるようにします。", code: "npm i -g @flatkey-ai/cli", no: "01", title: "CLI をインストール" }, { body: "コンソールから API Key をコピーし、プロバイダー key をローカル Agent に置かないようにします。", code: "flatkey login", no: "02", title: "ログイン" }, { body: "Codex、Claude Code、スクリプトから同じ台帳でモデルを呼び出します。", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "モデルを使う" }], sub: "CLI をインストールし、1 つの key でログイン。Codex や Claude Code のモデル呼び出しを同じ台帳に記録します。", title: "CLI を 3 ステップで接続。" },
    whyCards: [{ body: "モデル名、プロバイダー、価格は同じライブディレクトリから取得します。呼び出しは設定済み endpoint に送られ、静かなモデル差し替えはありません。", chips: ["公式 endpoints", "ディレクトリ同期", "テスト可能"], metric: "01", title: "呼び出すモデルを確認" }, { body: "失敗、遅延、プロバイダー、ルーティング判断はリクエストログに残ります。", chips: ["Failover", "リクエストログ", "ステータス"], metric: "02", title: "リクエストから調査" }, { body: "1 つのプリペイド残高で各モデル種別を利用し、利用量は照合できる形で記録されます。", chips: ["共通残高", "失敗時は課金なし", "台帳"], metric: "03", title: "コストを説明可能に" }, { body: "Sub-key に予算、モデル allowlist、請求、セキュリティレビュー、保持ポリシーをまとめます。", chips: ["Sub-key", "Allowlist", "請求"], metric: "04", title: "チーム単位で権限管理" }],
    voice: { aria: "顧客フィードバックのカルーセル", boardCopy: "Flatkey はモデル呼び出し、予算、失敗、請求を 1 つの流れにまとめ、ROI 判断を速くします。", boardMetric: "チームが繰り返す声", boardTitle: "Key を減らし、障害を減らし、支出を明確に", faqs: [["公式 endpoint ですか？", "はい。Flatkey はルーティング、請求、ガバナンスを統合し、静かなモデル差し替えは行いません。"], ["モデルはどう試せますか？", "各カードからモデルページを開き、prompt を編集して登録後に実行できます。"], ["価格はどこから来ますか？", "同じ価格ディレクトリから代表的な行を抽出しています。"], ["モデル種別で探せますか？", "動画、テキスト、音声、画像の入口を用意しています。"]], items: [{ metric: "Key 削減", quote: "各 Agent にプロバイダー key を個別に持たせる必要がなくなりました。", role: "AI アプリチーム" }, { metric: "支出が明確", quote: "動画、テキスト、画像が同じ残高と台帳で管理されます。", role: "Growth チーム" }, { metric: "移行が速い", quote: "OpenAI 互換インターフェースのまま、モデル切り替えで製品コードを書き換えません。", role: "Platform チーム" }, { metric: "調査が速い", quote: "失敗、モデル、プロバイダー、請求記録が同じ場所にあります。", role: "Ops チーム" }], kicker: "顧客の声", signals: ["公式 endpoints", "共通残高", "リクエスト台帳", "失敗時は課金なし", "Allowlist", "SLA"], sub: "重要なのはモデル数だけでなく、key の少なさ、明確な支出、速い移行、簡単なデバッグです。", title: "チームが繰り返し語る価値。" },
    final: { alt: "Flatkey の Bay Area 屋外広告", kicker: "ブランド実績 · BAY AREA", launch: [["01", "API Key を作成", "チーム key を生成し、環境ごとにポリシーを分けます。"], ["02", "モデルを選ぶ", "ディレクトリで endpoint、価格、利用可能グループを確認します。"], ["03", "本番を観測", "ログ、予算、SLA で本番トラフィックを管理します。"]], metrics: ["監査済み管理", "セキュリティ体制", "SLA 利用可"], sub: "モデルアクセス、料金、利用量、SLA、請求、権限を 1 つの本番ワークフローで管理します。", title: "マルチモーダル AI を本番へ。", trustZeroRetention: "リクエスト内容ゼロ保持" },
  },
  vi: {
    oneGateway: "Một gateway", typeEntry: "Lối vào theo loại",
    cli: { aria: "Các bước CLI nhanh", checks: ["Một API Key", "Số dư và ledger chung", "Lượt gọi truy vết được"], docs: "Đọc docs", guide: "Mở hướng dẫn CLI", kicker: "CLI QUICKSTART", range: "từ cài đặt đến lượt gọi đầu", stepsDone: "3 bước", steps: [{ body: "Cài Flatkey CLI toàn cục để dùng ngay trong terminal.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "Cài CLI" }, { body: "Sao chép API Key từ console và không để key provider trong agent local.", code: "flatkey login", no: "02", title: "Đăng nhập" }, { body: "Để Codex, Claude Code hoặc script gọi model với cùng một ledger.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Dùng model" }], sub: "Cài CLI, đăng nhập bằng một key, rồi để Codex hoặc Claude Code gọi model với một ledger chung.", title: "Kết nối CLI trong ba bước." },
    whyCards: [{ body: "Tên model, nhà cung cấp và giá đến từ cùng thư mục live. Lượt gọi đi tới endpoint đã cấu hình, không đổi model âm thầm.", chips: ["Endpoint chính thức", "Đồng bộ thư mục", "Có thể test"], metric: "01", title: "Biết model nào được gọi" }, { body: "Lỗi, độ trễ, provider và quyết định route đều nằm trong log request.", chips: ["Failover", "Log request", "Trạng thái"], metric: "02", title: "Debug từ request" }, { body: "Một số dư trả trước dùng cho mọi loại model và usage được ghi lại để đối soát.", chips: ["Số dư chung", "Lỗi không tính phí", "Ledger"], metric: "03", title: "Giải thích chi phí từng request" }, { body: "Sub-key mang ngân sách, allowlist model, hóa đơn, review bảo mật và policy lưu giữ.", chips: ["Sub-key", "Allowlist", "Hóa đơn"], metric: "04", title: "Quản lý quyền theo đội" }],
    voice: { aria: "Carousel phản hồi khách hàng", boardCopy: "Flatkey gom lượt gọi, ngân sách, lỗi và hóa đơn vào một workflow để đánh giá ROI nhanh hơn.", boardMetric: "Điều đội nhóm nhắc lại", boardTitle: "Ít key hơn, ít sự cố hơn, chi phí rõ hơn", faqs: [["Đây có phải endpoint chính thức?", "Có. Flatkey hợp nhất routing, billing và governance mà không đổi model âm thầm."], ["Test model thế nào?", "Mỗi card mở trang model với prompt editor và đăng ký để chạy."], ["Giá đến từ đâu?", "Homepage lấy các dòng đại diện từ cùng thư mục giá."], ["Có duyệt theo loại model không?", "Homepage có lối vào cho video, văn bản, âm thanh và hình ảnh."]], items: [{ metric: "Ít key hơn", quote: "Không cần giữ key provider riêng trong từng agent nữa.", role: "Đội app AI" }, { metric: "Chi phí rõ", quote: "Video, văn bản và hình ảnh cùng dùng một số dư và ledger.", role: "Đội growth" }, { metric: "Migrate nhanh", quote: "Interface tương thích OpenAI giữ nguyên nên đổi model không cần viết lại sản phẩm.", role: "Đội platform" }, { metric: "Debug nhanh", quote: "Lỗi, model, provider và billing record nằm chung một nơi.", role: "Đội ops" }], kicker: "TIẾNG NÓI KHÁCH HÀNG", signals: ["Endpoint chính thức", "Số dư chung", "Ledger request", "Lỗi không tính phí", "Allowlist", "SLA"], sub: "Đội nhóm quan tâm key ít hơn, chi phí rõ hơn, migrate nhanh hơn và debug dễ hơn.", title: "Điều các đội nhắc lại." },
    final: { alt: "Biển quảng cáo Flatkey tại Bay Area", kicker: "BẰNG CHỨNG THƯƠNG HIỆU · BAY AREA", launch: [["01", "Tạo API Key", "Tạo key nhóm và tách policy theo môi trường."], ["02", "Chọn model", "Kiểm tra endpoint, giá và group khả dụng trong thư mục."], ["03", "Quan sát production", "Kiểm soát traffic bằng log, ngân sách và SLA."]], metrics: ["Kiểm soát đã audit", "Chương trình bảo mật", "Có SLA"], sub: "Truy cập model, giá, usage, SLA, hóa đơn và quyền trong một workflow production.", title: "Đưa AI đa phương thức vào production.", trustZeroRetention: "Không lưu nội dung" },
  },
  de: {
    oneGateway: "Ein Gateway", typeEntry: "Einstieg nach Typ",
    cli: { aria: "CLI-Schnellstart", checks: ["Ein API-Key", "Gemeinsames Guthaben und Ledger", "Nachverfolgbare Aufrufe"], docs: "Docs lesen", guide: "CLI-Guide öffnen", kicker: "CLI QUICKSTART", range: "von Installation bis erstem Aufruf", stepsDone: "3 Schritte", steps: [{ body: "Installiere die Flatkey CLI global für die Nutzung im Terminal.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "CLI installieren" }, { body: "Kopiere einen API-Key aus der Konsole und halte Provider-Keys aus lokalen Agents heraus.", code: "flatkey login", no: "02", title: "Anmelden" }, { body: "Lass Codex, Claude Code oder Skripte Modelle mit demselben Ledger aufrufen.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Modelle nutzen" }], sub: "CLI installieren, mit einem Key anmelden und Codex oder Claude Code Modelle mit gemeinsamem Ledger aufrufen lassen.", title: "Verbinde deine CLI in drei Schritten." },
    whyCards: [{ body: "Modellnamen, Provider und Preise kommen aus demselben Live-Verzeichnis. Aufrufe gehen an den konfigurierten Endpoint ohne stille Modellwechsel.", chips: ["Offizielle Endpoints", "Verzeichnis-Sync", "Testbar"], metric: "01", title: "Wissen, welches Modell läuft" }, { body: "Fehler, Latenz, Provider und Routing-Entscheidungen bleiben am Request-Log.", chips: ["Failover", "Request-Logs", "Status"], metric: "02", title: "Vom Request aus debuggen" }, { body: "Ein Prepaid-Guthaben deckt alle Modelltypen ab und Usage wird zur Abstimmung erfasst.", chips: ["Gemeinsames Guthaben", "Keine Fehlerkosten", "Ledger"], metric: "03", title: "Kosten pro Request erklären" }, { body: "Sub-Keys tragen Budgets, Modell-Allowlists, Rechnungen, Security Reviews und Retention-Policy.", chips: ["Sub-Key", "Allowlist", "Rechnungen"], metric: "04", title: "Zugriff pro Team verwalten" }],
    voice: { aria: "Kundenstimmen-Karussell", boardCopy: "Flatkey bündelt Modellaufrufe, Budgets, Fehler und Rechnungen in einem Workflow, damit ROI schneller klar wird.", boardMetric: "Von Teams wiederholt", boardTitle: "Weniger Keys, weniger Vorfälle, klare Kosten", faqs: [["Sind das offizielle Endpoints?", "Ja. Flatkey vereinheitlicht Routing, Billing und Governance ohne stille Modellwechsel."], ["Wie testet man Modelle?", "Jede Karte öffnet die Modellseite mit Prompt-Bearbeitung und Registrierung zum Ausführen."], ["Woher kommen die Preise?", "Die Startseite nutzt repräsentative Zeilen aus demselben Preisverzeichnis."], ["Kann man nach Modelltyp browsen?", "Die Startseite bietet Einstiege für Video, Text, Audio und Bild."]], items: [{ metric: "Weniger Keys", quote: "Wir halten keine separaten Provider-Keys mehr in jedem Agent.", role: "AI-App-Team" }, { metric: "Klare Kosten", quote: "Video, Text und Bild laufen über ein Guthaben und ein Ledger.", role: "Growth-Team" }, { metric: "Schnellere Migration", quote: "Die OpenAI-kompatible Schnittstelle bleibt, daher schreibt Modellwechsel keinen Produktcode um.", role: "Platform-Team" }, { metric: "Schneller debuggen", quote: "Fehler, Modelle, Provider und Billing-Daten liegen zusammen.", role: "Ops-Team" }], kicker: "KUNDENSTIMMEN", signals: ["Offizielle Endpoints", "Gemeinsames Guthaben", "Request-Ledger", "Keine Fehlerkosten", "Allowlist", "SLA"], sub: "Teams wollen weniger Keys, klare Kosten, schnellere Migration und einfacheres Debugging.", title: "Was Teams immer wieder sagen." },
    final: { alt: "Flatkey Außenwerbung in der Bay Area", kicker: "MARKENBELEG · BAY AREA", launch: [["01", "API-Key erstellen", "Team-Keys erzeugen und Policies nach Umgebung trennen."], ["02", "Modelle wählen", "Endpoints, Preise und verfügbare Gruppen im Verzeichnis prüfen."], ["03", "Produktion beobachten", "Traffic mit Logs, Budgets und SLA-Abdeckung steuern."]], metrics: ["Auditierte Kontrolle", "Security-Programm", "SLA verfügbar"], sub: "Modellzugriff, Preise, Nutzung, SLA, Rechnungen und Rechte in einem Produktionsworkflow.", title: "Bring multimodale KI in Produktion.", trustZeroRetention: "Keine Speicherung" },
  },
  id: {
    oneGateway: "Satu gateway", typeEntry: "Masuk berdasarkan tipe",
    cli: { aria: "Langkah cepat CLI", checks: ["Satu API Key", "Saldo dan ledger bersama", "Panggilan dapat dilacak"], docs: "Baca docs", guide: "Buka panduan CLI", kicker: "CLI QUICKSTART", range: "dari instalasi ke panggilan pertama", stepsDone: "3 langkah", steps: [{ body: "Instal Flatkey CLI secara global agar bisa dipakai dari terminal.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "Instal CLI" }, { body: "Salin API Key dari console dan hindari key provider di agent lokal.", code: "flatkey login", no: "02", title: "Masuk" }, { body: "Biarkan Codex, Claude Code, atau script memanggil model dengan ledger yang sama.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Gunakan model" }], sub: "Instal CLI, masuk dengan satu key, lalu biarkan Codex atau Claude Code memanggil model dengan ledger bersama.", title: "Hubungkan CLI dalam tiga langkah." },
    whyCards: [{ body: "Nama model, provider, dan harga berasal dari direktori live yang sama. Panggilan menuju endpoint yang dikonfigurasi tanpa penggantian model diam-diam.", chips: ["Endpoint resmi", "Direktori sinkron", "Bisa dites"], metric: "01", title: "Tahu model yang dipanggil" }, { body: "Kegagalan, latensi, provider, dan keputusan routing tetap melekat pada log request.", chips: ["Failover", "Log request", "Status"], metric: "02", title: "Debug dari request" }, { body: "Satu saldo prabayar mencakup semua tipe model dan penggunaan dicatat untuk rekonsiliasi.", chips: ["Saldo bersama", "Gagal tanpa biaya", "Ledger"], metric: "03", title: "Jelaskan biaya per request" }, { body: "Sub-key membawa budget, allowlist model, invoice, review keamanan, dan kebijakan retensi.", chips: ["Sub-key", "Allowlist", "Invoice"], metric: "04", title: "Kelola akses per tim" }],
    voice: { aria: "Carousel suara pelanggan", boardCopy: "Flatkey menyatukan panggilan model, budget, kegagalan, dan invoice agar ROI lebih cepat dinilai.", boardMetric: "Sering disebut tim", boardTitle: "Key lebih sedikit, insiden lebih sedikit, biaya lebih jelas", faqs: [["Apakah ini endpoint resmi?", "Ya. Flatkey menyatukan routing, billing, dan governance tanpa penggantian model diam-diam."], ["Bagaimana cara mengetes model?", "Setiap card membuka halaman model dengan editor prompt dan pendaftaran untuk menjalankan."], ["Harga berasal dari mana?", "Homepage mengambil baris representatif dari direktori harga yang sama."], ["Bisa jelajah berdasarkan tipe model?", "Homepage menyediakan pintu masuk video, teks, audio, dan gambar."]], items: [{ metric: "Key lebih sedikit", quote: "Kami tidak lagi menyimpan key provider terpisah di setiap agent.", role: "Tim aplikasi AI" }, { metric: "Biaya jelas", quote: "Video, teks, dan gambar memakai saldo dan ledger yang sama.", role: "Tim growth" }, { metric: "Migrasi cepat", quote: "Interface kompatibel OpenAI tetap dipakai, jadi ganti model tidak menulis ulang kode produk.", role: "Tim platform" }, { metric: "Debug cepat", quote: "Kegagalan, model, provider, dan catatan billing berada bersama.", role: "Tim ops" }], kicker: "SUARA PELANGGAN", signals: ["Endpoint resmi", "Saldo bersama", "Ledger request", "Gagal tanpa biaya", "Allowlist", "SLA"], sub: "Tim peduli key lebih sedikit, biaya lebih jelas, migrasi lebih cepat, dan debug lebih mudah.", title: "Yang sering diulang tim." },
    final: { alt: "Billboard Flatkey di Bay Area", kicker: "BUKTI BRAND · BAY AREA", launch: [["01", "Buat API Key", "Buat key tim dan pisahkan policy berdasarkan environment."], ["02", "Pilih model", "Cek endpoint, harga, dan grup tersedia di direktori."], ["03", "Pantau production", "Kontrol traffic dengan log, budget, dan cakupan SLA."]], metrics: ["Kontrol teraudit", "Program keamanan", "SLA tersedia"], sub: "Akses model, harga, penggunaan, SLA, invoice, dan izin dalam satu workflow produksi.", title: "Bawa AI multimodal ke produksi.", trustZeroRetention: "Retensi nol" },
  },
};

function buildTranslatedHomeCopy(locale: HomeTranslatedLocale): HomePageCopy {
  const d = HOME_TRANSLATION_BASE[locale];
  const section = HOME_TRANSLATED_SECTIONS[locale];
  return {
    carousel: { aria: d.carousel, eyebrow: `Unified model gateway · ${d.endpoint} · ${d.billing}`, switchAria: d.switchType },
    ctaConsole: locale === "ja" ? "コンソールを開く" : locale === "ru" ? "Открыть консоль" : locale === "de" ? "Konsole öffnen" : locale === "fr" ? "Ouvrir la console" : locale === "es" ? "Abrir consola" : locale === "pt" ? "Abrir consola" : locale === "vi" ? "Mở console" : "Buka console",
    contactSales: d.contact,
    fallbackDirectory: d.directory,
    hero: {
      all: { copy: d.modelFlowSub, cta: "", fallbackCta: d.browseAll, fallbackMetric: d.endpoint, fallbackModelName: d.allDir, fallbackVendor: d.all, kicker: d.all, mode: d.all, subline: section.oneGateway, title: d.all },
      audio: { copy: d.audioCopy, cta: "", fallbackCta: d.openAudio, fallbackMetric: d.endpoint, fallbackModelName: d.audioModels, fallbackVendor: d.audio, kicker: d.audioModels, mode: d.audio, subline: d.audioSubline, title: d.audioModels },
      image: { copy: d.imageCopy, cta: "", fallbackCta: d.openImage, fallbackMetric: d.endpoint, fallbackModelName: d.imageModels, fallbackVendor: d.image, kicker: d.imageModels, mode: d.image, subline: d.imageSubline, title: d.imageModels },
      text: { copy: d.textCopy, cta: "", fallbackCta: d.openText, fallbackMetric: d.endpoint, fallbackModelName: d.textModels, fallbackVendor: d.text, kicker: d.textModels, mode: d.text, subline: d.textSubline, title: d.textModels },
      video: { copy: d.videoCopy, cta: "", fallbackCta: d.openVideo, fallbackMetric: d.endpoint, fallbackModelName: d.videoModels, fallbackVendor: d.video, kicker: d.videoModels, mode: d.video, subline: d.videoSubline, title: d.videoModels },
      useModel: d.useModel,
    },
    logo: { aria: d.models, connected: d.endpoint },
    modelFlow: {
      directoryFallback: d.directory,
      kicker: d.modelFlowKicker,
      proof: [d.directory, section.typeEntry, d.balance, d.traceable],
      sub: d.modelFlowSub,
      title: d.modelFlowTitle,
      types: {
        all: { api: "All", copy: d.modelFlowSub, cta: d.browseAll, title: d.allDir },
        audio: { api: "Audio", copy: d.audioCopy, cta: d.openAudio, title: d.audioModels },
        image: { api: "Image", copy: d.imageCopy, cta: d.openImage, title: d.imageModels },
        text: { api: "Text", copy: d.textCopy, cta: d.openText, title: d.textModels },
        video: { api: "Video", copy: d.videoCopy, cta: d.openVideo, title: d.videoModels },
      },
    },
    price: { aria: d.priceKicker, cardPolicy: `${d.endpoint} · ${d.balance} · ${d.failure}`, flatkeyFailure: d.failure, flatkeyFailureShort: d.failure, kicker: d.priceKicker, officialEndpoint: d.endpoint, officialLabel: d.official, perMillionInput: " / 1M input", perRequest: d.request, requestLedger: d.ledger, sharedBalance: d.sharedBalance, sub: d.priceSub, test: d.test, title: d.priceTitle, viewAll: d.viewAllPrices },
    cli: section.cli,
    why: {
      cards: section.whyCards,
      kicker: d.whyKicker,
      title: d.whyTitle,
    },
    voice: section.voice,
    final: section.final,
  };
}

const HOME_PAGE_COPY: Record<Locale, HomePageCopy> = withIdFallback({
  en: {
    carousel: { aria: "Multimodal model carousel", eyebrow: "Unified model gateway · official endpoints · observable billing", switchAria: "Switch carousel type" },
    ctaConsole: "Open console",
    contactSales: "Contact sales",
    fallbackDirectory: "Browse directory",
    hero: {
      all: { copy: "One key for real supported models, with routing, budgets, billing, and request logs in one place.", cta: "", fallbackCta: "View models", fallbackMetric: "Official endpoints", fallbackModelName: "Model directory", fallbackVendor: "All models", kicker: "All models", mode: "All models", subline: "One gateway", title: "Multimodal models" },
      audio: { copy: "Speech recognition, synthesis, audio understanding, and transcription under one budget policy.", cta: "", fallbackCta: "View audio models", fallbackMetric: "Official endpoint", fallbackModelName: "Audio model", fallbackVendor: "Audio", kicker: "Audio models", mode: "Audio", subline: "Speech and understanding", title: "Audio model" },
      image: { copy: "Posters, product visuals, illustrations, and bulk creative generation.", cta: "", fallbackCta: "View image models", fallbackMetric: "Official endpoint", fallbackModelName: "Image model", fallbackVendor: "Image generation", kicker: "Image generation", mode: "Image", subline: "High-quality visuals", title: "Image model" },
      text: { copy: "Chat, reasoning, coding agents, and long-context work routed from the same gateway.", cta: "", fallbackCta: "View text models", fallbackMetric: "Official endpoint", fallbackModelName: "Text model", fallbackVendor: "Text and reasoning", kicker: "Text and reasoning", mode: "Text", subline: "Reliable routing", title: "Text model" },
      video: { copy: "Text-to-video, image-to-video, ad clips, and motion assets through one API.", cta: "", fallbackCta: "View video models", fallbackMetric: "Official endpoint", fallbackModelName: "Video model", fallbackVendor: "Video generation", kicker: "Video generation", mode: "Video", subline: "Motion production", title: "Video model" },
      useModel: (modelName) => `Use ${modelName}`,
    },
    logo: { aria: "Supported models", connected: "official models" },
    modelFlow: {
      directoryFallback: "Browse directory",
      kicker: "MULTIMODAL MODEL FLOW",
      proof: ["Live model directory", "Type-based entry", "Shared balance", "Traceable requests"],
      sub: "Choose each model type separately while sharing one key, balance, permission policy, and request ledger.",
      title: "Connect video, image, text, and audio into one product flow.",
      types: {
        all: { api: "All", copy: "Browse the live directory from one entry. Video, image, text, and audio share one key, balance, policy, and request ledger.", cta: "Browse all models", title: "All model directory" },
        audio: { api: "Audio", copy: "Speech recognition, synthesis, audio understanding, and transcription under one ledger.", cta: "Open audio models", title: "Speech and audio" },
        image: { api: "Image", copy: "Posters, product visuals, illustrations, and bulk creative generation.", cta: "Open image models", title: "Image generation" },
        text: { api: "Text", copy: "Chat, reasoning, coding agents, and long-context work routed by team policy.", cta: "Open text models", title: "Text and reasoning" },
        video: { api: "Video", copy: "Short clips, ads, and image-to-video tasks with one queue, callback, and billing flow.", cta: "Open video models", title: "Video generation" },
      },
    },
    price: { aria: "Model price comparison marquee", cardPolicy: "Official endpoint · shared balance · no failure charge", flatkeyFailure: "$0 for Flatkey-side failures", flatkeyFailureShort: "No failure charge", kicker: "MODEL PRICE SNAPSHOT", officialEndpoint: "official endpoint", officialLabel: "Official", perMillionInput: " / 1M input", perRequest: " / request", requestLedger: "Auditable per request", sharedBalance: "Shared prepaid balance", sub: "Representative models from the live directory, with official reference prices and Flatkey prices. Click through to test each model.", test: "Test", title: "See real model cost first.", viewAll: "View all model prices" },
    cli: {
      aria: "CLI quickstart steps",
      checks: ["One API key", "Shared balance and ledger", "Traceable model calls"],
      docs: "Read docs",
      guide: "Open CLI guide",
      kicker: "CLI QUICKSTART",
      range: "install to first run",
      stepsDone: "3 steps",
      steps: [
        { body: "Install the Flatkey CLI globally so developers can use it from their terminal.", code: "npm i -g @flatkey-ai/cli", no: "01", title: "Install CLI" },
        { body: "Copy one API key from the console and keep provider keys out of local agents.", code: "flatkey login", no: "02", title: "Sign in" },
        { body: "Let Codex, Claude Code, or scripts call text, image, video, and audio models with one ledger.", code: "codex \"Generate assets with Flatkey\"", no: "03", title: "Use models" },
      ],
      sub: "Install the CLI, sign in with one key, then let Codex or Claude Code call models with a shared ledger.",
      title: "Connect your CLI in three steps.",
    },
    why: {
      cards: [
        { body: "Model names, vendors, and prices come from the same live directory. Calls route to the configured endpoint without silent model swaps.", chips: ["Official endpoints", "Directory sync", "Testable"], metric: "01", title: "Know which model is called" },
        { body: "A model can use multiple upstreams. Failures, latency, provider, and route decisions stay attached to the request log.", chips: ["Failover", "Request logs", "Status"], metric: "02", title: "Debug from the request" },
        { body: "One prepaid balance covers model types. Successful calls, failures, input, output, and cached usage are recorded for reconciliation.", chips: ["Shared balance", "No failure charge", "Ledger"], metric: "03", title: "Explain cost per request" },
        { body: "Sub-keys carry budgets, model allowlists, and environment policies. Invoices, security reviews, and retention settings live in one workflow.", chips: ["Sub-key", "Allowlist", "Invoices"], metric: "04", title: "Manage access by team" },
      ],
      kicker: "WHY FLATKEY",
      title: "Teams need a controllable model gateway.",
    },
    voice: {
      aria: "Customer voice marquee",
      boardCopy: "Flatkey brings model calls, budgets, failures, and invoices into one workflow so teams can judge ROI faster.",
      boardMetric: "Repeated by teams",
      boardTitle: "Fewer keys, fewer incidents, clearer spend",
      faqs: [["Are these official endpoints?", "Yes. Flatkey unifies routing, billing, and governance without silent model swaps."], ["How does model testing work?", "Each card opens the model landing page with prompt editing and sign-up to run."], ["Where does pricing come from?", "The homepage samples representative rows from the same pricing directory."], ["Can users browse by model type?", "The homepage exposes video, text, audio, and image entry points."]],
      items: [
        { metric: "Fewer keys", quote: "We no longer keep separate provider keys inside every agent. Budgets are visible in one place.", role: "AI app team" },
        { metric: "Clear spend", quote: "Video, text, and image calls all run through one balance and ledger.", role: "Growth team" },
        { metric: "Faster migration", quote: "The OpenAI-compatible interface stays, so model switching does not rewrite product code.", role: "Platform team" },
        { metric: "Faster debugging", quote: "Failures, model names, vendors, and billing records live together.", role: "Ops team" },
      ],
      kicker: "CUSTOMER VOICE",
      signals: ["Official endpoints", "Shared balance", "Request ledger", "No failure charge", "Allowlist", "SLA"],
      sub: "Teams care less about model count and more about fewer keys, clearer spend, faster migration, and easier debugging.",
      title: "What teams keep repeating.",
    },
    final: { alt: "Flatkey Bay Area billboard", kicker: "BRAND PROOF · BAY AREA", launch: [["01", "Create an API key", "Generate team keys and split policy by environment."], ["02", "Pick models", "Check endpoints, pricing, and available groups in the directory."], ["03", "Observe production", "Control traffic with logs, budgets, and SLA coverage."]], metrics: ["Audited control", "Security program", "SLA available"], sub: "Model access, pricing, usage, SLA, invoices, and permissions in one production workflow.", title: "Bring multimodal AI into production.", trustZeroRetention: "Zero retention" },
  },
  zh: {
    carousel: { aria: "多模态模型轮播", eyebrow: "统一模型网关 · 官方端点 · 可观测账单", switchAria: "切换轮播类型" },
    ctaConsole: "进入控制台",
    contactSales: "联系销售",
    fallbackDirectory: "查看模型目录",
    hero: {
      all: { copy: "一个 Key 管理真实可用模型，路由、账单、预算和调用记录都在一处。", cta: "", fallbackCta: "进入模型目录", fallbackMetric: "官方端点", fallbackModelName: "模型目录", fallbackVendor: "全部模型", kicker: "全部模型", mode: "全部模型", subline: "一个 Key 接入", title: "多模态模型" },
      audio: { copy: "语音识别、合成、理解和转写，和其他模型共用余额、权限和日志。", cta: "", fallbackCta: "查看音频模型", fallbackMetric: "官方端点", fallbackModelName: "音频模型", fallbackVendor: "音频能力", kicker: "音频模型", mode: "音频", subline: "语音音频", title: "音频模型" },
      image: { copy: "海报、商品图、插画和批量素材生成，直接进入模型页测试。", cta: "", fallbackCta: "查看图像模型", fallbackMetric: "官方端点", fallbackModelName: "图像模型", fallbackVendor: "图像生成", kicker: "图像生成", mode: "图像", subline: "图片生成", title: "图像模型" },
      text: { copy: "对话、推理、代码 Agent 和长上下文任务，按团队策略稳定路由。", cta: "", fallbackCta: "查看文本模型", fallbackMetric: "官方端点", fallbackModelName: "文本模型", fallbackVendor: "文本与推理", kicker: "文本与推理", mode: "文本", subline: "文本推理", title: "文本模型" },
      video: { copy: "文生视频、图生视频、短片和动态广告素材，统一排队、回调和计费。", cta: "", fallbackCta: "查看视频模型", fallbackMetric: "官方端点", fallbackModelName: "视频模型", fallbackVendor: "视频生成", kicker: "视频生成", mode: "视频", subline: "视频生成", title: "视频模型" },
      useModel: (modelName) => `使用 ${modelName}`,
    },
    logo: { aria: "已接入模型", connected: "已接入" },
    modelFlow: {
      directoryFallback: "查看模型目录",
      kicker: "多模态模型链路",
      proof: ["真实模型目录", "按类型进入", "共用余额", "请求可追踪"],
      sub: "四类模型分开选，也能共用同一个 Key、余额、权限和请求记录。模型入口来自当前模型目录，不做虚构能力。",
      title: "把视频、图像、文本、音频接成一条业务流",
      types: {
        all: { api: "All", copy: "从一个入口浏览当前可用模型。视频、图像、文本、音频共用同一个 Key、余额、权限和调用账本。", cta: "查看全部模型", title: "全部模型目录" },
        audio: { api: "Audio", copy: "语音识别、合成、理解和转写，和其他模型共用余额与日志。", cta: "进入音频模型", title: "语音音频" },
        image: { api: "Image", copy: "海报、商品图、插画和批量素材生成，直接进入模型页测试。", cta: "进入图像模型", title: "图像生成" },
        text: { api: "Text", copy: "对话、推理、代码 Agent 和长上下文任务，按团队策略稳定路由。", cta: "进入文本模型", title: "文本与推理" },
        video: { api: "Video", copy: "短片、广告素材和图生视频任务，队列、回调和计费统一处理。", cta: "进入视频模型", title: "视频生成" },
      },
    },
    price: { aria: "模型价格横向滚动对比", cardPolicy: "官方端点 · 统一余额 · 失败不扣费", flatkeyFailure: "失败不扣费", flatkeyFailureShort: "失败不扣费", kicker: "模型价格对比", officialEndpoint: "官方端点", officialLabel: "官方价", perMillionInput: " / 1M 输入", perRequest: " / 请求", requestLedger: "请求级账本", sharedBalance: "统一预付余额", sub: "从模型目录抽取真实可用的典型模型，展示官方参考价与 Flatkey 价格。点击卡片进入对应模型页测试。", test: "测试", title: "先看模型真实成本", viewAll: "查看全部模型价格" },
    cli: {
      aria: "CLI 快速开始步骤",
      checks: ["同一个 API Key", "统一余额和请求账本", "模型调用可追踪"],
      docs: "接入文档",
      guide: "查看 CLI 指南",
      kicker: "CLI 接入",
      range: "从安装到调用",
      stepsDone: "3 步完成",
      steps: [
        { body: "全局安装 Flatkey CLI，团队成员在自己的终端里直接运行。", code: "npm i -g @flatkey-ai/cli", no: "01", title: "安装 CLI" },
        { body: "从控制台复制 API Key，登录后写入本地配置，不需要到处分发上游 Key。", code: "flatkey login", no: "02", title: "登录 API Key" },
        { body: "让 Codex、Claude Code 或你的脚本调用模型，图片、视频、文本、音频共用余额和账本。", code: "codex \"用 Flatkey 生成这组素材\"", no: "03", title: "开始调用模型" },
      ],
      sub: "安装 CLI、登录 API Key，再把需求交给 Codex 或 Claude Code。模型调用、失败记录和用量都会进入同一套账本。",
      title: "三步把 CLI 接到 Flatkey",
    },
    why: {
      cards: [
        { body: "模型页展示的名称、供应商和价格来自同一份目录。接入后按端点调用，不在业务侧悄悄替换模型。", chips: ["官方端点", "目录同步", "可测试"], metric: "01", title: "先确认调用的是什么模型" },
        { body: "同一模型可配置多条上游，异常时按策略切换。状态、错误和耗时记录在同一个请求日志里。", chips: ["故障切换", "请求日志", "状态观测"], metric: "02", title: "出问题时能定位到请求" },
        { body: "预付余额覆盖不同模型类型。成功、失败、输入、输出和缓存用量都会进入账本，方便对账。", chips: ["统一余额", "失败不扣费", "用量账本"], metric: "03", title: "成本按请求说清楚" },
        { body: "团队可以按子 Key 设置预算、模型白名单和环境权限。发票、安全认证和零留存策略放在同一套流程里。", chips: ["子 Key", "模型白名单", "发票"], metric: "04", title: "权限按团队管理" },
      ],
      kicker: "为什么选择 Flatkey",
      title: "团队真正需要的是可控的模型网关",
    },
    voice: {
      aria: "客户反馈滚动展示",
      boardCopy: "Flatkey 把模型调用、预算、失败记录和发票归到同一套工作流，让研发和业务都更容易判断投入产出。",
      boardMetric: "客户反复提到",
      boardTitle: "少配置、少排障、少解释成本",
      faqs: [["模型来源可靠吗？", "可靠。Flatkey 连接官方模型端点，统一做鉴权、路由、账单和治理，不做静默换模。"], ["没有账号能先试吗？", "可以先进入模型页查看示例和提示词；真正生成时会引导注册或进入控制台。"], ["价格为什么放在首页？", "客户最先关心能不能用、稳不稳定、贵不贵。首页展示典型模型，完整价格在模型目录。"], ["多模态是怎么接入的？", "视频、图像、文本、音频共用一个 API Key、余额体系、权限策略和请求日志。"]],
      items: [
        { metric: "Key 变少", quote: "不用再给每个 Agent 单独塞一堆模型 Key，权限和预算终于能统一看。", role: "AI 应用团队" },
        { metric: "成本清楚", quote: "视频、文本、图片都走同一套余额和账本，给业务方解释成本简单很多。", role: "增长团队" },
        { metric: "迁移更快", quote: "OpenAI 兼容接口保留，模型切换不用重写业务侧调用。", role: "平台工程团队" },
        { metric: "排障更快", quote: "失败请求、模型、供应商和账单记录都在一起，定位问题不用翻多个后台。", role: "运维团队" },
      ],
      kicker: "客户声音",
      signals: ["官方端点", "统一余额", "请求账本", "失败不扣费", "模型白名单", "SLA"],
      sub: "他们关心的不是“模型很多”，而是 Key 更少、账更清楚、迁移更快、排障更简单。",
      title: "被反复提到的产品价值",
    },
    final: { alt: "Flatkey 湾区户外广告", kicker: "开始接入", launch: [["01", "创建 API Key", "从控制台生成团队 Key，按环境拆分权限。"], ["02", "选择模型", "在模型目录确认端点、价格和可用分组。"], ["03", "上线观测", "用请求日志、预算和 SLA 管住生产流量。"]], metrics: ["审计背书", "安全体系", "可签 SLA"], sub: "模型、价格、用量、SLA、发票和权限统一管理。团队可以更快上线，也能把成本和风险看清楚。", title: "把多模态模型接入你的产品", trustZeroRetention: "请求内容零留存" },
  },
  es: buildTranslatedHomeCopy("es"),
  fr: buildTranslatedHomeCopy("fr"),
  pt: buildTranslatedHomeCopy("pt"),
  ru: buildTranslatedHomeCopy("ru"),
  ja: buildTranslatedHomeCopy("ja"),
  vi: buildTranslatedHomeCopy("vi"),
  de: buildTranslatedHomeCopy("de"),
  id: buildTranslatedHomeCopy("id"),
});

const REPRESENTATIVE_PRICE_MATCHERS = [
  /^(gpt-|o\d|chatgpt|codex)/i,
  /^claude/i,
  /^(gemini|veo|imagen)/i,
  /^deepseek/i,
  /^qwen/i,
  /^(glm|chatglm)/i,
  /(gpt-image|image|imagen|dall)/i,
  /(seedance|veo|kling|sora)/i,
];

function formatHomePriceLabel(price: string, copy: HomePageCopy) {
  if (price.includes("/req")) return price.replace(" /req", copy.price.perRequest);
  return `${price}${copy.price.perMillionInput}`;
}

function buildRepresentativePriceComparisons(data: PricingData | undefined, copy: HomePageCopy): HomePriceComparison[] {
  if (!data || data.models.length === 0) return [];
  const rows = buildRowsForModels(sortPricingModelsBySeries(data.models), data.vendors, data.groupRatio);
  const selected: typeof rows = [];
  const seen = new Set<string>();
  const pick = (row: (typeof rows)[number]) => {
    const key = row.name.toLowerCase();
    if (seen.has(key)) return;
    seen.add(key);
    selected.push(row);
  };

  for (const matcher of REPRESENTATIVE_PRICE_MATCHERS) {
    const match = rows.find((row) => matcher.test(`${row.name} ${row.vendor}`));
    if (match) pick(match);
  }
  for (const row of rows) {
    if (selected.length >= 12) break;
    pick(row);
  }

  return selected.slice(0, 12).map((row, index) => ({
    flatkey: formatHomePriceLabel(row.discounted, copy),
    href: modelPublicPath(row.name),
    image: PRICE_POSTERS[index % PRICE_POSTERS.length],
    model: row.name,
    official: formatHomePriceLabel(row.official, copy),
    policy: copy.price.cardPolicy,
    tag: copy.price.test,
    vendor: row.vendor,
  }));
}

function homeModelSearchText(model: PricingData["models"][number]) {
  return [
    model.model_name,
    model.vendor_name,
    model.description,
    model.tags,
    ...(model.supported_endpoint_types ?? []),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

function modelMatchesHomeKind(model: PricingData["models"][number], kind: Exclude<HomeModelKind, "all">): boolean {
  const value = homeModelSearchText(model);
  const endpoints = (model.supported_endpoint_types ?? []).map((endpoint) => endpoint.toLowerCase());
  if (kind === "video") return /video|seedance|sora|veo|kling|wan|hailuo/.test(value) || endpoints.includes("openai-video");
  if (kind === "image") return model.image_ratio != null || endpoints.includes("image-generation") || /image|imagen|dall|gpt-image|banana/.test(value);
  if (kind === "audio") return model.audio_ratio != null || /audio|whisper|tts|voice|speech|suno/.test(value);
  return !modelMatchesHomeKind(model, "video") && !modelMatchesHomeKind(model, "image") && !modelMatchesHomeKind(model, "audio");
}

function pickHomeModelTags(data: PricingData | undefined, kind: HomeModelKind, limit = 3): string[] {
  if (!data || data.models.length === 0) return [];
  const models = sortPricingModelsBySeries(data.models);
  const selected: string[] = [];
  const seen = new Set<string>();
  const push = (model: PricingData["models"][number] | undefined) => {
    if (!model) return;
    const name = model.model_name;
    const key = name.toLowerCase();
    if (seen.has(key)) return;
    seen.add(key);
    selected.push(name);
  };

  if (kind === "all") {
    (["text", "image", "video", "audio"] as const).forEach((item) => {
      push(models.find((model) => modelMatchesHomeKind(model, item)));
    });
    if (selected.length < limit) {
      for (const model of models) {
        push(model);
        if (selected.length >= limit) break;
      }
    }
    return selected.slice(0, limit);
  }

  const candidates = models.filter((model) => modelMatchesHomeKind(model, kind));
  const preferred: Record<Exclude<HomeModelKind, "all">, RegExp[]> = {
    audio: [/whisper/i, /tts|voice|speech/i, /audio/i],
    image: [/gpt-image/i, /imagen/i, /qwen.*image|image.*qwen/i, /banana/i, /dall/i],
    text: [/^gpt|^o\d/i, /^claude/i, /^gemini/i, /^deepseek/i, /^qwen/i, /^glm|chatglm/i],
    video: [/seedance/i, /veo/i, /sora/i, /kling/i, /video/i],
  };

  for (const pattern of preferred[kind]) {
    push(candidates.find((model) => pattern.test(model.model_name)));
  }
  for (const model of candidates) {
    push(model);
    if (selected.length >= limit) break;
  }
  return selected.slice(0, limit);
}

function buildHomeModelTags(data: PricingData | undefined): HomeModelTags {
  return {
    all: pickHomeModelTags(data, "all"),
    audio: pickHomeModelTags(data, "audio"),
    image: pickHomeModelTags(data, "image"),
    text: pickHomeModelTags(data, "text"),
    video: pickHomeModelTags(data, "video"),
  };
}

function modelTypeLabel(tags: string[], fallback: string) {
  return tags.length > 0 ? tags.slice(0, 2).join(" · ") : fallback;
}

type HomeHeroModels = Record<HomeModelKind, PricingModel | undefined>;

const HERO_MODEL_PREFERENCES: Record<HomeModelKind, RegExp[]> = {
  all: [/^gpt-5(?:$|-)/i, /^claude.*sonnet/i, /^gemini.*pro/i, /^deepseek/i],
  audio: [/whisper/i, /gpt.*audio/i, /tts|voice|speech/i, /audio/i],
  image: [/^gpt-image-2/i, /imagen/i, /qwen.*image|image.*qwen/i, /banana/i, /dall/i],
  text: [/^claude.*sonnet/i, /^gemini.*pro/i, /^deepseek/i, /^qwen/i, /^gpt/i],
  video: [/seedance.*2/i, /seedance/i, /veo/i, /sora/i, /kling/i, /video/i],
};

function pickHeroModel(models: PricingModel[], kind: HomeModelKind, used: Set<string>) {
  const candidates = kind === "all" ? models : models.filter((model) => modelMatchesHomeKind(model, kind));
  const available = candidates.filter((model) => !used.has(model.model_name.toLowerCase()));
  const pool = available.length > 0 ? available : candidates;
  const preferred = HERO_MODEL_PREFERENCES[kind]
    .map((pattern) => pool.find((model) => pattern.test(model.model_name)))
    .find(Boolean);
  const selected = preferred ?? pool[0];
  if (selected) used.add(selected.model_name.toLowerCase());
  return selected;
}

function buildHomeHeroModels(data: PricingData | undefined): HomeHeroModels {
  const result: HomeHeroModels = { all: undefined, audio: undefined, image: undefined, text: undefined, video: undefined };
  if (!data || data.models.length === 0) return result;

  const models = sortPricingModelsBySeries(data.models);
  const used = new Set<string>();
  result.all = pickHeroModel(models, "all", used);
  result.image = pickHeroModel(models, "image", used);
  result.video = pickHeroModel(models, "video", used);
  result.text = pickHeroModel(models, "text", used);
  result.audio = pickHeroModel(models, "audio", used);
  return result;
}

function heroModelName(model: PricingModel | undefined, fallback: string) {
  return model?.model_name ?? fallback;
}

function heroModelVendor(model: PricingModel | undefined, fallback: string) {
  return model?.vendor_name ?? fallback;
}

function heroModelHref(model: PricingModel | undefined, fallback: string) {
  return model ? modelPublicPath(model.model_name) : fallback;
}

function heroModelCta(copy: HomePageCopy, model: PricingModel | undefined, fallback: string) {
  if (!model) return fallback;
  return copy.hero.useModel(model.model_name);
}

function getHeroModes(copy: HomePageCopy, heroModels: HomeHeroModels): HeroMode[] {
  const allModel = heroModels.all;
  const imageModel = heroModels.image;
  const videoModel = heroModels.video;
  const textModel = heroModels.text;
  const audioModel = heroModels.audio;
  const c = copy.hero;

  return [
    { ...c.all, cta: heroModelCta(copy, allModel, c.all.fallbackCta), href: heroModelHref(allModel, "/models"), image: HERO_ART.all.wide, kind: "all", metric: heroModelVendor(allModel, c.all.fallbackMetric), modelName: heroModelName(allModel, c.all.fallbackModelName), modelVendor: heroModelVendor(allModel, c.all.fallbackVendor), thumb: HERO_ART.all.wide },
    { ...c.image, cta: heroModelCta(copy, imageModel, c.image.fallbackCta), href: heroModelHref(imageModel, "/models?type=image"), image: HERO_ART.image.wide, kind: "image", metric: heroModelVendor(imageModel, c.image.fallbackMetric), modelName: heroModelName(imageModel, c.image.fallbackModelName), modelVendor: heroModelVendor(imageModel, c.image.fallbackVendor), thumb: HERO_ART.image.wide, title: heroModelName(imageModel, c.image.title) },
    { ...c.video, cta: heroModelCta(copy, videoModel, c.video.fallbackCta), href: heroModelHref(videoModel, "/models/seedance-api"), image: HERO_ART.video.wide, kind: "video", metric: heroModelVendor(videoModel, c.video.fallbackMetric), modelName: heroModelName(videoModel, c.video.fallbackModelName), modelVendor: heroModelVendor(videoModel, c.video.fallbackVendor), thumb: HERO_ART.video.wide, title: heroModelName(videoModel, c.video.title) },
    { ...c.text, cta: heroModelCta(copy, textModel, c.text.fallbackCta), href: heroModelHref(textModel, "/models"), image: HERO_ART.text.wide, kind: "text", metric: heroModelVendor(textModel, c.text.fallbackMetric), modelName: heroModelName(textModel, c.text.fallbackModelName), modelVendor: heroModelVendor(textModel, c.text.fallbackVendor), thumb: HERO_ART.text.wide, title: heroModelName(textModel, c.text.title) },
    { ...c.audio, cta: heroModelCta(copy, audioModel, c.audio.fallbackCta), href: heroModelHref(audioModel, "/models?type=audio"), image: HERO_ART.audio.wide, kind: "audio", metric: heroModelVendor(audioModel, c.audio.fallbackMetric), modelName: heroModelName(audioModel, c.audio.fallbackModelName), modelVendor: heroModelVendor(audioModel, c.audio.fallbackVendor), thumb: HERO_ART.audio.wide, title: heroModelName(audioModel, c.audio.title) },
  ];
}

function getModelTypes(copy: HomePageCopy, modelTags: HomeModelTags) {
  const typeCopy = copy.modelFlow.types;
  return [
    { ...typeCopy.all, href: "/models", image: HERO_ART.all.card, models: modelTags.all.slice(0, 4), tone: "all" },
    { ...typeCopy.video, href: "/models/seedance-api", image: HERO_ART.video.card, models: modelTags.video.slice(0, 3), tone: "video" },
    { ...typeCopy.image, href: "/models?type=image", image: HERO_ART.image.card, models: modelTags.image.slice(0, 3), tone: "image" },
    { ...typeCopy.text, href: "/models", image: HERO_ART.text.card, models: modelTags.text.slice(0, 3), tone: "text" },
    { ...typeCopy.audio, href: "/models?type=audio", image: HERO_ART.audio.card, models: modelTags.audio.slice(0, 3), tone: "audio" },
  ];
}

export function OnlineHomePage(props: { locale: Locale; pricingData?: PricingData }) {
  const copy = HOME_PAGE_COPY[props.locale];
  const modelTags = buildHomeModelTags(props.pricingData);
  const heroModels = buildHomeHeroModels(props.pricingData);
  const heroModes = getHeroModes(copy, heroModels);
  const modelTypes = getModelTypes(copy, modelTags);
  const priceComparisons = buildRepresentativePriceComparisons(props.pricingData, copy);
  const prices = priceComparisons;
  const priceRows = [prices.slice(0, 6), prices.slice(6, 12)].filter((row) => row.length > 0);
  const whyCards = copy.why.cards;
  const faqs = copy.voice.faqs;
  const cliSteps = copy.cli.steps;
  const voiceItems = copy.voice.items;

  return (
    <OnlineStaticShell locale={props.locale} pathname="/">
      <style>{`
        body:has(> header.hero.heroUnified){--fk-bg:#f6efe2;--fk-bg-2:#efe3cf;--fk-paper:#fffaf0;--fk-paper-2:#f8eddc;--fk-ink:#172017;--fk-muted:#647064;--fk-line:rgba(58,72,57,.16);--fk-gold:#d59a35;--fk-copper:#bd6b2b;--fk-sage:#647b55;--fk-night:#1a1711;background:var(--fk-bg);color:var(--fk-ink)}
        html.fk-theme-night body:has(> header.hero.heroUnified){--fk-bg:#12110f;--fk-bg-2:#1b1914;--fk-paper:#211f19;--fk-paper-2:#29261f;--fk-ink:#f4ead8;--fk-muted:#b9ad98;--fk-line:rgba(244,234,216,.15);--fk-gold:#f0c36c;--fk-copper:#e08c50;--fk-sage:#9eb58e;--fk-night:#0b0b09}
        body:has(> header.hero.heroUnified){display:flex;flex-direction:column}.nav{order:0}header.heroUnified{order:1}.modelLogoMarquee{order:2}.modelTypes{order:3}.priceProof{order:4}.cliQuick{order:5}.why{order:6}.voiceFaq{order:7}.ctaWrap{order:8}.megafoot{order:9}.stripe{order:10}
        .heroUnified{position:relative;min-height:min(820px,100svh);overflow:hidden;color:var(--fk-ink);border-bottom:1px solid var(--fk-line);background:radial-gradient(ellipse 70% 52% at 82% 18%,rgba(213,154,53,.24),transparent 62%),radial-gradient(ellipse 58% 48% at 10% 90%,rgba(142,176,138,.28),transparent 62%),linear-gradient(145deg,var(--fk-bg) 0%,#fff8e8 48%,var(--fk-bg-2) 100%)}
        html.fk-theme-night .heroUnified{background:radial-gradient(ellipse 72% 52% at 82% 18%,rgba(240,195,108,.18),transparent 62%),radial-gradient(ellipse 58% 48% at 10% 90%,rgba(158,181,142,.16),transparent 62%),linear-gradient(145deg,#11100d 0%,#1b1812 52%,#0f1712 100%)}
        .heroUnified:before{content:"";position:absolute;inset:0;background-image:linear-gradient(to right,rgba(73,82,58,.06) 1px,transparent 1px),linear-gradient(to bottom,rgba(73,82,58,.045) 1px,transparent 1px);background-size:74px 74px;mask-image:linear-gradient(to bottom,rgba(0,0,0,.85),transparent 92%);pointer-events:none}
        .heroGrid{position:relative;z-index:1;max-width:1320px;margin:0 auto;padding:118px 40px 78px;display:grid;grid-template-columns:minmax(0,.78fr) minmax(540px,1.22fr);gap:46px;align-items:center}.heroCopy{max-width:590px}.eyebrow{display:inline-flex;align-items:center;border:1px solid rgba(213,154,53,.28);border-radius:999px;background:rgba(255,250,240,.72);color:#6f4b15;padding:8px 13px;font:850 11px/1 var(--mono);letter-spacing:.08em;text-transform:uppercase;box-shadow:0 18px 48px -38px rgba(74,52,14,.55);backdrop-filter:blur(14px)}
        .heroTitle{margin:18px 0 0;max-width:620px;color:var(--fk-ink);font-family:var(--disp);font-size:clamp(50px,6.4vw,88px);font-weight:850;line-height:.9;letter-spacing:-.07em;text-wrap:balance}.heroTitle span{display:block;margin-top:10px;background:linear-gradient(96deg,var(--fk-copper),var(--fk-gold) 52%,var(--fk-sage));-webkit-background-clip:text;background-clip:text;color:transparent}.heroSub{max-width:540px;margin-top:18px;color:var(--fk-muted);font-size:17px;line-height:1.65}.heroCtas,.cliQuickActions,.ctaBtns{display:flex;flex-wrap:wrap;gap:12px;margin-top:24px}.heroCtas .btn,.cliQuickActions .btn,.ctaBtns .btn{min-height:50px;border-radius:999px;transition:transform .18s ease,box-shadow .18s ease,filter .18s ease}.heroCtas .btn:hover,.cliQuickActions .btn:hover,.ctaBtns .btn:hover{transform:translateY(-3px) scale(1.01)}.heroCtas .btn:active,.cliQuickActions .btn:active,.ctaBtns .btn:active{transform:translateY(0) scale(.97)}
        .heroPrimary{background:linear-gradient(135deg,var(--fk-gold),#f2c978)!important;color:#22170a!important;box-shadow:0 22px 50px -28px rgba(173,109,28,.75)!important}.heroSecondary,.cliSecondary{background:rgba(255,250,240,.7)!important;color:var(--fk-ink)!important;border:1px solid var(--fk-line)!important}.heroTrustGrid{display:flex;flex-wrap:wrap;gap:8px;margin-top:20px}.heroTrustItem{padding:9px 12px;border:1px solid var(--fk-line);border-radius:999px;background:rgba(255,250,240,.58);transition:transform .18s ease,border-color .18s ease}.heroTrustItem:hover{transform:translateY(-3px);border-color:rgba(213,154,53,.45)}.heroTrustItem b{margin-right:6px;color:var(--fk-copper);font:900 12px/1 var(--mono)}.heroTrustItem small{color:var(--fk-muted);font:760 10px/1 var(--mono)}
        .heroStageCarousel{position:relative;min-height:520px;border:1px solid rgba(255,250,240,.34);border-radius:34px;overflow:hidden;background:var(--fk-night);box-shadow:0 42px 120px -68px rgba(37,31,20,.85),inset 0 1px 0 rgba(255,255,255,.16);isolation:isolate;transition:transform .24s ease,box-shadow .24s ease,border-color .24s ease}.heroStageCarousel:hover{transform:translateY(-6px);border-color:rgba(213,154,53,.5);box-shadow:0 58px 132px -74px rgba(133,87,22,.82),inset 0 1px 0 rgba(255,255,255,.18)}.heroStageCarousel:active{transform:translateY(-2px) scale(.992)}
        .heroStageSlide{position:absolute;inset:0;display:grid;grid-template-columns:minmax(0,1fr) minmax(250px,310px) 138px;gap:18px;align-items:end;padding:30px;opacity:0;color:#fff;text-decoration:none;animation:heroStageCycle 24s infinite;animation-delay:calc(var(--slide-index) * 6s);pointer-events:none}.heroStageCarousel:hover .heroStageSlide,.heroStageCarousel:hover .heroStageNav:before{animation-play-state:paused}.heroStageImage{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;filter:saturate(1.04) contrast(1.02);transform:scale(1.04);animation:heroImageDrift 12s ease-in-out infinite alternate}.heroStageShade{position:absolute;inset:0;background:linear-gradient(90deg,rgba(19,17,12,.05),rgba(19,17,12,.38) 42%,rgba(19,17,12,.9) 78%),linear-gradient(180deg,rgba(19,17,12,.08),rgba(19,17,12,.52))}.heroStageCopy{position:relative;z-index:2;grid-column:2;align-self:end;padding-bottom:8px;opacity:0;transform:translateY(18px);animation:heroCopyCycle 24s infinite;animation-delay:calc(var(--slide-index) * 6s)}.heroStageCopy h3{margin-top:15px;color:#fff;font:850 clamp(30px,3.2vw,45px)/.98 var(--disp);letter-spacing:-.055em;text-wrap:balance}.heroStageCopy p{margin-top:12px;color:rgba(255,250,240,.74);font-size:13.5px;line-height:1.58}.heroStageActions{display:flex;gap:9px;flex-wrap:wrap;margin-top:18px}.heroModePill,.heroModeMetric,.heroModeLink{display:inline-flex;align-items:center;justify-content:center;min-height:36px;border-radius:999px;font:900 10px/1 var(--mono);letter-spacing:.06em}.heroModePill{padding:0 11px;background:rgba(240,195,108,.18);color:#ffe1a0;border:1px solid rgba(240,195,108,.34)}.heroModeMetric{padding:0 12px;background:rgba(255,250,240,.12);color:#fff;border:1px solid rgba(255,250,240,.18)}.heroModeLink{padding:0 14px;background:var(--fk-gold);color:#201406;box-shadow:0 18px 38px -26px rgba(240,195,108,.9)}
        .heroStageList{position:absolute;z-index:5;top:18px;right:18px;bottom:18px;width:126px;display:grid;grid-template-rows:repeat(4,1fr) auto;gap:9px}.heroStageNav,.heroStageAll{position:relative;overflow:hidden;border:1px solid rgba(255,250,240,.16);border-radius:17px;background:rgba(255,250,240,.09);color:rgba(255,250,240,.78);text-decoration:none;padding:12px;backdrop-filter:blur(12px);transition:transform .18s ease,background-color .18s ease,border-color .18s ease}.heroStageNav:hover,.heroStageAll:hover{transform:translateX(-4px);background:rgba(240,195,108,.18);border-color:rgba(240,195,108,.38);color:#fff}.heroStageNav:before{content:"";position:absolute;left:0;top:0;bottom:0;width:3px;background:var(--fk-gold);transform:scaleY(0);transform-origin:top;animation:heroNavCycle 24s infinite;animation-delay:calc(var(--slide-index) * 6s)}.heroStageNav span{display:block;color:#f5c86b;font:900 11px/1 var(--mono)}.heroStageNav b{display:block;margin-top:14px;color:#fff;font:850 16px/1 var(--disp)}.heroStageNav small{display:block;margin-top:5px;color:rgba(255,250,240,.54);font:750 9px/1.25 var(--mono)}.heroStageAll{display:flex;align-items:center;justify-content:center;min-height:38px;background:var(--fk-gold);color:#211405;font-weight:850}
        @keyframes heroStageCycle{0%,21%{opacity:1;pointer-events:auto}26%,100%{opacity:0;pointer-events:none}}@keyframes heroCopyCycle{0%,18%{opacity:1;transform:translateY(0)}24%,100%{opacity:0;transform:translateY(18px)}}@keyframes heroNavCycle{0%,21%{transform:scaleY(1)}26%,100%{transform:scaleY(0)}}@keyframes heroImageDrift{to{transform:scale(1.1) translate3d(-1.2%,1.1%,0)}}
        .modelLogoMarquee{background:linear-gradient(180deg,var(--fk-paper),var(--fk-bg));border-bottom:1px solid var(--fk-line);overflow:hidden}.modelLogoInner{max-width:1320px;margin:0 auto;padding:22px 40px;display:grid;grid-template-columns:260px minmax(0,1fr);gap:22px;align-items:center}.marqueeLabel span{display:block}.marqueeLabel span:first-child{color:var(--fk-ink);font:850 12px/1.2 var(--mono);letter-spacing:.08em;text-transform:uppercase}.marqueeLabel span:last-child{margin-top:7px;color:var(--fk-muted);font-size:12px}.logoTrack{display:flex;gap:12px;width:max-content;animation:logoMove 28s linear infinite}.logoTrack:hover{animation-play-state:paused}.logoPill{min-width:174px;height:62px;display:flex;align-items:center;gap:11px;border:1px solid var(--fk-line);border-radius:17px;background:rgba(255,250,240,.72);color:var(--fk-ink);text-decoration:none;padding:0 14px;box-shadow:0 18px 44px -36px rgba(30,33,22,.38);transition:transform .18s ease,border-color .18s ease,box-shadow .18s ease}.logoPill:hover{transform:translateY(-4px) scale(1.015);border-color:rgba(213,154,53,.45);box-shadow:0 26px 58px -42px rgba(93,68,24,.62)}.logoPill img{width:30px;height:30px;object-fit:contain}.logoPill b{display:block;font-size:13px}.logoPill span{display:block;margin-top:3px;color:var(--fk-muted);font:750 9px/1 var(--mono);text-transform:uppercase}@keyframes logoMove{to{transform:translateX(-50%)}}
        .modelTypes,.priceProof,.cliQuick,.why,.voiceFaq,.ctaWrap{position:relative;overflow:hidden;background:var(--fk-bg);color:var(--fk-ink);border-bottom:1px solid var(--fk-line)}.modelTypesIn,.priceProofIn,.cliQuickIn,.whyIn,.voiceFaqIn{max-width:1320px;margin:0 auto;padding:88px 40px}.sectionSplit,.priceProofHead{display:grid;grid-template-columns:minmax(0,.85fr) minmax(0,1.15fr);gap:44px;align-items:end}.kick2{color:var(--fk-copper);font:850 11px/1.2 var(--mono);letter-spacing:.1em;text-transform:uppercase}.sectionTitle{margin-top:16px;color:var(--fk-ink);font-family:var(--disp);font-size:clamp(40px,4.6vw,66px);font-weight:850;line-height:.98;letter-spacing:-.055em;text-wrap:balance}.sectionSub{margin:0;color:var(--fk-muted);font-size:16px;line-height:1.7}.typeGrid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px;margin-top:34px}.typeCard{min-height:390px;padding:16px;border:1px solid var(--fk-line);border-radius:26px;background:linear-gradient(180deg,var(--fk-paper),var(--fk-paper-2));color:var(--fk-ink);text-decoration:none;box-shadow:0 26px 74px -58px rgba(32,34,24,.55);transition:transform .22s ease,border-color .22s ease,box-shadow .22s ease}.typeCard:hover{transform:translateY(-9px) scale(1.012);border-color:rgba(213,154,53,.52);box-shadow:0 42px 96px -66px rgba(103,71,21,.74)}.typeCard:active{transform:translateY(-3px) scale(.982)}.typeVisual{height:190px;margin-bottom:20px;border-radius:20px;background:#17150f;overflow:hidden}.typeVisual img{width:100%;height:100%;object-fit:cover;display:block;transition:transform .5s ease}.typeCard:hover .typeVisual img{transform:scale(1.08)}.typeCard b{display:block;font:850 25px/1 var(--disp);letter-spacing:-.04em}.typeCard strong{display:block;margin-top:10px;color:var(--fk-copper);font:900 34px/1 var(--disp)}.typeCard p{margin-top:12px;color:var(--fk-muted);font-size:13px;line-height:1.55}
        .priceProof{background:linear-gradient(180deg,var(--fk-bg),var(--fk-bg-2))}.priceModelGrid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px;margin-top:34px}.priceModelCard{overflow:hidden;min-height:308px;border:1px solid var(--fk-line);border-radius:24px;background:rgba(255,250,240,.76);color:var(--fk-ink);text-decoration:none;box-shadow:0 26px 74px -58px rgba(35,35,24,.55);transition:transform .22s ease,border-color .22s ease,box-shadow .22s ease}.priceModelCard:hover{transform:translateY(-8px);border-color:rgba(213,154,53,.55);box-shadow:0 42px 92px -64px rgba(104,70,19,.72)}.priceModelCard:active{transform:translateY(-2px) scale(.982)}.priceModelPoster{height:132px;overflow:hidden;background:#15130e}.priceModelPoster img{width:100%;height:100%;object-fit:cover;display:block;transition:transform .5s ease}.priceModelCard:hover .priceModelPoster img{transform:scale(1.08)}.priceModelTop{display:flex;justify-content:space-between;gap:12px;padding:17px 17px 0}.priceModelTop b{display:block;max-width:190px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--fk-ink);font-size:15px}.priceModelTop small{display:block;margin-top:5px;color:var(--fk-muted);font-size:11px}.priceModelTop i{display:grid;place-items:center;height:30px;border-radius:999px;background:var(--fk-gold);color:#22170a;font:850 11px/1 var(--mono);font-style:normal;padding:0 10px}.priceModelNumbers{display:grid;grid-template-columns:1fr 1fr;gap:10px;padding:16px 17px 0}.priceModelNumbers span{min-width:0;border:1px solid var(--fk-line);border-radius:16px;background:rgba(255,255,255,.38);padding:12px}.priceModelNumbers small{display:block;color:var(--fk-muted);font:800 9px/1 var(--mono);letter-spacing:.06em;text-transform:uppercase}.priceModelNumbers s{display:block;margin-top:8px;color:#9b8b74;font-size:12px}.priceModelNumbers strong{display:block;margin-top:8px;color:var(--fk-copper);font:900 13px/1.2 var(--mono)}.priceModelCard p{padding:13px 17px 18px;color:var(--fk-muted);font-size:12px;line-height:1.55}.priceNote{display:flex;flex-wrap:wrap;gap:10px;margin-top:18px}.priceNote span,.priceNote a{border:1px solid var(--fk-line);border-radius:999px;background:rgba(255,250,240,.56);color:var(--fk-muted);text-decoration:none;padding:9px 12px;font:800 10px/1 var(--mono)}.priceNote a{color:var(--fk-copper)}
        .cliQuick{background:linear-gradient(180deg,var(--fk-paper),var(--fk-bg))}.cliQuickIn{display:grid;grid-template-columns:minmax(0,.9fr) minmax(420px,1.1fr);gap:34px;align-items:center}.cliPrimary{background:var(--fk-ink)!important;color:var(--fk-bg)!important}.cliQuickPanel{position:relative;overflow:hidden;border:1px solid var(--fk-line);border-radius:28px;background:linear-gradient(135deg,var(--fk-night),#283122);color:#f8eedc;padding:22px;box-shadow:0 34px 86px -58px rgba(30,32,21,.82);transition:transform .22s ease,box-shadow .22s ease,border-color .22s ease}.cliQuickPanel:before{content:"";position:absolute;inset:0;background-image:linear-gradient(to right,rgba(240,195,108,.08) 1px,transparent 1px),linear-gradient(to bottom,rgba(240,195,108,.06) 1px,transparent 1px);background-size:38px 38px;mask-image:radial-gradient(ellipse at 70% 0,black,transparent 80%)}.cliQuickPanel:hover{transform:translateY(-7px);border-color:rgba(213,154,53,.5);box-shadow:0 46px 100px -64px rgba(103,71,21,.82)}.cliQuickTop,.cliQuickPanel pre,.cliQuickChecks{position:relative;z-index:1}.cliQuickTop{display:flex;justify-content:space-between;gap:12px;color:#f0c36c;font:850 11px/1 var(--mono)}.cliQuickTop i{font-style:normal;color:rgba(248,238,220,.6)}.cliQuickPanel pre{margin:28px 0 0;padding:22px;border-radius:18px;background:rgba(255,250,240,.08);white-space:pre-wrap;color:#f8eedc;font:650 15px/1.7 var(--mono)}.cliQuickPanel pre span{color:#f0c36c}.cliQuickPanel pre em{font-style:normal;color:#cfe6c7}.cliQuickChecks{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:14px}.cliQuickChecks span{border:1px solid rgba(248,238,220,.14);border-radius:15px;background:rgba(255,250,240,.07);padding:13px;color:rgba(248,238,220,.78);font:760 10px/1.45 var(--mono)}
        .why{background:var(--fk-bg-2)}.whyIn{display:grid;grid-template-columns:330px 1fr;gap:34px}.whyGrid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.whyCard{min-height:236px;border:1px solid var(--fk-line);border-radius:24px;background:rgba(255,250,240,.68);padding:26px;color:var(--fk-ink);box-shadow:0 26px 74px -60px rgba(30,32,22,.55);transition:transform .22s ease,border-color .22s ease,box-shadow .22s ease}.whyCard:hover{transform:translateY(-8px);border-color:rgba(213,154,53,.48);box-shadow:0 42px 92px -66px rgba(91,64,18,.7)}.whyCard:first-child{background:linear-gradient(135deg,var(--fk-night),#253421);color:#fff}.whyCard:first-child p{color:rgba(248,238,220,.72)}.whyCard h3{font:850 23px/1 var(--disp);letter-spacing:-.04em}.whyCard p{margin-top:12px;color:var(--fk-muted);font-size:13.5px;line-height:1.65}.whyMini{display:flex;flex-wrap:wrap;gap:8px;margin-top:18px}.whyMini span{border:1px solid var(--fk-line);border-radius:999px;background:rgba(255,250,240,.58);padding:8px 10px;color:var(--fk-muted);font:800 10px/1 var(--mono)}
        .voiceGrid,.faqGrid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.voiceGrid{margin-top:34px}.faqGrid{margin-top:16px}.quoteCard,.faqCard{border:1px solid var(--fk-line);border-radius:22px;background:rgba(255,250,240,.68);padding:24px;color:var(--fk-ink);box-shadow:0 24px 68px -58px rgba(30,32,22,.55);transition:transform .18s ease,border-color .18s ease}.quoteCard:hover,.faqCard:hover{transform:translateY(-6px);border-color:rgba(213,154,53,.5)}.quoteCard p{font-size:18px;line-height:1.55}.quoteCard b{display:block;margin-top:18px;color:var(--fk-copper);font:800 11px/1 var(--mono);letter-spacing:.08em;text-transform:uppercase}.faqCard h3{font-size:15px;color:var(--fk-ink)}.faqCard p{margin-top:8px;color:var(--fk-muted);font-size:12.5px;line-height:1.6}
        .ctaWrap{padding:88px 40px;background:linear-gradient(180deg,var(--fk-bg),var(--fk-bg-2))}.ctaBanner{display:grid;grid-template-columns:minmax(0,.86fr) minmax(360px,1.14fr);gap:34px;align-items:center;max-width:1320px;margin:0 auto;padding:30px;border:1px solid var(--fk-line);border-radius:34px;background:linear-gradient(135deg,var(--fk-night),#2d2a1f);box-shadow:0 42px 110px -72px rgba(21,23,15,.92);text-align:left}.ctaIn{padding:18px}.ctaBanner .kick2{color:#f0c36c}.ctaBanner h2{margin-top:16px;color:#fff;font:850 clamp(38px,4.8vw,66px)/.98 var(--disp);letter-spacing:-.06em;text-wrap:balance}.ctaBanner p{margin:18px 0 28px;color:rgba(248,238,220,.72);line-height:1.7}.ctaBillboard{position:relative;overflow:hidden;aspect-ratio:16/10;border:1px solid rgba(248,238,220,.16);border-radius:24px;background:#111;box-shadow:0 34px 80px -52px #000}.ctaBillboard img{object-fit:cover;object-position:68% 38%;transition:transform .55s ease}.ctaBanner:hover .ctaBillboard img{transform:scale(1.04)}.ctaTrust{display:flex;flex-wrap:wrap;gap:8px;margin-top:12px}.ctaTrust span{border:1px solid rgba(248,238,220,.16);border-radius:999px;background:rgba(248,238,220,.08);padding:9px 12px;color:rgba(248,238,220,.78);font:850 10px/1 var(--mono)}
        .fk-reveal{opacity:0;transform:translateY(26px);transition:opacity .7s ease,transform .7s ease}.fk-reveal.fk-in{opacity:1;transform:none}@media(max-width:1120px){.heroGrid,.sectionSplit,.priceProofHead,.cliQuickIn,.whyIn,.ctaBanner{grid-template-columns:1fr}.heroProductRail{max-width:900px;width:100%}.priceModelGrid,.typeGrid{grid-template-columns:repeat(2,minmax(0,1fr))}.whyHead{position:static}}@media(max-width:720px){.heroGrid,.modelTypesIn,.priceProofIn,.cliQuickIn,.whyIn,.voiceFaqIn{padding-left:20px;padding-right:20px}.heroGrid{padding-top:104px}.heroTitle{font-size:clamp(43px,13vw,58px)}.heroSub{font-size:15.5px}.heroStageCarousel{min-height:520px;border-radius:24px}.heroStageSlide{grid-template-columns:1fr;padding:20px 20px 126px}.heroStageCopy{grid-column:1}.heroStageList{left:16px;right:16px;top:auto;bottom:14px;width:auto;height:98px;grid-template-columns:repeat(4,1fr);grid-template-rows:1fr}.heroStageNav{padding:10px 8px;border-radius:14px}.heroStageNav small,.heroStageAll{display:none}.heroStageNav b{font-size:13px;margin-top:10px}.heroStageShade{background:linear-gradient(180deg,rgba(19,17,12,.05),rgba(19,17,12,.86))}.heroTrustGrid{display:none}.priceModelGrid,.typeGrid,.voiceGrid,.faqGrid,.whyGrid,.cliQuickChecks{grid-template-columns:1fr}.typeCard{min-height:340px}.priceModelCard{min-height:292px}.ctaWrap{padding:64px 20px}.ctaBanner{padding:18px;border-radius:24px}.ctaBillboard{aspect-ratio:4/3}.modelLogoInner{grid-template-columns:1fr;padding-left:20px;padding-right:20px}.logoPill{min-width:150px}}
        body:has(> header.hero.heroUnified){--fk-bg:#fbf6ef;--fk-bg-2:#f4ece4;--fk-paper:#fffaf5;--fk-paper-2:#f7efe7;--fk-ink:#161312;--fk-muted:#665f58;--fk-line:rgba(31,26,23,.12);--fk-gold:#ffb86f;--fk-copper:#ff7358;--fk-sage:#7b6cff;--fk-night:#151217;background:var(--fk-bg)}
        html.fk-theme-night body:has(> header.hero.heroUnified){--fk-bg:#121115;--fk-bg-2:#1a1718;--fk-paper:#211d1e;--fk-paper-2:#2a2423;--fk-ink:#fff7ee;--fk-muted:#c7b9ac;--fk-line:rgba(255,247,238,.15);--fk-gold:#ffc982;--fk-copper:#ff8a68;--fk-sage:#a99bff;--fk-night:#0c0b0e}
        .heroUnified{min-height:100svh;color:#fff;background:#151217;border-bottom:0}
        .heroUnified:before{z-index:4;background-image:linear-gradient(to right,rgba(255,255,255,.07) 1px,transparent 1px),linear-gradient(to bottom,rgba(255,255,255,.05) 1px,transparent 1px);background-size:82px 82px;mask-image:linear-gradient(to bottom,rgba(0,0,0,.62),transparent 92%)}
        .heroUnified:after{content:"";position:absolute;inset:auto 0 0;z-index:4;height:34%;background:linear-gradient(180deg,transparent,var(--fk-bg));pointer-events:none}
        .heroGrid{position:relative;z-index:6;max-width:1360px;min-height:100svh;margin:0 auto;padding:136px clamp(22px,3vw,48px) 74px;display:grid;grid-template-columns:minmax(0,.96fr) minmax(310px,.54fr) 142px;gap:28px;align-items:end}
        .heroCopy{position:relative;z-index:8;max-width:650px;padding:24px 0 34px;color:#fff}.heroCopy:before{content:"";position:absolute;z-index:-1;inset:-28px -34px -30px -30px;border-radius:32px;background:radial-gradient(ellipse at 18% 0,rgba(255,184,111,.2),transparent 58%),linear-gradient(135deg,rgba(17,13,14,.5),rgba(17,13,14,.12));filter:blur(.1px)}
        .heroCopy .eyebrow{border-color:rgba(255,255,255,.18);background:rgba(255,255,255,.13);color:#fff4e6;box-shadow:inset 0 1px 0 rgba(255,255,255,.14),0 20px 54px -40px #000;backdrop-filter:blur(14px)}
        .heroTitle{max-width:680px;color:#fff;font-size:clamp(54px,6.2vw,92px);line-height:.88;letter-spacing:-.07em;text-shadow:0 20px 58px rgba(0,0,0,.38)}
        .heroTitle span{background:linear-gradient(96deg,#fff2dc 0%,var(--fk-gold) 34%,var(--fk-copper) 68%,#b8aeff 100%);-webkit-background-clip:text;background-clip:text;color:transparent}
        .heroSub{max-width:560px;color:rgba(255,247,238,.78);font-size:17px}.heroTrustItem{border-color:rgba(255,255,255,.14);background:rgba(255,255,255,.1);backdrop-filter:blur(12px)}.heroTrustItem b{color:#ffd49a}.heroTrustItem small{color:rgba(255,247,238,.66)}
        .heroProductRail{position:absolute;inset:0;z-index:1;max-width:none!important;width:100%!important;pointer-events:auto}.heroStageCarousel{position:absolute;inset:0;min-height:0;border:0;border-radius:0;background:#151217;box-shadow:none;transition:none}.heroStageCarousel:hover{transform:none;border-color:transparent;box-shadow:none}.heroStageSlide{position:absolute;inset:0;display:block;padding:0;color:#fff;opacity:0;animation:heroStageCycle 24s infinite;animation-delay:calc(var(--slide-index) * 6s);pointer-events:none}.heroStageImage{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;object-position:center;filter:saturate(1.06) contrast(1.02) brightness(.9);transform:scale(1.01);animation:heroImageDrift 14s ease-in-out infinite alternate}.heroStageShade{position:absolute;z-index:1;inset:0;background:linear-gradient(90deg,rgba(13,11,13,.82) 0%,rgba(13,11,13,.48) 36%,rgba(13,11,13,.18) 58%,rgba(13,11,13,.76) 100%),linear-gradient(180deg,rgba(13,11,13,.2) 0%,rgba(13,11,13,.24) 44%,rgba(13,11,13,.82) 100%)}
        .heroStageCopy{position:absolute;z-index:6;right:186px;bottom:84px;width:min(360px,25vw);padding:19px;border:1px solid rgba(255,255,255,.15);border-radius:24px;background:rgba(255,255,255,.1);box-shadow:inset 0 1px 0 rgba(255,255,255,.14),0 32px 86px -54px rgba(0,0,0,.82);backdrop-filter:blur(16px) saturate(1.18);opacity:0;transform:translateY(18px);animation:heroCopyCycle 24s infinite;animation-delay:calc(var(--slide-index) * 6s)}.heroStageCopy h3{font-size:clamp(28px,2.8vw,42px)}.heroStageCopy p{color:rgba(255,247,238,.72)}
        .heroStageList{position:absolute;z-index:9;top:50%;right:34px;bottom:auto;width:122px;height:min(520px,58svh);display:grid;grid-template-rows:repeat(4,1fr) auto;gap:10px;transform:translateY(-50%)}.heroStageNav,.heroStageAll{border-color:rgba(255,255,255,.17);background:rgba(255,255,255,.11);backdrop-filter:blur(16px) saturate(1.18)}.heroStageNav:hover,.heroStageAll:hover{background:rgba(255,184,111,.2);border-color:rgba(255,184,111,.42)}.heroStageNav:before{background:linear-gradient(180deg,var(--fk-gold),var(--fk-copper))}.heroStageNav span{color:#ffd49a}.heroStageAll{background:linear-gradient(135deg,#ffe0ba,var(--fk-copper));color:#20130f}
        .heroPrimary{background:linear-gradient(135deg,#ffe0ba 0%,#ffb86f 42%,#ff7358 100%)!important;color:#21130f!important;box-shadow:0 24px 52px -28px rgba(255,115,88,.9)!important}.heroSecondary{background:rgba(255,255,255,.12)!important;color:#fff7ee!important;border-color:rgba(255,255,255,.16)!important;backdrop-filter:blur(14px)}
        .modelLogoMarquee,.modelTypes,.priceProof,.cliQuick,.why,.voiceFaq,.ctaWrap{background:var(--fk-bg);color:var(--fk-ink)}.modelLogoMarquee{background:linear-gradient(180deg,var(--fk-bg),var(--fk-bg-2))}.typeCard,.priceModelCard,.quoteCard,.faqCard,.whyCard{background:rgba(255,250,245,.72)}.sectionTitle{word-break:keep-all;overflow-wrap:normal}.sectionSub{color:var(--fk-muted)}
        @media(max-width:1120px){.heroGrid{grid-template-columns:minmax(0,1fr) 132px;gap:18px}.heroCopy{max-width:620px}.heroStageCopy{right:176px;width:min(330px,34vw)}.heroProductRail{max-width:none!important}}
        @media(max-width:720px){.heroUnified{min-height:100svh}.heroGrid{grid-template-columns:1fr;min-height:100svh;padding:112px 20px 184px}.heroCopy{padding-bottom:0}.heroCopy:before{inset:-18px -16px;border-radius:24px}.heroTitle{font-size:clamp(46px,14vw,60px)}.heroSub{font-size:15.5px}.heroStageCopy{left:20px;right:20px;bottom:104px;width:auto;padding:14px;border-radius:18px}.heroStageCopy h3{font-size:24px}.heroStageCopy p,.heroStageActions{display:none}.heroStageList{left:16px;right:16px;top:auto;bottom:14px;width:auto;height:78px;grid-template-columns:repeat(4,1fr);grid-template-rows:1fr;transform:none}.heroStageNav{padding:10px 8px;border-radius:14px}.heroStageNav small,.heroStageAll{display:none}.heroStageNav b{font-size:13px;margin-top:8px}.heroStageShade{background:linear-gradient(180deg,rgba(13,11,13,.26),rgba(13,11,13,.88))}.heroTrustGrid{display:none}.modelLogoInner{grid-template-columns:1fr}}
        body:has(> header.hero.heroUnified) .heroUnified .heroGrid{position:static!important;z-index:auto!important;grid-template-columns:minmax(0,.95fr) minmax(0,.72fr) 142px}
        body:has(> header.hero.heroUnified) .heroUnified .heroProductRail{position:absolute!important;inset:0!important;z-index:1!important;max-width:none!important;width:100%!important}
        body:has(> header.hero.heroUnified) .heroUnified .heroCopy{position:relative!important;z-index:8!important}
        body:has(> header.hero.heroUnified) .heroUnified .heroStageCarousel{position:absolute!important;inset:0!important;min-height:0!important;border-radius:0!important}
        body:has(> header.hero.heroUnified) .heroUnified .heroStageImage{object-fit:cover;object-position:center}
        body:has(> header.hero.heroUnified) .sectionSplit,
        body:has(> header.hero.heroUnified) .priceProofHead{grid-template-columns:minmax(0,.78fr) minmax(360px,.86fr);align-items:start}
        body:has(> header.hero.heroUnified) .sectionTitle{max-width:760px;font-size:clamp(34px,4vw,58px);line-height:1.03;letter-spacing:-.045em;word-break:keep-all;overflow-wrap:normal}
        body:has(> header.hero.heroUnified) .whyIn{grid-template-columns:minmax(0,.76fr) minmax(0,1.24fr);align-items:start}
        body:has(> header.hero.heroUnified) .whyHead .sectionTitle{max-width:560px;font-size:clamp(34px,3.55vw,52px)}
        body:has(> header.hero.heroUnified) .whyGrid{align-self:start}
        body:has(> header.hero.heroUnified) .ctaBillboard{background:linear-gradient(135deg,#211a18,#080808)}
        body:has(> header.hero.heroUnified) .ctaBillboard img{object-fit:cover!important;opacity:1!important}
        @media(max-width:1120px){body:has(> header.hero.heroUnified) .heroUnified .heroGrid{grid-template-columns:minmax(0,1fr) 132px}body:has(> header.hero.heroUnified) .sectionSplit,body:has(> header.hero.heroUnified) .priceProofHead,body:has(> header.hero.heroUnified) .whyIn{grid-template-columns:1fr}body:has(> header.hero.heroUnified) .sectionTitle,body:has(> header.hero.heroUnified) .whyHead .sectionTitle{max-width:820px}}
        @media(max-width:720px){body:has(> header.hero.heroUnified) .heroUnified .heroGrid{grid-template-columns:1fr}body:has(> header.hero.heroUnified) .sectionTitle{font-size:clamp(32px,10vw,44px);line-height:1.08}}
        body:has(> header.hero.heroUnified){--fk-bg:#f7fcff;--fk-bg-2:#fff3f8;--fk-paper:#ffffff;--fk-paper-2:#f0f9ff;--fk-ink:#0d1b2d;--fk-muted:#54677c;--fk-line:rgba(34,92,128,.13);--fk-gold:#72cfff;--fk-copper:#ff8fb7;--fk-sage:#7f8cff;--fk-night:#eef9ff;background:linear-gradient(180deg,#f8fcff 0%,#fff5f9 100%)}
        body:has(> header.hero.heroUnified) .heroUnified{min-height:100svh;color:var(--fk-ink);background:radial-gradient(ellipse 46% 38% at 22% 18%,rgba(114,207,255,.34),transparent 68%),radial-gradient(ellipse 44% 42% at 80% 16%,rgba(255,159,192,.3),transparent 66%),linear-gradient(135deg,#f7fcff 0%,#eef9ff 44%,#fff3f8 100%);border-bottom:1px solid rgba(114,207,255,.18);isolation:isolate}
        body:has(> header.hero.heroUnified) .heroUnified:before{z-index:3;background-image:linear-gradient(to right,rgba(80,162,210,.09) 1px,transparent 1px),linear-gradient(to bottom,rgba(255,159,192,.08) 1px,transparent 1px);background-size:86px 86px;mask-image:linear-gradient(to bottom,rgba(0,0,0,.7),transparent 92%)}
        body:has(> header.hero.heroUnified) .heroUnified:after{z-index:4;height:28%;background:linear-gradient(180deg,transparent,#f7fcff);pointer-events:none}
        body:has(> header.hero.heroUnified) .heroStageCarousel{position:absolute!important;inset:0!important;z-index:1!important;min-height:0!important;border:0!important;border-radius:0!important;background:#f7fcff!important;box-shadow:none!important;overflow:hidden;pointer-events:none}
        body:has(> header.hero.heroUnified) .heroStageSlide{position:absolute!important;inset:0!important;display:block!important;padding:0!important;opacity:0!important;animation:none!important;transition:opacity .72s ease!important;pointer-events:none!important}
        body:has(> header.hero.heroUnified) .heroStageSlide.is-active{opacity:1!important}
        body:has(> header.hero.heroUnified) .heroStageImage{position:absolute!important;inset:0!important;width:100%!important;height:100%!important;object-fit:cover!important;object-position:center!important;filter:saturate(1.08) contrast(1.01) brightness(1.06)!important;opacity:.86;transform:scale(1.015)!important;animation:heroImageDrift 16s ease-in-out infinite alternate!important}
        body:has(> header.hero.heroUnified) .heroStageShade{position:absolute!important;z-index:1!important;inset:0!important;background:linear-gradient(90deg,rgba(247,252,255,.95) 0%,rgba(247,252,255,.78) 35%,rgba(247,252,255,.28) 62%,rgba(255,243,248,.88) 100%),linear-gradient(180deg,rgba(247,252,255,.42) 0%,rgba(247,252,255,.12) 48%,rgba(247,252,255,.9) 100%)!important}
        body:has(> header.hero.heroUnified) .heroGrid{position:relative!important;z-index:8!important;display:grid!important;grid-template-columns:minmax(0,1fr) minmax(310px,380px)!important;gap:clamp(26px,4vw,58px)!important;align-items:center!important;max-width:1420px!important;min-height:100svh!important;margin:0 auto!important;padding:126px clamp(22px,3.2vw,50px) 76px!important}
        body:has(> header.hero.heroUnified) .heroCopy{position:relative!important;z-index:8!important;max-width:710px!important;margin-left:clamp(24px,6vw,128px)!important;padding:28px!important;color:var(--fk-ink)!important;animation:heroCopyIn .45s ease both}
        body:has(> header.hero.heroUnified) .heroCopy:before{content:"";position:absolute;z-index:-1;inset:0;border:1px solid rgba(114,207,255,.22);border-radius:30px;background:linear-gradient(135deg,rgba(255,255,255,.68),rgba(255,255,255,.36));box-shadow:0 30px 90px -58px rgba(56,117,158,.42),inset 0 1px 0 rgba(255,255,255,.82);backdrop-filter:blur(18px) saturate(1.08)}
        body:has(> header.hero.heroUnified) .heroCopy .eyebrow{border-color:rgba(114,207,255,.34)!important;background:rgba(255,255,255,.68)!important;color:#0b78ad!important;box-shadow:0 14px 36px -28px rgba(56,117,158,.48)!important;text-shadow:none!important}
        body:has(> header.hero.heroUnified) .heroTitle{max-width:690px!important;margin-top:18px!important;color:#0d1b2d!important;font-size:clamp(48px,5.4vw,76px)!important;line-height:.96!important;letter-spacing:-.055em!important;text-shadow:none!important;text-wrap:balance}
        body:has(> header.hero.heroUnified) .heroTitle span{display:block;margin-top:10px;background:linear-gradient(96deg,#0b78ad 0%,#4c8dff 44%,#ff7fae 100%)!important;-webkit-background-clip:text!important;background-clip:text!important;color:transparent!important}
        body:has(> header.hero.heroUnified) .heroSub{max-width:640px!important;margin-top:18px!important;color:#40566b!important;font-size:17px!important;line-height:1.72!important;text-shadow:none!important}
        body:has(> header.hero.heroUnified) .heroCtas{gap:10px!important;margin-top:24px!important}
        body:has(> header.hero.heroUnified) .heroCtas .btn{min-height:48px!important;border-radius:999px!important;padding-inline:20px!important}
        body:has(> header.hero.heroUnified) .heroPrimary{background:linear-gradient(135deg,#72cfff 0%,#a7ddff 45%,#ffc1d6 100%)!important;color:#092235!important;border:1px solid rgba(255,255,255,.8)!important;box-shadow:0 22px 52px -30px rgba(40,143,205,.78),inset 0 1px 0 rgba(255,255,255,.78)!important}
        body:has(> header.hero.heroUnified) .heroSecondary{background:rgba(255,255,255,.72)!important;color:#12334f!important;border:1px solid rgba(114,207,255,.28)!important;box-shadow:0 18px 42px -34px rgba(56,117,158,.38)!important;backdrop-filter:blur(12px)}
        body:has(> header.hero.heroUnified) .heroGhost{background:rgba(255,255,255,.42)!important;color:#52687d!important;border:1px solid rgba(255,159,192,.26)!important;box-shadow:none!important}
        body:has(> header.hero.heroUnified) .heroPrimary:hover,body:has(> header.hero.heroUnified) .heroSecondary:hover,body:has(> header.hero.heroUnified) .heroGhost:hover{transform:translateY(-3px) scale(1.01)!important;filter:saturate(1.05)}
        body:has(> header.hero.heroUnified) .heroTrustGrid{display:flex!important;flex-wrap:wrap!important;gap:9px!important;margin-top:22px!important}
        body:has(> header.hero.heroUnified) .heroTrustItem{padding:9px 12px!important;border:1px solid rgba(114,207,255,.24)!important;border-radius:999px!important;background:rgba(255,255,255,.58)!important;color:#31536c!important;box-shadow:inset 0 1px 0 rgba(255,255,255,.7)!important;backdrop-filter:blur(10px)}
        body:has(> header.hero.heroUnified) .heroTrustItem b{margin:0!important;color:#0b78ad!important;font:900 11px/1 var(--mono)!important}
        body:has(> header.hero.heroUnified) .heroStageList{position:relative!important;z-index:9!important;top:auto!important;right:auto!important;bottom:auto!important;width:100%!important;height:auto!important;display:grid!important;grid-template-rows:repeat(5,82px)!important;gap:10px!important;transform:none!important;align-self:center!important}
        body:has(> header.hero.heroUnified) .heroStageNav{position:relative!important;display:grid!important;grid-template-columns:78px min-content 1fr!important;grid-template-rows:1fr 1fr!important;column-gap:12px!important;align-items:center!important;min-width:0!important;width:100%!important;min-height:82px!important;padding:9px!important;border:1px solid rgba(114,207,255,.22)!important;border-radius:22px!important;background:rgba(255,255,255,.56)!important;color:#18304b!important;text-align:left!important;font-family:var(--sans)!important;box-shadow:0 20px 54px -44px rgba(56,117,158,.42),inset 0 1px 0 rgba(255,255,255,.72)!important;backdrop-filter:blur(15px) saturate(1.08)!important;cursor:pointer!important;transition:transform .2s ease,border-color .2s ease,background-color .2s ease,box-shadow .2s ease!important;overflow:hidden!important}
        body:has(> header.hero.heroUnified) .heroStageNav:before{content:"";position:absolute;left:0;top:0;bottom:0;width:3px;background:linear-gradient(180deg,#72cfff,#ff9fc0);transform:scaleY(0);transform-origin:top;animation:none!important}
        body:has(> header.hero.heroUnified) .heroStageNav.is-active:before{transform:scaleY(1);animation:heroNavProgress 5.6s linear both!important}
        body:has(> header.hero.heroUnified) .heroStageNav img{grid-row:1/3;width:78px;height:64px;border-radius:16px;object-fit:cover;box-shadow:inset 0 0 0 1px rgba(255,255,255,.62);filter:saturate(1.06) contrast(1.02)}
        body:has(> header.hero.heroUnified) .heroStageNav span{grid-column:2;color:#0b78ad!important;font:900 10px/1 var(--mono)!important}
        body:has(> header.hero.heroUnified) .heroStageNav b{grid-column:3;margin:0!important;color:#102238!important;font:850 18px/1 var(--disp)!important;letter-spacing:-.03em!important}
        body:has(> header.hero.heroUnified) .heroStageNav small{grid-column:2/4;margin:0!important;color:#5d6c7c!important;font:800 10px/1.25 var(--mono)!important;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
        body:has(> header.hero.heroUnified) .heroStageNav:hover{transform:translateX(-4px) translateY(-2px)!important;border-color:rgba(255,159,192,.44)!important;background:rgba(255,255,255,.8)!important;box-shadow:0 26px 64px -44px rgba(56,117,158,.55),inset 0 1px 0 rgba(255,255,255,.86)!important}
        body:has(> header.hero.heroUnified) .heroStageNav.is-active{border-color:rgba(114,207,255,.58)!important;background:linear-gradient(135deg,rgba(232,248,255,.92),rgba(255,239,247,.86))!important;box-shadow:0 26px 72px -42px rgba(56,117,158,.5),inset 0 1px 0 rgba(255,255,255,.9)!important}
        @keyframes heroCopyIn{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:translateY(0)}}
        @keyframes heroNavProgress{from{transform:scaleY(0)}to{transform:scaleY(1)}}
        @media(max-width:1180px){body:has(> header.hero.heroUnified) .heroGrid{grid-template-columns:minmax(0,1fr)!important;align-items:end!important;padding-bottom:38px!important}body:has(> header.hero.heroUnified) .heroCopy{max-width:760px!important;margin-left:0!important}body:has(> header.hero.heroUnified) .heroStageList{grid-template-columns:repeat(5,minmax(132px,1fr))!important;grid-template-rows:1fr!important;overflow-x:auto!important;padding-bottom:4px!important}body:has(> header.hero.heroUnified) .heroStageNav{grid-template-columns:1fr!important;grid-template-rows:64px auto auto!important;min-width:132px!important;height:138px!important}body:has(> header.hero.heroUnified) .heroStageNav img{grid-row:auto;width:100%;height:64px}body:has(> header.hero.heroUnified) .heroStageNav span,body:has(> header.hero.heroUnified) .heroStageNav b,body:has(> header.hero.heroUnified) .heroStageNav small{grid-column:1!important}}
        @media(max-width:720px){body:has(> header.hero.heroUnified) .heroGrid{min-height:100svh!important;padding:102px 18px 24px!important;gap:20px!important}body:has(> header.hero.heroUnified) .heroCopy{padding:20px!important;border-radius:24px!important}body:has(> header.hero.heroUnified) .heroTitle{font-size:clamp(38px,11.5vw,54px)!important;line-height:1!important}body:has(> header.hero.heroUnified) .heroSub{font-size:15px!important;line-height:1.64!important}body:has(> header.hero.heroUnified) .heroCtas{flex-direction:column!important}body:has(> header.hero.heroUnified) .heroCtas .btn{width:100%!important}body:has(> header.hero.heroUnified) .heroTrustGrid{display:none!important}body:has(> header.hero.heroUnified) .heroStageShade{background:linear-gradient(180deg,rgba(247,252,255,.92),rgba(247,252,255,.56) 42%,rgba(255,243,248,.96))!important}}
        body:has(> header.hero.heroUnified) .modelLogoMarquee,
        body:has(> header.hero.heroUnified) .modelTypes,
        body:has(> header.hero.heroUnified) .priceProof,
        body:has(> header.hero.heroUnified) .cliQuick,
        body:has(> header.hero.heroUnified) .why,
        body:has(> header.hero.heroUnified) .voiceFaq,
        body:has(> header.hero.heroUnified) .ctaWrap{
          color:var(--fk-ink)!important;
          border-bottom:1px solid rgba(114,207,255,.18)!important;
        }
        body:has(> header.hero.heroUnified) .modelLogoMarquee{
          background:linear-gradient(180deg,#f7fcff 0%,#fff4f9 100%)!important;
        }
        body:has(> header.hero.heroUnified) .modelTypes{
          background:
            radial-gradient(ellipse 48% 42% at 88% 4%,rgba(255,159,192,.22),transparent 62%),
            linear-gradient(180deg,#fff4f9 0%,#f3fbff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .priceProof{
          background:
            radial-gradient(ellipse 42% 36% at 12% 0%,rgba(114,207,255,.24),transparent 64%),
            linear-gradient(180deg,#f3fbff 0%,#fff7fb 100%)!important;
        }
        body:has(> header.hero.heroUnified) .cliQuick{
          background:linear-gradient(180deg,#fff7fb 0%,#f6fcff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .why{
          background:
            radial-gradient(ellipse 38% 36% at 88% 10%,rgba(127,140,255,.16),transparent 62%),
            linear-gradient(180deg,#f6fcff 0%,#fff5fa 100%)!important;
        }
        body:has(> header.hero.heroUnified) .voiceFaq{
          background:linear-gradient(180deg,#fff5fa 0%,#f4fbff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .ctaWrap{
          background:
            radial-gradient(ellipse 52% 42% at 18% 0%,rgba(114,207,255,.24),transparent 64%),
            radial-gradient(ellipse 52% 42% at 86% 8%,rgba(255,159,192,.22),transparent 64%),
            linear-gradient(180deg,#f4fbff 0%,#fff6fb 100%)!important;
        }
        body:has(> header.hero.heroUnified) .kick2{
          color:#0b78ad!important;
        }
        body:has(> header.hero.heroUnified) .sectionTitle{
          color:#0d1b2d!important;
        }
        body:has(> header.hero.heroUnified) .sectionSub{
          color:#54677c!important;
        }
        body:has(> header.hero.heroUnified) .logoPill,
        body:has(> header.hero.heroUnified) .typeCard,
        body:has(> header.hero.heroUnified) .priceModelCard,
        body:has(> header.hero.heroUnified) .quoteCard,
        body:has(> header.hero.heroUnified) .faqCard,
        body:has(> header.hero.heroUnified) .whyCard{
          border-color:rgba(114,207,255,.2)!important;
          background:linear-gradient(180deg,rgba(255,255,255,.82),rgba(245,252,255,.66))!important;
          color:#0d1b2d!important;
          box-shadow:0 26px 80px -62px rgba(56,117,158,.42),inset 0 1px 0 rgba(255,255,255,.84)!important;
        }
        body:has(> header.hero.heroUnified) .logoPill:hover,
        body:has(> header.hero.heroUnified) .typeCard:hover,
        body:has(> header.hero.heroUnified) .priceModelCard:hover,
        body:has(> header.hero.heroUnified) .quoteCard:hover,
        body:has(> header.hero.heroUnified) .faqCard:hover,
        body:has(> header.hero.heroUnified) .whyCard:hover{
          border-color:rgba(255,159,192,.48)!important;
          box-shadow:0 34px 92px -62px rgba(56,117,158,.54),0 18px 42px -34px rgba(255,159,192,.36)!important;
        }
        body:has(> header.hero.heroUnified) .logoPill span,
        body:has(> header.hero.heroUnified) .typeCard p,
        body:has(> header.hero.heroUnified) .priceModelTop small,
        body:has(> header.hero.heroUnified) .priceModelCard p,
        body:has(> header.hero.heroUnified) .faqCard p,
        body:has(> header.hero.heroUnified) .whyCard p{
          color:#54677c!important;
        }
        body:has(> header.hero.heroUnified) .typeCard strong,
        body:has(> header.hero.heroUnified) .priceModelNumbers strong,
        body:has(> header.hero.heroUnified) .quoteCard b{
          color:#0b78ad!important;
        }
        body:has(> header.hero.heroUnified) .typeVisual,
        body:has(> header.hero.heroUnified) .priceModelPoster{
          background:#e8f8ff!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop i{
          background:linear-gradient(135deg,#72cfff,#ffc1d6)!important;
          color:#092235!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span{
          border-color:rgba(114,207,255,.18)!important;
          background:rgba(255,255,255,.58)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers s{
          color:#7f8fa1!important;
        }
        body:has(> header.hero.heroUnified) .priceNote span,
        body:has(> header.hero.heroUnified) .priceNote a,
        body:has(> header.hero.heroUnified) .whyMini span{
          border-color:rgba(114,207,255,.22)!important;
          background:rgba(255,255,255,.62)!important;
          color:#54677c!important;
        }
        body:has(> header.hero.heroUnified) .priceNote a,
        body:has(> header.hero.heroUnified) .whyMini span:hover{
          color:#0b78ad!important;
        }
        body:has(> header.hero.heroUnified) .cliPrimary{
          background:linear-gradient(135deg,#0e8ac5 0%,#5b8dff 100%)!important;
          color:#fff!important;
          box-shadow:0 24px 52px -34px rgba(40,143,205,.68)!important;
        }
        body:has(> header.hero.heroUnified) .cliSecondary{
          background:rgba(255,255,255,.68)!important;
          color:#12334f!important;
          border-color:rgba(114,207,255,.26)!important;
        }
        body:has(> header.hero.heroUnified) .cliQuickPanel{
          border-color:rgba(114,207,255,.26)!important;
          background:linear-gradient(135deg,#102844 0%,#174967 52%,#35507d 100%)!important;
          color:#effaff!important;
          box-shadow:0 36px 88px -58px rgba(35,95,138,.68)!important;
        }
        body:has(> header.hero.heroUnified) .cliQuickPanel:before{
          background-image:linear-gradient(to right,rgba(114,207,255,.12) 1px,transparent 1px),linear-gradient(to bottom,rgba(255,159,192,.1) 1px,transparent 1px)!important;
        }
        body:has(> header.hero.heroUnified) .cliQuickTop,
        body:has(> header.hero.heroUnified) .cliQuickPanel pre span{
          color:#9be1ff!important;
        }
        body:has(> header.hero.heroUnified) .cliQuickPanel pre{
          background:rgba(255,255,255,.08)!important;
          color:#effaff!important;
        }
        body:has(> header.hero.heroUnified) .cliQuickPanel pre em{
          color:#ffd6e5!important;
        }
        body:has(> header.hero.heroUnified) .cliQuickChecks span{
          border-color:rgba(255,255,255,.14)!important;
          background:rgba(255,255,255,.08)!important;
          color:rgba(239,250,255,.82)!important;
        }
        body:has(> header.hero.heroUnified) .whyCard:first-child{
          background:linear-gradient(135deg,#0e8ac5 0%,#7f8cff 54%,#ff9fc0 100%)!important;
          color:#fff!important;
        }
        body:has(> header.hero.heroUnified) .whyCard:first-child p{
          color:rgba(255,255,255,.82)!important;
        }
        body:has(> header.hero.heroUnified) .ctaBanner{
          border-color:rgba(114,207,255,.28)!important;
          background:linear-gradient(135deg,#0d7fb9 0%,#6f8eff 48%,#ff9fc0 100%)!important;
          box-shadow:0 42px 110px -68px rgba(56,117,158,.74)!important;
        }
        body:has(> header.hero.heroUnified) .ctaBanner .kick2{
          color:#dff6ff!important;
        }
        body:has(> header.hero.heroUnified) .ctaBanner h2{
          color:#fff!important;
        }
        body:has(> header.hero.heroUnified) .ctaBanner p{
          color:rgba(255,255,255,.82)!important;
        }
        body:has(> header.hero.heroUnified) .ctaTrust span{
          border-color:rgba(255,255,255,.22)!important;
          background:rgba(255,255,255,.14)!important;
          color:#fff!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot{
          background:linear-gradient(180deg,#eef9ff 0%,#fff2f8 100%)!important;
          color:#0d1b2d!important;
          border-top:1px solid rgba(114,207,255,.22)!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot:before{
          background:radial-gradient(ellipse at 18% 0,rgba(114,207,255,.28),transparent 58%),radial-gradient(ellipse at 88% 0,rgba(255,159,192,.24),transparent 52%)!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot .logo,
        body:has(> header.hero.heroUnified) > .megafoot .word{
          color:#0d1b2d!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot .logo img,
        body:has(> header.hero.heroUnified) > .megafoot .word img{
          background:linear-gradient(135deg,#e5f7ff,#fff0f6)!important;
          border:1px solid rgba(114,207,255,.28)!important;
          border-radius:12px!important;
          padding:4px!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot .brandcol p,
        body:has(> header.hero.heroUnified) > .megafoot .legal,
        body:has(> header.hero.heroUnified) > .megafoot .col h5,
        body:has(> header.hero.heroUnified) > .megafoot .footer-support{
          color:#5d6c7c!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot .col a,
        body:has(> header.hero.heroUnified) > .megafoot .legal a,
        body:has(> header.hero.heroUnified) > .megafoot .footer-support a{
          color:#18304b!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot .col a:hover,
        body:has(> header.hero.heroUnified) > .megafoot .legal a:hover,
        body:has(> header.hero.heroUnified) > .megafoot .footer-support a:hover{
          color:#0b78ad!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot .trustrow a,
        body:has(> header.hero.heroUnified) > .megafoot .trustrow .b{
          color:#18304b!important;
          border-color:rgba(114,207,255,.22)!important;
          background:rgba(255,255,255,.58)!important;
        }
        body:has(> header.hero.heroUnified) > .megafoot .trustrow>span:first-child{
          color:#0b78ad!important;
        }
        body:has(> header.hero.heroUnified)+.stripe .s1{background:#0d1b2d!important}
        body:has(> header.hero.heroUnified)+.stripe .s2{background:#72cfff!important}
        body:has(> header.hero.heroUnified)+.stripe .s3{background:#ff9fc0!important}
        body:has(> header.hero.heroUnified)+.stripe .s4{background:#7f8cff!important}
        body:has(> header.hero.heroUnified)+.stripe .s5{background:#f7fcff!important}
        body:has(> header.hero.heroUnified) .heroUnified{
          min-height:100svh!important;
          background:#f8fcff!important;
        }
        body:has(> header.hero.heroUnified) .heroUnified:before{
          opacity:.52!important;
          background-image:
            linear-gradient(to right,rgba(80,162,210,.055) 1px,transparent 1px),
            linear-gradient(to bottom,rgba(255,159,192,.052) 1px,transparent 1px)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageImage{
          object-position:center right!important;
          filter:saturate(1.02) contrast(.99) brightness(1.03)!important;
          opacity:.94!important;
        }
        body:has(> header.hero.heroUnified) .heroStageShade{
          background:
            linear-gradient(90deg,rgba(248,252,255,.97) 0%,rgba(248,252,255,.9) 30%,rgba(248,252,255,.44) 54%,rgba(255,245,250,.5) 78%,rgba(255,245,250,.88) 100%),
            linear-gradient(180deg,rgba(248,252,255,.22) 0%,rgba(248,252,255,.06) 48%,rgba(248,252,255,.88) 100%)!important;
        }
        body:has(> header.hero.heroUnified) .heroGrid{
          grid-template-columns:minmax(0,1fr) minmax(292px,360px)!important;
          max-width:1400px!important;
          gap:clamp(28px,5vw,72px)!important;
          padding:124px clamp(24px,4vw,64px) 76px!important;
        }
        body:has(> header.hero.heroUnified) .heroCopy{
          max-width:690px!important;
          margin-left:clamp(0px,4vw,72px)!important;
          padding:0!important;
          background:transparent!important;
          box-shadow:none!important;
          backdrop-filter:none!important;
          animation:heroCopyIn .42s cubic-bezier(.16,1,.3,1) both!important;
        }
        body:has(> header.hero.heroUnified) .heroCopy:before{
          display:none!important;
        }
        body:has(> header.hero.heroUnified) .heroCopy .eyebrow{
          padding:9px 14px!important;
          border:1px solid rgba(114,207,255,.26)!important;
          border-radius:999px!important;
          background:rgba(255,255,255,.56)!important;
          color:#0b78ad!important;
          box-shadow:0 18px 40px -32px rgba(56,117,158,.42),inset 0 1px 0 rgba(255,255,255,.82)!important;
          backdrop-filter:blur(10px)!important;
          letter-spacing:.06em!important;
          text-shadow:none!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle{
          max-width:660px!important;
          margin-top:20px!important;
          color:#071d32!important;
          font-size:clamp(50px,6.2vw,88px)!important;
          line-height:.98!important;
          letter-spacing:-.05em!important;
          text-shadow:0 1px 0 rgba(255,255,255,.88),0 22px 56px rgba(100,177,224,.12)!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle span{
          display:block!important;
          margin-top:8px!important;
          background:linear-gradient(96deg,#0b78ad 0%,#5c93ff 48%,#ff7fae 100%)!important;
          -webkit-background-clip:text!important;
          background-clip:text!important;
          color:transparent!important;
        }
        body:has(> header.hero.heroUnified) .heroSub{
          max-width:580px!important;
          margin-top:18px!important;
          color:#3f5a72!important;
          font-size:17px!important;
          line-height:1.72!important;
          text-shadow:0 1px 0 rgba(255,255,255,.82)!important;
        }
        body:has(> header.hero.heroUnified) .heroCtas{
          margin-top:26px!important;
          gap:12px!important;
        }
        body:has(> header.hero.heroUnified) .heroCtas .btn{
          min-height:52px!important;
          padding-inline:22px!important;
          border-radius:999px!important;
          font-weight:850!important;
        }
        body:has(> header.hero.heroUnified) .heroPrimary{
          background:linear-gradient(135deg,#66c9ff 0%,#95dbff 46%,#ffc0d6 100%)!important;
          color:#071d32!important;
          border:1px solid rgba(255,255,255,.86)!important;
          box-shadow:0 24px 56px -30px rgba(40,143,205,.68),inset 0 1px 0 rgba(255,255,255,.82)!important;
        }
        body:has(> header.hero.heroUnified) .heroSecondary,
        body:has(> header.hero.heroUnified) .heroGhost{
          background:rgba(255,255,255,.6)!important;
          color:#14304a!important;
          border:1px solid rgba(114,207,255,.22)!important;
          box-shadow:0 18px 42px -34px rgba(56,117,158,.38),inset 0 1px 0 rgba(255,255,255,.74)!important;
          backdrop-filter:blur(10px)!important;
        }
        body:has(> header.hero.heroUnified) .heroGhost{
          color:#5b6e80!important;
          border-color:rgba(255,159,192,.24)!important;
        }
        body:has(> header.hero.heroUnified) .heroTrustGrid{
          max-width:620px!important;
          display:flex!important;
          flex-wrap:wrap!important;
          gap:9px!important;
          margin-top:22px!important;
        }
        body:has(> header.hero.heroUnified) .heroTrustGrid:empty{
          display:none!important;
        }
        body:has(> header.hero.heroUnified) .heroTrustItem{
          max-width:190px!important;
          padding:9px 12px!important;
          border:1px solid rgba(114,207,255,.2)!important;
          border-radius:999px!important;
          background:rgba(255,255,255,.5)!important;
          color:#14304a!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.74)!important;
          backdrop-filter:blur(10px)!important;
        }
        body:has(> header.hero.heroUnified) .heroTrustItem b{
          display:block!important;
          max-width:100%!important;
          margin:0!important;
          overflow:hidden!important;
          text-overflow:ellipsis!important;
          white-space:nowrap!important;
          color:#0b78ad!important;
          font:900 11px/1 var(--mono)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageList{
          grid-template-rows:repeat(5,88px)!important;
          gap:11px!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav{
          min-height:88px!important;
          border-radius:24px!important;
          background:rgba(255,255,255,.58)!important;
          border-color:rgba(114,207,255,.2)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav img{
          width:82px!important;
          height:68px!important;
          border-radius:18px!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle span{
          color:#0b78ad!important;
          background:none!important;
          -webkit-background-clip:border-box!important;
          background-clip:border-box!important;
          -webkit-text-fill-color:#0b78ad!important;
          text-shadow:
            0 1px 0 rgba(255,255,255,.96),
            0 10px 28px rgba(14,116,180,.18)!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle span::first-letter{
          color:#ff7fae!important;
        }
        body:has(> header.hero.heroUnified) .heroSub{
          max-width:620px!important;
          color:#27435c!important;
          font-weight:650!important;
        }
        body:has(> header.hero.heroUnified) .heroCopy .eyebrow,
        body:has(> header.hero.heroUnified) .heroTrustItem,
        body:has(> header.hero.heroUnified) .heroSecondary,
        body:has(> header.hero.heroUnified) .heroGhost{
          background:rgba(255,255,255,.78)!important;
          border-color:rgba(70,172,230,.34)!important;
          box-shadow:
            0 16px 36px -30px rgba(32,116,178,.46),
            inset 0 1px 0 rgba(255,255,255,.9)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav{
          background:rgba(255,255,255,.78)!important;
          border-color:rgba(70,172,230,.28)!important;
          box-shadow:
            0 22px 52px -42px rgba(32,116,178,.48),
            inset 0 1px 0 rgba(255,255,255,.9)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav.is-active{
          position:relative!important;
          border-color:rgba(30,157,220,.66)!important;
          background:linear-gradient(135deg,rgba(239,250,255,.96),rgba(255,236,246,.94))!important;
          box-shadow:
            0 30px 76px -44px rgba(32,116,178,.58),
            0 0 0 1px rgba(255,255,255,.76) inset!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav:before{
          display:none!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav.is-active:after{
          content:""!important;
          position:absolute!important;
          inset:0!important;
          z-index:0!important;
          padding:2px!important;
          border-radius:inherit!important;
          background:conic-gradient(from -90deg,#30bfff calc(var(--hero-progress, 0) * 1%),rgba(255,143,190,.9) calc(var(--hero-progress, 0) * 1%),rgba(255,255,255,.1) 0)!important;
          -webkit-mask:linear-gradient(#000 0 0) content-box,linear-gradient(#000 0 0)!important;
          -webkit-mask-composite:xor!important;
          mask-composite:exclude!important;
          pointer-events:none!important;
          animation:heroCardBorderProgress 5.6s linear both!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav > *{
          position:relative!important;
          z-index:1!important;
        }
        @keyframes heroCardBorderProgress{
          from{--hero-progress:0}
          to{--hero-progress:100}
        }
        @property --hero-progress{
          syntax:"<number>";
          inherits:false;
          initial-value:0;
        }
        body:has(> header.hero.heroUnified) .heroStageNav b{
          color:#071d32!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav small{
          color:#4a6378!important;
        }
        body:has(> header.hero.heroUnified) .typeVisual,
        body:has(> header.hero.heroUnified) .priceModelPoster{
          background:#f1f9ff!important;
        }
        body:has(> header.hero.heroUnified) .typeCard strong{
          font-size:clamp(18px,1.8vw,24px)!important;
          line-height:1.14!important;
          letter-spacing:-.025em!important;
          word-break:break-word!important;
        }
        body:has(> header.hero.heroUnified) .typeVisual img,
        body:has(> header.hero.heroUnified) .priceModelPoster img{
          filter:saturate(1.03) contrast(.99) brightness(1.02)!important;
        }
        @media(max-width:1180px){
          body:has(> header.hero.heroUnified) .heroGrid{
            grid-template-columns:minmax(0,1fr)!important;
            padding-top:112px!important;
          }
          body:has(> header.hero.heroUnified) .heroCopy{
            margin-left:0!important;
            max-width:740px!important;
          }
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .heroGrid{
            padding:96px 18px 24px!important;
          }
          body:has(> header.hero.heroUnified) .heroTitle{
            font-size:clamp(40px,12vw,56px)!important;
            letter-spacing:-.04em!important;
          }
          body:has(> header.hero.heroUnified) .heroSub{
            font-size:15px!important;
          }
          body:has(> header.hero.heroUnified) .heroStageShade{
            background:linear-gradient(180deg,rgba(248,252,255,.96),rgba(248,252,255,.72) 48%,rgba(255,245,250,.96))!important;
          }
        }
        body:has(> header.hero.heroUnified){
          --fk-bg:#f6fbff;
          --fk-bg-2:#fff6fb;
          --fk-paper:#ffffff;
          --fk-paper-2:#eefaff;
          --fk-ink:#081b31;
          --fk-muted:#4d647a;
          --fk-line:rgba(39,121,170,.14);
          --fk-sky:#67d2ff;
          --fk-azure:#4d8dff;
          --fk-mint:#74e3cf;
          --fk-lavender:#a99bff;
          --fk-rose:#ff96c4;
          --fk-coral:#ffb28a;
          --fk-deep:#08223a;
          --fk-gold:var(--fk-sky);
          --fk-copper:var(--fk-rose);
          --fk-sage:var(--fk-mint);
          background:
            radial-gradient(ellipse 40% 30% at 6% 20%,rgba(103,210,255,.18),transparent 66%),
            radial-gradient(ellipse 44% 32% at 88% 12%,rgba(255,150,196,.16),transparent 66%),
            linear-gradient(180deg,#f6fbff 0%,#fff6fb 100%)!important;
        }
        body:has(> header.hero.heroUnified) .heroUnified{
          background:
            radial-gradient(ellipse 38% 34% at 16% 16%,rgba(103,210,255,.42),transparent 66%),
            radial-gradient(ellipse 34% 32% at 78% 10%,rgba(255,150,196,.32),transparent 64%),
            radial-gradient(ellipse 26% 26% at 58% 82%,rgba(116,227,207,.2),transparent 70%),
            radial-gradient(ellipse 28% 26% at 94% 74%,rgba(169,155,255,.2),transparent 68%),
            linear-gradient(135deg,#f6fbff 0%,#eefaff 43%,#fff6fb 100%)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageShade{
          background:
            linear-gradient(90deg,rgba(246,251,255,.98) 0%,rgba(246,251,255,.91) 29%,rgba(246,251,255,.45) 54%,rgba(255,246,251,.48) 78%,rgba(255,246,251,.9) 100%),
            radial-gradient(ellipse at 26% 22%,rgba(103,210,255,.2),transparent 52%),
            radial-gradient(ellipse at 72% 16%,rgba(255,150,196,.16),transparent 54%)!important;
        }
        body:has(> header.hero.heroUnified) .heroCopy .eyebrow{
          color:#0877ad!important;
          border-color:rgba(103,210,255,.32)!important;
          background:rgba(255,255,255,.72)!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle{
          color:var(--fk-ink)!important;
          letter-spacing:-.042em!important;
          overflow-wrap:anywhere!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle span{
          background:linear-gradient(94deg,var(--fk-azure) 0%,var(--fk-mint) 35%,var(--fk-lavender) 64%,var(--fk-rose) 100%)!important;
          -webkit-background-clip:text!important;
          background-clip:text!important;
          -webkit-text-fill-color:transparent!important;
          color:transparent!important;
          text-shadow:none!important;
        }
        body:has(> header.hero.heroUnified) .heroSub{
          color:#314e68!important;
        }
        body:has(> header.hero.heroUnified) .heroCtas{
          margin-top:24px!important;
        }
        body:has(> header.hero.heroUnified) .heroCtas .btn{
          min-width:188px!important;
          justify-content:center!important;
        }
        body:has(> header.hero.heroUnified) .heroPrimary{
          background:linear-gradient(135deg,var(--fk-sky) 0%,var(--fk-mint) 34%,var(--fk-lavender) 68%,var(--fk-rose) 100%)!important;
          color:#071d32!important;
          border:1px solid rgba(255,255,255,.88)!important;
          box-shadow:
            0 24px 56px -32px rgba(39,121,170,.62),
            0 12px 30px -26px rgba(255,150,196,.46),
            inset 0 1px 0 rgba(255,255,255,.84)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav{
          border-color:rgba(55,148,205,.2)!important;
          background:linear-gradient(135deg,rgba(255,255,255,.82),rgba(239,250,255,.72))!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav:hover{
          border-color:rgba(255,150,196,.48)!important;
          background:linear-gradient(135deg,rgba(255,255,255,.95),rgba(255,241,248,.86))!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav.is-active{
          border-color:rgba(77,141,255,.54)!important;
          background:linear-gradient(135deg,rgba(234,249,255,.96),rgba(244,242,255,.92) 52%,rgba(255,239,247,.94))!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav.is-active:after{
          background:conic-gradient(from -90deg,var(--fk-sky) calc(var(--hero-progress, 0) * .32%),var(--fk-mint) calc(var(--hero-progress, 0) * .58%),var(--fk-lavender) calc(var(--hero-progress, 0) * .82%),var(--fk-rose) calc(var(--hero-progress, 0) * 1%),rgba(255,255,255,.1) 0)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav b{
          color:#081b31!important;
          display:-webkit-box!important;
          -webkit-line-clamp:2!important;
          -webkit-box-orient:vertical!important;
          overflow:hidden!important;
          overflow-wrap:anywhere!important;
          line-height:1.04!important;
          font-size:clamp(14px,1.25vw,18px)!important;
        }
        body:has(> header.hero.heroUnified) .heroStageNav small{
          color:#536a80!important;
        }
        body:has(> header.hero.heroUnified) .modelLogoMarquee{
          background:
            radial-gradient(ellipse 34% 60% at 14% 0%,rgba(116,227,207,.16),transparent 68%),
            radial-gradient(ellipse 34% 60% at 88% 0%,rgba(169,155,255,.16),transparent 68%),
            linear-gradient(180deg,#f6fbff 0%,#fff6fb 100%)!important;
          overflow:hidden!important;
        }
        body:has(> header.hero.heroUnified) .modelLogoInner{
          grid-template-columns:minmax(0,1fr)!important;
          position:relative!important;
          overflow:hidden!important;
          padding-top:16px!important;
          padding-bottom:16px!important;
        }
        body:has(> header.hero.heroUnified) .marqueeLabel{
          position:relative!important;
          z-index:2!important;
          padding:14px 16px!important;
          border:1px solid rgba(55,148,205,.16)!important;
          border-radius:20px!important;
          background:rgba(255,255,255,.72)!important;
          box-shadow:0 18px 44px -36px rgba(39,121,170,.34),inset 0 1px 0 rgba(255,255,255,.84)!important;
        }
        body:has(> header.hero.heroUnified) .marqueeLabel span:first-child{
          color:#0877ad!important;
          letter-spacing:.06em!important;
        }
        body:has(> header.hero.heroUnified) .marqueeLabel span:last-child{
          color:#52697f!important;
        }
        body:has(> header.hero.heroUnified) .logoTrackViewport{
          position:relative!important;
          z-index:1!important;
          min-width:0!important;
          overflow:hidden!important;
          padding:8px 0!important;
          mask-image:linear-gradient(90deg,transparent 0,#000 7%,#000 93%,transparent 100%)!important;
        }
        body:has(> header.hero.heroUnified) .logoTrack{
          will-change:transform!important;
        }
        body:has(> header.hero.heroUnified) .logoPill{
          border-color:rgba(55,148,205,.18)!important;
          background:linear-gradient(135deg,rgba(255,255,255,.86),rgba(239,250,255,.7))!important;
        }
        body:has(> header.hero.heroUnified) .logoPill:hover{
          border-color:rgba(255,150,196,.5)!important;
          box-shadow:
            0 28px 64px -48px rgba(39,121,170,.48),
            0 16px 36px -30px rgba(255,150,196,.34)!important;
        }
        body:has(> header.hero.heroUnified) .typeCard:nth-child(1),
        body:has(> header.hero.heroUnified) .priceModelCard:nth-child(4n+1){
          background:linear-gradient(180deg,rgba(235,249,255,.9),rgba(255,255,255,.74))!important;
        }
        body:has(> header.hero.heroUnified) .typeCard:nth-child(2),
        body:has(> header.hero.heroUnified) .priceModelCard:nth-child(4n+2){
          background:linear-gradient(180deg,rgba(255,241,248,.9),rgba(255,255,255,.74))!important;
        }
        body:has(> header.hero.heroUnified) .typeCard:nth-child(3),
        body:has(> header.hero.heroUnified) .priceModelCard:nth-child(4n+3){
          background:linear-gradient(180deg,rgba(242,241,255,.88),rgba(255,255,255,.74))!important;
        }
        body:has(> header.hero.heroUnified) .typeCard:nth-child(4),
        body:has(> header.hero.heroUnified) .priceModelCard:nth-child(4n){
          background:linear-gradient(180deg,rgba(235,255,250,.86),rgba(255,255,255,.74))!important;
        }
        body:has(> header.hero.heroUnified) .kick2,
        body:has(> header.hero.heroUnified) .typeCard strong,
        body:has(> header.hero.heroUnified) .priceModelNumbers strong,
        body:has(> header.hero.heroUnified) .quoteCard b{
          color:#0877ad!important;
        }
        body:has(> header.hero.heroUnified) .cliPrimary,
        body:has(> header.hero.heroUnified) .ctaBanner{
          background:linear-gradient(135deg,#0877ad 0%,var(--fk-azure) 38%,var(--fk-lavender) 68%,var(--fk-rose) 100%)!important;
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .modelLogoInner{
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .marqueeLabel{
            padding:12px 14px!important;
          }
        }
        body:has(> header.hero.heroUnified) .heroGrid{
          max-width:1480px!important;
          gap:clamp(20px,3.4vw,52px)!important;
          padding-left:clamp(14px,2.2vw,32px)!important;
          padding-right:clamp(14px,2.2vw,32px)!important;
        }
        body:has(> header.hero.heroUnified) .heroCopy{
          margin-left:clamp(0px,1.8vw,28px)!important;
        }
        body:has(> header.hero.heroUnified) .modelLogoInner,
        body:has(> header.hero.heroUnified) .modelTypesIn,
        body:has(> header.hero.heroUnified) .priceProofIn,
        body:has(> header.hero.heroUnified) .cliQuickIn,
        body:has(> header.hero.heroUnified) .whyIn,
        body:has(> header.hero.heroUnified) .voiceFaqIn{
          max-width:1480px!important;
          padding-left:clamp(14px,2.2vw,32px)!important;
          padding-right:clamp(14px,2.2vw,32px)!important;
        }
        body:has(> header.hero.heroUnified) .ctaWrap{
          padding-left:clamp(14px,2.2vw,32px)!important;
          padding-right:clamp(14px,2.2vw,32px)!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle{
          max-width:600px!important;
          font-size:clamp(38px,4.8vw,64px)!important;
          line-height:1.04!important;
          letter-spacing:-.032em!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle span{
          margin-top:8px!important;
          font-size:.42em!important;
          line-height:1.08!important;
          letter-spacing:0!important;
          color:#385a72!important;
          background:none!important;
          -webkit-background-clip:border-box!important;
          background-clip:border-box!important;
          -webkit-text-fill-color:#385a72!important;
          font-weight:760!important;
          text-shadow:0 1px 0 rgba(255,255,255,.88)!important;
        }
        body:has(> header.hero.heroUnified) .heroTitle span::first-letter{
          color:inherit!important;
        }
        body:has(> header.hero.heroUnified) .heroPrimary{
          min-width:164px!important;
          min-height:48px!important;
          padding-inline:20px!important;
          color:#ffffff!important;
          background:linear-gradient(135deg,#0b3b5c 0%,#086b93 100%)!important;
          border:1px solid rgba(116,227,207,.52)!important;
          box-shadow:
            0 18px 42px -30px rgba(8,72,112,.64),
            0 0 0 4px rgba(116,227,207,.1),
            inset 0 1px 0 rgba(255,255,255,.2)!important;
          filter:none!important;
        }
        body:has(> header.hero.heroUnified) .heroPrimary:hover{
          color:#ffffff!important;
          background:linear-gradient(135deg,#08304d 0%,#075f83 100%)!important;
          border-color:rgba(103,210,255,.62)!important;
          box-shadow:
            0 22px 48px -30px rgba(8,72,112,.72),
            0 0 0 5px rgba(103,210,255,.13),
            inset 0 1px 0 rgba(255,255,255,.24)!important;
          transform:translateY(-2px)!important;
          filter:none!important;
        }
        body:has(> header.hero.heroUnified) .heroPrimary:active{
          transform:translateY(0) scale(.98)!important;
        }
        body:has(> header.hero.heroUnified) .modelTypes{
          background:
            radial-gradient(ellipse 34% 34% at 12% 8%,rgba(103,210,255,.18),transparent 64%),
            radial-gradient(ellipse 36% 34% at 90% 18%,rgba(169,155,255,.16),transparent 64%),
            linear-gradient(180deg,#fff6fb 0%,#f5fbff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .apiAppsHead{
          display:grid!important;
          grid-template-columns:minmax(0,.9fr) minmax(360px,.72fr)!important;
          gap:clamp(24px,4vw,64px)!important;
          align-items:end!important;
        }
        body:has(> header.hero.heroUnified) .apiAppsHead .sectionTitle{
          max-width:760px!important;
          font-size:clamp(34px,4.2vw,58px)!important;
          line-height:1.04!important;
          letter-spacing:-.04em!important;
        }
        body:has(> header.hero.heroUnified) .apiAppsHead .sectionSub{
          max-width:520px!important;
          color:#36566f!important;
        }
        body:has(> header.hero.heroUnified) .apiAppsProof{
          display:flex!important;
          flex-wrap:wrap!important;
          gap:8px!important;
          margin-top:16px!important;
        }
        body:has(> header.hero.heroUnified) .apiAppsProof span{
          border:1px solid rgba(55,148,205,.18)!important;
          border-radius:999px!important;
          background:rgba(255,255,255,.66)!important;
          color:#23536f!important;
          padding:8px 10px!important;
          font:800 10px/1 var(--mono)!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.82)!important;
        }
        body:has(> header.hero.heroUnified) .typeGrid{
          display:grid!important;
          grid-template-columns:repeat(2,minmax(0,1fr))!important;
          gap:16px!important;
          margin-top:30px!important;
        }
        body:has(> header.hero.heroUnified) .typeCard{
          min-height:260px!important;
          display:grid!important;
          grid-template-columns:minmax(0,1fr) 210px!important;
          gap:18px!important;
          align-items:stretch!important;
          padding:18px!important;
          border-radius:24px!important;
          overflow:hidden!important;
          text-decoration:none!important;
        }
        body:has(> header.hero.heroUnified) .typeCardCopy{
          min-width:0!important;
          display:flex!important;
          flex-direction:column!important;
          align-items:flex-start!important;
          position:relative!important;
          z-index:1!important;
        }
        body:has(> header.hero.heroUnified) .typeCardCopy>span{
          display:inline-flex!important;
          align-items:center!important;
          min-height:28px!important;
          padding:0 10px!important;
          border-radius:999px!important;
          background:rgba(255,255,255,.7)!important;
          color:#0877ad!important;
          font:900 10px/1 var(--mono)!important;
          letter-spacing:.04em!important;
          box-shadow:inset 0 0 0 1px rgba(55,148,205,.14)!important;
        }
        body:has(> header.hero.heroUnified) .typeCard b{
          margin-top:18px!important;
          color:#081b31!important;
          font:850 clamp(24px,2.6vw,34px)/1.02 var(--disp)!important;
          letter-spacing:-.035em!important;
        }
        body:has(> header.hero.heroUnified) .typeCard p{
          max-width:430px!important;
          margin-top:12px!important;
          color:#4a6378!important;
          font-size:13.5px!important;
          line-height:1.62!important;
        }
        body:has(> header.hero.heroUnified) .typeModels{
          display:flex!important;
          flex-wrap:wrap!important;
          gap:7px!important;
          margin-top:auto!important;
          padding-top:20px!important;
        }
        body:has(> header.hero.heroUnified) .typeModels i{
          max-width:160px!important;
          overflow:hidden!important;
          text-overflow:ellipsis!important;
          white-space:nowrap!important;
          border:1px solid rgba(55,148,205,.16)!important;
          border-radius:999px!important;
          background:rgba(255,255,255,.62)!important;
          color:#305872!important;
          padding:7px 9px!important;
          font:800 10px/1 var(--mono)!important;
          font-style:normal!important;
        }
        body:has(> header.hero.heroUnified) .typeCard strong{
          margin-top:14px!important;
          color:#0877ad!important;
          font:850 13px/1 var(--sans)!important;
          letter-spacing:0!important;
        }
        body:has(> header.hero.heroUnified) .typeCard strong:after{
          content:" →";
        }
        body:has(> header.hero.heroUnified) .typeVisual{
          height:auto!important;
          min-height:100%!important;
          margin:0!important;
          border-radius:18px!important;
          background:#eefaff!important;
        }
        body:has(> header.hero.heroUnified) .typeVisual img{
          width:100%!important;
          height:100%!important;
          object-fit:cover!important;
          filter:saturate(1.02) contrast(.99) brightness(1.02)!important;
        }
        body:has(> header.hero.heroUnified) .typeCard:hover{
          transform:translateY(-6px)!important;
          border-color:rgba(77,141,255,.34)!important;
          box-shadow:
            0 34px 86px -62px rgba(39,121,170,.46),
            0 14px 36px -30px rgba(255,150,196,.26)!important;
        }
        body:has(> header.hero.heroUnified) .typeCard-video{
          background:linear-gradient(135deg,rgba(235,249,255,.94),rgba(255,255,255,.72))!important;
        }
        body:has(> header.hero.heroUnified) .typeCard-image{
          background:linear-gradient(135deg,rgba(255,241,248,.94),rgba(255,255,255,.72))!important;
        }
        body:has(> header.hero.heroUnified) .typeCard-text{
          background:linear-gradient(135deg,rgba(242,241,255,.9),rgba(255,255,255,.72))!important;
        }
        body:has(> header.hero.heroUnified) .typeCard-audio{
          background:linear-gradient(135deg,rgba(235,255,250,.9),rgba(255,255,255,.72))!important;
        }
        @media(max-width:980px){
          body:has(> header.hero.heroUnified) .apiAppsHead,
          body:has(> header.hero.heroUnified) .typeGrid{
            grid-template-columns:1fr!important;
          }
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .typeCard{
            grid-template-columns:1fr!important;
            min-height:0!important;
          }
          body:has(> header.hero.heroUnified) .typeVisual{
            min-height:170px!important;
            order:-1!important;
          }
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .heroTitle{
            font-size:clamp(34px,10vw,48px)!important;
            line-height:1.08!important;
          }
          body:has(> header.hero.heroUnified) .heroTitle span{
            font-size:.48em!important;
          }
          body:has(> header.hero.heroUnified) .heroGrid,
          body:has(> header.hero.heroUnified) .modelLogoInner,
          body:has(> header.hero.heroUnified) .modelTypesIn,
          body:has(> header.hero.heroUnified) .priceProofIn,
          body:has(> header.hero.heroUnified) .cliQuickIn,
          body:has(> header.hero.heroUnified) .whyIn,
          body:has(> header.hero.heroUnified) .voiceFaqIn,
          body:has(> header.hero.heroUnified) .ctaWrap{
            padding-left:14px!important;
            padding-right:14px!important;
          }
          body:has(> header.hero.heroUnified) .heroCtas .btn{
            width:auto!important;
            min-width:148px!important;
          }
        }
        body:has(> header.hero.heroUnified){
          --fk-bg:#f2f9ff!important;
          --fk-bg-2:#fff3f8!important;
          --fk-paper:#ffffff!important;
          --fk-paper-2:#eaf8ff!important;
          --fk-ink:#061a2c!important;
          --fk-muted:#36546b!important;
          --fk-line:rgba(4,74,113,.2)!important;
          --fk-sky:#2db8f5!important;
          --fk-azure:#2469d9!important;
          --fk-mint:#0a9c86!important;
          --fk-lavender:#7568e8!important;
          --fk-rose:#d73679!important;
          --fk-coral:#ee815b!important;
          --fk-deep:#052a44!important;
          background:
            radial-gradient(ellipse 40% 30% at 6% 18%,rgba(45,184,245,.16),transparent 68%),
            radial-gradient(ellipse 38% 28% at 92% 10%,rgba(215,54,121,.13),transparent 66%),
            linear-gradient(180deg,#f2f9ff 0%,#fff3f8 100%)!important;
        }
        body:has(> header.hero.heroUnified) .kick2{
          color:#006b9a!important;
          letter-spacing:.08em!important;
        }
        body:has(> header.hero.heroUnified) .sectionTitle{
          color:#061a2c!important;
        }
        body:has(> header.hero.heroUnified) .sectionSub{
          color:#2d4b63!important;
          font-weight:620!important;
        }
        body:has(> header.hero.heroUnified) .priceProofHead{
          grid-template-columns:minmax(0,.86fr) minmax(360px,.7fr)!important;
          align-items:end!important;
        }
        body:has(> header.hero.heroUnified) .priceProofHead .sectionTitle{
          max-width:650px!important;
        }
        body:has(> header.hero.heroUnified) .priceProofHead .sectionSub{
          max-width:570px!important;
        }
        body:has(> header.hero.heroUnified) .priceModelGrid{
          grid-template-columns:repeat(4,minmax(0,1fr))!important;
          gap:18px!important;
          align-items:stretch!important;
          margin-top:32px!important;
        }
        body:has(> header.hero.heroUnified) .priceModelCard{
          display:flex!important;
          flex-direction:column!important;
          min-height:408px!important;
          overflow:hidden!important;
          border:1px solid rgba(4,74,113,.22)!important;
          border-radius:24px!important;
          background:linear-gradient(180deg,rgba(255,255,255,.95),rgba(239,250,255,.78))!important;
          box-shadow:0 28px 82px -62px rgba(4,52,82,.48),inset 0 1px 0 rgba(255,255,255,.92)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelCard:hover{
          transform:translateY(-7px)!important;
          border-color:rgba(215,54,121,.42)!important;
          box-shadow:0 38px 96px -62px rgba(4,52,82,.58),0 18px 44px -34px rgba(215,54,121,.24)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelPoster{
          height:auto!important;
          aspect-ratio:16/10!important;
          background:#dff5ff!important;
        }
        body:has(> header.hero.heroUnified) .priceModelPoster img{
          height:100%!important;
          object-fit:cover!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop{
          min-height:76px!important;
          padding:18px 18px 0!important;
          align-items:flex-start!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop b{
          max-width:none!important;
          display:-webkit-box!important;
          -webkit-line-clamp:2!important;
          -webkit-box-orient:vertical!important;
          overflow:hidden!important;
          white-space:normal!important;
          color:#061a2c!important;
          font-size:16px!important;
          line-height:1.18!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop small{
          color:#36546b!important;
          font-weight:760!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop i{
          flex:0 0 auto!important;
          min-width:48px!important;
          height:30px!important;
          background:#052a44!important;
          color:#fff!important;
          border:1px solid rgba(45,184,245,.35)!important;
          box-shadow:0 10px 24px -16px rgba(5,42,68,.58)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers{
          grid-template-columns:1fr 1fr!important;
          gap:10px!important;
          padding:15px 18px 0!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span{
          min-height:82px!important;
          padding:12px!important;
          border-color:rgba(4,74,113,.18)!important;
          background:#fff!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers small{
          color:#516a7c!important;
          font-size:9px!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers s{
          color:#72889a!important;
          font-size:13px!important;
          line-height:1.25!important;
          word-break:break-word!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers strong{
          color:#005f89!important;
          font:950 clamp(19px,1.55vw,26px)/1.04 var(--mono)!important;
          letter-spacing:-.03em!important;
          word-break:break-word!important;
          overflow-wrap:anywhere!important;
        }
        body:has(> header.hero.heroUnified) .priceModelCard p{
          min-height:52px!important;
          margin-top:auto!important;
          padding:13px 18px 18px!important;
          color:#31556b!important;
          font-size:12.5px!important;
          font-weight:700!important;
        }
        body:has(> header.hero.heroUnified) .priceNote span,
        body:has(> header.hero.heroUnified) .priceNote a{
          border-color:rgba(4,74,113,.22)!important;
          background:#fff!important;
          color:#214a63!important;
        }
        body:has(> header.hero.heroUnified) .priceNote a{
          color:#006b9a!important;
          font-weight:900!important;
        }
        body:has(> header.hero.heroUnified) .cliQuickIn{
          grid-template-columns:minmax(0,.76fr) minmax(460px,1.12fr)!important;
          align-items:center!important;
        }
        body:has(> header.hero.heroUnified) .cliPrimary{
          background:#052a44!important;
          color:#fff!important;
          border:1px solid rgba(45,184,245,.36)!important;
          box-shadow:0 22px 50px -32px rgba(5,42,68,.66)!important;
        }
        body:has(> header.hero.heroUnified) .cliSecondary{
          background:#fff!important;
          color:#052a44!important;
          border-color:rgba(4,74,113,.24)!important;
        }
        body:has(> header.hero.heroUnified) .cliStepperPanel{
          padding:22px!important;
          border:1px solid rgba(4,74,113,.28)!important;
          background:
            radial-gradient(ellipse 46% 42% at 86% 0%,rgba(45,184,245,.24),transparent 62%),
            linear-gradient(135deg,#052a44 0%,#07476a 58%,#0d635f 100%)!important;
          box-shadow:0 38px 96px -64px rgba(5,42,68,.72)!important;
        }
        body:has(> header.hero.heroUnified) .cliSteps{
          position:relative!important;
          z-index:1!important;
          display:grid!important;
          gap:12px!important;
          margin-top:22px!important;
        }
        body:has(> header.hero.heroUnified) .cliSteps:before{
          content:""!important;
          position:absolute!important;
          left:25px!important;
          top:26px!important;
          bottom:26px!important;
          width:2px!important;
          background:linear-gradient(180deg,#2db8f5,#0a9c86,#d73679)!important;
          opacity:.74!important;
        }
        body:has(> header.hero.heroUnified) .cliStep{
          position:relative!important;
          display:grid!important;
          grid-template-columns:52px minmax(0,1fr)!important;
          gap:13px!important;
          align-items:start!important;
        }
        body:has(> header.hero.heroUnified) .cliStepNo{
          position:relative!important;
          z-index:1!important;
          display:grid!important;
          place-items:center!important;
          width:52px!important;
          height:52px!important;
          border-radius:18px!important;
          background:#ffffff!important;
          color:#052a44!important;
          border:1px solid rgba(45,184,245,.42)!important;
          font:950 13px/1 var(--mono)!important;
          box-shadow:0 14px 34px -24px rgba(0,0,0,.42)!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody{
          min-width:0!important;
          min-height:118px!important;
          padding:16px!important;
          border:1px solid rgba(255,255,255,.14)!important;
          border-radius:20px!important;
          background:rgba(255,255,255,.1)!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.12)!important;
          transition:transform .2s ease,background-color .2s ease,border-color .2s ease!important;
        }
        body:has(> header.hero.heroUnified) .cliStep:hover .cliStepBody{
          transform:translateX(4px)!important;
          background:rgba(255,255,255,.16)!important;
          border-color:rgba(45,184,245,.34)!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody b{
          display:block!important;
          color:#fff!important;
          font:880 18px/1.1 var(--disp)!important;
          letter-spacing:-.02em!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody p{
          margin-top:8px!important;
          color:rgba(239,250,255,.82)!important;
          font-size:13px!important;
          line-height:1.55!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody code{
          display:block!important;
          margin-top:12px!important;
          padding:10px 12px!important;
          border-radius:13px!important;
          background:rgba(0,0,0,.22)!important;
          color:#b9f0ff!important;
          font:800 12px/1.35 var(--mono)!important;
          white-space:normal!important;
          word-break:break-word!important;
        }
        body:has(> header.hero.heroUnified) .whyIn{
          display:block!important;
        }
        body:has(> header.hero.heroUnified) .whyGrid{
          display:grid!important;
          grid-template-columns:repeat(4,minmax(0,1fr))!important;
          gap:16px!important;
          margin-top:32px!important;
        }
        body:has(> header.hero.heroUnified) .whyHead{
          max-width:940px!important;
        }
        body:has(> header.hero.heroUnified) .whyHead .sectionTitle{
          max-width:900px!important;
          font-size:clamp(34px,4.1vw,60px)!important;
          line-height:1.04!important;
          letter-spacing:-.04em!important;
        }
        body:has(> header.hero.heroUnified) .whyCard{
          position:relative!important;
          display:flex!important;
          flex-direction:column!important;
          min-width:0!important;
          min-height:292px!important;
          padding:22px!important;
          border-color:rgba(4,74,113,.22)!important;
          overflow:hidden!important;
          background:
            radial-gradient(ellipse 70% 50% at 96% 0%,rgba(45,184,245,.13),transparent 62%),
            linear-gradient(180deg,#ffffff 0%,#eefaff 100%)!important;
          animation:whyCardIn .72s cubic-bezier(.16,1,.3,1) both!important;
          animation-delay:calc(var(--why-index, 0) * 90ms)!important;
          transition:transform .22s ease,border-color .22s ease,box-shadow .22s ease!important;
        }
        body:has(> header.hero.heroUnified) .whyCard:after{
          content:""!important;
          position:absolute!important;
          left:22px!important;
          right:22px!important;
          bottom:0!important;
          height:3px!important;
          border-radius:999px 999px 0 0!important;
          background:linear-gradient(90deg,#2db8f5,#0a9c86,#d73679)!important;
          transform:scaleX(.18)!important;
          transform-origin:left!important;
          opacity:.55!important;
          transition:transform .28s ease,opacity .28s ease!important;
        }
        body:has(> header.hero.heroUnified) .whyCard:hover{
          transform:translateY(-6px)!important;
          border-color:rgba(45,184,245,.34)!important;
          box-shadow:0 32px 88px -62px rgba(5,42,68,.5)!important;
        }
        body:has(> header.hero.heroUnified) .whyCard:hover:after{
          transform:scaleX(1)!important;
          opacity:1!important;
        }
        body:has(> header.hero.heroUnified) .whyMetric{
          width:max-content!important;
          max-width:100%!important;
          padding:7px 10px!important;
          border-radius:999px!important;
          background:#052a44!important;
          color:#fff!important;
          font:900 10px/1 var(--mono)!important;
        }
        body:has(> header.hero.heroUnified) .whyCard:first-child{
          background:#fff!important;
          color:#061a2c!important;
        }
        body:has(> header.hero.heroUnified) .whyCard h3{
          margin-top:18px!important;
          color:#061a2c!important;
          font-size:clamp(20px,1.65vw,26px)!important;
          line-height:1.12!important;
          letter-spacing:-.025em!important;
          overflow-wrap:anywhere!important;
        }
        body:has(> header.hero.heroUnified) .whyCard p,
        body:has(> header.hero.heroUnified) .whyCard:first-child p{
          color:#36546b!important;
          font-size:13.5px!important;
          line-height:1.62!important;
        }
        body:has(> header.hero.heroUnified) .whyMini{
          margin-top:auto!important;
          padding-top:16px!important;
          display:flex!important;
          flex-wrap:wrap!important;
          gap:7px!important;
        }
        body:has(> header.hero.heroUnified) .whyMini span{
          border-color:rgba(4,74,113,.2)!important;
          background:#eefaff!important;
          color:#0b4a6a!important;
        }
        body:has(> header.hero.heroUnified) .voiceShowcase{
          display:grid!important;
          grid-template-columns:minmax(330px,.72fr) minmax(0,1.28fr)!important;
          gap:18px!important;
          margin-top:32px!important;
          align-items:stretch!important;
        }
        body:has(> header.hero.heroUnified) .voiceBoard{
          position:relative!important;
          overflow:hidden!important;
          min-height:260px!important;
          padding:24px!important;
          border:1px solid rgba(4,74,113,.26)!important;
          border-radius:26px!important;
          background:
            radial-gradient(ellipse 56% 50% at 92% 0%,rgba(45,184,245,.28),transparent 66%),
            linear-gradient(135deg,#052a44 0%,#07476a 58%,#0b7465 100%)!important;
          color:#fff!important;
          box-shadow:0 34px 90px -64px rgba(5,42,68,.74)!important;
        }
        body:has(> header.hero.heroUnified) .voiceBoard:after{
          content:""!important;
          position:absolute!important;
          inset:-40% auto -40% -30%!important;
          width:42%!important;
          background:linear-gradient(90deg,transparent,rgba(255,255,255,.18),transparent)!important;
          transform:skewX(-18deg) translateX(-120%)!important;
          animation:voiceBoardSheen 7s ease-in-out infinite!important;
          pointer-events:none!important;
        }
        body:has(> header.hero.heroUnified) .voiceBoard span{
          color:#b9f0ff!important;
          font:900 11px/1 var(--mono)!important;
          letter-spacing:.08em!important;
        }
        body:has(> header.hero.heroUnified) .voiceBoard strong{
          display:block!important;
          margin-top:18px!important;
          max-width:430px!important;
          color:#fff!important;
          font:900 clamp(30px,3.4vw,48px)/1.02 var(--disp)!important;
          letter-spacing:-.045em!important;
        }
        body:has(> header.hero.heroUnified) .voiceBoard p{
          margin-top:14px!important;
          max-width:430px!important;
          color:rgba(239,250,255,.78)!important;
          line-height:1.65!important;
        }
        body:has(> header.hero.heroUnified) .voiceSignals{
          display:flex!important;
          flex-wrap:wrap!important;
          gap:8px!important;
          margin-top:22px!important;
        }
        body:has(> header.hero.heroUnified) .voiceSignals i{
          border:1px solid rgba(255,255,255,.18)!important;
          border-radius:999px!important;
          background:rgba(255,255,255,.1)!important;
          color:#dff8ff!important;
          padding:8px 10px!important;
          font:850 10px/1 var(--mono)!important;
          font-style:normal!important;
        }
        body:has(> header.hero.heroUnified) .voiceWall{
          position:relative!important;
          overflow:hidden!important;
          min-height:260px!important;
          padding:4px!important;
          mask-image:linear-gradient(90deg,transparent 0,#000 8%,#000 92%,transparent 100%)!important;
        }
        body:has(> header.hero.heroUnified) .voiceTrack{
          display:flex!important;
          width:max-content!important;
          gap:14px!important;
          animation:voiceMove 24s linear infinite!important;
        }
        body:has(> header.hero.heroUnified) .voiceWall:hover .voiceTrack{
          animation-play-state:paused!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble{
          width:min(360px,72vw)!important;
          min-height:238px!important;
          display:flex!important;
          flex-direction:column!important;
          justify-content:space-between!important;
          border-color:rgba(4,74,113,.22)!important;
          background:linear-gradient(180deg,#fff,#edf9ff)!important;
          padding:22px!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble b{
          margin:0!important;
          color:#006b9a!important;
          font:950 12px/1 var(--mono)!important;
          letter-spacing:.06em!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble p{
          color:#0b263e!important;
          font-size:18px!important;
          line-height:1.58!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble small{
          color:#4c667b!important;
          font:850 11px/1 var(--mono)!important;
        }
        @keyframes voiceMove{
          to{transform:translateX(-50%)}
        }
        body:has(> header.hero.heroUnified) .faqGrid{
          margin-top:18px!important;
          grid-template-columns:repeat(4,minmax(0,1fr))!important;
        }
        body:has(> header.hero.heroUnified) .faqCard{
          min-height:150px!important;
          border-color:rgba(4,74,113,.2)!important;
          background:#fff!important;
        }
        body:has(> header.hero.heroUnified) .faqCard h3{
          color:#061a2c!important;
          font-size:15px!important;
          line-height:1.35!important;
        }
        body:has(> header.hero.heroUnified) .faqCard p{
          color:#36546b!important;
          font-weight:620!important;
        }
        body:has(> header.hero.heroUnified) .ctaWrap{
          padding-top:82px!important;
          padding-bottom:82px!important;
          background:
            radial-gradient(ellipse 42% 36% at 12% 8%,rgba(45,184,245,.16),transparent 66%),
            radial-gradient(ellipse 42% 36% at 88% 6%,rgba(215,54,121,.13),transparent 66%),
            linear-gradient(180deg,#f2f9ff 0%,#fff3f8 100%)!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel{
          max-width:1480px!important;
          grid-template-columns:minmax(0,.88fr) minmax(420px,1.12fr)!important;
          gap:28px!important;
          padding:28px!important;
          border:1px solid rgba(4,74,113,.24)!important;
          border-radius:32px!important;
          background:linear-gradient(135deg,rgba(255,255,255,.96),rgba(232,248,255,.8) 48%,rgba(255,239,247,.78))!important;
          box-shadow:0 38px 110px -72px rgba(4,52,82,.52),inset 0 1px 0 rgba(255,255,255,.94)!important;
        }
        body:has(> header.hero.heroUnified) .btcCopy{
          padding:18px!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .kick2{
          color:#006b9a!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel h2{
          max-width:620px!important;
          color:#061a2c!important;
          font-size:clamp(36px,4.8vw,68px)!important;
          line-height:1!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel p{
          max-width:560px!important;
          color:#2d4b63!important;
          font-weight:640!important;
        }
        body:has(> header.hero.heroUnified) .btcMetrics{
          display:grid!important;
          grid-template-columns:repeat(3,minmax(0,1fr))!important;
          gap:10px!important;
          margin:22px 0 24px!important;
          max-width:560px!important;
        }
        body:has(> header.hero.heroUnified) .btcMetrics span{
          min-width:0!important;
          padding:14px!important;
          border:1px solid rgba(4,74,113,.2)!important;
          border-radius:18px!important;
          background:#fff!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.9)!important;
        }
        body:has(> header.hero.heroUnified) .btcMetrics b{
          display:block!important;
          color:#052a44!important;
          font:950 clamp(17px,1.7vw,24px)/1 var(--mono)!important;
        }
        body:has(> header.hero.heroUnified) .btcMetrics small{
          display:block!important;
          margin-top:8px!important;
          color:#36546b!important;
          font-size:12px!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .btn.white{
          background:#052a44!important;
          color:#fff!important;
          border:1px solid rgba(45,184,245,.38)!important;
          box-shadow:0 22px 48px -32px rgba(5,42,68,.62)!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .btn.black{
          background:#fff!important;
          color:#052a44!important;
          border:1px solid rgba(4,74,113,.24)!important;
        }
        body:has(> header.hero.heroUnified) .btcMedia{
          position:relative!important;
          min-width:0!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .ctaBillboard{
          aspect-ratio:16/10!important;
          border:1px solid rgba(4,74,113,.2)!important;
          border-radius:26px!important;
          background:#e7f7ff!important;
          box-shadow:0 30px 78px -56px rgba(4,52,82,.46)!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .ctaBillboard img{
          object-fit:cover!important;
          object-position:center!important;
          filter:saturate(1.02) contrast(1.02) brightness(1.03)!important;
        }
        body:has(> header.hero.heroUnified) .btcFloatNote{
          position:absolute!important;
          left:18px!important;
          bottom:54px!important;
          max-width:min(360px,70%)!important;
          padding:14px 16px!important;
          border:1px solid rgba(255,255,255,.42)!important;
          border-radius:18px!important;
          background:rgba(5,42,68,.82)!important;
          color:#fff!important;
          box-shadow:0 22px 48px -32px rgba(5,42,68,.7)!important;
          backdrop-filter:blur(14px)!important;
        }
        body:has(> header.hero.heroUnified) .btcFloatNote b{
          display:block!important;
          font:900 14px/1.2 var(--sans)!important;
        }
        body:has(> header.hero.heroUnified) .btcFloatNote span{
          display:block!important;
          margin-top:6px!important;
          color:rgba(239,250,255,.78)!important;
          font-size:12px!important;
          line-height:1.4!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .ctaTrust{
          margin-top:12px!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .ctaTrust span{
          border-color:rgba(4,74,113,.2)!important;
          background:#fff!important;
          color:#214a63!important;
        }
        @media(max-width:1180px){
          body:has(> header.hero.heroUnified) .priceModelGrid,
          body:has(> header.hero.heroUnified) .faqGrid{
            grid-template-columns:repeat(2,minmax(0,1fr))!important;
          }
          body:has(> header.hero.heroUnified) .cliQuickIn,
          body:has(> header.hero.heroUnified) .whyIn,
          body:has(> header.hero.heroUnified) .voiceShowcase,
          body:has(> header.hero.heroUnified) .btcPanel{
            grid-template-columns:1fr!important;
          }
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .priceProofHead,
          body:has(> header.hero.heroUnified) .priceModelGrid,
          body:has(> header.hero.heroUnified) .whyGrid,
          body:has(> header.hero.heroUnified) .faqGrid,
          body:has(> header.hero.heroUnified) .btcMetrics{
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .priceModelCard{
            min-height:0!important;
          }
          body:has(> header.hero.heroUnified) .cliStepperPanel,
          body:has(> header.hero.heroUnified) .btcPanel{
            padding:16px!important;
            border-radius:24px!important;
          }
          body:has(> header.hero.heroUnified) .cliStep{
            grid-template-columns:46px minmax(0,1fr)!important;
          }
          body:has(> header.hero.heroUnified) .cliStepNo{
            width:46px!important;
            height:46px!important;
            border-radius:15px!important;
          }
          body:has(> header.hero.heroUnified) .cliSteps:before{
            left:22px!important;
          }
          body:has(> header.hero.heroUnified) .voiceShowcase{
            margin-top:24px!important;
          }
          body:has(> header.hero.heroUnified) .voiceBoard strong{
            font-size:clamp(28px,8vw,38px)!important;
          }
          body:has(> header.hero.heroUnified) .voiceWall{
            mask-image:none!important;
          }
          body:has(> header.hero.heroUnified) .btcFloatNote{
            position:static!important;
            max-width:none!important;
            margin-top:10px!important;
            background:#052a44!important;
          }
        }
        body:has(> header.hero.heroUnified) .modelTypes{
          background:
            radial-gradient(ellipse 42% 34% at 10% 6%,rgba(45,184,245,.18),transparent 66%),
            radial-gradient(ellipse 36% 34% at 88% 12%,rgba(215,54,121,.12),transparent 66%),
            linear-gradient(180deg,#fff3f8 0%,#edf8ff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .modelTypesIn{
          padding-top:84px!important;
          padding-bottom:92px!important;
        }
        body:has(> header.hero.heroUnified) .apiAppsHead{
          grid-template-columns:minmax(0,.78fr) minmax(420px,.62fr)!important;
          align-items:end!important;
        }
        body:has(> header.hero.heroUnified) .apiAppsHead .sectionTitle{
          max-width:780px!important;
          font-size:clamp(36px,4.4vw,62px)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid{
          display:grid!important;
          grid-template-columns:minmax(0,1.1fr) minmax(260px,.7fr) minmax(260px,.7fr)!important;
          grid-template-rows:repeat(2,minmax(244px,auto))!important;
          gap:16px!important;
          align-items:stretch!important;
          margin-top:34px!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard{
          min-height:0!important;
          position:relative!important;
          display:grid!important;
          grid-template-columns:minmax(0,1fr)!important;
          gap:0!important;
          padding:16px!important;
          border-radius:26px!important;
          overflow:hidden!important;
          border:1px solid rgba(4,74,113,.22)!important;
          background:#fff!important;
          isolation:isolate!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:before{
          content:""!important;
          position:absolute!important;
          inset:0!important;
          z-index:-1!important;
          opacity:.9!important;
          background:
            radial-gradient(ellipse 70% 50% at 88% 0%,rgba(45,184,245,.14),transparent 64%),
            linear-gradient(180deg,rgba(255,255,255,.96),rgba(239,250,255,.72))!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1){
          grid-row:1/3!important;
          grid-template-columns:minmax(0,.76fr) minmax(310px,.78fr)!important;
          gap:18px!important;
          min-height:510px!important;
          padding:18px!important;
          background:linear-gradient(135deg,#ffffff 0%,#eaf8ff 56%,#fff0f7 100%)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1) .typeVisual{
          order:2!important;
          min-height:100%!important;
          height:auto!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:not(:first-child){
          grid-template-columns:132px minmax(0,1fr)!important;
          gap:14px!important;
          align-items:stretch!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:not(:first-child) .typeVisual{
          order:-1!important;
          min-height:100%!important;
          height:auto!important;
          border-radius:20px!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCardCopy{
          justify-content:flex-start!important;
          min-height:0!important;
          padding:2px!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCardCopy>span{
          color:#fff!important;
          background:#052a44!important;
          box-shadow:none!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard b{
          margin-top:16px!important;
          color:#061a2c!important;
          font-size:clamp(22px,2.2vw,34px)!important;
          line-height:1.04!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:first-child b{
          font-size:clamp(34px,4.2vw,58px)!important;
          max-width:440px!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard p{
          max-width:470px!important;
          color:#2d4b63!important;
          font-weight:640!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeModels{
          margin-top:auto!important;
          padding-top:18px!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeModels i{
          border-color:rgba(4,74,113,.2)!important;
          background:#fff!important;
          color:#214a63!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard strong{
          width:max-content!important;
          max-width:100%!important;
          margin-top:14px!important;
          padding:10px 13px!important;
          border-radius:999px!important;
          background:#052a44!important;
          color:#fff!important;
          font:850 12px/1 var(--sans)!important;
          box-shadow:0 14px 30px -24px rgba(5,42,68,.52)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeVisual{
          margin:0!important;
          border:1px solid rgba(4,74,113,.16)!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.72)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeVisual{
          display:grid!important;
          place-items:center!important;
          background:
            radial-gradient(ellipse 62% 48% at 50% 0%,rgba(45,184,245,.16),transparent 66%),
            linear-gradient(180deg,#f4fbff,#fff7fb)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeVisual img{
          width:100%!important;
          height:100%!important;
          object-fit:cover!important;
          object-position:center!important;
          padding:0!important;
          filter:saturate(1.03) contrast(.99) brightness(1.02)!important;
          transform:none!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:hover .typeVisual img{
          transform:scale(1.025)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1) .typeVisual img{
          padding:0!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:hover{
          transform:translateY(-7px)!important;
          border-color:rgba(215,54,121,.4)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:hover strong{
          background:#063a5c!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerStack{
          display:grid!important;
          gap:16px!important;
          margin-top:34px!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerViewport{
          position:relative!important;
          overflow:hidden!important;
          padding:3px 0!important;
          mask-image:linear-gradient(90deg,transparent 0,#000 7%,#000 93%,transparent 100%)!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerTrack{
          display:flex!important;
          width:max-content!important;
          gap:16px!important;
          animation:priceTickerMove 34s linear infinite!important;
          will-change:transform!important;
        }
        body:has(> header.hero.heroUnified) .priceTicker-1 .priceTickerTrack{
          animation-name:priceTickerMoveReverse!important;
          animation-duration:40s!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerViewport:hover .priceTickerTrack{
          animation-play-state:paused!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard{
          flex:0 0 clamp(286px,24vw,360px)!important;
          width:clamp(286px,24vw,360px)!important;
          min-height:368px!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard .priceModelPoster{
          aspect-ratio:16/9!important;
        }
        @keyframes priceTickerMove{
          to{transform:translateX(calc(-50% - 8px))}
        }
        @keyframes priceTickerMoveReverse{
          from{transform:translateX(calc(-50% - 8px))}
          to{transform:translateX(0)}
        }
        @media(max-width:1180px){
          body:has(> header.hero.heroUnified) .modelFlowGrid{
            grid-template-columns:repeat(2,minmax(0,1fr))!important;
            grid-template-rows:auto!important;
          }
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1){
            grid-column:1/3!important;
            grid-row:auto!important;
          }
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:not(:first-child){
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:not(:first-child) .typeVisual{
            min-height:170px!important;
            order:2!important;
          }
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .apiAppsHead,
          body:has(> header.hero.heroUnified) .modelFlowGrid,
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1){
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1){
            grid-column:auto!important;
            min-height:0!important;
          }
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1) .typeVisual,
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:not(:first-child) .typeVisual{
            order:-1!important;
            min-height:180px!important;
          }
          body:has(> header.hero.heroUnified) .priceTickerViewport{
            mask-image:none!important;
          }
          body:has(> header.hero.heroUnified) .priceTickerCard{
            flex-basis:82vw!important;
            width:82vw!important;
          }
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid{
          grid-template-columns:minmax(0,1.04fr) minmax(220px,.68fr) minmax(220px,.68fr)!important;
          grid-template-rows:repeat(2,minmax(226px,auto))!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:nth-child(1){
          min-height:468px!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard b{
          display:-webkit-box!important;
          max-width:100%!important;
          -webkit-line-clamp:2!important;
          -webkit-box-orient:vertical!important;
          overflow:hidden!important;
          overflow-wrap:anywhere!important;
          word-break:break-word!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:not(:first-child) b{
          font-size:clamp(20px,1.75vw,28px)!important;
          line-height:1.08!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:first-child b{
          max-width:420px!important;
          font-size:clamp(32px,3.6vw,52px)!important;
        }
        body:has(> header.hero.heroUnified) .modelFlowGrid .typeModels i{
          max-width:100%!important;
          min-width:0!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerStack{
          gap:14px!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard{
          flex:0 0 clamp(336px,28vw,430px)!important;
          width:clamp(336px,28vw,430px)!important;
          min-height:430px!important;
          padding:0!important;
          overflow:hidden!important;
          background:
            radial-gradient(ellipse 64% 52% at 92% 0%,rgba(45,184,245,.13),transparent 62%),
            linear-gradient(180deg,#ffffff 0%,#eefaff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard:before{
          display:none!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard .priceModelPoster{
          position:relative!important;
          height:auto!important;
          aspect-ratio:16/9!important;
          margin:10px 10px 0!important;
          border:1px solid rgba(4,74,113,.14)!important;
          border-radius:20px!important;
          overflow:hidden!important;
          background:#dff5ff!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.72)!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard .priceModelPoster:after{
          content:""!important;
          position:absolute!important;
          inset:0!important;
          background:linear-gradient(180deg,transparent 42%,rgba(5,42,68,.16))!important;
          pointer-events:none!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard .priceModelPoster img{
          width:100%!important;
          height:100%!important;
          object-fit:cover!important;
          transform:scale(1.01)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop{
          min-height:0!important;
          padding:16px 18px 0!important;
          align-items:flex-start!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop b{
          max-width:260px!important;
          display:-webkit-box!important;
          -webkit-line-clamp:2!important;
          -webkit-box-orient:vertical!important;
          overflow:hidden!important;
          white-space:normal!important;
          color:#061a2c!important;
          font-size:18px!important;
          line-height:1.18!important;
          letter-spacing:-.02em!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop small{
          color:#426079!important;
          font:800 11px/1.25 var(--mono)!important;
          letter-spacing:.02em!important;
          text-transform:none!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop i{
          height:30px!important;
          min-width:50px!important;
          background:#052a44!important;
          color:#fff!important;
          border:1px solid rgba(45,184,245,.36)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers{
          padding:16px 18px 0!important;
          gap:12px!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span{
          min-height:88px!important;
          display:flex!important;
          flex-direction:column!important;
          justify-content:center!important;
          border-radius:18px!important;
          border-color:rgba(4,74,113,.15)!important;
          background:rgba(255,255,255,.72)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span:first-child{
          background:rgba(255,255,255,.54)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span:last-child{
          border-color:rgba(45,184,245,.28)!important;
          background:linear-gradient(180deg,#ffffff,#e6f8ff)!important;
          box-shadow:0 18px 42px -34px rgba(4,52,82,.35)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers small{
          margin:0!important;
          color:#5b7285!important;
          font:900 9px/1 var(--mono)!important;
          letter-spacing:.08em!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers s{
          margin-top:9px!important;
          color:#7890a2!important;
          font:850 12px/1.25 var(--mono)!important;
          text-decoration-thickness:1.5px!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers strong{
          margin-top:9px!important;
          color:#005f89!important;
          font:950 clamp(19px,1.45vw,25px)/1.05 var(--mono)!important;
          letter-spacing:-.025em!important;
        }
        body:has(> header.hero.heroUnified) .priceModelCard p{
          min-height:0!important;
          margin-top:auto!important;
          padding:14px 18px 18px!important;
          border-top:1px solid rgba(4,74,113,.1)!important;
          color:#31556b!important;
          font-size:12px!important;
          line-height:1.5!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard{
          border-radius:26px!important;
          border-color:rgba(5,42,68,.16)!important;
          background:#fff!important;
          box-shadow:
            0 28px 78px -58px rgba(5,42,68,.42),
            inset 0 1px 0 rgba(255,255,255,.92)!important;
        }
        body:has(> header.hero.heroUnified) .priceTickerCard:hover{
          transform:translateY(-6px)!important;
          border-color:rgba(45,184,245,.38)!important;
          box-shadow:
            0 36px 92px -60px rgba(5,42,68,.56),
            0 14px 34px -28px rgba(45,184,245,.28)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelTop{
          display:grid!important;
          grid-template-columns:minmax(0,1fr) auto!important;
          gap:12px!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers{
          position:relative!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers:before{
          content:""!important;
          position:absolute!important;
          left:50%!important;
          top:20px!important;
          bottom:4px!important;
          width:1px!important;
          background:rgba(5,42,68,.1)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span{
          border:0!important;
          box-shadow:none!important;
          padding:12px 10px!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span:first-child{
          background:#f5f8fb!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers span:last-child{
          background:linear-gradient(135deg,#e8f8ff 0%,#f4f1ff 56%,#fff2f7 100%)!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers strong{
          display:block!important;
          max-width:100%!important;
          color:#052a44!important;
          text-wrap:balance!important;
        }
        body:has(> header.hero.heroUnified) .priceModelNumbers strong::first-line{
          color:#006b9a!important;
        }
        body:has(> header.hero.heroUnified) .priceNote{
          justify-content:center!important;
          margin-top:24px!important;
        }
        body:has(> header.hero.heroUnified) .priceNote span,
        body:has(> header.hero.heroUnified) .priceNote a{
          background:rgba(255,255,255,.78)!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.9)!important;
        }
        body:has(> header.hero.heroUnified) .whyGrid{
          align-items:stretch!important;
        }
        body:has(> header.hero.heroUnified) .whyCard{
          min-height:270px!important;
          padding:22px!important;
          background:
            radial-gradient(ellipse 60% 48% at 96% 0%,rgba(45,184,245,.12),transparent 62%),
            linear-gradient(180deg,#fff,#f1faff)!important;
        }
        body:has(> header.hero.heroUnified) .whyCard:before{
          content:""!important;
          width:44px!important;
          height:5px!important;
          border-radius:999px!important;
          background:linear-gradient(90deg,#2db8f5,#0a9c86,#d73679)!important;
        }
        body:has(> header.hero.heroUnified) .whyMetric{
          margin-top:22px!important;
        }
        body:has(> header.hero.heroUnified) .voiceWall{
          min-height:0!important;
          padding:0!important;
          overflow:visible!important;
          mask-image:none!important;
        }
        body:has(> header.hero.heroUnified) .voiceTrack{
          display:grid!important;
          width:auto!important;
          grid-template-columns:repeat(2,minmax(0,1fr))!important;
          gap:14px!important;
          animation:none!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble{
          position:relative!important;
          width:auto!important;
          min-height:198px!important;
          border-radius:22px!important;
          background:linear-gradient(180deg,#fff,#f0faff)!important;
          overflow:hidden!important;
          transition:transform .22s ease,border-color .22s ease,box-shadow .22s ease,background-color .22s ease!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble:before{
          content:""!important;
          position:absolute!important;
          left:22px!important;
          top:18px!important;
          width:34px!important;
          height:3px!important;
          border-radius:999px!important;
          background:linear-gradient(90deg,#2db8f5,#0a9c86,#7568e8)!important;
          opacity:.75!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble:hover{
          border-color:rgba(45,184,245,.35)!important;
          box-shadow:0 28px 72px -56px rgba(5,42,68,.46)!important;
          background:linear-gradient(180deg,#fff,#e8f8ff)!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble p{
          font-size:16px!important;
          line-height:1.5!important;
        }
        body:has(> header.hero.heroUnified) .launchPanel{
          display:grid!important;
          gap:12px!important;
          min-height:100%!important;
          padding:18px!important;
          border:1px solid rgba(4,74,113,.2)!important;
          border-radius:26px!important;
          background:
            radial-gradient(ellipse 54% 46% at 90% 0%,rgba(45,184,245,.18),transparent 62%),
            linear-gradient(180deg,#fff,#eefaff)!important;
          box-shadow:0 30px 78px -58px rgba(4,52,82,.42),inset 0 1px 0 rgba(255,255,255,.9)!important;
        }
        body:has(> header.hero.heroUnified) .launchStep{
          display:grid!important;
          grid-template-columns:54px minmax(0,1fr)!important;
          gap:14px!important;
          align-items:start!important;
          padding:16px!important;
          border:1px solid rgba(4,74,113,.16)!important;
          border-radius:20px!important;
          background:rgba(255,255,255,.78)!important;
        }
        body:has(> header.hero.heroUnified) .launchStep span{
          display:grid!important;
          place-items:center!important;
          width:54px!important;
          height:54px!important;
          border-radius:18px!important;
          background:#052a44!important;
          color:#fff!important;
          font:950 13px/1 var(--mono)!important;
        }
        body:has(> header.hero.heroUnified) .launchStep b{
          display:block!important;
          color:#061a2c!important;
          font:900 20px/1.12 var(--disp)!important;
          letter-spacing:-.02em!important;
        }
        body:has(> header.hero.heroUnified) .launchStep small{
          display:block!important;
          margin-top:7px!important;
          color:#36546b!important;
          font-size:13px!important;
          line-height:1.5!important;
        }
        body:has(> header.hero.heroUnified) .ctaWrap{
          padding-top:92px!important;
          padding-bottom:96px!important;
          background:#f7fbff!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel{
          position:relative!important;
          grid-template-columns:minmax(0,.92fr) minmax(420px,.9fr)!important;
          gap:26px!important;
          padding:34px!important;
          border:1px solid rgba(4,74,113,.14)!important;
          border-radius:28px!important;
          background:#f7fbff!important;
          backdrop-filter:none!important;
          box-shadow:
            0 28px 80px -66px rgba(5,42,68,.38),
            inset 0 1px 0 rgba(255,255,255,.78)!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel:before{
          display:none!important;
        }
        body:has(> header.hero.heroUnified) .btcCopy{
          position:relative!important;
          z-index:1!important;
          padding:10px 8px!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel h2{
          max-width:660px!important;
          font-size:clamp(38px,4.2vw,62px)!important;
          letter-spacing:-.045em!important;
        }
        body:has(> header.hero.heroUnified) .btcMetrics{
          grid-template-columns:repeat(3,minmax(0,1fr))!important;
          gap:12px!important;
          margin:24px 0!important;
        }
        body:has(> header.hero.heroUnified) .btcMetrics span{
          min-height:84px!important;
          display:flex!important;
          flex-direction:column!important;
          justify-content:center!important;
          padding:14px!important;
          border:0!important;
          border-radius:20px!important;
          background:#fff!important;
          box-shadow:
            0 16px 40px -34px rgba(5,42,68,.36),
            inset 0 1px 0 rgba(255,255,255,.9)!important;
        }
        body:has(> header.hero.heroUnified) .btcMedia{
          display:grid!important;
          gap:12px!important;
          align-content:start!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .ctaBillboard{
          aspect-ratio:16/8.5!important;
          margin:0!important;
          border-radius:24px!important;
          border-color:rgba(5,42,68,.12)!important;
          box-shadow:0 26px 70px -54px rgba(5,42,68,.42)!important;
        }
        body:has(> header.hero.heroUnified) .launchPanel{
          padding:12px!important;
          border-radius:24px!important;
          background:#f7fbff!important;
        }
        body:has(> header.hero.heroUnified) .launchStep{
          min-height:86px!important;
          padding:13px!important;
          border-radius:18px!important;
        }
        body:has(> header.hero.heroUnified) .launchStep span{
          width:48px!important;
          height:48px!important;
          border-radius:16px!important;
        }
        body:has(> header.hero.heroUnified) .launchStep b{
          font-size:18px!important;
        }
        body:has(> header.hero.heroUnified) .ctaTrust{
          justify-content:flex-start!important;
          margin-top:2px!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .ctaTrust span{
          background:rgba(255,255,255,.76)!important;
        }
        @keyframes whyCardIn{
          from{opacity:0;transform:translateY(18px)}
          to{opacity:1;transform:translateY(0)}
        }
        @keyframes voiceCardFloat{
          0%,100%{box-shadow:0 22px 64px -58px rgba(5,42,68,.38)}
          50%{box-shadow:0 30px 78px -56px rgba(5,42,68,.5)}
        }
        @keyframes voiceBoardSheen{
          0%,42%{transform:skewX(-18deg) translateX(-120%);opacity:0}
          54%{opacity:1}
          70%,100%{transform:skewX(-18deg) translateX(420%);opacity:0}
        }
        body:has(> header.hero.heroUnified) .voiceWall{
          position:relative!important;
          min-height:300px!important;
          overflow:hidden!important;
          border-radius:26px!important;
          mask-image:none!important;
        }
        body:has(> header.hero.heroUnified) .voiceTrack{
          position:relative!important;
          display:block!important;
          width:100%!important;
          height:300px!important;
          animation:none!important;
        }
        body:has(> header.hero.heroUnified) .voiceTrack:after{
          content:""!important;
          position:absolute!important;
          left:24px!important;
          right:24px!important;
          bottom:20px!important;
          height:3px!important;
          border-radius:999px!important;
          background:linear-gradient(90deg,#2db8f5,#0a9c86,#7568e8,#d73679)!important;
          transform-origin:left!important;
          animation:voiceProgress 5s linear infinite!important;
          opacity:.8!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble{
          position:absolute!important;
          inset:4px!important;
          width:auto!important;
          min-height:0!important;
          opacity:0;
          transform:translateY(16px) scale(.985);
          pointer-events:none;
          animation:voiceFadeCycle 20s cubic-bezier(.16,1,.3,1) infinite!important;
          animation-delay:calc(var(--voice-index, 0) * 5s)!important;
          box-shadow:0 30px 86px -62px rgba(5,42,68,.5)!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble:hover,
        body:has(> header.hero.heroUnified) .voiceBubble:nth-child(2),
        body:has(> header.hero.heroUnified) .voiceBubble:nth-child(4),
        body:has(> header.hero.heroUnified) .voiceBubble:nth-child(2):hover,
        body:has(> header.hero.heroUnified) .voiceBubble:nth-child(4):hover{
          transform:translateY(0) scale(1);
        }
        body:has(> header.hero.heroUnified) .voiceBubble b{
          margin-top:18px!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble p{
          margin-top:28px!important;
          max-width:680px!important;
          font-size:clamp(18px,1.65vw,25px)!important;
          line-height:1.5!important;
          letter-spacing:-.02em!important;
        }
        body:has(> header.hero.heroUnified) .voiceBubble small{
          margin-top:auto!important;
          padding-top:18px!important;
        }
        body:has(> header.hero.heroUnified) .voiceWall:hover .voiceBubble,
        body:has(> header.hero.heroUnified) .voiceWall:hover .voiceTrack:after{
          animation-play-state:paused!important;
        }
        @keyframes voiceFadeCycle{
          0%{opacity:0;transform:translateY(16px) scale(.985);pointer-events:none}
          5%,24%{opacity:1;transform:translateY(0) scale(1);pointer-events:auto}
          30%,100%{opacity:0;transform:translateY(-14px) scale(.99);pointer-events:none}
        }
        @keyframes voiceProgress{
          from{transform:scaleX(0)}
          to{transform:scaleX(1)}
        }
        @media(max-width:1180px){
          body:has(> header.hero.heroUnified) .whyGrid{
            grid-template-columns:repeat(2,minmax(0,1fr))!important;
          }
          body:has(> header.hero.heroUnified) .voiceTrack{
            grid-template-columns:repeat(2,minmax(0,1fr))!important;
          }
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .modelFlowGrid{
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .modelFlowGrid .typeCard:first-child b{
            font-size:clamp(30px,8vw,40px)!important;
          }
          body:has(> header.hero.heroUnified) .priceTickerCard{
            flex-basis:84vw!important;
            width:84vw!important;
          }
          body:has(> header.hero.heroUnified) .voiceTrack{
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .voiceBubble:nth-child(2),
          body:has(> header.hero.heroUnified) .voiceBubble:nth-child(4),
          body:has(> header.hero.heroUnified) .voiceBubble:nth-child(2):hover,
          body:has(> header.hero.heroUnified) .voiceBubble:nth-child(4):hover{
            transform:none;
          }
          body:has(> header.hero.heroUnified) .whyGrid{
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .launchStep{
            grid-template-columns:46px minmax(0,1fr)!important;
          }
          body:has(> header.hero.heroUnified) .launchStep span{
            width:46px!important;
            height:46px!important;
            border-radius:15px!important;
          }
        }
        body:has(> header.hero.heroUnified) .cliQuick{
          background:linear-gradient(180deg,#fff7fb 0%,#f2f9ff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .why{
          background:linear-gradient(180deg,#f2f9ff 0%,#fff6fb 100%)!important;
        }
        body:has(> header.hero.heroUnified) .voiceFaq{
          background:linear-gradient(180deg,#fff6fb 0%,#f2f9ff 100%)!important;
        }
        body:has(> header.hero.heroUnified) .ctaWrap{
          background:linear-gradient(180deg,#f2f9ff 0%,#fff6fb 100%)!important;
        }
        body:has(> header.hero.heroUnified) .cliStepperPanel{
          border-color:rgba(4,74,113,.18)!important;
          background:linear-gradient(180deg,rgba(255,255,255,.86),rgba(238,250,255,.72))!important;
          color:#061a2c!important;
          box-shadow:0 28px 82px -62px rgba(5,42,68,.38),inset 0 1px 0 rgba(255,255,255,.86)!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody{
          border-color:rgba(4,74,113,.14)!important;
          background:rgba(255,255,255,.78)!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody b{
          color:#061a2c!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody p,
        body:has(> header.hero.heroUnified) .cliQuickChecks span{
          color:#36546b!important;
        }
        body:has(> header.hero.heroUnified) .cliStepBody code{
          background:#eef7fb!important;
          color:#052a44!important;
          border:1px solid rgba(4,74,113,.12)!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel{
          border:0!important;
          border-radius:0!important;
          background:transparent!important;
          box-shadow:none!important;
          padding:0!important;
        }
        body:has(> header.hero.heroUnified) .btcCopy{
          padding:0!important;
          border:0!important;
          background:transparent!important;
          box-shadow:none!important;
        }
        body:has(> header.hero.heroUnified) .btcMedia{
          padding:0!important;
          border:0!important;
          background:transparent!important;
          box-shadow:none!important;
        }
        body:has(> header.hero.heroUnified) .btcMetrics span,
        body:has(> header.hero.heroUnified) .launchStep{
          background:#ffffff!important;
          border:1px solid rgba(4,74,113,.14)!important;
          box-shadow:inset 0 1px 0 rgba(255,255,255,.86)!important;
        }
        body:has(> header.hero.heroUnified) .launchPanel{
          background:transparent!important;
          border:0!important;
          box-shadow:none!important;
          padding:0!important;
        }
        body:has(> header.hero.heroUnified) .btcPanel .ctaBillboard{
          box-shadow:0 24px 70px -58px rgba(5,42,68,.38)!important;
          background:#ffffff!important;
        }
        body:has(> header.hero.heroUnified) .ctaWrap{
          background:none!important;
          border-bottom:0!important;
          padding-top:88px!important;
        }
        body:has(> header.hero.heroUnified) .voiceFaq{
          background:#f2f9ff!important;
          border-bottom:0!important;
          padding-bottom:0!important;
        }
        body:has(> header.hero.heroUnified) .voiceFaqIn{
          padding-bottom:88px!important;
        }
        body:has(> header.hero.heroUnified) .ctaBanner,
        body:has(> header.hero.heroUnified) .btcPanel,
        body:has(> header.hero.heroUnified) .btcPanel:before,
        body:has(> header.hero.heroUnified) .btcCopy,
        body:has(> header.hero.heroUnified) .btcMedia,
        body:has(> header.hero.heroUnified) .launchPanel{
          background:transparent!important;
          box-shadow:none!important;
          border:0!important;
        }
        @media(max-width:720px){
          body:has(> header.hero.heroUnified) .heroUnified{
            min-height:680px!important;
            display:flex!important;
            align-items:stretch!important;
          }
          body:has(> header.hero.heroUnified) .heroGrid{
            min-height:680px!important;
            display:block!important;
            padding:108px 18px 52px!important;
          }
          body:has(> header.hero.heroUnified) .heroCopy{
            max-width:none!important;
            margin:0!important;
            padding:0!important;
          }
          body:has(> header.hero.heroUnified) .heroCopy:before{
            display:none!important;
          }
          body:has(> header.hero.heroUnified) .heroTitle{
            max-width:100%!important;
            min-height:112px!important;
            font-size:clamp(38px,12vw,54px)!important;
            line-height:1.05!important;
            letter-spacing:-.035em!important;
          }
          body:has(> header.hero.heroUnified) .heroSub{
            max-width:100%!important;
            min-height:74px!important;
            font-size:15px!important;
            line-height:1.62!important;
          }
          body:has(> header.hero.heroUnified) .heroCtas{
            min-height:50px!important;
          }
          body:has(> header.hero.heroUnified) .heroStageList{
            position:relative!important;
            z-index:9!important;
            display:block!important;
            width:100%!important;
            height:112px!important;
            margin-top:24px!important;
            overflow:hidden!important;
            padding:0!important;
            transform:none!important;
          }
          body:has(> header.hero.heroUnified) .heroStageNav{
            position:absolute!important;
            inset:0!important;
            display:grid!important;
            grid-template-columns:64px minmax(0,1fr)!important;
            grid-template-rows:1fr!important;
            align-items:center!important;
            gap:12px!important;
            width:100%!important;
            min-width:0!important;
            height:84px!important;
            min-height:84px!important;
            padding:10px 14px 10px 10px!important;
            border-radius:22px!important;
            opacity:0!important;
            pointer-events:none!important;
            transform:translateY(10px) scale(.985)!important;
            transition:opacity .32s ease,transform .32s ease!important;
          }
          body:has(> header.hero.heroUnified) .heroStageNav.is-active{
            opacity:1!important;
            pointer-events:auto!important;
            transform:translateY(0) scale(1)!important;
          }
          body:has(> header.hero.heroUnified) .heroStageNav img{
            grid-row:auto!important;
            grid-column:1!important;
            width:64px!important;
            height:64px!important;
            border-radius:18px!important;
          }
          body:has(> header.hero.heroUnified) .heroStageNav span,
          body:has(> header.hero.heroUnified) .heroStageNav small,
          body:has(> header.hero.heroUnified) .heroStageAll{
            display:none!important;
          }
          body:has(> header.hero.heroUnified) .heroStageNav b{
            grid-column:2!important;
            display:block!important;
            margin:0!important;
            font-size:13px!important;
            line-height:1.1!important;
            white-space:normal!important;
            overflow:hidden!important;
            display:-webkit-box!important;
            -webkit-box-orient:vertical!important;
            -webkit-line-clamp:2!important;
          }
          body:has(> header.hero.heroUnified) .heroStagePager{
            position:absolute!important;
            left:0!important;
            right:0!important;
            bottom:2px!important;
            display:flex!important;
            justify-content:center!important;
            gap:7px!important;
          }
          body:has(> header.hero.heroUnified) .heroStagePager i{
            width:7px!important;
            height:7px!important;
            border-radius:999px!important;
            background:rgba(4,74,113,.2)!important;
            transition:width .24s ease,background-color .24s ease!important;
          }
          body:has(> header.hero.heroUnified) .heroStagePager i.is-active{
            width:22px!important;
            background:#0877ad!important;
          }
          body:has(> header.hero.heroUnified) .heroProductRail{
            pointer-events:none!important;
          }
          body:has(> header.hero.heroUnified) .heroStageCopy{
            display:none!important;
          }
          body:has(> header.hero.heroUnified) .heroStageImage{
            object-position:center top!important;
            opacity:.42!important;
          }
          body:has(> header.hero.heroUnified) .heroStageShade{
            background:linear-gradient(180deg,rgba(246,251,255,.94),rgba(246,251,255,.74) 44%,rgba(255,246,251,.96))!important;
          }
          body:has(> header.hero.heroUnified) .ctaWrap{
            padding:64px 18px!important;
          }
          body:has(> header.hero.heroUnified) .btcPanel{
            display:grid!important;
            grid-template-columns:1fr!important;
            gap:24px!important;
          }
          body:has(> header.hero.heroUnified) .btcPanel h2{
            font-size:clamp(34px,10vw,46px)!important;
            line-height:1.05!important;
          }
          body:has(> header.hero.heroUnified) .btcMetrics{
            grid-template-columns:1fr!important;
          }
          body:has(> header.hero.heroUnified) .ctaBtns{
            flex-direction:column!important;
            align-items:stretch!important;
          }
          body:has(> header.hero.heroUnified) .ctaBtns .btn{
            width:100%!important;
          }
          body:has(> header.hero.heroUnified) .btcPanel .ctaBillboard{
            aspect-ratio:16/10!important;
          }
          body:has(> header.hero.heroUnified) .launchStep{
            grid-template-columns:46px minmax(0,1fr)!important;
          }
          body:has(> header.hero.heroUnified) .whyHead .sectionTitle,
          body:has(> header.hero.heroUnified) .whyCard h3,
          body:has(> header.hero.heroUnified) .whyCard p,
          body:has(> header.hero.heroUnified) .whyMini span{
            word-break:normal!important;
            overflow-wrap:anywhere!important;
            white-space:normal!important;
            text-wrap:wrap!important;
          }
          body:has(> header.hero.heroUnified) .whyHead .sectionTitle{
            max-width:100%!important;
            font-size:clamp(30px,9vw,42px)!important;
            line-height:1.12!important;
          }
          body:has(> header.hero.heroUnified) .whyCard{
            min-width:0!important;
          }
          body:has(> header.hero.heroUnified) .modelLogoInner{
            padding-top:10px!important;
            padding-bottom:10px!important;
          }
          body:has(> header.hero.heroUnified) .modelTypesIn,
          body:has(> header.hero.heroUnified) .priceProofIn,
          body:has(> header.hero.heroUnified) .cliQuickIn,
          body:has(> header.hero.heroUnified) .whyIn,
          body:has(> header.hero.heroUnified) .voiceFaqIn{
            padding-top:48px!important;
            padding-bottom:52px!important;
          }
          body:has(> header.hero.heroUnified) .typeGrid,
          body:has(> header.hero.heroUnified) .modelFlowGrid,
          body:has(> header.hero.heroUnified) .priceTickerStack,
          body:has(> header.hero.heroUnified) .voiceShowcase,
          body:has(> header.hero.heroUnified) .faqGrid,
          body:has(> header.hero.heroUnified) .whyGrid{
            margin-top:22px!important;
          }
          body:has(> header.hero.heroUnified) .sectionTitle{
            margin-top:10px!important;
          }
          body:has(> header.hero.heroUnified) .sectionSub{
            margin-top:10px!important;
          }
          body:has(> header.hero.heroUnified) .priceNote{
            margin-top:16px!important;
          }
          body:has(> header.hero.heroUnified) .cliQuickActions,
          body:has(> header.hero.heroUnified) .btcMetrics{
            margin-top:18px!important;
            margin-bottom:18px!important;
          }
          body:has(> header.hero.heroUnified) .ctaWrap{
            padding-top:52px!important;
            padding-bottom:56px!important;
          }
          body:has(> header.hero.heroUnified) .voiceFaqIn{
            padding-bottom:52px!important;
          }
        }
        @media(prefers-reduced-motion:reduce){.heroStageSlide,.heroStageCopy,.heroStageNav:before,.heroStageImage,.logoTrack,.voiceTrack,.fk-reveal{animation:none!important;transition:none!important}.heroStageSlide:first-child{opacity:1;pointer-events:auto}.heroStageSlide:first-child .heroStageCopy{opacity:1;transform:none}}
      `}</style>

      <OnlineHomeHeroCarousel copy={copy.carousel} heroModes={heroModes} locale={props.locale} />

      <section className="modelLogoMarquee" aria-label={copy.logo.aria}>
        <div className="modelLogoInner">
          <div className="logoTrackViewport">
            <div className="logoTrack">
              {[...logoMarquee, ...logoMarquee].map(([src, name], index) => (
                <Link className="logoPill" href={localizePath("/models", props.locale)} key={`${name}-${index}`}>
                  <img src={`/assets/logos/${src}`} alt="" />
                  <div><b>{name}</b><span>{copy.logo.connected}</span></div>
                </Link>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section className="modelTypes">
        <div className="modelTypesIn">
          <div className="apiAppsHead">
            <div>
              <div className="kick2">{copy.modelFlow.kicker}</div>
              <h2 className="sectionTitle">{copy.modelFlow.title}</h2>
            </div>
            <div>
              <p className="sectionSub">{copy.modelFlow.sub}</p>
              <div className="apiAppsProof">
                {copy.modelFlow.proof.map((item) => (
                  <span key={item}>{item}</span>
                ))}
              </div>
            </div>
          </div>
          <div className="typeGrid modelFlowGrid">
            {modelTypes.map((item) => (
              <Link className={`typeCard typeCard-${item.tone}`} href={localizePath(item.href, props.locale)} key={item.title}>
                <div className="typeCardCopy">
                  <span>{item.api}</span>
                  <b>{item.title}</b>
                  <p>{item.copy}</p>
                  <div className="typeModels">
                    {(item.models.length > 0 ? item.models : [copy.modelFlow.directoryFallback]).map((model) => (
                      <i key={model}>{model}</i>
                    ))}
                  </div>
                  <strong>{item.cta}</strong>
                </div>
                <div className="typeVisual"><img src={item.image} alt="" /></div>
              </Link>
            ))}
          </div>
        </div>
      </section>

      <section className="priceProof" id="price-comparison">
        <div className="priceProofIn">
          <div className="priceProofHead">
            <div>
              <div className="kick2">{copy.price.kicker}</div>
              <h2 className="sectionTitle">{copy.price.title}</h2>
            </div>
            <p className="sectionSub">{copy.price.sub}</p>
          </div>
          <div className="priceTickerStack" aria-label={copy.price.aria}>
            {priceRows.map((rowItems, rowIndex) => (
              <div className={`priceTickerViewport priceTicker-${rowIndex}`} key={`price-row-${rowIndex}`}>
                <div className="priceTickerTrack">
                  {[...rowItems, ...rowItems].map((row, index) => (
                    <Link className="priceModelCard priceTickerCard" href={localizePath(row.href, props.locale)} key={`${row.model}-${rowIndex}-${index}`}>
                      <div className="priceModelPoster"><img src={row.image} alt="" /></div>
                      <div className="priceModelTop">
                        <span><b>{row.model}</b><small>{row.vendor} · {copy.price.officialEndpoint}</small></span>
                        <i>{copy.price.test}</i>
                      </div>
                      <div className="priceModelNumbers">
                        <span><small>{copy.price.officialLabel}</small><s>{row.official}</s></span>
                        <span><small>Flatkey</small><strong>{row.flatkey}</strong></span>
                      </div>
                      <p>{row.policy}</p>
                    </Link>
                  ))}
                </div>
              </div>
            ))}
          </div>
          <div className="priceNote">
            <span>{copy.price.sharedBalance}</span>
            <span>{copy.price.flatkeyFailure}</span>
            <span>{copy.price.requestLedger}</span>
            <Link href={localizePath("/models", props.locale)}>{copy.price.viewAll}</Link>
          </div>
        </div>
      </section>

      <section className="cliQuick" id="cli-quickstart">
        <div className="cliQuickIn">
          <div className="cliQuickCopy">
            <div className="kick2">{copy.cli.kicker}</div>
            <h2 className="sectionTitle">{copy.cli.title}</h2>
            <p className="sectionSub" style={{ marginTop: 18 }}>{copy.cli.sub}</p>
            <div className="cliQuickActions">
              <Link className="btn big cliPrimary" href={localizePath("/cli", props.locale)}>{copy.cli.guide}</Link>
              <a className="btn big cliSecondary" href="https://docs.flatkey.ai/" target="_blank" rel="noopener noreferrer">{copy.cli.docs}</a>
            </div>
          </div>
          <div className="cliQuickPanel cliStepperPanel" aria-label={copy.cli.aria}>
            <div className="cliQuickTop"><span>{copy.cli.range}</span><i>{copy.cli.stepsDone}</i></div>
            <div className="cliSteps">
              {cliSteps.map((step) => (
                <article className="cliStep" key={step.no}>
                  <span className="cliStepNo">{step.no}</span>
                  <div className="cliStepBody">
                    <b>{step.title}</b>
                    <p>{step.body}</p>
                    <code>{step.code}</code>
                  </div>
                </article>
              ))}
            </div>
            <div className="cliQuickChecks">
              {copy.cli.checks.map((item) => <span key={item}>{item}</span>)}
            </div>
          </div>
        </div>
      </section>

      <section className="why" id="why">
        <div className="whyIn">
          <div className="whyHead">
            <div className="kick2">{copy.why.kicker}</div>
            <h2 className="sectionTitle">{copy.why.title}</h2>
          </div>
          <div className="whyGrid">
            {whyCards.map((item, index) => (
              <article className="whyCard" key={item.title} style={{ "--why-index": index } as CSSProperties}>
                <span className="whyMetric">{item.metric}</span>
                <h3>{item.title}</h3>
                <p>{item.body}</p>
                <div className="whyMini">
                  {item.chips.map((chip) => (
                    <span key={chip}>{chip}</span>
                  ))}
                </div>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="voiceFaq" id="reviews">
        <div className="voiceFaqIn">
          <div className="sectionSplit">
            <div>
              <div className="kick2">{copy.voice.kicker}</div>
              <h2 className="sectionTitle">{copy.voice.title}</h2>
            </div>
            <p className="sectionSub">{copy.voice.sub}</p>
          </div>
          <div className="voiceShowcase">
            <article className="voiceBoard">
              <span>{copy.voice.boardMetric}</span>
              <strong>{copy.voice.boardTitle}</strong>
              <p>{copy.voice.boardCopy}</p>
              <div className="voiceSignals">
                {copy.voice.signals.map((item) => (
                  <i key={item}>{item}</i>
                ))}
              </div>
            </article>
            <div className="voiceWall" aria-label={copy.voice.aria}>
              <div className="voiceTrack">
                {voiceItems.map((item, index) => (
                  <article className="quoteCard voiceBubble" key={item.metric} style={{ "--voice-index": index } as CSSProperties}>
                    <b>{item.metric}</b>
                    <p>{item.quote}</p>
                    <small>{item.role}</small>
                  </article>
                ))}
              </div>
            </div>
          </div>
          <div className="faqGrid">
            {faqs.map(([question, answer]) => (
              <article className="faqCard" key={question}>
                <h3>{question}</h3>
                <p>{answer}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section className="ctaWrap" id="brand-trust">
        <div className="ctaBanner btcPanel">
          <div className="ctaIn btcCopy">
            <div className="kick2">{copy.final.kicker}</div>
            <h2>{copy.final.title}</h2>
            <p>{copy.final.sub}</p>
            <div className="btcMetrics">
              <span><b>SOC 2</b><small>{copy.final.metrics[0]}</small></span>
              <span><b>ISO 27001</b><small>{copy.final.metrics[1]}</small></span>
              <span><b>99.5%</b><small>{copy.final.metrics[2]}</small></span>
            </div>
            <div className="ctaBtns">
              <a className="btn white big" href={consoleUrl("/sign-up")}>{copy.ctaConsole}</a>
              <Link className="btn black big" href={localizePath("/contact", props.locale)}>{copy.contactSales}</Link>
            </div>
          </div>
          <div className="ctaProof btcMedia">
            <figure className="ctaBillboard">
              <Image src="/assets/brand/bay-area-billboard-main.jpg" alt={copy.final.alt} fill sizes="(max-width: 900px) 100vw, 42vw" />
            </figure>
            <div className="launchPanel">
              {copy.final.launch.map(([no, title, body]) => (
                <div className="launchStep" key={no}>
                  <span>{no}</span>
                  <div><b>{title}</b><small>{body}</small></div>
                </div>
              ))}
            </div>
            <div className="ctaTrust">
              {["SOC 2 Type II", "ISO 27001", "99.5% SLA", copy.final.trustZeroRetention].map((item) => (
                <span key={item}>{item}</span>
              ))}
            </div>
          </div>
        </div>
      </section>
    </OnlineStaticShell>
  );
}
