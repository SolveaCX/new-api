"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowRight, Check, Copy, Play, X } from "lucide-react";
import { CdnFallbackImage, CdnFallbackVideo } from "@/components/cdn-media";
import { MediaPromptEditor } from "@/components/model-landing-page";
import {
  clearConsoleSessionHint,
  hasConsoleSessionHint,
  isVerifiedConsoleUserPayload,
  rememberConsoleSessionHint,
} from "@/lib/console-session-hint";
import type { Locale } from "@/lib/locales";
import { modelLandingCopy, type ModelLandingKey } from "@/lib/model-landing";
import { consoleUrl } from "@/lib/origins";
import { getPromptDetailCopy } from "@/lib/prompt-detail";
import type { PromptDetail } from "@/lib/prompt-detail-data";

const UI: Record<Locale, { input: string; output: string; prompt: string; loginTitle: string; loginMessage: string; keepEditing: string; signIn: string; close: string }> = {
  en: { input: "Input", output: "Output", prompt: "Prompt · English", loginTitle: "Sign in to generate", loginMessage: "Your edits are ready. Sign in to open Flatkey Playground with this prompt.", keepEditing: "Keep editing", signIn: "Sign in", close: "Close" },
  zh: { input: "输入", output: "输出", prompt: "提示词 · 英文", loginTitle: "登录后开始生成", loginMessage: "你的编辑已保留。登录后会带着当前提示词进入 Flatkey Playground。", keepEditing: "继续编辑", signIn: "前往登录", close: "关闭" },
  es: { input: "Entrada", output: "Salida", prompt: "Prompt · inglés", loginTitle: "Inicia sesión para generar", loginMessage: "Tus cambios están listos. Inicia sesión para abrir este prompt en Flatkey Playground.", keepEditing: "Seguir editando", signIn: "Iniciar sesión", close: "Cerrar" },
  fr: { input: "Entrée", output: "Sortie", prompt: "Prompt · anglais", loginTitle: "Connectez-vous pour générer", loginMessage: "Vos modifications sont prêtes. Connectez-vous pour ouvrir ce prompt dans Flatkey Playground.", keepEditing: "Continuer à modifier", signIn: "Se connecter", close: "Fermer" },
  pt: { input: "Entrada", output: "Saída", prompt: "Prompt · inglês", loginTitle: "Entre para gerar", loginMessage: "Suas edições estão prontas. Entre para abrir este prompt no Flatkey Playground.", keepEditing: "Continuar editando", signIn: "Entrar", close: "Fechar" },
  ru: { input: "Ввод", output: "Результат", prompt: "Промпт · английский", loginTitle: "Войдите для генерации", loginMessage: "Изменения сохранены. Войдите, чтобы открыть промпт в Flatkey Playground.", keepEditing: "Продолжить редактирование", signIn: "Войти", close: "Закрыть" },
  ja: { input: "入力", output: "出力", prompt: "プロンプト · 英語", loginTitle: "ログインして生成", loginMessage: "編集内容は保持されています。ログイン後、このプロンプトを Flatkey Playground で開きます。", keepEditing: "編集を続ける", signIn: "ログイン", close: "閉じる" },
  vi: { input: "Đầu vào", output: "Đầu ra", prompt: "Prompt · tiếng Anh", loginTitle: "Đăng nhập để tạo", loginMessage: "Nội dung chỉnh sửa đã sẵn sàng. Đăng nhập để mở prompt trong Flatkey Playground.", keepEditing: "Tiếp tục chỉnh sửa", signIn: "Đăng nhập", close: "Đóng" },
  de: { input: "Eingabe", output: "Ausgabe", prompt: "Prompt · Englisch", loginTitle: "Zum Erstellen anmelden", loginMessage: "Ihre Änderungen sind bereit. Melden Sie sich an, um den Prompt in Flatkey Playground zu öffnen.", keepEditing: "Weiter bearbeiten", signIn: "Anmelden", close: "Schließen" },
  id: { input: "Masukan", output: "Keluaran", prompt: "Prompt · bahasa Inggris", loginTitle: "Masuk untuk membuat", loginMessage: "Edit Anda sudah siap. Masuk untuk membuka prompt ini di Flatkey Playground.", keepEditing: "Lanjut mengedit", signIn: "Masuk", close: "Tutup" },
};

