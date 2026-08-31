import { describe, expect, it, vi } from "vitest";
import type { StoreApi } from "zustand";
import type { AppState } from "@/lib/state/store";
import type { TaskObjectiveRun } from "@/lib/types/objective";
import { registerObjectiveHandlers } from "./objective";

function newFakeStore() {
  const actions = {
    upsertAssessmentRun: vi.fn(),
    setAssessmentResult: vi.fn(),
    clearTaskAssessmentState: vi.fn(),
  };
  const store = { getState: () => actions } as unknown as StoreApi<AppState>;
  return { store, actions };
}

const run = { id: "run-1", task_id: "t1" } as TaskObjectiveRun;

function message<T>(action: string, payload: T) {
  return { action, payload } as never;
}

describe("registerObjectiveHandlers", () => {
  it("upserts a run on task.objective.run_updated", () => {
    const { store, actions } = newFakeStore();
    const handlers = registerObjectiveHandlers(store);
    handlers["task.objective.run_updated"]!(
      message("task.objective.run_updated", { task_id: "t1", run }),
    );
    expect(actions.upsertAssessmentRun).toHaveBeenCalledWith("t1", run);
  });

  it("sets criteria and verdict on task.objective.published", () => {
    const { store, actions } = newFakeStore();
    const handlers = registerObjectiveHandlers(store);
    handlers["task.objective.published"]!(
      message("task.objective.published", {
        task_id: "t1",
        run_id: "run-1",
        verdict: "partial",
        summary: "s",
        criteria: [{ id: "c1" }],
      }),
    );
    expect(actions.setAssessmentResult).toHaveBeenCalledWith("t1", {
      criteria: [{ id: "c1" }],
      verdict: "partial",
    });
  });

  it("clears state on task.objective.cleared", () => {
    const { store, actions } = newFakeStore();
    const handlers = registerObjectiveHandlers(store);
    handlers["task.objective.cleared"]!(message("task.objective.cleared", { task_id: "t1" }));
    expect(actions.clearTaskAssessmentState).toHaveBeenCalledWith("t1");
  });

  it("ignores malformed payloads", () => {
    const { store, actions } = newFakeStore();
    const handlers = registerObjectiveHandlers(store);
    handlers["task.objective.run_updated"]!(
      message("task.objective.run_updated", { task_id: "", run }),
    );
    handlers["task.objective.run_updated"]!(
      message("task.objective.run_updated", { task_id: "t1", run: null }),
    );
    handlers["task.objective.published"]!(
      message("task.objective.published", { task_id: "" }),
    );
    handlers["task.objective.cleared"]!(message("task.objective.cleared", { task_id: "" }));

    expect(actions.upsertAssessmentRun).not.toHaveBeenCalled();
    expect(actions.setAssessmentResult).not.toHaveBeenCalled();
    expect(actions.clearTaskAssessmentState).not.toHaveBeenCalled();
  });
});
