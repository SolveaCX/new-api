import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { localizePath } from "@/lib/locales";
import { ROUTER_ORIGIN } from "@/lib/origins";
import { cn } from "@/lib/utils";
import {
  SKAG_TRUST_LINE,
  getSkagLandingCtaUrl,
  skagLandingPath,
  type SkagLandingConfig,
} from "@/lib/skag-landing";

type Props = {
  config: SkagLandingConfig;
};

const skagGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const skagCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const skagMutedClass = "text-[#5C5861] dark:text-white/62";

// Google Ads SKAG landing page: the H1 echoes the ad keyword exactly and the
// first screen carries the value prop, price table, runnable snippet, CTA,
// and trust line. Styling mirrors the glm-landing/model-landing pages so the
// route looks native to the rest of the site.
export function SkagLandingPage({ config }: Props) {
  const ctaUrl = getSkagLandingCtaUrl();
  const apiBaseUrl = `${ROUTER_ORIGIN}/v1`;
  const locale = config.locale ?? "en";
  const pathname = config.pathname ?? skagLandingPath(config.slug);
  const trustLine = config.trustLine ?? SKAG_TRUST_LINE;
  const compactHero = config.compactHero ?? false;

  return (
    <SiteShell locale={locale} pathname={pathname} expandNavigationAtTablet>
      <main className="fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden="true" className={skagGridClass} />
        <section className={cn("relative z-10 border-b-2 border-[#101014] px-4 sm:px-6 dark:border-white/20", compactHero ? "pt-[calc(var(--fk-header-safe-area)+1rem)] pb-10 md:pt-[calc(var(--fk-header-safe-area)+1.5rem)] md:pb-14" : "pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-16 md:pb-20")}>
          <div className={cn("relative mx-auto grid max-w-[2160px] items-center", compactHero ? "gap-8 xl:grid-cols-[1fr_1fr]" : "gap-12 lg:grid-cols-[1fr_1fr]")}>
            <div className="mx-auto max-w-3xl text-center lg:mx-0 lg:text-left">
              <div className="inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-[#F9F871] px-4 py-2 font-mono text-xs font-black uppercase text-[#101014] shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                <span className="size-2 rounded-full bg-emerald-500 shadow-[0_0_16px_rgba(16,185,129,0.75)] dark:bg-emerald-300" />
                {config.badge}
              </div>

              <h1 className={cn("leading-[0.94] font-black tracking-normal text-balance", compactHero ? "mt-5 text-[clamp(2.35rem,5.4vw,4.25rem)]" : "mt-7 text-[clamp(2.7rem,7vw,6.4rem)]")}>
                {config.h1Lead}{" "}
                <span className="text-[#5852FF] dark:text-[#C8A8FF]">
                  {config.h1Accent}
                </span>
              </h1>
              <p className={cn(`mx-auto max-w-2xl text-lg leading-8 font-semibold lg:mx-0 ${skagMutedClass}`, compactHero ? "mt-4" : "mt-6")}>
                {config.description}
              </p>

              <div className={cn("flex flex-col items-center justify-center gap-4 sm:flex-row lg:justify-start", compactHero ? "mt-6" : "mt-8")}>
                <a
                  href={ctaUrl}
                  className="flatkey-primary-cta inline-flex min-h-14 w-full items-center justify-center gap-2 px-7 text-base sm:w-auto"
                >
                  {config.ctaLabel}
                  <ArrowRight className="size-4" />
                </a>
                {!config.hideSecondaryCta && (
                  <Link
                    href={localizePath("/pricing", locale)}
                    className="flatkey-cta-secondary inline-flex min-h-14 w-full items-center justify-center px-7 text-base sm:w-auto"
                  >
                    {config.secondaryCtaLabel ?? "See live pricing"}
                  </Link>
                )}
              </div>
              <p className={cn(`text-sm font-semibold ${skagMutedClass}`, compactHero ? "mt-4" : "mt-5")}>{trustLine}</p>

              <div className={cn(`${skagCardClass} p-5 text-left`, compactHero ? "mt-6" : "mt-8")}>
                <p className={`font-mono text-xs font-black uppercase ${skagMutedClass}`}>{config.pricingTitle}</p>
                <table className="mt-3 w-full text-sm">
                  <tbody>
                    {config.priceRows.map((row) => (
                      <tr key={row.label} className="border-b border-[#101014]/10 last:border-0 dark:border-white/10">
                        <td className={`py-2.5 pr-2 ${skagMutedClass}`}>{row.label}</td>
                        <td className="py-2.5 pr-2 text-right font-mono font-bold text-emerald-600 dark:text-emerald-300">{row.flatkey}</td>
                        <td className="py-2.5 text-right font-mono text-slate-400 line-through dark:text-slate-500">{row.official}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <p className={`mt-3 text-xs ${skagMutedClass}`}>{config.priceFootnote}</p>
              </div>
            </div>

            <CodeWindow config={config} apiBaseUrl={apiBaseUrl} />
          </div>
        </section>

        <section className="relative z-10 border-b-2 border-[#101014] bg-[#FFFDF6]/72 px-4 py-16 backdrop-blur-sm sm:px-6 md:py-18 dark:border-white/20 dark:bg-[#111116]/54">
          <div className="mx-auto max-w-[2160px]">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              {config.features.map((feature) => (
                <article
                  key={feature.title}
                  className={`${skagCardClass} p-6`}
                >
                  <h2 className="text-lg font-black tracking-normal text-[#101014] dark:text-white">{feature.title}</h2>
                  <p className={`mt-3 text-sm leading-7 ${skagMutedClass}`}>{feature.body}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-16 sm:px-6 md:py-18">
          <div className="mx-auto grid max-w-[2160px] gap-8 lg:grid-cols-[0.9fr_1.1fr]">
            <div className={`${skagCardClass} bg-[#F9F871]/75 p-8 dark:bg-white/8`}>
              <h2 className="text-3xl font-black tracking-normal text-[#101014] dark:text-white md:text-4xl">
                {config.h1Lead} {config.h1Accent}
              </h2>
              <p className={`mt-5 text-base leading-8 ${skagMutedClass}`}>{config.description}</p>
              <a
                href={ctaUrl}
                className="flatkey-primary-cta mt-8 inline-flex min-h-12 items-center justify-center gap-2 px-6 text-sm"
              >
                {config.ctaLabel}
                <ArrowRight className="size-4" />
              </a>
              <p className={`mt-4 text-xs font-semibold ${skagMutedClass}`}>{trustLine}</p>
            </div>
            <div className="space-y-4">
              {config.faq.map((faq) => (
                <article
                  key={faq.question}
                  className={`${skagCardClass} p-6`}
                >
                  <h3 className="text-base font-black text-[#101014] dark:text-white">{faq.question}</h3>
                  <p className={`mt-3 text-sm leading-7 ${skagMutedClass}`}>{faq.answer}</p>
                </article>
              ))}
            </div>
          </div>
        </section>
      </main>
    </SiteShell>
  );
}

function CodeWindow({ config, apiBaseUrl }: { config: SkagLandingConfig; apiBaseUrl: string }) {
  const snippets = [
    {
      label: "Python",
      code: `from openai import OpenAI

client = OpenAI(
    base_url="${apiBaseUrl}",
    api_key="YOUR_FLATKEY_KEY",
)

response = client.chat.completions.create(
    model="${config.exampleModel}",
    messages=[{"role": "user", "content": "Hello"}],
)
print(response.choices[0].message.content)`,
    },
    {
      label: "curl",
      code: `curl ${apiBaseUrl}/chat/completions \\
  -H "Authorization: Bearer $FLATKEY_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${config.exampleModel}",
    "messages": [{"role": "user", "content": "Hello"}]
  }'`,
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
        <span className="font-mono text-xs text-slate-500">{config.codeTitle}</span>
      </div>
      <div className="grid gap-4 p-4">
        {snippets.map((snippet) => (
          <div key={snippet.label} className="min-w-0 rounded-[1rem] border border-white/10 bg-[#060912]">
            <div className="border-b border-white/10 px-4 py-3 text-xs font-black text-[#C8A8FF]">{snippet.label}</div>
            <pre className="overflow-x-auto p-4 font-mono text-sm leading-7 text-slate-300">
              <code>{snippet.code}</code>
            </pre>
          </div>
        ))}
      </div>
    </div>
  );
}
