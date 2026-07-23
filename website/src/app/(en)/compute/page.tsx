import { ComputeLandingPage } from "@/components/compute-landing-page";
import { getComputeLandingMetadataInput } from "@/lib/compute-landing";
import { buildMetadata } from "@/lib/seo";

export const metadata = buildMetadata(getComputeLandingMetadataInput("en"));

export default function Page() {
  return <ComputeLandingPage locale="en" />;
}
