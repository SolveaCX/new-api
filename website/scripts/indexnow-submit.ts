const DEFAULT_SITE_ORIGIN = "https://flatkey.ai";
const INDEXNOW_ENDPOINT = "https://api.indexnow.org/IndexNow";
const INDEXNOW_KEY_FILE = "acc80000-1bb5-4504-8426-b6bac35d273a.txt";
const MAX_URLS_PER_REQUEST = 10_000;

type Options = {
  siteOrigin: string;
  key: string;
  keyLocation: string;
  urls: string[];
  sitemapUrl?: string;
};

function normalizeOrigin(value: string): string {
  return value.trim().replace(/\/+$/, "");
}

function readOption(args: string[], name: string): string | undefined {
  const index = args.findIndex((arg) => arg === name || arg.startsWith(`${name}=`));
  if (index < 0) return undefined;
  const arg = args[index];
  if (arg.startsWith(`${name}=`)) return arg.slice(name.length + 1);
  return args[index + 1];
}

function readRepeatedOption(args: string[], name: string): string[] {
  const values: string[] = [];
  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];
    if (arg === name && args[index + 1]) {
      values.push(args[index + 1]);
      index += 1;
    } else if (arg.startsWith(`${name}=`)) {
      values.push(arg.slice(name.length + 1));
    }
  }
  return values;
}

function parseOptions(): Options {
  const args = process.argv.slice(2);
  if (args.includes("--help") || args.includes("-h")) {
    console.log(`Usage: bun scripts/indexnow-submit.ts [options]

Options:
  --url <url>          Submit one URL; may be repeated
  --sitemap <url>      Read URLs from a sitemap and submit them in batches
  --site-origin <url>  Site origin (default: SITE_ORIGIN or https://flatkey.ai)
  --key <key>          IndexNow key (default: INDEXNOW_KEY or the hosted key)
`);
    process.exit(0);
  }

  const siteOrigin = normalizeOrigin(
    readOption(args, "--site-origin") ?? process.env.SITE_ORIGIN ?? DEFAULT_SITE_ORIGIN,
  );
  const key = readOption(args, "--key") ?? process.env.INDEXNOW_KEY ?? INDEXNOW_KEY_FILE.replace(/\.txt$/, "");
  const keyLocation = `${siteOrigin}/${INDEXNOW_KEY_FILE}`;
  const directUrls = readRepeatedOption(args, "--url");
  const sitemapUrl = readOption(args, "--sitemap");

  if (!directUrls.length && !sitemapUrl) {
    throw new Error("Provide at least one --url or a --sitemap URL.");
  }

  return { siteOrigin, key, keyLocation, urls: directUrls, sitemapUrl };
}

async function readSitemapUrls(sitemapUrl: string): Promise<string[]> {
  const response = await fetch(sitemapUrl, { headers: { accept: "application/xml,text/xml" } });
  if (!response.ok) {
    throw new Error(`Sitemap request failed with HTTP ${response.status}: ${sitemapUrl}`);
  }
  const xml = await response.text();
  return [...xml.matchAll(/<loc>\s*([^<]+?)\s*<\/loc>/gi)].map((match) => match[1]);
}

function uniqueUrls(urls: string[]): string[] {
  return [...new Set(urls.map((url) => url.trim()).filter(Boolean))];
}

async function submitBatch(options: Options, urls: string[]): Promise<void> {
  const host = new URL(options.siteOrigin).hostname;
  const response = await fetch(INDEXNOW_ENDPOINT, {
    method: "POST",
    headers: { "Content-Type": "application/json; charset=utf-8" },
    body: JSON.stringify({
      host,
      key: options.key,
      keyLocation: options.keyLocation,
      urlList: urls,
    }),
  });
  if (!response.ok) {
    const body = await response.text().catch(() => "");
    throw new Error(`IndexNow request failed with HTTP ${response.status}: ${body}`);
  }
  console.log(`IndexNow accepted ${urls.length} URL(s) (HTTP ${response.status}).`);
}

async function main(): Promise<void> {
  const options = parseOptions();
  const sitemapUrls = options.sitemapUrl ? await readSitemapUrls(options.sitemapUrl) : [];
  const urls = uniqueUrls([...options.urls, ...sitemapUrls]);
  if (!urls.length) throw new Error("No URLs found to submit.");

  for (let index = 0; index < urls.length; index += MAX_URLS_PER_REQUEST) {
    await submitBatch(options, urls.slice(index, index + MAX_URLS_PER_REQUEST));
  }
  console.log(`Submitted ${urls.length} unique URL(s) to IndexNow.`);
}

main().catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : error);
  process.exitCode = 1;
});
