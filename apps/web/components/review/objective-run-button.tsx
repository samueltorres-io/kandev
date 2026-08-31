"use client";

import { useCallback, useState } from "react";
import { IconLoader2, IconTargetArrow, IconX } from "@tabler/icons-react";
import { Button } from "@kandev/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@kandev/ui/tooltip";
import Link from "@/components/routing/app-link";
import {
  cancelObjectiveAssessment,
  runObjectiveAssessment,
} from "@/lib/api/domains/objective-api";
import { isAssessmentRunActive } from "@/lib/objective/verdict";
import { useHasObjectiveSource } from "@/hooks/domains/objective/use-has-objective-source";
import type { TaskObjectiveRun } from "@/lib/types/objective";
import { Trans, useTranslation } from "react-i18next";
import type { TFunction } from "i18next";

export type ObjectiveRunButtonProps = {
  taskId: string | null | undefined;
  sessionId?: string | null;
  activeRun: TaskObjectiveRun | null;
  compact?: boolean;
};

/**
 * Copy for a rejected run. `objective_agent_unavailable` is the one case the
 * user can fix, so it links to Settings rather than showing a dead end.
 */
function RunNotice({ code, message }: { code: string; message: string }) {
  const { t } = useTranslation();
  if (code === "objective_agent_unavailable") {
    return (
      <p className="text-xs text-muted-foreground" data-testid="objective-agent-unavailable">
        <Trans i18nKey="review:objectiveAgentUnavailableNotice">
          No inference-capable agent is configured for assessment.{" "}
          <Link href="/settings/utility-agents" className="cursor-pointer underline" />
        </Trans>
      </p>
    );
  }
  if (code === "objective_no_changes") {
    return (
      <p className="text-xs text-muted-foreground" data-testid="objective-no-changes">
        {t("review:objectiveNoChangesYet")}
      </p>
    );
  }
  return (
    <p className="text-xs text-destructive" data-testid="objective-run-error">
      {message}
    </p>
  );
}

function useRunAssessment(taskId: string | null | undefined, sessionId?: string | null) {
  const { t } = useTranslation();
  const [notice, setNotice] = useState<{ code: string; message: string } | null>(null);
  const [starting, setStarting] = useState(false);

  const start = useCallback(async () => {
    if (!taskId) return;
    setNotice(null);
    setStarting(true);
    try {
      await runObjectiveAssessment({ taskId, sessionId: sessionId ?? undefined });
    } catch (error) {
      const code = (error as { code?: string })?.code ?? "";
      const message = error instanceof Error ? error.message : t("review:objectiveCouldNotStart");
      setNotice({ code, message });
    } finally {
      setStarting(false);
    }
  }, [taskId, sessionId]);

  return { notice, starting, start, clearNotice: () => setNotice(null) };
}

function RunTrigger({
  busy,
  disabled,
  compact,
  onStart,
}: {
  busy: boolean;
  disabled: boolean;
  compact: boolean;
  onStart: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          size="sm"
          variant="ghost"
          className="cursor-pointer gap-1 px-2"
          onClick={onStart}
          disabled={disabled}
          aria-label={t("review:objectiveAssessTheseChanges")}
          data-testid="objective-run-changes"
        >
          {busy ? (
            <IconLoader2 className="h-4 w-4 animate-spin" />
          ) : (
            <IconTargetArrow className="h-4 w-4" />
          )}
          {!compact && <span className="text-xs">{t("review:objectiveAssess")}</span>}
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        {busy ? t("review:objectiveInProgress") : t("review:objectiveAssessTheseChanges")}
      </TooltipContent>
    </Tooltip>
  );
}

function resolveNotice(
  notice: { code: string; message: string } | null,
  activeRun: TaskObjectiveRun | null,
  t: TFunction,
): { code: string; message: string } | null {
  if (notice) return notice;
  if (activeRun?.status !== "failed") return null;
  return {
    code: activeRun.error_code,
    message: activeRun.error_message || t("review:objectiveAssessmentFailed"),
  };
}

export function ObjectiveRunButton({
  taskId,
  sessionId,
  activeRun,
  compact = false,
}: ObjectiveRunButtonProps) {
  const { t } = useTranslation();
  const hasSource = useHasObjectiveSource(taskId);
  const { notice, starting, start, clearNotice } = useRunAssessment(taskId, sessionId);
  const running = isAssessmentRunActive(activeRun);
  const shown = resolveNotice(notice, activeRun, t);

  const handleCancel = useCallback(async () => {
    if (!activeRun) return;
    clearNotice();
    await cancelObjectiveAssessment(activeRun.id).catch(() => {
      // Already terminal or unreachable; the run row carries the state.
    });
  }, [activeRun, clearNotice]);

  if (!hasSource) return null;

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <RunTrigger
        busy={running || starting}
        disabled={!taskId || running || starting}
        compact={compact}
        onStart={start}
      />

      {running && (
        <Button
          size="sm"
          variant="ghost"
          className="h-6 cursor-pointer gap-1 px-1.5 text-xs text-muted-foreground"
          onClick={handleCancel}
          data-testid="objective-cancel-run"
        >
          <IconX className="h-3.5 w-3.5" />
          {t("common:cancel")}
        </Button>
      )}

      {shown && <RunNotice code={shown.code} message={shown.message} />}
    </div>
  );
}
