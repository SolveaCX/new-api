import type { Metadata } from "next";
import type { Locale } from "./locales";
import { limitSeoDescription, limitSeoTitle } from "./model-landing";
import { SITE_ORIGIN } from "./origins";
import type { PromptDetail } from "./prompt-detail-data";
import { buildMetadata } from "./seo";

type SeoCopy = {
  image: string;
  video: string;
  description: (model: string, label: string, kind: string) => string;
};

const SEO_COPY: Record<Locale, SeoCopy> = {
  en: { image: "Image Prompt", video: "Video Prompt", description: (model, label, kind) => `Explore the ${model} ${kind.toLowerCase()} for ${label}: a matching example, the full English prompt and generation settings. Edit it and create your version in Flatkey Playground.` },
  zh: { image: "图片提示词", video: "视频提示词", description: (model, label, kind) => `查看 ${model} 的${label}${kind}：生成示例、完整英文提示词和参数配置。编辑创作说明，在 Flatkey Playground 生成你的版本。` },
  es: { image: "Prompt de imagen", video: "Prompt de vídeo", description: (model, label, kind) => `Explora el ${kind.toLowerCase()} de ${model} para ${label}: ejemplo generado, prompt completo en inglés y ajustes. Edítalo y crea tu versión en Flatkey Playground.` },
  fr: { image: "Prompt image", video: "Prompt vidéo", description: (model, label, kind) => `Découvrez le ${kind.toLowerCase()} ${model} pour ${label} : exemple généré, prompt anglais complet et paramètres. Modifiez-le dans Flatkey Playground.` },
  pt: { image: "Prompt de imagem", video: "Prompt de vídeo", description: (model, label, kind) => `Explore o ${kind.toLowerCase()} do ${model} para ${label}: exemplo gerado, prompt completo em inglês e configurações. Edite e crie sua versão no Flatkey Playground.` },
  ru: { image: "Промпт для изображения", video: "Промпт для видео", description: (model, label, kind) => `Изучите ${kind.toLowerCase()} ${model} для «${label}»: готовый пример, полный промпт на английском и параметры. Измените его в Flatkey Playground.` },
  ja: { image: "画像生成プロンプト", video: "動画生成プロンプト", description: (model, label, kind) => `${model} の「${label}」向け${kind}。生成例、英語の完全なプロンプト、設定を確認し、Flatkey Playground で編集して生成できます。` },
  vi: { image: "Prompt tạo ảnh", video: "Prompt tạo video", description: (model, label, kind) => `Khám phá ${kind.toLowerCase()} ${model} cho ${label}: ví dụ đã tạo, prompt tiếng Anh đầy đủ và cài đặt. Chỉnh sửa trong Flatkey Playground.` },
  de: { image: "Bild-Prompt", video: "Video-Prompt", description: (model, label, kind) => `Entdecke den ${kind} für ${label} mit ${model}: passendes Beispiel, vollständiger englischer Prompt und Einstellungen. Bearbeite ihn im Flatkey Playground.` },
  id: { image: "Prompt gambar", video: "Prompt video", description: (model, label, kind) => `Jelajahi ${kind.toLowerCase()} ${model} untuk ${label}: contoh hasil, prompt bahasa Inggris lengkap, dan pengaturan. Edit di Flatkey Playground.` },
};

export function getPromptDetailSeoText(detail: PromptDetail, locale: Locale) {
  const copy = SEO_COPY[locale];
  const kindLabel = detail.kind === "video" ? copy.video : copy.image;
  return {
    kindLabel,
    title: limitSeoTitle(`${detail.modelName} ${kindLabel}: ${detail.label} | Flatkey`),
    description: copy.description(detail.modelName, detail.label, kindLabel),
  };
}

export function buildPromptDetailMetadata(detail: PromptDetail, locale: Locale): Metadata {
  const { title, description } = getPromptDetailSeoText(detail, locale);
  const poster = detail.poster || detail.fallbackPoster;
  const image = poster ? new URL(poster, SITE_ORIGIN).toString() : undefined;
  return buildMetadata({
    title,
    description: limitSeoDescription(description),
    pathname: `/models/${detail.modelSlug}/prompts/${detail.id}`,
    locale,
    absoluteTitle: true,
    image,
  });
}
