"use client";

import Image from "next/image";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { LOCALES, type Locale, localizePath, withIdFallback } from "@/lib/locales";
import { consoleUrl } from "@/lib/origins";

type WelcomePromoCopy = {
  title: string;
  accent: string;
  topup: string;
  getApiKey: string;
  exploreModels: string;
  freeToTry: string;
  off: string;
  closeLabel: string;
  dialogLabel: string;
  reasoningModel: string;
  multimodalModel: string;
};

export const WELCOME_PROMO_COPY: Record<Locale, WelcomePromoCopy> = withIdFallback({
  en: {
    title: "FREE TRIAL",
    accent: "EXTRA 80% OFF",
    topup: "Top up your account – additional 80% off",
    getApiKey: "Get API Key",
    exploreModels: "Explore Models",
    freeToTry: "Free to try",
    off: "55% off",
    closeLabel: "Close promotion",
    dialogLabel: "Free trial promotion",
    reasoningModel: "DeepSeek reasoning model",
    multimodalModel: "GLM multimodal model",
  },
  zh: {
    title: "免费试用",
    accent: "额外 80% 折扣",
    topup: "充值到账户，额外享受 80% 折扣",
    getApiKey: "获取 API 密钥",
    exploreModels: "探索模型",
    freeToTry: "免费试用",
    off: "55% 折扣",
    closeLabel: "关闭优惠弹窗",
    dialogLabel: "免费试用优惠",
    reasoningModel: "DeepSeek 推理模型",
    multimodalModel: "GLM 多模态模型",
  },
  es: {
    title: "PRUEBA GRATIS",
    accent: "80 % EXTRA DE DESCUENTO",
    topup: "Recarga tu cuenta y obtén un 80 % de descuento adicional",
    getApiKey: "Obtener clave API",
    exploreModels: "Explorar modelos",
    freeToTry: "Prueba gratis",
    off: "55 % de descuento",
    closeLabel: "Cerrar promoción",
    dialogLabel: "Promoción de prueba gratis",
    reasoningModel: "Modelo de razonamiento DeepSeek",
    multimodalModel: "Modelo multimodal GLM",
  },
  fr: {
    title: "ESSAI GRATUIT",
    accent: "80 % DE REMISE EN PLUS",
    topup: "Rechargez votre compte : 80 % de remise supplémentaire",
    getApiKey: "Obtenir une clé API",
    exploreModels: "Explorer les modèles",
    freeToTry: "Essai gratuit",
    off: "55 % de remise",
    closeLabel: "Fermer la promotion",
    dialogLabel: "Promotion d'essai gratuit",
    reasoningModel: "Modèle de raisonnement DeepSeek",
    multimodalModel: "Modèle multimodal GLM",
  },
  pt: {
    title: "TESTE GRÁTIS",
    accent: "80% DE DESCONTO EXTRA",
    topup: "Adicione saldo à conta e ganhe 80% de desconto adicional",
    getApiKey: "Obter chave de API",
    exploreModels: "Explorar modelos",
    freeToTry: "Teste grátis",
    off: "55% de desconto",
    closeLabel: "Fechar promoção",
    dialogLabel: "Promoção de teste grátis",
    reasoningModel: "Modelo de raciocínio DeepSeek",
    multimodalModel: "Modelo multimodal GLM",
  },
  ru: {
    title: "БЕСПЛАТНЫЙ ПРОБНЫЙ ПЕРИОД",
    accent: "ЕЩЁ 80% СКИДКИ",
    topup: "Пополните счёт и получите дополнительные 80% скидки",
    getApiKey: "Получить API-ключ",
    exploreModels: "Изучить модели",
    freeToTry: "Бесплатно",
    off: "Скидка 55%",
    closeLabel: "Закрыть акцию",
    dialogLabel: "Акция бесплатного пробного периода",
    reasoningModel: "Модель рассуждений DeepSeek",
    multimodalModel: "Мультимодальная модель GLM",
  },
  ja: {
    title: "無料トライアル",
    accent: "さらに 80% OFF",
    topup: "アカウントにチャージすると、さらに 80% OFF",
    getApiKey: "API キーを取得",
    exploreModels: "モデルを見る",
    freeToTry: "無料で試す",
    off: "55% OFF",
    closeLabel: "キャンペーンを閉じる",
    dialogLabel: "無料トライアルキャンペーン",
    reasoningModel: "DeepSeek 推論モデル",
    multimodalModel: "GLM マルチモーダルモデル",
  },
  vi: {
    title: "DÙNG THỬ MIỄN PHÍ",
    accent: "GIẢM THÊM 80%",
    topup: "Nạp tiền vào tài khoản để được giảm thêm 80%",
    getApiKey: "Lấy API key",
    exploreModels: "Khám phá model",
    freeToTry: "Dùng thử miễn phí",
    off: "Giảm 55%",
    closeLabel: "Đóng khuyến mãi",
    dialogLabel: "Khuyến mãi dùng thử miễn phí",
    reasoningModel: "Model suy luận DeepSeek",
    multimodalModel: "Model đa phương thức GLM",
  },
  de: {
    title: "KOSTENLOS TESTEN",
    accent: "ZUSÄTZLICH 80 % RABATT",
    topup: "Laden Sie Ihr Konto auf und erhalten Sie zusätzlich 80 % Rabatt",
    getApiKey: "API-Key abrufen",
    exploreModels: "Modelle entdecken",
    freeToTry: "Kostenlos testen",
    off: "55 % Rabatt",
    closeLabel: "Aktion schließen",
    dialogLabel: "Aktion zum kostenlosen Testen",
    reasoningModel: "DeepSeek-Reasoning-Modell",
    multimodalModel: "Multimodales GLM-Modell",
  },
});

