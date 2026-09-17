import { CareersDetailPage } from "@/components/careers-page";
import { buildMetadata } from "@/lib/seo";

export const metadata = buildMetadata({
  title: "Business Development Representative — Careers",
  description: "Own Bay Area growth for Flatkey's unified AI API gateway.",
  pathname: "/careers/business-development-representative",
  locales: ["en", "zh"],
});

export default function Page() {
  return <CareersDetailPage locale="en" pathname="/careers/business-development-representative" />;
}
