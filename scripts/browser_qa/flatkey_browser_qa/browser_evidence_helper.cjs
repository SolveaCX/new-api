const fs = require("node:fs");
const readline = require("node:readline");
const path = require("node:path");

const SAFE_LOGICAL_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const API_KEY_PATTERN = /\bsk-[A-Za-z0-9_-]{8,}\b/g;
const MAX_EVENTS = 1000;
const MAX_EVENT_BYTES = 64 * 1024;
const MAX_TOTAL_BYTES = 2 * 1024 * 1024;
const INIT_FAILURE_MESSAGES = Object.freeze({
  init_connect_failed: "browser connection failed",
  init_context_failed: "default browser context unavailable",
  init_websocket_block_failed: "websocket blocking unavailable",
  init_download_block_failed: "download blocking unavailable",
  init_service_worker_block_failed: "service worker blocking unavailable",
  init_page_failed: "browser page unavailable",
  init_service_worker_bypass_failed: "service worker bypass unavailable",
  init_failed: "browser evidence initialization failed",
});
const SAFE_PROTOCOL_ERROR_CODES = new Set([...Object.keys(INIT_FAILURE_MESSAGES), "command_failed"]);

class BrowserEvidenceHelperError extends Error {
  constructor(code) {
    super(INIT_FAILURE_MESSAGES[code] || "browser evidence helper failed");
    this.name = "BrowserEvidenceHelperError";
    this.code = SAFE_PROTOCOL_ERROR_CODES.has(code) ? code : "command_failed";
  }
}

async function runInitStage(code, operation) {
  try {
    return await operation();
  } catch (_error) {
    throw new BrowserEvidenceHelperError(code);
  }
}

function safeScreenshotPath(runtimeDir, logicalName) {
  if (typeof logicalName !== "string" || !SAFE_LOGICAL_NAME.test(logicalName)) {
    throw new Error("invalid screenshot name");
  }
  if (logicalName.endsWith(".png") || logicalName.includes("/") || logicalName.includes("\\") || logicalName.includes("..")) {
    throw new Error("invalid screenshot name");
  }
  const root = fs.realpathSync(runtimeDir);
  const screenshotDir = path.join(root, "screenshots");
  fs.mkdirSync(screenshotDir, { recursive: true, mode: 0o700 });
  const dirReal = fs.realpathSync(screenshotDir);
  if (dirReal !== path.join(root, "screenshots")) {
    throw new Error("screenshot directory must be regular runtime directory");
  }
  const target = path.join(dirReal, `${logicalName}.png`);
  if (!target.startsWith(dirReal + path.sep)) {
    throw new Error("screenshot path escaped runtime");
  }
  if (fs.existsSync(target)) {
    throw new Error("screenshot already exists");
  }
  return target;
}

async function buildMask(page, sensitiveValues = []) {
  const masks = [];
  for (const selector of [
    "input",
    "textarea",
    "[contenteditable=true]",
    "[autocomplete='one-time-code']",
    "input[name*=code i]",
    "input[id*=code i]",
  ]) {
    if (typeof page.locator === "function") {
      masks.push(page.locator(selector));
    }
  }
  const secrets = Array.isArray(sensitiveValues) ? sensitiveValues : [];
  for (const value of secrets) {
    if (typeof value === "string" && value.length > 0 && typeof page.getByText === "function") {
      masks.push(page.getByText(value, { exact: true }));
    }
  }
  if (typeof page.getByText === "function") {
    masks.push(page.getByText(/\bsk-[A-Za-z0-9_-]{8,}\b/));
  }
  return masks;
}

async function captureScreenshot(page, runtimeDir, logicalName, sensitiveValues = []) {
  const target = safeScreenshotPath(runtimeDir, logicalName);
  const mask = await buildMask(page, sensitiveValues);
  if (mask.length === 0) {
    throw new Error("screenshot mask is required");
  }
  const buffer = await page.screenshot({ mask, type: "png" });
  if (!Buffer.isBuffer(buffer)) {
    throw new Error("playwright did not return screenshot buffer");
  }
  const fd = fs.openSync(target, fs.constants.O_CREAT | fs.constants.O_EXCL | fs.constants.O_WRONLY, 0o600);
  try {
    fs.writeFileSync(fd, buffer);
  } finally {
    fs.closeSync(fd);
  }
  return { path: `screenshots/${path.basename(target)}` };
}

