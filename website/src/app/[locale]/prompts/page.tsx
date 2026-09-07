import { notFound } from "next/navigation";
import { PromptDirectoryPage } from "@/components/prompt-directory";
import { fetchCliMediaPromptItems, promptLibraryCopy } from "@/lib/prompt-library";
import { isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ locale: string }>; searchParams?: Promise<Record<string, string | string[] | undefined>> };

export const revalidate = 300;

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const copy = promptLibraryCopy[params.locale];
  return buildMetadata({ title: copy.metaTitle, description: copy.metaDescription, pathname: "/prompts", locale: params.locale });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const searchParams = await props.searchParams;
  const items = await fetchCliMediaPromptItems();
  return <PromptDirectoryPage locale={params.locale} items={items} initialSearch={searchParams} />;
}
