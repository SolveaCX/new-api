import { NextResponse, type NextRequest } from "next/server";
import { APP_CONSOLE_ORIGIN } from "@/lib/origins";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const identityTarget = new URL(
    "/api/user/analytics-self",
    APP_CONSOLE_ORIGIN,
  );
  const cookie = request.headers.get("cookie") ?? "";
  try {
    const identityResponse = await fetch(identityTarget, {
      cache: "no-store",
      headers: {
        accept: "application/json",
        cookie,
      },
    });
    const identityBody = await identityResponse.text();
    if (!identityResponse.ok) {
      return new NextResponse(identityBody, {
        status: identityResponse.status,
        headers: {
          "content-type":
            identityResponse.headers.get("content-type") ??
            "application/json; charset=utf-8",
          "cache-control": "no-store",
        },
      });
    }

    const identityPayload = JSON.parse(identityBody) as {
      success?: unknown;
      data?: { id?: unknown; role?: unknown } | null;
    };
    const userId = identityPayload.data?.id;
    if (identityPayload.success !== true || typeof userId !== "number") {
      return NextResponse.json(identityPayload, {
        status: identityResponse.status,
        headers: { "cache-control": "no-store" },
      });
    }

    if (
      typeof identityPayload.data?.role === "number" &&
      identityPayload.data.role >= 10
    ) {
      return NextResponse.json(
        {
          ...identityPayload,
          data: { ...identityPayload.data, verification_required: false },
        },
        { headers: { "cache-control": "no-store" } },
      );
    }

    const statusTarget = new URL(
      "/api/user/self/phone-status",
      APP_CONSOLE_ORIGIN,
    );
    const statusResponse = await fetch(statusTarget, {
      cache: "no-store",
      headers: {
        accept: "application/json",
        cookie,
        "New-Api-User": String(userId),
      },
    });
    const statusPayload = (await statusResponse.json()) as {
      success?: unknown;
      data?: Record<string, unknown> | null;
    };
    if (!statusResponse.ok || statusPayload.success !== true) {
      return NextResponse.json(identityPayload, {
        headers: { "cache-control": "no-store" },
      });
    }

    return NextResponse.json(
      {
        ...identityPayload,
        data: { ...identityPayload.data, ...statusPayload.data },
      },
      {
        headers: {
          "cache-control": "no-store",
        },
      },
    );
  } catch {
    return NextResponse.json(
      { success: false, message: "Failed to fetch current user" },
      { status: 502, headers: { "cache-control": "no-store" } },
    );
  }
}
