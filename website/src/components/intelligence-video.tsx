"use client";

import { useEffect, useRef, useState } from "react";

type IntelligenceVideoProps = {
  className?: string;
  poster: string;
  src: string;
  fallbackPoster?: string;
  fallbackSrc?: string;
  ariaLabel: string;
};

export function IntelligenceVideo(props: IntelligenceVideoProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [source, setSource] = useState(props.src);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const playIfVideoTabIsActive = () => {
      const tab = document.querySelector<HTMLInputElement>("#intelligence-tab-2");
      if (!tab?.checked) return;
      const video = videoRef.current;
      if (!video) return;
      video.muted = true;
      video.defaultMuted = true;
      void video.play().catch(() => undefined);
    };

    document.addEventListener("click", playIfVideoTabIsActive, true);
    document.addEventListener("change", playIfVideoTabIsActive, true);
    playIfVideoTabIsActive();
    return () => {
      document.removeEventListener("click", playIfVideoTabIsActive, true);
      document.removeEventListener("change", playIfVideoTabIsActive, true);
    };
  }, []);

  if (failed && props.fallbackPoster) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img className={props.className} src={props.fallbackPoster} alt={props.ariaLabel} />;
  }

  return (
    <video
      ref={videoRef}
      className={props.className}
      autoPlay
      muted
      loop
      playsInline
      preload="auto"
      src={source}
      poster={source === props.src ? props.poster : props.fallbackPoster ?? props.poster}
      aria-label={props.ariaLabel}
      onError={() => {
        if (props.fallbackSrc && source !== props.fallbackSrc) {
          setSource(props.fallbackSrc);
          return;
        }
        setFailed(true);
      }}
      onLoadedData={(event) => {
        event.currentTarget.muted = true;
        void event.currentTarget.play().catch(() => undefined);
      }}
    >
    </video>
  );
}
