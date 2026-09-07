import { ArrowLeft, ArrowRight, ChevronRight, ExternalLink, Play, Sparkles } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { CliPromptActionPanel } from "@/components/cli-prompt-action-panel";
import { PromptFreeCta } from "@/components/prompt-free-cta";
import { SiteShell } from "@/components/site-shell";
import { PROMPT_IMAGE_PATH, PROMPT_VIDEO_PATH } from "@/lib/cli-landing";
import { fetchCliMediaPromptItem, fetchCliMediaPromptItems, type PromptArtifact, type PromptItem } from "@/lib/prompt-library";
import { type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type MediaKind = "image" | "video";

type CliMediaCopy = {
  artifact: string;
  back: string;
  browseImage: string;
  browseVideo: string;
  createKey: string;
  empty: string;
  filters: string[];
  heroBadge: string;
  heroBody: string;
  heroTitle: string;
  model: string;
  prompt: string;
  source: string;
  viewSource: string;
};

const copyByLocale: Record<Locale, Record<MediaKind, CliMediaCopy>> = withIdFallback({
  en: {
    image: {
      artifact: "Finished image",
      back: "Back to prompt library",
      browseImage: "Image",
      browseVideo: "Video",
      createKey: "Open Playground",
      empty: "No image prompts with finished examples are available yet.",
      filters: ["Characters", "Product visuals", "Storyboards", "Campaign assets"],
      heroBadge: "Image prompt library",
      heroBody: "Explore image prompts with real finished examples. Each entry shows the prompt, model, source, and final image so you can judge the result before using it.",
      heroTitle: "Image prompts with real results.",
      model: "Model",
      prompt: "Prompt",
      source: "Source",
      viewSource: "View source",
    },
    video: {
      artifact: "Finished video",
      back: "Back to prompt library",
      browseImage: "Image",
      browseVideo: "Video",
      createKey: "Open Playground",
      empty: "No video prompts with finished examples are available yet.",
      filters: ["UGC ads", "Product reveal", "Localization", "Storyboard to motion"],
      heroBadge: "Video prompt library",
      heroBody: "Explore video prompts with real clips and production examples. Preview the result, review the model and source, then adapt the prompt for your own video.",
      heroTitle: "Video prompts with real results.",
      model: "Model",
      prompt: "Prompt",
      source: "Source",
      viewSource: "View source",
    },
  },
  zh: {
    image: {
      artifact: "真实图片",
      back: "返回提示词库",
      browseImage: "图像",
      browseVideo: "视频",
      createKey: "进入 Playground",
      empty: "暂时还没有带真实示例的图片提示词。",
      filters: ["角色", "产品视觉", "分镜", "活动素材"],
      heroBadge: "图片提示词库",
      heroBody: "浏览带真实成片的图片提示词。每个案例都会展示提示词、所用模型、来源和最终效果，使用前就能判断是否适合。",
      heroTitle: "看得到效果的图片提示词。",
      model: "模型",
      prompt: "提示词",
      source: "来源",
      viewSource: "查看来源",
    },
    video: {
      artifact: "真实视频",
      back: "返回提示词库",
      browseImage: "图像",
      browseVideo: "视频",
      createKey: "进入 Playground",
      empty: "暂时还没有带真实示例的视频提示词。",
      filters: ["UGC 广告", "产品揭幕", "本地化", "分镜转视频"],
      heroBadge: "视频提示词库",
      heroBody: "浏览带真实成片的视频提示词。先预览效果、查看模型与来源，再把提示词改成适合你项目的版本。",
      heroTitle: "看得到效果的视频提示词。",
      model: "模型",
      prompt: "提示词",
      source: "来源",
      viewSource: "查看来源",
    },
  },
  es: {
    image: {
      artifact: "Imagen producida",
      back: "Volver a la biblioteca de prompts",
      browseImage: "Imagen",
      browseVideo: "Video",
      createKey: "Abrir Playground",
      empty: "Todavía no hay prompts de imagen con ejemplos terminados.",
      filters: ["Personajes", "Visuales de producto", "Guiones gráficos", "Campañas"],
      heroBadge: "Biblioteca de prompts de imagen",
      heroBody: "Explora prompts de imagen con resultados reales. Cada ejemplo incluye el prompt, el modelo, la fuente y la imagen final para que puedas evaluarlo antes de usarlo.",
      heroTitle: "Prompts de imagen con resultados reales.",
      model: "Modelo",
      prompt: "Prompt",
      source: "Fuente",
      viewSource: "Ver fuente",
    },
    video: {
      artifact: "Vídeo terminado",
      back: "Volver a la biblioteca de prompts",
      browseImage: "Imagen",
      browseVideo: "Video",
      createKey: "Abrir Playground",
      empty: "Todavía no hay prompts de vídeo con ejemplos terminados.",
      filters: ["Anuncios UGC", "Presentación de producto", "Localización", "Guion a vídeo"],
      heroBadge: "Biblioteca de prompts de vídeo",
      heroBody: "Explora prompts de vídeo con clips reales. Previsualiza el resultado, revisa el modelo y la fuente, y adapta el prompt a tu proyecto.",
      heroTitle: "Prompts de vídeo con resultados reales.",
      model: "Modelo",
      prompt: "Prompt",
      source: "Fuente",
      viewSource: "Ver fuente",
    },
  },
  fr: {
    image: { artifact: "Image finalisée", back: "Retour à la bibliothèque de prompts", browseImage: "Image", browseVideo: "Vidéo", createKey: "Ouvrir le Playground", empty: "Aucun prompt d’image avec exemple finalisé n’est encore disponible.", filters: ["Personnages", "Visuels produit", "Storyboards", "Campagnes"], heroBadge: "Bibliothèque de prompts d’image", heroBody: "Explorez des prompts d’image accompagnés de résultats réels. Chaque exemple présente le prompt, le modèle, la source et l’image finale pour vous aider à l’évaluer avant utilisation.", heroTitle: "Des prompts d’image avec de vrais résultats.", model: "Modèle", prompt: "Prompt", source: "Source", viewSource: "Voir la source" },
    video: { artifact: "Vidéo finalisée", back: "Retour à la bibliothèque de prompts", browseImage: "Image", browseVideo: "Vidéo", createKey: "Ouvrir le Playground", empty: "Aucun prompt vidéo avec exemple finalisé n’est encore disponible.", filters: ["Publicités UGC", "Présentation produit", "Localisation", "Storyboard animé"], heroBadge: "Bibliothèque de prompts vidéo", heroBody: "Explorez des prompts vidéo accompagnés de clips réels. Prévisualisez le résultat, vérifiez le modèle et la source, puis adaptez le prompt à votre projet.", heroTitle: "Des prompts vidéo avec de vrais résultats.", model: "Modèle", prompt: "Prompt", source: "Source", viewSource: "Voir la source" },
  },
  pt: {
    image: { artifact: "Imagem finalizada", back: "Voltar à biblioteca de prompts", browseImage: "Imagem", browseVideo: "Vídeo", createKey: "Abrir o Playground", empty: "Ainda não há prompts de imagem com exemplos finalizados.", filters: ["Personagens", "Visuais de produto", "Storyboards", "Campanhas"], heroBadge: "Biblioteca de prompts de imagem", heroBody: "Explore prompts de imagem com resultados reais. Cada exemplo mostra o prompt, o modelo, a fonte e a imagem final para você avaliar antes de usar.", heroTitle: "Prompts de imagem com resultados reais.", model: "Modelo", prompt: "Prompt", source: "Fonte", viewSource: "Ver a fonte" },
    video: { artifact: "Vídeo finalizado", back: "Voltar à biblioteca de prompts", browseImage: "Imagem", browseVideo: "Vídeo", createKey: "Abrir o Playground", empty: "Ainda não há prompts de vídeo com exemplos finalizados.", filters: ["Anúncios UGC", "Apresentação de produto", "Localização", "Storyboard em vídeo"], heroBadge: "Biblioteca de prompts de vídeo", heroBody: "Explore prompts de vídeo com clipes reais. Veja o resultado, confira o modelo e a fonte e adapte o prompt ao seu projeto.", heroTitle: "Prompts de vídeo com resultados reais.", model: "Modelo", prompt: "Prompt", source: "Fonte", viewSource: "Ver a fonte" },
  },
  ru: {
    image: { artifact: "Готовое изображение", back: "Назад к библиотеке промптов", browseImage: "Изображения", browseVideo: "Видео", createKey: "Открыть Playground", empty: "Промптов для изображений с готовыми примерами пока нет.", filters: ["Персонажи", "Товарная визуализация", "Раскадровки", "Рекламные материалы"], heroBadge: "Библиотека промптов для изображений", heroBody: "Изучайте промпты с реальными готовыми изображениями. В каждом примере указаны промпт, модель, источник и итоговый результат — всё необходимое для оценки до использования.", heroTitle: "Промпты для изображений с реальными результатами.", model: "Модель", prompt: "Промпт", source: "Источник", viewSource: "Открыть источник" },
    video: { artifact: "Готовое видео", back: "Назад к библиотеке промптов", browseImage: "Изображения", browseVideo: "Видео", createKey: "Открыть Playground", empty: "Промптов для видео с готовыми примерами пока нет.", filters: ["UGC-реклама", "Презентация продукта", "Локализация", "Раскадровка в видео"], heroBadge: "Библиотека промптов для видео", heroBody: "Изучайте видеопромпты с реальными роликами. Просмотрите результат, проверьте модель и источник, затем адаптируйте промпт под свой проект.", heroTitle: "Видеопромпты с реальными результатами.", model: "Модель", prompt: "Промпт", source: "Источник", viewSource: "Открыть источник" },
  },
  ja: {
    image: { artifact: "完成画像", back: "プロンプトライブラリに戻る", browseImage: "画像", browseVideo: "動画", createKey: "Playground を開く", empty: "完成例付きの画像プロンプトはまだありません。", filters: ["キャラクター", "商品ビジュアル", "絵コンテ", "キャンペーン素材"], heroBadge: "画像プロンプトライブラリ", heroBody: "実際の完成画像が付いたプロンプトを閲覧できます。各例にはプロンプト、使用モデル、出典、最終結果がまとまっているため、使う前に仕上がりを確認できます。", heroTitle: "実例で選べる画像プロンプト。", model: "モデル", prompt: "プロンプト", source: "出典", viewSource: "出典を見る" },
    video: { artifact: "完成動画", back: "プロンプトライブラリに戻る", browseImage: "画像", browseVideo: "動画", createKey: "Playground を開く", empty: "完成例付きの動画プロンプトはまだありません。", filters: ["UGC 広告", "商品紹介", "ローカライズ", "絵コンテから動画"], heroBadge: "動画プロンプトライブラリ", heroBody: "実際の動画が付いたプロンプトを閲覧できます。結果をプレビューし、モデルと出典を確認してから、自分のプロジェクト向けに調整できます。", heroTitle: "実例で選べる動画プロンプト。", model: "モデル", prompt: "プロンプト", source: "出典", viewSource: "出典を見る" },
  },
  vi: {
    image: { artifact: "Ảnh hoàn chỉnh", back: "Quay lại thư viện prompt", browseImage: "Hình ảnh", browseVideo: "Video", createKey: "Mở Playground", empty: "Chưa có prompt hình ảnh kèm ví dụ hoàn chỉnh.", filters: ["Nhân vật", "Hình ảnh sản phẩm", "Bảng phân cảnh", "Nội dung chiến dịch"], heroBadge: "Thư viện prompt hình ảnh", heroBody: "Khám phá prompt hình ảnh kèm kết quả thực tế. Mỗi ví dụ hiển thị prompt, mô hình, nguồn và ảnh hoàn chỉnh để bạn đánh giá trước khi sử dụng.", heroTitle: "Prompt hình ảnh với kết quả thực tế.", model: "Mô hình", prompt: "Prompt", source: "Nguồn", viewSource: "Xem nguồn" },
    video: { artifact: "Video hoàn chỉnh", back: "Quay lại thư viện prompt", browseImage: "Hình ảnh", browseVideo: "Video", createKey: "Mở Playground", empty: "Chưa có prompt video kèm ví dụ hoàn chỉnh.", filters: ["Quảng cáo UGC", "Giới thiệu sản phẩm", "Bản địa hóa", "Từ phân cảnh đến video"], heroBadge: "Thư viện prompt video", heroBody: "Khám phá prompt video kèm clip thực tế. Xem trước kết quả, kiểm tra mô hình và nguồn, sau đó điều chỉnh prompt cho dự án của bạn.", heroTitle: "Prompt video với kết quả thực tế.", model: "Mô hình", prompt: "Prompt", source: "Nguồn", viewSource: "Xem nguồn" },
  },
  de: {
    image: { artifact: "Fertiges Bild", back: "Zurück zur Prompt-Bibliothek", browseImage: "Bild", browseVideo: "Video", createKey: "Playground öffnen", empty: "Noch sind keine Bild-Prompts mit fertigen Beispielen verfügbar.", filters: ["Charaktere", "Produktvisuals", "Storyboards", "Kampagnenmaterial"], heroBadge: "Bibliothek für Bild-Prompts", heroBody: "Entdecken Sie Bild-Prompts mit echten Ergebnissen. Jedes Beispiel zeigt Prompt, Modell, Quelle und fertiges Bild, damit Sie das Ergebnis vor der Nutzung einschätzen können.", heroTitle: "Bild-Prompts mit echten Ergebnissen.", model: "Modell", prompt: "Prompt", source: "Quelle", viewSource: "Quelle ansehen" },
    video: { artifact: "Fertiges Video", back: "Zurück zur Prompt-Bibliothek", browseImage: "Bild", browseVideo: "Video", createKey: "Playground öffnen", empty: "Noch sind keine Video-Prompts mit fertigen Beispielen verfügbar.", filters: ["UGC-Anzeigen", "Produktpräsentation", "Lokalisierung", "Storyboard zu Video"], heroBadge: "Bibliothek für Video-Prompts", heroBody: "Entdecken Sie Video-Prompts mit echten Clips. Prüfen Sie Ergebnis, Modell und Quelle und passen Sie den Prompt anschließend an Ihr Projekt an.", heroTitle: "Video-Prompts mit echten Ergebnissen.", model: "Modell", prompt: "Prompt", source: "Quelle", viewSource: "Quelle ansehen" },
  },
  id: {
    image: { artifact: "Gambar jadi", back: "Kembali ke pustaka prompt", browseImage: "Gambar", browseVideo: "Video", createKey: "Buka Playground", empty: "Belum ada prompt gambar dengan contoh hasil akhir.", filters: ["Karakter", "Visual produk", "Storyboard", "Materi kampanye"], heroBadge: "Pustaka prompt gambar", heroBody: "Jelajahi prompt gambar dengan hasil nyata. Setiap contoh menampilkan prompt, model, sumber, dan gambar akhir agar Anda dapat menilainya sebelum digunakan.", heroTitle: "Prompt gambar dengan hasil nyata.", model: "Model", prompt: "Prompt", source: "Sumber", viewSource: "Lihat sumber" },
    video: { artifact: "Video jadi", back: "Kembali ke pustaka prompt", browseImage: "Gambar", browseVideo: "Video", createKey: "Buka Playground", empty: "Belum ada prompt video dengan contoh hasil akhir.", filters: ["Iklan UGC", "Peluncuran produk", "Lokalisasi", "Storyboard ke video"], heroBadge: "Pustaka prompt video", heroBody: "Jelajahi prompt video dengan klip nyata. Pratinjau hasil, periksa model dan sumber, lalu sesuaikan prompt untuk proyek Anda.", heroTitle: "Prompt video dengan hasil nyata.", model: "Model", prompt: "Prompt", source: "Sumber", viewSource: "Lihat sumber" },
  },
});

const uiCopyByLocale: Record<Locale, { promptLabel: string; countLabel: string; weekly: string; popular: Record<MediaKind, string>; curated: string; curatedTitle: Record<MediaKind, string>; more: string; related: string; backList: string; type: string; updated: string; owned: string }> = {
  en: { promptLabel: "Prompts", countLabel: "prompts", weekly: "Popular this week", popular: { image: "Popular image prompts", video: "Popular video prompts" }, curated: "Curated examples", curatedTitle: { image: "Image prompts with production-ready results", video: "Video prompts with production-ready results" }, more: "Keep exploring", related: "Related prompt results", backList: "Back to list", type: "Type", updated: "Updated", owned: "Flatkey-owned example" },
  zh: { promptLabel: "提示词", countLabel: "条提示词", weekly: "本周热门", popular: { image: "热门图片提示词", video: "热门视频提示词" }, curated: "精选案例", curatedTitle: { image: "带真实成片的图片提示词", video: "带真实成片的视频提示词" }, more: "继续浏览", related: "相关提示词案例", backList: "返回列表", type: "类型", updated: "更新", owned: "Flatkey 自有示例" },
  es: { promptLabel: "Prompts", countLabel: "prompts", weekly: "Popular esta semana", popular: { image: "Prompts de imagen populares", video: "Prompts de vídeo populares" }, curated: "Ejemplos seleccionados", curatedTitle: { image: "Prompts de imagen con resultados listos para producción", video: "Prompts de vídeo con resultados listos para producción" }, more: "Seguir explorando", related: "Resultados de prompts relacionados", backList: "Volver a la lista", type: "Tipo", updated: "Actualizado", owned: "Ejemplo propio de Flatkey" },
  fr: { promptLabel: "Prompts", countLabel: "prompts", weekly: "Populaires cette semaine", popular: { image: "Prompts d’image populaires", video: "Prompts vidéo populaires" }, curated: "Exemples sélectionnés", curatedTitle: { image: "Prompts d’image avec résultats prêts à produire", video: "Prompts vidéo avec résultats prêts à produire" }, more: "Continuer à explorer", related: "Résultats de prompts associés", backList: "Retour à la liste", type: "Type", updated: "Mise à jour", owned: "Exemple créé par Flatkey" },
  pt: { promptLabel: "Prompts", countLabel: "prompts", weekly: "Populares nesta semana", popular: { image: "Prompts de imagem populares", video: "Prompts de vídeo populares" }, curated: "Exemplos selecionados", curatedTitle: { image: "Prompts de imagem com resultados prontos para produção", video: "Prompts de vídeo com resultados prontos para produção" }, more: "Continuar explorando", related: "Resultados de prompts relacionados", backList: "Voltar à lista", type: "Tipo", updated: "Atualizado", owned: "Exemplo próprio da Flatkey" },
  ru: { promptLabel: "Промпты", countLabel: "промптов", weekly: "Популярное за неделю", popular: { image: "Популярные промпты для изображений", video: "Популярные промпты для видео" }, curated: "Подборка примеров", curatedTitle: { image: "Промпты для изображений с готовыми результатами", video: "Промпты для видео с готовыми результатами" }, more: "Продолжить просмотр", related: "Похожие примеры промптов", backList: "Назад к списку", type: "Тип", updated: "Обновлено", owned: "Пример, созданный Flatkey" },
  ja: { promptLabel: "プロンプト", countLabel: "件のプロンプト", weekly: "今週の人気", popular: { image: "人気の画像プロンプト", video: "人気の動画プロンプト" }, curated: "厳選された実例", curatedTitle: { image: "制作に使える完成例付き画像プロンプト", video: "制作に使える完成例付き動画プロンプト" }, more: "さらに見る", related: "関連するプロンプト実例", backList: "一覧に戻る", type: "種類", updated: "更新日", owned: "Flatkey 制作の実例" },
  vi: { promptLabel: "Prompt", countLabel: "prompt", weekly: "Phổ biến tuần này", popular: { image: "Prompt hình ảnh phổ biến", video: "Prompt video phổ biến" }, curated: "Ví dụ tuyển chọn", curatedTitle: { image: "Prompt hình ảnh với kết quả sẵn sàng sản xuất", video: "Prompt video với kết quả sẵn sàng sản xuất" }, more: "Khám phá thêm", related: "Kết quả prompt liên quan", backList: "Quay lại danh sách", type: "Loại", updated: "Cập nhật", owned: "Ví dụ do Flatkey tạo" },
  de: { promptLabel: "Prompts", countLabel: "Prompts", weekly: "Diese Woche beliebt", popular: { image: "Beliebte Bild-Prompts", video: "Beliebte Video-Prompts" }, curated: "Ausgewählte Beispiele", curatedTitle: { image: "Bild-Prompts mit produktionsreifen Ergebnissen", video: "Video-Prompts mit produktionsreifen Ergebnissen" }, more: "Weiter entdecken", related: "Ähnliche Prompt-Ergebnisse", backList: "Zurück zur Liste", type: "Typ", updated: "Aktualisiert", owned: "Von Flatkey erstelltes Beispiel" },
  id: { promptLabel: "Prompt", countLabel: "prompt", weekly: "Populer minggu ini", popular: { image: "Prompt gambar populer", video: "Prompt video populer" }, curated: "Contoh pilihan", curatedTitle: { image: "Prompt gambar dengan hasil siap produksi", video: "Prompt video dengan hasil siap produksi" }, more: "Jelajahi lainnya", related: "Hasil prompt terkait", backList: "Kembali ke daftar", type: "Jenis", updated: "Diperbarui", owned: "Contoh buatan Flatkey" },
};

export function cliMediaPath(kind: MediaKind) {
  return kind === "image" ? PROMPT_IMAGE_PATH : PROMPT_VIDEO_PATH;
}

export function cliMediaDetailPath(kind: MediaKind, slug: string) {
  return `${cliMediaPath(kind)}/${slug}`;
}

export function getCliMediaMetadata(kind: MediaKind, locale: Locale) {
  const copy = copyByLocale[locale][kind];
  return {
    title: `${copy.heroBadge} | Flatkey`,
    description: copy.heroBody,
    pathname: cliMediaPath(kind),
  };
}

export async function getCliMediaDetailMetadata(kind: MediaKind, slug: string, locale: Locale) {
  const item = await fetchCliMediaPromptItem(kind, slug);
  if (!item) return undefined;
  const title = item.title[locale] ?? item.title.en;
  const summary = item.summary[locale] ?? item.summary.en;
  const copy = copyByLocale[locale][kind];
  const ui = uiCopyByLocale[locale];
  return {
    title: `${title} — ${kind === "image" ? copy.browseImage : copy.browseVideo} ${ui.promptLabel} | Flatkey`,
    description: summary,
    pathname: cliMediaDetailPath(kind, slug),
  };
}

export async function CliMediaLibraryPage(props: { kind: MediaKind; locale: Locale }) {
  const copy = copyByLocale[props.locale][props.kind];
  const ui = uiCopyByLocale[props.locale];
  const items = await fetchCliMediaPromptItems(props.kind);
  const featuredItem = items[0];
  const weeklyItems = items.slice(0, 4);
  const curatedItems = items.slice(0, 8);
  const keyUrl = consoleUrl("/playground", new URLSearchParams({ generate: props.kind, lng: props.locale, source: "prompt-library" }).toString());
  const currentPath = cliMediaPath(props.kind);
  const displayName = props.kind === "image" ? copy.browseImage : copy.browseVideo;
  const isVideo = props.kind === "video";

  return (
    <SiteShell locale={props.locale} pathname={currentPath}>
      <main className="relative min-h-screen overflow-x-hidden bg-white text-[#171a21]">
        <section className="relative z-10 px-6 pt-10 pb-8 sm:px-8 md:pt-14 md:pb-10 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
              <PromptBreadcrumb locale={props.locale} kind={props.kind} />
              <div className="flex flex-wrap items-center gap-2">
                <a href={keyUrl} className="inline-flex h-10 items-center gap-2 rounded-lg bg-[#7c3aed] px-4 text-sm font-extrabold text-white shadow-[0_16px_34px_-18px_rgba(124,58,237,.65)] hover:bg-[#6d28d9]" style={{ color: "#fff" }}>
                  <Sparkles className="size-4" />
                  {copy.createKey}
                </a>
                <Link href={localizePath("/prompts", props.locale)} className="inline-flex h-10 items-center gap-2 rounded-lg border border-[#0B0B0F14] bg-white px-4 text-sm font-bold text-[#3d3845] shadow-sm hover:border-[#7c3aed]/35 hover:text-[#4c1d95]">
                  <ArrowLeft className="size-4" />
                  {copy.back}
                </Link>
              </div>
            </div>
            <div className={`grid gap-8 lg:items-center ${isVideo ? "lg:grid-cols-[0.7fr_1.3fr]" : "lg:grid-cols-[0.78fr_1.22fr]"}`}>
              <div>
                <p className="mb-4 inline-flex rounded-full border border-[#ded6f4] bg-[#f4f0ff] px-3.5 py-1.5 text-[13px] font-extrabold text-[#4c1d95]">{copy.heroBadge}</p>
                <h1 className={`max-w-xl font-extrabold tracking-tight ${isVideo ? "text-[clamp(2.15rem,4.4vw,3.7rem)] leading-none" : "text-[clamp(2.25rem,5vw,3.7rem)] leading-none"}`}>
                  {displayName}
                  <span className="block text-[#7c3aed]">{ui.promptLabel}</span>
                </h1>
                <p className="mt-5 max-w-lg text-sm leading-6 text-[#62626D] md:text-base">{copy.heroBody}</p>
                <div className="mt-6 flex flex-wrap gap-2">
                  {copy.filters.map((filter) => (
                    <span key={filter} className="rounded-full border border-[#0B0B0F14] bg-white px-3 py-1.5 text-xs font-bold text-[#45414C] shadow-sm">{filter}</span>
                  ))}
                </div>
                <div className="mt-7 flex items-center gap-3 text-xs font-semibold text-[#77727f]">
                  <span className="inline-flex size-2 rounded-full bg-emerald-500 shadow-[0_0_16px_rgba(16,185,129,0.4)]" />
                  <span>{items.length} {ui.countLabel}</span>
                  <span className="text-[#c8c3cf]">•</span>
                  <span>{copy.artifact}</span>
                </div>
              </div>
              {featuredItem ? <FeaturedPreview copy={copy} item={featuredItem} kind={props.kind} locale={props.locale} /> : null}
            </div>
          </div>
        </section>

        <section className="relative z-10 border-y border-[#0B0B0F0D] bg-[#f8f6fc] px-6 py-10 sm:px-8 md:py-12 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            {items.length === 0 ? (
              <div className="rounded-2xl border border-[#E7E4EC] bg-white p-8 text-sm text-[#62626D] shadow-sm">{copy.empty}</div>
            ) : (
              <>
                <SectionHeading eyebrow={ui.weekly} title={ui.popular[props.kind]} />
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  {weeklyItems.map((item) => (
                    <CompactPromptCard item={item} key={item.slug} kind={props.kind} locale={props.locale} />
                  ))}
                </div>

                <div className="mt-14">
                  <SectionHeading eyebrow={ui.curated} title={ui.curatedTitle[props.kind]} />
                  <div className={isVideo ? "grid gap-5 md:grid-cols-2 xl:grid-cols-3" : "columns-1 gap-5 md:columns-2 xl:columns-3"}>
                    {curatedItems.map((item) => (
                      <PromptCard copy={copy} item={item} key={item.slug} keyUrl={keyUrl} kind={props.kind} locale={props.locale} />
                    ))}
                  </div>
                </div>
              </>
            )}
          </div>
        </section>
        <PromptFreeCta locale={props.locale} kind={props.kind} />
      </main>
    </SiteShell>
  );
}