function projectConsoleEvent(event, sensitiveValues = []) {
  return {
    type: event && event.type,
    text: redactText(event && event.text, sensitiveValues),
    location: event && event.location,
  };
}

function projectNetworkEvent(event, sensitiveValues = []) {
  const projected = {};
  for (const key of ["url", "method", "status", "timing", "error"]) {
    if (event && Object.prototype.hasOwnProperty.call(event, key)) {
      projected[key] = key === "url" || key === "error" ? redactText(event[key], sensitiveValues) : event[key];
    }
  }
  return projected;
}

function isChromiumInternalServiceWorker(worker) {
  try {
    if (typeof worker?.url !== "function") {
      return false;
    }
    const workerUrl = worker.url();
    return typeof workerUrl === "string" && new URL(workerUrl).protocol === "chrome-extension:";
  } catch (_error) {
    return false;
  }
}

class BrowserEvidenceSession {
  constructor({ browser, runtimeDir, sensitiveValues = [], maxEvents = MAX_EVENTS, maxEventBytes = MAX_EVENT_BYTES, maxTotalBytes = MAX_TOTAL_BYTES, docsProxyUrl = null }) {
    this.browser = browser;
    this.runtimeDir = runtimeDir;
    this.sensitiveValues = Array.isArray(sensitiveValues) ? sensitiveValues.filter((value) => typeof value === "string" && value.length > 0) : [];
    this.maxEvents = maxEvents;
    this.maxEventBytes = maxEventBytes;
    this.maxTotalBytes = maxTotalBytes;
    this.totalBytes = 0;
    this.overflowError = null;
    this.console = [];
    this.network = [];
    this.context = null;
    this.page = null;
    this.docsProxyUrl = docsProxyUrl;
    this.pageSetupPromises = new Set();
    this.pageSetupError = null;
  }

  async start() {
    const contexts = await runInitStage(
      "init_context_failed",
      async () => typeof this.browser.contexts === "function" ? this.browser.contexts() : [],
    );
    this.context = contexts[0];
    if (!this.context) {
      throw new BrowserEvidenceHelperError("init_context_failed");
    }
    await runInitStage("init_websocket_block_failed", () => this._blockWebSockets());
    await runInitStage("init_download_block_failed", () => this._denyDownloads());
    await runInitStage("init_service_worker_block_failed", () => this._blockServiceWorkers());
    await runInitStage("init_context_failed", async () => this._attachContextListeners());
    const pages = await runInitStage(
      "init_page_failed",
      async () => typeof this.context.pages === "function" ? this.context.pages() : [],
    );
    this.page = pages[0] || await runInitStage(
      "init_page_failed",
      async () => typeof this.context.newPage === "function" ? this.context.newPage() : null,
    );
    if (!this.page) {
      throw new BrowserEvidenceHelperError("init_page_failed");
    }
    await runInitStage("init_service_worker_bypass_failed", () => this._setupPage(this.page));
    if (typeof this.context.on === "function") {
      await runInitStage("init_context_failed", async () => {
        this.context.on("page", (page) => {
          this._trackPageSetup(page);
        });
      });
    }
    return this;
  }

  async _blockWebSockets() {
    if (typeof this.context.routeWebSocket !== "function") {
      throw new Error("websocket blocking unavailable");
    }
    try {
      await this.context.routeWebSocket("**/*", async (socket) => {
        if (socket && typeof socket.close === "function") {
          await socket.close();
        }
      });
    } catch (_error) {
      throw new Error("websocket blocking unavailable");
    }
  }

