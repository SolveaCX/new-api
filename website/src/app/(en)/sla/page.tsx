import { PublicPage } from "@/components/public-page";
import { getPageContent } from "@/content/pages";
import { buildMetadata, getSeoLocaleOptions } from "@/lib/seo";

const content = getPageContent("sla", "en");

export const metadata = buildMetadata({
  title: content.title,
  description: content.description,
  pathname: "/sla",
  ...getSeoLocaleOptions("/sla", "en"),
});

export default function Page() {
  return <PublicPage locale="en" pageKey="sla" pathname="/sla" />;
}
