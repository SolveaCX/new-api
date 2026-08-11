"use client";

import { preload } from "react-dom";

const LOADING_TRANSITION_IMAGE_PRELOADS = ["f", "l", "a", "t", "k", "e", "y"].map(
  (letter) => `/assets/mascots/flatkey-brand-letter-${letter}.webp`,
);

export function LoadingTransitionPreloads() {
  for (const href of LOADING_TRANSITION_IMAGE_PRELOADS) {
    preload(href, { as: "image", type: "image/webp" });
  }

  return null;
}
