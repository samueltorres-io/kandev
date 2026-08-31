"use client";

import { IconTargetArrow } from "@tabler/icons-react";
import { ObjectiveCriterionRow } from "./objective-criterion-row";
import { unmetCount, verdictTone } from "@/lib/objective/verdict";
import type {
  ObjectiveVerdict,
  TaskObjectiveCriterion,
} from "@/lib/types/objective";
import { useTranslation } from "react-i18next";

const VERDICT_LABEL_KEYS: Record<Exclude<ObjectiveVerdict, "">, string> = {
  met: "review:objectiveVerdictMet",
  partial: "review:objectiveVerdictPartial",
  unmet: "review:objectiveVerdictUnmet",
};

const TONE_CLASS: Record<string, string> = {
  positive: "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
  warning: "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-400",
  negative: "border-destructive/40 bg-destructive/10 text-destructive",
  neutral: "border-border bg-muted/40 text-muted-foreground",
};

export type ObjectivePanelProps = {
  verdict: ObjectiveVerdict;
  summary: string;
  criteria: TaskObjectiveCriterion[];
  changedFileKeys: Set<string>;
  onNavigateToFile: (fileKey: string) => void;
  onSendToAgent?: (criterion: TaskObjectiveCriterion) => void;
};

/**
 * The objective-assessment result: a verdict banner, the agent's summary, and
 * the criterion checklist. The verdict is advisory; the human decides what to
 * do with it.
 */
export function ObjectivePanel({
  verdict,
  summary,
  criteria,
  changedFileKeys,
  onNavigateToFile,
  onSendToAgent,
}: ObjectivePanelProps) {
  const { t } = useTranslation();

  if (verdict === "" && criteria.length === 0) {
    return (
      <div
        className="px-3 py-4 text-center text-xs text-muted-foreground"
        data-testid="objective-panel-empty"
      >
        {t("review:objectiveNoAssessmentYet")}
      </div>
    );
  }

  const notMet = unmetCount(criteria);
  const tone = verdictTone(verdict);
  const label = verdict === "" ? t("review:objectiveVerdictPending") : t(VERDICT_LABEL_KEYS[verdict]);

  return (
    <div className="flex max-h-[min(60vh,30rem)] flex-col" data-testid="objective-panel">
      <div className={`flex items-center gap-2 border-b px-3 py-2 ${TONE_CLASS[tone]}`}>
        <IconTargetArrow className="h-4 w-4 shrink-0" />
        <span className="text-sm font-semibold">{label}</span>
        {criteria.length > 0 && (
          <span className="ml-auto text-xs">
            {t("review:objectiveUnmetCount", { count: notMet, total: criteria.length })}
          </span>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 py-2">
        {summary && (
          <p className="mb-2 px-1 text-xs leading-snug text-muted-foreground">{summary}</p>
        )}
        <div className="space-y-1">
          {criteria.map((criterion) => (
            <ObjectiveCriterionRow
              key={criterion.id}
              criterion={criterion}
              changedFileKeys={changedFileKeys}
              onNavigateToFile={onNavigateToFile}
              onSendToAgent={onSendToAgent}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
