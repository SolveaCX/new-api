import { notFound } from "next/navigation";
import { PromptBrowseDetailPage } from "@/components/prompt-directory";
import { fetchCliMediaPromptItems, promptLibraryCopy } from "@/lib/prompt-library";
import { isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ locale: string; slug: string }> };

export const revalidate = 300;

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").flatMap((locale) =>
    ["product", "character", "advertising", "storyboard"].map((slug) => ({ locale, slug })),
  );
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const copy = promptLibraryCopy[params.locale];
  const label = decodeURIComponent(params.slug).replace(/[-_]+/g, " ");
  return buildMetadata({
    title: `${label} — ${copy.metaTitle}`,
    description: copy.metaDescription,
    pathname: `/prompts/use-case/${params.slug}`,
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const useCase = decodeURIComponent(params.slug);
  const items = await fetchCliMediaPromptItems();
  if (!items.some((item) => item.tags.includes(useCase))) notFound();
  return <PromptBrowseDetailPage locale={params.locale} items={items.filter((item) => item.tags.includes(useCase))} title={useCase} description={promptLibraryCopy[params.locale].metaDescription} />;
}
