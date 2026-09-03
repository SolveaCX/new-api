import { ModelCollectionsIndex } from "@/components/model-collections-page";
import { getModelCollectionsSeoCopy } from "@/lib/model-collections";
import { buildMetadata } from "@/lib/seo";

const seoCopy = getModelCollectionsSeoCopy("en");

export const metadata = buildMetadata({
  title: seoCopy.title,
  description: seoCopy.description,
  pathname: "/collections",
  absoluteTitle: true,
});

export default function Page() {
  return <ModelCollectionsIndex locale="en" />;
}
