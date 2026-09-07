import { APP_CONSOLE_ORIGIN } from "@/lib/origins";

const MEDIA_ID_PATTERN = /^[a-f0-9]{64}\.(?:gif|jpg|png|webp)$/;
const CACHE_CONTROL = "public, max-age=31536000, immutable";

type RouteContext = { params: Promise<{ id: string }> };

export async function GET(request: Request, context: RouteContext) {
  const { id } = await context.params;
  if (!MEDIA_ID_PATTERN.test(id)) {
    return new Response(null, { status: 404 });
  }

  const target = new URL(`/media/website-featured/${id}`, APP_CONSOLE_ORIGIN);
  const ifNoneMatch = request.headers.get("if-none-match");

  try {
    const response = await fetch(target, {
      cache: "no-store",
      headers: ifNoneMatch ? { "if-none-match": ifNoneMatch } : undefined,
    });
    if (response.status === 304) {
      return new Response(null, {
        status: 304,
        headers: immutableMediaHeaders(response),
      });
    }
    if (!response.ok || !response.body) {
      return new Response(null, {
        status: response.status === 404 ? 404 : 502,
        headers: { "cache-control": "no-store" },
      });
    }

    return new Response(response.body, {
      status: 200,
      headers: immutableMediaHeaders(response),
    });
  } catch {
    return new Response(null, {
      status: 502,
      headers: { "cache-control": "no-store" },
    });
  }
}

function immutableMediaHeaders(response: Response): Headers {
  const headers = new Headers({
    "cache-control": CACHE_CONTROL,
    "content-type":
      response.headers.get("content-type") ?? "application/octet-stream",
    "x-content-type-options": "nosniff",
  });
  for (const name of ["content-length", "etag"]) {
    const value = response.headers.get(name);
    if (value) headers.set(name, value);
  }
  return headers;
}
