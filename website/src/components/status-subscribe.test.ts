import { describe, expect, test } from "bun:test";
import type { StatusComponent } from "@/lib/status";
import { initialStatusSubscriptionComponentIds } from "./status-subscribe";

function component(id: number): StatusComponent {
  return {
    id,
    slug: `model-${id}`,
    kind: "model",
    display_name: `Model ${id}`,
    lifecycle: "active",
    status: "operational",
    last_trustworthy_update_at: 1,
    coverage: 1_000_000,
  };
}

describe("status subscription", () => {
  test("selects only the visible one hundred components by default", () => {
    const ids = initialStatusSubscriptionComponentIds(Array.from({ length: 101 }, (_, index) => component(index + 1)));

    expect(ids).toHaveLength(100);
    expect(ids[0]).toBe(1);
    expect(ids.at(-1)).toBe(100);
    expect(ids).not.toContain(101);
  });
});