type ModelCardProps = {
  logo: string;
  logoAlt: string;
  name: string;
  description: string;
  offer: string;
  offerClassName?: string;
  highlighted?: boolean;
};

const desktopTitleClassByLocale: Record<Locale, string> = {
  en: "lg:text-[clamp(2.25rem,5.6vw,56px)]",
  zh: "lg:text-[clamp(2.25rem,5.6vw,56px)]",
  es: "lg:text-[42px]",
  fr: "lg:text-[42px]",
  pt: "lg:text-[36px]",
  ru: "lg:text-[32px]",
  ja: "lg:text-[clamp(2.25rem,5.6vw,56px)]",
  vi: "lg:text-[42px]",
  de: "lg:text-[36px]",
  id: "lg:text-[clamp(2.25rem,5.6vw,56px)]",
};

const desktopActionMarginClassByLocale: Record<Locale, string> = {
  en: "lg:mt-14",
  zh: "lg:mt-14",
  es: "lg:mt-6",
  fr: "lg:mt-6",
  pt: "lg:mt-4",
  ru: "lg:mt-4",
  ja: "lg:mt-14",
  vi: "lg:mt-6",
  de: "lg:mt-4",
  id: "lg:mt-14",
};

const offerTextClassByLocale: Record<Locale, string> = {
  en: "text-xs max-[479px]:text-[11px] lg:text-base",
  zh: "text-xs max-[479px]:text-[11px] lg:text-base",
  es: "text-xs max-[479px]:text-[10px] lg:text-[13px]",
  fr: "text-xs max-[479px]:text-[10px] lg:text-sm",
  pt: "text-xs max-[479px]:text-[10px] lg:text-[13px]",
  ru: "text-xs max-[479px]:text-[10px] lg:text-sm",
  ja: "text-xs max-[479px]:text-[11px] lg:text-base",
  vi: "text-xs max-[479px]:text-[10px] lg:text-[12px]",
  de: "text-xs max-[479px]:text-[10px] lg:text-[12px]",
  id: "text-xs max-[479px]:text-[11px] lg:text-base",
};

function ModelCard(props: ModelCardProps) {
  return (
    <div className="relative flex h-[clamp(56px,8vh,64px)] w-full items-center gap-2 overflow-hidden rounded-xl border border-white/65 bg-white/35 p-2 shadow-[0_3px_8px_rgba(0,0,0,0.02)] backdrop-blur-sm max-[479px]:h-[clamp(46px,8vh,52px)] lg:h-[86px] lg:gap-4 lg:rounded-2xl lg:p-5">
      <Image src={props.logo} alt={props.logoAlt} width={24} height={24} className="size-5 shrink-0 object-contain max-[479px]:size-[18px] lg:size-6" />
      <div className="box-border h-auto min-w-0 flex-[1_1_180px] shrink border-r border-black/[0.08] pr-2 lg:h-[46px] lg:w-auto lg:min-w-0 lg:pr-4">
        <p className="truncate text-sm leading-5 font-semibold tracking-[-0.02em] text-black max-[479px]:text-xs max-[479px]:leading-4 lg:text-base lg:leading-6">{props.name}</p>
        <p className="truncate text-[11px] leading-4 tracking-[-0.01em] text-black/60 max-[479px]:text-[10px] max-[479px]:leading-3 lg:text-[13px] lg:leading-5">{props.description}</p>
      </div>
      <span className={["flex h-7 min-w-0 max-w-[140px] shrink-0 items-center justify-center break-words whitespace-normal px-2 text-center font-medium tracking-[-0.02em] max-[479px]:h-6 max-[479px]:max-w-[120px] max-[479px]:px-1 lg:h-auto lg:min-h-[29px] lg:min-w-[101px] lg:max-w-[150px] lg:px-1 lg:leading-5 lg:whitespace-normal", props.offerClassName, props.highlighted ? "rounded-lg bg-[#fab8e5] text-black" : "text-[#5b20d1]"].filter(Boolean).join(" ")}>{props.offer}</span>
    </div>
  );
}

