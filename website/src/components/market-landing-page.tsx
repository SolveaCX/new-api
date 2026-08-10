import Link from "next/link";
import { ArrowRight, BadgeCheck, ShieldCheck, Wallet } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { localizePath } from "@/lib/locales";
import { getMarketConfig, getMarketLandingCtaUrl } from "@/lib/market-landing";

type Props = {
  slug: string;
};

const marketGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const marketCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const marketMutedClass = "text-[#5C5861] dark:text-white/62";

export function MarketLandingPage({ slug }: Props) {
  const cfg = getMarketConfig(slug);
  if (!cfg) return null;
  const { locale, copy } = cfg;
  const ctaUrl = getMarketLandingCtaUrl();

  return (
    <SiteShell locale={locale} pathname={slug} hideLanguageSwitcher>
      <main className="fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={marketGridClass} />
        {/* Hero */}
        <section className="relative z-10 border-b-2 border-[#101014] px-4 pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-16 sm:px-6 md:pb-20 dark:border-white/20">
          <div className="relative mx-auto max-w-5xl text-center">
            <div className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-[#F9F871] px-4 py-2 font-mono text-xs font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
              <Wallet className="size-3.5" />
              {copy.badge}
            </div>
            <p className="mt-8 font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">
              {copy.hero.eyebrow}
            </p>
            <h1 className="mt-4 text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
              {copy.hero.title}{" "}
              <span className="text-[#5852FF] dark:text-[#C8A8FF]">
                {copy.hero.highlight}
              </span>
            </h1>
            <p className={`mx-auto mt-7 max-w-2xl text-lg leading-8 font-semibold md:text-xl ${marketMutedClass}`}>
              {copy.hero.subtitle}
            </p>
            <div className="mt-9 flex flex-col items-center justify-center gap-4 sm:flex-row">
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
            <p className={`mt-5 text-sm font-semibold ${marketMutedClass}`}>{copy.hero.trustLine}</p>
          </div>
        </section>

        {/* Pain → solution table */}
        <section className="relative z-10 border-b-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-5xl">
            <h2 className="text-center text-[clamp(1.8rem,4vw,2.8rem)] font-black tracking-normal">{copy.painsTitle}</h2>
            <p className={`mx-auto mt-4 max-w-2xl text-center text-lg ${marketMutedClass}`}>
              {copy.painsSubtitle}
            </p>
            <div className={`${marketCardClass} mt-12 overflow-hidden`}>
              <div className="grid grid-cols-1 gap-px bg-[#101014]/12 md:grid-cols-2 dark:bg-white/10">
                <div className="bg-white px-6 py-4 text-sm font-black uppercase text-[#5C5861] dark:bg-[#111116] dark:text-white/62">
                  {copy.colYouSaid}
                </div>
                <div className="hidden bg-white px-6 py-4 text-sm font-black uppercase text-emerald-600 md:block dark:bg-[#111116] dark:text-emerald-400">
                  {copy.colWeSolve}
                </div>
                {copy.pains.map((pain, i) => (
                  <div key={i} className="contents">
                    <div className="bg-white px-6 py-6 text-lg font-semibold text-[#101014] italic dark:bg-[#111116] dark:text-white">
                      {pain.quote}
                    </div>
                    <div className="flex items-start gap-3 bg-[#F9F871]/35 px-6 py-6 text-base font-semibold text-[#101014] dark:bg-white/[0.06] dark:text-white/72">
                      <BadgeCheck className="mt-0.5 size-5 shrink-0 text-emerald-500" />
                      {pain.solution}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        {/* Trust anchor */}
        <section className="relative z-10 border-b-2 border-[#101014] px-4 py-16 sm:px-6 md:py-18 dark:border-white/20">
          <div className="mx-auto max-w-4xl text-center">
            <ShieldCheck className="mx-auto size-12 text-emerald-500" />
            <h2 className="mt-6 text-[clamp(1.7rem,4vw,2.6rem)] font-black tracking-normal">{copy.trust.title}</h2>
            <p className={`mx-auto mt-4 max-w-2xl text-lg ${marketMutedClass}`}>{copy.trust.subtitle}</p>
            <ul className="mx-auto mt-10 grid max-w-3xl gap-4 text-left sm:grid-cols-1">
              {copy.trust.points.map((point, i) => (
                <li
                  key={i}
                  className={`${marketCardClass} flex items-start gap-3 px-5 py-4 text-base font-semibold`}
                >
                  <BadgeCheck className="mt-0.5 size-5 shrink-0 text-emerald-500" />
                  {point}
                </li>
              ))}
            </ul>
          </div>
        </section>

        {/* Premium hook + models */}
        <section className="relative z-10 border-b-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-5xl">
            <div className={`${marketCardClass} bg-[#F9F871]/75 px-8 py-10 text-center dark:bg-white/8`}>
              <h2 className="text-[clamp(1.6rem,3.5vw,2.4rem)] font-black tracking-normal">{copy.premium.title}</h2>
              <p className={`mx-auto mt-4 max-w-2xl text-lg ${marketMutedClass}`}>{copy.premium.body}</p>
            </div>
            <div className="mt-12">
              <h3 className="text-center text-2xl font-black tracking-normal">{copy.models.title}</h3>
              <p className={`mt-2 text-center text-base ${marketMutedClass}`}>{copy.models.subtitle}</p>
              <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {copy.models.items.map((m, i) => (
                  <div
                    key={i}
                    className={`${marketCardClass} px-5 py-4`}
                  >
                    <div className="text-base font-black text-[#101014] dark:text-white">{m.name}</div>
                    <div className={`mt-1 text-sm ${marketMutedClass}`}>{m.note}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        {/* FAQ */}
        <section className="relative z-10 border-b-2 border-[#101014] px-4 py-16 sm:px-6 md:py-18 dark:border-white/20">
          <div className="mx-auto max-w-3xl">
            <h2 className="text-center text-[clamp(1.7rem,4vw,2.6rem)] font-black tracking-normal">{copy.faqTitle}</h2>
            <div className="mt-10 space-y-4">
              {copy.faqs.map((faq, i) => (
                <div
                  key={i}
                  className={`${marketCardClass} px-6 py-5`}
                >
                  <div className="text-lg font-black text-[#101014] dark:text-white">{faq.question}</div>
                  <div className={`mt-2 text-base ${marketMutedClass}`}>{faq.answer}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* Final CTA */}
        <section className="relative z-10 px-4 py-20 sm:px-6 md:py-24">
          <div className="mx-auto max-w-3xl text-center">
            <h2 className="text-[clamp(1.9rem,5vw,3.2rem)] font-black tracking-normal">{copy.finalCta.title}</h2>
            <p className={`mt-4 text-lg ${marketMutedClass}`}>{copy.finalCta.subtitle}</p>
            <a
              href={ctaUrl}
              className="flatkey-primary-cta mt-9 inline-flex min-h-14 items-center justify-center gap-2 px-8 text-base"
            >
              {copy.finalCta.button}
              <ArrowRight className="size-4" />
            </a>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}
