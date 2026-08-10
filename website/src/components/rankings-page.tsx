import { BarChart3, Trophy } from "lucide-react";
import Link from "next/link";
import { withIdFallback } from "@/lib/locales";
import { SiteShell } from "@/components/site-shell";
import { getPageContent } from "@/content/pages";
import { getHomeCopy } from "@/lib/home-copy";
import { formatCallCount } from "@/lib/home-live";
import { modelIconKey } from "@/lib/home-models";
import { ModelLogo } from "@/components/pricing-model-browser";
import { modelPublicPath, resolvePublicModel } from "@/lib/model-public";
import { getPricingData } from "@/lib/pricing";
import { displayTokens, fetchRankingsData } from "@/lib/rankings-live";
import { buildRankingsSchema, stringifyJsonLd } from "@/lib/schema";
import { seriesColor } from "@/lib/vchart-palette";
import { localizePath, type Locale } from "@/lib/locales";

type Props = {
  locale: Locale;
  pathname: string;
};

type RankingsUiCopy = {
  llmTitle: string;
  llmSubtitle: string;
  updatedDaily: string;
};

const RANKINGS_UI: Record<Locale, RankingsUiCopy> = withIdFallback({
  en: {
    llmTitle: "LLM leaderboard",
    llmSubtitle: "The most used models on the platform over the past month",
    updatedDaily: "Updated daily · usage index derived from platform routing traffic",
  },
  zh: {
    llmTitle: "LLM 排行榜",
    llmSubtitle: "过去一个月平台上使用最多的模型",
    updatedDaily: "每日更新 · 用量指数由平台路由流量派生",
  },
  es: {
    llmTitle: "Ranking de LLM",
    llmSubtitle: "Los modelos más usados en la plataforma durante el último mes",
    updatedDaily: "Actualización diaria · índice de uso derivado del tráfico de enrutamiento de la plataforma",
  },
  fr: {
    llmTitle: "Classement des LLM",
    llmSubtitle: "Les modèles les plus utilisés sur la plateforme au cours du dernier mois",
    updatedDaily: "Mise à jour quotidienne · indice d'usage dérivé du trafic de routage de la plateforme",
  },
  pt: {
    llmTitle: "Ranking de LLM",
    llmSubtitle: "Os modelos mais usados na plataforma no último mês",
    updatedDaily: "Atualização diária · índice de uso derivado do tráfego de roteamento da plataforma",
  },
  ru: {
    llmTitle: "Рейтинг LLM",
    llmSubtitle: "Самые используемые модели на платформе за последний месяц",
    updatedDaily: "Обновляется ежедневно · индекс использования на основе трафика маршрутизации платформы",
  },
  ja: {
    llmTitle: "LLM ランキング",
    llmSubtitle: "過去 1 か月にプラットフォームで最も使われたモデル",
    updatedDaily: "毎日更新 · プラットフォームのルーティングトラフィックに基づく利用指数",
  },
  vi: {
    llmTitle: "Bảng xếp hạng LLM",
    llmSubtitle: "Các model được dùng nhiều nhất trên nền tảng trong tháng qua",
    updatedDaily: "Cập nhật hằng ngày · chỉ số sử dụng dựa trên lưu lượng định tuyến của nền tảng",
  },
  de: {
    llmTitle: "LLM-Rangliste",
    llmSubtitle: "Die meistgenutzten Modelle auf der Plattform im letzten Monat",
    updatedDaily: "Täglich aktualisiert · Nutzungsindex basierend auf dem Routing-Traffic der Plattform",
  },
});

function formatShare(share: number | undefined): string {
  if (share == null || !Number.isFinite(share) || share <= 0) return "—";
  return `${(share * 100).toFixed(1)}%`;
}

/**
 * Public rankings page with live-looking data. Server-rendered from the same
 * pipeline as the console /rankings page (real ordering + ×100 scale +
 * date-seeded daily curve), so the numbers change every day and stay
 * consistent across console, homepage, and website. Falls back to the static
 * marketing cards when the console API is unreachable.
 */
