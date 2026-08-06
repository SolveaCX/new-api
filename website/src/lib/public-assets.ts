const assetOrigin = process.env.NEXT_PUBLIC_WEBSITE_ASSET_ORIGIN?.replace(/\/+$/, "") ?? "";

export function publicAssetUrl(path: string): string {
  if (!assetOrigin || /^https?:\/\//.test(path)) return path;
  return `${assetOrigin}${path.startsWith("/") ? path : `/${path}`}`;
}
