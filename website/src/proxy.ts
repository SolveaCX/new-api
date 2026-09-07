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
  const canonicalOrigin = new URL(SITE_ORIGIN);
  const redirectPath = resolvePermanentSeoRedirectPath(request.nextUrl.pathname);
  const shouldCanonicalizeHost = requestHostname(request) === `www.${canonicalOrigin.hostname}`;
  if (shouldCanonicalizeHost || redirectPath) {
    const url = request.nextUrl.clone();
    if (shouldCanonicalizeHost) {
      url.protocol = canonicalOrigin.protocol;
      url.host = canonicalOrigin.host;
      url.port = canonicalOrigin.port;
    }
    if (redirectPath) url.pathname = redirectPath;
    return NextResponse.redirect(url, 301);
  }

  if ((request.method === "GET" || request.method === "HEAD") && parseModelAliasPath(request.nextUrl.pathname)) {
    return redirectModelAliasOrContinue(request);
  }

  return routeByLanguagePreference(request);
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
