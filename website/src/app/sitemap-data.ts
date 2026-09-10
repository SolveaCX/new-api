import { unstable_cache } from "next/cache";
import { getBlogSitemapData } from "@/lib/blog";
import { APP_CONSOLE_ORIGIN, SITE_ORIGIN } from "@/lib/origins";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";

export async function loadSitemapData() {
  const [blog, pricing] = await Promise.all([
    getBlogSitemapData(),
    getPricingData(WEBSITE_PUBLIC_PRICING_GROUP),
  ]);
  if (pricing.models.length === 0) throw new Error("Sitemap pricing catalog is unavailable");
  if (blog === null) throw new Error("Sitemap blog catalog is unavailable");
  return { ...blog, pricing };
}

// Cache only complete public snapshots. Revalidation failures retain the last
// successful snapshot; a cold instance fails rather than publishing omissions.
// Each Cloud Run instance may warm independently. No correctness depends on
// process-local coordination, and deployments start with their own cache.
export const getCachedSitemapData = unstable_cache(loadSitemapData, [
  "complete-sitemap-data-v1",
  SITE_ORIGIN,
  APP_CONSOLE_ORIGIN,
  process.env.BLOGGER_API_URL ?? "default",
  process.env.BLOGGER_SITE_SLUG ?? "flatkey",
], { revalidate: 300 });
