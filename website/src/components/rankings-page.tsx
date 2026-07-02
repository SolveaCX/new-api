import { ArrowDownRight, ArrowUpRight, BarChart3, Crown, TrendingUp } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { localizePath, type Locale } from "@/lib/locales";
import { formatGrowth, formatShare, type RankingPeriod, type WebsiteRankingsData } from "@/lib/rankings";

type Props = {
  locale: Locale;
  rankings: WebsiteRankingsData;
};

type Copy = {
  eyebrow: string;
  title: string;
  description: string;
  dataPolicy: string;
  modelRankings: string;
  vendorShare: string;
  movers: string;
  droppers: string;
  rank: string;
  model: string;
  vendor: string;
  share: string;
  growth: string;
  topModel: string;
  noData: string;
  sampleLimited: string;
  periods: Record<RankingPeriod, string>;
};

const COPY: Record<Locale, Copy> = {
  en: {
    eyebrow: "Usage-ranked models",
    title: "AI model rankings from aggregated Flatkey usage signals",
    description: "A privacy-filtered view of routed model usage. Low-volume movements are suppressed, shares are rounded, and absolute token totals are never exposed.",
    dataPolicy: "Updated roughly every 5 minutes from delayed aggregate data. Growth is hidden when the previous sample is too small.",
    modelRankings: "Top models",
    vendorShare: "Provider share",
    movers: "Rising models",
    droppers: "Cooling models",
    rank: "Rank",
    model: "Model",
    vendor: "Provider",
    share: "Usage share",
    growth: "Growth",
    topModel: "Top model",
    noData: "Rankings data is warming up. Check back soon.",
    sampleLimited: "N/A",
    periods: { week: "Week", month: "Month" },
  },
  zh: {
    eyebrow: "按用量排序的模型",
    title: "基于 Flatkey 聚合用量信号的 AI 模型排行",
    description: "这是经过隐私过滤的模型路由用量视图；低样本波动会被隐藏，占比已做粗粒度处理，绝不公开绝对 token 总量。",
    dataPolicy: "数据约每 5 分钟从延迟聚合结果更新一次；当上一周期样本不足时，不展示具体增长率。",
    modelRankings: "热门模型",
    vendorShare: "供应商占比",
    movers: "上升模型",
    droppers: "回落模型",
    rank: "排名",
    model: "模型",
    vendor: "供应商",
    share: "用量占比",
    growth: "增长",
    topModel: "代表模型",
    noData: "排行榜数据正在生成，请稍后查看。",
    sampleLimited: "样本不足",
    periods: { week: "本周", month: "本月" },
  },
  es: {
    eyebrow: "Modelos por uso",
    title: "Rankings de modelos IA con señales agregadas de Flatkey",
    description: "Vista pública con filtros de privacidad. Se ocultan movimientos de bajo volumen, las cuotas se redondean y nunca se publican tokens absolutos.",
    dataPolicy: "Se actualiza aproximadamente cada 5 minutos con datos agregados diferidos. El crecimiento se oculta si la muestra previa es pequeña.",
    modelRankings: "Modelos top",
    vendorShare: "Cuota por proveedor",
    movers: "Modelos en alza",
    droppers: "Modelos a la baja",
    rank: "Rank",
    model: "Modelo",
    vendor: "Proveedor",
    share: "Cuota de uso",
    growth: "Crecimiento",
    topModel: "Modelo top",
    noData: "Los rankings se están preparando. Vuelve pronto.",
    sampleLimited: "N/D",
    periods: { week: "Semana", month: "Mes" },
  },
  fr: {
    eyebrow: "Modèles classés par usage",
    title: "Classement des modèles IA à partir de signaux agrégés Flatkey",
    description: "Vue filtrée pour la confidentialité. Les faibles volumes sont masqués, les parts sont arrondies et les tokens absolus ne sont jamais publiés.",
    dataPolicy: "Mise à jour environ toutes les 5 minutes depuis des données agrégées différées. La croissance est masquée si l'échantillon précédent est trop faible.",
    modelRankings: "Top modèles",
    vendorShare: "Part fournisseur",
    movers: "Modèles en hausse",
    droppers: "Modèles en baisse",
    rank: "Rang",
    model: "Modèle",
    vendor: "Fournisseur",
    share: "Part d'usage",
    growth: "Croissance",
    topModel: "Top modèle",
    noData: "Les classements sont en préparation. Revenez bientôt.",
    sampleLimited: "N/D",
    periods: { week: "Semaine", month: "Mois" },
  },
  pt: {
    eyebrow: "Modelos por uso",
    title: "Ranking de modelos de IA por sinais agregados da Flatkey",
    description: "Visualização com filtro de privacidade. Movimentos de baixo volume são ocultados, participações são arredondadas e tokens absolutos nunca são publicados.",
    dataPolicy: "Atualizado aproximadamente a cada 5 minutos com dados agregados defasados. O crescimento é ocultado quando a amostra anterior é pequena.",
    modelRankings: "Top modelos",
    vendorShare: "Participação por provedor",
    movers: "Modelos em alta",
    droppers: "Modelos em queda",
    rank: "Rank",
    model: "Modelo",
    vendor: "Provedor",
    share: "Participação de uso",
    growth: "Crescimento",
    topModel: "Top modelo",
    noData: "Os rankings estão aquecendo. Volte em breve.",
    sampleLimited: "N/D",
    periods: { week: "Semana", month: "Mês" },
  },
  ru: {
    eyebrow: "Модели по использованию",
    title: "Рейтинг AI-моделей по агрегированным сигналам Flatkey",
    description: "Публичный вид с фильтрами приватности. Низкообъемные изменения скрываются, доли округляются, абсолютные токены не публикуются.",
    dataPolicy: "Обновляется примерно каждые 5 минут по отложенным агрегированным данным. Рост скрывается, если предыдущая выборка мала.",
    modelRankings: "Топ моделей",
    vendorShare: "Доля провайдеров",
    movers: "Растущие модели",
    droppers: "Снижающиеся модели",
    rank: "Ранг",
    model: "Модель",
    vendor: "Провайдер",
    share: "Доля usage",
    growth: "Рост",
    topModel: "Топ модель",
    noData: "Рейтинг формируется. Загляните позже.",
    sampleLimited: "Н/Д",
    periods: { week: "Неделя", month: "Месяц" },
  },
  ja: {
    eyebrow: "利用量順のモデル",
    title: "Flatkey の集計利用シグナルに基づく AI モデルランキング",
    description: "プライバシー保護済みの公開ビューです。低ボリュームの変動は非表示、シェアは丸め、絶対 token 数は公開しません。",
    dataPolicy: "遅延集計データから約 5 分ごとに更新します。前期間のサンプルが小さい場合、成長率は表示しません。",
    modelRankings: "上位モデル",
    vendorShare: "プロバイダー比率",
    movers: "上昇モデル",
    droppers: "低下モデル",
    rank: "順位",
    model: "モデル",
    vendor: "プロバイダー",
    share: "利用シェア",
    growth: "成長",
    topModel: "代表モデル",
    noData: "ランキングデータを準備中です。しばらくしてから確認してください。",
    sampleLimited: "サンプル不足",
    periods: { week: "週", month: "月" },
  },
  vi: {
    eyebrow: "Mô hình xếp theo usage",
    title: "Xếp hạng mô hình AI theo tín hiệu usage tổng hợp trên Flatkey",
    description: "Chế độ xem đã lọc quyền riêng tư. Biến động khối lượng thấp bị ẩn, tỷ trọng được làm tròn và không công khai tổng token tuyệt đối.",
    dataPolicy: "Cập nhật khoảng mỗi 5 phút từ dữ liệu tổng hợp có độ trễ. Tăng trưởng bị ẩn khi mẫu kỳ trước quá nhỏ.",
    modelRankings: "Mô hình hàng đầu",
    vendorShare: "Tỷ trọng provider",
    movers: "Mô hình tăng hạng",
    droppers: "Mô hình giảm hạng",
    rank: "Hạng",
    model: "Mô hình",
    vendor: "Provider",
    share: "Tỷ trọng usage",
    growth: "Tăng trưởng",
    topModel: "Mô hình top",
    noData: "Dữ liệu xếp hạng đang được chuẩn bị. Hãy quay lại sau.",
    sampleLimited: "Không đủ mẫu",
    periods: { week: "Tuần", month: "Tháng" },
  },
  de: {
    eyebrow: "Modelle nach Nutzung",
    title: "KI-Modellrankings aus aggregierten Flatkey-Nutzungssignalen",
    description: "Eine datenschutzgefilterte Ansicht. Bewegungen mit geringem Volumen werden unterdrückt, Anteile gerundet und absolute Tokenzahlen nie veröffentlicht.",
    dataPolicy: "Etwa alle 5 Minuten aus verzögerten Aggregaten aktualisiert. Wachstum wird ausgeblendet, wenn die vorherige Stichprobe zu klein ist.",
    modelRankings: "Top-Modelle",
    vendorShare: "Provider-Anteil",
    movers: "Steigende Modelle",
    droppers: "Fallende Modelle",
    rank: "Rang",
    model: "Modell",
    vendor: "Provider",
    share: "Nutzungsanteil",
    growth: "Wachstum",
    topModel: "Top-Modell",
    noData: "Rankingdaten werden vorbereitet. Bitte später erneut prüfen.",
    sampleLimited: "k. A.",
    periods: { week: "Woche", month: "Monat" },
  },
};

