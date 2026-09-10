import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { FiveCreditPromoPage } from "@/components/five-credit-promo-page";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string }>;
};

export function generateStaticParams() {
  return [{ locale: "pt" }];
}

export async function generateMetadata(props: Props): Promise<Metadata> {
  const params = await props.params;
  if (params.locale !== "pt") return {};

  return buildMetadata({
    title: "Ganhe US$5 em Créditos para APIs de IA | Flatkey",
    absoluteTitle: true,
    description: "Crie sua conta na Flatkey, resgate US$5 em créditos e comece a testar APIs de IA.",
    pathname: "/5-credit-promo",
    locale: "pt",
    locales: ["pt"],
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (params.locale !== "pt") notFound();
  return <FiveCreditPromoPage />;
}