type ReferenceImageDraft = { id: string; name: string; size: number; type: string; previewUrl: string };
type FieldValues = Record<string, string | number | boolean>;

function initialFieldValues(detail: PromptDetail): FieldValues {
  const values: FieldValues = Object.fromEntries(detail.generator.fields.map((field) => [field.name, field.defaultValue]));
  if (detail.generator.videoModes?.length) {
    values.video_mode = detail.generator.defaultVideoMode
      ?? detail.generator.videoModes.find((option) => option.supported)?.value
      ?? detail.generator.videoModes[0].value;
  }
  const ratioField = detail.generator.fields.find((field) => field.name === "ratio");
  if (ratioField?.options?.includes(detail.ratio)) values.ratio = detail.ratio;
  const aspectRatioField = detail.generator.fields.find((field) => field.name === "aspect_ratio");
  if (aspectRatioField?.options?.includes(detail.ratio)) values.aspect_ratio = detail.ratio;
  const requestedRatio = detail.ratio.match(/^(\d+):(\d+)$/);
  const sizeField = detail.generator.fields.find((field) => field.name === "size");
  if (detail.kind === "image" && requestedRatio && sizeField?.options) {
    const target = Number(requestedRatio[1]) / Number(requestedRatio[2]);
    const sizes = sizeField.options
      .map((option) => ({ option, dimensions: option.match(/^(\d+)x(\d+)$/) }))
      .filter((entry) => entry.dimensions);
    const closest = sizes.sort((left, right) => {
      const leftRatio = Number(left.dimensions![1]) / Number(left.dimensions![2]);
      const rightRatio = Number(right.dimensions![1]) / Number(right.dimensions![2]);
      return Math.abs(Math.log(leftRatio / target)) - Math.abs(Math.log(rightRatio / target));
    })[0];
    if (closest) values.size = closest.option;
  }
  const durationField = detail.generator.fields.find((field) => field.name === "duration");
  if (durationField && detail.duration && detail.duration >= (durationField.min ?? 0) && detail.duration <= (durationField.max ?? Infinity)
    && (!durationField.options || durationField.options.includes(String(detail.duration)))) {
    values.duration = detail.duration;
  }
  return values;
}

function playgroundHref(detail: PromptDetail, locale: Locale, prompt: string, fields: FieldValues, referenceImages: ReferenceImageDraft[]): string {
  const draft = {
    source: "prompt_detail",
    model: detail.modelId,
    slug: detail.modelSlug,
    mediaKind: detail.kind,
    endpoint: detail.generator.endpoint,
    storageKey: detail.generator.storageKey,
    prompt,
    fields,
    referenceImages: referenceImages.map(({ name, size, type }) => ({ name, size, type })),
    locale,
  };
  const params = new URLSearchParams({
    model: detail.modelId,
    prompt,
    lng: locale,
    draft: JSON.stringify(draft),
    generate: detail.kind,
  });
  return consoleUrl("/playground", params.toString());
}

