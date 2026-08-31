"use client";

import { useState } from "react";
import { IconChevronDown, IconTargetArrow } from "@tabler/icons-react";
import { Button } from "@kandev/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@kandev/ui/popover";
import { ObjectivePanel } from "./objective-panel";
import { verdictTone } from "@/lib/objective/verdict";
import type { ObjectiveVerdict, TaskObjectiveCriterion } from "@/lib/types/objective";
import { useTranslation } from "react-i18next";

const TRIGGER_TONE_CLASS: Record<string, string> = {
  positive: "text-emerald-600 dark:text-emerald-400",
  warning: "text-amber-600 dark:text-amber-400",
  negative: "text-destructive",
  neutral: "text-muted-foreground",
};

const VERDICT_SHORT_KEYS: Record<Exclude<ObjectiveVerdict, "">, string> = {
  met: "review:objectiveVerdictShortMet",
  partial: "review:objectiveVerdictShortPartial",
  unmet: "review:objectiveVerdictShortUnmet",
};

export type ObjectiveButtonProps = {
  verdict: ObjectiveVerdict;
  summary: string;
  criteria: TaskObjectiveCriterion[];
  changedFileKeys: Set<string>;
  onNavigateToFile: (fileKey: string) => void;
  onSendToAgent?: (criterion: TaskObjectiveCriterion) => void;
};

/**
 * The objective-verdict control on the review top bar: a chip carrying the
 * rolled-up verdict that opens the full assessment panel. Renders nothing until
 * there is an assessment to show.
 */
export function ObjectiveButton({
  verdict,
  summary,
  criteria,
  changedFileKeys,
  onNavigateToFile,
  onSendToAgent,
}: ObjectiveButtonProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  if (verdict === "" && criteria.length === 0) return null;

  const tone = verdictTone(verdict);
  const shortLabel =
    verdict === "" ? t("review:objectiveVerdictShortPending") : t(VERDICT_SHORT_KEYS[verdict]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          size="sm"
          variant="outline"
          className={`h-6 cursor-pointer gap-1 px-1.5 text-[10px] ${TRIGGER_TONE_CLASS[tone]}`}
          aria-label={t("review:objectiveOpenPanel")}
          data-testid="objective-open-panel"
        >
          <IconTargetArrow className="h-3 w-3" />
          {shortLabel}
          <IconChevronDown className="h-3 w-3" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-96 p-0" data-testid="objective-popover">
        <ObjectivePanel
          verdict={verdict}
          summary={summary}
          criteria={criteria}
          changedFileKeys={changedFileKeys}
          onNavigateToFile={(fileKey) => {
            setOpen(false);
            onNavigateToFile(fileKey);
          }}
          onSendToAgent={onSendToAgent}
        />
      </PopoverContent>
    </Popover>
  );
}
