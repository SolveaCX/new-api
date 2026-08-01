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
