"use client";

import { useEffect, useRef } from "react";

type IntelligenceVideoProps = {
  className?: string;
  poster: string;
  src: string;
  ariaLabel: string;
};

export function IntelligenceVideo(props: IntelligenceVideoProps) {
  const videoRef = useRef<HTMLVideoElement>(null);

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

  return (
    <video
      ref={videoRef}
      className={props.className}
      autoPlay
      muted
      loop
      playsInline
      preload="auto"
      poster={props.poster}
      aria-label={props.ariaLabel}
      onLoadedData={(event) => {
        event.currentTarget.muted = true;
        void event.currentTarget.play().catch(() => undefined);
      }}
    >
      <source src={props.src} type="video/mp4" />
    </video>
  );
}