export function isWelcomePromoHomepage(pathname: string | null | undefined) {
  if (!pathname || pathname === "/") return pathname === "/";
  return LOCALES.some((locale) => locale !== "en" && pathname === `/${locale}`);
}

export function shouldSuppressWelcomePromo(pathname: string | null | undefined, navigationType: string | undefined) {
  return !isWelcomePromoHomepage(pathname) || navigationType === "back_forward";
}

export function WelcomePromoModal({ locale }: { locale: Locale }) {
  const pathname = usePathname();
  const [isOpen, setIsOpen] = useState(false);
  const [isEntered, setIsEntered] = useState(false);
  const [isBackgroundReady, setIsBackgroundReady] = useState(false);
  const popStateRef = useRef(false);
  const copy = useMemo(() => WELCOME_PROMO_COPY[locale] ?? WELCOME_PROMO_COPY.en, [locale]);
  const desktopTitleClass = desktopTitleClassByLocale[locale] ?? desktopTitleClassByLocale.en;
  const desktopActionMarginClass = desktopActionMarginClassByLocale[locale] ?? desktopActionMarginClassByLocale.en;

  useEffect(() => {
    if (!isWelcomePromoHomepage(pathname)) return;
    const image = new window.Image();
    image.onload = () => setIsBackgroundReady(true);
    image.src = "/assets/welcome-promo-bg.png";
    return () => {
      image.onload = null;
    };
  }, [pathname]);

  useEffect(() => {
    const onPopState = () => {
      popStateRef.current = true;
    };
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, [popStateRef]);

  useEffect(() => {
    const navigation = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming | undefined;
    const navigationType = navigation?.type;
    const wasPopState = popStateRef.current;
    popStateRef.current = false;
    const timer = window.setTimeout(() => {
      setIsEntered(false);
      setIsOpen(!shouldSuppressWelcomePromo(pathname, wasPopState ? "back_forward" : navigationType));
    }, 0);
    return () => window.clearTimeout(timer);
  }, [pathname]);

  useEffect(() => {
    if (!isOpen) return;
    const frame = window.requestAnimationFrame(() => setIsEntered(true));
    return () => window.cancelAnimationFrame(frame);
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setIsOpen(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div className={["fixed inset-0 z-[100] flex items-center justify-center bg-black/45 p-3 backdrop-blur-[2px] transition-opacity duration-300 ease-out motion-reduce:transition-none sm:p-5", isEntered ? "opacity-100" : "opacity-0"].join(" ")} onMouseDown={(event) => { if (event.target === event.currentTarget) setIsOpen(false); }}>
      <section role="dialog" aria-modal="true" aria-label={copy.dialogLabel} aria-labelledby="welcome-promo-title" aria-describedby="welcome-promo-description" className={["relative h-auto w-full max-w-[1000px] max-h-[calc(100svh-24px)] overflow-hidden rounded-3xl bg-white p-1 shadow-[0_30px_90px_-30px_rgba(15,23,42,0.55)] transition-[opacity,transform] duration-300 ease-out motion-reduce:transition-none sm:max-h-[calc(100svh-40px)] lg:aspect-[1000/498] lg:h-auto lg:w-[min(1000px,calc(100vw-32px))] lg:max-h-[calc(100dvh-32px)] lg:rounded-[32px] lg:p-2", isEntered ? "translate-y-0 scale-100 opacity-100" : "translate-y-2 scale-[0.98] opacity-0"].join(" ")}>
        <div className="relative flex h-auto flex-col overflow-hidden rounded-[20px] bg-[linear-gradient(180deg,#cab3fd_0%,#fbe0f0_50%,#faf8f9_100%)] px-4 pt-[clamp(40px,10vh,56px)] pb-[clamp(12px,4vh,24px)] text-black max-[479px]:gap-0 max-[479px]:px-3 max-[479px]:pt-[clamp(44px,10vh,56px)] max-[479px]:pb-2 sm:gap-0 sm:px-8 sm:pt-[clamp(60px,12vh,64px)] sm:pb-[clamp(16px,4vh,24px)] sm:max-lg:pt-[clamp(68px,12vh,72px)] lg:block lg:h-full lg:overflow-hidden lg:rounded-[26px] lg:px-12 lg:py-12">
          <button type="button" aria-label={copy.closeLabel} onClick={() => setIsOpen(false)} className="absolute top-3 right-3 z-10 inline-flex size-9 items-center justify-center rounded-xl bg-white/35 text-2xl leading-none transition hover:bg-white/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-black max-[479px]:top-1.5 max-[479px]:right-1.5 max-[479px]:size-8 max-[479px]:text-xl sm:top-5 sm:right-5 sm:size-10 lg:top-6 lg:right-6 lg:rounded-2xl lg:text-3xl">×</button>
          <Image
            src="/assets/welcome-promo-bg.png"
            alt=""
            fill
            sizes="(min-width: 1024px) 984px, calc(100vw - 24px)"
            priority
            onLoad={() => setIsBackgroundReady(true)}
            className={["pointer-events-none z-0 object-fill object-center transition-opacity duration-500 ease-out motion-reduce:transition-none lg:object-cover", isBackgroundReady ? "opacity-100" : "opacity-0"].join(" ")}
          />
          <div className="relative z-10 grid min-h-0 flex-none grid-cols-1 grid-rows-none gap-4 sm:max-lg:grid-cols-2 [@media(max-width:1023px)_and_(orientation:landscape)_and_(min-width:360px)]:grid-cols-2 max-[479px]:gap-0 sm:gap-4 lg:grid lg:h-full lg:flex-none lg:grid-rows-none lg:grid-cols-[minmax(0,1fr)_377px] lg:items-center lg:gap-8">
            <div>
              <p id="welcome-promo-title" className={["flex w-full flex-col gap-1 text-[clamp(1.5rem,min(9vw,7vh),3.5rem)] leading-[1.05] font-semibold tracking-[-0.04em] max-[479px]:text-[clamp(1.5rem,min(9vw,6vh),3.5rem)] sm:max-lg:text-[clamp(2rem,min(6vw,8vh),3rem)] sm:max-lg:leading-[1.05] lg:w-[441px] lg:max-w-full lg:leading-[1.2] lg:tracking-[-0.01em]", desktopTitleClass].join(" ")}><span>{copy.title}</span><span>{copy.accent}</span></p>
              <p id="welcome-promo-description" className="mt-3 max-w-xl text-sm leading-5 tracking-[-0.02em] max-[479px]:mt-2 max-[479px]:text-xs max-[479px]:leading-4 sm:text-base sm:leading-6 lg:mt-7 lg:text-xl lg:leading-7 lg:tracking-[-0.03em]">{copy.topup}</p>
              <div className={["mt-4 flex w-full gap-2 sm:mt-5 sm:gap-3 sm:max-lg:mt-4 sm:max-lg:gap-2 lg:w-auto lg:flex-wrap max-[479px]:mt-3 max-[479px]:flex-col", desktopActionMarginClass].join(" ")}>
                <a href={consoleUrl("/dashboard")} className="inline-flex h-[clamp(32px,6vh,44px)] min-w-0 flex-1 items-center justify-center rounded-full bg-black px-3 text-xs font-medium !text-white transition hover:bg-black/80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-black max-[479px]:h-[clamp(30px,6vh,44px)] sm:h-[clamp(32px,6vh,44px)] sm:px-5 sm:text-sm sm:max-lg:h-[clamp(32px,6vh,44px)] sm:max-lg:px-3 sm:max-lg:text-xs lg:h-12 lg:flex-none lg:px-7 lg:text-base max-[479px]:w-full max-[479px]:flex-none">{copy.getApiKey}<span aria-hidden="true" className="ml-2 text-lg leading-none !text-white lg:ml-3 lg:text-xl">↗</span></a>
                <a href={localizePath("/models", locale)} className="inline-flex h-[clamp(32px,6vh,44px)] min-w-0 flex-1 items-center justify-center rounded-full border !border-black px-3 text-xs font-medium text-black transition hover:bg-white/35 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-black max-[479px]:h-[clamp(30px,6vh,44px)] sm:h-[clamp(32px,6vh,44px)] sm:px-5 sm:text-sm sm:max-lg:h-[clamp(32px,6vh,44px)] sm:max-lg:px-3 sm:max-lg:text-xs lg:h-12 lg:min-w-[181px] lg:w-auto lg:flex-none lg:gap-2 lg:px-6 lg:text-lg lg:whitespace-nowrap max-[479px]:w-full max-[479px]:flex-none">{copy.exploreModels}</a>
              </div>
            </div>
            <div className="grid content-start gap-2 lg:content-normal lg:gap-3">
              <ModelCard logo="/assets/logos/deepseek.svg" logoAlt="DeepSeek" name="DeepSeek V4 Flash" description={copy.reasoningModel} offer={copy.freeToTry} offerClassName={offerTextClassByLocale[locale]} highlighted />
              <ModelCard logo="/assets/logos/deepseek.svg" logoAlt="DeepSeek" name="DeepSeek V4 Pro" description={copy.reasoningModel} offer={copy.off} offerClassName={offerTextClassByLocale[locale]} />
              <ModelCard logo="/assets/logos/zai.svg" logoAlt="GLM" name="GLM 5.3flash" description={copy.multimodalModel} offer={copy.off} offerClassName={offerTextClassByLocale[locale]} />
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
