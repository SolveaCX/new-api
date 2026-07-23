import { describe, expect, test } from "bun:test";
import sitemap from "@/app/sitemap";
import { LOCALES } from "@/lib/locales";
import type { PricingModel } from "@/lib/pricing";
import {
  KIMI_FAMILY_MODEL_IDS,
  KIMI_LANDING_PATH,
  KIMI_MODEL_ID,
  getKimiLandingCtaUrl,
  getKimiLandingMetadataInput,
  getKimiLandingPageCopy,
  resolveKimiFamilyModels,
} from "./kimi-landing";

describe("Kimi 3.0 landing page", () => {
  test("uses the approved route and conversion CTA", () => {
    expect(KIMI_LANDING_PATH).toBe("/kimi-3-0");
    expect(KIMI_MODEL_ID).toBe("kimi-k3");
    expect(getKimiLandingCtaUrl("https://console.example.test")).toBe(
      "https://console.example.test/sign-up?redirect=/keys"
    );
  });

  test("H1 echoes the paid-search keyword exactly", () => {
    const english = getKimiLandingPageCopy("en");
    const portuguese = getKimiLandingPageCopy("pt");

    expect(`${english.hero.title} ${english.hero.highlight}`).toBe("Kimi 3.0 API");
    expect(`${portuguese.hero.title} ${portuguese.hero.highlight}`).toBe("API Kimi 3.0");
  });

  test("has complete localized copy for every supported website locale", () => {
    for (const locale of LOCALES) {
      const copy = getKimiLandingPageCopy(locale);

      expect(`${copy.hero.title} ${copy.hero.highlight}`).toContain("Kimi 3.0");
      expect(copy.hero.subtitle.length).toBeGreaterThan(60);
      expect(copy.reasons).toHaveLength(2);
      expect(copy.features).toHaveLength(6);
      expect(copy.faqs).toHaveLength(3);
      expect(copy.code.model).toBe("kimi-k3");
      expect(copy.family.rows.map((row) => row.modelId)).toEqual([...KIMI_FAMILY_MODEL_IDS]);
      expect(copy.family.rows[0]?.name).toContain("Kimi 3.0");
    }
  });

  test("Portuguese copy is really localized and mentions Pix (top paying market)", () => {
    const portuguese = getKimiLandingPageCopy("pt");

    expect(portuguese.hero.subtitle).not.toBe(getKimiLandingPageCopy("en").hero.subtitle);
    expect(JSON.stringify(portuguese)).toContain("Pix");
  });

  test("keeps English and Portuguese ad keywords in metadata", () => {
    const english = getKimiLandingMetadataInput("en");
    const portuguese = getKimiLandingMetadataInput("pt");

    expect(english.pathname).toBe(KIMI_LANDING_PATH);
    expect(english.title.toLowerCase()).toContain("kimi 3.0 api");
    expect(english.description.toLowerCase()).toContain("kimi-k3");
    expect(portuguese.title.toLowerCase()).toContain("kimi 3.0");
    expect(portuguese.description.toLowerCase()).toContain("kimi-k3");
  });

  test("matches the live pricing catalog without inventing prices", () => {
    const models = [
      { model_name: "kimi-k3", quota_type: 0, model_ratio: 1, completion_ratio: 1 },
      { model_name: "kimi-k2.6", quota_type: 0, model_ratio: 1, completion_ratio: 1 },
    ] as PricingModel[];

    const resolved = resolveKimiFamilyModels(models);

    expect(resolved["kimi-k3"]?.model_name).toBe("kimi-k3");
    expect(resolved["kimi-k2.6"]?.model_name).toBe("kimi-k2.6");
    // Missing from the live payload → null → the page renders a pricing link.
    expect(resolved["kimi-k2.5"]).toBeNull();
  });

  test("adds the Kimi page to the sitemap", async () => {
    const entries = await sitemap();

    expect(entries.some((entry) => entry.url === "https://flatkey.ai/kimi-3-0")).toBe(true);
    expect(entries.some((entry) => entry.url === "https://flatkey.ai/pt/kimi-3-0")).toBe(true);
  });
});
