import { expect, test } from "bun:test";
import { generateMetadata as englishBlog } from "./(en)/blog/page";
import { generateMetadata as localizedBlog } from "./[locale]/blog/page";
import { generateMetadata as englishCategory } from "./(en)/blog/category/[slug]/page";
import { generateMetadata as localizedCategory } from "./[locale]/blog/category/[slug]/page";
import { SITE_ORIGIN } from "@/lib/origins";
import { blogListingSeoOptions } from "@/lib/blog-listing-seo";

test("blog pagination never advertises first-page translations and preserves robots policy", async () => {
  const searchParams = Promise.resolve({ page: "2" });
  for (const [metadata, path, index] of [
    [await englishBlog({ searchParams }), "/blog", false],
    [await localizedBlog({ params: Promise.resolve({ locale: "zh" }), searchParams }), "/zh/blog", false],
    [await englishCategory({ params: Promise.resolve({ slug: "news" }), searchParams }), "/blog/category/news", true],
    [await localizedCategory({ params: Promise.resolve({ slug: "news", locale: "de" }), searchParams }), "/de/blog/category/news", true],
  ] as const) {
    expect(metadata.alternates?.canonical).toBe(`${SITE_ORIGIN}${path}?page=2`);
    expect(metadata.alternates?.languages).toBeUndefined();
    expect(metadata.robots).toEqual({ index, follow: index });
  }
});

test("unfiltered first pages retain their language cluster; searches and malformed pages do not invent URLs", async () => {
  const metadata = await englishBlog({});
  expect(metadata.alternates?.languages?.["en-US"]).toBe(`${SITE_ORIGIN}/blog`);
  for (const search of [{ q: "term" }, { page: "1" }, { page: "0" }, { page: "-1" }, { page: ["2", "3"] }, { page: "2&x=1" }]) {
    expect(blogListingSeoOptions("/blog", search)).toEqual({ pathname: "/blog", locales: [] });
  }
  const filtered = await englishBlog({ searchParams: Promise.resolve({ q: "term" }) });
  expect(filtered.alternates?.languages).toBeUndefined();
  expect(filtered.robots).toEqual({ index: false, follow: false });
});
