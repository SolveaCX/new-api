import { notFound } from "next/navigation";
import { OnlineHomePage } from "@/components/online-home-page";
import { DEFAULT_LOCALE, isLocale, LOCALES } from "@/lib/locales";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { buildMetadata } from "@/lib/seo";

type Props = {
  params: Promise<{ locale: string }>;
};

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== DEFAULT_LOCALE).map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale)) return {};
  const zh = params.locale === "zh";
  return buildMetadata({
    title: zh ? "flatkey — 一个 Key 接入多模态 AI 模型" : "flatkey - One key for multimodal AI models",
    description: zh
      ? "flatkey 用一个 Key 接入文本、图像、视频、音频模型，统一路由到 GPT、Claude、Gemini、DeepSeek、Qwen、GLM、Seedance 等官方模型端点。"
      : "flatkey routes text, image, video, and audio requests to official GPT, Claude, Gemini, DeepSeek, Qwen, GLM, Seedance, and other frontier models with one key.",
    pathname: "/",
    locale: params.locale,
  });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === DEFAULT_LOCALE) notFound();
  const pricingData = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  return <OnlineHomePage locale={params.locale} pricingData={pricingData} />;
}
