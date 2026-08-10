"use client";

import Link from "next/link";
import { ArrowRight, ChevronDown, Globe2, Menu, X } from "lucide-react";
import { FocusEvent, useEffect, useMemo, useRef, useState } from "react";
import { FlatkeyBrandLogo } from "@/components/flatkey-brand-logo";
import { useSiteConfig } from "@/components/site-config-provider";
import { buildLanguagePreferenceCookieWrites } from "@/lib/language-routing";
import { CLI_LANDING_PATH, cliLandingCopy } from "@/lib/cli-landing";
import { getCopy } from "@/lib/copy";
import { LOCALE_LABELS, LOCALES, type Locale, localeLanguageTag, localizePath, stripLocale, withIdFallback } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";
import { TOOLS_LANDING_PATH, toolsLandingCopy } from "@/lib/tools-landing";
import { cn } from "@/lib/utils";

type Props = {
  locale: Locale;
  pathname: string;
  languageCookieDomain?: string;
  hideLanguageSwitcher?: boolean;
};

type NavItem = {
  external?: boolean;
  href: string;
  label: string;
  publicPath?: boolean;
};

type NavGroupKey = "products" | "developers" | "resources";

const legacyNavLabelByLocale: Record<Locale, { compute: string; playground: string; status: string; usecases: string }> = withIdFallback({
  en: { compute: "Compute", playground: "Playground", status: "Status", usecases: "Use cases" },
  zh: { compute: "算力", playground: "Playground", status: "服务状态", usecases: "使用场景" },
  es: { compute: "Compute", playground: "Playground", status: "Estado", usecases: "Casos de uso" },
  fr: { compute: "Compute", playground: "Playground", status: "Statut", usecases: "Cas d'usage" },
  pt: { compute: "Compute", playground: "Playground", status: "Status", usecases: "Casos de uso" },
  ru: { compute: "Compute", playground: "Playground", status: "Статус", usecases: "Сценарии" },
  ja: { compute: "Compute", playground: "Playground", status: "ステータス", usecases: "ユースケース" },
  vi: { compute: "Compute", playground: "Playground", status: "Trạng thái", usecases: "Use cases" },
  de: { compute: "Compute", playground: "Playground", status: "Status", usecases: "Anwendungsfälle" },
});

const enterpriseLabelByLocale: Record<Locale, string> = withIdFallback({
  en: "Enterprise",
  zh: "企业版",
  es: "Empresa",
  fr: "Enterprise",
  pt: "Empresarial",
  ru: "Enterprise",
  ja: "エンタープライズ",
  vi: "Doanh nghiệp",
  de: "Enterprise",
});

const dashboardLabelByLocale: Record<Locale, string> = withIdFallback({
  en: "Dashboard",
  zh: "控制台",
  es: "Dashboard",
  fr: "Dashboard",
  pt: "Dashboard",
  ru: "Dashboard",
  ja: "Dashboard",
  vi: "Dashboard",
  de: "Dashboard",
});

const startFreeLabelByLocale: Record<Locale, string> = withIdFallback({
  en: "Start free",
  zh: "免费使用",
  es: "Empieza gratis",
  fr: "Commencer",
  pt: "Começar grátis",
  ru: "Начать",
  ja: "無料で開始",
  vi: "Bắt đầu miễn phí",
  de: "Kostenlos starten",
});

const navGroupLabelByLocale: Record<Locale, { careers: string; developers: string; products: string; resources: string }> = withIdFallback({
  en: { products: "Products", developers: "Developers", resources: "Resources", careers: "Careers" },
  zh: { products: "产品", developers: "开发者", resources: "资源", careers: "加入我们" },
  es: { products: "Productos", developers: "Desarrolladores", resources: "Recursos", careers: "Carreras" },
  fr: { products: "Produits", developers: "Développeurs", resources: "Ressources", careers: "Carrières" },
  pt: { products: "Produtos", developers: "Desenvolvedores", resources: "Recursos", careers: "Carreiras" },
  ru: { products: "Продукты", developers: "Разработчикам", resources: "Ресурсы", careers: "Вакансии" },
  ja: { products: "プロダクト", developers: "開発者向け", resources: "リソース", careers: "採用情報" },
  vi: { products: "Sản phẩm", developers: "Nhà phát triển", resources: "Tài nguyên", careers: "Tuyển dụng" },
  de: { products: "Produkte", developers: "Entwickler", resources: "Ressourcen", careers: "Karriere" },
});

