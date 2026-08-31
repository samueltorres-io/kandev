import { describe, expect, it } from "vitest";
import { formatCriterionAsMarkdown } from "./format";
import type { TaskObjectiveCriterion } from "@/lib/types/objective";

function criterion(overrides: Partial<TaskObjectiveCriterion> = {}): TaskObjectiveCriterion {
  return {
    id: "c1",
    run_id: "r1",
    task_id: "t1",
    ordinal: 0,
    source: "derived",
    source_ref: "",
    text: "Adds a button",
    status: "unmet",
    rationale: "",
    evidence: [],
    created_at: "2026-08-01T10:00:00Z",
    ...overrides,
  };
}

describe("formatCriterionAsMarkdown", () => {
  it("includes the status label, source ref, rationale, and evidence locations", () => {
    const md = formatCriterionAsMarkdown(
      criterion({
        source_ref: "AC-3",
        rationale: "the handler is missing",
        evidence: [
          { file: "a.ts", line: 4, line_end: 8 },
          { repo: "web", file: "b.ts", line: 2 },
          { file: "c.ts" },
        ],
      }),
    );
    expect(md).toContain("**Not met** (AC-3)");
    expect(md).toContain("the handler is missing");
    expect(md).toContain("- a.ts:4-8");
    expect(md).toContain("- web/b.ts:2");
    expect(md).toContain("- c.ts");
  });

  it("omits the ref and evidence section when absent", () => {
    const md = formatCriterionAsMarkdown(criterion());
    expect(md).toContain("**Not met**");
    expect(md).not.toContain("Evidence:");
  });
});
