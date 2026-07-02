import { getBlogCategories, getBlogPosts } from "@/lib/blog";
import { SITE_ORIGIN } from "@/lib/origins";

export async function GET() {
  const [posts, categories] = await Promise.all([getBlogPosts(), getBlogCategories()]);
  const lines = [
    "# flatkey.ai",
    "",
    "flatkey.ai is a unified AI API gateway, model routing, billing, and operations platform.",
    "",
    "## Core Pages",
    "",
    `- Home: ${SITE_ORIGIN}/`,
    `- Model pricing: ${SITE_ORIGIN}/pricing`,
    `- Rankings: ${SITE_ORIGIN}/rankings`,
    `- Blog: ${SITE_ORIGIN}/blog`,
    `- Sitemap: ${SITE_ORIGIN}/sitemap.xml`,
  ];

  if (categories.length > 0) {
    lines.push("", "## Blog Categories", "");
    for (const category of categories) {
      lines.push(`- ${category.name}: ${SITE_ORIGIN}/blog/category/${category.slug}`);
    }
  }

  if (posts.list.length > 0) {
    lines.push("", "## Blog Articles", "");
    for (const post of posts.list) {
      lines.push(`- ${post.title}: ${SITE_ORIGIN}/blog/${post.slug}${post.summary ? ` - ${post.summary}` : ""}`);
    }
  }

  return new Response(`${lines.join("\n")}\n`, {
    headers: {
      "content-type": "text/plain; charset=utf-8",
      "cache-control": "public, max-age=300",
    },
  });
}
