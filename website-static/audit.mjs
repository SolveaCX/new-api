#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const root = path.join(path.dirname(fileURLToPath(import.meta.url)), "html");
const files = fs.readdirSync(root).filter((file) => file.endsWith(".html")).sort();
const errors = [];
const warnings = [];
const fail = (file, message) => errors.push(`${file}: ${message}`);

const appOnly = new Set(["console.html", "login.html", "onboarding.html", "signup.html"]);
const legacyRoutes = new Set(["/about", "/blog", "/models", "/pricing", "/rankings"]);
const requiredNavRoutes = ["models.html", "docs.html", "playground.html", "topup.html", "compute.html", "usecases.html"];

for (const file of files) {
  const html = fs.readFileSync(path.join(root, file), "utf8");

  if (!/<html\b[^>]*\blang="[^"]+"/i.test(html)) fail(file, "missing html lang");
  const viewports = [...html.matchAll(/<meta\s+name="viewport"\s+content="([^"]+)"/gi)];
  if (viewports.length !== 1) fail(file, `expected one viewport meta, found ${viewports.length}`);
  else if (!viewports[0][1].includes("width=device-width")) fail(file, `non-responsive viewport: ${viewports[0][1]}`);
  if (/\bid=""/.test(html)) fail(file, "contains an empty id");
  if (/<script\b[^>]*\bsrc=""/i.test(html)) fail(file, "contains an empty script src");
  if (/href="(?:#|javascript:[^"]*)"/i.test(html)) fail(file, "contains a placeholder or javascript link");
  if (/\bdata-i18n(?:-ph)?=/.test(html) && !/assets\/i18n\.js\?v=/.test(html)) {
    fail(file, "uses i18n keys without loading assets/i18n.js");
  }

  const ids = [...html.matchAll(/\bid="([^"]+)"/g)].map((match) => match[1]);
  const duplicateIds = [...new Set(ids.filter((id, index) => ids.indexOf(id) !== index))];
  if (duplicateIds.length) fail(file, `duplicate ids: ${duplicateIds.join(", ")}`);

  const hreflangs = [...html.matchAll(/hreflang="([^"]+)"/g)].map((match) => match[1]);
  const duplicateHreflangs = [...new Set(hreflangs.filter((lang, index) => hreflangs.indexOf(lang) !== index))];
  if (duplicateHreflangs.length) fail(file, `duplicate hreflang values: ${duplicateHreflangs.join(", ")}`);

  if (!appOnly.has(file)) {
    if (!/<footer\b/i.test(html)) fail(file, "public page is missing a semantic footer");
    if (!/<nav\b|class="dbar"/.test(html)) fail(file, "public page is missing navigation");
  }

  const nav = html.match(/<nav class="nav"[^>]*>([\s\S]*?)<\/nav>/);
  if (nav) {
    for (const route of requiredNavRoutes) {
      if (!new RegExp(`href="/?${route.replace(".", "\\.")}`).test(nav[1])) fail(file, `navigation is missing ${route}`);
    }
    const navComputeLinks = [...nav[1].matchAll(/data-i18n="nav\.compute"/g)].length;
    if (navComputeLinks > 1) fail(file, `navigation contains ${navComputeLinks} Compute links`);
  }

  for (const match of html.matchAll(/(?:href|src)="([^"]+)"/g)) {
    const ref = match[1].split(/[?#]/)[0];
    if (!ref || /^(?:https?:|mailto:|tel:|data:)/i.test(ref) || legacyRoutes.has(ref)) continue;
    if (ref.startsWith("/") && !/\.[a-z0-9]+$/i.test(ref)) continue;
    const local = ref.startsWith("/") ? ref.slice(1) : ref;
    if (!fs.existsSync(path.join(root, local))) fail(file, `missing local target: ${match[1]}`);
  }
}

const terms = fs.readFileSync(path.join(root, "terms.html"), "utf8");
const termNumbers = [...terms.matchAll(/<h2>(\d+)\./g)].map((match) => Number(match[1]));
const expectedTermNumbers = Array.from({ length: 19 }, (_, index) => index + 1);
if (JSON.stringify(termNumbers) !== JSON.stringify(expectedTermNumbers)) {
  fail("terms.html", `section sequence is ${termNumbers.join(", ")}; expected 1–19 exactly once`);
}

const sharedCss = fs.readFileSync(path.join(root, "fk2.css"), "utf8");
if (/\.megafoot\.slim\s*\{[^}]*\bposition\s*:\s*fixed\b/i.test(sharedCss)) {
  fail("fk2.css", "slim footer must stay in document flow instead of covering page content");
}
if (!/\.megafoot\.slim\s+\.word\s*\{[^}]*\bdisplay\s*:\s*none\b/i.test(sharedCss)) {
  fail("fk2.css", "slim footer must not render the oversized wordmark");
}

const i18nSource = fs.readFileSync(path.join(root, "assets/i18n.js"), "utf8");
const dictMatch = i18nSource.match(/var DICTS = (\{[\s\S]*?\n\});\n\n  var LEGAL_ROUTES/);
if (!dictMatch) {
  fail("assets/i18n.js", "cannot parse dictionaries");
} else {
  const dicts = vm.runInNewContext(`(${dictMatch[1]})`);
  const referenced = new Set();
  for (const file of files) {
    const html = fs.readFileSync(path.join(root, file), "utf8");
    for (const match of html.matchAll(/data-i18n(?:-ph)?="([^"]+)"/g)) referenced.add(match[1]);
  }
  for (const [locale, dictionary] of Object.entries(dicts)) {
    const missing = [...referenced].filter((key) => !(key in dictionary));
    if (missing.length) fail("assets/i18n.js", `${locale} is missing ${missing.length} referenced keys: ${missing.join(", ")}`);
  }
}

if (warnings.length) console.warn(`Warnings (${warnings.length})\n${warnings.join("\n")}\n`);
if (errors.length) {
  console.error(`Static website audit failed (${errors.length})\n${errors.join("\n")}`);
  process.exit(1);
}
console.log(`Static website audit passed: ${files.length} HTML files, responsive metadata, navigation, footer, links, hreflang and i18n coverage.`);
