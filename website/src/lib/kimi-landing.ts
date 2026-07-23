import type { SeoInput } from "@/lib/seo";
import { withIdFallback } from "@/lib/locales";
import type { Locale } from "./locales";
import { buildConsoleUrl, APP_CONSOLE_ORIGIN } from "./origins";
import { normalizeModelKey } from "./model-public";
import type { PricingModel } from "./pricing";

// Paid-search keyword-echo landing page for "kimi 3.0" campaigns.
// Same convention as /glm-5-2 (keyword "glm 5.2" → route /glm-5-2):
// keyword "kimi 3.0" → route /kimi-3-0, and the H1 echoes "Kimi 3.0 API".
// The upstream model id for Kimi 3.0 on the flatkey router is `kimi-k3`.

export const KIMI_LANDING_PATH = "/kimi-3-0";
export const KIMI_MODEL_ID = "kimi-k3";

// Family shown in the model table. Ids must match the router/pricing catalog.
export const KIMI_FAMILY_MODEL_IDS = ["kimi-k3", "kimi-k2.6", "kimi-k2.5"] as const;

export type KimiFamilyModelId = (typeof KIMI_FAMILY_MODEL_IDS)[number];

export type KimiLandingFeature = {
  title: string;
  body: string;
};

export type KimiLandingFaq = {
  question: string;
  answer: string;
};

export type KimiFamilyRow = {
  modelId: KimiFamilyModelId;
  name: string;
  bestFor: string;
};

export type KimiLandingPageCopy = {
  seo: {
    title: string;
    description: string;
  };
  badge: string;
  hero: {
    eyebrow: string;
    /** H1 = `${title} ${highlight}` and must echo the ad keyword exactly. */
    title: string;
    highlight: string;
    subtitle: string;
    primaryCta: string;
    secondaryCta: string;
    trustLine: string;
  };
  code: {
    kicker: string;
    title: string;
    subtitle: string;
    terminalTitle: string;
    sdkTab: string;
    curlTab: string;
    model: string;
  };
  family: {
    kicker: string;
    title: string;
    subtitle: string;
    modelColumn: string;
    bestForColumn: string;
    inputColumn: string;
    outputColumn: string;
    rows: KimiFamilyRow[];
    livePriceNote: string;
    priceUnavailable: string;
  };
  reasonsKicker: string;
  reasonsTitle: string;
  reasons: KimiLandingFeature[];
  featuresKicker: string;
  featuresTitle: string;
  features: KimiLandingFeature[];
  finalCta: {
    title: string;
    body: string;
    button: string;
  };
  faqs: KimiLandingFaq[];
};

