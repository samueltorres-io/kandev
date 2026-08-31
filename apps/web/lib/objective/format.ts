import type { TaskObjectiveCriterion } from "@/lib/types/objective";

/**
 * Status wording for the markdown block sent to an agent.
 *
 * Held stable in English so a reviewer's UI locale cannot change what the agent
 * is told. The on-screen row resolves `CRITERION_STATUS_LABEL_KEYS` instead.
 */
// i18n-exempt: agent-facing prompt content, deliberately not localized.
export const CRITERION_STATUS_LABELS: Record<string, string> = {
  met: "Met",
  partial: "Partially met",
  unmet: "Not met",
  unknown: "Undetermined",
};

/** Catalog keys for the same statuses, for anything rendered on screen. */
export const CRITERION_STATUS_LABEL_KEYS: Record<string, string> = {
  met: "review:objectiveStatusMet",
  partial: "review:objectiveStatusPartial",
  unmet: "review:objectiveStatusUnmet",
  unknown: "review:objectiveStatusUnknown",
};

export function criterionStatusLabel(status: string): string {
  return CRITERION_STATUS_LABELS[status] ?? status;
}

/** `repo/path:12-14` or `path:12`, the form used in agent context. */
function evidenceLocation(pointer: TaskObjectiveCriterion["evidence"][number]): string {
  const path = pointer.repo ? `${pointer.repo}/${pointer.file}` : pointer.file;
  if (!pointer.line) return path;
  if (pointer.line_end && pointer.line_end > pointer.line) {
    return `${path}:${pointer.line}-${pointer.line_end}`;
  }
  return `${path}:${pointer.line}`;
}

/**
 * Renders one criterion as the markdown block sent to an agent as follow-up
 * context. Mirrors `formatFindingAsMarkdown` so a criterion reads to the agent
 * like any other anchored review feedback.
 */
export function formatCriterionAsMarkdown(criterion: TaskObjectiveCriterion): string {
  const lines: string[] = ["### Objective Assessment Criterion", ""];
  const ref = criterion.source_ref ? ` (${criterion.source_ref})` : "";
  lines.push(`**${criterionStatusLabel(criterion.status)}**${ref}`, "", criterion.text);
  if (criterion.rationale) lines.push("", criterion.rationale);
  if (criterion.evidence.length > 0) {
    lines.push("", "Evidence:");
    for (const pointer of criterion.evidence) lines.push(`- ${evidenceLocation(pointer)}`);
  }
  lines.push("", "---", "");
  return lines.join("\n");
}
