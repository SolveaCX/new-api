import Link from "next/link";
import { ChevronDown, Globe2, Menu } from "lucide-react";
import type { ReactNode } from "react";
import { OnlineLanguageSelect } from "@/components/online-language-select";
import { LOCALES, LOCALE_LABELS, type Locale, localizePath, stripLocale, withIdFallback } from "@/lib/locales";
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

const navLabels: Record<
  Locale,
  {
    about: string;
    blog: string;
    careers: string;
    cli: string;
    compute: string;
    contact: string;
    developers: string;
    docs: string;
    menu: string;
    models: string;
    playground: string;
    pricing: string;
    product: string;
    rankings: string;
    resources: string;
    sales: string;
    signin: string;
    start: string;
    status: string;
    useCases: string;
  }
> = withIdFallback({
  en: { about: "About", blog: "Blog", careers: "Careers", cli: "CLI", compute: "Compute", contact: "Contact", developers: "Developers", docs: "Docs", menu: "Menu", models: "Models", playground: "Playground", pricing: "Pricing", product: "Product", rankings: "Rankings", resources: "Resources", sales: "Contact sales", signin: "Sign in", start: "Start free", status: "Status", useCases: "Use cases" },
  zh: { about: "关于", blog: "博客", careers: "加入我们", cli: "CLI", compute: "算力", contact: "联系", developers: "开发者", docs: "文档", menu: "菜单", models: "模型", playground: "Playground", pricing: "价格", product: "产品", rankings: "排行", resources: "资源", sales: "联系销售", signin: "登录", start: "免费试用", status: "状态", useCases: "使用场景" },
  es: { about: "Acerca de", blog: "Blog", careers: "Carreras", cli: "CLI", compute: "Compute", contact: "Contacto", developers: "Desarrolladores", docs: "Docs", menu: "Menú", models: "Modelos", playground: "Playground", pricing: "Precios", product: "Producto", rankings: "Rankings", resources: "Recursos", sales: "Ventas", signin: "Entrar", start: "Empezar", status: "Estado", useCases: "Casos de uso" },
  fr: { about: "À propos", blog: "Blog", careers: "Carrières", cli: "CLI", compute: "Compute", contact: "Contact", developers: "Développeurs", docs: "Docs", menu: "Menu", models: "Modèles", playground: "Playground", pricing: "Prix", product: "Produit", rankings: "Rankings", resources: "Ressources", sales: "Ventes", signin: "Connexion", start: "Essayer", status: "Statut", useCases: "Cas d'usage" },
  pt: { about: "Sobre", blog: "Blog", careers: "Carreiras", cli: "CLI", compute: "Compute", contact: "Contato", developers: "Desenvolvedores", docs: "Docs", menu: "Menu", models: "Modelos", playground: "Playground", pricing: "Preços", product: "Produto", rankings: "Rankings", resources: "Recursos", sales: "Vendas", signin: "Entrar", start: "Começar", status: "Status", useCases: "Casos de uso" },
  ru: { about: "О нас", blog: "Blog", careers: "Вакансии", cli: "CLI", compute: "Compute", contact: "Контакты", developers: "Разработчикам", docs: "Docs", menu: "Меню", models: "Модели", playground: "Playground", pricing: "Цены", product: "Продукт", rankings: "Rankings", resources: "Ресурсы", sales: "Sales", signin: "Войти", start: "Начать", status: "Status", useCases: "Сценарии" },
  ja: { about: "会社概要", blog: "Blog", careers: "採用情報", cli: "CLI", compute: "Compute", contact: "お問い合わせ", developers: "開発者向け", docs: "Docs", menu: "メニュー", models: "モデル", playground: "Playground", pricing: "料金", product: "プロダクト", rankings: "ランキング", resources: "リソース", sales: "営業に相談", signin: "ログイン", start: "無料開始", status: "Status", useCases: "ユースケース" },
  vi: { about: "Giới thiệu", blog: "Blog", careers: "Tuyển dụng", cli: "CLI", compute: "Compute", contact: "Liên hệ", developers: "Nhà phát triển", docs: "Docs", menu: "Menu", models: "Models", playground: "Playground", pricing: "Giá", product: "Sản phẩm", rankings: "Rankings", resources: "Tài nguyên", sales: "Liên hệ sales", signin: "Đăng nhập", start: "Bắt đầu", status: "Status", useCases: "Use cases" },
  de: { about: "Über uns", blog: "Blog", careers: "Karriere", cli: "CLI", compute: "Compute", contact: "Kontakt", developers: "Entwickler", docs: "Docs", menu: "Menü", models: "Modelle", playground: "Playground", pricing: "Preise", product: "Produkt", rankings: "Rankings", resources: "Ressourcen", sales: "Sales", signin: "Login", start: "Starten", status: "Status", useCases: "Anwendungsfälle" },
  id: { about: "Tentang", blog: "Blog", careers: "Karier", cli: "CLI", compute: "Komputasi", contact: "Kontak", developers: "Developer", docs: "Dokumentasi", menu: "Menu", models: "Model", playground: "Playground", pricing: "Harga", product: "Produk", rankings: "Peringkat", resources: "Resource", sales: "Hubungi sales", signin: "Masuk", start: "Mulai gratis", status: "Status", useCases: "Use case" },
});

const utilityLabels: Record<Locale, { language: string; support: string }> = withIdFallback({
  en: { language: "Language", support: "Support" },
  zh: { language: "切换语言", support: "支持" },
  es: { language: "Idioma", support: "Soporte" },
  fr: { language: "Langue", support: "Support" },
  pt: { language: "Idioma", support: "Suporte" },
  ru: { language: "Язык", support: "Поддержка" },
  ja: { language: "言語", support: "サポート" },
  vi: { language: "Ngôn ngữ", support: "Hỗ trợ" },
  de: { language: "Sprache", support: "Support" },
  id: { language: "Bahasa", support: "Dukungan" },
});

type OnlineNavItem = {
  active?: ShellProps["active"];
  external?: boolean;
  href: string;
  label: string;
};

export function OnlineNav(props: { active?: ShellProps["active"]; contactAction?: boolean; locale: Locale; pathname?: string }) {
  const internalHref = (href: string) => localizePath(href, props.locale);
  const labels = navLabels[props.locale];
  const currentPath = props.pathname ?? "/";
  const localePath = stripLocale(props.pathname ?? "/");
  const languageLabel = utilityLabels[props.locale].language;
  const navGroups: Array<{ items: OnlineNavItem[]; label: string }> = [
    {
      label: labels.product,
      items: [
        { active: "models", href: "/models", label: labels.models },
        { active: "playground", href: "/playground", label: labels.playground },
        { active: "compute", href: "/compute", label: labels.compute },
        { active: "usecases", href: "/usecases", label: labels.useCases },
        { href: "/rankings", label: labels.rankings },
      ],
    },
    {
      label: labels.developers,
      items: [
        { active: "cli", href: "/cli", label: labels.cli },
        { href: "/docs", label: labels.docs },
        { active: "status", href: "/status", label: labels.status },
      ],
    },
    {
      label: labels.resources,
      items: [
        { href: "/blog", label: labels.blog },
        { href: "/about", label: labels.about },
        { href: "/careers", label: labels.careers },
        { href: "/contact", label: labels.contact },
      ],
    },
  ];
  const primaryLinks: OnlineNavItem[] = [
    { active: "pricing", href: "/pricing", label: labels.pricing },
  ];

  const isActive = (item: OnlineNavItem) => (item.active != null && item.active === props.active) || currentPath === item.href;
  const itemHref = (item: OnlineNavItem) => (item.external ? item.href : internalHref(item.href));
  const renderMenuLink = (item: OnlineNavItem) =>
    item.external ? (
      <a className={isActive(item) ? "on" : undefined} href={item.href} key={item.href} target="_blank" rel="noopener noreferrer">
        {item.label}
      </a>
    ) : (
      <Link className={isActive(item) ? "on" : undefined} href={itemHref(item)} key={item.href}>
        {item.label}
      </Link>
    );
  const renderTopLink = (item: OnlineNavItem) =>
    item.external ? (
      <a className={`nav-top-link${isActive(item) ? " on" : ""}`} href={item.href} key={item.href} target="_blank" rel="noopener noreferrer">
        <span className="nav-group-dot" aria-hidden="true" />
        <span>{item.label}</span>
      </a>
    ) : (
      <Link className={`nav-top-link${isActive(item) ? " on" : ""}`} href={itemHref(item)} key={item.href}>
        <span className="nav-group-dot" aria-hidden="true" />
        <span>{item.label}</span>
      </Link>
    );

  return (
    <nav className="nav pricing-nav">
      <Link className="logo" href={internalHref("/")}>
        <img src={asset("flatkey-mark.svg?v=4")} alt="flatkey" />
        flatkey
      </Link>
      <div className="nav-center desktop-nav-groups">
        {navGroups.map((group) => {
          const current = group.items.some(isActive);
          return (
            <div className={`nav-group${current ? " is-current" : ""}`} key={group.label}>
              <button className="nav-group-trigger" type="button" aria-haspopup="true">
                <span className="nav-group-dot" aria-hidden="true" />
                <span>{group.label}</span>
                <ChevronDown aria-hidden="true" className="nav-chevron" />
              </button>
              <div className="nav-group-menu">{group.items.map(renderMenuLink)}</div>
            </div>
          );
        })}
        {primaryLinks.map(renderTopLink)}
      </div>
      <div className="sp" />
      <div className="mobileNavActions">
        <a className="mobileNavLogin" href={consoleUrl("/login")}>{labels.signin}</a>
        <details className="mobileNavMenu">
          <summary className="mobileNavTrigger" aria-label={labels.menu}>
            <Menu aria-hidden="true" />
            <span>{labels.menu}</span>
          </summary>
          <div className="mobileNavPanel">
            {navGroups.map((group) => (
              <div className="mobileNavGroup" key={group.label}>
                <b>{group.label}</b>
                {group.items.map(renderMenuLink)}
              </div>
            ))}
            <div className="mobileNavPrimary">{primaryLinks.map(renderMenuLink)}</div>
            <div className="mobileNavUtility">
              {props.contactAction !== false && <Link className="mobileNavSales" href={internalHref("/contact")}>{labels.sales}</Link>}
              <details className="mobileLangPage">
                <summary className="mobileLangOpen">
                  <span>
                    <Globe2 aria-hidden="true" />
                    <b>{languageLabel}</b>
                    <small>{LOCALE_LABELS[props.locale]}</small>
                  </span>
                  <ChevronDown aria-hidden="true" />
                </summary>
                <div className="mobileLangPanel">
                  <div className="mobileLangList">
                    {LOCALES.map((locale) => (
                      <a aria-current={locale === props.locale ? "true" : undefined} href={localizePath(localePath, locale)} key={locale}>
                        {LOCALE_LABELS[locale]}
                      </a>
                    ))}
                  </div>
                </div>
              </details>
            </div>
          </div>
        </details>
      </div>
      <div className="nav-actions">
        <OnlineLanguageSelect locale={props.locale} pathname={props.pathname ?? "/"} />
        {props.contactAction !== false && <Link className="btn nav-contact" href={internalHref("/contact")}>{labels.sales}</Link>}
        <a className="nav-login" href={consoleUrl("/login")}>{labels.signin}</a>
      </div>
    </nav>
  );
}

