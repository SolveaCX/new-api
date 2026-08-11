import Image from "next/image";
import Link from "next/link";
import { ArrowRight, Sparkles, Star } from "lucide-react";
import type { CSSProperties } from "react";
import { FlatkeyTallyEmbed } from "@/components/flatkey-tally-embed";
import { HomeSectionReveals } from "@/components/home-section-reveals";
import { ModelLogo } from "@/components/pricing-model-browser";
import { SiteShell } from "@/components/site-shell";
import { getCopy } from "@/lib/copy";
import { getHomeCopy } from "@/lib/home-copy";
import { buildRowsForModels, pickFlagshipModels, type HomePricedModel } from "@/lib/home-models";
import type { Locale } from "@/lib/locales";
import { localizePath, withIdFallback } from "@/lib/locales";
import { modelPublicPath } from "@/lib/model-public";
import { ROUTER_ORIGIN, consoleUrl } from "@/lib/origins";
import { getPricingData } from "@/lib/pricing";

const API_BASE_URL = `${ROUTER_ORIGIN}/v1`;

type Props = {
  locale: Locale;
};

type ExperienceCopy = {
  heroBadge: string;
  heroLine1: string;
  heroLine2: string;
  heroLine3: string;
  heroDescription: string;
  primaryCta: string;
  secondaryCta: string;
  modelTags: string[];
  featuredModelsCta: string;
  officialPriceLabel: string;
  flatkeyPriceLabel: string;
  priceEyebrow: string;
  priceTitle: string;
  priceDescription: string;
  priceTableNote: string;
  faqEyebrow: string;
  faqTitle: string;
  faqDescription: string;
  faqs: { question: string; answer: string }[];
  voicesEyebrow: string;
  voicesTitle: string;
  voices: { quote: string; role: string }[];
};

