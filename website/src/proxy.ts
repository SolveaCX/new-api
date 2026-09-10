import { type NextRequest, NextResponse } from "next/server";
import {
  LANGUAGE_PREFERENCE_COOKIE,
  buildLanguagePreferenceCookieWrites,
  getLanguageRedirectPath,
} from "@/lib/language-routing";
import { isLocale } from "@/lib/locales";
import { APP_CONSOLE_ORIGIN, SITE_ORIGIN } from "@/lib/origins";

const WEBSITE_PUBLIC_PRICING_GROUP = "plg";
const PERMANENT_LEGACY_PATHS = new Map([
  ["privacy-policy", "privacy"],
  ["user-agreement", "terms"],
  ["careers.html", "careers"],
  ["legal-sla", "sla"],
]);
const CANONICAL_MODEL_SLUG_OVERRIDES = new Map([
  ["minimax-h3", "minimax-h3"],
  ["seedance-2-5", "seedance-2.5"],
]);
const LEGACY_MODEL_VENDOR_PREFIXES = new Set([
  "ai",
  "ai21",
  "amazon",
  "anthropic",
  "bytedance",
  "cohere",
  "deepseek",
  "deepseek-ai",
  "google",
  "meta",
  "meta-llama",
  "microsoft",
  "minimax",
  "mistral",
  "moonshot",
  "moonshotai",
  "openai",
  "perplexity",
  "qwen",
  "rekaai",
  "sao10k",
  "~google",
  "~openai",
  "z-ai",
  "zai",
]);

type ModelAliasPath = {
  locale?: string;
  modelSegments: string[];
};

type PublicPricingPayload = {
  success?: boolean;
  data?: Array<{ model_name?: unknown }>;
};

/**
 * Resolve a historical multi-segment model URL against the live public model
 * catalog. Prefer an exact slash-containing model id when one exists; only
 * remove the first segment when the remaining id is itself a live model. This
 * avoids guessing whether a segment is a vendor prefix or part of the model
 * id and leaves retired or unknown URLs as real 404s.
 */
export function resolveModelAliasRedirectPath(pathname: string, modelNames: readonly string[]): string | null {
  const alias = parseModelAliasPath(pathname);
  if (!alias) return null;

  const namesByLowerCase = new Map(modelNames.map((name) => [name.toLowerCase(), name]));
  const fullModelId = alias.modelSegments.join("/");
  const unprefixedModelId = alias.modelSegments.slice(1).join("/");
  const modelName =
    namesByLowerCase.get(fullModelId.toLowerCase()) ?? namesByLowerCase.get(unprefixedModelId.toLowerCase());
  if (!modelName) return null;

  const slug = CANONICAL_MODEL_SLUG_OVERRIDES.get(modelName.toLowerCase()) ?? encodeURIComponent(modelName);
  return `${alias.locale ? `/${alias.locale}` : ""}/models/${slug}`;
}

