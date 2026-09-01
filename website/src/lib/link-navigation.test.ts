import { describe, expect, test, vi, beforeEach, afterEach } from "bun:test";
import { navigateToHref } from "./link-navigation";

describe("navigateToHref", () => {
  const originalWindow = globalThis.window;

  beforeEach(() => {
    globalThis.window = {
      location: {
        assign: vi.fn(),
      },
    } as unknown as Window & typeof globalThis.window;
  });

  afterEach(() => {
    globalThis.window = originalWindow;
    vi.restoreAllMocks();
  });

  test("forces same-tab navigation for an unmodified left click", () => {
    const preventDefault = vi.fn();
    navigateToHref({
      altKey: false,
      button: 0,
      ctrlKey: false,
      currentTarget: { href: "https://console.flatkey.ai/dashboard" },
      defaultPrevented: false,
      metaKey: false,
      preventDefault,
      shiftKey: false,
    } as unknown as Parameters<typeof navigateToHref>[0]);

    expect(preventDefault).toHaveBeenCalledTimes(1);
    expect(window.location.assign).toHaveBeenCalledWith(
      "https://console.flatkey.ai/dashboard",
    );
  });

  test("does not interfere with modified clicks", () => {
    const preventDefault = vi.fn();
    navigateToHref({
      altKey: false,
      button: 0,
      ctrlKey: true,
      currentTarget: { href: "https://console.flatkey.ai/dashboard" },
      defaultPrevented: false,
      metaKey: false,
      preventDefault,
      shiftKey: false,
    } as unknown as Parameters<typeof navigateToHref>[0]);

    expect(preventDefault).not.toHaveBeenCalled();
    expect(window.location.assign).not.toHaveBeenCalled();
  });
});
