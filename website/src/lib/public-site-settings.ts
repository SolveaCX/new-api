import { LOCALES, localizePath, stripLocale, type Locale } from "./locales";
import { APP_CONSOLE_ORIGIN } from "./origins";

export const DOCS_LINK_REVALIDATE_SECONDS = 60;
export const DOCS_LINK_TIMEOUT_MS = 3000;
export const ANNOUNCEMENT_LOGO_MAX_LENGTH = 64 * 1024;

type StatusPayload = {
  success?: unknown;
  data?: {
    docs_link?: unknown;
    google_client_id?: unknown;
    google_oauth?: unknown;
    announcements_enabled?: unknown;
    announcements?: unknown;
    welcome_promo_enabled?: unknown;
    welcome_promo?: unknown;
  } | null;
};

export type PublicAnnouncement = {
  id?: string | number;
  content: string;
  content_i18n?: AnnouncementLocaleMap;
  extra?: string;
  intro?: string;
  intro_i18n?: AnnouncementLocaleMap;
  link?: string;
  link_label?: string;
  link_label_i18n?: AnnouncementLocaleMap;
  logo?: string;
  publishDate?: string;
  type?: string;
};

export type WelcomePromoModel = {
  model_name: string;
  description: string;
  offer: string;
};

/** Locale-keyed copy carried by the console announcement configuration. */
export type AnnouncementLocaleMap = Partial<Record<Locale, string>>;

export type PublicSiteSettings = {
  docsUrl: string | null;
  googleOneTap: {
    clientId: string | null;
    enabled: boolean;
  };
  /** Undefined means the status endpoint was unavailable; an empty array means no ads are configured. */
  announcements?: PublicAnnouncement[];
  /** Whether the homepage welcome popup is shown. Defaults to true for backwards compatibility. */
  welcomePromoEnabled: boolean;
  /** Configured cards for the homepage welcome popup. */
  welcomePromo: WelcomePromoModel[];
};

export function normalizeDocsUrl(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  if (!trimmed) return null;

  try {
    const url = new URL(trimmed);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    return url.toString();
  } catch {
    return null;
  }
}

export async function getDocsUrl(): Promise<string | null> {
  return (await getPublicSiteSettings()).docsUrl;
}

export async function getPublicSiteSettings(): Promise<PublicSiteSettings> {
  try {
    const response = await fetch(new URL("/api/status", APP_CONSOLE_ORIGIN), {
      headers: { accept: "application/json" },
      // Console-managed homepage promotions must take effect immediately after
      // an operator toggles them. Do not reuse a stale server-side response.
      cache: "no-store",
      signal: AbortSignal.timeout(DOCS_LINK_TIMEOUT_MS),
    });
    if (!response.ok) return emptyPublicSiteSettings();

    const payload = (await response.json()) as StatusPayload;
    if (
      payload.success !== true ||
      !payload.data ||
      typeof payload.data !== "object"
    ) {
      return emptyPublicSiteSettings();
    }
    const googleClientId = normalizeGoogleClientId(
      payload.data.google_client_id,
    );
    return {
      docsUrl: normalizeDocsUrl(payload.data.docs_link),
      googleOneTap: {
        clientId: googleClientId,
        enabled: payload.data.google_oauth === true && googleClientId !== null,
      },
      welcomePromoEnabled: payload.data.welcome_promo_enabled !== false,
      welcomePromo: normalizeWelcomePromo(payload.data.welcome_promo),
      announcements:
        payload.data.announcements_enabled === true &&
        Array.isArray(payload.data.announcements)
          ? payload.data.announcements
              .map(normalizeAnnouncement)
              .filter((item): item is PublicAnnouncement => item !== null)
          : [],
    };
  } catch {
    return emptyPublicSiteSettings();
  }
}

function normalizeGoogleClientId(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed ? trimmed : null;
}

function emptyPublicSiteSettings(): PublicSiteSettings {
  return {
    docsUrl: null,
    googleOneTap: {
      clientId: null,
      enabled: false,
    },
    welcomePromoEnabled: true,
    welcomePromo: [],
  };
}

