import { describe, expect, it } from "vitest";
import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import type { TaskObjectiveCriterion, TaskObjectiveRun } from "@/lib/types/objective";
import { createObjectiveSlice } from "./objective-slice";
import type { ObjectiveSlice } from "./types";

function newStore() {
  return create<ObjectiveSlice>()(immer((set) => createObjectiveSlice(set)));
}

function run(overrides: Partial<TaskObjectiveRun> = {}): TaskObjectiveRun {
  return {
    id: "run-1",
    task_id: "t1",
    session_id: "s1",
    kind: "objective_check",
    trigger: "manual",
    workflow_step_id: "",
    agent_id: "claude-acp",
    model: "haiku",
    status: "pending",
    verdict: "",
    error_code: "",
    error_message: "",
    summary: "",
    finding_count: 0,
    file_count: 0,
    repository_count: 0,
    prompt_tokens: 0,
    response_tokens: 0,
    duration_ms: 0,
    created_at: "2026-08-24T10:00:00Z",
    ...overrides,
  } as TaskObjectiveRun;
}

function criterion(overrides: Partial<TaskObjectiveCriterion> = {}): TaskObjectiveCriterion {
  return {
    id: "c1",
    run_id: "run-1",
    task_id: "t1",
    ordinal: 0,
    source: "derived",
    source_ref: "",
    text: "Does the thing",
    status: "met",
    rationale: "",
    evidence: [],
    created_at: "2026-08-24T10:00:00Z",
    ...overrides,
  };
}

describe("setTaskAssessment", () => {
  it("replaces state and marks the task loaded, newest run first, capped", () => {
    const store = newStore();
    const runs = Array.from({ length: 30 }, (_, i) =>
      run({ id: `run-${i}`, created_at: `2026-08-24T${String(i).padStart(2, "0")}:00:00Z` }),
    );
    store.getState().setTaskAssessment("t1", { runs, criteria: [criterion()], verdict: "partial" });

    const state = store.getState().taskObjective;
    expect(state.runsByTaskId.t1).toHaveLength(20);
    expect(state.runsByTaskId.t1[0].id).toBe("run-29");
    expect(state.assessmentsByTaskId.t1.verdict).toBe("partial");
    expect(state.loadedTaskIds.t1).toBe(true);
  });
});

describe("setAssessmentResult", () => {
  it("replaces criteria and verdict without touching run history", () => {
    const store = newStore();
    store.getState().upsertAssessmentRun("t1", run());
    store.getState().setAssessmentResult("t1", {
      criteria: [criterion({ status: "unmet" })],
      verdict: "unmet",
    });

    const state = store.getState().taskObjective;
    expect(state.runsByTaskId.t1).toHaveLength(1);
    expect(state.assessmentsByTaskId.t1.verdict).toBe("unmet");
    expect(state.assessmentsByTaskId.t1.criteria[0].status).toBe("unmet");
  });
});

describe("upsertAssessmentRun", () => {
  it("replaces a run by id rather than appending", () => {
    const store = newStore();
    store.getState().upsertAssessmentRun("t1", run({ status: "pending" }));
    store.getState().upsertAssessmentRun("t1", run({ status: "running" }));

    const runs = store.getState().taskObjective.runsByTaskId.t1;
    expect(runs).toHaveLength(1);
    expect(runs[0].status).toBe("running");
  });
});

describe("clearTaskAssessmentState", () => {
  it("drops runs, assessment, and the loaded marker for that task only", () => {
    const store = newStore();
    store.getState().setTaskAssessment("t1", { runs: [run()], criteria: [], verdict: "met" });
    store.getState().setTaskAssessment("t2", { runs: [run({ id: "o" })], criteria: [], verdict: "" });
    store.getState().clearTaskAssessmentState("t1");

    const state = store.getState().taskObjective;
    expect(state.runsByTaskId.t1).toBeUndefined();
    expect(state.assessmentsByTaskId.t1).toBeUndefined();
    expect(state.loadedTaskIds.t1).toBeUndefined();
    expect(state.runsByTaskId.t2).toHaveLength(1);
  });
});
