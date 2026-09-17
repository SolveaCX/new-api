import Link from "next/link";
import { ArrowLeft, ArrowRight, CheckCircle2 } from "lucide-react";
import { SiteShell } from "@/components/site-shell";
import { localizePath, type Locale } from "@/lib/locales";

type CareersLocale = "en" | "zh";
const APPLY_EMAIL = "support@flatkey.ai";

type RoleCopy = {
  title: string; department: string; location: string; employment: string; locationType: string; summary: string;
  aboutCompany: string[]; responsibilitiesTitle: string; responsibilities: string[]; requirementsTitle: string; requirements: string[];
  preferredTitle: string; preferred: string[]; offerTitle: string; offer: string[]; applyTitle: string; applyBody: string;
};

const roleCopy: Record<CareersLocale, RoleCopy> = {
  en: {
    title: "Business Development Representative", department: "Sales", location: "San Jose, CA", employment: "Full time", locationType: "Hybrid · field-heavy",
    summary: "Own Bay Area growth for an AI infrastructure platform used by the teams building what comes next.",
    aboutCompany: ["Flatkey is a fast-growing AI infrastructure company building a unified AI API gateway. Our platform brings 200+ large language and multimodal models behind one API key, with intelligent routing, automatic failover, unified billing, usage monitoring, and observability.", "We process more than ten billion tokens each month for AI companies and agent builders across North America. This role is focused on the local Bay Area market: building relationships with Silicon Valley startups, enterprise R&D teams, and the technical leaders who are bringing AI into production."],
    responsibilitiesTitle: "What you'll do", responsibilities: ["Own Bay Area market development: prospect AI startups, agent teams, enterprise AI groups, and their technical decision-makers.", "Run technical discovery, demos, and proof-of-concept validation. Explain routing, cost optimization, failover, and unified billing in terms customers can act on.", "Stay close after launch: help unblock integrations, improve production usage, uncover expansion opportunities, and reduce churn.", "Bring back local market intelligence, competitive signals, and customer needs to the product team.", "Maintain pipeline and business metrics, and represent Flatkey at local AI and technology events."],
    requirementsTitle: "What we're looking for", requirements: ["A bachelor's degree in computer science or equivalent project experience; comfortable with LLM APIs, HTTP interfaces, and independently running a PoC.", "Fluent English and the confidence to meet Silicon Valley founders and engineers face to face.", "A self-directed operator who does not wait for inbound leads: you create momentum, own the follow-through, and actively control close timelines.", "Ability to work from our San Jose office in a hybrid arrangement, with regular in-person customer meetings across the Bay Area."],
    preferredTitle: "High-priority experience", preferred: ["Existing relationships across Bay Area technology and AI companies.", "Experience selling to CTOs, engineering leaders, or startup founders.", "Experience with agents, LangChain, AI platforms, developer tools, or infrastructure.", "Bilingual English and Mandarin."],
    offerTitle: "What we offer", offer: ["Competitive base salary plus uncapped performance commission.", "Equity package for high-impact individual contributors."], applyTitle: "How to apply?", applyBody: "Please send your resume to support@flatkey.ai. We'll contact you shortly.",
  },
  zh: {
    title: "商务拓展代表", department: "商务拓展", location: "San Jose, CA", employment: "全职", locationType: "混合办公 · 以现场为主", summary: "负责 AI 基础设施平台在旧金山湾区的增长，与正在构建下一代产品的团队建立关系。",
    aboutCompany: ["Flatkey.ai 是一家位于硅谷 San Jose 的 AI 基础设施公司，正在打造统一 AI API 网关。一个密钥即可调用 200+ 大模型和多模态模型，并获得智能路由、自动容灾、统一账单、用量监控与可观测性。", "平台每月处理超过百亿 Token，服务北美大量 AI 企业与 Agent 开发团队。本岗位负责美国湾区本地业务，对接硅谷 AI 初创、企业研发团队与推动 AI 落地的技术决策者。"],
    responsibilitiesTitle: "岗位职责", responsibilities: ["全权负责旧金山湾区市场拓展，挖掘 AI 初创、Agent 团队、企业 AI 研发部门客户，对接技术决策者。", "完成技术售前、Demo 演示、PoC 验证，讲解模型路由、成本优化、容灾降级、统一计费等方案，协助客户完成集成。", "客户上线后做好技术侧客户成功：排障调优，推动用量增长，挖掘增购机会，降低流失。", "收集湾区市场情报、竞品动态、客户需求，反馈产品团队。", "维护业务指标与管线数据，参与本地 AI 社区活动。"],
    requirementsTitle: "任职要求", requirements: ["CS 相关本科或同等项目经验，熟悉 LLM API、HTTP 接口，能独立跑通 PoC。", "流利英文，可面对面沟通硅谷创始人与工程师。", "使命感强，不被动等待客户，主动出击、积极把控 close deal 时间。", "需要在 San Jose 办公室办公，支持混合办公模式，并能在湾区进行客户拜访。"],
    preferredTitle: "优先考虑", preferred: ["已有硅谷湾区人脉。", "有 Agent / LangChain 销售经验。", "有 AI、API 平台、开发者工具或基础设施销售经验。", "中英双语。"],
    offerTitle: "我们提供", offer: ["有竞争力的基本薪资 + 不设上限的绩效佣金。", "面向高影响力个人贡献者的股权激励。"], applyTitle: "如何申请？", applyBody: "请将简历发送至 support@flatkey.ai，我们会尽快与您联系。",
  },
};