export async function CliMediaPromptDetailPage(props: { kind: MediaKind; locale: Locale; slug: string }) {
  const copy = copyByLocale[props.locale][props.kind];
  const ui = uiCopyByLocale[props.locale];
  const item = await fetchCliMediaPromptItem(props.kind, props.slug);
  const keyUrl = consoleUrl("/playground", new URLSearchParams({ generate: props.kind, lng: props.locale, source: "prompt-library" }).toString());

  if (!item) return null;

  const title = item.title[props.locale] ?? item.title.en;
  const summary = item.summary[props.locale] ?? item.summary.en;
  const currentPath = cliMediaDetailPath(props.kind, item.slug);
  const listPath = localizePath(cliMediaPath(props.kind), props.locale);
  const relatedItems = (await fetchCliMediaPromptItems(props.kind)).filter((candidate) => candidate.slug !== item.slug).slice(0, 3);
  const isVideo = props.kind === "video";
  const generateParams = new URLSearchParams({
    generate: props.kind,
    lng: props.locale,
    prompt: item.prompt,
    source: `flatkey-cli-${props.kind}-prompt`,
    slug: item.slug,
  });
  const generateUrl = consoleUrl("/playground", generateParams.toString());

  return (
    <SiteShell locale={props.locale} pathname={currentPath}>
      <main className="relative min-h-screen bg-white text-[#171a21]">
        <section className="relative z-10 px-6 pt-10 pb-8 sm:px-8 md:pt-14 md:pb-10 lg:px-10">
          <div className="mx-auto max-w-[1280px]">
            <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
              <PromptBreadcrumb locale={props.locale} kind={props.kind} itemTitle={title} />
              <div className="flex flex-wrap items-center gap-2">
                <a href={generateUrl} className="inline-flex h-10 items-center gap-2 rounded-lg bg-[#7c3aed] px-4 text-sm font-extrabold text-white shadow-[0_16px_34px_-18px_rgba(124,58,237,.65)] hover:bg-[#6d28d9]" style={{ color: "#fff" }}>
                  <Sparkles className="size-4" />
                  {copy.createKey}
                </a>
                <Link href={listPath} className="inline-flex h-10 items-center gap-2 rounded-xl border border-[#0B0B0F14] bg-white px-4 text-sm font-bold text-[#3d3845] shadow-sm hover:border-[#7c3aed]/35 hover:text-[#4c1d95]">
                  <ArrowLeft className="size-4" />
                  {ui.backList}
                </Link>
              </div>
            </div>
            <div className="mt-6 max-w-4xl">
              <div className="mb-4 flex flex-wrap gap-2">
                <span className="inline-flex items-center gap-1 rounded-full bg-violet-500/10 px-3 py-1 text-xs font-bold text-violet-700"><Sparkles className="size-3.5" />{copy.artifact}</span>
                <span className="rounded-full bg-white px-3 py-1 text-xs font-bold text-[#62626D] shadow-sm">{item.updatedAt}</span>
              </div>
              <h1 className="text-[clamp(2.25rem,5vw,3.7rem)] leading-none font-extrabold tracking-tight">{title}</h1>
              <p className="mt-5 max-w-2xl text-base leading-7 text-[#62626D]">{summary}</p>
            </div>
            <div className={`mt-8 grid gap-6 lg:items-start ${isVideo ? "lg:grid-cols-[minmax(0,1.25fr)_300px]" : "lg:grid-cols-[minmax(0,1fr)_320px]"}`}>
              <article className="overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_24px_70px_-46px_rgba(46,16,101,.26)]">
                <ArtifactPreview artifact={item.artifact} title={title} variant="detail" />
              </article>
              <aside className="space-y-4">
                <div className="rounded-2xl border border-[#0B0B0F14] bg-white p-5 shadow-sm">
                  <div className="grid grid-cols-2 gap-3">
                    <DetailMetric label={ui.type} value={props.kind === "image" ? copy.browseImage : copy.browseVideo} />
                    <DetailMetric label={ui.updated} value={item.updatedAt} />
                    <div className="col-span-2"><DetailMetric label={copy.model} value={item.model} /></div>
                  </div>
                  <div className="mt-4 grid gap-4 border-t border-[#0B0B0F10] pt-4">
                    <SourceInfo copy={copy} item={item} locale={props.locale} />
                  </div>
                  <div className="mt-5 flex flex-wrap gap-2">
                    {visibleTags(item).map((tag) => (
                      <span key={tag} className="rounded-full border border-[#0B0B0F12] bg-[#fbfaff] px-3 py-1 text-xs font-bold text-[#43434C]">{tag}</span>
                    ))}
                  </div>
                </div>
                <a className="flex items-center justify-between rounded-xl border border-violet-500/20 bg-violet-600 px-4 py-3 text-sm font-bold text-white shadow-[0_18px_38px_-26px_rgba(91,33,182,0.45)] hover:bg-violet-700" href={keyUrl} style={{ color: "#fff" }}>
                  <span className="inline-flex items-center gap-2"><Sparkles className="size-4" />{copy.createKey}</span>
                  <ArrowRight className="size-4" />
                </a>
              </aside>
            </div>
          </div>
        </section>

        <section className="border-y border-[#0B0B0F0D] bg-[#f8f6fc] px-6 py-8 sm:px-8 lg:px-10">
          <div className="mx-auto grid max-w-[1280px] gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
            <CliPromptActionPanel
              defaultPrompt={item.prompt}
              generateUrl={generateUrl}
              kind={props.kind}
              locale={props.locale}
              model={item.model}
              ratio={item.output.ratio}
              title={copy.prompt}
            />
            <aside className="space-y-4">
              <SourcePanel copy={copy} item={item} locale={props.locale} />
            </aside>
          </div>
        </section>

        {relatedItems.length > 0 ? (
          <section className="px-6 py-12 sm:px-8 lg:px-10">
            <div className="mx-auto max-w-[1280px]">
              <SectionHeading eyebrow={ui.more} title={ui.related} />
              <div className="grid gap-4 md:grid-cols-3">
                {relatedItems.map((related) => (
                  <CompactPromptCard item={related} key={related.slug} kind={props.kind} locale={props.locale} />
                ))}
              </div>
            </div>
          </section>
        ) : null}
        <PromptFreeCta locale={props.locale} kind={props.kind} />
      </main>
    </SiteShell>
  );
}

