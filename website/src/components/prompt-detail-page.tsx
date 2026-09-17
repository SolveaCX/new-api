import Link from "next/link";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { CdnFallbackImage } from "@/components/cdn-media";
import { PromptLibraryVideo } from "@/components/model-landing-page";
import { PromptDetailOverviewCta, PromptDetailWorkbench } from "@/components/prompt-detail-workbench";
import { SiteShell } from "@/components/site-shell";
import { type Locale, localizePath } from "@/lib/locales";
import { getPromptDetailCopy, promptDetailPath, promptExcerpt } from "@/lib/prompt-detail";
import type { PromptDetail } from "@/lib/prompt-detail-data";
import { getPromptDetailSeoText } from "@/lib/prompt-detail-seo";

const GUIDE: Record<Locale, { title: string; steps: readonly [string, string, string] }> = {
  en: { title: "How to use this prompt", steps: ["Review the matching example and full brief.", "Edit the English prompt to fit your project.", "Sign in and open the edited prompt in Playground."] },
  zh: { title: "如何使用这条提示词", steps: ["先查看对应的生成示例与完整描述。", "在左侧编辑英文提示词，调整到符合你的项目。", "登录后带着修改内容进入 Playground 生成。"] },
  es: { title: "Cómo usar este prompt", steps: ["Revisa el ejemplo y las instrucciones completas.", "Edita el prompt en inglés según tu proyecto.", "Inicia sesión y abre el prompt editado en Playground."] },
  fr: { title: "Comment utiliser ce prompt", steps: ["Consultez l’exemple et le brief complet.", "Adaptez le prompt anglais à votre projet.", "Connectez-vous pour l’ouvrir dans Playground."] },
  pt: { title: "Como usar este prompt", steps: ["Veja o exemplo e a descrição completa.", "Edite o prompt em inglês para seu projeto.", "Entre na conta e abra o prompt editado no Playground."] },
  ru: { title: "Как использовать промпт", steps: ["Изучите пример и полное описание.", "Измените английский промпт под свой проект.", "Войдите и откройте его в Playground."] },
  ja: { title: "このプロンプトの使い方", steps: ["作例と詳しい説明を確認します。", "英語のプロンプトを目的に合わせて編集します。", "ログインして編集内容を Playground で開きます。"] },
  vi: { title: "Cách dùng prompt này", steps: ["Xem ví dụ và mô tả đầy đủ.", "Chỉnh sửa prompt tiếng Anh cho dự án của bạn.", "Đăng nhập và mở prompt đã sửa trong Playground."] },
  de: { title: "So verwenden Sie diesen Prompt", steps: ["Beispiel und vollständige Beschreibung ansehen.", "Den englischen Prompt für Ihr Projekt anpassen.", "Anmelden und den bearbeiteten Prompt in Playground öffnen."] },
  id: { title: "Cara menggunakan prompt ini", steps: ["Lihat contoh dan deskripsi lengkap.", "Edit prompt bahasa Inggris untuk proyek Anda.", "Masuk lalu buka prompt yang diedit di Playground."] },
};

