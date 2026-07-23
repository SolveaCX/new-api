import Link from "next/link";
import { ArrowRight, Clapperboard, Cpu, KeyRound, Layers3, ShieldCheck, Users, Wallet, Zap } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { type Locale, localizePath } from "@/lib/locales";
import {
  COMPUTE_LANDING_PATH,
  getComputeLandingCtaUrl,
  getComputeLandingPageCopy,
} from "@/lib/compute-landing";

type Props = {
  locale: Locale;
};

const productIcons = [Zap, Cpu, Clapperboard] as const;
const unifiedIcons = [KeyRound, Wallet, ShieldCheck] as const;
const whoForIcons = [Layers3, Users, Cpu] as const;

export function ComputeLandingPage({ locale }: Props) {
  const copy = getComputeLandingPageCopy(locale);
  const ctaUrl = getComputeLandingCtaUrl();

  return (
    <SiteShell locale={locale} pathname={COMPUTE_LANDING_PATH}>
      <main className="home-landing relative overflow-x-hidden bg-[linear-gradient(180deg,#f4f0ff_0%,#fbfaff_28%,#ffffff_58%,#f4f1ff_100%)] dark:bg-[linear-gradient(180deg,#050712_0%,#080b18_36%,#070712_72%,#03040b_100%)]">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-0 bg-[linear-gradient(to_right,rgba(124,58,237,0.08)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.08)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-45"
        />

        {/* Hero: three compute product tiles, unified balance, softly styled. */}
        <section className="relative z-10 overflow-hidden px-6 pt-24 pb-14 md:pt-32 md:pb-20 lg:pt-36">
          <div
            aria-hidden
            className="home-hero-glow pointer-events-none absolute inset-0 -z-10 opacity-40 dark:opacity-55"
            style={{ background: "var(--home-hero-glow)" }}
          />
          <div
            aria-hidden
            className="absolute inset-0 -z-10 bg-[linear-gradient(to_right,rgba(124,58,237,0.16)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.14)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_64%_52%_at_50%_28%,black_20%,transparent_100%)] bg-[size:4rem_4rem] opacity-35 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.06)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.05)_1px,transparent_1px)] dark:opacity-40"
          />

          <div className="mx-auto grid max-w-6xl grid-cols-1 items-center gap-12 lg:grid-cols-12 lg:gap-8">
            <div className="flex flex-col items-start text-left lg:col-span-7">
              <div className="landing-animate-fade-up mb-5 inline-flex items-center gap-1.5 rounded-full border border-violet-500/25 bg-violet-500/10 px-3 py-1.5 text-[11px] font-medium text-violet-700 opacity-0 shadow-[0_12px_34px_-22px_rgba(124,58,237,0.75)]">
                <span className="relative flex size-1.5">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-violet-400 opacity-75" />
                  <span className="relative inline-flex size-1.5 rounded-full bg-violet-500" />
                </span>
                <span>{copy.badge}</span>
              </div>

              <p className="landing-animate-fade-up text-muted-foreground mb-3 text-xs font-medium tracking-widest uppercase opacity-0" style={{ animationDelay: "40ms" }}>
                {copy.hero.eyebrow}
              </p>

              <h1 className="landing-animate-fade-up text-[clamp(2.25rem,4.5vw,3.25rem)] leading-[1.15] font-bold tracking-tight" style={{ animationDelay: "60ms" }}>
                {copy.hero.title}{" "}
                <span className="bg-gradient-to-r from-violet-500 via-fuchsia-500 to-indigo-500 bg-clip-text text-transparent dark:from-violet-200 dark:via-fuchsia-300 dark:to-indigo-300">
                  {copy.hero.highlight}
                </span>
              </h1>
              <p className="landing-animate-fade-up text-muted-foreground/80 mt-5 max-w-xl text-base leading-relaxed opacity-0 md:text-[15px]" style={{ animationDelay: "120ms" }}>
                {copy.hero.subtitle}
              </p>

              <div className="landing-animate-fade-up mt-8 flex flex-wrap items-center gap-3 opacity-0" style={{ animationDelay: "180ms" }}>
                <a
                  className="flatkey-hero-cta group inline-flex h-11 items-center px-5 text-sm font-medium shadow-[0_16px_34px_-18px_rgba(124,58,237,0.85)] transition-colors hover:opacity-90"
                  href={ctaUrl}
                  style={{ borderRadius: "0.5rem" }}
                >
                  {copy.hero.primaryCta}
                  <ArrowRight className="ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5" />
                </a>
                <Link
                  className="inline-flex h-11 items-center rounded-lg border border-violet-500/20 bg-white/65 px-5 text-sm font-medium hover:border-violet-500/35 hover:bg-violet-500/10"
                  href={localizePath("/pricing", locale)}
                >
                  {copy.hero.secondaryCta}
                </Link>
              </div>

              <p className="landing-animate-fade-up text-muted-foreground mt-5 text-sm opacity-0" style={{ animationDelay: "220ms" }}>
                {copy.hero.trustLine}
              </p>
            </div>

            <div className="landing-animate-fade-up flex w-full justify-center opacity-0 lg:col-span-5 lg:justify-end" style={{ animationDelay: "260ms" }}>
              <ComputeVisual copy={copy} />
            </div>
          </div>
        </section>

        {/* Three product lines. */}
        <section className="relative z-10 overflow-hidden px-6 py-20 md:py-24">
          <div className="absolute inset-0 -z-10 bg-[linear-gradient(to_right,rgba(124,58,237,0.12)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.1)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_60%_52%_at_50%_42%,black_18%,transparent_90%)] bg-[size:4rem_4rem] opacity-40 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-40" />
          <div className="mx-auto max-w-6xl">
            <SectionHeading kicker={copy.productsKicker} title={copy.productsTitle} />
            <div className="mt-12 grid gap-5 md:grid-cols-3">
              {copy.products.map((product, index) => {
                const Icon = productIcons[index] ?? Zap;
                return (
                  <article
                    key={product.name}
                    className="flex flex-col rounded-3xl border border-violet-500/12 bg-white/70 p-7 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm transition-colors duration-300 hover:border-violet-500/24 hover:bg-white/80 md:p-8 dark:bg-white/[0.03] dark:hover:bg-white/[0.06]"
                  >
                    <div className="mb-6 flex size-14 items-center justify-center rounded-2xl border border-violet-500/20 bg-violet-500/8 text-violet-700 shadow-[0_18px_44px_-30px_rgba(124,58,237,0.8)] dark:text-violet-300">
                      <Icon className="size-6" strokeWidth={1.6} />
                    </div>
                    <h3 className="text-xl font-semibold tracking-tight">{product.name}</h3>
                    <p className="text-muted-foreground mt-3 text-sm leading-6">{product.tagline}</p>
                    <div className="mt-6 rounded-xl border border-violet-500/15 bg-violet-500/8 px-4 py-3 font-mono text-sm font-semibold tracking-tight text-violet-700 dark:text-violet-300">
                      {product.price}
                    </div>
                    <div className="mt-auto pt-6">
                      <p className="text-muted-foreground text-xs font-medium tracking-widest uppercase">{product.fitLabel}</p>
                      <p className="text-muted-foreground mt-2 text-sm leading-6">{product.fit}</p>
                    </div>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        {/* Unified billing / whitelabel. */}
        <section className="relative z-10 border-t border-violet-500/10 px-6 py-20 md:py-24">
          <div className="mx-auto max-w-6xl">
            <SectionHeading kicker={copy.unifiedKicker} title={copy.unifiedTitle} subtitle={copy.unifiedSubtitle} />
            <div className="mt-12 grid gap-5 md:grid-cols-3">
              {copy.unifiedPoints.map((point, index) => {
                const Icon = unifiedIcons[index] ?? KeyRound;
                return (
                  <article
                    key={point.title}
                    className="flex flex-col rounded-3xl border border-violet-500/12 bg-white/70 p-7 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm transition-colors duration-300 hover:border-violet-500/24 hover:bg-white/80 dark:bg-white/[0.03] dark:hover:bg-white/[0.06]"
                  >
                    <div className="mb-6 flex size-14 items-center justify-center rounded-2xl border border-violet-500/20 bg-violet-500/8 text-violet-700 shadow-[0_18px_44px_-30px_rgba(124,58,237,0.8)] dark:text-violet-300">
                      <Icon className="size-6" strokeWidth={1.6} />
                    </div>
                    <h3 className="text-lg font-semibold tracking-tight">{point.title}</h3>
                    <p className="text-muted-foreground mt-3 text-sm leading-6">{point.body}</p>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        {/* Price comparison. */}
        <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-20 md:py-24">
          <div className="absolute inset-0 -z-10 bg-[linear-gradient(to_right,rgba(124,58,237,0.12)_1px,transparent_1px),linear-gradient(to_bottom,rgba(124,58,237,0.1)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_58%_52%_at_50%_44%,black_16%,transparent_88%)] bg-[size:4rem_4rem] opacity-40 dark:bg-[linear-gradient(to_right,rgba(148,163,184,0.055)_1px,transparent_1px),linear-gradient(to_bottom,rgba(148,163,184,0.045)_1px,transparent_1px)] dark:opacity-40" />
          <div className="mx-auto max-w-5xl">
            <SectionHeading kicker={copy.pricingKicker} title={copy.pricingTitle} subtitle={copy.pricingSubtitle} />
            <div className="mt-12 overflow-hidden rounded-3xl border border-violet-500/12 bg-white/70 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm dark:bg-white/[0.03]">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[34rem] border-collapse text-left">
                  <thead>
                    <tr className="border-b border-violet-500/12">
                      <th className="text-muted-foreground px-5 py-4 text-xs font-medium tracking-widest uppercase">{copy.pricingCols.product}</th>
                      <th className="px-5 py-4 text-xs font-medium tracking-widest text-violet-700 uppercase dark:text-violet-300">{copy.pricingCols.flatkey}</th>
                      <th className="text-muted-foreground px-5 py-4 text-xs font-medium tracking-widest uppercase">{copy.pricingCols.elsewhere}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {copy.pricingRows.map((row) => (
                      <tr key={row.label} className="border-b border-violet-500/10 last:border-0">
                        <td className="px-5 py-5 text-sm font-semibold">{row.label}</td>
                        <td className="px-5 py-5 font-mono text-xl font-bold text-violet-600 dark:text-violet-300">{row.flatkey}</td>
                        <td className="px-5 py-5">
                          <span className="text-muted-foreground/70 font-mono text-base font-semibold line-through">{row.competitor}</span>
                          <span className="text-muted-foreground ml-2 text-xs">{row.note}</span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
            <p className="text-muted-foreground mt-4 text-center text-xs">{copy.pricingFootnote}</p>
          </div>
        </section>

        {/* Who it's for. */}
        <section className="relative z-10 border-t border-violet-500/10 px-6 py-20 md:py-24">
          <div className="mx-auto max-w-6xl">
            <SectionHeading kicker={copy.whoForKicker} title={copy.whoForTitle} />
            <div className="mt-12 grid gap-5 md:grid-cols-3">
              {copy.whoFor.map((item, index) => {
                const Icon = whoForIcons[index] ?? Layers3;
                return (
                  <article
                    key={item.title}
                    className="flex flex-col rounded-3xl border border-violet-500/12 bg-white/70 p-7 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm transition-colors duration-300 hover:border-violet-500/24 hover:bg-white/80 dark:bg-white/[0.03] dark:hover:bg-white/[0.06]"
                  >
                    <div className="mb-6 flex size-14 items-center justify-center rounded-2xl border border-violet-500/20 bg-violet-500/8 text-violet-700 shadow-[0_18px_44px_-30px_rgba(124,58,237,0.8)] dark:text-violet-300">
                      <Icon className="size-6" strokeWidth={1.6} />
                    </div>
                    <h3 className="text-lg font-semibold tracking-tight">{item.title}</h3>
                    <p className="text-muted-foreground mt-3 text-sm leading-6">{item.body}</p>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        {/* Final CTA + FAQ. */}
        <section className="relative z-10 overflow-hidden border-t border-violet-500/10 px-6 py-20 md:py-28">
          <div className="absolute inset-0 -z-10 opacity-20" style={{ background: "radial-gradient(ellipse 55% 45% at 30% 50%, rgba(124,58,237,0.28) 0%, transparent 70%), radial-gradient(ellipse 42% 38% at 70% 40%, rgba(217,70,239,0.2) 0%, transparent 70%)" }} />
          <div className="mx-auto grid max-w-6xl gap-8 lg:grid-cols-[0.9fr_1.1fr]">
            <div className="flex flex-col rounded-3xl border border-violet-500/16 bg-white/72 p-8 shadow-[0_28px_80px_-56px_rgba(91,33,182,0.82)] backdrop-blur-sm dark:bg-white/[0.04]">
              <h2 className="text-2xl leading-tight font-bold tracking-tight md:text-3xl">
                {copy.finalCta.title}
              </h2>
              <p className="text-muted-foreground/80 mt-5 text-base leading-relaxed">{copy.finalCta.body}</p>
              <a
                className="flatkey-hero-cta group mt-8 inline-flex h-11 w-fit items-center px-5 text-sm font-medium shadow-[0_16px_34px_-18px_rgba(124,58,237,0.85)] transition-colors hover:opacity-90"
                href={ctaUrl}
                style={{ borderRadius: "0.5rem" }}
              >
                {copy.finalCta.button}
                <ArrowRight className="ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5" />
              </a>
            </div>
            <div className="space-y-4">
              {copy.faqs.map((faq) => (
                <article
                  key={faq.question}
                  className="rounded-3xl border border-violet-500/12 bg-white/70 p-6 shadow-[0_24px_70px_-52px_rgba(91,33,182,0.78)] backdrop-blur-sm dark:bg-white/[0.03]"
                >
                  <h3 className="text-base font-semibold tracking-tight">{faq.question}</h3>
                  <p className="text-muted-foreground mt-3 text-sm leading-6">{faq.answer}</p>
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
    <div className="mx-auto max-w-3xl text-center">
      <p className="text-muted-foreground mb-3 text-xs font-medium tracking-widest uppercase">{props.kicker}</p>
      <h2 className="text-2xl leading-tight font-bold tracking-tight md:text-3xl">{props.title}</h2>
      {props.subtitle ? <p className="text-muted-foreground/80 mx-auto mt-5 max-w-2xl text-base leading-relaxed">{props.subtitle}</p> : null}
    </div>
  );
}

function ComputeVisual({ copy }: { copy: ReturnType<typeof getComputeLandingPageCopy> }) {
  const tiles = [
    { emoji: "⚡", title: copy.visual.serverless.title, meta: copy.visual.serverless.meta },
    { emoji: "🖥️", title: copy.visual.gpu.title, meta: copy.visual.gpu.meta },
    { emoji: "🎬", title: copy.visual.video.title, meta: copy.visual.video.meta },
  ];

  return (
    <div className="w-full max-w-md rounded-3xl border border-violet-500/12 bg-white/70 p-5 shadow-[0_30px_84px_-54px_rgba(91,33,182,0.85)] backdrop-blur-sm dark:bg-white/[0.03]">
      <div className="flex items-center justify-between border-b border-violet-500/12 px-1 pb-3">
        <div className="flex items-center gap-2">
          <span className="size-2.5 rounded-full bg-red-400/80" />
          <span className="size-2.5 rounded-full bg-amber-300/80" />
          <span className="size-2.5 rounded-full bg-emerald-300/80" />
        </div>
        <span className="text-muted-foreground font-mono text-xs">flatkey compute</span>
      </div>
      <div className="space-y-3 pt-4">
        {tiles.map((tile) => (
          <div
            key={tile.title}
            className="flex items-center gap-4 rounded-2xl border border-violet-500/12 bg-white/60 px-4 py-4 dark:bg-white/[0.02]"
          >
            <span
              aria-hidden="true"
              className="flex size-11 shrink-0 items-center justify-center rounded-xl border border-violet-500/20 bg-violet-500/8 text-xl leading-none"
            >
              {tile.emoji}
            </span>
            <div className="min-w-0">
              <p className="text-sm font-semibold tracking-tight">{tile.title}</p>
              <p className="text-muted-foreground font-mono text-xs text-violet-600 dark:text-violet-300">{tile.meta}</p>
            </div>
          </div>
        ))}
        <div className="flex items-center justify-center gap-2 rounded-2xl border border-violet-500/20 bg-violet-500/8 px-4 py-3 font-mono text-xs font-medium tracking-widest text-violet-700 uppercase dark:text-violet-300">
          <KeyRound className="size-3.5" />
          {copy.visual.balanceLine}
        </div>
      </div>
    </div>
  );
}
