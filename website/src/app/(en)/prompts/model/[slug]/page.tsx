import { notFound } from "next/navigation";
import { PromptDirectoryPage } from "@/components/prompt-directory";
import { fetchCliMediaPromptItems, promptLibraryCopy } from "@/lib/prompt-library";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ slug: string }> };
export const revalidate = 300;
export async function generateMetadata(props: Props) {
  const params = await props.params;
  const copy = promptLibraryCopy.en;
  const label = decodeURIComponent(params.slug).replace(/[-_]+/g, " ");
  return buildMetadata({ title: `${label} — ${copy.metaTitle}`, description: copy.metaDescription, pathname: `/prompts/model/${params.slug}`, locale: "en" });
}
export default async function Page(props: Props) {
  const params = await props.params;
  const model = decodeURIComponent(params.slug);
  const items = await fetchCliMediaPromptItems();
  if (!items.some((item) => item.model === model)) notFound();
  return <PromptDirectoryPage locale="en" items={items} initialSearch={{ model }} />;
}
