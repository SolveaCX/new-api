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

test("docs reader uses fresh cookie-free docs proxy context, strips sensitive headers, and closes on success", async () => {
  const calls = [];
  const context = fakeDocsContext(calls, {
    async goto(url) {
      calls.push(["goto", url]);
    },
    async content() {
      return "<html><body>hello docs</body></html>";
    },
    url() {
      return "https://docs.flatkey.ai/quickstart";
    },
  });
  const browser = {
    async newContext(options) {
      calls.push(["newContext", options]);
      return context;
    },
  };
  const session = new helper.BrowserEvidenceSession({ browser, runtimeDir: os.tmpdir(), docsProxyUrl: "http://127.0.0.1:4568" });

  const result = await session.readDocs("https://docs.flatkey.ai/quickstart");
  const routeHandler = calls.find((call) => call[0] === "route")[2];
  const continued = [];
  await routeHandler({
    request: () => ({
      url: () => "https://docs.flatkey.ai/quickstart",
      method: () => "GET",
      headers: () => ({ Cookie: "secret=value", Authorization: "Bearer secret", Accept: "text/html" }),
    }),
    continue: async (options) => continued.push(options),
    abort: async () => calls.push(["abort"]),
  });

  assert.equal(result.status, 200);
  assert.equal(result.url, "https://docs.flatkey.ai/quickstart");
  assert.match(result.text, /hello docs/);
  assert.deepEqual(calls[0][1].storageState, { cookies: [], origins: [] });
  assert.equal(calls[0][1].serviceWorkers, "block");
  assert.equal(calls[0][1].proxy.server, "http://127.0.0.1:4568");
  assert.equal(continued[0].headers.Cookie, undefined);
  assert.equal(continued[0].headers.Authorization, undefined);
  assert.deepEqual(calls.at(-1), ["close"]);
});

test("docs reader blocks cookies, writes, cross-origin redirects, and closes on failure", async () => {
  const blockedRequests = [
    ["https://docs.flatkey.ai/quickstart", "POST"],
    ["https://console.flatkey.ai/", "GET"],
  ];
  for (const [url, method] of blockedRequests) {
    const calls = [];
    const context = fakeDocsContext(calls, {
      async goto() {
        throw new Error("blocked");
      },
      url() {
        return "https://docs.flatkey.ai/quickstart";
      },
    });
    const browser = { async newContext() { return context; } };
    const session = new helper.BrowserEvidenceSession({ browser, runtimeDir: os.tmpdir(), docsProxyUrl: "http://127.0.0.1:4568" });
    await assert.rejects(() => session.readDocs("https://docs.flatkey.ai/quickstart"), /docs read failed/);
    const routeHandler = calls.find((call) => call[0] === "route")[2];
    await routeHandler({
      request: () => ({ url: () => url, method: () => method, headers: () => ({}) }),
      continue: async () => calls.push(["continue"]),
      abort: async () => calls.push(["abort"]),
    });
    assert.ok(calls.some((call) => call[0] === "abort"));
    assert.deepEqual(calls.at(-1), ["abort"]);
  }

  const rejectedBeforeContext = new helper.BrowserEvidenceSession({
    browser: { async newContext() { throw new Error("must not create docs context"); } },
    runtimeDir: os.tmpdir(),
    docsProxyUrl: "http://127.0.0.1:4568",
  });
  await assert.rejects(() => rejectedBeforeContext.readDocs("http://169.254.169.254/latest"), /docs url blocked/);

  const calls = [];
  const context = fakeDocsContext(calls, {
    async goto() {},
    async content() {
      return "bad redirect";
    },
    url() {
      return "https://staging-console.flatkey.ai/login";
    },
  });
  const session = new helper.BrowserEvidenceSession({
    browser: { async newContext() { return context; } },
    runtimeDir: os.tmpdir(),
    docsProxyUrl: "http://127.0.0.1:4568",
  });
  await assert.rejects(() => session.readDocs("https://docs.flatkey.ai/redirect"), /docs redirect blocked/);
  assert.deepEqual(calls.at(-1), ["close"]);
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

test("new page setup drain includes pages opened while evidence operation is waiting", async () => {
  await assertNewPagesOpenedDuringSetupWaitAreDrained("capture", (session) => session.captureScreenshot("during-drain"));
  await assertNewPagesOpenedDuringSetupWaitAreDrained("flush", (session) => session.flush());
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

async function assertNewPagesOpenedDuringSetupWaitAreDrained(operationName, startOperation) {
  const runtime = fs.mkdtempSync(path.join(os.tmpdir(), `browser-evidence-drain-${operationName}-`));
  const oldPage = fakeScreenshotPage();
  const page1 = fakeScreenshotPage();
  const page2 = fakeScreenshotPage();
  const page1Ready = createDeferred();
  const page2Ready = createDeferred();
  let pageHandler;
  const context = {
    pages: () => [oldPage],
    async addInitScript() {},
    serviceWorkers: () => [],
    async newCDPSession(page) {
      if (page === page1) {
        await page1Ready.promise;
      } else if (page === page2) {
        await page2Ready.promise;
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

  pageHandler(page1);
  let operationSettled = false;
  const operation = startOperation(session).finally(() => {
    operationSettled = true;
  });
  await delay(20);
  assert.equal(operationSettled, false, `${operationName} settled before page1 setup completed`);

  pageHandler(page2);
  page1Ready.resolve();
  await delay(20);
  assert.equal(operationSettled, false, `${operationName} settled before page2 setup completed`);

  page2Ready.resolve();
  await operation;
}

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

function fakeDocsContext(calls, page) {
  return {
    async route(pattern, handler) {
      calls.push(["route", pattern, handler]);
    },
    async newPage() {
      calls.push(["newPage"]);
      return page;
    },
    async close() {
      calls.push(["close"]);
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
