import type { MetadataRoute } from "next";
import { SITE_ORIGIN } from "@/lib/origins";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: "*",
        allow: "/",
        disallow: ["/cdn-cgi/", "/_next/", "/dashboard/", "/lp/"],
      },
    ],
    sitemap: `${SITE_ORIGIN}/sitemap.xml`,
  };
}
