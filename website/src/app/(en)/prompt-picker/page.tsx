import { PromptGalleryPicker } from "@/components/prompt-gallery-picker";
import { getPromptGalleryItems } from "@/lib/prompt-gallery-api";
import { getPromptGalleryPickerCopy } from "@/lib/prompt-gallery-picker-copy";
import { buildMetadata } from "@/lib/seo";

export const dynamic = "force-dynamic";

const copy = getPromptGalleryPickerCopy("en");

export const metadata = buildMetadata({
  title: `${copy.title} · Flatkey`,
  description: copy.description,
  pathname: "/prompt-picker",
  noIndex: true,
});

export default async function Page() {
  const result = await getPromptGalleryItems();
  return <PromptGalleryPicker locale="en" {...result} />;
}
