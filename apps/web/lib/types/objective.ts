import type { BackendMessage } from "@/lib/types/backend";
import type { TaskReviewRun } from "@/lib/types/review";

/** Rolled-up disposition of a task against its stated objective. */
export type ObjectiveVerdict = "" | "met" | "partial" | "unmet";

/** Per-criterion judgement. `unknown` and `partial` count as not-met. */
export type ObjectiveCriterionStatus = "met" | "partial" | "unmet" | "unknown";

/** Where a criterion came from: derived from the objective text, or lifted from a doc. */
export type ObjectiveCriterionSource = "derived" | "document";

/** Machine-readable failure codes the objective surface branches on. */
export type ObjectiveErrorCode =
  | "objective_agent_unavailable"
  | "objective_workspace_unavailable"
  | "objective_no_changes"
  | "objective_no_objective"
  | "objective_unparseable_response"
  | "objective_execution_failed"
  | "objective_cancelled";

/** One file/line pointer an assessment attached to a criterion. */
export type EvidencePointer = {
  repo?: string;
  file: string;
  line?: number;
  line_end?: number;
};

/**
 * An objective-assessment run. It is the same `task_review_runs` row shape as a
 * code-review run (`kind: "objective_check"`), plus the rolled-up `verdict`.
 */
export type TaskObjectiveRun = TaskReviewRun & {
  verdict: ObjectiveVerdict;
};

export type TaskObjectiveCriterion = {
  id: string;
  run_id: string;
  task_id: string;
  ordinal: number;
  source: ObjectiveCriterionSource;
  /** Heading or list-item ref (for example `AC-3`); empty for a free-form criterion. */
  source_ref: string;
  text: string;
  status: ObjectiveCriterionStatus;
  rationale: string;
  evidence: EvidencePointer[];
  created_at: string;
};

/** Response shape of `task.objective.get`. */
export type TaskObjectiveAssessment = {
  runs: TaskObjectiveRun[];
  criteria: TaskObjectiveCriterion[];
  verdict: ObjectiveVerdict;
};

/** The slice's per-task snapshot: latest assessment plus run history. */
export type TaskObjectiveSnapshot = {
  runs: TaskObjectiveRun[];
  criteria: TaskObjectiveCriterion[];
  verdict: ObjectiveVerdict;
};

export type ObjectiveBackendMessageMap = {
  "task.objective.run_updated": BackendMessage<
    "task.objective.run_updated",
    { task_id: string; run: TaskObjectiveRun }
  >;
  "task.objective.published": BackendMessage<
    "task.objective.published",
    {
      task_id: string;
      run_id: string;
      verdict: ObjectiveVerdict;
      summary: string;
      criteria: TaskObjectiveCriterion[];
    }
  >;
  "task.objective.cleared": BackendMessage<"task.objective.cleared", { task_id: string }>;
};
