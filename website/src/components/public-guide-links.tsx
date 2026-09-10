import Link from "next/link";
import type { Locale } from "@/lib/locales";

const paths = ["/gateway", "/openai-compatible", "/gpt-api-alternative", "/chinese-ai", "/tools/web-scraping-api", "/tools/google-search-api", "/apify-alternative"];
// These guides are intentionally single-language routes. Never prefix their
// paths with the current locale; visible language labels set expectations.
const copy: Record<Locale, { heading: string; labels: string[] }> = {
  en: { heading: "Integration guides and regional payment options", labels: ["AI API gateway", "OpenAI-compatible API", "GPT API alternative", "Chinese AI models", "Web scraping API", "Search API", "Apify alternative"] },
  zh: { heading: "接入指南与地区支付方式", labels: ["AI API 网关", "兼容 OpenAI 的 API", "GPT API 替代方案", "中国 AI 模型", "网页抓取 API", "搜索 API", "Apify 替代方案"] },
  es: { heading: "Guías de integración y opciones de pago regionales", labels: ["Gateway de API de IA", "API compatible con OpenAI", "Alternativa a la API de GPT", "Modelos de IA chinos", "API de extracción web", "API de búsqueda", "Alternativa a Apify"] },
  fr: { heading: "Guides d’intégration et moyens de paiement régionaux", labels: ["Passerelle API IA", "API compatible avec OpenAI", "Alternative à l’API GPT", "Modèles d’IA chinois", "API d’extraction web", "API de recherche", "Alternative à Apify"] },
  pt: { heading: "Guias de integração e opções de pagamento regionais", labels: ["Gateway de API de IA", "API compatível com OpenAI", "Alternativa à API do GPT", "Modelos chineses de IA", "API de extração web", "API de pesquisa", "Alternativa ao Apify"] },
  ru: { heading: "Руководства по интеграции и региональные способы оплаты", labels: ["Шлюз API ИИ", "API, совместимый с OpenAI", "Альтернатива API GPT", "Китайские модели ИИ", "API веб-скрейпинга", "API поиска", "Альтернатива Apify"] },
  ja: { heading: "導入ガイドと地域別の支払い方法", labels: ["AI API ゲートウェイ", "OpenAI 互換 API", "GPT API の代替", "中国の AI モデル", "ウェブスクレイピング API", "検索 API", "Apify の代替"] },
  vi: { heading: "Hướng dẫn tích hợp và phương thức thanh toán theo khu vực", labels: ["Cổng API AI", "API tương thích OpenAI", "Giải pháp thay thế API GPT", "Mô hình AI Trung Quốc", "API trích xuất web", "API tìm kiếm", "Giải pháp thay thế Apify"] },
  de: { heading: "Integrationsleitfäden und regionale Zahlungsmöglichkeiten", labels: ["KI-API-Gateway", "OpenAI-kompatible API", "Alternative zur GPT-API", "Chinesische KI-Modelle", "Web-Scraping-API", "Such-API", "Alternative zu Apify"] },
  id: { heading: "Panduan integrasi dan pilihan pembayaran regional", labels: ["Gateway API AI", "API yang kompatibel dengan OpenAI", "Alternatif API GPT", "Model AI Tiongkok", "API pengambilan data web", "API pencarian", "Alternatif Apify"] },
};
const regional = [
  { href: "/br", label: "Brasil · Pix", language: "Português", languageTag: "pt-BR" },
  { href: "/in", label: "India · UPI", language: "English", languageTag: "en" },
  { href: "/id-market", label: "Indonesia · QRIS", language: "Bahasa Indonesia", languageTag: "id" },
  { href: "/pt/5-credit-promo", label: "Créditos de boas-vindas", language: "Português", languageTag: "pt-BR" },
];

export function PublicGuideLinks({ locale }: { locale: Locale }) {
  const content = copy[locale];
  const links = [
    ...paths.map((href, index) => ({ href, label: content.labels[index], language: "English", languageTag: "en" })),
    ...regional,
  ];
  return (
    <section className="px-[var(--fk-site-gutter)] pb-16" aria-labelledby="public-guides-heading">
      <div className="mx-auto max-w-[var(--fk-site-max-width)]">
        <h2 id="public-guides-heading" className="mb-6 text-2xl font-bold">{content.heading}</h2>
        <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {links.map((link) => (
            <li key={link.href}>
              <Link href={link.href} hrefLang={link.languageTag} className="flex h-full items-center justify-between gap-3 rounded-lg border border-[#0B0B0F14] bg-white px-4 py-3 text-sm hover:border-violet-400">
                <span className="font-semibold">{link.label}</span>
                <span className="shrink-0 text-xs text-[#666672]">{link.language}</span>
              </Link>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
