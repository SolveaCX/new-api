import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { PromptFreeCta } from "./prompt-free-cta";

describe("PromptFreeCta", () => {
  test("explains that prompts are free and links to the product playground", () => {
    const html = renderToStaticMarkup(<PromptFreeCta locale="zh" kind="image" />);

    expect(html).toContain("提示词免费，生成从这里开始。");
    expect(html).toContain("进入 Flatkey 开始生成");
    expect(html).toContain("/playground?lng=zh&amp;source=prompt-library&amp;generate=image");
  });
});
