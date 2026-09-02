import { describe, expect, test } from "bun:test";
import { buildFeaturedSlides, FEATURED_SLIDES } from "./model-directory-featured";

describe("model directory featured carousel", () => {
  test("puts Fable 5.1 first with the requested artwork and copy", () => {
    const [first] = FEATURED_SLIDES;

    expect(first).toMatchObject({
      modelName: "claude-fable-5.1",
      displayName: "Claude Fable 5.1",
      image: "/assets/models-featured/claude-fable-5.1.png",
      tags: {
        en: ["Coding", "Agents", "Computer Use"],
      },
    });
    expect(first?.blurb.en).toContain("Anthropic's strongest coding model yet");
  });

  test("keeps the new slide first after filtering to live models", () => {
    const slides = buildFeaturedSlides([
      { model_name: "claude-fable-5.1" },
      { model_name: "deepseek-v4-pro" },
    ]);

    expect(slides.map((slide) => slide.modelName)).toEqual([
      "claude-fable-5.1",
      "deepseek-v4-pro",
    ]);
  });

  test("uses console featured order and model metadata for newly configured models", () => {
    const slides = buildFeaturedSlides([
      {
        model_name: "new-model",
        vendor_name: "Example AI",
        description: "A model configured from the console.",
        tags: "Coding,Long Context",
        icon: "/assets/new-model.png",
        featured_order: 0,
      },
      {
        model_name: "gpt-5.5",
        featured_order: 1,
      },
    ]);

    expect(slides.map((slide) => slide.modelName)).toEqual(["new-model", "gpt-5.5"]);
    expect(slides[0]).toMatchObject({
      displayName: "new-model",
      vendor: "Example AI",
      image: "/assets/new-model.png",
      tags: { en: ["Coding", "Long Context"] },
      blurb: { en: "A model configured from the console." },
    });
  });

  test("applies banner presentation overrides while keeping the model detail CTA", () => {
    const [slide] = buildFeaturedSlides([
      {
        model_name: "new-model",
        featured_order: 0,
        description: "Catalog description",
        featured_config: {
          display_name: "Launch name",
          description: "Banner description",
          tags: "New, Fast",
          background_image_url: "https://cdn.example/banner.png",
          fallback_background_image: "/assets/fallback.png",
        },
      },
    ]);

    expect(slide).toMatchObject({
      displayName: "Launch name",
      image: "https://cdn.example/banner.png",
      fallbackImage: "/assets/fallback.png",
      tags: { en: ["New", "Fast"] },
      blurb: { en: "Banner description" },
    });
  });
});
