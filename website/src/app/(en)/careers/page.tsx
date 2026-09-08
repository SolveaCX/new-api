import { CareersPage } from "@/components/careers-page";
import { buildMetadata } from "@/lib/seo";

export const metadata = buildMetadata({
  title: "Careers — Flatkey",
  description: "Join Flatkey in San Jose and help bring AI infrastructure to the Bay Area.",
  pathname: "/careers",
  locales: ["en", "zh"],
});

export default function Page() {
  return <CareersPage locale="en" pathname="/careers" />;
}
