import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { ModelsFeaturedCarousel } from "./models-featured-carousel";
import { getDirectoryCopy } from "@/lib/model-directory-copy";
import { FEATURED_SLIDES } from "@/lib/model-directory-featured";

describe("ModelsFeaturedCarousel", () => {
  test("keeps Seedance 2.5 and MiniMax H3 at the front of the carousel", () => {
    expect(FEATURED_SLIDES.slice(0, 2).map((slide) => slide.modelName)).toEqual(["seedance-2.5", "MiniMax-H3"]);
  });

  test("renders the complete featured-model description without a line clamp", () => {
    const slide = FEATURED_SLIDES[0];
    const html = renderToStaticMarkup(
      <ModelsFeaturedCarousel
        slides={[slide]}
        copy={getDirectoryCopy("en")}
        locale="en"
      />,
    );

    expect(html).toContain(slide.blurb.en);
    expect(html).toContain("whitespace-normal");
    expect(html).not.toContain("line-clamp-3");
  });
});
