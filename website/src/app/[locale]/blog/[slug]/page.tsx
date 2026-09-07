import { notFound } from "next/navigation";
import { BlogArticlePage } from "@/components/blog-pages";
import { getBlogPost, getBlogPostLocales } from "@/lib/blog";
import { getCopy } from "@/lib/copy";
import { isLocale } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ locale: string; slug: string }> };

export const dynamic = "force-dynamic";

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const [post, availableLocales] = await Promise.all([
    getBlogPost(params.slug, params.locale),
    getBlogPostLocales(params.slug),
  ]);
  if (!post) return {};
  const copy = getCopy(params.locale).blog;
  return buildMetadata({
    title: post?.title ?? copy.articleFallbackTitle,
    description: post?.summary ?? copy.articleFallbackDescription,
    pathname: `/blog/${params.slug}`,
    locale: params.locale,
    locales: availableLocales.length > 0 ? availableLocales : [params.locale],
    image: post.cover,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  return <BlogArticlePage locale={params.locale} slug={params.slug} />;
}