export function PromptDetailWorkbench({ detail, locale }: { detail: PromptDetail; locale: Locale }) {
  const copy = getPromptDetailCopy(locale);
  const ui = UI[locale];
  const [prompt, setPrompt] = useState(detail.prompt);
  const [fieldValues, setFieldValues] = useState<FieldValues>(() => initialFieldValues(detail));
  const [referenceImages, setReferenceImages] = useState<ReferenceImageDraft[]>([]);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const [checking, setChecking] = useState(false);
  const [showLogin, setShowLogin] = useState(false);
  const [isPlaying, setIsPlaying] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);
  const ratioParts = detail.ratio.match(/^(\d+):(\d+)$/);
  const isTallMedia = ratioParts ? Number(ratioParts[1]) <= Number(ratioParts[2]) : false;
  const previewClassName = isTallMedia ? "h-[520px] sm:h-[620px]" : "aspect-video";
  const mediaClassName = isTallMedia ? "block h-full w-auto max-w-full object-contain" : "block h-full w-full object-contain";
  const resumeVideo = () => {
    if (videoRef.current?.paused) void videoRef.current.play().catch(() => undefined);
  };

  useEffect(() => {
    if (!showLogin) return;
    const onEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setShowLogin(false);
    };
    document.addEventListener("keydown", onEscape);
    return () => document.removeEventListener("keydown", onEscape);
  }, [showLogin]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !detail.video) return;
    const start = () => { if (video.paused) void video.play().catch(() => undefined); };
    start();
    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) start();
    });
    observer.observe(video);
    document.addEventListener("visibilitychange", start);
    return () => {
      observer.disconnect();
      document.removeEventListener("visibilitychange", start);
    };
  }, [detail.video]);

  const copyPrompt = async () => {
    try {
      if (!navigator.clipboard?.writeText) throw new Error("Clipboard API unavailable");
      await navigator.clipboard.writeText(prompt);
      setCopyState("copied");
    } catch {
      const textarea = document.createElement("textarea");
      textarea.value = prompt;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      let copied = false;
      try {
        textarea.focus();
        textarea.select();
        copied = document.execCommand("copy");
      } catch {
        copied = false;
      } finally {
        textarea.remove();
      }
      setCopyState(copied ? "copied" : "failed");
    }
    window.setTimeout(() => setCopyState("idle"), 2200);
  };

  const run = async () => {
    if (!prompt.trim() || checking) return;
    setChecking(true);
    let signedIn = false;
    try {
      const response = await fetch("/api/current-user", {
        cache: "no-store",
        credentials: "same-origin",
        headers: { accept: "application/json" },
      });
      if (response.ok) {
        signedIn = isVerifiedConsoleUserPayload(await response.json());
        if (signedIn) rememberConsoleSessionHint();
      } else if (response.status === 401 || response.status === 403) {
        clearConsoleSessionHint();
      } else {
        signedIn = hasConsoleSessionHint();
      }
    } catch {
      signedIn = hasConsoleSessionHint();
    }
    setChecking(false);
    if (signedIn) {
      window.location.assign(playgroundHref(detail, locale, prompt.trim(), fieldValues, referenceImages));
    } else {
      setShowLogin(true);
    }
  };

  const loginRedirect = new URL(playgroundHref(detail, locale, prompt.trim(), fieldValues, referenceImages));
  const signInHref = consoleUrl("/sign-in", new URLSearchParams({ redirect: `${loginRedirect.pathname}${loginRedirect.search}` }).toString());

  return (
    <>
      <div className="grid items-start gap-5 lg:grid-cols-2">
        <section className="min-w-0 rounded-2xl border border-[#e7e3eb] bg-white">
          <div className="flex min-h-16 items-center justify-between gap-4 px-6 pt-5 sm:px-8">
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <h2 className="text-xs font-bold tracking-[.08em] text-[#24212a] uppercase">{ui.input}</h2>
              <span className="text-xs font-medium text-[#807a88]">{copy.fullPrompt}</span>
            </div>
            <button type="button" onClick={() => void copyPrompt()} className="inline-flex min-h-10 items-center gap-2 rounded-xl border border-[#e4deed] bg-white px-3 text-sm font-semibold text-[#514c5b] transition hover:border-[#b9a4ee] hover:bg-[#f7f3ff] hover:text-[#6d28d9] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#7c3aed]" aria-live="polite">
              {copyState === "copied" ? <Check size={15} /> : <Copy size={15} />}
              {copyState === "copied" ? copy.copied : copyState === "failed" ? copy.copyFailed : copy.copy}
            </button>
          </div>
          <div className="model-prototype prompt-detail-input px-6 pt-5 sm:px-8">
            <MediaPromptEditor
              generator={detail.generator}
              modelId={detail.modelId}
              locale={locale}
              prompt={prompt}
              fieldValues={fieldValues}
              referenceImages={referenceImages}
              onPromptChange={setPrompt}
              onFieldChange={(name, value) => setFieldValues((current) => ({ ...current, [name]: value }))}
              onReferenceImagesChange={setReferenceImages}
              t={(key, vars) => modelLandingCopy(locale, key as ModelLandingKey, vars)}
            />
          </div>
          <div className="px-6 pb-7 pt-6 sm:px-8">
            <button id="prompt-detail-generate" type="button" onClick={() => void run()} disabled={!prompt.trim() || checking} className="flex min-h-12 w-full items-center justify-center gap-2 rounded-xl bg-[#171821] px-5 text-sm font-bold text-white transition hover:bg-[#6d28d9] focus-visible:outline-2 focus-visible:outline-offset-3 focus-visible:outline-[#7c3aed] disabled:cursor-not-allowed disabled:opacity-50">
              {copy.create}<ArrowRight size={17} aria-hidden="true" />
            </button>
          </div>
        </section>

        <section className="min-w-0 rounded-2xl border border-[#e7e3eb] bg-white lg:sticky lg:top-24">
          <div className="flex min-h-16 items-center gap-3 px-6 pt-5 sm:px-8">
            <h2 className="text-xs font-bold tracking-[.08em] text-[#24212a] uppercase">{ui.output}</h2>
            <span className="text-xs font-medium text-[#807a88]">{copy.example}</span>
          </div>
          <div className="px-6 pb-5 pt-4 sm:px-8">
            <div onMouseEnter={resumeVideo} className={`relative flex w-full items-center justify-center overflow-hidden rounded-2xl bg-[#10131a] ${previewClassName}`}>
              {detail.video ? (
                <>
                  <CdnFallbackVideo ref={videoRef} src={detail.video} poster={detail.poster || detail.fallbackPoster} fallbackPoster={detail.fallbackPoster} fallbackAlt={detail.label} className={mediaClassName} autoPlay loop muted controls playsInline preload="auto" onPlaying={() => setIsPlaying(true)} aria-label={detail.label} />
                  {!isPlaying ? (
                    <button type="button" onClick={resumeVideo} onMouseEnter={resumeVideo} aria-label={`${copy.video} · ${detail.label}`} className="absolute inset-0 flex items-center justify-center bg-black/10 transition hover:bg-black/20 focus-visible:outline-3 focus-visible:outline-offset-[-3px] focus-visible:outline-[#a78bfa]">
                      <CdnFallbackImage src={detail.poster} fallbackSrc={detail.fallbackPoster} alt="" className={`absolute ${mediaClassName}`} />
                      <span className="relative flex size-16 items-center justify-center rounded-full bg-white/95 text-[#6d28d9] shadow-xl transition hover:scale-105"><Play size={25} fill="currentColor" className="ml-1" /></span>
                    </button>
                  ) : null}
                </>
              ) : (
                <CdnFallbackImage src={detail.poster} fallbackSrc={detail.fallbackPoster} alt={detail.label} className={mediaClassName} />
              )}
            </div>
          </div>
          <div className="flex items-center justify-between gap-4 border-t border-[#eeeaf4] px-6 py-5 text-sm text-[#60596a] sm:px-8">
            <div className="flex items-center gap-2"><Play size={15} className="text-[#8752d1]" aria-hidden="true" /><span>{detail.modelName}</span></div>
            <span className="rounded-full bg-[#f4f1f8] px-3 py-1 text-xs font-semibold">{detail.kind === "video" ? copy.video : copy.image} · {detail.ratio}</span>
          </div>
        </section>
      </div>

      {showLogin ? (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-[#16121f]/60 px-4 py-6 backdrop-blur-[6px]" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setShowLogin(false); }}>
          <div role="dialog" aria-modal="true" aria-labelledby="prompt-login-title" aria-describedby="prompt-login-description" className="relative w-full max-w-[460px] overflow-hidden rounded-3xl border border-[#e7e0f0] bg-white p-6 shadow-[0_32px_100px_-30px_rgba(23,16,45,.55)] sm:p-8">
            <div className="absolute inset-x-0 top-0 h-1.5 bg-gradient-to-r from-[#7c3aed] via-[#a855f7] to-[#d946ef]" aria-hidden="true" />
            <div className="flex items-start justify-between gap-4">
              <span className="flex size-12 items-center justify-center rounded-2xl bg-[#f2ebff] text-[#6d28d9]"><Play size={20} fill="currentColor" /></span>
              <button type="button" onClick={() => setShowLogin(false)} className="flex size-9 items-center justify-center rounded-xl text-[#706a7b] transition hover:bg-[#f5f1fa] hover:text-[#322a40] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#7c3aed]" aria-label={ui.close}><X size={19} /></button>
            </div>
            <h2 id="prompt-login-title" className="mt-6 text-[1.65rem] font-extrabold tracking-tight text-[#20222a]">{ui.loginTitle}</h2>
            <p id="prompt-login-description" className="mt-2 text-sm leading-7 text-[#625d6c]">{ui.loginMessage}</p>
            <div className="mt-6 rounded-2xl border border-[#e9e3f2] bg-[#faf8ff] px-4 py-3">
              <p className="text-xs font-bold text-[#6d28d9]">{detail.modelName}</p>
              <p className="mt-1 line-clamp-2 text-sm leading-6 text-[#5f5a68]">{prompt.trim()}</p>
            </div>
            <div className="mt-6 flex flex-col-reverse gap-3 sm:flex-row">
              <button type="button" onClick={() => setShowLogin(false)} className="flatkey-cta-secondary min-h-12 flex-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#7c3aed]">{ui.keepEditing}</button>
              <a href={signInHref} className="flatkey-cta-primary min-h-12 flex-1 !text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#7c3aed]">{ui.signIn}<ArrowRight size={16} aria-hidden="true" /></a>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}

/** Send the SEO conversion CTA to the console, or to signup with that destination. */
export function PromptDetailOverviewCta({ label, locale }: { label: string; locale: Locale }) {
  const [checking, setChecking] = useState(false);
  const openConsole = async () => {
    if (checking) return;
    setChecking(true);
    let signedIn = false;
    try {
      const response = await fetch("/api/current-user", {
        cache: "no-store",
        credentials: "same-origin",
        headers: { accept: "application/json" },
      });
      if (response.ok) {
        signedIn = isVerifiedConsoleUserPayload(await response.json());
        if (signedIn) rememberConsoleSessionHint();
      } else if (response.status === 401 || response.status === 403) {
        clearConsoleSessionHint();
      }
    } catch {
      // The signup route can still detect an existing console session.
    }
    const target = signedIn
      ? consoleUrl("/dashboard/overview")
      : consoleUrl("/sign-up", new URLSearchParams({ redirect: "/dashboard/overview", lng: locale }).toString());
    window.location.assign(target);
  };
  return (
    <button type="button" onClick={() => void openConsole()} disabled={checking} aria-busy={checking} className="prompt-detail-cta-primary inline-flex min-h-12 items-center justify-center gap-2 rounded-xl px-6 py-3 text-sm font-bold transition-colors focus-visible:outline-2 focus-visible:outline-offset-3 focus-visible:outline-[#7c3aed] disabled:cursor-wait disabled:opacity-70">
      {label}<ArrowRight size={17} aria-hidden="true" />
    </button>
  );
}
