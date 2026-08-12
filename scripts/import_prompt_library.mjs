#!/usr/bin/env node

import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const defaultSourceFile = "website/src/lib/generated-prompt-items.json";
const importOrigin = normalizeOrigin(process.env.PROMPT_LIBRARY_IMPORT_ORIGIN || process.env.APP_CONSOLE_ORIGIN || "http://localhost:3000");
const importToken = process.env.PROMPT_LIBRARY_IMPORT_TOKEN || "";
const sourceFile = resolve(process.cwd(), process.env.PROMPT_LIBRARY_IMPORT_FILE || defaultSourceFile);
const batchSize = clampInt(process.env.PROMPT_LIBRARY_IMPORT_BATCH_SIZE, 100, 1, 100);
const modelOverride = process.env.PROMPT_LIBRARY_IMPORT_MODEL || "gpt-image-2";

if (!importToken.trim()) {
  console.error("PROMPT_LIBRARY_IMPORT_TOKEN is required.");
  process.exit(1);
}

const raw = await readFile(sourceFile, "utf8");
const items = JSON.parse(raw);
if (!Array.isArray(items) || items.length === 0) {
  console.error(`No prompt items found in ${sourceFile}.`);
  process.exit(1);
}

let imported = 0;
let rejected = 0;

for (let offset = 0; offset < items.length; offset += batchSize) {
  const batch = items.slice(offset, offset + batchSize).map(toImportItem);
  const response = await fetch(`${importOrigin}/api/prompt-library/import`, {
    method: "POST",
    headers: {
      authorization: `Bearer ${importToken}`,
      "content-type": "application/json",
    },
    body: JSON.stringify({ items: batch }),
  });
  const body = await response.text();
  if (!response.ok) {
    console.error(`Batch ${offset / batchSize + 1} failed with HTTP ${response.status}: ${body}`);
    process.exit(1);
  }
  const payload = JSON.parse(body);
  if (!payload.success) {
    console.error(`Batch ${offset / batchSize + 1} failed: ${body}`);
    process.exit(1);
  }
  imported += payload.data?.imported || 0;
  rejected += payload.data?.rejected || 0;
  console.log(`Batch ${offset / batchSize + 1}: imported ${payload.data?.imported || 0}, rejected ${payload.data?.rejected || 0}`);
}

console.log(`Done. imported=${imported}, rejected=${rejected}, total=${items.length}`);

function toImportItem(item) {
  const source = item.source || {};
  return {
    artifact: item.artifact,
    category: item.category,
    model: item.model === "stable-diffusion" ? modelOverride : item.model,
    output: item.output,
    prompt: item.prompt,
    slug: item.slug,
    source: {
      label: source.label,
      platform: source.platform,
      url: source.url,
      captured_at: source.captured_at || source.capturedAt || item.updatedAt || new Date().toISOString().slice(0, 10),
    },
    summary: item.summary,
    tags: item.tags,
    title: item.title,
  };
}

function normalizeOrigin(value) {
  return String(value || "").replace(/\/+$/, "");
}

function clampInt(value, fallback, min, max) {
  const parsed = Number.parseInt(value || "", 10);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.min(Math.max(parsed, min), max);
}
