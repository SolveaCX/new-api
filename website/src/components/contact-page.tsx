import { CheckCircle2 } from "lucide-react";
import { FlatkeyTallyEmbed } from "@/components/flatkey-tally-embed";
import { HomeSupport } from "@/components/home-support";
import { SiteShell } from "@/components/site-shell";
import { getHomeCopy } from "@/lib/home-copy";
import { withIdFallback, type Locale } from "@/lib/locales";

type Props = {
  locale: Locale;
};

type ContactEnterpriseCopy = {
  eyebrow: string;
  title: string;
  description: string;
  bullets: string[];
  formTitle: string;
  formDescription: string;
};

const CONTACT_ENTERPRISE_COPY: Record<Locale, ContactEnterpriseCopy> = withIdFallback({
  en: {
    eyebrow: "Enterprise inquiry",
    title: "Tell us about your high-volume API needs.",
    description:
      "Use this form for enterprise contracts, invoices, procurement review, custom routing discounts, and SLA support.",
    bullets: ["Higher monthly usage", "Team procurement and invoicing", "Custom routing discounts"],
    formTitle: "Enterprise sales form",
    formDescription: "Share your company, expected volume, and requirements. Our team will follow up.",
  },
  zh: {
    eyebrow: "大客户咨询",
    title: "填写企业需求，我们会跟进方案。",
    description: "如需企业合同、发票、采购流程、定制路由折扣或 SLA 支持，请通过表单提交。",
    bullets: ["更高月度用量", "团队采购与发票支持", "定制路由折扣"],
    formTitle: "企业销售表单",
    formDescription: "留下公司、预计用量和具体需求，我们会尽快跟进。",
  },
  es: {
    eyebrow: "Consulta enterprise",
    title: "Cuéntanos tus necesidades API de alto volumen.",
    description:
      "Usa este formulario para contratos enterprise, facturas, compras, descuentos de routing personalizados y soporte SLA.",
    bullets: ["Mayor uso mensual", "Compras de equipo y facturación", "Descuentos de routing personalizados"],
    formTitle: "Formulario de ventas enterprise",
    formDescription: "Comparte empresa, volumen esperado y requisitos. El equipo responderá.",
  },
  fr: {
    eyebrow: "Demande entreprise",
    title: "Décrivez vos besoins API à fort volume.",
    description:
      "Utilisez ce formulaire pour contrats entreprise, factures, achats, remises de routage personnalisées et support SLA.",
    bullets: ["Usage mensuel plus élevé", "Achats d'équipe et facturation", "Remises de routage personnalisées"],
    formTitle: "Formulaire commercial entreprise",
    formDescription: "Indiquez entreprise, volume prévu et besoins. Notre équipe vous répondra.",
  },
  pt: {
    eyebrow: "Consulta enterprise",
    title: "Conte suas necessidades de API em alto volume.",
    description:
      "Use este formulário para contratos enterprise, faturas, compras, descontos personalizados de roteamento e suporte SLA.",
    bullets: ["Maior uso mensal", "Compras de equipe e faturamento", "Descontos personalizados de roteamento"],
    formTitle: "Formulário de vendas enterprise",
    formDescription: "Informe empresa, volume esperado e requisitos. Nossa equipe responderá.",
  },
  ru: {
    eyebrow: "Enterprise-заявка",
    title: "Расскажите о высоком объеме API.",
    description:
      "Форма подходит для enterprise-контрактов, счетов, procurement, кастомных routing-скидок и SLA-поддержки.",
    bullets: ["Больший месячный объем", "Procurement и счета для команды", "Кастомные routing-скидки"],
    formTitle: "Форма для enterprise-sales",
    formDescription: "Укажите компанию, ожидаемый объем и требования. Мы свяжемся с вами.",
  },
  ja: {
    eyebrow: "エンタープライズ相談",
    title: "高ボリューム API の要件をお知らせください。",
    description:
      "企業契約、請求書、購買対応、カスタム routing 割引、SLA サポートが必要な場合はこちらのフォームをご利用ください。",
    bullets: ["より高い月間利用量", "チーム購買と請求書対応", "カスタム routing 割引"],
    formTitle: "エンタープライズ営業フォーム",
    formDescription: "会社名、想定ボリューム、要件を共有してください。担当チームが連絡します。",
  },
  vi: {
    eyebrow: "Tư vấn enterprise",
    title: "Cho chúng tôi biết nhu cầu API volume lớn.",
    description:
      "Dùng form này cho hợp đồng enterprise, hóa đơn, procurement, giảm giá routing tùy chỉnh và hỗ trợ SLA.",
    bullets: ["Usage tháng cao hơn", "Procurement nhóm và hóa đơn", "Giảm giá routing tùy chỉnh"],
    formTitle: "Form sales enterprise",
    formDescription: "Chia sẻ công ty, volume dự kiến và yêu cầu. Đội ngũ sẽ phản hồi.",
  },
  de: {
    eyebrow: "Enterprise-Anfrage",
    title: "Beschreibe deinen API-Bedarf mit hohem Volumen.",
    description:
      "Nutze dieses Formular für Enterprise-Verträge, Rechnungen, Procurement, individuelle Routing-Rabatte und SLA-Support.",
    bullets: ["Höhere Monatsnutzung", "Team-Procurement und Rechnungen", "Individuelle Routing-Rabatte"],
    formTitle: "Enterprise-Sales-Formular",
    formDescription: "Teile Unternehmen, erwartetes Volumen und Anforderungen. Unser Team meldet sich.",
  },
});

