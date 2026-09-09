import { OnlineContactPage } from "@/components/online-contact-page";
import { buildMetadata } from "@/lib/seo";

type Props = { searchParams?: Promise<Record<string, string | string[] | undefined>> };

export async function generateMetadata(props: Props) {
  const searchParams = await props.searchParams;
  return buildMetadata({
    title: "flatkey - Contact sales",
    description:
      "Talk to flatkey sales for enterprise contracts below self-serve pricing, invoices, token governance and SLA support.",
    pathname: "/contact",
    noIndex: Object.keys(searchParams ?? {}).length > 0,
  });
}

export default function Page() {
  return <OnlineContactPage locale="en" />;
}
