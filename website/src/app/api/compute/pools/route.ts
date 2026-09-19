import { NextResponse } from "next/server";
import { ROUTER_ORIGIN } from "@/lib/origins";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const response = await fetch(new URL("/api/compute/market/public/pools", ROUTER_ORIGIN), {
      cache: "no-store",
      headers: { accept: "application/json" },
    });
    const body = await response.text();
    return new NextResponse(body, {
      status: response.status,
      headers: {
        "content-type": response.headers.get("content-type") ?? "application/json; charset=utf-8",
        "cache-control": "no-store",
      },
    });
  } catch {
    return NextResponse.json(
      { success: false, message: "Compute market is unreachable" },
      { status: 502, headers: { "cache-control": "no-store" } },
    );
  }
}