const EXPERIENCE_COPY: Record<Locale, ExperienceCopy> = withIdFallback({
  en: {
    heroBadge: "Omnimodal routing for real product teams",
    heroLine1: "All-modal AI.",
    heroLine2: "One key",
    heroLine3: "is enough.",
    heroDescription:
      "Seamlessly call frontier models like GPT-5, Claude, Seedance, and ElevenLabs. Unified routing, one prepaid balance, and one dashboard to control billing and budget.",
    primaryCta: "Get API key",
    secondaryCta: "Compare pricing",
    modelTags: ["GPT-5 text & code", "Claude agents", "Seedance video", "ElevenLabs voice", "DeepSeek reasoning", "Kimi long context", "GLM stack"],
    featuredModelsCta: "Explore all models",
    officialPriceLabel: "Official",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Official model price comparison",
    priceTitle: "Model price comparison",
    priceDescription: "Compare official model prices with flatkey's after-bonus price across text, image, video, voice, and reasoning calls.",
    priceTableNote: "After-bonus pricing combines supported model discounts with prepaid recharge credit.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription:
      "Straight answers on model authenticity, recharge bonuses, one endpoint, production routing, and privacy.",
    faqs: [
      { question: "Are these real official models?", answer: "Yes. flatkey.ai routes supported requests to official upstream model providers through one managed API gateway." },
      { question: "How can pricing reach up to 50% off?", answer: "Supported model rates can be lower than official list pricing, and paid recharge bonuses add extra usable balance. The two effects stack." },
      { question: "Do I need to change SDKs?", answer: `No. Keep your OpenAI-compatible client and point the base URL to ${API_BASE_URL}.` },
      { question: "Which modalities can I route through one key?", answer: "Use the same flatkey account for text, coding agents, reasoning, image, video, voice, and other supported model traffic." },
      { question: "Do you store prompts or completions?", answer: "Flatkey is designed for zero retention of request content, with GDPR, SOC 2, and ISO 27001 aligned controls." },
    ],
    voicesEyebrow: "Customer voices",
    voicesTitle: "What teams say after moving model spend into one key.",
    voices: [
      { quote: "We stopped switching dashboards just to check whether Kimi, GLM, and Claude calls were costing what we expected.", role: "AI product founder" },
      { quote: "The first win was simple: one endpoint for coding agents, then finance could actually see usage and recharge history.", role: "Engineering lead" },
      { quote: "We care less about a giant model wall and more about the five models our users ask for every week. Flatkey made that clean.", role: "Automation platform operator" },
    ],
  },
  zh: {
    heroBadge: "面向真实产品团队的全模态路由",
    heroLine1: "全模态 AI，",
    heroLine2: "一个 Key",
    heroLine3: "就够了。",
    heroDescription:
      "无缝调用 GPT-5、Claude、Seedance、ElevenLabs 等前沿模型。统一路由，一个预付余额，一站式控制账单与预算。",
    primaryCta: "获取 API Key",
    secondaryCta: "对比价格",
    modelTags: ["GPT-5 文本与代码", "Claude Agent", "Seedance 视频", "ElevenLabs 语音", "DeepSeek 推理", "Kimi 长上下文", "GLM 模型栈"],
    featuredModelsCta: "探索全部模型",
    officialPriceLabel: "官网",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "官方模型价格对比",
    priceTitle: "模型与官网价格对比",
    priceDescription: "对比文本、图像、视频、语音、推理模型的官网价格与 flatkey 充值后的实际价格。",
    priceTableNote: "充值后价格会叠加支持模型的折扣与预付充值赠送额度。",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription:
      "直接回答模型真实性、充值赠送、一个 endpoint、生产路由和隐私问题。",
    faqs: [
      { question: "这些是真实官方模型吗？", answer: "是。flatkey.ai 通过统一 API 网关，把支持的请求路由到官方上游模型服务。" },
      { question: "为什么实际价格最高能到 5 折？", answer: "支持模型本身会低于官方标价，付费充值还会获得赠送余额，两层优惠可以叠加。" },
      { question: "需要更换 SDK 吗？", answer: `不需要。保留兼容 OpenAI 的客户端，把 base URL 指向 ${API_BASE_URL} 即可。` },
      { question: "一个 Key 能路由哪些模态？", answer: "同一个 flatkey 账号可用于文本、编程 Agent、推理、图像、视频、语音等已支持模型流量。" },
      { question: "会存储 prompts 或 completions 吗？", answer: "Flatkey 按请求内容零留存设计，并对齐 GDPR、SOC 2、ISO 27001 控制要求。" },
    ],
    voicesEyebrow: "客户声音",
    voicesTitle: "团队把模型支出迁移到一个 key 后的反馈。",
    voices: [
      { quote: "我们不再为了确认 Kimi、GLM、Claude 的成本来回切不同后台，价格和用量都在一个地方。", role: "AI 产品创始人" },
      { quote: "第一个收益很直接：编程 Agent 只需要一个 endpoint，财务也终于能看到用量和充值记录。", role: "工程负责人" },
      { quote: "我们不需要一面巨大的模型墙，更需要用户每周都会问的那五个模型。Flatkey 把这件事做得很清楚。", role: "自动化平台运营负责人" },
    ],
  },
  es: {
    heroBadge: "Ruteo omnimodal para equipos de producto reales",
    heroLine1: "IA multimodal.",
    heroLine2: "Una key",
    heroLine3: "basta.",
    heroDescription:
      "Llama sin fricción a modelos frontier como GPT-5, Claude, Seedance y ElevenLabs. Ruteo unificado, saldo prepago y un panel para controlar facturación y presupuesto.",
    primaryCta: "Obtener API key",
    secondaryCta: "Comparar precios",
    modelTags: ["GPT-5 texto y código", "agentes Claude", "video Seedance", "voz ElevenLabs", "razonamiento DeepSeek", "contexto largo Kimi", "stack GLM"],
    featuredModelsCta: "Explorar todos los modelos",
    officialPriceLabel: "Oficial",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Comparación con precio oficial",
    priceTitle: "Comparación de precios de modelos",
    priceDescription: "Compara precios oficiales con el precio after-bonus de flatkey en texto, imagen, video, voz y razonamiento.",
    priceTableNote: "El precio after-bonus combina descuentos compatibles y crédito de recarga prepago.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "Respuestas sobre modelos reales, bonos, endpoint único, ruteo y privacidad.",
    faqs: [
      { question: "¿Son modelos oficiales reales?", answer: "Sí. flatkey.ai enruta solicitudes compatibles a proveedores upstream oficiales mediante un gateway gestionado." },
      { question: "¿Cómo llega el ahorro hasta 50%?", answer: "Algunos modelos tienen tarifa menor que la lista oficial y los bonos de recarga añaden saldo usable; ambos efectos se combinan." },
      { question: "¿Debo cambiar SDKs?", answer: `No. Mantén tu cliente compatible con OpenAI y usa ${API_BASE_URL} como base URL.` },
      { question: "¿Qué modalidades puedo enrutar con una key?", answer: "La misma cuenta flatkey sirve para texto, agentes de código, razonamiento, imagen, video, voz y otros modelos compatibles." },
      { question: "¿Guardan prompts o respuestas?", answer: "Flatkey está diseñado con retención cero del contenido y controles alineados con GDPR, SOC 2 e ISO 27001." },
    ],
    voicesEyebrow: "Voces de clientes",
    voicesTitle: "Lo que dicen los equipos al mover el gasto de modelos a una key.",
    voices: [
      { quote: "Dejamos de cambiar de panel para validar costes de Kimi, GLM y Claude.", role: "Fundador de producto IA" },
      { quote: "Un endpoint para agentes de código y finanzas puede ver uso y recargas.", role: "Líder de ingeniería" },
      { quote: "No necesitábamos un muro gigante de modelos, sino los cinco que nuestros usuarios piden.", role: "Operador de automatización" },
    ],
  },
  fr: {
    heroBadge: "Routage omnimodal pour vraies équipes produit",
    heroLine1: "IA multimodale.",
    heroLine2: "Une clé",
    heroLine3: "suffit.",
    heroDescription:
      "Appelez sans friction des modèles frontier comme GPT-5, Claude, Seedance et ElevenLabs. Routage unifié, solde prépayé et tableau de bord unique pour budget et facturation.",
    primaryCta: "Obtenir une clé API",
    secondaryCta: "Comparer les prix",
    modelTags: ["GPT-5 texte et code", "agents Claude", "vidéo Seedance", "voix ElevenLabs", "raisonnement DeepSeek", "long contexte Kimi", "stack GLM"],
    featuredModelsCta: "Explorer tous les modèles",
    officialPriceLabel: "Officiel",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Comparaison avec les prix officiels",
    priceTitle: "Comparaison des prix modèles",
    priceDescription: "Comparez les prix officiels avec le prix after-bonus flatkey pour texte, image, vidéo, voix et raisonnement.",
    priceTableNote: "Le prix after-bonus combine remises modèles et crédit de recharge prépayé.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "Réponses sur modèles réels, bonus, endpoint unique, routage et confidentialité.",
    faqs: [
      { question: "S'agit-il de vrais modèles officiels ?", answer: "Oui. flatkey.ai route les requêtes prises en charge vers des fournisseurs upstream officiels via une passerelle gérée." },
      { question: "Comment atteindre jusqu'à 50 % d'économie ?", answer: "Des tarifs modèles réduits peuvent se cumuler avec le crédit bonus des recharges payantes." },
      { question: "Faut-il changer de SDK ?", answer: `Non. Gardez votre client compatible OpenAI et utilisez ${API_BASE_URL} comme base URL.` },
      { question: "Quelles modalités passent par une seule clé ?", answer: "Le même compte flatkey couvre texte, agents de code, raisonnement, image, vidéo, voix et autres modèles pris en charge." },
      { question: "Stockez-vous prompts ou réponses ?", answer: "Flatkey est conçu pour une rétention zéro du contenu, avec contrôles alignés GDPR, SOC 2 et ISO 27001." },
    ],
    voicesEyebrow: "Voix clients",
    voicesTitle: "Ce que disent les équipes après avoir centralisé leurs dépenses modèles.",
    voices: [
      { quote: "Nous avons arrêté de passer d'un dashboard à l'autre pour vérifier les coûts Kimi, GLM et Claude.", role: "Fondateur produit IA" },
      { quote: "Un endpoint pour les agents de code, et la finance voit enfin usage et recharges.", role: "Lead engineering" },
      { quote: "Pas besoin d'un mur de modèles : il fallait les cinq que nos utilisateurs demandent.", role: "Opérateur automatisation" },
    ],
  },
  pt: {
    heroBadge: "Roteamento omnimodal para times de produto reais",
    heroLine1: "IA multimodal.",
    heroLine2: "Uma key",
    heroLine3: "é suficiente.",
    heroDescription:
      "Chame sem atrito modelos frontier como GPT-5, Claude, Seedance e ElevenLabs. Roteamento unificado, saldo pré-pago e um painel para controlar cobrança e orçamento.",
    primaryCta: "Obter API key",
    secondaryCta: "Comparar preços",
    modelTags: ["GPT-5 texto e código", "agentes Claude", "vídeo Seedance", "voz ElevenLabs", "raciocínio DeepSeek", "contexto longo Kimi", "stack GLM"],
    featuredModelsCta: "Explorar todos os modelos",
    officialPriceLabel: "Oficial",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Comparação com preço oficial",
    priceTitle: "Comparação de preços dos modelos",
    priceDescription: "Compare preços oficiais com o preço after-bonus da flatkey em texto, imagem, vídeo, voz e raciocínio.",
    priceTableNote: "O preço after-bonus combina descontos de modelo e crédito pré-pago.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "Respostas sobre modelos reais, bônus, endpoint único, roteamento e privacidade.",
    faqs: [
      { question: "São modelos oficiais reais?", answer: "Sim. A flatkey.ai roteia solicitações compatíveis para provedores upstream oficiais por um gateway gerenciado." },
      { question: "Como a economia chega a 50%?", answer: "Tarifas de modelo menores podem somar com bônus de recarga que adiciona saldo utilizável." },
      { question: "Preciso trocar SDKs?", answer: `Não. Mantenha seu cliente compatível com OpenAI e use ${API_BASE_URL} como base URL.` },
      { question: "Quais modalidades posso rotear com uma key?", answer: "A mesma conta flatkey cobre texto, agentes de código, raciocínio, imagem, vídeo, voz e outros modelos compatíveis." },
      { question: "Vocês armazenam prompts ou respostas?", answer: "Flatkey foi desenhado para retenção zero de conteúdo, com controles alinhados a GDPR, SOC 2 e ISO 27001." },
    ],
    voicesEyebrow: "Vozes de clientes",
    voicesTitle: "O que equipes dizem depois de centralizar gasto de modelos em uma key.",
    voices: [
      { quote: "Paramos de trocar de dashboard para validar custos de Kimi, GLM e Claude.", role: "Fundador de produto IA" },
      { quote: "Um endpoint para agentes de código, e financeiro consegue ver uso e recargas.", role: "Líder de engenharia" },
      { quote: "Não precisávamos de um mural de modelos; precisávamos dos cinco que usuários pedem.", role: "Operador de automação" },
    ],
  },
  ru: {
    heroBadge: "Омнимодальная маршрутизация для продуктовых команд",
    heroLine1: "Мультимодальный ИИ.",
    heroLine2: "Один ключ",
    heroLine3: "достаточно.",
    heroDescription:
      "Без лишней интеграции вызывайте GPT-5, Claude, Seedance, ElevenLabs и другие frontier models. Единый routing, prepaid balance и один dashboard для billing и budget.",
    primaryCta: "Получить API key",
    secondaryCta: "Сравнить цены",
    modelTags: ["GPT-5 text & code", "Claude agents", "Seedance video", "ElevenLabs voice", "DeepSeek reasoning", "Kimi long context", "GLM stack"],
    featuredModelsCta: "Смотреть все модели",
    officialPriceLabel: "Официально",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Сравнение с официальными ценами",
    priceTitle: "Сравнение цен моделей",
    priceDescription: "Сравните official prices и after-bonus цену flatkey для text, image, video, voice и reasoning вызовов.",
    priceTableNote: "After-bonus цена сочетает скидки моделей и prepaid recharge credit.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "Ответы про настоящие модели, бонусы, один endpoint, routing и privacy.",
    faqs: [
      { question: "Это настоящие официальные модели?", answer: "Да. flatkey.ai маршрутизирует поддерживаемые запросы к официальным upstream providers через managed API gateway." },
      { question: "Как экономия доходит до 50%?", answer: "Сниженные model rates складываются с bonus balance от платных пополнений." },
      { question: "Нужно менять SDK?", answer: `Нет. Оставьте OpenAI-compatible client и укажите ${API_BASE_URL} как base URL.` },
      { question: "Какие модальности можно маршрутизировать одним ключом?", answer: "Один аккаунт flatkey подходит для text, coding agents, reasoning, image, video, voice и других supported model traffic." },
      { question: "Вы храните prompts или completions?", answer: "Flatkey рассчитан на zero retention request content и controls aligned with GDPR, SOC 2, ISO 27001." },
    ],
    voicesEyebrow: "Отзывы клиентов",
    voicesTitle: "Что говорят команды после переноса model spend в один ключ.",
    voices: [
      { quote: "Мы перестали переключать dashboards, чтобы сверять затраты Kimi, GLM и Claude.", role: "Основатель AI-продукта" },
      { quote: "Один endpoint для coding agents, и финансы наконец видят usage и top-ups.", role: "Engineering lead" },
      { quote: "Нам нужны были не сотни логотипов, а пять моделей, которые спрашивают пользователи.", role: "Оператор automation platform" },
    ],
  },
  ja: {
    heroBadge: "プロダクトチーム向けの全モダリティルーティング",
    heroLine1: "全モーダルAI。",
    heroLine2: "1つのKey",
    heroLine3: "で十分。",
    heroDescription:
      "GPT-5、Claude、Seedance、ElevenLabs などの frontier model をシームレスに呼び出せます。統合ルーティング、プリペイド残高、請求と予算を管理する 1 つの dashboard。",
    primaryCta: "API key を取得",
    secondaryCta: "価格を比較",
    modelTags: ["GPT-5 テキストとコード", "Claude エージェント", "Seedance 動画", "ElevenLabs 音声", "DeepSeek 推論", "Kimi 長文脈", "GLM スタック"],
    featuredModelsCta: "すべてのモデルを見る",
    officialPriceLabel: "公式",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "公式価格との比較",
    priceTitle: "モデル価格比較",
    priceDescription: "テキスト、画像、動画、音声、推論の公式価格と flatkey の after-bonus 価格を比較します。",
    priceTableNote: "after-bonus 価格はモデル割引とプリペイド特典を組み合わせます。",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "本物のモデル、チャージ特典、1 endpoint、ルーティング、プライバシーについて。",
    faqs: [
      { question: "本物の公式モデルですか？", answer: "はい。flatkey.ai は対応リクエストを公式 upstream provider へ managed API gateway 経由でルーティングします。" },
      { question: "最大 50% オフはなぜ可能ですか？", answer: "低いモデル単価と有料チャージのボーナス残高が重ねがけされます。" },
      { question: "SDK の変更は必要ですか？", answer: `不要です。OpenAI 互換クライアントの base URL を ${API_BASE_URL} にします。` },
      { question: "1 つの Key でどのモダリティをルーティングできますか？", answer: "同じ flatkey アカウントで、テキスト、coding agent、推論、画像、動画、音声など対応モデルのトラフィックを扱えます。" },
      { question: "prompts や completions を保存しますか？", answer: "Flatkey は request content のゼロ保持を前提に設計し、GDPR、SOC 2、ISO 27001 に整合します。" },
    ],
    voicesEyebrow: "お客様の声",
    voicesTitle: "モデル支出を 1 key に集約したチームの声。",
    voices: [
      { quote: "Kimi、GLM、Claude のコスト確認で dashboard を行き来しなくなりました。", role: "AI プロダクト創業者" },
      { quote: "coding agent は 1 endpoint、財務は使用量とチャージ履歴を確認できます。", role: "エンジニアリング責任者" },
      { quote: "巨大なモデル一覧ではなく、ユーザーが毎週求める 5 つのモデルが必要でした。", role: "自動化プラットフォーム運営" },
    ],
  },
  vi: {
    heroBadge: "Routing toàn modality cho team sản phẩm thật",
    heroLine1: "AI đa phương thức.",
    heroLine2: "Một key",
    heroLine3: "là đủ.",
    heroDescription:
      "Gọi liền mạch GPT-5, Claude, Seedance, ElevenLabs và các frontier model khác. Routing thống nhất, một số dư trả trước, một dashboard để kiểm soát billing và budget.",
    primaryCta: "Lấy API key",
    secondaryCta: "So sánh giá",
    modelTags: ["GPT-5 text & code", "Claude agents", "Seedance video", "ElevenLabs voice", "DeepSeek reasoning", "Kimi long context", "GLM stack"],
    featuredModelsCta: "Khám phá tất cả model",
    officialPriceLabel: "Chính thức",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "So sánh với giá chính thức",
    priceTitle: "So sánh giá model",
    priceDescription: "So sánh giá chính thức với giá after-bonus của flatkey cho text, image, video, voice và reasoning.",
    priceTableNote: "Giá after-bonus kết hợp giảm giá model và credit nạp trả trước.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "Trả lời về model thật, bonus, một endpoint, routing và quyền riêng tư.",
    faqs: [
      { question: "Đây có phải model chính thức?", answer: "Có. flatkey.ai route request được hỗ trợ tới upstream provider chính thức qua managed API gateway." },
      { question: "Tiết kiệm tới 50% bằng cách nào?", answer: "Giá model thấp hơn có thể cộng với bonus balance từ nạp trả phí." },
      { question: "Có cần đổi SDK không?", answer: `Không. Giữ client tương thích OpenAI và đặt base URL là ${API_BASE_URL}.` },
      { question: "Một key route được những modality nào?", answer: "Cùng một tài khoản flatkey dùng cho text, coding agent, reasoning, image, video, voice và các model traffic được hỗ trợ." },
      { question: "Có lưu prompts hay completions không?", answer: "Flatkey được thiết kế zero retention nội dung request, với kiểm soát theo GDPR, SOC 2 và ISO 27001." },
    ],
    voicesEyebrow: "Khách hàng nói gì",
    voicesTitle: "Phản hồi khi team đưa chi phí model về một key.",
    voices: [
      { quote: "Chúng tôi không còn đổi dashboard để kiểm tra chi phí Kimi, GLM và Claude.", role: "Founder sản phẩm AI" },
      { quote: "Một endpoint cho coding agent, finance thấy được usage và lịch sử nạp.", role: "Trưởng nhóm kỹ thuật" },
      { quote: "Chúng tôi cần năm model người dùng hỏi mỗi tuần, không phải một bức tường model.", role: "Operator nền tảng automation" },
    ],
  },
  de: {
    heroBadge: "Omnimodales Routing für echte Produktteams",
    heroLine1: "Multimodale KI.",
    heroLine2: "Ein Key",
    heroLine3: "reicht.",
    heroDescription:
      "Rufe GPT-5, Claude, Seedance, ElevenLabs und weitere Frontier-Modelle nahtlos auf. Einheitliches Routing, Prepaid-Guthaben und ein Dashboard für Abrechnung und Budget.",
    primaryCta: "API Key holen",
    secondaryCta: "Preise vergleichen",
    modelTags: ["GPT-5 Text & Code", "Claude Agents", "Seedance Video", "ElevenLabs Voice", "DeepSeek Reasoning", "Kimi Long Context", "GLM Stack"],
    featuredModelsCta: "Alle Modelle ansehen",
    officialPriceLabel: "Offiziell",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Vergleich mit offiziellen Preisen",
    priceTitle: "Modellpreisvergleich",
    priceDescription: "Vergleiche offizielle Preise mit flatkeys after-bonus Preis für Text-, Bild-, Video-, Voice- und Reasoning-Aufrufe.",
    priceTableNote: "After-bonus Preise kombinieren Modellrabatte mit Prepaid-Aufladeguthaben.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "Antworten zu echten Modellen, Bonus, einem Endpoint, Routing und Datenschutz.",
    faqs: [
      { question: "Sind das echte offizielle Modelle?", answer: "Ja. flatkey.ai routet unterstützte Requests über ein managed API gateway zu offiziellen upstream providers." },
      { question: "Wie sind bis zu 50% Ersparnis möglich?", answer: "Niedrigere model rates können mit Bonusguthaben aus bezahlten Aufladungen kombiniert werden." },
      { question: "Muss ich SDKs ändern?", answer: `Nein. Behalte deinen OpenAI-compatible client und setze ${API_BASE_URL} als base URL.` },
      { question: "Welche Modalitäten kann ein Key routen?", answer: "Ein flatkey Account deckt Text, Coding Agents, Reasoning, Bild, Video, Voice und weiteren unterstützten Model Traffic ab." },
      { question: "Speichert ihr prompts oder completions?", answer: "Flatkey ist auf zero retention von request content ausgelegt und an GDPR, SOC 2 und ISO 27001 ausgerichtet." },
    ],
    voicesEyebrow: "Kundenstimmen",
    voicesTitle: "Was Teams sagen, nachdem Model Spend in einem Key liegt.",
    voices: [
      { quote: "Wir wechseln keine Dashboards mehr, um Kimi-, GLM- und Claude-Kosten zu prüfen.", role: "AI Product Founder" },
      { quote: "Ein Endpoint für Coding Agents, und Finance sieht Usage und Aufladungen.", role: "Engineering Lead" },
      { quote: "Wir brauchten nicht eine Modellwand, sondern die fünf Modelle, nach denen Nutzer fragen.", role: "Automation Operator" },
    ],
  },
  id: {
    heroBadge: "Routing omnimodal untuk tim produk nyata",
    heroLine1: "AI multimodal.",
    heroLine2: "Satu key",
    heroLine3: "sudah cukup.",
    heroDescription:
      "Panggil model frontier seperti GPT-5, Claude, Seedance, dan ElevenLabs dengan mulus. Routing terpadu, satu saldo prabayar, dan satu dashboard untuk mengontrol tagihan serta anggaran.",
    primaryCta: "Dapatkan API key",
    secondaryCta: "Bandingkan harga",
    modelTags: ["GPT-5 teks & kode", "Agen Claude", "Video Seedance", "Voice ElevenLabs", "Reasoning DeepSeek", "Kimi konteks panjang", "Stack GLM"],
    featuredModelsCta: "Lihat semua model",
    officialPriceLabel: "Resmi",
    flatkeyPriceLabel: "Flatkey",
    priceEyebrow: "Perbandingan harga model resmi",
    priceTitle: "Perbandingan harga model",
    priceDescription: "Bandingkan harga resmi dengan harga after-bonus flatkey untuk panggilan teks, gambar, video, suara, dan reasoning.",
    priceTableNote: "Harga after-bonus menggabungkan diskon model yang didukung dengan kredit top up prabayar.",
    faqEyebrow: "FAQ",
    faqTitle: "FAQ",
    faqDescription: "Jawaban langsung tentang model resmi, bonus top up, satu endpoint, routing produksi, dan privasi.",
    faqs: [
      { question: "Apakah ini model resmi yang nyata?", answer: "Ya. flatkey.ai merutekan request yang didukung ke provider upstream resmi melalui managed API gateway." },
      { question: "Bagaimana harga bisa hemat hingga 50%?", answer: "Tarif model yang didukung bisa lebih rendah dari daftar harga resmi, dan bonus top up berbayar menambah saldo yang bisa dipakai. Keduanya digabungkan." },
      { question: "Apakah saya perlu mengganti SDK?", answer: `Tidak. Tetap gunakan client kompatibel OpenAI dan arahkan base URL ke ${API_BASE_URL}.` },
      { question: "Modalitas apa saja yang bisa dirutekan dengan satu key?", answer: "Satu akun flatkey dapat dipakai untuk teks, coding agent, reasoning, gambar, video, suara, dan traffic model lain yang didukung." },
      { question: "Apakah prompts atau completions disimpan?", answer: "Flatkey dirancang untuk zero retention atas konten request, dengan kontrol yang selaras dengan GDPR, SOC 2, dan ISO 27001." },
    ],
    voicesEyebrow: "Suara pelanggan",
    voicesTitle: "Apa kata tim setelah memindahkan spend model ke satu key.",
    voices: [
      { quote: "Kami tidak lagi berpindah dashboard hanya untuk memeriksa biaya Kimi, GLM, dan Claude.", role: "Founder produk AI" },
      { quote: "Kemenangan pertama sederhana: satu endpoint untuk coding agent, lalu finance bisa melihat usage dan riwayat top up.", role: "Engineering lead" },
      { quote: "Kami tidak butuh dinding model raksasa; kami butuh lima model yang diminta pengguna setiap minggu.", role: "Operator platform otomasi" },
    ],
  },
});

