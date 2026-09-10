"use client";

import { useCallback, useState } from "react";

// Keep the fallback local: an unavailable image host must not break the cover too.
export const DEFAULT_BLOG_COVER = "/assets/blog-default-cover.svg";

export function blogCoverSource(cover?: string): string {
  const value = cover?.trim();
  if (!value || /[<>"'\s]/.test(value)) return DEFAULT_BLOG_COVER;
  if (value.startsWith("/") && !value.startsWith("//")) return value;
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) ? value : DEFAULT_BLOG_COVER;
  } catch {
    return DEFAULT_BLOG_COVER;
  }
}

export function BlogCover({ cover, title }: { cover?: string; title: string }) {
  const source = blogCoverSource(cover);
  const [failedSource, setFailedSource] = useState<string | null>(null);
  const [fallbackFailed, setFallbackFailed] = useState(false);
  const src = failedSource === source ? DEFAULT_BLOG_COVER : source;
  const handleFailure = useCallback(() => {
    if (src === DEFAULT_BLOG_COVER) setFallbackFailed(true);
    else setFailedSource(source);
  }, [src, source]);
  const checkInitialImage = useCallback((image: HTMLImageElement | null) => {
    // A cached failure can happen before React attaches the error listener.
    if (image?.complete && image.naturalWidth === 0) handleFailure();
  }, [handleFailure]);

  return (
    <div className="relative aspect-[16/9] overflow-hidden bg-[#f0eaff]">
      {src === DEFAULT_BLOG_COVER && fallbackFailed ? <span aria-hidden="true" className="absolute inset-0 flex items-center justify-center text-3xl font-bold tracking-tight text-[#702ee8]">
        flatkey
      </span> : null}
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        key={src}
        ref={checkInitialImage}
        src={src}
        alt={src === DEFAULT_BLOG_COVER ? "" : title}
        loading="lazy"
        decoding="async"
        hidden={src === DEFAULT_BLOG_COVER && fallbackFailed}
        onError={handleFailure}
        className="relative h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.03]"
      />
    </div>
  );
}
