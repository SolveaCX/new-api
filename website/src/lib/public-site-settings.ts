import { APP_CONSOLE_ORIGIN } from "./origins";

export const DOCS_LINK_REVALIDATE_SECONDS = 60;
export const DOCS_LINK_TIMEOUT_MS = 3000;

type StatusPayload = {
  success?: unknown;
  data?: { docs_link?: unknown } | null;
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
  try {
    const response = await fetch(new URL("/api/status", APP_CONSOLE_ORIGIN), {
      headers: { accept: "application/json" },
      next: { revalidate: DOCS_LINK_REVALIDATE_SECONDS },
      signal: AbortSignal.timeout(DOCS_LINK_TIMEOUT_MS),
    });
    if (!response.ok) return null;

    const payload = (await response.json()) as StatusPayload;
    if (payload.success !== true || !payload.data || typeof payload.data !== "object") return null;
    return normalizeDocsUrl(payload.data.docs_link);
  } catch {
    return null;
  }
}
