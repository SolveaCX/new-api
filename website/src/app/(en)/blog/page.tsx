import { BlogIndexPage, parseBlogSearch } from "@/components/blog-pages";
import { getCopy } from "@/lib/copy";
import { buildMetadata } from "@/lib/seo";
import { blogListingSeoOptions } from "@/lib/blog-listing-seo";

const copy = getCopy("en").blog;

type Props = { searchParams?: Promise<Record<string, string | string[] | undefined>> };

export async function generateMetadata(props: Props) {
  const searchParams = await props.searchParams;
  return buildMetadata({
    title: copy.title,
    description: copy.description,
    ...blogListingSeoOptions("/blog", searchParams),
    noIndex: Object.keys(searchParams ?? {}).length > 0,
  });
}

export default async function Page(props: Props) {
  const searchParams = await props.searchParams;
  return <BlogIndexPage locale="en" search={parseBlogSearch(searchParams)} />;
}
