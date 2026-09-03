"use client";

import { forwardRef, useState, type ComponentPropsWithoutRef } from "react";

type CdnFallbackImageProps = Omit<ComponentPropsWithoutRef<"img">, "src"> & {
  src: string;
  fallbackSrc?: string;
};

/** Render a CDN image first, then switch to the bundled source on load error. */
export function CdnFallbackImage(props: CdnFallbackImageProps) {
  const { src, fallbackSrc, onError, ...imageProps } = props;
  const [source, setSource] = useState(src);
  const [fallbackTried, setFallbackTried] = useState(false);

  return (
    <img
      {...imageProps}
      src={source}
      alt={imageProps.alt ?? ""}
      onError={(event) => {
        if (fallbackSrc && !fallbackTried && source !== fallbackSrc) {
          setFallbackTried(true);
          setSource(fallbackSrc);
          return;
        }
        onError?.(event);
      }}
    />
  );
}

type CdnFallbackVideoProps = Omit<ComponentPropsWithoutRef<"video">, "src"> & {
  src: string;
  fallbackSrc?: string;
  fallbackPoster?: string;
  fallbackAlt?: string;
};

/** Try the CDN clip, then the local clip, and finally its local poster. */
export const CdnFallbackVideo = forwardRef<HTMLVideoElement, CdnFallbackVideoProps>(function CdnFallbackVideo(props, ref) {
  const { src, fallbackSrc, fallbackPoster, fallbackAlt, onError, ...videoProps } = props;
  const [source, setSource] = useState(src);
  const [failed, setFailed] = useState(false);

  if (failed && fallbackPoster) {
    return (
      <CdnFallbackImage
        src={fallbackPoster}
        alt={fallbackAlt ?? ""}
        className={videoProps.className}
      />
    );
  }

  return (
    <video
      {...videoProps}
      muted
      ref={ref}
      src={source}
      onError={(event) => {
        if (!failed && fallbackSrc && source !== fallbackSrc) {
          setSource(fallbackSrc);
          return;
        }
        if (!failed && fallbackPoster) {
          setFailed(true);
          return;
        }
        onError?.(event);
      }}
    />
  );
});
