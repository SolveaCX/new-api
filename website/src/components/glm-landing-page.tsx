import Link from "next/link";
import { ArrowRight, BadgeDollarSign, Cpu, Globe2, KeyRound, Layers3, Router, ShieldCheck, Zap } from "lucide-react";
import { GlmApiVisual } from "@/components/glm-api-visual";
import { SiteShell } from "@/components/site-shell";
import { type Locale, localizePath } from "@/lib/locales";
import {
  GLM_FLATKEY_PERCENT,
  GLM_LANDING_PATH,
  GLM_OFFICIAL_PERCENT,
  GLM_SAVINGS_PERCENT,
  getGlmLandingCtaUrl,
  getGlmLandingPageCopy,
} from "@/lib/glm-landing";

type Props = {
  locale: Locale;
};

const featureIcons = [Router, Cpu, BadgeDollarSign, Globe2, Layers3, KeyRound] as const;
const glmGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const glmCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const glmMutedClass = "text-[#5C5861] dark:text-white/62";

export function GlmLandingPage({ locale }: Props) {
  const copy = getGlmLandingPageCopy(locale);
  const ctaUrl = getGlmLandingCtaUrl();

  return (
    <SiteShell locale={locale} pathname={GLM_LANDING_PATH}>
      <main className="fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden="true" className={glmGridClass} />
        <section className="relative z-10 border-b-2 border-[#101014] px-4 pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-16 sm:px-6 md:pb-20 dark:border-white/20">
          <div className="relative mx-auto grid max-w-[2160px] items-center gap-14 lg:grid-cols-[0.95fr_1.05fr]">
            <div className="mx-auto max-w-4xl text-center lg:mx-0 lg:text-left">
              <div className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-[#F9F871] px-4 py-2 font-mono text-xs font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                <span className="size-2 rounded-full bg-emerald-500 shadow-[0_0_16px_rgba(16,185,129,0.75)] dark:bg-emerald-300" />
                {copy.badge}
              </div>

              <p className="mt-8 font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">
                {copy.hero.eyebrow}
              </p>
              <h1 className="mt-4 text-[clamp(2.8rem,7vw,6.1rem)] leading-[0.94] font-black tracking-normal text-balance">
                {copy.hero.title}{" "}
                <span className="text-[#5852FF] dark:text-[#C8A8FF]">
                  {copy.hero.highlight}
                </span>
              </h1>
              <p className={`mx-auto mt-7 max-w-3xl text-lg leading-8 font-semibold lg:mx-0 md:text-xl md:leading-9 ${glmMutedClass}`}>
                {copy.hero.subtitle}
              </p>

              <div className="mt-9 flex flex-col items-center justify-center gap-4 sm:flex-row lg:justify-start">
                <a
                  href={ctaUrl}
                  className="flatkey-primary-cta inline-flex min-h-14 w-full items-center justify-center gap-2 px-7 text-base sm:w-auto"
                >
                  {copy.hero.primaryCta}
                  <ArrowRight className="size-4" />
                </a>
                <Link
                  href={localizePath("/pricing", locale)}
                  className="flatkey-cta-secondary inline-flex min-h-14 w-full items-center justify-center px-7 text-base sm:w-auto"
                >
                  {copy.hero.secondaryCta}
                </Link>
              </div>
              <p className={`mt-5 text-sm font-semibold ${glmMutedClass}`}>{copy.hero.trustLine}</p>
            </div>

            <GlmApiVisual copy={copy} />
          </div>
        </section>

        <section className="relative z-10 border-b-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-[2160px]">
            <SectionHeading kicker={copy.reasonsKicker} title={copy.reasonsTitle} />
            <div className="mt-10 grid gap-5 md:grid-cols-2">
              {copy.reasons.map((reason, index) => (
                <article
                  key={reason.title}
                  className={`${glmCardClass} p-7`}
                >
                  <div className={index === 0 ? "font-mono text-6xl font-black text-emerald-600 dark:text-emerald-300" : "text-6xl"}>
                    {index === 0 ? `-${GLM_SAVINGS_PERCENT}` : <Zap className="size-14 text-amber-500 dark:text-amber-300" />}
                  </div>
                  <h3 className="mt-7 text-2xl font-black tracking-normal text-[#101014] dark:text-white">{reason.title}</h3>
                  <p className={`mt-4 max-w-2xl text-base leading-8 ${glmMutedClass}`}>{reason.body}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 border-b-2 border-[#101014] px-4 py-16 sm:px-6 md:py-18 dark:border-white/20">
          <div className="mx-auto max-w-6xl text-center">
            <SectionHeading kicker={copy.pricing.kicker} title={copy.pricing.title} subtitle={copy.pricing.subtitle} />
            <div className="mx-auto mt-12 grid max-w-5xl gap-5 md:grid-cols-2">
              <PricePanel label={copy.pricing.officialLabel} value={GLM_OFFICIAL_PERCENT} muted />
              <PricePanel label={copy.pricing.flatkeyLabel} value={GLM_FLATKEY_PERCENT} accent />
            </div>
            <div className="mx-auto mt-6 max-w-3xl rounded-full border-2 border-[#101014] bg-[#F9F871] px-5 py-4 font-mono text-sm font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
              {copy.pricing.saveLine}
            </div>
            <p className={`mt-4 text-xs ${glmMutedClass}`}>{copy.pricing.footnote}</p>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-b-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="relative mx-auto grid max-w-[2160px] items-center gap-10 lg:grid-cols-[0.82fr_1.18fr]">
            <div>
              <p className="font-mono text-sm font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">{copy.code.kicker}</p>
              <h2 className="mt-4 text-4xl font-black tracking-normal text-[#101014] dark:text-white md:text-5xl">{copy.code.title}</h2>
              <p className={`mt-5 text-lg leading-8 ${glmMutedClass}`}>{copy.code.subtitle}</p>
            </div>
            <CodeWindow copy={copy} />
          </div>
        </section>

        <section className="relative z-10 border-b-2 border-[#101014] px-4 py-16 sm:px-6 md:py-18 dark:border-white/20">
          <div className="mx-auto max-w-[2160px]">
            <SectionHeading kicker={copy.featuresKicker} title={copy.featuresTitle} />
            <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {copy.features.map((feature, index) => {
                const Icon = featureIcons[index] ?? ShieldCheck;
                return (
                  <article key={feature.title} className={`${glmCardClass} p-6`}>
                    <Icon className="size-6 text-emerald-600 dark:text-emerald-300" />
                    <h3 className="mt-5 text-lg font-black tracking-normal text-[#101014] dark:text-white">{feature.title}</h3>
                    <p className={`mt-3 text-sm leading-7 ${glmMutedClass}`}>{feature.body}</p>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto grid max-w-[2160px] gap-8 lg:grid-cols-[0.9fr_1.1fr]">
            <div className={`${glmCardClass} bg-[#F9F871]/88 p-8 dark:bg-white/8`}>
              <h2 className="text-3xl font-black tracking-normal text-[#101014] dark:text-white md:text-5xl">{copy.finalCta.title}</h2>
              <p className={`mt-5 text-base leading-8 ${glmMutedClass}`}>{copy.finalCta.body}</p>
              <a
                href={ctaUrl}
                className="flatkey-primary-cta mt-8 inline-flex min-h-12 items-center justify-center gap-2 px-6 text-sm"
              >
                {copy.finalCta.button}
                <ArrowRight className="size-4" />
              </a>
            </div>
            <div className="space-y-4">
              {copy.faqs.map((faq) => (
                <article key={faq.question} className={`${glmCardClass} p-6`}>
                  <h3 className="text-base font-black text-[#101014] dark:text-white">{faq.question}</h3>
                  <p className={`mt-3 text-sm leading-7 ${glmMutedClass}`}>{faq.answer}</p>
                </article>
              ))}
            </div>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

function SectionHeading(props: { kicker: string; title: string; subtitle?: string }) {
  return (
    <div className="mx-auto max-w-4xl text-center">
      <p className="font-mono text-sm font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">{props.kicker}</p>
      <h2 className="mt-4 text-4xl leading-tight font-black tracking-normal text-[#101014] dark:text-white md:text-5xl">{props.title}</h2>
      {props.subtitle ? <p className={`mt-5 text-lg leading-8 ${glmMutedClass}`}>{props.subtitle}</p> : null}
    </div>
  );
}

function PricePanel(props: { label: string; value: string; accent?: boolean; muted?: boolean }) {
  return (
    <div
      className={[
        `${glmCardClass} p-8`,
        props.accent
          ? "bg-[#F9F871]/82 dark:bg-emerald-300/10"
          : "bg-white/72 dark:bg-white/[0.06]",
      ].join(" ")}
    >
      <p className={`font-mono text-sm font-black uppercase ${glmMutedClass}`}>{props.label}</p>
      <div className={["mt-8 font-mono text-7xl font-black", props.muted ? "text-slate-500 dark:text-slate-300" : "text-emerald-600 dark:text-emerald-300"].join(" ")}>
        {props.value}
      </div>
    </div>
  );
}

function CodeWindow({ copy }: { copy: ReturnType<typeof getGlmLandingPageCopy> }) {
  const snippets = [
    {
      label: copy.visual.tabs[0],
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
      label: copy.visual.tabs[1],
      code: `# ~/.claude/settings.json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://router.flatkey.ai",
    "ANTHROPIC_AUTH_TOKEN": "YOUR_FLATKEY_KEY",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "glm-5.2",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "glm-5.2",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "glm-5.2",
    "CLAUDE_CODE_AUTO_COMPACT_WINDOW": "1000000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "API_TIMEOUT_MS": "3000000"
  }
}`,
    },
  ];

  return (
    <div className="overflow-hidden rounded-[1.25rem] border-2 border-[#101014] bg-[#101014] text-white shadow-[5px_5px_0_#7C3AED] dark:border-white/20 dark:shadow-[5px_5px_0_rgba(124,58,237,0.72)]">
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="size-2.5 rounded-full bg-red-400" />
          <span className="size-2.5 rounded-full bg-amber-300" />
          <span className="size-2.5 rounded-full bg-emerald-300" />
        </div>
        <span className="font-mono text-xs text-slate-500">{copy.visual.terminalTitle}</span>
      </div>
      <div className="grid gap-4 p-4 lg:grid-cols-2">
        {snippets.map((snippet) => (
          <div key={snippet.label} className="min-w-0 rounded-[1rem] border border-white/10 bg-[#060912]">
            <div className="border-b border-white/10 px-4 py-3 text-xs font-black text-[#C8A8FF]">{snippet.label}</div>
            <pre className="min-h-[26rem] overflow-x-auto p-4 font-mono text-sm leading-7 text-slate-300">
              <code>{snippet.code}</code>
            </pre>
          </div>
        ))}
      </div>
    </div>
  );
}
