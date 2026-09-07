import { PublicPage } from "@/components/public-page";
import { getPageContent } from "@/content/pages";
import { buildMetadata, getSeoLocaleOptions } from "@/lib/seo";

const content = getPageContent("privacy", "en");

export const metadata = buildMetadata({
  title: content.title,
  description: content.description,
  pathname: "/privacy",
  ...getSeoLocaleOptions("/privacy", "en"),
});

export default function Page() {
  return <PublicPage locale="en" pageKey="privacy" pathname="/privacy" />;
}
