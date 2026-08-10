import {
  ArrowRight,
  Bot,
  Braces,
  Check,
  CircleDollarSign,
  Clapperboard,
  CloudSun,
  Globe2,
  LayoutDashboard,
  MessagesSquare,
  MousePointer2,
  SearchCheck,
  ShoppingBag,
  Sparkles,
  UsersRound,
} from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { ToolsCommandBox } from "@/components/tools-command-box";
import type { Locale } from "@/lib/locales";
import {
  getToolsMarketplaceUrl,
  getToolsSignupUrl,
  TOOLS_LANDING_PATH,
  TOOLS_SETUP_COMMAND,
  toolsLandingCopy,
} from "@/lib/tools-landing";

type Props = { locale: Locale };

const categoryIcons = [Globe2, UsersRound, MessagesSquare, ShoppingBag, MousePointer2, Clapperboard, SearchCheck, CloudSun] as const;
const methodIcons = [Bot, Braces, LayoutDashboard] as const;
const toolsGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const toolsCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const toolsMutedClass = "text-[#5C5861] dark:text-white/62";
const toolsEyebrowClass = "font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]";

export function ToolsLandingPage(props: Props) {
  const copy = toolsLandingCopy[props.locale];
  const marketplaceUrl = getToolsMarketplaceUrl(props.locale);
  const signupUrl = getToolsSignupUrl(props.locale);

  return (
    <SiteShell locale={props.locale} pathname={TOOLS_LANDING_PATH}>
      <main className="fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={toolsGridClass} />

        <section className="relative z-10 border-b-2 border-[#101014] px-4 pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-16 sm:px-6 md:pb-20 dark:border-white/20">
          <div className="relative mx-auto max-w-[2160px]">
            <div className="mx-auto flex max-w-4xl flex-col items-center text-center">
              <div className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                <span className="relative flex size-1.5"><span className="absolute inline-flex size-full animate-ping rounded-full bg-[#5852FF] opacity-70" /><span className="relative inline-flex size-1.5 rounded-full bg-[#5852FF]" /></span>
                {copy.badge}
              </div>
              <h1 className="mt-8 max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
                {copy.heroLead} <span className="text-[#5852FF] dark:text-[#C8A8FF]">{copy.heroAccent}</span> {copy.heroTail}
              </h1>
              <p className={`mt-7 max-w-2xl text-base leading-7 md:text-lg ${toolsMutedClass}`}>{copy.heroBody}</p>
              <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
                <a href={marketplaceUrl} className="flatkey-hero-cta group inline-flex h-11 items-center px-5 text-sm">
                  {copy.primaryCta}<ArrowRight className="ml-1.5 size-4 transition-transform group-hover:translate-x-0.5" />
                </a>
                <a href={signupUrl} className="flatkey-cta-secondary inline-flex h-11 items-center px-5 text-sm">
                  {copy.secondaryCta}
                </a>
              </div>
            </div>

            <div className="mx-auto mt-14 grid max-w-5xl gap-4 lg:grid-cols-[1.05fr_0.95fr]">
              <div className={`${toolsCardClass} p-5 sm:p-7`}>
                <p className="mb-3 font-mono text-xs font-black uppercase text-[#5C5861] dark:text-white/56">{copy.commandLabel}</p>
                <ToolsCommandBox command={TOOLS_SETUP_COMMAND} copyLabel={copy.copyLabel} copiedLabel={copy.copiedLabel} />
                <p className={`mt-4 text-xs leading-5 ${toolsMutedClass}`}>{copy.heroNote}</p>
              </div>
              <AgentRunDemo prompt={copy.demoPrompt} lines={copy.demoLines} bill={copy.demoBill} />
            </div>
          </div>
        </section>

        <section className="relative z-10 border-y-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-[2160px]">
            <SectionHeading eyebrow={copy.catalogEyebrow} title={copy.catalogTitle} body={copy.catalogBody} />
            <div className="mt-12 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {copy.categories.map((item, index) => {
                const Icon = categoryIcons[index];
                return (
                  <article key={item.title} className={`${toolsCardClass} group p-5 transition-transform hover:-translate-y-0.5`}>
                    <div className="flex size-10 items-center justify-center rounded-full border-2 border-[#101014] bg-[#EEE4FF] text-[#7C3AED] shadow-[3px_3px_0_#101014] transition-transform group-hover:scale-105 dark:border-white/20 dark:bg-white/10 dark:text-[#C8A8FF] dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]"><Icon className="size-5" strokeWidth={1.7} /></div>
                    <h3 className="mt-5 text-base font-black tracking-normal">{item.title}</h3>
                    <p className={`mt-2 text-sm leading-6 ${toolsMutedClass}`}>{item.body}</p>
                  </article>
                );
              })}
            </div>
            <div className="mt-8 flex justify-center">
              <a href={marketplaceUrl} className="flatkey-cta-secondary inline-flex h-11 items-center px-5 text-sm">{copy.primaryCta}<ArrowRight className="size-4" /></a>
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto max-w-[2160px]">
            <SectionHeading eyebrow={copy.flowEyebrow} title={copy.flowTitle} body={copy.flowBody} centered />
            <div className="mt-12 grid gap-4 md:grid-cols-3">
              {copy.steps.map((step, index) => (
                <article key={step.label} className={`${toolsCardClass} relative overflow-hidden p-6`}>
                  <span className="font-mono text-[10px] font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">{step.label}</span>
                  <div className="mt-7 flex items-center gap-2 rounded-[1rem] border-2 border-[#101014]/14 bg-white/68 p-3 font-mono text-xs dark:border-white/12 dark:bg-white/[0.06]">
                    <span className="flex size-7 items-center justify-center rounded-full border-2 border-[#101014] bg-[#5852FF] text-[11px] font-black text-white shadow-[2px_2px_0_#101014] dark:border-white/24 dark:bg-white dark:text-[#101014] dark:shadow-[2px_2px_0_rgba(255,255,255,0.16)]">{index + 1}</span>
                    <span className={toolsMutedClass}>{step.meta}</span>
                  </div>
                  <h3 className="mt-6 text-xl font-black tracking-normal">{step.title}</h3>
                  <p className={`mt-3 text-sm leading-6 ${toolsMutedClass}`}>{step.body}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 overflow-hidden border-y-2 border-[#101014] bg-[#101014] px-4 py-16 text-white shadow-[0_-2px_0_#101014] sm:px-6 md:py-18 dark:border-white/20">
          <div className="relative mx-auto max-w-[2160px]">
            <div className="grid gap-10 lg:grid-cols-[1fr_1.15fr] lg:items-end">
              <div>
                <p className="font-mono text-xs font-black uppercase text-[#C8A8FF]">{copy.balanceEyebrow}</p>
                <h2 className="mt-4 max-w-xl text-3xl leading-tight font-black tracking-normal md:text-5xl">{copy.balanceTitle}</h2>
                <p className="mt-5 max-w-xl text-sm leading-7 text-white/64 md:text-base">{copy.balanceBody}</p>
              </div>
              <div className="grid grid-cols-2 gap-3">
                {copy.stats.map((stat) => (
                  <div key={stat.label} className="rounded-[1rem] border-2 border-white/18 bg-white/8 p-5 sm:p-7">
                    <strong className="block text-2xl font-black tracking-normal text-[#F9F871] sm:text-3xl">{stat.value}</strong>
                    <span className="mt-2 block text-xs font-semibold text-white/60">{stat.label}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto max-w-[2160px]">
            <SectionHeading eyebrow={copy.connectEyebrow} title={copy.connectTitle} body={copy.connectBody} centered />
            <div className="mx-auto mt-12 grid max-w-5xl gap-4 md:grid-cols-3">
              {copy.methods.map((method, index) => {
                const Icon = methodIcons[index];
                return (
                  <article key={method.title} className={`${toolsCardClass} p-6`}>
                    <Icon className="size-6 text-[#7C3AED] dark:text-[#C8A8FF]" strokeWidth={1.7} />
                    <h3 className="mt-8 text-xl font-black">{method.title}</h3>
                    <p className={`mt-3 text-sm leading-6 ${toolsMutedClass}`}>{method.body}</p>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        <section className="relative z-10 border-y-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-4xl">
            <SectionHeading eyebrow={copy.faqEyebrow} title={copy.faqTitle} />
            <div className={`${toolsCardClass} mt-10 divide-y-2 divide-[#101014]/10 overflow-hidden dark:divide-white/10`}>
              {copy.faqs.map((item) => (
                <details key={item.question} className="group p-5">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-5 text-base font-black marker:content-none">
                    {item.question}<span className="flex size-7 shrink-0 items-center justify-center rounded-full border-2 border-[#101014] bg-white text-lg font-black transition-transform group-open:rotate-45 dark:border-white/20 dark:bg-white/10">+</span>
                  </summary>
                  <p className={`max-w-3xl pt-3 pr-10 text-sm leading-7 ${toolsMutedClass}`}>{item.answer}</p>
                </details>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-20 text-center sm:px-6 md:py-24">
          <div className="mx-auto max-w-3xl">
            <Sparkles className="mx-auto size-8 text-[#7C3AED] dark:text-[#C8A8FF]" strokeWidth={1.6} />
            <h2 className="mt-6 text-3xl leading-tight font-black tracking-normal md:text-5xl">{copy.finalTitle}</h2>
            <p className={`mx-auto mt-5 max-w-xl text-sm leading-7 md:text-base ${toolsMutedClass}`}>{copy.finalBody}</p>
            <a href={marketplaceUrl} className="flatkey-hero-cta group mt-8 inline-flex h-12 items-center px-6 text-sm">
              {copy.finalCta}<ArrowRight className="ml-1.5 size-4 transition-transform group-hover:translate-x-0.5" />
            </a>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

function AgentRunDemo(props: { prompt: string; lines: string[]; bill: string }) {
  return (
    <div className="overflow-hidden rounded-[1.25rem] border-2 border-[#101014] bg-[#101014] text-white shadow-[5px_5px_0_#7C3AED] dark:border-white/20 dark:shadow-[5px_5px_0_rgba(124,58,237,0.72)]">
      <div className="flex h-11 items-center gap-1.5 border-b border-white/8 px-4"><i className="size-2.5 rounded-full bg-[#ff675f]" /><i className="size-2.5 rounded-full bg-[#ffbd2e]" /><i className="size-2.5 rounded-full bg-[#29c940]" /><span className="ml-2 font-mono text-[10px] text-white/36">Flatkey Tools · agent run</span></div>
      <div className="p-5 font-mono text-[11px] leading-6 sm:p-6">
        <p className="text-white/70"><span className="text-[#C8A8FF]">❯</span> {props.prompt}</p>
        <div className="mt-5 space-y-1.5">
          {props.lines.map((line) => <p key={line} className="flex gap-2 text-white/48"><Check className="mt-1 size-3.5 shrink-0 text-emerald-400" />{line}</p>)}
        </div>
        <div className="mt-5 border-t border-white/8 pt-4 text-[#C8A8FF]"><CircleDollarSign className="mr-2 inline size-4" />{props.bill}</div>
      </div>
    </div>
  );
}

function SectionHeading(props: { eyebrow: string; title: string; body?: string; centered?: boolean }) {
  return (
    <div className={props.centered ? "mx-auto max-w-3xl text-center" : "max-w-3xl"}>
      <p className={toolsEyebrowClass}>{props.eyebrow}</p>
      <h2 className="mt-4 text-3xl leading-tight font-black tracking-normal md:text-5xl">{props.title}</h2>
      {props.body ? <p className={`mt-5 text-sm leading-7 md:text-base ${toolsMutedClass}`}>{props.body}</p> : null}
    </div>
  );
}