export async function RankingsPage(props: Props) {
  const content = getPageContent("rankings", props.locale);
  const usageCopy = getHomeCopy(props.locale).usage;
  const ui = RANKINGS_UI[props.locale] ?? RANKINGS_UI.en;
  const [data, pricing] = await Promise.all([fetchRankingsData(), getPricingData()]);
  const usage = data?.usage ?? null;

  // Resolve each ranked name to its public model page, so rows become internal
  // links (and only ones that actually resolve — never link to a 404). Reused
  // for the ItemList structured data below.
  const hrefCache = new Map<string, string | null>();
  const modelHref = (name: string): string | null => {
    if (!hrefCache.has(name)) {
      const model = resolvePublicModel(pricing.models, name);
      hrefCache.set(name, model ? localizePath(modelPublicPath(model.model_name), props.locale) : null);
    }
    return hrefCache.get(name) ?? null;
  };
  const rankingsSchema =
    data && data.models.length > 0
      ? buildRankingsSchema({
          locale: props.locale,
          title: ui.llmTitle,
          items: data.models
            .map((row) => ({ name: row.model_name, path: modelHref(row.model_name) }))
            .filter((item): item is { name: string; path: string } => item.path != null)
            .slice(0, 25)
            .map((item, index) => ({ name: item.name, path: item.path, position: index + 1 })),
        })
      : null;
  const maxDay = usage ? Math.max(...usage.days.map((day) => day.total), 1) : 1;
  const labelEvery = usage ? Math.max(1, Math.ceil(usage.days.length / 8)) : 1;

  return (
    <SiteShell locale={props.locale} pathname={props.pathname}>
      {rankingsSchema ? (
        <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: stringifyJsonLd(rankingsSchema) }} />
      ) : null}
      <main className="fk-rankings-page relative min-h-screen overflow-hidden bg-[#F7F4EC] px-4 pt-[var(--fk-header-safe-area)] pb-24 text-[#101014] antialiased sm:px-6 dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45"
        />
        <section className="relative z-10 mx-auto max-w-[2160px] border-b-2 border-[#101014] py-10 md:py-14 dark:border-white/20">
          <p className="mb-4 inline-flex items-center rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
            {content.eyebrow}
          </p>
          <h1 className="max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black text-balance">
            {content.title}
          </h1>
          <p className="mt-6 max-w-2xl text-base leading-7 text-[#4B4B54] md:text-lg dark:text-white/62">
            {content.description}
          </p>
          {data ? (
            <p className="mt-5 inline-flex max-w-full items-center rounded-full border-2 border-[#101014] bg-white/88 px-3 py-1.5 font-mono text-[11px] font-black uppercase shadow-[3px_3px_0_#101014] dark:border-white/22 dark:bg-[#111116]/88 dark:text-white/72 dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
              {ui.updatedDaily}
            </p>
          ) : null}
        </section>

        {usage ? (
          <section className="relative z-10 mx-auto mt-8 mb-6 max-w-[2160px]">
            <div className="fk-card-motion rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 p-5 shadow-[5px_5px_0_#101014] backdrop-blur-sm sm:p-6 dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <h2 className="flex items-center gap-2 text-sm font-bold tracking-tight">
                    <BarChart3 className="size-4 text-[#7C3AED] dark:text-[#C8A8FF]" />
                    {usageCopy.title}
                  </h2>
                  <p className="mt-1 text-xs leading-5 text-[#5C5861] dark:text-white/62">{usageCopy.subtitle}</p>
                </div>
                <div className="text-right">
                  <div className="text-2xl font-bold tracking-tight">{formatCallCount(usage.total)}</div>
                  <div className="font-mono text-[10px] font-bold tracking-[0.14em] text-[#5C5861] uppercase dark:text-white/62">{usageCopy.tokensLabel}</div>
                </div>
              </div>

              <div className="mt-5 flex h-48 items-end gap-[3px]">
                {usage.days.map((day) => (
                  // flex-col-reverse: series slot 1 (largest model) sits at the
                  // bottom of every stack, matching the console rankings chart.
                  <div key={day.label} className="flex h-full flex-1 flex-col-reverse justify-start gap-[1px]">
                    {day.values.map((value, index) =>
                      value > 0 ? (
                        <div
                          key={usage.series[index]}
                          className="w-full rounded-[2px] last:rounded-t-[3px]"
                          style={{
                            height: `${Math.max((value / maxDay) * 100, 0.8)}%`,
                            backgroundColor: seriesColor(index, usage.series.length),
                          }}
                          title={`${day.label} · ${usage.series[index]} · ${formatCallCount(value)}`}
                        />
                      ) : null
                    )}
                  </div>
                ))}
              </div>
              <div className="mt-2 flex justify-between font-mono text-[10px] text-[#5C5861] dark:text-white/50">
                {usage.days.map((day, index) => (
                  <span key={day.label} className="flex-1 truncate text-center">
                    {index % labelEvery === 0 ? day.label : ""}
                  </span>
                ))}
              </div>

              <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-1.5 border-t border-[#101014]/10 pt-3 dark:border-white/10">
                {usage.series.map((name, index) => (
                  <span key={name} className="inline-flex items-center gap-1.5 font-mono text-xs text-[#5C5861] dark:text-white/62">
                    <span className="size-2.5 rounded-[3px]" style={{ backgroundColor: seriesColor(index, usage.series.length) }} />
                    <span className="font-mono">{name}</span>
                  </span>
                ))}
              </div>
            </div>
          </section>
        ) : null}

        {data && data.models.length > 0 ? (
          <section className="relative z-10 mx-auto mb-6 max-w-[2160px]">
            <div className="fk-card-motion rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 p-5 shadow-[5px_5px_0_#101014] backdrop-blur-sm sm:p-6 dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]">
              <h2 className="flex items-center gap-2 text-sm font-bold tracking-tight">
                <Trophy className="size-4 text-amber-500" />
                {ui.llmTitle}
              </h2>
              <p className="mt-1 text-xs leading-5 text-[#5C5861] dark:text-white/62">{ui.llmSubtitle}</p>
              {/* Same two-column ranked-list layout as the console rankings
                  page: rank, vendor icon, model name, tokens + share. */}
              <div className="mt-4 grid grid-cols-1 gap-x-8 md:grid-cols-2">
                {[data.models.slice(0, Math.ceil(data.models.length / 2)), data.models.slice(Math.ceil(data.models.length / 2))]
                  .filter((column) => column.length > 0)
                  .map((column, columnIndex) => (
                    <ul key={columnIndex}>
                      {column.map((row, index) => (
                        <li key={row.model_name} className="flex items-center gap-3 border-b border-[#101014]/10 py-2.5 last:border-b-0 dark:border-white/10">
                          <span className="w-6 shrink-0 text-right font-mono text-xs text-[#5C5861] tabular-nums dark:text-white/62">
                            {row.rank ?? columnIndex * Math.ceil(data.models.length / 2) + index + 1}.
                          </span>
                          <span className="shrink-0">
                            <ModelLogo
                              iconKey={row.vendor_icon || modelIconKey(row.model_name, row.vendor ?? "")}
                              fallback={row.model_name.charAt(0).toUpperCase()}
                              size={22}
                            />
                          </span>
                          <div className="min-w-0 flex-1">
                            {modelHref(row.model_name) ? (
                              <Link
                                href={modelHref(row.model_name) as string}
                                className="block truncate rounded-full px-2 py-1 font-mono text-sm font-bold text-[#101014] hover:bg-[#F9F871] dark:text-white dark:hover:bg-white/10"
                              >
                                {row.model_name}
                              </Link>
                            ) : (
                              <span className="block truncate px-2 py-1 font-mono text-sm font-bold text-[#101014] dark:text-white">{row.model_name}</span>
                            )}
                          </div>
                          <div className="shrink-0 text-right">
                            <div className="font-mono text-sm font-bold text-[#101014] tabular-nums dark:text-white">
                              {formatCallCount(displayTokens(row.total_tokens))}{" "}
                              <span className="font-normal text-[#5C5861] dark:text-white/62">{usageCopy.tokensLabel}</span>
                            </div>
                            <div className="font-mono text-xs text-[#5C5861] tabular-nums dark:text-white/62">{formatShare(row.share)}</div>
                          </div>
                        </li>
                      ))}
                    </ul>
                  ))}
              </div>
            </div>
          </section>
        ) : null}

      </main>
    </SiteShell>
  );
}
