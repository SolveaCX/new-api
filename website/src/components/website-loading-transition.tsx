"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { FlatkeyBrandLogo } from "@/components/flatkey-brand-logo";

const LOADING_DELAY_MS = 80;

export function WebsiteLoadingTransition() {
  const [loadingTargetPath, setLoadingTargetPath] = useState<string | null>(null);
  const pathname = usePathname();
  const visible = loadingTargetPath !== null && loadingTargetPath !== pathname;

  useEffect(() => {
    let timer: number | undefined;

    const showSoon = (targetPath: string) => {
      window.clearTimeout(timer);
      timer = window.setTimeout(() => setLoadingTargetPath(targetPath), LOADING_DELAY_MS);
    };

    const onClick = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      const target = event.target instanceof Element ? event.target : null;
      const anchor = target?.closest("a[href]");
      if (!anchor) return;
      const href = anchor.getAttribute("href");
      if (!href || href.startsWith("#") || anchor.hasAttribute("download") || anchor.getAttribute("target") === "_blank") return;
      let nextUrl: URL;
      try {
        nextUrl = new URL(href, window.location.href);
      } catch {
        return;
      }
      if (nextUrl.protocol !== "http:" && nextUrl.protocol !== "https:") return;
      if (nextUrl.origin !== window.location.origin) return;
      if (nextUrl.href === window.location.href || (nextUrl.pathname === window.location.pathname && nextUrl.hash)) return;
      showSoon(nextUrl.pathname);
    };

    document.addEventListener("click", onClick, { capture: true });
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("click", onClick, { capture: true });
    };
  }, []);

  if (!visible) return null;

  return (
    <div className="fk-page-loading fixed inset-0 z-[1000] flex items-center justify-center overflow-hidden bg-[#F7F4EC]/98 px-4 dark:bg-[#050507]/98" role="status" aria-live="polite" aria-label="Loading">
      <div className="fk-page-loading-card">
        <FlatkeyBrandLogo className="fk-page-loading-logo" />
        <span className="fk-page-loading-track" aria-hidden="true" />
      </div>
    </div>
  );
}
