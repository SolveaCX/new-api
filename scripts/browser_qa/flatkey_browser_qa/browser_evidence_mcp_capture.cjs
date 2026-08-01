const { chromium } = require("playwright-core");
const helper = require("./browser_evidence_helper.cjs");

async function main() {
  const [, , runtimeDir, cdpEndpoint, logicalName] = process.argv;
  const browser = await chromium.connectOverCDP(cdpEndpoint);
  try {
    const context = browser.contexts()[0] || await browser.newContext();
    const page = context.pages()[0] || await context.newPage();
    const result = await helper.captureScreenshot(page, runtimeDir, logicalName, []);
    process.stdout.write(JSON.stringify(result));
  } finally {
    await browser.close();
  }
}

main().catch(() => process.exit(1));
