import Link from "next/link";
import {
  ArrowRight,
  Bot,
  Check,
  CircleDollarSign,
  DatabaseZap,
  KeyRound,
  Layers3,
  SearchCheck,
} from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { ToolsCommandBox } from "@/components/tools-command-box";
import {
  APIFY_ALTERNATIVE_PATH,
  APIFY_ALTERNATIVE_SETUP_COMMAND,
  apifyAlternativeCopy as copy,
  getApifyAlternativeMarketplaceUrl,
  getApifyAlternativeSignupUrl,
} from "@/lib/tools-conquest-landing";

const benefitIcons = [Layers3, CircleDollarSign, SearchCheck, DatabaseZap] as const;
const apifyGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const apifyCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const apifyMutedClass = "text-[#5C5861] dark:text-white/62";

export function ApifyAlternativePage() {
  const marketplaceUrl = getApifyAlternativeMarketplaceUrl();
  const signupUrl = getApifyAlternativeSignupUrl();

  return (
    <SiteShell locale="en" pathname={APIFY_ALTERNATIVE_PATH} hideLanguageSwitcher>
      <main className="fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={apifyGridClass} />
        <section className="relative z-10 border-b-2 border-[#101014] px-4 pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-16 sm:px-6 md:pb-20 dark:border-white/20">
          <div className="relative mx-auto grid max-w-[2160px] items-center gap-12 lg:grid-cols-[1.03fr_0.97fr]">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                <Bot className="size-3.5" />
                {copy.badge}
              </div>
              <h1 className="mt-7 text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
                {copy.h1Lead}{" "}
                <span className="text-[#5852FF] dark:text-[#C8A8FF]">
                  {copy.h1Accent}
                </span>
              </h1>
              <p className={`mt-7 max-w-2xl text-base leading-7 md:text-lg ${apifyMutedClass}`}>{copy.description}</p>
              <div className="mt-8 flex flex-col gap-3 sm:flex-row">
                <a href={marketplaceUrl} className="flatkey-hero-cta group inline-flex h-12 items-center justify-center px-6 text-sm">
                  {copy.primaryCta}<ArrowRight className="ml-1.5 size-4 transition-transform group-hover:translate-x-0.5" />
                </a>
                <a href={signupUrl} className="flatkey-cta-secondary inline-flex h-12 items-center justify-center px-6 text-sm">
                  {copy.secondaryCta}
                </a>
              </div>
              <p className={`mt-5 text-xs leading-5 ${apifyMutedClass}`}>{copy.trustLine}</p>
            </div>

            <div className="space-y-4">
              <div className={`${apifyCardClass} p-5 sm:p-7`}>
                <p className={`mb-3 font-mono text-xs font-black uppercase ${apifyMutedClass}`}>{copy.setupLabel}</p>
                <ToolsCommandBox command={APIFY_ALTERNATIVE_SETUP_COMMAND} copyLabel={copy.copyLabel} copiedLabel={copy.copiedLabel} />
              </div>
              <div className="overflow-hidden rounded-[1.25rem] border-2 border-[#101014] bg-[#101014] text-white shadow-[5px_5px_0_#7C3AED] dark:border-white/20 dark:shadow-[5px_5px_0_rgba(124,58,237,0.72)]">
                <div className="flex h-11 items-center gap-1.5 border-b border-white/8 px-4">
                  <i className="size-2.5 rounded-full bg-[#ff675f]" /><i className="size-2.5 rounded-full bg-[#ffbd2e]" /><i className="size-2.5 rounded-full bg-[#29c940]" />
                  <span className="ml-2 font-mono text-[10px] text-white/36">{copy.previewTitle}</span>
                </div>
                <div className="space-y-4 p-5 sm:p-6">
                  {copy.previewSteps.map((step, index) => (
                    <div key={step.label} className="flex gap-3">
                      <span className="flex size-7 shrink-0 items-center justify-center rounded-full border border-white/18 bg-white/10 font-mono text-[10px] text-[#C8A8FF]">{index + 1}</span>
                      <div><strong className="block text-sm font-semibold">{step.label}</strong><span className="mt-1 block text-xs leading-5 text-white/48">{step.value}</span></div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="relative z-10 border-b-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-[2160px]">
            <SectionHeading eyebrow={copy.whyEyebrow} title={copy.whyTitle} body={copy.whyBody} />
            <div className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              {copy.benefits.map((benefit, index) => {
                const Icon = benefitIcons[index];
                return (
                  <article key={benefit.title} className={`${apifyCardClass} p-6`}>
                    <Icon className="size-6 text-[#7C3AED] dark:text-[#C8A8FF]" strokeWidth={1.7} />
                    <h2 className="mt-7 text-lg font-black tracking-normal">{benefit.title}</h2>
                    <p className={`mt-3 text-sm leading-6 ${apifyMutedClass}`}>{benefit.body}</p>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto max-w-[2160px]">
            <SectionHeading eyebrow={copy.comparisonEyebrow} title={copy.comparisonTitle} body={copy.comparisonBody} />
            <div className={`${apifyCardClass} mt-10 overflow-x-auto`}>
              <table className="w-full min-w-[760px] border-separate border-spacing-0 text-left text-sm">
                <thead>
                  <tr>
                    {copy.comparisonHeaders.map((header, index) => (
                      <th key={header} className={`border-b-2 border-[#101014]/10 px-5 py-4 font-black dark:border-white/10 ${index === 1 ? "bg-[#EEE4FF]/60 text-[#101014] dark:bg-white/8 dark:text-white" : ""}`}>{header}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {copy.comparisonRows.map((row) => (
                    <tr key={row[0]}>
                      {row.map((cell, index) => (
                        <td key={cell} className={`border-b border-[#101014]/8 px-5 py-5 align-top leading-6 dark:border-white/8 ${index === 0 ? "font-semibold" : apifyMutedClass} ${index === 1 ? "bg-[#EEE4FF]/50 dark:bg-white/6" : ""}`}>{cell}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className={`mt-5 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs ${apifyMutedClass}`}>
              <span>{copy.comparisonNote}</span>
              {copy.sourceLinks.map((source) => <a key={source.href} href={source.href} target="_blank" rel="noreferrer" className="font-black text-[#7C3AED] hover:underline dark:text-[#C8A8FF]">{source.label}</a>)}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-y-2 border-[#101014] bg-[#101014] px-4 py-16 text-white sm:px-6 md:py-18 dark:border-white/20">
          <div className="relative mx-auto max-w-[2160px]">
            <SectionHeading eyebrow={copy.migrationEyebrow} title={copy.migrationTitle} />
            <div className="mt-12 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
              {copy.migrationSteps.map((step) => (
                <article key={step.number} className="border-t border-white/16 pt-5">
                  <span className="font-mono text-xs text-[#C8A8FF]">{step.number}</span>
                  <h2 className="mt-7 text-xl font-black">{step.title}</h2>
                  <p className="mt-3 text-sm leading-6 text-white/62">{step.body}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 border-b-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto grid max-w-[2160px] gap-12 lg:grid-cols-[0.9fr_1.1fr] lg:items-start">
            <SectionHeading eyebrow={copy.useCasesEyebrow} title={copy.useCasesTitle} />
            <div className="grid gap-3 sm:grid-cols-2">
              {copy.useCases.map((useCase) => (
                <div key={useCase} className={`${apifyCardClass} flex gap-3 p-4 text-sm leading-6`}>
                  <Check className="mt-1 size-4 shrink-0 text-[#7C3AED] dark:text-[#C8A8FF]" />{useCase}
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto max-w-5xl">
            <SectionHeading eyebrow={copy.faqEyebrow} title={copy.faqTitle} />
            <div className={`${apifyCardClass} mt-10 divide-y-2 divide-[#101014]/10 overflow-hidden dark:divide-white/10`}>
              {copy.faqs.map((item) => (
                <details key={item.question} className="group p-5">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-5 text-base font-black marker:content-none">
                    {item.question}<span className="flex size-7 shrink-0 items-center justify-center rounded-full border-2 border-[#101014] bg-white text-lg font-black transition-transform group-open:rotate-45 dark:border-white/20 dark:bg-white/10">+</span>
                  </summary>
                  <p className={`max-w-3xl pt-3 pr-10 text-sm leading-7 ${apifyMutedClass}`}>{item.answer}</p>
                </details>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 border-t-2 border-[#101014] px-4 py-20 text-center sm:px-6 md:py-24 dark:border-white/20">
          <div className="mx-auto max-w-3xl">
            <KeyRound className="mx-auto size-8 text-[#7C3AED] dark:text-[#C8A8FF]" strokeWidth={1.6} />
            <h2 className="mt-6 text-3xl leading-tight font-black tracking-normal md:text-5xl">{copy.finalTitle}</h2>
            <p className={`mx-auto mt-5 max-w-xl text-sm leading-7 md:text-base ${apifyMutedClass}`}>{copy.finalBody}</p>
            <a href={marketplaceUrl} className="flatkey-hero-cta group mt-8 inline-flex h-12 items-center px-6 text-sm">
              {copy.finalCta}<ArrowRight className="ml-1.5 size-4 transition-transform group-hover:translate-x-0.5" />
            </a>
            <p className={`mt-5 text-xs ${apifyMutedClass}`}>Flatkey is not affiliated with Apify.</p>
            <Link href="/tools" className="mt-4 inline-block text-xs font-black text-[#7C3AED] hover:underline dark:text-[#C8A8FF]">Explore all Flatkey Tools</Link>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

function SectionHeading(props: { eyebrow: string; title: string; body?: string }) {
  return (
    <div className="max-w-3xl">
      <p className="font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">{props.eyebrow}</p>
      <h2 className="mt-4 text-3xl leading-tight font-black tracking-normal md:text-5xl">{props.title}</h2>
      {props.body ? <p className={`mt-5 text-sm leading-7 md:text-base ${apifyMutedClass}`}>{props.body}</p> : null}
    </div>
  );
}
