import { notFound } from "next/navigation";
import { CareersDetailPage } from "@/components/careers-page";
import { buildMetadata } from "@/lib/seo";
import { isLocale, type Locale } from "@/lib/locales";

const CAREERS_LOCALES: Locale[] = ["zh", "es", "fr", "pt", "ru", "ja", "vi", "de", "id"];
type Props = { params: Promise<{ locale: string }> };

export async function generateMetadata({ params }: Props) {
  const { locale } = await params;
  if (!isLocale(locale) || !CAREERS_LOCALES.includes(locale)) return {};
  const isZh = locale === "zh";
  return buildMetadata({ title: isZh ? "商务拓展代表 — 招聘" : "Business Development Representative — Careers", description: isZh ? "加入 Flatkey San Jose 团队，在湾区推动 AI 基础设施落地。" : "Own Bay Area growth for Flatkey's unified AI API gateway.", pathname: "/careers/business-development-representative", locale, locales: ["en", "zh"] });
}

export default async function Page({ params }: Props) {
  const { locale } = await params;
  if (!isLocale(locale) || !CAREERS_LOCALES.includes(locale)) notFound();
  return <CareersDetailPage locale={locale} pathname="/careers/business-development-representative" />;
}
