import Link from "next/link";
import type { ReactNode } from "react";
import { OnlineLanguageSelect } from "@/components/online-language-select";
import { type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { getOnlineStaticCopy } from "@/lib/online-static-copy";
import { consoleUrl } from "@/lib/origins";

type ShellProps = {
  active?: "cli" | "models" | "pricing" | "playground" | "compute" | "usecases" | "status";
  children: ReactNode;
  contactAction?: boolean;
  locale: Locale;
  pathname?: string;
};

function asset(path: string) {
  return `/assets/${path}`;
}

const navGroupLabels: Record<Locale, { menu: string; products: string; resources: string }> = withIdFallback({
  en: { products: "Product", resources: "Resource", menu: "Menu" },
  zh: { products: "产品", resources: "资源", menu: "菜单" },
  es: { products: "Producto", resources: "Recursos", menu: "Menu" },
  fr: { products: "Produit", resources: "Ressources", menu: "Menu" },
  pt: { products: "Produto", resources: "Recursos", menu: "Menu" },
  ru: { products: "Продукт", resources: "Ресурсы", menu: "Меню" },
  ja: { products: "プロダクト", resources: "リソース", menu: "メニュー" },
  vi: { products: "Sản phẩm", resources: "Tài nguyên", menu: "Menu" },
  de: { products: "Produkt", resources: "Ressourcen", menu: "Menu" },
  id: { products: "Produk", resources: "Sumber daya", menu: "Menu" },
});

type NavLink = {
  active?: ShellProps["active"];
  external?: boolean;
  href: string;
  label: string;
  target?: "_blank";
};

function NavGroup(props: { current?: ShellProps["active"]; items: NavLink[]; label: string }) {
  const isCurrent = props.items.some((item) => item.active && item.active === props.current);
  return (
    <div className={`nav-group${isCurrent ? " is-current" : ""}`}>
      <button className="nav-group-trigger" type="button" aria-haspopup="true" aria-expanded="false">
        <span className="nav-group-dot" aria-hidden="true" />
        <span className="nav-group-label">{props.label}</span>
      </button>
      <div className="nav-group-menu" role="menu">
        {props.items.map((item) => {
          const className = item.active && item.active === props.current ? "is-current-link" : undefined;
          return item.external ? (
            <a className={className} href={item.href} key={item.href} target={item.target} rel={item.target ? "noopener noreferrer" : undefined} role="menuitem">
              {item.label}
            </a>
          ) : (
            <Link className={className} href={item.href} key={item.href} role="menuitem">
              {item.label}
            </Link>
          );
        })}
      </div>
    </div>
  );
}

function MobileNavGroup(props: { current?: ShellProps["active"]; items: NavLink[]; label: string }) {
  return (
    <details className="mobile-nav-section">
      <summary>{props.label}</summary>
      <div>
        {props.items.map((item) => renderNavItem(item, props.current))}
      </div>
    </details>
  );
}

function renderNavItem(item: NavLink, current?: ShellProps["active"]) {
  const className = item.active && item.active === current ? "is-current-link" : undefined;
  return item.external ? (
    <a className={className} href={item.href} key={item.href} target={item.target} rel={item.target ? "noopener noreferrer" : undefined} role="menuitem">
      {item.label}
    </a>
  ) : (
    <Link className={className} href={item.href} key={item.href} role="menuitem">
      {item.label}
    </Link>
  );
}

export function OnlineNav(props: { active?: ShellProps["active"]; contactAction?: boolean; locale: Locale; pathname?: string }) {
  const copy = getOnlineStaticCopy(props.locale);
  const groupLabels = navGroupLabels[props.locale];
  const internalHref = (href: string) => localizePath(href, props.locale);
  const signInHref = consoleUrl("/sign-in", `lng=${props.locale}`);
  const signUpHref = consoleUrl("/sign-up", `lng=${props.locale}`);
  const startFreeLabel = (props.locale === "en" ? "Start Free" : copy.nav.start).replace(/\s*\u2192\s*$/, "");
  const products: NavLink[] = [
    { active: "models", href: internalHref("/models"), label: copy.nav.models },
    { href: internalHref("/tools"), label: copy.nav.tools },
    { active: "playground", href: internalHref("/playground"), label: copy.nav.playground },
    { active: "compute", href: internalHref("/compute"), label: copy.nav.compute },
  ];
  const resources: NavLink[] = [
    { href: internalHref("/blog"), label: props.locale === "en" ? "Blogs" : copy.footer.blog.replace(" ↗", "") },
    { href: internalHref("/rankings"), label: props.locale === "en" ? "Ranking" : copy.nav.rankings },
    { active: "usecases", href: internalHref("/usecases"), label: props.locale === "en" ? "Use Cases" : copy.nav.useCases },
    { active: "status", href: internalHref("/status"), label: copy.nav.status },
  ];
  const topLevelLinks: NavLink[] = [
    { active: "cli", href: internalHref("/cli"), label: copy.nav.cli },
    { active: "pricing", href: internalHref("/pricing"), label: copy.nav.pricing },
  ];

  return (
    <nav className="nav pricing-nav">
      <Link className="logo" href={internalHref("/")}>
        <img src={asset("flatkey-mark.svg?v=4")} alt="flatkey" />
        flatkey
      </Link>
      <div className="desktop-nav-groups">
        <NavGroup current={props.active} label={groupLabels.products} items={products} />
        <NavGroup current={props.active} label={groupLabels.resources} items={resources} />
      </div>
      {topLevelLinks.map((item) => (
        <Link href={item.href} key={item.href} className={`nav-top-link${item.active && props.active === item.active ? " on" : ""}`}>
          <span className="nav-group-dot" aria-hidden="true" />
          <span>{item.label}</span>
        </Link>
      ))}
      <div className="sp" />
      <OnlineLanguageSelect locale={props.locale} pathname={props.pathname ?? "/"} />
      <a href={signInHref} data-i18n="nav.signin">{copy.nav.signin}</a>
      <a className="btn black" href={signUpHref} data-i18n="nav.start">{startFreeLabel}</a>
      <details className="mobile-nav-menu">
        <summary>
          <span aria-hidden="true" />
          <span>{groupLabels.menu}</span>
        </summary>
        <div className="mobile-nav-panel nav-panel-grouped">
          <div className="mobile-nav-signin">
            <a href={signInHref} data-i18n="nav.signin">{copy.nav.signin}</a>
          </div>
          <div className="mobile-nav-stack">
            <MobileNavGroup current={props.active} label={groupLabels.products} items={products} />
            <MobileNavGroup current={props.active} label={groupLabels.resources} items={resources} />
            {topLevelLinks.map((item) => renderNavItem(item, props.active))}
            <OnlineLanguageSelect locale={props.locale} pathname={props.pathname ?? "/"} variant="panel" />
          </div>
        </div>
      </details>
    </nav>
  );
}

export function OnlineFooter(props: { locale: Locale }) {
  const copy = getOnlineStaticCopy(props.locale);
  return (
    <>
      <footer className="megafoot">
        <div className="cols">
          <div className="col brandcol">
            <Link className="logo" href="/">
              <img src={asset("flatkey-mark.svg?v=4")} alt="flatkey" />
              flatkey
            </Link>
            <p>{copy.footer.brand}</p>
          </div>
          <div className="col">
            <h5>{copy.footer.product}</h5>
            <Link href="/models">{copy.footer.models}</Link>
            <Link href="/playground">{copy.footer.playground}</Link>
            <Link href="/rankings">{copy.footer.rankings}</Link>
            <Link href="/pricing">{copy.nav.pricing}</Link>
            <Link href="/compute">{copy.footer.compute}</Link>
            <Link href="/usecases">{copy.footer.useCases}</Link>
            <a href={consoleUrl("/dashboard")}>{copy.footer.console}</a>
          </div>
          <div className="col">
            <h5>{copy.footer.developers}</h5>
            <Link href="/cli" data-i18n="nav.cli">CLI</Link>
            <Link href="/docs">{copy.footer.docs}</Link>
            <Link href="/docs#community">gateway-bench</Link>
            <Link href="/status">{copy.footer.apiStatus}</Link>
            <a href="https://docs.flatkey.ai/llms.txt">llms.txt</a>
            <Link href="/blog">{copy.footer.blog}</Link>
          </div>
          <div className="col">
            <h5>{copy.footer.company}</h5>
            <Link href="/careers">{copy.footer.careers}</Link>
            <Link href="/contact">{copy.footer.contact}</Link>
            <Link href="/about">{copy.footer.about}</Link>
            <Link href="/terms">{copy.footer.termsFull}</Link>
            <Link href="/privacy">{copy.footer.privacyFull}</Link>
            <Link href="/sla">{copy.footer.serviceLevelFull}</Link>
            <Link href="/refund-policy">{copy.footer.refundFull}</Link>
          </div>
          <div className="col">
            <h5>{copy.footer.social}</h5>
            <a href="https://x.com/flatkey101">X @flatkey101</a>
            <a href="mailto:support@flatkey.ai">support@flatkey.ai</a>
            <Link href="/docs#community">GitHub</Link>
            <a href="https://www.linkedin.com/company/flatkey/">LinkedIn</a>
            <a href="https://discord.gg/VrbZFDXj5g">Discord</a>
          </div>
        </div>
        <div className="trustrow">
          <span>{copy.footer.trusted}</span>
          <a href="https://www.cert-assure.com/serchresult.php?type=Management+System+Certification&certificate=USA-SOC2-220513">SOC 2 Type II</a>
          <a href="https://www.cert-assure.com/serchresult.php?type=Management+System+Certification&certificate=USA-I-270513">ISO 27001</a>
          <span className="b">{copy.footer.gdpr}</span>
          <a href="https://www.vanta.com/integrations?built-by=Partner">{copy.footer.vanta}</a>
          <span className="b">{copy.footer.zeroRetention}</span>
        </div>
        <div className="bottom">
          <div className="legal">
            {copy.footer.legalPrefix} <Link href="/terms">{copy.footer.terms}</Link> · <Link href="/privacy">{copy.footer.privacy}</Link> · <Link href="/sla">{copy.footer.serviceLevel}</Link> ·{" "}
            <Link href="/refund-policy">{copy.footer.refund}</Link>
          </div>
          <div className="word">
            <img src={asset("flatkey-mark.svg?v=4")} alt="" />
            flatkey
          </div>
        </div>
      </footer>
      <div className="stripe">
        <i className="s1" />
        <i className="s2" />
        <i className="s3" />
        <i className="s4" />
        <i className="s5" />
      </div>
    </>
  );
}

export function OnlineStaticShell(props: ShellProps) {
  return (
    <>
      <style>
        {`
          .nav-group:hover .nav-group-menu,
          .nav-group:focus-within .nav-group-menu {
            opacity: 1;
            visibility: visible;
            transform: translate(-50%,0) scale(1);
            pointer-events: auto;
          }
          .nav-group:hover .nav-group-trigger,
          .nav-group:focus-within .nav-group-trigger {
            color: var(--ink);
            background: var(--line2);
            outline: none;
          }
        `}
      </style>
      <OnlineNav active={props.active} contactAction={props.contactAction} locale={props.locale} pathname={props.pathname} />
      {props.children}
      <OnlineFooter locale={props.locale} />
    </>
  );
}
