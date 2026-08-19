import { notFound } from "next/navigation";
import { PromptLibraryMediaPage } from "@/components/prompt-library-page";
import { isLocale, LOCALES } from "@/lib/locales";
import { getPromptLibraryPageCopy } from "@/lib/prompt-library-public";
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
    title: `Flatkey Prompts — ${copy.mediaTypes.video}`,
    description: copy.mediaTypeDescriptions.video,
    pathname: "/prompts/video",
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  return <PromptLibraryMediaPage locale={params.locale} mediaType="video" />;
}
