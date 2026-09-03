import { ModelCollectionsIndex } from "@/components/model-collections-page";
import { buildMetadata } from "@/lib/seo";

export const metadata = buildMetadata({
  title: "AI Model Collections | Flatkey",
  description: "Browse curated AI model collections for coding, image generation, video, tool calling, and more through one API.",
  pathname: "/collections",
  absoluteTitle: true,
});

export default function Page() {
  return <ModelCollectionsIndex locale="en" />;
}
