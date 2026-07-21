import { describe, expect, test } from "bun:test";
import type { Locale } from "./locales";
import { getStatusCopy } from "./status-copy";

const STATUS_LOCALES = ["en", "zh", "es", "fr", "pt", "ru", "ja", "vi", "de"] as const satisfies readonly Locale[];

describe("status copy", () => {
  test("provides complete status, filter, history, and subscription copy in all nine public locales", () => {
    for (const locale of STATUS_LOCALES) {
      const copy = getStatusCopy(locale);
      const required = [
        copy.title,
        copy.description,
        copy.filters.nameLabel,
        copy.filters.capabilityLabel,
        copy.filters.statusLabel,
        copy.states.operational,
        copy.states.degraded,
        copy.states.outage,
        copy.states.unknown,
        copy.states.maintenance,
        copy.freshness.stale,
        copy.lifecycle.retired,
        copy.history.availabilityLabel,
        copy.history.coverageLabel,
        copy.history.incidentCountLabel,
        copy.incidents.title,
        copy.maintenance.title,
        copy.subscribe.title,
        copy.subscribe.emailLabel,
        copy.subscribe.submitLabel,
      ];

      expect(required.every((value) => value.trim().length > 0)).toBe(true);
      expect(Object.keys(copy.history.ranges).sort()).toEqual(["24h", "30d", "7d", "90d"]);
    }
  });

  test("uses substantive translations instead of English fallbacks", () => {
    const english = getStatusCopy("en");

    for (const locale of STATUS_LOCALES.filter((value) => value !== "en")) {
      const localized = getStatusCopy(locale);
      const localizedFields = [
        localized.title,
        localized.description,
        localized.filters.capabilityLabel,
        localized.states.operational,
        localized.states.unknown,
        localized.history.availabilityLabel,
        localized.subscribe.title,
        localized.subscribe.submitLabel,
      ];
      const englishFields = [
        english.title,
        english.description,
        english.filters.capabilityLabel,
        english.states.operational,
        english.states.unknown,
        english.history.availabilityLabel,
        english.subscribe.title,
        english.subscribe.submitLabel,
      ];

      expect(localizedFields.every((value, index) => value !== englishFields[index])).toBe(true);
    }
  });
});
