import { APP_CONSOLE_ORIGIN } from "@/lib/origins";

const REVALIDATE_SECONDS = 60;
const MAX_SUBSCRIPTION_BODY_BYTES = 4 * 1024;
const MAX_IDENTIFIER_LENGTH = 191;
const INVALID_IDENTIFIER_CHARACTER = /[\\/%\u0000-\u001F\u007F]/u;
const FORWARDED_RESPONSE_HEADERS = ["content-type", "cache-control", "etag"] as const;

type RouteContext = { params: Promise<{ path: string[] }> };

class BodyTooLargeError extends Error {}

export async function GET(request: Request, context: RouteContext): Promise<Response> {
  const { path } = await context.params;
  if (!isAllowedGetPath(path)) {
    return isAllowedPostPath(path) ? methodNotAllowed("POST") : notFound();
  }

  const source = new URL(request.url);
  const target = statusTarget(path);
  target.search = source.search;

  try {
    const response = await fetch(target, {
      method: "GET",
      headers: forwardedRequestHeaders(request, false),
      next: { revalidate: REVALIDATE_SECONDS },
    });
    return publicUpstreamResponse(response, true);
  } catch {
    return jsonError(502, "monitoring unavailable");
  }
}

export async function POST(request: Request, context: RouteContext): Promise<Response> {
  const { path } = await context.params;
  if (!isAllowedPostPath(path)) {
    return isAllowedGetPath(path) ? methodNotAllowed("GET") : notFound();
  }

  let body: ArrayBuffer;
  try {
    body = await readBoundedBody(request);
  } catch (error) {
    if (error instanceof BodyTooLargeError) {
      return jsonError(413, "request body too large");
    }
    return jsonError(400, "invalid request body");
  }

  const target = statusTarget(path);
  try {
    const response = await fetch(target, {
      method: "POST",
      headers: forwardedRequestHeaders(request, true),
      body,
      cache: "no-store",
    });
    return publicUpstreamResponse(response, false);
  } catch {
    return jsonError(502, "monitoring unavailable");
  }
}

function isAllowedGetPath(path: readonly string[]): boolean {
  if (path.length === 1) {
    return path[0] === "summary" || path[0] === "components" || path[0] === "incidents" || path[0] === "maintenance";
  }
  if (path.length === 2) {
    if (path[0] === "components" || path[0] === "incidents") return isSafeIdentifier(path[1]);
    return path[0] === "subscriptions" && (path[1] === "verify" || path[1] === "unsubscribe");
  }
  return path.length === 3 && path[0] === "components" && isSafeIdentifier(path[1]) && path[2] === "history";
}

function isAllowedPostPath(path: readonly string[]): boolean {
  return (path.length === 1 && path[0] === "subscriptions") ||
    (path.length === 2 && path[0] === "subscriptions" && path[1] === "unsubscribe");
}

function isSafeIdentifier(value: string | undefined): value is string {
  return typeof value === "string" && value.length > 0 && value.length <= MAX_IDENTIFIER_LENGTH && value !== "." && value !== ".." &&
    !INVALID_IDENTIFIER_CHARACTER.test(value);
}

function statusTarget(path: readonly string[]): URL {
  return new URL(`/api/status/${path.map((segment) => encodeURIComponent(segment)).join("/")}`, APP_CONSOLE_ORIGIN);
}

function forwardedRequestHeaders(request: Request, includeContentType: boolean): Headers {
  const headers = new Headers();
  const accept = request.headers.get("accept");
  const ifNoneMatch = request.headers.get("if-none-match");
  const contentType = request.headers.get("content-type");
  if (accept) headers.set("accept", accept);
  if (ifNoneMatch && !includeContentType) headers.set("if-none-match", ifNoneMatch);
  if (includeContentType && contentType) headers.set("content-type", contentType);
  return headers;
}

function publicUpstreamResponse(response: Response, preserveCacheControl: boolean): Response {
  const headers = new Headers();
  const shouldPreserveCacheControl = preserveCacheControl && response.status < 400;
  for (const name of FORWARDED_RESPONSE_HEADERS) {
    if (name === "cache-control" && !shouldPreserveCacheControl) continue;
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (!shouldPreserveCacheControl) headers.set("cache-control", "no-store");
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}

async function readBoundedBody(request: Request): Promise<ArrayBuffer> {
  const contentLength = request.headers.get("content-length");
  if (contentLength) {
    const declaredLength = Number(contentLength);
    if (Number.isFinite(declaredLength) && declaredLength > MAX_SUBSCRIPTION_BODY_BYTES) {
      throw new BodyTooLargeError();
    }
  }

  if (!request.body) return new ArrayBuffer(0);
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > MAX_SUBSCRIPTION_BODY_BYTES) {
      await reader.cancel();
      throw new BodyTooLargeError();
    }
    chunks.push(value);
  }

  const body = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return body.buffer;
}

function notFound(): Response {
  return jsonError(404, "status endpoint not found");
}

function methodNotAllowed(allowedMethod: "GET" | "POST"): Response {
  const response = jsonError(405, "method not allowed");
  response.headers.set("allow", allowedMethod);
  return response;
}

function jsonError(status: number, message: string): Response {
  return Response.json({ success: false, message }, { status, headers: { "cache-control": "no-store" } });
}