const CTA: Record<Locale, { title: string; description: (model: string) => string; start: string }> = {
  en: { title: "Made for creators. Make your next idea real.", description: (model) => `Explore curated ${model} prompts and matching examples on Flatkey. Open your console to choose a model and start creating.`, start: "Create with Flatkey" },
  zh: { title: "为创作者而生，让好创意直接生成。", description: (model) => `在 Flatkey 探索精选的 ${model} 提示词与对应生成示例。进入控制台，选择模型，开始创作你的作品。`, start: "开始用 Flatkey 创作" },
  es: { title: "Creado para quienes imaginan y crean.", description: (model) => `Explora prompts seleccionados de ${model} y ejemplos en Flatkey. Abre la consola, elige un modelo y empieza a crear.`, start: "Crear con Flatkey" },
  fr: { title: "Pensé pour les créateurs et leurs idées.", description: (model) => `Explorez les prompts ${model} sélectionnés et leurs exemples sur Flatkey. Ouvrez la console, choisissez un modèle et commencez à créer.`, start: "Créer avec Flatkey" },
  pt: { title: "Feito para criadores. Dê vida à próxima ideia.", description: (model) => `Explore prompts selecionados de ${model} e exemplos no Flatkey. Abra o console, escolha um modelo e comece a criar.`, start: "Criar com Flatkey" },
  ru: { title: "Для творцов, которые воплощают идеи.", description: (model) => `Изучите подборку промптов ${model} и примеры в Flatkey. Откройте консоль, выберите модель и начните создавать.`, start: "Создать в Flatkey" },
  ja: { title: "クリエイターの発想を、作品に。", description: (model) => `Flatkey で厳選した ${model} のプロンプトと作例を確認。コンソールでモデルを選び、創作を始めましょう。`, start: "Flatkey で作成" },
  vi: { title: "Dành cho người sáng tạo. Biến ý tưởng thành tác phẩm.", description: (model) => `Khám phá prompt ${model} được tuyển chọn cùng ví dụ trên Flatkey. Mở bảng điều khiển, chọn mô hình và bắt đầu sáng tạo.`, start: "Sáng tạo với Flatkey" },
  de: { title: "Für Kreative. Mach deine nächste Idee wahr.", description: (model) => `Entdecke ausgewählte ${model}-Prompts und passende Beispiele bei Flatkey. Öffne die Konsole, wähle ein Modell und leg los.`, start: "Mit Flatkey erstellen" },
  id: { title: "Untuk kreator. Wujudkan ide berikutnya.", description: (model) => `Jelajahi prompt ${model} pilihan dan contoh hasil di Flatkey. Buka konsol, pilih model, dan mulai berkarya.`, start: "Berkarya dengan Flatkey" },
};

