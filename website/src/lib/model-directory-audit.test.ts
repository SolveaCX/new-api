import { describe, expect, test } from "bun:test";
import {
  auditModelDirectoryCatalog,
  renderModelDirectoryAuditMarkdown,
  type AuditIssue,
  type AuditIssueStatus,
  type AuditModelDirectoryMetadata,
  type AuditModelDirectoryRow,
} from "./model-directory-audit";

const COMPLETE_META: AuditModelDirectoryMetadata = {
  series: "GPT",
  vendor: "OpenAI",
  providers: ["OpenAI"],
  modalities: ["text", "image"],
  contextTokens: 128000,
  categories: ["Programming"],
  releasedAt: "2026-01-15",
  distillable: false,
};

const COMPLETE_ROW: AuditModelDirectoryRow = {
  name: "gpt-test",
  vendor: "OpenAI",
  billingUnit: "token",
  inputFilterUsd: 0.75,
  outputFilterUsd: 2.25,
};

function audit(overrides?: {
  rows?: AuditModelDirectoryRow[];
  metadata?: Record<string, AuditModelDirectoryMetadata>;
  generatedAt?: string;
}) {
  return auditModelDirectoryCatalog({
    generatedAt: overrides?.generatedAt ?? "2026-08-21T00:00:00.000Z",
    source: "fixture",
    rows: overrides?.rows ?? [COMPLETE_ROW],
    metadata: overrides?.metadata ?? { [COMPLETE_ROW.name]: COMPLETE_META },
  });
}

function issueKeys(issues: AuditIssue[]) {
  return issues.map((issue) => `${issue.status}:${issue.modelName}:${issue.field}`);
}