  async _denyDownloads() {
    if (!this.browser || typeof this.browser.newBrowserCDPSession !== "function") {
      throw new Error("download blocking unavailable");
    }
    let session;
    try {
      session = await this.browser.newBrowserCDPSession();
      if (!session || typeof session.send !== "function") {
        throw new Error("download blocking unavailable");
      }
      await session.send("Browser.setDownloadBehavior", { behavior: "deny" });
    } catch (_error) {
      throw new Error("download blocking unavailable");
    } finally {
      if (session && typeof session.detach === "function") {
        await session.detach().catch(() => {});
      }
    }
  }

  async _blockServiceWorkers() {
    if (typeof this.context.addInitScript !== "function") {
      throw new Error("service worker blocking unavailable");
    }
    if (typeof this.context.on === "function") {
      this.context.on("serviceworker", (worker) => {
        if (!isChromiumInternalServiceWorker(worker)) {
          throw new Error("service worker registration blocked");
        }
      });
    }
    if (
      typeof this.context.serviceWorkers === "function"
      && this.context.serviceWorkers().some((worker) => !isChromiumInternalServiceWorker(worker))
    ) {
      throw new Error("service worker already registered");
    }
    await this.context.addInitScript(`
      if (navigator.serviceWorker) {
        navigator.serviceWorker.register = async () => {
          throw new Error("Service workers are blocked by Flatkey staging QA");
        };
      }
    `);
    if (
      typeof this.context.serviceWorkers === "function"
      && this.context.serviceWorkers().some((worker) => !isChromiumInternalServiceWorker(worker))
    ) {
      throw new Error("service worker already registered");
    }
  }

  _attachContextListeners() {
    if (typeof this.context.on !== "function") {
      return;
    }
    this.context.on("requestfinished", async (request) => {
      try {
        const response = typeof request.response === "function" ? await request.response() : null;
        this._pushNetwork({
          url: typeof request.url === "function" ? request.url() : undefined,
          method: typeof request.method === "function" ? request.method() : undefined,
          status: response && typeof response.status === "function" ? response.status() : undefined,
          timing: typeof request.timing === "function" ? request.timing() : undefined,
        });
      } catch (_error) {
        return;
      }
    });
    this.context.on("requestfailed", (request) => {
      try {
        const failure = typeof request.failure === "function" ? request.failure() : null;
        this._pushNetwork({
          url: typeof request.url === "function" ? request.url() : undefined,
          method: typeof request.method === "function" ? request.method() : undefined,
          error: failure && failure.errorText,
          timing: typeof request.timing === "function" ? request.timing() : undefined,
        });
      } catch (_error) {
        return;
      }
    });
  }

  async _bypassServiceWorkerForPage(page) {
    if (typeof this.context.newCDPSession !== "function") {
      throw new Error("service worker bypass unavailable");
    }
    const session = await this.context.newCDPSession(page);
    if (!session || typeof session.send !== "function") {
      throw new Error("service worker bypass unavailable");
    }
    await session.send("Network.setBypassServiceWorker", { bypass: true });
  }

  async _setupPage(page) {
    await this._bypassServiceWorkerForPage(page);
    this._attachPage(page);
  }

  _trackPageSetup(page) {
    let setupPromise;
    setupPromise = this._setupPage(page)
      .catch((error) => {
        if (!this.pageSetupError) {
          this.pageSetupError = error;
        }
        throw error;
      })
      .finally(() => {
        this.pageSetupPromises.delete(setupPromise);
      });
    this.pageSetupPromises.add(setupPromise);
    setupPromise.catch(() => {});
    return setupPromise;
  }

  async _awaitPageSetups() {
    while (this.pageSetupPromises.size > 0) {
      const pendingSetups = Array.from(this.pageSetupPromises);
      await Promise.allSettled(pendingSetups);
      if (this.pageSetupError) {
        throw new Error("browser page setup failed");
      }
    }
    if (this.pageSetupError) {
      throw new Error("browser page setup failed");
    }
  }

  _attachPage(page) {
    if (!page || typeof page.on !== "function") {
      return;
    }
    page.on("console", (message) => {
      this._pushConsole({
        type: typeof message.type === "function" ? message.type() : undefined,
        text: typeof message.text === "function" ? message.text() : undefined,
        location: typeof message.location === "function" ? message.location() : undefined,
      });
    });
  }