export function RankingsPage({ locale, rankings }: Props) {
  const copy = COPY[locale] ?? COPY.en;
  const period = rankings.period ?? "week";

  return (
    <SiteShell locale={locale} pathname="/rankings">
      <main className="min-h-screen bg-[linear-gradient(180deg,#f7faf9_0%,#eef7f4_42%,#ffffff_100%)] text-slate-950 dark:bg-[linear-gradient(180deg,#05070a_0%,#08110f_50%,#05070a_100%)] dark:text-white">
        <div className="mx-auto w-full max-w-6xl px-6 pt-28 pb-16">
          <header className="max-w-3xl">
            <p className="inline-flex items-center gap-2 rounded-full border border-emerald-500/25 bg-emerald-500/10 px-3 py-1.5 text-xs font-bold tracking-[0.16em] text-emerald-700 uppercase dark:text-emerald-200">
              <BarChart3 className="size-3.5" />
              {copy.eyebrow}
            </p>
            <h1 className="mt-5 text-[clamp(2.2rem,5vw,4.6rem)] leading-[0.98] font-black tracking-tight">{copy.title}</h1>
            <p className="mt-5 max-w-2xl text-base leading-7 text-slate-600 dark:text-slate-300">{copy.description}</p>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-500 dark:text-slate-400">{copy.dataPolicy}</p>
            <PeriodNav current={period} copy={copy} locale={locale} />
          </header>

          {rankings.models.length === 0 ? (
            <p className="mt-10 rounded-2xl border border-slate-200 bg-white/78 p-6 text-slate-600 dark:border-white/10 dark:bg-white/[0.055] dark:text-slate-300">
              {copy.noData}
            </p>
          ) : (
            <div className="mt-10 grid gap-5 lg:grid-cols-[1.35fr_0.65fr]">
              <section className="rounded-2xl border border-slate-200 bg-white/78 p-4 shadow-[0_18px_70px_-52px_rgba(15,23,42,0.45)] dark:border-white/10 dark:bg-white/[0.055]">
                <h2 className="mb-3 flex items-center gap-2 text-lg font-black">
                  <Crown className="size-4 text-emerald-600" />
                  {copy.modelRankings}
                </h2>
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[680px] text-sm">
                    <thead className="text-left text-xs font-bold tracking-wide text-slate-500 uppercase dark:text-slate-400">
                      <tr>
                        <th className="px-3 py-3">{copy.rank}</th>
                        <th className="px-3 py-3">{copy.model}</th>
                        <th className="px-3 py-3">{copy.vendor}</th>
                        <th className="px-3 py-3 text-right">{copy.share}</th>
                        <th className="px-3 py-3 text-right">{copy.growth}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {rankings.models.map((model) => (
                        <tr key={model.model_name} className="border-t border-slate-200 dark:border-white/10">
                          <td className="px-3 py-3 font-mono font-black">#{model.rank}</td>
                          <td className="max-w-[260px] px-3 py-3">
                            <span className="block truncate font-mono font-black">{model.model_name}</span>
                          </td>
                          <td className="px-3 py-3 text-slate-600 dark:text-slate-300">{model.vendor}</td>
                          <td className="px-3 py-3 text-right font-mono font-black">{formatShare(model.share)}</td>
                          <td className="px-3 py-3 text-right">
                            <GrowthBadge value={model.growth_pct} fallback={copy.sampleLimited} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>

              <aside className="space-y-5">
                <section className="rounded-2xl border border-slate-200 bg-white/78 p-4 shadow-[0_18px_70px_-52px_rgba(15,23,42,0.45)] dark:border-white/10 dark:bg-white/[0.055]">
                  <h2 className="mb-3 flex items-center gap-2 text-lg font-black">
                    <TrendingUp className="size-4 text-emerald-600" />
                    {copy.vendorShare}
                  </h2>
                  <div className="space-y-3">
                    {rankings.vendors.slice(0, 6).map((vendor) => (
                      <div key={vendor.vendor} className="rounded-xl border border-slate-200 p-3 dark:border-white/10">
                        <div className="flex items-center justify-between gap-3">
                          <span className="font-bold">{vendor.vendor}</span>
                          <span className="font-mono text-sm font-black">{formatShare(vendor.share)}</span>
                        </div>
                        {vendor.top_model ? (
                          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                            {copy.topModel}: {vendor.top_model}
                          </p>
                        ) : null}
                      </div>
                    ))}
                  </div>
                </section>
                <MoverList title={copy.movers} items={rankings.top_movers} positive fallback={copy.sampleLimited} />
                <MoverList title={copy.droppers} items={rankings.top_droppers} fallback={copy.sampleLimited} />
              </aside>
            </div>
          )}
        </div>
      </main>
    </SiteShell>
  );
}

export function rankingsMetadataCopy(locale: Locale): { title: string; description: string } {
  const copy = COPY[locale] ?? COPY.en;
  return { title: copy.title, description: copy.description };
}

function PeriodNav(props: { current: RankingPeriod; copy: Copy; locale: Locale }) {
  return (
    <nav className="mt-7 flex flex-wrap gap-2" aria-label="Ranking period">
      {(Object.keys(props.copy.periods) as RankingPeriod[]).map((period) => (
        <a
          key={period}
          href={`${localizePath("/rankings", props.locale)}?period=${period}`}
          className={[
            "rounded-full border px-4 py-2 text-sm font-bold transition-colors",
            props.current === period
              ? "border-emerald-500 bg-emerald-600 text-white"
              : "border-slate-200 bg-white/70 text-slate-600 hover:border-emerald-400 dark:border-white/10 dark:bg-white/[0.05] dark:text-slate-300",
          ].join(" ")}
        >
          {props.copy.periods[period]}
        </a>
      ))}
    </nav>
  );
}

function GrowthBadge(props: { value?: number; fallback: string }) {
  if (props.value == null || !Number.isFinite(props.value)) {
    return <span className="font-mono text-xs font-black text-slate-500 dark:text-slate-400">{props.fallback}</span>;
  }
  const positive = props.value >= 0;
  const Icon = positive ? ArrowUpRight : ArrowDownRight;
  return (
    <span className={positive ? "font-mono font-black text-emerald-700 dark:text-emerald-200" : "font-mono font-black text-rose-600 dark:text-rose-200"}>
      <Icon className="mr-1 inline size-3.5" />
      {formatGrowth(props.value)}
    </span>
  );
}

function MoverList(props: { title: string; items: WebsiteRankingsData["top_movers"]; positive?: boolean; fallback: string }) {
  if (props.items.length === 0) return null;
  const Icon = props.positive ? ArrowUpRight : ArrowDownRight;
  return (
    <section className="rounded-2xl border border-slate-200 bg-white/78 p-4 shadow-[0_18px_70px_-52px_rgba(15,23,42,0.45)] dark:border-white/10 dark:bg-white/[0.055]">
      <h2 className="mb-3 flex items-center gap-2 text-lg font-black">
        <Icon className={props.positive ? "size-4 text-emerald-600" : "size-4 text-rose-500"} />
        {props.title}
      </h2>
      <div className="space-y-2">
        {props.items.map((item) => (
          <div key={item.model_name} className="rounded-xl border border-slate-200 p-3 dark:border-white/10">
            <p className="truncate font-mono text-sm font-black">{item.model_name}</p>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              #{item.current_rank} · {item.vendor} · {formatGrowth(item.growth_pct, props.fallback)}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}
