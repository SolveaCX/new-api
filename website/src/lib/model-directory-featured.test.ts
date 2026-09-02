import { describe, expect, test } from "bun:test";
import { buildFeaturedSlides, FEATURED_SLIDES } from "./model-directory-featured";

describe("model directory featured carousel", () => {
  test("puts Fable 5.1 first with the requested artwork and copy", () => {
    const [first] = FEATURED_SLIDES;

    expect(first).toMatchObject({
      modelName: "claude-fable-5.1",
      displayName: "Fable 5.1",
      image: "/assets/models-featured/claude-fable-5.1.png",
      tags: {
        en: ["Coding", "Agents", "Computer Use"],
      },
    });
    expect(first?.blurb.en).toContain("Anthropic's strongest coding model yet");
  });

  test("keeps the new slide first after filtering to live models", () => {
    const slides = buildFeaturedSlides(["claude-fable-5.1", "deepseek-v4-pro"]);

    expect(slides.map((slide) => slide.modelName)).toEqual([
      "claude-fable-5.1",
      "deepseek-v4-pro",
    ]);
  });
});