function PromptBreadcrumb(props: { locale: Locale; kind: MediaKind; itemTitle?: string }) {
  const copy = copyByLocale[props.locale][props.kind];
  const mediaLabel = props.kind === "image" ? copy.browseImage : copy.browseVideo;
  const crumbs = [
    { label: "Flatkey", href: localizePath("/", props.locale) },
    { label: mediaLabel, href: localizePath(cliMediaPath(props.kind), props.locale) },
  ];

  return (
    <nav aria-label="Breadcrumb" className="flex min-w-0 flex-wrap items-center gap-1 text-xs text-[#6B6475]">
      {crumbs.map((crumb, index) => (
        <span key={crumb.href} className="inline-flex min-w-0 items-center gap-1">
          {index > 0 ? <ChevronRight className="size-3 text-[#AAA3B2]" aria-hidden="true" /> : null}
          <Link href={crumb.href} className="truncate hover:text-[#4c1d95]">
            {crumb.label}
          </Link>
        </span>
      ))}
      {props.itemTitle ? (
        <span className="inline-flex min-w-0 items-center gap-1">
          <ChevronRight className="size-3 text-[#AAA3B2]" aria-hidden="true" />
          <span className="min-w-0 truncate font-mono text-[#0B0B0F]/80">{props.itemTitle}</span>
        </span>
      ) : null}
    </nav>
  );
}

