import { ArrowRight, BadgeDollarSign, Check, FileVideo2, GitBranch, ImageIcon, KeyRound, Layers3, MonitorPlay, Play, Terminal, WandSparkles } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import type { ReactNode } from "react";
import { SiteShell } from "@/components/site-shell";
import {
  CLI_IMAGE_PATH,
  CLI_LANDING_PATH,
  CLI_VIDEO_PATH,
  HIGGSFIELD_ALTERNATIVE_PATH,
  cliLandingCopy,
  higgsfieldAlternativeCopy,
} from "@/lib/cli-landing";
import { type Locale, localizePath } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type CliLandingPageProps = {
  locale: Locale;
};

type HiggsfieldAlternativePageProps = {
  locale: Locale;
};

const CLI_NPM_URL = "https://www.npmjs.com/package/@flatkey-ai/cli";
const CLI_GITHUB_URL = "https://github.com/flatkey-ai/flatkey-cli";

const workflowIcons = [FileVideo2, WandSparkles, MonitorPlay, Terminal] as const;
const painIcons = [BadgeDollarSign, GitBranch, Layers3, Check] as const;
const mediaAssets: Array<{
  href: string;
  icon: typeof ImageIcon;
  kind: "image" | "video";
  type: "image" | "video";
  media: string;
  poster?: string;
  showPlay: boolean;
}> = [
  {
    href: CLI_IMAGE_PATH,
    icon: ImageIcon,
    kind: "image",
    type: "image",
    media: "/assets/cli/campaign-hero.png",
    showPlay: false,
  },
  {
    href: CLI_VIDEO_PATH,
    icon: Play,
    kind: "video",
    type: "video",
    media: "/assets/cli/product-reveal.png",
    poster: "/assets/cli/product-reveal.png",
    showPlay: true,
  },
];

const mediaCategoryLabels: Record<Locale, Record<"image" | "video", string>> = {
  en: { image: "Image", video: "Video" },
  zh: { image: "图像", video: "视频" },
  es: { image: "Imagen", video: "Video" },
  fr: { image: "Image", video: "Video" },
  pt: { image: "Imagem", video: "Video" },
  ru: { image: "Image", video: "Video" },
  ja: { image: "画像", video: "動画" },
  vi: { image: "Hình ảnh", video: "Video" },
  de: { image: "Bild", video: "Video" },
  id: { image: "Image", video: "Video" },
};

const cliGridBackdropClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const cliHardCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const cliMutedClass = "text-[#5C5861] dark:text-white/62";
const cliEyebrowClass = "mb-3 font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]";

export function CliLandingPage(props: CliLandingPageProps) {
  const copy = cliLandingCopy[props.locale];
  const keyUrl = consoleUrl("/keys", `lng=${props.locale}`);

  return (
    <SiteShell locale={props.locale} pathname={CLI_LANDING_PATH}>
      <main className="cli-landing fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={cliGridBackdropClass} />
        <Hero
          eyebrow={copy.hero.eyebrow}
          title={copy.hero.title}
          accent={copy.hero.accent}
          body={copy.hero.body}
          primaryCta={copy.hero.primaryCta}
          primaryHref={CLI_NPM_URL}
          secondaryCta={copy.hero.secondaryCta}
          secondaryHref={keyUrl}
          stats={copy.stats}
          code={copy.codeSamples[0]}
        />
        <MediaExamples copy={copy.sections.media} locale={props.locale} />

        <section className="relative z-10 border-y-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-[2160px]">
            <p className={cliEyebrowClass}>
              {copy.sections.workflow.eyebrow}
            </p>
            <div className="grid gap-8 lg:grid-cols-[0.9fr_1.1fr] lg:items-end">
              <div>
                <h2 className="max-w-xl text-3xl leading-tight font-black tracking-normal md:text-4xl">
                  {copy.sections.workflow.title}
                </h2>
                <p className={`mt-4 max-w-2xl text-base leading-7 ${cliMutedClass}`}>
                  {copy.sections.workflow.body}
                </p>
              </div>
              <div className="grid gap-3 sm:grid-cols-2">
                {copy.sections.workflow.cards.map((card, index) => {
                  const Icon = workflowIcons[index % workflowIcons.length];
                  return (
                    <article key={card.title} className={`${cliHardCardClass} p-5`}>
                      <Icon className="mb-4 size-5 text-[#7C3AED] dark:text-[#C8A8FF]" strokeWidth={1.8} />
                      <h3 className="font-black tracking-normal">{card.title}</h3>
                      <p className={`mt-2 text-sm leading-6 ${cliMutedClass}`}>{card.body}</p>
                    </article>
                  );
                })}
              </div>
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto grid max-w-[2160px] gap-6 lg:grid-cols-2">
            <InfoPanel title={copy.sections.spend.title} body={copy.sections.spend.body} bullets={copy.sections.spend.bullets} icon={<KeyRound className="size-5" />} />
            <InfoPanel title={copy.sections.agent.title} body={copy.sections.agent.body} bullets={copy.sections.agent.bullets} icon={<Terminal className="size-5" />} code={copy.codeSamples[1].code} />
          </div>
        </section>

        <UseCases title={copy.sections.useCases.title} items={copy.sections.useCases.items} />
        <Faq title={copy.sections.faq.title} items={copy.sections.faq.items} />
        <ClosingCta
          title={copy.sections.cta.title}
          body={copy.sections.cta.body}
          primaryCta={copy.sections.cta.primaryCta}
          primaryHref={CLI_NPM_URL}
          secondaryCta={copy.sections.cta.secondaryCta}
          secondaryHref={keyUrl}
        />
      </main>
    </SiteShell>
  );
}

