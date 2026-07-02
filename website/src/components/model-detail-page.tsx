import { Activity, CheckCircle2, Code2, Gauge, Route, ShieldCheck } from "lucide-react";
import type { ReactNode } from "react";
import { SiteShell } from "@/components/site-shell";
import { buildModelQuickStart, getModelDetailPath } from "@/lib/model-detail";
import type { Locale } from "@/lib/locales";
import { ROUTER_ORIGIN } from "@/lib/origins";
import {
  formatModelPrice,
  isTokenBasedModel,
  type PricingModel,
} from "@/lib/pricing";

type Props = {
  locale: Locale;
  model: PricingModel;
  relatedModels: PricingModel[];
};

type Copy = {
  eyebrow: string;
  titleSuffix: string;
  description: string;
  livePricing: string;
  availability: string;
  endpoints: string;
  quickStart: string;
  inputPrice: string;
  outputPrice: string;
  requestPrice: string;
  cachePrice: string;
  supportedEndpoints: string;
  noDescription: string;
  relatedModels: string;
  viewModel: string;
  tokenBased: string;
  perRequest: string;
  statusAvailable: string;
  statusUnknown: string;
  contextNote: string;
};

const COPY: Record<Locale, Copy> = {
  en: {
    eyebrow: "Model detail",
    titleSuffix: "API pricing and availability",
    description: "Live flatkey pricing, endpoint support, availability, and a one-line OpenAI-compatible setup for this model.",
    livePricing: "Live pricing",
    availability: "Availability",
    endpoints: "Endpoint support",
    quickStart: "Quick start",
    inputPrice: "Input",
    outputPrice: "Output",
    requestPrice: "Request",
    cachePrice: "Cached input",
    supportedEndpoints: "Supported endpoints",
    noDescription: "No description is available yet. Use the live dashboard before production rollout.",
    relatedModels: "Related models",
    viewModel: "View model",
    tokenBased: "Token-based billing",
    perRequest: "Per-request billing",
    statusAvailable: "Available",
    statusUnknown: "Check live dashboard",
    contextNote: "Context window and streaming support can vary by upstream route; confirm live limits in the dashboard before release.",
  },
  zh: {
    eyebrow: "模型详情",
    titleSuffix: "API 价格与可用性",
    description: "查看该模型的 flatkey 实时价格、端点支持、可用性，以及一行 OpenAI 兼容接入方式。",
    livePricing: "实时价格",
    availability: "可用性",
    endpoints: "端点支持",
    quickStart: "快速接入",
    inputPrice: "输入",
    outputPrice: "输出",
    requestPrice: "请求",
    cachePrice: "缓存输入",
    supportedEndpoints: "支持的端点",
    noDescription: "暂无模型描述。生产上线前请在控制台确认实时状态。",
    relatedModels: "相关模型",
    viewModel: "查看模型",
    tokenBased: "按 Token 计费",
    perRequest: "按请求计费",
    statusAvailable: "可用",
    statusUnknown: "请查看实时控制台",
    contextNote: "上下文窗口和流式支持可能随上游路由变化；发布前请在控制台确认实时限制。",
  },
  es: {
    eyebrow: "Detalle del modelo",
    titleSuffix: "precios API y disponibilidad",
    description: "Precios en vivo de flatkey, endpoints, disponibilidad y configuración OpenAI-compatible para este modelo.",
    livePricing: "Precio en vivo",
    availability: "Disponibilidad",
    endpoints: "Endpoints",
    quickStart: "Inicio rápido",
    inputPrice: "Entrada",
    outputPrice: "Salida",
    requestPrice: "Solicitud",
    cachePrice: "Entrada cacheada",
    supportedEndpoints: "Endpoints compatibles",
    noDescription: "Aún no hay descripción. Confirma el estado en el dashboard antes de producción.",
    relatedModels: "Modelos relacionados",
    viewModel: "Ver modelo",
    tokenBased: "Facturación por tokens",
    perRequest: "Facturación por solicitud",
    statusAvailable: "Disponible",
    statusUnknown: "Revisa el dashboard",
    contextNote: "La ventana de contexto y streaming pueden variar por ruta upstream; confirma los límites antes del lanzamiento.",
  },
  fr: {
    eyebrow: "Détail du modèle",
    titleSuffix: "tarifs API et disponibilité",
    description: "Tarifs flatkey en direct, endpoints, disponibilité et configuration compatible OpenAI pour ce modèle.",
    livePricing: "Tarifs en direct",
    availability: "Disponibilité",
    endpoints: "Endpoints",
    quickStart: "Démarrage rapide",
    inputPrice: "Entrée",
    outputPrice: "Sortie",
    requestPrice: "Requête",
    cachePrice: "Entrée en cache",
    supportedEndpoints: "Endpoints pris en charge",
    noDescription: "Aucune description pour l'instant. Vérifiez le dashboard avant la production.",
    relatedModels: "Modèles liés",
    viewModel: "Voir le modèle",
    tokenBased: "Facturation au token",
    perRequest: "Facturation à la requête",
    statusAvailable: "Disponible",
    statusUnknown: "Vérifiez le dashboard",
    contextNote: "Contexte et streaming peuvent varier selon la route upstream ; confirmez les limites avant release.",
  },
  pt: {
    eyebrow: "Detalhe do modelo",
    titleSuffix: "preço API e disponibilidade",
    description: "Preço flatkey ao vivo, endpoints, disponibilidade e configuração compatível com OpenAI para este modelo.",
    livePricing: "Preço ao vivo",
    availability: "Disponibilidade",
    endpoints: "Endpoints",
    quickStart: "Início rápido",
    inputPrice: "Entrada",
    outputPrice: "Saída",
    requestPrice: "Requisição",
    cachePrice: "Entrada em cache",
    supportedEndpoints: "Endpoints compatíveis",
    noDescription: "Ainda não há descrição. Confirme no dashboard antes da produção.",
    relatedModels: "Modelos relacionados",
    viewModel: "Ver modelo",
    tokenBased: "Cobrança por tokens",
    perRequest: "Cobrança por requisição",
    statusAvailable: "Disponível",
    statusUnknown: "Verifique o dashboard",
    contextNote: "Janela de contexto e streaming podem variar por rota upstream; confirme limites antes do lançamento.",
  },
  ru: {
    eyebrow: "Детали модели",
    titleSuffix: "цены API и доступность",
    description: "Актуальная цена flatkey, endpoints, доступность и OpenAI-совместимое подключение для этой модели.",
    livePricing: "Актуальная цена",
    availability: "Доступность",
    endpoints: "Endpoints",
    quickStart: "Быстрый старт",
    inputPrice: "Ввод",
    outputPrice: "Вывод",
    requestPrice: "Запрос",
    cachePrice: "Кэшированный ввод",
    supportedEndpoints: "Поддерживаемые endpoints",
    noDescription: "Описание пока отсутствует. Проверьте статус в dashboard перед production.",
    relatedModels: "Похожие модели",
    viewModel: "Открыть модель",
    tokenBased: "Оплата по токенам",
    perRequest: "Оплата за запрос",
    statusAvailable: "Доступна",
    statusUnknown: "Проверьте dashboard",
    contextNote: "Контекст и streaming зависят от upstream-маршрута; подтвердите лимиты перед релизом.",
  },
  ja: {
    eyebrow: "モデル詳細",
    titleSuffix: "API 料金と可用性",
    description: "このモデルの flatkey ライブ料金、エンドポイント、可用性、OpenAI 互換の接続方法です。",
    livePricing: "ライブ料金",
    availability: "可用性",
    endpoints: "エンドポイント",
    quickStart: "クイックスタート",
    inputPrice: "入力",
    outputPrice: "出力",
    requestPrice: "リクエスト",
    cachePrice: "キャッシュ入力",
    supportedEndpoints: "対応エンドポイント",
    noDescription: "説明はまだありません。本番前にダッシュボードで状態を確認してください。",
    relatedModels: "関連モデル",
    viewModel: "モデルを見る",
    tokenBased: "トークン課金",
    perRequest: "リクエスト課金",
    statusAvailable: "利用可能",
    statusUnknown: "ダッシュボードを確認",
    contextNote: "コンテキスト長と streaming は upstream 経路で変わる場合があります。本番前に制限を確認してください。",
  },
  vi: {
    eyebrow: "Chi tiết mô hình",
    titleSuffix: "giá API và độ sẵn sàng",
    description: "Giá flatkey trực tiếp, endpoint, độ sẵn sàng và cách cấu hình tương thích OpenAI cho mô hình này.",
    livePricing: "Giá trực tiếp",
    availability: "Độ sẵn sàng",
    endpoints: "Endpoint",
    quickStart: "Bắt đầu nhanh",
    inputPrice: "Đầu vào",
    outputPrice: "Đầu ra",
    requestPrice: "Request",
    cachePrice: "Đầu vào cache",
    supportedEndpoints: "Endpoint hỗ trợ",
    noDescription: "Chưa có mô tả. Hãy kiểm tra dashboard trước khi chạy production.",
    relatedModels: "Mô hình liên quan",
    viewModel: "Xem mô hình",
    tokenBased: "Tính phí theo token",
    perRequest: "Tính phí theo request",
    statusAvailable: "Có sẵn",
    statusUnknown: "Kiểm tra dashboard",
    contextNote: "Cửa sổ context và streaming có thể thay đổi theo upstream; hãy xác nhận giới hạn trước khi phát hành.",
  },
  de: {
    eyebrow: "Modelldetails",
    titleSuffix: "API-Preise und Verfügbarkeit",
    description: "Live-flatkey-Preise, Endpoint-Support, Verfügbarkeit und OpenAI-kompatible Einrichtung für dieses Modell.",
    livePricing: "Live-Preise",
    availability: "Verfügbarkeit",
    endpoints: "Endpoints",
    quickStart: "Schnellstart",
    inputPrice: "Input",
    outputPrice: "Output",
    requestPrice: "Request",
    cachePrice: "Cached input",
    supportedEndpoints: "Unterstützte Endpoints",
    noDescription: "Noch keine Beschreibung. Prüfen Sie vor Production den Live-Status im Dashboard.",
    relatedModels: "Ähnliche Modelle",
    viewModel: "Modell ansehen",
    tokenBased: "Tokenbasierte Abrechnung",
    perRequest: "Abrechnung pro Request",
    statusAvailable: "Verfügbar",
    statusUnknown: "Dashboard prüfen",
    contextNote: "Kontextfenster und Streaming können je nach Upstream-Route variieren; prüfen Sie Limits vor dem Release.",
  },
};

