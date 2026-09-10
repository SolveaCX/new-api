import { expect, test } from "bun:test";
import { generateMetadata } from "./[locale]/5-credit-promo/page";

test("Portuguese promotion has matching single-language discovery and complete social metadata", async () => {
  const metadata = await generateMetadata({ params: Promise.resolve({ locale: "pt" }) });
  expect(metadata.alternates?.canonical).toBe("https://flatkey.ai/pt/5-credit-promo");
  expect(metadata.alternates?.languages).toEqual({ "pt-BR": "https://flatkey.ai/pt/5-credit-promo", "x-default": "https://flatkey.ai/pt/5-credit-promo" });
  expect(metadata.openGraph?.images).toBeTruthy();
  expect(metadata.twitter).toBeTruthy();
  expect(await generateMetadata({ params: Promise.resolve({ locale: "en" }) })).toEqual({});
});