export async function proxy(request: NextRequest): Promise<NextResponse> {
  // next/image fetches local assets through an internal HTTP request without
  // forwarded headers. Redirecting that request produces text instead of image
  // bytes. Keep asset handling ahead of all page canonicalization and cookies.
  const pathname = request.nextUrl.pathname;
  if (
    pathname.startsWith("/_next/") ||
    /\.(?:avif|gif|ico|jpe?g|png|svg|webp|woff2?|ttf|otf|eot|css|js|mjs|map|mp4|webm|mp3|wav|ogg)$/i.test(pathname)
  ) {
    return NextResponse.next();
  }

  const canonicalOrigin = new URL(SITE_ORIGIN);
  const redirectPath = resolvePermanentSeoRedirectPath(request.nextUrl.pathname);
  // Local development is often reached through a desktop reverse proxy that
  // adds `x-forwarded-proto: https`. Do not turn that into a redirect to the
  // production site; only canonicalize real public hosts (or an explicitly
  // forwarded public host used by the production proxy).
  const isLocalHost = ["localhost", "127.0.0.1", "::1"].includes(request.nextUrl.hostname);
  const forwardedHost = request.headers.get("x-forwarded-host")?.split(",")[0]?.trim();
  const hasPublicForwardedHost = Boolean(forwardedHost && !["localhost", "127.0.0.1", "[::1]"].includes(forwardedHost));
  const isLocalDevelopment = process.env.NODE_ENV === "development";
  const shouldCanonicalizeHost = !isLocalDevelopment && !isLocalHost && isKnownCanonicalHostAlias(requestHostname(request), canonicalOrigin.hostname);
  const shouldCanonicalizeProtocol = !isLocalDevelopment && (!isLocalHost || hasPublicForwardedHost) && requestProtocol(request) !== canonicalOrigin.protocol;
  const canonicalPath = normalizeCanonicalPath(request.nextUrl.pathname);
  const shouldCanonicalizeTrailingSlash = canonicalPath !== request.nextUrl.pathname;
  if (shouldCanonicalizeHost || shouldCanonicalizeProtocol || redirectPath || shouldCanonicalizeTrailingSlash) {
    // Use a standard URL here instead of NextURL.clone(): NextURL can retain
    // the original trailing-slash serialization even after pathname changes.
    const url = new URL(request.nextUrl.toString());
    if (shouldCanonicalizeHost || shouldCanonicalizeProtocol) {
      url.protocol = canonicalOrigin.protocol;
      url.host = canonicalOrigin.host;
      url.port = canonicalOrigin.port;
    }
    if (redirectPath) url.pathname = redirectPath;
    else if (shouldCanonicalizeTrailingSlash) url.pathname = canonicalPath;
    return NextResponse.redirect(url, 301);
  }

  if ((request.method === "GET" || request.method === "HEAD") && parseModelAliasPath(request.nextUrl.pathname)) {
    return redirectModelAliasOrContinue(request);
  }

  return routeByLanguagePreference(request);
}

/**
 * Keep historical public website hostnames out of Google's index. The
 * console/router hosts are intentionally not included: they are separate
 * applications and must never be redirected to the marketing site.
 */
function isKnownCanonicalHostAlias(hostname: string, canonicalHostname: string): boolean {
  return new Set([`www.${canonicalHostname}`, "tokenex.flatkey.ai"]).has(hostname);
}

function requestProtocol(request: NextRequest): string {
  const forwardedProto = request.headers.get("x-forwarded-proto")?.split(",")[0]?.trim();
  if (forwardedProto === "http" || forwardedProto === "https") return `${forwardedProto}:`;
  return request.nextUrl.protocol;
}

function normalizeCanonicalPath(pathname: string): string {
  if (pathname === "/") return "/";
  return pathname.replace(/\/+$/, "") || "/";
}

export function resolvePermanentSeoRedirectPath(pathname: string): string | null {
  const segments = pathname.split("/").filter(Boolean);
  const locale = isLocale(segments[0]) ? segments[0] : undefined;
  const routeSegments = locale ? segments.slice(1) : segments;
  const localePrefix = locale && locale !== "en" ? `/${locale}` : "";

  if (routeSegments.length === 1) {
    const destination = PERMANENT_LEGACY_PATHS.get(routeSegments[0]);
    if (destination) return `${localePrefix}/${destination}`;
  }

  if (
    routeSegments.length === 2 &&
    routeSegments[0] === "models" &&
    routeSegments[1] !== "minimax-h3" &&
    routeSegments[1].toLowerCase() === "minimax-h3"
  ) {
    return `${localePrefix}/models/minimax-h3`;
  }

  return null;
}

function requestHostname(request: NextRequest): string {
  const forwardedHost = request.headers.get("x-forwarded-host")?.split(",")[0]?.trim();
  const host = forwardedHost || request.headers.get("host");
  if (!host) return request.nextUrl.hostname;

  try {
    return new URL(`https://${host}`).hostname;
  } catch {
    return request.nextUrl.hostname;
  }
}

