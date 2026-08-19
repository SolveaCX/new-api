import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import {
  PromptLibraryMediaPage,
  PromptLibraryModelPage,
  PromptLibraryPage,
  PromptLibraryPromptPage,
} from "./prompt-library-page";

describe("PromptLibraryPage", () => {
  test("shows only weekly hot, browse by media, and browse by model on the hub", () => {
    const html = renderToStaticMarkup(<PromptLibraryPage locale="en" />);

    expect(html).toContain("Weekly hot");
    expect(html).toContain("Browse by media");
    expect(html).toContain("Browse by model");
    expect(html).not.toContain("Search prompts");
    expect(html).not.toContain("Filter");
  });

  test("renders a dedicated media page with page type and model breakdowns", () => {
    const html = renderToStaticMarkup(
      <PromptLibraryMediaPage locale="en" mediaType="image" />,
    );

    expect(html).toContain("/prompts/image");
    expect(html).toContain("Browse by page type");
    expect(html).toContain("Browse by model");
    expect(html).toContain("Weekly hot");
  });

  test("renders a dedicated prompt detail page with source and copy actions", () => {
    const html = renderToStaticMarkup(
      <PromptLibraryPromptPage
        locale="en"
        slug="convenience-store-night-scene"
      />,
    );

    expect(html).toContain("/prompts/convenience-store-night-scene");
    expect(html).toContain("Copy prompt");
    expect(html).toContain("View source");
    expect(html).toContain("GPT Image 2");
  });

  test("renders a dedicated model page with per-page-type groupings", () => {
    const html = renderToStaticMarkup(
      <PromptLibraryModelPage locale="en" modelSlug="gpt-image-2" />,
    );

    expect(html).toContain("/prompts/models/gpt-image-2");
    expect(html).toContain("Browse by page type");
    expect(html).toContain("Weekly hot");
    expect(html).toContain("GPT Image 2");
  });
});
