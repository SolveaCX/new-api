import { notFound } from "next/navigation";
import { PromptGalleryPicker } from "@/components/prompt-gallery-picker";
import { getPromptGalleryItems } from "@/lib/prompt-gallery-api";
import { getPromptGalleryPickerCopy } from "@/lib/prompt-gallery-picker-copy";
import { isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string }>;
};

export const dynamic = "force-dynamic";

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const copy = getPromptGalleryPickerCopy(params.locale);
  return buildMetadata({
    title: `${copy.title} · Flatkey`,
    description: copy.description,
    pathname: "/prompt-picker",
    locale: params.locale,
    noIndex: true,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  const result = await getPromptGalleryItems();
  return <PromptGalleryPicker locale={params.locale} {...result} />;
}