const english: KimiLandingPageCopy = {
  seo: {
    title: "Kimi 3.0 API — official kimi-k3 access, OpenAI-compatible",
    description:
      "Call the Kimi 3.0 API (model kimi-k3, Moonshot's reasoning model) through flatkey.ai: official upstream access with no mainland-China account, an OpenAI-compatible base_url swap, an instant API key, and prepaid credit to start small.",
  },
  badge: "kimi-k3 live on flatkey",
  hero: {
    eyebrow: "Moonshot Kimi 3.0 for builders outside mainland China",
    title: "Kimi 3.0",
    highlight: "API",
    subtitle:
      "Official direct access to kimi-k3 — Moonshot's reasoning model. No mainland-China account or phone number: swap one OpenAI-compatible base_url, get an instant key, and start small with prepaid credit.",
    primaryCta: "Get your Kimi 3.0 key",
    secondaryCta: "See live pricing",
    trustLine: "No card to start · OpenAI-compatible · instant key",
  },
  code: {
    kicker: "Drop-in code",
    title: "Switch with one base URL",
    subtitle: "Keep the OpenAI SDK or plain curl. Point traffic at flatkey and set the model to kimi-k3.",
    terminalTitle: "API example",
    sdkTab: "OpenAI SDK",
    curlTab: "curl",
    model: KIMI_MODEL_ID,
  },
  family: {
    kicker: "Model family",
    title: "One key, the whole Kimi family",
    subtitle: "Swap the model id to trade reasoning depth for cost — no new account, no new SDK.",
    modelColumn: "Model",
    bestForColumn: "Best for",
    inputColumn: "Input / 1M",
    outputColumn: "Output / 1M",
    rows: [
      { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "Flagship reasoning — hard problems, agents, long context" },
      { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Agentic coding and tool use" },
      { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "Everyday chat and high-volume workloads" },
    ],
    livePriceNote: "Live list prices from the flatkey pricing API — see the pricing page for current rates and top-up bonuses.",
    priceUnavailable: "See pricing",
  },
  reasonsKicker: "Why flatkey for Kimi 3.0",
  reasonsTitle: "Two reasons, no catch",
  reasons: [
    {
      title: "Official upstream, no mainland account",
      body: "flatkey fronts the official Kimi upstream for you. Sign up with Google or GitHub — no Chinese phone number, local ID, or RMB billing account required.",
    },
    {
      title: "Start small with prepaid credit",
      body: "Top up a few dollars, get an instant key, and watch spend live in the console. A prepaid balance keeps the bill bounded while you evaluate kimi-k3.",
    },
  ],
  featuresKicker: "Built for developers arriving from search",
  featuresTitle: "Everything a developer checks before trying a model",
  features: [
    { title: "OpenAI-compatible", body: "Your existing OpenAI SDK works as-is: change base_url and api_key, set the model to kimi-k3, keep everything else." },
    { title: "Reasoning model", body: "kimi-k3 is Moonshot's reasoning-class flagship — built for multi-step problems, agent loops, and long context." },
    { title: "Instant key", body: "Sign up and create an API key in minutes. No sales call, no approval queue." },
    { title: "Pay in USD or local methods", body: "Simple top-ups with international cards or local payment methods — no RMB account or cross-border transfers." },
    { title: "40+ other models", body: "The same key also calls GPT, Claude, Gemini, DeepSeek, GLM, Qwen, video, and image models." },
    { title: "Unified billing + analytics", body: "One prepaid balance, per-key usage analytics, and a single invoice across every model." },
  ],
  finalCta: {
    title: "Ship with Kimi 3.0 without reworking your stack",
    body: "Create one flatkey API key, point your OpenAI-compatible client at our router, and make your first kimi-k3 call in minutes.",
    button: "Get your Kimi 3.0 key",
  },
  faqs: [
    {
      question: "Is this the official Kimi 3.0 model?",
      answer:
        "Yes — flatkey routes the kimi-k3 model id to the official upstream. Confirm live availability and current pricing in the console before production rollout.",
    },
    {
      question: "Do I need a mainland-China account or phone number?",
      answer:
        "No. Sign up with Google or GitHub, top up in USD or local payment methods, and call kimi-k3 immediately — flatkey handles the upstream account for you.",
    },
    {
      question: "Do I need to rewrite my OpenAI SDK code?",
      answer:
        "No. Use the OpenAI-compatible base URL, keep your SDK, and set the model to kimi-k3. kimi-k2.6 and kimi-k2.5 work the same way.",
    },
  ],
};

const translations: Record<Locale, KimiLandingPageCopy> = withIdFallback({
  en: english,
  zh: {
    ...english,
    seo: {
      title: "Kimi 3.0 API — 官方 kimi-k3 直连，兼容 OpenAI",
      description:
        "通过 flatkey.ai 调用 Kimi 3.0 API（模型 kimi-k3，月之暗面推理模型）：官方上游直连，无需中国大陆账号，兼容 OpenAI 只改 base_url，即时发 key，预充值小额起步。",
    },
    badge: "kimi-k3 已上线 flatkey",
    hero: {
      eyebrow: "面向海外开发者的月之暗面 Kimi 3.0",
      title: "Kimi 3.0",
      highlight: "API",
      subtitle:
        "官方直连 kimi-k3 —— 月之暗面的推理模型。无需中国大陆账号或手机号：改一行 OpenAI 兼容 base_url，即时拿到 key，预充值小额起步。",
      primaryCta: "获取 Kimi 3.0 key",
      secondaryCta: "查看实时价格",
      trustLine: "无需信用卡开始 · 兼容 OpenAI · 即时发 key",
    },
    code: {
      kicker: "一行接入",
      title: "只改 base URL 即可切换",
      subtitle: "保留 OpenAI SDK 或直接用 curl，把流量指向 flatkey，并把 model 设为 kimi-k3。",
      terminalTitle: "API 示例",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "模型家族",
      title: "一把 key，整个 Kimi 家族",
      subtitle: "换一个 model id 就能在推理深度和成本之间切换——不用新账号，不用新 SDK。",
      modelColumn: "模型",
      bestForColumn: "适用场景",
      inputColumn: "输入 / 1M",
      outputColumn: "输出 / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "旗舰推理——复杂问题、Agent、长上下文" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Agent 编程与工具调用" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "日常对话与大批量调用" },
      ],
      livePriceNote: "价格来自 flatkey 实时定价 API——当前费率与充值赠额见定价页。",
      priceUnavailable: "查看价格",
    },
    reasonsKicker: "为什么用 flatkey 调 Kimi 3.0",
    reasonsTitle: "两个理由，没有套路",
    reasons: [
      {
        title: "官方上游，无需大陆账号",
        body: "flatkey 替你对接官方 Kimi 上游。用 Google 或 GitHub 注册即可——不需要中国手机号、本地身份证或人民币账户。",
      },
      {
        title: "预充值小额起步",
        body: "充几美元就能即时拿到 key，在控制台实时查看消耗。预充值余额让你在评估 kimi-k3 时账单始终可控。",
      },
    ],
    featuresKicker: "为搜索而来的开发者设计",
    featuresTitle: "试模型之前开发者要确认的信息，一屏讲清",
    features: [
      { title: "兼容 OpenAI", body: "现有 OpenAI SDK 原样可用：改 base_url 和 api_key，把 model 设为 kimi-k3，其余不动。" },
      { title: "推理模型", body: "kimi-k3 是月之暗面的旗舰推理模型——面向多步推理、Agent 循环和长上下文。" },
      { title: "即时发 key", body: "注册后几分钟内创建 API key。不用约销售，不用排队审批。" },
      { title: "美元或本地支付", body: "国际卡或本地支付方式轻松充值——无需人民币账户或跨境转账。" },
      { title: "40+ 其它模型", body: "同一把 key 还能调用 GPT、Claude、Gemini、DeepSeek、GLM、Qwen、视频和图像模型。" },
      { title: "统一账单与分析", body: "一个预充值余额、按 key 的用量分析、所有模型一张账单。" },
    ],
    finalCta: {
      title: "不用重写技术栈，也能用上 Kimi 3.0",
      body: "创建一把 flatkey API key，把 OpenAI 兼容客户端指向我们的 router，几分钟内完成第一次 kimi-k3 调用。",
      button: "获取 Kimi 3.0 key",
    },
    faqs: [
      {
        question: "这是官方的 Kimi 3.0 模型吗？",
        answer: "是的——flatkey 把 kimi-k3 这个模型 id 路由到官方上游。生产上线前请在控制台确认实时可用性与当前价格。",
      },
      {
        question: "需要中国大陆账号或手机号吗？",
        answer: "不需要。用 Google 或 GitHub 注册，用美元或本地方式充值，即可立即调用 kimi-k3——上游账号由 flatkey 处理。",
      },
      {
        question: "需要重写 OpenAI SDK 代码吗？",
        answer: "不需要。使用兼容 OpenAI 的 base URL，保留 SDK，把 model 设为 kimi-k3。kimi-k2.6 和 kimi-k2.5 用法相同。",
      },
    ],
  },
  es: {
    ...english,
    seo: {
      title: "Kimi 3.0 API — acceso oficial a kimi-k3, compatible con OpenAI",
      description:
        "Usa la Kimi 3.0 API (modelo kimi-k3, el modelo de razonamiento de Moonshot) con flatkey.ai: acceso oficial sin cuenta de China continental, cambio de base_url compatible con OpenAI, clave instantánea y crédito prepago para empezar en pequeño.",
    },
    badge: "kimi-k3 disponible en flatkey",
    hero: {
      eyebrow: "Moonshot Kimi 3.0 para desarrolladores fuera de China",
      title: "Kimi 3.0",
      highlight: "API",
      subtitle:
        "Acceso directo oficial a kimi-k3, el modelo de razonamiento de Moonshot. Sin cuenta ni teléfono de China continental: cambia una base_url compatible con OpenAI, obtén una clave al instante y empieza en pequeño con crédito prepago.",
      primaryCta: "Obtén tu clave Kimi 3.0",
      secondaryCta: "Ver precios en vivo",
      trustLine: "Sin tarjeta para empezar · compatible con OpenAI · clave instantánea",
    },
    code: {
      kicker: "Código directo",
      title: "Cambia solo la base URL",
      subtitle: "Mantén el SDK de OpenAI o usa curl. Envía el tráfico a flatkey y define el modelo kimi-k3.",
      terminalTitle: "Ejemplo API",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "Familia de modelos",
      title: "Una clave, toda la familia Kimi",
      subtitle: "Cambia el id del modelo para equilibrar profundidad de razonamiento y coste — sin nueva cuenta ni nuevo SDK.",
      modelColumn: "Modelo",
      bestForColumn: "Ideal para",
      inputColumn: "Entrada / 1M",
      outputColumn: "Salida / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "Razonamiento insignia — problemas difíciles, agentes, contexto largo" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Programación agéntica y uso de herramientas" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "Chat diario y cargas de alto volumen" },
      ],
      livePriceNote: "Precios de lista en vivo desde la API de precios de flatkey — consulta la página de precios para tarifas y bonos de recarga.",
      priceUnavailable: "Ver precios",
    },
    reasonsKicker: "Por qué flatkey para Kimi 3.0",
    reasonsTitle: "Dos razones, sin trampa",
    reasons: [
      {
        title: "Upstream oficial, sin cuenta china",
        body: "flatkey gestiona el upstream oficial de Kimi por ti. Regístrate con Google o GitHub — sin teléfono chino, identificación local ni cuenta en RMB.",
      },
      {
        title: "Empieza en pequeño con crédito prepago",
        body: "Recarga unos dólares, obtén una clave al instante y sigue el gasto en vivo en la consola. El saldo prepago mantiene la factura acotada mientras evalúas kimi-k3.",
      },
    ],
    featuresKicker: "Hecho para desarrolladores que llegan desde el buscador",
    featuresTitle: "Lo que un desarrollador revisa antes de probar un modelo",
    features: [
      { title: "Compatible con OpenAI", body: "Tu SDK de OpenAI funciona tal cual: cambia base_url y api_key, define el modelo kimi-k3 y no toques nada más." },
      { title: "Modelo de razonamiento", body: "kimi-k3 es el buque insignia de razonamiento de Moonshot — para problemas de varios pasos, bucles de agentes y contexto largo." },
      { title: "Clave instantánea", body: "Regístrate y crea una clave API en minutos. Sin llamadas de ventas ni colas de aprobación." },
      { title: "Paga en USD o métodos locales", body: "Recargas simples con tarjetas internacionales o métodos locales — sin cuenta en RMB ni transferencias transfronterizas." },
      { title: "40+ modelos más", body: "La misma clave también llama a GPT, Claude, Gemini, DeepSeek, GLM, Qwen, video e imagen." },
      { title: "Facturación y analítica unificadas", body: "Un saldo prepago, analítica de uso por clave y una sola factura para todos los modelos." },
    ],
    finalCta: {
      title: "Lanza con Kimi 3.0 sin rehacer tu stack",
      body: "Crea una clave flatkey, apunta tu cliente compatible con OpenAI a nuestro router y haz tu primera llamada kimi-k3 en minutos.",
      button: "Obtén tu clave Kimi 3.0",
    },
    faqs: [
      {
        question: "¿Es el modelo oficial Kimi 3.0?",
        answer: "Sí — flatkey enruta el id kimi-k3 al upstream oficial. Confirma la disponibilidad y el precio actual en la consola antes de producción.",
      },
      {
        question: "¿Necesito cuenta o teléfono de China continental?",
        answer: "No. Regístrate con Google o GitHub, recarga en USD o métodos locales y llama a kimi-k3 de inmediato — flatkey gestiona la cuenta upstream.",
      },
      {
        question: "¿Debo reescribir mi código del SDK de OpenAI?",
        answer: "No. Usa la base URL compatible con OpenAI, conserva el SDK y define el modelo kimi-k3. kimi-k2.6 y kimi-k2.5 funcionan igual.",
      },
    ],
  },
  fr: {
    ...english,
    seo: {
      title: "Kimi 3.0 API — accès officiel à kimi-k3, compatible OpenAI",
      description:
        "Utilisez l'API Kimi 3.0 (modèle kimi-k3, le modèle de raisonnement de Moonshot) via flatkey.ai : accès officiel sans compte en Chine continentale, simple changement de base_url compatible OpenAI, clé instantanée et crédit prépayé pour commencer petit.",
    },
    badge: "kimi-k3 disponible sur flatkey",
    hero: {
      eyebrow: "Moonshot Kimi 3.0 pour les développeurs hors de Chine",
      title: "Kimi 3.0",
      highlight: "API",
      subtitle:
        "Accès direct officiel à kimi-k3, le modèle de raisonnement de Moonshot. Sans compte ni numéro chinois : changez une base_url compatible OpenAI, obtenez une clé instantanée et commencez petit avec du crédit prépayé.",
      primaryCta: "Obtenir votre clé Kimi 3.0",
      secondaryCta: "Voir les prix en direct",
      trustLine: "Pas de carte au départ · compatible OpenAI · clé instantanée",
    },
    code: {
      kicker: "Code direct",
      title: "Changez seulement la base URL",
      subtitle: "Gardez le SDK OpenAI ou utilisez curl. Dirigez le trafic vers flatkey et définissez le modèle kimi-k3.",
      terminalTitle: "Exemple API",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "Famille de modèles",
      title: "Une clé, toute la famille Kimi",
      subtitle: "Changez l'id du modèle pour arbitrer entre profondeur de raisonnement et coût — sans nouveau compte ni nouveau SDK.",
      modelColumn: "Modèle",
      bestForColumn: "Idéal pour",
      inputColumn: "Entrée / 1M",
      outputColumn: "Sortie / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "Raisonnement phare — problèmes difficiles, agents, long contexte" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Codage agentique et usage d'outils" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "Chat quotidien et gros volumes" },
      ],
      livePriceNote: "Prix catalogue en direct depuis l'API de tarification flatkey — voir la page tarifs pour les taux actuels et bonus de recharge.",
      priceUnavailable: "Voir les prix",
    },
    reasonsKicker: "Pourquoi flatkey pour Kimi 3.0",
    reasonsTitle: "Deux raisons, sans piège",
    reasons: [
      {
        title: "Upstream officiel, sans compte chinois",
        body: "flatkey gère l'upstream officiel de Kimi pour vous. Inscrivez-vous avec Google ou GitHub — sans numéro chinois, pièce d'identité locale ni compte en RMB.",
      },
      {
        title: "Commencez petit avec du crédit prépayé",
        body: "Rechargez quelques dollars, obtenez une clé instantanée et suivez la dépense en direct dans la console. Le solde prépayé borne la facture pendant que vous évaluez kimi-k3.",
      },
    ],
    featuresKicker: "Conçu pour les développeurs venus de la recherche",
    featuresTitle: "Ce qu'un développeur vérifie avant d'essayer un modèle",
    features: [
      { title: "Compatible OpenAI", body: "Votre SDK OpenAI fonctionne tel quel : changez base_url et api_key, définissez le modèle kimi-k3, gardez le reste." },
      { title: "Modèle de raisonnement", body: "kimi-k3 est le fleuron de raisonnement de Moonshot — pour les problèmes multi-étapes, les boucles d'agents et le long contexte." },
      { title: "Clé instantanée", body: "Inscrivez-vous et créez une clé API en quelques minutes. Pas d'appel commercial, pas de file d'approbation." },
      { title: "Paiement USD ou local", body: "Recharges simples par carte internationale ou méthodes locales — sans compte RMB ni virement transfrontalier." },
      { title: "40+ autres modèles", body: "La même clé appelle aussi GPT, Claude, Gemini, DeepSeek, GLM, Qwen, vidéo et image." },
      { title: "Facturation et analytique unifiées", body: "Un solde prépayé, une analytique d'usage par clé et une seule facture pour tous les modèles." },
    ],
    finalCta: {
      title: "Lancez Kimi 3.0 sans refaire votre stack",
      body: "Créez une clé flatkey, pointez votre client compatible OpenAI vers notre routeur et passez votre premier appel kimi-k3 en quelques minutes.",
      button: "Obtenir votre clé Kimi 3.0",
    },
    faqs: [
      {
        question: "Est-ce le modèle officiel Kimi 3.0 ?",
        answer: "Oui — flatkey route l'id kimi-k3 vers l'upstream officiel. Vérifiez la disponibilité et le prix actuel dans la console avant la production.",
      },
      {
        question: "Faut-il un compte ou un numéro de Chine continentale ?",
        answer: "Non. Inscrivez-vous avec Google ou GitHub, rechargez en USD ou méthodes locales et appelez kimi-k3 immédiatement — flatkey gère le compte upstream.",
      },
      {
        question: "Dois-je réécrire mon code SDK OpenAI ?",
        answer: "Non. Utilisez la base URL compatible OpenAI, gardez le SDK et définissez le modèle kimi-k3. kimi-k2.6 et kimi-k2.5 fonctionnent pareil.",
      },
    ],
  },
  pt: {
    ...english,
    seo: {
      title: "API Kimi 3.0 — acesso oficial ao kimi-k3, compatível com OpenAI",
      description:
        "Use a API Kimi 3.0 (modelo kimi-k3, o modelo de raciocínio da Moonshot) pela flatkey.ai: acesso oficial sem conta na China continental, troca de base_url compatível com OpenAI, chave instantânea e crédito pré-pago para começar pequeno.",
    },
    badge: "kimi-k3 no ar na flatkey",
    hero: {
      eyebrow: "Moonshot Kimi 3.0 para devs fora da China continental",
      title: "API",
      highlight: "Kimi 3.0",
      subtitle:
        "Acesso direto oficial ao kimi-k3 — o modelo de raciocínio da Moonshot. Sem conta nem telefone da China continental: troque uma base_url compatível com OpenAI, receba a chave na hora e comece pequeno com crédito pré-pago.",
      primaryCta: "Pegar minha chave Kimi 3.0",
      secondaryCta: "Ver preços ao vivo",
      trustLine: "Sem cartão para começar · compatível com OpenAI · recarregue com Pix",
    },
    code: {
      kicker: "Código direto",
      title: "Troque apenas a base URL",
      subtitle: "Mantenha o SDK OpenAI ou use curl. Aponte o tráfego para a flatkey e defina o modelo kimi-k3.",
      terminalTitle: "Exemplo API",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "Família de modelos",
      title: "Uma chave, toda a família Kimi",
      subtitle: "Troque o id do modelo para equilibrar profundidade de raciocínio e custo — sem conta nova, sem SDK novo.",
      modelColumn: "Modelo",
      bestForColumn: "Ideal para",
      inputColumn: "Entrada / 1M",
      outputColumn: "Saída / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "Raciocínio de ponta — problemas difíceis, agentes, contexto longo" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Programação agêntica e uso de ferramentas" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "Chat do dia a dia e alto volume" },
      ],
      livePriceNote: "Preços de lista ao vivo da API de preços da flatkey — veja a página de preços para tarifas atuais e bônus de recarga.",
      priceUnavailable: "Ver preços",
    },
    reasonsKicker: "Por que flatkey para o Kimi 3.0",
    reasonsTitle: "Dois motivos, sem pegadinha",
    reasons: [
      {
        title: "Upstream oficial, sem conta chinesa",
        body: "A flatkey cuida do upstream oficial do Kimi por você. Cadastre-se com Google ou GitHub — sem telefone chinês, documento local ou conta em RMB.",
      },
      {
        title: "Comece pequeno com crédito pré-pago",
        body: "Recarregue alguns dólares, receba a chave na hora e acompanhe o gasto ao vivo no console. O saldo pré-pago mantém a conta sob controle enquanto você avalia o kimi-k3.",
      },
    ],
    featuresKicker: "Feito para devs que chegam pela busca",
    featuresTitle: "Tudo que um dev confere antes de testar um modelo",
    features: [
      { title: "Compatível com OpenAI", body: "Seu SDK OpenAI funciona como está: troque base_url e api_key, defina o modelo kimi-k3 e mantenha o resto." },
      { title: "Modelo de raciocínio", body: "kimi-k3 é o carro-chefe de raciocínio da Moonshot — para problemas de várias etapas, loops de agentes e contexto longo." },
      { title: "Chave instantânea", body: "Cadastre-se e crie uma chave API em minutos. Sem call de vendas, sem fila de aprovação." },
      { title: "Pague com Pix, USD ou cartão", body: "Recargas simples com Pix, cartão internacional ou outros métodos locais — sem conta em RMB nem transferência internacional." },
      { title: "40+ outros modelos", body: "A mesma chave também chama GPT, Claude, Gemini, DeepSeek, GLM, Qwen, vídeo e imagem." },
      { title: "Cobrança e analytics unificados", body: "Um saldo pré-pago, analytics de uso por chave e uma única fatura para todos os modelos." },
    ],
    finalCta: {
      title: "Lance com o Kimi 3.0 sem refazer seu stack",
      body: "Crie uma chave flatkey, aponte seu cliente compatível com OpenAI para nosso router e faça sua primeira chamada kimi-k3 em minutos.",
      button: "Pegar minha chave Kimi 3.0",
    },
    faqs: [
      {
        question: "É o modelo oficial Kimi 3.0?",
        answer: "Sim — a flatkey roteia o id kimi-k3 para o upstream oficial. Confirme a disponibilidade e o preço atual no console antes de ir para produção.",
      },
      {
        question: "Preciso de conta ou telefone da China continental?",
        answer: "Não. Cadastre-se com Google ou GitHub, recarregue com Pix, USD ou outros métodos locais e chame o kimi-k3 na hora — a flatkey cuida da conta upstream.",
      },
      {
        question: "Preciso reescrever meu código do SDK OpenAI?",
        answer: "Não. Use a base URL compatível com OpenAI, mantenha o SDK e defina o modelo kimi-k3. kimi-k2.6 e kimi-k2.5 funcionam do mesmo jeito.",
      },
    ],
  },
  ru: {
    ...english,
    seo: {
      title: "Kimi 3.0 API — официальный доступ к kimi-k3, OpenAI-compatible",
      description:
        "Вызывайте Kimi 3.0 API (модель kimi-k3, reasoning-модель Moonshot) через flatkey.ai: официальный upstream без аккаунта в материковом Китае, замена base_url в OpenAI-совместимом клиенте, мгновенный ключ и предоплаченный кредит для небольшого старта.",
    },
    badge: "kimi-k3 доступна на flatkey",
    hero: {
      eyebrow: "Moonshot Kimi 3.0 для разработчиков за пределами Китая",
      title: "Kimi 3.0",
      highlight: "API",
      subtitle:
        "Официальный прямой доступ к kimi-k3 — reasoning-модели Moonshot. Без аккаунта и телефона материкового Китая: поменяйте одну OpenAI-совместимую base_url, получите ключ мгновенно и начните с небольшого предоплаченного кредита.",
      primaryCta: "Получить ключ Kimi 3.0",
      secondaryCta: "Смотреть живые цены",
      trustLine: "Без карты на старте · OpenAI-compatible · мгновенный ключ",
    },
    code: {
      kicker: "Drop-in код",
      title: "Поменяйте только base URL",
      subtitle: "Оставьте OpenAI SDK или используйте curl. Направьте трафик в flatkey и задайте модель kimi-k3.",
      terminalTitle: "Пример API",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "Семейство моделей",
      title: "Один ключ — всё семейство Kimi",
      subtitle: "Меняйте id модели, балансируя глубину рассуждений и стоимость — без нового аккаунта и SDK.",
      modelColumn: "Модель",
      bestForColumn: "Лучше всего для",
      inputColumn: "Вход / 1M",
      outputColumn: "Выход / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "Флагманский reasoning — сложные задачи, агенты, длинный контекст" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Агентное программирование и инструменты" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "Повседневный чат и большие объемы" },
      ],
      livePriceNote: "Живые цены из pricing API flatkey — актуальные тарифы и бонусы пополнения на странице цен.",
      priceUnavailable: "Смотреть цены",
    },
    reasonsKicker: "Почему flatkey для Kimi 3.0",
    reasonsTitle: "Две причины, без подвоха",
    reasons: [
      {
        title: "Официальный upstream без китайского аккаунта",
        body: "flatkey работает с официальным upstream Kimi за вас. Регистрируйтесь через Google или GitHub — без китайского номера, местного ID и счета в RMB.",
      },
      {
        title: "Небольшой старт с предоплаченным кредитом",
        body: "Пополните на несколько долларов, мгновенно получите ключ и следите за расходами в консоли. Предоплаченный баланс держит счет под контролем, пока вы оцениваете kimi-k3.",
      },
    ],
    featuresKicker: "Для разработчиков из поиска",
    featuresTitle: "Все, что разработчик проверяет перед пробой модели",
    features: [
      { title: "OpenAI-compatible", body: "Ваш OpenAI SDK работает как есть: поменяйте base_url и api_key, задайте модель kimi-k3, остальное не трогайте." },
      { title: "Reasoning-модель", body: "kimi-k3 — флагманская reasoning-модель Moonshot: многошаговые задачи, агентные циклы, длинный контекст." },
      { title: "Мгновенный ключ", body: "Зарегистрируйтесь и создайте API-ключ за минуты. Без звонков отделу продаж и очередей на одобрение." },
      { title: "Оплата в USD или локально", body: "Простые пополнения международной картой или локальными методами — без счета в RMB и трансграничных переводов." },
      { title: "40+ других моделей", body: "Тот же ключ вызывает GPT, Claude, Gemini, DeepSeek, GLM, Qwen, видео и изображения." },
      { title: "Единый биллинг и аналитика", body: "Один предоплаченный баланс, аналитика по ключам и один счет за все модели." },
    ],
    finalCta: {
      title: "Запустите Kimi 3.0 без переделки стека",
      body: "Создайте flatkey API-ключ, направьте OpenAI-совместимый клиент на наш router и сделайте первый вызов kimi-k3 за минуты.",
      button: "Получить ключ Kimi 3.0",
    },
    faqs: [
      {
        question: "Это официальная модель Kimi 3.0?",
        answer: "Да — flatkey маршрутизирует id kimi-k3 на официальный upstream. Перед продакшеном проверьте доступность и актуальную цену в консоли.",
      },
      {
        question: "Нужен аккаунт или телефон материкового Китая?",
        answer: "Нет. Зарегистрируйтесь через Google или GitHub, пополните в USD или локальными методами и сразу вызывайте kimi-k3 — upstream-аккаунтом занимается flatkey.",
      },
      {
        question: "Нужно переписывать код под OpenAI SDK?",
        answer: "Нет. Используйте OpenAI-совместимую base URL, сохраните SDK и задайте модель kimi-k3. kimi-k2.6 и kimi-k2.5 работают так же.",
      },
    ],
  },
  ja: {
    ...english,
    seo: {
      title: "Kimi 3.0 API — kimi-k3への公式アクセス、OpenAI互換",
      description:
        "flatkey.aiでKimi 3.0 API（モデルkimi-k3、Moonshotの推論モデル）を利用。中国本土アカウント不要の公式アップストリーム、OpenAI互換のbase_url切り替え、即時APIキー、少額から始められるプリペイドクレジット。",
    },
    badge: "kimi-k3 flatkeyで提供中",
    hero: {
      eyebrow: "中国本土外の開発者向けMoonshot Kimi 3.0",
      title: "Kimi 3.0",
      highlight: "API",
      subtitle:
        "Moonshotの推論モデルkimi-k3へ公式に直接アクセス。中国本土のアカウントや電話番号は不要。OpenAI互換のbase_urlを1行変えるだけで、即時にキーを取得し、プリペイドクレジットで少額から始められます。",
      primaryCta: "Kimi 3.0キーを取得",
      secondaryCta: "ライブ料金を見る",
      trustLine: "カード不要で開始 · OpenAI互換 · 即時キー発行",
    },
    code: {
      kicker: "差し替えコード",
      title: "base URLだけを変更",
      subtitle: "OpenAI SDKまたはcurlをそのまま使用。トラフィックをflatkeyに向け、モデルにkimi-k3を指定します。",
      terminalTitle: "API例",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "モデルファミリー",
      title: "1つのキーでKimiファミリー全部",
      subtitle: "モデルidを切り替えるだけで、推論の深さとコストを調整。新しいアカウントもSDKも不要です。",
      modelColumn: "モデル",
      bestForColumn: "最適な用途",
      inputColumn: "入力 / 1M",
      outputColumn: "出力 / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "フラッグシップ推論——難問、エージェント、長文コンテキスト" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "エージェント型コーディングとツール利用" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "日常チャットと大量処理" },
      ],
      livePriceNote: "価格はflatkey料金APIのライブ表示です。最新レートとチャージボーナスは料金ページをご覧ください。",
      priceUnavailable: "料金を見る",
    },
    reasonsKicker: "Kimi 3.0にflatkeyを選ぶ理由",
    reasonsTitle: "理由は2つ、裏はありません",
    reasons: [
      {
        title: "公式アップストリーム、中国本土アカウント不要",
        body: "flatkeyが公式Kimiアップストリームとの接続を代行。GoogleまたはGitHubで登録するだけ——中国の電話番号、現地ID、人民元口座は不要です。",
      },
      {
        title: "プリペイドクレジットで少額スタート",
        body: "数ドルのチャージで即時キーを取得し、コンソールで支出をライブ確認。プリペイド残高なので、kimi-k3の評価中も請求額は上限内に収まります。",
      },
    ],
    featuresKicker: "検索から来た開発者のために",
    featuresTitle: "モデルを試す前に開発者が確認する情報を網羅",
    features: [
      { title: "OpenAI互換", body: "既存のOpenAI SDKがそのまま動作。base_urlとapi_keyを変え、モデルにkimi-k3を指定するだけです。" },
      { title: "推論モデル", body: "kimi-k3はMoonshotのフラッグシップ推論モデル。多段推論、エージェントループ、長文コンテキスト向けです。" },
      { title: "即時キー発行", body: "登録から数分でAPIキーを作成。営業への問い合わせや承認待ちはありません。" },
      { title: "USDまたは現地決済", body: "国際カードや現地決済で簡単チャージ——人民元口座や国際送金は不要です。" },
      { title: "40以上の他モデル", body: "同じキーでGPT、Claude、Gemini、DeepSeek、GLM、Qwen、動画、画像モデルも呼び出せます。" },
      { title: "統一請求と分析", body: "1つのプリペイド残高、キー単位の利用分析、全モデル1枚の請求書。" },
    ],
    finalCta: {
      title: "スタックを変えずにKimi 3.0を導入",
      body: "flatkey APIキーを作成し、OpenAI互換クライアントをrouterに向けるだけで、数分で最初のkimi-k3呼び出しができます。",
      button: "Kimi 3.0キーを取得",
    },
    faqs: [
      {
        question: "公式のKimi 3.0モデルですか？",
        answer: "はい。flatkeyはモデルid kimi-k3を公式アップストリームへルーティングします。本番導入前にコンソールで最新の可用性と価格をご確認ください。",
      },
      {
        question: "中国本土のアカウントや電話番号は必要ですか？",
        answer: "不要です。GoogleまたはGitHubで登録し、USDか現地決済でチャージすれば、すぐにkimi-k3を呼び出せます。アップストリームのアカウントはflatkeyが管理します。",
      },
      {
        question: "OpenAI SDKのコードを書き直す必要はありますか？",
        answer: "ありません。OpenAI互換のbase URLを使い、SDKを維持してモデルにkimi-k3を指定します。kimi-k2.6とkimi-k2.5も同じ使い方です。",
      },
    ],
  },
  vi: {
    ...english,
    seo: {
      title: "Kimi 3.0 API — truy cập chính thức kimi-k3, tương thích OpenAI",
      description:
        "Gọi Kimi 3.0 API (model kimi-k3, model suy luận của Moonshot) qua flatkey.ai: upstream chính thức không cần tài khoản Trung Quốc đại lục, chỉ đổi base_url tương thích OpenAI, nhận key ngay và bắt đầu nhỏ với credit trả trước.",
    },
    badge: "kimi-k3 đã có trên flatkey",
    hero: {
      eyebrow: "Moonshot Kimi 3.0 cho dev ngoài Trung Quốc đại lục",
      title: "Kimi 3.0",
      highlight: "API",
      subtitle:
        "Truy cập trực tiếp chính thức kimi-k3 — model suy luận của Moonshot. Không cần tài khoản hay số điện thoại Trung Quốc: đổi một base_url tương thích OpenAI, nhận key ngay và bắt đầu nhỏ với credit trả trước.",
      primaryCta: "Lấy key Kimi 3.0",
      secondaryCta: "Xem giá trực tiếp",
      trustLine: "Không cần thẻ để bắt đầu · tương thích OpenAI · nhận key ngay",
    },
    code: {
      kicker: "Code thả vào",
      title: "Chỉ đổi base URL",
      subtitle: "Giữ OpenAI SDK hoặc dùng curl. Trỏ traffic về flatkey và đặt model là kimi-k3.",
      terminalTitle: "Ví dụ API",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "Gia đình model",
      title: "Một key, cả gia đình Kimi",
      subtitle: "Đổi model id để cân bằng giữa độ sâu suy luận và chi phí — không cần tài khoản mới hay SDK mới.",
      modelColumn: "Model",
      bestForColumn: "Phù hợp nhất cho",
      inputColumn: "Input / 1M",
      outputColumn: "Output / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "Suy luận hàng đầu — bài toán khó, agent, ngữ cảnh dài" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Lập trình agentic và dùng công cụ" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "Chat hằng ngày và khối lượng lớn" },
      ],
      livePriceNote: "Giá niêm yết trực tiếp từ pricing API của flatkey — xem trang giá để biết mức giá hiện tại và bonus nạp tiền.",
      priceUnavailable: "Xem giá",
    },
    reasonsKicker: "Vì sao chọn flatkey cho Kimi 3.0",
    reasonsTitle: "Hai lý do, không bẫy",
    reasons: [
      {
        title: "Upstream chính thức, không cần tài khoản Trung Quốc",
        body: "flatkey kết nối upstream Kimi chính thức thay bạn. Đăng ký bằng Google hoặc GitHub — không cần số điện thoại Trung Quốc, giấy tờ địa phương hay tài khoản RMB.",
      },
      {
        title: "Bắt đầu nhỏ với credit trả trước",
        body: "Nạp vài đô la, nhận key ngay và theo dõi chi tiêu trực tiếp trong console. Số dư trả trước giữ hóa đơn trong tầm kiểm soát khi bạn đánh giá kimi-k3.",
      },
    ],
    featuresKicker: "Dành cho dev đến từ tìm kiếm",
    featuresTitle: "Những điều dev kiểm tra trước khi thử model",
    features: [
      { title: "Tương thích OpenAI", body: "OpenAI SDK hiện tại chạy nguyên trạng: đổi base_url và api_key, đặt model là kimi-k3, phần còn lại giữ nguyên." },
      { title: "Model suy luận", body: "kimi-k3 là model suy luận chủ lực của Moonshot — cho bài toán nhiều bước, vòng lặp agent và ngữ cảnh dài." },
      { title: "Nhận key ngay", body: "Đăng ký và tạo API key trong vài phút. Không cần gọi sales, không xếp hàng chờ duyệt." },
      { title: "Thanh toán USD hoặc nội địa", body: "Nạp tiền đơn giản bằng thẻ quốc tế hoặc phương thức nội địa — không cần tài khoản RMB hay chuyển khoản xuyên biên giới." },
      { title: "40+ model khác", body: "Cùng một key còn gọi được GPT, Claude, Gemini, DeepSeek, GLM, Qwen, video và hình ảnh." },
      { title: "Billing + analytics hợp nhất", body: "Một số dư trả trước, analytics theo từng key và một hóa đơn duy nhất cho mọi model." },
    ],
    finalCta: {
      title: "Dùng Kimi 3.0 mà không phải làm lại stack",
      body: "Tạo một flatkey API key, trỏ client tương thích OpenAI về router của chúng tôi và thực hiện lượt gọi kimi-k3 đầu tiên trong vài phút.",
      button: "Lấy key Kimi 3.0",
    },
    faqs: [
      {
        question: "Đây có phải model Kimi 3.0 chính thức không?",
        answer: "Đúng — flatkey định tuyến model id kimi-k3 tới upstream chính thức. Hãy xác nhận tính khả dụng và giá hiện tại trong console trước khi lên production.",
      },
      {
        question: "Có cần tài khoản hay số điện thoại Trung Quốc đại lục không?",
        answer: "Không. Đăng ký bằng Google hoặc GitHub, nạp bằng USD hoặc phương thức nội địa và gọi kimi-k3 ngay — flatkey lo tài khoản upstream cho bạn.",
      },
      {
        question: "Có cần viết lại code OpenAI SDK không?",
        answer: "Không. Dùng base URL tương thích OpenAI, giữ SDK và đặt model là kimi-k3. kimi-k2.6 và kimi-k2.5 dùng y hệt.",
      },
    ],
  },
  de: {
    ...english,
    seo: {
      title: "Kimi 3.0 API — offizieller kimi-k3 Zugang, OpenAI-kompatibel",
      description:
        "Nutze die Kimi 3.0 API (Modell kimi-k3, Moonshots Reasoning-Modell) ueber flatkey.ai: offizieller Upstream ohne Festland-China-Konto, OpenAI-kompatibler base_url-Wechsel, sofortiger API-Key und Prepaid-Guthaben fuer den kleinen Start.",
    },
    badge: "kimi-k3 live auf flatkey",
    hero: {
      eyebrow: "Moonshot Kimi 3.0 fuer Entwickler ausserhalb Festlandchinas",
      title: "Kimi 3.0",
      highlight: "API",
      subtitle:
        "Offizieller Direktzugang zu kimi-k3 — Moonshots Reasoning-Modell. Kein Festland-China-Konto oder Telefonnummer: eine OpenAI-kompatible base_url wechseln, sofort einen Key erhalten und mit Prepaid-Guthaben klein starten.",
      primaryCta: "Kimi 3.0 Key holen",
      secondaryCta: "Live-Preise ansehen",
      trustLine: "Keine Karte zum Start · OpenAI-kompatibel · sofortiger Key",
    },
    code: {
      kicker: "Drop-in Code",
      title: "Nur die base URL wechseln",
      subtitle: "Behalte das OpenAI SDK oder nutze curl. Route Traffic zu flatkey und setze das Modell auf kimi-k3.",
      terminalTitle: "API-Beispiel",
      sdkTab: "OpenAI SDK",
      curlTab: "curl",
      model: KIMI_MODEL_ID,
    },
    family: {
      kicker: "Modellfamilie",
      title: "Ein Key, die ganze Kimi-Familie",
      subtitle: "Wechsle die Modell-ID, um Reasoning-Tiefe gegen Kosten abzuwaegen — ohne neues Konto, ohne neues SDK.",
      modelColumn: "Modell",
      bestForColumn: "Am besten fuer",
      inputColumn: "Input / 1M",
      outputColumn: "Output / 1M",
      rows: [
        { modelId: "kimi-k3", name: "Kimi 3.0", bestFor: "Flaggschiff-Reasoning — harte Probleme, Agenten, langer Kontext" },
        { modelId: "kimi-k2.6", name: "Kimi K2.6", bestFor: "Agentisches Coding und Tool-Nutzung" },
        { modelId: "kimi-k2.5", name: "Kimi K2.5", bestFor: "Alltags-Chat und hohe Volumina" },
      ],
      livePriceNote: "Live-Listenpreise aus der flatkey Pricing-API — aktuelle Raten und Top-up-Boni auf der Preisseite.",
      priceUnavailable: "Preise ansehen",
    },
    reasonsKicker: "Warum flatkey fuer Kimi 3.0",
    reasonsTitle: "Zwei Gruende, kein Haken",
    reasons: [
      {
        title: "Offizieller Upstream, kein China-Konto",
        body: "flatkey uebernimmt den offiziellen Kimi-Upstream fuer dich. Registriere dich mit Google oder GitHub — ohne chinesische Telefonnummer, lokale ID oder RMB-Konto.",
      },
      {
        title: "Klein starten mit Prepaid-Guthaben",
        body: "Ein paar Dollar aufladen, sofort einen Key erhalten und die Ausgaben live in der Konsole verfolgen. Das Prepaid-Guthaben haelt die Rechnung begrenzt, waehrend du kimi-k3 evaluierst.",
      },
    ],
    featuresKicker: "Fuer Entwickler aus der Suche gebaut",
    featuresTitle: "Alles, was Entwickler vor dem Modelltest pruefen",
    features: [
      { title: "OpenAI-kompatibel", body: "Dein bestehendes OpenAI SDK laeuft unveraendert: base_url und api_key wechseln, Modell auf kimi-k3 setzen, Rest behalten." },
      { title: "Reasoning-Modell", body: "kimi-k3 ist Moonshots Reasoning-Flaggschiff — fuer mehrstufige Probleme, Agenten-Loops und langen Kontext." },
      { title: "Sofortiger Key", body: "Registrieren und in Minuten einen API-Key erstellen. Kein Sales-Call, keine Freigabe-Warteschlange." },
      { title: "USD oder lokal zahlen", body: "Einfache Top-ups mit internationalen Karten oder lokalen Methoden — ohne RMB-Konto oder Auslandsueberweisung." },
      { title: "40+ weitere Modelle", body: "Derselbe Key ruft auch GPT, Claude, Gemini, DeepSeek, GLM, Qwen, Video- und Bildmodelle auf." },
      { title: "Einheitliches Billing + Analytics", body: "Ein Prepaid-Guthaben, Nutzungsanalysen pro Key und eine Rechnung fuer alle Modelle." },
    ],
    finalCta: {
      title: "Mit Kimi 3.0 starten, ohne den Stack umzubauen",
      body: "Erstelle einen flatkey API-Key, zeige deinen OpenAI-kompatiblen Client auf unseren Router und mache deinen ersten kimi-k3 Call in Minuten.",
      button: "Kimi 3.0 Key holen",
    },
    faqs: [
      {
        question: "Ist das das offizielle Kimi 3.0 Modell?",
        answer: "Ja — flatkey routet die Modell-ID kimi-k3 zum offiziellen Upstream. Pruefe Verfuegbarkeit und aktuellen Preis in der Konsole vor dem Produktionsstart.",
      },
      {
        question: "Brauche ich ein Festland-China-Konto oder eine Telefonnummer?",
        answer: "Nein. Registriere dich mit Google oder GitHub, lade in USD oder lokalen Methoden auf und rufe kimi-k3 sofort auf — flatkey verwaltet das Upstream-Konto.",
      },
      {
        question: "Muss ich meinen OpenAI-SDK-Code neu schreiben?",
        answer: "Nein. Nutze die OpenAI-kompatible base URL, behalte das SDK und setze das Modell auf kimi-k3. kimi-k2.6 und kimi-k2.5 funktionieren genauso.",
      },
    ],
  },
});

export function getKimiLandingPageCopy(locale: Locale): KimiLandingPageCopy {
  return translations[locale] ?? translations.en;
}

export function getKimiLandingMetadataInput(locale: Locale): SeoInput {
  const copy = getKimiLandingPageCopy(locale);
  return {
    title: copy.seo.title,
    description: copy.seo.description,
    pathname: KIMI_LANDING_PATH,
    locale,
  };
}

export function getKimiLandingCtaUrl(origin = APP_CONSOLE_ORIGIN): string {
  return buildConsoleUrl("/sign-up", origin, "redirect=/keys");
}

// Match the Kimi family against the live pricing catalog (same source as the
// public /models pages) so the table never shows invented prices: a family
// member that is missing from the live payload renders a pricing-page link
// instead of a number.
export function resolveKimiFamilyModels(models: PricingModel[]): Record<KimiFamilyModelId, PricingModel | null> {
  const byKey = new Map(models.map((model) => [normalizeModelKey(model.model_name), model]));
  return Object.fromEntries(
    KIMI_FAMILY_MODEL_IDS.map((modelId) => [modelId, byKey.get(normalizeModelKey(modelId)) ?? null])
  ) as Record<KimiFamilyModelId, PricingModel | null>;
}
