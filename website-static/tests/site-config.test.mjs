import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";

const scriptPath = new URL("../html/assets/site-config.js", import.meta.url);

function loadApi({ withAbortController = true } = {}) {
  const context = {
    URL,
    clearTimeout,
    setTimeout,
  };
  if (withAbortController) context.AbortController = AbortController;
  context.globalThis = context;
  vm.createContext(context);
  vm.runInContext(readFileSync(scriptPath, "utf8"), context, {
    filename: "site-config.js",
  });
  return context.FlatkeySiteConfig;
}

test("normalizes only absolute HTTP and HTTPS documentation URLs", () => {
  const api = loadApi();

  assert.equal(
    api.normalizeDocsUrl(" https://docs.example.com/start "),
    "https://docs.example.com/start",
  );
  assert.equal(api.normalizeDocsUrl("http://docs.example.com"), "http://docs.example.com/");

  for (const value of [
    "",
    "   ",
    "/docs",
    "javascript:alert(1)",
    "data:text/plain,x",
    "not a url",
    null,
    undefined,
  ]) {
    assert.equal(api.normalizeDocsUrl(value), null);
  }
});

test("reads docs_link from the bounded same-origin status request", async () => {
  const api = loadApi();
  let requestedUrl;
  let requestedOptions;

  const url = await api.getDocsUrl(async (input, options) => {
    requestedUrl = input;
    requestedOptions = options;
    return {
      ok: true,
      json: async () => ({
        success: true,
        data: { docs_link: "https://docs.example.com" },
      }),
    };
  });

  assert.equal(url, "https://docs.example.com/");
  assert.equal(requestedUrl, "/api/status");
  assert.equal(requestedOptions.headers.accept, "application/json");
  assert.ok(requestedOptions.signal instanceof AbortSignal);
});

test("fetches the status without a signal when AbortController is unavailable", async () => {
  const api = loadApi({ withAbortController: false });
  let requestedOptions;

  const url = await api.getDocsUrl(async (_input, options) => {
    requestedOptions = options;
    return {
      ok: true,
      json: async () => ({
        success: true,
        data: { docs_link: "https://docs.example.com" },
      }),
    };
  });

  assert.equal(url, "https://docs.example.com/");
  assert.equal("signal" in requestedOptions, false);
});

test("bounds the status request when AbortController is unavailable", async () => {
  const api = loadApi({ withAbortController: false });

  const url = await api.getDocsUrl(
    () => new Promise((resolve) => {
      setTimeout(() => {
        resolve({
          ok: true,
          json: async () => ({
            success: true,
            data: { docs_link: "https://docs.example.com" },
          }),
        });
      }, 25);
    }),
    1,
  );

  assert.equal(url, null);
});

test("returns null for failed responses, envelopes, payloads, and requests", async () => {
  const api = loadApi();

  assert.equal(await api.getDocsUrl(async () => ({ ok: false })), null);
  assert.equal(
    await api.getDocsUrl(async () => ({
      ok: true,
      json: async () => ({
        success: false,
        data: { docs_link: "https://docs.example.com" },
      }),
    })),
    null,
  );
  assert.equal(
    await api.getDocsUrl(async () => ({
      ok: true,
      json: async () => ({ success: true, data: { docs_link: "javascript:alert(1)" } }),
    })),
    null,
  );
  assert.equal(
    await api.getDocsUrl(async () => {
      throw new Error("network");
    }),
    null,
  );
});

test("updates navigation and mega-footer links with safe external attributes", async () => {
  const api = loadApi();
  const links = [
    { setAttribute(name, value) { this[name] = value; } },
    { setAttribute(name, value) { this[name] = value; } },
  ];
  const root = {
    querySelectorAll(selector) {
      assert.match(selector, /\.nav/);
      assert.match(selector, /\.megafoot/);
      return links;
    },
  };

  const result = await api.configureDocsLinks(root, async () => ({
    ok: true,
    json: async () => ({
      success: true,
      data: { docs_link: "https://docs.example.com/start" },
    }),
  }));

  assert.equal(result, "https://docs.example.com/start");
  for (const link of links) {
    assert.equal(link.href, "https://docs.example.com/start");
    assert.equal(link.target, "_blank");
    assert.equal(link.rel, "noopener noreferrer");
  }
});

test("keeps the local documentation fallback when no safe setting is available", async () => {
  const api = loadApi();
  const link = {
    href: "/docs",
    setAttribute(name, value) {
      this[name] = value;
    },
  };
  const root = { querySelectorAll: () => [link] };

  const result = await api.configureDocsLinks(root, async () => ({
    ok: true,
    json: async () => ({ success: true, data: { docs_link: "" } }),
  }));

  assert.equal(result, null);
  assert.deepEqual(link, {
    href: "/docs",
    setAttribute: link.setAttribute,
  });
});
