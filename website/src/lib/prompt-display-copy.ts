import type { Locale } from "./locales";
import type { PromptItem } from "./prompt-library";

type DisplayCopy = { title: string; summary: string };

const titleCopy: Record<string, Partial<Record<Locale, string>>> = {
  "Comic Page Coloring & Translation": { zh: "漫画页面上色与翻译", es: "Colorear y traducir una página de cómic", fr: "Colorisation et traduction d’une page de BD", pt: "Colorir e traduzir uma página de quadrinhos", ru: "Раскраска и перевод страницы комикса", ja: "漫画ページの着色と翻訳", vi: "Tô màu và dịch trang truyện tranh", de: "Comicseite kolorieren und übersetzen", id: "Mewarnai dan menerjemahkan halaman komik" },
  "Pet Brand Collaboration Poster": { zh: "宠物品牌联名海报", es: "Póster de colaboración con una marca de mascotas", fr: "Affiche de collaboration avec une marque pour animaux", pt: "Pôster de colaboração com marca pet", ru: "Постер коллаборации бренда с питомцем", ja: "ペットブランドのコラボポスター", vi: "Poster hợp tác thương hiệu thú cưng", de: "Kooperationsposter für eine Haustiermarke", id: "Poster kolaborasi merek hewan peliharaan" },
  "Pro Instagram Photo Enhancement": { zh: "专业 Instagram 照片增强", es: "Mejora profesional de fotos para Instagram", fr: "Amélioration photo professionnelle pour Instagram", pt: "Aprimoramento profissional de foto para Instagram", ru: "Профессиональная обработка фото для Instagram", ja: "Instagram 写真のプロ向上加工", vi: "Nâng cấp ảnh Instagram chuyên nghiệp", de: "Professionelle Instagram-Fotoverbesserung", id: "Peningkatan foto Instagram profesional" },
};

const tagCopy: Record<Locale, Record<string, string>> = {
  en: { photography: "Photography", gaming: "Gaming", "ui-ux": "UI/UX", "video-animation": "Video & animation", "typography-poster": "Typography & posters", infographic: "Infographics", "character-consistency": "Character consistency", "image-editing": "Image editing", ecommerce: "Ecommerce", product: "Product", ugc: "UGC", ads: "Ads", localization: "Localization", launch: "Launch", i2v: "Image to video", t2v: "Text to video", character: "Character", "model-sheet": "Model sheet", "scene-bible": "Scene bible", storyboard: "Storyboard", commerce: "Commerce", hero: "Hero visual" },
  zh: { photography: "摄影", gaming: "游戏", "ui-ux": "UI/UX", "video-animation": "视频与动画", "typography-poster": "字体与海报", infographic: "信息图", "character-consistency": "角色一致性", "image-editing": "图像编辑", ecommerce: "电商", product: "产品", ugc: "UGC", ads: "广告", localization: "本地化", launch: "发布", i2v: "图生视频", t2v: "文生视频", character: "角色", "model-sheet": "角色设定", "scene-bible": "场景设定", storyboard: "分镜", commerce: "商业视觉", hero: "首图视觉" },
  es: { photography: "Fotografía", gaming: "Videojuegos", "ui-ux": "UI/UX", "video-animation": "Vídeo y animación", "typography-poster": "Tipografía y carteles", infographic: "Infografías", "character-consistency": "Consistencia de personajes", "image-editing": "Edición de imagen", ecommerce: "Comercio electrónico", product: "Producto", ugc: "UGC", ads: "Anuncios", localization: "Localización", launch: "Lanzamiento", i2v: "Imagen a vídeo", t2v: "Texto a vídeo", character: "Personaje", "model-sheet": "Hoja de modelo", "scene-bible": "Biblia de escena", storyboard: "Storyboard", commerce: "Comercio", hero: "Visual principal" },
  fr: { photography: "Photographie", gaming: "Jeux vidéo", "ui-ux": "UI/UX", "video-animation": "Vidéo et animation", "typography-poster": "Typographie et affiches", infographic: "Infographies", "character-consistency": "Cohérence des personnages", "image-editing": "Retouche d’image", ecommerce: "E-commerce", product: "Produit", ugc: "UGC", ads: "Publicités", localization: "Localisation", launch: "Lancement", i2v: "Image vers vidéo", t2v: "Texte vers vidéo", character: "Personnage", "model-sheet": "Planche de modèle", "scene-bible": "Bible de scène", storyboard: "Storyboard", commerce: "Commerce", hero: "Visuel principal" },
  pt: { photography: "Fotografia", gaming: "Jogos", "ui-ux": "UI/UX", "video-animation": "Vídeo e animação", "typography-poster": "Tipografia e pôsteres", infographic: "Infográficos", "character-consistency": "Consistência de personagem", "image-editing": "Edição de imagem", ecommerce: "E-commerce", product: "Produto", ugc: "UGC", ads: "Anúncios", localization: "Localização", launch: "Lançamento", i2v: "Imagem para vídeo", t2v: "Texto para vídeo", character: "Personagem", "model-sheet": "Folha de modelo", "scene-bible": "Bíblia de cena", storyboard: "Storyboard", commerce: "Comércio", hero: "Visual principal" },
  ru: { photography: "Фотография", gaming: "Игры", "ui-ux": "UI/UX", "video-animation": "Видео и анимация", "typography-poster": "Типографика и постеры", infographic: "Инфографика", "character-consistency": "Согласованность персонажа", "image-editing": "Редактирование изображений", ecommerce: "E-commerce", product: "Продукт", ugc: "UGC", ads: "Реклама", localization: "Локализация", launch: "Запуск", i2v: "Изображение в видео", t2v: "Текст в видео", character: "Персонаж", "model-sheet": "Лист персонажа", "scene-bible": "Библия сцены", storyboard: "Раскадровка", commerce: "Коммерция", hero: "Главный визуал" },
  ja: { photography: "写真", gaming: "ゲーム", "ui-ux": "UI/UX", "video-animation": "動画・アニメーション", "typography-poster": "タイポグラフィ・ポスター", infographic: "インフォグラフィック", "character-consistency": "キャラクターの一貫性", "image-editing": "画像編集", ecommerce: "EC", product: "商品", ugc: "UGC", ads: "広告", localization: "ローカライズ", launch: "ローンチ", i2v: "画像から動画", t2v: "テキストから動画", character: "キャラクター", "model-sheet": "モデルシート", "scene-bible": "シーン設定", storyboard: "絵コンテ", commerce: "商用ビジュアル", hero: "メインビジュアル" },
  vi: { photography: "Nhiếp ảnh", gaming: "Trò chơi", "ui-ux": "UI/UX", "video-animation": "Video và hoạt hình", "typography-poster": "Kiểu chữ và áp phích", infographic: "Đồ họa thông tin", "character-consistency": "Nhất quán nhân vật", "image-editing": "Chỉnh sửa ảnh", ecommerce: "Thương mại điện tử", product: "Sản phẩm", ugc: "UGC", ads: "Quảng cáo", localization: "Bản địa hóa", launch: "Ra mắt", i2v: "Ảnh thành video", t2v: "Văn bản thành video", character: "Nhân vật", "model-sheet": "Bảng nhân vật", "scene-bible": "Bảng thiết lập cảnh", storyboard: "Storyboard", commerce: "Hình ảnh thương mại", hero: "Hình ảnh chính" },
  de: { photography: "Fotografie", gaming: "Gaming", "ui-ux": "UI/UX", "video-animation": "Video & Animation", "typography-poster": "Typografie & Poster", infographic: "Infografiken", "character-consistency": "Charakterkonsistenz", "image-editing": "Bildbearbeitung", ecommerce: "E-Commerce", product: "Produkt", ugc: "UGC", ads: "Anzeigen", localization: "Lokalisierung", launch: "Launch", i2v: "Bild zu Video", t2v: "Text zu Video", character: "Charakter", "model-sheet": "Modellbogen", "scene-bible": "Szenenübersicht", storyboard: "Storyboard", commerce: "Commerce", hero: "Hero-Visual" },
  id: { photography: "Fotografi", gaming: "Game", "ui-ux": "UI/UX", "video-animation": "Video & animasi", "typography-poster": "Tipografi & poster", infographic: "Infografik", "character-consistency": "Konsistensi karakter", "image-editing": "Pengeditan gambar", ecommerce: "E-commerce", product: "Produk", ugc: "UGC", ads: "Iklan", localization: "Lokalisasi", launch: "Peluncuran", i2v: "Gambar ke video", t2v: "Teks ke video", character: "Karakter", "model-sheet": "Lembar model", "scene-bible": "Panduan adegan", storyboard: "Storyboard", commerce: "Visual komersial", hero: "Visual utama" },
};

