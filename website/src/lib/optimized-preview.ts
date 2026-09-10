// Only the oversized public previews verified in the Ahrefs report. Preserve
// original assets and all remote/user-provided media URLs unchanged.
const OVERSIZED_PREVIEWS = new Set([
  "/assets/prompts/selected-playground/book-cover.png",
  "/assets/prompts/selected-playground/ximen-qing-100-panel-storyboard.jpg",
  "/media/website-featured/de31c7e32d1280c2586d8f36b69bee1a83a2f48c4241bfc5fc790d75cdc2660f.png",
]);

export function optimizedPreviewSrc(src: string): string {
  return OVERSIZED_PREVIEWS.has(src)
    ? `/_next/image?url=${encodeURIComponent(src)}&w=1200&q=75`
    : src;
}