export function PromptDetailPage({ detail, locale }: { detail: PromptDetail; locale: Locale }) {
  const copy = getPromptDetailCopy(locale);
  const guide = GUIDE[locale];
  const cta = CTA[locale];
  const seo = getPromptDetailSeoText(detail, locale);
  const pathname = `/models/${detail.modelSlug}/prompts/${detail.id}`;
  const modelPath = localizePath(`/models/${detail.modelSlug}`, locale);
  const siblings = detail.siblings.filter((item) => item.id !== detail.id);

  return (
    <SiteShell locale={locale} pathname={pathname}>
      <main className="min-h-screen bg-[linear-gradient(180deg,#faf8ff_0%,#fff_420px)] pb-24 text-[#171a21]">
        <div className="mx-auto max-w-[1320px] px-5 pt-10 sm:px-8 lg:px-12 lg:pt-14">
          <nav aria-label="Breadcrumb" className="flex flex-wrap items-center gap-2 text-xs font-semibold text-[#706a7b]">
            <Link href={`${modelPath}#prompt-library`} className="inline-flex items-center gap-1.5 transition hover:text-[#6d28d9]"><ArrowLeft size={14} />{copy.back}</Link>
            <span aria-hidden="true">/</span>
            <span>{copy.eyebrow}</span>
          </nav>

          <header className="mt-8 max-w-4xl">
            <h1 className="text-[clamp(2.15rem,4.2vw,4.25rem)] font-extrabold leading-[1.09] tracking-[-.045em]">{detail.label}</h1>
            <p className="mt-4 max-w-2xl text-base leading-8 text-[#686574]">{seo.description}</p>
          </header>

          <div className="mt-7 flex flex-wrap items-center gap-2">
            <span className="rounded-full border border-[#e7e2ef] bg-white px-4 py-2 text-xs font-bold text-[#554f61]">{detail.kind === "video" ? copy.video : copy.image}</span>
            <span className="rounded-full border border-[#e7e2ef] bg-white px-4 py-2 text-xs font-bold text-[#554f61]">{copy.ratio} · {detail.ratio}</span>
            {detail.duration ? <span className="rounded-full border border-[#e7e2ef] bg-white px-4 py-2 text-xs font-bold text-[#554f61]">{copy.duration} · {detail.duration}s</span> : null}
          </div>

          <div className="mt-6">
            <PromptDetailWorkbench key={`${detail.modelId}:${detail.id}`} detail={detail} locale={locale} />
          </div>

          <section className="mt-16" aria-labelledby="prompt-guide-title">
            <h2 id="prompt-guide-title" className="text-2xl font-extrabold tracking-tight sm:text-3xl">{guide.title}</h2>
            <ol className="mt-6 grid gap-4 md:grid-cols-3">
              {guide.steps.map((step, index) => (
                <li key={step} className="rounded-2xl border border-[#e7e3eb] bg-white p-6">
                  <span className="grid size-9 place-items-center rounded-full bg-[#171821] text-sm font-bold text-white">{index + 1}</span>
                  <p className="mt-5 text-sm leading-7 text-[#625d6c]">{step}</p>
                </li>
              ))}
            </ol>
          </section>

          <section className="mt-20" aria-labelledby="related-prompts-title">
            <div className="flex items-end justify-between gap-5">
              <h2 id="related-prompts-title" className="text-2xl font-extrabold tracking-tight sm:text-3xl">{copy.more}</h2>
              <Link href={`${modelPath}#prompt-library`} className="inline-flex items-center gap-1.5 text-sm font-bold text-[#625d6c] transition hover:text-[#6d28d9]">{copy.back}<ArrowRight size={15} /></Link>
            </div>
            <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {siblings.map((item, index) => (
                <article key={item.id} className="group flex flex-col overflow-hidden rounded-2xl border border-[#e8e2f0] bg-white shadow-sm transition hover:-translate-y-1 hover:border-[#cdbcf3] hover:shadow-[0_20px_42px_-30px_rgba(76,29,149,.5)]">
                  <div className="relative flex aspect-video items-center justify-center overflow-hidden bg-[#10131a]">
                    {item.video ? (
                      <PromptLibraryVideo src={item.video} poster={item.poster} fallbackPoster={item.fallbackPoster} priority={index < 3} locale={locale} className="absolute inset-0 h-full w-full object-contain" />
                    ) : item.poster ? (
                      <Link href={promptDetailPath(detail.modelSlug, item.id, locale)} className="absolute inset-0 focus-visible:outline-3 focus-visible:outline-offset-[-3px] focus-visible:outline-[#7c3aed]" aria-label={item.label}>
                        <CdnFallbackImage src={item.poster} fallbackSrc={item.fallbackPoster} alt="" className="h-full w-full object-contain transition duration-300 group-hover:scale-[1.03]" />
                      </Link>
                    ) : null}
                  </div>
                  <div className="flex flex-1 flex-col px-4 pb-4 pt-4">
                    <h3 className="text-sm font-bold leading-5">{item.label}</h3>
                    <p className="mb-4 mt-2 line-clamp-3 text-sm leading-6 text-[#686574]">{promptExcerpt(item.prompt)}</p>
                    <a href={promptDetailPath(detail.modelSlug, item.id, locale)} className="prompt-related-cta mt-auto inline-flex min-h-10 w-full items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-sm font-bold shadow-sm transition-colors">{copy.open}<ArrowRight size={16} aria-hidden="true" /></a>
                  </div>
                </article>
              ))}
            </div>
          </section>

          <section className="prompt-detail-conversion relative mt-16 overflow-hidden rounded-3xl px-7 py-9 sm:px-10 sm:py-12" aria-labelledby="prompt-detail-cta-title">
            <div className="relative z-10 flex flex-col items-start justify-between gap-7 lg:flex-row lg:items-center">
              <div className="max-w-2xl">
                <p className="text-xs font-bold tracking-[.14em] text-[#c8b5ff] uppercase">{detail.modelName} · {seo.kindLabel}</p>
                <h2 id="prompt-detail-cta-title" className="mt-3 text-2xl font-extrabold tracking-tight sm:text-3xl">{cta.title}</h2>
                <p className="mt-3 text-sm leading-7 text-[#dbd5e9] sm:text-base">{cta.description(detail.modelName)}</p>
              </div>
              <div className="flex w-full shrink-0 sm:w-auto">
                <PromptDetailOverviewCta label={cta.start} locale={locale} />
              </div>
            </div>
          </section>
        </div>
      </main>
    </SiteShell>
  );
}
