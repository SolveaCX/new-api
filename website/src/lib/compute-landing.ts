import type { SeoInput } from "@/lib/seo";
import { withIdFallback } from "@/lib/locales";
import type { Locale } from "./locales";
import { buildConsoleUrl, APP_CONSOLE_ORIGIN } from "./origins";

export const COMPUTE_LANDING_PATH = "/compute";
export const COMPUTE_SERVERLESS_MODEL = "flatkey-compute-fast";
export const COMPUTE_GPU_4090_PRICE = "$0.72/hr";
export const COMPUTE_GPU_H100_PRICE = "$3.9/hr";
export const COMPUTE_VIDEO_PRICE = "$0.09/sec";
export const COMPUTE_GPU_SAVINGS = "~30%";

export type ComputeLandingFeature = {
  title: string;
  body: string;
};

export type ComputeLandingProduct = {
  name: string;
  tagline: string;
  price: string;
  fitLabel: string;
  fit: string;
};

export type ComputePricingRow = {
  label: string;
  flatkey: string;
  competitor: string;
  note: string;
};

export type ComputeLandingFaq = {
  question: string;
  answer: string;
};

export type ComputeLandingPageCopy = {
  seo: {
    title: string;
    description: string;
  };
  badge: string;
  hero: {
    eyebrow: string;
    title: string;
    highlight: string;
    subtitle: string;
    primaryCta: string;
    secondaryCta: string;
    trustLine: string;
  };
  visual: {
    serverless: { title: string; meta: string };
    gpu: { title: string; meta: string };
    video: { title: string; meta: string };
    balanceLine: string;
  };
  productsKicker: string;
  productsTitle: string;
  products: ComputeLandingProduct[];
  unifiedKicker: string;
  unifiedTitle: string;
  unifiedSubtitle: string;
  unifiedPoints: ComputeLandingFeature[];
  pricingKicker: string;
  pricingTitle: string;
  pricingSubtitle: string;
  pricingCols: { product: string; flatkey: string; elsewhere: string };
  pricingRows: ComputePricingRow[];
  pricingFootnote: string;
  whoForKicker: string;
  whoForTitle: string;
  whoFor: ComputeLandingFeature[];
  finalCta: {
    title: string;
    body: string;
    button: string;
  };
  faqs: ComputeLandingFaq[];
};

