import { describe, expect, test } from "bun:test";
import { getTallyEmbedUrl, getTallyFormId, getTallyFormUrl } from "./tally";

describe("Tally form routing", () => {
  test("uses dedicated forms for Chinese and Japanese", () => {
    expect(getTallyFormId("zh")).toBe("BzRylN");
    expect(getTallyFormId("ja")).toBe("vG9Z1Q");
  });

  test("uses the default form for other locales", () => {
    expect(getTallyFormId("en")).toBe("1A6gM4");
    expect(getTallyFormUrl("de")).toBe("https://tally.so/r/1A6gM4");
  });

  test("builds the embed URL from the localized form", () => {
    expect(getTallyEmbedUrl("ja")).toContain("https://tally.so/embed/vG9Z1Q?");
  });
});
