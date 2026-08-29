import { describe, expect, test } from "bun:test";
import { buildFeaturedSlides, FEATURED_SLIDES } from "./model-directory-featured";

describe("model directory featured carousel", () => {
  test("puts GLM-5.3 Flash first with the requested artwork and copy", () => {
    const [first] = FEATURED_SLIDES;

    expect(first).toMatchObject({
      modelName: "glm-5.3-flash",
      displayName: "GLM-5.3 Flash",
      image: "https://cdn.shulex-voc.com/flatkey/models-featured/glm-5.3-flash.png",
      fallbackImage: "/assets/models-featured/glm-5.3-flash.png",
      tags: {
        en: ["Coding", "Multimodal", "Long Context"],
      },
    });
    expect(first?.blurb.en).toContain("Ox Alpha, unmasked.");
  });

  test("keeps the new slide first after filtering to live models", () => {
    const slides = buildFeaturedSlides(["glm-5.3-flash", "deepseek-v4-pro"]);

    expect(slides.map((slide) => slide.modelName)).toEqual([
      "glm-5.3-flash",
      "deepseek-v4-pro",
    ]);
  });
});
