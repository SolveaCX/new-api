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

  const timeoutNames = [
    ...nginx.matchAll(/proxy_(connect|read|send)_timeout 3s;/g),
  ].map((match) => match[1]);
  assert.deepEqual(timeoutNames.sort(), ["connect", "read", "send"]);
  assert.match(nginx, /Cache-Control "public, max-age=60" always;/);
});

test("static HTML receives one shared configuration script and hidden pending links", () => {
  const nginx = read("../nginx.conf");
  const css = read("../html/fk2.css");

  assert.match(
    nginx,
    /sub_filter '<\/body>' '<script src="\/assets\/site-config\.js\?v=[^"]+"><\/script><\/body>';/,
  );
  for (const selector of [
    '.nav a[href="docs.html"]',
    '.nav a[href="/docs.html"]',
    '.megafoot .col a[href="docs.html"]',
    '.megafoot .col a[href="/docs.html"]',
  ]) {
    assert.ok(css.includes(selector), `missing hidden selector: ${selector}`);
  }
  assert.match(css, /display\s*:\s*none/);
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
