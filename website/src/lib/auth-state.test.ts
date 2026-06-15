import { describe, expect, test } from "bun:test";
import { getWebsiteSessionState } from "./auth-state";

describe("getWebsiteSessionState", () => {
  test("treats successful self response as authenticated", async () => {
    const requests: Array<{ input: string; init?: RequestInit }> = [];
    const fetcher: typeof fetch = async (input, init) => {
      requests.push({ input: String(input), init });
      return Response.json({ success: true, data: { id: 1, username: "demo" } });
    };

    const state = await getWebsiteSessionState("https://router.flatkey.ai", fetcher);

    expect(state.authenticated).toBe(true);
    expect(requests[0].input).toBe("https://router.flatkey.ai/api/user/self");
    expect(requests[0].init?.credentials).toBe("include");
    expect(requests[0].init?.cache).toBe("no-store");
  });

  test("treats failed or missing self data as anonymous", async () => {
    const state = await getWebsiteSessionState("https://router.flatkey.ai", async () =>
      Response.json({ success: false })
    );

    expect(state.authenticated).toBe(false);
  });
});
