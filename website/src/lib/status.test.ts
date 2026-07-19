import { describe, expect, test } from "bun:test";
import {
  STATUS_OVERALL_VALUES,
  fetchStatusComponent,
  fetchStatusComponentHistory,
  fetchStatusComponents,
  fetchStatusIncident,
  fetchStatusIncidents,
  fetchStatusMaintenance,
  fetchStatusSummary,
  previewStatusUnsubscribe,
  subscribeToStatus,
  unsubscribeFromStatus,
  verifyStatusSubscription,
  type StatusOverallValue,
} from "./status";

const now = () => Math.floor(Date.now() / 1000);

describe("status client", () => {
  test("matches the Go service overall status values exactly", () => {
    const expected = [
      "major_outage",
      "some_systems_affected",
      "degraded_performance",
      "monitoring_incomplete",
      "maintenance",
      "all_systems_operational",
    ] as const satisfies readonly StatusOverallValue[];

    expect(STATUS_OVERALL_VALUES).toEqual(expected);
  });

  test("fetches typed summary data with exactly 60-second revalidation", async () => {
    const originalFetch = globalThis.fetch;
    let requestedUrl = "";
    let requestedInit: RequestInit | undefined;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      requestedUrl = String(input);
      requestedInit = init;
      return Promise.resolve(
        Response.json({
          success: true,
          message: "",
          data: {
            generated_at: now(),
            last_trustworthy_update_at: now(),
            coverage: 900000,
            status: "monitoring_incomplete",
            message: "monitoring unavailable",
            components: [],
          },
        })
      );
    }) as typeof fetch;

    try {
      const result = await fetchStatusSummary();
      expect(requestedUrl).toBe("/api/status/summary");
      expect((requestedInit as RequestInit & { next?: { revalidate?: number } })?.next?.revalidate).toBe(60);
      expect(result.state).toBe("fresh");
      expect(result.data?.status).toBe("monitoring_incomplete");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("marks old unknown data stale without fabricating operational status", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() =>
      Promise.resolve(
        Response.json({
          success: true,
          data: {
            generated_at: 1,
            last_trustworthy_update_at: 0,
            coverage: 0,
            status: "all_systems_operational",
            components: [{ id: 1, slug: "router", kind: "router", display_name: "Router", lifecycle: "active", status: "unknown", last_trustworthy_update_at: 0, coverage: 0 }],
          },
        })
      )) as typeof fetch;

    try {
      const result = await fetchStatusSummary();
      expect(result.state).toBe("stale");
      expect(result.data?.status).toBe("monitoring_incomplete");
      expect(result.data?.components[0]?.status).toBe("unknown");
      expect(JSON.stringify(result)).not.toContain("operational");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("returns explicit monitoring-unavailable state without leaking failures", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => Promise.reject(new Error("password=hunter2"))) as typeof fetch;

    try {
      const result = await fetchStatusSummary();
      expect(result.state).toBe("monitoring-unavailable");
      expect(result.data.status).toBe("monitoring_incomplete");
      expect(result.data.components).toEqual([]);
      expect(JSON.stringify(result)).not.toContain("hunter2");
      expect(JSON.stringify(result)).not.toContain("operational");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("rejects component-only values as invalid summary overall status", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => Promise.resolve(Response.json({
      success: true,
      data: {
        generated_at: now(),
        last_trustworthy_update_at: now(),
        coverage: 1000000,
        status: "operational",
        components: [],
      },
    }))) as typeof fetch;

    try {
      const result = await fetchStatusSummary();
      expect(result.state).toBe("monitoring-unavailable");
      expect(result.data.status).toBe("monitoring_incomplete");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("builds all public data endpoint paths with encoded identifiers and query values", async () => {
    const originalFetch = globalThis.fetch;
    const requested: string[] = [];
    globalThis.fetch = ((input: RequestInfo | URL) => {
      requested.push(String(input));
      return Promise.resolve(Response.json({ success: true, data: { generated_at: now() } }));
    }) as typeof fetch;

    try {
      await fetchStatusComponents({ kind: "model", query: "gpt 5", capability: "text", status: "degraded" });
      await fetchStatusComponent("gpt 5");
      await fetchStatusComponentHistory("gpt 5", "7d");
      await fetchStatusIncidents();
      await fetchStatusIncident("inc public");
      await fetchStatusMaintenance();

      expect(requested).toEqual([
        "/api/status/components?kind=model&query=gpt+5&capability=text&status=degraded",
        "/api/status/components/gpt%205",
        "/api/status/components/gpt%205/history?range=7d",
        "/api/status/incidents",
        "/api/status/incidents/inc%20public",
        "/api/status/maintenance",
      ]);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("types subscription operations and keeps POST mutations uncached", async () => {
    const originalFetch = globalThis.fetch;
    const calls: Array<{ url: string; init?: RequestInit }> = [];
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ url: String(input), init });
      const data = String(input).includes("unsubscribe?")
        ? { message: "generic", can_unsubscribe: true }
        : { message: "generic" };
      return Promise.resolve(Response.json({ success: true, data }));
    }) as typeof fetch;

    try {
      expect((await subscribeToStatus({ email: "reader@example.com", component_ids: [1] })).state).toBe("fresh");
      expect((await verifyStatusSubscription("verify token")).state).toBe("fresh");
      expect((await previewStatusUnsubscribe("manage token")).data?.can_unsubscribe).toBe(true);
      expect((await unsubscribeFromStatus({ token: "manage token" })).state).toBe("fresh");

      expect(calls.map((call) => call.url)).toEqual([
        "/api/status/subscriptions",
        "/api/status/subscriptions/verify?token=verify+token",
        "/api/status/subscriptions/unsubscribe?token=manage+token",
        "/api/status/subscriptions/unsubscribe",
      ]);
      expect(calls[0]?.init?.cache).toBe("no-store");
      expect((calls[0]?.init as RequestInit & { next?: unknown })?.next).toBeUndefined();
      expect((calls[1]?.init as RequestInit & { next?: { revalidate?: number } })?.next?.revalidate).toBe(60);
      expect(calls[3]?.init?.cache).toBe("no-store");
    } finally {
      globalThis.fetch = originalFetch;
    }
  });
});
