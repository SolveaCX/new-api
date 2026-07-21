import { describe, expect, test } from "bun:test";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import type { StatusComponent, StatusIncident, StatusSummary, StatusValue } from "@/lib/status";
import { filterStatusComponents, getStatusPresentation, StatusPage } from "./status-page";

const NOW = 1_800_000_000;

function component(overrides: Partial<StatusComponent> & Pick<StatusComponent, "id" | "slug" | "display_name">): StatusComponent {
  return {
    kind: "model",
    lifecycle: "active",
    status: "operational",
    capability: "text",
    last_trustworthy_update_at: NOW,
    coverage: 965_000,
    ...overrides,
  };
}

const COMPONENTS: StatusComponent[] = [
  component({ id: 1, slug: "router", display_name: "Router", kind: "router" }),
  component({ id: 2, slug: "gpt-5", display_name: "GPT-5", capability: "text", status: "degraded" }),
  component({ id: 3, slug: "claude-sonnet", display_name: "Claude Sonnet", capability: "text", status: "outage" }),
  component({ id: 4, slug: "imagen", display_name: "Imagen", capability: "image", status: "maintenance" }),
  component({ id: 5, slug: "legacy-chat", display_name: "Legacy Chat", lifecycle: "retired", status: "unknown" }),
];

const SUMMARY: StatusSummary = {
  generated_at: NOW,
  last_trustworthy_update_at: NOW,
  coverage: 965_000,
  status: "all_systems_operational",
  components: COMPONENTS,
};

const INCIDENT: StatusIncident = {
  id: "inc-router",
  kind: "incident",
  title: "Router packet loss",
  impact: "minor",
  status: "resolved",
  started_at: NOW - 3_600,
  resolved_at: NOW - 1_800,
  updated_at: NOW - 1_800,
  component_ids: [1],
  updates: [{ state: "resolved", body: "Traffic returned to normal.", published_at: NOW - 1_800 }],
};

const MAINTENANCE: StatusIncident = {
  id: "maint-imagen",
  kind: "maintenance",
  title: "Imagen capacity maintenance",
  impact: "maintenance",
  status: "scheduled",
  scheduled_start_at: NOW + 3_600,
  scheduled_end_at: NOW + 7_200,
  updated_at: NOW,
  component_ids: [4],
  updates: [{ state: "scheduled", body: "Capacity work is scheduled.", published_at: NOW }],
};

