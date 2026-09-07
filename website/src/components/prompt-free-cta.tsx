import { ArrowRight, Sparkles } from "lucide-react";
import type { Locale } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type MediaKind = "image" | "video";

const copy: Record<Locale, { eyebrow: string; title: string; body: string; action: string }> = {
  en: { eyebrow: "Free prompt library", title: "The prompts are free. Your next creation starts here.", body: "Choose a prompt, open it in Flatkey, then adjust the model and settings to create your own result.", action: "Create in Flatkey" },
  zh: { eyebrow: "免费提示词库", title: "提示词免费，生成从这里开始。", body: "选择一个提示词，进入 Flatkey 调整模型和参数，生成属于你的图片或视频。", action: "进入 Flatkey 开始生成" },
  es: { eyebrow: "Biblioteca de prompts gratuita", title: "Los prompts son gratis. Tu próxima creación empieza aquí.", body: "Elige un prompt, ábrelo en Flatkey y ajusta el modelo y los parámetros para crear tu propio resultado.", action: "Crear en Flatkey" },
  fr: { eyebrow: "Bibliothèque de prompts gratuite", title: "Les prompts sont gratuits. Votre prochaine création commence ici.", body: "Choisissez un prompt, ouvrez-le dans Flatkey, puis ajustez le modèle et les paramètres pour créer votre propre résultat.", action: "Créer dans Flatkey" },
  pt: { eyebrow: "Biblioteca de prompts gratuita", title: "Os prompts são gratuitos. Sua próxima criação começa aqui.", body: "Escolha um prompt, abra-o no Flatkey e ajuste o modelo e os parâmetros para criar seu próprio resultado.", action: "Criar no Flatkey" },
  ru: { eyebrow: "Бесплатная библиотека промптов", title: "Промпты бесплатны. Начните создавать прямо сейчас.", body: "Выберите промпт, откройте его во Flatkey и настройте модель и параметры, чтобы получить собственный результат.", action: "Создать во Flatkey" },
  ja: { eyebrow: "無料プロンプトライブラリ", title: "プロンプトは無料。次の作品をここから始めましょう。", body: "プロンプトを選んで Flatkey で開き、モデルと設定を調整して自分だけの画像や動画を生成できます。", action: "Flatkey で生成する" },
  vi: { eyebrow: "Thư viện prompt miễn phí", title: "Prompt hoàn toàn miễn phí. Bắt đầu sáng tạo ngay tại đây.", body: "Chọn một prompt, mở trong Flatkey rồi điều chỉnh mô hình và thông số để tạo kết quả của riêng bạn.", action: "Tạo bằng Flatkey" },
  de: { eyebrow: "Kostenlose Prompt-Bibliothek", title: "Die Prompts sind kostenlos. Ihre nächste Kreation beginnt hier.", body: "Wählen Sie einen Prompt, öffnen Sie ihn in Flatkey und passen Sie Modell und Einstellungen für Ihr eigenes Ergebnis an.", action: "In Flatkey erstellen" },
  id: { eyebrow: "Pustaka prompt gratis", title: "Prompt tersedia gratis. Mulai kreasi berikutnya di sini.", body: "Pilih prompt, buka di Flatkey, lalu sesuaikan model dan pengaturan untuk membuat hasil Anda sendiri.", action: "Buat di Flatkey" },
};

export function PromptFreeCta({ locale, kind }: { locale: Locale; kind?: MediaKind }) {
  const text = copy[locale];
  const params = new URLSearchParams({ lng: locale, source: "prompt-library" });
  if (kind) params.set("generate", kind);

  return (
    <section className="border-t border-[#0B0B0F14] bg-white px-6 py-12 sm:px-8 md:py-16 lg:px-10">
      <div className="relative mx-auto max-w-[1280px] overflow-hidden rounded-[24px] bg-[#0B0B0F] px-7 py-10 text-white shadow-[0_30px_80px_-42px_rgba(31,12,58,.72)] md:px-10 md:py-12">
        <div className="pointer-events-none absolute -top-24 right-0 size-72 rounded-full bg-violet-600/30 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-28 left-1/3 size-64 rounded-full bg-fuchsia-500/15 blur-3xl" />
        <div className="relative flex flex-col gap-7 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-3xl">
            <p className="inline-flex items-center gap-2 text-xs font-black tracking-[0.16em] text-violet-300 uppercase"><Sparkles className="size-3.5" aria-hidden="true" />{text.eyebrow}</p>
            <h2 className="mt-4 text-3xl font-black tracking-[-0.035em] text-white md:text-4xl">{text.title}</h2>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-white/68 md:text-base">{text.body}</p>
          </div>
          <a href={consoleUrl("/playground", params.toString())} className="inline-flex h-11 w-fit shrink-0 items-center gap-2 rounded-full bg-white px-5 text-sm font-extrabold !text-[#0B0B0F] shadow-[0_16px_38px_-22px_rgba(255,255,255,.55)] transition-colors hover:bg-violet-100">
            {text.action}<ArrowRight className="size-4" aria-hidden="true" />
          </a>
        </div>
      </div>
    </section>
  );
}