function SectionHeading(props: { eyebrow: string; title: string }) {
  return (
    <div className="mb-6 flex items-end justify-between gap-4">
      <div>
        <p className="text-xs font-black tracking-[0.16em] text-violet-600 uppercase">{props.eyebrow}</p>
        <h2 className="mt-2 text-3xl font-black tracking-[-0.035em] text-[#0B0B0F]">{props.title}</h2>
      </div>
    </div>
  );
}

function FeaturedPreview(props: { copy: CliMediaCopy; item: PromptItem; kind: MediaKind; locale: Locale }) {
  const title = props.item.title[props.locale] ?? props.item.title.en;
  const href = localizePath(cliMediaDetailPath(props.kind, props.item.slug), props.locale);
  return (
    <article className="overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_24px_70px_-46px_rgba(46,16,101,.26)] lg:ml-auto lg:w-full">
      <Link aria-label={title} href={href}>
        <ArtifactPreview artifact={props.item.artifact} title={title} variant="hero" />
      </Link>
      <div className="flex items-center justify-between gap-3 border-t border-[#0B0B0F10] px-5 py-4">
        <div className="min-w-0">
          <p className="truncate text-sm font-black text-[#0B0B0F]">{title}</p>
          <p className="mt-1 truncate font-mono text-[11px] font-semibold text-[#77727f]">{props.item.model}</p>
        </div>
        <span className="rounded-full border border-violet-500/20 bg-violet-500/10 px-2.5 py-1 text-[11px] font-black text-violet-700">{props.copy.artifact}</span>
      </div>
    </article>
  );
}

