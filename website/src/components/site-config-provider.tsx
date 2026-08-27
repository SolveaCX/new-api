"use client";

import { createContext, useContext, type ReactNode } from "react";
import type { PublicAnnouncement } from "@/lib/public-site-settings";

type SiteConfig = {
  docsUrl: string | null;
  announcements?: PublicAnnouncement[];
};

const SiteConfigContext = createContext<SiteConfig>({ docsUrl: null });

export function SiteConfigProvider(
  props: SiteConfig & { children: ReactNode },
) {
  return (
    <SiteConfigContext.Provider
      value={{ docsUrl: props.docsUrl, announcements: props.announcements }}
    >
      {props.children}
    </SiteConfigContext.Provider>
  );
}

export function useSiteConfig(): SiteConfig {
  return useContext(SiteConfigContext);
}
