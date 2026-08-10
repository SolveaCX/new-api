"use client";

import {
  Activity,
  ArrowRight,
  BadgePercent,
  ChevronRight,
  Code2,
  Gauge,
  HeartPulse,
  Timer,
  Zap,
} from "lucide-react";
import Link from "next/link";
import { type ReactNode, useEffect, useState } from "react";
import { DailyHealthBars } from "@/components/home-health-bars";
import { ModelLogo } from "@/components/pricing-model-browser";
import {
  fetchHealthSummary,
  fetchModelTrend,
  formatCallCount,
  formatLatencyMs,
  formatSuccessRate,
  formatThroughput,
  trendAvgTtftMs,
  type HomePerfSummary,
  type HomeTrendPoint,
} from "@/lib/home-live";
import type { Locale } from "@/lib/locales";
import { localizePath } from "@/lib/locales";
import {
  MODEL_PUBLIC_COPY,
  buildModelExampleCurl,
  buildModelExampleNode,
  buildModelExamplePython,
  classifyModelHealthStatus,
  modelPublicPath,
  normalizeModelKey,
  type ModelPeer,
  type ModelPublicKind,
  type ModelPublicPriceRow,
} from "@/lib/model-public";
import { buildModelFaq, buildModelHowTo, buildModelIntro, modelSeoUi } from "@/lib/model-seo-content";
import { buildModelSchema, stringifyJsonLd } from "@/lib/schema";

export type ModelPublicPageProps = {
  locale: Locale;
  modelName: string;
  vendorName: string;
  vendorDescription: string;
  description: string;
  tags: string[];
  iconKey?: string;
  endpointTypes: string[];
  kind: ModelPublicKind;
  isTokenBilled: boolean;
  // Pre-formatted on the server from the pricing payload.
  priceRows: ModelPublicPriceRow[];
  savingsPct: number;
  inputList: string;
  inputDiscounted: string;
  outputDiscounted: string;
  inputDiscountedNum: number;
  comparison: ModelPeer[];
  related: ModelPeer[];
  apiBaseUrl: string;
  consoleTopUpUrl: string;
};

type CodeLang = "curl" | "python" | "node";
const CODE_LANGS: CodeLang[] = ["curl", "python", "node"];

