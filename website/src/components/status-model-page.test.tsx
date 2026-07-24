import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import type { StatusComponentHistoryData, StatusIncident, StatusPeriod } from "@/lib/status";
import { normalizeDailyHistory } from "./status-history-bars";
import { StatusModelPage } from "./status-model-page";

const DAY = 86_400;
const END_AT = 1_800_057_600;

function period(dayOffset: number, status: StatusPeriod["status"] = "operational"): StatusPeriod {
  return {
    period_start: END_AT - dayOffset * DAY,
    availability: status === "outage" ? 0 : 1_000_000,
    coverage: 1_000_000,
    status,
  };
}

const HISTORY: StatusComponentHistoryData = {
  generated_at: END_AT,
  last_trustworthy_update_at: END_AT,
  coverage: 875_000,
  range: "90d",
  component: {
    id: 8,
    slug: "legacy-chat",
    kind: "model",
    display_name: "Legacy Chat",
    capability: "text",
    lifecycle: "retired",
    status: "unknown",
    last_trustworthy_update_at: END_AT,
    coverage: 875_000,
  },
  availability: {
    availability_micros: 987_654,
    coverage_micros: 875_000,
    known_bucket_count: 87,
    unknown_bucket_count: 3,
    maintenance_bucket_count: 1,
  },
  periods: Array.from({ length: 90 }, (_, index) => period(89 - index)),
};

const INCIDENT: StatusIncident = {
  id: "inc-legacy",
  kind: "incident",
  title: "Legacy Chat interruption",
  impact: "major",
  status: "resolved",
  started_at: END_AT - DAY,
  resolved_at: END_AT - DAY + 1_800,
  updated_at: END_AT - DAY + 1_800,
  component_ids: [8],
  updates: [
    { state: "investigating", body: "We are investigating failed requests.", published_at: END_AT - DAY },
    { state: "resolved", body: "Requests are succeeding again.", published_at: END_AT - DAY + 1_800 },
  ],
};

describe("status history", () => {
  test("normalizes a 90-day calendar, marks missing periods unknown, and keeps each day's worst state", () => {
    const normalized = normalizeDailyHistory(
      [period(0, "operational"), period(0, "outage"), period(2, "maintenance")],
      { days: 90, endAt: END_AT }
    );

    expect(normalized).toHaveLength(90);
    expect(normalized.at(-1)?.status).toBe("outage");
    expect(normalized.at(-2)?.status).toBe("unknown");
    expect(normalized.at(-2)?.coverage).toBe(0);
    expect(normalized.at(-3)?.status).toBe("maintenance");
  });
});

describe("StatusModelPage", () => {
  test("renders 90 textual, tooltipped daily bars and selectable history ranges", () => {
    const html = renderToStaticMarkup(
      <StatusModelPage locale="en" history={HISTORY} freshness="fresh" incidents={[INCIDENT]} selectedRange="90d" />
    );

    expect(html.match(/data-status-day=/g)).toHaveLength(90);
    expect(html.match(/data-status-day=[^>]+(?:aria-label|title)=/g)).toHaveLength(90);
    for (const range of ["24h", "7d", "30d", "90d"]) {
      expect(html).toContain(`href="/status/models/legacy-chat?range=${range}"`);
    }
    expect(html).toContain('aria-current="page"');
  });

  test("shows trustworthy evidence, availability, coverage, incident count, timeline, and retired state", () => {
    const html = renderToStaticMarkup(
      <StatusModelPage locale="en" history={HISTORY} freshness="fresh" incidents={[INCIDENT]} selectedRange="90d" />
    );

    expect(html).toContain("Legacy Chat");
    expect(html).toMatch(/Retired/i);
    expect(html).toMatch(/Unknown/i);
    expect(html).toMatch(/Last (?:trustworthy )?update/i);
    expect(html).toContain("98.77%");
    expect(html).toContain("87.50%");
    expect(html).toMatch(/(?:Incident count|Incidents)[\s\S]*1/i);
    expect(html).toContain("Legacy Chat interruption");
    expect(html).toContain("We are investigating failed requests.");
    expect(html).toContain("Requests are succeeding again.");
  });

  test("localizes model history navigation without changing the model slug", () => {
    const html = renderToStaticMarkup(
      <StatusModelPage locale="de" history={HISTORY} freshness="fresh" incidents={[]} selectedRange="7d" />
    );
    expect(html).toContain('href="/de/status/models/legacy-chat?range=24h"');
    expect(html).toContain('href="/de/status/models/legacy-chat?range=90d"');
  });

  test("never renders retired operational evidence as a green current status", () => {
    const history = {
      ...HISTORY,
      component: { ...HISTORY.component, status: "operational" as const },
    };
    const html = renderToStaticMarkup(
      <StatusModelPage locale="en" history={history} freshness="fresh" incidents={[]} selectedRange="90d" />
    );

    expect(html).toMatch(/Retired/i);
    expect(html).toMatch(/Unknown/i);
    expect(html).not.toContain("text-emerald-800");
  });

  test("does not describe an unavailable incident feed as having no incidents", () => {
    const html = renderToStaticMarkup(
      <StatusModelPage
        locale="en"
        history={HISTORY}
        freshness="fresh"
        incidents={[]}
        incidentFreshness="monitoring-unavailable"
        selectedRange="90d"
      />
    );

    expect(html).toContain("Monitoring unavailable");
    expect(html).not.toContain("No recent incidents");
  });
});
