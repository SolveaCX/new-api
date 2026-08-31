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
  en: "DeepSeek V4 Pro is 15% off for a limited time. Join our Discord to get $5 in free credit.",
  zh: "DeepSeek V4 Pro 限时优惠 15% 折扣。加入我们的 Discord，领取 5 美元免费额度。",
  es: "DeepSeek V4 Pro tiene un 15 % de descuento por tiempo limitado. Únete a nuestro Discord y recibe 5 USD de crédito gratis.",
  fr: "DeepSeek V4 Pro est à -15 % pour une durée limitée. Rejoignez notre Discord pour recevoir 5 $ de crédit gratuit.",
  pt: "O DeepSeek V4 Pro está com 15% de desconto por tempo limitado. Entre no nosso Discord e ganhe US$ 5 em crédito grátis.",
  ru: "Скидка 15% на DeepSeek V4 Pro действует ограниченное время. Присоединяйтесь к нашему Discord и получите 5 $ бесплатного кредита.",
  ja: "DeepSeek V4 Pro が期間限定で 15% オフ。Discord に参加して、5 ドル分の無料クレジットを獲得しましょう。",
  vi: "DeepSeek V4 Pro giảm 15% trong thời gian có hạn. Tham gia Discord để nhận 5 USD tín dụng miễn phí.",
  de: "DeepSeek V4 Pro ist für kurze Zeit 15 % günstiger. Tritt unserem Discord bei und erhalte 5 $ Gratisguthaben.",
  id: "DeepSeek V4 Pro diskon 15% untuk waktu terbatas. Bergabunglah dengan Discord kami dan dapatkan kredit gratis senilai US$5.",
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
