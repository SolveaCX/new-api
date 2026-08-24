import { describe, expect, test } from "bun:test";
import { buildFilterGroups } from "./models-directory";

describe("buildFilterGroups", () => {
  test("places output modality and reasoning capability next to input modality", () => {
    const groups = buildFilterGroups("zh", [
      {
        author: "OpenAI",
        providers: ["OpenAI"],
        modalities: ["text", "image"],
        output_modalities: ["text"],
        reasoning: true,
        context_tokens: 128_000,
        series: "GPT",
        categories: ["reasoning"],
        released_at: "2026-08-01",
        distillable: false,
      },
    ]);

    expect(groups.slice(0, 3).map((group) => [group.key, group.label])).toEqual([
      ["modalities", "输入模态"],
      ["outputModalities", "输出模态"],
      ["reasoning", "模型能力"],
    ]);
    expect(groups[2]?.options).toEqual([
      { value: true, label: "推理模型" },
      { value: false, label: "非推理模型" },
    ]);
  });
});