function PromptCard(props: { copy: CliMediaCopy; item: PromptItem; keyUrl: string; kind: MediaKind; locale: Locale }) {
  const title = props.item.title[props.locale] ?? props.item.title.en;
  const summary = props.item.summary[props.locale] ?? props.item.summary.en;
  const href = localizePath(cliMediaDetailPath(props.kind, props.item.slug), props.locale);

  return (
    <article className={`mb-5 break-inside-avoid overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_20px_70px_-58px_rgba(46,16,101,.28)] transition-transform transition-shadow hover:-translate-y-1 hover:border-violet-500/30 hover:shadow-[0_28px_80px_-54px_rgba(91,33,182,.38)] ${props.kind === "video" ? "md:mb-6" : ""}`}>
      <Link aria-label={title} href={href}>
        <ArtifactPreview artifact={props.item.artifact} title={title} />
      </Link>
      <div className="p-5">
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1 rounded-full bg-violet-500/10 px-2.5 py-1 text-[11px] font-bold text-violet-700"><Sparkles className="size-3" />{props.copy.artifact}</span>
          <span className="rounded-full border border-[#0B0B0F14] bg-white px-2.5 py-1 text-[11px] font-bold text-[#45414C]">{props.item.model}</span>
          <span className="rounded-full bg-[#0B0B0F0A] px-2.5 py-1 text-[11px] font-bold text-[#62626D]">{props.item.updatedAt}</span>
        </div>
        <h2 className="text-xl font-semibold tracking-tight text-[#0B0B0F]">{title}</h2>
        <p className="mt-2 text-sm leading-6 text-[#62626D]">{summary}</p>
        <div className="mt-5 rounded-xl border border-[#0B0B0F10] bg-[#161020]">
          <div className="flex items-center justify-between border-b border-white/10 px-4 py-3">
            <span className="text-xs font-semibold text-white/60">{props.copy.prompt}</span>
          </div>
          <pre className="max-h-48 overflow-auto p-4 text-[12px] leading-6 whitespace-pre-wrap text-violet-100"><code>{props.item.prompt}</code></pre>
        </div>
        <div className="mt-5 flex flex-wrap gap-2">
          {!isOwnedSource(props.item) && props.item.source.url ? (
              <a className="inline-flex h-9 items-center gap-2 rounded-lg border border-[#0B0B0F14] bg-white px-3 text-sm font-semibold text-[#43434C] hover:border-violet-500/35 hover:text-[#0B0B0F]" href={props.item.source.url} target="_blank" rel="noopener noreferrer">
              <ExternalLink className="size-4" />
              {props.copy.viewSource}
            </a>
          ) : null}
          <a className="inline-flex h-9 items-center gap-2 rounded-lg border border-violet-500/20 bg-violet-600 px-3 text-sm font-semibold text-white shadow-[0_12px_28px_-18px_rgba(91,33,182,0.55)] hover:bg-violet-700" href={props.keyUrl} style={{ color: "#fff" }}>
            <Sparkles className="size-4" />
            {props.copy.createKey}
          </a>
        </div>
      </div>
    </article>
  );
}

