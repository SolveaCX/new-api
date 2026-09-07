import { PublicPage } from "@/components/public-page";
import { getPageContent } from "@/content/pages";
import { buildMetadata, getSeoLocaleOptions } from "@/lib/seo";

const pageKey = "refund-policy";
const pathname = "/refund-policy";
const content = getPageContent(pageKey, "en");

export const metadata = buildMetadata({
  title: content.title,
  description: content.description,
  pathname,
  ...getSeoLocaleOptions(pathname, "en"),
});

export default function Page() {
  return <PublicPage locale="en" pageKey={pageKey} pathname={pathname} />;
}
