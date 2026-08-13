import { PlaygroundPromptsExplorer } from "@/components/playground-prompts-explorer";
import { type Locale, withIdFallback } from "@/lib/locales";
import { getPlaygroundPromptItems } from "@/lib/playground-prompts";
import { OnlineStaticShell } from "./online-static-shell";

export const PLAYGROUND_PROMPTS_PATH = "/playground";

const metadataByLocale: Record<Locale, { description: string; title: string }> = withIdFallback({
  en: {
    description: "Browse Flatkey's image and video prompt library by collection, media type, model, and keyword. Every featured prompt is paired with a visible output.",
    title: "Flatkey AI Prompt Library - image and video prompts",
  },
  zh: {
    description: "按合集、媒介、模型和关键词浏览 Flatkey 图片与视频提示词库。精选提示词都配有可见产物。",
    title: "Flatkey AI 提示词资料库 - 图片与视频提示词",
  },
  es: {
    description: "Explora la biblioteca de prompts de imagen y video de Flatkey por colección, medio, modelo y palabra clave.",
    title: "Biblioteca de prompts IA de Flatkey - imagen y video",
  },
  fr: {
    description: "Explorez la bibliothèque de prompts image et vidéo de Flatkey par collection, média, modèle et mot-clé.",
    title: "Bibliothèque de prompts IA Flatkey - image et vidéo",
  },
  pt: {
    description: "Explore a biblioteca de prompts de imagem e vídeo da Flatkey por coleção, mídia, modelo e palavra-chave.",
    title: "Biblioteca de prompts IA da Flatkey - imagem e vídeo",
  },
  ru: {
    description: "Просматривайте image и video prompt library Flatkey по коллекциям, media type, model и keywords.",
    title: "Flatkey AI Prompt Library - image и video prompts",
  },
  ja: {
    description: "Flatkey の画像・動画プロンプトライブラリを、コレクション、メディア、モデル、キーワード別に閲覧できます。",
    title: "Flatkey AI プロンプトライブラリ - 画像と動画",
  },
  vi: {
    description: "Duyệt thư viện prompt hình ảnh và video của Flatkey theo bộ sưu tập, media, model và keyword.",
    title: "Thư viện prompt AI Flatkey - hình ảnh và video",
  },
  de: {
    description: "Durchsuche Flatkeys Bild- und Video-Prompt-Library nach Collection, Medium, Modell und Keyword.",
    title: "Flatkey AI Prompt Library - Bild- und Video-Prompts",
  },
});

export function getPlaygroundPromptsMetadata(locale: Locale) {
  const copy = metadataByLocale[locale] ?? metadataByLocale.en;
  return {
    title: copy.title,
    description: copy.description,
    pathname: PLAYGROUND_PROMPTS_PATH,
  };
}

export async function PlaygroundPromptsPage(props: { locale: Locale }) {
  const items = await getPlaygroundPromptItems();

  return (
    <OnlineStaticShell locale={props.locale} pathname={PLAYGROUND_PROMPTS_PATH}>
      <PlaygroundPromptsExplorer items={items} locale={props.locale} />
    </OnlineStaticShell>
  );
}