export function normalizeWelcomePromo(value: unknown): WelcomePromoModel[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((item) => {
      if (!item || typeof item !== "object" || Array.isArray(item)) return null;
      const raw = item as Record<string, unknown>;
      const modelName = typeof raw.model_name === "string" ? raw.model_name.trim() : "";
      const description = typeof raw.description === "string" ? raw.description.trim() : "";
      const offer = typeof raw.offer === "string" ? raw.offer.trim() : "";
      if (!modelName || !description || !offer) return null;
      return { model_name: modelName, description, offer };
    })
    .filter((item): item is WelcomePromoModel => item !== null)
    .slice(0, 3);
}

/**
 * Resolve a localized announcement field without making the old scalar fields
 * obsolete. The order is deliberately stable: visitor locale, English, then
 * the legacy scalar value.
 */
export function resolveAnnouncementLocaleText(
  map: AnnouncementLocaleMap | undefined,
  locale: Locale,
  legacyValue?: unknown,
): string {
  const localized = normalizeAnnouncementText(map?.[locale]);
  if (localized) return localized;
  const english = normalizeAnnouncementText(map?.en);
  if (english) return english;
  return normalizeAnnouncementText(legacyValue);
}

export function resolveAnnouncementContent(
  announcement: PublicAnnouncement,
  locale: Locale,
): string {
  return resolveAnnouncementLocaleText(
    announcement.content_i18n,
    locale,
    announcement.content,
  );
}

export function resolveAnnouncementIntro(
  announcement: PublicAnnouncement,
  locale: Locale,
): string {
  return resolveAnnouncementLocaleText(
    announcement.intro_i18n,
    locale,
    announcement.intro ?? announcement.extra,
  );
}

export function resolveAnnouncementLinkLabel(
  announcement: PublicAnnouncement,
  locale: Locale,
): string {
  return resolveAnnouncementLocaleText(
    announcement.link_label_i18n,
    locale,
    announcement.link_label,
  );
}

/**
 * Localize a safe, site-relative announcement URL while leaving external URLs
 * untouched. Existing locale prefixes are canonicalized to the visitor's
 * locale so a configured `/zh/foo` does not become `/zh/zh/foo`.
 */