describe("StatusPage", () => {
  test("pins Router first and renders every public model with semantic filters", () => {
    const html = renderToStaticMarkup(
      <StatusPage locale="en" summary={SUMMARY} freshness="fresh" incidents={[INCIDENT]} maintenance={[MAINTENANCE]} />
    );

    expect(html.indexOf("Router")).toBeLessThan(html.indexOf("GPT-5"));
    for (const item of COMPONENTS) expect(html).toContain(item.display_name);
    expect(html).toContain('name="query"');
    expect(html).toContain('name="capability"');
    expect(html).toContain('name="status"');
    expect(html).toContain('href="/status/models/gpt-5"');
  });

  test("filters components independently by name, capability, and status", () => {
    expect(filterStatusComponents(COMPONENTS, { query: "sonnet" }).map((item) => item.slug)).toEqual(["claude-sonnet"]);
    expect(filterStatusComponents(COMPONENTS, { capability: "image" }).map((item) => item.slug)).toEqual(["imagen"]);
    expect(filterStatusComponents(COMPONENTS, { status: "outage" }).map((item) => item.slug)).toEqual(["claude-sonnet"]);
  });

  test("represents every state with text, an icon, and color while failing stale or retired evidence closed", () => {
    const states: StatusValue[] = ["operational", "degraded", "outage", "unknown", "maintenance"];
    for (const status of states) {
      const presentation = getStatusPresentation({ locale: "en", status, freshness: "fresh", lifecycle: "active" });
      expect(presentation.status).toBe(status);
      expect(presentation.text).toBeTruthy();
      expect(presentation.icon).toBeTruthy();
      expect(presentation.colorClass).toMatch(/(?:text|bg|border)-/);
    }

    const stale = getStatusPresentation({ locale: "en", status: "operational", freshness: "stale", lifecycle: "active" });
    const retired = getStatusPresentation({ locale: "en", status: "operational", freshness: "fresh", lifecycle: "retired" });
    const maintenance = getStatusPresentation({ locale: "en", status: "maintenance", freshness: "fresh", lifecycle: "active" });
    expect(stale.status).toBe("unknown");
    expect(stale.text.toLowerCase()).toContain("stale");
    expect(retired.status).toBe("unknown");
    expect(retired.text.toLowerCase()).toContain("retired");
    expect(maintenance.status).toBe("maintenance");
    expect(stale.colorClass).not.toBe(getStatusPresentation({ locale: "en", status: "operational", freshness: "fresh", lifecycle: "active" }).colorClass);
  });

  test("shows freshness, coverage, incidents, maintenance, and an accessible subscription form", () => {
    const html = renderToStaticMarkup(
      <StatusPage locale="en" summary={SUMMARY} freshness="fresh" incidents={[INCIDENT]} maintenance={[MAINTENANCE]} />
    );

    expect(html).toContain("96.50%");
    expect(html).toContain("Router packet loss");
    expect(html).toContain("Imagen capacity maintenance");
    expect(html).toMatch(/Last (?:trustworthy )?update/i);
    expect(html).toMatch(/<form[\s>]/);
    expect(html).toMatch(/<label[^>]+for="status-subscription-email"/);
    expect(html).toMatch(/<input[^>]+id="status-subscription-email"[^>]+type="email"[^>]+required/);
    expect(html).toMatch(/<fieldset[\s>][\s\S]*<legend[\s>]/);
    expect(html).toMatch(/<button[^>]+type="submit"/);
  });

  test("does not describe unavailable incident or maintenance feeds as empty", () => {
    const html = renderToStaticMarkup(
      <StatusPage
        locale="en"
        summary={SUMMARY}
        freshness="fresh"
        incidents={[]}
        incidentFreshness="monitoring-unavailable"
        maintenance={[]}
        maintenanceFreshness="stale"
      />
    );

    expect(html).toContain("Monitoring unavailable");
    expect(html).toContain("Stale monitoring data");
    expect(html).not.toContain("No recent incidents");
    expect(html).not.toContain("No scheduled maintenance");
  });
});

function pageFiles(root: string, current = root): string[] {
  if (!existsSync(current)) return [];
  return readdirSync(current, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(current, entry.name);
    if (entry.isDirectory()) return pageFiles(root, path);
    return entry.name === "page.tsx" ? [path.slice(root.length + 1).replaceAll("\\", "/")] : [];
  }).sort();
}

describe("status routes", () => {
  test("keeps English root and localized status routes symmetric and discoverable", () => {
    const appRoot = resolve(import.meta.dir, "../app");
    const english = pageFiles(resolve(appRoot, "(en)/status"));
    const localized = pageFiles(resolve(appRoot, "[locale]/status"));
    expect(english).toEqual(["models/[slug]/page.tsx", "page.tsx"]);
    expect(localized).toEqual(english);

    const sitemapSource = readFileSync(resolve(appRoot, "sitemap.ts"), "utf8");
    expect(sitemapSource).toMatch(/entry\(["']\/status["']/);

    for (const route of [
      resolve(appRoot, "(en)/status/models/[slug]/page.tsx"),
      resolve(appRoot, "[locale]/status/models/[slug]/page.tsx"),
    ]) {
      const source = readFileSync(route, "utf8");
      expect(source).toMatch(/history\.state\s*===\s*["']not-found["']/);
      expect(source).toMatch(/notFound\(\)/);
    }
  });
});
