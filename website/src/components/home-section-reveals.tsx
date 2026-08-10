"use client";

import { useEffect } from "react";

const ROOT_SELECTOR = "[data-fk-home-reveal-root]";
const SECTION_SELECTOR = ".fk-section-reveal";
const SCROLL_RIGHT_SELECTOR = ".fk-section-scroll-right";
const READY_ATTRIBUTE = "data-fk-section-reveal-ready";
const VISIBLE_CLASS = "fk-section-visible";
const REVEAL_VIEWPORT_RATIO = 0.66;

export function HomeSectionReveals() {
  useEffect(() => {
    const root = document.querySelector<HTMLElement>(ROOT_SELECTOR);
    if (!root) return;

    const sections = Array.from(root.querySelectorAll<HTMLElement>(SECTION_SELECTOR));
    const scrollRightSections = Array.from(root.querySelectorAll<HTMLElement>(SCROLL_RIGHT_SELECTOR));
    if (sections.length === 0 && scrollRightSections.length === 0) return;

    const reveal = (section: HTMLElement) => {
      section.classList.add(VISIBLE_CLASS);
    };

    const isInRevealZone = (section: HTMLElement) => {
      const rect = section.getBoundingClientRect();
      return rect.top < window.innerHeight * REVEAL_VIEWPORT_RATIO && rect.bottom > window.innerHeight * 0.08;
    };

    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches || typeof IntersectionObserver === "undefined") {
      sections.forEach(reveal);
      scrollRightSections.forEach((section) => {
        section.style.setProperty("--fk-scroll-opacity", "1");
        section.style.setProperty("--fk-scroll-x", "0px");
        section.style.setProperty("--fk-scroll-blur", "0px");
        section.style.setProperty("--fk-scroll-scale", "1");
      });
      root.setAttribute(READY_ATTRIBUTE, "true");
      return;
    }

    const clamp = (value: number) => Math.min(1, Math.max(0, value));
    let scrollFrame = 0;

    const updateScrollRightSections = () => {
      scrollFrame = 0;
      const viewportHeight = window.innerHeight || 1;
      const startLine = viewportHeight * 1.15;
      const endLine = viewportHeight * 0.16;
      const travelPx = Math.min(340, Math.max(150, window.innerWidth * 0.26));

      for (const section of scrollRightSections) {
        const rect = section.getBoundingClientRect();
        const rawProgress = clamp((startLine - rect.top) / Math.max(1, startLine - endLine));
        const progress = rawProgress;
        section.style.setProperty("--fk-scroll-opacity", progress.toFixed(3));
        section.style.setProperty("--fk-scroll-x", `${((1 - progress) * travelPx).toFixed(1)}px`);
        section.style.setProperty("--fk-scroll-blur", `${((1 - progress) * 6).toFixed(2)}px`);
        section.style.setProperty("--fk-scroll-scale", (0.982 + progress * 0.018).toFixed(4));
      }
    };

    const scheduleScrollUpdate = () => {
      if (scrollFrame) return;
      scrollFrame = window.requestAnimationFrame(updateScrollRightSections);
    };

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const section = entry.target as HTMLElement;
          reveal(section);
          observer.unobserve(section);
        }
      },
      { rootMargin: "0px 0px -34% 0px", threshold: 0.01 }
    );

    updateScrollRightSections();
    root.setAttribute(READY_ATTRIBUTE, "true");
    window.requestAnimationFrame(() => {
      for (const section of sections) {
        if (isInRevealZone(section)) {
          reveal(section);
        } else {
          observer.observe(section);
        }
      }
    });
    window.addEventListener("scroll", scheduleScrollUpdate, { passive: true });
    window.addEventListener("resize", scheduleScrollUpdate);

    return () => {
      if (scrollFrame) window.cancelAnimationFrame(scrollFrame);
      observer.disconnect();
      window.removeEventListener("scroll", scheduleScrollUpdate);
      window.removeEventListener("resize", scheduleScrollUpdate);
      root.removeAttribute(READY_ATTRIBUTE);
      sections.forEach((section) => section.classList.remove(VISIBLE_CLASS));
      scrollRightSections.forEach((section) => {
        section.style.removeProperty("--fk-scroll-opacity");
        section.style.removeProperty("--fk-scroll-x");
        section.style.removeProperty("--fk-scroll-blur");
        section.style.removeProperty("--fk-scroll-scale");
      });
    };
  }, []);

  return null;
}
