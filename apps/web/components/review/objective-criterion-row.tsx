"use client";

import { useState } from "react";
import {
  IconChevronDown,
  IconChevronRight,
  IconCircleCheck,
  IconCircleDashed,
  IconCircleDot,
  IconCircleX,
  IconSend,
} from "@tabler/icons-react";
import { Button } from "@kandev/ui/button";
import { ClarificationMarkdown } from "@/components/task/chat/clarification-markdown";
import { reviewFileKey } from "@/components/review/types";
import { CRITERION_STATUS_LABEL_KEYS } from "@/lib/objective/format";
import type { ObjectiveCriterionStatus, TaskObjectiveCriterion } from "@/lib/types/objective";
import { useTranslation } from "react-i18next";

const STATUS_ICON: Record<ObjectiveCriterionStatus, typeof IconCircleCheck> = {
  met: IconCircleCheck,
  partial: IconCircleDot,
  unmet: IconCircleX,
  unknown: IconCircleDashed,
};

const STATUS_CLASS: Record<ObjectiveCriterionStatus, string> = {
  met: "text-emerald-600 dark:text-emerald-400",
  partial: "text-amber-600 dark:text-amber-400",
  unmet: "text-destructive",
  unknown: "text-muted-foreground",
};

type EvidenceRef = TaskObjectiveCriterion["evidence"][number];

function evidenceLabel(pointer: EvidenceRef): string {
  const path = pointer.repo ? `${pointer.repo}/${pointer.file}` : pointer.file;
  if (!pointer.line) return path;
  if (pointer.line_end && pointer.line_end > pointer.line) {
    return `${path}:${pointer.line}-${pointer.line_end}`;
  }
  return `${path}:${pointer.line}`;
}

function EvidenceList({
  evidence,
  changedFileKeys,
  onNavigateToFile,
}: {
  evidence: EvidenceRef[];
  changedFileKeys: Set<string>;
  onNavigateToFile: (fileKey: string) => void;
}) {
  if (evidence.length === 0) return null;
  return (
    <ul className="mt-1.5 space-y-0.5">
      {evidence.map((pointer, index) => {
        const key = reviewFileKey({
          path: pointer.file,
          repository_name: pointer.repo || undefined,
        });
        const navigable = Boolean(pointer.file) && changedFileKeys.has(key);
        const label = evidenceLabel(pointer);
        return (
          <li key={`${key}-${index}`} className="text-[11px]">
            {navigable ? (
              <button
                type="button"
                onClick={() => onNavigateToFile(key)}
                className="cursor-pointer font-mono text-primary underline underline-offset-2"
                data-testid="objective-evidence-link"
              >
                {label}
              </button>
            ) : (
              <span className="font-mono text-muted-foreground" data-testid="objective-evidence-text">
                {label}
              </span>
            )}
          </li>
        );
      })}
    </ul>
  );
}

export type ObjectiveCriterionRowProps = {
  criterion: TaskObjectiveCriterion;
  changedFileKeys: Set<string>;
  onNavigateToFile: (fileKey: string) => void;
  onSendToAgent?: (criterion: TaskObjectiveCriterion) => void;
};

export function ObjectiveCriterionRow({
  criterion,
  changedFileKeys,
  onNavigateToFile,
  onSendToAgent,
}: ObjectiveCriterionRowProps) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const Icon = STATUS_ICON[criterion.status] ?? IconCircleDashed;
  const hasDetail = Boolean(criterion.rationale) || criterion.evidence.length > 0;

  return (
    <div
      className="rounded-md border border-border/50 bg-muted/20 px-2 py-1.5"
      data-testid="objective-criterion-row"
      data-status={criterion.status}
    >
      <div className="flex items-start gap-1.5">
        <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${STATUS_CLASS[criterion.status]}`} />
        <button
          type="button"
          className="min-w-0 flex-1 cursor-pointer text-left"
          onClick={() => hasDetail && setExpanded((v) => !v)}
          aria-expanded={hasDetail ? expanded : undefined}
        >
          <span className="text-xs font-medium leading-snug text-foreground/90">
            {criterion.text}
          </span>
          {criterion.source_ref && (
            <span className="ml-1.5 rounded bg-muted px-1 py-0.5 text-[10px] text-muted-foreground">
              {criterion.source_ref}
            </span>
          )}
        </button>
        {hasDetail && (
          <span className="mt-0.5 shrink-0 text-muted-foreground">
            {expanded ? (
              <IconChevronDown className="h-3.5 w-3.5" />
            ) : (
              <IconChevronRight className="h-3.5 w-3.5" />
            )}
          </span>
        )}
        {onSendToAgent && (
          <Button
            size="sm"
            variant="ghost"
            className="h-6 shrink-0 cursor-pointer px-1"
            onClick={() => onSendToAgent(criterion)}
            aria-label={t("review:objectiveSendCriterionToAgent")}
            data-testid="objective-send-criterion"
          >
            <IconSend className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>

      <div className="pl-6 text-[11px] text-muted-foreground">
        {t(CRITERION_STATUS_LABEL_KEYS[criterion.status] ?? "review:objectiveStatusUnknown")}
      </div>

      {expanded && hasDetail && (
        <div className="mt-1.5 pl-6">
          {criterion.rationale && (
            <ClarificationMarkdown variant="block" linkBehavior="passive" className="text-xs">
              {criterion.rationale}
            </ClarificationMarkdown>
          )}
          <EvidenceList
            evidence={criterion.evidence}
            changedFileKeys={changedFileKeys}
            onNavigateToFile={onNavigateToFile}
          />
        </div>
      )}
    </div>
  );
}