export function OnlineFooter(props: { locale: Locale }) {
  const copy = getOnlineStaticCopy(props.locale);
  const href = (path: string) => localizePath(path, props.locale);
  return (
    <>
      <footer className="megafoot">
        <div className="cols">
          <div className="col brandcol">
            <Link className="logo" href={href("/")}>
              <img src={asset("flatkey-mark.svg?v=4")} alt="flatkey" />
              flatkey
            </Link>
            <p>{copy.footer.brand}</p>
          </div>
          <div className="col">
            <h5>{copy.footer.product}</h5>
            <Link href={href("/models")}>{copy.footer.models}</Link>
            <Link href={href("/playground")}>{copy.footer.playground}</Link>
            <Link href={href("/rankings")}>{copy.footer.rankings}</Link>
            <Link href={href("/pricing")}>{copy.nav.pricing}</Link>
            <Link href={href("/compute")}>{copy.footer.compute}</Link>
            <Link href={href("/usecases")}>{copy.footer.useCases}</Link>
            <a href={consoleUrl("/dashboard")}>{copy.footer.console}</a>
          </div>
          <div className="col">
            <h5>{copy.footer.developers}</h5>
            <Link href={href("/cli")} data-i18n="nav.cli">CLI</Link>
            <Link href={href("/docs")}>{copy.footer.docs}</Link>
            <Link href={`${href("/docs")}#community`}>gateway-bench</Link>
            <Link href={href("/status")}>{copy.footer.apiStatus}</Link>
            <a href="https://docs.flatkey.ai/llms.txt">llms.txt</a>
            <Link href={href("/blog")}>{copy.footer.blog}</Link>
          </div>
          <div className="col">
            <h5>{copy.footer.company}</h5>
            <Link href={href("/careers")}>{copy.footer.careers}</Link>
            <Link href={href("/contact")}>{copy.footer.contact}</Link>
            <Link href={href("/about")}>{copy.footer.about}</Link>
            <Link href={href("/terms")}>{copy.footer.termsFull}</Link>
            <Link href={href("/privacy")}>{copy.footer.privacyFull}</Link>
            <Link href={href("/sla")}>{copy.footer.serviceLevelFull}</Link>
            <Link href={href("/refund-policy")}>{copy.footer.refundFull}</Link>
          </div>
          <div className="col">
            <h5>{copy.footer.social}</h5>
            <a href="https://x.com/flatkey101">X @flatkey101</a>
            <a href="mailto:support@flatkey.ai">support@flatkey.ai</a>
            <Link href={`${href("/docs")}#community`}>GitHub</Link>
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
            {copy.footer.legalPrefix} <Link href={href("/terms")}>{copy.footer.terms}</Link> · <Link href={href("/privacy")}>{copy.footer.privacy}</Link> · <Link href={href("/sla")}>{copy.footer.serviceLevel}</Link> ·{" "}
            <Link href={href("/refund-policy")}>{copy.footer.refund}</Link>
          </div>
          <div className="word">
            <img src={asset("flatkey-mark.svg?v=4")} alt="" />
            flatkey
          </div>
        </div>
        <div className="footer-support">
          <span>{utilityLabels[props.locale].support}</span>
          <a href="mailto:support@flatkey.ai">support@flatkey.ai</a>
          <a href="https://discord.gg/VrbZFDXj5g">Discord</a>
          <a href="https://x.com/flatkey101">X @flatkey101</a>
          <Link href={href("/contact")}>{copy.nav.contact}</Link>
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
      <link rel="stylesheet" href="/fk2.css?v=728n" />
      <style>
        {`
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            z-index: 80;
            width: 100%;
            height: 78px;
            margin: 0;
            padding-inline: 26px;
            background: transparent !important;
            border-bottom-color: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
            transition:
              height .24s ease,
              margin .24s ease,
              background-color .24s ease,
              border-color .24s ease,
              box-shadow .24s ease,
              backdrop-filter .24s ease,
              transform .24s ease;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:after {
            content: "";
            position: absolute;
            left: 28px;
            right: 28px;
            bottom: 0;
            height: 1px;
            background: linear-gradient(90deg, transparent, rgba(255,179,71,.52), rgba(255,107,61,.34), transparent);
            pointer-events: none;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo {
            color: #fff !important;
            filter: drop-shadow(0 12px 28px rgba(0,0,0,.45));
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo img {
            background: rgba(255,247,232,.92) !important;
            border: 1px solid rgba(255,179,71,.42);
            border-radius: 14px !important;
            padding: 4px !important;
            box-shadow: 0 18px 44px -26px rgba(255,179,71,.74), inset 0 1px 0 rgba(255,255,255,.65) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions {
            border: 1px solid rgba(255,255,255,.1);
            border-radius: 999px;
            background: rgba(20,13,9,.22);
            box-shadow: inset 0 1px 0 rgba(255,255,255,.08);
            backdrop-filter: blur(10px);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups {
            padding: 4px;
            gap: 4px;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-pill-link,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-login {
            min-height: 38px;
            padding-inline: 14px;
            color: rgba(255,247,232,.9) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langsel {
            text-shadow: 0 1px 22px rgba(0,0,0,.45);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-pill-link:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-login:hover {
            background: rgba(255,179,71,.14) !important;
            color: #fff !important;
            transform: translateY(-1px);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > .btn.white,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a[data-i18n="nav.signin"],
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > .langsel {
            border-radius: 14px !important;
            background: rgba(255,255,255,.08) !important;
            border: 1px solid rgba(255,255,255,.12) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.08) !important;
            backdrop-filter: blur(12px);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > .btn.black {
            background: linear-gradient(135deg, #ffbd5a 0%, #ff8a3d 100%) !important;
            color: #1a0f08 !important;
            border: 1px solid rgba(255,179,71,.58);
            box-shadow: 0 18px 44px -22px rgba(255,139,61,.9), inset 0 1px 0 rgba(255,255,255,.38) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            position: fixed;
            top: 10px;
            left: 50%;
            right: auto;
            height: 64px;
            width: min(calc(100% - 28px), 1240px);
            margin: 0;
            padding-inline: 18px;
            border: 1px solid rgba(255,179,71,.18) !important;
            border-radius: 18px;
            background: rgba(20,13,9,.78) !important;
            box-shadow: 0 24px 70px -38px rgba(0,0,0,.78), inset 0 1px 0 rgba(255,255,255,.1) !important;
            backdrop-filter: blur(20px) saturate(1.25) !important;
            transform: translateX(-50%);
            transition:
              height .24s ease,
              margin .24s ease,
              background-color .24s ease,
              border-color .24s ease,
              box-shadow .24s ease,
              backdrop-filter .24s ease,
              transform .24s ease;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:before {
            inset: 0;
            height: auto;
            border-radius: inherit;
            background:
              radial-gradient(ellipse at 8% 0, rgba(255,179,71,.12), transparent 42%),
              radial-gradient(ellipse at 92% 0, rgba(255,107,61,.14), transparent 46%);
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo {
            font-size: 34px !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo img {
            width: 42px !important;
            height: 42px !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .btn {
            min-height: 40px;
            padding: 9px 16px;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-pill-link,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-login {
            border-radius: 12px;
            background: rgba(255,255,255,.045);
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-pill-link:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-login:hover {
            background: rgba(255,179,71,.12) !important;
            transform: translateY(-1px);
          }
          .nav-center {
            display: inline-flex;
            align-items: center;
            gap: 4px;
            padding: 5px;
            border-radius: 999px;
            background: rgba(13, 17, 16, .045);
            border: 1px solid rgba(13, 17, 16, .08);
          }
          .nav a.nav-top-link,
          .nav-pill-link,
          .nav-login {
            min-height: 38px;
            display: inline-flex;
            align-items: center;
            gap: 8px;
            padding: 0 13px;
            border-radius: 999px;
            color: var(--ink2);
            text-decoration: none;
            transition:
              transform .18s ease,
              background-color .18s ease,
              color .18s ease,
              box-shadow .18s ease;
          }
          .nav a.nav-top-link:hover,
          .nav a.nav-top-link.on,
          .nav-pill-link:hover,
          .nav-pill-link.on,
          .nav-login:hover {
            color: #1a0f08;
            background: rgba(255, 179, 71, .16);
            box-shadow: inset 0 0 0 1px rgba(255, 107, 61, .14);
          }
          .nav-chevron {
            width: 13px;
            height: 13px;
            opacity: .58;
            transition: transform .16s ease, opacity .16s ease;
          }
          .nav-group:hover .nav-chevron,
          .nav-group:focus-within .nav-chevron {
            transform: rotate(180deg);
            opacity: .86;
          }
          .nav-group-menu {
            width: 236px;
            padding: 8px;
            border-radius: 18px;
            background: rgba(255,250,240,.97);
            border-color: rgba(58,72,57,.13);
            box-shadow: 0 24px 76px -44px rgba(30,32,22,.58);
          }
          .nav .nav-group-menu a {
            min-height: 42px;
            border-radius: 13px;
            color: #263126 !important;
          }
          .nav .nav-group-menu a:hover,
          .nav .nav-group-menu a.on {
            background: rgba(213,154,53,.16) !important;
            color: #6f4a13 !important;
            transform: translateX(2px);
          }
          .mobileNavMenu {
            position: relative;
            display: none;
          }
          .mobileNavActions {
            display: none;
            align-items: center;
            gap: 8px;
            margin-left: auto;
          }
          .mobileNavLogin {
            height: 40px;
            min-width: 62px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            padding: 0 14px;
            border: 1px solid rgba(4,74,113,.12);
            border-radius: 999px;
            background: #fff;
            color: #061a2c;
            text-decoration: none;
            font: 850 12px/1 var(--mono);
            box-shadow: inset 0 1px 0 rgba(255,255,255,.88);
            transition: transform .18s ease, border-color .18s ease, background-color .18s ease, color .18s ease;
          }
          .mobileNavLogin:hover,
          .mobileNavLogin:focus-visible {
            transform: translateY(-1px);
            border-color: rgba(8,119,173,.22);
            background: #e8f6ff;
            color: #063a5c;
            outline: none;
          }
          .mobileNavTrigger {
            height: 42px;
            display: inline-flex;
            align-items: center;
            gap: 8px;
            padding: 0 12px;
            border: 1px solid rgba(13,17,16,.09);
            border-radius: 999px;
            background: rgba(255,250,240,.58);
            color: var(--ink2);
            list-style: none;
            cursor: pointer;
            font: 850 12px/1 var(--mono);
            transition: transform .18s ease, border-color .18s ease, background-color .18s ease, color .18s ease;
          }
          .mobileNavTrigger::-webkit-details-marker {
            display: none;
          }
          .mobileNavTrigger svg {
            width: 17px;
            height: 17px;
          }
          .mobileNavTrigger:hover,
          .mobileNavMenu[open] .mobileNavTrigger {
            transform: translateY(-2px);
            border-color: rgba(198,141,42,.36);
            background: rgba(229,174,74,.18);
            color: #6f4a13;
          }
          .mobileNavPanel {
            position: absolute;
            top: calc(100% + 12px);
            right: 0;
            z-index: 130;
            width: min(88vw, 360px);
            display: grid;
            gap: 10px;
            padding: 14px;
            border: 1px solid rgba(58,72,57,.14);
            border-radius: 22px;
            background: rgba(255,250,240,.98);
            box-shadow: 0 30px 90px -50px rgba(30,32,22,.62);
            backdrop-filter: blur(18px);
            animation: langMenuIn .16s ease both;
          }
          .mobileNavGroup {
            display: grid;
            grid-template-columns: 1fr;
            gap: 4px;
            padding: 8px;
            border: 1px solid rgba(58,72,57,.09);
            border-radius: 17px;
            background: rgba(255,255,255,.32);
          }
          .mobileNavGroup b {
            padding: 2px 8px 6px;
            color: #8a621f;
            font: 900 10px/1 var(--mono);
            letter-spacing: .08em;
            text-transform: uppercase;
          }
          .mobileNavGroup a,
          .mobileNavPrimary a {
            display: flex;
            align-items: center;
            justify-content: space-between;
            min-height: 38px;
            padding: 0 10px;
            border-radius: 12px;
            color: #263126 !important;
            text-decoration: none;
            font-weight: 760;
            transition: transform .16s ease, background-color .16s ease, color .16s ease;
          }
          .mobileNavGroup a:hover,
          .mobileNavGroup a.on,
          .mobileNavPrimary a:hover,
          .mobileNavPrimary a.on {
            transform: translateX(2px);
            background: rgba(213,154,53,.16);
            color: #6f4a13 !important;
          }
          .mobileNavPrimary {
            display: grid;
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: 8px;
          }
          .mobileNavPrimary a {
            justify-content: center;
            min-height: 42px;
            background: #1e281f;
            color: #fffaf0 !important;
          }
          .mobileNavUtility {
            display: grid;
            grid-template-columns: minmax(0,1fr) auto;
            align-items: center;
            gap: 10px;
            padding: 10px;
            border: 1px solid rgba(58,72,57,.09);
            border-radius: 17px;
            background: rgba(255,255,255,.36);
          }
          .mobileNavSales {
            min-height: 42px;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            padding: 0 14px;
            border-radius: 13px;
            border: 1px solid rgba(13,17,16,.1);
            background: #fff;
            color: #263126 !important;
            text-decoration: none;
            font-weight: 820;
          }
          .mobileLangPage {
            position: relative;
          }
          .mobileLangOpen {
            min-height: 42px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            gap: 12px;
            padding: 0 12px;
            border-radius: 13px;
            border: 1px solid rgba(13,17,16,.1);
            background: #fff;
            color: #263126;
            list-style: none;
            cursor: pointer;
          }
          .mobileLangOpen::-webkit-details-marker {
            display: none;
          }
          .mobileLangOpen span {
            min-width: 0;
            display: inline-flex;
            align-items: center;
            gap: 8px;
          }
          .mobileLangOpen svg {
            width: 17px;
            height: 17px;
            flex: none;
          }
          .mobileLangOpen b {
            font-size: 13px;
            line-height: 1;
          }
          .mobileLangOpen small {
            color: rgba(38,49,38,.56);
            font: 800 10px/1 var(--mono);
          }
          .mobileLangPanel {
            display: none;
          }
          .nav-actions {
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 5px;
            border-radius: 999px;
            background: rgba(13, 17, 16, .045);
            border: 1px solid rgba(13, 17, 16, .08);
          }
          .nav-actions .btn,
          .nav-actions a {
            white-space: nowrap;
          }
          .langIconSelect {
            position: relative;
            width: 42px;
            height: 42px;
            display: inline-grid;
            place-items: center;
            flex: none;
            border: 1px solid rgba(13, 17, 16, .09);
            border-radius: 999px;
            background: rgba(13, 17, 16, .045);
            color: var(--ink2);
            transition:
              transform .18s ease,
              border-color .18s ease,
              background-color .18s ease,
              color .18s ease;
          }
          .langIconSelect svg {
            width: 18px;
            height: 18px;
            pointer-events: none;
          }
          .langIconSelect:hover {
            transform: translateY(-2px);
            color: #1a0f08;
            border-color: rgba(255, 107, 61, .35);
            background: rgba(255, 179, 71, .16);
          }
          .langIconSelect:active {
            transform: translateY(0) scale(.96);
          }
          .langIconSelect .langsel {
            position: absolute !important;
            inset: 0;
            width: 100% !important;
            max-width: none !important;
            height: 100% !important;
            padding: 0 !important;
            border: 0 !important;
            border-radius: inherit !important;
            background: transparent !important;
            color: transparent !important;
            opacity: 0;
            cursor: pointer;
          }
          .nav-contact {
            min-height: 40px;
            border-radius: 999px !important;
            padding: 9px 15px !important;
            background: rgba(255, 252, 245, .74) !important;
            color: #1a0f08 !important;
            border: 1px solid rgba(13, 17, 16, .1) !important;
            box-shadow: none !important;
            transition: transform .18s ease, border-color .18s ease, background-color .18s ease;
          }
          .nav-start {
            min-height: 40px;
            border-radius: 999px !important;
            padding: 9px 18px !important;
            color: #1a0f08 !important;
            background: linear-gradient(135deg, #ffcf72 0%, #ff8a3d 56%, #ff6b3d 100%) !important;
            border: 1px solid rgba(255, 179, 71, .58) !important;
            box-shadow: 0 18px 44px -25px rgba(255, 107, 61, .82), inset 0 1px 0 rgba(255,255,255,.38) !important;
          }
          .nav-contact:hover,
          .nav-start:hover {
            transform: translateY(-2px);
          }
          .nav-contact:active,
          .nav-start:active,
          .nav a.nav-top-link:active,
          .nav-pill-link:active,
          .nav-login:active {
            transform: translateY(0) scale(.97);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions {
            padding: 5px;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-contact {
            color: rgba(255,247,232,.92) !important;
            background: rgba(255,255,255,.08) !important;
            border-color: rgba(255,255,255,.13) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-start {
            background: linear-gradient(135deg, #ffcf72 0%, #ff8a3d 56%, #ff6b3d 100%) !important;
            color: #1a0f08 !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            color: rgba(255,247,232,.9);
            background: rgba(255,255,255,.08);
            border-color: rgba(255,255,255,.12);
            box-shadow: inset 0 1px 0 rgba(255,255,255,.08);
            backdrop-filter: blur(12px);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover {
            color: #fff;
            background: rgba(255,179,71,.14);
            border-color: rgba(255,179,71,.36);
          }
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
          .footer-support {
            display: flex;
            flex-wrap: wrap;
            align-items: center;
            justify-content: center;
            gap: 10px 16px;
            margin-top: 24px;
            padding-top: 18px;
            border-top: 1px solid rgba(255,255,255,.12);
            color: rgba(247,243,234,.62);
            font: 700 11px/1.4 var(--mono);
            letter-spacing: .04em;
            text-transform: uppercase;
          }
          .footer-support a {
            color: rgba(247,243,234,.82);
            text-decoration: none;
            transition: color .16s ease, transform .16s ease;
          }
          .footer-support a:hover {
            color: #bbff4d;
            transform: translateY(-2px);
          }
          .footer-support a:active {
            transform: translateY(0) scale(.96);
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo {
            position: relative;
            min-height: 48px;
            padding: 5px 14px 5px 6px;
            border-radius: 999px;
            transition:
              transform .18s ease,
              background-color .18s ease,
              box-shadow .18s ease,
              color .18s ease;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:before {
            content: "";
            position: absolute;
            inset: 3px;
            z-index: -1;
            border-radius: inherit;
            background: rgba(248, 238, 220, .06);
            border: 1px solid rgba(248, 238, 220, .1);
            opacity: 0;
            transition: opacity .18s ease, background-color .18s ease, border-color .18s ease;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo img {
            transition: transform .22s ease, box-shadow .22s ease, background-color .22s ease;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:hover {
            transform: translateY(-2px);
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:hover:before {
            opacity: 1;
            background: rgba(244, 197, 107, .12);
            border-color: rgba(244, 197, 107, .28);
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:hover img {
            transform: rotate(-6deg) scale(1.08);
            box-shadow: 0 20px 42px -24px rgba(244,197,107,.85) !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:active {
            transform: translateY(0) scale(.97);
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo {
            color: #162019 !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo:before {
            opacity: 1;
            background: rgba(255, 248, 234, .82);
            border-color: rgba(40, 58, 47, .1);
            box-shadow: 0 18px 46px -32px rgba(19, 26, 22, .45);
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo:hover:before {
            background: #fff8ea;
            border-color: rgba(196, 142, 42, .35);
          }
          .themeToggle {
            width: 42px;
            height: 42px;
            display: inline-grid;
            place-items: center;
            flex: none;
            border: 1px solid rgba(13, 17, 16, .09);
            border-radius: 999px;
            background: rgba(13, 17, 16, .045);
            color: var(--ink2);
            transition: transform .18s ease, border-color .18s ease, background-color .18s ease, color .18s ease, box-shadow .18s ease;
          }
          .themeToggle svg {
            width: 18px;
            height: 18px;
          }
          .themeToggle:hover {
            transform: translateY(-2px);
            color: #1f1608;
            border-color: rgba(198, 141, 42, .36);
            background: rgba(229, 174, 74, .18);
          }
          .themeToggle:active {
            transform: translateY(0) scale(.96);
          }
          .langIconSelect summary {
            width: 100%;
            height: 100%;
            display: grid;
            place-items: center;
            list-style: none;
            border-radius: inherit;
            cursor: pointer;
          }
          .langIconSelect summary::-webkit-details-marker {
            display: none;
          }
          .langMenu {
            position: absolute;
            top: calc(100% + 10px);
            right: 0;
            z-index: 120;
            width: 168px;
            display: grid;
            gap: 4px;
            padding: 8px;
            border: 1px solid rgba(47, 54, 38, .14);
            border-radius: 18px;
            background: rgba(255, 250, 240, .96);
            box-shadow: 0 24px 70px -42px rgba(30, 31, 22, .58);
            backdrop-filter: blur(18px);
            transform-origin: top right;
            animation: langMenuIn .16s ease both;
          }
          .langMenu a {
            display: flex;
            align-items: center;
            min-height: 34px;
            padding: 0 10px;
            border-radius: 11px;
            color: #273126 !important;
            text-decoration: none;
            text-shadow: none !important;
            font: 750 12px/1 var(--mono);
            transition: transform .16s ease, background-color .16s ease, color .16s ease;
          }
          .langMenu a:hover,
          .langMenu a[aria-current="true"] {
            transform: translateX(2px);
            background: rgba(213, 154, 53, .16);
            color: #714812 !important;
          }
          @keyframes langMenuIn {
            from { opacity: 0; transform: translateY(-4px) scale(.98); }
            to { opacity: 1; transform: translateY(0) scale(1); }
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo {
            gap: 10px !important;
            min-height: 44px !important;
            padding: 5px 13px 5px 5px !important;
            font-size: 34px !important;
            letter-spacing: -1.5px !important;
            background: rgba(255,250,240,.42);
            border: 1px solid rgba(58,72,57,.1);
            box-shadow: 0 18px 46px -38px rgba(30,32,22,.45);
            backdrop-filter: blur(12px);
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo img {
            width: 38px !important;
            height: 38px !important;
            border-radius: 12px !important;
            padding: 3px !important;
            background: #fffaf0 !important;
            border: 1px solid rgba(213,154,53,.24) !important;
            box-shadow: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-pill-link,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-login {
            color: #1e281f !important;
            text-shadow: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions {
            background: rgba(255,250,240,.46);
            border-color: rgba(58,72,57,.1);
            box-shadow: 0 18px 46px -38px rgba(30,32,22,.45);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .themeToggle {
            color: #243024;
            background: rgba(255,250,240,.58);
            border-color: rgba(58,72,57,.12);
            box-shadow: 0 18px 46px -38px rgba(30,32,22,.45);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .themeToggle:hover {
            color: #6f4a13;
            background: rgba(229,174,74,.2);
            border-color: rgba(198,141,42,.38);
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            border-color: rgba(58,72,57,.13) !important;
            background: rgba(255,250,240,.86) !important;
            box-shadow: 0 22px 70px -42px rgba(30,32,22,.55), inset 0 1px 0 rgba(255,255,255,.58) !important;
          }
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav > a.logo,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav a.nav-pill-link,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .nav-login {
            color: #f4ead8 !important;
          }
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav > a.logo,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .nav-actions,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .langIconSelect,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .themeToggle {
            background: rgba(29, 27, 22, .66) !important;
            border-color: rgba(244,234,216,.14) !important;
          }
          html.fk-theme-night .langMenu {
            background: rgba(29,27,22,.96);
            border-color: rgba(244,234,216,.14);
          }
          html.fk-theme-night .mobileNavTrigger {
            color: #f4ead8;
            background: rgba(29,27,22,.66);
            border-color: rgba(244,234,216,.14);
          }
          html.fk-theme-night .mobileNavPanel {
            background: rgba(29,27,22,.97);
            border-color: rgba(244,234,216,.14);
          }
          html.fk-theme-night .mobileNavGroup {
            background: rgba(255,255,255,.04);
            border-color: rgba(244,234,216,.1);
          }
          html.fk-theme-night .mobileNavGroup b {
            color: #f0c36c;
          }
          html.fk-theme-night .mobileNavGroup a,
          html.fk-theme-night .mobileNavPrimary a {
            color: #f4ead8 !important;
          }
          html.fk-theme-night .mobileNavGroup a:hover,
          html.fk-theme-night .mobileNavGroup a.on {
            color: #f0c36c !important;
            background: rgba(240,195,108,.16);
          }
          html.fk-theme-night .langMenu a {
            color: #f4ead8 !important;
          }
          html.fk-theme-night .langMenu a:hover,
          html.fk-theme-night .langMenu a[aria-current="true"] {
            background: rgba(240,195,108,.18);
            color: #f0c36c !important;
          }
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav > a.logo img {
            background: #f4ead8 !important;
          }
          body:has(> header.hero.heroUnified) > .nav {
            isolation: isolate;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav {
            height: 82px;
            padding-inline: clamp(18px, 2.8vw, 38px);
            background: linear-gradient(180deg, rgba(9,9,12,.34), rgba(9,9,12,0)) !important;
            border-bottom-color: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:after {
            left: clamp(18px, 2.8vw, 38px);
            right: clamp(18px, 2.8vw, 38px);
            background: linear-gradient(90deg, transparent, rgba(255,255,255,.24), rgba(255,138,92,.34), transparent);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo {
            background: rgba(255,255,255,.12) !important;
            border-color: rgba(255,255,255,.16) !important;
            color: #fffaf4 !important;
            text-shadow: 0 10px 34px rgba(0,0,0,.42) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.16), 0 18px 48px -34px rgba(0,0,0,.72) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo img {
            background: #fffaf4 !important;
            border-color: rgba(255,138,92,.3) !important;
            box-shadow: 0 14px 32px -22px rgba(255,138,92,.88) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions {
            background: rgba(255,255,255,.12) !important;
            border-color: rgba(255,255,255,.16) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.14), 0 18px 52px -38px rgba(0,0,0,.7) !important;
            backdrop-filter: blur(16px) saturate(1.22) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-login {
            color: rgba(255,250,244,.88) !important;
            text-shadow: 0 8px 28px rgba(0,0,0,.42) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-login:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger {
            background: rgba(255,255,255,.18) !important;
            color: #fff !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .themeToggle {
            color: #fffaf4 !important;
            background: rgba(255,255,255,.12) !important;
            border-color: rgba(255,255,255,.18) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.14), 0 18px 52px -38px rgba(0,0,0,.7) !important;
            backdrop-filter: blur(16px) saturate(1.22) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-contact {
            color: #fffaf4 !important;
            background: rgba(255,255,255,.13) !important;
            border-color: rgba(255,255,255,.16) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-start {
            color: #21130f !important;
            background: linear-gradient(135deg, #ffe0b8 0%, #ff9f6e 46%, #ff7358 100%) !important;
            border-color: rgba(255,255,255,.2) !important;
            box-shadow: 0 22px 48px -26px rgba(255,115,88,.92), inset 0 1px 0 rgba(255,255,255,.46) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            top: 12px;
            height: 66px;
            width: min(calc(100% - 28px), 1248px);
            border-radius: 22px;
            border-color: rgba(255,122,89,.22) !important;
            background: rgba(255,252,246,.86) !important;
            color: #151311 !important;
            box-shadow: 0 26px 82px -48px rgba(42,30,22,.48), inset 0 1px 0 rgba(255,255,255,.74) !important;
            backdrop-filter: blur(22px) saturate(1.26) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:before {
            background:
              radial-gradient(ellipse at 8% 0, rgba(255,122,89,.14), transparent 44%),
              radial-gradient(ellipse at 92% 0, rgba(131,105,255,.12), transparent 48%);
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo {
            color: #151311 !important;
            background: rgba(255,255,255,.64) !important;
            border-color: rgba(22,22,26,.08) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions {
            background: rgba(255,255,255,.48) !important;
            border-color: rgba(22,22,26,.08) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-login {
            color: #3a3631 !important;
            text-shadow: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-login:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger {
            background: rgba(255,122,89,.13) !important;
            color: #17110f !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-dot {
            background: #ff7a59 !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .langIconSelect,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .themeToggle {
            color: #25211d !important;
            background: rgba(255,255,255,.58) !important;
            border-color: rgba(22,22,26,.1) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-contact {
            background: rgba(255,255,255,.6) !important;
            border-color: rgba(22,22,26,.09) !important;
            color: #25211d !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-start {
            color: #fff !important;
            background: linear-gradient(135deg, #17110f 0%, #3a2924 100%) !important;
            border-color: rgba(22,22,26,.12) !important;
            box-shadow: 0 18px 42px -28px rgba(23,17,15,.74), inset 0 1px 0 rgba(255,255,255,.14) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu {
            border-color: rgba(255,122,89,.18) !important;
            background: rgba(255,252,246,.96) !important;
            box-shadow: 0 30px 90px -52px rgba(42,30,22,.56) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a {
            color: #302c28 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on {
            background: linear-gradient(90deg, rgba(255,122,89,.14), rgba(131,105,255,.1)) !important;
            color: #17110f !important;
          }
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav {
            background: rgba(18,16,18,.72) !important;
            border-color: rgba(255,224,184,.18) !important;
          }
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .nav-actions,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .langIconSelect,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .themeToggle {
            background: rgba(255,255,255,.08) !important;
            border-color: rgba(255,255,255,.13) !important;
          }
          body:has(> header.hero.heroUnified) > .nav {
            --nav-sky: #72cfff;
            --nav-sky-soft: #e8f8ff;
            --nav-pink: #ff9fc0;
            --nav-ink: #0d1b2d;
            --nav-muted: #5d6c7c;
            isolation: isolate;
            color: var(--nav-ink) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav {
            height: 76px !important;
            padding-inline: clamp(18px, 3vw, 42px) !important;
            background: linear-gradient(180deg, rgba(255,255,255,.62), rgba(255,255,255,0)) !important;
            border-bottom-color: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:after {
            left: clamp(18px, 3vw, 42px) !important;
            right: clamp(18px, 3vw, 42px) !important;
            background: linear-gradient(90deg, transparent, rgba(114,207,255,.5), rgba(255,159,192,.5), transparent) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            top: 10px !important;
            height: 64px !important;
            width: min(calc(100% - 28px), 1240px) !important;
            border: 1px solid rgba(114,207,255,.34) !important;
            border-radius: 22px !important;
            background: rgba(250,253,255,.84) !important;
            color: var(--nav-ink) !important;
            box-shadow: 0 24px 80px -48px rgba(61,116,150,.34), inset 0 1px 0 rgba(255,255,255,.86) !important;
            backdrop-filter: blur(22px) saturate(1.18) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:before {
            background:
              radial-gradient(ellipse at 10% 0, rgba(114,207,255,.18), transparent 46%),
              radial-gradient(ellipse at 92% 0, rgba(255,159,192,.16), transparent 48%) !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo {
            gap: 9px !important;
            min-height: 42px !important;
            padding: 4px 12px 4px 4px !important;
            border-radius: 999px !important;
            color: var(--nav-ink) !important;
            background: rgba(255,255,255,.54) !important;
            border: 1px solid rgba(114,207,255,.24) !important;
            box-shadow: 0 18px 46px -36px rgba(61,116,150,.34) !important;
            text-shadow: none !important;
            backdrop-filter: blur(14px) saturate(1.1) !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo img {
            width: 36px !important;
            height: 36px !important;
            padding: 3px !important;
            border-radius: 12px !important;
            background: linear-gradient(135deg, #e5f7ff 0%, #fff0f6 100%) !important;
            border: 1px solid rgba(114,207,255,.36) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.9) !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:hover {
            color: #0b78ad !important;
            transform: translateY(-2px);
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:hover:before {
            opacity: 1;
            background: rgba(232,248,255,.88) !important;
            border-color: rgba(255,159,192,.34) !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:hover img {
            transform: rotate(-4deg) scale(1.06);
            box-shadow: 0 18px 38px -26px rgba(114,207,255,.78) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          body:has(> header.hero.heroUnified) > .nav .nav-actions {
            background: rgba(255,255,255,.5) !important;
            border: 1px solid rgba(114,207,255,.22) !important;
            box-shadow: 0 18px 50px -42px rgba(61,116,150,.32), inset 0 1px 0 rgba(255,255,255,.72) !important;
            backdrop-filter: blur(16px) saturate(1.1) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          body:has(> header.hero.heroUnified) > .nav a.nav-pill-link,
          body:has(> header.hero.heroUnified) > .nav .nav-login {
            color: var(--nav-ink) !important;
            text-shadow: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          body:has(> header.hero.heroUnified) > .nav a.nav-pill-link:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-login:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger {
            color: #0b78ad !important;
            background: rgba(232,248,255,.72) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-dot {
            background: rgba(114,207,255,.9) !important;
            box-shadow: 0 0 0 3px rgba(114,207,255,.14) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group.is-current .nav-group-dot,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link.on .nav-group-dot {
            background: #ff8fb7 !important;
            box-shadow: 0 0 0 4px rgba(255,159,192,.18) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu,
          body:has(> header.hero.heroUnified) > .nav .langMenu {
            border: 1px solid rgba(114,207,255,.24) !important;
            background: rgba(255,255,255,.97) !important;
            box-shadow: 0 28px 86px -50px rgba(61,116,150,.38) !important;
            backdrop-filter: blur(20px) saturate(1.14) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a,
          body:has(> header.hero.heroUnified) > .nav .langMenu a {
            color: #18304b !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:hover,
          body:has(> header.hero.heroUnified) > .nav .langMenu a[aria-current="true"] {
            background: linear-gradient(90deg, rgba(232,248,255,.96), rgba(255,239,247,.92)) !important;
            color: #0b78ad !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            width: 54px !important;
            color: #0d1b2d !important;
            background: rgba(255,255,255,.54) !important;
            border-color: rgba(114,207,255,.28) !important;
            box-shadow: 0 18px 46px -38px rgba(61,116,150,.34), inset 0 1px 0 rgba(255,255,255,.72) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect summary {
            display: inline-flex !important;
            align-items: center !important;
            justify-content: center !important;
            gap: 4px !important;
            padding: 0 7px !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect svg {
            width: 17px !important;
            height: 17px !important;
            stroke-width: 2.25 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langCurrent {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            min-width: 18px;
            height: 18px;
            border-radius: 999px;
            background: linear-gradient(135deg, rgba(232,248,255,.96), rgba(255,239,247,.96));
            color: #0b78ad;
            font: 900 9px/1 var(--mono);
            letter-spacing: 0;
            box-shadow: inset 0 0 0 1px rgba(114,207,255,.24);
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect[open] {
            color: #0b78ad !important;
            background: rgba(232,248,255,.9) !important;
            border-color: rgba(255,159,192,.42) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover .langCurrent,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect[open] .langCurrent {
            background: linear-gradient(135deg, #72cfff, #ffc1d6);
            color: #092235;
          }
          body:has(> header.hero.heroUnified) > .nav .langMenu {
            top: calc(100% + 12px) !important;
            right: 0 !important;
            width: 174px !important;
            transform-origin: top right !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-contact {
            color: #18304b !important;
            background: rgba(255,255,255,.62) !important;
            border-color: rgba(114,207,255,.26) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-contact:hover {
            color: #0b78ad !important;
            background: rgba(232,248,255,.9) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-start {
            color: #092235 !important;
            background: linear-gradient(135deg, #8ed9ff 0%, #ffc1d6 100%) !important;
            border-color: rgba(255,255,255,.7) !important;
            box-shadow: 0 18px 44px -28px rgba(114,207,255,.82), inset 0 1px 0 rgba(255,255,255,.7) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-start:hover {
            color: #071a2b !important;
            filter: saturate(1.06);
          }
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav > a.logo,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .nav-actions,
          html.fk-theme-night body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            background: rgba(255,255,255,.64) !important;
            border-color: rgba(114,207,255,.24) !important;
            color: var(--nav-ink) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:after {
            display: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            width: 56px !important;
            height: 42px !important;
            padding: 0 !important;
            display: inline-flex !important;
            align-items: center !important;
            justify-content: center !important;
            line-height: 1 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect summary {
            width: 100% !important;
            height: 100% !important;
            min-height: 0 !important;
            margin: 0 !important;
            padding: 0 !important;
            display: flex !important;
            align-items: center !important;
            justify-content: center !important;
            gap: 5px !important;
            line-height: 1 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect svg {
            display: block !important;
            flex: 0 0 auto !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langCurrent {
            flex: 0 0 auto !important;
            line-height: 1 !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            top: 12px !important;
            height: 66px !important;
            width: min(calc(100% - 32px), 1248px) !important;
            padding: 0 14px !important;
            gap: 10px !important;
            border: 1px solid rgba(125, 203, 246, .3) !important;
            border-radius: 24px !important;
            background:
              linear-gradient(135deg, rgba(250, 254, 255, .9) 0%, rgba(244, 251, 255, .88) 48%, rgba(255, 245, 250, .9) 100%) !important;
            box-shadow:
              0 24px 74px -48px rgba(44, 109, 154, .42),
              0 10px 28px -24px rgba(255, 143, 183, .32),
              inset 0 1px 0 rgba(255,255,255,.88) !important;
            backdrop-filter: blur(24px) saturate(1.16) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:before,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:after {
            display: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo {
            min-height: 44px !important;
            padding: 4px 10px 4px 3px !important;
            color: #102238 !important;
            background: transparent !important;
            border-color: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo:before {
            display: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo img {
            width: 38px !important;
            height: 38px !important;
            background: linear-gradient(135deg, #e7f8ff 0%, #fff0f6 100%) !important;
            border: 1px solid rgba(114,207,255,.34) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.9), 0 12px 26px -20px rgba(44,109,154,.46) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo:hover {
            background: rgba(255,255,255,.4) !important;
            color: #0b78ad !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-center,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions {
            padding: 0 !important;
            border: 0 !important;
            background: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-login {
            min-height: 40px !important;
            padding-inline: 12px !important;
            border-radius: 999px !important;
            color: #18304b !important;
            background: transparent !important;
            box-shadow: none !important;
            text-shadow: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link.on,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-login:hover,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger {
            color: #0b78ad !important;
            background: rgba(232,248,255,.72) !important;
            box-shadow: inset 0 0 0 1px rgba(114,207,255,.18) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            width: 56px !important;
            height: 40px !important;
            background: rgba(255,255,255,.38) !important;
            border-color: rgba(114,207,255,.24) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.76) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-contact {
            min-height: 40px !important;
            color: #18304b !important;
            background: rgba(255,255,255,.42) !important;
            border-color: rgba(114,207,255,.2) !important;
            box-shadow: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-start {
            min-height: 40px !important;
            color: #092235 !important;
            background: linear-gradient(135deg, #86d8ff 0%, #ffc1d6 100%) !important;
            border-color: rgba(255,255,255,.66) !important;
            box-shadow: 0 16px 34px -26px rgba(44,109,154,.62), inset 0 1px 0 rgba(255,255,255,.72) !important;
          }
          body:has(> header.hero.heroUnified) > .nav {
            position: fixed !important;
            top: 14px !important;
            left: 50% !important;
            right: auto !important;
            width: min(calc(100% - 48px), 1280px) !important;
            height: 66px !important;
            margin: 0 !important;
            padding: 0 14px !important;
            gap: 10px !important;
            border: 1px solid rgba(114,207,255,.32) !important;
            border-bottom: 1px solid rgba(114,207,255,.32) !important;
            border-radius: 26px !important;
            background: linear-gradient(135deg, rgba(255,255,255,.78) 0%, rgba(245,252,255,.74) 48%, rgba(255,245,250,.78) 100%) !important;
            box-shadow: 0 20px 68px -52px rgba(44,109,154,.36), inset 0 1px 0 rgba(255,255,255,.88) !important;
            color: #0d1b2d !important;
            backdrop-filter: blur(22px) saturate(1.14) !important;
            transform: translateX(-50%) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            top: 12px !important;
            width: min(calc(100% - 36px), 1248px) !important;
            height: 64px !important;
            background: linear-gradient(135deg, rgba(252,254,255,.9) 0%, rgba(244,251,255,.88) 48%, rgba(255,246,251,.9) 100%) !important;
            box-shadow: 0 26px 78px -48px rgba(44,109,154,.42), inset 0 1px 0 rgba(255,255,255,.92) !important;
          }
          body:has(> header.hero.heroUnified) > .nav:before,
          body:has(> header.hero.heroUnified) > .nav:after {
            display: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo {
            min-height: 44px !important;
            padding: 3px 10px 3px 3px !important;
            gap: 9px !important;
            border: 0 !important;
            border-radius: 999px !important;
            background: transparent !important;
            box-shadow: none !important;
            color: #0d1b2d !important;
            font-size: 30px !important;
            letter-spacing: -1.25px !important;
            text-shadow: none !important;
            backdrop-filter: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:before {
            display: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo img {
            width: 38px !important;
            height: 38px !important;
            padding: 3px !important;
            border-radius: 13px !important;
            background: linear-gradient(135deg, #e7f8ff 0%, #fff0f6 100%) !important;
            border: 1px solid rgba(114,207,255,.34) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.92), 0 10px 24px -18px rgba(44,109,154,.5) !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo:hover {
            background: rgba(255,255,255,.46) !important;
            color: #0b78ad !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-center,
          body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          body:has(> header.hero.heroUnified) > .nav .nav-actions {
            gap: 4px !important;
            padding: 0 !important;
            border: 0 !important;
            background: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          body:has(> header.hero.heroUnified) > .nav .nav-login {
            min-height: 40px !important;
            padding: 0 12px !important;
            border-radius: 999px !important;
            background: transparent !important;
            box-shadow: none !important;
            color: #18304b !important;
            text-shadow: none !important;
            font-weight: 780 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link.on,
          body:has(> header.hero.heroUnified) > .nav .nav-login:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger {
            background: rgba(232,248,255,.7) !important;
            color: #0b78ad !important;
            box-shadow: inset 0 0 0 1px rgba(114,207,255,.18) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-chevron {
            color: currentColor !important;
            opacity: .64 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-dot {
            width: 7px !important;
            height: 7px !important;
            background: #ff9fc0 !important;
            box-shadow: 0 0 0 4px rgba(255,159,192,.14) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group.is-current .nav-group-dot,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link.on .nav-group-dot {
            background: #72cfff !important;
            box-shadow: 0 0 0 4px rgba(114,207,255,.18) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            width: 56px !important;
            height: 40px !important;
            border: 1px solid rgba(114,207,255,.22) !important;
            background: rgba(255,255,255,.36) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.72) !important;
            color: #18304b !important;
          }
          body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect[open] {
            background: rgba(232,248,255,.76) !important;
            color: #0b78ad !important;
            border-color: rgba(255,159,192,.36) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-contact {
            min-height: 40px !important;
            padding: 0 15px !important;
            border-radius: 999px !important;
            color: #18304b !important;
            background: rgba(255,255,255,.38) !important;
            border: 1px solid rgba(114,207,255,.2) !important;
            box-shadow: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-contact:hover {
            background: rgba(232,248,255,.76) !important;
            color: #0b78ad !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-start {
            min-height: 42px !important;
            padding: 0 20px !important;
            border-radius: 999px !important;
            color: #092235 !important;
            background: linear-gradient(135deg, #84d7ff 0%, #ffc1d6 100%) !important;
            border: 1px solid rgba(255,255,255,.68) !important;
            box-shadow: 0 18px 38px -28px rgba(44,109,154,.62), inset 0 1px 0 rgba(255,255,255,.72) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-start:hover {
            filter: saturate(1.08) brightness(1.02);
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav {
            top: 18px !important;
            width: min(calc(100% - 48px), 1280px) !important;
            height: 64px !important;
            padding: 0 14px !important;
            border: 0 !important;
            border-bottom: 0 !important;
            border-color: transparent !important;
            border-radius: 0 !important;
            background: transparent !important;
            box-shadow: none !important;
            color: #061d35 !important;
            backdrop-filter: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:before,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:after {
            display: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-center,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions {
            border: 0 !important;
            background: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo {
            color: #061d35 !important;
            text-shadow:
              0 1px 0 rgba(255,255,255,.92),
              0 0 18px rgba(255,255,255,.82),
              0 10px 26px rgba(49,139,196,.16) !important;
            filter: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo img {
            border: 0 !important;
            background: rgba(255,255,255,.72) !important;
            box-shadow:
              0 14px 34px -24px rgba(49,139,196,.54),
              inset 0 1px 0 rgba(255,255,255,.9) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-login {
            color: #061d35 !important;
            background: transparent !important;
            border: 0 !important;
            box-shadow: none !important;
            font-weight: 860 !important;
            text-shadow:
              0 1px 0 rgba(255,255,255,.96),
              0 0 16px rgba(255,255,255,.9),
              0 9px 22px rgba(49,139,196,.18) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav a.nav-top-link.on,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-login:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger {
            color: #006ea8 !important;
            background: rgba(255,255,255,.58) !important;
            box-shadow:
              0 12px 30px -24px rgba(49,139,196,.58),
              inset 0 1px 0 rgba(255,255,255,.86) !important;
            text-shadow: none !important;
            transform: translateY(-1px);
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-dot {
            display: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            border: 0 !important;
            background: rgba(255,255,255,.62) !important;
            color: #061d35 !important;
            box-shadow:
              0 12px 30px -24px rgba(49,139,196,.52),
              inset 0 1px 0 rgba(255,255,255,.88) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .langIconSelect[open] {
            background: rgba(255,255,255,.82) !important;
            color: #006ea8 !important;
            box-shadow:
              0 16px 38px -26px rgba(49,139,196,.58),
              inset 0 1px 0 rgba(255,255,255,.92) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-contact {
            color: #061d35 !important;
            border: 0 !important;
            background: rgba(255,255,255,.62) !important;
            box-shadow:
              0 12px 30px -24px rgba(49,139,196,.52),
              inset 0 1px 0 rgba(255,255,255,.88) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-start {
            color: #061d35 !important;
            border: 0 !important;
            background: linear-gradient(135deg, rgba(113,211,255,.96) 0%, rgba(255,188,213,.96) 100%) !important;
            box-shadow:
              0 18px 42px -28px rgba(49,139,196,.62),
              inset 0 1px 0 rgba(255,255,255,.86) !important;
          }
          body:has(> header.hero.heroUnified) > .nav {
            transition:
              top .42s cubic-bezier(.16,1,.3,1),
              width .42s cubic-bezier(.16,1,.3,1),
              height .42s cubic-bezier(.16,1,.3,1),
              padding .42s cubic-bezier(.16,1,.3,1),
              border-color .42s cubic-bezier(.16,1,.3,1),
              border-radius .42s cubic-bezier(.16,1,.3,1),
              background .42s cubic-bezier(.16,1,.3,1),
              box-shadow .42s cubic-bezier(.16,1,.3,1),
              backdrop-filter .42s cubic-bezier(.16,1,.3,1),
              transform .42s cubic-bezier(.16,1,.3,1) !important;
            will-change: top, width, height, background, box-shadow, transform;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            animation: fkNavDock .46s cubic-bezier(.16,1,.3,1) both;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav {
            animation: fkNavFloat .36s cubic-bezier(.16,1,.3,1) both;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo,
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          body:has(> header.hero.heroUnified) > .nav .nav-login,
          body:has(> header.hero.heroUnified) > .nav .nav-contact,
          body:has(> header.hero.heroUnified) > .nav .nav-start,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            transition:
              transform .24s cubic-bezier(.16,1,.3,1),
              color .24s ease,
              background .24s ease,
              border-color .24s ease,
              box-shadow .24s ease,
              opacity .24s ease !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group {
            position: relative;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu {
            left: 50% !important;
            top: calc(100% + 14px) !important;
            width: 252px !important;
            padding: 9px !important;
            border: 1px solid rgba(114,207,255,.24) !important;
            border-radius: 22px !important;
            background:
              linear-gradient(135deg, rgba(255,255,255,.94) 0%, rgba(239,249,255,.92) 52%, rgba(255,242,248,.94) 100%) !important;
            box-shadow:
              0 28px 76px -44px rgba(49,139,196,.46),
              0 12px 28px -24px rgba(255,159,192,.35),
              inset 0 1px 0 rgba(255,255,255,.9) !important;
            backdrop-filter: blur(24px) saturate(1.14) !important;
            opacity: 0 !important;
            visibility: hidden !important;
            transform: translate(-50%, -8px) scale(.96) !important;
            transform-origin: top center !important;
            pointer-events: none !important;
            transition:
              opacity .22s cubic-bezier(.16,1,.3,1),
              visibility .22s ease,
              transform .22s cubic-bezier(.16,1,.3,1) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group:hover .nav-group-menu,
          body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-menu,
          body:has(> header.hero.heroUnified) > .nav .nav-group.is-open .nav-group-menu {
            opacity: 1 !important;
            visibility: visible !important;
            transform: translate(-50%, 0) scale(1) !important;
            pointer-events: auto !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu:before {
            content: "" !important;
            position: absolute !important;
            left: 0 !important;
            right: 0 !important;
            top: -14px !important;
            height: 14px !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a {
            position: relative !important;
            min-height: 44px !important;
            display: flex !important;
            align-items: center !important;
            justify-content: space-between !important;
            gap: 12px !important;
            padding: 0 13px !important;
            border-radius: 15px !important;
            color: #14304a !important;
            background: transparent !important;
            font-weight: 780 !important;
            text-decoration: none !important;
            transition:
              transform .18s cubic-bezier(.16,1,.3,1),
              background .18s ease,
              color .18s ease,
              box-shadow .18s ease !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:after {
            content: "→" !important;
            color: #57addb !important;
            font-family: var(--mono) !important;
            font-size: 12px !important;
            opacity: .68 !important;
            transition: transform .18s cubic-bezier(.16,1,.3,1), opacity .18s ease, color .18s ease !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on {
            color: #006ea8 !important;
            background: linear-gradient(135deg, rgba(232,248,255,.86), rgba(255,239,247,.72)) !important;
            box-shadow: inset 0 0 0 1px rgba(114,207,255,.2) !important;
            transform: translateX(3px) !important;
            outline: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover:after,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:focus-visible:after {
            color: #ff7fae !important;
            opacity: 1 !important;
            transform: translateX(3px) !important;
          }
          .mobileNavPanel {
            border-color: rgba(114,207,255,.24) !important;
            border-radius: 24px !important;
            background: linear-gradient(135deg, rgba(255,255,255,.96), rgba(240,250,255,.94) 50%, rgba(255,244,249,.96)) !important;
            box-shadow: 0 30px 84px -48px rgba(49,139,196,.48), inset 0 1px 0 rgba(255,255,255,.9) !important;
            animation: fkMenuPop .2s cubic-bezier(.16,1,.3,1) both !important;
          }
          .mobileNavGroup {
            border-color: rgba(114,207,255,.16) !important;
            background: rgba(255,255,255,.52) !important;
          }
          .mobileNavGroup b {
            color: #0b78ad !important;
          }
          .mobileNavGroup a:hover,
          .mobileNavGroup a.on,
          .mobileNavPrimary a:hover,
          .mobileNavPrimary a.on {
            background: linear-gradient(135deg, rgba(232,248,255,.88), rgba(255,239,247,.76)) !important;
            color: #006ea8 !important;
            transform: translateX(3px) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link {
            gap: 7px !important;
            padding-inline: 14px !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link.on,
          body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger {
            color: #075f92 !important;
            background: rgba(255,255,255,.52) !important;
            box-shadow: inset 0 0 0 1px rgba(80,162,210,.14) !important;
            text-shadow: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu {
            width: 238px !important;
            padding: 8px !important;
            border-radius: 18px !important;
            border-color: rgba(80,162,210,.2) !important;
            background: rgba(255,255,255,.96) !important;
            box-shadow:
              0 26px 70px -48px rgba(21,75,116,.42),
              inset 0 1px 0 rgba(255,255,255,.86) !important;
            backdrop-filter: blur(18px) saturate(1.06) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a {
            min-height: 42px !important;
            border-radius: 12px !important;
            color: #12263b !important;
            font-weight: 760 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:after {
            content: "" !important;
            display: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on {
            color: #063a5c !important;
            background: #eaf7ff !important;
            box-shadow: inset 0 0 0 1px rgba(80,162,210,.18) !important;
            transform: translateX(2px) !important;
          }
          @keyframes fkNavDock {
            from { opacity: .88; transform: translateX(-50%) translateY(-8px) scale(.985); }
            to { opacity: 1; transform: translateX(-50%) translateY(0) scale(1); }
          }
          @keyframes fkNavFloat {
            from { opacity: .94; transform: translateX(-50%) translateY(6px) scale(1.006); }
            to { opacity: 1; transform: translateX(-50%) translateY(0) scale(1); }
          }
          @keyframes fkMenuPop {
            from { opacity: 0; transform: translateY(-8px) scale(.97); }
            to { opacity: 1; transform: translateY(0) scale(1); }
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-dot {
            display: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          body:has(> header.hero.heroUnified) > .nav .nav-login,
          body:has(> header.hero.heroUnified) > .nav .nav-contact,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect {
            color: #061d35 !important;
            text-shadow: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link:hover,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link:focus-visible,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link.on,
          body:has(> header.hero.heroUnified) > .nav .nav-login:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-login:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-contact:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-contact:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect[open] {
            color: #063a5c !important;
            background: #ffffff !important;
            border-color: rgba(80,162,210,.26) !important;
            box-shadow:
              0 12px 26px -22px rgba(21,75,116,.3),
              inset 0 0 0 1px rgba(80,162,210,.16) !important;
            text-shadow: none !important;
            filter: none !important;
            outline: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-start:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-start:focus-visible {
            color: #061d35 !important;
            background: linear-gradient(135deg,#57c6ff 0%,#93ddff 46%,#ffb8d2 100%) !important;
            border-color: rgba(255,255,255,.88) !important;
            box-shadow:
              0 18px 38px -26px rgba(21,75,116,.46),
              inset 0 1px 0 rgba(255,255,255,.84) !important;
            filter: none !important;
            outline: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-chevron {
            opacity: .72 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-chevron,
          body:has(> header.hero.heroUnified) > .nav .nav-group:hover .nav-chevron {
            color: #063a5c !important;
            opacity: .92 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu,
          body:has(> header.hero.heroUnified) > .nav .langMenu {
            border: 1px solid rgba(80,162,210,.22) !important;
            background: rgba(255,255,255,.97) !important;
            box-shadow: 0 26px 70px -48px rgba(21,75,116,.46) !important;
            color: #061d35 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a,
          body:has(> header.hero.heroUnified) > .nav .langMenu a {
            color: #12263b !important;
            background: transparent !important;
            text-shadow: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:hover,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .langMenu a[aria-current="true"] {
            color: #063a5c !important;
            background: #e8f6ff !important;
            box-shadow: inset 0 0 0 1px rgba(80,162,210,.18) !important;
            text-shadow: none !important;
            outline: none !important;
          }
          .mobileNavTrigger:hover,
          .mobileNavMenu[open] .mobileNavTrigger,
          .mobileNavGroup a:hover,
          .mobileNavGroup a:focus-visible,
          .mobileNavGroup a.on,
          .mobileNavPrimary a:hover,
          .mobileNavPrimary a:focus-visible,
          .mobileNavPrimary a.on {
            color: #063a5c !important;
            background: #e8f6ff !important;
            border-color: rgba(80,162,210,.22) !important;
            box-shadow: inset 0 0 0 1px rgba(80,162,210,.16) !important;
            text-shadow: none !important;
            outline: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions {
            display: inline-flex !important;
            align-items: center !important;
            justify-content: flex-end !important;
            gap: 8px !important;
            height: 48px !important;
            margin-left: 10px !important;
            padding: 3px !important;
            border-radius: 999px !important;
            border: 1px solid rgba(55,148,205,.16) !important;
            background: rgba(255,255,255,.48) !important;
            box-shadow:
              0 18px 46px -38px rgba(39,121,170,.34),
              inset 0 1px 0 rgba(255,255,255,.82) !important;
            backdrop-filter: blur(16px) saturate(1.08) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login {
            height: 40px !important;
            min-height: 40px !important;
            display: inline-flex !important;
            align-items: center !important;
            justify-content: center !important;
            flex: none !important;
            border-radius: 999px !important;
            border: 1px solid rgba(55,148,205,.18) !important;
            color: #0b2740 !important;
            background: rgba(255,255,255,.72) !important;
            box-shadow: inset 0 1px 0 rgba(255,255,255,.88) !important;
            text-shadow: none !important;
            transform: translateZ(0) !important;
            transition:
              transform .2s cubic-bezier(.16,1,.3,1),
              color .2s ease,
              background .2s ease,
              border-color .2s ease,
              box-shadow .2s ease,
              filter .2s ease !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect {
            width: 42px !important;
            padding: 0 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect summary {
            width: 100% !important;
            height: 100% !important;
            display: flex !important;
            align-items: center !important;
            justify-content: center !important;
            padding: 0 !important;
            gap: 0 !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect svg {
            width: 18px !important;
            height: 18px !important;
            display: block !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langCurrent {
            display: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login {
            padding: 0 16px !important;
            font-size: 13px !important;
            font-weight: 840 !important;
            line-height: 1 !important;
            text-decoration: none !important;
            white-space: nowrap !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login {
            order: 3 !important;
            min-width: 72px !important;
            color: #061d35 !important;
            border-color: rgba(255,255,255,.86) !important;
            background: linear-gradient(135deg,#67d2ff 0%,#74e3cf 34%,#a99bff 68%,#ff96c4 100%) !important;
            box-shadow:
              0 18px 40px -30px rgba(39,121,170,.58),
              inset 0 1px 0 rgba(255,255,255,.84) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect[open] {
            color: #075f92 !important;
            border-color: rgba(255,150,196,.38) !important;
            background: rgba(255,255,255,.94) !important;
            box-shadow:
              0 16px 36px -30px rgba(39,121,170,.46),
              inset 0 1px 0 rgba(255,255,255,.92) !important;
            transform: translateY(-2px) !important;
            outline: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login:focus-visible {
            color: #061d35 !important;
            border-color: rgba(255,255,255,.92) !important;
            background: linear-gradient(135deg,#55caff 0%,#7de8d2 32%,#9f95ff 66%,#ff8fbd 100%) !important;
            box-shadow:
              0 20px 46px -28px rgba(39,121,170,.62),
              0 12px 30px -24px rgba(255,150,196,.44),
              inset 0 1px 0 rgba(255,255,255,.9) !important;
            filter: saturate(1.06) !important;
            transform: translateY(-2px) !important;
            outline: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact:active,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login:active,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect:active {
            transform: translateY(0) scale(.97) !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions {
            border-color: transparent !important;
            background: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact {
            color: #061d35 !important;
            border-color: rgba(255,255,255,.64) !important;
            background: rgba(255,255,255,.66) !important;
            box-shadow:
              0 14px 34px -28px rgba(39,121,170,.42),
              inset 0 1px 0 rgba(255,255,255,.86) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions {
            background: rgba(255,255,255,.64) !important;
            border-color: rgba(55,148,205,.2) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langMenu {
            right: 0 !important;
            top: calc(100% + 12px) !important;
            border-color: rgba(55,148,205,.22) !important;
            background: rgba(255,255,255,.97) !important;
            box-shadow:
              0 28px 76px -46px rgba(39,121,170,.46),
              0 12px 30px -26px rgba(255,150,196,.3),
              inset 0 1px 0 rgba(255,255,255,.9) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langMenu a {
            color: #12334f !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langMenu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langMenu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langMenu a[aria-current="true"] {
            color: #075f92 !important;
            background: linear-gradient(135deg,rgba(232,248,255,.92),rgba(255,239,247,.82)) !important;
            box-shadow: inset 0 0 0 1px rgba(55,148,205,.18) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            top: 12px !important;
            left: 50% !important;
            right: auto !important;
            width: min(calc(100% - 48px), 1280px) !important;
            max-width: 1280px !important;
            height: 64px !important;
            margin: 0 !important;
            padding: 0 14px !important;
            gap: 10px !important;
            border: 1px solid rgba(114,207,255,.32) !important;
            border-bottom: 1px solid rgba(114,207,255,.32) !important;
            border-radius: 26px !important;
            background:
              linear-gradient(135deg, rgba(252,254,255,.92) 0%, rgba(244,251,255,.9) 48%, rgba(255,246,251,.92) 100%) !important;
            box-shadow:
              0 26px 78px -48px rgba(44,109,154,.42),
              inset 0 1px 0 rgba(255,255,255,.92) !important;
            backdrop-filter: blur(24px) saturate(1.16) !important;
            transform: translateX(-50%) !important;
            animation: fkNavDock .46s cubic-bezier(.16,1,.3,1) both !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:before,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:after {
            display: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-center,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .desktop-nav-groups,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions {
            border-color: transparent !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:hover,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .langMenu a[aria-current="true"],
          .mobileNavGroup a:hover,
          .mobileNavGroup a:focus-visible,
          .mobileNavGroup a.on,
          .mobileNavPrimary a:hover,
          .mobileNavPrimary a:focus-visible,
          .mobileNavPrimary a.on {
            color: #08324e !important;
            -webkit-text-fill-color: #08324e !important;
            background: #eaf7ff !important;
            border-color: rgba(55,148,205,.22) !important;
            box-shadow: inset 0 0 0 1px rgba(55,148,205,.18) !important;
            text-shadow: none !important;
            filter: none !important;
            outline: none !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover *,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:focus-visible *,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on *,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:hover *,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:focus-visible *,
          body:has(> header.hero.heroUnified) > .nav .langMenu a[aria-current="true"] * {
            color: #08324e !important;
            -webkit-text-fill-color: #08324e !important;
          }
          body:has(> header.hero.heroUnified) > .nav,
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            --fk-home-gutter: clamp(14px, 2.2vw, 32px);
            top: 14px !important;
            left: 50% !important;
            right: auto !important;
            width: 100% !important;
            max-width: 1480px !important;
            height: 56px !important;
            padding-left: var(--fk-home-gutter) !important;
            padding-right: var(--fk-home-gutter) !important;
            border-radius: 18px !important;
            border: 0 !important;
            border-bottom: 0 !important;
            background: linear-gradient(135deg, rgba(252,254,255,.84) 0%, rgba(246,252,255,.82) 52%, rgba(255,247,251,.84) 100%) !important;
            box-shadow:
              0 16px 48px -38px rgba(44,109,154,.34),
              inset 0 1px 0 rgba(255,255,255,.86) !important;
            transform: translateX(-50%) !important;
            transition:
              top .34s cubic-bezier(.16,1,.3,1),
              height .34s cubic-bezier(.16,1,.3,1),
              border-color .34s ease,
              border-radius .34s cubic-bezier(.16,1,.3,1),
              background .34s ease,
              box-shadow .34s ease,
              backdrop-filter .34s ease !important;
          }
          html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav {
            border: 0 !important;
            border-bottom: 0 !important;
            background: transparent !important;
            box-shadow: none !important;
            backdrop-filter: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
            border: 1px solid rgba(114,207,255,.28) !important;
            border-bottom: 1px solid rgba(114,207,255,.28) !important;
            background: linear-gradient(135deg, rgba(252,254,255,.88) 0%, rgba(246,252,255,.86) 52%, rgba(255,247,251,.88) 100%) !important;
            box-shadow:
              0 16px 48px -38px rgba(44,109,154,.34),
              inset 0 1px 0 rgba(255,255,255,.86) !important;
            backdrop-filter: blur(22px) saturate(1.12) !important;
            animation: none !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo {
            min-height: 38px !important;
            font-size: 27px !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo img {
            width: 34px !important;
            height: 34px !important;
            border-radius: 12px !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav a.nav-top-link {
            min-height: 36px !important;
            padding-inline: 11px !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions {
            height: 42px !important;
            padding: 2px !important;
            background: rgba(255,255,255,.46) !important;
          }
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact,
          html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login {
            height: 36px !important;
            min-height: 36px !important;
          }
          body:has(> header.hero.heroUnified) > .nav > a.logo,
          body:has(> header.hero.heroUnified) > .nav > a.logo img,
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          body:has(> header.hero.heroUnified) > .nav .nav-actions,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .langIconSelect,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-contact,
          body:has(> header.hero.heroUnified) > .nav .nav-actions .nav-login {
            transition:
              min-height .34s cubic-bezier(.16,1,.3,1),
              height .34s cubic-bezier(.16,1,.3,1),
              width .34s cubic-bezier(.16,1,.3,1),
              padding .34s cubic-bezier(.16,1,.3,1),
              border-color .24s ease,
              border-radius .34s cubic-bezier(.16,1,.3,1),
              background .24s ease,
              box-shadow .24s ease,
              color .2s ease,
              transform .2s cubic-bezier(.16,1,.3,1) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-trigger,
          body:has(> header.hero.heroUnified) > .nav a.nav-top-link,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a,
          body:has(> header.hero.heroUnified) > .nav .langMenu a,
          .mobileNavGroup a,
          .mobileNavPrimary a {
            cursor: pointer !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu {
            width: 220px !important;
            padding: 7px !important;
            border-radius: 16px !important;
            border: 1px solid rgba(55,148,205,.18) !important;
            background: rgba(255,255,255,.97) !important;
            box-shadow:
              0 22px 58px -42px rgba(39,121,170,.38),
              inset 0 1px 0 rgba(255,255,255,.9) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a,
          body:has(> header.hero.heroUnified) > .nav .langMenu a {
            min-height: 38px !important;
            display: flex !important;
            align-items: center !important;
            justify-content: flex-start !important;
            gap: 8px !important;
            padding: 0 11px !important;
            border-radius: 11px !important;
            color: #18304b !important;
            -webkit-text-fill-color: #18304b !important;
            background: transparent !important;
            box-shadow: none !important;
            text-decoration: none !important;
            font-size: 13px !important;
            font-weight: 760 !important;
            line-height: 1.1 !important;
            white-space: nowrap !important;
            cursor: pointer !important;
            transition:
              transform .18s cubic-bezier(.16,1,.3,1),
              background .18s ease,
              color .18s ease,
              box-shadow .18s ease !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:hover,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .nav-group-menu a.on,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:hover,
          body:has(> header.hero.heroUnified) > .nav .langMenu a:focus-visible,
          body:has(> header.hero.heroUnified) > .nav .langMenu a[aria-current="true"] {
            color: #075f92 !important;
            -webkit-text-fill-color: #075f92 !important;
            background: #eef9ff !important;
            box-shadow: inset 0 0 0 1px rgba(55,148,205,.16) !important;
            transform: translateX(2px) !important;
          }
          @media (max-width: 900px) {
            body:has(> header.hero.heroUnified) > .nav,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
              position: fixed !important;
              top: 0 !important;
              left: 0 !important;
              right: 0 !important;
              width: 100% !important;
              height: 68px !important;
              margin: 0 !important;
              padding: 0 14px !important;
              border-radius: 0 !important;
              border: 0 !important;
              border-bottom: 1px solid rgba(4,74,113,.16) !important;
              background: rgba(247,252,255,.96) !important;
              box-shadow: 0 12px 34px -30px rgba(5,42,68,.42) !important;
              backdrop-filter: blur(18px) saturate(1.08) !important;
              animation: none !important;
              transform: none !important;
            }
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:after {
              display: none !important;
            }
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo,
            body:has(> header.hero.heroUnified) > .nav > a.logo {
              color: #061a2c !important;
              filter: none !important;
            }
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo img,
            body:has(> header.hero.heroUnified) > .nav > a.logo img {
              background: #fff !important;
              border: 1px solid rgba(4,74,113,.12) !important;
              border-radius: 12px !important;
              padding: 3px !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.88) !important;
            }
            body:has(> header.hero.heroUnified) > .nav .mobileNavTrigger {
              border-color: rgba(4,74,113,.14) !important;
              background: #fff !important;
              color: #061a2c !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.88) !important;
            }
            body:has(> header.hero.heroUnified) > .nav .mobileNavPanel {
              top: calc(100% + 10px) !important;
              border-color: rgba(4,74,113,.14) !important;
              background: rgba(255,255,255,.96) !important;
              box-shadow: 0 24px 70px -48px rgba(5,42,68,.48) !important;
            }
          }
          a.fk-click-loading,
          button.fk-click-loading {
            cursor: wait !important;
            pointer-events: none !important;
            opacity: .86 !important;
            filter: saturate(.92) !important;
          }
          a.fk-click-loading:not(.typeCard):not(.priceModelCard):not(.quoteCard):not(.faqCard):not(.whyCard):not(.logoPill)::after,
          button.fk-click-loading::after {
            content: "" !important;
            width: 14px !important;
            height: 14px !important;
            margin-left: 9px !important;
            display: inline-block !important;
            flex: 0 0 auto !important;
            border-radius: 999px !important;
            border: 2px solid currentColor !important;
            border-right-color: transparent !important;
            opacity: .82 !important;
            animation: fkButtonSpin .68s linear infinite !important;
            vertical-align: -2px !important;
          }
          a.fk-click-loading.logo::after {
            width: 12px !important;
            height: 12px !important;
            margin-left: 4px !important;
          }
          .typeCard.fk-click-loading,
          .priceModelCard.fk-click-loading,
          .logoPill.fk-click-loading {
            position: relative !important;
          }
          .typeCard.fk-click-loading::after,
          .priceModelCard.fk-click-loading::after,
          .logoPill.fk-click-loading::after {
            content: "" !important;
            position: absolute !important;
            right: 14px !important;
            top: 14px !important;
            width: 18px !important;
            height: 18px !important;
            border-radius: 999px !important;
            border: 2px solid #0877ad !important;
            border-right-color: transparent !important;
            background: rgba(255,255,255,.68) !important;
            box-shadow: 0 8px 20px -14px rgba(39,121,170,.4) !important;
            animation: fkButtonSpin .68s linear infinite !important;
          }
          @keyframes fkButtonSpin {
            to { transform: rotate(360deg); }
          }
          .fk-cursor {
            display: none;
          }
          .fk-cursor:before,
          .fk-cursor:after {
            content: "";
            position: absolute;
            border-radius: inherit;
            pointer-events: none;
          }
          .fk-cursor:before {
            inset: -8px;
            border: 1px solid rgba(255,189,90,.58);
            box-shadow: 0 0 22px rgba(255,189,90,.34);
            animation: fkCursorHalo 1.2s ease-in-out infinite alternate;
          }
          .fk-cursor:after {
            inset: 7px;
            border: 1px solid rgba(10,16,13,.72);
            box-shadow: inset 0 0 0 1px rgba(255,255,255,.86);
          }
          html.fk-cursor-live .fk-cursor {
            opacity: 1;
          }
          .fk-cursor.is-hover {
            width: 40px;
            height: 40px;
            border-color: #ffd38a;
            background:
              radial-gradient(circle at center, #0d1110 0 2px, #fff8ea 2.5px 5px, transparent 5.5px),
              radial-gradient(circle at center, rgba(255,189,90,.24), transparent 58%);
            box-shadow:
              0 0 0 1px rgba(9,13,12,.9),
              0 0 0 7px rgba(255,189,90,.2),
              0 0 34px rgba(255,189,90,.8),
              0 18px 38px rgba(9,13,12,.34);
          }
          .fk-cursor.is-down {
            width: 24px;
            height: 24px;
          }
          html.fk-theme-night .fk-cursor {
            --cursor-core: #fff8ea;
            border-color: rgba(255,248,234,.96);
            background:
              radial-gradient(circle at center, #ffcf72 0 2px, #1a120a 2.5px 4.5px, transparent 5px),
              conic-gradient(from 45deg, transparent 0 17%, rgba(255,207,114,.96) 17% 25%, transparent 25% 50%, rgba(255,126,76,.92) 50% 58%, transparent 58% 100%);
            box-shadow:
              0 0 0 1px rgba(255,248,234,.28),
              0 0 0 5px rgba(255,189,90,.16),
              0 0 30px rgba(255,189,90,.82),
              0 14px 34px rgba(0,0,0,.46);
          }
          .fk-star {
            --spark-size: 6.5px;
            --spark-color: #0877ad;
            --spark-glow: rgba(8,119,173,.68);
            --spark-duration: 1280ms;
            --spark-rotate: 0deg;
            --spark-rotate-end: 80deg;
            --tx: 0px;
            --ty: 0px;
            position: fixed;
            z-index: 9998;
            width: var(--spark-size);
            height: var(--spark-size);
            pointer-events: none;
            border-radius: 999px;
            background: radial-gradient(circle, #ffffff 0 12%, var(--spark-color) 22% 58%, transparent 74%);
            filter:
              drop-shadow(0 0 5px rgba(255,255,255,.78))
              drop-shadow(0 0 8px var(--spark-color))
              drop-shadow(0 0 16px var(--spark-glow));
            animation: fkStarFade var(--spark-duration) cubic-bezier(.12,.72,.18,1) forwards;
            will-change: transform, opacity;
          }
          .fk-star:before,
          .fk-star:after {
            content: "";
            position: absolute;
            left: 50%;
            top: 50%;
            border-radius: 999px;
            background: linear-gradient(90deg, transparent, #fff, var(--spark-color), transparent);
            pointer-events: none;
          }
          .fk-star:before {
            width: calc(var(--spark-size) * 2.45);
            height: 1px;
            transform: translate(-50%,-50%);
          }
          .fk-star:after {
            width: 1px;
            height: calc(var(--spark-size) * 2.45);
            transform: translate(-50%,-50%);
          }
          .fk-star.is-burst {
            --spark-duration: 1420ms;
            filter:
              drop-shadow(0 0 6px #fff)
              drop-shadow(0 0 12px var(--spark-color))
              drop-shadow(0 0 24px var(--spark-glow));
          }
          @keyframes fkCursorHalo {
            from { opacity: .34; transform: scale(.84); }
            to { opacity: .76; transform: scale(1.08); }
          }
          @keyframes fkStarFade {
            0% { opacity: 0; transform: translate3d(0,0,0) translate(-50%,-50%) scale(.32) rotate(var(--spark-rotate)); }
            16% { opacity: .98; transform: translate3d(0,0,0) translate(-50%,-50%) scale(1) rotate(var(--spark-rotate)); }
            64% { opacity: .68; transform: translate3d(calc(var(--tx) * .42), calc(var(--ty) * .42),0) translate(-50%,-50%) scale(.86) rotate(var(--spark-rotate)); }
            100% { opacity: 0; transform: translate3d(var(--tx), var(--ty),0) translate(-50%,-50%) scale(.18) rotate(var(--spark-rotate-end)); }
          }
          @media (pointer: fine) {
            .fk-star {
              transform-origin: center;
            }
          }
          @media (max-width: 900px) {
            .nav-center {
              display: none;
            }
            .mobileNavMenu {
              display: block;
              margin-left: auto;
            }
            .nav-actions {
              display: none !important;
              margin-left: 0;
            }
            .nav-contact {
              display: none !important;
            }
            .nav-actions .nav-login,
            .nav-actions .langIconSelect,
            .nav > .themeToggle {
              display: none !important;
            }
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
              top: 8px !important;
              left: 50% !important;
              width: 100% !important;
              max-width: 1480px !important;
              height: 54px !important;
              margin: 0 !important;
              border-radius: 16px !important;
              transform: translateX(-50%) !important;
            }
            body:has(> header.hero.heroUnified) > .nav,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
              --fk-home-gutter: 14px;
              top: 8px !important;
              width: 100% !important;
              max-width: 1480px !important;
              padding-left: var(--fk-home-gutter) !important;
              padding-right: var(--fk-home-gutter) !important;
            }
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo {
              font-size: 28px !important;
            }
          }
          @media (max-width: 620px) {
            .nav-start {
              min-height: 38px;
              padding: 8px 13px !important;
              font-size: 12px !important;
            }
            .mobileNavTrigger span {
              display: none;
            }
            .mobileNavTrigger {
              width: 40px;
              justify-content: center;
              padding: 0;
            }
            .mobileNavPanel {
              right: -78px;
              width: min(92vw, 340px);
            }
          }
          @media (max-width: 900px) {
            .desktop-nav-groups,
            .nav-center,
            .nav-actions {
              display: none !important;
            }
            .mobileNavActions {
              display: inline-flex !important;
              margin-left: auto !important;
            }
            .mobileNavMenu {
              display: block !important;
              margin-left: 0 !important;
            }
            body:has(> header.hero.heroUnified) > .nav,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav {
              position: fixed !important;
              top: 0 !important;
              left: 0 !important;
              right: 0 !important;
              width: 100% !important;
              max-width: none !important;
              height: 64px !important;
              margin: 0 !important;
              padding: 0 14px !important;
              gap: 10px !important;
              border: 0 !important;
              border-bottom: 1px solid rgba(4,74,113,.16) !important;
              border-radius: 0 !important;
              background: rgba(247,252,255,.97) !important;
              box-shadow: 0 12px 32px -28px rgba(5,42,68,.34) !important;
              color: #061a2c !important;
              backdrop-filter: blur(18px) saturate(1.08) !important;
              transform: none !important;
              animation: none !important;
            }
            body:has(> header.hero.heroUnified) > .nav:before,
            body:has(> header.hero.heroUnified) > .nav:after,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:before,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav:after,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:before,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav:after {
              display: none !important;
            }
            body:has(> header.hero.heroUnified) > .nav > a.logo,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo {
              min-height: 42px !important;
              padding: 3px 8px 3px 2px !important;
              border: 0 !important;
              background: transparent !important;
              box-shadow: none !important;
              color: #061a2c !important;
              filter: none !important;
              text-shadow: none !important;
              backdrop-filter: none !important;
              font-size: 28px !important;
              transform: none !important;
            }
            body:has(> header.hero.heroUnified) > .nav > a.logo img,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav > a.logo img,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav > a.logo img {
              width: 36px !important;
              height: 36px !important;
              padding: 3px !important;
              border: 1px solid rgba(4,74,113,.12) !important;
              border-radius: 12px !important;
              background: #fff !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.88) !important;
            }
            body:has(> header.hero.heroUnified) > .nav .sp {
              display: none !important;
            }
            .mobileNavLogin,
            body:has(> header.hero.heroUnified) > .nav .mobileNavLogin {
              height: 40px !important;
              min-height: 40px !important;
              min-width: 58px !important;
              padding: 0 13px !important;
              border: 1px solid rgba(4,74,113,.14) !important;
              background: #fff !important;
              color: #061a2c !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.9) !important;
              text-shadow: none !important;
            }
            .mobileNavTrigger,
            body:has(> header.hero.heroUnified) > .nav .mobileNavTrigger {
              width: 40px !important;
              height: 40px !important;
              min-height: 40px !important;
              justify-content: center !important;
              padding: 0 !important;
              border: 1px solid rgba(4,74,113,.14) !important;
              border-radius: 999px !important;
              background: #fff !important;
              color: #061a2c !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.9) !important;
              text-shadow: none !important;
            }
            .mobileNavTrigger span {
              display: none !important;
            }
            .mobileNavPanel,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPanel {
              position: fixed !important;
              top: 74px !important;
              left: 12px !important;
              right: 12px !important;
              z-index: 180 !important;
              width: auto !important;
              max-height: calc(100dvh - 90px) !important;
              overflow: auto !important;
              display: grid !important;
              gap: 10px !important;
              padding: 12px !important;
              border: 1px solid rgba(4,74,113,.14) !important;
              border-radius: 20px !important;
              background: rgba(255,255,255,.98) !important;
              box-shadow: 0 28px 76px -48px rgba(5,42,68,.48) !important;
              backdrop-filter: blur(18px) saturate(1.08) !important;
            }
            .mobileNavPanel:before,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPanel:before {
              content: "Menu" !important;
              display: block !important;
              padding: 2px 4px 4px !important;
              color: #075f92 !important;
              -webkit-text-fill-color: #075f92 !important;
              font: 900 12px/1 var(--mono) !important;
              letter-spacing: .08em !important;
              text-transform: uppercase !important;
            }
            .mobileNavGroup,
            .mobileNavUtility {
              border-color: rgba(4,74,113,.1) !important;
              background: rgba(247,252,255,.72) !important;
            }
            .mobileNavPanel,
            .mobileNavPanel *,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPanel,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPanel * {
              text-shadow: none !important;
              -webkit-text-fill-color: currentColor !important;
            }
            .mobileNavGroup b,
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup b {
              color: #075f92 !important;
              -webkit-text-fill-color: #075f92 !important;
            }
            .mobileNavGroup a,
            .mobileNavPrimary a,
            .mobileNavSales,
            .mobileNavUtility .langIconSelect,
            .mobileNavUtility .langIconSelect summary,
            .mobileNavUtility .langCurrent,
            .mobileNavUtility .langMenu a,
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup a,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPrimary a,
            body:has(> header.hero.heroUnified) > .nav .mobileNavSales,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility .langIconSelect,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility .langIconSelect summary,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility .langCurrent,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility .langMenu a {
              color: #061a2c !important;
              -webkit-text-fill-color: #061a2c !important;
              opacity: 1 !important;
            }
            .mobileNavGroup a,
            .mobileNavPrimary a,
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup a,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPrimary a {
              background: rgba(255,255,255,.82) !important;
              border: 1px solid rgba(4,74,113,.08) !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.9) !important;
            }
            .mobileNavGroup,
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup {
              gap: 6px !important;
              padding: 10px !important;
              border-radius: 18px !important;
              background: #f7fbff !important;
            }
            .mobileNavGroup a,
            .mobileNavPrimary a,
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup a,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPrimary a {
              min-height: 44px !important;
              justify-content: space-between !important;
              padding: 0 13px !important;
              border-radius: 14px !important;
              font-size: 14px !important;
              font-weight: 780 !important;
            }
            .mobileNavPrimary,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPrimary {
              grid-template-columns: 1fr !important;
            }
            .mobileNavUtility,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility {
              grid-template-columns: 1fr !important;
              gap: 8px !important;
              padding: 10px !important;
              border-radius: 18px !important;
              background: #f7fbff !important;
            }
            .mobileNavSales,
            .mobileLangOpen,
            body:has(> header.hero.heroUnified) > .nav .mobileNavSales,
            body:has(> header.hero.heroUnified) > .nav .mobileLangOpen {
              min-height: 46px !important;
              border-radius: 14px !important;
              border: 1px solid rgba(4,74,113,.1) !important;
              background: #fff !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.9) !important;
            }
            .mobileNavSales,
            body:has(> header.hero.heroUnified) > .nav .mobileNavSales {
              justify-content: flex-start !important;
              padding: 0 13px !important;
              font-size: 14px !important;
              font-weight: 820 !important;
            }
            .mobileLangPage,
            body:has(> header.hero.heroUnified) > .nav .mobileLangPage {
              position: static !important;
            }
            .mobileLangOpen,
            body:has(> header.hero.heroUnified) > .nav .mobileLangOpen {
              display: flex !important;
              align-items: center !important;
              justify-content: space-between !important;
              padding: 0 12px !important;
              color: #061a2c !important;
              -webkit-text-fill-color: #061a2c !important;
            }
            .mobileLangOpen span,
            body:has(> header.hero.heroUnified) > .nav .mobileLangOpen span {
              display: inline-flex !important;
              align-items: center !important;
              gap: 9px !important;
              min-width: 0 !important;
            }
            .mobileLangOpen b,
            body:has(> header.hero.heroUnified) > .nav .mobileLangOpen b {
              color: #061a2c !important;
              -webkit-text-fill-color: #061a2c !important;
              font-size: 14px !important;
            }
            .mobileLangOpen small,
            body:has(> header.hero.heroUnified) > .nav .mobileLangOpen small {
              color: #587086 !important;
              -webkit-text-fill-color: #587086 !important;
            }
            .mobileLangOpen svg,
            body:has(> header.hero.heroUnified) > .nav .mobileLangOpen svg {
              width: 18px !important;
              height: 18px !important;
              flex: none !important;
            }
            .mobileLangPage[open] .mobileLangPanel,
            body:has(> header.hero.heroUnified) > .nav .mobileLangPage[open] .mobileLangPanel {
              position: fixed !important;
              top: 74px !important;
              left: 12px !important;
              right: 12px !important;
              z-index: 190 !important;
              max-height: calc(100dvh - 90px) !important;
              overflow: auto !important;
              display: grid !important;
              gap: 10px !important;
              padding: 12px !important;
              border: 1px solid rgba(4,74,113,.14) !important;
              border-radius: 20px !important;
              background: rgba(255,255,255,.99) !important;
              box-shadow: 0 28px 76px -48px rgba(5,42,68,.48) !important;
              backdrop-filter: blur(18px) saturate(1.08) !important;
              animation: langMenuIn .16s ease both !important;
            }
            .mobileLangBack,
            body:has(> header.hero.heroUnified) > .nav .mobileLangBack {
              min-height: 40px !important;
              justify-self: start !important;
              padding: 0 12px !important;
              border: 1px solid rgba(4,74,113,.1) !important;
              border-radius: 999px !important;
              background: #f0f8ff !important;
              color: #075f92 !important;
              -webkit-text-fill-color: #075f92 !important;
              font: 850 12px/1 var(--mono) !important;
            }
            .mobileLangTitle,
            body:has(> header.hero.heroUnified) > .nav .mobileLangTitle {
              padding: 2px 4px 4px !important;
              color: #061a2c !important;
              -webkit-text-fill-color: #061a2c !important;
              font-size: 18px !important;
              font-weight: 900 !important;
            }
            .mobileLangList,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList {
              display: grid !important;
              gap: 8px !important;
            }
            .mobileLangList a,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList a {
              min-height: 44px !important;
              display: flex !important;
              align-items: center !important;
              justify-content: space-between !important;
              padding: 0 13px !important;
              border: 1px solid rgba(4,74,113,.08) !important;
              border-radius: 14px !important;
              background: #f7fbff !important;
              color: #061a2c !important;
              -webkit-text-fill-color: #061a2c !important;
              text-decoration: none !important;
              font-weight: 780 !important;
            }
            .mobileLangList a[aria-current="true"],
            .mobileLangList a:hover,
            .mobileLangList a:focus-visible,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList a[aria-current="true"],
            body:has(> header.hero.heroUnified) > .nav .mobileLangList a:hover,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList a:focus-visible {
              border-color: rgba(4,74,113,.18) !important;
              background: #e8f6ff !important;
              color: #075f92 !important;
              -webkit-text-fill-color: #075f92 !important;
              outline: none !important;
            }
            .mobileLangList a[aria-current="true"]:after,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList a[aria-current="true"]:after {
              content: "Current" !important;
              color: #075f92 !important;
              -webkit-text-fill-color: #075f92 !important;
              font: 850 10px/1 var(--mono) !important;
            }
            .mobileNavGroup a:hover,
            .mobileNavGroup a:focus-visible,
            .mobileNavGroup a.on,
            .mobileNavPrimary a:hover,
            .mobileNavPrimary a:focus-visible,
            .mobileNavPrimary a.on,
            .mobileNavUtility .langMenu a:hover,
            .mobileNavUtility .langMenu a:focus-visible,
            .mobileNavUtility .langMenu a[aria-current="true"],
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup a:hover,
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup a:focus-visible,
            body:has(> header.hero.heroUnified) > .nav .mobileNavGroup a.on,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPrimary a:hover,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPrimary a:focus-visible,
            body:has(> header.hero.heroUnified) > .nav .mobileNavPrimary a.on,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility .langMenu a:hover,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility .langMenu a:focus-visible,
            body:has(> header.hero.heroUnified) > .nav .mobileNavUtility .langMenu a[aria-current="true"] {
              color: #075f92 !important;
              -webkit-text-fill-color: #075f92 !important;
              background: #e8f6ff !important;
              border-color: rgba(4,74,113,.14) !important;
              outline: none !important;
            }
            .mobileNavUtility .langIconSelect {
              width: 48px !important;
              height: 42px !important;
              display: inline-grid !important;
              background: #fff !important;
              border-color: rgba(4,74,113,.14) !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.9) !important;
            }
            .mobileNavUtility .langMenu {
              position: absolute !important;
              right: 0 !important;
              top: calc(100% + 8px) !important;
              width: 176px !important;
              background: #fff !important;
              border-color: rgba(4,74,113,.14) !important;
              box-shadow: 0 22px 62px -42px rgba(5,42,68,.42) !important;
            }
            .mobileNavSales {
              color: #061a2c !important;
              background: #fff !important;
              border-color: rgba(4,74,113,.12) !important;
            }
            .mobileLangPage[open] .mobileLangOpen,
            body:has(> header.hero.heroUnified) > .nav .mobileLangPage[open] .mobileLangOpen {
              border-color: rgba(4,74,113,.18) !important;
              background: #e8f6ff !important;
            }
            .mobileLangPage[open] .mobileLangOpen > svg,
            body:has(> header.hero.heroUnified) > .nav .mobileLangPage[open] .mobileLangOpen > svg {
              transform: rotate(180deg) !important;
            }
            .mobileLangPage .mobileLangPanel,
            body:has(> header.hero.heroUnified) > .nav .mobileLangPage .mobileLangPanel {
              display: none !important;
            }
            .mobileLangPage[open] .mobileLangPanel,
            body:has(> header.hero.heroUnified) > .nav .mobileLangPage[open] .mobileLangPanel {
              position: static !important;
              z-index: auto !important;
              max-height: none !important;
              overflow: visible !important;
              display: block !important;
              margin-top: 8px !important;
              padding: 10px !important;
              border: 1px solid rgba(4,74,113,.1) !important;
              border-radius: 16px !important;
              background: rgba(255,255,255,.72) !important;
              box-shadow: inset 0 1px 0 rgba(255,255,255,.9) !important;
              backdrop-filter: none !important;
              animation: langMenuIn .14s ease both !important;
            }
            .mobileLangList,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList {
              display: grid !important;
              grid-template-columns: 1fr 1fr !important;
              gap: 8px !important;
            }
            .mobileLangList a,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList a {
              min-height: 40px !important;
              justify-content: center !important;
              padding: 0 10px !important;
              font-size: 12px !important;
              text-align: center !important;
            }
            .mobileLangList a[aria-current="true"]:after,
            body:has(> header.hero.heroUnified) > .nav .mobileLangList a[aria-current="true"]:after {
              content: "" !important;
              width: 6px !important;
              height: 6px !important;
              margin-left: 6px !important;
              border-radius: 999px !important;
              background: #075f92 !important;
            }
            .nav.pricing-nav > .nav-actions,
            body:has(> header.hero.heroUnified) > .nav.pricing-nav > .nav-actions,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav.pricing-nav > .nav-actions,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav.pricing-nav > .nav-actions {
              display: none !important;
              visibility: hidden !important;
              pointer-events: none !important;
              position: absolute !important;
              width: 0 !important;
              height: 0 !important;
              overflow: hidden !important;
              margin: 0 !important;
              padding: 0 !important;
              border: 0 !important;
            }
            .nav.pricing-nav > .nav-actions > *,
            body:has(> header.hero.heroUnified) > .nav.pricing-nav > .nav-actions > * {
              display: none !important;
            }
            .nav.pricing-nav > .mobileNavActions,
            body:has(> header.hero.heroUnified) > .nav.pricing-nav > .mobileNavActions,
            html:not(.fk-nav-scrolled) body:has(> header.hero.heroUnified) > .nav.pricing-nav > .mobileNavActions,
            html.fk-nav-scrolled body:has(> header.hero.heroUnified) > .nav.pricing-nav > .mobileNavActions {
              display: inline-flex !important;
              visibility: visible !important;
              pointer-events: auto !important;
              position: relative !important;
              width: auto !important;
              height: auto !important;
              overflow: visible !important;
            }
          }
          .langIconSelect button,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect button {
            width: 100% !important;
            height: 100% !important;
            display: grid !important;
            place-items: center !important;
            border: 0 !important;
            border-radius: inherit !important;
            background: transparent !important;
            color: inherit !important;
            cursor: default !important;
            padding: 0 !important;
            font: inherit !important;
          }
          .langIconSelect .langMenu,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect .langMenu {
            opacity: 0 !important;
            visibility: hidden !important;
            pointer-events: none !important;
            transform: translateY(-4px) scale(.98) !important;
            transition: opacity .14s ease, visibility .14s ease, transform .14s ease !important;
          }
          .langIconSelect:hover .langMenu,
          body:has(> header.hero.heroUnified) > .nav .langIconSelect:hover .langMenu {
            opacity: 1 !important;
            visibility: visible !important;
            pointer-events: auto !important;
            transform: translateY(0) scale(1) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group:focus-within .nav-group-menu,
          body:has(> header.hero.heroUnified) > .nav .nav-group.is-open .nav-group-menu {
            opacity: 0 !important;
            visibility: hidden !important;
            pointer-events: none !important;
            transform: translateY(8px) scale(.98) !important;
          }
          body:has(> header.hero.heroUnified) > .nav .nav-group:hover .nav-group-menu {
            opacity: 1 !important;
            visibility: visible !important;
            pointer-events: auto !important;
            transform: translateY(0) scale(1) !important;
          }
        `}
      </style>
      <script
        dangerouslySetInnerHTML={{
          __html: `(() => {
            const root = document.documentElement;
            try {
              root.classList.remove('fk-theme-night');
              window.localStorage.removeItem('fk-theme');
            } catch {}
            const update = () => root.classList.toggle('fk-nav-scrolled', window.scrollY > 24);
            update();
            window.addEventListener('scroll', update, { passive: true });
            const boot = () => {
              const clearClickLoading = () => {
                document.querySelectorAll('.fk-click-loading[aria-busy="true"]').forEach((item) => {
                  item.classList.remove('fk-click-loading');
                  item.removeAttribute('aria-busy');
                });
              };
              const shouldShowClickLoading = (event, anchor) => {
                if (!anchor || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return false;
                if (anchor.hasAttribute('download') || anchor.target === '_blank') return false;
                const rawHref = anchor.getAttribute('href') || '';
                if (!rawHref || rawHref.startsWith('#') || /^(mailto|tel|javascript):/i.test(rawHref)) return false;
                try {
                  const url = new URL(anchor.href, window.location.href);
                  if (url.pathname === window.location.pathname && url.search === window.location.search && url.hash) return false;
                } catch {}
                return true;
              };
              document.addEventListener('click', (event) => {
                const target = event.target instanceof Element ? event.target : null;
                const anchor = target?.closest('a[href]');
                if (anchor && shouldShowClickLoading(event, anchor)) {
                  clearClickLoading();
                  anchor.classList.add('fk-click-loading');
                  anchor.setAttribute('aria-busy', 'true');
                  window.setTimeout(clearClickLoading, 4500);
                }
                if (!target?.closest('.langIconSelect')) {
                  document.querySelectorAll('.langIconSelect[open]').forEach((item) => item.removeAttribute('open'));
                }
                if (!target?.closest('.mobileNavMenu')) {
                  document.querySelectorAll('.mobileNavMenu[open]').forEach((item) => item.removeAttribute('open'));
                }
              });
              window.addEventListener('pageshow', clearClickLoading);
              window.addEventListener('popstate', clearClickLoading);
              document.addEventListener('visibilitychange', () => {
                if (!document.hidden) clearClickLoading();
              });

              const setupReveal = () => {
                if (document.documentElement.classList.contains('fk-reveal-ready')) return;
                document.documentElement.classList.add('fk-reveal-ready');
                const isHomePage = Boolean(document.querySelector('body > header.hero.heroUnified'));
                const revealItems = isHomePage
                  ? Array.from(document.body.children).filter((item) => item.matches('header.hero.heroUnified, section'))
                  : [];
                revealItems.forEach((item) => item.classList.add('fk-reveal'));
                if ('IntersectionObserver' in window) {
                  const observer = new IntersectionObserver((entries) => {
                    entries.forEach((entry) => {
                      if (entry.isIntersecting) {
                        entry.target.classList.add('fk-in');
                        observer.unobserve(entry.target);
                      }
                    });
                  }, { rootMargin: '0px 0px -12% 0px', threshold: 0.12 });
                  revealItems.forEach((item) => observer.observe(item));
                } else {
                  revealItems.forEach((item) => item.classList.add('fk-in'));
                }
              };
              const scheduleReveal = () => window.setTimeout(setupReveal, 1200);
              if (document.readyState === 'complete') scheduleReveal();
              else window.addEventListener('load', scheduleReveal, { once: true });

              const finePointer = window.matchMedia?.('(pointer: fine)').matches;
              const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
              if (!finePointer || reducedMotion) return;
              const sparkColors = ['#0877ad', '#0a9c86', '#2469d9', '#7568e8', '#d73679'];
              let x = -80;
              let y = -80;
              let hasPointer = false;
              let lastStar = 0;
              let lastX = x;
              let lastY = y;
              const makeStar = (sx, sy, burst = false) => {
                const star = document.createElement('i');
                star.className = burst ? 'fk-star is-burst' : 'fk-star';
                const angle = Math.random() * Math.PI * 2;
                const distance = burst ? 18 + Math.random() * 18 : 10 + Math.random() * 22;
                const size = burst ? 6.5 + Math.random() * 3.2 : 4.6 + Math.random() * 3.4;
                const rotate = Math.round(Math.random() * 180);
                const color = sparkColors[Math.floor(Math.random() * sparkColors.length)];
                star.style.left = sx + 'px';
                star.style.top = sy + 'px';
                star.style.setProperty('--spark-size', size.toFixed(1) + 'px');
                star.style.setProperty('--spark-color', color);
                star.style.setProperty('--spark-glow', color);
                star.style.setProperty('--spark-duration', (burst ? 1250 + Math.random() * 300 : 1080 + Math.random() * 420).toFixed(0) + 'ms');
                star.style.setProperty('--spark-rotate', rotate + 'deg');
                star.style.setProperty('--spark-rotate-end', (rotate + 70 + Math.round(Math.random() * 45)) + 'deg');
                star.style.setProperty('--tx', (Math.cos(angle) * distance).toFixed(1) + 'px');
                star.style.setProperty('--ty', (Math.sin(angle) * distance).toFixed(1) + 'px');
                document.body.appendChild(star);
                window.setTimeout(() => star.remove(), burst ? 1800 : 1700);
              };
              window.addEventListener('mousemove', (event) => {
                x = event.clientX;
                y = event.clientY;
                if (!hasPointer) {
                  hasPointer = true;
                  lastX = x;
                  lastY = y;
                  return;
                }
                const now = performance.now();
                const moved = Math.hypot(x - lastX, y - lastY);
                if (now - lastStar > 46 && moved > 9) {
                  lastStar = now;
                  const count = moved > 58 ? 2 : 1;
                  for (let i = 0; i < count; i += 1) {
                    const t = (i + 0.35 + Math.random() * 0.45) / count;
                    const sx = x - (x - lastX) * t + (Math.random() * 4 - 2);
                    const sy = y - (y - lastY) * t + (Math.random() * 4 - 2);
                    makeStar(sx, sy);
                  }
                  lastX = x;
                  lastY = y;
                }
              }, { passive: true });
              window.addEventListener('mousedown', (event) => {
                x = event.clientX;
                y = event.clientY;
                for (let i = 0; i < 3; i += 1) makeStar(x, y, true);
              }, { passive: true });
            };
            if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot, { once: true });
            else boot();
          })();`,
        }}
      />
      <OnlineNav active={props.active} contactAction={props.contactAction} locale={props.locale} pathname={props.pathname} />
      {props.children}
      <OnlineFooter locale={props.locale} />
    </>
  );
}
