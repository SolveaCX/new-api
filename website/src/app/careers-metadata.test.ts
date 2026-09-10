import { describe, expect, test } from "bun:test";
import { generateMetadata as listMetadata } from "./[locale]/careers/page";
import { generateMetadata as jobMetadata } from "./[locale]/careers/business-development-representative/page";

describe("careers translation metadata", () => {
  for (const locale of ["es", "fr", "pt", "ru", "ja", "vi", "de", "id"]) {
    test(`${locale} fallback list and detail stay accessible but out of hreflang`, async () => {
      for (const generate of [listMetadata, jobMetadata]) {
        const metadata = await generate({params: Promise.resolve({locale})});
        expect(metadata.robots).toEqual({index: false, follow: false});
        expect(metadata.alternates?.languages).toBeUndefined();
        expect(metadata.alternates?.canonical).toContain(`/${locale}/careers`);
      }
    });
  }
  test("Chinese list and detail remain indexable with English reciprocity", async () => {
    for (const generate of [listMetadata, jobMetadata]) {
      const metadata = await generate({params: Promise.resolve({locale: "zh"})});
      expect(metadata.robots).toEqual({index: true, follow: true});
      expect(Object.keys(metadata.alternates?.languages ?? {})).toEqual(["en-US", "zh-CN", "x-default"]);
    }
  });
});
