const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const helper = require("./browser_evidence_helper.cjs");

test("captureScreenshot rejects paths and writes only masked screenshot buffer", async () => {
  const runtime = fs.mkdtempSync(path.join(os.tmpdir(), "browser-evidence-"));
  const calls = [];
  const page = {
    locator(selector) {
      return { selector };
    },
    getByText(text, options) {
      return { text, options };
    },
    async screenshot(options) {
      calls.push(options);
      assert.equal(Object.prototype.hasOwnProperty.call(options, "path"), false);
      assert.ok(Array.isArray(options.mask));
      assert.ok(options.mask.length >= 6);
      return Buffer.from("\x89PNG\r\n\x1a\nmasked", "binary");
    },
  };

  await assert.rejects(() => helper.captureScreenshot(page, runtime, "../bad", []), /invalid screenshot name/);
  await assert.rejects(() => helper.captureScreenshot(page, runtime, "bad.png", []), /invalid screenshot name/);
  await assert.rejects(() => helper.captureScreenshot(page, runtime, "bad\\name", []), /invalid screenshot name/);

  const result = await helper.captureScreenshot(page, runtime, "checkpoint", ["654321", "sk-12345678"]);
  assert.equal(result.path, "screenshots/checkpoint.png");
  assert.equal(calls.length, 1);
  const output = path.join(runtime, result.path);
  assert.ok(fs.readFileSync(output).includes(Buffer.from("masked")));
  await assert.rejects(() => helper.captureScreenshot(page, runtime, "checkpoint", []), /already exists/);
});

test("event projection drops network secrets and redacts console text", () => {
  const consoleEvent = helper.projectConsoleEvent({ type: "log", text: "code 654321 sk-12345678", args: ["secret"] });
  assert.deepEqual(consoleEvent, { type: "log", text: "code [REDACTED_CODE] [REDACTED_API_KEY]", location: undefined });

  const networkEvent = helper.projectNetworkEvent({
    url: "https://staging-console.flatkey.ai/api?token=secret",
    method: "POST",
    status: 200,
    timing: { startTime: 1 },
    headers: { cookie: "secret" },
    postData: "secret",
  });
  assert.deepEqual(Object.keys(networkEvent).sort(), ["method", "status", "timing", "url"]);
});

test("long lived helper masks known values, blocks service workers, buffers evidence, and closes browser", async () => {
  const runtime = fs.mkdtempSync(path.join(os.tmpdir(), "browser-evidence-session-"));
  const calls = [];
  const page = {
    locator(selector) {
      return { selector };
    },
    getByText(text, options) {
      calls.push(["mask-value", text, options]);
      return { text, options };
    },
    async screenshot(options) {
      calls.push(["screenshot", options]);
      return Buffer.from("\x89PNG\r\n\x1a\nmasked", "binary");
    },
  };
  const context = {
    pages() {
      return [page];
    },
    async addInitScript(source) {
      calls.push(["init-script", source]);
    },
    async newCDPSession(selectedPage) {
      calls.push(["cdp-session", selectedPage === page]);
      return {
        async send(method, params) {
          calls.push(["cdp-send", method, params]);
        },
      };
    },
    on(name, handler) {
      calls.push(["context-on", name]);
      if (name === "requestfinished") {
        handler({
          url: () => "https://staging-console.flatkey.ai/api?token=secret",
          method: () => "POST",
          response: async () => ({ status: () => 201 }),
          timing: () => ({ startTime: 2 }),
        });
      }
      if (name === "requestfailed") {
        handler({
          url: () => "https://staging-console.flatkey.ai/fail",
          method: () => "GET",
          failure: () => ({ errorText: "sk-12345678" }),
          timing: () => ({ startTime: 3 }),
        });
      }
    },
  };
  const browser = {
    contexts() {
      return [context];
    },
    async close() {
      calls.push(["close"]);
    },
  };
  page.on = (name, handler) => {
    calls.push(["page-on", name]);
    if (name === "console") {
      handler({
        type: () => "log",
        text: () => "hello owner+alias@gmail.com 654321",
        location: () => ({ url: "https://staging-console.flatkey.ai", lineNumber: 1 }),
      });
    }
  };

  const session = new helper.BrowserEvidenceSession({
    browser,
    runtimeDir: runtime,
    sensitiveValues: ["owner+alias@gmail.com", "owner@gmail.com", "alias", "pw-secret", "654321"],
    maxEvents: 5,
  });
  await session.start();
  const shot = await session.captureScreenshot("checkpoint");
  const flushed = await session.flush();
  await session.stop();

  assert.equal(shot.path, "screenshots/checkpoint.png");
  assert.ok(calls.some((call) => call[0] === "init-script" && call[1].includes("serviceWorker")));
  assert.ok(calls.some((call) => call[0] === "cdp-send" && call[1] === "Network.setBypassServiceWorker" && call[2].bypass === true));
  assert.ok(calls.some((call) => call[0] === "mask-value" && call[1] === "owner+alias@gmail.com"));
  assert.ok(calls.some((call) => call[0] === "mask-value" && call[1] === "654321"));
  assert.equal(flushed.console.length, 1);
  assert.equal(flushed.network.length, 2);
  assert.ok(!JSON.stringify(flushed).includes("owner+alias@gmail.com"));
  assert.deepEqual(calls.at(-1), ["close"]);
});

