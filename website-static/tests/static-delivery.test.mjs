import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

function read(relativePath) {
  return readFileSync(new URL(relativePath, import.meta.url), "utf8");
}

test("Nginx proxies the public status response with bounded browser caching", () => {
  const nginx = read("../nginx.conf");

  assert.match(nginx, /location = \/api\/status\s*\{/);
  assert.match(
    nginx,
    /proxy_pass \$\{APP_CONSOLE_ORIGIN\}\/api\/status;/,
  );
  assert.match(nginx, /proxy_set_header Cookie "";/);
  assert.match(nginx, /proxy_set_header Authorization "";/);

  const timeoutNames = [
    ...nginx.matchAll(/proxy_(connect|read|send)_timeout 3s;/g),
  ].map((match) => match[1]);
  assert.deepEqual(timeoutNames.sort(), ["connect", "read", "send"]);
  assert.match(nginx, /Cache-Control "public, max-age=60" always;/);
});

test("static HTML receives one shared configuration script and keeps local docs fallback visible", () => {
  const nginx = read("../nginx.conf");
  const css = read("../html/fk2.css");
  const indexHtml = read("../html/index.html");

  assert.match(
    nginx,
    /sub_filter '<\/body>' '<script src="\/assets\/site-config\.js\?v=[^"]+"><\/script><\/body>';/,
  );
  assert.doesNotMatch(nginx, /sub_filter 'fk2\.css\?v=716b'/);
  assert.match(indexHtml, /<a href="\/docs">Docs<\/a>/);
  assert.doesNotMatch(indexHtml, /href="[^"]*\.html/);
});

test("public static pages use one extensionless canonical route", () => {
  const nginx = read("../nginx.conf");
  const sitemap = read("../html/sitemap-v2.xml");

  for (const [route, file] of [
    ["models", "models.html"],
    ["docs", "docs.html"],
    ["playground", "playground.html"],
    ["pricing", "topup.html"],
    ["terms", "terms.html"],
  ]) {
    assert.match(nginx, new RegExp(`location = /${route} \\{ try_files /${file.replace(".", "\\.")} =404; \\}`));
  }
  assert.match(nginx, /location = \/topup\.html \{ return 301 \/pricing; \}/);
  assert.doesNotMatch(sitemap, /\.html</);
  assert.doesNotMatch(sitemap, /<loc>https:\/\/flatkey\.ai\/login<\/loc>/);
});

test("legacy HTML and sitemap responses normalize regional language tags", () => {
  const nginx = read("../nginx.conf");

  for (const [short, regional] of [["en", "en-US"], ["zh", "zh-CN"], ["ja", "ja-JP"]]) {
    assert.match(nginx, new RegExp(`sub_filter 'lang="${short}"' 'lang="${regional}"';`));
  }
  assert.match(nginx, /sub_filter_types application\/xml text\/xml;/);
  assert.match(nginx, /sub_filter_once off;/);
});

test("the Nginx image substitutes only the configured console origin", () => {
  const dockerfile = read("../Dockerfile");

  assert.match(
    dockerfile,
    /ENV APP_CONSOLE_ORIGIN=https:\/\/console\.flatkey\.ai/,
  );
  assert.match(
    dockerfile,
    /ENV NGINX_ENVSUBST_FILTER=APP_CONSOLE_ORIGIN/,
  );
  assert.match(
    dockerfile,
    /COPY nginx\.conf \/etc\/nginx\/templates\/default\.conf\.template/,
  );
  assert.doesNotMatch(
    dockerfile,
    /COPY nginx\.conf \/etc\/nginx\/conf\.d\/default\.conf/,
  );
});

test("the production workflow passes and smoke-tests the console origin", () => {
  const workflow = read("../../.github/workflows/gcp-deploy-website-static.yml");

  assert.match(
    workflow,
    /--update-env-vars[=\s"']+APP_CONSOLE_ORIGIN=\$\{APP_CONSOLE_ORIGIN\}/,
  );
  assert.match(workflow, /"\$C\/api\/status"/);
  assert.ok(
    workflow.includes(
      `grep -Eq '"success"[[:space:]]*:[[:space:]]*true'`,
    ),
  );
});