const english: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — all your compute, one key, one balance",
    description:
      "flatkey Compute puts serverless models (per token), whole-GPU rental (RTX 4090 ~$0.72/hr, H100 ~$3.9/hr with SSH), and per-second video generation behind a single API key and one shared balance. Upstream suppliers stay hidden.",
  },
  badge: "flatkey Compute live now",
  hero: {
    eyebrow: "One platform for all your compute",
    title: "All your compute,",
    highlight: "one key, one balance",
    subtitle:
      "Serverless models billed per token, whole GPUs rented by the hour, and video generation billed per second — all behind a single flatkey key and one shared balance. We hide the upstream suppliers so you ship on a clean, unified API.",
    primaryCta: "Open Compute",
    secondaryCta: "See pricing",
    trustLine: "One key · one balance · upstream suppliers hidden",
  },
  visual: {
    serverless: { title: "Serverless", meta: "per token" },
    gpu: { title: "GPU rental", meta: "per hour · SSH" },
    video: { title: "Video", meta: "per second" },
    balanceLine: "One key · one balance",
  },
  productsKicker: "Three compute lines",
  productsTitle: "Models, GPUs, and video — under one key",
  products: [
    {
      name: "Serverless models",
      tagline: "Deploy any open model as an API and pay per token — the lowest price for production inference.",
      price: `${COMPUTE_SERVERLESS_MODEL} · per-token billing`,
      fitLabel: "Best for",
      fit: "High-volume inference, chat, and agents that need the cheapest per-token rate.",
    },
    {
      name: "Whole-GPU rental",
      tagline: "Rent an entire GPU by the hour with SSH and Jupyter access. Boots in ~30 seconds, about 30% under retail.",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "Best for",
      fit: "Training, fine-tuning, and custom runtimes where you want the whole card.",
    },
    {
      name: "Video generation",
      tagline: "Generate video with Grok, billed per second. Text-to-video and image-to-video from the same key.",
      price: `Grok video ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "Best for",
      fit: "Creative pipelines and apps that render short clips on demand.",
    },
  ],
  unifiedKicker: "One key, one balance",
  unifiedTitle: "Three compute lines. One account.",
  unifiedSubtitle:
    "Models, GPUs, and video draw from the same flatkey key and the same prepaid balance. No per-supplier accounts, no separate invoices.",
  unifiedPoints: [
    {
      title: "One key for everything",
      body: "The same flatkey API key calls serverless models, provisions GPUs, and renders video. Rotate one secret, not five.",
    },
    {
      title: "One shared balance",
      body: "Top up once. Token spend, GPU-hours, and video-seconds all settle against the same balance with unified usage records.",
    },
    {
      title: "Whitelabel by default",
      body: "We hide the upstream providers behind our own API surface, so your integration never leaks the supplier name, host, or internal model.",
    },
  ],
  pricingKicker: "Pricing",
  pricingTitle: "Lower on every line",
  pricingSubtitle: "Same compute, less spend. Indicative rates — check live billing before launch.",
  pricingCols: { product: "Compute", flatkey: "flatkey", elsewhere: "Compared to" },
  pricingRows: [
    { label: "Serverless · per 1M tokens", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · per hour", flatkey: "$0.72", competitor: "$1.03", note: "vs retail" },
    { label: "Video · per second", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "Indicative figures for illustration. RTX 4090 ~$0.72/hr and H100 ~$3.9/hr reflect live GPU rental; recheck the billing configuration before you rely on any number.",
  whoForKicker: "Who it's for",
  whoForTitle: "Built for however you ship",
  whoFor: [
    {
      title: "Developers",
      body: "Prototype on serverless, burst onto a GPU, add video — without opening a new account each time.",
    },
    {
      title: "Teams",
      body: "One balance, one dashboard, and unified usage records keep spend visible across every compute line.",
    },
    {
      title: "AI agents",
      body: "A single key lets an agent pick the right compute — cheap tokens, a whole GPU, or a video render — programmatically.",
    },
  ],
  finalCta: {
    title: "One key for all your compute",
    body: "Create a flatkey key, top up once, and run serverless models, GPU rentals, and video from a single balance.",
    button: "Open Compute",
  },
  faqs: [
    {
      question: "How is each line billed?",
      answer:
        "Serverless models bill per token, GPU rentals bill per hour of uptime, and video generation bills per second — all drawn from the same prepaid balance.",
    },
    {
      question: "Can I SSH into a rented GPU?",
      answer:
        "Yes. Whole-GPU rentals come with SSH and Jupyter access and boot in about 30 seconds, so you get a full machine, not a sandbox.",
    },
    {
      question: "Do you hide the upstream suppliers?",
      answer:
        "Yes. Compute is whitelabeled behind our own API — responses and logs never expose the upstream provider, host, or internal model name.",
    },
    {
      question: "How much cheaper are the GPUs?",
      answer:
        "Whole-GPU rentals run roughly 30% under typical retail — for example RTX 4090 around $0.72/hr and H100 around $3.9/hr.",
    },
    {
      question: "How do I get started?",
      answer:
        "Open Compute, create one flatkey key, add balance, and point your first serverless, GPU, or video call at the same account.",
    },
  ],
};

const chinese: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — 所有算力，一个 key，一份余额",
    description:
      "flatkey Compute 把 Serverless 模型（按 token 计费）、GPU 整卡出租（RTX 4090 约 $0.72/小时、H100 约 $3.9/小时，带 SSH）和按秒计费的视频生成，统一到一把 API key、一份余额之下，并隐藏上游供应商。",
  },
  badge: "flatkey Compute 已上线",
  hero: {
    eyebrow: "所有算力，一个平台",
    title: "所有算力，",
    highlight: "一个 key，一份余额",
    subtitle:
      "Serverless 模型按 token 计费、GPU 整卡按小时出租、视频生成按秒计费——全部共用一把 flatkey key 和一份余额。我们隐藏上游供应商，让你在干净统一的 API 上开发。",
    primaryCta: "打开 Compute",
    secondaryCta: "查看价格",
    trustLine: "一把 key · 一份余额 · 上游供应商全隐藏",
  },
  visual: {
    serverless: { title: "Serverless", meta: "按 token" },
    gpu: { title: "GPU 出租", meta: "按小时 · SSH" },
    video: { title: "视频", meta: "按秒" },
    balanceLine: "一把 key · 一份余额",
  },
  productsKicker: "三条算力产品线",
  productsTitle: "模型、GPU、视频——共用一把 key",
  products: [
    {
      name: "Serverless 模型",
      tagline: "把任意开源模型部署为 API，按 token 计费——生产级推理的最低价。",
      price: `${COMPUTE_SERVERLESS_MODEL} · 按 token 计费`,
      fitLabel: "适合",
      fit: "需要最低 token 单价的高并发推理、对话和 agent 场景。",
    },
    {
      name: "GPU 整卡出租",
      tagline: "按小时租一整张 GPU，带 SSH 和 Jupyter，约 30 秒开机，比零售低约 30%。",
      price: `RTX 4090 约 ${COMPUTE_GPU_4090_PRICE} · H100 约 ${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "适合",
      fit: "训练、微调以及需要独占整张卡的自定义运行时。",
    },
    {
      name: "视频生成",
      tagline: "用 Grok 生成视频，按秒计费。文生视频、图生视频，共用同一把 key。",
      price: `Grok 视频 约 ${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "适合",
      fit: "按需渲染短片的创意流水线和应用。",
    },
  ],
  unifiedKicker: "一把 key，一份余额",
  unifiedTitle: "三条算力线，一个账户。",
  unifiedSubtitle:
    "模型、GPU 和视频都从同一把 flatkey key、同一份预付余额扣费。不用逐家开供应商账号，也没有分开的账单。",
  unifiedPoints: [
    {
      title: "一把 key 跑通一切",
      body: "同一把 flatkey API key 既调用 Serverless 模型、又开 GPU、又渲染视频。只需轮换一个密钥，而不是五个。",
    },
    {
      title: "一份共享余额",
      body: "充值一次即可。token 消耗、GPU 时长、视频秒数全部在同一份余额结算，用量记录也统一。",
    },
    {
      title: "默认白标",
      body: "我们把上游供应商藏在自有 API 之后，你的接入永远不会泄露供应商名称、主机或内部模型名。",
    },
  ],
  pricingKicker: "价格",
  pricingTitle: "每一条线都更低",
  pricingSubtitle: "同样的算力，花更少的钱。数字为示意——上线前请核对实时计费。",
  pricingCols: { product: "算力", flatkey: "flatkey", elsewhere: "对标" },
  pricingRows: [
    { label: "Serverless · 每百万 token", flatkey: "$0.20", competitor: "$0.28", note: "对标 Novita" },
    { label: "RTX 4090 · 每小时", flatkey: "$0.72", competitor: "$1.03", note: "对标零售" },
    { label: "视频 · 每秒", flatkey: "$0.09", competitor: "$0.13", note: "对标 fal" },
  ],
  pricingFootnote:
    "数字为示意用途。RTX 4090 约 $0.72/小时、H100 约 $3.9/小时来自实时 GPU 租用；在依赖任何数字前请再次核对计费配置。",
  whoForKicker: "为谁而建",
  whoForTitle: "适配你交付的每一种方式",
  whoFor: [
    {
      title: "开发者",
      body: "在 Serverless 上做原型、临时上 GPU、再加视频——无需每次都新开一个账号。",
    },
    {
      title: "团队",
      body: "一份余额、一个控制台、统一的用量记录，让每条算力线的花费都清清楚楚。",
    },
    {
      title: "AI agents",
      body: "一把 key 让 agent 用代码选对算力——便宜的 token、一整张 GPU，或一段视频渲染。",
    },
  ],
  finalCta: {
    title: "所有算力，一把 key",
    body: "创建一把 flatkey key，充值一次，就能用同一份余额跑 Serverless 模型、GPU 租用和视频。",
    button: "打开 Compute",
  },
  faqs: [
    {
      question: "每条线怎么计费？",
      answer: "Serverless 模型按 token 计费，GPU 租用按开机小时计费，视频生成按秒计费——全部从同一份预付余额扣除。",
    },
    {
      question: "能 SSH 进 GPU 吗？",
      answer: "可以。整卡租用自带 SSH 和 Jupyter，约 30 秒开机，你拿到的是一台完整机器，而不是沙箱。",
    },
    {
      question: "会隐藏上游供应商吗？",
      answer: "会。算力经过白标封装在我们自有 API 之后——响应和日志都不会暴露上游供应商、主机或内部模型名。",
    },
    {
      question: "GPU 便宜多少？",
      answer: "整卡租用大约比常见零售价低 30%——例如 RTX 4090 约 $0.72/小时、H100 约 $3.9/小时。",
    },
    {
      question: "如何开始？",
      answer: "打开 Compute，创建一把 flatkey key，充值余额，然后把第一次 Serverless、GPU 或视频调用指向同一个账户。",
    },
  ],
};

const spanish: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — todo tu cómputo, una key, un saldo",
    description:
      "flatkey Compute reúne modelos serverless (por token), alquiler de GPU completa (RTX 4090 ~$0.72/hr, H100 ~$3.9/hr con SSH) y generación de video por segundo detrás de una sola API key y un saldo compartido. Los proveedores upstream quedan ocultos.",
  },
  badge: "flatkey Compute ya disponible",
  hero: {
    eyebrow: "Una plataforma para todo tu cómputo",
    title: "Todo tu cómputo,",
    highlight: "una key, un saldo",
    subtitle:
      "Modelos serverless facturados por token, GPUs completas alquiladas por hora y generación de video facturada por segundo, todo detrás de una sola key de flatkey y un saldo compartido. Ocultamos a los proveedores upstream para que desarrolles sobre una API limpia y unificada.",
    primaryCta: "Abrir Compute",
    secondaryCta: "Ver precios",
    trustLine: "Una key · un saldo · proveedores upstream ocultos",
  },
  visual: {
    serverless: { title: "Serverless", meta: "por token" },
    gpu: { title: "Alquiler de GPU", meta: "por hora · SSH" },
    video: { title: "Video", meta: "por segundo" },
    balanceLine: "Una key · un saldo",
  },
  productsKicker: "Tres líneas de cómputo",
  productsTitle: "Modelos, GPUs y video — bajo una sola key",
  products: [
    {
      name: "Modelos serverless",
      tagline: "Despliega cualquier modelo abierto como API y paga por token: el precio más bajo para inferencia en producción.",
      price: `${COMPUTE_SERVERLESS_MODEL} · facturación por token`,
      fitLabel: "Ideal para",
      fit: "Inferencia de alto volumen, chat y agentes que necesitan la tarifa por token más barata.",
    },
    {
      name: "Alquiler de GPU completa",
      tagline: "Alquila una GPU entera por hora con acceso SSH y Jupyter. Arranca en ~30 segundos, alrededor de un 30% por debajo del precio de mercado.",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "Ideal para",
      fit: "Entrenamiento, fine-tuning y runtimes personalizados donde quieres la tarjeta completa.",
    },
    {
      name: "Generación de video",
      tagline: "Genera video con Grok, facturado por segundo. Texto a video e imagen a video desde la misma key.",
      price: `Grok video ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "Ideal para",
      fit: "Pipelines creativos y apps que renderizan clips cortos bajo demanda.",
    },
  ],
  unifiedKicker: "Una key, un saldo",
  unifiedTitle: "Tres líneas de cómputo. Una cuenta.",
  unifiedSubtitle:
    "Modelos, GPUs y video se descuentan de la misma key de flatkey y del mismo saldo prepago. Sin cuentas por proveedor, sin facturas separadas.",
  unifiedPoints: [
    {
      title: "Una key para todo",
      body: "La misma API key de flatkey llama a los modelos serverless, aprovisiona GPUs y renderiza video. Rota un secreto, no cinco.",
    },
    {
      title: "Un saldo compartido",
      body: "Recarga una vez. El gasto en tokens, las horas de GPU y los segundos de video se liquidan contra el mismo saldo con registros de uso unificados.",
    },
    {
      title: "Marca blanca por defecto",
      body: "Ocultamos a los proveedores upstream detrás de nuestra propia API, de modo que tu integración nunca revela el nombre del proveedor, el host ni el modelo interno.",
    },
  ],
  pricingKicker: "Precios",
  pricingTitle: "Más bajo en cada línea",
  pricingSubtitle: "El mismo cómputo, menos gasto. Tarifas orientativas — consulta la facturación en vivo antes de lanzar.",
  pricingCols: { product: "Cómputo", flatkey: "flatkey", elsewhere: "Comparado con" },
  pricingRows: [
    { label: "Serverless · por 1M tokens", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · por hora", flatkey: "$0.72", competitor: "$1.03", note: "vs mercado" },
    { label: "Video · por segundo", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "Cifras orientativas a modo de ilustración. RTX 4090 ~$0.72/hr y H100 ~$3.9/hr reflejan el alquiler de GPU en vivo; verifica la configuración de facturación antes de confiar en cualquier número.",
  whoForKicker: "Para quién es",
  whoForTitle: "Hecho para tu forma de desplegar",
  whoFor: [
    {
      title: "Desarrolladores",
      body: "Prototipa en serverless, escala puntualmente a una GPU, añade video — sin abrir una cuenta nueva cada vez.",
    },
    {
      title: "Equipos",
      body: "Un saldo, un panel y registros de uso unificados mantienen el gasto visible en cada línea de cómputo.",
    },
    {
      title: "Agentes de IA",
      body: "Una sola key permite que un agente elija el cómputo adecuado —tokens baratos, una GPU completa o un renderizado de video— de forma programática.",
    },
  ],
  finalCta: {
    title: "Una key para todo tu cómputo",
    body: "Crea una key de flatkey, recarga una vez y ejecuta modelos serverless, alquileres de GPU y video desde un único saldo.",
    button: "Abrir Compute",
  },
  faqs: [
    {
      question: "¿Cómo se factura cada línea?",
      answer:
        "Los modelos serverless se facturan por token, los alquileres de GPU por hora de actividad y la generación de video por segundo, todo descontado del mismo saldo prepago.",
    },
    {
      question: "¿Puedo conectarme por SSH a una GPU alquilada?",
      answer:
        "Sí. Los alquileres de GPU completa incluyen acceso SSH y Jupyter y arrancan en unos 30 segundos, así que obtienes una máquina completa, no un sandbox.",
    },
    {
      question: "¿Ocultan a los proveedores upstream?",
      answer:
        "Sí. El cómputo lleva marca blanca detrás de nuestra propia API: las respuestas y los logs nunca exponen el proveedor upstream, el host ni el nombre del modelo interno.",
    },
    {
      question: "¿Cuánto más baratas son las GPUs?",
      answer:
        "Los alquileres de GPU completa cuestan alrededor de un 30% menos que el precio de mercado habitual — por ejemplo, RTX 4090 en torno a $0.72/hr y H100 en torno a $3.9/hr.",
    },
    {
      question: "¿Cómo empiezo?",
      answer:
        "Abre Compute, crea una key de flatkey, añade saldo y dirige tu primera llamada de serverless, GPU o video a la misma cuenta.",
    },
  ],
};

const french: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — tout votre compute, une clé, un solde",
    description:
      "flatkey Compute réunit les modèles serverless (au token), la location de GPU entier (RTX 4090 ~$0.72/hr, H100 ~$3.9/hr avec SSH) et la génération de vidéo à la seconde derrière une seule clé API et un solde partagé. Les fournisseurs en amont restent masqués.",
  },
  badge: "flatkey Compute désormais disponible",
  hero: {
    eyebrow: "Une plateforme pour tout votre compute",
    title: "Tout votre compute,",
    highlight: "une clé, un solde",
    subtitle:
      "Des modèles serverless facturés au token, des GPU entiers loués à l'heure et de la génération de vidéo facturée à la seconde — le tout derrière une seule clé flatkey et un solde partagé. Nous masquons les fournisseurs en amont pour que vous développiez sur une API propre et unifiée.",
    primaryCta: "Ouvrir Compute",
    secondaryCta: "Voir les tarifs",
    trustLine: "Une clé · un solde · fournisseurs en amont masqués",
  },
  visual: {
    serverless: { title: "Serverless", meta: "au token" },
    gpu: { title: "Location de GPU", meta: "à l'heure · SSH" },
    video: { title: "Vidéo", meta: "à la seconde" },
    balanceLine: "Une clé · un solde",
  },
  productsKicker: "Trois lignes de compute",
  productsTitle: "Modèles, GPU et vidéo — sous une seule clé",
  products: [
    {
      name: "Modèles serverless",
      tagline: "Déployez n'importe quel modèle ouvert en tant qu'API et payez au token — le prix le plus bas pour l'inférence en production.",
      price: `${COMPUTE_SERVERLESS_MODEL} · facturation au token`,
      fitLabel: "Idéal pour",
      fit: "L'inférence à fort volume, le chat et les agents qui ont besoin du tarif au token le plus bas.",
    },
    {
      name: "Location de GPU entier",
      tagline: "Louez un GPU entier à l'heure avec accès SSH et Jupyter. Démarrage en ~30 secondes, environ 30 % sous le prix du marché.",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "Idéal pour",
      fit: "L'entraînement, le fine-tuning et les runtimes personnalisés où vous voulez la carte entière.",
    },
    {
      name: "Génération de vidéo",
      tagline: "Générez de la vidéo avec Grok, facturée à la seconde. Texte-vers-vidéo et image-vers-vidéo depuis la même clé.",
      price: `Grok vidéo ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "Idéal pour",
      fit: "Les pipelines créatifs et les applis qui génèrent de courts clips à la demande.",
    },
  ],
  unifiedKicker: "Une clé, un solde",
  unifiedTitle: "Trois lignes de compute. Un seul compte.",
  unifiedSubtitle:
    "Les modèles, les GPU et la vidéo puisent dans la même clé flatkey et le même solde prépayé. Aucun compte par fournisseur, aucune facture distincte.",
  unifiedPoints: [
    {
      title: "Une clé pour tout",
      body: "La même clé API flatkey appelle les modèles serverless, provisionne les GPU et génère la vidéo. Ne faites tourner qu'un seul secret, pas cinq.",
    },
    {
      title: "Un solde partagé",
      body: "Rechargez une fois. La consommation de tokens, les heures de GPU et les secondes de vidéo sont réglées sur le même solde avec des relevés d'usage unifiés.",
    },
    {
      title: "Marque blanche par défaut",
      body: "Nous masquons les fournisseurs en amont derrière notre propre API, si bien que votre intégration ne révèle jamais le nom du fournisseur, l'hôte ni le modèle interne.",
    },
  ],
  pricingKicker: "Tarifs",
  pricingTitle: "Moins cher sur chaque ligne",
  pricingSubtitle: "Le même compute, moins de dépenses. Tarifs indicatifs — vérifiez la facturation en direct avant le lancement.",
  pricingCols: { product: "Compute", flatkey: "flatkey", elsewhere: "Comparé à" },
  pricingRows: [
    { label: "Serverless · par 1M tokens", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · par heure", flatkey: "$0.72", competitor: "$1.03", note: "vs marché" },
    { label: "Vidéo · par seconde", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "Chiffres indicatifs à titre d'illustration. RTX 4090 ~$0.72/hr et H100 ~$3.9/hr reflètent la location de GPU en direct ; revérifiez la configuration de facturation avant de vous fier à un quelconque chiffre.",
  whoForKicker: "À qui ça s'adresse",
  whoForTitle: "Conçu pour votre façon de livrer",
  whoFor: [
    {
      title: "Développeurs",
      body: "Prototypez en serverless, basculez ponctuellement sur un GPU, ajoutez la vidéo — sans ouvrir un nouveau compte à chaque fois.",
    },
    {
      title: "Équipes",
      body: "Un solde, un tableau de bord et des relevés d'usage unifiés gardent la dépense visible sur chaque ligne de compute.",
    },
    {
      title: "Agents IA",
      body: "Une seule clé permet à un agent de choisir le bon compute — des tokens bon marché, un GPU entier ou un rendu vidéo — par programmation.",
    },
  ],
  finalCta: {
    title: "Une clé pour tout votre compute",
    body: "Créez une clé flatkey, rechargez une fois et exécutez modèles serverless, locations de GPU et vidéo depuis un solde unique.",
    button: "Ouvrir Compute",
  },
  faqs: [
    {
      question: "Comment chaque ligne est-elle facturée ?",
      answer:
        "Les modèles serverless sont facturés au token, les locations de GPU à l'heure de fonctionnement et la génération de vidéo à la seconde — le tout prélevé sur le même solde prépayé.",
    },
    {
      question: "Puis-je me connecter en SSH à un GPU loué ?",
      answer:
        "Oui. Les locations de GPU entier incluent l'accès SSH et Jupyter et démarrent en environ 30 secondes : vous obtenez une machine complète, pas un sandbox.",
    },
    {
      question: "Masquez-vous les fournisseurs en amont ?",
      answer:
        "Oui. Le compute est en marque blanche derrière notre propre API : les réponses et les logs n'exposent jamais le fournisseur en amont, l'hôte ni le nom du modèle interne.",
    },
    {
      question: "À combien s'élève l'économie sur les GPU ?",
      answer:
        "Les locations de GPU entier coûtent environ 30 % de moins que le prix de marché habituel — par exemple RTX 4090 autour de $0.72/hr et H100 autour de $3.9/hr.",
    },
    {
      question: "Comment démarrer ?",
      answer:
        "Ouvrez Compute, créez une clé flatkey, ajoutez du solde et dirigez votre premier appel serverless, GPU ou vidéo vers le même compte.",
    },
  ],
};

const portuguese: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — toda a sua computação, uma chave, um saldo",
    description:
      "O flatkey Compute reúne modelos serverless (por token), aluguel de GPU inteira (RTX 4090 ~$0.72/hr, H100 ~$3.9/hr com SSH) e geração de vídeo por segundo atrás de uma única API key e um saldo compartilhado. Os fornecedores upstream permanecem ocultos.",
  },
  badge: "flatkey Compute já disponível",
  hero: {
    eyebrow: "Uma plataforma para toda a sua computação",
    title: "Toda a sua computação,",
    highlight: "uma chave, um saldo",
    subtitle:
      "Modelos serverless cobrados por token, GPUs inteiras alugadas por hora e geração de vídeo cobrada por segundo — tudo atrás de uma única chave flatkey e um saldo compartilhado. Ocultamos os fornecedores upstream para você desenvolver sobre uma API limpa e unificada.",
    primaryCta: "Abrir Compute",
    secondaryCta: "Ver preços",
    trustLine: "Uma chave · um saldo · fornecedores upstream ocultos",
  },
  visual: {
    serverless: { title: "Serverless", meta: "por token" },
    gpu: { title: "Aluguel de GPU", meta: "por hora · SSH" },
    video: { title: "Vídeo", meta: "por segundo" },
    balanceLine: "Uma chave · um saldo",
  },
  productsKicker: "Três linhas de computação",
  productsTitle: "Modelos, GPUs e vídeo — sob uma única chave",
  products: [
    {
      name: "Modelos serverless",
      tagline: "Implante qualquer modelo aberto como API e pague por token — o menor preço para inferência em produção.",
      price: `${COMPUTE_SERVERLESS_MODEL} · cobrança por token`,
      fitLabel: "Ideal para",
      fit: "Inferência de alto volume, chat e agentes que precisam da menor tarifa por token.",
    },
    {
      name: "Aluguel de GPU inteira",
      tagline: "Alugue uma GPU inteira por hora com acesso SSH e Jupyter. Sobe em ~30 segundos, cerca de 30% abaixo do varejo.",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "Ideal para",
      fit: "Treinamento, fine-tuning e runtimes personalizados em que você quer a placa inteira.",
    },
    {
      name: "Geração de vídeo",
      tagline: "Gere vídeo com Grok, cobrado por segundo. Texto para vídeo e imagem para vídeo a partir da mesma chave.",
      price: `Grok vídeo ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "Ideal para",
      fit: "Pipelines criativos e apps que renderizam clipes curtos sob demanda.",
    },
  ],
  unifiedKicker: "Uma chave, um saldo",
  unifiedTitle: "Três linhas de computação. Uma conta.",
  unifiedSubtitle:
    "Modelos, GPUs e vídeo consomem da mesma chave flatkey e do mesmo saldo pré-pago. Sem contas por fornecedor, sem faturas separadas.",
  unifiedPoints: [
    {
      title: "Uma chave para tudo",
      body: "A mesma API key da flatkey chama os modelos serverless, provisiona GPUs e renderiza vídeo. Rotacione um segredo, não cinco.",
    },
    {
      title: "Um saldo compartilhado",
      body: "Recarregue uma vez. O gasto com tokens, as horas de GPU e os segundos de vídeo são liquidados no mesmo saldo, com registros de uso unificados.",
    },
    {
      title: "Marca branca por padrão",
      body: "Ocultamos os fornecedores upstream atrás da nossa própria API, de modo que sua integração nunca revela o nome do fornecedor, o host ou o modelo interno.",
    },
  ],
  pricingKicker: "Preços",
  pricingTitle: "Mais barato em cada linha",
  pricingSubtitle: "A mesma computação, menos gasto. Tarifas indicativas — confira a cobrança ao vivo antes do lançamento.",
  pricingCols: { product: "Computação", flatkey: "flatkey", elsewhere: "Comparado a" },
  pricingRows: [
    { label: "Serverless · por 1M tokens", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · por hora", flatkey: "$0.72", competitor: "$1.03", note: "vs varejo" },
    { label: "Vídeo · por segundo", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "Números indicativos para ilustração. RTX 4090 ~$0.72/hr e H100 ~$3.9/hr refletem o aluguel de GPU ao vivo; reveja a configuração de cobrança antes de confiar em qualquer número.",
  whoForKicker: "Para quem é",
  whoForTitle: "Feito para o jeito como você entrega",
  whoFor: [
    {
      title: "Desenvolvedores",
      body: "Prototipe no serverless, suba pontualmente para uma GPU, adicione vídeo — sem abrir uma conta nova a cada vez.",
    },
    {
      title: "Times",
      body: "Um saldo, um painel e registros de uso unificados mantêm o gasto visível em cada linha de computação.",
    },
    {
      title: "Agentes de IA",
      body: "Uma única chave permite que um agente escolha a computação certa — tokens baratos, uma GPU inteira ou uma renderização de vídeo — de forma programática.",
    },
  ],
  finalCta: {
    title: "Uma chave para toda a sua computação",
    body: "Crie uma chave flatkey, recarregue uma vez e rode modelos serverless, aluguéis de GPU e vídeo a partir de um único saldo.",
    button: "Abrir Compute",
  },
  faqs: [
    {
      question: "Como cada linha é cobrada?",
      answer:
        "Os modelos serverless são cobrados por token, os aluguéis de GPU por hora ligada e a geração de vídeo por segundo — tudo debitado do mesmo saldo pré-pago.",
    },
    {
      question: "Posso acessar uma GPU alugada via SSH?",
      answer:
        "Sim. Os aluguéis de GPU inteira vêm com acesso SSH e Jupyter e sobem em cerca de 30 segundos, então você recebe uma máquina completa, não um sandbox.",
    },
    {
      question: "Vocês ocultam os fornecedores upstream?",
      answer:
        "Sim. A computação usa marca branca atrás da nossa própria API — respostas e logs nunca expõem o fornecedor upstream, o host ou o nome do modelo interno.",
    },
    {
      question: "Quanto mais baratas são as GPUs?",
      answer:
        "Os aluguéis de GPU inteira custam cerca de 30% abaixo do varejo típico — por exemplo, RTX 4090 em torno de $0.72/hr e H100 em torno de $3.9/hr.",
    },
    {
      question: "Como eu começo?",
      answer:
        "Abra o Compute, crie uma chave flatkey, adicione saldo e aponte sua primeira chamada de serverless, GPU ou vídeo para a mesma conta.",
    },
  ],
};

const russian: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — все ваши вычисления, один ключ, один баланс",
    description:
      "flatkey Compute объединяет serverless-модели (за token), аренду целого GPU (RTX 4090 ~$0.72/hr, H100 ~$3.9/hr с SSH) и посекундную генерацию видео за одним API-ключом и общим балансом. Вышестоящие поставщики остаются скрытыми.",
  },
  badge: "flatkey Compute уже доступен",
  hero: {
    eyebrow: "Одна платформа для всех ваших вычислений",
    title: "Все ваши вычисления,",
    highlight: "один ключ, один баланс",
    subtitle:
      "Serverless-модели с оплатой за token, целые GPU в аренду по часам и генерация видео с оплатой за секунду — всё за одним ключом flatkey и общим балансом. Мы скрываем вышестоящих поставщиков, чтобы вы работали с чистым унифицированным API.",
    primaryCta: "Открыть Compute",
    secondaryCta: "Смотреть цены",
    trustLine: "Один ключ · один баланс · вышестоящие поставщики скрыты",
  },
  visual: {
    serverless: { title: "Serverless", meta: "за token" },
    gpu: { title: "Аренда GPU", meta: "за час · SSH" },
    video: { title: "Видео", meta: "за секунду" },
    balanceLine: "Один ключ · один баланс",
  },
  productsKicker: "Три линии вычислений",
  productsTitle: "Модели, GPU и видео — под одним ключом",
  products: [
    {
      name: "Serverless-модели",
      tagline: "Разверните любую открытую модель как API и платите за token — самая низкая цена для продакшн-инференса.",
      price: `${COMPUTE_SERVERLESS_MODEL} · оплата за token`,
      fitLabel: "Лучше всего для",
      fit: "Высоконагруженный инференс, чат и агенты, которым нужна самая дешёвая ставка за token.",
    },
    {
      name: "Аренда целого GPU",
      tagline: "Арендуйте целый GPU по часам с доступом по SSH и Jupyter. Запуск за ~30 секунд, примерно на 30% ниже розницы.",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "Лучше всего для",
      fit: "Обучения, дообучения и кастомных сред выполнения, где нужна вся карта целиком.",
    },
    {
      name: "Генерация видео",
      tagline: "Генерируйте видео с Grok, оплата за секунду. Текст-в-видео и изображение-в-видео с одного ключа.",
      price: `Grok видео ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "Лучше всего для",
      fit: "Креативных пайплайнов и приложений, которые рендерят короткие клипы по запросу.",
    },
  ],
  unifiedKicker: "Один ключ, один баланс",
  unifiedTitle: "Три линии вычислений. Один аккаунт.",
  unifiedSubtitle:
    "Модели, GPU и видео списываются с одного ключа flatkey и одного предоплаченного баланса. Никаких отдельных аккаунтов у поставщиков, никаких раздельных счетов.",
  unifiedPoints: [
    {
      title: "Один ключ на всё",
      body: "Один и тот же API-ключ flatkey вызывает serverless-модели, поднимает GPU и рендерит видео. Меняйте один секрет, а не пять.",
    },
    {
      title: "Один общий баланс",
      body: "Пополните один раз. Расход token, часы GPU и секунды видео списываются с одного баланса с едиными записями об использовании.",
    },
    {
      title: "White-label по умолчанию",
      body: "Мы скрываем вышестоящих поставщиков за собственным API, поэтому ваша интеграция никогда не раскрывает имя поставщика, хост или внутреннюю модель.",
    },
  ],
  pricingKicker: "Цены",
  pricingTitle: "Ниже по каждой линии",
  pricingSubtitle: "Те же вычисления, меньше расходов. Ориентировочные ставки — проверьте актуальную тарификацию перед запуском.",
  pricingCols: { product: "Вычисления", flatkey: "flatkey", elsewhere: "В сравнении с" },
  pricingRows: [
    { label: "Serverless · за 1M token", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · за час", flatkey: "$0.72", competitor: "$1.03", note: "vs розница" },
    { label: "Видео · за секунду", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "Ориентировочные цифры для иллюстрации. RTX 4090 ~$0.72/hr и H100 ~$3.9/hr отражают актуальную аренду GPU; перепроверьте конфигурацию тарификации, прежде чем полагаться на любое число.",
  whoForKicker: "Для кого это",
  whoForTitle: "Создано под ваш способ доставки",
  whoFor: [
    {
      title: "Разработчики",
      body: "Прототипируйте на serverless, точечно переключайтесь на GPU, добавляйте видео — не открывая новый аккаунт каждый раз.",
    },
    {
      title: "Команды",
      body: "Один баланс, один дашборд и единые записи об использовании держат расходы по каждой линии вычислений на виду.",
    },
    {
      title: "ИИ-агенты",
      body: "Один ключ позволяет агенту программно выбрать нужные вычисления — дешёвые token, целый GPU или рендер видео.",
    },
  ],
  finalCta: {
    title: "Один ключ для всех ваших вычислений",
    body: "Создайте ключ flatkey, пополните один раз и запускайте serverless-модели, аренду GPU и видео с одного баланса.",
    button: "Открыть Compute",
  },
  faqs: [
    {
      question: "Как тарифицируется каждая линия?",
      answer:
        "Serverless-модели тарифицируются за token, аренда GPU — за час работы, а генерация видео — за секунду, и всё списывается с одного предоплаченного баланса.",
    },
    {
      question: "Можно ли подключиться к арендованному GPU по SSH?",
      answer:
        "Да. Аренда целого GPU включает доступ по SSH и Jupyter и запускается примерно за 30 секунд, так что вы получаете полноценную машину, а не песочницу.",
    },
    {
      question: "Вы скрываете вышестоящих поставщиков?",
      answer:
        "Да. Вычисления идут под white-label за нашим собственным API — ответы и логи никогда не раскрывают вышестоящего поставщика, хост или имя внутренней модели.",
    },
    {
      question: "Насколько дешевле GPU?",
      answer:
        "Аренда целого GPU обходится примерно на 30% дешевле обычной розницы — например, RTX 4090 около $0.72/hr и H100 около $3.9/hr.",
    },
    {
      question: "Как начать?",
      answer:
        "Откройте Compute, создайте один ключ flatkey, пополните баланс и направьте первый вызов serverless, GPU или видео на один и тот же аккаунт.",
    },
  ],
};

const japanese: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — すべてのコンピュートを、1つのキー、1つの残高で",
    description:
      "flatkey Compute は、serverless モデル（token 単位）、GPU 丸ごとレンタル（RTX 4090 ~$0.72/hr、H100 ~$3.9/hr、SSH 対応）、秒単位の動画生成を、単一の API キーと共有残高の背後にまとめます。上流サプライヤーは隠蔽されます。",
  },
  badge: "flatkey Compute 提供開始",
  hero: {
    eyebrow: "すべてのコンピュートを1つのプラットフォームで",
    title: "すべてのコンピュートを、",
    highlight: "1つのキー、1つの残高で",
    subtitle:
      "token 単位で課金される serverless モデル、時間単位でレンタルする GPU 丸ごと、秒単位で課金される動画生成——そのすべてを単一の flatkey キーと共有残高の背後に。上流サプライヤーを隠蔽し、クリーンで統一された API 上で開発できます。",
    primaryCta: "Compute を開く",
    secondaryCta: "料金を見る",
    trustLine: "1つのキー · 1つの残高 · 上流サプライヤーは隠蔽",
  },
  visual: {
    serverless: { title: "Serverless", meta: "token 単位" },
    gpu: { title: "GPU レンタル", meta: "時間単位 · SSH" },
    video: { title: "動画", meta: "秒単位" },
    balanceLine: "1つのキー · 1つの残高",
  },
  productsKicker: "3つのコンピュートライン",
  productsTitle: "モデル、GPU、動画——1つのキーで",
  products: [
    {
      name: "Serverless モデル",
      tagline: "任意のオープンモデルを API としてデプロイし、token 単位で支払い——本番推論で最安。",
      price: `${COMPUTE_SERVERLESS_MODEL} · token 単位課金`,
      fitLabel: "最適な用途",
      fit: "最安の token 単価を必要とする大量推論、チャット、エージェント。",
    },
    {
      name: "GPU 丸ごとレンタル",
      tagline: "GPU を1枚丸ごと時間単位でレンタル。SSH と Jupyter に対応し、約30秒で起動、小売より約30%安い。",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "最適な用途",
      fit: "カード全体を占有したいトレーニング、fine-tuning、カスタムランタイム。",
    },
    {
      name: "動画生成",
      tagline: "Grok で動画を生成、秒単位で課金。テキストから動画、画像から動画を同じキーで。",
      price: `Grok 動画 ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "最適な用途",
      fit: "オンデマンドで短いクリップをレンダリングするクリエイティブパイプラインとアプリ。",
    },
  ],
  unifiedKicker: "1つのキー、1つの残高",
  unifiedTitle: "3つのコンピュートライン。1つのアカウント。",
  unifiedSubtitle:
    "モデル、GPU、動画はすべて同じ flatkey キーと同じ前払い残高から引き落とされます。サプライヤーごとのアカウントも、別々の請求書もありません。",
  unifiedPoints: [
    {
      title: "1つのキーですべて",
      body: "同じ flatkey API キーが serverless モデルを呼び出し、GPU をプロビジョニングし、動画をレンダリングします。ローテーションするシークレットは5つではなく1つ。",
    },
    {
      title: "1つの共有残高",
      body: "一度チャージするだけ。token の消費、GPU 時間、動画の秒数はすべて同じ残高で精算され、利用記録も統一されます。",
    },
    {
      title: "デフォルトでホワイトラベル",
      body: "上流プロバイダーを自社 API の背後に隠すため、あなたの実装がサプライヤー名、ホスト、内部モデル名を漏らすことは決してありません。",
    },
  ],
  pricingKicker: "料金",
  pricingTitle: "どのラインでもより安く",
  pricingSubtitle: "同じコンピュートで、支出はより少なく。参考価格です——ローンチ前に実際の課金をご確認ください。",
  pricingCols: { product: "コンピュート", flatkey: "flatkey", elsewhere: "比較対象" },
  pricingRows: [
    { label: "Serverless · 100万 token あたり", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · 1時間あたり", flatkey: "$0.72", competitor: "$1.03", note: "vs 小売" },
    { label: "動画 · 1秒あたり", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "参考用の数値です。RTX 4090 ~$0.72/hr と H100 ~$3.9/hr は実際の GPU レンタルを反映しています。いかなる数値も、依拠する前に課金設定を再確認してください。",
  whoForKicker: "対象ユーザー",
  whoForTitle: "あなたの届け方に合わせて",
  whoFor: [
    {
      title: "開発者",
      body: "serverless でプロトタイプを作り、必要なときに GPU にバースト、動画を追加——毎回新しいアカウントを開くことなく。",
    },
    {
      title: "チーム",
      body: "1つの残高、1つのダッシュボード、統一された利用記録が、すべてのコンピュートラインの支出を可視化します。",
    },
    {
      title: "AI エージェント",
      body: "1つのキーで、エージェントが適切なコンピュートをプログラムから選択——安価な token、GPU 丸ごと、あるいは動画レンダリング。",
    },
  ],
  finalCta: {
    title: "すべてのコンピュートを1つのキーで",
    body: "flatkey キーを作成し、一度チャージすれば、serverless モデル、GPU レンタル、動画を1つの残高から実行できます。",
    button: "Compute を開く",
  },
  faqs: [
    {
      question: "各ラインはどう課金されますか？",
      answer:
        "serverless モデルは token 単位、GPU レンタルは稼働時間単位、動画生成は秒単位で課金され、すべて同じ前払い残高から引き落とされます。",
    },
    {
      question: "レンタルした GPU に SSH で接続できますか？",
      answer:
        "はい。GPU 丸ごとレンタルには SSH と Jupyter アクセスが付き、約30秒で起動します。サンドボックスではなく、完全なマシンを手に入れられます。",
    },
    {
      question: "上流サプライヤーは隠蔽されますか？",
      answer:
        "はい。コンピュートは自社 API の背後でホワイトラベル化されており、レスポンスやログが上流プロバイダー、ホスト、内部モデル名を露出することはありません。",
    },
    {
      question: "GPU はどれくらい安いですか？",
      answer:
        "GPU 丸ごとレンタルは一般的な小売価格より約30%安くなります——例えば RTX 4090 は約 $0.72/hr、H100 は約 $3.9/hr です。",
    },
    {
      question: "どうやって始めますか？",
      answer:
        "Compute を開き、flatkey キーを1つ作成し、残高を追加して、最初の serverless、GPU、または動画の呼び出しを同じアカウントに向けてください。",
    },
  ],
};

const vietnamese: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — toàn bộ compute của bạn, một key, một số dư",
    description:
      "flatkey Compute gộp các mô hình serverless (theo token), thuê nguyên GPU (RTX 4090 ~$0.72/hr, H100 ~$3.9/hr có SSH) và tạo video theo giây sau một API key duy nhất và một số dư dùng chung. Các nhà cung cấp upstream được ẩn đi.",
  },
  badge: "flatkey Compute đã ra mắt",
  hero: {
    eyebrow: "Một nền tảng cho toàn bộ compute của bạn",
    title: "Toàn bộ compute của bạn,",
    highlight: "một key, một số dư",
    subtitle:
      "Mô hình serverless tính phí theo token, GPU nguyên chiếc thuê theo giờ và tạo video tính phí theo giây — tất cả sau một key flatkey duy nhất và một số dư dùng chung. Chúng tôi ẩn các nhà cung cấp upstream để bạn phát triển trên một API sạch và thống nhất.",
    primaryCta: "Mở Compute",
    secondaryCta: "Xem bảng giá",
    trustLine: "Một key · một số dư · nhà cung cấp upstream được ẩn",
  },
  visual: {
    serverless: { title: "Serverless", meta: "theo token" },
    gpu: { title: "Thuê GPU", meta: "theo giờ · SSH" },
    video: { title: "Video", meta: "theo giây" },
    balanceLine: "Một key · một số dư",
  },
  productsKicker: "Ba dòng compute",
  productsTitle: "Mô hình, GPU và video — dưới một key",
  products: [
    {
      name: "Mô hình serverless",
      tagline: "Triển khai bất kỳ mô hình mở nào dưới dạng API và trả phí theo token — mức giá thấp nhất cho suy luận production.",
      price: `${COMPUTE_SERVERLESS_MODEL} · tính phí theo token`,
      fitLabel: "Phù hợp nhất cho",
      fit: "Suy luận khối lượng lớn, chat và agent cần mức giá theo token rẻ nhất.",
    },
    {
      name: "Thuê nguyên GPU",
      tagline: "Thuê nguyên một GPU theo giờ với quyền truy cập SSH và Jupyter. Khởi động trong ~30 giây, rẻ hơn khoảng 30% so với giá bán lẻ.",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "Phù hợp nhất cho",
      fit: "Huấn luyện, fine-tuning và các runtime tùy chỉnh khi bạn muốn dùng trọn cả card.",
    },
    {
      name: "Tạo video",
      tagline: "Tạo video bằng Grok, tính phí theo giây. Text-to-video và image-to-video từ cùng một key.",
      price: `Grok video ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "Phù hợp nhất cho",
      fit: "Các pipeline sáng tạo và ứng dụng render các clip ngắn theo yêu cầu.",
    },
  ],
  unifiedKicker: "Một key, một số dư",
  unifiedTitle: "Ba dòng compute. Một tài khoản.",
  unifiedSubtitle:
    "Mô hình, GPU và video đều rút từ cùng một key flatkey và cùng một số dư trả trước. Không cần tài khoản riêng cho từng nhà cung cấp, không hóa đơn tách biệt.",
  unifiedPoints: [
    {
      title: "Một key cho mọi thứ",
      body: "Cùng một API key flatkey gọi các mô hình serverless, cấp phát GPU và render video. Xoay vòng một khóa bí mật, không phải năm.",
    },
    {
      title: "Một số dư dùng chung",
      body: "Nạp một lần. Chi phí token, số giờ GPU và số giây video đều được quyết toán trên cùng một số dư với bản ghi sử dụng thống nhất.",
    },
    {
      title: "White-label mặc định",
      body: "Chúng tôi ẩn các nhà cung cấp upstream sau API của riêng mình, nên phần tích hợp của bạn không bao giờ để lộ tên nhà cung cấp, host hay mô hình nội bộ.",
    },
  ],
  pricingKicker: "Bảng giá",
  pricingTitle: "Thấp hơn ở mọi dòng",
  pricingSubtitle: "Cùng compute, chi ít hơn. Mức giá tham khảo — hãy kiểm tra tính phí trực tiếp trước khi ra mắt.",
  pricingCols: { product: "Compute", flatkey: "flatkey", elsewhere: "So với" },
  pricingRows: [
    { label: "Serverless · mỗi 1M token", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · mỗi giờ", flatkey: "$0.72", competitor: "$1.03", note: "vs bán lẻ" },
    { label: "Video · mỗi giây", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "Các con số mang tính minh họa. RTX 4090 ~$0.72/hr và H100 ~$3.9/hr phản ánh việc thuê GPU trực tiếp; hãy kiểm tra lại cấu hình tính phí trước khi dựa vào bất kỳ con số nào.",
  whoForKicker: "Dành cho ai",
  whoForTitle: "Xây cho mọi cách bạn triển khai",
  whoFor: [
    {
      title: "Nhà phát triển",
      body: "Tạo prototype trên serverless, bùng lên một GPU khi cần, thêm video — mà không phải mở tài khoản mới mỗi lần.",
    },
    {
      title: "Đội nhóm",
      body: "Một số dư, một bảng điều khiển và bản ghi sử dụng thống nhất giúp chi phí luôn rõ ràng trên mọi dòng compute.",
    },
    {
      title: "AI agent",
      body: "Một key duy nhất cho phép agent chọn đúng compute — token rẻ, nguyên một GPU, hay một lần render video — bằng lập trình.",
    },
  ],
  finalCta: {
    title: "Một key cho toàn bộ compute của bạn",
    body: "Tạo một key flatkey, nạp một lần và chạy các mô hình serverless, thuê GPU và video từ một số dư duy nhất.",
    button: "Mở Compute",
  },
  faqs: [
    {
      question: "Mỗi dòng được tính phí thế nào?",
      answer:
        "Mô hình serverless tính phí theo token, thuê GPU tính theo giờ hoạt động, còn tạo video tính theo giây — tất cả đều trừ từ cùng một số dư trả trước.",
    },
    {
      question: "Tôi có thể SSH vào GPU đã thuê không?",
      answer:
        "Có. Gói thuê nguyên GPU đi kèm quyền truy cập SSH và Jupyter và khởi động trong khoảng 30 giây, nên bạn nhận được một máy hoàn chỉnh, không phải sandbox.",
    },
    {
      question: "Các bạn có ẩn nhà cung cấp upstream không?",
      answer:
        "Có. Compute được white-label sau API của riêng chúng tôi — phản hồi và log không bao giờ để lộ nhà cung cấp upstream, host hay tên mô hình nội bộ.",
    },
    {
      question: "GPU rẻ hơn bao nhiêu?",
      answer:
        "Thuê nguyên GPU rẻ hơn khoảng 30% so với giá bán lẻ thông thường — ví dụ RTX 4090 khoảng $0.72/hr và H100 khoảng $3.9/hr.",
    },
    {
      question: "Tôi bắt đầu thế nào?",
      answer:
        "Mở Compute, tạo một key flatkey, thêm số dư và trỏ lệnh gọi serverless, GPU hay video đầu tiên của bạn vào cùng một tài khoản.",
    },
  ],
};

const german: ComputeLandingPageCopy = {
  seo: {
    title: "flatkey Compute — Ihre gesamte Rechenleistung, ein Schlüssel, ein Guthaben",
    description:
      "flatkey Compute bündelt Serverless-Modelle (pro Token), die Vermietung ganzer GPUs (RTX 4090 ~$0.72/hr, H100 ~$3.9/hr mit SSH) und sekundengenaue Videogenerierung hinter einem einzigen API-Schlüssel und einem gemeinsamen Guthaben. Upstream-Anbieter bleiben verborgen.",
  },
  badge: "flatkey Compute ist jetzt verfügbar",
  hero: {
    eyebrow: "Eine Plattform für Ihre gesamte Rechenleistung",
    title: "Ihre gesamte Rechenleistung,",
    highlight: "ein Schlüssel, ein Guthaben",
    subtitle:
      "Serverless-Modelle, abgerechnet pro Token, ganze GPUs, stundenweise gemietet, und Videogenerierung, abgerechnet pro Sekunde — alles hinter einem einzigen flatkey-Schlüssel und einem gemeinsamen Guthaben. Wir verbergen die Upstream-Anbieter, damit Sie auf einer sauberen, einheitlichen API entwickeln.",
    primaryCta: "Compute öffnen",
    secondaryCta: "Preise ansehen",
    trustLine: "Ein Schlüssel · ein Guthaben · Upstream-Anbieter verborgen",
  },
  visual: {
    serverless: { title: "Serverless", meta: "pro Token" },
    gpu: { title: "GPU-Miete", meta: "pro Stunde · SSH" },
    video: { title: "Video", meta: "pro Sekunde" },
    balanceLine: "Ein Schlüssel · ein Guthaben",
  },
  productsKicker: "Drei Compute-Linien",
  productsTitle: "Modelle, GPUs und Video — unter einem Schlüssel",
  products: [
    {
      name: "Serverless-Modelle",
      tagline: "Stellen Sie jedes offene Modell als API bereit und zahlen Sie pro Token — der günstigste Preis für Inferenz in der Produktion.",
      price: `${COMPUTE_SERVERLESS_MODEL} · Abrechnung pro Token`,
      fitLabel: "Ideal für",
      fit: "Inferenz mit hohem Volumen, Chat und Agenten, die den günstigsten Preis pro Token brauchen.",
    },
    {
      name: "Vermietung ganzer GPUs",
      tagline: "Mieten Sie eine ganze GPU stundenweise mit SSH- und Jupyter-Zugang. Startet in ~30 Sekunden, rund 30 % unter dem Marktpreis.",
      price: `RTX 4090 ~${COMPUTE_GPU_4090_PRICE} · H100 ~${COMPUTE_GPU_H100_PRICE}`,
      fitLabel: "Ideal für",
      fit: "Training, Fine-Tuning und eigene Runtimes, bei denen Sie die ganze Karte wollen.",
    },
    {
      name: "Videogenerierung",
      tagline: "Generieren Sie Video mit Grok, abgerechnet pro Sekunde. Text-zu-Video und Bild-zu-Video über denselben Schlüssel.",
      price: `Grok Video ~${COMPUTE_VIDEO_PRICE}`,
      fitLabel: "Ideal für",
      fit: "Kreativ-Pipelines und Apps, die kurze Clips auf Abruf rendern.",
    },
  ],
  unifiedKicker: "Ein Schlüssel, ein Guthaben",
  unifiedTitle: "Drei Compute-Linien. Ein Konto.",
  unifiedSubtitle:
    "Modelle, GPUs und Video ziehen vom selben flatkey-Schlüssel und demselben Prepaid-Guthaben. Keine Konten pro Anbieter, keine separaten Rechnungen.",
  unifiedPoints: [
    {
      title: "Ein Schlüssel für alles",
      body: "Derselbe flatkey-API-Schlüssel ruft Serverless-Modelle auf, stellt GPUs bereit und rendert Video. Rotieren Sie ein Geheimnis, nicht fünf.",
    },
    {
      title: "Ein gemeinsames Guthaben",
      body: "Einmal aufladen. Token-Verbrauch, GPU-Stunden und Video-Sekunden werden alle über dasselbe Guthaben abgerechnet, mit einheitlichen Nutzungsdaten.",
    },
    {
      title: "White-Label als Standard",
      body: "Wir verbergen die Upstream-Anbieter hinter unserer eigenen API, sodass Ihre Integration niemals den Anbieternamen, den Host oder das interne Modell preisgibt.",
    },
  ],
  pricingKicker: "Preise",
  pricingTitle: "Auf jeder Linie günstiger",
  pricingSubtitle: "Dieselbe Rechenleistung, weniger Ausgaben. Richtwerte — prüfen Sie die Live-Abrechnung vor dem Start.",
  pricingCols: { product: "Rechenleistung", flatkey: "flatkey", elsewhere: "Verglichen mit" },
  pricingRows: [
    { label: "Serverless · pro 1M Token", flatkey: "$0.20", competitor: "$0.28", note: "vs Novita" },
    { label: "RTX 4090 · pro Stunde", flatkey: "$0.72", competitor: "$1.03", note: "vs Markt" },
    { label: "Video · pro Sekunde", flatkey: "$0.09", competitor: "$0.13", note: "vs fal" },
  ],
  pricingFootnote:
    "Richtwerte zur Veranschaulichung. RTX 4090 ~$0.72/hr und H100 ~$3.9/hr spiegeln die Live-GPU-Miete wider; prüfen Sie die Abrechnungskonfiguration erneut, bevor Sie sich auf eine Zahl verlassen.",
  whoForKicker: "Für wen",
  whoForTitle: "Gebaut für Ihre Art zu liefern",
  whoFor: [
    {
      title: "Entwickler",
      body: "Prototypen auf Serverless, bei Bedarf auf eine GPU ausweichen, Video ergänzen — ohne jedes Mal ein neues Konto zu eröffnen.",
    },
    {
      title: "Teams",
      body: "Ein Guthaben, ein Dashboard und einheitliche Nutzungsdaten halten die Ausgaben über jede Compute-Linie sichtbar.",
    },
    {
      title: "KI-Agenten",
      body: "Ein einziger Schlüssel lässt einen Agenten programmatisch die richtige Rechenleistung wählen — günstige Token, eine ganze GPU oder ein Video-Rendering.",
    },
  ],
  finalCta: {
    title: "Ein Schlüssel für Ihre gesamte Rechenleistung",
    body: "Erstellen Sie einen flatkey-Schlüssel, laden Sie einmal auf und betreiben Sie Serverless-Modelle, GPU-Mieten und Video über ein einziges Guthaben.",
    button: "Compute öffnen",
  },
  faqs: [
    {
      question: "Wie wird jede Linie abgerechnet?",
      answer:
        "Serverless-Modelle werden pro Token abgerechnet, GPU-Mieten pro Betriebsstunde und die Videogenerierung pro Sekunde — alles vom selben Prepaid-Guthaben abgezogen.",
    },
    {
      question: "Kann ich mich per SSH mit einer gemieteten GPU verbinden?",
      answer:
        "Ja. Die Vermietung ganzer GPUs umfasst SSH- und Jupyter-Zugang und startet in etwa 30 Sekunden, sodass Sie eine vollständige Maschine erhalten, keine Sandbox.",
    },
    {
      question: "Verbergen Sie die Upstream-Anbieter?",
      answer:
        "Ja. Compute läuft als White-Label hinter unserer eigenen API — Antworten und Logs geben niemals den Upstream-Anbieter, den Host oder den internen Modellnamen preis.",
    },
    {
      question: "Wie viel günstiger sind die GPUs?",
      answer:
        "Die Vermietung ganzer GPUs kostet rund 30 % weniger als der übliche Marktpreis — zum Beispiel RTX 4090 bei etwa $0.72/hr und H100 bei etwa $3.9/hr.",
    },
    {
      question: "Wie fange ich an?",
      answer:
        "Öffnen Sie Compute, erstellen Sie einen flatkey-Schlüssel, laden Sie Guthaben auf und richten Sie Ihren ersten Serverless-, GPU- oder Video-Aufruf auf dasselbe Konto.",
    },
  ],
};

const translations: Record<Locale, ComputeLandingPageCopy> = withIdFallback({
  en: english,
  zh: chinese,
  es: spanish,
  fr: french,
  pt: portuguese,
  ru: russian,
  ja: japanese,
  vi: vietnamese,
  de: german,
});

export function getComputeLandingPageCopy(locale: Locale): ComputeLandingPageCopy {
  return translations[locale] ?? translations.en;
}

export function getComputeLandingMetadataInput(locale: Locale): SeoInput {
  const copy = getComputeLandingPageCopy(locale);
  return {
    title: copy.seo.title,
    description: copy.seo.description,
    pathname: COMPUTE_LANDING_PATH,
    locale,
  };
}

export function getComputeLandingCtaUrl(origin = APP_CONSOLE_ORIGIN): string {
  return buildConsoleUrl("/sign-up", origin, "redirect=/keys");
}
