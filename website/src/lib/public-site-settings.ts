import { APP_CONSOLE_ORIGIN } from "./origins";

export const DOCS_LINK_REVALIDATE_SECONDS = 60;
export const DOCS_LINK_TIMEOUT_MS = 3000;

type StatusPayload = {
  success?: unknown;
  data?: {
    docs_link?: unknown;
    google_client_id?: unknown;
    google_oauth?: unknown;
    announcements_enabled?: unknown;
    announcements?: unknown;
  } | null;
};

export type PublicAnnouncement = {
  id?: string | number;
  content: string;
  extra?: string;
  link?: string;
  publishDate?: string;
  type?: string;
};

export type PublicSiteSettings = {
  docsUrl: string | null;
  googleOneTap: {
    clientId: string | null;
    enabled: boolean;
  };
  /** Undefined means the status endpoint was unavailable; an empty array means no ads are configured. */
  announcements?: PublicAnnouncement[];
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
      next: { revalidate: DOCS_LINK_REVALIDATE_SECONDS },
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
  };
}

function normalizeAnnouncement(value: unknown): PublicAnnouncement | null {
  if (!value || typeof value !== "object") return null;
  const item = value as Record<string, unknown>;
  const content = typeof item.content === "string" ? item.content.trim() : "";
  if (!content) return null;

  const link = normalizeAnnouncementLink(item.link ?? item.url);
  const extra = typeof item.extra === "string" ? item.extra.trim() : "";
  const publishDate =
    typeof item.publishDate === "string" ? item.publishDate : undefined;
  const type = typeof item.type === "string" ? item.type : undefined;
  const id =
    typeof item.id === "string" || typeof item.id === "number"
      ? item.id
      : undefined;

  return {
    content,
    ...(extra ? { extra } : {}),
    ...(link ? { link } : {}),
    ...(publishDate ? { publishDate } : {}),
    ...(type ? { type } : {}),
    ...(id !== undefined ? { id } : {}),
  };
}

function normalizeAnnouncementLink(value: unknown): string | null {
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
