import Link from "next/link";
import { ArrowRight, Check, CircleDollarSign, Compass, KeyRound, Scale, Workflow } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import {
  getToolsAdMarketplaceUrl,
  getToolsAdSignupUrl,
  toolsAdLandingPath,
  type ToolsAdLandingConfig,
} from "@/lib/tools-ad-landing";

export function ToolsAdLandingPage({ config }: { config: ToolsAdLandingConfig }) {
  const marketplaceUrl = getToolsAdMarketplaceUrl(config.slug);
  const signupUrl = getToolsAdSignupUrl(config.slug);
  const gridClass =
    "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
  const cardClass =
    "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
  const mutedClass = "text-[#5C5861] dark:text-white/62";

  return (
    <SiteShell locale="en" pathname={toolsAdLandingPath(config.slug)} hideLanguageSwitcher>
      <main className="fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={gridClass} />
        <section className="relative z-10 border-b-2 border-[#101014] px-4 pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-16 sm:px-6 md:pb-20 dark:border-white/20">
          <div className="relative mx-auto grid max-w-[2160px] items-center gap-12 lg:grid-cols-[0.92fr_1.08fr] lg:gap-16">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                <span className="size-2 rounded-full bg-[#5852FF]" />{config.badge}
              </div>
              <h1 className="mt-7 max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
                {config.h1}<br /><span className="text-[#1e67ff] dark:text-[#8fb4ff]">{config.h1Accent}</span>
              </h1>
              <p className={`mt-7 max-w-2xl text-base leading-7 md:text-lg ${mutedClass}`}>{config.description}</p>
              <div className="mt-8 flex flex-col gap-3 sm:flex-row">
                <a href={marketplaceUrl} className="flatkey-hero-cta group inline-flex h-13 items-center justify-center px-6 text-sm">
                  {config.primaryCta}<ArrowRight className="ml-2 size-4 transition-transform group-hover:translate-x-0.5" />
                </a>
                <a href={signupUrl} className="flatkey-cta-secondary inline-flex h-13 items-center justify-center px-6 text-sm">{config.secondaryCta}</a>
              </div>
              <div className="mt-6 flex flex-wrap gap-2">
                {config.chips.map((chip) => <span key={chip} className="rounded-full border border-[#101014]/14 bg-white/72 px-3 py-1.5 text-xs font-bold text-[#5C5861] dark:border-white/12 dark:bg-white/8 dark:text-white/62">{chip}</span>)}
              </div>
            </div>

            <div className="rounded-[1.25rem] border-2 border-[#101014] bg-[#101014] p-2 text-white shadow-[5px_5px_0_#7C3AED] dark:border-white/20 dark:shadow-[5px_5px_0_rgba(124,58,237,0.72)]">
              <div className="overflow-hidden rounded-[1rem] border border-white/10 bg-[#0b0d0a] text-white">
                <div className="flex items-center justify-between border-b border-white/10 px-4 py-3 font-mono text-[10px] font-black uppercase text-white/45">
                  <span>flatkey / bounded run</span><span className="text-[#F9F871]">ready</span>
                </div>
                <div className="p-5 sm:p-7">
                  <p className="font-mono text-[10px] font-black uppercase text-[#F9F871]">{config.promptLabel}</p>
                  <p className="mt-3 border-l-2 border-[#1e67ff] pl-4 font-mono text-sm leading-7 text-white/82">{config.prompt}</p>
                  <div className="mt-7 overflow-hidden rounded-[1rem] border border-white/10 bg-white/[0.035]">
                    <div className="border-b border-white/10 px-4 py-3 font-mono text-[10px] font-black uppercase text-white/42">{config.receiptTitle}</div>
                    {config.receiptRows.map((row) => (
                      <div key={row.label} className="grid gap-1 border-b border-white/8 px-4 py-3 last:border-0 sm:grid-cols-[90px_1fr]">
                        <span className="font-mono text-[11px] text-[#F9F871]">✓ {row.label}</span><span className="text-xs leading-5 text-white/60">{row.value}</span>
                      </div>
                    ))}
                  </div>
                  <div className="mt-5 flex items-center justify-between font-mono text-[10px] text-white/38">
                    <span>exact price shown before execution</span><span>one balance</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="relative z-10 border-y-2 border-[#101014] bg-[#101014] px-4 py-16 text-white sm:px-6 md:py-18 dark:border-white/20">
          <div className="mx-auto max-w-[2160px]">
            <div className="flex max-w-3xl items-start gap-4">
              <Compass className="mt-1 size-6 shrink-0 text-[#F9F871]" />
              <div><p className="font-mono text-xs font-black uppercase text-[#F9F871]">Why Flatkey</p><h2 className="mt-4 text-3xl leading-tight font-black tracking-normal md:text-5xl">One commercial surface for the whole job.</h2></div>
            </div>
            <div className="mt-12 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {config.benefits.map((benefit, index) => (
                <article key={benefit.title} className="rounded-[1rem] border-2 border-white/18 bg-white/8 p-6 md:p-7">
                  <span className="font-mono text-xs text-white/28">0{index + 1}</span>
                  <h2 className="mt-10 text-xl font-black tracking-normal">{benefit.title}</h2>
                  <p className="mt-4 text-sm leading-7 text-white/48">{benefit.body}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto max-w-[2160px]">
            <div className="grid gap-10 lg:grid-cols-[0.8fr_1.2fr]">
              <div>
                <Workflow className="size-7 text-[#1e67ff] dark:text-[#8fb4ff]" />
                <h2 className="mt-5 text-3xl leading-tight font-black tracking-normal md:text-5xl">{config.workflowTitle}</h2>
                <p className={`mt-5 max-w-xl text-sm leading-7 md:text-base ${mutedClass}`}>{config.workflowBody}</p>
              </div>
              <div className={`${cardClass} overflow-hidden`}>
                {config.workflowSteps.map((step) => (
                  <article key={step.number} className="grid gap-4 border-b border-[#101014]/14 px-5 py-7 last:border-b-0 sm:grid-cols-[70px_190px_1fr] dark:border-white/14">
                    <span className="font-mono text-xs text-[#1e67ff] dark:text-[#8fb4ff]">{step.number}</span>
                    <h3 className="font-black">{step.title}</h3>
                    <p className={`text-sm leading-6 ${mutedClass}`}>{step.body}</p>
                  </article>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className="relative z-10 border-y-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-[2160px]">
            <div className="max-w-3xl">
              <Scale className="size-7 text-[#1e67ff] dark:text-[#8fb4ff]" />
              <p className="mt-5 font-mono text-xs font-black uppercase text-[#1e67ff] dark:text-[#8fb4ff]">{config.comparison.eyebrow}</p>
              <h2 className="mt-4 text-3xl leading-tight font-black tracking-normal md:text-5xl">{config.comparison.title}</h2>
              <p className={`mt-5 text-sm leading-7 md:text-base ${mutedClass}`}>{config.comparison.body}</p>
            </div>
            <div className={`${cardClass} mt-12 overflow-x-auto`}>
              <table className="w-full min-w-[720px] border-separate border-spacing-0 text-left text-sm">
                <thead>
                  <tr>
                    {config.comparison.headers.map((header, index) => (
                      <th
                        key={header}
                        className={`border-b-2 border-[#101014]/12 px-5 py-4 font-black dark:border-white/12 ${index === 1 ? "bg-[#EEE4FF]/60 text-[#1e67ff] dark:bg-white/8 dark:text-[#8fb4ff]" : "text-[#5C5861] dark:text-white/58"}`}
                      >
                        {header}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {config.comparison.rows.map((row) => (
                    <tr key={row[0]}>
                      {row.map((cell, index) => (
                        <td
                          key={cell}
                          className={`border-b border-[#101014]/8 px-5 py-5 align-top leading-6 dark:border-white/8 ${index === 0 ? "font-semibold" : mutedClass} ${index === 1 ? "bg-[#EEE4FF]/50 font-medium dark:bg-white/8" : ""}`}
                        >
                          {cell}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className={`mt-6 flex flex-wrap items-center gap-x-4 gap-y-2 text-xs leading-6 ${mutedClass}`}>
              <span className="max-w-2xl">{config.comparison.note}</span>
              {config.comparison.sources.map((source) => (
                <a key={source.href} href={source.href} target="_blank" rel="noreferrer" className="font-semibold text-[#1e67ff] hover:underline dark:text-[#8fb4ff]">{source.label}</a>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto grid max-w-[2160px] gap-10 lg:grid-cols-[0.86fr_1.14fr] lg:items-start">
            <div><CircleDollarSign className="size-7 text-[#1e67ff] dark:text-[#8fb4ff]" /><h2 className="mt-5 text-3xl leading-tight font-black tracking-normal md:text-5xl">{config.useCasesTitle}</h2></div>
            <div className="grid gap-3 sm:grid-cols-2">
              {config.useCases.map((item) => <div key={item} className={`${cardClass} flex gap-3 p-4 text-sm font-semibold`}><Check className="mt-0.5 size-4 shrink-0 text-[#1e67ff] dark:text-[#8fb4ff]" />{item}</div>)}
            </div>
          </div>
        </section>

        <section className="relative z-10 border-y-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-5xl">
            <p className="font-mono text-xs font-black uppercase text-[#5C5861] dark:text-white/56">Questions before the first run</p>
            <div className={`${cardClass} mt-7 divide-y-2 divide-[#101014]/10 overflow-hidden dark:divide-white/10`}>
              {config.faqs.map((faq) => (
                <details key={faq.question} className="group p-6">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-6 text-lg font-black marker:content-none">{faq.question}<span className="font-mono text-2xl font-normal transition-transform group-open:rotate-45">+</span></summary>
                  <p className={`max-w-3xl pt-4 pr-12 text-sm leading-7 ${mutedClass}`}>{faq.answer}</p>
                </details>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 border-t-2 border-[#101014] px-4 py-20 text-center sm:px-6 md:py-24 dark:border-white/20">
          <div className="mx-auto max-w-3xl">
            <KeyRound className="mx-auto size-8 text-[#1e67ff] dark:text-[#8fb4ff]" />
            <h2 className="mt-6 text-3xl leading-tight font-black tracking-normal md:text-5xl">{config.finalTitle}</h2>
            <p className={`mx-auto mt-5 max-w-2xl text-sm leading-7 md:text-base ${mutedClass}`}>{config.finalBody}</p>
            <a href={marketplaceUrl} className="flatkey-hero-cta group mt-8 inline-flex h-13 items-center px-7 text-sm">{config.primaryCta}<ArrowRight className="ml-2 size-4 transition-transform group-hover:translate-x-0.5" /></a>
            <Link href="/tools" className="mt-5 block text-xs font-semibold text-[#1e67ff] hover:underline dark:text-[#8fb4ff]">Explore all Flatkey Tools</Link>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}
