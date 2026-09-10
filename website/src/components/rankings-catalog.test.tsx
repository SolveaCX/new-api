import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { RankingsPage } from "./rankings-page";
import { WEBSITE_PUBLIC_PRICING_GROUP } from "@/lib/pricing";

test("rankings uses the detail route public catalog without dropping historical usage rows", async () => {
  const originalFetch = globalThis.fetch;
  const requestedGroups: Array<string | null> = [];
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    if (url.pathname === "/api/rankings") return Response.json({success: true, data: {models: [
      {model_name: "public-model", rank: 1, total_tokens: 100},
      {model_name: "historical-model", rank: 2, total_tokens: 50},
    ]}});
    if (url.pathname === "/api/website/pricing") {
      requestedGroups.push(url.searchParams.get("group"));
      const names = url.searchParams.get("group") === WEBSITE_PUBLIC_PRICING_GROUP ? ["public-model"] : ["public-model", "historical-model"];
      return Response.json({success: true, data: names.map(model_name => ({model_name})), vendors: []});
    }
    throw new Error(`Unexpected fetch: ${url.pathname}`);
  }) as typeof fetch;
  try {
    for (const locale of ["en", "zh"] as const) {
      const page = await RankingsPage({locale, pathname: "/rankings"});
      const html = renderToStaticMarkup(<>{page.props.children}</>);
      expect(html).toContain(`href="${locale === "en" ? "" : "/zh"}/models/public-model"`);
      expect(html).not.toContain('/models/historical-model');
      expect(html).toContain('>historical-model</span>');
      expect(html).toContain('>public-model</a>');
    }
    expect(requestedGroups).toEqual(["plg", "plg"]);
  } finally {
    globalThis.fetch = originalFetch;
  }
});