  addSensitiveValues(values) {
    if (!Array.isArray(values)) {
      throw new Error("sensitive values must be an array");
    }
    for (const value of values) {
      if (typeof value === "string" && value.length > 0 && !this.sensitiveValues.includes(value)) {
        this.sensitiveValues.push(value);
      }
    }
  }

  async captureScreenshot(name) {
    await this._awaitPageSetups();
    return captureScreenshot(this.page, this.runtimeDir, name, this.sensitiveValues);
  }

  async flush() {
    await this._awaitPageSetups();
    if (this.overflowError) {
      throw new Error("browser evidence overflow");
    }
    const payload = {
      console: this.console.splice(0, this.console.length),
      network: this.network.splice(0, this.network.length),
    };
    return payload;
  }

  async readDocs(url) {
    const target = validateDocsUrl(url);
    if (!this.docsProxyUrl) {
      throw new Error("docs proxy unavailable");
    }
    if (!this.browser || typeof this.browser.newContext !== "function") {
      throw new Error("docs context unavailable");
    }
    const context = await this.browser.newContext({
      proxy: { server: this.docsProxyUrl },
      javaScriptEnabled: false,
      serviceWorkers: "block",
      acceptDownloads: false,
      storageState: { cookies: [], origins: [] },
    });
    try {
      if (typeof context.routeWebSocket === "function") {
        await context.routeWebSocket("**/*", async (socket) => {
          if (socket && typeof socket.close === "function") {
            await socket.close();
          }
        });
      }
      await context.route("**/*", async (route) => {
        const request = route.request();
        const requestUrl = typeof request.url === "function" ? request.url() : "";
        const method = typeof request.method === "function" ? request.method() : "";
        if (!isAllowedDocsRequest(requestUrl, method)) {
          await route.abort();
          return;
        }
        await route.continue({ headers: stripSensitiveRequestHeaders(request.headers()) });
      });
      const page = await context.newPage();
      try {
        await page.goto(target.href, { waitUntil: "domcontentloaded", timeout: 30000 });
      } catch (error) {
        throw new Error("docs read failed");
      }
      const finalUrl = typeof page.url === "function" ? page.url() : target.href;
      if (!isAllowedDocsRequest(finalUrl, "GET")) {
        throw new Error("docs redirect blocked");
      }
      const html = typeof page.content === "function" ? await page.content() : "";
      return { url: finalUrl, status: 200, text: textFromHtml(html) };
    } finally {
      await context.close();
    }
  }

  async stop() {
    if (this.browser && typeof this.browser.close === "function") {
      await this.browser.close();
    }
  }

  _pushConsole(event) {
    this._pushBounded(this.console, projectConsoleEvent(event, this.sensitiveValues));
  }

  _pushNetwork(event) {
    this._pushBounded(this.network, projectNetworkEvent(event, this.sensitiveValues));
  }

  _pushBounded(target, event) {
    if (this.overflowError) {
      throw this.overflowError;
    }
    if (this.console.length + this.network.length >= this.maxEvents) {
      this.overflowError = new Error("browser evidence event limit exceeded");
      throw this.overflowError;
    }
    const size = Buffer.byteLength(JSON.stringify(event), "utf8");
    if (size > this.maxEventBytes) {
      this.overflowError = new Error("browser evidence event too large");
      throw this.overflowError;
    }
    if (this.totalBytes + size > this.maxTotalBytes) {
      this.overflowError = new Error("browser evidence total too large");
      throw this.overflowError;
    }
    this.totalBytes += size;
    target.push(event);
  }
}

