import { describe, expect, test } from "bun:test";
import {
  buildConsoleSessionHintCookieWrites,
  hasConsoleSessionHintFromRequestCookieStore,
  isVerifiedConsoleUserPayload,
  shouldShowPhoneBindingPrompt,
  sharedCookieDomainForHostname,
} from "./console-session-hint";

function withWindow<T>(
  location: { hostname: string; protocol: string } | undefined,
  callback: () => T,
): T {
  const originalWindow = globalThis.window;
  if (location) {
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: {
        location,
      },
    });
  } else {
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: undefined,
    });
  }

  try {
    return callback();
  } finally {
    Object.defineProperty(globalThis, "window", {
      configurable: true,
      value: originalWindow,
    });
  }
}

describe("console session hint", () => {
  test("recognizes only verified current-user payloads", () => {
    expect(
      isVerifiedConsoleUserPayload({
        success: true,
        data: { id: 42 },
      }),
    ).toBe(true);

    expect(
      isVerifiedConsoleUserPayload({
        success: true,
        data: { id: 0 },
      }),
    ).toBe(false);
    expect(
      isVerifiedConsoleUserPayload({
        success: false,
        data: { id: 42 },
      }),
    ).toBe(false);
    expect(isVerifiedConsoleUserPayload(null)).toBe(false);
  });

  test("shows phone prompt for every unbound non-admin user", () => {
    expect(
      shouldShowPhoneBindingPrompt({
        success: true,
        data: { id: 42, role: 1, verification_required: true },
      }),
    ).toBe(true);
    expect(
      shouldShowPhoneBindingPrompt({
        success: true,
        data: {
          id: 42,
          role: 1,
          phone_bound: false,
          verification_required: false,
          sms_verification_enabled: true,
        },
      }),
    ).toBe(true);
    expect(
      shouldShowPhoneBindingPrompt({
        success: true,
        data: {
          id: 42,
          role: 1,
          phone_bound: true,
          verification_required: false,
          sms_verification_enabled: true,
        },
      }),
    ).toBe(false);
    expect(
      shouldShowPhoneBindingPrompt({
        success: true,
        data: { id: 1, role: 10, verification_required: true },
      }),
    ).toBe(false);
    expect(
      shouldShowPhoneBindingPrompt({
        success: true,
        data: { id: 42, verification_required: true },
      }),
    ).toBe(false);
  });

  test("shares hints only across flatkey.ai subdomains", () => {
    expect(sharedCookieDomainForHostname("flatkey.ai")).toBe(".flatkey.ai");
    expect(sharedCookieDomainForHostname("console.flatkey.ai")).toBe(
      ".flatkey.ai",
    );
    expect(sharedCookieDomainForHostname("staging-website.flatkey.ai")).toBe(
      ".flatkey.ai",
    );
    expect(sharedCookieDomainForHostname("localhost")).toBeNull();
    expect(sharedCookieDomainForHostname("preview.example.com")).toBeNull();
  });

  test("writes both host-scoped and shared-domain hint cookies on flatkey.ai", () => {
    withWindow(
      {
        hostname: "console.flatkey.ai",
        protocol: "https:",
      },
      () => {
        expect(buildConsoleSessionHintCookieWrites({ maxAge: 60 })).toEqual([
          "flatkey_console_session_hint=1; path=/; max-age=60; SameSite=Lax; Secure",
          "flatkey_console_session_hint=1; path=/; max-age=60; SameSite=Lax; Secure; Domain=.flatkey.ai",
        ]);
      },
    );
  });

  test("writes only a host-scoped hint cookie outside flatkey.ai", () => {
    withWindow(
      {
        hostname: "preview.example.com",
        protocol: "http:",
      },
      () => {
        expect(buildConsoleSessionHintCookieWrites({ maxAge: 60 })).toEqual([
          "flatkey_console_session_hint=1; path=/; max-age=60; SameSite=Lax",
        ]);
      },
    );
  });

  test("detects hint cookies from request cookie stores", () => {
    expect(
      hasConsoleSessionHintFromRequestCookieStore({
        get(name: string) {
          return name === "flatkey_console_session_hint"
            ? { value: "1" }
            : undefined;
        },
      }),
    ).toBe(true);
    expect(
      hasConsoleSessionHintFromRequestCookieStore({
        get() {
          return undefined;
        },
      }),
    ).toBe(false);
  });
});