const fallbackSummary: Record<Locale, Record<"image" | "video", string>> = {
  en: { image: "A real image result with the full prompt, model, and source attached.", video: "A real video result with the full prompt, model, and source attached." },
  zh: { image: "真实图片案例，附有完整提示词、适用模型和来源。", video: "真实视频案例，附有完整提示词、适用模型和来源。" },
  es: { image: "Un resultado de imagen real con el prompt completo, el modelo y la fuente.", video: "Un resultado de vídeo real con el prompt completo, el modelo y la fuente." },
  fr: { image: "Un résultat d’image réel avec le prompt complet, le modèle et la source.", video: "Un résultat vidéo réel avec le prompt complet, le modèle et la source." },
  pt: { image: "Um resultado de imagem real com prompt completo, modelo e fonte.", video: "Um resultado de vídeo real com prompt completo, modelo e fonte." },
  ru: { image: "Реальный результат для изображения с полным промптом, моделью и источником.", video: "Реальный видеорезультат с полным промптом, моделью и источником." },
  ja: { image: "完成画像に、プロンプト全文・モデル・出典を添えた実例です。", video: "完成動画に、プロンプト全文・モデル・出典を添えた実例です。" },
  vi: { image: "Ví dụ ảnh thực tế kèm prompt đầy đủ, model và nguồn.", video: "Ví dụ video thực tế kèm prompt đầy đủ, model và nguồn." },
  de: { image: "Ein echtes Bildergebnis mit vollständigem Prompt, Modell und Quelle.", video: "Ein echtes Videoergebnis mit vollständigem Prompt, Modell und Quelle." },
  id: { image: "Contoh hasil gambar nyata dengan prompt lengkap, model, dan sumber.", video: "Contoh hasil video nyata dengan prompt lengkap, model, dan sumber." },
};

export function localizePromptTag(tag: string, locale: Locale): string {
  return tagCopy[locale][tag.toLowerCase()] ?? tag;
}

export function getPromptDisplayCopy(item: PromptItem, locale: Locale): DisplayCopy {
  const title = titleCopy[item.title.en]?.[locale] ?? item.title[locale] ?? item.title.en;
  const summary = item.summary[locale] ?? item.summary.en;
  const hasLocalizedSummary = locale === "en" || (summary.trim() && summary !== item.summary.en);
  const kind = item.category === "video" ? "video" : "image";
  return { title, summary: hasLocalizedSummary ? summary : fallbackSummary[locale][kind] };
}