// Dedicated contact page: enterprise inquiry form first, then support channels.
export function ContactPage(props: Props) {
  const home = getHomeCopy(props.locale);
  const enterprise = CONTACT_ENTERPRISE_COPY[props.locale] ?? CONTACT_ENTERPRISE_COPY.en;

  return (
    <SiteShell locale={props.locale} pathname="/contact">
      <main className="relative overflow-x-hidden bg-[#F7F4EC] pt-[var(--fk-header-safe-area)] text-[#101014] dark:bg-[#050507] dark:text-[#F6F3EA]">
        <section id="enterprise" className="relative z-10 border-b-2 border-[#101014] px-4 py-12 sm:px-6 md:py-16 dark:border-white/20">
          <div aria-hidden className="fk-hero-grid absolute inset-0 opacity-55" />
          <div className="relative mx-auto grid max-w-[2160px] gap-8 lg:grid-cols-[0.82fr_1.18fr] lg:items-start">
            <div className="lg:sticky lg:top-28">
              <p className="mb-3 font-mono text-xs font-black uppercase text-[#7C3AED] dark:text-[#C8A8FF]">{enterprise.eyebrow}</p>
              <h1 className="max-w-2xl text-4xl leading-[1.02] font-black tracking-normal text-balance md:text-6xl">
                {enterprise.title}
              </h1>
              <p className="mt-5 max-w-xl text-base leading-7 font-semibold text-[#5C5861] dark:text-white/62">
                {enterprise.description}
              </p>
              <div className="mt-7 grid gap-3">
                {enterprise.bullets.map((bullet) => (
                  <p key={bullet} className="flex items-center gap-2 text-sm font-bold text-[#34343C] dark:text-white/74">
                    <CheckCircle2 className="size-4 shrink-0 text-[#7C3AED] dark:text-[#C8A8FF]" />
                    {bullet}
                  </p>
                ))}
              </div>
            </div>
            <div className="rounded-[1.35rem] border-2 border-[#101014] bg-[#FFFDF6]/94 p-4 shadow-[7px_7px_0_#101014] backdrop-blur-sm sm:p-5 dark:border-white/24 dark:bg-[#111116]/88 dark:shadow-[7px_7px_0_rgba(255,255,255,0.16)]">
              <div className="mb-4">
                <h2 className="text-xl font-black tracking-normal">{enterprise.formTitle}</h2>
                <p className="mt-2 text-sm leading-6 font-semibold text-[#5C5861] dark:text-white/62">{enterprise.formDescription}</p>
              </div>
              <FlatkeyTallyEmbed
                locale={props.locale}
                loading="eager"
                className="rounded-[1rem] border-2 border-[#101014]/12 bg-white/70 p-2 dark:border-white/14 dark:bg-white/[0.06]"
                iframeClassName="block h-[760px] min-h-[620px] w-full border-0 bg-transparent sm:h-[680px] lg:h-[720px]"
              />
            </div>
          </div>
        </section>
        <HomeSupport copy={home.support} />
      </main>
    </SiteShell>
  );
}