function copyFor(locale: Locale): RoleCopy { return roleCopy[locale === "zh" ? "zh" : "en"]; }
function Pill({ children }: { children: React.ReactNode }) { return <span className="rounded-full border border-violet-200 bg-violet-50 px-3 py-1.5 text-xs font-semibold text-violet-700">{children}</span>; }

function DetailList({ items }: { items: string[] }) { return <ul className="space-y-4">{items.map((item) => <li key={item} className="flex gap-3 leading-7 text-[#555560]"><CheckCircle2 className="mt-1 h-5 w-5 shrink-0 text-[#7028F5]" />{item}</li>)}</ul>; }

export function CareersDetailPage({ locale, pathname }: { locale: Locale; pathname: string }) {
  const t = copyFor(locale); const isZh = locale === "zh";
  return <SiteShell locale={locale} pathname={pathname}><main className="bg-white px-6 pb-24 pt-28 text-[#111114] sm:pt-36"><div className="mx-auto max-w-6xl"><Link href={localizePath("/careers", locale)} className="inline-flex items-center gap-2 text-sm font-semibold text-[#7028F5] hover:underline"><ArrowLeft className="h-4 w-4" />{isZh ? "返回职位列表" : "Back to careers"}</Link><div className="mt-10 grid gap-14 lg:grid-cols-[260px_1fr] lg:gap-20">
    <aside className="h-fit rounded-2xl border border-[#0B0B0F12] bg-[#fafafa] p-6 lg:sticky lg:top-28"><div className="space-y-5 text-sm"><div><p className="font-semibold text-[#83838E]">{isZh ? "地点" : "Location"}</p><p className="mt-1">{t.location}</p></div><div className="border-t border-[#0B0B0F12] pt-5"><p className="font-semibold text-[#83838E]">{isZh ? "工作类型" : "Employment type"}</p><p className="mt-1">{t.employment}</p></div><div className="border-t border-[#0B0B0F12] pt-5"><p className="font-semibold text-[#83838E]">{isZh ? "办公模式" : "Location type"}</p><p className="mt-1">{t.locationType}</p></div><div className="border-t border-[#0B0B0F12] pt-5"><p className="font-semibold text-[#83838E]">{isZh ? "部门" : "Department"}</p><p className="mt-1">{t.department}</p></div></div></aside>
    <article className="max-w-3xl"><div className="flex flex-wrap gap-2"><Pill>{isZh ? "正在招聘" : "Hiring"}</Pill><Pill>{t.locationType}</Pill></div><h1 className="mt-6 text-4xl font-bold tracking-tight sm:text-5xl">{t.title}</h1><p className="mt-5 text-xl leading-8 text-[#555560]">{t.summary}</p><div className="mt-12 space-y-12"><section><h2 className="mb-5 text-2xl font-bold">{isZh ? "公司简介" : "About Flatkey"}</h2>{t.aboutCompany.map((p) => <p key={p} className="mb-4 leading-7 text-[#555560]">{p}</p>)}</section><section><h2 className="mb-5 text-2xl font-bold">{t.responsibilitiesTitle}</h2><DetailList items={t.responsibilities} /></section><section><h2 className="mb-5 text-2xl font-bold">{t.requirementsTitle}</h2><DetailList items={t.requirements} /></section><section><h2 className="mb-5 text-2xl font-bold">{t.preferredTitle}</h2><DetailList items={t.preferred} /></section><section><h2 className="mb-5 text-2xl font-bold">{t.offerTitle}</h2><DetailList items={t.offer} /></section><section className="relative overflow-hidden rounded-3xl border border-violet-200 bg-[linear-gradient(135deg,#f8f5ff_0%,#eee7ff_100%)] p-7 sm:p-9"><div className="absolute -right-12 -top-16 h-40 w-40 rounded-full bg-violet-200/50 blur-2xl" /><div className="relative"><p className="text-xs font-bold uppercase tracking-[0.18em] text-violet-700">{isZh ? "加入 Flatkey" : "Join Flatkey"}</p><h2 className="mt-3 text-2xl font-bold">{t.applyTitle}</h2><p className="mt-3 max-w-xl leading-7 text-[#4e3a78]">{t.applyBody}</p><div className="mt-6 flex flex-wrap items-center gap-3"><a href={`mailto:${APPLY_EMAIL}`} className="inline-flex items-center gap-2 rounded-xl bg-[#7028F5] px-5 py-3 text-sm font-semibold !text-white shadow-[0_8px_18px_rgba(112,40,245,0.22)] transition hover:bg-[#5f1ddd]">{isZh ? "发送简历" : "Email your resume"}<ArrowRight className="h-4 w-4 text-white" /></a><a href={`mailto:${APPLY_EMAIL}`} className="text-sm font-semibold text-violet-800 underline decoration-violet-300 underline-offset-4 hover:text-violet-950">{APPLY_EMAIL}</a></div></div></section></div></article>
  </div></div></main></SiteShell>;
}
