import { describe, expect, test } from "bun:test";
import {
  getModelPromotions,
  modelPromotionLabel,
  sortModelsByPromotion,
} from "./model-promotions";

describe("model promotions", () => {
  test("marks the requested models", () => {
    expect(getModelPromotions("glm-5.3")).toEqual(["new"]);
    expect(getModelPromotions("glm-5.3-flash")).toEqual(["limited", "new"]);
    expect(getModelPromotions("deepseek-v4-pro")).toEqual(["limited"]);
    expect(getModelPromotions("deepseek-v4-flash")).toEqual(["free"]);
    expect(getModelPromotions("deepseek-v4-pro-0813")).toEqual([]);
    expect(getModelPromotions("deepseek-v4-flash-0813")).toEqual([]);
    expect(getModelPromotions("glm-5.3-flash-0813")).toEqual([]);
    expect(getModelPromotions("glm-5.3-0813")).toEqual([]);
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
    expect(getModelPromotions("claude-fable-5.1")).toEqual(["new"]);
    expect(getModelPromotions("anthropic/claude-fable-5.1-latest")).toEqual(["new"]);
  });

  test("sorts free and campaign models ahead of the existing order", () => {
    const models = [
      { model_name: "plain" },
      { model_name: "glm-5.3" },
      { model_name: "deepseek-v4-pro" },
      { model_name: "gpt-5.6-sol" },
      { model_name: "deepseek-v4-flash" },
    ];
    expect(sortModelsByPromotion(models).map((model) => model.model_name)).toEqual([
      "deepseek-v4-flash",
      "deepseek-v4-pro",
      "gpt-5.6-sol",
      "plain",
      "glm-5.3",
    ]);
    expect(sortModelsByPromotion([{ model_name: "plain" }, { model_name: "claude-fable-5.1" }]).map((model) => model.model_name)).toEqual([
      "claude-fable-5.1",
      "plain",
    ]);
  });

  test("localizes the new-release label", () => {
    expect(modelPromotionLabel("en", "new")).toBe("New release");
    expect(modelPromotionLabel("zh", "new")).toBe("新发布");
  });

  test("uses the complete localized limited-discount label", () => {
    expect(modelPromotionLabel("en", "limited")).toBe("Limited discount");
    expect(modelPromotionLabel("fr", "limited")).toBe("Remise à durée limitée");
    expect(modelPromotionLabel("zh", "limited")).toBe("限时折扣");
  });
});