export function ModelPublicPage(props: ModelPublicPageProps) {
  const copy = MODEL_PUBLIC_COPY[props.locale] ?? MODEL_PUBLIC_COPY.en;
  const ui = modelSeoUi(props.locale);
  const signUpUrl = localizePath("/sign-up", props.locale);
  const modelsUrl = localizePath("/models", props.locale);
  const peerUrl = (name: string) => localizePath(modelPublicPath(name), props.locale);

  // Health data is tagged with the model it belongs to, so navigating between
  // model pages never shows the previous model's numbers while the new request
  // is in flight (we only read the state when its model matches props).
  const [health, setHealth] = useState<{
    model: string;
    trend: HomeTrendPoint[];
    summary?: HomePerfSummary;
  }>({ model: "", trend: [] });
  const [lang, setLang] = useState<CodeLang>("curl");

  useEffect(() => {
    let cancelled = false;
    const model = props.modelName;
    const wanted = normalizeModelKey(model);
    // Whole-platform health (no group) — real total volume across all tiers.
    // Pass the model so the proxy returns only this model's summary row.
    Promise.all([fetchModelTrend(model), fetchHealthSummary(undefined, model)]).then(([points, byName]) => {
      if (cancelled) return;
      const summary =
        byName[model] ?? Object.values(byName).find((row) => normalizeModelKey(row.model_name) === wanted);
      setHealth({ model, trend: points, summary });
    });
    return () => {
      cancelled = true;
    };
  }, [props.modelName]);

  const fresh = health.model === props.modelName;
  const trend = fresh ? health.trend : [];
  const trendLoaded = fresh;
  const summary = fresh ? health.summary : undefined;

  const rates = trend.map((point) => point.success_rate).filter((value) => Number.isFinite(value));
  const trendSuccess = rates.length > 0 ? rates.reduce((sum, value) => sum + value, 0) / rates.length : undefined;
  const successRate = summary?.success_rate ?? trendSuccess;
  const ttft = summary?.avg_ttft_ms ?? trendAvgTtftMs(trend);
  const healthStatus = classifyModelHealthStatus(successRate);
  const online = healthStatus === "operational";
  const degraded = healthStatus === "degraded";

  // Programmatic SEO copy (English for now) + structured data, derived purely
  // from the live pricing so every model page is unique and crawlable.
  const seoInput = {
    modelName: props.modelName,
    vendorName: props.vendorName,
    kind: props.kind,
    isTokenBilled: props.isTokenBilled,
    savingsPct: props.savingsPct,
    inputList: props.inputList,
    inputDiscounted: props.inputDiscounted,
    outputDiscounted: props.outputDiscounted,
    routerBaseUrl: props.apiBaseUrl,
    comparison: props.comparison.map((peer) => ({ modelName: peer.modelName, inputPrice: peer.inputPrice })),
  };
  const intro = buildModelIntro(seoInput, props.locale);
  const howTo = buildModelHowTo(seoInput, props.locale);
  const faq = buildModelFaq(seoInput, props.locale);
  const schema = buildModelSchema({
    locale: props.locale,
    modelName: props.modelName,
    vendorName: props.vendorName,
    description: intro,
    inputPriceUsd: props.inputDiscountedNum,
    pagePath: peerUrl(props.modelName),
    faq,
  });

  const modality = props.kind === "image" ? "Image generation" : "Text / chat";
  const example =
    lang === "python"
      ? buildModelExamplePython({ apiBaseUrl: props.apiBaseUrl, modelName: props.modelName, kind: props.kind })
      : lang === "node"
        ? buildModelExampleNode({ apiBaseUrl: props.apiBaseUrl, modelName: props.modelName, kind: props.kind })
        : buildModelExampleCurl({ apiBaseUrl: props.apiBaseUrl, modelName: props.modelName, kind: props.kind });

  const about = props.description || props.vendorDescription;

  return (
    <main className="fk-model-detail-page fk-model-surface model-square-page relative min-h-screen overflow-x-hidden bg-[#F7F4EC] px-4 pt-[var(--fk-subpage-hero-safe-area)] pb-16 text-[#101014] dark:bg-[#050507] dark:text-[#F6F3EA]">
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: stringifyJsonLd(schema) }} />
      <div aria-hidden className="fk-hero-grid pointer-events-none absolute inset-0 -z-0" />
      <div aria-hidden className="fk-hero-wash pointer-events-none absolute inset-x-0 top-0 -z-0 h-[34rem]" />
      <div className="relative z-10 mx-auto w-full max-w-[2160px]">

      {/* Breadcrumb */}
      <nav aria-label="Breadcrumb" className="mb-4 flex min-w-0 flex-wrap items-center gap-1 text-xs font-bold text-[#5C5861] dark:text-white/62">
        <Link href={localizePath("/", props.locale)} className="hover:text-[#101014] dark:hover:text-white">
          flatkey.ai
        </Link>
        <ChevronRight className="size-3" />
        <Link href={modelsUrl} className="hover:text-[#101014] dark:hover:text-white">
          {copy.backToModels}
        </Link>
        <ChevronRight className="size-3" />
        <span className="min-w-0 break-all font-mono text-[#101014]/80 dark:text-white/80">{props.modelName}</span>
      </nav>

      {/* Header */}
      <div className="fk-model-hero-card mb-4 rounded-[1.35rem] border-2 border-[#101014] bg-[#FFFDF6]/92 p-5 shadow-[6px_6px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[6px_6px_0_rgba(255,255,255,0.16)]">
        <div className="flex items-start gap-3">
          <span className="flex size-12 shrink-0 items-center justify-center rounded-2xl border-2 border-[#101014] bg-white shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/8 dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
            <ModelLogo iconKey={props.iconKey} fallback={props.modelName.charAt(0).toUpperCase()} size={28} />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="min-w-0 break-words font-mono text-2xl leading-tight font-black tracking-normal text-[#101014] sm:text-3xl dark:text-white">{props.modelName}</h1>
            {trendLoaded && (online || degraded) ? (
              <span
                className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[10px] font-bold ${
                  online
                    ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                    : "bg-amber-500/10 text-amber-700 dark:text-amber-300"
                }`}
              >
                <span className={`size-1.5 rounded-full ${online ? "bg-emerald-500" : "bg-amber-500"}`} />
                {online ? copy.statusOnline : copy.statusDegraded}
              </span>
            ) : null}
            </div>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs font-semibold text-[#5C5861] dark:text-white/62">
            <span>{props.vendorName}</span>
            {props.endpointTypes.map((endpoint) => (
              <span
                key={endpoint}
                className="rounded-full border border-[#7C3AED]/25 bg-[#EEE4FF] px-2 py-0.5 font-mono text-[10px] font-bold text-[#4C1D95] dark:bg-[#7C3AED]/20 dark:text-[#C8A8FF]"
              >
                {endpoint}
              </span>
            ))}
            {props.tags.map((tag) => (
              <span key={tag} className="rounded-full border border-[#101014]/14 bg-white/68 px-2 py-0.5 text-[10px] font-bold dark:border-white/10 dark:bg-white/[0.06]">
                {tag}
              </span>
            ))}
          </div>
          </div>
        </div>

      {/* Intro + primary CTA */}
      <section className="mt-4">
        <p className="text-sm leading-relaxed text-[#37323F] dark:text-slate-200/82">{intro}</p>
        {about ? <p className="mt-2 text-sm leading-relaxed text-[#5C5861] dark:text-slate-300/72">{about}</p> : null}
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <Link
            href={signUpUrl}
            className="fk-button-motion inline-flex h-10 items-center gap-1.5 rounded-full border-2 border-[#101014] !bg-[#101014] px-4 text-sm font-black !text-white shadow-[4px_4px_0_#7C3AED] transition-opacity hover:opacity-90 dark:border-white dark:!bg-white dark:!text-[#101014]"
          >
            {ui.ctaSignUp}
            <ArrowRight className="size-4" />
          </Link>
          {props.savingsPct > 0 ? (
            <span className="text-xs font-semibold text-[#5C5861] dark:text-slate-300/72">
              {ui.saveVsOfficial.replace("{pct}", String(props.savingsPct))}
            </span>
          ) : null}
        </div>
      </section>
      </div>

      {/* Performance stat band */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={<HeartPulse className="size-3.5" />}
          label={copy.successRate}
          value={trendLoaded ? formatSuccessRate(successRate) : "…"}
          tone="emerald"
        />
        <StatCard
          icon={<Zap className="size-3.5" />}
          label={copy.throughput}
          value={trendLoaded ? formatThroughput(summary?.avg_tps) : "…"}
          tone="violet"
        />
        <StatCard
          icon={<Timer className="size-3.5" />}
          label={copy.ttft}
          value={trendLoaded ? formatLatencyMs(ttft) : "…"}
          tone="violet"
        />
        <StatCard
          icon={<Activity className="size-3.5" />}
          label={copy.requests}
          value={trendLoaded ? formatCallCount(summary?.request_count) : "…"}
          tone="violet"
        />
      </div>

      {/* Discount */}
      <div className="mt-3 rounded-[1.1rem] border-2 border-[#101014] bg-[#F9F871]/72 p-4 shadow-[4px_4px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#F9F871]/14 dark:shadow-[4px_4px_0_rgba(255,255,255,0.16)]">
        <div className="flex items-center gap-1.5 text-[11px] font-black tracking-normal text-[#4C1D95] uppercase dark:text-[#C8A8FF]">
          <BadgePercent className="size-3.5" />
          {copy.stackedDiscount}
        </div>
        <div className="mt-1 flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <span className="text-3xl font-black text-[#101014] dark:text-white">{copy.upToOff}</span>
          <a
            href={props.consoleTopUpUrl}
            className="text-xs font-bold text-[#5C5861] underline decoration-dotted underline-offset-2 hover:text-[#101014] dark:text-slate-300/72 dark:hover:text-white"
          >
            {copy.discountNote} →
          </a>
        </div>
      </div>

      {/* Pricing */}
      <Section title={copy.pricing}>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {props.priceRows.map((row) => (
            <div key={row.labelKey} className="rounded-[1rem] border-2 border-[#101014]/16 bg-white/72 p-4 dark:border-white/14 dark:bg-white/[0.06]">
              <div className="text-xs font-black text-[#5C5861] dark:text-slate-300/72">{copy[row.labelKey]}</div>
              <div className="mt-1 font-mono text-sm font-semibold text-[#6D6A72] tabular-nums dark:text-slate-300/60">
                {copy.listPrice} <span className="line-through">{row.list}</span>
              </div>
              <div className="mt-0.5 font-mono text-2xl font-black text-emerald-600 tabular-nums dark:text-emerald-400">
                {row.discounted}
                <span className="ml-1 text-sm font-semibold text-[#6D6A72] dark:text-slate-300/58">{copy.perMTokens}</span>
              </div>
            </div>
          ))}
        </div>
      </Section>

      {/* Specifications */}
      <Section title={ui.specs}>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
          <SpecRow label={ui.provider} value={props.vendorName} />
          <SpecRow label={ui.modality} value={modality} />
          <SpecRow label={ui.access} value="OpenAI-compatible API" />
          <SpecRow label={ui.endpoints} value={props.endpointTypes.join(", ") || "—"} mono />
        </dl>
      </Section>

      {/* How to use */}
      <Section title={ui.howToTitle.replace("{model}", props.modelName)}>
        <ol className="mb-4 space-y-3">
          {howTo.map((step, index) => (
            <li key={step.title} className="flex gap-3">
              <span className="flex size-6 shrink-0 items-center justify-center rounded-full border-2 border-[#101014] bg-[#F9F871] font-mono text-xs font-black text-[#101014] shadow-[1px_1px_0_#101014] dark:border-white/24 dark:shadow-[1px_1px_0_rgba(255,255,255,0.16)]">
                {index + 1}
              </span>
              <div>
                <div className="text-sm font-black">{step.title}</div>
                <div className="text-sm leading-relaxed text-[#5C5861] dark:text-slate-300/72">{step.body}</div>
              </div>
            </li>
          ))}
        </ol>
        <div className="mb-2 flex items-center justify-between gap-2">
          <span className="flex items-center gap-1.5 text-xs font-bold text-[#5C5861] dark:text-slate-300/72">
            <Code2 className="size-3.5" />
            {copy.apiTitle}
          </span>
          <div className="flex gap-1">
            {CODE_LANGS.map((item) => (
              <button
                key={item}
                type="button"
                onClick={() => setLang(item)}
                className={`rounded-md px-2 py-0.5 font-mono text-[11px] transition-colors ${
                  lang === item
                    ? "bg-[#EEE4FF] font-black text-[#4C1D95] dark:bg-[#7C3AED]/20 dark:text-[#C8A8FF]"
                    : "font-bold text-[#5C5861] hover:text-[#101014] dark:text-slate-300/72 dark:hover:text-white"
                }`}
              >
                {item}
              </button>
            ))}
          </div>
        </div>
        <pre className="overflow-x-auto rounded-xl border border-white/10 bg-[#0B0B0F] p-4 font-mono text-xs leading-relaxed text-zinc-100 shadow-[0_22px_60px_-44px_rgba(11,11,15,0.72)]">
          {example}
        </pre>
      </Section>

      {/* Comparison */}
      {props.comparison.length > 0 ? (
        <Section title={ui.compareTitle.replace("{model}", props.modelName)}>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[#101014]/12 text-left text-xs font-black text-[#5C5861] dark:border-white/12 dark:text-slate-300/72">
                  <th className="py-2 font-black">{ui.colModel}</th>
                  <th className="py-2 text-right font-black">{ui.colInputPrice}</th>
                </tr>
              </thead>
              <tbody>
                <tr className="border-b border-[#101014]/12 bg-[#F9F871]/22 dark:border-white/12 dark:bg-white/[0.06]">
                  <td className="py-2 font-mono font-semibold">{props.modelName}</td>
                  <td className="py-2 text-right font-mono tabular-nums">{props.inputDiscounted}</td>
                </tr>
                {props.comparison.map((peer) => (
                  <tr key={peer.modelName} className="border-b border-[#101014]/10 last:border-0 dark:border-white/10">
                    <td className="py-2">
                      <Link href={peerUrl(peer.modelName)} className="font-mono font-bold text-[#7C3AED] hover:underline dark:text-[#C8A8FF]">
                        {peer.modelName}
                      </Link>
                    </td>
                    <td className="py-2 text-right font-mono tabular-nums">{peer.inputPrice}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Section>
      ) : null}

      {/* 30-day availability trend */}
      <Section title={copy.availability} icon={<Gauge className="size-3.5" />}>
        <div className="h-16">
          {trend.length > 1 ? (
            <DailyHealthBars points={trend} label={copy.availability} heightPx={64} />
          ) : (
            <div className="flex h-full items-center text-xs font-semibold text-[#5C5861] dark:text-slate-300/58">
              {trendLoaded ? copy.noData : "…"}
            </div>
          )}
        </div>
      </Section>

      {/* FAQ */}
      <Section title={ui.faqTitle}>
        <div className="divide-y divide-[#101014]/10 dark:divide-white/10">
          {faq.map((item) => (
            <div key={item.q} className="py-3 first:pt-0 last:pb-0">
              <h3 className="text-sm font-black">{item.q}</h3>
              <p className="mt-1 text-sm leading-relaxed text-[#5C5861] dark:text-slate-300/72">{item.a}</p>
            </div>
          ))}
        </div>
      </Section>

      {/* Related models */}
      {props.related.length > 0 ? (
        <Section title={ui.relatedTitle.replace("{vendor}", props.vendorName)}>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {props.related.map((peer) => (
              <Link
                key={peer.modelName}
                href={peerUrl(peer.modelName)}
                className="fk-card-motion flex min-w-0 items-center justify-between rounded-lg border-2 border-[#101014]/14 bg-white/68 px-3 py-2 text-sm transition-colors hover:border-[#101014] hover:bg-[#F9F871]/28 dark:border-white/12 dark:bg-white/[0.06] dark:hover:border-white/28"
              >
                <span className="min-w-0 truncate font-mono font-bold">{peer.modelName}</span>
                <span className="ml-2 shrink-0 font-mono text-xs font-semibold text-[#5C5861] tabular-nums dark:text-slate-300/72">{peer.inputPrice}</span>
              </Link>
            ))}
          </div>
        </Section>
      ) : null}

      {/* Final CTA */}
      <section className="mt-6 rounded-[1.35rem] border-2 border-[#101014] bg-[#FFFDF6]/92 p-6 text-center shadow-[6px_6px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[6px_6px_0_rgba(255,255,255,0.16)]">
        <h2 className="break-words text-lg font-black">{ui.ctaTitle.replace("{model}", props.modelName)}</h2>
        <p className="mx-auto mt-1 max-w-xl text-sm font-semibold text-[#5C5861] dark:text-slate-300/72">{ui.ctaSubtitle}</p>
        <Link
          href={signUpUrl}
          className="fk-button-motion mt-4 inline-flex h-11 items-center gap-1.5 rounded-full border-2 border-[#101014] !bg-[#101014] px-5 text-sm font-black !text-white shadow-[4px_4px_0_#7C3AED] transition-opacity hover:opacity-90 dark:border-white dark:!bg-white dark:!text-[#101014]"
        >
          {ui.ctaSignUp}
          <ArrowRight className="size-4" />
        </Link>
      </section>
      </div>
    </main>
  );
}

function Section(props: { title: string; icon?: ReactNode; children: ReactNode }) {
  return (
    <section className="mt-4 rounded-[1.1rem] border-2 border-[#101014] bg-[#FFFDF6]/90 p-4 shadow-[4px_4px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[4px_4px_0_rgba(255,255,255,0.16)]">
      <h2 className="mb-3 flex items-center gap-1.5 text-xs font-black tracking-normal text-[#7C3AED] uppercase dark:text-[#C8A8FF]">
        {props.icon}
        {props.title}
      </h2>
      {props.children}
    </section>
  );
}

function SpecRow(props: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-baseline justify-between gap-3 border-b border-dashed border-[#101014]/14 py-1.5 last:border-0 dark:border-white/12">
      <dt className="text-xs font-semibold text-[#5C5861] dark:text-slate-300/72">{props.label}</dt>
      <dd className={`min-w-0 break-words text-right text-sm font-bold ${props.mono ? "font-mono text-xs" : ""}`}>{props.value}</dd>
    </div>
  );
}

function StatCard(props: { icon: ReactNode; label: string; value: string; tone: "emerald" | "violet" }) {
  const border =
    props.tone === "emerald"
      ? "border-emerald-600/45 bg-emerald-500/[0.10]"
      : "border-[#101014] bg-[#FFFDF6]/90 dark:border-white/24 dark:bg-[#111116]/88";
  const text = props.tone === "emerald" ? "text-emerald-700 dark:text-emerald-300" : "text-[#101014] dark:text-white";
  return (
    <div className={`rounded-[1.1rem] border-2 p-4 shadow-[4px_4px_0_#101014] backdrop-blur-sm dark:shadow-[4px_4px_0_rgba(255,255,255,0.16)] ${border}`}>
      <div className="flex items-center gap-1.5 text-[11px] font-black tracking-normal text-[#5C5861] uppercase dark:text-slate-300/72">
        {props.icon}
        {props.label}
      </div>
      <div className={`mt-1 font-mono text-2xl font-black tabular-nums ${text}`}>{props.value}</div>
    </div>
  );
}
