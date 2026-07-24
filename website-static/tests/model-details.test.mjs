import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import vm from "node:vm";

function read(relativePath) {
  return readFileSync(new URL(relativePath, import.meta.url), "utf8");
}

function catalog() {
  const window = {};
  vm.runInNewContext(read("../html/assets/model-catalog.js"), { window });
  return window.FLATKEY_MODEL_CATALOG;
}

test("the public image catalog exactly matches the six callable platform models", () => {
  const models = catalog();
  const html = read("../html/models.html");
  const detail = read("../html/assets/model-detail.js");
  const expected = [
    "gemini-2.5-flash-image",
    "gemini-3-pro-image",
    "gemini-3.1-flash-image",
    "gemini-3.1-flash-lite-image",
    "gpt-image-2",
    "nano-banana-pro-preview",
  ];

  assert.deepEqual(
    Object.keys(models).filter((id) => models[id].kind === "image").sort(),
    expected,
  );
  for (const model of expected) {
    assert.equal(models[model].kind, "image");
    assert.match(html, new RegExp(`<b>${model.replaceAll(".", "\\.")}</b>[\\s\\S]*?Available`));
  }
  assert.equal(models["gpt-image-2"].api, "images");
  for (const model of expected.filter((id) => id !== "gpt-image-2")) {
    assert.equal(models[model].api, "chat-image");
  }
  assert.doesNotMatch(html, /grok-imagine-image/i);
  assert.doesNotMatch(detail, /grok-imagine-image/i);
});

test("every displayed model receives a detail destination", () => {
  const html = read("../html/models.html");
  const models = catalog();
  const listed = [...html.matchAll(/<div class="m">[\s\S]*?<b>([^<]+)<\/b>[\s\S]*?<\/tr>/g)]
    .map((match) => match[1]);

  assert.ok(listed.length >= 31);
  for (const model of new Set(listed)) {
    assert.ok(models[model], model + " must exist in the shared model catalog");
  }
  assert.match(html, /detail\.href = "\/models\/" \+ encodeURIComponent\(model\)/);
});

test("detail examples use the production endpoint for every modality", () => {
  const detail = read("../html/assets/model-detail.js");
  const nginx = read("../nginx.conf");

  for (const endpoint of [
    "/v1/chat/completions",
    "/v1/images/generations",
    "/v1/video/generations",
    "/v1/videos/",
    "/v1/audio/speech",
    "/v1/audio/transcriptions",
    "/v1/realtime?model=",
  ]) {
    assert.ok(detail.includes(endpoint), endpoint + " example is missing");
  }
  assert.match(detail, /api === "chat-image"/);
  assert.match(detail, /Generated image is returned as a Markdown data URI/);
  assert.match(nginx, /location ~ \^\/models\/\[a-zA-Z0-9\._-\]\+\/\?\$/);
});
