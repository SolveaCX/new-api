import { SITE_ORIGIN } from "./origins";

/** Validate before SSR: an onError fallback is too late for non-JS crawlers. */
export function publicBlogImageSource(value?: string | null): string | undefined {
  const source = value?.trim();
  if (!source || /[<>"'\s\\]/.test(source) || source.startsWith("//")) return undefined;
  if (!source.startsWith("/") && !/^https?:\/\//i.test(source)) return undefined;
  try {
    const url = new URL(source, SITE_ORIGIN);
    const ownHost = [new URL(SITE_ORIGIN).hostname, "flatkey.ai", "www.flatkey.ai"].includes(url.hostname);
    if (/^staging[.-]/i.test(url.hostname) || url.pathname === "/api/status") return undefined;
    // Public page URLs (including relative placeholders resolved under /blog)
    // are not images. Keep uploads, static assets and external image services.
    if (ownHost && /^\/(?:[a-z]{2}\/)?blog(?:\/|$)/.test(url.pathname)) return undefined;
    if (url.username || url.password) return undefined;
    if (ownHost) return source.startsWith("/") ? source : `${SITE_ORIGIN}${url.pathname}${url.search}${url.hash}`;
    return url.protocol === "https:" ? source : undefined;
  } catch {
    return undefined;
  }
}
