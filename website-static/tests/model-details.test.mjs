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

test("Grok Imagine is registered as an image model without claiming verified availability", () => {
  const models = catalog();
  const html = read("../html/models.html");
  const detail = read("../html/assets/model-detail.js");

  assert.deepEqual(
    Object.keys(models).filter((id) => id.startsWith("grok-imagine-image")).sort(),
    ["grok-imagine-image", "grok-imagine-image-quality"],
  );
  assert.equal(models["grok-imagine-image"].kind, "image");
  assert.equal(models["grok-imagine-image-quality"].kind, "image");
  assert.match(html, /grok-imagine-image[\s\S]*Verification pending/);
  assert.match(detail, /New · verifying/);
  assert.match(detail, /provider metadata and health verification are still pending/);
});

test("every displayed model receives a detail destination", () => {
  const html = read("../html/models.html");
  const models = catalog();
  const listed = [...html.matchAll(/<div class="m">[\s\S]*?<b>([^<]+)<\/b>[\s\S]*?<\/tr>/g)]
    .map((match) => match[1]);

  assert.ok(listed.length >= 27);
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
  assert.doesNotMatch(
    detail,
    /imageBody[\s\S]{0,300}\b(?:size|quality|style)\s*:/,
    "xAI image examples must not send unsupported size/quality/style fields",
  );
  assert.match(nginx, /location ~ \^\/models\/\[a-zA-Z0-9\._-\]\+\/\?\$/);
});
