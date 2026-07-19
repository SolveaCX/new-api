import { KimiLandingPage } from "@/components/kimi-landing-page";
import { getKimiLandingMetadataInput } from "@/lib/kimi-landing";
import { buildMetadata } from "@/lib/seo";

export const metadata = buildMetadata(getKimiLandingMetadataInput("en"));

export default function Page() {
  return <KimiLandingPage locale="en" />;
}