const topNavClass =
  "fk-nav-pill fk-header-stagger inline-flex h-10 cursor-pointer items-center whitespace-nowrap rounded-full px-3.5 text-[15px] font-extrabold text-[#24242B] outline-none transition-[background-color,color,box-shadow,transform] hover:bg-[#EEE4FF] hover:text-[#101014] hover:shadow-[inset_0_0_0_1px_rgba(16,16,20,.12)] focus-visible:bg-[#EEE4FF] focus-visible:text-[#101014] focus-visible:shadow-[inset_0_0_0_1px_rgba(16,16,20,.12)] dark:text-white/78 dark:hover:bg-white/12 dark:hover:text-white dark:hover:shadow-[inset_0_0_0_1px_rgba(255,255,255,.16)]";
const topNavActiveClass = "border-2 border-[#101014] bg-[#101014] !text-white shadow-[3px_3px_0_#C8A8FF] hover:bg-[#101014] hover:!text-white hover:shadow-[3px_3px_0_#C8A8FF] focus-visible:shadow-[3px_3px_0_#C8A8FF] dark:border-white/24 dark:bg-white dark:!text-[#101014]";
const dropdownSurfaceClass =
  "fk-popover min-w-[270px] rounded-[1.1rem] border-2 border-[#101014] bg-[#FFFDF6]/98 p-2 shadow-[5px_5px_0_#101014,0_24px_70px_-48px_rgba(16,16,20,.48)] backdrop-blur-xl dark:border-white/22 dark:bg-[#101014]/98 dark:shadow-[5px_5px_0_rgba(255,255,255,.16)]";
const dropdownItemClass =
  "fk-dropdown-item flex h-10 w-full cursor-pointer items-center justify-between rounded-[0.85rem] px-3 text-left text-[14px] font-extrabold leading-none text-[#3F3F48] outline-none transition-[background-color,color,box-shadow,transform] hover:bg-[#EEE4FF] hover:text-[#101014] hover:shadow-[inset_0_0_0_1px_rgba(16,16,20,.10)] focus-visible:bg-[#EEE4FF] focus-visible:text-[#101014] focus-visible:shadow-[inset_0_0_0_1px_rgba(16,16,20,.10)] dark:text-white/72 dark:hover:bg-white/12 dark:hover:text-white dark:hover:shadow-[inset_0_0_0_1px_rgba(255,255,255,.14)]";
const dropdownItemActiveClass = "bg-[#101014] !text-white shadow-[2px_2px_0_#C8A8FF] hover:bg-[#101014] hover:!text-white hover:shadow-[2px_2px_0_#C8A8FF] dark:bg-white dark:!text-[#101014]";

function persistLanguagePreference(locale: Locale, cookieDomain?: string) {
  for (const cookie of buildLanguagePreferenceCookieWrites(locale, cookieDomain)) {
    document.cookie = cookie;
  }
}

function StaticLanguageSelect(props: { cookieDomain?: string; locale: Locale; pathname: string }) {
  const strippedPath = stripLocale(props.pathname);
  const [open, setOpen] = useState(false);
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cancelClose = () => {
    if (closeTimerRef.current) {
      clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  };

  const openMenu = () => {
    cancelClose();
    setOpen(true);
  };

  const scheduleClose = () => {
    cancelClose();
    closeTimerRef.current = setTimeout(() => setOpen(false), 220);
  };

  const handleBlur = (event: FocusEvent<HTMLDivElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget)) {
      scheduleClose();
    }
  };

  useEffect(() => {
    return () => {
      cancelClose();
    };
  }, []);

  return (
    <div className="fk-lang-menu relative" onPointerEnter={openMenu} onPointerLeave={scheduleClose} onFocus={openMenu} onBlur={handleBlur}>
      <button
        type="button"
        aria-label="Change language"
        aria-haspopup="menu"
        aria-expanded={open}
        className="fk-icon-motion inline-flex size-10 cursor-pointer items-center justify-center rounded-full border-2 border-[#101014] bg-white text-[#101014] outline-none shadow-[3px_3px_0_#101014] transition-colors hover:bg-[#EEE4FF] focus-visible:bg-[#EEE4FF] dark:border-white/24 dark:bg-white/10 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,.16)] dark:hover:bg-white dark:hover:text-[#101014]"
      >
        <Globe2 className="size-5" strokeWidth={2.25} />
      </button>
      <div className={cn("fk-dropdown-panel absolute top-full right-0 z-50 pt-2", open ? "fk-dropdown-panel-open" : "")}>
        <div className={cn("grid gap-1", dropdownSurfaceClass)}>
          {LOCALES.map((locale) => (
            <a
              key={locale}
              lang={localeLanguageTag(locale)}
              href={localizePath(strippedPath, locale)}
              className={cn(dropdownItemClass, locale === props.locale ? dropdownItemActiveClass : "")}
              aria-current={locale === props.locale ? "page" : undefined}
              onClick={() => persistLanguagePreference(locale, props.cookieDomain)}
            >
              <span>{LOCALE_LABELS[locale]}</span>
              <span className="font-mono text-[11px] uppercase opacity-62">{locale}</span>
            </a>
          ))}
        </div>
      </div>
    </div>
  );
}

