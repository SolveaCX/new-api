import { notFound, permanentRedirect } from "next/navigation";
import { PromptDetailPage } from "@/components/prompt-detail-page";
import { getPromptDetail } from "@/lib/prompt-detail-data";
import { promptDetailPath } from "@/lib/prompt-detail";
import { buildPromptDetailMetadata } from "@/lib/prompt-detail-seo";

type Props = { params: Promise<{ slug: string; promptId: string }> };

export async function generateMetadata({ params }: Props) {
  const { slug, promptId } = await params;
  const detail = await getPromptDetail(slug, promptId, "en");
  return detail ? buildPromptDetailMetadata(detail, "en") : {};
}

export default async function Page({ params }: Props) {
  const { slug, promptId } = await params;
  const detail = await getPromptDetail(slug, promptId, "en");
  if (!detail) notFound();
  if (slug !== detail.modelSlug) permanentRedirect(promptDetailPath(detail.modelSlug, detail.id, "en"));
  return <PromptDetailPage detail={detail} locale="en" />;
}