export function localizeAnnouncementLink(
  value: string | undefined,
  locale: Locale,
): string | undefined {
  const normalized = normalizeAnnouncementLink(value);
  if (!normalized) return undefined;
  if (!normalized.startsWith("/")) return normalized;

  const suffixStart = normalized.search(/[?#]/);
  const path =
    suffixStart === -1 ? normalized : normalized.slice(0, suffixStart);
  const suffix = suffixStart === -1 ? "" : normalized.slice(suffixStart);
  return `${localizePath(stripLocale(path), locale)}${suffix}`;
}

export function normalizeAnnouncement(
  value: unknown,
): PublicAnnouncement | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const item = value as Record<string, unknown>;
  const legacyContent = normalizeAnnouncementText(item.content);
  const contentI18n = normalizeAnnouncementLocaleMap(
    item.content_i18n,
    item.contentI18n,
    item.contentByLocale,
    typeof item.content === "object" ? item.content : undefined,
  );
  if (legacyContent && contentI18n && !contentI18n.en) {
    contentI18n.en = legacyContent;
  }
  const content = firstAnnouncementLocaleValue(contentI18n) || legacyContent;
  if (!content) return null;

  const legacyIntro = firstNonEmptyAnnouncementText(
    item.intro,
    item.extra,
    item.summary,
  );
  const introI18n = normalizeAnnouncementLocaleMap(
    item.intro_i18n,
    item.introI18n,
    item.extra_i18n,
    item.extraI18n,
    item.summary_i18n,
    item.summaryI18n,
  );
  if (legacyIntro && introI18n && !introI18n.en) {
    introI18n.en = legacyIntro;
  }
  const intro = firstAnnouncementLocaleValue(introI18n) || legacyIntro;

  const legacyLinkLabel = firstNonEmptyAnnouncementText(
    item.link_label,
    item.linkLabel,
    item.link_text,
    item.linkText,
  );
  const linkLabelI18n = normalizeAnnouncementLocaleMap(
    item.link_label_i18n,
    item.linkLabelI18n,
    item.link_text_i18n,
    item.linkTextI18n,
  );
  if (legacyLinkLabel && linkLabelI18n && !linkLabelI18n.en) {
    linkLabelI18n.en = legacyLinkLabel;
  }
  const linkLabel =
    firstAnnouncementLocaleValue(linkLabelI18n) || legacyLinkLabel;

  const link = normalizeFirstAnnouncementLink(item.link, item.href, item.url);
  const logo = normalizeFirstAnnouncementLogo(
    item.logo,
    item.icon,
    item.logo_url,
    item.icon_url,
  );
  const publishDate = normalizeAnnouncementText(item.publishDate) || undefined;
  const type = normalizeAnnouncementText(item.type) || undefined;
  const id =
    (typeof item.id === "string" && item.id.trim()) ||
    (typeof item.id === "number" && Number.isFinite(item.id)
      ? item.id
      : undefined);

  return {
    content,
    ...(contentI18n ? { content_i18n: contentI18n } : {}),
    ...(intro ? { intro, extra: intro } : {}),
    ...(introI18n ? { intro_i18n: introI18n } : {}),
    ...(link ? { link } : {}),
    ...(linkLabel ? { link_label: linkLabel } : {}),
    ...(linkLabelI18n ? { link_label_i18n: linkLabelI18n } : {}),
    ...(logo ? { logo } : {}),
    ...(publishDate ? { publishDate } : {}),
    ...(type ? { type } : {}),
    ...(id !== undefined ? { id } : {}),
  };
}

function normalizeAnnouncementText(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function firstNonEmptyAnnouncementText(...values: unknown[]): string {
  for (const value of values) {
    const text = normalizeAnnouncementText(value);
    if (text) return text;
  }
  return "";
}

function normalizeAnnouncementLocaleMap(
  ...values: unknown[]
): AnnouncementLocaleMap | undefined {
  const result: AnnouncementLocaleMap = {};
  for (const value of values) {
    const source = parseAnnouncementLocaleMap(value);
    if (!source) continue;
    for (const locale of LOCALES) {
      if (result[locale]) continue;
      const text = normalizeAnnouncementText(source[locale]);
      if (text) result[locale] = text;
    }
  }
  return Object.keys(result).length > 0 ? result : undefined;
}

function parseAnnouncementLocaleMap(
  value: unknown,
): Record<string, unknown> | null {
  if (typeof value === "string") {
    try {
      const parsed: unknown = JSON.parse(value);
      return parseAnnouncementLocaleMap(parsed);
    } catch {
      return null;
    }
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function firstAnnouncementLocaleValue(
  map: AnnouncementLocaleMap | undefined,
): string {
  if (!map) return "";
  for (const locale of LOCALES) {
    const value = normalizeAnnouncementText(map[locale]);
    if (value) return value;
  }
  return "";
}

function normalizeFirstAnnouncementLink(...values: unknown[]): string | null {
  for (const value of values) {
    const normalized = normalizeAnnouncementLink(value);
    if (normalized) return normalized;
  }
  return null;
}

function normalizeFirstAnnouncementLogo(...values: unknown[]): string | null {
  for (const value of values) {
    const normalized = normalizeAnnouncementLogo(value);
    if (normalized) return normalized;
  }
  return null;
}

export function normalizeAnnouncementLink(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  if (!trimmed) return null;
  if (trimmed.startsWith("/") && !trimmed.startsWith("//")) return trimmed;
  try {
    const url = new URL(trimmed);
    return url.protocol === "http:" || url.protocol === "https:"
      ? url.toString()
      : null;
  } catch {
    return null;
  }
}

function normalizeAnnouncementLogo(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  if (!trimmed || trimmed.length > ANNOUNCEMENT_LOGO_MAX_LENGTH) return null;

  if (trimmed.startsWith("/") && !trimmed.startsWith("//")) {
    // Keep built-in logos on the public site's two known asset roots. This
    // mirrors the console validator and prevents an old/malformed setting
    // from making every visitor fetch an arbitrary same-origin path.
    if (
      !/^\/(?:assets\/logos|logos)\/[A-Za-z0-9._/-]+$/.test(trimmed) ||
      trimmed.includes("..") ||
      trimmed.includes("?") ||
      trimmed.includes("#")
    ) {
      return null;
    }
    return trimmed;
  }

  const dataUrlPattern =
    /^data:image\/(png|jpeg|webp|svg\+xml|gif);base64,[A-Za-z0-9+/=\s]+$/i;
  if (dataUrlPattern.test(trimmed)) return trimmed;

  // Keep logo loading on the configured site or inline. The console accepts
  // data URLs for uploads; accepting arbitrary remote images would add an
  // uncontrolled third-party request to every page carrying the banner.
  return null;
}
