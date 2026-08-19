import { describe, expect, test } from "bun:test";
import { getPricingData } from "./pricing";
import { MODEL_DIRECTORY_META } from "./model-directory-meta-data";

// The static metadata table is keyed by exact model name, so it silently loses
// coverage whenever a model is added or renamed upstream. This test fetches the
// live catalogue and reports the drift by name.
//
// Skipped when the pricing API is unreachable, so offline runs and CI without
// network access do not fail on an unrelated dependency.

const MODELS_PAGE_PRICING_GROUP = "plg";

describe("model directory metadata coverage", () => {
  test("covers every model in the live catalogue", async () => {
    const pricing = await getPricingData(MODELS_PAGE_PRICING_GROUP);
    if (pricing.models.length === 0) {
      console.warn("pricing API unreachable — skipping metadata coverage check");
      return;
    }

    const live = new Set(pricing.models.map((model) => model.model_name));
    const covered = new Set(Object.keys(MODEL_DIRECTORY_META));

    const missing = [...live].filter((name) => !covered.has(name)).sort();
    const stale = [...covered].filter((name) => !live.has(name)).sort();

    // A stale entry is inert — it describes a model the catalogue no longer
    // serves, so nothing renders from it. Worth reporting, not worth failing.
    if (stale.length > 0) {
      console.warn(`directory metadata for models no longer live:\n  ${stale.join("\n  ")}`);
    }

    // A missing entry is a real degradation: the model still lists, but it
    // drops out of every metadata-driven filter until a row is added.
    expect(
      missing,
      `models live on /api/website/pricing with no directory metadata:\n  ${missing.join("\n  ")}`
    ).toEqual([]);
  }, 30_000);
});