async function redirectModelAliasOrContinue(request: NextRequest): Promise<NextResponse> {
  const modelNames = await fetchPublicModelNames();
  const redirectPath = resolveModelAliasRedirectPath(request.nextUrl.pathname, modelNames);
  if (!redirectPath) return routeByLanguagePreference(request);

  const url = request.nextUrl.clone();
  url.pathname = redirectPath;
  return NextResponse.redirect(url, 301);
}

function routeByLanguagePreference(request: NextRequest): NextResponse {
  const cookieLocale = request.cookies.get(LANGUAGE_PREFERENCE_COOKIE)?.value;
  const redirectPath = getLanguageRedirectPath({
    pathname: request.nextUrl.pathname,
    method: request.method,
    acceptLanguage: request.headers.get("accept-language"),
    cookieLocale,
    refererPathname: sameOriginRefererPathname(request),
    userAgent: request.headers.get("user-agent"),
  });

  const cookieHeader = request.headers.get("cookie");

  if (!redirectPath) return withLanguagePreferenceCookieMigration(NextResponse.next(), cookieLocale, cookieHeader);

  const url = request.nextUrl.clone();
  url.pathname = redirectPath;
  return withLanguagePreferenceCookieMigration(NextResponse.redirect(url, 307), cookieLocale, cookieHeader);
}

function parseModelAliasPath(pathname: string): ModelAliasPath | null {
  const segments = pathname.split("/").filter(Boolean);
  let modelsIndex = 0;
  let locale: string | undefined;

  if (segments[0] !== "models") {
    if (!isLocale(segments[0]) || segments[0] === "en" || segments[1] !== "models") return null;
    locale = segments[0];
    modelsIndex = 1;
  }

  const encodedModelSegments = segments.slice(modelsIndex + 1);
  if (segments[modelsIndex] !== "models" || encodedModelSegments.length < 2) return null;

  try {
    const modelSegments = encodedModelSegments.map((segment) => decodeURIComponent(segment));
    if (!modelSegments.every(Boolean) || !LEGACY_MODEL_VENDOR_PREFIXES.has(modelSegments[0].toLowerCase())) {
      return null;
    }
    return { locale, modelSegments };
  } catch {
    return null;
  }
}

async function fetchPublicModelNames(): Promise<string[]> {
  try {
    const url = new URL("/api/website/pricing", APP_CONSOLE_ORIGIN);
    url.searchParams.set("group", WEBSITE_PUBLIC_PRICING_GROUP);
    const response = await fetch(url, {
      cache: "no-store",
      headers: { accept: "application/json" },
      signal: AbortSignal.timeout(3_000),
    });
    if (!response.ok) return [];

    const payload = (await response.json()) as PublicPricingPayload;
    if (!payload.success || !Array.isArray(payload.data)) return [];
    return payload.data.flatMap((model) => (typeof model.model_name === "string" ? [model.model_name] : []));
  } catch {
    return [];
  }
}

function sameOriginRefererPathname(request: NextRequest): string | null {
  const referer = request.headers.get("referer");
  if (!referer) return null;

  try {
    const url = new URL(referer);
    return url.origin === request.nextUrl.origin ? url.pathname : null;
  } catch {
    return null;
  }
}

function withLanguagePreferenceCookieMigration(
  response: NextResponse,
  cookieLocale: string | undefined,
  cookieHeader?: string | null
): NextResponse {
  const cookieDomain = process.env.COOKIE_SESSION_DOMAIN?.trim();
  if (!cookieDomain) return response;

  const languageCookieCount = (cookieHeader ?? "")
    .split(";")
    .filter((part) => part.trim().startsWith(`${LANGUAGE_PREFERENCE_COOKIE}=`)).length;
  let cookieWrites: string[] = [];
  if (languageCookieCount > 1) {
    cookieWrites = [`${LANGUAGE_PREFERENCE_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`];
  } else if (isLocale(cookieLocale)) {
    cookieWrites = buildLanguagePreferenceCookieWrites(cookieLocale, cookieDomain);
  }

  for (const cookie of cookieWrites) {
    response.headers.append("Set-Cookie", cookie);
  }

  return response;
}
