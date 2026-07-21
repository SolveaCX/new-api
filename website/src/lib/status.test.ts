import { describe, expect, test } from "bun:test";
import { SITE_ORIGIN } from "./origins";
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
const statusUrl = (path: string) => new URL(path, SITE_ORIGIN).toString();

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

  test("uses the configured trusted site origin for SSR fetches with 60-second revalidation", async () => {
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
      expect(requestedUrl).toBe(statusUrl("/api/status/summary"));
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

  test("preserves confirmed component 404s instead of turning them into monitoring failures", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => Promise.resolve(Response.json({ success: false }, { status: 404 }))) as typeof fetch;

    try {
      expect(await fetchStatusComponent("missing-model")).toEqual({ state: "not-found", data: null });
      expect(await fetchStatusComponentHistory("missing-model", "90d")).toEqual({ state: "not-found", data: null });
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

  test("fails closed for malformed or future-dated summary DTOs", async () => {
    const originalFetch = globalThis.fetch;
    const timestamp = now();
    const component = {
      id: 1,
      slug: "router",
      kind: "router",
      display_name: "Router",
      lifecycle: "active",
      status: "operational",
      last_trustworthy_update_at: timestamp,
      coverage: 1000000,
    };
    const valid = {
      generated_at: timestamp,
      last_trustworthy_update_at: timestamp,
      coverage: 1000000,
      status: "all_systems_operational",
      components: [component],
    };
    const invalid = [
      { status: "all_systems_operational" },
      { ...valid, coverage: "full" },
      { ...valid, components: [{ ...component, status: "green" }] },
      { ...valid, components: [{ ...component, last_trustworthy_update_at: "now" }] },
      { ...valid, generated_at: timestamp + 3600 },
      { ...valid, generated_at: null },
    ];
    globalThis.fetch = (() => Promise.resolve(Response.json({ success: true, data: invalid.shift() }))) as typeof fetch;

    try {
      for (let index = 0; index < 6; index += 1) {
        const result = await fetchStatusSummary();
        expect(result.state).toBe("monitoring-unavailable");
        expect(result.data.status).toBe("monitoring_incomplete");
      }
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("fails closed when trustworthy evidence timestamps are far in the future", async () => {
    const originalFetch = globalThis.fetch;
    const timestamp = now();
    const component = {
      id: 1,
      slug: "router",
      kind: "router",
      display_name: "Router",
      lifecycle: "active",
      status: "operational",
      last_trustworthy_update_at: timestamp,
      coverage: 1000000,
    };
    const valid = {
      generated_at: timestamp,
      last_trustworthy_update_at: timestamp,
      coverage: 1000000,
      status: "all_systems_operational",
      components: [component],
    };
    const invalid = [
      { ...valid, last_trustworthy_update_at: timestamp + 3600 },
      { ...valid, components: [{ ...component, last_trustworthy_update_at: timestamp + 3600 }] },
    ];
    globalThis.fetch = (() => Promise.resolve(Response.json({ success: true, data: invalid.shift() }))) as typeof fetch;

    try {
      const results = [await fetchStatusSummary(), await fetchStatusSummary()];
      expect(results.map((result) => [result.state, result.data.status])).toEqual([
        ["monitoring-unavailable", "monitoring_incomplete"],
        ["monitoring-unavailable", "monitoring_incomplete"],
      ]);
    } finally {
      globalThis.fetch = originalFetch;
    }
  });

  test("does not pass malformed endpoint DTOs through as typed data", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = (() => Promise.resolve(Response.json({ success: true, data: {} }))) as typeof fetch;
    const fetchers = [
      () => fetchStatusComponents(),
      () => fetchStatusComponent("router"),
      () => fetchStatusComponentHistory("router"),
      () => fetchStatusIncidents(),
      () => fetchStatusIncident("inc_public"),
      () => fetchStatusMaintenance(),
      () => subscribeToStatus({ email: "reader@example.com", component_ids: [1] }),
      () => previewStatusUnsubscribe("manage-token"),
    ];

    try {
      for (const fetcher of fetchers) {
        expect(await fetcher()).toEqual({ state: "monitoring-unavailable", data: null });
      }
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
        statusUrl("/api/status/components?kind=model&query=gpt+5&capability=text&status=degraded"),
        statusUrl("/api/status/components/gpt%205"),
        statusUrl("/api/status/components/gpt%205/history?range=7d"),
        statusUrl("/api/status/incidents"),
        statusUrl("/api/status/incidents/inc%20public"),
        statusUrl("/api/status/maintenance"),
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
        statusUrl("/api/status/subscriptions"),
        statusUrl("/api/status/subscriptions/verify?token=verify+token"),
        statusUrl("/api/status/subscriptions/unsubscribe?token=manage+token"),
        statusUrl("/api/status/subscriptions/unsubscribe"),
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