function CompactPromptCard(props: { item: PromptItem; kind: MediaKind; locale: Locale }) {
  const title = props.item.title[props.locale] ?? props.item.title.en;
  const href = localizePath(cliMediaDetailPath(props.kind, props.item.slug), props.locale);
  return (
    <article className="overflow-hidden rounded-2xl border border-[#0B0B0F14] bg-white shadow-[0_18px_50px_-42px_rgba(46,16,101,.3)] transition-transform hover:-translate-y-1 hover:border-violet-500/30">
      <Link aria-label={title} href={href}>
        <ArtifactPreview artifact={props.item.artifact} title={title} variant="compact" />
      </Link>
      <div className="border-t border-[#0B0B0F10] p-4">
        <h3 className="line-clamp-2 min-h-10 text-sm font-black leading-5 text-[#0B0B0F]">{title}</h3>
        <div className="mt-3 flex items-center justify-end gap-2">
          <span className="mr-auto min-w-0 truncate font-mono text-[11px] font-semibold text-[#77727f]">{props.item.model}</span>
          <span className="rounded-full bg-violet-500/10 px-2 py-0.5 text-[11px] font-black text-violet-700">{props.item.updatedAt}</span>
        </div>
      </div>
    </article>
  );
}

function Info(props: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[11px] font-bold tracking-[0.12em] text-[#62626D] uppercase">{props.label}</p>
      <p className="mt-1 text-sm font-semibold text-[#0B0B0F]">{props.value}</p>
    </div>
  );
}

