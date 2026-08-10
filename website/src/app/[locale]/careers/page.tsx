import { notFound } from "next/navigation";
import { CareersPage } from "@/components/careers-page";
import { buildMetadata } from "@/lib/seo";
import { DEFAULT_LOCALE, isLocale, LOCALES } from "@/lib/locales";

type Props = { params: Promise<{ locale: string }> };

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== DEFAULT_LOCALE).map((locale) => ({ locale }));
}

export async function generateMetadata({ params }: Props) {
  const { locale } = await params;
  if (!isLocale(locale) || locale === DEFAULT_LOCALE) return {};
  return buildMetadata({
    title: "加入我们 — 来硅谷建一家 AI-native 公司",
    description: "加入 San Jose 一支小而快乐的 AI-native 团队：每个人带一队 agent 工作。我们招真正动手做出过东西的 Builder 和增长工程师。",
    pathname: "/careers",
    locale,
  });
}

export default async function Page({ params }: Props) {
  const { locale } = await params;
  if (!isLocale(locale) || locale === DEFAULT_LOCALE) notFound();
  return <CareersPage locale={locale} pathname="/careers" />;
}
