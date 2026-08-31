import type {
  EvidencePointer,
  ObjectiveCriterionStatus,
  ObjectiveVerdict,
  TaskObjectiveCriterion,
  TaskObjectiveRun,
} from "@/lib/types/objective";

/** Semantic tone for a verdict banner, matching the review severity palette. */
export type ObjectiveTone = "positive" | "warning" | "negative" | "neutral";

/** Banner tone for a rolled-up verdict. */
export function verdictTone(verdict: ObjectiveVerdict): ObjectiveTone {
  switch (verdict) {
    case "met":
      return "positive";
    case "partial":
      return "warning";
    case "unmet":
      return "negative";
    default:
      return "neutral";
  }
}

/** Row tone for a single criterion's status. */
export function criterionTone(status: ObjectiveCriterionStatus): ObjectiveTone {
  switch (status) {
    case "met":
      return "positive";
    case "partial":
      return "warning";
    case "unmet":
      return "negative";
    default:
      return "neutral";
  }
}

/**
 * How many criteria are not met. `partial` and `unknown` both count as not-met,
 * matching the backend verdict rollup.
 */
export function unmetCount(criteria: TaskObjectiveCriterion[]): number {
  return criteria.filter((c) => c.status !== "met").length;
}

/**
 * True when an evidence pointer targets a file in the reviewed change set, so
 * the row can offer a jump into the diff. A pointer to an unchanged file (or one
 * the diff viewer has no section for) renders as plain text instead.
 */
export function evidenceNavigable(
  pointer: EvidencePointer,
  changedFiles: Iterable<string>,
): boolean {
  if (!pointer.file) return false;
  const set = changedFiles instanceof Set ? changedFiles : new Set(changedFiles);
  return set.has(pointer.file);
}

/** The task's newest assessment run, which the toolbar reflects. */
export function latestAssessmentRun(runs: TaskObjectiveRun[]): TaskObjectiveRun | null {
  if (runs.length === 0) return null;
  return [...runs].sort((a, b) => b.created_at.localeCompare(a.created_at))[0];
}

/** True while an assessment run is still in flight and can be cancelled. */
export function isAssessmentRunActive(run: TaskObjectiveRun | null | undefined): boolean {
  return run?.status === "pending" || run?.status === "running";
}
