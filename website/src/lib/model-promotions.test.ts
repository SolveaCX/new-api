import { describe, expect, test } from "bun:test";
import { getModelPromotions, modelPromotionLabel } from "./model-promotions";

describe("model promotions", () => {
  test("marks the requested models", () => {
    expect(getModelPromotions("glm-5.3")).toEqual([]);
    expect(getModelPromotions("glm-5.3-flash")).toEqual(["new"]);
    expect(getModelPromotions("gpt-5.5")).toEqual([]);
    expect(getModelPromotions("gpt-5.6-sol")).toEqual(["hot"]);
    expect(getModelPromotions("claude-sonnet-4.5")).toEqual([]);
    expect(getModelPromotions("claude-sonnet-4-6")).toEqual(["hot"]);
  });

  test("localizes the new-release label", () => {
    expect(modelPromotionLabel("en", "new")).toBe("New release");
    expect(modelPromotionLabel("zh", "new")).toBe("新发布");
  });
});
