"use client";

import { createContext, useContext, type ReactNode } from "react";
import {
  defaultPromoBannerSettings,
  type PromoBannerSettings,
} from "@/lib/promo-banner";
import type { PublicAnnouncement } from "@/lib/public-site-settings";

type SiteConfig = {
  docsUrl: string | null;
  promoBanner: PromoBannerSettings;
  announcements?: PublicAnnouncement[];
};

const SiteConfigContext = createContext<SiteConfig>({
  docsUrl: null,
  promoBanner: defaultPromoBannerSettings(),
});

export function SiteConfigProvider(
  props: Partial<SiteConfig> & { children: ReactNode },
) {
  return (
    <SiteConfigContext.Provider
      value={{
        docsUrl: props.docsUrl ?? null,
        promoBanner: props.promoBanner ?? defaultPromoBannerSettings(),
        announcements: props.announcements,
      }}
    >
      {props.children}
    </SiteConfigContext.Provider>
  );
}

export function useSiteConfig(): SiteConfig {
  return useContext(SiteConfigContext);
}
