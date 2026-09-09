import { OnlinePricingPage } from "@/components/online-pricing-page";
import { buildMetadata } from "@/lib/seo";

type Props = { searchParams?: Promise<Record<string, string | string[] | undefined>> };

export async function generateMetadata(props: Props) {
  const searchParams = await props.searchParams;
  return buildMetadata({
    title: "flatkey - Pricing",
    description:
      "flatkey pricing with Go, Pro, Max and Enterprise plans covering official models and production usage controls.",
    pathname: "/pricing",
    noIndex: Object.keys(searchParams ?? {}).length > 0,
  });
}

export default function Page() {
  return <OnlinePricingPage locale="en" />;
}