export function HiggsfieldAlternativePage(props: HiggsfieldAlternativePageProps) {
  const copy = higgsfieldAlternativeCopy[props.locale];
  const keyUrl = consoleUrl("/keys", `lng=${props.locale}`);

  return (
    <SiteShell locale={props.locale} pathname={HIGGSFIELD_ALTERNATIVE_PATH}>
      <main className="min-h-screen bg-[#f8f7f2] text-[#151615] dark:bg-[#070807] dark:text-[#f6f4ee]">
        <section className="px-6 pt-28 pb-16 md:pt-34">
          <div className="mx-auto grid max-w-6xl gap-10 lg:grid-cols-[0.95fr_1.05fr] lg:items-center">
            <div>
              <p className="mb-4 inline-flex rounded-full border border-amber-700/20 bg-amber-100/70 px-3 py-1 text-xs font-semibold tracking-[0.16em] text-amber-800 uppercase dark:border-amber-300/20 dark:bg-amber-300/10 dark:text-amber-200">
                {copy.hero.eyebrow}
              </p>
              <h1 className="text-4xl leading-[1.05] font-semibold tracking-tight md:text-6xl">{copy.hero.title}</h1>
              <p className="mt-6 max-w-2xl text-base leading-7 text-[#5f665f] md:text-lg dark:text-[#b7bdb4]">{copy.hero.body}</p>
              <div className="mt-8 flex flex-wrap gap-3">
                <a className="flatkey-primary-cta inline-flex h-11 items-center px-5 text-sm" href={CLI_NPM_URL} target="_blank" rel="noopener noreferrer">
                  {copy.hero.primaryCta}
                  <ArrowRight className="ml-2 size-4" />
                </a>
                <a className="flatkey-cta-secondary inline-flex h-11 items-center px-5 text-sm" href="#comparison">
                  {copy.hero.secondaryCta}
                </a>
              </div>
            </div>
            <div className={`${cliHardCardClass} p-4`}>
              <div className="rounded-[1rem] border-2 border-[#101014] bg-[#101014] p-4 text-[#e7f1e8] dark:border-white/18">
                <div className="mb-3 flex items-center justify-between border-b border-white/10 pb-3">
                  <span className="text-xs text-white/55">flatkey production run</span>
                  <Image src="/flatkey-mark-dark.svg" alt="Flatkey" width={24} height={24} className="size-6" />
                </div>
                <pre className="overflow-x-auto text-[12px] leading-6 whitespace-pre-wrap text-emerald-100">
                  <code>{copy.migration.code}</code>
                </pre>
              </div>
            </div>
          </div>
        </section>

        <section className="border-y border-black/10 bg-white/70 px-6 py-14 dark:border-white/10 dark:bg-white/[0.03]">
          <div className="mx-auto max-w-6xl">
            <h2 className="text-2xl font-semibold tracking-tight md:text-3xl">{copy.position.title}</h2>
            <p className="mt-4 max-w-3xl text-base leading-7 text-[#5f665f] dark:text-[#b7bdb4]">{copy.position.body}</p>
          </div>
        </section>

        <section id="comparison" className="px-6 py-18">
          <div className="mx-auto max-w-6xl">
            <h2 className="text-3xl font-semibold tracking-tight">{copy.comparison.title}</h2>
            <div className="mt-8 overflow-hidden rounded-lg border border-black/10 bg-white dark:border-white/10 dark:bg-white/[0.04]">
              <div className="grid grid-cols-[0.85fr_1fr_1fr] border-b border-black/10 bg-[#ebe8de] text-sm font-semibold dark:border-white/10 dark:bg-white/[0.08]">
                {copy.comparison.headers.map((header) => (
                  <div key={header} className="p-4">{header}</div>
                ))}
              </div>
              {copy.comparison.rows.map((row) => (
                <div key={row[0]} className="grid grid-cols-[0.85fr_1fr_1fr] border-b border-black/8 text-sm last:border-b-0 dark:border-white/8">
                  <div className="p-4 font-semibold">{row[0]}</div>
                  <div className="p-4 leading-6 text-[#4f554f] dark:text-[#c5c9c2]">{row[1]}</div>
                  <div className="p-4 leading-6 text-[#4f554f] dark:text-[#c5c9c2]">{row[2]}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="border-y border-black/10 bg-[#ebe8de] px-6 py-18 dark:border-white/10 dark:bg-white/[0.03]">
          <div className="mx-auto max-w-6xl">
            <p className="mb-3 text-xs font-semibold tracking-[0.18em] text-amber-800 uppercase dark:text-amber-200">{copy.pains.eyebrow}</p>
            <h2 className="max-w-2xl text-3xl leading-tight font-semibold tracking-tight md:text-4xl">{copy.pains.title}</h2>
            <p className="mt-4 max-w-3xl text-base leading-7 text-[#5f665f] dark:text-[#b7bdb4]">{copy.pains.body}</p>
            <div className="mt-8 grid gap-3 md:grid-cols-2">
              {copy.pains.items.map((item, index) => {
                const Icon = painIcons[index % painIcons.length];
                return (
                  <article key={item.title} className="rounded-lg border border-black/10 bg-[#f8f7f2] p-5 dark:border-white/10 dark:bg-white/[0.05]">
                    <Icon className="mb-4 size-5 text-amber-800 dark:text-amber-200" strokeWidth={1.8} />
                    <h3 className="font-semibold tracking-tight">{item.title}</h3>
                    <p className="mt-2 text-sm leading-6 text-[#626861] dark:text-[#b7bdb4]">{item.body}</p>
                  </article>
                );
              })}
            </div>
          </div>
        </section>

        <section className="px-6 py-18">
          <div className="mx-auto grid max-w-6xl gap-8 lg:grid-cols-[0.85fr_1.15fr] lg:items-start">
            <div>
              <h2 className="text-3xl leading-tight font-semibold tracking-tight">{copy.migration.title}</h2>
              <p className="mt-4 text-base leading-7 text-[#5f665f] dark:text-[#b7bdb4]">{copy.migration.body}</p>
            </div>
            <CodeBlock label="flatkey-cli" code={copy.migration.code} />
          </div>
        </section>

        <ClosingCta
          title={copy.cta.title}
          body={copy.cta.body}
          primaryCta={copy.cta.primaryCta}
          primaryHref={CLI_NPM_URL}
          secondaryCta={copy.cta.secondaryCta}
          secondaryHref={keyUrl}
        />
      </main>
    </SiteShell>
  );
}

function MediaExamples(props: { copy: (typeof cliLandingCopy)[Locale]["sections"]["media"]; locale: Locale }) {
  const examples = [
    props.copy.items.find((item) => item.kind.toLowerCase().includes("image") || item.kind.includes("图片") || item.kind.includes("画像") || item.kind.includes("圖")),
    props.copy.items.find((item) => item.kind.toLowerCase().includes("video") || item.kind.includes("视频") || item.kind.includes("視頻") || item.kind.includes("動画") || item.kind.includes("Vídeo")),
  ].filter(Boolean) as typeof props.copy.items;

  return (
    <section className="relative z-10 border-y-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
      <div className="mx-auto max-w-[2160px]">
        <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div>
            <p className={cliEyebrowClass}>{props.copy.eyebrow}</p>
            <h2 className="max-w-2xl text-3xl leading-tight font-black tracking-normal md:text-4xl">{props.copy.title}</h2>
          </div>
          <p className={`max-w-md text-sm leading-6 ${cliMutedClass}`}>
            {props.copy.body}
          </p>
        </div>
        <div className="mt-8 grid gap-3 md:grid-cols-2">
          {mediaAssets.map((asset, index) => {
            const example = examples[index] ?? props.copy.items[index];
            const Icon = asset.icon;
            const label = mediaCategoryLabels[props.locale][asset.kind];
            return (
            <Link key={example.title} href={localizePath(asset.href, props.locale)} className={`${cliHardCardClass} group overflow-hidden transition-transform hover:-translate-y-0.5`}>
              <div className="relative aspect-[4/3] overflow-hidden bg-[#111412]">
                <Image
                  src={asset.type === "video" && asset.poster ? asset.poster : asset.media}
                  alt={example.title}
                  fill
                  sizes="(min-width: 768px) 50vw, 100vw"
                  className="object-cover"
                />
                <div className="absolute inset-0 bg-gradient-to-t from-black/28 via-transparent to-transparent" />
                <div className="absolute top-4 left-4 inline-flex items-center gap-1.5 rounded-full bg-black/70 px-2.5 py-1 text-[11px] font-semibold text-white">
                  <Icon className="size-3.5" />
                  {label}
                </div>
                {asset.showPlay ? (
                  <div className="absolute right-4 bottom-4 flex h-10 w-10 items-center justify-center rounded-full border-2 border-[#101014] bg-white text-[#101014] shadow-[3px_3px_0_#101014]">
                    <Play className="ml-0.5 size-5 fill-current" />
                  </div>
                ) : null}
                <div className="absolute right-4 left-4 bottom-4 hidden md:block">
                  <div className="h-2 rounded bg-black/15" style={{ width: `${72 - index * 5}%` }} />
                  <div className="mt-2 h-2 rounded bg-white/55" style={{ width: `${46 + index * 6}%` }} />
                </div>
              </div>
              <div className="p-5">
                <h3 className="flex items-center justify-between gap-3 font-black tracking-normal">
                  <span>{label}</span>
                  <ArrowRight className="size-4 shrink-0 text-[#7C3AED] transition-transform group-hover:translate-x-0.5 dark:text-[#C8A8FF]" />
                </h3>
                <p className={`mt-2 text-sm leading-6 ${cliMutedClass}`}>{example.body}</p>
                <p className="mt-4 rounded-[0.85rem] border-2 border-[#101014] bg-[#101014] p-3 font-mono text-[11px] leading-5 font-black text-white uppercase dark:border-white/18">{example.outcome}</p>
              </div>
            </Link>
            );
          })}
        </div>
      </div>
    </section>
  );
}

function Hero(props: {
  eyebrow: string;
  title: string;
  accent: string;
  body: string;
  primaryCta: string;
  primaryHref: string;
  secondaryCta: string;
  secondaryHref: string;
  stats: Array<{ value: string; label: string }>;
  code: { label: string; code: string };
}) {
  return (
    <section className="relative z-10 overflow-hidden px-4 pt-[var(--fk-header-safe-area)] pb-14 sm:px-6 md:pb-20">
      <div className="mx-auto grid max-w-[2160px] gap-10 border-b-2 border-[#101014] py-10 lg:grid-cols-[0.95fr_1.05fr] lg:items-center dark:border-white/20">
        <div>
          <p className="mb-5 inline-flex items-center gap-1.5 rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
            <span className="relative flex size-1.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[#7C3AED] opacity-75" />
              <span className="relative inline-flex size-1.5 rounded-full bg-[#7C3AED]" />
            </span>
            {props.eyebrow}
          </p>
          <h1 className="max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
            {props.title}
            <span className="block text-[#5852FF] dark:text-[#C8A8FF]">{props.accent}</span>
          </h1>
          <p className={`mt-6 max-w-2xl text-base leading-7 md:text-lg ${cliMutedClass}`}>{props.body}</p>
          <div className="mt-8 flex flex-wrap gap-3">
            <a className="flatkey-hero-cta group inline-flex h-11 items-center px-5 text-sm" href={props.primaryHref} target="_blank" rel="noopener noreferrer">
              {props.primaryCta}
              <ArrowRight className="ml-1.5 size-4 transition-transform duration-200 group-hover:translate-x-0.5" />
            </a>
            <a className="flatkey-cta-secondary inline-flex h-11 items-center px-5 text-sm" href={props.secondaryHref}>
              {props.secondaryCta}
            </a>
          </div>
          <div className="mt-10 grid max-w-xl grid-cols-3 gap-3">
            {props.stats.map((stat) => (
              <div key={stat.label} className="rounded-[1rem] border-2 border-[#101014] bg-[#FFFDF6]/88 p-3 shadow-[3px_3px_0_#101014] dark:border-white/18 dark:bg-white/8 dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                <div className="text-2xl font-black tracking-normal text-[#5852FF] dark:text-[#C8A8FF]">{stat.value}</div>
                <div className={`mt-1 text-xs leading-5 font-bold ${cliMutedClass}`}>{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
        <div className={`${cliHardCardClass} p-4`}>
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Image src="/flatkey-mark.svg" alt="Flatkey" width={28} height={28} className="size-7" />
              <span className="text-sm font-semibold">flatkey-cli</span>
            </div>
            <a className="rounded-full border border-[#101014]/14 bg-white px-3 py-1 text-xs font-black text-[#101014] hover:bg-[#F9F871] dark:border-white/14 dark:bg-white/8 dark:text-white" href={CLI_GITHUB_URL} target="_blank" rel="noopener noreferrer">
              GitHub
            </a>
          </div>
          <CodeBlock label={props.code.label} code={props.code.code} />
        </div>
      </div>
    </section>
  );
}

function CodeBlock(props: { label: string; code: string }) {
  return (
    <div className="overflow-hidden rounded-[1rem] border-2 border-[#101014] bg-[#101014] dark:border-white/18">
      <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
        <span className="text-xs text-white/55">{props.label}</span>
        <span className="font-mono text-xs text-[#C8A8FF]">$</span>
      </div>
      <pre className="overflow-x-auto p-4 text-[12px] leading-6 whitespace-pre-wrap text-[#EEE4FF]">
        <code>{props.code}</code>
      </pre>
    </div>
  );
}

function InfoPanel(props: { title: string; body: string; bullets: string[]; icon: ReactNode; code?: string }) {
  return (
    <article className={`${cliHardCardClass} p-6`}>
      <div className="mb-4 flex size-10 items-center justify-center rounded-full border-2 border-[#101014] bg-[#EEE4FF] text-[#7C3AED] shadow-[3px_3px_0_#101014] dark:border-white/20 dark:bg-white/10 dark:text-[#C8A8FF] dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
        {props.icon}
      </div>
      <h2 className="text-2xl font-black tracking-normal">{props.title}</h2>
      <p className={`mt-3 text-sm leading-6 ${cliMutedClass}`}>{props.body}</p>
      <ul className="mt-5 space-y-2">
        {props.bullets.map((bullet) => (
          <li key={bullet} className="flex gap-2 text-sm leading-6">
            <Check className="mt-1 size-4 shrink-0 text-emerald-600" />
            <span>{bullet}</span>
          </li>
        ))}
      </ul>
      {props.code ? <div className="mt-5"><CodeBlock label="automation" code={props.code} /></div> : null}
    </article>
  );
}

function UseCases(props: { title: string; items: Array<{ title: string; body: string }> }) {
  return (
    <section className="relative z-10 border-y-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
      <div className="mx-auto max-w-[2160px]">
        <h2 className="text-3xl font-black tracking-normal">{props.title}</h2>
        <div className="mt-8 grid gap-3 md:grid-cols-4">
          {props.items.map((item) => (
            <article key={item.title} className={`${cliHardCardClass} p-5`}>
              <h3 className="font-black tracking-normal">{item.title}</h3>
              <p className={`mt-2 text-sm leading-6 ${cliMutedClass}`}>{item.body}</p>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}

function Faq(props: { title: string; items: Array<{ question: string; answer: string }> }) {
  return (
    <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
      <div className="mx-auto max-w-5xl">
        <h2 className="text-3xl font-black tracking-normal">{props.title}</h2>
        <div className={`${cliHardCardClass} mt-8 divide-y-2 divide-[#101014]/10 overflow-hidden dark:divide-white/10`}>
          {props.items.map((item) => (
            <div key={item.question} className="p-5">
              <h3 className="font-black tracking-normal">{item.question}</h3>
              <p className={`mt-2 text-sm leading-6 ${cliMutedClass}`}>{item.answer}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function ClosingCta(props: {
  title: string;
  body: string;
  primaryCta: string;
  primaryHref: string;
  secondaryCta: string;
  secondaryHref: string;
}) {
  return (
    <section className="relative z-10 border-t-2 border-[#101014] px-4 py-16 sm:px-6 md:py-18 dark:border-white/20">
      <div className="mx-auto flex max-w-[2160px] flex-col gap-6 rounded-[1.25rem] border-2 border-[#101014] bg-[#101014] p-8 text-white shadow-[5px_5px_0_#7C3AED] md:flex-row md:items-center md:justify-between dark:border-white/24 dark:shadow-[5px_5px_0_rgba(124,58,237,0.72)]">
        <div>
          <h2 className="max-w-2xl text-3xl leading-tight font-black tracking-normal">{props.title}</h2>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-white/70">{props.body}</p>
        </div>
        <div className="flex shrink-0 flex-wrap gap-3">
          <a className="flatkey-cta-inverse inline-flex h-11 items-center px-5 text-sm" href={props.primaryHref} target="_blank" rel="noopener noreferrer">
            {props.primaryCta}
          </a>
          <a className="flatkey-cta-secondary inline-flex h-11 items-center px-5 text-sm" href={props.secondaryHref}>
            {props.secondaryCta}
          </a>
        </div>
      </div>
    </section>
  );
}
