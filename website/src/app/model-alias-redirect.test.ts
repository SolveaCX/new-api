import { afterEach, expect, test } from "bun:test";
import EnglishPage from "./(en)/models/[slug]/page";
import LocalizedPage from "./[locale]/models/[slug]/page";

const originalFetch = globalThis.fetch;
afterEach(() => { globalThis.fetch = originalFetch; });
function mockCatalog() {
  globalThis.fetch = (async (input) => {
    const path = new URL(String(input)).pathname;
    if (path === "/api/website/pricing") return Response.json({ success: true, data: [
      { model_name: "gpt-5.4-mini", quota_type: 0, model_ratio: 1, completion_ratio: 1 },
    ] });
    return new Response("unavailable fixture", { status: 503 });
  }) as typeof fetch;
}
async function redirectDigest(page: Promise<unknown>) {
  try { await page; throw new Error("Expected route redirect"); }
  catch (error) { return (error as { digest?: string }).digest; }
}
test("generic model aliases permanently redirect to their canonical page in the same locale", async () => {
  mockCatalog();
  expect(await redirectDigest(EnglishPage({ params: Promise.resolve({ slug: "gpt-5.4-mini-fk" }) })))
    .toBe("NEXT_REDIRECT;replace;/models/gpt-5.4-mini;308;");
  expect(await redirectDigest(LocalizedPage({ params: Promise.resolve({ slug: "gpt-5.4-mini-fk", locale: "fr" }) })))
    .toBe("NEXT_REDIRECT;replace;/fr/models/gpt-5.4-mini;308;");
});
test("canonical generic model URLs still render and unknown models remain not-found", async () => {
  mockCatalog();
  expect(await EnglishPage({ params: Promise.resolve({ slug: "gpt-5.4-mini" }) })).toBeTruthy();
  expect(await LocalizedPage({ params: Promise.resolve({ slug: "gpt-5.4-mini", locale: "fr" }) })).toBeTruthy();
  expect(await redirectDigest(EnglishPage({ params: Promise.resolve({ slug: "not-a-real-model" }) })))
    .toBe("NEXT_HTTP_ERROR_FALLBACK;404");
});
