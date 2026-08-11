import { notFound } from "next/navigation";
import { KimiLandingPage } from "@/components/kimi-landing-page";
import { getKimiLandingMetadataInput } from "@/lib/kimi-landing";
import { isLocale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string }>;
};

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") return {};
  return buildMetadata(getKimiLandingMetadataInput(params.locale));
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  return <KimiLandingPage locale={params.locale} />;
}