describe("model directory metadata audit", () => {
  test("complete catalogue reports no issues and records no production writes", () => {
    const report = audit();

    expect(report).toEqual({
      generatedAt: "2026-08-21T00:00:00.000Z",
      source: "fixture",
      modelCount: 1,
      metadataCount: 1,
      issues: [],
      wroteProduction: false,
    });
    expect(JSON.stringify(report, null, 2)).toContain('"wroteProduction": false');
  });

  test("missing required metadata fields are pending missing issues with affected filters", () => {
    const report = audit({
      metadata: {
        "gpt-test": {
          ...COMPLETE_META,
          vendor: "",
          providers: [],
          modalities: [],
          contextTokens: undefined,
          categories: [],
          releasedAt: undefined,
          distillable: undefined,
        },
      },
    });

    expect(issueKeys(report.issues)).toEqual([
      "missing:gpt-test:categories",
      "missing:gpt-test:contextTokens",
      "missing:gpt-test:distillable",
      "missing:gpt-test:modalities",
      "missing:gpt-test:providers",
      "missing:gpt-test:releasedAt",
      "missing:gpt-test:vendor",
    ]);
    expectEveryIssueIsPendingAndFilterAffected(report.issues);
  });

  test("invalid effective price and billing data are invalid issues", () => {
    const rows: AuditModelDirectoryRow[] = [
      { ...COMPLETE_ROW, name: "bad-billing", billingUnit: "seat" },
      { ...COMPLETE_ROW, name: "bad-input", inputFilterUsd: 0 },
      { ...COMPLETE_ROW, name: "bad-output", outputFilterUsd: Number.POSITIVE_INFINITY },
    ];
    const metadata = Object.fromEntries(rows.map((row) => [row.name, COMPLETE_META]));

    const report = audit({ rows, metadata });

    expect(issueKeys(report.issues)).toEqual([
      "invalid:bad-billing:billingUnit",
      "invalid:bad-input:inputFilterUsd",
      "invalid:bad-output:outputFilterUsd",
    ]);
    expectStatuses(report.issues, "invalid");
    expectEveryIssueIsPendingAndFilterAffected(report.issues);
  });

  test("live model with no exact metadata entry is an unknown-model issue", () => {
    const report = audit({ metadata: {} });

    expect(issueKeys(report.issues)).toEqual(["unknown-model:gpt-test:metadata"]);
    expectEveryIssueIsPendingAndFilterAffected(report.issues);
  });

  test("metadata entry with no live exact-name model is a stale-metadata issue", () => {
    const report = audit({
      metadata: {
        [COMPLETE_ROW.name]: COMPLETE_META,
        "retired-model": COMPLETE_META,
      },
    });

    expect(issueKeys(report.issues)).toEqual(["stale-metadata:retired-model:metadata"]);
    expectEveryIssueIsPendingAndFilterAffected(report.issues);
  });

  test("duplicate exact identities and alias collisions are deterministic collision issues", () => {
    const report = audit({
      rows: [
        { ...COMPLETE_ROW, name: "same-name", vendor: "OpenAI" },
        { ...COMPLETE_ROW, name: "same-name", vendor: "Azure" },
        { ...COMPLETE_ROW, name: "alias-target", vendor: "OpenAI" },
      ],
      metadata: {
        "same-name": { ...COMPLETE_META, aliases: ["alias-target"] },
        "alias-target": { ...COMPLETE_META, vendor: "Azure" },
      },
    });

    expect(report.issues.map((issue) => ({ status: issue.status, kind: issue.kind, modelName: issue.modelName, field: issue.field }))).toEqual([
      { status: "invalid", kind: "collision", modelName: "alias-target", field: "identity" },
      { status: "invalid", kind: "collision", modelName: "same-name", field: "identity" },
    ]);
    expectEveryIssueIsPendingAndFilterAffected(report.issues);
  });

  test("issue ordering is deterministic by status, model name, and field", () => {
    const first = audit({
      rows: [
        { ...COMPLETE_ROW, name: "z-live", inputFilterUsd: 0 },
        { ...COMPLETE_ROW, name: "a-live", billingUnit: "unknown" },
      ],
      metadata: {
        "z-live": COMPLETE_META,
        "a-live": COMPLETE_META,
        "m-stale": COMPLETE_META,
      },
    });
    const second = audit({
      rows: [
        { ...COMPLETE_ROW, name: "a-live", billingUnit: "unknown" },
        { ...COMPLETE_ROW, name: "z-live", inputFilterUsd: 0 },
      ],
      metadata: {
        "m-stale": COMPLETE_META,
        "a-live": COMPLETE_META,
        "z-live": COMPLETE_META,
      },
    });

    expect(issueKeys(first.issues)).toEqual([
      "invalid:a-live:billingUnit",
      "invalid:z-live:inputFilterUsd",
      "stale-metadata:m-stale:metadata",
    ]);
    expect(first.issues).toEqual(second.issues);
  });

  test("markdown contains read-only operator review warnings", () => {
    const markdown = renderModelDirectoryAuditMarkdown(audit({ metadata: {} }));

    expect(markdown).toContain("No production writes were performed");
    expect(markdown).toContain("Review these findings before any database update");
    expect(markdown).not.toContain("UPDATE ");
    expect(markdown).not.toContain("INSERT ");
  });

  test("suggestions are omitted unless trusted suggested value and source are supplied", () => {
    const report = audit({ metadata: {} });

    expect(report.issues[0]).not.toHaveProperty("suggestedValue");
    expect(report.issues[0]).not.toHaveProperty("source");
    expect(report.issues[0]).not.toHaveProperty("confidence");
    expect(report.issues[0]?.backfillSqlEligible).toBe(false);
  });
});

function expectEveryIssueIsPendingAndFilterAffected(issues: AuditIssue[]) {
  expect(issues.length).toBeGreaterThan(0);
  for (const issue of issues) {
    expect(issue.reviewStatus).toBe("pending");
    expect(issue.affectedFilters.length).toBeGreaterThan(0);
    expect(issue.backfillSqlEligible).toBe(false);
  }
}

function expectStatuses(issues: AuditIssue[], status: AuditIssueStatus) {
  expect(issues.every((issue) => issue.status === status)).toBe(true);
}