export function ModelDetailPage({ locale, model, relatedModels }: Props) {
  const copy = COPY[locale] ?? COPY.en;
  const tokenBased = isTokenBasedModel(model);
  const endpoints = model.supported_endpoint_types ?? [];
  const status = model.availability_status === "available" ? copy.statusAvailable : copy.statusUnknown;
  const group = (model.enable_groups ?? [])[0];

  return (
    <SiteShell locale={locale} pathname={getModelDetailPath(model, "en")}>
      <main className="min-h-screen bg-[linear-gradient(180deg,#f8fafc_0%,#eef7f4_44%,#ffffff_100%)] text-slate-950 dark:bg-[linear-gradient(180deg,#05070a_0%,#071311_52%,#05070a_100%)] dark:text-white">
        <div className="mx-auto w-full max-w-6xl px-6 pt-28 pb-16">
          <header className="max-w-3xl">
            <p className="inline-flex items-center gap-2 rounded-full border border-emerald-500/25 bg-emerald-500/10 px-3 py-1.5 text-xs font-bold tracking-[0.16em] text-emerald-700 uppercase dark:text-emerald-200">
              <ShieldCheck className="size-3.5" />
              {copy.eyebrow}
            </p>
            <h1 className="mt-5 break-words text-[clamp(2.1rem,5vw,4.8rem)] leading-[0.98] font-black tracking-tight">
              {model.model_name}
            </h1>
            <p className="mt-4 text-xl font-semibold text-slate-700 dark:text-slate-200">
              {copy.titleSuffix}
            </p>
            <p className="mt-4 max-w-2xl text-base leading-7 text-slate-600 dark:text-slate-300">
              {model.description || copy.description}
            </p>
          </header>

          <section className="mt-10 grid gap-4 md:grid-cols-3">
            <Metric title={copy.livePricing} icon={<Gauge className="size-4" />}>
              {tokenBased ? (
                <>
                  <PriceLine label={copy.inputPrice} value={`${formatModelPrice(model, "input")}/1M`} />
                  <PriceLine label={copy.outputPrice} value={`${formatModelPrice(model, "output")}/1M`} />
                  {model.cache_ratio != null ? <PriceLine label={copy.cachePrice} value={`${formatModelPrice(model, "cache")}/1M`} /> : null}
                </>
              ) : (
                <PriceLine label={copy.requestPrice} value={formatModelPrice(model)} />
              )}
            </Metric>
            <Metric title={copy.availability} icon={<Activity className="size-4" />}>
              <p className="text-2xl font-black text-emerald-700 dark:text-emerald-200">{status}</p>
              <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">{tokenBased ? copy.tokenBased : copy.perRequest}</p>
              {group ? <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">{group}</p> : null}
            </Metric>
            <Metric title={copy.endpoints} icon={<Route className="size-4" />}>
              <div className="flex flex-wrap gap-2">
                {(endpoints.length ? endpoints : ["openai"]).map((endpoint) => (
                  <span key={endpoint} className="rounded-full bg-slate-950 px-3 py-1 text-xs font-bold text-white dark:bg-white dark:text-slate-950">
                    {endpoint}
                  </span>
                ))}
              </div>
              <p className="mt-3 text-sm text-slate-500 dark:text-slate-400">{copy.supportedEndpoints}</p>
            </Metric>
          </section>

          <section className="mt-6 grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
            <div className="rounded-2xl border border-slate-200 bg-white/78 p-5 shadow-[0_18px_70px_-52px_rgba(15,23,42,0.45)] dark:border-white/10 dark:bg-white/[0.055]">
              <div className="mb-3 flex items-center gap-2 text-sm font-black text-slate-700 dark:text-slate-200">
                <Code2 className="size-4 text-emerald-600" />
                {copy.quickStart}
              </div>
              <pre className="overflow-x-auto rounded-xl bg-slate-950 p-4 text-[13px] leading-7 text-slate-100">
                <code>{buildModelQuickStart(model, ROUTER_ORIGIN)}</code>
              </pre>
            </div>
            <div className="rounded-2xl border border-slate-200 bg-white/78 p-5 shadow-[0_18px_70px_-52px_rgba(15,23,42,0.45)] dark:border-white/10 dark:bg-white/[0.055]">
              <div className="mb-3 flex items-center gap-2 text-sm font-black text-slate-700 dark:text-slate-200">
                <CheckCircle2 className="size-4 text-emerald-600" />
                {copy.availability}
              </div>
              <p className="text-sm leading-6 text-slate-600 dark:text-slate-300">
                {model.availability_reason || copy.contextNote}
              </p>
            </div>
          </section>

          {relatedModels.length > 0 ? (
            <section className="mt-8">
              <h2 className="text-xl font-black tracking-tight">{copy.relatedModels}</h2>
              <div className="mt-4 grid gap-3 md:grid-cols-3">
                {relatedModels.slice(0, 6).map((related) => (
                  <a
                    key={related.model_name}
                    href={getModelDetailPath(related, locale)}
                    className="rounded-2xl border border-slate-200 bg-white/72 p-4 transition-colors hover:border-emerald-400/55 hover:bg-emerald-50/80 dark:border-white/10 dark:bg-white/[0.05] dark:hover:border-emerald-300/35 dark:hover:bg-emerald-300/[0.08]"
                  >
                    <p className="break-all font-mono text-sm font-black">{related.model_name}</p>
                    <p className="mt-2 text-xs font-semibold text-slate-500 dark:text-slate-400">{related.vendor_name ?? "AI"}</p>
                    <p className="mt-3 text-xs font-bold text-emerald-700 dark:text-emerald-200">{copy.viewModel}</p>
                  </a>
                ))}
              </div>
            </section>
          ) : null}
        </div>
      </main>
    </SiteShell>
  );
}

function Metric(props: { title: string; icon: ReactNode; children: ReactNode }) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white/78 p-5 shadow-[0_18px_70px_-52px_rgba(15,23,42,0.45)] dark:border-white/10 dark:bg-white/[0.055]">
      <div className="mb-4 flex items-center gap-2 text-sm font-black text-slate-700 dark:text-slate-200">
        <span className="text-emerald-600 dark:text-emerald-200">{props.icon}</span>
        {props.title}
      </div>
      {props.children}
    </section>
  );
}

function PriceLine(props: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-slate-200 py-2 last:border-0 dark:border-white/10">
      <span className="text-sm text-slate-500 dark:text-slate-400">{props.label}</span>
      <span className="font-mono text-lg font-black text-slate-950 dark:text-white">{props.value}</span>
    </div>
  );
}
