import { describe, expect, test } from "bun:test";
import {
  isVerifiedConsoleUserPayload,
  sharedCookieDomainForHostname,
} from "./console-session-hint";

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
});
