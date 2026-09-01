const LOCAL_MODEL_COVER = "/assets/models-featured/flatkey-model-cover-clean.png";

/** Use the CDN directory when configured; keep local static hosting as fallback. */
export function modelCoverAssetUrl(): string {
  const cdn = process.env.NEXT_PUBLIC_MODEL_COVER_CDN?.trim().replace(/\/$/, "");
  return cdn ? `${cdn}/flatkey-model-cover-clean.png` : LOCAL_MODEL_COVER;
}
