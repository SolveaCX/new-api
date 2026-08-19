import { notFound } from "next/navigation";
import { PromptLibraryModelPage } from "@/components/prompt-library-page";
import {
  getPromptLibraryPageCopy,
  getPromptLibraryModelSummaries,
} from "@/lib/prompt-library-public";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ slug: string }>;
};

export function generateStaticParams() {
  return getPromptLibraryModelSummaries().map((item) => ({ slug: item.slug }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  const model = getPromptLibraryModelSummaries().find((item) => item.slug === params.slug);
  if (!model) return {};
  const copy = getPromptLibraryPageCopy("en");
  return buildMetadata({
    title: `Flatkey Prompts — ${model.displayName}`,
    description: copy.modelBrowseBody,
    pathname: `/prompts/models/${params.slug}`,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  const model = getPromptLibraryModelSummaries().find((item) => item.slug === params.slug);
  if (!model) notFound();
  return <PromptLibraryModelPage locale="en" modelSlug={params.slug} />;
}
