import { notFound } from "next/navigation";
import { PromptLibraryPromptPage } from "@/components/prompt-library-page";
import { isLocale, LOCALES } from "@/lib/locales";
import { getPromptLibraryExampleBySlug, getPromptLibraryExamples } from "@/lib/prompt-library-public";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string; slug: string }>;
};

export function generateStaticParams() {
  return LOCALES
    .filter((locale) => locale !== "en")
    .flatMap((locale) => getPromptLibraryExamples().map((item) => ({ locale, slug: item.slug })));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const item = getPromptLibraryExampleBySlug(params.slug);
  if (!item) return {};
  return buildMetadata({
    title: `Flatkey Prompts — ${item.title}`,
    description: item.prompt,
    pathname: `/prompts/${params.slug}`,
    locale: params.locale,
    image: item.previewImage.src,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const item = getPromptLibraryExampleBySlug(params.slug);
  if (!item) notFound();
  return <PromptLibraryPromptPage locale={params.locale} slug={params.slug} />;
}
