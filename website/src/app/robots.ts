import type { MetadataRoute } from "next";
import { SITE_ORIGIN } from "@/lib/origins";

const STAGING_WEBSITE_ORIGIN = "https://staging-website.flatkey.ai";

export function buildRobots(siteOrigin: string): MetadataRoute.Robots {
  if (siteOrigin === STAGING_WEBSITE_ORIGIN) {
    return {
      rules: [
        {
          userAgent: "*",
          disallow: "/",
        },
      ],
    };
  }

  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        // API routes are authenticated/service endpoints, not indexable pages.
        // Keep them out of crawl discovery so expected 401 responses do not
        // appear as coverage errors in Google Search Console.
        disallow: ["/api/", "/cdn-cgi/", "/_next/", "/dashboard/", "/lp/"],
      },
    ],
    sitemap: `${siteOrigin}/sitemap.xml`,
  };
}

export default function robots(): MetadataRoute.Robots {
  return buildRobots(SITE_ORIGIN);
}
