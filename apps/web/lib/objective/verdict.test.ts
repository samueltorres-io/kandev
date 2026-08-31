import { describe, expect, it } from "vitest";
import type { TaskObjectiveCriterion } from "@/lib/types/objective";
import {
  criterionTone,
  evidenceNavigable,
  latestAssessmentRun,
  unmetCount,
  verdictTone,
} from "./verdict";

function criterion(status: TaskObjectiveCriterion["status"]): TaskObjectiveCriterion {
  return {
    id: status,
    run_id: "r",
    task_id: "t",
    ordinal: 0,
    source: "derived",
    source_ref: "",
    text: status,
    status,
    rationale: "",
    evidence: [],
    created_at: "2026-08-24T10:00:00Z",
  };
}

describe("verdictTone / criterionTone", () => {
  it("maps met/partial/unmet and defaults to neutral", () => {
    expect(verdictTone("met")).toBe("positive");
    expect(verdictTone("partial")).toBe("warning");
    expect(verdictTone("unmet")).toBe("negative");
    expect(verdictTone("")).toBe("neutral");
    expect(criterionTone("unknown")).toBe("neutral");
  });
});

describe("unmetCount", () => {
  it("counts everything that is not met", () => {
    expect(
      unmetCount([
        criterion("met"),
        criterion("partial"),
        criterion("unmet"),
        criterion("unknown"),
      ]),
    ).toBe(3);
  });
});

describe("evidenceNavigable", () => {
  it("is true only for a file in the changed set", () => {
    expect(evidenceNavigable({ file: "a.go" }, ["a.go", "b.go"])).toBe(true);
    expect(evidenceNavigable({ file: "c.go" }, ["a.go", "b.go"])).toBe(false);
    expect(evidenceNavigable({ file: "" }, ["a.go"])).toBe(false);
  });
});

describe("latestAssessmentRun", () => {
  it("returns the newest run by created_at, or null", () => {
    expect(latestAssessmentRun([])).toBeNull();
    const runs = [
      { id: "old", created_at: "2026-08-24T09:00:00Z" },
      { id: "new", created_at: "2026-08-24T11:00:00Z" },
    ] as never;
    expect(latestAssessmentRun(runs)!.id).toBe("new");
  });
});
