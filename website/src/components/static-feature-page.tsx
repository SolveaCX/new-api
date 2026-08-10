import Link from "next/link";
import { ArrowRight, CheckCircle2 } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { type Locale, localizePath } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";
import { getStaticFeaturePage, type StaticFeaturePageKey } from "@/lib/static-feature-pages";

type Props = {
  pageKey: StaticFeaturePageKey;
  locale: Locale;
};

const staticGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const staticCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const staticMutedClass = "text-[#5C5861] dark:text-white/62";

function primaryHref(pageKey: StaticFeaturePageKey) {
  if (pageKey === "compute" || pageKey === "status" || pageKey === "playground" || pageKey === "docs") return consoleUrl("/dashboard");
  return consoleUrl("/sign-up");
}

function secondaryHref(pageKey: StaticFeaturePageKey, locale: Locale) {
  if (pageKey === "compute" || pageKey === "status" || pageKey === "topup") return localizePath("/contact", locale);
  if (pageKey === "docs" || pageKey === "model") return localizePath("/models", locale);
  if (pageKey === "usecases") return localizePath("/tools", locale);
  return localizePath("/pricing", locale);
}

export function StaticFeaturePage(props: Props) {
  const content = getStaticFeaturePage(props.pageKey, props.locale);

  return (
    <SiteShell locale={props.locale} pathname={content.pathname}>
      <main className="fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] text-[#101014] antialiased dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={staticGridClass} />
        <section className="relative z-10 border-b-2 border-[#101014] px-4 pt-[calc(var(--fk-header-safe-area)+2.5rem)] pb-12 text-center sm:px-6 md:pb-16 dark:border-white/20">
          <div className="mx-auto max-w-[2160px]">
            <p className="font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">{content.eyebrow}</p>
            <h1 className="mx-auto mt-5 max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
              {content.title}
            </h1>
            <p className={`mx-auto mt-5 max-w-3xl text-lg leading-8 ${staticMutedClass}`}>{content.description}</p>
            <div className="mt-8 flex flex-col items-center justify-center gap-3 sm:flex-row">
              <a
                href={primaryHref(props.pageKey)}
                className="flatkey-hero-cta inline-flex h-12 min-w-40 items-center justify-center gap-2 px-5 text-sm"
                style={{ color: "#fff" }}
              >
                {content.primary}
                <ArrowRight className="size-4" />
              </a>
              <Link
                href={secondaryHref(props.pageKey, props.locale)}
                className="flatkey-cta-secondary inline-flex h-12 min-w-40 items-center justify-center px-5 text-sm"
              >
                {content.secondary}
              </Link>
            </div>
          </div>
        </section>

        <section className="relative z-10 px-4 py-10 sm:px-6">
          <div className="mx-auto grid max-w-[2160px] gap-4 md:grid-cols-3">
            {content.stats.map((stat) => (
              <div key={`${stat.value}-${stat.label}`} className={`${staticCardClass} p-5`}>
                <div className="text-3xl font-black text-[#5852FF] dark:text-[#C8A8FF]">{stat.value}</div>
                <div className={`mt-2 text-sm font-bold ${staticMutedClass}`}>{stat.label}</div>
              </div>
            ))}
          </div>
        </section>

        <section className="relative z-10 px-4 pb-16 sm:px-6 md:pb-24">
          <div className="mx-auto grid max-w-[2160px] gap-4 lg:grid-cols-3">
            {content.sections.map((section) => (
              <article key={section.title} className={`${staticCardClass} p-6`}>
                <div className="mb-5 inline-flex size-10 items-center justify-center rounded-full border-2 border-[#101014] bg-[#EEE4FF] text-[#7C3AED] shadow-[3px_3px_0_#101014] dark:border-white/20 dark:bg-white/10 dark:text-[#C8A8FF] dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
                  <CheckCircle2 className="size-5" />
                </div>
                <p className="font-mono text-[11px] font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">{section.kicker}</p>
                <h2 className="mt-3 text-2xl leading-tight font-black tracking-normal">{section.title}</h2>
                <p className={`mt-4 text-[15px] leading-7 ${staticMutedClass}`}>{section.body}</p>
              </article>
            ))}
          </div>
        </section>
      </main>
    </SiteShell>
  );
}
