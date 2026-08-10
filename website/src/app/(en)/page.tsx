import { HomePage } from "@/components/home-page";
import { getCopy } from "@/lib/copy";
import { buildMetadata } from "@/lib/seo";

const copy = getCopy("en").home;

export const metadata = buildMetadata({
  title: copy.title,
  description: copy.description,
  pathname: "/",
});

export default function Page() {
  return <HomePage locale="en" />;
}
