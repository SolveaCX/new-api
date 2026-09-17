import { type Locale, localizePath } from "./locales";

export function promptDetailPath(modelSlug: string, promptId: string, locale: Locale): string {
  return localizePath(`/models/${encodeURIComponent(modelSlug)}/prompts/${encodeURIComponent(promptId)}`, locale);
}

export function promptExcerpt(prompt: string): string {
  return prompt.split(/\n\s*\n/, 1)[0].replace(/^(?:Creative direction|Scene):\s*/, "");
}

type PromptDetailCopy = {
  back: string;
  eyebrow: string;
  intro: string;
  example: string;
  image: string;
  video: string;
  ratio: string;
  duration: string;
  fullPrompt: string;
  copy: string;
  copied: string;
  copyFailed: string;
  create: string;
  more: string;
  open: string;
};

const COPY: Record<Locale, PromptDetailCopy> = {
  en: { back: "Back to model", eyebrow: "Prompt library", intro: "The complete English prompt and its matching example, ready to use with this model.", example: "Generated example", image: "Image", video: "Video", ratio: "Aspect ratio", duration: "Duration", fullPrompt: "Full prompt", copy: "Copy prompt", copied: "Copied", copyFailed: "Copy failed", create: "Create with this prompt", more: "Explore more prompts", open: "Create similar" },
  zh: { back: "返回模型详情", eyebrow: "提示词库", intro: "查看与此模型示例素材对应的完整英文提示词，并直接复制使用。", example: "生成示例", image: "图片", video: "视频", ratio: "画面比例", duration: "时长", fullPrompt: "完整提示词", copy: "复制提示词", copied: "已复制", copyFailed: "复制失败", create: "用这个提示词生成", more: "探索更多提示词", open: "生成同款" },
  es: { back: "Volver al modelo", eyebrow: "Biblioteca de prompts", intro: "El prompt completo en inglés y su ejemplo correspondiente, listos para usar con este modelo.", example: "Ejemplo generado", image: "Imagen", video: "Vídeo", ratio: "Relación de aspecto", duration: "Duración", fullPrompt: "Prompt completo", copy: "Copiar prompt", copied: "Copiado", copyFailed: "Error al copiar", create: "Crear con este prompt", more: "Explorar más prompts", open: "Crear similar" },
  fr: { back: "Retour au modèle", eyebrow: "Bibliothèque de prompts", intro: "Le prompt complet en anglais et son exemple correspondant, prêts à utiliser avec ce modèle.", example: "Exemple généré", image: "Image", video: "Vidéo", ratio: "Format", duration: "Durée", fullPrompt: "Prompt complet", copy: "Copier le prompt", copied: "Copié", copyFailed: "Échec de la copie", create: "Créer avec ce prompt", more: "Explorer d’autres prompts", open: "Créer similaire" },
  pt: { back: "Voltar ao modelo", eyebrow: "Biblioteca de prompts", intro: "O prompt completo em inglês e seu exemplo correspondente, prontos para usar com este modelo.", example: "Exemplo gerado", image: "Imagem", video: "Vídeo", ratio: "Proporção", duration: "Duração", fullPrompt: "Prompt completo", copy: "Copiar prompt", copied: "Copiado", copyFailed: "Falha ao copiar", create: "Criar com este prompt", more: "Explorar mais prompts", open: "Criar semelhante" },
  ru: { back: "Назад к модели", eyebrow: "Библиотека промптов", intro: "Полный промпт на английском и соответствующий пример для работы с этой моделью.", example: "Созданный пример", image: "Изображение", video: "Видео", ratio: "Соотношение сторон", duration: "Длительность", fullPrompt: "Полный промпт", copy: "Скопировать промпт", copied: "Скопировано", copyFailed: "Не удалось скопировать", create: "Создать по промпту", more: "Другие промпты", open: "Создать похожее" },
  ja: { back: "モデルに戻る", eyebrow: "プロンプトライブラリ", intro: "このモデルの作例に対応する英語のプロンプト全文を確認し、そのまま利用できます。", example: "生成例", image: "画像", video: "動画", ratio: "アスペクト比", duration: "長さ", fullPrompt: "プロンプト全文", copy: "プロンプトをコピー", copied: "コピーしました", copyFailed: "コピーできませんでした", create: "このプロンプトで生成", more: "他のプロンプトを見る", open: "同じように生成" },
  vi: { back: "Quay lại mô hình", eyebrow: "Thư viện prompt", intro: "Prompt tiếng Anh đầy đủ cùng ví dụ tương ứng, sẵn sàng dùng với mô hình này.", example: "Ví dụ đã tạo", image: "Hình ảnh", video: "Video", ratio: "Tỷ lệ khung hình", duration: "Thời lượng", fullPrompt: "Prompt đầy đủ", copy: "Sao chép prompt", copied: "Đã sao chép", copyFailed: "Sao chép thất bại", create: "Tạo bằng prompt này", more: "Khám phá thêm prompt", open: "Tạo tương tự" },
  de: { back: "Zurück zum Modell", eyebrow: "Prompt-Bibliothek", intro: "Der vollständige englische Prompt und das passende Beispiel zur Verwendung mit diesem Modell.", example: "Generiertes Beispiel", image: "Bild", video: "Video", ratio: "Seitenverhältnis", duration: "Dauer", fullPrompt: "Vollständiger Prompt", copy: "Prompt kopieren", copied: "Kopiert", copyFailed: "Kopieren fehlgeschlagen", create: "Mit diesem Prompt erstellen", more: "Weitere Prompts entdecken", open: "Ähnliches erstellen" },
  id: { back: "Kembali ke model", eyebrow: "Pustaka prompt", intro: "Prompt lengkap berbahasa Inggris dan contoh yang sesuai, siap digunakan dengan model ini.", example: "Contoh hasil", image: "Gambar", video: "Video", ratio: "Rasio aspek", duration: "Durasi", fullPrompt: "Prompt lengkap", copy: "Salin prompt", copied: "Tersalin", copyFailed: "Gagal menyalin", create: "Buat dengan prompt ini", more: "Jelajahi prompt lain", open: "Buat serupa" },
};

export function getPromptDetailCopy(locale: Locale): PromptDetailCopy {
  return COPY[locale];
}
