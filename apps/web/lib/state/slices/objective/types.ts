import type {
  ObjectiveVerdict,
  TaskObjectiveCriterion,
  TaskObjectiveRun,
  TaskObjectiveSnapshot,
} from "@/lib/types/objective";

export type ObjectiveSliceState = {
  taskObjective: {
    /** Latest completed assessment per task: criteria + rolled-up verdict. */
    assessmentsByTaskId: Record<string, { criteria: TaskObjectiveCriterion[]; verdict: ObjectiveVerdict }>;
    /** Bounded run history per task, newest first. */
    runsByTaskId: Record<string, TaskObjectiveRun[]>;
    /** Tasks whose assessment has been backfilled, so mount does not refetch. */
    loadedTaskIds: Record<string, boolean>;
  };
};

export type ObjectiveSliceActions = {
  /** Replaces a task's assessment state from a backfill snapshot. */
  setTaskAssessment: (taskId: string, snapshot: TaskObjectiveSnapshot) => void;
  /** Replaces the current assessment's criteria + verdict (a `published` event). */
  setAssessmentResult: (
    taskId: string,
    result: { criteria: TaskObjectiveCriterion[]; verdict: ObjectiveVerdict },
  ) => void;
  /** Inserts or replaces a run by id, keeping history capped and newest-first. */
  upsertAssessmentRun: (taskId: string, run: TaskObjectiveRun) => void;
  /** Drops all assessment state for a task. */
  clearTaskAssessmentState: (taskId: string) => void;
};

export type ObjectiveSlice = ObjectiveSliceState & ObjectiveSliceActions;
