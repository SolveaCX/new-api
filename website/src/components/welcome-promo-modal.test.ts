import { describe, expect, test } from "bun:test";
import { LOCALES } from "@/lib/locales";
import { isWelcomePromoHomepage, shouldSuppressWelcomePromo, WELCOME_PROMO_COPY } from "./welcome-promo-modal";

describe("welcome promotion modal", () => {
  test("provides translated copy for every website locale", () => {
    expect(Object.keys(WELCOME_PROMO_COPY).sort()).toEqual([...LOCALES].sort());
    for (const locale of LOCALES) {
      const copy = WELCOME_PROMO_COPY[locale];
      expect(copy.title.length).toBeGreaterThan(1);
      expect(copy.topup.length).toBeGreaterThan(10);
      expect(copy.closeLabel.length).toBeGreaterThan(1);
    }
    for (const locale of LOCALES.filter((item) => item !== "en" && item !== "id")) {
      expect(WELCOME_PROMO_COPY[locale].topup).not.toBe(WELCOME_PROMO_COPY.en.topup);
    }
  });

  test("uses the concise English offer labels", () => {
    expect(WELCOME_PROMO_COPY.en.freeToTry).toBe("Free");
    expect(WELCOME_PROMO_COPY.en.limited).toBe("Limited");
  });

  test("only allows the modal on the homepage and skips browser back navigation", () => {
    expect(isWelcomePromoHomepage("/")).toBe(true);
    expect(isWelcomePromoHomepage("/zh")).toBe(true);
    expect(isWelcomePromoHomepage("/pricing")).toBe(false);
    expect(shouldSuppressWelcomePromo("/pricing", "navigate")).toBe(true);
    expect(shouldSuppressWelcomePromo("/pricing", "back_forward")).toBe(true);
    expect(shouldSuppressWelcomePromo("/", "navigate")).toBe(false);
  });
});