test("long lived helper fails closed when service worker blocking cannot be applied", async () => {
  const browser = {
    contexts() {
      return [{
        pages() {
          return [];
        },
      }];
    },
  };
  const session = new helper.BrowserEvidenceSession({ browser, runtimeDir: os.tmpdir(), sensitiveValues: [] });
  await assert.rejects(() => session.start(), /service worker blocking unavailable/);
});

test("long lived helper fails closed when service worker bypass cannot be applied", async () => {
  const page = {
    on() {},
  };
  const browser = {
    contexts() {
      return [{
        pages() {
          return [page];
        },
        async addInitScript() {},
        serviceWorkers() {
          return [];
        },
      }];
    },
  };
  const session = new helper.BrowserEvidenceSession({ browser, runtimeDir: os.tmpdir(), sensitiveValues: [] });
  await assert.rejects(() => session.start(), /service worker bypass unavailable/);
});

test("long lived helper fails closed on event count and byte overflow", async () => {
  const session = new helper.BrowserEvidenceSession({
    browser: { contexts: () => [] },
    runtimeDir: os.tmpdir(),
    sensitiveValues: [],
    maxEvents: 1,
    maxEventBytes: 64,
    maxTotalBytes: 128,
  });

  session._pushConsole({ type: "log", text: "first" });
  assert.throws(() => session._pushConsole({ type: "log", text: "second" }), /browser evidence event limit exceeded/);
  await assert.rejects(() => session.flush(), /browser evidence overflow/);

  const largeEvent = new helper.BrowserEvidenceSession({
    browser: { contexts: () => [] },
    runtimeDir: os.tmpdir(),
    sensitiveValues: [],
    maxEventBytes: 32,
    maxTotalBytes: 1024,
  });
  assert.throws(() => largeEvent._pushConsole({ type: "log", text: "x".repeat(100) }), /browser evidence event too large/);

  const totalOverflow = new helper.BrowserEvidenceSession({
    browser: { contexts: () => [] },
    runtimeDir: os.tmpdir(),
    sensitiveValues: [],
    maxEvents: 10,
    maxEventBytes: 128,
    maxTotalBytes: 90,
  });
  totalOverflow._pushNetwork({ url: "https://staging-console.flatkey.ai/a", method: "GET" });
  assert.throws(() => totalOverflow._pushNetwork({ url: "https://staging-console.flatkey.ai/b", method: "GET" }), /browser evidence total too large/);
});

