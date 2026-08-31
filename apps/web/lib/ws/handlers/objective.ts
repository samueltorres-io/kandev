import type { StoreApi } from "zustand";
import type { AppState } from "@/lib/state/store";
import type { WsHandlers } from "@/lib/ws/handlers/types";

/**
 * Live objective-assessment updates. Runs and criteria are backend-persisted, so
 * they arrive here the same way native code-review updates do.
 */
export function registerObjectiveHandlers(store: StoreApi<AppState>): WsHandlers {
  return {
    "task.objective.run_updated": (message) => {
      const { task_id, run } = message.payload;
      if (!task_id || !run) return;
      store.getState().upsertAssessmentRun(task_id, run);
    },
    "task.objective.published": (message) => {
      const { task_id, criteria, verdict } = message.payload;
      if (!task_id) return;
      store.getState().setAssessmentResult(task_id, {
        criteria: criteria ?? [],
        verdict: verdict ?? "",
      });
    },
    "task.objective.cleared": (message) => {
      const { task_id } = message.payload;
      if (!task_id) return;
      store.getState().clearTaskAssessmentState(task_id);
    },
  };
}
