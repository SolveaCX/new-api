import { describe, expect, test } from "bun:test";
import { getModelPromotions, modelPromotionLabel } from "./model-promotions";

describe("model promotions", () => {
  test("marks the requested models", () => {
    expect(getModelPromotions("glm-5.3")).toEqual([]);
    expect(getModelPromotions("glm-5.3-flash")).toEqual(["limited", "new"]);
    expect(getModelPromotions("deepseek-v4-pro")).toEqual(["limited"]);
    expect(getModelPromotions("deepseek-v4-flash")).toEqual(["free"]);
    expect(getModelPromotions("deepseek-v4-pro-0813")).toEqual([]);
    expect(getModelPromotions("deepseek-v4-flash-0813")).toEqual([]);
    expect(getModelPromotions("glm-5.3-flash-0813")).toEqual([]);
    expect(getModelPromotions("qwen3.8-max")).toEqual([]);
    expect(getModelPromotions("qwen3.8-max-free")).toEqual([]);
    expect(getModelPromotions("kimi-k3")).toEqual([]);
    expect(getModelPromotions("ling-3.0-flash-fin")).toEqual([]);
    expect(getModelPromotions("gpt-5.5")).toEqual([]);
    expect(getModelPromotions("gpt-5.6-sol")).toEqual(["hot"]);
    expect(getModelPromotions("doubao-seedance-2-5-260628")).toEqual(["hot"]);
    expect(getModelPromotions("seedance-2.0")).toEqual([]);
    expect(getModelPromotions("claude-sonnet-4.5")).toEqual([]);
    expect(getModelPromotions("claude-sonnet-4-6")).toEqual(["hot"]);
    expect(getModelPromotions("claude-opus-4-7")).toEqual([]);
    expect(getModelPromotions("claude-opus-4-8")).toEqual(["hot"]);
    expect(getModelPromotions("claude-opus-5")).toEqual(["hot"]);
    expect(getModelPromotions("claude-sonnet-5")).toEqual(["hot"]);
  });

  test("localizes the new-release label", () => {
    expect(modelPromotionLabel("en", "new")).toBe("New release");
    expect(modelPromotionLabel("zh", "new")).toBe("新发布");
  });
});
