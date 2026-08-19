"use client";

import { createContext, useContext, type ReactNode } from "react";
import {
  defaultPromoBannerSettings,
  type PromoBannerSettings,
} from "@/lib/promo-banner";

type SiteConfig = {
  docsUrl: string | null;
  promoBanner: PromoBannerSettings;
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
      }}
    >
      {props.children}
    </SiteConfigContext.Provider>
  );
}

export function useSiteConfig(): SiteConfig {
  return useContext(SiteConfigContext);
}
