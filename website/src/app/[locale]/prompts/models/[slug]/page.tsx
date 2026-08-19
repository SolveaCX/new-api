import { notFound } from "next/navigation";
import { PromptLibraryModelPage } from "@/components/prompt-library-page";
import {
  isLocale,
  LOCALES,
} from "@/lib/locales";
import {
  getPromptLibraryPageCopy,
  getPromptLibraryModelSummaries,
} from "@/lib/prompt-library-public";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string; slug: string }>;
};

export function generateStaticParams() {
  return LOCALES
    .filter((locale) => locale !== "en")
    .flatMap((locale) => getPromptLibraryModelSummaries().map((item) => ({ locale, slug: item.slug })));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const model = getPromptLibraryModelSummaries().find((item) => item.slug === params.slug);
  if (!model) return {};
  const copy = getPromptLibraryPageCopy(params.locale);
  return buildMetadata({
    title: `Flatkey Prompts — ${model.displayName}`,
    description: copy.modelBrowseBody,
    pathname: `/prompts/models/${params.slug}`,
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const model = getPromptLibraryModelSummaries().find((item) => item.slug === params.slug);
  if (!model) notFound();
  return <PromptLibraryModelPage locale={params.locale} modelSlug={params.slug} />;
}