type FocusModel = {
  name: string;
  match: RegExp;
  iconKey: string;
  positionClass: string;
};

const FOCUS_MODELS: FocusModel[] = [
  { name: "GPT-5", match: /gpt-5|^gpt|openai/i, iconKey: "openai", positionClass: "left-3 top-[27%] xl:left-[7%]" },
  { name: "Claude", match: /claude|anthropic/i, iconKey: "claude-color", positionClass: "right-3 top-[27%] xl:right-[8%]" },
  { name: "Gemini", match: /gemini|google/i, iconKey: "gemini-color", positionClass: "left-3 bottom-[36%] xl:left-[4%]" },
  { name: "Grok", match: /grok|xai/i, iconKey: "grok", positionClass: "right-3 bottom-[36%] xl:right-[4%]" },
  { name: "Kimi", match: /kimi|moonshot/i, iconKey: "kimi-color", positionClass: "left-[11%] bottom-[16%] xl:left-[17%]" },
  { name: "Seedance", match: /seedance|bytedance|doubao/i, iconKey: "bytedance-color", positionClass: "right-[11%] bottom-[16%] xl:right-[17%]" },
];

const HOT_MODEL_PATTERNS = [
  /gpt-5\.6|gpt-5\.5|gpt-5\.1|gpt-5/i,
  /claude-(sonnet|opus).*4\.5|claude.*4/i,
  /gemini-3|gemini-2\.5|gemini/i,
  /grok-4|grok/i,
  /seedance.*2|seedance/i,
  /elevenlabs|eleven/i,
  /deepseek-(v3|r1)|deepseek/i,
  /kimi-k2|kimi/i,
  /qwen3|qwen/i,
  /glm-5|glm-4\.6|glm/i,
  /veo|imagen|sora/i,
];

