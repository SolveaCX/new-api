import Link from "next/link";
import { ArrowRight, BadgeDollarSign, Brain, Globe2, KeyRound, Layers3, Router, ShieldCheck, Zap } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { type Locale, localizePath } from "@/lib/locales";
import {
  KIMI_LANDING_PATH,
  getKimiLandingCtaUrl,
  getKimiLandingPageCopy,
  resolveKimiFamilyModels,
} from "@/lib/kimi-landing";
import {
  formatUsdPrice,
  getBestGroupRatio,
  getOfficialPriceUsd,
  getPricingData,
  isTokenBasedModel,
  type PricingModel,
} from "@/lib/pricing";

type Props = {
  locale: Locale;
};

const featureIcons = [Router, Brain, KeyRound, BadgeDollarSign, Layers3, Globe2] as const;

// Server component: fetches the same live pricing feed the public /models
// pages use, so the family table shows real list prices — never invented ones.
export async function KimiLandingPage({ locale }: Props) {
  const copy = getKimiLandingPageCopy(locale);
  const ctaUrl = getKimiLandingCtaUrl();
  const pricing = await getPricingData();
  const familyModels = resolveKimiFamilyModels(pricing.models);
  const pricingHref = localizePath("/pricing", locale);

  return (
    <SiteShell locale={locale} pathname={KIMI_LANDING_PATH}>
      <div className="min-h-screen overflow-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_28%,#ffffff_58%,#f4f1ff_100%)] text-slate-950 dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)] dark:text-white">
        <section className="relative border-b border-violet-500/10 pt-24 pb-20 dark:border-white/10 md:pt-32 md:pb-28">
          <div
            aria-hidden="true"
            className="absolute inset-0 bg-[radial-gradient(circle_at_46%_-18%,rgba(124,58,237,0.24),transparent_38%),radial-gradient(circle_at_82%_76%,rgba(79,70,229,0.14),transparent_34%),linear-gradient(180deg,#f6f2ff_0%,#fbfaff_48%,#ffffff_100%)] dark:bg-[radial-gradient(circle_at_50%_-20%,rgba(72,103,255,0.33),transparent_36%),radial-gradient(circle_at_86%_82%,rgba(130,80,255,0.22),transparent_32%),linear-gradient(180deg,#111a33_0%,#070911_48%,#05070d_100%)]"
          />
          <div
            aria-hidden="true"
            className="absolute inset-0 opacity-[0.34] [background-image:linear-gradient(rgba(124,58,237,0.1)_1px,transparent_1px),linear-gradient(90deg,rgba(124,58,237,0.1)_1px,transparent_1px)] [background-size:56px_56px] [mask-image:radial-gradient(ellipse_70%_58%_at_50%_18%,black_0%,transparent_78%)] dark:opacity-[0.18] dark:[background-image:linear-gradient(rgba(255,255,255,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.08)_1px,transparent_1px)]"
          />
          <div className="relative mx-auto grid max-w-7xl items-center gap-14 px-6 lg:grid-cols-[0.95fr_1.05fr]">
            <div className="mx-auto max-w-4xl text-center lg:mx-0 lg:text-left">
              <div className="inline-flex items-center gap-2 rounded-full border border-emerald-500/35 bg-emerald-50/85 px-4 py-2 font-mono text-xs font-bold tracking-[0.18em] text-emerald-700 uppercase shadow-[0_18px_48px_rgba(16,185,129,0.16)] dark:border-emerald-300/35 dark:bg-emerald-300/10 dark:text-emerald-300 dark:shadow-[0_0_40px_rgba(52,211,153,0.16)]">
                <span className="size-2 rounded-full bg-emerald-500 shadow-[0_0_16px_rgba(16,185,129,0.75)] dark:bg-emerald-300 dark:shadow-[0_0_16px_rgba(52,211,153,0.9)]" />
                {copy.badge}
              </div>

              <p className="mt-8 font-mono text-xs font-semibold tracking-[0.24em] text-violet-700 uppercase dark:text-violet-300">
                {copy.hero.eyebrow}
              </p>
              <h1 className="mt-4 text-[clamp(2.8rem,7vw,6.1rem)] leading-[0.98] font-black tracking-tight text-balance">
                {copy.hero.title}{" "}
                <span className="bg-gradient-to-r from-[#5d8cff] via-[#7f6bff] to-[#a855f7] bg-clip-text text-transparent">
                  {copy.hero.highlight}
                </span>
              </h1>
              <p className="mx-auto mt-7 max-w-3xl text-lg leading-8 font-medium text-slate-600 lg:mx-0 dark:text-slate-300 md:text-xl md:leading-9">
                {copy.hero.subtitle}
              </p>

              <div className="mt-9 flex flex-col items-center justify-center gap-4 sm:flex-row lg:justify-start">
                <a
                  href={ctaUrl}
                  className="inline-flex min-h-14 w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-[#5f86ff] to-[#8357ff] px-7 text-base font-extrabold text-white shadow-[0_22px_70px_rgba(95,134,255,0.35)] transition-transform hover:-translate-y-0.5 sm:w-auto"
                >
                  {copy.hero.primaryCta}
                  <ArrowRight className="size-4" />
                </a>
                <Link
                  href={pricingHref}
                  className="inline-flex min-h-14 w-full items-center justify-center rounded-lg border border-slate-300 bg-white/70 px-7 text-base font-extrabold text-slate-950 shadow-[0_18px_46px_rgba(15,23,42,0.08)] transition-colors hover:border-violet-400/70 dark:border-slate-600/70 dark:bg-slate-950/30 dark:text-white dark:shadow-none dark:hover:border-violet-300/60 sm:w-auto"
                >
                  {copy.hero.secondaryCta}
                </Link>
              </div>
              <p className="mt-5 text-sm font-medium text-slate-500 dark:text-slate-500">{copy.hero.trustLine}</p>
            </div>

            <CodeWindow copy={copy} />
          </div>
        </section>

        <section className="border-b border-violet-500/10 bg-white/55 px-6 py-20 dark:border-white/10 dark:bg-[#05070d] md:py-24">
          <div className="mx-auto max-w-6xl">
            <SectionHeading kicker={copy.family.kicker} title={copy.family.title} subtitle={copy.family.subtitle} />
            <div className="mt-10 overflow-x-auto rounded-lg border border-violet-200/70 bg-white/80 shadow-[0_24px_90px_rgba(79,70,229,0.12)] dark:border-slate-700/70 dark:bg-[#111720] dark:shadow-[0_24px_90px_rgba(0,0,0,0.22)]">
              <table className="w-full min-w-[40rem] text-left text-sm">
                <thead>
                  <tr className="border-b border-violet-200/70 font-mono text-xs font-bold tracking-[0.14em] text-slate-500 uppercase dark:border-slate-700/70">
                    <th className="px-6 py-4">{copy.family.modelColumn}</th>
                    <th className="px-6 py-4">{copy.family.bestForColumn}</th>
                    <th className="px-6 py-4 text-right">{copy.family.inputColumn}</th>
                    <th className="px-6 py-4 text-right">{copy.family.outputColumn}</th>
                  </tr>
                </thead>
                <tbody>
                  {copy.family.rows.map((row) => {
                    const liveModel = familyModels[row.modelId];
                    return (
                      <tr key={row.modelId} className="border-b border-violet-100/70 last:border-b-0 dark:border-slate-800">
                        <td className="px-6 py-5 align-top">
                          <div className="font-black text-slate-950 dark:text-white">{row.name}</div>
                          <code className="font-mono text-xs text-violet-700 dark:text-violet-300">{row.modelId}</code>
                        </td>
                        <td className="px-6 py-5 align-top leading-6 text-slate-600 dark:text-slate-400">{row.bestFor}</td>
                        <FamilyPriceCell model={liveModel} type="input" fallbackLabel={copy.family.priceUnavailable} pricingHref={pricingHref} groupRatio={pricing.groupRatio} />
                        <FamilyPriceCell model={liveModel} type="output" fallbackLabel={copy.family.priceUnavailable} pricingHref={pricingHref} groupRatio={pricing.groupRatio} />
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            <p className="mt-4 text-center text-xs text-slate-500 dark:text-slate-600">{copy.family.livePriceNote}</p>
          </div>
        </section>

        <section className="border-b border-violet-500/10 bg-[linear-gradient(180deg,#ffffff_0%,#f8f6ff_100%)] px-6 py-20 dark:border-white/10 dark:!bg-none dark:bg-[#05070d] md:py-24">
          <div className="mx-auto max-w-7xl">
            <SectionHeading kicker={copy.reasonsKicker} title={copy.reasonsTitle} />
            <div className="mt-10 grid gap-5 md:grid-cols-2">
              {copy.reasons.map((reason, index) => (
                <article
                  key={reason.title}
                  className="rounded-lg border border-violet-200/70 bg-white/80 p-7 shadow-[0_24px_90px_rgba(79,70,229,0.12)] dark:border-slate-700/70 dark:bg-[#111720] dark:shadow-[0_24px_90px_rgba(0,0,0,0.22)]"
                >
                  <div className="text-6xl">
                    {index === 0 ? (
                      <ShieldCheck className="size-14 text-emerald-600 dark:text-emerald-300" />
                    ) : (
                      <Zap className="size-14 text-amber-500 dark:text-amber-300" />
                    )}
                  </div>
                  <h3 className="mt-7 text-2xl font-black tracking-tight text-slate-950 dark:text-white">{reason.title}</h3>
                  <p className="mt-4 max-w-2xl text-base leading-8 text-slate-600 dark:text-slate-400">{reason.body}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="border-b border-violet-500/10 bg-[#fbfaff] px-6 py-20 dark:border-white/10 dark:bg-[#05070d] md:py-24">
          <div className="mx-auto max-w-7xl">
            <SectionHeading kicker={copy.featuresKicker} title={copy.featuresTitle} />
            <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {copy.features.map((feature, index) => {
                const Icon = featureIcons[index] ?? ShieldCheck;
                return (
                  <article key={feature.title} className="rounded-lg border border-violet-200/70 bg-white/80 p-6 shadow-[0_18px_48px_rgba(79,70,229,0.08)] dark:border-slate-800 dark:bg-[#0d121c] dark:shadow-none">
                    <Icon className="size-6 text-emerald-600 dark:text-emerald-300" />
                    <h3 className="mt-5 text-lg font-black tracking-tight text-slate-950 dark:text-white">{feature.title}</h3>
                    <p className="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-400">{feature.body}</p>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        <section className="bg-[linear-gradient(180deg,#ffffff_0%,#f4f1ff_100%)] px-6 py-20 dark:!bg-none dark:bg-[#05070d] md:py-24">
          <div className="mx-auto grid max-w-7xl gap-8 lg:grid-cols-[0.9fr_1.1fr]">
            <div className="rounded-lg border border-violet-300/40 bg-[linear-gradient(135deg,rgba(99,102,241,0.14),rgba(16,185,129,0.08),rgba(255,255,255,0.88))] p-8 shadow-[0_24px_80px_rgba(79,70,229,0.14)] dark:border-violet-400/25 dark:!bg-[linear-gradient(135deg,rgba(96,132,255,0.22),rgba(16,185,129,0.10),rgba(5,7,13,0.6))] dark:shadow-none">
              <h2 className="text-3xl font-black tracking-tight text-slate-950 dark:text-white md:text-5xl">{copy.finalCta.title}</h2>
              <p className="mt-5 text-base leading-8 text-slate-600 dark:text-slate-300">{copy.finalCta.body}</p>
              <a
                href={ctaUrl}
                className="mt-8 inline-flex min-h-12 items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-[#5f86ff] to-[#8357ff] px-6 text-sm font-black text-white shadow-[0_20px_60px_rgba(95,134,255,0.25)] transition-transform hover:-translate-y-0.5"
              >
                {copy.finalCta.button}
                <ArrowRight className="size-4" />
              </a>
            </div>
            <div className="space-y-4">
              {copy.faqs.map((faq) => (
                <article key={faq.question} className="rounded-lg border border-violet-200/70 bg-white/80 p-6 shadow-[0_18px_48px_rgba(79,70,229,0.08)] dark:border-slate-800 dark:bg-[#0d121c] dark:shadow-none">
                  <h3 className="text-base font-black text-slate-950 dark:text-white">{faq.question}</h3>
                  <p className="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-400">{faq.answer}</p>
                </article>
              ))}
            </div>
          </div>
        </section>
      </div>
    </SiteShell>
  );
}

function SectionHeading(props: { kicker: string; title: string; subtitle?: string }) {
  return (
    <div className="mx-auto max-w-4xl text-center">
      <p className="font-mono text-sm font-bold tracking-[0.26em] text-violet-700 uppercase dark:text-violet-300">{props.kicker}</p>
      <h2 className="mt-4 text-4xl leading-tight font-black tracking-tight text-slate-950 dark:text-white md:text-5xl">{props.title}</h2>
      {props.subtitle ? <p className="mt-5 text-lg leading-8 text-slate-600 dark:text-slate-400">{props.subtitle}</p> : null}
    </div>
  );
}

// Live flatkey list price (official ratio × best visible group ratio) per 1M
// tokens, straight from the pricing API. When the model is absent from the
// live payload we link to /pricing instead of showing a made-up number.
function FamilyPriceCell(props: {
  model: PricingModel | null;
  type: "input" | "output";
  fallbackLabel: string;
  pricingHref: string;
  groupRatio: Record<string, number>;
}) {
  const { model, type } = props;
  if (!model || !isTokenBasedModel(model)) {
    return (
      <td className="px-6 py-5 text-right align-top">
        <Link href={props.pricingHref} className="font-mono text-xs font-bold text-violet-700 underline-offset-4 hover:underline dark:text-violet-300">
          {props.fallbackLabel}
        </Link>
      </td>
    );
  }
  const price = getOfficialPriceUsd(model, type) * getBestGroupRatio(model, props.groupRatio);
  return (
    <td className="px-6 py-5 text-right align-top font-mono font-black text-emerald-600 dark:text-emerald-300">
      {formatUsdPrice(price)}
    </td>
  );
}

function CodeWindow({ copy }: { copy: ReturnType<typeof getKimiLandingPageCopy> }) {
  const snippets = [
    {
      label: copy.code.sdkTab,
      code: `from openai import OpenAI

client = OpenAI(
    base_url="https://router.flatkey.ai/v1",
    api_key="YOUR_FLATKEY_KEY",
)

client.chat.completions.create(
    model="${copy.code.model}",
    messages=[{"role": "user", "content": "..."}],
)`,
    },
    {
      label: copy.code.curlTab,
      code: `curl https://router.flatkey.ai/v1/chat/completions \\
  -H "Authorization: Bearer $FLATKEY_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${copy.code.model}",
    "messages": [
      {"role": "user", "content": "..."}
    ]
  }'`,
    },
  ];

  return (
    <div className="overflow-hidden rounded-lg border border-slate-800 bg-[#090d16] shadow-[0_34px_110px_rgba(79,70,229,0.20),0_14px_44px_rgba(15,23,42,0.18)] dark:border-slate-700 dark:shadow-[0_40px_120px_rgba(0,0,0,0.34)]">
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="size-2.5 rounded-full bg-red-400" />
          <span className="size-2.5 rounded-full bg-amber-300" />
          <span className="size-2.5 rounded-full bg-emerald-300" />
        </div>
        <span className="font-mono text-xs text-slate-500">{copy.code.terminalTitle}</span>
      </div>
      <div className="grid gap-4 p-4 lg:grid-cols-2">
        {snippets.map((snippet) => (
          <div key={snippet.label} className="min-w-0 rounded-lg border border-white/10 bg-[#060912]">
            <div className="border-b border-white/10 px-4 py-3 text-xs font-bold tracking-wide text-violet-300">{snippet.label}</div>
            <pre className="min-h-[18rem] overflow-x-auto p-4 font-mono text-sm leading-7 text-slate-300">
              <code>{snippet.code}</code>
            </pre>
          </div>
        ))}
      </div>
    </div>
  );
}
