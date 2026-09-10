import type { SeoInput } from "./seo";

/** Query views are not translation equivalents of the unfiltered first page. */
export function blogListingSeoOptions(
  pathname: string,
  search?: Record<string, string | string[] | undefined>,
): Pick<SeoInput, "pathname" | "locales"> {
  if (!search || Object.keys(search).length === 0) return { pathname };
  const page = typeof search.page === "string" && /^[1-9]\d*$/.test(search.page)
    ? Number(search.page) : 1;
  const canonicalPath = Number.isSafeInteger(page) && page > 1 ? `${pathname}?page=${page}` : pathname;
  // Locale catalogs can have different lengths/order. Do not claim page N is
  // equivalent to page 1, or invent page-N alternates that may not exist.
  // Indexability remains the caller's existing policy.
  return { pathname: canonicalPath, locales: [] };
}