const FLOATING_LOGO_ENTRY_OFFSETS = [
  { x: "54vw", y: "24vh" },
  { x: "-54vw", y: "24vh" },
  { x: "44vw", y: "-34vh" },
  { x: "-44vw", y: "-34vh" },
  { x: "56vw", y: "-10vh" },
  { x: "-56vw", y: "-10vh" },
];

export async function HomePage(props: Props) {
  const baseCopy = getCopy(props.locale);
  const home = getHomeCopy(props.locale);
  const experience = EXPERIENCE_COPY[props.locale] ?? EXPERIENCE_COPY.en;
  const pricing = await getPricingData();
  const allRows = buildRowsForModels(pricing.models, pricing.vendors, pricing.groupRatio);
  const focusRows = pickFocusRows(allRows, pickFlagshipModels(pricing, FOCUS_MODELS.length), home.compare.official, home.compare.flatkey);
  const featuredRows = pickFeaturedRows(allRows, focusRows);
  const signUpUrl = consoleUrl("/sign-up", `redirect=/keys&lng=${props.locale}`);
  const ctaDescription = baseCopy.home.cta.description.replace("{{host}}", ROUTER_ORIGIN.replace(/^https?:\/\//, ""));

  return (
    <SiteShell locale={props.locale} pathname="/">
      <main data-fk-home-reveal-root className="fk-new-home relative overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <HeroSection experience={experience} locale={props.locale} signUpUrl={signUpUrl} focusRows={focusRows} />
        <HeroModelBannerSection rows={focusRows} locale={props.locale} />
        <PriceComparisonSection experience={experience} home={home} locale={props.locale} rows={featuredRows} />
        <VoicesSection experience={experience} />
        <FaqSection experience={experience} />
        <BottomCtaSection cta={baseCopy.home.cta} ctaDescription={ctaDescription} home={home} signUpUrl={signUpUrl} locale={props.locale} />
        <HomeSectionReveals />
      </main>
    </SiteShell>
  );
}

function HeroSection(props: {
  experience: ExperienceCopy;
  locale: Locale;
  signUpUrl: string;
  focusRows: HomePricedModel[];
}) {
  return (
    <section className="relative min-h-[100svh] overflow-hidden border-b-2 border-[#101014] px-4 py-4 sm:px-6 lg:py-5 dark:border-white/20">
      <div aria-hidden className="fk-hero-grid absolute inset-0" />
      <div aria-hidden className="fk-hero-wash absolute inset-x-0 top-0 h-full" />

      <div className="relative mx-auto min-h-[calc(100svh-2rem)] max-w-[2160px]">
        {FOCUS_MODELS.map((model, index) => {
          const row = props.focusRows[index] ?? fallbackRow(model, "Official", "After bonus");
          return <FloatingLogo key={model.name} model={model} row={row} index={index} locale={props.locale} />;
        })}

        <div className="fk-hero-content-reveal relative z-10 flex min-h-[calc(100svh-2rem)] w-full min-w-0 flex-col items-center justify-center text-center">
          <div className="fk-enter inline-flex w-full max-w-[calc(100vw-2rem)] min-w-0 items-center justify-center gap-2 rounded-full border-2 border-[#101014] bg-white/92 px-3 py-2 text-center text-[10px] leading-4 font-bold uppercase whitespace-normal shadow-[3px_3px_0_#101014] backdrop-blur sm:w-auto sm:px-4 sm:text-xs dark:border-white/25 dark:bg-[#111116]/82 dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
            <Sparkles className="size-3.5 text-[#7C3AED]" strokeWidth={2.6} />
            <span className="min-w-0">{props.experience.heroBadge}</span>
          </div>

          <h1 className="relative mt-4 w-full max-w-[calc(100vw-2rem)] min-w-0 text-[clamp(2.05rem,8.25vw,6.45rem)] leading-[1.01] font-extrabold tracking-normal text-balance sm:max-w-[1080px] sm:text-[clamp(2.6rem,5.6vw,6.45rem)]">
            <span className="fk-hero-title-line block">
              <BrandedTitleLine text={props.experience.heroLine1} />
            </span>
            <span className="fk-hero-title-line fk-enter-delay-1 block">
              <BrandedTitleLine text={props.experience.heroLine2} />
            </span>
            <span className="fk-hero-title-line fk-enter-delay-2 fk-hero-purple block">
              <BrandedTitleLine text={props.experience.heroLine3} />
            </span>
          </h1>

          <p className="fk-enter fk-hero-secondary-entry mt-4 w-full max-w-[calc(100vw-2rem)] min-w-0 rounded-[1.15rem] bg-[#F7F4EC]/72 px-4 py-2 text-[14.5px] leading-6 font-medium text-[#4D4D56] backdrop-blur sm:max-w-2xl sm:text-base dark:bg-[#050507]/62 dark:text-white/70">
            {props.experience.heroDescription}
          </p>

          <div className="fk-enter fk-hero-secondary-entry fk-hero-actions-entry mt-5 flex w-full max-w-[calc(100vw-2rem)] flex-col items-center justify-center gap-3 sm:w-auto sm:max-w-none sm:flex-row sm:flex-wrap">
            <a
              href={props.signUpUrl}
              className="fk-button-motion group inline-flex h-11 w-full max-w-[18rem] items-center justify-center rounded-full border-2 border-[#101014] !bg-[#101014] px-6 text-sm font-bold !text-white shadow-[4px_4px_0_#7C3AED] sm:w-auto sm:min-w-40 dark:border-white dark:!bg-white dark:!text-[#101014]"
            >
              {props.experience.primaryCta}
              <ArrowRight className="ml-2 size-4 transition-transform group-hover:translate-x-0.5" />
            </a>
            <Link
              href={localizePath("/pricing", props.locale)}
              className="fk-button-motion inline-flex h-11 w-full max-w-[18rem] items-center justify-center rounded-full border-2 border-[#101014] bg-[#F9F871] px-6 text-sm font-bold !text-[#101014] shadow-[4px_4px_0_#101014] sm:w-auto sm:min-w-40 dark:border-white/30"
            >
              {props.experience.secondaryCta}
            </Link>
          </div>

        </div>
      </div>
    </section>
  );
}

function BrandedTitleLine(props: { text: string }) {
  const { initial, rest } = splitFirstGrapheme(props.text);
  if (!initial) return props.text;
  return (
    <span className="fk-hero-title-initial-wrap" aria-label={props.text}>
      <span className="fk-hero-title-initial" aria-hidden="true">
        <span className="fk-hero-title-initial-char">{initial}</span>
      </span>
      <span className="fk-hero-title-rest-text" aria-hidden="true">{rest}</span>
    </span>
  );
}

function FloatingLogo(props: { model: FocusModel; row: HomePricedModel; index: number; locale: Locale }) {
  const entryOffset = FLOATING_LOGO_ENTRY_OFFSETS[props.index] ?? { x: "0", y: "0" };
  const href = localizePath(modelPublicPath(props.row.name), props.locale);
  return (
    <Link
      href={href}
      className={`fk-float-logo-entry fk-model-logo-link group absolute z-0 hidden lg:flex ${props.model.positionClass}`}
      style={
        {
          "--fk-logo-from-x": entryOffset.x,
          "--fk-logo-from-y": entryOffset.y,
          "--fk-logo-from-rotate": props.index % 2 === 0 ? "-10deg" : "10deg",
          animationDelay: "0ms",
        } as CSSProperties
      }
      aria-label={`Open ${props.row.name} model page`}
    >
      <div className="fk-float-logo" style={{ animationDelay: `${props.index * 0.28}s` }}>
        <div className="fk-hot-model-badge flex items-center gap-2.5 rounded-full border-2 border-[#101014] bg-[#FFFDF6]/94 px-3 py-2 shadow-[4px_4px_0_#101014] backdrop-blur dark:border-white/22 dark:bg-[#111116]/94 dark:shadow-[4px_4px_0_rgba(255,255,255,0.16)]">
          <ModelLogoSurface iconKey={props.model.iconKey} fallback={props.model.name.charAt(0)} size={30} className="size-10 rounded-full" />
          <span className="pr-1 font-mono text-[12px] font-extrabold tracking-normal text-[#101014] dark:text-white">{props.model.name}</span>
        </div>
        <HoverDog className="fk-model-hover-dog-floating" />
      </div>
    </Link>
  );
}

function splitFirstGrapheme(value: string): { initial: string; rest: string } {
  const [initial = "", ...rest] = Array.from(value.trimStart());
  return { initial, rest: rest.join("") };
}

function HeroModelBannerSection(props: { rows: HomePricedModel[]; locale: Locale }) {
  return (
    <section className="fk-section-reveal fk-section-reveal-compact fk-section-model-strip relative overflow-hidden border-b-2 border-[#101014] bg-[#FFFDF6] py-6 dark:border-white/20 dark:bg-[#0B0B10]">
      <HeroModelBanner rows={props.rows} locale={props.locale} />
    </section>
  );
}

function HeroModelBanner(props: { rows: HomePricedModel[]; locale: Locale }) {
  const bannerRows = props.rows.length > 0 ? props.rows : FOCUS_MODELS.map((model) => fallbackRow(model, "Official", "After bonus"));
  const items = [...bannerRows, ...bannerRows, ...bannerRows];
  return (
    <div className="home-model-marquee relative z-10 w-full overflow-hidden">
      <div className="pointer-events-none absolute inset-y-0 left-0 z-10 w-24 bg-gradient-to-r from-[#FFFDF6] to-transparent dark:from-[#0B0B10]" />
      <div className="pointer-events-none absolute inset-y-0 right-0 z-10 w-24 bg-gradient-to-l from-[#FFFDF6] to-transparent dark:from-[#0B0B10]" />
      <div className="home-model-marquee-track flex w-max items-center gap-14 px-10">
        {items.map((row, index) => (
          <HeroModelMark key={`${row.name}-${index}`} row={row} locale={props.locale} />
        ))}
      </div>
    </div>
  );
}

function HeroModelMark(props: { row: HomePricedModel; locale: Locale }) {
  const href = localizePath(modelPublicPath(props.row.name), props.locale);
  return (
    <Link
      href={href}
      className="fk-icon-motion fk-model-logo-link group relative flex h-16 w-28 shrink-0 items-center justify-center opacity-85 hover:opacity-100"
      title={props.row.name}
      aria-label={`Open ${props.row.name} model page`}
    >
      <ModelLogoSurface iconKey={props.row.iconKey} fallback={props.row.name.charAt(0).toUpperCase()} size={38} className="size-14 rounded-2xl" />
      <HoverDog />
    </Link>
  );
}

function HoverDog(props: { className?: string }) {
  return (
    <Image
      src="/assets/mascots/flatkey-mascot-run-transparent.png"
      alt=""
      width={128}
      height={128}
      className={`fk-model-hover-dog ${props.className ?? ""}`}
    />
  );
}

function ModelLogoSurface(props: { iconKey?: string; fallback: string; size: number; className: string }) {
  const darkSurface = needsDarkLogoSurface(props.iconKey);
  return (
    <span
      className={`flex shrink-0 items-center justify-center border shadow-[0_10px_24px_-18px_rgba(16,16,20,0.45)] ${
        darkSurface ? "border-white/18 bg-[#101014] text-white" : "border-[#101014]/10 bg-white text-[#101014]"
      } ${props.className}`}
    >
      <ModelLogo iconKey={props.iconKey} fallback={props.fallback} size={props.size} />
    </span>
  );
}

function needsDarkLogoSurface(iconKey?: string): boolean {
  return /(^|[-_.])(elevenlabs|openrouter|perplexity|moonshot|kimi)([-_.]|$)/i.test(iconKey ?? "");
}

function PriceComparisonSection(props: {
  experience: ExperienceCopy;
  home: ReturnType<typeof getHomeCopy>;
  locale: Locale;
  rows: HomePricedModel[];
}) {
  return (
    <section className="fk-section-scroll-right relative overflow-hidden border-b-2 border-[#101014] bg-[#FFFDF6] px-5 py-16 text-[#101014] sm:px-6 lg:px-8 lg:py-24 2xl:px-10 dark:border-white/20 dark:bg-[#0B0B10] dark:text-[#F6F3EA]">
      <div aria-hidden className="fk-hero-grid absolute inset-0 opacity-55" />
      <div className="relative mx-auto max-w-[1920px]">
        <div className="grid gap-5 lg:grid-cols-[0.88fr_1.12fr] lg:items-end">
          <div>
            <p className="font-mono text-xs font-bold uppercase text-[#7C3AED]">{props.experience.priceEyebrow}</p>
            <h2 className="mt-3 text-[2.35rem] leading-[1.1] font-extrabold tracking-normal sm:text-5xl">{props.experience.priceTitle}</h2>
          </div>
          <p className="max-w-2xl text-base leading-7 font-medium text-[#575762] sm:text-[17px] dark:text-white/68">{props.experience.priceDescription}</p>
        </div>

        <div className="mt-10 max-w-xl">
          <div className="fk-card-motion rounded-[1.35rem] border-2 border-[#101014] bg-[#5852FF] p-6 text-white shadow-[6px_6px_0_#101014] dark:border-white/20 dark:shadow-[6px_6px_0_rgba(255,255,255,0.14)]">
            <p className="font-mono text-xs font-bold uppercase text-white/64">{props.home.compare.spotlightBadge}</p>
            <div className="mt-4 text-5xl leading-none font-extrabold text-[#F9F871] sm:text-6xl">{props.home.compare.spotlightValue}</div>
            <p className="mt-4 text-sm leading-6 font-medium text-white/78">{props.home.compare.save}</p>
          </div>
        </div>

        <div className="mt-5 flex flex-wrap gap-2">
          {props.experience.modelTags.map((tag) => (
            <span key={tag} className="fk-chip-motion rounded-full border border-[#101014]/12 bg-white/78 px-3.5 py-2 text-sm font-semibold text-[#43434C] dark:border-white/12 dark:bg-white/8 dark:text-white/70">
              {tag}
            </span>
          ))}
        </div>

        <div className="mt-6 overflow-hidden rounded-[1.5rem] border-2 border-[#101014] bg-white/86 shadow-[8px_8px_0_#C8A8FF] backdrop-blur dark:border-white/20 dark:bg-white/8 dark:shadow-[8px_8px_0_rgba(124,58,237,0.45)]">
          <div className="grid grid-cols-[4.5rem_minmax(10rem,1.8fr)_minmax(6rem,0.9fr)_minmax(5.2rem,0.8fr)_minmax(4.4rem,0.75fr)_minmax(5.4rem,0.75fr)_1.4rem] items-center gap-4 border-b-2 border-[#101014] bg-[#F7F4EC] px-4 py-3 font-mono text-[11px] font-bold uppercase text-[#777782] max-md:hidden dark:border-white/20 dark:bg-white/6 dark:text-white/46">
            <span />
            <span>{props.experience.priceTitle}</span>
            <span>{props.experience.officialPriceLabel}</span>
            <span>{props.experience.flatkeyPriceLabel}</span>
            <span>{props.experience.officialPriceLabel}</span>
            <span>OFF</span>
            <span />
          </div>
          {props.rows.slice(0, 18).map((row, index) => (
            <FeaturedModelRow key={`${row.name}-${index}`} row={row} index={index} locale={props.locale} />
          ))}
        </div>

        <div className="mt-7 flex flex-col items-start justify-between gap-4 sm:flex-row sm:items-center">
          <p className="max-w-xl text-sm leading-6 font-medium text-[#6A6A75] dark:text-white/54">{props.experience.priceTableNote}</p>
          <Link
            href={localizePath("/models", props.locale)}
            className="fk-button-motion inline-flex h-11 items-center rounded-full border-2 border-[#101014] bg-[#5852FF] px-6 text-sm font-bold !text-white shadow-[4px_4px_0_#101014] dark:border-white/24 dark:shadow-[4px_4px_0_rgba(255,255,255,0.15)]"
          >
            {props.experience.featuredModelsCta}
          </Link>
        </div>
      </div>
    </section>
  );
}

function FeaturedModelRow(props: { row: HomePricedModel; index: number; locale: Locale }) {
  const discount = formatDiscount(props.row.official, props.row.discounted, props.index);
  const href = localizePath(modelPublicPath(props.row.name), props.locale);
  return (
    <Link href={href} className="fk-price-row grid min-h-16 grid-cols-[4.5rem_minmax(10rem,1.8fr)_minmax(6rem,0.9fr)_minmax(5.2rem,0.8fr)_minmax(4.4rem,0.75fr)_minmax(5.4rem,0.75fr)_4.3rem] items-center gap-4 border-b border-[#101014]/10 px-4 py-3.5 last:border-b-0 max-md:grid-cols-[4rem_minmax(0,1fr)_5.25rem] max-md:gap-3 dark:border-white/10" aria-label={`Open ${props.row.name} model page`}>
      <div className="flex h-12 w-16 items-center justify-center overflow-hidden rounded-xl">
        <ModelLogoSurface iconKey={props.row.iconKey} fallback={props.row.name.charAt(0).toUpperCase()} size={34} className="size-11 rounded-md" />
      </div>
      <div className="min-w-0">
        <div className="fk-price-row-text truncate text-[16px] leading-5 font-bold text-[#101014] dark:text-[#F6F3EA]">{props.row.name}</div>
      </div>
      <div className="fk-price-row-text truncate text-[13px] font-semibold text-[#7A7A85] max-md:hidden dark:text-white/46">{props.row.vendor}</div>
      <div className="fk-price-row-text fk-price-row-strong fk-price-row-price font-mono text-[24px] leading-none font-bold text-[#5852FF] max-md:text-right max-md:text-lg">{props.row.discounted}</div>
      <div className="fk-price-row-text font-mono text-[13px] font-semibold text-[#8C8C97] line-through max-md:hidden">{props.row.official}</div>
      <div className="fk-price-row-text fk-price-row-strong font-mono text-[15px] font-bold whitespace-nowrap text-[#15803D] max-md:hidden">{discount}% OFF</div>
      <div className="flex items-center justify-end gap-2 max-md:hidden">
        {[0.35, 0.5, 0.68, 0.85].map((height, barIndex) => (
          <span key={barIndex} className="w-[3px] rounded-full bg-[#22C55E]" style={{ height: `${22 * height}px` }} />
        ))}
        <span className="font-mono text-[11px] font-extrabold text-[#15803D]">99.9</span>
      </div>
    </Link>
  );
}

function FaqSection(props: { experience: ExperienceCopy }) {
  return (
    <section className="fk-section-scroll-right relative border-b-2 border-[#101014] px-5 py-16 sm:px-6 lg:px-8 lg:py-24 2xl:px-10 dark:border-white/20">
      <div aria-hidden className="fk-hero-grid absolute inset-0 opacity-65" />
      <div className="relative mx-auto max-w-[1920px]">
        <SectionHeader eyebrow={props.experience.faqEyebrow} title={props.experience.faqTitle} description={props.experience.faqDescription} />
        <div className="mt-10">
          <div className="grid gap-3 lg:grid-cols-2">
            {props.experience.faqs.map((faq, index) => (
              <article
                key={faq.question}
                className="fk-card-motion grid gap-4 rounded-[1.25rem] border-2 border-[#101014] bg-white/86 p-5 shadow-[4px_4px_0_#101014] backdrop-blur md:grid-cols-[3.5rem_1fr] dark:border-white/20 dark:bg-white/8 dark:shadow-[4px_4px_0_rgba(255,255,255,0.14)]"
              >
                <span className="flex size-12 items-center justify-center rounded-xl border-2 border-[#101014] bg-[#C8A8FF] font-mono text-sm font-bold text-[#101014] dark:border-white/20">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <div>
                  <h3 className="text-lg leading-snug font-bold tracking-normal">{faq.question}</h3>
                  <p className="mt-2 text-sm leading-6 font-medium text-[#5D5D68] dark:text-white/62">{faq.answer}</p>
                </div>
              </article>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function VoicesSection(props: { experience: ExperienceCopy }) {
  return (
    <section className="fk-section-scroll-right relative overflow-hidden border-b-2 border-[#101014] bg-[#F7F4EC] px-5 py-16 sm:px-6 lg:px-8 lg:py-24 2xl:px-10 dark:border-white/20 dark:bg-[#050507]">
      <div aria-hidden className="fk-hero-grid absolute inset-0 opacity-80" />
      <div aria-hidden className="fk-hero-wash absolute inset-x-0 top-0 h-full opacity-70" />
      <div className="relative mx-auto max-w-[1920px]">
        <SectionHeader eyebrow={props.experience.voicesEyebrow} title={props.experience.voicesTitle} />
        <div className="mt-10 grid gap-4 lg:grid-cols-3">
          {props.experience.voices.map((voice, index) => {
            const initial = voice.role.trim().charAt(0).toUpperCase();
            return (
              <article key={voice.role} className="fk-card-motion rounded-[1.35rem] border-2 border-[#101014] bg-white/86 p-6 text-[#101014] shadow-[6px_6px_0_#101014] backdrop-blur dark:border-white/20 dark:bg-white/8 dark:text-[#F6F3EA] dark:shadow-[6px_6px_0_rgba(255,255,255,0.14)]">
                <div className="mb-8 flex items-center gap-1 text-[#7C3AED]">
                  {Array.from({ length: 5 }).map((_, starIndex) => (
                    <Star key={starIndex} className="size-4 fill-current" strokeWidth={2.2} />
                  ))}
                </div>
                <p className="text-xl leading-8 font-bold tracking-normal">&ldquo;{voice.quote}&rdquo;</p>
                <div className="mt-8 flex items-center gap-3">
                  <span className={`flex size-11 items-center justify-center rounded-full border-2 border-[#101014] font-mono text-base font-bold text-[#101014] ${index === 0 ? "bg-[#92F2A2]" : index === 1 ? "bg-[#F9F871]" : "bg-[#C8A8FF]"}`}>
                    {initial}
                  </span>
                  <span className="font-mono text-xs leading-5 font-bold uppercase">{voice.role}</span>
                </div>
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function BottomCtaSection(props: {
  cta: ReturnType<typeof getCopy>["home"]["cta"];
  ctaDescription: string;
  home: ReturnType<typeof getHomeCopy>;
  signUpUrl: string;
  locale: Locale;
}) {
  return (
    <section className="fk-section-scroll-right fk-section-reveal-deep px-5 py-14 sm:px-6 lg:px-8 lg:py-20 2xl:px-10">
      <div className="mx-auto grid max-w-[2160px] gap-5 lg:grid-cols-[0.98fr_1.02fr] lg:items-stretch">
        <div className="fk-card-motion relative min-h-[360px] overflow-hidden rounded-[2rem] border-2 border-[#101014] bg-white/92 px-5 py-8 shadow-[7px_7px_0_#101014] sm:px-8 dark:border-white/18 dark:bg-white/8 dark:shadow-[7px_7px_0_rgba(255,255,255,0.14)]">
          <div aria-hidden className="fk-hero-grid absolute inset-0 opacity-45" />
          <div className="relative z-10">
            <p className="font-mono text-xs font-bold uppercase text-[#7C3AED]">Contact sales</p>
            <h2 className="mt-4 max-w-xl text-[clamp(2.3rem,4.3vw,4.7rem)] leading-[0.98] font-extrabold tracking-normal text-[#20242D] dark:text-[#F6F3EA]">
              Questions?
              <span className="block text-[#5852FF]">Talk to us.</span>
            </h2>
            <p className="mt-5 max-w-xl text-base leading-7 font-semibold text-[#575762] dark:text-white/62">{props.home.support.description}</p>
            <div className="mt-8 grid gap-3 sm:grid-cols-2">
              {props.home.stats.slice(0, 4).map((stat) => (
                <div key={stat.value} className="rounded-[1.05rem] border border-[#101014]/12 bg-[#FFFDF6]/82 p-4 dark:border-white/12 dark:bg-white/8">
                  <div className="font-mono text-2xl font-extrabold text-[#5852FF]">{stat.value}</div>
                  <div className="mt-2 text-sm leading-5 font-semibold text-[#777782] dark:text-white/54">{stat.label}</div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="fk-card-motion relative min-h-[360px] overflow-hidden rounded-[2rem] border-2 border-[#101014] bg-[#FFFDF6] p-5 shadow-[7px_7px_0_#C8A8FF] dark:border-white/18 dark:bg-white/8 dark:shadow-[7px_7px_0_rgba(124,58,237,0.42)]">
          <div className="mb-4">
            <p className="font-mono text-xs font-bold uppercase text-[#7C3AED]">Enterprise</p>
            <h2 className="mt-3 text-3xl leading-tight font-extrabold tracking-normal text-[#20242D] dark:text-[#F6F3EA]">
              {props.cta.titleLine1}
              <span className="block text-[#5852FF]">{props.cta.titleLine2}</span>
            </h2>
            <p className="mt-3 max-w-xl text-sm leading-6 font-semibold text-[#575762] dark:text-white/62">{props.ctaDescription}</p>
          </div>
          <FlatkeyTallyEmbed
            locale={props.locale}
            className="rounded-[1.2rem] bg-white/70 dark:bg-white/5"
            iframeClassName="block h-[430px] w-full border-0 bg-transparent sm:h-[400px] lg:h-[430px]"
          />
        </div>
      </div>
    </section>
  );
}

function SectionHeader(props: { eyebrow: string; title: string; description?: string; inverse?: boolean }) {
  return (
    <div className="max-w-3xl">
      <p className={`font-mono text-xs font-bold uppercase ${props.inverse ? "text-[#92F2A2]" : "text-[#7C3AED]"}`}>{props.eyebrow}</p>
      <h2 className="mt-3 text-4xl leading-[1.12] font-extrabold tracking-normal sm:text-5xl">{props.title}</h2>
      {props.description ? (
        <p className={`mt-5 text-base leading-7 font-medium ${props.inverse ? "text-white/68" : "text-[#575762] dark:text-white/68"}`}>
          {props.description}
        </p>
      ) : null}
    </div>
  );
}

function pickFocusRows(
  allRows: HomePricedModel[],
  flagshipRows: HomePricedModel[],
  officialLabel: string,
  discountedLabel: string
): HomePricedModel[] {
  const source = [...allRows, ...flagshipRows];
  return FOCUS_MODELS.map((focus) => {
    const row = source.find((candidate) => focus.match.test(candidate.name) || focus.match.test(candidate.vendor));
    return row ?? fallbackRow(focus, officialLabel, discountedLabel);
  });
}

function pickFeaturedRows(allRows: HomePricedModel[], focusRows: HomePricedModel[]): HomePricedModel[] {
  const seen = new Set<string>();
  const hotRows = allRows
    .map((row) => ({ row, score: hotModelScore(row) }))
    .filter((entry) => entry.score >= 0)
    .sort((a, b) => a.score - b.score)
    .map((entry) => entry.row);
  const source = [...focusRows, ...hotRows, ...allRows];
  const selected: HomePricedModel[] = [];
  for (const row of source) {
    if (seen.has(row.name)) continue;
    seen.add(row.name);
    selected.push(row);
    if (selected.length >= 16) break;
  }
  return selected;
}

function hotModelScore(row: HomePricedModel): number {
  const haystack = `${row.name} ${row.vendor}`;
  const score = HOT_MODEL_PATTERNS.findIndex((pattern) => pattern.test(haystack));
  return score;
}

function formatDiscount(official: string, discounted: string, index: number): number {
  const officialValue = parsePrice(official);
  const discountedValue = parsePrice(discounted);
  if (officialValue > 0 && discountedValue > 0 && discountedValue < officialValue) {
    return Math.max(1, Math.round((1 - discountedValue / officialValue) * 100));
  }
  const fallback = [36, 20, 73, 38, 75, 20, 40, 24, 20, 57, 64, 20, 20, 40, 20, 40];
  return fallback[index % fallback.length];
}

function parsePrice(value: string): number {
  const match = value.match(/\$([0-9]+(?:\.[0-9]+)?)/);
  return match ? Number(match[1]) : 0;
}

function fallbackRow(model: FocusModel, officialLabel: string, discountedLabel: string): HomePricedModel {
  return {
    name: model.name,
    vendor: model.name,
    official: officialLabel,
    discounted: discountedLabel,
    iconKey: model.iconKey,
  };
}
