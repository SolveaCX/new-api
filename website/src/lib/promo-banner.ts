import { LOCALES, type Locale } from "./locales";

/**
 * Global banner shown at the very top of every public website page.
 *
 * `content` is a locale -> copy map configured in the console under
 * Site & Branding → Official website content. When it is empty the website
 * falls back to the built-in default banner, so an unconfigured deployment
 * renders exactly as it did before the setting existed.
 */
export type PromoBannerSettings = {
  content: Partial<Record<Locale, string>>;
  enabled: boolean;
  href: string;
  icon: string;
};

export const DEFAULT_PROMO_BANNER_CONTENT: Record<Locale, string> = {
  en: "DeepSeek V4 is here. Join our Discord get $5 free credits.",
  zh: "DeepSeek V4 来了。加入我们的 Discord，领取 5 美元免费额度。",
  es: "DeepSeek V4 ya está aquí. Únete a nuestro Discord y recibe 5 USD en créditos gratis.",
  fr: "DeepSeek V4 est arrivé. Rejoignez notre Discord et recevez 5 $ de crédits gratuits.",
  pt: "O DeepSeek V4 chegou. Entre no nosso Discord e ganhe US$ 5 em créditos grátis.",
  ru: "DeepSeek V4 уже здесь. Присоединяйтесь к нашему Discord и получите 5 $ бесплатных кредитов.",
  ja: "DeepSeek V4 が登場。Discord に参加して、5 ドル分の無料クレジットを獲得しましょう。",
  vi: "DeepSeek V4 đã ra mắt. Tham gia Discord của chúng tôi để nhận 5 USD tín dụng miễn phí.",
  de: "DeepSeek V4 ist da. Tritt unserem Discord bei und sichere dir 5 $ Gratisguthaben.",
  id: "DeepSeek V4 telah hadir. Gabung Discord kami dan dapatkan kredit gratis $5.",
};

export const DEFAULT_PROMO_BANNER_HREF = "/blog/deepseek-v4-pro-vs-flash";
export const DEFAULT_PROMO_BANNER_ICON = "/assets/logos/deepseek.svg";

const ICON_DATA_URL_PATTERN =
  /^data:image\/(png|jpeg|webp|svg\+xml|gif);base64,[A-Za-z0-9+/=\s]+$/i;

export function defaultPromoBannerSettings(): PromoBannerSettings {
  return {
    content: { ...DEFAULT_PROMO_BANNER_CONTENT },
    enabled: true,
    href: DEFAULT_PROMO_BANNER_HREF,
    icon: DEFAULT_PROMO_BANNER_ICON,
  };
}

/**
 * Normalize the banner fields carried by the console `/api/status` payload.
 *
 * Falls back to the built-in banner as a whole — never field by field — so an
 * operator never ends up with new copy pointing at the old campaign link.
 */
export function normalizePromoBannerSettings(data: {
  official_website_banner_content?: unknown;
  official_website_banner_enabled?: unknown;
  official_website_banner_href?: unknown;
  official_website_banner_icon?: unknown;
}): PromoBannerSettings {
  const content = normalizeBannerContent(data.official_website_banner_content);
  if (Object.keys(content).length === 0) {
    return {
      ...defaultPromoBannerSettings(),
      enabled: data.official_website_banner_enabled !== false,
    };
  }

  return {
    content,
    enabled: data.official_website_banner_enabled !== false,
    href: normalizeBannerHref(data.official_website_banner_href),
    icon: normalizeBannerIcon(data.official_website_banner_icon),
  };
}

/** Resolve the copy for one locale, falling back to the configured English copy. */
export function promoBannerCopyForLocale(
  content: Partial<Record<Locale, string>>,
  locale: Locale,
): string {
  return content[locale] ?? content.en ?? "";
}

function normalizeBannerContent(
  value: unknown,
): Partial<Record<Locale, string>> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};

  const source = value as Record<string, unknown>;
  const content: Partial<Record<Locale, string>> = {};
  for (const locale of LOCALES) {
    const copy = source[locale];
    if (typeof copy !== "string") continue;
    const trimmed = copy.trim();
    if (trimmed) content[locale] = trimmed;
  }
  return content.en ? content : {};
}

function normalizeBannerHref(value: unknown): string {
  if (typeof value !== "string") return "";
  const trimmed = value.trim();
  if (!trimmed) return "";

  // Protocol-relative URLs would silently leave the site; reject them outright.
  if (trimmed.startsWith("//")) return "";
  if (trimmed.startsWith("/")) return trimmed;

  try {
    const url = new URL(trimmed);
    if (url.protocol !== "http:" && url.protocol !== "https:") return "";
    return url.toString();
  } catch {
    return "";
  }
}

function normalizeBannerIcon(value: unknown): string {
  if (typeof value !== "string") return "";
  const trimmed = value.trim();
  if (!trimmed) return "";
  return ICON_DATA_URL_PATTERN.test(trimmed) ? trimmed : "";
}
