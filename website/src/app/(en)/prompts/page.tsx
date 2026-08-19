import { PromptLibraryPage } from "@/components/prompt-library-page";
import { PROMPTS_PATH, getPromptLibraryPageCopy } from "@/lib/prompt-library-public";
import { buildMetadata } from "@/lib/seo";

const copy = getPromptLibraryPageCopy("en");

export const metadata = buildMetadata({
  title: copy.metaTitle,
  description: copy.metaDescription,
  pathname: PROMPTS_PATH,
});

export default function Page() {
  return <PromptLibraryPage locale="en" />;
}