function DetailMetric(props: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-[#0B0B0F10] bg-[#fbfaff] p-3">
      <p className="text-[11px] font-bold tracking-[0.12em] text-[#62626D] uppercase">{props.label}</p>
      <p className="mt-1 truncate text-sm font-black text-[#0B0B0F]">{props.value}</p>
    </div>
  );
}

function visibleTags(item: PromptItem) {
  return item.tags.filter((tag) => !(isOwnedSource(item) && tag.toLowerCase() === "solvea"));
}

function isOwnedSource(item: PromptItem) {
  return item.source.platform === "Local migration" || item.source.label.toLowerCase().includes("owned");
}

function ownedSourceLabel(locale: Locale) {
  return locale === "zh" ? "自有素材" : "Owned asset";
}

function sourceDisplayLabel(item: PromptItem, locale: Locale) {
  return isOwnedSource(item) ? ownedSourceLabel(locale) : item.source.label;
}

function SourceInfo(props: { copy: CliMediaCopy; item: PromptItem; locale: Locale }) {
  const label = sourceDisplayLabel(props.item, props.locale);
  return (
    <div>
      <p className="text-[11px] font-bold tracking-[0.12em] text-[#62626D] uppercase">{props.copy.source}</p>
      {isOwnedSource(props.item) || !props.item.source.url ? (
        <p className="mt-1 text-sm font-semibold text-[#0B0B0F]">{label}</p>
      ) : (
        <a className="mt-1 inline-flex items-center gap-1 text-sm font-semibold text-[#5b21b6] hover:text-[#0B0B0F]" href={props.item.source.url} target="_blank" rel="noopener noreferrer">
          {label}
          <ExternalLink className="size-3.5" />
        </a>
      )}
    </div>
  );
}

