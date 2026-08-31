"use client";

import { useAppStore } from "@/components/state-provider";

/**
 * True when a task has something an assessment can derive criteria from: a
 * non-empty description or a saved plan document. When false, the run control is
 * hidden so `objective_no_objective` is normally unreachable from the UI.
 *
 * v1 limitation: the backend does not read `docs/specs/**`, so neither does this.
 */
export function useHasObjectiveSource(taskId: string | null | undefined): boolean {
  const hasDescription = useAppStore((state) =>
    Boolean(taskId && state.kanban.tasks.find((task) => task.id === taskId)?.description?.trim()),
  );
  const hasPlan = useAppStore((state) => Boolean(taskId && state.taskPlans.byTaskId[taskId]));
  return hasDescription || hasPlan;
}
