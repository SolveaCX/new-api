import { notFound } from "next/navigation";
import { getPlaygroundPromptsMetadata, PlaygroundPromptsPage } from "@/components/playground-prompts-page";
import { isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string }>;
};

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const page = getPlaygroundPromptsMetadata(params.locale);
  return buildMetadata({ title: page.title, description: page.description, pathname: page.pathname, locale: params.locale });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  return <PlaygroundPromptsPage locale={params.locale} />;
}
