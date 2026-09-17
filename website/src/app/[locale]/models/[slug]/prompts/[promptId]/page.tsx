import { notFound, permanentRedirect } from "next/navigation";
import { PromptDetailPage } from "@/components/prompt-detail-page";
import { getPromptDetail } from "@/lib/prompt-detail-data";
import { promptDetailPath } from "@/lib/prompt-detail";
import { isLocale } from "@/lib/locales";
import { buildPromptDetailMetadata } from "@/lib/prompt-detail-seo";

type Props = { params: Promise<{ locale: string; slug: string; promptId: string }> };

export async function generateMetadata({ params }: Props) {
  const { locale, slug, promptId } = await params;
  if (!isLocale(locale) || locale === "en") return {};
  const detail = await getPromptDetail(slug, promptId, locale);
  if (!detail) return {};
  return buildPromptDetailMetadata(detail, locale);
}

export default async function Page({ params }: Props) {
  const { locale, slug, promptId } = await params;
  if (!isLocale(locale) || locale === "en") notFound();
  const detail = await getPromptDetail(slug, promptId, locale);
  if (!detail) notFound();
  if (slug !== detail.modelSlug) permanentRedirect(promptDetailPath(detail.modelSlug, detail.id, locale));
  return <PromptDetailPage detail={detail} locale={locale} />;
}