test("new page setup pending blocks capture and flush until service worker bypass completes", async () => {
  const runtime = fs.mkdtempSync(path.join(os.tmpdir(), "browser-evidence-pending-page-"));
  const oldPage = fakeScreenshotPage();
  const newPage = fakeScreenshotPage();
  const deferred = createDeferred();
  let pageHandler;
  const context = {
    pages: () => [oldPage],
    async addInitScript() {},
    serviceWorkers: () => [],
    async newCDPSession(page) {
      if (page === newPage) {
        await deferred.promise;
      }
      return { send: async () => {} };
    },
    on(name, handler) {
      if (name === "page") {
        pageHandler = handler;
      }
    },
  };
  const session = new helper.BrowserEvidenceSession({ browser: { contexts: () => [context] }, runtimeDir: runtime });
  await session.start();

  pageHandler(newPage);
  let captureSettled = false;
  const capture = session.captureScreenshot("after-new-page").finally(() => {
    captureSettled = true;
  });
  await delay(20);
  assert.equal(captureSettled, false);

  deferred.resolve();
  await capture;
  await session.flush();
});

test("new page setup rejection makes capture and flush fail closed", async () => {
  const runtime = fs.mkdtempSync(path.join(os.tmpdir(), "browser-evidence-rejected-page-"));
  const oldPage = fakeScreenshotPage();
  const newPage = fakeScreenshotPage();
  let pageHandler;
  const context = {
    pages: () => [oldPage],
    async addInitScript() {},
    serviceWorkers: () => [],
    async newCDPSession(page) {
      if (page === newPage) {
        throw new Error("cdp bypass failed");
      }
      return { send: async () => {} };
    },
    on(name, handler) {
      if (name === "page") {
        pageHandler = handler;
      }
    },
  };
  const session = new helper.BrowserEvidenceSession({ browser: { contexts: () => [context] }, runtimeDir: runtime });
  await session.start();

  pageHandler(newPage);
  await delay(20);

  await assert.rejects(() => session.captureScreenshot("after-failed-page"), /browser page setup failed/);
  await assert.rejects(async () => session.flush(), /browser page setup failed/);
});

test("new page setup success attaches listeners and keeps evidence usable", async () => {
  const runtime = fs.mkdtempSync(path.join(os.tmpdir(), "browser-evidence-ready-page-"));
  const oldPage = fakeScreenshotPage();
  const newPage = fakeScreenshotPage();
  const calls = [];
  let pageHandler;
  const context = {
    pages: () => [oldPage],
    async addInitScript() {},
    serviceWorkers: () => [],
    async newCDPSession(page) {
      calls.push(["cdp", page === newPage ? "new" : "old"]);
      return { send: async (method, params) => calls.push(["send", method, params]) };
    },
    on(name, handler) {
      if (name === "page") {
        pageHandler = handler;
      }
    },
  };
  const session = new helper.BrowserEvidenceSession({ browser: { contexts: () => [context] }, runtimeDir: runtime });
  await session.start();

  pageHandler(newPage);
  await session.captureScreenshot("after-ready-page");
  newPage.emitConsole("new page ready");
  const flushed = await session.flush();

  assert.ok(calls.some((call) => call[0] === "cdp" && call[1] === "new"));
  assert.ok(calls.some((call) => call[0] === "send" && call[1] === "Network.setBypassServiceWorker" && call[2].bypass === true));
  assert.equal(flushed.console.length, 1);
  assert.equal(flushed.console[0].text, "new page ready");
});

function fakeScreenshotPage() {
  const listeners = {};
  return {
    locator(selector) {
      return { selector };
    },
    getByText(text, options) {
      return { text, options };
    },
    async screenshot(options) {
      assert.ok(Array.isArray(options.mask));
      return Buffer.from("\x89PNG\r\n\x1a\nmasked", "binary");
    },
    on(name, handler) {
      listeners[name] = handler;
    },
    emitConsole(text) {
      listeners.console({
        type: () => "log",
        text: () => text,
        location: () => ({ url: "https://staging-console.flatkey.ai", lineNumber: 1 }),
      });
    },
  };
}

function createDeferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
