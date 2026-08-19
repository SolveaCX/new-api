import { notFound } from "next/navigation";
import { PromptLibraryPage } from "@/components/prompt-library-page";
import { PROMPTS_PATH, getPromptLibraryPageCopy } from "@/lib/prompt-library-public";
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
  const copy = getPromptLibraryPageCopy(params.locale);
  return buildMetadata({
    title: copy.metaTitle,
    description: copy.metaDescription,
    pathname: PROMPTS_PATH,
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  return <PromptLibraryPage locale={params.locale} />;
}
