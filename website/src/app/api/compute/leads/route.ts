import { NextResponse, type NextRequest } from "next/server";
import { ROUTER_ORIGIN } from "@/lib/origins";

export const dynamic = "force-dynamic";

const NO_STORE = { "cache-control": "no-store" };

async function forward(target: URL, init: RequestInit) {
  try {
    const response = await fetch(target, { ...init, cache: "no-store" });
    const body = await response.text();
    return new NextResponse(body, {
      status: response.status,
      headers: {
        "content-type": response.headers.get("content-type") ?? "application/json; charset=utf-8",
        ...NO_STORE,
      },
    });
  } catch {
    return NextResponse.json(
      { success: false, message: "Compute market is unreachable, please retry" },
      { status: 502, headers: NO_STORE },
    );
  }
}

export async function POST(request: NextRequest) {
  const payload = await request.text();
  if (payload.length > 16_000) {
    return NextResponse.json({ success: false, message: "Request too large" }, { status: 413, headers: NO_STORE });
  }
  const forwardedFor =
    request.headers.get("cf-connecting-ip") ?? request.headers.get("x-forwarded-for") ?? "";
  return forward(new URL("/api/compute/market/public/leads", ROUTER_ORIGIN), {
    method: "POST",
    headers: {
      "content-type": "application/json",
      accept: "application/json",
      "x-forwarded-for": forwardedFor,
    },
    body: payload,
  });
}

export async function GET(request: NextRequest) {
  const code = request.nextUrl.searchParams.get("code") ?? "";
  if (!/^[A-Za-z0-9]{4,16}$/.test(code)) {
    return NextResponse.json({ success: false, message: "Invalid code" }, { status: 400, headers: NO_STORE });
  }
  return forward(new URL(`/api/compute/market/public/leads/${code.toUpperCase()}`, ROUTER_ORIGIN), {
    headers: { accept: "application/json" },
  });
}
