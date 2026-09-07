import { PromptDirectoryPage } from "@/components/prompt-directory";
import { fetchCliMediaPromptItems, promptLibraryCopy } from "@/lib/prompt-library";
import { buildMetadata } from "@/lib/seo";

type Props = { searchParams?: Promise<Record<string, string | string[] | undefined>> };

export const revalidate = 300;

export function generateMetadata() {
  const copy = promptLibraryCopy.en;
  return buildMetadata({ title: "Flatkey Prompt Directory", description: copy.metaDescription, pathname: "/prompts" });
}

export default async function Page(props: Props) {
  const searchParams = await props.searchParams;
  const items = await fetchCliMediaPromptItems();
  return <PromptDirectoryPage locale="en" items={items} initialSearch={searchParams} />;
}
