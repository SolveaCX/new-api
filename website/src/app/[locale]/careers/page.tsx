import { notFound } from "next/navigation";
import { CareersPage } from "@/components/careers-page";
import { buildMetadata } from "@/lib/seo";
import { isLocale, type Locale } from "@/lib/locales";

// Careers uses Chinese copy for zh and English copy for every other localized route.
const CAREERS_LOCALES: Locale[] = ["zh", "es", "fr", "pt", "ru", "ja", "vi", "de", "id"];

type Props = { params: Promise<{ locale: string }> };

export async function generateMetadata({ params }: Props) {
  const { locale } = await params;
  if (!isLocale(locale) || !CAREERS_LOCALES.includes(locale)) return {};
  const isZh = locale === "zh";
  const indexable = locale === "zh";
  return buildMetadata({
    title: isZh ? "加入我们 — Flatkey" : "Careers — Flatkey",
    description: isZh ? "加入 Flatkey San Jose 团队，在湾区推动 AI 基础设施落地。" : "Join Flatkey in San Jose and help bring AI infrastructure to the Bay Area.",
    pathname: "/careers",
    locale,
    locales: indexable ? ["en", "zh"] : [],
    noIndex: !indexable,
  });
}

export default async function Page({ params }: Props) {
  const { locale } = await params;
  if (!isLocale(locale) || !CAREERS_LOCALES.includes(locale)) notFound();
  return <CareersPage locale={locale} pathname="/careers" />;
}
