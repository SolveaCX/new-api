import { PublicPage } from "@/components/public-page";
import { getPageContent } from "@/content/pages";
import { buildMetadata, getSeoLocaleOptions } from "@/lib/seo";

const content = getPageContent("about", "en");

export const metadata = buildMetadata({
  title: content.title,
  description: content.description,
  pathname: "/about",
  ...getSeoLocaleOptions("/about", "en"),
});

export default function Page() {
  return <PublicPage locale="en" pageKey="about" pathname="/about" />;
}
