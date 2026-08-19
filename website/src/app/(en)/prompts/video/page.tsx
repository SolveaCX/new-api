import { PromptLibraryMediaPage } from "@/components/prompt-library-page";
import { getPromptLibraryPageCopy } from "@/lib/prompt-library-public";
import { buildMetadata } from "@/lib/seo";

const copy = getPromptLibraryPageCopy("en");

export const metadata = buildMetadata({
  title: `Flatkey Prompts — ${copy.mediaTypes.video}`,
  description: copy.mediaTypeDescriptions.video,
  pathname: "/prompts/video",
});

export default function Page() {
  return <PromptLibraryMediaPage locale="en" mediaType="video" />;
}
