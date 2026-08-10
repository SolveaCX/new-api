import { notFound } from "next/navigation";
import { HomePage } from "@/components/home-page";
import { getCopy } from "@/lib/copy";
import { DEFAULT_LOCALE, isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string }>;
};

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== DEFAULT_LOCALE).map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const copy = getCopy(params.locale).home;
  return buildMetadata({
    title: copy.title,
    description: copy.description,
    pathname: "/",
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === DEFAULT_LOCALE) notFound();
  return <HomePage locale={params.locale} />;
}
