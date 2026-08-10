import Image from "next/image";
import { LegalMarkdown, getLegalHeadings } from "@/components/legal-markdown";
import { withIdFallback } from "@/lib/locales";
import { SiteShell } from "@/components/site-shell";
import { getPageContent, type PublicPageKey } from "@/content/pages";
import type { Locale } from "@/lib/locales";

type Props = {
  locale: Locale;
  pageKey: PublicPageKey;
  pathname: string;
};

const aboutPhotos = [
  { src: "/team/amazon-accelerate-team.jpg", className: "row-span-2 md:col-span-4" },
  { src: "/team/team-dinner.jpg", className: "md:col-span-8" },
  { src: "/team/product-conversations.jpg", className: "md:col-span-4" },
  { src: "/team/seattle-community.jpg", className: "md:col-span-4" },
] as const;

const subpageGridClass =
  "pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,rgba(16,16,20,0.07)_1px,transparent_1px),linear-gradient(to_bottom,rgba(16,16,20,0.07)_1px,transparent_1px)] bg-[size:4.5rem_4.5rem] opacity-70 dark:bg-[linear-gradient(to_right,rgba(255,255,255,0.075)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.055)_1px,transparent_1px)] dark:opacity-45";
const subpageCardClass =
  "rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/94 shadow-[5px_5px_0_#101014] backdrop-blur-sm dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[5px_5px_0_rgba(255,255,255,0.16)]";
const subpageMutedClass = "text-[#5C5861] dark:text-white/62";

export function PublicPage(props: Props) {
  const content = getPageContent(props.pageKey, props.locale);
  const headings = content.document ? getLegalHeadings(content.document) : [];
  const copy = publicPageCopy(props.locale);

  if (content.document) {
    return (
      <SiteShell locale={props.locale} pathname={props.pathname}>
        <main className="public-page fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] px-4 pt-[var(--fk-header-safe-area)] pb-24 text-[#101014] antialiased sm:px-6 dark:bg-[#050507] dark:text-[#F6F3EA]">
          <div aria-hidden className={subpageGridClass} />
          <section className="relative z-10 mx-auto max-w-[2160px] border-b-2 border-[#101014] py-10 md:py-14 dark:border-white/20">
            <p className="mb-4 inline-flex rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
              {content.eyebrow}
            </p>
            <h1 className="max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
              {content.title}
            </h1>
            <p className={`mt-6 max-w-3xl text-base leading-7 text-balance md:text-lg ${subpageMutedClass}`}>
              {content.description}
            </p>
            {content.updated ? (
              <p className={`mt-4 font-mono text-xs font-bold uppercase ${subpageMutedClass}`}>
                {copy.lastUpdated}: {content.updated}
              </p>
            ) : null}
          </section>
          <section className="relative z-10 mx-auto mt-8 grid max-w-[2160px] items-start gap-12 lg:grid-cols-[minmax(0,1fr)_260px]">
            <article className={`${subpageCardClass} p-6 md:p-9`}>
              <LegalMarkdown markdown={content.document} />
            </article>
            {headings.length > 0 ? (
              <aside className="sticky top-24 hidden text-sm lg:block">
                <h2 className="mb-3 font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">
                  {copy.tableOfContents}
                </h2>
                <ul className="space-y-1.5">
                  {headings.map((heading) => (
                    <li key={heading.id}>
                      <a
                        className={`block rounded-full px-3 py-2 leading-snug font-bold transition-colors hover:bg-[#F9F871] hover:text-[#101014] dark:hover:bg-white/10 dark:hover:text-white ${subpageMutedClass}`}
                        href={`#${heading.id}`}
                      >
                        {heading.text}
                      </a>
                    </li>
                  ))}
                </ul>
              </aside>
            ) : null}
          </section>
        </main>
      </SiteShell>
    );
  }

  return (
    <SiteShell locale={props.locale} pathname={props.pathname}>
      <main className="public-page fk-subpage-surface relative min-h-screen overflow-hidden bg-[#F7F4EC] px-4 pt-[var(--fk-header-safe-area)] pb-24 text-[#101014] antialiased sm:px-6 dark:bg-[#050507] dark:text-[#F6F3EA]">
        <div aria-hidden className={subpageGridClass} />
        <section className="relative z-10 mx-auto max-w-[2160px] border-b-2 border-[#101014] py-10 md:py-14 dark:border-white/20">
          <p className="mb-4 inline-flex rounded-full border-2 border-[#101014] bg-[#F9F871] px-3 py-1.5 font-mono text-[11px] font-black uppercase shadow-[3px_3px_0_#101014] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,0.16)]">
            {content.eyebrow}
          </p>
          <h1 className="max-w-5xl text-[clamp(2.7rem,7vw,6.4rem)] leading-[0.94] font-black tracking-normal text-balance">
            {content.title}
          </h1>
          <p className={`mt-6 max-w-3xl text-base leading-7 md:text-lg ${subpageMutedClass}`}>
            {content.description}
          </p>
        </section>
        {props.pageKey === "about" ? (
          <section
            aria-hidden
            className={`${subpageCardClass} relative z-10 mx-auto mt-8 mb-12 grid max-w-[2160px] auto-rows-[170px] grid-cols-2 gap-3 overflow-hidden p-3 md:auto-rows-[220px] md:grid-cols-12`}
          >
            {aboutPhotos.map((photo) => (
              <div key={photo.src} className={`relative overflow-hidden rounded-[1rem] border border-[#101014]/12 dark:border-white/12 ${photo.className}`}>
                <Image
                  src={photo.src}
                  alt=""
                  fill
                  loading="eager"
                  sizes="(min-width: 768px) 66vw, 50vw"
                  className="object-cover transition duration-500 hover:scale-[1.02]"
                />
              </div>
            ))}
          </section>
        ) : null}
        <section className="relative z-10 mx-auto mt-8 grid max-w-[2160px] gap-5 md:grid-cols-3">
          {(content.sections ?? []).map((section) => (
            <article
              key={section.title}
              className={`${subpageCardClass} min-h-[210px] p-7 md:p-8`}
            >
              <h2 className="mb-4 text-xl font-black tracking-normal">{section.title}</h2>
              <p className={`text-sm leading-7 md:text-[15px] ${subpageMutedClass}`}>{section.body}</p>
            </article>
          ))}
        </section>
      </main>
    </SiteShell>
  );
}

const PUBLIC_PAGE_COPY: Record<Locale, { lastUpdated: string; tableOfContents: string }> =withIdFallback({
  en: { lastUpdated: "Last updated", tableOfContents: "Table of contents" },
  zh: { lastUpdated: "最后更新", tableOfContents: "目录" },
  es: { lastUpdated: "Última actualización", tableOfContents: "Índice" },
  fr: { lastUpdated: "Dernière mise à jour", tableOfContents: "Sommaire" },
  pt: { lastUpdated: "Última atualização", tableOfContents: "Índice" },
  ru: { lastUpdated: "Последнее обновление", tableOfContents: "Содержание" },
  ja: { lastUpdated: "最終更新", tableOfContents: "目次" },
  vi: { lastUpdated: "Cập nhật lần cuối", tableOfContents: "Mục lục" },
  de: { lastUpdated: "Zuletzt aktualisiert", tableOfContents: "Inhaltsverzeichnis" },
});

function publicPageCopy(locale: Locale) {
  return PUBLIC_PAGE_COPY[locale] ?? PUBLIC_PAGE_COPY.en;
}
