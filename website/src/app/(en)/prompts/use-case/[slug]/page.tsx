import { notFound } from "next/navigation";
import { PromptBrowseDetailPage } from "@/components/prompt-directory";
import { fetchCliMediaPromptItems, promptLibraryCopy } from "@/lib/prompt-library";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ slug: string }> };

export const revalidate = 300;

export async function generateMetadata(props: Props) {
  const params = await props.params;
  const copy = promptLibraryCopy.en;
  const label = decodeURIComponent(params.slug).replace(/[-_]+/g, " ");
  return buildMetadata({ title: `${label} — ${copy.metaTitle}`, description: copy.metaDescription, pathname: `/prompts/use-case/${params.slug}`, locale: "en" });
}

export default async function Page(props: Props) {
  const params = await props.params;
  const useCase = decodeURIComponent(params.slug);
  const items = await fetchCliMediaPromptItems();
  if (!items.some((item) => item.tags.includes(useCase))) notFound();
  return <PromptBrowseDetailPage locale="en" items={items.filter((item) => item.tags.includes(useCase))} title={useCase} description={promptLibraryCopy.en.metaDescription} />;
}
