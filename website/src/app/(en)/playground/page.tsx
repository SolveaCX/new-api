import { getPlaygroundPromptsMetadata, PlaygroundPromptsPage } from "@/components/playground-prompts-page";
import { buildMetadata } from "@/lib/seo";

const page = getPlaygroundPromptsMetadata("en");

export const metadata = buildMetadata({
  title: page.title,
  description: page.description,
  pathname: page.pathname,
});

export default function Page() {
  return <PlaygroundPromptsPage locale="en" />;
}