async function runProtocol({ input = process.stdin, output = process.stdout, connectOverCDP } = {}) {
  const selectedConnect = connectOverCDP || defaultConnectOverCDP;
  let session = null;
  const rl = readline.createInterface({ input, crlfDelay: Infinity });
  for await (const line of rl) {
    let request;
    try {
      request = JSON.parse(line);
      if (!request || typeof request !== "object" || typeof request.id !== "number" || typeof request.command !== "string") {
        throw new Error("invalid request");
      }
      const params = request.params && typeof request.params === "object" ? request.params : {};
      let result = {};
      if (request.command === "init") {
        try {
          const browser = await runInitStage("init_connect_failed", () => selectedConnect(params.cdpEndpoint));
          session = await new BrowserEvidenceSession({
            browser,
            runtimeDir: params.runtimeDir,
            sensitiveValues: params.sensitiveValues,
            docsProxyUrl: params.docsProxyUrl,
          }).start();
        } catch (error) {
          if (error instanceof BrowserEvidenceHelperError) {
            throw error;
          }
          throw new BrowserEvidenceHelperError("init_failed");
        }
      } else if (request.command === "captureScreenshot") {
        requireSession(session);
        result = await session.captureScreenshot(params.name);
      } else if (request.command === "addSensitiveValues") {
        requireSession(session);
        session.addSensitiveValues(params.values);
      } else if (request.command === "flush") {
        requireSession(session);
        result = await session.flush();
      } else if (request.command === "readDocs") {
        requireSession(session);
        result = await session.readDocs(params.url);
      } else if (request.command === "close") {
        if (session) {
          await session.stop();
          session = null;
        }
      } else {
        throw new Error("unknown command");
      }
      output.write(JSON.stringify({ id: request.id, ok: true, result }) + "\n");
    } catch (error) {
      const id = request && typeof request.id === "number" ? request.id : 0;
      const fallback = request && request.command === "init" ? "init_failed" : "command_failed";
      const errorCode = error instanceof BrowserEvidenceHelperError && SAFE_PROTOCOL_ERROR_CODES.has(error.code)
        ? error.code
        : fallback;
      output.write(JSON.stringify({ id, ok: false, error: errorCode }) + "\n");
    }
  }
}

function requireSession(session) {
  if (!session) {
    throw new Error("browser evidence helper not initialized");
  }
}

async function defaultConnectOverCDP(endpoint) {
  const { chromium } = require("playwright-core");
  return chromium.connectOverCDP(endpoint);
}

function validateDocsUrl(value) {
  let parsed;
  try {
    parsed = new URL(value);
  } catch (_error) {
    throw new Error("docs url blocked");
  }
  if (parsed.protocol !== "https:" || parsed.origin !== "https://docs.flatkey.ai" || parsed.username || parsed.password) {
    throw new Error("docs url blocked");
  }
  return parsed;
}

function isAllowedDocsRequest(value, method) {
  let parsed;
  try {
    parsed = new URL(value);
  } catch (_error) {
    return false;
  }
  return parsed.protocol === "https:" && parsed.origin === "https://docs.flatkey.ai" && (method === "GET" || method === "HEAD");
}

function stripSensitiveRequestHeaders(headers = {}) {
  const cleaned = {};
  for (const [key, value] of Object.entries(headers || {})) {
    const lower = key.toLowerCase();
    if (lower === "cookie" || lower === "authorization" || lower === "proxy-authorization") {
      continue;
    }
    cleaned[key] = value;
  }
  return cleaned;
}

function textFromHtml(html) {
  if (typeof html !== "string") {
    return "";
  }
  return html.replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, " ")
    .replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, 60000);
}

function redactText(value, sensitiveValues = []) {
  if (typeof value !== "string") {
    return value;
  }
  let redacted = value.replace(API_KEY_PATTERN, "[REDACTED_API_KEY]").replace(/\b\d{6}\b/g, "[REDACTED_CODE]");
  for (const secret of sensitiveValues) {
    if (typeof secret === "string" && secret.length > 0) {
      redacted = redacted.split(secret).join("[REDACTED_SECRET]");
    }
  }
  return redacted;
}

module.exports = {
  BrowserEvidenceSession,
  buildMask,
  captureScreenshot,
  projectConsoleEvent,
  projectNetworkEvent,
  runProtocol,
  safeScreenshotPath,
  stripSensitiveRequestHeaders,
  validateDocsUrl,
};

if (require.main === module) {
  runProtocol().catch(() => process.exit(1));
}
