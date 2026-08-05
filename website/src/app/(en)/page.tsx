import { OnlineHomePage } from "@/components/online-home-page";
import { getPricingData, WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";
import { buildMetadata } from "@/lib/seo";

export const metadata = buildMetadata({
  title: "flatkey - One key for multimodal AI models",
  description:
    "flatkey routes text, image, video, and audio requests to official GPT, Claude, Gemini, DeepSeek, Qwen, GLM, Seedance, and other frontier models with one key.",
  pathname: "/",
});

export default async function Page() {
  const pricingData = await getPricingData(WEBSITE_PUBLIC_PRICING_GROUP);
  return <OnlineHomePage locale="en" pricingData={pricingData} />;
}
