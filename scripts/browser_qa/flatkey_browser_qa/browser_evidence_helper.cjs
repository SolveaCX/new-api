const fs = require("node:fs");
const readline = require("node:readline");
const path = require("node:path");

const SAFE_LOGICAL_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const API_KEY_PATTERN = /\bsk-[A-Za-z0-9_-]{8,}\b/g;
const MAX_EVENTS = 1000;
const MAX_EVENT_BYTES = 64 * 1024;
const MAX_TOTAL_BYTES = 2 * 1024 * 1024;

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

class BrowserEvidenceSession {
  constructor({ browser, runtimeDir, sensitiveValues = [], maxEvents = MAX_EVENTS, maxEventBytes = MAX_EVENT_BYTES, maxTotalBytes = MAX_TOTAL_BYTES }) {
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
  }

  async start() {
    const contexts = typeof this.browser.contexts === "function" ? this.browser.contexts() : [];
    this.context = contexts[0];
    if (!this.context) {
      throw new Error("default browser context unavailable");
    }
    await this._blockServiceWorkers();
    this._attachContextListeners();
    const pages = typeof this.context.pages === "function" ? this.context.pages() : [];
    this.page = pages[0] || (typeof this.context.newPage === "function" ? await this.context.newPage() : null);
    if (!this.page) {
      throw new Error("browser page unavailable");
    }
    await this._bypassServiceWorkerForPage(this.page);
    this._attachPage(this.page);
    if (typeof this.context.on === "function") {
      this.context.on("page", async (page) => {
        await this._bypassServiceWorkerForPage(page);
        this._attachPage(page);
      });
    }
    return this;
  }

  async _blockServiceWorkers() {
    if (typeof this.context.addInitScript !== "function") {
      throw new Error("service worker blocking unavailable");
    }
    if (typeof this.context.serviceWorkers === "function" && this.context.serviceWorkers().length > 0) {
      throw new Error("service worker already registered");
    }
    await this.context.addInitScript(`
      if (navigator.serviceWorker) {
        navigator.serviceWorker.register = async () => {
          throw new Error("Service workers are blocked by Flatkey staging QA");
        };
      }
    `);
    if (typeof this.context.on === "function") {
      this.context.on("serviceworker", () => {
        throw new Error("service worker registration blocked");
      });
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
    return captureScreenshot(this.page, this.runtimeDir, name, this.sensitiveValues);
  }

  flush() {
    if (this.overflowError) {
      throw new Error("browser evidence overflow");
    }
    const payload = {
      console: this.console.splice(0, this.console.length),
      network: this.network.splice(0, this.network.length),
    };
    return payload;
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
        const browser = await selectedConnect(params.cdpEndpoint);
        session = await new BrowserEvidenceSession({
          browser,
          runtimeDir: params.runtimeDir,
          sensitiveValues: params.sensitiveValues,
        }).start();
      } else if (request.command === "captureScreenshot") {
        requireSession(session);
        result = await session.captureScreenshot(params.name);
      } else if (request.command === "addSensitiveValues") {
        requireSession(session);
        session.addSensitiveValues(params.values);
      } else if (request.command === "flush") {
        requireSession(session);
        result = session.flush();
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
      output.write(JSON.stringify({ id, ok: false, error: "browser evidence helper failed" }) + "\n");
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
};

if (require.main === module) {
  runProtocol().catch(() => process.exit(1));
}
