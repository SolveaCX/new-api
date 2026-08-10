"use client";

import Link from "next/link";
import { ArrowRight, Mail, MessageCircle } from "lucide-react";
import { FlatkeyBrandLogo } from "@/components/flatkey-brand-logo";
import { useSiteConfig } from "@/components/site-config-provider";
import { getCopy } from "@/lib/copy";
import { type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type SiteFooterProps = {
  locale: Locale;
};

type FooterLink = {
  external?: boolean;
  href: string;
  label: string;
};

const footerLabels = withIdFallback({
  en: {
    product: "Product",
    developers: "Developers",
    company: "Company",
    tools: "Tools",
    playground: "Playground",
    compute: "Compute",
    usecases: "Use cases",
    status: "API status",
    careers: "Careers",
    about: "About",
    contact: "Talk to sales",
    contactChannels: "Contact channels",
    contactDesc: "Enterprise contracts, invoices, model governance, procurement and SLA support.",
    trusted: "TRUSTED & VERIFIED BY",
    zeroRetention: "Zero retention of request content",
  },
  zh: {
    product: "产品",
    developers: "开发者",
    company: "公司",
    tools: "工具",
    playground: "Playground",
    compute: "算力",
    usecases: "使用场景",
    status: "服务状态",
    careers: "加入我们",
    about: "关于我们",
    contact: "联系销售",
    contactChannels: "联系方式",
    contactDesc: "企业合同、发票、模型治理、采购流程和 SLA 支持。",
    trusted: "可信与认证",
    zeroRetention: "不保留请求内容",
  },
  es: {
    product: "Producto",
    developers: "Desarrolladores",
    company: "Empresa",
    tools: "Herramientas",
    playground: "Playground",
    compute: "Compute",
    usecases: "Casos de uso",
    status: "Estado de API",
    careers: "Carreras",
    about: "Acerca de",
    contact: "Contactar ventas",
    contactChannels: "Canales de contacto",
    contactDesc: "Contratos enterprise, facturas, gobierno de modelos, procurement y SLA.",
    trusted: "CONFIANZA Y VERIFICACIÓN",
    zeroRetention: "Cero retención del contenido de solicitudes",
  },
  fr: {
    product: "Produit",
    developers: "Développeurs",
    company: "Entreprise",
    tools: "Outils",
    playground: "Playground",
    compute: "Compute",
    usecases: "Cas d'usage",
    status: "Statut API",
    careers: "Carrières",
    about: "À propos",
    contact: "Contacter l'équipe commerciale",
    contactChannels: "Canaux de contact",
    contactDesc: "Contrats enterprise, factures, gouvernance modèles, achats et SLA.",
    trusted: "CONFIANCE ET VÉRIFICATION",
    zeroRetention: "Aucune rétention du contenu des requêtes",
  },
  pt: {
    product: "Produto",
    developers: "Desenvolvedores",
    company: "Empresa",
    tools: "Ferramentas",
    playground: "Playground",
    compute: "Compute",
    usecases: "Casos de uso",
    status: "Status da API",
    careers: "Carreiras",
    about: "Sobre",
    contact: "Falar com vendas",
    contactChannels: "Canais de contato",
    contactDesc: "Contratos enterprise, faturas, governança de modelos, procurement e SLA.",
    trusted: "CONFIÁVEL E VERIFICADO",
    zeroRetention: "Zero retenção do conteúdo das solicitações",
  },
  ru: {
    product: "Продукт",
    developers: "Разработчикам",
    company: "Компания",
    tools: "Инструменты",
    playground: "Playground",
    compute: "Compute",
    usecases: "Сценарии",
    status: "Статус API",
    careers: "Вакансии",
    about: "О нас",
    contact: "Связаться с продажами",
    contactChannels: "Каналы связи",
    contactDesc: "Enterprise-контракты, счета, управление моделями, закупки и SLA.",
    trusted: "ДОВЕРИЕ И ПРОВЕРКА",
    zeroRetention: "Нулевое хранение содержимого запросов",
  },
  ja: {
    product: "プロダクト",
    developers: "開発者向け",
    company: "会社",
    tools: "ツール",
    playground: "Playground",
    compute: "Compute",
    usecases: "ユースケース",
    status: "API ステータス",
    careers: "採用情報",
    about: "会社概要",
    contact: "営業に相談",
    contactChannels: "お問い合わせ先",
    contactDesc: "Enterprise 契約、請求書、モデル管理、購買、SLA サポート。",
    trusted: "信頼と認証",
    zeroRetention: "リクエスト内容を保持しません",
  },
  vi: {
    product: "Sản phẩm",
    developers: "Nhà phát triển",
    company: "Công ty",
    tools: "Tools",
    playground: "Playground",
    compute: "Compute",
    usecases: "Use cases",
    status: "Trạng thái API",
    careers: "Tuyển dụng",
    about: "Giới thiệu",
    contact: "Liên hệ sales",
    contactChannels: "Kênh liên hệ",
    contactDesc: "Hợp đồng enterprise, hóa đơn, quản trị model, procurement và SLA.",
    trusted: "ĐÁNG TIN CẬY & ĐÃ XÁC MINH",
    zeroRetention: "Không lưu nội dung yêu cầu",
  },
  de: {
    product: "Produkt",
    developers: "Entwickler",
    company: "Unternehmen",
    tools: "Tools",
    playground: "Playground",
    compute: "Compute",
    usecases: "Anwendungsfälle",
    status: "API-Status",
    careers: "Karriere",
    about: "Über uns",
    contact: "Sales kontaktieren",
    contactChannels: "Kontaktkanäle",
    contactDesc: "Enterprise-Verträge, Rechnungen, Modell-Governance, Procurement und SLA.",
    trusted: "VERTRAUEN & VERIFIZIERUNG",
    zeroRetention: "Keine Speicherung von Anfrageinhalten",
  },
});

function FooterColumn(props: { links: FooterLink[]; locale: Locale; title: string }) {
  return (
    <div className="min-w-0">
      <h3 className="mb-3.5 font-mono text-[11px] font-extrabold uppercase text-[#7C3AED]">{props.title}</h3>
      <div className="grid gap-[10px]">
        {props.links.map((link) =>
          link.external || link.href.startsWith("mailto:") ? (
            <a
              key={link.href}
              href={link.href}
              target={link.external ? "_blank" : undefined}
              rel={link.external ? "noopener noreferrer" : undefined}
              className="fk-footer-link w-fit text-[15px] leading-6 font-bold text-[#101014] no-underline hover:text-[#2F2AAE] dark:text-white/78 dark:hover:text-white"
            >
              {link.label}
            </a>
          ) : (
            <Link
              key={link.href}
              href={link.href === "/llms.txt" ? link.href : localizePath(link.href, props.locale)}
              className="fk-footer-link w-fit text-[15px] leading-6 font-bold text-[#101014] no-underline hover:text-[#2F2AAE] dark:text-white/78 dark:hover:text-white"
            >
              {link.label}
            </Link>
          )
        )}
      </div>
    </div>
  );
}

export function SiteFooter(props: SiteFooterProps) {
  const { docsUrl } = useSiteConfig();
  const siteCopy = getCopy(props.locale);
  const copy = siteCopy.footer;
  const labels = footerLabels[props.locale] ?? footerLabels.en;
  const currentYear = new Date().getFullYear();
  const productLinks: FooterLink[] = [
    { href: "/models", label: siteCopy.nav.modelPricing },
    { href: "/tools", label: labels.tools },
    { href: "/playground", label: labels.playground },
    { href: "/rankings", label: siteCopy.nav.rankings },
    { href: "/pricing", label: siteCopy.nav.pricing },
    { href: "/compute", label: labels.compute },
    { href: "/usecases", label: labels.usecases },
  ];
  const developerLinks: FooterLink[] = [
    { href: "/cli", label: "CLI" },
    ...(docsUrl ? [{ href: docsUrl, label: siteCopy.nav.docs, external: true }] : []),
    { href: "/status", label: labels.status },
    { href: "/llms.txt", label: "llms.txt" },
    { href: consoleUrl("/dashboard"), label: `${siteCopy.nav.console} ↗`, external: true },
  ];
  const companyLinks: FooterLink[] = [
    { href: "/careers", label: labels.careers },
    { href: "/contact", label: labels.contact },
    { href: "/about", label: labels.about },
    { href: "/blog", label: `${siteCopy.nav.blog} ↗` },
    { href: "/terms", label: copy.termsOfService },
    { href: "/privacy", label: copy.privacyPolicy },
    { href: "/sla", label: copy.serviceLevelAgreement },
    { href: "/refund-policy", label: copy.refundPolicy },
  ];

  return (
    <footer className="fk-site-footer relative overflow-hidden border-t-2 border-[#101014] bg-[#F7F4EC] text-[#101014] dark:border-white/18 dark:bg-[#050507] dark:text-[#F6F3EA]">
      <div aria-hidden className="fk-hero-grid absolute inset-0 opacity-35" />
      <div className="relative mx-auto max-w-[2160px] px-5 py-12 sm:px-6 lg:px-8 xl:px-10">
        <div className="grid gap-10 lg:grid-cols-[1.25fr_repeat(3,minmax(0,0.72fr))]">
          <div>
            <Link href={localizePath("/", props.locale)} className="inline-flex items-center">
              <FlatkeyBrandLogo className="[&_[data-flatkey-wordmark='true']]:text-[32px]" />
              <span className="sr-only">flatkey.ai</span>
            </Link>
            <p className="mt-4 max-w-[330px] text-[15px] leading-7 font-semibold text-[#575762] dark:text-white/62">{copy.tagline}</p>
            <a
              href="mailto:support@flatkey.ai"
              className="mt-5 inline-flex items-center gap-2 rounded-full border-2 border-[#101014] bg-[#FFFDF6] px-4 py-2 text-sm font-extrabold shadow-[3px_3px_0_#101014] hover:bg-[#F9F871] dark:border-white/18 dark:bg-white/8 dark:shadow-[3px_3px_0_rgba(255,255,255,.16)]"
            >
              <Mail className="size-4" />
              support@flatkey.ai
            </a>
          </div>
          <FooterColumn title={labels.product} links={productLinks} locale={props.locale} />
          <FooterColumn title={labels.developers} links={developerLinks} locale={props.locale} />
          <FooterColumn title={labels.company} links={companyLinks} locale={props.locale} />
        </div>

        <div className="mt-10 flex flex-col gap-4 border-t-2 border-[#101014] pt-6 dark:border-white/16 xl:flex-row xl:items-center xl:justify-between">
          <div className="max-w-2xl">
            <h3 className="font-mono text-[11px] font-extrabold uppercase text-[#7C3AED]">{labels.contactChannels}</h3>
            <p className="mt-2 text-sm leading-6 font-semibold text-[#666672] dark:text-white/60">{labels.contactDesc}</p>
          </div>
          <div className="flex flex-wrap gap-2 xl:justify-end">
            <a href="mailto:support@flatkey.ai" className="fk-button-motion inline-flex h-10 items-center gap-2 rounded-full border border-[#101014]/14 bg-white px-4 text-sm font-extrabold text-[#101014] hover:border-[#101014] hover:bg-[#F9F871] dark:border-white/14 dark:bg-white/8 dark:text-white dark:hover:bg-white/14">
              <Mail className="size-4" />
              support@flatkey.ai
            </a>
            <a href="https://discord.gg/VrbZFDXj5g" target="_blank" rel="noopener noreferrer" className="fk-button-motion inline-flex h-10 items-center gap-2 rounded-full border border-[#101014]/14 bg-white px-4 text-sm font-extrabold text-[#101014] hover:border-[#101014] hover:bg-[#F9F871] dark:border-white/14 dark:bg-white/8 dark:text-white dark:hover:bg-white/14">
              <MessageCircle className="size-4" />
              Discord
            </a>
            <a href="https://www.linkedin.com/company/flatkey/" target="_blank" rel="noopener noreferrer" className="fk-button-motion inline-flex h-10 items-center gap-2 rounded-full border border-[#101014]/14 bg-white px-4 text-sm font-extrabold text-[#101014] hover:border-[#101014] hover:bg-[#F9F871] dark:border-white/14 dark:bg-white/8 dark:text-white dark:hover:bg-white/14">
              LinkedIn
              <ArrowRight className="size-4 -rotate-45" />
            </a>
            <a href="https://x.com/flatkey101" target="_blank" rel="noopener noreferrer" className="fk-button-motion inline-flex h-10 items-center gap-2 rounded-full border border-[#101014]/14 bg-white px-4 text-sm font-extrabold text-[#101014] hover:border-[#101014] hover:bg-[#F9F871] dark:border-white/14 dark:bg-white/8 dark:text-white dark:hover:bg-white/14">
              X @flatkey101
              <ArrowRight className="size-4 -rotate-45" />
            </a>
            <Link href={localizePath("/contact", props.locale)} className="fk-button-motion inline-flex h-10 items-center gap-2 rounded-full bg-[#101014] px-4 text-sm font-extrabold !text-white hover:bg-[#5852FF] dark:bg-white dark:!text-[#101014]">
              {labels.contact}
              <ArrowRight className="size-4" />
            </Link>
          </div>
        </div>
      </div>

      <div className="relative mx-auto flex max-w-[2160px] flex-wrap items-center gap-2 border-t-2 border-[#101014] px-5 py-5 sm:px-6 lg:px-8 xl:px-10 dark:border-white/16">
        <span className="mr-2 font-mono text-[10.5px] font-extrabold text-[#83838E]">{labels.trusted}</span>
        {["SOC 2 Type II", "ISO 27001", "GDPR compliant", "Vanta monitored", labels.zeroRetention].map((badge) => (
          <span key={badge} className="rounded-full border border-[#101014]/12 bg-white/78 px-3 py-1.5 text-[12.5px] font-bold text-[#43434C] dark:border-white/12 dark:bg-white/8 dark:text-white/62">
            {badge}
          </span>
        ))}
      </div>

      <div className="relative mx-auto flex max-w-[2160px] flex-col gap-3 px-5 pb-8 text-sm leading-7 font-semibold text-[#575762] sm:px-6 lg:flex-row lg:items-center lg:justify-between lg:px-8 xl:px-10 dark:text-white/58">
        <p>© {currentYear} flatkey.ai · VOC AI INC, San Jose, CA. {copy.defaultCopyright}</p>
        <div className="flex flex-wrap gap-x-4 gap-y-2">
          <Link className="underline underline-offset-4" href={localizePath("/terms", props.locale)}>Terms</Link>
          <Link className="underline underline-offset-4" href={localizePath("/privacy", props.locale)}>Privacy</Link>
          <Link className="underline underline-offset-4" href={localizePath("/sla", props.locale)}>SLA</Link>
          <Link className="underline underline-offset-4" href={localizePath("/refund-policy", props.locale)}>Refunds</Link>
        </div>
      </div>
    </footer>
  );
}
