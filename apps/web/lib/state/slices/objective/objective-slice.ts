import type { StateCreator } from "zustand";
import type { TaskObjectiveRun } from "@/lib/types/objective";
import type { ObjectiveSlice, ObjectiveSliceState } from "./types";

export const defaultObjectiveState: ObjectiveSliceState = {
  taskObjective: {
    assessmentsByTaskId: {},
    runsByTaskId: {},
    loadedTaskIds: {},
  },
};

/** How many runs a task keeps client-side; matches the backend read cap. */
const RUN_HISTORY_LIMIT = 20;

type ImmerSet = Parameters<
  StateCreator<ObjectiveSlice, [["zustand/immer", never]], [], ObjectiveSlice>
>[0];

/** Newest run first, so the toolbar always reflects the latest pass. */
function sortRunsNewestFirst(runs: TaskObjectiveRun[]): TaskObjectiveRun[] {
  return [...runs].sort((a, b) => b.created_at.localeCompare(a.created_at));
}

/**
 * Typed as a plain factory over `set` rather than a full `StateCreator` because
 * that is all this slice uses (matches `createReviewSlice`).
 */
export const createObjectiveSlice = (set: ImmerSet): ObjectiveSlice => ({
  ...defaultObjectiveState,

  setTaskAssessment: (taskId, snapshot) =>
    set((draft) => {
      draft.taskObjective.runsByTaskId[taskId] = sortRunsNewestFirst(snapshot.runs).slice(
        0,
        RUN_HISTORY_LIMIT,
      );
      draft.taskObjective.assessmentsByTaskId[taskId] = {
        criteria: snapshot.criteria,
        verdict: snapshot.verdict,
      };
      draft.taskObjective.loadedTaskIds[taskId] = true;
    }),

  setAssessmentResult: (taskId, result) =>
    set((draft) => {
      draft.taskObjective.assessmentsByTaskId[taskId] = {
        criteria: result.criteria,
        verdict: result.verdict,
      };
    }),

  upsertAssessmentRun: (taskId, run) =>
    set((draft) => {
      const existing = draft.taskObjective.runsByTaskId[taskId] ?? [];
      const without = existing.filter((r) => r.id !== run.id);
      draft.taskObjective.runsByTaskId[taskId] = sortRunsNewestFirst([run, ...without]).slice(
        0,
        RUN_HISTORY_LIMIT,
      );
    }),

  clearTaskAssessmentState: (taskId) =>
    set((draft) => {
      delete draft.taskObjective.assessmentsByTaskId[taskId];
      delete draft.taskObjective.runsByTaskId[taskId];
      delete draft.taskObjective.loadedTaskIds[taskId];
    }),
});