export function SiteHeader(props: Props) {
  const copy = getCopy(props.locale);
  const { docsUrl } = useSiteConfig();
  const cliCopy = cliLandingCopy[props.locale] ?? cliLandingCopy.en;
  const toolsCopy = toolsLandingCopy[props.locale] ?? toolsLandingCopy.en;
  const legacyLabels = legacyNavLabelByLocale[props.locale] ?? legacyNavLabelByLocale.en;
  const groupLabels = navGroupLabelByLocale[props.locale] ?? navGroupLabelByLocale.en;
  const startFreeLabel = startFreeLabelByLocale[props.locale] ?? startFreeLabelByLocale.en;
  const enterpriseLabel = enterpriseLabelByLocale[props.locale] ?? enterpriseLabelByLocale.en;
  const dashboardLabel = dashboardLabelByLocale[props.locale] ?? dashboardLabelByLocale.en;
  const [mobileOpen, setMobileOpen] = useState(false);
  const [authenticated, setAuthenticated] = useState(false);
  const [openMenu, setOpenMenu] = useState<NavGroupKey | null>(null);
  const navCloseTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const currentPath = stripLocale(props.pathname);
  const signInHref = consoleUrl("/sign-in", `lng=${props.locale}`);
  const signUpHref = consoleUrl("/sign-up", `lng=${props.locale}`);
  const dashboardHref = consoleUrl("/dashboard", `lng=${props.locale}`);
  const accountHref = authenticated ? dashboardHref : signInHref;
  const accountLabel = authenticated ? dashboardLabel : copy.nav.signIn;
  const isHome = currentPath === "/";

  const productItems = useMemo<NavItem[]>(
    () => [
      { href: "/models", label: copy.nav.modelPricing, publicPath: true },
      { href: TOOLS_LANDING_PATH, label: toolsCopy.navLabel, publicPath: true },
      { href: "/playground", label: legacyLabels.playground, publicPath: true },
      { href: "/rankings", label: copy.nav.rankings, publicPath: true },
      { href: "/compute", label: legacyLabels.compute, publicPath: true },
      { href: "/usecases", label: legacyLabels.usecases, publicPath: true },
    ],
    [copy.nav.modelPricing, copy.nav.rankings, legacyLabels.compute, legacyLabels.playground, legacyLabels.usecases, toolsCopy.navLabel]
  );
  const developerItems = useMemo<NavItem[]>(
    () => [
      ...(docsUrl ? [{ href: docsUrl, label: copy.nav.docs, external: true }] : []),
      { href: "/status", label: legacyLabels.status, publicPath: true },
    ],
    [copy.nav.docs, docsUrl, legacyLabels.status]
  );
  const resourceItems = useMemo<NavItem[]>(
    () => [
      { href: "/blog", label: copy.nav.blog, publicPath: true },
      { href: "/about", label: copy.nav.about, publicPath: true },
      { href: "/careers", label: groupLabels.careers, publicPath: true },
      { href: "/contact", label: copy.nav.contact, publicPath: true },
    ],
    [copy.nav.about, copy.nav.blog, copy.nav.contact, groupLabels.careers]
  );
  const topLevelItems = useMemo<NavItem[]>(
    () => [
      { href: CLI_LANDING_PATH, label: cliCopy.navLabel, publicPath: true },
      { href: "/#enterprise", label: enterpriseLabel, publicPath: true },
      { href: "/pricing", label: copy.nav.pricing, publicPath: true },
    ],
    [cliCopy.navLabel, copy.nav.pricing, enterpriseLabel]
  );
  const mobileItems = [...productItems, ...topLevelItems, ...developerItems, ...resourceItems];

  useEffect(() => {
    const controller = new AbortController();
    let active = true;

    void fetch("/api/auth/session", {
      cache: "no-store",
      credentials: "include",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) return false;
        const payload = (await response.json().catch(() => null)) as { authenticated?: boolean } | null;
        return payload?.authenticated === true;
      })
      .then((value) => {
        if (active) setAuthenticated(value);
      })
      .catch(() => {
        if (active) setAuthenticated(false);
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, []);

  useEffect(() => {
    document.body.style.overflow = mobileOpen ? "hidden" : "";
    return () => {
      document.body.style.overflow = "";
    };
  }, [mobileOpen]);

  const cancelNavClose = () => {
    if (navCloseTimerRef.current) {
      clearTimeout(navCloseTimerRef.current);
      navCloseTimerRef.current = null;
    }
  };

  const openNavMenu = (menu: NavGroupKey) => {
    cancelNavClose();
    setOpenMenu(menu);
  };

  const scheduleNavClose = () => {
    cancelNavClose();
    navCloseTimerRef.current = setTimeout(() => setOpenMenu(null), 220);
  };

  const handleNavGroupBlur = (event: FocusEvent<HTMLDivElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget)) {
      scheduleNavClose();
    }
  };

  useEffect(() => {
    return () => {
      cancelNavClose();
    };
  }, []);

  const renderNavLink = (item: NavItem, compact = false) => {
    const active = item.publicPath && currentPath === item.href;
    const className = cn(compact ? dropdownItemClass : topNavClass, active ? (compact ? dropdownItemActiveClass : topNavActiveClass) : "");

    return item.external ? (
      <a key={item.href} className={className} href={item.href} target="_blank" rel="noopener noreferrer" onClick={() => setMobileOpen(false)}>
        <span>{item.label}</span>
        {compact ? <ArrowRight className="size-3.5 -rotate-45 opacity-60" /> : null}
      </a>
    ) : (
      <Link key={item.href} className={className} href={item.publicPath ? localizePath(item.href, props.locale) : item.href} onClick={() => setMobileOpen(false)}>
        <span>{item.label}</span>
        {compact ? <ArrowRight className="size-3.5 -rotate-45 opacity-45" /> : null}
      </Link>
    );
  };

  const renderNavGroup = (key: NavGroupKey, label: string, items: NavItem[]) => {
    const open = openMenu === key;

    return (
    <div className="fk-nav-menu relative" onPointerEnter={() => openNavMenu(key)} onPointerLeave={scheduleNavClose} onFocus={() => openNavMenu(key)} onBlur={handleNavGroupBlur}>
      <button type="button" className={cn(topNavClass, "gap-1.5")} aria-haspopup="menu" aria-expanded={open}>
        <span>{label}</span>
        <ChevronDown className="fk-nav-caret size-4" strokeWidth={2.4} />
      </button>
      <div className={cn("fk-dropdown-panel absolute top-full left-0 z-50 pt-2", open ? "fk-dropdown-panel-open" : "")}>
        <div className={cn("grid gap-1", dropdownSurfaceClass)}>
          {items.map((item) => renderNavLink(item, true))}
        </div>
      </div>
    </div>
    );
  };

  return (
    <header className={cn("fk-site-header fixed inset-x-0 top-0 z-50 h-0 bg-transparent px-4 pt-3 text-[#101014]", isHome && "fk-home-header-arrival")}>
      <nav className="fk-site-nav fk-header-card mx-auto flex h-[62px] max-w-[2160px] items-center gap-3 rounded-full border-2 border-[#101014] bg-[#FBF7EE]/94 px-2 shadow-[6px_6px_0_#101014,0_18px_52px_-36px_rgba(16,16,20,.48)] backdrop-blur-xl dark:border-white/24 dark:bg-[#101014]/90 dark:shadow-[6px_6px_0_rgba(255,255,255,.16)]">
        <Link href={localizePath("/", props.locale)} className="fk-site-logo-link fk-header-stagger inline-flex h-12 cursor-pointer shrink-0 items-center rounded-full px-1.5 pr-3 transition-colors hover:bg-white/76 dark:hover:bg-white/10">
          <FlatkeyBrandLogo />
          <span className="sr-only">flatkey.ai</span>
        </Link>

        <div className="hidden min-w-0 flex-1 items-center justify-center min-[1180px]:flex">
          <div className="fk-nav-shell inline-flex items-center gap-1 rounded-full p-1">
            {renderNavGroup("products", groupLabels.products, productItems)}
            {renderNavGroup("developers", groupLabels.developers, developerItems)}
            {renderNavGroup("resources", groupLabels.resources, resourceItems)}
            {topLevelItems.map((item) => renderNavLink(item))}
          </div>
        </div>

        <div className="ml-auto hidden shrink-0 items-center gap-2 min-[1180px]:flex">
          {!props.hideLanguageSwitcher && (
            <StaticLanguageSelect locale={props.locale} pathname={props.pathname} cookieDomain={props.languageCookieDomain} />
          )}
          <div className="flex items-center gap-1 rounded-full p-1">
            <a className="fk-button-motion fk-header-stagger inline-flex h-9 cursor-pointer items-center justify-center whitespace-nowrap rounded-full border-2 border-[#101014] bg-white px-4 text-[14.5px] font-extrabold !text-[#101014] shadow-[3px_3px_0_#101014] hover:bg-[#EEE4FF] hover:!text-[#101014] dark:border-white/24 dark:bg-white/10 dark:!text-white dark:shadow-[3px_3px_0_rgba(255,255,255,.16)]" href={accountHref}>
              {accountLabel}
            </a>
            <a
              className="fk-button-motion fk-header-stagger inline-flex h-9 cursor-pointer items-center justify-center whitespace-nowrap rounded-full border-2 border-[#101014] !bg-[#101014] px-4 text-[14.5px] font-extrabold !text-white shadow-[4px_4px_0_#7C3AED] hover:!bg-[#3E36F6] hover:!text-white dark:border-white dark:!bg-white dark:!text-[#101014] dark:shadow-[4px_4px_0_#7C3AED]"
              href={signUpHref}
            >
              {startFreeLabel}
              <ArrowRight className="ml-1.5 size-4" strokeWidth={2.6} />
            </a>
          </div>
        </div>

        <button
          type="button"
          className="fk-icon-motion ml-auto inline-flex size-[42px] cursor-pointer items-center justify-center rounded-full border-2 border-[#101014] bg-white text-[#101014] shadow-[3px_3px_0_#101014] min-[1180px]:hidden dark:border-white/24 dark:bg-white/8 dark:text-white dark:shadow-[3px_3px_0_rgba(255,255,255,.16)]"
          aria-label={copy.nav.toggle}
          aria-expanded={mobileOpen}
          onClick={() => setMobileOpen((value) => !value)}
        >
          {mobileOpen ? <X className="size-5" /> : <Menu className="size-5" />}
        </button>
      </nav>

      <div
        className={cn(
          "fixed inset-x-3 top-[90px] z-40 rounded-[1.25rem] border-2 border-[#101014] bg-[#FFFDF6]/98 p-3 shadow-[6px_6px_0_#101014,0_24px_70px_-46px_rgba(16,16,20,.52)] backdrop-blur-xl transition min-[1180px]:hidden dark:border-white/22 dark:bg-[#101014]/96 dark:shadow-[6px_6px_0_rgba(255,255,255,.16)]",
          mobileOpen ? "translate-y-0 opacity-100" : "pointer-events-none -translate-y-3 opacity-0"
        )}
      >
        <div className="grid gap-1 sm:grid-cols-2">{mobileItems.map((item) => renderNavLink(item, true))}</div>
        <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-[#101014]/10 pt-4 dark:border-white/12">
          <div className="flex items-center gap-1 rounded-full bg-white/72 p-1 ring-1 ring-[#101014]/10 dark:bg-white/7 dark:ring-white/10">
            <a className="fk-button-motion inline-flex h-9 cursor-pointer items-center justify-center rounded-full border-2 border-[#101014] bg-white px-3.5 text-sm font-extrabold !text-[#101014] shadow-[3px_3px_0_#101014] hover:bg-[#EEE4FF] dark:border-white/24 dark:bg-white/10 dark:!text-white dark:shadow-[3px_3px_0_rgba(255,255,255,.16)]" href={accountHref}>
              {accountLabel}
            </a>
            <a className="fk-button-motion inline-flex h-9 cursor-pointer items-center justify-center rounded-full border-2 border-[#101014] !bg-[#101014] px-4 text-sm font-extrabold !text-white shadow-[4px_4px_0_#7C3AED] dark:border-white dark:!bg-white dark:!text-[#101014]" href={signUpHref}>
              {startFreeLabel}
            </a>
          </div>
          {!props.hideLanguageSwitcher && (
            <StaticLanguageSelect locale={props.locale} pathname={props.pathname} cookieDomain={props.languageCookieDomain} />
          )}
        </div>
      </div>
    </header>
  );
}
