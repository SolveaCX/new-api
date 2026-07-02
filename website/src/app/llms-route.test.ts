import { describe, expect, test } from "bun:test";
import { GET } from "./llms.txt/route";
import { SITE_ORIGIN } from "@/lib/origins";

describe("llms.txt", () => {
  test("uses the resolved site origin for public URLs", async () => {
    const response = await GET();
    const body = await response.text();

    expect(body).toContain(`- Home: ${SITE_ORIGIN}/`);
    expect(body).toContain(`- Rankings: ${SITE_ORIGIN}/rankings`);
    expect(body).toContain(`- Sitemap: ${SITE_ORIGIN}/sitemap.xml`);
  });
});
