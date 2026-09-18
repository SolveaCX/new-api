import type { Locale } from "@/lib/locales";
import { formatResolvedModelDisplayPrice, resolveModelDisplayPrice, type DisplayPricingDimension, type PricingModel } from "@/lib/pricing";

const labels: Record<Locale, readonly string[]> = {
  en: ["Configured reference", "Current public price", "Input", "Output", "Cache read", "Cache write", "Image", "Audio input", "Audio output", "Per second", "Per request"],
  zh: ["配置参考价", "当前公开价", "输入", "输出", "缓存读取", "缓存写入", "图像", "音频输入", "音频输出", "每秒", "每次请求"],
  es: ["Precio de referencia configurado", "Precio público actual", "Entrada", "Salida", "Lectura de caché", "Escritura de caché", "Imagen", "Audio de entrada", "Audio de salida", "Por segundo", "Por solicitud"],
  fr: ["Tarif de référence configuré", "Tarif public actuel", "Entrée", "Sortie", "Lecture du cache", "Écriture du cache", "Image", "Audio en entrée", "Audio en sortie", "Par seconde", "Par requête"],
  pt: ["Preço de referência configurado", "Preço público atual", "Entrada", "Saída", "Leitura de cache", "Gravação de cache", "Imagem", "Áudio de entrada", "Áudio de saída", "Por segundo", "Por solicitação"],
  ru: ["Настроенная базовая цена", "Текущая публичная цена", "Ввод", "Вывод", "Чтение кеша", "Запись кеша", "Изображение", "Аудиоввод", "Аудиовывод", "За секунду", "За запрос"],
  ja: ["設定された基準料金", "現在の公開料金", "入力", "出力", "キャッシュ読み取り", "キャッシュ書き込み", "画像", "音声入力", "音声出力", "1秒あたり", "リクエストあたり"],
  vi: ["Giá tham chiếu đã cấu hình", "Giá công khai hiện tại", "Đầu vào", "Đầu ra", "Đọc bộ nhớ đệm", "Ghi bộ nhớ đệm", "Hình ảnh", "Âm thanh đầu vào", "Âm thanh đầu ra", "Mỗi giây", "Mỗi yêu cầu"],
  de: ["Konfigurierter Referenzpreis", "Aktueller öffentlicher Preis", "Eingabe", "Ausgabe", "Cache-Lesen", "Cache-Schreiben", "Bild", "Audio-Eingabe", "Audio-Ausgabe", "Pro Sekunde", "Pro Anfrage"],
  id: ["Harga referensi yang dikonfigurasi", "Harga publik saat ini", "Input", "Output", "Baca cache", "Tulis cache", "Gambar", "Input audio", "Output audio", "Per detik", "Per permintaan"],
};
const dimensions: DisplayPricingDimension[] = ["input", "output", "cache", "create_cache", "image", "audio_input", "audio_output", "second", "request"];

export function CollectionDiscountPrices({ model, locale }: { model: PricingModel; locale: Locale }) {
  const copy = labels[locale];
  const rows = dimensions.flatMap((dimension, index) => {
    const pair = model.display_pricing?.prices[dimension];
    if (typeof pair?.configured !== "number" || !Number.isFinite(pair.configured) || pair.configured < 0 ||
        typeof pair.plg !== "number" || !Number.isFinite(pair.plg) || pair.plg < 0 || pair.plg >= pair.configured) return [];
    const reference = resolveModelDisplayPrice(model, dimension, "configured");
    const current = resolveModelDisplayPrice(model, dimension, "plg");
    if (!reference || !current) return [];
    return [{ dimension, label: copy[index + 2], reference: formatResolvedModelDisplayPrice(reference), current: formatResolvedModelDisplayPrice(current) }];
  });
  if (!rows.length) return null;
  return <dl className="mt-6 grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-3">
    {rows.map((row) => <div key={row.dimension} className="min-w-0 rounded-xl border border-[#EDE8F5] bg-[#FAF8FD] p-4 sm:p-5">
      <dt className="mb-4 font-semibold text-[#201D28]">{row.label}</dt>
      <dd className="space-y-3">
        <div><span className="block text-xs leading-5 text-[#777180]">{copy[1]}</span><span className="mt-1 block break-words text-lg font-semibold tabular-nums tracking-tight text-[#6D28D9]">{row.current}</span></div>
        <div className="border-t border-[#EDE8F5] pt-3"><span className="block text-xs leading-5 text-[#777180]">{copy[0]}</span><s className="mt-1 block text-sm tabular-nums text-[#777180]">{row.reference}</s></div>
      </dd>
    </div>)}
  </dl>;
}
