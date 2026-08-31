"use client";

import { useEffect, useMemo } from "react";
import { useAppStore, useAppStoreApi } from "@/components/state-provider";
import { getObjectiveAssessment } from "@/lib/api/domains/objective-api";
import { latestAssessmentRun } from "@/lib/objective/verdict";
import type {
  ObjectiveVerdict,
  TaskObjectiveCriterion,
  TaskObjectiveRun,
} from "@/lib/types/objective";

const EMPTY_RUNS: TaskObjectiveRun[] = [];
const EMPTY_CRITERIA: TaskObjectiveCriterion[] = [];

/**
 * Reads a task's objective-assessment state, backfilling it once on mount.
 *
 * Live `task.objective.*` events can fire before the page's WS subscription is
 * established, so the store is seeded from a one-shot read the same way
 * `useTaskReview` does.
 */
export function useTaskObjective(taskId: string | null | undefined) {
  const storeApi = useAppStoreApi();
  const runs = useAppStore((state) =>
    taskId ? (state.taskObjective.runsByTaskId[taskId] ?? EMPTY_RUNS) : EMPTY_RUNS,
  );
  const assessment = useAppStore((state) =>
    taskId ? state.taskObjective.assessmentsByTaskId[taskId] : undefined,
  );
  const loaded = useAppStore((state) =>
    taskId ? Boolean(state.taskObjective.loadedTaskIds[taskId]) : false,
  );

  useEffect(() => {
    if (!taskId || loaded) return;
    let cancelled = false;
    getObjectiveAssessment(taskId)
      .then((snapshot) => {
        if (cancelled) return;
        storeApi.getState().setTaskAssessment(taskId, snapshot);
      })
      .catch(() => {
        // A failed backfill is not worth a toast: live events still populate the
        // panel, and the run control surfaces any real failure itself.
      });
    return () => {
      cancelled = true;
    };
  }, [taskId, loaded, storeApi]);

  return useMemo(() => {
    const activeRun = latestAssessmentRun(runs);
    const criteria = assessment?.criteria ?? EMPTY_CRITERIA;
    const verdict: ObjectiveVerdict = assessment?.verdict ?? activeRun?.verdict ?? "";
    return { runs, criteria, assessment, activeRun, verdict };
  }, [runs, assessment]);
}
