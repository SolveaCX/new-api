import { notFound } from "next/navigation";
import { ModelCollectionsIndex } from "@/components/model-collections-page";
import { isLocale, type Locale, LOCALES } from "@/lib/locales";
import { buildMetadata } from "@/lib/seo";

type Props = { params: Promise<{ locale: string }> };

export function generateStaticParams() {
  return LOCALES.filter((locale) => locale !== "en").map((locale) => ({ locale }));
}

export async function generateMetadata(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") return {};
  const title = params.locale === "zh" ? "AI 模型集合 | Flatkey" : "AI Model Collections | Flatkey";
  const description = params.locale === "zh" ? "浏览适合编程、图像生成、视频和工具调用的 AI 模型集合。" : "Browse curated AI model collections for coding, image generation, video, tool calling, and more.";
  return buildMetadata({ title, description, pathname: "/collections", locale: params.locale as Locale, absoluteTitle: true });
}

export default async function Page(props: Props) {
  const params = await props.params;
  if (!isLocale(params.locale) || params.locale === "en") notFound();
  return <ModelCollectionsIndex locale={params.locale as Locale} />;
}
