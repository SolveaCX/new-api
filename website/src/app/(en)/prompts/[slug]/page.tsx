import { notFound } from "next/navigation";
import { PromptLibraryPromptPage } from "@/components/prompt-library-page";
import { getPromptLibraryExampleBySlug, getPromptLibraryExamples } from "@/lib/prompt-library-public";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return getPromptLibraryExamples().map((item) => ({ slug: item.slug }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  const item = getPromptLibraryExampleBySlug(params.slug);
  if (!item) return {};
  return buildMetadata({
    title: `Flatkey Prompts — ${item.title}`,
    description: item.prompt,
    pathname: `/prompts/${params.slug}`,
    image: item.previewImage.src,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  const item = getPromptLibraryExampleBySlug(params.slug);
  if (!item) notFound();
  return <PromptLibraryPromptPage locale="en" slug={params.slug} />;
}