function SourcePanel(props: { copy: CliMediaCopy; item: PromptItem; locale: Locale }) {
  const label = sourceDisplayLabel(props.item, props.locale);
  return (
    <div className="rounded-2xl border border-[#0B0B0F12] bg-white p-5 shadow-sm">
      <p className="text-xs font-black tracking-[0.12em] text-violet-600 uppercase">{props.copy.source}</p>
      <p className="mt-3 text-sm font-semibold text-[#0B0B0F]">{label}</p>
      {isOwnedSource(props.item) || !props.item.source.url ? (
        <p className="mt-1 text-sm text-[#62626D]">{uiCopyByLocale[props.locale].owned}</p>
      ) : (
        <>
          <p className="mt-1 text-sm text-[#62626D]">{props.item.source.platform}</p>
          <a className="mt-4 inline-flex h-10 items-center gap-2 rounded-xl border border-[#0B0B0F14] bg-white px-3 text-sm font-semibold text-[#43434C] hover:border-violet-500/35 hover:text-[#0B0B0F]" href={props.item.source.url} target="_blank" rel="noopener noreferrer">
            <ExternalLink className="size-4" />
            {props.copy.viewSource}
          </a>
        </>
      )}
    </div>
  );
}

function ArtifactPreview(props: { artifact: PromptArtifact; title: string; variant?: "compact" | "detail" | "hero" | "tile" }) {
  const aspect = props.variant === "compact" ? "aspect-[4/3]" : props.variant === "hero" || props.variant === "detail" ? "aspect-[16/9]" : "aspect-[16/10]";
  const mediaFillClass = "mx-auto h-full w-auto max-w-none";
  if (props.artifact.kind === "video") {
    return (
      <div className={`relative flex ${aspect} items-center justify-center overflow-hidden bg-[#211c2d]`}>
        {isVideoFile(props.artifact.url) ? (
          <video
            aria-label={props.artifact.alt}
            autoPlay
            className={mediaFillClass}
            loop
            muted
            playsInline
            poster={props.artifact.poster}
            preload={props.variant === "hero" ? "auto" : "metadata"}
          >
            <source src={props.artifact.url} type={videoMimeType(props.artifact.url)} />
          </video>
        ) : (
          <Image src={props.artifact.poster} alt={props.artifact.alt} width={1600} height={900} sizes="(min-width: 1024px) 50vw, 100vw" className={mediaFillClass} />
        )}
        <div className="pointer-events-none absolute inset-0 ring-1 ring-inset ring-white/10" />
        {!isVideoFile(props.artifact.url) ? (
          <div className="pointer-events-none absolute right-3 bottom-3 flex h-9 w-9 items-center justify-center rounded-full border border-white/55 bg-white/90 text-violet-700 shadow-[0_16px_32px_-20px_rgba(11,11,15,0.55)]">
            <Play className="ml-0.5 size-4 fill-current" />
          </div>
        ) : null}
      </div>
    );
  }

  if (props.artifact.kind === "image") {
    return (
      <div className={`relative flex ${aspect} items-center justify-center overflow-hidden bg-[#211c2d]`}>
        <Image src={props.artifact.url} alt={props.artifact.alt} width={1600} height={1200} sizes="(min-width: 1024px) 50vw, 100vw" className={mediaFillClass} />
      </div>
    );
  }

  if (props.artifact.kind === "storyboard") {
    return (
      <div className={`grid ${aspect} grid-cols-3 gap-1 bg-[#161020] p-2`}>
        {props.artifact.frames.map((frame, index) => (
          <div key={frame} className="rounded-md border border-white/10 bg-white/8 p-2 text-[11px] leading-4 text-violet-100">
            <span className="mb-1 block font-semibold text-violet-300">{index + 1}</span>
            {frame}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className={`${aspect} bg-[#161020] p-5 text-violet-100`}>
      <p className="text-sm font-semibold">{props.title}</p>
    </div>
  );
}

function isVideoFile(url: string) {
  return /\.(mp4|webm|mov)(\?|$)/i.test(url);
}

function videoMimeType(url: string) {
  if (/\.webm(\?|$)/i.test(url)) return "video/webm";
  if (/\.mov(\?|$)/i.test(url)) return "video/quicktime";
  return "video/mp4";
}
