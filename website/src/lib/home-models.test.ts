import { describe, expect, test } from "bun:test";
import { buildRowsForModels } from "./home-models";
import type { PricingModel } from "./pricing";

describe("models directory display prices", () => {
  test("uses the server-computed PLG price without applying another browser discount", () => {
    const model: PricingModel = {
      model_name: "dynamic-model",
      quota_type: 0,
      model_ratio: 0,
      completion_ratio: 0,
      display_prices: {
        input: { configured: "4", plg: "2" },
        output: { configured: "12", plg: "6" },
        cache: null,
        cache_creation: null,
        image: null,
        audio_input: null,
        audio_output: null,
        request: null,
      },
    };

    const rows = buildRowsForModels([model], [], { plg: 0.1 });

    expect(rows).toHaveLength(1);
    expect(rows[0]?.official).toBe("$4");
    expect(rows[0]?.discounted).toBe("$2");
  });
});
