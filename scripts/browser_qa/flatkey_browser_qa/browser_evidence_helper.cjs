const fs = require("node:fs");
const path = require("node:path");

const SAFE_LOGICAL_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/;
const API_KEY_PATTERN = /\bsk-[A-Za-z0-9_-]{8,}\b/g;

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

function projectConsoleEvent(event) {
  return {
    type: event && event.type,
    text: redactText(event && event.text),
    location: event && event.location,
  };
}

function projectNetworkEvent(event) {
  const projected = {};
  for (const key of ["url", "method", "status", "timing", "error"]) {
    if (event && Object.prototype.hasOwnProperty.call(event, key)) {
      projected[key] = key === "url" || key === "error" ? redactText(event[key]) : event[key];
    }
  }
  return projected;
}

function redactText(value) {
  if (typeof value !== "string") {
    return value;
  }
  return value.replace(API_KEY_PATTERN, "[REDACTED_API_KEY]").replace(/\b\d{6}\b/g, "[REDACTED_CODE]");
}

module.exports = {
  buildMask,
  captureScreenshot,
  projectConsoleEvent,
  projectNetworkEvent,
  safeScreenshotPath,
};
