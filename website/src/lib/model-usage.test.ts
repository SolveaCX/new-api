import { afterEach, describe, expect, test } from "bun:test";
import { fetchModelUsage } from "./model-usage";

// The Activity chart on model pages is a public claim about how much traffic a
// model actually takes. Every path that cannot substantiate that claim must
// return null so the section is omitted -- an invented curve on an SEO page
// reads as measured platform volume.

const originalFetch = globalThis.fetch;
const originalKey = process.env.WEBSITE_METRICS_KEY;

function stubFetch(response: { ok: boolean; body?: unknown }) {
  globalThis.fetch = (async () =>
    ({
      ok: response.ok,
      json: async () => response.body,
    }) as unknown as Response) as typeof fetch;
}

afterEach(() => {
  globalThis.fetch = originalFetch;
  if (originalKey === undefined) delete process.env.WEBSITE_METRICS_KEY;
  else process.env.WEBSITE_METRICS_KEY = originalKey;
});

describe("fetchModelUsage", () => {
  test("returns null when no metrics key is configured", async () => {
    delete process.env.WEBSITE_METRICS_KEY;

    expect(await fetchModelUsage("seedance-2.5")).toBeNull();
  });

  test("returns null when the model has no recorded traffic", async () => {
    process.env.WEBSITE_METRICS_KEY = "test-key";
    // Exactly what staging returns for seedance-2.5: a successful, authorised
    // answer that says "this model was not called".
    stubFetch({ ok: true, body: { success: true, model: "seedance-2.5", total: 0, points: [] } });

    expect(await fetchModelUsage("seedance-2.5")).toBeNull();
  });

  test("returns null when the feed request fails", async () => {
    process.env.WEBSITE_METRICS_KEY = "test-key";
    stubFetch({ ok: false });

    expect(await fetchModelUsage("seedance-2.5")).toBeNull();
  });

  test("returns null when the feed reports failure", async () => {
    process.env.WEBSITE_METRICS_KEY = "test-key";
    stubFetch({ ok: true, body: { success: false } });

    expect(await fetchModelUsage("seedance-2.5")).toBeNull();
  });

  test("returns null when the feed throws", async () => {
    process.env.WEBSITE_METRICS_KEY = "test-key";
    globalThis.fetch = (async () => {
      throw new Error("network down");
    }) as typeof fetch;

    expect(await fetchModelUsage("seedance-2.5")).toBeNull();
  });

  test("returns the measured series when the model has real traffic", async () => {
    process.env.WEBSITE_METRICS_KEY = "test-key";
    stubFetch({
      ok: true,
      body: {
        success: true,
        model: "seedance-2.0",
        total: 4,
        points: [{ date: 1787097600, count: 4 }],
      },
    });

    const usage = await fetchModelUsage("seedance-2.0");

    expect(usage).not.toBeNull();
    expect(usage!.model).toBe("seedance-2.0");
    expect(usage!.total).toBe(4);
    expect(usage!.points).toEqual([{ date: 1787097600, count: 4 }]);
  });

  test("never reports a series as a placeholder", async () => {
    process.env.WEBSITE_METRICS_KEY = "test-key";
    stubFetch({
      ok: true,
      body: { success: true, model: "seedance-2.0", total: 4, points: [{ date: 1787097600, count: 4 }] },
    });

    const usage = await fetchModelUsage("seedance-2.0");

    // `placeholder` existed so callers could label invented data. Nothing
    // should be able to set it any more.
    expect(usage).not.toHaveProperty("placeholder", true);
  });
});
